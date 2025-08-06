package tree

import (
	"testing"

	"github.com/bioflowy/flowy/pkg/env"
	"github.com/bioflowy/flowy/pkg/errors"
	"github.com/bioflowy/flowy/pkg/expr"
	"github.com/bioflowy/flowy/pkg/types"
)

func TestWorkflowGraphBuilder(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Create simple workflow nodes
	decl1 := NewDecl("step1", types.NewString(false), expr.NewStringLiteral("hello", pos), pos)
	decl2 := NewDecl("step2", types.NewInt(false), expr.NewIntLiteral(42, pos), pos)

	nodes := []WorkflowNode{decl1, decl2}
	initialEnv := env.NewBindings[types.Base]()

	builder := NewWorkflowGraphBuilder(nodes, initialEnv)

	dependencies, err := builder.BuildDependencyGraph()
	if err != nil {
		t.Errorf("Failed to build dependency graph: %v", err)
	}

	if len(dependencies) != 2 {
		t.Errorf("Expected 2 nodes in dependency graph, got %d", len(dependencies))
	}

	if _, exists := dependencies["step1"]; !exists {
		t.Error("Expected step1 in dependency graph")
	}

	if _, exists := dependencies["step2"]; !exists {
		t.Error("Expected step2 in dependency graph")
	}

	// Both should have no dependencies (literal initializations)
	if len(dependencies["step1"]) != 0 {
		t.Errorf("Expected step1 to have no dependencies, got %d", len(dependencies["step1"]))
	}

	if len(dependencies["step2"]) != 0 {
		t.Errorf("Expected step2 to have no dependencies, got %d", len(dependencies["step2"]))
	}
}

func TestValidateWorkflowGraph(t *testing.T) {
	// Test valid acyclic graph
	validDeps := map[string][]string{
		"A": {},
		"B": {"A"},
		"C": {"B"},
	}

	if err := ValidateWorkflowGraph(validDeps); err != nil {
		t.Errorf("Valid graph failed validation: %v", err)
	}

	// Test graph with cycle
	cyclicDeps := map[string][]string{
		"A": {"B"},
		"B": {"C"},
		"C": {"A"}, // Creates cycle A -> B -> C -> A
	}

	if err := ValidateWorkflowGraph(cyclicDeps); err == nil {
		t.Error("Cyclic graph should have failed validation")
	}

	// Test self-referencing cycle
	selfCycleDeps := map[string][]string{
		"A": {"A"}, // Self cycle
	}

	if err := ValidateWorkflowGraph(selfCycleDeps); err == nil {
		t.Error("Self-referencing graph should have failed validation")
	}
}

func TestTopologicalSort(t *testing.T) {
	// Test simple linear dependency chain
	deps := map[string][]string{
		"A": {},
		"B": {"A"},
		"C": {"B"},
	}

	order, err := TopologicalSort(deps)
	if err != nil {
		t.Errorf("Topological sort failed: %v", err)
	}

	if len(order) != 3 {
		t.Errorf("Expected 3 nodes in order, got %d", len(order))
	}

	// Check that dependencies are respected
	posA, posB, posC := -1, -1, -1
	for i, node := range order {
		switch node {
		case "A":
			posA = i
		case "B":
			posB = i
		case "C":
			posC = i
		}
	}

	if posA >= posB || posB >= posC {
		t.Errorf("Topological order violated: A(%d), B(%d), C(%d)", posA, posB, posC)
	}

	// Test parallel dependencies
	parallelDeps := map[string][]string{
		"A": {},
		"B": {},
		"C": {"A", "B"},
	}

	order2, err := TopologicalSort(parallelDeps)
	if err != nil {
		t.Errorf("Parallel dependencies topological sort failed: %v", err)
	}

	if len(order2) != 3 {
		t.Errorf("Expected 3 nodes in parallel order, got %d", len(order2))
	}

	// C should come after both A and B
	posA2, posB2, posC2 := -1, -1, -1
	for i, node := range order2 {
		switch node {
		case "A":
			posA2 = i
		case "B":
			posB2 = i
		case "C":
			posC2 = i
		}
	}

	if posC2 <= posA2 || posC2 <= posB2 {
		t.Errorf("Parallel topological order violated: A(%d), B(%d), C(%d)", posA2, posB2, posC2)
	}
}

func TestTopologicalSortCyclicGraph(t *testing.T) {
	cyclicDeps := map[string][]string{
		"A": {"B"},
		"B": {"A"},
	}

	_, err := TopologicalSort(cyclicDeps)
	if err == nil {
		t.Error("Topological sort should fail for cyclic graph")
	}
}

func TestResolveWorkflowTypes(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Create workflow nodes
	decl1 := NewDecl("var1", types.NewString(false), nil, pos)
	decl2 := NewDecl("var2", types.NewInt(false), nil, pos)
	call1 := NewCall("task1", "my_task", nil, make(map[string]expr.Expr), pos)

	nodes := []WorkflowNode{decl1, decl2, call1}
	initialEnv := env.NewBindings[types.Base]()

	finalEnv, err := ResolveWorkflowTypes(nodes, initialEnv)
	if err != nil {
		t.Errorf("Failed to resolve workflow types: %v", err)
	}

	// Check that all nodes are in the final environment
	if _, err := finalEnv.ResolveBinding("var1"); err != nil {
		t.Error("Expected var1 in final environment")
	}
	if _, err := finalEnv.ResolveBinding("var2"); err != nil {
		t.Error("Expected var2 in final environment")
	}
	if _, err := finalEnv.ResolveBinding("task1"); err != nil {
		t.Error("Expected task1 in final environment")
	}
}

func TestFindNodeByName(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	decl := NewDecl("target", types.NewString(false), nil, pos)
	call := NewCall("other", "task", nil, make(map[string]expr.Expr), pos)

	nodes := []WorkflowNode{decl, call}

	// Test finding existing node
	found := FindNodeByName(nodes, "target")
	if found != decl {
		t.Errorf("Expected to find decl node, got %v", found)
	}

	// Test finding non-existent node
	notFound := FindNodeByName(nodes, "missing")
	if notFound != nil {
		t.Errorf("Expected nil for missing node, got %v", notFound)
	}
}

func TestCollectAllDependencies(t *testing.T) {
	deps := map[string][]string{
		"A": {},
		"B": {"A"},
		"C": {"B"},
		"D": {"B", "C"},
	}

	visited := make(map[string]bool)
	allDeps := CollectAllDependencies("D", deps, visited)

	expectedDeps := map[string]bool{
		"B": true,
		"C": true,
		"A": true, // Transitive dependency through B and C
	}

	if len(allDeps) != len(expectedDeps) {
		t.Errorf("Expected %d dependencies, got %d", len(expectedDeps), len(allDeps))
	}

	for _, dep := range allDeps {
		if !expectedDeps[dep] {
			t.Errorf("Unexpected dependency: %s", dep)
		}
	}
}

func TestValidateDocumentStructure(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Test valid document
	validDoc := NewDocument(
		[]*Import{},
		nil,
		[]*Task{
			NewTask("task1", []*Decl{}, []*Decl{}, nil, pos),
			NewTask("task2", []*Decl{}, []*Decl{}, nil, pos),
		},
		[]*StructTypeDef{
			NewStructTypeDef("Struct1", []*StructMember{}, pos),
		},
		pos,
	)

	if err := ValidateDocumentStructure(validDoc); err != nil {
		t.Errorf("Valid document failed validation: %v", err)
	}

	// Test document with duplicate task names
	invalidDocTasks := NewDocument(
		[]*Import{},
		nil,
		[]*Task{
			NewTask("duplicate", []*Decl{}, []*Decl{}, nil, pos),
			NewTask("duplicate", []*Decl{}, []*Decl{}, nil, pos),
		},
		[]*StructTypeDef{},
		pos,
	)

	if err := ValidateDocumentStructure(invalidDocTasks); err == nil {
		t.Error("Document with duplicate task names should fail validation")
	}

	// Test document with duplicate struct names
	invalidDocStructs := NewDocument(
		[]*Import{},
		nil,
		[]*Task{},
		[]*StructTypeDef{
			NewStructTypeDef("duplicate", []*StructMember{}, pos),
			NewStructTypeDef("duplicate", []*StructMember{}, pos),
		},
		pos,
	)

	if err := ValidateDocumentStructure(invalidDocStructs); err == nil {
		t.Error("Document with duplicate struct names should fail validation")
	}

	// Test nil document
	if err := ValidateDocumentStructure(nil); err == nil {
		t.Error("Nil document should fail validation")
	}
}

func TestValidateWorkflowStructure(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Test valid workflow
	validWorkflow := NewWorkflow("valid", []*Decl{}, []WorkflowNode{
		NewDecl("step1", types.NewString(false), expr.NewStringLiteral("hello", pos), pos),
		NewDecl("step2", types.NewInt(false), expr.NewIntLiteral(42, pos), pos),
	}, []*Decl{}, pos)

	if err := ValidateWorkflowStructure(validWorkflow); err != nil {
		t.Errorf("Valid workflow failed validation: %v", err)
	}

	// Test nil workflow (should pass)
	if err := ValidateWorkflowStructure(nil); err != nil {
		t.Errorf("Nil workflow validation should pass: %v", err)
	}
}

// Integration tests

func TestComplexWorkflowValidation(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Create a complex workflow with multiple node types
	inputs := []*Decl{
		NewDecl("input_files", types.NewArray(types.NewString(false), false, false), nil, pos),
		NewDecl("threshold", types.NewInt(false), nil, pos),
	}

	// Workflow body with various node types
	body := []WorkflowNode{
		// Declaration
		NewDecl("processed_count", types.NewInt(false), expr.NewIntLiteral(0, pos), pos),

		// Call
		NewCall("preprocess", "prep_task", nil, map[string]expr.Expr{
			"files": expr.NewIdentifier("input_files", pos),
		}, pos),

		// Scatter
		NewScatter("file", expr.NewIdentifier("preprocess.output_files", pos), []WorkflowNode{
			NewCall("process", "process_task", nil, map[string]expr.Expr{
				"file": expr.NewIdentifier("file", pos),
			}, pos),
		}, pos),

		// Conditional
		NewConditional(
			expr.NewBinaryOp(expr.NewIdentifier("threshold", pos), ">", expr.NewIntLiteral(0, pos), pos),
			[]WorkflowNode{
				NewCall("filter", "filter_task", nil, make(map[string]expr.Expr), pos),
			},
			pos,
		),
	}

	outputs := []*Decl{
		NewDecl("results", types.NewArray(types.NewString(false), false, false),
			expr.NewIdentifier("scatter_results", pos), pos),
	}

	workflow := NewWorkflow("complex_workflow", inputs, body, outputs, pos)

	// Validate the complex workflow
	if err := ValidateWorkflowStructure(workflow); err != nil {
		t.Errorf("Complex workflow validation failed: %v", err)
	}

	// Test dependency graph building
	initialEnv := env.NewBindings[types.Base]()
	for _, input := range inputs {
		initialEnv = initialEnv.Bind(input.Name, input.Type, &input.Pos)
	}

	builder := NewWorkflowGraphBuilder(body, initialEnv)
	deps, err := builder.BuildDependencyGraph()
	if err != nil {
		t.Errorf("Failed to build dependency graph for complex workflow: %v", err)
	}

	// Should have entries for all nodes
	expectedNodes := []string{"processed_count", "preprocess", "scatter_file", "conditional"}
	for _, node := range expectedNodes {
		if _, exists := deps[node]; !exists {
			t.Errorf("Expected node %s in dependency graph", node)
		}
	}
}

func TestTypeResolutionOrder(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Create nodes that depend on each other
	decl1 := NewDecl("first", types.NewString(false), expr.NewStringLiteral("hello", pos), pos)
	// In a real implementation, this would reference first's output
	decl2 := NewDecl("second", types.NewString(false), expr.NewIdentifier("first", pos), pos)

	nodes := []WorkflowNode{decl1, decl2}
	initialEnv := env.NewBindings[types.Base]()

	// Resolve types - should handle the dependency order
	finalEnv, err := ResolveWorkflowTypes(nodes, initialEnv)
	if err != nil {
		t.Errorf("Failed to resolve types with dependencies: %v", err)
	}

	// Both should be in final environment
	if _, err := finalEnv.ResolveBinding("first"); err != nil {
		t.Error("Expected first in final environment")
	}
	if _, err := finalEnv.ResolveBinding("second"); err != nil {
		t.Error("Expected second in final environment")
	}
}
