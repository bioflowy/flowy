# Phase 25: Infrastructure Foundation - Context

**Gathered:** 2026-03-20
**Status:** Ready for planning

<domain>
## Phase Boundary

Create the `flowy-ui` Go binary that serves an embedded React SPA via go:embed and reverse-proxies API requests to flowy-server. Scaffold the React + TypeScript + Vite + shadcn/ui + Tailwind v4 project. Establish the build pipeline (Makefile) and add CORS middleware to flowy-server for development mode. This phase delivers the empty app shell — no feature screens yet.

</domain>

<decisions>
## Implementation Decisions

### SPA Serving Architecture
- `flowy-ui` is a separate Go binary in `cmd/flowy-ui/`
- Embeds the built SPA from `web/dist/` using `//go:embed all:web/dist`
- Uses `fs.Sub` to strip the embedded directory prefix
- SPA fallback handler: intercepts 404s from the file server and re-serves `index.html` (excluding `/api/*` paths which return JSON errors)
- Default port: `:3000` (configurable via flag)
- `--api-addr` flag specifies flowy-server address (default `http://localhost:8124`)

### API Proxy Design
- UI accesses API via `/api/*` prefix (e.g., `/api/runs`, `/api/projects`)
- flowy-ui strips `/api/` prefix and proxies to flowy-server (which has no prefix)
- Uses `net/http/httputil.ReverseProxy` with `Rewrite` field (not deprecated `Director`)
- In production: same-origin proxy eliminates CORS entirely

### Development Workflow
- Run both `flowy-ui` and Vite dev server simultaneously
- flowy-ui serves the Go binary + proxy; Vite provides HMR
- Vite's `server.proxy` also configured for `/api` → flowy-server as a fallback dev option
- CORS middleware on flowy-server: config-gated (`cors.enabled: true` in flowy.yaml), registered at top-level chi router before JWT middleware

### Layout Shell
- Left sidebar navigation, GitHub-style (clean, information-dense)
- Sidebar contains: project selector at top, then feature menu (Registry, Runs, Members, etc.)
- Global items: Users (admin only), account menu at bottom
- Content area to the right of sidebar
- No mobile responsiveness required for v2.0

### Build Pipeline
- Package manager: pnpm
- `make build-ui` target: runs `cd web && pnpm install && pnpm build` then `go build -o ./flowy-ui ./cmd/flowy-ui`
- Frontend build output: `web/dist/`
- Makefile integrates into existing build system alongside `build` target for Go binaries

### Project Structure
- Frontend workspace: `web/` at repository root
- Feature-based directory layout: `web/src/features/users/`, `web/src/features/projects/`, etc.
- Shared components: `web/src/components/` (shadcn/ui components)
- API client: plain `fetch` + hand-written functions per endpoint (no axios)
- State: TanStack Query v5 for server state, zustand for client state (auth token)
- Routing: React Router v7 in library mode (`<BrowserRouter>` + `<Routes>`)

### Claude's Discretion
- Sidebar navigation item grouping and icons
- Exact spacing, typography, color scheme within shadcn/ui defaults
- Error and loading state patterns
- Vite + TypeScript configuration details
- flowy-ui config file format (flags only vs YAML)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing server architecture
- `internal/server/server.go` — `(*Server).newRouter()` is where chi middleware chain is assembled; CORS must be added here
- `internal/config/config.go` — Config struct; needs new CORS config field
- `cmd/flowy-server/main.go` — Server binary entry point; reference for flowy-ui binary structure

### Existing build system
- `Makefile` — Current build targets; `build-ui` target must integrate alongside existing `build` target

### Research outputs
- `.planning/research/STACK.md` — Pinned library versions (React 19, Vite 6, Tailwind v4, shadcn/ui, TanStack Query v5, React Router v7)
- `.planning/research/ARCHITECTURE.md` — go:embed + SPA fallback pattern, reverse proxy architecture, Vite proxy config
- `.planning/research/PITFALLS.md` — SPA routing fallback, go:embed fs.Sub, CORS middleware positioning, Vite base path

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/config/config.go` Config struct: add `CORS CORSConfig` field following existing pattern (StorageConfig, AuthConfig)
- `cmd/flowy-server/main.go`: reference for binary structure (flag parsing, config load, graceful shutdown)
- `internal/server/server.go` `newRouter()`: chi middleware chain where CORS middleware must be inserted at top

### Established Patterns
- chi v5 middleware chain: Logger → Recoverer → (CORS here) → JWT → route groups
- YAML config with `applyDefaults()` for zero-value fields
- Separate binary per concern (`cmd/flowy`, `cmd/flowy-server`, `cmd/flowy-daemon`)
- Flag-based config path: `-config ./flowy.yaml`

### Integration Points
- `cmd/flowy-ui/main.go` — new binary, follows existing cmd/ pattern
- `Makefile` — new `build-ui` target, new `UI_BIN` variable
- `internal/server/server.go` `newRouter()` — CORS middleware insertion point
- `internal/config/config.go` — new CORSConfig struct
- `web/` — new directory at repository root (not inside cmd/)

</code_context>

<specifics>
## Specific Ideas

- GitHub-style UI: clean, information-dense sidebar with project context switching
- flowy-ui + Vite HMR development workflow (both running simultaneously)
- /api/* prefix for all API calls from frontend, stripped by proxy before reaching flowy-server

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 25-infrastructure-foundation*
*Context gathered: 2026-03-20*
