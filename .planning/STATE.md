---
gsd_state_version: 1.0
milestone: v1.1
milestone_name: Distributed Execution
status: planning
stopped_at: Completed 10-server-binary-dag-dispatch/10-02-PLAN.md
last_updated: "2026-03-11T23:50:00.147Z"
last_activity: 2026-03-11 — Roadmap created for v1.1 (4 phases, 25/25 requirements mapped)
progress:
  total_phases: 4
  completed_phases: 2
  total_plans: 4
  completed_plans: 4
  percent: 0
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-11)

**Core value:** WDL 1.2で記述されたワークフローをDockerコンテナ上で正確に実行できること
**Current focus:** Phase 9 — API Contract + Persistence Foundation

## Current Position

Phase: 9 of 12 (API Contract + Persistence Foundation)
Plan: — (not yet planned)
Status: Ready to plan
Last activity: 2026-03-11 — Roadmap created for v1.1 (4 phases, 25/25 requirements mapped)

Progress: [░░░░░░░░░░] 0%

## Performance Metrics

**Velocity:**
- Total plans completed: 0 (v1.1)
- Average duration: — min
- Total execution time: — hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| — | — | — | — |

*Updated after each plan completion*
| Phase 09-api-contract-persistence-foundation P01 | 2 | 2 tasks | 8 files |
| Phase 09-api-contract-persistence-foundation P02 | 7 | 2 tasks | 6 files |
| Phase 10-server-binary-dag-dispatch P01 | 4 | 2 tasks | 7 files |
| Phase 10-server-binary-dag-dispatch P02 | 8 | 3 tasks | 5 files |

## Accumulated Context

### Decisions

- Separate binaries: flowy (client), flowy-server, flowy-daemon
- Daemon polls server for tasks (not server-push); long-poll (30s timeout) preferred over interval polling
- Shared filesystem (NFS) first, S3 deferred to v2+
- `flowy run` maintains synchronous behavior for spectool compatibility; server is required (no local fallback)
- SQLite with WAL mode + `modernc.org/sqlite` (CGO-free); chi v5 for routing; goose for migrations
- Atomic task claiming via single `UPDATE WHERE state='PENDING'` — SELECT+UPDATE is a TOCTOU race
- `runTaskFn` injection point in `wdl/workflow/scheduler.go` is the designed extension seam for distributed dispatch
- Tasks table must include `last_heartbeat_at` and `shard_index` columns from Phase 9 — cannot retrofit later
- [Phase 09-01]: DefaultDSN() returns DBDSN unchanged if explicitly set, otherwise constructs SQLite WAL DSN pointing to {base_dir}/flowy.db
- [Phase 09-01]: internal/api is a strict leaf package — only stdlib imports allowed, verified by go vet
- [Phase 09-01]: CreateRunDirs uses os.MkdirAll (not os.Mkdir) to ensure idempotency
- [Phase 09-02]: ClaimTask uses RETURNING with all columns in single UPDATE — avoids deadlock from opening reader while writer rows cursor is still open under single-connection writer pool
- [Phase 09-02]: OpenMemory() uses file::memory:?cache=shared DSN so writer and reader see the same in-memory database
- [Phase 09-02]: FindStaleTasks uses string modifier ('-N seconds') for SQLite datetime() — not integer
- [Phase 10-server-binary-dag-dispatch]: UpdateHeartbeatByDaemon uses single UPDATE WHERE daemon_id=? AND state IN CLAIMED/RUNNING — one call per heartbeat interval regardless of task count
- [Phase 10-server-binary-dag-dispatch]: RunWorkflowWithDispatch injects TaskDispatchFn into scheduler.taskRunner; nil falls back to package-level runTaskFn for backward compat
- [Phase 10-server-binary-dag-dispatch]: TaskDir format: {baseDir}/runs/{runID}/tasks/{taskName}_{shardIndex}
- [Phase 10-server-binary-dag-dispatch]: handleSubmit uses context.WithCancel(context.Background()) for run goroutines — ensures goroutine outlives HTTP request
- [Phase 10-server-binary-dag-dispatch]: SetStaleInterval() exported on Server for test-controllable ticker interval (default 30s, tests use 500ms)
- [Phase 10-server-binary-dag-dispatch]: Task outputs.json uses plain output names without task prefix — scheduler wraps in StructValue keyed by call alias

### Pending Todos

None.

### Blockers/Concerns

- Phase 10 (long-poll implementation): Go long-poll patterns (`http.Flusher` vs. short-interval timeout) should be researched during Phase 10 planning — goroutine count under simultaneous daemon connections needs evaluation
- Phase 11 (output collection): Confirm daemon can invoke `wdl/task` output collection with shared-FS absolute paths without triggering `filepath.Abs()` in `pathmap.go` — needs code walkthrough during Phase 11 planning

## Session Continuity

Last session: 2026-03-11T23:50:00.143Z
Stopped at: Completed 10-server-binary-dag-dispatch/10-02-PLAN.md
Resume file: None
