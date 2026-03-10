# Phase 3: Type Checker - Context

**Gathered:** 2026-03-10
**Status:** Ready for planning

<domain>
## Phase Boundary

Validate and resolve all type declarations in a WDL 1.2 document: forward reference resolution, scatter/if output type promotion, 6 shallow coercion rules, and struct field validation. Enables `flowy check` to report type errors with source locations. Expression evaluation (computing values) is Phase 4 scope.

</domain>

<decisions>
## Implementation Decisions

### Package structure and API
- New package `wdl/checker`
- Public API: `checker.Check(doc *ast.Document) (TypeEnv, []error)`
- `TypeEnv` is `map[string]types.Type` (keyed by declaration name string; scopes managed via nested structure)
- Collect all type errors before returning — `flowy check` reports all errors at once
- AST is not mutated (remains immutable)

### Forward reference resolution algorithm
- **Two-pass approach**:
  - Pass 1: Scan all declarations and pre-populate the symbol table (TypeEnv)
  - Pass 2: Validate types against the fully-populated TypeEnv
- **Cycle detection**: toposort (Kahn's algorithm) detects circular dependencies and reports them as errors
- Undefined variable references inside scatter/if scopes are treated as **scope rule violation errors**, not cycle errors

### scatter/if type promotion
- Promoted in real-time during Pass 2 (when inner declarations are registered into the outer scope)
- Helper functions: `arrayize(T) → Array[T]`, `optionalize(T) → T?`
- **Idempotency check required**: `optionalize` must check `IsOptional()==true` and skip promotion (never produce `T??`)
- Nested scatter-inside-if: T → T? (if) → Array[T?] (scatter), applied in order

### Type validation scope (Phase 3)
- Validate declared types in input/output sections
- Detect input type mismatches (e.g., String passed to Int declaration) — reported in Phase 3
- Validate struct field types; type struct literals and field access expressions
- Return types for primitive literals (Int, String, etc.) — "shallow literal" typing
- Implement all 6 WDL 1.2 coercion rules **shallowly** (no recursion into compound type parameters)
- Complex expression type inference (function calls, operator expressions) deferred to Phase 4

### WDL 1.2 coercion rules (6 rules, shallow)
- `Int → Float`
- `T → T?`
- `String ↔ File`
- `T → Array[T]`
- Remaining 2 rules per WDL 1.2 spec
- Shallow only: no recursive application into compound type parameters

### Claude's Discretion
- Concrete data structure for nested scope in TypeEnv (linked list of maps vs. stack, etc.)
- Error struct design for TypeError (reuse existing ParseError or define new type)
- Implementation details of the toposort

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `wdl/types` package: `Type` interface, all primitive/compound/Optional types implemented. `WithOptional(bool)` makes idempotency checks straightforward.
- `wdl/ast` package: Every AST node embeds `Pos` (File, Line, Col) — precise source locations available for all type errors.
- `wdl/parser` package: `ParseError` struct (with line/col) already defined — reference design for TypeError.

### Established Patterns
- Errors include file path and line number (`Pos` embedded in every AST node)
- Cross-package boundaries use interfaces (`Type` interface, `ContainerRunner` interface) — checker should follow the same pattern

### Integration Points
- `checker.Check()` input: `*ast.Document` returned by `parser.Parse()`
- `checker.Check()` output: `TypeEnv` consumed by Phase 4 `evaluator` package
- Phase 7 `flowy check` CLI command calls `checker.Check()` directly

</code_context>

<specifics>
## Specific Ideas

- Follow miniwdl's approach: two-pass + toposort for cycle detection (miniwdl uses Kahn's algorithm in `_detect_cycles()`)
- Port miniwdl's `arrayize`/`optionalize` helper design directly to Go

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 03-type-checker*
*Context gathered: 2026-03-10*
