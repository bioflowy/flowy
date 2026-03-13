---
phase: 14-storage-abstraction
plan: 02
subsystem: storage
tags: [go, storage, backend, refactor, abstraction]

# Dependency graph
requires:
  - phase: 14-01
    provides: Backend interface and LocalBackend implementation in internal/storage
provides:
  - Server struct with backend storage.Backend field
  - server.New() accepting Backend as third parameter
  - All server-side file I/O routed through Backend (dispatch.go, handlers.go)
  - cmd/flowy-server wired with storage.NewBackend(cfg) factory
affects:
  - 15-daemon-s3
  - 16-s3-backend
  - 17-daemon-refactor

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Backend dependency injection: Server accepts storage.Backend at construction time"
    - "All direct os.ReadFile/WriteFile replaced with backend.DownloadFile/UploadFile"
    - "All store path helpers replaced with backend.TaskDir/CreateRunDirs/CreateTaskDirs"

key-files:
  created: []
  modified:
    - internal/server/server.go
    - internal/server/dispatch.go
    - internal/server/handlers.go
    - internal/server/server_test.go
    - cmd/flowy-server/main.go

key-decisions:
  - "store import kept in dispatch.go and handlers.go — still needed for store.Task/Run types and state constants"
  - "filepath.Join retained for sub-path construction within task dirs (Backend handles I/O, not sub-path logic)"
  - "daemon.go left unmodified (Phase 17 scope per plan)"

patterns-established:
  - "Server.backend field: single Backend instance injected at New() replaces all ad-hoc os/store calls"
  - "loadTaskOutputs signature updated to accept Backend as first parameter for testability"

requirements-completed:
  - STOR-02
  - STOR-03

# Metrics
duration: 3min
completed: 2026-03-12
---

# Phase 14 Plan 02: Server Backend Injection Summary

**Backend dependency injection into Server struct: all os.ReadFile/WriteFile and store path helpers in dispatch.go and handlers.go replaced with s.backend.* calls, making server ready for S3 in Phase 16 with zero code changes**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-12T23:53:54Z
- **Completed:** 2026-03-12T23:57:00Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments

- Added `backend storage.Backend` field to `Server` struct and updated `New()` to accept it as third parameter
- Replaced all direct `os.ReadFile`/`os.WriteFile` calls in dispatch.go and handlers.go with `s.backend.DownloadFile`/`s.backend.UploadFile`
- Replaced all `store.CreateRunDirs`, `store.CreateTaskDirs`, `store.TaskDir` calls in dispatch.go and handlers.go with `s.backend.*` equivalents
- Updated `loadTaskOutputs` to receive Backend as first parameter
- Wired `storage.NewBackend(cfg)` in `cmd/flowy-server/main.go`
- All 248+ existing tests pass unchanged after refactor

## Task Commits

Each task was committed atomically:

1. **Task 1: Inject Backend into Server and update all call sites** - `4fbd184` (feat)
2. **Task 2: Update server tests and verify full suite passes** - `bde5e03` (feat)

## Files Created/Modified

- `internal/server/server.go` - Added `backend storage.Backend` field; updated `New()` signature
- `internal/server/dispatch.go` - Replaced os/store calls with s.backend.*; updated loadTaskOutputs signature
- `internal/server/handlers.go` - Replaced os/store calls with s.backend.*; removed `os` import
- `internal/server/server_test.go` - Added storage import; pass backend to server.New()
- `cmd/flowy-server/main.go` - Added storage import; create backend via factory before server.New()

## Decisions Made

- Kept `store` import in both dispatch.go and handlers.go — it is still required for `store.Task`, `store.Run`, `store.TaskCompleted`, and other types/constants.
- Kept `filepath.Join` for constructing sub-paths within task directories (e.g., `filepath.Join(taskDir, "inputs.json")`) — Backend manages file I/O, not sub-path construction.
- Did not modify `internal/daemon/daemon.go` — that is Phase 17 scope.

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None.

## Next Phase Readiness

- Server package is fully abstracted behind Backend interface
- Phase 16 can implement S3Backend without touching dispatch.go or handlers.go
- All 248+ tests pass; server binary builds cleanly

---
*Phase: 14-storage-abstraction*
*Completed: 2026-03-12*
