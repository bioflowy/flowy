# Phase 27: User Management - Context

**Gathered:** 2026-04-06
**Status:** Ready for planning

<domain>
## Phase Boundary

Admins can view and manage all user accounts — creating new users, deactivating/reactivating existing accounts, and resetting passwords — via a dedicated users screen. Non-admin users do not see the users navigation item.

</domain>

<decisions>
## Implementation Decisions

### User table display
- Columns: Username, Role (badge), Status (active/inactive badge), Created date
- Display name and email omitted from table (available via API but not needed for admin overview)
- Row actions via three-dot (…) dropdown menu at end of each row
- Actions: Deactivate (for active users), Reactivate (for inactive users), Reset Password
- Hide action dropdown for the currently logged-in admin (prevent self-deactivation)
- Admin-only nav: hide "Users" sidebar item for non-admin users (role-based navigation)

### Create user form
- Dialog overlay triggered by "Create User" button above the table (consistent with Phase 26 dialog pattern)
- Required fields: Username, Password, Role (dropdown)
- Optional fields: Display Name, Email
- Role dropdown: admin, executor, viewer — default to "viewer" (least privilege)
- Password validation: minimum 8 characters (same as Phase 26)
- After success: Sonner toast ("User created"), dialog closes, table auto-refreshes via React Query invalidation

### Deactivate/reactivate flow
- Deactivate: simple confirmation dialog — "Deactivate {username}? They will no longer be able to log in." with Cancel/Deactivate buttons
- Deactivate button styled as destructive (red variant)
- Reactivate: single click from dropdown, no confirmation dialog — immediate action + success toast
- Both actions refresh the table to show updated status badge
- Backend: existing `DELETE /users/{id}` for deactivation; NEW `PATCH /users/{id}/reactivate` endpoint needed

### Admin password reset
- Dialog overlay triggered from row dropdown menu
- Admin enters the new password manually (communicates to user out-of-band)
- Form fields: New Password, Confirm Password (same zod validation as Phase 26, min 8 chars)
- After success: Sonner toast ("Password reset for {username}"), dialog closes
- Backend: NEW `PUT /users/{id}/password` endpoint needed (admin-only, no current_password required)

### Claude's Discretion
- Table component choice (shadcn/ui Table or custom)
- Loading/empty states for the users table
- Pagination or scroll strategy if user count grows
- Exact badge colors for roles and status

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Backend user API (existing)
- `internal/server/handlers_auth.go` — `handleListUsers` (GET /users), `handleCreateUser` (POST /users), `handleDeleteUser` (DELETE /users/{id} — deactivation)
- `internal/api/types.go` — `CreateUserRequest`, `UserResponse` structs (fields: id, username, display_name, email, global_role, is_active, timestamps)
- `internal/server/server.go` lines 119-124 — Route registration with roleRequired("admin") middleware

### Backend user store
- `internal/store/store.go` — User store interface (CreateUser, ListUsers, DeactivateUser, etc.)

### Frontend auth foundation (Phase 26)
- `web/src/store/authStore.ts` — Auth store with token, username, role — use `role` for admin-only UI gating
- `web/src/lib/apiClient.ts` — API client with Bearer token injection
- `web/src/components/layout/Sidebar.tsx` — Sidebar navigation — add conditional "Users" item for admin role
- `web/src/App.tsx` — Route structure — add /users route inside ProtectedRoute

### UI patterns established in Phase 26
- `web/src/features/auth/ChangePasswordDialog.tsx` — Dialog + form pattern to reuse for create user and reset password dialogs
- `web/src/features/auth/LoginPage.tsx` — Form error handling pattern (inline errors, generic messages)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- shadcn/ui components installed: Button, Card, Dialog, Form, Input, Label, Separator, Tooltip
- ChangePasswordDialog pattern: Dialog + react-hook-form + zod — reuse for Create User and Reset Password dialogs
- apiClient with Bearer token injection — ready for admin API calls
- Sonner toast — already configured for success notifications
- authStore.role — available to gate admin-only UI elements

### Established Patterns
- Dialog-based forms: react-hook-form + zod validation + Sonner toast on success (Phase 26)
- Route structure: ProtectedRoute wraps all authenticated routes, login is outside AppShell
- API error handling: ApiError class with status code, generic error messages

### Integration Points
- Sidebar.tsx: add "Users" nav item conditionally when authStore.role === "admin"
- App.tsx: add `/users` route inside ProtectedRoute
- Backend: add 2 new endpoints — `PATCH /users/{id}/reactivate`, `PUT /users/{id}/password`

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 27-user-management*
*Context gathered: 2026-04-06*
