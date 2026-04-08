# Phase 35: Run Status Polling - Research

**Researched:** 2026-04-07
**Domain:** React Query polling / React frontend
**Confidence:** HIGH

## Summary

Phase 35 adds automatic status polling to RunsPage and RunDetailPage. Runs in PENDING or RUNNING state trigger polling every 3 seconds; polling stops automatically when all visible runs reach a terminal state (COMPLETED, FAILED, ABORTED). A pulsing dot visual indicator communicates active polling, and a separate 1-second interval updates the duration timer on RUNNING runs between poll cycles.

The implementation is entirely in the `web/` frontend. The existing `useRuns` hook uses TanStack Query v5, which natively supports `refetchInterval` as a callback function — this is the primary mechanism. No new npm packages are required. The only structural decision is whether to add polling directly into the `useRuns` hook or create a thin `usePollingRuns` wrapper; both are clean options, with the wrapper being slightly preferable for test isolation.

**Primary recommendation:** Add `refetchInterval` as a callback to `useRuns` (or a thin wrapper), returning `3000` when any run is PENDING/RUNNING and `false` otherwise. Add a `useEffect` + `setInterval` for the 1-second duration tick in RunDetailPage.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- Show a subtle pulsing dot next to the status badge for PENDING/RUNNING states to indicate live polling is active
- Update the duration timer display every 1 second via a separate interval for RUNNING runs
- No "last updated" timestamp
- Silent retry on next interval if a poll request fails (React Query handles retries, no toast spam)
- Use 3-second polling interval for both RunsPage and RunDetailPage
- Pause polling when browser tab is not visible — use React Query's built-in visibility detection; resume immediately when tab regains focus

### Claude's Discretion
- Implementation mechanism: React Query's `refetchInterval` option with a callback function that returns `false` when polling should stop
- Hook structure: extend existing `useRuns` hook or create a separate `useRunPolling` hook — whichever is cleaner

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| EXEC-04 | User can monitor running workflow status via poll-based updates (auto-stop on terminal state) | React Query `refetchInterval` callback pattern; pulsing dot indicator; 1s duration tick; tab visibility pause |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| @tanstack/react-query | 5.95.2 (installed) | Query + polling | Already in use; `refetchInterval` callback built in |
| react | 19.0.0 (installed) | Component state + effects | Project stack |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| vitest | 3.2.4 (installed) | Unit tests | Existing test framework for all web tests |
| @testing-library/react | 16.3.2 (installed) | Component rendering in tests | Existing pattern for all feature tests |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `refetchInterval` callback | `useEffect` + `setInterval` + manual `queryClient.invalidateQueries` | More code, misses built-in visibility detection, more test surface area |
| `refetchInterval` callback | WebSocket / SSE | Server-side changes required, deferred to v2.2+ per REQUIREMENTS.md |

**Installation:**
No new packages needed. All dependencies already installed.

**Version verification:**
- `@tanstack/react-query` installed: 5.95.2 [VERIFIED: package.json in web/node_modules]
- `@tanstack/query-core` installed: 5.95.2 [VERIFIED: pnpm store path]
- Latest registry version: 5.96.2 [VERIFIED: npm view @tanstack/react-query version]

## Architecture Patterns

### Recommended Project Structure
```
web/src/features/runs/
├── useRuns.ts           # extend with refetchInterval callback (or keep pure)
├── usePollingRuns.ts    # (optional) thin wrapper that adds polling options
├── RunsPage.tsx         # add pulsing dot to status badge
├── RunDetailPage.tsx    # add pulsing dot + 1s duration tick
└── __tests__/
    ├── RunsPage.test.tsx       # extend with polling stop/start assertions
    └── RunDetailPage.test.tsx  # extend with polling and tick assertions
```

### Pattern 1: refetchInterval Callback in useRuns (inline extension)
**What:** Add `refetchInterval` and `refetchIntervalInBackground: false` directly to the existing `useQuery` call in `useRuns`.
**When to use:** Simplest change; acceptable if polling is always desired when slug is present.
**Example:**
```typescript
// Source: query-core types — _tsup-dts-rollup.d.ts (installed @tanstack/query-core@5.95.2)
// refetchInterval?: number | false | ((query: Query<...>) => number | false | undefined)

const TERMINAL_STATES = new Set(['COMPLETED', 'FAILED', 'ABORTED'])

export function useRuns(slug: string) {
  return useQuery<RunListItem[]>({
    queryKey: ['projects', slug, 'runs'],
    queryFn: () => apiClient.get(`/projects/${slug}/runs`),
    enabled: !!slug,
    refetchInterval: (query) => {
      const runs = query.state.data
      if (!runs) return false
      const hasActive = runs.some((r) => !TERMINAL_STATES.has(r.state))
      return hasActive ? 3000 : false
    },
    refetchIntervalInBackground: false, // pauses when tab is not visible (built-in)
  })
}
```

### Pattern 2: Thin Wrapper usePollingRuns (better for test isolation)
**What:** Keep `useRuns` unchanged (returns bare query), add `usePollingRuns` that spreads polling options on top.
**When to use:** Preferred when tests for non-polling pages should not see `refetchInterval` in mock assertions. Cleaner separation of concerns.
**Example:**
```typescript
// Source: same query-core type signature as above
export function usePollingRuns(slug: string) {
  return useQuery<RunListItem[]>({
    queryKey: ['projects', slug, 'runs'],
    queryFn: () => apiClient.get(`/projects/${slug}/runs`),
    enabled: !!slug,
    refetchInterval: (query) => {
      const data = query.state.data
      if (!data) return false
      return data.some((r) => !TERMINAL_STATES.has(r.state)) ? 3000 : false
    },
    refetchIntervalInBackground: false,
  })
}
```
RunsPage and RunDetailPage import `usePollingRuns`; `useRuns` stays unchanged for non-polling contexts.

### Pattern 3: 1-second Duration Tick in RunDetailPage
**What:** A `useEffect` + `setInterval` forces a re-render every second to keep the duration timer live.
**When to use:** Only when the displayed run has `state === 'RUNNING'` — avoids unnecessary re-renders for terminal runs.
**Example:**
```typescript
// Source: React docs — useEffect / setInterval pattern
import { useState, useEffect } from 'react'

// Inside RunDetailPage component, after resolving `run`:
const [, forceUpdate] = useState(0)
useEffect(() => {
  if (run?.state !== 'RUNNING') return
  const id = setInterval(() => forceUpdate((n) => n + 1), 1000)
  return () => clearInterval(id)
}, [run?.state])
```
The existing `formatDuration` function already uses `Date.now()` for RUNNING state, so a re-render is all that is needed.

### Pattern 4: Pulsing Dot Visual Indicator
**What:** Render a small pulsing dot adjacent to the status badge when state is PENDING or RUNNING.
**When to use:** Only for active (non-terminal) states.
**Example:**
```tsx
{/* Source: Tailwind CSS animate-pulse utility */}
{!TERMINAL_STATES.has(run.state) && (
  <span className="inline-block h-2 w-2 rounded-full bg-yellow-400 animate-pulse" />
)}
<Badge variant="outline" className={statusBadgeClassName(run.state)}>
  {run.state}
</Badge>
```

### Anti-Patterns to Avoid
- **Polling inside RunDetailPage at the single-run level:** RunDetailPage already derives its run from the `useRuns` (or `usePollingRuns`) list query. Adding a separate single-run polling query would create two concurrent polling loops. Use list-level polling only.
- **Using `refetchOnWindowFocus: false`:** This disables focus-based refetches globally for the query. React Query's `refetchIntervalInBackground: false` already handles tab visibility for the interval — do not disable focus refetch, which is separate and desirable behavior.
- **Toast on poll failure:** The locked decision prohibits toast spam for transient network errors during polling. React Query's default retry behavior handles transient failures silently.
- **Polling when no slug:** The `enabled: !!slug` guard already prevents queries when slug is absent. The `refetchInterval` callback receives `query.state.data === undefined` in that case and returns `false` — consistent behavior.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Tab visibility detection | Custom `visibilitychange` event listener | `refetchIntervalInBackground: false` | Built into React Query; correctly handles race conditions and SSR |
| Polling stop condition | Manual `clearInterval` in component | `refetchInterval` returning `false` | React Query manages the interval lifecycle declaratively |
| Retry on network error | Custom catch + retry logic | React Query default retry behavior | Handles exponential backoff, jitter, and max retries already |

**Key insight:** React Query's `refetchInterval` callback receives the full `Query` object including `query.state.data`, so the stop condition can be evaluated from live data without additional state.

## Common Pitfalls

### Pitfall 1: refetchInterval callback receives Query object, not data directly
**What goes wrong:** Developers write `refetchInterval: (data) => ...` expecting the fetched data, but in TanStack Query v5 the callback receives the `Query` object.
**Why it happens:** The v4-to-v5 migration changed the callback signature. In v4 it received `data` directly.
**How to avoid:** Access data as `query.state.data`. See verified type: `(query: Query<TQueryFnData, ...>) => number | false | undefined`.
**Warning signs:** TypeScript error "Property 'state' does not exist" — confirms you passed wrong param name.

### Pitfall 2: Duration display freezes between 3-second polls
**What goes wrong:** Without the 1-second `setInterval`, the displayed duration only updates when a poll fires (every 3 seconds), causing visible jumps.
**Why it happens:** React does not re-render on elapsed time; `formatDuration` uses `Date.now()` but is only called during renders.
**How to avoid:** Add the separate 1-second tick interval as described in Pattern 3, gated on `state === 'RUNNING'`.

### Pitfall 3: Memory leak if interval is not cleaned up
**What goes wrong:** The 1-second `setInterval` continues running after the component unmounts.
**Why it happens:** `setInterval` without a cleanup function in `useEffect`.
**How to avoid:** Always return `() => clearInterval(id)` from the `useEffect`. Verified by standard React cleanup pattern.
**Warning signs:** React warning "Can't perform a state update on an unmounted component" in test output.

### Pitfall 4: Polling fires for already-completed runs on RunDetailPage
**What goes wrong:** User opens RunDetailPage for a COMPLETED run; polling fires anyway.
**Why it happens:** The `refetchInterval` callback fires once on mount before any data arrives (when `query.state.data` is `undefined`). Returning `3000` when data is `undefined` would cause one spurious poll.
**How to avoid:** Return `false` when `data` is `undefined` or empty. The recommended implementation above does this: `if (!runs) return false`.

### Pitfall 5: Test using vi.useFakeTimers conflicts with React Query internals
**What goes wrong:** `vi.useFakeTimers()` causes React Query's internal scheduler to behave unexpectedly, breaking unrelated tests.
**Why it happens:** React Query uses `setTimeout`/`setInterval` internally; fake timers intercept all timer calls.
**How to avoid:** For polling tests, prefer asserting the `refetchInterval` option passed to the mock rather than advancing fake timers. If advancing timers is needed, wrap in `act()` and call `vi.runAllTimers()` carefully. Alternatively mock `usePollingRuns` entirely and assert the rendered output for terminal vs active states.

## Code Examples

### Complete usePollingRuns hook
```typescript
// Source: verified against installed @tanstack/query-core@5.95.2 type signature
import { useQuery } from '@tanstack/react-query'
import { apiClient } from '@/lib/apiClient'
import type { RunListItem } from '@/features/runs/useRuns'

const TERMINAL_STATES = new Set(['COMPLETED', 'FAILED', 'ABORTED'])

export function usePollingRuns(slug: string) {
  return useQuery<RunListItem[]>({
    queryKey: ['projects', slug, 'runs'],
    queryFn: () => apiClient.get(`/projects/${slug}/runs`),
    enabled: !!slug,
    refetchInterval: (query) => {
      const data = query.state.data
      if (!data || data.length === 0) return false
      return data.some((r) => !TERMINAL_STATES.has(r.state)) ? 3000 : false
    },
    refetchIntervalInBackground: false,
  })
}
```

### Duration tick in RunDetailPage
```typescript
// Source: React useEffect documentation pattern
const [tick, setTick] = useState(0) // tick value unused; just forces re-render
useEffect(() => {
  if (run?.state !== 'RUNNING') return
  const id = setInterval(() => setTick((n) => n + 1), 1000)
  return () => clearInterval(id)
}, [run?.state])
```

### Pulsing dot with status badge (RunsPage row)
```tsx
// Source: Tailwind CSS animate-pulse (existing Tailwind v4 setup)
<TableCell>
  <div className="flex items-center gap-1.5">
    {!TERMINAL_STATES.has(run.state) && (
      <span
        className="h-2 w-2 rounded-full bg-yellow-400 animate-pulse"
        aria-label="polling active"
      />
    )}
    <Badge variant="outline" className={statusBadgeClassName(run.state)}>
      {run.state}
    </Badge>
  </div>
</TableCell>
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `refetchInterval(data)` callback receives data directly | `refetchInterval(query)` callback receives Query object | TanStack Query v5 (2023) | Must access `query.state.data` not first param directly |
| `useIsFetching()` for visual indicator | Custom pulsing dot controlled by state check | N/A — project decision | Simpler, no global fetching state dependency |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `formatDuration` re-evaluating `Date.now()` on every render is sufficient for 1-second accuracy | Architecture Patterns Pattern 3 | Timer display could drift; would need explicit timestamp state instead of forceUpdate |
| A2 | `refetchIntervalInBackground: false` causes React Query to pause the interval when the page visibility API reports hidden | Don't Hand-Roll | Tab switching would not pause polling; resource waste |

Note: A2 is tagged ASSUMED from training knowledge. The installed source code confirms `refetchIntervalInBackground` is a real option (verified from type definitions), but the exact behavior of "pauses when tab hidden" was not traced through query-core source code in this session.

## Open Questions

1. **Shared TERMINAL_STATES constant location**
   - What we know: Both `RunsPage.tsx`, `RunDetailPage.tsx`, and `usePollingRuns.ts` will need the same set `['COMPLETED', 'FAILED', 'ABORTED']`
   - What's unclear: Should it live in `useRuns.ts`, a separate `runStates.ts` constants file, or be inlined per-file?
   - Recommendation: Export `TERMINAL_STATES` from `useRuns.ts` alongside the types — already the shared import point for both pages.

2. **Hook name: useRuns vs usePollingRuns**
   - What we know: CONTEXT.md gives Claude discretion; both inline extension and wrapper are acceptable
   - What's unclear: Inline extension (`useRuns`) is simpler but couples polling to all callers; wrapper (`usePollingRuns`) is cleaner but adds a file
   - Recommendation: Use `usePollingRuns` as a thin wrapper. Keeps existing `useRuns` tests unchanged and makes the polling intent explicit at the call site.

## Environment Availability

Step 2.6: SKIPPED — this phase is purely frontend code changes. No external services, CLIs, or runtimes beyond the existing Node.js/npm toolchain are required.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Vitest 3.2.4 |
| Config file | `web/vitest.config.ts` |
| Quick run command | `cd web && npx vitest run src/features/runs/` |
| Full suite command | `cd web && npx vitest run` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| EXEC-04 | usePollingRuns returns refetchInterval=3000 when any run is PENDING/RUNNING | unit | `cd web && npx vitest run src/features/runs/__tests__/usePollingRuns.test.ts` | ❌ Wave 0 |
| EXEC-04 | usePollingRuns returns refetchInterval=false when all runs are terminal | unit | `cd web && npx vitest run src/features/runs/__tests__/usePollingRuns.test.ts` | ❌ Wave 0 |
| EXEC-04 | usePollingRuns returns refetchInterval=false when data is undefined | unit | `cd web && npx vitest run src/features/runs/__tests__/usePollingRuns.test.ts` | ❌ Wave 0 |
| EXEC-04 | RunsPage pulsing dot renders for RUNNING/PENDING rows, absent for terminal | unit | `cd web && npx vitest run src/features/runs/__tests__/RunsPage.test.tsx` | ✅ (extend existing) |
| EXEC-04 | RunDetailPage pulsing dot renders for RUNNING run | unit | `cd web && npx vitest run src/features/runs/__tests__/RunDetailPage.test.tsx` | ✅ (extend existing) |
| EXEC-04 | RunDetailPage pulsing dot absent for COMPLETED run | unit | `cd web && npx vitest run src/features/runs/__tests__/RunDetailPage.test.tsx` | ✅ (extend existing) |
| EXEC-04 | TypeScript build clean after changes | static | `cd web && npx tsc --noEmit` | ✅ (always run) |

### Sampling Rate
- **Per task commit:** `cd web && npx vitest run src/features/runs/ && npx tsc --noEmit`
- **Per wave merge:** `cd web && npx vitest run`
- **Phase gate:** Full suite green + `make build-ui` before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `web/src/features/runs/__tests__/usePollingRuns.test.ts` — covers EXEC-04 hook logic (refetchInterval callback behavior)

*(Existing test files `RunsPage.test.tsx` and `RunDetailPage.test.tsx` are present and will be extended in-place.)*

## Project Constraints (from CLAUDE.md)

| Directive | Applies to This Phase |
|-----------|----------------------|
| All docs/comments/commit messages in English | Yes — all new files and code comments |
| After modifying `web/`, run `cd web && npx tsc --noEmit` in every task's verify block | Yes — every task that modifies `.tsx`/`.ts` files |
| Final verification uses `make build-ui` | Yes — phase gate |
| Technical debt doc required when applying workaround fix | If applicable during execution |
| ADR required when architectural discussion reaches conclusion | If applicable during execution |

## Security Domain

This phase adds no authentication endpoints, session management, or new API routes. It extends existing authenticated query hooks only. No ASVS categories apply beyond what is already established in the project.

## Sources

### Primary (HIGH confidence)
- `@tanstack/query-core@5.95.2` installed source — `_tsup-dts-rollup.d.ts` confirming `refetchInterval` callback signature and `refetchIntervalInBackground` option [VERIFIED: file path /home/uehara/flowy/web/node_modules/.pnpm/@tanstack+query-core@5.95.2/...]
- `web/src/features/runs/useRuns.ts` — existing hook structure [VERIFIED: read directly]
- `web/src/features/runs/RunDetailPage.tsx` — existing component structure [VERIFIED: read directly]
- `web/src/features/runs/RunsPage.tsx` — existing component + `formatDuration` using `Date.now()` [VERIFIED: read directly]
- `web/package.json` — installed dependency versions [VERIFIED: read directly]
- `web/vitest.config.ts` — test framework configuration [VERIFIED: read directly]
- Vitest run: 126 tests pass [VERIFIED: `npx vitest run` output]

### Secondary (MEDIUM confidence)
- npm registry: `@tanstack/react-query` latest = 5.96.2 [VERIFIED: npm view]

### Tertiary (LOW confidence)
- `refetchIntervalInBackground: false` pauses polling when tab is not visible: [ASSUMED] — option existence confirmed by types, behavior assumed from training knowledge

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all packages verified from installed node_modules and package.json
- Architecture: HIGH — refetchInterval callback type signature verified from installed source
- Pitfalls: HIGH for P1 (type sig verified) / MEDIUM for P2-P5 (standard React + React Query patterns from training, consistent with installed code)

**Research date:** 2026-04-07
**Valid until:** 2026-05-07 (stable library, no fast-moving API surface)
