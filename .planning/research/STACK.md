# Stack Research: WDL Workflow Engine (Go)

**Project:** flowy — WDL 1.2 workflow engine (Go port of miniwdl)
**Researched:** 2026-03-11 (v1.1 update; v1.0 stack preserved below)
**Overall confidence:** HIGH

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

*Stack research for: flowy v1.1 distributed server/daemon/client*
*Researched: 2026-03-11*
