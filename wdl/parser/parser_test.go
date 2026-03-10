package parser_test

import "testing"

// TestParseDocument verifies that a complete WDL document can be parsed into
// an ast.Document with correct task, workflow, and struct definitions. (PARSE-01)
func TestParseDocument(t *testing.T) {
	t.Skip("not implemented: PARSE-01")
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

// TestParseError verifies that parse errors include precise source location
// information (file, line, col) and produce rustc-style formatted messages. (PARSE-08)
func TestParseError(t *testing.T) {
	t.Skip("not implemented: PARSE-08")
}

// TestStructDef verifies that WDL struct definitions are correctly parsed,
// including all member declarations with their types. (PARSE-09)
func TestStructDef(t *testing.T) {
	t.Skip("not implemented: PARSE-09")
}
