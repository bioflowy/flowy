package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bioflowy/flowy/internal/store"
)

func TestRunInputsDir(t *testing.T) {
	got := store.RunInputsDir("/data", "abc-123")
	want := filepath.Join("/data", "runs", "abc-123", "inputs")
	if got != want {
		t.Errorf("RunInputsDir() = %q, want %q", got, want)
	}
}

func TestRunOutputsDir(t *testing.T) {
	got := store.RunOutputsDir("/data", "abc-123")
	want := filepath.Join("/data", "runs", "abc-123", "outputs")
	if got != want {
		t.Errorf("RunOutputsDir() = %q, want %q", got, want)
	}
}

func TestCreateRunDirs(t *testing.T) {
	baseDir := t.TempDir()
	runID := "test-run-001"

	if err := store.CreateRunDirs(baseDir, runID); err != nil {
		t.Fatalf("CreateRunDirs() unexpected error: %v", err)
	}

	// Verify inputs directory was created
	inputsDir := store.RunInputsDir(baseDir, runID)
	if info, err := os.Stat(inputsDir); err != nil {
		t.Errorf("inputs dir not created: %v", err)
	} else if !info.IsDir() {
		t.Errorf("inputs path is not a directory: %s", inputsDir)
	}

	// Verify outputs directory was created
	outputsDir := store.RunOutputsDir(baseDir, runID)
	if info, err := os.Stat(outputsDir); err != nil {
		t.Errorf("outputs dir not created: %v", err)
	} else if !info.IsDir() {
		t.Errorf("outputs path is not a directory: %s", outputsDir)
	}
}

func TestTaskDir(t *testing.T) {
	got := store.TaskDir("/data", "abc-123", "hello", 0)
	want := filepath.Join("/data", "runs", "abc-123", "tasks", "hello_0")
	if got != want {
		t.Errorf("TaskDir() = %q, want %q", got, want)
	}

	// Verify scatter shard index is included.
	got2 := store.TaskDir("/base", "run-42", "scatter_task", 3)
	want2 := filepath.Join("/base", "runs", "run-42", "tasks", "scatter_task_3")
	if got2 != want2 {
		t.Errorf("TaskDir() scatter = %q, want %q", got2, want2)
	}
}

func TestCreateTaskDirs(t *testing.T) {
	baseDir := t.TempDir()
	runID := "test-run-task-001"

	if err := store.CreateTaskDirs(baseDir, runID, "my_task", 0); err != nil {
		t.Fatalf("CreateTaskDirs() unexpected error: %v", err)
	}

	taskDir := store.TaskDir(baseDir, runID, "my_task", 0)
	if info, err := os.Stat(taskDir); err != nil {
		t.Errorf("task dir not created: %v", err)
	} else if !info.IsDir() {
		t.Errorf("task path is not a directory: %s", taskDir)
	}

	// Idempotent: second call must not error.
	if err := store.CreateTaskDirs(baseDir, runID, "my_task", 0); err != nil {
		t.Errorf("CreateTaskDirs() second call unexpected error: %v", err)
	}
}

func TestCreateRunDirsIdempotent(t *testing.T) {
	baseDir := t.TempDir()
	runID := "test-run-idempotent"

	// First call
	if err := store.CreateRunDirs(baseDir, runID); err != nil {
		t.Fatalf("CreateRunDirs() first call unexpected error: %v", err)
	}

	// Write a file inside to confirm directory is not removed/recreated
	inputsDir := store.RunInputsDir(baseDir, runID)
	markerPath := filepath.Join(inputsDir, "marker.txt")
	if err := os.WriteFile(markerPath, []byte("marker"), 0o644); err != nil {
		t.Fatalf("failed to write marker file: %v", err)
	}

	// Second call should not error
	if err := store.CreateRunDirs(baseDir, runID); err != nil {
		t.Fatalf("CreateRunDirs() second call unexpected error: %v", err)
	}

	// Marker file should still exist (idempotent = does not wipe existing content)
	if _, err := os.Stat(markerPath); err != nil {
		t.Errorf("marker file missing after second CreateRunDirs call: %v", err)
	}
}
