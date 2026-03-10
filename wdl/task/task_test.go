package task_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/task"
	"github.com/bioflowy/flowy/wdl/types"
	"github.com/bioflowy/flowy/wdl/values"
)

// ShellRunner is a mock ContainerRunner that executes the command script directly
// on the host, ignoring Docker-specific semantics. It finds the _command bind
// in req.Binds, reads the script file, and runs it via bash.
//
// This enables end-to-end task execution tests without a live Docker daemon.
type ShellRunner struct{}

// Run implements ContainerRunner for ShellRunner.
// It finds the host path for /mnt/inputs/_command and /mnt/outputs from req.Binds,
// then executes the command script via bash with cmd.Dir set to the work directory.
func (r *ShellRunner) Run(ctx context.Context, req task.RunRequest) (task.RunResult, error) {
	// Find the script path from binds: look for ::/mnt/inputs/_command
	var scriptPath string
	var workDir string
	for _, bind := range req.Binds {
		parts := strings.Split(bind, ":")
		if len(parts) < 2 {
			continue
		}
		hostPath := parts[0]
		containerPath := parts[1]
		if containerPath == "/mnt/inputs/_command" {
			scriptPath = hostPath
		}
		if containerPath == "/mnt/outputs" {
			workDir = hostPath
		}
	}
	if scriptPath == "" {
		return task.RunResult{}, nil
	}

	cmd := exec.CommandContext(ctx, "bash", scriptPath)
	if workDir != "" {
		cmd.Dir = workDir
	}
	// Pass environment variables from the request
	cmd.Env = append(os.Environ(), req.Env...)

	stdout, err := cmd.Output()
	if err != nil {
		// ExitError means the script ran but exited non-zero
		if exitErr, ok := err.(*exec.ExitError); ok {
			return task.RunResult{
				ExitCode: exitErr.ExitCode(),
				Stdout:   stdout,
				Stderr:   exitErr.Stderr,
			}, nil
		}
		return task.RunResult{}, err
	}

	return task.RunResult{
		ExitCode: 0,
		Stdout:   stdout,
		Stderr:   nil,
	}, nil
}

// makeEchoTask builds an ast.Task that echoes a fixed string and captures stdout.
//
//	task echo_task {
//	  command { echo "hello" }
//	  output { String msg = read_string(stdout()) }
//	}
func makeEchoTask() *ast.Task {
	return &ast.Task{
		Name: "echo_task",
		Command: &ast.CommandExpr{
			Parts: []ast.StringPart{
				&ast.RawStringPart{Val: "\necho \"hello\"\n"},
			},
		},
		Outputs: []*ast.Decl{
			{
				Name: "msg",
				Type: types.StringType{},
				Expr: &ast.ApplyExpr{
					Name: "read_string",
					Args: []ast.Expr{
						&ast.ApplyExpr{Name: "stdout", Args: nil},
					},
				},
			},
		},
	}
}

// TestRunTaskEcho verifies that a simple echo task returns stdout as output.
func TestRunTaskEcho(t *testing.T) {
	execRoot := t.TempDir()
	runner := &ShellRunner{}

	outputs, err := task.RunTask(context.Background(), makeEchoTask(), map[string]interface{}{}, runner, execRoot)
	if err != nil {
		t.Fatalf("RunTask error: %v", err)
	}

	v, ok := outputs["msg"]
	if !ok {
		t.Fatal("output 'msg' not found")
	}
	sv, ok := v.(values.StringValue)
	if !ok {
		t.Fatalf("expected StringValue, got %T", v)
	}
	// read_string strips trailing newline
	if sv.Value != "hello" {
		t.Errorf("msg = %q, want %q", sv.Value, "hello")
	}
}

// TestRunTaskWithInputs verifies that Int and String inputs are bound and usable
// in the command template.
func TestRunTaskWithInputs(t *testing.T) {
	execRoot := t.TempDir()
	runner := &ShellRunner{}

	taskAST := &ast.Task{
		Name: "input_task",
		Inputs: []*ast.Decl{
			{Name: "count", Type: types.IntType{}},
			{Name: "label", Type: types.StringType{}},
		},
		Command: &ast.CommandExpr{
			Parts: []ast.StringPart{
				&ast.RawStringPart{Val: "\necho "},
				&ast.PlaceholderPart{Expr: &ast.IdentExpr{Name: "label"}},
				&ast.RawStringPart{Val: " "},
				&ast.PlaceholderPart{Expr: &ast.IdentExpr{Name: "count"}},
				&ast.RawStringPart{Val: "\n"},
			},
		},
		Outputs: []*ast.Decl{
			{
				Name: "result",
				Type: types.StringType{},
				Expr: &ast.ApplyExpr{
					Name: "read_string",
					Args: []ast.Expr{
						&ast.ApplyExpr{Name: "stdout", Args: nil},
					},
				},
			},
		},
	}

	outputs, err := task.RunTask(context.Background(), taskAST, map[string]interface{}{
		"count": float64(42),
		"label": "sample",
	}, runner, execRoot)
	if err != nil {
		t.Fatalf("RunTask error: %v", err)
	}

	v, ok := outputs["result"]
	if !ok {
		t.Fatal("output 'result' not found")
	}
	sv, ok := v.(values.StringValue)
	if !ok {
		t.Fatalf("expected StringValue, got %T", v)
	}
	if sv.Value != "sample 42" {
		t.Errorf("result = %q, want %q", sv.Value, "sample 42")
	}
}

// TestRunTaskFailure verifies that a non-zero exit code returns a *TaskError.
func TestRunTaskFailure(t *testing.T) {
	execRoot := t.TempDir()
	runner := &ShellRunner{}

	taskAST := &ast.Task{
		Name: "failing_task",
		Command: &ast.CommandExpr{
			Parts: []ast.StringPart{
				&ast.RawStringPart{Val: "\nexit 1\n"},
			},
		},
	}

	_, err := task.RunTask(context.Background(), taskAST, map[string]interface{}{}, runner, execRoot)
	if err == nil {
		t.Fatal("expected error for non-zero exit, got nil")
	}

	te, ok := err.(*task.TaskError)
	if !ok {
		t.Fatalf("expected *TaskError, got %T: %v", err, err)
	}
	if te.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", te.ExitCode)
	}
}

// TestRunTaskMissingInput verifies that a missing required input returns an error
// before the container is executed.
func TestRunTaskMissingInput(t *testing.T) {
	execRoot := t.TempDir()
	runner := &ShellRunner{}

	taskAST := &ast.Task{
		Name: "missing_input_task",
		Inputs: []*ast.Decl{
			{Name: "required", Type: types.StringType{}},
		},
		Command: &ast.CommandExpr{
			Parts: []ast.StringPart{
				&ast.RawStringPart{Val: "\necho hello\n"},
			},
		},
	}

	_, err := task.RunTask(context.Background(), taskAST, map[string]interface{}{}, runner, execRoot)
	if err == nil {
		t.Fatal("expected error for missing required input, got nil")
	}
	// Should NOT be a TaskError (that would mean container ran)
	if _, ok := err.(*task.TaskError); ok {
		t.Errorf("error should not be TaskError for missing input; got TaskError: %v", err)
	}
}

// TestRunTaskFileOutput verifies that a task producing a file output returns a
// FileValue with the host path.
func TestRunTaskFileOutput(t *testing.T) {
	execRoot := t.TempDir()
	runner := &ShellRunner{}

	taskAST := &ast.Task{
		Name: "file_output_task",
		Command: &ast.CommandExpr{
			Parts: []ast.StringPart{
				&ast.RawStringPart{Val: "\necho \"data\" > result.txt\n"},
			},
		},
		Outputs: []*ast.Decl{
			{
				Name: "out_file",
				Type: types.FileType{},
				// Use glob() to find result.txt in the work directory
				Expr: &ast.ApplyExpr{
					Name: "glob",
					Args: []ast.Expr{
						&ast.StringExpr{
							Parts: []ast.StringPart{
								&ast.RawStringPart{Val: "result.txt"},
							},
						},
					},
				},
			},
		},
	}

	outputs, err := task.RunTask(context.Background(), taskAST, map[string]interface{}{}, runner, execRoot)
	if err != nil {
		t.Fatalf("RunTask error: %v", err)
	}

	v, ok := outputs["out_file"]
	if !ok {
		t.Fatal("output 'out_file' not found")
	}

	// glob() returns Array[File]
	av, ok := v.(values.ArrayValue)
	if !ok {
		t.Fatalf("expected ArrayValue from glob(), got %T", v)
	}
	if len(av.Items) == 0 {
		t.Fatal("glob() returned empty array; expected result.txt")
	}

	fv, ok := av.Items[0].(values.FileValue)
	if !ok {
		t.Fatalf("expected FileValue in array, got %T", av.Items[0])
	}

	// Verify it's a host path (within execRoot) and not a container path
	if !filepath.IsAbs(fv.Value) {
		t.Errorf("expected absolute host path, got %q", fv.Value)
	}
	if strings.HasPrefix(fv.Value, "/mnt/") {
		t.Errorf("file value looks like a container path: %q", fv.Value)
	}
	// Verify the file actually exists
	if _, statErr := os.Stat(fv.Value); os.IsNotExist(statErr) {
		t.Errorf("output file does not exist at %q", fv.Value)
	}
}
