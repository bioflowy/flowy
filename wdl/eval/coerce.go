package eval

import (
	"fmt"
	"sort"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/types"
	"github.com/bioflowy/flowy/wdl/values"
)

// CoerceValue attempts to coerce v to the target type according to WDL spec rules.
// Returns the coerced value and true on success, or (v, false) if no coercion applies.
//
// Supported coercions:
//   - String ↔ File
//   - String ↔ Directory
//   - Int → Float
//   - T → T? (optional wrapping)
//   - Array element-wise coercion
//   - Map key/value coercion
//   - Pair element coercion
//   - Struct → Struct (structural compatibility)
func CoerceValue(v values.Value, target types.Type, structDefs map[string]*ast.StructDef) (values.Value, bool) {
	if v == nil {
		return v, false
	}

	// NullValue is always compatible with optional types.
	if _, isNull := v.(values.NullValue); isNull {
		if target.IsOptional() {
			return v, true
		}
		return v, false
	}

	// Strip optional wrapper from target for coercion checks.
	baseTarget := target
	if target.IsOptional() {
		baseTarget = target.WithOptional(false)
	}

	// If the value already has the right type, no coercion needed.
	if v.ValueType().TypeName() == baseTarget.TypeName() {
		return v, true
	}

	switch baseTarget.(type) {
	case types.FileType:
		if sv, ok := v.(values.StringValue); ok {
			return values.FileValue{Type: types.FileType{}, Value: sv.Value}, true
		}
	case types.DirectoryType:
		if sv, ok := v.(values.StringValue); ok {
			return values.DirectoryValue{Type: types.DirectoryType{}, Value: sv.Value}, true
		}
	case types.StringType:
		if fv, ok := v.(values.FileValue); ok {
			return values.StringValue{Type: types.StringType{}, Value: fv.Value}, true
		}
		if dv, ok := v.(values.DirectoryValue); ok {
			return values.StringValue{Type: types.StringType{}, Value: dv.Value}, true
		}
	case types.FloatType:
		if iv, ok := v.(values.IntValue); ok {
			return values.FloatValue{Type: types.FloatType{}, Value: float64(iv.Value)}, true
		}
	}

	// Array element-wise coercion
	if at, ok := baseTarget.(types.ArrayType); ok {
		if av, ok := v.(values.ArrayValue); ok {
			coerced := make([]values.Value, len(av.Items))
			anyCoerced := false
			for i, item := range av.Items {
				c, changed := CoerceValue(item, at.ItemType, structDefs)
				if changed && c.ValueType().TypeName() != item.ValueType().TypeName() {
					anyCoerced = true
				}
				coerced[i] = c
			}
			if anyCoerced {
				return values.ArrayValue{Type: at, Items: coerced}, true
			}
			return v, true
		}
	}

	// Map key/value coercion
	if mt, ok := baseTarget.(types.MapType); ok {
		if mv, ok := v.(values.MapValue); ok {
			coerced := make([]values.MapEntry, len(mv.Entries))
			anyCoerced := false
			for i, e := range mv.Entries {
				ck, kok := CoerceValue(e.Key, mt.KeyType, structDefs)
				cv, vok := CoerceValue(e.Value, mt.ValType, structDefs)
				if !kok {
					ck = e.Key
				}
				if !vok {
					cv = e.Value
				}
				if (kok && ck.ValueType().TypeName() != e.Key.ValueType().TypeName()) ||
					(vok && cv.ValueType().TypeName() != e.Value.ValueType().TypeName()) {
					anyCoerced = true
				}
				coerced[i] = values.MapEntry{Key: ck, Value: cv}
			}
			if anyCoerced {
				return values.MapValue{Type: mt, Entries: coerced}, true
			}
			return v, true
		}
	}

	// Pair element coercion
	if pt, ok := baseTarget.(types.PairType); ok {
		if pv, ok := v.(values.PairValue); ok {
			cl, lok := CoerceValue(pv.Left, pt.LeftType, structDefs)
			cr, rok := CoerceValue(pv.Right, pt.RightType, structDefs)
			if !lok {
				cl = pv.Left
			}
			if !rok {
				cr = pv.Right
			}
			lChanged := lok && cl.ValueType().TypeName() != pv.Left.ValueType().TypeName()
			rChanged := rok && cr.ValueType().TypeName() != pv.Right.ValueType().TypeName()
			if lChanged || rChanged {
				return values.PairValue{Type: pt, Left: cl, Right: cr}, true
			}
			return v, true
		}
	}

	// Struct-to-struct coercion (structural compatibility)
	if st, ok := baseTarget.(types.StructType); ok {
		if sv, ok := v.(values.StructValue); ok {
			return coerceStruct(sv, st, structDefs)
		}
	}

	return v, false
}

// coerceStruct attempts to coerce a StructValue to a different struct type
// by checking structural compatibility and coercing individual fields.
func coerceStruct(sv values.StructValue, targetType types.StructType, structDefs map[string]*ast.StructDef) (values.Value, bool) {
	if structDefs == nil {
		return sv, false
	}

	targetDef, ok := structDefs[targetType.Name]
	if !ok {
		return sv, false
	}

	newFields := make(map[string]values.Value, len(targetDef.Members))
	for _, member := range targetDef.Members {
		fv, exists := sv.Fields[member.Name]
		if !exists {
			if member.Type.IsOptional() {
				newFields[member.Name] = values.NullValue{}
				continue
			}
			return sv, false
		}
		coerced, ok := CoerceValue(fv, member.Type, structDefs)
		if ok {
			newFields[member.Name] = coerced
		} else {
			newFields[member.Name] = fv
		}
	}

	return values.StructValue{
		Type:   targetType,
		Fields: newFields,
	}, true
}

// EvalDecl evaluates an expression and coerces the result to the target type.
// This is the standard way to evaluate WDL declarations.
func EvalDecl(expr ast.Expr, env *Env, ctx ExecContext, targetType types.Type) (values.Value, error) {
	v, err := Eval(expr, env, ctx)
	if err != nil {
		return nil, err
	}
	coerced, _ := CoerceValue(v, targetType, ctx.StructDefs)
	// Enforce non-empty array constraint (Array[T]+).
	if at, ok := targetType.(types.ArrayType); ok && at.Nonempty {
		if av, ok := coerced.(values.ArrayValue); ok && len(av.Items) == 0 {
			return nil, fmt.Errorf("cannot assign empty array to non-empty type %s", targetType.TypeName())
		}
	}
	return coerced, nil
}

// CoerceForComparison extracts a string representation from File/Directory values
// for comparison with String values, as required by WDL spec.
// Returns the (possibly coerced) values and whether any coercion was applied.
func CoerceForComparison(left, right values.Value) (values.Value, values.Value, bool) {
	// File/Directory ↔ String comparison: extract path as string
	switch l := left.(type) {
	case values.FileValue:
		if _, ok := right.(values.StringValue); ok {
			return values.StringValue{Type: types.StringType{}, Value: l.Value}, right, true
		}
	case values.DirectoryValue:
		if _, ok := right.(values.StringValue); ok {
			return values.StringValue{Type: types.StringType{}, Value: l.Value}, right, true
		}
	case values.StringValue:
		if r, ok := right.(values.FileValue); ok {
			return left, values.StringValue{Type: types.StringType{}, Value: r.Value}, true
		}
		if r, ok := right.(values.DirectoryValue); ok {
			return left, values.StringValue{Type: types.StringType{}, Value: r.Value}, true
		}
	}
	return left, right, false
}

// StructKeys returns the field names of a struct as a sorted string array.
// WDL spec: keys(Struct) returns Array[String] of field names.
func StructKeys(sv values.StructValue, structDefs map[string]*ast.StructDef) []string {
	// If we have a struct definition, use the definition order.
	if structDefs != nil {
		if def, ok := structDefs[sv.Type.Name]; ok {
			keys := make([]string, len(def.Members))
			for i, m := range def.Members {
				keys[i] = m.Name
			}
			return keys
		}
	}
	// Fallback: sorted field names from the value.
	keys := make([]string, 0, len(sv.Fields))
	for k := range sv.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
