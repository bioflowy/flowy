# Phase 25: Infrastructure Foundation - Research

**Researched:** 2026-03-20
**Domain:** Go binary (go:embed + reverse proxy) + React/TypeScript/Vite SPA scaffold + CORS middleware + Makefile pipeline
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**SPA Serving Architecture**
- `flowy-ui` is a separate Go binary in `cmd/flowy-ui/`
- Embeds the built SPA from `web/dist/` using `//go:embed all:web/dist`
- Uses `fs.Sub` to strip the embedded directory prefix
- SPA fallback handler: intercepts 404s from the file server and re-serves `index.html` (excluding `/api/*` paths which return JSON errors)
- Default port: `:3000` (configurable via flag)
- `--api-addr` flag specifies flowy-server address (default `http://localhost:8124`)

**API Proxy Design**
- UI accesses API via `/api/*` prefix (e.g., `/api/runs`, `/api/projects`)
- flowy-ui strips `/api/` prefix and proxies to flowy-server (which has no prefix)
- Uses `net/http/httputil.ReverseProxy` with `Rewrite` field (not deprecated `Director`)
- In production: same-origin proxy eliminates CORS entirely

**Development Workflow**
- Run both `flowy-ui` and Vite dev server simultaneously
- flowy-ui serves the Go binary + proxy; Vite provides HMR
- Vite's `server.proxy` also configured for `/api` → flowy-server as a fallback dev option
- CORS middleware on flowy-server: config-gated (`cors.enabled: true` in flowy.yaml), registered at top-level chi router before JWT middleware

**Layout Shell**
- Left sidebar navigation, GitHub-style (clean, information-dense)
- Sidebar contains: project selector at top, then feature menu (Registry, Runs, Members, etc.)
- Global items: Users (admin only), account menu at bottom
- Content area to the right of sidebar
- No mobile responsiveness required for v2.0

**Build Pipeline**
- Package manager: pnpm
- `make build-ui` target: runs `cd web && pnpm install && pnpm build` then `go build -o ./flowy-ui ./cmd/flowy-ui`
- Frontend build output: `web/dist/`
- Makefile integrates into existing build system alongside `build` target for Go binaries

**Project Structure**
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

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| INFRA-01 | flowy-ui Go binary serves embedded React SPA via go:embed with SPA fallback routing | go:embed + fs.Sub pattern, spaHandler 404-intercept, `//go:embed all:web/dist` |
| INFRA-02 | flowy-ui reverse-proxies /api/* requests to flowy-server | `net/http/httputil.ReverseProxy` with `Rewrite` field, prefix stripping |
| INFRA-03 | Vite + React 19 + TypeScript + shadcn/ui + Tailwind v4 project scaffold | Full frontend stack versions and install commands documented below |
| INFRA-04 | flowy-server supports config-gated CORS middleware for development mode | `go-chi/cors` at top-level chi router, `CORSConfig` in `internal/config/config.go` |
| INFRA-05 | Makefile `build-ui` target builds frontend and Go binary | pnpm-based build, dependency ordering, existing Makefile integration points |
</phase_requirements>

---

## Summary

Phase 25 creates the full infrastructure for the v2.0 Web UI: a new `flowy-ui` Go binary, a React SPA workspace, config-gated CORS on `flowy-server`, and a unified Makefile build target. All components are greenfield — neither `web/` nor `cmd/flowy-ui/` exist yet. The existing Go module at `github.com/bioflowy/flowy` (Go 1.25) will host the new binary; `go-chi/cors` is the only new Go dependency.

The architecture has two deployment modes. In production, `flowy-ui` serves the embedded SPA and proxies `/api/*` to `flowy-server` on the same origin — no CORS needed. In development, Vite dev server (`:5173`) serves the SPA with HMR while its `server.proxy` forwards `/api` to `flowy-server`; alternatively, `flowy-server` can have CORS enabled so the browser can reach it directly from Vite.

The critical implementation constraint is **build ordering**: `pnpm build` must complete before `go build` runs, because `//go:embed all:web/dist` requires `web/dist/` to exist at compile time. The Makefile `build-ui` target must enforce this sequence.

**Primary recommendation:** Implement in four sequential tasks: (1) `cmd/flowy-ui/main.go` with go:embed + spaHandler + ReverseProxy, (2) `web/` React+Vite scaffold with shadcn/ui init, (3) CORS on `flowy-server`, (4) `make build-ui` target + end-to-end smoke test.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `embed` | Go 1.25 (stable since 1.16) | Embed `web/dist/` into binary | Zero-dependency single-binary deployment |
| Go stdlib `net/http/httputil` | Go 1.25 | `ReverseProxy` for `/api/*` → flowy-server | Official stdlib; `Rewrite` field is the non-deprecated API since Go 1.20 |
| Go stdlib `io/fs` | Go 1.25 | `fs.Sub` to strip `web/dist/` prefix | Required to make embedded paths serve correctly |
| `github.com/go-chi/cors` | latest (v1.2.1) | CORS middleware on flowy-server | Official go-chi ecosystem package; integrates cleanly with existing chi v5 router |
| React | 19.x (19.2.4) | UI component model | Current stable; shadcn/ui and TanStack Query v5 both fully support React 19 |
| TypeScript | 5.x (5.9.3 stable) | Type safety | TS 6.0 is RC only — stay on 5.x |
| Vite | 6.x | Build tooling + dev server | shadcn/ui `init` targets Vite 6; Vite 8 (Rolldown) exists but shadcn CLI not yet updated |
| `@vitejs/plugin-react` | 4.x | React Fast Refresh + JSX transform | Official Vite 6 React plugin |
| Tailwind CSS | v4.x | Utility-first CSS | shadcn/ui fully supports Tailwind v4; CSS-first config (`@import "tailwindcss"`) |
| `@tailwindcss/vite` | v4.x | Tailwind v4 Vite plugin | Replaces postcss pipeline; add as `tailwindcss()` Vite plugin |
| shadcn/ui | CLI-managed | Copy-paste component library (Radix UI) | Not an npm package — CLI copies components into `src/components/ui/` |
| react-router | 7.x (7.13.1) | Client-side routing in library mode | v7 merges `react-router-dom`; use `<BrowserRouter>` + `<Routes>` |
| `@tanstack/react-query` | 5.x (5.91.x) | Server state (fetch, cache, refetch) | Standard for REST API data in React; locked decision |
| zustand | 5.x | Client-side auth token state | ~1.2 KB; locked decision for auth/session state |
| lucide-react | latest | Icons | shadcn/ui default icon library |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| sonner | latest | Toast notifications | shadcn/ui now prefers sonner over its own toast |
| clsx + tailwind-merge | latest | Conditional class composition | Auto-installed by `shadcn init`; provides the `cn()` utility |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| pnpm | npm | CONTEXT.md locks pnpm. npm creates `package-lock.json`; pnpm creates `pnpm-lock.yaml`. Do not mix. |
| plain fetch (locked) | axios | CONTEXT.md locks plain fetch. Axios has cleaner interceptors but adds 14 KB. |
| zustand (locked) | React Context | Context re-renders all consumers. Zustand selectors avoid unnecessary renders. |
| Vite 6 | Vite 8 | Vite 8 uses Rolldown; shadcn CLI not yet updated for it. Stay on 6. |
| React Router v7 library mode | TanStack Router | TanStack Router has better type-safe route params. React Router v7 has more shadcn examples. |

### Installation

```bash
# Create Vite + React + TypeScript project in web/
cd /repo
pnpm create vite@6 web -- --template react-ts
cd web
pnpm install

# Tailwind CSS v4 (Vite plugin — no postcss.config.js needed)
pnpm add -D tailwindcss @tailwindcss/vite

# TanStack Query v5
pnpm add @tanstack/react-query

# React Router v7 (library mode — single package, not react-router-dom)
pnpm add react-router

# State management
pnpm add zustand

# Icons (shadcn/ui dependency)
pnpm add lucide-react

# Toast notifications
pnpm add sonner

# Initialize shadcn/ui (interactive — choose default theme, CSS variables: yes)
pnpm dlx shadcn@latest init

# Add initial components for shell layout
pnpm dlx shadcn@latest add button separator tooltip
```

Go dependency:
```bash
go get github.com/go-chi/cors
```

---

## Architecture Patterns

### Recommended Project Structure

```
flowy/
├── cmd/
│   ├── flowy/              # existing CLI
│   ├── flowy-server/       # existing API server
│   ├── flowy-daemon/       # existing daemon
│   └── flowy-ui/           # NEW: SPA server binary
│       └── main.go         # embed directive + HTTP wiring
├── web/                    # NEW: React SPA workspace
│   ├── package.json
│   ├── pnpm-lock.yaml
│   ├── vite.config.ts      # proxy /api/* → flowy-server in dev
│   ├── tsconfig.json
│   ├── tsconfig.app.json
│   ├── tsconfig.node.json
│   ├── index.html
│   └── src/
│       ├── main.tsx        # React entry point
│       ├── App.tsx         # Router setup + QueryClientProvider
│       ├── index.css       # @import "tailwindcss" + shadcn CSS vars
│       ├── api/            # plain fetch functions per resource
│       │   └── client.ts   # base fetch, setToken, ApiError
│       ├── features/       # feature-based layout (locked decision)
│       │   ├── users/
│       │   ├── projects/
│       │   ├── registry/
│       │   └── runs/
│       ├── components/     # shared components
│       │   ├── ui/         # shadcn/ui generated (do not edit)
│       │   └── layout/     # AppShell, Sidebar, etc.
│       └── lib/
│           └── utils.ts    # cn() function from shadcn
├── internal/
│   ├── config/config.go    # MODIFIED: add CORSConfig
│   └── server/server.go    # MODIFIED: add cors middleware
└── go.mod                  # MODIFIED: add go-chi/cors
```

### Pattern 1: go:embed + fs.Sub for Embedded SPA

**What:** `//go:embed all:web/dist` embeds the entire Vite build output. `fs.Sub` creates a sub-filesystem rooted at `web/dist` so paths like `index.html` resolve correctly without the directory prefix.

**When to use:** Always in the production binary. The `all:` prefix is required to include dot-files (`.vite/` manifest files).

**Example:**
```go
// cmd/flowy-ui/main.go

package main

import (
    "embed"
    "flag"
    "io/fs"
    "log"
    "net/http"
    "net/http/httputil"
    "net/url"
    "strings"
)

//go:embed all:web/dist
var embeddedUI embed.FS

func main() {
    apiAddr := flag.String("api-addr", "http://localhost:8124", "address of flowy-server")
    listenAddr := flag.String("addr", ":3000", "address to listen on")
    flag.Parse()

    // Sub-filesystem: paths resolve as "index.html" not "web/dist/index.html"
    distFS, err := fs.Sub(embeddedUI, "web/dist")
    if err != nil {
        log.Fatalf("flowy-ui: embed sub: %v", err)
    }

    // Startup assertion: fail fast if the embed is misconfigured.
    if _, err := fs.Stat(distFS, "index.html"); err != nil {
        log.Fatalf("flowy-ui: embedded web/dist/index.html not found — run 'make build-ui' first")
    }

    apiBase, err := url.Parse(*apiAddr)
    if err != nil {
        log.Fatalf("flowy-ui: parse api-addr: %v", err)
    }

    mux := http.NewServeMux()

    // Proxy /api/ to flowy-server (strip /api prefix before forwarding).
    proxy := &httputil.ReverseProxy{
        Rewrite: func(r *httputil.ProxyRequest) {
            r.SetURL(apiBase)
            r.Out.URL.Path = strings.TrimPrefix(r.In.URL.Path, "/api")
            if r.Out.URL.Path == "" {
                r.Out.URL.Path = "/"
            }
            r.SetXForwarded()
        },
    }
    mux.Handle("/api/", proxy)

    // SPA: serve static assets; fallback to index.html for React Router paths.
    mux.Handle("/", spaHandler(distFS))

    log.Printf("flowy-ui: listening on %s (api: %s)", *listenAddr, *apiAddr)
    log.Fatal(http.ListenAndServe(*listenAddr, mux))
}

// spaHandler returns index.html for any path not found in the embedded FS.
// This is required for React Router client-side navigation on hard refresh.
func spaHandler(fsys fs.FS) http.Handler {
    fileServer := http.FileServer(http.FS(fsys))
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        path := strings.TrimPrefix(r.URL.Path, "/")
        if path == "" {
            path = "index.html"
        }
        if _, err := fsys.Open(path); err != nil {
            // Path not in embedded FS — serve index.html for React Router.
            r2 := r.Clone(r.Context())
            r2.URL.Path = "/"
            fileServer.ServeHTTP(w, r2)
            return
        }
        fileServer.ServeHTTP(w, r)
    })
}
```

### Pattern 2: CORS Config on flowy-server

**What:** Add `CORSConfig` to `internal/config/config.go` and register `go-chi/cors` as the first middleware in `newRouter()`, behind a config gate.

**When to use:** Development only (`cors.enabled: true` in flowy.yaml). Never in production.

**Example — config.go addition:**
```go
// CORSConfig holds CORS settings for the development workflow.
// In production, flowy-ui proxies /api/* on the same origin so CORS is not needed.
type CORSConfig struct {
    Enabled        bool     `yaml:"enabled"`
    AllowedOrigins []string `yaml:"allowed_origins"`
}

// Add to Config struct:
CORS CORSConfig `yaml:"cors"`
```

**Example — server.go newRouter() modification:**
```go
import "github.com/go-chi/cors"

func (s *Server) newRouter() *chi.Mux {
    r := chi.NewRouter()
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)

    // CORS: config-gated, dev-only. Must be top-level — before JWT middleware —
    // so that OPTIONS preflight requests are handled before auth rejects them.
    if s.cfg.CORS.Enabled {
        r.Use(cors.Handler(cors.Options{
            AllowedOrigins:   s.cfg.CORS.AllowedOrigins,
            AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
            AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
            AllowCredentials: false,
            MaxAge:           300,
        }))
    }

    // ... rest of router unchanged
```

**Example — flowy.yaml dev config:**
```yaml
cors:
  enabled: true
  allowed_origins:
    - "http://localhost:5173"
```

### Pattern 3: Vite Configuration for Dev Proxy

**What:** Vite's `server.proxy` forwards `/api` requests to flowy-server, stripping the prefix. The React SPA uses `/api/*` paths identically in dev and prod.

```typescript
// web/vite.config.ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
  ],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  base: './',   // relative asset paths — works at any mount point
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8124',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api/, ''),
      },
    },
  },
  build: {
    outDir: 'dist',
  },
})
```

### Pattern 4: Makefile build-ui Target

**What:** Sequential build: pnpm install → pnpm build → go build. The Go build step will fail with a clear error if `web/dist/` does not exist (the `//go:embed` directive enforces this).

```makefile
# Add to Makefile alongside existing build target

UI_BIN := ./flowy-ui

build-ui: web/dist
	go build -o $(UI_BIN) ./cmd/flowy-ui

web/dist: web/package.json web/pnpm-lock.yaml $(shell find web/src -name '*.ts' -o -name '*.tsx' -o -name '*.css' 2>/dev/null)
	cd web && pnpm install --frozen-lockfile && pnpm build

.PHONY: build-ui
```

Note: `--frozen-lockfile` prevents pnpm from updating `pnpm-lock.yaml` silently during CI builds.

### Pattern 5: React App Shell Structure

**What:** Minimal working app shell for Phase 25 — no feature screens, just the routing skeleton and layout shell.

```tsx
// web/src/App.tsx
import { BrowserRouter, Routes, Route, Navigate } from 'react-router'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AppShell } from '@/components/layout/AppShell'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: (failureCount, error) => {
        // Never retry auth errors — handled by auth redirect.
        if (error instanceof Error && 'status' in error) {
          const status = (error as { status: number }).status
          if (status === 401 || status === 403) return false
        }
        return failureCount < 2
      },
    },
  },
})

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route path="/" element={<AppShell />}>
            <Route index element={<Navigate to="/projects" replace />} />
            {/* Feature routes added in subsequent phases */}
          </Route>
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  )
}
```

### Anti-Patterns to Avoid

- **`//go:embed web/dist` without `all:`:** Silently excludes hidden files (`.vite/` manifest). Always use `all:web/dist`.
- **No `fs.Sub` call:** Paths resolve as `web/dist/index.html` instead of `index.html`. Always call `fs.Sub(embeddedUI, "web/dist")`.
- **CORS middleware inside a chi route group:** OPTIONS preflight never reaches middleware registered inside a group. Always register at top-level `r.Use(cors.Handler(...))`.
- **`AllowedOrigins: []string{"*"}` with credentials:** Browsers reject this combination per CORS spec. Use explicit origin list.
- **`ReverseProxy` with `Director` field:** `Director` does not clean incoming `X-Forwarded-*` headers (security issue). Use `Rewrite` field (Go 1.20+ recommendation).
- **Vite `base: "/"` (default):** Writes absolute asset paths into `index.html`. Assets 404 behind a sub-path proxy. Use `base: "./"`.
- **Running `go build ./cmd/flowy-ui` before `pnpm build`:** go build fails with `pattern all:web/dist: no matching files found`. Always build frontend first.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| CORS headers | Custom CORS middleware | `github.com/go-chi/cors` | Handles preflight, method filtering, credentials, MaxAge correctly. 15-line manual impl misses edge cases (preflight without auth header, wildcard + credentials interaction). |
| HTTP reverse proxy | Manual `http.Client` + response copy | `net/http/httputil.ReverseProxy` | Handles connection hijacking, streaming, hop-by-hop headers, error propagation. Manual proxy misses chunked encoding and WebSocket upgrades. |
| Static file caching | Custom Cache-Control headers | `http.FileServer` built-in | `http.FileServer` sends `Last-Modified` and handles `If-Modified-Since` automatically. |
| Component library | Custom Radix/accessible components | shadcn/ui CLI | ARIA, keyboard nav, focus management in accessible components is 1000+ lines of correct code per component. |

**Key insight:** The infrastructure layer (CORS, reverse proxy, file serving) is solved by stdlib and go-chi. The frontend component layer is solved by shadcn/ui. Phase 25 is wiring, not building.

---

## Common Pitfalls

### Pitfall 1: SPA Routing Falls Through to Go 404 on Browser Refresh

**What goes wrong:** User navigates to `/projects` via React Router, then presses F5. The browser sends a real HTTP GET `/projects` to `flowy-ui`. Go's file server finds no file at that path and returns a 404 response body before `spaHandler` can intercept it.

**Why it happens:** `http.FileServer` writes the 404 status and body before control returns to the wrapping handler. The naive approach of checking the response code after `fileServer.ServeHTTP(w, r)` is too late — headers are already sent.

**How to avoid:** Use the pre-flight `fsys.Open(path)` check pattern (shown in Pattern 1 above). Try to open the file before calling the file server; if not found, rewrite the URL to `"/"` and serve `index.html` instead. This avoids the ResponseWriter interception problem entirely.

**Warning signs:** `curl http://localhost:3000/projects` returns `{"error":"not found"}` or a plain 404 page, not HTML.

### Pitfall 2: go:embed Wrong Subtree — All Assets 404

**What goes wrong:** `//go:embed all:web/dist` embeds files at paths like `web/dist/index.html`. `http.FileServer(http.FS(embeddedUI))` looks for `index.html` at the root of the FS — but it is at `web/dist/index.html`. Every request returns 404.

**How to avoid:** Always call `fs.Sub(embeddedUI, "web/dist")` immediately after the embed declaration. Add the startup assertion `fs.Stat(distFS, "index.html")` to detect misconfiguration at binary start, not in production.

**Warning signs:** `http://localhost:3000/` returns a directory listing or 404 instead of the React app.

### Pitfall 3: CORS Middleware Inside a Route Group

**What goes wrong:** `go-chi/cors` is registered inside a route group (e.g., the JWT-authenticated group). OPTIONS preflight requests from the browser arrive before any auth header exists. The chi router routes the OPTIONS method to `NotFound` because no OPTIONS handler is registered in that group. The browser sees `404 No 'Access-Control-Allow-Origin' header` and blocks the actual request.

**How to avoid:** Register `cors.Handler` at the top of `newRouter()` before any `r.Group()` or `r.Route()` calls, and before JWT middleware. This is documented in go-chi/cors README.

**Warning signs:** Browser console shows `CORS policy: No 'Access-Control-Allow-Origin' header`. POST and PUT requests fail while GET requests succeed (because GET doesn't trigger preflight).

### Pitfall 4: pnpm install Modifies pnpm-lock.yaml in CI

**What goes wrong:** `pnpm install` (without flags) updates `pnpm-lock.yaml` if package versions in `package.json` have newer ranges. In CI, this can pull different versions than tested locally. The Makefile `build-ui` target should use `pnpm install --frozen-lockfile` for reproducibility.

**How to avoid:** Use `--frozen-lockfile` in the Makefile. For local development, `pnpm install` (without flag) is fine.

### Pitfall 5: `base: '/'` in Vite Config (Default)

**What goes wrong:** Vite defaults `base` to `"/"`, which writes absolute asset paths (`src="/assets/main-abc123.js"`) into `index.html`. If `flowy-ui` is ever served behind a path-prefixed proxy (e.g., nginx at `/ui/`), all asset requests 404.

**How to avoid:** Set `base: "./"` in `vite.config.ts` from the start. Relative paths work at any mount point.

### Pitfall 6: API Prefix Double-Stripping

**What goes wrong:** Both Vite's `server.proxy` `rewrite` function AND the `ReverseProxy.Rewrite` function strip `/api`. In dev mode through `flowy-ui`, the path gets stripped twice: `/api/projects` → `/projects` (ReverseProxy strips) → OK. But if someone configures Vite to also rewrite while routing through flowy-ui, the path becomes `` (empty).

**How to avoid:** Vite's proxy is used in dev mode (direct to flowy-server). flowy-ui's ReverseProxy is used in production. They are mutually exclusive. Verify with a smoke test: `curl http://localhost:3000/api/health` must return `{"status":"ok"}`, not 404 or 500.

---

## Code Examples

### go:embed with fs.Sub and Startup Assertion

```go
// Source: Go stdlib embed package documentation + fs.Sub pattern
//go:embed all:web/dist
var embeddedUI embed.FS

func mustDistFS() fs.FS {
    sub, err := fs.Sub(embeddedUI, "web/dist")
    if err != nil {
        log.Fatalf("flowy-ui: embed fs.Sub: %v", err)
    }
    // Fail fast if the frontend was not built before 'go build'.
    if _, err := fs.Stat(sub, "index.html"); err != nil {
        log.Fatalf("flowy-ui: web/dist/index.html missing — run 'cd web && pnpm build' first")
    }
    return sub
}
```

### ReverseProxy with Rewrite (non-deprecated API)

```go
// Source: https://pkg.go.dev/net/http/httputil#ReverseProxy — Rewrite field (Go 1.20+)
proxy := &httputil.ReverseProxy{
    Rewrite: func(r *httputil.ProxyRequest) {
        r.SetURL(apiBase)                                          // sets Host + scheme
        r.Out.URL.Path = strings.TrimPrefix(r.In.URL.Path, "/api")
        if r.Out.URL.Path == "" {
            r.Out.URL.Path = "/"
        }
        r.Out.URL.RawQuery = r.In.URL.RawQuery                    // preserve query params
        r.SetXForwarded()                                          // sets X-Forwarded-* safely
    },
}
```

### CORSConfig struct addition (config.go)

```go
// Source: go-chi/cors README + existing Config pattern in internal/config/config.go
type CORSConfig struct {
    Enabled        bool     `yaml:"enabled"`
    AllowedOrigins []string `yaml:"allowed_origins"`
}

// In Config struct — add after Auth field:
CORS CORSConfig `yaml:"cors"`
```

### shadcn/ui init CSS setup (Tailwind v4)

```css
/* web/src/index.css — generated by `pnpm dlx shadcn@latest init` */
@import "tailwindcss";

/* shadcn/ui CSS custom properties (OKLCH colors for Tailwind v4) */
@layer base {
  :root {
    --background: oklch(1 0 0);
    --foreground: oklch(0.145 0 0);
    /* ... shadcn init generates the full set */
  }
}
```

### Minimal AppShell Layout (sidebar + content)

```tsx
// web/src/components/layout/AppShell.tsx
import { Outlet } from 'react-router'
import { Sidebar } from './Sidebar'

export function AppShell() {
  return (
    <div className="flex h-screen overflow-hidden">
      <Sidebar />
      <main className="flex-1 overflow-y-auto p-6">
        <Outlet />
      </main>
    </div>
  )
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `postcss.config.js` + `tailwind.config.js` | CSS-first: `@import "tailwindcss"` in index.css | Tailwind v4 (early 2026) | No `tailwind.config.js` needed; `@tailwindcss/vite` plugin handles everything |
| `react-router-dom` + `react-router` as separate packages | Single `react-router` package | React Router v7 (Nov 2024) | Installing `react-router-dom` is redundant in v7 |
| `httputil.ReverseProxy.Director` | `httputil.ReverseProxy.Rewrite` | Go 1.20 (2023) | `Director` had security issues with X-Forwarded-* headers; `Rewrite` is the current API |
| shadcn/ui with Tailwind v3 HSL colors | OKLCH color space + CSS variables | Tailwind v4 migration (early 2026) | Better color interpolation; shadcn CLI auto-handles when detecting Tailwind v4 |

**Deprecated/outdated:**
- `Create React App (CRA)`: Deprecated 2023. Use `pnpm create vite@6`.
- `httputil.NewSingleHostReverseProxy`: Uses deprecated `Director` internally. Use `&httputil.ReverseProxy{Rewrite: ...}`.
- `react-router-dom`: Merged into `react-router` at v7. Do not install separately.

---

## Open Questions

1. **pnpm version constraint**
   - What we know: CONTEXT.md locks pnpm as the package manager
   - What's unclear: No specific pnpm version pinned. Vite 6 requires Node 18+.
   - Recommendation: Use whatever pnpm version is available on the dev machine; add `"packageManager": "pnpm@x.y.z"` to `package.json` once established.

2. **flowy-ui config file vs flags only**
   - What we know: CONTEXT.md marks this as Claude's discretion. `--api-addr` and `--addr` are confirmed flags.
   - What's unclear: Whether additional config (e.g., log level, TLS) needs YAML config file.
   - Recommendation: Flags-only for Phase 25. The binary is simple (two settings). Add YAML config in a later phase if the flag list grows.

3. **shadcn/ui init prompts**
   - What we know: `pnpm dlx shadcn@latest init` is interactive. The installer asks about style (Default/New York), base color, CSS variables.
   - What's unclear: Which style to pick — locked decision doesn't specify.
   - Recommendation: Use "Default" style with "slate" base color and CSS variables enabled. This matches the GitHub-style aesthetic requested in CONTEXT.md and can be changed trivially later.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go test (`go test ./...`) — existing; no Jest/Vitest for Phase 25 |
| Config file | none (existing `go test` infrastructure) |
| Quick run command | `go test ./cmd/flowy-ui/... ./internal/config/... ./internal/server/...` |
| Full suite command | `go test ./... -count=1` |

Note: Phase 25 creates infrastructure, not feature logic. The primary validation is a build smoke test (`make build-ui`) and manual browser verification, not unit tests. Go server-side changes (CORS config) have existing test infrastructure.

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| INFRA-01 | `GET /projects` returns HTML (index.html), not 404 | smoke | `go build ./cmd/flowy-ui && ./flowy-ui & curl -s http://localhost:3000/projects | head -1` | Wave 0 |
| INFRA-01 | F5 on `/projects` returns React app | manual | manual browser test | N/A |
| INFRA-02 | `/api/health` proxied to flowy-server | smoke | `curl -s http://localhost:3000/api/health` | Wave 0 |
| INFRA-03 | Vite dev server starts without errors | smoke | `cd web && pnpm dev --port 5173` | Wave 0 |
| INFRA-03 | Production build produces `web/dist/` | smoke | `cd web && pnpm build` | Wave 0 |
| INFRA-04 | OPTIONS preflight returns CORS headers | smoke | `curl -s -X OPTIONS -H "Origin: http://localhost:5173" http://localhost:8124/auth/login -v 2>&1 | grep Access-Control` | Wave 0 |
| INFRA-04 | CORS disabled by default (no `cors.enabled` in config) | unit | `go test ./internal/server/...` | Wave 0 |
| INFRA-05 | `make build-ui` completes without error | smoke | `make build-ui` | Wave 0 |
| INFRA-05 | Resulting `./flowy-ui` binary serves SPA | smoke | `./flowy-ui & sleep 1 && curl -s http://localhost:3000/ | grep -c "<!doctype html"` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/config/... ./internal/server/...` (fast, covers CORS config changes)
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green + `make build-ui` smoke test passes before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `cmd/flowy-ui/main.go` — does not exist yet; must be created as Wave 0 foundation
- [ ] `web/` directory — does not exist yet; Vite scaffold must be created
- [ ] `go-chi/cors` in `go.mod` — must be added with `go get github.com/go-chi/cors`
- [ ] `internal/config/config.go` — CORSConfig struct addition required
- [ ] `internal/server/server.go` — CORS middleware registration required

*(No existing test files need to be created — existing `go test ./...` infrastructure covers all new Go server changes once code exists.)*

---

## Sources

### Primary (HIGH confidence)
- Go stdlib `embed` package — `//go:embed all:` directive, `fs.Sub` requirement: https://pkg.go.dev/embed
- Go stdlib `net/http/httputil` — `ReverseProxy.Rewrite` field (Go 1.20+ API): https://pkg.go.dev/net/http/httputil#ReverseProxy
- go-chi/cors README — middleware placement, preflight handling, credentials + wildcard mutual exclusion: https://github.com/go-chi/cors
- Vite 6 docs — `base` option, `server.proxy`, `@tailwindcss/vite`: https://vite.dev/config/
- shadcn/ui Tailwind v4 docs — CSS-first config, OKLCH colors: https://ui.shadcn.com/docs/tailwind-v4
- shadcn/ui React 19 docs — full compatibility confirmed: https://ui.shadcn.com/docs/react-19
- React Router v7 library mode — `<BrowserRouter>` + `<Routes>` usage: https://reactrouter.com/how-to/spa
- Codebase analysis: `internal/server/server.go`, `internal/config/config.go`, `cmd/flowy-server/main.go`, `Makefile` — existing patterns (HIGH, direct inspection)

### Secondary (MEDIUM confidence)
- Serving SPA from Go with 404 fallback: https://hackandsla.sh/posts/2021-11-06-serve-spa-from-go/
- Embedding React app in Go binary pattern: https://www.smartinary.com/blog/how-to-embed-a-react-app-in-a-go-binary/

### Tertiary (LOW confidence)
- None — all critical claims verified against official docs or direct source inspection.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all versions verified against official sources in prior research (STACK.md, researched 2026-03-20)
- Architecture: HIGH — go:embed + ReverseProxy patterns from official Go stdlib docs; CORS pattern from go-chi/cors README
- Pitfalls: HIGH — verified against official docs and PITFALLS.md (researched 2026-03-20) which cross-referenced Go, Vite, and go-chi sources
- Makefile integration: HIGH — existing Makefile read directly; pnpm decision from CONTEXT.md

**Research date:** 2026-03-20
**Valid until:** 2026-04-20 (stable stack — Vite/shadcn/ui update cadence is the main risk)

**Key discrepancy noted:** `STACK.md` recommends `npm` while `CONTEXT.md` (locked decisions) specifies `pnpm`. CONTEXT.md takes precedence — all install commands use `pnpm`.
