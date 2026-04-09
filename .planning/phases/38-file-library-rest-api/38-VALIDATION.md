---
phase: 38
slug: file-library-rest-api
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-09
---

# Phase 38 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard `testing` package |
| **Config file** | none — `go test ./...` convention |
| **Quick run command** | `go test ./internal/server/... -run TestHandleFiles -v` |
| **Full suite command** | `go test ./internal/server/... -count=1` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/server/... -count=1`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 38-01-01 | 01 | 1 | FILE-01 | — | N/A | unit | `go test ./internal/server/... -run TestHandleListFiles` | ❌ W0 | ⬜ pending |
| 38-01-02 | 01 | 1 | FILE-01 | — | Empty list returns `{"files":[]}` not null | unit | `go test ./internal/server/... -run TestHandleListFiles_Empty` | ❌ W0 | ⬜ pending |
| 38-01-03 | 01 | 1 | FILE-01 | T-38-01 | Viewer gets 200; unauthenticated gets 401 | unit | `go test ./internal/server/... -run TestHandleListFiles_Auth` | ❌ W0 | ⬜ pending |
| 38-01-04 | 01 | 1 | FILE-02 | — | DELETE existing record returns 204 | unit | `go test ./internal/server/... -run TestHandleDeleteFile_Success` | ❌ W0 | ⬜ pending |
| 38-01-05 | 01 | 1 | FILE-02 | — | DELETE nonexistent ID returns 404 | unit | `go test ./internal/server/... -run TestHandleDeleteFile_NotFound` | ❌ W0 | ⬜ pending |
| 38-01-06 | 01 | 1 | FILE-02 | T-38-02 | DELETE cross-project ID returns 404 | unit | `go test ./internal/server/... -run TestHandleDeleteFile_CrossProject` | ❌ W0 | ⬜ pending |
| 38-01-07 | 01 | 1 | FILE-02 | T-38-01 | Viewer DELETE gets 403 | unit | `go test ./internal/server/... -run TestHandleDeleteFile_Auth` | ❌ W0 | ⬜ pending |
| 38-02-01 | 02 | 1 | IMPORT-01 | — | POST multipart stores file and returns 201 with ManagedFile record | unit | `go test ./internal/server/... -run TestHandleImportFile_Browser` | ❌ W0 | ⬜ pending |
| 38-02-02 | 02 | 1 | IMPORT-01 | T-38-03 | POST multipart path traversal in filename is sanitized | unit | `go test ./internal/server/... -run TestHandleImportFile_Browser_PathTraversal` | ❌ W0 | ⬜ pending |
| 38-02-03 | 02 | 1 | IMPORT-02 | — | POST S3 URI with valid object returns 201 | unit | `go test ./internal/server/... -run TestHandleImportFile_S3` | ❌ W0 | ⬜ pending |
| 38-02-04 | 02 | 1 | IMPORT-02 | — | POST S3 URI with nonexistent object returns 400 | unit | `go test ./internal/server/... -run TestHandleImportFile_S3_NotFound` | ❌ W0 | ⬜ pending |
| 38-02-05 | 02 | 1 | IMPORT-03 | T-38-04 | POST local path within ImportRoot returns 201 | unit | `go test ./internal/server/... -run TestHandleImportFile_LocalPath` | ❌ W0 | ⬜ pending |
| 38-02-06 | 02 | 1 | IMPORT-03 | T-38-04 | POST local path traversal outside ImportRoot returns 400 | unit | `go test ./internal/server/... -run TestHandleImportFile_LocalPath_Traversal` | ❌ W0 | ⬜ pending |
| 38-02-07 | 02 | 1 | IMPORT-03 | T-38-04 | POST local path when ImportRoot unset returns 400 | unit | `go test ./internal/server/... -run TestHandleImportFile_LocalPath_NoImportRoot` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/server/handlers_files_test.go` — covers FILE-01, FILE-02, IMPORT-01, IMPORT-02, IMPORT-03 (new file)
- [ ] Mock backend for S3 tests: reuse or extend `mockBackend` from existing server tests

*Existing test infrastructure (`newAuthTestServer`, `issueTestToken`, `createTestUser`, `setupUploadTest` helpers in handlers_*_test.go) covers all phase requirements.*

---

## Manual-Only Verifications

None — all phase behaviors have automated verification.

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
