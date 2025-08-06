package expr

import (
	"fmt"

	"github.com/bioflowy/flowy/pkg/env"
	"github.com/bioflowy/flowy/pkg/errors"
	"github.com/bioflowy/flowy/pkg/types"
	"github.com/bioflowy/flowy/pkg/values"
)

// IfThenElse represents a conditional expression (if condition then expr1 else expr2)
type IfThenElse struct {
	baseExpr
	Condition Expr
	ThenExpr  Expr
	ElseExpr  Expr
}

// NewIfThenElse creates a new conditional expression
func NewIfThenElse(condition, thenExpr, elseExpr Expr, pos errors.SourcePosition) *IfThenElse {
	return &IfThenElse{
		baseExpr:  NewBaseExpr(pos),
		Condition: condition,
		ThenExpr:  thenExpr,
		ElseExpr:  elseExpr,
	}
}

func (i *IfThenElse) InferType(typeEnv *env.Bindings[types.Base], stdlib StdLib) (types.Base, error) {
	// Check condition type - must be Boolean
	conditionType, err := i.Condition.InferType(typeEnv, stdlib)
	if err != nil {
		return nil, err
	}

	if err := conditionType.Check(types.NewBoolean(false), true); err != nil {
		return nil, &errors.InvalidType{
			ValidationError: errors.NewValidationErrorFromPos(i.pos, fmt.Sprintf("type mismatch: expected %s, got %s", "Boolean", conditionType.String())),
		}
	}

	// Infer types of both branches
	thenType, err := i.ThenExpr.InferType(typeEnv, stdlib)
	if err != nil {
		return nil, err
	}

	elseType, err := i.ElseExpr.InferType(typeEnv, stdlib)
	if err != nil {
		return nil, err
	}

	// Unify the types of both branches
	helper := InferTypeHelper{}
	resultType, err := helper.UnifyTypes([]types.Base{thenType, elseType}, i.pos)
	if err != nil {
		return nil, err
	}

	return resultType, nil
}

func (i *IfThenElse) Eval(valueEnv *env.Bindings[values.Base], stdlib StdLib) (values.Base, error) {
	// Evaluate condition
	conditionValue, err := i.Condition.Eval(valueEnv, stdlib)
	if err != nil {
		return nil, err
	}

	// Coerce condition to Boolean
	boolValue, err := conditionValue.Coerce(types.NewBoolean(false))
	if err != nil {
		return nil, errors.NewEvalErrorFromPos(i.pos, "condition must be Boolean: "+err.Error())
	}

	boolVal := boolValue.(*values.BooleanValue)
	condition := boolVal.Value().(bool)

	// Evaluate the appropriate branch
	if condition {
		return i.ThenExpr.Eval(valueEnv, stdlib)
	} else {
		return i.ElseExpr.Eval(valueEnv, stdlib)
	}
}

func (i *IfThenElse) TypeCheck(expectedType types.Base, typeEnv *env.Bindings[types.Base], stdlib StdLib) error {
	// First check that the overall type matches
	ifType, err := i.InferType(typeEnv, stdlib)
	if err != nil {
		return err
	}

	helper := TypeCheckHelper{}
	if err := helper.CheckCoercion(ifType, expectedType, i.pos); err != nil {
		return err
	}

	// Type check the condition
	if err := i.Condition.TypeCheck(types.NewBoolean(false), typeEnv, stdlib); err != nil {
		return err
	}

	// Type check both branches against the expected result type
	if err := i.ThenExpr.TypeCheck(expectedType, typeEnv, stdlib); err != nil {
		return err
	}

	if err := i.ElseExpr.TypeCheck(expectedType, typeEnv, stdlib); err != nil {
		return err
	}

	return nil
}

func (i *IfThenElse) Children() []Expr {
	return []Expr{i.Condition, i.ThenExpr, i.ElseExpr}
}

func (i *IfThenElse) String() string {
	return fmt.Sprintf("if %s then %s else %s", i.Condition.String(), i.ThenExpr.String(), i.ElseExpr.String())
}

// LogicalAnd represents a logical AND expression (expr1 && expr2)
type LogicalAnd struct {
	baseExpr
	Left  Expr
	Right Expr
}

// NewLogicalAnd creates a new logical AND expression
func NewLogicalAnd(left, right Expr, pos errors.SourcePosition) *LogicalAnd {
	return &LogicalAnd{
		baseExpr: NewBaseExpr(pos),
		Left:     left,
		Right:    right,
	}
}

func (l *LogicalAnd) InferType(typeEnv *env.Bindings[types.Base], stdlib StdLib) (types.Base, error) {
	// Check left operand
	leftType, err := l.Left.InferType(typeEnv, stdlib)
	if err != nil {
		return nil, err
	}

	if err := leftType.Check(types.NewBoolean(false), true); err != nil {
		return nil, &errors.InvalidType{
			ValidationError: errors.NewValidationErrorFromPos(l.pos, fmt.Sprintf("type mismatch: expected %s, got %s", "Boolean", leftType.String())),
		}
	}

	// Check right operand
	rightType, err := l.Right.InferType(typeEnv, stdlib)
	if err != nil {
		return nil, err
	}

	if err := rightType.Check(types.NewBoolean(false), true); err != nil {
		return nil, &errors.InvalidType{
			ValidationError: errors.NewValidationErrorFromPos(l.pos, fmt.Sprintf("type mismatch: expected %s, got %s", "Boolean", rightType.String())),
		}
	}

	// Result is Boolean, optional if either operand is optional
	optional := leftType.Optional() || rightType.Optional()
	return types.NewBoolean(optional), nil
}

func (l *LogicalAnd) Eval(valueEnv *env.Bindings[values.Base], stdlib StdLib) (values.Base, error) {
	// Evaluate left operand
	leftValue, err := l.Left.Eval(valueEnv, stdlib)
	if err != nil {
		return nil, err
	}

	// Check for null
	if _, isNull := leftValue.(*values.Null); isNull {
		return leftValue, nil // Short-circuit with null
	}

	// Coerce to Boolean
	boolLeft, err := leftValue.Coerce(types.NewBoolean(false))
	if err != nil {
		return nil, errors.NewEvalErrorFromPos(l.pos, "logical AND requires Boolean operand: "+err.Error())
	}

	leftBool := boolLeft.(*values.BooleanValue).Value().(bool)
	if !leftBool {
		// Short-circuit: false && anything = false
		return values.NewBoolean(false, false), nil
	}

	// Evaluate right operand
	rightValue, err := l.Right.Eval(valueEnv, stdlib)
	if err != nil {
		return nil, err
	}

	// Check for null
	if _, isNull := rightValue.(*values.Null); isNull {
		return rightValue, nil
	}

	// Coerce to Boolean
	boolRight, err := rightValue.Coerce(types.NewBoolean(false))
	if err != nil {
		return nil, errors.NewEvalErrorFromPos(l.pos, "logical AND requires Boolean operand: "+err.Error())
	}

	rightBool := boolRight.(*values.BooleanValue).Value().(bool)
	return values.NewBoolean(rightBool, false), nil
}

func (l *LogicalAnd) TypeCheck(expectedType types.Base, typeEnv *env.Bindings[types.Base], stdlib StdLib) error {
	andType, err := l.InferType(typeEnv, stdlib)
	if err != nil {
		return err
	}

	helper := TypeCheckHelper{}
	if err := helper.CheckCoercion(andType, expectedType, l.pos); err != nil {
		return err
	}

	// Type check operands
	if err := l.Left.TypeCheck(types.NewBoolean(false), typeEnv, stdlib); err != nil {
		return err
	}

	if err := l.Right.TypeCheck(types.NewBoolean(false), typeEnv, stdlib); err != nil {
		return err
	}

	return nil
}

func (l *LogicalAnd) Children() []Expr {
	return []Expr{l.Left, l.Right}
}

func (l *LogicalAnd) String() string {
	return fmt.Sprintf("(%s && %s)", l.Left.String(), l.Right.String())
}

// LogicalOr represents a logical OR expression (expr1 || expr2)
type LogicalOr struct {
	baseExpr
	Left  Expr
	Right Expr
}

// NewLogicalOr creates a new logical OR expression
func NewLogicalOr(left, right Expr, pos errors.SourcePosition) *LogicalOr {
	return &LogicalOr{
		baseExpr: NewBaseExpr(pos),
		Left:     left,
		Right:    right,
	}
}

func (l *LogicalOr) InferType(typeEnv *env.Bindings[types.Base], stdlib StdLib) (types.Base, error) {
	// Check left operand
	leftType, err := l.Left.InferType(typeEnv, stdlib)
	if err != nil {
		return nil, err
	}

	if err := leftType.Check(types.NewBoolean(false), true); err != nil {
		return nil, &errors.InvalidType{
			ValidationError: errors.NewValidationErrorFromPos(l.pos, fmt.Sprintf("type mismatch: expected %s, got %s", "Boolean", leftType.String())),
		}
	}

	// Check right operand
	rightType, err := l.Right.InferType(typeEnv, stdlib)
	if err != nil {
		return nil, err
	}

	if err := rightType.Check(types.NewBoolean(false), true); err != nil {
		return nil, &errors.InvalidType{
			ValidationError: errors.NewValidationErrorFromPos(l.pos, fmt.Sprintf("type mismatch: expected %s, got %s", "Boolean", rightType.String())),
		}
	}

	// Result is Boolean, optional if either operand is optional
	optional := leftType.Optional() || rightType.Optional()
	return types.NewBoolean(optional), nil
}

func (l *LogicalOr) Eval(valueEnv *env.Bindings[values.Base], stdlib StdLib) (values.Base, error) {
	// Evaluate left operand
	leftValue, err := l.Left.Eval(valueEnv, stdlib)
	if err != nil {
		return nil, err
	}

	// Check for null
	if _, isNull := leftValue.(*values.Null); isNull {
		return leftValue, nil // Short-circuit with null
	}

	// Coerce to Boolean
	boolLeft, err := leftValue.Coerce(types.NewBoolean(false))
	if err != nil {
		return nil, errors.NewEvalErrorFromPos(l.pos, "logical OR requires Boolean operand: "+err.Error())
	}

	leftBool := boolLeft.(*values.BooleanValue).Value().(bool)
	if leftBool {
		// Short-circuit: true || anything = true
		return values.NewBoolean(true, false), nil
	}

	// Evaluate right operand
	rightValue, err := l.Right.Eval(valueEnv, stdlib)
	if err != nil {
		return nil, err
	}

	// Check for null
	if _, isNull := rightValue.(*values.Null); isNull {
		return rightValue, nil
	}

	// Coerce to Boolean
	boolRight, err := rightValue.Coerce(types.NewBoolean(false))
	if err != nil {
		return nil, errors.NewEvalErrorFromPos(l.pos, "logical OR requires Boolean operand: "+err.Error())
	}

	rightBool := boolRight.(*values.BooleanValue).Value().(bool)
	return values.NewBoolean(rightBool, false), nil
}

func (l *LogicalOr) TypeCheck(expectedType types.Base, typeEnv *env.Bindings[types.Base], stdlib StdLib) error {
	orType, err := l.InferType(typeEnv, stdlib)
	if err != nil {
		return err
	}

	helper := TypeCheckHelper{}
	if err := helper.CheckCoercion(orType, expectedType, l.pos); err != nil {
		return err
	}

	// Type check operands
	if err := l.Left.TypeCheck(types.NewBoolean(false), typeEnv, stdlib); err != nil {
		return err
	}

	if err := l.Right.TypeCheck(types.NewBoolean(false), typeEnv, stdlib); err != nil {
		return err
	}

	return nil
}

func (l *LogicalOr) Children() []Expr {
	return []Expr{l.Left, l.Right}
}

func (l *LogicalOr) String() string {
	return fmt.Sprintf("(%s || %s)", l.Left.String(), l.Right.String())
}

// LogicalNot represents a logical NOT expression (!expr)
type LogicalNot struct {
	baseExpr
	Operand Expr
}

// NewLogicalNot creates a new logical NOT expression
func NewLogicalNot(operand Expr, pos errors.SourcePosition) *LogicalNot {
	return &LogicalNot{
		baseExpr: NewBaseExpr(pos),
		Operand:  operand,
	}
}

func (l *LogicalNot) InferType(typeEnv *env.Bindings[types.Base], stdlib StdLib) (types.Base, error) {
	operandType, err := l.Operand.InferType(typeEnv, stdlib)
	if err != nil {
		return nil, err
	}

	if err := operandType.Check(types.NewBoolean(false), true); err != nil {
		return nil, &errors.InvalidType{
			ValidationError: errors.NewValidationErrorFromPos(l.pos, fmt.Sprintf("type mismatch: expected %s, got %s", "Boolean", operandType.String())),
		}
	}

	// Result is Boolean with same optionality as operand
	return types.NewBoolean(operandType.Optional()), nil
}

func (l *LogicalNot) Eval(valueEnv *env.Bindings[values.Base], stdlib StdLib) (values.Base, error) {
	operandValue, err := l.Operand.Eval(valueEnv, stdlib)
	if err != nil {
		return nil, err
	}

	// Check for null
	if _, isNull := operandValue.(*values.Null); isNull {
		return operandValue, nil
	}

	// Coerce to Boolean
	boolValue, err := operandValue.Coerce(types.NewBoolean(false))
	if err != nil {
		return nil, errors.NewEvalErrorFromPos(l.pos, "logical NOT requires Boolean operand: "+err.Error())
	}

	operandBool := boolValue.(*values.BooleanValue).Value().(bool)
	return values.NewBoolean(!operandBool, false), nil
}

func (l *LogicalNot) TypeCheck(expectedType types.Base, typeEnv *env.Bindings[types.Base], stdlib StdLib) error {
	notType, err := l.InferType(typeEnv, stdlib)
	if err != nil {
		return err
	}

	helper := TypeCheckHelper{}
	if err := helper.CheckCoercion(notType, expectedType, l.pos); err != nil {
		return err
	}

	// Type check operand
	return l.Operand.TypeCheck(types.NewBoolean(false), typeEnv, stdlib)
}

func (l *LogicalNot) Children() []Expr {
	return []Expr{l.Operand}
}

func (l *LogicalNot) String() string {
	return fmt.Sprintf("!%s", l.Operand.String())
}