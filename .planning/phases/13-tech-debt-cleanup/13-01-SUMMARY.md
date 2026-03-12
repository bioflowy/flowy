---
phase: 13-tech-debt-cleanup
plan: 01
subsystem: documentation, refactoring
tags: [tech-debt, wdl, values, code-deduplication, adr, documentation]

# Dependency graph
requires:
  - phase: 12-client-cli-spectool-compatibility
    plan: 03
    provides: ADR 0001 removing heuristic file staging from client CLI
provides:
  - wdl/values/convert.go with shared ValueToInterface and ValuesToJSON functions
  - Phase 12 VERIFICATION.md truths #10 and #11 updated to reflect ADR 0001 state
  - Phase 12-02 SUMMARY CLI-05 annotated with staging removal note
  - store.RunInputsDir and store.RunOutputsDir documented as internal helpers
  - daemon cfg.BaseDir log line clarified with TaskClaimResponse comment
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Shared value conversion: ValueToInterface and ValuesToJSON live in wdl/values package, imported by server and daemon"
    - "store helper functions documented as internal: RunInputsDir/RunOutputsDir marked test-only in doc comments"

key-files:
  created:
    - wdl/values/convert.go
    - wdl/values/convert_test.go
  modified:
    - internal/server/dispatch.go
    - internal/daemon/daemon.go
    - internal/daemon/daemon_test.go
    - internal/store/rundir.go
    - .planning/phases/12-client-cli-spectool-compatibility/12-VERIFICATION.md
    - .planning/phases/12-client-cli-spectool-compatibility/12-02-SUMMARY.md

key-decisions:
  - "ValueToInterface is exported from wdl/values package (not wdl/values/convert sub-package) so existing import paths in server and daemon remain unchanged"
  - "daemon_test.go TestOutputsJSON updated to call values.ValuesToJSON directly (package-level test in package daemon had direct access to removed private function)"

patterns-established:
  - "Single source of truth for WDL-to-JSON conversion: wdl/values.ValueToInterface is the canonical converter for all consumers"

requirements-completed: []

# Metrics
duration: 4min
completed: 2026-03-12
---

# Phase 13 Plan 01: Tech Debt Cleanup Summary

**Deduplicated valueToInterface into wdl/values/convert.go (eliminating identical 50-line functions from server and daemon), updated Phase 12 VERIFICATION.md to reflect ADR 0001 file-staging removal, and added clarifying doc comments to internal store helpers**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-12T06:14:16Z
- **Completed:** 2026-03-12T06:18:13Z
- **Tasks:** 2 (Task 1: docs, Task 2: TDD refactoring)
- **Files modified:** 8

## Accomplishments

- Fixed Phase 12 VERIFICATION.md truths #10 and #11 from VERIFIED (stale) to NOT APPLICABLE, with ADR 0001 cross-reference; updated score from "14/14" to "12/12 verified, 2 not applicable (ADR 0001)"
- Updated Phase 12-02 SUMMARY CLI-05 annotation to note staging code was removed in plan 12-03 per ADR 0001
- Created `wdl/values/convert.go` with exported `ValueToInterface` and `ValuesToJSON`, eliminating two identical 50-line private implementations from server/dispatch.go and daemon/daemon.go
- Added internal-helper doc comments to `store.RunInputsDir` and `store.RunOutputsDir` clarifying they are used by CreateRunDirs and tests only; production code receives paths via TaskClaimResponse
- Added clarifying comment to daemon's cfg.BaseDir log line explaining all task paths come from server via TaskClaimResponse

## Task Commits

Each task was committed atomically:

1. **Task 1: Fix Phase 12 docs to reflect ADR 0001** - `3230bf0` (docs)
2. **Task 2 (TDD RED): Add failing tests** - `61f0f61` (test)
3. **Task 2 (TDD GREEN): Implementation** - `4983604` (feat)

_Note: Task 2 is a TDD task with two commits: test (RED) then feat (GREEN)_

## Files Created/Modified

- `wdl/values/convert.go` - New shared ValueToInterface and ValuesToJSON functions (exported from values package)
- `wdl/values/convert_test.go` - Tests for all value types: primitives, array, struct, pair, map, ValuesToJSON
- `internal/server/dispatch.go` - Removed local valueToInterface (48 lines); both call sites updated to values.ValueToInterface
- `internal/daemon/daemon.go` - Removed local valueToInterface + valuesToJSON (58 lines); updated to values.ValuesToJSON; added cfg.BaseDir clarifying comment
- `internal/daemon/daemon_test.go` - TestOutputsJSON updated to call values.ValuesToJSON
- `internal/store/rundir.go` - Added internal-helper doc comments to RunInputsDir and RunOutputsDir
- `.planning/phases/12-client-cli-spectool-compatibility/12-VERIFICATION.md` - Truths #10 and #11 updated; score updated; key link and CLI-05 marked per ADR 0001
- `.planning/phases/12-client-cli-spectool-compatibility/12-02-SUMMARY.md` - CLI-05 annotation and staging removal note added

## Decisions Made

- `ValueToInterface` is exported from the `wdl/values` package (not a sub-package) so the existing `values` import in server and daemon required no import path changes.
- `daemon_test.go` is in `package daemon` (not `package daemon_test`) so it had direct access to the removed private `valuesToJSON` function. Updated the test to call `values.ValuesToJSON` instead (Rule 1 auto-fix).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] daemon_test.go TestOutputsJSON called removed private valuesToJSON**
- **Found during:** Task 2 (TDD GREEN — implementation)
- **Issue:** daemon_test.go is in `package daemon` and directly called the private `valuesToJSON` function that was removed as part of the consolidation. Build failed with `undefined: valuesToJSON`.
- **Fix:** Updated `TestOutputsJSON` to call `values.ValuesToJSON` directly — the exact same behavior, now using the shared implementation.
- **Files modified:** `internal/daemon/daemon_test.go`
- **Verification:** `go test ./internal/daemon/` passes
- **Committed in:** `4983604` (Task 2 GREEN commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 - bug in test caused by removing private function)
**Impact on plan:** Necessary fix — the test was testing the removed function directly. No scope creep; now tests the canonical shared implementation.

## Issues Encountered

None — plan executed with one minor auto-fix for the daemon test that directly referenced the private function being removed.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- v1.1 milestone tech debt is fully resolved
- `wdl/values.ValueToInterface` is the canonical WDL-to-JSON converter for all future phases
- Phase 12 documentation accurately reflects the post-ADR-0001 codebase state

---
*Phase: 13-tech-debt-cleanup*
*Completed: 2026-03-12*
