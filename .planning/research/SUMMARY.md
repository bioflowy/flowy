# Project Research Summary

**Project:** flowy — WDL 1.2 workflow engine (Go), v1.1 distributed server/daemon/client milestone
**Domain:** Distributed bioinformatics workflow orchestration
**Researched:** 2026-03-11
**Confidence:** HIGH

## Executive Summary

flowy v1.1 extends a working v1.0 WDL engine (parser, type checker, evaluator, Docker executor, DAG scheduler, CLI) into a distributed three-process architecture: a persistent REST server (`flowy-server`), one or more polling worker daemons (`flowy-daemon`), and an extended CLI client (`flowy`). Research across Cromwell, Nextflow, Temporal, and direct codebase analysis shows this is a well-understood domain with established patterns. The key design insight is that the v1.0 engine already has the seam for this extension: `wdl/workflow/scheduler.go` exposes a `runTaskFn` package-level variable that can be swapped at server startup to replace in-process Docker execution with SQLite-backed async dispatch to daemons. This means the most complex, most tested engine code — DAG scheduling, scatter, errgroup concurrency — remains completely unchanged.

The recommended approach is a polling-based daemon model backed by SQLite with WAL mode. Daemons poll `GET /internal/tasks/next` (long-poll, 30s timeout), claim tasks atomically via a single `UPDATE ... WHERE state='PENDING'` SQL statement, execute using the existing `task.RunTask()` function, and report completion via `POST /internal/tasks/{id}/complete`. A shared filesystem (NFS or local directory, config-driven) carries WDL files and task inputs/outputs between processes. The stack is deliberately minimal: chi v5 for routing, modernc.org/sqlite (CGO-free) for persistence, goose for migrations, gopkg.in/yaml.v3 for config, and standard library for everything else. No message queue, no JWT, no gRPC.

The primary risks are not architectural but operational: file path assumptions embedded in task records that break on multi-host deployments, task double-claim races without atomic SQL claiming, SQLite locking without WAL mode, and daemon crashes leaving tasks stuck in RUNNING state without a heartbeat mechanism. All four must be addressed in Phase 1 before any execution code is written. A secondary risk is `flowy run` backward compatibility for spectool — the synchronous CLI mode must survive the transition and is non-negotiable for conformance suite operation.

---

## Key Findings

### Recommended Stack

The v1.0 stack (Go 1.25, hand-written recursive descent parser, Docker SDK, cobra CLI, testify, slog, errgroup) is unchanged. For v1.1, five libraries are added. The routing layer uses `chi/v5` (v5.2.5) rather than net/http ServeMux because v1.1 needs middleware grouping for daemon vs. client auth scopes — ServeMux cannot do this without manual wrapper boilerplate. SQLite uses `modernc.org/sqlite` (CGO-free) rather than `mattn/go-sqlite3` because the CI/CD pipeline and cross-compilation targets cannot rely on a C compiler; the 2x INSERT-heavy performance penalty is irrelevant for flowy's actual write volume (task status updates). Schema migrations use `goose/v3` with `go:embed` SQL files because the server is a single binary and external migration tools create deployment synchronization problems. Config parsing uses `gopkg.in/yaml.v3` directly because Viper's 15 transitive dependencies plus known goroutine-safety issues are unjustified for a 5-8 field config struct. Task dispatch uses stdlib `time.Ticker` polling — no Redis, no NATS — because HPC bioinformatics tasks run for minutes to hours and sub-second dispatch latency is irrelevant.

**Core technologies (v1.1 additions):**
- `github.com/go-chi/chi/v5` v5.2.5 — REST API routing; middleware grouping per auth scope; 100% net/http compatible
- `modernc.org/sqlite` v1.36.x — SQLite persistence; CGO-free; cross-compiles without a C toolchain
- `github.com/pressly/goose/v3` v3.27.0 — DB migrations embedded in binary via `go:embed`; Go 1.25+ minimum
- `gopkg.in/yaml.v3` v3.0.1 — config file parsing; zero dependencies; sufficient for flat struct
- stdlib `time.Ticker` + `net/http` client — daemon polling; no external queue infrastructure needed

See `.planning/research/STACK.md` for full rationale and what NOT to use.

### Expected Features

The v1.1 MVP is a complete distributed execution layer. The `flowy run` synchronous mode (spectool compatibility) is the highest-priority non-negotiable feature beyond the core server/daemon/client split. Multi-daemon scatter distribution (different daemons claiming different scatter shards) is a P1 differentiator that requires shard-indexed task rows in the DB schema — this design decision must be made before schema definition.

**Must have (table stakes):**
- REST API: submit / status / outputs / abort / health endpoints — baseline workflow server contract (Cromwell model)
- `flowy-server` binary: DAG orchestration + SQLite state + task dispatch queue
- `flowy-daemon` binary: poll → atomic claim → execute → report, with heartbeat
- SQLite schema: runs + tasks tables with WAL mode and status/run_id indexes
- Atomic task claiming: single `UPDATE WHERE state='PENDING'`, check rows-affected == 1
- Task + run state machines: PENDING → CLAIMED → RUNNING → COMPLETED/FAILED
- Shared filesystem file staging: config-driven base_dir, client inputs, daemon outputs
- Stale task detection: server background reaper goroutine for tasks with expired heartbeat
- `flowy run` synchronous mode: submit → poll → print outputs (spectool-compatible, backward-compatible)
- `flowy submit` / `flowy status` / `flowy outputs` CLI commands

**Should have (competitive):**
- `flowy abort <run-id>` — after basic submit/run/status flow validated
- `GET /workflows?status=Running` list endpoint — operational visibility
- `GET /workflows/{id}/logs` endpoint — task log retrieval
- Daemon auto-registration — diagnostics for multi-daemon deployments

**Defer (v2+):**
- Execution cache / call cache — complex input hashing and invalidation logic
- Workflow resume / checkpoint restart — partial scatter completion is ambiguous
- S3 / GCS storage backend — explicitly out of scope per PROJECT.md
- Authentication (JWT, API keys) — not needed for single-team HPC use in v1.1
- PostgreSQL backend — only needed at 20+ daemons or >100 completions per second

See `.planning/research/FEATURES.md` for the complete feature dependency graph and MVP checklist.

### Architecture Approach

The architecture is a clean three-layer separation: client (HTTP REST consumer), server (DAG orchestrator + task queue + SQLite state), and daemon (stateless worker). The critical insight from direct codebase analysis is that the `runTaskFn` package-level variable in `wdl/workflow/scheduler.go` is already the designed injection point — adding a public `SetRunTaskFn()` setter (one-line change) allows the server to inject a distributed task dispatcher without touching any scheduler logic. The daemon reuses `wdl/task.RunTask()` unchanged. Task dispatch sends `WDLPath + TaskName + InputsJSON` (not a serialized AST) so the daemon re-parses locally from the shared filesystem — this avoids adding JSON tags to all AST node types. All daemons are stateless; authoritative state lives exclusively in the server's SQLite database. The `internal/store` interface is the only boundary that needs to change if a PostgreSQL backend is ever needed.

**Major components:**
1. `internal/api` — shared HTTP DTO types (no business logic); imported by both client and server to prevent type drift; established in Phase 1
2. `internal/store` — SQLite persistence via `database/sql` + goose migrations; pure persistence, no HTTP logic; unit-testable without a running server
3. `internal/server` — HTTP handlers + workflow orchestrator; separated from `cmd/` for `httptest.NewServer` testability
4. `internal/config` — YAML config loading with env var override support
5. `cmd/flowy-server` — thin main() wiring server + store + config; runs DAG scheduler in-process with injected `runTaskFn`
6. `cmd/flowy-daemon` — poll loop + `task.RunTask()` invocation + heartbeat goroutine; stateless worker; CGO-free
7. `cmd/flowy` (extended) — add submit/status/outputs subcommands; make `run` config-aware with distributed/local fallback

See `.planning/research/ARCHITECTURE.md` for full component diagrams, REST API endpoint list, SQLite schema, and shared filesystem layout.

### Critical Pitfalls

1. **File path host assumptions** — Server creates task records with absolute paths. On multi-host deployments, daemon hosts receive paths that do not exist on their filesystems. Prevention: define a config-driven `shared_root` that is the same absolute path on all hosts; never embed server-local `/tmp` in task records; validate shared FS mount at daemon startup. Address in Phase 1 before any task record serialization.

2. **Task double-claim race** — Multiple daemons polling simultaneously both see the same PENDING task and both attempt to claim it. Prevention: implement claiming as a single atomic `UPDATE tasks SET state='CLAIMED' WHERE id=? AND state='PENDING'`; check rows-affected == 1; a SELECT + UPDATE is a TOCTOU race. Address in Phase 1 in the claim endpoint's first commit.

3. **SQLite locking without WAL mode** — The server runs multiple concurrent goroutines (HTTP handler, scheduler, reaper). Without WAL mode, write contention produces `database is locked` errors. Prevention: `PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;` at DB open; `db.SetMaxOpenConns(1)` for the write connection. Address in Phase 1 during store initialization.

4. **Daemon crash leaves tasks RUNNING forever** — Without a heartbeat mechanism, the server cannot distinguish an executing daemon from a crashed one. Prevention: daemon sends `POST /tasks/{id}/heartbeat` every 30s; server background goroutine reaps tasks with `last_heartbeat_at` older than 90s, marking them FAILED. Heartbeat is load-bearing for correctness, not a v1.x addition. Address in Phase 1.

5. **Orphaned Docker containers on daemon SIGTERM** — Daemon exits immediately on SIGTERM; Docker containers continue running on the host unconsumed. Prevention: trap SIGTERM via `signal.NotifyContext` and cancel root context, which propagates into `docker.go`'s existing `ctx.Done()` container cleanup path; wait on `sync.WaitGroup` with 30s deadline before exiting. Address in Phase 2 before any task execution code.

See `.planning/research/PITFALLS.md` for 5 additional critical pitfalls (scatter fan-out throttling, error serialization, NFS metadata caching, configuration drift, `flowy run` poll loop termination) plus performance traps, security mistakes, and recovery strategies.

---

## Implications for Roadmap

Based on combined research, four phases with a clear dependency chain:

### Phase 1: API Contract + Persistence Foundation

**Rationale:** All subsequent phases depend on the shared DTO types and SQLite schema being stable. Four critical pitfalls (path assumptions, double-claim, SQLite WAL, heartbeat schema) cannot be retrofitted — they must be correct from the first commit. The architecture research confirms `internal/api` and `internal/store` have no circular dependencies and can be built first. The DB schema decision for scatter shard indexing must be made here before Phase 2 creates task rows.
**Delivers:** `internal/api` (shared DTO types), `internal/config` (YAML loading), `internal/store` (SQLite schema + CRUD with WAL + goose migrations), `last_heartbeat_at` column in tasks table, shard_index in tasks schema
**Addresses:** REST API response shapes, SQLite schema for runs + tasks, task + run state machine definitions, shared type package preventing API drift
**Avoids:** Type drift between client and server (shared `internal/api`), double-claim race (atomic UPDATE pattern locked in), SQLite locking (WAL configured at store init), NFS metadata cache (daemon-collects-outputs-before-reporting design locked in schema)

### Phase 2: Server Binary + DAG Dispatch

**Rationale:** `flowy-server` is the core of the distributed system; it must work before the daemon is useful. The server's orchestration logic (injecting `SetRunTaskFn` into the scheduler) is the most architecturally novel piece and should be validated early. Scatter fan-out throttling (Pitfall 6) must be designed here before scatter task creation is implemented.
**Delivers:** `cmd/flowy-server` with full REST API, workflow submission + DAG scheduling with distributed `runTaskFn`, task queue with atomic claiming, heartbeat reaper goroutine, run concurrency semaphore (max concurrent workflows configurable)
**Uses:** chi v5, modernc.org/sqlite, goose, `wdl/workflow` (via SetRunTaskFn), `wdl/parser`, `wdl/checker`, `internal/store`, `internal/api`
**Implements:** `wdl/workflow.SetRunTaskFn()` addition (1-line change to existing package); server HTTP handler layer + orchestrator
**Avoids:** Per-run uncontrolled goroutines exhausting memory (semaphore-gated), scatter fan-out overwhelming daemons (server-side task promotion throttling)

### Phase 3: Daemon Binary + Task Execution

**Rationale:** Daemon depends on a working server API (Phase 2). Daemon is architecturally simpler but operationally critical. Graceful shutdown (SIGTERM → context cancel → Docker cleanup → WaitGroup drain) must be implemented before any task execution code to avoid orphaned containers on any daemon kill.
**Delivers:** `cmd/flowy-daemon` with poll loop, atomic task claiming, `task.RunTask()` invocation, local output collection + value serialization before reporting completion, heartbeat goroutine, SIGTERM graceful shutdown with 30s deadline
**Uses:** `wdl/task.RunTask()` (unchanged interface), `internal/api`, `internal/config`, stdlib `signal.NotifyContext`, `sync.WaitGroup`
**Implements:** Daemon layer; output collection design (daemon pushes values via API, server does not read files directly — prevents NFS metadata cache issue)
**Avoids:** Orphaned Docker containers (graceful shutdown), shared FS permissions issues (daemon-side collection before reporting), NFS metadata cache (daemon collects and serializes outputs locally)

### Phase 4: Client CLI + Spectool Compatibility

**Rationale:** Client depends on working server + daemon (Phases 2-3). `flowy run` synchronous mode is the highest-risk feature for regression — spectool conformance tests call it directly and expect specific exit codes and output formats. All terminal state transitions (COMPLETED, FAILED, ABORTED) must be verified before declaring spectool compatibility.
**Delivers:** Extended `cmd/flowy` with `submit`/`status`/`outputs` subcommands; config-aware `flowy run` (distributed mode if server configured, local fallback otherwise); correct exit code mapping; run ID printed to stderr before polling; structured error output with task name + exit code + stderr snippet
**Uses:** `internal/api`, `internal/config`, existing cobra CLI infrastructure
**Implements:** `flowy run` poll loop with all terminal states, connection-error retry backoff, max timeout, backward-compatible local fallback
**Avoids:** Poll loop that never terminates on FAILED state (all terminal states handled), spectool regression (local fallback preserves v1.0 behavior when no server configured)

### Phase Ordering Rationale

- **Phase 1 before everything:** The DTO types and SQLite schema are imported by all three binaries. Schema decisions (shard_index for scatter, heartbeat column) cannot be changed without migration overhead once Phases 2 and 3 are built.
- **Server before daemon:** The daemon's poll API must exist before the daemon can function. Server-first also enables testing the claim atomicity logic independently before adding the execution layer.
- **Daemon before client polish:** The end-to-end path (submit → execute → complete → outputs) must work before the client's synchronous mode can be validated against real executions.
- **Spectool validation last:** Spectool runs the full pipeline. It is the integration test for all four phases combined, not a mid-development checkpoint.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 2 (Server + DAG Dispatch):** The long-poll implementation (server holds open HTTP connection, notifies via channel or `sync.Cond` when task available) has multiple valid approaches; the choice affects server goroutine count under load. Recommend a brief research pass on Go long-poll patterns with `http.Flusher` vs. short-interval timeout polling before committing to an implementation.
- **Phase 3 (Daemon output collection):** Output collection via `wdl/task` internals — specifically `outputs.go` and `CollectOutputs` — needs a code walkthrough to confirm the daemon can invoke it correctly with shared-FS absolute paths without triggering the PathMap `filepath.Abs()` pitfall documented in PITFALLS.md.

Phases with standard patterns (skip research-phase):
- **Phase 1 (API Contract + Persistence):** SQLite WAL + goose migrations + YAML config are fully documented, stable patterns. All library choices verified. No novel decisions.
- **Phase 4 (Client CLI):** Cobra subcommands and HTTP poll loop are standard Go patterns. The spectool interface is defined by existing conformance tests.

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All library choices verified via pkg.go.dev, GitHub releases, official docs. Version compatibility confirmed against go.mod (Go 1.25). Alternatives compared and rejected with documented rationale. |
| Features | HIGH | Core features derived from direct Cromwell + Nextflow API analysis and live codebase inspection. Feature dependencies mapped with implementation code references (e.g., `scheduler.go` line 18 for `runTaskFn`). |
| Architecture | HIGH | Based on direct codebase analysis of `scheduler.go`, `task.RunTask`, `dag.go`, `pathmap.go`, `docker.go`. The `runTaskFn` injection point confirmed present. GA4GH TES pattern validated. |
| Pitfalls | HIGH | 10 pitfalls verified against Temporal, Cromwell, SQLite documentation, and Go distributed systems literature. Direct code paths in `pathmap.go` and `docker.go` inspected for pitfalls 1 and 5. |

**Overall confidence:** HIGH

### Gaps to Address

- **Long-poll vs. interval-poll production tradeoff:** Research recommends long-poll (30s timeout) but documents interval polling (5s) as a simpler fallback. The implementation choice should be validated once Phase 2 server is runnable — if server goroutine count under simultaneous daemon connections is higher than expected, fall back to interval polling with no other changes needed.
- **Output collection code path:** The exact call sequence from daemon to `wdl/task` output collection (`outputs.go`) needs a walkthrough during Phase 3 planning to confirm shared-FS absolute paths thread through correctly without triggering `filepath.Abs()` in `pathmap.go`.
- **`flowy run` local-vs-distributed dispatch edge case:** The config-detection logic (`if config.ServerURL != ""`) is simple but needs explicit test coverage for the case where a config file exists but the server is unreachable — should it fall back to local execution or fail fast with a clear error?

---

## Sources

### Primary (HIGH confidence)
- flowy codebase: `wdl/workflow/scheduler.go`, `wdl/task/task.go`, `wdl/task/pathmap.go`, `wdl/task/docker.go`, `cmd/flowy/main.go` — direct inspection
- [go-chi/chi releases](https://github.com/go-chi/chi/releases) — v5.2.5 confirmed latest stable (February 2026)
- [modernc.org/sqlite — pkg.go.dev](https://pkg.go.dev/modernc.org/sqlite) — CGO-free, 2562+ importers, SQLite 3.51.2
- [goose releases](https://github.com/pressly/goose/releases) — v3.27.0, Go 1.25+ minimum confirmed
- [SQLite WAL mode documentation](https://sqlite.org/wal.html) — concurrency semantics
- [SQLite locking v3](https://sqlite.org/lockingv3.html) — write serialization model
- [Temporal activity heartbeat documentation](https://docs.temporal.io/encyclopedia/detecting-activity-failures) — heartbeat pattern for daemon liveness
- [SQLite atomic task claim — jasongorman.uk](https://jasongorman.uk/writing/sqlite-background-job-system/) — UPDATE WHERE pattern with rows-affected check
- [Building a Durable Execution Engine with SQLite — morling.dev](https://www.morling.dev/blog/building-durable-execution-engine-with-sqlite/) — PENDING/WAITING/COMPLETE state model
- [Cromwell Tools API Documentation](https://cromwell-tools.readthedocs.io/en/latest/API/) — endpoint list and workflow states
- [GA4GH Task Execution Service OpenAPI spec](https://github.com/ga4gh/task-execution-schemas) — REST naming conventions

### Secondary (MEDIUM confidence)
- [Nextflow Task Processing — deepwiki.com](https://deepwiki.com/nextflow-io/nextflow/3.3-task-processing-and-execution) — task state machine analysis (third-party source)
- [Cromwell architecture reference](https://cromwell.readthedocs.io/en/stable/) — polling interval patterns (different tech stack, same domain)
- [miniwdl-cloud](https://github.com/mlin/miniwdl-cloud) — shared filesystem approach for WDL distribution
- [Abusing SQLite to Handle Concurrency — SkyPilot Blog](https://blog.skypilot.co/abusing-sqlite-to-handle-concurrency/) — practical Go + SQLite patterns
- [Graceful Shutdown in Go — VictoriaMetrics](https://victoriametrics.com/blog/go-graceful-shutdown/) — signal handling for in-flight tasks

### Tertiary (supporting/cross-reference)
- [goqite — maragudk/goqite](https://github.com/maragudk/goqite) — confirms SQLite-backed task queue is viable Go pattern
- [Backpressure in Distributed Systems — DZone](https://dzone.com/articles/backpressure-in-distributed-systems) — scatter fan-out throttling patterns
- [Go ServeMux vs Chi — calhoun.io](https://www.calhoun.io/go-servemux-vs-chi/) — middleware grouping gap analysis

---
*Research completed: 2026-03-11*
*Ready for roadmap: yes*
