package stdlib

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/uehara/flowy/pkg/types"
	"github.com/uehara/flowy/pkg/values"
)

// initStringFunctions initializes string manipulation functions
func (b *Base) initStringFunctions() {
	// sub function - substitute regex pattern
	b.SubStr = &staticFunction{
		name: "sub",
		argumentTypes: []types.Base{
			types.NewString(false), // input string
			types.NewString(false), // pattern
			types.NewString(false), // replacement
		},
		returnType: types.NewString(false),
		f: func(args []values.Base) (values.Base, error) {
			input := args[0].(*values.StringValue).Value()
			pattern := args[1].(*values.StringValue).Value()
			replacement := args[2].(*values.StringValue).Value()

			re, err := regexp.Compile(pattern)
			if err != nil {
				return nil, err
			}

			result := re.ReplaceAllString(input, replacement)
			return values.NewString(result), nil
		},
	}

	// basename function
	b.Basename = &staticFunction{
		name: "basename",
		argumentTypes: []types.Base{
			types.NewString(false),      // path
			types.NewString(true), // optional suffix to remove
		},
		returnType: types.NewString(false),
		f: func(args []values.Base) (values.Base, error) {
			path := args[0].(*values.StringValue).Value()
			base := filepath.Base(path)

			// Remove suffix if provided
			if len(args) > 1 {
				if suffixVal, ok := args[1].(*values.StringValue); ok {
					suffix := suffixVal.Value()
					base = strings.TrimSuffix(base, suffix)
				}
			}

			return values.NewString(base), nil
		},
	}

	// sep function - join array with separator
	b.Sep = &staticFunction{
		name: "sep",
		argumentTypes: []types.Base{
			types.NewString(false),                            // separator
			types.NewArray(types.NewString(false), false, false), // array to join
		},
		returnType: types.NewString(false),
		f: func(args []values.Base) (values.Base, error) {
			separator := args[0].(*values.StringValue).Value()
			array := args[1].(*values.ArrayValue)

			parts := make([]string, len(array.Value()))
			for i, v := range array.Value() {
				strVal := v.(*values.StringValue)
				parts[i] = strVal.Value()
			}

			result := strings.Join(parts, separator)
			return values.NewString(result), nil
		},
	}

	// defined function - check if value is not null
	b.Defined = &staticFunction{
		name:          "defined",
		argumentTypes: []types.Base{types.NewAny(true)}, // accepts optional of any type
		returnType:    types.NewBoolean(false),
		f: func(args []values.Base) (values.Base, error) {
			_, isNull := args[0].(*values.NullValue)
			return values.NewBoolean(!isNull), nil
		},
	}
}