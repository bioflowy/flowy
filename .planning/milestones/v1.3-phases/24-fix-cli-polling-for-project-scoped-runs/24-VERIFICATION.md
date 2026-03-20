---
phase: 24-fix-cli-polling-for-project-scoped-runs
verified: 2026-03-20T06:00:00Z
status: passed
score: 4/4 must-haves verified
re_verification: false
---

# Phase 24: Fix CLI Polling for Project-Scoped Runs Verification Report

**Phase Goal:** CLI can poll run status and retrieve outputs after project-scoped or registry-ref submission when auth is enabled
**Verified:** 2026-03-20T06:00:00Z
**Status:** passed
**Re-verification:** No -- initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | CLI can poll run status after project-scoped submission with auth enabled | VERIFIED | `StatusInProject` method exists at line 163 of `internal/client/client.go`, calls `/projects/{slug}/runs/{id}/status`. `pollUntilDone` at line 348 of `cmd/flowy/main.go` conditionally calls `StatusInProject` when `projectSlug != ""`. 5 client tests pass. |
| 2 | CLI can retrieve outputs after project-scoped run completes with auth enabled | VERIFIED | `OutputsInProject` method exists at line 192 of `internal/client/client.go`, calls `/projects/{slug}/runs/{id}/outputs` with 200/409/404 handling. `runRun` at line 317 conditionally calls `OutputsInProject` when `projectSlug != ""`. |
| 3 | Full E2E flow works: submit by registry ref -> poll -> outputs with auth enabled | VERIFIED | `TestRunRegistryRefProjectScoped` in `cmd/flowy/run_test.go` (line 407) exercises the full flow: submit to `/projects/test-proj/workflows`, poll via `/projects/test-proj/runs/{id}/status`, fetch via `/projects/test-proj/runs/{id}/outputs`. Test also asserts non-project routes are NOT called (atomic counters verify isolation). Test passes. |
| 4 | Existing non-project (auth-disabled) polling and output retrieval remain unchanged | VERIFIED | Original `Status()` and `Outputs()` methods unchanged at lines 135 and 230 of `client.go`. `pollUntilDone` falls back to `c.Status()` when `projectSlug == ""`. `runRun` falls back to `c.Outputs()` when `projectSlug == ""`. All existing tests pass (`go test ./cmd/flowy/ -count=1` and `go test ./internal/client/ -count=1`). |

**Score:** 4/4 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/client/client.go` | StatusInProject method | VERIFIED | `func (c *Client) StatusInProject` at line 163, hits `/projects/{slug}/runs/{id}/status` |
| `internal/client/client.go` | OutputsInProject method | VERIFIED | `func (c *Client) OutputsInProject` at line 192, hits `/projects/{slug}/runs/{id}/outputs` with 200/409/404 handling |
| `internal/client/client_test.go` | Tests for StatusInProject and OutputsInProject | VERIFIED | 5 tests: `TestStatusInProject_Success`, `TestStatusInProject_NotFound`, `TestOutputsInProject_Success`, `TestOutputsInProject_NotCompleted`, `TestOutputsInProject_NotFound` -- all pass |
| `cmd/flowy/main.go` | pollUntilDone and runRun using project-scoped methods | VERIFIED | `pollUntilDone` accepts `projectSlug` param (line 348), conditional routing at line 361. `runRun` conditional outputs at line 317. |
| `cmd/flowy/run_test.go` | TestRunRegistryRefProjectScoped | VERIFIED | Test at line 407, verifies full project-scoped flow with non-project route isolation assertion |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `cmd/flowy/main.go pollUntilDone` | `client.StatusInProject` | conditional call when projectSlug non-empty | WIRED | Line 362: `resp, err = c.StatusInProject(ctx, projectSlug, runID)` |
| `cmd/flowy/main.go runRun` | `client.OutputsInProject` | conditional call when projectSlug non-empty | WIRED | Line 318: `outputsResp, err = c.OutputsInProject(ctx, projectSlug, submittedRunID)` |
| `client.StatusInProject` | server `/projects/{slug}/runs/{id}/status` | HTTP GET | WIRED | URL constructed at line 164: `"/projects/" + projectSlug + "/runs/" + runID + "/status"` |
| `client.OutputsInProject` | server `/projects/{slug}/runs/{id}/outputs` | HTTP GET | WIRED | URL constructed at line 193: `"/projects/" + projectSlug + "/runs/" + runID + "/outputs"` |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| EXEC-01 | 24-01-PLAN.md | Executor can submit a run by registry reference | SATISFIED | Full project-scoped submit->poll->outputs flow verified. `SubmitToProject` + `StatusInProject` + `OutputsInProject` chain is wired and tested. `TestRunRegistryRefProjectScoped` exercises the complete E2E path. |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (none) | - | - | - | No anti-patterns found in modified files |

### Human Verification Required

### 1. Live E2E with Auth Server

**Test:** Start flowy-server with auth enabled, create a project, push a WDL, then run `flowy run myworkflow:latest --project myproj --server http://localhost:8124` and observe polling and output retrieval.
**Expected:** CLI prints state transitions on stderr, then outputs JSON on stdout. No 404 errors during polling.
**Why human:** Requires a running server with auth, project, and registry data. Unit tests mock the HTTP layer but do not exercise real server route matching and middleware.

### Gaps Summary

No gaps found. All four observable truths are verified with concrete evidence. Both new client methods exist, are substantive (not stubs), and are wired into the CLI via conditional routing. The test suite covers the project-scoped flow and verifies isolation from non-project routes. Commits `ebf7ab4` and `295f508` are present in the repository. Build and vet are clean.

---

_Verified: 2026-03-20T06:00:00Z_
_Verifier: Claude (gsd-verifier)_
