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
| 12-01-T1 | 01 | 1 | CLI-05, CLI-06 | unit | `go test ./internal/config/ ./internal/server/ -count=1` | existing | ⬜ pending |
| 12-01-T2 | 01 | 1 | CLI-05 | unit | `go test ./internal/client/ -count=1 -v` | ❌ W0 | ⬜ pending |
| 12-01-T3 | 01 | 1 | CLI-06 | unit | `go build ./internal/server/ && go test ./internal/server/ -count=1` | existing | ⬜ pending |
| 12-02-T1 | 02 | 2 | CLI-01, CLI-02, CLI-03, CLI-05 | integration | `go test ./cmd/flowy/ -run "TestSubmit\|TestStatus\|TestOutputs\|TestStage" -count=1 -v` | ❌ W0 | ⬜ pending |
| 12-03-T1 | 03 | 3 | CLI-04, CLI-06 | integration | `go build ./... && go test ./cmd/flowy/ -run "TestRun" -count=1 -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/client/client_test.go` — unit tests for Client Submit, Status, Outputs, TaskRun methods using httptest.NewServer
- [ ] `cmd/flowy/client_test.go` — integration tests for submit, status, outputs subcommands using httptest.NewServer
- [ ] `cmd/flowy/run_test.go` — updated tests for run subcommand (workflow and --task modes) using httptest.NewServer

*Existing test infrastructure (go test) already available. No framework install needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `flowy run` with actual server+daemon | CLI-04 | Requires Docker and running server/daemon processes | Start flowy-server, start flowy-daemon, run `flowy run` with a simple WDL |
| `flowy run --task` with actual server+daemon | CLI-04 | Requires Docker and running server/daemon processes | Start flowy-server, start flowy-daemon, run `flowy run --task hello test.wdl` |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
