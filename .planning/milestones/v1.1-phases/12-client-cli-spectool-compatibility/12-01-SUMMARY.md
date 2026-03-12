---
phase: 12-client-cli-spectool-compatibility
plan: 01
subsystem: api
tags: [http-client, server-handlers, prefix-matching, task-run, wdl]

# Dependency graph
requires:
  - phase: 11-daemon-binary-task-execution
    provides: Server handlers, store interface, dispatch goroutine, daemon HTTP pattern
  - phase: 10-server-binary-dag-dispatch
    provides: runWorkflow, makeDispatchFn, route registration pattern
  - phase: 09-api-contract-persistence-foundation
    provides: api/types.go leaf package, store.Store interface

provides:
  - api.SubmitRequest with RunID and SearchPaths fields
  - api.TaskRunRequest and api.TaskRunResponse DTOs
  - config.Default() exported constructor for config-file-free operation
  - Prefix matching in handleStatus and handleOutputs via GetRunByPrefix fallback
  - POST /tasks/run endpoint wrapping named task in synthetic workflow
  - internal/client package with Submit, Status, Outputs, TaskRun HTTP methods

affects: [12-02, 12-03, 12-04, 12-05]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Client HTTP pattern: json.Marshal -> NewRequestWithContext -> Do -> decode (mirrors daemon pattern)"
    - "Prefix matching: GetRun first, GetRunByPrefix fallback, 404 if both fail"
    - "Synthetic workflow: ast.Workflow wrapping single ast.Call for task-run dispatch"
    - "Input prefixing: taskName.inputName format for synthetic workflow inputs"

key-files:
  created:
    - internal/client/client.go
    - internal/client/client_test.go
  modified:
    - internal/api/types.go
    - internal/config/config.go
    - internal/server/handlers.go
    - internal/server/server.go

key-decisions:
  - "TaskRunRequest and TaskRunResponse DTOs added in Task 2 RED phase (needed for client_test.go compilation) rather than Task 3, resolving natural ordering dependency"
  - "handleOutputs uses run.ID (resolved full UUID) not raw runID parameter for outputsPath — ensures correct path when prefix was given"
  - "handleTaskRun prefixes task inputs with callAlias (taskName.inputKey) before passing to runWorkflow — matches dispatch mechanism expected format"
  - "Synthetic workflow Body uses ast.WorkflowNode interface — ast.Call implements isWorkflowNode() directly"

patterns-established:
  - "Client package: New(serverURL) -> Client{serverURL, httpClient{30s timeout}} — same shape as Daemon"
  - "Error propagation: decode api.ErrorResponse on non-success, wrap in descriptive fmt.Errorf"
  - "409 Conflict from /outputs: special-cased to 'workflow not yet completed (state: ...)' message"

requirements-completed: [CLI-05, CLI-06]

# Metrics
duration: 3min
completed: 2026-03-12
---

# Phase 12 Plan 01: Server Prerequisites and Client HTTP Package Summary

**HTTP client package for CLI-to-server transport, prefix run ID lookup, client-provided RunID/SearchPaths in SubmitRequest, and POST /tasks/run endpoint with synthetic workflow dispatch**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-12T02:34:45Z
- **Completed:** 2026-03-12T02:37:50Z
- **Tasks:** 3
- **Files modified:** 6

## Accomplishments

- Extended `api.SubmitRequest` with `RunID` and `SearchPaths` fields enabling pre-submission file staging and import resolution from CLI flags
- Added `config.Default()` so CLI binaries work without a config file using sensible defaults (BaseDir=./flowy-data, ServerURL=http://localhost:8124)
- Implemented prefix matching in `handleStatus` and `handleOutputs` via `GetRunByPrefix` fallback, enabling short run ID prefixes in CLI queries
- Added `POST /tasks/run` endpoint that wraps a named WDL task in a synthetic workflow and dispatches it through the existing `runWorkflow` mechanism
- Created `internal/client` package with `Submit`, `Status`, `Outputs`, `TaskRun` methods following the established daemon HTTP pattern
- All 10 client tests and full test suite (19 packages) pass

## Task Commits

Each task was committed atomically:

1. **Task 1: Server-side prerequisites** - `a01b782` (feat)
2. **Task 2: Client HTTP package RED** - `a1369d9` (test) — failing tests + stub
3. **Task 2: Client HTTP package GREEN** - `db6f8dc` (feat) — full implementation
4. **Task 3: POST /tasks/run endpoint** - `82f4990` (feat)

_Note: TDD task (Task 2) has two commits: test (RED) then feat (GREEN)_

## Files Created/Modified

- `internal/api/types.go` - Added `RunID`, `SearchPaths` to `SubmitRequest`; added `TaskRunRequest`, `TaskRunResponse` DTOs
- `internal/config/config.go` - Added `Default()` exported constructor
- `internal/server/handlers.go` - Prefix matching in `handleStatus`/`handleOutputs`; updated `handleSubmit` for RunID/SearchPaths; new `handleTaskRun` handler
- `internal/server/server.go` - Registered `POST /tasks/run` route
- `internal/client/client.go` - New HTTP client package with Submit, Status, Outputs, TaskRun methods
- `internal/client/client_test.go` - 10 table-driven tests covering success and error paths

## Decisions Made

- `TaskRunRequest` and `TaskRunResponse` DTOs were added during Task 2 RED phase (not Task 3) because `client_test.go` compilation requires them. Task 3 skipped the DTO addition step since it was already done.
- `handleOutputs` uses `run.ID` (the resolved full UUID from the store) for the `outputsPath` and response, not the raw URL parameter — ensures correctness when a prefix was provided.
- `handleTaskRun` prefixes task inputs as `taskName.inputName` before passing to `runWorkflow` because the dispatch mechanism expects `callAlias.inputName` format when resolving inputs for a call.
- `ast.Call.Callee` is `[]string` (dot-separated task name), so `&ast.Call{Callee: []string{req.TaskName}, Alias: req.TaskName}` is the correct constructor form.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Ordering] TaskRunRequest/TaskRunResponse added in Task 2 RED phase**
- **Found during:** Task 2 (TDD RED — writing client_test.go)
- **Issue:** `client_test.go` imports `api.TaskRunRequest` and `api.TaskRunResponse`, which are only specified in Task 3. Tests would not compile without them.
- **Fix:** Added both DTOs to `internal/api/types.go` during the RED commit so tests compile and fail at runtime (panic: not implemented), not at compile time.
- **Files modified:** `internal/api/types.go`
- **Verification:** Tests compiled and panicked correctly; all fields match Task 3 spec exactly.
- **Committed in:** `a1369d9` (Task 2 RED commit)

---

**Total deviations:** 1 auto-fixed (ordering/dependency)
**Impact on plan:** Zero scope creep — the DTOs are exactly as specified in Task 3. Only the commit ordering changed.

## Issues Encountered

None — plan executed as specified aside from the ordering deviation above.

## User Setup Required

None - no external service configuration required.

## Self-Check: PASSED

All created files exist on disk. All task commits verified in git log.

## Next Phase Readiness

- All server-side prerequisites for CLI subcommands are in place
- `internal/client.Client` is ready for use by `flowy submit`, `flowy status`, `flowy outputs`, and `flowy run --task`
- `config.Default()` enables CLI binaries to work without a config file
- Ready to implement CLI subcommands in plans 02-05

---
*Phase: 12-client-cli-spectool-compatibility*
*Completed: 2026-03-12*
