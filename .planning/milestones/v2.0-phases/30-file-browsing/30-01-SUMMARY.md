---
phase: 30-file-browsing
plan: 01
subsystem: api
tags: [go, sqlite, chi, http-handlers, file-download, project-runs]

# Dependency graph
requires:
  - phase: 28-project-management
    provides: project store, project membership, projectRoleRequired middleware
  - phase: 26-authentication
    provides: JWT auth, Store interface, server infrastructure
provides:
  - GET /projects/{slug}/runs endpoint returning RunListItem JSON array
  - GET /projects/{slug}/runs/{id}/files?path=... file download/preview endpoint
  - ListRunsByProject store method (SQLite implementation)
  - RunListItem API response type
affects: [30-02-file-browsing-frontend]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "ListRunsByProject uses ORDER BY created_at DESC for newest-first ordering"
    - "File download handler uses raw path parameter passed directly to storage backend"
    - "Preview mode uses DownloadFile (buffered) capped at 100KB; download mode uses DownloadFileStream"
    - "Path traversal protection: strings.Contains(filePath, '..') rejected with 400"

key-files:
  created:
    - internal/server/handlers_runs_test.go
  modified:
    - internal/api/types.go
    - internal/store/store.go
    - internal/store/sqlite/runs.go
    - internal/store/sqlite/runs_test.go
    - internal/server/handlers.go
    - internal/server/server.go

key-decisions:
  - "File download handler uses the raw ?path= query param directly as the storage path — no path building from runID/baseDir"
  - "Preview mode uses DownloadFile (full read into memory, then cap) rather than streaming partial read"
  - "Empty run list returns [] (not null) via explicit nil-to-empty-slice guard before writeJSON"

patterns-established:
  - "TDD: write failing tests first in runs_test.go / handlers_runs_test.go, then implement to pass"
  - "Project foreign key requires CreateProject before CreateRun in tests"
  - "createRunsTestProject helper follows registry test pattern (createRegistryTestProject)"

requirements-completed: [FILE-01, FILE-03, FILE-04]

# Metrics
duration: 4min
completed: 2026-04-07
---

# Phase 30 Plan 01: Backend Endpoints for File Browsing Summary

**ListRunsByProject store method + GET /projects/{slug}/runs and GET /projects/{slug}/runs/{id}/files HTTP handlers with path traversal protection and 100KB preview cap**

## Performance

- **Duration:** 4 min
- **Started:** 2026-04-07T03:02:56Z
- **Completed:** 2026-04-07T03:07:00Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments
- Added `RunListItem` API type and `ListRunsByProject` store interface method + SQLite implementation with `ORDER BY created_at DESC`
- Added `handleListRuns` returning JSON array of RunListItem with optional `?status=` filter and guaranteed non-null empty array
- Added `handleFileDownload` streaming files via LocalBackend with Content-Disposition attachment header; preview mode returns text/plain capped at 100KB; path traversal via `..` rejected with 400

## Task Commits

Each task was committed atomically:

1. **Task 1: Add ListRunsByProject store method and RunListItem API type** - `90c8279` (feat)
2. **Task 2: Add handleListRuns and handleFileDownload HTTP handlers** - `5fd2e05` (feat)

**Plan metadata:** (pending)

_Note: TDD tasks had test→feat commits combined per task_

## Files Created/Modified
- `internal/api/types.go` - Added RunListItem struct with ID, State, RegistryRef, WDLPath, CreatedAt, UpdatedAt
- `internal/store/store.go` - Added ListRunsByProject to Store interface
- `internal/store/sqlite/runs.go` - Implemented ListRunsByProject with ORDER BY created_at DESC
- `internal/store/sqlite/runs_test.go` - Added 3 tests: Empty, OrderedNewestFirst, ScopedToProject
- `internal/server/handlers.go` - Added handleListRuns and handleFileDownload handlers; added time import
- `internal/server/server.go` - Wired GET /runs/ and GET /runs/{id}/files routes with viewer role
- `internal/server/handlers_runs_test.go` - 8 new tests covering all handler behaviors

## Decisions Made
- File download handler accepts the raw `?path=` parameter directly as the storage backend path (not constructed from baseDir/runID), keeping the handler simple and consistent with how LocalBackend.DownloadFileStream works
- Preview mode reads the full file into memory then truncates (via DownloadFile), not streaming; acceptable for <=100KB use case
- Empty run list returns `[]` not `null` via explicit guard before writeJSON

## Deviations from Plan

**1. [Rule 1 - Bug] Fixed project foreign key violation in store tests**
- **Found during:** Task 1 (TDD RED phase, tests showed FOREIGN KEY constraint failed)
- **Issue:** Test created runs with a plain string project_id without first creating a project record; SQLite foreign key enforcement rejected the insert
- **Fix:** Each test that creates project-scoped runs now calls `s.CreateProject` first using the existing `newProject` helper
- **Files modified:** internal/store/sqlite/runs_test.go
- **Verification:** All 3 ListRunsByProject tests pass
- **Committed in:** 90c8279 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 - bug fix)
**Impact on plan:** Essential for correctness — tests would have been testing against broken data. No scope creep.

## Issues Encountered
- None beyond the FK constraint fix above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Backend endpoints are complete and tested; Plan 02 (frontend file browsing UI) can implement against these endpoints
- Both endpoints require viewer role; frontend must pass JWT token in Authorization header
- File download path must be an absolute path accessible by the server process

## Self-Check: PASSED
- All 7 modified/created files exist on disk
- Both task commits (90c8279, 5fd2e05) exist in git log
- Full test suite passes: `go test ./internal/store/sqlite/... ./internal/server/... -count=1`

---
*Phase: 30-file-browsing*
*Completed: 2026-04-07*
