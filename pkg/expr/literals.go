package expr

import (
	"fmt"
	"strconv"

	"github.com/bioflowy/flowy/pkg/env"
	"github.com/bioflowy/flowy/pkg/errors"
	"github.com/bioflowy/flowy/pkg/types"
	"github.com/bioflowy/flowy/pkg/values"
)

// BooleanLiteral represents a boolean literal expression
type BooleanLiteral struct {
	baseExpr
	Value bool
}

// NewBooleanLiteral creates a new boolean literal
func NewBooleanLiteral(value bool, pos errors.SourcePosition) *BooleanLiteral {
	return &BooleanLiteral{
		baseExpr: NewBaseExpr(pos),
		Value:    value,
	}
}

func (b *BooleanLiteral) InferType(typeEnv *env.Bindings[types.Base], stdlib StdLib) (types.Base, error) {
	return types.NewBoolean(false), nil
}

func (b *BooleanLiteral) Eval(valueEnv *env.Bindings[values.Base], stdlib StdLib) (values.Base, error) {
	return values.NewBoolean(b.Value, false), nil
}

func (b *BooleanLiteral) TypeCheck(expectedType types.Base, typeEnv *env.Bindings[types.Base], stdlib StdLib) error {
	boolType := types.NewBoolean(false)
	helper := TypeCheckHelper{}
	return helper.CheckCoercion(boolType, expectedType, b.pos)
}

func (b *BooleanLiteral) Literal() (values.Base, bool) {
	return values.NewBoolean(b.Value, false), true
}

func (b *BooleanLiteral) String() string {
	return strconv.FormatBool(b.Value)
}

// IntLiteral represents an integer literal expression
type IntLiteral struct {
	baseExpr
	Value int64
}

// NewIntLiteral creates a new integer literal
func NewIntLiteral(value int64, pos errors.SourcePosition) *IntLiteral {
	return &IntLiteral{
		baseExpr: NewBaseExpr(pos),
		Value:    value,
	}
}

func (i *IntLiteral) InferType(typeEnv *env.Bindings[types.Base], stdlib StdLib) (types.Base, error) {
	return types.NewInt(false), nil
}

func (i *IntLiteral) Eval(valueEnv *env.Bindings[values.Base], stdlib StdLib) (values.Base, error) {
	return values.NewInt(i.Value, false), nil
}

func (i *IntLiteral) TypeCheck(expectedType types.Base, typeEnv *env.Bindings[types.Base], stdlib StdLib) error {
	intType := types.NewInt(false)
	helper := TypeCheckHelper{}
	return helper.CheckCoercion(intType, expectedType, i.pos)
}

func (i *IntLiteral) Literal() (values.Base, bool) {
	return values.NewInt(i.Value, false), true
}

func (i *IntLiteral) String() string {
	return strconv.FormatInt(i.Value, 10)
}

// FloatLiteral represents a floating-point literal expression
type FloatLiteral struct {
	baseExpr
	Value float64
}

// NewFloatLiteral creates a new float literal
func NewFloatLiteral(value float64, pos errors.SourcePosition) *FloatLiteral {
	return &FloatLiteral{
		baseExpr: NewBaseExpr(pos),
		Value:    value,
	}
}

func (f *FloatLiteral) InferType(typeEnv *env.Bindings[types.Base], stdlib StdLib) (types.Base, error) {
	return types.NewFloat(false), nil
}

func (f *FloatLiteral) Eval(valueEnv *env.Bindings[values.Base], stdlib StdLib) (values.Base, error) {
	return values.NewFloat(f.Value, false), nil
}

func (f *FloatLiteral) TypeCheck(expectedType types.Base, typeEnv *env.Bindings[types.Base], stdlib StdLib) error {
	floatType := types.NewFloat(false)
	helper := TypeCheckHelper{}
	return helper.CheckCoercion(floatType, expectedType, f.pos)
}

func (f *FloatLiteral) Literal() (values.Base, bool) {
	return values.NewFloat(f.Value, false), true
}

func (f *FloatLiteral) String() string {
	return strconv.FormatFloat(f.Value, 'f', -1, 64)
}

// StringLiteral represents a string literal expression
type StringLiteral struct {
	baseExpr
	Value         string
	Interpolation []Expr // For ${expr} interpolation within strings
}

// NewStringLiteral creates a new string literal
func NewStringLiteral(value string, pos errors.SourcePosition) *StringLiteral {
	return &StringLiteral{
		baseExpr:      NewBaseExpr(pos),
		Value:         value,
		Interpolation: nil,
	}
}

// NewInterpolatedString creates a new string literal with interpolation
func NewInterpolatedString(value string, interpolation []Expr, pos errors.SourcePosition) *StringLiteral {
	return &StringLiteral{
		baseExpr:      NewBaseExpr(pos),
		Value:         value,
		Interpolation: interpolation,
	}
}

func (s *StringLiteral) InferType(typeEnv *env.Bindings[types.Base], stdlib StdLib) (types.Base, error) {
	// Type check interpolated expressions if present
	if s.Interpolation != nil {
		for _, expr := range s.Interpolation {
			exprType, err := expr.InferType(typeEnv, stdlib)
			if err != nil {
				return nil, err
			}
			// All interpolated expressions should be coercible to String
			if err := exprType.Check(types.NewString(false), true); err != nil {
				return nil, errors.NewInvalidType(nil, fmt.Sprintf("type mismatch: expected %s, got %s", "String", exprType.String()))
			}
		}
	}
	return types.NewString(false), nil
}

func (s *StringLiteral) Eval(valueEnv *env.Bindings[values.Base], stdlib StdLib) (values.Base, error) {
	if s.Interpolation == nil {
		// Simple string literal
		return values.NewString(s.Value, false), nil
	}

	// String interpolation - evaluate embedded expressions
	result := s.Value
	for _, expr := range s.Interpolation {
		val, err := expr.Eval(valueEnv, stdlib)
		if err != nil {
			return nil, err
		}

		// Coerce to string
		stringVal, err := val.Coerce(types.NewString(false))
		if err != nil {
			return nil, errors.NewEvalError(nil, "cannot convert expression to string: "+err.Error())
		}

		// Replace placeholder with actual value
		strValue := stringVal.(*values.StringValue)
		// In a real implementation, we'd need to track placeholder positions
		// For now, we'll just append - this would need more sophisticated string building
		result += strValue.Value().(string)
	}

	return values.NewString(result, false), nil
}

func (s *StringLiteral) TypeCheck(expectedType types.Base, typeEnv *env.Bindings[types.Base], stdlib StdLib) error {
	// First ensure this string can coerce to expected type
	stringType := types.NewString(false)
	helper := TypeCheckHelper{}
	if err := helper.CheckCoercion(stringType, expectedType, s.pos); err != nil {
		return err
	}

	// Type check interpolated expressions
	if s.Interpolation != nil {
		for _, expr := range s.Interpolation {
			if err := expr.TypeCheck(types.NewString(false), typeEnv, stdlib); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *StringLiteral) Literal() (values.Base, bool) {
	if s.Interpolation != nil {
		return nil, false // Not a literal if it has interpolation
	}
	return values.NewString(s.Value, false), true
}

func (s *StringLiteral) Children() []Expr {
	if s.Interpolation == nil {
		return []Expr{}
	}
	return s.Interpolation
}

func (s *StringLiteral) String() string {
	if s.Interpolation == nil {
		return fmt.Sprintf(`"%s"`, s.Value)
	}
	return fmt.Sprintf(`"%s" (with interpolation)`, s.Value)
}

// NullLiteral represents a null/None literal expression
type NullLiteral struct {
	baseExpr
}

// NewNullLiteral creates a new null literal
func NewNullLiteral(pos errors.SourcePosition) *NullLiteral {
	return &NullLiteral{
		baseExpr: NewBaseExpr(pos),
	}
}

func (n *NullLiteral) InferType(typeEnv *env.Bindings[types.Base], stdlib StdLib) (types.Base, error) {
	return types.NewAny(true, true), nil // None literal - optional Any
}

func (n *NullLiteral) Eval(valueEnv *env.Bindings[values.Base], stdlib StdLib) (values.Base, error) {
	return values.NewNull(types.NewAny(true, true)), nil
}

func (n *NullLiteral) TypeCheck(expectedType types.Base, typeEnv *env.Bindings[types.Base], stdlib StdLib) error {
	if !expectedType.Optional() {
		return errors.NewInvalidType(nil, fmt.Sprintf("type mismatch: expected %s, got %s", expectedType.String(), "None"))
	}
	return nil
}

func (n *NullLiteral) Literal() (values.Base, bool) {
	return values.NewNull(types.NewAny(true, true)), true
}

func (n *NullLiteral) String() string {
	return "None"
}