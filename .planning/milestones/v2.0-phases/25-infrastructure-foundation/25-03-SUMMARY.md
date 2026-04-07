---
phase: 25-infrastructure-foundation
plan: "03"
subsystem: infra
tags: [go, embed, spa, reverse-proxy, react, vite, makefile]

# Dependency graph
requires:
  - phase: 25-01
    provides: web/dist/ React SPA build artifacts embedded by web/embed.go
provides:
  - flowy-ui Go binary serving embedded React SPA on :3000 with SPA fallback
  - web/embed.go go:embed package exporting DistFS
  - cmd/flowy-ui/main.go with spaHandler and ReverseProxy for /api/*
  - Makefile build-ui target with pnpm -> go pipeline
affects:
  - 25-04
  - phase-26-auth
  - deployment

# Tech tracking
tech-stack:
  added:
    - "Go embed (stdlib io/fs, embed)"
    - "Go net/http/httputil.ReverseProxy with Rewrite API (Go 1.20+)"
  patterns:
    - "go:embed all:dist in separate package (web/) to avoid .. path restriction"
    - "Pre-flight fs.Open check in spaHandler to avoid ResponseWriter interception"
    - "Makefile phony target with file-based prerequisite (web/dist) for correct ordering"

key-files:
  created:
    - web/embed.go
    - cmd/flowy-ui/main.go
  modified:
    - Makefile
    - .gitignore

key-decisions:
  - "go:embed lives in web/embed.go (not cmd/flowy-ui) — Go forbids .. in embed paths"
  - "ReverseProxy.Rewrite API used instead of deprecated Director (Go 1.20+)"
  - "SPA fallback via pre-flight fs.Open check, not 404 status interception"
  - "vite.config.ts base changed from './' to '/' — relative base breaks asset URLs when served at root"
  - "Catch-all route added to App.tsx for React Router to match /projects on hard refresh"

patterns-established:
  - "Pattern 1: Embed sub-package — place go:embed directive in the same directory as the assets"
  - "Pattern 2: SPA handler — pre-flight fs.Open before serving; fallback to index.html on error"
  - "Pattern 3: Makefile file-prerequisite — use web/dist as phony dependency to enforce build order"

requirements-completed: [INFRA-01, INFRA-02, INFRA-05]

# Metrics
duration: 15min
completed: "2026-03-24"
---

# Phase 25 Plan 03: flowy-ui Binary Summary

**Go binary serving embedded React SPA via go:embed with SPA fallback routing and /api/* reverse proxy to flowy-server**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-03-24T03:01:53Z
- **Completed:** 2026-03-24T03:16:32Z
- **Tasks:** 3 (2 auto + 1 human-verify checkpoint)
- **Files modified:** 4

## Accomplishments

- web/embed.go created with `//go:embed all:dist` exporting DistFS for use by cmd/flowy-ui
- cmd/flowy-ui/main.go implements spaHandler (SPA fallback via pre-flight fs.Open) and ReverseProxy (/api/* to flowy-server with prefix stripping)
- Makefile build-ui target orchestrates `pnpm install --frozen-lockfile && pnpm build` then `go build -o ./flowy-ui`; clean target extended; .gitignore updated
- User verified SPA renders correctly in browser on both / and /projects (hard refresh)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create web/embed.go and cmd/flowy-ui/main.go** - `53ff45a` (feat)
2. **Task 2: Add build-ui Makefile target and update .gitignore** - `215628a` (feat)
3. **Task 3 (checkpoint fix): Fix SPA base path and catch-all route** - `ba8798f` (fix)

## Files Created/Modified

- `web/embed.go` - Go package exporting `DistFS embed.FS` with `//go:embed all:dist`
- `cmd/flowy-ui/main.go` - flowy-ui binary: spaHandler, ReverseProxy, startup index.html assertion
- `Makefile` - Added UI_BIN, build-ui target, web/dist file-prerequisite target, extended clean
- `.gitignore` - Added web/node_modules/, web/dist/, /flowy-ui

## Decisions Made

- go:embed directive placed in `web/embed.go` (not cmd/flowy-ui) because Go's embed does not permit `..` path traversal; the package exports `DistFS` which main.go imports
- Used `ReverseProxy.Rewrite` (Go 1.20+ API) rather than the deprecated `Director` field
- SPA fallback implemented via pre-flight `fs.Open` check; avoids the ResponseWriter-already-written problem that arises when intercepting 404 after `http.FileServer` responds

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed Vite base path causing broken asset URLs**
- **Found during:** Task 3 verification (human-verify checkpoint)
- **Issue:** vite.config.ts had `base: './'` which caused relative asset paths; when served from root by Go's http.FileServer, all JS/CSS 404'd
- **Fix:** Changed `base` to `'/'` in vite.config.ts; re-ran `make build-ui`
- **Files modified:** web/vite.config.ts, web/dist/ (rebuilt)
- **Verification:** curl http://localhost:3000/ returned assets with /assets/ absolute paths; browser DevTools showed no 404s
- **Committed in:** ba8798f

**2. [Rule 1 - Bug] Added React Router catch-all route so /projects renders on hard refresh**
- **Found during:** Task 3 verification (human-verify checkpoint)
- **Issue:** App.tsx lacked a `<Route path="*">` catch-all, so navigating directly to /projects showed a blank page after spaHandler served index.html
- **Fix:** Added catch-all route `<Route path="*" element={<Navigate to="/projects" replace />}` in App.tsx
- **Files modified:** web/src/App.tsx
- **Verification:** curl http://localhost:3000/projects returned DOCTYPE; browser confirmed sidebar rendered on hard refresh
- **Committed in:** ba8798f

---

**Total deviations:** 2 auto-fixed (2 bugs found during human-verify checkpoint)
**Impact on plan:** Both fixes necessary for SPA routing correctness. No scope creep.

## Issues Encountered

The two deviations above were the only issues. Both were surfaced at the human-verify checkpoint when the user tested in a real browser and confirmed resolution.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- flowy-ui binary is production-ready for local use: `make build-ui && ./flowy-ui`
- /api/* proxy is wired; flowy-server must be running on :8124 for API calls to succeed
- Phase 26 (auth) can build on this binary without changes to the server layer

---
*Phase: 25-infrastructure-foundation*
*Completed: 2026-03-24*
