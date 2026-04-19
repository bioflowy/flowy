---
phase: 38-file-library-rest-api
plan: "02"
subsystem: server
tags: [api, imports, multipart, s3, local-path]
dependency_graph:
  requires: [38-01]
  provides: [browser-import-handler, s3-import-handler, local-import-guard, import-tests]
  affects: [internal/server]
tech_stack:
  added: []
  patterns: [content-type-dispatch, filepath-root-guard, backend-stat-validation, mock-backend-tests]
key_files:
  created: []
  modified:
    - internal/server/handlers_files.go
    - internal/server/handlers_files_test.go
decisions:
  - POST /projects/{slug}/files dispatches by Content-Type: multipart for browser uploads, JSON for URI imports
  - S3 imports validate existence through `storage.ParseS3URI` plus `backend.Stat(key)`
  - Local imports require `ImportRoot` and reject any path outside that cleaned root
metrics:
  completed_date: "2026-04-13"
  tasks_completed: 1
  files_changed: 2
---

# Phase 38 Plan 02: Import Handler and Tests Summary

**One-liner:** Implemented browser upload, S3 URI import, and guarded local-path import for `POST /projects/{slug}/files`, with test coverage for success and failure paths.

## What Was Built

1. **Browser upload import**
   - Accepts multipart `file`
   - Sanitizes the filename with `filepath.Base`
   - Stores through `backend.UploadFileStream`
   - Registers a managed-file record and returns `{"file": ...}`

2. **S3 URI import**
   - Accepts JSON `{"uri":"s3://bucket/key"}`
   - Validates format with `storage.ParseS3URI`
   - Uses `backend.Stat(key)` to confirm object availability and size
   - Stores the full URI in the managed-file record

3. **Server-local import**
   - Requires `cfg.ImportRoot`
   - Resolves absolute paths for both root and target
   - Rejects targets outside the cleaned import root
   - Registers the local file path when validation succeeds

4. **Test coverage**
   - Browser upload success
   - Browser filename traversal sanitization
   - S3 import success
   - S3 import missing object
   - Local import within root
   - Local import outside root
   - Local import when `ImportRoot` is unset
   - Viewer POST forbidden

## Verification Results

```bash
GOCACHE=/tmp/go-build go test ./internal/server -run 'TestHandle(ListFiles|DeleteFile|ImportFile)' -count=1
GOCACHE=/tmp/go-build go test ./internal/server -count=1
```

Both commands passed.

## Deviations from Plan

None. The final handler uses a small shared record-creation helper so all three import paths return a consistent response shape.

## Commits

No commit created in this session.

## Self-Check

- [x] `handleImportFile` dispatches by Content-Type
- [x] Browser uploads create managed-file records
- [x] S3 URIs are parsed and validated before registration
- [x] Local paths are constrained to `ImportRoot`
- [x] `internal/server/handlers_files_test.go` covers all import modes and edge cases
