# Stack Research: WDL Workflow Engine (Go)

**Project:** flowy — WDL 1.2 workflow engine (Go port of miniwdl)
**Researched:** 2026-03-10
**Overall confidence:** MEDIUM-HIGH

---

## Recommended Stack

| Layer | Library | Version | Purpose | Confidence |
|-------|---------|---------|---------|------------|
| **Language** | Go | 1.24.x | Core language (latest stable as of 2026-03) | HIGH |
| **Parser** | Hand-written recursive descent + Pratt | — | WDL 1.2 lexer + AST builder | HIGH |
| **Container** | github.com/docker/docker (moby client) | v28.x+incompatible | Docker API: pull images, run containers, stream logs | HIGH |
| **CLI** | github.com/spf13/cobra | v1.10.x | `flowy run`, `flowy check` subcommands | HIGH |
| **Testing** | github.com/stretchr/testify | v1.11.1 | Assertions, require, test suites for parser/engine tests | HIGH |
| **Logging** | log/slog (stdlib) | Go 1.21+ | Structured logging, zero external dependency | HIGH |
| **Concurrency** | stdlib (goroutines + channels) | — | Parallel task/scatter execution, DAG scheduling | HIGH |

---

## Key Decisions

### 1. Parser: Hand-Written Recursive Descent (NOT a parser generator)

**Recommendation:** Write a hand-written recursive descent lexer + parser with Pratt expression parsing for operator precedence.

**Rationale:**

- **WDL 1.2 has no official ANTLR4 grammar for the Go target.** The official `openwdl/wdl` repo only maintains ANTLR4 grammars for WDL 1.0 and Python/Java/JavaScript runtimes. There is no maintained `.g4` file for WDL 1.2 with a Go-ready generator.

- **The only existing Go WDL parser (gowdl) is abandoned.** `github.com/w-gao/gowdl` targets WDL 1.0/1.1 only, has incomplete AST bindings, and is explicitly not under active development. It cannot be used as a base.

- **Participle v2 is LL(k) and does not support left recursion.** WDL expressions (`1 + 2 * 3`, `a.b.c`, `f(x)[i]`) require left-recursive or Pratt-style grammars. Participle requires grammar rewriting that becomes unmaintainable for a language this complex. Its struct-tag DSL also makes it hard to produce clean, typed AST nodes matching miniwdl's `Tree.py` / `Expr.py` model.

- **ANTLR4 Go runtime (antlr4-go/antlr v4.13.1) is viable but adds significant build complexity.** You need the Java ANTLR tool at codegen time, the Go runtime at compile time, and must maintain a `.g4` file that no one else in the community is using for WDL 1.2 + Go. Error messages from generated parsers are harder to customise, which matters for `flowy check`.

- **Go itself uses a hand-written recursive descent parser.** Production evidence (Go, GCC post-switch, V8, Roslyn) consistently shows that hand-written parsers produce better error messages, easier debugging, and tighter control over AST shape — all critical for a `check` command.

- **Pratt parsing** (top-down operator precedence) is the standard technique for expressions in recursive descent parsers. It naturally handles operator precedence and left-associativity without grammar rewrites. It is well-documented and straightforward to implement in Go.

**Implementation sketch:**

```
lexer/   — hand-written lexer: tokens for WDL 1.2 keywords, identifiers,
           string interpolation (~{} and ${}), heredoc, integers/floats
parser/  — recursive descent for declarations, tasks, workflows, scatter, if
           Pratt parser for expressions (apply, index, member access, arithmetic)
ast/     — typed structs matching WDL types: Task, Workflow, Scatter, If,
           Decl, Expr variants (Apply, Get, Index, IfThenElse, etc.)
```

Reference: miniwdl's `WDL/_grammar.py` (lark EBNF) and `WDL/_parser.py` (transformer) are the authoritative source for grammar rules. Translate these directly to Go recursive descent rather than copying the lark EBNF verbatim.

**Confidence:** HIGH — production pattern, no viable alternative for WDL 1.2 + Go.

---

### 2. Docker Integration: docker/docker moby client (NOT subprocess)

**Recommendation:** Use `github.com/docker/docker` client directly.

**Rationale:**

- **Programmatic control over container lifecycle.** The moby client provides `ImagePull`, `ContainerCreate`, `ContainerStart`, `ContainerWait`, `ContainerLogs`, and `ContainerRemove`. This is exactly what task execution needs: pull image if absent, create container with bind mounts for inputs/outputs, stream stdout/stderr to files, wait for exit code.

- **Subprocess (`docker run`) is brittle.** Parsing exit codes and error output from a shell subprocess is fragile. Volume mount flag construction is error-prone string interpolation. Log streaming requires either polling or complex pipe management.

- **The moby client is the de-facto standard.** It is what Docker Compose, Docker Desktop, and Kubernetes CRI-Docker all use. The `github.com/docker/docker` v28.x module (published Nov 2025) is stable and actively maintained.

- **`+incompatible` suffix is expected.** The module has not adopted Go module path versioning for major versions, so `v28.x+incompatible` is normal and not a quality signal. All major Docker tooling uses it this way.

**Key API surface needed for flowy:**

```go
cli, _ := client.NewClientWithOpts(client.FromEnv)
// ImagePull with auth (for private registries later)
// ContainerCreate with HostConfig.Binds for input/output dirs
// ContainerStart
// ContainerWait (blocking, returns StatusCode)
// ContainerLogs (with options for stdout/stderr to files)
// ContainerRemove with RemoveVolumes=true
```

**Confidence:** HIGH — official SDK, actively maintained, correct abstraction level.

---

### 3. Expression Evaluation: Custom Tree-Walking Interpreter

**Recommendation:** Implement a tree-walking interpreter directly over the AST produced by the hand-written parser. Do NOT use a general-purpose expression library (expr-lang/expr, cel-go, tengo).

**Rationale:**

- **WDL expressions are typed and domain-specific.** WDL has `File`, `Directory`, `Map[K,V]`, `Array[T]`, `Pair[L,R]`, `Object`, and `Optional[T]` types. None of the general-purpose Go expression libraries understand these types or can be extended to do so without more work than writing the evaluator directly.

- **expr-lang/expr targets struct-field access and simple predicates**, not a full type system with coercion rules, interpolation contexts, and stdlib function dispatch. Mapping WDL's `if x then y else z` and `Sep(",", array)` onto expr's model is awkward.

- **cel-go is designed for policy/predicate evaluation**, not data transformation pipelines. Its type system is proto-centric and does not map naturally to WDL's type lattice.

- **The tree-walker is small.** WDL expressions are not deeply nested in practice. A `Eval(expr Expr, env Env) (Value, error)` dispatch function with a type-switch over AST nodes is ~500–1000 lines and completely under your control. miniwdl's `Expr.py` is the exact reference.

**Implementation:**

```go
type Evaluator struct { ... }
func (e *Evaluator) Eval(expr ast.Expr, env *Env) (value.Value, error) {
    switch x := expr.(type) {
    case *ast.Apply:   // function call dispatch to stdlib
    case *ast.Get:     // member access / map lookup
    case *ast.IfThenElse:
    case *ast.String:  // interpolation expansion
    // ...
    }
}
```

**Confidence:** HIGH — standard approach for domain-specific languages with custom type systems.

---

### 4. Standard Library Functions: Hand-Implemented in Go

**Recommendation:** Implement all WDL 1.2 stdlib functions in a `stdlib/` package as plain Go functions dispatched by name from the evaluator.

**Rationale:**

- WDL stdlib is ~30 functions (string ops, file ops, array/map manipulation, math). This is a fixed, well-specified surface (`sep`, `read_lines`, `write_lines`, `read_json`, `write_json`, `size`, `basename`, `defined`, `select_first`, etc.).
- The WDL 1.2 spec defines exact behaviour for each function. Implementing them directly in Go ensures correctness against the spec without an intermediate translation layer.
- miniwdl's `_stdlib.py` is the reference implementation — each function is short and independently testable.

**Confidence:** HIGH.

---

### 5. CLI: spf13/cobra v1.10.x

**Recommendation:** Use cobra for the CLI layer.

**Rationale:**

- Cobra is the Go CLI standard: used by kubectl, docker CLI, Hugo, GitHub CLI. Its subcommand model (`flowy run`, `flowy check`) maps directly to what PROJECT.md requires.
- It provides automatic `--help`, flag parsing, completion generation, and error formatting with no configuration.
- No competing library is worth considering at this scale. The standard library's `flag` package lacks subcommand support.

**Version:** v1.10.x (published Dec 2025, latest stable).

**Confidence:** HIGH.

---

### 6. Testing: testify v1.11.1 + Go stdlib testing

**Recommendation:** Use `github.com/stretchr/testify` for assertions alongside the standard `testing` package.

**Rationale:**

- `testify/assert` and `testify/require` produce readable failure messages for deeply nested AST comparisons — critical for parser testing where you need to know exactly which node differs.
- `testify/suite` provides setup/teardown for integration tests that spin up Docker containers or write temp files (needed for task execution tests).
- Table-driven tests (Go stdlib) + `require.Equal` assertions is the idiomatic Go testing pattern for parser/interpreter test suites.
- Published Aug 2025 (v1.11.1), actively maintained, 11k+ GitHub stars.

**Note on fuzz testing:** Go's built-in `testing/fuzz` (available since Go 1.18) should be used for the parser (fuzz with arbitrary WDL-like input to catch panics). No external library needed.

**Confidence:** HIGH.

---

### 7. Logging: log/slog (stdlib)

**Recommendation:** Use the standard library `log/slog` package introduced in Go 1.21.

**Rationale:**

- Zero external dependency. The project has enough complexity without adding a logging dependency.
- Structured key-value logging works well for workflow engine diagnostics (task name, container ID, exit code as fields).
- Performance (650ns/op) is adequate; this is not a hot-logging path.
- If higher performance is needed later, zap provides a compatible handler interface.

**Confidence:** HIGH.

---

### 8. Concurrency: stdlib goroutines + channels (no external DAG library)

**Recommendation:** Implement workflow DAG scheduling with goroutines and channels directly. No external DAG/workflow orchestration library.

**Rationale:**

- WDL's concurrency model is well-defined: tasks in a workflow run when their declared inputs are available (data-dependency driven). Scatter produces a slice of parallel tasks. This maps cleanly to a fan-out/fan-in pattern with goroutines.
- External DAG libraries add significant complexity and opacity to what is fundamentally a `sync.WaitGroup` + channel fan-out problem.
- The implementation pattern: build a dependency graph at load time, use a goroutine-per-task model, block on input channels, collect outputs. `context.Context` for cancellation propagation.

**Confidence:** HIGH — standard Go concurrency primitives are sufficient for v1 scope.

---

## What NOT to Use

| Candidate | Reason to Avoid |
|-----------|----------------|
| **goyacc (LALR)** | LALR(1) cannot parse WDL's expression grammar without grammar hacks (member access `a.b.c`, function call `f(x)`, and array index `a[i]` create ambiguity). Error messages are opaque. Grammar maintenance is painful. |
| **participle/v2** | LL(k) — no left recursion. WDL expressions require left-recursive rules. Struct-tag DSL makes it hard to produce the typed AST nodes matching miniwdl's model. Would require major grammar rewriting. |
| **antlr4-go/antlr v4.13.1** | Requires Java ANTLR tool at build time. No community-maintained WDL 1.2 `.g4` grammar for Go. Generated parser code is 3-5x the size of hand-written equivalent. Error recovery is harder to customise. |
| **gowdl (w-gao/gowdl)** | Explicitly abandoned. Supports WDL 1.0/1.1 only. Incomplete AST. Cannot be built upon. |
| **expr-lang/expr** | Designed for struct-field predicate evaluation. Cannot represent WDL's File/Directory/Map/Pair types. No type coercion rules. |
| **cel-go** | Proto-centric type system. Heavy dependency. Designed for CEL (policy language), not WDL data transformations. |
| **docker subprocess (os/exec)** | Fragile string construction for volume mounts and env vars. No structured access to container exit codes and log streams. Hard to test. |
| **logrus / zap** | External logging dependency unnecessary when `log/slog` covers the use case. Add later only if benchmarking shows a bottleneck. |

---

## Installation

```bash
# Core runtime dependency
go get github.com/docker/docker@latest          # v28.x+incompatible

# CLI
go get github.com/spf13/cobra@latest            # v1.10.x

# Test dependencies
go get github.com/stretchr/testify@v1.11.1

# No other external dependencies needed for v1
```

---

## Sources

- [participle v2 — GitHub](https://github.com/alecthomas/participle) — latest release v2.1.4, Mar 2024
- [participle v2 — pkg.go.dev](https://pkg.go.dev/github.com/alecthomas/participle/v2) — LL(k) limitation documented
- [antlr4-go/antlr v4 — pkg.go.dev](https://pkg.go.dev/github.com/antlr4-go/antlr/v4) — v4.13.1, May 2024
- [docker/docker — pkg.go.dev](https://pkg.go.dev/github.com/docker/docker) — v28.5.2+incompatible, Nov 2025
- [spf13/cobra — pkg.go.dev](https://pkg.go.dev/github.com/spf13/cobra) — v1.10.x, Dec 2025
- [stretchr/testify — pkg.go.dev](https://pkg.go.dev/github.com/stretchr/testify) — v1.11.1, Aug 2025
- [log/slog — pkg.go.dev](https://pkg.go.dev/log/slog) — stdlib since Go 1.21
- [gowdl — GitHub](https://github.com/w-gao/gowdl) — abandoned, WDL 1.0/1.1 only
- [openwdl/wdl — GitHub](https://github.com/openwdl/wdl) — official WDL spec; ANTLR4 grammar only for 1.0, not 1.2
- [DoltHub blog: goyacc tips and tricks](https://www.dolthub.com/blog/2023-07-28-goyacc-parser-tips-tricks/) — LALR(1) limitations documented
- [Parser generators vs hand-written — HN thread 2021](https://news.ycombinator.com/item?id=28258945) — production evidence for hand-written parsers
- [Go 1.25 release](https://go.dev/blog/go1.25) — Go 1.25 released Aug 2025; Go 1.24 is current LTS-equivalent
- [miniwdl _parser.py — GitHub](https://github.com/chanzuckerberg/miniwdl/blob/main/WDL/_parser.py) — reference implementation grammar/transformer structure
