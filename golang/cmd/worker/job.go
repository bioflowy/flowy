package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/bioflowy/flowy/golang/cmd/worker/api"
)

type PreparedJob struct {
	Id               string
	Commands         []string
	Stdin            io.ReadCloser
	Stdout           io.WriteCloser
	Stderr           io.WriteCloser
	StderrBuffer     *BufferWriteCloser
	Env              map[string]string
	Cwd              string
	ContainerWorkDir string
	TmpDir           string
	RemoveTmpDir     bool
	OutputBaseDir    string
	OutputBindings   []api.OutputBinding
	Timelimit        *int32
	Fileitems        []api.MapperEnt
	InplaceUpdate    bool
}

func prepareOutput(config *api.SharedFileSystemConfig, fileManager FileManager, job *api.ApiGetExectableJobPost200ResponseInner, pipeMap map[string]*Pipe) error {
	err := os.MkdirAll(job.Cwd, 0770)
	if err != nil {
		return err
	}
	for _, out := range job.OutputBindings {
		if out.Streamable != nil && *out.Streamable {
			outputPath := filepath.Join(job.Cwd, out.Glob[0])
			if err := syscall.Mkfifo(outputPath, 0666); err != nil {
				return fmt.Errorf("failed to create named pipe %s: %v", outputPath, err)
			}
			pipe := InitPipe(outputPath)
			pipeMap[outputPath] = pipe

		}
	}
	return nil
}
func prepareForDocker(fileManager FileManager, job *api.ApiGetExectableJobPost200ResponseInner) ([]string, error) {
	var dockerCommands []string
	dockerCommands = append(dockerCommands, *job.DockerExec)
	dockerCommands = append(dockerCommands, "run", "-i")
	mountParams, err := stageForDocker(fileManager, job.Cwd, job.ContainerOutdir, append(job.Fileitems, job.Generatedlist...), job.InplaceUpdate)
	if err != nil {
		return nil, err
	}
	dockerCommands = append(dockerCommands, mountParams...)
	dockerCommands = append(dockerCommands, "--workdir="+job.ContainerOutdir)
	dockerCommands = append(dockerCommands, "--read-only=true")
	if job.StdoutPath != nil {
		dockerCommands = append(dockerCommands, "--log-driver=none")
	}
	if job.Networkaccess {
		if job.Runtime.CustomNet != nil {
			dockerCommands = append(dockerCommands, "--net="+*(job.Runtime.CustomNet))
		}
	} else {
		dockerCommands = append(dockerCommands, "--net=none")
	}
	dockerCommands = append(dockerCommands, "--rm")
	dockerCommands = append(dockerCommands, "--user=1001:1001")
	dockerCommands = append(dockerCommands, "--env=HOME="+job.ContainerOutdir)
	dockerCommands = append(dockerCommands, "--env=TMPDIR=/tmp")
	dockerCommands = append(dockerCommands, *job.DockerImage)
	return dockerCommands, nil
}
func prepareJob(config *api.SharedFileSystemConfig, fileManager FileManager, job *api.ApiGetExectableJobPost200ResponseInner, pipeMap map[string]*Pipe) (*PreparedJob, error) {
	var err error
	var commands2 []string
	if job.DockerImage != nil {
		commands2, err = prepareForDocker(fileManager, job)
		if err != nil {
			return nil, err
		}
	} else {
		err := stageForCommandLine(fileManager, append(job.Fileitems, job.Generatedlist...), job.InplaceUpdate, pipeMap)
		if err != nil {
			return nil, err
		}
	}
	for _, c := range job.Commands {
		commands2 = append(commands2, toString(c))
	}
	var stdin io.ReadCloser = nil
	var stdout io.WriteCloser = nil
	var stderr io.WriteCloser = nil
	if job.StdinPath != nil {
		stdinPath := *job.StdinPath
		if strings.HasPrefix(stdinPath, "s3://") {
			tmppath, err := downloadS3FileToTemp(config, stdinPath, nil)
			if err != nil {
				return nil, err

			}
			stdinPath = tmppath
		}
		stdin, err = os.Open(stdinPath)
		if err != nil {
			return nil, err
		}
	}
	var stderrb *BufferWriteCloser = nil
	if job.StdoutPath != nil {
		stdoutPath := *job.StdoutPath
		stdout, err = os.Create(stdoutPath)
		if err != nil {
			return nil, err
		}
	}
	if job.StderrPath != nil {
		stderr, err = os.Create(*job.StderrPath)
		if err != nil {
			return nil, err
		}
	} else {
		w := NewBufferWriteCloser()
		stderrb = w
		stderr = w

	}
	return &PreparedJob{
		Id:               job.Id,
		Commands:         commands2,
		Stdin:            stdin,
		Stdout:           stdout,
		Stderr:           stderr,
		StderrBuffer:     stderrb,
		Env:              job.Env,
		Cwd:              job.Cwd,
		ContainerWorkDir: job.ContainerOutdir,
		TmpDir:           job.TmpDir,
		RemoveTmpDir:     job.RemoveTmpDir,
		Timelimit:        job.Timelimit,
		OutputBaseDir:    *job.OutputBaseDir,
		OutputBindings:   job.OutputBindings,
		Fileitems:        job.Fileitems,
		InplaceUpdate:    job.InplaceUpdate,
	}, nil
}

func executeJob(job *PreparedJob) (int, error) {
	var err error = nil
	fmt.Println("start commands " + job.Commands[0])
	cmd := exec.Command(job.Commands[0], job.Commands[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = job.Stdin, job.Stdout, job.Stderr
	cmd.Dir = job.Cwd
	cmd.Env = os.Environ()
	for k, v := range job.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	if job.Timelimit != nil && *job.Timelimit > 0 {
		timer := time.AfterFunc(time.Duration(*job.Timelimit)*time.Second, func() {
			cmd.Process.Kill()
		})
		defer timer.Stop()
	}

	err = cmd.Start()
	if err != nil {
		return -1, err
	}

	err = cmd.Wait()
	if job.Stdout != nil {
		job.Stdout.Close()
	}
	if job.Stderr != nil {
		job.Stderr.Close()
	}
	if job.Stdin != nil {
		job.Stdin.Close()
	}
	if job.StderrBuffer != nil {
		fmt.Fprint(os.Stderr, job.StderrBuffer.String())
	}
	if err != nil {
		exitError, ok := err.(*exec.ExitError)
		if ok {
			return exitError.ExitCode(), nil
		}
		return -1, err
	}
	return cmd.ProcessState.ExitCode(), nil
}

func execAndUpload(c *api.APIClient, fileManager FileManager, config *api.SharedFileSystemConfig, job *PreparedJob) {
	ctx := context.Background()

	log.Default().Printf("job command = %+v", job.Commands)
	os.MkdirAll(job.Cwd, 0770)
	os.MkdirAll(job.TmpDir, 0770)
	exitCode, err := executeJob(job)
	if err != nil {
		reportFailed(c, job.Id, err)
		return
	}
	var downloadPaths = fileManager.GetDownloadFileMap()
	cwlOutputPath := filepath.Join(job.Cwd, "cwl.output.json")
	_, err = os.Stat(cwlOutputPath)
	if !os.IsNotExist(err) {
		results, err := loadCwlOutputJson(cwlOutputPath)
		if err != nil {
			reportFailed(c, job.Id, err)
			return
		}
		err = VisitFileOrDirectory(results, func(value FileOrDirectory) error {
			return RevmapFile(job.ContainerWorkDir, job.Cwd, value, job.Fileitems)
		})
		if err != nil {
			reportFailed(c, job.Id, err)
			return
		}

		err = uploadOutputs(fileManager, job.OutputBaseDir, results, downloadPaths, job.InplaceUpdate)
		if err != nil {
			reportFailed(c, job.Id, err)
			return
		}
		r := c.DefaultAPI.ApiJobFinishedPost(ctx).JobFinishedRequest(api.JobFinishedRequest{
			Id:          job.Id,
			IsCwlOutput: true,
			ExitCode:    int32(exitCode),
			Results:     results,
		})
		log.Default().Printf("job(%s) Finished exitCode = %d", job.Commands[0], exitCode)
		log.Default().Printf("job Finished results = %+v", results)

		r.Execute()
		// os.RemoveAll(job.Cwd)
		for _, localPath := range downloadPaths {
			os.RemoveAll(localPath)
		}
		fmt.Print(exitCode)

	} else {
		results := map[string]interface{}{}
		for _, output := range job.OutputBindings {
			files2, err := globOutput(
				job.ContainerWorkDir,
				output,
				job.Cwd,
				true,
			)
			if err != nil {
				reportFailed(c, job.Id, err)
				return
			}
			if len(files2) > 0 {
				results[output.Name] = files2
				for _, file := range files2 {
					err = collect_secondary_files(c, config, job.Id, output, file, job.Cwd, job.ContainerWorkDir, true, job.Fileitems)
					if err != nil {
						reportFailed(c, job.Id, err)
						return
					}
				}
			}
			outputEval, ok := output.GetOutputEvalOk()
			if ok {
				var exitCode32 int32 = int32(exitCode)
				ret, err := do_eval(c, job.Id, *outputEval, files2, &exitCode32)
				if err != nil {
					reportFailed(c, job.Id, err)
					return
				}
				results[output.Name] = ret
			}
		}
		err := VisitFileOrDirectory(results, func(f FileOrDirectory) error {
			return RevmapFile(job.ContainerWorkDir, job.Cwd, f, job.Fileitems)
		})
		if err != nil {
			reportFailed(c, job.Id, err)
			return
		}
		uploadOutputs(fileManager, job.OutputBaseDir, results, downloadPaths, job.InplaceUpdate)
		r := c.DefaultAPI.ApiJobFinishedPost(ctx).JobFinishedRequest(api.JobFinishedRequest{
			Id:          job.Id,
			IsCwlOutput: false,
			ExitCode:    int32(exitCode),
			Results:     results,
		})
		log.Default().Printf("job(%s) Finished exitCode = %d", job.Commands[0], exitCode)
		log.Default().Printf("job Finished results = %+v", results)

		r.Execute()
		// os.RemoveAll(job.Cwd)
		for _, localPath := range downloadPaths {
			os.RemoveAll(localPath)
		}
		if job.RemoveTmpDir {
			os.RemoveAll(job.TmpDir)
		}
		fmt.Print(exitCode)

	}
}
