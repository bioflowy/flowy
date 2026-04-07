# Phase 30: File Browsing - Context

**Gathered:** 2026-04-07
**Status:** Ready for planning

<domain>
## Phase Boundary

Users can view workflow runs for a project, browse output files for completed runs, download files directly from S3 or local storage through the browser, and preview small text files inline. Unlike Phases 27-29 (frontend-only), this phase requires new backend endpoints: list runs by project, and file download/preview via storage backend proxy.

</domain>

<decisions>
## Implementation Decisions

### Runs list display
- Table layout on `/projects/{slug}/runs` — consistent with Phase 27 (Users), Phase 28 (Members), Phase 29 (Registry) table pattern
- Columns: Run ID (truncated UUID), Status (colored badge), Registry Ref (or WDL path if file-based run), Started (timestamp), Duration (calculated from CreatedAt/UpdatedAt)
- Status badges: colored for each state — green=Completed, yellow=Running, red=Failed, gray=Pending
- Clicking a run row navigates to `/projects/{slug}/runs/{id}` detail page — consistent with registry entry detail navigation pattern from Phase 29

### Run detail page
- Two-section layout: metadata header + outputs table (like registry entry detail page pattern from Phase 29)
- Metadata section: Run ID (full), status badge, registry ref or WDL path, start time, end time, duration
- Outputs table below metadata: flat table of WDL output entries (key→value pairs from OutputsJSON)

### Output file display
- Flat table of output entries — WDL outputs are key→value pairs, not hierarchical filesystem
- Columns: Output name (WDL key), Value/Path, Type indicator
- File-type outputs get a "File" badge and download button; string/int/other outputs show value inline as text

### Download behavior
- Backend proxies all downloads — frontend doesn't know which storage backend is used (S3 vs local)
- Single endpoint: `GET /projects/{slug}/runs/{id}/files?path=...` streams file from storage backend
- Content-Disposition header triggers browser download
- Transparent across S3 and local storage — secure (no direct S3 access needed)

### Inline text preview
- Click to expand preview below the output row — text content appears in a code block (similar to WDL source viewer from Phase 29)
- Size threshold: under 100KB = previewable; larger files are download-only
- Same download endpoint with `?preview=true` parameter returns content inline (Content-Type: text/plain) instead of as attachment

### Backend endpoints (NEW)
- `GET /projects/{slug}/runs` — list all runs for project, newest first. Optional `?status=completed` filter. No pagination for v2.0. Requires `viewer` role.
- `GET /projects/{slug}/runs/{id}/files?path=...` — stream file from storage backend. Add `?preview=true` for inline text preview (Content-Type: text/plain, 100KB max). Requires `viewer` role.
- `store.ListRunsByProject(ctx, projectID)` — new store method needed
- Run list response type needed in `internal/api/types.go`

### Claude's Discretion
- Loading/empty states for runs table and outputs table
- Exact status badge colors (green/yellow/red/gray or Tailwind equivalents)
- Duration formatting (human-readable like "2m 30s" or raw seconds)
- File type detection heuristic (how to distinguish File outputs from primitive values in OutputsJSON)
- Error state when run is not completed (show status but no outputs)
- Sidebar navigation link placement for Runs

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Backend - Existing run infrastructure
- `internal/server/handlers.go` — handleStatus (GET /workflows/{id}/status), handleOutputs (GET /workflows/{id}/outputs), handleSubmit (POST) — patterns for new list-runs and file-download handlers
- `internal/server/server.go` lines 148-155 — Existing `/runs` route group with projectRoleRequired middleware (submit=executor, status/outputs=viewer, abort=executor)
- `internal/api/types.go` — StatusResponse, OutputsResponse, SubmitRequest — patterns for new RunListResponse type
- `internal/store/store.go` lines 122-133 — Run struct (ID, ProjectID, WDLPath, RegistryRef, RegistryDigest, InputsJSON, State, OutputsJSON, CreatedAt, UpdatedAt)
- `internal/store/sqlite/runs.go` — CreateRun, GetRun, GetRunByPrefix, UpdateRunState — patterns for new ListRunsByProject method

### Backend - Storage download
- `internal/storage/backend.go` lines 19-39 — Backend interface with DownloadFile(path) method, RunOutputsDir helper
- `internal/storage/local.go` — LocalBackend.DownloadFile implementation
- `internal/storage/s3.go` — S3Backend.DownloadFile implementation

### Frontend patterns (Phase 26-29)
- `web/src/features/registry/RegistryPage.tsx` — Table + badge + dropdown menu pattern to replicate for runs list
- `web/src/features/registry/RegistryEntryPage.tsx` — Detail page with metadata + table pattern to replicate for run detail
- `web/src/features/registry/WDLSourceViewer.tsx` — Code viewer pattern to replicate for inline text file preview
- `web/src/features/registry/useRegistry.ts` — React Query hooks pattern to replicate for runs
- `web/src/lib/apiClient.ts` — API client with get/post/put/delete/patch + Bearer token

### UI components available
- `web/src/components/ui/` — table, badge, button, card, separator, tooltip

### Route structure
- `web/src/App.tsx` — Add routes for `/projects/:slug/runs` and `/projects/:slug/runs/:id` inside ProtectedRoute
- `web/src/components/layout/Sidebar.tsx` — Add Runs nav link

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- shadcn/ui Table, Badge, Button: reuse Phase 27-29 patterns for runs table and status badges
- React Query hooks pattern from `useRegistry.ts`: replicate for `useRuns.ts`
- WDLSourceViewer component: pattern for inline text file preview (code block with line numbers + copy button)
- apiClient with all HTTP methods: ready for new runs API calls

### Established Patterns
- Feature folder structure: `web/src/features/runs/` with hooks, pages
- Table rows with colored badges for categorical data (registry kind badges → run status badges)
- Navigation: list page with clickable rows → detail page (registry and project patterns)
- React Query invalidation on mutation success
- Project-scoped API calls using `useProjectStore` activeSlug

### Integration Points
- App.tsx: add `/projects/:slug/runs` and `/projects/:slug/runs/:id` routes inside ProtectedRoute
- Sidebar.tsx: add Runs nav link at `/projects/{slug}/runs`
- Backend server.go: add new route handlers inside existing `/runs` route group
- Backend store: add ListRunsByProject to Store interface and SQLiteStore

</code_context>

<specifics>
## Specific Ideas

- Backend proxies all file downloads — no presigned S3 URLs, keeps frontend simple and secure
- Preview uses same endpoint as download with `?preview=true` flag — single endpoint, two modes
- Run status badges mirror registry kind badge pattern but with state-based colors

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 30-file-browsing*
*Context gathered: 2026-04-07*
