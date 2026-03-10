---
phase: 04-expression-evaluator-and-standard-library
plan: 02
subsystem: eval
tags: [go, ast, expression-evaluator, binary-operators, unary-operators, wdl, tdd]

# Dependency graph
requires:
  - phase: 04-expression-evaluator-and-standard-library
    plan: 01
    provides: "wdl/eval package scaffold with Eval() type-switch skeleton and test stubs"
  - phase: 02-lexer-parser-and-ast
    provides: "ast.Expr interface and all concrete expression node types; lexer token constants"
  - phase: 01-type-data-structures
    provides: "values.Value interface and all concrete value types; types package"
provides:
  - "Fully implemented Eval() for all 14 ast.Expr cases (literals, binary/unary ops, if-then-else, ident, compound constructors, struct/pair/array/map access)"
  - "evalBinary: WDL arithmetic promotion, ** -> Float via math.Pow, integer division truncation, comparison, logical operators"
  - "evalUnary: negate Int/Float, invert Boolean"
  - "evalGet: struct field access and pair left/right access with EvalError on miss"
  - "evalIndex: array bounds-checked int index and map linear-scan key lookup"
  - "primitiveEqual helper for map key comparison"
  - "TestEvalBinary, TestEvalUnary, TestEvalIfThenElse, TestEvalGetExpr, TestEvalIndexExpr all pass"
affects:
  - 04-03-stdlib-string-command
  - 04-04-stdlib-file-io
  - 04-05-stdlib-array-string
  - 04-06-stdlib-numeric

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "evalBinary dispatches on lexer.Tok* constants via switch — same token constants used by parser"
    - "WDL Int op Float arithmetic: check either side for FloatValue, promote Int with float64() before operation"
    - "evalGet and evalIndex as standalone helpers called from Eval() type-switch"
    - "primitiveEqual linear scan for map key lookup — compound types always return false"

key-files:
  created: []
  modified:
    - wdl/eval/eval.go
    - wdl/eval/eval_test.go

key-decisions:
  - "TokBangEq in plan maps to TokNotEq in lexer; TokBang maps to TokNot — fixed during implementation"
  - "GetExpr uses Object/Member field names (not Expr/Field as plan docs stated) — matched actual ast.GetExpr struct"
  - "IndexExpr uses Collection/Index field names (not Expr/Index as plan docs stated) — matched actual ast.IndexExpr struct"
  - "Both tasks implemented in single TDD cycle: tests and implementation committed together due to compound constructors spanning both tasks"

patterns-established:
  - "evalGet: type-switch on StructValue vs PairValue; PairValue uses string match on 'left'/'right'"
  - "evalIndex: type-switch on ArrayValue vs MapValue; MapValue uses primitiveEqual linear scan"
  - "arithmetic helpers (asFloat, asInt, toFloat) separated from evalArith to keep logic readable"

requirements-completed: [EXPR-01, EXPR-02, EXPR-03, EXPR-04]

# Metrics
duration: 8min
completed: 2026-03-10
---

# Phase 4 Plan 02: Expression Evaluator Core Summary

**Eval() fully implemented for all 14 WDL expression types: literals, arithmetic/comparison/logical binary ops with Int/Float promotion, unary minus/not, if-then-else, ident lookup, compound constructors, struct/pair field access, and array/map index access**

## Performance

- **Duration:** 8 min
- **Started:** 2026-03-10T10:15:00Z
- **Completed:** 2026-03-10T10:23:00Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- All 14 Eval() type-switch cases implemented — no more "not yet implemented" stubs for EXPR-01 through EXPR-04
- evalBinary handles all 14 operators with correct WDL promotion rules (Int+Float->Float, **->Float via math.Pow, integer division truncates)
- evalGet and evalIndex helpers deliver EXPR-03 and EXPR-04 with correct EvalError on miss/out-of-bounds
- All 5 test functions (TestEvalBinary, TestEvalUnary, TestEvalIfThenElse, TestEvalGetExpr, TestEvalIndexExpr) pass with 35+ sub-cases
- Full test suite `go test ./...` exits 0 — no regressions

## Task Commits

Each task was committed atomically:

1. **Task 1 + Task 2: Implement all Eval() cases including GetExpr/IndexExpr** - `9e14dcc` (feat)

**Plan metadata:** _(final docs commit hash pending)_

_Note: Both tasks implemented in a single TDD cycle. The compound constructors (ArrayExpr, MapExpr, PairExpr) required by Task 1 and the helpers for Task 2 (evalGet, evalIndex) were all implemented together in one cohesive implementation pass. Tests for both tasks were written first (RED) then implementation (GREEN) in the same session._

## Files Created/Modified
- `wdl/eval/eval.go` - All 14 Eval() cases implemented; evalBinary, evalUnary, evalGet, evalIndex, primitiveEqual, asFloat, asInt, toFloat helpers
- `wdl/eval/eval_test.go` - TestEvalBinary (29 sub-cases), TestEvalUnary (5 sub-cases), TestEvalIfThenElse (3 sub-cases), TestEvalGetExpr (6 sub-cases), TestEvalIndexExpr (7 sub-cases)

## Decisions Made
- **TokBangEq -> TokNotEq, TokBang -> TokNot:** The plan documentation used aspirational token names; the actual lexer constants are TokNotEq and TokNot. Matched actual code.
- **ast.GetExpr.Object/Member not .Expr/.Field:** Plan docs listed field names inconsistent with actual ast.GetExpr struct. Used actual field names discovered in wdl/ast/expr.go.
- **ast.IndexExpr.Collection not .Expr:** Same pattern — plan docs were aspirational. Used actual Collection field name.
- **Single commit for both tasks:** Tasks 1 and 2 share the same file (eval.go) and the implementation naturally spans both. Writing them as a cohesive unit then verifying both test sets is more correct than artificially splitting.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Corrected lexer token constant names in evalBinary/evalUnary**
- **Found during:** Task 1 (implementing evalBinary and evalUnary)
- **Issue:** Plan documentation used TokBangEq and TokBang but actual lexer exports TokNotEq and TokNot
- **Fix:** Used TokNotEq for != operator and TokNot for unary ! operator
- **Files modified:** wdl/eval/eval.go
- **Verification:** `go build ./wdl/eval/` exits 0; TestEvalBinary != case passes
- **Committed in:** 9e14dcc (Task 1 commit)

**2. [Rule 1 - Bug] Corrected ast.GetExpr and ast.IndexExpr field names**
- **Found during:** Task 2 (implementing evalGet and evalIndex)
- **Issue:** Plan code snippets used e.Expr/e.Field for GetExpr and e.Expr for IndexExpr; actual struct has Object/Member and Collection fields
- **Fix:** Used Object.Member fields for GetExpr and Collection field for IndexExpr as defined in wdl/ast/expr.go
- **Files modified:** wdl/eval/eval.go
- **Verification:** All TestEvalGetExpr and TestEvalIndexExpr sub-cases pass
- **Committed in:** 9e14dcc (Task 1+2 commit)

---

**Total deviations:** 2 auto-fixed (2 bugs — aspirational plan docs vs actual code field/constant names)
**Impact on plan:** Essential fixes for compilation. No scope creep.

## Issues Encountered
- Plan documentation had aspirational token/field names inconsistent with implemented code from prior phases. Discovered during implementation and corrected via deviation Rule 1.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- EXPR-01 through EXPR-04 fully delivered
- Eval() has no more "not yet implemented" stubs for core expression types
- StringExpr and CommandExpr still return "not yet implemented" — these are EXPR-05/EXPR-06 scope for plans 04-03+
- stdlibFuncs dispatch map is ready to receive stdlib registrations from plans 04-04 through 04-06
- Wave 2 plans (04-03 through 04-06) can build on the full expression evaluation foundation

---
*Phase: 04-expression-evaluator-and-standard-library*
*Completed: 2026-03-10*
