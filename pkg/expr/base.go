// Package expr provides WDL expression AST nodes and evaluation
package expr

import (
	"fmt"
	
	"github.com/bioflowy/flowy/pkg/env"
	"github.com/bioflowy/flowy/pkg/errors"
	"github.com/bioflowy/flowy/pkg/types"
	"github.com/bioflowy/flowy/pkg/values"
)

// Expr represents a WDL expression AST node
type Expr interface {
	// InferType performs type inference for this expression
	InferType(typeEnv *env.Bindings[types.Base], stdlib StdLib) (types.Base, error)

	// Eval evaluates this expression to produce a value
	Eval(valueEnv *env.Bindings[values.Base], stdlib StdLib) (values.Base, error)

	// TypeCheck verifies this expression can produce the expected type
	TypeCheck(expectedType types.Base, typeEnv *env.Bindings[types.Base], stdlib StdLib) error

	// Literal returns the literal value if this is a constant expression
	Literal() (values.Base, bool)

	// Pos returns the source position of this expression
	Pos() errors.SourcePosition

	// Children returns child expressions for tree traversal
	Children() []Expr

	// String returns a string representation for debugging
	String() string
}

// StdLib defines the interface for WDL standard library functions
type StdLib interface {
	// HasFunction checks if a function with the given name exists
	HasFunction(name string) bool

	// GetFunction returns function metadata
	GetFunction(name string) (*Function, error)

	// CallFunction invokes a function with given arguments
	CallFunction(name string, args []values.Base, pos errors.SourcePosition) (values.Base, error)

	// HasOperator checks if a binary/unary operator exists
	HasOperator(op string) bool

	// CallOperator invokes an operator with given arguments
	CallOperator(op string, args []values.Base, pos errors.SourcePosition) (values.Base, error)
}

// Function represents metadata about a WDL function
type Function struct {
	Name       string
	ParamTypes []types.Base
	ReturnType types.Base
	Variadic   bool
}

// baseExpr provides common functionality for expression implementations
type baseExpr struct {
	pos errors.SourcePosition
}

// NewBaseExpr creates a new base expression
func NewBaseExpr(pos errors.SourcePosition) baseExpr {
	return baseExpr{pos: pos}
}

func (b baseExpr) Pos() errors.SourcePosition {
	return b.pos
}

func (b baseExpr) Literal() (values.Base, bool) {
	return nil, false
}

func (b baseExpr) Children() []Expr {
	return []Expr{}
}

// TypeCheckHelper provides utilities for type checking
type TypeCheckHelper struct{}

// CheckCoercion verifies that fromType can coerce to toType
func (h TypeCheckHelper) CheckCoercion(fromType, toType types.Base, pos errors.SourcePosition) error {
	if err := fromType.Check(toType, true); err != nil {
		return &errors.InvalidType{
			ValidationError: errors.NewValidationErrorFromPos(pos, fmt.Sprintf("type mismatch: expected %s, got %s", toType.String(), fromType.String())),
		}
	}
	return nil
}

// CheckArity verifies function call has correct number of arguments
func (h TypeCheckHelper) CheckArity(funcName string, expected, actual int, variadic bool, pos errors.SourcePosition) error {
	if variadic {
		if actual < expected {
			return &errors.WrongArity{
				ValidationError: errors.NewValidationErrorFromPos(pos, fmt.Sprintf("wrong number of arguments for %s: expected at least %d, got %d", funcName, expected, actual)),
			}
		}
	} else {
		if actual != expected {
			return &errors.WrongArity{
				ValidationError: errors.NewValidationErrorFromPos(pos, fmt.Sprintf("wrong number of arguments for %s: expected %d, got %d", funcName, expected, actual)),
			}
		}
	}
	return nil
}

// InferTypeHelper provides utilities for type inference
type InferTypeHelper struct{}

// UnifyTypes attempts to unify multiple types into a common type
func (h InferTypeHelper) UnifyTypes(typeList []types.Base, pos errors.SourcePosition) (types.Base, error) {
	if len(typeList) == 0 {
		return nil, &errors.InvalidType{
			ValidationError: errors.NewValidationErrorFromPos(pos, "cannot unify empty type list"),
		}
	}

	result := typeList[0]
	for _, t := range typeList[1:] {
		unified, err := types.Unify(result, t)
		if err != nil {
			return nil, &errors.InvalidType{
				ValidationError: errors.NewValidationErrorFromPos(pos, fmt.Sprintf("cannot unify types %s and %s", result.String(), t.String())),
			}
		}
		result = unified
	}
	return result, nil
}

// EvalHelper provides utilities for expression evaluation
type EvalHelper struct{}

// CoerceValue attempts to coerce a value to the target type
func (h EvalHelper) CoerceValue(value values.Base, targetType types.Base, pos errors.SourcePosition) (values.Base, error) {
	result, err := value.Coerce(targetType)
	if err != nil {
		return nil, errors.NewEvalErrorFromPos(pos, "type coercion failed: "+err.Error())
	}
	return result, nil
}

// CheckNonNull verifies that a value is not null when required
func (h EvalHelper) CheckNonNull(value values.Base, pos errors.SourcePosition) error {
	if _, isNull := value.(*values.Null); isNull {
		return errors.NewNullValueFromPos(pos)
	}
	return nil
}