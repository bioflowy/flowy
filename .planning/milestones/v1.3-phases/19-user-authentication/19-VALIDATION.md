---
phase: 19
slug: user-authentication
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-17
---

# Phase 19 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + net/http/httptest (stdlib) |
| **Config file** | none — Go test toolchain uses `go test ./...` |
| **Quick run command** | `go test ./internal/auth/... ./internal/server/... -run TestAuth -timeout 30s` |
| **Full suite command** | `go test ./... -timeout 120s` |
| **Estimated runtime** | ~30 seconds (quick), ~120 seconds (full) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/auth/... ./internal/server/... -timeout 60s`
- **After every plan wave:** Run `go test ./... -timeout 120s`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 19-01-01 | 01 | 1 | AUTH-01 | unit | `go test ./internal/server/... -run TestHandleLogin -timeout 30s` | ❌ W0 | ⬜ pending |
| 19-01-02 | 01 | 1 | AUTH-01 | unit | `go test ./internal/server/... -run TestHandleLoginInvalid -timeout 30s` | ❌ W0 | ⬜ pending |
| 19-02-01 | 02 | 1 | AUTH-02 | unit | `go test ./internal/server/... -run TestJWTMiddleware -timeout 30s` | ❌ W0 | ⬜ pending |
| 19-02-02 | 02 | 1 | AUTH-02 | unit | `go test ./internal/server/... -run TestJWTMiddlewareValid -timeout 30s` | ❌ W0 | ⬜ pending |
| 19-03-01 | 03 | 1 | AUTH-03 | unit | `go test ./cmd/flowy-server/... -run TestInitAdmin -timeout 30s` | ❌ W0 | ⬜ pending |
| 19-03-02 | 03 | 1 | AUTH-03 | unit | `go test ./cmd/flowy-server/... -run TestInitAdminExists -timeout 30s` | ❌ W0 | ⬜ pending |
| 19-04-01 | 04 | 2 | AUTH-04 | unit | `go test ./internal/server/... -run TestHandleCreateUser -timeout 30s` | ❌ W0 | ⬜ pending |
| 19-04-02 | 04 | 2 | AUTH-04 | unit | `go test ./internal/server/... -run TestHandleCreateUserForbidden -timeout 30s` | ❌ W0 | ⬜ pending |
| 19-05-01 | 05 | 2 | AUTH-05 | unit | `go test ./internal/server/... -run TestHandleListUsers -timeout 30s` | ❌ W0 | ⬜ pending |
| 19-05-02 | 05 | 2 | AUTH-05 | unit | `go test ./internal/server/... -run TestHandleDeleteUser -timeout 30s` | ❌ W0 | ⬜ pending |
| 19-06-01 | 06 | 3 | AUTH-06 | regression | `go test ./internal/server/... -timeout 60s` | ✅ existing | ⬜ pending |
| 19-06-02 | 06 | 3 | AUTH-06 | regression | `go test ./internal/server/... -run TestClaim -timeout 30s` | ✅ existing | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/auth/auth_test.go` — stubs for JWT issue/validate and bcrypt helpers
- [ ] `internal/server/handlers_auth_test.go` — stubs for AUTH-01 through AUTH-05 handler tests
- [ ] `cmd/flowy-server/initadmin_test.go` — stubs for AUTH-03 `--init-admin` logic
- [ ] `go get github.com/golang-jwt/jwt/v5` — jwt library not yet in go.mod

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Interactive password prompt for --init-admin | AUTH-03 | Requires terminal TTY input | Run `flowy-server --init-admin` in terminal, verify hidden password prompt appears |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
