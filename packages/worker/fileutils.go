package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}
func WriteToFile(dest string, content string) error {
	destdir := filepath.Dir(dest)
	if !fileExists(destdir) {
		err := os.MkdirAll(destdir, 0755)
		if err != nil {
			return err
		}
	}
	file, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.WriteString(file, content)
	return err
}
func symlink(src string, dest string, error_if_exists bool) error {
	destdir := filepath.Dir(dest)
	if !fileExists(destdir) {
		err := os.MkdirAll(dest, 0755)
		if err != nil {
			return err
		}
	}
	if fileExists(dest) {
		if error_if_exists {
			return errors.New("File already exists")
		} else {
			return nil
		}
	}
	err := os.Symlink(src, dest)
	return err
}

func copy(src string, dest string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// コピー先のファイルを作成
	dstFile, err := os.Create(dest)
	if err != nil {
		panic(err)
	}
	defer dstFile.Close()

	// データをコピー
	_, err = io.Copy(dstFile, srcFile)
	return err
}
func WalkWithSymlink(directoryPath string, walkFunc filepath.WalkFunc) error {
	return walk(directoryPath, walkFunc, nil)
}

func walk(directoryPath string, walkFunc filepath.WalkFunc, prefix *string) error {
	if strings.HasSuffix(directoryPath, "/") {
		directoryPath = directoryPath + "/"
	}
	err := filepath.Walk(directoryPath, func(path string, info os.FileInfo, err error) error {
		orgPath := path
		if prefix != nil {
			orgPath = filepath.Join(*prefix, strings.TrimPrefix(path, directoryPath))
		}
		if info.Mode()&os.ModeSymlink != 0 {
			realpath, err := filepath.EvalSymlinks(path)
			if err != nil {
				return err
			}
			walk(realpath, walkFunc, &orgPath)
		} else {
			err := walkFunc(orgPath, info, err)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return err
}
