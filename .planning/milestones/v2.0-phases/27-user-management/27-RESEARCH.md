# Phase 27: User Management - Research

**Researched:** 2026-04-06
**Domain:** Admin user management UI + 2 new backend endpoints
**Confidence:** HIGH

## Summary

Phase 27 builds a user management screen for admins. The backend already has most endpoints (GET /users, POST /users, DELETE /users/{id} for deactivation) and store methods. Two new endpoints are needed: `PATCH /users/{id}/reactivate` and `PUT /users/{id}/password` (admin reset). The store layer already has `UpdateUser` which sets `is_active` and `password_hash`, so both new endpoints can be implemented without new store methods -- though adding a dedicated `ReactivateUser` store method improves clarity.

The frontend needs a new `/users` route with a table page, three dialog forms (create user, deactivate confirmation, reset password), and a row action dropdown menu. The established Phase 26 pattern (Dialog + react-hook-form + zod + Sonner toast) applies directly. Several new shadcn/ui components must be added: Table, Badge, DropdownMenu, Select, and AlertDialog.

**Primary recommendation:** Build backend endpoints first (simple additions to existing patterns), then frontend page with table + dialogs following Phase 26 patterns exactly.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- Table columns: Username, Role (badge), Status (active/inactive badge), Created date
- Row actions via three-dot dropdown menu: Deactivate/Reactivate/Reset Password
- Hide action dropdown for currently logged-in admin (prevent self-deactivation)
- Admin-only nav: hide "Users" sidebar item for non-admin users
- Create user dialog: required fields (Username, Password, Role dropdown), optional (Display Name, Email), default role "viewer"
- Password validation: minimum 8 characters
- Deactivate: confirmation dialog with destructive (red) button
- Reactivate: immediate action, no confirmation, success toast
- Backend: existing DELETE /users/{id} for deactivation; NEW PATCH /users/{id}/reactivate; NEW PUT /users/{id}/password
- Admin password reset: dialog with New Password + Confirm Password, no current_password required

### Claude's Discretion
- Table component choice (shadcn/ui Table or custom)
- Loading/empty states for the users table
- Pagination or scroll strategy if user count grows
- Exact badge colors for roles and status

### Deferred Ideas (OUT OF SCOPE)
None
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| USER-01 | Admin can view all users in a table | GET /users already exists; frontend needs Table component + UsersPage |
| USER-02 | Admin can create a new user | POST /users already exists; frontend needs CreateUserDialog |
| USER-03 | Admin can deactivate/reactivate a user | DELETE /users/{id} exists for deactivation; NEW PATCH /users/{id}/reactivate needed; frontend needs DeactivateDialog + reactivate action |
| USER-04 | Admin can reset another user's password | NEW PUT /users/{id}/password endpoint needed; frontend needs ResetPasswordDialog |
</phase_requirements>

## Standard Stack

### Core (already installed)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| React | 19.x | UI framework | Already in project |
| @tanstack/react-query | 5.x | Server state management | Already configured with auth error handling |
| react-hook-form | 7.x | Form state | Already used in ChangePasswordDialog |
| zod | 4.x | Schema validation | Already used in Phase 26 |
| sonner | 1.x | Toast notifications | Already configured |
| lucide-react | 0.475+ | Icons | Already in project |
| go-chi/chi | v5 | Go HTTP router | Already used for all endpoints |

### New shadcn/ui Components Needed
| Component | Purpose | Install Command |
|-----------|---------|-----------------|
| Table | User list display | `npx shadcn@latest add table` |
| Badge | Role and status indicators | `npx shadcn@latest add badge` |
| DropdownMenu | Row action menu (three-dot) | `npx shadcn@latest add dropdown-menu` |
| Select | Role dropdown in create form | `npx shadcn@latest add select` |
| AlertDialog | Deactivate confirmation | `npx shadcn@latest add alert-dialog` |

**Installation:**
```bash
cd web && npx shadcn@latest add table badge dropdown-menu select alert-dialog
```

## Architecture Patterns

### Recommended Project Structure
```
web/src/features/users/
  UsersPage.tsx              # Main page with table + "Create User" button
  CreateUserDialog.tsx       # Dialog form for new user
  ResetPasswordDialog.tsx    # Dialog form for admin password reset
  DeactivateDialog.tsx       # AlertDialog for deactivation confirmation
  useUsers.ts                # React Query hooks (useUsers, useCreateUser, etc.)
internal/server/
  handlers_auth.go           # Add handleReactivateUser, handleAdminResetPassword
internal/api/types.go        # Add AdminResetPasswordRequest
```

### Pattern 1: React Query Mutations with Table Invalidation
**What:** Each mutation (create, deactivate, reactivate, reset password) invalidates the `["users"]` query key to auto-refresh the table.
**When to use:** All write operations on user data.
**Example:**
```typescript
// useUsers.ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '@/lib/apiClient'

interface UserResponse {
  id: string
  username: string
  display_name: string
  email: string
  global_role: string
  is_active: boolean
  created_at: string
  updated_at: string
}

export function useUsers() {
  return useQuery<UserResponse[]>({
    queryKey: ['users'],
    queryFn: () => apiClient.get('/users'),
  })
}

export function useCreateUser() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: CreateUserRequest) => apiClient.post('/users', data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['users'] }),
  })
}
```

### Pattern 2: apiClient Extension for DELETE and PATCH
**What:** The current apiClient only has `get`, `post`, `put`. Need to add `delete` and `patch` methods.
**When to use:** Deactivation (DELETE) and reactivation (PATCH).
**Example:**
```typescript
// Add to apiClient.ts
export const apiClient = {
  get: <T>(path: string) => request<T>(path, { method: 'GET' }),
  post: <T>(path: string, body: unknown) =>
    request<T>(path, { method: 'POST', body: JSON.stringify(body) }),
  put: <T>(path: string, body: unknown) =>
    request<T>(path, { method: 'PUT', body: JSON.stringify(body) }),
  delete: <T>(path: string) => request<T>(path, { method: 'DELETE' }),
  patch: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'PATCH', ...(body ? { body: JSON.stringify(body) } : {}) }),
}
```

### Pattern 3: Conditional Sidebar Navigation
**What:** Show "Users" nav item only when `authStore.role === "admin"`.
**When to use:** Admin-only UI gating.
**Example:**
```typescript
// In Sidebar.tsx, filter globalNavItems based on role
const role = useAuthStore((s) => s.role)
// Only render globalNavItems section when role === 'admin'
```

### Pattern 4: Backend Handler for Reactivation
**What:** New endpoint `PATCH /users/{id}/reactivate` using existing `UpdateUser` store method or new `ReactivateUser`.
**Example:**
```go
func (s *Server) handleReactivateUser(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    user, err := s.store.GetUserByID(r.Context(), id)
    if err != nil {
        writeError(w, http.StatusNotFound, "user not found")
        return
    }
    user.IsActive = true
    if err := s.store.UpdateUser(r.Context(), user); err != nil {
        writeError(w, http.StatusInternalServerError, "failed to reactivate user")
        return
    }
    writeJSON(w, http.StatusOK, userToResponse(user))
}
```

### Pattern 5: Backend Handler for Admin Password Reset
**What:** New endpoint `PUT /users/{id}/password` (admin only, no current_password required).
**Example:**
```go
// New request type in api/types.go
type AdminResetPasswordRequest struct {
    NewPassword string `json:"new_password"`
}

func (s *Server) handleAdminResetPassword(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    var req api.AdminResetPasswordRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }
    user, err := s.store.GetUserByID(r.Context(), id)
    if err != nil {
        writeError(w, http.StatusNotFound, "user not found")
        return
    }
    hash, err := auth.HashPassword(req.NewPassword, auth.DefaultBcryptCost)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to reset password")
        return
    }
    user.PasswordHash = hash
    if err := s.store.UpdateUser(r.Context(), user); err != nil {
        writeError(w, http.StatusInternalServerError, "failed to reset password")
        return
    }
    w.WriteHeader(http.StatusNoContent)
}
```

### Anti-Patterns to Avoid
- **Custom table implementation:** Use shadcn/ui Table component, not a hand-rolled HTML table. It provides consistent styling and accessibility.
- **Separate store methods when UpdateUser suffices:** `UpdateUser` already handles `is_active` and `password_hash` changes. Adding a `ReactivateUser` store method is optional for clarity but functionally `UpdateUser` works.
- **Forgetting to hide self-action:** The logged-in admin must not see the action dropdown on their own row. Compare `user.id` with the current user's ID from JWT claims.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Data table | Custom table HTML/CSS | shadcn/ui Table | Accessible, styled, consistent with project |
| Dropdown menu | Custom popover + buttons | shadcn/ui DropdownMenu | Handles focus, keyboard nav, outside click |
| Confirmation dialog | window.confirm() | shadcn/ui AlertDialog | Consistent styling, accessible, custom buttons |
| Role selector | Custom dropdown | shadcn/ui Select | Accessible, keyboard-navigable |
| Badge/pills | Custom spans | shadcn/ui Badge | Consistent color variants |
| Form validation | Manual if/else | zod + react-hook-form | Already established pattern |

**Key insight:** Every UI primitive needed is available as a shadcn/ui component. The only custom code is the page layout, data fetching hooks, and business logic for which rows show which actions.

## Common Pitfalls

### Pitfall 1: Self-Deactivation
**What goes wrong:** Admin deactivates their own account and gets locked out.
**Why it happens:** No check comparing action target with current user.
**How to avoid:** Hide the action dropdown for the row where `user.id === currentUserId`. The current user's ID is available from the JWT claims (need to decode from token or add `userId` to authStore).
**Warning signs:** Action menu visible on own row during manual testing.

### Pitfall 2: Missing apiClient Methods
**What goes wrong:** Frontend can't call DELETE or PATCH endpoints.
**Why it happens:** apiClient currently only has `get`, `post`, `put` -- no `delete` or `patch`.
**How to avoid:** Add `delete` and `patch` methods to apiClient BEFORE implementing mutation hooks.
**Warning signs:** TypeScript errors when trying to call `apiClient.delete()`.

### Pitfall 3: Route Registration Order in Chi
**What goes wrong:** `/users/{id}/reactivate` or `/users/{id}/password` conflicts with `/users/{id}` or `/users/me`.
**Why it happens:** Chi routes are matched in registration order; `/users/me` must be registered before `/users/{id}` routes.
**How to avoid:** Register specific paths (`/users/me`, `/users/me/password`) before parameterized paths (`/users/{id}`, `/users/{id}/reactivate`, `/users/{id}/password`).
**Warning signs:** 404 or wrong handler invoked during testing.

### Pitfall 4: Admin Role Not Available for UI Gating
**What goes wrong:** Sidebar shows "Users" to non-admin users, or table page is accessible without admin role.
**Why it happens:** Only checking backend authorization, not frontend gating.
**How to avoid:** authStore already has `role`. Use it in Sidebar.tsx to conditionally render the "Users" nav item, and optionally redirect non-admins from /users route.
**Warning signs:** Non-admin user sees "Users" in sidebar.

### Pitfall 5: Current User ID Not in AuthStore
**What goes wrong:** Cannot determine which table row is the current user to hide actions.
**Why it happens:** authStore stores `token`, `username`, `role` but NOT `userId`.
**How to avoid:** Either add `userId` to authStore (set during login from JWT decode or /users/me response), OR compare by `username` since that's also unique.
**Warning signs:** Cannot identify self-row in the users table.

## Code Examples

### Existing ChangePasswordDialog Pattern (reuse for all dialogs)
```typescript
// Pattern: Dialog + react-hook-form + zod + Sonner toast
// Source: web/src/features/auth/ChangePasswordDialog.tsx
// 1. Define zod schema
// 2. useForm with zodResolver
// 3. onSubmit calls apiClient, toast.success, form.reset, onOpenChange(false)
// 4. Catch ApiError for inline error display
```

### Backend Route Registration Pattern
```go
// Source: internal/server/server.go lines 119-124
// New routes go inside the JWT+admin group:
r.With(roleRequired("admin")).Patch("/users/{id}/reactivate", s.handleReactivateUser)
r.With(roleRequired("admin")).Put("/users/{id}/password", s.handleAdminResetPassword)
```

### shadcn/ui Table Usage
```typescript
// After: npx shadcn@latest add table
import {
  Table, TableBody, TableCell, TableHead,
  TableHeader, TableRow,
} from '@/components/ui/table'

<Table>
  <TableHeader>
    <TableRow>
      <TableHead>Username</TableHead>
      <TableHead>Role</TableHead>
      <TableHead>Status</TableHead>
      <TableHead>Created</TableHead>
      <TableHead className="w-[50px]" />
    </TableRow>
  </TableHeader>
  <TableBody>
    {users.map((user) => (
      <TableRow key={user.id}>
        <TableCell>{user.username}</TableCell>
        <TableCell><Badge>{user.global_role}</Badge></TableCell>
        <TableCell>
          <Badge variant={user.is_active ? 'default' : 'secondary'}>
            {user.is_active ? 'Active' : 'Inactive'}
          </Badge>
        </TableCell>
        <TableCell>{formatDate(user.created_at)}</TableCell>
        <TableCell>{/* DropdownMenu actions */}</TableCell>
      </TableRow>
    ))}
  </TableBody>
</Table>
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Separate store methods per operation | Reuse UpdateUser for reactivation + password reset | Current design | Fewer store interface changes |
| window.confirm for destructive actions | AlertDialog component | shadcn/ui standard | Better UX, consistent styling |

**Note:** The project uses zod v4 (4.3.6). This is a recent major version. The `z.object()` and `.refine()` APIs used in Phase 26 remain the same in v4.

## Discretion Recommendations

### Table Component: Use shadcn/ui Table (not @tanstack/react-table)
**Recommendation:** Plain shadcn/ui Table. The user count in a flowy deployment is expected to be small (tens, not thousands). A simple static table is sufficient. @tanstack/react-table adds unnecessary complexity for sorting/pagination that isn't needed.
**Confidence:** HIGH

### Loading/Empty States
**Recommendation:** 
- Loading: Skeleton rows or a centered Loader2 spinner (consistent with Phase 26 submit button pattern)
- Empty: Simple centered text "No users found" within the table body
**Confidence:** HIGH

### Pagination
**Recommendation:** No pagination for v2.0. Show all users. The API returns all users already (no pagination params). If user count grows beyond ~100, pagination can be added later.
**Confidence:** HIGH

### Badge Colors
**Recommendation:**
- Role badges: `variant="outline"` for all roles (neutral, distinguishable by text)
- Status: `variant="default"` (filled) for Active, `variant="secondary"` (muted) for Inactive
- Alternatively for destructive feel: Inactive could use `variant="destructive"` but secondary is less alarming
**Confidence:** MEDIUM -- designer preference, easy to change

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework (frontend) | vitest 3.2.4 + @testing-library/react 16.3.2 |
| Framework (backend) | Go testing + httptest |
| Config file (frontend) | web/vitest.config.ts |
| Quick run command (frontend) | `cd web && npx vitest run --reporter=verbose` |
| Quick run command (backend) | `go test ./internal/server/ -run TestReactivate -v` |
| Full suite command | `cd web && npx vitest run && cd .. && go test ./internal/...` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| USER-01 | Users table renders list from API | unit (frontend) | `cd web && npx vitest run src/features/users/__tests__/UsersPage.test.tsx` | Wave 0 |
| USER-02 | Create user dialog submits and refreshes | unit (frontend) | `cd web && npx vitest run src/features/users/__tests__/CreateUserDialog.test.tsx` | Wave 0 |
| USER-02 | POST /users handler creates user | unit (backend) | `go test ./internal/server/ -run TestCreateUser -v` | Exists (handlers_auth_test.go) |
| USER-03 | Deactivate/reactivate changes user status | unit (backend) | `go test ./internal/server/ -run TestReactivateUser -v` | Wave 0 |
| USER-03 | Deactivate confirmation dialog flow | unit (frontend) | `cd web && npx vitest run src/features/users/__tests__/UsersPage.test.tsx` | Wave 0 |
| USER-04 | Admin reset password endpoint | unit (backend) | `go test ./internal/server/ -run TestAdminResetPassword -v` | Wave 0 |
| USER-04 | Reset password dialog submits | unit (frontend) | `cd web && npx vitest run src/features/users/__tests__/ResetPasswordDialog.test.tsx` | Wave 0 |

### Sampling Rate
- **Per task commit:** `cd web && npx vitest run` (frontend) + `go test ./internal/server/ -v` (backend)
- **Per wave merge:** Full suite
- **Phase gate:** Full suite green before /gsd:verify-work

### Wave 0 Gaps
- [ ] `web/src/features/users/__tests__/UsersPage.test.tsx` -- covers USER-01, USER-03 (frontend)
- [ ] `web/src/features/users/__tests__/CreateUserDialog.test.tsx` -- covers USER-02 (frontend)
- [ ] `web/src/features/users/__tests__/ResetPasswordDialog.test.tsx` -- covers USER-04 (frontend)
- [ ] Backend tests for new endpoints in `internal/server/handlers_auth_test.go` -- covers USER-03 (reactivate), USER-04 (admin reset)

## Open Questions

1. **Current user ID for self-row detection**
   - What we know: authStore has `username` and `role` but no `userId`. The users table API returns `id` per user.
   - What's unclear: Whether to add `userId` to authStore or compare by `username`.
   - Recommendation: Compare by `username` -- it is unique and already in authStore. No store changes needed.

2. **Backend password validation**
   - What we know: Frontend validates min 8 chars via zod. Backend `handleCreateUser` does not validate password length.
   - What's unclear: Whether to add server-side password length validation.
   - Recommendation: Add server-side validation (min 8 chars) in both create user and admin reset password handlers for defense in depth. Low effort, high value.

## Sources

### Primary (HIGH confidence)
- `internal/server/handlers_auth.go` -- existing user CRUD handlers, route patterns
- `internal/api/types.go` -- existing request/response DTOs
- `internal/store/store.go` -- Store interface, User model, existing methods
- `internal/store/sqlite/users.go` -- UpdateUser implementation (handles is_active + password_hash)
- `internal/server/server.go` -- Route registration with roleRequired middleware
- `web/src/features/auth/ChangePasswordDialog.tsx` -- Dialog + form pattern
- `web/src/lib/apiClient.ts` -- API client (needs delete/patch methods)
- `web/src/store/authStore.ts` -- Auth store with role for admin gating
- `web/src/components/layout/Sidebar.tsx` -- Navigation structure (globalNavItems already has Users placeholder)
- `web/package.json` -- Current dependency versions

### Secondary (MEDIUM confidence)
- shadcn/ui component install commands (based on standard shadcn CLI patterns)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all libraries already installed; only new shadcn/ui components to add
- Architecture: HIGH -- extends existing patterns directly (handlers, dialogs, store methods)
- Pitfalls: HIGH -- identified from reading actual codebase (missing apiClient methods, route ordering, self-deactivation)

**Research date:** 2026-04-06
**Valid until:** 2026-05-06 (stable -- internal admin UI, no external API dependencies)
