---
phase: 23
slug: provenance-tracking
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-20
---

# Phase 23 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing (stdlib) |
| **Config file** | none — `go test ./...` discovers tests automatically |
| **Quick run command** | `go test ./internal/server/ -run TestProvenance -v` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/server/ -run TestProvenance -v`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 23-01-01 | 01 | 1 | PROV-01 | unit | `go test ./internal/store/... -run TestProvenance -v` | ❌ W0 | ⬜ pending |
| 23-01-02 | 01 | 1 | PROV-01 | integration | `go test ./internal/server/ -run TestProvenanceCreatedOnSubmit -v` | ❌ W0 | ⬜ pending |
| 23-01-03 | 01 | 1 | PROV-01 | integration | `go test ./internal/server/ -run TestProvenanceNotCreatedForFilePathRun -v` | ❌ W0 | ⬜ pending |
| 23-01-04 | 01 | 1 | PROV-01 | integration | `go test ./internal/server/ -run TestProvenanceCreatedOnTaskRun -v` | ❌ W0 | ⬜ pending |
| 23-02-01 | 02 | 1 | PROV-02 | integration | `go test ./internal/server/ -run TestProvenanceUpdatedOnCompletion -v` | ❌ W0 | ⬜ pending |
| 23-02-02 | 02 | 1 | PROV-02 | integration | `go test ./internal/server/ -run TestProvenanceUpdatedOnFailure -v` | ❌ W0 | ⬜ pending |
| 23-03-01 | 02 | 1 | PROV-03 | integration | `go test ./internal/server/ -run TestHandleListProvenance -v` | ❌ W0 | ⬜ pending |
| 23-03-02 | 02 | 1 | PROV-03 | integration | `go test ./internal/server/ -run TestHandleListProvenanceEntryFilter -v` | ❌ W0 | ⬜ pending |
| 23-03-03 | 02 | 1 | PROV-03 | integration | `go test ./internal/server/ -run TestHandleListProvenanceUserFilter -v` | ❌ W0 | ⬜ pending |
| 23-03-04 | 02 | 1 | PROV-03 | integration | `go test ./internal/server/ -run TestHandleListProvenanceProjectScope -v` | ❌ W0 | ⬜ pending |
| 23-04-01 | 02 | 1 | PROV-04 | integration | `go test ./internal/server/ -run TestHandleGetProvenance -v` | ❌ W0 | ⬜ pending |
| 23-04-02 | 02 | 1 | PROV-04 | integration | `go test ./internal/server/ -run TestHandleGetProvenanceFields -v` | ❌ W0 | ⬜ pending |
| 23-04-03 | 02 | 1 | PROV-04 | integration | `go test ./internal/server/ -run TestHandleGetProvenanceNotFound -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/server/handlers_provenance_test.go` — integration tests for PROV-01 through PROV-04
- [ ] `internal/store/sqlite/provenance.go` — store implementation
- [ ] `internal/store/sqlite/migrations/00008_provenance.sql` — migration

*Existing infrastructure covers test helpers: `newAuthTestServer`, `issueTestToken`, `createTestUser`, `createRegistryTestProject`, `doAuthRequest`, `pushRegistryEntry`.*

---

## Manual-Only Verifications

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
