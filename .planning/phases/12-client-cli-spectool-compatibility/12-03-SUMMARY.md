---
phase: 12-client-cli-spectool-compatibility
plan: "03"
subsystem: cli
tags: [go, httptest, client, polling, sigint]

requires:
  - phase: 12-client-cli-spectool-compatibility-01
    provides: internal/client.Client with Submit/Status/Outputs/TaskRun methods
  - phase: 12-client-cli-spectool-compatibility-02
    provides: stageInputFiles, stageFile, runSubmit, runStatus, runOutputs in cmd/flowy/main.go

provides:
  - "Server-based runRun: stage WDL+inputs, submit, pollUntilDone, print outputs"
  - "pollUntilDone helper with 2s ticker, state-change logging, quiet mode"
  - "--task flag routes through POST /tasks/run (server-side task dispatch)"
  - "signal.NotifyContext for SIGINT/SIGTERM clean exit"
  - "httptest-based run tests replacing old mockRunner pattern"

affects:
  - spectool-integration
  - e2e-testing

tech-stack:
  added: []
  patterns:
    - "pollUntilDone: ticker-based polling with transient error resilience and quiet mode"
    - "signal.NotifyContext as root context for all HTTP calls — Ctrl+C cancels in-flight requests"
    - "TDD: write failing httptest tests first, then implement to GREEN"

key-files:
  created: []
  modified:
    - cmd/flowy/main.go
    - cmd/flowy/run_test.go
    - cmd/flowy/verbose_test.go

key-decisions:
  - "--verbose accepted as no-op with note 'not yet supported in server mode' — avoids breaking spectool invocations"
  - "pollUntilDone uses 2s ticker (not long-poll) for simplicity; transient poll failures print warning and continue"
  - "Both --task and workflow modes share the same poll loop and output fetch path"
  - "reorderArgs knownValueFlags updated: removed --dir (dropped), kept all server flags"

patterns-established:
  - "runRun flag parsing: -i and --input as aliases; reorderArgs handles spectool positional arg order"
  - "Cleanup on error: os.RemoveAll staged run dir when submit fails"

requirements-completed: [CLI-04, CLI-06]

duration: 30min
completed: 2026-03-12
---

# Phase 12 Plan 03: Replace runRun with Server-Based Implementation Summary

**Server-dispatched `flowy run` that stages WDL+inputs, submits to flowy-server, polls status with state-change logging, and prints JSON outputs on COMPLETED — replacing all local Docker execution.**

## Performance

- **Duration:** ~30 min
- **Started:** 2026-03-12T02:00:00Z
- **Completed:** 2026-03-12T02:30:00Z
- **Tasks:** 1 (TDD: RED → GREEN)
- **Files modified:** 3

## Accomplishments

- Replaced local Docker execution (`newRunner`, `setupExecDir`, `printVerboseOutput`, `randomHex`) with server-based `runRun`
- Implemented `pollUntilDone` helper: 2s ticker, prints `run <short-id>: STATE` on state changes, `--quiet` suppresses output, transient errors print warning and continue
- Signal context via `signal.NotifyContext` — Ctrl+C cancels HTTP requests cleanly
- `--task` flag submits via `POST /tasks/run`; both modes share the same poll+outputs path
- `--verbose` accepted as no-op with descriptive note (spectool compatibility)
- Rewrote `run_test.go` with 13 `httptest`-based tests; rewrote `verbose_test.go` for server mode
- All 33 `cmd/flowy` tests pass; `go test ./...` passes across 19 packages

## Task Commits

1. **Task 1: Replace runRun with server-based implementation** — `899fa60` (feat)

**Plan metadata:** _(pending final metadata commit)_

## Files Created/Modified

- `/home/uehara/flowy/cmd/flowy/main.go` — Replaced runRun with server-based implementation; added pollUntilDone; removed setupExecDir, printVerboseOutput, randomHex, newRunner
- `/home/uehara/flowy/cmd/flowy/run_test.go` — Complete rewrite with httptest mock servers
- `/home/uehara/flowy/cmd/flowy/verbose_test.go` — Rewritten for server mode (removed verboseMockRunner)

## Decisions Made

- `--verbose` accepted as no-op with note to stderr: "note: --verbose not yet supported in server mode" — avoids breaking spectool invocations that pass `--verbose`
- `pollUntilDone` uses `time.NewTicker(2s)` for simplicity; transient network errors print warning and continue polling (resilient to momentary connectivity drops)
- `--dir` flag removed entirely (server controls execution directories, not client)
- Both `--task` and workflow mode use identical poll + outputs fetch logic

## Deviations from Plan

None — plan executed exactly as written. The `verbose_test.go` rewrite was included in scope as the file used `runArgsWithRunner` which was removed.

## Issues Encountered

None. The `verbose_test.go` file referenced `runArgsWithRunner` which was removed as part of this plan, so it needed to be updated alongside `run_test.go`. This was expected per the plan's action section.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `flowy run` is now fully server-based and spectool-compatible
- All three binaries (flowy, flowy-server, flowy-daemon) build from `go build ./...`
- Phase 12 is now complete — all client CLI + spectool compatibility work done

---
*Phase: 12-client-cli-spectool-compatibility*
*Completed: 2026-03-12*
