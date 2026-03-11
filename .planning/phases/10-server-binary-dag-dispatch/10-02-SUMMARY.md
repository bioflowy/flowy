---
phase: 10-server-binary-dag-dispatch
plan: 02
subsystem: server
tags: [server, http, dispatch, dag, sqlite, tdd, chi]

# Dependency graph
requires:
  - phase: 10-server-binary-dag-dispatch
    plan: 01
    provides: Store interface (GetTaskByID, UpdateHeartbeatByDaemon), TaskDir helpers, RunWorkflowWithDispatch

provides:
  - internal/server package: Server struct, chi router, all HTTP handlers
  - makeDispatchFn: SQLite task enqueue + inputs.json write + polling
  - waitForTask: 2-second poll loop with context cancellation
  - loadTaskOutputs: outputs.json reader with JSON-to-WDL value conversion
  - StartStaleDetection: background goroutine resetting stale tasks to PENDING
  - cmd/flowy-server/main.go: buildable binary with graceful shutdown

affects: [11-daemon-binary]

# Tech tracking
tech-stack:
  added:
    - "chi v5 (go-chi/chi/v5): HTTP routing with middleware.Logger, middleware.Recoverer"
    - "google/uuid: run ID generation"
  patterns:
    - "Separate run context from request context (context.Background() for goroutines)"
    - "SetStaleInterval() for test-friendly ticker configuration"
    - "TDD RED-GREEN pattern: failing tests first, implementation second"

key-files:
  created:
    - internal/server/server.go
    - internal/server/handlers.go
    - internal/server/dispatch.go
    - internal/server/server_test.go
    - cmd/flowy-server/main.go

key-decisions:
  - "TestSubmit_Valid uses abort to unblock run goroutine before Shutdown() — avoids indefinite wait"
  - "SetStaleInterval() added to Server for test-controllable ticker (500ms in tests, 30s in production)"
  - "outputs.json uses plain output names (without task prefix) — scheduler wraps in StructValue keyed by call alias"
  - "handleAbort checks s.runs map first; if run exists in store but not map it's already completed (404)"

patterns-established:
  - "Run goroutine lifecycle: PENDING -> RUNNING (goroutine start) -> COMPLETED/FAILED/ABORTED"
  - "Dispatch model: dispatchFn creates SQLite task row, writes inputs.json, polls GetTaskByID until terminal"
  - "JSON error responses on all routes including chi default 404/405 via NotFound/MethodNotAllowed overrides"

requirements-completed: [SRV-01, SRV-02, SRV-03, SRV-04, SRV-05, SRV-06, SRV-09]

# Metrics
duration: 8min
completed: 2026-03-11
---

# Phase 10 Plan 02: Server Package and Binary Summary

**Complete server package with chi router, all HTTP handlers, DAG dispatch via SQLite enqueue, stale detection goroutine, and buildable flowy-server binary**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-03-11T23:39:36Z
- **Completed:** 2026-03-11T23:47:44Z
- **Tasks:** 3
- **Files created:** 5

## Accomplishments

- Full `internal/server` package: Server struct, chi router with JSON 404/405 overrides, all 8 HTTP endpoints
- `makeDispatchFn` implements the SQLite enqueue pattern: creates Task row, writes inputs.json, polls via GetTaskByID
- `waitForTask` polls every 2 seconds with context cancellation support (abort propagation)
- `loadTaskOutputs` reads outputs.json from task directory and converts JSON primitives to WDL values
- `StartStaleDetection` background goroutine resets stale tasks to PENDING every 30s (configurable for tests)
- `cmd/flowy-server/main.go` binary with -config flag, signal.NotifyContext for graceful shutdown
- 21 tests pass including end-to-end dispatch integration test (claim task → write outputs → verify COMPLETED)
- Race detector clean: `go test ./internal/server/... -race` passes

## Task Commits

Each task was committed atomically:

1. **Task 1: Server struct, router, JSON helpers, read-only handlers** - `830cfea` (test RED) + `5171b3d` (feat GREEN)
2. **Task 2: Submit, dispatch, abort, stale detection** - `5171b3d` (feat)
3. **Task 3: Daemon endpoints and binary entry point** - `7ff5359` (feat)

## Files Created/Modified

- `internal/server/server.go` - Server struct, New(), newRouter(), ServeHTTP, StartStaleDetection, Shutdown, SetStaleInterval
- `internal/server/handlers.go` - All 8 HTTP handlers: handleHealth, handleStatus, handleOutputs, handleSubmit, handleAbort, handleClaim, handleResult, handleHeartbeat
- `internal/server/dispatch.go` - makeDispatchFn, waitForTask, loadTaskOutputs, jsonToValue, runWorkflow
- `internal/server/server_test.go` - 21 integration tests using in-memory SQLiteStore and httptest
- `cmd/flowy-server/main.go` - Binary entry point with graceful shutdown

## Decisions Made

- `SetStaleInterval()` exported on Server for test-controllable ticker: avoids 30-second waits in tests while keeping production at 30s
- `handleSubmit` uses `context.WithCancel(context.Background())` for run goroutines, NOT `r.Context()` — ensures goroutine outlives HTTP request
- `handleAbort` checks `s.runs` map first; if run is in store but not in map it's already completed, returns 404 "run not found or already completed"
- `outputs.json` task format uses plain output names (e.g., `"out"`) without task prefix — scheduler's `runCall` wraps them in StructValue keyed by call alias
- `TestSubmit_Valid` calls abort before `srv.Shutdown()` to prevent indefinite wait on blocked dispatch goroutine

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed TestSubmit_Valid assertion and shutdown deadlock**
- **Found during:** Task 2 execution
- **Issue:** Test asserted `RunPending` state but goroutine races to `RunRunning` immediately; `srv.Shutdown()` deadlocked because workflow goroutine blocked in `waitForTask`
- **Fix:** Changed assertion to accept `RunPending` OR `RunRunning`; added abort before Shutdown() to unblock goroutine
- **Files modified:** `internal/server/server_test.go`
- **Not a separate commit:** Fixed inline during GREEN phase

**2. [Rule 1 - Bug] Fixed TestDispatch_Integration outputs.json key format**
- **Found during:** Task 2 execution
- **Issue:** Test wrote `{"hello.out": "Hello, world!"}` but scheduler expects plain output names `{"out": "Hello, world!"}` — the StructValue wrapping happens in scheduler.runCall, not in dispatchFn
- **Fix:** Changed outputs.json key from `"hello.out"` to `"out"` in test
- **Files modified:** `internal/server/server_test.go`

**3. [Rule 2 - Missing functionality] Added SetStaleInterval() to Server**
- **Found during:** Task 2 execution (TestStaleDetection)
- **Issue:** 30-second ticker made the stale detection test require 30+ seconds to run
- **Fix:** Added `staleInterval time.Duration` field to Server, defaults to 30s, `SetStaleInterval()` allows tests to use 500ms
- **Files modified:** `internal/server/server.go`

## Self-Check: PASSED

All files present:
- FOUND: internal/server/server.go
- FOUND: internal/server/handlers.go
- FOUND: internal/server/dispatch.go
- FOUND: internal/server/server_test.go
- FOUND: cmd/flowy-server/main.go

All commits present:
- FOUND: 830cfea (test RED)
- FOUND: 5171b3d (feat submit/dispatch/abort/stale)
- FOUND: 7ff5359 (feat daemon endpoints + binary)
