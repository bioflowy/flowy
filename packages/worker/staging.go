package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/bioflowy/flowy-cwl3/flowydeamon/api"
)

func stageForCommandLine(fileManager FileManager, items []api.MapperEnt, inplaceUpdate bool, pipeMap map[string]*Pipe) error {
	var targets = []string{}
	for _, item := range items {
		if !item.Staged {
			continue
		}
		if item.Type == "WritableFile" {
			if inplaceUpdate {
				if fileManager.NeedDownload(item.Resolved) {
					_, err := fileManager.Download(item.Resolved, item.Target)
					if err != nil {
						return err
					}
				} else {
					err := symlink(item.Resolved, item.Target, true)
					if err != nil {
						return err
					}
				}
			} else {
				if fileManager.NeedDownload(item.Resolved) {
					_, err := fileManager.Download(item.Resolved, item.Target)
					if err != nil {
						return err
					}
				} else {
					err := symlink(item.Resolved, item.Target, true)
					if err != nil {
						return err
					}
				}
			}
		} else if item.Type == "WritableDirectory" {
			if strings.HasPrefix(item.Resolved, "_:") {
				os.Mkdir(item.Target, 0755)
			} else if inplaceUpdate {
				if fileManager.NeedDownload(item.Resolved) {
					_, err := fileManager.Download(item.Resolved, item.Target)
					if err != nil {
						return err
					}
				} else {
					err := symlink(item.Resolved, item.Target, true)
					if err != nil {
						return err
					}
				}
			} else {
				if fileManager.NeedDownload(item.Resolved) {
					_, err := fileManager.Download(item.Resolved, item.Target)
					if err != nil {
						return err
					}
				} else {
					err := fileManager.CopyDir(item.Resolved, item.Target)
					if err != nil {
						return err
					}
				}
			}
		} else if item.Type == "CreateFile" || item.Type == "CreateWritableFile" {
			err := WriteToFile(item.Target, item.Resolved)
			if err != nil {
				return err
			}
		} else if item.Type == "Directory" && strings.HasPrefix(item.Resolved, "_:") {
			os.MkdirAll(item.Target, 0755)
		} else {
			if item.GetStreamable() && pipeMap[item.Resolved] != nil {
				err := ensureDirExists(item.Target)
				if err != nil {
					return err
				}
				if err = syscall.Mkfifo(item.Target, 0666); err != nil {
					return fmt.Errorf("failed to create named pipe %s: %v", item.Target, err)
				}
				pipe := pipeMap[item.Resolved]
				pipe.addWriter(func() (io.WriteCloser, error) {
					return os.Create(item.Target)
				})
			} else {
				needDownload := fileManager.NeedDownload(item.Resolved)
				if needDownload {
					_, err := fileManager.Download(item.Resolved, item.Target)
					if err != nil {
						return err
					}
				} else {
					err := symlink(item.Resolved, item.Target, true)
					if err != nil {
						return err
					}
				}
				targets = append(targets, item.Target)
			}
		}
	}
	return nil
}
func stageForDocker(fileManager FileManager, hostWorkdir string, containerWorkDir string, items []api.MapperEnt, inplaceUpdate bool) ([]string, error) {
	var dockerCommands []string
	dockerCommands = append(dockerCommands, "--mount=type=bind,source="+hostWorkdir+",target="+containerWorkDir)
	tmpDir, err := os.MkdirTemp("", "flowy-tmpdir")
	if err != nil {
		return nil, err
	}
	dockerCommands = append(dockerCommands, "--mount=type=bind,source="+tmpDir+",target=/tmp")
	for _, item := range items {
		if !item.Staged {
			continue
		}
		if item.Type == "WritableFile" {
			if inplaceUpdate {
				in_workdir := strings.HasPrefix(item.Target, containerWorkDir)
				hostTargetPath := strings.Replace(item.Target, containerWorkDir, hostWorkdir, 1)
				if in_workdir {
					if fileManager.NeedDownload(item.Resolved) {
						_, err := fileManager.Download(item.Resolved, hostTargetPath)
						if err != nil {
							return nil, err
						}
					} else {
						dockerCommands = append(dockerCommands, "--mount=type=bind,source="+item.Resolved+",target="+item.Target)
						err := symlink(item.Resolved, hostTargetPath, true)
						if err != nil {
							return nil, err
						}
					}
				} else {
					if fileManager.NeedDownload(item.Resolved) {
						_, err := fileManager.Download(item.Resolved, hostTargetPath)
						if err != nil {
							return nil, err
						}
						dockerCommands = append(dockerCommands, "--mount=type=bind,source="+hostTargetPath+",target="+item.Target)
					} else {
						dockerCommands = append(dockerCommands, "--mount=type=bind,source="+item.Resolved+",target="+item.Target)
					}
				}
			} else {
				// always copy when writable file
				target := strings.Replace(item.Target, containerWorkDir, hostWorkdir, 1)
				err = fileManager.CopyFile(item.Resolved, target)
				if err != nil {
					return nil, err
				}
			}
		} else if item.Type == "WritableDirectory" {
			var hostOutdirTarget *string = nil
			if strings.HasPrefix(item.Target, containerWorkDir) {
				hostTarget := strings.Replace(item.Target, containerWorkDir, hostWorkdir, 1)
				hostOutdirTarget = &hostTarget
			}
			if strings.HasPrefix(item.Resolved, "_:") {
				if hostOutdirTarget != nil {
					// create new directory in workdir
					err := os.MkdirAll(*hostOutdirTarget, 0755)
					if err != nil {
						return nil, err
					}
				} else {
					// create new directory in staging dir
					err := os.MkdirAll(item.Target, 0755)
					if err != nil {
						return nil, err
					}
					dockerCommands = append(dockerCommands, "--mount=type=bind,source="+item.Target+",target="+item.Target)
				}
			} else {
				if inplaceUpdate {
					dockerCommands = append(dockerCommands, "--mount=type=bind,source="+item.Resolved+",target="+item.Target)
				} else {
					if hostOutdirTarget != nil {
						// copy directory to workdir
						err = fileManager.CopyDir(item.Resolved, *hostOutdirTarget)
						if err != nil {
							return nil, err
						}
					} else {
						// copy directory to staged dir
						err = fileManager.CopyDir(item.Resolved, item.Target)
						if err != nil {
							return nil, err
						}
						// mount to container
						dockerCommands = append(dockerCommands, "--mount=type=bind,source="+item.Target+",target="+item.Target)
					}
				}
			}
		} else if item.Type == "CreateFile" {
			staging := false
			hostPath := item.Target
			if strings.HasPrefix(item.Target, containerWorkDir) {
				hostPath = strings.Replace(item.Target, containerWorkDir, hostWorkdir, 1)
				staging = true
			}
			err = WriteToFile(hostPath, item.Resolved)
			if err != nil {
				return nil, err
			}
			if !staging {
				dockerCommands = append(dockerCommands, "--mount=type=bind,source="+hostPath+",target="+item.Target)
			}
		} else if item.Type == "File" || item.Type == "Directory" {
			if strings.HasPrefix(item.Resolved, "_:") {
				if item.Type == "Directory" {
					hostPath := strings.Replace(item.Target, containerWorkDir, hostWorkdir, 1)
					err = os.Mkdir(hostPath, 0755)
					if err != nil {
						return nil, err
					}
				}
			} else {
				containerTargetPath := item.Target
				in_workdir := strings.HasPrefix(item.Target, containerWorkDir)
				sourcePath, err := fileUriToPath(item.Resolved)
				if err != nil {
					return nil, err
				}
				hostTargetPath := item.Target
				if in_workdir {
					// convert container path to host path
					hostTargetPath = strings.Replace(containerTargetPath, containerWorkDir, hostWorkdir, 1)
				}
				needDownload := fileManager.NeedDownload(item.Resolved)
				if needDownload {
					_, err := fileManager.Download(item.Resolved, hostTargetPath)
					if err != nil {
						return nil, err
					}
					sourcePath = hostTargetPath
				}
				cmd := []string{"--mount=type=bind", "source=" + sourcePath, "target=" + containerTargetPath}
				if !inplaceUpdate {
					cmd = append(cmd, "readonly")
				}
				dockerCommands = append(dockerCommands, strings.Join(cmd, ","))
				if !needDownload && in_workdir {
					err := symlink(sourcePath, hostTargetPath, true)
					if err != nil {
						return nil, err
					}
				}
			}
		}
	}
	return dockerCommands, nil
}
