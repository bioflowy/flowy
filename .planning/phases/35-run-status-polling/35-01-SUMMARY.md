---
phase: 35-run-status-polling
plan: "01"
subsystem: web/runs
tags: [polling, react-query, hooks, typescript]
dependency_graph:
  requires: []
  provides:
    - usePollingRuns hook with auto-stop polling logic
    - TERMINAL_STATES constant for reuse in Plan 02
  affects:
    - web/src/features/runs/useRuns.ts
    - web/src/features/runs/usePollingRuns.ts
tech_stack:
  added: []
  patterns:
    - TanStack Query v5 refetchInterval callback pattern (query.state.data access)
    - Mock-and-capture test strategy for hook options
key_files:
  created:
    - web/src/features/runs/usePollingRuns.ts
    - web/src/features/runs/__tests__/usePollingRuns.test.ts
  modified:
    - web/src/features/runs/useRuns.ts
decisions:
  - TERMINAL_STATES exported as a Set from useRuns.ts for O(1) lookup and reuse in Plan 02
  - refetchInterval uses query.state.data (TanStack Query v5 API), not data parameter directly
  - Tests mock useQuery and capture passed options rather than rendering with QueryClient to avoid fake timer complexity
metrics:
  duration_minutes: 10
  completed_date: "2026-04-07"
  tasks_completed: 2
  tasks_total: 2
  files_created: 2
  files_modified: 1
---

# Phase 35 Plan 01: usePollingRuns Hook and TERMINAL_STATES Summary

**One-liner:** Polling-enabled runs query hook with 3s refetchInterval for active runs and auto-stop on terminal state, sharing cache with existing useRuns via identical queryKey.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Export TERMINAL_STATES and create usePollingRuns hook | 6ee3124 | useRuns.ts, usePollingRuns.ts |
| 2 | Create usePollingRuns unit tests | cf49c72 | __tests__/usePollingRuns.test.ts |

## What Was Built

### TERMINAL_STATES constant (useRuns.ts)
Added `export const TERMINAL_STATES = new Set(['COMPLETED', 'FAILED', 'ABORTED'])` above `useRuns` function. Uses a `Set` for O(1) membership testing. Exported for reuse by Plan 02 consumers.

### usePollingRuns hook (usePollingRuns.ts)
- Same `queryKey` as `useRuns` — React Query cache deduplication means no duplicate network requests if both are mounted simultaneously
- `refetchInterval` callback uses TanStack Query v5 signature: receives `query` object, accesses `query.state.data` (not a `data` parameter)
- Returns `3000` (3 seconds) when any run has a non-terminal state
- Returns `false` when data is undefined, empty, or all runs are terminal
- `refetchIntervalInBackground: false` pauses polling when browser tab is hidden

### usePollingRuns.test.ts
8 test cases covering:
- TERMINAL_STATES constant membership and size
- refetchInterval=3000 for RUNNING and PENDING states
- refetchInterval=false for all-COMPLETED, mixed-terminal, undefined data, empty array
- refetchIntervalInBackground=false option is passed

Test strategy: mock `useQuery` via `vi.mock`, capture the options object passed to it, then call the `refetchInterval` callback directly. Avoids fake timer complexity entirely.

## Verification

- `npx tsc --noEmit`: passes (0 errors)
- `npx vitest run src/features/runs/`: 20 tests pass (8 new + 12 existing unchanged)

## Deviations from Plan

None - plan executed exactly as written.

## Known Stubs

None - the hook is fully wired to apiClient and does not contain placeholder data.

## Threat Flags

None - no new trust boundaries introduced. Hook wraps existing authenticated API endpoint with identical queryKey.

## Self-Check: PASSED

- [x] web/src/features/runs/usePollingRuns.ts exists
- [x] web/src/features/runs/__tests__/usePollingRuns.test.ts exists
- [x] web/src/features/runs/useRuns.ts contains TERMINAL_STATES export
- [x] Commits 6ee3124 and cf49c72 exist in git log
- [x] All 20 tests pass
- [x] TypeScript compiles cleanly
