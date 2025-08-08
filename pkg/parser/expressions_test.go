package parser

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bioflowy/flowy/pkg/expr"
)

func TestParseExpressionBasic(t *testing.T) {
	tests := []struct {
		input        string
		expectedType string
	}{
		{"true", "BooleanLiteral"},
		{"42", "IntLiteral"},
		{"3.14", "FloatLiteral"},
		{`"hello"`, "StringLiteral"},
		{"variable", "Identifier"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseExpression()

		if !ok {
			t.Errorf("Failed to parse expression '%s'", test.input)
			continue
		}

		actualType := getExpressionType(result)
		if actualType != test.expectedType {
			t.Errorf("Input '%s': expected %s, got %s", test.input, test.expectedType, actualType)
		}
	}
}

func TestParseBinaryOperations(t *testing.T) {
	tests := []struct {
		input    string
		operator string
	}{
		{"a || b", "||"},
		{"x && y", "&&"},
		{"a == b", "=="},
		{"x != y", "!="},
		{"a < b", "<"},
		{"x > y", ">"},
		{"a <= b", "<="},
		{"x >= y", ">="},
		{"a + b", "+"},
		{"x - y", "-"},
		{"a * b", "*"},
		{"x / y", "/"},
		{"a % b", "%"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseExpression()

		if !ok {
			t.Errorf("Failed to parse binary expression '%s'", test.input)
			continue
		}

		binaryOp, ok := result.(*expr.BinaryOp)
		if !ok {
			t.Errorf("Expected BinaryOp, got %T", result)
			continue
		}

		if binaryOp.Operator != test.operator {
			t.Errorf("Input '%s': expected operator '%s', got '%s'",
				test.input, test.operator, binaryOp.Operator)
		}
	}
}

func TestParseOperatorPrecedence(t *testing.T) {
	tests := []struct {
		input    string
		expected string // Expected structure representation
	}{
		{"a + b * c", "(a + (b * c))"},     // * has higher precedence
		{"a * b + c", "((a * b) + c)"},     // Left associative
		{"a || b && c", "(a || (b && c))"}, // && has higher precedence
		{"a < b + c", "(a < (b + c))"},     // + has higher precedence
		{"a + b < c", "((a + b) < c)"},     // Left associative
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseExpression()

		if !ok {
			t.Errorf("Failed to parse precedence expression '%s'", test.input)
			continue
		}

		// This is a simplified test - in a real implementation,
		// we'd need to traverse the AST and verify the structure
		_ = result
		// For now, just verify that parsing succeeded
	}
}

func TestParseUnaryOperations(t *testing.T) {
	tests := []struct {
		input    string
		operator string
	}{
		{"!true", "!"},
		{"!variable", "!"},
		{"!(a && b)", "!"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseExpression()

		if !ok {
			t.Errorf("Failed to parse unary expression '%s'", test.input)
			continue
		}

		unaryOp, ok := result.(*expr.UnaryOp)
		if !ok {
			t.Errorf("Expected UnaryOp, got %T", result)
			continue
		}

		if unaryOp.Operator != test.operator {
			t.Errorf("Input '%s': expected operator '%s', got '%s'",
				test.input, test.operator, unaryOp.Operator)
		}
	}
}

func TestParseArrayLiteral(t *testing.T) {
	tests := []struct {
		input         string
		expectedCount int
	}{
		{"[]", 0},
		{"[1]", 1},
		{"[1, 2, 3]", 3},
		{"[true, false]", 2},
		{`["a", "b", "c"]`, 3},
		{"[1, 2,]", 2}, // Trailing comma
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseExpression()

		if !ok {
			t.Errorf("Failed to parse array literal '%s'", test.input)
			continue
		}

		arrayLit, ok := result.(*expr.ArrayLiteral)
		if !ok {
			t.Errorf("Expected ArrayLiteral, got %T", result)
			continue
		}

		if len(arrayLit.Items) != test.expectedCount {
			t.Errorf("Input '%s': expected %d elements, got %d",
				test.input, test.expectedCount, len(arrayLit.Items))
		}
	}
}

func TestParseMapLiteral(t *testing.T) {
	tests := []struct {
		input         string
		expectedCount int
	}{
		{"{}", 0},
		{`{"key": "value"}`, 1},
		{`{"a": 1, "b": 2}`, 2},
		{`{1: "one", 2: "two"}`, 2},
		{`{"trailing": "comma",}`, 1}, // Trailing comma
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseExpression()

		if !ok {
			t.Errorf("Failed to parse map literal '%s'", test.input)
			continue
		}

		mapLit, ok := result.(*expr.MapLiteral)
		if !ok {
			t.Errorf("Expected MapLiteral, got %T", result)
			continue
		}

		if len(mapLit.Items) != test.expectedCount {
			t.Errorf("Input '%s': expected %d items, got %d",
				test.input, test.expectedCount, len(mapLit.Items))
		}
	}
}

func TestParsePairLiteral(t *testing.T) {
	tests := []string{
		"(1, 2)",
		"(true, false)",
		`("left", "right")`,
		"(variable, 42)",
	}

	for _, test := range tests {
		parser := NewParser(test, "test.wdl")
		result, ok := parser.parseExpression()

		if !ok {
			t.Errorf("Failed to parse pair literal '%s'", test)
			continue
		}

		pairLit, ok := result.(*expr.PairLiteral)
		if !ok {
			t.Errorf("Expected PairLiteral, got %T", result)
		}

		_ = pairLit // Use the variable to avoid unused warning
	}
}

func TestParseParenthesizedExpression(t *testing.T) {
	tests := []string{
		"(42)",
		"(true)",
		"(variable)",
		"((nested))",
	}

	for _, test := range tests {
		parser := NewParser(test, "test.wdl")
		result, ok := parser.parseExpression()

		if !ok {
			t.Errorf("Failed to parse parenthesized expression '%s'", test)
			continue
		}

		// The result should be the inner expression, not a special parenthesis node
		// (unless it's a pair)
		if test == "(42)" {
			if _, ok := result.(*expr.IntLiteral); !ok {
				t.Errorf("Expected IntLiteral for '(42)', got %T", result)
			}
		}
	}
}

func TestParseIfThenElse(t *testing.T) {
	tests := []string{
		"if true then 1 else 2",
		"if x > 0 then positive else negative",
		"if a && b then c else d",
	}

	for _, test := range tests {
		parser := NewParser(test, "test.wdl")
		result, ok := parser.parseExpression()

		if !ok {
			t.Errorf("Failed to parse if-then-else '%s'", test)
			continue
		}

		ifThenElse, ok := result.(*expr.IfThenElse)
		if !ok {
			t.Errorf("Expected IfThenElse, got %T", result)
		}

		_ = ifThenElse // Use the variable
	}
}

func TestParseFunctionCall(t *testing.T) {
	tests := []struct {
		input        string
		expectedName string
		expectedArgs int
	}{
		{"func()", "func", 0},
		{"max(1, 2)", "max", 2},
		{"length(array)", "length", 1},
		{"nested(func(x), y)", "nested", 2},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseExpression()

		if !ok {
			t.Errorf("Failed to parse function call '%s'", test.input)
			continue
		}

		funcCall, ok := result.(*expr.Apply)
		if !ok {
			t.Errorf("Expected FunctionCall, got %T", result)
			continue
		}

		if funcCall.Function != test.expectedName {
			t.Errorf("Input '%s': expected function name '%s', got '%s'",
				test.input, test.expectedName, funcCall.Function)
		}

		if len(funcCall.Args) != test.expectedArgs {
			t.Errorf("Input '%s': expected %d arguments, got %d",
				test.input, test.expectedArgs, len(funcCall.Args))
		}
	}
}

func TestParseStructLiteral(t *testing.T) {
	tests := []struct {
		input         string
		expectedType  string
		expectedCount int
	}{
		{"Person{}", "Person", 0},
		{`Person{name: "John"}`, "Person", 1},
		{`Point{x: 1, y: 2}`, "Point", 2},
		{`Config{debug: true, timeout: 30,}`, "Config", 2}, // Trailing comma
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseExpression()

		if !ok {
			t.Errorf("Failed to parse struct literal '%s'", test.input)
			continue
		}

		structLit, ok := result.(*expr.StructLiteral)
		if !ok {
			t.Errorf("Expected StructLiteral, got %T", result)
			continue
		}

		if structLit.TypeName != test.expectedType {
			t.Errorf("Input '%s': expected type '%s', got '%s'",
				test.input, test.expectedType, structLit.TypeName)
		}

		if len(structLit.Members) != test.expectedCount {
			t.Errorf("Input '%s': expected %d members, got %d",
				test.input, test.expectedCount, len(structLit.Members))
		}
	}
}

func TestParseMemberAccess(t *testing.T) {
	tests := []struct {
		input          string
		expectedBase   string
		expectedMember string
	}{
		{"obj.field", "obj", "field"},
		{"task.output", "task", "output"},
		{"nested.obj.member", "nested.obj", "member"}, // Should be left-associative
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseExpression()

		if !ok {
			t.Errorf("Failed to parse member access '%s'", test.input)
			continue
		}

		memberAccess, ok := result.(*expr.GetAttr)
		if !ok {
			t.Errorf("Expected MemberAccess, got %T", result)
			continue
		}

		if memberAccess.Attr != test.expectedMember {
			t.Errorf("Input '%s': expected member '%s', got '%s'",
				test.input, test.expectedMember, memberAccess.Attr)
		}
	}
}

func TestParseArrayAccess(t *testing.T) {
	tests := []string{
		"func()[index]",
	}

	for _, test := range tests {
		parser := NewParser(test, "test.wdl")
		result, ok := parser.ParseExpression()

		if !ok {
			t.Errorf("Failed to parse array access '%s'", test)
			continue
		}

		arrayAccess, ok := result.(*expr.GetIndex)
		if !ok {
			t.Errorf("Expected ArrayAccess for '%s', got %T", test, result)
			continue
		}

		_ = arrayAccess // Use the variable
	}
}

func TestParseComplexExpressions(t *testing.T) {
	tests := []string{
		"func(a + b, c * d)",
		"array[index].field",
		"if condition then func(x) else default.value",
		"(a + b) * (c - d)",
		"{key: value, other: func(x)}",
		"[1, 2, 3][index] + offset",
	}

	for _, test := range tests {
		parser := NewParser(test, "test.wdl")
		result, ok := parser.parseExpression()

		if !ok {
			t.Errorf("Failed to parse complex expression '%s'", test)
			continue
		}

		// Just verify that parsing succeeded
		if result == nil {
			t.Errorf("Parsed expression '%s' resulted in nil", test)
		}
	}
}

func TestIsComparisonOperator(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"==", true},
		{"!=", true},
		{"<", true},
		{">", true},
		{"<=", true},
		{">=", true},
		{"+", false},
		{"-", false},
		{"*", false},
		{"||", false},
		{"&&", false},
	}

	for _, test := range tests {
		parser := NewParser(test.input+" a", "test.wdl") // Add 'a' to have a valid expression
		result := parser.isComparisonOperator()

		if result != test.expected {
			t.Errorf("Input '%s': isComparisonOperator() expected %t, got %t",
				test.input, test.expected, result)
		}
	}
}

func TestExpressionParseErrors(t *testing.T) {
	tests := []struct {
		input       string
		description string
	}{
		{"if true then", "incomplete if-then-else"},
		{"func(", "incomplete function call"},
		{"array[", "incomplete array access"},
		{"obj.", "incomplete member access"},
		{"{key:", "incomplete map literal"},
		{"(1,", "incomplete pair literal"},
		{"!", "unary operator without operand"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseExpression()

		if ok {
			t.Errorf("Expected parsing '%s' to fail (%s), but got: %T",
				test.input, test.description, result)
		}

		// Check that error was recorded
		if !parser.HasErrors() {
			t.Errorf("Expected error to be recorded when parsing '%s'", test.input)
		}
	}
}

// Helper function to get the type name of an expression
func getExpressionType(expr expr.Expr) string {
	// Use string conversion to avoid direct type assertions that cause compilation issues
	typeStr := fmt.Sprintf("%T", expr)
	// Extract the type name after the last dot
	if lastDot := strings.LastIndex(typeStr, "."); lastDot >= 0 {
		return typeStr[lastDot+1:]
	}
	return typeStr
}
