# Phase 10: Server Binary + DAG Dispatch - Context

**Gathered:** 2026-03-11
**Status:** Ready for planning

<domain>
## Phase Boundary

`flowy-server` binary that accepts workflow submissions via REST API, schedules the DAG in-process using `runTaskFn` injection, and dispatches tasks to the SQLite queue for daemon pickup. Includes all daemon-facing endpoints (claim, result, heartbeat) so Phase 11 only builds the daemon client. Does NOT include daemon implementation (Phase 11) or client CLI (Phase 12).

</domain>

<decisions>
## Implementation Decisions

### REST URL paths
- User-facing endpoints under `/workflows`: POST /workflows, GET /workflows/{id}/status, GET /workflows/{id}/outputs, POST /workflows/{id}/abort
- Daemon-facing endpoints under `/tasks`: POST /tasks/claim, POST /tasks/{id}/result
- Heartbeat at POST /heartbeat (flat, daemon-level not task-level)
- Health check at GET /health (flat, standard for liveness probes)
- All error responses use JSON `ErrorResponse` body, including 404/405 (override chi defaults)
- Fix `api/types.go` comments to use `/workflows` instead of `/runs`

### Daemon-facing API scope
- Phase 10 implements all three daemon endpoints: claim, result, heartbeat
- POST /tasks/claim returns HTTP 204 No Content when no PENDING tasks exist (daemon retries on its own interval)
- POST /heartbeat with `{daemon_id}` refreshes all RUNNING tasks owned by that daemon (single call per interval regardless of task count)
- Background goroutine runs stale task detection periodically using `FindStaleTasks` from store, resets stale tasks to PENDING

### Task I/O on shared FS
- Per-task directory layout: `{base_dir}/runs/{run_id}/tasks/{task_name}_{shard_index}/`
- Task inputs written as `inputs.json` (single JSON file with all input key-value pairs)
- Task outputs written as `outputs.json` by daemon (single JSON file, symmetric with inputs)
- Daemon creates the per-task directory (server only writes task row with directory paths in claim response)
- Server's `waitForTask` reads `outputs.json` from the per-task directory on COMPLETED

### Dispatch wait strategy
- `dispatchFn` polls the store every 2 seconds to check task state
- No timeout — wait indefinitely until daemon completes or run is aborted (HPC jobs can take hours)
- Stale detection handles dead daemons: task reset to PENDING, another daemon picks it up, dispatchFn transparently sees eventual completion (auto-retry)
- Log a warning when a dispatched task has been unclaimed for >5 minutes (helps operators notice missing daemons)

### Claude's Discretion
- Exact middleware ordering (Logger, Recoverer, MaxBytesReader)
- Internal server package structure (server.go, handlers.go, dispatch.go split)
- Error message wording for validation failures
- Stale detection interval (reasonable default, e.g., every 30s or 60s)
- Test file organization and helper structure

</decisions>

<specifics>
## Specific Ideas

- `api/types.go` comments must be updated to reflect `/workflows` not `/runs` — this is a bug from Phase 9
- `RunWorkflowWithDispatch` exported function in `wdl/workflow/` is the clean API extension for injecting dispatchFn (do NOT modify unexported `taskRunner` field from outside package)
- `GetTaskByID` must be added to Store interface and SQLiteStore — needed for efficient task polling in waitForTask

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/store/store.go`: Store interface with CreateRun, GetRun, CreateTask, ListTasksByRun, ClaimTask, UpdateTaskState, UpdateHeartbeat, FindStaleTasks — all ready for server handlers
- `internal/api/types.go`: All DTO types already defined (SubmitRequest/Response, StatusResponse, OutputsResponse, AbortResponse, ErrorResponse, TaskClaimRequest/Response, HeartbeatRequest, TaskResultRequest)
- `internal/config/config.go`: Config struct with Server.ListenAddr, DefaultDSN() — ready for main.go
- `wdl/workflow/scheduler.go`: `taskRunner` field (unexported) and `runTaskFn` package-level var — injection seam for distributed dispatch
- `wdl/workflow/workflow.go`: `RunWorkflow` function — needs `RunWorkflowWithDispatch` variant
- `github.com/go-chi/chi/v5` v5.2.5: Already in go.mod
- `github.com/google/uuid` v1.6.0: Already in go.mod

### Established Patterns
- `errgroup` for bounded concurrency in scheduler — scheduler already handles parallel task execution
- Hand-written lexer/parser (no frameworks) — keep server code similarly minimal
- `internal/api` is a strict leaf package (stdlib only) — server imports api, not the other way around
- Store interface pattern — server imports `internal/store` (interface), not `internal/store/sqlite` (implementation)

### Integration Points
- `cmd/flowy-server/main.go`: New binary entry point (thin: load config, open store, construct server, ListenAndServe, shutdown)
- `internal/server/`: New package for Server struct, handlers, and dispatch logic
- `wdl/workflow/workflow.go`: Add `RunWorkflowWithDispatch` exported function
- `internal/store/store.go`: Add `GetTaskByID` to Store interface
- `internal/store/sqlite/`: Implement `GetTaskByID` in SQLiteStore
- `internal/store/rundir.go`: May need extension for per-task directory path helpers

</code_context>

<deferred>
## Deferred Ideas

- Long-poll for daemon task claiming (replace short-poll 204 with blocking wait) — evaluate in Phase 11 per STATE.md blocker note
- WebSocket/SSE for real-time status updates — out of scope per REQUIREMENTS.md

</deferred>

---

*Phase: 10-server-binary-dag-dispatch*
*Context gathered: 2026-03-11*
