---
phase: 20-projects-and-rbac
plan: "02"
subsystem: server
tags: [http, handlers, rbac, middleware, project, membership, chi, integration-tests]

# Dependency graph
requires:
  - phase: 20-01
    provides: Project/ProjectMember models, 13 Store interface methods, SQLiteStore implementation
provides:
  - Project and membership request/response DTOs in api/types.go
  - projectContext middleware (slug -> *store.Project injection with 404 guard)
  - projectRoleRequired middleware with global admin bypass and roleAtLeast() helper
  - 5 project CRUD handlers (create, list, get, update, delete)
  - 4 membership handlers (add, list, update, remove) with last-admin protection
  - /projects route group wired in newRouter() auth-enabled branch
  - Integration tests covering all PROJ-01 through PROJ-06 requirements
affects:
  - 20-03-rbac-middleware (extends project RBAC for run submission)
  - 21-registry (project context for registry entries)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "projectContext middleware injects *store.Project via unexported context key (same pattern as auth.UserClaimsKey)"
    - "projectRoleRequired checks claims.Role == admin first for global bypass (per research pitfall 2)"
    - "validSlug regex ^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$ — 1-char slugs allowed (single alphanumeric)"
    - "handleListProjects uses claims.Role == admin to branch between ListAllProjects and ListProjectsForUser"
    - "handleRemoveMember delegates entirely to RemoveMemberIfNotLastAdmin for last-admin protection"

key-files:
  created:
    - internal/server/handlers_projects.go
    - internal/server/handlers_members.go
    - internal/server/handlers_projects_test.go
  modified:
    - internal/api/types.go
    - internal/server/middleware.go
    - internal/server/server.go

key-decisions:
  - "projectRoleRequired route group uses its own JWTMiddleware invocation (separate from user management group) — cleaner than nesting inside existing group"
  - "handleCreateProject and handleListProjects read claims directly rather than relying on projectRoleRequired — POST /projects and GET /projects are pre-slug routes with no project context"
  - "validSlug allows single-character slugs (regex uses ? to make the middle group optional)"

# Metrics
duration: 3min
completed: 2026-03-17
---

# Phase 20 Plan 02: Project CRUD Handlers and RBAC Middleware Summary

**Project HTTP API with per-project RBAC middleware, 9 handlers, /projects route group, and comprehensive integration tests**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-17T19:25:45Z
- **Completed:** 2026-03-17T19:28:47Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments

- Added 6 new DTO types to api/types.go: CreateProjectRequest, UpdateProjectRequest, ProjectResponse, AddMemberRequest, UpdateMemberRequest, MemberResponse
- Implemented projectContext middleware that resolves {slug} URL param to *store.Project via context, returning 404 for unknown/inactive projects
- Implemented projectRoleRequired middleware with global admin bypass (claims.Role == "admin"), membership check via store.GetProjectMember, and roleAtLeast() helper (viewer=1, executor=2, admin=3)
- Created 5 project CRUD handlers covering all POST/GET/PUT/DELETE operations
- Created 4 membership management handlers with handleRemoveMember delegating to RemoveMemberIfNotLastAdmin for last-admin protection
- Wired /projects route group in newRouter() auth-enabled branch with correct middleware chain
- Written 16+ integration tests covering all PROJ-01 through PROJ-06 requirements
- Full test suite (go test ./... -count=1) passes with zero regressions

## Task Commits

Each task was committed atomically:

1. **Task 1: API types, RBAC middleware, project CRUD handlers** - `2d4205b` (feat)
2. **Task 2: Membership handlers, route wiring, integration tests** - `c331c04` (feat)

## Files Created/Modified

- `internal/api/types.go` - Added 6 DTO types for projects and membership
- `internal/server/middleware.go` - Added projectContext, projectFromContext, projectRoleRequired, roleRank, roleAtLeast
- `internal/server/handlers_projects.go` - handleCreateProject, handleListProjects, handleGetProject, handleUpdateProject, handleDeleteProject; validSlug, projectToResponse helpers
- `internal/server/handlers_members.go` - handleAddMember, handleListMembers, handleUpdateMember, handleRemoveMember; validProjectRole, memberToResponse helpers
- `internal/server/server.go` - /projects route group with JWT, projectContext, projectRoleRequired middleware chain
- `internal/server/handlers_projects_test.go` - 16+ integration tests covering project CRUD, membership, RBAC, global admin bypass

## Decisions Made

- /projects route group has its own JWTMiddleware invocation separate from the existing user management group — avoids nesting complexity
- handleCreateProject and handleListProjects read claims directly since they operate before a project slug is known (no projectRoleRequired applicable)
- validSlug regex makes the middle [a-z0-9-]{0,62}[a-z0-9] group optional via ? so single-character slugs like "a" are valid

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All PROJ-01 through PROJ-06 requirements fulfilled; project HTTP layer is complete
- Plan 20-03 (run association with projects) can use projectFromContext and the /projects/{slug} route group
- Phase 21 (registry) can extend the project context pattern for registry-scoped endpoints

## Self-Check: PASSED

All created files verified on disk. All task commits verified in git history.

---
*Phase: 20-projects-and-rbac*
*Completed: 2026-03-17*
