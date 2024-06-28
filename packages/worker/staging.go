package main

import (
	"os"
	"strings"

	"github.com/bioflowy/flowy-cwl3/flowydeamon/api"
)

func stageForCommandLine(fileManager FileManager, items []api.MapperEnt, inplaceUpdate bool) error {
	var targets = []string{}
	for _, item := range items {
		if contains(targets, item.Target) {
			continue
		}
		if item.Type == "WritableFile" {
			err := copy(item.Resolved, item.Target)
			if err != nil {
				return err
			}
		} else if item.Type == "WritableDirectory" {
			if strings.HasPrefix(item.Resolved, "_:") {
				os.Mkdir(item.Target, 0755)
			} else if inplaceUpdate {
				err := symlink(item.Resolved, item.Target, false)
				if err != nil {
					return err
				}
			} else {
				err := fileManager.CopyDir(item.Resolved, item.Target)
				if err != nil {
					return err
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
	var targets []string
	for _, item := range items {
		if !item.Staged {
			continue
		}
		if item.Type == "WritableFile" {
			if inplaceUpdate {
				if !contains(targets, item.Target) {
					in_workdir := strings.HasPrefix(item.Target, containerWorkDir)
					dockerCommands = append(dockerCommands, "--mount=type=bind,source="+item.Resolved+",target="+item.Target)
					targets = append(targets, item.Target)
					if in_workdir {
						hostTargetPath := strings.Replace(item.Target, containerWorkDir, hostWorkdir, 1)
						err := symlink(item.Resolved, hostTargetPath, true)
						if err != nil {
							return nil, err
						}
					}
				}
			} else {
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
				if !contains(targets, item.Target) {
					containerTargetPath := item.Target
					in_workdir := strings.HasPrefix(item.Target, containerWorkDir)
					sourcePath := fileUriToPath(item.Resolved)
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
					targets = append(targets, item.Target)
					if !needDownload && in_workdir {
						err := symlink(sourcePath, hostTargetPath, true)
						if err != nil {
							return nil, err
						}
					}
				}
			}
		}
	}
	return dockerCommands, nil
}
