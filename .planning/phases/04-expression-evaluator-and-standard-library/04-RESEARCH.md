# Phase 4: Expression Evaluator and Standard Library - Research

**Researched:** 2026-03-10
**Domain:** Go expression evaluation, WDL 1.2 semantics, stdlib function dispatch
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- `write_lines()`, `write_tsv()`, `write_map()`, `write_json()` and other `write_*` functions create temp files using `os.MkdirTemp()` (real filesystem, no abstraction layer). Unit tests use `t.TempDir()` for temp file isolation; cleanup is automatic. No FS interface / in-memory mock needed.
- `stdout()` and `stderr()` return runtime error (`EvalError`) when called outside a task execution context. Message: `"stdout()/stderr() only valid inside a task execution"` (or similar). Phase 5 wires real stdout/stderr paths via `ExecContext`.
- `glob()` requires `ExecContext{WorkDir string}`. If `WorkDir == ""`, return runtime error. Phase 5 sets `WorkDir` to the task execution directory. Unit tests inject a `t.TempDir()` path as `WorkDir`.
- `size()` calls `os.Stat()` directly on the host path from `FileValue.Value`.
- The evaluator receives an `ExecContext` struct with at minimum: `WorkDir string`, `StdoutPath string`, `StderrPath string`. Empty string means "not in task context".
- Phase 4 unit tests: pass zero-value `ExecContext{}` for pure expression tests; inject temp paths for file I/O tests.
- New package `wdl/eval` (mirrors `wdl/checker` structure).

### Claude's Discretion
- Runtime error type design (`EvalError` struct shape, whether to embed `ast.Pos`)
- Whether evaluator stops at first runtime error or attempts to continue (single-expression eval → stop at first makes sense)
- None-guard strategy in operator dispatch (per-operator or top-level)
- Command dedent edge case handling (all-whitespace lines, None in placeholder)
- Stdlib function dispatch mechanism (registry map vs. switch)

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| EXPR-01 | Arithmetic, comparison, logical operators including `**` evaluate to correct typed Values | Binary/unary operator dispatch; `math.Pow` for `**`; integer division truncates toward zero |
| EXPR-02 | if-then-else expression evaluates the condition and returns appropriate branch | `IfThenElse` AST node dispatch; condition must be `BooleanValue` |
| EXPR-03 | Member access (`struct.field`, `pair.left`, `pair.right`) returns correct element or runtime error | `GetExpr` dispatch on `StructValue.Fields` and `PairValue.Left/Right` |
| EXPR-04 | Array/map index access returns correct element or runtime error on out-of-bounds | `IndexExpr` dispatch; `ArrayValue.Items[i]` with bounds check; `MapValue.Entries` linear scan |
| EXPR-05 | Function calls evaluate stdlib functions | `ApplyExpr` dispatch to function registry |
| EXPR-06 | Command template expanded by `~{}` placeholder substitution + dedent algorithm | `CommandExpr` evaluation: sentinel substitution then minimum-indent detection then strip then substitute |
| EXPR-07 | Command block dedent: sentinel-before-indent, not indent-before-sentinel | Confirmed by miniwdl source: replace placeholders with literal `"~{}"` sentinel before computing min-indent |
| STDLIB-01 | File I/O functions: `read_lines`, `read_tsv`, `read_map`, `read_json`, `read_boolean`, `read_int`, `read_float`, `read_string`, `write_lines`, `write_tsv`, `write_map`, `write_json` | `os.ReadFile` + `os.MkdirTemp`; `encoding/json` for JSON; TSV via `strings.Split` |
| STDLIB-02 | Filesystem functions: `glob`, `basename`, `dirname`, `size`, `stdout`, `stderr` | `filepath.Glob`, `filepath.Base`, `filepath.Dir`, `os.Stat().Size()`, `ExecContext` fields |
| STDLIB-03 | Array functions: `length`, `flatten`, `transpose`, `zip`, `cross`, `prefix`, `suffix`, `quote`, `squote`, `unzip`, `select_all`, `select_first`, `chunk` (WDL 1.2) | Pure Go slice operations; `chunk` is WDL 1.2 new |
| STDLIB-04 | String functions: `sub`, `find` (WDL 1.2), `matches` (WDL 1.2), `join_paths` (WDL 1.2) | `regexp.MustCompile` + `ReplaceAllString`; POSIX ERE via `regexp` package |
| STDLIB-05 | Type conversion/utility: `defined`, `keys`, `values` (WDL 1.2), `as_map`, `as_pairs`, `collect_by_key`, `contains` (WDL 1.2), `contains_key` (WDL 1.2) | Map iteration; `values` is WDL 1.2 new |
| STDLIB-06 | Numeric functions: `ceil`, `floor`, `round` (half-up), `min`, `max` | `math.Ceil`, `math.Floor`; `round` = `math.Floor(x+0.5)` for half-up (NOT `math.Round` which is round-half-away-from-zero in Go, which matches) |
</phase_requirements>

---

## Summary

Phase 4 builds the `wdl/eval` package — a tree-walking interpreter for WDL 1.2 expressions. The evaluator dispatches on the existing `ast.Expr` type hierarchy (14 node types), produces values from the existing `wdl/values` package, and reports errors via a new `EvalError` type modeled after `checker.TypeError`. The standard library (~30 functions) is implemented as a dispatch table inside the same package, organized into six groups matching the requirements.

The most implementation-sensitive pieces are: (1) the command block dedent algorithm (sentinel-before-indent, confirmed by miniwdl source), (2) `round()` must use half-up semantics (`math.Floor(x+0.5)`), and (3) WDL 1.2 new functions (`chunk`, `find`, `matches`, `join_paths`, `values`, `contains`, `contains_key`) require POSIX ERE regex via Go's `regexp` package.

The entire phase is self-contained: no Docker, no running containers, no network. All file I/O uses real filesystem. Tests use `t.TempDir()`. The `ExecContext` struct gates context-sensitive functions (`stdout`, `stderr`, `glob`).

**Primary recommendation:** Implement `wdl/eval` as a single `Eval(expr ast.Expr, env *Env, ctx ExecContext) (values.Value, error)` function with a type-switch, plus a `stdlib` map of `string -> func([]values.Value, ExecContext) (values.Value, error)`.

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `encoding/json` | stdlib | JSON read/write for `read_json`, `write_json` | Built-in, no deps |
| `regexp` | stdlib | POSIX ERE for `sub`, `find`, `matches` | Built-in; Go uses RE2 syntax (POSIX ERE compatible for these operations) |
| `math` | stdlib | `math.Pow`, `math.Ceil`, `math.Floor`, `math.Round` | Built-in |
| `path/filepath` | stdlib | `filepath.Base`, `filepath.Dir`, `filepath.Glob`, `filepath.Join` | Built-in |
| `os` | stdlib | `os.ReadFile`, `os.WriteFile`, `os.MkdirTemp`, `os.Stat` | Built-in |
| `strings` | stdlib | TSV splitting, string manipulation | Built-in |
| `bufio` | stdlib | Line-by-line reading for `read_lines` | Built-in |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `fmt` | stdlib | `EvalError.Error()` formatting | Always |
| `strconv` | stdlib | `read_int`, `read_float`, `read_boolean` parsing | Scalar file reads |
| `sort` | stdlib | Stable map key ordering in `keys()` if needed | Map functions |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `regexp` (RE2) | `regexp/syntax` POSIX | WDL spec says POSIX ERE; Go's `regexp` is RE2, which covers all patterns used in WDL stdlib; acceptable |
| Registry map | Giant switch | Map is more maintainable as function count grows; switch avoids allocation; map is idiomatic for large dispatch |
| `math.Round` | `math.Floor(x+0.5)` | `math.Round` in Go is "round half away from zero" — for positive numbers this matches half-up, but for negative (e.g. -2.5) `math.Round` gives -3 while half-up gives -2. Use `math.Floor(x+0.5)` to be explicit about WDL semantics. |

**Installation:** No additional packages needed. All dependencies are Go standard library.

---

## Architecture Patterns

### Recommended Project Structure
```
wdl/eval/
├── eval.go          # Eval() entry point, type-switch dispatch on ast.Expr
├── env.go           # Env struct (runtime variable bindings: name -> values.Value)
├── error.go         # EvalError struct with ast.Pos; mirrors checker.TypeError
├── context.go       # ExecContext struct definition
├── stdlib.go        # stdlib dispatch map + all function implementations
├── command.go       # command template expansion + dedent algorithm
└── eval_test.go     # tests for all EXPR-* and STDLIB-* requirements
```

### Pattern 1: Tree-walking Evaluator with Type-switch

**What:** Single recursive `Eval` function dispatching on `ast.Expr` concrete types.
**When to use:** Standard for small-to-medium expression languages with a fixed node set.

```go
// Source: established Go pattern for AST interpreters
func Eval(expr ast.Expr, env *Env, ctx ExecContext) (values.Value, error) {
    switch e := expr.(type) {
    case *ast.BoolLit:
        return values.BooleanValue{Type: types.BooleanType{}, Value: e.Value}, nil
    case *ast.IntLit:
        return values.IntValue{Type: types.IntType{}, Value: e.Value}, nil
    case *ast.BinaryExpr:
        return evalBinary(e, env, ctx)
    case *ast.IfThenElse:
        return evalIfThenElse(e, env, ctx)
    case *ast.ApplyExpr:
        return evalApply(e, env, ctx)
    // ... all 14 ast.Expr types
    default:
        return nil, &EvalError{Pos: exprPos(expr), Message: "unknown expression type"}
    }
}
```

### Pattern 2: Runtime Environment (Env)

**What:** Linked-list scope mirroring `checker.Scope` but mapping names to `values.Value`.
**When to use:** For variable lookup during expression evaluation.

```go
// Source: mirrors wdl/checker/checker.go Scope pattern
type Env struct {
    bindings map[string]values.Value
    parent   *Env
}

func (e *Env) Lookup(name string) (values.Value, bool) {
    for cur := e; cur != nil; cur = cur.parent {
        if v, ok := cur.bindings[name]; ok {
            return v, true
        }
    }
    return nil, false
}
```

### Pattern 3: EvalError

**What:** Runtime error type with source position, mirrors `checker.TypeError`.

```go
// Source: mirrors wdl/checker/checker.go TypeError
type EvalError struct {
    Pos     ast.Pos
    Message string
}

func (e *EvalError) Error() string {
    return fmt.Sprintf("eval error: %s\n  --> %s:%d:%d",
        e.Message, e.Pos.File, e.Pos.Line, e.Pos.Col)
}
```

### Pattern 4: Stdlib Dispatch Map

**What:** Map from function name to implementation function.
**When to use:** ~30 functions — a map is more maintainable than a 30-arm switch.

```go
// Source: common Go dispatch pattern for stdlib implementations
type stdlibFn func(args []values.Value, ctx ExecContext) (values.Value, error)

var stdlibFuncs = map[string]stdlibFn{
    "length":      stdLength,
    "flatten":     stdFlatten,
    "read_lines":  stdReadLines,
    "sub":         stdSub,
    // ...
}

func evalApply(e *ast.ApplyExpr, env *Env, ctx ExecContext) (values.Value, error) {
    fn, ok := stdlibFuncs[e.Name]
    if !ok {
        return nil, &EvalError{Pos: e.Pos, Message: fmt.Sprintf("unknown function %q", e.Name)}
    }
    args := make([]values.Value, len(e.Args))
    for i, a := range e.Args {
        v, err := Eval(a, env, ctx)
        if err != nil {
            return nil, err
        }
        args[i] = v
    }
    return fn(args, ctx)
}
```

### Pattern 5: Command Dedent Algorithm (EXPR-06, EXPR-07)

**What:** Two-pass algorithm — sentinel substitution then indent detection then strip then actual substitution.
**When to use:** Mandatory for all `CommandExpr` evaluation.

The algorithm (confirmed from miniwdl source `WDL.Expr.String._dedent()`):

```
Step 1: Evaluate each PlaceholderPart to its string representation (sentinel phase)
        Replace each placeholder position in the raw text with the literal string "~{}"
        This forms a pseudo-string for indent analysis.

Step 2: Split pseudo-string into lines.
        Find minimum leading whitespace across all non-empty, non-whitespace-only lines.
        (Empty lines and all-whitespace lines are excluded from min-indent computation.)

Step 3: Strip min-indent leading whitespace from every non-empty line.

Step 4: Replace "~{}" sentinel positions with the actual evaluated placeholder values.
```

```go
// Source: adapted from miniwdl WDL/Expr.py String._dedent()
func evalCommand(e *ast.CommandExpr, env *Env, ctx ExecContext) (values.Value, error) {
    // Phase 1: build sentinel string + collect evaluated placeholder values
    const sentinel = "~{}"
    var sentinelText strings.Builder
    evaledParts := make([]string, 0, len(e.Parts))

    for _, part := range e.Parts {
        switch p := part.(type) {
        case *ast.RawStringPart:
            sentinelText.WriteString(p.Val)
        case *ast.PlaceholderPart:
            v, err := Eval(p.Expr, env, ctx)
            if err != nil {
                return nil, err
            }
            sentinelText.WriteString(sentinel)
            evaledParts = append(evaledParts, valueToString(v, p.Options))
        }
    }

    // Phase 2: compute minimum indent from sentinel pseudo-string
    minIndent := computeMinIndent(sentinelText.String())

    // Phase 3+4: rebuild output with indent stripped and sentinels replaced
    result := applyDedentAndSubstitute(sentinelText.String(), minIndent, evaledParts, sentinel)
    return values.StringValue{Type: types.StringType{}, Value: result}, nil
}
```

### Anti-Patterns to Avoid

- **nil for NullValue:** Always use `values.NullValue{}`, never `nil` for absent optional values. The existing codebase enforces this.
- **Direct type-name comparison in dispatch:** Use Go type assertions (`case *ast.BinaryExpr`), not string comparison on type names.
- **Mutable shared ArrayValue/MapValue:** Deep-copy Items/Entries slices before mutation. The `values.go` comment explicitly flags this.
- **Wrong round() implementation:** `math.Round(-2.5)` returns `-3` (round half away from zero). WDL 1.2 specifies half-up. Use `math.Floor(x+0.5)` for correct WDL semantics for all floats.
- **Dedent before substitute:** Computing min-indent on the raw template text (before placeholder expansion) is correct. Computing it after would include expanded multi-line values in the indent calculation, which is wrong.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON parsing/serialization | Custom parser | `encoding/json` | Handles nesting, escaping, all WDL JSON types |
| Regex for `sub`/`find`/`matches` | Custom matcher | `regexp.MustCompile` | POSIX ERE semantics; handles backtracking correctly |
| File path manipulation | String concatenation | `filepath.Base`, `filepath.Dir`, `filepath.Join` | Handles OS-specific separators, `..`, `.` correctly |
| Glob expansion | `filepath.Walk` | `filepath.Glob` | Spec-compliant glob matching |
| Minimum indent detection | Ad-hoc loop | `strings.Builder` + `strings.Split("\n")` + manual min | Simple enough to hand-roll safely; see algorithm above |

**Key insight:** All stdlib operations are thin wrappers over Go stdlib. Never build custom I/O, regex, or JSON handling.

---

## Common Pitfalls

### Pitfall 1: Wrong `round()` for Negative Half-Values

**What goes wrong:** `math.Round(-2.5)` returns `-3`; WDL 1.2 specifies half-up, which for `-2.5` is `-2`.
**Why it happens:** Go's `math.Round` is "round half away from zero", not "round half up toward positive infinity".
**How to avoid:** Implement as `int64(math.Floor(float64(x) + 0.5))`.
**Warning signs:** Test case `round(-2.5)` returns -3 instead of -2.

### Pitfall 2: Dedent Computed on Post-Expansion Text

**What goes wrong:** If placeholder values are substituted before computing min-indent, multi-line placeholder values change the indentation structure and the wrong amount is stripped.
**Why it happens:** Natural implementation order — evaluate then measure.
**How to avoid:** Sentinel-first: replace placeholders with fixed-width `"~{}"` before measuring, then substitute actual values after stripping.
**Warning signs:** Command templates with multi-line placeholder values produce incorrect indentation.

### Pitfall 3: NullValue Compared with nil

**What goes wrong:** `if v == nil` misses `NullValue{}` — the evaluator fails to guard optional values correctly.
**Why it happens:** Habit from nullable-pointer languages.
**How to avoid:** Always check `_, isNull := v.(values.NullValue)` or type-assert to `values.NullValue`.
**Warning signs:** Operators on optional values panic or return wrong results when the value is absent.

### Pitfall 4: MapValue Lookup is O(n)

**What goes wrong:** Map index access must scan `Entries []MapEntry` linearly (no hash map backing).
**Why it happens:** `MapValue` uses a slice to preserve insertion order (WDL Map iteration order is defined).
**How to avoid:** Implement `MapValue` index as a sequential scan. Do not build a secondary hash map. For small maps this is fine; WDL maps in practice are small.
**Warning signs:** N/A for correctness, but note for performance if very large maps appear in tests.

### Pitfall 5: ExecContext-Dependent Functions Called Without Context

**What goes wrong:** `stdout()`, `stderr()`, `glob()` called when `ExecContext` fields are empty strings — they must return `EvalError`, not panic.
**Why it happens:** Missing guard check at function entry.
**How to avoid:** Each context-sensitive stdlib function checks `ctx.StdoutPath == ""` (or `ctx.WorkDir == ""`) first and returns `EvalError` if empty.
**Warning signs:** Test without `ExecContext` panics or returns wrong value.

### Pitfall 6: `contains` type matching for Map keys

**What goes wrong:** `contains(Array[X], X)` uses Go `==` operator which may not work correctly for compound `values.Value` types (ArrayValue, MapValue are not comparable).
**Why it happens:** WDL `contains` is defined only for arrays of primitive type `P` (same constraint as `Map` keys).
**How to avoid:** Only support primitive Value types (`BooleanValue`, `IntValue`, `FloatValue`, `StringValue`, `FileValue`) in `contains` comparison. Use a helper that compares by concrete type + value.
**Warning signs:** Compiler error or panic when comparing non-comparable types.

---

## Code Examples

### Binary Operator Dispatch

```go
// Source: standard Go tree-walking interpreter pattern
func evalBinary(e *ast.BinaryExpr, env *Env, ctx ExecContext) (values.Value, error) {
    left, err := Eval(e.Left, env, ctx)
    if err != nil {
        return nil, err
    }
    right, err := Eval(e.Right, env, ctx)
    if err != nil {
        return nil, err
    }
    switch e.Op {
    case lexer.TokPlus:
        return evalAdd(left, right, e.Pos)
    case lexer.TokMinus:
        return evalSub(left, right, e.Pos)
    case lexer.TokStarStar:
        return evalPow(left, right, e.Pos)
    case lexer.TokEqEq:
        return evalEq(left, right, e.Pos)
    case lexer.TokAnd:
        return evalAnd(left, right, e.Pos)
    // ...
    }
}
```

### read_lines Implementation

```go
// Source: os.ReadFile + bufio.Scanner pattern
func stdReadLines(args []values.Value, ctx ExecContext) (values.Value, error) {
    path := args[0].(values.FileValue).Value
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, &EvalError{Message: fmt.Sprintf("read_lines: %v", err)}
    }
    lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
    items := make([]values.Value, len(lines))
    for i, l := range lines {
        items[i] = values.StringValue{Type: types.StringType{}, Value: l}
    }
    return values.ArrayValue{
        Type:  types.ArrayType{ItemType: types.StringType{}},
        Items: items,
    }, nil
}
```

### write_lines Implementation

```go
// Source: os.MkdirTemp + os.WriteFile pattern (confirmed locked decision)
func stdWriteLines(args []values.Value, ctx ExecContext) (values.Value, error) {
    arr := args[0].(values.ArrayValue)
    dir, err := os.MkdirTemp("", "wdl-write-*")
    if err != nil {
        return nil, &EvalError{Message: fmt.Sprintf("write_lines: mkdir: %v", err)}
    }
    path := filepath.Join(dir, "lines.txt")
    var sb strings.Builder
    for _, item := range arr.Items {
        sb.WriteString(item.(values.StringValue).Value)
        sb.WriteByte('\n')
    }
    if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
        return nil, &EvalError{Message: fmt.Sprintf("write_lines: write: %v", err)}
    }
    return values.FileValue{Type: types.FileType{}, Value: path}, nil
}
```

### Correct round() Half-Up

```go
// Source: WDL 1.2 spec "round half up" (NOT banker's rounding)
func stdRound(args []values.Value, ctx ExecContext) (values.Value, error) {
    f := args[0].(values.FloatValue).Value
    // math.Floor(x+0.5) gives half-up for all values, including negatives
    result := int64(math.Floor(f + 0.5))
    return values.IntValue{Type: types.IntType{}, Value: result}, nil
}
```

### chunk() — WDL 1.2 New

```go
// Source: WDL 1.2 spec Array[Array[X]] chunk(Array[X], Int)
func stdChunk(args []values.Value, ctx ExecContext) (values.Value, error) {
    arr := args[0].(values.ArrayValue)
    n := int(args[1].(values.IntValue).Value)
    if n <= 0 {
        return nil, &EvalError{Message: "chunk: size must be > 0"}
    }
    var chunks []values.Value
    for i := 0; i < len(arr.Items); i += n {
        end := i + n
        if end > len(arr.Items) {
            end = len(arr.Items)
        }
        chunk := values.ArrayValue{Type: arr.Type, Items: arr.Items[i:end]}
        chunks = append(chunks, chunk)
    }
    return values.ArrayValue{
        Type:  types.ArrayType{ItemType: arr.Type},
        Items: chunks,
    }, nil
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| WDL 1.1: no `chunk`, `find`, `matches`, `join_paths`, `values`, `contains`, `contains_key` | WDL 1.2 adds these 7 new functions | WDL 1.2.0 (2024) | Must implement all 7 in Phase 4 |
| Dedent after expansion | Sentinel-before-indent | Clarified in WDL 1.2 spec/miniwdl | EXPR-07 specifically targets the correct algorithm |
| `Object` type (WDL 1.0/1.1) | Replaced by `Struct` in WDL 1.2 | WDL 1.2 | `Object` is out of scope per REQUIREMENTS.md |

**Deprecated/outdated:**
- `Object` type: WDL 1.2 deprecated, out of scope for this project.
- `read_object` / `write_object` / `read_objects` / `write_objects`: These are documented but relate to the deprecated `Object` type. Omit from Phase 4 unless explicitly required (not listed in STDLIB-01).

---

## Open Questions

1. **`read_tsv` with header argument**
   - What we know: `read_tsv(File, Boolean, Array[String])` overload exists in WDL 1.2 docs but returns `Array[Object]`
   - What's unclear: `Object` type is out of scope; STDLIB-01 lists `read_tsv` without the header overload
   - Recommendation: Implement only `Array[Array[String]] read_tsv(File)` (the basic overload). Skip the `Array[Object]` overload until Object type is in scope.

2. **`join_paths` exact signature**
   - What we know: Three overloads documented — `join_paths(File, String)`, `join_paths(File, Array[String]+)`, `join_paths(Array[String]+)`
   - What's unclear: Which overloads are required for Phase 4? All three?
   - Recommendation: Implement all three overloads using `filepath.Join`. Runtime dispatch based on argument count and types.

3. **`contains_key` with `Array[String]` nested key path**
   - What we know: `Boolean contains_key(Map[String, Y]|Struct|Object, Array[String])` is documented for nested lookups
   - What's unclear: With `Object` out of scope, the `Struct` case may be needed
   - Recommendation: Implement `contains_key(Map, P)` (single key) fully. For the `Array[String]` path variant, implement only for `MapValue` with nested map values. Defer Struct nested path access.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go standard `testing` package (go test) |
| Config file | none — `go test ./...` convention |
| Quick run command | `go test ./wdl/eval/ -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| EXPR-01 | Arithmetic/comparison/logical operators, `**` | unit | `go test ./wdl/eval/ -run TestEvalBinary -count=1` | ❌ Wave 0 |
| EXPR-02 | if-then-else returns correct branch | unit | `go test ./wdl/eval/ -run TestEvalIfThenElse -count=1` | ❌ Wave 0 |
| EXPR-03 | Struct/pair field access returns element or error | unit | `go test ./wdl/eval/ -run TestEvalGetExpr -count=1` | ❌ Wave 0 |
| EXPR-04 | Array/map index access with bounds check | unit | `go test ./wdl/eval/ -run TestEvalIndexExpr -count=1` | ❌ Wave 0 |
| EXPR-05 | Stdlib function calls via ApplyExpr | unit | `go test ./wdl/eval/ -run TestEvalApply -count=1` | ❌ Wave 0 |
| EXPR-06 | Command template placeholder substitution + dedent | unit | `go test ./wdl/eval/ -run TestEvalCommand -count=1` | ❌ Wave 0 |
| EXPR-07 | Dedent uses sentinel-before-indent order | unit | `go test ./wdl/eval/ -run TestCommandDedent -count=1` | ❌ Wave 0 |
| STDLIB-01 | File I/O functions read/write correctly | unit | `go test ./wdl/eval/ -run TestStdlibFileIO -count=1` | ❌ Wave 0 |
| STDLIB-02 | Filesystem functions (glob, basename, dirname, size, stdout, stderr) | unit | `go test ./wdl/eval/ -run TestStdlibFilesystem -count=1` | ❌ Wave 0 |
| STDLIB-03 | Array functions (length, flatten, chunk, etc.) | unit | `go test ./wdl/eval/ -run TestStdlibArray -count=1` | ❌ Wave 0 |
| STDLIB-04 | String functions (sub, find, matches, join_paths) | unit | `go test ./wdl/eval/ -run TestStdlibString -count=1` | ❌ Wave 0 |
| STDLIB-05 | Type/utility functions (defined, keys, values, as_map, etc.) | unit | `go test ./wdl/eval/ -run TestStdlibUtility -count=1` | ❌ Wave 0 |
| STDLIB-06 | Numeric functions (ceil, floor, round half-up, min, max) | unit | `go test ./wdl/eval/ -run TestStdlibNumeric -count=1` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./wdl/eval/ -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `wdl/eval/eval.go` — core evaluator (new package, does not exist)
- [ ] `wdl/eval/env.go` — runtime Env struct
- [ ] `wdl/eval/error.go` — EvalError type
- [ ] `wdl/eval/context.go` — ExecContext struct
- [ ] `wdl/eval/stdlib.go` — stdlib dispatch + all function implementations
- [ ] `wdl/eval/command.go` — command dedent algorithm
- [ ] `wdl/eval/eval_test.go` — all test functions for EXPR-* and STDLIB-*

---

## Sources

### Primary (HIGH confidence)
- miniwdl source `WDL/Expr.py` (via WebFetch of miniwdl ReadTheDocs module source) — command dedent algorithm: sentinel `"~{}"` substitution before min-indent computation
- `docs.openwdl.org/reference/stdlib/array.html` — array function signatures including `chunk` (WDL 1.2), `contains` (WDL 1.2)
- `docs.openwdl.org/reference/stdlib/string` — `find` (String?, WDL 1.2), `matches` (Boolean, WDL 1.2), `sub` signatures
- `docs.openwdl.org/reference/stdlib/map` — `values` (WDL 1.2), `contains_key` (WDL 1.2) signatures
- `docs.openwdl.org/reference/stdlib/file.html` — `join_paths` overloads (WDL 1.2), file I/O function signatures
- `docs.openwdl.org/reference/stdlib/numeric.html` — `round()` half-up specification confirmed
- Existing codebase: `wdl/values/values.go`, `wdl/ast/expr.go`, `wdl/checker/checker.go` — concrete types and patterns to follow

### Secondary (MEDIUM confidence)
- OpenWDL announcement blog (openwdl.org) — WDL 1.2 new functions confirmed: `chunk`, `find`, `matches`, `join_paths`, `values`, `contains`, `contains_key`
- WebSearch results cross-referencing WDL 1.2 spec and miniwdl docs — dedent algorithm and round() semantics

### Tertiary (LOW confidence)
- None — all critical claims are verified from official sources or existing codebase.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all Go stdlib, no third-party dependencies
- Architecture: HIGH — mirrors existing `checker` package patterns verified in codebase
- Pitfalls: HIGH — dedent algorithm verified from miniwdl source; round() from official WDL docs; NullValue pattern from existing codebase
- Stdlib function signatures: HIGH — verified from docs.openwdl.org official reference

**Research date:** 2026-03-10
**Valid until:** 2026-06-10 (WDL 1.2 spec is stable; stdlib signatures rarely change)
