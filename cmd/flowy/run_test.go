package main

import (
	"context"
	"encoding/json"
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
// returns exit 0, stderr contains "Execution directory:", and stdout is valid JSON.
func TestRunWorkflow(t *testing.T) {
	wdlPath := filepath.Join(testdataDir(), "valid_workflow.wdl")
	code, stdout, stderr := runArgsWithRunner([]string{"run", wdlPath}, successMock())
	if code != 0 {
		t.Errorf("expected exit 0, got %d (stderr: %q, stdout: %q)", code, stderr, stdout)
	}
	if !strings.Contains(stderr, "Execution directory:") {
		t.Errorf("expected stderr to contain 'Execution directory:', got %q", stderr)
	}
	if !json.Valid([]byte(strings.TrimSpace(stdout))) {
		t.Errorf("expected stdout to be valid JSON, got %q", stdout)
	}
}

// TestRunWithInput tests that --input loads JSON and passes inputs to RunWorkflow.
// stdout must be valid JSON.
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
	if !json.Valid([]byte(strings.TrimSpace(stdout))) {
		t.Errorf("expected stdout to be valid JSON, got %q", stdout)
	}
}

// TestRunTask tests that --task runs a single task instead of the workflow.
// stdout must be valid JSON with taskName.outputName keys.
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
	trimmed := strings.TrimSpace(stdout)
	if !json.Valid([]byte(trimmed)) {
		t.Errorf("expected stdout to be valid JSON for task run, got %q", stdout)
	}
	var got map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &got); err != nil {
		t.Fatalf("unmarshal stdout: %v", err)
	}
	// Verify keys have say_hello. prefix.
	for k := range got {
		if !strings.HasPrefix(k, "say_hello.") {
			t.Errorf("expected key to have 'say_hello.' prefix, got %q", k)
		}
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

// TestRunWorkflowStdoutJSON: workflow run stdout is valid JSON with entityName.outputName keys.
func TestRunWorkflowStdoutJSON(t *testing.T) {
	wdlPath := filepath.Join(testdataDir(), "valid_workflow.wdl")
	code, stdout, stderr := runArgsWithRunner([]string{"run", wdlPath}, successMock())
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stderr: %q)", code, stderr)
	}
	trimmed := strings.TrimSpace(stdout)
	var got map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %q", err, stdout)
	}
	// greet workflow has no output block so got should be empty map — that's still valid JSON.
	// Verify stdout contains ONLY JSON: no "Outputs written to" text.
	if strings.Contains(stdout, "Outputs written to") {
		t.Errorf("stdout must not contain 'Outputs written to', got %q", stdout)
	}
}

// TestRunStdoutIsValidJSON: json.Unmarshal(stdout) succeeds on workflow run.
func TestRunStdoutIsValidJSON(t *testing.T) {
	wdlPath := filepath.Join(testdataDir(), "valid_workflow.wdl")
	code, stdout, stderr := runArgsWithRunner([]string{"run", wdlPath}, successMock())
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stderr: %q)", code, stderr)
	}
	if !json.Valid([]byte(strings.TrimSpace(stdout))) {
		t.Errorf("stdout is not valid JSON: %q", stdout)
	}
}

// TestRunWithOutputFlag: --output /tmp/out.json writes JSON to that path; stdout is empty.
func TestRunWithOutputFlag(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.json")
	wdlPath := filepath.Join(testdataDir(), "valid_workflow.wdl")
	code, stdout, stderr := runArgsWithRunner(
		[]string{"run", "--output", outPath, wdlPath},
		successMock(),
	)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stderr: %q)", code, stderr)
	}
	// stdout should be empty when --output flag is used.
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("expected empty stdout with --output flag, got %q", stdout)
	}
	// The specified file should exist and contain valid JSON.
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading output file %q: %v", outPath, err)
	}
	if !json.Valid(data) {
		t.Errorf("output file %q is not valid JSON: %q", outPath, data)
	}
}

// TestRunTaskStdoutJSON: --task run stdout is valid JSON with taskName.outputName keys.
func TestRunTaskStdoutJSON(t *testing.T) {
	wdlPath := filepath.Join(testdataDir(), "valid_workflow.wdl")
	taskInputPath := filepath.Join(testdataDir(), "task_inputs.json")
	code, stdout, stderr := runArgsWithRunner(
		[]string{"run", "--task", "say_hello", "--input", taskInputPath, wdlPath},
		successMock(),
	)
	if code != 0 {
		t.Fatalf("expected exit 0 for --task, got %d (stderr: %q)", code, stderr)
	}
	trimmed := strings.TrimSpace(stdout)
	var got map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %q", err, stdout)
	}
	// Verify keys have say_hello. prefix.
	for k := range got {
		if !strings.HasPrefix(k, "say_hello.") {
			t.Errorf("expected key prefix 'say_hello.', got %q", k)
		}
	}
}
