# Phase 23: Provenance Tracking - Context

**Gathered:** 2026-03-20
**Status:** Ready for planning

<domain>
## Phase Boundary

Record a provenance audit trail for every registry-based run linking it to its registry version, inputs, submitting user, and (after completion) outputs. Viewers can list provenance records for a project and retrieve a single record by run ID. File-path (non-registry) runs do not generate provenance records. Per-task provenance (PROV-F01) is deferred to a future milestone.

</domain>

<decisions>
## Implementation Decisions

### Data model & storage
- Separate `provenance` table (not enriching the existing runs table) — cleaner separation, easier to extend for future per-task provenance (PROV-F01)
- Normalized + snapshot approach: FK references to `runs`, `users`, `registry_entries`, `registry_versions` for clean indexed joins, PLUS snapshot `registry_ref` and `registry_digest` as TEXT columns so provenance survives registry entry deletion
- Inputs and outputs are NOT duplicated in provenance — read from `runs` table via join on `run_id` FK
- Provenance record is 1:1 with run (one provenance per run)

### Recording triggers
- Provenance created inline in `handleSubmit` and `handleTaskRun` — immediately after `CreateRun` succeeds, call `CreateProvenance` in the same handler
- Only for registry-based runs (`registry_ref != ""`) — file-path runs do not create provenance
- Provenance updated when run completes — in the `UpdateRunOutputs`/completion path, mark provenance `updated_at` (outputs read via join, no separate write needed)
- Provenance created for ALL registry runs regardless of outcome — failed and aborted runs have provenance with no outputs, preserving the audit trail of what was attempted

### Query API design
- List: `GET /projects/{slug}/provenance` — returns summary list for project
- Detail: `GET /projects/{slug}/provenance/{run_id}` — returns full record with version, digest, inputs, outputs, user
- Summary list fields per item: `run_id`, `registry_ref`, `registry_digest`, `user` (username), `run_state`, `created_at` — no inputs/outputs in list
- Detail fields: all summary fields + `inputs_json`, `outputs_json`, `registry_entry_name`, `registry_entry_kind`, `updated_at`
- Basic query filters on list endpoint: `?entry=<name>` (filter by registry entry name), `?user=<username>` — enough for "all runs of tool X" and "all runs by user Y"
- No pagination for v1 — can be added later
- Viewer role or above can access provenance (consistent with registry list permission)

### File-path run handling
- File-path (non-registry) runs do NOT generate provenance records
- Provenance is only meaningful when there's a versioned registry reference for reproducibility
- This keeps provenance clean — every record has a reproducible version digest

### Claude's Discretion
- Provenance table column specifics (exact column names, index choices)
- Migration number (next after 00007_run_registry_ref.sql)
- Whether to use a single store method or separate Create/Update methods for provenance lifecycle
- Detail response JSON structure beyond the specified fields
- Filter implementation (exact query parameter names, SQL WHERE generation)
- Test structure and organization

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Run submission handlers (provenance creation point)
- `internal/server/handlers.go` — handleSubmit and handleTaskRun: where CreateProvenance call will be added after CreateRun
- `internal/server/handlers.go` — resolveRegistryRef: returns registry_ref, registry_digest that provenance needs

### Store layer (model and persistence)
- `internal/store/store.go` — Run struct (has RegistryRef, RegistryDigest, InputsJSON, OutputsJSON), Store interface (new provenance methods will be added here)
- `internal/store/sqlite/runs.go` — CreateRun, UpdateRunOutputs patterns to follow for provenance

### Registry store (FK targets)
- `internal/store/sqlite/registry.go` — GetRegistryVersionByTag/GetRegistryVersionByDigest for resolving entry_id and version_id
- `internal/store/store.go` — RegistryEntry, RegistryVersion structs (FK targets for provenance)

### Auth context (submitting user)
- `internal/server/handlers.go` — projectIDFromContext pattern for extracting user context from JWT claims
- `internal/auth/` — JWT claims structure, userIDFromContext if exists

### Migration patterns
- `internal/store/sqlite/migrations/` — existing migrations 00001-00007, goose format

### Existing API types
- `internal/api/types.go` — existing response structs to follow for provenance response types

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `resolveRegistryRef` in handlers.go — already returns the entry name, digest, and WDL content; can be used to extract `registry_entry_id` and `registry_version_id` for provenance FK population
- `projectIDFromContext(r.Context())` — extracts project ID for scoping provenance queries
- `writeError`/`ErrorResponse` pattern — consistent error responses
- `newTestServer()`/`doRequest()`/`createRegistryTestProject()` test helpers — reusable for provenance integration tests

### Established Patterns
- Store interface with SQLite implementation — new provenance methods follow same pattern
- `NOT NULL DEFAULT ''` for string columns that may be empty (Phase 22 convention)
- goose migrations with `-- +goose Up` / `-- +goose Down` markers
- Handler pattern: decode request → validate → store call → write JSON response
- `scanRun`/`scanRunRows` pattern for reading rows into structs

### Integration Points
- `handleSubmit` and `handleTaskRun` — add CreateProvenance call after CreateRun
- `UpdateRunOutputs` or run completion path — update provenance `updated_at`
- `server.go` route registration — add `/projects/{slug}/provenance` routes in project middleware group
- Store interface — add CreateProvenance, GetProvenance, ListProvenance methods

</code_context>

<specifics>
## Specific Ideas

- Normalized FK references enable efficient "all runs of tool X" queries via indexed `registry_entry_id` column
- Snapshot `registry_ref` and `registry_digest` strings ensure provenance survives even if registry entries are deleted — audit trail permanence
- Summary list keeps responses small for projects with many runs; detail endpoint provides full data including inputs/outputs via join

</specifics>

<deferred>
## Deferred Ideas

- Per-task W3C PROV entity-activity-agent graph (PROV-F01) — 3-5x scope of run-level provenance; future milestone
- CWLProv-style research object export (PROV-F02) — academic publishing use case; future milestone
- Pagination on list endpoint — add when project run counts warrant it
- File-path run provenance — could be added later if audit requirements expand

</deferred>

---

*Phase: 23-provenance-tracking*
*Context gathered: 2026-03-20*
