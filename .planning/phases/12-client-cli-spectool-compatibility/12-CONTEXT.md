# Phase 12: Client CLI + Spectool Compatibility - Context

**Gathered:** 2026-03-12
**Status:** Ready for planning

<domain>
## Phase Boundary

The `flowy` binary is extended with `submit`, `status`, `outputs` subcommands for distributed workflows. `flowy run` is replaced with a synchronous server-based implementation (submit → poll → print outputs). `flowy run --task` also goes through the server via a new task endpoint. The `check` subcommand remains unchanged (local parse + type-check). A new `internal/client/` package provides the HTTP client. File staging copies local input files to the shared filesystem using a heuristic approach.

</domain>

<decisions>
## Implementation Decisions

### `run` command behavior
- `flowy run` submits to server, polls until terminal state, prints outputs on success — no local fallback
- `flowy run --task <name>` also goes through the server via a new `POST /tasks/run` endpoint (not local Docker)
- Spectool requires server+daemon running for all tests (both workflow and task)
- Keep `--verbose` flag (print task stdout/stderr) — works by fetching output from shared FS
- Drop `--dir` flag — server/daemon controls execution directories
- Keep `--path` flag — send as `search_paths` field in submit request so server passes it to the parser
- `--output <file>` flag retained for writing outputs JSON to a file instead of stdout

### Polling feedback
- Print periodic status lines to stderr on state changes: `run abc12345: PENDING`, `run abc12345: RUNNING`
- Only print when state changes, not every poll cycle
- Add `--quiet` flag to suppress status lines (spectool can use this if needed)
- On success: print JSON outputs to stdout
- On failure: print `error: workflow FAILED (run: <full-uuid>)` + `See flowy-data/runs/<uuid>/.../tasks/ for logs` to stderr, exit code 1

### `--server` and `--config` flags
- All server-facing subcommands (`submit`, `status`, `outputs`, `run`) accept `--server <url>` flag
- All server-facing subcommands accept `--config <path>` flag
- Priority: `--server` flag > config file > default `http://localhost:8124`
- Per-subcommand flags (not top-level), consistent with existing `flag.NewFlagSet` pattern

### Input file staging
- Heuristic approach: scan input JSON string values, resolve absolute and relative paths, check if file exists locally and is NOT already under `baseDir` — if so, copy to shared FS
- Client generates a UUID locally as the run ID, stages files to `{baseDir}/runs/{uuid}/inputs/`, sends UUID as `run_id` hint in `SubmitRequest`
- Server accepts optional `run_id` field in `SubmitRequest` — uses it if provided, generates one if not
- WDL file also staged if its path is not already under `baseDir`
- Clean up staged directories on submission failure (deferred cleanup)
- Long-term vision: Galaxy-like model where files must be imported to server first (`flowy import`) — deferred to future phase

### New server endpoints (scope extension)
- `POST /tasks/run` — single-task execution endpoint for `flowy run --task`
- `SubmitRequest` extended with optional `run_id` (client-generated UUID hint) and `search_paths` (import search directories)
- `handleStatus` and `handleOutputs` updated to support run ID prefix matching via `GetRunByPrefix` (decided in Phase 9)

### Claude's Discretion
- Poll interval (default ~2 seconds, not user-configurable)
- `POST /tasks/run` request/response DTO design
- Exact error messages and formatting
- How `--verbose` retrieves task stdout/stderr from shared FS in server mode
- Test file organization (client_test.go structure)
- Whether `config.Default()` or alternative approach for config-less client operation

</decisions>

<specifics>
## Specific Ideas

- Status line format: `run abc12345: RUNNING` (short prefix of run ID, like git)
- Failure output includes path hint to task logs on shared FS for debugging
- Spectool can use `--quiet` to suppress stderr status lines if they interfere
- `flowy submit` stdout is plain UUID only (no decoration) — enables `RUN_ID=$(flowy submit ...)`
- `flowy status` stdout is plain state string — enables `if [ "$(flowy status $ID)" = "COMPLETED" ]`

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/daemon/daemon.go`: HTTP client pattern (`claimTask`, `reportResult`, `sendHeartbeat`) — exact model for `internal/client/Client`
- `internal/api/types.go`: `SubmitRequest`, `SubmitResponse`, `StatusResponse`, `OutputsResponse`, `ErrorResponse` all defined
- `internal/config/config.go`: `ClientConfig{ServerURL}`, `applyDefaults()` — needs `Default()` exported function
- `internal/store/rundir.go`: `CreateRunDirs`, `RunInputsDir`, `TaskDir` — for file staging paths
- `cmd/flowy/main.go`: Existing `run()` dispatcher with `switch args[0]`, `runRun()` function to replace, `runCheck()` to preserve unchanged
- `github.com/google/uuid` v1.6.0: Already in go.mod for client-side UUID generation

### Established Patterns
- `flag.NewFlagSet` per subcommand (no cobra — Phase 7 decision)
- `run(args, stdout, stderr) int` testable-main pattern
- `reorderArgs()` for spectool flag compatibility (flags after positional args)
- `httptest.NewServer` in `internal/daemon/daemon_test.go` — pattern for client tests
- `internal/api` is a strict leaf package (stdlib only)

### Integration Points
- `cmd/flowy/main.go`: Add `submit`, `status`, `outputs` cases to switch; replace `runRun()` implementation
- `internal/client/client.go`: New package — `Client` struct with `Submit`, `Status`, `Outputs` methods
- `internal/api/types.go`: Add `RunID string` and `SearchPaths []string` to `SubmitRequest`; add task run DTO types
- `internal/server/handlers.go`: Add `handleTaskRun` endpoint; update `handleStatus`/`handleOutputs` for prefix matching
- `internal/config/config.go`: Add `Default() *Config` exported function

</code_context>

<deferred>
## Deferred Ideas

- `flowy import` command — Galaxy-like file import model where files must be explicitly imported to server before use as inputs. Replace heuristic staging with explicit imports. Future milestone.
- Task-level error details in API response — server currently doesn't expose task-level failure details (exit code, failed task name) via `OutputsResponse`. Future API enhancement.

</deferred>

---

*Phase: 12-client-cli-spectool-compatibility*
*Context gathered: 2026-03-12*
