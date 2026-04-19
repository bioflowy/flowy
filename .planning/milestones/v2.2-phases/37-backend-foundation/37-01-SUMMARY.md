---
phase: 37-backend-foundation
plan: 01
subsystem: store
tags: [sqlite, migration, managed-files, crud, transaction]
dependency_graph:
  requires: []
  provides: [managed_files-table, ManagedFile-model, Store-CRUD-ManagedFile, CompleteRun-tx]
  affects: [internal/store, internal/store/sqlite]
tech_stack:
  added: []
  patterns: [goose-migration, sql-NullString-nullable, BeginTx-defer-Rollback]
key_files:
  created:
    - internal/store/sqlite/migrations/00009_managed_files.sql
    - internal/store/sqlite/managed_files.go
    - internal/store/sqlite/managed_files_test.go
  modified:
    - internal/store/store.go
decisions:
  - ManagedFile.RunID is *string (nil for manually imported files without an originating run)
  - CompleteRun uses BeginTx + defer tx.Rollback() for atomicity; rollback after commit is a no-op
  - ListManagedFiles returns non-nil empty slice when no records exist (consistent with other List* methods)
metrics:
  duration_seconds: 139
  completed_date: "2026-04-09"
  tasks_completed: 1
  files_changed: 4
---

# Phase 37 Plan 01: ManagedFile Model, Store CRUD, and CompleteRun Transaction Summary

**One-liner:** SQLite migration + ManagedFile CRUD Store methods + atomic CompleteRun transaction using BeginTx/defer-Rollback pattern.

## What Was Built

Added the complete data persistence layer for managed files:

1. **Migration `00009_managed_files.sql`** — Creates `managed_files` table with `id`, `project_id` (FK NOT NULL), `run_id` (FK nullable for manually imported files), `path`, `file_type`, `label`, `size`, `created_at`. Adds indices on `project_id` and `run_id` for Phase 39 auto-registration queries.

2. **`ManagedFile` struct in `store.go`** — Model with `RunID *string` (nil for manual imports), `Size int64`, and `CreatedAt time.Time`. Appended after `ProvenanceDetail`.

3. **Store interface methods** — Five new methods added to the `Store` interface: `CreateManagedFile`, `ListManagedFiles`, `GetManagedFileByID`, `DeleteManagedFile`, and `CompleteRun`.

4. **`managed_files.go` implementation** — All five methods on `*SQLiteStore`. All INSERT/UPDATE/DELETE use `s.writer`. All SELECT use `s.reader`. All SQL parameters use `?` placeholders (no string interpolation). Uses `sql.NullString` to handle nullable `run_id` in scans.

5. **`managed_files_test.go` tests** — 9 tests covering: schema verification (table columns + indices), CRUD round-trip with nil RunID, CRUD with non-nil RunID, list empty/populated, delete, FK constraint enforcement, CompleteRun with empty files, CompleteRun with files, and CompleteRun atomicity (duplicate PK causes rollback, run stays PENDING).

## Verification Results

```
go build ./internal/store/... -- PASS (compile-time Store assertion passes)
go test ./internal/store/... -count=1 -- PASS (all existing + 9 new tests)
```

Specific new tests:
- TestManagedFileSchema -- PASS
- TestManagedFileCRUD_RoundTrip -- PASS
- TestManagedFileCRUD_WithRunID -- PASS
- TestManagedFileList -- PASS
- TestManagedFileDelete -- PASS
- TestManagedFileFK -- PASS
- TestCompleteRun_EmptyFiles -- PASS
- TestCompleteRun_WithFiles -- PASS
- TestCompleteRun_Atomicity -- PASS

## Commits

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 37-01-01 | Migration, ManagedFile model, Store interface, CRUD + CompleteRun | d9edda5 | 4 files (migration, store.go, managed_files.go, managed_files_test.go) |

## Deviations from Plan

None - plan executed exactly as written.

## Threat Model Coverage

| ID | Threat | Status |
|----|--------|--------|
| T-37-01 | SQL injection via ManagedFile field values | Mitigated — all queries use `?` placeholders; verified in managed_files.go |
| T-37-02 | FK bypass — non-existent project_id or run_id | Mitigated — SQLite `foreign_keys(ON)` in DSN; TestManagedFileFK confirms error returned |
| T-37-03 | Transaction isolation failure in CompleteRun | Mitigated — `defer tx.Rollback()` ensures atomicity; TestCompleteRun_Atomicity verifies rollback on failure |

## Known Stubs

None.

## Threat Flags

None — no new network endpoints or auth paths introduced. Pure data layer.

## Self-Check: PASSED

- [x] `internal/store/sqlite/migrations/00009_managed_files.sql` exists
- [x] `internal/store/sqlite/managed_files.go` exists
- [x] `internal/store/sqlite/managed_files_test.go` exists
- [x] `internal/store/store.go` modified
- [x] Commit d9edda5 exists
- [x] All 9 new tests pass
- [x] All existing store tests pass unchanged
