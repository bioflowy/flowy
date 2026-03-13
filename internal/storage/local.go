// Package storage — LocalBackend wraps internal/store filesystem helpers.
package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/bioflowy/flowy/internal/store"
)

// LocalBackend implements Backend using the local filesystem.
// All path helpers delegate to the corresponding store.* functions so that
// directory layout is defined in exactly one place.
type LocalBackend struct{}

// RunInputsDir delegates to store.RunInputsDir.
func (l *LocalBackend) RunInputsDir(baseDir, runID string) string {
	return store.RunInputsDir(baseDir, runID)
}

// RunOutputsDir delegates to store.RunOutputsDir.
func (l *LocalBackend) RunOutputsDir(baseDir, runID string) string {
	return store.RunOutputsDir(baseDir, runID)
}

// TaskDir delegates to store.TaskDir.
func (l *LocalBackend) TaskDir(baseDir, runID, taskName string, shardIndex int) string {
	return store.TaskDir(baseDir, runID, taskName, shardIndex)
}

// CreateRunDirs delegates to store.CreateRunDirs.
func (l *LocalBackend) CreateRunDirs(baseDir, runID string) error {
	return store.CreateRunDirs(baseDir, runID)
}

// CreateTaskDirs delegates to store.CreateTaskDirs.
func (l *LocalBackend) CreateTaskDirs(baseDir, runID, taskName string, shardIndex int) error {
	return store.CreateTaskDirs(baseDir, runID, taskName, shardIndex)
}

// UploadFile writes data to path, creating intermediate directories as needed.
func (l *LocalBackend) UploadFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("storage: create parent dir for %q: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("storage: write file %q: %w", path, err)
	}
	return nil
}

// DownloadFile reads and returns the contents of path.
func (l *LocalBackend) DownloadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("storage: read file %q: %w", path, err)
	}
	return data, nil
}

// UploadDirectory is a no-op for LocalBackend because the source path IS the
// storage path in local mode — no transfer is required.
func (l *LocalBackend) UploadDirectory(path string, localSrc string) error {
	return nil
}

// DownloadDirectory is a no-op for LocalBackend because the destination path
// IS the storage path in local mode — no transfer is required.
func (l *LocalBackend) DownloadDirectory(path string, localDst string) error {
	return nil
}

// UploadFileStream reads from r and writes to path, creating intermediate
// directories as needed. It is a thin wrapper around os.Create + io.Copy.
func (l *LocalBackend) UploadFileStream(path string, r io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("storage: create parent dir for %q: %w", path, err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("storage: create file %q: %w", path, err)
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("storage: write stream to %q: %w", path, err)
	}
	return nil
}

// DownloadFileStream opens path and copies its contents to w.
func (l *LocalBackend) DownloadFileStream(path string, w io.Writer) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("storage: open file %q: %w", path, err)
	}
	defer f.Close()
	if _, err := io.Copy(w, f); err != nil {
		return fmt.Errorf("storage: read stream from %q: %w", path, err)
	}
	return nil
}

// WriteJSON marshals v to JSON and writes it to path via UploadFile.
func (l *LocalBackend) WriteJSON(path string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("storage: marshal JSON for %q: %w", path, err)
	}
	return l.UploadFile(path, data)
}

// ReadJSON reads the file at path and unmarshals its contents into v.
func (l *LocalBackend) ReadJSON(path string, v interface{}) error {
	data, err := l.DownloadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("storage: unmarshal JSON from %q: %w", path, err)
	}
	return nil
}
