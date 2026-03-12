package task

import (
	"fmt"
	"math"
	"path/filepath"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/eval"
	"github.com/bioflowy/flowy/wdl/types"
	"github.com/bioflowy/flowy/wdl/values"
)

// BindInputs converts a JSON input map into a typed WDL eval.Env and populates
// the given PathMap with all File/Directory values found in the inputs.
//
// For each input declaration in task.Inputs:
//   - If the JSON map contains the key (bare or "taskName.field"): convert using jsonToWDLValue (type-aware)
//   - If absent and decl has a default expr: evaluate the default via eval.Eval
//   - If absent, no default, and type is optional: bind NullValue{}
//   - If absent, no default, and type is required: return error
//
// Key lookup follows the WDL convention: first try "taskName.fieldName" (as produced
// by spectool and other conformance tools), then fall back to the bare field name.
//
// After binding a value, collectPaths walks it to register all File/Directory
// paths in the PathMap.
//
// Returns: (env with all bindings, env-var strings for env=true inputs, error)
func BindInputs(task *ast.Task, jsonInputs map[string]interface{}, pm *PathMap, wdlDir string, structDefs ...map[string]*ast.StructDef) (*eval.Env, []string, error) {
	var sd map[string]*ast.StructDef
	if len(structDefs) > 0 {
		sd = structDefs[0]
	}
	env := eval.NewEnv(nil)
	var envVars []string

	for _, decl := range task.Inputs {
		var val values.Value

		// Try "taskName.fieldName" first (spectool / WDL convention), then bare name.
		raw, found := jsonInputs[task.Name+"."+decl.Name]
		if !found {
			raw, found = jsonInputs[decl.Name]
		}
		if found {
			// Convert JSON value using declaration type for type-aware coercion
			converted, err := jsonToWDLValue(raw, decl.Type, sd, wdlDir)
			if err != nil {
				return nil, nil, fmt.Errorf("input %q: %w", decl.Name, err)
			}
			val = converted
		} else if decl.Expr != nil {
			// Evaluate default expression
			v, err := eval.Eval(decl.Expr, env, eval.ExecContext{})
			if err != nil {
				return nil, nil, fmt.Errorf("input %q default expression: %w", decl.Name, err)
			}
			val = v
		} else if decl.Type.IsOptional() {
			// Optional input with no value provided and no default
			val = values.NullValue{}
		} else {
			// Required input not provided
			return nil, nil, fmt.Errorf("missing required input: %q", decl.Name)
		}

		// Collect File/Directory paths into the PathMap
		collectPaths(val, pm)

		// Add to env
		env.Define(decl.Name, val)

		// If env=true modifier, add to env var list
		if decl.Env {
			envVars = append(envVars, decl.Name+"="+valueToEnvString(val))
		}
	}

	return env, envVars, nil
}

// jsonToWDLValue converts a JSON-decoded value (from json.Unmarshal) to a WDL Value,
// using the declaration type to guide type-specific conversions.
//
// Key differences from eval.jsonToValue:
//   - string -> FileValue when declType is FileType
//   - string -> DirectoryValue when declType is DirectoryType
//   - float64 -> IntValue when declType is IntType (always truncates)
//   - For compound types (Array, Map, Pair): recurse with item/key/value types
//   - Falls back to untyped conversion for AnyType or unrecognized types
func jsonToWDLValue(v interface{}, declType types.Type, structDefs map[string]*ast.StructDef, wdlDir string) (values.Value, error) {
	if v == nil {
		return values.NullValue{}, nil
	}

	// Strip optional wrapper to get base type for switch
	baseType := declType
	// (We use the concrete type regardless of Optional flag for conversion)

	switch dt := baseType.(type) {
	case types.FileType:
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("expected string for File, got %T", v)
		}
		// Resolve relative paths to absolute and canonicalize.
		s = cleanPath(s, wdlDir)
		return values.FileValue{Type: dt, Value: s}, nil

	case types.DirectoryType:
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("expected string for Directory, got %T", v)
		}
		// Resolve relative paths to absolute and canonicalize.
		s = cleanPath(s, wdlDir)
		return values.DirectoryValue{Type: dt, Value: s}, nil

	case types.StringType:
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("expected string for String, got %T", v)
		}
		return values.StringValue{Type: dt, Value: s}, nil

	case types.IntType:
		switch x := v.(type) {
		case float64:
			return values.IntValue{Type: dt, Value: int64(x)}, nil
		case int64:
			return values.IntValue{Type: dt, Value: x}, nil
		case int:
			return values.IntValue{Type: dt, Value: int64(x)}, nil
		default:
			return nil, fmt.Errorf("expected number for Int, got %T", v)
		}

	case types.FloatType:
		switch x := v.(type) {
		case float64:
			return values.FloatValue{Type: dt, Value: x}, nil
		case int64:
			return values.FloatValue{Type: dt, Value: float64(x)}, nil
		default:
			return nil, fmt.Errorf("expected number for Float, got %T", v)
		}

	case types.BooleanType:
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("expected bool for Boolean, got %T", v)
		}
		return values.BooleanValue{Type: dt, Value: b}, nil

	case types.ArrayType:
		arr, ok := v.([]interface{})
		if !ok {
			return nil, fmt.Errorf("expected array for Array[...], got %T", v)
		}
		items := make([]values.Value, len(arr))
		for i, elem := range arr {
			item, err := jsonToWDLValue(elem, dt.ItemType, structDefs, wdlDir)
			if err != nil {
				return nil, fmt.Errorf("array item %d: %w", i, err)
			}
			items[i] = item
		}
		return values.ArrayValue{Type: dt, Items: items}, nil

	case types.MapType:
		m, ok := v.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected object for Map[...], got %T", v)
		}
		entries := make([]values.MapEntry, 0, len(m))
		for k, mv := range m {
			kv, err := jsonToWDLValue(k, dt.KeyType, structDefs, wdlDir)
			if err != nil {
				return nil, fmt.Errorf("map key %q: %w", k, err)
			}
			vv, err := jsonToWDLValue(mv, dt.ValType, structDefs, wdlDir)
			if err != nil {
				return nil, fmt.Errorf("map value for key %q: %w", k, err)
			}
			entries = append(entries, values.MapEntry{Key: kv, Value: vv})
		}
		return values.MapValue{Type: dt, Entries: entries}, nil

	case types.PairType:
		m, ok := v.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected object for Pair, got %T", v)
		}
		left, err := jsonToWDLValue(m["left"], dt.LeftType, structDefs, wdlDir)
		if err != nil {
			return nil, fmt.Errorf("pair left: %w", err)
		}
		right, err := jsonToWDLValue(m["right"], dt.RightType, structDefs, wdlDir)
		if err != nil {
			return nil, fmt.Errorf("pair right: %w", err)
		}
		return values.PairValue{Type: dt, Left: left, Right: right}, nil

	case types.StructType:
		m, ok := v.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected object for Struct %q, got %T", dt.Name, v)
		}
		fields := make(map[string]values.Value, len(m))
		if sd, hasDef := structDefs[dt.Name]; hasDef {
			for _, member := range sd.Members {
				if fv, exists := m[member.Name]; exists {
					val, err := jsonToWDLValue(fv, member.Type, structDefs, wdlDir)
					if err != nil {
						return nil, fmt.Errorf("struct %q field %q: %w", dt.Name, member.Name, err)
					}
					fields[member.Name] = val
				} else if member.Type.IsOptional() {
					fields[member.Name] = values.NullValue{}
				}
			}
		} else {
			for k, fv := range m {
				fields[k] = jsonToValueUntyped(fv)
			}
		}
		return values.StructValue{Type: dt, Fields: fields}, nil

	default:
		// Fall back to untyped conversion (AnyType or unknown type)
		return jsonToValueUntyped(v), nil
	}
}

// jsonToValueUntyped is the untyped fallback for JSON-to-WDL conversion,
// equivalent to eval.jsonToValue. Used for AnyType and compound type fallback.
func jsonToValueUntyped(v interface{}) values.Value {
	switch x := v.(type) {
	case bool:
		return values.BooleanValue{Type: types.BooleanType{}, Value: x}
	case float64:
		if math.Trunc(x) == x && !math.IsInf(x, 0) {
			return values.IntValue{Type: types.IntType{}, Value: int64(x)}
		}
		return values.FloatValue{Type: types.FloatType{}, Value: x}
	case string:
		return values.StringValue{Type: types.StringType{}, Value: x}
	case []interface{}:
		items := make([]values.Value, len(x))
		for i, elem := range x {
			items[i] = jsonToValueUntyped(elem)
		}
		var itemType types.Type = types.AnyType{}
		if len(items) > 0 {
			itemType = items[0].ValueType()
		}
		return values.ArrayValue{
			Type:  types.ArrayType{ItemType: itemType},
			Items: items,
		}
	case map[string]interface{}:
		entries := make([]values.MapEntry, 0, len(x))
		for k, val := range x {
			entries = append(entries, values.MapEntry{
				Key:   values.StringValue{Type: types.StringType{}, Value: k},
				Value: jsonToValueUntyped(val),
			})
		}
		return values.MapValue{
			Type:    types.MapType{KeyType: types.StringType{}, ValType: types.AnyType{}},
			Entries: entries,
		}
	default:
		return values.NullValue{}
	}
}

// cleanPath resolves relative paths to absolute and canonicalizes ".." components.
// If wdlDir is non-empty, relative paths are resolved relative to it (the WDL
// file's parent directory, per WDL 1.2 spec). Otherwise, falls back to CWD.
func cleanPath(p string, wdlDir string) string {
	if !filepath.IsAbs(p) {
		if wdlDir != "" {
			return filepath.Clean(filepath.Join(wdlDir, p))
		}
		if abs, err := filepath.Abs(p); err == nil {
			return filepath.Clean(abs)
		}
	}
	return filepath.Clean(p)
}

// collectPaths walks a WDL value and registers all FileValue and DirectoryValue
// paths in the PathMap. Compound values (Array, Map, Pair, Struct) are traversed
// recursively.
func collectPaths(v values.Value, pm *PathMap) {
	switch val := v.(type) {
	case values.FileValue:
		pm.AddFile(val.Value)
	case values.DirectoryValue:
		pm.AddDir(val.Value)
	case values.ArrayValue:
		for _, item := range val.Items {
			collectPaths(item, pm)
		}
	case values.MapValue:
		for _, e := range val.Entries {
			collectPaths(e.Key, pm)
			collectPaths(e.Value, pm)
		}
	case values.PairValue:
		collectPaths(val.Left, pm)
		collectPaths(val.Right, pm)
	case values.StructValue:
		for _, fv := range val.Fields {
			collectPaths(fv, pm)
		}
	}
}

// valueToEnvString converts a WDL value to a string suitable for use in an
// environment variable assignment. Uses simple fmt.Sprintf for stringification.
func valueToEnvString(v values.Value) string {
	switch val := v.(type) {
	case values.StringValue:
		return val.Value
	case values.IntValue:
		return fmt.Sprintf("%d", val.Value)
	case values.FloatValue:
		return fmt.Sprintf("%g", val.Value)
	case values.BooleanValue:
		if val.Value {
			return "true"
		}
		return "false"
	case values.FileValue:
		return val.Value
	case values.DirectoryValue:
		return val.Value
	default:
		return fmt.Sprintf("%v", v)
	}
}
