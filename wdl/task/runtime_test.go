package task_test

import (
	"testing"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/eval"
	"github.com/bioflowy/flowy/wdl/task"
	"github.com/bioflowy/flowy/wdl/types"
)

// makeStringLit creates an ast.StringExpr with a single raw part.
func makeStringLit(s string) *ast.StringExpr {
	return &ast.StringExpr{
		Parts: []ast.StringPart{
			&ast.RawStringPart{Val: s},
		},
	}
}

// makeIntLit creates an ast.IntLit.
func makeIntLit(n int64) *ast.IntLit {
	return &ast.IntLit{Value: n}
}

func TestParseRuntime_DockerImage(t *testing.T) {
	taskAST := &ast.Task{
		Runtime: map[string]ast.Expr{
			"docker": makeStringLit("ubuntu:22.04"),
		},
	}
	env := eval.NewEnv(nil)
	cfg, err := task.ParseRuntime(taskAST, env)
	if err != nil {
		t.Fatalf("ParseRuntime error: %v", err)
	}
	if cfg.Image != "ubuntu:22.04" {
		t.Errorf("Image = %q, want %q", cfg.Image, "ubuntu:22.04")
	}
}

func TestParseRuntime_Default(t *testing.T) {
	taskAST := &ast.Task{
		Runtime: map[string]ast.Expr{},
	}
	env := eval.NewEnv(nil)
	cfg, err := task.ParseRuntime(taskAST, env)
	if err != nil {
		t.Fatalf("ParseRuntime error: %v", err)
	}
	if cfg.Image != "ubuntu:latest" {
		t.Errorf("Image = %q, want %q", cfg.Image, "ubuntu:latest")
	}
}

func TestParseRuntime_CPU(t *testing.T) {
	taskAST := &ast.Task{
		Runtime: map[string]ast.Expr{
			"cpu": makeIntLit(4),
		},
	}
	env := eval.NewEnv(nil)
	cfg, err := task.ParseRuntime(taskAST, env)
	if err != nil {
		t.Fatalf("ParseRuntime error: %v", err)
	}
	want := int64(4 * 1_000_000_000)
	if cfg.NanoCPUs != want {
		t.Errorf("NanoCPUs = %d, want %d", cfg.NanoCPUs, want)
	}
}

func TestParseRuntime_Memory(t *testing.T) {
	taskAST := &ast.Task{
		Runtime: map[string]ast.Expr{
			"memory": makeStringLit("2 GiB"),
		},
	}
	env := eval.NewEnv(nil)
	cfg, err := task.ParseRuntime(taskAST, env)
	if err != nil {
		t.Fatalf("ParseRuntime error: %v", err)
	}
	want := int64(2 * 1024 * 1024 * 1024)
	if cfg.Memory != want {
		t.Errorf("Memory = %d, want %d", cfg.Memory, want)
	}
}

func TestParseRuntime_RequirementsFirst(t *testing.T) {
	// Requirements takes precedence over Runtime
	taskAST := &ast.Task{
		Runtime: map[string]ast.Expr{
			"docker": makeStringLit("ubuntu:22.04"),
		},
		Requirements: map[string]ast.Expr{
			"container": makeStringLit("debian:bullseye"),
		},
	}
	env := eval.NewEnv(nil)
	cfg, err := task.ParseRuntime(taskAST, env)
	if err != nil {
		t.Fatalf("ParseRuntime error: %v", err)
	}
	if cfg.Image != "debian:bullseye" {
		t.Errorf("Image = %q, want %q (Requirements should override Runtime)", cfg.Image, "debian:bullseye")
	}
}

func TestParseRuntime_ContainerKeyPreferred(t *testing.T) {
	// "container" key preferred over "docker" key
	taskAST := &ast.Task{
		Runtime: map[string]ast.Expr{
			"docker":    makeStringLit("ubuntu:22.04"),
			"container": makeStringLit("alpine:3.18"),
		},
	}
	env := eval.NewEnv(nil)
	cfg, err := task.ParseRuntime(taskAST, env)
	if err != nil {
		t.Fatalf("ParseRuntime error: %v", err)
	}
	if cfg.Image != "alpine:3.18" {
		t.Errorf("Image = %q, want %q ('container' key should be preferred)", cfg.Image, "alpine:3.18")
	}
}

// makeOptionalType wraps a type as optional.
func makeOptionalType(t types.Type) types.Type {
	return t.WithOptional(true)
}
