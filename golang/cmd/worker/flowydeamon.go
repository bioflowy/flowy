package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/bioflowy/flowy/golang/cmd/worker/api"
	"github.com/bioflowy/flowy/golang/internal"
)

type FileSystemEntity interface {
	GetLocation() string
	GetPath() string
	SetLocation(string)
	SetPath(string)
	GetBasename() string
	SetBasename(string)
}

type LoggingRoundTripper struct {
	Proxied http.RoundTripper
}

func (lrt *LoggingRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	log.Printf("Request URL: %s\n", r.URL.String())
	return lrt.Proxied.RoundTrip(r)
}
func collect_secondary_files(
	c *api.APIClient,
	config *api.SharedFileSystemConfig,
	id string,
	schema api.OutputBinding,
	result map[string]interface{},
	outdir string,
	builderOutDir string,
	computeCheckSum bool,
	fileitems []api.MapperEnt,
) error {
	if !internal.IsFile(result) {
		return nil
	}
	primary := internal.DataMap(result)
	fullPath := primary.GetPath()
	sepIndex := strings.LastIndex(fullPath, string(filepath.Separator))
	var pathprefix string
	if sepIndex != -1 {
		pathprefix = fullPath[:sepIndex+1]
	} else {
		pathprefix = fullPath
	}
	for _, sf := range schema.SecondaryFiles {
		var sf_required = false
		if sf.RequiredString != nil {
			sf_required_eval, err := do_eval(c, id, *sf.RequiredString, primary, nil)
			if err != nil {
				return err
			}
			required_bool, ok := sf_required_eval.(bool)
			if !ok {
				return errors.New(
					`expressions in the field 'required' must evaluate to a Boolean (true or false) or None. Got ${str(
				  sf_required_eval,
				)} for ${sf.requiredString}`,
				)
			}
			sf_required = required_bool
		} else if sf.RequiredBoolean != nil {
			sf_required = *sf.RequiredBoolean
		}
		var sfpath interface{}
		if strings.Contains(sf.Pattern, "$(") || strings.Contains(sf.Pattern, "${") {
			var err error
			sfpath, err = do_eval(c, id, sf.Pattern, primary, nil)
			if err != nil {
				return err
			}
		} else {
			sfpath = substitute(primary.GetBasename(), sf.Pattern)
		}

		for _, sfitem := range aslist(sfpath) {
			if sfitem == nil {
				continue
			}
			var secondaryFile internal.DataMap = map[string]interface{}{}
			switch sfitem2 := sfitem.(type) {
			case string:
				secondaryFile.SetPath(pathprefix + sfitem2)
			case map[string]interface{}:
				secondaryFile = sfitem2
			}
			if secondaryFile.HasPath() && !secondaryFile.HasLocation() {
				RevmapFile(builderOutDir, outdir, secondaryFile, fileitems)
			}
			if isFile(config, outdir, secondaryFile.GetLocation()) {
				secondaryFile.SetClass("File")
				if computeCheckSum {
					err := internal.ComputeChecksums(secondaryFile)
					if err != nil {
						return err
					}
				}
				secondaryFile.ClearDirName()
			} else if isDir(config, outdir, secondaryFile.GetLocation()) {
				secondaryFile.SetClass("Directory")
				secondaryFile.ClearDirName()
			}
			sf := append(primary.GetSecondaryFiles(), secondaryFile)
			primary.SetSecondaryFiles(sf)
		}
		if sf_required {
			if len(primary.GetSecondaryFiles()) == 0 {
				return errors.New("Missing required secondary file for output " + primary.GetLocation())
			}
		}
	}
	return nil
}
func abspath(src string, basedir string) (string, error) {
	var abpath string

	u, err := url.Parse(src)
	if err != nil {
		return "", err
	}

	if strings.HasPrefix(src, "file://") {
		abpath, err = uriFilePath(src)
		if err != nil {
			return "", err
		}
	} else if u.Scheme == "http" || u.Scheme == "https" {
		return src, nil
	} else {
		if strings.HasPrefix(basedir, "file://") {
			if filepath.IsAbs(src) {
				abpath = src
			} else {
				abpath = basedir + "/" + src
			}
		} else {
			if filepath.IsAbs(src) {
				abpath = src
			} else {
				abpath = filepath.Join(basedir, src)
			}
		}
	}

	return abpath, nil
}
func reportFailed(c *api.APIClient, jobId string, err error) {
	ctx := context.Background()
	r := c.DefaultAPI.ApiJobFailedPost(ctx).ApiJobFailedPostRequest(api.ApiJobFailedPostRequest{
		Id:       jobId,
		ErrorMsg: err.Error(),
	})
	r.Execute()
}
func loadCwlOutputJson(jsonPath string) (map[string]interface{}, error) {
	file, err := os.Open(jsonPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// デコード先のmapを準備
	var data map[string]interface{} // もしくは、適切なデータ構造を使用

	// JSONファイルをデコード
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&data)
	return data, err
}
func GetAndExecuteJob(c *api.APIClient, fileManager FileManager, config *api.SharedFileSystemConfig) {
	ctx := context.Background()
	res, httpres, err := c.DefaultAPI.ApiGetExectableJobPost(ctx).Execute()

	if err != nil {
		return
	}
	if httpres.StatusCode == 200 {
		pipeMap := map[string]*Pipe{}
		if len(res) == 0 {
			return
		}
		for _, job := range res {
			err := prepareOutput(config, fileManager, &job, pipeMap)
			if err != nil {
				reportFailed(c, job.Id, err)
				return
			}
		}
		var jobs []*PreparedJob
		for _, job := range res {
			pjob, err := prepareJob(config, fileManager, &job, pipeMap)
			if err != nil {
				reportFailed(c, job.Id, err)
				return
			}
			jobs = append(jobs, pjob)
		}
		for _, pipe := range pipeMap {
			go func(pipe *Pipe) {
				pipe.Run()
			}(pipe)
		}
		for _, job := range jobs {
			go execAndUpload(c, fileManager, config, job)
		}
	}
}
func uploadOutputs(fileManager FileManager, outputBaseDir string, results map[string]interface{}, downloadPaths map[string]string, inplaceUpdate bool) error {
	if !strings.HasPrefix(outputBaseDir, "s3://") {
		return nil
	}
	err := internal.VisitFileOrDirectory(results, true, func(f_or_d internal.FileOrDirectory) error {
		if f_or_d.IsFile() {
			file := f_or_d.(internal.File)
			var s3url *string = nil
			if inplaceUpdate {
				p, err := uriFilePath(file.GetLocation())
				if err != nil {
					return err
				}
				for s3path, localPath := range downloadPaths {
					if localPath == p {
						s3url = &s3path
					}
				}
			}
			path, err := uploadToS3(fileManager, file.GetLocation(), s3url)
			if err != nil {
				return err
			}
			file.SetLocation(path)
			file.ClearPath()
			// for _, secondaryFile := range file.GetSecondaryFiles() {
			// 	if secondaryFile.IsFile() {
			// 		if !strings.HasPrefix(secondaryFile.GetLocation(), "s3://") {
			// 			path, err := uploadToS3(config, secondaryFile.GetLocation(), nil)
			// 			if err != nil {
			// 				return err
			// 			}
			// 			secondaryFile.SetLocation(path)
			// 			secondaryFile.ClearPath()
			// 		}
			// 	} else if secondaryFile.IsDirectory() {
			// 		path, err := uploadToS3(config, secondaryFile.GetLocation(), nil)
			// 		if err != nil {
			// 			return err
			// 		}
			// 		secondaryFile.SetLocation(path)
			// 		secondaryFile.ClearPath()
			// 	}
			// }
		} else if f_or_d.IsDirectory() {
			directory := f_or_d.(internal.Directory)
			var s3url *string = nil
			if inplaceUpdate {
				p, err := uriFilePath(directory.GetLocation())
				if err != nil {
					return err
				}
				for s3path, localPath := range downloadPaths {
					if localPath == p {
						s3url = &s3path
					}
				}
			}
			path, err := uploadToS3(fileManager, directory.GetLocation(), s3url)
			if err != nil {
				return err
			}
			directory.SetLocation(path)
			directory.ClearPath()
		}
		return nil
	})
	return err
}
func uploadToS3(fileManager FileManager, filePath string, s3url *string) (string, error) {
	if strings.HasPrefix(filePath, "s3://") {
		return filePath, nil
	}
	if fileManager.GetType() != "s3" {
		return "file:/" + filePath, nil
	}
	filePath, err := fileUriToPath(filePath)
	if err != nil {
		return "", err
	}
	s3fileManager := fileManager.(*S3FileManager)
	uploader := s3manager.NewUploader(s3fileManager.session)
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return "", err
	}
	if fileInfo.IsDir() {
		return uploadDirectory(uploader, s3fileManager, filePath, s3url)
	} else {
		return uploadFile(uploader, s3fileManager, filePath, s3url)
	}
}
func reportWorkerStarted(c *api.APIClient) (*api.SharedFileSystemConfig, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	var sysinfo syscall.Sysinfo_t
	err = syscall.Sysinfo(&sysinfo)
	if err != nil {
		return nil, err
	}
	req := c.DefaultAPI.ApiWorkerStartedPost(context.Background()).ApiWorkerStartedPostRequest(api.ApiWorkerStartedPostRequest{
		Hostname: hostname,
		Cpu:      int32(runtime.NumCPU()),
		Memory:   int32(sysinfo.Totalram / 1024 / 1024),
	})
	res, _, err := req.Execute()
	return res, err
}

func main() {
	cfg := api.NewConfiguration()
	cfg.Scheme = "http"
	cfg.Host = "127.0.0.1:5173"
	// cfg.HTTPClient = &http.Client{
	// 	Transport: &LoggingRoundTripper{Proxied: http.DefaultTransport},
	// }
	// cfg.Debug = true
	c := api.NewAPIClient(cfg)
	var err error = nil
	var config *api.SharedFileSystemConfig = nil
	for {
		config, err = reportWorkerStarted(c)
		if err != nil {
			time.Sleep(time.Second)
		} else {
			break
		}
	}
	fileNamager, err := NewFileManager(config)
	if err != nil {
		panic(err)
	}
	for {
		GetAndExecuteJob(c, fileNamager, config)
		time.Sleep(1 * time.Second)
	}
}
func do_eval(c *api.APIClient, id string, expression string, primary interface{}, exitCode *int32) (interface{}, error) {
	req := c.DefaultAPI.ApiDoEvalPost(context.Background()).ApiDoEvalPostRequest(api.ApiDoEvalPostRequest{
		Id:       id,
		Ex:       expression,
		Context:  &primary,
		ExitCode: exitCode,
	})
	res, httpres, err := req.Execute()
	if httpres != nil && httpres.StatusCode != 200 {
		return nil, errors.New("do_eval api returning http code " + httpres.Status)
	}
	if err != nil {
		return nil, err
	}
	return res, nil
}
func aslist(val interface{}) []interface{} {
	if val == nil {
		return []interface{}{}
	}
	if reflect.TypeOf(val).Kind() != reflect.Slice {
		return []interface{}{val}
	}
	return val.([]interface{})
}
func isFile(config *api.SharedFileSystemConfig, dir string, filepath string) bool {
	if config != nil && strings.HasPrefix(filepath, "s3://") {
		h, err := headS3Object(config, filepath)
		if err != nil {
			return false
		} else {
			return h == "file"
		}
	}
	path, err := abspath(filepath, dir)
	if err != nil {
		return false
	}
	fileInfo, err := os.Stat(path)
	return err == nil && !fileInfo.IsDir()
}
func isS3Dir(config *api.SharedFileSystemConfig, s3url string) (bool, error) {
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
	var region string = "ap-northeast-1"
	// AWSセッションを作成
	sess, err := session.NewSession(&aws.Config{
		Region:           &region, // 適切なリージョンに変更してください
		Credentials:      credentials.NewStaticCredentials(*config.AccessKey, *config.SecretKey, ""),
		Endpoint:         aws.String(*config.Endpoint),
		S3ForcePathStyle: aws.Bool(true),
	})
	if err != nil {
		return false, err
	}

	// S3サービスクライアントを作成
	svc := s3.New(sess)
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
func isDir(config *api.SharedFileSystemConfig, dir string, filepath string) bool {
	if config != nil && strings.HasPrefix(filepath, "s3://") {
		h, err := isS3Dir(config, filepath)
		if err != nil {
			return false
		} else {
			return h
		}
	}
	path, err := abspath(filepath, dir)
	if err != nil {
		return false
	}
	fileInfo, err := os.Stat(path)
	return err == nil && fileInfo.IsDir()
}

// func convertToFile(data DataMap) *ChildFile {
// 	file := ChildFile{
// 		Class:    "File",
// 		Location: data.GetStringPtr("location"),
// 		Basename: data.GetStringPtr("basename"),
// 		Nameroot: data.GetStringPtr("nameroot"),
// 		Nameext:  data.GetStringPtr("nameext"),
// 		Checksum: data.GetStringPtr("checksum"),
// 		Size:     data.GetFloat32Ptr("size"),
// 		Format:   data.GetStringPtr("format"),
// 		Contents: data.GetStringPtr("contents"),
// 	}
// 	return &file
// }
// func convertToDirectory(data DataMap) *ChildDirectory {
// 	file := ChildDirectory{
// 		Class:    "Directory",
// 		Location: data.GetStringPtr("location"),
// 		Basename: data.GetStringPtr("basename"),
// 	}
// 	return &file
// }

// substitute -If string begins with one or more caret ^ characters, for each caret, remove the last file extension from the path
// (the last period . and all following characters). If there are no file extensions, the path is unchanged.
// Append the remainder of the string to the end of the file path.
func substitute(value string, replace string) string {
	if strings.HasPrefix(replace, "^") {
		lastDotIndex := strings.LastIndex(value, ".")
		if lastDotIndex != -1 {
			return substitute(value[:lastDotIndex], replace[1:])
		} else {
			return value + strings.TrimLeft(replace, "^")
		}
	}
	return value + replace
}
func ensureWritable(targetPath string, includeRoot bool) error {
	addWritableFlag := func(p string) error {
		stat, err := os.Stat(p)
		if err != nil {
			return err
		}
		mode := stat.Mode()
		newMode := mode | 0200 // Adding write permission for the owner
		return os.Chmod(p, newMode)
	}

	stat, err := os.Stat(targetPath)
	if err != nil {
		return err
	}

	if stat.IsDir() {
		if includeRoot {
			err := addWritableFlag(targetPath)
			if err != nil {
				return err
			}
		}

		items, err := os.ReadDir(targetPath)
		if err != nil {
			return err
		}

		for _, item := range items {
			itemPath := filepath.Join(targetPath, item.Name())
			if item.IsDir() {
				err := ensureWritable(itemPath, true) // Recursive call for directories
				if err != nil {
					return err
				}
			} else {
				err := addWritableFlag(itemPath) // Directly add flag for files
				if err != nil {
					return err
				}
			}
		}
	} else {
		err := addWritableFlag(targetPath)
		if err != nil {
			return err
		}
	}
	return nil
}

func removeIgnorePermissionError(filePath string) error {
	err := os.RemoveAll(filePath)
	if err != nil {
		if os.IsPermission(err) {
			// Log the permission error
			// Replace with your logger if necessary
			log.Printf("Permission denied when trying to remove outdir %s\n", filePath)
		} else {
			return err
		}
	}
	return nil
}

func uriFilePath(inputUrl string) (string, error) {
	u, err := url.Parse(inputUrl)
	if err != nil {
		return "", err
	}
	if u.Scheme != "file" {
		return "", fmt.Errorf("not a file URI: %s", inputUrl)
	}
	return filepath.FromSlash(u.Path), nil
}
func pathToFileURL(inputPath string) (string, error) {
	absPath, err := filepath.Abs(inputPath)
	if err != nil {
		return "", err
	}
	u := url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(absPath),
	}
	return u.String(), nil
}
func fileUri(inputPath string, splitFrag bool) (string, error) {
	if strings.HasPrefix(inputPath, "file://") {
		return inputPath, nil
	}

	var frag string
	var pathWithoutFrag string

	if splitFrag {
		pathSplit := strings.SplitN(inputPath, "#", 2)
		pathWithoutFrag = pathSplit[0]
		if len(pathSplit) == 2 {
			frag = "#" + url.QueryEscape(pathSplit[1])
		}
	} else {
		pathWithoutFrag = inputPath
	}

	absPath, err := filepath.Abs(pathWithoutFrag)
	if err != nil {
		return "", err
	}

	urlPath := url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(absPath),
	}

	uri := urlPath.String()
	if strings.HasPrefix(uri, "/") {
		return "file:" + uri + frag, nil
	}
	return uri + frag, nil
}
func Join(paths ...string) string {
	count := len(paths) - 1
	for ; count > 0; count-- {
		if strings.HasPrefix(paths[count], "/") {
			break
		}
	}
	return strings.Join(paths[count:], "/")
}
func FileUrlJoin(baseurl string, path string) string {
	if strings.HasSuffix(baseurl, "/") {
		return baseurl + path
	} else {
		return baseurl + "/" + path

	}
}
func RevmapFile(builderOutdir, outdir string, f internal.FileOrDirectory, fileitems []api.MapperEnt) error {
	if strings.HasPrefix(outdir, "/") {
		outdir, _ = fileUri(outdir, false)
	}
	if f.HasLocation() && !f.HasPath() {
		location := f.GetLocation()
		if strings.HasPrefix(location, "file://") {
			path, err := uriFilePath(location)
			if err != nil {
				return err
			}
			f.SetPath(path)
		} else {
			f.SetLocation(FileUrlJoin(outdir, location))
			return nil
		}
	}
	if f.HasPath() {
		path1 := Join(builderOutdir, f.GetPath())
		uripath, _ := fileUri(path1, false)
		f.ClearPath()
		if !f.HasBasename() {
			f.SetBasename(filepath.Base(path1))
		}
		for _, mapent := range fileitems {
			if path1 == mapent.Target && !strings.HasPrefix(mapent.Type, "Writable") {
				f.SetLocation(mapent.Resolved)
				return nil
			}
		}
		if uripath == outdir || strings.HasPrefix(uripath, outdir+string(filepath.Separator)) || strings.HasPrefix(uripath, outdir+"/") {
			f.SetLocation(uripath)
		} else if path1 == builderOutdir || strings.HasPrefix(path1, builderOutdir+string(filepath.Separator)) || strings.HasPrefix(path1, builderOutdir+"/") {
			path2 := strings.Join(strings.Split(path1[len(builderOutdir)+1:], string(filepath.Separator)), "/")
			f.SetLocation(Join(outdir, path2))
		} else {
			return errors.New("output file path must be within designated output directory or an input file pass through")
		}
		return nil
	}
	return errors.New("output File object is missing both 'location' and 'path' fields")
}
func splitext(path string) (root, ext string) {
	ext = filepath.Ext(path)
	root = path[:len(path)-len(ext)]
	return
}

// convertToFileOrDirectory determines if the given path is a file or directory and returns the corresponding struct.
func convertToFileOrDirectory(builderOutdir, prefix, path1 string) (map[string]interface{}, error) {
	stat, err := os.Stat(path1)
	if err != nil {
		return nil, err
	}

	relPath, err := filepath.Rel(prefix, path1)
	if err != nil {
		return nil, err
	}
	if stat.Mode().IsDir() {
		path2 := filepath.Join(builderOutdir, filepath.FromSlash(relPath))
		directory := internal.NewDirectory(path1, &path2)
		return directory, nil
	} else {
		if stat.Mode()&os.ModeNamedPipe != 0 {
			//remove named pile and create dummy file
			err := os.Remove(path1)
			if err != nil {
				return nil, err
			}
			f, err := os.Create(path1)
			if err != nil {
				return nil, err
			}
			f.Close()
		}
		path2 := filepath.Join(builderOutdir, filepath.FromSlash(relPath))
		file := internal.NewFile(path1, &path2)
		return file, nil
	}
}

func listdir(dir, fn string) ([]string, error) {
	absPath, err := abspath(fn, dir)
	if err != nil {
		return nil, err
	}
	// ディレクトリ内のエントリを取得
	entries, err := ioutil.ReadDir(absPath)
	if err != nil {
		return nil, err
	}

	// 各エントリのURIを作成
	var uris []string
	for _, entry := range entries {
		entryPath := filepath.Join(absPath, entry.Name())
		uri := "file://" + entryPath
		if strings.HasPrefix(fn, "file://") {
			if strings.HasSuffix(fn, "/") {
				uri = fn + entry.Name()
			} else {
				uri = fn + "/" + entry.Name()
			}
		}
		uris = append(uris, uri)
	}

	return uris, nil
}
func get_listing(outdir string, dir internal.Directory, recursive bool) error {
	var listing = []map[string]interface{}{}
	ls, err := listdir(outdir, dir.GetLocation())
	if err != nil {
		return err
	}
	for _, ld := range ls {
		fileUri(ld, false)
		if isDir(nil, outdir, ld) {
			ent := internal.NewDirectory(ld, nil)
			// if (recursive) {
			// get_listing(fs_access, ent, recursive);
			// }
			var m map[string]interface{} = ent
			listing = append(listing, m)
		} else {
			ent := internal.NewFile(ld, nil)
			listing = append(listing, ent)
		}
	}
	dir.SetListing(listing)
	return nil
}
func globOutput(builderOutdir string, binding api.OutputBinding, outdir string, computeChecksum bool) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	// Example of globbing in Go
	for _, glob := range binding.Glob {
		globPath := Join(outdir, glob)
		if strings.HasPrefix(globPath, outdir) {
		} else if globPath == "." {
			globPath = outdir
		} else if strings.HasPrefix(globPath, "/") {
			return results, errors.New("glob patterns must not start with '/'")
		}
		matches, err := filepath.Glob(globPath) // This needs to be adapted to your specific logic
		if err != nil {
			return results, err
		}

		for _, match := range matches {
			f, err := convertToFileOrDirectory(builderOutdir, outdir, match)
			if err != nil {
				return results, err
			}
			if internal.IsFile(f) {
				file := internal.DataMap(f)
				if binding.LoadContents != nil && *binding.LoadContents {
					content, _ := contentLimitRespectedReadBytes(file.GetLocation())
					file.SetContent(content)
				}

				if computeChecksum {
					internal.ComputeChecksums(file)
				}
				var m map[string]interface{} = file
				results = append(results, m)
			} else if internal.IsDirectory(f) {
				d := internal.DataMap(f)
				if binding.LoadListing != nil && *binding.LoadListing != api.NO_LISTING {
					get_listing(outdir, d, *binding.LoadListing == api.DEEP_LISTING)
				}
				results = append(results, f)
			} else if err != nil {
				return results, err
			}
		}

	}
	return results, nil
}

const CONTENT_LIMIT = 64 * 1024 // Set your content limit here

// Helper functions like ensureWritable, copyFile, isSymlink, and removeIgnorePermissionError need to be implemented.
func contentLimitRespectedReadBytes(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	buffer := make([]byte, CONTENT_LIMIT+1)
	bytesRead, err := file.Read(buffer)
	if err != nil {
		return "", err
	}

	if bytesRead > CONTENT_LIMIT {
		return "", fmt.Errorf("file is too large, loadContents limited to %d bytes", CONTENT_LIMIT)
	}

	return string(buffer[:bytesRead]), nil
}
func downloadFile(svc *s3.S3, bucket, key, filePath string) error {
	// Ensure the local directory structure exists
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("error creating directory: %v", err)
	}

	// Create a file to write the download to
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("error creating file: %v", err)
	}
	defer file.Close()

	// Download the file
	objInput := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}
	output, err := svc.GetObject(objInput)
	if err != nil {
		return fmt.Errorf("error getting object: %v", err)
	}
	defer output.Body.Close()

	if _, err = io.Copy(file, output.Body); err != nil {
		return fmt.Errorf("error downloading object: %v", err)
	}

	return nil
}

func DownloadDirectory(svc *s3.S3, bucket, prefix, localDir string) error {
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	}

	// List objects
	err := svc.ListObjectsV2Pages(input, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, obj := range page.Contents {
			// Create file path based on object key
			filePath := filepath.Join(localDir, strings.TrimPrefix(*obj.Key, prefix))
			if strings.HasSuffix(*obj.Key, "/") {
				_, err := os.Stat(filePath)
				if !os.IsExist(err) {
					os.MkdirAll(filePath, 0775)
				}
				continue
			}
			if err := downloadFile(svc, bucket, *obj.Key, filePath); err != nil {
				fmt.Printf("Failed to download file: %s, error: %v\n", *obj.Key, err)
			} else {
				fmt.Printf("File downloaded: %s\n", filePath)
			}
		}
		return !lastPage
	})

	if err != nil {
		return fmt.Errorf("error listing objects: %v", err)
	}

	return nil
}
func headS3Object(config *api.SharedFileSystemConfig, s3URL string) (string, error) {
	// URLを解析してバケットとキーを取得
	u, err := url.Parse(s3URL)
	if err != nil {
		return "", err
	}
	bucket := u.Host
	key := strings.TrimPrefix(u.Path, "/")
	var region string = "ap-northeast-1"
	// AWSセッションを作成
	sess, err := session.NewSession(&aws.Config{
		Region:           &region, // 適切なリージョンに変更してください
		Credentials:      credentials.NewStaticCredentials(*config.AccessKey, *config.SecretKey, ""),
		Endpoint:         aws.String(*config.Endpoint),
		S3ForcePathStyle: aws.Bool(true),
	})
	if err != nil {
		return "", err
	}

	// S3サービスクライアントを作成
	svc := s3.New(sess)
	input := &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	// S3オブジェクトのメタデータを取得
	_, err = svc.HeadObject(input)
	if err == nil {
		return "file", nil
	}
	key += "/"
	input = &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}
	_, err = svc.HeadObject(input)
	if err == nil {
		return "directory", err
	}
	return "", err
}
func downloadS3FileToTemp(config *api.SharedFileSystemConfig, s3URL string, dstPath *string) (string, error) {
	// URLを解析してバケットとキーを取得
	u, err := url.Parse(s3URL)
	if err != nil {
		return "", err
	}
	bucket := u.Host
	key := strings.TrimPrefix(u.Path, "/")
	var region string = "ap-northeast-1"
	// AWSセッションを作成
	sess, err := session.NewSession(&aws.Config{
		Region:           &region, // 適切なリージョンに変更してください
		Credentials:      credentials.NewStaticCredentials(*config.AccessKey, *config.SecretKey, ""),
		Endpoint:         aws.String(*config.Endpoint),
		S3ForcePathStyle: aws.Bool(true),
	})
	if err != nil {
		return "", err
	}

	// S3サービスクライアントを作成
	svc := s3.New(sess)
	input := &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	// S3オブジェクトのメタデータを取得
	_, err = svc.HeadObject(input)
	if err != nil {
		key += "/"
		isdir, err := isS3Dir(config, s3URL)
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
func toString(commandStr []api.CommandStringInner) string {
	var str = ""
	for _, p := range commandStr {
		if p.Type != "Literal" {
			str = str + p.Value
		} else if p.Type != "Key" {
			// TODO
			str = str + p.Value
		}
	}
	return str
}
