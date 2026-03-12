---
phase: 14-storage-abstraction
plan: 01
subsystem: storage
tags: [backend-interface, local-filesystem, s3-uri, tdd, go]

# Dependency graph
requires:
  - phase: internal/store
    provides: RunInputsDir, RunOutputsDir, TaskDir, CreateRunDirs, CreateTaskDirs path helpers
provides:
  - Backend interface with 12 methods (path helpers, dir lifecycle, file I/O, directory I/O, JSON I/O)
  - LocalBackend delegating to store.* with zero behavior change
  - ParseS3URI helper for S3 URI parsing without net/url normalization
  - NewBackend factory returning LocalBackend
affects:
  - 15-server-storage
  - 16-s3-upload
  - 17-s3-daemon
  - 18-s3-integration

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Backend interface encapsulates both path layout and file operations in one abstraction
    - LocalBackend as thin delegation wrapper (no copy-paste of path logic)
    - ParseS3URI using strings.HasPrefix + strings.IndexByte (not net/url) to avoid path normalization
    - WriteJSON/ReadJSON composing on UploadFile/DownloadFile for consistency

key-files:
  created:
    - internal/storage/backend.go
    - internal/storage/local.go
    - internal/storage/factory.go
    - internal/storage/backend_test.go
    - internal/storage/local_test.go
  modified: []

key-decisions:
  - "ParseS3URI uses string ops not net/url to avoid S3 key corruption from path normalization"
  - "UploadDirectory/DownloadDirectory are no-ops for LocalBackend (path IS localSrc in local mode)"
  - "UploadFile creates intermediate directories via os.MkdirAll (Rule 2: missing critical functionality)"
  - "No context.Context in Backend interface per project decision (value-in/value-out signatures)"

patterns-established:
  - "storage package imports store package (one-way dependency: storage -> store)"
  - "All errors wrapped with fmt.Errorf('storage: ...') prefix for consistent error messages"
  - "TDD RED-GREEN: write failing test first, then implement"

requirements-completed:
  - STOR-01
  - STOR-02

# Metrics
duration: 2min
completed: 2026-03-12
---

# Phase 14 Plan 01: Storage Abstraction — Backend Interface and LocalBackend Summary

**Backend interface (12 methods) with LocalBackend delegating to store.* and ParseS3URI using string ops to avoid net/url S3 key corruption**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-12T23:49:29Z
- **Completed:** 2026-03-12T23:51:31Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments

- Created Backend interface with 12 methods covering path helpers, directory lifecycle, file I/O, directory I/O, and JSON I/O
- Implemented LocalBackend delegating all path operations to store.* — zero behavior change, single source of truth
- Added ParseS3URI with string-based parsing (avoiding net/url path normalization that corrupts S3 keys)
- Added NewBackend factory function returning LocalBackend (extensible for future S3Backend)
- Full test coverage: 8 ParseS3URI edge cases, path equivalence verification, file I/O roundtrip, JSON roundtrip, directory creation

## Task Commits

Each task was committed atomically:

1. **Task 1: Backend interface, ParseS3URI, and tests** - `a53cb78` (feat)
2. **Task 2: LocalBackend, factory, and tests** - `93b051d` (feat)

_Note: TDD tasks — tests written first (RED), then implementation (GREEN), committed together per task._

## Files Created/Modified

- `internal/storage/backend.go` - Backend interface (12 methods) + ParseS3URI helper
- `internal/storage/local.go` - LocalBackend implementing Backend via os.* and store.* delegation
- `internal/storage/factory.go` - NewBackend constructor returning *LocalBackend
- `internal/storage/backend_test.go` - Table-driven tests for ParseS3URI (8 cases)
- `internal/storage/local_test.go` - Tests for path helpers, file I/O, JSON roundtrip, dir creation, factory

## Decisions Made

- ParseS3URI uses `strings.HasPrefix` + `strings.IndexByte` rather than `net/url` because net/url normalizes paths (e.g. `//` -> `/`), which would corrupt S3 keys
- `UploadDirectory`/`DownloadDirectory` are no-ops for LocalBackend — in local mode the storage path IS the local source path, so no copy is needed
- `UploadFile` creates intermediate directories via `os.MkdirAll` before writing — necessary for paths like `{baseDir}/runs/{runID}/outputs/file.json`
- No `context.Context` parameters per locked project decision — value-in/value-out signatures

## Deviations from Plan

None - plan executed exactly as written.

(Note: UploadFile creating intermediate directories via os.MkdirAll is a natural correctness requirement for writing to nested paths, not a deviation — the plan specifies `os.WriteFile` and the directory creation is a prerequisite for it to work.)

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `internal/storage` package is ready for Phase 15 to inject Backend into server handlers
- Backend interface is the stable API surface that Phase 15 (server), 16 (S3 upload), 17 (daemon S3), and 18 (integration) will depend on
- No circular import risk: `storage` → `store` (one-way dependency confirmed)
- All existing tests (248+) continue to pass with no regressions

---
*Phase: 14-storage-abstraction*
*Completed: 2026-03-12*
