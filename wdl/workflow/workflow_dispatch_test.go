package workflow

import (
	"context"
	"testing"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/task"
	"github.com/bioflowy/flowy/wdl/types"
	"github.com/bioflowy/flowy/wdl/values"
)

// TestRunWorkflowWithDispatch verifies that a custom dispatch function is called
// instead of the default Docker runner, and that workflow outputs are returned.
func TestRunWorkflowWithDispatch(t *testing.T) {
	dispatchCalled := false
	mockDispatch := func(
		_ context.Context, tsk *ast.Task, _ map[string]interface{},
		_ task.ContainerRunner, _ string,
		_ ...map[string]*ast.StructDef,
	) (map[string]values.Value, error) {
		dispatchCalled = true
		return map[string]values.Value{
			"result": values.IntValue{Value: 99},
		}, nil
	}

	myTask := &ast.Task{
		Name: "dispatch_task",
		Inputs: []*ast.Decl{
			{Name: "x", Type: types.IntType{}},
		},
		Outputs: []*ast.Decl{
			{Name: "result", Type: types.IntType{}},
		},
	}
	call := &ast.Call{Callee: []string{"dispatch_task"}, Alias: ""}
	wfOutput := &ast.Decl{Name: "out", Type: types.IntType{}, Expr: memberExpr("dispatch_task", "result")}
	doc := &ast.Document{
		Tasks: []*ast.Task{myTask},
		Workflows: []*ast.Workflow{
			{
				Name:    "wf",
				Body:    []ast.WorkflowNode{call},
				Outputs: []*ast.Decl{wfOutput},
			},
		},
	}

	outputs, err := RunWorkflowWithDispatch(
		context.Background(), doc, "wf", nil, &noopRunner{}, t.TempDir(), mockDispatch,
	)
	if err != nil {
		t.Fatalf("RunWorkflowWithDispatch: %v", err)
	}
	if !dispatchCalled {
		t.Error("expected custom dispatch function to be called, but it was not")
	}
	if v, ok := outputs["out"]; !ok {
		t.Error("expected workflow output 'out' to be set")
	} else if iv, ok := v.(values.IntValue); !ok || iv.Value != 99 {
		t.Errorf("expected out=99, got %v", v)
	}
}

// TestRunWorkflowWithDispatch_ContextCancel verifies that context cancellation
// propagates to the dispatch function and the workflow returns an error.
func TestRunWorkflowWithDispatch_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	blockDispatch := func(
		innerCtx context.Context, _ *ast.Task, _ map[string]interface{},
		_ task.ContainerRunner, _ string,
		_ ...map[string]*ast.StructDef,
	) (map[string]values.Value, error) {
		cancel() // trigger cancellation
		<-innerCtx.Done()
		return nil, innerCtx.Err()
	}

	myTask := &ast.Task{
		Name:   "blocking_task",
		Inputs: nil,
		Outputs: []*ast.Decl{
			{Name: "result", Type: types.IntType{}},
		},
	}
	call := &ast.Call{Callee: []string{"blocking_task"}, Alias: ""}
	doc := &ast.Document{
		Tasks: []*ast.Task{myTask},
		Workflows: []*ast.Workflow{
			{Name: "wf", Body: []ast.WorkflowNode{call}},
		},
	}

	_, err := RunWorkflowWithDispatch(
		ctx, doc, "wf", nil, &noopRunner{}, t.TempDir(), blockDispatch,
	)
	if err == nil {
		t.Error("expected error from cancelled context, got nil")
	}
}
