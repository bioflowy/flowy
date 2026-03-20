---
phase: 21-wdl-registry
plan: "02"
subsystem: api
tags: [wdl, registry, http-handlers, sha256, content-addressed, rbac]

requires:
  - phase: 21-01
    provides: Registry store interface — PushRegistryVersion, GetRegistryVersionByTag, GetRegistryVersionByDigest, GetRegistryEntry, UpsertRegistryTag

provides:
  - POST /projects/{slug}/registry — push WDL with SHA-256 digest + auto-classification (201/200/422/409)
  - GET /projects/{slug}/registry/{name} — pull WDL by ?tag= or ?digest= (200/400/404)
  - Stub handlers for list/versions/delete/tag/delete-tag (Plan 03)
  - Registry route group wired inside /{slug} route block (auth-enabled only)
  - Full test coverage for push and pull handlers (10 tests)

affects: [21-03, any phase using registry HTTP API]

tech-stack:
  added: []
  patterns:
    - "computeDigest: sha256.Sum256 → 'sha256:<hex>' format"
    - "classifyWDL: parser.Parse then check doc.Workflows/doc.Tasks"
    - "PushRegistryVersion idempotent: returns (version, created bool, err) — handler maps to 201/200"
    - "Kind mismatch detection: error string contains 'kind' → map to 409"

key-files:
  created:
    - internal/server/handlers_registry.go
    - internal/server/handlers_registry_test.go
  modified:
    - internal/api/types.go
    - internal/server/server.go

key-decisions:
  - "Stub handlers (list/versions/delete/tag/delete-tag) added in handlers_registry.go returning 501 — avoids Plan 03 being blocked on compilation"
  - "Kind mismatch error detection uses strings.Contains(err.Error(), 'kind') — matches the error returned by PushRegistryVersion"
  - "createRegistryTestProject uses uuid-based unique slugs to avoid conflicts between parallel tests"

patterns-established:
  - "Registry handler pattern: extract project from context, validate input, call store, map store result to HTTP status"
  - "Pull handler: mutual exclusion of ?tag= and ?digest= enforced at handler level with specific 400 messages"

requirements-completed: [REG-01, REG-03, REG-05]

duration: 6min
completed: 2026-03-20
---

# Phase 21 Plan 02: Registry Push/Pull Handlers Summary

**SHA-256 content-addressed WDL push/pull via HTTP: auto-classifies tool/workflow, idempotent push (201/200), 422 for bad WDL, 409 for kind mismatch, pull by tag or digest**

## Performance

- **Duration:** ~6 min
- **Started:** 2026-03-20T01:44:00Z
- **Completed:** 2026-03-20T01:49:58Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- Push handler computes SHA-256 digest, classifies WDL via `parser.Parse`, calls `PushRegistryVersion`, and maps the `created` bool to 201 vs 200
- Pull handler supports `?tag=` and `?digest=` with mutually exclusive validation and specific 404 error messages
- 10 tests cover: new push 201, idempotent 200, invalid WDL 422, invalid name 400, classification tool/workflow, kind mismatch 409, pull by tag 200, pull by digest 200, not found 404, no param 400
- Registry routes wired inside `/{slug}` route group (auth-enabled only) with RBAC: executor=push, viewer=pull, admin=tag/delete

## Task Commits

1. **Task 1: Add registry API types and push/pull handlers** - `d636b08` (feat)
2. **Task 2: Wire registry routes and add push/pull tests** - `1e65a64` (feat)

## Files Created/Modified

- `internal/api/types.go` — Appended 6 registry DTO types: RegistryPushRequest, RegistryPushResponse, RegistryPullResponse, RegistryEntryResponse, RegistryVersionResponse, RegistryTagRequest
- `internal/server/handlers_registry.go` — Push/pull handlers, computeDigest, classifyWDL, validTagName helpers, stub handlers for Plan 03
- `internal/server/server.go` — Added /registry route group inside /{slug} block with role requirements
- `internal/server/handlers_registry_test.go` — 10 tests for push and pull handlers

## Decisions Made

- Added stub handlers (returning 501) for list/versions/delete/tag/delete-tag in handlers_registry.go so the server compiles with all routes wired before Plan 03 implements them.
- Kind mismatch detection uses `strings.Contains(err.Error(), "kind")` which matches the error returned by the SQLite store's `PushRegistryVersion` implementation.

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Push and pull endpoints functional and tested; Plan 03 can implement list, versions, tag management, and delete on top of the same pattern.
- Stub handlers return 501 to prevent confusion with 404.

---
*Phase: 21-wdl-registry*
*Completed: 2026-03-20*
