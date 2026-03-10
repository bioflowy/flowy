package types

// Type is the interface implemented by all WDL type structs.
type Type interface {
	TypeName() string       // canonical WDL string, e.g. "Int", "Array[String]?"
	IsOptional() bool       // true if this is T?
	WithOptional(bool) Type // returns immutable copy with optional flag set
}

// optSuffix returns "?" when optional is true, "" otherwise.
func optSuffix(optional bool) string {
	if optional {
		return "?"
	}
	return ""
}

// BooleanType represents the WDL Boolean primitive type.
type BooleanType struct {
	Optional bool
}

func (t BooleanType) TypeName() string      { return "Boolean" + optSuffix(t.Optional) }
func (t BooleanType) IsOptional() bool      { return t.Optional }
func (t BooleanType) WithOptional(v bool) Type { c := t; c.Optional = v; return c }

// IntType represents the WDL Int primitive type.
type IntType struct {
	Optional bool
}

func (t IntType) TypeName() string      { return "Int" + optSuffix(t.Optional) }
func (t IntType) IsOptional() bool      { return t.Optional }
func (t IntType) WithOptional(v bool) Type { c := t; c.Optional = v; return c }

// FloatType represents the WDL Float primitive type.
type FloatType struct {
	Optional bool
}

func (t FloatType) TypeName() string      { return "Float" + optSuffix(t.Optional) }
func (t FloatType) IsOptional() bool      { return t.Optional }
func (t FloatType) WithOptional(v bool) Type { c := t; c.Optional = v; return c }

// StringType represents the WDL String primitive type.
type StringType struct {
	Optional bool
}

func (t StringType) TypeName() string      { return "String" + optSuffix(t.Optional) }
func (t StringType) IsOptional() bool      { return t.Optional }
func (t StringType) WithOptional(v bool) Type { c := t; c.Optional = v; return c }

// FileType represents the WDL File primitive type.
type FileType struct {
	Optional bool
}

func (t FileType) TypeName() string      { return "File" + optSuffix(t.Optional) }
func (t FileType) IsOptional() bool      { return t.Optional }
func (t FileType) WithOptional(v bool) Type { c := t; c.Optional = v; return c }

// DirectoryType represents the WDL Directory primitive type (WDL 1.2 first-class primitive).
type DirectoryType struct {
	Optional bool
}

func (t DirectoryType) TypeName() string      { return "Directory" + optSuffix(t.Optional) }
func (t DirectoryType) IsOptional() bool      { return t.Optional }
func (t DirectoryType) WithOptional(v bool) Type { c := t; c.Optional = v; return c }

// AnyType represents either the unconstrained Any type or the None literal when Optional=true.
// AnyType{Optional:true} is the type of NullValue and absent optionals.
type AnyType struct {
	Optional bool
}

func (t AnyType) TypeName() string {
	if t.Optional {
		return "None"
	}
	return "Any"
}
func (t AnyType) IsOptional() bool      { return t.Optional }
func (t AnyType) WithOptional(v bool) Type { c := t; c.Optional = v; return c }

// ArrayType represents a WDL Array[T] compound type.
// Nonempty=true corresponds to the Array[T]+ quantifier.
type ArrayType struct {
	ItemType Type
	Optional bool
	Nonempty bool
}

func (t ArrayType) TypeName() string {
	name := "Array[" + t.ItemType.TypeName() + "]"
	if t.Nonempty {
		name += "+"
	}
	return name + optSuffix(t.Optional)
}
func (t ArrayType) IsOptional() bool      { return t.Optional }
func (t ArrayType) WithOptional(v bool) Type { c := t; c.Optional = v; return c }

// MapType represents a WDL Map[K,V] compound type.
type MapType struct {
	KeyType  Type
	ValType  Type
	Optional bool
}

func (t MapType) TypeName() string {
	return "Map[" + t.KeyType.TypeName() + "," + t.ValType.TypeName() + "]" + optSuffix(t.Optional)
}
func (t MapType) IsOptional() bool      { return t.Optional }
func (t MapType) WithOptional(v bool) Type { c := t; c.Optional = v; return c }

// PairType represents a WDL Pair[L,R] compound type.
type PairType struct {
	LeftType  Type
	RightType Type
	Optional  bool
}

func (t PairType) TypeName() string {
	return "Pair[" + t.LeftType.TypeName() + "," + t.RightType.TypeName() + "]" + optSuffix(t.Optional)
}
func (t PairType) IsOptional() bool      { return t.Optional }
func (t PairType) WithOptional(v bool) Type { c := t; c.Optional = v; return c }

// StructType represents a WDL struct type by name.
// Note: field is "Name" (not "TypeName") to avoid collision with TypeName() method.
// Warning: Members map is a reference type. Deep-copy before mutation (Phase 3 type checker responsibility).
type StructType struct {
	Name     string
	Members  map[string]Type
	Optional bool
}

func (t StructType) TypeName() string      { return t.Name + optSuffix(t.Optional) }
func (t StructType) IsOptional() bool      { return t.Optional }
func (t StructType) WithOptional(v bool) Type { c := t; c.Optional = v; return c }
