---
phase: 35-run-status-polling
plan: "02"
subsystem: web/runs
tags: [polling, react-query, hooks, typescript, vitest]
dependency_graph:
  requires:
    - phase: 35-01
      provides: usePollingRuns hook and TERMINAL_STATES constant
  provides:
    - RunsPage wired to usePollingRuns with pulsing dot for active runs
    - RunDetailPage wired to usePollingRuns with pulsing dot and 1-second duration tick
  affects:
    - web/src/features/runs/RunsPage.tsx
    - web/src/features/runs/RunDetailPage.tsx
tech_stack:
  added: []
  patterns:
    - usePollingRuns swap pattern — replace useRuns with usePollingRuns to get auto-refresh
    - Pulsing dot via aria-label="polling active" and animate-pulse for non-terminal states
    - 1-second tick via setInterval + useState in useEffect gated on run.state === RUNNING
key_files:
  created: []
  modified:
    - web/src/features/runs/RunsPage.tsx
    - web/src/features/runs/RunDetailPage.tsx
    - web/src/features/runs/__tests__/RunsPage.test.tsx
    - web/src/features/runs/__tests__/RunDetailPage.test.tsx
key_decisions:
  - "Pulsing dot uses aria-label for testability — getByLabelText('polling active') works in both Vitest and user testing"
  - "TERMINAL_STATES imported directly in each page for dot visibility logic — no wrapper needed"
  - "1-second tick state only triggers re-render; formatDuration reads Date.now() directly so no extra state needed for actual time value"
patterns_established:
  - "Pulsing dot: aria-label='polling active' span with animate-pulse class, hidden when TERMINAL_STATES.has(run.state)"
  - "Duration tick: const [, setTick] = useState(0) + useEffect with setInterval(1000) gated on run.state"
requirements_completed:
  - EXEC-04
duration: 12min
completed: "2026-04-07"
---

# Phase 35 Plan 02: RunsPage and RunDetailPage Polling Integration Summary

**RunsPage and RunDetailPage wired to usePollingRuns with pulsing yellow dot for PENDING/RUNNING states and 1-second duration tick for RUNNING runs on detail page.**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-04-07T01:05:00Z
- **Completed:** 2026-04-07T01:17:00Z
- **Tasks:** 2 of 3 complete (Task 3 is human-verify checkpoint)
- **Files modified:** 4

## Accomplishments

- RunsPage now uses usePollingRuns — auto-refreshes every 3 seconds while any visible run is non-terminal, stops automatically when all runs reach terminal state
- RunDetailPage now uses usePollingRuns and shows a pulsing yellow dot next to the status badge for PENDING and RUNNING runs; dot disappears on COMPLETED, FAILED, ABORTED
- RunDetailPage has a 1-second duration tick via setInterval so the duration counter updates smoothly for RUNNING runs rather than jumping on every 3-second poll
- Tests updated: 25 tests pass across usePollingRuns (8), RunsPage (7), RunDetailPage (10) — added dot visibility tests for RUNNING, PENDING, COMPLETED states in both pages
- TypeScript compiles cleanly

## Task Commits

1. **Task 1: Wire polling and pulsing dot into RunsPage and RunDetailPage** - `3a152c4` (feat)
2. **Task 2: Extend page tests for polling indicator and hook swap** - `fda87ae` (test)
3. **Task 3: Visual verification** - checkpoint:human-verify (pending human)

## Files Created/Modified

- `web/src/features/runs/RunsPage.tsx` - Swapped useRuns→usePollingRuns, added TERMINAL_STATES import, added pulsing dot in status TableCell
- `web/src/features/runs/RunDetailPage.tsx` - Swapped useRuns→usePollingRuns, added TERMINAL_STATES import, added useEffect tick, added pulsing dot in metadata header
- `web/src/features/runs/__tests__/RunsPage.test.tsx` - Added usePollingRuns mock, swapped setupMocks to use it, added 2 pulsing dot tests (7 total)
- `web/src/features/runs/__tests__/RunDetailPage.test.tsx` - Added usePollingRuns mock, swapped setupMocks to use it, added 3 pulsing dot tests (10 total)

## Decisions Made

- Pulsing dot uses `aria-label="polling active"` for testability — `getByLabelText` works cleanly in both Vitest and real browser accessibility tooling
- TERMINAL_STATES imported directly in each page; no need for a wrapper helper
- 1-second tick state (`const [, setTick] = useState(0)`) triggers re-render only — `formatDuration` reads `Date.now()` internally so no additional value needs to be tracked

## Deviations from Plan

None - plan executed exactly as written.

## Known Stubs

None - both pages are fully wired to real API calls via usePollingRuns/apiClient.

## Threat Flags

None - no new trust boundaries introduced. Pages use existing authenticated API via usePollingRuns (identical queryKey to useRuns).

## Issues Encountered

None.

## Next Phase Readiness

- EXEC-04 (Poll-based run status monitoring in browser) is complete pending human visual verification
- Task 3 checkpoint requires human to verify: pulsing dot visibility, 3-second poll auto-refresh, 1-second duration tick, polling stops on terminal state, background tab pause behavior
- No blockers for subsequent phases

---
*Phase: 35-run-status-polling*
*Completed: 2026-04-07*
