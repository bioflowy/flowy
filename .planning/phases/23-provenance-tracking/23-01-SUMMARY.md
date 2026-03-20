---
phase: 23-provenance-tracking
plan: "01"
subsystem: provenance
tags: [provenance, sqlite, migration, store, handlers, dispatch]
dependency_graph:
  requires: [registry-store, run-store, auth-middleware]
  provides: [provenance-write-api]
  affects: [handleSubmit, handleTaskRun, runWorkflow, waitForTaskAndFinalize]
tech_stack:
  added: []
  patterns: [store-interface-extension, non-fatal-audit-logging, tdd-red-green]
key_files:
  created:
    - internal/store/sqlite/migrations/00008_provenance.sql
    - internal/store/sqlite/provenance.go
    - internal/store/sqlite/provenance_test.go
    - internal/server/handlers_provenance_test.go
  modified:
    - internal/store/store.go
    - internal/server/handlers.go
    - internal/server/dispatch.go
decisions:
  - "TouchProvenanceUpdatedAt returns nil for non-existent run_id — missing record is not an error, enabling calls before provenance exists without special-casing"
  - "Provenance creation is non-fatal in handlers — audit failure must not block run execution"
  - "resolveRegistryRef now delegates to resolveRegistryRefFull to avoid code duplication while exposing entryID and versionID"
  - "claimsFromContext nil-guard in handlers — provenance is skipped if auth is disabled (claims == nil)"
metrics:
  duration: "~5 minutes"
  completed_date: "2026-03-20"
  tasks_completed: 2
  tasks_total: 2
  files_created: 4
  files_modified: 3
---

# Phase 23 Plan 01: Provenance Write Side Summary

SQLite migration, store model, and handler integration for registry-based run provenance recording — write side (PROV-01 and PROV-02).

## Tasks Completed

| Task | Name | Commit | Key Files |
| ---- | ---- | ------ | --------- |
| 1 | Migration, store model, store interface, and SQLite implementation | d5c1f79 | 00008_provenance.sql, store.go, provenance.go, provenance_test.go |
| 2 | resolveRegistryRefFull helper, provenance in handlers, touch in dispatch | 7cabf62 | handlers.go, dispatch.go, handlers_provenance_test.go |

## What Was Built

### Migration 00008

`internal/store/sqlite/migrations/00008_provenance.sql` creates the `provenance` table with:
- FK references to `runs`, `users`, `registry_entries`, `registry_versions`
- `UNIQUE (run_id)` constraint ensuring one provenance record per run
- Three indexes: `idx_provenance_run`, `idx_provenance_entry`, `idx_provenance_user`

### Store Layer

`internal/store/store.go` adds three new model types and four new interface methods:

- `Provenance` struct: full provenance record with all FK IDs
- `ProvenanceSummary`: lightweight list row (for Plan 02 list API)
- `ProvenanceDetail`: rich joined view for GET single provenance
- `Store` interface extended with `CreateProvenance`, `TouchProvenanceUpdatedAt`, `GetProvenance`, `ListProvenance`

`internal/store/sqlite/provenance.go` implements all four methods on `SQLiteStore`.

### Handler Integration

`resolveRegistryRefFull` added to `handlers.go` — returns `wdlContent, digest, entryID, versionID` for registry lookups. The existing `resolveRegistryRef` now delegates to it to avoid duplication.

`handleSubmit` and `handleTaskRun` both call `resolveRegistryRefFull`, capture `entryID` and `versionID`, and create a `Provenance` record after successful `CreateRun`. Provenance creation is non-fatal.

`dispatch.go` calls `TouchProvenanceUpdatedAt` at all terminal state transitions:
- `runWorkflow`: FAILED and COMPLETED
- `waitForTaskAndFinalize`: FAILED and COMPLETED

### Tests

Store-level unit tests in `provenance_test.go`:
- `TestProvenanceCreate`: insert and retrieve via GetProvenance
- `TestProvenanceCreateDuplicate`: UNIQUE constraint rejection
- `TestProvenanceTouchUpdatedAt`: timestamp advancement after touch
- `TestProvenanceTouchUpdatedAtMissing`: nil return for non-existent run_id

Integration tests in `handlers_provenance_test.go`:
- `TestProvenanceCreatedOnSubmit`: registry workflow submit creates provenance
- `TestProvenanceNotCreatedForFilePathRun`: file-path run creates no provenance
- `TestProvenanceCreatedOnTaskRun`: registry task run creates provenance
- `TestProvenanceUpdatedOnCompletion`: TouchProvenanceUpdatedAt advances updated_at

## Deviations from Plan

None - plan executed exactly as written.

## Verification Results

```
go build ./...                                               # exit 0
go test ./internal/store/... -run TestProvenance -v         # 4/4 PASS
go test ./internal/server/ -run TestProvenance -v           # 4/4 PASS
go test ./...                                               # all 22 packages PASS
```

Migration 00008 exists with correct schema, UNIQUE (run_id), and all three indexes.
Store interface has 4 new methods. SQLiteStore implements all 4.
handlers.go contains resolveRegistryRefFull and 2 CreateProvenance calls.
dispatch.go contains 3 TouchProvenanceUpdatedAt calls (runWorkflow FAILED, runWorkflow COMPLETED, waitForTaskAndFinalize FAILED/COMPLETED).

## Self-Check: PASSED
