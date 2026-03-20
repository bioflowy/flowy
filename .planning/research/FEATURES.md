# Feature Research

**Domain:** Web UI management SPA for WDL workflow engine — login, user/project/registry/file management screens
**Researched:** 2026-03-20
**Confidence:** HIGH (screen patterns based on direct API inspection), MEDIUM (UX patterns from comparable tools), LOW (file browser — no file-list API exists yet)

---

## Context: Existing Backend API

All REST endpoints already exist in flowy-server. This research covers only what the **frontend** needs to implement against those endpoints.

### Available API Surface (confirmed by code inspection)

| Endpoint | Purpose | Auth Required |
|----------|---------|---------------|
| `POST /auth/login` | Credential exchange → JWT token | None |
| `GET /users/me` | Current user profile | JWT |
| `PUT /users/me/password` | Change own password | JWT |
| `POST /users` | Create user | JWT + global admin |
| `GET /users` | List all users | JWT + global admin |
| `DELETE /users/{id}` | Deactivate user | JWT + global admin |
| `POST /projects` | Create project | JWT |
| `GET /projects` | List accessible projects | JWT |
| `GET /projects/{slug}` | Get project detail | JWT + viewer |
| `PUT /projects/{slug}` | Update project description | JWT + project admin |
| `DELETE /projects/{slug}` | Deactivate project | JWT + project admin |
| `GET /projects/{slug}/members` | List project members | JWT + viewer |
| `POST /projects/{slug}/members` | Add member | JWT + project admin |
| `PUT /projects/{slug}/members/{username}` | Change member role | JWT + project admin |
| `DELETE /projects/{slug}/members/{username}` | Remove member | JWT + project admin |
| `GET /projects/{slug}/registry` | List registry entries | JWT + viewer |
| `GET /projects/{slug}/registry/{name}` | Pull WDL (by tag or digest) | JWT + viewer |
| `GET /projects/{slug}/registry/{name}/versions` | List all versions | JWT + viewer |
| `POST /projects/{slug}/registry/{name}/tags` | Create/move tag | JWT + project admin |
| `DELETE /projects/{slug}/registry/{name}/tags/{tag}` | Delete tag | JWT + project admin |
| `DELETE /projects/{slug}/registry` | Delete entry | JWT + project admin |
| `GET /projects/{slug}/runs/{id}/status` | Run status | JWT + viewer |
| `GET /projects/{slug}/runs/{id}/outputs` | Run outputs (JSON map) | JWT + viewer |
| `GET /projects/{slug}/provenance` | Provenance list (filterable) | JWT + viewer |
| `GET /projects/{slug}/provenance/{run_id}` | Provenance detail | JWT + viewer |

### API Gaps for File Browsing

The backend currently provides no file-listing or file-download endpoints. Output paths arrive as string values inside the outputs JSON map (e.g., `"output_file": "/data/runs/abc/outputs/result.tsv"` for local, or `"output_file": "s3://bucket/prefix/runs/abc/result.tsv"` for S3). The UI cannot directly browse or download these without new server endpoints. This is a hard dependency for the file browser screen.

---

## Feature Landscape

### Screen 1: Login

#### Table Stakes

| Feature | Why Expected | Complexity | API Dependency |
|---------|--------------|------------|----------------|
| Username + password form | Entry point for all authenticated users; no auth = no access to any other screen | LOW | `POST /auth/login` |
| JWT storage in memory or sessionStorage | Token must persist across SPA route changes; localStorage is insecure (XSS steals tokens) | LOW | Client-side only |
| Redirect to original URL after login | Standard web app UX — navigating to a protected URL while logged out should resume after auth | LOW | Client-side routing |
| Clear error message on 401 | User must know if credentials are wrong vs server is down | LOW | HTTP 401 response parsing |
| Disable form during in-flight request | Prevent duplicate submissions | LOW | TanStack Query mutation state |

#### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Persist session across page reload | Using sessionStorage (not memory) means token survives F5 without re-login | LOW | sessionStorage is cleared on tab close; acceptable trade-off vs localStorage |
| Auto-logout on JWT expiry | Server will return 401 on expired token; UI should intercept globally and redirect to login | LOW | TanStack Query global error handler or Axios interceptor pattern |

#### Anti-Features

| Feature | Why Problematic | Alternative |
|---------|-----------------|-------------|
| "Remember me" with localStorage | XSS exposure; PROJECT.md does not include refresh tokens, so long-lived tokens in localStorage is a security liability | sessionStorage (tab-scoped) is acceptable without refresh tokens |
| OAuth2 / SSO login button | Explicitly out of scope per PROJECT.md; backend has no OAuth2 support | Plain username/password |
| Biometric / WebAuthn | No backend support; enormous scope | Username/password sufficient |

---

### Screen 2: User Management

**Access:** Global admin role only. Non-admins must not see this screen at all (hide in nav, guard route).

#### Table Stakes

| Feature | Why Expected | Complexity | API Dependency |
|---------|--------------|------------|----------------|
| List all users (table view) | Admin needs an overview of who exists in the system | LOW | `GET /users` |
| Show active/inactive status visually | Deactivated users must be distinguishable at a glance | LOW | `is_active` field in `UserResponse` |
| Create user form (username, password, display name, email, global role) | Admin lifecycle management | LOW | `POST /users` |
| Deactivate user (confirm dialog) | Soft-delete — no hard delete via API; admin needs a way to revoke access | LOW | `DELETE /users/{id}` |
| Show global role (admin/user) per row | Admins need to know who else has admin privileges | LOW | `global_role` field in `UserResponse` |

#### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Inline validation on create form | Username empty / password too short caught before API call, not after | LOW | Client-side validation with React Hook Form or Zod |
| Search/filter users by username or display name | Useful once user count exceeds 20–30; simple client-side filter is sufficient | LOW | Client-side filter on fetched list; no API pagination endpoint exists |
| Copy user ID to clipboard | Admin may need UUID for debugging or scripting | LOW | navigator.clipboard API |

#### Anti-Features

| Feature | Why Problematic | Alternative |
|---------|-----------------|-------------|
| Edit all user fields inline | `PUT /users/{id}` does not exist — only `PUT /users/me/password`. Exposing phantom edit buttons that fail silently is harmful | Show fields as read-only; only deactivate is a write operation |
| Hard delete | API performs soft-delete only (`DeactivateUser`). UI should not imply permanent removal | Label as "Deactivate" not "Delete"; show deactivated users as greyed-out rows |
| Bulk operations (select all, bulk deactivate) | API has no batch endpoint; implementing multi-call sequences with partial failure handling is disproportionate complexity | Single-row actions only; bulk is a future enhancement |
| Password reset by admin | API provides only `PUT /users/me/password` (self-service). No admin-reset endpoint exists | Admin creates a new account; user changes password on first login (no first-login forced password change mechanism in v1.3) |

---

### Screen 3: Project Management

**Access:** All authenticated users see projects they are a member of. Global admin sees all projects via `ListAllProjects` path (middleware allows admin to see projects they are not a member of). Project admin role gates write operations.

#### Table Stakes

| Feature | Why Expected | Complexity | API Dependency |
|---------|--------------|------------|----------------|
| List projects (cards or table) | Entry point for all project-scoped work; users expect to see their workspaces at a glance | LOW | `GET /projects` |
| Create project form (slug + description) | Any authenticated user can create a project | LOW | `POST /projects` |
| Project detail page showing description, members, slug | Users need to understand the project's purpose and who has access | LOW | `GET /projects/{slug}` |
| Members list with role badges | Viewer must see who has what access level | LOW | `GET /projects/{slug}/members` |
| Add member form (username + role selection) | Project admin needs to grant access to colleagues | LOW | `POST /projects/{slug}/members` |
| Change member role (dropdown or radio) | Access changes without removing and re-adding the member | LOW | `PUT /projects/{slug}/members/{username}` |
| Remove member (confirm dialog) | Project admin must be able to revoke access | LOW | `DELETE /projects/{slug}/members/{username}` |
| Update project description | Projects evolve; description should be editable | LOW | `PUT /projects/{slug}` |
| Deactivate project (project admin only, confirm dialog) | Lifecycle management; no accidental deletion without confirmation | LOW | `DELETE /projects/{slug}` |

#### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Role badge color coding (admin=red, executor=blue, viewer=grey) | Instant visual recognition of who can do what; matches shadcn/ui Badge variants | LOW | CSS only |
| Slug validation in create form (lowercase alphanumeric + hyphens) | Slug format is enforced by the API; surface the constraint before submission | LOW | Regex validation client-side |
| Prevent removing last admin (client hint) | API already enforces `RemoveMemberIfNotLastAdmin`; pre-check on the client avoids a confusing 400 error | MEDIUM | Count admins in the members list before allowing the action; show tooltip explaining why the button is disabled |
| Project member count on project card | Administrators want a quick overview without entering each project | LOW | Compute from `members` response (must be loaded) or add a summary count field |

#### Anti-Features

| Feature | Why Problematic | Alternative |
|---------|-----------------|-------------|
| Inline slug editing after creation | Slug is the project's URL identifier; the API has no slug-change endpoint. Changing the slug would break all bookmarks and CLI references | Slug is read-only after creation; only description is editable |
| Project archiving with visibility toggle | API only has `is_active` soft-delete; no archived/active distinction beyond that | Use "Deactivate" consistently with a confirm dialog |
| Nested sub-projects | Not in the data model; excessive scope | Flat project list only |

---

### Screen 4: Registry Browsing

**Access:** Any project member (viewer+) can browse. Project admin can manage tags and delete entries. Executor can push (push is CLI-only in this milestone; UI is read-oriented).

#### Table Stakes

| Feature | Why Expected | Complexity | API Dependency |
|---------|--------------|------------|----------------|
| Registry entry list (name, kind=tool/workflow, version count, tags) | Users need to discover what tools and workflows are available in the project | LOW | `GET /projects/{slug}/registry` |
| Kind badge (tool / workflow) | Distinguishes WDL tasks from full workflows at a glance | LOW | `kind` field in `RegistryEntryResponse` |
| Tags displayed as chips on each entry | Tags (e.g. `latest`, `prod`) are the primary reference for execution; must be visible without drilling in | LOW | `tags` array in `RegistryEntryResponse` |
| Entry detail: versions list (digest, tags, created date) | Developers need to see version history to understand what changed and pin to a specific digest | LOW | `GET /projects/{slug}/registry/{name}/versions` |
| WDL content viewer (read-only, syntax highlighted) | The content is the value; users need to inspect the WDL before submitting a run | MEDIUM | `GET /projects/{slug}/registry/{name}?tag=X` or `?digest=Y` |
| Copy registry reference to clipboard (`name:tag` or `name@sha256:hex`) | Users will use this reference in CLI submissions; copy-to-clipboard is the expected UX | LOW | Compose string from list data; navigator.clipboard |
| Delete tag (project admin only) | Tag management is an expected admin operation | LOW | `DELETE /projects/{slug}/registry/{name}/tags/{tag}` |
| Create/move tag (project admin only) | Tag reassignment is needed to promote versions (e.g. move `prod` tag to a new digest) | LOW | `POST /projects/{slug}/registry/{name}/tags` |
| Delete registry entry (project admin only, confirm dialog) | Lifecycle management; irreversible, so confirmation required | LOW | `DELETE /projects/{slug}/registry/{name}` |

#### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Digest truncated display (`sha256:abc1234…`) with copy-full on click | Full 72-char digest is unwieldy; truncated display with clipboard access matches Docker Hub UX | LOW | String truncation + Tooltip with full digest + copy button |
| WDL syntax highlighting | WDL is not a standard language; most syntax highlighters support it as a subset of CWL or with a custom grammar | MEDIUM | Use `highlight.js` with WDL grammar or fall back to generic programming language highlighting. Prism has WDL plugin (LOW confidence — needs verification). |
| Filter entries by kind (tool / workflow) | Registry can grow; filtering helps bioinformaticians find the right entry type | LOW | Client-side filter on the fetched list |
| Version comparison (side-by-side diff of two digests) | Developers need to understand what changed between versions | HIGH | Requires fetching two versions and a diff library (e.g. `diff`). Deferred — nice to have but not table stakes for v2.0. |

#### Anti-Features

| Feature | Why Problematic | Alternative |
|---------|-----------------|-------------|
| Push WDL via UI (file upload or text editor) | Registry push is a developer/CI operation via CLI. Web-based WDL editor adds validation complexity (WDL parse errors) and is far out of scope for a management UI | Browse and tag management only; push stays in CLI |
| Full WDL editor with syntax check | Would require a WDL language server or WASM-compiled parser in the browser | Read-only viewer with syntax highlighting |
| Automatic tag suggestions (semver increment) | The engine treats tags as arbitrary strings; generating semver suggestions is opinionated and adds complexity | Let the user type the tag string |

---

### Screen 5: File Browsing and Download

**Access:** Any project member (viewer+). Files are output paths from completed runs.

**Critical note:** The backend has NO file listing or file download endpoint. Output paths are returned by `GET /projects/{slug}/runs/{id}/outputs` as a JSON map (e.g., `{"aligned.bam": "/data/runs/abc/outputs/aligned.bam"}`). The UI cannot serve these files directly. New server-side endpoints are required before this screen can be built.

#### Required New Backend Endpoints (Blocking)

Before the file browser screen can be implemented, the `flowy-server` or `flowy-ui` proxy must provide:

| Needed Endpoint | Purpose | Notes |
|-----------------|---------|-------|
| `GET /projects/{slug}/runs` | List runs for a project (with status, created_at) | `ListRunsByProject` store method does not exist yet |
| `GET /projects/{slug}/runs/{id}/files` | List output files for a completed run (name, size, type) | Needs backend to resolve paths from OutputsJSON |
| `GET /projects/{slug}/runs/{id}/files/download?path=...` | Stream/redirect file content to browser | For local backend: serve file bytes; for S3 backend: generate presigned URL and redirect |

#### Table Stakes (once backend endpoints exist)

| Feature | Why Expected | Complexity | API Dependency |
|---------|--------------|------------|----------------|
| Run list per project (ID, status badge, created date) | Starting point for finding output files — users identify which run produced the files they want | LOW | New `GET /projects/{slug}/runs` endpoint |
| Status badge (PENDING/RUNNING/COMPLETED/FAILED/ABORTED) | Users only want to browse files from COMPLETED runs | LOW | `run_state` field |
| Output file list for a run (filename, path) | Direct access to what the workflow produced | LOW | New `GET /projects/{slug}/runs/{id}/files` or parse OutputsJSON |
| Download file button | The core value of the file browser — retrieve computation results | MEDIUM | New download/presigned-URL endpoint |
| Filter runs by status | Large deployments accumulate many runs; filtering to COMPLETED is the most common need | LOW | Client-side filter or query param |

#### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Presigned S3 URL redirect for S3 backend | Files can be large; streaming through the proxy server is wasteful. S3 presigned URLs let the browser download directly from S3. | MEDIUM | `flowy-ui` generates presigned URL server-side (AWS SDK v2 `PresignGetObject`) and redirects the browser. Local backend serves bytes directly. |
| File type icons (`.bam`, `.vcf`, `.tsv`, `.fastq`) | Bioinformatics output types are recognizable; icons help users identify file types without opening them | LOW | Map extension → icon in UI; no backend change |
| Copy output path to clipboard | Advanced users may want the `s3://bucket/key` or filesystem path for use in next-run inputs | LOW | Display path value from OutputsJSON; clipboard copy |
| Provenance link from run to registry entry | Show which WDL version produced the files, with a link to the registry entry | LOW | Combine provenance API (`GET /projects/{slug}/provenance/{run_id}`) with registry browsing link |

#### Anti-Features

| Feature | Why Problematic | Alternative |
|---------|-----------------|-------------|
| Directory tree browser | Workflow outputs are flat key-value pairs in OutputsJSON (e.g. `"workflow.bam": "path"`). There is no directory structure in the data model. Generating a tree view from output paths would be fragile | Flat list of output files keyed by WDL output name |
| File upload (stage inputs via UI) | Inputs staging (copying files to the run input directory before submission) is a CLI concern. Run submission via UI is out of scope for v2.0 | Download only |
| File preview in browser (BAM viewer, VCF viewer) | Domain-specific bioinformatics viewers (e.g. IGV.js) are enormous scope; BAM files require indexed access | External tools; provide download link and documentation |
| Streaming log tail (stdout/stderr per task) | No task log API exists; PROJECT.md defers WebSocket/SSE | Status polling only; logs are a future milestone |

---

## Feature Dependencies

```
Login Screen
    └──required by──> All other screens (JWT must exist in session)
    └──produces──> JWT token stored in sessionStorage

User Management Screen
    └──requires──> Login (JWT)
    └──requires──> global_role = "admin" in JWT claims (gate route + nav item)
    └──calls──> GET /users, POST /users, DELETE /users/{id}

Project Management Screen
    └──requires──> Login (JWT)
    └──calls──> GET /projects, POST /projects, GET /projects/{slug}/members, etc.

Registry Browsing Screen
    └──requires──> Login (JWT)
    └──requires──> Project selection (project slug in route)
    └──calls──> GET /projects/{slug}/registry, GET /projects/{slug}/registry/{name}/versions

File Browsing Screen
    └──requires──> Login (JWT)
    └──requires──> Project selection (project slug in route)
    └──requires──> New backend endpoints (GET /projects/{slug}/runs, file download)
    └──enhances──> Registry Browsing (provenance links)

WDL Content Viewer
    └──requires──> Registry entry + version selection
    └──enhances──> Registry Browsing Screen
```

### Dependency Notes

- **Login must be the first screen built.** All other screens fail to load without a valid JWT. The TanStack Query global 401 interceptor and sessionStorage token must be in place before any other screen is wired.
- **Project selection is a prerequisite for registry and file browsing.** Both screens are project-scoped; the project slug must be in the URL route (e.g., `/projects/my-project/registry`). A project list/selector must be navigable before entering either screen.
- **File browsing is blocked by new backend endpoints.** Building the file browser before `GET /projects/{slug}/runs` and the download endpoint exist is impossible. This screen should be deferred to a later phase or the blocking endpoints must be built first within the same milestone.
- **User management is independent of all other screens** except login. It can be built in parallel with project/registry screens.

---

## MVP Definition

### Launch With (v2.0)

Minimum set for the Web UI milestone to be complete and useful.

- [ ] **Login screen** — JWT acquisition; redirect to dashboard on success
- [ ] **User management screen** (admin only) — list, create, deactivate; show global role
- [ ] **Project list screen** — list accessible projects; create new project
- [ ] **Project detail screen** — description, member list with roles, add/remove/change-role members
- [ ] **Registry entry list screen** — entries with kind + tags in selected project
- [ ] **Registry entry detail screen** — version history, WDL content viewer, tag management (admin)
- [ ] **Run list screen** (prerequisite for file browsing) — list runs per project with status
- [ ] **File browser screen** — output file list per run with download (requires new backend endpoints)
- [ ] **CORS support** — `flowy-server` must send CORS headers for dev mode (`flowy-ui` proxy handles it in production)
- [ ] **`flowy-ui` binary** — Go binary embedding the React SPA and reverse-proxying `/api/` to `flowy-server`

### Add After Validation (v2.x)

- [ ] **Run submission form** — submit a workflow from the registry via the UI (requires input form generation from WDL type schema)
- [ ] **WDL diff viewer** — compare two versions side by side
- [ ] **Provenance detail screen** — full audit trail with inputs snapshot rendered as a table
- [ ] **Refresh token / auto-renew** — seamless session management for long-lived admin sessions
- [ ] **File type icons for bioinformatics formats** — `.bam`, `.vcf`, `.fastq`, `.bed` recognized by extension

### Future Consideration (v3+)

- [ ] **IGV.js or similar in-browser genomics viewer** — view BAM/VCF files directly
- [ ] **WebSocket run status updates** — real-time progress without polling
- [ ] **OAuth2/SSO login** — enterprise auth integration
- [ ] **Audit log screen** — who did what, when (requires audit log backend)

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Login screen + JWT session | HIGH | LOW | P1 |
| Project list + create | HIGH | LOW | P1 |
| Project detail + member management | HIGH | LOW | P1 |
| Registry entry list | HIGH | LOW | P1 |
| Registry entry detail + WDL viewer | HIGH | MEDIUM | P1 |
| User management (admin) | HIGH | LOW | P1 |
| Run list per project | HIGH | MEDIUM | P1 (requires new backend endpoint) |
| File browser + download | HIGH | HIGH | P1 (blocked by new backend endpoints) |
| Tag management in registry | MEDIUM | LOW | P1 |
| WDL syntax highlighting | MEDIUM | MEDIUM | P2 |
| Client-side slug / form validation | MEDIUM | LOW | P2 |
| Presigned S3 URL download | HIGH | MEDIUM | P2 (requires S3 backend on server) |
| Run submission via UI | MEDIUM | HIGH | P3 |
| WDL diff viewer | LOW | HIGH | P3 |
| Bioinformatics file viewers | LOW | HIGH | P3 |

**Priority key:** P1 = must have for v2.0 launch, P2 = ship if time allows, P3 = future milestone

---

## Competitor Feature Analysis

| Feature | Cromwell UI (WDL-native) | Nextflow Tower | Our Approach |
|---------|--------------------------|----------------|--------------|
| Login | Google OAuth2 (Cromwell SaaS) / no auth (local) | GitHub OAuth2 + org SSO | Username/password + JWT — self-hosted, no external IdP |
| User management | Admin via IAM (SaaS) or no UI (local) | Workspace admin panel | Dedicated admin screen; global role gating |
| Project/workspace | Terra Workspaces (GCP IAM backed) | Workspaces per org | `projects` table with slug + member roles |
| Registry browsing | None (Cromwell has no built-in registry) | None natively (uses Docker Hub / Wave) | Project-scoped registry with version history and tag management UI |
| Run history | Run list with status filtering, call-level detail | Pipeline run list with task timeline | Run list per project + status badge; call-level is future work |
| File download | Terra file browser (GCS only) | S3/GCS download links | Output file list from OutputsJSON + presigned URL or proxy download |
| WDL viewer | None | N/A (Nextflow DSL2) | Read-only viewer with syntax highlighting |

---

## Sources

- flowy source `internal/server/server.go` — complete route map confirming all available endpoints (HIGH confidence — direct inspection)
- flowy source `internal/api/types.go` — all request/response DTOs confirming field names and types (HIGH confidence — direct inspection)
- flowy source `internal/store/store.go` — Store interface confirming what queries exist and which are missing (HIGH confidence — direct inspection)
- flowy source `internal/server/handlers.go` — `handleOutputs` confirms OutputsJSON is a flat key-value map, not a directory tree (HIGH confidence — direct inspection)
- [JWT token storage in SPAs: in-memory vs sessionStorage vs localStorage](https://cybersierra.co/blog/react-jwt-storage-guide/) (MEDIUM confidence — WebSearch)
- [S3 presigned URLs for React download flows](https://oneuptime.com/blog/post/2026-02-12-s3-presigned-urls-react-application/view) (MEDIUM confidence — WebSearch)
- [Handling JWT in React admin apps — in-memory token pattern](https://marmelab.com/blog/2020/07/02/manage-your-jwt-react-admin-authentication-in-memory.html) (MEDIUM confidence — WebSearch)
- [Azure Logic Apps / GitHub Actions workflow run history UI patterns](https://learn.microsoft.com/en-us/azure/logic-apps/view-workflow-status-run-history) (MEDIUM confidence — comparable tool UX)
- [AWS Amplify Storage Browser for S3 — file browser pattern](https://ui.docs.amplify.aws/react/connected-components/storage/storage-browser) (MEDIUM confidence — comparable tool UX)
- PROJECT.md — explicit out-of-scope items (WebSocket/SSE, OAuth2/LDAP, run submission via UI) (HIGH confidence — project spec)

---

*Feature research for: flowy v2.0 — Web UI management SPA*
*Researched: 2026-03-20*
