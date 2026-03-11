// Package checker_test contains tests for the wdl/checker type-checking package.
// All tests currently call t.Fatal("not implemented") as stubs; they will be
// filled in by Plans 02-04 in Phase 3.
package checker

import (
	"os"
	"strings"
	"testing"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/parser"
	"github.com/bioflowy/flowy/wdl/types"
)

// parseFixture reads and parses a WDL file relative to the module root.
func parseFixture(t *testing.T, path string) *ast.Document {
	t.Helper()
	src, err := os.ReadFile("../../" + path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	doc, err := parser.Parse(path, src)
	if err != nil {
		t.Fatalf("parse fixture %s: %v", path, err)
	}
	return doc
}

// TestCoerces checks that Coerces() implements WDL 1.2 shallow coercion rules.
func TestCoerces(t *testing.T) {
	cases := []struct {
		name string
		from types.Type
		to   types.Type
		want bool
	}{
		// Rule 2: Int -> Float
		{"Int->Float", types.IntType{}, types.FloatType{}, true},
		// Rule 3a: String -> File
		{"String->File", types.StringType{}, types.FileType{}, true},
		// Rule 3b: String -> Directory
		{"String->Directory", types.StringType{}, types.DirectoryType{}, true},
		// Rule 4: File -> String
		{"File->String", types.FileType{}, types.StringType{}, true},
		// Rule 1: T -> T? (same base type, target is optional)
		{"Int->Int?", types.IntType{}, types.IntType{Optional: true}, true},
		// Rule 5: Array[T]+ -> Array[T] (nonempty drop, shallow item check)
		{
			"Array[Int]+->Array[Int]",
			types.ArrayType{ItemType: types.IntType{}, Nonempty: true},
			types.ArrayType{ItemType: types.IntType{}, Nonempty: false},
			true,
		},
		// False: unrelated types
		{"String->Int", types.StringType{}, types.IntType{}, false},
		// True: Array item types coerce recursively (String → File)
		{
			"Array[String]->Array[File] (recursive coercion)",
			types.ArrayType{ItemType: types.StringType{}},
			types.ArrayType{ItemType: types.FileType{}},
			true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Coerces(tc.from, tc.to)
			if got != tc.want {
				t.Errorf("Coerces(%v, %v) = %v; want %v", tc.from, tc.to, got, tc.want)
			}
		})
	}
}

// TestOptionalizeIdempotent checks that optionalize does not double-wrap T? to T??,
// and that calling optionalize twice is the same as calling it once.
func TestOptionalizeIdempotent(t *testing.T) {
	// optionalize of a non-optional type must make it optional
	result := optionalize(types.IntType{})
	if !result.IsOptional() {
		t.Errorf("optionalize(IntType{}) must be optional; got IsOptional()=false")
	}
	if result.TypeName() != "Int?" {
		t.Errorf("optionalize(IntType{}).TypeName() = %q; want %q", result.TypeName(), "Int?")
	}

	// optionalize on already-optional type must be a no-op (idempotency guard)
	alreadyOpt := types.IntType{Optional: true}
	result2 := optionalize(alreadyOpt)
	if result2.TypeName() != "Int?" {
		t.Errorf("optionalize(IntType{Optional:true}).TypeName() = %q; want %q (must not double-wrap)",
			result2.TypeName(), "Int?")
	}

	// Applying optionalize twice must equal applying it once
	once := optionalize(types.IntType{})
	twice := optionalize(optionalize(types.IntType{}))
	if once.TypeName() != twice.TypeName() {
		t.Errorf("optionalize applied twice = %q; want %q (idempotent)",
			twice.TypeName(), once.TypeName())
	}
}

// TestForwardRef checks that the checker accepts a task where a declaration
// references a name defined later in the same task body (forward reference).
func TestForwardRef(t *testing.T) {
	doc := parseFixture(t, "wdl/testdata/forward_ref.wdl")
	_, errs := Check(doc)
	if len(errs) != 0 {
		t.Fatalf("expected 0 errors, got %d: %v", len(errs), errs)
	}
}

// TestCycleDetection checks that the checker reports an error when two task
// declarations form a mutual dependency cycle.
func TestCycleDetection(t *testing.T) {
	// Build an in-memory Document: task with decl a = b and decl b = a.
	doc := &ast.Document{
		Tasks: []*ast.Task{
			{
				Name: "cycle_task",
				Decls: []*ast.Decl{
					{
						Type: types.IntType{},
						Name: "a",
						Expr: &ast.IdentExpr{Name: "b"},
					},
					{
						Type: types.IntType{},
						Name: "b",
						Expr: &ast.IdentExpr{Name: "a"},
					},
				},
			},
		},
	}
	_, errs := Check(doc)
	if len(errs) == 0 {
		t.Fatal("expected cycle error, got none")
	}
	if !strings.Contains(errs[0].Error(), "cycle") {
		t.Fatalf("expected error containing 'cycle', got: %v", errs[0])
	}
}

// TestTypeMismatch checks that the checker reports a type error when a String
// variable is initialized from an Int-typed identifier.
func TestTypeMismatch(t *testing.T) {
	doc := parseFixture(t, "wdl/testdata/type_mismatch.wdl")
	_, errs := Check(doc)
	if len(errs) == 0 {
		t.Fatal("expected type mismatch error, got none")
	}
	if !strings.Contains(errs[0].Error(), "type error") {
		t.Fatalf("expected error containing 'type error', got: %v", errs[0])
	}
}

// TestScatterPromotion checks that the checker promotes a scatter body String
// declaration to Array[String] in the enclosing scope.
func TestScatterPromotion(t *testing.T) {
	doc := parseFixture(t, "wdl/testdata/scatter_if.wdl")
	env, errs := Check(doc)
	if len(errs) != 0 {
		t.Fatalf("expected 0 errors, got %d: %v", len(errs), errs)
	}
	// The scatter output "scatter_out" must be Array[String] in the TypeEnv.
	got, ok := env["scatter_out"]
	if !ok {
		t.Fatal("scatter_out not found in TypeEnv")
	}
	if got.TypeName() != "Array[String]" {
		t.Fatalf("scatter_out: expected Array[String], got %s", got.TypeName())
	}
}

// TestIfPromotion checks that the checker wraps an if-block output declaration
// as optional (T?) in the enclosing scope.
func TestIfPromotion(t *testing.T) {
	doc := parseFixture(t, "wdl/testdata/scatter_if.wdl")
	env, errs := Check(doc)
	if len(errs) != 0 {
		t.Fatalf("expected 0 errors, got %d: %v", len(errs), errs)
	}
	// The if output "if_out" must be optional (String?) in the TypeEnv.
	got, ok := env["if_out"]
	if !ok {
		t.Fatal("if_out not found in TypeEnv")
	}
	if !got.IsOptional() {
		t.Fatalf("if_out: expected optional type, got %s", got.TypeName())
	}
}

// TestStructLiteral checks that the checker validates a struct literal where
// all fields match the declared struct type.
func TestStructLiteral(t *testing.T) {
	doc := parseFixture(t, "wdl/testdata/struct_literal.wdl")
	_, errs := Check(doc)
	if len(errs) != 0 {
		t.Fatalf("expected 0 errors for valid struct literal, got %d: %v", len(errs), errs)
	}
}

// TestStructFieldAccess checks that the checker resolves a field access
// expression (p.x) to the correct type in the TypeEnv.
func TestStructFieldAccess(t *testing.T) {
	doc := parseFixture(t, "wdl/testdata/struct_literal.wdl")
	env, errs := Check(doc)
	if len(errs) != 0 {
		t.Fatalf("expected 0 errors, got %d: %v", len(errs), errs)
	}
	// The field access result "px" must be Int.
	got, ok := env["px"]
	if !ok {
		t.Fatal("px not found in TypeEnv")
	}
	if got.TypeName() != "Int" {
		t.Fatalf("px: expected Int, got %s", got.TypeName())
	}
}

// TestCallOutputInWorkflow checks that a workflow output referencing a call output
// (e.g., sayHello.greeting) resolves correctly without type errors.
func TestCallOutputInWorkflow(t *testing.T) {
	doc := parseFixture(t, "wdl/testdata/hello.wdl")
	env, errs := Check(doc)
	if len(errs) != 0 {
		t.Fatalf("expected 0 errors, got %d: %v", len(errs), errs)
	}
	got, ok := env["result"]
	if !ok {
		t.Fatal("result not found in TypeEnv")
	}
	if got.TypeName() != "String" {
		t.Fatalf("result: expected String, got %s", got.TypeName())
	}
}

// TestCallOutputInline checks that an inline task+workflow with a call output
// reference type-checks without errors.
func TestCallOutputInline(t *testing.T) {
	doc := parseFixture(t, "wdl/testdata/call_output.wdl")
	env, errs := Check(doc)
	if len(errs) != 0 {
		t.Fatalf("expected 0 errors, got %d: %v", len(errs), errs)
	}
	got, ok := env["msg"]
	if !ok {
		t.Fatal("msg not found in TypeEnv")
	}
	if got.TypeName() != "String" {
		t.Fatalf("msg: expected String, got %s", got.TypeName())
	}
}

// TestScatterCallPromotion checks that a call inside a scatter block has its
// outputs promoted to Array[T] in the enclosing workflow scope.
func TestScatterCallPromotion(t *testing.T) {
	doc := parseFixture(t, "wdl/testdata/scatter_call.wdl")
	env, errs := Check(doc)
	if len(errs) != 0 {
		t.Fatalf("expected 0 errors, got %d: %v", len(errs), errs)
	}
	got, ok := env["results"]
	if !ok {
		t.Fatal("results not found in TypeEnv")
	}
	if got.TypeName() != "Array[String]" {
		t.Fatalf("results: expected Array[String], got %s", got.TypeName())
	}
}

// TestConditionalCallPromotion checks that a call inside a conditional block
// has its outputs promoted to T? in the enclosing workflow scope.
func TestConditionalCallPromotion(t *testing.T) {
	// Build in-memory AST: task with String output, workflow with conditional call.
	doc := &ast.Document{
		Tasks: []*ast.Task{
			{
				Name: "innerTask",
				Outputs: []*ast.Decl{
					{
						Type: types.StringType{},
						Name: "out",
					},
				},
			},
		},
		Workflows: []*ast.Workflow{
			{
				Name: "condWf",
				Inputs: []*ast.Decl{
					{
						Type: types.BooleanType{},
						Name: "flag",
					},
				},
				Body: []ast.WorkflowNode{
					&ast.Conditional{
						Expr: &ast.IdentExpr{Name: "flag"},
						Body: []ast.WorkflowNode{
							&ast.Call{
								Callee: []string{"innerTask"},
							},
						},
					},
				},
				Outputs: []*ast.Decl{
					{
						Type: types.StringType{Optional: true},
						Name: "cond_out",
						Expr: &ast.GetExpr{
							Object: &ast.IdentExpr{Name: "innerTask"},
							Member: "out",
						},
					},
				},
			},
		},
	}
	env, errs := Check(doc)
	if len(errs) != 0 {
		t.Fatalf("expected 0 errors, got %d: %v", len(errs), errs)
	}
	got, ok := env["cond_out"]
	if !ok {
		t.Fatal("cond_out not found in TypeEnv")
	}
	if got.TypeName() != "String?" {
		t.Fatalf("cond_out: expected String?, got %s", got.TypeName())
	}
}

// TestNestedIfScatterPromotion checks that when a variable is declared inside
// a scatter block nested inside a conditional block (if { scatter { Int x } }),
// the type becomes Array[Int]? (not Array[Int?]). Conversely, scatter { if { Int x } }
// should produce Array[Int?].
func TestNestedIfScatterPromotion(t *testing.T) {
	doc := parseFixture(t, "wdl/testdata/if_scatter_nested.wdl")
	env, errs := Check(doc)
	if len(errs) != 0 {
		t.Fatalf("expected 0 errors, got %d: %v", len(errs), errs)
	}

	// if { scatter { Int result } } -> Array[Int]?
	got, ok := env["result"]
	if !ok {
		t.Fatal("result not found in TypeEnv")
	}
	expected := "Array[Int]?"
	if got.TypeName() != expected {
		t.Fatalf("result: expected %s, got %s", expected, got.TypeName())
	}

	// scatter { if { Int inner } } -> Array[Int?]
	got2, ok := env["inner"]
	if !ok {
		t.Fatal("inner not found in TypeEnv")
	}
	expected2 := "Array[Int?]"
	if got2.TypeName() != expected2 {
		t.Fatalf("inner: expected %s, got %s", expected2, got2.TypeName())
	}
}

// TestStructUndefined checks that the checker reports an error when a struct
// literal references a struct name that is not defined in the document.
func TestStructUndefined(t *testing.T) {
	// Build an in-memory Document with a StructExpr referencing non-existent "Bogus".
	doc := &ast.Document{
		Tasks: []*ast.Task{
			{
				Name: "bogus_task",
				Decls: []*ast.Decl{
					{
						Type: types.StructType{Name: "Bogus"},
						Name: "val",
						Expr: &ast.StructExpr{
							TypeName: "Bogus",
							Members:  []ast.StructMember{},
						},
					},
				},
			},
		},
	}
	_, errs := Check(doc)
	if len(errs) == 0 {
		t.Fatal("expected undefined struct error, got none")
	}
}
