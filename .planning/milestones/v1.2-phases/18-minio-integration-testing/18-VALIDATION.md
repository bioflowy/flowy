---
phase: 18
slug: minio-integration-testing
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-03-13
validated: 2026-03-13
---

# Phase 18 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing stdlib + testcontainers-go v0.41.0 |
| **Config file** | none (go test flags) |
| **Quick run command** | `go test -tags integration -timeout 2m -run TestS3RoundTrip ./internal/integration/...` |
| **Full suite command** | `go test -tags integration -timeout 2m ./internal/integration/...` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -tags integration -timeout 2m -run TestS3RoundTrip ./internal/integration/...`
- **After every plan wave:** Run `go test -tags integration -timeout 2m ./internal/integration/...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 18-01-01 | 01 | 1 | TEST-01 | integration | `go test -tags integration -timeout 2m -count=1 -v ./internal/integration/...` | yes | ✅ green |
| 18-01-02 | 01 | 1 | TEST-02 | integration | `go test -tags integration -timeout 2m -count=1 -v -run TestLED ./internal/daemon/...` | yes | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `internal/integration/main_test.go` — TestMain, MinIO container startup, bucket creation
- [x] `internal/integration/s3_roundtrip_test.go` — TEST-01 single-task S3 round-trip
- [x] `internal/daemon/led_integration_test.go` — TEST-02 LED localize/delocalize via S3 (replaces planned intermediate_test.go)
- [x] `go get github.com/testcontainers/testcontainers-go/modules/minio@v0.41.0` — add to go.mod
- [x] `go get github.com/testcontainers/testcontainers-go@v0.41.0` — add to go.mod
- [x] Makefile `spectool-s3` target — MinIO + server + daemon + spectool with S3 config
- [x] Makefile `integration` target — `go test -tags integration ...`

*If none: "Existing infrastructure covers all phase requirements."*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| spectool-s3 156/156 pass | TEST-01 | Requires Docker + MinIO + full spectool binary | Run `make spectool-s3` and verify 156/156 pass |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated 2026-03-13 (all integration tests executed and green)
