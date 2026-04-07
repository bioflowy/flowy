---
phase: 33
slug: cli-auth-and-registry-commands
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-07
---

# Phase 33 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard Go test toolchain |
| **Quick run command** | `go test ./cmd/flowy/... ./internal/client/... -count=1` |
| **Full suite command** | `go test ./cmd/flowy/... ./internal/client/... ./internal/server/... -count=1` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./cmd/flowy/... ./internal/client/... -count=1`
- **After every plan wave:** Run `go test ./cmd/flowy/... ./internal/client/... ./internal/server/... -count=1`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 33-01-01 | 01 | 1 | CLI-01, CLI-02 | — | Credentials file mode 0600 | unit | `go test ./internal/client/... -run TestCredentials -count=1` | ❌ W0 | ⬜ pending |
| 33-02-01 | 02 | 1 | REG-CLI-01 thru REG-CLI-05 | — | 401→human message | integration | `go test ./cmd/flowy/... -run TestRegistry -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/client/credentials_test.go` — tests for credential load/save/permissions
- [ ] `cmd/flowy/registry_test.go` — tests for registry subcommands (or integration tests)

*Existing test infrastructure covers shared fixtures.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `flowy login` prompts for password (hidden) | CLI-01 | Requires terminal TTY | Run `flowy login --server http://localhost:8080`, verify password hidden |
| Human-readable error on expired token | CLI-02 | Requires expired JWT | Wait for token expiry or set short TTL, run any registry command |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
