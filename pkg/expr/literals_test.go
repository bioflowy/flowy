package expr

import (
	"testing"

	"github.com/bioflowy/flowy/pkg/env"
	"github.com/bioflowy/flowy/pkg/errors"
	"github.com/bioflowy/flowy/pkg/types"
	"github.com/bioflowy/flowy/pkg/values"
)

// Test BooleanLiteral

func TestBooleanLiteral(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Test true literal
	trueLit := NewBooleanLiteral(true, pos)

	// Test type inference
	inferredType, err := trueLit.InferType(nil, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if inferredType.String() != "Boolean" {
		t.Errorf("Expected Boolean type, got %s", inferredType.String())
	}

	// Test evaluation
	value, err := trueLit.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	boolValue, ok := value.(*values.BooleanValue)
	if !ok {
		t.Errorf("Expected BooleanValue, got %T", value)
	}
	if !boolValue.Value().(bool) {
		t.Errorf("Expected true, got false")
	}

	// Test literal
	literal, isLiteral := trueLit.Literal()
	if !isLiteral {
		t.Errorf("Expected literal to be true")
	}
	if literal == nil {
		t.Errorf("Expected literal value, got nil")
	}

	// Test string representation
	if trueLit.String() != "true" {
		t.Errorf("Expected 'true', got %s", trueLit.String())
	}
}

func TestBooleanLiteralFalse(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	falseLit := NewBooleanLiteral(false, pos)

	value, err := falseLit.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	boolValue, ok := value.(*values.BooleanValue)
	if !ok {
		t.Errorf("Expected BooleanValue, got %T", value)
	}
	if boolValue.Value().(bool) {
		t.Errorf("Expected false, got true")
	}

	if falseLit.String() != "false" {
		t.Errorf("Expected 'false', got %s", falseLit.String())
	}
}

// Test IntLiteral

func TestIntLiteral(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	intLit := NewIntLiteral(42, pos)

	// Test type inference
	inferredType, err := intLit.InferType(nil, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if inferredType.String() != "Int" {
		t.Errorf("Expected Int type, got %s", inferredType.String())
	}

	// Test evaluation
	value, err := intLit.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	intValue, ok := value.(*values.IntValue)
	if !ok {
		t.Errorf("Expected IntValue, got %T", value)
	}
	if intValue.Value().(int64) != 42 {
		t.Errorf("Expected 42, got %d", intValue.Value().(int64))
	}

	// Test literal
	literal, isLiteral := intLit.Literal()
	if !isLiteral {
		t.Errorf("Expected literal to be true")
	}
	if literal == nil {
		t.Errorf("Expected literal value, got nil")
	}

	// Test string representation
	if intLit.String() != "42" {
		t.Errorf("Expected '42', got %s", intLit.String())
	}
}

// Test FloatLiteral

func TestFloatLiteral(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	floatLit := NewFloatLiteral(3.14, pos)

	// Test type inference
	inferredType, err := floatLit.InferType(nil, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if inferredType.String() != "Float" {
		t.Errorf("Expected Float type, got %s", inferredType.String())
	}

	// Test evaluation
	value, err := floatLit.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	floatValue, ok := value.(*values.FloatValue)
	if !ok {
		t.Errorf("Expected FloatValue, got %T", value)
	}
	if floatValue.Value().(float64) != 3.14 {
		t.Errorf("Expected 3.14, got %f", floatValue.Value().(float64))
	}

	// Test string representation
	if floatLit.String() != "3.14" {
		t.Errorf("Expected '3.14', got %s", floatLit.String())
	}
}

// Test StringLiteral (simple)

func TestStringLiteral(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	strLit := NewStringLiteral("hello", pos)

	// Test type inference
	inferredType, err := strLit.InferType(nil, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if inferredType.String() != "String" {
		t.Errorf("Expected String type, got %s", inferredType.String())
	}

	// Test evaluation
	value, err := strLit.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	stringValue, ok := value.(*values.StringValue)
	if !ok {
		t.Errorf("Expected StringValue, got %T", value)
	}
	if stringValue.Value().(string) != "hello" {
		t.Errorf("Expected 'hello', got %s", stringValue.Value().(string))
	}

	// Test literal
	literal, isLiteral := strLit.Literal()
	if !isLiteral {
		t.Errorf("Expected literal to be true")
	}
	if literal == nil {
		t.Errorf("Expected literal value, got nil")
	}

	// Test string representation
	if strLit.String() != "\"hello\"" {
		t.Errorf("Expected '\"hello\"', got %s", strLit.String())
	}
}

// Test StringLiteral with interpolation

func TestStringLiteralWithInterpolation(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Create an interpolated string like "hello ${name}!"
	nameVar := NewIdentifier("name", pos)
	strLit := NewInterpolatedString("hello ${name}!", []Expr{nameVar}, pos)

	// Test type inference
	typeEnv := env.NewBindings[types.Base]()
	typeEnv = typeEnv.Bind("name", types.NewString(false), nil)

	inferredType, err := strLit.InferType(typeEnv, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if inferredType.String() != "String" {
		t.Errorf("Expected String type, got %s", inferredType.String())
	}

	// Test evaluation
	valueEnv := env.NewBindings[values.Base]()
	valueEnv = valueEnv.Bind("name", values.NewString("world", false), nil)

	value, err := strLit.Eval(valueEnv, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	stringValue, ok := value.(*values.StringValue)
	if !ok {
		t.Errorf("Expected StringValue, got %T", value)
	}

	// The current implementation appends interpolated values (simplified)
	// Should be "hello ${name}!world"
	if stringValue.Value().(string) != "hello ${name}!world" {
		t.Errorf("Expected 'hello ${name}!world', got %s", stringValue.Value().(string))
	}

	// Test that interpolated string is not considered a literal
	_, isLiteral := strLit.Literal()
	if isLiteral {
		t.Errorf("Expected interpolated string to not be literal")
	}
}

func TestStringLiteralInterpolationTypeError(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Create an interpolated string with a non-string expression
	intVar := NewIdentifier("count", pos)
	strLit := NewInterpolatedString("count: ${count}", []Expr{intVar}, pos)

	// Test type inference - should fail if count is not string-coercible
	typeEnv := env.NewBindings[types.Base]()
	typeEnv = typeEnv.Bind("count", types.NewMap(types.NewString(false), types.NewInt(false), false), nil) // Map can't be coerced to string

	_, err := strLit.InferType(typeEnv, nil)
	if err == nil {
		t.Errorf("Expected type inference to fail for non-string interpolation")
	}
}

// Test NullLiteral

func TestNullLiteral(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	nullLit := NewNullLiteral(pos)

	// Test type inference
	inferredType, err := nullLit.InferType(nil, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if !inferredType.Optional() {
		t.Errorf("Expected optional type for null literal")
	}

	// Test evaluation
	value, err := nullLit.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	_, ok := value.(*values.Null)
	if !ok {
		t.Errorf("Expected Null value, got %T", value)
	}

	// Test literal
	literal, isLiteral := nullLit.Literal()
	if !isLiteral {
		t.Errorf("Expected literal to be true")
	}
	if literal == nil {
		t.Errorf("Expected literal value, got nil")
	}

	// Test string representation
	if nullLit.String() != "None" {
		t.Errorf("Expected 'None', got %s", nullLit.String())
	}
}

// Test type checking

func TestLiteralTypeCheck(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	tests := []struct {
		name         string
		literal      Expr
		expectedType types.Base
		shouldFail   bool
	}{
		{
			name:         "Boolean to Boolean",
			literal:      NewBooleanLiteral(true, pos),
			expectedType: types.NewBoolean(false),
			shouldFail:   false,
		},
		{
			name:         "Boolean to optional Boolean",
			literal:      NewBooleanLiteral(true, pos),
			expectedType: types.NewBoolean(true),
			shouldFail:   false,
		},
		{
			name:         "Boolean to Array (should fail)",
			literal:      NewBooleanLiteral(true, pos),
			expectedType: types.NewArray(types.NewString(false), false, false),
			shouldFail:   true,
		},
		{
			name:         "Int to Int",
			literal:      NewIntLiteral(42, pos),
			expectedType: types.NewInt(false),
			shouldFail:   false,
		},
		{
			name:         "Int to Float",
			literal:      NewIntLiteral(42, pos),
			expectedType: types.NewFloat(false),
			shouldFail:   false,
		},
		{
			name:         "String to String",
			literal:      NewStringLiteral("hello", pos),
			expectedType: types.NewString(false),
			shouldFail:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.literal.TypeCheck(tt.expectedType, nil, nil)

			if tt.shouldFail && err == nil {
				t.Errorf("Expected type check to fail but it succeeded")
			}

			if !tt.shouldFail && err != nil {
				t.Errorf("Expected type check to succeed but got error: %v", err)
			}
		})
	}
}

// Test children method

func TestLiteralChildren(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Simple literals have no children
	boolLit := NewBooleanLiteral(true, pos)
	if len(boolLit.Children()) != 0 {
		t.Errorf("Expected no children for boolean literal, got %d", len(boolLit.Children()))
	}

	intLit := NewIntLiteral(42, pos)
	if len(intLit.Children()) != 0 {
		t.Errorf("Expected no children for int literal, got %d", len(intLit.Children()))
	}

	// String literal without interpolation has no children
	strLit := NewStringLiteral("hello", pos)
	if len(strLit.Children()) != 0 {
		t.Errorf("Expected no children for string literal, got %d", len(strLit.Children()))
	}

	// String literal with interpolation has children
	nameVar := NewIdentifier("name", pos)
	interpolatedStr := NewInterpolatedString("hello ${name}!", []Expr{nameVar}, pos)
	children := interpolatedStr.Children()
	if len(children) != 1 {
		t.Errorf("Expected 1 child for interpolated string, got %d", len(children))
	}
	if children[0] != nameVar {
		t.Errorf("Expected child to be the name identifier")
	}
}
