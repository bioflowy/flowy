---
phase: quick-4
plan: 1
subsystem: workflow
tags: [bugfix, regression-test, json-input, workflow-execution]
dependency_graph:
  requires: []
  provides: [fully-qualified-json-key-lookup]
  affects: [wdl/workflow/convert.go]
tech_stack:
  added: []
  patterns: [try-fully-qualified-then-fallback]
key_files:
  created: []
  modified:
    - wdl/workflow/convert.go
    - wdl/workflow/convert_test.go
decisions:
  - "Fully-qualified key (wf.Name+'.'+decl.Name) checked before unqualified fallback — standard WDL 1.2 JSON input format"
metrics:
  duration: "~5min"
  completed: "2026-03-11"
  tasks: 2
  files_modified: 2
---

# Quick Task 4: Fix Workflow Input Binding to Support Fully-Qualified JSON Keys — Summary

**One-liner:** bindWorkflowInputs now resolves fully-qualified JSON keys like "hello.name" before falling back to bare "name", matching the WDL 1.2 input JSON standard.

## Objective

Add regression tests for the already-applied fix to `bindWorkflowInputs` in `wdl/workflow/convert.go` that enables fully-qualified JSON key lookup.

## What Was Done

### Task 1: Add unit tests for bindWorkflowInputs fully-qualified key support

Added four test functions to `wdl/workflow/convert_test.go`:

- `TestBindWorkflowInputs_FullyQualified`: workflow "hello" with JSON key "hello.name" resolves correctly
- `TestBindWorkflowInputs_UnqualifiedFallback`: JSON key "name" (bare) still resolves as backward-compatible fallback
- `TestBindWorkflowInputs_FullyQualifiedTakesPrecedence`: when both "hello.name" and "name" are in JSON, the fully-qualified value wins
- `TestBindWorkflowInputs_DefaultUsedWhenMissing`: absent JSON key with a default expression uses the default value

A `makeStringWorkflow` helper was introduced to construct minimal `ast.Workflow` fixtures for the tests.

### Task 2: Commit fix and tests

Staged both `convert.go` (the pre-applied fix) and `convert_test.go` (new tests) and committed atomically as `3b283ff`.

## Verification

- `go test ./wdl/workflow/ -run TestBindWorkflowInputs -v` — all four new tests pass
- `go test ./...` — no regressions across the full codebase

## Deviations from Plan

None — plan executed exactly as written. The fix was already present in the working tree as noted in the task context; only tests were added.

## Self-Check: PASSED

- File `wdl/workflow/convert_test.go` contains all four TestBindWorkflowInputs_* functions
- File `wdl/workflow/convert.go` contains `jsonInputs[wf.Name+"."+decl.Name]` fully-qualified lookup
- Commit `3b283ff` exists and contains both files
