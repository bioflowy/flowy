# Pitfalls Research

**Domain:** Distributed WDL workflow engine — splitting monolithic CLI into server/daemon/client
**Researched:** 2026-03-11
**Confidence:** HIGH (patterns verified against Cromwell, Temporal, and distributed systems literature; codebase reviewed directly)

---

## Critical Pitfalls

### Pitfall 1: File Path Assumptions — Absolute Host Paths Embedded in Task Records

**What goes wrong:**
The monolithic engine uses `os.MkdirTemp(execRoot, task.Name+"-")` to create absolute paths like `/tmp/flowy-20260311-a1b2/task-abc123/work`, then passes those absolute paths through the entire execution pipeline. In the distributed architecture, the server creates these paths when scheduling, then sends them to a daemon running on a different host. The daemon receives `/tmp/flowy-.../work` — a path that does not exist on its filesystem.

The existing `PathMap` in `wdl/task/pathmap.go` resolves relative paths to absolute at call time using the calling process's CWD. When this runs on the daemon host, `filepath.Abs()` resolves against the daemon's CWD, producing daemon-local absolute paths that are meaningless to the server.

**Why it happens:**
The in-process model never needs to distinguish "where was this path created" from "where will this path be used." Adding a network boundary makes this invisible assumption explicit and fatal. Developers typically discover this only when the first task runs on a remote daemon.

**How to avoid:**
Define a shared execution root as a config-driven directory that is the same absolute path on both the server and all daemons (NFS mount, e.g., `/mnt/flowy-shared/runs`). The server creates subdirectories under this root; daemons read files under the same root. Never embed the server's local `/tmp` in task records. The config must enforce this: daemon startup should validate the configured shared root is accessible and writable.

Document the constraint explicitly: if server and daemon run on the same host (development mode), the shared root can be a local directory, but the code must not assume this.

**Warning signs:**
- First remote daemon run fails with "no such file or directory" on input file paths.
- Works on single-host dev setup, breaks on any two-host deployment.
- `filepath.Abs()` in `pathmap.go` resolves differently on daemon than on server.

**Phase to address:**
Phase 1 (Server + Daemon foundation) — The shared filesystem design must be locked before any task record serialization is implemented.

---

### Pitfall 2: Task State Transition Race — Double-Claim by Multiple Daemons

**What goes wrong:**
Multiple daemons polling the server simultaneously can both see a task in `PENDING` state and both attempt to claim it. Without an atomic compare-and-swap on the state transition, both daemons receive the task, run it in parallel, and both report completion. The server either accepts a duplicate result (corrupting outputs) or rejects one (causing spurious failure).

This is the most common race condition in polling-based distributed work queues.

**Why it happens:**
The naive implementation is: daemon polls → GET /tasks/pending → pick first → POST /tasks/{id}/claim. The GET and the POST are separate HTTP round trips. Between them, another daemon can execute the same sequence.

**How to avoid:**
Implement claiming as a single atomic SQL UPDATE with a WHERE clause that checks the current state:

```sql
UPDATE tasks
SET state = 'RUNNING', daemon_id = ?, claimed_at = ?
WHERE id = ? AND state = 'PENDING'
```

Return the number of rows affected. If 0 rows were updated, the claim failed (another daemon won); the daemon must discard this task and poll again. Do not use a SELECT followed by an UPDATE — this is a TOCTOU race even within a single SQLite connection unless the entire sequence is wrapped in an exclusive transaction.

In WAL mode, SQLite serializes all writes, so the atomic UPDATE is safe. With multiple daemons connecting over the network to the server (not directly to SQLite), this logic lives in the server and is naturally serialized.

**Warning signs:**
- The same task appears in `running` state twice in the database.
- Duplicate output files with conflicting content.
- Tasks that complete twice: server logs show two `COMPLETED` transitions for the same task ID.

**Phase to address:**
Phase 1 (Server + Daemon foundation) — The claim endpoint must use this pattern from its first commit.

---

### Pitfall 3: SQLite Concurrent Access — Server + Background Goroutines Without WAL Mode

**What goes wrong:**
The server uses SQLite for workflow and task state persistence. The server runs multiple goroutines: the HTTP handler goroutine (serving daemon polls), the scheduler goroutine (advancing the DAG), and potentially a heartbeat-checker goroutine. These all write to SQLite concurrently. Without WAL mode, SQLite uses journal mode where a write blocks all readers. Under load, goroutines contend for the write lock, producing `SQLITE_BUSY` errors.

The Go `database/sql` connection pool also causes problems: if multiple goroutines hold connections from the pool and one begins a write transaction, others may get `database is locked` even in WAL mode if the connection pool size exceeds SQLite's single-writer limit.

**Why it happens:**
SQLite WAL mode is not the default. Using `database/sql` with the default pool size of N allows N concurrent open transactions, which exceeds SQLite's concurrency model for writers.

**How to avoid:**
Enable WAL mode at database open time:
```sql
PRAGMA journal_mode=WAL;
PRAGMA busy_timeout=5000;
```

Set `db.SetMaxOpenConns(1)` for the write connection (or use a single-connection pool for writes). Reads can use a separate read-only connection pool. Alternatively, use a single goroutine as the database actor (all DB writes go through a channel to this goroutine), which eliminates concurrency entirely.

For the v1.1 milestone with a single server process and a handful of daemons, the simple approach is sufficient: `SetMaxOpenConns(1)`, WAL mode, `busy_timeout=5000ms`.

**Warning signs:**
- `database is locked` errors under any concurrent load.
- Flaky integration tests that pass alone but fail when run in parallel.
- Server hangs when multiple daemons poll simultaneously.

**Phase to address:**
Phase 1 (Server + Daemon foundation) — Configure at database initialization; not something to retrofit.

---

### Pitfall 4: Daemon Crash Leaves Tasks in RUNNING State Forever (No Heartbeat)

**What goes wrong:**
A daemon claims a task, starts the Docker container, and then crashes (OOM, host reboot, network partition). The task remains in `RUNNING` state in the server's database indefinitely. The server's DAG scheduler waits for this task to complete before advancing, so the entire workflow hangs.

Without a heartbeat mechanism, the server cannot distinguish "daemon is running this task" from "daemon crashed 20 minutes ago."

**Why it happens:**
In the monolithic model, process death = engine death = clean state. Adding a daemon introduces a separate process whose liveness must be tracked independently.

**How to avoid:**
Implement a two-level timeout system modeled on Temporal's heartbeat pattern:

1. **Daemon heartbeat**: Daemon sends `POST /tasks/{id}/heartbeat` every N seconds while the task runs. The server records `last_heartbeat_at` on the task row.

2. **Server reaper**: A background goroutine on the server runs every M seconds (M > N) and marks any `RUNNING` task where `last_heartbeat_at < now - timeout` as `FAILED` with reason `daemon_timeout`. This makes the task available for retry or marks the workflow failed.

3. **Task claimed_at safety net**: Even if the heartbeat goroutine is never started, any task stuck in `RUNNING` for more than `max_task_duration` should be reaped.

Timeout values must be configurable (not hardcoded). Reasonable defaults: heartbeat interval 30s, reaper interval 60s, timeout 3x heartbeat interval (90s).

The daemon must send heartbeats from a goroutine separate from the Docker execution goroutine — not after the container exits.

**Warning signs:**
- Workflows hang indefinitely after a daemon is killed.
- Tasks remain in `RUNNING` state after daemon process is gone.
- No `last_heartbeat_at` column in the tasks table.

**Phase to address:**
Phase 1 (Server + Daemon foundation) — The heartbeat mechanism is load-bearing for correctness; cannot be deferred.

---

### Pitfall 5: Orphaned Docker Containers When Daemon Receives SIGTERM

**What goes wrong:**
The daemon is executing a Docker container when it receives SIGTERM (graceful shutdown signal). If the daemon exits immediately, the Docker container continues running on the host — there is no parent process to reap it. The container accumulates, consuming resources, and its output is never collected. The server marks the task as timed out (via heartbeat reaper), but the container may run for hours.

**Why it happens:**
Graceful shutdown in Go requires explicit signal handling. The default Go behavior on SIGTERM is to terminate immediately. The Docker SDK `Run` method in `docker.go` starts a container and waits on channels — interrupting this mid-flight leaves the container running.

**How to avoid:**
The daemon must:

1. Trap `SIGTERM` / `SIGINT` via `signal.NotifyContext` and cancel the root context.

2. The context cancellation propagates into the Docker `runner.Run(ctx, req)` call, which already handles `ctx.Done()` by calling `cli.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true})` (this path is already implemented in `docker.go`).

3. The daemon must wait for all in-flight task goroutines to complete (via `sync.WaitGroup` or `errgroup.Wait()`) before exiting. The shutdown must have a deadline (e.g., 30s) after which the daemon force-exits and relies on the server's heartbeat reaper to clean up state.

4. Before exiting, report task failures to the server for any tasks that were cancelled due to shutdown.

The key insight: `docker.go` already has the cleanup path on `ctx.Done()`. The daemon just needs to (a) pass a cancellable context and (b) wait for that path to execute before the process exits.

**Warning signs:**
- `docker ps` shows lingering containers after daemon is stopped.
- Daemon exits instantly when killed; no log message about waiting for in-flight tasks.
- No `signal.NotifyContext` or `signal.Notify` call in daemon main.

**Phase to address:**
Phase 2 (Daemon execution) — Implement signal handling before any task execution code.

---

### Pitfall 6: Scatter Fan-Out Overwhelming Daemons — No Global Concurrency Limit

**What goes wrong:**
A WDL scatter over 10,000 items creates 10,000 tasks in `PENDING` state simultaneously. All daemons start claiming tasks as fast as they can poll. Each daemon runs up to its local concurrency limit (currently hardcoded to 8 in the scheduler). With 5 daemons, 40 Docker containers start simultaneously. The host runs out of memory, Docker OOMs containers, and the entire workflow fails.

The current in-process scheduler has `g.SetLimit(8)` as a global limit. In the distributed model, this limit only applies per-daemon and is no longer a system-wide constraint.

**Why it happens:**
The monolithic scheduler controls total parallelism. The distributed model replaces that single limit with N daemon-local limits that are not coordinated. The server creates all scatter tasks at once with no throttling.

**How to avoid:**
The server must implement scatter task emission as a rate-limited or semaphore-controlled process rather than creating all N tasks atomically.

Options (in order of implementation complexity):
1. **Server-side concurrency limit**: The server tracks how many tasks are in `RUNNING` state for a given workflow and only promotes tasks from `QUEUED` to `PENDING` when the running count is below a configured limit. This is the simplest approach and matches how Cromwell handles scatter parallelism.

2. **Per-daemon capacity signaling**: Daemons report their available capacity in the poll response, and the server only marks tasks pending when a daemon has capacity. More complex but more accurate.

For v1.1, option 1 is sufficient. The limit should be configurable per-workflow with a server-wide default.

**Warning signs:**
- All scatter tasks transition to `PENDING` simultaneously regardless of daemon capacity.
- Host memory exhaustion when running large scatter workflows.
- Docker containers being OOM-killed.
- No concept of "max concurrent tasks" in the server configuration.

**Phase to address:**
Phase 2 (Daemon execution + scatter) — Must be designed before scatter task creation is implemented.

---

### Pitfall 7: Error Serialization Loses Detail Across Network Boundary

**What goes wrong:**
The existing engine uses Go's `error` interface and `fmt.Errorf("...%w", err)` chains. `TaskError` carries `TaskName`, `ExitCode`, and `Stderr` as structured fields. When a daemon executes a task and it fails, this error must cross the HTTP boundary from daemon to server, and then from server to client.

If the daemon serializes the error as a plain string (JSON `"error": "task X failed: exit 1"`) the server loses the exit code and stderr. The client cannot distinguish "task failed with exit 127 (command not found)" from "task failed with exit 1 (user error)" from "daemon crashed." Spectool conformance tests may fail because they check for specific error semantics.

**Why it happens:**
Go's `error` interface has no standard JSON serialization. Developers tend to serialize `err.Error()` as a string, losing structure.

**How to avoid:**
Define an explicit `TaskResult` struct for the daemon → server API response:

```go
type TaskResult struct {
    State    string `json:"state"`     // "completed" | "failed" | "error"
    ExitCode int    `json:"exit_code"` // container exit code
    Stderr   string `json:"stderr"`    // last N bytes of stderr
    Error    string `json:"error"`     // daemon-level error message (not task failure)
}
```

The server reconstructs a `TaskError` from this payload and stores it in the database. The client receives structured failure information via the status API.

Define error categories: task failure (container exited non-zero), infrastructure failure (daemon couldn't start Docker), and system failure (daemon crashed / heartbeat timeout). Clients need to distinguish these to provide useful diagnostics.

**Warning signs:**
- Client outputs "task failed" with no exit code or stderr.
- All failure types look identical from the client's perspective.
- Daemon response body contains only `{"error": "some string"}`.

**Phase to address:**
Phase 1 (Server + Daemon API design) — Define the API contract before implementing either side.

---

### Pitfall 8: Shared Filesystem Permissions and File Visibility Timing

**What goes wrong:**
Two subtle problems with NFS or shared directories:

**Problem A — Permissions**: The daemon runs Docker containers as root (or as the image's default user). The container writes output files to `/mnt/outputs` (which is bind-mounted to a shared directory). These files may be created with ownership `root:root` and permissions `0600`, making them unreadable by the server process running as a different user.

**Problem B — NFS metadata caching**: NFS clients cache directory metadata. A daemon writes a file to the shared filesystem, then reports completion to the server. The server immediately tries to read the file for output collection. The server's NFS client has stale cache and reports "file not found." This is the NFS close-to-open coherency window.

**Why it happens:**
Problem A: Docker container UID/GID does not match host service UIDs.
Problem B: NFS attribute cache TTL is typically 3-30 seconds by default. The server queries the filesystem before the cache refreshes.

**How to avoid:**
For Problem A: Configure Docker to run containers with explicit UID/GID matching the service user (`--user $(id -u):$(id -g)`), or use `chmod`/`chown` as a post-execution step in the daemon before reporting completion.

For Problem B: Either (a) the daemon collects and parses outputs locally before reporting completion (pushing output values rather than having the server pull them), or (b) the server retries file existence checks with exponential backoff. Option (a) is architecturally cleaner and matches miniwdl's design where the runner (daemon) collects outputs and returns values.

Document in deployment notes: `noac` NFS mount option disables attribute caching but hurts performance. Prefer the architectural fix (option a) over NFS tuning.

**Warning signs:**
- "Permission denied" when server reads task output files.
- "File not found" immediately after daemon reports task completion, then file appears 5 seconds later.
- Works on single-host (local filesystem) but breaks on NFS.

**Phase to address:**
Phase 2 (Daemon execution) — Output collection design must be decided before implementation.

---

### Pitfall 9: Configuration Drift Between Server and Daemon Versions

**What goes wrong:**
Server v1.1 is deployed. A daemon running v1.0 (from before the distributed refactor) polls the server. The daemon's API client expects a different JSON schema for task records. Alternatively, a new field is added to the server's task assignment response, and old daemons silently ignore it — but that field is required for correct execution (e.g., a new `env_vars` field for task environment variables).

This is a particularly insidious problem because it produces incorrect silent behavior, not a visible error.

**Why it happens:**
In the monolithic model, all code runs in one binary — version mismatch is impossible. Adding separate binaries makes independent deployment possible and therefore version drift possible.

**How to avoid:**
1. The server must include a `version` field in all API responses. Daemons must reject connections to servers with incompatible versions and log a clear error message.

2. Define the API contract as Go structs with `json:"..."` tags shared in a common package (e.g., `pkg/api/types.go`). Both server and daemon import this package. API changes require updating the shared struct and both callers simultaneously.

3. Use explicit versioning in the API path (`/api/v1/tasks/poll`) so future breaking changes can be introduced as `/api/v2/` without breaking existing daemons.

4. Add the daemon's version to every heartbeat and poll request. The server can log version mismatches for visibility.

**Warning signs:**
- Server and daemon compile independently with no shared API type package.
- API types defined inline in each binary rather than in a shared package.
- No version handshake at daemon startup.

**Phase to address:**
Phase 1 (API design) — Shared types package must be established before either binary is implemented.

---

### Pitfall 10: `flowy run` Synchronous Mode — Poll Loop That Never Terminates

**What goes wrong:**
The `flowy run` command must remain synchronous for spectool compatibility: submit a workflow, poll for completion, print outputs. The naive implementation is a tight `for` loop polling the server every N milliseconds. Two failure modes:

**Mode A**: The workflow fails, the server sets workflow state to `FAILED`, but the client's poll loop only checks for `COMPLETED` and loops forever (100% CPU, no output).

**Mode B**: The server becomes temporarily unreachable (restart, network blip). The poll loop returns a connection error, and the client exits with error — abandoning a workflow that is still running on the server.

**Why it happens:**
Mode A: Incomplete terminal state handling.
Mode B: Conflating "server unreachable" (transient) with "workflow failed" (permanent).

**How to avoid:**
The poll loop must:
1. Treat `COMPLETED`, `FAILED`, and `ABORTED` as terminal states that exit the loop.
2. Distinguish connection errors (retry with backoff) from application errors (exit immediately).
3. Have a maximum total timeout with a clear error message ("timed out waiting for workflow; run ID: X").
4. Print the workflow run ID to stderr immediately after submit, so the user can reattach if the client is killed.

For spectool compatibility, the synchronous mode must map workflow terminal states to exit codes matching the original `flowy run` behavior (exit 0 for success, exit 1 for task failure).

**Warning signs:**
- `flowy run` spins at 100% CPU when a workflow fails.
- Client exits when server is restarted mid-workflow.
- No run ID printed before polling begins.
- Poll loop only handles `state == "completed"`.

**Phase to address:**
Phase 3 (Client CLI + spectool compatibility) — Test all terminal state transitions before declaring spectool compatibility.

---

## Technical Debt Patterns

Shortcuts that seem reasonable but create long-term problems.

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Hardcode shared filesystem path in source | Avoids config complexity | Breaks any non-standard deployment; breaks tests on CI without that path | Never — use config from day one |
| Skip heartbeat, rely on task timeout only | Simpler daemon | Stuck tasks take `max_task_duration` to be reaped (could be hours for genomics) | Never — heartbeat is the correct primitive |
| Serialize errors as plain strings | Faster to implement | Clients cannot distinguish failure types; spectool test failures are opaque | Never for task results; acceptable for internal logs |
| Single SQLite connection pool without WAL | Simpler setup | `database is locked` under any concurrent load | Only in single-goroutine server (no concurrent handlers) |
| Store Docker container ID in memory (not DB) | Simpler daemon state | Container cannot be cleaned up after daemon restart | Only in MVP where daemon restart = workflow restart |
| Copy all scatter tasks to PENDING at once | Simpler DAG advancement | OOM on large scatters | Only if scatter size is bounded (< 100 items) |

---

## Integration Gotchas

Common mistakes when connecting the new distributed components to the existing engine code.

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| `task.RunTask` reuse in daemon | Call `RunTask` with daemon's local `execRoot`; paths in task record point to shared FS | Daemon must reconstruct `execRoot` from the task's shared-FS execution directory, not from daemon's local temp |
| PathMap reuse | `PathMap.AddFile` calls `filepath.Abs()` with daemon's CWD — produces daemon-local path | Input files on shared FS are already absolute shared-FS paths; pass them as-is, skip `filepath.Abs()` |
| `workflow.RunWorkflow` reuse in server | Server calls the existing in-process scheduler, bypassing distributed dispatch | Server scheduler must be replaced or wrapped to dispatch to daemon via HTTP rather than calling `runTaskFn` directly |
| `docker.go` DockerRunner reuse | Works unchanged on daemon host — Docker SDK uses `DOCKER_HOST` from env | Correct: DockerRunner is already host-agnostic via env config. Document that `DOCKER_HOST` must be set on daemon host. |
| `outputs.go` output collection | Server calls `CollectOutputs` assuming files are on server's local FS | Daemon must call `CollectOutputs` (files are on daemon host's shared FS mount) and return values via API |
| SQLite open in multiple goroutines | `sql.Open()` called in each handler; multiple connection objects created | Single `*sql.DB` created at server startup; shared across all handlers via dependency injection |
| WDL AST parsing | Server parses WDL once at submit time; daemon re-parses at execution time (wasted work, drift risk) | Server parses and serializes the evaluated task inputs as JSON; daemon receives inputs as data, not WDL source |

---

## Performance Traps

Patterns that work at small scale but fail as scatter width or workflow count grows.

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Polling all PENDING tasks per daemon request | Works fine; at 10K tasks, each poll returns 10K rows | Limit poll response to `min(daemon_capacity, N)` tasks per request | > 1000 pending tasks |
| Loading full workflow state for each status request | Fast for 1 workflow; slow for 100 concurrent | Index tasks by workflow_id; use aggregated counts not full row scans | > 50 concurrent workflows |
| Storing full stderr in task row | Convenient; at 1MB stderr × 1000 tasks, SQLite file grows 1GB | Cap stored stderr at 8KB; write full logs to shared FS file | > 100 tasks with verbose output |
| Scatter tasks all in one SQL INSERT transaction | Fast for 100; at 10K items, single transaction holds write lock for seconds | Batch inserts in chunks of 500 | Scatter width > 500 |
| Heartbeat reaper scans all RUNNING tasks every N seconds | Fine for 10 running tasks; at 1000, full table scan every 60s | Index `state` and `last_heartbeat_at`; reaper scans only the index | > 100 concurrently running tasks |

---

## Security Mistakes

Domain-specific security issues for a multi-host execution system.

| Mistake | Risk | Prevention |
|---------|------|------------|
| No authentication on server REST API | Any process on the network can submit arbitrary WDL workflows and execute Docker containers | Add shared-secret token auth (header: `Authorization: Bearer <token>`) even for v1.1; this is minimal but sufficient for trusted-network HPC |
| Daemon runs Docker with `--privileged` implicitly | Container escape from task code | Never pass `--privileged`; the existing `docker.go` does not, but any new bind mount helpers must not add it |
| Task stderr/stdout logged to shared FS world-readable | Workflow data exposed to other users on shared filesystem | Create per-workflow directories with `0700` permissions; daemon must be the only writer |
| Daemon trusts `task_id` in poll response without validation | Malicious server (or man-in-the-middle) sends daemon a task with crafted file paths that escape shared FS | Validate that all paths in task records are under the configured shared root before executing |
| Configuration file contains shared secret in plaintext | Secret exposed in logs or version control | Use env variable for secret; document clearly; add shared secret to `.gitignore` template |

---

## UX Pitfalls

Common user experience mistakes specific to distributed workflow execution.

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| No run ID printed at submit time | User kills `flowy run`, cannot reattach to running workflow | Print run ID to stderr immediately after submit, before polling starts |
| Opaque "workflow failed" with no task details | User must grep server logs to find which task failed and why | Client status command must show failed task name, exit code, stderr snippet |
| Daemon startup silently succeeds but shared FS is not mounted | Tasks fail with "file not found" hours later | Daemon startup must validate shared FS access and fail fast with clear error |
| Server port already in use gives Go default error | "bind: address already in use" is not actionable | Wrap net.Listen error to say "port 8080 is already in use; is flowy-server already running?" |
| Daemon not running when workflow is submitted | Workflow queued but never executes; user waits forever | Server should warn (not block) when no daemons are registered at submit time |

---

## "Looks Done But Isn't" Checklist

Things that appear complete but are missing critical pieces.

- [ ] **Task claiming**: Endpoint exists and returns task — verify the claim is atomic (single UPDATE WHERE state='PENDING'; check rows-affected == 1)
- [ ] **Heartbeat reaper**: Heartbeat endpoint exists — verify the server background goroutine is actually checking `last_heartbeat_at` and marking timed-out tasks FAILED
- [ ] **Graceful shutdown**: Daemon handles SIGTERM — verify Docker containers are actually stopped (not orphaned) when daemon is killed mid-task
- [ ] **Output collection location**: Daemon "reports completion" — verify it has already collected and serialized output values before reporting (server must not re-read files from shared FS)
- [ ] **`flowy run` spectool mode**: Exits 0 on success — verify exits 1 on task failure, not 0 with empty outputs
- [ ] **Scatter throttling**: Server creates scatter tasks — verify tasks are throttled (not all created PENDING simultaneously for large scatters)
- [ ] **Shared FS path validation**: Config accepts `shared_root` — verify daemon validates it is mounted and writable at startup
- [ ] **Error structure**: Daemon returns task result — verify `exit_code` and `stderr` are present as structured fields, not inside a string message

---

## Recovery Strategies

When pitfalls occur despite prevention, how to recover.

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Tasks stuck RUNNING after daemon crash | LOW (if heartbeat reaper exists) | Wait for reaper timeout; or admin endpoint `POST /admin/tasks/{id}/requeue` |
| Duplicate task execution (double-claim) | MEDIUM | Mark duplicate run FAILED; re-run workflow or resume from last checkpoint (v2 feature) |
| Shared FS full (large scatter outputs) | HIGH | Pause server, move/delete old runs from shared FS, resume; requires manual intervention |
| SQLite database locked under load | MEDIUM | Restart server; WAL mode means no corruption; investigate connection pool size |
| Orphaned Docker containers on daemon host | LOW | `docker rm -f $(docker ps -aq --filter label=flowy-run)` — requires containers to be labeled at creation |
| Config drift causing API mismatch | MEDIUM | Version check at daemon startup catches this before any work runs; upgrade daemon binary |
| NFS metadata cache causing "file not found" | LOW (if daemon collects outputs) | Architectural fix (daemon pushes values); retry with backoff as fallback |

---

## Pitfall-to-Phase Mapping

How roadmap phases should address these pitfalls.

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| File path host assumptions | Phase 1: Shared filesystem design | Integration test: server and daemon run in separate directories; task inputs resolve correctly |
| Task double-claim race | Phase 1: Claim endpoint implementation | Concurrent claim test: 10 goroutines claim same task simultaneously; exactly 1 succeeds |
| SQLite locking under load | Phase 1: Database initialization | Load test: 10 concurrent daemon polls; no `database is locked` errors |
| No heartbeat / orphaned tasks | Phase 1: Heartbeat mechanism | Kill daemon mid-task; verify server marks task FAILED within 3x heartbeat interval |
| Orphaned Docker containers | Phase 2: Daemon graceful shutdown | SIGTERM daemon during task; verify `docker ps` shows container removed |
| Scatter fan-out overwhelm | Phase 2: Scatter dispatch throttling | Submit scatter over 1000 items; verify max concurrent tasks matches configured limit |
| Error serialization loss | Phase 1: API contract definition | Daemon returns task with exit code 42 and stderr; client shows both values |
| Shared FS permissions | Phase 2: Daemon output collection | Container writes root-owned file; server can read it |
| NFS metadata cache | Phase 2: Output collection design | Daemon collects outputs locally and pushes values; server does not read files directly |
| Configuration drift | Phase 1: Shared API types package | Both binaries import same `pkg/api` package; compile error if API diverges |
| `flowy run` poll loop | Phase 3: Client CLI + spectool | Workflow fails mid-run; client exits 1 with task failure details within 5 seconds |

---

## Sources

- [Temporal activity heartbeat and timeout documentation](https://docs.temporal.io/encyclopedia/detecting-activity-failures) — heartbeat pattern for daemon liveness
- [Temporal error handling in distributed systems](https://temporal.io/blog/error-handling-in-distributed-systems) — structured error propagation
- [SQLite WAL mode documentation](https://sqlite.org/wal.html) — concurrency semantics for server use
- [SQLite locking and concurrency v3](https://sqlite.org/lockingv3.html) — write serialization model
- [Abusing SQLite to Handle Concurrency — SkyPilot Blog](https://blog.skypilot.co/abusing-sqlite-to-handle-concurrency/) — practical patterns for Go + SQLite
- [Graceful Shutdown in Go: Practical Patterns — VictoriaMetrics](https://victoriametrics.com/blog/go-graceful-shutdown/) — signal handling for in-flight tasks
- [Implementing Graceful Shutdown in Go — RudderStack](https://www.rudderstack.com/blog/implementing-graceful-shutdown-in-go/) — WaitGroup + context cancel pattern
- [Cromwell backend documentation: file localization](https://cromwell.readthedocs.io/en/stable/backends/HPC/) — how Cromwell handles file staging across hosts
- [Cromwell task localization: symlink limitation](https://cromwell.readthedocs.io/en/develop/optimizations/FileLocalization/) — NFS hard-link pitfalls
- [Backpressure in Distributed Systems — DZone](https://dzone.com/articles/backpressure-in-distributed-systems) — scatter fan-out throttling patterns
- [Building a distributed workflow engine from scratch — DEV Community](https://dev.to/acoh3n/building-a-distributed-workflow-engine-from-scratch-22kl) — polling daemon architecture
- [QCon SF: Database-Backed Workflow Orchestration — InfoQ](https://www.infoq.com/news/2025/11/database-backed-workflow/) — SQLite-backed workflow engine patterns
- flowy codebase: `wdl/task/task.go`, `wdl/task/pathmap.go`, `wdl/task/docker.go`, `wdl/workflow/scheduler.go` — existing in-process execution model

---
*Pitfalls research for: distributed WDL workflow execution (flowy v1.1 server/daemon/client split)*
*Researched: 2026-03-11*
