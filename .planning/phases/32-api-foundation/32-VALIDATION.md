---
phase: 32
slug: api-foundation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-07
---

# Phase 32 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard Go test toolchain |
| **Quick run command** | `go test ./internal/server/... -run TestInputs\|TestUpload -count=1` |
| **Full suite command** | `go test ./internal/server/... -count=1` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/server/... -run TestInputs\|TestUpload -count=1`
- **After every plan wave:** Run `go test ./internal/server/... -count=1`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 32-01-01 | 01 | 1 | API-01 | — | N/A | integration | `go test ./internal/server/... -run TestInputs -count=1` | ❌ W0 | ⬜ pending |
| 32-01-02 | 01 | 1 | API-02 | — | MaxBytesReader applied before ParseMultipartForm | integration | `go test ./internal/server/... -run TestUpload -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/server/handlers_inputs_test.go` — tests for input schema endpoint (API-01)
- [ ] `internal/server/handlers_upload_test.go` — tests for file upload endpoint (API-02)

*Existing test infrastructure (`newAuthTestServer`, `doAuthRequest`, `issueTestToken`) covers shared fixtures.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| S3 upload path accessible by daemon | API-02 | Requires running daemon + S3 | Upload file, verify daemon can read returned path |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
