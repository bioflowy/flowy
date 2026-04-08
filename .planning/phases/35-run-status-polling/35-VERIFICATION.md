---
phase: 35-run-status-polling
verified: 2026-04-07T01:30:00Z
status: human_needed
score: 10/12
overrides_applied: 0
gaps: []
human_verification:
  - test: "Open RunDetailPage for an already-completed run and observe network requests"
    expected: "After the initial page load fetch, no further polling requests are issued (no repeated GET /projects/{slug}/runs requests in the Network tab)"
    why_human: "The code returns 5000ms for empty/undefined data (before first fetch completes), then stops when completed runs are returned. Whether this constitutes 'never triggers polling' per ROADMAP SC4 requires human judgement about the brief initial poll window."
  - test: "Confirm pulsing dot disappears and polling stops after run completes"
    expected: "Pulsing yellow dot vanishes from status badge; network tab shows no further GET requests to /runs after the run state transitions to COMPLETED/FAILED/ABORTED"
    why_human: "Visual and network behavior can only be confirmed in a running browser environment with live API responses."
---

# Phase 35: Run Status Polling — Verification Report

**Phase Goal:** Browser users see live run status without manual page refresh, and polling stops automatically when a run reaches a terminal state
**Verified:** 2026-04-07T01:30:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

**ROADMAP Success Criteria (primary contract):**

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| SC1 | RunDetailPage polls run status every 3 seconds while PENDING or RUNNING | VERIFIED | `usePollingRuns` returns `3000` when any run is non-terminal; `RunDetailPage` uses `usePollingRuns`; confirmed by test `returns 3000 when any run is RUNNING/PENDING` |
| SC2 | Polling stops automatically when run reaches COMPLETED, FAILED, or ABORTED | VERIFIED | `refetchInterval` returns `false` when all runs are in `TERMINAL_STATES`; test confirms |
| SC3 | RunsPage auto-refreshes while any run is non-terminal, then stops | VERIFIED | `RunsPage` uses `usePollingRuns`; same refetchInterval logic applies to the list |
| SC4 | Opening RunDetailPage for an already-completed run never triggers polling | UNCERTAIN | After initial data load, completed-run detail page returns `false` and polling stops. However, `usePollingRuns` returns 5000ms for empty/undefined data (before first fetch resolves). Whether this constitutes a poll is ambiguous — needs human confirmation |

**PLAN must_haves (supplemental):**

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| P1 | usePollingRuns returns refetchInterval 3000 when any run is PENDING or RUNNING | VERIFIED | `usePollingRuns.ts` line 16: `return data.some((r) => !TERMINAL_STATES.has(r.state)) ? 3000 : false` |
| P2 | usePollingRuns returns refetchInterval false when all runs are in terminal state | VERIFIED | Same expression — returns `false` when no non-terminal runs |
| P3 | usePollingRuns returns refetchInterval false when data is undefined | DEVIATED | Returns `5000` instead of `false` when data is undefined or empty array. Intentional change per commit `4a0e13c` ("WR-04 keep polling when runs list is empty so new runs appear automatically") |
| P4 | TERMINAL_STATES constant is exported from useRuns.ts | VERIFIED | `useRuns.ts` line 18: `export const TERMINAL_STATES = new Set(['COMPLETED', 'FAILED', 'ABORTED'])` |
| P5 | RunsPage auto-refreshes while any visible run is non-terminal | VERIFIED | `RunsPage.tsx` uses `usePollingRuns`; pulsing dot shown for non-terminal states |
| P6 | RunsPage stops refreshing when all visible runs are terminal | VERIFIED | Polling logic returns `false` when all runs are terminal |
| P7 | RunDetailPage polls while viewed run is PENDING or RUNNING | VERIFIED | `RunDetailPage.tsx` uses `usePollingRuns`; pulsing dot shown |
| P8 | RunDetailPage stops polling when run reaches COMPLETED, FAILED, or ABORTED | VERIFIED | refetchInterval returns `false` for all-terminal data |
| P9 | Pulsing dot appears for PENDING and RUNNING runs | VERIFIED | `aria-label="polling active"` span with `animate-pulse` rendered when `!TERMINAL_STATES.has(run.state)` in both pages |
| P10 | No pulsing dot for COMPLETED, FAILED, ABORTED | VERIFIED | Dot conditionally rendered only when state is non-terminal; test: `does not show pulsing dot for COMPLETED run` passes in both pages |
| P11 | RunDetailPage duration timer updates every 1 second for RUNNING runs | VERIFIED | `setInterval(() => setTick((n) => n + 1), 1000)` gated on `run?.state !== 'RUNNING'` in `RunDetailPage.tsx` lines 88-93 |
| P12 | Opening RunDetailPage for an already-completed run never triggers polling | UNCERTAIN | See SC4 above — ambiguous due to 5000ms fallback for initial empty data |

**Score:** 10/12 truths verified (2 uncertain, deferred to human verification)

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `web/src/features/runs/usePollingRuns.ts` | Polling-enabled runs hook | VERIFIED | 20 lines, exports `usePollingRuns`, imports `TERMINAL_STATES` from `useRuns`, uses TanStack Query v5 callback pattern |
| `web/src/features/runs/useRuns.ts` | TERMINAL_STATES export | VERIFIED | Line 18: `export const TERMINAL_STATES = new Set(['COMPLETED', 'FAILED', 'ABORTED'])` |
| `web/src/features/runs/__tests__/usePollingRuns.test.ts` | Hook unit tests (min 40 lines) | VERIFIED | 89 lines, 8 test cases in 2 describe blocks |
| `web/src/features/runs/RunsPage.tsx` | Polling-enabled runs list with pulsing dots | VERIFIED | Uses `usePollingRuns`, contains `animate-pulse` span with `aria-label="polling active"` |
| `web/src/features/runs/RunDetailPage.tsx` | Polling-enabled run detail with pulsing dot and 1s duration tick | VERIFIED | Uses `usePollingRuns`, contains setInterval tick, contains pulsing dot |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `usePollingRuns.ts` | `useRuns.ts` | import TERMINAL_STATES, RunListItem | WIRED | Line 3-4: `import type { RunListItem }` and `import { TERMINAL_STATES }` from `@/features/runs/useRuns` |
| `RunsPage.tsx` | `usePollingRuns.ts` | import usePollingRuns | WIRED | Line 18: `import { usePollingRuns } from '@/features/runs/usePollingRuns'`; used at line 61: `usePollingRuns(slug ?? '')` |
| `RunDetailPage.tsx` | `usePollingRuns.ts` | import usePollingRuns | WIRED | Line 7: `import { usePollingRuns } from '@/features/runs/usePollingRuns'`; used at line 76: `usePollingRuns(slug ?? '')` |
| `RunDetailPage.tsx` | `useRuns.ts` | import TERMINAL_STATES | WIRED | Line 6: `import { useRunOutputs, TERMINAL_STATES } from '@/features/runs/useRuns'`; used at line 307 |

---

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `RunsPage.tsx` | `runs` from `usePollingRuns` | `apiClient.get('/projects/{slug}/runs')` | Yes — real API call via `apiClient` | FLOWING |
| `RunDetailPage.tsx` | `run` from `usePollingRuns` + find by id | Same API call as above | Yes — real API call | FLOWING |
| `RunDetailPage.tsx` | `setTick` | `setInterval(..., 1000)` gated on `run.state === 'RUNNING'` | Yes — timer-driven re-render | FLOWING |

---

### Behavioral Spot-Checks

| Behavior | Check | Result | Status |
|----------|-------|--------|--------|
| All 25 tests pass | `npx vitest run src/features/runs/` | 25 passed (3 test files) | PASS |
| TypeScript compiles | `npx tsc --noEmit` | 0 errors | PASS |
| `TERMINAL_STATES` exported | `grep "export const TERMINAL_STATES" useRuns.ts` | Found at line 18 | PASS |
| `usePollingRuns` uses correct TanStack v5 pattern | `grep "query.state.data" usePollingRuns.ts` | Found at line 13 | PASS |
| Pulsing dot in both pages | `grep "aria-label=\"polling active\"" RunsPage.tsx RunDetailPage.tsx` | Found in both | PASS |
| Duration tick uses 1000ms interval | `grep "setInterval.*1000" RunDetailPage.tsx` | Found at line 91 | PASS |

---

### Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
|-------------|-------------|-------------|--------|---------|
| EXEC-04 | 35-01, 35-02 | User can monitor running workflow status via poll-based updates (auto-stop on terminal state) | VERIFIED | `usePollingRuns` polls at 3000ms for active runs, stops on terminal state; both pages wired; pulsing dot visual; tests pass |

No orphaned requirements for Phase 35 — REQUIREMENTS.md maps only EXEC-04 to Phase 35, and both plans claim it.

---

### Anti-Patterns Found

| File | Pattern | Severity | Impact |
|------|---------|----------|--------|
| `usePollingRuns.ts:14` | Returns `5000` (not `false`) when data is undefined/empty | INFO | Intentional deviation from PLAN truth — ensures new runs appear without manual refresh. Not a stub. Does not conflict with ROADMAP SC2. Marginally conflicts with strict reading of SC4 (initial poll before data loads). |

No placeholder, TODO, or empty implementation patterns found in any phase 35 files.

---

### Human Verification Required

#### 1. Polling stops after run completes (visual + network)

**Test:** In a browser with DevTools Network tab open, navigate to a project's Runs page while a workflow is running. Observe the pulsing yellow dot and repeated GET requests to `/projects/{slug}/runs`. Wait for the run to complete. Verify the pulsing dot disappears and no further GET requests are made.

**Expected:** Pulsing dot disappears immediately after state changes to COMPLETED/FAILED/ABORTED. No further polling requests appear in the network tab after that point.

**Why human:** Requires live API responses and browser timing to observe; cannot be verified with static mocks.

#### 2. Already-completed run detail — initial polling behavior

**Test:** Navigate directly to a RunDetailPage for a run that is already COMPLETED. Observe the Network tab for the first 10 seconds.

**Expected:** One initial GET request is made to load the run list. After that, no further requests occur (because all runs are in terminal state). The brief 5000ms fallback for the initial empty-data state should not cause perceptible repeated requests for a completed run.

**Why human:** The code returns 5000ms for the initial empty/undefined data window, then `false` once data arrives. Whether this constitutes "never triggers polling" per ROADMAP SC4 depends on whether the initial data load counts as a "poll" — a human judgement call requiring observation in a real browser.

---

### Gaps Summary

No automated gaps found. Two PLAN truths deviated from their original wording (returns 5000 instead of false for empty/undefined data), but this was an intentional post-plan fix documented in commit `4a0e13c`. The deviation does not violate ROADMAP SC1, SC2, or SC3. SC4 requires human confirmation.

All 25 tests pass. TypeScript compiles cleanly. All five required artifacts exist and are substantively implemented. All four key links are wired. Data flows from real API calls through both pages.

---

_Verified: 2026-04-07T01:30:00Z_
_Verifier: Claude (gsd-verifier)_
