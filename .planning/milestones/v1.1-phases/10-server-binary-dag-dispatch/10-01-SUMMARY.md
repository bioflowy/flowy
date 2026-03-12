---
phase: 10-server-binary-dag-dispatch
plan: 01
subsystem: database
tags: [store, sqlite, workflow, dag, dispatch, tdd]

# Dependency graph
requires:
  - phase: 09-api-contract-persistence-foundation
    provides: Store interface, SQLiteStore, rundir helpers, api types, wdl/workflow package

provides:
  - GetTaskByID(ctx, taskID int64) in Store interface and SQLiteStore
  - UpdateHeartbeatByDaemon(ctx, daemonID string) in Store interface and SQLiteStore
  - TaskDir() and CreateTaskDirs() per-task directory helpers in internal/store/rundir.go
  - TaskDispatchFn type and RunWorkflowWithDispatch() exported from wdl/workflow
  - Corrected api/types.go comments referencing /workflows instead of /runs

affects: [10-02-server-package, 11-daemon-binary]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Daemon heartbeat via single UPDATE per interval (UpdateHeartbeatByDaemon)"
    - "Workflow dispatch injection via TaskDispatchFn function type"
    - "Shared internal helper (runWorkflowInternal) serving two public entry points"

key-files:
  created:
    - wdl/workflow/workflow_dispatch_test.go
  modified:
    - internal/store/store.go
    - internal/store/sqlite/tasks.go
    - internal/store/sqlite/sqlite_test.go
    - internal/store/rundir.go
    - internal/store/rundir_test.go
    - wdl/workflow/workflow.go
    - internal/api/types.go

key-decisions:
  - "UpdateHeartbeatByDaemon uses single UPDATE WHERE daemon_id=? AND state IN ('CLAIMED','RUNNING') — one call per heartbeat interval regardless of task count"
  - "RunWorkflowWithDispatch delegates to runWorkflowInternal with dispatchFn set as scheduler.taskRunner — nil means use default runTaskFn"
  - "TaskDispatchFn matches the internal taskRunner signature — server passes its SQLite enqueue function directly"

patterns-established:
  - "Dual-entry-point pattern: RunWorkflow and RunWorkflowWithDispatch share runWorkflowInternal to avoid code duplication"
  - "TaskDir format: {baseDir}/runs/{runID}/tasks/{taskName}_{shardIndex}"

requirements-completed: [SRV-06]

# Metrics
duration: 4min
completed: 2026-03-11
---

# Phase 10 Plan 01: Foundation Extensions for Server Package Summary

**Store interface extended with GetTaskByID and UpdateHeartbeatByDaemon; TaskDir helpers and RunWorkflowWithDispatch added as server integration seams**

## Performance

- **Duration:** ~4 min
- **Started:** 2026-03-11T23:33:10Z
- **Completed:** 2026-03-11T23:37:04Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments
- Added GetTaskByID and UpdateHeartbeatByDaemon to Store interface + SQLiteStore with full TDD coverage
- Added TaskDir/CreateTaskDirs helpers for per-task working directories
- Exported RunWorkflowWithDispatch as the extension seam for server-side task enqueuing
- Fixed api/types.go comments from /runs to /workflows

## Task Commits

Each task was committed atomically:

1. **Task 1: Add GetTaskByID and UpdateHeartbeatByDaemon to Store interface and SQLiteStore** - `9af7921` (feat)
2. **Task 2: Add per-task directory helpers, RunWorkflowWithDispatch, and fix api comments** - `7daeb70` (feat)

## Files Created/Modified
- `internal/store/store.go` - Added GetTaskByID and UpdateHeartbeatByDaemon to Store interface
- `internal/store/sqlite/tasks.go` - Implemented GetTaskByID and UpdateHeartbeatByDaemon for SQLiteStore
- `internal/store/sqlite/sqlite_test.go` - Added TestGetTaskByID and TestUpdateHeartbeatByDaemon
- `internal/store/rundir.go` - Added TaskDir() and CreateTaskDirs() helpers
- `internal/store/rundir_test.go` - Added TestTaskDir and TestCreateTaskDirs
- `wdl/workflow/workflow.go` - Added TaskDispatchFn type, RunWorkflowWithDispatch, and runWorkflowInternal refactor
- `wdl/workflow/workflow_dispatch_test.go` - Created: dispatch tests with mock function and context cancel
- `internal/api/types.go` - Fixed all route comments from /runs to /workflows

## Decisions Made
- UpdateHeartbeatByDaemon uses a single `UPDATE WHERE daemon_id=? AND state IN ('CLAIMED','RUNNING')` — matches the CONTEXT.md design principle of "single call per interval regardless of task count"
- RunWorkflowWithDispatch sets `scheduler.taskRunner = dispatchFn` — nil dispatchFn causes scheduler.runCall to fall back to the package-level `runTaskFn`, preserving backward compatibility
- Extracted `runWorkflowInternal` as a private helper — both RunWorkflow and RunWorkflowWithDispatch call it, keeping logic DRY

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None — the existing scanTask helper in sqlite/tasks.go was reused directly for GetTaskByID, and the scheduler.taskRunner field already existed in the struct so injection was straightforward.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- All interfaces Plan 02 (server package) needs are now in place
- Store interface has GetTaskByID, UpdateHeartbeatByDaemon; SQLiteStore implements both
- RunWorkflowWithDispatch is exported and ready for the server to inject its task enqueue function
- TaskDir provides the per-task directory path formula the server needs for TaskClaimResponse

---
*Phase: 10-server-binary-dag-dispatch*
*Completed: 2026-03-11*
