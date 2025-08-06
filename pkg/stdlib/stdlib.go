package stdlib

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/bioflowy/flowy/pkg/errors"
	"github.com/bioflowy/flowy/pkg/expr"
	"github.com/bioflowy/flowy/pkg/types"
	"github.com/bioflowy/flowy/pkg/values"
)

// StandardLibrary provides a basic implementation of the WDL standard library
type StandardLibrary struct {
	functions map[string]*expr.Function
	operators map[string]bool
}

// NewStandardLibrary creates a new standard library instance
func NewStandardLibrary() *StandardLibrary {
	stdlib := &StandardLibrary{
		functions: make(map[string]*expr.Function),
		operators: make(map[string]bool),
	}

	stdlib.registerFunctions()
	stdlib.registerOperators()
	return stdlib
}

func (s *StandardLibrary) registerFunctions() {
	// String functions
	s.functions["length"] = &expr.Function{
		Name:       "length",
		ParamTypes: []types.Base{types.NewString(false)},
		ReturnType: types.NewInt(false),
		Variadic:   false,
	}

	s.functions["sub"] = &expr.Function{
		Name:       "sub",
		ParamTypes: []types.Base{types.NewString(false), types.NewString(false), types.NewString(false)},
		ReturnType: types.NewString(false),
		Variadic:   false,
	}

	// Array functions
	s.functions["size"] = &expr.Function{
		Name:       "size",
		ParamTypes: []types.Base{types.NewArray(types.NewAny(false, false), false, false)},
		ReturnType: types.NewInt(false),
		Variadic:   false,
	}

	s.functions["select_first"] = &expr.Function{
		Name:       "select_first",
		ParamTypes: []types.Base{types.NewArray(types.NewAny(true, false), false, false)},
		ReturnType: types.NewAny(false, false),
		Variadic:   false,
	}

	// Math functions
	s.functions["floor"] = &expr.Function{
		Name:       "floor",
		ParamTypes: []types.Base{types.NewFloat(false)},
		ReturnType: types.NewInt(false),
		Variadic:   false,
	}

	s.functions["ceil"] = &expr.Function{
		Name:       "ceil",
		ParamTypes: []types.Base{types.NewFloat(false)},
		ReturnType: types.NewInt(false),
		Variadic:   false,
	}

	s.functions["round"] = &expr.Function{
		Name:       "round",
		ParamTypes: []types.Base{types.NewFloat(false)},
		ReturnType: types.NewInt(false),
		Variadic:   false,
	}

	// Type conversion functions
	s.functions["str"] = &expr.Function{
		Name:       "str",
		ParamTypes: []types.Base{types.NewAny(false, false)},
		ReturnType: types.NewString(false),
		Variadic:   false,
	}

	s.functions["int"] = &expr.Function{
		Name:       "int",
		ParamTypes: []types.Base{types.NewString(false)},
		ReturnType: types.NewInt(false),
		Variadic:   false,
	}

	s.functions["float"] = &expr.Function{
		Name:       "float",
		ParamTypes: []types.Base{types.NewString(false)},
		ReturnType: types.NewFloat(false),
		Variadic:   false,
	}

	// File functions (basic stubs)
	s.functions["basename"] = &expr.Function{
		Name:       "basename",
		ParamTypes: []types.Base{types.NewString(false)},
		ReturnType: types.NewString(false),
		Variadic:   false,
	}

	s.functions["dirname"] = &expr.Function{
		Name:       "dirname",
		ParamTypes: []types.Base{types.NewString(false)},
		ReturnType: types.NewString(false),
		Variadic:   false,
	}
}

func (s *StandardLibrary) registerOperators() {
	// Arithmetic operators
	s.operators["+"] = true
	s.operators["-"] = true
	s.operators["*"] = true
	s.operators["/"] = true
	s.operators["%"] = true

	// Comparison operators
	s.operators["<"] = true
	s.operators["<="] = true
	s.operators[">"] = true
	s.operators[">="] = true
	s.operators["=="] = true
	s.operators["!="] = true

	// Logical operators
	s.operators["&&"] = true
	s.operators["||"] = true
	s.operators["!"] = true
}

// HasFunction checks if a function with the given name exists
func (s *StandardLibrary) HasFunction(name string) bool {
	_, exists := s.functions[name]
	return exists
}

// GetFunction returns function metadata
func (s *StandardLibrary) GetFunction(name string) (*expr.Function, error) {
	if fn, exists := s.functions[name]; exists {
		return fn, nil
	}
	return nil, fmt.Errorf("unknown function: %s", name)
}

// CallFunction invokes a function with given arguments
func (s *StandardLibrary) CallFunction(name string, args []values.Base, pos errors.SourcePosition) (values.Base, error) {
	switch name {
	case "length":
		return s.length(args, pos)
	case "sub":
		return s.sub(args, pos)
	case "size":
		return s.size(args, pos)
	case "select_first":
		return s.selectFirst(args, pos)
	case "floor":
		return s.floor(args, pos)
	case "ceil":
		return s.ceil(args, pos)
	case "round":
		return s.round(args, pos)
	case "str":
		return s.str(args, pos)
	case "int":
		return s.parseInt(args, pos)
	case "float":
		return s.parseFloat(args, pos)
	case "basename":
		return s.basename(args, pos)
	case "dirname":
		return s.dirname(args, pos)
	default:
		return nil, errors.NewEvalError(nil, fmt.Sprintf("unknown function: %s", name))
	}
}

// HasOperator checks if a binary/unary operator exists
func (s *StandardLibrary) HasOperator(op string) bool {
	return s.operators[op]
}

// CallOperator invokes an operator with given arguments
func (s *StandardLibrary) CallOperator(op string, args []values.Base, pos errors.SourcePosition) (values.Base, error) {
	// Basic operators are handled in the expression implementations
	// This method can be used for extended operators
	return nil, errors.NewEvalError(nil, fmt.Sprintf("operator %s not implemented in stdlib", op))
}

// String functions

func (s *StandardLibrary) length(args []values.Base, _ errors.SourcePosition) (values.Base, error) {
	if len(args) != 1 {
		return nil, errors.NewWrongArity(nil, "length", 1)
	}

	strValue, err := args[0].Coerce(types.NewString(false))
	if err != nil {
		return nil, errors.NewEvalError(nil, "length() requires string argument: "+err.Error())
	}

	str := strValue.(*values.StringValue).Value().(string)
	return values.NewInt(int64(len(str)), false), nil
}

func (s *StandardLibrary) sub(args []values.Base, _ errors.SourcePosition) (values.Base, error) {
	if len(args) != 3 {
		return nil, errors.NewWrongArity(nil, "sub", 3)
	}

	// Convert arguments to strings
	strValue, err := args[0].Coerce(types.NewString(false))
	if err != nil {
		return nil, errors.NewEvalError(nil, "sub() first argument must be string: "+err.Error())
	}

	patternValue, err := args[1].Coerce(types.NewString(false))
	if err != nil {
		return nil, errors.NewEvalError(nil, "sub() second argument must be string: "+err.Error())
	}

	replacementValue, err := args[2].Coerce(types.NewString(false))
	if err != nil {
		return nil, errors.NewEvalError(nil, "sub() third argument must be string: "+err.Error())
	}

	str := strValue.(*values.StringValue).Value().(string)
	pattern := patternValue.(*values.StringValue).Value().(string)
	replacement := replacementValue.(*values.StringValue).Value().(string)

	// Simple string replacement - in a full implementation this would support regex
	result := strings.ReplaceAll(str, pattern, replacement)
	return values.NewString(result, false), nil
}

// Array functions

func (s *StandardLibrary) size(args []values.Base, _ errors.SourcePosition) (values.Base, error) {
	if len(args) != 1 {
		return nil, errors.NewWrongArity(nil, "size", 1)
	}

	arrayValue, ok := args[0].(*values.ArrayValue)
	if !ok {
		return nil, errors.NewEvalError(nil, "size() requires array argument")
	}

	return values.NewInt(int64(len(arrayValue.Items())), false), nil
}

func (s *StandardLibrary) selectFirst(args []values.Base, _ errors.SourcePosition) (values.Base, error) {
	if len(args) != 1 {
		return nil, errors.NewWrongArity(nil, "select_first", 1)
	}

	arrayValue, ok := args[0].(*values.ArrayValue)
	if !ok {
		return nil, errors.NewEvalError(nil, "select_first() requires array argument")
	}

	items := arrayValue.Items()
	for _, item := range items {
		if _, isNull := item.(*values.Null); !isNull {
			return item, nil
		}
	}

	// All items are null, return null
	return values.NewNull(types.NewAny(true, true)), nil
}

// Math functions

func (s *StandardLibrary) floor(args []values.Base, _ errors.SourcePosition) (values.Base, error) {
	if len(args) != 1 {
		return nil, errors.NewWrongArity(nil, "floor", 1)
	}

	floatValue, err := args[0].Coerce(types.NewFloat(false))
	if err != nil {
		return nil, errors.NewEvalError(nil, "floor() requires numeric argument: "+err.Error())
	}

	f := floatValue.(*values.FloatValue).Value().(float64)
	return values.NewInt(int64(math.Floor(f)), false), nil
}

func (s *StandardLibrary) ceil(args []values.Base, _ errors.SourcePosition) (values.Base, error) {
	if len(args) != 1 {
		return nil, errors.NewWrongArity(nil, "ceil", 1)
	}

	floatValue, err := args[0].Coerce(types.NewFloat(false))
	if err != nil {
		return nil, errors.NewEvalError(nil, "ceil() requires numeric argument: "+err.Error())
	}

	f := floatValue.(*values.FloatValue).Value().(float64)
	return values.NewInt(int64(math.Ceil(f)), false), nil
}

func (s *StandardLibrary) round(args []values.Base, _ errors.SourcePosition) (values.Base, error) {
	if len(args) != 1 {
		return nil, errors.NewWrongArity(nil, "round", 1)
	}

	floatValue, err := args[0].Coerce(types.NewFloat(false))
	if err != nil {
		return nil, errors.NewEvalError(nil, "round() requires numeric argument: "+err.Error())
	}

	f := floatValue.(*values.FloatValue).Value().(float64)
	return values.NewInt(int64(math.Round(f)), false), nil
}

// Type conversion functions

func (s *StandardLibrary) str(args []values.Base, _ errors.SourcePosition) (values.Base, error) {
	if len(args) != 1 {
		return nil, errors.NewWrongArity(nil, "str", 1)
	}

	// Use the value's string representation
	strValue, err := args[0].Coerce(types.NewString(false))
	if err != nil {
		// Fall back to string representation
		return values.NewString(args[0].String(), false), nil
	}

	return strValue, nil
}

func (s *StandardLibrary) parseInt(args []values.Base, _ errors.SourcePosition) (values.Base, error) {
	if len(args) != 1 {
		return nil, errors.NewWrongArity(nil, "int", 1)
	}

	strValue, err := args[0].Coerce(types.NewString(false))
	if err != nil {
		return nil, errors.NewEvalError(nil, "int() requires string argument: "+err.Error())
	}

	str := strValue.(*values.StringValue).Value().(string)
	intVal, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return nil, errors.NewEvalError(nil, fmt.Sprintf("cannot parse '%s' as integer: %v", str, err))
	}

	return values.NewInt(intVal, false), nil
}

func (s *StandardLibrary) parseFloat(args []values.Base, _ errors.SourcePosition) (values.Base, error) {
	if len(args) != 1 {
		return nil, errors.NewWrongArity(nil, "float", 1)
	}

	strValue, err := args[0].Coerce(types.NewString(false))
	if err != nil {
		return nil, errors.NewEvalError(nil, "float() requires string argument: "+err.Error())
	}

	str := strValue.(*values.StringValue).Value().(string)
	floatVal, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return nil, errors.NewEvalError(nil, fmt.Sprintf("cannot parse '%s' as float: %v", str, err))
	}

	return values.NewFloat(floatVal, false), nil
}

// File functions

func (s *StandardLibrary) basename(args []values.Base, _ errors.SourcePosition) (values.Base, error) {
	if len(args) != 1 {
		return nil, errors.NewWrongArity(nil, "basename", 1)
	}

	strValue, err := args[0].Coerce(types.NewString(false))
	if err != nil {
		return nil, errors.NewEvalError(nil, "basename() requires string argument: "+err.Error())
	}

	path := strValue.(*values.StringValue).Value().(string)
	// Simple basename implementation
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return values.NewString("", false), nil
	}
	return values.NewString(parts[len(parts)-1], false), nil
}

func (s *StandardLibrary) dirname(args []values.Base, _ errors.SourcePosition) (values.Base, error) {
	if len(args) != 1 {
		return nil, errors.NewWrongArity(nil, "dirname", 1)
	}

	strValue, err := args[0].Coerce(types.NewString(false))
	if err != nil {
		return nil, errors.NewEvalError(nil, "dirname() requires string argument: "+err.Error())
	}

	path := strValue.(*values.StringValue).Value().(string)
	// Simple dirname implementation
	lastSlash := strings.LastIndex(path, "/")
	if lastSlash == -1 {
		return values.NewString(".", false), nil
	}
	if lastSlash == 0 {
		return values.NewString("/", false), nil
	}
	return values.NewString(path[:lastSlash], false), nil
}
