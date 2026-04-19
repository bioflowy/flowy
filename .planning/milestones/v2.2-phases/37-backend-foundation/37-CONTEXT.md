# Phase 37: Backend Foundation - Context

**Gathered:** 2026-04-09
**Status:** Ready for planning

<domain>
## Phase Boundary

Lay the data layer and storage interfaces needed to persist and measure managed files. No HTTP endpoints, no UI changes. All subsequent phases (38, 39, 40) build on this foundation.

Deliverables:
- SQLite migration `00009_managed_files.sql` creating the `managed_files` table
- `ManagedFile` model type in `internal/store`
- CRUD methods on the `Store` interface for managed files
- `CompleteRun` method on `Store` that wraps state transition + output storage + file registration in one SQLite transaction
- `Stat(path string) (int64, error)` on the `Backend` interface, implemented by both `LocalBackend` and `S3Backend`
- `HeadObject` added to the `S3APIClient` interface and satisfied by the real AWS client

</domain>

<decisions>
## Implementation Decisions

### managed_files Table Schema
- **D-01:** Migration `00009_managed_files.sql` creates the full schema for v2.2 — no follow-up ALTER migrations needed in Phase 38 or 39.
- **D-02:** Columns: `id TEXT PRIMARY KEY`, `project_id TEXT NOT NULL REFERENCES projects(id)`, `run_id TEXT REFERENCES runs(id)` (nullable — null for manually imported files), `path TEXT NOT NULL`, `file_type TEXT NOT NULL DEFAULT ''`, `label TEXT NOT NULL DEFAULT ''`, `size INTEGER NOT NULL DEFAULT 0`, `created_at DATETIME NOT NULL DEFAULT (datetime('now'))`.
- **D-03:** Add indices on `project_id` and `run_id` (nullable FK index is useful for Phase 39 AUTOREG queries).
- **D-04:** Follow the existing migration pattern: `-- +goose Up` / `-- +goose Down`, `CREATE TABLE IF NOT EXISTS`, `DROP TABLE IF EXISTS`.

### ManagedFile Model
- **D-05:** Define `ManagedFile` struct in `internal/store/store.go` alongside other model types. Fields mirror the DB columns: `ID`, `ProjectID`, `RunID *string` (pointer for nullable), `Path`, `FileType`, `Label`, `Size int64`, `CreatedAt time.Time`.

### Store Interface — CRUD Methods
- **D-06:** Add to the `Store` interface:
  - `CreateManagedFile(ctx context.Context, f *ManagedFile) error`
  - `ListManagedFiles(ctx context.Context, projectID string) ([]ManagedFile, error)`
  - `GetManagedFileByID(ctx context.Context, id string) (*ManagedFile, error)`
  - `DeleteManagedFile(ctx context.Context, id string) error`
- **D-07:** Callers set `f.ID` (UUID) before calling `CreateManagedFile`, consistent with all other Create methods in the interface.

### Store Interface — CompleteRun
- **D-08:** Signature: `CompleteRun(ctx context.Context, runID string, projectID string, outputsJSON string, files []ManagedFile) error`
- **D-09:** Implementation wraps three operations in a single SQLite transaction: UPDATE runs SET state='COMPLETED', outputs_json=? WHERE id=?; then INSERT each ManagedFile in files (if len > 0); then the transaction commits.
- **D-10:** Callers pass `nil` or empty slice for `files` when no file registration is needed. Phase 39 wires up the non-empty case.
- **D-11:** Phase 37's SQLite implementation of `CompleteRun` must call it from the existing dispatch path that currently calls `UpdateRunState` + `UpdateRunOutputs` separately. Wire the replacement call in `CompleteRun`'s own implementation; the dispatch code in `server/` can be updated in Phase 39 (or leave the existing separate calls as-is until Phase 39 replaces them).

### Backend.Stat
- **D-12:** Add `Stat(path string) (int64, error)` to the `Backend` interface. Consistent with existing Backend methods — no `context.Context` in the signature.
- **D-13:** `LocalBackend.Stat`: use `os.Stat(path).Size()`. Return `(0, err)` on failure.
- **D-14:** `S3Backend.Stat`: call `HeadObject(context.Background(), ...)` internally, return `*ContentLength`. Return `(0, err)` on failure (including key-not-found).

### S3APIClient.HeadObject
- **D-15:** Add `HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)` to the `S3APIClient` interface. The real `*s3.Client` already satisfies this — no wrapper needed.
- **D-16:** Update any mock `S3APIClient` implementations in test files to add a stub `HeadObject` method (can return nil, nil by default).

### Existing Tests
- **D-17:** All existing tests must pass unchanged. No behavioral changes to existing methods — this phase is additive only.

### Claude's Discretion
- Index naming convention for the new table (follow existing `idx_{table}_{column}` pattern)
- Whether to add a `UNIQUE (project_id, path)` constraint on `managed_files` — acceptable if it aids data integrity, but not required by success criteria

</decisions>

<specifics>
## Specific Ideas

- The SQLite `CompleteRun` implementation should use `database/sql` `Tx` directly (same as other multi-step operations in `internal/store/sqlite/`). Check `runs.go` for the transaction pattern.
- `run_id` is nullable to support manually imported files (Phase 38) that have no originating run. Use `*string` in Go model; `NULL` in SQLite.
- `label` defaults to empty string in the DB; Phase 38's import handler will set it to the filename at insert time.

</specifics>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

- `internal/store/store.go` — Store interface and all model types; add ManagedFile here
- `internal/store/sqlite/runs.go` — SQLite transaction patterns; reference for CompleteRun impl
- `internal/store/sqlite/sqlite.go` — DB setup and migration embed
- `internal/store/sqlite/migrations/00008_provenance.sql` — Latest migration; use as schema pattern
- `internal/storage/backend.go` — Backend interface; add Stat here
- `internal/storage/s3.go` — S3APIClient interface and S3Backend; add HeadObject and Stat impl
- `.planning/REQUIREMENTS.md` — FILE-01 through PICKER-02 requirements for v2.2
- `.planning/ROADMAP.md` Phase 37 success criteria — authoritative checklist for this phase

</canonical_refs>

<deferred>
## Deferred Ideas

None surfaced during discussion.

</deferred>
