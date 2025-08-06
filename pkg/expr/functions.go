package expr

import (
	"fmt"
	"strings"

	"github.com/bioflowy/flowy/pkg/env"
	"github.com/bioflowy/flowy/pkg/errors"
	"github.com/bioflowy/flowy/pkg/types"
	"github.com/bioflowy/flowy/pkg/values"
)

// Apply represents a function call expression func(arg1, arg2, ...)
type Apply struct {
	baseExpr
	Function string
	Args     []Expr
}

// NewApply creates a new function call expression
func NewApply(function string, args []Expr, pos errors.SourcePosition) *Apply {
	return &Apply{
		baseExpr: NewBaseExpr(pos),
		Function: function,
		Args:     args,
	}
}

func (a *Apply) InferType(typeEnv *env.Bindings[types.Base], stdlib StdLib) (types.Base, error) {
	if stdlib == nil {
		return nil, errors.NewUnknownIdentifier(nil, a.Function)
	}

	// Check if function exists
	if !stdlib.HasFunction(a.Function) {
		return nil, errors.NewUnknownIdentifier(nil, a.Function)
	}

	// Get function metadata
	fn, err := stdlib.GetFunction(a.Function)
	if err != nil {
		return nil, err
	}

	// Check arity
	helper := TypeCheckHelper{}
	if err := helper.CheckArity(a.Function, len(fn.ParamTypes), len(a.Args), fn.Variadic, a.pos); err != nil {
		return nil, err
	}

	// Type check arguments
	for i, arg := range a.Args {
		var expectedType types.Base = types.NewAny(false, false)
		if i < len(fn.ParamTypes) {
			expectedType = fn.ParamTypes[i]
		} else if fn.Variadic && len(fn.ParamTypes) > 0 {
			// For variadic functions, use the last parameter type for remaining args
			expectedType = fn.ParamTypes[len(fn.ParamTypes)-1]
		}

		argType, err := arg.InferType(typeEnv, stdlib)
		if err != nil {
			return nil, err
		}

		if err := argType.Check(expectedType, true); err != nil {
			return nil, errors.NewInvalidType(nil, fmt.Sprintf("type mismatch: expected %s, got %s", expectedType.String(), argType.String()))
		}
	}

	return fn.ReturnType, nil
}

func (a *Apply) Eval(valueEnv *env.Bindings[values.Base], stdlib StdLib) (values.Base, error) {
	if stdlib == nil {
		return nil, errors.NewUnknownIdentifier(nil, a.Function)
	}

	// Evaluate arguments
	var args []values.Base
	for _, arg := range a.Args {
		val, err := arg.Eval(valueEnv, stdlib)
		if err != nil {
			return nil, err
		}
		args = append(args, val)
	}

	// Call function through stdlib
	return stdlib.CallFunction(a.Function, args, a.pos)
}

func (a *Apply) TypeCheck(expectedType types.Base, typeEnv *env.Bindings[types.Base], stdlib StdLib) error {
	resultType, err := a.InferType(typeEnv, stdlib)
	if err != nil {
		return err
	}

	helper := TypeCheckHelper{}
	if err := helper.CheckCoercion(resultType, expectedType, a.pos); err != nil {
		return err
	}

	// Type check all arguments (already done in InferType, but we need to ensure it's called)
	_, err = a.InferType(typeEnv, stdlib)
	return err
}

func (a *Apply) Children() []Expr {
	return a.Args
}

func (a *Apply) String() string {
	var args []string
	for _, arg := range a.Args {
		args = append(args, arg.String())
	}
	return fmt.Sprintf("%s(%s)", a.Function, strings.Join(args, ", "))
}

// BinaryOp represents a binary operator expression (left op right)
type BinaryOp struct {
	baseExpr
	Left     Expr
	Operator string
	Right    Expr
}

// NewBinaryOp creates a new binary operator expression
func NewBinaryOp(left Expr, operator string, right Expr, pos errors.SourcePosition) *BinaryOp {
	return &BinaryOp{
		baseExpr: NewBaseExpr(pos),
		Left:     left,
		Operator: operator,
		Right:    right,
	}
}

func (b *BinaryOp) InferType(typeEnv *env.Bindings[types.Base], stdlib StdLib) (types.Base, error) {
	leftType, err := b.Left.InferType(typeEnv, stdlib)
	if err != nil {
		return nil, err
	}

	rightType, err := b.Right.InferType(typeEnv, stdlib)
	if err != nil {
		return nil, err
	}

	// Handle different operator categories
	switch b.Operator {
	case "+", "-", "*", "/", "%":
		// Arithmetic operators
		return b.inferArithmeticType(leftType, rightType)
	case "==", "!=":
		// Equality operators - can compare any types
		optional := leftType.Optional() || rightType.Optional()
		return types.NewBoolean(optional), nil
	case "<", "<=", ">", ">=":
		// Comparison operators - require comparable types
		return b.inferComparisonType(leftType, rightType)
	default:
		// Try stdlib operators if available
		if stdlib != nil && stdlib.HasOperator(b.Operator) {
			// For now, assume operators return their input types or Boolean
			// This would need more sophisticated handling in a real implementation
			if strings.Contains(b.Operator, "=") || strings.Contains(b.Operator, "<") || strings.Contains(b.Operator, ">") {
				optional := leftType.Optional() || rightType.Optional()
				return types.NewBoolean(optional), nil
			}
			// Default to unifying the operand types
			helper := InferTypeHelper{}
			return helper.UnifyTypes([]types.Base{leftType, rightType}, b.pos)
		}
		return nil, errors.NewEvalError(nil, "unknown binary operator: "+b.Operator)
	}
}

func (b *BinaryOp) inferArithmeticType(leftType, rightType types.Base) (types.Base, error) {
	// Check if operands are numeric
	leftNumeric := b.isNumericType(leftType)
	rightNumeric := b.isNumericType(rightType)

	if !leftNumeric {
		return nil, errors.NewInvalidType(nil, fmt.Sprintf("type mismatch: expected %s, got %s", "numeric", leftType.String()))
	}
	if !rightNumeric {
		return nil, errors.NewInvalidType(nil, fmt.Sprintf("type mismatch: expected %s, got %s", "numeric", rightType.String()))
	}

	// String concatenation for +
	if b.Operator == "+" {
		leftString := b.isStringType(leftType)
		rightString := b.isStringType(rightType)
		if leftString || rightString {
			optional := leftType.Optional() || rightType.Optional()
			return types.NewString(optional), nil
		}
	}

	// Numeric operations - return Float if either operand is Float, otherwise Int
	optional := leftType.Optional() || rightType.Optional()
	if b.isFloatType(leftType) || b.isFloatType(rightType) {
		return types.NewFloat(optional), nil
	}
	return types.NewInt(optional), nil
}

func (b *BinaryOp) inferComparisonType(leftType, rightType types.Base) (types.Base, error) {
	// Check if operands are comparable
	if !b.isComparableType(leftType) {
		return nil, errors.NewInvalidType(nil, fmt.Sprintf("type mismatch: expected %s, got %s", "comparable", leftType.String()))
	}
	if !b.isComparableType(rightType) {
		return nil, errors.NewInvalidType(nil, fmt.Sprintf("type mismatch: expected %s, got %s", "comparable", rightType.String()))
	}

	// Result is always Boolean
	optional := leftType.Optional() || rightType.Optional()
	return types.NewBoolean(optional), nil
}

func (b *BinaryOp) isNumericType(t types.Base) bool {
	switch t.(type) {
	case *types.IntType, *types.FloatType:
		return true
	default:
		return false
	}
}

func (b *BinaryOp) isStringType(t types.Base) bool {
	_, ok := t.(*types.StringType)
	return ok
}

func (b *BinaryOp) isFloatType(t types.Base) bool {
	_, ok := t.(*types.FloatType)
	return ok
}

func (b *BinaryOp) isComparableType(t types.Base) bool {
	switch t.(type) {
	case *types.IntType, *types.FloatType, *types.StringType, *types.BooleanType:
		return true
	default:
		return false
	}
}

func (b *BinaryOp) Eval(valueEnv *env.Bindings[values.Base], stdlib StdLib) (values.Base, error) {
	leftValue, err := b.Left.Eval(valueEnv, stdlib)
	if err != nil {
		return nil, err
	}

	rightValue, err := b.Right.Eval(valueEnv, stdlib)
	if err != nil {
		return nil, err
	}

	// Handle null propagation
	if _, isNull := leftValue.(*values.Null); isNull {
		return leftValue, nil
	}
	if _, isNull := rightValue.(*values.Null); isNull {
		return rightValue, nil
	}

	// Handle different operator categories
	switch b.Operator {
	case "+":
		return b.evalAddition(leftValue, rightValue)
	case "-":
		return b.evalArithmetic(leftValue, rightValue, b.subtractNumbers)
	case "*":
		return b.evalArithmetic(leftValue, rightValue, b.multiplyNumbers)
	case "/":
		return b.evalArithmetic(leftValue, rightValue, b.divideNumbers)
	case "%":
		return b.evalArithmetic(leftValue, rightValue, b.moduloNumbers)
	case "==":
		return b.evalEquality(leftValue, rightValue, true)
	case "!=":
		return b.evalEquality(leftValue, rightValue, false)
	case "<", "<=", ">", ">=":
		return b.evalComparison(leftValue, rightValue)
	default:
		// Try stdlib operators
		if stdlib != nil && stdlib.HasOperator(b.Operator) {
			return stdlib.CallOperator(b.Operator, []values.Base{leftValue, rightValue}, b.pos)
		}
		return nil, errors.NewEvalError(nil, "unknown binary operator: "+b.Operator)
	}
}

func (b *BinaryOp) evalAddition(left, right values.Base) (values.Base, error) {
	// String concatenation
	if _, isString := left.(*values.StringValue); isString {
		return b.evalStringConcat(left, right)
	}
	if _, isString := right.(*values.StringValue); isString {
		return b.evalStringConcat(left, right)
	}

	// Numeric addition
	return b.evalArithmetic(left, right, b.addNumbers)
}

func (b *BinaryOp) evalStringConcat(left, right values.Base) (values.Base, error) {
	leftStr, err := left.Coerce(types.NewString(false))
	if err != nil {
		return nil, errors.NewEvalError(nil, "cannot convert to string: "+err.Error())
	}

	rightStr, err := right.Coerce(types.NewString(false))
	if err != nil {
		return nil, errors.NewEvalError(nil, "cannot convert to string: "+err.Error())
	}

	leftVal := leftStr.(*values.StringValue).Value().(string)
	rightVal := rightStr.(*values.StringValue).Value().(string)

	return values.NewString(leftVal+rightVal, false), nil
}

func (b *BinaryOp) evalArithmetic(left, right values.Base, op func(interface{}, interface{}) (interface{}, error)) (values.Base, error) {
	// Convert to numeric types
	leftFloat, leftErr := left.Coerce(types.NewFloat(false))
	rightFloat, rightErr := right.Coerce(types.NewFloat(false))

	// If both can be converted to int, prefer int
	leftInt, leftIntErr := left.Coerce(types.NewInt(false))
	rightInt, rightIntErr := right.Coerce(types.NewInt(false))

	if leftIntErr == nil && rightIntErr == nil {
		// Both are integers
		leftVal := leftInt.(*values.IntValue).Value().(int64)
		rightVal := rightInt.(*values.IntValue).Value().(int64)
		result, err := op(leftVal, rightVal)
		if err != nil {
			return nil, errors.NewEvalError(nil, err.Error())
		}
		if intResult, ok := result.(int64); ok {
			return values.NewInt(intResult, false), nil
		}
		if floatResult, ok := result.(float64); ok {
			return values.NewFloat(floatResult, false), nil
		}
	}

	// Fall back to float arithmetic
	if leftErr != nil {
		return nil, errors.NewEvalError(nil, "cannot convert to number: "+leftErr.Error())
	}
	if rightErr != nil {
		return nil, errors.NewEvalError(nil, "cannot convert to number: "+rightErr.Error())
	}

	leftVal := leftFloat.(*values.FloatValue).Value().(float64)
	rightVal := rightFloat.(*values.FloatValue).Value().(float64)
	result, err := op(leftVal, rightVal)
	if err != nil {
		return nil, errors.NewEvalError(nil, err.Error())
	}

	if floatResult, ok := result.(float64); ok {
		return values.NewFloat(floatResult, false), nil
	}

	return nil, errors.NewEvalError(nil, "invalid arithmetic result")
}

func (b *BinaryOp) addNumbers(left, right interface{}) (interface{}, error) {
	switch l := left.(type) {
	case int64:
		if r, ok := right.(int64); ok {
			return l + r, nil
		}
	case float64:
		if r, ok := right.(float64); ok {
			return l + r, nil
		}
	}
	return nil, fmt.Errorf("invalid addition operands")
}

func (b *BinaryOp) subtractNumbers(left, right interface{}) (interface{}, error) {
	switch l := left.(type) {
	case int64:
		if r, ok := right.(int64); ok {
			return l - r, nil
		}
	case float64:
		if r, ok := right.(float64); ok {
			return l - r, nil
		}
	}
	return nil, fmt.Errorf("invalid subtraction operands")
}

func (b *BinaryOp) multiplyNumbers(left, right interface{}) (interface{}, error) {
	switch l := left.(type) {
	case int64:
		if r, ok := right.(int64); ok {
			return l * r, nil
		}
	case float64:
		if r, ok := right.(float64); ok {
			return l * r, nil
		}
	}
	return nil, fmt.Errorf("invalid multiplication operands")
}

func (b *BinaryOp) divideNumbers(left, right interface{}) (interface{}, error) {
	switch l := left.(type) {
	case int64:
		if r, ok := right.(int64); ok {
			if r == 0 {
				return nil, fmt.Errorf("division by zero")
			}
			// Integer division returns float to handle fractions
			return float64(l) / float64(r), nil
		}
	case float64:
		if r, ok := right.(float64); ok {
			if r == 0.0 {
				return nil, fmt.Errorf("division by zero")
			}
			return l / r, nil
		}
	}
	return nil, fmt.Errorf("invalid division operands")
}

func (b *BinaryOp) moduloNumbers(left, right interface{}) (interface{}, error) {
	// Modulo only works with integers
	switch l := left.(type) {
	case int64:
		if r, ok := right.(int64); ok {
			if r == 0 {
				return nil, fmt.Errorf("modulo by zero")
			}
			return l % r, nil
		}
	}
	return nil, fmt.Errorf("modulo requires integer operands")
}

func (b *BinaryOp) evalEquality(left, right values.Base, equal bool) (values.Base, error) {
	// Use the values' own equality comparison
	isEqual := b.valuesEqual(left, right)
	result := isEqual
	if !equal {
		result = !result
	}
	return values.NewBoolean(result, false), nil
}

func (b *BinaryOp) valuesEqual(left, right values.Base) bool {
	// Simple equality check - in a real implementation this would be more sophisticated
	return left.String() == right.String()
}

func (b *BinaryOp) evalComparison(left, right values.Base) (values.Base, error) {
	// Convert to comparable types and compare
	result, err := b.compareValues(left, right)
	if err != nil {
		return nil, errors.NewEvalError(nil, err.Error())
	}

	var boolResult bool
	switch b.Operator {
	case "<":
		boolResult = result < 0
	case "<=":
		boolResult = result <= 0
	case ">":
		boolResult = result > 0
	case ">=":
		boolResult = result >= 0
	}

	return values.NewBoolean(boolResult, false), nil
}

func (b *BinaryOp) compareValues(left, right values.Base) (int, error) {
	// Try numeric comparison first
	leftFloat, leftErr := left.Coerce(types.NewFloat(false))
	rightFloat, rightErr := right.Coerce(types.NewFloat(false))

	if leftErr == nil && rightErr == nil {
		leftVal := leftFloat.(*values.FloatValue).Value().(float64)
		rightVal := rightFloat.(*values.FloatValue).Value().(float64)
		if leftVal < rightVal {
			return -1, nil
		} else if leftVal > rightVal {
			return 1, nil
		}
		return 0, nil
	}

	// Try string comparison
	leftStr, leftErr := left.Coerce(types.NewString(false))
	rightStr, rightErr := right.Coerce(types.NewString(false))

	if leftErr == nil && rightErr == nil {
		leftVal := leftStr.(*values.StringValue).Value().(string)
		rightVal := rightStr.(*values.StringValue).Value().(string)
		return strings.Compare(leftVal, rightVal), nil
	}

	return 0, fmt.Errorf("incomparable values")
}

func (b *BinaryOp) TypeCheck(expectedType types.Base, typeEnv *env.Bindings[types.Base], stdlib StdLib) error {
	opType, err := b.InferType(typeEnv, stdlib)
	if err != nil {
		return err
	}

	helper := TypeCheckHelper{}
	if err := helper.CheckCoercion(opType, expectedType, b.pos); err != nil {
		return err
	}

	// Type checking is already done in InferType
	return nil
}

func (b *BinaryOp) Children() []Expr {
	return []Expr{b.Left, b.Right}
}

func (b *BinaryOp) String() string {
	return fmt.Sprintf("(%s %s %s)", b.Left.String(), b.Operator, b.Right.String())
}

// UnaryOp represents a unary operator expression (op expr)
type UnaryOp struct {
	baseExpr
	Operator string
	Operand  Expr
}

// NewUnaryOp creates a new unary operator expression
func NewUnaryOp(operator string, operand Expr, pos errors.SourcePosition) *UnaryOp {
	return &UnaryOp{
		baseExpr: NewBaseExpr(pos),
		Operator: operator,
		Operand:  operand,
	}
}

func (u *UnaryOp) InferType(typeEnv *env.Bindings[types.Base], stdlib StdLib) (types.Base, error) {
	operandType, err := u.Operand.InferType(typeEnv, stdlib)
	if err != nil {
		return nil, err
	}

	switch u.Operator {
	case "-", "+":
		// Numeric unary operators
		if u.isNumericType(operandType) {
			return operandType, nil
		}
		return nil, errors.NewInvalidType(nil, fmt.Sprintf("type mismatch: expected %s, got %s", "numeric", operandType.String()))
	case "!":
		// Logical NOT - already handled in control.go as LogicalNot
		if err := operandType.Check(types.NewBoolean(false), true); err != nil {
			return nil, errors.NewInvalidType(nil, fmt.Sprintf("type mismatch: expected %s, got %s", "Boolean", operandType.String()))
		}
		return types.NewBoolean(operandType.Optional()), nil
	default:
		return nil, errors.NewEvalError(nil, "unknown unary operator: "+u.Operator)
	}
}

func (u *UnaryOp) isNumericType(t types.Base) bool {
	switch t.(type) {
	case *types.IntType, *types.FloatType:
		return true
	default:
		return false
	}
}

func (u *UnaryOp) Eval(valueEnv *env.Bindings[values.Base], stdlib StdLib) (values.Base, error) {
	operandValue, err := u.Operand.Eval(valueEnv, stdlib)
	if err != nil {
		return nil, err
	}

	// Handle null
	if _, isNull := operandValue.(*values.Null); isNull {
		return operandValue, nil
	}

	switch u.Operator {
	case "+":
		// Unary plus - just return the operand if it's numeric
		if u.isNumericValue(operandValue) {
			return operandValue, nil
		}
		return nil, errors.NewEvalError(nil, "unary + requires numeric value")
	case "-":
		// Unary minus
		return u.negateValue(operandValue)
	case "!":
		// Logical NOT
		boolValue, err := operandValue.Coerce(types.NewBoolean(false))
		if err != nil {
			return nil, errors.NewEvalError(nil, "logical NOT requires Boolean: "+err.Error())
		}
		operandBool := boolValue.(*values.BooleanValue).Value().(bool)
		return values.NewBoolean(!operandBool, false), nil
	default:
		return nil, errors.NewEvalError(nil, "unknown unary operator: "+u.Operator)
	}
}

func (u *UnaryOp) isNumericValue(v values.Base) bool {
	switch v.(type) {
	case *values.IntValue, *values.FloatValue:
		return true
	default:
		return false
	}
}

func (u *UnaryOp) negateValue(v values.Base) (values.Base, error) {
	switch val := v.(type) {
	case *values.IntValue:
		intVal := val.Value().(int64)
		return values.NewInt(-intVal, false), nil
	case *values.FloatValue:
		floatVal := val.Value().(float64)
		return values.NewFloat(-floatVal, false), nil
	default:
		return nil, errors.NewEvalError(nil, "unary - requires numeric value")
	}
}

func (u *UnaryOp) TypeCheck(expectedType types.Base, typeEnv *env.Bindings[types.Base], stdlib StdLib) error {
	opType, err := u.InferType(typeEnv, stdlib)
	if err != nil {
		return err
	}

	helper := TypeCheckHelper{}
	return helper.CheckCoercion(opType, expectedType, u.pos)
}

func (u *UnaryOp) Children() []Expr {
	return []Expr{u.Operand}
}

func (u *UnaryOp) String() string {
	return fmt.Sprintf("%s%s", u.Operator, u.Operand.String())
}