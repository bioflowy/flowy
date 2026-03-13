---
phase: quick-6
plan: 1
subsystem: store, server, workflow
tags: [s3-mode, outputs, db-storage, bug-fix]
dependency_graph:
  requires: []
  provides: [run-outputs-in-db, db-first-output-read]
  affects: [spectool-s3-tests, handleOutputs, runWorkflowInternal]
tech_stack:
  added: []
  patterns: [store-first-with-file-fallback, non-fatal-file-write]
key_files:
  created:
    - internal/store/sqlite/migrations/00003_run_outputs.sql
  modified:
    - internal/store/store.go
    - internal/store/sqlite/runs.go
    - internal/store/sqlite/sqlite_test.go
    - internal/server/dispatch.go
    - internal/server/handlers.go
    - wdl/workflow/workflow.go
    - wdl/workflow/scheduler_test.go
decisions:
  - DB-first output storage mirrors existing task output pattern (store-first with file fallback)
  - File write failure in waitForTaskAndFinalize is now non-fatal (logged only) to avoid FAILED state when S3 path is unavailable
  - runWorkflowInternal no longer writes any local files; caller (dispatch.go) handles persistence
metrics:
  duration: "8 min"
  completed_date: "2026-03-13"
  tasks_completed: 2
  files_changed: 7
---

# Quick Task 6: Fix S3 Mode spectool Failures — Save Workflow Outputs to DB

**One-liner:** DB-based run output storage with file fallback eliminates `outputs.json: no such file or directory` failures in S3 mode.

## Objective

Fix 93/96 spectool-s3 test failures caused by `runWorkflowInternal()` attempting to write
`outputs.json` and `error.json` to a local directory that does not exist in S3 mode.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Add OutputsJSON to Run struct, migration, and Store methods | 8710c75 | store.go, migrations/00003, runs.go, sqlite_test.go |
| 2 | Save run outputs to DB and remove local file writes from workflow engine | 785a66d | dispatch.go, handlers.go, workflow.go, scheduler_test.go |

## What Was Built

### Task 1: Run output persistence layer

- Added `OutputsJSON string` field to `store.Run` struct with comment
- Added `UpdateRunOutputs(ctx, id, outputsJSON string) error` to `Store` interface
- Created migration `00003_run_outputs.sql` adding `outputs_json TEXT` column to runs table
- Implemented `UpdateRunOutputs` in `SQLiteStore` following the `UpdateTaskOutputs` pattern
- Updated `scanRun` and `scanRunRows` to read `outputs_json` as `sql.NullString`
- Updated SQL queries in `GetRun` and `GetRunByPrefix` to SELECT 7 columns
- Added `TestUpdateRunOutputs` test and updated `TestCreateAndGetRun` to verify empty initial state

### Task 2: Wire up DB storage and remove local file writes

- Removed `writeOutputsJSON` and `writeErrorJSON` calls from `runWorkflowInternal`; function now just returns outputs/error to caller
- Added `UpdateRunOutputs` call in `runWorkflow()` before the file write (DB-first)
- Added `UpdateRunOutputs` call in `waitForTaskAndFinalize()` with non-fatal file write
- Changed `waitForTaskAndFinalize` file write failure from fatal (set FAILED state) to non-fatal (log only)
- Added DB-first path in `handleOutputs`: reads `run.OutputsJSON` from DB if set, falls back to file for older runs
- Updated scheduler tests: removed file-existence assertions, verify returned outputs map instead
- Renamed `TestRunWorkflow_OutputsJSON` to `TestRunWorkflow_OutputsReturned`
- Renamed `TestRunWorkflow_FailFast_ErrorJSON` to `TestRunWorkflow_FailFast`

## Decisions Made

1. **DB-first with file fallback pattern** — mirrors the existing task output approach (Phase 17 decision). Ensures backward compatibility with local-mode runs while fixing S3 mode.

2. **Non-fatal file write in waitForTaskAndFinalize** — previously, a failed `UploadFile` would set the run to FAILED. In S3 mode where the local path doesn't exist, this caused runs to be marked FAILED even after successful task completion. Now the file write is best-effort.

3. **Remove all local file writes from workflow engine** — `runWorkflowInternal` is a pure computation function; it should return outputs to the caller, not decide where to persist them. The caller (`dispatch.go`) is the right place for persistence decisions.

## Deviations from Plan

None — plan executed exactly as written.

## Verification

- `go test ./... -count=1` passes (20/20 packages)
- `go build ./...` succeeds
- The specific error pattern `writing outputs.json: open .../runs/.../outputs.json: no such file or directory` is eliminated

## Self-Check: PASSED

- migration file: `/home/uehara/flowy/internal/store/sqlite/migrations/00003_run_outputs.sql` — FOUND
- commit 8710c75 — FOUND
- commit 785a66d — FOUND
