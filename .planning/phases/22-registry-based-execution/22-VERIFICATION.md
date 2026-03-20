---
phase: 22-registry-based-execution
verified: 2026-03-20T00:00:00Z
status: passed
score: 14/14 must-haves verified
---

# Phase 22: Registry-Based Execution Verification Report

**Phase Goal:** Executors can submit workflow runs using a registry reference instead of a file path, and the server resolves the WDL content automatically
**Verified:** 2026-03-20
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| #  | Truth | Status | Evidence |
|----|-------|--------|----------|
| 1  | POST /workflows with registry_ref resolves WDL from registry and starts a run | VERIFIED | `TestHandleSubmitRegistryRef` passes; `resolveRegistryRef` called in `handleSubmit` line 179 |
| 2  | POST /tasks/run with registry_ref resolves WDL from registry and starts a task run | VERIFIED | `TestHandleTaskRunRegistryRef` passes; `resolveRegistryRef` called in `handleTaskRun` line 287 |
| 3  | Both wdl_path and registry_ref provided returns 400 | VERIFIED | `TestHandleSubmitBothFields` and `TestHandleTaskRunBothFields` pass; error message "exactly one of wdl_path or registry_ref must be set, not both" at lines 161, 264 |
| 4  | Neither wdl_path nor registry_ref provided returns 400 | VERIFIED | `TestHandleSubmitNeitherField` passes; error message "one of wdl_path or registry_ref is required" at lines 165, 268 |
| 5  | registry_ref on non-project endpoint returns 400 | VERIFIED | `TestHandleSubmitRegistryRefNoProject` passes; error message "registry_ref requires a project-scoped endpoint" at lines 175, 283 |
| 6  | Unknown registry ref returns 422 | VERIFIED | `TestHandleSubmitRegistryRefNotFound` passes; `writeError(w, http.StatusUnprocessableEntity, ...)` in `resolveRegistryRef` |
| 7  | handleClaim returns non-empty WDLContent for registry-based runs | VERIFIED | `TestHandleClaimRegistryRun` passes; `run.RegistryRef != ""` branch at line 486 re-resolves by digest via `GetRegistryVersionByDigest` |
| 8  | Registry run stores registry_ref and registry_digest in database | VERIFIED | `TestRegistryRunPersistence` passes; INSERT at `runs.go` line 19 includes `registry_ref, registry_digest` |
| 9  | flowy run my-tool:latest executes using registry ref instead of file path | VERIFIED | `isRegistryRef` in `main.go` detects `name:tag` form; routes to `SubmitToProject` / `TaskRunInProject` |
| 10 | flowy submit my-wf:latest submits using registry ref | VERIFIED | `runSubmit` has identical `isRegistryRef` detection and `SubmitToProject` routing |
| 11 | flowy run my-tool@sha256:abc... uses digest-based registry ref | VERIFIED | `isRegistryRef` returns true for `@sha256:` form; `TestIsRegistryRef` covers this case |
| 12 | flowy run hello.wdl still works as a file path (no regression) | VERIFIED | `isRegistryRef("hello.wdl")` returns false; existing `Submit`/`TaskRun` methods unchanged; full test suite passes |
| 13 | --project flag overrides default_project config | VERIFIED | `projectFlag` parsed from flagset; config `DefaultProject` used as fallback; both present in `runRun` and `runSubmit` |
| 14 | Registry ref without project context produces a clear error | VERIFIED | `TestRunRun_RegistryRef_NoProject` passes; stderr message "registry reference requires --project flag or default_project in config" |

**Score:** 14/14 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/store/sqlite/migrations/00007_run_registry_ref.sql` | Migration adding registry_ref and registry_digest columns | VERIFIED | Contains both `ALTER TABLE runs ADD COLUMN registry_ref TEXT NOT NULL DEFAULT ''` and `registry_digest` |
| `internal/store/store.go` | Run struct with RegistryRef and RegistryDigest fields | VERIFIED | Lines 126-127: `RegistryRef string` and `RegistryDigest string` |
| `internal/store/sqlite/runs.go` | Updated CreateRun INSERT, scanRun, scanRunRows with new columns | VERIFIED | INSERT includes 7 columns; SELECT includes 10 columns; both scan functions include `&r.RegistryRef, &r.RegistryDigest` |
| `internal/api/types.go` | RegistryRef field on SubmitRequest and TaskRunRequest | VERIFIED | Lines 12 and 113: `RegistryRef string json:"registry_ref,omitempty"` on both structs |
| `internal/server/handlers.go` | parseRegistryRef helper, resolveRegistryRef helper, modified handleSubmit, handleTaskRun, handleClaim | VERIFIED | All 5 functions present and wired; `run.RegistryRef != ""` branch in `handleClaim` at line 486 |
| `internal/server/handlers_registry_exec_test.go` | Integration tests for registry-based execution | VERIFIED | 10 named test functions covering all success and error paths |
| `internal/config/config.go` | DefaultProject field on ClientConfig | VERIFIED | Line 106: `DefaultProject string yaml:"default_project"` |
| `internal/client/client.go` | SubmitToProject and TaskRunInProject project-scoped client methods | VERIFIED | Lines 67 and 101: both methods post to `/projects/{projectSlug}/workflows` and `/projects/{projectSlug}/tasks/run` |
| `cmd/flowy/main.go` | isRegistryRef helper, --project flag, registry ref detection in runRun and runSubmit | VERIFIED | `isRegistryRef` at line 677, `isAlpha` at line 693, `--project` flag and detection in both `runRun` and `runSubmit` |
| `cmd/flowy/main_test.go` | Unit tests for isRegistryRef and registry ref integration | VERIFIED | `TestIsRegistryRef`, `TestIsRegistryRef_EdgeCases`, `TestRunRun_RegistryRef_NoProject` all present |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `handlers.go (handleSubmit)` | `store.GetRegistryVersionByTag / GetRegistryVersionByDigest` | `resolveRegistryRef` helper | WIRED | `resolveRegistryRef` called at line 179; internally calls both store methods |
| `handlers.go (handleClaim)` | `store.GetRegistryVersionByDigest` | re-resolve by stored RegistryDigest | WIRED | `run.RegistryRef != ""` branch at line 486; calls `GetRegistryVersionByDigest` at line 489 |
| `store/sqlite/runs.go (CreateRun)` | runs table | INSERT with registry_ref, registry_digest | WIRED | INSERT at line 19 includes both columns; passed as `r.RegistryRef, r.RegistryDigest` |
| `cmd/flowy/main.go (runRun)` | `api.SubmitRequest / api.TaskRunRequest` | `isRegistryRef` sets RegistryRef instead of WDLPath | WIRED | `RegistryRef: registryRef` in both request structs at lines 243 and 278 |
| `cmd/flowy/main.go (runRun)` | `client.SubmitToProject / client.TaskRunInProject` | project-scoped URL when registry ref detected | WIRED | `c.TaskRunInProject` at line 272; `c.SubmitToProject` at line 292 |
| `server.go` | `handleSubmit / handleTaskRun` | project-scoped routes `/projects/{slug}/workflows` and `/projects/{slug}/tasks/run` | WIRED | Lines 143-144 register both routes within project-context middleware chain |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| EXEC-01 | 22-01-PLAN.md, 22-02-PLAN.md | Executor can submit a run by registry reference (`project/name:tag` or `project/name@sha256:hex`) | SATISFIED | Server accepts `registry_ref` on project-scoped routes (22-01); CLI auto-detects refs and routes to project endpoint (22-02); `TestHandleSubmitRegistryRef` and `TestIsRegistryRef` pass |
| EXEC-02 | 22-01-PLAN.md | Server resolves registry ref to WDL content and delivers via existing `WDLContent` path | SATISFIED | `resolveRegistryRef` fetches WDL content from store; `handleClaim` re-resolves by digest and populates `WDLContent` in `TaskClaimResponse`; `TestHandleClaimRegistryRun` confirms non-empty WDLContent |

### Anti-Patterns Found

None. No TODO/FIXME/PLACEHOLDER comments found in modified files. No stub implementations detected.

### Human Verification Required

None. All behaviors are verifiable programmatically and all tests pass.

### Gaps Summary

No gaps. All must-haves verified, all artifacts substantive and wired, all key links confirmed, both EXEC-01 and EXEC-02 satisfied, full test suite passes (22/22 packages).

---

## Test Run Evidence

```
ok  github.com/bioflowy/flowy/cmd/flowy           2.044s
ok  github.com/bioflowy/flowy/internal/client     0.013s
ok  github.com/bioflowy/flowy/internal/config     0.006s
ok  github.com/bioflowy/flowy/internal/server     5.288s
ok  github.com/bioflowy/flowy/internal/store      0.004s
ok  github.com/bioflowy/flowy/internal/store/sqlite 2.764s
```

All 22 packages pass `go test ./... -count=1`.

---

_Verified: 2026-03-20_
_Verifier: Claude (gsd-verifier)_
