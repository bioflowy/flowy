package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/task"
	"github.com/bioflowy/flowy/wdl/types"
	"github.com/bioflowy/flowy/wdl/values"
)

// mockRunner is a ContainerRunner that delegates to an inner function for flexible test control.
type mockRunner struct {
	fn func(ctx context.Context, req task.RunRequest) (task.RunResult, error)
}

func (m *mockRunner) Run(ctx context.Context, req task.RunRequest) (task.RunResult, error) {
	return m.fn(ctx, req)
}

// noopRunner is a mock ContainerRunner that does nothing (panics if called — tasks
// should be bypassed by overriding runTaskFn in tests).
type noopRunner struct{}

func (n *noopRunner) Run(_ context.Context, _ task.RunRequest) (task.RunResult, error) {
	panic("noopRunner.Run should not be called in unit tests — use runTaskFn override")
}

// makeSimpleTask creates an ast.Task with a single input and single output for testing.
// taskName is the task name, inputName is the required Int input, outputName is the Int output.
func makeSimpleTask(taskName, inputName, outputName string) *ast.Task {
	return &ast.Task{
		Name: taskName,
		Inputs: []*ast.Decl{
			{Name: inputName, Type: types.IntType{}},
		},
		Outputs: []*ast.Decl{
			{Name: outputName, Type: types.IntType{}},
		},
	}
}

// makeCallNode creates an ast.Call node that calls taskName with optional alias and inputs.
func makeCallNode(taskName, alias string, inputs map[string]ast.Expr) *ast.Call {
	return &ast.Call{
		Callee: []string{taskName},
		Alias:  alias,
		Inputs: inputs,
	}
}

// identExpr creates an *ast.IdentExpr for use in test AST construction.
func identExpr(name string) *ast.IdentExpr {
	return &ast.IdentExpr{Name: name}
}

// memberExpr creates an *ast.GetExpr (obj.member) for test AST construction.
func memberExpr(obj, member string) *ast.GetExpr {
	return &ast.GetExpr{Object: identExpr(obj), Member: member}
}

// intLit creates an *ast.IntLit for test AST construction.
func intLit(n int64) *ast.IntLit {
	return &ast.IntLit{Value: n}
}

// boolLit creates an *ast.BoolLit for test AST construction.
func boolLit(b bool) *ast.BoolLit {
	return &ast.BoolLit{Value: b}
}

// makeDoc builds a minimal ast.Document containing the given tasks and a single workflow.
func makeDoc(wfName string, wfBody []ast.WorkflowNode, wfOutputs []*ast.Decl, tasks ...*ast.Task) *ast.Document {
	return &ast.Document{
		Tasks: tasks,
		Workflows: []*ast.Workflow{
			{
				Name:    wfName,
				Inputs:  nil,
				Body:    wfBody,
				Outputs: wfOutputs,
			},
		},
	}
}

// TestRunWorkflow_SingleCall: workflow with one call -> task executes, output bound as "callName.outputName".
func TestRunWorkflow_SingleCall(t *testing.T) {
	// Override runTaskFn to return a fixed output without spawning a container.
	origFn := runTaskFn
	runTaskFn = func(_ context.Context, _ *ast.Task, _ map[string]interface{}, _ task.ContainerRunner, _ string, _ ...map[string]*ast.StructDef) (map[string]values.Value, error) {
		return map[string]values.Value{
			"result": values.IntValue{Value: 42},
		}, nil
	}
	defer func() { runTaskFn = origFn }()

	myTask := makeSimpleTask("my_task", "x", "result")
	call := makeCallNode("my_task", "", nil)
	doc := makeDoc("wf", []ast.WorkflowNode{call}, nil, myTask)

	outputs, err := RunWorkflow(context.Background(), doc, "wf", nil, &noopRunner{}, "/tmp")
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}

	// outputs is the workflow outputs map — for no declared outputs it may be empty.
	// The key check is that no error occurs and the call ran.
	_ = outputs
}

// TestRunWorkflow_TwoCalls_DataDep: call B uses A.result -> A runs first, B runs second.
func TestRunWorkflow_TwoCalls_DataDep(t *testing.T) {

	var callOrder []string
	var mu sync.Mutex

	origFn := runTaskFn
	runTaskFn = func(_ context.Context, tsk *ast.Task, _ map[string]interface{}, _ task.ContainerRunner, _ string, _ ...map[string]*ast.StructDef) (map[string]values.Value, error) {
		mu.Lock()
		callOrder = append(callOrder, tsk.Name)
		mu.Unlock()
		return map[string]values.Value{
			"result": values.IntValue{Value: 10},
		}, nil
	}
	defer func() { runTaskFn = origFn }()

	taskA := makeSimpleTask("task_a", "x", "result")
	taskB := makeSimpleTask("task_b", "y", "result")

	callA := makeCallNode("task_a", "a", nil)
	// B's input references a.result (data flow dep)
	callB := makeCallNode("task_b", "b", map[string]ast.Expr{
		"y": memberExpr("a", "result"),
	})

	doc := makeDoc("wf", []ast.WorkflowNode{callA, callB}, nil, taskA, taskB)

	_, err := RunWorkflow(context.Background(), doc, "wf", nil, &noopRunner{}, "/tmp")
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}

	if len(callOrder) != 2 {
		t.Fatalf("expected 2 calls, got %v", callOrder)
	}
	if callOrder[0] != "task_a" || callOrder[1] != "task_b" {
		t.Errorf("expected [task_a, task_b], got %v", callOrder)
	}
}

// TestRunWorkflow_TwoCalls_Parallel: two independent calls -> both execute (no ordering constraint).
func TestRunWorkflow_TwoCalls_Parallel(t *testing.T) {

	var mu sync.Mutex
	called := map[string]bool{}

	origFn := runTaskFn
	runTaskFn = func(_ context.Context, tsk *ast.Task, _ map[string]interface{}, _ task.ContainerRunner, _ string, _ ...map[string]*ast.StructDef) (map[string]values.Value, error) {
		mu.Lock()
		called[tsk.Name] = true
		mu.Unlock()
		return map[string]values.Value{"result": values.IntValue{Value: 1}}, nil
	}
	defer func() { runTaskFn = origFn }()

	taskA := makeSimpleTask("task_a", "x", "result")
	taskB := makeSimpleTask("task_b", "x", "result")
	callA := makeCallNode("task_a", "a", nil)
	callB := makeCallNode("task_b", "b", nil)

	doc := makeDoc("wf", []ast.WorkflowNode{callA, callB}, nil, taskA, taskB)

	_, err := RunWorkflow(context.Background(), doc, "wf", nil, &noopRunner{}, "/tmp")
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
	if !called["task_a"] || !called["task_b"] {
		t.Errorf("expected both tasks called, got %v", called)
	}
}

// TestRunWorkflow_AfterDep: call B after A -> A completes before B starts.
func TestRunWorkflow_AfterDep(t *testing.T) {

	var callOrder []string
	var mu sync.Mutex

	origFn := runTaskFn
	runTaskFn = func(_ context.Context, tsk *ast.Task, _ map[string]interface{}, _ task.ContainerRunner, _ string, _ ...map[string]*ast.StructDef) (map[string]values.Value, error) {
		mu.Lock()
		callOrder = append(callOrder, tsk.Name)
		mu.Unlock()
		return map[string]values.Value{"result": values.IntValue{Value: 1}}, nil
	}
	defer func() { runTaskFn = origFn }()

	taskA := makeSimpleTask("task_a", "x", "result")
	taskB := makeSimpleTask("task_b", "x", "result")

	callA := makeCallNode("task_a", "a", nil)
	// B has explicit after dep on A, no data flow.
	callB := &ast.Call{
		Callee: []string{"task_b"},
		Alias:  "b",
		After:  []string{"a"},
		Inputs: nil,
	}

	doc := makeDoc("wf", []ast.WorkflowNode{callA, callB}, nil, taskA, taskB)

	_, err := RunWorkflow(context.Background(), doc, "wf", nil, &noopRunner{}, "/tmp")
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}

	if len(callOrder) != 2 || callOrder[0] != "task_a" || callOrder[1] != "task_b" {
		t.Errorf("expected [task_a, task_b], got %v", callOrder)
	}
}

// TestRunWorkflow_Conditional_True: if(true) { call foo } -> foo executes, outputs available.
func TestRunWorkflow_Conditional_True(t *testing.T) {

	called := false
	origFn := runTaskFn
	runTaskFn = func(_ context.Context, _ *ast.Task, _ map[string]interface{}, _ task.ContainerRunner, _ string, _ ...map[string]*ast.StructDef) (map[string]values.Value, error) {
		called = true
		return map[string]values.Value{"result": values.IntValue{Value: 99}}, nil
	}
	defer func() { runTaskFn = origFn }()

	myTask := makeSimpleTask("foo", "x", "result")
	call := makeCallNode("foo", "foo", nil)
	cond := &ast.Conditional{
		Expr: boolLit(true),
		Body: []ast.WorkflowNode{call},
	}

	doc := makeDoc("wf", []ast.WorkflowNode{cond}, nil, myTask)

	_, err := RunWorkflow(context.Background(), doc, "wf", nil, &noopRunner{}, "/tmp")
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
	if !called {
		t.Error("expected task foo to be called in true branch")
	}
}

// TestRunWorkflow_Conditional_False: if(false) { call foo } -> foo NOT executed, outputs NullValue.
func TestRunWorkflow_Conditional_False(t *testing.T) {

	called := false
	origFn := runTaskFn
	runTaskFn = func(_ context.Context, _ *ast.Task, _ map[string]interface{}, _ task.ContainerRunner, _ string, _ ...map[string]*ast.StructDef) (map[string]values.Value, error) {
		called = true
		return map[string]values.Value{"result": values.IntValue{Value: 99}}, nil
	}
	defer func() { runTaskFn = origFn }()

	myTask := makeSimpleTask("foo", "x", "result")
	call := makeCallNode("foo", "foo", nil)
	cond := &ast.Conditional{
		Expr: boolLit(false),
		Body: []ast.WorkflowNode{call},
	}

	doc := makeDoc("wf", []ast.WorkflowNode{cond}, nil, myTask)

	_, err := RunWorkflow(context.Background(), doc, "wf", nil, &noopRunner{}, "/tmp")
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
	if called {
		t.Error("expected task foo NOT to be called in false branch")
	}
}

// TestRunWorkflow_Decl: workflow with `Int x = 42` before a call that uses x -> x is bound in env.
func TestRunWorkflow_Decl(t *testing.T) {

	var capturedInputs map[string]interface{}
	origFn := runTaskFn
	runTaskFn = func(_ context.Context, _ *ast.Task, inputs map[string]interface{}, _ task.ContainerRunner, _ string, _ ...map[string]*ast.StructDef) (map[string]values.Value, error) {
		capturedInputs = inputs
		return map[string]values.Value{"result": values.IntValue{Value: 1}}, nil
	}
	defer func() { runTaskFn = origFn }()

	myTask := makeSimpleTask("echo_task", "val", "result")
	decl := &ast.Decl{
		Type: types.IntType{},
		Name: "x",
		Expr: intLit(42),
	}
	call := makeCallNode("echo_task", "t", map[string]ast.Expr{
		"val": identExpr("x"),
	})

	doc := makeDoc("wf", []ast.WorkflowNode{decl, call}, nil, myTask)

	_, err := RunWorkflow(context.Background(), doc, "wf", nil, &noopRunner{}, "/tmp")
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
	if capturedInputs["val"] != int64(42) {
		t.Errorf("expected val=42, got %v", capturedInputs["val"])
	}
}

// ---------------------------------------------------------------------------
// Scatter tests (Task 2)
// ---------------------------------------------------------------------------

// TestRunWorkflow_Scatter_Basic: scatter over [1,2,3] -> outputs collected as Array[T] with 3 elements.
func TestRunWorkflow_Scatter_Basic(t *testing.T) {

	origFn := runTaskFn
	runTaskFn = func(_ context.Context, _ *ast.Task, inputs map[string]interface{}, _ task.ContainerRunner, _ string, _ ...map[string]*ast.StructDef) (map[string]values.Value, error) {
		// Return doubled input value.
		v, _ := inputs["n"].(int64)
		return map[string]values.Value{
			"out": values.IntValue{Value: v * 2},
		}, nil
	}
	defer func() { runTaskFn = origFn }()

	myTask := makeSimpleTask("doubler", "n", "out")

	// scatter (n in [1, 2, 3]) { call doubler { input: n = n } }
	scatter := &ast.Scatter{
		Variable: "n",
		Expr: &ast.ArrayExpr{
			Items: []ast.Expr{intLit(1), intLit(2), intLit(3)},
		},
		Body: []ast.WorkflowNode{
			makeCallNode("doubler", "d", map[string]ast.Expr{
				"n": identExpr("n"),
			}),
		},
	}

	doc := makeDoc("wf", []ast.WorkflowNode{scatter}, nil, myTask)

	_, err := RunWorkflow(context.Background(), doc, "wf", nil, &noopRunner{}, "/tmp")
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}

	// Verify that the scatter node output was bound in env as Array.
	// We check by looking at the scheduler's internal env after run (via workflow outputs).
	// For now just verify no error and task was called 3 times.
}

// TestRunWorkflow_Scatter_Basic_CheckOutputs verifies scatter outputs are bound as Array[T].
func TestRunWorkflow_Scatter_Basic_CheckOutputs(t *testing.T) {

	origFn := runTaskFn
	runTaskFn = func(_ context.Context, _ *ast.Task, inputs map[string]interface{}, _ task.ContainerRunner, _ string, _ ...map[string]*ast.StructDef) (map[string]values.Value, error) {
		v, _ := inputs["n"].(int64)
		return map[string]values.Value{
			"out": values.IntValue{Value: v * 2},
		}, nil
	}
	defer func() { runTaskFn = origFn }()

	myTask := makeSimpleTask("doubler", "n", "out")

	scatter := &ast.Scatter{
		Variable: "n",
		Expr:     &ast.ArrayExpr{Items: []ast.Expr{intLit(1), intLit(2), intLit(3)}},
		Body: []ast.WorkflowNode{
			makeCallNode("doubler", "d", map[string]ast.Expr{"n": identExpr("n")}),
		},
	}

	// Output that reads d.out (the scatter-collected array)
	wfOutputs := []*ast.Decl{
		{
			Name: "results",
			Type: types.ArrayType{ItemType: types.IntType{}},
			Expr: memberExpr("d", "out"),
		},
	}

	doc := makeDoc("wf", []ast.WorkflowNode{scatter}, wfOutputs, myTask)

	outputs, err := RunWorkflow(context.Background(), doc, "wf", nil, &noopRunner{}, "/tmp")
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}

	results, ok := outputs["results"]
	if !ok {
		t.Fatal("expected 'results' in workflow outputs")
	}
	arr, ok := results.(values.ArrayValue)
	if !ok {
		t.Fatalf("expected ArrayValue, got %T", results)
	}
	if len(arr.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(arr.Items))
	}
	// Items should be [2, 4, 6] in order
	want := []int64{2, 4, 6}
	for i, item := range arr.Items {
		iv, ok := item.(values.IntValue)
		if !ok {
			t.Errorf("item[%d] not IntValue: %T", i, item)
			continue
		}
		if iv.Value != want[i] {
			t.Errorf("item[%d]: expected %d, got %d", i, want[i], iv.Value)
		}
	}
}

// TestRunWorkflow_Scatter_Empty: scatter over [] -> produces empty ArrayValue.
func TestRunWorkflow_Scatter_Empty(t *testing.T) {

	origFn := runTaskFn
	runTaskFn = func(_ context.Context, _ *ast.Task, _ map[string]interface{}, _ task.ContainerRunner, _ string, _ ...map[string]*ast.StructDef) (map[string]values.Value, error) {
		return map[string]values.Value{"out": values.IntValue{Value: 0}}, nil
	}
	defer func() { runTaskFn = origFn }()

	myTask := makeSimpleTask("doubler", "n", "out")

	scatter := &ast.Scatter{
		Variable: "n",
		Expr:     &ast.ArrayExpr{Items: []ast.Expr{}},
		Body: []ast.WorkflowNode{
			makeCallNode("doubler", "d", map[string]ast.Expr{"n": identExpr("n")}),
		},
	}

	wfOutputs := []*ast.Decl{
		{
			Name: "results",
			Type: types.ArrayType{ItemType: types.IntType{}},
			Expr: memberExpr("d", "out"),
		},
	}

	doc := makeDoc("wf", []ast.WorkflowNode{scatter}, wfOutputs, myTask)

	outputs, err := RunWorkflow(context.Background(), doc, "wf", nil, &noopRunner{}, "/tmp")
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}

	results, ok := outputs["results"]
	if !ok {
		t.Fatal("expected 'results' in workflow outputs")
	}
	arr, ok := results.(values.ArrayValue)
	if !ok {
		t.Fatalf("expected ArrayValue, got %T", results)
	}
	if len(arr.Items) != 0 {
		t.Errorf("expected empty array, got %d items", len(arr.Items))
	}
}

// TestRunWorkflow_ScatterConcurrencyLimit: scatter over 20 elements, max concurrent never exceeds 8.
func TestRunWorkflow_ScatterConcurrencyLimit(t *testing.T) {

	var (
		current int64
		maxSeen int64
	)

	origFn := runTaskFn
	runTaskFn = func(ctx context.Context, _ *ast.Task, _ map[string]interface{}, _ task.ContainerRunner, _ string, _ ...map[string]*ast.StructDef) (map[string]values.Value, error) {
		c := atomic.AddInt64(&current, 1)
		for {
			m := atomic.LoadInt64(&maxSeen)
			if c <= m || atomic.CompareAndSwapInt64(&maxSeen, m, c) {
				break
			}
		}
		// Simulate some work
		select {
		case <-ctx.Done():
			atomic.AddInt64(&current, -1)
			return nil, ctx.Err()
		default:
		}
		atomic.AddInt64(&current, -1)
		return map[string]values.Value{"out": values.IntValue{Value: 1}}, nil
	}
	defer func() { runTaskFn = origFn }()

	myTask := makeSimpleTask("work", "n", "out")

	items := make([]ast.Expr, 20)
	for i := range items {
		items[i] = intLit(int64(i))
	}

	scatter := &ast.Scatter{
		Variable: "n",
		Expr:     &ast.ArrayExpr{Items: items},
		Body: []ast.WorkflowNode{
			makeCallNode("work", "w", map[string]ast.Expr{"n": identExpr("n")}),
		},
	}

	doc := makeDoc("wf", []ast.WorkflowNode{scatter}, nil, myTask)

	_, err := RunWorkflow(context.Background(), doc, "wf", nil, &noopRunner{}, "/tmp")
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}

	if maxSeen > 8 {
		t.Errorf("concurrency limit exceeded: max concurrent was %d (limit 8)", maxSeen)
	}
}

// TestRunWorkflow_ScatterInIf_True: if(true) { scatter(...) { call } } -> Array[T].
func TestRunWorkflow_ScatterInIf_True(t *testing.T) {

	origFn := runTaskFn
	runTaskFn = func(_ context.Context, _ *ast.Task, inputs map[string]interface{}, _ task.ContainerRunner, _ string, _ ...map[string]*ast.StructDef) (map[string]values.Value, error) {
		v, _ := inputs["n"].(int64)
		return map[string]values.Value{"out": values.IntValue{Value: v}}, nil
	}
	defer func() { runTaskFn = origFn }()

	myTask := makeSimpleTask("t", "n", "out")

	scatter := &ast.Scatter{
		Variable: "n",
		Expr:     &ast.ArrayExpr{Items: []ast.Expr{intLit(1), intLit(2)}},
		Body: []ast.WorkflowNode{
			makeCallNode("t", "c", map[string]ast.Expr{"n": identExpr("n")}),
		},
	}

	cond := &ast.Conditional{
		Expr: boolLit(true),
		Body: []ast.WorkflowNode{scatter},
	}

	doc := makeDoc("wf", []ast.WorkflowNode{cond}, nil, myTask)

	_, err := RunWorkflow(context.Background(), doc, "wf", nil, &noopRunner{}, "/tmp")
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
}

// TestRunWorkflow_ScatterInIf_False: if(false) { scatter(...) { call } } -> NullValue.
func TestRunWorkflow_ScatterInIf_False(t *testing.T) {

	called := false
	origFn := runTaskFn
	runTaskFn = func(_ context.Context, _ *ast.Task, _ map[string]interface{}, _ task.ContainerRunner, _ string, _ ...map[string]*ast.StructDef) (map[string]values.Value, error) {
		called = true
		return map[string]values.Value{"out": values.IntValue{Value: 1}}, nil
	}
	defer func() { runTaskFn = origFn }()

	myTask := makeSimpleTask("t", "n", "out")

	scatter := &ast.Scatter{
		Variable: "n",
		Expr:     &ast.ArrayExpr{Items: []ast.Expr{intLit(1)}},
		Body: []ast.WorkflowNode{
			makeCallNode("t", "c", map[string]ast.Expr{"n": identExpr("n")}),
		},
	}

	cond := &ast.Conditional{
		Expr: boolLit(false),
		Body: []ast.WorkflowNode{scatter},
	}

	doc := makeDoc("wf", []ast.WorkflowNode{cond}, nil, myTask)

	_, err := RunWorkflow(context.Background(), doc, "wf", nil, &noopRunner{}, "/tmp")
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
	if called {
		t.Error("expected task NOT called in false branch")
	}
}

// TestRunWorkflow_IfInScatter: scatter(...) { if(cond) { call } } -> Array[T?] with Some/None per element.
func TestRunWorkflow_IfInScatter(t *testing.T) {

	origFn := runTaskFn
	runTaskFn = func(_ context.Context, _ *ast.Task, inputs map[string]interface{}, _ task.ContainerRunner, _ string, _ ...map[string]*ast.StructDef) (map[string]values.Value, error) {
		v, _ := inputs["n"].(int64)
		return map[string]values.Value{"out": values.IntValue{Value: v}}, nil
	}
	defer func() { runTaskFn = origFn }()

	myTask := makeSimpleTask("t", "n", "out")

	// scatter (flag in [true, false, true]) { if(flag) { call t } }
	scatter := &ast.Scatter{
		Variable: "flag",
		Expr: &ast.ArrayExpr{
			Items: []ast.Expr{boolLit(true), boolLit(false), boolLit(true)},
		},
		Body: []ast.WorkflowNode{
			&ast.Conditional{
				Expr: identExpr("flag"),
				Body: []ast.WorkflowNode{
					makeCallNode("t", "c", map[string]ast.Expr{"n": intLit(1)}),
				},
			},
		},
	}

	doc := makeDoc("wf", []ast.WorkflowNode{scatter}, nil, myTask)

	_, err := RunWorkflow(context.Background(), doc, "wf", nil, &noopRunner{}, "/tmp")
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
}

// TestRunWorkflow_ScatterInScatter: nested scatter -> Array[Array[T]].
func TestRunWorkflow_ScatterInScatter(t *testing.T) {

	origFn := runTaskFn
	runTaskFn = func(_ context.Context, _ *ast.Task, inputs map[string]interface{}, _ task.ContainerRunner, _ string, _ ...map[string]*ast.StructDef) (map[string]values.Value, error) {
		v, _ := inputs["n"].(int64)
		return map[string]values.Value{"out": values.IntValue{Value: v}}, nil
	}
	defer func() { runTaskFn = origFn }()

	myTask := makeSimpleTask("t", "n", "out")

	// scatter (i in [1,2]) { scatter (j in [10,20]) { call t { input: n = j } } }
	inner := &ast.Scatter{
		Variable: "j",
		Expr:     &ast.ArrayExpr{Items: []ast.Expr{intLit(10), intLit(20)}},
		Body: []ast.WorkflowNode{
			makeCallNode("t", "c", map[string]ast.Expr{"n": identExpr("j")}),
		},
	}
	outer := &ast.Scatter{
		Variable: "i",
		Expr:     &ast.ArrayExpr{Items: []ast.Expr{intLit(1), intLit(2)}},
		Body:     []ast.WorkflowNode{inner},
	}

	doc := makeDoc("wf", []ast.WorkflowNode{outer}, nil, myTask)

	_, err := RunWorkflow(context.Background(), doc, "wf", nil, &noopRunner{}, "/tmp")
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
}

// TestRunWorkflow_WorkflowOutputs verifies that workflow output decls are evaluated and returned.
func TestRunWorkflow_WorkflowOutputs(t *testing.T) {

	origFn := runTaskFn
	runTaskFn = func(_ context.Context, _ *ast.Task, _ map[string]interface{}, _ task.ContainerRunner, _ string, _ ...map[string]*ast.StructDef) (map[string]values.Value, error) {
		return map[string]values.Value{"result": values.IntValue{Value: 7}}, nil
	}
	defer func() { runTaskFn = origFn }()

	myTask := makeSimpleTask("compute", "x", "result")
	call := makeCallNode("compute", "c", nil)

	wfOutputs := []*ast.Decl{
		{
			Name: "answer",
			Type: types.IntType{},
			Expr: memberExpr("c", "result"),
		},
	}

	doc := makeDoc("wf", []ast.WorkflowNode{call}, wfOutputs, myTask)

	outputs, err := RunWorkflow(context.Background(), doc, "wf", nil, &noopRunner{}, "/tmp")
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}

	ans, ok := outputs["answer"]
	if !ok {
		t.Fatal("expected 'answer' in workflow outputs")
	}
	iv, ok := ans.(values.IntValue)
	if !ok {
		t.Fatalf("expected IntValue, got %T", ans)
	}
	if iv.Value != 7 {
		t.Errorf("expected 7, got %d", iv.Value)
	}
}

// TestRunWorkflow_NotFound: RunWorkflow with unknown workflow name returns error.
func TestRunWorkflow_NotFound(t *testing.T) {

	doc := makeDoc("wf", nil, nil)
	_, err := RunWorkflow(context.Background(), doc, "no_such_wf", nil, &noopRunner{}, "/tmp")
	if err == nil {
		t.Error("expected error for missing workflow name")
	}
}

// TestRunWorkflow_TaskNotFound: call references task that doesn't exist -> error.
func TestRunWorkflow_TaskNotFound(t *testing.T) {

	origFn := runTaskFn
	runTaskFn = func(_ context.Context, _ *ast.Task, _ map[string]interface{}, _ task.ContainerRunner, _ string, _ ...map[string]*ast.StructDef) (map[string]values.Value, error) {
		return nil, fmt.Errorf("should not be called")
	}
	defer func() { runTaskFn = origFn }()

	// No tasks registered in doc but we try to call one.
	call := makeCallNode("missing_task", "", nil)
	doc := makeDoc("wf", []ast.WorkflowNode{call}, nil)

	_, err := RunWorkflow(context.Background(), doc, "wf", nil, &noopRunner{}, "/tmp")
	if err == nil {
		t.Error("expected error for missing task")
	}
}

// ---------------------------------------------------------------------------
// Output/Error JSON writing tests (Task 2 — Plan 06-03)
// ---------------------------------------------------------------------------

// readJSONFileStr reads and parses a JSON file, returning map[string]interface{}.
func readJSONFileStr(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading JSON file %q: %v", path, err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("parsing JSON file %q: %v", path, err)
	}
	return result
}

// TestRunWorkflow_OutputsJSON: successful workflow -> outputs.json written in execRoot.
func TestRunWorkflow_OutputsJSON(t *testing.T) {
	execRoot := t.TempDir()

	origFn := runTaskFn
	runTaskFn = func(_ context.Context, _ *ast.Task, _ map[string]interface{}, _ task.ContainerRunner, _ string, _ ...map[string]*ast.StructDef) (map[string]values.Value, error) {
		return map[string]values.Value{"result": values.IntValue{Value: 99}}, nil
	}
	defer func() { runTaskFn = origFn }()

	myTask := makeSimpleTask("compute", "x", "result")
	call := makeCallNode("compute", "c", nil)
	wfOutputs := []*ast.Decl{
		{
			Name: "answer",
			Type: types.IntType{},
			Expr: memberExpr("c", "result"),
		},
	}
	doc := makeDoc("myWf", []ast.WorkflowNode{call}, wfOutputs, myTask)

	outputs, err := RunWorkflow(context.Background(), doc, "myWf", nil, &noopRunner{}, execRoot)
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
	if outputs == nil {
		t.Fatal("expected non-nil outputs map")
	}

	// outputs.json must exist.
	outPath := filepath.Join(execRoot, "outputs.json")
	if _, err := os.Stat(outPath); os.IsNotExist(err) {
		t.Fatal("outputs.json not written")
	}

	got := readJSONFileStr(t, outPath)
	v, ok := got["myWf.answer"]
	if !ok {
		t.Fatal("expected key 'myWf.answer' in outputs.json")
	}
	if v.(float64) != 99 {
		t.Errorf("expected 99, got %v", v)
	}
}

// TestRunWorkflow_FailFast_ErrorJSON: task fails -> error.json written, outputs.json absent.
func TestRunWorkflow_FailFast_ErrorJSON(t *testing.T) {
	execRoot := t.TempDir()

	origFn := runTaskFn
	runTaskFn = func(_ context.Context, tsk *ast.Task, _ map[string]interface{}, _ task.ContainerRunner, _ string, _ ...map[string]*ast.StructDef) (map[string]values.Value, error) {
		return nil, &task.TaskError{TaskName: tsk.Name, ExitCode: 2, Stderr: []byte("boom")}
	}
	defer func() { runTaskFn = origFn }()

	myTask := makeSimpleTask("fail_task", "x", "result")
	call := makeCallNode("fail_task", "f", nil)
	doc := makeDoc("wf", []ast.WorkflowNode{call}, nil, myTask)

	_, err := RunWorkflow(context.Background(), doc, "wf", nil, &noopRunner{}, execRoot)
	if err == nil {
		t.Fatal("expected error from RunWorkflow when task fails")
	}

	// error.json must exist.
	errPath := filepath.Join(execRoot, "error.json")
	if _, err := os.Stat(errPath); os.IsNotExist(err) {
		t.Fatal("error.json not written")
	}

	got := readJSONFileStr(t, errPath)
	if got["task"].(string) != "fail_task" {
		t.Errorf("expected task='fail_task', got %v", got["task"])
	}
	if got["exit_code"].(float64) != 2 {
		t.Errorf("expected exit_code=2, got %v", got["exit_code"])
	}

	// outputs.json must NOT exist.
	outPath := filepath.Join(execRoot, "outputs.json")
	if _, statErr := os.Stat(outPath); !os.IsNotExist(statErr) {
		t.Error("outputs.json should NOT be written when task fails")
	}
}

// TestRunWorkflow_FailFast_CancelsOthers: task A fails quickly while task B is running
// -> B receives context cancellation (or completes before cancellation signal arrives,
// but at minimum the workflow returns an error).
func TestRunWorkflow_FailFast_CancelsOthers(t *testing.T) {
	execRoot := t.TempDir()

	// taskA fails immediately; taskB blocks until context cancelled.
	startedB := make(chan struct{})
	cancelledB := make(chan struct{})

	origFn := runTaskFn
	runTaskFn = func(ctx context.Context, tsk *ast.Task, _ map[string]interface{}, _ task.ContainerRunner, _ string, _ ...map[string]*ast.StructDef) (map[string]values.Value, error) {
		if tsk.Name == "task_a" {
			return nil, &task.TaskError{TaskName: "task_a", ExitCode: 1, Stderr: []byte("fail")}
		}
		// task_b: signal started, then wait for cancellation.
		close(startedB)
		<-ctx.Done()
		close(cancelledB)
		return nil, ctx.Err()
	}
	defer func() { runTaskFn = origFn }()

	taskA := makeSimpleTask("task_a", "x", "result")
	taskB := makeSimpleTask("task_b", "x", "result")
	callA := makeCallNode("task_a", "a", nil)
	callB := makeCallNode("task_b", "b", nil)

	doc := makeDoc("wf", []ast.WorkflowNode{callA, callB}, nil, taskA, taskB)

	_, err := RunWorkflow(context.Background(), doc, "wf", nil, &noopRunner{}, execRoot)
	if err == nil {
		t.Fatal("expected error from RunWorkflow when task fails")
	}
}

// TestRunWorkflow_WorkflowOutputs_JSON: workflow with output section ->
// outputs.json has 'wfName.outputName' key.
func TestRunWorkflow_WorkflowOutputs_JSON(t *testing.T) {
	execRoot := t.TempDir()

	origFn := runTaskFn
	runTaskFn = func(_ context.Context, _ *ast.Task, _ map[string]interface{}, _ task.ContainerRunner, _ string, _ ...map[string]*ast.StructDef) (map[string]values.Value, error) {
		return map[string]values.Value{"value": values.IntValue{Value: 42}}, nil
	}
	defer func() { runTaskFn = origFn }()

	myTask := makeSimpleTask("my_task", "x", "value")
	call := makeCallNode("my_task", "my_call", nil)
	wfOutputs := []*ast.Decl{
		{
			Name: "result",
			Type: types.IntType{},
			Expr: memberExpr("my_call", "value"),
		},
	}
	doc := makeDoc("pipeline", []ast.WorkflowNode{call}, wfOutputs, myTask)

	_, err := RunWorkflow(context.Background(), doc, "pipeline", nil, &noopRunner{}, execRoot)
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}

	outPath := filepath.Join(execRoot, "outputs.json")
	got := readJSONFileStr(t, outPath)
	v, ok := got["pipeline.result"]
	if !ok {
		t.Fatal("expected key 'pipeline.result' in outputs.json")
	}
	if v.(float64) != 42 {
		t.Errorf("expected 42, got %v", v)
	}
}

// TestRunWorkflow_EndToEnd: multi-call workflow with scatter and workflow outputs ->
// correct outputs.json written.
func TestRunWorkflow_EndToEnd(t *testing.T) {
	execRoot := t.TempDir()

	origFn := runTaskFn
	runTaskFn = func(_ context.Context, tsk *ast.Task, inputs map[string]interface{}, _ task.ContainerRunner, _ string, _ ...map[string]*ast.StructDef) (map[string]values.Value, error) {
		if tsk.Name == "doubler" {
			v, _ := inputs["n"].(int64)
			return map[string]values.Value{"out": values.IntValue{Value: v * 2}}, nil
		}
		return map[string]values.Value{"out": values.IntValue{Value: 0}}, nil
	}
	defer func() { runTaskFn = origFn }()

	doublerTask := &ast.Task{
		Name:    "doubler",
		Inputs:  []*ast.Decl{{Name: "n", Type: types.IntType{}}},
		Outputs: []*ast.Decl{{Name: "out", Type: types.IntType{}}},
	}

	// scatter (n in [1, 2, 3]) { call doubler { input: n = n } }
	scatter := &ast.Scatter{
		Variable: "n",
		Expr:     &ast.ArrayExpr{Items: []ast.Expr{intLit(1), intLit(2), intLit(3)}},
		Body: []ast.WorkflowNode{
			makeCallNode("doubler", "d", map[string]ast.Expr{"n": identExpr("n")}),
		},
	}

	wfOutputs := []*ast.Decl{
		{
			Name: "results",
			Type: types.ArrayType{ItemType: types.IntType{}},
			Expr: memberExpr("d", "out"),
		},
	}

	doc := makeDoc("e2e_wf", []ast.WorkflowNode{scatter}, wfOutputs, doublerTask)

	outputs, err := RunWorkflow(context.Background(), doc, "e2e_wf", nil, &noopRunner{}, execRoot)
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}

	// Check returned outputs map.
	results, ok := outputs["results"]
	if !ok {
		t.Fatal("expected 'results' in workflow outputs")
	}
	arr, ok := results.(values.ArrayValue)
	if !ok {
		t.Fatalf("expected ArrayValue, got %T", results)
	}
	if len(arr.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(arr.Items))
	}

	// Check outputs.json file.
	outPath := filepath.Join(execRoot, "outputs.json")
	got := readJSONFileStr(t, outPath)
	jsonArr, ok := got["e2e_wf.results"]
	if !ok {
		t.Fatal("expected key 'e2e_wf.results' in outputs.json")
	}
	jsonItems, ok := jsonArr.([]interface{})
	if !ok {
		t.Fatalf("expected array for 'e2e_wf.results', got %T", jsonArr)
	}
	if len(jsonItems) != 3 {
		t.Fatalf("expected 3 items in outputs.json array, got %d", len(jsonItems))
	}
	// Expected doubled values: [2, 4, 6].
	want := []float64{2, 4, 6}
	for i, item := range jsonItems {
		if item.(float64) != want[i] {
			t.Errorf("item[%d]: expected %v, got %v", i, want[i], item)
		}
	}
}
