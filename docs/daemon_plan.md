# Daemon Implementation Plan

## Goal
Build a long-running daemon (`daemon-flowy`) that polls `flowy-server` for jobs, executes them asynchronously, and reports results back.

## Architecture Overview
- Runtime: Tokio async runtime.
- Components:
  - `WorkerConfig`: parsed from CLI / `~/.flowy` (server URL, worker_id, max_concurrency, heartbeat).
  - `ServerClient`: REST client encapsulating job claim, heartbeat, completion APIs.
  - `JobManager`: tracks in-flight jobs, spawns task runners, handles heartbeats.
  - `TaskRunner`: wraps existing execution logic (workflow/task) inside non-blocking async process.
  - `flowy-client`: enqueues a job and polls the job-status endpoint until the daemon completes execution.

## Server API Extensions (Phase 1)
1. `POST /api/v1/workers/register` (optional) to obtain worker_id.
2. `POST /api/v1/jobs/claim` → returns job payload (wdl, inputs, options, run_id, base_dir).
3. `GET /api/v1/jobs/{run_id}` → current job status + (when terminal) success/failure payload.
4. `POST /api/v1/jobs/{run_id}/heartbeat` → payload { worker_id, metrics?, timestamp }.
5. `POST /api/v1/jobs/{run_id}/complete` → payload { execute_response }.
6. `POST /api/v1/jobs/{run_id}/failed` → payload { message, stderr?, exit_code }.
7. (Optional) `POST /api/v1/jobs/{run_id}/cancel` to support server-driven cancellation.

Server stores job state machine: Pending → Running → (Succeeded | Failed | TimedOut | Canceled).

## Daemon Flow (Phase 2)
1. Load config; create `ServerClient` with reqwest.
2. Spawn main loop:
   - While running: if capacity available, `claim_job()`.
   - On claim success: register job in `JobManager`, spawn `TaskRunner`.
   - Sleep/poll interval configurable (e.g., 5-10s with jitter).
3. `TaskRunner` steps:
   - Stage WDL & inputs in temp dir.
   - Launch subprocess via Tokio `Command` or `spawn_blocking` to call existing executor.
   - Return immediately with handle; store start time.
4. Heartbeat loop (per job):
   - Interval = `lease_duration/2` (default lease 300s).
   - Send heartbeat; on failure retry with backoff; after N failures mark job failed & stop process.
   - If server responds cancellation → kill task, report canceled.
5. Completion:
   - Await subprocess exit.
   - Collect outputs (stdout/stderr via existing executor artifacts) and duration.
   - POST to `/complete` or `/failed`.
   - Cleanup temp artifacts.

## Non-blocking Execution (Phase 3)
- Refactor executor entry points to support async interface:
  - Provide wrapper `async fn run_remote_job(job: JobPayload) -> Result<ExecuteResponse, Error>` using `spawn_blocking`.
  - Ensure `set_input_base_dir` is applied.
- Consider streaming logs later.

## Fault Tolerance
- Heartbeats detect worker crashes; server requeues after timeout.
- On daemon restart: call `/jobs/running?worker_id=` to reconcile; optionally resume or mark failed.
- Track retry counts server-side to avoid livelock.

## CLI Changes
- `daemon-flowy` options: `--server`, `--worker-id`, `--max-jobs`, `--heartbeat`, `--poll-interval`, `--debug`.
- Integrate with `cli_config` for defaults (`WORKER_ID`, `MAX_JOBS`, `HEARTBEAT_SECS`).

## Logging & Metrics
- Structured logs per job: claimed, started, heartbeats, stdout/stderr paths, completion.
- Future work: expose metrics endpoint, integrate with tracing.

## Testing Strategy
- Unit: mock `ServerClient` to simulate job lifecycle, heartbeat failures.
- Integration: spawn local `flowy-server` with test endpoints, run daemon in test harness.
- Stress: concurrency tests, long-running job simulation.

## Deliverables / Milestones
1. Server API endpoints + schema updates.
2. `ServerClient` REST wrapper with retries/backoff.
3. `JobManager` + heartbeat loop + async task execution.
4. CLI/config updates (`flowy-client` defaults to queue execution, worker defaults), logging.
5. Tests & docs (`docs/daemon_plan.md`, README updates).
