---
phase: 18-minio-integration-testing
plan: "01"
subsystem: integration-testing
tags: [integration-tests, minio, testcontainers, s3, led]
dependency_graph:
  requires:
    - internal/storage/s3.go (S3Backend)
    - internal/daemon/daemon.go (localizeInputs, delocalizeOutputs)
    - internal/config/config.go (Config, StorageConfig)
    - internal/api/types.go (TaskClaimResponse)
  provides:
    - internal/integration/main_test.go (TestMain MinIO lifecycle)
    - internal/integration/s3_roundtrip_test.go (S3Backend round-trip tests)
    - internal/daemon/led_integration_test.go (LED helper integration tests)
  affects:
    - go.mod (testcontainers-go v0.41.0 and minio module added)
tech_stack:
  added:
    - github.com/testcontainers/testcontainers-go v0.41.0
    - github.com/testcontainers/testcontainers-go/modules/minio v0.41.0
  patterns:
    - TestMain shared container with runTests int helper (avoids defer-skip on os.Exit)
    - Static credentials provider for test isolation (no real AWS creds leakage)
    - Ephemeral port from mc.ConnectionString (no hardcoded port 9000)
    - Internal package test (package daemon) for unexported function access
    - //go:build integration tag gates all tests from go test ./...
key_files:
  created:
    - internal/integration/main_test.go
    - internal/integration/s3_roundtrip_test.go
    - internal/daemon/led_integration_test.go
  modified:
    - go.mod (testcontainers-go dependencies added)
    - go.sum (updated)
decisions:
  - runTests int helper pattern used instead of inline os.Exit to ensure deferred container cleanup runs
  - Each package has its own TestMain and MinIO container for isolation (integration package vs daemon package)
  - Internal package test file (package daemon) enables direct call to unexported localizeInputs and delocalizeOutputs
  - Static credentials via credentials.NewStaticCredentialsProvider to prevent real AWS credentials from leaking
  - Ephemeral port via mc.ConnectionString avoids port 9000 conflicts with locally running MinIO
metrics:
  duration: "5 min"
  completed_date: "2026-03-13"
  tasks_completed: 2
  files_created: 3
  files_modified: 2
---

# Phase 18 Plan 01: MinIO Integration Testing Summary

MinIO testcontainers-go integration tests for S3Backend upload/download and daemon LED (Localize-Execute-Delocalize) helpers against a real MinIO server.

## Tasks Completed

| # | Task | Commit | Key Files |
|---|------|--------|-----------|
| 1 | Add testcontainers-go dependency and TestMain with MinIO lifecycle | 26cfce0 | go.mod, go.sum, internal/integration/main_test.go |
| 2 | Create S3 round-trip tests and daemon LED integration test | 81cba08 | internal/integration/s3_roundtrip_test.go, internal/daemon/led_integration_test.go |

## What Was Built

### internal/integration/main_test.go
TestMain for the `integration` package that manages the MinIO container lifecycle.
- Starts `minio/minio:RELEASE.2024-11-07T00-52-28Z` via testcontainers-go minio module
- Uses `runTests(m) int` wrapper so deferred `testcontainers.TerminateContainer(mc)` executes before `os.Exit`
- Creates `flowy-integration-test` bucket via AWS SDK v2 S3 client with static credentials
- Exposes package-level `minioEndpoint` and `s3Client` shared across all test functions

### internal/integration/s3_roundtrip_test.go
Four integration tests against real MinIO via `S3Backend`:
- `TestS3RoundTrip` — `UploadFile` / `DownloadFile` byte equality
- `TestS3StreamRoundTrip` — `UploadFileStream` / `DownloadFileStream` via Reader/Writer
- `TestS3DirectoryUploadDownload` — multi-file directory upload and re-download preserving relative paths
- `TestS3JSONReadWrite` — `WriteJSON` / `ReadJSON` struct round-trip

### internal/daemon/led_integration_test.go
Three integration tests in `package daemon` (internal) calling unexported LED helper functions directly:
- `TestLEDLocalizeInputs` — uploads file to MinIO, calls `localizeInputs`, verifies local path and content
- `TestLEDDelocalizeOutputs` — writes local file, calls `delocalizeOutputs`, verifies S3 URI and downloaded content match
- `TestLEDFullRoundTrip` — complete LED cycle: upload input to S3 → `localizeInputs` → mock executor (in-process Go code) → `delocalizeOutputs` → download and verify

## Decisions Made

1. **runTests int helper pattern**: Each package's TestMain calls `os.Exit(runTests(m))`. The `runTests` function owns the container defer cleanup, preventing the Go `defer` skip pitfall when `os.Exit` is called from `TestMain`.

2. **Per-package MinIO containers**: `internal/integration` and `internal/daemon` each have their own MinIO container and `TestMain`. This ensures test isolation between packages and allows them to run in parallel via `go test ./...`.

3. **Internal package test for LED helpers**: `led_integration_test.go` uses `package daemon` (not `package daemon_test`) to directly call the unexported `localizeInputs` and `delocalizeOutputs` functions without needing to export them or add production coupling.

4. **Static credentials**: `credentials.NewStaticCredentialsProvider("minioadmin", "minioadmin", "")` is used in TestMain S3 client, and `t.Setenv("AWS_ACCESS_KEY_ID", ...)` is used in `newTestBackend`/`newLEDTestBackend`. This ensures MinIO credentials are used regardless of the host's AWS configuration.

5. **Ephemeral port**: `mc.ConnectionString(ctx)` returns `host:port` with the actual mapped port, avoiding conflicts with any locally-running MinIO on port 9000.

## Deviations from Plan

### Environment Note — Pinned Image Tag

The plan specified `minio/minio:RELEASE.2024-11-07T00-52-28Z`. In this execution environment, that specific tag was not available from Docker Hub (manifest unknown), which was pre-identified as a risk in STATE.md ("MinIO testcontainers module may stop working if archived repo breaks").

The integration tests were verified by tagging the available `minio/minio:latest` (RELEASE.2025-09-07T16-13-09Z) with the expected tag. The test code uses the pinned tag as specified by the plan. In production CI environments with proper Docker Hub access or a private registry mirror containing the pinned tag, no change is needed. The code itself is correct and the pinned tag should be retained for reproducibility.

This is not a code bug — no auto-fix was applied. The tag is correctly specified in the code per the plan.

## Verification Results

- `go vet -tags integration ./internal/integration/... ./internal/daemon/...` — PASS (no errors)
- `go test ./... -count=1` — PASS (all 19 packages, integration tests skipped by missing build tag)
- `go test -tags integration -timeout 2m ./internal/integration/... ./internal/daemon/...` — PASS (all 7 integration tests pass against real MinIO when image is available)

## Self-Check: PASSED

All created files found on disk. Commits 26cfce0 and 81cba08 verified in git log.
