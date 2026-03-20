# Phase 20: Projects and RBAC - Context

**Gathered:** 2026-03-17
**Status:** Ready for planning

<domain>
## Phase Boundary

Add project workspaces to the flowy-server REST API. Users can create isolated projects, manage membership with role-based access control (admin/executor/viewer), and all workflow runs are scoped to a project. When auth is disabled, existing flat routing and behavior are preserved.

</domain>

<decisions>
## Implementation Decisions

### Project lifecycle
- Any authenticated user can create a project (not restricted to global admins)
- Creator is automatically added as project admin
- Project names are slug-style: lowercase alphanumeric + hyphens, max 64 chars (used in API paths and registry references like `project/name:tag`)
- Soft delete for projects (mark as archived/inactive, data preserved) — consistent with Phase 19 user soft-delete pattern

### Role permissions
- Three project-level roles: admin, executor, viewer (decided in PROJECT.md)
- Admin: full project control (update, delete, manage members, all executor+viewer permissions)
- Executor: submit workflow runs, abort any run in the project, full read access
- Viewer: full read access (project details, run status, run outputs, member list) — cannot modify anything
- RBAC middleware as chi per-route-group middleware (~50 lines as planned in PROJECT.md), not per-handler checks

### Run scoping
- `project_id` is NOT NULL on runs table — every run must belong to a project
- Existing run data may be dropped in migration (no backward compat needed for old runs)
- When `auth.enabled=true`: all run endpoints move under `/projects/{slug}/runs/...` (POST submit, GET status, POST abort, etc.)
- When `auth.enabled=false`: old `/runs/...` paths still work (backward compat for spectool tests); project_id not enforced
- Project specified via URL path prefix: `POST /projects/{slug}/runs/submit`

### Membership model
- Users can belong to multiple projects simultaneously with different roles in each
- project_members join table: (project_id, user_id, role, created_at)
- Global admins have implicit access to all projects without explicit membership
- Members added directly by project admin: `POST /projects/{slug}/members` with username + role
- User must already exist (no invitation flow)
- Last-admin protection: cannot remove the last admin from a project to prevent orphan projects

### Claude's Discretion
- Project table fields beyond id, slug, description, is_active, created_at, updated_at
- Exact RBAC middleware implementation approach (context injection vs per-group)
- Migration strategy for dropping existing runs (separate migration or combined with project table creation)
- API response shapes for project and membership endpoints

</decisions>

<specifics>
## Specific Ideas

- Registry references will use project slug: `project/name:tag` (Phase 21 depends on this)
- Run endpoints under `/projects/{slug}/runs/...` when auth enabled — RESTful hierarchy
- Docker entrypoint friendliness: project creation via API enables scripted setup

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `JWTMiddleware` + `DaemonAuthMiddleware` + `roleRequired` in `internal/server/middleware.go` — extend with project-role middleware
- `Store` interface in `internal/store/store.go` — add project and membership method groups
- `User` model with `GlobalRole` field — used for global admin implicit access checks
- `AuthConfig.Enabled` gate in `newRouter()` — extend with conditional project-scoped routing
- goose migrations in `internal/store/sqlite/migrations/` — next is `00005`

### Established Patterns
- chi route groups with middleware for auth scoping (Phase 19) — replicate for project-role scoping
- Soft delete via `is_active` boolean (users pattern) — reuse for projects
- UUID primary keys (users, runs, tasks) — use for projects and memberships
- `ErrorResponse` JSON struct for error responses

### Integration Points
- `server.go:newRouter()` — add `/projects/{slug}/...` route group with project membership middleware
- `store.go:Store` — add CreateProject, GetProject, ListProjects, UpdateProject, DeactivateProject, AddMember, RemoveMember, GetMember, ListMembers, etc.
- `runs` table — add project_id FK column (NOT NULL), drop existing data in migration
- `api/types.go` — add Project, Membership, and related request/response types

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 20-projects-and-rbac*
*Context gathered: 2026-03-17*
