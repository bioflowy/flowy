---
phase: 22-registry-based-execution
plan: 01
subsystem: api
tags: [sqlite, migration, registry, wdl, handlers]

# Dependency graph
requires:
  - phase: 21-wdl-registry
    provides: "Registry store methods: GetRegistryVersionByTag, GetRegistryVersionByDigest, registry schema"
provides:
  - "Migration 00007 adding registry_ref and registry_digest columns to runs table"
  - "store.Run model with RegistryRef and RegistryDigest fields"
  - "handleSubmit accepts registry_ref and resolves WDL from registry"
  - "handleTaskRun accepts registry_ref and resolves WDL from registry"
  - "handleClaim re-resolves WDL content by digest for registry-based runs"
  - "Convenience routes /projects/{slug}/workflows and /projects/{slug}/tasks/run"
affects:
  - 22-02
  - any executor that submits runs via registry reference

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "parseRegistryRef splits name:tag or name@sha256:hex into components"
    - "resolveRegistryRef fetches WDL content from store and returns (content, digest)"
    - "handleClaim re-resolves registry runs by digest for reproducibility"

key-files:
  created:
    - internal/store/sqlite/migrations/00007_run_registry_ref.sql
    - internal/store/sqlite/runs_test.go
    - internal/server/handlers_registry_exec_test.go
  modified:
    - internal/store/store.go
    - internal/store/sqlite/runs.go
    - internal/api/types.go
    - internal/server/handlers.go
    - internal/server/server.go

key-decisions:
  - "Convenience routes /projects/{slug}/workflows and /projects/{slug}/tasks/run added for registry-based execution — reuses existing handleSubmit/handleTaskRun with project context middleware"
  - "handleClaim re-resolves registry WDL by stored RegistryDigest (not tag) for reproducibility"
  - "registry_ref and registry_digest columns added as NOT NULL DEFAULT '' — empty string for file-path runs"

patterns-established:
  - "parseRegistryRef: unified parsing for name:tag and name@sha256:hex formats"
  - "Mutual exclusion validation: wdl_path and registry_ref are mutually exclusive with 400 responses"
  - "Registry resolution returns 422 for unknown refs (not 404), distinguishing from resource-not-found"

requirements-completed: [EXEC-01, EXEC-02]

# Metrics
duration: 4min
completed: 2026-03-20
---

# Phase 22 Plan 01: Registry-Based Execution Summary

**Registry-based workflow execution via name:tag or name@sha256:hex refs with mutual exclusion validation, 422 for unknown refs, and reproducible handleClaim re-resolution by stored digest**

## Performance

- **Duration:** ~4 min
- **Started:** 2026-03-20T03:09:00Z
- **Completed:** 2026-03-20T03:14:02Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments
- SQLite migration adds registry_ref and registry_digest columns to runs table (NOT NULL DEFAULT '')
- store.Run model, CreateRun INSERT, scanRun, and scanRunRows updated for 10-column layout
- handleSubmit and handleTaskRun accept registry_ref with mutual exclusion validation
- resolveRegistryRef helper resolves both tag and digest forms; returns 422 on not-found
- handleClaim re-resolves WDL content from registry by stored RegistryDigest for reproducibility
- 10 integration tests covering all success and error paths

## Task Commits

Each task was committed atomically:

1. **Task 1: Migration, model, API types, and store layer** - `7f0b1d9` (feat)
2. **Task 2: Handler resolution logic, handleClaim registry branch, and integration tests** - `492a9a1` (feat)

## Files Created/Modified
- `internal/store/sqlite/migrations/00007_run_registry_ref.sql` - Migration adding registry_ref and registry_digest columns
- `internal/store/store.go` - RegistryRef and RegistryDigest fields on Run struct
- `internal/store/sqlite/runs.go` - Updated INSERT, SELECT, scan functions for 10-column layout
- `internal/api/types.go` - RegistryRef field on SubmitRequest and TaskRunRequest
- `internal/server/handlers.go` - parseRegistryRef, resolveRegistryRef helpers; updated handleSubmit, handleTaskRun, handleClaim
- `internal/server/server.go` - Added /projects/{slug}/workflows and /projects/{slug}/tasks/run routes
- `internal/store/sqlite/runs_test.go` - Store tests for registry fields round-trip
- `internal/server/handlers_registry_exec_test.go` - 10 integration tests for registry-based execution

## Decisions Made
- Convenience routes `/projects/{slug}/workflows` and `/projects/{slug}/tasks/run` added alongside existing `/projects/{slug}/runs/submit` and `/projects/{slug}/runs/task-run` — reuses the same handlers with project context injected by middleware
- handleClaim re-resolves by stored RegistryDigest (not by tag) to ensure reproducibility — tag may have moved to a different version by the time a daemon claims the task
- Registry resolution errors return 422 (Unprocessable Entity) to distinguish from 404 (resource not found) — the request was syntactically valid but the referenced registry entry was not found

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Minor: `newTestServer` returns 4 values but initial test used 3 — fixed immediately (compile-time error)

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Registry-based execution complete; POST /projects/{slug}/workflows and /projects/{slug}/tasks/run accept registry_ref
- handleClaim delivers WDL content for registry runs to daemons
- Ready for Plan 02 (executor-side client integration or end-to-end verification)
- All 21 packages pass `go test ./... -count=1`

## Self-Check: PASSED

All created files exist. Both task commits verified: 7f0b1d9, 492a9a1.

---
*Phase: 22-registry-based-execution*
*Completed: 2026-03-20*
