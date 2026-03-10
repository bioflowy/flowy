# Phase 1: Type Data Structures - Context

**Gathered:** 2026-03-10
**Status:** Ready for planning

<domain>
## Phase Boundary

Define all WDL 1.2 type and value data structures in Go; initialize the Go module. No parsing, evaluation, or execution — this is the data model that every downstream phase builds on.

Specifically: implement the `Type` interface and all concrete type structs (primitive + compound + optional), implement the `Value` interface and all concrete value structs, initialize `go.mod` as `github.com/bioflowy/flowy` with Go 1.24.

</domain>

<decisions>
## Implementation Decisions

### Naming alignment with miniwdl

- Go type names **mirror miniwdl's Python names closely**: `Int`, `Float`, `Boolean`, `String`, `File`, `Directory`, `Array`, `Map`, `Pair` (package `wdl/types`)
- This makes cross-referencing `WDL/Type.py` and `WDL/Value.py` straightforward when porting logic
- Exception: WDL Struct type definition → `StructType` (not `Struct`) to avoid ambiguity with Go's `struct` keyword concept

### Package layout

- **Two separate packages**: `wdl/types` and `wdl/values` (mirroring miniwdl's Type.py / Value.py split)
- Import paths: `github.com/bioflowy/flowy/wdl/types` and `github.com/bioflowy/flowy/wdl/values`
- Downstream packages (type checker, evaluator) import only what they need — type checker imports `types` but not `values`

### Value struct naming

- Value structs use explicit suffix: `IntValue`, `FloatValue`, `BooleanValue`, `StringValue`, `FileValue`, `DirectoryValue`, `ArrayValue`, `MapValue`, `PairValue`
- Struct value instance → `StructValue` (paired with `StructType` in the types package)
- Makes call sites unambiguous: `types.Int{}` vs `values.IntValue{}`

### Struct naming (special case)

- WDL Struct type definition → `StructType` in `wdl/types`
  - Fields: `TypeName string`, `Members map[string]Type`
- WDL Struct runtime instance → `StructValue` in `wdl/values`
  - Fields: `Type types.StructType`, `Fields map[string]Value`

### Claude's Discretion

- Optional (`T?`) representation in Go — pointer-based, Optional[T] wrapper, or tagged union
- Type interface methods beyond type tagging (String(), Equals(), IsOptional(), etc.)
- None literal representation
- File/Directory value backing type (string path vs. richer struct)
- Internal package layout beyond the two top-level packages (e.g., `internal/` subpackages)

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets

- None — greenfield project. No existing Go code.

### Established Patterns

- Reference implementation: `../miniwdl/WDL/Type.py` and `../miniwdl/WDL/Value.py` define the source of truth for type semantics and naming
- `../miniwdl/WDL/Tree.py` defines AST nodes (relevant to Phase 2, but type fields appear here)

### Integration Points

- `wdl/types` will be imported by: Phase 2 (AST nodes carry type fields), Phase 3 (type checker), Phase 4 (expression evaluator)
- `wdl/values` will be imported by: Phase 4 (evaluator produces values), Phase 5 (task execution binds values)
- `go.mod` initialized here; all downstream phases add their dependencies

</code_context>

<specifics>
## Specific Ideas

- No specific product references — standard Go conventions apply within the decisions above

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 01-type-data-structures*
*Context gathered: 2026-03-10*
