# Phase 18: MinIO Integration Testing - Context

**Gathered:** 2026-03-13
**Status:** Ready for planning

<domain>
## Phase Boundary

End-to-end integration tests using a real MinIO server validate S3 workflow execution, and spectool 156/156 passes on both local and S3 backends. This phase adds testcontainers-go integration tests and a `make spectool-s3` target. No production code changes — testing infrastructure only.

</domain>

<decisions>
## Implementation Decisions

### Test container lifecycle
- Use testcontainers-go for programmatic MinIO lifecycle in Go integration tests
- Pin MinIO image to `minio/minio:RELEASE.2024-11-07T00-52-28Z` (last stable release before repo archival)
- Hardcoded test credentials: `minioadmin` / `minioadmin` (MinIO defaults)
- One MinIO container per test package via TestMain — shared across all test functions, torn down after all tests complete

### Integration test scope
- Tests live in `internal/integration/` package, separate from unit tests
- Cover single-task S3 round-trip: upload input to MinIO → submit task → daemon localizes → mock executor runs → daemon delocalizes → verify output in MinIO
- Use Go function calls (in-process Server/Daemon wiring), not binary subprocesses
- Mock executor replaces RunTask — reads inputs from stageDir, writes output files to workDir. No Docker dependency in integration tests

### Spectool S3 mode
- New Makefile target: `make spectool-s3`
- MinIO managed via `docker run` in Makefile with pinned image tag — same PID-file pattern as existing server/daemon lifecycle
- Auto-create test bucket via `mc mb` or `aws s3 mb` CLI after MinIO readiness check
- Generate temporary `config.yaml` with S3 settings (endpoint, bucket, credentials, path-style) — server and daemon read via `-c` flag

### CI gating
- Integration tests gated with `//go:build integration` build tag — `go test ./...` skips them
- Timeout: 2 minutes (`-timeout 2m`)
- New Makefile target: `make integration` runs `go test -tags integration -timeout 2m ./internal/integration/...`

### Claude's Discretion
- testcontainers-go container request configuration details
- TestMain setup/teardown helper structure
- Mock executor implementation (how to inject into daemon)
- Config.yaml template for spectool-s3 (exact YAML structure)
- MinIO readiness check method in Makefile (curl vs mc vs aws cli)
- Bucket naming convention for tests

</decisions>

<specifics>
## Specific Ideas

- The spectool-s3 target should mirror the existing `make spectool` structure closely — start MinIO + server + daemon, run spectool, stop all three
- Integration tests prove the LED pattern works end-to-end without relying on Docker containers for WDL task execution — the mock executor validates S3 I/O without container overhead
- The pinned MinIO image tag avoids the archived repo risk flagged in STATE.md

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/storage/s3.go`: S3Backend with full UploadFile/DownloadFile/UploadDirectory — integration tests validate these against real MinIO
- `internal/storage/s3_test.go`: Mock S3 client patterns (mockS3APIClient, mockTransferManager) — integration tests use real client instead
- `internal/daemon/daemon.go`: executeTaskS3 with LED pattern — integration tests exercise this code path
- `internal/config/config.go`: StorageConfig with UsePathStyle, Endpoint — used to configure S3Backend for MinIO
- `Makefile`: Existing spectool target with server/daemon PID management — spectool-s3 reuses this pattern

### Established Patterns
- Server/daemon lifecycle in Makefile: start with `&`, PID file, curl readiness check, kill on cleanup
- Unit tests use mock interfaces (mockS3APIClient) — integration tests bypass mocks for real S3 API
- Build tag gating not yet used in project — this introduces `//go:build integration` as a new pattern

### Integration Points
- `internal/storage/factory.go:NewBackend()` — integration tests construct S3Backend via this factory with MinIO config
- `internal/daemon/daemon.go:executeTaskS3()` — the primary code path under test
- `Makefile` — new `integration` and `spectool-s3` targets

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 18-minio-integration-testing*
*Context gathered: 2026-03-13*
