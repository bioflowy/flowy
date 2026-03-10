package values

import "github.com/bioflowy/flowy/wdl/types"

// Value is the interface implemented by all WDL runtime value structs.
type Value interface {
	ValueType() types.Type
}

// BooleanValue holds a WDL Boolean runtime value.
type BooleanValue struct {
	Type  types.BooleanType
	Value bool
}

func (v BooleanValue) ValueType() types.Type { return v.Type }

// IntValue holds a WDL Int runtime value.
type IntValue struct {
	Type  types.IntType
	Value int64
}

func (v IntValue) ValueType() types.Type { return v.Type }

// FloatValue holds a WDL Float runtime value.
type FloatValue struct {
	Type  types.FloatType
	Value float64
}

func (v FloatValue) ValueType() types.Type { return v.Type }

// StringValue holds a WDL String runtime value.
type StringValue struct {
	Type  types.StringType
	Value string
}

func (v StringValue) ValueType() types.Type { return v.Type }

// FileValue holds a WDL File runtime value.
// Value is an absolute host filesystem path.
type FileValue struct {
	Type  types.FileType
	Value string
}

func (v FileValue) ValueType() types.Type { return v.Type }

// DirectoryValue holds a WDL Directory runtime value. New in WDL 1.2.
// Value is an absolute host filesystem path.
type DirectoryValue struct {
	Type  types.DirectoryType
	Value string
}

func (v DirectoryValue) ValueType() types.Type { return v.Type }

// NullValue is the runtime representation of the WDL None literal and any absent optional value.
// It is a concrete zero-size sentinel, NOT a nil interface.
// Use NullValue{} everywhere; never use nil to represent an absent WDL value.
type NullValue struct{}

func (v NullValue) ValueType() types.Type { return types.AnyType{Optional: true} }

// ArrayValue holds a WDL Array[T] runtime value.
// Items slice is a reference type. Deep-copy before mutation (Phase 4 evaluator responsibility).
type ArrayValue struct {
	Type  types.ArrayType
	Items []Value
}

func (v ArrayValue) ValueType() types.Type { return v.Type }

// MapEntry is a single key-value entry in a WDL Map.
type MapEntry struct {
	Key   Value
	Value Value
}

// MapValue holds a WDL Map[K,V] runtime value.
// Entries preserves insertion order (WDL Map iteration order is defined).
type MapValue struct {
	Type    types.MapType
	Entries []MapEntry
}

func (v MapValue) ValueType() types.Type { return v.Type }

// PairValue holds a WDL Pair[L,R] runtime value.
type PairValue struct {
	Type  types.PairType
	Left  Value
	Right Value
}

func (v PairValue) ValueType() types.Type { return v.Type }

// StructValue holds a WDL struct runtime value.
// Fields map is a reference type. Deep-copy before mutation (Phase 4 evaluator responsibility).
type StructValue struct {
	Type   types.StructType
	Fields map[string]Value
}

func (v StructValue) ValueType() types.Type { return v.Type }
