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
  info: 2
  total: 6
status: issues_found
---

# Phase 35: Code Review Report

**Reviewed:** 2026-04-07T00:00:00Z
**Depth:** standard
**Files Reviewed:** 7
**Status:** issues_found

## Summary

This phase introduces run-status polling via `usePollingRuns` and wires it into `RunDetailPage` and `RunsPage`. The polling hook itself is well-structured: it uses TanStack Query's `refetchInterval` callback to stop polling once all runs reach a terminal state, and `refetchIntervalInBackground: false` prevents unnecessary background traffic.

Four warnings were found:

1. A variable shadowing bug in `RunDetailPage` where an inner `id` (setInterval handle) shadows the outer route-param `id` from `useParams`.
2. A cross-browser reliability issue with the detached anchor element used for file downloads.
3. A broken extension-extraction expression in `isPreviewable` that silently produces a wrong `ext` string for files with no dot in their name.
4. A polling dead-zone: once the runs list is empty, `refetchInterval` returns `false` permanently, so externally submitted runs will never appear without a page refresh.

Two info-level items were also found: dead code (`useRuns` hook) and a fragile non-null assertion pattern inside `renderOutputsSection`.

---

## Warnings

### WR-01: Variable shadowing — `id` (setInterval handle) shadows route param `id`

**File:** `web/src/features/runs/RunDetailPage.tsx:89`

**Issue:** `useParams` destructures `{ slug, id }` at line 72. Inside the `useEffect` callback at line 89, `const id = setInterval(...)` declares a new `id` in the inner block scope, silently shadowing the route-param `id`. TypeScript does not error because the inner `const` is in a nested closure scope. The `clearInterval(id)` cleanup correctly uses the inner variable, but any future developer who reads the `useEffect` and expects `id` to refer to the run UUID will be misled.

**Fix:**
```ts
useEffect(() => {
  if (run?.state !== 'RUNNING') return
  const timer = setInterval(() => setTick((n) => n + 1), 1000)
  return () => clearInterval(timer)
}, [run?.state])
```

---

### WR-02: Detached anchor element used for file download — unreliable in Firefox

**File:** `web/src/features/runs/RunDetailPage.tsx:119-121`

**Issue:** The download handler creates an `<a>` element, sets `href` and `download`, and calls `a.click()` without ever appending the element to the document. Chromium accepts clicks on detached elements, but Firefox requires the element to be in the DOM for programmatic `click()` to trigger a download. This will silently fail in Firefox.

**Fix:**
```ts
const a = document.createElement('a')
a.href = URL.createObjectURL(blob)
a.download = path.split('/').pop() ?? 'file'
document.body.appendChild(a)
a.click()
document.body.removeChild(a)
URL.revokeObjectURL(a.href)
```

---

### WR-03: `isPreviewable` produces wrong `ext` for paths with no file extension

**File:** `web/src/features/runs/RunDetailPage.tsx:51`

**Issue:** `path.lastIndexOf('.')` returns `-1` when the path contains no dot. `path.slice(-1)` then returns only the last character of the path string — not a file extension. The lookup against `PREVIEWABLE_EXTENSIONS` happens to return `false` in practice (because no single character is in the set), but this is accidental correctness. If `PREVIEWABLE_EXTENSIONS` ever gains a single-character entry, or if the logic is reused in a different context, the function will misfire for extensionless paths.

**Fix:**
```ts
function isPreviewable(path: string): boolean {
  const dotIndex = path.lastIndexOf('.')
  if (dotIndex === -1) return false
  const ext = path.slice(dotIndex)
  return PREVIEWABLE_EXTENSIONS.has(ext)
}
```

---

### WR-04: Polling stops permanently when runs list is empty — new runs never appear

**File:** `web/src/features/runs/usePollingRuns.ts:12-15`

**Issue:** `refetchInterval` returns `false` when `data` is `undefined` or an empty array. This is correct for the initial load (where `enabled: !!slug` drives the first fetch). However, once the initial fetch resolves to an empty array, polling is disabled and never re-enabled. If a user lands on the Runs page before submitting any jobs, and then submits a run via the CLI, the page will never auto-update — the new run will not appear until the user manually refreshes.

This is a product-level UX regression compared to a simple unconditional poll.

**Fix:** Return a fixed interval (e.g., `5000`) when data is empty or undefined, so the page continues checking for newly submitted runs. Only stop polling when all _existing_ runs are in terminal states and the list is non-empty:

```ts
refetchInterval: (query) => {
  const data = query.state.data
  // No data yet or empty list: keep polling so new runs appear
  if (!data || data.length === 0) return 5000
  // All runs terminal: stop
  return data.some((r) => !TERMINAL_STATES.has(r.state)) ? 3000 : false
},
```

Note: the existing unit test at `usePollingRuns.test.ts:72-79` asserts that `false` is returned for empty/undefined data. If the above fix is applied, those two test cases must be updated to expect `5000`.

---

## Info

### IN-01: `useRuns` is exported but never used — dead code

**File:** `web/src/features/runs/useRuns.ts:20-26`

**Issue:** `useRuns` is defined and exported but neither `RunsPage` nor `RunDetailPage` imports it. Both pages use `usePollingRuns` instead. The function is dead code that duplicates the same TanStack Query setup without the polling behavior, which could confuse future developers about which hook to use.

**Fix:** Remove the `useRuns` function, or add a JSDoc comment explicitly marking it as the non-polling variant (if it is intentionally kept for future use). The `RunsPage.test.tsx` and `RunDetailPage.test.tsx` mock files also list `useRuns: vi.fn()` in their `vi.mock` factory, which can be cleaned up once the function is removed.

---

### IN-02: Non-null assertion on `run` inside `renderOutputsSection` is fragile

**File:** `web/src/features/runs/RunDetailPage.tsx:144`

**Issue:** `renderOutputsSection` uses `run!.state` and `run!.state` (lines 144, 146, 147, 148). The function is only ever called after the `if (!run)` guard at line 130, so the assertion is safe today. But the function is defined before the guard and could be called or extracted earlier without TypeScript complaining, because the outer `run` variable's type is `RunListItem | undefined`.

**Fix:** Pass `run` as an explicit parameter to enforce type safety at the call site:

```ts
function renderOutputsSection(run: RunListItem) {
  if (run.state !== 'COMPLETED') { ... }
  // ...
}
// at call site:
{renderOutputsSection(run)}
```

Alternatively, keep the pattern but remove the `!` assertions and use optional chaining, since `run` is guaranteed non-null by the guard above.

---

_Reviewed: 2026-04-07T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
