package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bioflowy/flowy/wdl/task"
)

// mockRunner is a ContainerRunner implementation for testing.
// It returns a canned RunResult and optionally writes stdout/stderr files
// to emulate task execution without Docker.
type mockRunner struct {
	result   task.RunResult
	err      error
	execDirs []string // tracks execRoot dirs passed via side-channel (unused for basic tests)
}

func (m *mockRunner) Run(_ context.Context, _ task.RunRequest) (task.RunResult, error) {
	return m.result, m.err
}

// successMock returns a mock that simulates a successful task run.
func successMock() *mockRunner {
	return &mockRunner{
		result: task.RunResult{
			ExitCode: 0,
			Stdout:   []byte("Hello, world!\n"),
			Stderr:   []byte{},
		},
	}
}

// runArgsWithRunner calls run() with the given args and a specific ContainerRunner.
// It temporarily replaces the package-level newRunner variable.
func runArgsWithRunner(args []string, runner task.ContainerRunner) (code int, stdout, stderr string) {
	orig := newRunner
	newRunner = func() task.ContainerRunner { return runner }
	defer func() { newRunner = orig }()
	return runArgs(args)
}

// TestRunMissingWDLArg tests that "flowy run" without a WDL file returns exit 2.
func TestRunMissingWDLArg(t *testing.T) {
	code, _, stderr := runArgsWithRunner([]string{"run"}, successMock())
	if code != 2 {
		t.Errorf("expected exit 2 for missing WDL arg, got %d (stderr: %q)", code, stderr)
	}
}

// TestRunWorkflow tests that "flowy run valid_workflow.wdl" with a mock runner
// returns exit 0, stderr contains "Execution directory:", and stdout contains "outputs.json".
func TestRunWorkflow(t *testing.T) {
	wdlPath := filepath.Join(testdataDir(), "valid_workflow.wdl")
	code, stdout, stderr := runArgsWithRunner([]string{"run", wdlPath}, successMock())
	if code != 0 {
		t.Errorf("expected exit 0, got %d (stderr: %q, stdout: %q)", code, stderr, stdout)
	}
	if !strings.Contains(stderr, "Execution directory:") {
		t.Errorf("expected stderr to contain 'Execution directory:', got %q", stderr)
	}
	if !strings.Contains(stdout, "outputs.json") {
		t.Errorf("expected stdout to contain 'outputs.json', got %q", stdout)
	}
}

// TestRunWithInput tests that --input loads JSON and passes inputs to RunWorkflow.
func TestRunWithInput(t *testing.T) {
	wdlPath := filepath.Join(testdataDir(), "valid_workflow.wdl")
	inputPath := filepath.Join(testdataDir(), "inputs.json")
	code, stdout, stderr := runArgsWithRunner(
		[]string{"run", "--input", inputPath, wdlPath},
		successMock(),
	)
	if code != 0 {
		t.Errorf("expected exit 0, got %d (stderr: %q, stdout: %q)", code, stderr, stdout)
	}
	if !strings.Contains(stdout, "outputs.json") {
		t.Errorf("expected stdout to contain 'outputs.json', got %q", stdout)
	}
}

// TestRunTask tests that --task runs a single task instead of the workflow.
func TestRunTask(t *testing.T) {
	wdlPath := filepath.Join(testdataDir(), "valid_workflow.wdl")
	taskInputPath := filepath.Join(testdataDir(), "task_inputs.json")
	code, stdout, stderr := runArgsWithRunner(
		[]string{"run", "--task", "say_hello", "--input", taskInputPath, wdlPath},
		successMock(),
	)
	if code != 0 {
		t.Errorf("expected exit 0 for --task, got %d (stderr: %q, stdout: %q)", code, stderr, stdout)
	}
	if !strings.Contains(stdout, "outputs.json") {
		t.Errorf("expected stdout to contain 'outputs.json' for task run, got %q", stdout)
	}
}

// TestRunTaskNotFound tests that --task with a nonexistent task name returns exit 1.
func TestRunTaskNotFound(t *testing.T) {
	wdlPath := filepath.Join(testdataDir(), "valid_workflow.wdl")
	code, _, stderr := runArgsWithRunner(
		[]string{"run", "--task", "nonexistent", wdlPath},
		successMock(),
	)
	if code != 1 {
		t.Errorf("expected exit 1 for nonexistent task, got %d (stderr: %q)", code, stderr)
	}
}

// TestRunWithDir tests that --dir uses the specified directory as execRoot.
func TestRunWithDir(t *testing.T) {
	dir := t.TempDir()
	wdlPath := filepath.Join(testdataDir(), "valid_workflow.wdl")
	code, stdout, stderr := runArgsWithRunner(
		[]string{"run", "--dir", dir, wdlPath},
		successMock(),
	)
	if code != 0 {
		t.Errorf("expected exit 0 with --dir, got %d (stderr: %q, stdout: %q)", code, stderr, stdout)
	}
	// Stderr should mention the specified dir as execution directory.
	if !strings.Contains(stderr, dir) {
		t.Errorf("expected stderr to contain dir path %q, got %q", dir, stderr)
	}
}

// TestRunWithDirNonEmpty tests that --dir with a non-empty directory returns exit 1.
func TestRunWithDirNonEmpty(t *testing.T) {
	dir := t.TempDir()
	// Write a file to make the dir non-empty.
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	wdlPath := filepath.Join(testdataDir(), "valid_workflow.wdl")
	code, _, stderr := runArgsWithRunner(
		[]string{"run", "--dir", dir, wdlPath},
		successMock(),
	)
	if code != 1 {
		t.Errorf("expected exit 1 for non-empty dir, got %d (stderr: %q)", code, stderr)
	}
}

// TestRunDefaultExecDir tests that without --dir, a unique temp dir is created with "flowy-" prefix.
func TestRunDefaultExecDir(t *testing.T) {
	wdlPath := filepath.Join(testdataDir(), "valid_workflow.wdl")
	code, _, stderr := runArgsWithRunner([]string{"run", wdlPath}, successMock())
	if code != 0 {
		t.Errorf("expected exit 0, got %d (stderr: %q)", code, stderr)
	}
	// Execution directory line should contain "flowy-" prefix.
	if !strings.Contains(stderr, "flowy-") {
		t.Errorf("expected stderr to contain 'flowy-' prefix in exec dir, got %q", stderr)
	}
}
