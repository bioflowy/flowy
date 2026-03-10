package eval

import "testing"

// TestEvalBinary tests binary operator evaluation (EXPR-01).
// Covers: +, -, *, /, %, **, ==, !=, <, <=, >, >=, &&, ||
func TestEvalBinary(t *testing.T) {
	t.Skip("not yet implemented")
}

// TestEvalUnary tests unary operator evaluation (EXPR-01).
// Covers: unary -, unary !
func TestEvalUnary(t *testing.T) {
	t.Skip("not yet implemented")
}

// TestEvalIfThenElse tests conditional expression evaluation (EXPR-02).
// Covers: if Cond then Then else Else
func TestEvalIfThenElse(t *testing.T) {
	t.Skip("not yet implemented")
}

// TestEvalGetExpr tests member access expression evaluation (EXPR-03).
// Covers: struct.field, pair.left, pair.right
func TestEvalGetExpr(t *testing.T) {
	t.Skip("not yet implemented")
}

// TestEvalIndexExpr tests index access expression evaluation (EXPR-04).
// Covers: array[i] and map[k]
func TestEvalIndexExpr(t *testing.T) {
	t.Skip("not yet implemented")
}

// TestEvalApply tests function call expression evaluation (EXPR-05).
// Calls a registered stub function to verify dispatch mechanism.
func TestEvalApply(t *testing.T) {
	t.Skip("not yet implemented")
}

// TestEvalCommand tests command template expansion (EXPR-06).
// Covers: placeholder interpolation within command blocks.
func TestEvalCommand(t *testing.T) {
	t.Skip("not yet implemented")
}

// TestCommandDedent tests the sentinel-before-indent dedent algorithm (EXPR-07).
// Covers: leading whitespace removal from command block content.
func TestCommandDedent(t *testing.T) {
	t.Skip("not yet implemented")
}
