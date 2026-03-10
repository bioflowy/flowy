package workflow

import (
	"testing"

	"github.com/bioflowy/flowy/wdl/ast"
)

// makeCall builds a minimal ast.Call for testing buildDAG.
// alias can be "" to test callee-based ID derivation.
// after lists explicit ordering dependencies.
// inputs maps input names to IdentExpr or GetExpr referencing other calls.
func makeCall(callee []string, alias string, after []string, inputs map[string]ast.Expr) *ast.Call {
	return &ast.Call{
		Callee: callee,
		Alias:  alias,
		After:  after,
		Inputs: inputs,
	}
}

// ident returns an *ast.IdentExpr referencing the given name.
func ident(name string) ast.Expr {
	return &ast.IdentExpr{Name: name}
}

// getExpr returns an *ast.GetExpr representing obj.member (e.g., callA.output).
func getExpr(obj, member string) ast.Expr {
	return &ast.GetExpr{
		Object: &ast.IdentExpr{Name: obj},
		Member: member,
	}
}

// TestBuildDAG_LinearChain: B uses A's output => order must be [A, B].
func TestBuildDAG_LinearChain(t *testing.T) {
	callA := makeCall([]string{"taskA"}, "A", nil, nil)
	callB := makeCall([]string{"taskB"}, "B", nil, map[string]ast.Expr{
		"input1": getExpr("A", "output"),
	})

	body := []ast.WorkflowNode{callA, callB}
	nodes, err := buildDAG(body)
	if err != nil {
		t.Fatalf("buildDAG returned error: %v", err)
	}

	order, err := topoSort(nodes)
	if err != nil {
		t.Fatalf("topoSort returned error: %v", err)
	}

	if len(order) != 2 {
		t.Fatalf("expected 2 nodes in order, got %d: %v", len(order), order)
	}

	// A must come before B
	aIdx, bIdx := -1, -1
	for i, id := range order {
		if id == "A" {
			aIdx = i
		}
		if id == "B" {
			bIdx = i
		}
	}
	if aIdx < 0 || bIdx < 0 {
		t.Fatalf("expected both A and B in order, got %v", order)
	}
	if aIdx >= bIdx {
		t.Errorf("expected A before B, got order %v", order)
	}
}

// TestBuildDAG_Parallel: two independent calls A and B both have indegree 0.
func TestBuildDAG_Parallel(t *testing.T) {
	callA := makeCall([]string{"taskA"}, "A", nil, nil)
	callB := makeCall([]string{"taskB"}, "B", nil, nil)

	body := []ast.WorkflowNode{callA, callB}
	nodes, err := buildDAG(body)
	if err != nil {
		t.Fatalf("buildDAG returned error: %v", err)
	}

	if nodes["A"].indegree != 0 {
		t.Errorf("expected A.indegree=0, got %d", nodes["A"].indegree)
	}
	if nodes["B"].indegree != 0 {
		t.Errorf("expected B.indegree=0, got %d", nodes["B"].indegree)
	}

	order, err := topoSort(nodes)
	if err != nil {
		t.Fatalf("topoSort returned error: %v", err)
	}
	if len(order) != 2 {
		t.Fatalf("expected 2 nodes, got %d: %v", len(order), order)
	}
}

// TestBuildDAG_AfterDependency: B after A creates an edge A->B without data-flow.
func TestBuildDAG_AfterDependency(t *testing.T) {
	callA := makeCall([]string{"taskA"}, "A", nil, nil)
	callB := makeCall([]string{"taskB"}, "B", []string{"A"}, nil) // explicit after

	body := []ast.WorkflowNode{callA, callB}
	nodes, err := buildDAG(body)
	if err != nil {
		t.Fatalf("buildDAG returned error: %v", err)
	}

	// A must be in B's deps
	bNode := nodes["B"]
	foundA := false
	for _, dep := range bNode.deps {
		if dep == "A" {
			foundA = true
		}
	}
	if !foundA {
		t.Errorf("expected B to depend on A via after, deps=%v", bNode.deps)
	}

	order, err := topoSort(nodes)
	if err != nil {
		t.Fatalf("topoSort returned error: %v", err)
	}

	aIdx, bIdx := -1, -1
	for i, id := range order {
		if id == "A" {
			aIdx = i
		}
		if id == "B" {
			bIdx = i
		}
	}
	if aIdx >= bIdx {
		t.Errorf("expected A before B in order %v", order)
	}
}

// TestBuildDAG_Cycle: A->B->A must return a cycle error.
func TestBuildDAG_Cycle(t *testing.T) {
	callA := makeCall([]string{"taskA"}, "A", nil, map[string]ast.Expr{
		"in": getExpr("B", "output"),
	})
	callB := makeCall([]string{"taskB"}, "B", nil, map[string]ast.Expr{
		"in": getExpr("A", "output"),
	})

	body := []ast.WorkflowNode{callA, callB}
	nodes, err := buildDAG(body)
	if err != nil {
		t.Fatalf("buildDAG returned unexpected error: %v", err)
	}

	_, err = topoSort(nodes)
	if err == nil {
		t.Fatal("expected cycle error from topoSort, got nil")
	}
}

// TestBuildDAG_CallAlias: call with Alias uses alias as node ID, not callee name.
func TestBuildDAG_CallAlias(t *testing.T) {
	// Callee is "my_task" but alias is "my_alias"
	callA := makeCall([]string{"my_task"}, "my_alias", nil, nil)

	body := []ast.WorkflowNode{callA}
	nodes, err := buildDAG(body)
	if err != nil {
		t.Fatalf("buildDAG returned error: %v", err)
	}

	if _, ok := nodes["my_alias"]; !ok {
		t.Errorf("expected node with id 'my_alias', got keys: %v", nodeKeys(nodes))
	}
	if _, ok := nodes["my_task"]; ok {
		t.Errorf("expected no node with id 'my_task' (alias should take precedence)")
	}
}

// TestBuildDAG_MixedDataFlowAndAfter: C depends on A via data-flow and B via after; both edges.
func TestBuildDAG_MixedDataFlowAndAfter(t *testing.T) {
	callA := makeCall([]string{"taskA"}, "A", nil, nil)
	callB := makeCall([]string{"taskB"}, "B", nil, nil)
	callC := makeCall([]string{"taskC"}, "C", []string{"B"}, map[string]ast.Expr{
		"in": getExpr("A", "output"), // data-flow from A
		// after B is explicit
	})

	body := []ast.WorkflowNode{callA, callB, callC}
	nodes, err := buildDAG(body)
	if err != nil {
		t.Fatalf("buildDAG returned error: %v", err)
	}

	cNode := nodes["C"]
	hasA, hasB := false, false
	for _, dep := range cNode.deps {
		if dep == "A" {
			hasA = true
		}
		if dep == "B" {
			hasB = true
		}
	}
	if !hasA {
		t.Errorf("expected C to depend on A via data-flow, deps=%v", cNode.deps)
	}
	if !hasB {
		t.Errorf("expected C to depend on B via after, deps=%v", cNode.deps)
	}

	order, err := topoSort(nodes)
	if err != nil {
		t.Fatalf("topoSort returned error: %v", err)
	}
	if len(order) != 3 {
		t.Fatalf("expected 3 nodes in order, got %d: %v", len(order), order)
	}

	cIdx := -1
	for i, id := range order {
		if id == "C" {
			cIdx = i
		}
	}
	// A and B must both precede C
	for i, id := range order {
		if (id == "A" || id == "B") && i >= cIdx {
			t.Errorf("expected %s before C in order %v", id, order)
		}
	}
}

// nodeKeys returns a slice of map keys for error messages.
func nodeKeys(m map[string]*dagNode) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
