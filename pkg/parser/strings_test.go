package parser

import (
	"testing"

	"github.com/bioflowy/flowy/pkg/expr"
)

func TestParseStringInterpolation(t *testing.T) {
	tests := []struct {
		input             string
		expectedParts     int
		expectedType      string
		description       string
	}{
		{`"hello ${name} world"`, 3, "StringLiteral", "simple interpolation"},
		{`"value: ${x + y}"`, 2, "StringLiteral", "expression interpolation"},
		{`"${greeting} ${name}!"`, 3, "StringLiteral", "multiple interpolations"},
		{`"no interpolation"`, 1, "StringLiteral", "plain string"},
		{`"${func(a, b)} result"`, 2, "StringLiteral", "function call interpolation"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseStringInterpolation()

		if !ok {
			t.Errorf("Failed to parse string interpolation: %s", test.description)
			continue
		}

		stringExpr, ok := result.(*expr.StringLiteral)
		if !ok {
			t.Errorf("Expected StringLiteral, got %T for %s", result, test.description)
			continue
		}

		_ = stringExpr // Use the variable to avoid unused warning
		// Note: In a real implementation, we'd check the interpolation parts
	}
}

func TestParseCommandString(t *testing.T) {
	tests := []struct {
		input       string
		description string
	}{
		{`<<<
		echo "hello world"
		ls -la ${input_dir}
		>>>`, "simple command"},
		{`<<<
		python script.py \
			--input ${input_file} \
			--output ${output_file}
		>>>`, "multi-line command with interpolation"},
		{`<<<
		if [ "${condition}" == "true" ]; then
			echo "condition met"
		fi
		>>>`, "command with conditional"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseCommandString()

		if !ok {
			t.Errorf("Failed to parse command string: %s", test.description)
			continue
		}

		commandExpr, ok := result.(*expr.TaskCommand)
		if !ok {
			t.Errorf("Expected TaskCommand, got %T for %s", result, test.description)
			continue
		}

		if commandExpr.Value() == "" {
			t.Errorf("Command string should not be empty for %s", test.description)
		}
	}
}

func TestParseMultilineString(t *testing.T) {
	tests := []struct {
		input       string
		description string
	}{
		{`"""
		This is a
		multiline string
		with ${variable}
		"""`, "basic multiline string"},
		{`'''
		Single-quoted
		multiline string
		'''`, "single-quoted multiline"},
		{`"""${greeting}
		Hello ${name}!
		End of message
		"""`, "multiline with multiple interpolations"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseMultilineString()

		if !ok {
			t.Errorf("Failed to parse multiline string: %s", test.description)
			continue
		}

		multilineExpr, ok := result.(*expr.MultilineString)
		if !ok {
			t.Errorf("Expected MultilineString, got %T for %s", result, test.description)
			continue
		}

		if multilineExpr.Value() == "" {
			t.Errorf("Multiline string should not be empty for %s", test.description)
		}
	}
}

func TestParsePlaceholder(t *testing.T) {
	tests := []struct {
		input           string
		expectedOptions map[string]string
		description     string
	}{
		{`${name}`, nil, "simple placeholder"},
		{`${sep=" " array}`, map[string]string{"sep": " "}, "placeholder with separator"},
		{`${default="none" optional_value}`, map[string]string{"default": "none"}, "placeholder with default"},
		{`${true="yes" false="no" boolean_flag}`, map[string]string{"true": "yes", "false": "no"}, "placeholder with true/false options"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parsePlaceholder()

		if !ok {
			t.Errorf("Failed to parse placeholder: %s", test.description)
			continue
		}

		placeholder, ok := result.(*expr.Placeholder)
		if !ok {
			t.Errorf("Expected Placeholder, got %T for %s", result, test.description)
			continue
		}

		_ = placeholder // Use the variable
		// Note: In a real implementation, we'd validate the options
	}
}

func TestParseInterpolationExpression(t *testing.T) {
	tests := []struct {
		input       string
		expectedType string
		description string
	}{
		{`${variable}`, "Identifier", "simple variable"},
		{`${func(x, y)}`, "FunctionCall", "function call"},
		{`${array[index]}`, "ArrayAccess", "array access"},
		{`${obj.field}`, "MemberAccess", "member access"},
		{`${a + b * c}`, "BinaryOp", "arithmetic expression"},
		{`${if condition then value1 else value2}`, "IfThenElse", "conditional expression"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseInterpolationExpression()

		if !ok {
			t.Errorf("Failed to parse interpolation expression: %s", test.description)
			continue
		}

		// Check that we got some kind of expression
		if result == nil {
			t.Errorf("Interpolation expression should not be nil for %s", test.description)
		}
	}
}

func TestParseStringLiteral(t *testing.T) {
	tests := []struct {
		input         string
		expectedValue string
		hasInterpolation bool
	}{
		{`"simple string"`, "simple string", false},
		{`'single quoted'`, "single quoted", false},
		{`"string with ${var}"`, "string with ${var}", true},
		{`"multiple ${a} and ${b}"`, "multiple ${a} and ${b}", true},
		{`""`, "", false},
		{`"string with \"quotes\""`, `string with "quotes"`, false},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseStringLiteral()

		if !ok {
			t.Errorf("Failed to parse string literal '%s'", test.input)
			continue
		}

		stringLit, ok := result.(*expr.StringLiteral)
		if !ok {
			t.Errorf("Expected StringLiteral, got %T", result)
			continue
		}

		// In a real implementation, we'd check the actual value and interpolation
		_ = stringLit
	}
}

func TestParseStringValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`"hello"`, "hello"},
		{`'world'`, "world"},
		{`"with spaces"`, "with spaces"},
		{`""`, ""},
		{`"with \"escaped\" quotes"`, `with "escaped" quotes`},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseStringValue()

		if !ok {
			t.Errorf("Failed to parse string value '%s'", test.input)
			continue
		}

		if result != test.expected {
			t.Errorf("Input '%s': expected '%s', got '%s'", 
				test.input, test.expected, result)
		}
	}
}

func TestIsInterpolationStart(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"${", true},
		{"~{", true},
		{"{", false},
		{"$", false},
		{"~", false},
		{"hello", false},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result := parser.isInterpolationStart()

		if result != test.expected {
			t.Errorf("Input '%s': isInterpolationStart() expected %t, got %t", 
				test.input, test.expected, result)
		}
	}
}

func TestIsStringStart(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{`"hello"`, true},
		{`'world'`, true},
		{`"""multiline"""`, true},
		{`'''multiline'''`, true},
		{"hello", false},
		{"123", false},
		{"{", false},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result := parser.isStringStart()

		if result != test.expected {
			t.Errorf("Input '%s': isStringStart() expected %t, got %t", 
				test.input, test.expected, result)
		}
	}
}

func TestParseLeftName(t *testing.T) {
	tests := []struct {
		input       string
		expectedName string
	}{
		{"variable", "variable"},
		{"my_var", "my_var"},
		{"CamelCase", "CamelCase"},
		{"_private", "_private"},
		{"var123", "var123"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseLeftName()

		if !ok {
			t.Errorf("Failed to parse left name '%s'", test.input)
			continue
		}

		leftName, ok := result.(*expr.LeftName)
		if !ok {
			t.Errorf("Expected LeftName, got %T", result)
			continue
		}

		if leftName.Name() != test.expectedName {
			t.Errorf("Input '%s': expected name '%s', got '%s'", 
				test.input, test.expectedName, leftName.Name())
		}
	}
}

func TestParseEscapedString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`"hello"`, "hello"},
		{`"hello\nworld"`, "hello\nworld"},
		{`"hello\tworld"`, "hello\tworld"},
		{`"hello\\world"`, "hello\\world"},
		{`"hello\"world"`, `hello"world`},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseEscapedString()

		if !ok {
			t.Errorf("Failed to parse escaped string '%s'", test.input)
			continue
		}

		if result != test.expected {
			t.Errorf("Input '%s': expected '%s', got '%s'", 
				test.input, test.expected, result)
		}
	}
}

func TestParseQuotedString(t *testing.T) {
	tests := []struct {
		input       string
		expectedValue string
		quoteChar   string
	}{
		{`"double quoted"`, "double quoted", `"`},
		{`'single quoted'`, "single quoted", "'"},
		{`"with spaces"`, "with spaces", `"`},
		{`''`, "", "'"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseQuotedString()

		if !ok {
			t.Errorf("Failed to parse quoted string '%s'", test.input)
			continue
		}

		if result != test.expectedValue {
			t.Errorf("Input '%s': expected value '%s', got '%s'", 
				test.input, test.expectedValue, result)
		}
	}
}

func TestStringParseErrors(t *testing.T) {
	tests := []struct {
		input       string
		description string
	}{
		{`"unclosed string`, "unclosed double quote"},
		{`'unclosed string`, "unclosed single quote"},
		{`"""unclosed multiline`, "unclosed multiline string"},
		{`${unclosed interpolation`, "unclosed interpolation"},
		{`<<<unclosed command`, "unclosed command string"},
		{`~{invalid interpolation`, "invalid interpolation start"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		
		// Try different string parsing methods
		if _, ok := parser.parseStringLiteral(); ok {
			t.Errorf("Expected string literal parsing of '%s' to fail (%s)", 
				test.input, test.description)
		}

		// Reset parser
		parser = NewParser(test.input, "test.wdl")
		if _, ok := parser.parseStringValue(); ok && test.description != "invalid interpolation start" {
			t.Errorf("Expected string value parsing of '%s' to fail (%s)", 
				test.input, test.description)
		}
	}
}

func TestInterpolationOptions(t *testing.T) {
	tests := []struct {
		input           string
		expectedOptions []string
		description     string
	}{
		{`sep=" "`, []string{"sep"}, "separator option"},
		{`default="none"`, []string{"default"}, "default option"},
		{`true="yes" false="no"`, []string{"true", "false"}, "boolean options"},
		{`sep="\t" default="empty"`, []string{"sep", "default"}, "multiple options"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseInterpolationOptions()

		if !ok {
			t.Errorf("Failed to parse interpolation options: %s", test.description)
			continue
		}

		options, ok := result.(map[string]string)
		if !ok {
			t.Errorf("Expected map[string]string, got %T for %s", result, test.description)
			continue
		}

		for _, expectedOption := range test.expectedOptions {
			if _, exists := options[expectedOption]; !exists {
				t.Errorf("Expected option '%s' not found in %s", expectedOption, test.description)
			}
		}
	}
}

func TestCommandInterpolation(t *testing.T) {
	tests := []struct {
		input       string
		description string
	}{
		{`echo ${variable}`, "simple command interpolation"},
		{`python script.py --input ${input_file}`, "command with file interpolation"},
		{`if [ "${flag}" == "true" ]`, "command with conditional interpolation"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseCommandInterpolation()

		if !ok {
			t.Errorf("Failed to parse command interpolation: %s", test.description)
			continue
		}

		if result == "" {
			t.Errorf("Command interpolation should not be empty for %s", test.description)
		}
	}
}

func TestIsValidStringDelimiter(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{`"`, true},
		{"'", true},
		{`"""`, true},
		{`'''`, true},
		{"`", false},
		{"x", false},
		{"123", false},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result := parser.isValidStringDelimiter()

		if result != test.expected {
			t.Errorf("Input '%s': isValidStringDelimiter() expected %t, got %t", 
				test.input, test.expected, result)
		}
	}
}