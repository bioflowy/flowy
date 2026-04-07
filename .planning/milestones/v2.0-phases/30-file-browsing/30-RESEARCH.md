# Phase 30: File Browsing - Research

**Researched:** 2026-04-07
**Domain:** Go HTTP handlers + SQLite store + React Query frontend + file streaming
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- Table layout on `/projects/{slug}/runs` — consistent with Phase 27/28/29 table pattern
- Columns: Run ID (truncated UUID), Status (colored badge), Registry Ref (or WDL path if file-based run), Started (timestamp), Duration (calculated from CreatedAt/UpdatedAt)
- Status badges: green=Completed, yellow=Running, red=Failed, gray=Pending
- Clicking a run row navigates to `/projects/{slug}/runs/{id}` detail page
- Run detail page: two-section layout (metadata header + outputs table)
- Metadata section: Run ID (full), status badge, registry ref or WDL path, start time, end time, duration
- Outputs table: flat table of WDL output entries (key/value pairs from OutputsJSON)
- Columns: Output name (WDL key), Value/Path, Type indicator
- File-type outputs get "File" badge and download button; string/int/other outputs show value inline
- Backend proxies all downloads — no direct S3 access; single endpoint `GET /projects/{slug}/runs/{id}/files?path=...`
- Content-Disposition header triggers browser download
- Click-to-expand inline preview below output row; text in code block (like WDLSourceViewer)
- Size threshold: under 100KB = previewable; `?preview=true` returns Content-Type: text/plain inline
- `GET /projects/{slug}/runs` — list all runs, newest first; optional `?status=completed`; `viewer` role required
- `GET /projects/{slug}/runs/{id}/files?path=...` — stream file; `?preview=true` for inline; `viewer` role required
- `store.ListRunsByProject(ctx, projectID)` — new store method needed
- New `RunListResponse` type needed in `internal/api/types.go`

### Claude's Discretion

- Loading/empty states for runs table and outputs table
- Exact status badge colors (Tailwind equivalents)
- Duration formatting (human-readable like "2m 30s" or raw seconds)
- File type detection heuristic (how to distinguish File outputs from primitives in OutputsJSON)
- Error state when run is not completed (show status but no outputs)
- Sidebar navigation link placement for Runs

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| FILE-01 | User can view list of workflow runs in a project | New `GET /projects/{slug}/runs` endpoint + `ListRunsByProject` store method + `RunsPage.tsx` frontend |
| FILE-02 | User can view output files for a completed run | New `RunDetailPage.tsx` using existing `GET /{id}/outputs` endpoint + OutputsJSON parsing |
| FILE-03 | User can download output files (S3/local) | New `GET /projects/{slug}/runs/{id}/files?path=...` endpoint using `backend.DownloadFileStream` |
| FILE-04 | User can preview small text files inline | Same file endpoint with `?preview=true` + 100KB limit; frontend expand-in-row pattern |
</phase_requirements>

---

## Summary

Phase 30 adds file browsing capabilities on top of a well-established codebase pattern. Phases 27-29 established a repeatable pattern: React Query hooks in a feature folder, shadcn/ui Table + Badge, list page with clickable rows navigating to a detail page. This phase follows the same pattern for runs.

The most significant new work is backend: two new HTTP handlers (`handleListRuns`, `handleFileDownload`) and one new store method (`ListRunsByProject`). The store layer already has all the fields needed on the `Run` struct (ProjectID, State, CreatedAt, UpdatedAt, RegistryRef, WDLPath, OutputsJSON). The storage backend already has `DownloadFileStream` on both `LocalBackend` and `S3Backend`. No new infrastructure is required.

The file download handler must proxy the stream from storage to the HTTP response writer directly — avoid buffering the entire file into memory. The preview mode shares the same endpoint with a `?preview=true` query parameter, applying a 100KB read limit before responding.

**Primary recommendation:** Implement as two plans: Plan 01 adds backend endpoints + store method; Plan 02 adds frontend pages. Frontend depends on the backend being available (or mocked in tests).

---

## Standard Stack

### Core (all already in project)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| go-chi/chi | v5 | HTTP routing + middleware | Already used for all routes |
| tanstack/react-query | ^5 | Server state + caching | Already used in all feature hooks |
| react-router | ^7 | Client routing | Already used in App.tsx |
| shadcn/ui | components present | Table, Badge, Button | Already used in Phases 27-29 |
| vitest + @testing-library/react | current | Frontend unit tests | Established test pattern |
| net/http/httptest | stdlib | Go HTTP handler tests | Used in all server_test files |

### No New Dependencies Required

All required capabilities exist in the current stack. The file download handler uses only stdlib (`io.Copy`, `http.ResponseWriter`).

---

## Architecture Patterns

### Backend: New Endpoint Registration Pattern

All new project-scoped routes go inside the `/runs` route group in `server.go` (lines 149-155). Follow exact pattern of existing routes:

```go
// In newRouter(), inside r.Route("/runs", ...) block:
r.With(s.projectRoleRequired("viewer")).Get("/", s.handleListRuns)
r.With(s.projectRoleRequired("viewer")).Get("/{id}/files", s.handleFileDownload)
```

Source: `internal/server/server.go` lines 149-155

### Backend: Handler Pattern

Follow `handleOutputs` as the model for `handleListRuns`. Follow `handleOutputs` + streaming for `handleFileDownload`:

```go
// handleListRuns: GET /projects/{slug}/runs
func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
    projectID := projectIDFromContext(r.Context())
    statusFilter := r.URL.Query().Get("status")
    runs, err := s.store.ListRunsByProject(r.Context(), projectID)
    // filter by status if provided, map to RunListItem, writeJSON
}

// handleFileDownload: GET /projects/{slug}/runs/{id}/files?path=...
func (s *Server) handleFileDownload(w http.ResponseWriter, r *http.Request) {
    runID := chi.URLParam(r, "id")
    path := r.URL.Query().Get("path")
    preview := r.URL.Query().Get("preview") == "true"
    // validate run exists and belongs to project
    // if preview: read up to 100KB, set Content-Type: text/plain
    // else: set Content-Disposition: attachment; filename="...", stream via DownloadFileStream
}
```

### Backend: File Streaming (no memory buffering)

For download mode, stream directly to `http.ResponseWriter` using `DownloadFileStream`:

```go
w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
w.Header().Set("Content-Type", "application/octet-stream")
if err := s.backend.DownloadFileStream(path, w); err != nil {
    // Cannot write error JSON — headers already sent if streaming started
    // Log the error; response will be incomplete
    log.Printf("handleFileDownload: stream %s: %v", path, err)
}
```

Source: `internal/storage/local.go` DownloadFileStream, `internal/storage/s3.go` DownloadFileStream

### Backend: Preview Mode (100KB limit)

For preview mode, read at most 100KB using `DownloadFile` (buffered):

```go
data, err := s.backend.DownloadFile(path)
if err != nil { ... }
const maxPreview = 100 * 1024
if len(data) > maxPreview {
    data = data[:maxPreview]
}
w.Header().Set("Content-Type", "text/plain; charset=utf-8")
w.WriteHeader(http.StatusOK)
w.Write(data)
```

### Backend: New Store Method

Add `ListRunsByProject` to Store interface in `internal/store/store.go` and implement in `internal/store/sqlite/runs.go`. Follow `GetRunByPrefix` pattern (uses `scanRunRows`):

```go
// Store interface addition:
ListRunsByProject(ctx context.Context, projectID string) ([]Run, error)

// SQLite implementation:
func (s *SQLiteStore) ListRunsByProject(ctx context.Context, projectID string) ([]store.Run, error) {
    rows, err := s.reader.QueryContext(ctx,
        `SELECT id, wdl_path, inputs_json, state, outputs_json, created_at, updated_at, project_id, registry_ref, registry_digest
           FROM runs WHERE project_id = ? ORDER BY created_at DESC`, projectID)
    // scan using scanRunRows pattern
}
```

### Backend: New API Types

Add to `internal/api/types.go`:

```go
// RunListItem is a single row in the runs list response.
type RunListItem struct {
    ID          string `json:"id"`
    State       string `json:"state"`
    RegistryRef string `json:"registry_ref,omitempty"`
    WDLPath     string `json:"wdl_path,omitempty"`
    CreatedAt   string `json:"created_at"`
    UpdatedAt   string `json:"updated_at"`
}
```

### Frontend: Feature Folder Structure

```
web/src/features/runs/
├── useRuns.ts          # React Query hooks (list runs, get run outputs)
├── RunsPage.tsx        # List page — /projects/:slug/runs
└── RunDetailPage.tsx   # Detail page — /projects/:slug/runs/:id
```

Follow `web/src/features/registry/` as the direct template.

### Frontend: React Query Hook Pattern

Follow `useRegistry.ts` exactly:

```typescript
// useRuns.ts
export function useRuns(slug: string) {
  return useQuery<RunListItem[]>({
    queryKey: ['projects', slug, 'runs'],
    queryFn: () => apiClient.get(`/projects/${slug}/runs`),
    enabled: !!slug,
  })
}

export function useRunOutputs(slug: string, id: string) {
  return useQuery<OutputsResponse>({
    queryKey: ['projects', slug, 'runs', id, 'outputs'],
    queryFn: () => apiClient.get(`/projects/${slug}/runs/${id}/outputs`),
    enabled: !!slug && !!id,
  })
}
```

### Frontend: File Download (browser-initiated via URL)

The download button should construct the download URL and use `window.location.href` or a hidden anchor tag — NOT `apiClient.get()` because the response is a binary stream, not JSON:

```typescript
function downloadFile(slug: string, runId: string, path: string, token: string) {
  // Construct URL with auth token as query param OR use fetch + blob
  // Preferred: fetch with auth header, then create blob URL
  const url = `/api/projects/${slug}/runs/${runId}/files?path=${encodeURIComponent(path)}`
  fetch(url, { headers: { Authorization: `Bearer ${token}` } })
    .then(res => res.blob())
    .then(blob => {
      const a = document.createElement('a')
      a.href = URL.createObjectURL(blob)
      a.download = path.split('/').pop() ?? 'file'
      a.click()
      URL.revokeObjectURL(a.href)
    })
}
```

This is a critical pattern difference from the existing `apiClient` — binary downloads require fetch+blob, not `apiClient.get()` which calls `res.json()`.

### Frontend: Inline Preview Fetch

For text preview, use `apiClient` with a typed response (the backend returns plain text, but we can read it as text):

```typescript
// Custom fetch for text preview — apiClient.get() calls .json() which fails on plain text
async function fetchPreview(slug: string, runId: string, path: string): Promise<string> {
  const token = useAuthStore.getState().token
  const res = await fetch(
    `/api/projects/${slug}/runs/${runId}/files?path=${encodeURIComponent(path)}&preview=true`,
    { headers: { Authorization: `Bearer ${token}` } }
  )
  if (!res.ok) throw new Error('preview failed')
  return res.text()
}
```

### Frontend: File Type Detection

WDL outputs are `map[string]interface{}` in `OutputsJSON`. File outputs are string values starting with `file://` or containing `/` path separators, or matching S3 URI pattern `s3://`. Primitive outputs are numbers, booleans, or plain strings without path-like patterns.

Detection heuristic (at discretion):
- `typeof value === 'string' && (value.startsWith('/') || value.startsWith('s3://') || value.startsWith('file://'))` → treat as File
- Otherwise → show as text value

### Frontend: Duration Formatting

Calculate from `created_at` (start) and `updated_at` (end/last-updated) timestamps. For running jobs, show elapsed from `created_at` to now. Human-readable format at Claude's discretion — e.g., "2m 30s" using a simple formatter.

### Frontend: Route Registration

In `web/src/App.tsx`, add inside the ProtectedRoute > AppShell block:

```tsx
<Route path="projects/:slug/runs" element={<RunsPage />} />
<Route path="projects/:slug/runs/:id" element={<RunDetailPage />} />
```

The Sidebar already has a "Runs" nav link at `/projects/${activeSlug}/runs` (verified in `Sidebar.tsx` line 67). No Sidebar changes needed.

### Recommended Project Structure

```
internal/api/types.go              + RunListItem type
internal/store/store.go            + ListRunsByProject interface method
internal/store/sqlite/runs.go      + ListRunsByProject implementation
internal/server/handlers.go        + handleListRuns, handleFileDownload
internal/server/server.go          + route registration
web/src/features/runs/
├── useRuns.ts
├── RunsPage.tsx
└── RunDetailPage.tsx
web/src/App.tsx                    + 2 new routes
```

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| File streaming to HTTP response | Custom buffering loop | `backend.DownloadFileStream(path, w)` | Already handles both local and S3; streaming avoids OOM on large files |
| Auth token injection | Manual header construction | `useAuthStore.getState().token` + standard `fetch` | Same pattern as `apiClient.ts` |
| Route middleware | Custom auth check in handler | `r.With(s.projectRoleRequired("viewer"))` | Already handles project membership + role check |
| Project ID extraction | Parse slug again | `projectIDFromContext(r.Context())` | Already injected by `projectContext` middleware |

**Key insight:** The storage backend's `DownloadFileStream` already abstracts S3 vs local — the handler is storage-agnostic.

---

## Common Pitfalls

### Pitfall 1: Writing error response after starting to stream

**What goes wrong:** Handler sets Content-Disposition, starts writing response body via `DownloadFileStream`, then hits an error mid-stream and tries to call `writeError()`. By then, `WriteHeader` was already called (implicitly on first Write), so the status code cannot be changed.

**Why it happens:** `http.ResponseWriter` is write-once for headers.

**How to avoid:** For download mode, validate the path and verify the run exists BEFORE calling `DownloadFileStream`. Only start streaming when confident the file exists. Log mid-stream errors but do not attempt to write an error body.

**Warning signs:** `superfluous response.WriteHeader call` in logs; truncated downloads.

### Pitfall 2: Using apiClient.get() for binary/text downloads

**What goes wrong:** `apiClient.get<T>()` calls `res.json()` on the response. Binary file data and `text/plain` responses are not valid JSON — this throws a parse error.

**Why it happens:** The existing `apiClient` is designed for JSON API responses only.

**How to avoid:** Use raw `fetch()` with `res.blob()` for downloads and `res.text()` for text preview. Both paths need the `Authorization: Bearer` header injected manually from `useAuthStore.getState().token`.

**Warning signs:** `SyntaxError: Unexpected token` in the browser console on download click.

### Pitfall 3: Path traversal in file download endpoint

**What goes wrong:** `?path=../../etc/passwd` could traverse outside the run's output directory if not validated.

**Why it happens:** The path parameter is passed directly to the storage backend.

**How to avoid:** Validate that the requested path starts with the run's expected output directory prefix. For local backend, `path` should start with `s.cfg.BaseDir`. For S3, keys are namespaced under the run ID. At minimum, reject paths containing `..`.

**Warning signs:** Security audit failures; file contents leaking from outside run directories.

### Pitfall 4: OutputsJSON parsing when empty or null

**What goes wrong:** `handleOutputs` (existing) checks `run.OutputsJSON != ""` before unmarshaling. A completed run created before the OutputsJSON column existed may have empty OutputsJSON even if outputs exist.

**Why it happens:** The existing `handleOutputs` has a fallback to read `outputs.json` from the filesystem. The frontend using `GET /{id}/outputs` already handles this transparently.

**How to avoid:** Frontend uses the existing `GET /projects/{slug}/runs/{id}/outputs` endpoint (not a new one) for run detail output display. This endpoint already handles the backward-compat fallback.

**Warning signs:** Empty outputs table for old runs.

### Pitfall 5: Sidebar already has Runs link

**What goes wrong:** Planner adds a Sidebar task to add the Runs link, but it already exists.

**Why it happens:** The Sidebar in `Sidebar.tsx` line 67 already includes `{ to: /projects/${activeSlug}/runs, label: 'Runs', icon: Play }`.

**How to avoid:** Do NOT add a Sidebar task. Only add App.tsx route registration.

---

## Code Examples

### Existing Pattern: DownloadFileStream (verified)

```go
// Source: internal/storage/local.go LocalBackend.DownloadFileStream
func (l *LocalBackend) DownloadFileStream(path string, w io.Writer) error {
    f, err := os.Open(path)
    // ...
    io.Copy(w, f)
}

// Source: internal/storage/s3.go S3Backend.DownloadFileStream
func (b *S3Backend) DownloadFileStream(path string, w io.Writer) error {
    out, err := b.s3c.GetObject(context.Background(), &s3.GetObjectInput{...})
    // ...
    io.Copy(w, out.Body)
}
```

Both `io.Writer` implementations — pass `http.ResponseWriter` directly.

### Existing Pattern: scanRunRows (for ListRunsByProject)

```go
// Source: internal/store/sqlite/runs.go
func scanRunRows(rows *sql.Rows) (*store.Run, error) {
    var r store.Run
    var state string
    var outputsJSON sql.NullString
    var projectID sql.NullString
    err := rows.Scan(&r.ID, &r.WDLPath, &r.InputsJSON, &state, &outputsJSON,
        &r.CreatedAt, &r.UpdatedAt, &projectID, &r.RegistryRef, &r.RegistryDigest)
    // ...
}
```

Reuse this function directly for `ListRunsByProject` — same SELECT column order.

### Existing Pattern: Auth-enabled test server

```go
// Source: internal/server/handlers_auth_test.go
func newAuthTestServer(t *testing.T) (*server.Server, *sqlite.SQLiteStore, func()) {
    cfg := &config.Config{
        Auth: config.AuthConfig{Enabled: true, JWTSecret: string(testJWTSecret)},
        // ...
    }
    // ...
}

func doAuthRequest(t *testing.T, srv *server.Server, method, path string, body interface{}, token string) *httptest.ResponseRecorder {
    // ...
    req.Header.Set("Authorization", "Bearer "+token)
    // ...
}
```

New handler tests for `handleListRuns` and `handleFileDownload` must use `newAuthTestServer` + `doAuthRequest` because they are project-scoped endpoints requiring JWT.

### Existing Pattern: React Query hook with slug guard

```typescript
// Source: web/src/features/registry/useRegistry.ts
export function useRegistryEntries(slug: string) {
  return useQuery<RegistryEntryResponse[]>({
    queryKey: ['projects', slug, 'registry'],
    queryFn: () => apiClient.get(`/projects/${slug}/registry`),
    enabled: !!slug,
  })
}
```

`enabled: !!slug` prevents firing before the slug is available from `useParams`.

### Existing Pattern: Loading skeleton rows

```tsx
// Source: web/src/features/registry/RegistryPage.tsx
{isLoading ? (
  Array.from({ length: 4 }).map((_, i) => (
    <TableRow key={i} data-testid="skeleton-row">
      <TableCell><div className="h-4 w-24 animate-pulse rounded bg-muted" /></TableCell>
      // ...
    </TableRow>
  ))
) : ...}
```

Use 5 skeleton rows for the runs table (one per column).

---

## State of the Art

| Old Approach | Current Approach | Notes |
|--------------|------------------|-------|
| Presigned S3 URLs for downloads | Backend proxy streaming | Locked decision: no direct S3 access |
| File hierarchy browser | Flat key/value output table | WDL outputs are named pairs, not a filesystem |
| Pagination | No pagination for v2.0 | Locked decision |

---

## Open Questions

1. **Path validation for file download**
   - What we know: OutputsJSON contains the output file paths as stored by the storage backend (either local absolute paths or S3 keys)
   - What's unclear: Whether local paths always start with `s.cfg.BaseDir` — they should based on `RunOutputsDir`, but older runs may differ
   - Recommendation: Validate that path does not contain `..` as a minimum; document as acceptable for v2.0 internal tool

2. **Content-Length header for file downloads**
   - What we know: `DownloadFileStream` streams without knowing size up front for S3 (no `HeadObject` call)
   - What's unclear: Whether browsers need Content-Length for progress indicators
   - Recommendation: Omit Content-Length for v2.0; browsers handle chunked transfer encoding correctly

3. **Run detail page when run is not completed**
   - What we know: Left to Claude's discretion — show status but no outputs
   - Recommendation: Show metadata section always; outputs table shows "Run not yet completed" message when `state !== 'COMPLETED'`

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Vitest (frontend), Go test (backend) |
| Config file | `web/vitest.config.ts` (frontend), standard `go test` (backend) |
| Quick run command (frontend) | `cd /home/uehara/flowy/web && pnpm test --run` |
| Quick run command (backend) | `cd /home/uehara/flowy && go test ./internal/store/sqlite/... ./internal/server/... -count=1` |
| Full suite command | `cd /home/uehara/flowy/web && pnpm test --run && cd /home/uehara/flowy && go test ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| FILE-01 | `ListRunsByProject` store method returns runs ordered newest-first | unit | `go test ./internal/store/sqlite/... -run TestListRunsByProject` | ❌ Wave 0 |
| FILE-01 | `GET /projects/{slug}/runs` returns 200 with run list | unit | `go test ./internal/server/... -run TestHandleListRuns` | ❌ Wave 0 |
| FILE-01 | RunsPage renders run list table with status badges | unit | `cd web && pnpm test --run -- runs/RunsPage` | ❌ Wave 0 |
| FILE-02 | RunDetailPage renders metadata + outputs table | unit | `cd web && pnpm test --run -- runs/RunDetailPage` | ❌ Wave 0 |
| FILE-02 | RunDetailPage shows "not completed" message for non-COMPLETED runs | unit | `cd web && pnpm test --run -- runs/RunDetailPage` | ❌ Wave 0 |
| FILE-03 | `GET /projects/{slug}/runs/{id}/files?path=...` streams file with Content-Disposition | unit | `go test ./internal/server/... -run TestHandleFileDownload` | ❌ Wave 0 |
| FILE-03 | Download button triggers fetch+blob in browser | unit | `cd web && pnpm test --run -- runs/RunDetailPage` | ❌ Wave 0 |
| FILE-04 | `GET .../files?path=...&preview=true` returns text/plain, max 100KB | unit | `go test ./internal/server/... -run TestHandleFileDownload` | ❌ Wave 0 |
| FILE-04 | Preview expands inline in output row | unit | `cd web && pnpm test --run -- runs/RunDetailPage` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `go test ./internal/store/sqlite/... -count=1` (backend) or `cd /home/uehara/flowy/web && pnpm test --run` (frontend)
- **Per wave merge:** Both backend and frontend test suites
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `internal/store/sqlite/runs_test.go` — add `TestListRunsByProject` tests
- [ ] `internal/server/handlers_runs_test.go` — new file for `TestHandleListRuns`, `TestHandleFileDownload`
- [ ] `web/src/features/runs/__tests__/RunsPage.test.tsx` — new file
- [ ] `web/src/features/runs/__tests__/RunDetailPage.test.tsx` — new file

---

## Sources

### Primary (HIGH confidence)

- `internal/server/server.go` — route structure, middleware patterns, runs route group
- `internal/server/handlers.go` — handleOutputs as model for new handlers
- `internal/server/server_test.go` — newTestServer, doRequest patterns
- `internal/server/handlers_auth_test.go` — newAuthTestServer, doAuthRequest for auth-enabled tests
- `internal/store/store.go` — Store interface, Run struct fields
- `internal/store/sqlite/runs.go` — CreateRun, scanRunRows patterns
- `internal/storage/backend.go` — Backend interface, DownloadFileStream
- `internal/storage/local.go` — LocalBackend.DownloadFileStream implementation
- `internal/storage/s3.go` — S3Backend.DownloadFileStream implementation
- `internal/api/types.go` — existing response type patterns
- `web/src/features/registry/useRegistry.ts` — React Query hook pattern
- `web/src/features/registry/RegistryPage.tsx` — table + badge + navigate pattern
- `web/src/features/registry/RegistryEntryPage.tsx` — metadata header + table detail pattern
- `web/src/features/registry/WDLSourceViewer.tsx` — code viewer for inline preview
- `web/src/lib/apiClient.ts` — existing API client (JSON-only; binary needs raw fetch)
- `web/src/App.tsx` — route registration pattern
- `web/src/components/layout/Sidebar.tsx` — already has Runs nav link (no changes needed)
- `.planning/config.json` — nyquist_validation: true (validation section required)

### Secondary (MEDIUM confidence)

None — all findings are from primary codebase inspection.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries are in-project, versions verified by inspection
- Architecture: HIGH — patterns verified from existing handlers and frontend features
- Pitfalls: HIGH — derived from reading actual code, not speculation
- Validation: HIGH — existing test infrastructure verified by file inspection

**Research date:** 2026-04-07
**Valid until:** 2026-05-07 (stable codebase, patterns unlikely to change)
