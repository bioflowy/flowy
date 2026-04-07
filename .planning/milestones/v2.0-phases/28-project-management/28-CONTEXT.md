# Phase 28: Project Management - Context

**Gathered:** 2026-04-06
**Status:** Ready for planning

<domain>
## Phase Boundary

Users can view the projects they belong to, create new projects (global admins only), and admins can manage project membership and role assignments through a project detail screen. Backend endpoints already exist — this phase builds the frontend UI and wires it to existing API. One backend change: relax GET /users from admin-only to any-authenticated-user so Add Member dropdown works for all project admins.

</domain>

<decisions>
## Implementation Decisions

### Project list display
- Card grid layout on `/projects` page — responsive grid of project cards
- Each card shows: project name (title), slug (subtitle), description snippet (2 lines max) — no member count (ProjectResponse lacks member_count field; accepted trade-off to avoid N+1)
- Clicking a card sets it as the active project in the sidebar dropdown AND navigates to `/projects/{slug}`
- Default landing page after login remains `/projects` — user picks a project from the grid
- Sidebar has a project dropdown selector at top; selecting a project shows project-scoped nav items (Registry, Runs, Members) below it
- When no project is selected, sidebar shows only global nav (Users for admins) and the project dropdown with placeholder

### Project detail page
- Single scrollable page at `/projects/{slug}` with two sections: project info header + member list table
- Header shows: project name, slug, description, "Edit Description" and "Delete Project" buttons (visible to project admins only)
- Member table below header with columns: Username, Role (inline Select dropdown), Joined date, actions (three-dot menu)
- "Add Member" button above the member table
- URL structure: `/projects/{slug}` for detail, future sub-pages at `/projects/{slug}/registry`, `/projects/{slug}/runs`
- Project admins can edit description and delete the project (not limited to global admins)

### Member management
- Add member: Dialog overlay with user dropdown (all active users not yet in project) + role select (admin/executor/viewer, default viewer) + Add button. Backend change: relax GET /users from admin-only to any authenticated user so the dropdown works for all project admins.
- Role change: Inline Select dropdown in role column — changing immediately updates via API + success toast (no dialog)
- Remove member: Three-dot dropdown on row with "Remove" option — confirmation dialog "Remove {username} from {project}?" with destructive red button
- Self-protection: hide three-dot menu and disable role dropdown for the logged-in user's own row (same pattern as Phase 27 users table)

### Project creation
- Dedicated `/projects/new` page (not a dialog) — "Create Project" button on the project list page navigates here
- Fields: Name (required), Slug (auto-generated from name, manually overridable), Description (optional textarea)
- Slug auto-generation: lowercase name with spaces replaced by hyphens, user can edit the slug field manually
- Only global admins can create projects (show "Create Project" button only when `role === 'admin'`)
- Creator is automatically added as project admin (existing backend behavior)
- After creation: navigate to `/projects/{slug}` and set as active project in sidebar

### Claude's Discretion
- Empty state for project list (no projects yet)
- Loading skeletons for card grid and member table
- Project card hover/click styling
- Exact badge colors for roles in member table
- Delete project confirmation dialog wording
- Edit description UX (inline edit vs dialog)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Backend project API (existing — all endpoints ready)
- `internal/server/handlers_projects.go` — Project CRUD: POST /projects, GET /projects, GET /projects/{slug}, PUT /projects/{slug}, DELETE /projects/{slug}
- `internal/server/handlers_projects.go` — Member management: POST /projects/{slug}/members, GET /projects/{slug}/members, PUT /projects/{slug}/members/{id}, DELETE /projects/{slug}/members/{id}
- `internal/api/types.go` — `CreateProjectRequest`, `UpdateProjectRequest`, `ProjectResponse`, `AddMemberRequest`, `UpdateMemberRequest`, `MemberResponse`
- `internal/server/server.go` — Route registration with projectRoleRequired middleware

### Backend project store
- `internal/store/sqlite/projects.go` — Project DB operations (CreateProject, ListProjectsForUser, ListAllProjects, GetProjectBySlug, UpdateProject, DeactivateProject)
- `internal/store/store.go` — Project store interface

### Frontend foundation (Phase 26-27 patterns)
- `web/src/store/authStore.ts` — Auth store with token, username, role for admin gating
- `web/src/lib/apiClient.ts` — API client with get/post/put/delete/patch + Bearer token
- `web/src/features/users/useUsers.ts` — React Query hooks pattern to replicate for projects/members
- `web/src/features/users/UsersPage.tsx` — Table + badge + dropdown menu pattern
- `web/src/features/users/CreateUserDialog.tsx` — Dialog form pattern (react-hook-form + zod)
- `web/src/components/layout/Sidebar.tsx` — Navigation structure with admin-only gating
- `web/src/App.tsx` — Route structure inside ProtectedRoute

### UI components available
- `web/src/components/ui/` — table, badge, dropdown-menu, select, alert-dialog, dialog, button, card, form, input, label, separator, tooltip

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- shadcn/ui Card component: use for project cards in grid
- shadcn/ui Table, Badge, DropdownMenu, Select, AlertDialog: reuse Phase 27 patterns for member table
- React Query hooks pattern from `useUsers.ts`: replicate for `useProjects.ts` and `useProjectMembers.ts`
- Dialog + react-hook-form + zod pattern: reuse for Add Member dialog
- Sonner toast: already configured for success notifications
- apiClient with all HTTP methods: ready for project/member API calls

### Established Patterns
- Feature folder structure: `web/src/features/{feature}/` with hooks, page, dialog components
- Admin-only gating: `authStore.role === 'admin'` for conditional rendering (sidebar, buttons)
- Table rows: three-dot dropdown for actions, self-row protection by comparing current user
- Form validation: zod schemas with react-hook-form
- React Query invalidation on mutation success

### Integration Points
- Sidebar.tsx: add project dropdown selector, project-scoped nav items (Registry, Runs, Members)
- App.tsx: add `/projects/{slug}` and `/projects/new` routes inside ProtectedRoute
- Need project context state (active project) — likely a zustand store or React context for sidebar dropdown

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 28-project-management*
*Context gathered: 2026-04-06*
