---
phase: 16
slug: s3backend-implementation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-13
---

# Phase 16 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib testing (go test) |
| **Config file** | none — go test ./... discovers tests by convention |
| **Quick run command** | `go test ./internal/storage/... -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/storage/... -count=1`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 16-01-01 | 01 | 1 | S3-01 | unit | `go test ./internal/storage/... -run TestS3Backend_UploadFile -count=1` | ❌ W0 | ⬜ pending |
| 16-01-02 | 01 | 1 | S3-01 | unit | `go test ./internal/storage/... -run TestS3Backend_UploadFile_Error -count=1` | ❌ W0 | ⬜ pending |
| 16-01-03 | 01 | 1 | S3-02 | unit | `go test ./internal/storage/... -run TestS3Backend_DownloadFile -count=1` | ❌ W0 | ⬜ pending |
| 16-01-04 | 01 | 1 | S3-02 | unit | `go test ./internal/storage/... -run TestS3Backend_DownloadFile_Error -count=1` | ❌ W0 | ⬜ pending |
| 16-01-05 | 01 | 1 | S3-03 | unit | `go test ./internal/storage/... -run TestS3Backend_UploadFile_Multipart -count=1` | ❌ W0 | ⬜ pending |
| 16-01-06 | 01 | 1 | S3-04 | unit | `go test ./internal/storage/... -run TestNewS3Backend_MinIOConfig -count=1` | ❌ W0 | ⬜ pending |
| 16-01-07 | 01 | 1 | S3-05 | unit | `go test ./internal/storage/... -run TestS3Backend_TaskDir -count=1` | ❌ W0 | ⬜ pending |
| 16-01-08 | 01 | 1 | S3-05 | unit | `go test ./internal/storage/... -run TestS3Backend_UploadFile_ScatterKey -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/storage/s3.go` — S3Backend struct and all Backend interface methods
- [ ] `internal/storage/s3_test.go` — mockS3Client + unit tests covering S3-01 through S3-05
- [ ] `go.mod` / `go.sum` additions: `aws-sdk-go-v2/service/s3`, `aws-sdk-go-v2/feature/s3/transfermanager`, `aws-sdk-go-v2/config`, `aws-sdk-go-v2/credentials`, `aws-sdk-go-v2/aws`

*Existing infrastructure covers test framework — only new test files and dependencies needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| MinIO end-to-end upload/download | S3-04 | Requires running MinIO instance | Start MinIO with docker, configure endpoint, upload and download a file |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
