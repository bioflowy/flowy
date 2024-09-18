package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/bioflowy/flowy/golang/cmd/cwlclient/api"
	"github.com/bioflowy/flowy/golang/internal"
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
	useContainer := true
	r := api.ApiExecuteJobPostRequest{
		ToolPath:      tool_path,
		JobPath:       job_path,
		ClientWorkDir: cwd,
		Basedir:       &basedir,
		UseContainer:  &useContainer,
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

func visitlisting(
	objs []map[string]interface{},
	stagedir string,
	basedir string,
	copy bool,
	staged bool,
	targets map[string]string,
) {
	for _, obj := range objs {
		visit(obj, stagedir, basedir, copy, staged, targets)
	}
}

// generateUniqueDestination generates a unique destination path for a given source.
// It ensures that each source has a unique destination in the targets map.
// If a collision occurs, it appends a counter to the destination until a unique path is found.
//
// Parameters:
//   - source: The original source path
//   - dist: The desired destination path
//   - targets: A map of existing destination-to-source mappings
//
// Returns:
//   - bool: True if copying is necessary, false otherwise
//   - string: The final (possibly modified) destination path
func generateUniqueDistination(source string, dist string, targets map[string]string) (bool, string) {
	prevSource, exists := targets[dist]
	if !exists {
		// If the destination is not already registered, copying is necessary
		targets[dist] = source
		return true, dist
	}
	if prevSource == source {
		// If the destination and source are the same, no copying is needed
		return false, dist
	}
	// If the destination is the same but the source is different, modify the destination

	counter := 1
	newDist := dist
	for {
		newDist = fmt.Sprintf("%s_%d", dist, counter)
		if _, exists := targets[newDist]; !exists {
			targets[newDist] = source
			// If the source is different, copying is necessary
			return true, newDist
		}
		counter++
	}
}

func visit(
	obj map[string]interface{},
	stagedir string,
	basedir string,
	copy bool,
	staged bool,
	targets map[string]string,
) error {
	fd := internal.DataMap(obj)
	sourcePath, err := internal.ResolvePath(uriToPath(fd.GetLocation()))
	if err != nil {
		return err
	}
	needCopy, tgt := generateUniqueDistination(sourcePath, path.Join(stagedir, fd.GetBasename()), targets)
	if fd.IsDirectory() {
		var dir internal.Directory = fd
		location := uriToPath(dir.GetLocation())

		if staged {
			internal.RemovePath(tgt)
			if strings.HasPrefix(location, "_:") {
				os.MkdirAll(tgt, 0755)
			} else {
				internal.CopyDir(location, tgt)
			}
		}
		dir.SetLocation(tgt)
		if strings.HasPrefix("file://", location) {
			//
			staged = false
		}
		dir.SetLocation(tgt)
		visitlisting(
			dir.GetListing(),
			tgt,
			basedir,
			copy,
			staged,
			targets,
		)
	} else if fd.IsFile() {
		var file internal.File = fd
		location := uriToPath(file.GetLocation())
		if staged {
			if strings.HasPrefix(location, "_:") {
				err := os.WriteFile(tgt, []byte(file.GetContent()), 0644)
				if err != nil {
					return err
				}
			} else if needCopy {
				internal.CopyFile(location, tgt)
			}
		}
		file.SetLocation(tgt)
		visitlisting(
			file.GetSecondaryFiles(),
			stagedir,
			basedir,
			copy,
			staged,
			targets,
		)
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

// 本来はworker側でDirectoryのlistingを取得するが、一時的にclient側でlistingを取得する
func get_listing(rec internal.Directory) {
	/// Expand, recursively, any 'listing' fields in a Directory."""
	if rec.GetListing() != nil {
		return
	}
	lists := []map[string]interface{}{}

	location := uriToPath(rec.GetLocation())
	files, err := os.ReadDir(location)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		if file.IsDir() {
			var file2 internal.DataMap = internal.DataMap{
				"class":    "Directory",
				"location": "file://" + path.Join(location, file.Name()),
				"basename": file.Name(),
			}
			get_listing(file2)
			lists = append(lists, file2)
		} else {
			var file2 internal.DataMap = internal.DataMap{
				"class":    "File",
				"location": "file://" + path.Join(location, file.Name()),
				"basename": file.Name(),
			}
			internal.ComputeChecksums(file2)
			lists = append(lists, file2)
		}
	}
	rec.SetListing(lists)
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
				f_or_ds := internal.CollectDirEntries(jsonData)
				targets := map[string]string{}
				for _, f_or_d := range f_or_ds {
					internal.VisitDirectory(f_or_d, true, func(d internal.Directory) error {
						get_listing(d)
						return nil
					})
					visit(f_or_d, cwd, cwd, true, true, targets)
				}
				err = internal.VisitFile(jsonData, true, func(f internal.File) error {
					return internal.ComputeChecksums(f)
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
