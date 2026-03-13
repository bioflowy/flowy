---
phase: 17
slug: daemon-staging
status: complete
nyquist_compliant: true
wave_0_complete: true
created: 2026-03-13
validated: 2026-03-13
---

# Phase 17 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard `testing` package (go 1.25) |
| **Config file** | none — go test ./... |
| **Quick run command** | `go test ./internal/daemon/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/daemon/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 17-01-01 | 01 | 1 | DAEM-01 | unit | `go test ./internal/daemon/... -run TestLocalizeInputs_CreatesStageDir` | YES | green |
| 17-01-02 | 01 | 1 | DAEM-01 | unit | `go test ./internal/daemon/... -run TestWalkAndLocalize` | YES | green |
| 17-01-03 | 01 | 1 | DAEM-02 | unit | `go test ./internal/daemon/... -run TestDelocalizeOutputs` | YES | green |
| 17-01-04 | 01 | 1 | DAEM-02 | unit | `go test ./internal/daemon/... -run TestReportResultWithOutputs` | YES | green |
| 17-01-05 | 01 | 1 | DAEM-03 | unit | `go test ./internal/daemon/... -run TestStageDirCleanupOnSuccess` | YES | green |
| 17-01-06 | 01 | 1 | DAEM-03 | unit | `go test ./internal/daemon/... -run TestStageDirPreservedOnDelocalizeFailure` | YES | green |
| 17-01-07 | 01 | 1 | DAEM-04 | unit | `go test ./internal/daemon/... -run TestLocalizeFailureReported` | YES | green |
| 17-01-08 | 01 | 1 | DAEM-04 | unit | `go test ./internal/daemon/... -run TestDelocalizeFailureReported` | YES | green |
| 17-02-01 | 02 | 1 | DAEM-01..04 | unit | `go test ./internal/api/... -run TestAPITypesJSON` | YES | green |
| 17-02-02 | 02 | 1 | DAEM-01..04 | unit | `go test ./internal/server/... -run TestHandleClaim_Inputs` | YES | green |
| 17-02-03 | 02 | 1 | DAEM-01..04 | unit | `go test ./internal/server/... -run TestHandleResult_Outputs` | YES | green |

*Status: pending · green · red · flaky*

---

## Wave 0 Requirements

- [x] `internal/daemon/daemon_test.go` — extended with localize/delocalize/staging/error-reporting test functions
- [x] `internal/server/server_test.go` — extended with handleClaim_Inputs and handleResult_Outputs tests
- [x] `internal/store/sqlite/migrations/00002_task_outputs.sql` — migration for outputs_json column
- [x] `internal/store/sqlite/tasks.go` — UpdateTaskOutputs method

*Existing infrastructure covers framework needs — `go test` already works.*

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
