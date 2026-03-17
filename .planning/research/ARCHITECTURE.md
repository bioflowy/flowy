# Architecture Research

**Domain:** WDL workflow engine — user auth, project RBAC, WDL registry, provenance tracking (v1.3)
**Researched:** 2026-03-17
**Confidence:** HIGH (direct codebase analysis; all existing code read in full)

---

## Existing Architecture (v1.2 Baseline)

Before describing what changes, the baseline must be understood precisely.

### Current Component Map

```
  ┌─────────────────────────────────────────────────────────────────┐
  │  flowy (client CLI)  cmd/flowy/main.go                          │
  │  check | run | submit | status | outputs                        │
  │  internal/client/  — HTTP wrappers around api.* types           │
  └──────────────────────────┬──────────────────────────────────────┘
                             │ HTTP REST (no auth today)
  ┌──────────────────────────▼──────────────────────────────────────┐
  │  flowy-server  cmd/flowy-server/main.go                         │
  │  ┌──────────────┐  ┌──────────────┐  ┌───────────────────────┐  │
  │  │ chi v5 router│  │ DAG Scheduler│  │  Stale Detection      │  │
  │  │ server.go    │  │ dispatch.go  │  │  (goroutine)          │  │
  │  └──────────────┘  └──────────────┘  └───────────────────────┘  │
  │                                                                  │
  │  User-facing routes:                                             │
  │    GET  /health                                                  │
  │    POST /workflows                                               │
  │    GET  /workflows/{id}/status                                   │
  │    GET  /workflows/{id}/outputs                                  │
  │    POST /workflows/{id}/abort                                    │
  │                                                                  │
  │  Daemon-facing routes:                                           │
  │    POST /tasks/run                                               │
  │    POST /tasks/claim                                             │
  │    POST /tasks/{id}/result                                       │
  │    POST /heartbeat                                               │
  │                                                                  │
  │  ┌──────────────────────────────────────────────────────────┐   │
  │  │  internal/store/Store (interface)                         │   │
  │  │  internal/store/sqlite/SQLiteStore (implementation)       │   │
  │  │  Tables: runs, tasks                                      │   │
  │  │  Migrations: goose embedded (migrations/*.sql)            │   │
  │  └──────────────────────────────────────────────────────────┘   │
  │  ┌──────────────────────────────────────────────────────────┐   │
  │  │  internal/storage/Backend (interface)                     │   │
  │  │  LocalBackend | S3Backend                                 │   │
  │  └──────────────────────────────────────────────────────────┘   │
  └─────────────────────────────────────────────────────────────────┘
                             │ POST /tasks/claim  (poll)
  ┌──────────────────────────▼──────────────────────────────────────┐
  │  flowy-daemon  cmd/flowy-daemon/main.go                         │
  │  internal/daemon/daemon.go                                      │
  │  workerLoop → claimTask → executeTask (Local or S3)             │
  │  LED pattern for S3: Localize → Execute → Delocalize            │
  └─────────────────────────────────────────────────────────────────┘
```

### Key Structural Properties to Preserve

- `Server` struct is constructed with injected `store.Store` and `storage.Backend`; easy to extend constructor.
- `chi.Mux.newRouter()` centralises all route registration; adding route groups is clean.
- `store.Store` is an interface — new tables can be added via new methods without breaking existing implementations.
- Goose embedded migrations are the established pattern for schema changes.
- `api` package is a leaf package (no internal imports); all DTOs live here.
- Daemon has no authentication token infrastructure; it identifies itself only by `DaemonID` string.
- Config is loaded from YAML by all three binaries; `config.Config` struct is shared.

---

## Target Architecture (v1.3)

### System Overview: After v1.3

```
  ┌─────────────────────────────────────────────────────────────────┐
  │  flowy (client CLI)                                              │
  │  + login / logout / whoami commands (new)                       │
  │  + registry push / pull / list commands (new)                   │
  │  Stores JWT in ~/.flowy-token (or env FLOWY_TOKEN)              │
  │  Sends Authorization: Bearer <token> on all server requests      │
  └──────────────────────┬──────────────────────────────────────────┘
                         │ HTTP REST
  ┌──────────────────────▼──────────────────────────────────────────┐
  │  flowy-server                                                    │
  │                                                                  │
  │  ┌──────────────────────────────────────────────────────────┐   │
  │  │  Auth middleware (chi middleware stack)                   │   │
  │  │  JWTMiddleware: validates Bearer token, sets userID in ctx│   │
  │  │  ExemptRoutes: POST /auth/login, GET /health              │   │
  │  └───────────────────────────────────────────────────────────┘   │
  │                                                                  │
  │  Route groups:                                                   │
  │    Public:  POST /auth/login, GET /health                        │
  │    User:    /projects/**, /registry/**, /workflows/**,           │
  │             /runs/**                                             │
  │    Daemon:  /tasks/**, /heartbeat (token or shared secret)       │
  │                                                                  │
  │  ┌──────────────────────────────────────────────────────────┐   │
  │  │  RBAC middleware (per project route group)                │   │
  │  │  RequireProjectRole(role): reads project_members,        │   │
  │  │  returns 403 if userID lacks minimum role                 │   │
  │  └──────────────────────────────────────────────────────────┘   │
  │                                                                  │
  │  Store interface — extended with new method groups:              │
  │  ┌──────────────────────────────────────────────────────────┐   │
  │  │  User CRUD (users table)                                  │   │
  │  │  Project CRUD + membership (projects, project_members)    │   │
  │  │  Registry CRUD (registry_entries, registry_tags)          │   │
  │  │  Provenance write/read (provenance_records)               │   │
  │  │  Runs / Tasks (existing — extended with project_id,       │   │
  │  │               registry_ref, provenance linking)           │   │
  │  └──────────────────────────────────────────────────────────┘   │
  └──────────────────────┬──────────────────────────────────────────┘
                         │ /tasks/claim  (daemon token or shared secret)
  ┌──────────────────────▼──────────────────────────────────────────┐
  │  flowy-daemon                                                    │
  │  Daemon authenticates with server via daemon-token in config     │
  │  (no user identity; daemon operates in system context)           │
  │  TaskClaimResponse extended with provenance_run_id link          │
  └─────────────────────────────────────────────────────────────────┘
```

---

## Component Responsibilities: New vs Modified

| Component | Status | Responsibility |
|-----------|--------|----------------|
| `internal/auth/` | NEW | JWT sign/verify, password hashing (bcrypt), token store helpers |
| `internal/registry/` | NEW | WDL content-hash computation, registry store operations, tag resolution |
| `internal/provenance/` | NEW | Provenance record construction, linking outputs to registry refs |
| `internal/store/Store` interface | MODIFIED | Add user/project/registry/provenance method groups |
| `internal/store/sqlite/SQLiteStore` | MODIFIED | Implement new Store methods; add goose migrations 00004–00008 |
| `internal/api/types.go` | MODIFIED | Add auth, project, registry, provenance DTOs |
| `internal/server/server.go` | MODIFIED | Server struct gets `jwtSecret []byte`; `newRouter` adds auth middleware and new route groups |
| `internal/server/handlers.go` | MODIFIED | Auth handlers (login), project handlers, registry handlers, provenance handlers |
| `internal/server/dispatch.go` | MODIFIED | `makeDispatchFn` records provenance after task completion |
| `internal/config/config.go` | MODIFIED | Add `AuthConfig` (jwt secret, token TTL) and `DaemonConfig.Token` |
| `cmd/flowy/main.go` | MODIFIED | Add `login`, `logout`, `whoami`, `registry push/pull/list` subcommands; token file management |
| `internal/client/` | MODIFIED | Add `Authorization` header to all requests; add registry client methods |
| `internal/daemon/daemon.go` | MODIFIED | Send daemon token in `Authorization` header on claim/result/heartbeat |

---

## New Packages

### `internal/auth/`

Responsible for: JWT token generation and validation, password hashing, daemon authentication.

```
internal/auth/
├── jwt.go          # GenerateToken(userID, secret, ttl) string
                    # ValidateToken(token, secret) (Claims, error)
                    # Claims struct: {UserID, Role, Exp}
├── password.go     # HashPassword(plain) (string, error) — bcrypt cost 12
                    # CheckPassword(plain, hash) error
└── middleware.go   # JWTMiddleware(secret []byte) func(http.Handler) http.Handler
                    # Sets chi context key "userID" to validated claims.UserID
                    # Returns 401 for missing/invalid/expired tokens
                    # DaemonAuthMiddleware(secret string) — validates daemon shared secret
```

Library choice: `golang.org/x/crypto` is already in go.sum (via docker transitively) — use it for bcrypt. For JWT, add `github.com/golang-jwt/jwt/v5` (the maintained successor to dgrijalva/jwt-go; MIT license, ~5k stars, standard HMAC-SHA256 support). No OAuth2/LDAP per project scope.

### `internal/registry/`

Responsible for: WDL content addressing (SHA-256 of normalized content), registry store operations, tag management, WDL parsing from registry entries.

```
internal/registry/
├── hash.go         # ContentHash(wdlSource []byte) string
                    #   → "sha256:" + hex.EncodeToString(sha256.Sum256(source))
├── store.go        # RegistryStore interface methods (thin wrappers over store.Store)
                    # Push(ctx, projectID, name, wdlSource, tag, pushedByUserID) (*Entry, error)
                    # Resolve(ctx, projectID, ref) (*Entry, error)
                    #   ref = "name:tag" or "name@sha256:abc..."
├── parse.go        # ParseFromEntry(e *Entry) (*ast.Document, error)
                    # Parses WDL source stored in registry_entries.wdl_source
└── types.go        # Entry struct, Tag struct (mirrors DB schema)
```

### `internal/provenance/`

Responsible for: building provenance records when a run completes, linking outputs to their source registry entry and input parameters.

```
internal/provenance/
├── record.go       # Record struct: {RunID, ProjectID, RegistryEntryHash,
                    #   RegistryTag, InputsJSON, OutputsJSON, CompletedAt}
└── write.go        # WriteRecord(ctx, store, record) error
                    # Called from dispatch.go after runWorkflow/waitForTask completes
```

---

## Database Schema: New Migrations

The existing migration sequence is `00001_init.sql`, `00002_task_outputs.sql`, `00003_run_outputs.sql`. New migrations follow in order.

### Migration 00004: Users

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    id           TEXT PRIMARY KEY,        -- UUID v4
    username     TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,          -- bcrypt hash
    created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
-- +goose Down
DROP TABLE IF EXISTS users;
```

### Migration 00005: Projects and Membership

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS projects (
    id           TEXT PRIMARY KEY,        -- UUID v4
    name         TEXT UNIQUE NOT NULL,
    created_by   TEXT NOT NULL REFERENCES users(id),
    created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS project_members (
    project_id  TEXT NOT NULL REFERENCES projects(id),
    user_id     TEXT NOT NULL REFERENCES users(id),
    role        TEXT NOT NULL DEFAULT 'viewer', -- 'admin' | 'executor' | 'viewer'
    PRIMARY KEY (project_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_project_members_user ON project_members(user_id);
-- +goose Down
DROP TABLE IF EXISTS project_members;
DROP TABLE IF EXISTS projects;
```

### Migration 00006: WDL Registry

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS registry_entries (
    content_hash  TEXT PRIMARY KEY,       -- "sha256:<hex>" — content-addressed
    project_id    TEXT NOT NULL REFERENCES projects(id),
    name          TEXT NOT NULL,          -- human name: "samtools-sort"
    wdl_source    TEXT NOT NULL,          -- full WDL source text
    pushed_by     TEXT NOT NULL REFERENCES users(id),
    created_at    DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS registry_tags (
    project_id    TEXT NOT NULL REFERENCES projects(id),
    name          TEXT NOT NULL,          -- "samtools-sort"
    tag           TEXT NOT NULL,          -- "v1.2", "latest"
    content_hash  TEXT NOT NULL REFERENCES registry_entries(content_hash),
    updated_at    DATETIME NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (project_id, name, tag)
);

CREATE INDEX IF NOT EXISTS idx_registry_entries_project ON registry_entries(project_id);
CREATE INDEX IF NOT EXISTS idx_registry_tags_project    ON registry_tags(project_id);
-- +goose Down
DROP TABLE IF EXISTS registry_tags;
DROP TABLE IF EXISTS registry_entries;
```

### Migration 00007: Run Project Linkage

```sql
-- +goose Up
ALTER TABLE runs ADD COLUMN project_id      TEXT REFERENCES projects(id);
ALTER TABLE runs ADD COLUMN registry_ref    TEXT; -- "name:tag" or "name@sha256:..."
                                                  -- NULL for runs submitted without registry
-- +goose Down
-- SQLite does not support DROP COLUMN; migration is irreversible in SQLite.
-- For rollback: recreate table without these columns.
```

Note: SQLite supports `ALTER TABLE ADD COLUMN` only (not DROP COLUMN). This is acceptable — nullable columns with no default are backward compatible.

### Migration 00008: Provenance

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS provenance_records (
    id                   TEXT PRIMARY KEY,  -- UUID v4
    run_id               TEXT NOT NULL REFERENCES runs(id),
    project_id           TEXT NOT NULL REFERENCES projects(id),
    registry_entry_hash  TEXT REFERENCES registry_entries(content_hash),
    registry_tag         TEXT,              -- tag at time of execution
    inputs_json          TEXT NOT NULL,     -- snapshot of run inputs
    outputs_json         TEXT NOT NULL,     -- snapshot of run outputs
    completed_at         DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_provenance_run      ON provenance_records(run_id);
CREATE INDEX IF NOT EXISTS idx_provenance_project  ON provenance_records(project_id);
CREATE INDEX IF NOT EXISTS idx_provenance_hash     ON provenance_records(registry_entry_hash);
-- +goose Down
DROP TABLE IF EXISTS provenance_records;
```

---

## Store Interface Extensions

The `store.Store` interface gains four new method groups. The existing `SQLiteStore` implements all of them. New methods are appended — existing callers are unaffected.

```go
// User operations
CreateUser(ctx context.Context, u *User) error
GetUserByUsername(ctx context.Context, username string) (*User, error)
GetUserByID(ctx context.Context, id string) (*User, error)

// Project operations
CreateProject(ctx context.Context, p *Project) error
GetProjectByID(ctx context.Context, id string) (*Project, error)
ListProjects(ctx context.Context, userID string) ([]Project, error)
AddProjectMember(ctx context.Context, projectID, userID, role string) error
GetProjectRole(ctx context.Context, projectID, userID string) (string, error)
RemoveProjectMember(ctx context.Context, projectID, userID string) error

// Registry operations
CreateRegistryEntry(ctx context.Context, e *RegistryEntry) error
GetRegistryEntry(ctx context.Context, contentHash string) (*RegistryEntry, error)
ListRegistryEntries(ctx context.Context, projectID string) ([]RegistryEntry, error)
SetRegistryTag(ctx context.Context, projectID, name, tag, contentHash string) error
GetRegistryTag(ctx context.Context, projectID, name, tag string) (*RegistryTag, error)
ListRegistryTags(ctx context.Context, projectID, name string) ([]RegistryTag, error)

// Provenance operations
CreateProvenanceRecord(ctx context.Context, r *ProvenanceRecord) error
GetProvenanceRecord(ctx context.Context, id string) (*ProvenanceRecord, error)
ListProvenanceByRun(ctx context.Context, runID string) ([]ProvenanceRecord, error)
ListProvenanceByProject(ctx context.Context, projectID string) ([]ProvenanceRecord, error)
ListProvenanceByRegistryEntry(ctx context.Context, contentHash string) ([]ProvenanceRecord, error)
```

These methods are added to the `store.Store` interface in `internal/store/store.go`, with model types (`User`, `Project`, `RegistryEntry`, etc.) defined there alongside existing `Run` and `Task`.

---

## API Routes: New Endpoints

All new user-facing routes sit behind `JWTMiddleware`. Routes that are project-scoped additionally check RBAC via `RequireProjectRole` middleware applied to the route group.

```
POST   /auth/login                         → LoginHandler       (public)
POST   /auth/logout                        → LogoutHandler      (JWT required; client-side token drop)
GET    /auth/whoami                        → WhoamiHandler      (JWT required)

POST   /users                              → CreateUserHandler  (JWT required; admin only or open reg)

GET    /projects                           → ListProjectsHandler (JWT required)
POST   /projects                           → CreateProjectHandler (JWT required)
GET    /projects/{projectID}               → GetProjectHandler  (JWT; viewer+)
POST   /projects/{projectID}/members       → AddMemberHandler   (JWT; admin)
DELETE /projects/{projectID}/members/{uid} → RemoveMemberHandler (JWT; admin)

GET    /projects/{projectID}/registry                  → ListRegistryHandler (JWT; viewer+)
POST   /projects/{projectID}/registry                  → PushHandler         (JWT; executor+)
GET    /projects/{projectID}/registry/{name}           → ListTagsHandler     (JWT; viewer+)
GET    /projects/{projectID}/registry/{name}/{ref}     → PullHandler         (JWT; viewer+)

POST   /projects/{projectID}/workflows                 → SubmitHandler (JWT; executor+)
GET    /projects/{projectID}/runs/{id}/status          → StatusHandler (JWT; viewer+)
GET    /projects/{projectID}/runs/{id}/outputs         → OutputsHandler (JWT; viewer+)
POST   /projects/{projectID}/runs/{id}/abort           → AbortHandler  (JWT; executor+)

GET    /projects/{projectID}/provenance                → ListProvenanceHandler (JWT; viewer+)
GET    /projects/{projectID}/provenance/{id}           → GetProvenanceHandler  (JWT; viewer+)

-- Existing daemon routes unchanged, add daemon token check:
POST   /tasks/claim                        → ClaimHandler  (daemon token)
POST   /tasks/{id}/result                  → ResultHandler (daemon token)
POST   /heartbeat                          → HeartbeatHandler (daemon token)
```

The existing unauthenticated workflow routes (`POST /workflows`, `GET /workflows/{id}/status`, etc.) are either:
- Kept as-is for backward compatibility with the spectool conformance suite (they can require no auth in local test mode), OR
- Deprecated in favour of the project-scoped routes.

Recommendation: Keep existing unauthenticated routes operational when `auth.enabled: false` in config (default for spectool compatibility). When `auth.enabled: true`, existing routes are gated by JWT middleware. This avoids breaking spectool while still enabling v1.3 features.

---

## Auth Data Flow

### Login Flow (Client → Server)

```
flowy login --username alice --password secret
    ↓
POST /auth/login  {username, password}
    ↓
server: GetUserByUsername → CheckPassword (bcrypt compare)
    ↓ success
GenerateToken(userID, jwtSecret, ttl=24h) → signed JWT string
    ↓
Response: {token: "eyJ..."}
    ↓
Client writes token to ~/.flowy-token
```

### Authenticated Request Flow

```
flowy submit project1 workflow.wdl
    ↓ reads ~/.flowy-token
POST /projects/{id}/workflows
  Authorization: Bearer eyJ...
    ↓
JWTMiddleware: ValidateToken → extracts userID → sets in chi context
    ↓
RequireProjectRole("executor"): GetProjectRole(projectID, userID) → check >= executor
    ↓
SubmitHandler: reads projectID from URL, userID from context
Creates run with project_id = projectID, submits as before
```

### Daemon Authentication

The daemon does not have a user identity. It authenticates with a shared secret configured in `flowy.yaml`:

```yaml
daemon:
  server_url: http://localhost:8124
  token: "daemon-shared-secret-here"   # NEW
```

The daemon sends `Authorization: Bearer <token>` on all requests. The server validates this against `config.Auth.DaemonToken` (a configured secret, not a user JWT). This keeps daemons out of the RBAC system entirely — daemons can claim any task, not just tasks for specific projects.

---

## Registry Data Flow

### Push Flow

```
flowy registry push --project proj1 --name samtools-sort --tag v1.2 samtools_sort.wdl
    ↓
Client reads WDL file content as bytes
POST /projects/proj1/registry
  Body: {name, tag, wdl_source}
  Authorization: Bearer <executor token>
    ↓
Server: ContentHash(wdl_source) → "sha256:abc123..."
Check: registry_entries already has this hash? → idempotent upsert
CreateRegistryEntry if new
SetRegistryTag(projectID, name, tag, hash) → upsert registry_tags
    ↓
Response: {content_hash, name, tag}
```

### Pull / Execute from Registry

```
flowy run --project proj1 --registry samtools-sort:v1.2 --i inputs.json
    ↓
POST /projects/proj1/workflows
  Body: {registry_ref: "samtools-sort:v1.2", inputs}
    ↓
Server: Resolve("samtools-sort:v1.2") → registry_entries row → wdl_source
ParseFromEntry(entry) → *ast.Document  (in-memory, no filesystem path)
Create run with registry_ref = "samtools-sort:v1.2"
Proceed as handleSubmit does now (runWorkflow goroutine)
    ↓
On completion: CreateProvenanceRecord{run_id, project_id, registry_entry_hash, ...}
```

The WDL content is already in-memory from the registry entry. The daemon receives `WDLContent` in `TaskClaimResponse` (this field already exists since v1.2 — used to avoid shared FS dependency). The content is the same WDL source stored in `registry_entries.wdl_source`. No filesystem path is needed.

---

## Provenance Data Flow

```
dispatch.go: runWorkflow completes successfully
    ↓
if run.RegistryRef != "" && run.ProjectID != "":
    CreateProvenanceRecord{
        ID:                 uuid.New(),
        RunID:              runID,
        ProjectID:          run.ProjectID,
        RegistryEntryHash:  resolvedEntry.ContentHash,
        RegistryTag:        run.RegistryRef (tag portion, if any),
        InputsJSON:         run.InputsJSON,  // already stored in runs table
        OutputsJSON:        run.OutputsJSON, // already updated by runWorkflow
        CompletedAt:        time.Now(),
    }
```

Provenance records are written after `UpdateRunOutputs` succeeds. They are append-only — no updates, only inserts. Querying by `registry_entry_hash` gives the full execution history for any specific WDL version.

---

## chi Router Integration

The existing `newRouter()` is replaced with a structured version using route groups:

```go
func (s *Server) newRouter() *chi.Mux {
    r := chi.NewRouter()
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)

    // Public routes (no auth).
    r.Get("/health", s.handleHealth)
    r.Post("/auth/login", s.handleLogin)

    // Authenticated user routes.
    r.Group(func(r chi.Router) {
        r.Use(auth.JWTMiddleware(s.jwtSecret))

        r.Get("/auth/whoami", s.handleWhoami)
        r.Post("/auth/logout", s.handleLogout)
        r.Post("/users", s.handleCreateUser)
        r.Get("/projects", s.handleListProjects)
        r.Post("/projects", s.handleCreateProject)

        // Project-scoped routes.
        r.Route("/projects/{projectID}", func(r chi.Router) {
            r.Use(s.projectContextMiddleware) // loads project, sets in ctx
            r.Get("/", s.handleGetProject)

            // Admin-only.
            r.Group(func(r chi.Router) {
                r.Use(auth.RequireProjectRole("admin"))
                r.Post("/members", s.handleAddMember)
                r.Delete("/members/{userID}", s.handleRemoveMember)
            })

            // Executor+ for write operations.
            r.Group(func(r chi.Router) {
                r.Use(auth.RequireProjectRole("executor"))
                r.Post("/registry", s.handleRegistryPush)
                r.Post("/workflows", s.handleProjectSubmit)
                r.Post("/runs/{runID}/abort", s.handleProjectAbort)
            })

            // Viewer+ for read operations.
            r.Group(func(r chi.Router) {
                r.Use(auth.RequireProjectRole("viewer"))
                r.Get("/registry", s.handleRegistryList)
                r.Get("/registry/{name}", s.handleRegistryListTags)
                r.Get("/registry/{name}/{ref}", s.handleRegistryPull)
                r.Get("/runs/{runID}/status", s.handleProjectStatus)
                r.Get("/runs/{runID}/outputs", s.handleProjectOutputs)
                r.Get("/provenance", s.handleListProvenance)
                r.Get("/provenance/{id}", s.handleGetProvenance)
            })
        })

        // Backward-compat unauthenticated workflow routes (only if auth.enabled: false).
        // When auth.enabled: true, these require JWT too.
        r.Post("/workflows", s.handleSubmit)
        r.Get("/workflows/{id}/status", s.handleStatus)
        r.Get("/workflows/{id}/outputs", s.handleOutputs)
        r.Post("/workflows/{id}/abort", s.handleAbort)
    })

    // Daemon-facing routes (daemon shared secret, not user JWT).
    r.Group(func(r chi.Router) {
        r.Use(auth.DaemonAuthMiddleware(s.cfg.Auth.DaemonToken))
        r.Post("/tasks/run", s.handleTaskRun)
        r.Post("/tasks/claim", s.handleClaim)
        r.Post("/tasks/{id}/result", s.handleResult)
        r.Post("/heartbeat", s.handleHeartbeat)
    })

    r.NotFound(func(w http.ResponseWriter, req *http.Request) {
        writeError(w, http.StatusNotFound, "not found")
    })
    r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
        writeError(w, http.StatusMethodNotAllowed, "method not allowed")
    })

    return r
}
```

---

## Server Struct Changes

```go
type Server struct {
    cfg       *config.Config
    store     store.Store
    backend   storage.Backend
    jwtSecret []byte          // NEW: parsed from cfg.Auth.JWTSecret at construction

    router *chi.Mux

    mu   sync.Mutex
    runs map[string]context.CancelFunc

    wg        sync.WaitGroup
    stopStale context.CancelFunc
    staleInterval time.Duration
}

func New(cfg *config.Config, st store.Store, backend storage.Backend) *Server {
    s := &Server{
        cfg:           cfg,
        store:         st,
        backend:       backend,
        jwtSecret:     []byte(cfg.Auth.JWTSecret),
        runs:          make(map[string]context.CancelFunc),
        staleInterval: 30 * time.Second,
    }
    s.router = s.newRouter()
    return s
}
```

---

## Config Changes

```go
type AuthConfig struct {
    Enabled     bool   `yaml:"enabled"`       // Default false (spectool compat)
    JWTSecret   string `yaml:"jwt_secret"`    // HMAC-SHA256 secret; min 32 bytes in prod
    TokenTTLHrs int    `yaml:"token_ttl_hrs"` // Default 24
    DaemonToken string `yaml:"daemon_token"`  // Shared secret for daemon auth
}

type Config struct {
    // ... existing fields ...
    Auth AuthConfig `yaml:"auth"`
}
```

When `Auth.Enabled = false` (default), all middleware is bypassed — spectool tests continue to pass without modification. When `Auth.Enabled = true`, JWT middleware is applied.

---

## Recommended Project Structure After v1.3

```
internal/
├── auth/                       # NEW — JWT, bcrypt, middleware
│   ├── jwt.go
│   ├── password.go
│   └── middleware.go
├── registry/                   # NEW — WDL registry operations
│   ├── hash.go
│   ├── store.go
│   ├── parse.go
│   └── types.go
├── provenance/                 # NEW — provenance record construction
│   ├── record.go
│   └── write.go
├── api/
│   └── types.go                # MODIFIED — auth, project, registry, provenance DTOs
├── config/
│   └── config.go               # MODIFIED — AuthConfig struct added
├── store/
│   ├── store.go                # MODIFIED — new method groups added to interface
│   ├── rundir.go               # UNCHANGED
│   └── sqlite/
│       ├── sqlite.go           # UNCHANGED
│       ├── runs.go             # MODIFIED — UpdateRun adds project_id / registry_ref
│       ├── tasks.go            # UNCHANGED
│       ├── users.go            # NEW — user CRUD implementations
│       ├── projects.go         # NEW — project + membership CRUD implementations
│       ├── registry.go         # NEW — registry_entries + registry_tags implementations
│       ├── provenance.go       # NEW — provenance_records implementations
│       └── migrations/
│           ├── 00001_init.sql          # existing
│           ├── 00002_task_outputs.sql  # existing
│           ├── 00003_run_outputs.sql   # existing
│           ├── 00004_users.sql         # NEW
│           ├── 00005_projects.sql      # NEW
│           ├── 00006_registry.sql      # NEW
│           ├── 00007_runs_project.sql  # NEW
│           └── 00008_provenance.sql    # NEW
├── server/
│   ├── server.go               # MODIFIED — jwtSecret field, newRouter restructured
│   ├── handlers.go             # MODIFIED — existing handlers unchanged; new handlers added
│   ├── handlers_auth.go        # NEW — login, logout, whoami, createUser
│   ├── handlers_project.go     # NEW — project CRUD, membership
│   ├── handlers_registry.go    # NEW — push, pull, list
│   ├── handlers_provenance.go  # NEW — list, get
│   ├── dispatch.go             # MODIFIED — provenance write after completion
│   └── rundir.go               # UNCHANGED
├── client/                     # MODIFIED — add Authorization header, registry methods
├── storage/                    # UNCHANGED
└── daemon/
    └── daemon.go               # MODIFIED — send daemon token in Authorization header
```

---

## Integration Points: Explicit Change List

### What Stays Completely Unchanged

- `wdl/` packages (parser, evaluator, task executor, values, types) — zero changes
- `internal/storage/` (Backend interface, LocalBackend, S3Backend) — zero changes
- `internal/store/sqlite/sqlite.go` — zero changes (new SQL files added, existing untouched)
- `internal/store/sqlite/tasks.go` — zero changes
- `internal/server/rundir.go` — zero changes
- `cmd/flowy-daemon/main.go` — minimal change: pass daemon token from config

### What Is Modified

**`internal/store/store.go`** — Interface extended with new method groups. Existing methods untouched. Implementations split into per-concern files (`users.go`, `projects.go`, etc.) to keep file sizes manageable.

**`internal/api/types.go`** — New DTO structs appended. Existing structs (`SubmitRequest`, `TaskClaimResponse`, etc.) gain optional fields only (zero-value backward compatible). Specifically, `SubmitRequest` gains optional `ProjectID string` and `RegistryRef string` fields.

**`internal/server/server.go`** — `jwtSecret` field added. `newRouter()` restructured to route groups. Constructor signature unchanged (`New(cfg, store, backend)`).

**`internal/server/handlers.go`** — Existing handlers untouched. New handler files split by concern to avoid bloat.

**`internal/server/dispatch.go`** — `runWorkflow` and `waitForTaskAndFinalize` gain provenance write call after successful completion. The write is non-fatal (log on error, do not fail the run).

**`internal/config/config.go`** — `AuthConfig` struct added. `applyDefaults` sets `Auth.Enabled = false` and `Auth.TokenTTLHrs = 24`. No existing field changed.

**`internal/daemon/daemon.go`** — `claimTask`, `reportResult`, `sendHeartbeat` all send `Authorization: Bearer <daemonToken>` header when `cfg.Auth.DaemonToken != ""`. Backward compatible (no header when token is empty).

**`cmd/flowy/main.go`** — New subcommands: `login`, `logout`, `whoami`, `registry`. Existing subcommands gain optional `--project` flag for project-scoped submission. Token file read/write via `~/.flowy-token` or `FLOWY_TOKEN` env var.

---

## Data Flow: Registry Execution with Provenance

```
[Client]
  flowy run --project proj1 --registry samtools-sort:v1.2 --i inputs.json
      ↓
POST /projects/proj1/workflows
  Body: {registry_ref: "samtools-sort:v1.2", inputs: {...}}
  Authorization: Bearer <user JWT>

[Server — handleProjectSubmit]
  1. JWTMiddleware validates token → extracts userID
  2. RequireProjectRole("executor") checks project_members
  3. GetRegistryTag(projectID, "samtools-sort", "v1.2") → registry_entry.content_hash
  4. GetRegistryEntry(content_hash) → entry.wdl_source
  5. ParseFromEntry(entry) → *ast.Document  (in-memory, no file path)
  6. CreateRun with project_id, registry_ref, wdl_path = "<registry>samtools-sort@sha256:abc123"
  7. Launch runWorkflow goroutine

[Server — runWorkflow goroutine]
  Executes as before: makeDispatchFn → daemon claims tasks
  WDLContent delivered to daemon via TaskClaimResponse.WDLContent (existing mechanism)
  On completion:
    UpdateRunOutputs
    CreateProvenanceRecord{run_id, project_id, registry_entry_hash, ...}
    UpdateRunState(COMPLETED)

[Client — polls /projects/proj1/runs/{id}/status]
  Receives COMPLETED
  GET /projects/proj1/runs/{id}/outputs → returns outputs
  GET /projects/proj1/provenance?run_id={id} → returns provenance record
```

---

## Scaling Considerations

| Concern | At v1.3 Scale | Future Consideration |
|---------|---------------|----------------------|
| JWT validation | Stateless; O(1) per request | If token revocation needed, add token blocklist table |
| SQLite with auth tables | 5 new tables; WAL mode handles concurrent reads | If users > 10k or concurrent writes spike, consider PostgreSQL |
| Registry WDL storage | `wdl_source TEXT` in SQLite; fine for hundreds of tools | For thousands of large WDL files, store in S3 and keep only hash in DB |
| Provenance records | Append-only; indexed by run_id, project_id, content_hash | Query performance fine at thousands of records; add pagination for lists |
| Daemon count | Unchanged; daemon auth is a simple header check | No limit increase implied |

---

## Anti-Patterns

### Anti-Pattern 1: User Identity in the Daemon

**What people do:** Give the daemon a user account and have it submit runs as a regular user.

**Why it's wrong:** Daemons are infrastructure, not users. Tying daemon lifecycle to user credentials introduces token expiry problems during long-running scatter jobs.

**Do this instead:** Daemon uses a long-lived shared secret (`daemon_token`) configured in YAML. It is validated by a separate `DaemonAuthMiddleware` that does not go through the RBAC project membership check.

### Anti-Pattern 2: Storing WDL in the Storage Backend (S3) Instead of SQLite

**What people do:** Store registry WDL files as S3 objects, keep only the S3 URI in SQLite.

**Why it's wrong:** WDL source files are small (typically < 50 KB). Fetching them from S3 for every execution adds unnecessary latency and a network dependency that prevents the registry from working in local-only deployments.

**Do this instead:** Store `wdl_source TEXT` directly in `registry_entries`. SQLite TEXT columns handle large strings fine. If file sizes grow beyond reason, revisit in a future milestone.

### Anti-Pattern 3: Embedding RBAC Logic in Handlers

**What people do:** Sprinkle `GetProjectRole(...)` checks inside each handler function.

**Why it's wrong:** Duplicates authorization logic, easy to forget in new handlers, untestable in isolation.

**Do this instead:** Use chi middleware (`RequireProjectRole`) applied to route groups. Authorization is declared at the router level and is automatically applied to all routes in the group. Handlers assume the caller is authorized.

### Anti-Pattern 4: Using a New JWT Library Without Existing Dependency Audit

**What people do:** Add `github.com/dgrijalva/jwt-go` (archived) or a lesser-known library.

**Why it's wrong:** `dgrijalva/jwt-go` was archived due to CVE-2020-26160 and is not maintained. Lesser-known libraries have smaller security review surface.

**Do this instead:** Use `github.com/golang-jwt/jwt/v5` — the official community-maintained successor, MIT license, actively maintained as of 2026, standard HMAC-SHA256 support.

### Anti-Pattern 5: Requiring `project_id` for All Existing Runs

**What people do:** Make `project_id` NOT NULL on the `runs` table and require it from all callers.

**Why it's wrong:** Existing spectool tests and backward-compatible clients submit without `project_id`. Making it required breaks all existing callers.

**Do this instead:** `project_id` is nullable on `runs`. Existing routes work without project context. Project-scoped routes set it. Provenance is only written when both `project_id` and `registry_ref` are present.

---

## Build Order

Dependencies between the new components drive this order:

```
Step 1  internal/auth/ — jwt.go, password.go
        No internal deps. Pure stdlib + golang-jwt/jwt/v5.
        Unblocks: all middleware and handler work.

Step 2  internal/store/store.go — add new model types (User, Project, etc.)
        internal/store/sqlite/migrations/00004_users.sql + users.go
        Unblocks: auth handlers (need CreateUser, GetUserByUsername).

Step 3  internal/auth/middleware.go — JWTMiddleware, DaemonAuthMiddleware
        internal/config/config.go — AuthConfig
        internal/server/server.go — add jwtSecret, restructure newRouter()
        Gate: existing spectool tests still pass (auth.enabled=false by default).

Step 4  internal/store/sqlite/migrations/00005_projects.sql + projects.go
        internal/server/handlers_auth.go — login, logout, whoami, createUser
        internal/server/handlers_project.go — project CRUD, membership
        cmd/flowy/main.go — login, logout, whoami subcommands
        Gate: auth + project management works end-to-end.

Step 5  internal/store/sqlite/migrations/00006_registry.sql + registry.go
        internal/registry/ — hash.go, store.go, parse.go
        internal/server/handlers_registry.go — push, pull, list
        cmd/flowy/main.go — registry subcommands
        Gate: push/pull WDL, tag resolution, execute from registry.

Step 6  internal/store/sqlite/migrations/00007_runs_project.sql
        internal/store/sqlite/migrations/00008_provenance.sql + provenance.go
        internal/provenance/ — record.go, write.go
        internal/server/dispatch.go — provenance write after completion
        internal/server/handlers_provenance.go — list, get
        Gate: provenance records created and queryable.

Step 7  Daemon token auth (minimal change to daemon.go + config)
        Gate: daemon continues to claim tasks after auth is enabled.
```

Steps 1–3 are prerequisite infrastructure. Each step is independently testable before proceeding to the next.

---

## Sources

- Direct codebase analysis: all files in `internal/server/`, `internal/store/`, `internal/api/`, `internal/config/`, `internal/daemon/`, `internal/storage/`, `cmd/flowy/main.go` — HIGH confidence
- chi v5 middleware and route groups: https://go-chi.io/#/pages/middleware — HIGH confidence (official docs)
- `golang-jwt/jwt/v5` documentation: https://pkg.go.dev/github.com/golang-jwt/jwt/v5 — HIGH confidence
- `golang.org/x/crypto/bcrypt`: already in transitive deps (`go.sum` confirms `golang.org/x/crypto v0.48.0`) — HIGH confidence
- Docker-registry content-addressed storage pattern (SHA-256 of content as primary key): well-established; same model as OCI image manifests — HIGH confidence
- SQLite `ALTER TABLE ADD COLUMN` limitation (no DROP COLUMN): https://www.sqlite.org/lang_altertable.html — HIGH confidence

---

*Architecture research for: flowy v1.3 — user auth, project RBAC, WDL registry, provenance tracking*
*Researched: 2026-03-17*
