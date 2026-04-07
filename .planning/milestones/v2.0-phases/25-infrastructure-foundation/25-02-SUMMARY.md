---
phase: 25-infrastructure-foundation
plan: "02"
subsystem: infra
tags: [cors, go-chi, middleware, config]

# Dependency graph
requires: []
provides:
  - CORSConfig struct in internal/config/config.go with Enabled and AllowedOrigins fields
  - cors.Handler middleware in internal/server/server.go registered at top-level chi router before JWT middleware
  - go-chi/cors v1.2.2 dependency
affects: [25-infrastructure-foundation, 26-auth, web-frontend]

# Tech tracking
tech-stack:
  added: [github.com/go-chi/cors v1.2.2]
  patterns:
    - Config-gated middleware pattern: optional middleware added only when cfg.CORS.Enabled is true
    - CORS middleware positioned at top-level chi router (before route groups with JWT) to allow OPTIONS preflight without auth

key-files:
  created: []
  modified:
    - internal/config/config.go
    - internal/server/server.go
    - go.mod
    - go.sum

key-decisions:
  - "CORS is disabled by default (cors.enabled: false) — no behavior change for existing deployments"
  - "CORS middleware placed at top-level chi router before all route groups to ensure OPTIONS preflight succeeds without JWT authentication"
  - "AllowCredentials set to false for v2.0 MVP (sessionStorage JWT, not cookie-based)"

patterns-established:
  - "Config-gated middleware: feature flags in Config struct control optional middleware registration"

requirements-completed: [INFRA-04]

# Metrics
duration: 8min
completed: 2026-03-24
---

# Phase 25 Plan 02: CORS Middleware Summary

**Config-gated CORS middleware added to flowy-server using go-chi/cors, enabling Vite dev server (localhost:5173) cross-origin API requests without affecting production deployments**

## Performance

- **Duration:** 8 min
- **Started:** 2026-03-24T00:00:00Z
- **Completed:** 2026-03-24T00:08:00Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- Added CORSConfig struct to internal/config/config.go with Enabled (bool) and AllowedOrigins ([]string) fields
- Added CORS field to Config struct, maintaining backward compatibility (defaults to disabled)
- Registered cors.Handler middleware at top-level chi router in server.go, positioned before JWT route groups so OPTIONS preflight requests are not rejected by auth middleware
- All existing server and config tests pass with no regressions

## Task Commits

Each task was committed atomically:

1. **Task 1: Add CORSConfig to config.go and go-chi/cors dependency** - `896111f` (feat)
2. **Task 2: Register CORS middleware in flowy-server router** - `44f2aa5` (feat)

## Files Created/Modified

- `internal/config/config.go` - Added CORSConfig struct with Enabled/AllowedOrigins fields; added CORS field to Config struct
- `internal/server/server.go` - Added go-chi/cors import; registered config-gated cors.Handler middleware after Recoverer, before NotFound/routes
- `go.mod` - Added github.com/go-chi/cors v1.2.2 direct dependency
- `go.sum` - Updated checksums for go-chi/cors

## Decisions Made

- CORS disabled by default: no change to existing deployments; opt-in via `cors.enabled: true` in config
- CORS middleware placed at top-level router (not inside route groups) to prevent OPTIONS preflight from hitting JWT middleware — this is the correct fix for the RESEARCH.md Pitfall 3 scenario
- `AllowCredentials: false` because v2.0 uses sessionStorage JWT, not httpOnly cookies

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required. To enable CORS during development, add to flowy.yaml:

```yaml
cors:
  enabled: true
  allowed_origins:
    - "http://localhost:5173"
```

## Next Phase Readiness

- CORS infrastructure complete; Vite dev server can make cross-origin requests to flowy-server when cors.enabled is true
- No blockers for subsequent phases

---
*Phase: 25-infrastructure-foundation*
*Completed: 2026-03-24*
