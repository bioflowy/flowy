---
phase: 12
slug: client-cli-spectool-compatibility
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-12
---

# Phase 12 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing (`go test`) |
| **Config file** | none — standard `go test ./...` |
| **Quick run command** | `go test ./cmd/flowy/ ./internal/client/ -count=1` |
| **Full suite command** | `go test ./... -count=1 && go build ./...` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./cmd/flowy/ ./internal/client/ -count=1`
- **After every plan wave:** Run `go test ./... -count=1 && go build ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 12-01-01 | 01 | 1 | CLI-05 | unit | `go test ./internal/client/ -run TestStage -count=1` | ❌ W0 | ⬜ pending |
| 12-01-02 | 01 | 1 | CLI-01 | unit | `go test ./internal/client/ -run TestSubmit -count=1` | ❌ W0 | ⬜ pending |
| 12-01-03 | 01 | 1 | CLI-02 | unit | `go test ./internal/client/ -run TestStatus -count=1` | ❌ W0 | ⬜ pending |
| 12-01-04 | 01 | 1 | CLI-03 | unit | `go test ./internal/client/ -run TestOutputs -count=1` | ❌ W0 | ⬜ pending |
| 12-02-01 | 02 | 2 | CLI-01 | integration | `go test ./cmd/flowy/ -run TestSubmit -count=1` | ❌ W0 | ⬜ pending |
| 12-02-02 | 02 | 2 | CLI-02 | integration | `go test ./cmd/flowy/ -run TestStatus -count=1` | ❌ W0 | ⬜ pending |
| 12-02-03 | 02 | 2 | CLI-03 | integration | `go test ./cmd/flowy/ -run TestOutputs -count=1` | ❌ W0 | ⬜ pending |
| 12-02-04 | 02 | 2 | CLI-04 | integration | `go test ./cmd/flowy/ -run TestRun -count=1` | ❌ W0 | ⬜ pending |
| 12-02-05 | 02 | 2 | CLI-06 | build | `go build ./...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/client/client_test.go` — unit tests for Client Submit, Status, Outputs methods using httptest.NewServer
- [ ] `cmd/flowy/client_test.go` — integration tests for submit, status, outputs, run subcommands using httptest.NewServer

*Existing test infrastructure (go test) already available. No framework install needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `flowy run` with actual server+daemon | CLI-04 | Requires Docker and running server/daemon processes | Start flowy-server, start flowy-daemon, run `flowy run` with a simple WDL |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
