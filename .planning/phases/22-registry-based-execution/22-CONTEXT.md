# Phase 22: Registry-Based Execution - Context

**Gathered:** 2026-03-20
**Status:** Ready for planning

<domain>
## Phase Boundary

Executors can submit workflow runs and task runs using a registry reference (`name:tag` or `name@sha256:hex`) instead of a WDL file path. The server resolves the reference to WDL content from the project registry and executes it identically to file-path submissions. Both modes coexist — neither breaks the other. CLI gains auto-detection of registry references.

</domain>

<decisions>
## Implementation Decisions

### Reference input format
- New `registry_ref` field on both `SubmitRequest` and `TaskRunRequest` (not overloading `wdl_path`)
- Exactly one of `wdl_path` or `registry_ref` must be set — mutually exclusive, 400 if both provided
- Reference format: `name:tag` or `name@sha256:hex` — short form, always resolves within current project context
- Both POST /workflows (handleSubmit) and POST /tasks/run (handleTaskRun) accept `registry_ref`

### Resolution flow
- Early resolve in handler: check `registry_ref` first, call store.GetRegistryVersionByTag/Digest to fetch WDL content, then pass to `parser.Parse(name, []byte(content))` instead of reading from disk
- No middleware — resolution is a few lines at the top of each handler before the existing parse call
- Both file-path and registry-ref modes always work regardless of auth mode — registry is additive, not replacing

### Run record storage
- For registry runs: `WDLPath` is empty, `RegistryRef` stores the user-provided reference (e.g. `align-reads:prod`)
- `RegistryDigest` also stored — the resolved `sha256:hex` at submission time, for reproducibility and Phase 23 provenance
- For file-path runs: `WDLPath` is set, `RegistryRef` and `RegistryDigest` are empty
- Requires migration to add `registry_ref` and `registry_digest` columns to runs table

### CLI integration
- `flowy run` and `flowy submit` auto-detect registry refs from positional argument
- Detection rule: if arg contains `:` (not as Windows drive letter) or `@sha256:`, treat as registry ref; otherwise treat as file path
- `flowy run align-reads:prod -i reads=sample.bam` works identically to `flowy run hello.wdl -i reads=sample.bam`
- Project context: add `default_project` to flowy config; `--project` flag overrides; required when using registry refs
- Input handling identical for both modes — same `-i` flag, same `--inputs` JSON file support

### Error semantics
- Registry ref that doesn't resolve: 422 Unprocessable Entity
- Specific error messages matching registry pull pattern: `entry 'foo' not found` vs `tag 'prod' not found for entry 'foo'`
- Both fields provided: 400 `exactly one of wdl_path or registry_ref must be set, not both`
- Neither field provided: 400 `one of wdl_path or registry_ref is required`
- Registry ref on non-project endpoint (auth disabled): 400 `registry_ref requires a project-scoped endpoint`

### Claude's Discretion
- Exact ref parsing implementation (splitting on `:` vs `@sha256:`)
- How to structure the resolution helper function (shared between handleSubmit and handleTaskRun)
- Migration number for adding columns to runs table
- Config file format for `default_project` setting
- Test structure — integration tests vs unit tests for resolution logic

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing submission handlers
- `internal/server/handlers.go` — handleSubmit (line 118) and handleTaskRun (line 194): current WDL resolution from file path
- `internal/api/types.go` — SubmitRequest, TaskRunRequest, TaskClaimResponse with WDLContent field

### Registry store layer (Phase 21)
- `internal/store/store.go` — Store interface with GetRegistryVersionByTag, GetRegistryVersionByDigest methods
- `internal/store/sqlite/registry.go` — SQLite implementation of registry store methods
- `internal/server/handlers_registry.go` — Registry pull handler shows resolution pattern (tag vs digest lookup)

### Run persistence
- `internal/store/store.go` — Run struct definition, CreateRun method
- `internal/store/sqlite/migrations/` — existing migration files (00001 through 00006)

### CLI
- `cmd/flowy/main.go` — CLI entry point, flag parsing, run/submit commands
- `internal/client/client.go` — Client.TaskRun and submit methods

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `store.GetRegistryVersionByTag(ctx, entryID, tag)` and `store.GetRegistryVersionByDigest(ctx, entryID, digest)` — already return WDL content as string
- `parser.Parse(name, []byte(content))` — accepts in-memory content, no file system needed
- `projectIDFromContext(r.Context())` — extracts project ID for registry lookups
- `ErrorResponse` struct and `writeError` helper for consistent error formatting

### Established Patterns
- Handler validation: check required fields first, return 400 early
- `parser.Parse(path, nil)` for file paths vs `parser.Parse(name, []byte(content))` for in-memory — both already used
- `store.Run` struct with nullable string fields (e.g. `ProjectID` is empty when auth disabled)

### Integration Points
- `handleSubmit` and `handleTaskRun` — add registry_ref branch before existing parser.Parse call
- `store.Run` struct — add RegistryRef and RegistryDigest fields
- New migration — add columns to runs table
- `cmd/flowy/main.go` — add ref detection in positional arg processing
- `internal/config/config.go` — add DefaultProject field

</code_context>

<specifics>
## Specific Ideas

- Registry references use the same `name:tag` / `name@sha256:hex` format as Docker image refs — familiar to users
- The `RegistryDigest` field on Run enables Phase 23 provenance tracking without re-resolution
- Auto-detection in CLI means users don't need to learn a new flag — `flowy run align-reads:prod` just works

</specifics>

<deferred>
## Deferred Ideas

- Cross-project registry references (`project/name:tag`) — future when cross-project sharing (REG-F01) is implemented
- Registry-only mode (force all runs through registry when auth enabled) — could be a config option in a future phase
- Import resolution for registry WDLs (WDL files in registry that import other WDL files) — requires import resolver changes, separate phase

</deferred>

---

*Phase: 22-registry-based-execution*
*Context gathered: 2026-03-20*
