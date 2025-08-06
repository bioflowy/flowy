package tree

import (
	"testing"

	"github.com/bioflowy/flowy/pkg/env"
	"github.com/bioflowy/flowy/pkg/errors"
	"github.com/bioflowy/flowy/pkg/expr"
	"github.com/bioflowy/flowy/pkg/types"
)

func TestCall(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	inputs := map[string]expr.Expr{
		"input1": expr.NewStringLiteral("hello", pos),
		"input2": expr.NewIntLiteral(42, pos),
	}

	call := NewCall("my_call", "my_task", nil, inputs, pos)

	if call.Name != "my_call" {
		t.Errorf("Expected name 'my_call', got '%s'", call.Name)
	}

	if call.Callee != "my_task" {
		t.Errorf("Expected callee 'my_task', got '%s'", call.Callee)
	}

	if call.Alias != nil {
		t.Errorf("Expected no alias, got %v", call.Alias)
	}

	if len(call.Inputs) != 2 {
		t.Errorf("Expected 2 inputs, got %d", len(call.Inputs))
	}

	if call.WorkflowNodeName() != "my_call" {
		t.Errorf("Expected workflow node name 'my_call', got '%s'", call.WorkflowNodeName())
	}
}

func TestCallWithAlias(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	alias := "task_alias"

	call := NewCall("original_name", "task", &alias, make(map[string]expr.Expr), pos)

	if call.WorkflowNodeName() != "task_alias" {
		t.Errorf("Expected workflow node name 'task_alias', got '%s'", call.WorkflowNodeName())
	}

	if call.GetEffectiveName() != "task_alias" {
		t.Errorf("Expected effective name 'task_alias', got '%s'", call.GetEffectiveName())
	}

	expectedString := "call task as task_alias"
	if call.String() != expectedString {
		t.Errorf("Expected '%s', got '%s'", expectedString, call.String())
	}
}

func TestCallWithoutAlias(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	call := NewCall("task_name", "task", nil, make(map[string]expr.Expr), pos)

	if call.GetEffectiveName() != "task_name" {
		t.Errorf("Expected effective name 'task_name', got '%s'", call.GetEffectiveName())
	}

	expectedString := "call task"
	if call.String() != expectedString {
		t.Errorf("Expected '%s', got '%s'", expectedString, call.String())
	}
}

func TestCallDependencies(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	typeEnv := env.NewBindings[types.Base]()

	// Create call with literal inputs (no dependencies)
	inputs := map[string]expr.Expr{
		"input1": expr.NewStringLiteral("hello", pos),
		"input2": expr.NewIntLiteral(42, pos),
	}

	call := NewCall("simple_call", "task", nil, inputs, pos)

	deps, err := call.Dependencies(typeEnv)
	if err != nil {
		t.Errorf("Unexpected error getting dependencies: %v", err)
	}

	// Should have no dependencies from literal inputs
	if len(deps) != 0 {
		t.Errorf("Expected no dependencies, got %d", len(deps))
	}
}

func TestCallWithAfterDependencies(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	typeEnv := env.NewBindings[types.Base]()

	call := NewCall("dependent_call", "task", nil, make(map[string]expr.Expr), pos)
	call.SetAfter([]string{"previous_task1", "previous_task2"})

	deps, err := call.Dependencies(typeEnv)
	if err != nil {
		t.Errorf("Unexpected error getting dependencies: %v", err)
	}

	if len(deps) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(deps))
	}

	expectedDeps := map[string]bool{"previous_task1": true, "previous_task2": true}
	for _, dep := range deps {
		if !expectedDeps[dep] {
			t.Errorf("Unexpected dependency: %s", dep)
		}
	}
}

func TestCallAddToTypeEnv(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	initialEnv := env.NewBindings[types.Base]()
	call := NewCall("test_call", "task", nil, make(map[string]expr.Expr), pos)

	newEnv, err := call.AddToTypeEnv(initialEnv)
	if err != nil {
		t.Errorf("Unexpected error adding to type environment: %v", err)
	}

	// Check that the call is added to the environment
	if _, err := newEnv.ResolveBinding("test_call"); err != nil {
		t.Error("Expected 'test_call' to be in type environment")
	}

	// Check that original environment is unchanged
	if _, err := initialEnv.ResolveBinding("test_call"); err == nil {
		t.Error("Original environment should not have been modified")
	}
}

func TestCallAddInput(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	call := NewCall("test_call", "task", nil, nil, pos)

	// Add inputs using AddInput method
	input1 := expr.NewStringLiteral("hello", pos)
	input2 := expr.NewIntLiteral(42, pos)

	call.AddInput("input1", input1)
	call.AddInput("input2", input2)

	if len(call.Inputs) != 2 {
		t.Errorf("Expected 2 inputs after adding, got %d", len(call.Inputs))
	}

	if call.Inputs["input1"] != input1 {
		t.Errorf("Expected input1 to be %v, got %v", input1, call.Inputs["input1"])
	}

	if call.Inputs["input2"] != input2 {
		t.Errorf("Expected input2 to be %v, got %v", input2, call.Inputs["input2"])
	}
}

func TestCallSourcePosition(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 5, Column: 10}

	call := NewCall("pos_test", "task", nil, make(map[string]expr.Expr), pos)

	if call.SourcePosition() != pos {
		t.Errorf("Expected position %v, got %v", pos, call.SourcePosition())
	}
}

// Integration tests

func TestCallInWorkflow(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Create a workflow with multiple calls showing dependencies
	call1 := NewCall("step1", "task1", nil, map[string]expr.Expr{
		"input": expr.NewStringLiteral("hello", pos),
	}, pos)

	call2 := NewCall("step2", "task2", nil, map[string]expr.Expr{
		"input": expr.NewIdentifier("step1.output", pos), // Depends on step1
	}, pos)

	call3 := NewCall("step3", "task3", nil, make(map[string]expr.Expr), pos)
	call3.SetAfter([]string{"step1", "step2"}) // Explicit dependency

	nodes := []WorkflowNode{call1, call2, call3}

	// Build dependency graph
	initialEnv := env.NewBindings[types.Base]()
	builder := NewWorkflowGraphBuilder(nodes, initialEnv)
	dependencies, err := builder.BuildDependencyGraph()
	if err != nil {
		t.Errorf("Failed to build dependency graph: %v", err)
	}

	// Check dependencies
	if len(dependencies["step1"]) != 0 {
		t.Errorf("Expected step1 to have no dependencies, got %d", len(dependencies["step1"]))
	}

	// step3 should have explicit dependencies
	if len(dependencies["step3"]) != 2 {
		t.Errorf("Expected step3 to have 2 dependencies, got %d", len(dependencies["step3"]))
	}
}

func TestCallTypeEnvironmentBuilding(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Create sequential calls
	call1 := NewCall("first", "task1", nil, make(map[string]expr.Expr), pos)
	call2 := NewCall("second", "task2", nil, make(map[string]expr.Expr), pos)

	nodes := []WorkflowNode{call1, call2}

	// Resolve types
	initialEnv := env.NewBindings[types.Base]()
	finalEnv, err := ResolveWorkflowTypes(nodes, initialEnv)
	if err != nil {
		t.Errorf("Failed to resolve workflow types: %v", err)
	}

	// Both calls should be in the final environment
	if _, err := finalEnv.ResolveBinding("first"); err != nil {
		t.Error("Expected 'first' in final environment")
	}
	if _, err := finalEnv.ResolveBinding("second"); err != nil {
		t.Error("Expected 'second' in final environment")
	}
}

func TestCallValidation(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Test call validation in workflow context
	call1 := NewCall("call1", "task", nil, make(map[string]expr.Expr), pos)
	call2 := NewCall("call2", "task", nil, make(map[string]expr.Expr), pos)

	// Create workflow
	workflow := NewWorkflow("test_workflow", []*Decl{}, []WorkflowNode{call1, call2}, []*Decl{}, pos)

	// Validate workflow structure
	if err := ValidateWorkflowStructure(workflow); err != nil {
		t.Errorf("Workflow validation failed: %v", err)
	}
}
