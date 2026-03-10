---
phase: 3
slug: type-checker
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-10
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go built-in testing (`testing` package), Go 1.24 |
| **Config file** | none (standard `go test ./...`) |
| **Quick run command** | `go test ./wdl/checker/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./wdl/checker/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 3-01-01 | 01 | 0 | TYPE-05,06,07,08 | unit | `go test ./wdl/checker/... -run TestCoerces` | ❌ W0 | ⬜ pending |
| 3-01-02 | 01 | 0 | TYPE-05 | unit | `go test ./wdl/checker/... -run TestCoerces` | ❌ W0 | ⬜ pending |
| 3-01-03 | 01 | 1 | TYPE-05 | unit | `go test ./wdl/checker/... -run TestCoerces` | ❌ W0 | ⬜ pending |
| 3-02-01 | 02 | 1 | TYPE-06 | unit | `go test ./wdl/checker/... -run TestForwardRef` | ❌ W0 | ⬜ pending |
| 3-02-02 | 02 | 1 | TYPE-06 | unit | `go test ./wdl/checker/... -run TestCycleDetection` | ❌ W0 | ⬜ pending |
| 3-02-03 | 02 | 1 | TYPE-06 | unit | `go test ./wdl/checker/... -run TestTypeMismatch` | ❌ W0 | ⬜ pending |
| 3-03-01 | 03 | 2 | TYPE-07 | unit | `go test ./wdl/checker/... -run TestScatterPromotion` | ❌ W0 | ⬜ pending |
| 3-03-02 | 03 | 2 | TYPE-07 | unit | `go test ./wdl/checker/... -run TestIfPromotion` | ❌ W0 | ⬜ pending |
| 3-03-03 | 03 | 2 | TYPE-07 | unit | `go test ./wdl/checker/... -run TestOptionalizeIdempotent` | ❌ W0 | ⬜ pending |
| 3-04-01 | 04 | 2 | TYPE-08 | unit | `go test ./wdl/checker/... -run TestStructLiteral` | ❌ W0 | ⬜ pending |
| 3-04-02 | 04 | 2 | TYPE-08 | unit | `go test ./wdl/checker/... -run TestStructFieldAccess` | ❌ W0 | ⬜ pending |
| 3-04-03 | 04 | 2 | TYPE-08 | unit | `go test ./wdl/checker/... -run TestStructUndefined` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `wdl/checker/checker_test.go` — test stubs for TYPE-05, TYPE-06, TYPE-07, TYPE-08 (all tests above)
- [ ] `wdl/checker/checker.go` — package skeleton with `Check()`, `TypeEnv`, `Scope`, `TypeError` signatures (stub implementations)
- [ ] `wdl/checker/coerce.go` — `Coerces()` function stub
- [ ] `wdl/checker/promote.go` — `arrayize()`, `optionalize()` function stubs
- [ ] `wdl/testdata/forward_ref.wdl` — WDL fixture for forward reference tests
- [ ] `wdl/testdata/scatter_if.wdl` — WDL fixture for scatter/if promotion tests
- [ ] `wdl/testdata/type_mismatch.wdl` — WDL fixture for type mismatch error tests
- [ ] `wdl/testdata/struct_literal.wdl` — WDL fixture for struct literal/field tests

---

## Manual-Only Verifications

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
