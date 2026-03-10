---
phase: 5
slug: task-execution
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-10
---

# Phase 5 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing (stdlib `testing` package) |
| **Config file** | none — `go test ./...` discovers all `*_test.go` files |
| **Quick run command** | `go test ./wdl/task/... -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./wdl/task/... -count=1`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 05-01-01 | 01 | 1 | TASK-01 | unit | `go test ./wdl/task/... -run TestBindInputs -v` | ❌ W0 | ⬜ pending |
| 05-01-02 | 01 | 1 | TASK-02 | unit | `go test ./wdl/task/... -run TestParseRuntime -v` | ❌ W0 | ⬜ pending |
| 05-01-03 | 01 | 1 | TASK-03 | unit | `go test ./wdl/task/... -run TestBuildCommand -v` | ❌ W0 | ⬜ pending |
| 05-02-01 | 02 | 1 | TASK-04 | unit | `go test ./wdl/task/... -run TestPathMap -v` | ❌ W0 | ⬜ pending |
| 05-03-01 | 03 | 2 | TASK-05 | integration | `go test ./wdl/task/... -run TestDockerRunner -tags integration` | ❌ W0 | ⬜ pending |
| 05-03-02 | 03 | 2 | TASK-06 | unit | `go test ./wdl/task/... -run TestCollectOutputs -v` | ❌ W0 | ⬜ pending |
| 05-04-01 | 04 | 3 | TASK-07 | unit | `go test ./wdl/task/... -run TestRunTask -v` | ❌ W0 | ⬜ pending |
| 05-04-02 | 04 | 3 | TASK-08 | unit | `go test ./wdl/task/... -run TestTaskError -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `wdl/task/inputs_test.go` — stubs for TASK-01 (bindInputs unit tests)
- [ ] `wdl/task/runtime_test.go` — stubs for TASK-02 (parseRuntime unit tests)
- [ ] `wdl/task/pathmap_test.go` — stubs for TASK-04 (PathMap unit tests)
- [ ] `wdl/task/task_test.go` — stubs for TASK-03, TASK-06, TASK-07, TASK-08 (command, outputs, end-to-end, error)
- [ ] `wdl/task/docker_test.go` — stubs for TASK-05 (integration, `//go:build integration` tag)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Docker lifecycle with real daemon | TASK-05 | Requires live Docker daemon | Run `go test ./wdl/task/... -run TestDockerRunner -tags integration` with Docker running |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
