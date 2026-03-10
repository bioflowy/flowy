# Project Research Summary

**Project:** flowy — WDL 1.2 workflow engine in Go (port of miniwdl)
**Domain:** Domain-specific language interpreter + container orchestrator (bioinformatics)
**Researched:** 2026-03-10
**Confidence:** HIGH

## Executive Summary

flowy is a classic interpreter pipeline that transforms WDL 1.2 source files into Docker container executions. The architecture follows a well-trodden path — lexer → parser → AST → type checker → expression evaluator → workflow scheduler → task runner — where each stage communicates only through well-typed structs. The right approach is to hand-write every layer rather than rely on parser generators or general-purpose expression libraries: no maintained WDL 1.2 grammar exists for Go parser generators, no existing Go WDL parser is usable, and the type system is too domain-specific for cel-go or expr-lang. The reference implementation is miniwdl (Python), whose source files (`Expr.py`, `Type.py`, `Tree.py`, `_grammar.py`, `runtime/workflow.py`) are the authoritative ground truth for every non-obvious decision.

The recommended stack is deliberately minimal: Go 1.24 standard library covers concurrency, logging, and testing; the only required external dependencies are the Docker moby client (v28.x), cobra (CLI), and testify (test assertions). The WDL 1.2 spec and miniwdl source are both HIGH-confidence, spec-grounded references, so feature scope is clear and non-speculative.

The two highest risks are correctness traps in the type system and concurrency safety in the workflow scheduler. WDL's type coercion rules are asymmetric and shallow, scatter/conditional blocks apply different type promotions that must be idempotent, and Go's static type system requires every runtime `Value` to carry an explicit WDL type tag. On the concurrency side, scatter goroutines must flow through `errgroup.WithContext` with a bounded limit; any shared output state must be written from a single scheduler goroutine, not from scatter worker goroutines. Both risks have known prevention strategies documented in the pitfalls research.

---

## Key Findings

### Recommended Stack

The stack is intentionally lean. A hand-written stateful lexer (Rob Pike's state-function pattern) handles WDL's two command block styles and string interpolation nesting correctly — no generated lexer can express this. A hand-written recursive descent parser with a Pratt expression sub-parser handles WDL 1.2's full grammar, producing strongly typed AST nodes that mirror miniwdl's `Tree.py` / `Expr.py` model. A tree-walking evaluator over that AST implements all ~30 WDL 1.2 stdlib functions directly in Go. The Docker moby client (not subprocess) provides structured container lifecycle control. `golang.org/x/sync/errgroup` with `SetLimit` is the correct concurrency primitive for bounded scatter parallelism.

**Core technologies:**
- **Go 1.24** — core language; goroutines + channels cover all concurrency needs natively
- **Hand-written lexer/parser + Pratt expressions** — no viable alternative for WDL 1.2 + Go; better error messages than any generator
- **github.com/docker/docker v28.x** — moby client; structured container lifecycle API; the industry standard
- **github.com/spf13/cobra v1.10.x** — subcommand CLI (`flowy run`, `flowy check`); universal Go CLI library
- **github.com/stretchr/testify v1.11.1** — assertion library; required for readable AST comparison failures
- **log/slog (stdlib)** — structured logging; zero external dependency
- **golang.org/x/sync/errgroup** — bounded scatter goroutines with context cancellation propagation

See `.planning/research/STACK.md` for full rationale and what NOT to use.

### Expected Features

WDL 1.2 compliance requires implementing the full language in dependency order: types → expressions → tasks → workflows → CLI. The feature set is well-specified by the OpenWDL spec and docs.openwdl.org; there is no ambiguity about what is table stakes vs. optional.

**Must have (table stakes):**
- Complete WDL 1.2 type system: Boolean, Int, Float, String, File, Directory, Array[T], Map[K,V], Pair[X,Y], Struct, Object (deprecated but required), Optional (T?), NonEmpty (Array[T]+), plus all 6 coercion rules
- Full expression evaluation: all operators including `**` (new in 1.2), both interpolation styles (`~{}` and `${}`), if-then-else, member access, index access
- Task definition: all sections (input, command, output, runtime/requirements, hints, meta, parameter_meta); both command block delimiters
- Workflow definition: call, scatter, if, nested scatter/if, sub-workflow calls, local imports, `after` dependency
- All ~30 WDL 1.2 stdlib functions (file I/O, array, map, string, numeric)
- Docker container execution: image pull, input file staging, command execution, stdout/stderr capture, output collection
- Workflow DAG scheduler: parallel task execution, scatter fan-out/gather, conditional optional wrapping
- CLI: `flowy run` and `flowy check`, JSON input files, `--task`, `--dir`, `--path`, exit codes

**Should have (competitive differentiators):**
- Native Go binary — zero-dependency install, no JVM or Python runtime required
- Fast cold-start for CI/CD — sub-second vs. 10–30s for Cromwell/JVM
- Informative static error messages from `flowy check` — precise line/column and type mismatch descriptions
- Parallel scatter with configurable concurrency limit (default: NumCPU)
- Structured JSON output by default (outputs.json, error.json)
- Strict WDL 1.2-only focus — clean parser, no legacy version baggage

**Defer (v2+):**
- HTTP/HTTPS import resolution — adds auth, caching, network error complexity
- Call output caching — requires stable hashing; build correctness first
- Non-Docker backends (Podman, Singularity) — single well-tested backend over 5 partial ones
- WDL draft-2/1.0/1.1 support — separate grammar, high maintenance cost
- REST API server / web UI — v2 milestone
- Cloud backend execution (AWS Batch, GCP) — explicitly out of scope for v1
- Workflow resumption / partial re-run — requires persistent state

See `.planning/research/FEATURES.md` for the complete spec compliance checklist.

### Architecture Approach

The engine is a classic interpreter pipeline with clean stage boundaries and no circular imports. The key structural rule is that nothing in the parser layer imports the runtime layer — the parser is pure syntax. The type checker operates as a separate second pass over the complete AST (required for forward reference resolution). The Docker SDK is isolated behind a `ContainerRunner` interface to enable unit testing without a live daemon. Workflow scheduling uses a single-goroutine state machine that dispatches worker goroutines and receives results over a channel — shared state never exists across goroutines.

**Major components (build order):**
1. **`internal/parser/ast/`** — typed AST node structs with embedded SourcePos; leaf package, no internal imports
2. **`internal/parser/`** — stateful lexer + recursive descent parser + Pratt expression parser
3. **`internal/typesystem/`** — WDL type hierarchy (`Type` interface + concrete types), value hierarchy (`Value` interface + concrete values with embedded WDL type tags), type checker pass
4. **`internal/runtime/env.go`** — immutable linked-list `*Env` (scatter-safe; no copying needed)
5. **`internal/expr/`** — tree-walking expression evaluator + all stdlib function implementations
6. **`internal/runtime/task.go`** — task input binding, command template expansion (pure WDL logic, no Docker imports)
7. **`internal/runtime/runner.go`** — Docker SDK interaction only; implements `ContainerRunner` interface
8. **`internal/runtime/workflow.go`** — DAG scheduler state machine; dispatches via errgroup; handles scatter/gather and conditional wrapping
9. **`internal/cli/`** — cobra command handlers wiring all above
10. **`cmd/flowy/main.go`** — thin entry point

See `.planning/research/ARCHITECTURE.md` for complete data flow diagrams and interface signatures.

### Critical Pitfalls

Seven pitfalls are rated CRITICAL (cause rewrites or silent correctness failures):

1. **Command block dedent computed incorrectly** — dedent must be calculated BEFORE interpolation using minimum-whitespace algorithm over pseudo-string with placeholder sentinels; applying it after expansion or using first-line indentation is wrong. Prevention: implement as a pre-pass over the `parts []` slice.

2. **Scatter/conditional type promotion applied incorrectly** — scatter wraps output types in `Array[T]`; conditional wraps in `T?`; nesting compounds; multiple optionals must NOT stack (`T??` is illegal — stay singly optional). Prevention: implement as two separate idempotent functions `arrayize(t, nonempty)` and `optionalize(t)`.

3. **WDL type coercion is shallow and asymmetric** — `Int` coerces to `Float` at the top level only; `Array[Int]` does NOT coerce to `Array[Float]`; `String`↔`File` is bidirectional but File must be preferred during type unification. Prevention: implement coercion as non-recursive; write a full coercion matrix test.

4. **Command block placeholder syntax differs between `{}` and `<<<>>>`** — `command {}` accepts both `~{}` and `${}`; `command <<<>>>` accepts ONLY `~{}`; `${}` inside `<<<>>>` must be treated as a literal bash variable. Prevention: two separate lexer states per command block style.

5. **Docker file path mapping requires bidirectional host↔container map** — host paths must never be passed into containers directly; output path resolution must check the reverse map first; duplicate basenames need counter suffixes. Prevention: maintain `inputPathMap` and `inputPathMapRev` before container start.

6. **Scatter goroutine leaks under cancellation** — `sync.WaitGroup` has no cancellation; goroutines blocked on Docker API calls will leak. Prevention: always use `errgroup.WithContext` + `SetLimit`; never close result channel from a goroutine; collect results into pre-allocated slice by scatter index.

7. **WDL runtime values need explicit WDL type tags in Go structs** — empty `ArrayValue{items: []Value{}}` cannot distinguish `Array[Int]` from `Array[String]` without an embedded `wdlType ArrayType` field. Prevention: every concrete `Value` struct must store its WDL type as a field; `NullValue` carries `AnyOptional` type.

See `.planning/research/PITFALLS.md` for 10 additional moderate/minor pitfalls with exact prevention strategies.

---

## Implications for Roadmap

The feature dependency chain from FEATURES.md maps directly to a 6-phase build sequence. Each phase is a hard prerequisite for the next; there is no way to reorder. Phases 4, 5, and 6 have internal parallelism opportunities (task runner and workflow scheduler share the same foundation but can be developed concurrently).

### Phase 1: Lexer, Parser, and AST

**Rationale:** Everything depends on the parser. It is the leaf layer with no internal imports. This is also where the hardest correctness traps live (command block delimiter modes, string interpolation nesting, Pratt precedence) — get these right first while the project is small and tests are fast.
**Delivers:** `ast.Document` from any valid WDL 1.2 source file; `flowy check` parse errors with line/column
**Addresses:** All parser features from FEATURES.md Phase 1 checklist
**Avoids:** Pitfall 4 (command block placeholder modes), Pitfall 16 (left recursion in expressions), Pitfall 9 (deprecated placeholder options must parse)
**Research flag:** None — hand-written recursive descent with Pratt is a well-documented pattern; miniwdl `_grammar.py` is the reference grammar

### Phase 2: Type System and Type Checker

**Rationale:** Forward reference resolution in WDL requires the full AST before type checking begins. The type system is foundational to the evaluator and runtime — getting types wrong here causes cascading silent failures. All critical type-system pitfalls (coercion, value tagging, None literal, Map ordering) must be resolved before any code runs.
**Delivers:** Validated AST with resolved types; `flowy check` type errors with precise messages; `flowy check` fully functional
**Addresses:** FEATURES.md Phase 2 checklist; all type system table stakes
**Avoids:** Pitfall 3 (shallow coercion), Pitfall 7 (value type tags), Pitfall 10 (File/String unification), Pitfall 11 (Map ordering), Pitfall 17 (None literal type), Pitfall 2 (scatter/conditional type promotion)
**Research flag:** None — type rules are fully specified in the WDL 1.2 spec and miniwdl `Type.py`

### Phase 3: Expression Evaluator and Standard Library

**Rationale:** Expression evaluation is required by both task execution (command expansion, output expressions) and workflow execution (declarations, scatter array expressions). Implementing all 30 stdlib functions here in isolation, before the runtime exists, allows thorough unit testing against known inputs/outputs with no Docker dependency.
**Delivers:** Working expression evaluator; all stdlib functions; `Eval(expr, env) → Value` interface
**Addresses:** FEATURES.md Phase 3 checklist; all stdlib function tables
**Avoids:** Pitfall 1 (command block dedent algorithm), Pitfall 12 (read_boolean case sensitivity), Pitfall 13 (round() half-up rounding)
**Research flag:** None — miniwdl `Expr.py` and `StdLib.py` are line-for-line references

### Phase 4: Task Execution

**Rationale:** Single-task execution (the innermost unit of the engine) can be built and tested before the workflow scheduler exists. This phase is the only place where Docker integration lives, and isolating it behind `ContainerRunner` interface makes testing tractable. Task execution is a prerequisite for workflow execution.
**Delivers:** End-to-end single task runs via Docker; input file staging; stdout/stderr capture; output collection
**Addresses:** FEATURES.md Phase 4 checklist; Docker container execution table stakes
**Avoids:** Pitfall 5 (bidirectional path mapping), Pitfall 8 (optional vs required output files), Anti-Pattern 3 (mixing Docker calls into task logic)
**Research flag:** Low concern — Docker moby client API is well-documented; bidirectional path map pattern is established from miniwdl

### Phase 5: Workflow Execution (DAG Scheduler)

**Rationale:** This is the most architecturally complex component — the single-goroutine state machine that dispatches parallel task goroutines, handles scatter fan-out/gather, conditional optional wrapping, and sub-workflow calls. Build after task execution is solid.
**Delivers:** Full multi-task workflow execution; scatter parallelism; conditional blocks; sub-workflows; outputs.json
**Addresses:** FEATURES.md Phase 5 checklist; parallel task execution, scatter/gather, conditional semantics
**Avoids:** Pitfall 2 (scatter/conditional type wrapping), Pitfall 6 (goroutine leaks), Pitfall 14 (concurrent shared state), Anti-Pattern 4 (unbounded goroutine spawning)
**Research flag:** RECOMMEND research-phase — the DAG scheduler with scatter/gather semantics and nested scatter-in-conditional combinations is the most complex novel logic. Worth reviewing miniwdl's StateMachine and errgroup patterns in detail before implementation planning.

### Phase 6: CLI and Integration

**Rationale:** The CLI layer is a thin cobra wrapper over the engine. It is last because it requires all other components. This phase also covers integration testing with real WDL files, structured output formatting, and error presentation polish.
**Delivers:** `flowy run` and `flowy check` as usable CLI tools; JSON input parsing; colored terminal output; exit codes; `--task`, `--dir`, `--path` flags
**Addresses:** FEATURES.md Phase 6 checklist; all CLI table stakes; differentiator features (single binary, fast startup, structured output)
**Avoids:** Pitfall 15 (JSON scientific notation in inputs)
**Research flag:** None — cobra patterns are well-established; input parsing uses standard `encoding/json`

### Phase Ordering Rationale

- **Strict dependency chain:** Parser → Types → Evaluator → Task → Workflow → CLI is non-negotiable. Each layer's outputs are the inputs of the next. Attempting to build task execution without a validated type system leads to discovered type bugs during Docker testing.
- **Types before evaluator:** WDL value types must be tagged in structs (Pitfall 7) before any evaluation code is written, or the evaluator will be built on the wrong abstraction.
- **Task before workflow:** End-to-end single-task Docker tests provide a fast feedback loop. Workflow scheduler bugs are harder to diagnose when task execution is also uncertain.
- **Scatter/gather is the hardest problem:** Isolating it in Phase 5 (not merged with task execution) keeps Phase 4 tractable and Phase 5 reviewable.
- **`flowy check` emerges naturally at Phase 2:** The type checker completes the static analysis pipeline. This means a useful tool exists after Phase 2, not only after Phase 6.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 5 (Workflow DAG Scheduler):** The scatter-in-conditional and conditional-in-scatter nesting combinations produce complex type promotion rules; the errgroup-based state machine pattern needs careful planning to avoid the goroutine leak and shared-state pitfalls. Review miniwdl `runtime/workflow.py` StateMachine implementation line-by-line before implementation.

Phases with standard patterns (skip research-phase):
- **Phase 1 (Parser):** Hand-written recursive descent + Pratt is a standard, well-documented technique. miniwdl `_grammar.py` is the reference grammar. No ambiguity.
- **Phase 2 (Type System):** WDL spec + miniwdl `Type.py` are complete references. Rules are explicit.
- **Phase 3 (Evaluator + Stdlib):** miniwdl `Expr.py` + `StdLib.py` are line-for-line references for ~500–1000 lines of Go. Straightforward translation.
- **Phase 4 (Task Execution):** Docker moby client API is well-documented. Path mapping pattern comes directly from miniwdl `task_container.py`. Established.
- **Phase 6 (CLI):** Cobra patterns are universal Go practice.

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All library choices verified against pkg.go.dev; rejections have clear documented rationale; no viable alternatives exist for WDL 1.2 + Go |
| Features | HIGH | Grounded directly in WDL 1.2 spec (SPEC.md), docs.openwdl.org, and miniwdl source code — all authoritative primary sources |
| Architecture | HIGH | Component boundaries and patterns sourced from miniwdl source + golang-standards/project-layout + Go stdlib `go/ast` design; all architectural decisions have HIGH confidence rating in the research |
| Pitfalls | HIGH | 7 of 17 pitfalls sourced from actual bugs in Toil, miniwdl, and the WDL spec changelog with specific issue references; others derived from direct source code review |

**Overall confidence:** HIGH

### Gaps to Address

- **Docker `ContainerRunner` interface design:** The exact interface signature (ContainerSpec fields for env vars, resource limits, timeout) should be finalized before Phase 4 begins to avoid breaking the task/runner split.
- **Sub-workflow call semantics in Phase 5:** The research notes sub-workflow calls as HIGH complexity. The exact mechanism for passing input Env across the workflow/sub-workflow boundary warrants a targeted review of miniwdl `Tree.py` `Call._add_to_type_env` before Phase 5 planning.
- **`errgroup` vs. channel-based scheduler:** The architecture recommends a channel-based scheduler (single writer), but also references errgroup. The exact pattern for combining both without redundancy should be resolved during Phase 5 planning.
- **Test strategy for Docker integration:** Phase 4 tests require a live Docker daemon. The CI/CD setup for this (Docker-in-Docker vs. socket mounting) is not addressed in the research and should be decided before Phase 4 begins.

---

## Sources

### Primary (HIGH confidence)
- [WDL 1.2 SPEC.md](https://github.com/openwdl/wdl/blob/wdl-1.2/SPEC.md) — authoritative language specification
- [docs.openwdl.org](https://docs.openwdl.org) — stdlib function signatures, task/workflow reference
- miniwdl source `../miniwdl/WDL/` — ground truth for grammar, type rules, expression AST, stdlib, container path mapping, workflow state machine (reviewed locally)
- [docker/docker v28.x — pkg.go.dev](https://pkg.go.dev/github.com/docker/docker) — Docker moby client API
- [spf13/cobra v1.10.x — pkg.go.dev](https://pkg.go.dev/github.com/spf13/cobra) — CLI library
- [golang.org/x/sync/errgroup — pkg.go.dev](https://pkg.go.dev/golang.org/x/sync/errgroup) — bounded goroutine group

### Secondary (MEDIUM confidence)
- [golang-standards/project-layout](https://github.com/golang-standards/project-layout) — package organization conventions
- [go/ast package](https://pkg.go.dev/go/ast) — Node interface design reference
- [stretchr/testify v1.11.1 — pkg.go.dev](https://pkg.go.dev/github.com/stretchr/testify) — test assertion library
- [Announcing WDL 1.2.0 — OpenWDL](https://openwdl.org/wdl/bioinformatics/workflows/announcing-wdl-1-2-0/) — WDL 1.2 new features summary

### Tertiary (supporting/cross-reference)
- [miniwdl Medium article](https://medium.com/czi-technology/miniwdl-17ecdaf40944) — architecture overview context
- [DataBiosphere/toil issue #4854](https://github.com/DataBiosphere/toil/issues/4854) — command block dedent bug (real-world pitfall evidence)
- [miniwdl issue #701](https://github.com/chanzuckerberg/miniwdl/issues/701) — read_boolean whitespace bug (real-world pitfall evidence)
- [DoltHub blog: goyacc tips and tricks](https://www.dolthub.com/blog/2023-07-28-goyacc-parser-tips-tricks/) — parser generator limitations documented

---
*Research completed: 2026-03-10*
*Ready for roadmap: yes*
