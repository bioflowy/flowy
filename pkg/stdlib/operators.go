package stdlib

import (
	"fmt"

	"github.com/uehara/flowy/pkg/env"
	"github.com/uehara/flowy/pkg/errors"
	"github.com/uehara/flowy/pkg/expr"
	"github.com/uehara/flowy/pkg/types"
	"github.com/uehara/flowy/pkg/values"
)

// atOperator implements array/map access operator
type atOperator struct{}

func (op *atOperator) InferType(e *expr.Apply) (types.Base, error) {
	if len(e.Arguments()) != 2 {
		return nil, fmt.Errorf("array/map access requires exactly 2 arguments")
	}

	lhs := e.Arguments()[0]
	rhs := e.Arguments()[1]

	lhsType, err := lhs.InferType(env.NewBindings[types.Base]())
	if err != nil {
		return nil, err
	}

	rhsType, err := rhs.InferType(env.NewBindings[types.Base]())
	if err != nil {
		return nil, err
	}

	// Array access
	if arrType, ok := lhsType.(*types.ArrayType); ok {
		if !rhsType.CheckType(types.NewInt(false)) {
			return nil, &errors.StaticTypeMismatch{
				BaseError: errors.BaseError{Node: rhs},
				Expected:  types.NewInt(false),
				Actual:    rhsType,
				Message:   "Array index",
			}
		}
		return arrType.ItemType(), nil
	}

	// Map access
	if mapType, ok := lhsType.(*types.MapType); ok {
		keyType, valType := mapType.ItemTypes()
		if !rhsType.CheckType(keyType) {
			return nil, &errors.StaticTypeMismatch{
				BaseError: errors.BaseError{Node: rhs},
				Expected:  keyType,
				Actual:    rhsType,
				Message:   "Map key",
			}
		}
		return valType, nil
	}

	// Any type (e.g., from read_json)
	if anyType, ok := lhsType.(*types.AnyType); ok && !anyType.Optional() {
		return types.NewAny(false), nil
	}

	return nil, &errors.NotAnArray{
		BaseError: errors.BaseError{Node: lhs},
	}
}

func (op *atOperator) Call(e *expr.Apply, bindings env.Bindings[values.Base], stdlib *Base) (values.Base, error) {
	lhsVal, err := e.Arguments()[0].Eval(bindings, stdlib)
	if err != nil {
		return nil, err
	}

	rhsVal, err := e.Arguments()[1].Eval(bindings, stdlib)
	if err != nil {
		return nil, err
	}

	// Map access
	if mapVal, ok := lhsVal.(*values.MapValue); ok {
		for _, pair := range mapVal.Value() {
			if pair[0].Equals(rhsVal) {
				return pair[1], nil
			}
		}
		return nil, &errors.OutOfBounds{
			BaseError: errors.BaseError{
				Node:    e.Arguments()[1],
				Message: "Map key not found",
			},
		}
	}

	// Array access
	if arrVal, ok := lhsVal.(*values.ArrayValue); ok {
		indexVal, ok := rhsVal.(*values.IntValue)
		if !ok {
			return nil, fmt.Errorf("array index must be an integer")
		}
		index := int(indexVal.Value())
		if index < 0 || index >= len(arrVal.Value()) {
			return nil, &errors.OutOfBounds{
				BaseError: errors.BaseError{
					Node:    e.Arguments()[1],
					Message: "Array index out of bounds",
				},
			}
		}
		return arrVal.Value()[index], nil
	}

	// Struct access (for read_json)
	if structVal, ok := lhsVal.(*values.StructValue); ok {
		keyVal, ok := rhsVal.(*values.StringValue)
		if !ok {
			return nil, fmt.Errorf("struct member name must be a string")
		}
		if val, exists := structVal.Value()[keyVal.Value()]; exists {
			return val, nil
		}
		return nil, &errors.OutOfBounds{
			BaseError: errors.BaseError{
				Node:    e.Arguments()[1],
				Message: "struct member not found",
			},
		}
	}

	return nil, fmt.Errorf("invalid operand for access operator")
}

// andOperator implements logical AND with short-circuit evaluation
type andOperator struct{}

func (op *andOperator) InferType(e *expr.Apply) (types.Base, error) {
	if len(e.Arguments()) != 2 {
		return nil, fmt.Errorf("&& requires exactly 2 arguments")
	}

	for _, arg := range e.Arguments() {
		argType, err := arg.InferType(env.NewBindings[types.Base]())
		if err != nil {
			return nil, err
		}
		if !argType.CheckType(types.NewBoolean(false)) {
			return nil, &errors.IncompatibleOperand{
				BaseError: errors.BaseError{
					Node:    arg,
					Message: "non-Boolean operand to &&",
				},
			}
		}
		if argType.Optional() {
			return nil, &errors.IncompatibleOperand{
				BaseError: errors.BaseError{
					Node:    arg,
					Message: "optional Boolean? operand to &&",
				},
			}
		}
	}
	return types.NewBoolean(false), nil
}

func (op *andOperator) Call(e *expr.Apply, bindings env.Bindings[values.Base], stdlib *Base) (values.Base, error) {
	lhs, err := e.Arguments()[0].Eval(bindings, stdlib)
	if err != nil {
		return nil, err
	}

	lhsBool, ok := lhs.(*values.BooleanValue)
	if !ok {
		return nil, fmt.Errorf("expected Boolean value")
	}

	// Short-circuit evaluation
	if !lhsBool.Value() {
		return values.NewBoolean(false), nil
	}

	rhs, err := e.Arguments()[1].Eval(bindings, stdlib)
	if err != nil {
		return nil, err
	}

	rhsBool, ok := rhs.(*values.BooleanValue)
	if !ok {
		return nil, fmt.Errorf("expected Boolean value")
	}

	return values.NewBoolean(rhsBool.Value()), nil
}

// orOperator implements logical OR with short-circuit evaluation
type orOperator struct{}

func (op *orOperator) InferType(e *expr.Apply) (types.Base, error) {
	if len(e.Arguments()) != 2 {
		return nil, fmt.Errorf("|| requires exactly 2 arguments")
	}

	for _, arg := range e.Arguments() {
		argType, err := arg.InferType(env.NewBindings[types.Base]())
		if err != nil {
			return nil, err
		}
		if !argType.CheckType(types.NewBoolean(false)) {
			return nil, &errors.IncompatibleOperand{
				BaseError: errors.BaseError{
					Node:    arg,
					Message: "non-Boolean operand to ||",
				},
			}
		}
		if argType.Optional() {
			return nil, &errors.IncompatibleOperand{
				BaseError: errors.BaseError{
					Node:    arg,
					Message: "optional Boolean? operand to ||",
				},
			}
		}
	}
	return types.NewBoolean(false), nil
}

func (op *orOperator) Call(e *expr.Apply, bindings env.Bindings[values.Base], stdlib *Base) (values.Base, error) {
	lhs, err := e.Arguments()[0].Eval(bindings, stdlib)
	if err != nil {
		return nil, err
	}

	lhsBool, ok := lhs.(*values.BooleanValue)
	if !ok {
		return nil, fmt.Errorf("expected Boolean value")
	}

	// Short-circuit evaluation
	if lhsBool.Value() {
		return values.NewBoolean(true), nil
	}

	rhs, err := e.Arguments()[1].Eval(bindings, stdlib)
	if err != nil {
		return nil, err
	}

	rhsBool, ok := rhs.(*values.BooleanValue)
	if !ok {
		return nil, fmt.Errorf("expected Boolean value")
	}

	return values.NewBoolean(rhsBool.Value()), nil
}

// arithmeticOperator implements arithmetic operations
type arithmeticOperator struct {
	name string
	op   func(l, r interface{}) interface{}
}

func (ao *arithmeticOperator) InferType(e *expr.Apply) (types.Base, error) {
	if len(e.Arguments()) != 2 {
		return nil, fmt.Errorf("%s requires exactly 2 arguments", ao.name)
	}

	lhsType, err := e.Arguments()[0].InferType(env.NewBindings[types.Base]())
	if err != nil {
		return nil, err
	}

	rhsType, err := e.Arguments()[1].InferType(env.NewBindings[types.Base]())
	if err != nil {
		return nil, err
	}

	// Return Float if either operand is Float
	var returnType types.Base = types.NewInt(false)
	if _, ok := lhsType.(*types.FloatType); ok {
		returnType = types.NewFloat(false)
	}
	if _, ok := rhsType.(*types.FloatType); ok {
		returnType = types.NewFloat(false)
	}

	// Check both operands can be coerced to the return type
	if !lhsType.CheckType(returnType) || !rhsType.CheckType(returnType) {
		return nil, &errors.IncompatibleOperand{
			BaseError: errors.BaseError{
				Node:    e,
				Message: fmt.Sprintf("Non-numeric operand to %s operator", ao.name),
			},
		}
	}

	return returnType, nil
}

func (ao *arithmeticOperator) Call(e *expr.Apply, bindings env.Bindings[values.Base], stdlib *Base) (values.Base, error) {
	returnType, err := ao.InferType(e)
	if err != nil {
		return nil, err
	}

	lhs, err := e.Arguments()[0].Eval(bindings, stdlib)
	if err != nil {
		return nil, err
	}
	lhs, err = lhs.Coerce(returnType)
	if err != nil {
		return nil, err
	}

	rhs, err := e.Arguments()[1].Eval(bindings, stdlib)
	if err != nil {
		return nil, err
	}
	rhs, err = rhs.Coerce(returnType)
	if err != nil {
		return nil, err
	}

	// Perform operation based on type
	if _, isFloat := returnType.(*types.FloatType); isFloat {
		lhsVal := lhs.(*values.FloatValue).Value()
		rhsVal := rhs.(*values.FloatValue).Value()
		result := ao.op(lhsVal, rhsVal)
		if result == nil {
			return nil, fmt.Errorf("arithmetic operation failed")
		}
		return values.NewFloat(result.(float64)), nil
	}

	lhsVal := lhs.(*values.IntValue).Value()
	rhsVal := rhs.(*values.IntValue).Value()
	result := ao.op(lhsVal, rhsVal)
	if result == nil {
		return nil, fmt.Errorf("arithmetic operation failed")
	}
	return values.NewInt(result.(int64)), nil
}

// addOperator handles both arithmetic addition and string concatenation
type addOperator struct {
	arithmeticOperator
}

func newAddOperator() *addOperator {
	return &addOperator{
		arithmeticOperator: arithmeticOperator{
			name: "+",
			op: func(l, r interface{}) interface{} {
				switch v := l.(type) {
				case int64:
					return v + r.(int64)
				case float64:
					return v + r.(float64)
				case string:
					return v + r.(string)
				default:
					return nil
				}
			},
		},
	}
}

func (ao *addOperator) InferType(e *expr.Apply) (types.Base, error) {
	if len(e.Arguments()) != 2 {
		return nil, fmt.Errorf("+ requires exactly 2 arguments")
	}

	lhsType, err := e.Arguments()[0].InferType(env.NewBindings[types.Base]())
	if err != nil {
		return nil, err
	}

	rhsType, err := e.Arguments()[1].InferType(env.NewBindings[types.Base]())
	if err != nil {
		return nil, err
	}

	// Check for string concatenation
	if _, ok := lhsType.(*types.StringType); ok {
		if rhsType.CheckType(types.NewString(false)) {
			return types.NewString(false), nil
		}
	}
	if _, ok := rhsType.(*types.StringType); ok {
		if lhsType.CheckType(types.NewString(false)) {
			return types.NewString(false), nil
		}
	}

	// Otherwise, treat as arithmetic addition
	return ao.arithmeticOperator.InferType(e)
}

func (ao *addOperator) Call(e *expr.Apply, bindings env.Bindings[values.Base], stdlib *Base) (values.Base, error) {
	returnType, err := ao.InferType(e)
	if err != nil {
		return nil, err
	}

	// Handle string concatenation
	if _, ok := returnType.(*types.StringType); ok {
		lhs, err := e.Arguments()[0].Eval(bindings, stdlib)
		if err != nil {
			return nil, err
		}
		lhs, err = lhs.Coerce(types.NewString(false))
		if err != nil {
			return nil, err
		}

		rhs, err := e.Arguments()[1].Eval(bindings, stdlib)
		if err != nil {
			return nil, err
		}
		rhs, err = rhs.Coerce(types.NewString(false))
		if err != nil {
			return nil, err
		}

		lhsStr := lhs.(*values.StringValue).Value()
		rhsStr := rhs.(*values.StringValue).Value()
		return values.NewString(lhsStr + rhsStr), nil
	}

	// Handle arithmetic addition
	return ao.arithmeticOperator.Call(e, bindings, stdlib)
}

// interpolationAddOperator handles addition within string interpolation
type interpolationAddOperator struct {
	addOperator
}

func (io *interpolationAddOperator) InferType(e *expr.Apply) (types.Base, error) {
	lhsType, err := e.Arguments()[0].InferType(env.NewBindings[types.Base]())
	if err != nil {
		return nil, err
	}

	rhsType, err := e.Arguments()[1].InferType(env.NewBindings[types.Base]())
	if err != nil {
		return nil, err
	}

	// Check if either operand is String
	eitherString := false
	if _, ok := lhsType.(*types.StringType); ok {
		eitherString = true
	}
	if _, ok := rhsType.(*types.StringType); ok {
		eitherString = true
	}

	// Check if either operand is optional
	eitherOptional := lhsType.Optional() || rhsType.Optional()

	// Check if both can be coerced to String
	bothStringifiable := lhsType.CheckType(types.NewString(true)) && rhsType.CheckType(types.NewString(true))

	if eitherString && eitherOptional && bothStringifiable {
		return types.NewString(true), nil
	}

	return io.addOperator.InferType(e)
}

func (io *interpolationAddOperator) Call(e *expr.Apply, bindings env.Bindings[values.Base], stdlib *Base) (values.Base, error) {
	lhs, err := e.Arguments()[0].Eval(bindings, stdlib)
	if err != nil {
		return nil, err
	}

	rhs, err := e.Arguments()[1].Eval(bindings, stdlib)
	if err != nil {
		return nil, err
	}

	// Return null if either operand is null
	if _, ok := lhs.(*values.NullValue); ok {
		return values.NewNull(), nil
	}
	if _, ok := rhs.(*values.NullValue); ok {
		return values.NewNull(), nil
	}

	return io.addOperator.Call(e, bindings, stdlib)
}

// equalityOperator implements == and != operators
type equalityOperator struct {
	negate bool
}

func (eo *equalityOperator) InferType(e *expr.Apply) (types.Base, error) {
	if len(e.Arguments()) != 2 {
		op := "=="
		if eo.negate {
			op = "!="
		}
		return nil, fmt.Errorf("%s requires exactly 2 arguments", op)
	}

	lhsType, err := e.Arguments()[0].InferType(env.NewBindings[types.Base]())
	if err != nil {
		return nil, err
	}

	rhsType, err := e.Arguments()[1].InferType(env.NewBindings[types.Base]())
	if err != nil {
		return nil, err
	}

	// Check if types are equatable
	if !lhsType.Equatable(rhsType) {
		return nil, &errors.IncompatibleOperand{
			BaseError: errors.BaseError{
				Node:    e,
				Message: fmt.Sprintf("Cannot test equality of %s and %s", lhsType.String(), rhsType.String()),
			},
		}
	}

	return types.NewBoolean(false), nil
}

func (eo *equalityOperator) Call(e *expr.Apply, bindings env.Bindings[values.Base], stdlib *Base) (values.Base, error) {
	lhs, err := e.Arguments()[0].Eval(bindings, stdlib)
	if err != nil {
		return nil, err
	}

	rhs, err := e.Arguments()[1].Eval(bindings, stdlib)
	if err != nil {
		return nil, err
	}

	result := lhs.Equals(rhs)
	if eo.negate {
		result = !result
	}
	return values.NewBoolean(result), nil
}

// comparisonOperator implements <, <=, >, >= operators
type comparisonOperator struct {
	name string
	op   func(l, r interface{}) bool
}

func (co *comparisonOperator) InferType(e *expr.Apply) (types.Base, error) {
	if len(e.Arguments()) != 2 {
		return nil, fmt.Errorf("%s requires exactly 2 arguments", co.name)
	}

	lhsType, err := e.Arguments()[0].InferType(env.NewBindings[types.Base]())
	if err != nil {
		return nil, err
	}

	rhsType, err := e.Arguments()[1].InferType(env.NewBindings[types.Base]())
	if err != nil {
		return nil, err
	}

	// Check if types are comparable
	if !lhsType.Comparable(rhsType) {
		return nil, &errors.IncompatibleOperand{
			BaseError: errors.BaseError{
				Node:    e,
				Message: fmt.Sprintf("Cannot compare %s and %s", lhsType.String(), rhsType.String()),
			},
		}
	}

	return types.NewBoolean(false), nil
}

func (co *comparisonOperator) Call(e *expr.Apply, bindings env.Bindings[values.Base], stdlib *Base) (values.Base, error) {
	lhs, err := e.Arguments()[0].Eval(bindings, stdlib)
	if err != nil {
		return nil, err
	}

	rhs, err := e.Arguments()[1].Eval(bindings, stdlib)
	if err != nil {
		return nil, err
	}

	// Extract raw values for comparison
	var lhsVal, rhsVal interface{}

	switch v := lhs.(type) {
	case *values.IntValue:
		lhsVal = v.Value()
		if rv, ok := rhs.(*values.IntValue); ok {
			rhsVal = rv.Value()
		}
	case *values.FloatValue:
		lhsVal = v.Value()
		if rv, ok := rhs.(*values.FloatValue); ok {
			rhsVal = rv.Value()
		}
	case *values.StringValue:
		lhsVal = v.Value()
		if rv, ok := rhs.(*values.StringValue); ok {
			rhsVal = rv.Value()
		}
	default:
		return nil, fmt.Errorf("unsupported types for comparison")
	}

	if rhsVal == nil {
		return nil, fmt.Errorf("type mismatch in comparison")
	}

	result := co.op(lhsVal, rhsVal)
	return values.NewBoolean(result), nil
}