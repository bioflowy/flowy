---
phase: 30-file-browsing
plan: 02
subsystem: web-frontend
tags: [react, react-query, file-browsing, runs, outputs, download, preview]
dependency_graph:
  requires: [30-01]
  provides: [runs-list-page, run-detail-page, file-download, file-preview]
  affects: [web/src/App.tsx]
tech_stack:
  added: []
  patterns:
    - React Query hooks (useRuns, useRunOutputs) replicating useRegistry pattern
    - Raw fetch with Authorization header for binary downloads (not apiClient)
    - fetch+blob pattern for file download with URL.createObjectURL
    - Raw fetch with ?preview=true for text file preview
    - Per-key preview toggle state with lazy fetch-on-first-expand
key_files:
  created:
    - web/src/features/runs/useRuns.ts
    - web/src/features/runs/RunsPage.tsx
    - web/src/features/runs/RunDetailPage.tsx
    - web/src/features/runs/__tests__/RunsPage.test.tsx
    - web/src/features/runs/__tests__/RunDetailPage.test.tsx
  modified:
    - web/src/App.tsx
decisions:
  - "useAuthStore.getState().token used for raw fetch calls (not apiClient) to support binary downloads and text previews"
  - "Run detail finds run from list via useRuns (no dedicated single-run GET endpoint in backend)"
  - "Preview expansion is lazy: content fetched only on first expand, then cached in useState"
  - "formatDuration exported from RunsPage.tsx and imported into RunDetailPage.tsx to avoid duplication"
metrics:
  duration: "4m 3s"
  completed_date: "2026-04-07"
  tasks_completed: 2
  files_changed: 6
---

# Phase 30 Plan 02: File Browsing Frontend Summary

**One-liner:** Run list and detail pages with status badges, file download via fetch+blob, and inline text preview with per-key toggle state.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Create useRuns hooks, RunsPage, and RunsPage tests | 36b79a3 | useRuns.ts, RunsPage.tsx, RunsPage.test.tsx |
| 2 | Create RunDetailPage with outputs table, download, preview, and route wiring | 69c1a59 | RunDetailPage.tsx, RunDetailPage.test.tsx, App.tsx |

## What Was Built

### useRuns.ts
React Query hooks for the runs feature:
- `useRuns(slug)` — fetches `GET /projects/{slug}/runs`, returns `RunListItem[]`
- `useRunOutputs(slug, id)` — fetches `GET /projects/{slug}/runs/{id}/outputs`, returns `OutputsResponse`
- `RunListItem` and `OutputsResponse` interfaces exported for TypeScript consumers

### RunsPage.tsx
Run list page at `/projects/:slug/runs`:
- 5-column table: Run ID (truncated UUID with Tooltip), Status (colored Badge), Source (registry_ref or wdl_path, truncated to 40 chars), Started (formatted date), Duration
- Status badge colors: green (COMPLETED), yellow (RUNNING), red (FAILED), gray (PENDING/default)
- Loading: 5 skeleton rows with `data-testid="skeleton-row"`
- Empty: "No runs yet." with CLI hint
- Error: "Failed to load runs. Please refresh the page."
- Row click navigates to `/projects/{slug}/runs/{id}`
- `formatDuration` helper exported for reuse in RunDetailPage

### RunDetailPage.tsx
Run detail page at `/projects/:slug/runs/:id`:
- Metadata header: run ID heading (8 chars with Tooltip), full ID, status badge, formatted start time, duration, source label
- Outputs table for COMPLETED runs: file detection via `isFilePath()`, type badge (File / primitive type), value/action column
- File row: filename, Download button (fetch+blob with loading spinner and toast.error fallback)
- Preview button: shown for `.txt .log .wdl .json .yaml .yml .tsv .csv`, lazy-fetched, expandable `<pre>` block
- Preview limited to 100KB with truncation notice
- Non-COMPLETED runs: state-specific message (RUNNING / FAILED / PENDING)
- Route wired in App.tsx

## Test Coverage

- `RunsPage.test.tsx`: 5 tests (loading, run rows, empty state, navigation, error state)
- `RunDetailPage.test.tsx`: 7 tests (metadata, outputs table, RUNNING message, FAILED message, download button, preview button, no preview for .bam)
- Full regression: 111 tests pass across 19 test files
- TypeScript: `npx tsc --noEmit` exits clean

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check

Files created:
- web/src/features/runs/useRuns.ts: FOUND
- web/src/features/runs/RunsPage.tsx: FOUND
- web/src/features/runs/RunDetailPage.tsx: FOUND
- web/src/features/runs/__tests__/RunsPage.test.tsx: FOUND
- web/src/features/runs/__tests__/RunDetailPage.test.tsx: FOUND

Commits:
- 36b79a3: feat(30-02): create useRuns hooks and RunsPage — FOUND
- 69c1a59: feat(30-02): create RunDetailPage with outputs table, download, preview, and route wiring — FOUND
