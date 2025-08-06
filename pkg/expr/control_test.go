package expr

import (
	"testing"

	"github.com/bioflowy/flowy/pkg/env"
	"github.com/bioflowy/flowy/pkg/errors"
	"github.com/bioflowy/flowy/pkg/types"
	"github.com/bioflowy/flowy/pkg/values"
)

// Test IfThenElse

func TestIfThenElseBasic(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// if true then "yes" else "no"
	condition := NewBooleanLiteral(true, pos)
	thenExpr := NewStringLiteral("yes", pos)
	elseExpr := NewStringLiteral("no", pos)
	ifExpr := NewIfThenElse(condition, thenExpr, elseExpr, pos)

	// Test type inference
	inferredType, err := ifExpr.InferType(nil, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if inferredType.String() != "String" {
		t.Errorf("Expected String type, got %s", inferredType.String())
	}

	// Test evaluation
	value, err := ifExpr.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	stringValue, ok := value.(*values.StringValue)
	if !ok {
		t.Errorf("Expected StringValue, got %T", value)
	}
	if stringValue.Value().(string) != "yes" {
		t.Errorf("Expected 'yes', got %s", stringValue.Value().(string))
	}

	// Test string representation
	expected := "if true then \"yes\" else \"no\""
	if ifExpr.String() != expected {
		t.Errorf("Expected '%s', got %s", expected, ifExpr.String())
	}

	// Test children
	children := ifExpr.Children()
	if len(children) != 3 {
		t.Errorf("Expected 3 children, got %d", len(children))
	}
}

func TestIfThenElseFalseCondition(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// if false then 10 else 20
	condition := NewBooleanLiteral(false, pos)
	thenExpr := NewIntLiteral(10, pos)
	elseExpr := NewIntLiteral(20, pos)
	ifExpr := NewIfThenElse(condition, thenExpr, elseExpr, pos)

	// Test evaluation
	value, err := ifExpr.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	intValue, ok := value.(*values.IntValue)
	if !ok {
		t.Errorf("Expected IntValue, got %T", value)
	}
	if intValue.Value().(int64) != 20 {
		t.Errorf("Expected 20, got %d", intValue.Value().(int64))
	}
}

func TestIfThenElseTypeUnification(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// if true then 1 else 2.5 (Int and Float should unify to Float)
	condition := NewBooleanLiteral(true, pos)
	thenExpr := NewIntLiteral(1, pos)
	elseExpr := NewFloatLiteral(2.5, pos)
	ifExpr := NewIfThenElse(condition, thenExpr, elseExpr, pos)

	// Test type inference
	inferredType, err := ifExpr.InferType(nil, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if inferredType.String() != "Float" {
		t.Errorf("Expected Float type, got %s", inferredType.String())
	}
}

func TestIfThenElseIncompatibleTypes(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// if true then 1 else "hello" (incompatible types)
	condition := NewBooleanLiteral(true, pos)
	thenExpr := NewIntLiteral(1, pos)
	elseExpr := NewStringLiteral("hello", pos)
	ifExpr := NewIfThenElse(condition, thenExpr, elseExpr, pos)

	// Test type inference - should fail
	_, err := ifExpr.InferType(nil, nil)
	if err == nil {
		t.Errorf("Expected type inference to fail for incompatible branch types")
	}
}

func TestIfThenElseNonBooleanCondition(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// if "string" then 1 else 2 (non-boolean condition)
	condition := NewStringLiteral("string", pos)
	thenExpr := NewIntLiteral(1, pos)
	elseExpr := NewIntLiteral(2, pos)
	ifExpr := NewIfThenElse(condition, thenExpr, elseExpr, pos)

	// Test type inference - should fail
	_, err := ifExpr.InferType(nil, nil)
	if err == nil {
		t.Errorf("Expected type inference to fail for non-boolean condition")
	}
}

func TestIfThenElseWithVariables(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// if condition then x else y
	conditionVar := NewIdentifier("condition", pos)
	thenVar := NewIdentifier("x", pos)
	elseVar := NewIdentifier("y", pos)
	ifExpr := NewIfThenElse(conditionVar, thenVar, elseVar, pos)

	// Create environments
	typeEnv := env.NewBindings[types.Base]()
	typeEnv = typeEnv.Bind("condition", types.NewBoolean(false), nil)
	typeEnv = typeEnv.Bind("x", types.NewInt(false), nil)
	typeEnv = typeEnv.Bind("y", types.NewInt(false), nil)

	valueEnv := env.NewBindings[values.Base]()
	valueEnv = valueEnv.Bind("condition", values.NewBoolean(true, false), nil)
	valueEnv = valueEnv.Bind("x", values.NewInt(42, false), nil)
	valueEnv = valueEnv.Bind("y", values.NewInt(100, false), nil)

	// Test evaluation
	value, err := ifExpr.Eval(valueEnv, nil)
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
}

// Test LogicalAnd

func TestLogicalAndBasic(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// true && false
	leftExpr := NewBooleanLiteral(true, pos)
	rightExpr := NewBooleanLiteral(false, pos)
	andExpr := NewLogicalAnd(leftExpr, rightExpr, pos)

	// Test type inference
	inferredType, err := andExpr.InferType(nil, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if inferredType.String() != "Boolean" {
		t.Errorf("Expected Boolean type, got %s", inferredType.String())
	}

	// Test evaluation
	value, err := andExpr.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	boolValue, ok := value.(*values.BooleanValue)
	if !ok {
		t.Errorf("Expected BooleanValue, got %T", value)
	}
	if boolValue.Value().(bool) != false {
		t.Errorf("Expected false, got %t", boolValue.Value().(bool))
	}

	// Test string representation
	expected := "(true && false)"
	if andExpr.String() != expected {
		t.Errorf("Expected '%s', got %s", expected, andExpr.String())
	}
}

func TestLogicalAndShortCircuit(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// false && true (should short-circuit)
	leftExpr := NewBooleanLiteral(false, pos)
	rightExpr := NewBooleanLiteral(true, pos)
	andExpr := NewLogicalAnd(leftExpr, rightExpr, pos)

	// Test evaluation
	value, err := andExpr.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	boolValue, ok := value.(*values.BooleanValue)
	if !ok {
		t.Errorf("Expected BooleanValue, got %T", value)
	}
	if boolValue.Value().(bool) != false {
		t.Errorf("Expected false (short-circuit), got %t", boolValue.Value().(bool))
	}
}

func TestLogicalAndTrueTrue(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// true && true
	leftExpr := NewBooleanLiteral(true, pos)
	rightExpr := NewBooleanLiteral(true, pos)
	andExpr := NewLogicalAnd(leftExpr, rightExpr, pos)

	// Test evaluation
	value, err := andExpr.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	boolValue, ok := value.(*values.BooleanValue)
	if !ok {
		t.Errorf("Expected BooleanValue, got %T", value)
	}
	if boolValue.Value().(bool) != true {
		t.Errorf("Expected true, got %t", boolValue.Value().(bool))
	}
}

func TestLogicalAndWithNull(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// null && true (should return null)
	leftExpr := NewNullLiteral(pos)
	rightExpr := NewBooleanLiteral(true, pos)
	andExpr := NewLogicalAnd(leftExpr, rightExpr, pos)

	// Test evaluation
	value, err := andExpr.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	_, ok := value.(*values.Null)
	if !ok {
		t.Errorf("Expected Null value, got %T", value)
	}
}

func TestLogicalAndNonBooleanOperand(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// "string" && true (non-boolean operand)
	leftExpr := NewStringLiteral("string", pos)
	rightExpr := NewBooleanLiteral(true, pos)
	andExpr := NewLogicalAnd(leftExpr, rightExpr, pos)

	// Test type inference - should fail
	_, err := andExpr.InferType(nil, nil)
	if err == nil {
		t.Errorf("Expected type inference to fail for non-boolean operand")
	}
}

// Test LogicalOr

func TestLogicalOrBasic(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// false || true
	leftExpr := NewBooleanLiteral(false, pos)
	rightExpr := NewBooleanLiteral(true, pos)
	orExpr := NewLogicalOr(leftExpr, rightExpr, pos)

	// Test evaluation
	value, err := orExpr.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	boolValue, ok := value.(*values.BooleanValue)
	if !ok {
		t.Errorf("Expected BooleanValue, got %T", value)
	}
	if boolValue.Value().(bool) != true {
		t.Errorf("Expected true, got %t", boolValue.Value().(bool))
	}

	// Test string representation
	expected := "(false || true)"
	if orExpr.String() != expected {
		t.Errorf("Expected '%s', got %s", expected, orExpr.String())
	}
}

func TestLogicalOrShortCircuit(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// true || false (should short-circuit)
	leftExpr := NewBooleanLiteral(true, pos)
	rightExpr := NewBooleanLiteral(false, pos)
	orExpr := NewLogicalOr(leftExpr, rightExpr, pos)

	// Test evaluation
	value, err := orExpr.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	boolValue, ok := value.(*values.BooleanValue)
	if !ok {
		t.Errorf("Expected BooleanValue, got %T", value)
	}
	if boolValue.Value().(bool) != true {
		t.Errorf("Expected true (short-circuit), got %t", boolValue.Value().(bool))
	}
}

func TestLogicalOrFalseFalse(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// false || false
	leftExpr := NewBooleanLiteral(false, pos)
	rightExpr := NewBooleanLiteral(false, pos)
	orExpr := NewLogicalOr(leftExpr, rightExpr, pos)

	// Test evaluation
	value, err := orExpr.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	boolValue, ok := value.(*values.BooleanValue)
	if !ok {
		t.Errorf("Expected BooleanValue, got %T", value)
	}
	if boolValue.Value().(bool) != false {
		t.Errorf("Expected false, got %t", boolValue.Value().(bool))
	}
}

// Test LogicalNot

func TestLogicalNotBasic(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// !true
	operand := NewBooleanLiteral(true, pos)
	notExpr := NewLogicalNot(operand, pos)

	// Test type inference
	inferredType, err := notExpr.InferType(nil, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if inferredType.String() != "Boolean" {
		t.Errorf("Expected Boolean type, got %s", inferredType.String())
	}

	// Test evaluation
	value, err := notExpr.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	boolValue, ok := value.(*values.BooleanValue)
	if !ok {
		t.Errorf("Expected BooleanValue, got %T", value)
	}
	if boolValue.Value().(bool) != false {
		t.Errorf("Expected false, got %t", boolValue.Value().(bool))
	}

	// Test string representation
	expected := "!true"
	if notExpr.String() != expected {
		t.Errorf("Expected '%s', got %s", expected, notExpr.String())
	}

	// Test children
	children := notExpr.Children()
	if len(children) != 1 {
		t.Errorf("Expected 1 child, got %d", len(children))
	}
}

func TestLogicalNotFalse(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// !false
	operand := NewBooleanLiteral(false, pos)
	notExpr := NewLogicalNot(operand, pos)

	// Test evaluation
	value, err := notExpr.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	boolValue, ok := value.(*values.BooleanValue)
	if !ok {
		t.Errorf("Expected BooleanValue, got %T", value)
	}
	if boolValue.Value().(bool) != true {
		t.Errorf("Expected true, got %t", boolValue.Value().(bool))
	}
}

func TestLogicalNotWithNull(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// !null
	operand := NewNullLiteral(pos)
	notExpr := NewLogicalNot(operand, pos)

	// Test evaluation - should return null
	value, err := notExpr.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	_, ok := value.(*values.Null)
	if !ok {
		t.Errorf("Expected Null value, got %T", value)
	}
}

func TestLogicalNotNonBoolean(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// !"string" (non-boolean operand)
	operand := NewStringLiteral("string", pos)
	notExpr := NewLogicalNot(operand, pos)

	// Test type inference - should fail
	_, err := notExpr.InferType(nil, nil)
	if err == nil {
		t.Errorf("Expected type inference to fail for non-boolean operand")
	}
}

// Test complex expressions

func TestComplexLogicalExpression(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// !(true && false) || (false || true)
	// Should evaluate to: !false || true = true || true = true

	// Build the expression
	trueExpr1 := NewBooleanLiteral(true, pos)
	falseExpr1 := NewBooleanLiteral(false, pos)
	andExpr := NewLogicalAnd(trueExpr1, falseExpr1, pos)
	notExpr := NewLogicalNot(andExpr, pos)

	falseExpr2 := NewBooleanLiteral(false, pos)
	trueExpr2 := NewBooleanLiteral(true, pos)
	orExpr1 := NewLogicalOr(falseExpr2, trueExpr2, pos)

	finalOrExpr := NewLogicalOr(notExpr, orExpr1, pos)

	// Test evaluation
	value, err := finalOrExpr.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	boolValue, ok := value.(*values.BooleanValue)
	if !ok {
		t.Errorf("Expected BooleanValue, got %T", value)
	}
	if boolValue.Value().(bool) != true {
		t.Errorf("Expected true, got %t", boolValue.Value().(bool))
	}
}

// Test type checking

func TestControlExpressionTypeCheck(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	tests := []struct {
		name         string
		expr         Expr
		expectedType types.Base
		shouldFail   bool
	}{
		{
			name:         "if-then-else to String",
			expr:         NewIfThenElse(NewBooleanLiteral(true, pos), NewStringLiteral("yes", pos), NewStringLiteral("no", pos), pos),
			expectedType: types.NewString(false),
			shouldFail:   false,
		},
		{
			name:         "logical AND to Boolean",
			expr:         NewLogicalAnd(NewBooleanLiteral(true, pos), NewBooleanLiteral(false, pos), pos),
			expectedType: types.NewBoolean(false),
			shouldFail:   false,
		},
		{
			name:         "logical OR to Boolean",
			expr:         NewLogicalOr(NewBooleanLiteral(true, pos), NewBooleanLiteral(false, pos), pos),
			expectedType: types.NewBoolean(false),
			shouldFail:   false,
		},
		{
			name:         "logical NOT to Boolean",
			expr:         NewLogicalNot(NewBooleanLiteral(true, pos), pos),
			expectedType: types.NewBoolean(false),
			shouldFail:   false,
		},
		{
			name:         "if-then-else to wrong type",
			expr:         NewIfThenElse(NewBooleanLiteral(true, pos), NewStringLiteral("yes", pos), NewStringLiteral("no", pos), pos),
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

// Test children methods

func TestControlExpressionChildren(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	// Test IfThenElse children
	condition := NewBooleanLiteral(true, pos)
	thenExpr := NewStringLiteral("yes", pos)
	elseExpr := NewStringLiteral("no", pos)
	ifExpr := NewIfThenElse(condition, thenExpr, elseExpr, pos)

	children := ifExpr.Children()
	if len(children) != 3 {
		t.Errorf("Expected 3 children for IfThenElse, got %d", len(children))
	}
	if children[0] != condition || children[1] != thenExpr || children[2] != elseExpr {
		t.Errorf("IfThenElse children don't match expected order")
	}

	// Test LogicalAnd children
	leftExpr := NewBooleanLiteral(true, pos)
	rightExpr := NewBooleanLiteral(false, pos)
	andExpr := NewLogicalAnd(leftExpr, rightExpr, pos)

	children = andExpr.Children()
	if len(children) != 2 {
		t.Errorf("Expected 2 children for LogicalAnd, got %d", len(children))
	}
	if children[0] != leftExpr || children[1] != rightExpr {
		t.Errorf("LogicalAnd children don't match expected order")
	}

	// Test LogicalNot children
	operand := NewBooleanLiteral(true, pos)
	notExpr := NewLogicalNot(operand, pos)

	children = notExpr.Children()
	if len(children) != 1 {
		t.Errorf("Expected 1 child for LogicalNot, got %d", len(children))
	}
	if children[0] != operand {
		t.Errorf("LogicalNot child doesn't match expected")
	}
}
