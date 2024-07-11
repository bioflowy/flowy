package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type BufferWriteCloser struct {
	buffer *bytes.Buffer
	closed bool
}

func NewBufferWriteCloser() *BufferWriteCloser {
	return &BufferWriteCloser{
		buffer: &bytes.Buffer{},
		closed: false,
	}
}

func (bwc *BufferWriteCloser) Write(p []byte) (n int, err error) {
	if bwc.closed {
		return 0, fmt.Errorf("cannot write to closed buffer")
	}
	return bwc.buffer.Write(p)
}

func (bwc *BufferWriteCloser) Close() error {
	if bwc.closed {
		return fmt.Errorf("buffer already closed")
	}
	bwc.closed = true
	return nil
}

func (bwc *BufferWriteCloser) String() string {
	return bwc.buffer.String()
}
func fileUriToPath(uri string) (string, error) {
	parsedURI, err := url.Parse(uri)
	return parsedURI.Path, err
}

func removeDirectoryRecursive(dir string) error {
	// ディレクトリ内のファイルとサブディレクトリを取得
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	// ディレクトリ内の各エントリに対して処理を行う
	for _, entry := range entries {
		// エントリの絶対パスを取得
		fullPath := filepath.Join(dir, entry.Name())

		// エントリがディレクトリの場合は再帰的に削除
		if entry.IsDir() {
			err := removeDirectoryRecursive(fullPath)
			if err != nil {
				return err
			}
		} else {
			// エントリがファイルの場合は削除
			err := os.Remove(fullPath)
			if err != nil {
				return err
			}
		}
	}

	// 最後に、空になったディレクトリを削除
	err = os.Remove(dir)
	if err != nil {
		return err
	}

	return nil
}
func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}
func copyDir(src string, dest string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(dest, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		} else {
			return copyFile(path, destPath)
		}
	})
}

func copyFile(src, dest string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return err
	}

	// コピー元のファイルモードをコピー先に適用
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dest, srcInfo.Mode())
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
func ensureDirExists(path string) error {
	destdir := filepath.Dir(path)
	if fileExists(destdir) {
		return nil
	}
	return os.MkdirAll(destdir, 0755)
}
func symlink(src string, dest string, error_if_exists bool) error {
	ensureDirExists(dest)
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
