---
phase: 19-user-authentication
plan: "02"
subsystem: auth-implementation
tags: [auth, jwt, bcrypt, middleware, handlers, router]
dependency_graph:
  requires:
    - 19-01 (AuthConfig, User model, Store interface user methods)
  provides:
    - JWT issue/validate via internal/auth package
    - JWTMiddleware and DaemonAuthMiddleware in internal/server/middleware.go
    - 6 auth HTTP handlers in internal/server/handlers_auth.go
    - Restructured newRouter with 3 conditional route groups in internal/server/server.go
    - Auth API types in internal/api/types.go
  affects:
    - internal/store/store.go (UpdateUser, GetUserByUsernameAny added)
    - internal/store/sqlite/users.go (UpdateUser, GetUserByUsernameAny implemented)
tech_stack:
  added:
    - github.com/golang-jwt/jwt/v5 v5.3.1
  patterns:
    - TDD: RED (missing package/types) -> GREEN (implementation) for both tasks
    - Minimal Store interface in auth package to avoid import cycles
    - Constant-time comparison for daemon token (crypto/subtle)
    - GetUserByUsernameAny for login to distinguish inactive vs not-found
    - Three route groups: public / JWT-protected / daemon-protected
key_files:
  created:
    - internal/auth/auth.go
    - internal/auth/auth_test.go
    - internal/server/middleware.go
    - internal/server/handlers_auth.go
    - internal/server/handlers_auth_test.go
  modified:
    - internal/api/types.go
    - internal/server/server.go
    - internal/store/store.go
    - internal/store/sqlite/users.go
decisions:
  - "GetUserByUsernameAny added to Store interface — login handler uses it to distinguish inactive (account disabled) from non-existent (invalid credentials) without username enumeration risk on the specific error message"
  - "IssueTokenWithExpiry exported separately to allow test flexibility (negative duration = pre-expired tokens) while keeping IssueToken as the production convenience wrapper"
  - "UpdateUser added to Store interface to support handleChangePassword — cleaner than a dedicated UpdatePassword method since user updates may grow"
  - "Router restructure done in Task 2 commit alongside middleware/handlers for atomicity — tasks 2 and 3 are tightly coupled via newRouter referencing middleware and handler methods"
metrics:
  duration: "386 seconds"
  completed_date: "2026-03-17"
  tasks_completed: 3
  tasks_total: 3
  files_created: 5
  files_modified: 4
---

# Phase 19 Plan 02: JWT Auth Implementation Summary

**One-liner:** JWT auth package (HS256 issue/validate, bcrypt helpers, auto-secret), JWTMiddleware + DaemonAuthMiddleware, 6 auth handlers (login/user CRUD/me/password), and router restructured into 3 conditional route groups — zero behavioral change when auth.enabled=false.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Create auth package with JWT and bcrypt helpers | b5a7a89 | internal/auth/auth.go, internal/auth/auth_test.go |
| 2 | Create middleware and auth handlers | 7f37333 | internal/server/middleware.go, handlers_auth.go, handlers_auth_test.go, internal/api/types.go, internal/store/store.go, internal/store/sqlite/users.go |
| 3 | Restructure newRouter with conditional auth route groups | 7f37333 | internal/server/server.go |

## What Was Built

### Auth Package (internal/auth/auth.go)

- `Claims` struct with UserID, Username, Role, and jwt.RegisteredClaims
- `UserClaimsKey` context key (unexported type) for middleware injection
- `IssueToken(secret, userID, username, role)` — HS256 JWT, 7-day expiry
- `IssueTokenWithExpiry` — exported for test flexibility (negative duration = expired)
- `ValidateToken(secret, tokenStr)` — parses and validates, rejects wrong-secret/expired/malformed
- `HashPassword(password, cost)` — bcrypt wrapper, DefaultBcryptCost=12
- `CheckPassword(hash, password)` — bcrypt comparison
- `ResolveJWTSecret(ctx, configSecret, store)` — config → DB → auto-generate (32 random bytes hex-encoded)
- Minimal `Store` interface (GetServerConfig/SetServerConfig only) to avoid import cycles

### Middleware (internal/server/middleware.go)

- `JWTMiddleware(secret)` — extracts Bearer token, validates via auth.ValidateToken, injects claims into context; 401 on missing/invalid
- `DaemonAuthMiddleware(daemonToken)` — constant-time comparison (crypto/subtle); 401 on mismatch
- `roleRequired(role)` — reads claims from context, 403 if role does not match; used as `r.With(roleRequired("admin"))`
- `claimsFromContext(ctx)` — helper for handlers to retrieve injected claims
- `extractBearerToken(r)` — parses "Bearer <token>" format

### Auth Handlers (internal/server/handlers_auth.go)

- `handleLogin` — uses GetUserByUsernameAny to distinguish inactive (account disabled) from not-found (invalid credentials); issues JWT on success
- `handleCreateUser` — admin-only, bcrypt hash, UUID v4, returns UserResponse 201
- `handleListUsers` — admin-only, returns []UserResponse 200
- `handleDeleteUser` — admin-only, soft-delete via DeactivateUser, 204
- `handleGetMe` — reads claims from context, returns own profile 200
- `handleChangePassword` — verifies current password, updates hash via UpdateUser, 204
- `handleLoginDisabled` — returns 200 with empty token when auth.enabled=false (no-op)

### Router Restructure (internal/server/server.go)

Three conditional route groups when auth.enabled=true:
1. **Public**: GET /health, POST /auth/login (no middleware)
2. **JWT-protected**: workflow routes + user management (JWTMiddleware)
   - Admin-only: POST/GET /users, DELETE /users/{id} (roleRequired("admin"))
   - All users: GET /users/me, PUT /users/me/password
3. **Daemon-protected**: task/heartbeat routes (DaemonAuthMiddleware)

When auth.enabled=false: flat registration identical to pre-auth behavior.

Server.New() calls auth.ResolveJWTSecret only when cfg.Auth.Enabled=true; s.jwtSecret remains nil when disabled.

### API Types (internal/api/types.go)

Added: LoginRequest, LoginResponse, CreateUserRequest, UserResponse, ChangePasswordRequest.

### Store Additions (Rule 2 — missing critical functionality)

- `UpdateUser(ctx, *User)` — added to Store interface and SQLiteStore; updates password_hash, display_name, email, global_role, is_active
- `GetUserByUsernameAny(ctx, username)` — added to Store interface and SQLiteStore; lookups without is_active=1 filter for login disambiguation

## Tests

**internal/auth** (8 tests):
- TestIssueAndValidateToken, TestValidateToken_Expired, TestValidateToken_WrongSecret, TestValidateToken_Malformed
- TestHashAndCheckPassword
- TestResolveJWTSecret_FromConfig, TestResolveJWTSecret_AutoGenerate, TestResolveJWTSecret_FromDB

**internal/server** (13 new auth tests + 20 existing tests = 33 total):
- TestHandleLogin_Success/InvalidPassword/InactiveUser/UnknownUser
- TestHandleCreateUser_Admin/NonAdmin
- TestHandleListUsers_Admin
- TestHandleDeleteUser_Admin
- TestHandleGetMe
- TestHandleChangePassword
- TestJWTMiddleware_NoHeader/InvalidToken
- TestDaemonAuthMiddleware_ValidToken/InvalidToken
- All 20 pre-existing server tests pass unchanged (auth.enabled=false default)

## Verification

```
go test ./internal/auth/... -v -timeout 30s           # 8/8 PASS
go test ./internal/server/... -timeout 60s            # 33/33 PASS
go test ./... -timeout 120s                           # All 22 packages PASS
```

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical Functionality] Added UpdateUser to Store interface**
- **Found during:** Task 2 (handleChangePassword implementation)
- **Issue:** handleChangePassword needs to persist the new password hash, but Store had no method to update a user record
- **Fix:** Added UpdateUser(ctx, *User) to store.Store interface and implemented in SQLiteStore
- **Files modified:** internal/store/store.go, internal/store/sqlite/users.go
- **Commit:** 7f37333

**2. [Rule 2 - Missing Critical Functionality] Added GetUserByUsernameAny to Store interface**
- **Found during:** Task 2 (TestHandleLogin_InactiveUser failing — returned "invalid credentials" instead of "account disabled")**
- **Issue:** GetUserByUsername filters is_active=1, making it impossible to distinguish inactive accounts from non-existent ones in the login handler
- **Fix:** Added GetUserByUsernameAny(ctx, username) that queries without is_active filter; updated handleLogin to use it
- **Files modified:** internal/store/store.go, internal/store/sqlite/users.go, internal/server/handlers_auth.go
- **Commit:** 7f37333

**3. [Rule 3 - Blocking Issue] Tasks 2 and 3 committed together**
- **Found during:** Task 2 execution
- **Issue:** newRouter() references JWTMiddleware, DaemonAuthMiddleware, roleRequired, handleLogin, handleGetMe, etc. — all Task 2 artifacts. Task 3 cannot be implemented separately without those handlers existing first.
- **Fix:** Implemented router restructure (Task 3) atomically with the handlers in the same commit
- **Files modified:** internal/server/server.go
- **Commit:** 7f37333

## Self-Check

Files exist:
- internal/auth/auth.go — FOUND
- internal/auth/auth_test.go — FOUND
- internal/server/middleware.go — FOUND
- internal/server/handlers_auth.go — FOUND
- internal/server/handlers_auth_test.go — FOUND
- internal/api/types.go — FOUND (modified)
- internal/server/server.go — FOUND (modified)
- internal/store/store.go — FOUND (modified)
- internal/store/sqlite/users.go — FOUND (modified)

Commits exist:
- b5a7a89 feat(19-02): create auth package with JWT and bcrypt helpers — FOUND
- 7f37333 feat(19-02): create middleware, auth handlers, and restructured router — FOUND

## Self-Check: PASSED
