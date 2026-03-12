---
phase: 11-daemon-binary-task-execution
plan: 01
subsystem: daemon
tags: [daemon, docker, polling, heartbeat, graceful-shutdown, wdl, task-execution]

# Dependency graph
requires:
  - phase: 10-server-binary-dag-dispatch
    provides: POST /tasks/claim, POST /heartbeat, POST /tasks/{id}/result endpoints and store.GetRun
  - phase: 09-api-contract-persistence-foundation
    provides: api.TaskClaimResponse, api.HeartbeatRequest, api.TaskResultRequest types
provides:
  - WDLPath field in api.TaskClaimResponse (server populates from store.GetRun)
  - Workers field in config.DaemonConfig with default 1
  - internal/daemon package with New/Daemon/Run/Shutdown and full poll-execute-report cycle
affects: [12-client-binary-flowy-run, cmd/flowy-daemon]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Graceful shutdown: poll context cancelled on SIGTERM, task execution uses context.Background()"
    - "valuesToJSON converts WDL values.Value to JSON-compatible interface{} via type switch"
    - "Worker goroutines track in-flight tasks via shared sync.WaitGroup"

key-files:
  created:
    - internal/daemon/daemon.go
  modified:
    - internal/api/types.go
    - internal/config/config.go
    - internal/server/handlers.go

key-decisions:
  - "daemon_id format is hostname-PID (passed in from main.go, not generated inside daemon package)"
  - "Poll interval is 5-second short poll ticker (not long-poll HTTP) for simplicity"
  - "No hard shutdown timeout: wg.Wait() until Docker finishes; stale detection handles stuck daemons"
  - "handleClaim calls s.store.GetRun to get WDLPath; failure is non-fatal (warns, sets empty string)"
  - "executeTask uses context.Background() not signal ctx, protecting Docker from SIGTERM cancellation"

patterns-established:
  - "Daemon worker pattern: workerLoop ticks, calls claim, spawns executeTask with background context"
  - "Fire-and-forget reportResult: errors logged but not returned; server stale detection is safety net"

requirements-completed: [DMN-01, DMN-02, DMN-03, DMN-04, DMN-05]

# Metrics
duration: 11min
completed: 2026-03-12
---

# Phase 11 Plan 01: Daemon Server Prerequisites and Core Worker Implementation Summary

**Daemon worker package implementing poll-claim-execute-report-heartbeat lifecycle with Docker execution and graceful SIGTERM shutdown via split context**

## Performance

- **Duration:** ~11 min
- **Started:** 2026-03-12T00:15:27Z
- **Completed:** 2026-03-12T00:26:03Z
- **Tasks:** 2
- **Files modified:** 4 (3 modified, 1 created)

## Accomplishments
- Added `WDLPath` field to `api.TaskClaimResponse` and updated `handleClaim` to populate it from `store.GetRun`
- Added `Workers` field to `config.DaemonConfig` with default 1 in `applyDefaults`
- Created `internal/daemon/daemon.go` with full daemon implementation: `New`, `Run`, `Shutdown`, `workerLoop`, `claimTask`, `executeTask`, `sendHeartbeat`, `startHeartbeat`, `reportResult`, `valuesToJSON`
- Graceful shutdown design: poll goroutines use signal context (stop on SIGTERM), Docker execution uses `context.Background()` (completes naturally)

## Task Commits

Each task was committed atomically:

1. **Task 1: Add WDLPath to TaskClaimResponse, Workers to DaemonConfig, update handleClaim** - `7da9614` (feat)
2. **Task 2: Create internal/daemon package with full daemon implementation** - `3785264` (feat)

**Plan metadata:** (docs commit to follow)

## Files Created/Modified
- `internal/api/types.go` - Added `WDLPath string json:"wdl_path"` field to `TaskClaimResponse`
- `internal/config/config.go` - Added `Workers int yaml:"workers"` to `DaemonConfig` with default 1
- `internal/server/handlers.go` - `handleClaim` now calls `s.store.GetRun` to populate `WDLPath` in response
- `internal/daemon/daemon.go` - Full daemon worker implementation (364 lines)

## Decisions Made
- `daemon_id` format is `hostname-PID`, passed from `main.go` (not UUID per user decision)
- Short poll with 5-second ticker chosen over long-poll for simplicity
- No hard shutdown timeout: `wg.Wait()` blocks until Docker completes; server's stale detection handles stuck daemons
- `handleClaim` failure to get run WDLPath is non-fatal: logs warning, sets `WDLPath=""`, daemon will fail on WDL parse with clear error

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- `internal/daemon` package is complete and compiles
- Ready for Phase 12: `cmd/flowy-daemon/main.go` can use `daemon.New(cfg, daemonID).Run(ctx)` with OS signal handling
- All existing server tests pass (5 packages, 0 failures)

## Self-Check: PASSED

- internal/daemon/daemon.go: FOUND
- internal/api/types.go: FOUND
- internal/config/config.go: FOUND
- internal/server/handlers.go: FOUND
- 11-01-SUMMARY.md: FOUND
- Commit 7da9614: FOUND
- Commit 3785264: FOUND

---
*Phase: 11-daemon-binary-task-execution*
*Completed: 2026-03-12*
