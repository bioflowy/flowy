---
phase: 10-server-binary-dag-dispatch
verified: 2026-03-11T23:52:40Z
status: passed
score: 11/11 must-haves verified
re_verification: false
---

# Phase 10: Server Binary and DAG Dispatch Verification Report

**Phase Goal:** Server binary that accepts workflow submissions, dispatches tasks respecting DAG order, and exposes operational endpoints
**Verified:** 2026-03-11T23:52:40Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | POST /workflows with valid WDL path and inputs returns 201 with run_id, run exists in DB as PENDING | VERIFIED | `handleSubmit` in handlers.go:82-141; `TestSubmit_Valid` in server_test.go:307 |
| 2 | GET /workflows/{id}/status returns the current run state for a submitted workflow | VERIFIED | `handleStatus` in handlers.go:34-46; `TestStatus_Found` in server_test.go:97 |
| 3 | GET /workflows/{id}/outputs returns output key-value pairs after workflow reaches COMPLETED | VERIFIED | `handleOutputs` in handlers.go:48-79; `TestOutputs_Completed` in server_test.go:143 |
| 4 | POST /workflows/{id}/abort transitions a running workflow to ABORTED | VERIFIED | `handleAbort` in handlers.go:144-190; `TestAbort` in server_test.go:369 |
| 5 | GET /health returns 200 | VERIFIED | `handleHealth` in handlers.go:29-32; `TestHealth` in server_test.go:82 |
| 6 | DAG scheduler dispatches tasks to SQLite queue via dispatchFn instead of Docker | VERIFIED | `makeDispatchFn`/`runWorkflow` in dispatch.go; `TestDispatch_Integration` in server_test.go:424 |
| 7 | POST /tasks/claim returns a claimed task or 204 when none available | VERIFIED | `handleClaim` in handlers.go:192-224; `TestClaim_HasPending` and `TestClaim_NoPending` in server_test.go:578, 619 |
| 8 | POST /tasks/{id}/result transitions task to COMPLETED or FAILED | VERIFIED | `handleResult` in handlers.go:226-258; `TestResult_Completed`, `TestResult_Failed` in server_test.go:641, 677 |
| 9 | POST /heartbeat refreshes heartbeats for all tasks owned by a daemon | VERIFIED | `handleHeartbeat` in handlers.go:260-278; `TestHeartbeat` in server_test.go:735 |
| 10 | Background goroutine detects stale tasks and resets them to PENDING | VERIFIED | `StartStaleDetection` in server.go:96-122; `TestStaleDetection` in server_test.go:505 |
| 11 | flowy-server binary builds under cmd/flowy-server/ | VERIFIED | `go build -o /dev/null ./cmd/flowy-server/` succeeds; `cmd/flowy-server/main.go` (88 lines) |

**Score:** 11/11 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/server/server.go` | Server struct, New(), router wiring, ServeHTTP, stale detection goroutine | VERIFIED | 142 lines; exports `New`, `Server`, `StartStaleDetection`, `Shutdown`, `SetStaleInterval` |
| `internal/server/handlers.go` | All HTTP handler methods and JSON helpers | VERIFIED | 279 lines; contains `handleSubmit`, all 8 handlers implemented |
| `internal/server/dispatch.go` | makeDispatchFn, waitForTask, runWorkflow goroutine, loadTaskOutputs | VERIFIED | 202 lines; contains `makeDispatchFn`, `waitForTask`, `loadTaskOutputs`, `runWorkflow` |
| `internal/server/server_test.go` | Integration tests for all endpoints using httptest | VERIFIED | 779 lines (min 100 required); 21 test functions covering all endpoints |
| `cmd/flowy-server/main.go` | Binary entry point with config loading, store opening, graceful shutdown | VERIFIED | 88 lines; contains `signal.NotifyContext`, `server.New`, graceful shutdown sequence |
| `internal/store/store.go` | GetTaskByID and UpdateHeartbeatByDaemon in Store interface | VERIFIED | Both methods present at lines 88-95 |
| `internal/store/sqlite/tasks.go` | SQLiteStore implementations of GetTaskByID and UpdateHeartbeatByDaemon | VERIFIED | Both implementations present at lines 102-148 |
| `internal/store/rundir.go` | TaskDir path helper for per-task directories | VERIFIED | `TaskDir` and `CreateTaskDirs` at lines 24-36 |
| `wdl/workflow/workflow.go` | RunWorkflowWithDispatch exported function | VERIFIED | `TaskDispatchFn` type and `RunWorkflowWithDispatch` exported at lines 17-59 |
| `internal/api/types.go` | Corrected DTO type comments referencing /workflows | VERIFIED | All comments use `/workflows` (not `/runs`) |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/server/handlers.go` | `internal/store/store.go` | `s.store.` method calls | VERIFIED | 11 `s.store.*` calls covering GetRun, CreateRun, UpdateRunState, ClaimTask, UpdateTaskState, UpdateHeartbeatByDaemon, ListTasksByRun |
| `internal/server/dispatch.go` | `wdl/workflow/workflow.go` | `workflow.RunWorkflowWithDispatch` call | VERIFIED | dispatch.go:174 calls `workflow.RunWorkflowWithDispatch` |
| `internal/server/dispatch.go` | `internal/store/store.go` | `s.store.CreateTask` and `s.store.GetTaskByID` | VERIFIED | dispatch.go:52 (`CreateTask`) and dispatch.go:75 (`GetTaskByID`) |
| `cmd/flowy-server/main.go` | `internal/server/server.go` | `server.New()` constructor | VERIFIED | main.go:44 calls `server.New(cfg, st)` |
| `internal/server/server.go` | `internal/store/store.go` | Store interface (not concrete SQLiteStore) | VERIFIED | server.go:25 `store store.Store` — interface only; no `store/sqlite` import in server package |
| `internal/store/store.go` | `internal/store/sqlite/tasks.go` | Store interface implementation | VERIFIED | SQLiteStore implements `GetTaskByID` and `UpdateHeartbeatByDaemon` |
| `wdl/workflow/workflow.go` | `internal/server` | `RunWorkflowWithDispatch` exported function | VERIFIED | Called from dispatch.go:174 |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|---------|
| SRV-01 | 10-02 | POST /workflows endpoint for workflow submission | SATISFIED | `handleSubmit` + `TestSubmit_Valid` |
| SRV-02 | 10-02 | GET /workflows/{id}/status endpoint | SATISFIED | `handleStatus` + `TestStatus_Found` |
| SRV-03 | 10-02 | GET /workflows/{id}/outputs endpoint | SATISFIED | `handleOutputs` + `TestOutputs_Completed` |
| SRV-04 | 10-02 | POST /workflows/{id}/abort endpoint | SATISFIED | `handleAbort` + `TestAbort` |
| SRV-05 | 10-02 | GET /health endpoint | SATISFIED | `handleHealth` + `TestHealth` |
| SRV-06 | 10-01, 10-02 | DAG scheduler with runTaskFn injection to SQLite queue | SATISFIED | `RunWorkflowWithDispatch` + `makeDispatchFn` + `TestDispatch_Integration` |
| SRV-09 | 10-02 | flowy-server binary under cmd/flowy-server/ | SATISFIED | `cmd/flowy-server/main.go`; binary builds successfully |

**Note on SRV-07 and SRV-08:** REQUIREMENTS.md maps SRV-07 (SQLite WAL persistence) and SRV-08 (stale task detection) to Phase 9 and Phase 10 respectively. Neither plan's `requirements` field claims SRV-07 or SRV-08 for Phase 10. SRV-07 was addressed in Phase 9. SRV-08 (stale detection) is implemented in Phase 10 via `StartStaleDetection` (server.go:96-122) and tested in `TestStaleDetection` (server_test.go:505) — the implementation is complete even though the plan `requirements` field did not claim it. No orphaned requirement gap exists.

### Anti-Patterns Found

| File | Pattern | Severity | Assessment |
|------|---------|----------|------------|
| `internal/server/dispatch.go:143` | Comment noting complex WDL types deferred to Phase 11 | Info | Intentional — `jsonToValue` handles primitives; complex types (Array, Map) stringify as JSON. Documented limitation, not a blocker. |

No blocker or warning-level anti-patterns found. All handlers are fully implemented (no 501 stubs). No TODO/FIXME markers remain in server code.

### Human Verification Required

None. All phase 10 behaviors are verifiable programmatically:

- All 21 server integration tests pass (including dispatch end-to-end with SQLite)
- Race detector passes: `go test ./internal/server/... -race`
- Binary builds: `go build -o /dev/null ./cmd/flowy-server/`
- Full test suite passes: all 16 packages, 0 failures

## Test Results Summary

```
ok  github.com/bioflowy/flowy/internal/server       2.743s   (21 tests)
ok  github.com/bioflowy/flowy/internal/store        0.005s
ok  github.com/bioflowy/flowy/internal/store/sqlite 2.246s
ok  github.com/bioflowy/flowy/wdl/workflow          0.027s
Race detector: ok  github.com/bioflowy/flowy/internal/server  4.429s
Binary build: go build -o /dev/null ./cmd/flowy-server/  (exit 0)
Full suite: all 16 packages pass
```

## Goal Achievement Summary

The phase goal is fully achieved. The codebase now contains:

1. A buildable `flowy-server` binary (`cmd/flowy-server/main.go`) with signal handling and graceful shutdown.
2. A complete `internal/server` package with chi routing, all 8 HTTP endpoints (4 user-facing, 3 daemon-facing, 1 health), and JSON error responses on 404/405.
3. DAG dispatch via `makeDispatchFn`: creates SQLite task rows, writes `inputs.json`, polls `GetTaskByID` until daemon completion — replacing local Docker execution.
4. Stale detection goroutine resetting tasks with expired heartbeats to PENDING.
5. Foundation extensions (Plan 01): `GetTaskByID`, `UpdateHeartbeatByDaemon`, `TaskDir`, `RunWorkflowWithDispatch` — all tested and wired.
6. All 7 required SRV requirements (SRV-01 through SRV-06, SRV-09) satisfied with passing tests.

---

_Verified: 2026-03-11T23:52:40Z_
_Verifier: Claude (gsd-verifier)_
