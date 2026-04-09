---
phase: 35-run-status-polling
fixed_at: 2026-04-07T00:00:00Z
review_path: .planning/phases/35-run-status-polling/35-REVIEW.md
iteration: 1
findings_in_scope: 4
fixed: 4
skipped: 0
status: all_fixed
---

# Phase 35: Code Review Fix Report

**Fixed at:** 2026-04-07T00:00:00Z
**Source review:** .planning/phases/35-run-status-polling/35-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 4
- Fixed: 4
- Skipped: 0

## Fixed Issues

### WR-01: Variable shadowing — `id` (setInterval handle) shadows route param `id`

**Files modified:** `web/src/features/runs/RunDetailPage.tsx`
**Commit:** a97b348
**Applied fix:** Renamed the `setInterval` return value from `id` to `timer` inside the `useEffect` callback, eliminating the shadowing of the `id` route param from `useParams`.

---

### WR-02: Detached anchor element used for file download — unreliable in Firefox

**Files modified:** `web/src/features/runs/RunDetailPage.tsx`
**Commit:** de97100
**Applied fix:** Added `document.body.appendChild(a)` before `a.click()` and `document.body.removeChild(a)` after, ensuring the anchor is attached to the DOM during the click event so Firefox triggers the download correctly.

---

### WR-03: `isPreviewable` produces wrong `ext` for paths with no file extension

**Files modified:** `web/src/features/runs/RunDetailPage.tsx`
**Commit:** 0b3501f
**Applied fix:** Added an early-return guard (`if (dotIndex === -1) return false`) in `isPreviewable` so that extensionless paths are correctly rejected rather than relying on accidental correctness from `path.slice(-1)`.

---

### WR-04: Polling stops permanently when runs list is empty — new runs never appear

**Files modified:** `web/src/features/runs/usePollingRuns.ts`, `web/src/features/runs/__tests__/usePollingRuns.test.ts`
**Commit:** 4a0e13c
**Applied fix:** Changed `refetchInterval` to return `5000` (instead of `false`) when `data` is undefined or an empty array. This keeps polling active so runs submitted after the page load appear automatically. Updated the two affected test cases in `usePollingRuns.test.ts` to expect `5000` instead of `false` for the undefined and empty-array cases.

---

_Fixed: 2026-04-07T00:00:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
