# Requirements: flowy

**Defined:** 2026-03-11
**Core Value:** WDL 1.2で記述されたワークフローをDockerコンテナ上で正確に実行できること

## v1.1 Requirements

Requirements for distributed server/daemon/client architecture. Each maps to roadmap phases.

### Server

- [x] **SRV-01**: Server exposes POST /workflows endpoint to accept workflow submission (WDL path + inputs JSON), returns run ID
- [x] **SRV-02**: Server exposes GET /workflows/{id}/status endpoint returning run state
- [x] **SRV-03**: Server exposes GET /workflows/{id}/outputs endpoint returning output paths after completion
- [x] **SRV-04**: Server exposes POST /workflows/{id}/abort endpoint to cancel a running workflow
- [x] **SRV-05**: Server exposes GET /health endpoint for liveness checks
- [x] **SRV-06**: Server runs DAG scheduler in-process using runTaskFn injection to dispatch tasks to SQLite queue
- [x] **SRV-07**: Server persists workflow runs and task state in SQLite with WAL mode
- [x] **SRV-08**: Server detects stale tasks (no heartbeat within timeout) and resets them to PENDING
- [x] **SRV-09**: Server binary ships as `flowy-server` (`cmd/flowy-server/`)

### Daemon

- [x] **DMN-01**: Daemon polls server for available tasks and claims them atomically
- [x] **DMN-02**: Daemon executes claimed tasks via existing Docker engine (task.RunTask)
- [x] **DMN-03**: Daemon sends periodic heartbeat to server for running tasks
- [x] **DMN-04**: Daemon handles SIGTERM gracefully, completing in-flight tasks before shutdown
- [x] **DMN-05**: Daemon writes task outputs to shared filesystem convention path
- [x] **DMN-06**: Daemon binary ships as `flowy-daemon` (`cmd/flowy-daemon/`)

### Client

- [ ] **CLI-01**: User can submit a workflow with `flowy submit <wdl> -i <inputs>`
- [ ] **CLI-02**: User can check run status with `flowy status <run-id>`
- [ ] **CLI-03**: User can retrieve outputs with `flowy outputs <run-id>`
- [ ] **CLI-04**: `flowy run` works synchronously (submit → poll → print outputs) requiring a running server; no local fallback
- [ ] **CLI-05**: Client copies local input files to shared filesystem before submitting
- [ ] **CLI-06**: Client binary ships as `flowy` (`cmd/flowy/`)

### Storage

- [x] **STR-01**: Config file (YAML) specifies shared filesystem base directory
- [x] **STR-02**: Config file has sections for server, daemon, and client settings
- [x] **STR-03**: Runs are organized under `{base_dir}/runs/{run_id}/` with inputs/ and outputs/ subdirectories
- [x] **STR-04**: All three binaries read the same config file format

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
| SRV-01 | Phase 10 | Complete |
| SRV-02 | Phase 10 | Complete |
| SRV-03 | Phase 10 | Complete |
| SRV-04 | Phase 10 | Complete |
| SRV-05 | Phase 10 | Complete |
| SRV-06 | Phase 10 | Complete |
| SRV-07 | Phase 9 | Complete |
| SRV-08 | Phase 9 | Complete |
| SRV-09 | Phase 10 | Complete |
| DMN-01 | Phase 11 | Complete |
| DMN-02 | Phase 11 | Complete |
| DMN-03 | Phase 11 | Complete |
| DMN-04 | Phase 11 | Complete |
| DMN-05 | Phase 11 | Complete |
| DMN-06 | Phase 11 | Complete |
| CLI-01 | Phase 12 | Pending |
| CLI-02 | Phase 12 | Pending |
| CLI-03 | Phase 12 | Pending |
| CLI-04 | Phase 12 | Pending |
| CLI-05 | Phase 12 | Pending |
| CLI-06 | Phase 12 | Pending |
| STR-01 | Phase 9 | Complete |
| STR-02 | Phase 9 | Complete |
| STR-03 | Phase 9 | Complete |
| STR-04 | Phase 9 | Complete |

**Coverage:**
- v1.1 requirements: 25 total
- Mapped to phases: 25
- Unmapped: 0

---
*Requirements defined: 2026-03-11*
*Last updated: 2026-03-11 after roadmap creation*
