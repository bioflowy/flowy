package parser_test

import (
	"strings"
	"testing"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/parser"
	"github.com/bioflowy/flowy/wdl/types"
)

// parseWDL is a helper that parses a WDL source string from "test.wdl".
func parseWDL(src string) (*ast.Document, error) {
	return parser.Parse("test.wdl", []byte(src))
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

// TestParseDocument verifies that a minimal WDL document is parsed correctly. (PARSE-01 partial)
// Full PARSE-01 coverage (imports, multi-element documents) is completed in Plan 05.
func TestParseDocument(t *testing.T) {
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

// TestStringInterpolation verifies that string interpolation placeholders are
// correctly parsed into StringExpr with mixed RawStringPart and PlaceholderPart. (PARSE-05)
func TestStringInterpolation(t *testing.T) {
	t.Skip("not implemented: PARSE-05")
}

// TestOperatorPrecedence verifies that the Pratt expression parser respects
// WDL operator precedence and associativity rules, including ** (exponentiation). (PARSE-06)
func TestOperatorPrecedence(t *testing.T) {
	t.Skip("not implemented: PARSE-06")
}

// TestImportResolution verifies that import statements are recursively resolved,
// with circular import detection and duplicate name checking. (PARSE-07)
func TestImportResolution(t *testing.T) {
	t.Skip("not implemented: PARSE-07")
}
