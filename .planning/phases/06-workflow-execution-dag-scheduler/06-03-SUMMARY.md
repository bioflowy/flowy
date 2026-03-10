---
phase: 06-workflow-execution-dag-scheduler
plan: 03
subsystem: wdl/workflow
tags: [output-serialization, fail-fast, json, workflow-execution]
dependency_graph:
  requires: [06-02]
  provides: [outputs.json-writing, error.json-writing, fail-fast-cancellation]
  affects: [wdl/workflow/workflow.go, wdl/workflow/output.go]
tech_stack:
  added: []
  patterns:
    - errors.As for TaskError extraction from wrapped error chains
    - json.MarshalIndent for human-readable output files
    - filepath.Rel for File/Directory path relativization
key_files:
  created:
    - wdl/workflow/output.go
    - wdl/workflow/output_test.go
  modified:
    - wdl/workflow/workflow.go
    - wdl/workflow/scheduler_test.go
decisions:
  - "outputs.json not written on any task failure — locked decision from plan interfaces"
  - "TaskError preserved through fmt.Errorf %w wrapping; errors.As extracts it in writeErrorJSON"
  - "valueToJSONOutput is a separate function from valueToJSON (same structure but relativizes File/Directory paths)"
  - "writeErrorJSON uses errors.As to detect *task.TaskError anywhere in the error chain (supports wrapped errors)"
metrics:
  duration: 3min
  completed: 2026-03-10
  tasks_completed: 2
  files_created: 2
  files_modified: 2
---

# Phase 6 Plan 3: Workflow Output Serialization and Fail-Fast Cancellation Summary

Implemented outputs.json and error.json writing completing the workflow execution pipeline, with miniwdl-compatible key format and fail-fast error propagation.

## Objective

Complete the workflow execution pipeline by writing structured JSON output on success (`outputs.json`) and structured error on failure (`error.json`), matching miniwdl format for ecosystem compatibility.

## Tasks Completed

### Task 1: outputs.json and error.json serialization functions

Created `wdl/workflow/output.go` with three functions:
- `writeOutputsJSON(wfName, outputs, execRoot, destPath)`: Formats keys as `wfName.outputName`, converts values via `valueToJSONOutput` (which relativizes File/Directory paths from execRoot), then writes with `json.MarshalIndent`
- `valueToJSONOutput(v, execRoot)`: Same structure as `valueToJSON` but recurses with path relativization for `FileValue` and `DirectoryValue` using `filepath.Rel`
- `writeErrorJSON(err, destPath)`: Uses `errors.As` to extract `*task.TaskError` fields (TaskName, ExitCode); falls back to task="" and exit_code=-1 for generic errors

All 8 unit tests pass (TestWriteOutputsJSON_* and TestWriteErrorJSON_*).

### Task 2: Wire output writing into RunWorkflow

Updated `wdl/workflow/workflow.go` RunWorkflow to:
- On `s.run()` success: evaluates workflow output declarations against final env, writes `outputs.json`
- On `s.run()` error: writes `error.json`, returns error WITHOUT writing `outputs.json`

Fail-fast cancellation already worked via `errgroup.WithContext` — when any goroutine's `g.Go` function returns non-nil, errgroup cancels its derived context (`gCtx`), which propagates to all running tasks through the `ctx` parameter.

Added 5 integration tests to `scheduler_test.go` using `t.TempDir()` for isolated execRoot directories.

## Verification

```
go test -race ./wdl/workflow/... -count=1 -v
# 48 tests PASS
go build ./...
# BUILD OK
go vet ./wdl/workflow/...
# VET OK
```

## Deviations from Plan

None — plan executed exactly as written.

The fail-fast cancellation described in the plan was already implemented in Plan 06-02 via `errgroup.WithContext`. The scheduler's `g.Go` wrapper returns the task error, which causes errgroup to cancel `gCtx`, propagating cancellation to all running tasks. No additional changes to `scheduler.go` were needed.

## Key Links Established

- `RunWorkflow` calls `writeOutputsJSON` on success, `writeErrorJSON` on failure
- `writeOutputsJSON` uses `filepath.Rel(execRoot, path)` for File/Directory paths
- `writeErrorJSON` uses `errors.As(err, &taskErr)` to extract `*task.TaskError` through wrapping chains

## Self-Check: PASSED

- wdl/workflow/output.go: FOUND
- wdl/workflow/output_test.go: FOUND
- wdl/workflow/workflow.go: modified with writeOutputsJSON/writeErrorJSON calls
- Commits: ed6f5c6 (test RED task1), d3ebde4 (feat task1), 27e7b72 (test RED task2), 610c25f (feat task2)
