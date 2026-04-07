---
phase: 29-registry-browsing
plan: 01
subsystem: ui
tags: [react, react-query, tanstack-query, typescript, vitest, shadcn, table, badge, registry]

# Dependency graph
requires:
  - phase: 28-project-detail
    provides: useProjectMembers hook for admin detection, AlertDialog pattern, dropdown menu pattern
  - phase: 27-users-management
    provides: UsersPage table pattern, DropdownMenu pattern, skeleton loading rows
provides:
  - useRegistry.ts with 6 React Query hooks (useRegistryEntries, useRegistryVersions, useRegistryContent, useDeleteRegistryEntry, useUpsertTag, useDeleteTag) and 3 TypeScript interfaces
  - RegistryPage.tsx list page with kind badges, tags, admin delete dropdown, empty/loading/error states
  - RegistryEntryPage.tsx placeholder for Plan 02
  - Routes /projects/:slug/registry and /projects/:slug/registry/:name in App.tsx
affects: [29-02-registry-entry-detail, any future plan consuming registry hooks]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Registry list page uses same table + skeleton + dropdown pattern as UsersPage and ProjectDetailPage members table
    - Admin detection via members list (project-scoped role), same as Phase 28 pattern
    - Kind badges use variant="outline" with custom color className (purple for workflow, blue for tool)
    - Delete flow: AlertDialog with Keep Entry / Delete buttons, mutateAsync with loading state

key-files:
  created:
    - web/src/features/registry/useRegistry.ts
    - web/src/features/registry/RegistryPage.tsx
    - web/src/features/registry/__tests__/RegistryPage.test.tsx
    - web/src/features/registry/RegistryEntryPage.tsx
  modified:
    - web/src/App.tsx

key-decisions:
  - "RegistryEntryPage placeholder created in Plan 01 to satisfy App.tsx import; full implementation deferred to Plan 02"
  - "Admin detection reuses project-members list pattern from Phase 28 (not global role)"

patterns-established:
  - "Kind badge pattern: Badge variant=outline with className conditional on kind value (blue tool / purple workflow)"

requirements-completed: [REG-01]

# Metrics
duration: 5min
completed: 2026-04-07
---

# Phase 29 Plan 01: Registry Data Layer and List Page Summary

**React Query hooks for all registry API endpoints plus a sortable table list page with kind badges, admin delete flow, and full test coverage**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-04-07T02:11:51Z
- **Completed:** 2026-04-07T02:13:51Z
- **Tasks:** 2
- **Files modified:** 5 (3 created + 1 placeholder + 1 modified)

## Accomplishments

- Created `useRegistry.ts` with all 6 React Query hooks and 3 TypeScript interfaces covering every registry API endpoint
- Built `RegistryPage.tsx` table with kind badges (blue tool / purple workflow), tag text, version count, admin-only three-dot delete dropdown, and all render states (loading skeleton, empty, error)
- Wired `/projects/:slug/registry` and `/projects/:slug/registry/:name` routes in `App.tsx`; added `RegistryEntryPage` placeholder for Plan 02
- All 89 tests pass; TypeScript compiles clean

## Task Commits

1. **Task 1: Create useRegistry.ts hooks and RegistryPage.tsx list page with tests** - `58baa5f` (feat)
2. **Task 2: Wire registry routes in App.tsx** - `9b5f68d` (feat)

## Files Created/Modified

- `web/src/features/registry/useRegistry.ts` - All React Query hooks and TypeScript interfaces for registry API
- `web/src/features/registry/RegistryPage.tsx` - Registry list page with table, kind badges, admin dropdown, AlertDialog delete flow
- `web/src/features/registry/__tests__/RegistryPage.test.tsx` - 6 unit tests covering all render states, navigation, admin visibility
- `web/src/features/registry/RegistryEntryPage.tsx` - Placeholder component for Plan 02
- `web/src/App.tsx` - Added two registry routes and imports

## Decisions Made

- `RegistryEntryPage.tsx` placeholder created in Plan 01 to unblock the App.tsx import; full implementation is Plan 02's scope.
- Admin detection reuses the project-members list pattern from Phase 28 (`members?.find(m => m.username === currentUsername)?.role === 'admin'`) — not the global `authStore.role`.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Plan 02 can immediately import `useRegistryVersions`, `useRegistryContent`, `useUpsertTag`, `useDeleteTag` from `useRegistry.ts`
- `RegistryEntryPage` placeholder is wired at `/projects/:slug/registry/:name` and ready for implementation
- No blockers.

---
*Phase: 29-registry-browsing*
*Completed: 2026-04-07*
