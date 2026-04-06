---
phase: 28-project-management
plan: 01
subsystem: ui
tags: [react, zustand, react-query, shadcn, typescript, project-management]

# Dependency graph
requires:
  - phase: 26-authentication
    provides: JWT auth, useAuthStore, apiClient, form patterns
  - phase: 27-user-management
    provides: UsersPage patterns, shadcn component install flow, test patterns
provides:
  - useProjectStore (Zustand store, sessionStorage, 'flowy-project' key)
  - useProjects hook suite (9 hooks + ProjectResponse + MemberResponse interfaces)
  - ProjectsPage at /projects (card grid, loading/empty states, admin-only CTA)
  - CreateProjectPage at /projects/new (name/slug/description form, auto-slug generation)
  - GET /users accessible to any authenticated user (not admin-only)
  - textarea.tsx shadcn component
  - App.tsx routes: /projects, /projects/new, /projects/:slug
affects: [28-project-management-plan-02, any phase using project data hooks]

# Tech tracking
tech-stack:
  added: [shadcn textarea component]
  patterns: [Zustand selector mocking with mockImplementation for tests, useProjectStore selector pattern]

key-files:
  created:
    - web/src/store/projectStore.ts
    - web/src/features/projects/useProjects.ts
    - web/src/features/projects/ProjectsPage.tsx
    - web/src/features/projects/CreateProjectPage.tsx
    - web/src/features/projects/__tests__/useProjects.test.ts
    - web/src/features/projects/__tests__/ProjectsPage.test.tsx
    - web/src/features/projects/__tests__/CreateProjectPage.test.tsx
    - web/src/components/ui/textarea.tsx
  modified:
    - internal/server/server.go
    - internal/server/handlers_auth_test.go
    - web/src/App.tsx

key-decisions:
  - "GET /users relaxed from admin-only to any-authenticated-user to unblock Plan 02 Add Member dropdown"
  - "Zustand selector mocks use mockImplementation to handle (s) => s.field call pattern"
  - "No member_count on project cards — ProjectResponse has no such field, avoids N+1 queries"
  - "slugManuallyEdited flag stops auto-generation once user directly edits slug field"

patterns-established:
  - "useProjectStore selector mock: vi.mocked(useProjectStore).mockImplementation((selector) => selector(state))"
  - "Slug auto-generation: generateSlug() lowercases, trims, replaces spaces with hyphens, strips non-alphanumeric"

requirements-completed: [PROJ-01, PROJ-02]

# Metrics
duration: 5min
completed: 2026-04-06
---

# Phase 28 Plan 01: Project Management Data Layer and Pages Summary

**Zustand projectStore + 9 React Query project hooks + ProjectsPage card grid + CreateProjectPage with slug auto-generation, wired into App.tsx routes**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-04-06T08:32:00Z
- **Completed:** 2026-04-06T08:36:40Z
- **Tasks:** 3 (Task 0 + Task 1 + Task 2)
- **Files modified:** 11

## Accomplishments
- Relaxed GET /users from admin-only to any-authenticated-user with new Go test
- Built full project data layer: projectStore (Zustand + sessionStorage), 9 React Query hooks (useProjects, useProjectDetail, useProjectMembers, useCreateProject, useUpdateProject, useDeleteProject, useAddMember, useUpdateMember, useRemoveMember), ProjectResponse and MemberResponse TypeScript interfaces
- Delivered ProjectsPage (responsive card grid with loading skeletons, empty state, admin-only Create button) and CreateProjectPage (name/slug/description form with auto-slug generation, submit sends only slug+description to API)
- Added textarea shadcn component and wired /projects, /projects/new, /projects/:slug routes in App.tsx

## Task Commits

Each task was committed atomically:

1. **Task 0: Relax GET /users** - `b014a02` (feat)
2. **Task 1: Project data layer** - `63972e4` (feat)
3. **Task 2: Pages + routes** - `8433f22` (feat)

## Files Created/Modified
- `internal/server/server.go` - Moved GET /users out of admin-only middleware group
- `internal/server/handlers_auth_test.go` - Added TestHandleListUsers_NonAdmin
- `web/src/store/projectStore.ts` - Zustand store with sessionStorage persistence
- `web/src/features/projects/useProjects.ts` - 9 React Query hooks + 2 interfaces
- `web/src/features/projects/ProjectsPage.tsx` - Card grid page at /projects
- `web/src/features/projects/CreateProjectPage.tsx` - Create form at /projects/new
- `web/src/features/projects/__tests__/useProjects.test.ts` - 9 hook tests
- `web/src/features/projects/__tests__/ProjectsPage.test.tsx` - 6 page tests
- `web/src/features/projects/__tests__/CreateProjectPage.test.tsx` - 3 form tests
- `web/src/components/ui/textarea.tsx` - shadcn Textarea component
- `web/src/App.tsx` - Added 3 project routes + ProjectsPage/CreateProjectPage imports

## Decisions Made
- GET /users relaxed from admin-only to any-authenticated-user — required for Plan 02's Add Member dropdown where project admins (not global admins) need to search users
- No member_count on project cards — ProjectResponse intentionally has no member_count field; showing member count would require N+1 queries per card, accepted trade-off per CONTEXT.md
- slugManuallyEdited flag: once the user directly edits the slug field, auto-generation from name stops, giving full control
- Zustand selector mock pattern established: `mockImplementation((selector) => typeof selector === 'function' ? selector(state) : state)` to handle `useStore((s) => s.field)` call pattern in tests

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered
- Zustand selector mocking: initial test used `mockReturnValue(wholeObject)` but ProjectsPage calls `useProjectStore((s) => s.setActiveProject)` as a selector. Fixed by using `mockImplementation` to handle the selector function call. No code change to implementation required — only test mock strategy adjusted.

## User Setup Required
None — no external service configuration required.

## Next Phase Readiness
- Plan 02 (project detail, member management) can import all hooks from useProjects.ts
- useProjectStore provides activeProjectSlug for project context
- /projects/:slug route exists with placeholder — Plan 02 replaces the placeholder with ProjectDetailPage
- GET /users is now accessible to any authenticated user, enabling Add Member search dropdown

## Self-Check: PASSED

All files created:
- FOUND: web/src/store/projectStore.ts
- FOUND: web/src/features/projects/useProjects.ts
- FOUND: web/src/features/projects/ProjectsPage.tsx
- FOUND: web/src/features/projects/CreateProjectPage.tsx
- FOUND: web/src/components/ui/textarea.tsx

All commits present:
- FOUND: b014a02 (Task 0)
- FOUND: 63972e4 (Task 1)
- FOUND: 8433f22 (Task 2)

---
*Phase: 28-project-management*
*Completed: 2026-04-06*
