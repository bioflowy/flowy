---
phase: 18-minio-integration-testing
plan: "02"
subsystem: testing
tags: [makefile, minio, s3, docker, testcontainers, conformance]

# Dependency graph
requires:
  - phase: 18-minio-integration-testing plan 01
    provides: testcontainers-go dependency and integration test infrastructure

provides:
  - make integration target for running go test -tags integration against MinIO testcontainer
  - make spectool-s3 target for full conformance testing with S3/MinIO backend

affects:
  - ci/cd pipelines that invoke Makefile targets
  - developer workflow for S3 backend validation

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Docker --cidfile pattern for container lifecycle management (vs PID files for processes)"
    - "Port separation: spectool-s3 uses 8125 to run alongside spectool on 8124"
    - "AWS credentials via env vars for server/daemon (not stored in config.yaml)"

key-files:
  created: []
  modified:
    - Makefile

key-decisions:
  - "Use docker run --cidfile (.minio.cid) instead of PID file for MinIO: containers don't have a process PID accessible in parent shell"
  - "Port 8125 for spectool-s3 server to allow simultaneous execution alongside regular spectool on 8124"
  - "AWS credentials exported as env vars before server/daemon start: config.yaml intentionally omits credential storage"
  - "Cleanup runs unconditionally via captured exit code pattern (SPECTOOL_EXIT=$$?) identical to existing spectool target"

patterns-established:
  - "CID-file pattern: docker run --rm -d --cidfile .minio.cid for containers; docker rm -f $(cat .minio.cid) for teardown"
  - "Pinned MinIO image in Makefile variable MINIO_IMAGE := minio/minio:RELEASE.2024-11-07T00-52-28Z"

requirements-completed: [TEST-01, TEST-02]

# Metrics
duration: 2min
completed: 2026-03-13
---

# Phase 18 Plan 02: Makefile Integration and spectool-s3 Targets Summary

**Two Makefile targets added: `make integration` for testcontainer-based Go tests and `make spectool-s3` for full WDL conformance testing against a MinIO S3 backend using pinned image and Docker CID-file lifecycle.**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-13T05:57:05Z
- **Completed:** 2026-03-13T05:59:25Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments

- Added `make integration` target running `go test -tags integration -timeout 2m -count=1 ./internal/integration/...`
- Added `make spectool-s3` target with full MinIO container lifecycle (start, health check, bucket creation, S3 config generation, server/daemon startup, spectool run, full teardown)
- Added MinIO-specific variables: `MINIO_IMAGE` (pinned), `MINIO_PORT`, `MINIO_CID`, `TEST_BUCKET`, `S3_CONFIG`, `S3_PORT`
- Updated `.PHONY` to include both new targets

## Task Commits

Each task was committed atomically:

1. **Task 1: Add make integration and make spectool-s3 targets to Makefile** - `c6fad71` (feat)

## Files Created/Modified

- `Makefile` - Added MINIO_* variables, integration target, and spectool-s3 target with full MinIO lifecycle

## Decisions Made

- Used Docker `--cidfile` pattern for MinIO container (`.minio.cid`) rather than PID file — Docker containers don't expose a host-accessible PID, so CID-based cleanup via `docker rm -f` is the correct approach.
- Port 8125 for `spectool-s3` server to allow simultaneous execution alongside regular `spectool` target on port 8124, enabling parallel use cases.
- AWS credentials (`AWS_ACCESS_KEY_ID=minioadmin`, `AWS_SECRET_ACCESS_KEY=minioadmin`) exported as env vars before `flowy-server` and `flowy-daemon` start, consistent with the design decision that config.yaml does not store credentials.
- `spectool-s3` uses the identical captured-exit-code cleanup pattern as the existing `spectool` target (`SPECTOOL_EXIT=$$?`) ensuring MinIO and server/daemon cleanup even on test failure.

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 18 plan 02 complete: Makefile targets ready for use
- `make integration` can be run once Docker is available to exercise S3 round-trip tests
- `make spectool-s3` can be run once binaries are built and aws CLI is available
- Remaining plan 01 work (s3_roundtrip_test.go, led_integration_test.go) would complete the integration test suite

---
*Phase: 18-minio-integration-testing*
*Completed: 2026-03-13*

## Self-Check: PASSED

- Makefile: FOUND
- Commit c6fad71: FOUND
- 18-02-SUMMARY.md: FOUND
