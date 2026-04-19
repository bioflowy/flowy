---
phase: 37
slug: backend-foundation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-09
---

# Phase 37 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard `testing` package |
| **Config file** | none |
| **Quick run command** | `go test ./internal/store/... ./internal/storage/... -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~15 seconds (quick), ~60 seconds (full) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/store/... ./internal/storage/... -count=1`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| migration | 01 | 1 | SC-1 | — | N/A | unit | `go test ./internal/store/sqlite/... -run TestManagedFilesSchema -count=1` | ❌ W0 | ⬜ pending |
| store-crud | 01 | 1 | SC-2a | — | N/A | unit | `go test ./internal/store/sqlite/... -run TestManagedFile -count=1` | ❌ W0 | ⬜ pending |
| complete-run | 01 | 1 | SC-2b | — | N/A | unit | `go test ./internal/store/sqlite/... -run TestCompleteRun -count=1` | ❌ W0 | ⬜ pending |
| local-stat | 01 | 1 | SC-3 | — | N/A | unit | `go test ./internal/storage/... -run TestLocalBackend_Stat -count=1` | ❌ W0 | ⬜ pending |
| s3-stat | 01 | 1 | SC-4 | — | N/A | unit | `go test ./internal/storage/... -run TestS3Backend_Stat -count=1` | ❌ W0 | ⬜ pending |
| regression | 01 | 1 | SC-5 | — | N/A | regression | `go test ./internal/store/... ./internal/storage/... -count=1` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/store/sqlite/managed_files_test.go` — tests for SC-1, SC-2a, SC-2b (schema check, CRUD round-trip, CompleteRun atomicity)
- [ ] `internal/storage/stat_test.go` — tests for SC-3 (LocalBackend.Stat) and SC-4 (S3Backend.Stat via mock HeadObject)

---

## Manual-Only Verifications

All phase behaviors have automated verification.

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
