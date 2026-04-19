# Phase 37: Backend Foundation - Research

**Researched:** 2026-04-09
**Domain:** Go — SQLite migrations (goose), store.Store interface extension, database/sql transactions, Backend interface extension, AWS SDK v2 S3 HeadObject
**Confidence:** HIGH

## Summary

Phase 37 is a purely additive, non-breaking change to two packages: `internal/store` and `internal/storage`. All decisions are fully locked in CONTEXT.md. The codebase is well-structured with consistent patterns that make each deliverable straightforward to implement.

The three work streams are independent and can be sequenced in any order within a wave:
1. SQLite migration + `ManagedFile` model + CRUD Store methods
2. `CompleteRun` transactional method on the Store interface and SQLiteStore
3. `Backend.Stat` + `S3APIClient.HeadObject` interface additions

The existing test infrastructure (`sqlite_test.go` with `openTestStore`, `s3_test.go` with `mockS3Client`) provides the exact patterns needed for new tests. The compile-time assertion `var _ store.Store = (*SQLiteStore)(nil)` at line 33 of `sqlite.go` will fail at build time if any new interface method is not implemented — this is the primary safety net.

**Primary recommendation:** Implement all three streams in a single wave. The compile-time assertion makes omissions immediately visible; additive-only changes to existing methods mean no existing test can break.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**managed_files Table Schema**
- D-01: Migration `00009_managed_files.sql` creates the full schema for v2.2 — no follow-up ALTER migrations needed in Phase 38 or 39.
- D-02: Columns: `id TEXT PRIMARY KEY`, `project_id TEXT NOT NULL REFERENCES projects(id)`, `run_id TEXT REFERENCES runs(id)` (nullable), `path TEXT NOT NULL`, `file_type TEXT NOT NULL DEFAULT ''`, `label TEXT NOT NULL DEFAULT ''`, `size INTEGER NOT NULL DEFAULT 0`, `created_at DATETIME NOT NULL DEFAULT (datetime('now'))`.
- D-03: Add indices on `project_id` and `run_id`.
- D-04: Follow existing migration pattern: `-- +goose Up` / `-- +goose Down`, `CREATE TABLE IF NOT EXISTS`, `DROP TABLE IF EXISTS`.

**ManagedFile Model**
- D-05: Define `ManagedFile` struct in `internal/store/store.go`. Fields: `ID`, `ProjectID`, `RunID *string`, `Path`, `FileType`, `Label`, `Size int64`, `CreatedAt time.Time`.

**Store Interface — CRUD Methods**
- D-06: Add `CreateManagedFile`, `ListManagedFiles`, `GetManagedFileByID`, `DeleteManagedFile` to the `Store` interface.
- D-07: Callers set `f.ID` (UUID) before calling `CreateManagedFile`.

**Store Interface — CompleteRun**
- D-08: Signature: `CompleteRun(ctx context.Context, runID string, projectID string, outputsJSON string, files []ManagedFile) error`
- D-09: Single SQLite transaction: UPDATE runs state='COMPLETED' + outputs_json; INSERT each ManagedFile; COMMIT.
- D-10: Callers pass `nil` or empty slice for `files` when no file registration needed.
- D-11: Phase 37's `CompleteRun` implementation is standalone; dispatch code in `server/` is updated in Phase 39.

**Backend.Stat**
- D-12: `Stat(path string) (int64, error)` — no context.Context, consistent with existing Backend methods.
- D-13: `LocalBackend.Stat`: `os.Stat(path).Size()`. Return `(0, err)` on failure.
- D-14: `S3Backend.Stat`: call `HeadObject(context.Background(), ...)`, return `*ContentLength`. Return `(0, err)` on failure.

**S3APIClient.HeadObject**
- D-15: Add `HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)` to `S3APIClient`. Real `*s3.Client` already satisfies it.
- D-16: Update mock `S3APIClient` in `s3_test.go` to add stub `HeadObject` method (can return nil, nil by default).

**Existing Tests**
- D-17: All existing tests must pass unchanged. This phase is additive only.

### Claude's Discretion

- Index naming convention for the new table (follow existing `idx_{table}_{column}` pattern)
- Whether to add a `UNIQUE (project_id, path)` constraint on `managed_files`

### Deferred Ideas (OUT OF SCOPE)

None surfaced during discussion.
</user_constraints>

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `database/sql` | stdlib | SQLite transactions via `*sql.Tx` | Already used throughout `internal/store/sqlite/` |
| `modernc.org/sqlite` | v1.46.1 | Pure-Go SQLite driver registered as "sqlite" | Already in go.mod, used by SQLiteStore |
| `github.com/pressly/goose/v3` | v3.27.0 | Embedded SQL migrations, auto-applied on Open | Already in go.mod, wired in `sqlite.go` |
| `github.com/aws/aws-sdk-go-v2/service/s3` | v1.97.0 | `HeadObjectInput`, `HeadObjectOutput`, `HeadObject` method | Already in go.mod, used by S3Backend |

[VERIFIED: go.mod lines 5-20 in project]

All dependencies are already present in go.mod — no new packages to install.

**Installation:** No new dependencies required.

## Architecture Patterns

### Recommended Project Structure

New files to create:
```
internal/store/sqlite/managed_files.go     # CRUD methods on SQLiteStore
internal/store/sqlite/managed_files_test.go  # Tests for CRUD + CompleteRun
internal/store/sqlite/migrations/00009_managed_files.sql  # Goose migration
```

Files to modify:
```
internal/store/store.go                    # Add ManagedFile struct + Store interface methods
internal/store/sqlite/runs.go              # Add CompleteRun method (or managed_files.go)
internal/storage/backend.go               # Add Stat to Backend interface
internal/storage/local.go                 # Implement LocalBackend.Stat
internal/storage/s3.go                    # Add HeadObject to S3APIClient, implement S3Backend.Stat
internal/storage/s3_test.go               # Add HeadObject stub to mockS3Client
```

### Pattern 1: Goose Migration Format

**What:** SQL migrations with `-- +goose Up` / `-- +goose Down` markers, embedded via `//go:embed migrations/*.sql` in sqlite.go.

**When to use:** Any schema change. Run automatically by `goose.Up(writer, "migrations")` in `open()`.

**Example (from `00008_provenance.sql`):**
```sql
-- Source: internal/store/sqlite/migrations/00008_provenance.sql
-- +goose Up
CREATE TABLE IF NOT EXISTS provenance (
    id                  TEXT     PRIMARY KEY,
    run_id              TEXT     NOT NULL REFERENCES runs(id),
    ...
    created_at          DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at          DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE (run_id)
);
CREATE INDEX IF NOT EXISTS idx_provenance_run   ON provenance(run_id);
CREATE INDEX IF NOT EXISTS idx_provenance_entry ON provenance(registry_entry_id);

-- +goose Down
DROP TABLE IF EXISTS provenance;
```

The new migration follows the same pattern:
```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS managed_files (
    id          TEXT     PRIMARY KEY,
    project_id  TEXT     NOT NULL REFERENCES projects(id),
    run_id      TEXT     REFERENCES runs(id),
    path        TEXT     NOT NULL,
    file_type   TEXT     NOT NULL DEFAULT '',
    label       TEXT     NOT NULL DEFAULT '',
    size        INTEGER  NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_managed_files_project ON managed_files(project_id);
CREATE INDEX IF NOT EXISTS idx_managed_files_run     ON managed_files(run_id);

-- +goose Down
DROP TABLE IF EXISTS managed_files;
```

[VERIFIED: pattern from 00008_provenance.sql, D-02 schema from CONTEXT.md]

### Pattern 2: Store Interface + SQLiteStore Implementation

**What:** Define methods on `store.Store` interface in `store.go`; implement on `*SQLiteStore` in a new `managed_files.go` file. The compile-time assertion in `sqlite.go` (`var _ store.Store = (*SQLiteStore)(nil)`) will fail at build time if any method is missing.

**Example — CreateManagedFile (modeled after CreateProvenance):**
```go
// Source: pattern from internal/store/sqlite/provenance.go
func (s *SQLiteStore) CreateManagedFile(ctx context.Context, f *store.ManagedFile) error {
    _, err := s.writer.ExecContext(ctx,
        `INSERT INTO managed_files (id, project_id, run_id, path, file_type, label, size)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
        f.ID, f.ProjectID, f.RunID, f.Path, f.FileType, f.Label, f.Size,
    )
    if err != nil {
        return fmt.Errorf("sqlite: CreateManagedFile: %w", err)
    }
    return nil
}
```

Note on nullable `run_id`: `f.RunID` is `*string`. SQLite driver handles `nil` pointer as NULL correctly with `database/sql`.

### Pattern 3: SQLite Transaction via database/sql Tx

**What:** Use `s.writer.BeginTx(ctx, nil)` to acquire a `*sql.Tx`, execute statements, then `tx.Commit()`. Use `defer tx.Rollback()` for automatic rollback on early return (Rollback after Commit is a no-op in `database/sql`).

**When to use:** `CompleteRun` — wraps UPDATE runs + INSERT managed_files in one atomic operation.

**Example pattern (adapted from existing project code):**
```go
// Source: database/sql standard pattern; confirmed present in registry.go via writer connection
func (s *SQLiteStore) CompleteRun(ctx context.Context, runID, projectID, outputsJSON string, files []store.ManagedFile) error {
    tx, err := s.writer.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("sqlite: CompleteRun begin: %w", err)
    }
    defer tx.Rollback()

    _, err = tx.ExecContext(ctx,
        `UPDATE runs SET state = 'COMPLETED', outputs_json = ?, updated_at = datetime('now') WHERE id = ?`,
        outputsJSON, runID,
    )
    if err != nil {
        return fmt.Errorf("sqlite: CompleteRun update run: %w", err)
    }

    for _, f := range files {
        _, err = tx.ExecContext(ctx,
            `INSERT INTO managed_files (id, project_id, run_id, path, file_type, label, size)
             VALUES (?, ?, ?, ?, ?, ?, ?)`,
            f.ID, f.ProjectID, f.RunID, f.Path, f.FileType, f.Label, f.Size,
        )
        if err != nil {
            return fmt.Errorf("sqlite: CompleteRun insert managed_file %q: %w", f.ID, err)
        }
    }

    if err := tx.Commit(); err != nil {
        return fmt.Errorf("sqlite: CompleteRun commit: %w", err)
    }
    return nil
}
```

[VERIFIED: `database/sql` transaction API; `s.writer` is `*sql.DB`; BeginTx confirmed as standard pattern]

**Important:** `PushRegistryVersion` in `registry.go` uses sequential writes on `s.writer` without an explicit transaction. `CompleteRun` requires an explicit `Tx` because the UPDATE + INSERT(s) must be atomic. Use `s.writer.BeginTx`.

### Pattern 4: Backend Interface Addition + LocalBackend Stat

**What:** Add `Stat(path string) (int64, error)` to the `Backend` interface in `backend.go`. Implement on `LocalBackend` using `os.Stat`. No context — consistent with all other Backend methods (confirmed by reading `backend.go` lines 1-49).

**Example:**
```go
// In backend.go — add to Backend interface
Stat(path string) (int64, error)

// In local.go — implement
func (l *LocalBackend) Stat(path string) (int64, error) {
    fi, err := os.Stat(path)
    if err != nil {
        return 0, fmt.Errorf("storage: stat %q: %w", path, err)
    }
    return fi.Size(), nil
}
```

[VERIFIED: `backend.go` interface has no context.Context; `os.Stat` returns `FileInfo` with `Size() int64`]

### Pattern 5: S3APIClient Interface Extension + S3Backend.Stat

**What:** Add `HeadObject` to `S3APIClient` interface; add stub to `mockS3Client`; implement `S3Backend.Stat` calling `HeadObject`. `HeadObjectOutput.ContentLength` is `*int64`.

**Example:**
```go
// In s3.go — add to S3APIClient interface
HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)

// In s3.go — implement S3Backend.Stat
func (b *S3Backend) Stat(path string) (int64, error) {
    out, err := b.s3c.HeadObject(context.Background(), &s3.HeadObjectInput{
        Bucket: aws.String(b.bucket),
        Key:    aws.String(path),
    })
    if err != nil {
        return 0, fmt.Errorf("s3: stat %s: %w", path, err)
    }
    if out.ContentLength == nil {
        return 0, nil
    }
    return *out.ContentLength, nil
}

// In s3_test.go — add to mockS3Client struct and add method
headObjectFn func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)

func (m *mockS3Client) HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
    if m.headObjectFn != nil {
        return m.headObjectFn(ctx, params, optFns...)
    }
    return nil, nil // default stub: no-op
}
```

[VERIFIED: `go doc github.com/aws/aws-sdk-go-v2/service/s3 Client.HeadObject` — signature confirmed. `HeadObjectOutput.ContentLength` is `*int64` — confirmed via `go doc`]

### Pattern 6: scanManagedFile with Nullable run_id

**What:** When scanning `managed_files` rows, `run_id` is nullable. Use `sql.NullString` to scan it, then assign to `*string`.

**Example:**
```go
func scanManagedFile(runID sql.NullString, ...) {
    var f store.ManagedFile
    var runIDNull sql.NullString
    // ... scan into runIDNull ...
    if runIDNull.Valid {
        f.RunID = &runIDNull.String
    }
    return &f, nil
}
```

[VERIFIED: same pattern used for `project_id` in `scanRun`/`scanRunRows` in runs.go lines 127-140]

### Anti-Patterns to Avoid

- **Using `s.reader` for writes:** All INSERT/UPDATE/DELETE must use `s.writer` (single-connection serialization). CRUD methods on managed_files must use `s.writer`.
- **Using `s.reader` inside a transaction:** Once you call `s.writer.BeginTx`, all statements in the transaction must use `tx.ExecContext` / `tx.QueryRowContext`, not `s.writer` or `s.reader` directly.
- **Forgetting `defer tx.Rollback()`:** Without it, a failed commit leaks the transaction. Rollback after Commit is a no-op.
- **Omitting `created_at` from INSERT:** The DB column has `DEFAULT (datetime('now'))` — omitting it from the INSERT is fine and idiomatic for this codebase (see `CreateTask` which omits `created_at`).
- **Adding context.Context to Backend.Stat:** Decision D-12 explicitly excludes it to match existing Backend signature convention.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Transaction rollback safety | Manual error/cleanup logic | `defer tx.Rollback()` (no-op after Commit) | Idiomatic Go; prevents leaks on all exit paths |
| Nullable FK scanning | Custom nil checks | `sql.NullString` | Standard `database/sql` pattern already used in `scanRun` |
| Migration ordering | Custom version tracking | goose sequence numbers (00009 > 00008) | Already wired in `sqlite.go` via `goose.Up` |
| S3 file size | Custom GetObject + len(body) | `HeadObject` + `ContentLength` | HeadObject does not download body — correct and efficient |

## Common Pitfalls

### Pitfall 1: Compile-time assertion catches missing interface methods
**What goes wrong:** Adding methods to `store.Store` without implementing them on `SQLiteStore` causes a compile error at `var _ store.Store = (*SQLiteStore)(nil)` in `sqlite.go:33`.
**Why it happens:** Interface satisfaction is checked at compile time.
**How to avoid:** Implement all interface methods before running `go build` or `go test`. The compile-time check is a feature — it will surface the gap immediately.
**Warning signs:** Build error referencing `sqlite.go:33`.

### Pitfall 2: mockS3Client not satisfying S3APIClient after HeadObject addition
**What goes wrong:** Adding `HeadObject` to the `S3APIClient` interface without adding the stub method to `mockS3Client` in `s3_test.go` breaks all existing S3 tests.
**Why it happens:** `mockS3Client` is used as `S3APIClient` throughout `s3_test.go`. Adding a method to the interface requires all implementations — including mocks — to satisfy it.
**How to avoid:** Add stub `HeadObject` to `mockS3Client` (field + method) in the same commit that adds it to the interface.
**Warning signs:** Compile error in `s3_test.go`: `mockS3Client does not implement S3APIClient`.

### Pitfall 3: ContentLength nil dereference on HeadObject response
**What goes wrong:** `HeadObjectOutput.ContentLength` is `*int64`. Dereferencing without nil check panics.
**Why it happens:** AWS SDK v2 uses pointer fields for optional values.
**How to avoid:** Always nil-check before dereferencing: `if out.ContentLength == nil { return 0, nil }`.
**Warning signs:** Panic in `S3Backend.Stat` during tests.

### Pitfall 4: Transaction on reader pool instead of writer
**What goes wrong:** Calling `s.reader.BeginTx` instead of `s.writer.BeginTx` for `CompleteRun` creates a read-only transaction that will error on UPDATE/INSERT.
**Why it happens:** The store has two pools; writes must go through `s.writer`.
**How to avoid:** Always use `s.writer.BeginTx` for any transaction containing writes.
**Warning signs:** `attempt to write a readonly database` error.

### Pitfall 5: Missing newProject helper in managed_files test
**What goes wrong:** Tests for `CreateManagedFile` require a valid `project_id` FK. Inserting without a matching project violates the FK constraint (`PRAGMA foreign_keys(ON)`).
**Why it happens:** The DB is opened with `foreign_keys(ON)`.
**How to avoid:** Use the existing `newProject` helper from `sqlite_test.go` (defined in `projects_test.go` or `sqlite_test.go`) to create a project first. For `run_id` FK, create a run first if testing non-null run_id.
**Warning signs:** `FOREIGN KEY constraint failed` error in tests.

## Code Examples

### managed_files Migration

```sql
-- Source: CONTEXT.md D-02, D-04; pattern from 00008_provenance.sql
-- +goose Up
CREATE TABLE IF NOT EXISTS managed_files (
    id          TEXT     PRIMARY KEY,
    project_id  TEXT     NOT NULL REFERENCES projects(id),
    run_id      TEXT     REFERENCES runs(id),
    path        TEXT     NOT NULL,
    file_type   TEXT     NOT NULL DEFAULT '',
    label       TEXT     NOT NULL DEFAULT '',
    size        INTEGER  NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_managed_files_project ON managed_files(project_id);
CREATE INDEX IF NOT EXISTS idx_managed_files_run     ON managed_files(run_id);

-- +goose Down
DROP TABLE IF EXISTS managed_files;
```

### ManagedFile Struct

```go
// Source: CONTEXT.md D-05; location: internal/store/store.go
// ManagedFile represents a file tracked in the project library.
// RunID is nil for manually imported files that have no originating run.
type ManagedFile struct {
    ID        string
    ProjectID string
    RunID     *string // nil for manually imported files
    Path      string
    FileType  string
    Label     string
    Size      int64
    CreatedAt time.Time
}
```

### Store Interface Methods

```go
// Source: CONTEXT.md D-06, D-08; location: internal/store/store.go

// ManagedFile operations

// CreateManagedFile persists a new managed file. f.ID must be set by the caller.
CreateManagedFile(ctx context.Context, f *ManagedFile) error
// ListManagedFiles returns all managed files for the given project.
ListManagedFiles(ctx context.Context, projectID string) ([]ManagedFile, error)
// GetManagedFileByID returns the managed file with the given ID.
GetManagedFileByID(ctx context.Context, id string) (*ManagedFile, error)
// DeleteManagedFile removes the managed file record with the given ID.
DeleteManagedFile(ctx context.Context, id string) error
// CompleteRun atomically marks the run as COMPLETED with the given outputs and
// inserts any provided managed file records. Pass nil or empty slice for files
// when no file registration is needed.
CompleteRun(ctx context.Context, runID string, projectID string, outputsJSON string, files []ManagedFile) error
```

### Test Helper Pattern (from sqlite_test.go)

```go
// Source: internal/store/sqlite/sqlite_test.go openTestStore + newProject pattern
func TestCreateManagedFile_Basic(t *testing.T) {
    s := openTestStore(t)
    ctx := context.Background()

    p := newProject("test-project")
    if err := s.CreateProject(ctx, p); err != nil {
        t.Fatalf("CreateProject: %v", err)
    }

    f := &store.ManagedFile{
        ID:        "mf-0001",
        ProjectID: p.ID,
        RunID:     nil, // manually imported
        Path:      "/data/sample.fastq",
        FileType:  "fastq",
        Label:     "sample.fastq",
        Size:      1024,
    }
    if err := s.CreateManagedFile(ctx, f); err != nil {
        t.Fatalf("CreateManagedFile: %v", err)
    }
    // verify via GetManagedFileByID...
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Separate `UpdateRunOutputs` + `UpdateRunState` calls | `CompleteRun` wrapping both in one TX | Phase 37 (this phase) | Output + state transition are now atomic — no partial state |

**No deprecated items:** This is greenfield interface extension.

## Assumptions Log

> All claims in this research were verified against the codebase or official Go tooling. No unverified assumptions.

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| — | — | — | — |

**All claims verified:** No `[ASSUMED]` tags in this research.

## Open Questions (RESOLVED)

1. **UNIQUE (project_id, path) constraint**
   - What we know: CONTEXT.md marks this as Claude's Discretion
   - What's unclear: Whether Phase 38 import handler will ever insert duplicate (project_id, path) combinations intentionally
   - Recommendation: Omit the UNIQUE constraint from Phase 37's migration. Phase 38 can add it via a follow-up migration if the import handler enforces uniqueness. This avoids a constraint that might complicate Phase 38 testing.

2. **CompleteRun placement (runs.go vs managed_files.go)**
   - What we know: CONTEXT.md says "check runs.go for the transaction pattern"; the method touches both the `runs` table and `managed_files` table
   - Recommendation: Implement in `managed_files.go` since that is the primary new file. The method is tightly coupled to managed_files insertion logic.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All builds and tests | Yes | 1.25.0 | — |
| goose (embedded) | Migration execution | Yes (via go module) | v3.27.0 | — |
| modernc.org/sqlite (embedded) | SQLiteStore | Yes (via go module) | v1.46.1 | — |
| aws-sdk-go-v2/service/s3 | HeadObject | Yes (via go module) | v1.97.0 | — |

**No missing dependencies.** All required packages are already in go.mod.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go standard `testing` package |
| Config file | none (no go test flags needed) |
| Quick run command | `go test ./internal/store/... ./internal/storage/... -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements → Test Map

This phase has no user-facing requirement IDs (it is foundational). The success criteria map to the following tests:

| Success Criterion | Behavior | Test Type | Automated Command | File Exists? |
|-------------------|----------|-----------|-------------------|-------------|
| SC-1: `managed_files` table exists after migration | Schema check | unit | `go test ./internal/store/sqlite/... -run TestManagedFilesSchema -count=1` | No — Wave 0 |
| SC-2a: CRUD Store methods on ManagedFile | Round-trip create/list/get/delete | unit | `go test ./internal/store/sqlite/... -run TestManagedFile -count=1` | No — Wave 0 |
| SC-2b: CompleteRun wraps state + files in one TX | Atomic: if file insert fails, run state not updated | unit | `go test ./internal/store/sqlite/... -run TestCompleteRun -count=1` | No — Wave 0 |
| SC-3: LocalBackend.Stat returns file size | Known-size file | unit | `go test ./internal/storage/... -run TestLocalBackend_Stat -count=1` | No — Wave 0 |
| SC-4: S3Backend.Stat calls HeadObject | Mock HeadObject returns ContentLength | unit | `go test ./internal/storage/... -run TestS3Backend_Stat -count=1` | No — Wave 0 |
| SC-5: All existing tests pass | Regression | regression | `go test ./internal/store/... ./internal/storage/... -count=1` | Yes |

### Sampling Rate
- **Per task commit:** `go test ./internal/store/... ./internal/storage/... -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps

- `internal/store/sqlite/managed_files_test.go` — covers SC-1, SC-2a, SC-2b
- `internal/storage/local_test.go` additions — covers SC-3 (`TestLocalBackend_Stat`)
- `internal/storage/s3_test.go` additions — covers SC-4 (`TestS3Backend_Stat`, `headObjectFn` stub in `mockS3Client`)

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | n/a — no auth changes |
| V3 Session Management | No | n/a |
| V4 Access Control | No | n/a — no HTTP endpoints |
| V5 Input Validation | Partial | path and ID strings are stored as-is; no user input in Phase 37 |
| V6 Cryptography | No | n/a |

Phase 37 adds no HTTP endpoints, no user input handling, and no authentication changes. The security surface is limited to:
- SQL injection prevention: all queries use parameterized placeholders (`?`) — consistent with existing codebase [VERIFIED: all ExecContext calls in sqlite package use `?` parameters]
- Foreign key constraints enforced at DB level (`PRAGMA foreign_keys(ON)` in DSN)

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| SQL injection via ManagedFile fields | Tampering | Parameterized queries (`?`) throughout — already the project standard |

## Sources

### Primary (HIGH confidence)

- Project codebase — `internal/store/store.go`, `internal/store/sqlite/runs.go`, `internal/store/sqlite/sqlite.go`, `internal/store/sqlite/migrations/00008_provenance.sql`, `internal/storage/backend.go`, `internal/storage/s3.go`, `internal/storage/s3_test.go`, `internal/storage/local.go` — read directly
- Go stdlib `database/sql` — BeginTx, Tx.ExecContext, Tx.Commit, Tx.Rollback — training knowledge corroborated by usage in project
- AWS SDK v2 HeadObject signature — verified via `go doc github.com/aws/aws-sdk-go-v2/service/s3 Client.HeadObject` and `go doc ... HeadObjectOutput`

### Secondary (MEDIUM confidence)

- CONTEXT.md decisions D-01 through D-17 — treated as authoritative locked decisions

### Tertiary (LOW confidence)

None.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all dependencies verified in go.mod, APIs confirmed via go doc
- Architecture: HIGH — patterns directly read from existing codebase
- Pitfalls: HIGH — derived from reading actual code (compile-time assertion, foreign key pragma, pointer fields)

**Research date:** 2026-04-09
**Valid until:** 2026-05-09 (stable Go/SQLite stack)
