package tree

import (
	"testing"

	"github.com/bioflowy/flowy/pkg/env"
	"github.com/bioflowy/flowy/pkg/errors"
	"github.com/bioflowy/flowy/pkg/expr"
	"github.com/bioflowy/flowy/pkg/types"
)

func TestDecl(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Test declaration without initialization
	declType := types.NewString(false)
	decl := NewDecl("test_var", declType, nil, pos)

	if decl.Name != "test_var" {
		t.Errorf("Expected name 'test_var', got '%s'", decl.Name)
	}

	if decl.Type != declType {
		t.Errorf("Expected type %v, got %v", declType, decl.Type)
	}

	if decl.Expr != nil {
		t.Errorf("Expected nil expression, got %v", decl.Expr)
	}

	// Test WorkflowNode interface
	if decl.WorkflowNodeName() != "test_var" {
		t.Errorf("Expected workflow node name 'test_var', got '%s'", decl.WorkflowNodeName())
	}
}

func TestDeclWithInitialization(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Create initialization expression
	initExpr := expr.NewStringLiteral("hello", pos)
	decl := NewDecl("initialized_var", types.NewString(false), initExpr, pos)

	if decl.Expr != initExpr {
		t.Errorf("Expected initialization expression %v, got %v", initExpr, decl.Expr)
	}
}

func TestDeclDependencies(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	typeEnv := env.NewBindings[types.Base]()

	// Test declaration without initialization - should have no dependencies
	decl1 := NewDecl("simple", types.NewString(false), nil, pos)
	deps1, err := decl1.Dependencies(typeEnv)
	if err != nil {
		t.Errorf("Unexpected error getting dependencies: %v", err)
	}
	if len(deps1) != 0 {
		t.Errorf("Expected no dependencies for uninitialized declaration, got %d", len(deps1))
	}

	// Test declaration with literal initialization - should have no dependencies
	initExpr := expr.NewStringLiteral("hello", pos)
	decl2 := NewDecl("literal_init", types.NewString(false), initExpr, pos)
	deps2, err := decl2.Dependencies(typeEnv)
	if err != nil {
		t.Errorf("Unexpected error getting dependencies: %v", err)
	}
	if len(deps2) != 0 {
		t.Errorf("Expected no dependencies for literal initialization, got %d", len(deps2))
	}
}

func TestDeclAddToTypeEnv(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	initialEnv := env.NewBindings[types.Base]()
	declType := types.NewString(false)
	decl := NewDecl("new_var", declType, nil, pos)

	newEnv, err := decl.AddToTypeEnv(initialEnv)
	if err != nil {
		t.Errorf("Unexpected error adding to type environment: %v", err)
	}

	// Check that the new variable is in the environment
	binding, err := newEnv.ResolveBinding("new_var")
	if err != nil {
		t.Error("Expected 'new_var' to be in type environment")
	} else if binding.Value() != declType {
		t.Errorf("Expected type %v, got %v", declType, binding.Value())
	}

	// Check that original environment is unchanged (immutability)
	if _, err := initialEnv.ResolveBinding("new_var"); err == nil {
		t.Error("Original environment should not have been modified")
	}
}

func TestInput(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	decls := []*Decl{
		NewDecl("input1", types.NewString(false), nil, pos),
		NewDecl("input2", types.NewInt(false), nil, pos),
	}

	input := NewInput(decls, pos)

	if len(input.Decls) != 2 {
		t.Errorf("Expected 2 declarations, got %d", len(input.Decls))
	}

	if input.Decls[0].Name != "input1" {
		t.Errorf("Expected first declaration name 'input1', got '%s'", input.Decls[0].Name)
	}

	if input.Decls[1].Name != "input2" {
		t.Errorf("Expected second declaration name 'input2', got '%s'", input.Decls[1].Name)
	}

	if input.SourcePosition() != pos {
		t.Errorf("Expected position %v, got %v", pos, input.SourcePosition())
	}
}

func TestOutput(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Create output declarations with expressions
	outputExpr := expr.NewIdentifier("some_var", pos)
	decls := []*Decl{
		NewDecl("output1", types.NewString(false), outputExpr, pos),
	}

	output := NewOutput(decls, pos)

	if len(output.Decls) != 1 {
		t.Errorf("Expected 1 declaration, got %d", len(output.Decls))
	}

	if output.Decls[0].Name != "output1" {
		t.Errorf("Expected declaration name 'output1', got '%s'", output.Decls[0].Name)
	}

	if output.Decls[0].Expr != outputExpr {
		t.Errorf("Expected expression %v, got %v", outputExpr, output.Decls[0].Expr)
	}
}

func TestMultipleDeclsInTypeEnv(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Create multiple declarations
	decl1 := NewDecl("var1", types.NewString(false), nil, pos)
	decl2 := NewDecl("var2", types.NewInt(false), nil, pos)
	decl3 := NewDecl("var3", types.NewBoolean(false), nil, pos)

	// Add them to type environment sequentially
	env := env.NewBindings[types.Base]()

	env, err := decl1.AddToTypeEnv(env)
	if err != nil {
		t.Errorf("Error adding decl1: %v", err)
	}

	env, err = decl2.AddToTypeEnv(env)
	if err != nil {
		t.Errorf("Error adding decl2: %v", err)
	}

	env, err = decl3.AddToTypeEnv(env)
	if err != nil {
		t.Errorf("Error adding decl3: %v", err)
	}

	// Check all variables are present
	if _, err := env.ResolveBinding("var1"); err != nil {
		t.Error("Expected 'var1' in environment")
	}
	if _, err := env.ResolveBinding("var2"); err != nil {
		t.Error("Expected 'var2' in environment")
	}
	if _, err := env.ResolveBinding("var3"); err != nil {
		t.Error("Expected 'var3' in environment")
	}

	// Check types
	if binding, err := env.ResolveBinding("var1"); err == nil {
		if _, ok := binding.Value().(*types.StringType); !ok {
			t.Errorf("Expected String type for var1, got %T", binding.Value())
		}
	}

	if binding, err := env.ResolveBinding("var2"); err == nil {
		if _, ok := binding.Value().(*types.IntType); !ok {
			t.Errorf("Expected Int type for var2, got %T", binding.Value())
		}
	}

	if binding, err := env.ResolveBinding("var3"); err == nil {
		if _, ok := binding.Value().(*types.BooleanType); !ok {
			t.Errorf("Expected Boolean type for var3, got %T", binding.Value())
		}
	}
}

func TestWalkExpressionTree(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Create a simple expression tree: "hello" + "world"
	left := expr.NewStringLiteral("hello", pos)
	right := expr.NewStringLiteral("world", pos)
	binOp := expr.NewBinaryOp(left, "+", right, pos)

	// Count nodes in the tree
	nodeCount := 0
	walkExpressionTree(binOp, func(e expr.Expr) {
		nodeCount++
	})

	// Should visit: binOp, left, right = 3 nodes
	if nodeCount != 3 {
		t.Errorf("Expected 3 nodes, got %d", nodeCount)
	}

	// Test with nil expression
	walkExpressionTree(nil, func(e expr.Expr) {
		t.Error("Visitor should not be called for nil expression")
	})
}

func TestCollectExpressionDependencies(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	typeEnv := env.NewBindings[types.Base]()

	// Test with literal expression - should have no dependencies
	literal := expr.NewStringLiteral("hello", pos)
	deps := collectExpressionDependencies(literal, typeEnv)
	if len(deps) != 0 {
		t.Errorf("Expected no dependencies for literal, got %d", len(deps))
	}

	// Test with nil expression
	deps = collectExpressionDependencies(nil, typeEnv)
	if len(deps) != 0 {
		t.Errorf("Expected no dependencies for nil expression, got %d", len(deps))
	}
}

// Integration tests

func TestDeclWorkflowIntegration(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Simulate a simple workflow with declarations
	nodes := []WorkflowNode{
		NewDecl("step1", types.NewString(false), expr.NewStringLiteral("hello", pos), pos),
		NewDecl("step2", types.NewInt(false), expr.NewIntLiteral(42, pos), pos),
	}

	// Build type environment
	initialEnv := env.NewBindings[types.Base]()
	finalEnv, err := ResolveWorkflowTypes(nodes, initialEnv)
	if err != nil {
		t.Errorf("Failed to resolve workflow types: %v", err)
	}

	// Check both declarations are in final environment
	if _, err := finalEnv.ResolveBinding("step1"); err != nil {
		t.Error("Expected 'step1' in final environment")
	}
	if _, err := finalEnv.ResolveBinding("step2"); err != nil {
		t.Error("Expected 'step2' in final environment")
	}

	// Build dependency graph
	builder := NewWorkflowGraphBuilder(nodes, initialEnv)
	deps, err := builder.BuildDependencyGraph()
	if err != nil {
		t.Errorf("Failed to build dependency graph: %v", err)
	}

	// Check dependencies
	if len(deps["step1"]) != 0 {
		t.Errorf("Expected no dependencies for step1, got %d", len(deps["step1"]))
	}
	if len(deps["step2"]) != 0 {
		t.Errorf("Expected no dependencies for step2, got %d", len(deps["step2"]))
	}
}
