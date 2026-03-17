# Project Research Summary

**Project:** flowy — WDL 1.2 workflow engine (Go)
**Domain:** Multi-user bioinformatics workflow engine — user auth, project RBAC, WDL registry, provenance tracking (v1.3)
**Researched:** 2026-03-17
**Confidence:** HIGH

## Executive Summary

flowy v1.3 adds four interconnected features to an existing, working WDL workflow engine: user authentication, project-scoped RBAC, a WDL content-addressed registry, and run provenance tracking. The codebase already ships a solid foundation — chi v5 router, SQLite/WAL with goose migrations, a `Store` interface with injected dependencies, and an S3 storage backend — so v1.3 is a layered addition, not a rewrite. The recommended approach is sequential: auth first, projects and RBAC second, WDL registry third, registry-based execution fourth, and provenance as a capstone that ties all prior layers together. Every feature from v1.3 is either blocked by auth or blocks provenance, so the dependency chain drives the phase order unambiguously.

The key technical decisions are already settled. No substantial new third-party libraries are required: `github.com/golang-jwt/jwt/v5` (one new direct dep, v5.3.1 January 2026), `golang.org/x/crypto/bcrypt` (already transitive via Docker SDK), and `github.com/opencontainers/go-digest` (already transitive) cover the full stack. RBAC is implemented as a ~50-line hand-rolled chi middleware — the static three-role system (admin/executor/viewer) does not warrant Casbin or any external RBAC library. WDL source is stored as `TEXT` directly in SQLite rather than in S3, because WDL files are small (typically < 50 KB) and keeping them in the database removes cross-service dependencies and allows registry to work in local-only deployments. The content-addressed registry follows the Docker distribution model: blobs are immutable and addressed by SHA-256 digest; tags are mutable pointers to a blob version.

The primary risks are architectural and must be resolved before writing any handler code. The highest-risk decision is daemon authentication: the server has two distinct clients (human users and automated daemons), and adding JWT middleware without first structuring the router into separate sub-groups will silently break all task execution. The second critical risk is provenance granularity — run-level provenance cannot later answer "which tool version produced this file?" if a workflow calls multiple registered tools, and schema migrations to fix this are classified as HIGH recovery cost. Both pitfalls have documented avoidance strategies and are addressed directly by the phase ordering recommended below.

## Key Findings

### Recommended Stack

The v1.3 stack adds minimally on top of the existing v1.2 foundation. All auth and registry capabilities are covered by the existing dependency graph plus one new direct dependency. The `golang-jwt/jwt/v5` library (v5.3.1, January 28, 2026) is the maintained successor to the archived `dgrijalva/jwt-go`; it has zero runtime dependencies and is used by 13,000+ Go modules. RBAC is hand-rolled — three static roles at compile time do not justify a policy DSL. The content digest format (`sha256:<hex>`) reuses `github.com/opencontainers/go-digest`, already present via the Docker SDK. The key rejection is `go-chi/jwtauth/v5`, which depends on the full `lestrrat-go/jwx` JOSE implementation — 50x the needed surface area for HS256 token sign/verify.

See `.planning/research/STACK.md` for full library comparison, rejected alternatives, and exact API usage patterns.

**Core technologies:**
- `github.com/golang-jwt/jwt/v5` v5.3.1: JWT sign/verify — zero deps, HS256, maintained community successor to dgrijalva
- `golang.org/x/crypto/bcrypt` (already in go.mod): password hashing — cost 12 (~300ms), Go team maintained
- `github.com/opencontainers/go-digest` (already in go.mod): SHA-256 content digest — canonical `sha256:<hex>` format
- Hand-rolled `internal/auth/rbac.go` (~50 lines): three-role RBAC — no Casbin, no gorbac, static enum
- `crypto/sha256` (stdlib): WDL content hashing — no external dep for content addressing

### Expected Features

See `.planning/research/FEATURES.md` for full feature table, schema sketch, competitor analysis (Terra, Dockstore, Cromwell), and complete feature dependency graph.

**Must have (table stakes) — all P1 for v1.3:**
- Username/password login returning a signed JWT — entry point for all auth
- JWT middleware protecting all non-public routes — without this, auth is theater
- User creation and listing (admin-only) — lifecycle management
- Projects as isolation boundaries — every team workflow platform requires this
- Three-tier RBAC per project (admin/executor/viewer) — maps directly to workflow engine operations
- WDL registry push with SHA-256 content digest — core registry write
- WDL registry pull by tag or digest — tag is mutable, digest is immutable
- Execute a registered workflow/tool by registry ref — the purpose of the registry
- Run-level provenance record — audit trail linking run to registry version, inputs, submitting user
- Provenance read endpoints — reproducibility query surface

**Should have (differentiators):**
- Content-hash (digest) addressing guaranteeing byte-identical WDL retrieval
- Provenance stores digest, not tag — tag can be moved, digest cannot
- Registry entries project-scoped — no cross-team name collisions
- Registry-ref execution requires no shared filesystem — already unlocked by v1.2 `WDLContent` field in `TaskClaimResponse`

**Defer (v1.x or v2+):**
- OAuth2/SSO — enormous scope, no current requirement; username/password + JWT is stable and extendable
- Refresh token rotation — disproportionate for batch workflow API used via scripting, not interactive sessions
- Per-task provenance (W3C PROV graph) — research-grade, 3-5x the scope of run-level provenance
- PostgreSQL migration — no measurable write contention at current scale; SQLite WAL is sufficient
- GA4GH TRS API compatibility — defer until Dockstore/Terra interoperability is explicitly requested

### Architecture Approach

v1.3 follows an additive architecture: three new internal packages (`internal/auth/`, `internal/registry/`, `internal/provenance/`) are added, the `store.Store` interface gains four new method groups (user, project, registry, provenance), and the chi router is restructured into explicit sub-routers (public, user-JWT, daemon-token). The WDL parser, storage backend, and daemon execution loop are untouched. The `auth.enabled` config flag (default `false`) ensures all existing spectool conformance tests continue to pass without modification when auth is not configured.

See `.planning/research/ARCHITECTURE.md` for full component map, all five migration scripts, store interface extensions, data flow diagrams, and the seven-step build order.

**Major components:**
1. `internal/auth/` (NEW) — JWT sign/verify, bcrypt hashing, chi middleware for JWT and daemon auth
2. `internal/registry/` (NEW) — SHA-256 content hashing, push/pull logic, tag resolution
3. `internal/provenance/` (NEW) — record construction, linking run completion to registry digest
4. `internal/store/Store` interface (MODIFIED) — extended with user/project/registry/provenance method groups; existing methods untouched
5. `internal/server/server.go` (MODIFIED) — `jwtSecret` field, structured route groups, backwards-compat `auth.enabled` flag

**Build order driven by strict dependencies:**
1. `internal/auth/jwt.go`, `password.go` — no internal deps; unblocks all middleware
2. Store interface + migration 00004 (users) — unblocks auth handlers
3. Auth middleware + config + router restructure — spectool gate: `auth.enabled=false` passes all existing tests
4. Migration 00005 (projects/members) + auth/project handlers + CLI login commands
5. Migration 00006 (registry) + `internal/registry/` + registry handlers + CLI registry commands
6. Migrations 00007-00008 (runs project linkage, provenance) + `internal/provenance/` + dispatch integration
7. Daemon token auth — minimal change to `daemon.go` and config

### Critical Pitfalls

See `.planning/research/PITFALLS.md` for full detail including recovery costs, verification checklists, integration gotchas, and security mistakes.

1. **Daemon endpoints left unauthenticated when user JWT is added** — design the three sub-router groups (public, user-JWT, daemon-token) in `newRouter()` before writing any JWT code; if daemon routes fall under JWT middleware, all task execution silently stops with runs stuck in PENDING; verified as the top auth retrofit failure mode
2. **Object-level authorization (BOLA) missing** — JWT verification confirms authentication but not authorization; every handler that fetches a resource by UUID must verify `resource.project_id` matches the caller's project membership; OWASP API Security Top 10 #1 risk; cannot be fixed solely with middleware
3. **Mutable tag overwrites breaking reproducibility** — implement `UNIQUE(entry_id, tag)` constraint at migration time and return HTTP 409 on re-tag; always store the resolved SHA-256 digest (not the tag string) in provenance records; silent overwrite is how bioinformatics tool versioning breaks downstream reproducibility
4. **Store interface bypassed with raw SQL in handlers** — establish the `store.Store` extension pattern with the very first User entity in Phase 1; no `*sql.DB` references or SQL string literals belong in handler files; this pattern prevents fragmented query logic and enables testing
5. **SQLite write transaction held open during expensive operations** — compute SHA-256 of WDL content before opening a write transaction; never hold a write lock across CPU-intensive work; long transactions block daemon heartbeats under concurrent load, causing tasks to be incorrectly marked stale

## Implications for Roadmap

Based on combined research, the feature dependency chain is linear and non-negotiable: auth enables projects, projects enable RBAC, RBAC guards the registry, and the registry enables provenance. No phase can be reordered without breaking a downstream dependency.

### Phase 1: User Authentication Foundation

**Rationale:** Auth is the prerequisite for every other v1.3 feature. JWT claims carry `user_id` and `role` that all subsequent phases read from request context. The route group restructure must happen here to permanently avoid the daemon authentication pitfall — it cannot be deferred to "after auth works."
**Delivers:** `users` table (migration 00004), `POST /auth/login`, `GET /auth/whoami`, `POST /users` (admin), `internal/auth/` package (jwt.go, password.go, middleware.go, rbac.go), JWT middleware on user routes, daemon shared-secret middleware on daemon routes, `~/.flowy-token` management in CLI, startup validation rejecting empty or short `FLOWY_JWT_SECRET`.
**Addresses:** Username/password login, JWT session, user lifecycle management (table stakes).
**Avoids:** Pitfall 1 (daemon auth — route groups defined here), Pitfall 6 (JWT secret in source — env var required at startup), Pitfall 7 (bcrypt cost — named constant `bcryptCost = 12`), Pitfall 8 (store interface pattern established with first entity).

### Phase 2: Projects and RBAC

**Rationale:** Projects are the isolation boundary that all registry and provenance features require. RBAC middleware must exist before any project-scoped resource endpoint is written, or object-level authorization gaps will be retrofitted rather than designed in.
**Delivers:** `projects` and `project_members` tables (migration 00005), `POST/GET /projects`, `GET /projects/{id}`, `POST /projects/{id}/members`, `DELETE /projects/{id}/members/{uid}`, `RequireProjectRole` chi middleware applied to route groups, `project_id` nullable FK added to `runs` table (migration 00007 partial).
**Addresses:** Projects as isolation boundaries, three-tier RBAC per project (table stakes).
**Avoids:** Pitfall 2 (BOLA — every resource gets `project_id` FK before any resource handler is written).

### Phase 3: WDL Registry

**Rationale:** Registry is the core deliverable of v1.3. Requires users (`pushed_by` FK) and projects (`project_id` FK). The three-table schema (registry_entries/registry_versions/registry_tags) must be designed before any push endpoint is implemented to avoid the content-hash type collision and mutable tag overwrite pitfalls.
**Delivers:** `registry_entries`, `registry_versions`, `registry_tags` tables (migration 00006), push endpoint (`POST /projects/{pid}/registry`, executor+), pull by tag and digest (`GET /projects/{pid}/registry/{name}/{ref}`, viewer+), list endpoints, CLI `registry push/pull/list` subcommands, idempotent re-upload of identical WDL content (UNIQUE on `(entry_id, digest)`), SHA-256 computed before transaction opens.
**Addresses:** WDL registry push/pull with SHA-256 content digest, digest-based immutable addressing (table stakes + differentiator).
**Avoids:** Pitfall 3 (content hash type collision — three-table schema separates blobs from named entries), Pitfall 4 (mutable tags — UNIQUE constraint + HTTP 409 on re-tag), Pitfall 9 (SQLite writer contention — hash computed before write transaction).

### Phase 4: Registry-Based Execution

**Rationale:** Registry ref submission connects the registry to the existing execution engine. Requires registry (ref resolution) and projects (executor role check). The `WDLContent` field in `TaskClaimResponse` already exists from v1.2, so the daemon side requires zero changes — this phase is primarily wiring.
**Delivers:** Registry ref resolution in submit handler, `registry_ref` column on `runs` table (migration 00007 remainder), `POST /projects/{pid}/workflows` accepting `{"registry_ref": "name:tag", "inputs": {...}}` alongside existing `wdl_path` submission, CLI `--registry ref` flag on submit/run commands, backward-compatible `wdl_path` submission continues to work unchanged.
**Addresses:** Execute registered workflow/tool by registry ref (table stakes).
**Uses:** Existing `WDLContent` field in `TaskClaimResponse` (v1.2 mechanism, no daemon changes required).

### Phase 5: Provenance Tracking

**Rationale:** Provenance is the capstone feature. It requires a completed run (dispatch.go integration), a registry version (`version_id` FK), a user identity (`submitted_by` FK), and a project (`project_id` FK). All four prior phases must be complete. The schema granularity decision must be made at schema design time — retrofitting task-level granularity after deployment is a HIGH-cost recovery.
**Delivers:** `provenance` table (migration 00008), provenance record written in `dispatch.go` after successful run completion (non-fatal — logs on error, does not fail the run), `GET /projects/{pid}/provenance` (list), `GET /projects/{pid}/provenance/{id}` (single), digest stored in record (not just tag string at submission time), provenance daemon token auth (step 7 of build order).
**Addresses:** Run-level provenance write and read, digest-linked provenance (table stakes + differentiator).
**Avoids:** Pitfall 5 (provenance granularity — schema references `version_id` carrying an immutable digest, not a mutable tag; both tag-at-submit and resolved digest are stored).

### Phase Ordering Rationale

- Auth must come first: JWT claims carry `user_id` that every downstream feature reads from context; no project membership, registry push, or provenance record can exist without a user identity.
- Projects before registry: `registry_entries.project_id` is a NOT NULL FK; registry cannot exist without projects; RBAC must be in place before registry write endpoints go live.
- Registry before execution-from-ref: The submit handler must resolve `registry_ref` to WDL content before it can create a run.
- Registry before provenance: `provenance.version_id` is a FK into `registry_versions`; provenance rows cannot be written without the registry.
- Daemon auth wired in Phase 1 because the route group restructure enabling daemon-token middleware must happen before any JWT middleware is deployed. This is the single highest-risk ordering constraint — getting it wrong causes silent, total execution failure.

### Research Flags

Phases with standard, well-documented patterns (likely no need for `/gsd:research-phase`):
- **Phase 1 (Auth):** chi middleware, golang-jwt/jwt v5, bcrypt — thoroughly documented; exact implementation spelled out in ARCHITECTURE.md including full token design and rbac.go implementation.
- **Phase 2 (Projects + RBAC):** Standard CRUD with chi route groups and RBAC middleware — no novel patterns; ARCHITECTURE.md has the full router structure.
- **Phase 4 (Registry Execution):** Reuses existing `WDLContent` path — primarily wiring, minimal new design surface.

Phases that benefit from deeper review before implementation:
- **Phase 3 (WDL Registry):** The three-table blob/entry/tag schema has subtle correctness requirements (idempotent re-upload, tag immutability invariant, UNIQUE constraint semantics in SQLite with upsert). Review PITFALLS.md Pitfalls 3 and 4 carefully before schema finalization. The distinction between FEATURES.md schema (separate `registry_versions` table with UUID PK) and ARCHITECTURE.md schema (content hash as PK on `registry_entries`) must be resolved to a single canonical design before migrations are written.
- **Phase 5 (Provenance):** Granularity decision (run-level vs task-level) has HIGH recovery cost if wrong. The PITFALLS.md "Looks Done But Isn't" checklist should be used as the acceptance criterion. Decision: run-level provenance linking to a single workflow registry entry is the v1.3 scope; task-level lineage is v2.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All libraries verified at pkg.go.dev; two are already in go.mod as transitive deps; golang-jwt/jwt v5.3.1 published January 28, 2026 — confirmed current |
| Features | HIGH | Auth/RBAC patterns HIGH (official docs, production patterns); registry design HIGH (Docker distribution spec is the canonical reference); provenance schema MEDIUM-HIGH (run-level well-established; task-level granularity adds implementation complexity deferred to v2) |
| Architecture | HIGH | Based on direct codebase analysis of all files in `internal/server/`, `internal/store/`, `internal/api/`, `internal/config/`, `internal/daemon/`, `cmd/flowy/main.go` |
| Pitfalls | HIGH | Auth/RBAC pitfalls from OWASP and production experience; registry pitfalls from Docker distribution spec and CAS literature; provenance pitfalls from bioinformatics workflow community (CWLProv, Seqera); SQLite contention from official SQLite WAL docs and production post-mortems |

**Overall confidence:** HIGH

### Gaps to Address

- **Registry schema canonical version:** FEATURES.md and ARCHITECTURE.md describe slightly different registry schemas. FEATURES.md uses a separate `registry_versions` table with a UUID primary key (more normalized, supports future metadata per version). ARCHITECTURE.md uses `content_hash` as the primary key directly on `registry_entries` (simpler, fewer joins). The roadmap plan for Phase 3 must pick one and commit before migrations are written. ARCHITECTURE.md's content-hash-as-PK approach is simpler; FEATURES.md's version-table approach is more extensible. Recommendation: use the FEATURES.md three-table schema (`registry_entries`, `registry_versions`, `registry_tags`) as it maps more cleanly to the Docker distribution spec.
- **Admin seed on first run:** FEATURES.md lists `POST /users` as admin-only, which creates a chicken-and-egg problem. Either a migration seeds a default admin user with a configurable password, or a `flowy-server --init-admin` CLI flag is provided. This decision must be explicit in Phase 1 planning before any implementation begins.
- **Backward-compatible route transition:** When `auth.enabled: true`, the existing unauthenticated routes (`POST /workflows`, `GET /workflows/{id}/status`) must still pass spectool conformance. The `auth.enabled` config flag handles this, but the transition path for existing API clients needs to be documented in the Phase 1 plan.

## Sources

### Primary (HIGH confidence)

- Direct codebase analysis (`internal/server/`, `internal/store/`, `internal/api/`, `internal/config/`, `internal/daemon/`, `cmd/flowy/`) — architecture and integration points
- [golang-jwt/jwt/v5 pkg.go.dev](https://pkg.go.dev/github.com/golang-jwt/jwt/v5) — v5.3.1 API, `WithValidMethods`, `RegisteredClaims`
- [golang.org/x/crypto/bcrypt pkg.go.dev](https://pkg.go.dev/golang.org/x/crypto/bcrypt) — Go team maintained, already in go.mod
- [Docker Distribution Specification](https://distribution.github.io/distribution/spec/api/) — immutable digest vs mutable tag semantics
- [chi v5 middleware documentation](https://go-chi.io/#/pages/middleware) — route groups, middleware ordering
- [OWASP API Security Top 10 — BOLA](https://owasp.org/API-Security/editions/2023/en/0xa1-broken-object-level-authorization/) — object-level authorization is the #1 API security risk
- [SQLite Write-Ahead Logging](https://sqlite.org/wal.html) — single writer, WAL semantics
- [GA4GH Tool Registry Service (TRS) data model](https://ga4gh.github.io/tool-registry-service-schemas/DataModel/) — registry versioning reference
- [golang-jwt/jwt v5.3.1 release — January 28, 2026](https://pkg.go.dev/github.com/golang-jwt/jwt/v5) — confirmed current maintained fork

### Secondary (MEDIUM confidence)

- [Dockstore introduction](https://docs.dockstore.org/en/develop/dockstore-introduction.html) — organization/collection scoping model, four-tier role reference
- [Terra pipeline versioning](https://support.terra.bio/hc/en-us/articles/360037484111) — Docker digest resolved at submit; three-tier role reference
- [CWLProv — W3C PROV workflow provenance](https://academic.oup.com/gigascience/article/8/11/giz095/5611001) — task-level granularity requirement for scientific reproducibility
- [Seqera Community — provenance discussion](https://community.seqera.io/t/provenance-or-audit-trail-of-computation/1174) — bioinformatics community expectations for provenance
- [10 RBAC Best Practices — Oso](https://www.osohq.com/learn/rbac-best-practices) — scoped roles, role proliferation anti-patterns
- [Role-based access control in Golang with jwt-go and chi](https://dev.to/bensonmacharia/role-based-access-control-in-golang-with-jwt-go-ijn) — chi middleware RBAC pattern

### Tertiary (LOW confidence)

- CNCF 2024 survey (via PITFALLS.md) — "65% of organizations experienced breakages from mutable tags" — directional, validates immutable tag enforcement requirement
- [Cromwell SecurityRecommendations.md](https://cromwell.readthedocs.io/en/stable/CommandLine/) — "Cromwell is NOT on its own a security appliance" — confirms auth must be layered explicitly on workflow engines

---
*Research completed: 2026-03-17*
*Ready for roadmap: yes*
