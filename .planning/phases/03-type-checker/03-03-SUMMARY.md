---
phase: 03-type-checker
plan: "03"
subsystem: type-checker
tags: [go, wdl, type-checking, tdd, toposort, scatter, if-promotion]

# Dependency graph
requires:
  - phase: 03-02
    provides: Coerces() with 6 WDL coercion rules; optionalize() with idempotency guard
  - phase: 03-01
    provides: checker package skeleton, TypeError, TypeEnv, Scope, test stubs

provides:
  - "Full Check() implementation: two-pass type resolution with Kahn toposort"
  - "collectWorkflowBodyGroups: scatter/if promotion with inner scope for loop variable"
  - "validateGroups: Kahn toposort cycle detection + literal/ident type mismatch reporting"
  - "collectIdents: full AST walk for dependency edge extraction"
  - "scatterVarElementType: derives scatter loop variable type from Array collection"
  - "TestForwardRef, TestCycleDetection, TestTypeMismatch, TestScatterPromotion, TestIfPromotion: all PASS"

affects:
  - 03-04 (struct literal and field-access checking can reuse Check() infrastructure)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Kahn's algorithm for toposort: inDegree map + dependents map + zero-in-degree queue"
    - "declGroup struct: pairs ast.Decl with its validation Scope (inner scope for scatter bodies)"
    - "collectWorkflowBodyGroups + collectWorkflowBodyGroupsInner: dual-pass registration with promotion functions"
    - "scatterVarElementType: IdentExpr lookup -> ArrayType.ItemType, fallback to AnyType{}"
    - "promoteFn closure composition: outermost promotion wraps innermost (scatter-inside-if produces Array[T?])"

key-files:
  created: []
  modified:
    - wdl/checker/checker.go
    - wdl/checker/checker_test.go

key-decisions:
  - "declGroup struct introduced to pair each ast.Decl with its validation scope — required for scatter body declarations to resolve the loop variable (item) during type checking"
  - "Cycle error message prefixed with 'cycle:' to satisfy strings.Contains(err, 'cycle') assertion in TestCycleDetection"
  - "collectWorkflowBodyGroupsInner as separate function from collectWorkflowBodyGroups: avoids leaking innerScope into Conditional processing at the same nesting level"
  - "Toposort edges only include peer declarations (outerScope.decls), not inherited scope names — prevents cross-scope false cycle detection"
  - "Complex expressions (BinaryExpr, ApplyExpr, StringExpr with interpolation) return nil from checkExprType — deferred to Phase 4"

patterns-established:
  - "Two-pass check pattern: Pass 1 registers all names first (forward reference support), Pass 2 validates in topo order"
  - "Inner scope for scatter variable: created per-Scatter, not shared with outer — enables proper loop variable scoping"

requirements-completed:
  - TYPE-06
  - TYPE-07

# Metrics
duration: 8min
completed: "2026-03-10"
---

# Phase 3 Plan 03: Core Type Checker — Two-Pass Resolution and Kahn Toposort Summary

**Full Check() with two-pass resolution: Kahn toposort for cycle detection, scatter/if output promotion via arrayize/optionalize, and type mismatch detection for literals and ident references**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-03-10T08:19:00Z
- **Completed:** 2026-03-10T08:27:43Z
- **Tasks:** 1 (TDD: RED confirmed, GREEN implemented, REFACTOR reviewed — no changes needed)
- **Files modified:** 2

## Accomplishments

- `Check(doc *ast.Document)` fully implemented with two-pass design
- Pass 1: `checkTask` and `checkWorkflow` register all declared types into per-scope maps; `collectWorkflowBodyGroups` applies `arrayize()` for scatter bodies and `optionalize()` for conditional bodies
- Pass 2: `validateGroups` runs Kahn's algorithm per scope for cycle detection, then calls `checkExprType` in topological order
- `declGroup` struct introduced to allow scatter body declarations to be validated against an inner scope that includes the scatter loop variable
- `scatterVarElementType` derives the loop variable type from the scatter collection expression (handles IdentExpr → ArrayType.ItemType; AnyType fallback for complex expressions)
- `collectIdents` walks the full AST expression tree to extract all IdentExpr names for dependency edge construction
- All 5 target tests pass: TestForwardRef, TestCycleDetection, TestTypeMismatch, TestScatterPromotion, TestIfPromotion
- Pre-existing passing tests (TestCoerces, TestOptionalizeIdempotent) remain green
- Pre-existing failing stubs (TestStructLiteral, TestStructFieldAccess, TestStructUndefined) remain in place for Plan 04

## Task Commits

1. **RED: Failing test stubs confirmed (from Plan 01)** — `6ab8b2e` / existing stubs with `t.Fatal("not implemented")`
2. **GREEN: Implement Check() and remove t.Fatal guards** — `1c4a1d4` (feat — checker.go + checker_test.go)

## Files Created/Modified

- `wdl/checker/checker.go` — Full Check() implementation: two-pass resolution, Kahn toposort, scatter/if promotion, type mismatch detection, collectIdents
- `wdl/checker/checker_test.go` — Removed `t.Fatal("not implemented")` guards from TestForwardRef, TestCycleDetection, TestTypeMismatch, TestScatterPromotion, TestIfPromotion

## Decisions Made

- `declGroup` struct introduced: pairs `*ast.Decl` with `*Scope` for validation — required because scatter body declarations are validated against an inner scope that contains the scatter loop variable
- Cycle error prefixed with `"cycle:"` to satisfy `strings.Contains(errs[0].Error(), "cycle")` in TestCycleDetection (plan-specified message format includes "cycle" but not as a standalone word)
- `collectWorkflowBodyGroupsInner` is a separate function from `collectWorkflowBodyGroups` to correctly propagate innerScope into nested Scatter/Conditional nodes without affecting top-level Conditional processing
- Toposort dependency edges only include names in `outerScope.decls` (direct peer declarations), not inherited parent scope names — prevents cross-scope false cycle detection

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added declGroup struct and inner scope for scatter variable resolution**
- **Found during:** GREEN phase, first test run
- **Issue:** TestScatterPromotion and TestIfPromotion failed with `undefined name "item"` — the scatter loop variable `item` was not in scope during validation of `String scatter_out = item`
- **Fix:** Introduced `declGroup` struct pairing each `ast.Decl` with its validation scope; created `scatterVarElementType` helper to derive loop variable type; added `collectWorkflowBodyGroupsInner` to propagate inner scope through nested nodes
- **Files modified:** wdl/checker/checker.go
- **Committed in:** 1c4a1d4

**2. [Rule 1 - Bug] Cycle error message updated to include "cycle" substring**
- **Found during:** GREEN phase, first test run
- **Issue:** TestCycleDetection expected `strings.Contains(err, "cycle")` but initial error was `"circular dependency among declarations: [a b]"` — does not contain "cycle"
- **Fix:** Changed error format to `"cycle: circular dependency among declarations: %v"`
- **Files modified:** wdl/checker/checker.go
- **Committed in:** 1c4a1d4

## Issues Encountered

None beyond the two auto-fixed deviations above.

## User Setup Required

None.

## Next Phase Readiness

- `Check()` is fully implemented and tested for forward references, cycle detection, type mismatch, scatter/if promotion
- Plan 04 (struct checking) can extend `Check()` by handling `StructExpr` in `checkExprType` and adding struct member validation
- The `declGroup` / `validateGroups` infrastructure is reusable for struct scope validation in Plan 04

## Self-Check: PASSED

- FOUND: wdl/checker/checker.go
- FOUND: wdl/checker/checker_test.go
- FOUND commit: 1c4a1d4 feat(03-03): implement Check() with two-pass type resolution and Kahn toposort

---
*Phase: 03-type-checker*
*Completed: 2026-03-10*
