# Phase 23: Provenance Tracking - Research

**Researched:** 2026-03-20
**Domain:** Go / SQLite / chi HTTP — provenance audit trail for registry-based workflow runs
**Confidence:** HIGH

## Summary

Phase 23 adds a provenance audit table that is populated immediately after every registry-based run is submitted and updated when the run completes. The data model is a normalized+snapshot hybrid: FK references to `runs`, `users`, `registry_entries`, and `registry_versions` for efficient joins, plus snapshot TEXT columns (`registry_ref`, `registry_digest`) so the audit trail survives registry entry deletion. Inputs and outputs are NOT duplicated — they are fetched via join on `runs`.

Two new HTTP endpoints are added under the project's chi route group: a list endpoint returning summary rows, and a detail endpoint returning the full provenance record including inputs and outputs. Both require at minimum `viewer` project role, consistent with the rest of the project API. Filters `?entry=<name>` and `?user=<username>` cover the primary query patterns.

All code follows patterns already established in phases 19-22: store interface methods with a SQLiteStore implementation, goose migrations, chi middleware groups, and `api/types.go` response structs. No new dependencies are needed.

**Primary recommendation:** Implement provenance as a direct extension of the Phase 22 run-submission path, with a `CreateProvenance` call inserted immediately after `CreateRun` in both `handleSubmit` and `handleTaskRun`, and a `TouchProvenanceUpdatedAt` call in the run-completion path (both `runWorkflow` and `waitForTaskAndFinalize`).

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- Separate `provenance` table (not enriching the existing runs table) — cleaner separation, easier to extend for future per-task provenance (PROV-F01)
- Normalized + snapshot approach: FK references to `runs`, `users`, `registry_entries`, `registry_versions` for clean indexed joins, PLUS snapshot `registry_ref` and `registry_digest` as TEXT columns so provenance survives registry entry deletion
- Inputs and outputs are NOT duplicated in provenance — read from `runs` table via join on `run_id` FK
- Provenance record is 1:1 with run (one provenance per run)
- Provenance created inline in `handleSubmit` and `handleTaskRun` — immediately after `CreateRun` succeeds, call `CreateProvenance` in the same handler
- Only for registry-based runs (`registry_ref != ""`) — file-path runs do not create provenance
- Provenance updated when run completes — in the `UpdateRunOutputs`/completion path, mark provenance `updated_at` (outputs read via join, no separate write needed)
- Provenance created for ALL registry runs regardless of outcome — failed and aborted runs have provenance with no outputs, preserving the audit trail of what was attempted
- List: `GET /projects/{slug}/provenance` — returns summary list for project
- Detail: `GET /projects/{slug}/provenance/{run_id}` — returns full record with version, digest, inputs, outputs, user
- Summary list fields per item: `run_id`, `registry_ref`, `registry_digest`, `user` (username), `run_state`, `created_at` — no inputs/outputs in list
- Detail fields: all summary fields + `inputs_json`, `outputs_json`, `registry_entry_name`, `registry_entry_kind`, `updated_at`
- Basic query filters on list endpoint: `?entry=<name>` (filter by registry entry name), `?user=<username>` — enough for "all runs of tool X" and "all runs by user Y"
- No pagination for v1 — can be added later
- Viewer role or above can access provenance (consistent with registry list permission)
- File-path (non-registry) runs do NOT generate provenance records

### Claude's Discretion

- Provenance table column specifics (exact column names, index choices)
- Migration number (next after 00007_run_registry_ref.sql)
- Whether to use a single store method or separate Create/Update methods for provenance lifecycle
- Detail response JSON structure beyond the specified fields
- Filter implementation (exact query parameter names, SQL WHERE generation)
- Test structure and organization

### Deferred Ideas (OUT OF SCOPE)

- Per-task W3C PROV entity-activity-agent graph (PROV-F01) — 3-5x scope of run-level provenance; future milestone
- CWLProv-style research object export (PROV-F02) — academic publishing use case; future milestone
- Pagination on list endpoint — add when project run counts warrant it
- File-path run provenance — could be added later if audit requirements expand
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| PROV-01 | System records provenance on run submission (registry version digest, inputs, submitting user) | `CreateProvenance` called in `handleSubmit`/`handleTaskRun` after `CreateRun`; user ID from `claimsFromContext`; digest already on `run.RegistryDigest`; entry/version IDs from `resolveRegistryRef` extended return |
| PROV-02 | System updates provenance with outputs when run completes | `TouchProvenanceUpdatedAt` called from `runWorkflow` and `waitForTaskAndFinalize` in dispatch.go at completion points; outputs read via join from runs table |
| PROV-03 | Viewer can list provenance records for a project | `GET /projects/{slug}/provenance` with `s.projectRoleRequired("viewer")` guard; `ListProvenance` store method with optional entry/user filters |
| PROV-04 | Viewer can get provenance for a specific run (version, digest, inputs, outputs, user) | `GET /projects/{slug}/provenance/{run_id}` with viewer guard; `GetProvenance` store method joining provenance + runs + users + registry_entries + registry_versions |
</phase_requirements>

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| database/sql + mattn/go-sqlite3 | already in project | SQLite persistence | Established in project; all prior phases use it |
| go-chi/chi/v5 | already in project | HTTP routing with middleware | All routes use chi; project-scoped route group already exists |
| google/uuid | already in project | Generate provenance record IDs | Same pattern as runs, projects, registry entries |
| github.com/pressly/goose/v3 | already in project | SQL migrations | Migrations 00001-00007 already use goose format |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| encoding/json | stdlib | Marshal/unmarshal inputs/outputs | Used in all existing handlers |
| database/sql.NullString | stdlib | Nullable columns in scan | Used in scanRun for outputs_json, project_id |

**No new dependencies required.** Provenance tracking is pure Go + SQLite using the established project stack.

---

## Architecture Patterns

### Recommended Project Structure

New files to create:
```
internal/
├── store/
│   └── sqlite/
│       ├── provenance.go          # CreateProvenance, GetProvenance, ListProvenance, TouchProvenanceUpdatedAt
│       └── migrations/
│           └── 00008_provenance.sql  # provenance table + indexes
├── server/
│   ├── handlers_provenance.go     # handleListProvenance, handleGetProvenance
│   └── handlers_provenance_test.go
├── api/
│   └── types.go                   # add ProvenanceSummary, ProvenanceDetail structs (existing file)
└── store/
    └── store.go                   # add Provenance struct + Store interface methods (existing file)
```

### Pattern 1: Migration — Provenance Table

The `provenance` table follows the registry migration pattern from `00006_registry.sql`. Next migration is `00008_provenance.sql`.

**Recommended schema (Claude's discretion on column names/indexes):**

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS provenance (
    id                  TEXT     PRIMARY KEY,
    run_id              TEXT     NOT NULL REFERENCES runs(id),
    user_id             TEXT     NOT NULL REFERENCES users(id),
    registry_entry_id   TEXT     NOT NULL REFERENCES registry_entries(id),
    registry_version_id TEXT     NOT NULL REFERENCES registry_versions(id),
    registry_ref        TEXT     NOT NULL DEFAULT '',
    registry_digest     TEXT     NOT NULL DEFAULT '',
    created_at          DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at          DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE (run_id)
);
CREATE INDEX IF NOT EXISTS idx_provenance_run      ON provenance(run_id);
CREATE INDEX IF NOT EXISTS idx_provenance_entry    ON provenance(registry_entry_id);
CREATE INDEX IF NOT EXISTS idx_provenance_user     ON provenance(user_id);
CREATE INDEX IF NOT EXISTS idx_provenance_project  ON provenance(registry_entry_id);

-- +goose Down
DROP TABLE IF EXISTS provenance;
```

Key design choices:
- `UNIQUE (run_id)` enforces 1:1 with runs at DB level
- `NOT NULL DEFAULT ''` for snapshot strings — consistent with Phase 22 convention
- FK to `registry_entry_id` enables `?entry=<name>` filter without a join to registry_entries
- FK to `registry_version_id` enables detail response to include `registry_entry_name` and `registry_entry_kind` via join
- `updated_at` is the "completion timestamp" — bumped when run completes; no separate outputs column needed

**Note on project scoping:** Provenance is scoped to a project via the `registry_entry_id` FK chain (`registry_entries.project_id`). The list query uses `WHERE re.project_id = ?` via a join. There is no direct `project_id` column on provenance because the project is always derivable from the registry entry.

### Pattern 2: Store Model and Interface

**Provenance struct in `store.go`:**

```go
// Source: patterns from store.go RegistryEntry, RegistryVersion structs
type Provenance struct {
    ID                string
    RunID             string
    UserID            string
    RegistryEntryID   string
    RegistryVersionID string
    RegistryRef       string
    RegistryDigest    string
    CreatedAt         time.Time
    UpdatedAt         time.Time
}
```

**New Store interface methods:**

```go
// CreateProvenance creates a provenance record immediately after a registry-based run is created.
// Returns an error if a provenance record for runID already exists.
CreateProvenance(ctx context.Context, p *Provenance) error

// TouchProvenanceUpdatedAt bumps updated_at for the provenance record linked to runID.
// Called when the run reaches a terminal state (COMPLETED, FAILED, ABORTED).
// Returns nil (not an error) if no provenance record exists for the run.
TouchProvenanceUpdatedAt(ctx context.Context, runID string) error

// GetProvenance returns the provenance detail record for the given runID.
// Joins provenance + runs + users + registry_entries + registry_versions to populate all fields.
GetProvenance(ctx context.Context, projectID, runID string) (*ProvenanceDetail, error)

// ListProvenance returns summary provenance records for all runs in a project.
// entryFilter and userFilter are optional; empty string means "no filter".
ListProvenance(ctx context.Context, projectID, entryFilter, userFilter string) ([]ProvenanceSummary, error)
```

**Read models (used only by store layer):**

```go
// ProvenanceSummary is the summary row returned by ListProvenance.
type ProvenanceSummary struct {
    RunID          string
    RegistryRef    string
    RegistryDigest string
    Username       string
    RunState       string
    CreatedAt      time.Time
}

// ProvenanceDetail is the full record returned by GetProvenance.
type ProvenanceDetail struct {
    RunID              string
    RegistryRef        string
    RegistryDigest     string
    Username           string
    RunState           string
    InputsJSON         string
    OutputsJSON        string
    RegistryEntryName  string
    RegistryEntryKind  string
    CreatedAt          time.Time
    UpdatedAt          time.Time
}
```

### Pattern 3: CreateProvenance in Handlers

**In `handleSubmit` (and same pattern in `handleTaskRun`):**

```go
// Source: existing CreateRun pattern in handlers.go
run := &store.Run{ ... }
if err := s.store.CreateRun(r.Context(), run); err != nil {
    writeError(w, http.StatusInternalServerError, fmt.Sprintf("create run: %v", err))
    return
}

// Record provenance for registry-based runs only.
if req.RegistryRef != "" {
    // Resolve entry and version IDs for FK population.
    // resolveRegistryRef already called above; extend it or do a second lookup.
    entryID, versionID, lookupErr := s.resolveRegistryRefIDs(r.Context(), projectID, req.RegistryRef)
    if lookupErr != nil {
        log.Printf("handleSubmit: resolve registry IDs for provenance: %v", lookupErr)
        // Non-fatal: run is already created; provenance is best-effort.
    } else {
        claims := claimsFromContext(r.Context())
        prov := &store.Provenance{
            ID:                uuid.New().String(),
            RunID:             runID,
            UserID:            claims.UserID,
            RegistryEntryID:   entryID,
            RegistryVersionID: versionID,
            RegistryRef:       req.RegistryRef,
            RegistryDigest:    registryDigest,
        }
        if provErr := s.store.CreateProvenance(r.Context(), prov); provErr != nil {
            log.Printf("handleSubmit: CreateProvenance for run %s: %v", runID, provErr)
            // Non-fatal: audit failure should not block run execution.
        }
    }
}
```

**Important:** `resolveRegistryRef` currently returns `(wdlContent, digest, error)` but does not return entry/version IDs. A new helper `resolveRegistryRefIDs` (or an extended return from `resolveRegistryRef`) is needed to get `entryID` and `versionID` without a second DB round-trip. The SQLite queries `GetRegistryVersionByTag` and `GetRegistryVersionByDigest` already return both `*RegistryVersion` (which has `EntryID`) and `*RegistryEntry` (which has `ID`), so the IDs are available — they just need to be plumbed through.

**Option A (recommended):** Add a private `resolveRegistryRefWithIDs(ctx, projectID, ref) (wdlContent, digest, entryID, versionID, error)` helper that replaces the existing `resolveRegistryRef` calls internally.

**Option B:** Call `GetRegistryEntry` + `GetRegistryVersionByDigest` after `CreateRun` for a second lookup. Slightly less efficient but simpler.

### Pattern 4: TouchProvenanceUpdatedAt in dispatch.go

There are two completion paths that need the touch:

1. `runWorkflow` in `dispatch.go` — around lines 287-288 where `UpdateRunState COMPLETED` is called, and around line 260 where `UpdateRunState FAILED` is called.
2. `waitForTaskAndFinalize` in `dispatch.go` — around lines 306, 319, 334 where terminal states are set.

Because `TouchProvenanceUpdatedAt` is non-fatal (provenance records may not exist for older runs or file-path runs), the pattern is:

```go
// Source: same "non-fatal log" pattern as UpdateRunOutputs in dispatch.go
if err := s.store.TouchProvenanceUpdatedAt(context.Background(), runID); err != nil {
    log.Printf("runWorkflow %s: TouchProvenanceUpdatedAt: %v", runID, err)
}
```

### Pattern 5: API Response Types in `api/types.go`

```go
// Source: modeled after RegistryEntryResponse, MemberResponse in api/types.go

// ProvenanceSummaryResponse is a single item in the provenance list response.
type ProvenanceSummaryResponse struct {
    RunID          string `json:"run_id"`
    RegistryRef    string `json:"registry_ref"`
    RegistryDigest string `json:"registry_digest"`
    Username       string `json:"username"`
    RunState       string `json:"run_state"`
    CreatedAt      string `json:"created_at"`
}

// ProvenanceDetailResponse is the full provenance record for a single run.
type ProvenanceDetailResponse struct {
    RunID             string                 `json:"run_id"`
    RegistryRef       string                 `json:"registry_ref"`
    RegistryDigest    string                 `json:"registry_digest"`
    Username          string                 `json:"username"`
    RunState          string                 `json:"run_state"`
    Inputs            map[string]interface{} `json:"inputs,omitempty"`
    Outputs           map[string]interface{} `json:"outputs,omitempty"`
    RegistryEntryName string                 `json:"registry_entry_name"`
    RegistryEntryKind string                 `json:"registry_entry_kind"`
    CreatedAt         string                 `json:"created_at"`
    UpdatedAt         string                 `json:"updated_at"`
}
```

**Note on Inputs/Outputs:** The detail response unmarshals `runs.inputs_json` and `runs.outputs_json` into `map[string]interface{}` before writing the response. Use `omitempty` so empty maps/nil are omitted rather than serialized as `null`. When outputs are not yet available (run still running), `outputs` is absent from the response.

### Pattern 6: Route Registration in `server.go`

Following the existing project route pattern:

```go
// Source: server.go lines 147-157 — registry sub-resource pattern
// Add inside r.Route("/{slug}", ...) after the registry block:

// Provenance sub-resource (audit trail for registry-based runs).
r.Route("/provenance", func(r chi.Router) {
    r.With(s.projectRoleRequired("viewer")).Get("/", s.handleListProvenance)
    r.With(s.projectRoleRequired("viewer")).Get("/{run_id}", s.handleGetProvenance)
})
```

### Pattern 7: Handler Implementation

```go
// Source: modeled after handleRegistryList in handlers_registry.go

func (s *Server) handleListProvenance(w http.ResponseWriter, r *http.Request) {
    proj := projectFromContext(r.Context())
    if proj == nil {
        writeError(w, http.StatusNotFound, "project not found")
        return
    }
    entryFilter := r.URL.Query().Get("entry")
    userFilter := r.URL.Query().Get("user")

    items, err := s.store.ListProvenance(r.Context(), proj.ID, entryFilter, userFilter)
    if err != nil {
        writeError(w, http.StatusInternalServerError, fmt.Sprintf("list provenance: %v", err))
        return
    }
    // Build response slice...
    writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetProvenance(w http.ResponseWriter, r *http.Request) {
    runID := chi.URLParam(r, "run_id")
    proj := projectFromContext(r.Context())
    detail, err := s.store.GetProvenance(r.Context(), proj.ID, runID)
    if err != nil {
        writeError(w, http.StatusNotFound, fmt.Sprintf("provenance not found: %s", runID))
        return
    }
    // Unmarshal inputs/outputs and build ProvenanceDetailResponse...
    writeJSON(w, http.StatusOK, resp)
}
```

### Pattern 8: SQLite Implementation

**CreateProvenance** — use `s.writer` (same as CreateRun):

```go
// Source: CreateRun in runs.go
func (s *SQLiteStore) CreateProvenance(ctx context.Context, p *store.Provenance) error {
    now := time.Now().UTC().Format(time.RFC3339)
    _, err := s.writer.ExecContext(ctx,
        `INSERT INTO provenance
         (id, run_id, user_id, registry_entry_id, registry_version_id,
          registry_ref, registry_digest, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        p.ID, p.RunID, p.UserID, p.RegistryEntryID, p.RegistryVersionID,
        p.RegistryRef, p.RegistryDigest, now, now,
    )
    if err != nil {
        return fmt.Errorf("sqlite: CreateProvenance: %w", err)
    }
    return nil
}
```

**TouchProvenanceUpdatedAt** — use `s.writer`; return nil if no rows affected (not an error):

```go
func (s *SQLiteStore) TouchProvenanceUpdatedAt(ctx context.Context, runID string) error {
    _, err := s.writer.ExecContext(ctx,
        `UPDATE provenance SET updated_at = datetime('now') WHERE run_id = ?`, runID)
    if err != nil {
        return fmt.Errorf("sqlite: TouchProvenanceUpdatedAt: %w", err)
    }
    return nil
}
```

**ListProvenance** — join to get username and run state; filter via WHERE clauses:

```go
// Source: ListRegistryEntries pattern — reader connection, conditional WHERE
func (s *SQLiteStore) ListProvenance(ctx context.Context, projectID, entryFilter, userFilter string) ([]store.ProvenanceSummary, error) {
    // Build query with optional filters
    q := `SELECT p.run_id, p.registry_ref, p.registry_digest,
                 u.username, r.state, p.created_at
          FROM provenance p
          JOIN runs r ON r.id = p.run_id
          JOIN users u ON u.id = p.user_id
          JOIN registry_entries re ON re.id = p.registry_entry_id
          WHERE re.project_id = ?`
    args := []interface{}{projectID}
    if entryFilter != "" {
        q += ` AND re.name = ?`
        args = append(args, entryFilter)
    }
    if userFilter != "" {
        q += ` AND u.username = ?`
        args = append(args, userFilter)
    }
    q += ` ORDER BY p.created_at DESC`
    // ...scan rows...
}
```

**GetProvenance** — full join including registry_versions for entry kind:

```go
// Source: scanVersionWithEntry pattern for multi-table joins
func (s *SQLiteStore) GetProvenance(ctx context.Context, projectID, runID string) (*store.ProvenanceDetail, error) {
    row := s.reader.QueryRowContext(ctx,
        `SELECT p.run_id, p.registry_ref, p.registry_digest,
                u.username, r.state, r.inputs_json, r.outputs_json,
                re.name, re.kind, p.created_at, p.updated_at
         FROM provenance p
         JOIN runs r       ON r.id  = p.run_id
         JOIN users u      ON u.id  = p.user_id
         JOIN registry_entries re   ON re.id = p.registry_entry_id
         WHERE re.project_id = ? AND p.run_id = ?`,
        projectID, runID,
    )
    // ...scan and return...
}
```

### Anti-Patterns to Avoid

- **Duplicating inputs/outputs in provenance:** Wastes storage and creates consistency issues. Read them from `runs` via join on `run_id` — this is a locked decision.
- **Creating provenance before CreateRun succeeds:** If CreateRun fails, there is no run record to link to. Always create provenance AFTER CreateRun returns nil.
- **Making provenance creation block the HTTP response:** If CreateProvenance fails, log the error but still return 201 and start the run. Audit trail failure must not block workflow execution.
- **Omitting provenance for failed/aborted runs:** Provenance captures what was ATTEMPTED. Even runs that never complete need a record.
- **Forgetting to scope list query to project:** List query MUST join to `registry_entries` and filter by `re.project_id = ?` to prevent cross-project data leaks.
- **Missing index on registry_entry_id:** Without this index, `?entry=<name>` filter will scan the full table. Add `idx_provenance_entry` in the migration.
- **Adding provenance routes in the auth-disabled branch:** Provenance requires project context (JWT required), so routes only belong in the auth-enabled project route group. Auth-disabled deployments do not get provenance endpoints.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Content-type headers and JSON encoding | Custom response writer | `writeJSON` / `writeError` already in server package | Existing helpers handle headers and encoding consistently |
| Auth and role enforcement | Custom middleware | `s.projectRoleRequired("viewer")` chi middleware | Already handles global admin bypass, membership check, role comparison |
| Project context | Parse slug from URL | `projectFromContext(r.Context())` | `projectContext` middleware already injects it |
| User claims | Parse JWT manually | `claimsFromContext(r.Context())` returns `*auth.Claims` with `UserID`, `Username`, `Role` | JWT middleware already validated and injected claims |
| UUID generation | Custom ID scheme | `uuid.New().String()` | Used consistently for all record IDs |
| Time formatting | Custom time.Time serialization | `time.Now().UTC().Format(time.RFC3339)` for writes; `parseTime` (existing helper) for reads | Consistent with all existing migrations and store implementations |

---

## Common Pitfalls

### Pitfall 1: resolveRegistryRef Does Not Return Entry/Version IDs

**What goes wrong:** `resolveRegistryRef` returns `(wdlContent, digest, error)` — it does NOT return `entryID` or `versionID`. The handler needs these IDs to populate the provenance FK columns. If the caller does a second lookup by digest after `CreateRun`, there's a small window where `resolveRegistryRef` succeeded but the second lookup fails (e.g., entry deleted between calls — unlikely but possible).

**How to avoid:** Extend `resolveRegistryRef` to return `(wdlContent, digest, entryID, versionID, error)`, or add a companion `resolveRegistryRefIDs` helper. Both `GetRegistryVersionByTag` and `GetRegistryVersionByDigest` already return the `*RegistryEntry` alongside the version, so the IDs are readily available — they just need plumbing.

**Warning signs:** If provenance creation compiles but the handler calls a separate store method to look up entry/version IDs, there are two round-trips where one would suffice.

### Pitfall 2: Provenance Query Without Project Scope

**What goes wrong:** If `ListProvenance` omits a project scope filter, users with viewer role on project A could see provenance for project B.

**How to avoid:** Always join `provenance → registry_entries` and filter `WHERE re.project_id = ?`. Verified by checking that `GetProvenance` also passes `projectID` and the query enforces it. The project ID comes from `projectFromContext(r.Context()).ID`.

**Warning signs:** Any query on the `provenance` table without a join to `registry_entries` is missing project scoping.

### Pitfall 3: Missing FK IDs Break Provenance Insert

**What goes wrong:** `CreateProvenance` requires `registry_entry_id` and `registry_version_id` as non-nullable FKs. If these are populated with empty strings, the INSERT will violate the NOT NULL constraint.

**How to avoid:** The handler must resolve entry/version IDs before calling `CreateProvenance`. If resolution fails (non-fatal path), log the error and skip provenance creation rather than inserting empty strings.

**Warning signs:** Any INSERT into provenance with empty string for FK columns.

### Pitfall 4: Completion Touch in Wrong Goroutine

**What goes wrong:** The run completion happens in background goroutines (`runWorkflow`, `waitForTaskAndFinalize`) that use `context.Background()` rather than `r.Context()`. If `TouchProvenanceUpdatedAt` is called from the HTTP handler (which exits immediately), it misses the completion event.

**How to avoid:** Add `TouchProvenanceUpdatedAt` calls inside `dispatch.go` at the same points where `UpdateRunState` is called with terminal states. Both run completion paths are in `dispatch.go` lines 229-334.

**Warning signs:** `TouchProvenanceUpdatedAt` only in handler code, not in dispatch goroutines.

### Pitfall 5: Auth-Disabled Servers Get No Provenance Routes

**What goes wrong:** The provenance routes require project context (JWT must have been validated), so they only exist in the auth-enabled code path. If someone adds provenance handlers in the `else` branch of `newRouter`, the provenance queries will fail because `projectFromContext` returns nil.

**How to avoid:** Provenance routes are only registered inside the `r.Route("/projects", ...)` block inside `if s.cfg.Auth.Enabled { ... }`. No routes in the auth-disabled else branch.

---

## Code Examples

Verified patterns from existing source code:

### Injecting User Context (from middleware.go)

```go
// Source: internal/server/middleware.go:163
func claimsFromContext(ctx context.Context) *auth.Claims {
    v := ctx.Value(auth.UserClaimsKey)
    if v == nil {
        return nil
    }
    c, _ := v.(*auth.Claims)
    return c
}
// Claims has: UserID string, Username string, Role string
```

### Project Context Injection (from middleware.go + handlers.go)

```go
// Source: internal/server/handlers.go:107-116
func projectIDFromContext(ctx context.Context) string {
    proj := projectFromContext(ctx)
    if proj == nil {
        return ""
    }
    return proj.ID
}
// projectFromContext returns *store.Project injected by projectContext middleware
```

### GetRegistryVersionByTag / GetRegistryVersionByDigest Return Signature

```go
// Source: internal/store/store.go:270-273
GetRegistryVersionByTag(ctx context.Context, projectID, name, tag string) (*RegistryVersion, *RegistryEntry, error)
GetRegistryVersionByDigest(ctx context.Context, projectID, name, digest string) (*RegistryVersion, *RegistryEntry, error)
// RegistryVersion.ID = versionID; RegistryVersion.EntryID = entryID
// RegistryEntry.ID = entryID; RegistryEntry.Name = entry name; RegistryEntry.Kind = "tool"/"workflow"
```

### Pattern for Non-Fatal Store Operation (from dispatch.go)

```go
// Source: internal/server/dispatch.go:274-276
if err := s.store.UpdateRunOutputs(context.Background(), runID, string(data)); err != nil {
    log.Printf("runWorkflow %s: UpdateRunOutputs: %v", runID, err)
}
// Non-fatal: operation logged but execution continues
```

### Conditional SQL Filter Construction (from registry.go ListRegistryEntries)

```go
// Source: internal/store/sqlite/registry.go:140-148
rows, err := s.reader.QueryContext(ctx,
    `SELECT re.name, re.kind, re.created_at, ...
     FROM registry_entries re
     WHERE re.project_id = ? AND re.is_active = 1
     ORDER BY re.created_at ASC`,
    projectID,
)
```

### NOT NULL DEFAULT '' Convention for String Columns (from 00007 migration)

```sql
-- Source: internal/store/sqlite/migrations/00007_run_registry_ref.sql
ALTER TABLE runs ADD COLUMN registry_ref TEXT NOT NULL DEFAULT '';
ALTER TABLE runs ADD COLUMN registry_digest TEXT NOT NULL DEFAULT '';
```

### goose Migration Format

```sql
-- Source: internal/store/sqlite/migrations/00006_registry.sql
-- +goose Up
CREATE TABLE IF NOT EXISTS ...;
CREATE INDEX IF NOT EXISTS ...;

-- +goose Down
DROP TABLE IF EXISTS ...;
```

### Test Fixture Pattern (from handlers_registry_exec_test.go)

```go
// Source: internal/server/handlers_registry_exec_test.go:34-41
func newRegistryExecFixture(t *testing.T) (*server.Server, *sqlite.SQLiteStore, func(), string, string, string) {
    t.Helper()
    srv, st, cleanup := newAuthTestServer(t)
    createTestUser(t, st, "exec-user-1", "alice", "pass", "admin")
    token := issueTestToken(t, "exec-user-1", "alice", "admin")
    proj := createRegistryTestProject(t, st, "exec-user-1")
    return srv, st, cleanup, "exec-user-1", token, proj.Slug
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| No provenance tracking | Provenance table linked to runs | Phase 23 (now) | Audit trail for registry-based reproducibility |
| resolveRegistryRef returns (content, digest) | Will need (content, digest, entryID, versionID) | Phase 23 | Internal helper extension for FK population |
| dispatch.go calls only UpdateRunState/UpdateRunOutputs at completion | Also calls TouchProvenanceUpdatedAt | Phase 23 | Provenance updated_at reflects true completion time |

---

## Open Questions

1. **Should provenance creation failure block run creation?**
   - What we know: The locked decision says provenance is created inline after `CreateRun`. The CONTEXT does not specify whether a provenance failure should roll back or be non-fatal.
   - What's unclear: If `CreateProvenance` fails (e.g., FK constraint because entry was deleted between resolution and insert), should the run be aborted or continue?
   - Recommendation: Non-fatal — log the error but return 201 and start the run. Audit trail failure should not block workflow execution. This matches the `UpdateRunOutputs` pattern in dispatch.go.

2. **How to handle provenance for runs submitted before Phase 23 is deployed?**
   - What we know: Old runs will have no provenance records. `GetProvenance` on an old run_id will return 404.
   - What's unclear: Should 404 be the correct response or should we return a "no provenance" indicator?
   - Recommendation: Return 404 with a clear error message `"provenance not found for run: <id>"`. This is correct behavior — provenance did not exist before Phase 23.

3. **Is `resolveRegistryRefWithIDs` a new helper or an extension of `resolveRegistryRef`?**
   - What we know: Both `GetRegistryVersionByTag` and `GetRegistryVersionByDigest` already return `*RegistryVersion` and `*RegistryEntry` with all needed IDs.
   - Recommendation: Add a new private method `resolveRegistryRefFull` that returns `(wdlContent, digest, entryID, versionID, error)`. This avoids changing the signature of the existing `resolveRegistryRef` which is called in `handleClaim` for re-resolution (where IDs are not needed).

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) — no external test framework |
| Config file | none — `go test ./...` discovers tests automatically |
| Quick run command | `go test ./internal/server/ -run TestProvenance -v` |
| Full suite command | `go test ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PROV-01 | After submitting a registry run, provenance record exists with digest, run_id, user | integration | `go test ./internal/server/ -run TestProvenanceCreatedOnSubmit -v` | Wave 0 |
| PROV-01 | File-path runs do NOT create provenance | integration | `go test ./internal/server/ -run TestProvenanceNotCreatedForFilePathRun -v` | Wave 0 |
| PROV-01 | handleTaskRun with registry_ref also creates provenance | integration | `go test ./internal/server/ -run TestProvenanceCreatedOnTaskRun -v` | Wave 0 |
| PROV-02 | After run completes, provenance updated_at is bumped | integration | `go test ./internal/server/ -run TestProvenanceUpdatedOnCompletion -v` | Wave 0 |
| PROV-02 | Failed/aborted runs have provenance with no outputs | integration | `go test ./internal/server/ -run TestProvenanceUpdatedOnFailure -v` | Wave 0 |
| PROV-03 | GET /projects/{slug}/provenance returns list for project | integration | `go test ./internal/server/ -run TestHandleListProvenance -v` | Wave 0 |
| PROV-03 | ?entry filter returns only matching runs | integration | `go test ./internal/server/ -run TestHandleListProvenanceEntryFilter -v` | Wave 0 |
| PROV-03 | ?user filter returns only matching runs | integration | `go test ./internal/server/ -run TestHandleListProvenanceUserFilter -v` | Wave 0 |
| PROV-03 | Viewer cannot see other project's provenance | integration | `go test ./internal/server/ -run TestHandleListProvenanceProjectScope -v` | Wave 0 |
| PROV-04 | GET /projects/{slug}/provenance/{run_id} returns full detail | integration | `go test ./internal/server/ -run TestHandleGetProvenance -v` | Wave 0 |
| PROV-04 | Detail includes inputs_json, outputs_json, entry_name, entry_kind | integration | `go test ./internal/server/ -run TestHandleGetProvenanceFields -v` | Wave 0 |
| PROV-04 | GET for non-existent run_id returns 404 | integration | `go test ./internal/server/ -run TestHandleGetProvenanceNotFound -v` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/server/ -run TestProvenance -v`
- **Per wave merge:** `go test ./...`
- **Phase gate:** `go test ./...` green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/server/handlers_provenance_test.go` — all PROV-01 through PROV-04 integration tests
- [ ] `internal/store/sqlite/provenance.go` — store implementation (Wave 0 creates stub file)
- [ ] `internal/store/sqlite/migrations/00008_provenance.sql` — migration

*(All test infrastructure exists: `newAuthTestServer`, `issueTestToken`, `createTestUser`, `createRegistryTestProject`, `doAuthRequest`, `pushRegistryEntry` helpers are in place and reusable.)*

---

## Sources

### Primary (HIGH confidence)
- `/home/uehara/flowy/internal/server/handlers.go` — handleSubmit, handleTaskRun, resolveRegistryRef implementation
- `/home/uehara/flowy/internal/store/store.go` — Store interface and all model structs
- `/home/uehara/flowy/internal/store/sqlite/runs.go` — CreateRun, scanRun patterns
- `/home/uehara/flowy/internal/store/sqlite/registry.go` — GetRegistryVersionByTag/ByDigest, join patterns
- `/home/uehara/flowy/internal/store/sqlite/migrations/00006_registry.sql` — registry table schema conventions
- `/home/uehara/flowy/internal/store/sqlite/migrations/00007_run_registry_ref.sql` — NOT NULL DEFAULT '' convention
- `/home/uehara/flowy/internal/server/server.go` — route registration patterns
- `/home/uehara/flowy/internal/server/middleware.go` — claimsFromContext, projectFromContext, projectRoleRequired
- `/home/uehara/flowy/internal/server/dispatch.go` — run completion paths (UpdateRunState, UpdateRunOutputs call sites)
- `/home/uehara/flowy/internal/auth/auth.go` — Claims struct (UserID, Username, Role)
- `/home/uehara/flowy/internal/api/types.go` — response struct conventions
- `/home/uehara/flowy/internal/server/handlers_registry_exec_test.go` — test fixture patterns

### Secondary (MEDIUM confidence)
- `23-CONTEXT.md` — user decisions (locked + discretion areas confirmed against code)

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all dependencies already in project; no new packages needed
- Architecture: HIGH — patterns directly from inspected source files; no extrapolation
- Pitfalls: HIGH — derived from actual code paths and FK constraints in the schema
- SQL queries: HIGH — modeled directly on existing ListRegistryEntries and scanVersionWithEntry patterns

**Research date:** 2026-03-20
**Valid until:** 2026-04-20 (stable Go + SQLite stack; internal patterns do not drift)
