# Phase 35: Run Status Polling - Context

**Gathered:** 2026-04-08
**Status:** Ready for planning

<domain>
## Phase Boundary

Browser users see live run status without manual page refresh. RunDetailPage and RunsPage auto-poll the server while runs are in non-terminal states (PENDING, RUNNING) and stop automatically when all visible runs reach terminal states (COMPLETED, FAILED, ABORTED).

</domain>

<decisions>
## Implementation Decisions

### Polling Visual Feedback
- Show a subtle pulsing dot next to the status badge for PENDING/RUNNING states to indicate live polling is active
- Update the duration timer display every 1 second via a separate interval for RUNNING runs — gives continuous feedback between 3-second poll cycles
- No "last updated" timestamp — polling every 3s is fast enough, extra text adds clutter

### Polling Error Handling
- Silent retry on next interval if a poll request fails — React Query handles retries, no toast spam for transient network issues
- Use 3-second polling interval for both RunsPage and RunDetailPage — consistent behavior per success criteria
- Pause polling when browser tab is not visible — use React Query's built-in visibility detection to save resources; resume immediately when tab regains focus

### Claude's Discretion
- Implementation mechanism: React Query's `refetchInterval` option with a callback function that returns `false` when polling should stop
- Hook structure: extend existing `useRuns` hook or create a separate `useRunPolling` hook — whichever is cleaner

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `web/src/features/runs/useRuns.ts` — existing `useRuns` and `useRunOutputs` hooks using React Query
- `web/src/features/runs/RunDetailPage.tsx` — run detail view with status badge and outputs section
- `web/src/features/runs/RunsPage.tsx` — run list with status badges and `formatDuration` helper
- `web/src/features/runs/RunsPage.tsx:formatDuration` — already handles RUNNING state with `Date.now()`

### Established Patterns
- React Query `useQuery` with `queryKey` arrays and `enabled` guards
- Status badge styling via `statusBadgeClassName()` function (same in both pages)
- Badge component from shadcn/ui with variant="outline" and custom className

### Integration Points
- `useRuns(slug)` returns all runs for a project — add `refetchInterval` conditionally
- `RunDetailPage` finds single run from `useRuns` result — needs individual run polling or list-level polling
- `useRunOutputs` already guards on `state === 'COMPLETED'` — will auto-fetch outputs when polling detects completion

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches using React Query's built-in polling capabilities.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>
