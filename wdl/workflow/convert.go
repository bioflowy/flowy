package workflow

import (
	"fmt"
	"path/filepath"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/eval"
	"github.com/bioflowy/flowy/wdl/types"
	"github.com/bioflowy/flowy/wdl/values"
)

// valueToJSON converts a WDL values.Value to a Go primitive suitable for use
// in the map[string]interface{} input expected by task.RunTask.
//
// Conversion rules:
//   - IntValue      → int64
//   - FloatValue    → float64
//   - StringValue   → string
//   - BooleanValue  → bool
//   - FileValue     → string (host path)
//   - DirectoryValue → string (host path)
//   - NullValue     → nil
//   - ArrayValue    → []interface{} (recursive)
//   - MapValue      → map[string]interface{} (key converted via valueToJSON, must be stringifiable)
//   - PairValue     → map[string]interface{}{"left": ..., "right": ...}
//   - StructValue   → map[string]interface{} (field name → value)
func valueToJSON(v values.Value) interface{} {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case values.IntValue:
		return val.Value
	case values.FloatValue:
		return val.Value
	case values.StringValue:
		return val.Value
	case values.BooleanValue:
		return val.Value
	case values.FileValue:
		return val.Value
	case values.DirectoryValue:
		return val.Value
	case values.NullValue:
		return nil
	case values.ArrayValue:
		result := make([]interface{}, len(val.Items))
		for i, item := range val.Items {
			result[i] = valueToJSON(item)
		}
		return result
	case values.MapValue:
		result := make(map[string]interface{}, len(val.Entries))
		for _, entry := range val.Entries {
			// Convert key to string for JSON map compatibility.
			k := valueToJSON(entry.Key)
			var keyStr string
			switch ks := k.(type) {
			case string:
				keyStr = ks
			default:
				keyStr = fmt.Sprintf("%v", k)
			}
			result[keyStr] = valueToJSON(entry.Value)
		}
		return result
	case values.PairValue:
		return map[string]interface{}{
			"left":  valueToJSON(val.Left),
			"right": valueToJSON(val.Right),
		}
	case values.StructValue:
		result := make(map[string]interface{}, len(val.Fields))
		for name, fv := range val.Fields {
			result[name] = valueToJSON(fv)
		}
		return result
	default:
		return nil
	}
}

// evalCallInputs evaluates all input expressions for a call statement using the
// given environment, and converts the resulting values to JSON-compatible Go types.
// Returns a map[string]interface{} ready to pass to task.RunTask as jsonInputs.
func evalCallInputs(call *ast.Call, env *eval.Env) (map[string]interface{}, error) {
	result := make(map[string]interface{}, len(call.Inputs))
	for name, expr := range call.Inputs {
		v, err := eval.Eval(expr, env, eval.ExecContext{})
		if err != nil {
			return nil, fmt.Errorf("evaluating input %q for call %v: %w", name, call.Callee, err)
		}
		result[name] = valueToJSON(v)
	}
	return result, nil
}

// bindWorkflowInputs binds a workflow's declared inputs from JSON inputs into an eval.Env.
// For each input declaration in wf.Inputs:
//   - If present in jsonInputs: use type-aware conversion (File, Directory, Struct, etc.)
//   - If absent and has default Expr: evaluate the default expression
//   - If absent, no default, and optional: bind NullValue{}
//   - If absent, no default, and required: return error
func bindWorkflowInputs(wf *ast.Workflow, jsonInputs map[string]interface{}, structDefs ...map[string]*ast.StructDef) (*eval.Env, error) {
	var sd map[string]*ast.StructDef
	if len(structDefs) > 0 {
		sd = structDefs[0]
	}
	env := eval.NewEnv(nil)

	for _, decl := range wf.Inputs {
		var val values.Value

		// Try fully-qualified key first (e.g. "hello.name"), then unqualified (e.g. "name").
		raw, found := jsonInputs[wf.Name+"."+decl.Name]
		if !found {
			raw, found = jsonInputs[decl.Name]
		}
		if found {
			val = jsonToValueTyped(raw, decl.Type, sd)
		} else if decl.Expr != nil {
			v, err := eval.Eval(decl.Expr, env, eval.ExecContext{})
			if err != nil {
				return nil, fmt.Errorf("workflow input %q default expression: %w", decl.Name, err)
			}
			val = v
		} else if decl.Type.IsOptional() {
			val = values.NullValue{}
		} else {
			return nil, fmt.Errorf("missing required workflow input: %q", decl.Name)
		}

		env.Define(decl.Name, val)
	}

	return env, nil
}

// cleanPath resolves relative paths to absolute and canonicalizes ".." components.
func cleanPath(p string) string {
	if !filepath.IsAbs(p) {
		if abs, err := filepath.Abs(p); err == nil {
			return filepath.Clean(abs)
		}
	}
	return filepath.Clean(p)
}

// jsonToValueTyped converts a JSON-decoded value to a WDL Value using the
// declaration type for type-aware conversions (File, Directory, Struct, etc.).
func jsonToValueTyped(v interface{}, declType types.Type, structDefs map[string]*ast.StructDef) values.Value {
	if v == nil {
		return values.NullValue{}
	}

	switch dt := declType.(type) {
	case types.FileType:
		if s, ok := v.(string); ok {
			return values.FileValue{Type: dt, Value: cleanPath(s)}
		}
		return jsonToValueUntyped(v)

	case types.DirectoryType:
		if s, ok := v.(string); ok {
			return values.DirectoryValue{Type: dt, Value: cleanPath(s)}
		}
		return jsonToValueUntyped(v)

	case types.StringType:
		if s, ok := v.(string); ok {
			return values.StringValue{Type: dt, Value: s}
		}
		return jsonToValueUntyped(v)

	case types.IntType:
		switch x := v.(type) {
		case float64:
			return values.IntValue{Type: dt, Value: int64(x)}
		case int64:
			return values.IntValue{Type: dt, Value: x}
		case int:
			return values.IntValue{Type: dt, Value: int64(x)}
		}
		return jsonToValueUntyped(v)

	case types.FloatType:
		switch x := v.(type) {
		case float64:
			return values.FloatValue{Type: dt, Value: x}
		case int64:
			return values.FloatValue{Type: dt, Value: float64(x)}
		}
		return jsonToValueUntyped(v)

	case types.BooleanType:
		if b, ok := v.(bool); ok {
			return values.BooleanValue{Type: dt, Value: b}
		}
		return jsonToValueUntyped(v)

	case types.ArrayType:
		if arr, ok := v.([]interface{}); ok {
			items := make([]values.Value, len(arr))
			for i, elem := range arr {
				items[i] = jsonToValueTyped(elem, dt.ItemType, structDefs)
			}
			return values.ArrayValue{Type: dt, Items: items}
		}
		return jsonToValueUntyped(v)

	case types.MapType:
		if m, ok := v.(map[string]interface{}); ok {
			entries := make([]values.MapEntry, 0, len(m))
			for k, mv := range m {
				kv := jsonToValueTyped(k, dt.KeyType, structDefs)
				vv := jsonToValueTyped(mv, dt.ValType, structDefs)
				entries = append(entries, values.MapEntry{Key: kv, Value: vv})
			}
			return values.MapValue{Type: dt, Entries: entries}
		}
		return jsonToValueUntyped(v)

	case types.PairType:
		if m, ok := v.(map[string]interface{}); ok {
			left := jsonToValueTyped(m["left"], dt.LeftType, structDefs)
			right := jsonToValueTyped(m["right"], dt.RightType, structDefs)
			return values.PairValue{Type: dt, Left: left, Right: right}
		}
		return jsonToValueUntyped(v)

	case types.StructType:
		if m, ok := v.(map[string]interface{}); ok {
			fields := make(map[string]values.Value, len(m))
			// Look up struct definition for field types.
			if sd, hasDef := structDefs[dt.Name]; hasDef {
				for _, member := range sd.Members {
					if fv, exists := m[member.Name]; exists {
						fields[member.Name] = jsonToValueTyped(fv, member.Type, structDefs)
					}
				}
				// Include any extra fields not in the struct definition.
				for k, fv := range m {
					if _, already := fields[k]; !already {
						fields[k] = jsonToValueUntyped(fv)
					}
				}
			} else {
				// No struct definition available; convert fields untyped.
				for k, fv := range m {
					fields[k] = jsonToValueUntyped(fv)
				}
			}
			return values.StructValue{Type: dt, Fields: fields}
		}
		return jsonToValueUntyped(v)

	default:
		return jsonToValueUntyped(v)
	}
}

// jsonToValueUntyped is the untyped fallback for JSON-to-WDL conversion.
// Duplicated from task package since task.jsonToWDLValue is unexported.
func jsonToValueUntyped(v interface{}) values.Value {
	if v == nil {
		return values.NullValue{}
	}
	switch x := v.(type) {
	case bool:
		return values.BooleanValue{Value: x}
	case float64:
		// Whole-number float64 values are promoted to IntValue.
		if x == float64(int64(x)) {
			return values.IntValue{Value: int64(x)}
		}
		return values.FloatValue{Value: x}
	case int64:
		return values.IntValue{Value: x}
	case int:
		return values.IntValue{Value: int64(x)}
	case string:
		return values.StringValue{Value: x}
	case []interface{}:
		items := make([]values.Value, len(x))
		for i, elem := range x {
			items[i] = jsonToValueUntyped(elem)
		}
		return values.ArrayValue{Items: items}
	case map[string]interface{}:
		entries := make([]values.MapEntry, 0, len(x))
		for k, mv := range x {
			entries = append(entries, values.MapEntry{
				Key:   values.StringValue{Value: k},
				Value: jsonToValueUntyped(mv),
			})
		}
		return values.MapValue{Entries: entries}
	default:
		return values.NullValue{}
	}
}

