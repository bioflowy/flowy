package checker

import "github.com/bioflowy/flowy/wdl/types"

// arrayize wraps t in an ArrayType, producing Array[T].
// Used when a scatter block promotes its body declarations to array-typed outputs.
func arrayize(t types.Type) types.Type {
	return types.ArrayType{ItemType: t}
}

// optionalize wraps t as optional (T?).
// The stub returns t unchanged; Plan 02 will add the idempotency guard to
// prevent double-wrapping (T?? → T?).
func optionalize(t types.Type) types.Type {
	return t
}
