# Feature Research

**Domain:** Distributed WDL workflow execution — server/daemon/client architecture
**Researched:** 2026-03-11
**Confidence:** HIGH (core patterns), MEDIUM (implementation details)
**Scope:** v1.1 milestone only. v1.0 engine features (parser, type checker, evaluator, Docker executor, DAG scheduler, stdlib, CLI) are already built.

---

## Feature Landscape

### Table Stakes (Users Expect These)

Features that any distributed workflow execution system must have. Missing any one makes the system incomplete or unusable for its intended purpose.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| REST API: POST /workflows — submit workflow | Standard entry point in every workflow server (Cromwell, Nextflow API). Returns run ID for subsequent operations. | MEDIUM | Accepts WDL file path (shared FS) + inputs JSON. Returns `{run_id: "uuid"}`. |
| REST API: GET /workflows/{id}/status — poll run state | Required for async submission model. Clients must be able to check whether a workflow is still running. | LOW | Returns `{status: "Running|Succeeded|Failed|Aborted"}`. Cromwell uses the same 6-state model. |
| REST API: GET /workflows/{id}/outputs — fetch results | Run is useless without retrieving outputs. Must return structured paths after completion. | LOW | Returns `{outputs: {"task.output": "path"}}` matching WDL output variable names. |
| REST API: GET /workflows/{id}/logs — fetch task logs | Essential for debugging failed tasks. Users need stdout/stderr per task call. | MEDIUM | Returns per-call-attempt stdout/stderr paths or inline content. |
| REST API: POST /workflows/{id}/abort — cancel a run | Users need to stop runaway or mistaken workflows. | LOW | Transitions state to Aborting → Aborted; signals daemon to stop dispatching new tasks. |
| REST API: GET /health — liveness probe | Required for operations: know if server is up before submitting. | LOW | Returns `{status: "ok"}` with 200. No auth needed. |
| Daemon: poll server for ready tasks | Core of the worker model. Daemon continuously polls `GET /tasks/next` and claims an available task. | MEDIUM | Uses SQLite `BEGIN IMMEDIATE` to prevent double-claim. Poll interval configurable (default 5s). |
| Daemon: report task result to server | Daemon must notify server when a task completes (exit code, output paths, stdout/stderr). | MEDIUM | `POST /tasks/{id}/complete` or `PATCH /tasks/{id}` with exit code + outputs map. |
| Daemon: execute claimed task via existing Docker engine | Daemon reuses `wdl/task` package directly. No re-implementation of Docker logic. | LOW | Dependency on existing `task.RunTask`. |
| Daemon: heartbeat to prevent task timeout | Without heartbeat, server cannot detect crashed daemon; task would stay Running forever. | MEDIUM | `POST /tasks/{id}/heartbeat` every N seconds; server marks tasks stale if no heartbeat for configurable timeout (default 60s). |
| SQLite persistence: workflow run table | Stores run_id, status, WDL path, inputs JSON, submitted_at, completed_at. | LOW | WAL mode required for concurrent reader (daemon) + writer (server). |
| SQLite persistence: task table | Stores task_id, run_id, call_name, shard_index, status, daemon_id, attempt, stdout_path, stderr_path, outputs_json. | MEDIUM | Status enum: PENDING, CLAIMED, RUNNING, COMPLETED, FAILED. |
| Shared filesystem: config-driven base directory | Server and all daemons must see the same path. Config key `storage.base_dir`. | LOW | NFS/local share. Server writes input files; daemon reads them; daemon writes outputs; server reads them. |
| Shared filesystem: input staging by client | Client uploads WDL inputs (File values) to shared dir before submitting run. Server references those paths in task records. | MEDIUM | Client `flowy submit` copies local files to `{base_dir}/runs/{run_id}/inputs/`. |
| Shared filesystem: output collection by daemon | Daemon writes task outputs to `{base_dir}/runs/{run_id}/tasks/{task_id}/outputs/`. Server reads these paths to build the workflow outputs response. | MEDIUM | Consistent path convention required across server/daemon. |
| Client CLI: `flowy submit <wdl> [inputs]` | Users need to submit workflows from the command line. | LOW | Stages input files, calls POST /workflows, prints run_id. |
| Client CLI: `flowy status <run-id>` | Users need to check status of a submitted run. | LOW | Calls GET /workflows/{id}/status, prints status. |
| Client CLI: `flowy outputs <run-id>` | Users need to retrieve output paths after completion. | LOW | Calls GET /workflows/{id}/outputs, prints JSON. |
| Client CLI: `flowy run <wdl> [inputs]` (synchronous mode) | Required for spectool compatibility. `flowy run` must continue to work end-to-end. In distributed mode: submit → poll until terminal → print outputs. | HIGH | Spectool calls `flowy run` and expects outputs.json at completion. Must be backward-compatible with v1.0 behavior. |
| Server binary: `flowy-server` | Separate process from client; runs persistently. Accepts REST requests, manages DAG scheduling, dispatches tasks to daemon queue. | HIGH | New binary at `cmd/flowy-server/`. |
| Daemon binary: `flowy-daemon` | Separate process per worker node. Polls server, executes Docker tasks. Horizontally scalable. | HIGH | New binary at `cmd/flowy-daemon/`. |
| Run state machine: Submitted → Running → Succeeded/Failed/Aborted | Standard workflow lifecycle. Server transitions states as tasks complete. | MEDIUM | Run enters Running when first task starts; Succeeded when all tasks complete; Failed when any task fails and no retries remain. |
| Task state machine: PENDING → CLAIMED → RUNNING → COMPLETED/FAILED | Standard task lifecycle for polling-based worker systems. | MEDIUM | CLAIMED state prevents double-dispatch. Daemon sets to RUNNING after Docker start. |

### Differentiators (Competitive Advantage)

Features that add value beyond the baseline distributed execution model.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| `flowy run` backward compatibility (spectool mode) | Spectool runs `flowy run` directly. If distributed mode is transparent, the conformance suite keeps working without changes. | HIGH | Most WDL engines break CLI compatibility when adding server mode. This is a strong differentiator for CI/CD integration. |
| Multi-daemon scatter distribution | Scatter shards are independent tasks; multiple daemons can claim different shards, achieving real parallelism across machines. | HIGH | Requires task table to store shard_index per task record. Daemon claims any PENDING task for the current run. |
| Single config file for all three binaries | `flowy.toml` or `flowy.yaml` with `[server]`, `[daemon]`, `[client]` sections. Each binary reads its section. No per-binary config management. | LOW | Reduces operational friction vs. Cromwell's complex HOCON configuration. |
| Daemon auto-registration with server | Daemon registers its host/ID on startup; server tracks active daemons. Useful for diagnostics and detecting stale daemons. | LOW | `POST /daemons/register` on startup; `DELETE /daemons/{id}` on shutdown. Low value in MVP but low cost. |
| SQLite-only persistence (no external DB) | Cromwell requires MySQL for production; flowy uses SQLite with WAL mode. Zero external dependencies for the server. | LOW | Appropriate for single-server deployments. If multi-server is needed, defer to v2 with PostgreSQL. |
| REST API: GET /workflows?status=Running — list/query runs | Operational visibility: see all running/completed runs without knowing their IDs. | LOW | Simple SQL query with status filter. Cromwell has this; it's expected in production use. |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Push-based task dispatch (server pushes to daemon) | Seems more efficient; no polling delay | Requires daemon to be reachable from server (reverse connectivity); NAT/firewall issues common in HPC clusters; much more complex server-side fan-out logic | Polling model: daemon only needs outbound access to server. Interval 2-5s is acceptable for batch workloads. |
| WebSocket / Server-Sent Events for status | Real-time status updates in web UIs | First milestone has no web UI; adds dependencies (gorilla/websocket) and complexity; polling at 2s intervals is sufficient for batch workflows | Client polls GET /workflows/{id}/status in synchronous `flowy run` mode |
| JWT / OAuth2 authentication | Secure multi-user access | Auth infrastructure is a project in itself; adds third-party dependencies; HPC clusters typically use network-level security | Run on trusted network (HPC), no auth in v1. Add basic auth or token header in v1.x if needed. |
| Execution cache / call cache | Avoid re-running unchanged tasks (Cromwell call cache) | Requires stable hashing of inputs + WDL source + Docker image digest; complex invalidation logic; storage management | Not in v1.1. Document as v2 feature. |
| Workflow resume / checkpoint restart | Re-start a failed workflow from last completed task | Requires careful state reconstruction; scatter partial completions are ambiguous | Not in v1.1. Full re-run is acceptable for initial distributed mode. |
| S3 / GCS object storage as file backend | Cloud-native file staging | Requires signed URL handling, multipart upload, download caching; large surface area | Shared filesystem (NFS) first. S3 is explicitly deferred (see PROJECT.md). |
| gRPC API instead of REST | Performance, type-safety, streaming | Not standard in bioinformatics tooling; spectool and community tools expect REST; adds code generation step | REST with JSON. Standard in the domain. |
| Kubernetes / container orchestration integration | Cloud-native deployment | Enormous scope increase; out of v1.1 target | Docker daemon per node is sufficient for HPC environments. |
| Distributed SQLite (Litestream, Turso) | SQLite replication across nodes | Only the server binary writes to SQLite; daemons talk to server via REST, never SQLite directly. Replication adds operational complexity for no benefit in single-server setup | Single-server SQLite is sufficient. |

---

## Feature Dependencies

```
[Client CLI: flowy submit / status / outputs / run]
    └──requires──> [REST API: submit / status / outputs endpoints]
                       └──requires──> [Server: flowy-server binary]
                                          └──requires──> [SQLite: runs + tasks tables]
                                          └──requires──> [DAG Scheduler (existing, wdl/workflow)]

[Daemon: flowy-daemon binary]
    └──requires──> [REST API: GET /tasks/next + POST /tasks/{id}/complete]
    └──requires──> [Docker task executor (existing, wdl/task)]
    └──requires──> [Shared filesystem: task input/output directories]

[Multi-daemon scatter distribution]
    └──requires──> [Task table: shard_index per task record]
    └──requires──> [Daemon: claim any PENDING task (not just own run's tasks)]

[flowy run (synchronous/spectool mode)]
    └──requires──> [REST API: submit + status + outputs]
    └──requires──> [Client: poll-until-terminal loop]

[Shared filesystem file staging]
    └──requires──> [Server: base_dir config]
    └──requires──> [Daemon: writes outputs to convention path]
    └──requires──> [Client: copies input File values to shared dir before submit]

[Daemon heartbeat]
    └──requires──> [SQLite: last_heartbeat_at column in tasks table]
    └──requires──> [Server: background goroutine to detect stale tasks → reset to PENDING]

[Task state machine: PENDING → CLAIMED → RUNNING → COMPLETED/FAILED]
    └──enhances──> [Daemon polling (prevents double-claim via atomic CLAIMED transition)]
    └──enhances──> [Run state machine (server computes run status from task states)]

[Run state machine: Submitted → Running → Succeeded/Failed/Aborted]
    └──requires──> [All tasks in run have reached terminal state (COMPLETED or FAILED)]
```

### Dependency Notes

- **`flowy run` synchronous mode requires all REST endpoints**: The synchronous mode is a thin loop over submit + poll status + fetch outputs. All three server endpoints must work before this can be implemented.
- **Multi-daemon scatter requires task-level granularity in DB**: The server must create one task record per scatter shard, not one record per scatter block. This decision affects the DB schema and must be made in Phase 1 of implementation.
- **Daemon heartbeat requires server-side background goroutine**: A goroutine in flowy-server periodically queries for tasks with last_heartbeat_at older than timeout and resets them to PENDING. Must be part of server startup, not a separate process.
- **Shared filesystem path convention must be consistent**: Server, daemon, and client must agree on the path layout for run directories. Define once in a shared `storage` package; import from all three binaries. Any drift causes silent data loss.
- **Existing `wdl/workflow` scheduler is reused unchanged in server**: The server runs the DAG scheduler in-process, but instead of calling `task.RunTask` directly, it dispatches tasks to the SQLite queue and waits for daemon completions. The `runTaskFn` indirection already exists in `scheduler.go` (line 18) — this is the exact seam to exploit.

---

## MVP Definition

### Launch With (v1.1)

Minimum viable set for the distributed architecture milestone.

- [ ] `flowy-server` binary: REST API (submit/status/outputs/abort/health), SQLite state, DAG dispatch to task queue — *core value delivery*
- [ ] `flowy-daemon` binary: poll for tasks, claim atomically, run via Docker, report result, heartbeat — *enables distributed execution*
- [ ] Shared filesystem: config-driven base_dir, client input staging, daemon output collection — *file passing between components*
- [ ] SQLite schema: runs + tasks tables, WAL mode, proper indexes on (status, run_id) — *durable state*
- [ ] Task state machine: PENDING → CLAIMED → RUNNING → COMPLETED/FAILED with atomic claim — *prevents double-execution*
- [ ] Run state machine: Submitted → Running → Succeeded/Failed — *visible lifecycle*
- [ ] Client `flowy submit` / `flowy status` / `flowy outputs` commands — *user-facing operations*
- [ ] `flowy run` synchronous mode: submit → poll → print outputs (spectool compatibility) — *backward compatibility, non-negotiable*
- [ ] Stale task detection: server resets CLAIMED/RUNNING tasks with expired heartbeat to PENDING — *resilience against daemon crashes*

### Add After Validation (v1.x)

Features to add once core distributed execution is working.

- [ ] `flowy abort <run-id>` — add after basic submit/run/status flow is stable
- [ ] `GET /workflows?status=Running` list endpoint — once basic operations work, operational visibility becomes useful
- [ ] Daemon auto-registration (`POST /daemons/register`) — useful for diagnostics once multi-daemon deployments are common
- [ ] `GET /workflows/{id}/logs` — task log retrieval endpoint; defer until after core status/outputs work

### Future Consideration (v2+)

- [ ] Execution cache / call cache — complex hashing, invalidation; defer until v1.1 is stable
- [ ] Workflow resume from checkpoint — requires careful partial-state reconstruction
- [ ] S3 / GCS storage backend — explicitly out of scope per PROJECT.md
- [ ] Authentication (JWT, API keys) — needed for multi-tenant deployment; not for single-team HPC use
- [ ] PostgreSQL backend — if multi-server coordination is needed
- [ ] Web UI for workflow monitoring

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| REST API: submit + status + outputs | HIGH | MEDIUM | P1 |
| flowy-server binary | HIGH | HIGH | P1 |
| flowy-daemon binary | HIGH | HIGH | P1 |
| SQLite schema: runs + tasks | HIGH | LOW | P1 |
| Task state machine + atomic claim | HIGH | MEDIUM | P1 |
| Shared filesystem file staging | HIGH | MEDIUM | P1 |
| flowy run synchronous mode (spectool) | HIGH | MEDIUM | P1 |
| Stale task / heartbeat recovery | MEDIUM | MEDIUM | P1 |
| flowy submit / status / outputs CLI | MEDIUM | LOW | P1 |
| REST API: abort | MEDIUM | LOW | P2 |
| REST API: GET /workflows list | LOW | LOW | P2 |
| REST API: GET /workflows/{id}/logs | MEDIUM | LOW | P2 |
| Daemon auto-registration | LOW | LOW | P3 |
| Call cache / execution cache | HIGH | HIGH | P3 |
| S3 storage backend | MEDIUM | HIGH | P3 |

**Priority key:**
- P1: Must have for v1.1 launch
- P2: Should have, add in v1.x polish
- P3: Nice to have, future consideration

---

## Competitor Feature Analysis

How Cromwell, Nextflow, and Snakemake handle the same features:

| Feature | Cromwell | Nextflow | Our Approach |
|---------|----------|----------|--------------|
| Workflow submission | POST /api/workflows/v1 (multipart: WDL file + inputs JSON + options) | `nextflow run <pipeline>` or NF-Tower API | POST /workflows with shared-FS WDL path + inputs JSON body. Simpler than Cromwell multipart. |
| Run ID / handle | UUID returned at submit | Session hash | UUID v4. Standard and easy to pass around. |
| Status endpoint | GET /api/workflows/v1/{id}/status → `{status: "Running"}` | Polled via NF-Tower or log file | GET /workflows/{id}/status → `{status: "Running"}`. Same model as Cromwell. |
| Workflow states | Submitted, Running, Aborting, Aborted, Failed, Succeeded | queued, running, succeeded, failed, cancelled | Same 6 states as Cromwell. Industry standard. |
| Worker model | Backend plugins (JES, PAPIv2, TES, local) polled from Cromwell | Executors (local, SLURM, Kubernetes) | Polling daemon model. Simpler than Cromwell backends. Daemon only needs outbound HTTP to server. |
| Task claiming | Internal actor system (Akka) — not exposed | Internal channel/actor | SQLite `BEGIN IMMEDIATE` transaction. Go-native, no external queuing system. |
| File staging | Options: `final_workflow_outputs_dir`; GCS/S3 for cloud backends | `stageInMode` (symlink/copy/link), `workDir` | Config-driven `base_dir` on shared FS. Client stages inputs explicitly; daemon writes outputs to convention path. |
| Persistence | MySQL (production) or in-memory (dev) | In-process only (no server persistence in open-source) | SQLite with WAL mode. Zero external dependencies. |
| Scatter distribution | Jobs dispatched to cloud batch or HPC scheduler | DataFlow queue (internal) | Multiple daemons claim different shard tasks from SQLite queue. |
| Synchronous mode | `cromwell run` (run mode, single JVM) | `nextflow run` (blocking) | `flowy run` submits to server + polls until terminal. Spectool-compatible. |
| Config format | HOCON (complex, JVM-specific) | Groovy DSL or YAML (`nextflow.config`) | TOML or YAML, single file, minimal sections. |
| Binary separation | Single JAR (server+engine) | Single JAR / script | Three separate Go binaries: flowy, flowy-server, flowy-daemon. Clean process model. |

---

## Engine Reuse Map

The v1.0 engine components that the distributed architecture reuses vs. replaces:

| v1.0 Component | Package | v1.1 Reuse Pattern |
|----------------|---------|-------------------|
| Parser | `wdl/parser` | Server calls parser to validate WDL on submit |
| Type checker | `wdl/checker` | Server type-checks WDL on submit (fail fast) |
| DAG scheduler | `wdl/workflow` | Server runs scheduler in-process; replaces `runTaskFn` with SQLite dispatch |
| Docker executor | `wdl/task` | Daemon calls `task.RunTask` directly |
| Expression evaluator | `wdl/eval` | Unchanged; needed by both server (output binding) and daemon (command expansion) |
| `flowy run` CLI logic | `cmd/flowy` | Refactored: add `--server` flag for distributed mode; defaults to local mode if no server configured |

The `runTaskFn` variable in `wdl/workflow/scheduler.go` (line 18) is the exact injection point for replacing local Docker execution with server-side task dispatch. This is the architectural seam that makes the v1.1 refactor clean.

---

## Sources

- [Cromwell Server Mode — cromwell.readthedocs.io](https://cromwell.readthedocs.io/en/stable/tutorials/ServerMode/) — REST API structure, workflow states (MEDIUM confidence — docs don't specify every endpoint format)
- [Cromwell Tools API Documentation](https://cromwell-tools.readthedocs.io/en/latest/API/) — full endpoint list: submit, status, metadata, query, abort, health (HIGH confidence)
- [Nextflow Task Processing and Execution — deepwiki.com](https://deepwiki.com/nextflow-io/nextflow/3.3-task-processing-and-execution) — TaskRun lifecycle, BashWrapperBuilder, task state machine (MEDIUM confidence — third-party analysis of NF source)
- [SQLite Background Job System — jasongorman.uk](https://jasongorman.uk/writing/sqlite-background-job-system/) — atomic task claim with `BEGIN IMMEDIATE`, WAL pragmas, poll interval patterns (HIGH confidence — primary source with code examples)
- [Building a Durable Execution Engine with SQLite — morling.dev](https://www.morling.dev/blog/building-durable-execution-engine-with-sqlite/) — PENDING/WAITING/COMPLETE state model, idempotency requirements (HIGH confidence)
- [goqite — github.com/maragudk/goqite](https://github.com/maragudk/goqite) — Go SQLite-backed queue library, SQS-inspired (MEDIUM confidence)
- [Temporal Workflow Engine Principles](https://temporal.io/blog/workflow-engine-principles) — task queue decoupling from workflow logic, worker polling patterns (HIGH confidence)
- [Nextflow Working with Files](https://www.nextflow.io/docs/latest/working-with-files.html) — file staging into work directory, stageInMode patterns (HIGH confidence)
- [AWS Prescriptive Guidance: Parallelization and scatter-gather patterns](https://docs.aws.amazon.com/prescriptive-guidance/latest/agentic-ai-patterns/parallelization-and-scatter-gather-patterns.html) — scatter-gather design pattern (MEDIUM confidence)
- flowy source `wdl/workflow/scheduler.go` — `runTaskFn` injection point, existing errgroup concurrency model (HIGH confidence — direct code inspection)
- flowy source `go.mod` — current dependencies: Go 1.25, Docker SDK, no HTTP router or SQLite yet (HIGH confidence)

---

*Feature research for: flowy v1.1 distributed WDL workflow execution*
*Researched: 2026-03-11*
