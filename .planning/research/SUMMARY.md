# Project Research Summary

**Project:** flowy v2.0 — React Web UI management SPA
**Domain:** React SPA embedded in Go binary, consuming an existing chi v5 REST API
**Researched:** 2026-03-20
**Confidence:** HIGH

## Executive Summary

flowy v2.0 adds a browser-based management UI to an existing, fully functional Go workflow
engine. The approach is a pure SPA (React 19 + TypeScript + Vite 6) embedded into a new
`flowy-ui` Go binary via `go:embed`. In production the binary serves the SPA and reverse-proxies
`/api/*` to `flowy-server`, keeping both on the same origin and eliminating CORS entirely. In
development, the Vite dev server handles hot-module replacement and proxies API calls, so the Go
binary never needs to be rebuilt during UI development. This is a well-understood pattern with
official Go and Vite documentation, and the existing backend requires only one minimal change: a
config-gated CORS middleware added to `flowy-server` for development mode.

All five screens — Login, User Management, Project Management, Registry Browsing, and File
Browsing — map directly to existing REST endpoints. The one exception is the File Browser, which
requires three new backend endpoints (`GET /projects/{slug}/runs`, a file listing endpoint, and a
file download/presigned-URL endpoint) that do not exist yet. This is the only hard backend
dependency blocking a frontend phase. Everything else can be built against the current API surface.

The main risks are implementation-level rather than architectural. The SPA routing fallback on the
Go side must be wired correctly from day one (browser refresh breaks all routes otherwise), the
`fs.Sub` call on the embedded FS must be present (otherwise all assets 404), and the JWT storage
strategy must be decided before any auth code is written. None of these risks are blocking; all
have clear, verified prevention strategies documented in PITFALLS.md.

## Key Findings

### Recommended Stack

The frontend adds React 19, TypeScript 5.x, Vite 6, Tailwind CSS v4, shadcn/ui, React Router v7
(library mode), TanStack Query v5, zustand, and axios to the project. Every version choice is
driven by shadcn/ui compatibility: Vite 6 (not 8) because shadcn CLI targets 6; Tailwind v4
because shadcn now defaults to it; React 19 because both TanStack Query v5 and shadcn fully
support it. The Go side adds only `github.com/go-chi/cors` and a new `cmd/flowy-ui/` binary — the
existing codebase is otherwise untouched.

See `.planning/research/STACK.md` for full library comparison, rejected alternatives, version
compatibility matrix, and installation commands.

**Core technologies:**
- React 19.2.4 — UI rendering — current stable; React 19 removes `forwardRef` boilerplate
- TypeScript 5.x (5.9.3 stable) — type safety — TS 6.0 is RC-only as of March 2026; stay on 5.x
- Vite 6.x — build tool and dev server — shadcn CLI targets Vite 6; avoid Vite 8 until shadcn catches up
- Tailwind CSS v4 — styling — CSS-first config (`@import "tailwindcss"`); no `tailwind.config.js`
- shadcn/ui (CLI-managed) — component library — React 19 + Tailwind v4 fully supported; never edit generated files manually
- React Router v7 library mode — client routing — use `react-router` package only (v7 merged `react-router-dom`); library mode is not Remix
- TanStack Query v5 — server state — CRUD-heavy management UI benefits from `invalidateQueries` + mutation model
- zustand v5 — client state — auth token + user session; avoids React Context re-render churn for global state
- axios v1 — HTTP client — request/response interceptors for global JWT injection and 401 redirect
- `go:embed` + `httputil.ReverseProxy` — production binary — single binary deployment; no Node.js runtime needed in production
- Node.js 22 LTS — runtime for build tools — Vite 6 requires Node 18+

**Critical version notes:**
- Use Vite 6, NOT Vite 8 (Rolldown-based; shadcn init targets Vite 6)
- Use `react-router` (v7), NOT `react-router-dom` (merged in v7; both packages would be redundant)
- Use Tailwind v4, NOT v3 (shadcn CLI defaults to v4 as of early 2026)
- TypeScript 5.x, NOT 6.0 (6.0 is RC as of March 2026)
- `//go:embed all:web/dist` — the `all:` prefix is required to include dot-files in the Vite output

### Expected Features

All five screens are required for v2.0 to be useful. Login is the prerequisite for all other
screens. User Management and Project Management can be built in parallel. Registry Browsing
depends on project selection being navigable. File Browsing is blocked by three missing backend
endpoints and should be the last screen built.

See `.planning/research/FEATURES.md` for full feature tables, competitor analysis, MVP definition,
and complete feature dependency graph.

**Must have (table stakes):**
- Login screen: username/password form, JWT session (sessionStorage or httpOnly cookie), 401 auto-redirect, disable-on-submit
- User Management (admin only): list users, create user, deactivate user — no edit (API has no `PUT /users/{id}`)
- Project list + create; project detail with description edit and member management (add/change-role/remove)
- Registry entry list with kind badge (tool/workflow) and tags; entry detail with version history, WDL content viewer, tag management
- Run list per project with status badges — requires new `GET /projects/{slug}/runs` endpoint
- File browser: output file list + download per run — requires three new backend endpoints
- CORS support on flowy-server (dev mode only, config-gated via `cors.enabled`)
- `flowy-ui` binary embedding built SPA and proxying `/api/*` to flowy-server with configurable `--api-addr` flag

**Should have (v2.0 if time allows):**
- WDL syntax highlighting in content viewer (highlight.js or Prism)
- Client-side form validation before API calls (slug format regex, password length)
- Presigned S3 URL redirect for file downloads (browser downloads directly from S3 for large files)
- Prevent-last-admin client-side hint on project member removal
- Role badge color coding (admin/executor/viewer)
- Digest truncated display with copy-full-digest on click

**Defer to v2.x or later:**
- Run submission form (requires WDL input schema → form generation)
- WDL diff viewer (two-version side-by-side comparison)
- Provenance detail screen (full inputs snapshot rendered as table)
- Refresh token / auto-renew
- Real-time run status via WebSocket or SSE
- OAuth2/SSO login
- Bioinformatics file viewers (IGV.js, BAM/VCF in-browser viewing)

### Architecture Approach

The architecture is a clean separation of concerns: `flowy-ui` binary is a pure static file server
plus transparent reverse proxy with no domain knowledge; `flowy-server` is unchanged except for a
config-gated CORS middleware; the React SPA communicates exclusively with `flowy-server` through
the `/api/` prefix intercepted by both the Vite proxy (dev) and the Go reverse proxy (prod).
State management is split: TanStack Query owns all server state; zustand (or React context) owns
auth token and user session; component local state handles UI-only concerns such as form values
and modal open state.

See `.planning/research/ARCHITECTURE.md` for full component diagram, code examples for all key
patterns, data flow diagrams (login, authenticated fetch, 401 handling), CORS configuration, and
the eight-step build order.

**Major components:**
1. `cmd/flowy-ui/main.go` — `go:embed all:web/dist`; `spaHandler` with 404-fallback to `index.html`; `httputil.ReverseProxy` at `/api/`; configurable `--api-addr` flag; startup assertion on embedded FS
2. `web/src/api/client.ts` — centralized fetch/axios wrapper; module-level token state; `ApiError` class with HTTP status; 401 handler
3. `web/src/context/AuthContext.tsx` (or zustand auth store) — JWT state; login/logout; sessionStorage hydration on mount
4. `web/src/hooks/` — TanStack Query hooks per resource (`useUsers`, `useProjects`, `useRegistry`, `useRuns`, `useProvenance`)
5. `web/src/pages/` — route-level components (LoginPage, UsersPage, ProjectsPage, RegistryPage, FileBrowserPage)
6. `internal/server/server.go` (modified) — `go-chi/cors` middleware, registered at top-level router only, config-gated

**Note on directory naming:** STACK.md uses `web/` while ARCHITECTURE.md uses `ui/` for the
frontend workspace and corresponding `go:embed` path. Pick one name in Phase 1 and use it
everywhere.

**Build pipeline:**
```
make build-ui:
  cd web && npm run build     # outputs web/dist/
  go build ./cmd/flowy-ui/   # go:embed picks up web/dist/
```

### Critical Pitfalls

See `.planning/research/PITFALLS.md` for full detail including recovery costs, verification
checklists, security mistakes, and the "Looks Done But Isn't" acceptance checklist.

1. **SPA routing breaks on browser refresh** — React Router routes exist only in JS; hard refresh sends real HTTP to the Go server, which has no matching route and returns JSON 404. Fix: `spaHandler` with 404-intercept that returns `index.html` for all non-`/api/` paths. Must be in place before any route is tested. Verify by pressing F5 on `/projects` — must return the React app, not a JSON error.

2. **`go:embed` wrong subtree — all assets 404** — `//go:embed all:web/dist` preserves the `web/dist/` prefix inside the embedded FS. Without `fs.Sub(embeddedFS, "web/dist")`, the file server finds nothing at `index.html`. Add `fs.Sub` immediately and add a startup assertion (`fs.Stat(uiFS, "index.html")`) that panics if the subtree is wrong.

3. **CORS middleware in wrong router position — OPTIONS preflight returns 404** — `go-chi/cors` must be registered at the top-level chi router (`r.Use(cors.Handler(...))`), before any route groups. Placement inside a route group causes preflight OPTIONS requests to hit `NotFound` before the middleware runs. Never use `AllowedOrigins: ["*"]` with `AllowCredentials: true` — browsers refuse this combination per the CORS spec.

4. **JWT in localStorage — XSS token theft** — For an internal tool managing genomic data with RBAC, a stolen admin JWT grants full access to all projects, registry entries, and output files. Use `httpOnly; SameSite=Strict` cookies set by the server, or at minimum `sessionStorage` (not `localStorage`). This decision must be made before any auth code is written; retrofitting requires coordinated frontend + backend changes.

5. **TanStack Query retries 401 three times by default** — When a JWT expires, every active query fires simultaneously and each retries 3x, creating a flood of concurrent auth calls. Configure `QueryClient.defaultOptions.queries.retry` to return `false` for 401 and 403 before building any authenticated screen.

6. **S3 file downloads buffered in memory** — `io.ReadAll(s3Result.Body)` allocates the entire file on the Go heap. For 2 GB BAM files this causes OOM. Always stream with `io.Copy(w, result.Body)`. Presigned S3 URL redirect (browser downloads directly from S3) is even better for large files and eliminates the memory issue entirely.

7. **`flowy-ui` API URL hardcoded** — `localhost:8124` baked in makes the binary unusable in any real deployment. Expose as `--api-addr` flag from the first line of `cmd/flowy-ui/main.go`.

8. **Vite default `base: "/"` breaks sub-path deployment** — Absolute asset paths in `index.html` break when `flowy-ui` is placed behind a reverse proxy at a non-root path. Set `base: "./"` in `vite.config.ts` at project initialization. Verify: `grep 'src="/' web/dist/index.html` must return nothing.

## Implications for Roadmap

The architecture research prescribes a clear build order driven by two dependency types: the Go
binary scaffold must exist before the SPA can be integrated, and certain screens depend on others
or on new backend endpoints. The recommended phase structure directly maps to the eight-step build
order in ARCHITECTURE.md.

### Phase 1: Binary Scaffold and SPA Foundation

**Rationale:** All frontend work requires the Go binary and SPA skeleton to exist. The four
critical pitfalls that affect the binary (SPA routing fallback, go:embed subtree, Vite base path,
hardcoded API URL) must all be resolved here — retrofitting them later disrupts every subsequent
phase.

**Delivers:**
- `cmd/flowy-ui/main.go` with `go:embed all:web/dist`, `spaHandler` with 404 fallback, `httputil.ReverseProxy`, `--api-addr` flag
- `web/` Vite 6 + React 19 + TypeScript 5.x + Tailwind v4 + shadcn/ui scaffold
- `web/vite.config.ts` with `base: "./"` and `/api` proxy to `flowy-server`
- `web/src/api/client.ts` base HTTP client (token injection layer, `ApiError` class)
- `make build-ui` target with correct dependency ordering (npm build before go build)
- Startup assertion: `fs.Stat(uiFS, "index.html")` panics on wrong subtree

**Avoids pitfalls:** SPA routing fallback (Pitfall 1), go:embed wrong subtree (Pitfall 2), Vite absolute base path (Pitfall 8), hardcoded API URL (Pitfall 7)

### Phase 2: CORS and Authentication

**Rationale:** CORS must be configured before any authenticated API call can be made from the
Vite dev server. JWT session strategy must be locked in before any auth code is written —
changing storage strategy later requires coordinated frontend + backend changes.

**Delivers:**
- `internal/config/config.go` — `CORSConfig` struct (enabled flag, allowed origins)
- `internal/server/server.go` — `go-chi/cors` at top-level router, config-gated
- `go.mod` — `github.com/go-chi/cors`
- `web/src/context/AuthContext.tsx` — JWT state, login, logout, sessionStorage hydration on mount
- `web/src/pages/LoginPage.tsx` — username/password form, disable-on-submit, error display, redirect to original URL after login
- `QueryClient` with custom `retry` (disabled for 401/403)
- Protected route wrapper that redirects unauthenticated users to `/login`
- Auto-logout on 401: global error handler in QueryClient calls `AuthContext.logout()`

**Avoids pitfalls:** CORS middleware position (Pitfall 3), JWT in localStorage (Pitfall 4), TanStack Query retry storm (Pitfall 5)

### Phase 3: User Management

**Rationale:** User Management is independent of all other screens except Login. It can be built
immediately after auth is wired, in parallel with Project screens. Admin-only route gating
(checking `global_role === "admin"` from JWT claims) is validated here before it is needed
elsewhere.

**Delivers:**
- `web/src/api/users.ts` + `web/src/hooks/useUsers.ts`
- `web/src/pages/UsersPage.tsx` — list users with active/inactive status, create user form, deactivate with confirm dialog
- Admin-only route guard (hide from nav, guard route for non-admins)
- Client-side filter by username or display name
- No edit fields for fields with no corresponding PUT endpoint (no phantom `PUT /users/{id}`)

### Phase 4: Project Management

**Rationale:** Projects are the organizing unit for all subsequent screens. Registry and File
Browsing both require a project slug in the URL. The project list must be navigable before any
project-scoped screen can be reached.

**Delivers:**
- `web/src/api/projects.ts` + `web/src/hooks/useProjects.ts`
- `web/src/pages/ProjectsPage.tsx` — project list (cards), create project with slug format validation
- `web/src/pages/ProjectDetailPage.tsx` — description edit, member list with role badges, add/change/remove member, prevent-last-admin hint
- Role badge color coding (admin/executor/viewer)
- Slug is read-only after creation (no slug-change API endpoint exists)

### Phase 5: Registry Browsing

**Rationale:** Registry is the read-oriented core of the workflow engine UI. All endpoints exist
in the current API. This phase depends on Phase 4 (project slug must be in the URL route).

**Delivers:**
- `web/src/api/registry.ts` + `web/src/hooks/useRegistry.ts`
- `web/src/pages/RegistryPage.tsx` — entry list with kind badge (tool/workflow), tags as chips, filter by kind
- Registry entry detail: version history with digest truncation, WDL content viewer, tag management (admin), copy registry reference to clipboard
- WDL syntax highlighting (highlight.js with WDL grammar, or generic fallback)

**Open question for planning:** Verify highlight.js WDL grammar or Prism WDL plugin availability before implementation. Fall back to generic code highlighting if no WDL-specific grammar is satisfactory.

### Phase 6: File Browsing and Download

**Rationale:** This is the only phase with hard backend dependencies. Three new endpoints must be
built before any frontend work can proceed. S3 streaming and presigned URL patterns have specific
correctness requirements distinct from the other screens.

**Delivers (backend — new endpoints on flowy-server):**
- `GET /projects/{slug}/runs` — list runs with `run_state`, `created_at`, `registry_ref`
- `GET /projects/{slug}/runs/{id}/files` — list output files resolved from OutputsJSON
- `GET /projects/{slug}/runs/{id}/files/download?path=...` — stream file bytes OR redirect to presigned S3 URL

**Delivers (frontend):**
- `web/src/api/runs.ts` + `web/src/hooks/useRuns.ts`
- `web/src/pages/FileBrowserPage.tsx` — run list with status badges, filter by status, output file list, download button
- `Content-Disposition: attachment` and `Content-Length` headers on all downloads
- Streaming via `io.Copy` (never `io.ReadAll`)
- Authorization check: validate that requested file path belongs to a run the caller has viewer access to
- Presigned S3 URL redirect for S3 backend (browser downloads directly from S3)

**Avoids pitfalls:** S3 download memory buffering (Pitfall 6), missing Content-Disposition header, download authorization absent

### Phase 7: Production Integration and Build Validation

**Rationale:** The `go:embed` production path must be validated end-to-end before any phase is
considered production-ready. This is the integration gate.

**Delivers:**
- Verified `make build-ui` producing a single `flowy-ui` binary that serves the full SPA
- All items from PITFALLS.md "Looks Done But Isn't" checklist verified green
- Production CORS disabled verification (flowy-server must NOT have `cors_enabled: true` in production)
- Deployment documentation

### Phase Ordering Rationale

- Phase 1 before all others: the binary and SPA scaffold are the foundation; all embed, routing fallback, Vite base path, and API URL pitfalls must be resolved here or they block every subsequent phase.
- Phase 2 before Phases 3-6: all authenticated screens require JWT session and 401 handling; CORS configuration must precede any API call from the Vite dev server.
- Phases 3 and 4 are independent of each other and can be parallelized after Phase 2 completes.
- Phase 5 requires Phase 4 (project slug must be navigable before entering registry; TanStack Query query key `['registry', slug]` requires slug from the URL).
- Phase 6 is last among feature phases because it requires new backend endpoints and has the most complex implementation (streaming, presigned URLs, authorization checks on download paths).
- Phase 7 is the integration gate and must follow all feature phases.

### Research Flags

Phases needing deeper research during planning:
- **Phase 6 (File Browsing):** Three new backend endpoints require Store-layer design (`ListRunsByProject`, output file resolution from OutputsJSON). S3 presigned URL redirect pattern (AWS SDK v2 `PresignGetObject`, 302 redirect flow, S3 bucket CORS policy for presigned URLs) needs implementation detail verification. Download authorization logic needs explicit design (what membership check applies to a file path from OutputsJSON).

Phases with standard patterns (skip `/gsd:research-phase`):
- **Phase 1:** go:embed, Vite scaffold, `fs.Sub`, `httputil.ReverseProxy` are all documented in official Go and Vite sources with HIGH confidence.
- **Phase 2:** chi CORS middleware, React auth context pattern, TanStack Query retry configuration are well-documented standard patterns.
- **Phase 3:** User CRUD against existing endpoints — no novel patterns.
- **Phase 4:** Project + member CRUD against existing endpoints — no novel patterns.
- **Phase 5:** Registry browsing against existing endpoints. WDL highlighting worth a quick check during implementation but does not block planning.
- **Phase 7:** Integration verification is mechanical; no research needed.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All technology choices verified against official docs. Version pinning is specific and based on a compatibility matrix (React 19 + shadcn/ui + TanStack Query v5 + Vite 6 + Tailwind v4 all confirmed compatible). |
| Features | HIGH (screens 1-4), MEDIUM (screen 5) | Screens 1-4 derived from direct code inspection of `server.go`, `types.go`, `store.go`. File browser feature scope is MEDIUM because the three required backend endpoints do not exist yet — scope may shift when they are designed. |
| Architecture | HIGH | Go stdlib patterns (`go:embed`, `httputil.ReverseProxy`, `http.FileServer`, `fs.Sub`) sourced from official docs. React architecture patterns (TanStack Query, centralized API client, Auth context) are well-established with multiple confirming sources. |
| Pitfalls | HIGH | All 8 critical pitfalls sourced from official docs or well-regarded community posts with verified reproduction steps and prevention patterns. |

**Overall confidence:** HIGH

### Gaps to Address

- **`web/` vs `ui/` directory name:** STACK.md uses `web/` and ARCHITECTURE.md uses `ui/` for the frontend workspace. Pick one name in Phase 1 planning and use it everywhere (including the `//go:embed` path). Either is fine; consistency matters.
- **JWT storage strategy — final decision:** ARCHITECTURE.md recommends `sessionStorage` (pragmatic for internal tool); PITFALLS.md recommends `httpOnly` cookies (more secure, eliminates XSS token theft). The team must decide before Phase 2 begins. sessionStorage is acceptable for MVP if documented as technical debt. httpOnly cookies require a Go-side `SetCookie` call in `handleLogin` and `credentials: 'include'` in fetch calls.
- **WDL syntax highlighting library:** Prism WDL plugin availability has LOW confidence. Verify `highlight.js` WDL grammar or Prism WDL plugin during Phase 5 planning. Generic code highlighting (any language) is an acceptable fallback and should not block planning.
- **File browser backend scope:** The three new endpoints for Phase 6 have no existing implementation. Store-layer interface changes need design before Phase 6 planning begins. Be explicit about scope: `ListRunsByProject` is a query, output file resolution requires parsing OutputsJSON string values to extract file paths. Keep new endpoints minimal to avoid turning a UI milestone into a large backend feature.

## Sources

### Primary (HIGH confidence)
- flowy source `internal/server/server.go` — complete route map (direct code inspection)
- flowy source `internal/api/types.go` — all request/response DTOs confirming field names and types (direct code inspection)
- flowy source `internal/store/store.go` — Store interface confirming what queries exist and which are missing (direct code inspection)
- flowy source `internal/server/handlers.go` — `handleOutputs` confirms OutputsJSON is a flat key-value map (direct code inspection)
- Go stdlib `embed` package — `pkg.go.dev/embed` — `//go:embed all:` directive
- Go stdlib `net/http/httputil` — `pkg.go.dev/net/http/httputil` — `ReverseProxy.Rewrite` pattern (Go 1.20+)
- go-chi/cors — `pkg.go.dev/github.com/go-chi/cors` — CORS middleware placement and credentials rules
- Vite server.proxy — `vite.dev/config/server-options` — dev proxy configuration
- Vite build options — `vite.dev/guide/build` — `base` option and relative paths
- React 19 release — `react.dev/blog/2024/12/05/react-19` — stable December 2024
- shadcn/ui React 19 docs — `ui.shadcn.com/docs/react-19` — full React 19 compatibility confirmed
- shadcn/ui Tailwind v4 docs — `ui.shadcn.com/docs/tailwind-v4` — Tailwind v4 fully supported
- TanStack Query v5 official docs — `tanstack.com/query/latest` — query keys, retry configuration, mutation invalidation
- React Router v7 SPA mode — `reactrouter.com/how-to/spa` — library mode vs framework mode
- PROJECT.md — explicit out-of-scope items (WebSocket/SSE, OAuth2, run submission via UI)

### Secondary (MEDIUM confidence)
- Serving SPA from Go — `hackandsla.sh/posts/2021-11-06-serve-spa-from-go/` — spaResponseWriter 404-intercept pattern
- Embedding React in Go binary — `smartinary.com/blog/how-to-embed-a-react-app-in-a-go-binary/` — fs.Sub pattern
- JWT token storage comparison — `descope.com/blog/post/developer-guide-jwt-storage` — sessionStorage vs localStorage security tradeoffs
- React Query auth token refresh — `elazizi.com/posts/react-query-auth-token-refresh/` — 401 retry deduplication with TanStack Query
- S3 presigned URLs for React downloads — OneUptime blog — presigned URL redirect pattern
- S3 streaming download — DEV Community — memory pitfalls of buffered (`io.ReadAll`) S3 downloads
- shadcn/ui production pitfalls — `paulserban.eu` — bundle size and barrel import warnings
- State management 2026 survey — zustand as pragmatic default for most React apps
- JWT in React SPAs — marmelab.com — in-memory token pattern and sessionStorage trade-offs

### Tertiary (LOW confidence)
- WDL syntax highlighting — Prism WDL plugin availability is unverified; needs validation during Phase 5 planning before committing to a specific highlighting library.

---
*Research completed: 2026-03-20*
*Ready for roadmap: yes*
