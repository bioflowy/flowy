// Package storage — S3Backend implements Backend using AWS SDK v2 with MinIO
// compatibility and multipart upload via transfermanager.
package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"

	"github.com/bioflowy/flowy/internal/config"
)

// S3APIClient is the subset of the AWS SDK s3.Client API used by S3Backend.
// Defining it as an interface enables mock injection in tests.
type S3APIClient interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
}

// TransferManagerUploader is the subset of the transfermanager.Client API used
// by S3Backend. Defining it as an interface enables mock injection in tests.
type TransferManagerUploader interface {
	UploadObject(ctx context.Context, input *transfermanager.UploadObjectInput, opts ...func(*transfermanager.Options)) (*transfermanager.UploadObjectOutput, error)
}

// S3Backend implements Backend using Amazon S3 (or MinIO) as the object store.
// All path helpers produce slash-separated S3 key prefixes; the local baseDir
// parameter is ignored because S3 uses bucket+prefix addressing.
type S3Backend struct {
	bucket    string
	prefix    string
	threshold int64
	s3c       S3APIClient
	tm        TransferManagerUploader
}

// NewS3Backend constructs an S3Backend from the project configuration.
// When cfg.Storage.Endpoint is non-empty, path-style addressing and a custom
// base endpoint are configured so the backend works with MinIO.
func NewS3Backend(cfg *config.Config) (*S3Backend, error) {
	sc := cfg.Storage

	region := sc.Region
	if region == "" {
		region = "us-east-1" // default required for MinIO compatibility
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("s3: load AWS config: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if sc.Endpoint != "" {
			o.BaseEndpoint = aws.String(sc.Endpoint)
			o.UsePathStyle = true // required for MinIO
		}
	})

	tm := transfermanager.New(s3Client, func(o *transfermanager.Options) {
		if sc.MultipartThreshold > 0 {
			o.MultipartUploadThreshold = sc.MultipartThreshold
		}
	})

	return &S3Backend{
		bucket:    sc.Bucket,
		prefix:    sc.Prefix,
		threshold: sc.MultipartThreshold,
		s3c:       s3Client,
		tm:        tm,
	}, nil
}

// newS3BackendForTest constructs an S3Backend with injected mock clients for
// unit testing without real AWS or MinIO infrastructure.
func newS3BackendForTest(bucket, prefix string, threshold int64, s3c S3APIClient, tm TransferManagerUploader) *S3Backend {
	return &S3Backend{
		bucket:    bucket,
		prefix:    prefix,
		threshold: threshold,
		s3c:       s3c,
		tm:        tm,
	}
}

// keyJoin builds an S3 object key by joining segments with "/" separators.
// Empty segments are filtered out. The backend prefix is prepended when set.
// filepath.Join is intentionally NOT used here — S3 keys must use "/" and must
// not be subject to OS-specific path normalization.
func (b *S3Backend) keyJoin(segments ...string) string {
	parts := make([]string, 0, len(segments)+1)
	if b.prefix != "" {
		parts = append(parts, strings.TrimRight(b.prefix, "/"))
	}
	for _, s := range segments {
		if s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "/")
}

// RunInputsDir returns the S3 key prefix for run inputs.
// baseDir is ignored — S3 uses bucket+prefix addressing.
func (b *S3Backend) RunInputsDir(baseDir, runID string) string {
	return b.keyJoin("runs", runID, "inputs")
}

// RunOutputsDir returns the S3 key prefix for run outputs.
// baseDir is ignored — S3 uses bucket+prefix addressing.
func (b *S3Backend) RunOutputsDir(baseDir, runID string) string {
	return b.keyJoin("runs", runID, "outputs")
}

// TaskDir returns the S3 key prefix for a specific task shard.
// The shard index is embedded in the key to prevent scatter collision (S3-05).
// baseDir is ignored — S3 uses bucket+prefix addressing.
func (b *S3Backend) TaskDir(baseDir, runID, taskName string, shardIndex int) string {
	return b.keyJoin("runs", runID, "tasks", fmt.Sprintf("%s_%d", taskName, shardIndex))
}

// CreateRunDirs is a no-op for S3Backend. S3 creates keys on PutObject;
// there is no directory concept to pre-create.
func (b *S3Backend) CreateRunDirs(baseDir, runID string) error {
	return nil
}

// CreateTaskDirs is a no-op for S3Backend. S3 creates keys on PutObject;
// there is no directory concept to pre-create.
func (b *S3Backend) CreateTaskDirs(baseDir, runID, taskName string, shardIndex int) error {
	return nil
}

// UploadFile uploads data to the S3 key identified by path. The transfermanager
// handles the threshold check internally, using PutObject for small payloads
// and multipart upload for large ones.
func (b *S3Backend) UploadFile(path string, data []byte) error {
	return b.UploadFileStream(path, bytes.NewReader(data))
}

// DownloadFile downloads the S3 object at key path and returns its contents.
// os.ErrNotExist is returned when the key does not exist (NoSuchKey).
func (b *S3Backend) DownloadFile(path string) ([]byte, error) {
	var buf bytes.Buffer
	if err := b.DownloadFileStream(path, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// UploadFileStream uploads data from r to the S3 key identified by path.
// The transfermanager selects PutObject or multipart upload automatically.
func (b *S3Backend) UploadFileStream(path string, r io.Reader) error {
	_, err := b.tm.UploadObject(context.Background(), &transfermanager.UploadObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(path),
		Body:   r,
	})
	if err != nil {
		return fmt.Errorf("s3: upload %s: %w", path, err)
	}
	return nil
}

// DownloadFileStream downloads the S3 object at key path and copies it to w.
// os.ErrNotExist is returned when the key does not exist (NoSuchKey).
func (b *S3Backend) DownloadFileStream(path string, w io.Writer) error {
	out, err := b.s3c.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return fmt.Errorf("s3: download %s: %w", path, os.ErrNotExist)
		}
		return fmt.Errorf("s3: download %s: %w", path, err)
	}
	defer out.Body.Close()
	if _, err := io.Copy(w, out.Body); err != nil {
		return fmt.Errorf("s3: read body %s: %w", path, err)
	}
	return nil
}

// UploadDirectory walks localSrc and uploads each file under the S3 prefix path.
func (b *S3Backend) UploadDirectory(path string, localSrc string) error {
	return filepath.WalkDir(localSrc, func(filePath string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(localSrc, filePath)
		if err != nil {
			return fmt.Errorf("s3: compute relative path: %w", err)
		}
		// Convert OS-specific separators to S3 key separators.
		keyRel := filepath.ToSlash(rel)
		key := path + "/" + keyRel

		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("s3: read local file %q: %w", filePath, err)
		}
		return b.UploadFile(key, data)
	})
}

// DownloadDirectory downloads all S3 objects under prefix path into localDst.
// Pagination is handled automatically via ContinuationToken.
func (b *S3Backend) DownloadDirectory(path string, localDst string) error {
	prefix := path + "/"
	var continuationToken *string

	for {
		out, err := b.s3c.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{
			Bucket:            aws.String(b.bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return fmt.Errorf("s3: list objects under %q: %w", prefix, err)
		}

		for _, obj := range out.Contents {
			if obj.Key == nil {
				continue
			}
			key := *obj.Key

			// Strip the prefix to get relative path under localDst.
			rel := strings.TrimPrefix(key, prefix)
			if rel == "" {
				continue
			}
			// Convert S3 key separator to OS path separator.
			localPath := filepath.Join(localDst, filepath.FromSlash(rel))

			if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
				return fmt.Errorf("s3: create dir for %q: %w", localPath, err)
			}
			data, err := b.DownloadFile(key)
			if err != nil {
				return fmt.Errorf("s3: download object %q: %w", key, err)
			}
			if err := os.WriteFile(localPath, data, 0o644); err != nil {
				return fmt.Errorf("s3: write local file %q: %w", localPath, err)
			}
		}

		if out.IsTruncated == nil || !*out.IsTruncated {
			break
		}
		continuationToken = out.NextContinuationToken
	}
	return nil
}

// Stat returns the size in bytes of the S3 object at the given key.
// Uses HeadObject internally -- does not download the object body.
func (b *S3Backend) Stat(path string) (int64, error) {
	out, err := b.s3c.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return 0, fmt.Errorf("s3: stat %s: %w", path, err)
	}
	if out.ContentLength == nil {
		return 0, nil
	}
	return *out.ContentLength, nil
}

// WriteJSON marshals v to JSON and uploads it to the S3 key identified by path.
func (b *S3Backend) WriteJSON(path string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("s3: marshal JSON for %q: %w", path, err)
	}
	return b.UploadFile(path, data)
}

// ReadJSON downloads the S3 object at key path and unmarshals its contents into v.
func (b *S3Backend) ReadJSON(path string, v interface{}) error {
	data, err := b.DownloadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("s3: unmarshal JSON from %q: %w", path, err)
	}
	return nil
}
