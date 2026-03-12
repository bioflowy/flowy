---
phase: 10
slug: server-binary-dag-dispatch
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-11
---

# Phase 10 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard `testing` package (go test) |
| **Config file** | none — standard Go test tooling |
| **Quick run command** | `go test ./internal/server/... -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/server/... -count=1`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 10-01-01 | 01 | 1 | SRV-01 | integration | `go test ./internal/server/... -run TestSubmit -count=1` | ❌ W0 | ⬜ pending |
| 10-01-02 | 01 | 1 | SRV-02 | integration | `go test ./internal/server/... -run TestStatus -count=1` | ❌ W0 | ⬜ pending |
| 10-01-03 | 01 | 1 | SRV-03 | integration | `go test ./internal/server/... -run TestOutputs -count=1` | ❌ W0 | ⬜ pending |
| 10-01-04 | 01 | 1 | SRV-04 | integration | `go test ./internal/server/... -run TestAbort -count=1` | ❌ W0 | ⬜ pending |
| 10-01-05 | 01 | 1 | SRV-05 | unit | `go test ./internal/server/... -run TestHealth -count=1` | ❌ W0 | ⬜ pending |
| 10-01-06 | 01 | 1 | SRV-06 | integration | `go test ./internal/server/... -run TestDispatch -count=1` | ❌ W0 | ⬜ pending |
| 10-01-07 | 01 | 1 | SRV-09 | build | `go build ./cmd/flowy-server/` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/server/server_test.go` — integration tests covering SRV-01 through SRV-06 using httptest
- [ ] `internal/server/handlers_test.go` — handler unit tests with mock store
- [ ] Store extension: `GetTaskByID` in `internal/store/store.go` and `internal/store/sqlite/tasks.go`
- [ ] `wdl/workflow/workflow.go` — add `RunWorkflowWithDispatch` exported function

*No new packages to install — all dependencies already in go.mod*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Graceful shutdown on SIGTERM | SRV-09 | Requires signal delivery to process | Run `flowy-server`, send SIGTERM, verify clean exit |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
