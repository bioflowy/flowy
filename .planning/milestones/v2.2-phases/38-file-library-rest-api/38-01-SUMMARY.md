---
phase: 38-file-library-rest-api
plan: "01"
subsystem: server
tags: [api, managed-files, routes, rbac, config]
dependency_graph:
  requires: [37-backend-foundation]
  provides: [managed-file-dtos, import-root-config, files-route-wiring, list-delete-handlers]
  affects: [internal/api, internal/config, internal/server]
tech_stack:
  added: []
  patterns: [chi-route-guarding, store-backed-listing, cross-project-delete-guard]
key_files:
  created:
    - internal/server/handlers_files.go
    - internal/server/handlers_files_test.go
  modified:
    - internal/api/types.go
    - internal/config/config.go
    - internal/server/server.go
decisions:
  - GET /projects/{slug}/files is viewer-readable; POST and DELETE remain executor-only
  - DELETE checks record ownership by project before removing a managed file row
  - Config.ImportRoot is config-only; empty string disables server-local import mode without changing defaults
metrics:
  completed_date: "2026-04-13"
  tasks_completed: 1
  files_changed: 5
---

# Phase 38 Plan 01: API Types, Route Wiring, GET/DELETE Handlers Summary

**One-liner:** Added the managed-file API DTOs, `Config.ImportRoot`, `/projects/{slug}/files` route wiring, and store-backed GET/DELETE handlers with RBAC and cross-project guards.

## What Was Built

1. **Managed file DTOs** in `internal/api/types.go`:
   - `ManagedFileResponse`
   - `ListManagedFilesResponse`
   - `ImportFileRequest`
   - `ImportFileResponse`

2. **Config support** in `internal/config/config.go`:
   - Added `ImportRoot string \`yaml:"import_root"\`` to gate server-local imports without introducing a default path.

3. **Project route wiring** in `internal/server/server.go`:
   - `GET /projects/{slug}/files` behind `viewer`
   - `POST /projects/{slug}/files` behind `executor`
   - `DELETE /projects/{slug}/files/{id}` behind `executor`

4. **Managed-file handlers** in `internal/server/handlers_files.go`:
   - `managedFileToResponse`
   - `handleListFiles`
   - `handleDeleteFile`
   - shared helper for managed-file creation used by later import paths

5. **Plan 01 tests** in `internal/server/handlers_files_test.go`:
   - Empty list returns `{"files":[]}`
   - Viewer can list files
   - Unauthenticated list returns 401
   - Response field mapping is correct
   - Delete returns 204 for existing record
   - Delete returns 404 for nonexistent and cross-project IDs
   - Viewer delete returns 403

## Verification Results

```bash
GOCACHE=/tmp/go-build go test ./internal/server -run 'TestHandle(ListFiles|DeleteFile|ImportFile)' -count=1
GOCACHE=/tmp/go-build go test ./internal/server -count=1
```

Both commands passed.

## Deviations from Plan

None. Plan 01 scope was implemented as specified, with POST scaffolding living in the same handler file so Plan 02 could extend it cleanly.

## Commits

No commit created in this session.

## Self-Check

- [x] `internal/api/types.go` defines managed-file request/response DTOs
- [x] `internal/config/config.go` defines `ImportRoot`
- [x] `internal/server/server.go` wires `/files` routes
- [x] `internal/server/handlers_files.go` implements list/delete behavior
- [x] `internal/server/handlers_files_test.go` covers list/delete RBAC and cross-project guard
