package stdlib

import (
	"github.com/google/uuid"
	"github.com/uehara/flowy/pkg/env"
	"github.com/uehara/flowy/pkg/errors"
	"github.com/uehara/flowy/pkg/expr"
	"github.com/uehara/flowy/pkg/types"
	"github.com/uehara/flowy/pkg/values"
)

// Function is the interface for standard library function implementations
type Function interface {
	// InferType checks the types of arguments and returns the function's return type
	InferType(e *expr.Apply) (types.Base, error)

	// Call invokes the function with evaluated arguments
	Call(e *expr.Apply, bindings env.Bindings[values.Base], stdlib *Base) (values.Base, error)
}

// EagerFunction is a helper for functions that evaluate all arguments eagerly
type EagerFunction interface {
	Function
	CallEager(e *expr.Apply, arguments []values.Base) (values.Base, error)
}

// Base is the base class for standard library implementations
type Base struct {
	wdlVersion string
	writeDir   string

	// Language built-in operators
	At              Function // array/map access operator
	Land            Function // logical AND
	Lor             Function // logical OR
	Negate          Function // logical NOT
	Add             Function // addition/concatenation
	InterpolationAdd Function // addition in string interpolation
	Sub             Function // subtraction
	Mul             Function // multiplication
	Div             Function // division
	Rem             Function // remainder/modulo
	Eqeq            Function // equality
	Neq             Function // inequality
	Lt              Function // less than
	Lte             Function // less than or equal
	Gt              Function // greater than
	Gte             Function // greater than or equal

	// Math functions
	Floor  Function
	Ceil   Function
	Round  Function
	Length Function

	// String functions
	SubStr   Function // substitute regex
	Basename Function
	Sep      Function

	// Utility functions
	Defined      Function
	SelectFirst  Function
	SelectAll    Function

	// Array functions
	Range     Function
	Prefix    Function
	Suffix    Function
	Zip       Function
	Unzip     Function
	Cross     Function
	Flatten   Function
	Transpose Function

	// File I/O functions
	WriteLines Function
	WriteTsv   Function
	WriteMap   Function
	WriteJson  Function
	ReadInt    Function
	ReadFloat  Function
	ReadString Function
	ReadBoolean Function
	ReadLines  Function
	ReadTsv    Function
	ReadMap    Function
	ReadJson   Function
	ReadObject Function
	ReadObjects Function

	// WDL 1.1+ functions
	Min             Function
	Max             Function
	Quote           Function
	Squote          Function
	Keys            Function
	AsMap           Function
	AsPairs         Function
	CollectByKey    Function
}

// New creates a new standard library instance
func New(wdlVersion string, writeDir string) *Base {
	if writeDir == "" {
		writeDir = "/tmp" // Default temp directory
	}
	
	b := &Base{
		wdlVersion: wdlVersion,
		writeDir:   writeDir,
	}

	// Initialize built-in operators
	b.At = &atOperator{}
	b.Land = &andOperator{}
	b.Lor = &orOperator{}
	b.Negate = &staticFunction{
		name:          "_negate",
		argumentTypes: []types.Base{types.NewBoolean(false)},
		returnType:    types.NewBoolean(false),
		f: func(args []values.Base) (values.Base, error) {
			boolVal := args[0].(*values.BooleanValue)
			return values.NewBoolean(!boolVal.Value()), nil
		},
	}

	// Initialize arithmetic operators
	b.Add = &addOperator{}
	b.InterpolationAdd = &interpolationAddOperator{}
	b.Sub = &arithmeticOperator{name: "-", op: func(l, r interface{}) interface{} {
		switch v := l.(type) {
		case int64:
			return v - r.(int64)
		case float64:
			return v - r.(float64)
		default:
			return nil
		}
	}}
	b.Mul = &arithmeticOperator{name: "*", op: func(l, r interface{}) interface{} {
		switch v := l.(type) {
		case int64:
			return v * r.(int64)
		case float64:
			return v * r.(float64)
		default:
			return nil
		}
	}}
	b.Div = &arithmeticOperator{name: "/", op: func(l, r interface{}) interface{} {
		switch v := l.(type) {
		case int64:
			return v / r.(int64)
		case float64:
			return v / r.(float64)
		default:
			return nil
		}
	}}
	b.Rem = &staticFunction{
		name:          "_rem",
		argumentTypes: []types.Base{types.NewInt(false), types.NewInt(false)},
		returnType:    types.NewInt(false),
		f: func(args []values.Base) (values.Base, error) {
			l := args[0].(*values.IntValue).Value()
			r := args[1].(*values.IntValue).Value()
			return values.NewInt(l % r), nil
		},
	}

	// Initialize comparison operators
	b.Eqeq = &equalityOperator{negate: false}
	b.Neq = &equalityOperator{negate: true}
	b.Lt = &comparisonOperator{name: "<", op: func(l, r interface{}) bool {
		switch v := l.(type) {
		case int64:
			return v < r.(int64)
		case float64:
			return v < r.(float64)
		case string:
			return v < r.(string)
		default:
			return false
		}
	}}
	b.Lte = &comparisonOperator{name: "<=", op: func(l, r interface{}) bool {
		switch v := l.(type) {
		case int64:
			return v <= r.(int64)
		case float64:
			return v <= r.(float64)
		case string:
			return v <= r.(string)
		default:
			return false
		}
	}}
	b.Gt = &comparisonOperator{name: ">", op: func(l, r interface{}) bool {
		switch v := l.(type) {
		case int64:
			return v > r.(int64)
		case float64:
			return v > r.(float64)
		case string:
			return v > r.(string)
		default:
			return false
		}
	}}
	b.Gte = &comparisonOperator{name: ">=", op: func(l, r interface{}) bool {
		switch v := l.(type) {
		case int64:
			return v >= r.(int64)
		case float64:
			return v >= r.(float64)
		case string:
			return v >= r.(string)
		default:
			return false
		}
	}}

	// Initialize math functions
	b.initMathFunctions()

	// Initialize string functions
	b.initStringFunctions()

	// Initialize array functions
	b.initArrayFunctions()

	// Initialize file I/O functions
	b.initFileFunctions()

	// Initialize WDL 1.1+ functions if applicable
	if wdlVersion != "draft-2" && wdlVersion != "1.0" {
		b.initAdvancedFunctions()
	}

	return b
}

// GetFunction retrieves a function by name
func (b *Base) GetFunction(name string) Function {
	switch name {
	// Built-in operators
	case "_at":
		return b.At
	case "_land":
		return b.Land
	case "_lor":
		return b.Lor
	case "_negate":
		return b.Negate
	case "_add":
		return b.Add
	case "_interpolation_add":
		return b.InterpolationAdd
	case "_sub":
		return b.Sub
	case "_mul":
		return b.Mul
	case "_div":
		return b.Div
	case "_rem":
		return b.Rem
	case "_eqeq":
		return b.Eqeq
	case "_neq":
		return b.Neq
	case "_lt":
		return b.Lt
	case "_lte":
		return b.Lte
	case "_gt":
		return b.Gt
	case "_gte":
		return b.Gte

	// Math functions
	case "floor":
		return b.Floor
	case "ceil":
		return b.Ceil
	case "round":
		return b.Round
	case "length":
		return b.Length

	// String functions
	case "sub":
		return b.SubStr
	case "basename":
		return b.Basename
	case "sep":
		return b.Sep

	// Utility functions
	case "defined":
		return b.Defined
	case "select_first":
		return b.SelectFirst
	case "select_all":
		return b.SelectAll

	// Array functions
	case "range":
		return b.Range
	case "prefix":
		return b.Prefix
	case "suffix":
		return b.Suffix
	case "zip":
		return b.Zip
	case "unzip":
		return b.Unzip
	case "cross":
		return b.Cross
	case "flatten":
		return b.Flatten
	case "transpose":
		return b.Transpose

	// File I/O functions
	case "write_lines":
		return b.WriteLines
	case "write_tsv":
		return b.WriteTsv
	case "write_map":
		return b.WriteMap
	case "write_json":
		return b.WriteJson
	case "read_int":
		return b.ReadInt
	case "read_float":
		return b.ReadFloat
	case "read_string":
		return b.ReadString
	case "read_boolean":
		return b.ReadBoolean
	case "read_lines":
		return b.ReadLines
	case "read_tsv":
		return b.ReadTsv
	case "read_map":
		return b.ReadMap
	case "read_json":
		return b.ReadJson
	case "read_object":
		return b.ReadObject
	case "read_objects":
		return b.ReadObjects

	// WDL 1.1+ functions
	case "min":
		return b.Min
	case "max":
		return b.Max
	case "quote":
		return b.Quote
	case "squote":
		return b.Squote
	case "keys":
		return b.Keys
	case "as_map":
		return b.AsMap
	case "as_pairs":
		return b.AsPairs
	case "collect_by_key":
		return b.CollectByKey

	default:
		return nil
	}
}

// DevirtualizeFilename converts a virtual filename to an actual filesystem path
func (b *Base) DevirtualizeFilename(filename string) (string, error) {
	// TODO: implement file virtualization logic
	return filename, nil
}

// VirtualizeFilename converts a filesystem path to a virtual filename
func (b *Base) VirtualizeFilename(filename string) (string, error) {
	// TODO: implement file virtualization logic
	return filename, nil
}

// TaskOutputs extends Base with task-specific output functions
type TaskOutputs struct {
	*Base
	Stdout Function
	Stderr Function
	Glob   Function
}

// NewTaskOutputs creates a new standard library with task output functions
func NewTaskOutputs(wdlVersion string, writeDir string) *TaskOutputs {
	t := &TaskOutputs{
		Base: New(wdlVersion, writeDir),
	}

	// Initialize task-specific output functions
	t.Stdout = &staticFunction{
		name:          "stdout",
		argumentTypes: []types.Base{},
		returnType:    types.NewFile(false),
		f: func(args []values.Base) (values.Base, error) {
			return nil, &errors.RuntimeError{
				BaseError: errors.BaseError{Message: "stdout() not available in this context"},
			}
		},
	}

	t.Stderr = &staticFunction{
		name:          "stderr",
		argumentTypes: []types.Base{},
		returnType:    types.NewFile(false),
		f: func(args []values.Base) (values.Base, error) {
			return nil, &errors.RuntimeError{
				BaseError: errors.BaseError{Message: "stderr() not available in this context"},
			}
		},
	}

	t.Glob = &staticFunction{
		name:          "glob",
		argumentTypes: []types.Base{types.NewString(false)},
		returnType:    types.NewArray(types.NewFile(false), false, false),
		f: func(args []values.Base) (values.Base, error) {
			return nil, &errors.RuntimeError{
				BaseError: errors.BaseError{Message: "glob() not available in this context"},
			}
		},
	}

	return t
}

// Helper function to generate a unique filename
func generateFilename() string {
	return uuid.New().String()
}