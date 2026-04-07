---
phase: 31-audit-gap-closure
plan: "02"
subsystem: security
tags: [security, auth, api, frontend]
dependency_graph:
  requires: []
  provides: [secured-get-users, project-scoped-user-list, updated-add-member-dialog]
  affects: [internal/server, web/src/features/projects]
tech_stack:
  added: []
  patterns: [roleRequired middleware, projectRoleRequired middleware, project-scoped API endpoint]
key_files:
  created: []
  modified:
    - internal/server/server.go
    - internal/server/handlers_auth.go
    - internal/server/handlers_auth_test.go
    - web/src/features/projects/useProjects.ts
    - web/src/features/projects/AddMemberDialog.tsx
    - web/src/features/projects/__tests__/AddMemberDialog.test.tsx
decisions:
  - "GET /users restricted to global admin only; project-scoped GET /projects/{slug}/users added for project admin Add Member dropdown"
  - "handleListUsersForProject filters inactive users — only active users can meaningfully be added as project members"
metrics:
  duration: "~8 minutes"
  completed: "2026-04-07T05:28:37Z"
  tasks: 2
  files: 6
---

# Phase 31 Plan 02: Admin-Only GET /users with Project-Scoped User List Summary

**One-liner:** Secured GET /users to admin-only and added GET /projects/{slug}/users for project admins to power the Add Member dropdown.

## What Was Built

This plan closed a P2 security gap: any authenticated user could enumerate all platform users via GET /users. The fix is a two-part approach:

1. **Backend:** Added `roleRequired("admin")` middleware to the existing GET /users route. Created a new `handleListUsersForProject` handler and corresponding route `GET /projects/{slug}/users` protected by `projectRoleRequired("admin")`. The project-scoped handler returns only active users.

2. **Frontend:** Added `useProjectUsers(slug)` hook to `useProjects.ts` that fetches from the project-scoped endpoint. Updated `AddMemberDialog.tsx` to use `useProjectUsers` instead of `useUsers`, so the Add Member dropdown no longer calls the admin-only global user list. Updated the test mock accordingly.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Add admin guard to GET /users and create project-scoped user list endpoint | b95a679 | server.go, handlers_auth.go, handlers_auth_test.go |
| 2 | Update AddMemberDialog to use project-scoped user list endpoint | 9573a98 | useProjects.ts, AddMemberDialog.tsx, AddMemberDialog.test.tsx |

## Verification Results

- `go test ./internal/server/ -run TestHandleListUsers -v`: NonAdmin gets 403, Admin gets 200 — PASS
- `go test ./internal/server/ -run TestHandleListUsersForProject -v`: ProjectAdmin gets 200 — PASS
- `cd web && npx vitest run`: 112 tests across 19 files — all PASS
- `grep "roleRequired.*admin.*Get.*users" internal/server/server.go`: confirms admin middleware on GET /users
- `grep "useProjectUsers" web/src/features/projects/AddMemberDialog.tsx`: confirms new hook usage

## Decisions Made

- GET /users restricted to global admin only; project-scoped GET /projects/{slug}/users added for project admin Add Member dropdown.
- `handleListUsersForProject` filters inactive users — only active users can meaningfully be added as project members.

## Deviations from Plan

None - plan executed exactly as written.

## Self-Check: PASSED

- internal/server/server.go — FOUND
- internal/server/handlers_auth.go — FOUND (contains handleListUsersForProject)
- internal/server/handlers_auth_test.go — FOUND (contains TestHandleListUsers_NonAdmin expecting 403, TestHandleListUsersForProject_ProjectAdmin)
- web/src/features/projects/useProjects.ts — FOUND (exports useProjectUsers)
- web/src/features/projects/AddMemberDialog.tsx — FOUND (uses useProjectUsers, no useUsers import)
- web/src/features/projects/__tests__/AddMemberDialog.test.tsx — FOUND (mocks useProjectUsers)
- Commit b95a679 — FOUND
- Commit 9573a98 — FOUND
