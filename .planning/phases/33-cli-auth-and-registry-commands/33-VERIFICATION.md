---
phase: 33-cli-auth-and-registry-commands
verified: 2026-04-07T00:00:00Z
status: passed
score: 13/13 must-haves verified
re_verification: false
---

# Phase 33: CLI Auth and Registry Commands — Verification Report

**Phase Goal:** CLI users can authenticate once with `flowy login` and then push, list, pull, tag, and delete registry entries without repeating credentials
**Verified:** 2026-04-07
**Status:** PASSED
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | User can run `flowy login --server <url>`, enter username/password, and have JWT saved to ~/.flowy/credentials.yaml | VERIFIED | `runLogin` in cmd/flowy/login.go calls `client.Login` then `client.SaveCredentials`; `TestLoginSuccess` passes |
| 2 | Credentials file is written with mode 0600 in a directory with mode 0700 | VERIFIED | credentials.go:68 `os.MkdirAll(dir, 0700)`, :77 `os.WriteFile(path, data, 0600)`, :83 `os.Chmod(path, 0600)` enforces unconditionally |
| 3 | Any CLI command can load saved token from credentials file via LoadCredentials() | VERIFIED | `loadCredsAndClient` in registry.go calls `client.LoadCredentials()` then `client.NewWithToken()` — used by all 6 registry subcommands |
| 4 | Missing credentials file produces human-readable error directing user to run `flowy login` | VERIFIED | LoadCredentials returns "credentials: no credentials found — run `flowy login --server <url>` to authenticate"; `loadCredsAndClient` prints "Not authenticated. Run `flowy login --server <url>` first."; `TestRegistryMissingCredentials` passes |
| 5 | Expired/invalid token (401) produces human-readable error, not raw JSON | VERIFIED | `handleClientError` maps `ErrUnauthorized` to "Token expired or invalid. Run `flowy login` to authenticate."; `TestRegistryUnauthorized` passes |
| 6 | Client struct supports Bearer token injection on every HTTP request | VERIFIED | `doRequest` in client.go:57 sets `Authorization: Bearer <token>` when token is non-empty; all methods (including pre-existing ones) route through `doRequest` |
| 7 | User can push a WDL file via `flowy registry push` and see the digest | VERIFIED | `runRegistryPush` reads file, calls `c.RegistryPush`, prints `Pushed {name} digest:{hash}`; `TestRegistryPushSuccess` passes |
| 8 | User can list registry entries via `flowy registry list` as an aligned table | VERIFIED | `runRegistryList` uses `text/tabwriter` with NAME/KIND/LATEST TAG/VERSIONS columns; `TestRegistryListSuccess` passes |
| 9 | User can pull WDL source to stdout via `flowy registry pull` | VERIFIED | `runRegistryPull` calls `fmt.Fprint(stdout, resp.Content)` with no added newline; `TestRegistryPullSuccess` passes |
| 10 | User can create a tag via `flowy registry tag` | VERIFIED | `runRegistryTag` calls `c.RegistryTag`, prints "Tagged {name} {tag}"; `TestRegistryTagSuccess` passes |
| 11 | User can delete a tag via `flowy registry delete-tag` | VERIFIED | `runRegistryDeleteTag` calls `c.RegistryDeleteTag`, prints "Deleted tag {tag} from {name}"; `TestRegistryDeleteTagSuccess` passes |
| 12 | Admin can delete a registry entry via `flowy registry delete` | VERIFIED | `runRegistryDelete` calls `c.RegistryDelete`, prints "Deleted {name}"; `TestRegistryDeleteSuccess` passes |
| 13 | All registry commands load credentials automatically and fail with human-readable error if missing/expired | VERIFIED | `loadCredsAndClient` helper called at top of all 6 subcommand handlers; 401/403 mapped by `handleClientError`; tests for missing creds, 401, 403 all pass |

**Score:** 13/13 truths verified

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/client/credentials.go` | Credentials struct, LoadCredentials, SaveCredentials | VERIFIED | 87 lines; all three exports present; 0600/0700 permissions enforced |
| `internal/client/credentials_test.go` | Unit tests for credential file I/O | VERIFIED | Exists; tests for permission, round-trip, and missing file |
| `internal/client/client.go` | NewWithToken, doRequest, Login, 6 registry methods, ErrUnauthorized, ErrForbidden | VERIFIED | 545 lines; all exports confirmed present |
| `internal/client/registry_test.go` | 13 httptest-based tests for registry client methods | VERIFIED | File exists; all 13 tests pass |
| `cmd/flowy/login.go` | runLogin subcommand | VERIFIED | 102 lines; full implementation with term.ReadPassword, error handling, token-zero safety |
| `cmd/flowy/login_test.go` | Unit tests for login command | VERIFIED | 6 tests pass (TestLoginSuccess, TestLoginMissingServer, TestLoginServerError, TestLoginConnectionError, TestLoginEmptyToken, TestLoginRequestBody) |
| `cmd/flowy/registry.go` | runRegistry dispatcher and all 6 subcommand functions | VERIFIED | 306 lines; all 8 functions present (runRegistry + 6 subcommands + helpers) |
| `cmd/flowy/registry_test.go` | 13 tests for all registry subcommands | VERIFIED | All 13 tests pass |
| `cmd/flowy/main.go` | `case "login"` and `case "registry"` in switch | VERIFIED | Lines 55 and 57 confirmed; usage text at lines 76 and 82 |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `cmd/flowy/login.go` | `internal/client/client.go` | `client.New(serverURL).Login(ctx, username, password)` | WIRED | login.go:72-73 calls `client.New(*serverFlag)` then `c.Login(...)` |
| `cmd/flowy/login.go` | `internal/client/credentials.go` | `client.SaveCredentials` | WIRED | login.go:95 calls `client.SaveCredentials(creds)` |
| `internal/client/client.go` | Authorization header | `Bearer token on every request when token is set` | WIRED | client.go:57 `req.Header.Set("Authorization", "Bearer "+c.token)` inside `doRequest`; all methods use `doRequest` |
| `cmd/flowy/registry.go` | `internal/client/credentials.go` | `client.LoadCredentials()` at start of each subcommand | WIRED | `loadCredsAndClient` helper calls `client.LoadCredentials()`; all 6 subcommands call `loadCredsAndClient` before any HTTP operation |
| `cmd/flowy/registry.go` | `internal/client/client.go` | `client.NewWithToken(creds.Server, creds.Token)` then registry methods | WIRED | `loadCredsAndClient` returns `client.NewWithToken(creds.Server, creds.Token, 0)` |
| `internal/client/client.go` | server registry endpoints | `HTTP requests with Bearer token to /projects/{slug}/registry/*` | WIRED | All 6 registry methods build URL with `/projects/<slug>/registry` prefix and call `c.doRequest` which injects Bearer header |

---

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| CLI-01 | 33-01 | User can log in via `flowy login` and JWT token saved to credentials file | SATISFIED | `runLogin` + `SaveCredentials` + tests pass |
| CLI-02 | 33-01 | All authenticated CLI commands automatically load saved token | SATISFIED | `loadCredsAndClient` called by all 6 registry subcommands; credentials loaded from file automatically |
| REG-CLI-01 | 33-02 | User can push a WDL file via `flowy registry push` | SATISFIED | `runRegistryPush` + `RegistryPush` client method + test pass |
| REG-CLI-02 | 33-02 | User can list registry entries via `flowy registry list` | SATISFIED | `runRegistryList` with tabwriter + `RegistryList` method + test pass |
| REG-CLI-03 | 33-02 | User can pull WDL source via `flowy registry pull` | SATISFIED | `runRegistryPull` with `--tag`/`--digest` support + `RegistryPull` method + test pass |
| REG-CLI-04 | 33-02 | User can manage tags via `flowy registry tag` and `flowy registry delete-tag` | SATISFIED | Both `runRegistryTag` and `runRegistryDeleteTag` implemented and tested |
| REG-CLI-05 | 33-02 | Admin can delete a registry entry via `flowy registry delete` | SATISFIED | `runRegistryDelete` + `RegistryDelete` client method (returns `ErrForbidden` on 403) + test pass |

No orphaned requirements: all 7 requirement IDs (CLI-01, CLI-02, REG-CLI-01 through REG-CLI-05) are covered by plans 33-01 and 33-02 respectively and are all accounted for.

---

### Anti-Patterns Found

None. No TODO/FIXME/placeholder comments, no stub return values, no empty handlers found in any phase-modified file.

---

### Human Verification Required

#### 1. Terminal password echo suppression

**Test:** Run `flowy login --server http://localhost:8124` against a real server. Type password characters.
**Expected:** Password characters do not appear in the terminal.
**Why human:** `term.ReadPassword` behavior requires an actual TTY; automated tests override `readPasswordFn` and cannot verify the no-echo property.

#### 2. Credential file permissions on macOS/Linux

**Test:** After `flowy login`, run `ls -la ~/.flowy/credentials.yaml` and `ls -la ~/.flowy/`.
**Expected:** File shows `-rw-------` (0600); directory shows `drwx------` (0700).
**Why human:** File permission tests use `t.Setenv("HOME", tmpDir)` and verify in-process, but this confirms the actual binary behaves correctly on the real filesystem.

---

### Gaps Summary

No gaps. All must-haves verified. Phase goal fully achieved.

---

## Test Results

| Test Suite | Command | Result |
|-----------|---------|--------|
| Client package | `go test ./internal/client/...` | PASS |
| CLI login tests | `go test ./cmd/flowy/... -run TestLogin` | 6/6 PASS |
| CLI registry tests | `go test ./cmd/flowy/... -run TestRegistry` | 13/13 PASS |
| Full suite | `go test ./... -count=1 -timeout 120s` | 20/20 packages PASS |

## Commits Verified

| Commit | Description |
|--------|-------------|
| a95ac86 | feat(33-01): implement credential I/O, NewWithToken, and Login on Client |
| 3f7f2e1 | feat(33-01): add login command and wire into main.go |
| a4a4b8a | feat(33-02): add registry client methods (RegistryPush, List, Pull, Tag, DeleteTag, Delete) |
| b4839b0 | feat(33-02): add flowy registry subcommands and wire into main.go |

---

_Verified: 2026-04-07_
_Verifier: Claude (gsd-verifier)_
