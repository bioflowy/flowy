---
phase: 03-type-checker
plan: "04"
subsystem: type-checker
tags: [go, wdl, type-checking, tdd, struct, field-access]

# Dependency graph
requires:
  - phase: 03-03
    provides: Full Check() implementation with two-pass resolution and Kahn toposort

provides:
  - "checkStructExpr(): validates StructExpr against structDefs — undefined struct, missing/unknown field, field type mismatch"
  - "checkGetExpr(): validates GetExpr field access — non-struct base, unknown field, fallback to structDefs for name-only StructType"
  - "Check() Pass 1 extended with structDefs map built from doc.Structs"
  - "struct_check.go: extracted struct/field-access helpers (>30 lines per REFACTOR rule)"
  - "TestStructLiteral, TestStructFieldAccess, TestStructUndefined: all PASS"
  - "go test ./... fully green — Phase 3 complete"

affects:
  - Phase 4 (runtime evaluation can extend checkExprType for BinaryExpr, ApplyExpr etc.)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "structDefs map[string]*ast.StructDef built in Check() Pass 1 from doc.Structs and threaded through checkTask/checkWorkflow/validateGroups/checkExprType"
    - "deep-copy of StructType.Members before returning resolved StructType (per types.StructType warning)"
    - "structDefs fallback in checkGetExpr: when scope holds a name-only StructType (Pass 1 registered type with nil Members), look up field types from structDefs"

key-files:
  created:
    - wdl/checker/struct_check.go
  modified:
    - wdl/checker/checker.go
    - wdl/checker/checker_test.go

key-decisions:
  - "checkGetExpr falls back to structDefs when StructType.Members is nil: Pass 1 registers declared types (StructType{Name:'Point'} with no Members); the resolved type with Members is only known after checkStructExpr runs for the initializer expression. The fallback avoids requiring a two-stage resolution within a single pass."
  - "checkStructExpr and checkGetExpr extracted to struct_check.go: both exceed 30 lines (checkStructExpr ~97 lines, checkGetExpr ~55 lines), triggering the REFACTOR rule from the plan"
  - "structDefs threaded as parameter through checkTask, checkWorkflow, validateGroups, checkExprType: avoids global state and keeps functions pure and testable"

patterns-established:
  - "structDefs fallback pattern: look up full field map from structDefs when a StructType in scope has no Members populated"

requirements-completed:
  - TYPE-08

# Metrics
duration: 4min
completed: "2026-03-10"
---

# Phase 3 Plan 04: Struct Literal and Field Access Validation Summary

**checkStructExpr and checkGetExpr integrated into Check() pass 2 via struct_check.go; all struct validation tests pass; go test ./... fully green; Phase 3 complete**

## Performance

- **Duration:** ~4 min
- **Started:** 2026-03-10T08:31:57Z
- **Completed:** 2026-03-10T08:35:57Z
- **Tasks:** 1 (TDD: RED confirmed, GREEN implemented, REFACTOR applied)
- **Files modified:** 2
- **Files created:** 1

## Accomplishments

- `Check()` Pass 1 now builds `structDefs map[string]*ast.StructDef` from `doc.Structs` and threads it through to `checkTask`, `checkWorkflow`, `validateGroups`, and `checkExprType`
- `checkStructExpr` implemented: validates struct name existence, presence of all required fields, absence of unknown fields, and field value type compatibility via `Coerces()`
- `checkGetExpr` implemented: resolves base IdentExpr type, asserts `StructType`, looks up field in Members with structDefs fallback for name-only StructType entries registered by Pass 1
- Both functions extracted to `struct_check.go` per the REFACTOR rule (both exceed 30 lines)
- `t.Fatal("not implemented")` guards removed from TestStructLiteral, TestStructFieldAccess, TestStructUndefined
- All three struct tests PASS; all pre-existing tests continue to PASS; `go test ./...` is fully green
- Phase 3 (wdl/checker) is complete

## Task Commits

1. **feat(03-04): implement struct literal and field access validation** — `5b4625c` (checker.go + checker_test.go + struct_check.go)

## Files Created/Modified

- `wdl/checker/struct_check.go` — New file: checkStructExpr and checkGetExpr implementations
- `wdl/checker/checker.go` — Extended: structDefs map in Check() Pass 1; structDefs threaded through call chain; checkExprType extended with StructExpr and GetExpr cases; checkStructExpr/checkGetExpr removed (moved to struct_check.go)
- `wdl/checker/checker_test.go` — Removed `t.Fatal("not implemented")` from TestStructLiteral, TestStructFieldAccess, TestStructUndefined

## Decisions Made

- `checkGetExpr` falls back to `structDefs` when `StructType.Members` is nil: Pass 1 registers declared types using `d.Type` directly from the AST (a `StructType` with no Members populated). The resolved type with Members is only produced by `checkStructExpr` after evaluating the initializer. The fallback avoids requiring a separate pre-pass to populate struct member maps.
- `checkStructExpr` and `checkGetExpr` extracted to `struct_check.go`: both exceed 30 lines (97 and 55 respectively), triggering the REFACTOR condition stated in the plan.
- `structDefs` threaded as a parameter (not global state): consistent with the existing pattern of passing explicit context through the call chain.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed checkGetExpr failing on name-only StructType from Pass 1**
- **Found during:** GREEN phase, first test run (TestStructLiteral and TestStructFieldAccess both failed)
- **Issue:** `scope.lookup("p")` returns `StructType{Name:"Point", Members:nil}` because Pass 1 registers `d.Type` from the AST declaration, which is a name-only StructType without a Members map. `checkGetExpr` then checked `Members["x"]` on a nil map and reported "has no field x".
- **Fix:** Added structDefs fallback in `checkGetExpr`: when `st.Members` is empty, look up the struct definition in `structDefs` and build the members map from `def.Members`.
- **Files modified:** wdl/checker/struct_check.go (checkGetExpr function)
- **Committed in:** 5b4625c

## Issues Encountered

One auto-fixed bug (see above). No other issues.

## User Setup Required

None.

## Next Phase Readiness

- Phase 3 is fully complete: all TYPE-05 through TYPE-08 requirements satisfied
- `go test ./...` is green across all packages
- Phase 4 (runtime evaluation) can extend `checkExprType` for BinaryExpr, ApplyExpr, etc. using the same structDefs pattern established here

## Self-Check: PASSED

- FOUND: wdl/checker/struct_check.go
- FOUND: wdl/checker/checker.go
- FOUND: wdl/checker/checker_test.go
- FOUND commit: 5b4625c feat(03-04): implement struct literal and field access validation

---
*Phase: 03-type-checker*
*Completed: 2026-03-10*
