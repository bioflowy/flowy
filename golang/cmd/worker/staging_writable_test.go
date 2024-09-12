package main

import (
	"os"
	"testing"

	"github.com/bioflowy/flowy/golang/cmd/worker/api"
	"github.com/stretchr/testify/assert"
)

func stageWritableFileForDokerTest(t *testing.T, fileManager FileManager, resolved string, target string, hostworkdir string, containerWorkDir string, mount string) {
	mounts, err := stageForDocker(fileManager, hostworkdir, containerWorkDir, []api.MapperEnt{
		{
			Type:     "WritableFile",
			Resolved: resolved,
			Target:   target,
		},
	}, false)
	if err != nil {
		panic(err)
	}
	assert.Equal(t, mount, mounts[2])
}
func TestStageLocalWritableFileForDocker(t *testing.T) {
	t.Run("local writable file in workdir with docker", func(t *testing.T) {
		hostworkdir, err := os.MkdirTemp("", "hostworkdir")
		if err != nil {
			panic(err)
		}
		testFile := "/home/testuser/test.txt"
		defer os.Remove(hostworkdir)
		fileManager := LocalFileManager{}
		stageWritableFileForDokerTest(t, &fileManager, "file:/"+testFile, "/CONTAINER_WORK/test.txt", hostworkdir, "/CONTAINER_WORK",
			"--mount=type=bind,source="+testFile+",target=/CONTAINER_WORK/test.txt")
		path, err := os.Readlink(hostworkdir + "/test.txt")
		if err != nil {
			panic(err)
		}
		assert.Equal(t, testFile, path)
	})
}

func TestStateLocalWritableFileForCommandLine(t *testing.T) {
	t.Run("local file in workdir with docker", func(t *testing.T) {
		hostworkdir, err := os.MkdirTemp("", "hostworkdir")
		if err != nil {
			panic(err)
		}
		testFile := "/home/testuser/test.txt"
		defer os.Remove(hostworkdir)
		fileManager := LocalFileManager{}
		stageFileForDokerTest(t, &fileManager, "file:/"+testFile, "/CONTAINER_WORK/test.txt", hostworkdir, "/CONTAINER_WORK",
			"--mount=type=bind,source="+testFile+",target=/CONTAINER_WORK/test.txt,readonly")
		path, err := os.Readlink(hostworkdir + "/test.txt")
		if err != nil {
			panic(err)
		}
		assert.Equal(t, testFile, path)
	})
}

func TestS3WritableFileForWithDocker(t *testing.T) {
	t.Run("s3 file in staging dir with docker", func(t *testing.T) {
		hostworkdir, err := os.MkdirTemp("", "hostworkdir")
		if err != nil {
			panic(err)
		}
		defer os.Remove(hostworkdir)
		fileInfo, err := os.Stat("/var/lib/cwl/stg0000")
		if fileInfo != nil {
			os.RemoveAll("/var/lib/cwl/stg0000")
			if err != nil {
				panic(err)
			}
		}

		var filemanager FileManager = &DummyS3FileManager{}
		stageFileForDokerTest(t, filemanager, "file://home/user/test.txt", "/var/lib/cwl/stg0000/test.txt", hostworkdir, "/CONTAINER_WORK",
			"--mount=type=bind,source=/var/lib/cwl/stg0000/test.txt,target=/var/lib/cwl/stg0000/test.txt,readonly")
		exist := checkFileExists("/var/lib/cwl/stg0000/test.txt")
		if !exist {
			t.Errorf("file not found")
		}
		os.RemoveAll("/var/lib/cwl/stg0000")
	})
	t.Run("s3 file in workdir with docker", func(t *testing.T) {
		hostworkdir, err := os.MkdirTemp("", "hostworkdir")
		if err != nil {
			panic(err)
		}
		testFile := "/home/testuser/test.txt"
		defer os.Remove(hostworkdir)
		var filemanager FileManager = &DummyS3FileManager{}
		stageFileForDokerTest(t, filemanager, "file:/"+testFile, "/CONTAINER_WORK/test.txt", hostworkdir, "/CONTAINER_WORK",
			"--mount=type=bind,source="+hostworkdir+"/test.txt,target=/CONTAINER_WORK/test.txt,readonly")
		exist := checkFileExists(hostworkdir + "/test.txt")
		if !exist {
			t.Errorf("file not found")
		}
	})
}
