---
phase: quick
plan: 2
subsystem: type-checker
tags: [bugfix, type-checker, call-output, scope-registration]
dependency_graph:
  requires: []
  provides: [call-output-type-resolution]
  affects: [wdl/checker]
tech_stack:
  added: []
  patterns: [registerCallOutputs-helper, two-pass-toposort-edge-building]
key_files:
  created:
    - wdl/testdata/call_output.wdl
    - wdl/testdata/scatter_call.wdl
  modified:
    - wdl/checker/checker.go
    - wdl/checker/checker_test.go
decisions:
  - "registerCallOutputs extracted as shared helper for both collect functions"
  - "Two-pass groupByName construction in validateGroups to handle forward references correctly"
  - "Toposort edge filter: only peer declarations (in groupByName), not call aliases"
metrics:
  duration: 3min
  completed: 2026-03-11
---

# Quick Task 2: Fix Type Checker to Register Call Output Summary

Register call aliases as StructType in workflow scope so call.output references resolve without false "undefined name" errors; scatter promotes members to Array[T], conditional to T?.

## What Changed

### Task 1: Test fixtures and failing tests (TDD RED)
- Created `wdl/testdata/call_output.wdl` (inline task+workflow with call output reference)
- Created `wdl/testdata/scatter_call.wdl` (scatter wrapping a call, output promoted to Array[String])
- Added 4 test cases: TestCallOutputInWorkflow, TestCallOutputInline, TestScatterCallPromotion, TestConditionalCallPromotion
- All tests failed with "undefined name" errors confirming the bug

### Task 2: Implement call output registration (TDD GREEN)
- Added `case *ast.Call` to `collectWorkflowBodyGroups` and `collectWorkflowBodyGroupsInner`
- Extracted `registerCallOutputs` helper that resolves the called task, builds a StructType from task outputs with promoteFn applied to each member, and registers it under the call alias (or task name)
- Modified `checkWorkflow` to accept `tasks []*ast.Task` and build a `taskMap` for call resolution
- Threaded `taskMap` through all recursive calls to collect functions
- Fixed `validateGroups` toposort edge building: split into two passes so that `groupByName` is fully populated before building dependency edges, and only peer declarations (not call aliases) are counted as dependencies

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Toposort cycle false positive on call alias references**
- **Found during:** Task 2
- **Issue:** After registering call aliases in `outerScope.decls`, the toposort edge builder treated them as peer declaration dependencies. Since call aliases have no corresponding `declGroup` node, any declaration referencing a call output was flagged as a cycle.
- **Fix:** Split `validateGroups` loop into two passes (first build `groupByName`, then build edges). Added `groupByName[ident]` check so only actual peer declarations are dependency edges.
- **Files modified:** wdl/checker/checker.go
- **Commit:** 9d024c5

## Verification

- `go test ./wdl/checker/ -v` -- all 15 tests pass, 0 failures
- `go vet ./wdl/checker/` -- no issues
- `go build ./...` -- full project compiles

## Self-Check: PASSED
