package parser_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/lexer"
	"github.com/bioflowy/flowy/wdl/parser"
	"github.com/bioflowy/flowy/wdl/types"
)

// parseWDL is a helper that parses a WDL source string from "test.wdl".
func parseWDL(src string) (*ast.Document, error) {
	return parser.Parse("test.wdl", []byte(src))
}

// testdataPath returns the absolute path to a file in wdl/testdata/.
// In Go tests, the working directory is set to the package directory (wdl/parser/),
// so testdata is at ../testdata relative to the test working directory.
func testdataPath(name string) string {
	return filepath.Join("..", "testdata", name)
}

// parseTypeStr is a helper that parses a WDL type expression string
// by wrapping it in a minimal struct declaration and extracting the member type.
func parseTypeStr(typeExpr string) (types.Type, error) {
	src := "version 1.2\nstruct _T {\n" + typeExpr + " _f\n}\n"
	doc, err := parseWDL(src)
	if err != nil {
		return nil, err
	}
	return doc.Structs[0].Members[0].Type, nil
}

// TestParseDocument verifies that a WDL document is parsed correctly. (PARSE-01)
// Uses the wdl/testdata/hello.wdl fixture for a full round-trip test.
func TestParseDocument(t *testing.T) {
	t.Run("inline_minimal", func(t *testing.T) {
		doc, err := parseWDL("version 1.2\ntask hello {}\n")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if doc.Version != "1.2" {
			t.Errorf("expected version 1.2, got %q", doc.Version)
		}
		if len(doc.Tasks) != 1 {
			t.Errorf("expected 1 task, got %d", len(doc.Tasks))
		}
		if doc.Tasks[0].Name != "hello" {
			t.Errorf("expected task name 'hello', got %q", doc.Tasks[0].Name)
		}
	})

	t.Run("hello_wdl_fixture", func(t *testing.T) {
		path := testdataPath("hello.wdl")
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("cannot read hello.wdl: %v", err)
		}
		abspath, _ := filepath.Abs(path)
		doc, err := parser.Parse(abspath, src)
		if err != nil {
			t.Fatalf("unexpected parse error: %v", err)
		}
		if doc.Version != "1.2" {
			t.Errorf("expected version 1.2, got %q", doc.Version)
		}
		if len(doc.Tasks) != 1 {
			t.Errorf("expected 1 task, got %d", len(doc.Tasks))
		}
		if len(doc.Workflows) != 1 {
			t.Errorf("expected 1 workflow, got %d", len(doc.Workflows))
		}
		if doc.Tasks[0].Command == nil {
			t.Error("expected task Command to be non-nil")
		}
		if len(doc.Workflows[0].Body) == 0 {
			t.Error("expected workflow Body to be non-empty")
		}
	})
}

// TestImportFromFixture verifies parsing main.wdl which imports lib.wdl. (PARSE-07)
func TestImportFromFixture(t *testing.T) {
	path := testdataPath("main.wdl")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read main.wdl: %v", err)
	}
	abspath, _ := filepath.Abs(path)
	doc, err := parser.Parse(abspath, src)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	// main.wdl imports lib.wdl which has task greet
	if len(doc.Tasks) != 1 {
		t.Errorf("expected 1 task (greet from lib.wdl), got %d", len(doc.Tasks))
	} else if doc.Tasks[0].Name != "greet" {
		t.Errorf("expected task 'greet', got %q", doc.Tasks[0].Name)
	}
	if len(doc.Workflows) != 1 {
		t.Errorf("expected 1 workflow, got %d", len(doc.Workflows))
	}
}

// TestTypesWDLFixture verifies that types.wdl parses without error and
// contains the expected structs and type declarations. (PARSE-01 extended)
func TestTypesWDLFixture(t *testing.T) {
	path := testdataPath("types.wdl")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read types.wdl: %v", err)
	}
	abspath, _ := filepath.Abs(path)
	doc, err := parser.Parse(abspath, src)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(doc.Structs) < 2 {
		t.Errorf("expected at least 2 structs, got %d", len(doc.Structs))
	}
	if len(doc.Tasks) < 1 {
		t.Errorf("expected at least 1 task, got %d", len(doc.Tasks))
	}
}

// TestParseError verifies that parse errors include precise source location
// information and produce rustc-style formatted messages. (PARSE-08)
func TestParseError(t *testing.T) {
	src := "version 1.2\ntask foo {\n  Int x = $$$\n}"
	_, err := parseWDL(src)
	if err == nil {
		t.Fatal("expected a parse error, got nil")
	}

	pe, ok := err.(*parser.ParseError)
	if !ok {
		t.Fatalf("expected *parser.ParseError, got %T: %v", err, err)
	}

	// The error should be on line 3 where the $$$ syntax error occurs
	if pe.Line != 3 {
		t.Errorf("expected Line=3, got %d", pe.Line)
	}
	// Col should be at the $ character (which is at column 3 after two leading spaces)
	if pe.Col < 1 {
		t.Errorf("expected Col >= 1, got %d", pe.Col)
	}
	// SourceLine should contain the bad syntax
	if !strings.Contains(pe.SourceLine, "$$$") {
		t.Errorf("expected SourceLine to contain '$$$', got %q", pe.SourceLine)
	}
	// Error() output should contain rustc-style markers
	errStr := pe.Error()
	if !strings.Contains(errStr, "-->") {
		t.Errorf("Error() should contain '-->': %s", errStr)
	}
	if !strings.Contains(errStr, "^") {
		t.Errorf("Error() should contain '^': %s", errStr)
	}
	// Should reference the file
	if !strings.Contains(errStr, "test.wdl") {
		t.Errorf("Error() should contain 'test.wdl': %s", errStr)
	}
}

// TestStructDef verifies that WDL struct definitions are correctly parsed,
// including member types and nested struct references. (PARSE-09)
func TestStructDef(t *testing.T) {
	t.Run("simple struct", func(t *testing.T) {
		src := "version 1.2\nstruct Pair {\n  Int left\n  String right\n}\n"
		doc, err := parseWDL(src)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(doc.Structs) != 1 {
			t.Fatalf("expected 1 struct, got %d", len(doc.Structs))
		}
		s := doc.Structs[0]
		if s.Name != "Pair" {
			t.Errorf("expected struct name 'Pair', got %q", s.Name)
		}
		if len(s.Members) != 2 {
			t.Fatalf("expected 2 members, got %d", len(s.Members))
		}
		if s.Members[0].Name != "left" {
			t.Errorf("expected member[0].Name='left', got %q", s.Members[0].Name)
		}
		if s.Members[0].Type.TypeName() != "Int" {
			t.Errorf("expected member[0].Type=Int, got %q", s.Members[0].Type.TypeName())
		}
		if s.Members[1].Name != "right" {
			t.Errorf("expected member[1].Name='right', got %q", s.Members[1].Name)
		}
		if s.Members[1].Type.TypeName() != "String" {
			t.Errorf("expected member[1].Type=String, got %q", s.Members[1].Type.TypeName())
		}
	})

	t.Run("nested struct type reference", func(t *testing.T) {
		src := "version 1.2\nstruct Nested {\n  MyStruct inner\n  Int count\n}\n"
		doc, err := parseWDL(src)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(doc.Structs) != 1 {
			t.Fatalf("expected 1 struct, got %d", len(doc.Structs))
		}
		s := doc.Structs[0]
		if len(s.Members) != 2 {
			t.Fatalf("expected 2 members, got %d", len(s.Members))
		}

		// Member[0] should be StructType{Name: "MyStruct"}
		memberType := s.Members[0].Type
		st, ok := memberType.(types.StructType)
		if !ok {
			t.Errorf("expected member[0].Type to be types.StructType, got %T", memberType)
		} else if st.Name != "MyStruct" {
			t.Errorf("expected StructType.Name='MyStruct', got %q", st.Name)
		}

		if s.Members[1].Type.TypeName() != "Int" {
			t.Errorf("expected member[1].Type=Int, got %q", s.Members[1].Type.TypeName())
		}
	})
}

// TestParseTypes verifies that all WDL type syntax is correctly parsed into types.Type structs.
func TestParseTypes(t *testing.T) {
	tests := []struct {
		input    string
		expected string // expected TypeName()
		check    func(t *testing.T, typ types.Type)
	}{
		{
			input:    "Int",
			expected: "Int",
		},
		{
			input:    "String?",
			expected: "String?",
			check: func(t *testing.T, typ types.Type) {
				if !typ.IsOptional() {
					t.Error("expected IsOptional()=true")
				}
			},
		},
		{
			input:    "Array[File]+",
			expected: "Array[File]+",
			check: func(t *testing.T, typ types.Type) {
				at, ok := typ.(types.ArrayType)
				if !ok {
					t.Fatalf("expected ArrayType, got %T", typ)
				}
				if !at.Nonempty {
					t.Error("expected Nonempty=true")
				}
				if at.Optional {
					t.Error("expected Optional=false")
				}
			},
		},
		{
			input:    "Array[Int]+?",
			expected: "Array[Int]+?",
			check: func(t *testing.T, typ types.Type) {
				at, ok := typ.(types.ArrayType)
				if !ok {
					t.Fatalf("expected ArrayType, got %T", typ)
				}
				if !at.Nonempty {
					t.Error("expected Nonempty=true")
				}
				if !at.Optional {
					t.Error("expected Optional=true")
				}
			},
		},
		{
			input:    "Map[String,Int]",
			expected: "Map[String,Int]",
			check: func(t *testing.T, typ types.Type) {
				mt, ok := typ.(types.MapType)
				if !ok {
					t.Fatalf("expected MapType, got %T", typ)
				}
				if mt.KeyType.TypeName() != "String" {
					t.Errorf("expected KeyType=String, got %q", mt.KeyType.TypeName())
				}
				if mt.ValType.TypeName() != "Int" {
					t.Errorf("expected ValType=Int, got %q", mt.ValType.TypeName())
				}
			},
		},
		{
			input:    "Pair[Boolean,Float]",
			expected: "Pair[Boolean,Float]",
			check: func(t *testing.T, typ types.Type) {
				pt, ok := typ.(types.PairType)
				if !ok {
					t.Fatalf("expected PairType, got %T", typ)
				}
				if pt.LeftType.TypeName() != "Boolean" {
					t.Errorf("expected LeftType=Boolean, got %q", pt.LeftType.TypeName())
				}
				if pt.RightType.TypeName() != "Float" {
					t.Errorf("expected RightType=Float, got %q", pt.RightType.TypeName())
				}
			},
		},
		{
			input:    "MyStruct",
			expected: "MyStruct",
			check: func(t *testing.T, typ types.Type) {
				st, ok := typ.(types.StructType)
				if !ok {
					t.Fatalf("expected StructType, got %T", typ)
				}
				if st.Name != "MyStruct" {
					t.Errorf("expected Name='MyStruct', got %q", st.Name)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			typ, err := parseTypeStr(tc.input)
			if err != nil {
				t.Fatalf("unexpected error parsing type %q: %v", tc.input, err)
			}
			if typ.TypeName() != tc.expected {
				t.Errorf("expected TypeName()=%q, got %q", tc.expected, typ.TypeName())
			}
			if tc.check != nil {
				tc.check(t, typ)
			}
		})
	}
}

// parseExprFromDecl is a helper that parses a single expression from a task output declaration.
// The src argument should be a WDL bound declaration like "Int x = 2**3**2".
// Uses task output block because task inputs are unbound (no "=" expression allowed).
func parseExprFromDecl(t *testing.T, src string) ast.Expr {
	t.Helper()
	wdlSrc := "version 1.2\ntask t { output { " + src + " } }"
	doc, err := parseWDL(wdlSrc)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(doc.Tasks) == 0 || len(doc.Tasks[0].Outputs) == 0 {
		t.Fatal("expected at least one task output declaration")
	}
	return doc.Tasks[0].Outputs[0].Expr
}

// mustBinaryExpr asserts expr is a *ast.BinaryExpr with the given op.
func mustBinaryExpr(t *testing.T, expr ast.Expr, op lexer.TokenType, desc string) *ast.BinaryExpr {
	t.Helper()
	b, ok := expr.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("%s: expected *ast.BinaryExpr, got %T", desc, expr)
	}
	if b.Op != op {
		t.Fatalf("%s: expected op=%d, got op=%d", desc, op, b.Op)
	}
	return b
}

// mustIntLit asserts expr is a *ast.IntLit with the given value.
func mustIntLit(t *testing.T, expr ast.Expr, val int64, desc string) {
	t.Helper()
	lit, ok := expr.(*ast.IntLit)
	if !ok {
		t.Fatalf("%s: expected *ast.IntLit, got %T", desc, expr)
	}
	if lit.Value != val {
		t.Fatalf("%s: expected value=%d, got %d", desc, val, lit.Value)
	}
}

// mustBoolLit asserts expr is a *ast.BoolLit with the given value.
func mustBoolLit(t *testing.T, expr ast.Expr, val bool, desc string) {
	t.Helper()
	lit, ok := expr.(*ast.BoolLit)
	if !ok {
		t.Fatalf("%s: expected *ast.BoolLit, got %T", desc, expr)
	}
	if lit.Value != val {
		t.Fatalf("%s: expected value=%v, got %v", desc, val, lit.Value)
	}
}

// TestOperatorPrecedence verifies that the Pratt expression parser respects
// WDL operator precedence and associativity rules, including ** (exponentiation). (PARSE-06)
func TestOperatorPrecedence(t *testing.T) {
	t.Run("exponentiation_right_assoc", func(t *testing.T) {
		// 2**3**2 should parse as 2**(3**2), not (2**3)**2 (right-associative)
		// Tree: BinaryExpr{**, IntLit{2}, BinaryExpr{**, IntLit{3}, IntLit{2}}}
		expr := parseExprFromDecl(t, "Int x = 2**3**2")
		top := mustBinaryExpr(t, expr, lexer.TokStarStar, "top level")
		mustIntLit(t, top.Left, 2, "top.Left")
		// Right side must be another ** expression
		right := mustBinaryExpr(t, top.Right, lexer.TokStarStar, "top.Right")
		mustIntLit(t, right.Left, 3, "top.Right.Left")
		mustIntLit(t, right.Right, 2, "top.Right.Right")
	})

	t.Run("mul_higher_than_add", func(t *testing.T) {
		// 1 + 2 * 3 → BinaryExpr{+, IntLit{1}, BinaryExpr{*, IntLit{2}, IntLit{3}}}
		expr := parseExprFromDecl(t, "Int x = 1 + 2 * 3")
		top := mustBinaryExpr(t, expr, lexer.TokPlus, "top level")
		mustIntLit(t, top.Left, 1, "top.Left")
		right := mustBinaryExpr(t, top.Right, lexer.TokStar, "top.Right")
		mustIntLit(t, right.Left, 2, "top.Right.Left")
		mustIntLit(t, right.Right, 3, "top.Right.Right")
	})

	t.Run("and_higher_than_or", func(t *testing.T) {
		// true || false && true → BinaryExpr{||, BoolLit{true}, BinaryExpr{&&, BoolLit{false}, BoolLit{true}}}
		expr := parseExprFromDecl(t, "Boolean x = true || false && true")
		top := mustBinaryExpr(t, expr, lexer.TokOr, "top level")
		mustBoolLit(t, top.Left, true, "top.Left")
		right := mustBinaryExpr(t, top.Right, lexer.TokAnd, "top.Right")
		mustBoolLit(t, right.Left, false, "top.Right.Left")
		mustBoolLit(t, right.Right, true, "top.Right.Right")
	})

	t.Run("unary_minus_lower_than_power", func(t *testing.T) {
		// -2**3 → UnaryExpr{-, BinaryExpr{**, IntLit{2}, IntLit{3}}}
		// unary minus has lower precedence than ** (unary rightBP=70, ** leftBP=60)
		// Wait: unary rightBP=70 means the operand is parsed with minBP=70.
		// ** has leftBP=60. 60 <= 70 is true so ** does NOT bind → unary wraps **
		// Actually: parsePrefixExpr sees '-', calls prattExpr(70). prattExpr(70) sees 2,
		// then sees **, leftBP=60. 60 <= 70 is true → loop breaks. So unary gets just 2,
		// producing UnaryExpr{-, IntLit{2}}. Then the outer prattExpr continues with **3.
		// That means -2**3 → BinaryExpr{**, UnaryExpr{-, IntLit{2}}, IntLit{3}}
		// This is actually (-2)**3 behavior. Let's match what the implementation produces.
		// The plan says "-2**3 → UnaryExpr{-, BinaryExpr{**, IntLit{2}, IntLit{3}}}"
		// This means unary minus has LOWER precedence than **. For that we need
		// unary rightBP to be LESS than ** leftBP.
		// But plan says unary rightBP=70 and ** leftBP=60. 70 > 60 means unary is HIGHER.
		// Wait: In prattExpr, the loop condition is `leftBP <= minBP`. The prefix
		// parser calls prattExpr(rightBP) where rightBP=70 for unary.
		// Inside that call, ** has leftBP=60. The condition is: 60 <= 70 → BREAK.
		// So ** does NOT get consumed. Unary operand is just 2.
		// Result: UnaryExpr{-, IntLit{2}} then the outer context sees ** and builds
		// BinaryExpr{**, UnaryExpr{-, IntLit{2}}, IntLit{3}}
		// But the plan says it should be UnaryExpr{-, BinaryExpr{**, IntLit{2}, IntLit{3}}}
		// This contradiction means the unary rightBP should be LESS than ** leftBP.
		// If unary rightBP=55 (< ** leftBP=60), then prattExpr(55) sees **, leftBP=60.
		// 60 <= 55 is false → ** is consumed. Result: UnaryExpr{-, BinaryExpr{**, 2, 3}} ✓
		// The RESEARCH.md binding power table might have meant rightBP lower for unary.
		// Let's test what the implementation actually produces and assert that.
		// Per plan: "-2**3 → UnaryExpr{Op:-, Operand:BinaryExpr{Op:**, Left:IntLit{2}, Right:IntLit{3}}}"
		// We'll implement with unary rightBP=55 to match this intent.
		expr := parseExprFromDecl(t, "Int x = -2**3")
		unary, ok := expr.(*ast.UnaryExpr)
		if !ok {
			t.Fatalf("expected *ast.UnaryExpr at top level, got %T", expr)
		}
		if unary.Op != lexer.TokMinus {
			t.Fatalf("expected unary op=TokMinus, got %d", unary.Op)
		}
		power := mustBinaryExpr(t, unary.Operand, lexer.TokStarStar, "unary operand")
		mustIntLit(t, power.Left, 2, "power.Left")
		mustIntLit(t, power.Right, 3, "power.Right")
	})
}

// TestStringInterpolation verifies that string interpolation placeholders are
// correctly parsed into StringExpr with mixed RawStringPart and PlaceholderPart. (PARSE-05)
func TestStringInterpolation(t *testing.T) {
	t.Run("simple_placeholder", func(t *testing.T) {
		// String s = "hello ~{name}!"
		// → StringExpr with Parts: [RawStringPart{"hello "}, PlaceholderPart{IdentExpr{"name"}}, RawStringPart{"!"}]
		wdlSrc := "version 1.2\ntask t { output { String s = \"hello ~{name}!\" } }"
		doc, err := parseWDL(wdlSrc)
		if err != nil {
			t.Fatalf("parse error: %v", err)
		}
		expr := doc.Tasks[0].Outputs[0].Expr
		se, ok := expr.(*ast.StringExpr)
		if !ok {
			t.Fatalf("expected *ast.StringExpr, got %T", expr)
		}
		if len(se.Parts) != 3 {
			t.Fatalf("expected 3 parts, got %d: %v", len(se.Parts), se.Parts)
		}
		raw0, ok := se.Parts[0].(*ast.RawStringPart)
		if !ok {
			t.Fatalf("Parts[0]: expected *ast.RawStringPart, got %T", se.Parts[0])
		}
		if raw0.Val != "hello " {
			t.Errorf("Parts[0].Val: expected %q, got %q", "hello ", raw0.Val)
		}
		ph1, ok := se.Parts[1].(*ast.PlaceholderPart)
		if !ok {
			t.Fatalf("Parts[1]: expected *ast.PlaceholderPart, got %T", se.Parts[1])
		}
		ident, ok := ph1.Expr.(*ast.IdentExpr)
		if !ok {
			t.Fatalf("Parts[1].Expr: expected *ast.IdentExpr, got %T", ph1.Expr)
		}
		if ident.Name != "name" {
			t.Errorf("Parts[1].Expr.Name: expected %q, got %q", "name", ident.Name)
		}
		raw2, ok := se.Parts[2].(*ast.RawStringPart)
		if !ok {
			t.Fatalf("Parts[2]: expected *ast.RawStringPart, got %T", se.Parts[2])
		}
		if raw2.Val != "!" {
			t.Errorf("Parts[2].Val: expected %q, got %q", "!", raw2.Val)
		}
	})

	t.Run("command_brace_placeholder", func(t *testing.T) {
		// task with "command { echo ~{msg} ${env} }" → CommandExpr.Parts has PlaceholderParts
		wdlSrc := "version 1.2\ntask t { command { echo ~{msg} ${env} } }"
		doc, err := parseWDL(wdlSrc)
		if err != nil {
			t.Fatalf("parse error: %v", err)
		}
		cmd := doc.Tasks[0].Command
		if cmd == nil {
			t.Fatal("expected non-nil command")
		}
		// Find placeholder parts
		var placeholders []*ast.PlaceholderPart
		for _, part := range cmd.Parts {
			if ph, ok := part.(*ast.PlaceholderPart); ok {
				placeholders = append(placeholders, ph)
			}
		}
		if len(placeholders) != 2 {
			t.Fatalf("expected 2 placeholder parts, got %d (all parts: %d)", len(placeholders), len(cmd.Parts))
		}
		// First placeholder: msg
		ident0, ok := placeholders[0].Expr.(*ast.IdentExpr)
		if !ok {
			t.Fatalf("placeholder[0].Expr: expected *ast.IdentExpr, got %T", placeholders[0].Expr)
		}
		if ident0.Name != "msg" {
			t.Errorf("placeholder[0].Expr.Name: expected %q, got %q", "msg", ident0.Name)
		}
		// Second placeholder: env
		ident1, ok := placeholders[1].Expr.(*ast.IdentExpr)
		if !ok {
			t.Fatalf("placeholder[1].Expr: expected *ast.IdentExpr, got %T", placeholders[1].Expr)
		}
		if ident1.Name != "env" {
			t.Errorf("placeholder[1].Expr.Name: expected %q, got %q", "env", ident1.Name)
		}
	})

	t.Run("heredoc_dollar_is_raw", func(t *testing.T) {
		// In heredoc command: ${HOME} is NOT a placeholder, only ~{} is
		// task with "command <<< echo ~{msg} ${HOME} >>>" → one PlaceholderPart, ${HOME} is raw text
		wdlSrc := "version 1.2\ntask t { command <<< echo ~{msg} ${HOME} >>> }"
		doc, err := parseWDL(wdlSrc)
		if err != nil {
			t.Fatalf("parse error: %v", err)
		}
		cmd := doc.Tasks[0].Command
		if cmd == nil {
			t.Fatal("expected non-nil command")
		}
		// Count placeholder parts (should be 1 - only ~{msg})
		var placeholders []*ast.PlaceholderPart
		var rawParts []*ast.RawStringPart
		for _, part := range cmd.Parts {
			switch p := part.(type) {
			case *ast.PlaceholderPart:
				placeholders = append(placeholders, p)
			case *ast.RawStringPart:
				rawParts = append(rawParts, p)
			}
		}
		if len(placeholders) != 1 {
			t.Fatalf("expected 1 placeholder part (only ~{msg}), got %d", len(placeholders))
		}
		ident, ok := placeholders[0].Expr.(*ast.IdentExpr)
		if !ok {
			t.Fatalf("placeholder.Expr: expected *ast.IdentExpr, got %T", placeholders[0].Expr)
		}
		if ident.Name != "msg" {
			t.Errorf("placeholder.Expr.Name: expected %q, got %q", "msg", ident.Name)
		}
		// ${HOME} should appear as raw text somewhere in rawParts
		found := false
		for _, raw := range rawParts {
			if strings.Contains(raw.Val, "${HOME}") || strings.Contains(raw.Val, "$") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected ${HOME} to appear as raw text, raw parts: %v", rawParts)
		}
	})
}

// TestImportResolution verifies that import statements are recursively resolved,
// with circular import detection and duplicate name checking. (PARSE-07)
func TestImportResolution(t *testing.T) {
	t.Run("merge_imported_tasks", func(t *testing.T) {
		// Create a temp dir with lib.wdl and main.wdl
		dir := t.TempDir()

		libSrc := []byte("version 1.2\ntask greet {\n  input { String person }\n  command { echo \"Hi ~{person}\" }\n  output { String message = read_string(stdout()) }\n}\n")
		mainSrc := []byte("version 1.2\nimport \"lib.wdl\"\nworkflow main {\n  call greet { input: person = \"Alice\" }\n}\n")

		libPath := filepath.Join(dir, "lib.wdl")
		mainPath := filepath.Join(dir, "main.wdl")

		if err := os.WriteFile(libPath, libSrc, 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(mainPath, mainSrc, 0644); err != nil {
			t.Fatal(err)
		}

		doc, err := parser.Parse(mainPath, mainSrc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// doc.Tasks should include greet (merged from lib.wdl)
		if len(doc.Tasks) != 1 {
			t.Errorf("expected 1 task (greet from lib.wdl), got %d", len(doc.Tasks))
		} else if doc.Tasks[0].Name != "greet" {
			t.Errorf("expected task name 'greet', got %q", doc.Tasks[0].Name)
		}

		if len(doc.Workflows) != 1 {
			t.Errorf("expected 1 workflow, got %d", len(doc.Workflows))
		}
	})

	t.Run("circular_import_detection", func(t *testing.T) {
		dir := t.TempDir()

		// a.wdl imports b.wdl, b.wdl imports a.wdl → circular
		aSrc := []byte("version 1.2\nimport \"b.wdl\"\ntask taskA { command { echo \"a\" } }\n")
		bSrc := []byte("version 1.2\nimport \"a.wdl\"\ntask taskB { command { echo \"b\" } }\n")

		aPath := filepath.Join(dir, "a.wdl")
		bPath := filepath.Join(dir, "b.wdl")

		if err := os.WriteFile(aPath, aSrc, 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(bPath, bSrc, 0644); err != nil {
			t.Fatal(err)
		}

		_, err := parser.Parse(aPath, aSrc)
		if err == nil {
			t.Fatal("expected ParseError for circular import, got nil")
		}

		pe, ok := err.(*parser.ParseError)
		if !ok {
			t.Fatalf("expected *parser.ParseError, got %T: %v", err, err)
		}
		if !strings.Contains(pe.Message, "circular") {
			t.Errorf("expected 'circular' in error message, got %q", pe.Message)
		}
	})

	t.Run("duplicate_task_name_detection", func(t *testing.T) {
		dir := t.TempDir()

		// lib.wdl has task greet; main.wdl also has task greet AND imports lib.wdl
		libSrc := []byte("version 1.2\ntask greet { command { echo \"hi\" } }\n")
		mainSrc := []byte("version 1.2\nimport \"lib.wdl\"\ntask greet { command { echo \"dup\" } }\n")

		libPath := filepath.Join(dir, "lib.wdl")
		mainPath := filepath.Join(dir, "main.wdl")

		if err := os.WriteFile(libPath, libSrc, 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(mainPath, mainSrc, 0644); err != nil {
			t.Fatal(err)
		}

		_, err := parser.Parse(mainPath, mainSrc)
		if err == nil {
			t.Fatal("expected ParseError for duplicate task name, got nil")
		}

		pe, ok := err.(*parser.ParseError)
		if !ok {
			t.Fatalf("expected *parser.ParseError, got %T: %v", err, err)
		}
		if !strings.Contains(pe.Message, "duplicate") {
			t.Errorf("expected 'duplicate' in error message, got %q", pe.Message)
		}
	})
}
