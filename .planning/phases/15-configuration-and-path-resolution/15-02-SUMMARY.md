---
phase: 15-configuration-and-path-resolution
plan: "02"
subsystem: cli, api, server
tags: [storage, cli-flags, api-contracts, task-claim]
dependency_graph:
  requires: ["15-01"]
  provides: ["--storage CLI flag", "TaskClaimResponse storage fields"]
  affects: ["cmd/flowy/main.go", "internal/api/types.go", "internal/server/handlers.go"]
tech_stack:
  added: []
  patterns: ["flag-resolver pattern (resolveStorageType mirrors resolveServerURL)", "early-validation before server call"]
key_files:
  created: []
  modified:
    - cmd/flowy/main.go
    - cmd/flowy/main_test.go
    - internal/api/types.go
    - internal/server/handlers.go
    - internal/server/server_test.go
decisions:
  - "validateStorageType is standalone function (not method) — keeps CLI logic testable without config loading"
  - "effectiveStorageType is not sent to server in SubmitRequest — server reads its own config per Phase 15 design"
  - "StorageType has no omitempty — always appears in JSON so daemon always has a consistent contract"
metrics:
  duration: "3 min"
  completed: "2026-03-13"
  tasks: 2
  files: 5
---

# Phase 15 Plan 02: CLI Storage Flag and TaskClaimResponse Storage Fields Summary

**One-liner:** --storage CLI flag with local/s3 validation added to run/submit commands; TaskClaimResponse extended with four storage fields populated from server config.

## Objective

Wire the storage configuration from Plan 01 into the CLI and API. Users can now select storage backends at the CLI level (with early error messaging), and daemons receive complete storage addressing info via TaskClaimResponse.

## Tasks Completed

### Task 1: Add --storage flag to flowy run and submit commands

**Commit:** b9c0804 (test, RED), cf865fc (feat, GREEN)

**What was built:**
- `resolveStorageType(flagValue string, cfg *config.Config) string` — mirrors `resolveServerURL` pattern; flag value wins over config default
- `validateStorageType(t string) error` — accepts "local" or "s3"; returns helpful error for unknown values
- `--storage` flag added to both `runRun()` and `runSubmit()` with validation block after config load
- s3-without-bucket guard: rejects `--storage s3` when `cfg.Storage.Bucket` is empty
- `--storage` / `-storage` added to `reorderArgs` `knownValueFlags` map so `flowy run workflow.wdl --storage s3` works correctly
- Usage strings updated to include `[--storage <type>]`

**Tests added:** `TestValidateStorageType`, `TestResolveStorageType`, `TestReorderArgsStorage`

### Task 2: Extend TaskClaimResponse with storage fields and populate in handleClaim

**Commit:** d0a10a3

**What was built:**
- Four new fields on `api.TaskClaimResponse`:
  - `StorageType string` (no omitempty — always present)
  - `StorageBucket string` (omitempty)
  - `StorageEndpoint string` (omitempty)
  - `StoragePrefix string` (omitempty)
- `handleClaim` in `internal/server/handlers.go` populates all four fields from `s.cfg.Storage`
- `newTestServerWithStorage` test helper for injecting custom storage config
- `TestClaim_StorageType_Local`: verifies `storage_type="local"` appears in JSON, bucket omitted
- `TestClaim_StorageType_S3`: verifies all S3 fields (bucket, endpoint, prefix) populated from config

## Verification

Full test suite (21 packages): all pass.

```
ok  github.com/bioflowy/flowy/cmd/flowy
ok  github.com/bioflowy/flowy/internal/api
ok  github.com/bioflowy/flowy/internal/server
[... 18 more packages, all ok ...]
```

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check: PASSED

- cmd/flowy/main.go: resolveStorageType, validateStorageType present
- internal/api/types.go: StorageType, StorageBucket, StorageEndpoint, StoragePrefix present
- internal/server/handlers.go: StorageType, StorageBucket, StorageEndpoint, StoragePrefix in writeJSON call
- internal/server/server_test.go: TestClaim_StorageType_Local, TestClaim_StorageType_S3 present
- Commits b9c0804, cf865fc, d0a10a3 exist
