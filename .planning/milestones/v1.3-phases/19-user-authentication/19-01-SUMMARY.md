---
phase: 19-user-authentication
plan: "01"
subsystem: auth-foundation
tags: [auth, config, store, sqlite, migration]
dependency_graph:
  requires: []
  provides:
    - AuthConfig struct in internal/config/config.go
    - User model in internal/store/store.go
    - Store interface user+server_config methods
    - SQLite migration 00004 (users + server_config tables)
    - SQLiteStore user CRUD implementation
    - SQLiteStore server_config implementation
  affects:
    - internal/store/sqlite (SQLiteStore must now implement 8 new methods)
tech_stack:
  added: []
  patterns:
    - TDD: RED (build assertion fails) → GREEN (implementation passes)
    - rowScanner interface for testable scan logic
    - INSERT OR REPLACE for server_config upsert
    - Soft-delete pattern (is_active=0) for user deactivation
key_files:
  created:
    - internal/store/sqlite/migrations/00004_users.sql
    - internal/store/sqlite/users.go
    - internal/store/sqlite/server_config.go
    - internal/store/sqlite/users_test.go
  modified:
    - internal/config/config.go
    - internal/store/store.go
decisions:
  - "Auth.Enabled defaults to false (bool zero value) — no applyDefaults entry needed; all 156 spectool tests continue to pass"
  - "GetUserByUsername filters is_active=1; GetUserByID does not filter (used for admin inspection of deactivated accounts)"
  - "parseTime supports RFC3339 and SQLite datetime() format to handle both written and default timestamps"
  - "CountUsers counts all users including inactive (gives total user count for chicken-and-egg admin seeding logic)"
metrics:
  duration: "153 seconds"
  completed_date: "2026-03-17"
  tasks_completed: 2
  tasks_total: 2
  files_created: 4
  files_modified: 2
---

# Phase 19 Plan 01: Auth Data Foundation Summary

**One-liner:** AuthConfig struct, User model, 8-method Store interface extension, SQLite migration 00004 (users + server_config tables), and full CRUD implementation — zero behavioral change when auth.enabled is false.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Add AuthConfig to config and User model + store interface | 3f77894 | internal/config/config.go, internal/store/store.go |
| 2 | Create migration 00004 and SQLite store implementations | 66eb254 | migrations/00004_users.sql, users.go, server_config.go, users_test.go |

## What Was Built

### AuthConfig (internal/config/config.go)

New `AuthConfig` struct added to `Config`:

- `Enabled bool` — defaults false; when false, all auth middleware is bypassed
- `JWTSecret string` — HMAC-SHA256 signing key; auto-generated when empty and Enabled=true
- `DaemonToken string` — shared daemon secret; auto-generated when empty and Enabled=true

### User Model (internal/store/store.go)

New `User` struct with fields: ID, Username, PasswordHash, DisplayName, Email, GlobalRole ("admin"/"executor"/"viewer"), IsActive, CreatedAt, UpdatedAt.

### Store Interface Extensions (internal/store/store.go)

6 user methods: CreateUser, GetUserByUsername, GetUserByID, ListUsers, DeactivateUser, CountUsers.
2 server_config methods: GetServerConfig, SetServerConfig.

### Migration 00004 (internal/store/sqlite/migrations/00004_users.sql)

- `users` table: TEXT primary key UUID, UNIQUE username, bcrypt hash, soft-delete via is_active INTEGER
- `server_config` table: simple key-value store for auto-generated secrets
- Index on users.username for fast login lookups

### SQLite Implementation

- `users.go`: Full CRUD with soft-delete. GetUserByUsername filters `is_active=1`; GetUserByID returns regardless of active state.
- `server_config.go`: GetServerConfig (returns sql.ErrNoRows if missing); SetServerConfig (INSERT OR REPLACE upsert).

## Tests

7 new tests in `users_test.go`, all passing:

1. TestCreateUser_and_GetByUsername — full field round-trip verification
2. TestCreateUser_DuplicateUsername — UNIQUE constraint error
3. TestGetUserByID — retrieval by UUID
4. TestListUsers — ordering and count
5. TestDeactivateUser — soft-delete; GetUserByUsername fails; GetUserByID shows is_active=false
6. TestCountUsers — count includes inactive users
7. TestServerConfig — missing key returns sql.ErrNoRows; set/get round-trip; upsert overwrites

Uses `bcrypt.MinCost` for speed in tests.

## Verification

```
go build ./internal/config/... ./internal/store/...  # OK
go test ./internal/store/... -timeout 60s            # All pass
go test ./... -timeout 120s                          # All 21 packages pass (0 failures)
```

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check

Files exist:
- internal/config/config.go — FOUND (AuthConfig added)
- internal/store/store.go — FOUND (User + 8 methods added)
- internal/store/sqlite/migrations/00004_users.sql — FOUND
- internal/store/sqlite/users.go — FOUND
- internal/store/sqlite/server_config.go — FOUND
- internal/store/sqlite/users_test.go — FOUND

Commits exist:
- 3f77894 feat(19-01): add AuthConfig struct and User model with store interface methods — FOUND
- 66eb254 feat(19-01): add migration 00004 and SQLite user/server_config implementations — FOUND

## Self-Check: PASSED
