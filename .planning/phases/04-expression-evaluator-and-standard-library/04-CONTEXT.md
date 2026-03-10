# Phase 4: Expression Evaluator and Standard Library - Context

**Gathered:** 2026-03-10
**Status:** Ready for planning

<domain>
## Phase Boundary

Evaluate any WDL 1.2 expression to a typed Value, and implement all ~30 WDL 1.2 stdlib functions — without requiring a running Docker daemon. This phase delivers a standalone `wdl/eval` package that Phase 5 (Task Execution) will call with a populated ExecContext.

</domain>

<decisions>
## Implementation Decisions

### Stdlib file I/O — temp file location
- `write_lines()`, `write_tsv()`, `write_map()`, `write_json()` and other `write_*` functions create temp files using `os.MkdirTemp()` (real filesystem, no abstraction layer).
- Unit tests use `t.TempDir()` for temp file isolation; cleanup is automatic.
- No FS interface / in-memory mock needed — real I/O is idiomatic Go and matches miniwdl behavior.

### Stdlib file I/O — execution-context-dependent functions
- `stdout()` and `stderr()` return **runtime error** (`EvalError`) when called outside a task execution context.
  - Message: `"stdout()/stderr() only valid inside a task execution"` (or similar).
  - Phase 5 wires real stdout/stderr paths via `ExecContext` before calling the evaluator.
- `glob()` requires `ExecContext{WorkDir string}`:
  - If `WorkDir == ""` (not in task), return runtime error.
  - Phase 5 sets `WorkDir` to the task execution directory.
  - Unit tests inject a `t.TempDir()` path as `WorkDir`.
- `size()` calls `os.Stat()` directly on the host path from `FileValue.Value`.
  - In Phase 5+, the path map translates container-internal paths to host paths **before** calling the evaluator, so `size()` always receives a valid host path.

### ExecContext struct
- The evaluator receives an `ExecContext` struct (or pointer) with at minimum:
  - `WorkDir string` — task execution directory; empty string means "not in task context"
  - `StdoutPath string` — path to captured stdout file; empty string means "not in task context"
  - `StderrPath string` — path to captured stderr file; empty string means "not in task context"
- Phase 4 unit tests: pass zero-value `ExecContext{}` for pure expression tests; inject temp paths for file I/O tests.
- Phase 5: populate all fields before evaluating task output expressions.

### Claude's Discretion
- Runtime error type design (`EvalError` struct shape, whether to embed `ast.Pos`)
- Whether evaluator stops at first runtime error or attempts to continue (single-expression eval → stop at first makes sense)
- None-guard strategy in operator dispatch (per-operator or top-level)
- Command dedent edge case handling (all-whitespace lines, None in placeholder)
- Stdlib function dispatch mechanism (registry map vs. switch)

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `wdl/values`: Complete `Value` interface + all concrete types (`BooleanValue`, `IntValue`, `FloatValue`, `StringValue`, `FileValue`, `DirectoryValue`, `NullValue{}`, `ArrayValue`, `MapValue`, `PairValue`, `StructValue`) — evaluator produces these directly.
- `wdl/ast/expr.go`: All AST expression nodes (`BoolLit`, `IntLit`, `FloatLit`, `NullLit`, `StringExpr`, `CommandExpr`, `ArrayExpr`, `MapExpr`, `PairExpr`, `StructExpr`, `IfThenElse`, `BinaryExpr`, `UnaryExpr`, `IndexExpr`, `GetExpr`, `ApplyExpr`, `IdentExpr`) — evaluator dispatches on these.
- `wdl/checker.Scope`: Linked-list scope structure — model for runtime `Env` (same shape, but maps names to `values.Value` instead of `types.Type`).
- `wdl/checker.TypeError`: Error pattern with `ast.Pos` — `EvalError` should follow the same design.

### Established Patterns
- `NullValue{}` (not nil) is the sentinel for absent optional values — evaluator must match against `NullValue{}`, never compare to nil.
- All errors include source `ast.Pos` (File, Line, Col) — `EvalError` must follow this pattern.
- Package boundary: new package `wdl/eval` (mirrors `wdl/checker` structure).

### Integration Points
- Input: `ast.Expr` node + runtime `Env` (variable bindings as `values.Value`).
- Input from checker: `checker.TypeEnv` consumed during Phase 4 for expression type validation completion.
- Output: `values.Value` or `EvalError`.
- Phase 5 wires `ExecContext{WorkDir, StdoutPath, StderrPath}` when calling the evaluator for task output sections.

</code_context>

<specifics>
## Specific Ideas

No specific references or "I want it like X" moments — open to standard approaches following miniwdl semantics.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 04-expression-evaluator-and-standard-library*
*Context gathered: 2026-03-10*
