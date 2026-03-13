package pathutil_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bioflowy/flowy/wdl/pathutil"
)

func TestCleanPath(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("could not get working directory: %v", err)
	}

	tests := []struct {
		name   string
		p      string
		wdlDir string
		want   string
	}{
		{
			name:   "s3 URI unchanged with no wdlDir",
			p:      "s3://bucket/key",
			wdlDir: "",
			want:   "s3://bucket/key",
		},
		{
			name:   "s3 URI with dotdot unchanged (no filepath.Clean corruption)",
			p:      "s3://bucket/path/../key",
			wdlDir: "",
			want:   "s3://bucket/path/../key",
		},
		{
			name:   "s3 URI with wdlDir ignores wdlDir",
			p:      "s3://bucket/key",
			wdlDir: "/some/wdldir",
			want:   "s3://bucket/key",
		},
		{
			name:   "absolute path cleaned",
			p:      "/abs/path",
			wdlDir: "",
			want:   filepath.Clean("/abs/path"),
		},
		{
			name:   "absolute path with dotdot normalized",
			p:      "/abs/../normalized",
			wdlDir: "",
			want:   "/normalized",
		},
		{
			name:   "relative file with wdlDir joined and cleaned",
			p:      "relative.txt",
			wdlDir: "/wdl/dir",
			want:   "/wdl/dir/relative.txt",
		},
		{
			name:   "relative sub-path with wdlDir joined",
			p:      "sub/file.txt",
			wdlDir: "/wdl/dir",
			want:   "/wdl/dir/sub/file.txt",
		},
		{
			name:   "relative path without wdlDir resolves against CWD",
			p:      "relative.txt",
			wdlDir: "",
			want:   filepath.Join(cwd, "relative.txt"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := pathutil.CleanPath(tc.p, tc.wdlDir)
			if got != tc.want {
				t.Errorf("CleanPath(%q, %q) = %q, want %q", tc.p, tc.wdlDir, got, tc.want)
			}
		})
	}
}
