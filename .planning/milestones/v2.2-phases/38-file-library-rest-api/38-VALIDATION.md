---
phase: 38
slug: file-library-rest-api
status: verified
nyquist_compliant: true
wave_0_complete: true
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
| **Quick run command** | `GOCACHE=/tmp/go-build go test ./internal/server -run 'TestHandle(ListFiles\|DeleteFile\|ImportFile)' -count=1` |
| **Full suite command** | `GOCACHE=/tmp/go-build go test ./internal/server -count=1` |
| **Estimated runtime** | ~8 seconds |

---

## Sampling Rate

- **After every task commit:** Run `GOCACHE=/tmp/go-build go test ./internal/server -run 'TestHandle(ListFiles|DeleteFile|ImportFile)' -count=1`
- **After every plan wave:** Run `GOCACHE=/tmp/go-build go test ./internal/server -count=1`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 38-01-01 | 01 | 1 | FILE-01 | — | GET `/projects/{slug}/files` returns the managed file list for the current project | unit | `go test ./internal/server -run '^TestHandleListFiles_AuthAndFields$' -count=1` | ✅ | ✅ green |
| 38-01-02 | 01 | 1 | FILE-01 | — | Empty file library returns `{"files":[]}` rather than `null` | unit | `go test ./internal/server -run '^TestHandleListFiles_Empty$' -count=1` | ✅ | ✅ green |
| 38-01-03 | 01 | 1 | FILE-01 | T-38-01 | Viewer can list files and unauthenticated access is rejected | unit | `go test ./internal/server -run '^TestHandleListFiles_AuthAndFields$' -count=1` | ✅ | ✅ green |
| 38-01-04 | 01 | 1 | FILE-02 | — | DELETE removes an owned managed file record and returns `204` | unit | `go test ./internal/server -run '^TestHandleDeleteFile$' -count=1` | ✅ | ✅ green |
| 38-01-05 | 01 | 1 | FILE-02 | — | DELETE for a nonexistent managed file returns `404` | unit | `go test ./internal/server -run '^TestHandleDeleteFile_NonexistentAndCrossProject$' -count=1` | ✅ | ✅ green |
| 38-01-06 | 01 | 1 | FILE-02 | T-38-02 | DELETE for a cross-project managed file returns `404` without disclosing ownership | unit | `go test ./internal/server -run '^TestHandleDeleteFile_NonexistentAndCrossProject$' -count=1` | ✅ | ✅ green |
| 38-01-07 | 01 | 1 | FILE-02 | T-38-01 | Viewer DELETE is forbidden by project RBAC | unit | `go test ./internal/server -run '^TestHandleDeleteFile_ViewerForbidden$' -count=1` | ✅ | ✅ green |
| 38-02-01 | 02 | 1 | IMPORT-01 | — | Multipart browser upload stores the file and returns `201` with a managed file record | unit | `go test ./internal/server -run '^TestHandleImportFile_BrowserUpload$' -count=1` | ✅ | ✅ green |
| 38-02-02 | 02 | 1 | IMPORT-01 | T-38-03 | Multipart upload sanitizes a traversal filename to its basename | unit | `go test ./internal/server -run '^TestHandleImportFile_BrowserUploadPathTraversal$' -count=1` | ✅ | ✅ green |
| 38-02-03 | 02 | 1 | IMPORT-02 | — | Valid `s3://` import returns `201` and records size/label from storage metadata | unit | `go test ./internal/server -run '^TestHandleImportFile_S3URI$' -count=1` | ✅ | ✅ green |
| 38-02-04 | 02 | 1 | IMPORT-02 | — | Missing or inaccessible `s3://` object returns `400` | unit | `go test ./internal/server -run '^TestHandleImportFile_S3URINotFound$' -count=1` | ✅ | ✅ green |
| 38-02-05 | 02 | 1 | IMPORT-03 | T-38-04 | Local path import succeeds for a file inside `ImportRoot` | unit | `go test ./internal/server -run '^TestHandleImportFile_LocalPath$' -count=1` | ✅ | ✅ green |
| 38-02-06 | 02 | 1 | IMPORT-03 | T-38-04 | Local path import rejects a path outside `ImportRoot` with `400` | unit | `go test ./internal/server -run '^TestHandleImportFile_LocalPathOutsideImportRoot$' -count=1` | ✅ | ✅ green |
| 38-02-07 | 02 | 1 | IMPORT-03 | T-38-07 | Local path import is disabled when `ImportRoot` is unset | unit | `go test ./internal/server -run '^TestHandleImportFile_LocalPathImportRootUnset$' -count=1` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `internal/server/handlers_files_test.go` covers FILE-01, FILE-02, IMPORT-01, IMPORT-02, and IMPORT-03
- [x] Mock backend coverage for upload, S3 stat, and local-path validation exists in `internal/server/handlers_files_test.go`

Existing infrastructure covers all phase requirements.

---

## Manual-Only Verifications

All phase behaviors have automated verification.

---

## Validation Audit 2026-04-19

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

Audit evidence:
- `GOCACHE=/tmp/go-build go test ./internal/server -run 'TestHandle(ListFiles|DeleteFile|ImportFile)' -count=1` passed on 2026-04-19.
- `GOCACHE=/tmp/go-build go test ./internal/server -count=1` passed on 2026-04-19.
- `go test ./internal/server -list 'TestHandle(ListFiles|DeleteFile|ImportFile)'` confirms the exact handler test inventory matches the requirement map.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 10s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-04-19
