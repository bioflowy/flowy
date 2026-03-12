---
phase: 11
slug: daemon-binary-task-execution
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-12
---

# Phase 11 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing (stdlib) |
| **Config file** | none (uses `go test ./...`) |
| **Quick run command** | `go test ./internal/daemon/... -count=1 -timeout 30s` |
| **Full suite command** | `go test ./... -count=1 -timeout 120s` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/daemon/... -count=1 -timeout 30s`
- **After every plan wave:** Run `go test ./... -count=1 -timeout 120s`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 11-01-01 | 01 | 1 | DMN-06 | smoke | `go build ./cmd/flowy-daemon/` | ❌ W0 | ⬜ pending |
| 11-01-02 | 01 | 1 | DMN-01 | unit | `go test ./internal/daemon/... -run TestConcurrentClaim -count=1` | ❌ W0 | ⬜ pending |
| 11-01-03 | 01 | 1 | DMN-02 | integration | `go test ./internal/daemon/... -run TestExecuteTask -count=1` | ❌ W0 | ⬜ pending |
| 11-01-04 | 01 | 1 | DMN-03 | unit | `go test ./internal/daemon/... -run TestHeartbeat -count=1` | ❌ W0 | ⬜ pending |
| 11-01-05 | 01 | 1 | DMN-04 | integration | `go test ./internal/daemon/... -run TestGracefulShutdown -count=1` | ❌ W0 | ⬜ pending |
| 11-01-06 | 01 | 1 | DMN-05 | unit | `go test ./internal/daemon/... -run TestOutputsJSON -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/daemon/daemon_test.go` — stubs for DMN-01 through DMN-05
- [ ] `cmd/flowy-daemon/main.go` — minimal binary entry point (DMN-06 smoke test)
- [ ] `internal/api/types.go` — add `WDLPath` field to `TaskClaimResponse`
- [ ] `internal/server/handlers.go` — populate `WDLPath` in `handleClaim`

*Existing infrastructure (go test, httptest, in-memory SQLiteStore) covers all phase requirements.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| End-to-end server+daemon with real Docker | DMN-02 | Requires Docker daemon running | Start flowy-server, start flowy-daemon, submit WDL via API, verify outputs appear |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
