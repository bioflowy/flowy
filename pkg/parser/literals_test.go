package parser

import (
	"testing"

	"github.com/bioflowy/flowy/pkg/expr"
)

func TestParseBooleanLiteral(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"true", true},
		{"false", false},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseBooleanLiteral()

		if !ok {
			t.Errorf("Failed to parse boolean literal '%s'", test.input)
			continue
		}

		boolLit, ok := result.(*expr.BooleanLiteral)
		if !ok {
			t.Errorf("Expected BooleanLiteral, got %T", result)
			continue
		}

		if literal, isLiteral := boolLit.Literal(); isLiteral {
			if boolValue, ok := literal.(*expr.BooleanValue); ok {
				if boolValue.Value().(bool) != test.expected {
					t.Errorf("Expected %t, got %t", test.expected, boolValue.Value().(bool))
				}
			} else {
				t.Errorf("Expected BooleanValue, got %T", literal)
			}
		} else {
			t.Error("BooleanLiteral should be a literal")
		}
	}
}

func TestParseNullLiteral(t *testing.T) {
	parser := NewParser("None", "test.wdl")
	result, ok := parser.parseNullLiteral()

	if !ok {
		t.Error("Failed to parse null literal")
		return
	}

	nullLit, ok := result.(*expr.NullLiteral)
	if !ok {
		t.Errorf("Expected NullLiteral, got %T", result)
		return
	}

	if literal, isLiteral := nullLit.Literal(); isLiteral {
		if _, ok := literal.(*expr.NullValue); !ok {
			t.Errorf("Expected NullValue, got %T", literal)
		}
	} else {
		t.Error("NullLiteral should be a literal")
	}
}

func TestParseIntLiteral(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"0", 0},
		{"42", 42},
		{"123", 123},
		{"999", 999},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseIntLiteral()

		if !ok {
			t.Errorf("Failed to parse int literal '%s'", test.input)
			continue
		}

		intLit, ok := result.(*expr.IntLiteral)
		if !ok {
			t.Errorf("Expected IntLiteral, got %T", result)
			continue
		}

		if literal, isLiteral := intLit.Literal(); isLiteral {
			if intValue, ok := literal.(*expr.IntValue); ok {
				if intValue.Value().(int64) != test.expected {
					t.Errorf("Expected %d, got %d", test.expected, intValue.Value().(int64))
				}
			} else {
				t.Errorf("Expected IntValue, got %T", literal)
			}
		} else {
			t.Error("IntLiteral should be a literal")
		}
	}
}

func TestParseFloatLiteral(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"0.0", 0.0},
		{"3.14", 3.14},
		{"2.5", 2.5},
		{"1.0e10", 1.0e10},
		{"2E-3", 2E-3},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseFloatLiteral()

		if !ok {
			t.Errorf("Failed to parse float literal '%s'", test.input)
			continue
		}

		floatLit, ok := result.(*expr.FloatLiteral)
		if !ok {
			t.Errorf("Expected FloatLiteral, got %T", result)
			continue
		}

		if literal, isLiteral := floatLit.Literal(); isLiteral {
			if floatValue, ok := literal.(*expr.FloatValue); ok {
				if floatValue.Value().(float64) != test.expected {
					t.Errorf("Expected %f, got %f", test.expected, floatValue.Value().(float64))
				}
			} else {
				t.Errorf("Expected FloatValue, got %T", literal)
			}
		} else {
			t.Error("FloatLiteral should be a literal")
		}
	}
}

func TestParseStringLiteral(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`"hello"`, "hello"},
		{`'world'`, "world"},
		{`"hello world"`, "hello world"},
		{`""`, ""},
		{`''`, ""},
		{`"with spaces"`, "with spaces"},
		{`'single quoted'`, "single quoted"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseStringLiteral()

		if !ok {
			t.Errorf("Failed to parse string literal %s", test.input)
			continue
		}

		stringLit, ok := result.(*expr.StringLiteral)
		if !ok {
			t.Errorf("Expected StringLiteral, got %T", result)
			continue
		}

		if literal, isLiteral := stringLit.Literal(); isLiteral {
			if stringValue, ok := literal.(*expr.StringValue); ok {
				if stringValue.Value().(string) != test.expected {
					t.Errorf("Expected '%s', got '%s'", test.expected, stringValue.Value().(string))
				}
			} else {
				t.Errorf("Expected StringValue, got %T", literal)
			}
		} else {
			t.Error("StringLiteral should be a literal")
		}
	}
}

func TestParseSignedNumber(t *testing.T) {
	tests := []struct {
		input       string
		expectedType string
		expectedVal  interface{}
	}{
		{"-42", "IntLiteral", int64(-42)},
		{"+42", "IntLiteral", int64(42)},
		{"-3.14", "FloatLiteral", float64(-3.14)},
		{"+2.5", "FloatLiteral", float64(2.5)},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseSignedNumber()

		if !ok {
			t.Errorf("Failed to parse signed number '%s'", test.input)
			continue
		}

		switch test.expectedType {
		case "IntLiteral":
			intLit, ok := result.(*expr.IntLiteral)
			if !ok {
				t.Errorf("Expected IntLiteral, got %T", result)
				continue
			}
			if literal, isLiteral := intLit.Literal(); isLiteral {
				if intValue, ok := literal.(*expr.IntValue); ok {
					if intValue.Value().(int64) != test.expectedVal.(int64) {
						t.Errorf("Expected %d, got %d", test.expectedVal.(int64), intValue.Value().(int64))
					}
				}
			}

		case "FloatLiteral":
			floatLit, ok := result.(*expr.FloatLiteral)
			if !ok {
				t.Errorf("Expected FloatLiteral, got %T", result)
				continue
			}
			if literal, isLiteral := floatLit.Literal(); isLiteral {
				if floatValue, ok := literal.(*expr.FloatValue); ok {
					if floatValue.Value().(float64) != test.expectedVal.(float64) {
						t.Errorf("Expected %f, got %f", test.expectedVal.(float64), floatValue.Value().(float64))
					}
				}
			}
		}
	}
}

func TestParseLiteral(t *testing.T) {
	tests := []struct {
		input       string
		expectedType string
	}{
		{"true", "BooleanLiteral"},
		{"false", "BooleanLiteral"},
		{"None", "NullLiteral"},
		{"42", "IntLiteral"},
		{"3.14", "FloatLiteral"},
		{`"hello"`, "StringLiteral"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseLiteral()

		if !ok {
			t.Errorf("Failed to parse literal '%s'", test.input)
			continue
		}

		actualType := ""
		switch result.(type) {
		case *expr.BooleanLiteral:
			actualType = "BooleanLiteral"
		case *expr.NullLiteral:
			actualType = "NullLiteral"
		case *expr.IntLiteral:
			actualType = "IntLiteral"
		case *expr.FloatLiteral:
			actualType = "FloatLiteral"
		case *expr.StringLiteral:
			actualType = "StringLiteral"
		default:
			actualType = "Unknown"
		}

		if actualType != test.expectedType {
			t.Errorf("Input '%s': expected %s, got %s", test.input, test.expectedType, actualType)
		}
	}
}

func TestParseAnyLiteral(t *testing.T) {
	tests := []struct {
		input       string
		expectedType string
	}{
		{"+42", "IntLiteral"},
		{"-42", "IntLiteral"},
		{"+3.14", "FloatLiteral"},
		{"-3.14", "FloatLiteral"},
		{"true", "BooleanLiteral"},
		{"false", "BooleanLiteral"},
		{"None", "NullLiteral"},
		{`"hello"`, "StringLiteral"},
		{"42", "IntLiteral"},
		{"3.14", "FloatLiteral"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseAnyLiteral()

		if !ok {
			t.Errorf("Failed to parse any literal '%s'", test.input)
			continue
		}

		actualType := ""
		switch result.(type) {
		case *expr.BooleanLiteral:
			actualType = "BooleanLiteral"
		case *expr.NullLiteral:
			actualType = "NullLiteral"
		case *expr.IntLiteral:
			actualType = "IntLiteral"
		case *expr.FloatLiteral:
			actualType = "FloatLiteral"
		case *expr.StringLiteral:
			actualType = "StringLiteral"
		default:
			actualType = "Unknown"
		}

		if actualType != test.expectedType {
			t.Errorf("Input '%s': expected %s, got %s", test.input, test.expectedType, actualType)
		}
	}
}

func TestParseNumberLiteral(t *testing.T) {
	tests := []struct {
		input       string
		expectedType string
	}{
		{"42", "IntLiteral"},
		{"3.14", "FloatLiteral"},
		{"0", "IntLiteral"},
		{"0.0", "FloatLiteral"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseNumberLiteral()

		if !ok {
			t.Errorf("Failed to parse number literal '%s'", test.input)
			continue
		}

		actualType := ""
		switch result.(type) {
		case *expr.IntLiteral:
			actualType = "IntLiteral"
		case *expr.FloatLiteral:
			actualType = "FloatLiteral"
		default:
			actualType = "Unknown"
		}

		if actualType != test.expectedType {
			t.Errorf("Input '%s': expected %s, got %s", test.input, test.expectedType, actualType)
		}
	}
}

func TestParsePrimitiveLiteral(t *testing.T) {
	tests := []struct {
		input       string
		expectedType string
	}{
		{"true", "BooleanLiteral"},
		{"false", "BooleanLiteral"},
		{"None", "NullLiteral"},
		{"42", "IntLiteral"},
		{"3.14", "FloatLiteral"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parsePrimitiveLiteral()

		if !ok {
			t.Errorf("Failed to parse primitive literal '%s'", test.input)
			continue
		}

		actualType := ""
		switch result.(type) {
		case *expr.BooleanLiteral:
			actualType = "BooleanLiteral"
		case *expr.NullLiteral:
			actualType = "NullLiteral"
		case *expr.IntLiteral:
			actualType = "IntLiteral"
		case *expr.FloatLiteral:
			actualType = "FloatLiteral"
		default:
			actualType = "Unknown"
		}

		if actualType != test.expectedType {
			t.Errorf("Input '%s': expected %s, got %s", test.input, test.expectedType, actualType)
		}
	}
}

func TestLiteralCheckers(t *testing.T) {
	tests := []struct {
		input       string
		isLiteral   bool
		isNumeric   bool
		isBoolean   bool
		isNull      bool
		isString    bool
	}{
		{"true", true, false, true, false, false},
		{"false", true, false, true, false, false},
		{"None", true, false, false, true, false},
		{"42", true, true, false, false, false},
		{"3.14", true, true, false, false, false},
		{`"hello"`, true, false, false, false, true},
		{"workflow", false, false, false, false, false}, // identifier
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")

		if parser.isLiteralToken() != test.isLiteral {
			t.Errorf("Input '%s': isLiteralToken() expected %t, got %t", 
				test.input, test.isLiteral, parser.isLiteralToken())
		}
		if parser.isNumericLiteral() != test.isNumeric {
			t.Errorf("Input '%s': isNumericLiteral() expected %t, got %t", 
				test.input, test.isNumeric, parser.isNumericLiteral())
		}
		if parser.isBooleanLiteral() != test.isBoolean {
			t.Errorf("Input '%s': isBooleanLiteral() expected %t, got %t", 
				test.input, test.isBoolean, parser.isBooleanLiteral())
		}
		if parser.isNullLiteral() != test.isNull {
			t.Errorf("Input '%s': isNullLiteral() expected %t, got %t", 
				test.input, test.isNull, parser.isNullLiteral())
		}
		if parser.isStringLiteral() != test.isString {
			t.Errorf("Input '%s': isStringLiteral() expected %t, got %t", 
				test.input, test.isString, parser.isStringLiteral())
		}
	}
}

func TestParseConstantExpression(t *testing.T) {
	tests := []struct {
		input       string
		expectedType string
	}{
		{"true", "BooleanLiteral"},
		{"42", "IntLiteral"},
		{"-42", "IntLiteral"},
		{"3.14", "FloatLiteral"},
		{`"constant"`, "StringLiteral"},
		{"None", "NullLiteral"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseConstantExpression()

		if !ok {
			t.Errorf("Failed to parse constant expression '%s'", test.input)
			continue
		}

		actualType := ""
		switch result.(type) {
		case *expr.BooleanLiteral:
			actualType = "BooleanLiteral"
		case *expr.NullLiteral:
			actualType = "NullLiteral"
		case *expr.IntLiteral:
			actualType = "IntLiteral"
		case *expr.FloatLiteral:
			actualType = "FloatLiteral"
		case *expr.StringLiteral:
			actualType = "StringLiteral"
		default:
			actualType = "Unknown"
		}

		if actualType != test.expectedType {
			t.Errorf("Input '%s': expected %s, got %s", test.input, test.expectedType, actualType)
		}
	}
}

func TestLiteralParseErrors(t *testing.T) {
	tests := []struct {
		input       string
		parseFunc   func(*Parser) (expr.Expr, bool)
		description string
	}{
		{"workflow", (*Parser).parseBooleanLiteral, "non-boolean"},
		{"workflow", (*Parser).parseNullLiteral, "non-null"},
		{"true", (*Parser).parseIntLiteral, "non-integer"},
		{"42", (*Parser).parseFloatLiteral, "non-float"},
		{"42", (*Parser).parseStringLiteral, "non-string"},
		{"workflow", (*Parser).parseSignedNumber, "non-number"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := test.parseFunc(parser)

		if ok {
			t.Errorf("Expected parsing '%s' with %s to fail, but it succeeded with result: %T", 
				test.input, test.description, result)
		}

		// Check that error was recorded
		if !parser.HasErrors() {
			t.Errorf("Expected error to be recorded when parsing '%s' with %s", 
				test.input, test.description)
		}
	}
}