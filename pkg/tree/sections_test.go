package tree

import (
	"testing"

	"github.com/bioflowy/flowy/pkg/env"
	"github.com/bioflowy/flowy/pkg/errors"
	"github.com/bioflowy/flowy/pkg/expr"
	"github.com/bioflowy/flowy/pkg/types"
)

func TestScatter(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Create collection expression
	collection := expr.NewArrayLiteral([]expr.Expr{
		expr.NewStringLiteral("a", pos),
		expr.NewStringLiteral("b", pos),
		expr.NewStringLiteral("c", pos),
	}, pos)

	// Create scatter body
	body := []WorkflowNode{
		NewDecl("item_result", types.NewString(false), expr.NewIdentifier("item", pos), pos),
	}

	scatter := NewScatter("item", collection, body, pos)

	if scatter.Variable != "item" {
		t.Errorf("Expected variable 'item', got '%s'", scatter.Variable)
	}

	if scatter.Collection != collection {
		t.Errorf("Expected collection %v, got %v", collection, scatter.Collection)
	}

	if len(scatter.Body) != 1 {
		t.Errorf("Expected 1 body node, got %d", len(scatter.Body))
	}

	expectedName := "scatter_item"
	if scatter.WorkflowNodeName() != expectedName {
		t.Errorf("Expected workflow node name '%s', got '%s'", expectedName, scatter.WorkflowNodeName())
	}
}

func TestScatterGetters(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	collection := expr.NewIdentifier("my_array", pos)
	body := []WorkflowNode{
		NewDecl("result", types.NewInt(false), nil, pos),
	}

	scatter := NewScatter("x", collection, body, pos)

	if scatter.GetScatterVariable() != "x" {
		t.Errorf("Expected scatter variable 'x', got '%s'", scatter.GetScatterVariable())
	}

	if scatter.GetCollection() != collection {
		t.Errorf("Expected collection %v, got %v", collection, scatter.GetCollection())
	}

	if len(scatter.GetBody()) != 1 {
		t.Errorf("Expected 1 body node, got %d", len(scatter.GetBody()))
	}
}

func TestScatterDependencies(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	typeEnv := env.NewBindings[types.Base]()

	// Scatter with literal collection - no dependencies
	literalCollection := expr.NewArrayLiteral([]expr.Expr{
		expr.NewIntLiteral(1, pos),
		expr.NewIntLiteral(2, pos),
	}, pos)

	scatter1 := NewScatter("i", literalCollection, []WorkflowNode{}, pos)

	deps1, err := scatter1.Dependencies(typeEnv)
	if err != nil {
		t.Errorf("Unexpected error getting dependencies: %v", err)
	}

	if len(deps1) != 0 {
		t.Errorf("Expected no dependencies for literal collection, got %d", len(deps1))
	}

	// Scatter with variable collection - should have dependencies
	// Note: The current implementation returns empty deps, but in a full
	// implementation this would extract dependencies from the collection expression
	varCollection := expr.NewIdentifier("input_array", pos)
	scatter2 := NewScatter("item", varCollection, []WorkflowNode{}, pos)

	deps2, err := scatter2.Dependencies(typeEnv)
	if err != nil {
		t.Errorf("Unexpected error getting dependencies: %v", err)
	}

	// Current implementation returns empty - this is a placeholder
	if len(deps2) != 0 {
		t.Errorf("Expected no dependencies (placeholder implementation), got %d", len(deps2))
	}
}

func TestScatterAddToTypeEnv(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	collection := expr.NewArrayLiteral([]expr.Expr{}, pos)
	body := []WorkflowNode{
		NewDecl("result", types.NewString(false), nil, pos),
	}

	scatter := NewScatter("item", collection, body, pos)

	initialEnv := env.NewBindings[types.Base]()
	newEnv, err := scatter.AddToTypeEnv(initialEnv)
	if err != nil {
		t.Errorf("Unexpected error adding to type environment: %v", err)
	}

	// Current implementation is placeholder - doesn't modify environment
	if newEnv != initialEnv {
		t.Errorf("Expected environment to be unchanged (placeholder implementation)")
	}
}

func TestConditional(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Create condition expression
	condition := expr.NewBooleanLiteral(true, pos)

	// Create conditional body
	body := []WorkflowNode{
		NewDecl("conditional_result", types.NewString(false), expr.NewStringLiteral("yes", pos), pos),
	}

	conditional := NewConditional(condition, body, pos)

	if conditional.Condition != condition {
		t.Errorf("Expected condition %v, got %v", condition, conditional.Condition)
	}

	if len(conditional.Body) != 1 {
		t.Errorf("Expected 1 body node, got %d", len(conditional.Body))
	}

	if conditional.WorkflowNodeName() != "conditional" {
		t.Errorf("Expected workflow node name 'conditional', got '%s'", conditional.WorkflowNodeName())
	}

	expectedString := "if (true)"
	if conditional.String() != expectedString {
		t.Errorf("Expected '%s', got '%s'", expectedString, conditional.String())
	}
}

func TestConditionalGetters(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	condition := expr.NewIdentifier("should_run", pos)
	body := []WorkflowNode{
		NewDecl("optional_result", types.NewInt(false), nil, pos),
	}

	conditional := NewConditional(condition, body, pos)

	if conditional.GetCondition() != condition {
		t.Errorf("Expected condition %v, got %v", condition, conditional.GetCondition())
	}

	if len(conditional.GetBody()) != 1 {
		t.Errorf("Expected 1 body node, got %d", len(conditional.GetBody()))
	}
}

func TestConditionalDependencies(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	typeEnv := env.NewBindings[types.Base]()

	// Conditional with literal condition - no dependencies
	literalCondition := expr.NewBooleanLiteral(false, pos)
	conditional1 := NewConditional(literalCondition, []WorkflowNode{}, pos)

	deps1, err := conditional1.Dependencies(typeEnv)
	if err != nil {
		t.Errorf("Unexpected error getting dependencies: %v", err)
	}

	if len(deps1) != 0 {
		t.Errorf("Expected no dependencies for literal condition, got %d", len(deps1))
	}

	// Conditional with variable condition
	varCondition := expr.NewIdentifier("should_execute", pos)
	conditional2 := NewConditional(varCondition, []WorkflowNode{}, pos)

	deps2, err := conditional2.Dependencies(typeEnv)
	if err != nil {
		t.Errorf("Unexpected error getting dependencies: %v", err)
	}

	// Current implementation returns empty - this is a placeholder
	if len(deps2) != 0 {
		t.Errorf("Expected no dependencies (placeholder implementation), got %d", len(deps2))
	}
}

func TestConditionalAddToTypeEnv(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	condition := expr.NewBooleanLiteral(true, pos)
	body := []WorkflowNode{
		NewDecl("maybe_result", types.NewString(false), nil, pos),
	}

	conditional := NewConditional(condition, body, pos)

	initialEnv := env.NewBindings[types.Base]()
	newEnv, err := conditional.AddToTypeEnv(initialEnv)
	if err != nil {
		t.Errorf("Unexpected error adding to type environment: %v", err)
	}

	// Current implementation is placeholder - doesn't modify environment
	if newEnv != initialEnv {
		t.Errorf("Expected environment to be unchanged (placeholder implementation)")
	}
}

func TestConditionalStringRepresentation(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	tests := []struct {
		name      string
		condition expr.Expr
		expected  string
	}{
		{
			name:      "boolean literal",
			condition: expr.NewBooleanLiteral(true, pos),
			expected:  "if (true)",
		},
		{
			name:      "identifier",
			condition: expr.NewIdentifier("flag", pos),
			expected:  "if (flag)",
		},
		{
			name:      "comparison",
			condition: expr.NewBinaryOp(expr.NewIntLiteral(5, pos), ">", expr.NewIntLiteral(3, pos), pos),
			expected:  "if ((5 > 3))",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conditional := NewConditional(tt.condition, []WorkflowNode{}, pos)
			if conditional.String() != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, conditional.String())
			}
		})
	}
}

// Integration tests

func TestScatterInWorkflow(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Create a workflow with scatter section
	collection := expr.NewIdentifier("input_files", pos)
	scatterBody := []WorkflowNode{
		NewCall("process", "process_file", nil, map[string]expr.Expr{
			"file": expr.NewIdentifier("file", pos),
		}, pos),
	}

	scatter := NewScatter("file", collection, scatterBody, pos)

	// Create workflow with scatter
	inputs := []*Decl{
		NewDecl("input_files", types.NewArray(types.NewString(false), false, false), nil, pos),
	}

	workflow := NewWorkflow("scatter_workflow", inputs, []WorkflowNode{scatter}, []*Decl{}, pos)

	// Validate workflow structure
	if err := ValidateWorkflowStructure(workflow); err != nil {
		t.Errorf("Scatter workflow validation failed: %v", err)
	}
}

func TestConditionalInWorkflow(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Create a workflow with conditional section
	condition := expr.NewIdentifier("run_optional", pos)
	conditionalBody := []WorkflowNode{
		NewCall("optional_task", "task", nil, map[string]expr.Expr{
			"input": expr.NewStringLiteral("conditional", pos),
		}, pos),
	}

	conditional := NewConditional(condition, conditionalBody, pos)

	// Create workflow with conditional
	inputs := []*Decl{
		NewDecl("run_optional", types.NewBoolean(false), nil, pos),
	}

	workflow := NewWorkflow("conditional_workflow", inputs, []WorkflowNode{conditional}, []*Decl{}, pos)

	// Validate workflow structure
	if err := ValidateWorkflowStructure(workflow); err != nil {
		t.Errorf("Conditional workflow validation failed: %v", err)
	}
}

func TestNestedSections(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Create nested scatter and conditional
	innerCondition := expr.NewBooleanLiteral(true, pos)
	innerConditional := NewConditional(innerCondition, []WorkflowNode{
		NewDecl("inner_result", types.NewString(false), nil, pos),
	}, pos)

	scatterBody := []WorkflowNode{innerConditional}
	collection := expr.NewArrayLiteral([]expr.Expr{
		expr.NewIntLiteral(1, pos),
		expr.NewIntLiteral(2, pos),
	}, pos)

	scatter := NewScatter("i", collection, scatterBody, pos)

	// Test that nested structure is properly maintained
	if len(scatter.GetBody()) != 1 {
		t.Errorf("Expected 1 node in scatter body, got %d", len(scatter.GetBody()))
	}

	if _, ok := scatter.GetBody()[0].(*Conditional); !ok {
		t.Errorf("Expected first body node to be Conditional, got %T", scatter.GetBody()[0])
	}
}

func TestSectionSourcePositions(t *testing.T) {
	pos1 := errors.SourcePosition{URI: "test.wdl", Line: 10, Column: 5}
	pos2 := errors.SourcePosition{URI: "test.wdl", Line: 20, Column: 3}

	scatter := NewScatter("x", expr.NewArrayLiteral([]expr.Expr{}, pos1), []WorkflowNode{}, pos1)
	conditional := NewConditional(expr.NewBooleanLiteral(true, pos2), []WorkflowNode{}, pos2)

	if scatter.SourcePosition() != pos1 {
		t.Errorf("Expected scatter position %v, got %v", pos1, scatter.SourcePosition())
	}

	if conditional.SourcePosition() != pos2 {
		t.Errorf("Expected conditional position %v, got %v", pos2, conditional.SourcePosition())
	}
}
