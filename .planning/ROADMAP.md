# Roadmap: flowy

## Overview

flowy is a WDL 1.2 workflow engine in Go. v1.0 shipped as a CLI tool with full parse → typecheck → evaluate → execute pipeline. Spectool conformance baseline: 45/160. v1.1 splits the monolithic CLI into a distributed server/daemon/client architecture for multi-host workflow execution.

## Milestones

- ✅ **v1.0 MVP** — Phases 1-8 (shipped 2026-03-11)
- 🚧 **v1.1 Distributed Execution** — Phases 9-12 (in progress)

## Phases

<details>
<summary>✅ v1.0 MVP (Phases 1-8) — SHIPPED 2026-03-11</summary>

- [x] Phase 1: Type Data Structures (1/1 plans) — completed 2026-03-10
- [x] Phase 2: Lexer, Parser, and AST (5/5 plans) — completed 2026-03-10
- [x] Phase 3: Type Checker (4/4 plans) — completed 2026-03-10
- [x] Phase 4: Expression Evaluator and Standard Library (6/6 plans) — completed 2026-03-10
- [x] Phase 5: Task Execution (3/3 plans) — completed 2026-03-10
- [x] Phase 6: Workflow Execution, DAG Scheduler (3/3 plans) — completed 2026-03-10
- [x] Phase 7: CLI and Integration (2/2 plans) — completed 2026-03-11
- [x] Phase 8: Specification Compliance (2/2 plans) — completed 2026-03-11

Full details: `.planning/milestones/v1.0-ROADMAP.md`

</details>

### 🚧 v1.1 Distributed Execution (In Progress)

**Milestone Goal:** Split monolithic CLI into server/daemon/client architecture so workflows execute across multiple hosts via REST API, SQLite task queue, and shared filesystem.

- [x] **Phase 9: API Contract + Persistence Foundation** - Shared DTO types, YAML config, and SQLite schema that all three binaries depend on (completed 2026-03-11)
- [x] **Phase 10: Server Binary + DAG Dispatch** - `flowy-server` with REST API, workflow orchestration, and task queue (completed 2026-03-11)
- [x] **Phase 11: Daemon Binary + Task Execution** - `flowy-daemon` polling worker with Docker execution and heartbeat (completed 2026-03-12)
- [ ] **Phase 12: Client CLI + Spectool Compatibility** - Extended `flowy` with submit/status/outputs commands and synchronous `run` mode

## Phase Details

### Phase 9: API Contract + Persistence Foundation
**Goal**: Shared foundation packages exist and are correct, so all three binaries can be built on stable contracts without drift or schema migration surprises
**Depends on**: Phase 8 (v1.0 complete)
**Requirements**: STR-01, STR-02, STR-03, STR-04, SRV-07, SRV-08
**Success Criteria** (what must be TRUE):
  1. A config YAML file can be written once and loaded correctly by all three binaries (server, daemon, client) with server, daemon, and client sections each present
  2. The shared filesystem layout `{base_dir}/runs/{run_id}/inputs/` and `{base_dir}/runs/{run_id}/outputs/` is created correctly at runtime and accessible from a path defined in config
  3. The SQLite database opens in WAL mode, migrations run on startup without error, and the runs + tasks tables exist with the correct schema including `last_heartbeat_at` and `shard_index` columns
  4. A stale task (no heartbeat update within configurable timeout) can be detected by querying the tasks table — the detection query returns the expected stale rows in an integration test
  5. All shared HTTP DTO types (workflow submission request, status response, outputs response) are defined in `internal/api` and compile without import cycles
**Plans:** 2/2 plans complete
Plans:
- [ ] 09-01-PLAN.md — Config package, API DTO types, and filesystem layout helpers
- [ ] 09-02-PLAN.md — Store interface and SQLite implementation with migrations

### Phase 10: Server Binary + DAG Dispatch
**Goal**: `flowy-server` binary runs, accepts workflow submissions via REST API, schedules the DAG, and dispatches tasks to the SQLite queue for daemon pickup
**Depends on**: Phase 9
**Requirements**: SRV-01, SRV-02, SRV-03, SRV-04, SRV-05, SRV-06, SRV-09
**Success Criteria** (what must be TRUE):
  1. `POST /workflows` with a valid WDL path and inputs JSON returns a run ID and HTTP 201, and the run appears in the SQLite database with state PENDING
  2. `GET /workflows/{id}/status` returns the current run state (PENDING, RUNNING, COMPLETED, FAILED) for a submitted workflow
  3. `GET /workflows/{id}/outputs` returns output key-value pairs after a workflow reaches COMPLETED state
  4. `POST /workflows/{id}/abort` transitions a running workflow to ABORTED and cancels its in-progress tasks
  5. `GET /health` returns HTTP 200 and the server correctly ships as the `flowy-server` binary under `cmd/flowy-server/`
**Plans:** 2/2 plans complete
Plans:
- [ ] 10-01-PLAN.md — Store extensions (GetTaskByID, UpdateHeartbeatByDaemon), per-task directory helpers, RunWorkflowWithDispatch, api comment fix
- [ ] 10-02-PLAN.md — Server package with all REST handlers, DAG dispatch logic, stale detection, daemon-facing endpoints, and flowy-server binary

### Phase 11: Daemon Binary + Task Execution
**Goal**: `flowy-daemon` binary runs, claims tasks from the server atomically, executes them via Docker, writes outputs to the shared filesystem, and handles crashes gracefully
**Depends on**: Phase 10
**Requirements**: DMN-01, DMN-02, DMN-03, DMN-04, DMN-05, DMN-06
**Success Criteria** (what must be TRUE):
  1. When the server has a PENDING task and two daemons poll simultaneously, exactly one daemon claims the task — the other receives an empty response and retries without error
  2. A claimed task executes via the existing Docker engine and its outputs are written to `{base_dir}/runs/{run_id}/outputs/` on the shared filesystem before the daemon reports completion to the server
  3. The server marks the run COMPLETED and makes outputs available after the daemon reports task completion — an end-to-end workflow (server + daemon) runs a real WDL task successfully
  4. A daemon that receives SIGTERM completes its in-flight task and shuts down cleanly within 30 seconds without leaving orphaned Docker containers
  5. The daemon sends a heartbeat to the server every 30 seconds for running tasks, and the `flowy-daemon` binary ships under `cmd/flowy-daemon/`
**Plans:** 2/2 plans complete
Plans:
- [ ] 11-01-PLAN.md — Server-side prerequisites (WDLPath, Workers config) and daemon package implementation
- [ ] 11-02-PLAN.md — Daemon integration tests and flowy-daemon binary entry point

### Phase 12: Client CLI + Spectool Compatibility
**Goal**: The `flowy` binary provides submit/status/outputs commands for distributed workflows, and `flowy run` works synchronously (submit → poll → print outputs) requiring a running server
**Depends on**: Phase 11
**Requirements**: CLI-01, CLI-02, CLI-03, CLI-04, CLI-05, CLI-06
**Success Criteria** (what must be TRUE):
  1. `flowy submit <wdl> -i <inputs.json>` copies local input files to the shared filesystem, submits to the server, and prints the run ID to stdout
  2. `flowy status <run-id>` prints the current workflow state and `flowy outputs <run-id>` prints output key-value pairs for a completed run
  3. `flowy run <wdl> -i <inputs.json>` completes synchronously — submits the workflow, polls until a terminal state, prints outputs on success or an error with task name and exit code on failure, and exits with the correct exit code
  4. `flowy run` without a reachable server exits with a clear error message (server is required for execution; no local fallback)
  5. The `flowy` client binary ships under `cmd/flowy/` and all three binaries (`flowy`, `flowy-server`, `flowy-daemon`) build from a single `go build ./...`
**Plans:** 1/3 plans executed
Plans:
- [ ] 12-01-PLAN.md — Server-side prerequisites (API types, config, prefix matching) and client HTTP package
- [ ] 12-02-PLAN.md — Submit, status, outputs subcommands with file staging
- [ ] 12-03-PLAN.md — Replace run command with server-based implementation and update tests

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Type Data Structures | v1.0 | 1/1 | Complete | 2026-03-10 |
| 2. Lexer, Parser, and AST | v1.0 | 5/5 | Complete | 2026-03-10 |
| 3. Type Checker | v1.0 | 4/4 | Complete | 2026-03-10 |
| 4. Expression Evaluator and Standard Library | v1.0 | 6/6 | Complete | 2026-03-10 |
| 5. Task Execution | v1.0 | 3/3 | Complete | 2026-03-10 |
| 6. Workflow Execution (DAG Scheduler) | v1.0 | 3/3 | Complete | 2026-03-10 |
| 7. CLI and Integration | v1.0 | 2/2 | Complete | 2026-03-11 |
| 8. Specification Compliance | v1.0 | 2/2 | Complete | 2026-03-11 |
| 9. API Contract + Persistence Foundation | 2/2 | Complete   | 2026-03-11 | - |
| 10. Server Binary + DAG Dispatch | 2/2 | Complete    | 2026-03-11 | - |
| 11. Daemon Binary + Task Execution | 2/2 | Complete    | 2026-03-12 | - |
| 12. Client CLI + Spectool Compatibility | 1/3 | In Progress|  | - |
