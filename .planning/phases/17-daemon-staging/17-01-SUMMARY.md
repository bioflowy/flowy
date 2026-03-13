---
phase: 17-daemon-staging
plan: "01"
subsystem: api-contract
tags: [api, store, sqlite, migration, daemon, inputs, outputs]
dependency_graph:
  requires: []
  provides:
    - TaskClaimResponse.Inputs and WDLContent fields
    - TaskResultRequest.Outputs field
    - store.Task.InputsJSON and OutputsJSON fields
    - Store.UpdateTaskOutputs and UpdateTaskInputs methods
    - SQLite migration 00002 (inputs_json, outputs_json columns)
    - Server-side API-based input/output delivery
  affects:
    - internal/api/types.go
    - internal/store/store.go
    - internal/store/sqlite/tasks.go
    - internal/server/handlers.go
    - internal/server/dispatch.go
tech_stack:
  added: []
  patterns:
    - TDD (RED then GREEN)
    - Store fallback pattern: DB-first, file-based fallback for backward compat
    - Server method loadTaskOutputs replaces standalone function to access store
key_files:
  created:
    - internal/store/sqlite/migrations/00002_task_outputs.sql
  modified:
    - internal/api/types.go
    - internal/store/store.go
    - internal/store/sqlite/tasks.go
    - internal/server/handlers.go
    - internal/server/dispatch.go
decisions:
  - loadTaskOutputs converted from standalone function to Server method to enable store access without passing store as parameter
  - File-based outputs.json fallback retained in loadTaskOutputs for backward compatibility with local-mode daemons
  - UpdateTaskInputs/UpdateTaskOutputs failures in handleTaskRun and makeDispatchFn are logged but non-fatal; daemon can still read inputs.json from shared FS
  - storage import removed from dispatch.go since loadTaskOutputs now uses s.backend via Server receiver
metrics:
  duration: "5 min"
  completed: "2026-03-13"
  tasks_completed: 2
  files_changed: 6
---

# Phase 17 Plan 01: API-Based Input/Output Delivery (Server Side) Summary

**One-liner:** Extended API contract with Inputs/WDLContent/Outputs fields, SQLite migration 00002, and server-side store-first output loading with file-based fallback.

## Tasks Completed

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | API types, migration, and store layer | 9c9633a | types.go, store.go, tasks.go, 00002 migration |
| 2 | Server handlers and dispatch | 06d63a2 | handlers.go, dispatch.go |

## What Was Built

### API Contract Extensions

`TaskClaimResponse` now carries:
- `Inputs map[string]interface{}` (json:"inputs,omitempty") — task inputs for daemon
- `WDLContent string` (json:"wdl_content,omitempty") — raw WDL file content

`TaskResultRequest` now carries:
- `Outputs map[string]interface{}` (json:"outputs,omitempty") — daemon-reported outputs

### Store Layer

`store.Task` struct extended with `InputsJSON string` and `OutputsJSON string`.

`Store` interface extended with:
- `UpdateTaskOutputs(ctx, taskID, outputsJSON) error`
- `UpdateTaskInputs(ctx, taskID, inputsJSON) error`

SQLite migration `00002_task_outputs.sql` adds `inputs_json TEXT` and `outputs_json TEXT` columns to tasks table. All four SELECT queries in `tasks.go` updated to include the new columns. `scanTask` scans them as `sql.NullString`.

### Server Handlers

**handleClaim**: Reads WDL file via `os.ReadFile(run.WDLPath)`, unmarshals task's InputsJSON (falling back to run.InputsJSON), and populates `Inputs` and `WDLContent` in the response.

**handleResult**: If `req.Outputs` is non-nil, marshals and persists via `UpdateTaskOutputs` before updating task state.

**handleTaskRun**: Calls `UpdateTaskInputs` after `CreateTask` to store inputs in DB.

### Dispatch

**makeDispatchFn**: Calls `UpdateTaskInputs` after `CreateTask`. File-based `inputs.json` write retained for local-mode backward compatibility.

**loadTaskOutputs**: Converted from standalone function to `(*Server).loadTaskOutputs(ctx, taskID, runID, taskName, shardIndex)`. Reads `t.OutputsJSON` from store first; falls back to file-based `outputs.json` download when OutputsJSON is empty (backward compat for local-mode daemon).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Removed unused storage import from dispatch.go**
- **Found during:** Task 2 compilation
- **Issue:** After converting `loadTaskOutputs` to a Server method, the `storage.Backend` parameter was removed, leaving the `storage` import unused
- **Fix:** Removed the import; `s.backend` is accessed via the Server receiver which already holds it
- **Files modified:** internal/server/dispatch.go
- **Commit:** 06d63a2

## Self-Check: PASSED

All created/modified files confirmed present. Both task commits (9c9633a, 06d63a2) confirmed in git log.
