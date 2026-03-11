---
gsd_state_version: 1.0
milestone: v1.1
milestone_name: Distributed Execution
status: ready_to_plan
stopped_at: null
last_updated: "2026-03-11"
last_activity: 2026-03-11 — Roadmap created for v1.1 (4 phases, 25 requirements mapped)
progress:
  total_phases: 4
  completed_phases: 0
  total_plans: 0
  completed_plans: 0
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

## Accumulated Context

### Decisions

- Separate binaries: flowy (client), flowy-server, flowy-daemon
- Daemon polls server for tasks (not server-push); long-poll (30s timeout) preferred over interval polling
- Shared filesystem (NFS) first, S3 deferred to v2+
- `flowy run` maintains synchronous behavior for spectool compatibility; local fallback if no server configured
- SQLite with WAL mode + `modernc.org/sqlite` (CGO-free); chi v5 for routing; goose for migrations
- Atomic task claiming via single `UPDATE WHERE state='PENDING'` — SELECT+UPDATE is a TOCTOU race
- `runTaskFn` injection point in `wdl/workflow/scheduler.go` is the designed extension seam for distributed dispatch
- Tasks table must include `last_heartbeat_at` and `shard_index` columns from Phase 9 — cannot retrofit later

### Pending Todos

None.

### Blockers/Concerns

- Phase 10 (long-poll implementation): Go long-poll patterns (`http.Flusher` vs. short-interval timeout) should be researched during Phase 10 planning — goroutine count under simultaneous daemon connections needs evaluation
- Phase 11 (output collection): Confirm daemon can invoke `wdl/task` output collection with shared-FS absolute paths without triggering `filepath.Abs()` in `pathmap.go` — needs code walkthrough during Phase 11 planning

## Session Continuity

Last session: 2026-03-11
Stopped at: Roadmap created — ready to plan Phase 9
Resume file: None
