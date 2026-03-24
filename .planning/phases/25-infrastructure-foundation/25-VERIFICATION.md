---
phase: 25-infrastructure-foundation
verified: 2026-03-24T03:20:00Z
status: passed
score: 11/11 must-haves verified
re_verification: false
---

# Phase 25: Infrastructure Foundation Verification Report

**Phase Goal:** The flowy-ui binary exists, serves the embedded React SPA with correct SPA routing fallback, and the build pipeline produces a single deployable binary from one make target.
**Verified:** 2026-03-24T03:20:00Z
**Status:** passed
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| #  | Truth | Status | Evidence |
|----|-------|--------|----------|
| 1  | `make build-ui` produces a flowy-ui binary without errors | VERIFIED | `go build ./cmd/flowy-ui` exits 0; `./flowy-ui` binary exists at 9.5 MB (built 2026-03-24) |
| 2  | flowy-ui binary serves the embedded React SPA (index.html) on GET / | VERIFIED | `curl http://localhost:3000/` returns HTTP 200 with `<!doctype html>` |
| 3  | flowy-ui returns index.html for unknown paths like /projects (SPA fallback) | VERIFIED | `curl http://localhost:3000/projects` returns HTTP 200 with `<!doctype html>`; `curl http://localhost:3000/nonexistent/path` returns HTTP 200 |
| 4  | flowy-ui proxies /api/* requests to flowy-server with /api prefix stripped | VERIFIED | `curl http://localhost:3000/api/runs` returns HTTP 502 (proxy connection refused — correct: routes to flowy-server, not served as SPA) |
| 5  | All SPA assets (JS, CSS) load without 404 from the embedded filesystem | VERIFIED | `web/dist/assets/` contains `index-BawgE39P.css` and `index-DYuyJ5Po.js`; vite.config.ts `base: '/'` produces absolute asset paths |
| 6  | pnpm build in web/ produces a web/dist/ directory with index.html and JS/CSS assets | VERIFIED | `web/dist/` exists with `index.html` and `assets/` directory containing hashed JS and CSS |
| 7  | App shell renders a left sidebar with navigation items and a content area | VERIFIED | Sidebar.tsx has Registry/Runs/Members/Users/Account NavLinks with active state; AppShell.tsx renders Sidebar + Outlet; App.tsx contains catch-all Route path="*" |
| 8  | flowy-server accepts cross-origin requests from http://localhost:5173 when cors.enabled is true | VERIFIED | `cors.Handler` registered at top-level chi router before JWT route groups; `go test ./internal/server/...` exits 0 |
| 9  | flowy-server does not send CORS headers when cors.enabled is false (default) | VERIFIED | CORS block guarded by `if s.cfg.CORS.Enabled`; CORSConfig.Enabled defaults to false (zero value) |
| 10 | OPTIONS preflight requests receive correct headers before JWT middleware | VERIFIED | CORS middleware registered before `r.NotFound` and all `r.Group` JWT blocks; positioning confirmed in server.go lines 90-98 vs JWT at line 117+ |
| 11 | Makefile build-ui target builds frontend then Go binary in correct order | VERIFIED | `build-ui: web/dist` file-prerequisite enforces pnpm build before go build; `web/dist:` target runs `pnpm install --frozen-lockfile && pnpm build` |

**Score:** 11/11 truths verified

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd/flowy-ui/main.go` | flowy-ui entry point with spaHandler and ReverseProxy | VERIFIED | Contains `spaHandler`, `httputil.ReverseProxy` with `Rewrite:` (not deprecated Director), `fs.Sub(web.DistFS, "dist")`, startup `fs.Stat` assertion |
| `web/embed.go` | go:embed directive for web/dist/ assets | VERIFIED | `package web`, `//go:embed all:dist`, `var DistFS embed.FS` — all present, 7 lines |
| `Makefile` | build-ui target that builds frontend then Go binary | VERIFIED | `UI_BIN := ./flowy-ui`, `build-ui: web/dist`, `web/dist:` with pnpm, `clean` includes `$(UI_BIN)` and `rm -rf web/dist`, `.PHONY` includes `build-ui` |
| `web/package.json` | Frontend project manifest with all locked dependencies | VERIFIED | Contains `react`, `@tanstack/react-query`, `react-router`, `tailwindcss`, `@tailwindcss/vite` |
| `web/vite.config.ts` | Vite build config with Tailwind v4 plugin, path alias, API proxy | VERIFIED | `base: '/'` (corrected from `'./'`); proxy and tailwindcss plugin confirmed |
| `web/src/App.tsx` | React app root with BrowserRouter, QueryClientProvider, AppShell route | VERIFIED | BrowserRouter, QueryClientProvider, AppShell, Navigate redirect, catch-all `path="*"` route |
| `web/src/components/layout/AppShell.tsx` | Root layout with sidebar + content Outlet | VERIFIED | `flex h-screen overflow-hidden`, `<Sidebar />`, `<Outlet />`, `flex-1 overflow-y-auto p-6` |
| `web/src/components/layout/Sidebar.tsx` | Left sidebar navigation with project context and feature menu | VERIFIED | `w-60`, `bg-sidebar`, `border-r`, Registry/Runs/Members/Users/Account NavLinks with `isActive` state |
| `internal/config/config.go` | CORSConfig struct with Enabled and AllowedOrigins fields | VERIFIED | `type CORSConfig struct` with `Enabled bool yaml:"enabled"` and `AllowedOrigins []string yaml:"allowed_origins"`; `CORS CORSConfig yaml:"cors"` in Config |
| `internal/server/server.go` | cors.Handler middleware registered at top-level chi router before JWT | VERIFIED | Import `github.com/go-chi/cors`; `if s.cfg.CORS.Enabled` block at lines 90-98, before `r.NotFound` and JWT groups |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `web/embed.go` | `web/dist/` | `//go:embed all:dist` directive | WIRED | Directive present; `web/dist/` exists with assets; `go build ./cmd/flowy-ui` succeeds |
| `cmd/flowy-ui/main.go` | `web/embed.go` | `import github.com/bioflowy/flowy/web` + `web.DistFS` | WIRED | Import at line 15; `fs.Sub(web.DistFS, "dist")` at line 24 |
| `cmd/flowy-ui/main.go spaHandler` | `web/dist/index.html` | pre-flight `fsys.Open(path)` fallback | WIRED | `fsys.Open(path)` check at line 73; fallback sets `r2.URL.Path = "/"` — confirmed returning HTML for /projects |
| `cmd/flowy-ui/main.go proxy` | flowy-server | `httputil.ReverseProxy` with `Rewrite:` | WIRED | `ReverseProxy.Rewrite` strips `/api` prefix; `/api/` route registered; returns 502 when server unreachable (correct proxy behavior) |
| `Makefile build-ui` | `web/dist/` | `pnpm install --frozen-lockfile && pnpm build` | WIRED | File-prerequisite `web/dist: web/package.json ...` enforces correct ordering |
| `web/src/main.tsx` | `web/src/App.tsx` | `ReactDOM.createRoot` render | WIRED | `createRoot` with `<App />` in main.tsx |
| `web/src/App.tsx` | `web/src/components/layout/AppShell.tsx` | React Router `Route element` | WIRED | `<Route element={<AppShell />}>` at line 25 |
| `web/src/components/layout/AppShell.tsx` | `web/src/components/layout/Sidebar.tsx` | JSX `<Sidebar />` | WIRED | `import { Sidebar } from './Sidebar'`; `<Sidebar />` in JSX |
| `internal/server/server.go` | `internal/config/config.go` | `s.cfg.CORS.Enabled` check | WIRED | `s.cfg.CORS.Enabled` at line 90; `s.cfg.CORS.AllowedOrigins` at line 92 |
| `internal/server/server.go` | `github.com/go-chi/cors` | import and `cors.Handler` call | WIRED | Import at line 16; `cors.Handler(cors.Options{...})` at line 91 |

---

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| INFRA-01 | 25-03 | flowy-ui Go binary serves embedded React SPA via go:embed with SPA fallback routing | SATISFIED | `cmd/flowy-ui/main.go` + `web/embed.go` confirmed; SPA fallback verified via curl returning 200 with HTML for /projects and /nonexistent/path |
| INFRA-02 | 25-03 | flowy-ui reverse-proxies /api/* requests to flowy-server | SATISFIED | `mux.Handle("/api/", proxy)` with ReverseProxy.Rewrite stripping /api prefix; curl /api/runs returns 502 (routed to server, not SPA) |
| INFRA-03 | 25-01 | Vite + React 19 + TypeScript + shadcn/ui + Tailwind v4 project scaffold | SATISFIED | `web/` directory with full stack; `pnpm build` produces `web/dist/`; all tech versions confirmed in package.json |
| INFRA-04 | 25-02 | flowy-server supports config-gated CORS middleware for development mode | SATISFIED | CORSConfig in config.go; cors.Handler in server.go behind `if s.cfg.CORS.Enabled`; all server tests pass |
| INFRA-05 | 25-03 | Makefile `build-ui` target builds frontend and Go binary | SATISFIED | `build-ui: web/dist` file-prerequisite; `go build -o $(UI_BIN) ./cmd/flowy-ui`; clean target removes both binary and dist |

All 5 requirements accounted for. No orphaned requirements found in REQUIREMENTS.md for Phase 25.

Note: REQUIREMENTS.md still shows INFRA-01, INFRA-02, INFRA-05 as `Pending` with `[ ]` checkboxes. This is a documentation state tracking issue — the requirements are factually implemented and verified. The REQUIREMENTS.md tracking table was not updated after phase completion.

---

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `web/src/components/layout/Sidebar.tsx` | 25 | Comment `{/* Project selector placeholder */}` | Info | Expected — project selector is a known placeholder for Phase 27. The `div` renders "Select project" text which is intentional scaffolding, not a broken feature. Not a goal blocker. |

No blockers. No empty implementations, no `return null`, no stub handlers.

---

### Human Verification Required

The following items require human browser testing to fully confirm visual and interactive behavior. Automated curl tests confirmed HTTP responses are correct.

#### 1. Visual Sidebar Layout

**Test:** Run `./flowy-ui`, open browser at http://localhost:3000
**Expected:** Left sidebar 240px wide with "flowy" title, "Select project" placeholder, Registry/Runs/Members/Users/Account navigation items, main content area to the right
**Why human:** Visual rendering, CSS token application (bg-sidebar, bg-primary active state), and icon display cannot be verified programmatically

#### 2. Active Navigation State

**Test:** Click "Registry" nav item in sidebar
**Expected:** Item highlights with `bg-primary text-primary-foreground` styling; URL changes to /registry
**Why human:** NavLink isActive callback behavior and visual styling requires browser interaction

#### 3. Hard Refresh on Deep Path

**Test:** Navigate to http://localhost:3000/projects in browser; press F5
**Expected:** React app loads with sidebar visible (not 404 page)
**Why human:** Browser F5 is the real SPA routing test; curl confirmed SPA fallback returns HTML but browser rendering of the React Router catch-all route needs visual confirmation

---

### Gaps Summary

No gaps. All 11 observable truths verified. All 10 artifacts present, substantive, and wired. All 5 requirement IDs satisfied with evidence. All 6 commits from summaries confirmed in git history.

One minor documentation inconsistency: REQUIREMENTS.md tracking table still shows INFRA-01, INFRA-02, INFRA-05 as Pending (`[ ]`). This has no functional impact but should be corrected when the next phase updates STATE.md.

---

_Verified: 2026-03-24T03:20:00Z_
_Verifier: Claude (gsd-verifier)_
