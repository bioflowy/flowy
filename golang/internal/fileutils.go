package internal

import (
	"crypto/sha1"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// ResolvePath returns the real path of the given path,
// resolving any symlinks. If the path doesn't exist,
// it returns the cleaned original path.
func ResolvePath(path string) (string, error) {
	// First, clean the path
	cleanPath := filepath.Clean(path)

	// Try to evaluate symlinks
	realPath, err := filepath.EvalSymlinks(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			// If the path doesn't exist, return the cleaned original path
			return cleanPath, nil
		}
		// For other errors, return the error
		return "", fmt.Errorf("error resolving path: %w", err)
	}

	// Return the resolved path
	return realPath, nil
}

// collect File or Directory entories from json
// come from cwltool/process.py
func CollectDirEntries(
	args interface{}) []map[string]interface{} {
	f_or_ds := make([]map[string]interface{}, 0)
	switch v := args.(type) {
	case map[string]interface{}:
		var v2 DataMap = v
		if v2.IsFile() || v2.IsDirectory() {
			f_or_ds = append(f_or_ds, v2)
		} else {
			for _, value := range v {
				entries := CollectDirEntries(value)
				f_or_ds = append(f_or_ds, entries...)
			}
		}
	case []interface{}:
		for _, value := range v {
			entries := CollectDirEntries(value)
			f_or_ds = append(f_or_ds, entries...)
		}
	}
	return f_or_ds
}
func RemovePath(path string) error {
	// First, get information about the path
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("error accessing path: %v", err)
	}
	return os.RemoveAll(path)
}

// CopyDir recursively copies a directory structure, handling symlinks.
func CopyDir(src string, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	si, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !si.IsDir() {
		return fmt.Errorf("source is not a directory")
	}

	err = os.MkdirAll(dst, si.Mode())
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			err = CopyDir(srcPath, dstPath)
			if err != nil {
				return err
			}
		} else {
			// Skip symlinks pointing to directories to avoid recursive loops
			if entry.Type()&os.ModeSymlink != 0 {
				linkTarget, err := os.Readlink(srcPath)
				if err != nil {
					return fmt.Errorf("failed to read symlink %s: %v", srcPath, err)
				}

				// Resolve the link target relative to the source directory
				fullLinkTarget := filepath.Join(filepath.Dir(srcPath), linkTarget)

				fi, err := os.Stat(fullLinkTarget)
				if err != nil {
					return fmt.Errorf("failed to stat link target %s: %v", fullLinkTarget, err)
				}

				if fi.IsDir() {
					// Create a new symlink
					err = os.Symlink(linkTarget, dstPath)
					if err != nil {
						return fmt.Errorf("failed to create symlink %s -> %s: %v", dstPath, linkTarget, err)
					}
					continue
				}
			}

			err = CopyFile(srcPath, dstPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// CopyFile copies a single file from src to dst.
func CopyFile(src, dst string) error {
	var err error
	var srcfd *os.File
	var dstfd *os.File
	var srcinfo os.FileInfo

	if srcfd, err = os.Open(src); err != nil {
		return err
	}
	defer srcfd.Close()

	if dstfd, err = os.Create(dst); err != nil {
		return err
	}
	defer dstfd.Close()

	if _, err = io.Copy(dstfd, srcfd); err != nil {
		return err
	}
	if srcinfo, err = os.Stat(src); err != nil {
		return err
	}
	return os.Chmod(dst, srcinfo.Mode())
}
func uriFilePath(inputUrl string) (string, error) {
	if !strings.HasPrefix(inputUrl, "file://") {
		return inputUrl, nil
	}
	u, err := url.Parse(inputUrl)
	if err != nil {
		return "", err
	}
	if u.Scheme != "file" {
		return "", fmt.Errorf("not a file URI: %s", inputUrl)
	}
	return filepath.FromSlash(u.Path), nil
}

func ComputeChecksums(file File) error {
	if !file.HasChecksum() {
		hash := sha1.New()
		p, err := uriFilePath(file.GetLocation())
		if err != nil {
			return nil
		}
		fileHandle, err := os.Open(p)
		if err != nil {
			return err
		}
		defer fileHandle.Close()

		_, err = io.Copy(hash, fileHandle)
		if err != nil {
			return err
		}
		fileInfo, err := fileHandle.Stat()
		if err != nil {
			return err
		}
		file.SetSize(fileInfo.Size())
		if file.HasChecksum() {
			return nil
		}
		checksum := fmt.Sprintf("sha1$%x", hash.Sum(nil))
		file.SetChecksum(checksum)

	}

	return nil
}
