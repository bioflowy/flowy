---
phase: 17-daemon-staging
verified: 2026-03-13T04:54:00Z
status: passed
score: 9/9 must-haves verified
re_verification: false
---

# Phase 17: Daemon Staging Verification Report

**Phase Goal:** The daemon correctly localizes S3 inputs to POSIX paths before container execution and delocalizes container outputs back to S3, with no s3:// URIs reaching Docker containers
**Verified:** 2026-03-13T04:54:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A WDL task with s3:// file inputs executes via Docker with all inputs as local POSIX paths | VERIFIED | `executeTaskS3` calls `localizeInputs` then passes `localizedInputs` (not `claim.Inputs`) to `task.RunTask` at daemon.go:469 |
| 2 | Task output files are uploaded to S3 after container exit with s3:// URIs in result API | VERIFIED | `delocalizeOutputs` walks workDir, uploads via `UploadFileStream`, builds `localToS3` map, rewrites outputs; `reportResultWithOutputs` sends `TaskResultRequest.Outputs` |
| 3 | Local staging files are removed after successful S3 upload | VERIFIED | `keepStage=false` defer at daemon.go:461-465 calls `os.RemoveAll(stageDir)` on success |
| 4 | S3 upload/download failures are reported to server as task failures with descriptive error messages | VERIFIED | `walkAndLocalize` wraps errors with `"localize: download %s: %w"`; `delocalizeOutputs` wraps with `"delocalize: upload %s: %w"`; both trigger `reportResult` with `"FAILED"` state |
| 5 | Local mode (StorageType=local) works unchanged with API-based input delivery | VERIFIED | `executeTaskLocal` reads from `claim.Inputs` first, falls back to `inputs.json` only when `claim.Inputs == nil` |
| 6 | TaskClaimResponse carries Inputs map and WDLContent string populated from stored run data | VERIFIED | `handleClaim` in handlers.go reads task.InputsJSON (fallback to run.InputsJSON), reads WDL file via `os.ReadFile`, populates both fields in response |
| 7 | TaskResultRequest carries Outputs map that daemon sends after task execution | VERIFIED | `TaskResultRequest.Outputs map[string]interface{}` at api/types.go:159; `reportResultWithOutputs` marshals and sends it |
| 8 | Server persists task outputs in SQLite via outputs_json column | VERIFIED | `handleResult` marshals `req.Outputs` and calls `s.store.UpdateTaskOutputs`; `loadTaskOutputs` reads `t.OutputsJSON` first |
| 9 | makeDispatchFn stores inputs in task record instead of writing only inputs.json file | VERIFIED | dispatch.go:61 calls `s.store.UpdateTaskInputs` after `CreateTask`; file-based write retained for backward compat |

**Score:** 9/9 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/api/types.go` | Extended TaskClaimResponse with Inputs+WDLContent, TaskResultRequest with Outputs | VERIFIED | Lines 131-139 (Inputs, WDLContent), lines 155-160 (Outputs). Correct JSON tags with `omitempty`. |
| `internal/store/sqlite/migrations/00002_task_outputs.sql` | Schema migration adding inputs_json and outputs_json columns | VERIFIED | `ALTER TABLE tasks ADD COLUMN inputs_json TEXT` and `outputs_json TEXT` with goose Up/Down markers |
| `internal/store/sqlite/tasks.go` | UpdateTaskOutputs, UpdateTaskInputs, scanTask with new columns | VERIFIED | All four SELECT queries include `inputs_json, outputs_json`; scanTask uses `sql.NullString` for both; UpdateTaskOutputs at line 226; UpdateTaskInputs at line 237 |
| `internal/server/handlers.go` | handleClaim populates Inputs+WDLContent; handleResult persists Outputs | VERIFIED | handleClaim lines 352-398; handleResult lines 428-435 |
| `internal/server/dispatch.go` | loadTaskOutputs reads from store; makeDispatchFn stores inputs | VERIFIED | loadTaskOutputs at line 111, DB-first with file fallback; makeDispatchFn UpdateTaskInputs at line 61 |
| `internal/daemon/daemon.go` | LED pattern: localizeInputs, delocalizeOutputs, buildBackendFromClaim, refactored executeTask | VERIFIED | All five functions present (lines 160, 178, 232, 261, 285); executeTask refactored into executeTaskS3/executeTaskLocal |
| `internal/daemon/daemon_test.go` | Unit tests for localize, delocalize, staging cleanup, error reporting | VERIFIED | 13 LED-specific test functions present and passing |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `daemon.go:executeTaskS3` | `storage/s3.go:S3Backend` | `buildBackendFromClaim` constructs S3Backend from claim fields | WIRED | daemon.go:439 calls `buildBackendFromClaim(claim)` |
| `daemon.go:localizeInputs` | `storage/backend.go:ParseS3URI` | `walkAndLocalize` calls `ParseS3URI` to extract key, then `DownloadFile` | WIRED | daemon.go:185 `_, key, err := storage.ParseS3URI(val)` |
| `daemon.go:delocalizeOutputs` | `storage/s3.go:UploadFileStream` | WalkDir output files, upload each via `backend.UploadFileStream` | WIRED | daemon.go:319 `backend.UploadFileStream(s3Key, f)` |
| `daemon.go:executeTask` | `api/types.go:TaskClaimResponse` | reads `claim.Inputs` and `claim.WDLContent` instead of files | WIRED | daemon.go:395-399 (WDLContent parse), 447-449 (claim.Inputs for S3), 495-496 (claim.Inputs for local) |
| `daemon.go:reportResultWithOutputs` | `api/types.go:TaskResultRequest` | sends outputs map in result API request | WIRED | daemon.go:352 `api.TaskResultRequest{State: state, Error: errMsg, Outputs: outputs}` |
| `handlers.go:handleClaim` | `store/sqlite/tasks.go` | reads task.InputsJSON, populates TaskClaimResponse.Inputs and WDLContent | WIRED | handlers.go:372-380 unmarshal InputsJSON; line 393-394 set Inputs and WDLContent in response |
| `handlers.go:handleResult` | `store/sqlite/tasks.go:UpdateTaskOutputs` | marshals req.Outputs to JSON, calls UpdateTaskOutputs | WIRED | handlers.go:429-434 |
| `dispatch.go:loadTaskOutputs` | `store/sqlite/tasks.go` | reads t.OutputsJSON instead of downloading outputs.json file | WIRED | dispatch.go:113-124 reads OutputsJSON; file fallback at lines 127-138 |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| DAEM-01 | 17-01, 17-02 | Daemon downloads S3 input files to local staging directory before container execution (localize) | SATISFIED | `localizeInputs` + `walkAndLocalize` implement full recursive download; `task.RunTask` receives `localizedInputs` (POSIX paths) not `claim.Inputs` |
| DAEM-02 | 17-01, 17-02 | Daemon uploads task output files from local directory to S3 after container execution (delocalize) | SATISFIED | `delocalizeOutputs` walks workDir, uploads via `UploadFileStream`; s3:// URIs sent via result API |
| DAEM-03 | 17-02 | Daemon cleans up local staging files after successful S3 upload | SATISFIED | `keepStage=false` defer cleans stageDir on success; `keepStage=true` preserves on delocalize failure |
| DAEM-04 | 17-01, 17-02 | Daemon handles S3 upload/download failures with proper error reporting to server | SATISFIED | Prefixed error messages ("localize: download …", "delocalize: upload …") trigger `reportResult("FAILED", …)` calls |

No orphaned requirements found — all DAEM-01 through DAEM-04 are claimed by plans 17-01 and 17-02 and implemented in the codebase.

### Anti-Patterns Found

No anti-patterns detected. Scan of daemon.go and daemon_test.go revealed:
- No TODO/FIXME/XXX/HACK/PLACEHOLDER comments
- No empty implementations (return null, return {}, etc.)
- No console.log-only handlers
- No stub functions

Note: File-based `inputs.json` / `outputs.json` writes are intentionally retained in local mode for backward compatibility — this is a documented design decision in the SUMMARY, not a stub.

### Human Verification Required

The following items cannot be verified programmatically and require human testing with a real MinIO or AWS S3 endpoint:

#### 1. End-to-End S3 LED Cycle

**Test:** Submit a WDL task with `s3://` file inputs against a server configured with S3/MinIO storage. Let the daemon execute and report completion.
**Expected:** Input files are staged locally before Docker runs, output files appear in S3 bucket, result API response carries s3:// output URIs, stageDir is absent after task completes.
**Why human:** Requires live MinIO instance (integration tests planned for Phase 18).

#### 2. StageDir Preservation on Delocalize Failure

**Test:** Force an upload failure mid-delocalize (e.g., network partition or revoke bucket permissions after localization). Check filesystem after failure.
**Expected:** Staging directory at `/tmp/flowy-stage-*` remains on disk with staged input files.
**Why human:** Requires injecting a real network fault; unit test `TestStageDirPreservedOnDelocalizeFailure` tests the logic but not the full executeTaskS3 flow.

### Gaps Summary

No gaps found. All 9 observable truths are verified, all 7 key links are wired, all 4 requirements are satisfied, and all tests pass (13 LED tests + existing test suite).

---

_Verified: 2026-03-13T04:54:00Z_
_Verifier: Claude (gsd-verifier)_
