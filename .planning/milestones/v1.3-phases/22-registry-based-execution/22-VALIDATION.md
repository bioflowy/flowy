---
phase: 22
slug: registry-based-execution
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-20
---

# Phase 22 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing (stdlib) with httptest |
| **Config file** | none — tests use `sqlite.OpenMemory()` and `httptest.NewRequest` |
| **Quick run command** | `go test ./internal/server/... -run TestRegistryRef -v` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/server/... -count=1`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 22-01-01 | 01 | 1 | EXEC-01 | unit | `go test ./cmd/flowy/... -run TestIsRegistryRef -v` | ❌ W0 | ⬜ pending |
| 22-01-02 | 01 | 1 | EXEC-01 | integration | `go test ./internal/server/... -run TestHandleSubmitRegistryRef -v` | ❌ W0 | ⬜ pending |
| 22-01-03 | 01 | 1 | EXEC-01 | integration | `go test ./internal/server/... -run TestHandleTaskRunRegistryRef -v` | ❌ W0 | ⬜ pending |
| 22-01-04 | 01 | 1 | EXEC-01 | integration | `go test ./internal/server/... -run TestHandleSubmitBothFields -v` | ❌ W0 | ⬜ pending |
| 22-01-05 | 01 | 1 | EXEC-01 | integration | `go test ./internal/server/... -run TestHandleSubmitNeitherField -v` | ❌ W0 | ⬜ pending |
| 22-01-06 | 01 | 1 | EXEC-01 | integration | `go test ./internal/server/... -run TestHandleSubmitRegistryRefNoProject -v` | ❌ W0 | ⬜ pending |
| 22-01-07 | 01 | 1 | EXEC-01 | integration | `go test ./internal/server/... -run TestHandleSubmitRegistryRefNotFound -v` | ❌ W0 | ⬜ pending |
| 22-02-01 | 02 | 1 | EXEC-02 | integration | `go test ./internal/server/... -run TestHandleClaimRegistryRun -v` | ❌ W0 | ⬜ pending |
| 22-02-02 | 02 | 1 | EXEC-02 | integration | `go test ./internal/server/... -run TestRegistryRunPersistence -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/server/handlers_registry_exec_test.go` — stubs for EXEC-01 and EXEC-02 server-side tests
- [ ] `cmd/flowy/main_test.go` additions — stubs for EXEC-01 CLI auto-detection tests
- [ ] `internal/store/sqlite/migrations/00007_run_registry_ref.sql` — migration needed before store tests

*Existing infrastructure covers test helpers: `sqlite.OpenMemory()`, `newTestServer()`, `doRequest()`, `decodeJSON()`, `createRegistryTestProject()`, `addProjectMember()`.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Windows drive letter not detected as registry ref | EXEC-01 | Cross-platform behavior | Run `isRegistryRef("C:\\path\\file.wdl")` unit test on Linux — passes; verify logic handles single-letter prefix |

*All other phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
