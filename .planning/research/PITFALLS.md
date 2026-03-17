# Pitfalls Research

**Domain:** Adding user auth, project RBAC, WDL registry, and provenance tracking to existing Go workflow engine
**Researched:** 2026-03-17
**Confidence:** HIGH (auth/RBAC pitfalls from production experience and official docs; registry pitfalls from Docker registry spec and CAS literature; provenance pitfalls from bioinformatics workflow community)

---

## Critical Pitfalls

### Pitfall 1: Daemon Endpoints Left Unauthenticated When Auth Is Added to User Endpoints

**What goes wrong:**
The developer adds JWT middleware to user-facing routes (`POST /workflows`, `GET /workflows/{id}/status`, etc.) but forgets that the daemon also calls the server (`POST /tasks/claim`, `POST /tasks/{id}/result`, `POST /heartbeat`). After auth is deployed, existing daemons start returning 401 and all task execution silently stops. The server processes incoming work but no daemon can claim tasks, so runs get stuck in PENDING forever.

**Why it happens:**
The server has two distinct clients — human users and automated daemons. When auth is retrofit on top of an existing unauthenticated API, the natural instinct is to protect "user endpoints." Daemon endpoints look like an internal implementation detail and get overlooked. Flowy's current router (see `internal/server/server.go`) does not separate daemon routes from user routes into different middleware groups — all routes live in one chi.Mux.

**How to avoid:**
Design the auth middleware split before writing a single line of JWT code. Split the chi router into three explicit sub-routers: `public` (health, login/register), `user` (JWT required), and `daemon` (separate daemon token or pre-shared key). Create these route groups in `newRouter()` first, wire empty middleware, then implement auth. This forces the question "what auth does this route need?" for every handler rather than allowing it to be deferred.

Use a separate credential type for daemons (a static pre-shared key in `flowy.yaml` or a daemon-specific JWT with a `role: daemon` claim) rather than re-using user JWT tokens. The daemon is a machine client and rotating user sessions should not break daemon connectivity.

**Warning signs:**
- All runs stuck in PENDING after deploying auth changes
- Daemon logs show `401 Unauthorized` on `/tasks/claim`
- Server logs show zero task claim attempts

**Phase to address:**
Phase 1 (User Authentication) — define the route group structure before implementing any JWT logic. Never add middleware after routes are defined.

---

### Pitfall 2: Resource Ownership Check Missing — Authorization Without Scope

**What goes wrong:**
JWT verification confirms the user is authenticated but nothing checks that the user owns or has access to the specific resource being accessed. Any authenticated user can `GET /workflows/{id}/outputs` for a run they did not submit, or `POST /registries/{id}/tags` for a tool registered in another project.

This is the most common RBAC mistake: treating authentication as authorization. OWASP API Security Top 10 lists "Broken Object Level Authorization" (BOLA) as the #1 API security risk.

**Why it happens:**
When auth is added to an existing system, the developer focuses on "is this user logged in?" because that is what was missing. The existing handlers already fetch by UUID (`GetRun(ctx, runID)`) and return the resource if found. Adding a JWT middleware makes the handler feel "secured" but the handler body still returns the resource to anyone with a valid token.

**How to avoid:**
Every handler that fetches a resource must perform a two-step check:
1. Authenticate: valid JWT present (middleware)
2. Authorize: resource belongs to requester's project AND requester has required role for the operation (handler body)

For flowy, this means every `Run` and `RegistryItem` must have a `project_id` column. Every handler must: (a) extract `project_id` from the JWT claims, (b) load the resource, (c) verify `resource.ProjectID == claimProjectID`, (d) verify the user's role permits the operation. Steps (b)-(d) cannot be in middleware because they require the specific resource.

Define a helper: `func (s *Server) authorizeProjectResource(ctx context.Context, userID string, projectID string, requiredRole Role) error` — call it from every handler.

**Warning signs:**
- Any handler that fetches by UUID without checking project membership
- Handlers that rely solely on middleware for authorization
- `GetRun` called before project membership is verified

**Phase to address:**
Phase 2 (Project Management and RBAC) — enforce at schema design time that every resource has `project_id`. Add authorization helpers before implementing resource endpoints.

---

### Pitfall 3: Content Hash as Primary Key — Collision Between Tool and Workflow

**What goes wrong:**
The registry stores WDL files keyed by SHA-256 content hash. A WDL `task` block and a WDL `workflow` block are stored in the same table. Two different files produce the same hash (SHA-256 collision is theoretically possible but practically improbable). More practically: the schema stores tasks and workflows in the same namespace, so `sha256:abc123` might refer to either a task or a workflow. A `GET /registry/sha256:abc123` request returns the wrong type and the caller parses it as the wrong entity.

The more common real failure: the schema does not distinguish between "same content hash, different name/description" — if two users upload the same WDL content for tools with different display names, the second upload silently reuses the first object's metadata.

**Why it happens:**
Copying the Docker registry model literally: Docker stores image layers by digest only, making all layers interchangeable. WDL tools are not image layers — they have names, descriptions, owners, and types (task vs workflow). The content hash identifies the WDL bytes, not the registry entry.

**How to avoid:**
Separate the content store from the registry index:
- **Content store** (`wdl_blobs` table): `(sha256 TEXT PK, content TEXT NOT NULL, size_bytes INT)` — deduplicated WDL source
- **Registry index** (`registry_items` table): `(id UUID PK, project_id, name, type [task|workflow], sha256 FK, created_by, created_at)` — named, owned entries pointing to blobs
- **Tags** (`registry_tags` table): `(item_id FK, tag TEXT, sha256 FK, created_at)` — mutable pointers, immutable content

This mirrors the Docker distribution spec correctly: digests identify immutable blobs, manifests identify versioned items, tags are mutable references to manifests.

**Warning signs:**
- Single table for registry items using content hash as primary key
- No separate tag/manifest layer
- Schema allows two items with the same hash but different names without distinguishing them

**Phase to address:**
Phase 3 (WDL Registry) — design the three-table schema before writing any registry API code.

---

### Pitfall 4: Mutable Tags Allow Silent Overwrite of Existing Versions

**What goes wrong:**
A user tags a WDL tool as `v1.0`. Later, they upload a new version and tag it `v1.0` again. The old content is silently replaced. Running workflows that referenced `my-tool:v1.0` now execute different code without any record that the tag moved. Reproducibility — a core requirement for bioinformatics — is broken.

This is the exact behavior that `latest` tags in Docker have caused in production. The CNCF 2024 survey found 65% of organizations experienced breakages from mutable tags.

**Why it happens:**
Tags feel like version numbers to users, and version numbers are usually fixed after release. Developers implement tags as a simple key-value update (`UPDATE registry_tags SET sha256 = ? WHERE item_id = ? AND tag = ?`) without any write-once enforcement or audit trail.

**How to avoid:**
Implement two categories of tags explicitly in the API:

1. **Immutable tags** (`v1.0`, `v2.3`, semantic versions): once a sha256 is bound to this tag for this item, the binding cannot be changed. Enforce at the SQL level with a `UNIQUE(item_id, tag)` constraint plus an application-level check that returns HTTP 409 if the tag already exists with a different sha256.

2. **Mutable labels** (`latest`, `stable`, `dev`): explicitly documented as moving pointers; their mutable status is visible in the API response.

Always allow addressing by content hash directly (`my-tool@sha256:abc123`) — this is the canonical immutable reference for provenance.

**Warning signs:**
- Tags implemented as simple UPSERT with no conflict check
- No distinction between "version tag" and "alias tag" in the schema or API
- Provenance records store only the tag name, not the resolved sha256

**Phase to address:**
Phase 3 (WDL Registry) — enforce at schema design (UNIQUE constraint) before any write endpoint is implemented.

---

### Pitfall 5: Provenance Records Written at Wrong Granularity — Run-Level Instead of Task-Level

**What goes wrong:**
Provenance is recorded once per workflow run: "workflow `my-workflow:v1.0` produced these outputs." But a workflow run consists of multiple task executions, each potentially from a different registered tool version. If task `align` came from `bwa-aligner:v2.1` and task `sort` came from `samtools-sort:v1.3`, recording only the workflow-level provenance loses the per-tool traceability. When a user asks "which version of samtools was used to produce `sample.bam`?", the answer is not in the provenance records.

**Why it happens:**
Workflow-level provenance is easier to implement — one record per `Run` in the `runs` table. Task-level provenance requires joining `tasks`, `runs`, and the registry. Developers reach for the simple model first.

**Why it matters for flowy specifically:**
Flowy already stores individual tasks in the `tasks` table. The WDL registry will store individual tools (tasks) and workflows separately. A workflow may call tools from the registry. The natural provenance unit is therefore the task execution, not the run.

**How to avoid:**
Design the provenance schema at task-execution granularity:
```
provenance_records:
  id UUID PK
  run_id FK → runs.id
  task_id FK → tasks.id
  registry_item_id FK → registry_items.id (nullable if not from registry)
  sha256 TEXT NOT NULL  -- content hash at time of execution
  tag TEXT             -- tag used to resolve (nullable if addressed by hash)
  inputs_sha256 TEXT   -- hash of task inputs JSON
  output_files JSON    -- [{path, sha256, size_bytes}]
  executed_at DATETIME
```

The run-level provenance can always be derived by grouping task-level records. The reverse is not true.

**Warning signs:**
- Provenance table has one row per run, not one row per task
- `output_files` stored only at run level
- Provenance schema does not reference the `tasks` table

**Phase to address:**
Phase 5 (Provenance Tracking) — design schema before implementing. Verify with the query "given this output file sha256, which tool version and inputs produced it?" — if the schema cannot answer it, it is insufficient.

---

### Pitfall 6: JWT Secret Stored in Source Code or Config Without Rotation Path

**What goes wrong:**
The JWT signing secret is hardcoded in `flowy.yaml` or, worse, in Go source code. Every token ever issued by the server is now permanently compromised if any deployment's config file leaks. There is no mechanism to invalidate all outstanding tokens without taking the server down.

**Why it happens:**
Getting auth working quickly requires a secret somewhere. The config file is the obvious place. The rotation problem feels like a "future problem."

**How to avoid:**
- Load the JWT secret exclusively from environment variable (`FLOWY_JWT_SECRET`) with a documented fallback to a config file field. Never provide a default value — start-up must fail loudly if the secret is not configured, forcing operators to set it deliberately.
- Use at minimum a 256-bit randomly generated secret for HS256 (`openssl rand -hex 32`).
- Document the token invalidation strategy upfront even if not implemented in v1.3: either short-lived tokens (15 min access + refresh token) or a token version field in the JWT claims that can be incremented to invalidate all tokens for a user.
- Never log the JWT secret. Add a `String() string` method on the config struct that redacts the secret field.

**Warning signs:**
- `cfg.JWT.Secret = "change-me"` as a default value anywhere in code
- JWT secret visible in `flowy.yaml.example`
- No start-up validation that the secret is non-empty and meets minimum length

**Phase to address:**
Phase 1 (User Authentication) — before writing any token issuance code.

---

### Pitfall 7: bcrypt Cost Factor Too Low — Password Hashing Brute-Forceable

**What goes wrong:**
Passwords are hashed with `bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)` where `bcrypt.DefaultCost` is 10. As of 2026, bcrypt cost 10 can be cracked at approximately 100k hashes/sec on a modern GPU. If the SQLite database file is exfiltrated, offline brute-force of weak passwords is feasible within hours.

**Why it happens:**
`bcrypt.DefaultCost` sounds like the right answer. In 2012 when bcrypt was standardized, cost 10 was appropriate. Hardware has improved significantly.

**How to avoid:**
Use `bcrypt.Cost` of 12 as minimum for new deployments in 2026. Document the cost factor as a named constant (`const bcryptCost = 12`) so it can be increased in future migrations. Note: increasing cost on a live system requires rehashing on login — plan for a `password_cost` column in the `users` table to identify which rows need rehashing.

**Warning signs:**
- `bcrypt.DefaultCost` used directly without a named constant
- No `password_cost` column for future migration path
- Cost factor hardcoded as literal integer

**Phase to address:**
Phase 1 (User Authentication) — at password hashing implementation time.

---

### Pitfall 8: Store Interface Not Extended — New Resources Bypass Existing Abstraction

**What goes wrong:**
The existing `store.Store` interface (`internal/store/store.go`) defines methods for `Run` and `Task` only. When users, projects, and registry items are added, developers create ad-hoc SQL functions directly in handler code rather than extending the Store interface. Within two phases, there are SQL queries in handlers, queries in a separate `auth.go` file, and queries in a `registry.go` file — three different query patterns with no consistent transaction handling, no testability, and no interface that could be swapped for a different database.

**Why it happens:**
The Store interface exists but adding to it feels like overhead. It is faster to write a quick `db.QueryRow(...)` directly in the handler.

**How to avoid:**
Every new entity (User, Project, ProjectMembership, RegistryItem, RegistryTag, WDLBlob, ProvenanceRecord) gets methods on the Store interface before any handler uses that entity. Use the existing pattern: add methods to `store.Store`, add a migration in `internal/store/sqlite/migrations/`, implement in `internal/store/sqlite/`. The handler only receives a `store.Store`.

As the store grows, consider sub-interfaces: `store.AuthStore`, `store.RegistryStore`, `store.ProvenanceStore` — each embeddable in `store.Store`. This keeps the interface coherent and allows partial mocking in tests.

**Warning signs:**
- `*sql.DB` passed into handlers rather than `store.Store`
- Raw SQL strings in handler files
- New entity methods not added to `store.Store` interface

**Phase to address:**
Phase 1 (User Authentication) — establish the pattern of store interface extension as the very first change to the codebase.

---

### Pitfall 9: SQLite Writer Contention — Long Transactions Block Auth, Registry, and Execution Simultaneously

**What goes wrong:**
Flowy's SQLite store uses a single-writer connection (`writer.SetMaxOpenConns(1)`) which is correct for SQLite WAL mode. But as new features are added — user sessions, project membership checks, registry uploads, provenance writes — more code paths hold write transactions simultaneously. A large provenance write (many output files) can block a concurrent task status update, causing the daemon heartbeat to time out and tasks to be incorrectly marked stale.

**Why it happens:**
SQLite WAL allows only one writer at a time. Write transactions held open while doing non-DB work (parsing WDL, computing SHA-256 of content, network I/O) block all other writers for the duration. This is invisible in unit tests and in single-user usage but manifests under concurrent execution in multi-daemon deployments.

**How to avoid:**
- Keep write transactions as short as possible: compute SHA-256, validate WDL, etc. **before** opening the transaction; write only the result.
- Never hold a write transaction open across network I/O or expensive CPU work.
- For registry blob uploads: compute hash and store content first, then insert the registry record in a single short transaction.
- Add transaction duration logging in development: log a warning if any write transaction exceeds 100ms.
- The existing `writer.SetMaxOpenConns(1)` + `reader` pool pattern is correct — do not change it, but be disciplined about transaction scope.

**Warning signs:**
- `BEGIN TRANSACTION` followed by WDL parsing or SHA-256 computation
- Write transactions that span HTTP request body reads
- Daemon heartbeat timeouts under concurrent load

**Phase to address:**
Phase 3 (WDL Registry) when the first large content write (WDL blob upload) is implemented.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Single `admin` user hardcoded, no user table | Auth works for demo immediately | Cannot add real users; auth migration is painful | Never — build the user table from day one |
| Roles as enum in JWT claim only (no DB) | No join needed for role check | Cannot change roles without token rotation; role changes take effect only on next login | Acceptable for v1.3 MVP if token TTL is short (≤ 1 hour) |
| Registry tags as simple UPSERT | Tagging code is 3 lines | Silent overwrite breaks reproducibility — core requirement for bioinformatics | Never — immutable version tags must be enforced from day one |
| Provenance as run-level JSON column | One column added to `runs` | Cannot answer "which tool version produced this file?"; requires schema migration to fix | Never — task-level granularity is required for meaningful provenance |
| Daemon bypasses auth entirely (no daemon auth) | Daemons work immediately with auth added | Any process on the network can claim tasks and inject results | Acceptable for v1.3 single-host; document as known gap; block before multi-host deployment |
| Password hashing with `bcrypt.DefaultCost` | Works and passes tests | Brute-forceable if DB leaks | Never — use named constant ≥ 12 from day one |
| Content hash computed on upload, not verified | Less code | Re-uploads can produce different hash for same content if encoding differs | Never — hash must be canonical; normalize WDL content before hashing |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| go-chi/jwtauth | Adding `jwtauth.Verifier` middleware to top-level router, protecting daemon routes | Use sub-routers: `r.Group(func(r chi.Router) { r.Use(jwtauth.Verifier, jwtauth.Authenticator); ... })` for user routes only |
| golang-jwt/jwt/v5 | Using `jwt.Parse` without explicit algorithm validation | Always pass `jwt.WithValidMethods([]string{"HS256"})` to `jwt.NewParser`; prevents algorithm substitution attacks |
| bcrypt | Using `bcrypt.DefaultCost` (10) directly | Define `const bcryptCost = 12`; store cost per user row for future migration |
| goose migrations | Adding a NOT NULL column to existing `runs` table without a DEFAULT | SQLite allows NOT NULL ADD COLUMN only with a default value; always include `DEFAULT` or use two-migration approach (add nullable, backfill, add constraint) |
| chi middleware ordering | Applying middleware after route definitions | In chi, all `r.Use()` calls must come before `r.Get/Post/...` calls within the same router level |
| SQLite WAL single writer | Holding a write transaction open while computing SHA-256 of WDL content | Compute hash before opening transaction; insert result in a short atomic write |

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Project membership check on every request via SQL join | Latency spike under concurrent requests; SQLite reader pool exhausted | Cache membership in JWT claims (re-issued on project role change); short TTL tokens reduce staleness | Under 10+ concurrent authenticated requests |
| Registry listing without pagination | `GET /registry` returns all items; response grows unbounded | Add `limit`/`offset` or cursor pagination from day one; default page size ≤ 100 | When registry exceeds ~500 items |
| SHA-256 of WDL content computed in handler goroutine | Blocks other requests during large uploads | `crypto/sha256` is fast in Go; acceptable for WDL files (rarely > 1MB); no action needed unless profiling shows it as a bottleneck | Practically never for WDL files |
| Provenance output file hashes computed synchronously | Slow task completion acknowledgment for tasks with many outputs | Compute output hashes during daemon delocalization (already has the files); pass hashes in `TaskResult`; server writes provenance in a single transaction | Tasks producing 1000+ output files |
| Full table scan on `registry_items` for name lookup | Slow registry search | Add `CREATE INDEX idx_registry_items_name ON registry_items(project_id, name)` in migration | When registry exceeds ~1000 items per project |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Storing JWT secret in `flowy.yaml` with no rotation path | Full token forgery if config leaks | Load from env var (`FLOWY_JWT_SECRET`); refuse to start if unset; document rotation procedure |
| WDL content stored without size limit | Denial of service via 100MB WDL file upload | Add `Content-Length` check before reading body; reject if > 10MB (WDL files are never this large) |
| Daemon task results accepted without verification of run ownership | Any process can inject fake task results for any run | Include `daemon_id` in `TaskClaimResponse`; verify `task.DaemonID == incoming daemon_id` in `handleResult` |
| SHA-256 content hash treated as a secret | Users assume hash is opaque identifier | It is not secret — SHA-256 of publicly accessible WDL content is computable by anyone. Store project-level access control on registry items, not on the hash itself |
| Tag `latest` in provenance records | Provenance records become unreproducible | Provenance must always store the resolved `sha256` at time of execution, never just the tag name |
| Password in plaintext in request logs | Log scraping reveals credentials | Ensure request body is not logged by chi logger middleware; add explicit `password` field redaction |

---

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Authentication errors return 500 instead of 401/403 | Users cannot distinguish "server crashed" from "not authorized" | Explicit `401 Unauthorized` for missing/invalid token; `403 Forbidden` for valid token but insufficient role |
| `GET /registry` returns all items across all projects | User sees other projects' tools | Always scope registry queries to the authenticated user's accessible projects |
| Registry item addressed only by name, not hash | Running `my-tool:v1.0` after a tag overwrite silently changes behavior | Always echo the resolved sha256 in `POST /registry/{item}/run` response; clients can detect drift |
| No indication of which project a run belongs to | Users cannot find runs in multi-project context | Add `project_id` to `StatusResponse` and `OutputsResponse` from the start |
| Role change takes effect only after token re-issue | Admin demotes a user; user retains admin access until token expires | Document this limitation clearly; use short token TTL (≤ 1 hour) to bound the window |

---

## "Looks Done But Isn't" Checklist

- [ ] **JWT auth on daemon routes:** Verify that `/tasks/claim`, `/tasks/{id}/result`, and `/heartbeat` require appropriate credentials — not user JWTs but also not zero auth.
- [ ] **Object-level authorization:** Verify that `GET /workflows/{id}/outputs` returns 403 (not 404) when the run belongs to a different project.
- [ ] **Immutable tags:** Verify that re-tagging `my-tool:v1.0` to a different sha256 returns HTTP 409, not HTTP 200.
- [ ] **Provenance at task level:** Verify that a workflow with 3 tasks produces 3 provenance records, not 1.
- [ ] **Resolved sha256 in provenance:** Verify that the provenance record stores the sha256 of the WDL content, not just the tag string.
- [ ] **Content hash correctness:** Verify that uploading the same WDL file twice produces the same sha256 and the second upload is a no-op (deduplication).
- [ ] **bcrypt cost:** Verify that password hash strings in the DB start with `$2a$12$` (cost 12), not `$2a$10$` (cost 10).
- [ ] **JWT secret validation at startup:** Verify that the server refuses to start if `FLOWY_JWT_SECRET` is empty or shorter than 32 bytes.
- [ ] **Store interface coverage:** Verify that no handler file contains a raw `*sql.DB` reference or SQL string literal.
- [ ] **Migration reversibility:** Verify that every migration has a corresponding `-- +goose Down` block that correctly reverses the schema change.

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Daemon endpoints left unauthenticated | LOW | Add chi sub-router for daemon routes; no schema change required; redeploy server and daemons |
| Missing object-level authorization | MEDIUM | Add `project_id` FK to `runs` table via migration; audit all handler code; add authorization helper; test with multi-project fixture |
| Mutable tags already deployed | HIGH | Add new `UNIQUE(item_id, tag, sha256)` partial constraint; write migration that logs all tag overwrites as `_overwritten_at_{timestamp}` aliases; document breaking change to API consumers |
| Provenance at run-level, need task-level | HIGH | Add `task_id` column to provenance table (nullable for backward compat); backfill is impossible for historical runs; accept gap for pre-migration data; enforce task-level for all new runs |
| JWT secret leaked | HIGH | Generate new secret; redeploy server (all outstanding tokens instantly invalid); users must re-login; if using refresh tokens, revoke all refresh token records in DB |
| bcrypt cost too low | LOW | Update `bcryptCost` constant; on next login, detect cost mismatch and rehash; existing hashes remain valid until user logs in |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Daemon endpoints left unauthenticated | Phase 1 — define sub-router groups before any middleware | Integration test: daemon claims task without user JWT and succeeds; user JWT on daemon route returns 401 |
| Resource ownership missing | Phase 2 — all resources get `project_id` FK before any handler is written | Test: user A cannot access user B's run; returns 403 not 200 |
| Content hash/type collision in registry | Phase 3 — three-table schema (blobs, items, tags) designed upfront | Schema review: no table uses sha256 as sole primary key for named items |
| Mutable tag overwrites | Phase 3 — UNIQUE constraint on `(item_id, tag)` at migration time | Test: second PUT to same tag returns 409; first sha256 is unchanged |
| Provenance at wrong granularity | Phase 5 — schema references `tasks.id`, not just `runs.id` | Query test: given output file sha256, retrieve the specific task and tool version that produced it |
| JWT secret in source code | Phase 1 — env var loading before first token issuance | Startup test: server exits non-zero if `FLOWY_JWT_SECRET` unset |
| bcrypt cost too low | Phase 1 — named constant defined at implementation time | Test: hash strings in DB contain `$12$` not `$10$` |
| Store interface bypassed | Phase 1 — pattern established with first User entity | Code review gate: no `*sql.DB` in handler package |
| SQLite writer contention | Phase 3 — transaction scope discipline for blob uploads | Load test: concurrent task heartbeats remain < 200ms during registry upload |

---

## Sources

- [Docker Distribution Specification — Content Addressable Storage](https://distribution.github.io/distribution/spec/api/) — immutable digest vs mutable tag semantics
- [OWASP API Security Top 10 — BOLA (Broken Object Level Authorization)](https://owasp.org/API-Security/editions/2023/en/0xa1-broken-object-level-authorization/) — object-level auth is top API security risk
- [golang-jwt/jwt/v5 — Go Packages](https://pkg.go.dev/github.com/golang-jwt/jwt/v5) — current supported algorithms, `WithValidMethods` option
- [go-chi/jwtauth — Go Packages](https://pkg.go.dev/github.com/go-chi/jwtauth/v5) — chi-compatible JWT middleware, sub-router pattern
- [SQLite Write-Ahead Logging — official SQLite docs](https://sqlite.org/wal.html) — single writer, checkpoint starvation
- [Gotchas with SQLite in Production — Anže's Blog](https://blog.pecar.me/sqlite-prod/) — WAL writer blocking patterns
- [Sharing interoperable workflow provenance — GigaScience](https://academic.oup.com/gigascience/article/8/11/giz095/5611001) — CWLProv best practices; task-level granularity requirement
- [Provenance or Audit Trail of computation — Seqera Community](https://community.seqera.io/t/provenance-or-audit-trail-of-computation/1174) — bioinformatics community discussion on provenance requirements
- [10 RBAC Best Practices — Oso](https://www.osohq.com/learn/rbac-best-practices) — scoped roles, role proliferation anti-patterns
- [Image Tag Best Practices — Azure Container Registry](https://learn.microsoft.com/en-us/azure/container-registry/container-registry-image-tag-version) — immutable tags for production, tag vs. digest distinction
- [CVE-2026-29000 pac4j-jwt Authentication Bypass](https://www.codeant.ai/security-research/pac4j-jwt-authentication-bypass-public-key) — algorithm substitution attack pattern; validates need for `WithValidMethods`
- Cromwell SecurityRecommendations.md — "Cromwell is NOT on its own a security appliance" — confirms auth must be layered explicitly

---
*Pitfalls research for: user auth + RBAC + WDL registry + provenance on existing Go workflow engine*
*Researched: 2026-03-17*
