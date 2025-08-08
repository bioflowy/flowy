package stdlib

import (
	"math"

	"github.com/uehara/flowy/pkg/types"
	"github.com/uehara/flowy/pkg/values"
)

// initMathFunctions initializes mathematical functions
func (b *Base) initMathFunctions() {
	// floor function
	b.Floor = &staticFunction{
		name:          "floor",
		argumentTypes: []types.Base{types.NewFloat(false)},
		returnType:    types.NewInt(false),
		f: func(args []values.Base) (values.Base, error) {
			floatVal := args[0].(*values.FloatValue)
			return values.NewInt(int64(math.Floor(floatVal.Value()))), nil
		},
	}

	// ceil function
	b.Ceil = &staticFunction{
		name:          "ceil",
		argumentTypes: []types.Base{types.NewFloat(false)},
		returnType:    types.NewInt(false),
		f: func(args []values.Base) (values.Base, error) {
			floatVal := args[0].(*values.FloatValue)
			return values.NewInt(int64(math.Ceil(floatVal.Value()))), nil
		},
	}

	// round function - implements round-half-up
	b.Round = &staticFunction{
		name:          "round",
		argumentTypes: []types.Base{types.NewFloat(false)},
		returnType:    types.NewInt(false),
		f: func(args []values.Base) (values.Base, error) {
			floatVal := args[0].(*values.FloatValue)
			v := floatVal.Value()
			// Round-half-up: add 0.5 and floor for positive, subtract 0.5 and ceil for negative
			if v >= 0 {
				return values.NewInt(int64(math.Floor(v + 0.5))), nil
			}
			return values.NewInt(int64(math.Ceil(v - 0.5))), nil
		},
	}

	// length function
	b.Length = &staticFunction{
		name:          "length",
		argumentTypes: []types.Base{types.NewArray(types.NewAny(false), false, false)},
		returnType:    types.NewInt(false),
		f: func(args []values.Base) (values.Base, error) {
			arrVal := args[0].(*values.ArrayValue)
			return values.NewInt(int64(len(arrVal.Value()))), nil
		},
	}

	// WDL 1.1+ math functions
	if b.wdlVersion != "draft-2" && b.wdlVersion != "1.0" {
		// min function
		b.Min = &arithmeticOperator{
			name: "min",
			op: func(l, r interface{}) interface{} {
				switch v := l.(type) {
				case int64:
					rv := r.(int64)
					if v < rv {
						return v
					}
					return rv
				case float64:
					rv := r.(float64)
					return math.Min(v, rv)
				default:
					return nil
				}
			},
		}

		// max function
		b.Max = &arithmeticOperator{
			name: "max",
			op: func(l, r interface{}) interface{} {
				switch v := l.(type) {
				case int64:
					rv := r.(int64)
					if v > rv {
						return v
					}
					return rv
				case float64:
					rv := r.(float64)
					return math.Max(v, rv)
				default:
					return nil
				}
			},
		}
	}
}