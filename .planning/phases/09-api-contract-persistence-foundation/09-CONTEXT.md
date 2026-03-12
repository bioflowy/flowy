# Phase 9: API Contract + Persistence Foundation - Context

**Gathered:** 2026-03-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Shared foundation packages (`internal/config`, `internal/store`, `internal/api`) that all three binaries depend on. Includes YAML config loader, SQLite persistence with WAL mode and goose migrations, HTTP DTO types, and shared filesystem layout helpers. No binary entry points (those are Phases 10-12).

</domain>

<decisions>
## Implementation Decisions

### Config file discovery
- Search order: `--config` flag first, then `./flowy.yaml` in current directory
- Client (`flowy`) works without config — defaults to connecting `localhost:8124`
- Server (`flowy-server`) and daemon (`flowy-daemon`) require a config file — error if not found
- No environment variable lookup (keep it simple)

### Default port
- Default server listen address: `:8124`
- Client default server URL: `http://localhost:8124`
- Port 8124 chosen as an uncommon port to avoid conflicts

### Run ID format
- Full UUID v4 for workflow run IDs (36-char format: `a1b2c3d4-e5f6-7890-abcd-ef1234567890`)
- Prefix matching supported in client commands (like git short hashes — type enough chars to be unique, error on ambiguity)
- Task IDs are sequential integers (auto-increment) — internal, users rarely reference them directly

### Workflow run states
- 5 states: PENDING → RUNNING → COMPLETED / FAILED / ABORTED
- PENDING: submitted, waiting for tasks to be scheduled
- RUNNING: at least one task executing
- COMPLETED: all tasks succeeded
- FAILED: a task failed (unrecoverable)
- ABORTED: user cancelled via abort API

### Task states
- 6 states: PENDING → CLAIMED → RUNNING → COMPLETED / FAILED / ABORTED
- CLAIMED: daemon reserved the task atomically but has not yet started Docker execution
- Stale detection applies to both CLAIMED and RUNNING tasks (heartbeat expired → reset to PENDING)
- ABORTED: cascaded from parent run abort (PENDING/CLAIMED → immediate ABORTED; RUNNING → cancel signal → ABORTED)

### Config defaults
- Heartbeat: daemon sends every 30 seconds, server timeout at 60 seconds (1 missed heartbeat tolerated)
- Base directory: `./flowy-data` (relative to working directory; HPC users override to NFS mount)
- SQLite database: `{base_dir}/flowy.db` by default; `db_dsn` in config can override for advanced setups
- DB driver: `sqlite` default (interface supports future `postgres` backend via PG-01)

### Claude's Discretion
- Exact Config struct field naming and YAML tag naming
- Store interface method signatures beyond what research defined
- Migration file naming convention details
- Error message wording
- Test file organization

</decisions>

<specifics>
## Specific Ideas

- Client must always connect to server — no local execution fallback mode (server is required for all execution)
- Run ID prefix matching should behave like `git` — error with "ambiguous run ID" message listing matches when prefix is not unique

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cmd/flowy/main.go`: Existing CLI entry point — will be extended in Phase 12
- `go.mod`: Uses `github.com/bioflowy/flowy` module path; yaml.v3 already in go.sum

### Established Patterns
- Hand-written lexer/parser (Rob Pike pattern) — no framework dependencies
- `errgroup` for bounded concurrency — may inform store connection pooling
- `wdl/workflow/scheduler.go` has `runTaskFn` injection point for distributed dispatch

### Integration Points
- New packages go under `internal/` (currently empty — Phase 9 creates the directory structure)
- `cmd/flowy-server/` and `cmd/flowy-daemon/` directories created in Phases 10-11 (not Phase 9)
- Config and store packages must compile independently with no cross-dependencies

</code_context>

<deferred>
## Deferred Ideas

- PostgreSQL backend (PG-01) — v2+ scope; Store interface prepared in Phase 9

</deferred>

---

*Phase: 09-api-contract-persistence-foundation*
*Context gathered: 2026-03-11*
