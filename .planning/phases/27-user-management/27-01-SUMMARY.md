---
phase: 27-user-management
plan: "01"
subsystem: backend-api + frontend-data-layer
tags: [go, typescript, react-query, shadcn-ui, jwt, tdd]
dependency_graph:
  requires: []
  provides:
    - PATCH /users/{id}/reactivate handler
    - PUT /users/{id}/password (admin) handler
    - apiClient delete and patch methods
    - shadcn/ui table, badge, dropdown-menu, select, alert-dialog components
    - React Query hooks for user CRUD
  affects:
    - internal/server/handlers_auth.go
    - internal/server/server.go
    - web/src/lib/apiClient.ts
    - web/src/features/users/useUsers.ts
tech_stack:
  added:
    - "@radix-ui/react-alert-dialog"
    - "@radix-ui/react-dropdown-menu"
    - "@radix-ui/react-select"
  patterns:
    - TDD (RED-GREEN) for all new code
    - React Query useMutation with query invalidation
    - chi URL params for RESTful route parameters
key_files:
  created:
    - web/src/components/ui/table.tsx
    - web/src/components/ui/badge.tsx
    - web/src/components/ui/dropdown-menu.tsx
    - web/src/components/ui/select.tsx
    - web/src/components/ui/alert-dialog.tsx
    - web/src/features/users/useUsers.ts
    - web/src/features/users/__tests__/useUsers.test.tsx
  modified:
    - internal/api/types.go
    - internal/server/handlers_auth.go
    - internal/server/handlers_auth_test.go
    - internal/server/server.go
    - web/src/lib/apiClient.ts
    - web/src/lib/__tests__/apiClient.test.ts
decisions:
  - "shadcn CLI places files in literal @/ directory; moved manually to src/components/ui/ (consistent with Phase 26 pattern)"
  - "PATCH added to CORS AllowedMethods alongside the new PATCH /users/{id}/reactivate route"
metrics:
  duration: 4 min
  completed: "2026-04-06"
  tasks_completed: 2
  files_modified: 14
---

# Phase 27 Plan 01: Backend Endpoints, apiClient Extensions, and React Query Hooks Summary

Two new Go handlers (reactivate user + admin password reset), delete/patch apiClient methods, five shadcn/ui components, and five React Query hooks for complete user CRUD data layer.

## Tasks Completed

| # | Name | Commit | Status |
|---|------|--------|--------|
| 1 | Backend endpoints + apiClient extensions | d41af19 | Done |
| 2 | Install shadcn/ui components + React Query hooks | 6e1062d | Done |

## What Was Built

### Task 1: Backend endpoints + apiClient extensions

**Backend (Go):**
- Added `AdminResetPasswordRequest` DTO to `internal/api/types.go`
- Implemented `handleReactivateUser` — PATCH `/users/{id}/reactivate` (admin only): sets `is_active=true`, returns updated user as JSON 200
- Implemented `handleAdminResetPassword` — PUT `/users/{id}/password` (admin only): validates min 8 chars, hashes new password, returns 204
- Registered both routes in `server.go` after the existing DELETE route (AFTER `/users/me` routes to avoid chi conflicts)
- Added PATCH to CORS AllowedMethods (Rule 2 auto-fix: PATCH was missing despite new PATCH route)

**Frontend (TypeScript):**
- Added `delete` and `patch` methods to `apiClient` with correct Bearer token injection
- `patch` accepts optional body (spread only when defined)

Tests: 6 new backend tests, 3 new apiClient tests — all passing.

### Task 2: shadcn/ui components + React Query hooks

**shadcn/ui components installed** (moved from `@/` to `src/components/ui/`):
- `table.tsx` — Table, TableHeader, TableBody, TableRow, TableHead, TableCell, TableCaption, TableFooter
- `badge.tsx` — Badge + badgeVariants
- `dropdown-menu.tsx` — DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem, etc.
- `select.tsx` — Select, SelectTrigger, SelectContent, SelectItem, SelectValue, etc.
- `alert-dialog.tsx` — AlertDialog, AlertDialogTrigger, AlertDialogContent, AlertDialogAction, AlertDialogCancel, etc.

**React Query hooks** (`web/src/features/users/useUsers.ts`):
- `useUsers` — `GET /users`, queryKey `['users']`
- `useCreateUser` — `POST /users`, invalidates `['users']`
- `useDeactivateUser` — `DELETE /users/{id}`, invalidates `['users']`
- `useReactivateUser` — `PATCH /users/{id}/reactivate`, invalidates `['users']`
- `useResetPassword` — `PUT /users/{id}/password`, invalidates `['users']`

Tests: 5 hook tests — all passing.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing functionality] PATCH missing from CORS AllowedMethods**
- **Found during:** Task 1 route registration
- **Issue:** CORS config only listed GET, POST, PUT, DELETE, OPTIONS — new PATCH endpoint would fail CORS preflight
- **Fix:** Added PATCH to `AllowedMethods` in `server.go`
- **Files modified:** `internal/server/server.go`
- **Commit:** d41af19

**2. [Rule 3 - Known shadcn CLI placement issue] Components placed in @/ literal directory**
- **Found during:** Task 2 shadcn install
- **Issue:** shadcn CLI created `web/@/components/ui/` instead of `web/src/components/ui/`
- **Fix:** Moved all 5 new files to correct location and removed `@/` directory (documented in plan as known issue from Phase 26-01)
- **Files modified:** component files moved
- **Commit:** 6e1062d

## Verification Results

```
go test ./internal/server/ -run "TestReactivateUser|TestAdminResetPassword" -v
ok  github.com/bioflowy/flowy/internal/server — 6/6 PASS

cd web && npx vitest run --reporter=verbose
Test Files  7 passed (7)
Tests       38 passed (38)

cd web && npx tsc --noEmit
(no errors)
```

## Self-Check: PASSED
