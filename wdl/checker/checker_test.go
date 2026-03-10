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

// TestCoerces checks that Coerces() implements WDL coercion rules.
func TestCoerces(t *testing.T) {
	t.Fatal("not implemented")

	// WDL coercion rules (reference only — not executed yet):
	// Coerces(IntType{}, FloatType{}) == true
	// Coerces(StringType{}, FileType{}) == true
	// Coerces(StringType{}, DirectoryType{}) == true
	// Coerces(FileType{}, StringType{}) == true
	// Coerces(IntType{}, IntType{Optional:true}) == true  // T → T?
	// Coerces(ArrayType{ItemType:IntType{},Nonempty:true}, ArrayType{ItemType:IntType{},Nonempty:false}) == true  // Array[T]+ → Array[T]
	// Coerces(StringType{}, IntType{}) == false
	// Coerces(ArrayType{ItemType:StringType{}}, ArrayType{ItemType:FileType{}}) == false  // no item coercion
	_ = Coerces(types.IntType{}, types.FloatType{})
	_ = Coerces(types.StringType{}, types.FileType{})
	_ = Coerces(types.StringType{}, types.DirectoryType{})
	_ = Coerces(types.FileType{}, types.StringType{})
	_ = Coerces(types.IntType{}, types.IntType{Optional: true})
	_ = Coerces(types.ArrayType{ItemType: types.IntType{}, Nonempty: true}, types.ArrayType{ItemType: types.IntType{}, Nonempty: false})
	_ = Coerces(types.StringType{}, types.IntType{})
	_ = Coerces(types.ArrayType{ItemType: types.StringType{}}, types.ArrayType{ItemType: types.FileType{}})
}

// TestOptionalizeIdempotent checks that optionalize does not double-wrap T? to T??.
func TestOptionalizeIdempotent(t *testing.T) {
	t.Fatal("not implemented")

	// optionalize(IntType{Optional:true}) must remain Int? (not Int??)
	_ = optionalize(types.IntType{Optional: true})
}

// TestForwardRef checks that the checker accepts a task where a declaration
// references a name defined later in the same task body (forward reference).
func TestForwardRef(t *testing.T) {
	t.Fatal("not implemented")

	doc := parseFixture(t, "wdl/testdata/forward_ref.wdl")
	_, errs := Check(doc)
	if len(errs) != 0 {
		t.Fatalf("expected 0 errors, got %d: %v", len(errs), errs)
	}
}

// TestCycleDetection checks that the checker reports an error when two task
// declarations form a mutual dependency cycle.
func TestCycleDetection(t *testing.T) {
	t.Fatal("not implemented")

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
	t.Fatal("not implemented")

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
	t.Fatal("not implemented")

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
	t.Fatal("not implemented")

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
	t.Fatal("not implemented")

	doc := parseFixture(t, "wdl/testdata/struct_literal.wdl")
	_, errs := Check(doc)
	if len(errs) != 0 {
		t.Fatalf("expected 0 errors for valid struct literal, got %d: %v", len(errs), errs)
	}
}

// TestStructFieldAccess checks that the checker resolves a field access
// expression (p.x) to the correct type in the TypeEnv.
func TestStructFieldAccess(t *testing.T) {
	t.Fatal("not implemented")

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

// TestStructUndefined checks that the checker reports an error when a struct
// literal references a struct name that is not defined in the document.
func TestStructUndefined(t *testing.T) {
	t.Fatal("not implemented")

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
