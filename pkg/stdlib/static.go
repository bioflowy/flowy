package stdlib

import (
	"fmt"

	"github.com/uehara/flowy/pkg/env"
	"github.com/uehara/flowy/pkg/errors"
	"github.com/uehara/flowy/pkg/expr"
	"github.com/uehara/flowy/pkg/types"
	"github.com/uehara/flowy/pkg/values"
)

// staticFunction implements a function with static argument and return types
type staticFunction struct {
	name          string
	argumentTypes []types.Base
	returnType    types.Base
	f             func([]values.Base) (values.Base, error)
}

func (sf *staticFunction) InferType(e *expr.Apply) (types.Base, error) {
	// Calculate minimum number of required arguments (non-optional)
	minArgs := len(sf.argumentTypes)
	for i := len(sf.argumentTypes) - 1; i >= 0; i-- {
		if sf.argumentTypes[i].Optional() {
			minArgs--
		} else {
			break
		}
	}

	// Check argument count
	if len(e.Arguments()) > len(sf.argumentTypes) || len(e.Arguments()) < minArgs {
		return nil, &errors.WrongArity{
			BaseError: errors.BaseError{
				Node: e,
			},
			Expected: len(sf.argumentTypes),
		}
	}

	// Type check each argument
	for i, arg := range e.Arguments() {
		argType, err := arg.InferType(env.NewBindings[types.Base]())
		if err != nil {
			return nil, err
		}
		if !argType.CheckType(sf.argumentTypes[i]) {
			return nil, &errors.StaticTypeMismatch{
				BaseError: errors.BaseError{
					Node: arg,
				},
				Expected: sf.argumentTypes[i],
				Actual:   argType,
				Message:  fmt.Sprintf("for %s argument #%d", sf.name, i+1),
			}
		}
	}

	return sf.returnType, nil
}

func (sf *staticFunction) Call(e *expr.Apply, bindings env.Bindings[values.Base], stdlib *Base) (values.Base, error) {
	// Evaluate all arguments
	args := make([]values.Base, len(e.Arguments()))
	for i, arg := range e.Arguments() {
		val, err := arg.Eval(bindings, stdlib)
		if err != nil {
			return nil, err
		}
		// Coerce to expected type
		coerced, err := val.Coerce(sf.argumentTypes[i])
		if err != nil {
			return nil, err
		}
		args[i] = coerced
	}

	// Call the function
	result, err := sf.f(args)
	if err != nil {
		return nil, &errors.EvalError{
			BaseError: errors.BaseError{
				Node:    e,
				Message: fmt.Sprintf("function evaluation failed: %v", err),
			},
		}
	}

	// Coerce result to return type
	return result.Coerce(sf.returnType)
}

// eagerFunctionBase provides common functionality for eager functions
type eagerFunctionBase struct {
	callEager func(*expr.Apply, []values.Base) (values.Base, error)
}

func (ef *eagerFunctionBase) Call(e *expr.Apply, bindings env.Bindings[values.Base], stdlib *Base) (values.Base, error) {
	// Evaluate all arguments eagerly
	args := make([]values.Base, len(e.Arguments()))
	for i, arg := range e.Arguments() {
		val, err := arg.Eval(bindings, stdlib)
		if err != nil {
			return nil, err
		}
		args[i] = val
	}
	return ef.callEager(e, args)
}