# Domain Pitfalls

**Domain:** WDL 1.2 workflow engine (Go port of miniwdl)
**Researched:** 2026-03-10

---

## Critical Pitfalls

Mistakes that cause rewrites, spec non-compliance, or silent correctness failures.

---

### Pitfall 1: Command Block Dedent Computed After Expression Interpolation

**What goes wrong:**
The WDL spec requires removing "common leading whitespace" from command blocks. An implementation that naively strips the indentation level of the opening line, or that strips whitespace after interpolating placeholders, will produce incorrect results.

**Why it happens:**
It is tempting to (a) evaluate all `~{}` placeholders first, then strip whitespace from the resulting string, or (b) strip only the indentation of the first non-blank line uniformly. Both are wrong.

The correct algorithm (confirmed from miniwdl `Expr._dedent`):
1. Build a pseudo-string with `~{}` substituted for every placeholder (to avoid placeholders distorting line-start detection).
2. Find the minimum indentation across all non-blank lines in that pseudo-string.
3. Strip exactly that many leading characters from every non-blank line, interleaving placeholders unchanged.
4. Dedent BEFORE interpolation — the placeholder content is not relevant to dedent calculation.

Toil had a long-standing bug where it stripped based on the first line's indentation, breaking scripts whose lines varied in depth (GitHub issue #4854).

**Consequences:**
- Generated shell scripts have wrong indentation, causing subtle bash errors.
- Heredoc-in-heredoc or multi-indent scripts silently misbehave.
- Test cases with uniformly indented commands pass; mixed-indent cases fail.

**Prevention:**
Implement dedent as a separate pre-pass over the `parts []` slice (mix of raw strings and placeholder AST nodes). Use a sentinel to represent placeholders when computing the minimum whitespace prefix. Apply removal only to string parts, never to placeholder nodes. Add unit tests with mixed-indentation commands explicitly.

**Warning signs:**
- `command <<<` tests pass but `command {}` tests behave differently.
- Test commands that use bash `if/else` (which naturally have mixed indentation) fail.

**Phase:** Parser / AST evaluation phase (command block expansion).

---

### Pitfall 2: Scatter Wraps Output Types in Array[T], Conditional Wraps in T?

**What goes wrong:**
A scatter section takes every declaration or call output of type `T` inside it and presents that binding as `Array[T]` to the outside scope. A conditional (`if`) section takes every binding of type `T` and presents it as `T?` (optional).

Nesting these produces compounding: a declaration of type `Int` inside `scatter { if { ... } }` is seen as `Array[Int?]` outside. Critically, nested conditionals do NOT produce `Int??`; WDL 1.2 deliberately collapses multiple optional layers — the type remains singly optional (`T?` not `T??`).

**Why it happens:**
The type-environment promotion logic is easy to implement incorrectly. A common mistake is to apply optional-wrapping multiple times when conditionals are nested, or to forget that the `nonempty` quantifier on an array (i.e. `Array[T]+`) must propagate from the scatter expression's own static type.

From miniwdl `Tree.py` lines 883-895 and 942-961: scatter uses `Type.Array(binding.value, nonempty=nonempty)` where `nonempty` is derived from whether the scatter expression is itself `Array[T]+`. The conditional uses `binding.value.copy(optional=True)`.

**Consequences:**
- Type-checking passes with wrong types; runtime coercion errors appear later.
- `select_first` / `select_all` usage by the user will fail if the engine presents the wrong type.
- Workflows with nested scatter-in-conditional or conditional-in-scatter silently produce wrong types.

**Prevention:**
- Implement type-env propagation as two clearly named functions: `arrayize(t, nonempty)` and `optionalize(t)`.
- `optionalize` must be idempotent — calling it on an already-optional type must not add a second `?`.
- `arrayize` must correctly inherit `nonempty` from the scatter expression's static array type.
- Add tests for all four nesting combinations: scatter-in-scatter, conditional-in-scatter, scatter-in-conditional, conditional-in-conditional.

**Warning signs:**
- `Array[Int?]` showing up as `Array[Int]` or `Int??` in test output.
- Failures only on nested scatter/conditional workflows.

**Phase:** Type system and workflow dependency analysis phase.

---

### Pitfall 3: WDL Type Coercion Rules Are Asymmetric and Context-Dependent

**What goes wrong:**
WDL 1.2 coercion rules are deceptively specific. The correct table (from miniwdl `Type.py` documentation):

1. `Int` coerces to `Float`
2. `Boolean`, `Int`, `Float`, `File` coerce to `String`
3. `String` coerces to `File`, `Int`, `Float`
4. `Array[T]` coerces to `String` only if `T` does as well
5. `T` coerces to `T?` but NOT the reverse (statically)
6. `Array[T]+` coerces to `Array[T]` but NOT the reverse (statically)

Rule 1 applies only at the "top level" — Int/Float coercion does NOT propagate inside compound types. `Array[Int]` does NOT coerce to `Array[Float]`.

The `check_quant=False` mode (used by miniwdl in some expression contexts) relaxes rules 5 and 6 at static check time but those reversed coercions can still fail at runtime.

**Why it happens:**
Developers model coercion as a simple `isAssignable(from, to)` predicate and accidentally allow deep coercion within compound types.

**Consequences:**
- `Array[Int]` silently treated as `Array[Float]`, causing runtime failures.
- Optional stripping or promotion applied incorrectly, causing wrong type errors.
- Spec-compliant WDL rejected; non-compliant WDL accepted.

**Prevention:**
- Implement coercion checks with a `checkQuant bool` parameter that mirrors miniwdl.
- The `check` method should only apply numeric coercion at the immediate level, not recursively into the item type of Array or value type of Map.
- String ↔ File coercion is allowed; String → Directory is NOT.
- Write a matrix of all type-pair coercion tests; use the miniwdl test suite as ground truth.

**Warning signs:**
- `Array[Int]` passes where `Array[Float]` is expected.
- Optional errors only appear at runtime, not at parse/check time.

**Phase:** Type system implementation phase.

---

### Pitfall 4: Command Block Placeholder Syntax Mismatch Between `command {}` and `command <<<>>>`

**What goes wrong:**
WDL 1.2 has two command block delimiters with different placeholder rules:

- `command { ... }`: both `~{}` and `${}` are valid placeholders. The `${}` style is permitted for backwards compatibility but conflicts with bash `${VAR}` syntax, causing parser ambiguity.
- `command <<< ... >>>`: ONLY `~{}` is valid. The `${}` is intentionally NOT a placeholder here, so bash variables can be written as-is without escaping.

A parser that accepts `${}` inside `command <<<>>>` blocks will silently corrupt scripts that use bash variables. Conversely, a parser that rejects `${}` inside `command {}` breaks backwards-compatible WDL.

The grammar (confirmed from miniwdl `_grammar.py` lines 218-222):
```
command1: "{" (COMMAND1_FRAGMENT? _EITHER_DELIM placeholder "}")* ...  // either ~{ or ${
command2: "<<<" (COMMAND2_FRAGMENT? "~{" placeholder "}")* ...         // only ~{
```

**Consequences:**
- Bash variables like `${HOME}` inside `command <<<>>>` are incorrectly interpolated as WDL expressions (parse error or wrong output).
- WDL that mixes bash and WDL variables in `command {}` blocks is silently broken.

**Prevention:**
- Implement two separate lexer states or grammar rules for the two command block styles.
- `command <<<>>>` lexer should treat `${` as an ordinary literal character sequence.
- `command {}` lexer should treat both `~{` and `${` as placeholder starts.
- Test with bash scripts that include `${variable}` literally inside `command <<<>>>`.

**Warning signs:**
- Test command blocks containing bash `${PWD}` or `${1}` fail to parse correctly.
- Placeholder detection tests pass but integration tests with real bash scripts fail.

**Phase:** Lexer/parser implementation phase.

---

### Pitfall 5: Docker File Path Mapping — Host Paths vs Container Paths

**What goes wrong:**
WDL tasks run inside Docker containers. Input files live on the host filesystem. The engine must:
1. Map every input `File`/`Directory` value to a stable in-container path.
2. Pass the container path (not the host path) when expanding the task command.
3. After the task exits, reverse-map output `File` paths from container back to host.
4. Handle the case where a task output path refers to an input file (the engine must recognize it as an already-mapped path, not a new output).

A common mistake is to pass host paths directly into the container command, which either fails (path does not exist inside container) or is a security issue.

From miniwdl `task_container.py`: the engine maintains `input_path_map: Dict[str, str]` (host → container) and `input_path_map_rev: Dict[str, str]` (container → host). Output delocalization checks `container_path in input_path_map_rev` first to handle output-equals-input cases.

**Consequences:**
- Task commands receive host paths, fail inside container.
- Output files that are the same as inputs cause duplicate-copy or "file not found" errors.
- Path collisions: two different input files with the same basename get overwritten.

**Prevention:**
- Maintain bidirectional maps (host↔container) built before container start.
- Assign container paths under a known prefix (e.g. `/mnt/flowy_task/inputs/`); use a counter suffix to deduplicate identical basenames.
- During output collection, first check whether the container path is in the reverse map before treating it as a new host file.
- Input files must be mounted read-only by default (bind-mount with `:ro`); document that tasks needing to rename/move inputs require a copy step.

**Warning signs:**
- Tasks with two input files of the same name fail.
- Output files that are processed versions of inputs (same name, different path) are not found.

**Phase:** Docker task execution phase.

---

### Pitfall 6: Goroutine Leaks in Scatter Execution

**What goes wrong:**
Scatter launches N goroutines in parallel. If one goroutine panics or the scatter is cancelled, the remaining goroutines must be cleaned up. Using a raw `sync.WaitGroup` makes this hard: the WaitGroup has no cancellation channel, so goroutines that are blocked on Docker API calls or channel sends will leak indefinitely.

Additionally, sending on a closed result channel (e.g. when the scatter is cancelled before all goroutines finish) causes a panic.

**Why it happens:**
The natural first pass uses `sync.WaitGroup` + a shared error variable + a results slice. Under cancellation, goroutines that haven't been checked by the scheduler continue running after the caller has moved on.

**Consequences:**
- Docker containers accumulate without cleanup on workflow cancellation or error.
- Memory/goroutine leaks grow with scatter width.
- "send on closed channel" panics in error paths.

**Prevention:**
Use `golang.org/x/sync/errgroup` with `errgroup.WithContext`. Pattern:

```go
g, ctx := errgroup.WithContext(parentCtx)
g.SetLimit(maxParallel)          // bound concurrency
for _, item := range scatterItems {
    item := item                 // capture loop variable
    g.Go(func() error {
        return runTask(ctx, item)
    })
}
if err := g.Wait(); err != nil { ... }
```

When `ctx` is cancelled (first error), all subsequent `runTask` calls see `ctx.Done()` and exit early. Never close a results channel from inside a goroutine; collect results into a pre-allocated slice indexed by scatter position instead.

Run `go test -race` on all scatter tests. Use `goleak` in tests to detect goroutine leaks.

**Warning signs:**
- `go test -race` reports data races on the error variable or results slice.
- Workflow cancellation leaves orphaned Docker containers (`docker ps` shows lingering containers).
- Test suite takes longer than expected when error cases are exercised.

**Phase:** Scatter/workflow execution phase.

---

### Pitfall 7: Modeling WDL's Dynamic Type System in Go's Static Type System

**What goes wrong:**
miniwdl uses Python's dynamic typing: a WDL `Value` is a Python object; its `type` attribute is set at construction and checked at runtime. In Go, this becomes an interface (e.g. `type Value interface { Type() WDLType; ... }`) with concrete structs per WDL type.

The pitfall is modeling WDL's `Any` type (used for empty arrays, None literals) or compound values like `Map[String, Int]` where the Go struct must carry a runtime `WDLType` descriptor. Without that descriptor, two different `Array` values cannot know whether they are `Array[Int]` or `Array[String]` at runtime — the Go type system cannot distinguish them because both are `ArrayValue{items: []Value{}}`.

**Consequences:**
- Empty array `[]` cannot be type-unified with a non-empty array.
- Map/Pair iteration fails when the Go struct has no runtime type tag.
- Type assertions (`v.(*ArrayValue)`) succeed but `v.Type()` returns wrong element type for coercion checks.

**Prevention:**
Every concrete `Value` struct must store its WDL type as a field, not infer it from the Go concrete type:
```go
type ArrayValue struct {
    wdlType ArrayType  // carries item type, nonempty flag
    Items   []Value
}
```
This mirrors miniwdl's `Value.Array` which carries `type: Type.Array(item_type)`. For `None` literals, use a `NullValue` struct carrying `WDLType = Any(optional=true)` so coercion checks can treat it correctly.

Type-switch (`switch v := val.(type)`) should dispatch on Go concrete type for structural access, but always use `.Type()` for WDL-level type decisions.

**Warning signs:**
- Empty array literals `[]` cause panic or wrong-type errors in type unification.
- Map operations on `Map[String, Any]` produce wrong element types.

**Phase:** Type system and value representation phase (foundational — do this first).

---

## Moderate Pitfalls

---

### Pitfall 8: Optional Output Files Must Be Distinguishable from Required Missing Files

**What goes wrong:**
When a task declares an output like `File? out = "result.txt"`, if `result.txt` does not exist in the container work directory, the output value must be `None` (not an error). But if a non-optional `File out = "result.txt"` is declared and the file is missing, the task must fail.

Implementations frequently unify these two cases incorrectly, either failing both or succeeding both.

**Consequences:**
- Optional outputs that are conditionally produced cause spurious task failures.
- Required outputs that are missing are silently treated as null.

**Prevention:**
During output collection, resolve each declared output expression. If the resulting `File` path does not exist on the host:
- If the declared type has `optional=true`, substitute `NullValue`.
- Otherwise, raise a runtime error: "required output file not found."
Extend this to `Array[File?]` where individual elements may be null.

**Warning signs:**
- Tasks with `if (condition) then file_path else None` output patterns fail unconditionally.

**Phase:** Docker task execution / output collection phase.

---

### Pitfall 9: `sep`, `true/false`, `default` Placeholder Options Are Deprecated But Must Still Parse

**What goes wrong:**
WDL 1.2 marks expression placeholder options (`sep=`, `true=`, `false=`, `default=`) as deprecated (to be removed in WDL 2.0). However, they are still valid in WDL 1.2 and must be parsed and evaluated correctly.

Example: `~{sep=", " array_val}` is legal WDL 1.2 even though users should migrate to `~{sep(", ", array_val)}`.

A parser that omits these options will reject a large proportion of real-world WDL 1.2 files.

**Prevention:**
Include placeholder option parsing (`sep`, `true`, `false`, `default`) in the grammar. Emit a lint warning but not an error. Validate that `sep` is only used with `Array` expressions, and `true`/`false` only with `Boolean` expressions (miniwdl enforces this at type-check time in `Expr.Placeholder._infer_type`).

**Phase:** Parser phase.

---

### Pitfall 10: `String.coerces(File)` and `File.coerces(String)` Are Both True — Use Carefully

**What goes wrong:**
WDL 1.2 allows `String` to coerce to `File` (a string literal path becomes a file reference) and `File` to coerce to `String`. This bidirectional coercion means a type-unification algorithm that simply finds a common supertype may incorrectly unify `[some_file, some_string]` as `Array[String]` when `Array[File]` was intended (or vice versa).

miniwdl's `Type.unify` has explicit logic to prefer `File` over `String` when both are candidates, but the exact behavior is subtle.

**Consequences:**
- `read_string(some_file)` stops working because the file path was coerced to a plain string.
- Output file paths become string values that are not tracked for output collection.

**Prevention:**
Implement type unification with explicit File/String precedence: when unifying `[File, String]`, the result must be `File`, not `String`, because losing file-tracking semantics is always wrong. Add a test that unifies a heterogeneous array literal `[some_file, "literal"]` and verifies the resulting type is `Array[File]`.

**Phase:** Type system phase.

---

### Pitfall 11: Map and Pair Key/Value Ordering and Equality Semantics

**What goes wrong:**
WDL `Map` is defined in the spec as having insertion-order semantics (iteration order equals insertion order). Go's built-in `map` is unordered. Using `map[string]Value` directly will produce non-deterministic output from `write_map`, `read_map`, and any stdlib function that iterates map keys.

Additionally, `Map` keys must be comparable primitive types. Attempting to use `Map[Array[String], Int]` or other compound key types must be rejected at type-check time.

**Consequences:**
- `write_map` produces different file content on different runs, breaking reproducibility.
- Downstream tools that parse map outputs get non-deterministic key order.

**Prevention:**
Implement WDL `Map` as an ordered structure (e.g., a slice of key-value pairs backed by a map for O(1) lookup). This is confirmed by the WDL 1.2 spec's requirement for deterministic map ordering. Reject non-primitive map key types during type checking.

**Phase:** Type system / stdlib implementation phase.

---

### Pitfall 12: `read_bool` Case and Whitespace Sensitivity

**What goes wrong:**
The WDL 1.2 spec states `read_boolean` is case-insensitive and whitespace-insensitive (e.g., `" True\n"` must return `true`). miniwdl had a bug (Issue #701, July 2024) where non-lowercase boolean values with whitespace were rejected.

**Prevention:**
Implement `read_boolean` with `strings.TrimSpace` + `strings.ToLower` before comparing to `"true"` / `"false"`. Add test cases: `"true\n"`, `"  False  "`, `"TRUE"`.

**Phase:** Stdlib implementation phase.

---

### Pitfall 13: `round()` Must Use "Half Up" Not "Half to Even" (Banker's Rounding)

**What goes wrong:**
Go's `math.Round` uses "half away from zero" (which is equivalent to "half up" for positive numbers), which matches WDL's spec. However, if an implementation uses Python-style `round()` as reference, Python uses "round half to even" (banker's rounding), which differs for midpoint values: `round(2.5)` is `2` in Python but `3` in WDL.

miniwdl v1.13.0 explicitly changed `round()` to use "half up" to align with the WDL spec.

**Prevention:**
Use `math.Round` from Go's standard library (half-away-from-zero = half-up for positive values). Do not use `math.RoundToEven`. Add test: `round(2.5) == 3`, `round(3.5) == 4`, `round(-2.5) == -3`.

**Phase:** Stdlib implementation phase.

---

### Pitfall 14: Concurrent Access to Shared State in the Workflow Evaluator

**What goes wrong:**
A workflow evaluator maintains a dependency graph and a set of "ready nodes" (nodes whose inputs are all resolved). As scatter goroutines complete and resolve outputs, they concurrently write into the shared output environment. Using a plain `map` for the output environment with no synchronization causes `fatal error: concurrent map writes` — a process-terminating panic, not a recoverable error.

**Consequences:**
- Workflow engine crashes on any workflow with parallel tasks.
- The crash is non-deterministic (appears only when two goroutines write simultaneously).

**Prevention:**
Protect the shared output environment with a `sync.Mutex` or use `sync.Map`. A cleaner design is to have scatter goroutines send completed outputs over a channel, with a single goroutine ("scheduler loop") that reads from the channel and updates the dependency graph. This avoids shared-state entirely. Use `go test -race` on all workflow integration tests from day one.

**Warning signs:**
- Tests pass in isolation but fail under `go test -race`.
- Larger workflow tests are flaky.

**Phase:** Scatter/workflow execution phase.

---

## Minor Pitfalls

---

### Pitfall 15: Exponential Notation in JSON/YAML Input Files

**What goes wrong:**
JSON input files for WDL workflows may contain scientific notation like `1.5e-10`. Standard JSON parsers handle this, but a custom JSON-to-WDL-value converter that only handles plain decimals will fail silently or reject valid input.

**Prevention:**
Use Go's `encoding/json` with `json.Number` type to preserve numeric representations, then parse via `strconv.ParseFloat`. Add test inputs with values like `1e6`, `1.5e-10`, `-3.2E+4`.

**Phase:** Input parsing / CLI phase.

---

### Pitfall 16: Parser Grammar Left Recursion in Expression Rules

**What goes wrong:**
WDL expression grammar includes left-recursive patterns (e.g., member access `expr.field`, indexing `expr[idx]`, function calls `expr(args)`). Participle (Go parser library) does not support left recursion. Goyacc requires explicit precedence declarations and has shift/reduce conflicts by default for common expression patterns.

If the grammar is naively copied from miniwdl's lark grammar (which handles left recursion via PEG), it will not compile in goyacc or will produce incorrect parse trees in participle without restructuring.

**Prevention:**
Use goyacc with explicit `%left` / `%right` precedence declarations for all binary operators. Or use a hand-written Pratt parser for expressions (recommended: simpler to get precedence correct, no conflicts). The WDL expression grammar has 15+ operator precedence levels; encode them explicitly.

**Phase:** Parser implementation phase.

---

### Pitfall 17: WDL `None` Literal Has Type `Any(optional=true)`, Not `T?` for Any Specific T

**What goes wrong:**
The `None` literal in WDL is the only value of type `Any?` (or `Null`). It can be assigned to any optional type. An implementation that assigns it a specific type (e.g., `Int?`) at the point of the literal will fail when it is used in a context requiring `String?` or `File?`.

**Prevention:**
Represent `None` as a `NullValue` carrying `WDLType = AnyOptional`. During type-checking, `AnyOptional.coerces(T?)` returns true for any `T?`. At runtime, coercion of `NullValue` to any optional type returns `NullValue` unchanged.

**Phase:** Type system / expression evaluator phase.

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| Lexer/Parser | `command <<<>>>` must not treat `${}` as placeholder | Separate lexer modes per command block style |
| Parser | Left-recursive expression grammar | Use Pratt parser or explicit goyacc precedence |
| Parser | Placeholder option parsing (`sep=`, `true=`) | Include in grammar; mark deprecated at lint level |
| Type system | `Array[Int]` incorrectly coerces to `Array[Float]` | Coercion is shallow — no recursion into compound types |
| Type system | Empty array `[]` type | Carry runtime WDL type in every `ArrayValue` struct |
| Type system | `File`/`String` unification | Prefer `File` in unify when both are candidates |
| Type system | `None` literal | Use `AnyOptional` type, not a specific `T?` |
| Type system | `Map` ordering | Use ordered map (slice + map), not `map[K]V` |
| Workflow analysis | Scatter/conditional type wrapping | `arrayize` and `optionalize` must be separate, idempotent |
| Expression eval | Command block dedent | Dedent before interpolation; use min-whitespace algorithm |
| Expression eval | `round()` rounding mode | Use half-up (Go `math.Round`), not banker's rounding |
| Expression eval | `read_boolean` sensitivity | Case-insensitive + whitespace-trim |
| Docker execution | Input file path mapping | Maintain bidirectional host↔container map; use `:ro` mounts |
| Docker execution | Optional vs required output files | Check optional flag before raising missing-file error |
| Concurrency | Scatter goroutine leak | Use `errgroup.WithContext` + `SetLimit`; never close result channel from goroutine |
| Concurrency | Shared output environment | Single-writer pattern or `sync.Mutex`; `go test -race` from day one |

---

## Sources

- [WDL 1.2 Specification (openwdl/wdl)](https://github.com/openwdl/wdl/blob/wdl-1.2/SPEC.md)
- [miniwdl releases changelog (chanzuckerberg/miniwdl)](https://github.com/chanzuckerberg/miniwdl/releases)
- [Automatic Coercions Considered Harmful — openwdl issue #373](https://github.com/openwdl/wdl/issues/373)
- [Handle leading whitespace in WDL command blocks more correctly — DataBiosphere/toil issue #4854](https://github.com/DataBiosphere/toil/issues/4854)
- [miniwdl Expr.py — _dedent implementation](https://github.com/chanzuckerberg/miniwdl/blob/main/WDL/Expr.py) (reviewed locally at `../miniwdl/WDL/Expr.py`)
- [miniwdl Tree.py — Scatter.add_to_type_env and Conditional.add_to_type_env](https://github.com/chanzuckerberg/miniwdl/blob/main/WDL/Tree.py) (reviewed locally)
- [miniwdl Type.py — coercion rules documentation](https://github.com/chanzuckerberg/miniwdl/blob/main/WDL/Type.py) (reviewed locally)
- [miniwdl task_container.py — input_path_map design](https://github.com/chanzuckerberg/miniwdl/blob/main/WDL/runtime/task_container.py) (reviewed locally)
- [miniwdl _grammar.py — command1/command2 lexer rules](https://github.com/chanzuckerberg/miniwdl/blob/main/WDL/_grammar.py) (reviewed locally)
- [read_bool whitespace/capitalization issue — miniwdl #701](https://github.com/chanzuckerberg/miniwdl/issues/701)
- [miniwdl runner advanced documentation](https://miniwdl.readthedocs.io/en/latest/runner_advanced.html)
- [errgroup package — golang.org/x/sync](https://pkg.go.dev/golang.org/x/sync/errgroup)
- [Go Data Race Detector](https://go.dev/doc/articles/race_detector)
- [Faster interpreters in Go — PlanetScale](https://planetscale.com/blog/faster-interpreters-in-go-catching-up-with-cpp)
- [Writing yacc parsers with golang — DoltHub](https://www.dolthub.com/blog/2023-07-28-goyacc-parser-tips-tricks/)
