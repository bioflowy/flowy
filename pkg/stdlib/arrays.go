package stdlib

import (
	"fmt"

	"github.com/uehara/flowy/pkg/env"
	"github.com/uehara/flowy/pkg/errors"
	"github.com/uehara/flowy/pkg/expr"
	"github.com/uehara/flowy/pkg/types"
	"github.com/uehara/flowy/pkg/values"
)

// initArrayFunctions initializes array manipulation functions
func (b *Base) initArrayFunctions() {
	b.Range = &rangeFunction{}
	b.Prefix = &prefixFunction{}
	b.Suffix = &suffixFunction{}
	b.Zip = &zipFunction{}
	b.Unzip = &unzipFunction{}
	b.Cross = &crossFunction{}
	b.Flatten = &flattenFunction{}
	b.Transpose = &transposeFunction{}
	b.SelectFirst = &selectFirstFunction{}
	b.SelectAll = &selectAllFunction{}
}

// rangeFunction implements range(int) -> int array
type rangeFunction struct{}

func (f *rangeFunction) InferType(e *expr.Apply) (types.Base, error) {
	if len(e.Arguments()) != 1 {
		return nil, &errors.WrongArity{
			BaseError: errors.BaseError{Node: e},
			Expected:  1,
		}
	}

	argType, err := e.Arguments()[0].InferType(env.NewBindings[types.Base]())
	if err != nil {
		return nil, err
	}

	if !argType.CheckType(types.NewInt(false)) {
		return nil, &errors.StaticTypeMismatch{
			BaseError: errors.BaseError{Node: e.Arguments()[0]},
			Expected:  types.NewInt(false),
			Actual:    argType,
		}
	}

	// Check if we can statically determine the array is nonempty
	nonempty := false
	if intLit, ok := e.Arguments()[0].(*expr.IntLiteral); ok && intLit.GetValue() > 0 {
		nonempty = true
	}

	return types.NewArray(types.NewInt(false), nonempty, false), nil
}

func (f *rangeFunction) Call(e *expr.Apply, bindings env.Bindings[values.Base], stdlib *Base) (values.Base, error) {
	arg, err := e.Arguments()[0].Eval(bindings, stdlib)
	if err != nil {
		return nil, err
	}

	intVal := arg.(*values.IntValue)
	n := intVal.Value()
	if n < 0 {
		return nil, &errors.EvalError{
			BaseError: errors.BaseError{
				Node:    e,
				Message: "range() got negative argument",
			},
		}
	}

	result := make([]values.Base, n)
	for i := int64(0); i < n; i++ {
		result[i] = values.NewInt(i)
	}

	return values.NewArray(types.NewInt(false), result), nil
}

// prefixFunction implements prefix(string, string array) -> string array
type prefixFunction struct{}

func (f *prefixFunction) InferType(e *expr.Apply) (types.Base, error) {
	if len(e.Arguments()) != 2 {
		return nil, &errors.WrongArity{
			BaseError: errors.BaseError{Node: e},
			Expected:  2,
		}
	}

	prefixType, err := e.Arguments()[0].InferType(env.NewBindings[types.Base]())
	if err != nil {
		return nil, err
	}
	if !prefixType.CheckType(types.NewString(false)) {
		return nil, &errors.StaticTypeMismatch{
			BaseError: errors.BaseError{Node: e.Arguments()[0]},
			Expected:  types.NewString(false),
			Actual:    prefixType,
		}
	}

	arrayType, err := e.Arguments()[1].InferType(env.NewBindings[types.Base]())
	if err != nil {
		return nil, err
	}
	if !arrayType.CheckType(types.NewArray(types.NewString(false), false, false)) {
		return nil, &errors.StaticTypeMismatch{
			BaseError: errors.BaseError{Node: e.Arguments()[1]},
			Expected:  types.NewArray(types.NewString(false), false, false),
			Actual:    arrayType,
		}
	}

	// Preserve nonempty property
	nonempty := false
	if arrType, ok := arrayType.(*types.ArrayType); ok {
		nonempty = arrType.Nonempty()
	}

	return types.NewArray(types.NewString(false), nonempty, false), nil
}

func (f *prefixFunction) Call(e *expr.Apply, bindings env.Bindings[values.Base], stdlib *Base) (values.Base, error) {
	prefixArg, err := e.Arguments()[0].Eval(bindings, stdlib)
	if err != nil {
		return nil, err
	}
	prefix := prefixArg.(*values.StringValue).Value()

	arrayArg, err := e.Arguments()[1].Eval(bindings, stdlib)
	if err != nil {
		return nil, err
	}
	array := arrayArg.(*values.ArrayValue)

	result := make([]values.Base, len(array.Value()))
	for i, v := range array.Value() {
		strVal, err := v.Coerce(types.NewString(false))
		if err != nil {
			return nil, err
		}
		result[i] = values.NewString(prefix + strVal.(*values.StringValue).Value())
	}

	return values.NewArray(types.NewString(false), result), nil
}

// suffixFunction implements suffix(string, string array) -> string array
type suffixFunction struct{}

func (f *suffixFunction) InferType(e *expr.Apply) (types.Base, error) {
	if len(e.Arguments()) != 2 {
		return nil, &errors.WrongArity{
			BaseError: errors.BaseError{Node: e},
			Expected:  2,
		}
	}

	suffixType, err := e.Arguments()[0].InferType(env.NewBindings[types.Base]())
	if err != nil {
		return nil, err
	}
	if !suffixType.CheckType(types.NewString(false)) {
		return nil, &errors.StaticTypeMismatch{
			BaseError: errors.BaseError{Node: e.Arguments()[0]},
			Expected:  types.NewString(false),
			Actual:    suffixType,
		}
	}

	arrayType, err := e.Arguments()[1].InferType(env.NewBindings[types.Base]())
	if err != nil {
		return nil, err
	}
	if !arrayType.CheckType(types.NewArray(types.NewString(false), false, false)) {
		return nil, &errors.StaticTypeMismatch{
			BaseError: errors.BaseError{Node: e.Arguments()[1]},
			Expected:  types.NewArray(types.NewString(false), false, false),
			Actual:    arrayType,
		}
	}

	// Preserve nonempty property
	nonempty := false
	if arrType, ok := arrayType.(*types.ArrayType); ok {
		nonempty = arrType.Nonempty()
	}

	return types.NewArray(types.NewString(false), nonempty, false), nil
}

func (f *suffixFunction) Call(e *expr.Apply, bindings env.Bindings[values.Base], stdlib *Base) (values.Base, error) {
	suffixArg, err := e.Arguments()[0].Eval(bindings, stdlib)
	if err != nil {
		return nil, err
	}
	suffix := suffixArg.(*values.StringValue).Value()

	arrayArg, err := e.Arguments()[1].Eval(bindings, stdlib)
	if err != nil {
		return nil, err
	}
	array := arrayArg.(*values.ArrayValue)

	result := make([]values.Base, len(array.Value()))
	for i, v := range array.Value() {
		strVal, err := v.Coerce(types.NewString(false))
		if err != nil {
			return nil, err
		}
		result[i] = values.NewString(strVal.(*values.StringValue).Value() + suffix)
	}

	return values.NewArray(types.NewString(false), result), nil
}

// selectFirstFunction implements select_first(Array[T?]) -> T
type selectFirstFunction struct{}

func (f *selectFirstFunction) InferType(e *expr.Apply) (types.Base, error) {
	if len(e.Arguments()) != 1 {
		return nil, &errors.WrongArity{
			BaseError: errors.BaseError{Node: e},
			Expected:  1,
		}
	}

	argType, err := e.Arguments()[0].InferType(env.NewBindings[types.Base]())
	if err != nil {
		return nil, err
	}

	arrType, ok := argType.(*types.ArrayType)
	if !ok || argType.Optional() {
		return nil, &errors.StaticTypeMismatch{
			BaseError: errors.BaseError{Node: e.Arguments()[0]},
			Expected:  types.NewArray(types.NewAny(false), false, false),
			Actual:    argType,
		}
	}

	if _, ok := arrType.ItemType().(*types.AnyType); ok {
		return nil, &errors.IndeterminateType{
			BaseError: errors.BaseError{
				Node:    e.Arguments()[0],
				Message: "can't infer item type of empty array",
			},
		}
	}

	// Return non-optional version of item type
	itemType := arrType.ItemType()
	if itemType.Optional() {
		// Create non-optional version
		switch t := itemType.(type) {
		case *types.IntType:
			return types.NewInt(false), nil
		case *types.FloatType:
			return types.NewFloat(false), nil
		case *types.StringType:
			return types.NewString(false), nil
		case *types.BooleanType:
			return types.NewBoolean(false), nil
		case *types.FileType:
			return types.NewFile(false), nil
		case *types.DirectoryType:
			return types.NewDirectory(false), nil
		case *types.ArrayType:
			return types.NewArray(t.ItemType(), t.Nonempty(), false), nil
		case *types.MapType:
			k, v := t.ItemTypes()
			return types.NewMap(k, v, false), nil
		case *types.PairType:
			return types.NewPair(t.LeftType(), t.RightType(), false), nil
		default:
			return itemType, nil
		}
	}
	return itemType, nil
}

func (f *selectFirstFunction) Call(e *expr.Apply, bindings env.Bindings[values.Base], stdlib *Base) (values.Base, error) {
	arg, err := e.Arguments()[0].Eval(bindings, stdlib)
	if err != nil {
		return nil, err
	}

	arr := arg.(*values.ArrayValue)
	for _, item := range arr.Value() {
		if _, isNull := item.(*values.NullValue); !isNull {
			return item, nil
		}
	}

	return nil, &errors.EvalError{
		BaseError: errors.BaseError{
			Node:    e,
			Message: "select_first() given empty or all-null array; prevent this or append a default value",
		},
	}
}

// selectAllFunction implements select_all(Array[T?]) -> Array[T]
type selectAllFunction struct{}

func (f *selectAllFunction) InferType(e *expr.Apply) (types.Base, error) {
	if len(e.Arguments()) != 1 {
		return nil, &errors.WrongArity{
			BaseError: errors.BaseError{Node: e},
			Expected:  1,
		}
	}

	argType, err := e.Arguments()[0].InferType(env.NewBindings[types.Base]())
	if err != nil {
		return nil, err
	}

	arrType, ok := argType.(*types.ArrayType)
	if !ok || argType.Optional() {
		return nil, &errors.StaticTypeMismatch{
			BaseError: errors.BaseError{Node: e.Arguments()[0]},
			Expected:  types.NewArray(types.NewAny(false), false, false),
			Actual:    argType,
		}
	}

	if _, ok := arrType.ItemType().(*types.AnyType); ok {
		return nil, &errors.IndeterminateType{
			BaseError: errors.BaseError{
				Node:    e.Arguments()[0],
				Message: "can't infer item type of empty array",
			},
		}
	}

	// Return array with non-optional item type
	itemType := arrType.ItemType()
	if itemType.Optional() {
		// Create non-optional version
		switch t := itemType.(type) {
		case *types.IntType:
			return types.NewArray(types.NewInt(false), false, false), nil
		case *types.FloatType:
			return types.NewArray(types.NewFloat(false), false, false), nil
		case *types.StringType:
			return types.NewArray(types.NewString(false), false, false), nil
		case *types.BooleanType:
			return types.NewArray(types.NewBoolean(false), false, false), nil
		case *types.FileType:
			return types.NewArray(types.NewFile(false), false, false), nil
		case *types.DirectoryType:
			return types.NewArray(types.NewDirectory(false), false, false), nil
		case *types.ArrayType:
			return types.NewArray(types.NewArray(t.ItemType(), t.Nonempty(), false), false, false), nil
		default:
			return types.NewArray(itemType, false, false), nil
		}
	}
	return types.NewArray(itemType, false, false), nil
}

func (f *selectAllFunction) Call(e *expr.Apply, bindings env.Bindings[values.Base], stdlib *Base) (values.Base, error) {
	arg, err := e.Arguments()[0].Eval(bindings, stdlib)
	if err != nil {
		return nil, err
	}

	arr := arg.(*values.ArrayValue)
	arrType := arr.Type().(*types.ArrayType)

	var result []values.Base
	for _, item := range arr.Value() {
		if _, isNull := item.(*values.NullValue); !isNull {
			result = append(result, item)
		}
	}

	// Create non-optional item type for result
	itemType := arrType.ItemType()
	if itemType.Optional() {
		// Make non-optional version
		switch t := itemType.(type) {
		case *types.IntType:
			itemType = types.NewInt(false)
		case *types.FloatType:
			itemType = types.NewFloat(false)
		case *types.StringType:
			itemType = types.NewString(false)
		case *types.BooleanType:
			itemType = types.NewBoolean(false)
		case *types.FileType:
			itemType = types.NewFile(false)
		case *types.DirectoryType:
			itemType = types.NewDirectory(false)
		case *types.ArrayType:
			itemType = types.NewArray(t.ItemType(), t.Nonempty(), false)
		default:
			// Keep as-is
		}
	}

	return values.NewArray(itemType, result), nil
}

// zipFunction implements zip(Array[A], Array[B]) -> Array[Pair[A,B]]
type zipFunction struct{}

func (f *zipFunction) InferType(e *expr.Apply) (types.Base, error) {
	if len(e.Arguments()) != 2 {
		return nil, &errors.WrongArity{
			BaseError: errors.BaseError{Node: e},
			Expected:  2,
		}
	}

	lhsType, err := e.Arguments()[0].InferType(env.NewBindings[types.Base]())
	if err != nil {
		return nil, err
	}
	lhsArr, ok := lhsType.(*types.ArrayType)
	if !ok || lhsType.Optional() {
		return nil, &errors.StaticTypeMismatch{
			BaseError: errors.BaseError{Node: e.Arguments()[0]},
			Expected:  types.NewArray(types.NewAny(false), false, false),
			Actual:    lhsType,
		}
	}
	if _, ok := lhsArr.ItemType().(*types.AnyType); ok {
		return nil, &errors.IndeterminateType{
			BaseError: errors.BaseError{
				Node:    e.Arguments()[0],
				Message: "can't infer item type of empty array",
			},
		}
	}

	rhsType, err := e.Arguments()[1].InferType(env.NewBindings[types.Base]())
	if err != nil {
		return nil, err
	}
	rhsArr, ok := rhsType.(*types.ArrayType)
	if !ok || rhsType.Optional() {
		return nil, &errors.StaticTypeMismatch{
			BaseError: errors.BaseError{Node: e.Arguments()[1]},
			Expected:  types.NewArray(types.NewAny(false), false, false),
			Actual:    rhsType,
		}
	}
	if _, ok := rhsArr.ItemType().(*types.AnyType); ok {
		return nil, &errors.IndeterminateType{
			BaseError: errors.BaseError{
				Node:    e.Arguments()[1],
				Message: "can't infer item type of empty array",
			},
		}
	}

	pairType := types.NewPair(lhsArr.ItemType(), rhsArr.ItemType(), false)
	nonempty := lhsArr.Nonempty() || rhsArr.Nonempty()
	return types.NewArray(pairType, nonempty, false), nil
}

func (f *zipFunction) Call(e *expr.Apply, bindings env.Bindings[values.Base], stdlib *Base) (values.Base, error) {
	lhs, err := e.Arguments()[0].Eval(bindings, stdlib)
	if err != nil {
		return nil, err
	}
	lhsArr := lhs.(*values.ArrayValue)

	rhs, err := e.Arguments()[1].Eval(bindings, stdlib)
	if err != nil {
		return nil, err
	}
	rhsArr := rhs.(*values.ArrayValue)

	if len(lhsArr.Value()) != len(rhsArr.Value()) {
		return nil, &errors.EvalError{
			BaseError: errors.BaseError{
				Node:    e,
				Message: "zip(): input arrays must have equal length",
			},
		}
	}

	resultType, _ := f.InferType(e)
	pairType := resultType.(*types.ArrayType).ItemType().(*types.PairType)

	result := make([]values.Base, len(lhsArr.Value()))
	for i := range lhsArr.Value() {
		result[i] = values.NewPair(
			pairType.LeftType(),
			pairType.RightType(),
			lhsArr.Value()[i],
			rhsArr.Value()[i],
		)
	}

	return values.NewArray(pairType, result), nil
}

// crossFunction implements cross(Array[A], Array[B]) -> Array[Pair[A,B]]
type crossFunction struct{}

func (f *crossFunction) InferType(e *expr.Apply) (types.Base, error) {
	// Same type inference as zip
	return (&zipFunction{}).InferType(e)
}

func (f *crossFunction) Call(e *expr.Apply, bindings env.Bindings[values.Base], stdlib *Base) (values.Base, error) {
	lhs, err := e.Arguments()[0].Eval(bindings, stdlib)
	if err != nil {
		return nil, err
	}
	lhsArr := lhs.(*values.ArrayValue)

	rhs, err := e.Arguments()[1].Eval(bindings, stdlib)
	if err != nil {
		return nil, err
	}
	rhsArr := rhs.(*values.ArrayValue)

	resultType, _ := f.InferType(e)
	pairType := resultType.(*types.ArrayType).ItemType().(*types.PairType)

	var result []values.Base
	for _, lhsItem := range lhsArr.Value() {
		for _, rhsItem := range rhsArr.Value() {
			result = append(result, values.NewPair(
				pairType.LeftType(),
				pairType.RightType(),
				lhsItem,
				rhsItem,
			))
		}
	}

	return values.NewArray(pairType, result), nil
}

// unzipFunction implements unzip(Array[Pair[A,B]]) -> Pair[Array[A], Array[B]]
type unzipFunction struct{}

func (f *unzipFunction) InferType(e *expr.Apply) (types.Base, error) {
	if len(e.Arguments()) != 1 {
		return nil, &errors.WrongArity{
			BaseError: errors.BaseError{Node: e},
			Expected:  1,
		}
	}

	argType, err := e.Arguments()[0].InferType(env.NewBindings[types.Base]())
	if err != nil {
		return nil, err
	}

	arrType, ok := argType.(*types.ArrayType)
	if !ok || argType.Optional() {
		return nil, &errors.StaticTypeMismatch{
			BaseError: errors.BaseError{Node: e.Arguments()[0]},
			Expected:  types.NewArray(types.NewPair(types.NewAny(false), types.NewAny(false), false), false, false),
			Actual:    argType,
		}
	}

	pairType, ok := arrType.ItemType().(*types.PairType)
	if !ok || arrType.ItemType().Optional() {
		return nil, &errors.StaticTypeMismatch{
			BaseError: errors.BaseError{Node: e.Arguments()[0]},
			Expected:  types.NewArray(types.NewPair(types.NewAny(false), types.NewAny(false), false), false, false),
			Actual:    argType,
		}
	}

	leftArr := types.NewArray(pairType.LeftType(), arrType.Nonempty(), false)
	rightArr := types.NewArray(pairType.RightType(), arrType.Nonempty(), false)
	return types.NewPair(leftArr, rightArr, false), nil
}

func (f *unzipFunction) Call(e *expr.Apply, bindings env.Bindings[values.Base], stdlib *Base) (values.Base, error) {
	arg, err := e.Arguments()[0].Eval(bindings, stdlib)
	if err != nil {
		return nil, err
	}

	arr := arg.(*values.ArrayValue)
	resultType, _ := f.InferType(e)
	pairType := resultType.(*types.PairType)
	leftArrType := pairType.LeftType().(*types.ArrayType)
	rightArrType := pairType.RightType().(*types.ArrayType)

	leftItems := make([]values.Base, len(arr.Value()))
	rightItems := make([]values.Base, len(arr.Value()))

	for i, item := range arr.Value() {
		pair := item.(*values.PairValue)
		leftItems[i] = pair.Left()
		rightItems[i] = pair.Right()
	}

	leftArr := values.NewArray(leftArrType.ItemType(), leftItems)
	rightArr := values.NewArray(rightArrType.ItemType(), rightItems)

	return values.NewPair(leftArrType, rightArrType, leftArr, rightArr), nil
}

// flattenFunction implements flatten(Array[Array[T]]) -> Array[T]
type flattenFunction struct{}

func (f *flattenFunction) InferType(e *expr.Apply) (types.Base, error) {
	if len(e.Arguments()) != 1 {
		return nil, &errors.WrongArity{
			BaseError: errors.BaseError{Node: e},
			Expected:  1,
		}
	}

	argType, err := e.Arguments()[0].InferType(env.NewBindings[types.Base]())
	if err != nil {
		return nil, err
	}

	if !argType.CheckType(types.NewArray(types.NewAny(false), false, false)) {
		return nil, &errors.StaticTypeMismatch{
			BaseError: errors.BaseError{Node: e.Arguments()[0]},
			Expected:  types.NewArray(types.NewAny(false), false, false),
			Actual:    argType,
		}
	}

	arrType := argType.(*types.ArrayType)
	if _, ok := arrType.ItemType().(*types.AnyType); ok {
		return types.NewArray(types.NewAny(false), false, false), nil
	}

	innerArr, ok := arrType.ItemType().(*types.ArrayType)
	if !ok || arrType.ItemType().Optional() {
		return nil, &errors.StaticTypeMismatch{
			BaseError: errors.BaseError{Node: e.Arguments()[0]},
			Expected:  types.NewArray(types.NewArray(types.NewAny(false), false, false), false, false),
			Actual:    argType,
		}
	}

	return types.NewArray(innerArr.ItemType(), false, false), nil
}

func (f *flattenFunction) Call(e *expr.Apply, bindings env.Bindings[values.Base], stdlib *Base) (values.Base, error) {
	arg, err := e.Arguments()[0].Eval(bindings, stdlib)
	if err != nil {
		return nil, err
	}

	resultType, _ := f.InferType(e)
	itemType := resultType.(*types.ArrayType).ItemType()

	arr := arg.(*values.ArrayValue)
	var result []values.Base

	for _, row := range arr.Value() {
		rowArr := row.(*values.ArrayValue)
		result = append(result, rowArr.Value()...)
	}

	return values.NewArray(itemType, result), nil
}

// transposeFunction implements transpose(Array[Array[T]]) -> Array[Array[T]]
type transposeFunction struct{}

func (f *transposeFunction) InferType(e *expr.Apply) (types.Base, error) {
	if len(e.Arguments()) != 1 {
		return nil, &errors.WrongArity{
			BaseError: errors.BaseError{Node: e},
			Expected:  1,
		}
	}

	argType, err := e.Arguments()[0].InferType(env.NewBindings[types.Base]())
	if err != nil {
		return nil, err
	}

	if !argType.CheckType(types.NewArray(types.NewAny(false), false, false)) {
		return nil, &errors.StaticTypeMismatch{
			BaseError: errors.BaseError{Node: e.Arguments()[0]},
			Expected:  types.NewArray(types.NewAny(false), false, false),
			Actual:    argType,
		}
	}

	arrType := argType.(*types.ArrayType)
	if _, ok := arrType.ItemType().(*types.AnyType); ok {
		return types.NewArray(types.NewAny(false), false, false), nil
	}

	innerArr, ok := arrType.ItemType().(*types.ArrayType)
	if !ok || arrType.ItemType().Optional() {
		return nil, &errors.StaticTypeMismatch{
			BaseError: errors.BaseError{Node: e.Arguments()[0]},
			Expected:  types.NewArray(types.NewArray(types.NewAny(false), false, false), false, false),
			Actual:    argType,
		}
	}

	return types.NewArray(types.NewArray(innerArr.ItemType(), false, false), false, false), nil
}

func (f *transposeFunction) Call(e *expr.Apply, bindings env.Bindings[values.Base], stdlib *Base) (values.Base, error) {
	arg, err := e.Arguments()[0].Eval(bindings, stdlib)
	if err != nil {
		return nil, err
	}

	resultType, _ := f.InferType(e)
	innerType := resultType.(*types.ArrayType).ItemType().(*types.ArrayType)
	itemType := innerType.ItemType()

	mat := arg.(*values.ArrayValue)
	if len(mat.Value()) == 0 {
		return values.NewArray(innerType, []values.Base{}), nil
	}

	// Get dimensions
	firstRow := mat.Value()[0].(*values.ArrayValue)
	numCols := len(firstRow.Value())

	// Check for ragged matrix
	for _, row := range mat.Value() {
		rowArr := row.(*values.ArrayValue)
		if len(rowArr.Value()) != numCols {
			return nil, &errors.EvalError{
				BaseError: errors.BaseError{
					Node:    e,
					Message: "transpose(): ragged input matrix",
				},
			}
		}
	}

	// Create result columns
	result := make([]values.Base, numCols)
	for col := 0; col < numCols; col++ {
		column := make([]values.Base, len(mat.Value()))
		for row := 0; row < len(mat.Value()); row++ {
			rowArr := mat.Value()[row].(*values.ArrayValue)
			column[row] = rowArr.Value()[col]
		}
		result[col] = values.NewArray(itemType, column)
	}

	return values.NewArray(innerType, result), nil
}