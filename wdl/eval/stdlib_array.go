package eval

import (
	"fmt"

	"github.com/bioflowy/flowy/wdl/types"
	"github.com/bioflowy/flowy/wdl/values"
)

// init registers all STDLIB-03 (array) functions into the stdlibFuncs dispatch map.
func init() {
	stdlibFuncs["length"]       = stdLength
	stdlibFuncs["flatten"]      = stdFlatten
	stdlibFuncs["transpose"]    = stdTranspose
	stdlibFuncs["zip"]          = stdZip
	stdlibFuncs["cross"]        = stdCross
	stdlibFuncs["prefix"]       = stdPrefix
	stdlibFuncs["suffix"]       = stdSuffix
	stdlibFuncs["quote"]        = stdQuote
	stdlibFuncs["squote"]       = stdSquote
	stdlibFuncs["unzip"]        = stdUnzip
	stdlibFuncs["select_all"]   = stdSelectAll
	stdlibFuncs["select_first"] = stdSelectFirst
	stdlibFuncs["chunk"]        = stdChunk
}

// valueToStr returns the string form of a WDL value for prefix/suffix/quote/squote operations.
func valueToStr(v values.Value) string {
	switch x := v.(type) {
	case values.StringValue:
		return x.Value
	case values.IntValue:
		return fmt.Sprintf("%d", x.Value)
	case values.FloatValue:
		return fmt.Sprintf("%g", x.Value)
	case values.BooleanValue:
		if x.Value {
			return "true"
		}
		return "false"
	case values.FileValue:
		return x.Value
	default:
		return fmt.Sprintf("%v", v)
	}
}

// stdLength implements length(Array[X]) -> Int, length(Map[K,V]) -> Int, length(String) -> Int.
func stdLength(args []values.Value, ctx ExecContext) (values.Value, error) {
	if len(args) < 1 {
		return nil, &EvalError{Message: "length: expected argument"}
	}
	switch v := args[0].(type) {
	case values.ArrayValue:
		return values.IntValue{Type: types.IntType{}, Value: int64(len(v.Items))}, nil
	case values.MapValue:
		return values.IntValue{Type: types.IntType{}, Value: int64(len(v.Entries))}, nil
	case values.StringValue:
		return values.IntValue{Type: types.IntType{}, Value: int64(len([]rune(v.Value)))}, nil
	default:
		return nil, &EvalError{Message: fmt.Sprintf("length: expected Array, Map, or String, got %T", args[0])}
	}
}

// stdFlatten implements flatten(Array[Array[X]]) -> Array[X].
// Concatenates all inner arrays into a single flat array.
func stdFlatten(args []values.Value, ctx ExecContext) (values.Value, error) {
	if len(args) < 1 {
		return nil, &EvalError{Message: "flatten: expected Array[Array[X]] argument"}
	}
	outer, ok := args[0].(values.ArrayValue)
	if !ok {
		return nil, &EvalError{Message: fmt.Sprintf("flatten: expected ArrayValue, got %T", args[0])}
	}
	var items []values.Value
	var itemType types.Type = types.AnyType{}
	for _, elem := range outer.Items {
		inner, ok := elem.(values.ArrayValue)
		if !ok {
			return nil, &EvalError{Message: fmt.Sprintf("flatten: inner element is not ArrayValue, got %T", elem)}
		}
		if len(inner.Items) > 0 {
			itemType = inner.Items[0].ValueType()
		}
		items = append(items, inner.Items...)
	}
	if items == nil {
		items = []values.Value{}
	}
	return values.ArrayValue{
		Type:  types.ArrayType{ItemType: itemType},
		Items: items,
	}, nil
}

// stdTranspose implements transpose(Array[Array[X]]) -> Array[Array[X]].
// Treats the input as a matrix and returns its transpose.
// Jagged input: uses the first row length as the column count.
func stdTranspose(args []values.Value, ctx ExecContext) (values.Value, error) {
	if len(args) < 1 {
		return nil, &EvalError{Message: "transpose: expected Array[Array[X]] argument"}
	}
	outer, ok := args[0].(values.ArrayValue)
	if !ok {
		return nil, &EvalError{Message: fmt.Sprintf("transpose: expected ArrayValue, got %T", args[0])}
	}
	if len(outer.Items) == 0 {
		return values.ArrayValue{Type: outer.Type, Items: []values.Value{}}, nil
	}
	// Determine number of columns from first row.
	row0, ok := outer.Items[0].(values.ArrayValue)
	if !ok {
		return nil, &EvalError{Message: "transpose: inner elements must be ArrayValue"}
	}
	cols := len(row0.Items)
	rows := len(outer.Items)
	// Build rows x cols matrix
	matrix := make([]values.ArrayValue, rows)
	for i, rowVal := range outer.Items {
		row, ok := rowVal.(values.ArrayValue)
		if !ok {
			return nil, &EvalError{Message: "transpose: inner elements must be ArrayValue"}
		}
		matrix[i] = row
	}
	// Produce transposed result: cols rows, each of length rows
	result := make([]values.Value, cols)
	for c := 0; c < cols; c++ {
		colItems := make([]values.Value, 0, rows)
		for r := 0; r < rows; r++ {
			if c < len(matrix[r].Items) {
				colItems = append(colItems, matrix[r].Items[c])
			}
		}
		var itemType types.Type = types.AnyType{}
		if len(colItems) > 0 {
			itemType = colItems[0].ValueType()
		}
		result[c] = values.ArrayValue{
			Type:  types.ArrayType{ItemType: itemType},
			Items: colItems,
		}
	}
	return values.ArrayValue{
		Type:  outer.Type,
		Items: result,
	}, nil
}

// stdZip implements zip(Array[X], Array[Y]) -> Array[Pair[X,Y]].
// Zips two arrays element-wise; uses the length of the shorter array.
func stdZip(args []values.Value, ctx ExecContext) (values.Value, error) {
	if len(args) < 2 {
		return nil, &EvalError{Message: "zip: expected two Array arguments"}
	}
	a1, ok1 := args[0].(values.ArrayValue)
	a2, ok2 := args[1].(values.ArrayValue)
	if !ok1 || !ok2 {
		return nil, &EvalError{Message: "zip: both arguments must be ArrayValue"}
	}
	n := len(a1.Items)
	if len(a2.Items) < n {
		n = len(a2.Items)
	}
	items := make([]values.Value, n)
	for i := 0; i < n; i++ {
		l := a1.Items[i]
		r := a2.Items[i]
		items[i] = values.PairValue{
			Type:  types.PairType{LeftType: l.ValueType(), RightType: r.ValueType()},
			Left:  l,
			Right: r,
		}
	}
	var pairType types.Type = types.AnyType{}
	if n > 0 {
		pairType = items[0].ValueType()
	}
	return values.ArrayValue{
		Type:  types.ArrayType{ItemType: pairType},
		Items: items,
	}, nil
}

// stdCross implements cross(Array[X], Array[Y]) -> Array[Pair[X,Y]].
// Returns the Cartesian product: for each x in X, for each y in Y, emit Pair(x,y).
func stdCross(args []values.Value, ctx ExecContext) (values.Value, error) {
	if len(args) < 2 {
		return nil, &EvalError{Message: "cross: expected two Array arguments"}
	}
	a1, ok1 := args[0].(values.ArrayValue)
	a2, ok2 := args[1].(values.ArrayValue)
	if !ok1 || !ok2 {
		return nil, &EvalError{Message: "cross: both arguments must be ArrayValue"}
	}
	items := make([]values.Value, 0, len(a1.Items)*len(a2.Items))
	for _, x := range a1.Items {
		for _, y := range a2.Items {
			items = append(items, values.PairValue{
				Type:  types.PairType{LeftType: x.ValueType(), RightType: y.ValueType()},
				Left:  x,
				Right: y,
			})
		}
	}
	var pairType types.Type = types.AnyType{}
	if len(items) > 0 {
		pairType = items[0].ValueType()
	}
	return values.ArrayValue{
		Type:  types.ArrayType{ItemType: pairType},
		Items: items,
	}, nil
}

// stdPrefix implements prefix(String prefix, Array[P]) -> Array[String].
// Prepends the given prefix string to the string form of each array element.
func stdPrefix(args []values.Value, ctx ExecContext) (values.Value, error) {
	if len(args) < 2 {
		return nil, &EvalError{Message: "prefix: expected (String, Array) arguments"}
	}
	pfx, ok := args[0].(values.StringValue)
	if !ok {
		return nil, &EvalError{Message: fmt.Sprintf("prefix: first argument must be StringValue, got %T", args[0])}
	}
	av, ok := args[1].(values.ArrayValue)
	if !ok {
		return nil, &EvalError{Message: fmt.Sprintf("prefix: second argument must be ArrayValue, got %T", args[1])}
	}
	items := make([]values.Value, len(av.Items))
	for i, item := range av.Items {
		items[i] = values.StringValue{Type: types.StringType{}, Value: pfx.Value + valueToStr(item)}
	}
	return values.ArrayValue{
		Type:  types.ArrayType{ItemType: types.StringType{}},
		Items: items,
	}, nil
}

// stdSuffix implements suffix(String suffix, Array[P]) -> Array[String].
// Appends the given suffix string to the string form of each array element.
func stdSuffix(args []values.Value, ctx ExecContext) (values.Value, error) {
	if len(args) < 2 {
		return nil, &EvalError{Message: "suffix: expected (String, Array) arguments"}
	}
	sfx, ok := args[0].(values.StringValue)
	if !ok {
		return nil, &EvalError{Message: fmt.Sprintf("suffix: first argument must be StringValue, got %T", args[0])}
	}
	av, ok := args[1].(values.ArrayValue)
	if !ok {
		return nil, &EvalError{Message: fmt.Sprintf("suffix: second argument must be ArrayValue, got %T", args[1])}
	}
	items := make([]values.Value, len(av.Items))
	for i, item := range av.Items {
		items[i] = values.StringValue{Type: types.StringType{}, Value: valueToStr(item) + sfx.Value}
	}
	return values.ArrayValue{
		Type:  types.ArrayType{ItemType: types.StringType{}},
		Items: items,
	}, nil
}

// stdQuote implements quote(Array[P]) -> Array[String].
// Wraps each element's string form in double quotes.
func stdQuote(args []values.Value, ctx ExecContext) (values.Value, error) {
	if len(args) < 1 {
		return nil, &EvalError{Message: "quote: expected Array argument"}
	}
	av, ok := args[0].(values.ArrayValue)
	if !ok {
		return nil, &EvalError{Message: fmt.Sprintf("quote: expected ArrayValue, got %T", args[0])}
	}
	items := make([]values.Value, len(av.Items))
	for i, item := range av.Items {
		items[i] = values.StringValue{Type: types.StringType{}, Value: `"` + valueToStr(item) + `"`}
	}
	return values.ArrayValue{
		Type:  types.ArrayType{ItemType: types.StringType{}},
		Items: items,
	}, nil
}

// stdSquote implements squote(Array[P]) -> Array[String].
// Wraps each element's string form in single quotes.
func stdSquote(args []values.Value, ctx ExecContext) (values.Value, error) {
	if len(args) < 1 {
		return nil, &EvalError{Message: "squote: expected Array argument"}
	}
	av, ok := args[0].(values.ArrayValue)
	if !ok {
		return nil, &EvalError{Message: fmt.Sprintf("squote: expected ArrayValue, got %T", args[0])}
	}
	items := make([]values.Value, len(av.Items))
	for i, item := range av.Items {
		items[i] = values.StringValue{Type: types.StringType{}, Value: "'" + valueToStr(item) + "'"}
	}
	return values.ArrayValue{
		Type:  types.ArrayType{ItemType: types.StringType{}},
		Items: items,
	}, nil
}

// stdUnzip implements unzip(Array[Pair[X,Y]]) -> Pair[Array[X], Array[Y]].
// Splits an array of pairs into two parallel arrays.
func stdUnzip(args []values.Value, ctx ExecContext) (values.Value, error) {
	if len(args) < 1 {
		return nil, &EvalError{Message: "unzip: expected Array[Pair] argument"}
	}
	av, ok := args[0].(values.ArrayValue)
	if !ok {
		return nil, &EvalError{Message: fmt.Sprintf("unzip: expected ArrayValue, got %T", args[0])}
	}
	lefts := make([]values.Value, 0, len(av.Items))
	rights := make([]values.Value, 0, len(av.Items))
	for _, item := range av.Items {
		pv, ok := item.(values.PairValue)
		if !ok {
			return nil, &EvalError{Message: fmt.Sprintf("unzip: expected PairValue elements, got %T", item)}
		}
		lefts = append(lefts, pv.Left)
		rights = append(rights, pv.Right)
	}
	var leftItemType, rightItemType types.Type = types.AnyType{}, types.AnyType{}
	if len(lefts) > 0 {
		leftItemType = lefts[0].ValueType()
		rightItemType = rights[0].ValueType()
	}
	leftArr := values.ArrayValue{Type: types.ArrayType{ItemType: leftItemType}, Items: lefts}
	rightArr := values.ArrayValue{Type: types.ArrayType{ItemType: rightItemType}, Items: rights}
	return values.PairValue{
		Type:  types.PairType{LeftType: leftArr.ValueType(), RightType: rightArr.ValueType()},
		Left:  leftArr,
		Right: rightArr,
	}, nil
}

// stdSelectAll implements select_all(Array[X?]) -> Array[X].
// Filters out all NullValue entries from the array.
func stdSelectAll(args []values.Value, ctx ExecContext) (values.Value, error) {
	if len(args) < 1 {
		return nil, &EvalError{Message: "select_all: expected Array argument"}
	}
	av, ok := args[0].(values.ArrayValue)
	if !ok {
		return nil, &EvalError{Message: fmt.Sprintf("select_all: expected ArrayValue, got %T", args[0])}
	}
	items := make([]values.Value, 0, len(av.Items))
	for _, item := range av.Items {
		if _, isNull := item.(values.NullValue); !isNull {
			items = append(items, item)
		}
	}
	return values.ArrayValue{
		Type:  av.Type,
		Items: items,
	}, nil
}

// stdSelectFirst implements select_first(Array[X?]) -> X.
// Returns the first non-NullValue item; returns EvalError if all items are null.
func stdSelectFirst(args []values.Value, ctx ExecContext) (values.Value, error) {
	if len(args) < 1 {
		return nil, &EvalError{Message: "select_first: expected Array argument"}
	}
	av, ok := args[0].(values.ArrayValue)
	if !ok {
		return nil, &EvalError{Message: fmt.Sprintf("select_first: expected ArrayValue, got %T", args[0])}
	}
	for _, item := range av.Items {
		if _, isNull := item.(values.NullValue); !isNull {
			return item, nil
		}
	}
	return nil, &EvalError{Message: "select_first: all elements are null"}
}

// stdChunk implements chunk(Array[X], Int n) -> Array[Array[X]].
// Splits the array into sub-arrays of at most n elements.
// Returns EvalError if n <= 0.
func stdChunk(args []values.Value, ctx ExecContext) (values.Value, error) {
	if len(args) < 2 {
		return nil, &EvalError{Message: "chunk: expected (Array, Int) arguments"}
	}
	av, ok := args[0].(values.ArrayValue)
	if !ok {
		return nil, &EvalError{Message: fmt.Sprintf("chunk: first argument must be ArrayValue, got %T", args[0])}
	}
	nv, ok := args[1].(values.IntValue)
	if !ok {
		return nil, &EvalError{Message: fmt.Sprintf("chunk: second argument must be IntValue, got %T", args[1])}
	}
	n := nv.Value
	if n <= 0 {
		return nil, &EvalError{Message: fmt.Sprintf("chunk: chunk size must be > 0, got %d", n)}
	}
	chunks := make([]values.Value, 0)
	total := int64(len(av.Items))
	for i := int64(0); i < total; i += n {
		end := i + n
		if end > total {
			end = total
		}
		chunk := av.Items[i:end]
		chunks = append(chunks, values.ArrayValue{
			Type:  av.Type,
			Items: chunk,
		})
	}
	return values.ArrayValue{
		Type:  types.ArrayType{ItemType: av.Type},
		Items: chunks,
	}, nil
}
