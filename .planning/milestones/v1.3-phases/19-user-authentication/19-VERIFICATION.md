---
phase: 19-user-authentication
verified: 2026-03-17T00:00:00Z
status: passed
score: 18/18 must-haves verified
re_verification: false
---

# Phase 19: User Authentication Verification Report

**Phase Goal:** Users can securely authenticate with the server and all non-public routes are protected by JWT tokens
**Verified:** 2026-03-17
**Status:** PASSED
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | AuthConfig struct exists with Enabled (default false), JWTSecret, and DaemonToken fields | VERIFIED | `internal/config/config.go` lines 38-49; `Auth AuthConfig` field on Config struct at line 67 |
| 2 | User model and store methods are defined in the Store interface | VERIFIED | `internal/store/store.go`: User struct lines 37-47; 9 user methods + 2 server_config methods in Store interface lines 131-155 |
| 3 | SQLite migration 00004 creates users and server_config tables | VERIFIED | `internal/store/sqlite/migrations/00004_users.sql`: CREATE TABLE users and server_config with correct schema |
| 4 | SQLite store implements all user CRUD and server_config methods | VERIFIED | `internal/store/sqlite/users.go` (CreateUser, GetUserByUsername, GetUserByUsernameAny, GetUserByID, ListUsers, DeactivateUser, UpdateUser, CountUsers) and `server_config.go` (GetServerConfig, SetServerConfig) |
| 5 | Existing tests pass unchanged (auth.enabled defaults false) | VERIFIED | All 22 packages pass with `go test ./...` |
| 6 | POST /auth/login with valid credentials returns a signed JWT token | VERIFIED | `handleLogin` in `handlers_auth.go` uses `auth.IssueToken`; `TestHandleLogin_Success` PASS |
| 7 | POST /auth/login with invalid credentials returns 401 | VERIFIED | `handleLogin` returns 401 "invalid credentials" for unknown user and wrong password; `TestHandleLogin_InvalidPassword` and `TestHandleLogin_UnknownUser` PASS |
| 8 | Protected routes without valid JWT return 401 Unauthorized | VERIFIED | `JWTMiddleware` in `middleware.go` returns 401 on missing/invalid token; `TestJWTMiddleware_NoHeader` and `TestJWTMiddleware_InvalidToken` PASS |
| 9 | Protected routes with valid JWT succeed | VERIFIED | `TestHandleGetMe`, `TestHandleListUsers_Admin`, `TestHandleCreateUser_Admin` all PASS with valid JWT |
| 10 | Admin can POST /users to create a new user | VERIFIED | `handleCreateUser` with `roleRequired("admin")` in router; `TestHandleCreateUser_Admin` PASS |
| 11 | Non-admin POST /users returns 403 | VERIFIED | `roleRequired("admin")` middleware returns 403; `TestHandleCreateUser_NonAdmin` PASS |
| 12 | Admin can GET /users to list users and DELETE /users/{id} to soft-delete | VERIFIED | `handleListUsers` and `handleDeleteUser` behind `roleRequired("admin")`; `TestHandleListUsers_Admin` and `TestHandleDeleteUser_Admin` PASS |
| 13 | GET /users/me returns own profile | VERIFIED | `handleGetMe` reads claims from context, returns UserResponse; `TestHandleGetMe` PASS |
| 14 | PUT /users/me/password changes own password | VERIFIED | `handleChangePassword` verifies current password, hashes new, calls `UpdateUser`; `TestHandleChangePassword` PASS |
| 15 | When auth.enabled=false, all routes work without auth headers | VERIFIED | `newRouter()` else-branch registers all routes flat with no middleware; all 20 pre-existing server tests pass unchanged |
| 16 | Daemon routes use DaemonAuthMiddleware, not JWTMiddleware | VERIFIED | `server.go` lines 121-127: daemon routes in separate Group with `DaemonAuthMiddleware`; `TestDaemonAuthMiddleware_ValidToken` and `TestDaemonAuthMiddleware_InvalidToken` PASS |
| 17 | --init-admin with --username and --password creates an admin user and exits | VERIFIED | `runInitAdmin` in `initadmin.go`; `--init-admin` path in `main.go` calls `runInitAdmin` then `os.Exit(0)` before `server.New()`; `TestRunInitAdmin_Success` PASS |
| 18 | Daemon sends Bearer token on all requests when auth.daemon_token configured | VERIFIED | `bearerTransport` in `daemon.go` injects `Authorization: Bearer` header; `daemon.New()` wraps transport when token non-empty; `TestBearerTransport_WithToken` and `TestDaemonNew_WithToken` PASS |

**Score:** 18/18 truths verified

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/config/config.go` | AuthConfig sub-struct with yaml tags | VERIFIED | AuthConfig{Enabled, JWTSecret, DaemonToken} at lines 38-49; Auth field on Config at line 67 |
| `internal/store/store.go` | User model + user store methods in Store interface | VERIFIED | User struct + 9 user methods (including UpdateUser, GetUserByUsernameAny added as plan deviations) + 2 server_config methods |
| `internal/store/sqlite/migrations/00004_users.sql` | users table + server_config table migration | VERIFIED | Both CREATE TABLE statements with correct columns, UNIQUE constraint on username, index on username |
| `internal/store/sqlite/users.go` | SQLite implementation of user store methods | VERIFIED | 8 user methods fully implemented including UpdateUser and GetUserByUsernameAny; rowScanner interface for testability |
| `internal/store/sqlite/server_config.go` | GetServerConfig and SetServerConfig implementations | VERIFIED | GetServerConfig with sql.ErrNoRows wrapping; SetServerConfig with INSERT OR REPLACE upsert |
| `internal/auth/auth.go` | JWT issue/validate, bcrypt helpers, secret resolution | VERIFIED | IssueToken, IssueTokenWithExpiry, ValidateToken, HashPassword, CheckPassword, ResolveJWTSecret all present |
| `internal/server/middleware.go` | JWTMiddleware, DaemonAuthMiddleware, roleRequired | VERIFIED | All three middleware functions implemented; claimsFromContext and extractBearerToken helpers present |
| `internal/server/handlers_auth.go` | Login, user CRUD, me, password change handlers | VERIFIED | handleLogin, handleCreateUser, handleListUsers, handleDeleteUser, handleGetMe, handleChangePassword, handleLoginDisabled all present |
| `internal/server/server.go` | Restructured newRouter with 3 route groups | VERIFIED | Three conditional groups (public, JWT-protected, daemon-protected) when auth.enabled=true; flat when disabled |
| `cmd/flowy-server/initadmin.go` | Init admin logic as testable function | VERIFIED | runInitAdmin function with InitAdminStore minimal interface and storeAdapter pattern |
| `cmd/flowy-server/main.go` | --init-admin, --username, --password flags | VERIFIED | All three flags registered; init-admin path runs before server.New() call |
| `internal/daemon/daemon.go` | bearerTransport for token injection | VERIFIED | bearerTransport struct with RoundTrip cloning request; New() conditionally wraps transport |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/store/store.go` | `internal/store/sqlite/users.go` | Store interface implementation | VERIFIED | `func (s *SQLiteStore) CreateUser` and all other methods implement Store interface |
| `internal/config/config.go` | `internal/server/server.go` | Config.Auth field consumed by server | VERIFIED | `s.cfg.Auth.Enabled` checked in `New()` and `newRouter()`; `cfg.Auth.JWTSecret` passed to `ResolveJWTSecret` |
| `internal/server/middleware.go` | `internal/auth/auth.go` | ValidateToken call in JWTMiddleware | VERIFIED | `auth.ValidateToken(secret, tokenStr)` at middleware.go line 24 |
| `internal/server/handlers_auth.go` | `internal/auth/auth.go` | IssueToken in handleLogin, HashPassword in handleCreateUser | VERIFIED | `auth.IssueToken` at handlers_auth.go line 46; `auth.HashPassword` at line 69 |
| `internal/server/handlers_auth.go` | `internal/store/store.go` | Store methods for user CRUD | VERIFIED | `s.store.GetUserByUsernameAny`, `s.store.CreateUser`, `s.store.ListUsers`, `s.store.DeactivateUser` all called in handlers |
| `internal/server/server.go` | `internal/server/middleware.go` | r.Use(JWTMiddleware) and r.Use(DaemonAuthMiddleware) in route groups | VERIFIED | `r.Use(JWTMiddleware(s.jwtSecret))` at server.go line 104; `r.Use(DaemonAuthMiddleware(s.cfg.Auth.DaemonToken))` at line 122 |
| `cmd/flowy-server/main.go` | `cmd/flowy-server/initadmin.go` | runInitAdmin call before server start | VERIFIED | `runInitAdmin(context.Background(), &storeAdapter{st: st}, iac)` at main.go line 62; occurs before `server.New()` at line 72 |
| `internal/daemon/daemon.go` | `internal/config/config.go` | cfg.Auth.DaemonToken used in bearerTransport | VERIFIED | `cfg.Auth.DaemonToken != ""` check in `New()` at daemon.go line 60 |

---

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| AUTH-01 | 19-02 | User can log in with username and password, receiving a JWT token | SATISFIED | `handleLogin` returns signed JWT on valid credentials; `TestHandleLogin_Success` PASS |
| AUTH-02 | 19-02 | All API routes (except login and health) are protected by JWT middleware | SATISFIED | Router groups all workflow + user management routes under `JWTMiddleware`; daemon routes under `DaemonAuthMiddleware`; `TestJWTMiddleware_NoHeader` PASS |
| AUTH-03 | 19-03 | Server seeds an initial admin user on first run | SATISFIED | `--init-admin` CLI flag creates admin user and exits; `TestRunInitAdmin_Success` PASS; errors on pre-existing users |
| AUTH-04 | 19-02 | Admin can create new users with username and password | SATISFIED | `POST /users` behind `roleRequired("admin")`; `TestHandleCreateUser_Admin` PASS; `TestHandleCreateUser_NonAdmin` returns 403 |
| AUTH-05 | 19-02 | Admin can list and delete users | SATISFIED | `GET /users` and `DELETE /users/{id}` behind `roleRequired("admin")`; `TestHandleListUsers_Admin` and `TestHandleDeleteUser_Admin` PASS |
| AUTH-06 | 19-01, 19-02, 19-03 | Auth can be disabled via config flag (auth.enabled: false) for backward compatibility | SATISFIED | `Auth.Enabled` defaults to false (bool zero value); else-branch in `newRouter()` registers all routes flat with no middleware; all 20 pre-existing server tests pass unchanged; daemon token injection conditional on `cfg.Auth.DaemonToken != ""` |

All 6 requirements (AUTH-01 through AUTH-06) satisfied. No orphaned requirements detected.

---

### Anti-Patterns Found

None. All key files examined:

- `internal/auth/auth.go` — no TODOs, no stubs, no empty returns
- `internal/server/handlers_auth.go` — all handlers fully implemented; `handleLoginDisabled` intentionally returns 200 with empty token (correct behavior for disabled auth)
- `internal/server/middleware.go` — no stubs
- `internal/server/server.go` — router fully wired
- `cmd/flowy-server/initadmin.go` — fully implemented
- `internal/daemon/daemon.go` — bearerTransport fully implemented

---

### Human Verification Required

None identified. All critical behaviors are covered by automated tests:

- Login flow (success, wrong password, inactive user, unknown user)
- JWT middleware enforcement (missing header, invalid token, valid token)
- Daemon auth middleware (valid token, invalid token)
- Role enforcement (admin succeeds, non-admin gets 403)
- Password change (verifies current, updates hash)
- --init-admin (creates user, errors on duplicate, validates bcrypt hash)
- Bearer token injection in daemon (with and without token)
- Backward compatibility when auth.enabled=false (all 20 pre-existing server tests pass)

---

### Commits Verified

All 6 documented commits exist in git history:

| Commit | Message |
|--------|---------|
| 3f77894 | feat(19-01): add AuthConfig struct and User model with store interface methods |
| 66eb254 | feat(19-01): add migration 00004 and SQLite user/server_config implementations |
| b5a7a89 | feat(19-02): create auth package with JWT and bcrypt helpers |
| 7f37333 | feat(19-02): create middleware, auth handlers, and restructured router |
| dc6306f | feat(19-03): implement --init-admin CLI command for admin user seeding |
| 83c148b | feat(19-03): add bearerTransport for daemon bearer token injection |

---

### Plan Deviations (Verified Present and Correct)

Plan 19-02 added two Store interface methods beyond the original spec. These were required to support complete behavior:

- `UpdateUser(ctx, *User)` — required by `handleChangePassword` to persist the new password hash
- `GetUserByUsernameAny(ctx, username)` — required by `handleLogin` to distinguish inactive accounts (401 "account disabled") from non-existent accounts (401 "invalid credentials")

Both methods are implemented in SQLiteStore and tested. They do not break any requirements; they strengthen AUTH-01.

---

_Verified: 2026-03-17_
_Verifier: Claude (gsd-verifier)_
