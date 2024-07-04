package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/bioflowy/flowy-cwl3/flowydeamon/api"
)

type S3FileManager struct {
	RootUrl         string
	Region          string
	Endpoint        *string
	AccessKey       *string
	SecretKey       *string
	session         *session.Session
	DownloadFileMap map[string]string
}

func (f *S3FileManager) GetDownloadFileMap() map[string]string {
	return f.DownloadFileMap
}
func (f *S3FileManager) CopyFile(srcPath string, destPath string) error {
	_, err := downloadS3File(f.session, srcPath, &destPath)
	return err
}
func (fm *S3FileManager) CopyDir(src, dest string) error {
	_, err := downloadS3File(fm.session, src, &dest)
	return err
}
func (f *S3FileManager) NeedDownload(fileurl string) bool {
	// always need download when s3 file
	return true
}

func (f *S3FileManager) Download(fileurl string, localPath string) (bool, error) {
	// if file exists on s3 bucket, download that to tmpfile and return tmpfile path
	_, err := downloadS3File(f.session, fileurl, &localPath)
	f.DownloadFileMap[fileurl] = localPath
	return true, err
}
func (f *S3FileManager) GetType() string {
	// do nothing when shared file system is available
	return "s3"
}
func (f *S3FileManager) GetFileUrl(localPath string, fileurl *string) (string, error) {
	return uploadToS3(f, localPath, fileurl)
}

func NewFileManager(config *api.SharedFileSystemConfig) (FileManager, error) {
	if config.Type == "nfs" {
		return &LocalFileManager{}, nil
	} else if config.Type == "s3" {
		sess, err := session.NewSession(&aws.Config{
			Credentials:      credentials.NewStaticCredentials(*config.AccessKey, *config.SecretKey, ""),
			Region:           aws.String(*config.Region), // Set your AWS region
			Endpoint:         config.Endpoint,
			S3ForcePathStyle: aws.Bool(true),
		})
		if err != nil {
			return nil, err
		}
		return &S3FileManager{
			RootUrl:         config.RootUrl,
			Region:          *config.Region,
			Endpoint:        config.Endpoint,
			AccessKey:       config.AccessKey,
			SecretKey:       config.SecretKey,
			session:         sess,
			DownloadFileMap: map[string]string{},
		}, nil
	} else {
		panic("Unexpected File System Type")
	}
}
func uploadFile(uploader *s3manager.Uploader, fileManager *S3FileManager, filePath string, s3url *string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	u, err := url.Parse(fileManager.RootUrl)
	if err != nil {
		return "", err
	}
	filePath = strings.TrimPrefix(filePath, "/")
	var key = ""
	if s3url != nil {
		u, err := url.Parse(*s3url)
		if err != nil {
			return "", err
		}

		key = u.Path
	} else {
		key = strings.TrimPrefix(filepath.Join(u.Path, filePath), "/")
	}

	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(u.Host), // Set your bucket name
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		return "", err
	}
	return "s3://" + u.Host + "/" + key, nil
}
func uploadDirectory(uploader *s3manager.Uploader, fileManager *S3FileManager, directoryPath string, s3url *string) (string, error) {
	u, err := url.Parse(fileManager.RootUrl)
	if err != nil {
		return "", err
	}

	err = WalkWithSymlink(directoryPath, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			var s3urlp *string
			if s3url != nil {
				//もともとのs3
				news3url := *s3url + strings.TrimPrefix(path, directoryPath)
				s3urlp = &news3url
			}
			_, err := uploadFile(uploader, fileManager, path, s3urlp)
			if err != nil {
				return err
			}
		} else {
			emptyBuffer := bytes.NewBuffer([]byte{})
			key := filepath.Join(u.Path, path)
			if s3url != nil {
				org, err := url.Parse(*s3url)
				if err != nil {
					return err
				}
				key = filepath.Join(org.Path, strings.TrimPrefix(path, directoryPath))
			}
			if !strings.HasSuffix(key, "/") {
				key += "/"
			}
			ui := s3manager.UploadInput{
				Bucket: aws.String(u.Host), // Set your bucket name
				Key:    aws.String(key),
				Body:   emptyBuffer,
			}
			_, err = uploader.Upload(&ui)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return "", err
	}
	if s3url != nil {
		return *s3url, nil
	} else {
		directoryPath = strings.TrimPrefix(directoryPath, "/")
		key := strings.TrimPrefix(filepath.Join(u.Path, directoryPath), "/")
		// すべてのURLを結合して返す
		return "s3://" + u.Host + "/" + key, nil
	}

}

func downloadS3File(session *session.Session, s3URL string, dstPath *string) (string, error) {
	// URLを解析してバケットとキーを取得
	u, err := url.Parse(s3URL)
	if err != nil {
		return "", err
	}
	bucket := u.Host
	key := strings.TrimPrefix(u.Path, "/")
	// S3サービスクライアントを作成
	svc := s3.New(session)
	input := &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	// S3オブジェクトのメタデータを取得
	_, err = svc.HeadObject(input)
	if err != nil {
		key += "/"
		isdir, err := isS3Dir2(session, s3URL)
		if err != nil {
			return "", err
		}
		if isdir {
			if dstPath == nil {
				tmpPath, err := os.MkdirTemp("", "flowy-")
				if err != nil {
					return "", err
				}
				dstPath = &tmpPath
			}
			DownloadDirectory(svc, bucket, key, *dstPath)
			return *dstPath, nil
		} else {
			return "", errors.New("Unkown path " + s3URL)
		}
	}
	// S3オブジェクトを取得
	resp, err := svc.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var destFile *os.File
	// 一時ファイルを作成
	if dstPath == nil {
		destFile, err = ioutil.TempFile("", "flowy-")
		if err != nil {
			return "", err
		}
		defer destFile.Close()
	} else {
		ensureDirExists(*dstPath)
		destFile, err = os.Create(*dstPath)
		if err != nil {
			return "", err
		}
	}

	// S3オブジェクトの内容を一時ファイルに書き込み
	if _, err := io.Copy(destFile, resp.Body); err != nil {
		return "", err
	}

	return destFile.Name(), nil
}
func isS3Dir2(session *session.Session, s3url string) (bool, error) {
	u, err := url.Parse(s3url)
	if err != nil {
		return false, err
	}
	bucket := u.Host
	key := u.Path
	key = strings.TrimPrefix(key, "/")
	if strings.HasSuffix(key, "/") {
		key = key + "/"
	}

	// S3サービスクライアントを作成
	svc := s3.New(session)
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(key),
	}

	// Call S3 to list objects
	resp, err := svc.ListObjectsV2(input)
	if err != nil {
		return false, fmt.Errorf("failed to list objects, %v", err)
	}

	// Check if any objects are returned
	return len(resp.Contents) > 0, nil
}
