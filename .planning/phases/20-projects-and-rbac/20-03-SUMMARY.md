---
phase: 20-projects-and-rbac
plan: "03"
subsystem: api
tags: [chi, rbac, project-scoped-runs, workflow]

# Dependency graph
requires:
  - phase: 20-02
    provides: projectContext middleware, projectRoleRequired RBAC middleware, store.Project struct
  - phase: 20-01
    provides: Run.ProjectID field, store.Store project methods

provides:
  - "Project-scoped run routes: POST /projects/{slug}/runs/submit, GET /{id}/status, /{id}/outputs, POST /{id}/abort"
  - "projectIDFromContext helper that reads store.Project from request context"
  - "handleSubmit sets Run.ProjectID from projectIDFromContext (works for both auth-enabled and auth-disabled)"
  - "Auth-disabled backward compat: /workflows routes preserved, /projects routes not registered"

affects:
  - "21-registry"
  - "spectool integration tests"

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Project context flows through chi route groups into run records via projectIDFromContext"
    - "Single handleSubmit serves both /workflows (no project) and /projects/{slug}/runs/submit (with project)"
    - "Auth-enabled branch removes /workflows routes; auth-disabled branch keeps /workflows routes unchanged"

key-files:
  created: []
  modified:
    - internal/server/handlers.go
    - internal/server/server.go
    - internal/server/handlers_projects_test.go

key-decisions:
  - "projectIDFromContext returns empty string (not an error) when no project in context — enables same handler for both paths"
  - "/workflows routes removed from auth-enabled JWT group; project-scoped /runs routes are the replacement"
  - "executor role required for submit/abort; viewer role sufficient for status/outputs reads"

patterns-established:
  - "Pattern: context-to-record injection — helpers like projectIDFromContext read context and return empty/zero value when absent, never panic"

requirements-completed: [PROJ-07]

# Metrics
duration: 2min
completed: 2026-03-17
---

# Phase 20 Plan 03: Project-scoped Run Routes Summary

**Project-scoped workflow run endpoints under /projects/{slug}/runs/... with projectIDFromContext injecting project_id into run records, preserving auth-disabled /workflows backward compatibility.**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-17T19:30:34Z
- **Completed:** 2026-03-17T19:32:35Z
- **Tasks:** 1 (TDD: RED + GREEN)
- **Files modified:** 3

## Accomplishments

- Added `projectIDFromContext` helper in `handlers.go` that extracts project ID from request context via `projectFromContext`, returning empty string when no project is present (auth-disabled path)
- Modified `handleSubmit` to set `Run.ProjectID = projectIDFromContext(r.Context())` — single-line addition; when no project is in context, ProjectID remains empty string (maps to NULL in SQLite)
- Registered `/projects/{slug}/runs` sub-resource in auth-enabled branch: POST `/submit` and `/task-run` (executor), GET `/{id}/status` and `/{id}/outputs` (viewer), POST `/{id}/abort` (executor)
- Removed `/workflows` routes from the auth-enabled JWT group — project-scoped routes are the auth-enabled replacement; auth-disabled branch retains `/workflows` unchanged
- Full test suite passes including all 156+ spectool-exercised paths

## Task Commits

TDD task with two atomic commits:

1. **RED - Failing tests** - `ce07689` (test)
2. **GREEN - Implementation** - `9917bf4` (feat)

**Plan metadata:** (docs commit — see below)

_TDD: RED commit first (failing tests), then GREEN (implementation passing all tests)_

## Files Created/Modified

- `internal/server/handlers.go` - Added `projectIDFromContext` helper; modified `handleSubmit` to set `Run.ProjectID`
- `internal/server/server.go` - Added `/runs` sub-resource under `/projects/{slug}`; removed `/workflows` from auth-enabled JWT group
- `internal/server/handlers_projects_test.go` - Added `TestProjectRunSubmit`, `TestProjectRunSubmit_ViewerForbidden`, `TestProjectRunSubmit_NonMemberForbidden`, `TestProjectRunStatus`, `TestProjectRunOutputs`, `TestAuthDisabledBackcompat`

## Decisions Made

- `projectIDFromContext` returns empty string (not an error) when no project is in context — same `handleSubmit` function serves both `/workflows` (no project) and project-scoped routes, keeping behavior consistent
- `/workflows` routes removed from the auth-enabled branch to avoid dual registration; auth-disabled branch is untouched to preserve spectool compatibility
- `executor` role required for submit/abort; `viewer` role sufficient for read-only status/outputs endpoints

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- PROJ-07 complete: all workflow runs submitted under auth-enabled mode carry `project_id` in the database
- Ready for Phase 21 (Registry) — project isolation is fully in place
- Auth-disabled paths (spectool integration tests) remain completely unchanged

---
*Phase: 20-projects-and-rbac*
*Completed: 2026-03-17*
