---
phase: 21
slug: wdl-registry
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-20
---

# Phase 21 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing (`testing` package), httptest |
| **Config file** | none — stdlib testing |
| **Quick run command** | `go test ./internal/server/ -run TestRegistry -v` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/server/ -run TestRegistry -v`
- **After every plan wave:** Run `go test ./internal/server/ ./internal/store/... -v`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 21-01-01 | 01 | 1 | REG-01 | unit | `go test ./internal/server/ -run TestRegistryPush -v` | ❌ W0 | ⬜ pending |
| 21-01-02 | 01 | 1 | REG-01 | unit | `go test ./internal/server/ -run TestRegistryPush_InvalidWDL -v` | ❌ W0 | ⬜ pending |
| 21-01-03 | 01 | 1 | REG-02 | unit | `go test ./internal/server/ -run TestRegistryTag -v` | ❌ W0 | ⬜ pending |
| 21-01-04 | 01 | 1 | REG-02 | unit | `go test ./internal/server/ -run TestRegistryTag_Forbidden -v` | ❌ W0 | ⬜ pending |
| 21-01-05 | 01 | 1 | REG-03 | unit | `go test ./internal/server/ -run TestRegistryPull_ByTag -v` | ❌ W0 | ⬜ pending |
| 21-01-06 | 01 | 1 | REG-03 | unit | `go test ./internal/server/ -run TestRegistryPull_ByDigest -v` | ❌ W0 | ⬜ pending |
| 21-01-07 | 01 | 1 | REG-03 | unit | `go test ./internal/server/ -run TestRegistryPull_NotFound -v` | ❌ W0 | ⬜ pending |
| 21-01-08 | 01 | 1 | REG-04 | unit | `go test ./internal/server/ -run TestRegistryList -v` | ❌ W0 | ⬜ pending |
| 21-01-09 | 01 | 1 | REG-04 | unit | `go test ./internal/server/ -run TestRegistryListVersions -v` | ❌ W0 | ⬜ pending |
| 21-01-10 | 01 | 1 | REG-05 | unit | `go test ./internal/server/ -run TestRegistryPush_Classification -v` | ❌ W0 | ⬜ pending |
| 21-01-11 | 01 | 1 | REG-06 | unit | `go test ./internal/server/ -run TestRegistryDelete -v` | ❌ W0 | ⬜ pending |
| 21-01-12 | 01 | 1 | REG-06 | unit | `go test ./internal/server/ -run TestRegistryDelete_Forbidden -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/server/handlers_registry_test.go` — stubs for REG-01 through REG-06
- [ ] `internal/store/sqlite/registry.go` — store implementation (needed before handler tests can run)
- [ ] `internal/store/sqlite/migrations/00006_registry.sql` — schema needed before store tests

*Existing infrastructure covers test framework — reuses stdlib testing + httptest already in use*

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
