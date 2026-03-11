# Requirements: flowy

**Defined:** 2026-03-11
**Core Value:** WDL 1.2で記述されたワークフローをDockerコンテナ上で正確に実行できること

## v1.1 Requirements

Requirements for distributed server/daemon/client architecture. Each maps to roadmap phases.

### Server

- [ ] **SRV-01**: Server exposes POST /workflows endpoint to accept workflow submission (WDL path + inputs JSON), returns run ID
- [ ] **SRV-02**: Server exposes GET /workflows/{id}/status endpoint returning run state
- [ ] **SRV-03**: Server exposes GET /workflows/{id}/outputs endpoint returning output paths after completion
- [ ] **SRV-04**: Server exposes POST /workflows/{id}/abort endpoint to cancel a running workflow
- [ ] **SRV-05**: Server exposes GET /health endpoint for liveness checks
- [ ] **SRV-06**: Server runs DAG scheduler in-process using runTaskFn injection to dispatch tasks to SQLite queue
- [ ] **SRV-07**: Server persists workflow runs and task state in SQLite with WAL mode
- [ ] **SRV-08**: Server detects stale tasks (no heartbeat within timeout) and resets them to PENDING
- [ ] **SRV-09**: Server binary ships as `flowy-server` (`cmd/flowy-server/`)

### Daemon

- [ ] **DMN-01**: Daemon polls server for available tasks and claims them atomically
- [ ] **DMN-02**: Daemon executes claimed tasks via existing Docker engine (task.RunTask)
- [ ] **DMN-03**: Daemon sends periodic heartbeat to server for running tasks
- [ ] **DMN-04**: Daemon handles SIGTERM gracefully, completing in-flight tasks before shutdown
- [ ] **DMN-05**: Daemon writes task outputs to shared filesystem convention path
- [ ] **DMN-06**: Daemon binary ships as `flowy-daemon` (`cmd/flowy-daemon/`)

### Client

- [ ] **CLI-01**: User can submit a workflow with `flowy submit <wdl> -i <inputs>`
- [ ] **CLI-02**: User can check run status with `flowy status <run-id>`
- [ ] **CLI-03**: User can retrieve outputs with `flowy outputs <run-id>`
- [ ] **CLI-04**: `flowy run` works synchronously (submit → poll → print outputs) for spectool compatibility
- [ ] **CLI-05**: Client copies local input files to shared filesystem before submitting
- [ ] **CLI-06**: Client binary ships as `flowy` (`cmd/flowy/`)

### Storage

- [ ] **STR-01**: Config file (YAML) specifies shared filesystem base directory
- [ ] **STR-02**: Config file has sections for server, daemon, and client settings
- [ ] **STR-03**: Runs are organized under `{base_dir}/runs/{run_id}/` with inputs/ and outputs/ subdirectories
- [ ] **STR-04**: All three binaries read the same config file format

## Future Requirements

### v1.x (After Validation)

- **API-01**: GET /workflows?status=Running — list/query runs
- **API-02**: GET /workflows/{id}/logs — task log retrieval
- **DMN-07**: Daemon auto-registration with server (POST /daemons/register)

### v2+ (Deferred)

- **CACHE-01**: Execution cache / call cache (task output reuse)
- **RESUME-01**: Workflow resume from checkpoint
- **S3-01**: S3/GCS object storage backend
- **AUTH-01**: Authentication (JWT, API keys) for multi-tenant deployment
- **PG-01**: PostgreSQL backend for multi-server coordination

## Out of Scope

| Feature | Reason |
|---------|--------|
| S3/object storage | Shared filesystem first; S3 deferred to future milestone |
| HTTP(S) import resolution | Not needed for distributed architecture |
| Execution cache | Complex hashing/invalidation; defer until v1.1 stable |
| Workflow resume | Requires careful partial-state reconstruction |
| Docker-alternative backends | Docker sufficient for v1.1 |
| WebSocket/SSE for status | No web UI; polling sufficient for batch workflows |
| JWT/OAuth2 authentication | HPC clusters use network-level security; add later if needed |
| gRPC API | REST is standard in bioinformatics tooling |
| Kubernetes integration | Enormous scope; Docker per node sufficient for HPC |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| SRV-01 | — | Pending |
| SRV-02 | — | Pending |
| SRV-03 | — | Pending |
| SRV-04 | — | Pending |
| SRV-05 | — | Pending |
| SRV-06 | — | Pending |
| SRV-07 | — | Pending |
| SRV-08 | — | Pending |
| SRV-09 | — | Pending |
| DMN-01 | — | Pending |
| DMN-02 | — | Pending |
| DMN-03 | — | Pending |
| DMN-04 | — | Pending |
| DMN-05 | — | Pending |
| DMN-06 | — | Pending |
| CLI-01 | — | Pending |
| CLI-02 | — | Pending |
| CLI-03 | — | Pending |
| CLI-04 | — | Pending |
| CLI-05 | — | Pending |
| CLI-06 | — | Pending |
| STR-01 | — | Pending |
| STR-02 | — | Pending |
| STR-03 | — | Pending |
| STR-04 | — | Pending |

**Coverage:**
- v1.1 requirements: 25 total
- Mapped to phases: 0
- Unmapped: 25 ⚠️

---
*Requirements defined: 2026-03-11*
*Last updated: 2026-03-11 after initial definition*
