package values

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bioflowy/flowy/pkg/types"
)

// FromJSON creates a WDL value from JSON data
func FromJSON(data json.RawMessage, typ types.Base) (Base, error) {
	// Handle null values
	var nullCheck interface{}
	if err := json.Unmarshal(data, &nullCheck); err == nil && nullCheck == nil {
		if !typ.Optional() {
			return nil, fmt.Errorf("null value for non-optional type %s", typ.String())
		}
		return NewNull(typ), nil
	}

	switch t := typ.(type) {
	case *types.BooleanType:
		var val bool
		if err := json.Unmarshal(data, &val); err != nil {
			return nil, fmt.Errorf("cannot parse Boolean: %v", err)
		}
		return NewBoolean(val, t.Optional()), nil

	case *types.IntType:
		var val int64
		// Try to parse as number first
		if err := json.Unmarshal(data, &val); err != nil {
			// Try as float and convert
			var floatVal float64
			if err := json.Unmarshal(data, &floatVal); err != nil {
				return nil, fmt.Errorf("cannot parse Int: %v", err)
			}
			val = int64(floatVal)
		}
		return NewInt(val, t.Optional()), nil

	case *types.FloatType:
		var val float64
		if err := json.Unmarshal(data, &val); err != nil {
			return nil, fmt.Errorf("cannot parse Float: %v", err)
		}
		return NewFloat(val, t.Optional()), nil

	case *types.StringType:
		var val string
		if err := json.Unmarshal(data, &val); err != nil {
			return nil, fmt.Errorf("cannot parse String: %v", err)
		}
		return NewString(val, t.Optional()), nil

	case *types.FileType:
		var val string
		if err := json.Unmarshal(data, &val); err != nil {
			return nil, fmt.Errorf("cannot parse File: %v", err)
		}
		return NewFile(val, t.Optional()), nil

	case *types.DirectoryType:
		var val string
		if err := json.Unmarshal(data, &val); err != nil {
			return nil, fmt.Errorf("cannot parse Directory: %v", err)
		}
		return NewDirectory(val, t.Optional()), nil

	case *types.ArrayType:
		var items []json.RawMessage
		if err := json.Unmarshal(data, &items); err != nil {
			return nil, fmt.Errorf("cannot parse Array: %v", err)
		}
		array := NewArray(t.ItemType(), t.Optional(), t.NonEmpty())
		for i, itemData := range items {
			item, err := FromJSON(itemData, t.ItemType())
			if err != nil {
				return nil, fmt.Errorf("cannot parse array item %d: %v", i, err)
			}
			array.Add(item)
		}
		if t.NonEmpty() && len(array.Items()) == 0 {
			return nil, fmt.Errorf("empty array for non-empty type %s", t.String())
		}
		return array, nil

	case *types.MapType:
		var entries map[string]json.RawMessage
		if err := json.Unmarshal(data, &entries); err != nil {
			return nil, fmt.Errorf("cannot parse Map: %v", err)
		}
		mapVal := NewMap(t.KeyType(), t.ValueType(), t.Optional())
		for k, valueData := range entries {
			value, err := FromJSON(valueData, t.ValueType())
			if err != nil {
				return nil, fmt.Errorf("cannot parse map value for key %s: %v", k, err)
			}
			mapVal.Set(k, value)
		}
		return mapVal, nil

	case *types.PairType:
		var items []json.RawMessage
		if err := json.Unmarshal(data, &items); err != nil {
			return nil, fmt.Errorf("cannot parse Pair: %v", err)
		}
		if len(items) != 2 {
			return nil, fmt.Errorf("pair must have exactly 2 elements, got %d", len(items))
		}
		left, err := FromJSON(items[0], t.LeftType())
		if err != nil {
			return nil, fmt.Errorf("cannot parse pair left value: %v", err)
		}
		right, err := FromJSON(items[1], t.RightType())
		if err != nil {
			return nil, fmt.Errorf("cannot parse pair right value: %v", err)
		}
		return NewPair(t.LeftType(), t.RightType(), left, right, t.Optional()), nil

	case *types.StructInstanceType:
		var members map[string]json.RawMessage
		if err := json.Unmarshal(data, &members); err != nil {
			return nil, fmt.Errorf("cannot parse Struct: %v", err)
		}
		structMembers := make(map[string]Base)
		for name, memberType := range t.Members() {
			if memberData, ok := members[name]; ok {
				member, err := FromJSON(memberData, memberType)
				if err != nil {
					return nil, fmt.Errorf("cannot parse struct member %s: %v", name, err)
				}
				structMembers[name] = member
			} else if !memberType.Optional() {
				return nil, fmt.Errorf("missing required struct member %s", name)
			}
		}
		return NewStruct(t.TypeName(), t.Members(), structMembers, t.Optional()), nil

	case *types.ObjectType:
		var members map[string]json.RawMessage
		if err := json.Unmarshal(data, &members); err != nil {
			return nil, fmt.Errorf("cannot parse Object: %v", err)
		}
		objectMembers := make(map[string]Base)
		for k, v := range members {
			// Object members can be any type, try to infer
			member, err := inferValueFromJSON(v)
			if err != nil {
				return nil, fmt.Errorf("cannot parse object member %s: %v", k, err)
			}
			objectMembers[k] = member
		}
		return NewObject(objectMembers, t.Optional()), nil

	default:
		return nil, fmt.Errorf("unsupported type for JSON parsing: %s", typ.String())
	}
}

// inferValueFromJSON attempts to infer the type of a JSON value
func inferValueFromJSON(data json.RawMessage) (Base, error) {
	var val interface{}
	if err := json.Unmarshal(data, &val); err != nil {
		return nil, err
	}

	switch v := val.(type) {
	case nil:
		return NewNull(types.NewAny(true, true)), nil
	case bool:
		return NewBoolean(v, false), nil
	case float64:
		// Check if it's actually an integer
		if v == float64(int64(v)) {
			return NewInt(int64(v), false), nil
		}
		return NewFloat(v, false), nil
	case string:
		return NewString(v, false), nil
	case []interface{}:
		// Infer as Array[Any]
		items := make([]Base, len(v))
		for i, item := range v {
			itemData, _ := json.Marshal(item)
			itemVal, err := inferValueFromJSON(itemData)
			if err != nil {
				return nil, err
			}
			items[i] = itemVal
		}
		return NewArrayWithItems(types.NewAny(false, false), items, false, false), nil
	case map[string]interface{}:
		// Infer as Object
		members := make(map[string]Base)
		for k, v := range v {
			memberData, _ := json.Marshal(v)
			member, err := inferValueFromJSON(memberData)
			if err != nil {
				return nil, err
			}
			members[k] = member
		}
		return NewObject(members, false), nil
	default:
		return nil, fmt.Errorf("cannot infer type for value: %v", v)
	}
}

// RewritePaths rewrites file/directory paths in a value
func RewritePaths(value Base, rewriter func(string) string) Base {
	switch v := value.(type) {
	case *FileValue:
		return NewFile(rewriter(v.value), v.Type().Optional())
	case *DirectoryValue:
		return NewDirectory(rewriter(v.value), v.Type().Optional())
	case *ArrayValue:
		items := make([]Base, len(v.items))
		for i, item := range v.items {
			items[i] = RewritePaths(item, rewriter)
		}
		if arrayType, ok := v.Type().(*types.ArrayType); ok {
			return NewArrayWithItems(arrayType.ItemType(), items, arrayType.Optional(), arrayType.NonEmpty())
		}
		return v
	case *MapValue:
		entries := make(map[string]Base)
		for k, val := range v.entries {
			entries[k] = RewritePaths(val, rewriter)
		}
		if mapType, ok := v.Type().(*types.MapType); ok {
			return NewMapWithEntries(mapType.KeyType(), mapType.ValueType(), entries, mapType.Optional())
		}
		return v
	case *PairValue:
		left := RewritePaths(v.left, rewriter)
		right := RewritePaths(v.right, rewriter)
		if pairType, ok := v.Type().(*types.PairType); ok {
			return NewPair(pairType.LeftType(), pairType.RightType(), left, right, pairType.Optional())
		}
		return v
	case *StructValue:
		members := make(map[string]Base)
		for k, val := range v.members {
			members[k] = RewritePaths(val, rewriter)
		}
		if structType, ok := v.Type().(*types.StructInstanceType); ok {
			return NewStruct(structType.TypeName(), structType.Members(), members, structType.Optional())
		}
		return v
	case *ObjectValue:
		members := make(map[string]Base)
		for k, val := range v.members {
			members[k] = RewritePaths(val, rewriter)
		}
		return NewObject(members, v.Type().Optional())
	default:
		return value
	}
}

// PathRewriter is a function that rewrites paths
type PathRewriter func(string) string

// MakePathRewriter creates a path rewriter that maps paths from one directory to another
func MakePathRewriter(fromDir, toDir string) PathRewriter {
	return func(path string) string {
		if filepath.IsAbs(path) {
			if strings.HasPrefix(path, fromDir) {
				relPath, _ := filepath.Rel(fromDir, path)
				return filepath.Join(toDir, relPath)
			}
		}
		return path
	}
}
