package workflow

import (
	"testing"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/eval"
	"github.com/bioflowy/flowy/wdl/types"
	"github.com/bioflowy/flowy/wdl/values"
)

// TestValueToJSON_Primitives tests that primitive WDL values convert correctly.
func TestValueToJSON_Primitives(t *testing.T) {
	tests := []struct {
		name     string
		input    values.Value
		expected interface{}
	}{
		{"Int", values.IntValue{Type: types.IntType{}, Value: 42}, int64(42)},
		{"Float", values.FloatValue{Type: types.FloatType{}, Value: 3.14}, float64(3.14)},
		{"String", values.StringValue{Type: types.StringType{}, Value: "hi"}, "hi"},
		{"BoolTrue", values.BooleanValue{Type: types.BooleanType{}, Value: true}, true},
		{"BoolFalse", values.BooleanValue{Type: types.BooleanType{}, Value: false}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := valueToJSON(tc.input)
			if got != tc.expected {
				t.Errorf("valueToJSON(%v) = %v (%T), want %v (%T)", tc.input, got, got, tc.expected, tc.expected)
			}
		})
	}
}

// TestValueToJSON_File tests File and Directory values are converted to string paths.
func TestValueToJSON_File(t *testing.T) {
	fv := values.FileValue{Type: types.FileType{}, Value: "/tmp/x"}
	got := valueToJSON(fv)
	if got != "/tmp/x" {
		t.Errorf("expected \"/tmp/x\", got %v", got)
	}

	dv := values.DirectoryValue{Type: types.DirectoryType{}, Value: "/tmp/dir"}
	got = valueToJSON(dv)
	if got != "/tmp/dir" {
		t.Errorf("expected \"/tmp/dir\", got %v", got)
	}
}

// TestValueToJSON_Array tests that ArrayValue is recursively converted.
func TestValueToJSON_Array(t *testing.T) {
	av := values.ArrayValue{
		Type:  types.ArrayType{ItemType: types.IntType{}},
		Items: []values.Value{
			values.IntValue{Type: types.IntType{}, Value: 1},
			values.IntValue{Type: types.IntType{}, Value: 2},
		},
	}
	got := valueToJSON(av)
	arr, ok := got.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", got)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 items, got %d", len(arr))
	}
	if arr[0] != int64(1) || arr[1] != int64(2) {
		t.Errorf("expected [1, 2], got %v", arr)
	}
}

// TestValueToJSON_Map tests that MapValue with StringValue keys converts correctly.
func TestValueToJSON_Map(t *testing.T) {
	mv := values.MapValue{
		Type: types.MapType{KeyType: types.StringType{}, ValType: types.IntType{}},
		Entries: []values.MapEntry{
			{Key: values.StringValue{Type: types.StringType{}, Value: "a"}, Value: values.IntValue{Type: types.IntType{}, Value: 10}},
			{Key: values.StringValue{Type: types.StringType{}, Value: "b"}, Value: values.IntValue{Type: types.IntType{}, Value: 20}},
		},
	}
	got := valueToJSON(mv)
	m, ok := got.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", got)
	}
	if m["a"] != int64(10) {
		t.Errorf("expected m[\"a\"]=10, got %v", m["a"])
	}
	if m["b"] != int64(20) {
		t.Errorf("expected m[\"b\"]=20, got %v", m["b"])
	}
}

// TestValueToJSON_Null tests that NullValue converts to nil.
func TestValueToJSON_Null(t *testing.T) {
	got := valueToJSON(values.NullValue{})
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

// TestValueToJSON_Nested tests that nested ArrayValue containing MapValues converts correctly.
func TestValueToJSON_Nested(t *testing.T) {
	innerMap := values.MapValue{
		Type: types.MapType{KeyType: types.StringType{}, ValType: types.StringType{}},
		Entries: []values.MapEntry{
			{Key: values.StringValue{Type: types.StringType{}, Value: "k"}, Value: values.StringValue{Type: types.StringType{}, Value: "v"}},
		},
	}
	outerArr := values.ArrayValue{
		Type:  types.ArrayType{ItemType: types.AnyType{}},
		Items: []values.Value{innerMap},
	}

	got := valueToJSON(outerArr)
	arr, ok := got.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", got)
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 item, got %d", len(arr))
	}
	m, ok := arr[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{} at arr[0], got %T", arr[0])
	}
	if m["k"] != "v" {
		t.Errorf("expected m[\"k\"]=\"v\", got %v", m["k"])
	}
}

// makeStringWorkflow builds a minimal ast.Workflow with a single String input declaration.
// The returned *ast.Workflow has Name set to wfName and one Decl with Name=inputName.
// If defaultVal is non-empty, the Decl's Expr is set to a StringExpr literal.
// If optional is true, the Decl's Type is set to StringType{Optional:true}.
func makeStringWorkflow(wfName, inputName, defaultVal string, optional bool) *ast.Workflow {
	decl := &ast.Decl{
		Type: types.StringType{Optional: optional},
		Name: inputName,
	}
	if defaultVal != "" {
		decl.Expr = &ast.StringExpr{
			Parts: []ast.StringPart{&ast.RawStringPart{Val: defaultVal}},
		}
	}
	return &ast.Workflow{
		Name:   wfName,
		Inputs: []*ast.Decl{decl},
	}
}

// TestBindWorkflowInputs_FullyQualified verifies that a fully-qualified JSON key
// (e.g. "hello.name") is resolved correctly for a workflow named "hello" with
// input "name".
func TestBindWorkflowInputs_FullyQualified(t *testing.T) {
	wf := makeStringWorkflow("hello", "name", "", false)
	jsonInputs := map[string]interface{}{
		"hello.name": "world",
	}

	env, err := bindWorkflowInputs(wf, jsonInputs)
	if err != nil {
		t.Fatalf("bindWorkflowInputs returned error: %v", err)
	}

	got, ok := env.Lookup("name")
	if !ok {
		t.Fatal("expected 'name' to be bound in env, but not found")
	}
	sv, ok := got.(values.StringValue)
	if !ok {
		t.Fatalf("expected StringValue, got %T", got)
	}
	if sv.Value != "world" {
		t.Errorf("expected value \"world\", got %q", sv.Value)
	}
}

// TestBindWorkflowInputs_UnqualifiedFallback verifies that an unqualified JSON key
// (e.g. "name") is accepted as a fallback when no fully-qualified key exists.
func TestBindWorkflowInputs_UnqualifiedFallback(t *testing.T) {
	wf := makeStringWorkflow("hello", "name", "", false)
	jsonInputs := map[string]interface{}{
		"name": "fallback",
	}

	env, err := bindWorkflowInputs(wf, jsonInputs)
	if err != nil {
		t.Fatalf("bindWorkflowInputs returned error: %v", err)
	}

	got, ok := env.Lookup("name")
	if !ok {
		t.Fatal("expected 'name' to be bound in env, but not found")
	}
	sv, ok := got.(values.StringValue)
	if !ok {
		t.Fatalf("expected StringValue, got %T", got)
	}
	if sv.Value != "fallback" {
		t.Errorf("expected value \"fallback\", got %q", sv.Value)
	}
}

// TestBindWorkflowInputs_FullyQualifiedTakesPrecedence verifies that when both
// "hello.name" and "name" are present in the JSON map, the fully-qualified value wins.
func TestBindWorkflowInputs_FullyQualifiedTakesPrecedence(t *testing.T) {
	wf := makeStringWorkflow("hello", "name", "", false)
	jsonInputs := map[string]interface{}{
		"hello.name": "qualified",
		"name":       "unqualified",
	}

	env, err := bindWorkflowInputs(wf, jsonInputs)
	if err != nil {
		t.Fatalf("bindWorkflowInputs returned error: %v", err)
	}

	got, ok := env.Lookup("name")
	if !ok {
		t.Fatal("expected 'name' to be bound in env, but not found")
	}
	sv, ok := got.(values.StringValue)
	if !ok {
		t.Fatalf("expected StringValue, got %T", got)
	}
	if sv.Value != "qualified" {
		t.Errorf("expected \"qualified\" to take precedence, got %q", sv.Value)
	}
}

// TestBindWorkflowInputs_DefaultUsedWhenMissing verifies that when no JSON key
// is provided for an input with a default expression, the default value is used.
func TestBindWorkflowInputs_DefaultUsedWhenMissing(t *testing.T) {
	wf := makeStringWorkflow("hello", "greeting", "default_greeting", false)
	jsonInputs := map[string]interface{}{} // no key for "greeting"

	env, err := bindWorkflowInputs(wf, jsonInputs)
	if err != nil {
		t.Fatalf("bindWorkflowInputs returned error: %v", err)
	}

	got, ok := env.Lookup("greeting")
	if !ok {
		t.Fatal("expected 'greeting' to be bound in env, but not found")
	}
	sv, ok := got.(values.StringValue)
	if !ok {
		t.Fatalf("expected StringValue, got %T", got)
	}
	if sv.Value != "default_greeting" {
		t.Errorf("expected value \"default_greeting\", got %q", sv.Value)
	}
}

// TestEvalCallInputs tests that evalCallInputs correctly evaluates input expressions.
func TestEvalCallInputs(t *testing.T) {
	// Build an env with "x" = IntValue(99)
	env := eval.NewEnv(nil)
	env.Define("x", values.IntValue{Type: types.IntType{}, Value: 99})

	// Build a call with input "myParam" = IdentExpr("x")
	call := &ast.Call{
		Callee: []string{"some_task"},
		Inputs: map[string]ast.Expr{
			"myParam": &ast.IdentExpr{Name: "x"},
		},
	}

	result, err := evalCallInputs(call, env)
	if err != nil {
		t.Fatalf("evalCallInputs returned error: %v", err)
	}
	if result["myParam"] != int64(99) {
		t.Errorf("expected myParam=99, got %v", result["myParam"])
	}
}
