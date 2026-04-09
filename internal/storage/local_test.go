// Package storage_test tests the LocalBackend implementation.
package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bioflowy/flowy/internal/config"
	"github.com/bioflowy/flowy/internal/storage"
	"github.com/bioflowy/flowy/internal/store"
)

// pathEquivalenceTests verifies that LocalBackend path helpers produce
// byte-for-byte identical results to the underlying store.* functions.
func TestLocalBackend_PathHelpers(t *testing.T) {
	b := &storage.LocalBackend{}
	base := "/tmp/test-base"
	runID := "run-abc-123"

	tests := []struct {
		name   string
		got    string
		expect string
	}{
		{
			name:   "RunInputsDir",
			got:    b.RunInputsDir(base, runID),
			expect: store.RunInputsDir(base, runID),
		},
		{
			name:   "RunOutputsDir",
			got:    b.RunOutputsDir(base, runID),
			expect: store.RunOutputsDir(base, runID),
		},
		{
			name:   "TaskDir shardIndex=0",
			got:    b.TaskDir(base, runID, "myTask", 0),
			expect: store.TaskDir(base, runID, "myTask", 0),
		},
		{
			name:   "TaskDir shardIndex=3",
			got:    b.TaskDir(base, runID, "scatterTask", 3),
			expect: store.TaskDir(base, runID, "scatterTask", 3),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expect {
				t.Errorf("got %q, want %q", tt.got, tt.expect)
			}
		})
	}
}

func TestLocalBackend_CreateRunDirs(t *testing.T) {
	b := &storage.LocalBackend{}
	base := t.TempDir()
	runID := "run-create-test"

	if err := b.CreateRunDirs(base, runID); err != nil {
		t.Fatalf("CreateRunDirs failed: %v", err)
	}

	// Verify inputs and outputs dirs were created.
	inputsDir := store.RunInputsDir(base, runID)
	outputsDir := store.RunOutputsDir(base, runID)

	for _, dir := range []string{inputsDir, outputsDir} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("expected directory %q to exist: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %q to be a directory", dir)
		}
	}

	// Idempotent: calling again must not error.
	if err := b.CreateRunDirs(base, runID); err != nil {
		t.Errorf("second CreateRunDirs returned unexpected error: %v", err)
	}
}

func TestLocalBackend_CreateTaskDirs(t *testing.T) {
	b := &storage.LocalBackend{}
	base := t.TempDir()
	runID := "run-task-test"
	taskName := "align"
	shardIndex := 2

	if err := b.CreateTaskDirs(base, runID, taskName, shardIndex); err != nil {
		t.Fatalf("CreateTaskDirs failed: %v", err)
	}

	taskDir := store.TaskDir(base, runID, taskName, shardIndex)
	info, err := os.Stat(taskDir)
	if err != nil {
		t.Fatalf("expected task directory %q to exist: %v", taskDir, err)
	}
	if !info.IsDir() {
		t.Errorf("expected %q to be a directory", taskDir)
	}
}

func TestLocalBackend_UploadDownloadFile(t *testing.T) {
	b := &storage.LocalBackend{}
	dir := t.TempDir()
	path := filepath.Join(dir, "test-file.txt")
	data := []byte("hello, storage world\n")

	if err := b.UploadFile(path, data); err != nil {
		t.Fatalf("UploadFile failed: %v", err)
	}

	got, err := b.DownloadFile(path)
	if err != nil {
		t.Fatalf("DownloadFile failed: %v", err)
	}

	if string(got) != string(data) {
		t.Errorf("DownloadFile = %q, want %q", string(got), string(data))
	}
}

func TestLocalBackend_UploadFile_CreatesIntermediateDirs(t *testing.T) {
	b := &storage.LocalBackend{}
	dir := t.TempDir()
	// Path with nested subdirectories that don't yet exist.
	path := filepath.Join(dir, "a", "b", "c", "file.txt")
	data := []byte("nested data")

	if err := b.UploadFile(path, data); err != nil {
		t.Fatalf("UploadFile with nested path failed: %v", err)
	}

	got, err := b.DownloadFile(path)
	if err != nil {
		t.Fatalf("DownloadFile failed: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("got %q, want %q", string(got), string(data))
	}
}

func TestLocalBackend_DownloadFile_NotFound(t *testing.T) {
	b := &storage.LocalBackend{}
	_, err := b.DownloadFile("/nonexistent/path/file.txt")
	if err == nil {
		t.Error("DownloadFile with missing file should return error")
	}
}

func TestLocalBackend_WriteReadJSON(t *testing.T) {
	type testStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	b := &storage.LocalBackend{}
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")

	original := testStruct{Name: "my-task", Value: 42}
	if err := b.WriteJSON(path, original); err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}

	var got testStruct
	if err := b.ReadJSON(path, &got); err != nil {
		t.Fatalf("ReadJSON failed: %v", err)
	}

	if got.Name != original.Name || got.Value != original.Value {
		t.Errorf("ReadJSON result %+v does not match original %+v", got, original)
	}
}

func TestLocalBackend_UploadDownloadDirectory_NoOp(t *testing.T) {
	b := &storage.LocalBackend{}
	if err := b.UploadDirectory("/some/path", "/local/src"); err != nil {
		t.Errorf("UploadDirectory should be a no-op: got error %v", err)
	}
	if err := b.DownloadDirectory("/some/path", "/local/dst"); err != nil {
		t.Errorf("DownloadDirectory should be a no-op: got error %v", err)
	}
}

func TestNewBackend_ReturnsBackend(t *testing.T) {
	cfg := config.Default()
	b := storage.NewBackend(cfg)
	if b == nil {
		t.Fatal("NewBackend returned nil")
	}
	// Verify it satisfies the Backend interface at compile time (type assertion).
	var _ storage.Backend = b
}

// TestLocalBackend_Stat verifies that Stat returns correct file sizes and errors.
func TestLocalBackend_Stat(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		b := &storage.LocalBackend{}
		dir := t.TempDir()
		path := filepath.Join(dir, "testfile.txt")

		// Write exactly 42 bytes.
		content := make([]byte, 42)
		for i := range content {
			content[i] = 'x'
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		size, err := b.Stat(path)
		if err != nil {
			t.Fatalf("Stat returned unexpected error: %v", err)
		}
		if size != 42 {
			t.Errorf("Stat size: want 42, got %d", size)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		b := &storage.LocalBackend{}
		size, err := b.Stat("/nonexistent/path/to/missing/file.txt")
		if err == nil {
			t.Fatal("Stat on non-existent file should return error, got nil")
		}
		if size != 0 {
			t.Errorf("Stat on non-existent file should return size 0, got %d", size)
		}
	})
}
