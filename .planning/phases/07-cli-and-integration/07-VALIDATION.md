---
phase: 7
slug: cli-and-integration
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-10
---

# Phase 7 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing stdlib (`testing` package) |
| **Config file** | none — `go test ./...` from repo root |
| **Quick run command** | `go test ./cmd/flowy/... ./wdl/parser/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./cmd/flowy/... -count=1`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 07-01-01 | 01 | 1 | CLI-01 | integration | `go test ./cmd/flowy/ -run TestCheck` | ❌ W0 | ⬜ pending |
| 07-01-02 | 01 | 1 | CLI-02 | integration | `go test ./cmd/flowy/ -run TestRun_Workflow` | ❌ W0 | ⬜ pending |
| 07-01-03 | 01 | 1 | CLI-03 | integration | `go test ./cmd/flowy/ -run TestRun_Task` | ❌ W0 | ⬜ pending |
| 07-01-04 | 01 | 1 | CLI-04 | integration | `go test ./cmd/flowy/ -run TestCheck_WithPath` | ❌ W0 | ⬜ pending |
| 07-01-05 | 01 | 1 | CLI-05 | unit | `go test ./cmd/flowy/ -run TestExecDir` | ❌ W0 | ⬜ pending |
| 07-01-06 | 01 | 1 | CLI-06 | integration | covered by CLI-01/02 tests | ❌ W0 | ⬜ pending |
| 07-01-07 | 01 | 1 | CLI-07 | integration | `go test ./cmd/flowy/ -run TestRun_Verbose` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `cmd/flowy/main.go` — CLI entry point with testable `run()` function
- [ ] `cmd/flowy/main_test.go` — stubs for CLI-01 through CLI-07
- [ ] `wdl/parser/parser_paths_test.go` — covers `ParseWithSearchPaths` (CLI-04)

*No framework install needed — Go testing stdlib is built in.*

---

## Manual-Only Verifications

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
