---
phase: 35-run-status-polling
reviewed: 2026-04-07T00:00:00Z
depth: standard
files_reviewed: 7
files_reviewed_list:
  - web/src/features/runs/RunDetailPage.tsx
  - web/src/features/runs/RunsPage.tsx
  - web/src/features/runs/__tests__/RunDetailPage.test.tsx
  - web/src/features/runs/__tests__/RunsPage.test.tsx
  - web/src/features/runs/__tests__/usePollingRuns.test.ts
  - web/src/features/runs/usePollingRuns.ts
  - web/src/features/runs/useRuns.ts
findings:
  critical: 0
  warning: 4
  info: 3
  total: 7
status: issues_found
---

# Phase 35: Code Review Report

**Reviewed:** 2026-04-07T00:00:00Z
**Depth:** standard
**Files Reviewed:** 7
**Status:** issues_found

## Summary

This phase introduces run-status polling via `usePollingRuns` and wires it into `RunDetailPage` and `RunsPage`. The polling hook is well-structured: it uses TanStack Query's `refetchInterval` callback to adapt polling frequency based on run states, stops when all runs reach terminal states, and avoids unnecessary background traffic via `refetchIntervalInBackground: false`. Previously reported issues (variable shadowing, detached anchor element, `isPreviewable` off-by-one, and the polling dead-zone on empty list) have all been fixed in the current code.

Four new warnings were found: an indefinite loading skeleton when a run ID is not present in the polled list; a shared `queryKey` collision between the dead `useRuns` hook and the active `usePollingRuns` hook; a `NaN`-producing code path in `formatDuration` for invalid date strings; and a cross-page import that couples two independent page components through a utility function. Three informational items cover dead code and a missing explanatory comment on the tick timer.

No critical (security or data-loss) issues were found.

---

## Warnings

### WR-01: Run not found renders loading skeleton indefinitely

**File:** `web/src/features/runs/RunDetailPage.tsx:76-139`

**Issue:** `run` is derived by searching the polled list: `runs?.find((r) => r.id === id)`. If the run ID from the URL does not match any item in the list — because polling has not yet returned data, the run was deleted, or the URL is malformed — the `!run` guard at line 134 falls through to a skeleton placeholder that never resolves. There is no timeout, no error message, and no distinction between "still loading" and "loaded but not found."

**Fix:** Distinguish loading state from a genuine not-found condition:

```tsx
const { data: runs, isLoading } = usePollingRuns(slug ?? '')
const run = runs?.find((r) => r.id === id)

if (isLoading && !run) {
  return (
    <div className="p-6">
      <div className="h-4 w-32 animate-pulse rounded bg-muted" />
    </div>
  )
}

if (!run) {
  return (
    <div className="p-6">
      <p className="text-muted-foreground text-sm">Run not found.</p>
    </div>
  )
}
```

---

### WR-02: Shared `queryKey` between dead `useRuns` and active `usePollingRuns` — latent cache collision

**File:** `web/src/features/runs/useRuns.ts:22` and `web/src/features/runs/usePollingRuns.ts:9`

**Issue:** Both hooks register the identical query key `['projects', slug, 'runs']`. TanStack Query deduplicates cache entries by key, so if any component ever calls `useRuns` alongside `usePollingRuns` for the same slug, the query registered second will silently override the first's configuration — including whether `refetchInterval` is active. Since `useRuns` has no `refetchInterval`, this could silently disable polling. The fact that `useRuns` is currently unused (see IN-01) mitigates this in practice, but the duplicate key is a trap for future developers.

**Fix:** Either delete `useRuns` entirely (preferred — see IN-01), or assign it a distinct query key if a non-polling variant is intentionally needed:

```ts
// In useRuns.ts — only if the hook must be retained:
queryKey: ['projects', slug, 'runs', 'snapshot'],
```

---

### WR-03: `formatDuration` produces `NaN` output for invalid date strings

**File:** `web/src/features/runs/RunsPage.tsx:36-46`

**Issue:** `new Date(createdAt).getTime()` returns `NaN` for any non-ISO string. Arithmetic with `NaN` propagates through all subsequent operations, and comparisons like `NaN < 60` evaluate to `false`, so the function falls through to return a string such as `"NaNh NaNm"`. The `RunListItem` interface types `created_at` and `updated_at` as `string` with no runtime validation, so malformed server data reaches this function unchecked.

**Fix:** Guard against invalid dates before doing arithmetic:

```ts
export function formatDuration(createdAt: string, updatedAt: string, state: string): string {
  if (state === 'PENDING') return '\u2014'

  const start = new Date(createdAt).getTime()
  const rawEnd = state === 'RUNNING' ? Date.now() : new Date(updatedAt).getTime()

  if (isNaN(start) || isNaN(rawEnd)) return '\u2014'

  const totalSeconds = Math.floor((rawEnd - start) / 1000)
  if (totalSeconds < 0) return '\u2014'
  if (totalSeconds < 60) return `${totalSeconds}s`
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  if (minutes < 60) return `${minutes}m ${seconds}s`
  const hours = Math.floor(minutes / 60)
  const mins = minutes % 60
  return `${hours}h ${mins}m`
}
```

---

### WR-04: `formatDuration` imported from `RunsPage` into `RunDetailPage` — cross-page coupling

**File:** `web/src/features/runs/RunDetailPage.tsx:9`

**Issue:** `formatDuration` is a pure utility function exported from `RunsPage.tsx` and imported directly into `RunDetailPage.tsx`. This couples two independent page-level components at the module level: a rename or relocation of `RunsPage.tsx` would break `RunDetailPage.tsx`. It also prevents tree-shaking from eliminating `RunsPage` code when only `RunDetailPage` is in the render tree.

**Fix:** Move `formatDuration` (and `statusBadgeClassName`, which is duplicated verbatim in both files — see IN-02) to a shared utility module such as `web/src/features/runs/runUtils.ts`, and import from there in both pages.

---

## Info

### IN-01: `useRuns` is dead code

**File:** `web/src/features/runs/useRuns.ts:20-26`

**Issue:** `useRuns` is exported but not imported anywhere in the codebase. All run-list fetching goes through `usePollingRuns`. The mock in `RunsPage.test.tsx` and `RunDetailPage.test.tsx` still includes `useRuns: vi.fn()` even though neither test uses it. The dead hook inflates the bundle and creates the query-key collision risk described in WR-02.

**Fix:** Delete the `useRuns` function from `useRuns.ts` and remove the corresponding `useRuns: vi.fn()` entries from both test mock factories.

---

### IN-02: `statusBadgeClassName` is duplicated verbatim in both page components

**File:** `web/src/features/runs/RunsPage.tsx:20-31` and `web/src/features/runs/RunDetailPage.tsx:28-39`

**Issue:** The two function bodies are identical. Any future addition of a new state string (e.g., `'ABORTED'`) to one file must be manually mirrored in the other.

**Fix:** Move to the shared utility module proposed in WR-04 (`runUtils.ts`) and import in both pages.

---

### IN-03: 1-second tick timer in `RunDetailPage` lacks an explanatory comment

**File:** `web/src/features/runs/RunDetailPage.tsx:88-93`

**Issue:** The `setTick` state and associated `useEffect` exist solely to re-render the component every second while a run is `RUNNING`, so the live duration display in the header stays current. The purpose is non-obvious — a future reader may mistake it for redundant polling or remove it assuming `usePollingRuns`'s 3-second refetch is sufficient.

**Fix:** Add a brief inline comment:

```tsx
// Re-render every second while RUNNING so the live duration counter updates smoothly.
// usePollingRuns only refetches server data every 3 s; this local tick drives the UI clock.
const [, setTick] = useState(0)
useEffect(() => {
  if (run?.state !== 'RUNNING') return
  const timer = setInterval(() => setTick((n) => n + 1), 1000)
  return () => clearInterval(timer)
}, [run?.state])
```

---

_Reviewed: 2026-04-07T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
