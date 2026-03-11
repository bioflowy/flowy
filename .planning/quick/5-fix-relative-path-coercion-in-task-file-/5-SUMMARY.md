---
phase: quick-5
plan: 1
subsystem: task-execution
tags: [testing, regression, outputs, path-resolution]
dependency_graph:
  requires: []
  provides: [regression-test-coerce-output-value]
  affects: [wdl/task]
tech_stack:
  added: []
  patterns: [table-driven-tests, internal-package-test]
key_files:
  created:
    - wdl/task/outputs_test.go
  modified: []
key_decisions:
  - "Used package task (internal) for outputs_test.go — coerceOutputValue is unexported; external package task_test cannot access it"
metrics:
  duration: 5min
  completed: 2026-03-11
---

# Phase quick-5 Plan 1: Fix Relative Path Coercion in Task File/Directory Outputs — Summary

**One-liner:** Regression tests for coerceOutputValue covering relative-path resolution against workDir for File and Directory output types.

## What Was Done

Added `wdl/task/outputs_test.go` with 7 table-driven subtests for `coerceOutputValue`. The fix in `outputs.go` was already applied; this plan adds test coverage to prevent future regression.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Add regression tests for coerceOutputValue | 2642dd2 | wdl/task/outputs_test.go |

## Test Coverage

All 7 test cases pass:

1. Relative path + File type: resolves against workDir (`"output.txt"` + `"/work"` → `"/work/output.txt"`)
2. Relative path + Directory type: resolves against workDir (`"results"` + `"/work"` → `"/work/results"`)
3. Absolute path + File type: unchanged (`"/abs/file.txt"` stays `"/abs/file.txt"`)
4. Absolute path + Directory type: unchanged (`"/abs/dir"` stays `"/abs/dir"`)
5. String type (no coercion): returns original StringValue
6. Non-string value: IntValue returned unchanged regardless of declared type
7. Empty workDir with relative path: relative path preserved as-is

## Deviations from Plan

### Auto-fixed Issues

None — plan executed exactly as written.

### Notes

- The plan noted that `coerceOutputValue` is unexported and suggested checking existing test file conventions. All existing files in `wdl/task/` use `package task_test`. However, since the function is unexported, `package task` (internal) was required. This matches the plan's own guidance ("if any use `package task`, follow that" — and the plan anticipated this case).

## Self-Check: PASSED

- File created: `/home/uehara/flowy/wdl/task/outputs_test.go` — FOUND
- Commit: `2642dd2` — FOUND
- All 7 subtests pass: CONFIRMED
- Full task package test suite passes: CONFIRMED
