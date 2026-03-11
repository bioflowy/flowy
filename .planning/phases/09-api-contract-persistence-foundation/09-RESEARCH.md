# Phase 9: API Contract + Persistence Foundation - Research

**Researched:** 2026-03-11
**Domain:** Go shared packages — YAML config, SQLite/WAL/goose migrations, filesystem layout, HTTP DTO types
**Confidence:** HIGH

## Summary

Phase 9 establishes the foundational shared packages that all three binaries (flowy, flowy-server, flowy-daemon) will depend on. There are three distinct sub-problems: (1) a YAML config loader with per-binary sections, (2) a SQLite persistence layer with WAL mode and goose-managed schema, and (3) a shared `internal/api` package containing HTTP DTO types that compiles without import cycles.

The technology choices are already decided in STATE.md: `modernc.org/sqlite` (CGO-free), `github.com/pressly/goose/v3` (embedded migrations), `github.com/go-chi/chi/v5` (HTTP router — used starting Phase 10 but DTOs needed now), and `gopkg.in/yaml.v3` (already in go.sum). None of these packages are in go.mod yet except yaml.v3. This phase adds all three remaining libraries and creates the shared packages.

The critical schema decisions are locked: the `tasks` table MUST include `last_heartbeat_at` and `shard_index` from the start — retrofitting columns onto an existing SQLite table requires ALTER TABLE (which has limited support in SQLite). Stale task detection is a pure SQL query (`WHERE last_heartbeat_at < datetime('now', '-N seconds')`) and does not require application-level polling logic in Phase 9 — just the correct schema and a verified query.

**Primary recommendation:** Create three packages — `internal/config`, `internal/store`, `internal/api` — each consumed independently by server, daemon, and client. Use embedded SQL migrations via `embed.FS`. Keep all three packages free of cross-dependencies.

**Future-proofing:** `Store` is defined as a Go interface so that a PostgreSQL backend (PG-01, v2+) can be added later without changing callers. The SQLite implementation lives under `internal/store/sqlite/`. Config includes a `db_driver` field to select the backend at runtime. SQL queries stay inside the implementation package, allowing dialect-specific SQL without leaking through the interface.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| STR-01 | Config file (YAML) specifies shared filesystem base directory | yaml.v3 struct unmarshal; nested struct with `yaml` tags |
| STR-02 | Config file has sections for server, daemon, and client settings | Single top-level Config struct with Server/Daemon/Client sub-structs |
| STR-03 | Runs organized under `{base_dir}/runs/{run_id}/inputs/` and `outputs/` | `os.MkdirAll` with path joining at runtime; path helpers in internal/store |
| STR-04 | All three binaries read the same config file format | Single `internal/config` package loaded by each binary's main.go |
| SRV-07 | Server persists workflow runs and task state in SQLite with WAL mode | modernc.org/sqlite + WAL PRAGMA + separate writer pool (MaxOpenConns=1) |
| SRV-08 | Server detects stale tasks (no heartbeat within timeout) and resets them to PENDING | SQL query `WHERE last_heartbeat_at < datetime('now','-N seconds')` on tasks table |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `modernc.org/sqlite` | v1.46.1 | CGO-free SQLite driver | Zero CGO; cross-compiles to any platform; already decided in STATE.md |
| `github.com/pressly/goose/v3` | v3.27.0 | SQL schema migrations | Embed.FS support; startup `Up()` call; "sqlite3" dialect for SQLite |
| `gopkg.in/yaml.v3` | v3.0.1 | YAML config parsing | Already in go.sum; standard yaml library for Go |
| `github.com/go-chi/chi/v5` | v5.2.3+ | HTTP routing (DTOs defined now, router used in Phase 10) | Decided in STATE.md; only the DTO types are in scope for Phase 9 |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `database/sql` (stdlib) | — | DB pool management | Always; modernc.org/sqlite registers as standard driver |
| `embed` (stdlib) | — | Embed SQL migration files into binary | Use with `//go:embed migrations/*.sql` directive |
| `os` (stdlib) | — | Filesystem directory creation | Use in run directory setup helpers |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| modernc.org/sqlite | mattn/go-sqlite3 | mattn requires CGO; prevents cross-compilation; rejected in STATE.md |
| goose/v3 | golang-migrate/migrate | goose is simpler for startup auto-migration; embed.FS first-class |
| yaml.v3 | viper | viper is heavyweight; yaml.v3 already present in go.sum |

**Installation:**
```bash
go get modernc.org/sqlite@v1.46.1
go get github.com/pressly/goose/v3@v3.27.0
go get github.com/go-chi/chi/v5@latest
```

Note: `gopkg.in/yaml.v3` is already in go.sum and available.

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── config/           # YAML config loader (all three binaries import this)
│   └── config.go
├── store/            # Store interface + model types (no DB driver imports)
│   ├── store.go      # Store interface definition + model structs (Run, Task)
│   ├── sqlite/       # SQLite implementation of Store interface
│   │   ├── sqlite.go # Open(), Close(), migrations, WAL pragma setup
│   │   ├── runs.go   # Run CRUD (CreateRun, GetRun, UpdateRunState)
│   │   ├── tasks.go  # Task CRUD (CreateTask, ClaimTask, UpdateHeartbeat, FindStaleTasks)
│   │   └── migrations/
│   │       └── 00001_init.sql
│   └── postgres/     # (v2+ PG-01) PostgreSQL implementation — not in Phase 9 scope
└── api/              # HTTP DTO types only (no import of store or config)
    └── types.go      # SubmitRequest, StatusResponse, OutputsResponse

cmd/
├── flowy/            # existing binary (will add config loading in Phase 12)
├── flowy-server/     # Phase 10
└── flowy-daemon/     # Phase 11
```

### Pattern 1: YAML Config with Per-Binary Sections

**What:** Single struct with nested sub-structs, one per binary. All three binaries unmarshal the same file; each reads only its section.

**When to use:** Any shared config file consumed by multiple binaries.

**Example:**
```go
// internal/config/config.go
// Source: gopkg.in/yaml.v3 official docs

package config

import (
    "os"
    "gopkg.in/yaml.v3"
)

type Config struct {
    BaseDir  string       `yaml:"base_dir"`
    DBDriver string       `yaml:"db_driver"` // "sqlite" (default) or "postgres" (v2+)
    DBDSN    string       `yaml:"db_dsn"`    // driver-specific connection string
    Server   ServerConfig `yaml:"server"`
    Daemon   DaemonConfig `yaml:"daemon"`
    Client   ClientConfig `yaml:"client"`
}

type ServerConfig struct {
    ListenAddr          string `yaml:"listen_addr"`
    HeartbeatTimeoutSec int    `yaml:"heartbeat_timeout_sec"`
}

type DaemonConfig struct {
    ServerURL        string `yaml:"server_url"`
    HeartbeatSec     int    `yaml:"heartbeat_sec"`
    PollTimeoutSec   int    `yaml:"poll_timeout_sec"`
}

type ClientConfig struct {
    ServerURL string `yaml:"server_url"`
}

func Load(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    var cfg Config
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}
```

### Pattern 2: Store Interface + SQLite Implementation

**What:** `internal/store` defines a `Store` interface and model types (Run, Task). `internal/store/sqlite` provides the SQLite implementation with WAL mode, separate writer/reader pools, and goose migrations. This allows adding a PostgreSQL backend (v2+ PG-01) without changing any caller code.

**When to use:** Always. All callers depend on the interface, never on the concrete implementation.

**Example — Interface (internal/store/store.go):**
```go
package store

import "context"

type Run struct {
    ID         string
    WDLPath    string
    InputsJSON string
    State      string
    CreatedAt  string
    UpdatedAt  string
}

type Task struct {
    ID              string
    RunID           string
    TaskName        string
    State           string
    ShardIndex      int
    DaemonID        string
    LastHeartbeatAt string
    CreatedAt       string
    UpdatedAt       string
}

type Store interface {
    // Run operations
    CreateRun(ctx context.Context, r *Run) error
    GetRun(ctx context.Context, id string) (*Run, error)
    UpdateRunState(ctx context.Context, id, state string) error

    // Task operations
    CreateTask(ctx context.Context, t *Task) error
    ClaimTask(ctx context.Context, taskID, daemonID string) error
    UpdateHeartbeat(ctx context.Context, taskID string) error
    FindStaleTasks(ctx context.Context, timeoutSec int) ([]Task, error)

    // Lifecycle
    Close() error
}
```

**Example — SQLite implementation (internal/store/sqlite/sqlite.go):**
```go
package sqlite

import (
    "database/sql"
    "embed"
    "fmt"

    "github.com/pressly/goose/v3"
    _ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type SQLiteStore struct {
    writer *sql.DB
    reader *sql.DB
}

func Open(dsn string) (*SQLiteStore, error) {
    // dsn example: "file:./flowy.db?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"

    writer, err := sql.Open("sqlite", dsn)
    if err != nil {
        return nil, fmt.Errorf("open sqlite writer: %w", err)
    }
    writer.SetMaxOpenConns(1) // SQLite: single writer enforced

    reader, err := sql.Open("sqlite", dsn)
    if err != nil {
        return nil, fmt.Errorf("open sqlite reader: %w", err)
    }

    // Run goose migrations on writer connection.
    goose.SetBaseFS(embedMigrations)
    if err := goose.SetDialect("sqlite3"); err != nil {
        return nil, fmt.Errorf("goose dialect: %w", err)
    }
    if err := goose.Up(writer, "migrations"); err != nil {
        return nil, fmt.Errorf("goose up: %w", err)
    }

    return &SQLiteStore{writer: writer, reader: reader}, nil
}

func (s *SQLiteStore) Close() error {
    s.writer.Close()
    return s.reader.Close()
}

// ... Run and Task methods implement store.Store interface
```

**Example — Factory function wired from config (used in server main.go):**
```go
// Phase 10: cmd/flowy-server/main.go
func openStore(cfg *config.Config) (store.Store, error) {
    switch cfg.DBDriver {
    case "sqlite", "":
        return sqlite.Open(cfg.DBDSN)
    // case "postgres":  // v2+ PG-01
    //     return postgres.Open(cfg.DBDSN)
    default:
        return nil, fmt.Errorf("unsupported db_driver: %s", cfg.DBDriver)
    }
}
```

### Pattern 3: Goose SQL Migration File Format

**What:** Sequential SQL files in `internal/store/migrations/`. Goose reads `-- +goose Up` and `-- +goose Down` markers.

**When to use:** Every schema change as its own numbered file.

**Example:**
```sql
-- internal/store/migrations/00001_init.sql
-- +goose Up
CREATE TABLE IF NOT EXISTS runs (
    id          TEXT PRIMARY KEY,
    wdl_path    TEXT NOT NULL,
    inputs_json TEXT NOT NULL,
    state       TEXT NOT NULL DEFAULT 'PENDING',
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS tasks (
    id                TEXT PRIMARY KEY,
    run_id            TEXT NOT NULL REFERENCES runs(id),
    task_name         TEXT NOT NULL,
    state             TEXT NOT NULL DEFAULT 'PENDING',
    shard_index       INTEGER NOT NULL DEFAULT 0,
    daemon_id         TEXT,
    last_heartbeat_at DATETIME,
    created_at        DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at        DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_tasks_run_id ON tasks(run_id);
CREATE INDEX IF NOT EXISTS idx_tasks_state  ON tasks(state);

-- +goose Down
DROP TABLE IF EXISTS tasks;
DROP TABLE IF EXISTS runs;
```

### Pattern 4: Stale Task Detection Query

**What:** Pure SQL query against `last_heartbeat_at`. No application-level polling in Phase 9 — just the correct schema and a verified integration test.

**When to use:** Whenever the server reset loop runs (Phase 10). Phase 9 only needs to verify the query works.

**Example:**
```go
// internal/store/sqlite/tasks.go

// FindStaleTasks returns tasks in RUNNING state whose heartbeat has not been
// updated within timeoutSec seconds. SQLite-specific datetime() syntax.
func (s *SQLiteStore) FindStaleTasks(ctx context.Context, timeoutSec int) ([]Task, error) {
    const q = `
        SELECT id, run_id, task_name, state, shard_index, daemon_id, last_heartbeat_at
        FROM tasks
        WHERE state = 'RUNNING'
          AND last_heartbeat_at < datetime('now', printf('-%d seconds', ?))
    `
    rows, err := s.reader.QueryContext(ctx, q, timeoutSec)
    // ... scan rows
}
```

**Simpler verified form:**
```sql
SELECT id FROM tasks
WHERE state = 'RUNNING'
  AND last_heartbeat_at < datetime('now', '-' || ? || ' seconds')
```

Note: SQLite `datetime('now', '-N seconds')` requires the modifier to be a string. Use string concatenation or `printf` when binding parameters.

### Pattern 5: HTTP DTO Types in internal/api (No Import Cycles)

**What:** Plain Go structs with JSON tags only. No imports from `store` or `config`. Compile-only check.

**When to use:** Shared request/response shapes consumed by server handler and client HTTP calls.

**Example:**
```go
// internal/api/types.go

package api

// SubmitRequest is the payload for POST /workflows.
type SubmitRequest struct {
    WDLPath    string                 `json:"wdl_path"`
    InputsJSON map[string]interface{} `json:"inputs"`
}

// SubmitResponse is returned by POST /workflows.
type SubmitResponse struct {
    RunID string `json:"run_id"`
}

// StatusResponse is returned by GET /workflows/{id}/status.
type StatusResponse struct {
    RunID  string `json:"run_id"`
    State  string `json:"state"`
}

// OutputsResponse is returned by GET /workflows/{id}/outputs.
type OutputsResponse struct {
    RunID   string                 `json:"run_id"`
    Outputs map[string]interface{} `json:"outputs"`
}
```

### Anti-Patterns to Avoid

- **Single sql.DB for both reads and writes:** Under WAL mode, a single pool can still produce `SQLITE_BUSY` when multiple goroutines contend. Use separate writer (MaxOpenConns=1) and reader pools.
- **Pragmas in SQL migration files:** Pragma settings are per-connection; they must be set in the DSN or via a connection hook, not in `.sql` migration files (they run once on a single connection).
- **Calling `goose.SetDialect("sqlite")` instead of `"sqlite3"`:** The goose library recognizes `"sqlite3"` for SQLite databases. Using `"sqlite"` will return an error.
- **`internal/api` importing `internal/store`:** This creates an import cycle if store eventually needs API types. Keep `internal/api` as a leaf package with only `encoding/json`-compatible types.
- **`datetime('now', '-5 seconds')` with integer parameter binding:** SQLite requires the modifier to be a string, not an integer. When using `?` placeholders, construct the string: `'-' || ? || ' seconds'` or use `printf`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| SQL schema versioning | Custom version table + IF NOT EXISTS guards | goose/v3 with embed.FS | Goose handles ordering, idempotency, down migrations, locking |
| SQLite WAL pragma application | Init function that runs PRAGMA manually per connection | DSN `_pragma=` query parameters | DSN pragmas apply to every connection in the pool automatically |
| YAML config validation | Custom field presence checks | yaml.v3 struct decode + Go zero-value checks | yaml.v3 returns errors for type mismatches; zero-value detection is idiomatic |
| Run directory creation | Recursive mkdir in each binary | Helper function in `internal/store` or `internal/config` | Single tested function prevents path convention drift across binaries |
| DB-specific SQL in callers | Switch statements on driver type in handler code | Store interface with per-driver implementations | Dialect-specific SQL stays inside `internal/store/sqlite/` (or `postgres/`); callers only see the interface |

**Key insight:** SQLite's WAL mode handles most concurrency for this workload. The main risk is programmer error in PRAGMA setup — use DSN parameters to enforce them at open time. The Store interface ensures dialect-specific concerns never leak into caller code, making a future PostgreSQL backend a pure addition (no modifications to existing callers).

## Common Pitfalls

### Pitfall 1: goose dialect string mismatch
**What goes wrong:** `goose.SetDialect("sqlite")` returns `"unknown dialect: sqlite"`. Migrations never run. Server panics at startup.
**Why it happens:** modernc.org/sqlite registers the driver as `"sqlite"`, but goose's dialect name for SQLite is `"sqlite3"`. These are independent naming namespaces.
**How to avoid:** Always call `goose.SetDialect("sqlite3")` regardless of driver name.
**Warning signs:** `goose.Up()` returns an error immediately without touching the database.

### Pitfall 2: SQLite SQLITE_BUSY under concurrent writes
**What goes wrong:** Server and daemon both try to UPDATE a task row simultaneously; one gets `database is locked`.
**Why it happens:** Without `busy_timeout`, SQLite returns SQLITE_BUSY immediately instead of waiting.
**How to avoid:** Set `_pragma=busy_timeout(5000)` in DSN (5 second wait before error). Also limit writer pool to `MaxOpenConns(1)`.
**Warning signs:** Intermittent `SQLITE_BUSY` or `database is locked` errors under load.

### Pitfall 3: datetime() stale-task query with integer binding
**What goes wrong:** `datetime('now', ?)` with `?=300` (integer) returns NULL because SQLite expects the modifier to be a string like `"-300 seconds"`.
**Why it happens:** SQLite's `datetime()` modifier must be a string. Binding an integer directly does not coerce to the expected format.
**How to avoid:** Build the string in Go before binding: `fmt.Sprintf("-%d seconds", timeoutSec)`, or use SQLite's `printf` function inline.
**Warning signs:** `FindStaleTasks` returns empty results even when heartbeats are clearly stale.

### Pitfall 4: embed.FS path must match //go:embed directive
**What goes wrong:** `goose.Up(db, "migrations")` returns `"no migration files found"` even though files exist.
**Why it happens:** The directory passed to `goose.Up` must match the prefix in the `//go:embed` tag. If the embed directive is `//go:embed migrations/*.sql`, pass `"migrations"` to `Up`, not `"."` or a full path.
**How to avoid:** Always pass the directory name exactly as it appears in the embed glob prefix.
**Warning signs:** goose logs zero migration files discovered.

### Pitfall 5: internal/api importing internal/store
**What goes wrong:** Import cycle: `store` needs `api.StatusResponse` for HTTP helpers; `api` needs `store.Run` for conversion. Compiler refuses to build.
**Why it happens:** Bidirectional dependency between packages is disallowed in Go.
**How to avoid:** `internal/api` must be a pure leaf package — only stdlib imports (`encoding/json` types if needed). Any conversion between store models and API DTOs lives in a third package (server handler layer in Phase 10).
**Warning signs:** `import cycle not allowed` compiler error.

## Code Examples

### Opening the DB with pragmas via DSN (inside internal/store/sqlite/)
```go
// Source: modernc.org/sqlite v1.46.1 docs (pkg.go.dev/modernc.org/sqlite)
import _ "modernc.org/sqlite"

dsn := "file:/var/lib/flowy/flowy.db?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
db, err := sql.Open("sqlite", dsn)
db.SetMaxOpenConns(1) // writer: serialized
```

### Running embedded goose migrations at startup
```go
// Source: pressly/goose v3.27.0 docs (pkg.go.dev/github.com/pressly/goose/v3)

//go:embed migrations/*.sql
var embedMigrations embed.FS

goose.SetBaseFS(embedMigrations)
if err := goose.SetDialect("sqlite3"); err != nil {
    log.Fatal(err)
}
if err := goose.Up(db, "migrations"); err != nil {
    log.Fatal(err)
}
```

### YAML config load
```go
// Source: gopkg.in/yaml.v3 (already in go.sum)
data, _ := os.ReadFile(path)
var cfg config.Config
yaml.Unmarshal(data, &cfg)
// cfg.BaseDir, cfg.Server.ListenAddr, cfg.Daemon.ServerURL all accessible
```

### Run directory creation helper
```go
// internal/store or internal/config — shared by all three binaries
func RunInputsDir(baseDir, runID string) string {
    return filepath.Join(baseDir, "runs", runID, "inputs")
}
func RunOutputsDir(baseDir, runID string) string {
    return filepath.Join(baseDir, "runs", runID, "outputs")
}
func CreateRunDirs(baseDir, runID string) error {
    if err := os.MkdirAll(RunInputsDir(baseDir, runID), 0755); err != nil {
        return err
    }
    return os.MkdirAll(RunOutputsDir(baseDir, runID), 0755)
}
```

### Stale task detection (integration-test-verified pattern)
```go
// internal/store/sqlite/tasks.go
// Use string concatenation to build the SQLite modifier correctly.
// PostgreSQL equivalent would use: NOW() - INTERVAL '? seconds'
modifier := fmt.Sprintf("-%d seconds", timeoutSec)
rows, err := db.QueryContext(ctx,
    `SELECT id FROM tasks WHERE state = 'RUNNING' AND last_heartbeat_at < datetime('now', ?)`,
    modifier,
)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| mattn/go-sqlite3 (CGO) | modernc.org/sqlite (pure Go) | ~2021, mainstream by 2023 | Cross-compilation without C toolchain |
| Manual schema versioning | goose/v3 with embed.FS | embed.FS added Go 1.16 | Migrations bundled in binary; no external files needed |
| `goose.SetDialect("sqlite")` | `goose.SetDialect("sqlite3")` | goose v3 consistent naming | The string "sqlite3" is always correct for SQLite regardless of driver |

**Deprecated/outdated:**
- `mattn/go-sqlite3`: Requires CGO. Rejected for this project (cross-compilation requirement).
- Bare `PRAGMA journal_mode=WAL` SQL in Go code without DSN params: Works but applies only to the current connection; DSN params are applied to all pool connections automatically.

## Open Questions

1. **goose locking under concurrent startup**
   - What we know: `goose.Up` is safe to call from a single process; it uses an advisory lock in the goose_db_version table.
   - What's unclear: If server and daemon both open the same DB and call `goose.Up` simultaneously (unlikely in practice — daemon does not open the server's DB directly), there could be a lock contention window.
   - Recommendation: Only server calls `goose.Up`; daemon and client open DB read-only or not at all in Phase 9. This is fine since daemon talks to server over HTTP in v1.1.

2. **Config file path convention**
   - What we know: All three binaries need to load the same config file.
   - What's unclear: Whether to use a flag (`--config`), environment variable (`FLOWY_CONFIG`), or fixed default path (`/etc/flowy/config.yaml`, `~/.config/flowy/config.yaml`).
   - Recommendation: Support `--config` flag with default `./flowy.yaml` for dev convenience. Phase 9 defines the loader; each binary's main.go adds the flag in its own phase.

3. **PostgreSQL backend (PG-01, v2+) — design decision made**
   - Decision: `Store` is a Go interface. SQLite implementation is in `internal/store/sqlite/`. PostgreSQL implementation will be added as `internal/store/postgres/` when PG-01 is in scope.
   - Config uses `db_driver` ("sqlite" default) and `db_dsn` fields. Factory function in server main.go switches on `db_driver`.
   - Phase 9 scope: SQLite implementation only. The interface and config fields are the only PostgreSQL-related work.
   - When PG-01 arrives: Add `internal/store/postgres/` package, goose migrations with `"postgres"` dialect, `pgx` driver, and a `case "postgres"` in the factory. No changes to existing callers.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go standard `testing` package (go test) |
| Config file | none — standard Go test tooling, no config file |
| Quick run command | `go test ./internal/... -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| STR-01 | Config file with base_dir loads correctly | unit | `go test ./internal/config/... -run TestLoad -count=1` | Wave 0 |
| STR-02 | Config has server, daemon, client sections all present | unit | `go test ./internal/config/... -run TestSections -count=1` | Wave 0 |
| STR-03 | Run directories created at correct paths | unit | `go test ./internal/store/... -run TestRunDirs -count=1` | Wave 0 |
| STR-04 | All three binaries parse same config format | unit | `go test ./internal/config/... -count=1` (single shared loader) | Wave 0 |
| SRV-07 | SQLite opens in WAL mode, migrations run, tables exist | integration | `go test ./internal/store/sqlite/... -run TestOpen -count=1` | Wave 0 |
| SRV-07 | runs + tasks tables have correct schema including heartbeat/shard cols | integration | `go test ./internal/store/sqlite/... -run TestSchema -count=1` | Wave 0 |
| SRV-08 | Stale task detection query returns expected stale rows | integration | `go test ./internal/store/sqlite/... -run TestFindStaleTasks -count=1` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/... -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/config/config_test.go` — covers STR-01, STR-02, STR-04
- [ ] `internal/store/sqlite/sqlite_test.go` — covers SRV-07, SRV-08, STR-03
- [ ] `internal/api/types_test.go` — compile check for DTO types (no import cycles)
- [ ] Framework install: `go get modernc.org/sqlite@v1.46.1 github.com/pressly/goose/v3@v3.27.0 github.com/go-chi/chi/v5@latest` — must precede any store tests

## Sources

### Primary (HIGH confidence)
- `pkg.go.dev/modernc.org/sqlite` v1.46.1 — driver name "sqlite", DSN pragma syntax, WAL mode
- `pkg.go.dev/github.com/pressly/goose/v3` v3.27.0 — SetBaseFS, SetDialect("sqlite3"), Up() API
- `pkg.go.dev/gopkg.in/yaml.v3` — struct tags, Unmarshal API
- STATE.md project decisions — technology choices already locked (modernc.org/sqlite, goose, chi v5)

### Secondary (MEDIUM confidence)
- Jacob Gold "Go + SQLite Best Practices" — separate reader/writer pool pattern, busy_timeout 10s, WAL pragmas
- pressly/goose README — embed.FS migration pattern, dialect string confirmed as "sqlite3"
- WebSearch cross-verification: single writer MaxOpenConns(1) pattern confirmed by multiple sources

### Tertiary (LOW confidence)
- SQLite datetime() string modifier behavior — verified by SQLite official PRAGMA docs; integration test will confirm the binding pattern

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries verified via pkg.go.dev with exact versions
- Architecture: HIGH — patterns are standard Go; locked decisions from STATE.md confirmed
- Pitfalls: HIGH for goose dialect / SQLITE_BUSY (multiple sources); MEDIUM for datetime() binding (needs integration test confirmation)

**Research date:** 2026-03-11
**Valid until:** 2026-09-11 (stable libraries; goose and modernc.org/sqlite update frequently but APIs are stable)
