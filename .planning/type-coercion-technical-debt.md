# Type Coercion Technical Debt

## Problem

WDL requires implicit type coercion in declarations and expressions. Currently, coercion
is handled ad-hoc in multiple call sites via `coerceToType()` in `wdl/workflow/convert.go`,
which is a surface-level workaround rather than a proper solution.

### Current approach (workaround)

`coerceToType(v values.Value, target types.Type) values.Value` is called manually after
`eval.Eval()` at each declaration evaluation site:

- `wdl/workflow/scheduler.go` — workflow body declarations (line ~115)

This function handles: String↔File, String↔Directory, Int→Float.

### What's missing

1. **Workflow output declarations** (`wdl/workflow/workflow.go:109`) — output evaluation
   does not call `coerceToType`. If a workflow output has type `File` but the expression
   returns `StringValue`, it will fail.

2. **Task private declarations** (`wdl/task/task.go:99`) — task-level private declarations
   also do not apply coercion. `String x = some_file` would not be coerced.

3. **Scatter/conditional body declarations** — declarations inside scatter/if blocks
   evaluated in `executeScatter`/`executeConditional` may also need coercion.

4. **Expression-level coercion** — WDL specifies that `==` between String and File should
   compare the string representations. Currently, `eval.evalEqual` does not coerce operands,
   so `String == File` fails with "cannot compare". This should be handled in the eval
   layer, not by coercing declarations.

5. **Compound type coercion** — `Array[String]` to `Array[File]`, `Map[String, File]` to
   `Map[String, String]`, etc. The current `coerceToType` only handles scalar types.

6. **Struct field coercion** — when assigning a struct value to a struct-typed declaration,
   individual fields may need coercion (e.g., a `String` field to a `File` field).

### Root cause

The fundamental issue is that `eval.Eval()` is type-unaware. It evaluates expressions
purely based on runtime values without reference to the expected target type. WDL's type
coercion rules require knowing the target type at assignment/binding time.

### Proper solution

Introduce a centralized coercion layer in the `eval` package:

```go
// CoerceValue attempts to coerce v to the target type according to WDL spec rules.
// Returns the coerced value and true on success, or (v, false) if no coercion applies.
func CoerceValue(v values.Value, target types.Type, structDefs map[string]*ast.StructDef) (values.Value, bool)
```

This function should:

1. Handle all WDL-specified implicit coercions:
   - String ↔ File
   - String ↔ Directory
   - Int → Float
   - T → T? (optional wrapping)
   - Struct A → Struct B (structural compatibility)
   - Array/Map element-wise coercion

2. Be called from a single point: a new `EvalDecl(expr, env, ctx, targetType)` function
   that wraps `Eval()` + `CoerceValue()`.

3. Handle `==` comparison coercion in `evalEqual()` — when comparing File and String,
   coerce File to String (extract path) before comparing.

### Migration path

1. Create `eval.CoerceValue()` with full WDL coercion rules.
2. Create `eval.EvalDecl()` that calls `Eval()` then `CoerceValue()`.
3. Replace all `eval.Eval()` + manual `coerceToType()` calls with `eval.EvalDecl()`.
4. Update `evalEqual()` to apply coercion before comparing.
5. Remove `coerceToType()` from `wdl/workflow/convert.go`.

### Affected files

- `wdl/eval/eval.go` — add `CoerceValue`, `EvalDecl`, update `evalEqual`
- `wdl/workflow/scheduler.go` — replace `coerceToType` with `eval.EvalDecl`
- `wdl/workflow/workflow.go` — use `eval.EvalDecl` for output evaluation
- `wdl/workflow/convert.go` — remove `coerceToType`
- `wdl/task/task.go` — use `eval.EvalDecl` for private declarations
