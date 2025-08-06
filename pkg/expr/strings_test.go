package expr

import (
	"testing"

	"github.com/bioflowy/flowy/pkg/env"
	"github.com/bioflowy/flowy/pkg/errors"
	"github.com/bioflowy/flowy/pkg/types"
	"github.com/bioflowy/flowy/pkg/values"
)

// Test TaskCommand

func TestTaskCommand(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	// Simple command without interpolation
	cmd := NewTaskCommand("    echo hello\n    echo world", nil, pos)
	
	// Test type inference
	inferredType, err := cmd.InferType(nil, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if inferredType.String() != "String" {
		t.Errorf("Expected String type, got %s", inferredType.String())
	}
	
	// Test evaluation (should dedent)
	value, err := cmd.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	stringValue, ok := value.(*values.StringValue)
	if !ok {
		t.Errorf("Expected StringValue, got %T", value)
	}
	
	expected := "echo hello\necho world"
	if stringValue.Value().(string) != expected {
		t.Errorf("Expected '%s', got '%s'", expected, stringValue.Value().(string))
	}
	
	// Test string representation
	if cmd.String() != "command{    echo hello\n    echo world}" {
		t.Errorf("Unexpected string representation: %s", cmd.String())
	}
}

func TestTaskCommandWithInterpolation(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	// Create command with interpolation
	nameVar := NewIdentifier("input", pos)
	interpolation := []Expr{nameVar}
	cmd := NewTaskCommand("echo ${input}", interpolation, pos)
	
	// Create environments
	typeEnv := env.NewBindings[types.Base]()
	typeEnv = typeEnv.Bind("input", types.NewString(false), nil)
	
	valueEnv := env.NewBindings[values.Base]()
	valueEnv = valueEnv.Bind("input", values.NewString("hello", false), nil)
	
	// Test type inference
	inferredType, err := cmd.InferType(typeEnv, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if inferredType.String() != "String" {
		t.Errorf("Expected String type, got %s", inferredType.String())
	}
	
	// Test evaluation
	value, err := cmd.Eval(valueEnv, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	stringValue, ok := value.(*values.StringValue)
	if !ok {
		t.Errorf("Expected StringValue, got %T", value)
	}
	
	// Should append interpolated value (simplified implementation)
	if stringValue.Value().(string) != "echo ${input}hello" {
		t.Errorf("Expected 'echo ${input}hello', got '%s'", stringValue.Value().(string))
	}
	
	// Test children
	children := cmd.Children()
	if len(children) != 1 {
		t.Errorf("Expected 1 child, got %d", len(children))
	}
}

// Test MultilineString

func TestMultilineString(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	// Test multiline string processing
	value := "    line1\\n    line2\\n    line3"
	ms := NewMultilineString(value, nil, pos)
	
	// Test type inference
	inferredType, err := ms.InferType(nil, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if inferredType.String() != "String" {
		t.Errorf("Expected String type, got %s", inferredType.String())
	}
	
	// Test evaluation
	result, err := ms.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	stringValue, ok := result.(*values.StringValue)
	if !ok {
		t.Errorf("Expected StringValue, got %T", result)
	}
	
	// Should process escape sequences and dedent
	expected := "line1\nline2\nline3"
	if stringValue.Value().(string) != expected {
		t.Errorf("Expected '%s', got '%s'", expected, stringValue.Value().(string))
	}
	
	// Test string representation
	expectedStr := `""""    line1\n    line2\n    line3""""`
	if ms.String() != expectedStr {
		t.Errorf("Expected '%s', got '%s'", expectedStr, ms.String())
	}
}

func TestMultilineStringWithInterpolation(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	// Create multiline string with interpolation
	nameVar := NewIdentifier("name", pos)
	interpolation := []Expr{nameVar}
	ms := NewMultilineString("Hello ${name}\\nWorld", interpolation, pos)
	
	// Create environments
	typeEnv := env.NewBindings[types.Base]()
	typeEnv = typeEnv.Bind("name", types.NewString(false), nil)
	
	valueEnv := env.NewBindings[values.Base]()
	valueEnv = valueEnv.Bind("name", values.NewString("WDL", false), nil)
	
	// Test evaluation
	result, err := ms.Eval(valueEnv, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	stringValue, ok := result.(*values.StringValue)
	if !ok {
		t.Errorf("Expected StringValue, got %T", result)
	}
	
	// Should process and append interpolated value
	expected := "Hello ${name}\nWorldWDL"
	if stringValue.Value().(string) != expected {
		t.Errorf("Expected '%s', got '%s'", expected, stringValue.Value().(string))
	}
}

// Test LeftName

func TestLeftName(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	leftName := NewLeftName("variable", pos)
	
	// LeftName should fail type inference
	_, err := leftName.InferType(nil, nil)
	if err == nil {
		t.Errorf("Expected LeftName type inference to fail")
	}
	
	// LeftName should fail evaluation
	_, err = leftName.Eval(nil, nil)
	if err == nil {
		t.Errorf("Expected LeftName evaluation to fail")
	}
	
	// LeftName should fail type checking
	err = leftName.TypeCheck(types.NewString(false), nil, nil)
	if err == nil {
		t.Errorf("Expected LeftName type check to fail")
	}
	
	// Test string representation
	if leftName.String() != "_LeftName(variable)" {
		t.Errorf("Expected '_LeftName(variable)', got '%s'", leftName.String())
	}
}

// Test Placeholder

func TestPlaceholder(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	// Simple placeholder
	expr := NewStringLiteral("hello", pos)
	placeholder := NewPlaceholder(expr, nil, pos)
	
	// Test type inference
	inferredType, err := placeholder.InferType(nil, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if inferredType.String() != "String" {
		t.Errorf("Expected String type, got %s", inferredType.String())
	}
	
	// Test evaluation
	result, err := placeholder.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	stringValue, ok := result.(*values.StringValue)
	if !ok {
		t.Errorf("Expected StringValue, got %T", result)
	}
	if stringValue.Value().(string) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", stringValue.Value().(string))
	}
	
	// Test string representation
	if placeholder.String() != "${\"hello\"}" {
		t.Errorf("Expected '${\"hello\"}', got '%s'", placeholder.String())
	}
}

func TestPlaceholderWithBooleanOptions(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	// Boolean expression with true/false options
	expr := NewBooleanLiteral(true, pos)
	trueVal := "YES"
	falseVal := "NO"
	options := &PlaceholderOptions{
		TrueValue:  &trueVal,
		FalseValue: &falseVal,
	}
	placeholder := NewPlaceholder(expr, options, pos)
	
	// Test evaluation
	result, err := placeholder.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	stringValue, ok := result.(*values.StringValue)
	if !ok {
		t.Errorf("Expected StringValue, got %T", result)
	}
	if stringValue.Value().(string) != "YES" {
		t.Errorf("Expected 'YES', got '%s'", stringValue.Value().(string))
	}
	
	// Test with false
	expr2 := NewBooleanLiteral(false, pos)
	placeholder2 := NewPlaceholder(expr2, options, pos)
	result2, err := placeholder2.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	stringValue2, ok := result2.(*values.StringValue)
	if !ok {
		t.Errorf("Expected StringValue, got %T", result2)
	}
	if stringValue2.Value().(string) != "NO" {
		t.Errorf("Expected 'NO', got '%s'", stringValue2.Value().(string))
	}
}

func TestPlaceholderWithArraySeparator(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	// Array expression with separator
	items := []Expr{
		NewStringLiteral("a", pos),
		NewStringLiteral("b", pos),
		NewStringLiteral("c", pos),
	}
	arrayExpr := NewArrayLiteral(items, pos)
	
	options := &PlaceholderOptions{
		Separator: ",",
	}
	placeholder := NewPlaceholder(arrayExpr, options, pos)
	
	// Test evaluation
	result, err := placeholder.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	stringValue, ok := result.(*values.StringValue)
	if !ok {
		t.Errorf("Expected StringValue, got %T", result)
	}
	if stringValue.Value().(string) != "a,b,c" {
		t.Errorf("Expected 'a,b,c', got '%s'", stringValue.Value().(string))
	}
}

func TestPlaceholderWithDefault(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	// Expression that will fail (unknown identifier)
	expr := NewIdentifier("unknown", pos)
	defaultExpr := NewStringLiteral("default_value", pos)
	
	options := &PlaceholderOptions{
		Default: defaultExpr,
	}
	placeholder := NewPlaceholder(expr, options, pos)
	
	// Test evaluation - should use default when main expression fails
	result, err := placeholder.Eval(env.NewBindings[values.Base](), nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	stringValue, ok := result.(*values.StringValue)
	if !ok {
		t.Errorf("Expected StringValue, got %T", result)
	}
	if stringValue.Value().(string) != "default_value" {
		t.Errorf("Expected 'default_value', got '%s'", stringValue.Value().(string))
	}
}

func TestPlaceholderChildren(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	expr := NewStringLiteral("main", pos)
	defaultExpr := NewStringLiteral("default", pos)
	
	options := &PlaceholderOptions{
		Default: defaultExpr,
	}
	placeholder := NewPlaceholder(expr, options, pos)
	
	children := placeholder.Children()
	if len(children) != 2 {
		t.Errorf("Expected 2 children, got %d", len(children))
	}
	
	if children[0] != expr {
		t.Errorf("Expected first child to be main expression")
	}
	if children[1] != defaultExpr {
		t.Errorf("Expected second child to be default expression")
	}
}

// Test type checking

func TestStringTypeCheck(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	tests := []struct {
		name         string
		expr         Expr
		expectedType types.Base
		shouldFail   bool
	}{
		{
			name:         "TaskCommand to String",
			expr:         NewTaskCommand("echo hello", nil, pos),
			expectedType: types.NewString(false),
			shouldFail:   false,
		},
		{
			name:         "MultilineString to String",
			expr:         NewMultilineString("hello\\nworld", nil, pos),
			expectedType: types.NewString(false),
			shouldFail:   false,
		},
		{
			name:         "Placeholder to String", 
			expr:         NewPlaceholder(NewStringLiteral("test", pos), nil, pos),
			expectedType: types.NewString(false),
			shouldFail:   false,
		},
		{
			name:         "TaskCommand to Array (should fail)",
			expr:         NewTaskCommand("echo hello", nil, pos),
			expectedType: types.NewArray(types.NewString(false), false, false),
			shouldFail:   true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.expr.TypeCheck(tt.expectedType, nil, nil)
			
			if tt.shouldFail && err == nil {
				t.Errorf("Expected type check to fail but it succeeded")
			}
			
			if !tt.shouldFail && err != nil {
				t.Errorf("Expected type check to succeed but got error: %v", err)
			}
		})
	}
}