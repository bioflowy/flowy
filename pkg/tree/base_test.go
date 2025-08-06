package tree

import (
	"testing"

	"github.com/bioflowy/flowy/pkg/env"
	"github.com/bioflowy/flowy/pkg/errors"
	"github.com/bioflowy/flowy/pkg/expr"
	"github.com/bioflowy/flowy/pkg/types"
)

func TestBaseSourceNode(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	node := NewBaseSourceNode(pos)

	if node.SourcePosition() != pos {
		t.Errorf("Expected position %v, got %v", pos, node.SourcePosition())
	}
}

func TestDocument(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Create test components
	imports := []*Import{
		NewImport("lib.wdl", "lib", nil, pos),
	}

	tasks := []*Task{
		NewTask("test_task", []*Decl{}, []*Decl{}, nil, pos),
	}

	structs := []*StructTypeDef{
		NewStructTypeDef("TestStruct", []*StructMember{}, pos),
	}

	workflow := NewWorkflow("test_workflow", []*Decl{}, []WorkflowNode{}, []*Decl{}, pos)

	doc := NewDocument(imports, workflow, tasks, structs, pos)

	// Test basic properties
	if len(doc.Imports) != 1 {
		t.Errorf("Expected 1 import, got %d", len(doc.Imports))
	}

	if len(doc.Tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(doc.Tasks))
	}

	if len(doc.Structs) != 1 {
		t.Errorf("Expected 1 struct, got %d", len(doc.Structs))
	}

	if doc.Workflow == nil {
		t.Error("Expected workflow to be present")
	}

	if doc.SourcePosition() != pos {
		t.Errorf("Expected position %v, got %v", pos, doc.SourcePosition())
	}
}

func TestImport(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	alias := "mylib"

	imp := NewImport("lib.wdl", "lib", &alias, pos)

	if imp.URI != "lib.wdl" {
		t.Errorf("Expected URI 'lib.wdl', got '%s'", imp.URI)
	}

	if imp.Namespace != "lib" {
		t.Errorf("Expected namespace 'lib', got '%s'", imp.Namespace)
	}

	if imp.Alias == nil || *imp.Alias != "mylib" {
		t.Errorf("Expected alias 'mylib', got %v", imp.Alias)
	}

	// Test without alias
	imp2 := NewImport("other.wdl", "other", nil, pos)
	if imp2.Alias != nil {
		t.Errorf("Expected no alias, got %v", imp2.Alias)
	}
}

func TestTask(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	inputs := []*Decl{
		NewDecl("input1", types.NewString(false), nil, pos),
	}

	outputs := []*Decl{
		NewDecl("output1", types.NewString(false), nil, pos),
	}

	task := NewTask("test_task", inputs, outputs, nil, pos)

	if task.Name != "test_task" {
		t.Errorf("Expected name 'test_task', got '%s'", task.Name)
	}

	if len(task.Inputs) != 1 {
		t.Errorf("Expected 1 input, got %d", len(task.Inputs))
	}

	if len(task.Outputs) != 1 {
		t.Errorf("Expected 1 output, got %d", len(task.Outputs))
	}

	if task.Runtime == nil {
		t.Error("Expected runtime map to be initialized")
	}

	if task.Meta == nil {
		t.Error("Expected meta map to be initialized")
	}

	if task.ParameterMeta == nil {
		t.Error("Expected parameter meta map to be initialized")
	}
}

func TestWorkflow(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	inputs := []*Decl{
		NewDecl("workflow_input", types.NewString(false), nil, pos),
	}

	outputs := []*Decl{
		NewDecl("workflow_output", types.NewString(false), nil, pos),
	}

	body := []WorkflowNode{
		NewDecl("intermediate", types.NewInt(false), nil, pos),
	}

	workflow := NewWorkflow("test_workflow", inputs, body, outputs, pos)

	if workflow.Name != "test_workflow" {
		t.Errorf("Expected name 'test_workflow', got '%s'", workflow.Name)
	}

	if len(workflow.Inputs) != 1 {
		t.Errorf("Expected 1 input, got %d", len(workflow.Inputs))
	}

	if len(workflow.Outputs) != 1 {
		t.Errorf("Expected 1 output, got %d", len(workflow.Outputs))
	}

	if len(workflow.Body) != 1 {
		t.Errorf("Expected 1 body node, got %d", len(workflow.Body))
	}
}

func TestStructTypeDef(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	members := []*StructMember{
		NewStructMember("field1", types.NewString(false), pos),
		NewStructMember("field2", types.NewInt(false), pos),
	}

	structDef := NewStructTypeDef("TestStruct", members, pos)

	if structDef.Name != "TestStruct" {
		t.Errorf("Expected name 'TestStruct', got '%s'", structDef.Name)
	}

	if len(structDef.Members) != 2 {
		t.Errorf("Expected 2 members, got %d", len(structDef.Members))
	}

	// Test member details
	if structDef.Members[0].Name != "field1" {
		t.Errorf("Expected first member name 'field1', got '%s'", structDef.Members[0].Name)
	}

	if structDef.Members[1].Name != "field2" {
		t.Errorf("Expected second member name 'field2', got '%s'", structDef.Members[1].Name)
	}
}

func TestStructMember(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	memberType := types.NewString(false)

	member := NewStructMember("test_field", memberType, pos)

	if member.Name != "test_field" {
		t.Errorf("Expected name 'test_field', got '%s'", member.Name)
	}

	if member.Type != memberType {
		t.Errorf("Expected type %v, got %v", memberType, member.Type)
	}

	if member.SourcePosition() != pos {
		t.Errorf("Expected position %v, got %v", pos, member.SourcePosition())
	}
}

func TestTaskCommand(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Create a simple command expression (using string literal for simplicity)
	commandExpr := expr.NewStringLiteral("echo hello", pos)
	taskCmd := NewTaskCommand(commandExpr, pos)

	if taskCmd.Command != commandExpr {
		t.Errorf("Expected command expression %v, got %v", commandExpr, taskCmd.Command)
	}

	if taskCmd.SourcePosition() != pos {
		t.Errorf("Expected position %v, got %v", pos, taskCmd.SourcePosition())
	}
}

// Integration tests

func TestDocumentValidation(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Test valid document
	validDoc := NewDocument([]*Import{}, nil, []*Task{}, []*StructTypeDef{}, pos)
	if err := ValidateDocumentStructure(validDoc); err != nil {
		t.Errorf("Expected valid document to pass validation, got error: %v", err)
	}

	// Test document with duplicate task names
	tasks := []*Task{
		NewTask("duplicate", []*Decl{}, []*Decl{}, nil, pos),
		NewTask("duplicate", []*Decl{}, []*Decl{}, nil, pos),
	}
	invalidDoc := NewDocument([]*Import{}, nil, tasks, []*StructTypeDef{}, pos)
	if err := ValidateDocumentStructure(invalidDoc); err == nil {
		t.Error("Expected duplicate task names to fail validation")
	}
}

func TestWorkflowWithInputsOutputs(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Create workflow with inputs and outputs
	inputs := []*Decl{
		NewDecl("input_str", types.NewString(false), nil, pos),
		NewDecl("input_int", types.NewInt(false), nil, pos),
	}

	outputs := []*Decl{
		NewDecl("output_str", types.NewString(false), nil, pos),
	}

	// Simple body with a declaration
	body := []WorkflowNode{
		NewDecl("intermediate", types.NewString(false), nil, pos),
	}

	workflow := NewWorkflow("complex_workflow", inputs, body, outputs, pos)

	// Test type environment building
	initialEnv := env.NewBindings[types.Base]()
	for _, input := range inputs {
		initialEnv = initialEnv.Bind(input.Name, input.Type, &input.Pos)
	}

	finalEnv, err := ResolveWorkflowTypes(workflow.Body, initialEnv)
	if err != nil {
		t.Errorf("Failed to resolve workflow types: %v", err)
	}

	// Check that intermediate variable is in final environment
	if _, err := finalEnv.ResolveBinding("intermediate"); err != nil {
		t.Error("Expected 'intermediate' to be in final type environment")
	}
}
