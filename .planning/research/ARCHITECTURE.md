# Architecture Research

**Domain:** React SPA + Go REST API integration — flowy v2.0 Web UI
**Researched:** 2026-03-20
**Confidence:** HIGH

---

## Context: What Is Being Added

flowy ships three Go binaries (flowy, flowy-server, flowy-daemon). v2.0 adds a fourth binary,
`flowy-ui`, that:

1. Embeds a compiled React SPA (Vite build output) via `go:embed`
2. Serves the SPA over HTTP (static files + index.html fallback for client-side routing)
3. Proxies all `/api/*` requests to a running `flowy-server` instance
4. Supports a dev mode where Vite dev server handles hot reload, proxying API to `flowy-server`

The React SPA communicates exclusively with `flowy-server` for all data. `flowy-ui` is a pure
serving layer — it knows nothing about the domain; it only serves files and forwards traffic.

---

## Standard Architecture

### System Overview

```
┌──────────────────────────────────────────────────────────────────────────┐
│  Browser                                                                  │
│  React SPA (React Router, TanStack Query, shadcn/ui, Tailwind CSS)       │
│  Auth context: JWT stored in memory (React state) + sessionStorage        │
└────────────────────────┬─────────────────────────────────────────────────┘
                         │ HTTP
         ┌───────────────┴──────────────────┐
         │ PRODUCTION                        │ DEV (vite dev server :5173)
         ▼                                   ▼
┌────────────────────┐             ┌──────────────────────┐
│  flowy-ui binary   │             │  Vite dev server      │
│  :8080             │             │  serves React files   │
│  ┌──────────────┐  │             │  server.proxy:        │
│  │ SPA handler  │  │             │  /api/* → :8124       │
│  │ go:embed     │  │             └──────────┬───────────┘
│  │ index.html   │  │                        │
│  │ fallback     │  │   ┌────────────────────┘
│  └──────┬───────┘  │   │
│         │ /api/*   │   │ /api/* (with Authorization: Bearer <token>)
│  ┌──────▼───────┐  │   │
│  │ ReverseProxy │  │   │
│  │ httputil     │  │   │
│  └──────┬───────┘  │   │
└─────────│──────────┘   │
          └──────────────┘
                         │
                         ▼
          ┌──────────────────────────────┐
          │  flowy-server  :8124          │
          │  chi v5 router               │
          │  JWT middleware              │
          │  CORS middleware (dev only)  │
          │  REST API:                   │
          │    POST /auth/login          │
          │    GET  /users/me            │
          │    GET  /projects            │
          │    GET  /projects/{slug}/... │
          │    GET  /projects/{slug}/    │
          │         registry/...         │
          │    GET  /projects/{slug}/    │
          │         provenance/...       │
          └──────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Implementation |
|-----------|----------------|----------------|
| `flowy-ui` binary | Serve embedded SPA + reverse proxy `/api/*` to flowy-server | `cmd/flowy-ui/main.go`, `net/http`, `httputil.ReverseProxy` |
| SPA handler | Serve static assets; return `index.html` for unmatched paths | `fs.Sub(embeddedFS, "ui/dist")` + 404-intercept pattern |
| Reverse proxy handler | Strip `/api` prefix, forward to `flowy-server`, copy response | `httputil.ReverseProxy` with `Rewrite` function |
| React SPA | UI rendering, client-side routing, API calls, auth state | React 19, React Router, TanStack Query |
| API client layer | Typed fetch wrappers, JWT header injection, error normalization | `ui/src/api/` — plain `fetch` or `axios`, organized by resource |
| Auth context | In-memory JWT storage, login/logout, token injection | React context + `sessionStorage` for refresh-safe persistence |
| TanStack Query | Server state: caching, background refetch, loading/error states | `useQuery` / `useMutation` per resource |
| `flowy-server` CORS | Allow `http://localhost:5173` in dev; no CORS needed in prod | `go-chi/cors` middleware, config-gated |

---

## Recommended Project Structure

```
flowy/
├── cmd/
│   ├── flowy/                  # existing client CLI
│   ├── flowy-server/           # existing API server
│   ├── flowy-daemon/           # existing task daemon
│   └── flowy-ui/               # NEW: SPA server binary
│       └── main.go             # embed directive + HTTP wiring
├── ui/                         # NEW: React SPA workspace
│   ├── package.json
│   ├── vite.config.ts          # proxy /api/* → flowy-server in dev
│   ├── tsconfig.json
│   ├── index.html
│   └── src/
│       ├── main.tsx            # React entry point
│       ├── App.tsx             # Router setup
│       ├── api/                # API client layer
│       │   ├── client.ts       # base fetch wrapper, token injection
│       │   ├── auth.ts         # login, me
│       │   ├── users.ts        # user CRUD
│       │   ├── projects.ts     # project + member endpoints
│       │   ├── registry.ts     # registry list/pull endpoints
│       │   ├── runs.ts         # run status, outputs
│       │   └── provenance.ts   # provenance list/get
│       ├── hooks/              # TanStack Query custom hooks
│       │   ├── useAuth.ts      # login mutation, me query
│       │   ├── useProjects.ts  # useQuery wrappers for projects
│       │   ├── useRegistry.ts  # registry query hooks
│       │   └── useProvenance.ts
│       ├── context/
│       │   └── AuthContext.tsx  # JWT token state, login/logout
│       ├── components/
│       │   ├── ui/             # shadcn/ui generated components (do not edit)
│       │   └── app/            # application-specific components
│       ├── pages/              # route-level components
│       │   ├── LoginPage.tsx
│       │   ├── UsersPage.tsx
│       │   ├── ProjectsPage.tsx
│       │   ├── RegistryPage.tsx
│       │   ├── FileBrowserPage.tsx
│       │   └── ProvenancePage.tsx
│       └── lib/
│           └── utils.ts        # shadcn/ui utility (cn function)
├── internal/                   # existing Go packages
└── go.mod                      # Go module (flowy-ui binary uses same module)
```

### Structure Rationale

- **`cmd/flowy-ui/`:** Follows the established flowy pattern — one directory per binary. The `//go:embed` directive lives here and points to `../../ui/dist`.
- **`ui/`:** Keeps the entire frontend workspace at repo root level, separate from Go packages. `go:embed` spans the boundary at build time.
- **`ui/src/api/`:** One file per resource group mirrors the server-side handler files. This makes it trivial to find the client counterpart of any server endpoint.
- **`ui/src/hooks/`:** TanStack Query hooks are separated from pure API functions. API functions are plain async functions (testable without React); hooks are the React integration layer.
- **`ui/src/context/`:** Single auth context owns JWT state. All other components read from it — no token logic scattered across components.
- **`ui/src/components/ui/`:** shadcn/ui puts generated components here by convention. These files are owned by shadcn/ui tooling and must not be hand-edited.

---

## Architectural Patterns

### Pattern 1: go:embed + Sub Filesystem for SPA

**What:** The `flowy-ui` binary embeds the entire `ui/dist/` directory at compile time using
`//go:embed`. `fs.Sub` creates a sub-filesystem so paths like `index.html` resolve correctly
without the `ui/dist/` prefix. A 404-intercept pattern returns `index.html` for any path not
found in the embedded FS — this is required for client-side React Router to work on hard refresh
or direct URL navigation.

**When to use:** Always for the production binary. The dev mode bypasses this entirely (Vite
serves files directly).

**Trade-offs:** Binary size grows by the size of the compiled SPA (typically 1-5 MB for a
Tailwind + shadcn/ui app, which is negligible). The binary must be rebuilt when the SPA changes.
This is acceptable because the build pipeline (`make build-ui`) generates the `ui/dist/` before
Go compilation runs.

**Example:**

```go
// cmd/flowy-ui/main.go

package main

import (
    "embed"
    "io/fs"
    "log"
    "net/http"
    "net/http/httputil"
    "net/url"
    "strings"
)

//go:embed all:ui/dist
var uiFS embed.FS

func main() {
    serverURL := "http://localhost:8124" // from config/env
    apiBase, _ := url.Parse(serverURL)

    // Sub-filesystem: paths resolve as "index.html" not "ui/dist/index.html"
    distFS, err := fs.Sub(uiFS, "ui/dist")
    if err != nil {
        log.Fatal(err)
    }

    mux := http.NewServeMux()

    // Proxy /api/* to flowy-server (strip /api prefix)
    proxy := &httputil.ReverseProxy{
        Rewrite: func(r *httputil.ProxyRequest) {
            r.SetURL(apiBase)
            r.Out.URL.Path = strings.TrimPrefix(r.In.URL.Path, "/api")
            r.SetXForwarded()
        },
    }
    mux.Handle("/api/", proxy)

    // SPA: serve static assets, fall back to index.html for unknown paths
    mux.Handle("/", spaHandler(distFS))

    log.Fatal(http.ListenAndServe(":8080", mux))
}

// spaHandler returns index.html for any path not found in the embedded FS.
// This is required for React Router client-side navigation to work on reload.
func spaHandler(fsys fs.FS) http.Handler {
    fileServer := http.FileServer(http.FS(fsys))
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Try to open the file from the embedded FS.
        path := strings.TrimPrefix(r.URL.Path, "/")
        if path == "" {
            path = "index.html"
        }
        _, err := fsys.Open(path)
        if err != nil {
            // Not found in FS: serve index.html for client-side routing.
            r2 := *r
            r2.URL.Path = "/"
            fileServer.ServeHTTP(w, &r2)
            return
        }
        fileServer.ServeHTTP(w, r)
    })
}
```

### Pattern 2: Vite server.proxy for Development Mode

**What:** During development, Vite's built-in dev server handles hot module replacement (HMR)
and proxies API calls to `flowy-server`. The React SPA code makes all API calls to `/api/*`
without knowing whether it is in dev or prod mode. In dev, Vite intercepts those calls and
forwards them; in prod, `flowy-ui` intercepts them via the reverse proxy.

**When to use:** Always during frontend development. This eliminates the need to run `flowy-ui`
during development and avoids rebuilding the Go binary on every UI change.

**Trade-offs:** Vite dev server only runs during development — it does not affect the production
build. The proxy is configured once in `vite.config.ts` and matches the production routing scheme
exactly.

**Example:**

```typescript
// ui/vite.config.ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
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
})
```

With this configuration, `fetch('/api/projects')` in the React SPA becomes
`http://localhost:8124/projects` at the `flowy-server`. The path rewrite strips the `/api`
prefix, matching the server's actual route scheme exactly.

### Pattern 3: Centralized API Client with Token Injection

**What:** A single `api/client.ts` module owns all fetch logic: base URL, token injection,
error normalization. Resource-specific modules (`api/projects.ts`, `api/registry.ts`) call
`client.get<T>()` / `client.post<T>()` — they never call `fetch` directly and never read from
the auth context directly. Token injection happens in the base client layer via an exported
`setToken(t: string)` function that the auth context calls on login.

**When to use:** Always. This is the standard pattern for any SPA with a REST backend.

**Trade-offs:** Module-level token state (not React context) is used to avoid React dependency
in the API layer. This makes the API functions testable without a React context wrapper and
avoids the need to pass token through every call site.

**Example:**

```typescript
// ui/src/api/client.ts

const BASE = '/api'   // same in dev (Vite proxy) and prod (flowy-ui proxy)

let _token: string | null = null

export function setToken(token: string | null) {
  _token = token
  if (token) {
    sessionStorage.setItem('flowy_token', token)
  } else {
    sessionStorage.removeItem('flowy_token')
  }
}

export function loadSavedToken(): string | null {
  return sessionStorage.getItem('flowy_token')
}

type Method = 'GET' | 'POST' | 'PUT' | 'DELETE'

export async function request<T>(
  method: Method,
  path: string,
  body?: unknown
): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }
  if (_token) {
    headers['Authorization'] = `Bearer ${_token}`
  }

  const res = await fetch(`${BASE}${path}`, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new ApiError(res.status, err.error ?? res.statusText)
  }

  return res.json() as Promise<T>
}

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message)
  }
}

export const get = <T>(path: string) => request<T>('GET', path)
export const post = <T>(path: string, body: unknown) => request<T>('POST', path, body)
export const put = <T>(path: string, body: unknown) => request<T>('PUT', path, body)
export const del = <T>(path: string) => request<T>('DELETE', path)
```

```typescript
// ui/src/api/projects.ts
import { get, post } from './client'

export interface Project {
  id: string
  slug: string
  description: string
  is_active: boolean
  created_at: string
  updated_at: string
}

export const listProjects = () => get<Project[]>('/projects')
export const getProject = (slug: string) => get<Project>(`/projects/${slug}`)
export const createProject = (slug: string, description: string) =>
  post<Project>('/projects', { slug, description })
```

### Pattern 4: JWT Token in sessionStorage (pragmatic choice)

**What:** The JWT token is stored in `sessionStorage` and also held in module-level state in
`api/client.ts`. On app load, `AuthContext` calls `loadSavedToken()` and restores the token from
`sessionStorage`. This survives page refresh within the same browser tab but clears on tab close.

**When to use:** This is the pragmatic choice for an internal management tool that uses JWT
issued by `flowy-server` (7-day TTL). It is not the most secure option for a public-facing app
but is appropriate for this domain.

**Trade-offs:**

- `sessionStorage` is vulnerable to XSS (same as `localStorage`). However, the flowy UI serves
  no user-generated content and has no third-party scripts, making XSS risk minimal.
- HttpOnly cookies are more secure but require the `flowy-ui` proxy to handle cookie setting,
  complicating the architecture. This is out of scope for v2.0.
- `sessionStorage` vs `localStorage`: sessionStorage is preferred because tokens clear on tab
  close, reducing the window for token theft.

The server already issues 7-day JWTs (`IssueToken` in `internal/auth/auth.go`). There is no
refresh token mechanism. The UI should show a "session expired" screen when the server returns
401, prompting re-login.

### Pattern 5: Auth Context with Protected Routes

**What:** A `AuthContext` React context provides `{token, user, login, logout}` to all
components. `login()` calls `POST /api/auth/login`, stores the token via `setToken()`, and
fetches `/api/users/me` to populate `user`. Protected routes check `token !== null` and redirect
to `/login` if not authenticated.

**Example:**

```typescript
// ui/src/context/AuthContext.tsx
import { createContext, useContext, useState, useEffect, ReactNode } from 'react'
import { setToken, loadSavedToken, get, post } from '@/api/client'
import type { User } from '@/api/auth'

interface AuthState {
  token: string | null
  user: User | null
  login: (username: string, password: string) => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setTokenState] = useState<string | null>(null)
  const [user, setUser] = useState<User | null>(null)

  useEffect(() => {
    const saved = loadSavedToken()
    if (saved) {
      setToken(saved)
      setTokenState(saved)
      get<User>('/users/me').then(setUser).catch(() => logout())
    }
  }, [])

  async function login(username: string, password: string) {
    const { token: t } = await post<{ token: string }>(
      '/auth/login',
      { username, password }
    )
    setToken(t)
    setTokenState(t)
    const me = await get<User>('/users/me')
    setUser(me)
  }

  function logout() {
    setToken(null)
    setTokenState(null)
    setUser(null)
  }

  return (
    <AuthContext.Provider value={{ token, user, login, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export const useAuth = () => {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
```

---

## Data Flow

### Login Flow

```
LoginPage: user submits form
    ↓
AuthContext.login(username, password)
    ↓
POST /api/auth/login  →  [Vite proxy or flowy-ui proxy]  →  POST /auth/login @ flowy-server
    ↓
flowy-server: CheckPassword → IssueToken → { token: "eyJ..." }
    ↓
AuthContext: setToken(t) → sessionStorage + module-level state
    ↓
GET /api/users/me → flowy-server → { id, username, global_role, ... }
    ↓
AuthContext: setUser(me)
    ↓
React Router: navigate("/projects")
```

### Authenticated Data Fetch Flow

```
ProjectsPage mounts
    ↓
useProjects() hook: useQuery({ queryKey: ['projects'], queryFn: listProjects })
    ↓
TanStack Query: calls listProjects() if cache is stale
    ↓
listProjects() → get('/projects') → fetch('/api/projects', { Authorization: Bearer <token> })
    ↓
[Vite proxy or flowy-ui proxy] → GET /projects @ flowy-server
    ↓
flowy-server: JWTMiddleware validates token → handleListProjects
    ↓
JSON response → TanStack Query cache → React re-renders ProjectsPage
```

### 401 Handling Flow

```
Any API call → 401 Unauthorized
    ↓
ApiError(401, "invalid or expired token")
    ↓
TanStack Query: error state
    ↓
Global error handler in QueryClient (onError) checks status === 401
    ↓
AuthContext.logout()  →  navigate("/login")
```

### State Management

```
[AuthContext] — owns token + user identity (React state + sessionStorage)
    ↓ (token injected into module-level client.ts)
[TanStack Query] — owns all server state (projects, users, registry, etc.)
    ↓ (useQuery / useMutation per resource)
[Components] — read from TanStack Query, write via mutations
    ↓ (invalidate queries after mutations to trigger refetch)
```

No Zustand, Redux, or Jotai is needed. TanStack Query handles all server state; React context
handles auth state. Component-local state handles UI-only state (form values, modal open/closed).

---

## CORS Configuration on flowy-server

### When CORS Is Needed

CORS headers are only required when the browser's origin (Vite dev server at `:5173`) differs
from the API origin (flowy-server at `:8124`). In production, `flowy-ui` serves both the SPA
and the proxied API on the same origin (`:8080`), so no CORS is needed.

CORS must be config-gated so it is not active in production by accident.

### Implementation

Add `go-chi/cors` as a new dependency to `flowy-server`. CORS middleware must be registered
as the first middleware on the chi router (before JWT middleware).

```go
// internal/server/server.go — modified newRouter()

import "github.com/go-chi/cors"

func (s *Server) newRouter() *chi.Mux {
    r := chi.NewRouter()
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)

    // CORS: only active when cfg.CORS.Enabled = true (dev mode).
    // Must be first middleware so OPTIONS preflight requests are handled
    // before JWTMiddleware rejects them as unauthenticated.
    if s.cfg.CORS.Enabled {
        r.Use(cors.Handler(cors.Options{
            AllowedOrigins:   s.cfg.CORS.AllowedOrigins, // ["http://localhost:5173"]
            AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
            AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
            AllowCredentials: false,
            MaxAge:           300,
        }))
    }

    // ... rest of router unchanged
}
```

Add to `config.Config`:

```go
type CORSConfig struct {
    Enabled        bool     `yaml:"enabled"`
    AllowedOrigins []string `yaml:"allowed_origins"`
}

type Config struct {
    // ... existing fields ...
    CORS CORSConfig `yaml:"cors"`
}
```

Example `flowy.yaml` for development:

```yaml
cors:
  enabled: true
  allowed_origins:
    - "http://localhost:5173"
```

---

## Integration Points

### New vs. Modified Components

| Component | Status | What Changes |
|-----------|--------|--------------|
| `cmd/flowy-ui/main.go` | NEW | `//go:embed`, `spaHandler`, `ReverseProxy`, HTTP listen |
| `ui/` directory | NEW | Full React SPA workspace (Vite, React, TanStack Query, shadcn/ui) |
| `internal/server/server.go` | MODIFIED | Add CORS middleware (config-gated); no other changes |
| `internal/config/config.go` | MODIFIED | Add `CORSConfig` struct |
| `go.mod` | MODIFIED | Add `github.com/go-chi/cors` |
| All existing handlers | UNCHANGED | No handler changes needed |
| `flowy-server` routing | UNCHANGED | No route changes; existing `/auth/login`, `/projects/*` etc. work as-is |
| `flowy-daemon` | UNCHANGED | Not involved in UI |
| `flowy` CLI | UNCHANGED | Not involved in UI |

### External Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| Browser SPA ↔ flowy-ui | HTTP (static assets + `/api/*` proxy) | Same origin in prod; no CORS needed |
| Browser SPA ↔ Vite dev server | HTTP (HMR + `/api/*` proxy) | Dev only; Vite forwards to flowy-server |
| flowy-ui ↔ flowy-server | HTTP reverse proxy | `ReverseProxy.Rewrite` strips `/api` prefix |
| Vite dev server ↔ flowy-server | HTTP proxy | `server.proxy` in vite.config.ts strips `/api` |
| flowy-server CORS ↔ browser | preflight + response headers | Only when `cors.enabled: true` |

### Build Pipeline Integration

```
make build-ui:
  cd ui && npm run build     # outputs ui/dist/
  go build ./cmd/flowy-ui/   # go:embed picks up ui/dist/

make dev-ui:
  # Terminal 1: flowy-server (with cors.enabled: true)
  go run ./cmd/flowy-server/ --config flowy-dev.yaml
  # Terminal 2: Vite dev server
  cd ui && npm run dev
```

The `//go:embed all:ui/dist` directive requires `ui/dist/` to exist at Go compile time.
If it does not exist, `go build` fails with a clear error. This prevents accidentally shipping
a binary with a missing UI.

---

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| Single operator (lab server) | Single `flowy-ui` + single `flowy-server` on same host; no load balancing needed |
| Team (10-50 users) | Same architecture; SQLite WAL handles concurrent reads; `flowy-server` is the bottleneck before UI |
| Larger deployment | Run `flowy-server` behind nginx/caddy; `flowy-ui` stays unchanged (no state) |

The UI binary is stateless — it holds no session state and no database connection. Horizontal
scaling of `flowy-ui` is trivial (run multiple instances behind a load balancer). `flowy-server`
is the stateful component.

### Scaling Priorities

1. **First bottleneck:** SQLite write lock under concurrent workflow submissions — not UI-related;
   addressed by existing WAL + atomic task claiming architecture.
2. **Second bottleneck:** `flowy-server` CPU for JWT validation — stateless; can add multiple
   `flowy-server` instances with a shared SQLite file on a network filesystem (NFS/EFS).

---

## Anti-Patterns

### Anti-Pattern 1: Reading Token from AuthContext in API Functions

**What people do:** Import `useAuth()` inside `api/projects.ts` to get the token for each call.

**Why it's wrong:** API functions (`api/*.ts`) must be plain async functions, not React hooks.
Calling a hook outside a component violates React's hook rules and prevents testing API
functions outside React.

**Do this instead:** Store token in module-level state in `client.ts` via `setToken()`. The auth
context calls `setToken()` once on login. All subsequent API calls pick up the token automatically
without any React dependency in the API layer.

### Anti-Pattern 2: Wildcard CORS in Production

**What people do:** Set `AllowedOrigins: []string{"*"}` and leave `cors.enabled: true` in the
production config.

**Why it's wrong:** Any origin can make credentialed requests to flowy-server, bypassing the
same-origin protection that `flowy-ui` provides for free in production.

**Do this instead:** `cors.enabled: false` in production configs (the default). CORS middleware
is a dev-only feature. The `flowy-ui` reverse proxy makes CORS irrelevant in production because
the SPA and the API share the same origin.

### Anti-Pattern 3: Using Director Instead of Rewrite in ReverseProxy

**What people do:** Use `httputil.NewSingleHostReverseProxy(target)` which uses the deprecated
`Director` internally, then try to modify the request after the fact.

**Why it's wrong:** `Director` has documented security issues: it does not clean incoming
X-Forwarded-* headers from clients, allowing header spoofing. `NewSingleHostReverseProxy` also
does not rewrite the Host header, which can confuse some backends.

**Do this instead:** Use `&httputil.ReverseProxy{Rewrite: func(r *httputil.ProxyRequest) {...}}`
with `r.SetURL(target)` and `r.SetXForwarded()`. This is the current Go stdlib recommendation
as of Go 1.20+.

### Anti-Pattern 4: Fetching Token from sessionStorage in Every API Call

**What people do:** Read `sessionStorage.getItem('flowy_token')` inside every fetch call.

**Why it's wrong:** `sessionStorage` access is synchronous and slow if called in a tight loop.
More importantly, it decentralizes token management — if the storage key changes, every call
site breaks.

**Do this instead:** Load from `sessionStorage` once on app startup in `AuthContext.useEffect`.
Thereafter, token is in module-level state in `client.ts`. Zero storage reads after init.

### Anti-Pattern 5: Serving API Routes Under the SPA Handler

**What people do:** Register both the SPA catch-all and API routes on the same `http.ServeMux`
but in wrong order, causing API routes to be shadowed by the SPA catch-all.

**Why it's wrong:** `http.ServeMux` applies longest-prefix matching. If `"/"` is registered, it
matches everything including `/api/...` unless `/api/` is also registered with higher specificity.

**Do this instead:** Register `/api/` before the SPA catch-all `/`. In Go's `http.ServeMux`,
`"/api/"` (with trailing slash) is more specific than `"/"` and takes precedence. Explicitly
verify by testing that `GET /api/health` returns JSON, not HTML.

### Anti-Pattern 6: Storing JWT in localStorage

**What people do:** Use `localStorage` because it persists across browser restarts.

**Why it's wrong:** `localStorage` is accessible from any JavaScript on the page. While XSS
risk is low for this internal tool, using `sessionStorage` costs nothing and reduces the attack
window by clearing on tab close.

**Do this instead:** `sessionStorage`. Tokens clear when the tab closes. For users who want
persistence, the 7-day token TTL means they must re-login at most once per week anyway.

---

## Build Order for v2.0 Phases

Dependencies between components drive this order:

```
Step 1: Go binary scaffold (flowy-ui)
  cmd/flowy-ui/main.go — embed directive, SPA handler, ReverseProxy stub
  Serves static "coming soon" page, forwards /api/* to flowy-server
  Gate: binary builds and serves index.html

Step 2: React SPA scaffold
  ui/ — Vite init, React Router, TanStack Query, shadcn/ui, Tailwind CSS
  ui/src/api/client.ts — base client, setToken, ApiError
  ui/src/context/AuthContext.tsx — token state, login, logout
  Gate: npm run dev starts, /api/health returns JSON via proxy

Step 3: CORS on flowy-server + Login page
  internal/config/config.go — CORSConfig
  internal/server/server.go — cors middleware (config-gated)
  go.mod — add go-chi/cors
  ui/src/pages/LoginPage.tsx — login form
  Gate: login form calls POST /auth/login, receives JWT, stores in sessionStorage

Step 4: User management UI
  ui/src/api/users.ts + hooks/useUsers.ts
  ui/src/pages/UsersPage.tsx
  Gate: admin can list users, create user, toggle active

Step 5: Project management UI
  ui/src/api/projects.ts + hooks/useProjects.ts
  ui/src/pages/ProjectsPage.tsx (list + create)
  ui/src/pages/ProjectDetailPage.tsx (members, roles)
  Gate: create project, add members, change roles

Step 6: Registry browsing UI
  ui/src/api/registry.ts + hooks/useRegistry.ts
  ui/src/pages/RegistryPage.tsx
  Gate: list entries, view versions, view tags

Step 7: File browsing and provenance UI
  ui/src/api/provenance.ts + hooks/useProvenance.ts
  ui/src/pages/ProvenancePage.tsx
  ui/src/pages/FileBrowserPage.tsx
  Gate: view provenance records, browse output files with download links

Step 8: go:embed integration + production build
  cmd/flowy-ui/main.go — switch from stub to embedded ui/dist/
  Makefile — build-ui target, pre-build check
  Gate: make build-ui produces a single binary that serves the full SPA
```

Steps 1-2 are the foundation; Steps 3-8 are independent features that can be worked in
parallel once the scaffold exists. Step 8 is integration validation — it must be done before
any phase is considered complete for production delivery.

---

## Sources

- Go stdlib `net/http/httputil` — ReverseProxy.Rewrite pattern: https://pkg.go.dev/net/http/httputil#ReverseProxy (HIGH confidence, official Go documentation)
- go-chi/cors documentation: https://pkg.go.dev/github.com/go-chi/cors (HIGH confidence, official package docs)
- Vite server.proxy configuration: https://vite.dev/config/server-options (HIGH confidence, official Vite docs)
- Serving SPA from Go with 404 fallback pattern: https://hackandsla.sh/posts/2021-11-06-serve-spa-from-go/ (MEDIUM confidence, well-regarded community post; pattern verified against stdlib behavior)
- Embedding React app in Go binary: https://www.smartinary.com/blog/how-to-embed-a-react-app-in-a-go-binary/ (MEDIUM confidence, consistent with stdlib embed docs)
- JWT token storage comparison: https://www.descope.com/blog/post/developer-guide-jwt-storage (MEDIUM confidence, multiple sources agree)
- TanStack Query best practices: https://tanstack.com/query/latest/docs/framework/react/overview (HIGH confidence, official docs)
- Direct codebase analysis: `internal/server/server.go`, `internal/server/middleware.go`, `internal/auth/auth.go`, `internal/api/types.go`, `cmd/flowy-server/main.go` — existing architecture constraints (HIGH confidence)

---

*Architecture research for: flowy v2.0 — React SPA + Go REST API integration*
*Researched: 2026-03-20*
