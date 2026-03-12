// Package storage_test tests the storage package's ParseS3URI helper.
package storage_test

import (
	"testing"

	"github.com/bioflowy/flowy/internal/storage"
)

func TestParseS3URI(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		wantBucket  string
		wantKey     string
		wantErr     bool
	}{
		{
			name:       "valid URI with nested path",
			uri:        "s3://mybucket/path/to/file",
			wantBucket: "mybucket",
			wantKey:    "path/to/file",
			wantErr:    false,
		},
		{
			name:       "valid URI with single-level key",
			uri:        "s3://mybucket/file.txt",
			wantBucket: "mybucket",
			wantKey:    "file.txt",
			wantErr:    false,
		},
		{
			name:    "empty string",
			uri:     "",
			wantErr: true,
		},
		{
			name:    "wrong scheme http",
			uri:     "http://bucket/key",
			wantErr: true,
		},
		{
			name:    "missing bucket s3://",
			uri:     "s3://",
			wantErr: true,
		},
		{
			name:    "empty bucket s3:///key",
			uri:     "s3:///key",
			wantErr: true,
		},
		{
			name:    "trailing slash empty key",
			uri:     "s3://bucket/",
			wantErr: true,
		},
		{
			name:    "no key only bucket",
			uri:     "s3://bucket",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucket, key, err := storage.ParseS3URI(tt.uri)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseS3URI(%q) = (%q, %q, nil), want error", tt.uri, bucket, key)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseS3URI(%q) returned unexpected error: %v", tt.uri, err)
				return
			}
			if bucket != tt.wantBucket {
				t.Errorf("ParseS3URI(%q) bucket = %q, want %q", tt.uri, bucket, tt.wantBucket)
			}
			if key != tt.wantKey {
				t.Errorf("ParseS3URI(%q) key = %q, want %q", tt.uri, key, tt.wantKey)
			}
		})
	}
}
