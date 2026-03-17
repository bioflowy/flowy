---
phase: 19-user-authentication
plan: "03"
subsystem: auth-cli-daemon
tags: [auth, cli, daemon, init-admin, bearer-token, http-transport]
dependency_graph:
  requires:
    - internal/auth/auth.go (HashPassword, DefaultBcryptCost from plan 19-02)
    - internal/store/store.go (User, CountUsers, CreateUser from plan 19-01)
    - internal/config/config.go (AuthConfig.DaemonToken from plan 19-01)
  provides:
    - --init-admin CLI flag for admin user seeding
    - bearerTransport for authenticated daemon-to-server HTTP communication
  affects:
    - cmd/flowy-server/main.go (new flags; init-admin path exits before HTTP server)
    - internal/daemon/daemon.go (bearerTransport injected via New())
tech_stack:
  added:
    - golang.org/x/term (hidden password prompt in terminal)
  patterns:
    - TDD: failing tests first, then minimal implementation
    - Minimal interface (InitAdminStore) for testability without importing full store
    - storeAdapter pattern to bridge store.Store to minimal interface
    - http.RoundTripper wrapper (bearerTransport) for transparent header injection
key_files:
  created:
    - cmd/flowy-server/initadmin.go
    - cmd/flowy-server/initadmin_test.go
    - internal/daemon/bearer_test.go
  modified:
    - cmd/flowy-server/main.go
    - internal/daemon/daemon.go
    - go.mod (golang.org/x/term added)
    - go.sum
decisions:
  - "InitAdminStore defined as minimal interface (CountUsers + CreateUser with createUserArgs) to avoid test imports of full store package"
  - "storeAdapter bridges store.Store to InitAdminStore in production main.go path"
  - "errUsersExist is a sentinel error for distinguishing pre-existing users from other errors"
  - "bearerTransport clones request before mutation (req.Clone) to satisfy http.RoundTripper contract"
  - "daemon.New() uses plain http.DefaultTransport when token is empty — zero behavioral change"
metrics:
  duration: "357 seconds"
  completed_date: "2026-03-17"
  tasks_completed: 2
  tasks_total: 2
  files_created: 3
  files_modified: 4
---

# Phase 19 Plan 03: Init-Admin CLI and Daemon Bearer Token Summary

**One-liner:** --init-admin CLI command for admin seeding (username/password flags or interactive prompt, bcrypt hashed) plus bearerTransport for authenticated daemon-to-server HTTP communication when auth.daemon_token is configured.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Implement --init-admin CLI command | dc6306f | initadmin.go, initadmin_test.go, main.go, go.mod, go.sum |
| 2 | Add bearer token injection to daemon HTTP client | 83c148b | daemon.go, bearer_test.go |

## What Was Built

### Task 1: --init-admin CLI Command (cmd/flowy-server/initadmin.go)

Solves the chicken-and-egg problem: POST /users is admin-only, but no admin exists initially.

**Types added:**
- `InitAdminConfig` — username, password, bcrypt cost params
- `InitAdminStore` — minimal interface (CountUsers + CreateUser via createUserArgs)
- `createUserArgs` — separates store.User from the interface for testability
- `storeAdapter` — bridges full `store.Store` to `InitAdminStore` in production path

**`runInitAdmin(ctx, st, iac)` behavior:**
1. Validates username is non-empty (returns error "username is required")
2. Calls `CountUsers` — if count > 0, returns `errUsersExist` sentinel
3. If password is empty, calls `promptPassword` twice (hidden input via `term.ReadPassword`)
4. Hashes password with `auth.HashPassword(password, iac.BcryptCost)`
5. Generates UUID, calls `CreateUser` with global_role="admin", is_active=true
6. Prints "Admin user created: {username}"

**`promptPassword(prompt)`** — writes prompt to stderr, uses `golang.org/x/term` for hidden input, prints newline after.

**main.go flags added:**
- `--init-admin` (bool) — triggers init path, exits before HTTP server start
- `--username` (string) — passed as InitAdminConfig.Username
- `--password` (string) — passed as InitAdminConfig.Password (optional; prompts if empty)

Init-admin path runs between store open and backend/server construction — no HTTP server is started.

### Task 2: Daemon Bearer Token Injection (internal/daemon/daemon.go)

Enables authenticated daemon-to-server communication when auth is enabled.

**`bearerTransport` struct:**
```go
type bearerTransport struct {
    token string
    base  http.RoundTripper
}
```
`RoundTrip` clones request before setting `Authorization: Bearer {token}`. When token is empty, no header is added and base transport is used unchanged.

**`daemon.New()` update:**
- If `cfg.Auth.DaemonToken != ""`, wraps `http.DefaultTransport` with `bearerTransport`
- Otherwise uses plain `http.DefaultTransport`
- All daemon HTTP calls (claimTask, sendHeartbeat, reportResult) automatically get the header

## Tests

### init-admin tests (cmd/flowy-server/initadmin_test.go)
- `TestRunInitAdmin_Success` — empty DB, username+password → user created with admin role, is_active=true
- `TestRunInitAdmin_UsersExist` — CountUsers=1 → errUsersExist returned
- `TestRunInitAdmin_EmptyUsername` — empty username → error returned
- `TestRunInitAdmin_PasswordHashed` — bcrypt.CompareHashAndPassword verifies stored hash

### bearer transport tests (internal/daemon/bearer_test.go)
- `TestBearerTransport_WithToken` — transport with token → Authorization header present in request
- `TestBearerTransport_WithoutToken` — transport with empty token → no Authorization header
- `TestDaemonNew_WithToken` — New() with DaemonToken → claimTask sends Authorization header
- `TestDaemonNew_WithoutToken` — New() without DaemonToken → no Authorization header sent

## Verification

```
go test ./cmd/flowy-server/... -timeout 30s    # 4 tests pass
go test ./internal/daemon/... -timeout 30s     # All daemon tests pass
go test ./... -timeout 120s                    # All 21 packages pass
```

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Auth package (19-02) not yet committed but files existed**
- **Found during:** Task 1 setup
- **Issue:** Plan 19-03 depends on `auth.HashPassword` and `auth.DefaultBcryptCost` from plan 19-02, which was not committed. The files existed in the working tree (untracked) from a previous partial execution.
- **Fix:** Auth package files were already present and working; proceeded without modification.
- **Impact:** No code change needed; plan 19-03 executed successfully.

**2. [Rule 1 - Bug] Test used nil context in claimTask**
- **Found during:** Task 2 GREEN phase
- **Issue:** `TestDaemonNew_WithToken` and `TestDaemonNew_WithoutToken` passed `nil` as context to `claimTask`, causing "nil Context" error.
- **Fix:** Changed to `context.Background()`.
- **Files modified:** internal/daemon/bearer_test.go

## Self-Check

Files exist:
- cmd/flowy-server/initadmin.go — FOUND
- cmd/flowy-server/initadmin_test.go — FOUND
- internal/daemon/bearer_test.go — FOUND

Commits exist:
- dc6306f feat(19-03): implement --init-admin CLI command for admin user seeding — FOUND
- 83c148b feat(19-03): add bearerTransport for daemon bearer token injection — FOUND

## Self-Check: PASSED
