package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/bioflowy/flowy/golang/cmd/client/api"
	"github.com/urfave/cli/v2"
)

func exec_job(tool_path string, job_path *string) (int, interface{}) {
	cfg := api.NewConfiguration()
	cfg.Scheme = "http"
	cfg.Host = "127.0.0.1:5173"
	// cfg.HTTPClient = &http.Client{
	// 	Transport: &LoggingRoundTripper{Proxied: http.DefaultTransport},
	// }
	// cfg.Debug = true
	ctx := context.Background()
	c := api.NewAPIClient(cfg)
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error: %s", err.Error())
		os.Exit(1)
	}
	basedir := fmt.Sprintf("file://%s", cwd)
	r := api.ApiExecuteJobPostRequest{
		ToolPath:      tool_path,
		JobPath:       job_path,
		ClientWorkDir: cwd,
		Basedir:       &basedir,
	}
	res, _, err := c.DefaultAPI.ApiExecuteJobPost(ctx).ApiExecuteJobPostRequest(r).Execute()
	if err != nil {
		fmt.Printf("Error: %s", err.Error())
		return 1, nil
	}
	error, _ := res.GetErrorOk()
	if error != nil {
		fmt.Printf("Error: %s", *error)
		return 1, nil
	}
	jobId := res.GetJobId()
	for {
		res, _, err := c.DefaultAPI.ApiGetJobInfoPost(ctx).ApiGetJobInfoPostRequest(api.ApiGetJobInfoPostRequest{JobId: jobId}).Execute()
		if err != nil {
			log.Fatalf("JSON encoding error: %v", err)
			return 1, nil
		}
		if res.GetStatus() == "success" {
			return 0, res.GetResult()
		} else if res.GetStatus() == "permanentFail" || res.GetStatus() == "temporaryFail" {
			return 1, res.Result
		} else {
			time.Sleep(1 * time.Second)
		}
	}

}

type DataMap map[string]interface{}

func (o DataMap) GetClass() string {
	l, exists := o["class"]
	if !exists {
		return ""
	}
	class, ok := l.(string)
	if ok {
		return class
	} else {
		return ""
	}
}

type FileOrDirectory interface {
	GetClass() string
	IsFile() bool
	IsDirectory() bool
	HasLocation() bool
	GetLocation() string
	SetLocation(string)
	ClearPath()
	HasPath() bool
	GetPath() string
	SetPath(string)
	HasBasename() bool
	GetBasename() string
	SetBasename(string)
}
type Directory interface {
	GetClass() string
	IsFile() bool
	IsDirectory() bool
	HasLocation() bool
	GetLocation() string
	SetLocation(string)
	ClearPath()
	HasPath() bool
	GetPath() string
	SetPath(string)
	GetListing() []FileOrDirectory
	SetListing([]FileOrDirectory)
	HasBasename() bool
	GetBasename() string
	SetBasename(string)
}
type File interface {
	GetClass() string
	IsFile() bool
	IsDirectory() bool
	HasLocation() bool
	GetLocation() string
	SetLocation(string)
	HasPath() bool
	GetPath() string
	SetPath(string)
	ClearPath()
	HasBasename() bool
	GetBasename() string
	SetBasename(string)
	GetDirname() string
	SetDirname(string)
	GetNameroot() string
	SetNameroot(string)
	HasChecksum() bool
	GetChecksum() string
	SetChecksum(string)
	GetSize() int64
	SetSize(int64)
	GetContent() string
	SetContent(string)
	GetWritable() bool
	SetWritable(bool)
	GetSecondaryFiles() []FileOrDirectory
	SetSecondaryFiles([]FileOrDirectory)
}

func (o DataMap) IsFile() bool {
	return o.GetClass() == "File"
}
func (o DataMap) IsDirectory() bool {
	return o.GetClass() == "Directory"
}

// GetLocation returns the Location field value if set, zero value otherwise.
func (o DataMap) HasLocation() bool {
	_, exists := o["location"]
	return exists
}
func (o DataMap) GetLocation() string {
	l, exists := o["location"]
	if !exists {
		return ""
	}
	location, ok := l.(string)
	if ok {
		return location
	} else {
		return ""
	}
}
func (o DataMap) SetLocation(location string) {
	o["location"] = location
}

// GetLocation returns the Location field value if set, zero value otherwise.
func (o DataMap) HasPath() bool {
	_, exists := o["path"]
	return exists
}
func (o DataMap) GetPath() string {
	p, exists := o["path"]
	if !exists {
		return ""
	}
	value, ok := p.(string)
	if ok {
		return value
	} else {
		return ""
	}
}
func (o DataMap) ClearPath() {
	delete(o, "path")
}
func (o DataMap) ClearDirName() {
	delete(o, "dirname")
}
func (o DataMap) SetPath(path string) {
	o["path"] = path
}
func (o DataMap) HasBasename() bool {
	_, exists := o["basename"]
	return exists
}
func (o DataMap) GetBasename() string {
	v, exists := o["basename"]
	if !exists {
		return ""
	}
	value, ok := v.(string)
	if ok {
		return value
	} else {
		return ""
	}
}
func (o DataMap) SetBasename(path string) {
	o["basename"] = path
}
func (o DataMap) GetDirname() string {
	v, exists := o["dirname"]
	if !exists {
		return ""
	}
	value, ok := v.(string)
	if ok {
		return value
	} else {
		return ""
	}
}
func (o DataMap) SetDirname(path string) {
	o["dirname"] = path
}
func (o DataMap) GetNameroot() string {
	v, exists := o["nameroot"]
	if !exists {
		return ""
	}
	value, ok := v.(string)
	if ok {
		return value
	} else {
		return ""
	}
}
func (o DataMap) SetNameroot(path string) {
	o["nameroot"] = path
}
func (o DataMap) GetNameext() string {
	v, exists := o["nameext"]
	if !exists {
		return ""
	}
	value, ok := v.(string)
	if ok {
		return value
	} else {
		return ""
	}
}
func (o DataMap) SetNameext(path string) {
	o["nameext"] = path
}
func (o DataMap) HasChecksum() bool {
	_, exists := o["checksum"]
	return exists
}

func (o DataMap) GetChecksum() string {
	v, exists := o["checksum"]
	if !exists {
		return ""
	}
	value, ok := v.(string)
	if ok {
		return value
	} else {
		return ""
	}
}
func (o DataMap) SetChecksum(path string) {
	o["checksum"] = path
}
func (o DataMap) SetContent(path string) {
	o["contents"] = path
}

func VisitFileOrDirectory(arg interface{}, visitFunc func(FileOrDirectory) error) error {
	if arg == nil {
		return nil
	}
	callVisitFunc := func(dm DataMap) error {
		if dm.GetClass() == "File" || dm.GetClass() == "Directory" {
			return visitFunc(dm)
		} else {
			for _, value := range dm {
				err := VisitFileOrDirectory(value, visitFunc)
				if err != nil {
					return err
				}
			}
		}
		return nil
	}
	switch v := arg.(type) {
	case map[string]interface{}:
		var v2 DataMap = v
		err := callVisitFunc(v)
		if v2.IsFile() {
			secondaryFiles := v2.GetSecondaryFiles()
			for _, secondary := range secondaryFiles {
				err := callVisitFunc(secondary.(DataMap))
				if err != nil {
					return err
				}
			}
		} else if v2.IsDirectory() {
			lists := v2.GetListing()
			for _, list := range lists {
				err := callVisitFunc(list.(DataMap))
				if err != nil {
					return err
				}
			}
		}
		for _, val := range v {
			err := VisitFileOrDirectory(val, visitFunc)
			if err != nil {
				return err
			}
		}
		return err
	case []interface{}:
		for _, val := range v {
			err := VisitFileOrDirectory(val, visitFunc)
			if err != nil {
				return err
			}
		}
	case []FileOrDirectory:
		for _, val := range v {
			err := VisitFileOrDirectory(val, visitFunc)
			if err != nil {
				return err
			}
		}
	case []File:
		for _, val := range v {
			err := VisitFileOrDirectory(val, visitFunc)
			if err != nil {
				return err
			}
		}
	default:
	}
	valueType := reflect.TypeOf(arg)
	value := reflect.ValueOf(arg)
	if valueType.Kind() == reflect.Array || valueType.Kind() == reflect.Slice {
		for i := 0; i < value.Len(); i++ {
			err := VisitFileOrDirectory(value.Index(i), visitFunc)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
func uriToPath(uri string) string {
	if strings.HasPrefix(uri, "file:///") {
		return strings.TrimPrefix(uri, "file://")
	} else if strings.HasPrefix(uri, "file://") {
		return strings.TrimPrefix(uri, "file:/")
	}
	return uri
}
func (o DataMap) SetSecondaryFiles(secondaryFiles []FileOrDirectory) {
	o["secondaryFiles"] = secondaryFiles
}
func (o DataMap) GetSecondaryFiles() []FileOrDirectory {
	v, exists := o["secondaryFiles"]
	if !exists {
		return nil
	}
	value, ok := v.([]FileOrDirectory)
	if ok {
		return value
	} else {
		return nil
	}
}
func (o DataMap) GetListing() []FileOrDirectory {
	l, exists := o["listing"]
	if !exists {
		return []FileOrDirectory{}
	}
	class, ok := l.([]FileOrDirectory)
	if ok {
		return class
	} else {
		return []FileOrDirectory{}
	}
}

func main() {
	app := &cli.App{
		Name:  "cwl-executor",
		Usage: "execute cwl workflow",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "outdir",
				Aliases:  []string{"o"},
				Usage:    "Output directory",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "basedir",
				Aliases:  []string{"b"},
				Usage:    "base directory for input",
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "quiet",
				Aliases:  []string{"q"},
				Usage:    "suppress log output",
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "use_container",
				Usage:    "use container for execution",
				Value:    true,
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "export",
				Usage:    "export result files",
				Value:    true,
				Required: false,
			},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() < 1 {
				return cli.Exit("Error: tool_path is required", 1)
			}
			toolPath := c.Args().Get(0)
			var jobPath *string = nil
			if c.Args().Len() > 1 {
				jp := c.Args().Get(1)
				jobPath = &jp
			}
			ret, jsonData := exec_job(toolPath, jobPath)

			if ret != 0 {
				return cli.Exit("", ret)
			}
			if c.Bool("export") {
				cwd, err := os.Getwd()
				if err != nil {
					return cli.Exit(err, 1)
				}
				err = VisitFileOrDirectory(jsonData, func(f_or_d FileOrDirectory) error {
					if f_or_d.IsFile() {
						location := path.Join(cwd, f_or_d.GetBasename())
						err = os.Symlink(uriToPath(f_or_d.GetLocation()), location)
						if err != nil {
							return cli.Exit(err, 1)
						}
						f_or_d.SetLocation(location)
					} else if f_or_d.IsDirectory() {
						location := path.Join(cwd, f_or_d.GetBasename())
						err = os.Symlink(uriToPath(f_or_d.GetLocation()), location)
						if err != nil {
							return cli.Exit(err, 1)
						}
						f_or_d.SetLocation(location)
					}
					return nil
				})
				if err != nil {
					return cli.Exit(err, 1)
				}
			}
			jsonDataStr, err := json.Marshal(jsonData)
			if err != nil {
				log.Fatalf("JSON encoding error: %v", err)
			}

			fmt.Println(string(jsonDataStr))
			return cli.Exit("", 0)
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
