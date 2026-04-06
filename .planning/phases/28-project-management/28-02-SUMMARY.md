---
phase: 28-project-management
plan: "02"
subsystem: ui
tags: [react, typescript, shadcn-ui, react-query, zustand, react-router, vitest]

# Dependency graph
requires:
  - phase: 28-01
    provides: useProjects hooks (useProjectDetail, useProjectMembers, useUpdateProject, useDeleteProject, useUpdateMember, useRemoveMember), projectStore (useProjectStore, activeProjectSlug), useUsers accessible to all authenticated users
provides:
  - ProjectDetailPage with project info header, member management table, and 4 dialogs
  - AddMemberDialog with user Select dropdown populated from GET /users
  - Refactored Sidebar with functional project dropdown and project-scoped navigation
affects: [phase-29, phase-30, sidebar-consumers]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Project-level admin detection: currentMember?.role === 'admin' from members list, NOT global authStore.role"
    - "AlertDialog async pattern: e.preventDefault() blocks default close, manual onOpenChange(false) after await"
    - "Sidebar project nav guard: activeSlug && (...) prevents /projects/null/registry nav links"
    - "Self-row protection: member.username === currentUsername to disable role Select and hide action menu"

key-files:
  created:
    - web/src/features/projects/ProjectDetailPage.tsx
    - web/src/features/projects/AddMemberDialog.tsx
    - web/src/features/projects/__tests__/ProjectDetailPage.test.tsx
    - web/src/features/projects/__tests__/AddMemberDialog.test.tsx
  modified:
    - web/src/components/layout/Sidebar.tsx
    - web/src/App.tsx

key-decisions:
  - "Project admin detection uses members list (currentMember?.role === 'admin'), never global authStore.role — ensures project-scoped access control"
  - "AddMemberDialog always uses Select dropdown (no text-input fallback) — GET /users relaxed to all authenticated users in Plan 01"
  - "Sidebar project nav items guarded by activeSlug && to prevent null-slug URL generation"
  - "Overview nav item uses end=true to avoid matching all project sub-routes"

patterns-established:
  - "AlertDialog async pattern: e.preventDefault() + manual onOpenChange(false) after await (from Phase 27, confirmed in Phase 28)"
  - "Self-row protection: username equality check disables role controls and hides action menus"
  - "Project-scoped admin check from members list — not global role"

requirements-completed: [PROJ-03, PROJ-04]

# Metrics
duration: 4min
completed: 2026-04-06
---

# Phase 28 Plan 02: Project Detail Page and Sidebar Summary

**ProjectDetailPage with member CRUD management (inline role change, add/remove member, delete project, edit description), AddMemberDialog using user Select dropdown, and Sidebar refactored with functional project dropdown and project-scoped nav**

## Performance

- **Duration:** 4 min
- **Started:** 2026-04-06T08:38:46Z
- **Completed:** 2026-04-06T08:43:05Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments

- Built ProjectDetailPage with project info header, project-level admin detection from members list, member table with inline role Select, self-row protection, and all 4 dialogs (add member, remove member AlertDialog, delete project AlertDialog, edit description Dialog)
- Built AddMemberDialog with user Select dropdown fetched from GET /users (accessible to all authenticated users per Plan 01), filtering out existing members from dropdown options
- Refactored Sidebar with functional project dropdown (Select from useProjects), project-scoped nav items guarded by activeSlug null check, removing all hardcoded /registry /runs /members paths
- Full test coverage: 13 new tests across ProjectDetailPage and AddMemberDialog, all 83 tests pass

## Task Commits

Each task was committed atomically:

1. **Task 1: ProjectDetailPage, AddMemberDialog, App.tsx update** - `47fc65b` (feat)
2. **Task 2: Sidebar refactor — project dropdown and project-scoped nav** - `cd36825` (feat)

**Plan metadata:** (pending — this commit)

## Files Created/Modified

- `web/src/features/projects/ProjectDetailPage.tsx` - Project detail with info header, member table, inline role change, 4 dialogs
- `web/src/features/projects/AddMemberDialog.tsx` - Add member dialog with user Select dropdown from GET /users
- `web/src/features/projects/__tests__/ProjectDetailPage.test.tsx` - 8 tests covering admin visibility, self-protection, loading states
- `web/src/features/projects/__tests__/AddMemberDialog.test.tsx` - 5 tests covering rendering, user dropdown, member filtering
- `web/src/components/layout/Sidebar.tsx` - Functional project dropdown + project-scoped nav, removed static projectNavItems
- `web/src/App.tsx` - Replaced placeholder ProjectDetailPage component with real import

## Decisions Made

- Project admin detection uses `currentMember?.role === 'admin'` from the members list, not `authStore.role === 'admin'`. This enables project-scoped access control where a non-global-admin user can be a project admin.
- AddMemberDialog always uses `<Select>` dropdown (no text-input fallback for non-admins). GET /users was relaxed in Plan 01 to be accessible to all authenticated users, removing the need for a conditional text input.
- Sidebar project nav guard `activeSlug &&` prevents `/projects/null/registry` or `/projects/undefined/registry` being generated when no project is selected.
- Overview nav item in project-scoped nav uses `end={true}` to avoid matching on all `/projects/{slug}/*` sub-routes simultaneously.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- AddMemberDialog tests initially used `getByText('Add Member')` and `getByText('viewer')` which failed because multiple elements matched (dialog title + submit button, Select display value + hidden option element). Fixed by switching to `getAllByText` with existence and length assertions.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- ProjectDetailPage is complete and functional at `/projects/:slug`
- Sidebar project dropdown provides navigation to project-scoped routes (/registry, /runs, /members)
- Phase 29 can build project registry and run views using the existing project nav infrastructure
- Phase 30 file browsing can extend the established pattern for project-scoped pages

---
*Phase: 28-project-management*
*Completed: 2026-04-06*
