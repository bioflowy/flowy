package checker

import "github.com/bioflowy/flowy/wdl/types"

// Coerces reports whether WDL coercion rules permit assigning a value of type
// from to a variable of type to. The stub always returns false; Plan 02 implements
// the full coercion table.
func Coerces(from, to types.Type) bool {
	return false
}
