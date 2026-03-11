package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bioflowy/flowy/wdl/task"
)

// verboseMockRunner is a ContainerRunner that writes stdout/stderr files
// to the task execution directory to simulate real task output.
// The exec dir is inferred from the bind mounts in RunRequest.
type verboseMockRunner struct {
	stdoutContent string
	stderrContent string
}

func (m *verboseMockRunner) Run(_ context.Context, _ task.RunRequest) (task.RunResult, error) {
	// Return canned stdout/stderr — RunTask will write them to execDir/stdout.txt and execDir/stderr.txt.
	return task.RunResult{ExitCode: 0, Stdout: []byte(m.stdoutContent), Stderr: []byte(m.stderrContent)}, nil
}

// TestRunVerbosePrintsTaskOutput tests that --verbose prints task stdout/stderr to stderr.
func TestRunVerbosePrintsTaskOutput(t *testing.T) {
	runner := &verboseMockRunner{
		stdoutContent: "Hello from task!\n",
		stderrContent: "task stderr output\n",
	}

	wdlPath := filepath.Join(testdataDir(), "valid_workflow.wdl")
	code, _, stderr := runArgsWithRunner(
		[]string{"run", "--verbose", wdlPath},
		runner,
	)
	if code != 0 {
		t.Errorf("expected exit 0 with --verbose, got %d (stderr: %q)", code, stderr)
	}
	if !strings.Contains(stderr, "Hello from task!") {
		t.Errorf("expected stderr to contain task stdout content, got %q", stderr)
	}
	if !strings.Contains(stderr, "task stderr output") {
		t.Errorf("expected stderr to contain task stderr content, got %q", stderr)
	}
}

// TestRunWithoutVerboseDoesNotPrintTaskOutput tests that without --verbose,
// task stdout/stderr is NOT printed to the terminal stderr.
func TestRunWithoutVerboseDoesNotPrintTaskOutput(t *testing.T) {
	runner := &verboseMockRunner{
		stdoutContent: "Hello from task!\n",
		stderrContent: "task stderr output\n",
	}

	wdlPath := filepath.Join(testdataDir(), "valid_workflow.wdl")
	code, _, stderr := runArgsWithRunner(
		[]string{"run", wdlPath},
		runner,
	)
	if code != 0 {
		t.Errorf("expected exit 0 without --verbose, got %d (stderr: %q)", code, stderr)
	}
	if strings.Contains(stderr, "Hello from task!") {
		t.Errorf("expected stderr NOT to contain task stdout content without --verbose, got %q", stderr)
	}
	if strings.Contains(stderr, "task stderr output") {
		t.Errorf("expected stderr NOT to contain task stderr content without --verbose, got %q", stderr)
	}
	// Only the execution directory line should be in stderr.
	if !strings.Contains(stderr, "Execution directory:") {
		t.Errorf("expected stderr to contain 'Execution directory:', got %q", stderr)
	}
}
