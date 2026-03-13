---
phase: 14
slug: storage-abstraction
status: complete
nyquist_compliant: true
wave_0_complete: true
created: 2026-03-12
validated: 2026-03-13
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
| 14-01-01 | 01 | 1 | STOR-01 | compilation | `go build ./internal/storage/...` | ✅ | ✅ green |
| 14-01-02 | 01 | 1 | STOR-01 | unit | `go test ./internal/storage/... -run TestParseS3URI -count=1` | ✅ | ✅ green |
| 14-01-03 | 01 | 1 | STOR-02 | unit | `go test ./internal/storage/... -run TestLocalBackend -count=1` | ✅ | ✅ green |
| 14-01-04 | 01 | 1 | STOR-02 | unit | `go test ./internal/storage/... -run TestLocalBackend_WriteReadJSON -count=1` | ✅ | ✅ green |
| 14-01-05 | 01 | 1 | STOR-03 | integration | `go test ./internal/server/... -count=1` | ✅ | ✅ green |
| 14-01-06 | 01 | 1 | STOR-03 | unit | `go test ./internal/store/... -count=1` | ✅ | ✅ green |
| 14-01-07 | 01 | 1 | STOR-03 | integration | `go test ./... -count=1` | ✅ | ✅ green |
| 14-NY-01 | NY | - | STOR-02 | unit | `go test ./internal/storage/... -run TestLocalBackend_UploadDownloadFileStream_Roundtrip -count=1` | ✅ | ✅ green |
| 14-NY-02 | NY | - | STOR-02 | unit | `go test ./internal/storage/... -run TestLocalBackend_UploadFileStream_CreatesIntermediateDirs -count=1` | ✅ | ✅ green |
| 14-NY-03 | NY | - | STOR-02 | unit | `go test ./internal/storage/... -run TestLocalBackend_DownloadFileStream_NotFound -count=1` | ✅ | ✅ green |
| 14-NY-04 | NY | - | STOR-02 | unit | `go test ./internal/storage/... -run TestNewBackend_ReturnsLocalBackend -count=1` | ✅ | ✅ green |
| 14-NY-05 | NY | - | STOR-01 | unit | `go test ./internal/storage/... -run TestParseS3URI_PreservesSpecialChars -count=1` | ✅ | ✅ green |

*Status: pending · ✅ green · red · flaky*

---

## Wave 0 Requirements

- [x] `internal/storage/backend_test.go` — tests for ParseS3URI (STOR-01)
- [x] `internal/storage/local_test.go` — tests for LocalBackend path helpers and JSON roundtrip (STOR-02)
- [x] `internal/storage/` package — exists with backend.go, local.go, factory.go
- [x] `internal/storage/nyquist_phase14_test.go` — streaming I/O, factory type assertion, special char S3 URIs

*Existing test infrastructure (server_test.go, rundir_test.go) covers STOR-03 requirements.*

---

## Manual-Only Verifications

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 15s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** complete (2026-03-13, Nyquist auditor)
