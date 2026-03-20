---
phase: 20-projects-and-rbac
plan: "01"
subsystem: database
tags: [sqlite, goose, migration, store, project, membership, rbac]

# Dependency graph
requires:
  - phase: 19-user-authentication
    provides: User model, Store interface patterns, users table in SQLite
provides:
  - Project and ProjectMember models in store.go
  - 13 new Store interface methods (7 project + 6 membership ops)
  - SQLite migration 00005 creating projects and project_members tables
  - project_id column on runs table
  - Full SQLiteStore implementation for all project and membership methods
  - RemoveMemberIfNotLastAdmin last-admin protection logic
affects:
  - 20-02-project-crud-handlers
  - 20-03-rbac-middleware
  - 21-registry

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "project_id nullable in SQL, enforced non-empty at application layer"
    - "Writer connection used for both count check and delete in RemoveMemberIfNotLastAdmin (serialized safety)"
    - "Soft-delete via is_active=0 for projects (same pattern as users)"

key-files:
  created:
    - internal/store/sqlite/migrations/00005_projects.sql
    - internal/store/sqlite/projects.go
    - internal/store/sqlite/members.go
    - internal/store/sqlite/projects_test.go
    - internal/store/sqlite/members_test.go
  modified:
    - internal/store/store.go
    - internal/store/sqlite/runs.go

key-decisions:
  - "project_id on runs is nullable in SQL (NOT NULL not enforced by SQLite ALTER TABLE), enforced non-empty at app layer"
  - "RemoveMemberIfNotLastAdmin uses writer connection for count+delete to leverage MaxOpenConns=1 serialization"
  - "Migration 00005 clears existing run and task data before adding project_id column — no historical data to migrate"

patterns-established:
  - "scanProject/scanMember helpers follow rowScanner interface pattern from scanUser"
  - "projectRowScanner/memberRowScanner interfaces enable testing without real DB"
  - "ListProjectsForUser uses INNER JOIN project_members instead of subquery"

requirements-completed: [PROJ-01, PROJ-02, PROJ-03, PROJ-04, PROJ-05, PROJ-06, PROJ-07]

# Metrics
duration: 4min
completed: 2026-03-17
---

# Phase 20 Plan 01: Project Data Layer Summary

**SQLite-backed Project and ProjectMember store with 13-method interface, goose migration 00005, and last-admin protection**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-17T19:18:31Z
- **Completed:** 2026-03-17T19:22:40Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments

- Project and ProjectMember models added to store.go with all required fields; Run.ProjectID field added
- 13 new Store interface methods declared (CreateProject through RemoveMemberIfNotLastAdmin)
- Migration 00005 creates projects and project_members tables and adds project_id to runs with correct goose annotations
- Full SQLiteStore implementation in projects.go and members.go following established writer/reader connection patterns
- RemoveMemberIfNotLastAdmin uses writer connection for count+delete atomicity (no separate transaction needed due to MaxOpenConns=1 serialization)
- runs.go updated to write and read project_id column; all existing run tests continue to pass

## Task Commits

Each task was committed atomically:

1. **Task 1: Add models, migration, and Store interface methods** - `7b86251` (feat)
2. **Task 2 (RED): Failing tests for project and membership operations** - `4f4bf94` (test)
3. **Task 2 (GREEN): Implement SQLite store methods** - `2027e82` (feat)

## Files Created/Modified

- `internal/store/store.go` - Added Project, ProjectMember structs; ProjectID on Run; 13 Store interface methods
- `internal/store/sqlite/migrations/00005_projects.sql` - DDL for projects, project_members, project_id on runs
- `internal/store/sqlite/projects.go` - CreateProject, GetProjectBySlug, GetProjectByID, ListProjectsForUser, ListAllProjects, UpdateProject, DeactivateProject
- `internal/store/sqlite/members.go` - AddMember, GetProjectMember, UpdateMemberRole, RemoveMember, ListMembers, CountProjectAdmins, RemoveMemberIfNotLastAdmin
- `internal/store/sqlite/projects_test.go` - 7 test functions for project CRUD and run ProjectID round-trip
- `internal/store/sqlite/members_test.go` - 8 test functions including last-admin protection (blocked + allowed cases)
- `internal/store/sqlite/runs.go` - Write project_id in CreateRun; read project_id in GetRun and GetRunByPrefix

## Decisions Made

- project_id on runs is nullable in SQL (SQLite ALTER TABLE cannot add NOT NULL without default); enforced non-empty at application layer per research pitfall 1
- RemoveMemberIfNotLastAdmin uses writer connection for both count check and delete, avoiding a separate transaction while leveraging MaxOpenConns=1 serialization for safety (per research pitfall 3)
- Migration 00005 clears existing run and task data before adding project_id — acceptable since there is no valid project to assign historical runs to

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All 13 Store interface methods implemented and tested; SQLiteStore compiles and satisfies store.Store interface
- Project CRUD handlers (plan 20-02) can build directly on these store methods
- RBAC middleware (plan 20-03) can use GetProjectMember and ListMembers for role checks
- Full test suite (go test ./... -count=1) passes with no regressions

## Self-Check: PASSED

All created files verified on disk. All task commits verified in git history.

---
*Phase: 20-projects-and-rbac*
*Completed: 2026-03-17*
