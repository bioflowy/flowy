# Stack Research: WDL Workflow Engine (Go)

**Project:** flowy — WDL 1.2 workflow engine (Go port of miniwdl)
**Researched:** 2026-03-17 (v1.3 update; v1.0–v1.2 stacks preserved below)
**Overall confidence:** HIGH

---

## v1.3 Additions: User Auth, Project RBAC, WDL Registry, Provenance

This section covers NEW libraries needed for the v1.3 milestone.
The v1.0, v1.1, and v1.2 stacks are unchanged — see preserved sections below.

### Core Technologies Added for v1.3

| Layer | Library | Version | Purpose | Why Recommended |
|-------|---------|---------|---------|-----------------|
| **JWT** | `github.com/golang-jwt/jwt/v5` | v5.3.1 | Sign and verify user session tokens | Zero dependencies; focused JWT-only library; HS256 with one secret is all flowy needs; `RegisteredClaims` struct covers exp/sub/iat out of the box |
| **Password hashing** | `golang.org/x/crypto/bcrypt` | v0.49.0 (already transitive) | Hash and verify user passwords | Already in go.mod as transitive dep via Docker SDK; bcrypt is the idiomatic Go choice; cost=12 provides ~300ms verification, acceptable for login |
| **Content digest** | `github.com/opencontainers/go-digest` | v1.0.0 (already transitive) | SHA256 content-hash for WDL source blobs | Already in go.mod as transitive dep via Docker SDK; `digest.FromBytes()` returns `sha256:<hex>` format matching Docker/OCI conventions; immutable content addressing |

No new third-party libraries are required beyond what is already in go.mod. All capabilities are covered by the existing dependency graph plus two packages that are already transitive dependencies.

### Key Decisions

---

#### 1. JWT: golang-jwt/jwt v5 directly (not go-chi/jwtauth, not lestrrat-go/jwx)

**Recommendation:** Use `github.com/golang-jwt/jwt/v5` v5.3.1 directly. Do NOT use the `go-chi/jwtauth` middleware wrapper.

**Rationale:**

- **go-chi/jwtauth v5 depends on lestrrat-go/jwx v3**, not golang-jwt. This is a substantial dependency: lestrrat-go/jwx is a full JOSE implementation covering JWE, JWK, JWS, and JWT. For flowy's requirement — sign a `{"sub": userID, "role": "admin"}` token with HMAC-SHA256 — this is 50x the needed surface area.

- **golang-jwt/jwt v5 has zero runtime dependencies.** The entire library is stdlib-only. v5.3.1 was published January 28, 2026. This is the natural successor to the dgrijalva/jwt-go library that most Go JWT tutorials reference. 13,000+ importers.

- **Writing JWT middleware for chi is 30 lines of Go.** chi's middleware contract is a plain `func(http.Handler) http.Handler`. Extract the `Authorization: Bearer <token>` header, call `jwt.ParseWithClaims`, put the claims on `context.Context`. No magic needed. Go's chi v1.1 decision (section below) already established that the codebase uses minimal middleware.

- **HS256 with a server secret is correct for this use case.** flowy uses username/password auth, not a federated identity provider. RS256 (asymmetric) is necessary when tokens must be verified by a third party. Here the server both issues and verifies all tokens — HS256 is simpler and sufficient.

- **Token design:**
  ```go
  // internal/auth/token.go
  type Claims struct {
      jwt.RegisteredClaims
      UserID string `json:"uid"`
      Role   string `json:"role"` // "admin" | "executor" | "viewer"
  }

  func IssueToken(secret []byte, userID, role string, ttl time.Duration) (string, error) {
      claims := Claims{
          RegisteredClaims: jwt.RegisteredClaims{
              Subject:   userID,
              IssuedAt:  jwt.NewNumericDate(time.Now()),
              ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
          },
          UserID: userID,
          Role:   role,
      }
      return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).
          SignedString(secret)
  }
  ```

- **Security: always use `jwt.WithValidMethods([]string{"HS256"})`** in `ParseWithClaims` to prevent algorithm substitution (alg=none attack).

**Confidence:** HIGH — pkg.go.dev v5.3.1 published January 28, 2026; official docs verified.

---

#### 2. Password Hashing: golang.org/x/crypto/bcrypt (already in go.mod)

**Recommendation:** Use `golang.org/x/crypto/bcrypt` — the package is already a transitive dependency (via Docker SDK). Promote it to a direct `require` in go.mod when first used.

**Rationale:**

- **Already present.** `go.mod` lists `golang.org/x/crypto v0.48.0` as an indirect dependency. Upgrading to `v0.49.0` (latest as of March 11, 2026) adds no new module.

- **bcrypt is the standard Go answer for password hashing.** The `golang.org/x/crypto` package is maintained by the Go team. It provides `GenerateFromPassword` and `CompareHashAndPassword`. No third-party library is needed.

- **Use cost 12.** Default is 10 (~100ms on modern hardware). Cost 12 (~300ms) is a reasonable tradeoff between security and UX for an admin tool. This is not a high-throughput login endpoint.

- **72-byte limit.** bcrypt silently truncates passwords longer than 72 bytes. For flowy (admin/HPC tool), this is not a concern in practice. Document it; do not pre-hash.

- **Alternatives explicitly rejected:**
  - `argon2id` (`golang.org/x/crypto/argon2`) — more memory-hard than bcrypt but adds complexity; bcrypt is sufficient and universally understood
  - Third-party wrappers — no value added over the x/crypto package for this use case

**Confidence:** HIGH — golang.org/x/crypto is maintained by the Go team; bcrypt package verified at pkg.go.dev.

---

#### 3. RBAC: Hand-Written Middleware (not Casbin, not gorbac)

**Recommendation:** Implement RBAC as a single ~50-line Go file in `internal/auth/`. No external RBAC library.

**Rationale:**

- **flowy has exactly three roles: admin, executor, viewer.** The role model does not have inheritance, resource-level policies, or dynamic rule evaluation. It is a static enum. This is the simplest case of RBAC.

- **Casbin is designed for complex policy models.** It uses a configuration DSL (`.conf` files) to describe access control models. For three static roles, this is accidental complexity that makes the code harder to read and reason about, not easier.

- **gorbac and similar libraries** add an in-memory role store with runtime mutation. flowy roles are fixed at compile time — there is nothing to store or mutate.

- **The complete implementation:**
  ```go
  // internal/auth/rbac.go
  type Role string

  const (
      RoleAdmin    Role = "admin"
      RoleExecutor Role = "executor"
      RoleViewer   Role = "viewer"
  )

  // RequireRole returns a chi middleware that enforces minimum role.
  // Role ordering: viewer < executor < admin.
  func RequireRole(minimum Role) func(http.Handler) http.Handler {
      return func(next http.Handler) http.Handler {
          return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
              claims, ok := ClaimsFromContext(r.Context())
              if !ok || !hasRole(Role(claims.Role), minimum) {
                  http.Error(w, "forbidden", http.StatusForbidden)
                  return
              }
              next.ServeHTTP(w, r)
          })
      }
  }

  func hasRole(actual, minimum Role) bool {
      order := map[Role]int{RoleViewer: 0, RoleExecutor: 1, RoleAdmin: 2}
      return order[actual] >= order[minimum]
  }
  ```
  This is the entire authorization layer. Wire it as chi middleware on protected route groups.

- **Project membership** (user X has role Y in project Z) is a database concern, not a library concern. Store it in a `project_members` table. The middleware reads the project context from the URL, fetches the member role, and enforces it.

**Confidence:** HIGH — hand-rolled RBAC is idiomatic Go for fixed role sets; Casbin overhead verified via documentation and community comparisons.

---

#### 4. WDL Registry Content-Hash Versioning: stdlib crypto/sha256 + opencontainers/go-digest (already in go.mod)

**Recommendation:** Use Go's stdlib `crypto/sha256` for hashing and `github.com/opencontainers/go-digest` for the canonical `sha256:<hex>` digest string format. Do not use a Docker registry client library.

**Rationale:**

- **opencontainers/go-digest is already a transitive dependency.** `go.mod` lists `github.com/opencontainers/go-digest v1.0.0` (via Docker SDK). The library provides `digest.FromBytes(content)` which returns a `Digest` of the form `sha256:7173b809ca12ec5dee4506cd86be934c4596dd234ee82c0662eac04a8c2c71dc`. This is the same format Docker uses for image layers.

- **Content-addressing pattern for WDL registry:**
  ```
  // Content table stores WDL source by hash — immutable
  wdl_blobs: (digest TEXT PK, content TEXT, size_bytes INT, created_at DATETIME)

  // Registry entries link human-readable names+tags to digests
  registry_items: (id, name, kind, created_at)  // kind: "tool" | "workflow"
  registry_versions: (id, item_id, tag, digest, created_at)
  ```
  `tag` is mutable (can be re-pointed to a new digest, like Docker `latest`). `digest` is immutable. Looking up by digest is always unambiguous; looking up by tag is a soft reference. This exactly mirrors the Docker registry model.

- **The registry does NOT need to implement the Docker Distribution HTTP API.** flowy's WDL registry is a first-party service with its own REST API. Using go-containerregistry or distribution/distribution would be enormous over-engineering — those libraries implement the full OCI distribution spec for container images.

- **Deduplication is free.** If the same WDL source is registered under two different names/versions, the blob is stored once. The `wdl_blobs` table is an append-only content store; `registry_versions` is the mutable index.

**Confidence:** HIGH — opencontainers/go-digest already in go.mod; pattern verified against Docker/OCI conventions.

---

#### 5. Provenance Tracking: SQLite + New Tables (no external lineage library)

**Recommendation:** Store provenance as relational data in SQLite using new migration files. No external lineage tracking library (no OpenLineage, no OpenMetadata).

**Rationale:**

- **Provenance requirements are narrow and well-defined.** Each output file needs: which tool/workflow produced it, which version (digest + tag), which run ID, which input files were consumed. This is 2-3 tables with foreign keys.

- **External lineage tools (OpenLineage, OpenMetadata) are infrastructure products.** They require running separate services, have their own API, and target data engineering pipelines with hundreds of jobs. For flowy's use case — "which WDL tool produced this output file?" — they are vastly over-scoped.

- **Schema:**
  ```sql
  -- New migration: 00004_registry_and_provenance.sql
  CREATE TABLE registry_items (
      id          TEXT PRIMARY KEY,  -- UUID
      name        TEXT NOT NULL,
      kind        TEXT NOT NULL,     -- 'tool' | 'workflow'
      created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
  );

  CREATE TABLE registry_versions (
      id          TEXT PRIMARY KEY,
      item_id     TEXT NOT NULL REFERENCES registry_items(id),
      tag         TEXT,              -- nullable; mutable pointer
      digest      TEXT NOT NULL,     -- sha256:<hex>; immutable
      created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
      UNIQUE(item_id, tag)
  );

  CREATE TABLE wdl_blobs (
      digest      TEXT PRIMARY KEY,  -- sha256:<hex>
      content     TEXT NOT NULL,
      size_bytes  INTEGER NOT NULL,
      created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
  );

  CREATE TABLE provenance (
      id              TEXT PRIMARY KEY,
      run_id          TEXT NOT NULL REFERENCES runs(id),
      registry_version_id TEXT REFERENCES registry_versions(id),
      output_path     TEXT NOT NULL,  -- backend path of the output file
      created_at      DATETIME NOT NULL DEFAULT (datetime('now'))
  );
  ```

- **goose handles migrations** as it did for v1.1–v1.2. Add migration `00004_registry_and_provenance.sql` following the existing pattern.

- **Provenance is append-only.** Never update or delete provenance records. If a run is re-executed, create a new provenance row. This is sufficient for audit purposes and avoids concurrency issues.

**Confidence:** HIGH — design is a direct extension of the existing SQLite schema; no new libraries needed.

---

#### 6. Project Management: SQLite + chi sub-routers (no external project management library)

**Recommendation:** Add project and membership tables via goose migration. No external library.

**Rationale:**

- **Schema extension:**
  ```sql
  -- Part of 00004_registry_and_provenance.sql or a separate 00004_projects.sql
  CREATE TABLE users (
      id            TEXT PRIMARY KEY,
      username      TEXT NOT NULL UNIQUE,
      password_hash TEXT NOT NULL,
      created_at    DATETIME NOT NULL DEFAULT (datetime('now'))
  );

  CREATE TABLE projects (
      id          TEXT PRIMARY KEY,
      name        TEXT NOT NULL UNIQUE,
      created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
  );

  CREATE TABLE project_members (
      project_id  TEXT NOT NULL REFERENCES projects(id),
      user_id     TEXT NOT NULL REFERENCES users(id),
      role        TEXT NOT NULL,  -- 'admin' | 'executor' | 'viewer'
      PRIMARY KEY (project_id, user_id)
  );
  ```

- **Route structure with chi:**
  ```
  POST   /auth/login                  -- returns JWT; no auth required
  POST   /auth/register               -- create user (admin only in production)

  POST   /projects                    -- JWT required; admin role
  GET    /projects/{projectID}        -- JWT required; project member (any role)
  POST   /projects/{projectID}/members -- JWT required; project admin role

  POST   /projects/{projectID}/registry/tools     -- register WDL tool
  GET    /projects/{projectID}/registry/tools     -- list tools
  POST   /projects/{projectID}/workflows          -- submit workflow (executor+)
  GET    /projects/{projectID}/workflows/{id}     -- status (viewer+)
  ```
  Use chi's `r.Route("/projects/{projectID}", ...)` to group project-scoped endpoints under shared JWT + project membership middleware.

- **The existing `Store` interface** needs new methods for user, project, and registry CRUD. Extend `internal/store/store.go` with new interfaces following the same pattern as `Run` and `Task` operations.

**Confidence:** HIGH — direct extension of the established chi + SQLite pattern.

---

### Supporting Libraries (v1.3)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `golang-jwt/jwt/v5` | v5.3.1 | JWT token generation and validation | New direct dependency; all auth handlers |
| `golang.org/x/crypto/bcrypt` | v0.49.0 | Password hashing | Promote from indirect to direct; login + registration handlers |
| `opencontainers/go-digest` | v1.0.0 | Canonical sha256 digest string format | Promote from indirect to direct; WDL blob storage |
| `github.com/google/uuid` | v1.6.0 (already direct) | UUID generation for user/project/registry IDs | Already in go.mod; use for all new entity IDs |

---

### Installation (v1.3 new dependencies only)

```bash
# JWT library (new direct dependency)
go get github.com/golang-jwt/jwt/v5@v5.3.1

# Promote existing transitive deps to direct (no version bump needed)
# golang.org/x/crypto is already v0.48.0; consider upgrading to v0.49.0
go get golang.org/x/crypto@v0.49.0

# opencontainers/go-digest is already v1.0.0; no version change needed
# Just add an import in the registry code and run go mod tidy
```

After adding imports for `golang.org/x/crypto/bcrypt` and `opencontainers/go-digest`, run `go mod tidy` to reclassify them from `// indirect` to direct dependencies in go.mod. No new module downloads are needed.

---

### Alternatives Considered (v1.3)

| Recommended | Alternative | Why Not |
|-------------|-------------|---------|
| `golang-jwt/jwt/v5` direct | `go-chi/jwtauth/v5` | jwtauth depends on lestrrat-go/jwx v3 (full JOSE suite); ~10 new transitive deps for a feature that needs 30 lines of custom middleware |
| `golang-jwt/jwt/v5` direct | `lestrrat-go/jwx/v3` directly | Same issue; JWE/JWK/JWS features are not needed; JWT-only library is the right scope |
| Hand-rolled RBAC | Casbin | Policy DSL configuration (.conf files) is overhead for a 3-role static model; Casbin is designed for complex attribute-based policies |
| Hand-rolled RBAC | gorbac | In-memory runtime role store is unnecessary when roles are a compile-time constant |
| `crypto/sha256` + `go-digest` | OCI registry client (`go-containerregistry`) | Implements full OCI distribution spec; the WDL registry is a custom REST API, not a Docker-compatible registry |
| bcrypt cost=12 | argon2id | More memory-hard but requires tuning memory/iterations per deployment; bcrypt is universally understood and sufficient |
| SQLite provenance tables | OpenLineage/OpenMetadata | External infrastructure services; multi-service deployment for a feature that needs 3 tables and a SELECT |

---

### What NOT to Add

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| **`go-chi/jwtauth`** | Brings lestrrat-go/jwx v3 (full JOSE) as a dependency; overkill for HS256 token check | `golang-jwt/jwt/v5` + 30-line custom chi middleware |
| **`apache/casbin`** | Policy DSL and persistence adapter overhead for 3 static roles | Hand-written `RequireRole` middleware in `internal/auth/` |
| **`go-containerregistry`** | Full OCI client; the WDL registry is not a Docker-protocol registry | `opencontainers/go-digest` (already in go.mod) + custom REST API |
| **OAuth2 / SSO libraries** | Out of scope per PROJECT.md — username/password is sufficient for v1.3 | `golang-jwt/jwt/v5` + bcrypt |
| **session store (Redis, memcached)** | Stateless JWT is sufficient; server does not need to track sessions | JWT with expiry in claims; client refreshes by re-logging in |
| **GORM / sqlc** | Already rejected in v1.1 section; same reasoning applies to the new tables | `database/sql` + hand-written queries following existing store patterns |

---

### Version Compatibility (v1.3)

| Package | Compatible With | Notes |
|---------|-----------------|-------|
| `golang-jwt/jwt/v5 v5.3.1` | Go 1.18+ | NEW direct dependency; go.mod is `go 1.25.0` — compatible |
| `golang.org/x/crypto v0.49.0` | Go 1.20+ | Promote from indirect; already in module graph at v0.48.0 |
| `opencontainers/go-digest v1.0.0` | Go 1.13+ | Already in go.mod; promote to direct by adding an import |
| `github.com/google/uuid v1.6.0` | Go 1.15+ | Already a direct dependency; use for all new entity IDs |

---

### Integration Points for v1.3

The new capabilities plug into the existing server architecture at specific seams:

1. **JWT middleware** wraps the chi router's user-facing route group. The existing daemon-facing routes (`/tasks/*`, `/heartbeat`) continue to use the static bearer token from v1.1.

2. **Project membership check** is a second middleware layer inside project-scoped routes. Reads `{projectID}` from the URL, queries `project_members`, sets the effective role on context. This runs after JWT validation.

3. **Registry** is a new set of handlers in `internal/server/handlers_registry.go`. Uses the existing `Store` interface extended with registry methods.

4. **Provenance** is recorded in `handleResult` (in `internal/server/handlers.go`) when a task completes. If the run was submitted with a registry version reference, write a `provenance` row linking the output paths to that version.

5. **Workflow submission** gains an optional `registry_version_id` field. When present, the server resolves the WDL content from `wdl_blobs` instead of reading from the filesystem. Backward-compatible: existing `wdl_path` submissions continue to work.

---

### Sources

- [golang-jwt/jwt v5 — pkg.go.dev](https://pkg.go.dev/github.com/golang-jwt/jwt/v5) — v5.3.1 published January 28, 2026; 13,000+ importers; HIGH confidence
- [golang-jwt/jwt releases — GitHub](https://github.com/golang-jwt/jwt/releases) — v5.3.1 confirmed latest stable; HIGH confidence
- [golang.org/x/crypto/bcrypt — pkg.go.dev](https://pkg.go.dev/golang.org/x/crypto/bcrypt) — v0.49.0 March 11, 2026; Go team maintained; HIGH confidence
- [opencontainers/go-digest — pkg.go.dev](https://pkg.go.dev/github.com/opencontainers/go-digest) — v1.0.0 stable; already in go.mod as transitive dep; HIGH confidence
- [go-chi/jwtauth v5 — pkg.go.dev](https://pkg.go.dev/github.com/go-chi/jwtauth/v5) — v5.4.0 February 26, 2026; depends on lestrrat-go/jwx; evaluated and not recommended; MEDIUM confidence
- [lestrrat-go/jwx releases — GitHub](https://github.com/lestrrat-go/jwx/releases) — v3.0.13 released; full JOSE suite; too broad for this use case; MEDIUM confidence
- [apache/casbin — casbin.org](https://casbin.org/) — policy DSL for complex RBAC; overhead not justified for 3 static roles; HIGH confidence assessment

---

## v1.2 Additions: S3 Object Storage Backend

This section covers NEW libraries needed for the v1.2 S3 milestone.
The v1.0 and v1.1 stacks are unchanged — see the preserved sections below.

### Core Technologies Added for v1.2

| Layer | Library | Version | Purpose | Why Recommended |
|-------|---------|---------|---------|-----------------|
| **S3 client** | `github.com/aws/aws-sdk-go-v2/service/s3` | v1.78.x | S3 API calls (PutObject, GetObject, HeadObject, DeleteObject) | Official AWS SDK; MinIO-compatible via `BaseEndpoint` + `UsePathStyle`; actively maintained (5,200+ importers); modular — only add service packages you use |
| **SDK config** | `github.com/aws/aws-sdk-go-v2/config` | v1.32.x | Load AWS credentials and region from env/file/instance metadata | Provides the default credential chain; static credentials via `WithCredentialsProvider` for MinIO testing; same package for AWS and local MinIO |
| **Static credentials** | `github.com/aws/aws-sdk-go-v2/credentials` | v1.17.x | `credentials.NewStaticCredentialsProvider` for test/MinIO config | Needed when bypassing the full credential chain; zero additional transitive dependencies |
| **Integration testing** | `github.com/testcontainers/testcontainers-go/modules/minio` | v0.36.x | Spin up a real MinIO container during integration tests | Official testcontainers MinIO module; manages container lifecycle automatically; uses Docker (already a hard dependency); no separate mock needed |

### Key Decisions

#### 1. AWS SDK for Go v2 (not v1, not minio-go)

**Recommendation:** Use `github.com/aws/aws-sdk-go-v2/service/s3` as the primary S3 client for both AWS S3 and MinIO.

**Rationale:**

- **AWS SDK v1 is end-of-support.** The `github.com/aws/aws-sdk-go` repository explicitly states it has reached end-of-support. All new Go S3 code should use v2.

- **AWS SDK v2 is MinIO-compatible with two options.** MinIO implements the S3 API. To point the SDK at MinIO:
  ```go
  client := s3.NewFromConfig(cfg, func(o *s3.Options) {
      o.BaseEndpoint = aws.String("http://localhost:9000")
      o.UsePathStyle = true // required for MinIO without wildcard TLS
  })
  ```
  `UsePathStyle = true` forces `http://host/bucket/key` instead of `http://bucket.host/key`. This is the v2 replacement for the v1 `S3ForcePathStyle` option. Confirmed working in GitHub issue #2573.

- **minio-go (github.com/minio/minio-go/v7) is NOT recommended as primary client.** minio-go is a good library but introduces a second S3 client interface into the codebase. The project needs to work with both AWS S3 and MinIO — maintaining a single abstraction over two different client SDKs doubles the integration surface. AWS SDK v2 speaks to both. Use minio-go only if AWS SDK v2 cannot support a specific MinIO feature needed (none identified for v1.2 scope).

- **AWS SDK v2 is modular.** Only import the packages you need. For flowy's scope:
  - `service/s3` — core operations
  - `config` — credential loading
  - `credentials` — static credentials for MinIO tests
  - No need for CloudFormation, EC2, IAM, or other service packages.

**Confidence:** HIGH — verified via pkg.go.dev (v1.78.x published March 2026), official GitHub releases, and issue #2573 confirming `UsePathStyle` works.

---

#### 2. Storage Abstraction: Hand-Written Interface (not afero, not vfs, not sajari/storage)

**Recommendation:** Define a narrow `StorageBackend` interface in `internal/storage/` — implement it for shared filesystem and S3. Do not adopt a third-party abstraction library.

**Rationale:**

- **The existing STACK.md v1.1 section (Decision 6) already ruled out afero** for the shared FS case. That reasoning applies here too: afero's interface is wider than needed and designed around `os.File` semantics that don't map cleanly to S3.

- **io/fs.FS is read-only.** The stdlib `io/fs.FS` interface (Go 1.16+) only covers read operations (`Open`). WDL task execution requires write operations (stage inputs to working directory, collect outputs). A read-only interface cannot model the full storage lifecycle.

- **Third-party abstraction libraries are poorly maintained or over-scoped.** From research:
  - `sajari/storage` — last significant activity 2020; not actively maintained
  - `spf13/afero` — write operations supported but the interface leaks OS filesystem semantics (file modes, symlinks) that S3 does not have
  - `c2fo/vfs` — complex multi-backend library with its own URI scheme; far more than needed
  - None of these are used by reference WDL engines (miniwdl uses Python pathlib + boto3 with a custom abstraction)

- **The required interface is narrow and project-specific:**
  ```go
  // internal/storage/storage.go
  type Backend interface {
      // Stage a local file to the backend working directory.
      // Returns the backend path (s3://bucket/key or /shared/fs/path).
      StageInput(ctx context.Context, localPath string, dest string) (string, error)
      // Download a backend path to a local file for Docker container access.
      FetchOutput(ctx context.Context, backendPath string, localDest string) error
      // Check if a backend path exists.
      Stat(ctx context.Context, path string) (bool, error)
      // Delete a backend path (cleanup).
      Delete(ctx context.Context, path string) error
      // Return the base URI for a new workflow execution.
      WorkDir(runID string) string
  }
  ```
  This interface is intentionally minimal. It covers exactly what the DAG scheduler and task executor need. Both `LocalBackend` and `S3Backend` can implement it cleanly.

- **Path scheme resolution** (`s3://` → S3 backend, `/absolute` → local FS, `relative` → default backend) is a routing decision in the input resolver, not in the backend itself. Keep them separate.

**Confidence:** HIGH — based on research of available libraries and analysis of the existing codebase's needs.

---

#### 3. Integration Testing: testcontainers-go MinIO Module (not docker-compose, not mock)

**Recommendation:** Use `github.com/testcontainers/testcontainers-go/modules/minio` for S3 integration tests.

**Rationale:**

- **flowy-daemon already depends on Docker.** The Docker SDK (`github.com/docker/docker`) is a core dependency. testcontainers-go runs MinIO in a Docker container managed from Go test code — no new system dependency.

- **testcontainers provides automatic lifecycle management.** The MinIO container starts before the test and is terminated after (via `defer testcontainers.TerminateContainer(minioContainer)`). No lingering containers from failed test runs.

- **The MinIO module is purpose-built:**
  ```go
  minioContainer, err := minio.Run(ctx, "minio/minio:RELEASE.2024-01-16T16-07-38Z",
      minio.WithUsername("testuser"),
      minio.WithPassword("testpassword"),
  )
  endpoint, err := minioContainer.ConnectionString(ctx)
  // endpoint = "http://localhost:XXXXX"
  ```
  `ConnectionString()` returns the container's port-forwarded endpoint URL, which feeds directly into the S3 client `BaseEndpoint` option.

- **Spectool conformance (156/156) must pass on the S3 backend.** This requires a real S3-compatible server, not a mock. testcontainers + MinIO is the standard Go pattern for this level of integration testing.

- **testcontainers-go v0.36.x** is the pinned version in the modules/ subdirectory. The MinIO module has been available since v0.28.0. Do not use `RunContainer` (deprecated) — use `Run`.

- **CI compatibility:** testcontainers requires a Docker socket. If CI runs in a Docker-in-Docker environment, set `TESTCONTAINERS_RYUK_DISABLED=true` to avoid Ryuk container teardown issues in DinD.

**Confidence:** HIGH — verified via golang.testcontainers.org/modules/minio/ official documentation.

---

#### 4. S3 Transfer Manager: stdlib s3.PutObject / s3.GetObject (not feature/s3/manager)

**Recommendation:** Use `s3.PutObject` and `s3.GetObject` directly for single-file operations. Do NOT add `feature/s3/manager` or `feature/s3/transfermanager`.

**Rationale:**

- **`feature/s3/manager` is deprecated** as of its latest release (v1.22.6, March 2026). It is superseded by `feature/s3/transfermanager`, per AWS's own package documentation.

- **`feature/s3/transfermanager` is pre-1.0 (v0.1.8)** — explicitly unstable. AWS marks it as such on pkg.go.dev. Do not take a pre-1.0 dependency for core functionality.

- **WDL task files in bioinformatics are typically 1 MB – 10 GB.** For files that require multipart upload (> 5 GB), the standard approach is to use `s3.CreateMultipartUpload` / `s3.UploadPart` / `s3.CompleteMultipartUpload` manually, or wrap this in a small helper. This is ~80 lines of Go and avoids a deprecated dependency.

- **Defer multipart until needed.** In v1.2, implement single-part upload/download first (covers files up to 5 GB, which is adequate for initial S3 support). Add multipart in a follow-on milestone if benchmarking shows it is needed.

**Confidence:** HIGH — pkg.go.dev confirmed `feature/s3/manager` deprecated; `transfermanager` is v0.1.8 unstable.

---

### Supporting Libraries (v1.2)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/aws/aws-sdk-go-v2/aws` | v1.36.x | `aws.String()`, `aws.Bool()` helpers; types shared across SDK packages | Always, when using any aws-sdk-go-v2 package |
| `github.com/testcontainers/testcontainers-go` | v0.36.x | Base testcontainers runtime (required by MinIO module) | Integration tests only (`_test.go` files) |

---

### Installation (v1.2 new dependencies only)

```bash
# S3 client and config
go get github.com/aws/aws-sdk-go-v2/service/s3@latest
go get github.com/aws/aws-sdk-go-v2/config@latest
go get github.com/aws/aws-sdk-go-v2/credentials@latest
go get github.com/aws/aws-sdk-go-v2/aws@latest

# Integration testing (MinIO via testcontainers)
go get github.com/testcontainers/testcontainers-go@latest
go get github.com/testcontainers/testcontainers-go/modules/minio@latest
```

Note: `testcontainers-go` and its MinIO module use separate Go modules. The MinIO module version must match the base testcontainers version.

---

### MinIO Configuration Pattern

The complete pattern for creating an S3 client that works with both AWS and MinIO:

```go
// For AWS S3 (production)
cfg, err := config.LoadDefaultConfig(ctx,
    config.WithRegion("us-east-1"),
)
client := s3.NewFromConfig(cfg)

// For MinIO (local / integration tests)
cfg, err := config.LoadDefaultConfig(ctx,
    config.WithRegion("us-east-1"),
    config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
        "minioadmin", "minioadmin", "",
    )),
)
client := s3.NewFromConfig(cfg, func(o *s3.Options) {
    o.BaseEndpoint = aws.String("http://localhost:9000")
    o.UsePathStyle = true // required — MinIO lacks wildcard TLS for virtual-host style
})
```

This means S3Backend receives `*s3.Client` and a bucket name — the same client type for both AWS and MinIO. The endpoint difference is in construction, not usage.

---

### Alternatives Considered (v1.2)

| Recommended | Alternative | Why Not |
|-------------|-------------|---------|
| AWS SDK v2 for S3 | `minio/minio-go/v7` | Two client SDKs for one protocol; AWS SDK v2 speaks to both AWS and MinIO; single interface in the codebase |
| Hand-written `StorageBackend` interface | `spf13/afero` | afero leaks OS FS semantics (modes, symlinks) incompatible with S3; narrower interface fits WDL staging lifecycle better |
| Hand-written `StorageBackend` interface | `sajari/storage` | Unmaintained since 2020; read/write interface but no active community |
| testcontainers MinIO | docker-compose for tests | docker-compose requires external file, manual teardown, not integrated with `go test`; testcontainers is fully Go-native |
| testcontainers MinIO | Mock S3 (aws/aws-sdk-go-v2/testing) | Mock does not test real S3 wire protocol; spectool conformance requires real I/O |
| Direct `PutObject`/`GetObject` | `feature/s3/manager` | Deprecated; superseded by pre-1.0 unstable transfermanager; single-part is sufficient for v1.2 |

---

### What NOT to Add

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| **`github.com/aws/aws-sdk-go`** (v1) | End-of-support; no new features; security updates only until EOL | `github.com/aws/aws-sdk-go-v2` |
| **`feature/s3/manager`** | Deprecated in March 2026; the replacement (`transfermanager`) is pre-1.0 | Direct `PutObject`/`GetObject` with optional manual multipart |
| **`feature/s3/transfermanager`** | Unstable v0.1.x; API may change; do not build core functionality on pre-1.0 deps | Revisit when it reaches v1.0 |
| **`minio/minio-go/v7`** (as primary client) | Introduces a second S3 interface; AWS SDK v2 already speaks S3 protocol to MinIO | AWS SDK v2 with `BaseEndpoint` + `UsePathStyle` |
| **`spf13/afero`** | OS-FS interface incompatible with S3; was deliberately excluded in v1.1 decision | Hand-written `StorageBackend` interface in `internal/storage/` |
| **MinIO server binary embedded in tests** | Large binary (~100 MB); version pinning is fragile; testcontainers handles this cleanly | `testcontainers-go/modules/minio` |

---

### Version Compatibility (v1.2)

| Package | Compatible With | Notes |
|---------|-----------------|-------|
| `aws-sdk-go-v2/service/s3 v1.78.x` | Go 1.21+ | go.mod is `go 1.25.0` — compatible |
| `aws-sdk-go-v2/config v1.32.x` | Go 1.21+ | Same module group; always update config + credentials + s3 together |
| `testcontainers-go v0.36.x` | Go 1.21+, Docker 20.10+ | Requires Docker socket; set `TESTCONTAINERS_RYUK_DISABLED=true` in Docker-in-Docker CI |
| `testcontainers-go/modules/minio v0.36.x` | Must match base testcontainers version | Minor version must match base module |

---

### Sources

- [aws/aws-sdk-go-v2/service/s3 — pkg.go.dev](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3) — v1.78.x latest March 2026; 5,227 importers; HIGH confidence
- [aws/aws-sdk-go-v2/config — pkg.go.dev](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/config) — v1.32.11 March 2026; credential chain documentation; HIGH confidence
- [aws/aws-sdk-go-v2 issue #2573 — GitHub](https://github.com/aws/aws-sdk-go-v2/issues/2573) — `UsePathStyle` confirmed working for MinIO; HIGH confidence
- [aws/aws-sdk-go — GitHub](https://github.com/aws/aws-sdk-go) — v1 end-of-support declaration; HIGH confidence
- [feature/s3/manager — pkg.go.dev](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/feature/s3/manager) — deprecated March 2026; superseded by transfermanager; HIGH confidence
- [feature/s3/transfermanager — pkg.go.dev](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager) — v0.1.8 unstable; HIGH confidence
- [testcontainers-go/modules/minio — golang.testcontainers.org](https://golang.testcontainers.org/modules/minio/) — available since v0.28.0; `Run()` function signature; HIGH confidence
- [minio/minio-go/v7 — pkg.go.dev](https://pkg.go.dev/github.com/minio/minio-go/v7) — v7.0.99 March 2026; 3,090 importers; evaluated and not recommended for this use case; MEDIUM confidence

---

## v1.1 Additions: Distributed Server / Daemon / Client

This section covers NEW libraries needed for the v1.1 distributed architecture milestone.
The v1.0 stack (parser, Docker SDK, CLI, testing) is unchanged — see the preserved section below.

### Core Technologies Added for v1.1

| Layer | Library | Version | Purpose | Why Recommended |
|-------|---------|---------|---------|-----------------|
| **HTTP router** | `github.com/go-chi/chi/v5` | v5.2.5 | REST API routing for flowy-server | 100% net/http compatible; built-in middleware subpackage covers RequestID, Logger, Recoverer; no reflection magic; idiomatic Go |
| **HTTP logging** | `github.com/go-chi/httplog/v3` | v3.3.0 | Structured HTTP request logging | Uses stdlib log/slog (zero external deps); already pairs with chi; smart log-level by status code |
| **SQLite driver** | `modernc.org/sqlite` | v1.36.x | SQLite persistence — workflow runs, task state | CGO-free: no C compiler required in CI/CD, cross-compiles cleanly; flowy's write volume is low (task status updates), so the modest performance gap vs mattn is irrelevant |
| **DB migrations** | `github.com/pressly/goose/v3` | v3.27.0 | Schema migrations embedded in binary | Supports `go:embed` SQL files; pure Go; no external tool needed at runtime; widely used in Go services |
| **YAML config** | `gopkg.in/yaml.v3` | v3.0.1 | Config file parsing (`~/.flowy/config.yaml`) | Stdlib-quality stability; zero dependencies; sufficient for a flat config struct; Viper is overkill here |

### Key Decisions

#### 1. HTTP Router: chi v5 (not net/http ServeMux, not gin, not echo)

**Recommendation:** Use `github.com/go-chi/chi/v5` v5.2.5.

**Rationale:**

- **net/http ServeMux (Go 1.22+) lacks middleware grouping.** The server needs per-route middleware: some endpoints require a daemon auth token, others require a user token. ServeMux has no sub-router or middleware group concept. Every route would need manual middleware wrapping — that's what chi solves cleanly.

- **gin and echo add unnecessary weight.** Both use custom context types that break compatibility with standard `http.Handler` middleware from the ecosystem. They also use reflection-based binding. For a REST API that serves ~6 routes, this is over-engineering with a higher cognitive cost.

- **chi is 100% net/http compatible.** Every handler is `http.Handler`. Any standard library middleware, any other Go HTTP tool, integrates without adapters. The existing codebase uses standard `net/http` client calls — consistency matters.

- **chi's built-in middleware subpackage covers everything needed:**
  - `middleware.RequestID` — injects `X-Request-Id` header and context value; correlates logs
  - `middleware.Logger` — per-request structured logging (integrates with slog via httplog)
  - `middleware.Recoverer` — catches panics, returns HTTP 500, prints request ID in trace
  - `middleware.Timeout` — per-request timeout cancellation
  - No external middleware packages needed for v1.1

- **chi v5.2.5** (released February 2026) bumps the minimum Go to 1.22 and uses the new `http.Request.PathValue()` API internally. Compatible with Go 1.24+.

**Confidence:** HIGH — verified via pkg.go.dev, GitHub releases, and official docs.

---

#### 2. SQLite Driver: modernc.org/sqlite (not mattn/go-sqlite3)

**Recommendation:** Use `modernc.org/sqlite` v1.36.x.

**Rationale:**

- **CGO-free is the deciding factor.** mattn/go-sqlite3 requires CGO, which means:
  - `CGO_ENABLED=1` in all CI environments
  - A C compiler on the build machine
  - Cross-compilation (e.g., building a Linux binary on macOS for deployment) requires a C cross-toolchain
  - The existing Makefile and GitHub Actions pipeline would need a GCC installation step

- **Performance gap is immaterial for this workload.** modernc.org/sqlite is ~2x slower on INSERT-heavy benchmarks. flowy-server's write patterns are: insert a workflow run, update task status (perhaps 10-100 updates per workflow). This is not a database benchmark — it is incidental persistence. The performance ceiling of modernc is 100x above flowy's actual needs.

- **modernc.org/sqlite is production-grade.** It is imported by 2,562+ Go packages (as of February 2026, v1.46.1 on pkg.go.dev), actively maintained, and embeds SQLite 3.51.2.

- **database/sql compatibility.** Both drivers implement the standard `database/sql` interface. If flowy ever needs to switch drivers (e.g., to a Postgres backend for a hosted service), only the `sql.Open` call changes.

**Confidence:** HIGH — verified via pkg.go.dev, benchmark comparisons, and GitHub issue discussions.

---

#### 3. Database Migrations: goose v3 (embedded SQL files)

**Recommendation:** Use `github.com/pressly/goose/v3` v3.27.0 with `go:embed` for SQL migration files.

**Rationale:**

- **Migrations must be embedded in the binary.** flowy-server is deployed as a single binary. A separate migration tool or external migration files create a deployment synchronization problem. goose supports `fs.FS` directly — embed `migrations/*.sql` into the binary with `//go:embed`.

- **SQL-first migrations match the project.** flowy's schema is simple (3-4 tables). Writing migrations as plain SQL is readable, reviewable, and version-controlled. ORM-based migration tools (GORM AutoMigrate) are opaque and hard to audit.

- **golang-migrate is the alternative** but requires more setup for the embedded FS pattern. goose's API is simpler for a single-database application:
  ```go
  goose.SetBaseFS(embedMigrations)
  goose.Up(db, "migrations")
  ```

- **goose v3.27.0** requires Go 1.25+ (minimum). Since go.mod already specifies `go 1.25.0`, this is compatible.

**Confidence:** HIGH — verified via GitHub releases page (v3.27.0, February 22 2025).

---

#### 4. YAML Config: gopkg.in/yaml.v3 (not Viper, not koanf)

**Recommendation:** Use `gopkg.in/yaml.v3` v3.0.1 directly. Do not use Viper.

**Rationale:**

- **The config requirements are minimal.** flowy needs to read a config file like:
  ```yaml
  server:
    address: "localhost:8080"
    token: "secret"
  storage:
    shared_dir: "/mnt/nfs/flowy-work"
  ```
  This is a flat struct with ~5-8 fields. yaml.v3 unmarshals this in 5 lines of Go.

- **Viper is overkill and has hidden costs.** Viper pulls in ~15 transitive dependencies, has known goroutine-safety issues, and is in a maintenance-only mode (features deferred to Viper v2 which has no release date). It adds more complexity than it solves for a fixed-schema config file.

- **koanf is a cleaner Viper alternative** but is still unnecessary overhead when the config struct does not need hot-reload, multiple format support, or environment variable layering. Those features can be added later if needed.

- **Environment variable overrides** can be implemented with 20 lines of Go stdlib (`os.Getenv`) without a framework.

**Confidence:** HIGH — gopkg.in/yaml.v3 is stable, well-tested, and sufficient.

---

#### 5. Task Queue / Polling Pattern: Stdlib (no external queue library)

**Recommendation:** Implement daemon polling using stdlib `net/http` client + `time.Ticker`. No external message queue (no Redis, no NATS, no RabbitMQ).

**Rationale:**

- **The architecture is polling-based by design** (see PROJECT.md Key Decisions: "Daemon polls server (not push)"). A push-based message broker would require the daemon to maintain a persistent connection to the server — that increases operational complexity and introduces a new infrastructure dependency.

- **Polling with HTTP is simple and observable.** The daemon calls `GET /api/tasks/pending` every N seconds. The server queries SQLite for unclaimed tasks and returns a JSON list. The daemon claims a task atomically with `POST /api/tasks/{id}/claim`. This is 50 lines of Go and trivially debuggable.

- **Atomic claim via SQLite UPDATE ... WHERE status='pending' AND claimer IS NULL returning id** eliminates double-claiming without a distributed lock. Multiple daemons polling simultaneously get different tasks — last-write-wins is safe when the WHERE clause is idempotent.

- **External queues (Redis, NATS) would be justified** if: (a) sub-second task dispatch latency is required, (b) the server needed to push work rather than daemons pull it, or (c) the task volume exceeds what SQLite can handle. None of these apply to v1.1 (HPC workloads run for minutes to hours; SQLite handles thousands of writes/second).

**Pattern sketch:**
```go
// In flowy-daemon
ticker := time.NewTicker(cfg.PollInterval)
for range ticker.C {
    tasks, err := client.ClaimPendingTasks(ctx, daemonID, maxConcurrent)
    for _, task := range tasks {
        go executor.Run(ctx, task)
    }
}
```

**Confidence:** HIGH — matches the declared architecture intent; no external library needed.

---

#### 6. Shared Filesystem: stdlib os/path/filepath (no abstraction library)

**Recommendation:** Use Go stdlib `os`, `path/filepath`, and `io` for all shared filesystem operations. Do not use spf13/afero.

**Rationale:**

- **The shared filesystem is a real POSIX directory** (NFS mount, local path, or Docker volume). All operations are standard: `os.MkdirAll`, `os.CopyFS`, `filepath.Join`. There is no mock filesystem needed in production.

- **afero adds complexity without benefit here.** afero is valuable when you need to swap filesystem backends (in-memory for tests, real FS for production). flowy's filesystem tests can use `t.TempDir()` directly — the standard library provides this.

- **The client's input-staging and daemon's output-collection** are sequential file copies, not parallel streaming. They do not require any abstraction.

- **If S3 support is added in a future milestone**, that is the correct time to introduce an abstraction layer (and at that point, use `io/fs.FS` interface with a custom S3 implementation rather than afero, since `io/fs` is now stdlib-standard).

**Confidence:** HIGH.

---

#### 7. Auth / Request ID Middleware: chi built-ins (no external auth library)

**Recommendation:** For v1.1, use `middleware.RequestID` from chi for request tracing and a simple static token check in a custom middleware for daemon/client authentication. No JWT, no OAuth, no external auth library.

**Rationale:**

- **v1.1 auth requirement is a shared secret token** (see PROJECT.md: "flowy-daemon polls server" — both daemon and client authenticate with a configured token). A static `Authorization: Bearer <token>` check is 15 lines of Go middleware.

- **JWT is unnecessary complexity** for a local/on-prem deployment where the token is a static secret in a config file. JWT rotation, claims validation, and expiry management are all overhead for a tool that runs in a private HPC cluster.

- **Request IDs** from `middleware.RequestID` propagate via `X-Request-Id` header and `context.Context`. Use `middleware.GetReqID(ctx)` in handler logs to correlate request traces across server logs and client output.

**Confidence:** HIGH.

---

### Supporting Libraries (v1.1)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `encoding/json` (stdlib) | Go 1.24+ | JSON request/response serialization | All API endpoints — no external JSON library needed |
| `context` (stdlib) | Go 1.24+ | Request cancellation propagation from HTTP handler to Docker executor | Always pass ctx through the call chain |
| `golang.org/x/sync/errgroup` | v0.20.0 | Already in go.mod; parallel task execution in daemon | Reuse existing dependency for concurrent task dispatch |

---

### Installation (v1.1 new dependencies only)

```bash
# HTTP router
go get github.com/go-chi/chi/v5@v5.2.5

# HTTP request logging (slog-based, zero external deps)
go get github.com/go-chi/httplog/v3@v3.3.0

# SQLite driver (CGO-free)
go get modernc.org/sqlite@latest

# Database migrations
go get github.com/pressly/goose/v3@v3.27.0

# YAML config parsing
go get gopkg.in/yaml.v3@v3.0.1
```

---

### Multi-Binary Build Pattern

The three binaries share the same Go module. Use standard `cmd/` layout:

```
cmd/
  flowy/         — client CLI (existing, extend with submit/status/outputs)
  flowy-server/  — REST API server + DAG scheduler
  flowy-daemon/  — task poller + Docker executor
```

Each `cmd/` directory has its own `main.go`. Shared logic lives in internal packages:

```
internal/
  api/           — shared HTTP client/server types (request/response structs)
  store/         — SQLite queries via database/sql + goose migrations
  config/        — YAML config loading
```

Build with:
```bash
go build -o bin/flowy          ./cmd/flowy
go build -o bin/flowy-server   ./cmd/flowy-server
go build -o bin/flowy-daemon   ./cmd/flowy-daemon
```

---

### Alternatives Considered (v1.1)

| Recommended | Alternative | Why Not |
|-------------|-------------|---------|
| `chi v5` | `net/http ServeMux` | No middleware grouping; sub-router per auth scope is manual boilerplate |
| `chi v5` | `gin` | Custom context type breaks net/http compatibility; reflection-based binding is opaque |
| `chi v5` | `echo` | Same issue as gin; echo's context is not http.ResponseWriter |
| `modernc.org/sqlite` | `mattn/go-sqlite3` | Requires CGO; breaks cross-compilation; performance gap irrelevant for flowy's workload |
| `goose v3` | `golang-migrate` | golang-migrate's embedded FS API is more verbose; goose is simpler for a single-database app |
| `gopkg.in/yaml.v3` | `spf13/viper` | 15+ transitive deps; goroutine safety issues; maintenance-only; overkill for a flat config struct |
| stdlib polling | Redis/NATS | Adds external infrastructure dependency; HPC tasks run minutes-to-hours, sub-second dispatch irrelevant |
| stdlib fs ops | `spf13/afero` | Unnecessary abstraction; `t.TempDir()` covers test isolation needs without a mock FS |

---

### What NOT to Add

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| **GORM** | ORM magic obscures SQL; auto-migration is hard to audit; reflection overhead; poor SQLite support historically | `database/sql` + goose migrations + hand-written queries |
| **sqlc** | Code generation adds a build step and external tool dependency; 3-4 tables with simple queries do not justify it | `database/sql` with explicit `Scan` calls — 50-100 lines total |
| **JWT / OAuth libraries** | v1.1 auth is a static shared secret; full auth framework is premature optimization | 15-line custom `BearerToken` middleware |
| **Redis / NATS / RabbitMQ** | External infrastructure; push messaging is not needed when polling fits the declared architecture | stdlib `time.Ticker` polling loop |
| **spf13/afero** | Filesystem abstraction not needed until S3 milestone | stdlib `os`, `t.TempDir()` for tests |
| **protobuf / gRPC** | REST + JSON is sufficient; gRPC adds codegen toolchain and client complexity | `encoding/json` + chi |
| **OpenTelemetry (for v1.1)** | Docker SDK already imports OTEL transitively; don't add more OTEL instrumentation until observability is a stated requirement | stdlib `log/slog` + request IDs for tracing |

---

## Version Compatibility

| Package | Compatible With | Notes |
|---------|-----------------|-------|
| `chi/v5 v5.2.5` | Go 1.22+ | Requires Go 1.22 minimum; go.mod is `go 1.25.0` — compatible |
| `modernc.org/sqlite v1.36.x` | Go 1.18+ | CGO-free; works with all Go 1.24+ |
| `goose/v3 v3.27.0` | Go 1.25+ | Minimum Go 1.25; matches go.mod `go 1.25.0` |
| `gopkg.in/yaml.v3 v3.0.1` | Go 1.13+ | Stable; no compatibility concerns |
| `go-chi/httplog/v3 v3.3.0` | Go 1.21+ | Requires log/slog (Go 1.21+); compatible with 1.25 |
| `golang.org/x/sync v0.20.0` | Already in go.mod | Already a transitive dependency via Docker SDK |
| `aws-sdk-go-v2/service/s3 v1.78.x` | Go 1.21+ | go.mod is `go 1.25.0` — compatible |
| `aws-sdk-go-v2/config v1.32.x` | Go 1.21+ | Same module group; update config + credentials + s3 together |
| `testcontainers-go v0.36.x` | Go 1.21+, Docker 20.10+ | Requires Docker socket; DinD needs `TESTCONTAINERS_RYUK_DISABLED=true` |
| `testcontainers-go/modules/minio v0.36.x` | Must match base testcontainers version | Minor version must match base module |
| `golang-jwt/jwt/v5 v5.3.1` | Go 1.18+ | NEW v1.3 direct dependency; compatible with Go 1.25 |
| `golang.org/x/crypto v0.49.0` | Go 1.20+ | Promote from indirect in v1.3; bcrypt for password hashing |
| `opencontainers/go-digest v1.0.0` | Go 1.13+ | Already transitive; promote to direct in v1.3 for registry content hashing |

---

## Sources

- [go-chi/chi releases — GitHub](https://github.com/go-chi/chi/releases) — v5.2.5 confirmed latest stable (February 2026)
- [chi/v5/middleware — pkg.go.dev](https://pkg.go.dev/github.com/go-chi/chi/v5/middleware) — RequestID, Logger, Recoverer, BasicAuth documented
- [go-chi/httplog/v3 — pkg.go.dev](https://pkg.go.dev/github.com/go-chi/httplog/v3) — v3.3.0, slog-based, zero external dependencies
- [modernc.org/sqlite — pkg.go.dev](https://pkg.go.dev/modernc.org/sqlite) — v1.46.1 (Feb 2026), SQLite 3.51.2, 2562+ importers
- [sqlite-cgo-no-cgo benchmarks — multiprocessio/GitHub](https://github.com/multiprocessio/sqlite-cgo-no-cgo) — performance comparison mattn vs modernc
- [goose releases — GitHub](https://github.com/pressly/goose/releases) — v3.27.0 confirmed latest (February 2025); requires Go 1.25+
- [gopkg.in/yaml.v3 — pkg.go.dev](https://pkg.go.dev/gopkg.in/yaml.v3) — v3.0.1 stable
- [Go's 1.22+ ServeMux vs Chi — calhoun.io](https://www.calhoun.io/go-servemux-vs-chi/) — middleware grouping gap analysis
- [chi v5 docs — go-chi.io](https://go-chi.io/) — official documentation
- [aws/aws-sdk-go-v2/service/s3 — pkg.go.dev](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3) — v1.78.x latest March 2026; 5,227 importers
- [aws/aws-sdk-go-v2/config — pkg.go.dev](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/config) — v1.32.11 March 2026
- [aws/aws-sdk-go-v2 issue #2573 — GitHub](https://github.com/aws/aws-sdk-go-v2/issues/2573) — UsePathStyle confirmed working for MinIO
- [aws/aws-sdk-go — GitHub](https://github.com/aws/aws-sdk-go) — v1 end-of-support declaration
- [feature/s3/manager — pkg.go.dev](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/feature/s3/manager) — deprecated March 2026
- [feature/s3/transfermanager — pkg.go.dev](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager) — v0.1.8 unstable
- [testcontainers-go/modules/minio — golang.testcontainers.org](https://golang.testcontainers.org/modules/minio/) — official documentation
- [minio/minio-go/v7 — pkg.go.dev](https://pkg.go.dev/github.com/minio/minio-go/v7) — v7.0.99 March 2026; evaluated and not recommended
- [golang-jwt/jwt v5 — pkg.go.dev](https://pkg.go.dev/github.com/golang-jwt/jwt/v5) — v5.3.1 January 28, 2026; 13,000+ importers; HIGH confidence
- [golang-jwt/jwt releases — GitHub](https://github.com/golang-jwt/jwt/releases) — v5.3.1 confirmed latest stable
- [golang.org/x/crypto/bcrypt — pkg.go.dev](https://pkg.go.dev/golang.org/x/crypto/bcrypt) — v0.49.0 March 11, 2026; Go team maintained
- [opencontainers/go-digest — pkg.go.dev](https://pkg.go.dev/github.com/opencontainers/go-digest) — v1.0.0 stable; already in go.mod
- [go-chi/jwtauth v5 — pkg.go.dev](https://pkg.go.dev/github.com/go-chi/jwtauth/v5) — v5.4.0 February 26, 2026; depends on lestrrat-go/jwx; evaluated and not recommended
- [lestrrat-go/jwx releases — GitHub](https://github.com/lestrrat-go/jwx/releases) — v3.0.13 released; full JOSE suite; too broad for this use case

---

## v1.0 Stack (Preserved — No Changes)

| Layer | Library | Version | Purpose | Confidence |
|-------|---------|---------|---------|------------|
| **Language** | Go | 1.25.0 | Core language | HIGH |
| **Parser** | Hand-written recursive descent + Pratt | — | WDL 1.2 lexer + AST builder | HIGH |
| **Container** | `github.com/docker/docker` (moby client) | v28.x+incompatible | Docker API: pull images, run containers, stream logs | HIGH |
| **CLI** | `github.com/spf13/cobra` | v1.10.x | `flowy run`, `flowy check` subcommands | HIGH |
| **Testing** | `github.com/stretchr/testify` | v1.11.1 | Assertions, require, test suites | HIGH |
| **Logging** | `log/slog` (stdlib) | Go 1.21+ | Structured logging, zero external dependency | HIGH |
| **Concurrency** | stdlib goroutines + `errgroup` | — | Parallel task/scatter execution, DAG scheduling | HIGH |

See the original v1.0 research in git history for full rationale on these choices.

---

*Stack research for: flowy v1.3 user auth, project RBAC, WDL registry, provenance tracking*
*Researched: 2026-03-17*
