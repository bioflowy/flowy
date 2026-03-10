package workflow

import (
	"fmt"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/eval"
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
//   - If present in jsonInputs: use the value directly (as untyped conversion)
//   - If absent and has default Expr: evaluate the default expression
//   - If absent, no default, and optional: bind NullValue{}
//   - If absent, no default, and required: return error
func bindWorkflowInputs(wf *ast.Workflow, jsonInputs map[string]interface{}) (*eval.Env, error) {
	env := eval.NewEnv(nil)

	for _, decl := range wf.Inputs {
		var val values.Value

		raw, found := jsonInputs[decl.Name]
		if found {
			val = jsonToValueUntyped(raw)
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
