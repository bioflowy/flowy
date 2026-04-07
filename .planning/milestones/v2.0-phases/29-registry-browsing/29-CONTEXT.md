# Phase 29: Registry Browsing - Context

**Gathered:** 2026-04-07
**Status:** Ready for planning

<domain>
## Phase Boundary

Users can browse WDL registry entries within a project, view version history with tags, view WDL source code of any version, and admins can create/move/delete tags on registry entries. All backend API endpoints already exist — this phase builds the frontend UI and wires it to existing API.

</domain>

<decisions>
## Implementation Decisions

### Entry list display
- Table layout on `/projects/{slug}/registry` — consistent with Phase 27 (Users) and Phase 28 (Members) table pattern
- Columns: Name, Kind (colored badge — blue for tool, purple for workflow), Versions (count), Tags (comma-separated text, dash when none), Created date
- Clicking a row navigates to `/projects/{slug}/registry/{name}` detail page — consistent with project list → project detail navigation pattern
- Admin-only row actions: three-dot dropdown menu with "Delete" option + confirmation dialog (AlertDialog, destructive red button)
- No "push" or "create" action in the UI — registry entries are created via CLI push only

### Version history view
- Claude's Discretion — table on the entry detail page showing versions with digest (truncated), tag badges, created date
- Default: show all versions sorted by newest first

### WDL source viewer
- Claude's Discretion — read-only code block with syntax highlighting, copy button, and line numbers
- Displayed on the entry detail page when a version is selected (or latest by default)

### Tag management UX
- Claude's Discretion — admin-only actions for creating/moving/deleting tags
- Delete tag: confirmation dialog (destructive action pattern from Phase 27)
- Create/move tag: dialog with tag name input + version digest selector

### Claude's Discretion
- Loading/empty states for registry entry table and version list
- Exact badge colors for kind (tool/workflow)
- Entry detail page layout (version table + source viewer arrangement)
- Syntax highlighting library choice for WDL source
- Tag badge styling in version table
- Pagination or scroll strategy if entry/version count grows

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Backend registry API (all endpoints exist)
- `internal/server/handlers_registry.go` — All registry handlers: list entries (GET /projects/{slug}/registry), list versions (GET /projects/{slug}/registry/{name}/versions), pull content (GET /projects/{slug}/registry/{name}?tag= or ?digest=), push (POST), tag (POST /projects/{slug}/registry/{name}/tags), delete tag (DELETE /projects/{slug}/registry/{name}/tags/{tag}), delete entry (DELETE /projects/{slug}/registry/{name})
- `internal/api/types.go` lines 263-312 — `RegistryEntryResponse` (name, kind, version_count, tags[], created_at), `RegistryVersionResponse` (id, digest, tags[], created_at), `RegistryPullResponse` (name, digest, kind, content), `RegistryTagRequest` (tag, digest)
- `internal/server/server.go` — Route registration with projectRoleRequired middleware (viewer for list/pull, admin for tag/delete)

### Frontend patterns (Phase 26-28)
- `web/src/features/users/UsersPage.tsx` — Table + badge + dropdown menu pattern to replicate
- `web/src/features/users/useUsers.ts` — React Query hooks pattern to replicate for registry
- `web/src/features/projects/ProjectDetailPage.tsx` — Detail page with member table pattern
- `web/src/lib/apiClient.ts` — API client with get/post/put/delete/patch + Bearer token
- `web/src/components/layout/Sidebar.tsx` — Already has Registry nav link at `/projects/{slug}/registry`

### UI components available
- `web/src/components/ui/` — table, badge, dropdown-menu, alert-dialog, dialog, button, card, form, input, label, select, separator, tooltip

### Route structure
- `web/src/App.tsx` — Add routes for `/projects/:slug/registry` and `/projects/:slug/registry/:name` inside ProtectedRoute

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- shadcn/ui Table, Badge, DropdownMenu, AlertDialog: reuse Phase 27/28 patterns for entry table and admin actions
- React Query hooks pattern from `useUsers.ts` and `useProjects.ts`: replicate for `useRegistry.ts`
- Dialog + react-hook-form + zod pattern: reuse for tag creation dialog
- Sonner toast: already configured for success notifications
- apiClient with all HTTP methods: ready for registry API calls

### Established Patterns
- Feature folder structure: `web/src/features/registry/` with hooks, pages, dialogs
- Table rows: three-dot dropdown for admin actions, Badge for categorical data
- Navigation: list page with clickable rows → detail page (projects pattern)
- React Query invalidation on mutation success
- Admin-only gating: check project member role for tag/delete actions

### Integration Points
- App.tsx: add `/projects/:slug/registry` and `/projects/:slug/registry/:name` routes inside ProtectedRoute
- Sidebar.tsx: Registry nav link already exists at `/projects/{slug}/registry` — no changes needed
- Project context: use `useProjectStore` activeSlug for API calls scoped to current project
- Project member role: use project members API to determine if current user is admin (Phase 28 pattern for project-scoped admin detection)

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

*Phase: 29-registry-browsing*
*Context gathered: 2026-04-07*
