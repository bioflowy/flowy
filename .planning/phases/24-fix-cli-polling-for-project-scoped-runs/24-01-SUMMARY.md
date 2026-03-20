---
phase: 24-fix-cli-polling-for-project-scoped-runs
plan: 01
subsystem: api
tags: [cli, http-client, project-scoped, polling]

requires:
  - phase: 22-cli-registry-integration
    provides: "SubmitToProject and TaskRunInProject client methods, isRegistryRef detection"
provides:
  - "StatusInProject client method for project-scoped run status polling"
  - "OutputsInProject client method for project-scoped run output retrieval"
  - "pollUntilDone and runRun conditional project-scoped routing"
affects: []

tech-stack:
  added: []
  patterns:
    - "Conditional project-scoped vs non-project HTTP routing in CLI based on projectSlug"

key-files:
  created: []
  modified:
    - internal/client/client.go
    - internal/client/client_test.go
    - cmd/flowy/main.go
    - cmd/flowy/run_test.go

key-decisions:
  - "StatusInProject and OutputsInProject follow exact same pattern as existing Status/Outputs methods"
  - "pollUntilDone takes projectSlug parameter with empty-string check for backward compatibility"

patterns-established:
  - "Project-scoped client methods: append InProject suffix, add projectSlug param, hit /projects/{slug}/runs/{id}/* routes"

requirements-completed: [EXEC-01]

duration: 2min
completed: 2026-03-20
---

# Phase 24 Plan 01: Fix CLI Polling for Project-Scoped Runs Summary

**Project-scoped StatusInProject and OutputsInProject client methods with conditional routing in pollUntilDone and runRun**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-20T05:13:57Z
- **Completed:** 2026-03-20T05:16:07Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Added StatusInProject and OutputsInProject methods to client library hitting /projects/{slug}/runs/{id}/status and /outputs
- Updated pollUntilDone to conditionally use project-scoped status polling when projectSlug is non-empty
- Updated runRun to conditionally use project-scoped output retrieval when projectSlug is non-empty
- Full E2E registry-ref flow (submit -> poll -> outputs) now uses project-scoped routes with auth enabled
- All existing non-project tests pass unchanged (backward compatibility preserved)

## Task Commits

Each task was committed atomically:

1. **Task 1: Add StatusInProject and OutputsInProject to client library** - `ebf7ab4` (feat)
2. **Task 2: Update pollUntilDone and runRun to use project-scoped methods** - `295f508` (feat)

_Note: TDD tasks — tests written first (RED), then implementation (GREEN)._

## Files Created/Modified
- `internal/client/client.go` - Added StatusInProject and OutputsInProject methods
- `internal/client/client_test.go` - Added 5 tests for project-scoped client methods
- `cmd/flowy/main.go` - Updated pollUntilDone signature and runRun outputs fetch with projectSlug routing
- `cmd/flowy/run_test.go` - Added TestRunRegistryRefProjectScoped verifying project-scoped flow and non-project route isolation

## Decisions Made
- StatusInProject and OutputsInProject follow the exact same pattern as existing Status and Outputs methods for consistency
- pollUntilDone takes projectSlug as a parameter (empty string = use legacy non-project route) rather than a separate function

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- EXEC-01 gap is closed: full E2E flow (login -> push -> submit by registry ref -> poll -> outputs) now works with auth enabled
- v1.3 milestone should now have 25/25 requirements complete

---
*Phase: 24-fix-cli-polling-for-project-scoped-runs*
*Completed: 2026-03-20*
