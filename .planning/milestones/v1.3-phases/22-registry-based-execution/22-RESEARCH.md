# Phase 22: Registry-Based Execution - Research

**Researched:** 2026-03-20
**Domain:** Go HTTP handlers, SQLite migrations, CLI argument parsing, WDL execution pipeline
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Reference input format:**
- New `registry_ref` field on both `SubmitRequest` and `TaskRunRequest` (not overloading `wdl_path`)
- Exactly one of `wdl_path` or `registry_ref` must be set — mutually exclusive, 400 if both provided
- Reference format: `name:tag` or `name@sha256:hex` — short form, always resolves within current project context
- Both POST /workflows (handleSubmit) and POST /tasks/run (handleTaskRun) accept `registry_ref`

**Resolution flow:**
- Early resolve in handler: check `registry_ref` first, call store.GetRegistryVersionByTag/Digest to fetch WDL content, then pass to `parser.Parse(name, []byte(content))` instead of reading from disk
- No middleware — resolution is a few lines at the top of each handler before the existing parse call
- Both file-path and registry-ref modes always work regardless of auth mode — registry is additive, not replacing

**Run record storage:**
- For registry runs: `WDLPath` is empty, `RegistryRef` stores the user-provided reference (e.g. `align-reads:prod`)
- `RegistryDigest` also stored — the resolved `sha256:hex` at submission time, for reproducibility and Phase 23 provenance
- For file-path runs: `WDLPath` is set, `RegistryRef` and `RegistryDigest` are empty
- Requires migration to add `registry_ref` and `registry_digest` columns to runs table

**CLI integration:**
- `flowy run` and `flowy submit` auto-detect registry refs from positional argument
- Detection rule: if arg contains `:` (not as Windows drive letter) or `@sha256:`, treat as registry ref; otherwise treat as file path
- `flowy run align-reads:prod -i reads=sample.bam` works identically to `flowy run hello.wdl -i reads=sample.bam`
- Project context: add `default_project` to flowy config; `--project` flag overrides; required when using registry refs
- Input handling identical for both modes — same `-i` flag, same `--inputs` JSON file support

**Error semantics:**
- Registry ref that doesn't resolve: 422 Unprocessable Entity
- Specific error messages: `entry 'foo' not found` vs `tag 'prod' not found for entry 'foo'`
- Both fields provided: 400 `exactly one of wdl_path or registry_ref must be set, not both`
- Neither field provided: 400 `one of wdl_path or registry_ref is required`
- Registry ref on non-project endpoint (auth disabled): 400 `registry_ref requires a project-scoped endpoint`

### Claude's Discretion
- Exact ref parsing implementation (splitting on `:` vs `@sha256:`)
- How to structure the resolution helper function (shared between handleSubmit and handleTaskRun)
- Migration number for adding columns to runs table
- Config file format for `default_project` setting
- Test structure — integration tests vs unit tests for resolution logic

### Deferred Ideas (OUT OF SCOPE)
- Cross-project registry references (`project/name:tag`) — future when cross-project sharing (REG-F01) is implemented
- Registry-only mode (force all runs through registry when auth enabled) — could be a config option in a future phase
- Import resolution for registry WDLs (WDL files in registry that import other WDL files) — requires import resolver changes, separate phase
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| EXEC-01 | Executor can submit a run by registry reference (`project/name:tag` or `project/name@sha256:hex`) | CLI auto-detection, `registry_ref` field on SubmitRequest/TaskRunRequest, handler resolution logic |
| EXEC-02 | Server resolves registry ref to WDL content and delivers via existing `WDLContent` path | `GetRegistryVersionByTag`/`GetRegistryVersionByDigest` already return WDL content; `parser.Parse(name, []byte(content))` already accepts in-memory; `WDLContent` in TaskClaimResponse already populated |
</phase_requirements>

## Summary

Phase 22 connects the WDL registry (Phase 21) to the execution pipeline (existing handlers). The work is entirely additive — the two submission handlers gain a branch at their top, two API structs gain a field each, the `Run` model gains two new nullable fields, and the CLI gains auto-detection logic. No existing code is removed or replaced.

The registry store layer from Phase 21 already provides exactly the methods needed: `GetRegistryVersionByTag` and `GetRegistryVersionByDigest` return a `*RegistryVersion` containing the `WDLContent` string. The parser already accepts in-memory content via `parser.Parse(name, []byte(content))`. The `handleClaim` handler already reads `run.WDLPath` and reads the file from disk to populate `WDLContent` in the TaskClaimResponse — for registry runs, `WDLPath` will be empty, so the claim handler needs to also check `run.RegistryRef` and read `WDLContent` from the stored content or re-resolve from the registry.

The most nuanced implementation detail is the claim path: when a daemon claims a task, `handleClaim` must populate `WDLContent` for the daemon. For registry runs, it cannot read from disk. The clean solution is to store the resolved WDL content on the Run record itself (as `WDLContent`), or to re-resolve from the registry at claim time. The CONTEXT.md decisions imply storing `RegistryDigest` on the run, which enables re-resolution from registry. However, reading from `store.GetRegistryVersionByDigest` at claim time is also valid since the digest is stored.

**Primary recommendation:** Add `registry_ref`, `registry_digest`, and optionally `wdl_content` to the `Run` struct and runs table. Store resolved WDL content in `wdl_content` on the Run record so `handleClaim` never needs to touch the registry at claim time — this is cleaner and avoids a second lookup. However, since `WDLContent` is already populated by the daemon claim path for file-path runs (by reading the file), for registry runs the symmetrical approach is to resolve once at submission time and cache in the DB.

## Standard Stack

### Core (all existing — no new dependencies)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `internal/store` | project-local | Run/Task/Registry model and Store interface | Existing persistence layer |
| `internal/store/sqlite` | project-local | SQLite implementations of Store | Already used throughout |
| `internal/api` | project-local | HTTP DTO types | Already used throughout |
| `internal/server` | project-local | HTTP handlers | Target of modification |
| `internal/config` | project-local | YAML config with `gopkg.in/yaml.v3` | Already used |
| `internal/client` | project-local | HTTP client used by CLI | Target of modification |
| `github.com/pressly/goose/v3` | existing | SQL migrations | Already used for 00001-00006 |

No new external dependencies are required for this phase.

## Architecture Patterns

### Recommended Change Set
```
internal/
├── api/
│   └── types.go              # Add RegistryRef string to SubmitRequest and TaskRunRequest
├── store/
│   └── store.go              # Add RegistryRef, RegistryDigest fields to Run struct
├── store/sqlite/
│   ├── migrations/
│   │   └── 00007_run_registry_ref.sql  # ADD COLUMN registry_ref, registry_digest to runs
│   └── runs.go               # Update CreateRun INSERT, scanRun/scanRunRows SELECT+SCAN
├── server/
│   └── handlers.go           # handleSubmit and handleTaskRun: add registry_ref branch
├── config/
│   └── config.go             # Add DefaultProject string to ClientConfig
└── cmd/flowy/
    └── main.go               # isRegistryRef() helper; runRun and runSubmit use it
```

### Pattern 1: Handler Registry Resolution Branch

The resolution branch belongs at the TOP of each handler, immediately after decoding the request body and before any existing validation.

```go
// Source: inferred from handlers_registry.go pattern (handleRegistryPull)

// resolveRegistryRef resolves a registry reference ("name:tag" or "name@sha256:digest")
// to WDL content. projectID must be non-empty. Returns (content, resolvedDigest, error).
func (s *Server) resolveRegistryRef(ctx context.Context, projectID, ref string) (string, string, error) {
    name, tag, digest, err := parseRegistryRef(ref)
    if err != nil {
        return "", "", err
    }
    if tag != "" {
        v, _, err := s.store.GetRegistryVersionByTag(ctx, projectID, name, tag)
        if err != nil {
            return "", "", fmt.Errorf("tag %q not found for entry %q", tag, name)
        }
        return v.WDLContent, v.Digest, nil
    }
    v, _, err := s.store.GetRegistryVersionByDigest(ctx, projectID, name, digest)
    if err != nil {
        return "", "", fmt.Errorf("digest %q not found for entry %q", digest, name)
    }
    return v.WDLContent, v.Digest, nil
}

// parseRegistryRef splits "name:tag" or "name@sha256:hex" into components.
func parseRegistryRef(ref string) (name, tag, digest string, err error) {
    if idx := strings.Index(ref, "@sha256:"); idx >= 0 {
        return ref[:idx], "", ref[idx+1:], nil // digest = "sha256:hex"
    }
    if idx := strings.LastIndex(ref, ":"); idx >= 0 {
        return ref[:idx], ref[idx+1:], "", nil
    }
    return "", "", "", fmt.Errorf("invalid registry ref %q: must be name:tag or name@sha256:hex", ref)
}
```

**Modified handleSubmit validation block:**
```go
// Source: handlers.go handleSubmit — new validation replaces existing:
//   if req.WDLPath == "" { writeError(400, "wdl_path is required") }

// Validate mutual exclusion.
if req.WDLPath != "" && req.RegistryRef != "" {
    writeError(w, http.StatusBadRequest, "exactly one of wdl_path or registry_ref must be set, not both")
    return
}
if req.WDLPath == "" && req.RegistryRef == "" {
    writeError(w, http.StatusBadRequest, "one of wdl_path or registry_ref is required")
    return
}

// Resolve registry ref if provided.
var wdlContent string
var registryDigest string
if req.RegistryRef != "" {
    projectID := projectIDFromContext(r.Context())
    if projectID == "" {
        writeError(w, http.StatusBadRequest, "registry_ref requires a project-scoped endpoint")
        return
    }
    var resolveErr error
    wdlContent, registryDigest, resolveErr = s.resolveRegistryRef(r.Context(), projectID, req.RegistryRef)
    if resolveErr != nil {
        writeError(w, http.StatusUnprocessableEntity, resolveErr.Error())
        return
    }
}

// Parse — file path or in-memory content.
var doc *ast.Document
var err error
if wdlContent != "" {
    doc, err = parser.Parse(req.RegistryRef, []byte(wdlContent))
} else if len(req.SearchPaths) > 0 {
    doc, err = parser.ParseWithSearchPaths(req.WDLPath, nil, req.SearchPaths)
} else {
    doc, err = parser.Parse(req.WDLPath, nil)
}
```

### Pattern 2: Run Record with Registry Fields

```go
// Source: store/store.go Run struct — add two fields

type Run struct {
    ID             string
    ProjectID      string
    WDLPath        string    // Empty for registry runs
    RegistryRef    string    // User-provided ref, e.g. "align-reads:prod"; empty for file-path runs
    RegistryDigest string    // Resolved digest at submission time; empty for file-path runs
    InputsJSON     string
    State          RunState
    OutputsJSON    string
    CreatedAt      time.Time
    UpdatedAt      time.Time
}
```

### Pattern 3: Migration Adding Nullable Columns

Following the established migration pattern (goose format):

```sql
-- Source: migrations/00005_projects.sql pattern
-- 00007_run_registry_ref.sql

-- +goose Up
ALTER TABLE runs ADD COLUMN registry_ref    TEXT NOT NULL DEFAULT '';
ALTER TABLE runs ADD COLUMN registry_digest TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQLite does not support DROP COLUMN in all versions; recreate table.
CREATE TABLE runs_old AS
    SELECT id, wdl_path, inputs_json, state, outputs_json, created_at, updated_at, project_id
    FROM runs;
DROP TABLE runs;
ALTER TABLE runs_old RENAME TO runs;
```

Note: Using `NOT NULL DEFAULT ''` avoids nullable handling in scan functions and keeps the existing `sql.NullString` pattern unnecessary for these fields. This matches the decisions (empty string signals "not set").

### Pattern 4: CLI Registry Ref Auto-Detection

```go
// Source: cmd/flowy/main.go — new helper function

// isRegistryRef reports whether arg looks like a registry reference.
// A registry ref contains ":" not preceded by a drive letter (Windows compat)
// or contains "@sha256:".
func isRegistryRef(arg string) bool {
    if strings.Contains(arg, "@sha256:") {
        return true
    }
    // ":" present and not a Windows drive letter (e.g., "C:")
    idx := strings.Index(arg, ":")
    if idx < 0 {
        return false
    }
    // Drive letters are single alphabetical characters.
    return idx != 1 || !isLetter(arg[0])
}

func isLetter(b byte) bool {
    return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
```

**Modified runRun — submission branch:**
```go
// Replace: absWDL, err := filepath.Abs(wdlArg)
// With:
var absWDL string
var registryRef string
if isRegistryRef(wdlArg) {
    registryRef = wdlArg
} else {
    absWDL, err = filepath.Abs(wdlArg)
    if err != nil {
        fmt.Fprintf(stderr, "error: resolving WDL path %q: %v\n", wdlArg, err)
        return 1
    }
}
// Then set req.WDLPath = absWDL (may be "") or req.RegistryRef = registryRef
```

### Pattern 5: handleClaim Must Populate WDLContent for Registry Runs

Current `handleClaim` (line 388-395) reads `run.WDLPath` from disk:
```go
if wdlPath != "" {
    wdlBytes, readErr := os.ReadFile(wdlPath)
    ...
    wdlContent = string(wdlBytes)
}
```

For registry runs, `WDLPath` is empty. The daemon still needs `WDLContent`. Two options:

1. **Re-resolve from registry at claim time**: call `resolveRegistryRef` using `run.RegistryRef` and `run.RegistryDigest` — requires the store to be consulted again.
2. **Cache WDL content on the Run record**: add a `WDLContent` column to the runs table (populated at submission time for registry runs) — eliminates the extra lookup.

**Recommendation:** Use option 1 (re-resolve from registry at claim time). Caching WDL content in the runs table duplicates large TEXT data that's already in `registry_versions`. The `run.RegistryRef` (split into name and tag/digest) plus `run.ProjectID` is sufficient to re-resolve. Alternatively, since `RegistryDigest` is stored, call `GetRegistryVersionByDigest` — no ambiguity, always resolves to the exact version submitted.

```go
// In handleClaim, after wdlPath assignment:
if wdlPath != "" {
    // File-path run: read from disk as before.
    wdlBytes, readErr := os.ReadFile(wdlPath)
    ...
} else if run.RegistryRef != "" {
    // Registry run: re-resolve by digest for reproducibility.
    name, _, _, _ := parseRegistryRef(run.RegistryRef)
    v, _, resolveErr := s.store.GetRegistryVersionByDigest(r.Context(), run.ProjectID, name, run.RegistryDigest)
    if resolveErr != nil {
        log.Printf("handleClaim: re-resolve registry ref %q: %v", run.RegistryRef, resolveErr)
    } else {
        wdlContent = v.WDLContent
    }
}
```

### Pattern 6: Config DefaultProject Field

```go
// Source: config/config.go ClientConfig

type ClientConfig struct {
    ServerURL      string `yaml:"server_url"`
    DefaultProject string `yaml:"default_project"`  // ADD: slug of the default project for registry refs
}
```

The CLI reads `cfg.Client.DefaultProject` and uses it when a `--project` flag is not provided. This is already the established pattern (`resolveServerURL` uses `cfg.Client.ServerURL` as fallback).

### Anti-Patterns to Avoid

- **Overloading `wdl_path` with registry refs**: explicitly rejected. A new `registry_ref` field keeps semantics clear.
- **Middleware for resolution**: explicitly rejected. Resolution is a few lines in each handler.
- **Cross-project refs in this phase**: deferred. Current ref format is always project-scoped.
- **Caching WDL content on the runs table**: prefer re-resolution from registry using stored digest. Avoids duplicating large blobs.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Digest comparison | Custom string comparison logic | Direct string equality (`v.Digest == run.RegistryDigest`) | Digest is already normalized as `sha256:<hex>` by `computeDigest` |
| In-memory WDL parse | Custom parse invocation | `parser.Parse(name, []byte(content))` | Already used in `classifyWDL` in `handlers_registry.go` |
| SHA-256 computation | Custom hash | `computeDigest()` in `handlers_registry.go` | Already exists, same format |
| Registry tag/digest lookup | Custom SQL queries | `store.GetRegistryVersionByTag` / `store.GetRegistryVersionByDigest` | Already implemented in Phase 21 |

## Common Pitfalls

### Pitfall 1: `handleClaim` Leaves Daemon Without WDL Content for Registry Runs
**What goes wrong:** `handleClaim` reads `run.WDLPath` from disk for `WDLContent`. For registry runs, `WDLPath` is empty, so `wdlContent` stays empty. The daemon receives an empty `WDLContent` and cannot parse the WDL to execute the task.
**Why it happens:** The file-path and registry paths are symmetric at submission time but diverge at claim time. The existing code only handles file-path runs.
**How to avoid:** Add an explicit branch in `handleClaim` for the `run.RegistryRef != ""` case, re-resolving by digest from the registry store.
**Warning signs:** Integration test where a task is claimed after a registry-ref submission — check that `TaskClaimResponse.WDLContent` is non-empty.

### Pitfall 2: `scanRun` Scan Column Count Mismatch After Migration
**What goes wrong:** After adding `registry_ref` and `registry_digest` columns to the runs table, `scanRun` and `scanRunRows` still scan only 8 columns (id, wdl_path, inputs_json, state, outputs_json, created_at, updated_at, project_id). The SELECT queries must be updated to include the new columns, and the `Scan()` calls must include corresponding destination variables.
**Why it happens:** SQLite `SELECT *` is used nowhere — the query strings are explicit. But if the SELECT is not updated, the new columns are silently omitted.
**How to avoid:** Update both the SELECT string and the `Scan()` call in `scanRun` and `scanRunRows` together. Write a test that reads back a registry run and asserts `RegistryRef` and `RegistryDigest` are populated.

### Pitfall 3: CLI Detection of Windows Drive Letters
**What goes wrong:** On Windows, `C:\path\to\file.wdl` contains `:` and would incorrectly be detected as a registry ref.
**Why it happens:** Naive `strings.Contains(arg, ":")` check.
**How to avoid:** The `isRegistryRef` helper must exclude single-letter + colon prefixes (`C:`, `D:`, etc.). The CONTEXT.md decision already calls this out.

### Pitfall 4: `wdl_path NOT NULL` Constraint in Existing Schema
**What goes wrong:** The original `runs` table was created with `wdl_path TEXT NOT NULL` (migration 00001). For registry runs, `WDLPath` is empty — inserting an empty string satisfies `NOT NULL` but is semantically different from NULL. The `CreateRun` INSERT already passes `r.WDLPath` directly.
**Why it happens:** The constraint was created before registry runs existed.
**How to avoid:** Empty string (`""`) satisfies `NOT NULL`, so no schema change is needed for this column. The `Run.WDLPath` field will simply be empty for registry runs. The application layer distinguishes via `RegistryRef != ""`.

### Pitfall 5: `--project` Flag Must Be Added to `reorderArgs`
**What goes wrong:** The `reorderArgs` function in `main.go` hardcodes known value-carrying flags. Adding `--project` without adding it to `knownValueFlags` means `--project myproj align-reads:prod` reorders incorrectly — `myproj` gets treated as a positional arg.
**Why it happens:** The flag reordering map is maintained manually.
**How to avoid:** Add `"--project": true, "-project": true` to `knownValueFlags` when adding the `--project` flag.

### Pitfall 6: `GetRegistryVersionByTag` Signature Requires projectID + name + tag
**What goes wrong:** The store methods take three separate parameters: `projectID`, `name`, and `tag`. The registry ref `align-reads:prod` encodes `name=align-reads`, `tag=prod`. The project ID comes from context. These must be parsed and threaded correctly.
**Why it happens:** The ref format is compact; the store API is decomposed.
**How to avoid:** The `resolveRegistryRef` helper must call `parseRegistryRef` first, then pass the decomposed parts to the store method. Use a shared helper function to avoid repeating this in both handlers.

## Code Examples

### Existing Pattern: In-Memory WDL Parse (from handlers_registry.go)
```go
// Source: internal/server/handlers_registry.go classifyWDL
doc, err := parser.Parse(name, []byte(content))
```
This is exactly the pattern needed in handlers after registry resolution.

### Existing Pattern: Tag vs Digest Lookup (from handlers_registry.go)
```go
// Source: internal/server/handlers_registry.go handleRegistryPull
if tag != "" {
    v, entry, err := s.store.GetRegistryVersionByTag(r.Context(), proj.ID, name, tag)
    ...
}
v, entry, err := s.store.GetRegistryVersionByDigest(r.Context(), proj.ID, name, digest)
```
This pattern is directly reusable in `resolveRegistryRef`.

### Existing Pattern: projectIDFromContext (from handlers.go)
```go
// Source: internal/server/handlers.go handleSubmit
ProjectID: projectIDFromContext(r.Context()),
```
Returns empty string when auth is disabled — exactly the check for "registry_ref requires a project-scoped endpoint".

### Existing Pattern: writeError with 422 (from handlers_registry.go)
```go
// Source: internal/server/handlers_registry.go handleRegistryPush
writeError(w, http.StatusUnprocessableEntity, err.Error())
```
422 is already used for WDL resolution failures in registry — consistent pattern for this phase.

### Existing Pattern: ALTER TABLE Migration with Goose Down Recreate
```sql
-- Source: migrations/00005_projects.sql
CREATE TABLE runs_old AS SELECT ... FROM runs;
DROP TABLE runs;
ALTER TABLE runs_old RENAME TO runs;
```
SQLite does not support DROP COLUMN in older versions. The goose down migration must recreate the table.

### Existing Pattern: Nullable Column in CreateRun (from runs.go)
```go
// Source: internal/store/sqlite/runs.go CreateRun
var projectID *string
if r.ProjectID != "" {
    projectID = &r.ProjectID
}
_, err := s.writer.ExecContext(ctx,
    `INSERT INTO runs (id, wdl_path, inputs_json, state, project_id) VALUES (?, ?, ?, ?, ?)`,
    r.ID, r.WDLPath, r.InputsJSON, string(r.State), projectID,
)
```
For `registry_ref` and `registry_digest`, using `NOT NULL DEFAULT ''` avoids the nullable pointer pattern — insert the string directly.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| WDL always from disk file | WDL from registry OR disk | Phase 22 | Handlers must branch on `registry_ref` field |
| `wdl_path NOT NULL` required in submissions | Either `wdl_path` or `registry_ref` required | Phase 22 | Validation logic in handlers changes |
| CLI always resolves path to absolute | CLI auto-detects ref vs path | Phase 22 | `isRegistryRef()` helper needed |

## Open Questions

1. **Where to store resolved WDL content for claim path**
   - What we know: `handleClaim` must send `WDLContent` to daemon; for registry runs, WDLPath is empty.
   - What's unclear: Re-resolve from registry at claim time (extra store call) vs. cache `wdl_content` on runs table (data duplication).
   - Recommendation: Re-resolve at claim time using stored `RegistryDigest`. The content is in `registry_versions`; the second lookup is cheap (indexed by digest). No data duplication. This is the cleaner approach.

2. **Migration number**
   - What we know: Last migration is `00006_registry.sql`.
   - What's unclear: Are any other migrations planned between 21 and 22?
   - Recommendation: Use `00007_run_registry_ref.sql` — next in sequence.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) with httptest |
| Config file | none — tests use `sqlite.OpenMemory()` and `httptest.NewRequest` |
| Quick run command | `go test ./internal/server/... -run TestRegistryRef -v` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| EXEC-01 | `flowy submit` sends `registry_ref` when arg is `name:tag` | unit (CLI) | `go test ./cmd/flowy/... -run TestIsRegistryRef -v` | No — Wave 0 |
| EXEC-01 | POST /workflows with `registry_ref` returns 201 | integration | `go test ./internal/server/... -run TestHandleSubmitRegistryRef -v` | No — Wave 0 |
| EXEC-01 | POST /tasks/run with `registry_ref` returns 201 | integration | `go test ./internal/server/... -run TestHandleTaskRunRegistryRef -v` | No — Wave 0 |
| EXEC-01 | Both `wdl_path` and `registry_ref` returns 400 | integration | `go test ./internal/server/... -run TestHandleSubmitBothFields -v` | No — Wave 0 |
| EXEC-01 | Neither `wdl_path` nor `registry_ref` returns 400 | integration | `go test ./internal/server/... -run TestHandleSubmitNeitherField -v` | No — Wave 0 |
| EXEC-01 | Registry ref on non-project endpoint returns 400 | integration | `go test ./internal/server/... -run TestHandleSubmitRegistryRefNoProject -v` | No — Wave 0 |
| EXEC-01 | Unknown registry ref returns 422 | integration | `go test ./internal/server/... -run TestHandleSubmitRegistryRefNotFound -v` | No — Wave 0 |
| EXEC-02 | `handleClaim` returns non-empty `WDLContent` for registry run | integration | `go test ./internal/server/... -run TestHandleClaimRegistryRun -v` | No — Wave 0 |
| EXEC-02 | Registry run stores `registry_ref` and `registry_digest` in DB | integration | `go test ./internal/server/... -run TestRegistryRunPersistence -v` | No — Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/server/... -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/server/handlers_registry_exec_test.go` (or additions to `handlers_registry_test.go`) — covers EXEC-01, EXEC-02 server-side
- [ ] `cmd/flowy/main_test.go` additions — covers EXEC-01 CLI auto-detection (`TestIsRegistryRef`, `TestRunRun_RegistryRef`)
- [ ] Migration file `internal/store/sqlite/migrations/00007_run_registry_ref.sql` — needed before any store tests

*(Existing test infrastructure: `sqlite.OpenMemory()`, `newTestServer()`, `doRequest()`, `decodeJSON()`, `createRegistryTestProject()`, `addProjectMember()` — all reusable for new tests without modification.)*

## Sources

### Primary (HIGH confidence)
- Direct code read: `internal/server/handlers.go` — handleSubmit (line 117), handleTaskRun (line 191), handleClaim (line 354-426)
- Direct code read: `internal/api/types.go` — SubmitRequest, TaskRunRequest, TaskClaimResponse
- Direct code read: `internal/store/store.go` — Run struct, GetRegistryVersionByTag, GetRegistryVersionByDigest
- Direct code read: `internal/store/sqlite/registry.go` — GetRegistryVersionByTag, GetRegistryVersionByDigest implementations
- Direct code read: `internal/server/handlers_registry.go` — resolution pattern in handleRegistryPull, writeError pattern
- Direct code read: `internal/store/sqlite/runs.go` — CreateRun, scanRun, scanRunRows
- Direct code read: `internal/store/sqlite/migrations/00001_init.sql` — runs table initial schema
- Direct code read: `internal/store/sqlite/migrations/00005_projects.sql` — ALTER TABLE pattern, goose Down recreate pattern
- Direct code read: `internal/store/sqlite/migrations/00006_registry.sql` — last migration number (00006)
- Direct code read: `cmd/flowy/main.go` — runRun, runSubmit, reorderArgs structure
- Direct code read: `internal/config/config.go` — ClientConfig, applyDefaults pattern
- Direct code read: `internal/client/client.go` — Submit, TaskRun methods
- Direct code read: `.planning/config.json` — nyquist_validation: true confirmed

### Secondary (MEDIUM confidence)
- CONTEXT.md canonical refs section — verified against actual file contents (all match)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all dependencies already present in the project; no new libraries needed
- Architecture: HIGH — patterns verified directly against existing source code; every pattern cited has a working code example in the same codebase
- Pitfalls: HIGH — derived from reading actual implementation code and schema; not from external sources

**Research date:** 2026-03-20
**Valid until:** Stable — this is internal code, no external API changes expected
