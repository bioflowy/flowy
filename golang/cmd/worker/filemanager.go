package main

import (
	"strings"

	"github.com/bioflowy/flowy/golang/internal"
)

type FileManager interface {
	CopyFile(srcPath string, destPath string) error
	CopyDir(srcPath string, destPath string) error
	NeedDownload(srcUrl string) bool
	Download(srcUrl string, target string) (bool, error)
	GetFileUrl(localPath string, fileurl *string) (string, error)
	GetType() string
	GetDownloadFileMap() map[string]string
}

type LocalFileManager struct {
}

func (f *LocalFileManager) GetDownloadFileMap() map[string]string {
	return nil
}
func (f *LocalFileManager) CopyFile(srcPath string, destPath string) error {
	return copy(srcPath, destPath)
}
func (f *LocalFileManager) CopyDir(srcPath string, destPath string) error {
	return internal.CopyDir(srcPath, destPath)
}
func (f *LocalFileManager) NeedDownload(fileurl string) bool {
	// never need download when local filesystem is available
	return false
}

func (f *LocalFileManager) Download(fileurl string, localPath string) (bool, error) {
	panic("currently not supported")
}
func (f *LocalFileManager) GetFileUrl(localPath string, fileurl *string) (string, error) {
	if strings.HasPrefix(localPath, "file://") {
		return localPath, nil
	} else {
		return "file:/" + localPath, nil
	}
	// do nothing when shared file system is available
}
func (f *LocalFileManager) GetType() string {
	// do nothing when shared file system is available
	return "nfs"
}
