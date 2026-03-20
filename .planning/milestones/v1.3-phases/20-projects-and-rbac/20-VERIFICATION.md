---
phase: 20-projects-and-rbac
verified: 2026-03-17T19:36:47Z
status: passed
score: 13/13 must-haves verified
re_verification: false
---

# Phase 20: Projects and RBAC Verification Report

**Phase Goal:** Users can create isolated project workspaces and control who can do what via role-based membership
**Verified:** 2026-03-17T19:36:47Z
**Status:** passed
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Project and ProjectMember models exist with all required fields | VERIFIED | `store.go` lines 52-74: `type Project struct`, `type ProjectMember struct`, `Run.ProjectID string` |
| 2 | Store interface has all project and membership CRUD methods | VERIFIED | 13 methods declared in `store.go` lines 174-208: 7 project + 5 membership + RemoveMemberIfNotLastAdmin |
| 3 | SQLite migration creates projects and project_members tables and adds project_id to runs | VERIFIED | `00005_projects.sql`: CREATE TABLE projects, CREATE TABLE project_members, ALTER TABLE runs ADD COLUMN project_id |
| 4 | Store implementation passes unit tests for all CRUD operations | VERIFIED | `go test ./internal/store/sqlite/... -count=1` passes (2.475s, no failures) |
| 5 | Last-admin protection prevents removing the only admin from a project | VERIFIED | `RemoveMemberIfNotLastAdmin` in `members.go`; `TestRemoveMemberIfNotLastAdmin_Blocked` and `_Allowed` in `members_test.go` |
| 6 | Authenticated user can POST /projects to create a project and become admin automatically | VERIFIED | `handleCreateProject` calls `store.CreateProject` then `store.AddMember` with role "admin"; `TestCreateProject` passes |
| 7 | User sees only their projects in GET /projects (global admin sees all) | VERIFIED | `handleListProjects` branches on `claims.Role == "admin"` for `ListAllProjects` vs `ListProjectsForUser`; `TestListProjects_MemberOnly` and `TestListProjects_GlobalAdmin` pass |
| 8 | Project member can GET /projects/{slug} to see details | VERIFIED | `r.With(s.projectRoleRequired("viewer")).Get("/", s.handleGetProject)` wired; `TestGetProject_Member` passes |
| 9 | Project admin can PUT/DELETE /projects/{slug}; non-admin gets 403 | VERIFIED | Routes use `projectRoleRequired("admin")`; `TestUpdateProject_Admin` and `TestGetProject_NonMember` (403) pass |
| 10 | Project admin can manage members; non-admin gets 403 | VERIFIED | Members routes use `projectRoleRequired("admin")`; `TestMembership` and role-enforcement tests pass |
| 11 | Last-admin removal returns 400 error | VERIFIED | `handleRemoveMember` delegates to `RemoveMemberIfNotLastAdmin`; `TestRemoveMember_LastAdmin` passes |
| 12 | Global admin bypasses membership check | VERIFIED | `projectRoleRequired` checks `claims.Role == "admin"` first (middleware.go line 70); `TestGlobalAdminBypass` passes |
| 13 | Run submitted under /projects/{slug}/runs/submit has project_id set in the database | VERIFIED | `handlers.go` line 165: `ProjectID: projectIDFromContext(r.Context())`; `TestProjectRunSubmit` and auth-disabled backcompat pass |

**Score:** 13/13 truths verified

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/store/store.go` | Project, ProjectMember models + 13 Store interface methods | VERIFIED | Models at lines 52-74; 13 interface methods at lines 174-208 |
| `internal/store/sqlite/migrations/00005_projects.sql` | DDL for projects, project_members; project_id on runs | VERIFIED | CREATE TABLE projects, project_members; ALTER TABLE runs ADD COLUMN project_id |
| `internal/store/sqlite/projects.go` | SQLiteStore project CRUD methods | VERIFIED | 7 methods: CreateProject through DeactivateProject, all implemented with real DB queries |
| `internal/store/sqlite/members.go` | SQLiteStore membership CRUD methods | VERIFIED | 7 methods: AddMember through RemoveMemberIfNotLastAdmin, all implemented |
| `internal/store/sqlite/projects_test.go` | Unit tests for project store operations | VERIFIED | 7 test functions including ProjectID round-trip |
| `internal/store/sqlite/members_test.go` | Unit tests for membership including last-admin protection | VERIFIED | 8 test functions including TestRemoveMemberIfNotLastAdmin_Blocked and _Allowed |
| `internal/api/types.go` | Project and membership request/response DTOs | VERIFIED | 6 DTO types: CreateProjectRequest, UpdateProjectRequest, ProjectResponse, AddMemberRequest, UpdateMemberRequest, MemberResponse |
| `internal/server/middleware.go` | projectContext and projectRoleRequired middleware | VERIFIED | projectContext (line 38), projectFromContext (line 53), projectRoleRequired (line 66), roleAtLeast helper |
| `internal/server/handlers_projects.go` | Project CRUD handlers | VERIFIED | 5 handlers: handleCreateProject, handleListProjects, handleGetProject, handleUpdateProject, handleDeleteProject |
| `internal/server/handlers_members.go` | Membership management handlers | VERIFIED | 4 handlers: handleAddMember, handleListMembers, handleUpdateMember, handleRemoveMember |
| `internal/server/server.go` | Project route group in newRouter | VERIFIED | /projects route group at lines 114-142 within auth-enabled branch |
| `internal/server/handlers.go` | projectIDFromContext helper; handleSubmit sets project_id | VERIFIED | projectIDFromContext at line 109; Run.ProjectID set at line 165 |
| `internal/server/handlers_projects_test.go` | Integration tests for project and membership HTTP endpoints | VERIFIED | 16+ tests covering all PROJ-01 to PROJ-07 scenarios including run submission |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/server/middleware.go` | `internal/store/store.go` | projectContext calls store.GetProjectBySlug | WIRED | Line 41: `s.store.GetProjectBySlug(r.Context(), slug)` |
| `internal/server/handlers_projects.go` | `internal/store/store.go` | handlers call store project methods | WIRED | Line 68: `s.store.CreateProject(r.Context(), proj)` |
| `internal/server/server.go` | `internal/server/middleware.go` | route groups use projectContext and projectRoleRequired | WIRED | Lines 121, 123-131, 135-139: `s.projectContext`, `s.projectRoleRequired(...)` |
| `internal/server/handlers.go` | `internal/store/store.go` | handleSubmit sets Run.ProjectID from projectIDFromContext | WIRED | Line 165: `ProjectID: projectIDFromContext(r.Context())` |
| `internal/server/server.go` | `internal/server/handlers.go` | run routes registered under /projects/{slug}/runs | WIRED | Lines 135-139: runs sub-resource with handleSubmit, handleStatus, handleOutputs, handleAbort |

---

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| PROJ-01 | 20-01, 20-02 | User can create a project with name and description | SATISFIED | `handleCreateProject` validates slug, creates Project struct, persists via store; integration test `TestCreateProject` passes |
| PROJ-02 | 20-01, 20-02 | User can list projects they are a member of | SATISFIED | `handleListProjects` calls `ListProjectsForUser` for non-admins; `TestListProjects_MemberOnly` passes |
| PROJ-03 | 20-01, 20-02 | User can view project details | SATISFIED | `handleGetProject` reads project from context (injected by projectContext middleware); `TestGetProject_Member` passes |
| PROJ-04 | 20-01, 20-02 | Project admin can update or delete a project | SATISFIED | PUT/DELETE routes protected by `projectRoleRequired("admin")`; `TestUpdateProject_Admin`, `TestDeleteProject_Admin` pass |
| PROJ-05 | 20-01, 20-02 | Project admin can add/remove members with role (admin/executor/viewer) | SATISFIED | `handleAddMember`/`handleRemoveMember` with role validation; `TestMembership` and `TestLastAdminProtection` pass |
| PROJ-06 | 20-01, 20-02 | Project admin can change a member's role | SATISFIED | `handleUpdateMember` validates new role and calls `store.UpdateMemberRole`; `TestUpdateMemberRole` in members_test.go passes |
| PROJ-07 | 20-01, 20-03 | Workflow runs are scoped to a project (project_id on runs table) | SATISFIED | migration 00005 adds project_id column; `projectIDFromContext` injects project ID into `Run.ProjectID` on submit; `TestProjectRunSubmit` passes |

No orphaned requirements. All 7 PROJ requirements are claimed by plans and verified in code.

---

### Anti-Patterns Found

No anti-patterns detected in phase-modified files. No TODO/FIXME/placeholder comments, no empty handler implementations, no stub return values found.

---

### Human Verification Required

None. All observable truths are verifiable through code inspection and automated tests.

The following behaviors were confirmed programmatically via integration tests:

1. **Auth-disabled backward compatibility** — `TestAuthDisabledBackcompat` verifies `POST /workflows` still works and `POST /projects/...` returns 404 when auth is disabled.
2. **Non-member 403 enforcement** — `TestGetProject_NonMember`, `TestProjectRunSubmit_NonMemberForbidden` verify membership gating.
3. **Role hierarchy** — `TestProjectRunSubmit_ViewerForbidden` verifies executor-minimum for submit; viewer role allows read-only endpoints.

---

### Test Suite Results

| Package | Result | Duration |
|---------|--------|----------|
| `internal/store/sqlite` | PASS | 2.475s |
| `internal/server` | PASS | 4.387s |
| All packages (`go test ./...`) | PASS | All green, 0 failures |

---

## Summary

Phase 20 goal is fully achieved. All three waves of execution (data layer, HTTP API + RBAC, project-scoped runs) delivered working, tested code. The codebase supports isolated project workspaces with create/list/get/update/delete operations, role-based membership (admin/executor/viewer) with last-admin protection, global admin bypass, and workflow runs scoped to a project via `project_id` on run records. Auth-disabled paths are unchanged. All 7 PROJ requirements are satisfied.

---

_Verified: 2026-03-17T19:36:47Z_
_Verifier: Claude (gsd-verifier)_
