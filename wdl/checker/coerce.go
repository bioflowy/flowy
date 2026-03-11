package checker

import (
	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/types"
)

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
	// Rule 0: None → T? (null/None is assignable to any optional type)
	if _, isAny := from.(types.AnyType); isAny && from.IsOptional() && to.IsOptional() {
		return true
	}

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
		_ = f
		if _, ok := to.(types.FloatType); ok {
			return true
		}

	case types.StringType:
		// Rule 3a: String → File
		_ = f
		if _, ok := to.(types.FileType); ok {
			return true
		}
		// Rule 3b: String → Directory
		if _, ok := to.(types.DirectoryType); ok {
			return true
		}

	case types.FileType:
		// Rule 4: File → String
		_ = f
		if _, ok := to.(types.StringType); ok {
			return true
		}

	case types.ArrayType:
		if t, ok := to.(types.ArrayType); ok {
			// Rule 5: Array[T]+ → Array[T] (nonempty to regular)
			// Also: Array[Int] → Array[Float] (recursive item coercion)
			if f.Nonempty && !t.Nonempty {
				return Coerces(f.ItemType, t.ItemType)
			}
			if !f.Nonempty || t.Nonempty {
				return Coerces(f.ItemType, t.ItemType)
			}
		}

	case types.MapType:
		// Map[K1,V1] → Map[K2,V2] if K1→K2 and V1→V2
		if t, ok := to.(types.MapType); ok {
			return Coerces(f.KeyType, t.KeyType) && Coerces(f.ValType, t.ValType)
		}

	case types.PairType:
		// Pair[L1,R1] → Pair[L2,R2] if L1→L2 and R1→R2
		if t, ok := to.(types.PairType); ok {
			return Coerces(f.LeftType, t.LeftType) && Coerces(f.RightType, t.RightType)
		}
	}

	// Rule 6: identity — same canonical type name (covers same-type T→T).
	return from.TypeName() == to.TypeName()
}

// CoercesWithStructs extends Coerces with structural struct compatibility.
// Two struct types are coercible if they have the same fields with compatible types.
func CoercesWithStructs(from, to types.Type, structDefs map[string]*ast.StructDef) bool {
	// Handle optional wrapping first.
	if to.IsOptional() && !from.IsOptional() {
		return CoercesWithStructs(from, to.WithOptional(false), structDefs)
	}

	// If non-struct types, delegate to basic Coerces.
	ft, fromIsStruct := from.(types.StructType)
	tt, toIsStruct := to.(types.StructType)

	if !fromIsStruct || !toIsStruct {
		// Handle compound types with potential struct elements.
		switch f := from.(type) {
		case types.ArrayType:
			if t, ok := to.(types.ArrayType); ok {
				return CoercesWithStructs(f.ItemType, t.ItemType, structDefs)
			}
		case types.MapType:
			if t, ok := to.(types.MapType); ok {
				return CoercesWithStructs(f.KeyType, t.KeyType, structDefs) &&
					CoercesWithStructs(f.ValType, t.ValType, structDefs)
			}
		case types.PairType:
			if t, ok := to.(types.PairType); ok {
				return CoercesWithStructs(f.LeftType, t.LeftType, structDefs) &&
					CoercesWithStructs(f.RightType, t.RightType, structDefs)
			}
		}
		return Coerces(from, to)
	}

	// Same struct name: always compatible.
	if ft.Name == tt.Name {
		return true
	}

	// Different struct names: check structural compatibility.
	if structDefs == nil {
		return false
	}
	fromDef, ok1 := structDefs[ft.Name]
	toDef, ok2 := structDefs[tt.Name]
	if !ok1 || !ok2 {
		return false
	}

	// Must have same number of members.
	if len(fromDef.Members) != len(toDef.Members) {
		return false
	}

	// Build a map from field name to type for the source struct.
	fromFields := make(map[string]types.Type, len(fromDef.Members))
	for _, m := range fromDef.Members {
		fromFields[m.Name] = m.Type
	}

	// Check each target field has a matching source field with compatible type.
	for _, m := range toDef.Members {
		ft, exists := fromFields[m.Name]
		if !exists {
			return false
		}
		if !CoercesWithStructs(ft, m.Type, structDefs) {
			return false
		}
	}

	return true
}
