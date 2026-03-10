package checker

import "github.com/bioflowy/flowy/wdl/types"

// arrayize wraps t in an ArrayType, producing Array[T].
// Used when a scatter block promotes its body declarations to array-typed outputs.
func arrayize(t types.Type) types.Type {
	return types.ArrayType{ItemType: t}
}

// optionalize returns t as an optional type (T?).
// If t.IsOptional() is already true, returns t unchanged (idempotency guard)
// to prevent double-wrapping (T? must not become T??).
func optionalize(t types.Type) types.Type {
	if t.IsOptional() {
		return t
	}
	return t.WithOptional(true)
}
