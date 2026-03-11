---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
status: planning
stopped_at: Phase 8 context gathered
last_updated: "2026-03-11T01:59:00.161Z"
last_activity: 2026-03-10 — Roadmap created; all 57 v1 requirements mapped to 6 phases
progress:
  total_phases: 8
  completed_phases: 7
  total_plans: 24
  completed_plans: 24
  percent: 0
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-10)

**Core value:** WDL 1.2で記述されたワークフローをDockerコンテナ上で正確に実行できること
**Current focus:** Phase 1 — Type Data Structures

## Current Position

Phase: 1 of 6 (Lexer, Parser, and AST)
Plan: 0 of TBD in current phase
Status: Ready to plan
Last activity: 2026-03-10 — Roadmap created; all 57 v1 requirements mapped to 6 phases

Progress: [░░░░░░░░░░] 0%

## Performance Metrics

**Velocity:**
- Total plans completed: 0
- Average duration: -
- Total execution time: 0 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| - | - | - | - |

**Recent Trend:**
- Last 5 plans: -
- Trend: -

*Updated after each plan completion*
| Phase 01-type-data-structures P01 | 15min | 2 tasks | 5 files |
| Phase 02-lexer-parser-and-ast P01 | 3min | 2 tasks | 7 files |
| Phase 02-lexer-parser-and-ast P02 | 3min | 2 tasks | 3 files |
| Phase 02-lexer-parser-and-ast P03 | 8min | 2 tasks | 4 files |
| Phase 02-lexer-parser-and-ast P04 | 9min | 2 tasks | 4 files |
| Phase 02-lexer-parser-and-ast P05 | 4min | 2 tasks | 6 files |
| Phase 03-type-checker P01 | 2 | 2 tasks | 8 files |
| Phase 03-type-checker P02 | 3 | 1 tasks | 3 files |
| Phase 03-type-checker P03 | 8min | 1 tasks | 2 files |
| Phase 03-type-checker P04 | 4min | 1 tasks | 3 files |
| Phase 04-expression-evaluator-and-standard-library P01 | 2 | 2 tasks | 8 files |
| Phase 04-expression-evaluator-and-standard-library P02 | 8min | 2 tasks | 2 files |
| Phase 04-expression-evaluator-and-standard-library P03 | 2min | 2 tasks | 4 files |
| Phase 04-expression-evaluator-and-standard-library P04 | 5min | 2 tasks | 2 files |
| Phase 04-expression-evaluator-and-standard-library P05 | 4min | 2 tasks | 4 files |
| Phase 04-expression-evaluator-and-standard-library P06 | 3min | 2 tasks | 3 files |
| Phase 05-task-execution P01 | 4min | 2 tasks | 10 files |
| Phase 05-task-execution P02 | 4min | 2 tasks | 6 files |
| Phase 05-task-execution P03 | 10min | 2 tasks | 4 files |
| Phase 06-workflow-execution-dag-scheduler P01 | 2min | 2 tasks | 5 files |
| Phase 06-workflow-execution-dag-scheduler P02 | 9min | 2 tasks | 7 files |
| Phase 06-workflow-execution-dag-scheduler P03 | 3min | 2 tasks | 4 files |
| Phase 07-cli-and-integration P01 | 3min | 2 tasks | 9 files |
| Phase 07-cli-and-integration P02 | 4min | 2 tasks | 6 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- [Init]: Hand-written lexer/parser (Rob Pike state-function pattern + Pratt) — no viable WDL 1.2 grammar for Go parser generators
- [Init]: Docker moby client v28.x behind ContainerRunner interface — enables unit testing without live daemon
- [Init]: golang.org/x/sync/errgroup with SetLimit for scatter parallelism — prevents goroutine leaks
- [Phase 01-type-data-structures]: AnyType{Optional:true} serves as None literal type — TypeName()=='None', IsOptional()==true — no separate NoneType struct needed
- [Phase 01-type-data-structures]: Value structs embed concrete type fields (not interface) so empty ArrayValue distinguishes Array[Int] from Array[String]
- [Phase 01-type-data-structures]: StructType field named 'Name' not 'TypeName' to prevent Go interface method collision
- [Phase 02-lexer-parser-and-ast]: TokenType = int alias in wdl/ast/expr.go avoids circular import between ast and lexer packages
- [Phase 02-lexer-parser-and-ast]: Pos embedded as plain struct field (not pointer) in every AST node for zero-allocation direct field access
- [Phase 02-lexer-parser-and-ast]: WorkflowNode, StringPart, Expr use unexported marker methods to prevent unintended external implementations
- [Phase 02-lexer-parser-and-ast]: Pull-model Next() lexer API with auto command-mode detection after 'command' keyword — parser does not need SetCommandMode call
- [Phase 02-lexer-parser-and-ast]: TokXxx exported naming for all token constants enables external lexer_test package to reference them directly
- [Phase 02-lexer-parser-and-ast]: parseExpr stub propagates TokError as ParseError; expectName() accepts type-keywords as struct/task/workflow names
- [Phase 02-lexer-parser-and-ast]: Workflow input/output sections parsed directly in parseWorkflow loop; body-only elements use parseWorkflowBodyElement
- [Phase 02-lexer-parser-and-ast]: ** right-associativity: rightBP=59 < leftBP=60 with loop condition leftBP<=minBP — unambiguously right-associative for 2**3**2→2**(3**2)
- [Phase 02-lexer-parser-and-ast]: Unary minus/not rightBP=55 (between */ at 50 and ** at 60): -x**2→-(x**2) matching Python/math convention
- [Phase 02-lexer-parser-and-ast]: isNameToken expanded with TokEnv — 'env' keyword valid as variable reference in expression context (needed for ${env} in command blocks)
- [Phase 02-lexer-parser-and-ast]: parseFile takes optional src []byte: non-nil for root, nil for imported files (read from disk via os.ReadFile)
- [Phase 02-lexer-parser-and-ast]: Circular import chain reconstructed via chain []string alongside visited map — O(1) detection plus human-readable error messages
- [Phase 02-lexer-parser-and-ast]: Workflow input declarations use parseDecl(true) to allow optional default values per WDL 1.2 spec
- [Phase 03-type-checker]: checker_test uses package checker (not checker_test) to test unexported optionalize() — plan specifies checker_test but internal access required
- [Phase 03-type-checker]: Coerces() implements 6 WDL coercion rules shallow (no recursive item coercion for Array items) — matches spec intent
- [Phase 03-type-checker]: optionalize() has idempotency guard (T? stays T?, not T??) — prevents double-wrapping in scatter/if promotion
- [Phase 03-type-checker]: Rule 1 T->T? applied recursively in Coerces(): strip optional from , recurse — rules 2-6 never see mismatched optional flags
- [Phase 03-type-checker]: Rule 5 Array[T]+->Array[T] compares item types by TypeName() only (shallow), not via recursive Coerces()
- [Phase 03-type-checker]: optionalize() uses IsOptional() guard before WithOptional(true) to ensure idempotency (T? not double-wrapped)
- [Phase 03-type-checker]: declGroup struct pairs ast.Decl with validation Scope for scatter body inner scope resolution
- [Phase 03-type-checker]: Toposort edges limited to outerScope.decls peers only — prevents cross-scope false cycle detection
- [Phase 03-type-checker]: checkExprType skips complex expressions (BinaryExpr, ApplyExpr) — deferred to Phase 4
- [Phase 03-type-checker]: checkGetExpr falls back to structDefs when StructType.Members is nil — Pass 1 registers name-only StructType; fallback avoids a separate pre-pass to populate struct member maps
- [Phase 03-type-checker]: checkStructExpr and checkGetExpr extracted to struct_check.go — both exceed 30 lines, triggering REFACTOR condition from the plan
- [Phase 04-expression-evaluator-and-standard-library]: exprPos() helper uses type-switch since ast.Expr interface has no ExprPos() method (only unexported exprNode() marker)
- [Phase 04-expression-evaluator-and-standard-library]: ApplyExpr case in Eval() delegates to evalApply() immediately — stdlibFuncs dispatch works as soon as Wave 1 plans register functions
- [Phase 04-expression-evaluator-and-standard-library]: stdlibFuncs map starts empty; downstream plans register implementations by assigning to the package-level var
- [Phase 04-expression-evaluator-and-standard-library]: TokBangEq in plan maps to TokNotEq in lexer; TokBang maps to TokNot — corrected during implementation
- [Phase 04-expression-evaluator-and-standard-library]: GetExpr uses Object/Member field names; IndexExpr uses Collection field name — plan docs were aspirational
- [Phase 04-expression-evaluator-and-standard-library]: partOpts slice removed from evalCommand: options applied in valueToString before sentinel substitution — no need to store separately
- [Phase 04-expression-evaluator-and-standard-library]: Both tasks implemented in single TDD cycle: tests and implementation committed together since they cover the same file (stdlib_fileio.go)
- [Phase 04-expression-evaluator-and-standard-library]: jsonToValue integer detection: math.Trunc(x)==x guards whole-number float64 -> IntValue promotion
- [Phase 04-expression-evaluator-and-standard-library]: valueToStr helper in stdlib_array.go avoids name collision with valueToString(v, opts) in command.go
- [Phase 04-expression-evaluator-and-standard-library]: stdRound uses math.Floor(x+0.5) for WDL half-up semantics — round(-2.5)==-2, NOT math.Round which gives -3
- [Phase 04-expression-evaluator-and-standard-library]: stdMatches anchors pattern with ^$  to enforce whole-string match (partial match returns false)
- [Phase 04-expression-evaluator-and-standard-library]: MapValue.Type is a concrete types.MapType struct (no type assertion needed) — plan action suggested .(types.MapType) which is a compile error
- [Phase 04-expression-evaluator-and-standard-library]: collect_by_key uses linear scan with primitiveEqual() for insertion-order-preserving key deduplication
- [Phase 05-task-execution]: PathMap collision strategy: 8 hex chars from SHA-256 of host path as prefix — deterministic, compact
- [Phase 05-task-execution]: BindInputs/ParseMemoryString/ParseRuntime exported — external test package (task_test) requires exported symbols
- [Phase 05-task-execution]: jsonToValueUntyped duplicated from eval package — eval.jsonToValue is unexported, cannot cross package boundary
- [Phase 05-task-execution]: ForEach(fn) and Parent() added to eval.Env — minimal exports for translateEnv in task/command.go; preserves encapsulation
- [Phase 05-task-execution]: coerceOutputValue converts StringValue to FileValue/DirectoryValue for File/Directory output decls — eval.Eval returns StringValue for string interpolation
- [Phase 05-task-execution]: BuildCommand/CollectOutputs exported (capital B/C) — task_test external test package requires exported symbols
- [Phase 05-task-execution]: DockerRunner stateless (no fields) creates Docker client per Run() call for thread-safety; ContainerWait before ContainerStart prevents race condition on fast-exiting containers; AutoRemove disabled to allow post-exit log collection
- [Phase 06-workflow-execution-dag-scheduler]: dagNode tracks top-level workflow elements (call, scatter, conditional); nested calls inside scatter/conditional are NOT separate DAG nodes
- [Phase 06-workflow-execution-dag-scheduler]: callID() returns alias if set, else last callee element — matches WDL spec call references
- [Phase 06-workflow-execution-dag-scheduler]: topoSort uses Kahn's algorithm with sorted queue for deterministic ordering of independent nodes
- [Phase 06-workflow-execution-dag-scheduler]: Call outputs bound as StructValue under call alias — enables GetExpr alias.field resolution via eval.evalGet
- [Phase 06-workflow-execution-dag-scheduler]: taskRunner field on scheduler propagates to child schedulers — avoids global runTaskFn races in executeBody/scatter/conditional
- [Phase 06-workflow-execution-dag-scheduler]: injectDeclNodes() two-pointer merge inserts Decl nodes into topo order at original body positions
- [Phase 06-workflow-execution-dag-scheduler]: outputs.json not written on any task failure; writeErrorJSON uses errors.As for TaskError extraction from wrapped error chains; valueToJSONOutput relativizes File/Directory paths via filepath.Rel
- [Phase 07-cli-and-integration]: ParseWithSearchPaths delegates to parseFileWithPaths with nil searchPaths for parseFile backward compat
- [Phase 07-cli-and-integration]: run(args, stdout, stderr) testable-main pattern with no subprocess needed in CLI tests
- [Phase 07-cli-and-integration]: TypeError rustc-style formatting in main.go readSourceLine+printRustcError — checker TypeError.Error() lacks source line
- [Phase 07-cli-and-integration]: newRunner package-level factory var enables ContainerRunner injection without subprocess in CLI tests
- [Phase 07-cli-and-integration]: printVerboseOutput post-hoc approach reads stdout.txt/stderr.txt after execution (zero API changes to inner packages)

### Pending Todos

None yet.

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 1 | Create test data, sample workflow, and testing documentation for flowy | 2026-03-11 | a55514c | [1-create-test-data-sample-workflow-and-tes](./quick/1-create-test-data-sample-workflow-and-tes/) |
| 2 | Fix type checker to register call output in workflow scope | 2026-03-11 | 9d024c5 | [2-fix-type-checker-to-register-call-output](./quick/2-fix-type-checker-to-register-call-output/) |
| 3 | Restore output blocks to all example WDL workflows | 2026-03-11 | 21e4be2 | [3-restore-output-blocks-to-all-example-wdl](./quick/3-restore-output-blocks-to-all-example-wdl/) |
| 4 | Fix workflow input binding to support fully-qualified JSON keys | 2026-03-11 | 4b60e61 | [4-fix-workflow-input-binding-to-support-fu](./quick/4-fix-workflow-input-binding-to-support-fu/) |
| 5 | Fix relative path coercion in task File/Directory outputs | 2026-03-11 | 2642dd2 | [5-fix-relative-path-coercion-in-task-file-](./quick/5-fix-relative-path-coercion-in-task-file-/) |

### Roadmap Evolution

- Phase 8 added: Add spectool integration for WDL specification compliance testing

### Blockers/Concerns

- [Research]: Phase 5 DAG scheduler needs deeper review of miniwdl runtime/workflow.py StateMachine before planning — scatter-in-conditional nesting is complex
- [Research]: ContainerRunner interface exact signature (env vars, resource limits, timeout fields) should be finalized before Phase 4 planning begins
- [Research]: errgroup vs. channel-based scheduler pattern interaction needs resolution during Phase 5 planning

## Session Continuity

Last session: 2026-03-11T01:59:00.156Z
Stopped at: Phase 8 context gathered
Resume file: .planning/phases/08-add-spectool-integration-for-wdl-specification-compliance-testing/08-CONTEXT.md
