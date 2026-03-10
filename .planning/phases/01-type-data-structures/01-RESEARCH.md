# Phase 1: Type Data Structures - Research

**Researched:** 2026-03-10
**Domain:** Go type system design, WDL 1.2 type semantics
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- Go type names mirror miniwdl's Python names closely: `Int`, `Float`, `Boolean`, `String`, `File`, `Directory`, `Array`, `Map`, `Pair` (package `wdl/types`)
- Exception: WDL Struct type definition → `StructType` (not `Struct`) to avoid ambiguity
- Two separate packages: `wdl/types` and `wdl/values` (mirroring miniwdl's Type.py / Value.py split)
- Import paths: `github.com/bioflowy/flowy/wdl/types` and `github.com/bioflowy/flowy/wdl/values`
- Value structs use explicit suffix: `IntValue`, `FloatValue`, `BooleanValue`, `StringValue`, `FileValue`, `DirectoryValue`, `ArrayValue`, `MapValue`, `PairValue`
- Struct value instance → `StructValue` (paired with `StructType` in the types package)
- `StructType` fields: `TypeName string`, `Members map[string]Type`
- `StructValue` fields: `Type types.StructType`, `Fields map[string]Value`
- Go module: `github.com/bioflowy/flowy`, Go 1.24

### Claude's Discretion

- Optional (`T?`) representation in Go — pointer-based, Optional[T] wrapper, or tagged union
- Type interface methods beyond type tagging (String(), Equals(), IsOptional(), etc.)
- None literal representation
- File/Directory value backing type (string path vs. richer struct)
- Internal package layout beyond the two top-level packages (e.g., `internal/` subpackages)

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| TYPE-01 | Primitive types (Boolean, Int, Float, String, File) with type-tagged Value structs | Type interface design + miniwdl reference semantics |
| TYPE-02 | Directory type (WDL 1.2 new feature) | WDL 1.2 spec confirms Directory is a first-class primitive; miniwdl models it as a String subtype |
| TYPE-03 | Compound types (Array[T], Map[K,V], Pair[L,R], Struct) with type parameters | Miniwdl reference + Go generics/struct field design |
| TYPE-04 | Optional type (T?) with None literal and type inference | WDL 1.2 spec + miniwdl `Type.Any(null=True)` + optional flag pattern |
</phase_requirements>

---

## Summary

Phase 1 is a pure data-model phase: define all WDL 1.2 types and values as Go structs and interfaces, and initialize `go.mod`. Nothing is parsed, evaluated, or executed — the output is importable packages that every downstream phase builds on.

The WDL 1.2 type system has six primitives (Boolean, Int, Float, String, File, Directory), four compound types (Array[T], Map[K,V], Pair[L,R], Struct), and two quantifiers: optional (`?`) and nonempty (`+`, Array only). The reference implementation in Python (miniwdl `WDL/Type.py` and `WDL/Value.py`) provides the authoritative semantic model. Go translation is straightforward: a `Type` interface with a small method set, concrete structs for each type, a parallel `Value` interface, and concrete value structs — all following the locked naming decisions.

The key discretion choices are: (1) representing `T?` as a boolean `Optional bool` flag embedded in every type struct (mirroring miniwdl exactly, zero extra allocation, easy to copy), (2) a `NullValue` struct for the None literal (not a pointer nil), (3) `FileValue`/`DirectoryValue` backing type as plain `string` path (matches miniwdl; richer struct adds no value at this phase), and (4) a minimal `Type` interface: `TypeName() string`, `IsOptional() bool`, `WithOptional(bool) Type`.

**Primary recommendation:** Embed `Optional bool` in every concrete type struct. Use a `NullValue` sentinel struct for None. Keep interfaces narrow — only what downstream consumers (type checker, evaluator) provably need.

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go standard library | 1.24 | `fmt`, `strings` for String() methods | No external deps needed for pure data model |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `testing` (stdlib) | 1.24 | Unit tests | Required — nyquist_validation is enabled |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `Optional bool` field in each struct | Generic `Optional[T]` wrapper | Wrapper adds allocations and complicates the type hierarchy; bool flag is zero-cost and matches miniwdl exactly |
| `Optional bool` field in each struct | Pointer-to-type (e.g., `*IntType`) | Pointers make equality comparison fragile and types non-comparable; nil is ambiguous (absent vs. zero) |
| `NullValue` sentinel struct | `nil` interface value | nil interface is not a valid `Value` — callers must type-assert; a concrete Null struct is self-describing and safe |

**Installation:**
```bash
cd /home/uehara/flowy
go mod init github.com/bioflowy/flowy
# No external dependencies required for this phase
```

---

## Architecture Patterns

### Recommended Project Structure
```
go.mod
go.sum                         # (empty after init, filled by later phases)
wdl/
├── types/
│   ├── types.go               # Type interface + all concrete type structs
│   └── types_test.go          # Table-driven tests for all types
└── values/
    ├── values.go              # Value interface + all concrete value structs
    └── values_test.go         # Table-driven tests for all values
```

No `internal/` subpackage is needed for this phase — both packages are public by design (type checker imports `types`, evaluator imports both).

### Pattern 1: Type Interface with Embedded Optional Flag

**What:** Each concrete type struct carries an `Optional bool` field. The `Type` interface exposes `TypeName()`, `IsOptional()`, and `WithOptional(bool) Type`. The `WithOptional` method returns a copy with the flag set, enabling immutable type variants without allocation gymnastics.

**When to use:** Everywhere a WDL type is manipulated. The pattern maps directly to miniwdl's `Base._optional` + `Base.copy(optional=…)`.

**Example:**
```go
// wdl/types/types.go

type Type interface {
    // TypeName returns the canonical WDL type name string (e.g., "Int", "Array[String]").
    TypeName() string
    // IsOptional reports whether this is a T? variant.
    IsOptional() bool
    // WithOptional returns a copy of this type with the optional flag set to v.
    // The original is not modified (types are treated as immutable).
    WithOptional(v bool) Type
}

// IntType is the WDL Int type.
type IntType struct {
    Optional bool
}

func (t IntType) TypeName() string        { return "Int" + optSuffix(t.Optional) }
func (t IntType) IsOptional() bool        { return t.Optional }
func (t IntType) WithOptional(v bool) Type { c := t; c.Optional = v; return c }

func optSuffix(optional bool) string {
    if optional { return "?" }
    return ""
}
```

Source: miniwdl `WDL/Type.py` Base class pattern; Go adaptation per standard Go value-receiver immutability conventions.

### Pattern 2: Value Interface with Type Tag

**What:** Every value struct carries a `Type` field (the concrete type from `wdl/types`) so the evaluator can inspect the runtime type without type-asserting. The `Value` interface exposes `ValueType() types.Type`.

**When to use:** All value instantiation. The type tag is set at construction and never changes.

**Example:**
```go
// wdl/values/values.go

import "github.com/bioflowy/flowy/wdl/types"

type Value interface {
    // ValueType returns the WDL type of this value.
    ValueType() types.Type
}

type IntValue struct {
    Type  types.IntType
    Value int64
}

func (v IntValue) ValueType() types.Type { return v.Type }

// NullValue is the WDL None literal. It satisfies Value and is the only
// runtime representation of an absent optional.
type NullValue struct{}

func (v NullValue) ValueType() types.Type {
    // None is typed as Any(optional=true) in miniwdl. We use a zero IntType
    // with Optional=true as a placeholder; the type checker resolves the
    // concrete type from context.
    return types.AnyType{Optional: true}
}
```

Source: miniwdl `WDL/Value.py` Base + Null pattern.

### Pattern 3: Array Non-empty Quantifier

**What:** `Array[T]` has both an `Optional bool` and a `Nonempty bool`. The `Nonempty` flag corresponds to the `Array[T]+` quantifier in WDL. Both flags are independent.

**Example:**
```go
type ArrayType struct {
    ItemType Type
    Optional bool
    Nonempty bool   // Array[T]+ requires at least one element at runtime
}

func (t ArrayType) TypeName() string {
    s := "Array[" + t.ItemType.TypeName() + "]"
    if t.Nonempty { s += "+" }
    if t.Optional { s += "?" }
    return s
}
```

Source: miniwdl `WDL/Type.py` Array class `_nonempty` field.

### Pattern 4: Map Key/Value Pairs

**What:** `Map[K,V]` carries two type parameters. In miniwdl, keys must be primitive types (enforced by the type checker in Phase 3, not here). The data structure stores them as two `Type` fields.

**Example:**
```go
type MapType struct {
    KeyType   Type
    ValueType Type
    Optional  bool
}
```

### Pattern 5: StructType with Named Members

**What:** `StructType` carries `TypeName string` (the declared struct name) and `Members map[string]Type`. Members are populated at parse time (Phase 2); the struct here is just the data container.

**Example:**
```go
// In wdl/types/types.go
type StructType struct {
    TypeName string            // e.g., "SampleInfo"
    Members  map[string]Type   // member name → member type
    Optional bool
}
```

Note: `TypeName()` method conflicts with field name — use a method name `WDLTypeName()` or rename the field to `Name`. Recommendation: field is `Name string`, method is `TypeName() string { return t.Name + optSuffix(t.Optional) }`.

### Pattern 6: AnyType for Unresolved Literals

**What:** An `AnyType` struct (maps to miniwdl's `Type.Any`) represents the type of empty array literals `[]`, empty map literals `{}`, and the `None` literal. The type checker in Phase 3 resolves these to concrete types from context. `AnyType{Optional: true}` is the type of `None`.

**Example:**
```go
type AnyType struct {
    Optional bool  // true for None literal
}

func (t AnyType) TypeName() string {
    if t.Optional { return "None" }
    return "Any"
}
```

### Anti-Patterns to Avoid

- **Using nil to represent None/absent values:** nil interface values cause silent panics and are not self-describing. Use `NullValue{}` sentinel.
- **Mutable type structs:** Type structs passed between packages must behave as values. Always copy-on-modify (as in `WithOptional`). Do not store pointers to type structs in compound types.
- **Sharing the StructType.Members map across copies:** `map` is a reference type in Go. Deep-copy `Members` if mutation is needed (only relevant in Phase 3 type checker, not here).
- **Naming the `TypeName()` interface method the same as the `StructType.Name` field:** causes a method/field collision. Use `Name string` as the field and `TypeName() string` as the method.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Type equality / string representation | Custom equality struct | Implement `TypeName() string` and compare strings | Miniwdl does exactly this: `str(type) == str(other_type)`. Avoids deep-equal pitfalls with maps. |
| Optional handling in compound types | Recursive optional propagation | Optional flag only on the outermost type struct | WDL forbids `Int??`; optional is always the outermost quantifier. Inner types can also be optional independently (e.g., `Array[String?]?`). |
| JSON serialization of values | Custom JSON encoder | `encoding/json` struct tags + implement `MarshalJSON` per type | Standard library handles this; don't build a separate serializer at this phase. |

**Key insight:** This phase has no complex algorithms — it is purely data structure definition. The risk is over-engineering (adding methods that belong to the type checker or evaluator). Keep interfaces minimal.

---

## Common Pitfalls

### Pitfall 1: StructType Field vs Method Name Collision
**What goes wrong:** Defining `StructType.TypeName string` and then implementing `func (t StructType) TypeName() string` on the `Type` interface — Go rejects this because the field and method share a name.
**Why it happens:** The locked decision names the field `TypeName string`. The `Type` interface method is also naturally called `TypeName()`.
**How to avoid:** Name the struct field `Name string` (not `TypeName`). The `TypeName()` method returns `t.Name + optSuffix(t.Optional)`.
**Warning signs:** `cannot use StructType as type Type (missing TypeName method)` or compile-time ambiguity errors.

### Pitfall 2: Optional Propagation on WithOptional
**What goes wrong:** `WithOptional` returns `Type` (interface), but the caller loses the concrete type information and cannot access fields like `ArrayType.Nonempty` without a type assertion.
**Why it happens:** Interface return type erases concrete type.
**How to avoid:** This is acceptable — callers that need concrete fields should type-assert. The interface return is correct because the concrete type is the same, just with a flag flipped. Document this behavior.
**Warning signs:** Excessive type assertions in Phase 3 type checker code suggest the interface is under-serving its consumers. Reassess then, not now.

### Pitfall 3: map[string]Type in StructType is a Reference Type
**What goes wrong:** Copying a `StructType` value copies the map header but not the underlying map data. Two "copies" share mutation.
**Why it happens:** Go maps are reference types; structs containing maps do not deep-copy on assignment.
**How to avoid:** In Phase 1 (data structure definition), document this in a code comment. The type checker (Phase 3) must copy members explicitly before mutating. Do not add a `Clone()` method in Phase 1 — it is premature.
**Warning signs:** Unexpected mutation of `StructType.Members` when you believed you were working with a copy.

### Pitfall 4: ArrayValue Internal Slice is Also a Reference Type
**What goes wrong:** Same issue as Pitfall 3 — copying `ArrayValue` shares the underlying `[]Value` slice.
**Why it happens:** Go slices are reference types.
**How to avoid:** Document in code comments. Deep-copy only when mutation is needed (evaluator, Phase 4).

### Pitfall 5: go.mod Toolchain Directive
**What goes wrong:** On Go 1.21+, `go mod init` may write a `toolchain` directive in addition to the `go` directive (e.g., `toolchain go1.24.6`). This is fine but can surprise developers on different Go patch versions.
**Why it happens:** Go 1.21 introduced toolchain management.
**How to avoid:** Accept the generated `go.mod` as-is. Do not hand-edit the toolchain directive.
**Warning signs:** CI failures on machines with different Go patch versions — ensure CI uses the same toolchain or remove the toolchain directive.

---

## Code Examples

Verified patterns from official sources and miniwdl reference:

### Complete Type Interface and Primitive Types
```go
// wdl/types/types.go
package types

// Type is the interface implemented by all WDL type descriptors.
// Types are treated as immutable values; WithOptional returns a copy.
type Type interface {
    TypeName() string        // canonical WDL type string, e.g. "Int", "Array[String]?"
    IsOptional() bool        // true if this is T?
    WithOptional(bool) Type  // returns a copy with the optional flag set
}

func optSuffix(optional bool) string {
    if optional {
        return "?"
    }
    return ""
}

// --- Primitives ---

type BooleanType struct{ Optional bool }
func (t BooleanType) TypeName() string        { return "Boolean" + optSuffix(t.Optional) }
func (t BooleanType) IsOptional() bool        { return t.Optional }
func (t BooleanType) WithOptional(v bool) Type { c := t; c.Optional = v; return c }

type IntType struct{ Optional bool }
func (t IntType) TypeName() string        { return "Int" + optSuffix(t.Optional) }
func (t IntType) IsOptional() bool        { return t.Optional }
func (t IntType) WithOptional(v bool) Type { c := t; c.Optional = v; return c }

type FloatType struct{ Optional bool }
func (t FloatType) TypeName() string        { return "Float" + optSuffix(t.Optional) }
func (t FloatType) IsOptional() bool        { return t.Optional }
func (t FloatType) WithOptional(v bool) Type { c := t; c.Optional = v; return c }

type StringType struct{ Optional bool }
func (t StringType) TypeName() string        { return "String" + optSuffix(t.Optional) }
func (t StringType) IsOptional() bool        { return t.Optional }
func (t StringType) WithOptional(v bool) Type { c := t; c.Optional = v; return c }

type FileType struct{ Optional bool }
func (t FileType) TypeName() string        { return "File" + optSuffix(t.Optional) }
func (t FileType) IsOptional() bool        { return t.Optional }
func (t FileType) WithOptional(v bool) Type { c := t; c.Optional = v; return c }

// DirectoryType is new in WDL 1.2.
type DirectoryType struct{ Optional bool }
func (t DirectoryType) TypeName() string        { return "Directory" + optSuffix(t.Optional) }
func (t DirectoryType) IsOptional() bool        { return t.Optional }
func (t DirectoryType) WithOptional(v bool) Type { c := t; c.Optional = v; return c }

// AnyType is used for unresolved literals (empty [], {}) and the None literal.
// AnyType{Optional: true} is the type of the None literal.
type AnyType struct{ Optional bool }
func (t AnyType) TypeName() string {
    if t.Optional { return "None" }
    return "Any"
}
func (t AnyType) IsOptional() bool        { return t.Optional }
func (t AnyType) WithOptional(v bool) Type { c := t; c.Optional = v; return c }
```

### Compound Types
```go
// --- Compound types ---

type ArrayType struct {
    ItemType Type
    Optional bool
    Nonempty bool // Array[T]+ quantifier
}

func (t ArrayType) TypeName() string {
    s := "Array[" + t.ItemType.TypeName() + "]"
    if t.Nonempty {
        s += "+"
    }
    return s + optSuffix(t.Optional)
}
func (t ArrayType) IsOptional() bool        { return t.Optional }
func (t ArrayType) WithOptional(v bool) Type { c := t; c.Optional = v; return c }

type MapType struct {
    KeyType   Type
    ValType   Type
    Optional  bool
}

func (t MapType) TypeName() string {
    return "Map[" + t.KeyType.TypeName() + "," + t.ValType.TypeName() + "]" + optSuffix(t.Optional)
}
func (t MapType) IsOptional() bool        { return t.Optional }
func (t MapType) WithOptional(v bool) Type { c := t; c.Optional = v; return c }

type PairType struct {
    LeftType  Type
    RightType Type
    Optional  bool
}

func (t PairType) TypeName() string {
    return "Pair[" + t.LeftType.TypeName() + "," + t.RightType.TypeName() + "]" + optSuffix(t.Optional)
}
func (t PairType) IsOptional() bool        { return t.Optional }
func (t PairType) WithOptional(v bool) Type { c := t; c.Optional = v; return c }

// StructType is the WDL struct type definition.
// Note: field is named "Name" (not "TypeName") to avoid collision with the
// TypeName() interface method.
type StructType struct {
    Name     string            // declared struct name, e.g. "SampleInfo"
    Members  map[string]Type   // member name → member WDL type
    Optional bool
}

func (t StructType) TypeName() string        { return t.Name + optSuffix(t.Optional) }
func (t StructType) IsOptional() bool        { return t.Optional }
func (t StructType) WithOptional(v bool) Type { c := t; c.Optional = v; return c }
// Warning: Members map is NOT deep-copied by WithOptional. See Pitfall 3.
```

### Value Interface and Value Structs
```go
// wdl/values/values.go
package values

import "github.com/bioflowy/flowy/wdl/types"

// Value is the interface implemented by all WDL runtime values.
type Value interface {
    ValueType() types.Type
}

// --- Primitives ---

type BooleanValue struct {
    Type  types.BooleanType
    Value bool
}
func (v BooleanValue) ValueType() types.Type { return v.Type }

type IntValue struct {
    Type  types.IntType
    Value int64
}
func (v IntValue) ValueType() types.Type { return v.Type }

type FloatValue struct {
    Type  types.FloatType
    Value float64
}
func (v FloatValue) ValueType() types.Type { return v.Type }

type StringValue struct {
    Type  types.StringType
    Value string
}
func (v StringValue) ValueType() types.Type { return v.Type }

// FileValue holds a filesystem path. The string is a host absolute path.
// Path validation (trailing slash, existence) is the evaluator's responsibility (Phase 4).
type FileValue struct {
    Type  types.FileType
    Value string // absolute host path
}
func (v FileValue) ValueType() types.Type { return v.Type }

// DirectoryValue holds a directory path. New in WDL 1.2.
type DirectoryValue struct {
    Type  types.DirectoryType
    Value string // absolute host path
}
func (v DirectoryValue) ValueType() types.Type { return v.Type }

// --- None literal ---

// NullValue is the runtime representation of the WDL None literal and any
// absent optional value. It is NOT a nil interface — it is a concrete zero-size
// sentinel that satisfies the Value interface.
type NullValue struct{}
func (v NullValue) ValueType() types.Type { return types.AnyType{Optional: true} }

// --- Compound values ---

type ArrayValue struct {
    Type  types.ArrayType
    Items []Value
}
func (v ArrayValue) ValueType() types.Type { return v.Type }

type MapValue struct {
    Type    types.MapType
    Entries []MapEntry // ordered; insertion order preserved
}
type MapEntry struct {
    Key   Value
    Value Value
}
func (v MapValue) ValueType() types.Type { return v.Type }

type PairValue struct {
    Type  types.PairType
    Left  Value
    Right Value
}
func (v PairValue) ValueType() types.Type { return v.Type }

// StructValue is a runtime instance of a WDL Struct.
type StructValue struct {
    Type   types.StructType
    Fields map[string]Value // member name → value
}
func (v StructValue) ValueType() types.Type { return v.Type }
```

### Table-Driven Test Pattern
```go
// wdl/types/types_test.go
package types_test

import (
    "testing"
    "github.com/bioflowy/flowy/wdl/types"
)

func TestTypeName(t *testing.T) {
    cases := []struct {
        name     string
        typ      types.Type
        wantName string
    }{
        {"Int", types.IntType{}, "Int"},
        {"Int?", types.IntType{Optional: true}, "Int?"},
        {"Array[String]", types.ArrayType{ItemType: types.StringType{}}, "Array[String]"},
        {"Array[String]+", types.ArrayType{ItemType: types.StringType{}, Nonempty: true}, "Array[String]+"},
        {"Array[String]+?", types.ArrayType{ItemType: types.StringType{}, Nonempty: true, Optional: true}, "Array[String]+?"},
        {"Map[String,Int]", types.MapType{KeyType: types.StringType{}, ValType: types.IntType{}}, "Map[String,Int]"},
        {"Pair[Int,Boolean]", types.PairType{LeftType: types.IntType{}, RightType: types.BooleanType{}}, "Pair[Int,Boolean]"},
        {"None", types.AnyType{Optional: true}, "None"},
        {"Any", types.AnyType{}, "Any"},
        {"Directory", types.DirectoryType{}, "Directory"},
        {"StructType", types.StructType{Name: "Sample"}, "Sample"},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            if got := tc.typ.TypeName(); got != tc.wantName {
                t.Errorf("TypeName() = %q, want %q", got, tc.wantName)
            }
        })
    }
}

func TestWithOptional(t *testing.T) {
    base := types.IntType{}
    opt := base.WithOptional(true)
    if !opt.IsOptional() {
        t.Error("WithOptional(true) did not set optional")
    }
    if base.IsOptional() {
        t.Error("WithOptional mutated original (must return copy)")
    }
}
```

### go.mod Initialization
```bash
cd /home/uehara/flowy
go mod init github.com/bioflowy/flowy
# Result: go.mod with:
#   module github.com/bioflowy/flowy
#   go 1.24
#   toolchain go1.24.6   (auto-added; acceptable)
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| WDL Object type (deprecated) | Struct type | WDL 1.0 | Object is OUT OF SCOPE; do not implement |
| No Directory type | Directory as first-class primitive | WDL 1.2 | TYPE-02 requires this; treat identically to File in the data model |
| Go 1.17 module graph | Go 1.21+ toolchain directive | Go 1.21 | `go mod init` may emit `toolchain go1.24.x`; accept it |

**Deprecated/outdated:**
- WDL `Object` type: deprecated in WDL 1.0, removed in 1.2. Do NOT implement `ObjectType` or `ObjectValue`.
- `Array[T]` `nonempty` quantifier in miniwdl uses `_nonempty` (private). In Go, use exported `Nonempty bool` field.

---

## Open Questions

1. **`Type` interface method set — is `WithOptional` sufficient?**
   - What we know: Phase 3 (type checker) needs to compare types, copy types with different quantifiers, and check coercibility.
   - What's unclear: Whether the type checker needs additional methods (e.g., `Equals(Type) bool`, `Parameters() []Type`) directly on the interface, or whether type assertions to concrete types are acceptable there.
   - Recommendation: Keep the interface minimal now (TypeName, IsOptional, WithOptional). Phase 3 planning will surface any gaps; adding methods to an interface is a breaking change so err on the side of narrow.

2. **`StructType.Members` mutability across phases**
   - What we know: Members are populated in Phase 2 (parser creates StructType from `struct` declarations). Phase 3 type checker reads them.
   - What's unclear: Whether a deep-copy helper on `StructType` should be defined now or deferred to Phase 2/3.
   - Recommendation: Defer to Phase 2. Document the reference-type hazard in Phase 1 code comments.

3. **`go.sum` and empty modules**
   - What we know: `go mod init` creates `go.mod`. `go.sum` is created on first `go get`/`go mod tidy`.
   - What's unclear: Whether to commit an empty `go.sum` or omit it.
   - Recommendation: Do not create `go.sum` manually. Run `go mod tidy` after `go mod init` — it will create `go.sum` (possibly empty). Commit both.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` package (stdlib, no version) |
| Config file | none — `go test ./...` is self-contained |
| Quick run command | `go test ./wdl/types/... ./wdl/values/...` |
| Full suite command | `go test ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TYPE-01 | All primitive types have correct TypeName() output and IsOptional() behavior | unit | `go test ./wdl/types/... -run TestTypeName` | ❌ Wave 0 |
| TYPE-01 | All primitive Value structs satisfy the Value interface and return correct ValueType() | unit | `go test ./wdl/values/... -run TestValueType` | ❌ Wave 0 |
| TYPE-02 | DirectoryType and DirectoryValue are present and behave identically to FileType/FileValue | unit | `go test ./wdl/types/... -run TestDirectory` | ❌ Wave 0 |
| TYPE-03 | Compound types (Array, Map, Pair, StructType) TypeName() includes type parameters | unit | `go test ./wdl/types/... -run TestCompound` | ❌ Wave 0 |
| TYPE-03 | ArrayType Nonempty flag is reflected in TypeName() output | unit | `go test ./wdl/types/... -run TestArrayNonempty` | ❌ Wave 0 |
| TYPE-04 | WithOptional(true/false) returns a copy; original unmodified | unit | `go test ./wdl/types/... -run TestWithOptional` | ❌ Wave 0 |
| TYPE-04 | NullValue.ValueType() returns AnyType{Optional: true} | unit | `go test ./wdl/values/... -run TestNullValue` | ❌ Wave 0 |
| TYPE-04 | AnyType{Optional: true}.TypeName() returns "None" | unit | `go test ./wdl/types/... -run TestAnyType` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./wdl/types/... ./wdl/values/...`
- **Per wave merge:** `go test ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `wdl/types/types.go` — all type structs (TYPE-01, TYPE-02, TYPE-03, TYPE-04)
- [ ] `wdl/types/types_test.go` — table-driven tests for TypeName, IsOptional, WithOptional
- [ ] `wdl/values/values.go` — all value structs (TYPE-01, TYPE-02, TYPE-03, TYPE-04)
- [ ] `wdl/values/values_test.go` — table-driven tests for ValueType
- [ ] `go.mod` — `go mod init github.com/bioflowy/flowy`

*(No existing test infrastructure — this is a greenfield project.)*

---

## Sources

### Primary (HIGH confidence)
- `/home/uehara/miniwdl/WDL/Type.py` — complete miniwdl type class hierarchy; authoritative semantic reference
- `/home/uehara/miniwdl/WDL/Value.py` — complete miniwdl value class hierarchy; authoritative semantic reference
- WDL 1.2 spec (raw.githubusercontent.com/openwdl/wdl/wdl-1.2/SPEC.md) — type system section; all six primitives, four compound types, optional quantifier, None literal

### Secondary (MEDIUM confidence)
- Go official docs (go.dev/doc/tutorial/create-module) — `go mod init` command and go.mod format verified
- Go official wiki (go.dev/wiki/TableDrivenTests) — table-driven test pattern
- Go 1.21 release notes — toolchain directive behavior in go.mod

### Tertiary (LOW confidence)
- WebSearch results on Go optional type patterns — cross-verified with official Go proposals (github.com/golang/go/issues/59202) confirming no stdlib Optional type; pointer vs bool-flag tradeoff

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — greenfield, stdlib only, no external dependencies to verify
- Architecture: HIGH — directly derived from miniwdl reference (read in full) + WDL 1.2 spec
- Pitfalls: HIGH — Go field/method naming collision is a known Go-specific trap verified against spec; map/slice reference-type behavior is a Go fundamental

**Research date:** 2026-03-10
**Valid until:** 2026-09-10 (stable domain — Go type system and WDL 1.2 spec do not change frequently)
