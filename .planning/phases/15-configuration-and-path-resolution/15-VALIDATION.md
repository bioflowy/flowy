---
phase: 15
slug: configuration-and-path-resolution
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-13
---

# Phase 15 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing (stdlib) |
| **Config file** | none — standard `go test` |
| **Quick run command** | `go test ./internal/config/... ./wdl/pathutil/... ./internal/api/... ./internal/server/... -count=1` |
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
| 15-01-01 | 01 | 1 | PATH-01 | unit | `go test ./wdl/pathutil/... -run TestCleanPath -count=1` | ❌ W0 | ⬜ pending |
| 15-01-02 | 01 | 1 | PATH-02 | unit | `go test ./wdl/pathutil/... -run TestCleanPath_S3IgnoresWdlDir -count=1` | ❌ W0 | ⬜ pending |
| 15-01-03 | 01 | 1 | PATH-03 | unit | `go test ./wdl/pathutil/... -run TestCleanPath_AbsLocal -count=1` | ❌ W0 | ⬜ pending |
| 15-01-04 | 01 | 1 | PATH-04 | unit | `go test ./wdl/pathutil/... -run TestCleanPath_Relative -count=1` | ❌ W0 | ⬜ pending |
| 15-01-05 | 01 | 1 | CONF-01 | unit | `go test ./internal/config/... -run TestLoadStorageConfig -count=1` | ❌ W0 | ⬜ pending |
| 15-01-06 | 01 | 1 | CONF-01 | unit | `go test ./internal/config/... -run TestValidate -count=1` | ❌ W0 | ⬜ pending |
| 15-02-01 | 02 | 2 | CONF-03 | unit | `go test ./cmd/flowy/... -run TestRunStorageFlag -count=1` | ❌ W0 | ⬜ pending |
| 15-02-02 | 02 | 2 | CONF-04 | unit | `go test ./internal/server/... -run TestHandleClaim_StorageFields -count=1` | ❌ W0 | ⬜ pending |
| 15-02-03 | 02 | 2 | CONF-02 | integration | `go test ./... -count=1` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `wdl/pathutil/pathutil_test.go` — stubs for PATH-01 through PATH-04
- [ ] `internal/config/config_test.go` additions — stubs for CONF-01, CONF-02 (file exists, add cases)
- [ ] `internal/server/server_test.go` additions — stubs for CONF-04 claim response fields

*Existing test infrastructure (go test ./...) covers CONF-02 integration requirements.*

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
