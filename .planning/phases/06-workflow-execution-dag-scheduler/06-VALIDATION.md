---
phase: 6
slug: workflow-execution-dag-scheduler
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-10
---

# Phase 6 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` package (stdlib) |
| **Config file** | none — `go test ./wdl/workflow/...` |
| **Quick run command** | `go test ./wdl/workflow/... -count=1` |
| **Full suite command** | `go test -race ./wdl/workflow/... -count=1` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./wdl/workflow/... -count=1`
- **After every plan wave:** Run `go test -race ./wdl/workflow/... -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 6-01-01 | 01 | 1 | WF-01 | unit | `go test ./wdl/workflow/... -run TestBuildDAG -count=1` | ❌ W0 | ⬜ pending |
| 6-01-02 | 01 | 1 | WF-02 | integration | `go test ./wdl/workflow/... -run TestSingleCall -count=1` | ❌ W0 | ⬜ pending |
| 6-01-03 | 01 | 1 | WF-03 | integration | `go test ./wdl/workflow/... -run TestScatter -count=1` | ❌ W0 | ⬜ pending |
| 6-01-04 | 01 | 1 | WF-04 | unit | `go test ./wdl/workflow/... -run TestConditional -count=1` | ❌ W0 | ⬜ pending |
| 6-01-05 | 01 | 1 | WF-05 | integration | `go test ./wdl/workflow/... -run TestAfterDependency -count=1` | ❌ W0 | ⬜ pending |
| 6-01-06 | 01 | 1 | WF-06 | unit | `go test -race ./wdl/workflow/... -run TestScatterConcurrencyLimit -count=1` | ❌ W0 | ⬜ pending |
| 6-01-07 | 01 | 1 | WF-07 | integration | `go test ./wdl/workflow/... -run TestOutputsJSON -count=1` | ❌ W0 | ⬜ pending |
| 6-01-08 | 01 | 1 | WF-08 | integration | `go test ./wdl/workflow/... -run TestFailFast -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `wdl/workflow/workflow_test.go` — test stubs for WF-01 through WF-08; uses ShellRunner mock (no Docker)
- [ ] `wdl/workflow/workflow.go` — package stub with `RunWorkflow` signature
- [ ] `wdl/workflow/dag.go` — package stub for `buildDAG`
- [ ] Add `golang.org/x/sync` to `go.mod`: `go get golang.org/x/sync@v0.20.0`

*All test infrastructure is new for this phase — no existing framework to reuse.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Scatter goroutine count ≤ 8 under real Docker load | WF-06 | Requires Docker daemon and multiple containers | Run `flowy run` with a scatter WDL; observe `docker stats` shows ≤ 8 containers |

*Automated tests use a mock runner to verify SetLimit(8) is called correctly.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
