package eval

import (
	"fmt"

	"github.com/bioflowy/flowy/wdl/types"
	"github.com/bioflowy/flowy/wdl/values"
)

func init() {
	stdlibFuncs["defined"] = stdDefined
	stdlibFuncs["keys"] = stdKeys
	stdlibFuncs["values"] = stdValues
	stdlibFuncs["as_map"] = stdAsMap
	stdlibFuncs["as_pairs"] = stdAsPairs
	stdlibFuncs["collect_by_key"] = stdCollectByKey
	stdlibFuncs["contains"] = stdContains
	stdlibFuncs["contains_key"] = stdContainsKey
}

// stdDefined returns BooleanValue{true} if the argument is not NullValue, false otherwise.
// WDL signature: Boolean defined(X?)
func stdDefined(args []values.Value, _ ExecContext) (values.Value, error) {
	_, isNull := args[0].(values.NullValue)
	return values.BooleanValue{Type: types.BooleanType{}, Value: !isNull}, nil
}

// stdKeys returns an ArrayValue of map keys in insertion order.
// WDL signature: Array[K] keys(Map[K,V])
func stdKeys(args []values.Value, _ ExecContext) (values.Value, error) {
	mv := args[0].(values.MapValue)
	items := make([]values.Value, len(mv.Entries))
	for i, e := range mv.Entries {
		items[i] = e.Key
	}
	return values.ArrayValue{
		Type:  types.ArrayType{ItemType: mv.Type.KeyType},
		Items: items,
	}, nil
}

// stdValues returns an ArrayValue of map values in insertion order.
// WDL signature: Array[V] values(Map[K,V])
func stdValues(args []values.Value, _ ExecContext) (values.Value, error) {
	mv := args[0].(values.MapValue)
	items := make([]values.Value, len(mv.Entries))
	for i, e := range mv.Entries {
		items[i] = e.Value
	}
	return values.ArrayValue{
		Type:  types.ArrayType{ItemType: mv.Type.ValType},
		Items: items,
	}, nil
}

// stdAsMap converts an Array[Pair[K,V]] to a MapValue preserving order.
// WDL signature: Map[K,V] as_map(Array[Pair[K,V]])
func stdAsMap(args []values.Value, _ ExecContext) (values.Value, error) {
	av := args[0].(values.ArrayValue)
	entries := make([]values.MapEntry, len(av.Items))
	seen := make(map[string]bool)
	var keyType, valType types.Type = types.AnyType{}, types.AnyType{}
	for i, item := range av.Items {
		pv := item.(values.PairValue)
		keyStr := fmt.Sprintf("%v", pv.Left)
		if seen[keyStr] {
			return nil, &EvalError{Message: fmt.Sprintf("as_map: duplicate key: %v", pv.Left)}
		}
		seen[keyStr] = true
		entries[i] = values.MapEntry{Key: pv.Left, Value: pv.Right}
		if i == 0 {
			keyType = pv.Left.ValueType()
			valType = pv.Right.ValueType()
		}
	}
	return values.MapValue{
		Type:    types.MapType{KeyType: keyType, ValType: valType},
		Entries: entries,
	}, nil
}

// stdAsPairs converts a MapValue to an ArrayValue of PairValues in insertion order.
// WDL signature: Array[Pair[K,V]] as_pairs(Map[K,V])
func stdAsPairs(args []values.Value, _ ExecContext) (values.Value, error) {
	mv := args[0].(values.MapValue)
	pairType := types.PairType{LeftType: mv.Type.KeyType, RightType: mv.Type.ValType}
	items := make([]values.Value, len(mv.Entries))
	for i, e := range mv.Entries {
		items[i] = values.PairValue{
			Type:  pairType,
			Left:  e.Key,
			Right: e.Value,
		}
	}
	return values.ArrayValue{
		Type:  types.ArrayType{ItemType: pairType},
		Items: items,
	}, nil
}

// stdCollectByKey groups Pair values by key, producing Map[K, Array[V]].
// WDL signature: Map[K,Array[V]] collect_by_key(Array[Pair[K,V]])
func stdCollectByKey(args []values.Value, _ ExecContext) (values.Value, error) {
	av := args[0].(values.ArrayValue)

	// resultEntries preserves insertion order of first-seen keys.
	var resultEntries []values.MapEntry
	var keyType, itemType types.Type = types.AnyType{}, types.AnyType{}

	for _, item := range av.Items {
		pv := item.(values.PairValue)
		if keyType == (types.AnyType{}) {
			keyType = pv.Left.ValueType()
			itemType = pv.Right.ValueType()
		}

		// Find existing entry with matching key (linear scan using primitiveEqual).
		found := -1
		for j, re := range resultEntries {
			if primitiveEqual(re.Key, pv.Left) {
				found = j
				break
			}
		}
		if found >= 0 {
			// Append to existing ArrayValue for this key.
			existing := resultEntries[found].Value.(values.ArrayValue)
			existing.Items = append(existing.Items, pv.Right)
			resultEntries[found].Value = existing
		} else {
			// New key: create a single-element ArrayValue.
			resultEntries = append(resultEntries, values.MapEntry{
				Key: pv.Left,
				Value: values.ArrayValue{
					Type:  types.ArrayType{ItemType: itemType},
					Items: []values.Value{pv.Right},
				},
			})
		}
	}

	valType := types.ArrayType{ItemType: itemType}
	return values.MapValue{
		Type:    types.MapType{KeyType: keyType, ValType: valType},
		Entries: resultEntries,
	}, nil
}

// stdContains checks whether a primitive value exists in an ArrayValue.
// WDL signature: Boolean contains(Array[P], P)  -- P must be a primitive type
func stdContains(args []values.Value, _ ExecContext) (values.Value, error) {
	av := args[0].(values.ArrayValue)
	search := args[1]
	for _, item := range av.Items {
		if primitiveEqual(item, search) {
			return values.BooleanValue{Type: types.BooleanType{}, Value: true}, nil
		}
	}
	return values.BooleanValue{Type: types.BooleanType{}, Value: false}, nil
}

// stdContainsKey checks whether a key exists in a MapValue (single-key overload).
// WDL signature: Boolean contains_key(Map[K,V], K)
func stdContainsKey(args []values.Value, _ ExecContext) (values.Value, error) {
	mv := args[0].(values.MapValue)
	key := args[1]
	for _, e := range mv.Entries {
		if primitiveEqual(e.Key, key) {
			return values.BooleanValue{Type: types.BooleanType{}, Value: true}, nil
		}
	}
	return values.BooleanValue{Type: types.BooleanType{}, Value: false}, nil
}
