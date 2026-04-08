---
phase: 35-run-status-polling
fixed_at: 2026-04-07T00:00:00Z
review_path: .planning/phases/35-run-status-polling/35-REVIEW.md
iteration: 2
findings_in_scope: 4
fixed: 2
skipped: 2
status: partial
---

# Phase 35: Code Review Fix Report

**Fixed at:** 2026-04-07T00:00:00Z
**Source review:** .planning/phases/35-run-status-polling/35-REVIEW.md
**Iteration:** 2

**Summary:**
- Findings in scope: 4 (WR-01, WR-02, WR-03, WR-04)
- Fixed: 2
- Skipped: 2

## Fixed Issues

### WR-01: Run not found renders loading skeleton indefinitely

**Files modified:** `web/src/features/runs/RunDetailPage.tsx`
**Commit:** 55528e2
**Applied fix:** Destructured `isLoading` from `usePollingRuns`. Replaced the single `if (!run)` skeleton guard with two guards: `if (isLoading && !run)` returns the skeleton (only while data is genuinely loading), and a follow-up `if (!run)` returns a "Run not found." message so users see a clear error instead of an indefinite spinner when the run ID is absent from the loaded list.

### WR-03: `formatDuration` produces `NaN` output for invalid date strings

**Files modified:** `web/src/features/runs/RunsPage.tsx`
**Commit:** dc48981
**Applied fix:** Renamed `end` to `rawEnd` and added an `isNaN(start) || isNaN(rawEnd)` guard that returns `—` before any arithmetic. Also added a `totalSeconds < 0` guard (negative duration from clock skew) that likewise returns `—`. This ensures malformed or missing server date strings produce a safe placeholder instead of propagating `NaN` through formatting logic.

## Skipped Issues

### WR-02: Shared `queryKey` between dead `useRuns` and active `usePollingRuns` — latent cache collision

**File:** `web/src/features/runs/useRuns.ts:22` and `web/src/features/runs/usePollingRuns.ts:9`
**Reason:** Skipped — architectural, not a bug. The collision is only possible if `useRuns` is ever called alongside `usePollingRuns` for the same slug. Since `useRuns` is currently unused dead code (IN-01), there is no active user impact. The proper resolution (delete `useRuns` or rename its key) is a refactoring task, not a bug fix for this polling phase.

### WR-04: `formatDuration` imported from `RunsPage` into `RunDetailPage` — cross-page coupling

**File:** `web/src/features/runs/RunDetailPage.tsx:9`
**Reason:** Skipped — architectural, not a bug. Moving `formatDuration` (and `statusBadgeClassName`) to a shared `runUtils.ts` module is the correct long-term fix, but it is a refactoring task. The current import works correctly at runtime and introduces no functional defect in this phase.

---

_Fixed: 2026-04-07T00:00:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 2_
