---
phase: 14
slug: storage-abstraction
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-12
---

# Phase 14 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing (stdlib) |
| **Config file** | none — standard `go test` |
| **Quick run command** | `go test ./internal/storage/... ./internal/server/... ./internal/store/... -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/storage/... ./internal/server/... ./internal/store/... -count=1`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 14-01-01 | 01 | 1 | STOR-01 | compilation | `go build ./internal/storage/...` | ❌ W0 | ⬜ pending |
| 14-01-02 | 01 | 1 | STOR-01 | unit | `go test ./internal/storage/... -run TestParseS3URI -count=1` | ❌ W0 | ⬜ pending |
| 14-01-03 | 01 | 1 | STOR-02 | unit | `go test ./internal/storage/... -run TestLocalBackend -count=1` | ❌ W0 | ⬜ pending |
| 14-01-04 | 01 | 1 | STOR-02 | unit | `go test ./internal/storage/... -run TestLocalBackendJSON -count=1` | ❌ W0 | ⬜ pending |
| 14-01-05 | 01 | 1 | STOR-03 | integration | `go test ./internal/server/... -count=1` | ✅ | ⬜ pending |
| 14-01-06 | 01 | 1 | STOR-03 | unit | `go test ./internal/store/... -count=1` | ✅ | ⬜ pending |
| 14-01-07 | 01 | 1 | STOR-03 | integration | `go test ./... -count=1` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/storage/backend_test.go` — stubs for ParseS3URI (STOR-01)
- [ ] `internal/storage/local_test.go` — stubs for LocalBackend path helpers and JSON roundtrip (STOR-02)
- [ ] `internal/storage/` package — does not exist yet, must be created

*Existing test infrastructure (server_test.go, rundir_test.go) covers STOR-03 requirements.*

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
