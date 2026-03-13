---
phase: 17-daemon-staging
plan: "02"
subsystem: daemon-led
tags: [daemon, s3, led, localize, delocalize, storage, tdd]
dependency_graph:
  requires:
    - TaskClaimResponse.Inputs and WDLContent fields (17-01)
    - TaskResultRequest.Outputs field (17-01)
    - storage.Backend interface with UploadFileStream and DownloadFile (14-01)
    - storage.ParseS3URI (14-01)
    - storage.NewS3Backend (16-01)
  provides:
    - LED pattern: localizeInputs, delocalizeOutputs, buildBackendFromClaim
    - rewriteOutputPaths: rewrites local paths to s3:// URIs in JSON maps
    - reportResultWithOutputs: reports task results with outputs map
    - executeTaskS3: full LED cycle for S3-backed task execution
    - executeTaskLocal: local mode with API-based input delivery
  affects:
    - internal/daemon/daemon.go
    - internal/daemon/daemon_test.go
tech_stack:
  added: []
  patterns:
    - TDD (RED then GREEN)
    - LED (Localize-Execute-Delocalize) for S3 task execution
    - mockBackend test injection pattern
    - keepStage flag for conditional stageDir cleanup
key_files:
  created: []
  modified:
    - internal/daemon/daemon.go
    - internal/daemon/daemon_test.go
decisions:
  - executeTask refactored into executeTaskS3 and executeTaskLocal for clear S3/local branching
  - keepStage flag with defer used to preserve stageDir only on delocalize failure
  - Local mode retains outputs.json write for backward compatibility with file-based readers
  - Local mode now also calls reportResultWithOutputs so server stores outputs in DB
  - wdlDir is empty string in S3 mode since all staged inputs use absolute paths
metrics:
  duration: "5 min"
  completed: "2026-03-13"
  tasks_completed: 2
  files_changed: 2
---

# Phase 17 Plan 02: Daemon LED Pattern Implementation Summary

**One-liner:** LED (Localize-Execute-Delocalize) pattern for S3-backed tasks — s3:// inputs staged locally, container runs with POSIX paths, outputs uploaded back to S3 with URI rewriting.

## Tasks Completed

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | Implement LED functions and refactor executeTask | f0c4d09 | daemon.go |
| 2 | Unit tests for daemon LED functions | 678df37 | daemon_test.go |

## What Was Built

### LED Functions (daemon.go)

**buildBackendFromClaim(claim)**: Constructs an `S3Backend` from `TaskClaimResponse` storage fields (Type, Bucket, Endpoint, Prefix). MultipartThreshold defaults to 100 MiB.

**walkAndLocalize(v, backend, inputsDir)**: Recursive type switch over JSON-compatible values. Downloads any `s3://` string to `inputsDir` using `filepath.Base(key)` as local filename. Passes through non-s3:// strings, booleans, numbers, nil, arrays, and maps unchanged. Returns error with `"localize: download s3://..."` prefix on failure.

**localizeInputs(inputs, backend)**: Creates `os.MkdirTemp("", "flowy-stage-")`, calls `walkAndLocalize`, cleans up stageDir on error.

**delocalizeOutputs(outputs, workDir, claim, backend)**: Walks `workDir` via `filepath.WalkDir`, uploads each file via `backend.UploadFileStream(claim.OutputsDir + "/" + keyRel, f)`. Builds `localToS3` map and calls `rewriteOutputPaths` on the JSON outputs. Returns error with `"delocalize: upload"` prefix on first upload failure.

**rewriteOutputPaths(v, localToS3)**: Recursive type switch — replaces local path strings with s3:// URIs from the mapping. Recurses arrays and maps.

**reportResultWithOutputs(ctx, taskID, state, errMsg, outputs)**: Sends `TaskResultRequest` with `Outputs` field populated. The original `reportResult` now delegates to this function with `nil` outputs.

### executeTask Refactoring

`executeTask` now:
1. Parses WDL from `claim.WDLContent` (falls back to `claim.WDLPath` for backward compat)
2. Branches on `claim.StorageType == "s3"` to call `executeTaskS3` or `executeTaskLocal`

**executeTaskS3**: Full LED cycle.
- `buildBackendFromClaim` → `localizeInputs` → `task.RunTask` → `delocalizeOutputs` → `reportResultWithOutputs`
- `keepStage` flag with defer: stageDir cleaned on success/localize/run failure, preserved on delocalize failure

**executeTaskLocal**: Uses `claim.Inputs` directly (falls back to `inputs.json` for backward compat).
- Writes `outputs.json` for local file readers (backward compat)
- Also calls `reportResultWithOutputs` so server stores outputs in DB

### Tests (daemon_test.go)

Added `mockBackend` struct implementing `storage.Backend` with injectable `downloadFn` and `uploadStreamFn`. 13 new test functions cover all LED behavior including error paths, nested structures, and staging lifecycle.

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check: PASSED

All created/modified files confirmed present. Both task commits (f0c4d09, 678df37) confirmed in git log.
