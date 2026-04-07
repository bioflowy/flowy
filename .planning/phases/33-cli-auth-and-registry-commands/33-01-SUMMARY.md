---
phase: 33-cli-auth-and-registry-commands
plan: "01"
subsystem: cli-auth
tags: [cli, auth, credentials, jwt, login]
dependency_graph:
  requires: []
  provides: [internal/client/credentials.go, internal/client/client.go, cmd/flowy/login.go]
  affects: [cmd/flowy/main.go, internal/client/client.go]
tech_stack:
  added: [golang.org/x/term]
  patterns: [credential-file-io, bearer-token-injection, tdd]
key_files:
  created:
    - internal/client/credentials.go
    - internal/client/credentials_test.go
    - cmd/flowy/login.go
    - cmd/flowy/login_test.go
  modified:
    - internal/client/client.go
    - cmd/flowy/main.go
    - cmd/flowy/main_test.go
decisions:
  - "Added stdin io.Reader parameter to run() to make login testable without a real terminal"
  - "Used package-level readPasswordFn variable for term.ReadPassword so tests can override it"
  - "doRequest helper centralizes Bearer token injection — existing New() clients unaffected (empty token = no header)"
metrics:
  duration_minutes: 25
  completed_date: "2026-04-07"
  tasks_completed: 2
  files_changed: 7
---

# Phase 33 Plan 01: CLI Auth — Login Command and Credential Infrastructure Summary

**One-liner:** JWT credential file I/O (0600/0700) with `flowy login` command using term.ReadPassword and Bearer token injection on all Client requests.

## What Was Built

- `internal/client/credentials.go`: `Credentials` struct (YAML-tagged), `LoadCredentials`, `SaveCredentials` with strict file permissions (0600 file, 0700 directory), and `CredentialsPath` helper.
- `internal/client/client.go`: Added `token` field, `NewWithToken` constructor, `doRequest` helper for uniform Bearer header injection, `Login` method (POST /auth/login), and sentinel errors `ErrUnauthorized`/`ErrForbidden`. All existing methods updated to use `doRequest`.
- `cmd/flowy/login.go`: `runLogin` subcommand with `--server` flag (required), stdin-based username prompt, `term.ReadPassword` for password (no echo), human-readable errors for all failure modes, empty-token warning.
- `cmd/flowy/main.go`: Added `stdin io.Reader` parameter to `run()`, dispatches `"login"` case, usage text updated.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Credential file I/O and Client token support | a95ac86 | internal/client/credentials.go, internal/client/client.go, internal/client/credentials_test.go |
| 2 | Login command and main.go wiring | 3f7f2e1 | cmd/flowy/login.go, cmd/flowy/login_test.go, cmd/flowy/main.go, cmd/flowy/main_test.go |

TDD commits:
- f73c176: test(33-01): failing tests for credentials I/O and client token support (RED)
- a95ac86: feat(33-01): implement credentials, NewWithToken, Login (GREEN)
- d92dc27: test(33-01): failing tests for login command (RED)
- 3f7f2e1: feat(33-01): login command and main.go wiring (GREEN)

## Decisions Made

1. **stdin parameter on run()**: Added `stdin io.Reader` to the top-level `run()` function signature so `runLogin` can read username from an injectable reader. This keeps the login command fully testable without a real terminal. All existing callers pass `os.Stdin` (production) or `bytes.NewReader(nil)` (tests).

2. **readPasswordFn package variable**: Used a package-level `var readPasswordFn` defaulting to `term.ReadPassword(syscall.Stdin)` so tests can override it without needing a PTY. This avoids a full mock interface for a single function.

3. **doRequest centralization**: All client methods now call `c.doRequest(req)` which conditionally sets `Authorization: Bearer <token>`. The existing `New()` constructor leaves `token` as empty string, so no header is added — fully backward compatible.

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check

### Files Created/Modified
- [x] FOUND: internal/client/credentials.go
- [x] FOUND: internal/client/credentials_test.go
- [x] FOUND: internal/client/client.go (modified)
- [x] FOUND: cmd/flowy/login.go
- [x] FOUND: cmd/flowy/login_test.go
- [x] FOUND: cmd/flowy/main.go (modified)

### Commits Verified
- [x] f73c176 — test RED: credentials
- [x] a95ac86 — feat GREEN: credentials + client
- [x] d92dc27 — test RED: login command
- [x] 3f7f2e1 — feat GREEN: login command + main wiring

### Test Results
- go test ./internal/client/... — PASS
- go test ./cmd/flowy/... — PASS
- go test ./... — ALL PASS (24 packages)
- go vet ./... — PASS

## Self-Check: PASSED
