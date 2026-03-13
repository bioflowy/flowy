---
phase: 15
slug: configuration-and-path-resolution
status: complete
nyquist_compliant: true
wave_0_complete: true
created: 2026-03-13
validated: 2026-03-13
---

# Phase 15 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing (stdlib) |
| **Config file** | none — standard `go test` |
| **Quick run command** | `go test ./internal/config/... ./wdl/pathutil/... ./internal/api/... ./internal/server/... ./cmd/flowy/... -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/config/... ./wdl/pathutil/... ./internal/api/... ./internal/server/... -count=1`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 15-01-01 | 01 | 1 | PATH-01 | unit | `go test ./wdl/pathutil/... -run TestCleanPath/s3_URI_unchanged -count=1` | yes | green |
| 15-01-02 | 01 | 1 | PATH-02 | unit | `go test ./wdl/pathutil/... -run TestCleanPath/s3_URI_with_wdlDir -count=1` | yes | green |
| 15-01-03 | 01 | 1 | PATH-03 | unit | `go test ./wdl/pathutil/... -run TestCleanPath/absolute_path -count=1` | yes | green |
| 15-01-04 | 01 | 1 | PATH-04 | unit | `go test ./wdl/pathutil/... -run TestCleanPath/relative -count=1` | yes | green |
| 15-01-05 | 01 | 1 | CONF-01 | unit | `go test ./internal/config/... -run TestLoadStorageConfig -count=1` | yes | green |
| 15-01-06 | 01 | 1 | CONF-01 | unit | `go test ./internal/config/... -run "TestValidate(LocalOK|S3OK|S3MissingBucket|UnknownType)" -count=1` | yes | green |
| 15-02-01 | 02 | 2 | CONF-03 | unit | `go test ./cmd/flowy/... -run "TestValidateStorageType|TestResolveStorageType|TestReorderArgsStorage" -count=1` | yes | green |
| 15-02-02 | 02 | 2 | CONF-04 | unit | `go test ./internal/server/... -run "TestClaim_StorageType" -count=1` | yes | green |
| 15-02-03 | 02 | 2 | CONF-02 | integration | `go test ./internal/config/... -run "TestLoadStorageDefaults|TestResolveStorageType" -count=1` | yes | green |

*Status: pending · green · red · flaky*

---

## Wave 0 Requirements

- [x] `wdl/pathutil/pathutil_test.go` — covers PATH-01 through PATH-04
- [x] `internal/config/config_test.go` — covers CONF-01, CONF-02
- [x] `cmd/flowy/main_test.go` — covers CONF-03
- [x] `internal/server/server_test.go` — covers CONF-04

*All test files exist with passing tests.*

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

**Approval:** complete (2026-03-13)
