# Feature Research

**Domain:** Multi-user WDL workflow engine — user auth, project RBAC, WDL registry, provenance tracking
**Researched:** 2026-03-17
**Confidence:** HIGH (auth/RBAC patterns in Go+chi), MEDIUM (registry design), MEDIUM (provenance schema)

---

## Existing Baseline (Do Not Re-implement)

The following are already shipped and are dependencies for v1.3 features:

- WDL 1.2 parsing, type-checking, execution via Docker (v1.0)
- flowy-server REST API with chi v5, SQLite/WAL, goose migrations (v1.1)
- S3/MinIO storage backend with LED daemon pattern (v1.2)
- `runs` and `tasks` tables, `Store` interface with `CreateRun`/`GetRun`/`UpdateRunState` etc. (v1.1)
- `WDLContent` field in `TaskClaimResponse` already allows daemons to receive WDL inline — no shared FS needed for registry-sourced runs (v1.2 already unlocked this)

**Library choices already confirmed by ecosystem research:**
- `golang-jwt/jwt/v5` — the maintained fork of the abandoned `dgrijalva/jwt-go`; standard for Go JWT (HIGH confidence)
- `go-chi/jwtauth/v5` — chi-native JWT middleware; direct fit for the existing chi v5 router (HIGH confidence)
- `bcrypt` via `golang.org/x/crypto/bcrypt` — the standard Go password hashing approach (HIGH confidence)

---

## Feature Landscape

### Table Stakes (Users Expect These)

Features without which the v1.3 milestone is incomplete or unusable.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Username/password login returning a JWT | Every multi-user REST API needs a login endpoint; JWT is the stateless standard for Go REST APIs | LOW | `POST /auth/login` → `{token, expires_at}`. bcrypt for password hash. HS256 signing sufficient for single-server; RS256 only needed for multi-service token delegation (out of scope per PROJECT.md). |
| JWT middleware protecting all non-public routes | Without this, auth is theater — existing endpoints become unguarded | LOW | `go-chi/jwtauth/v5` attaches as chi group middleware wrapping all existing and new protected routes. Public routes: `POST /auth/login`, `GET /health`. |
| User creation and listing (admin-only) | Multi-user system needs lifecycle management | LOW | `POST /users` (admin only) + `GET /users` (admin only). Seed an initial `admin` user via a migration with a configurable password, or a `flowy-server --init-admin` flag. |
| Projects as isolation boundaries | Every bioinformatics platform groups work into projects/workspaces: Terra uses Workspaces, Cromwell-as-a-service uses namespaces. Without projects, all runs are globally visible. | MEDIUM | A project owns runs, registry entries, and provenance records. `runs` table gains a `project_id` FK (nullable for backward compat with pre-v1.3 runs). |
| Three-tier RBAC per project: admin / executor / viewer | Terra uses Owner/Writer/Reader; Dockstore uses Admin/Maintainer/Member. Three tiers covers 95% of team workflows without complexity overhead. | MEDIUM | Admin: manage members, delete project, delete registry entries. Executor: submit runs, push registry versions. Viewer: read runs, pull registry content, read provenance. Roles are per-project (user can be executor in project A and viewer in project B). |
| WDL registry: push a versioned entry | The core deliverable. Users need a canonical place to store and retrieve WDL by content hash or tag. Terra/Dockstore are external services; flowy should have a built-in registry for self-hosted use. | HIGH | Two entry types mirror Docker Hub concepts: Tool (WDL task) and Workflow (WDL workflow). Each push stores raw WDL bytes, computes SHA-256 content hash (digest), records a version row. Tag is optional — if provided, the tag pointer is updated to the new version. |
| Registry pull by tag or digest | Pull must resolve `name:tag` or `name@sha256:abc` to exact WDL content. Same model as Docker image tags vs digests: tags are mutable pointers, digests are immutable content addresses. | MEDIUM | `GET /projects/{pid}/registry/{name}/versions/{ref}` where `ref` is a tag string or `sha256:hex`. Tags are mutable — pushing a new version with the same tag moves the pointer. Digests are immutable — once a digest exists, it always returns the same bytes. (HIGH confidence — verified Docker registry pattern) |
| Execute a registered workflow/tool by reference | The purpose of a registry is to submit by name rather than filesystem path. | MEDIUM | `POST /projects/{pid}/workflows` accepts `{"registry_ref": "name:tag", "inputs": {...}}` in addition to the existing `{"wdl_path": "...", "inputs": {...}}`. Server resolves ref → WDL bytes, passes via existing `WDLContent` field in `TaskClaimResponse` (v1.2 already supports this path). |
| Provenance record per run | Every scientific workflow platform records what ran, with what inputs, producing which outputs. Cromwell writes per-run metadata JSON; CWLProv uses W3C PROV; Terra records submission history. | MEDIUM | A `provenance` table: `run_id`, `project_id`, `entry_id`, `version_id` (FK to registry_versions — immutable digest), `submitted_by` (user_id), `inputs_snapshot` (JSON), `outputs_snapshot` (JSON — populated on completion), `submitted_at`, `completed_at`. |
| Provenance read endpoints | Users need to audit what ran and reproduce results by referencing the exact WDL version used. | LOW | `GET /projects/{pid}/provenance` (list) and `GET /workflows/{run_id}/provenance` (single). Returns registry name, tag at submission time, digest, inputs snapshot, outputs snapshot, submitting user. |

### Differentiators (Competitive Advantage)

Features that create real value beyond baseline compliance and distinguish flowy from just running WDL files.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Content-hash (digest) addressing for WDL versions | Guarantees exact reproducibility — `name@sha256:abc` always returns byte-identical WDL, unlike mutable tags. Identical to the Docker content-addressable layer model that made container registries trustworthy. | LOW | SHA-256 of the raw WDL bytes. Computed on push, stored as `digest` column. Immutable once stored — same bytes pushed twice is idempotent (UNIQUE on `(entry_id, digest)`). |
| Provenance links digest (not tag) | A run's provenance record stores the immutable `version_id` (which carries the digest), not just the tag string. Tags can be moved; digests cannot. Six months later, an analyst can pull the exact WDL that produced a result. | LOW | `provenance.version_id` is a FK to `registry_versions.id`. That row carries the `digest`. Provenance response includes both the tag string at submission time (recorded in the provenance row) and the digest. |
| Registry entries are project-scoped (not global) | Prevents name collisions across teams. Mirrors Dockstore's organization model and Docker Hub's user/org namespaces. Teams can have their own `align:latest` without conflicting. | LOW | `registry_entries.project_id` FK. Permissions enforced via RBAC: executor+ can push, viewer+ can pull, admin can delete. |
| Execution from registry ref requires no shared FS | Resolving a registry ref to in-memory WDL content lets daemons on remote hosts execute without needing access to the server's filesystem. Already unlocked by v1.2's `WDLContent` field in `TaskClaimResponse`. | LOW | No new mechanism needed — the registry lookup in the submit handler simply sets `WDLContent`. Daemons already handle this path. |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| OAuth2 / LDAP / SSO | Teams want SSO for single sign-on convenience | Enormous scope: external IdP dependency, token exchange flows, PKCE, refresh token rotation. PROJECT.md explicitly defers this. | Username/password + JWT covers v1.3. Internal claims structure stays stable, so OAuth2 can be layered later by having an OAuth2 proxy issue JWTs in the same format. |
| Refresh tokens and token rotation | Production security best practice for Web UIs | Adds a second token type, a token revocation store, and client-side refresh logic. Disproportionate for a batch-workflow API where tokens are scripted (not interactive). | Use reasonably long-lived JWTs (8–24h) for v1.3. Add `POST /auth/refresh` when a Web UI with session UX is added. |
| Semantic versioning enforcement on registry tags | Dockstore and WARP pipelines use semver; users expect it | Enforcing semver at the API level (rejecting non-semver tags) breaks scripting, CI pipelines, and any workflow that tags by git SHA. | Accept any tag string. Document semver as a recommended convention. The engine has no semantic need to parse version numbers. |
| Per-task provenance (W3C PROV Entity-Activity-Agent graph) | CWLProv and academic publishing workflows want fine-grained lineage at the task level | The task table already has inputs/outputs per task; building a full W3C PROV graph is a research-grade feature that is 3–5x the scope of run-level provenance. | Run-level provenance (what workflow version, which inputs, which outputs, who submitted) covers 95% of audit and reproducibility needs. Per-task lineage is a future milestone. |
| Global (cross-project) registry namespace | Simpler: one place to find all WDL tools | Name collisions, access control complexity, accidental overwrites across teams. Dockstore learned this and added org/namespace scoping. | Project-scoped registry. A future `is_public` flag can make a project's registry readable globally without merging namespaces. |
| Real-time execution events (WebSocket / SSE) | Nicer UX for long-running workflows | PROJECT.md explicitly defers WebSocket/SSE. Polling is sufficient for batch workflows. | Polling `GET /workflows/{id}/status`. SSE is a future milestone when a Web UI is added. |
| PostgreSQL migration for new tables | Scale concerns | The existing system already uses SQLite with WAL and has 100% spectool conformance. Adding tables to SQLite is straightforward with goose. PostgreSQL is a major dependency addition for no current benefit. | SQLite with goose migrations. Revisit when concurrent write load or team size justifies it. |

---

## Feature Dependencies

```
User Auth (users table, bcrypt, login endpoint, JWT middleware)
    └──required by──> All protected API routes (existing + new)
    └──required by──> Project membership (members reference user IDs)
    └──required by──> Provenance "submitted_by" field

Projects (projects table, CRUD endpoints)
    └──required by──> Project RBAC (roles are per-project)
    └──required by──> Registry entries (project-scoped)
    └──required by──> Provenance (project_id FK)
    └──required by──> Runs (project_id FK on existing runs table)

Project RBAC (project_members table, role middleware)
    └──required by──> Registry push/delete (executor+ only)
    └──required by──> Run submission within project context
    └──required by──> Provenance read (viewer+)

WDL Registry (registry_entries + registry_versions tables, push/pull endpoints)
    └──required by──> Execution from registry ref
    └──required by──> Provenance (version_id FK)

Execution from registry ref
    └──enhances──> Existing run submission (POST /workflows)
    └──depends on──> WDL Registry (ref → WDL content lookup)
    └──depends on──> Projects (run associated with project)
    └──uses──> Existing WDLContent path in TaskClaimResponse (v1.2)

Provenance Tracking (provenance table, write on submit, update on completion)
    └──depends on──> Existing runs table (run_id FK)
    └──depends on──> WDL Registry (version_id FK → immutable digest)
    └──depends on──> User Auth (submitted_by FK → user_id)
    └──depends on──> Projects (project_id FK)
```

### Dependency Notes

- **User auth is the foundation.** Every other feature enforces access via the JWT user identity. Auth must be buildable and testable before RBAC middleware is wired.
- **Projects require auth.** `project_members` rows reference `users.id`. Projects can be scaffolded once users exist.
- **Registry requires projects.** Registry entries carry `project_id`. The registry push endpoint must enforce RBAC (executor role), so project membership must be queryable first.
- **Provenance requires registry, auth, and projects.** The provenance row needs `version_id` (from registry), `submitted_by` (from auth), and `project_id` (from projects). All three must exist.
- **Execution from registry ref requires registry but not provenance.** Provenance is written alongside execution; the execution path itself only needs the WDL content resolved from the registry.
- **Existing `runs` table gets `project_id` (nullable).** Pre-v1.3 runs (no project) are represented as `project_id = NULL`. New project-scoped submissions require a non-null `project_id`.

---

## MVP Definition

### Launch With (v1.3)

Minimum viable set for the milestone to be complete and useful.

- [ ] `users` table + bcrypt password hash + seed admin on first run — foundation for all auth
- [ ] `POST /auth/login` returning a signed JWT (golang-jwt/jwt/v5) — entry point
- [ ] JWT middleware (go-chi/jwtauth/v5) protecting all existing and new routes — security boundary
- [ ] `POST /users` and `GET /users` (admin role only) — user lifecycle management
- [ ] `projects` table + `POST /projects`, `GET /projects`, `GET /projects/{id}` — isolation boundary
- [ ] `project_members` table + `POST /projects/{id}/members`, `GET /projects/{id}/members` — RBAC foundation
- [ ] RBAC middleware enforcing admin/executor/viewer per project on relevant routes — access control
- [ ] `registry_entries` + `registry_versions` tables with SHA-256 digest on WDL content — core registry schema
- [ ] `POST /projects/{pid}/registry/{name}/versions` (push, executor+) — registry write
- [ ] `GET /projects/{pid}/registry/{name}/versions/{ref}` (pull by tag or digest, viewer+) — registry read
- [ ] Run submission by registry ref: `POST /projects/{pid}/workflows` accepting `registry_ref` — wires registry to execution
- [ ] `provenance` table + write on project-scoped run submission, update outputs on completion — audit trail
- [ ] `GET /projects/{pid}/provenance` and `GET /workflows/{run_id}/provenance` — provenance read

### Add After Validation (v1.x)

- [ ] `is_public` flag on registry entries — add when cross-project sharing is requested
- [ ] `POST /auth/refresh` (refresh token) — add when a Web UI with session UX is introduced
- [ ] Per-task provenance (task-level lineage) — add when compliance requirements demand it
- [ ] `DELETE /projects/{pid}/registry/{name}` with cascade — add when registry cleanup becomes a pain point

### Future Consideration (v2+)

- [ ] OAuth2 / SSO integration — defer until enterprise customers require it
- [ ] W3C PROV export (CWLProv-style research objects) — defer until academic publishing workflow is needed
- [ ] GA4GH TRS API compatibility — defer until interoperability with Dockstore/Terra is required
- [ ] PostgreSQL storage backend — defer until SQLite write contention becomes measurable

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| User auth (login + JWT middleware) | HIGH | LOW | P1 |
| Projects CRUD | HIGH | LOW | P1 |
| Project RBAC (3 roles) | HIGH | MEDIUM | P1 |
| WDL registry push (with SHA-256 digest) | HIGH | MEDIUM | P1 |
| WDL registry pull (tag + digest resolution) | HIGH | LOW | P1 |
| Execution from registry ref | HIGH | MEDIUM | P1 |
| Run-level provenance write | HIGH | MEDIUM | P1 |
| Provenance read endpoints | HIGH | LOW | P1 |
| Public registry entries flag | MEDIUM | LOW | P2 |
| Refresh tokens | LOW | MEDIUM | P3 |
| Per-task provenance | LOW | HIGH | P3 |
| OAuth2/SSO | LOW | HIGH | P3 |

**Priority key:** P1 = must have for v1.3 launch, P2 = next minor release, P3 = future milestone

---

## Competitor Feature Analysis

| Feature | Terra / Cromwell-as-a-service | Dockstore | Our Approach |
|---------|-------------------------------|-----------|--------------|
| Authentication | Google OAuth2 / workspace login | GitHub OAuth2 | Username/password + JWT — no external IdP, self-hosted friendly |
| Project/workspace isolation | Workspace per project, Google IAM backend | Organization + collection scoping | `projects` table with `project_members` rows and per-project roles |
| RBAC roles | Owner / Writer / Reader (3 tiers) | Admin / Maintainer / Member / Reader (4 tiers) | Admin / Executor / Viewer (3 tiers — maps directly to workflow engine operations) |
| Workflow registry | Uses Dockstore or GitHub for registry | Registers Docker+WDL/CWL/Nextflow; GA4GH TRS API | Project-scoped WDL-only registry with SHA-256 content-hash versioning |
| Versioning model | Semver + GitHub tags; Terra resolves Docker digest before run | Tool versions tied to GitHub repo tags; DOI per version | Content-hash (SHA-256 of WDL bytes) + mutable tags pointing to a version row |
| Immutable addressing | Docker image digest resolved at job submission | Version ID stable once created | `sha256:hex` digest stable and immutable; tag is a mutable pointer |
| Provenance / metadata | Cromwell metadata API: run-level inputs/outputs, call-level stdout/stderr | DOI per version; no run-level execution provenance | `provenance` table: run → registry version digest → inputs snapshot → submitting user → outputs snapshot |
| Per-task lineage | Cromwell stores call-level inputs/outputs/returncode per task | Not applicable | Deferred; existing `tasks` table already has `inputs_json`/`outputs_json` per task |

---

## Schema Sketch

New tables required (goose migrations):

```sql
-- Migration 00004
CREATE TABLE users (
    id           TEXT PRIMARY KEY,  -- UUID v4
    username     TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,    -- bcrypt hash
    global_role  TEXT NOT NULL DEFAULT 'user',  -- 'admin' | 'user'
    created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Migration 00005
CREATE TABLE projects (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL UNIQUE,
    description  TEXT NOT NULL DEFAULT '',
    created_by   TEXT NOT NULL REFERENCES users(id),
    created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE project_members (
    project_id  TEXT NOT NULL REFERENCES projects(id),
    user_id     TEXT NOT NULL REFERENCES users(id),
    role        TEXT NOT NULL,  -- 'admin' | 'executor' | 'viewer'
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (project_id, user_id)
);

-- Migration 00006
CREATE TABLE registry_entries (
    id           TEXT PRIMARY KEY,
    project_id   TEXT NOT NULL REFERENCES projects(id),
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    entry_type   TEXT NOT NULL,  -- 'tool' | 'workflow'
    created_by   TEXT NOT NULL REFERENCES users(id),
    created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE (project_id, name)
);

CREATE TABLE registry_versions (
    id          TEXT PRIMARY KEY,
    entry_id    TEXT NOT NULL REFERENCES registry_entries(id),
    digest      TEXT NOT NULL,   -- 'sha256:' + hex(SHA256(wdl_content))
    wdl_content TEXT NOT NULL,   -- raw WDL source bytes
    created_by  TEXT NOT NULL REFERENCES users(id),
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE (entry_id, digest)    -- same content pushed twice is idempotent
);

CREATE TABLE registry_tags (
    entry_id    TEXT NOT NULL REFERENCES registry_entries(id),
    tag         TEXT NOT NULL,
    version_id  TEXT NOT NULL REFERENCES registry_versions(id),
    updated_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (entry_id, tag)  -- tag is mutable: upsert to move tag
);

-- Migration 00007
-- Add project_id to existing runs table (nullable for backward compat)
ALTER TABLE runs ADD COLUMN project_id TEXT REFERENCES projects(id);

CREATE TABLE provenance (
    id               TEXT PRIMARY KEY,
    run_id           TEXT NOT NULL REFERENCES runs(id),
    project_id       TEXT NOT NULL REFERENCES projects(id),
    entry_id         TEXT NOT NULL REFERENCES registry_entries(id),
    version_id       TEXT NOT NULL REFERENCES registry_versions(id),
    tag_at_submit    TEXT,       -- tag string at submission time; NULL if submitted by digest
    submitted_by     TEXT NOT NULL REFERENCES users(id),
    inputs_snapshot  TEXT NOT NULL,  -- JSON snapshot of inputs at submission
    outputs_snapshot TEXT,           -- JSON snapshot of outputs; NULL until run completes
    submitted_at     DATETIME NOT NULL DEFAULT (datetime('now')),
    completed_at     DATETIME
);
```

The `registry_tags` table is split from `registry_versions` to make the tag-pointer-update an upsert
(`INSERT OR REPLACE`) rather than an UPDATE that could conflict with the immutability invariant of
`registry_versions`. This mirrors the Docker registry's separation of manifest (immutable) from tag
(mutable pointer).

---

## Sources

- [GA4GH Tool Registry Service (TRS) — data model and versioning](https://ga4gh.github.io/tool-registry-service-schemas/DataModel/)
- [Dockstore — workflow versioning and GA4GH TRS API](https://docs.dockstore.org/en/develop/dockstore-introduction.html)
- [Terra — how pipeline versioning works (Docker digest at submit)](https://support.terra.bio/hc/en-us/articles/360037484111-How-does-pipeline-versioning-work)
- [Docker tagging best practices — immutable digest vs mutable tag model](https://stevelasker.blog/2018/03/01/docker-tagging-best-practices-for-tagging-and-versioning-docker-images/)
- [How to handle JWT authentication securely in Go — golang-jwt/jwt/v5](https://oneuptime.com/blog/post/2026-01-07-go-jwt-authentication/view)
- [go-chi/jwtauth/v5 — chi-native JWT middleware package](https://pkg.go.dev/github.com/go-chi/jwtauth/v5)
- [Role-based access control in Golang with jwt-go and chi](https://dev.to/bensonmacharia/role-based-access-control-in-golang-with-jwt-go-ijn)
- [Protect your Chi API with RBAC and JWT validation](https://docs.logto.io/api-protection/go/chi)
- [CWLProv — W3C PROV-based interoperable workflow provenance](https://academic.oup.com/gigascience/article/8/11/giz095/5611001)
- [FAIR cookbook — provenance information for scientific workflows](https://fairplus.github.io/the-fair-cookbook/content/recipes/reusability/provenance.html)
- [Cromwell metadata API — run-level provenance pattern](https://cromwell.readthedocs.io/en/stable/CommandLine/)
- flowy source `internal/api/types.go` — `WDLContent` field in `TaskClaimResponse` already enables in-memory WDL delivery to daemons (HIGH confidence — direct inspection)
- flowy source `internal/store/sqlite/migrations/` — existing schema; new migrations must extend it (HIGH confidence — direct inspection)

---

*Feature research for: flowy v1.3 — user auth, project RBAC, WDL registry, provenance tracking*
*Researched: 2026-03-17*
