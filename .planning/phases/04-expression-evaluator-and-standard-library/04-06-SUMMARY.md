---
phase: 04-expression-evaluator-and-standard-library
plan: 06
subsystem: eval
tags: [go, stdlib, utility-functions, wdl, tdd, map-operations]

# Dependency graph
requires:
  - phase: 04-expression-evaluator-and-standard-library
    plan: 02
    provides: "primitiveEqual() helper in eval.go for safe primitive comparison"
  - phase: 04-expression-evaluator-and-standard-library
    plan: 04
    provides: "stdlib.go with stdlibFuncs dispatch map and evalApply()"
  - phase: 04-expression-evaluator-and-standard-library
    plan: 05
    provides: "31 stdlib functions registered (STDLIB-03, STDLIB-04, STDLIB-06)"
provides:
  - "STDLIB-05: 8 utility functions (defined, keys, values, as_map, as_pairs, collect_by_key, contains, contains_key) registered in stdlibFuncs"
  - "EXPR-05: full ApplyExpr dispatch chain verified end-to-end via TestEvalApply"
  - "TestStdlibUtility (11 sub-cases) and TestEvalApply (4 sub-cases) and TestStdlibCount all PASS"
  - "Total stdlibFuncs: 48 registered functions across all stdlib files"
affects:
  - "Phase 5 DAG scheduler — stdlib complete; all WDL 1.2 functions available at runtime"

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "primitiveEqual() reused from eval.go for contains/contains_key — no Go == on interface types"
    - "collect_by_key uses linear scan over resultEntries for key deduplication — correct for ordered insertion semantics"
    - "MapValue.Type accessed as concrete types.MapType struct (no type assertion needed)"

key-files:
  created:
    - wdl/eval/stdlib_util.go
  modified:
    - wdl/eval/stdlib_test.go
    - wdl/eval/eval_test.go

key-decisions:
  - "MapValue.Type is a concrete types.MapType struct, not an interface — remove type assertion (.(types.MapType)) from plan action"
  - "collect_by_key linear scan over resultEntries sufficient for WDL use cases — no hash map needed"
  - "48 total registered stdlib functions (plan estimated ~30) — all WDL 1.2 stdlib covered"

patterns-established:
  - "init() per file self-registers into stdlibFuncs — pattern consistent across all 5 stdlib files"

requirements-completed: [STDLIB-05, EXPR-05]

# Metrics
duration: 3min
completed: 2026-03-10
---

# Phase 4 Plan 06: Utility Stdlib and ApplyExpr Dispatch Verification Summary

**8 WDL 1.2 utility functions (defined/keys/values/as_map/as_pairs/collect_by_key/contains/contains_key) implemented and full ApplyExpr dispatch chain verified end-to-end with 48 total stdlib functions registered**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-10T10:32:42Z
- **Completed:** 2026-03-10T10:35:40Z
- **Tasks:** 2
- **Files modified:** 3 (1 created, 2 updated)

## Accomplishments

- **stdlib_util.go**: 8 STDLIB-05 functions registered; defined, keys, values, as_map, as_pairs, collect_by_key, contains, contains_key all implemented and verified
- **TestStdlibUtility**: 11 sub-cases PASS covering all 8 utility functions including collect_by_key with grouped key behavior
- **TestEvalApply**: 4 sub-cases PASS verifying full dispatch path: Eval() -> evalApply() -> stdlibFuncs[name] -> implementation
- **TestStdlibCount**: confirms 48 registered functions (>= 30 required by plan)
- **Full suite**: `go test ./...` exits 0, no regressions across all 7 packages

## Task Commits

1. **Task 1: STDLIB-05 utility functions** - `f5a2127` (feat)
2. **Task 2: End-to-end ApplyExpr dispatch verification** - `37397d1` (feat)

## Files Created/Modified

- `/home/uehara/flowy/wdl/eval/stdlib_util.go` - 8 utility functions, init() registration
- `/home/uehara/flowy/wdl/eval/stdlib_test.go` - TestStdlibUtility (11 sub-cases) implemented
- `/home/uehara/flowy/wdl/eval/eval_test.go` - TestEvalApply (4 sub-cases) + TestStdlibCount implemented

## Decisions Made

- **MapValue.Type is concrete struct, not interface**: Plan action suggested `mv.Type.(types.MapType)` type assertion, but `MapValue.Type` is declared as `types.MapType` (concrete struct), not `types.Type` interface. Removed the type assertion. Discovered at first compile, fixed immediately (Rule 1 deviation).
- **collect_by_key linear scan**: Used linear scan over resultEntries to find matching keys with `primitiveEqual()`. This preserves insertion order correctly and avoids unsafe Go map comparisons on interface values.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Removed invalid type assertions from MapValue.Type access**
- **Found during:** Task 1 (first compile attempt)
- **Issue:** Plan action suggested `mv.Type.(types.MapType)` but `values.MapValue.Type` is declared as `types.MapType` (concrete struct), making the type assertion a compile error
- **Fix:** Accessed `mv.Type.KeyType` and `mv.Type.ValType` directly without assertion
- **Files modified:** wdl/eval/stdlib_util.go
- **Verification:** `go build ./wdl/eval/` exits 0; TestStdlibUtility all 11 sub-cases pass
- **Committed in:** f5a2127

---

**Total deviations:** 1 auto-fixed (1 bug — invalid type assertion on concrete struct field)
**Impact on plan:** Trivial fix. No behavior change, no scope creep.

## Issues Encountered

- Plan action suggested type assertion on MapValue.Type but the field is a concrete struct. This is a doc consistency issue; the fix was a one-line change per affected function.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All 48 stdlib functions registered and tested across 5 files (stdlib_fileio.go, stdlib.go, stdlib_array.go, stdlib_string.go, stdlib_numeric.go, stdlib_util.go)
- EXPR-01 through EXPR-07 and STDLIB-01 through STDLIB-06 all addressed
- Phase 4 is complete
- Phase 5 DAG scheduler can proceed with full WDL 1.2 stdlib available

---
*Phase: 04-expression-evaluator-and-standard-library*
*Completed: 2026-03-10*

## Self-Check: PASSED

- FOUND: wdl/eval/stdlib_util.go
- FOUND: wdl/eval/stdlib_test.go (modified)
- FOUND: wdl/eval/eval_test.go (modified)
- FOUND: commit f5a2127 (feat(04-06): implement STDLIB-05 utility functions)
- FOUND: commit 37397d1 (feat(04-06): verify end-to-end ApplyExpr dispatch and full suite green)
