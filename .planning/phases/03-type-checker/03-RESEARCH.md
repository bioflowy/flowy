# Phase 3: Type Checker - Research

**Researched:** 2026-03-10
**Domain:** WDL 1.2 type system — coercion, forward reference resolution, scatter/if promotion, struct validation
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- New package `wdl/checker`
- Public API: `checker.Check(doc *ast.Document) (TypeEnv, []error)`
- `TypeEnv` is `map[string]types.Type` (keyed by declaration name string; scopes managed via nested structure)
- Collect all type errors before returning — `flowy check` reports all errors at once
- AST is not mutated (remains immutable)
- **Two-pass approach**: Pass 1 scans all declarations and pre-populates the symbol table (TypeEnv); Pass 2 validates types against the fully-populated TypeEnv
- **Cycle detection**: toposort (Kahn's algorithm) detects circular dependencies and reports them as errors
- Undefined variable references inside scatter/if scopes are treated as scope rule violation errors, not cycle errors
- Scatter/if promotion: `arrayize(T) → Array[T]`, `optionalize(T) → T?`; idempotency check required: `optionalize` must check `IsOptional()==true` and skip (never produce `T??`)
- Nested scatter-inside-if: T → T? (if) → Array[T?] (scatter), applied in order
- Validate declared types in input/output sections; detect input type mismatches
- Validate struct field types; type struct literals and field access expressions
- Return types for primitive literals (Int, String, etc.) — "shallow literal" typing
- Implement all 6 WDL 1.2 coercion rules shallowly (no recursion into compound type parameters)
- Complex expression type inference (function calls, operator expressions) deferred to Phase 4
- Follow miniwdl's approach: two-pass + toposort for cycle detection (Kahn's algorithm)

### Claude's Discretion

- Concrete data structure for nested scope in TypeEnv (linked list of maps vs. stack, etc.)
- Error struct design for TypeError (reuse existing ParseError or define new type)
- Implementation details of the toposort

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| TYPE-05 | WDL 1.2 coercion rules 6 kinds (Int→Float, T→T?, String↔File, T→Array[T], etc.) implemented shallowly and non-recursively | Coercion table in Standard Stack; `Coerces()` function pattern in Code Examples |
| TYPE-06 | Type checker resolves all declarations in tasks and workflows including forward references | Two-pass algorithm with Kahn's toposort in Architecture Patterns |
| TYPE-07 | scatter block outputs promoted to `Array[T]`; if block outputs promoted to `T?` (idempotent) | `arrayize`/`optionalize` helpers; nested promotion ordering in Architecture Patterns |
| TYPE-08 | Struct type checker validates field types; types struct literals and field access expressions | Struct validation pattern; `StructDef` + `StructExpr` + `GetExpr` handling in Code Examples |
</phase_requirements>

---

## Summary

Phase 3 implements the `wdl/checker` package, which takes an `*ast.Document` produced by the parser and returns a `TypeEnv` plus a slice of type errors. The checker must resolve every declaration name across tasks, workflows, and structs, handling forward references via a two-pass approach. It must promote types through scatter and conditional scopes, enforce the 6 WDL 1.2 coercion rules, and validate struct field types and literal expressions.

The existing codebase gives the checker everything it needs without new external dependencies. Every AST node already embeds a `Pos` struct (File, Line, Col), providing precise source locations for error messages. The `types` package supplies all type constructors and `WithOptional(bool)` for idempotent optional promotion. The `parser.ParseError` struct provides a reference design for `TypeError`.

The core algorithm challenge is Kahn's topological sort for cycle detection within a scope. The scatter/if promotion logic is well-bounded: apply `optionalize` (with idempotency guard) then `arrayize` for each nesting level as declarations from inner scopes are registered into the outer scope. No external libraries are needed; standard Go maps and slices are sufficient for all data structures.

**Primary recommendation:** Implement `wdl/checker` as a two-file package (`checker.go` for the public API and scope logic, `coerce.go` for the coercion rules table), with `TypeError` as a new struct mirroring `ParseError`, and `Scope` as a linked-list of maps for lexical scoping.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/bioflowy/flowy/wdl/types` | local | All WDL type structs and `Type` interface | Already implemented in Phase 1; checker builds on it directly |
| `github.com/bioflowy/flowy/wdl/ast` | local | AST nodes with embedded `Pos` | Already implemented in Phase 2; checker consumes `*ast.Document` |
| Go standard library only | go 1.24.6 | maps, slices, sort | No external packages needed for a type checker of this scope |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `fmt` | stdlib | Error message formatting | In `TypeError.Error()` |
| `strings` | stdlib | Caret formatting in rustc-style errors | In `TypeError.Error()` if following ParseError design |
| `sort` | stdlib | Deterministic error ordering | Sort errors by (File, Line, Col) before return |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Linked-list Scope struct | `[]map[string]types.Type` (slice stack) | Slice stack is simpler but requires index management; linked list allows O(1) push/pop with no reallocation |
| New `TypeError` struct | Reuse `parser.ParseError` | `parser.ParseError` is in the `parser` package; importing it from `checker` creates an undesirable dependency. Define `checker.TypeError` instead |
| Hand-rolled toposort | `golang.org/x/graph` | `x/graph` is not in go.mod and adds a dependency for ~30 lines of Kahn's algorithm |

**Installation:**

No new packages required. The checker uses only local packages and the Go standard library.

## Architecture Patterns

### Recommended Package Structure

```
wdl/checker/
├── checker.go          # Public API: Check(), TypeEnv, Scope, two-pass logic
├── coerce.go           # Coerces() function implementing 6 WDL 1.2 rules
├── promote.go          # arrayize(), optionalize() helpers
└── checker_test.go     # Tests for TYPE-05 through TYPE-08
```

### Pattern 1: Two-Pass Type Resolution

**What:** Pass 1 traverses every `*ast.Decl` in tasks, workflows, and struct definitions and pre-populates a `TypeEnv` with the declared type (without validating the initializer expression). Pass 2 re-traverses every declaration and validates the initializer expression's type against the declared type, using the fully-populated `TypeEnv` for name lookups.

**When to use:** Any scope where forward references are legal — i.e., any task body or workflow body in WDL.

**Example:**

```go
// Source: CONTEXT.md / miniwdl design reference
func Check(doc *ast.Document) (TypeEnv, []error) {
    env := make(TypeEnv)
    var errs []error

    // Pass 1: collect all declarations
    for _, task := range doc.Tasks {
        s := newScope(nil)
        collectDecls(s, task.Inputs)
        collectDecls(s, task.Decls)
        collectDecls(s, task.Outputs)
        env[task.Name] = taskScopeType(s)
    }
    // Pass 2: validate types
    for _, task := range doc.Tasks {
        errs = append(errs, validateTask(task, env)...)
    }
    return env, errs
}
```

### Pattern 2: Scope as Linked List of Maps

**What:** A `Scope` struct holds a `map[string]types.Type` for the current lexical level and a `*Scope` pointer to the enclosing scope. Lookups traverse the chain; insertions write only into the current level.

**When to use:** Every nested construct — scatter body, if body, task body — gets its own `Scope` linked to the parent.

**Example:**

```go
// Source: standard Go type-checker idiom
type Scope struct {
    decls  map[string]types.Type
    parent *Scope
}

func newScope(parent *Scope) *Scope {
    return &Scope{decls: make(map[string]types.Type), parent: parent}
}

func (s *Scope) lookup(name string) (types.Type, bool) {
    if t, ok := s.decls[name]; ok {
        return t, true
    }
    if s.parent != nil {
        return s.parent.lookup(name)
    }
    return nil, false
}

func (s *Scope) define(name string, t types.Type) {
    s.decls[name] = t
}
```

### Pattern 3: Kahn's Algorithm for Cycle Detection

**What:** Build a dependency graph among declarations in a single scope (e.g., a task body). Run Kahn's algorithm: iteratively remove nodes with zero in-degree. If nodes remain after the loop, they form a cycle.

**When to use:** Any scope where a declaration's initializer references another declaration in the same scope. Scopes are: task body (Inputs + Decls + Outputs), workflow body (flat Decls + Output Decls).

**Example:**

```go
// Source: Kahn's algorithm (CLRS §22.4 / miniwdl _detect_cycles())
func toposort(nodes []string, edges map[string][]string) ([]string, error) {
    inDeg := make(map[string]int, len(nodes))
    for _, n := range nodes {
        inDeg[n] = 0
    }
    for _, deps := range edges {
        for _, d := range deps {
            inDeg[d]++
        }
    }
    var queue []string
    for _, n := range nodes {
        if inDeg[n] == 0 {
            queue = append(queue, n)
        }
    }
    var order []string
    for len(queue) > 0 {
        n := queue[0]; queue = queue[1:]
        order = append(order, n)
        for _, dep := range edges[n] {
            inDeg[dep]--
            if inDeg[dep] == 0 {
                queue = append(queue, dep)
            }
        }
    }
    if len(order) != len(nodes) {
        // find remaining nodes — these form cycles
        return nil, cycleError(nodes, order)
    }
    return order, nil
}
```

### Pattern 4: scatter/if Type Promotion

**What:** When registering declarations from a scatter or conditional body into the enclosing scope, wrap the declared type. For a scatter: `Array[T]`. For a conditional: `T?` (with idempotency guard). Nesting applies the wrapping in order from innermost to outermost.

**When to use:** Every `*ast.Scatter` and `*ast.Conditional` node encountered during Pass 1.

**Example:**

```go
// Source: CONTEXT.md / miniwdl arrayize/optionalize design
func arrayize(t types.Type) types.Type {
    return types.ArrayType{ItemType: t}
}

func optionalize(t types.Type) types.Type {
    if t.IsOptional() {
        return t // idempotency: never produce T??
    }
    return t.WithOptional(true)
}

// When processing scatter output into parent scope:
// promotedType = arrayize(innerDeclType)

// When processing if output into parent scope:
// promotedType = optionalize(innerDeclType)

// Nested scatter-inside-if: inner decl type T
//   After if promotion  → T? (optionalize)
//   After scatter promotion → Array[T?] (arrayize of the already-optionalized T?)
```

### Pattern 5: TypeError Struct

**What:** A new struct in the `checker` package that mirrors `parser.ParseError` but does not depend on the `parser` package. Carries `ast.Pos` for source location and a human-readable message.

**When to use:** Every type mismatch, undefined name, type cycle, or scope violation detected during Pass 2.

**Example:**

```go
// Source: parser.ParseError reference design in wdl/parser/parser.go
type TypeError struct {
    Pos     ast.Pos
    Message string
}

func (e *TypeError) Error() string {
    return fmt.Sprintf("type error: %s\n  --> %s:%d:%d",
        e.Message, e.Pos.File, e.Pos.Line, e.Pos.Col)
}
```

### Anti-Patterns to Avoid

- **Mutating the AST:** The checker must not set any field on `*ast.Decl` or any other AST node. All type information lives in `TypeEnv` and `Scope` only.
- **Recursive coercion:** The `Coerces()` function checks only the outermost type layer. It does not recurse into Array item types, Map key/value types, etc. Recursive coercion is out of scope for Phase 3.
- **Returning on first error:** The checker must accumulate all errors and return them all. Returning early loses information for the `flowy check` user.
- **Producing `T??`:** Always call `optionalize` through the helper that guards with `IsOptional()`. Never call `WithOptional(true)` directly without the guard.
- **Shallow struct copy mutation:** `StructType.Members` is a reference-type map. The comment in `types.go` says "Deep-copy before mutation (Phase 3 type checker responsibility)." If a `StructType` is looked up and needs a modified copy (e.g., to add `Optional`), deep-copy `Members` first.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Optional flag on any type | Custom wrapper struct | `t.WithOptional(true)` from `types.Type` interface | Already implemented; returns immutable copy |
| Checking if a type is optional | Custom flag inspection | `t.IsOptional()` | Already implemented on all type structs |
| Wrapping type in Array | Custom struct | `types.ArrayType{ItemType: t}` | Direct struct literal; no helper needed |
| Scope chain lookup | Recursive map search | `Scope.lookup(name)` linked-list traversal | Standard pattern; already described |
| Toposort | Third-party graph library | 30-line Kahn's algorithm in `checker.go` | No dependency needed; algorithm is trivial |

**Key insight:** The `types` package was deliberately designed for the type checker. `WithOptional(bool)` returns an immutable copy precisely so `optionalize()` can be implemented as a one-liner without mutation concerns.

## Common Pitfalls

### Pitfall 1: Double-Optional from Nested if Blocks

**What goes wrong:** An `if` block nested inside another `if` block might cause `optionalize` to be applied twice, producing `T??`.

**Why it happens:** Each `if` block during Pass 1 wraps inner declarations with `optionalize`. Without the idempotency guard, a declaration in the innermost `if` scope gets `optionalize` applied by the inner `if`, then `optionalize` applied again by the outer `if`.

**How to avoid:** Always use the `optionalize()` helper that checks `t.IsOptional()` before calling `t.WithOptional(true)`. The guard makes the operation idempotent.

**Warning signs:** `TypeName()` contains `??`; tests for nested `if` produce wrong types.

### Pitfall 2: StructType Members Map Aliasing

**What goes wrong:** A `StructType` retrieved from `TypeEnv` has its `Members` map modified in place, silently mutating all other references to the same struct type.

**Why it happens:** Go maps are reference types. `StructType` uses a `map[string]Type` for `Members`. The comment in `wdl/types/types.go` warns: "Deep-copy before mutation (Phase 3 type checker responsibility)."

**How to avoid:** Before adding a member or setting optional on a `StructType`, create a new `Members` map and copy all entries from the original.

**Warning signs:** Test for one struct type unexpectedly changes behavior of a different test; members appear in the wrong struct.

### Pitfall 3: Pass 1 Scope Pollution

**What goes wrong:** Scatter and conditional body declarations are registered into the global/task scope at their declared type (not promoted), causing the outer scope to have `T` where it should have `Array[T]` or `T?`.

**Why it happens:** Forgetting to apply `arrayize`/`optionalize` during Pass 1 when registering inner declarations outward.

**How to avoid:** In Pass 1, when processing a `*ast.Scatter` or `*ast.Conditional`, recursively collect inner declarations into a temporary inner scope, then register each inner declaration into the outer scope with the appropriate promotion applied.

**Warning signs:** Test for scatter output type shows `String` instead of `Array[String]`.

### Pitfall 4: Cycle Detection Scope Boundaries

**What goes wrong:** The cycle detector treats all declarations across all tasks/workflows as a single graph, finding false cycles between same-named variables in different tasks.

**Why it happens:** Kahn's algorithm builds a single dependency graph; if not scoped per-task, name collisions cause incorrect edges.

**How to avoid:** Run the toposort independently per scope (per task body, per workflow body). Each scope gets its own dependency graph.

**Warning signs:** Valid WDL with two tasks each having a variable named `x` reports a cycle error.

### Pitfall 5: Coercion Applied Recursively

**What goes wrong:** The coercion function recurses into Array item types and checks whether `Array[String]` coerces to `Array[File]` (it should be out of scope for Phase 3).

**Why it happens:** It seems helpful to extend coercion transitively, but Phase 3 scope is shallow-only. Recursive compound type coercion is Phase 4 territory.

**How to avoid:** `Coerces(from, to types.Type) bool` checks only the outermost type name and optional flag. It does not unwrap `ArrayType.ItemType` or `MapType.ValType`.

**Warning signs:** Tests for `Array[String]` → `Array[File]` coercion pass when they should be rejected in Phase 3.

## Code Examples

Verified patterns from existing codebase:

### TypeError struct design (mirrors ParseError)

```go
// Source: wdl/parser/parser.go ParseError reference design
// wdl/checker/checker.go
import (
    "fmt"
    "github.com/bioflowy/flowy/wdl/ast"
)

type TypeError struct {
    Pos     ast.Pos
    Message string
}

func (e *TypeError) Error() string {
    return fmt.Sprintf("type error: %s\n  --> %s:%d:%d",
        e.Message, e.Pos.File, e.Pos.Line, e.Pos.Col)
}
```

### TypeEnv and public API signature

```go
// Source: CONTEXT.md locked decision
// wdl/checker/checker.go

// TypeEnv maps declaration names to their resolved WDL types.
// The outer map key is the task/workflow name for top-level scopes.
// Inner maps carry per-declaration types for that scope.
type TypeEnv map[string]types.Type

// Check validates all type declarations in doc and returns the resolved TypeEnv.
// All type errors are collected before returning; the caller receives the full list.
// The AST is never mutated.
func Check(doc *ast.Document) (TypeEnv, []error) { ... }
```

### Coerces function (6 shallow rules)

```go
// Source: WDL 1.2 spec coercion table (raw.githubusercontent.com/openwdl/wdl/wdl-1.2/SPEC.md)
// wdl/checker/coerce.go

// Coerces reports whether a value of type `from` may be used where type `to` is expected.
// Rules are applied shallowly (outermost type only); compound type parameters are not inspected.
func Coerces(from, to types.Type) bool {
    // Rule 1: T → T? (if `to` is optional and `from` is the same base type)
    if to.IsOptional() && !from.IsOptional() {
        return Coerces(from, to.WithOptional(false))
    }
    // Strip matching optional wrappers
    if from.IsOptional() != to.IsOptional() {
        return false
    }
    switch f := from.(type) {
    case types.IntType:
        // Rule 2: Int → Float
        _, toFloat := to.(types.FloatType)
        return toFloat
    case types.StringType:
        // Rule 3a: String → File
        _, toFile := to.(types.FileType)
        // Rule 3b: String → Directory
        _, toDir := to.(types.DirectoryType)
        return toFile || toDir
    case types.FileType:
        // Rule 4: File → String
        _, toString := to.(types.StringType)
        return toString
    case types.ArrayType:
        // Rule 5: Array[T]+ → Array[T] (drop nonempty quantifier)
        toArr, ok := to.(types.ArrayType)
        return ok && f.Nonempty && !toArr.Nonempty
        // Note: item type coercion is NOT checked (shallow only)
    }
    return false
}
```

### Struct deep-copy before mutation

```go
// Source: wdl/types/types.go comment warning
// wdl/checker/checker.go
func deepCopyStructType(s types.StructType) types.StructType {
    members := make(map[string]types.Type, len(s.Members))
    for k, v := range s.Members {
        members[k] = v
    }
    return types.StructType{Name: s.Name, Members: members, Optional: s.Optional}
}
```

### Accessing Pos from any AST node

```go
// Source: wdl/ast/ast.go — every node embeds Pos as a plain struct field
// Direct field access: no pointer indirection needed
decl := task.Inputs[0]
pos := decl.Pos  // ast.Pos{File: "...", Line: 5, Col: 3}
err := &TypeError{Pos: pos, Message: fmt.Sprintf("...")}
```

### Struct literal validation (StructExpr)

```go
// Source: wdl/ast/expr.go StructExpr definition
// For each StructMember in StructExpr.Members:
//   1. Look up the field name in the StructDef's members
//   2. Determine the type of StructMember.Value (literal type or TypeEnv lookup)
//   3. Check Coerces(valueType, fieldType) or exact match
func checkStructExpr(expr *ast.StructExpr, scope *Scope, structDefs map[string]*ast.StructDef) (types.Type, *TypeError) {
    def, ok := structDefs[expr.TypeName]
    if !ok {
        return nil, &TypeError{Pos: expr.Pos, Message: fmt.Sprintf("undefined struct %q", expr.TypeName)}
    }
    for _, m := range expr.Members {
        // find field in def, validate type
    }
    // return the StructType from the resolved def
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Single-pass type checking (fails on forward refs) | Two-pass: collect all, then validate | WDL 1.1+ / miniwdl standard | Forward references work without reordering declarations |
| Mutable AST annotation | Immutable AST + separate TypeEnv | miniwdl / this project | AST can be re-checked; no side effects across passes |
| Recursive coercion | Shallow coercion in Phase 3, recursive in Phase 4 | This project's phased design | Keeps Phase 3 scope tight; Phase 4 handles complex expressions |

**Deprecated/outdated:**

- Object type: WDL 1.2 deprecated `Object`. Out of scope per REQUIREMENTS.md. Do not add `ObjectType` support.

## Open Questions

1. **Full TypeEnv shape for Phase 4 consumption**
   - What we know: Phase 4 expression evaluator will consume the `TypeEnv` returned by `checker.Check()`. The CONTEXT.md specifies `map[string]types.Type` keyed by declaration name.
   - What's unclear: Does Phase 4 need a hierarchical/nested `TypeEnv` (with task-scope and workflow-scope sub-maps), or a flat map with namespaced keys (e.g., `"task.sayHello.name"`)?
   - Recommendation: Design `TypeEnv` as `map[string]types.Type` at the top level with task/workflow names as first-level keys, and document the exact format in `checker.go` package comments. Phase 4 planning will refine this.

2. **Struct field validation depth**
   - What we know: Phase 3 validates struct field types and struct literal expressions. Struct members can themselves be struct types (nested structs per `types.wdl` fixture).
   - What's unclear: Does Phase 3 recursively validate nested struct fields (e.g., `Person.addr.city` in the test fixture), or only the immediate field types?
   - Recommendation: Validate immediate field types in Pass 1 (all field types are declared types from `StructDef.Members`). Nested struct access validation (GetExpr chains) can be covered in Phase 3 or deferred to Phase 4. The CONTEXT.md includes "field access expressions are correctly typed" under Phase 3 scope.

3. **Coercion rule 6 identity**
   - What we know: The CONTEXT.md mentions "6 coercion rules" and the WDL 1.2 spec table has 6 rows in the primary coercion section: `File←String`, `Directory←String`, `Float←Int`, `Y?←X`, `Array[Y]←Array[X]`, `Array[Y]←Array[X]+`.
   - What's unclear: The CONTEXT.md's list of 5 named rules is: `Int→Float`, `T→T?`, `String↔File`, `T→Array[T]`, plus "Remaining 2 rules per WDL 1.2 spec". The spec table shows `Directory←String` and `Array[Y]←Array[X]` (element coercion) as additional rules. `T→Array[T]` does not appear in the spec's primary table as a standalone rule.
   - Recommendation: Implement exactly what the spec table shows. The 6 rows are: (1) `File←String`, (2) `Directory←String`, (3) `Float←Int`, (4) `T→T?`, (5) `Array[Y]←Array[X]` (shallow, no item type check), (6) `Array[T]←Array[T]+`. Confirm with team before planning if `T→Array[T]` coercion should be included.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go built-in testing (`testing` package), go 1.24.6 |
| Config file | none (standard `go test ./...`) |
| Quick run command | `go test ./wdl/checker/...` |
| Full suite command | `go test ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TYPE-05 | `Coerces(IntType{}, FloatType{})` returns true; `Coerces(StringType{}, FileType{})` returns true; recursive coercion rejected | unit | `go test ./wdl/checker/... -run TestCoerces` | Wave 0 |
| TYPE-05 | `Coerces(ArrayType{...Nonempty:true}, ArrayType{...Nonempty:false})` returns true | unit | `go test ./wdl/checker/... -run TestCoerces` | Wave 0 |
| TYPE-06 | Task with forward-reference declaration resolves without error | unit | `go test ./wdl/checker/... -run TestForwardRef` | Wave 0 |
| TYPE-06 | Circular reference between two declarations produces cycle error | unit | `go test ./wdl/checker/... -run TestCycleDetection` | Wave 0 |
| TYPE-06 | Type mismatch (String assigned to Int) produces TypeError with correct Pos | unit | `go test ./wdl/checker/... -run TestTypeMismatch` | Wave 0 |
| TYPE-07 | scatter variable output typed as `Array[T]` | unit | `go test ./wdl/checker/... -run TestScatterPromotion` | Wave 0 |
| TYPE-07 | if block output typed as `T?`; nested scatter-in-if gives `Array[T?]` | unit | `go test ./wdl/checker/... -run TestIfPromotion` | Wave 0 |
| TYPE-07 | `optionalize` applied twice still gives `T?` not `T??` | unit | `go test ./wdl/checker/... -run TestOptionalizeIdempotent` | Wave 0 |
| TYPE-08 | Struct literal with all correct field types passes; missing field is error | unit | `go test ./wdl/checker/... -run TestStructLiteral` | Wave 0 |
| TYPE-08 | Field access `expr.field` on a StructType returns correct field type | unit | `go test ./wdl/checker/... -run TestStructFieldAccess` | Wave 0 |
| TYPE-08 | Undefined struct name in struct literal produces TypeError | unit | `go test ./wdl/checker/... -run TestStructUndefined` | Wave 0 |

### Sampling Rate

- **Per task commit:** `go test ./wdl/checker/...`
- **Per wave merge:** `go test ./...`
- **Phase gate:** `go test ./...` green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `wdl/checker/checker_test.go` — covers TYPE-05, TYPE-06, TYPE-07, TYPE-08 (all tests above)
- [ ] `wdl/checker/checker.go` — package skeleton with `Check()`, `TypeEnv`, `Scope`, `TypeError` signatures (stub implementations)
- [ ] `wdl/checker/coerce.go` — `Coerces()` function stub
- [ ] `wdl/checker/promote.go` — `arrayize()`, `optionalize()` function stubs
- [ ] WDL fixtures in `wdl/testdata/` — `forward_ref.wdl`, `scatter_if.wdl`, `type_mismatch.wdl`, `struct_literal.wdl` for integration-style tests

## Sources

### Primary (HIGH confidence)

- `wdl/types/types.go` (local) — Type interface, all primitive/compound type structs, `WithOptional`, `IsOptional`; StructType Members copy warning
- `wdl/ast/ast.go`, `wdl/ast/expr.go` (local) — Full AST node inventory, `Pos` embedded in every node, `StructExpr`, `Scatter`, `Conditional`, `Decl` structures
- `wdl/parser/parser.go` (local) — `ParseError` reference design for `TypeError`
- [WDL 1.2 SPEC.md — raw GitHub](https://raw.githubusercontent.com/openwdl/wdl/wdl-1.2/SPEC.md) — 6-row coercion table, scatter Array[T] output semantics

### Secondary (MEDIUM confidence)

- [miniwdl WDL.Type documentation](https://miniwdl.readthedocs.io/en/latest/_modules/WDL/Type.html) — `coerces()` method design, `_check_optional()` idempotency pattern, `arrayize`/`optionalize` design reference
- [openwdl/wdl SPEC.md at wdl-1.2](https://github.com/openwdl/wdl/blob/wdl-1.2/SPEC.md) — Type coercion section, scatter/if semantics

### Tertiary (LOW confidence)

- None — all material was verified against official sources or local codebase.

## Metadata

**Confidence breakdown:**

- Standard stack: HIGH — all packages are local (already implemented) or Go stdlib; no external dependencies
- Architecture: HIGH — two-pass + Kahn's algorithm is directly documented in CONTEXT.md as a locked decision; patterns are standard Go type-checker idioms
- Pitfalls: HIGH — StructType aliasing warning is explicitly in the codebase source code; double-optional and scope pollution are confirmed by the `optionalize` idempotency requirement in CONTEXT.md
- Coercion rules: MEDIUM — WDL 1.2 spec table has 6 rows confirmed; exact identity of rule 6 (whether `T→Array[T]` is included) has one open question; see Open Questions section

**Research date:** 2026-03-10
**Valid until:** 2026-05-10 (stable spec; 60-day validity)
