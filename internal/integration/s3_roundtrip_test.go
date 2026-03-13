//go:build integration

package integration

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bioflowy/flowy/internal/config"
	"github.com/bioflowy/flowy/internal/storage"
)

// newTestBackend constructs an S3Backend pointing to the shared MinIO container.
// It sets AWS credentials via t.Setenv so they are restored after the test ends.
func newTestBackend(t *testing.T) *storage.S3Backend {
	t.Helper()
	t.Setenv("AWS_ACCESS_KEY_ID", testUser)
	t.Setenv("AWS_SECRET_ACCESS_KEY", testPassword)

	cfg := config.Config{}
	cfg.Storage.Type = "s3"
	cfg.Storage.Bucket = testBucket
	cfg.Storage.Endpoint = minioEndpoint
	cfg.Storage.Region = "us-east-1"
	cfg.Storage.MultipartThreshold = 100 * 1024 * 1024 // 100 MiB

	backend, err := storage.NewS3Backend(&cfg)
	if err != nil {
		t.Fatalf("NewS3Backend: %v", err)
	}
	return backend
}

// TestS3RoundTrip verifies that a file uploaded via UploadFile can be downloaded
// back with identical content using DownloadFile.
func TestS3RoundTrip(t *testing.T) {
	backend := newTestBackend(t)
	const key = "test-roundtrip/input.txt"
	const content = "hello from integration test"

	if err := backend.UploadFile(key, []byte(content)); err != nil {
		t.Fatalf("UploadFile: %v", err)
	}

	got, err := backend.DownloadFile(key)
	if err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}

	if string(got) != content {
		t.Errorf("content mismatch: got %q, want %q", string(got), content)
	}
}

// TestS3StreamRoundTrip verifies that UploadFileStream and DownloadFileStream
// preserve content across the network.
func TestS3StreamRoundTrip(t *testing.T) {
	backend := newTestBackend(t)
	const key = "test-stream-roundtrip/data.bin"
	const content = "streaming hello from integration test"

	if err := backend.UploadFileStream(key, strings.NewReader(content)); err != nil {
		t.Fatalf("UploadFileStream: %v", err)
	}

	var buf bytes.Buffer
	if err := backend.DownloadFileStream(key, &buf); err != nil {
		t.Fatalf("DownloadFileStream: %v", err)
	}

	if buf.String() != content {
		t.Errorf("stream content mismatch: got %q, want %q", buf.String(), content)
	}
}

// TestS3DirectoryUploadDownload verifies that UploadDirectory and DownloadDirectory
// correctly transfer a set of files, preserving their relative paths and contents.
func TestS3DirectoryUploadDownload(t *testing.T) {
	backend := newTestBackend(t)

	// Create a local source directory with three files.
	srcDir := t.TempDir()
	files := map[string]string{
		"alpha.txt":        "alpha content",
		"sub/beta.txt":     "beta content",
		"sub/deep/gamma.txt": "gamma content",
	}
	for rel, content := range files {
		path := filepath.Join(srcDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll %q: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile %q: %v", path, err)
		}
	}

	const s3Prefix = "test-directory-roundtrip"
	if err := backend.UploadDirectory(s3Prefix, srcDir); err != nil {
		t.Fatalf("UploadDirectory: %v", err)
	}

	// Download to a fresh destination directory.
	dstDir := t.TempDir()
	if err := backend.DownloadDirectory(s3Prefix, dstDir); err != nil {
		t.Fatalf("DownloadDirectory: %v", err)
	}

	// Verify every file is present with correct content.
	for rel, want := range files {
		path := filepath.Join(dstDir, filepath.FromSlash(rel))
		got, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("ReadFile %q after download: %v", path, err)
			continue
		}
		if string(got) != want {
			t.Errorf("file %q content: got %q, want %q", rel, string(got), want)
		}
	}
}

// TestS3JSONReadWrite verifies that WriteJSON and ReadJSON correctly round-trip
// a structured value through S3.
func TestS3JSONReadWrite(t *testing.T) {
	backend := newTestBackend(t)
	const key = "test-json-roundtrip/config.json"

	type testStruct struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
		Flag  bool   `json:"flag"`
	}
	want := testStruct{Name: "flowy-test", Count: 42, Flag: true}

	if err := backend.WriteJSON(key, want); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	var got testStruct
	if err := backend.ReadJSON(key, &got); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}

	if got.Name != want.Name {
		t.Errorf("Name: got %q, want %q", got.Name, want.Name)
	}
	if got.Count != want.Count {
		t.Errorf("Count: got %d, want %d", got.Count, want.Count)
	}
	if got.Flag != want.Flag {
		t.Errorf("Flag: got %v, want %v", got.Flag, want.Flag)
	}
}
