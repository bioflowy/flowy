# Phase 20: Projects and RBAC - Research

**Researched:** 2026-03-17
**Domain:** Go REST API — multi-tenant project workspaces, role-based access control, schema migration, chi middleware
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Project lifecycle:**
- Any authenticated user can create a project (not restricted to global admins)
- Creator is automatically added as project admin
- Project names are slug-style: lowercase alphanumeric + hyphens, max 64 chars
- Soft delete for projects (mark as archived/inactive, is_active boolean) — consistent with Phase 19 user soft-delete pattern

**Role permissions:**
- Three project-level roles: admin, executor, viewer
- Admin: full project control (update, delete, manage members, all executor+viewer permissions)
- Executor: submit workflow runs, abort any run in the project, full read access
- Viewer: full read access (project details, run status, run outputs, member list) — cannot modify anything
- RBAC middleware as chi per-route-group middleware (~50 lines), not per-handler checks

**Run scoping:**
- `project_id` is NOT NULL on runs table — every run must belong to a project
- Existing run data may be dropped in migration (no backward compat needed for old runs)
- When `auth.enabled=true`: all run endpoints move under `/projects/{slug}/runs/...`
- When `auth.enabled=false`: old `/runs/...` paths still work; project_id not enforced
- Project specified via URL path prefix: `POST /projects/{slug}/runs/submit`

**Membership model:**
- Users can belong to multiple projects simultaneously with different roles in each
- `project_members` join table: (project_id, user_id, role, created_at)
- Global admins have implicit access to all projects without explicit membership
- Members added directly by project admin: `POST /projects/{slug}/members` with username + role
- User must already exist (no invitation flow)
- Last-admin protection: cannot remove the last admin from a project to prevent orphan projects

### Claude's Discretion
- Project table fields beyond id, slug, description, is_active, created_at, updated_at
- Exact RBAC middleware implementation approach (context injection vs per-group)
- Migration strategy for dropping existing runs (separate migration or combined with project table creation)
- API response shapes for project and membership endpoints

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| PROJ-01 | User can create a project with name and description | Store.CreateProject + POST /projects handler |
| PROJ-02 | User can list projects they are a member of | Store.ListProjectsForUser + GET /projects handler; global admin sees all |
| PROJ-03 | User can view project details | Store.GetProjectBySlug + GET /projects/{slug} handler |
| PROJ-04 | Project admin can update or delete a project | Store.UpdateProject + Store.DeactivateProject + RBAC middleware enforcing admin role |
| PROJ-05 | Project admin can add/remove members with role | Store.AddMember + Store.RemoveMember + POST/DELETE /projects/{slug}/members/{username} |
| PROJ-06 | Project admin can change a member's role | Store.UpdateMemberRole + PUT /projects/{slug}/members/{username} |
| PROJ-07 | Workflow runs are scoped to a project (project_id on runs table) | Migration 00005 adds project_id column; run submission under /projects/{slug}/runs/submit |
</phase_requirements>

---

## Summary

Phase 20 extends the flowy-server REST API with project workspaces and per-project RBAC. The implementation is purely additive in the data layer (two new tables: `projects`, `project_members`) with one modification to the existing `runs` table (add `project_id` NOT NULL). No new Go dependencies are required — all building blocks are already present: chi v5 route groups, goose migrations, SQLite WAL store, JWT claims in context.

The RBAC middleware follows the same chi middleware pattern established in Phase 19 (`roleRequired`). The project-role middleware needs two things from context: the authenticated user's claims (from JWT) and the resolved project membership record (injected by a `projectContext` middleware that runs first). Global admins bypass project membership checks entirely.

The auth-disabled path (`auth.enabled=false`) must keep the old `/workflows/...` and `/tasks/...` routes untouched to preserve spectool compatibility. In auth-disabled mode, run submission uses dummy project handling or skips project_id entirely in SQL (via `NULL` with a relaxed NOT NULL constraint gated on auth mode, or by using a sentinel project).

**Primary recommendation:** Implement in four sequential waves: (1) migrations + store layer, (2) project CRUD handlers, (3) membership handlers + RBAC middleware, (4) run routing under project scope.

---

## Standard Stack

### Core (already in go.mod — no new dependencies needed)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/go-chi/chi/v5 | v5.2.5 | HTTP routing + middleware groups | Already in use; `r.Route` + `r.Use` is the per-group middleware pattern |
| modernc.org/sqlite | v1.46.1 | SQLite persistence | Already in use via SQLiteStore |
| github.com/pressly/goose/v3 | v3.27.0 | SQL migrations | Already wired in `sqlite.go`; next migration is 00005 |
| github.com/google/uuid | v1.6.0 | UUID primary keys | Already in use for run/user IDs |

### No New Dependencies

All required functionality is available in the existing stack. RBAC is ~50 lines of hand-rolled chi middleware (per Phase 19 architectural decision). No Casbin, no policy engine.

## Architecture Patterns

### Recommended Project Structure

```
internal/
├── server/
│   ├── handlers_projects.go      # CRUD handlers for projects
│   ├── handlers_members.go       # Membership management handlers
│   ├── handlers_runs_project.go  # Run submission/status under project scope
│   └── middleware.go             # projectContext + projectRoleRequired (extend existing)
├── store/
│   ├── store.go                  # Add Project, Member models + Store interface methods
│   └── sqlite/
│       ├── projects.go           # SQLiteStore project methods
│       ├── members.go            # SQLiteStore membership methods
│       └── migrations/
│           └── 00005_projects.sql
```

### Pattern 1: Project Context Middleware

The project-context middleware resolves `{slug}` from the URL path, loads the project from the store, and injects it into `r.Context()`. RBAC middleware downstream reads both JWT claims and the project record.

```go
// internal/server/middleware.go (addition)

type projectContextKey int
const projectKey projectContextKey = iota

// projectContext resolves {slug} and injects *store.Project into context.
// Returns 404 if the project does not exist or is inactive.
func (s *Server) projectContext(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        slug := chi.URLParam(r, "slug")
        proj, err := s.store.GetProjectBySlug(r.Context(), slug)
        if err != nil {
            writeError(w, http.StatusNotFound, "project not found")
            return
        }
        ctx := context.WithValue(r.Context(), projectKey, proj)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// projectRoleRequired returns a middleware that enforces a minimum project role.
// Role hierarchy: admin > executor > viewer.
// Global admins (claims.Role == "admin") bypass membership check.
func (s *Server) projectRoleRequired(minRole string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            claims := claimsFromContext(r.Context())
            if claims == nil {
                writeError(w, http.StatusUnauthorized, "not authenticated")
                return
            }
            // Global admin bypasses per-project membership.
            if claims.Role == "admin" {
                next.ServeHTTP(w, r)
                return
            }
            proj := r.Context().Value(projectKey).(*store.Project)
            member, err := s.store.GetProjectMember(r.Context(), proj.ID, claims.UserID)
            if err != nil {
                writeError(w, http.StatusForbidden, "not a project member")
                return
            }
            if !roleAtLeast(member.Role, minRole) {
                writeError(w, http.StatusForbidden, "insufficient project role")
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}

// roleAtLeast returns true if actualRole >= requiredRole in the hierarchy.
var roleRank = map[string]int{"viewer": 1, "executor": 2, "admin": 3}

func roleAtLeast(actual, required string) bool {
    return roleRank[actual] >= roleRank[required]
}
```

### Pattern 2: chi Route Group for Project-Scoped Routes

```go
// internal/server/server.go — newRouter() addition (auth.enabled=true branch)

r.Route("/projects", func(r chi.Router) {
    r.Use(JWTMiddleware(s.jwtSecret))
    r.Post("/", s.handleCreateProject)       // any authenticated user
    r.Get("/", s.handleListProjects)          // filtered to membership

    r.Route("/{slug}", func(r chi.Router) {
        r.Use(s.projectContext)

        r.With(s.projectRoleRequired("viewer")).Get("/", s.handleGetProject)
        r.With(s.projectRoleRequired("admin")).Put("/", s.handleUpdateProject)
        r.With(s.projectRoleRequired("admin")).Delete("/", s.handleDeleteProject)

        // Members sub-resource
        r.With(s.projectRoleRequired("viewer")).Get("/members", s.handleListMembers)
        r.With(s.projectRoleRequired("admin")).Post("/members", s.handleAddMember)
        r.With(s.projectRoleRequired("admin")).Put("/members/{username}", s.handleUpdateMember)
        r.With(s.projectRoleRequired("admin")).Delete("/members/{username}", s.handleRemoveMember)

        // Runs sub-resource (replaces old /workflows/... when auth enabled)
        r.Route("/runs", func(r chi.Router) {
            r.With(s.projectRoleRequired("executor")).Post("/submit", s.handleProjectSubmit)
            r.With(s.projectRoleRequired("viewer")).Get("/{id}/status", s.handleStatus)
            r.With(s.projectRoleRequired("viewer")).Get("/{id}/outputs", s.handleOutputs)
            r.With(s.projectRoleRequired("executor")).Post("/{id}/abort", s.handleAbort)
        })
    })
})
```

### Pattern 3: Migration Strategy

Two options for migration 00005:

**Option A (Recommended): Combined migration**
Drop all existing run data in the same migration that adds project_id and creates the projects/project_members tables. This is safe because backward compat for old runs is explicitly waived.

```sql
-- 00005_projects.sql

-- +goose Up
CREATE TABLE IF NOT EXISTS projects (
    id          TEXT     PRIMARY KEY,
    slug        TEXT     NOT NULL UNIQUE,
    description TEXT     NOT NULL DEFAULT '',
    is_active   INTEGER  NOT NULL DEFAULT 1,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_projects_slug ON projects(slug);

CREATE TABLE IF NOT EXISTS project_members (
    project_id  TEXT     NOT NULL REFERENCES projects(id),
    user_id     TEXT     NOT NULL REFERENCES users(id),
    role        TEXT     NOT NULL,  -- 'admin', 'executor', 'viewer'
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (project_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_project_members_user ON project_members(user_id);

-- Drop existing run data (no backward compat needed).
DELETE FROM tasks;
DELETE FROM runs;

-- Add project_id column. Auth-disabled mode uses a sentinel value or NULL;
-- auth-enabled mode enforces NOT NULL at the application layer.
ALTER TABLE runs ADD COLUMN project_id TEXT REFERENCES projects(id);

-- +goose Down
ALTER TABLE runs DROP COLUMN project_id;
DROP TABLE IF EXISTS project_members;
DROP TABLE IF EXISTS projects;
```

**Option B: Separate migrations (00005 + 00006)**
Keeps project table creation and runs modification separate. Adds clarity but more files. Given the waived backward compat, Option A is simpler.

**Note on NOT NULL enforcement:** SQLite `ALTER TABLE ADD COLUMN` cannot add a NOT NULL column without a default. Use `NULL` in the schema and enforce NOT NULL at the application layer (server checks project_id is set before CreateRun when auth is enabled). This also naturally handles the auth-disabled path without sentinel projects.

### Pattern 4: Store Model Additions

```go
// internal/store/store.go additions

// Project represents a project workspace.
type Project struct {
    ID          string
    Slug        string
    Description string
    IsActive    bool
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

// ProjectMember represents a user's membership in a project.
type ProjectMember struct {
    ProjectID string
    UserID    string
    Role      string    // "admin", "executor", "viewer"
    CreatedAt time.Time
}
```

Store interface additions (grouped):

```go
// Project operations
CreateProject(ctx context.Context, p *Project) error
GetProjectBySlug(ctx context.Context, slug string) (*Project, error)
GetProjectByID(ctx context.Context, id string) (*Project, error)
ListProjectsForUser(ctx context.Context, userID string) ([]Project, error)
ListAllProjects(ctx context.Context) ([]Project, error)  // for global admins
UpdateProject(ctx context.Context, p *Project) error
DeactivateProject(ctx context.Context, id string) error

// Membership operations
AddMember(ctx context.Context, m *ProjectMember) error
GetProjectMember(ctx context.Context, projectID, userID string) (*ProjectMember, error)
UpdateMemberRole(ctx context.Context, projectID, userID, role string) error
RemoveMember(ctx context.Context, projectID, userID string) error
ListMembers(ctx context.Context, projectID string) ([]ProjectMember, error)
CountProjectAdmins(ctx context.Context, projectID string) (int, error)  // for last-admin protection
```

The `Run` struct gains an optional `ProjectID` field:
```go
type Run struct {
    // ... existing fields ...
    ProjectID string  // empty string when auth disabled; set when auth enabled
}
```

`CreateRun` must be updated to write `project_id` when set.

### Anti-Patterns to Avoid

- **Per-handler role checks:** The CONTEXT.md decision requires per-route-group middleware. Do not replicate the pattern of checking membership inside every handler function body.
- **Slug as primary key:** Use UUID as primary key for projects; slug is a unique index. This allows slug changes without cascading FK updates (if slug renames are ever added).
- **Blocking on slug validation in middleware:** Validate slug format (lowercase alphanumeric + hyphens, max 64 chars) in the handler that creates/updates projects, not in middleware.
- **auth-disabled path breaking spectool:** The old `/workflows/...` flat routes must remain in the `else` branch of `newRouter()`. Do not remove them.
- **Last-admin check race condition:** Use `CountProjectAdmins` inside a write transaction (or at minimum immediately before `RemoveMember`) to prevent a concurrent removal that orphans the project.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| URL param extraction | Custom slug parser | `chi.URLParam(r, "slug")` | Already used throughout handlers.go |
| Migration sequencing | Manual SQL runner | goose (already wired) | Embedded migrations, automatic on Open() |
| UUID generation | rand.Read + hex | `uuid.New().String()` | Already used for user/run IDs |
| JSON request decoding | Manual reader | `json.NewDecoder(r.Body).Decode(&req)` | Established pattern in all handlers |
| Soft delete | Hard delete + archive table | `is_active = 0` boolean flag | Phase 19 pattern for users; reuse for projects |

**Key insight:** Every infrastructure primitive needed for this phase (routing, migrations, UUID, JSON, soft-delete) is already in the codebase. This phase is entirely a data model + API surface extension.

---

## Common Pitfalls

### Pitfall 1: SQLite ALTER TABLE Cannot Add NOT NULL Column Without Default

**What goes wrong:** `ALTER TABLE runs ADD COLUMN project_id TEXT NOT NULL` fails in SQLite because existing rows have no value for the new column.

**Why it happens:** SQLite's `ALTER TABLE ADD COLUMN` restrictions — unlike PostgreSQL, it cannot backfill existing rows with a computed default or a subquery.

**How to avoid:** Add the column as nullable (`project_id TEXT REFERENCES projects(id)`). Enforce NOT NULL at the application layer: when `auth.enabled=true`, the handler sets `project_id` before calling `CreateRun`; when auth is disabled, `project_id` is left NULL.

**Warning signs:** Migration fails at `goose.Up()` with `"Cannot add a NOT NULL column with no default value"`.

### Pitfall 2: Global Admin Implicit Access

**What goes wrong:** Global admin (JWT claims `role == "admin"`) calls `GET /projects/{slug}` but has no row in `project_members`. The `projectRoleRequired` middleware fetches the member record, gets a "not found" error, and returns 403.

**Why it happens:** The implicit access shortcut is not coded in the middleware.

**How to avoid:** In `projectRoleRequired`, check `claims.Role == "admin"` first and call `next.ServeHTTP` immediately if true — before any membership DB query. Document this with a comment.

### Pitfall 3: Last-Admin Protection Race

**What goes wrong:** Two concurrent requests both call `DELETE /projects/{slug}/members/{username}` on the last two admins. Both read `CountProjectAdmins == 2`, both proceed, and the project ends up with zero admins.

**Why it happens:** SQLite WAL mode allows concurrent reads. The count-then-delete is not atomic.

**How to avoid:** For production correctness, wrap the count check and delete in a single transaction. Alternatively, use a database constraint: a trigger or deferred check. For this phase, a simple approach is: always execute `CountProjectAdmins` and `RemoveMember` sequentially on the writer connection (single connection = serialised writes in SQLiteStore). Document the assumption in code comments. A full transaction is cleaner but adds boilerplate.

**Simplest safe implementation:** Since `SQLiteStore.writer` has `MaxOpenConns(1)`, all writes are serialised. A read-then-write sequence on the same connection is safe against concurrent writes from other goroutines because only one write can execute at a time. Add a `RemoveMemberIfNotLastAdmin` method that does both operations in sequence on the writer.

### Pitfall 4: Slug Validation Location

**What goes wrong:** Slug validation regex is placed in middleware, causing confusing 404-style errors for format violations (the project lookup fails with "not found" rather than "invalid slug format").

**Why it happens:** Slug validation and slug lookup are conflated.

**How to avoid:** Validate slug format only in `handleCreateProject` and `handleUpdateProject`. In the URL-resolving middleware (`projectContext`), just look up the slug — if the slug is malformed and thus not in the DB, it returns 404 (which is acceptable; users shouldn't be constructing arbitrary malformed slugs manually).

### Pitfall 5: Run Routes Duplication

**What goes wrong:** The submit/status/outputs/abort handlers are duplicated — one copy under `/projects/{slug}/runs/...` and one copy under the old `/workflows/...` path. They diverge over time.

**Why it happens:** Auth-gated routing requires two separate handler registrations.

**How to avoid:** Keep a single handler function per operation (e.g., `handleSubmit`). The auth-enabled version of `handleSubmit` reads `project_id` from the context (injected by `projectContext` middleware). The auth-disabled version of `handleSubmit` leaves `project_id` empty. Use a helper `projectIDFromContext(ctx) string` that returns empty string when no project is in context. This way both branches call the same handler.

### Pitfall 6: ListProjects Returns All Projects to Any User

**What goes wrong:** `GET /projects` returns all projects on the server, not filtered to the requester's memberships.

**Why it happens:** Handler calls `ListAllProjects` instead of `ListProjectsForUser`.

**How to avoid:** `handleListProjects` checks: if `claims.Role == "admin"`, call `ListAllProjects`; otherwise call `ListProjectsForUser(ctx, claims.UserID)`. This is tested by success criterion 1 in the phase spec.

---

## Code Examples

### Creating a Project with Auto-Membership

```go
// internal/server/handlers_projects.go
func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
    claims := claimsFromContext(r.Context())
    if claims == nil {
        writeError(w, http.StatusUnauthorized, "not authenticated")
        return
    }

    var req api.CreateProjectRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }
    if !validSlug(req.Slug) {
        writeError(w, http.StatusBadRequest, "slug must be lowercase alphanumeric and hyphens, max 64 chars")
        return
    }

    proj := &store.Project{
        ID:          uuid.New().String(),
        Slug:        req.Slug,
        Description: req.Description,
        IsActive:    true,
    }
    if err := s.store.CreateProject(r.Context(), proj); err != nil {
        writeError(w, http.StatusConflict, "project slug already exists")
        return
    }

    // Creator becomes admin automatically.
    m := &store.ProjectMember{
        ProjectID: proj.ID,
        UserID:    claims.UserID,
        Role:      "admin",
    }
    if err := s.store.AddMember(r.Context(), m); err != nil {
        log.Printf("handleCreateProject: add creator as admin: %v", err)
        writeError(w, http.StatusInternalServerError, "failed to create project")
        return
    }

    writeJSON(w, http.StatusCreated, projectToResponse(proj))
}

// validSlug returns true for lowercase alphanumeric + hyphen strings, max 64 chars.
var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}[a-z0-9]$|^[a-z0-9]$`)

func validSlug(s string) bool {
    return len(s) > 0 && len(s) <= 64 && slugRe.MatchString(s)
}
```

### Slug Regex Note

The slug pattern `^[a-z0-9][a-z0-9-]{0,62}[a-z0-9]$|^[a-z0-9]$` allows single-char slugs and prevents leading/trailing hyphens. This matches the typical DNS hostname label convention which is appropriate for registry references like `project/name:tag`.

### Run Submission Under Project Context

```go
// handleProjectSubmit: same as handleSubmit but injects project_id
func (s *Server) handleProjectSubmit(w http.ResponseWriter, r *http.Request) {
    proj, _ := r.Context().Value(projectKey).(*store.Project)
    // ... same parsing logic as handleSubmit ...
    run := &store.Run{
        ID:        runID,
        WDLPath:   req.WDLPath,
        InputsJSON: string(inputsJSON),
        State:     store.RunPending,
        ProjectID: proj.ID,
    }
    // ... rest of handleSubmit logic ...
}
```

### SQLite Membership SQL Patterns

```sql
-- ListProjectsForUser: projects where user is a member
SELECT p.id, p.slug, p.description, p.is_active, p.created_at, p.updated_at
  FROM projects p
  JOIN project_members pm ON pm.project_id = p.id
 WHERE pm.user_id = ? AND p.is_active = 1
 ORDER BY p.created_at ASC;

-- CountProjectAdmins: used for last-admin protection
SELECT COUNT(*) FROM project_members
 WHERE project_id = ? AND role = 'admin';

-- GetProjectMember: membership lookup for RBAC
SELECT project_id, user_id, role, created_at
  FROM project_members
 WHERE project_id = ? AND user_id = ?;
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Flat `/workflows/...` routes | `/projects/{slug}/runs/...` when auth enabled | Phase 20 | Run endpoints move into project hierarchy |
| No project_id on runs | `project_id` column on runs table | Migration 00005 | All new runs carry project association |
| Global RBAC only (admin/executor/viewer) | Project-level RBAC + global RBAC | Phase 20 | Fine-grained per-resource access control |

**Nothing deprecated in this phase** — the old `/workflows/...` flat routes are preserved for auth-disabled mode.

---

## Open Questions

1. **project_id NOT NULL enforcement strategy**
   - What we know: SQLite `ALTER TABLE ADD COLUMN` cannot add NOT NULL without a default; application-layer enforcement is the standard SQLite workaround.
   - What's unclear: Whether to use NULL for auth-disabled path or a sentinel project (e.g., a `__default__` project auto-created when auth is disabled).
   - Recommendation: Use NULL for auth-disabled path. Keep the schema permissive; the application enforces the invariant. This is simpler and avoids introducing a sentinel object that leaks into listing endpoints.

2. **handleSubmit vs handleProjectSubmit — shared or separate**
   - What we know: The context says run endpoints move under `/projects/{slug}/runs/...` when auth is enabled, but old `/workflows/...` paths remain when disabled.
   - What's unclear: Whether to have one handler with a nil-safe `projectIDFromContext` or two handler functions.
   - Recommendation: One handler with a `projectIDFromContext(ctx) string` helper returning `""` when no project in context. Avoids code duplication. Tested separately for auth-enabled and auth-disabled configurations.

3. **UpdatedAt on project_members**
   - What we know: The join table schema defined in CONTEXT.md has `created_at` but no `updated_at`.
   - What's unclear: Whether role updates need an audit timestamp.
   - Recommendation: Add `updated_at` to `project_members` as Claude's discretion field. Low cost, maintains consistency with other tables, useful for debugging.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go testing + httptest (stdlib) |
| Config file | none — `go test ./...` |
| Quick run command | `go test ./internal/store/sqlite/... ./internal/server/... -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements to Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PROJ-01 | POST /projects creates project and adds creator as admin | integration | `go test ./internal/server/... -run TestCreateProject -count=1` | Wave 0 |
| PROJ-02 | GET /projects returns only projects the caller is a member of | integration | `go test ./internal/server/... -run TestListProjects -count=1` | Wave 0 |
| PROJ-03 | GET /projects/{slug} returns project details to members | integration | `go test ./internal/server/... -run TestGetProject -count=1` | Wave 0 |
| PROJ-04 | Admin can update/delete; non-admin gets 403 | integration | `go test ./internal/server/... -run TestProjectAdminOps -count=1` | Wave 0 |
| PROJ-05 | Admin adds/removes members; last-admin protection fires | integration | `go test ./internal/server/... -run TestMembership -count=1` | Wave 0 |
| PROJ-06 | Admin changes member role | integration | `go test ./internal/server/... -run TestUpdateMemberRole -count=1` | Wave 0 |
| PROJ-07 | Run submitted under /projects/{slug}/runs/submit has project_id set | integration | `go test ./internal/server/... -run TestProjectRun -count=1` | Wave 0 |
| PROJ-07 | Auth-disabled path: old /workflows/... routes still work | integration | `go test ./internal/server/... -run TestAuthDisabledBackcompat -count=1` | Wave 0 |
| PROJ-01-06 | Store layer: CRUD correctness | unit | `go test ./internal/store/sqlite/... -run TestProject -count=1` | Wave 0 |

### Sampling Rate

- **Per task commit:** `go test ./internal/store/sqlite/... ./internal/server/... -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green (including existing spectool-exercised paths) before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `internal/server/handlers_projects_test.go` — covers PROJ-01 through PROJ-07 HTTP layer
- [ ] `internal/store/sqlite/projects_test.go` — covers store-layer project CRUD
- [ ] `internal/store/sqlite/members_test.go` — covers store-layer membership CRUD + last-admin protection

*(Existing test infrastructure — `sqlite.OpenMemory()`, `newAuthTestServer()`, `doAuthRequest()` — is reusable for all new tests.)*

---

## Sources

### Primary (HIGH confidence)

- Direct codebase inspection: `internal/server/middleware.go`, `internal/server/server.go`, `internal/store/store.go`, `internal/store/sqlite/users.go`, `internal/store/sqlite/migrations/00004_users.sql` — all patterns derived from the existing, working Phase 19 implementation
- go.mod v1.25.0 — exact dependency versions confirmed
- chi v5 documentation pattern: `r.Route` + `r.Use` for per-group middleware — consistent with existing Phase 19 usage in `newRouter()`
- SQLite ALTER TABLE constraints — established behavior; confirmed by existing migration patterns in the codebase

### Secondary (MEDIUM confidence)

- SQLite NOT NULL column addition restriction: well-known SQLite limitation (no verification against official docs performed; HIGH confidence from practical experience and codebase evidence of working around datetime defaults)

### Tertiary (LOW confidence)

- Last-admin protection race analysis: reasoning from SQLiteStore `MaxOpenConns(1)` writer serialisation — not formally verified against chi/SQLite documentation

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries already in use, no new dependencies
- Architecture: HIGH — patterns directly derived from Phase 19 existing code
- Pitfalls: HIGH (items 1-2, 5-6) / MEDIUM (items 3-4) — items 3-4 are race analysis reasoning

**Research date:** 2026-03-17
**Valid until:** 2026-04-17 (stable stack, no fast-moving dependencies)
