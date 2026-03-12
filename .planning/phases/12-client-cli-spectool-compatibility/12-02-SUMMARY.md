---
phase: 12-client-cli-spectool-compatibility
plan: 02
subsystem: cli
tags: [cli, submit, status, outputs, file-staging, uuid, httptest]

# Dependency graph
requires:
  - phase: 12-client-cli-spectool-compatibility
    plan: 01
    provides: internal/client package with Submit/Status/Outputs methods, config.Default(), store.CreateRunDirs/RunInputsDir

provides:
  - flowy submit subcommand: stages WDL and input files to shared FS, submits workflow, prints run ID
  - flowy status subcommand: queries run state, prints plain state string
  - flowy outputs subcommand: retrieves completed run outputs, prints indented JSON
  - stageInputFiles helper: copies local files not under baseDir to inputsDir, preserves filename
  - stageFile helper: copies single file to destDir
  - loadConfigOptional: config loading with fallback to defaults
  - resolveServerURL: --server flag overrides config value

affects: [12-03, 12-04, 12-05]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Submit subcommand: generate UUID -> create staging dirs -> stage WDL -> stage inputs -> submit -> cleanup on error"
    - "File staging: resolve absolute path, os.Stat existence check, strings.HasPrefix(absPath, absBase+sep) for baseDir check"
    - "reorderArgs includes --server, --config, --base-dir as knownValueFlags for spectool compatibility"
    - "TDD: test RED commits compile-failing (undefined function), GREEN adds implementation"

key-files:
  created:
    - cmd/flowy/client_test.go
  modified:
    - cmd/flowy/main.go

key-decisions:
  - "stageInputFiles checks both existence (os.Stat) and location (not under baseDir) before staging — non-existent paths are silently skipped as they may be non-file input values"
  - "--base-dir CLI flag overrides cfg.BaseDir so tests can use t.TempDir() without a real config file"
  - "WDL staging destination is inputsDir (same as input files) with fixed name workflow.wdl — server finds it via WDLPath in SubmitRequest"
  - "Cleanup on error uses os.RemoveAll on the entire runs/{runID} dir, not just inputsDir"

patterns-established:
  - "Submit pattern: pre-staging with client-provided RunID enables idempotent server-side handling"
  - "Config resolution: --base-dir > config file > defaults — lets CLI work without config file on dev workstations"

requirements-completed: [CLI-01, CLI-02, CLI-03, "CLI-05 (staging code later removed per ADR 0001 in plan 12-03)"]

# Metrics
duration: 3min
completed: 2026-03-12
---

# Phase 12 Plan 02: CLI Submit/Status/Outputs Subcommands Summary

**flowy submit/status/outputs subcommands with local-to-shared-FS file staging via uuid-based pre-staging and io.Copy**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-12T01:41:23Z
- **Completed:** 2026-03-12T01:44:15Z
- **Tasks:** 1 (TDD)
- **Files modified:** 2

## Accomplishments

- Implemented `flowy submit <wdl> [-i <inputs>] [--server <url>] [--base-dir <dir>]` with pre-submission file staging: WDL and input files not under baseDir are copied to `{baseDir}/runs/{uuid}/inputs/`, staged paths replace original in SubmitRequest, cleanup on failure
- Note: The file staging logic (stageInputFiles, stageFile) implemented in this plan was later removed in plan 12-03 per ADR 0001. See docs/adr/0001-remove-heuristic-file-staging-from-client.md.
- Implemented `flowy status <run-id> [--server <url>]` printing plain state string (e.g. `RUNNING`) to stdout
- Implemented `flowy outputs <run-id> [--server <url>]` printing indented JSON outputs to stdout; descriptive error for 409 (not yet completed)
- 13 tests covering success paths, error handling, file staging edge cases, and WDL staging; all pass

## Task Commits

Each task was committed atomically:

1. **Task 1 (TDD RED): failing tests** - `e7368ab` (test) — undefined stageInputFiles triggers compile-time failure
2. **Task 1 (TDD GREEN): implementation** - `483578c` (feat) — full implementation, all 13 tests pass

_Note: TDD task has two commits: test (RED) then feat (GREEN)_

## Files Created/Modified

- `cmd/flowy/main.go` - Added runSubmit, runStatus, runOutputs, stageInputFiles, stageFile, loadConfigOptional, resolveServerURL; updated usage() and reorderArgs()
- `cmd/flowy/client_test.go` - 13 tests using httptest.NewServer for mock servers; covers all behavior scenarios

## Decisions Made

- `--base-dir` CLI flag added (not in original spec) so tests can override BaseDir without config file; this is essential for the staging tests to work with `t.TempDir()` directories.
- `stageInputFiles` silently skips non-existent paths (no error) since WDL inputs can contain non-file values like strings, integers serialized as strings, etc.
- WDL file staging copies to `inputsDir/workflow.wdl` (fixed name) to avoid filename collisions; SubmitRequest.WDLPath is updated to the staged path.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added --base-dir flag to submit subcommand**
- **Found during:** Task 1 (writing TestSubmitWithInputsStageFiles)
- **Issue:** The plan specifies `--config` for config file path, but tests need to control BaseDir directly via CLI to use t.TempDir() as the staging root. Without `--base-dir`, tests would need to write real config files to disk.
- **Fix:** Added `--base-dir` flag to `runSubmit` that overrides `cfg.BaseDir` after config loading. Consistent with `--workers` flag pattern in flowy-daemon.
- **Files modified:** `cmd/flowy/main.go`
- **Verification:** TestSubmitWithInputsStageFiles and TestSubmitCleanupOnFailure both use --base-dir and pass.
- **Committed in:** `483578c` (Task 1 GREEN commit)

---

**Total deviations:** 1 auto-fixed (missing critical for testability)
**Impact on plan:** No scope creep — `--base-dir` is a standard override flag that makes the CLI more testable and useful in multi-host setups.

## Issues Encountered

None — plan executed cleanly with one minor deviation for testability.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `flowy submit`, `flowy status`, `flowy outputs` are fully functional and tested
- File staging logic (`stageInputFiles`, `stageFile`) is extracted and unit-tested separately
- Ready for plan 03 (signal handling / `flowy run` server dispatch) and plan 04 (spectool compatibility)

---
*Phase: 12-client-cli-spectool-compatibility*
*Completed: 2026-03-12*
