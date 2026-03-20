---
phase: 21-wdl-registry
plan: "03"
subsystem: api
tags: [registry, wdl, tag-management, list, delete, go, sqlite]

requires:
  - phase: 21-wdl-registry plan 01
    provides: three-table registry schema and store interface (registry_entries/registry_versions/registry_tags)
  - phase: 21-wdl-registry plan 02
    provides: push/pull handlers, stub handlers, API types (RegistryTagRequest/RegistryEntryResponse/RegistryVersionResponse)

provides:
  - Full WDL registry API: tag creation/reassignment/deletion, entry list, version list, entry soft-delete
  - Comprehensive tests covering all five new endpoints with RBAC enforcement

affects:
  - phase 22 and beyond: registry API is now fully operational for client integration

tech-stack:
  added: []
  patterns:
    - Forbidden tests use separate admin and executor users with distinct project roles to properly exercise RBAC
    - Nil-safe Tags field: store slices may be nil; handlers coerce to []string{} before JSON serialization to avoid null
    - 204 No Content returned for both delete operations (entry and tag) with no response body

key-files:
  created: []
  modified:
    - internal/server/handlers_registry.go
    - internal/server/handlers_registry_test.go

key-decisions:
  - "Forbidden tests require two distinct users: one admin (project creator) and one executor (project member); createRegistryTestProject hardcodes admin role, so a separate addProjectMember helper was added for executor membership setup"
  - "ListRegistryVersions returns 404 on any store error — consistent with treating missing entry as not-found rather than exposing internal errors"

patterns-established:
  - "addProjectMember helper: adds a user to a project with a specified role directly via store, used in RBAC tests where the executor should NOT have admin access"
  - "pushWDL helper: DRY push wrapper used across all new tag/list/delete tests to reduce boilerplate"

requirements-completed: [REG-02, REG-04, REG-06]

duration: 4min
completed: 2026-03-20
---

# Phase 21 Plan 03: WDL Registry Tag, List, and Delete Summary

**Full WDL registry API: tag assignment/reassignment/deletion, entry listing with version counts and tag names, version listing with tags, and admin-only entry soft-delete — replacing all five stubs with tested implementations**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-20T01:52:56Z
- **Completed:** 2026-03-20T01:56:28Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- Replaced all five stub handlers (returning 501) with full implementations in `handlers_registry.go`
- handleRegistryTag: validates tag name (1-128 chars, no @/:), looks up entry and version by digest, calls UpsertRegistryTag; returns 200 with tag/digest/entry
- handleRegistryDeleteTag: looks up entry, removes tag without affecting version; returns 204
- handleRegistryList/handleRegistryListVersions: map store list items to API response types with nil-safe []string{} tags; return JSON arrays
- handleRegistryDelete: soft-deletes entry; returns 204
- 9 new test functions cover tag create/reassign/forbidden/invalid-name, tag delete, entry list, version list, entry delete, and delete forbidden

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement tag, list, and delete handlers** - `0052e90` (feat)
2. **Task 2: Add tests for tag, list, and delete endpoints** - `d6e6322` (test)

## Files Created/Modified

- `/home/uehara/flowy/internal/server/handlers_registry.go` - Five stub handlers replaced with full implementations
- `/home/uehara/flowy/internal/server/handlers_registry_test.go` - 9 new test functions + addProjectMember/pushWDL helpers + sampleToolWDLv2 constant

## Decisions Made

- Forbidden tests require two distinct users: admin (project creator) and executor (project member with executor role). The existing `createRegistryTestProject` helper hardcodes "admin" project role, so a new `addProjectMember(t, st, projID, userID, role)` helper was added to attach an executor to the project at the correct role level.
- `ListRegistryVersions` returns 404 on any store error — treats a missing entry as not-found rather than exposing internal errors to clients.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Test setup produced wrong RBAC: executor was inadvertently project admin**

- **Found during:** Task 2 (TestRegistryTag_Forbidden, TestRegistryDelete_Forbidden)
- **Issue:** The plan said "Create user with executor role" and use createRegistryTestProject, but that helper always adds the user as project admin regardless of global role. A project admin bypasses executor restrictions so the tests got 200 instead of 403.
- **Fix:** Rewrote the two forbidden tests to use separate admin and executor users. Admin creates/owns the project; executor is added via new `addProjectMember` helper with role "executor". Added `addProjectMember` helper to test file.
- **Files modified:** internal/server/handlers_registry_test.go
- **Verification:** TestRegistryTag_Forbidden and TestRegistryDelete_Forbidden both return 403 as expected
- **Committed in:** d6e6322 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 - Bug)
**Impact on plan:** Fix was essential for test correctness. No scope creep.

## Issues Encountered

None — after the RBAC test setup fix, all tests passed on the second run.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- WDL registry API is fully implemented and tested (push, pull, tag, list, delete)
- Phase 21 is complete: all three plans (schema + store, push/pull handlers, tag/list/delete handlers) are done
- Phase 22 can proceed with registry client integration or any downstream feature

---
*Phase: 21-wdl-registry*
*Completed: 2026-03-20*
