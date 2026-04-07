---
phase: 32-api-foundation
plan: "02"
subsystem: server
tags: [upload, file, multipart, api, security]
dependency_graph:
  requires: [32-01]
  provides: [API-02, upload-endpoint]
  affects: [34-browser-registry-upload, 36-ui-workflow-execution]
tech_stack:
  added: []
  patterns: [multipart/form-data, http.MaxBytesReader, filepath.Base sanitization, uuid-per-upload]
key_files:
  created:
    - internal/server/handlers_uploads.go
    - internal/server/handlers_uploads_test.go
  modified:
    - internal/api/types.go
    - internal/server/server.go
decisions:
  - "UploadResponse.Path is an absolute server-side filesystem path; clients pass this directly as a File-typed workflow input"
  - "UUID sub-directory per upload prevents filename collisions without content-addressing"
  - "MaxBytesReader applied before ParseMultipartForm — body limit enforced before any reading"
metrics:
  duration: "7 minutes"
  completed: "2026-04-07"
  tasks_completed: 1
  files_created: 2
  files_modified: 2
---

# Phase 32 Plan 02: File Upload Endpoint Summary

**One-liner:** Multipart file upload endpoint POST /projects/{slug}/uploads storing files at {base_dir}/projects/{slug}/uploads/{uuid}/{filename} with 100 MB limit, path-traversal sanitization, and executor-role gating.

## Objective

Add `POST /projects/{slug}/uploads` to flowy-server so that Browser and CLI clients can upload File-typed workflow inputs. The endpoint returns the server-accessible absolute path that clients pass as an `inputs` value on workflow submission.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| RED | Failing tests for upload endpoint | cae8ef0 | internal/server/handlers_uploads_test.go |
| GREEN | Upload DTO, handler, route registration | 700b640 | internal/api/types.go, internal/server/handlers_uploads.go, internal/server/server.go |

## Decisions Made

1. **Path format is absolute server-side path.** `UploadResponse.Path` is an absolute path on the server filesystem. Phase 34/36 clients pass this value directly as a File-typed input — no URL indirection layer.

2. **UUID per upload sub-directory.** Each upload gets its own `{uuid}/` prefix under `uploads/` to guarantee no filename collisions between concurrent uploads of identically-named files.

3. **MaxBytesReader before ParseMultipartForm.** The 100 MB body limit is applied via `http.MaxBytesReader` wrapping `r.Body` before any multipart parsing occurs. This prevents reading the full body into memory before rejecting oversized uploads.

4. **filepath.Base for path traversal prevention.** Filename is sanitized via `filepath.Base(header.Filename)` which strips all directory components including `../` sequences. The test confirms `../../../etc/passwd` becomes `/passwd`.

## Deviations from Plan

None — plan executed exactly as written.

## Verification Results

```
go test ./internal/server/ -run "TestUpload" -v -count=1
--- PASS: TestUpload (0.02s)
--- PASS: TestUpload_PathTraversal (0.01s)
--- PASS: TestUpload_MissingFileField (0.01s)
--- PASS: TestUpload_TooLarge (0.83s)
--- PASS: TestUpload_Auth_Executor (0.02s)
--- PASS: TestUpload_FileStored (0.01s)
PASS
ok  github.com/bioflowy/flowy/internal/server  0.929s

go test ./internal/server/... -count=1
ok  github.com/bioflowy/flowy/internal/server  7.013s
```

No regressions in the full server test suite.

## Self-Check: PASSED

- [x] internal/api/types.go contains `type UploadResponse struct` — FOUND
- [x] internal/server/handlers_uploads.go contains `func (s *Server) handleUpload(` — FOUND
- [x] internal/server/handlers_uploads.go contains `http.MaxBytesReader(w, r.Body, 100<<20)` — FOUND
- [x] internal/server/handlers_uploads.go contains `filepath.Base(header.Filename)` — FOUND
- [x] internal/server/server.go contains `Post("/uploads", s.handleUpload)` — FOUND
- [x] internal/server/handlers_uploads_test.go contains `func TestUpload(` — FOUND
- [x] internal/server/handlers_uploads_test.go contains `func TestUpload_TooLarge(` — FOUND
- [x] internal/server/handlers_uploads_test.go contains `func TestUpload_Auth` — FOUND
- [x] All TestUpload* tests PASS — VERIFIED
- [x] Full server suite passes — VERIFIED (7.013s)
- [x] Commits exist: cae8ef0 (tests), 700b640 (implementation) — VERIFIED
