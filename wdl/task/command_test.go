package task_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/eval"
	"github.com/bioflowy/flowy/wdl/task"
	"github.com/bioflowy/flowy/wdl/types"
	"github.com/bioflowy/flowy/wdl/values"
)

// makeCommandExpr builds a CommandExpr from a single raw text part.
func makeCommandExpr(raw string) *ast.CommandExpr {
	return &ast.CommandExpr{
		Parts: []ast.StringPart{
			&ast.RawStringPart{Val: raw},
		},
	}
}

// makeCommandExprWithPlaceholder builds a CommandExpr: prefix + ${ident} + suffix.
func makeCommandExprWithPlaceholder(prefix, identName, suffix string) *ast.CommandExpr {
	return &ast.CommandExpr{
		Parts: []ast.StringPart{
			&ast.RawStringPart{Val: prefix},
			&ast.PlaceholderPart{Expr: &ast.IdentExpr{Name: identName}},
			&ast.RawStringPart{Val: suffix},
		},
	}
}

// TestBuildCommand_SimpleCommand verifies that buildCommand writes the expanded
// command to execDir/command and returns the path.
func TestBuildCommand_SimpleCommand(t *testing.T) {
	execDir := t.TempDir()
	taskAST := &ast.Task{
		Name:    "test_task",
		Command: makeCommandExpr("\necho hello\n"),
	}
	env := eval.NewEnv(nil)
	pm := task.NewPathMap()

	scriptPath, err := task.BuildCommand(taskAST, env, pm, execDir, "")
	if err != nil {
		t.Fatalf("BuildCommand error: %v", err)
	}

	// Should return path to execDir/command
	wantPath := filepath.Join(execDir, "command")
	if scriptPath != wantPath {
		t.Errorf("scriptPath = %q, want %q", scriptPath, wantPath)
	}

	// File should exist and contain the expanded command
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if !strings.Contains(string(data), "echo hello") {
		t.Errorf("command file content %q does not contain 'echo hello'", string(data))
	}
}

// TestBuildCommand_ContainerPath verifies that file input values in the env are
// translated to container paths in the expanded command.
func TestBuildCommand_ContainerPath(t *testing.T) {
	execDir := t.TempDir()
	pm := task.NewPathMap()

	// Register a host file and record its container path
	containerPath := pm.AddFile("/host/data/input.bam")

	// Set up env with file value using host path (pre-translation)
	env := eval.NewEnv(nil)
	env.Define("bam", values.FileValue{Type: types.FileType{}, Value: "/host/data/input.bam"})

	taskAST := &ast.Task{
		Name:    "test_task",
		Command: makeCommandExprWithPlaceholder("\nsamtools view ", "bam", "\n"),
	}

	scriptPath, err := task.BuildCommand(taskAST, env, pm, execDir, "")
	if err != nil {
		t.Fatalf("BuildCommand error: %v", err)
	}

	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	content := string(data)

	// Command must use container path, NOT host path
	if strings.Contains(content, "/host/data/input.bam") {
		t.Errorf("command file contains host path %q; should use container path %q", "/host/data/input.bam", containerPath)
	}
	if !strings.Contains(content, containerPath) {
		t.Errorf("command file %q does not contain container path %q", content, containerPath)
	}
}

// TestBuildCommand_WritesFile verifies that the script file is written to disk at execDir/command.
func TestBuildCommand_WritesFile(t *testing.T) {
	execDir := t.TempDir()
	taskAST := &ast.Task{
		Name:    "test_task",
		Command: makeCommandExpr("\necho test\n"),
	}
	env := eval.NewEnv(nil)
	pm := task.NewPathMap()

	_, err := task.BuildCommand(taskAST, env, pm, execDir, "")
	if err != nil {
		t.Fatalf("BuildCommand error: %v", err)
	}

	expectedPath := filepath.Join(execDir, "command")
	if _, statErr := os.Stat(expectedPath); os.IsNotExist(statErr) {
		t.Errorf("command file not found at %q", expectedPath)
	}
}

// TestCollectOutputs_StringOutput verifies that a String output declaration is
// correctly evaluated and returned.
func TestCollectOutputs_StringOutput(t *testing.T) {
	taskAST := &ast.Task{
		Name: "test_task",
		Outputs: []*ast.Decl{
			{
				Name: "result",
				Type: types.StringType{},
				Expr: &ast.StringExpr{
					Parts: []ast.StringPart{&ast.RawStringPart{Val: "hello output"}},
				},
			},
		},
	}

	env := eval.NewEnv(nil)
	ctx := eval.ExecContext{WorkDir: "/tmp/work"}
	pm := task.NewPathMap()

	outputs, err := task.CollectOutputs(taskAST, env, ctx, pm)
	if err != nil {
		t.Fatalf("CollectOutputs error: %v", err)
	}

	v, ok := outputs["result"]
	if !ok {
		t.Fatal("output 'result' not found")
	}
	sv, ok := v.(values.StringValue)
	if !ok {
		t.Fatalf("expected StringValue, got %T", v)
	}
	if sv.Value != "hello output" {
		t.Errorf("result = %q, want %q", sv.Value, "hello output")
	}
}

// TestCollectOutputs_FileOutput verifies that a File output returns a FileValue
// with the host path (not container path).
func TestCollectOutputs_FileOutput(t *testing.T) {
	execDir := t.TempDir()
	workDir := filepath.Join(execDir, "work")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Create a result file in the work dir
	resultPath := filepath.Join(workDir, "result.txt")
	if err := os.WriteFile(resultPath, []byte("data"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	taskAST := &ast.Task{
		Name: "test_task",
		Outputs: []*ast.Decl{
			{
				Name: "out_file",
				Type: types.FileType{},
				Expr: &ast.StringExpr{
					Parts: []ast.StringPart{
						&ast.RawStringPart{Val: workDir + "/result.txt"},
					},
				},
			},
		},
	}

	env := eval.NewEnv(nil)
	ctx := eval.ExecContext{WorkDir: workDir}
	pm := task.NewPathMap()

	outputs, err := task.CollectOutputs(taskAST, env, ctx, pm)
	if err != nil {
		t.Fatalf("CollectOutputs error: %v", err)
	}

	v, ok := outputs["out_file"]
	if !ok {
		t.Fatal("output 'out_file' not found")
	}
	fv, ok := v.(values.FileValue)
	if !ok {
		t.Fatalf("expected FileValue, got %T", v)
	}
	// Must be host path
	if fv.Value != resultPath {
		t.Errorf("out_file = %q, want %q", fv.Value, resultPath)
	}
}

// TestCollectOutputs_MultipleOutputs verifies that multiple output declarations
// are all evaluated and returned.
func TestCollectOutputs_MultipleOutputs(t *testing.T) {
	taskAST := &ast.Task{
		Name: "test_task",
		Outputs: []*ast.Decl{
			{
				Name: "a",
				Type: types.StringType{},
				Expr: &ast.StringExpr{
					Parts: []ast.StringPart{&ast.RawStringPart{Val: "val_a"}},
				},
			},
			{
				Name: "b",
				Type: types.StringType{},
				Expr: &ast.StringExpr{
					Parts: []ast.StringPart{&ast.RawStringPart{Val: "val_b"}},
				},
			},
		},
	}

	env := eval.NewEnv(nil)
	ctx := eval.ExecContext{WorkDir: "/tmp/work"}
	pm := task.NewPathMap()

	outputs, err := task.CollectOutputs(taskAST, env, ctx, pm)
	if err != nil {
		t.Fatalf("CollectOutputs error: %v", err)
	}

	if len(outputs) != 2 {
		t.Errorf("expected 2 outputs, got %d", len(outputs))
	}
	for _, key := range []string{"a", "b"} {
		if _, ok := outputs[key]; !ok {
			t.Errorf("output %q not found", key)
		}
	}
}
