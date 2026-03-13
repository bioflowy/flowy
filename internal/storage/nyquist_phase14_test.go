// Package storage_test — Nyquist validation tests for Phase 14 (Storage Abstraction).
// These tests fill coverage gaps identified during phase validation for STOR-01, STOR-02.
package storage_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bioflowy/flowy/internal/config"
	"github.com/bioflowy/flowy/internal/storage"
)

// STOR-02: UploadFileStream + DownloadFileStream roundtrip on LocalBackend.
// The streaming methods are part of the Backend interface but had zero test coverage.
func TestLocalBackend_UploadDownloadFileStream_Roundtrip(t *testing.T) {
	b := &storage.LocalBackend{}
	dir := t.TempDir()
	path := filepath.Join(dir, "stream-test.bin")
	want := "streaming content with special chars: \x00\xff\n"

	// Upload via stream.
	if err := b.UploadFileStream(path, strings.NewReader(want)); err != nil {
		t.Fatalf("UploadFileStream failed: %v", err)
	}

	// Download via stream.
	var buf bytes.Buffer
	if err := b.DownloadFileStream(path, &buf); err != nil {
		t.Fatalf("DownloadFileStream failed: %v", err)
	}

	if buf.String() != want {
		t.Errorf("DownloadFileStream = %q, want %q", buf.String(), want)
	}
}

// STOR-02: UploadFileStream creates intermediate directories (same as UploadFile).
func TestLocalBackend_UploadFileStream_CreatesIntermediateDirs(t *testing.T) {
	b := &storage.LocalBackend{}
	dir := t.TempDir()
	path := filepath.Join(dir, "x", "y", "z", "streamed.txt")
	data := "nested stream data"

	if err := b.UploadFileStream(path, strings.NewReader(data)); err != nil {
		t.Fatalf("UploadFileStream with nested path failed: %v", err)
	}

	// Verify data via DownloadFile (cross-method verification).
	got, err := b.DownloadFile(path)
	if err != nil {
		t.Fatalf("DownloadFile after stream upload failed: %v", err)
	}
	if string(got) != data {
		t.Errorf("got %q, want %q", string(got), data)
	}
}

// STOR-02: DownloadFileStream returns error for non-existent file.
func TestLocalBackend_DownloadFileStream_NotFound(t *testing.T) {
	b := &storage.LocalBackend{}
	var buf bytes.Buffer
	err := b.DownloadFileStream("/nonexistent/path/file.txt", &buf)
	if err == nil {
		t.Error("DownloadFileStream with missing file should return error")
	}
}

// STOR-02: NewBackend with default config returns a *LocalBackend concrete type.
func TestNewBackend_ReturnsLocalBackend(t *testing.T) {
	cfg := config.Default()
	b := storage.NewBackend(cfg)

	_, ok := b.(*storage.LocalBackend)
	if !ok {
		t.Errorf("NewBackend(default config) returned %T, want *storage.LocalBackend", b)
	}
}

// STOR-01: ParseS3URI preserves keys with special characters that net/url would corrupt.
// This validates the design decision to use string ops rather than net/url.
func TestParseS3URI_PreservesSpecialChars(t *testing.T) {
	tests := []struct {
		name       string
		uri        string
		wantBucket string
		wantKey    string
	}{
		{
			name:       "key with double slashes",
			uri:        "s3://bucket/path//double//slashes",
			wantBucket: "bucket",
			wantKey:    "path//double//slashes",
		},
		{
			name:       "key with spaces (url-encoded style preserved)",
			uri:        "s3://bucket/path/with%20spaces/file.txt",
			wantBucket: "bucket",
			wantKey:    "path/with%20spaces/file.txt",
		},
		{
			name:       "key with dot segments preserved",
			uri:        "s3://bucket/path/../dots/./file",
			wantBucket: "bucket",
			wantKey:    "path/../dots/./file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucket, key, err := storage.ParseS3URI(tt.uri)
			if err != nil {
				t.Fatalf("ParseS3URI(%q) returned unexpected error: %v", tt.uri, err)
			}
			if bucket != tt.wantBucket {
				t.Errorf("bucket = %q, want %q", bucket, tt.wantBucket)
			}
			if key != tt.wantKey {
				t.Errorf("key = %q, want %q", key, tt.wantKey)
			}
		})
	}
}
