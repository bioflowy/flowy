package eval

import (
	"fmt"
	"math"

	"github.com/bioflowy/flowy/wdl/types"
	"github.com/bioflowy/flowy/wdl/values"
)

// init registers all STDLIB-06 (numeric) functions into the stdlibFuncs dispatch map.
func init() {
	stdlibFuncs["ceil"]  = stdCeil
	stdlibFuncs["floor"] = stdFloor
	stdlibFuncs["round"] = stdRound
	stdlibFuncs["min"]   = stdMin
	stdlibFuncs["max"]   = stdMax
}

// stdCeil implements ceil(Float) -> Int.
// Returns the smallest integer >= x.
func stdCeil(args []values.Value, ctx ExecContext) (values.Value, error) {
	if len(args) < 1 {
		return nil, &EvalError{Message: "ceil: expected Float argument"}
	}
	fv, ok := args[0].(values.FloatValue)
	if !ok {
		return nil, &EvalError{Message: fmt.Sprintf("ceil: expected FloatValue, got %T", args[0])}
	}
	return values.IntValue{Type: types.IntType{}, Value: int64(math.Ceil(fv.Value))}, nil
}

// stdFloor implements floor(Float) -> Int.
// Returns the largest integer <= x.
func stdFloor(args []values.Value, ctx ExecContext) (values.Value, error) {
	if len(args) < 1 {
		return nil, &EvalError{Message: "floor: expected Float argument"}
	}
	fv, ok := args[0].(values.FloatValue)
	if !ok {
		return nil, &EvalError{Message: fmt.Sprintf("floor: expected FloatValue, got %T", args[0])}
	}
	return values.IntValue{Type: types.IntType{}, Value: int64(math.Floor(fv.Value))}, nil
}

// stdRound implements round(Float) -> Int.
// Uses WDL half-up semantics: math.Floor(x + 0.5).
// CRITICAL: Do NOT use math.Round — it gives wrong result for round(-2.5).
// math.Round(-2.5) == -3, but WDL half-up requires round(-2.5) == -2.
func stdRound(args []values.Value, ctx ExecContext) (values.Value, error) {
	if len(args) < 1 {
		return nil, &EvalError{Message: "round: expected Float argument"}
	}
	fv, ok := args[0].(values.FloatValue)
	if !ok {
		return nil, &EvalError{Message: fmt.Sprintf("round: expected FloatValue, got %T", args[0])}
	}
	return values.IntValue{Type: types.IntType{}, Value: int64(math.Floor(fv.Value + 0.5))}, nil
}

// numericArgs extracts float64 representations of two numeric args.
// Returns (val1, val2, isFloat1, isFloat2) where isFloatN indicates FloatValue input.
func numericArgs(args []values.Value, fname string) (float64, float64, bool, bool, error) {
	if len(args) < 2 {
		return 0, 0, false, false, &EvalError{Message: fname + ": expected two numeric arguments"}
	}
	var v1, v2 float64
	var f1, f2 bool
	switch a := args[0].(type) {
	case values.IntValue:
		v1 = float64(a.Value)
	case values.FloatValue:
		v1 = a.Value
		f1 = true
	default:
		return 0, 0, false, false, &EvalError{Message: fmt.Sprintf("%s: first argument must be Int or Float, got %T", fname, args[0])}
	}
	switch a := args[1].(type) {
	case values.IntValue:
		v2 = float64(a.Value)
	case values.FloatValue:
		v2 = a.Value
		f2 = true
	default:
		return 0, 0, false, false, &EvalError{Message: fmt.Sprintf("%s: second argument must be Int or Float, got %T", fname, args[1])}
	}
	return v1, v2, f1, f2, nil
}

// stdMin implements min(X, X) -> X where X is Int or Float.
// Both Int -> IntValue; both Float -> FloatValue; mixed -> FloatValue (promote Int to Float).
func stdMin(args []values.Value, ctx ExecContext) (values.Value, error) {
	v1, v2, f1, f2, err := numericArgs(args, "min")
	if err != nil {
		return nil, err
	}
	var result float64
	if v1 < v2 {
		result = v1
	} else {
		result = v2
	}
	if !f1 && !f2 {
		// Both Int: compare as int64 for exact semantics
		i1 := args[0].(values.IntValue).Value
		i2 := args[1].(values.IntValue).Value
		if i1 < i2 {
			return values.IntValue{Type: types.IntType{}, Value: i1}, nil
		}
		return values.IntValue{Type: types.IntType{}, Value: i2}, nil
	}
	return values.FloatValue{Type: types.FloatType{}, Value: result}, nil
}

// stdMax implements max(X, X) -> X where X is Int or Float.
// Both Int -> IntValue; both Float -> FloatValue; mixed -> FloatValue (promote Int to Float).
func stdMax(args []values.Value, ctx ExecContext) (values.Value, error) {
	v1, v2, f1, f2, err := numericArgs(args, "max")
	if err != nil {
		return nil, err
	}
	var result float64
	if v1 > v2 {
		result = v1
	} else {
		result = v2
	}
	if !f1 && !f2 {
		// Both Int: compare as int64 for exact semantics
		i1 := args[0].(values.IntValue).Value
		i2 := args[1].(values.IntValue).Value
		if i1 > i2 {
			return values.IntValue{Type: types.IntType{}, Value: i1}, nil
		}
		return values.IntValue{Type: types.IntType{}, Value: i2}, nil
	}
	return values.FloatValue{Type: types.FloatType{}, Value: result}, nil
}
