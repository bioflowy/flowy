---
phase: 23-provenance-tracking
plan: "02"
subsystem: api
tags: [provenance, audit-trail, rest-api, chi, sqlite, go]

# Dependency graph
requires:
  - phase: 23-01
    provides: "provenance table, store interface (GetProvenance/ListProvenance), CreateProvenance/TouchProvenanceUpdatedAt"
provides:
  - "GET /projects/{slug}/provenance — list endpoint with ?entry= and ?user= filters"
  - "GET /projects/{slug}/provenance/{run_id} — detail endpoint with inputs, outputs, registry entry info"
  - "ProvenanceSummaryResponse and ProvenanceDetailResponse API types"
  - "Integration tests verifying list/filter/scope/detail/404 behavior"
affects: [provenance-consumers, audit-tooling, future-reporting-phases]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Provenance query handlers follow same projectFromContext + store call + response mapping pattern as registry handlers"
    - "JSON inputs/outputs fields use json.Unmarshal with omitempty — nil when run incomplete"

key-files:
  created:
    - internal/server/handlers_provenance.go
  modified:
    - internal/api/types.go
    - internal/server/server.go
    - internal/server/handlers_provenance_test.go

key-decisions:
  - "Provenance routes registered only in auth-enabled branch — no auth-disabled read path needed"
  - "handleGetProvenance maps store errors to 404 regardless of cause — keeps error surface minimal"
  - "Inputs/outputs fields in ProvenanceDetailResponse use omitempty — absent from JSON when run not yet complete"

patterns-established:
  - "Provenance read handlers: projectFromContext -> store query -> response mapping, consistent with registry pattern"

requirements-completed: [PROV-03, PROV-04]

# Metrics
duration: 3min
completed: 2026-03-20
---

# Phase 23 Plan 02: Provenance Query API Summary

**REST list and detail endpoints for provenance audit trail with entry/user filters, inputs/outputs unmarshaling, and project-scoped isolation**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-20T04:25:36Z
- **Completed:** 2026-03-20T04:28:46Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- GET /projects/{slug}/provenance returns summary list of provenance records with optional ?entry= and ?user= query filters
- GET /projects/{slug}/provenance/{run_id} returns full detail including registry entry name/kind, inputs (unmarshaled from run), and outputs (if run completed)
- Project scoping enforced at store layer — cross-project provenance leakage impossible
- 7 integration tests covering all required behaviors pass; full test suite green (22 packages)

## Task Commits

Each task was committed atomically:

1. **Task 1: API response types, provenance handlers, and route registration** - `ccaeec1` (feat)
2. **Task 2: Integration tests for provenance list and detail endpoints** - `8e10cba` (test)

**Plan metadata:** (docs commit below)

## Files Created/Modified
- `internal/api/types.go` - Added ProvenanceSummaryResponse and ProvenanceDetailResponse structs
- `internal/server/handlers_provenance.go` - handleListProvenance and handleGetProvenance handlers
- `internal/server/server.go` - Registered /provenance routes under /{slug} in auth-enabled branch
- `internal/server/handlers_provenance_test.go` - 7 new integration tests plus retained Plan 01 tests

## Decisions Made
- Provenance routes registered only in auth-enabled branch — no auth-disabled equivalent needed (provenance requires user identity)
- handleGetProvenance maps all store errors to 404 — keeps error surface minimal for callers
- Inputs/outputs fields in ProvenanceDetailResponse use `omitempty` — absent from JSON when run not yet complete, avoiding confusion about missing data vs empty data

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Provenance feature is complete: write path (Plan 01) and read path (Plan 02) both implemented and tested
- Phase 23 is fully complete — both PROV-01 through PROV-04 requirements satisfied
- Future reporting or audit tooling can consume the provenance API using viewer-level access

## Self-Check: PASSED

All files verified present. All task commits found in git history.

---
*Phase: 23-provenance-tracking*
*Completed: 2026-03-20*
