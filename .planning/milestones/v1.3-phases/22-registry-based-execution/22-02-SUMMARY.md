---
phase: 22-registry-based-execution
plan: 02
subsystem: cli
tags: [cli, registry, wdl, client, config]

# Dependency graph
requires:
  - phase: 22-registry-based-execution
    plan: 01
    provides: "Project-scoped routes /projects/{slug}/workflows and /projects/{slug}/tasks/run accepting registry_ref"
provides:
  - "isRegistryRef helper detecting name:tag and name@sha256:hex forms, excluding Windows drive letters"
  - "isAlpha helper for drive-letter exclusion in isRegistryRef"
  - "DefaultProject field on ClientConfig (internal/config/config.go)"
  - "SubmitToProject method on Client (internal/client/client.go)"
  - "TaskRunInProject method on Client (internal/client/client.go)"
  - "--project flag on flowy run and flowy submit"
  - "Registry ref auto-detection in runRun and runSubmit; routes to project-scoped endpoints"
affects:
  - any future CLI work touching flowy run / flowy submit
  - spectool integration (uses flowy run under the hood)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "isRegistryRef: detect name:tag or name@sha256:hex, exclude Windows drive letters (single alpha + colon)"
    - "Registry ref routing: isRegistryRef check -> SubmitToProject/TaskRunInProject; else existing Submit/TaskRun"

key-files:
  created: []
  modified:
    - internal/config/config.go
    - internal/client/client.go
    - cmd/flowy/main.go
    - cmd/flowy/main_test.go

key-decisions:
  - "Windows drive letter exclusion in isRegistryRef: idx==1 && isAlpha(arg[0]) is false — C:\\path is not a registry ref"
  - "Error on registry ref without project is a user-facing validation, not an architectural choice — returns exit 1"
  - "SubmitToProject and TaskRunInProject are additive — existing Submit and TaskRun unchanged for file-path backward compat"

patterns-established:
  - "isRegistryRef: unified detection for both tag and digest forms before routing to project-scoped endpoints"

requirements-completed: [EXEC-01]

# Metrics
duration: ~3min
completed: 2026-03-20
---

# Phase 22 Plan 02: CLI Registry Ref Auto-Detection Summary

**CLI auto-detects name:tag / name@sha256:hex registry references in flowy run and flowy submit, routes to /projects/{slug}/workflows or /projects/{slug}/tasks/run, with --project flag and default_project config support**

## Performance

- **Duration:** ~3 min
- **Started:** 2026-03-20T03:16:26Z
- **Completed:** 2026-03-20T03:18:49Z
- **Tasks:** 1
- **Files modified:** 4

## Accomplishments
- isRegistryRef helper correctly identifies registry refs (name:tag, name@sha256:hex) vs file paths, excluding Windows drive letters
- DefaultProject field added to ClientConfig for config-driven default project slug
- SubmitToProject and TaskRunInProject client methods added posting to /projects/{slug}/workflows and /projects/{slug}/tasks/run
- --project flag wired into both runRun and runSubmit with config fallback to default_project
- Usage strings updated to reflect new flag and registry ref argument form
- TDD: failing tests committed first (89216a0), passing implementation committed second (a0b313d)

## Task Commits

Each task was committed atomically (TDD: test then implementation):

1. **RED: Failing tests for isRegistryRef and registry-ref no-project error** - `89216a0` (test)
2. **GREEN: Full implementation** - `a0b313d` (feat)

**Plan metadata:** (docs commit below)

_Note: TDD task — test commit precedes implementation commit_

## Files Created/Modified
- `internal/config/config.go` - Added DefaultProject field to ClientConfig
- `internal/client/client.go` - Added SubmitToProject and TaskRunInProject methods
- `cmd/flowy/main.go` - Added isRegistryRef, isAlpha, --project flag, registry ref routing in runRun and runSubmit
- `cmd/flowy/main_test.go` - Added TestIsRegistryRef, TestIsRegistryRef_EdgeCases, TestRunRun_RegistryRef_NoProject

## Decisions Made
- Windows drive letter exclusion: `idx == 1 && isAlpha(arg[0])` — single alphabetical character before colon signals a drive letter, not a tag separator. This covers C:, D:, etc. while allowing :empty-name (colon at index 0) to be treated as a registry ref.
- Additive client methods: existing Submit and TaskRun methods left unchanged; SubmitToProject and TaskRunInProject are new methods — this preserves backward compatibility with all existing file-path callers.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Registry-based execution is now end-to-end: server accepts registry_ref (Plan 01), CLI detects and sends it (Plan 02)
- `flowy run my-tool:latest --project myproj` routes to /projects/myproj/workflows with registry_ref set
- `flowy run hello.wdl` still routes to /workflows via file path — no regression
- All 22 packages pass `go test ./... -count=1`

## Self-Check: PASSED

All modified files exist. Both task commits verified: 89216a0 (test), a0b313d (feat).

---
*Phase: 22-registry-based-execution*
*Completed: 2026-03-20*
