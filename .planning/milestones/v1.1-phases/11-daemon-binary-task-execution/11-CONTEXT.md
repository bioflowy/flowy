# Phase 11: Daemon Binary + Task Execution - Context

**Gathered:** 2026-03-12
**Status:** Ready for planning

<domain>
## Phase Boundary

`flowy-daemon` binary that polls the server for available tasks, claims them atomically, executes them via Docker using the existing `task.RunTask` pipeline, writes outputs to the shared filesystem, and handles graceful shutdown. Does NOT include client CLI (Phase 12) or server changes beyond adding `wdl_path` to TaskClaimResponse.

</domain>

<decisions>
## Implementation Decisions

### Concurrency model
- Configurable number of workers via `--workers=N` flag and `workers` field in DaemonConfig YAML section
- Default: 1 worker (safe default, single-task behavior out of the box)
- CLI flag overrides config file value
- Each worker is an independent goroutine that claims and executes tasks in a loop
- Scatter parallelism achieved by running multiple workers per daemon or multiple daemon processes

### Graceful shutdown
- On SIGTERM: stop claiming new tasks, wait for ALL in-flight workers to finish their Docker execution, then exit
- No hard timeout — let Docker containers complete naturally (stale detection handles truly stuck daemons)
- Use `sync.WaitGroup` to track in-flight workers; poll context cancelled stops the claim loop

### Daemon identity
- daemon_id format: `hostname-PID` (e.g. `node01-12345`) — auto-generated, not configurable
- Generated once at process start, used for all claim and heartbeat requests
- No override via config or flag — keeps it simple, avoids misconfiguration

### Startup logging
- Log daemon_id, server URL, worker count, and base_dir on startup
- Example: `daemon starting daemon_id=node01-12345 server=http://localhost:8124 workers=1 base_dir=/mnt/shared/flowy-data`

### Polling strategy
- Claude's Discretion (not discussed — see below)

### Claude's Discretion
- Polling interval (short-poll with fixed interval vs long-poll; STATE.md noted this for evaluation)
- Log format (structured JSON vs plain text)
- Exact error handling and retry logic for HTTP failures
- Heartbeat goroutine design (per-worker vs single daemon-level)
- WDL parsing approach (how daemon uses wdl_path from claim response)
- Output serialization format for outputs.json

</decisions>

<specifics>
## Specific Ideas

- Workers default to 1 so the daemon behaves predictably out of the box — operators opt-in to concurrency
- daemon_id as `hostname-PID` gives human-readable identity for HPC environments where operators ssh into nodes

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/config/config.go`: DaemonConfig struct already has `ServerURL` and `HeartbeatSec` fields — add `Workers int` field
- `internal/api/types.go`: TaskClaimRequest/Response, HeartbeatRequest, TaskResultRequest all defined
- `wdl/task/task.go`: `RunTask(ctx, task, jsonInputs, runner, execRoot, structDefs...)` — the full execution pipeline
- `wdl/task/task.go`: `NewDockerRunner()` — creates Docker container runner
- `internal/store/rundir.go`: `TaskDir()` helper for constructing per-task directory paths

### Established Patterns
- `errgroup` for bounded concurrency in scheduler — similar pattern for worker pool
- Server uses `context.Background()` for long-lived goroutines (dispatch, stale detection)
- `internal/api` is a strict leaf package (stdlib only) — adding `WDLPath` field is safe

### Integration Points
- `cmd/flowy-daemon/main.go`: New binary entry point (load config, construct daemon, run poll loop, handle signals)
- `internal/api/types.go`: Add `WDLPath string` to TaskClaimResponse
- `internal/server/handlers.go`: Populate `WDLPath` in handleClaim via `store.GetRun()`
- `internal/config/config.go`: Add `Workers int` to DaemonConfig

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 11-daemon-binary-task-execution*
*Context gathered: 2026-03-12*
