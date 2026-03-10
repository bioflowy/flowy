---
phase: 4
slug: expression-evaluator-and-standard-library
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-10
---

# Phase 4 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none |
| **Quick run command** | `go test ./wdl/eval/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./wdl/eval/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 4-01-01 | 01 | 0 | EXPR-01 | unit | `go test ./wdl/eval/...` | ❌ W0 | ⬜ pending |
| 4-01-02 | 01 | 1 | EXPR-01 | unit | `go test ./wdl/eval/... -run TestArithmetic` | ✅ | ⬜ pending |
| 4-01-03 | 01 | 1 | EXPR-02 | unit | `go test ./wdl/eval/... -run TestComparison` | ✅ | ⬜ pending |
| 4-01-04 | 01 | 1 | EXPR-03 | unit | `go test ./wdl/eval/... -run TestLogical` | ✅ | ⬜ pending |
| 4-01-05 | 01 | 1 | EXPR-04 | unit | `go test ./wdl/eval/... -run TestIfThenElse` | ✅ | ⬜ pending |
| 4-02-01 | 02 | 1 | EXPR-05 | unit | `go test ./wdl/eval/... -run TestStructFieldAccess` | ✅ | ⬜ pending |
| 4-02-02 | 02 | 1 | EXPR-05 | unit | `go test ./wdl/eval/... -run TestPairAccess` | ✅ | ⬜ pending |
| 4-02-03 | 02 | 1 | EXPR-05 | unit | `go test ./wdl/eval/... -run TestArrayIndex` | ✅ | ⬜ pending |
| 4-03-01 | 03 | 1 | EXPR-06 | unit | `go test ./wdl/eval/... -run TestCommandDedent` | ✅ | ⬜ pending |
| 4-03-02 | 03 | 1 | EXPR-07 | unit | `go test ./wdl/eval/... -run TestCommandInterp` | ✅ | ⬜ pending |
| 4-04-01 | 04 | 2 | STDLIB-01 | unit | `go test ./wdl/eval/... -run TestStdlibFileIO` | ✅ | ⬜ pending |
| 4-04-02 | 04 | 2 | STDLIB-02 | unit | `go test ./wdl/eval/... -run TestStdlibArray` | ✅ | ⬜ pending |
| 4-04-03 | 04 | 2 | STDLIB-03 | unit | `go test ./wdl/eval/... -run TestStdlibString` | ✅ | ⬜ pending |
| 4-04-04 | 04 | 2 | STDLIB-04 | unit | `go test ./wdl/eval/... -run TestStdlibNumeric` | ✅ | ⬜ pending |
| 4-04-05 | 04 | 2 | STDLIB-05 | unit | `go test ./wdl/eval/... -run TestStdlibUtility` | ✅ | ⬜ pending |
| 4-04-06 | 04 | 2 | STDLIB-06 | unit | `go test ./wdl/eval/... -run TestStdlibWDL12` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `wdl/eval/eval_test.go` — stubs for all EXPR-* and STDLIB-* requirements
- [ ] `wdl/eval/stdlib_test.go` — stdlib function test stubs
- [ ] `wdl/eval/command_test.go` — dedent algorithm test stubs

*All test files must compile (even with `t.Skip()`) before Wave 1 begins.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Negative round half-up (-2.5 → -2) | STDLIB-04 | Edge case numeric semantics | Run `go test ./wdl/eval/... -run TestRoundHalfUp` and verify -2.5 rounds to -2 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
