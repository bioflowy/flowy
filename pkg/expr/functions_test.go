package expr

import (
	"testing"

	"github.com/bioflowy/flowy/pkg/env"
	"github.com/bioflowy/flowy/pkg/errors"
	"github.com/bioflowy/flowy/pkg/types"
	"github.com/bioflowy/flowy/pkg/values"
)

// Test helper functions for function testing
func lengthFunction(args []values.Base, pos errors.SourcePosition) (values.Base, error) {
	if len(args) != 1 {
		return nil, errors.NewEvalErrorFromPos(pos, "length expects 1 argument")
	}
	str, ok := args[0].(*values.StringValue)
	if !ok {
		return nil, errors.NewEvalErrorFromPos(pos, "length expects string argument")
	}
	return values.NewInt(int64(len(str.Value().(string))), false), nil
}

func definedFunction(args []values.Base, pos errors.SourcePosition) (values.Base, error) {
	if len(args) != 1 {
		return nil, errors.NewEvalErrorFromPos(pos, "defined expects 1 argument")
	}
	_, isNull := args[0].(*values.Null)
	return values.NewBoolean(!isNull, false), nil
}

func powerOperator(args []values.Base, pos errors.SourcePosition) (values.Base, error) {
	return values.NewFloat(8.0, false), nil // 2^3 = 8 for testing
}

// Enhanced mock stdlib wrapper
type functionTestStdLib struct {
	*mockStdLib
}

func newFunctionTestStdLib() *functionTestStdLib {
	base := &mockStdLib{
		functions: map[string]*Function{
			"length": {
				Name:       "length",
				ParamTypes: []types.Base{types.NewString(false)},
				ReturnType: types.NewInt(false),
				Variadic:   false,
			},
			"defined": {
				Name:       "defined",
				ParamTypes: []types.Base{types.NewAny(true, false)},
				ReturnType: types.NewBoolean(false),
				Variadic:   false,
			},
		},
	}
	
	return &functionTestStdLib{mockStdLib: base}
}

func (f *functionTestStdLib) CallFunction(name string, args []values.Base, pos errors.SourcePosition) (values.Base, error) {
	switch name {
	case "length":
		return lengthFunction(args, pos)
	case "defined":
		return definedFunction(args, pos)
	}
	return nil, errors.NewEvalErrorFromPos(pos, "unknown function: "+name)
}

func (f *functionTestStdLib) HasOperator(op string) bool {
	return op == "**" || op == "+" || op == "-" || op == "==" || op == "!="
}

func (f *functionTestStdLib) CallOperator(op string, args []values.Base, pos errors.SourcePosition) (values.Base, error) {
	switch op {
	case "**":
		return powerOperator(args, pos)
	default:
		return nil, errors.NewEvalErrorFromPos(pos, "unknown operator: "+op)
	}
}

// Test Apply (function calls)

func TestApplyBasicFunction(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	stdlib := newFunctionTestStdLib()
	
	// length("hello")
	arg := NewStringLiteral("hello", pos)
	apply := NewApply("length", []Expr{arg}, pos)
	
	// Test type inference
	inferredType, err := apply.InferType(nil, stdlib)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if inferredType.String() != "Int" {
		t.Errorf("Expected Int type, got %s", inferredType.String())
	}
	
	// Test evaluation
	value, err := apply.Eval(nil, stdlib)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	intValue, ok := value.(*values.IntValue)
	if !ok {
		t.Errorf("Expected IntValue, got %T", value)
	}
	if intValue.Value().(int64) != 5 {
		t.Errorf("Expected 5, got %d", intValue.Value().(int64))
	}
	
	// Test string representation
	expected := "length(\"hello\")"
	if apply.String() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, apply.String())
	}
	
	// Test children
	children := apply.Children()
	if len(children) != 1 {
		t.Errorf("Expected 1 child, got %d", len(children))
	}
}

func TestApplyUnknownFunction(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	// unknown_func()
	apply := NewApply("unknown_func", []Expr{}, pos)
	
	// Test with nil stdlib
	_, err := apply.InferType(nil, nil)
	if err == nil {
		t.Errorf("Expected type inference to fail with nil stdlib")
	}
	
	// Test with stdlib that doesn't have the function
	stdlib := newFunctionTestStdLib()
	_, err = apply.InferType(nil, stdlib)
	if err == nil {
		t.Errorf("Expected type inference to fail for unknown function")
	}
}

func TestApplyWithVariables(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	stdlib := newFunctionTestStdLib()
	
	// length(name) where name is a variable
	nameVar := NewIdentifier("name", pos)
	apply := NewApply("length", []Expr{nameVar}, pos)
	
	// Create environments
	typeEnv := env.NewBindings[types.Base]()
	typeEnv = typeEnv.Bind("name", types.NewString(false), nil)
	
	valueEnv := env.NewBindings[values.Base]()
	valueEnv = valueEnv.Bind("name", values.NewString("world", false), nil)
	
	// Test evaluation
	value, err := apply.Eval(valueEnv, stdlib)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	intValue, ok := value.(*values.IntValue)
	if !ok {
		t.Errorf("Expected IntValue, got %T", value)
	}
	if intValue.Value().(int64) != 5 {
		t.Errorf("Expected 5, got %d", intValue.Value().(int64))
	}
}

func TestApplyTypeCheck(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	stdlib := newFunctionTestStdLib()
	
	arg := NewStringLiteral("test", pos)
	apply := NewApply("length", []Expr{arg}, pos)
	
	tests := []struct {
		name         string
		expectedType types.Base
		shouldFail   bool
	}{
		{
			name:         "correct type",
			expectedType: types.NewInt(false),
			shouldFail:   false,
		},
		{
			name:         "wrong type",
			expectedType: types.NewArray(types.NewString(false), false, false),
			shouldFail:   true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := apply.TypeCheck(tt.expectedType, nil, stdlib)
			
			if tt.shouldFail && err == nil {
				t.Errorf("Expected type check to fail but it succeeded")
			}
			
			if !tt.shouldFail && err != nil {
				t.Errorf("Expected type check to succeed but got error: %v", err)
			}
		})
	}
}

// Test BinaryOp (binary operators)

func TestBinaryOpArithmetic(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	tests := []struct {
		name     string
		left     Expr
		operator string
		right    Expr
		expected interface{}
		expType  string
	}{
		{
			name:     "int addition",
			left:     NewIntLiteral(5, pos),
			operator: "+",
			right:    NewIntLiteral(3, pos),
			expected: int64(8),
			expType:  "Int",
		},
		{
			name:     "float addition",
			left:     NewFloatLiteral(2.5, pos),
			operator: "+",
			right:    NewFloatLiteral(1.5, pos),
			expected: 4.0,
			expType:  "Float",
		},
		{
			name:     "mixed int float addition",
			left:     NewIntLiteral(5, pos),
			operator: "+",
			right:    NewFloatLiteral(2.5, pos),
			expected: 7.5,
			expType:  "Float",
		},
		{
			name:     "string concatenation",
			left:     NewStringLiteral("hello", pos),
			operator: "+",
			right:    NewStringLiteral(" world", pos),
			expected: "hello world",
			expType:  "String",
		},
		{
			name:     "int subtraction",
			left:     NewIntLiteral(10, pos),
			operator: "-",
			right:    NewIntLiteral(3, pos),
			expected: int64(7),
			expType:  "Int",
		},
		{
			name:     "int multiplication",
			left:     NewIntLiteral(4, pos),
			operator: "*",
			right:    NewIntLiteral(3, pos),
			expected: int64(12),
			expType:  "Int",
		},
		{
			name:     "int division",
			left:     NewIntLiteral(8, pos),
			operator: "/",
			right:    NewIntLiteral(2, pos),
			expected: 4.0,
			expType:  "Float", // Division always returns float
		},
		{
			name:     "int modulo",
			left:     NewIntLiteral(10, pos),
			operator: "%",
			right:    NewIntLiteral(3, pos),
			expected: int64(1),
			expType:  "Int",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binOp := NewBinaryOp(tt.left, tt.operator, tt.right, pos)
			
			// Test type inference
			inferredType, err := binOp.InferType(nil, nil)
			if err != nil {
				t.Errorf("Type inference failed: %v", err)
			}
			if inferredType.String() != tt.expType {
				t.Errorf("Expected %s type, got %s", tt.expType, inferredType.String())
			}
			
			// Test evaluation
			value, err := binOp.Eval(nil, nil)
			if err != nil {
				t.Errorf("Evaluation failed: %v", err)
			}
			
			switch tt.expType {
			case "Int":
				intValue, ok := value.(*values.IntValue)
				if !ok {
					t.Errorf("Expected IntValue, got %T", value)
				}
				if intValue.Value().(int64) != tt.expected.(int64) {
					t.Errorf("Expected %d, got %d", tt.expected.(int64), intValue.Value().(int64))
				}
			case "Float":
				floatValue, ok := value.(*values.FloatValue)
				if !ok {
					t.Errorf("Expected FloatValue, got %T", value)
				}
				if floatValue.Value().(float64) != tt.expected.(float64) {
					t.Errorf("Expected %f, got %f", tt.expected.(float64), floatValue.Value().(float64))
				}
			case "String":
				stringValue, ok := value.(*values.StringValue)
				if !ok {
					t.Errorf("Expected StringValue, got %T", value)
				}
				if stringValue.Value().(string) != tt.expected.(string) {
					t.Errorf("Expected '%s', got '%s'", tt.expected.(string), stringValue.Value().(string))
				}
			}
		})
	}
}

func TestBinaryOpComparison(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	tests := []struct {
		name     string
		left     Expr
		operator string
		right    Expr
		expected bool
	}{
		{
			name:     "int less than",
			left:     NewIntLiteral(5, pos),
			operator: "<",
			right:    NewIntLiteral(10, pos),
			expected: true,
		},
		{
			name:     "int greater than",
			left:     NewIntLiteral(10, pos),
			operator: ">",
			right:    NewIntLiteral(5, pos),
			expected: true,
		},
		{
			name:     "int less than or equal",
			left:     NewIntLiteral(5, pos),
			operator: "<=",
			right:    NewIntLiteral(5, pos),
			expected: true,
		},
		{
			name:     "int greater than or equal",
			left:     NewIntLiteral(10, pos),
			operator: ">=",
			right:    NewIntLiteral(5, pos),
			expected: true,
		},
		{
			name:     "string comparison",
			left:     NewStringLiteral("apple", pos),
			operator: "<",
			right:    NewStringLiteral("banana", pos),
			expected: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binOp := NewBinaryOp(tt.left, tt.operator, tt.right, pos)
			
			// Test type inference
			inferredType, err := binOp.InferType(nil, nil)
			if err != nil {
				t.Errorf("Type inference failed: %v", err)
			}
			if inferredType.String() != "Boolean" {
				t.Errorf("Expected Boolean type, got %s", inferredType.String())
			}
			
			// Test evaluation
			value, err := binOp.Eval(nil, nil)
			if err != nil {
				t.Errorf("Evaluation failed: %v", err)
			}
			boolValue, ok := value.(*values.BooleanValue)
			if !ok {
				t.Errorf("Expected BooleanValue, got %T", value)
			}
			if boolValue.Value().(bool) != tt.expected {
				t.Errorf("Expected %t, got %t", tt.expected, boolValue.Value().(bool))
			}
		})
	}
}

func TestBinaryOpEquality(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	// Test equality
	eqOp := NewBinaryOp(NewIntLiteral(5, pos), "==", NewIntLiteral(5, pos), pos)
	value, err := eqOp.Eval(nil, nil)
	if err != nil {
		t.Errorf("Equality evaluation failed: %v", err)
	}
	boolValue, ok := value.(*values.BooleanValue)
	if !ok {
		t.Errorf("Expected BooleanValue, got %T", value)
	}
	if !boolValue.Value().(bool) {
		t.Errorf("Expected true for equality, got false")
	}
	
	// Test inequality
	neqOp := NewBinaryOp(NewIntLiteral(5, pos), "!=", NewIntLiteral(3, pos), pos)
	value, err = neqOp.Eval(nil, nil)
	if err != nil {
		t.Errorf("Inequality evaluation failed: %v", err)
	}
	boolValue, ok = value.(*values.BooleanValue)
	if !ok {
		t.Errorf("Expected BooleanValue, got %T", value)
	}
	if !boolValue.Value().(bool) {
		t.Errorf("Expected true for inequality, got false")
	}
}

func TestBinaryOpNullPropagation(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	// null + 5 should return null
	binOp := NewBinaryOp(NewNullLiteral(pos), "+", NewIntLiteral(5, pos), pos)
	value, err := binOp.Eval(nil, nil)
	if err != nil {
		t.Errorf("Null propagation evaluation failed: %v", err)
	}
	_, ok := value.(*values.Null)
	if !ok {
		t.Errorf("Expected Null value, got %T", value)
	}
}

func TestBinaryOpInvalidTypes(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	// "hello" - 5 should fail (string subtraction)
	binOp := NewBinaryOp(NewStringLiteral("hello", pos), "-", NewIntLiteral(5, pos), pos)
	_, err := binOp.InferType(nil, nil)
	if err == nil {
		t.Errorf("Expected type inference to fail for string subtraction")
	}
}

func TestBinaryOpWithStdlibOperator(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	stdlib := newFunctionTestStdLib()
	
	// 2 ** 3 (power operator from stdlib)
	binOp := NewBinaryOp(NewIntLiteral(2, pos), "**", NewIntLiteral(3, pos), pos)
	
	// Test evaluation with stdlib
	value, err := binOp.Eval(nil, stdlib)
	if err != nil {
		t.Errorf("Stdlib operator evaluation failed: %v", err)
	}
	floatValue, ok := value.(*values.FloatValue)
	if !ok {
		t.Errorf("Expected FloatValue, got %T", value)
	}
	if floatValue.Value().(float64) != 8.0 {
		t.Errorf("Expected 8.0, got %f", floatValue.Value().(float64))
	}
}

func TestBinaryOpChildren(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	left := NewIntLiteral(5, pos)
	right := NewIntLiteral(3, pos)
	binOp := NewBinaryOp(left, "+", right, pos)
	
	children := binOp.Children()
	if len(children) != 2 {
		t.Errorf("Expected 2 children, got %d", len(children))
	}
	if children[0] != left || children[1] != right {
		t.Errorf("Children don't match expected order")
	}
}

// Test UnaryOp (unary operators)

func TestUnaryOpNumeric(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	tests := []struct {
		name     string
		operator string
		operand  Expr
		expected interface{}
		expType  string
	}{
		{
			name:     "unary plus int",
			operator: "+",
			operand:  NewIntLiteral(5, pos),
			expected: int64(5),
			expType:  "Int",
		},
		{
			name:     "unary minus int",
			operator: "-",
			operand:  NewIntLiteral(5, pos),
			expected: int64(-5),
			expType:  "Int",
		},
		{
			name:     "unary plus float",
			operator: "+",
			operand:  NewFloatLiteral(3.14, pos),
			expected: 3.14,
			expType:  "Float",
		},
		{
			name:     "unary minus float",
			operator: "-",
			operand:  NewFloatLiteral(3.14, pos),
			expected: -3.14,
			expType:  "Float",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unaryOp := NewUnaryOp(tt.operator, tt.operand, pos)
			
			// Test type inference
			inferredType, err := unaryOp.InferType(nil, nil)
			if err != nil {
				t.Errorf("Type inference failed: %v", err)
			}
			if inferredType.String() != tt.expType {
				t.Errorf("Expected %s type, got %s", tt.expType, inferredType.String())
			}
			
			// Test evaluation
			value, err := unaryOp.Eval(nil, nil)
			if err != nil {
				t.Errorf("Evaluation failed: %v", err)
			}
			
			switch tt.expType {
			case "Int":
				intValue, ok := value.(*values.IntValue)
				if !ok {
					t.Errorf("Expected IntValue, got %T", value)
				}
				if intValue.Value().(int64) != tt.expected.(int64) {
					t.Errorf("Expected %d, got %d", tt.expected.(int64), intValue.Value().(int64))
				}
			case "Float":
				floatValue, ok := value.(*values.FloatValue)
				if !ok {
					t.Errorf("Expected FloatValue, got %T", value)
				}
				if floatValue.Value().(float64) != tt.expected.(float64) {
					t.Errorf("Expected %f, got %f", tt.expected.(float64), floatValue.Value().(float64))
				}
			}
		})
	}
}

func TestUnaryOpLogicalNot(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	// !true
	unaryOp := NewUnaryOp("!", NewBooleanLiteral(true, pos), pos)
	
	// Test type inference
	inferredType, err := unaryOp.InferType(nil, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if inferredType.String() != "Boolean" {
		t.Errorf("Expected Boolean type, got %s", inferredType.String())
	}
	
	// Test evaluation
	value, err := unaryOp.Eval(nil, nil)
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

func TestUnaryOpNullPropagation(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	// -null should return null
	unaryOp := NewUnaryOp("-", NewNullLiteral(pos), pos)
	value, err := unaryOp.Eval(nil, nil)
	if err != nil {
		t.Errorf("Null propagation evaluation failed: %v", err)
	}
	_, ok := value.(*values.Null)
	if !ok {
		t.Errorf("Expected Null value, got %T", value)
	}
}

func TestUnaryOpInvalidTypes(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	// -"hello" should fail (string negation)
	unaryOp := NewUnaryOp("-", NewStringLiteral("hello", pos), pos)
	_, err := unaryOp.InferType(nil, nil)
	if err == nil {
		t.Errorf("Expected type inference to fail for string negation")
	}
	
	// !"hello" should fail (string logical NOT)
	unaryOp2 := NewUnaryOp("!", NewStringLiteral("hello", pos), pos)
	_, err = unaryOp2.InferType(nil, nil)
	if err == nil {
		t.Errorf("Expected type inference to fail for string logical NOT")
	}
}

func TestUnaryOpUnknownOperator(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	// ~5 (unknown operator)
	unaryOp := NewUnaryOp("~", NewIntLiteral(5, pos), pos)
	_, err := unaryOp.InferType(nil, nil)
	if err == nil {
		t.Errorf("Expected type inference to fail for unknown operator")
	}
}

func TestUnaryOpChildren(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	operand := NewIntLiteral(5, pos)
	unaryOp := NewUnaryOp("-", operand, pos)
	
	children := unaryOp.Children()
	if len(children) != 1 {
		t.Errorf("Expected 1 child, got %d", len(children))
	}
	if children[0] != operand {
		t.Errorf("Child doesn't match expected operand")
	}
}

func TestUnaryOpString(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	operand := NewIntLiteral(5, pos)
	unaryOp := NewUnaryOp("-", operand, pos)
	
	expected := "-5"
	if unaryOp.String() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, unaryOp.String())
	}
}

// Test complex expressions

func TestComplexArithmeticExpression(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	// (5 + 3) * 2 - 1
	add := NewBinaryOp(NewIntLiteral(5, pos), "+", NewIntLiteral(3, pos), pos)
	mul := NewBinaryOp(add, "*", NewIntLiteral(2, pos), pos)
	sub := NewBinaryOp(mul, "-", NewIntLiteral(1, pos), pos)
	
	value, err := sub.Eval(nil, nil)
	if err != nil {
		t.Errorf("Complex expression evaluation failed: %v", err)
	}
	intValue, ok := value.(*values.IntValue)
	if !ok {
		t.Errorf("Expected IntValue, got %T", value)
	}
	if intValue.Value().(int64) != 15 {
		t.Errorf("Expected 15, got %d", intValue.Value().(int64))
	}
}

func TestMixedTypeArithmetic(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	// 5 + 2.5 (int + float = float)
	binOp := NewBinaryOp(NewIntLiteral(5, pos), "+", NewFloatLiteral(2.5, pos), pos)
	
	// Test type inference
	inferredType, err := binOp.InferType(nil, nil)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}
	if inferredType.String() != "Float" {
		t.Errorf("Expected Float type, got %s", inferredType.String())
	}
	
	// Test evaluation
	value, err := binOp.Eval(nil, nil)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}
	floatValue, ok := value.(*values.FloatValue)
	if !ok {
		t.Errorf("Expected FloatValue, got %T", value)
	}
	if floatValue.Value().(float64) != 7.5 {
		t.Errorf("Expected 7.5, got %f", floatValue.Value().(float64))
	}
}

// Test error handling

func TestDivisionByZero(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	// 5 / 0
	binOp := NewBinaryOp(NewIntLiteral(5, pos), "/", NewIntLiteral(0, pos), pos)
	_, err := binOp.Eval(nil, nil)
	if err == nil {
		t.Errorf("Expected division by zero to fail")
	}
}

func TestModuloByZero(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	
	// 5 % 0
	binOp := NewBinaryOp(NewIntLiteral(5, pos), "%", NewIntLiteral(0, pos), pos)
	_, err := binOp.Eval(nil, nil)
	if err == nil {
		t.Errorf("Expected modulo by zero to fail")
	}
}

// Test type checking

func TestFunctionOperatorTypeCheck(t *testing.T) {
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	stdlib := newFunctionTestStdLib()
	
	tests := []struct {
		name         string
		expr         Expr
		expectedType types.Base
		shouldFail   bool
	}{
		{
			name:         "function result to correct type",
			expr:         NewApply("length", []Expr{NewStringLiteral("test", pos)}, pos),
			expectedType: types.NewInt(false),
			shouldFail:   false,
		},
		{
			name:         "binary op to correct type",
			expr:         NewBinaryOp(NewIntLiteral(5, pos), "+", NewIntLiteral(3, pos), pos),
			expectedType: types.NewInt(false),
			shouldFail:   false,
		},
		{
			name:         "unary op to correct type",
			expr:         NewUnaryOp("-", NewIntLiteral(5, pos), pos),
			expectedType: types.NewInt(false),
			shouldFail:   false,
		},
		{
			name:         "function result to wrong type",
			expr:         NewApply("length", []Expr{NewStringLiteral("test", pos)}, pos),
			expectedType: types.NewArray(types.NewString(false), false, false),
			shouldFail:   true,
		},
		{
			name:         "binary op to wrong type",
			expr:         NewBinaryOp(NewIntLiteral(5, pos), "+", NewIntLiteral(3, pos), pos),
			expectedType: types.NewBoolean(false),
			shouldFail:   true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.expr.TypeCheck(tt.expectedType, nil, stdlib)
			
			if tt.shouldFail && err == nil {
				t.Errorf("Expected type check to fail but it succeeded")
			}
			
			if !tt.shouldFail && err != nil {
				t.Errorf("Expected type check to succeed but got error: %v", err)
			}
		})
	}
}