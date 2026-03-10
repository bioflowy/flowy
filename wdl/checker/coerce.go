package checker

import "github.com/bioflowy/flowy/wdl/types"

// Coerces reports whether WDL coercion rules permit assigning a value of type
// from to a variable of type to. Rules are applied shallowly (outermost type only).
//
// The 6 rules, in evaluation order:
//
//	Rule 1 (T → T?): if `to` is optional and `from` is not, strip the optional
//	                 flag from `to` and recurse to check the non-optional form.
//	Rule 2 (Int → Float): IntType coerces to FloatType.
//	Rule 3a (String → File): StringType coerces to FileType.
//	Rule 3b (String → Directory): StringType coerces to DirectoryType.
//	Rule 4 (File → String): FileType coerces to StringType.
//	Rule 5 (Array[T]+ → Array[T]): a non-empty array coerces to the same array
//	                                type without the Nonempty constraint.
//	                                Item types are compared by TypeName (shallow).
//	Rule 6 (identity): if from.TypeName() == to.TypeName(), return true.
func Coerces(from, to types.Type) bool {
	// Rule 1: T → T?
	// If the target is optional and the source is not, strip the optional flag
	// from the target and retry. This handles Int→Int?, String→String?, etc.
	if to.IsOptional() && !from.IsOptional() {
		return Coerces(from, to.WithOptional(false))
	}

	// Rules 2-5 use a type-switch on `from`.
	switch f := from.(type) {
	case types.IntType:
		// Rule 2: Int → Float
		// Strip optional from `to` for comparison because Rule 1 already
		// handled T→T?; at this point both sides should have matching optional flags.
		if _, ok := to.(types.FloatType); ok {
			return true
		}

	case types.StringType:
		// Rule 3a: String → File
		if _, ok := to.(types.FileType); ok {
			return true
		}
		// Rule 3b: String → Directory
		if _, ok := to.(types.DirectoryType); ok {
			return true
		}
		_ = f // silence unused variable warning

	case types.FileType:
		// Rule 4: File → String
		if _, ok := to.(types.StringType); ok {
			return true
		}
		_ = f // silence unused variable warning

	case types.ArrayType:
		// Rule 5: Array[T]+ → Array[T]
		// The source must be non-empty and the target must be an ArrayType with the
		// same item TypeName and Nonempty==false. Item types are compared shallowly
		// by TypeName(); we do NOT recurse into Coerces for the item types.
		if f.Nonempty {
			if t, ok := to.(types.ArrayType); ok && !t.Nonempty {
				return f.ItemType.TypeName() == t.ItemType.TypeName()
			}
		}
	}

	// Rule 6: identity — same canonical type name (covers same-type T→T).
	return from.TypeName() == to.TypeName()
}
