// Package storage defines the Backend interface and helpers for abstracting
// file storage operations. The LocalBackend wraps local filesystem operations;
// future phases will add S3Backend for object storage.
package storage

import (
	"errors"
	"io"
	"strings"
)

// Backend abstracts file storage operations for workflow run data.
// All methods use whole-file semantics ([]byte) rather than streaming I/O
// to keep callers simple. Path-building helpers are included so that both
// local FS paths and future S3 key layouts are encapsulated in one interface.
//
// Important: no context.Context parameters — per project decision, signatures
// use value-in/value-out without context propagation.
type Backend interface {
	// Path helpers — compute the canonical path/key for a run or task.
	RunInputsDir(baseDir, runID string) string
	RunOutputsDir(baseDir, runID string) string
	TaskDir(baseDir, runID, taskName string, shardIndex int) string

	// Directory lifecycle — create the directory structure for a run or task.
	// Implementations must be idempotent (safe to call multiple times).
	CreateRunDirs(baseDir, runID string) error
	CreateTaskDirs(baseDir, runID, taskName string, shardIndex int) error

	// File I/O — upload (write) and download (read) individual files.
	UploadFile(path string, data []byte) error
	DownloadFile(path string) ([]byte, error)

	// Directory I/O — upload and download directory trees.
	// For LocalBackend these are no-ops because the path IS the local source.
	UploadDirectory(path string, localSrc string) error
	DownloadDirectory(path string, localDst string) error

	// JSON I/O — marshal/unmarshal a value to/from a storage path.
	WriteJSON(path string, v interface{}) error
	ReadJSON(path string, v interface{}) error

	// Streaming I/O — upload and download individual files where callers
	// control the reader/writer lifecycle. Prefer these over UploadFile and
	// DownloadFile when the data source is already an io.Reader (e.g. HTTP
	// response body, pipe) to avoid double-buffering large payloads.
	UploadFileStream(path string, r io.Reader) error
	DownloadFileStream(path string, w io.Writer) error

	// Stat returns the size in bytes of the file or object at path.
	// Returns (0, error) if the file does not exist or cannot be inspected.
	Stat(path string) (int64, error)
}

// ParseS3URI parses an S3 URI of the form "s3://bucket/key" and returns the
// bucket and key components. It uses simple string operations rather than
// net/url to avoid path normalization that would corrupt S3 keys.
//
// Errors are returned for:
//   - non-s3:// scheme
//   - empty or missing bucket name
//   - missing or empty key (including trailing slash with no content)
func ParseS3URI(uri string) (bucket, key string, err error) {
	const prefix = "s3://"

	if uri == "" {
		return "", "", errors.New("storage: empty URI")
	}
	if !strings.HasPrefix(uri, prefix) {
		return "", "", errors.New("storage: URI must start with s3://")
	}

	// Everything after "s3://"
	rest := uri[len(prefix):]

	// rest must be "bucket/key" — find the first slash to split them.
	slashIdx := strings.IndexByte(rest, '/')
	if slashIdx < 0 {
		// No slash at all: "s3://bucket" — no key.
		return "", "", errors.New("storage: S3 URI missing key (expected s3://bucket/key)")
	}

	bucket = rest[:slashIdx]
	key = rest[slashIdx+1:]

	if bucket == "" {
		return "", "", errors.New("storage: S3 URI has empty bucket name")
	}
	if key == "" {
		return "", "", errors.New("storage: S3 URI has empty key (trailing slash not allowed)")
	}

	return bucket, key, nil
}
