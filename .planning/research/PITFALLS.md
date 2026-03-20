# Pitfalls Research

**Domain:** Adding React + TypeScript SPA to an existing Go REST API (flowy v2.0 Web UI)
**Researched:** 2026-03-20
**Confidence:** HIGH (go:embed and SPA routing from official Go docs and community patterns; JWT storage from security literature; CORS from go-chi/cors official docs; S3 download pitfalls from AWS documentation and community bug reports; shadcn/ui from official docs)

---

## Critical Pitfalls

### Pitfall 1: SPA Routing Falls Through to Go's 404 Handler on Browser Refresh

**What goes wrong:**
React Router handles client-side navigation transparently, but when a user bookmarks `/projects/bioinformatics` or refreshes the page, the browser sends a real HTTP GET to the Go server for that path. The Go router has no route registered for `/projects/bioinformatics` (it only knows about API routes and the static file server), so the chi `NotFound` handler fires and returns a JSON `{"error":"not found"}` instead of `index.html`. The user sees a blank page or an error in the browser.

**Why it happens:**
Go's `http.FileServer` (and by extension `http.FS`) returns a 404 response for paths that do not map to a real file in the embedded FS. The SPA's routes exist only in the JavaScript bundle; the server never knows about them. Developers embed the React build and serve it, test by loading `/`, see it work, and consider it done — without testing `F5` on any deep route.

The existing `flowy-server` router has this pattern already:
```go
r.NotFound(func(w http.ResponseWriter, req *http.Request) {
    writeError(w, http.StatusNotFound, "not found")
})
```
This returns JSON 404. The `flowy-ui` binary must override this with an `index.html` fallback for all non-API, non-asset paths.

**How to avoid:**
In `flowy-ui`, use a custom `http.ResponseWriter` wrapper that intercepts 404 responses from the file server and re-serves `index.html` instead. The canonical Go pattern:

```go
func spaHandler(staticFS fs.FS) http.HandlerFunc {
    fsServer := http.FileServer(http.FS(staticFS))
    return func(w http.ResponseWriter, r *http.Request) {
        // Try to serve the real file first.
        // If the file doesn't exist, the file server returns 404.
        // Intercept that 404 and serve index.html for React Router.
        interceptor := &spaResponseWriter{ResponseWriter: w}
        fsServer.ServeHTTP(interceptor, r)
        if interceptor.status == http.StatusNotFound {
            // Reset and serve index.html
            r2 := r.Clone(r.Context())
            r2.URL.Path = "/"
            fsServer.ServeHTTP(w, r2)
        }
    }
}
```

Register this handler only for paths that do NOT start with `/api/`. API routes must still return real 404s, not `index.html`.

**Warning signs:**
- `F5` on any route other than `/` shows a blank page or JSON error
- `curl -v http://localhost:PORT/projects/foo` returns `{"error":"not found"}` from `flowy-ui`
- The catch-all file server is registered without a custom 404 fallback

**Phase to address:**
Phase 1 (flowy-ui binary scaffolding + go:embed setup) — the fallback handler must be in place before any route is tested in the browser. Test immediately by navigating to a deep route and pressing F5.

---

### Pitfall 2: go:embed Embeds the Wrong Subtree — 404 for All Assets

**What goes wrong:**
The Vite build outputs files into `ui/dist/`. The Go source file uses `//go:embed ui/dist` which embeds the `ui/dist` directory at path `ui/dist/index.html`. But the file server is configured to serve from the embedded FS root, so it looks for `index.html` at the root — which does not exist. All requests return 404.

The subtler variant: `//go:embed all:ui/dist` embeds correctly, but `http.FS(embeddedFS)` serves under path `ui/dist/index.html`. The file server returns 404 for `/index.html` because it looks in the wrong subtree.

**Why it happens:**
`go:embed` preserves the full directory structure from the source tree into the embedded FS. A file at `ui/dist/index.html` is accessible as `ui/dist/index.html` in the embedded FS, not as `index.html`. Developers forget the `fs.Sub` call that strips the prefix.

**How to avoid:**
Always call `fs.Sub` to create a new FS rooted at the build output directory:

```go
//go:embed all:ui/dist
var embeddedUI embed.FS

func uiFS() fs.FS {
    sub, err := fs.Sub(embeddedUI, "ui/dist")
    if err != nil {
        panic(fmt.Sprintf("embed: sub ui/dist: %v", err))
    }
    return sub
}
```

The `all:` prefix is required to include dot-files (`.nojekyll`, hidden Vite output). Without it, hidden files are silently excluded, causing partial failures.

Verify with a startup test: call `fs.Stat(uiFS(), "index.html")` at `init` time and panic if it fails. This surfaces the wrong subtree immediately on binary startup, not at runtime.

**Warning signs:**
- `http://localhost:PORT/` returns 404 or directory listing instead of the React app
- `fs.Stat(embeddedFS, "index.html")` returns "file not found"
- `fs.Stat(embeddedFS, "ui/dist/index.html")` succeeds when it should not

**Phase to address:**
Phase 1 (flowy-ui binary scaffolding) — add the startup assertion immediately after wiring `go:embed`.

---

### Pitfall 3: CORS Middleware Applied Inside a Route Group — Preflight OPTIONS Never Reaches Middleware

**What goes wrong:**
The development workflow requires `flowy-ui` (Vite dev server on `:5173`) to call `flowy-server` (on `:8124`). CORS headers are needed on `flowy-server` in dev mode. The developer adds `go-chi/cors` middleware inside a route group rather than at the top-level router. OPTIONS preflight requests from the browser reach the router, find no matching route, and get a JSON 404 before the CORS middleware has a chance to add `Access-Control-Allow-*` headers. The browser sees a CORS error and blocks the actual request.

**Why it happens:**
The chi docs say "middleware registered within a group only applies to routes within that group." Preflight OPTIONS requests are a separate HTTP method — they only match if an OPTIONS route is explicitly registered. When CORS middleware is applied inside a group with `r.Group(func(r chi.Router) { r.Use(cors.Handler(...)); r.Get(...) })`, the OPTIONS handler is not automatically registered.

The existing `flowy-server` router (confirmed in `internal/server/server.go`) registers middleware at the top level but the CORS middleware for dev mode will be added later. The go-chi/cors docs explicitly warn: "Applying with within a `r.Group()` or using `With()` will not work without routes matching OPTIONS added."

**How to avoid:**
Register `go-chi/cors` at the very top of the chi.Mux, before any route groups:

```go
r := chi.NewRouter()
r.Use(middleware.Logger)
r.Use(middleware.Recoverer)

// CORS must be top-level middleware — NOT inside a route group.
if cfg.UI.CORSEnabled {
    r.Use(cors.Handler(cors.Options{
        AllowedOrigins:   []string{"http://localhost:5173"},
        AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
        AllowCredentials: true,
        MaxAge:           300,
    }))
}
```

Never use `AllowedOrigins: []string{"*"}` with `AllowCredentials: true` — browsers reject this combination. Use the explicit Vite dev server origin.

Gate CORS behind a config flag (`cors_enabled: true` in `flowy.yaml` for dev). Production deployment via `flowy-ui` (same origin) needs no CORS.

**Warning signs:**
- Browser console shows `CORS policy: No 'Access-Control-Allow-Origin' header`
- Preflight OPTIONS requests return 404 in network inspector
- CORS works for GET but fails for POST/PUT (POST triggers a preflight that fails)
- `AllowCredentials: true` with `AllowedOrigins: ["*"]` — this silently breaks in browsers

**Phase to address:**
Phase 2 (development proxy setup / CORS configuration on flowy-server) — add CORS support to flowy-server before building any authenticated API calls in the frontend.

---

### Pitfall 4: JWT Token Stored in localStorage — XSS Exposes the Token

**What goes wrong:**
The React app stores the JWT from `POST /auth/login` in `localStorage`. A single XSS vulnerability anywhere in the app (including injected via a third-party script in `node_modules`) gives an attacker access to `localStorage`, stealing the token and enabling full account impersonation for the token's lifetime.

For a bioinformatics workflow engine managing sensitive genomic data with RBAC roles, a stolen admin JWT gives full access to all projects, all registry entries, and all output files.

**Why it happens:**
`localStorage` is the path of least resistance: no server changes needed, survives page refresh, simple `localStorage.getItem('token')` everywhere. Most tutorials demonstrate it this way. The XSS attack vector feels abstract until it happens.

**How to avoid:**
For the v2.0 milestone, store the JWT in an `httpOnly; Secure; SameSite=Strict` cookie set by the server. The browser sends it automatically on every request; JavaScript cannot read it.

Server-side change required in `handlers_auth.go`:
```go
http.SetCookie(w, &http.Cookie{
    Name:     "flowy_token",
    Value:    tokenString,
    HttpOnly: true,
    Secure:   true,         // enforce in production; relax to false in localhost dev
    SameSite: http.SameSiteStrictMode,
    Path:     "/",
    MaxAge:   int(tokenTTL.Seconds()),
})
```

Frontend change: remove `Authorization: Bearer ...` header from API calls; the cookie is sent automatically. The `credentials: 'include'` fetch option is required for cross-origin calls during development.

If the project defers httpOnly cookies to a later milestone and uses localStorage for MVP, document this as technical debt and enforce a short token TTL (15-30 minutes) to bound exposure.

**Warning signs:**
- `localStorage.setItem('token', ...)` anywhere in the frontend codebase
- Token visible in browser DevTools → Application → Local Storage
- Any `eval()` or `dangerouslySetInnerHTML` in the React codebase with user-provided content

**Phase to address:**
Phase 3 (Login screen + JWT session management) — the storage strategy must be decided before writing a single line of auth code in the frontend.

---

### Pitfall 5: Vite base Path Mismatch — Assets Load in Dev but 404 in Production

**What goes wrong:**
The Vite dev server serves assets from `/assets/main-abc123.js`. The production build embedded in Go serves from the same path. But if the `flowy-ui` binary is placed behind a reverse proxy at `/ui/`, all asset requests go to `/ui/assets/main-abc123.js` — a path that the file server does not recognise because it expects `/assets/`. All JavaScript and CSS fail to load; the browser renders a blank page with no errors in the Go logs.

The subtler case: `<script src="/assets/main.js">` in `index.html` uses an absolute path. If the Go binary is configured to serve the SPA at a non-root path, every absolute asset reference breaks.

**Why it happens:**
Vite defaults to `base: "/"` which writes absolute paths into `index.html`. Developers test locally at `localhost:5173/` (root) and `localhost:PORT/` (root), never behind a sub-path proxy. The mismatch only appears in deployed environments.

**How to avoid:**
Set `base: "./"` in `vite.config.ts` to generate relative asset paths. Relative paths work regardless of the mount point:

```typescript
// vite.config.ts
export default defineConfig({
  base: "./",   // relative paths — works at any mount point
  // ...
})
```

Verify after build: `grep -r 'src="/' ui/dist/index.html` should return nothing. All `src` and `href` attributes should be relative (`./assets/...`).

For `flowy-ui` serving at root (`/`), absolute paths also work — but relative paths are strictly safer and cost nothing.

**Warning signs:**
- Assets 404 when tested behind a path-prefixed proxy
- `index.html` in the build output contains `src="/assets/..."` (absolute paths)
- Vite config has no `base` setting (defaults to `"/"`)

**Phase to address:**
Phase 1 (Vite + React project setup) — set `base: "./"` in the initial `vite.config.ts` before any component is built.

---

### Pitfall 6: S3 File Download via Go Proxy Buffers Entire File in Memory

**What goes wrong:**
The file browser requires downloading workflow output files from S3. The naive implementation fetches the S3 object via AWS SDK into memory (`GetObject` → `io.ReadAll`) then writes it to the HTTP response. For a 2 GB genomics output file (BAM, FASTQ), this allocates 2 GB on the Go heap, causes a memory spike, and may OOM the process. Under concurrent downloads, multiple goroutines each hold multi-GB buffers.

**Why it happens:**
`io.ReadAll(resp.Body)` is the common pattern in Go tutorials. It works for small files and looks simple. The S3 SDK's `GetObject` response is an `io.ReadCloser` — developers miss that they can stream it directly to `http.ResponseWriter` without buffering.

**How to avoid:**
Stream the S3 response body directly to the HTTP response writer:

```go
func (s *Server) handleDownloadFile(w http.ResponseWriter, r *http.Request) {
    result, err := s.s3Client.GetObject(r.Context(), &s3.GetObjectInput{
        Bucket: aws.String(bucket),
        Key:    aws.String(key),
    })
    if err != nil { /* ... */ }
    defer result.Body.Close()

    // Set filename for browser download prompt.
    filename := path.Base(key)
    w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
    w.Header().Set("Content-Type", "application/octet-stream")
    if result.ContentLength != nil {
        w.Header().Set("Content-Length", strconv.FormatInt(*result.ContentLength, 10))
    }

    // Stream: no io.ReadAll, no intermediate buffer.
    if _, err := io.Copy(w, result.Body); err != nil {
        log.Printf("download: stream copy: %v", err)
    }
}
```

`Content-Length` must be set before the first write — once `w.WriteHeader` or the first `Write` is called, headers are locked.

For very large files (>5 GB), consider pre-signed S3 URLs as an alternative: generate a time-limited URL server-side and redirect the browser to it. The browser downloads directly from S3, bypassing the Go proxy entirely. This is the preferred approach when MinIO/S3 is in the same network as the browser (no extra latency).

**Warning signs:**
- `io.ReadAll(s3Result.Body)` in any download handler
- Memory spikes on the `flowy-ui` process during file downloads
- No `Content-Length` header on download responses (browser cannot show progress)
- Missing `Content-Disposition: attachment` header (browser renders the file inline instead of downloading)

**Phase to address:**
Phase 6 (File browser + download) — implement streaming from the first line; never prototype with `io.ReadAll` and "fix it later."

---

### Pitfall 7: TanStack Query Retries 401 Responses — Simultaneous Token Refresh Storms

**What goes wrong:**
TanStack Query's default retry behaviour retries failed queries 3 times. When a JWT expires, every active query on the current page fires simultaneously, all receive 401, and all trigger a token refresh attempt. The result is 3-10 concurrent `POST /auth/refresh` calls. If the server issues a new token on the first call and invalidates the old one, the subsequent refresh calls fail, the user is logged out unexpectedly, and query data is never populated.

Even without refresh tokens (flowy v2.0 uses single tokens, not refresh token pairs), the retry storm floods the server's `POST /auth/login` endpoint or causes confusing error states where some queries succeed with a new token and others fail with the old one.

**Why it happens:**
Developers wire up TanStack Query with default settings, add a global `onError` handler that redirects to `/login` on 401, and test with a non-expired token. The expiry scenario only manifests in production or in long browser sessions. The 3x retry default is not visible in the query configuration — it is a global default.

**How to avoid:**
Disable automatic retries for 401 responses at the QueryClient level:

```typescript
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: (failureCount, error) => {
        // Never retry on 401 (auth error) or 403 (permission error).
        if (error instanceof ApiError && (error.status === 401 || error.status === 403)) {
          return false;
        }
        return failureCount < 2;
      },
    },
  },
});
```

For 401 handling: intercept at the fetch layer (Axios interceptor or a custom `fetchWithAuth` wrapper), attempt token refresh once, retry the original request with the new token, then propagate the error if refresh fails. TanStack Query should see at most one 401 — from the failed refresh, at which point `retry: false` for 401 prevents further retries.

**Warning signs:**
- Browser network tab shows 3+ simultaneous `POST /auth/login` or refresh calls after a token expires
- QueryClient created without a custom `retry` function
- `onError` 401 handler redirects to login without deduplicating refresh attempts

**Phase to address:**
Phase 3 (Login + JWT session management) — configure the QueryClient's retry policy and auth error handling before building any authenticated screens.

---

### Pitfall 8: flowy-ui Binary Hard-Codes flowy-server URL — Breaks in Diverse Deployment Environments

**What goes wrong:**
The `flowy-ui` binary proxies API calls to `flowy-server`. If the server URL is baked into the binary as a compile-time constant or a default that cannot be overridden, deployers who run `flowy-server` on a non-default address (not `localhost:8124`) cannot configure the proxy without recompiling. The binary is useless in any environment where the API server is on a different host (e.g., `flowy-server` on a cluster node, `flowy-ui` on a gateway).

**Why it happens:**
During development, both binaries run locally. `localhost:8124` always works. The need for configuration only becomes obvious when the first non-developer tries to deploy the system.

**How to avoid:**
Expose the API server URL as a required flag on `flowy-ui`:

```go
apiAddr := flag.String("api-addr", "http://localhost:8124", "address of flowy-server")
```

The `flowy-ui` binary either:
1. Proxies `/api/` to `apiAddr` (reverse proxy pattern — browser calls same-origin `/api/`, UI binary forwards to flowy-server), or
2. Embeds the API base URL in `index.html` at startup time by templating it into the HTML (simpler but requires a template step in the Go binary).

Option 1 (reverse proxy) is strongly preferred: it avoids CORS entirely in production (same origin), keeps credentials handling simple (httpOnly cookies path is `/api/`), and allows the API server to be unreachable from the public internet.

Option 2 (inject URL into HTML) requires CORS to be enabled on `flowy-server` in all production deployments — a security surface that should only be needed in development.

**Warning signs:**
- `const apiBase = "http://localhost:8124"` anywhere in Go source
- `flowy-ui` has no CLI flag for the API server address
- `flowy-server` CORS config lists production origins (not just `localhost:5173`)

**Phase to address:**
Phase 1 (flowy-ui binary scaffolding) — the reverse proxy vs. direct API pattern must be decided before any Go code is written. This decision affects the CORS requirements, the fetch configuration in React, and the deployment documentation.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| JWT in localStorage | No server-side cookie changes | Any XSS steals the token permanently for its TTL | Only if TTL is ≤ 15 min and documented as debt; never in production with long TTLs |
| io.ReadAll for S3 downloads | Simple implementation | OOM under concurrent large file downloads | Never — streaming costs the same lines of code |
| Absolute Vite base path (`"/"`) | Default behavior, no config | Breaks when deployed behind a sub-path proxy | Acceptable only if deployment is guaranteed to always be at root |
| Hardcoded `localhost:8124` API URL | Works for local dev | Unusable in any real deployment | Never — always expose as a CLI flag |
| SPA fallback serving `index.html` for all paths including `/api/` | Simpler middleware | API errors masked as React app; confusing debugging | Never — always exclude `/api/` prefix from fallback |
| Disabled retry for all errors (not just 401) | Simpler QueryClient config | Network blips cause unnecessary query failures | Acceptable for MVP; distinguish 401/403 from transient errors in production |
| CORS wildcard `AllowedOrigins: ["*"]` with credentials | Works for testing | Browsers refuse `*` + credentials; security hole | Never — always list explicit dev origins |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| go:embed + Vite | Embedding `ui/dist` without `fs.Sub` — file paths include `ui/dist/` prefix | `fs.Sub(embeddedFS, "ui/dist")` creates FS rooted at build output |
| go:embed hidden files | Using `//go:embed ui/dist` without `all:` prefix — Vite's `.vite/` cache dir and hidden manifests excluded | Use `//go:embed all:ui/dist` |
| go-chi/cors | Applying CORS middleware inside a route group — OPTIONS preflight never reaches it | Always register at top-level `r.Use(cors.Handler(...))` before any routes |
| go-chi/cors credentials | `AllowCredentials: true` + `AllowedOrigins: ["*"]` | Use explicit origin list: `["http://localhost:5173"]` |
| TanStack Query + 401 | Default 3x retry floods the server on token expiry | Disable retry for 401/403 in QueryClient `defaultOptions` |
| S3 download + browser | Missing `Content-Disposition: attachment` — browser renders binary as text or opens inline | Always set `Content-Disposition: attachment; filename="..."` |
| S3 download + browser | Missing `Content-Length` — browser shows no progress bar | Set `Content-Length` from `result.ContentLength` before first write |
| S3 + CORS (presigned URLs) | Presigned URL generated with wrong region or scheme — browser CORS error | Generate presigned URLs server-side; redirect browser to them via 302; set S3 bucket CORS policy |
| Vite + httpOnly cookie | `fetch` requests without `credentials: 'include'` in dev mode — cookie not sent cross-origin | Set `credentials: 'include'` globally or in the Axios/fetch wrapper |
| chi NotFound + SPA | chi's JSON `NotFound` handler intercepts all unmatched paths before the SPA handler can serve `index.html` | Register SPA handler as a catch-all route at the end, not as chi's NotFound |

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| S3 download buffered in memory | `flowy-ui` process OOM; memory = file size per concurrent download | Stream `result.Body` directly to `http.ResponseWriter` | First download of a file > available RAM |
| No `Content-Length` on download | Browser shows indeterminate spinner; resumable downloads impossible | Read `result.ContentLength` from S3 GetObject response and set header | Every download — poor UX always |
| Serving build artifacts in dev mode from `go:embed` | Stale UI after frontend code changes — must rebuild Go binary | In dev mode, serve from the filesystem directly; use `go:embed` only for production build | Every frontend iteration in dev |
| Large shadcn/ui bundle from barrel imports | Slow initial load; all Radix UI primitives shipped even if unused | Import individual components (`import { Button } from "@/components/ui/button"`), never from barrel | When bundle exceeds ~200 KB gzipped |
| TanStack Query polling for run status | High request volume on long-running workflows with many concurrent users | Use exponential backoff for polling; `refetchInterval: (data) => data?.status === 'RUNNING' ? 5000 : false` | 10+ concurrent users polling 10+ runs |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| JWT in localStorage | Token theft via XSS; full account takeover for token TTL | Store in `httpOnly; SameSite=Strict` cookie; JavaScript cannot read it |
| Missing `Secure` flag on auth cookie | Cookie transmitted over plain HTTP; MITM token theft | Set `Secure: true` in production; gate on `cfg.UI.TLS` or `X-Forwarded-Proto` |
| CORS `AllowedOrigins: ["*"]` with `AllowCredentials: true` | Browsers block this; attackers on any domain can read authenticated responses if misconfigured | Explicit origin whitelist only; `"*"` and credentials are mutually exclusive per the CORS spec |
| S3 pre-signed URLs with no expiry | Permanent public access to output files | Set `PresignExpiry` to ≤ 1 hour; document that pre-signed links in browser history expire |
| `Content-Security-Policy` header absent | XSS vectors remain exploitable; no mitigation in depth | Add CSP header via middleware: `script-src 'self'`; restrict `connect-src` to the API origin |
| Proxy endpoint forwards all headers including `Host` | Host header injection on the proxied flowy-server | Strip `Host` and `X-Forwarded-*` headers before proxying; set explicit `X-Forwarded-For` |
| Download endpoint no authorization check | Any authenticated user can download any file path by guessing keys | Validate that the requested S3 key belongs to a run in a project the user has viewer access to |

---

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Browser refresh on any route except `/` shows error | Users lose navigation state and see errors; feels broken | Implement SPA fallback (Pitfall 1) — test by pressing F5 on every route |
| No loading state during S3 file downloads | User clicks download, nothing appears to happen; clicks again, double-downloads | Show a loading spinner or progress indicator; disable the button after first click |
| JWT expiry silently fails API calls | User works for 30 min, saves data, gets a mysterious error; data loss | Detect 401, show a "Session expired — please log in again" modal; preserve the current URL for post-login redirect |
| File browser shows S3 keys verbatim | `workflow-runs/abc123/output/sample.bam` is cryptic | Display only the filename; show the full path in a tooltip or details panel |
| Download of large file with no Content-Length | Browser progress bar is indeterminate; no way to estimate time remaining | Always set `Content-Length` from the S3 object's `ContentLength` field |
| Pagination absent on registry/run lists | Lists grow unbounded; browser hangs on 1000+ runs | Add server-side pagination from day one; default page size ≤ 50; add page size to all list responses |

---

## "Looks Done But Isn't" Checklist

- [ ] **SPA routing fallback:** Press F5 on `/projects/foo`, `/users`, `/registry/bar` — all must return the React app, not a 404 or JSON error.
- [ ] **go:embed correct subtree:** `fs.Stat(uiFS(), "index.html")` succeeds at startup; no `ui/dist/` prefix in served paths.
- [ ] **CORS preflight:** Send `OPTIONS /api/auth/login` with `Origin: http://localhost:5173` from a browser or curl — must return `Access-Control-Allow-Origin` and status 204, not 404.
- [ ] **JWT session persistence across refresh:** Log in, refresh the page (`F5`) — user must remain logged in (session survives). Cookie approach makes this automatic; localStorage approach requires explicit re-hydration on mount.
- [ ] **JWT expiry handling:** Let the token expire (set a short test TTL), then make an API call — must show a "session expired" prompt, not a silent failure.
- [ ] **S3 file download streaming:** Download a file > 10 MB — the `flowy-ui` process RSS must not spike by the file size. Verify with `ps aux` or a memory profiler.
- [ ] **Content-Disposition header:** Download any output file — browser must save it with the correct filename, not render it inline.
- [ ] **Production CORS disabled:** In production mode (flowy-ui serving SPA at root), `flowy-server` must NOT have `cors_enabled: true` — same-origin calls need no CORS.
- [ ] **Vite relative paths:** `grep -r 'src="/'  ui/dist/index.html` returns nothing — all asset paths must be relative.
- [ ] **API URL configurable:** `flowy-ui --api-addr http://remote-host:8124` must work; hardcoded `localhost:8124` must not appear in source.
- [ ] **Download authorization check:** `GET /download?key=workflow-runs/OTHER_PROJECT/secret.bam` must return 403, not the file.
- [ ] **shadcn/ui bundle size:** Vite build output `ui/dist/assets/*.js` gzipped size must be < 250 KB for initial chunk; verify with `du -sh ui/dist/assets/`.

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| SPA routing broken on refresh | LOW | Add `spaResponseWriter` interceptor to file server; no schema change; rebuild flowy-ui binary |
| go:embed wrong subtree | LOW | Add `fs.Sub` call and startup assertion; rebuild; caught immediately |
| CORS middleware in wrong position | LOW | Move `r.Use(cors.Handler(...))` to top of `newRouter()`; redeploy |
| JWT in localStorage (security retrofit) | MEDIUM | Add `SetCookie` to `handleLogin`; change frontend to drop `Authorization` header; set `credentials: 'include'`; requires coordinated deploy of both binaries |
| S3 download buffered in memory (OOM retrofit) | LOW | Replace `io.ReadAll` with `io.Copy`; no API or schema changes |
| Hardcoded API URL retrofit | MEDIUM | Add `--api-addr` flag; update all deployment scripts; rebuild binary |
| Token retry storm (TanStack Query retrofit) | LOW | Update `QueryClient` `defaultOptions.queries.retry`; frontend-only change |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| SPA routing fallback missing | Phase 1 — flowy-ui scaffolding | F5 on `/projects` returns React app; `curl /projects` returns HTML not JSON |
| go:embed wrong subtree | Phase 1 — flowy-ui scaffolding | Startup assertion `fs.Stat(uiFS(), "index.html")` passes |
| Vite absolute base path | Phase 1 — Vite project setup | `grep 'src="/' ui/dist/index.html` returns nothing |
| API URL hardcoded | Phase 1 — flowy-ui scaffolding | `flowy-ui --api-addr http://other:9000` successfully proxies requests |
| CORS middleware position | Phase 2 — CORS + dev proxy setup | OPTIONS preflight to `flowy-server` returns 204 with CORS headers from `localhost:5173` |
| JWT in localStorage | Phase 3 — Login + JWT session | Browser DevTools → Application → Cookies shows `flowy_token`; Local Storage is empty |
| TanStack Query retry storm | Phase 3 — Login + JWT session | Expired token triggers single redirect to login, not 3x retry flood in network tab |
| S3 download buffered in memory | Phase 6 — File browser + download | Process RSS does not grow by file size during download of 100 MB test file |
| Download missing Content-Disposition | Phase 6 — File browser + download | Browser saves file with correct name; does not open inline |
| Download authorization absent | Phase 6 — File browser + download | Requesting another project's file key returns 403 |

---

## Sources

- [go-chi/cors — Official README](https://github.com/go-chi/cors/blob/master/README.md) — CORS middleware placement, preflight pitfalls, credentials + wildcard mutual exclusion
- [Serving Single-Page Apps From Go — hackandsla.sh](https://hackandsla.sh/posts/2021-11-06-serve-spa-from-go/) — spaResponseWriter pattern, Go SPA fallback canonical reference
- [embed package — pkg.go.dev](https://pkg.go.dev/embed) — go:embed path rules, all: prefix, leading/trailing slash constraints
- [go:embed use case of a static subdirectory — golang/go Issue #51987](https://github.com/golang/go/issues/51987) — fs.Sub requirement for subdirectory roots
- [Fileserver example for an SPA — go-chi/chi Issue #611](https://github.com/go-chi/chi/issues/611) — chi-specific SPA catch-all pattern
- [React Authentication: How to Store JWT in a Cookie — Medium](https://medium.com/@ryanchenkie_40935/react-authentication-how-to-store-jwt-in-a-cookie-346519310e81) — httpOnly cookie approach
- [LocalStorage vs Cookies — DEV Community](https://dev.to/cotter/localstorage-vs-cookies-all-you-need-to-know-about-storing-jwt-tokens-securely-in-the-front-end-15id) — XSS risk of localStorage, hybrid approach
- [Streaming Downloads from S3 in React and Node.js — DEV Community](https://dev.to/samitkapoor/streaming-downloads-from-s3-in-react-and-nodejs-the-right-way-3kce) — memory pitfalls of buffered downloads
- [Deep dive into CORS configs on Amazon S3 — AWS Blog](https://aws.amazon.com/blogs/media/deep-dive-into-cors-configs-on-aws-s3-how-to/) — S3 bucket CORS policy for presigned URLs
- [Building for Production — Vite Official Docs](https://vite.dev/guide/build) — base option, relative paths, asset rewriting
- [Inconsistent asset paths between dev and build — vitejs/vite Issue #10217](https://github.com/vitejs/vite/issues/10217) — base path edge cases
- [React Query Auth Token Refresh — elazizi.com](https://elazizi.com/posts/react-query-auth-token-refresh/) — 401 retry deduplication patterns with TanStack Query
- [5 Critical shadcn/ui Pitfalls — paulserban.eu](https://www.paulserban.eu/blog/post/5-critical-shadcnui-pitfalls-that-break-production-apps-and-how-to-avoid-them/) — bundle size, tree shaking, barrel import pitfalls
- [How to embed a React app in a Go binary — Smartinary LLC](https://www.smartinary.com/blog/how-to-embed-a-react-app-in-a-go-binary/) — fs.Sub pattern, http.FS wiring
- Fearless CORS: a design philosophy for CORS middleware (jub0bs.com) — security pitfalls of overly broad CORS origins

---
*Pitfalls research for: adding React SPA (React 19, TypeScript, Vite, shadcn/ui, TanStack Query) to existing Go REST API (chi v5, JWT auth, S3 storage) via go:embed in separate flowy-ui binary*
*Researched: 2026-03-20*
