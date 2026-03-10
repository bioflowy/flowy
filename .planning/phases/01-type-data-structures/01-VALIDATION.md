---
phase: 1
slug: type-data-structures
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-10
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` package (stdlib) |
| **Config file** | none — `go test ./...` is self-contained |
| **Quick run command** | `go test ./wdl/types/... ./wdl/values/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~2 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./wdl/types/... ./wdl/values/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 1-01-01 | 01 | 0 | TYPE-01 | unit | `go test ./wdl/types/... -run TestTypeName` | ❌ W0 | ⬜ pending |
| 1-01-02 | 01 | 0 | TYPE-01 | unit | `go test ./wdl/values/... -run TestValueType` | ❌ W0 | ⬜ pending |
| 1-01-03 | 01 | 0 | TYPE-02 | unit | `go test ./wdl/types/... -run TestDirectory` | ❌ W0 | ⬜ pending |
| 1-01-04 | 01 | 0 | TYPE-03 | unit | `go test ./wdl/types/... -run TestCompound` | ❌ W0 | ⬜ pending |
| 1-01-05 | 01 | 0 | TYPE-03 | unit | `go test ./wdl/types/... -run TestArrayNonempty` | ❌ W0 | ⬜ pending |
| 1-01-06 | 01 | 0 | TYPE-04 | unit | `go test ./wdl/types/... -run TestWithOptional` | ❌ W0 | ⬜ pending |
| 1-01-07 | 01 | 0 | TYPE-04 | unit | `go test ./wdl/values/... -run TestNullValue` | ❌ W0 | ⬜ pending |
| 1-01-08 | 01 | 0 | TYPE-04 | unit | `go test ./wdl/types/... -run TestAnyType` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `wdl/types/types.go` — all type structs (TYPE-01, TYPE-02, TYPE-03, TYPE-04)
- [ ] `wdl/types/types_test.go` — table-driven tests for TypeName, IsOptional, WithOptional
- [ ] `wdl/values/values.go` — all value structs (TYPE-01, TYPE-02, TYPE-03, TYPE-04)
- [ ] `wdl/values/values_test.go` — table-driven tests for ValueType
- [ ] `go.mod` — `go mod init github.com/bioflowy/flowy`

*No existing test infrastructure — greenfield project.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `go.mod` module path is `github.com/bioflowy/flowy` with Go 1.24 | TYPE-01 | File content check, not logic | `cat go.mod` and verify module line |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
