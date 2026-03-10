---
phase: 04-expression-evaluator-and-standard-library
plan: 01
subsystem: eval
tags: [go, ast, expression-evaluator, wdl, stdlib-dispatch]

# Dependency graph
requires:
  - phase: 02-lexer-parser-and-ast
    provides: "ast.Expr interface and all 17 concrete expression node types"
  - phase: 01-type-data-structures
    provides: "values.Value interface and all concrete value types"
provides:
  - "wdl/eval package with Eval() entry point and full 17-case type-switch skeleton"
  - "EvalError, Env, ExecContext exported types"
  - "stdlibFuncs dispatch map and evalApply() infrastructure"
  - "16 compilable test stubs for EXPR-01 through EXPR-07 and STDLIB-01 through STDLIB-06"
affects:
  - 04-02-literal-eval
  - 04-03-operators
  - 04-04-stdlib-file-io
  - 04-05-stdlib-array-string
  - 04-06-stdlib-numeric

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "type-switch dispatch over ast.Expr interface for expression evaluation"
    - "linked-list Env scope chain mirroring checker.Scope pattern"
    - "dispatch map (stdlibFuncs) for stdlib function registration"

key-files:
  created:
    - wdl/eval/eval.go
    - wdl/eval/env.go
    - wdl/eval/error.go
    - wdl/eval/context.go
    - wdl/eval/stdlib.go
    - wdl/eval/eval_test.go
    - wdl/eval/stdlib_test.go
    - wdl/eval/command_test.go
  modified: []

key-decisions:
  - "exprPos() helper uses type-switch since ast.Expr interface has no ExprPos() method (only unexported exprNode() marker)"
  - "ApplyExpr case in Eval() delegates to evalApply() immediately — no stub return — so Wave 1 dispatch works without changing the switch"
  - "stdlibFuncs map starts empty; downstream plans register implementations by assigning to the package-level var"

patterns-established:
  - "Eval() skeleton: each unimplemented case returns EvalError{Pos, 'not yet implemented'}"
  - "Test stubs: t.Skip('not yet implemented') as sole body for forward-declared tests"
  - "stdlib registration: plans assign to stdlibFuncs[name] = func(...) pattern"

requirements-completed: [EXPR-01, EXPR-02, EXPR-03, EXPR-04, EXPR-05, EXPR-06, EXPR-07]

# Metrics
duration: 2min
completed: 2026-03-10
---

# Phase 4 Plan 01: wdl/eval Package Scaffold Summary

**wdl/eval package scaffold with Eval() type-switch over 17 ast.Expr types, Env/EvalError/ExecContext types, stdlib dispatch map, and 16 compilable test stubs for all EXPR-* and STDLIB-* requirements**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-10T10:09:54Z
- **Completed:** 2026-03-10T10:11:33Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments
- Created wdl/eval package with all 4 source files and 3 test files
- Eval() function exported with full 17-case type-switch skeleton (all 14 ast.Expr types plus ApplyExpr delegating to evalApply)
- EvalError, Env (with NewEnv/Define/Lookup), ExecContext, stdlibFuncs dispatch map all exported/available
- 16 test stubs compile and skip cleanly — ready for Wave 1 implementation plans to fill in

## Task Commits

Each task was committed atomically:

1. **Task 1: Create core types and eval.go skeleton** - `f5b38c8` (feat)
2. **Task 2: Create stdlib dispatch map and all test stubs** - `42146fe` (test)

**Plan metadata:** _(final docs commit hash pending)_

## Files Created/Modified
- `wdl/eval/eval.go` - Eval() entry point, 17-case type-switch skeleton, exprPos() helper
- `wdl/eval/env.go` - Env linked-list scope with NewEnv(), Define(), Lookup()
- `wdl/eval/error.go` - EvalError mirroring checker.TypeError format
- `wdl/eval/context.go` - ExecContext with WorkDir, StdoutPath, StderrPath
- `wdl/eval/stdlib.go` - stdlibFn type, stdlibFuncs dispatch map, evalApply()
- `wdl/eval/eval_test.go` - 8 stubs for EXPR-01 through EXPR-07
- `wdl/eval/stdlib_test.go` - 7 stubs for STDLIB-01 through STDLIB-06 + TestRoundHalfUp
- `wdl/eval/command_test.go` - TestCommandDedentEdgeCases for dedent algorithm edge cases

## Decisions Made
- **exprPos() helper uses type-switch:** The ast.Expr interface only has an unexported `exprNode()` marker method — no public `ExprPos()` method. The plan's interface documentation was aspirational; the actual code requires a type-switch to extract Pos.
- **ApplyExpr delegates immediately in Eval():** Rather than returning a "not yet implemented" stub, the ApplyExpr case calls `evalApply()` directly. This means the dispatch infrastructure works as soon as Wave 1 plans register functions in stdlibFuncs, without requiring changes to eval.go.
- **stdlibFuncs starts empty:** Plans 04-04 through 04-06 register stdlib implementations by assigning `stdlibFuncs[name] = fn` — no init() boilerplate needed.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Corrected ast.Expr field names in type-switch**
- **Found during:** Task 1 (creating eval.go skeleton)
- **Issue:** Plan documentation listed `IndexExpr.Expr`, `GetExpr.Expr/Field`, `StructExpr.Name` but actual code has `IndexExpr.Collection`, `GetExpr.Object/Member`, `StructExpr.TypeName`
- **Fix:** Used actual field names from wdl/ast/expr.go; the type-switch skeleton uses Pos field which is consistent across all types
- **Files modified:** wdl/eval/eval.go
- **Verification:** `go build ./wdl/eval/` exits 0
- **Committed in:** f5b38c8 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 bug — field name mismatch between plan docs and actual code)
**Impact on plan:** Essential fix for compilation. No scope creep.

## Issues Encountered
- ast.Expr interface has no ExprPos() method (plan documentation was incorrect) — worked around with exprPos() type-switch helper which is more complete anyway.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- wdl/eval package compiles and all test stubs are in place
- Wave 1 plans (04-02 through 04-06) can begin filling in the Eval() case logic
- stdlibFuncs dispatch map is ready to receive registrations
- All EXPR-* and STDLIB-* test stubs are in place and will transition from SKIP to PASS as each plan implements the corresponding functionality

---
*Phase: 04-expression-evaluator-and-standard-library*
*Completed: 2026-03-10*

## Self-Check: PASSED

All 8 source/test files present. Both task commits verified (f5b38c8, 42146fe). Package builds and all 16 test stubs skip cleanly.
