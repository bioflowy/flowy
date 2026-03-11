# Architecture Research

**Domain:** Distributed WDL Workflow Engine — server/daemon/client integration with existing Go monolith
**Researched:** 2026-03-11
**Confidence:** HIGH (based on direct codebase analysis + GA4GH TES patterns + Go ecosystem)

---

## Standard Architecture

### System Overview

```
  ┌──────────────────────────────────────────────────────────────────┐
  │                        CLIENT LAYER                               │
  │  ┌────────────────────────────────────────────────────────────┐  │
  │  │  cmd/flowy  (CLI binary)                                    │  │
  │  │  submit | status | outputs | run (sync mode)                │  │
  │  └──────────────────────────┬─────────────────────────────────┘  │
  └─────────────────────────────│────────────────────────────────────┘
                                │ HTTP REST (JSON)
  ┌─────────────────────────────▼────────────────────────────────────┐
  │                        SERVER LAYER                               │
  │  ┌────────────────────────────────────────────────────────────┐  │
  │  │  cmd/flowy-server                                           │  │
  │  │  ┌──────────────────┐  ┌──────────────────────────────┐   │  │
  │  │  │  HTTP handler    │  │  Workflow orchestrator        │   │  │
  │  │  │  (net/http)      │  │  (uses wdl/workflow directly) │   │  │
  │  │  └──────────────────┘  └──────────────────────────────┘   │  │
  │  │  ┌──────────────────────────────────────────────────────┐  │  │
  │  │  │  internal/store  (SQLite via database/sql + CGo)     │  │  │
  │  │  └──────────────────────────────────────────────────────┘  │  │
  │  └────────────────────────────────────────────────────────────┘  │
  └──────────────────────────────────────────────────────────────────┘
           │ shared filesystem           │ HTTP long-poll
           │ (NFS / local mount)         │
  ┌────────▼────────────────────────────▼────────────────────────────┐
  │                        DAEMON LAYER                               │
  │  ┌────────────────────────────────────────────────────────────┐  │
  │  │  cmd/flowy-daemon  (one process per worker host)           │  │
  │  │  ┌──────────────────┐  ┌──────────────────────────────┐   │  │
  │  │  │  Poll loop       │  │  Task executor                │   │  │
  │  │  │  GET /tasks/next │  │  wraps wdl/task.RunTask()     │   │  │
  │  │  └──────────────────┘  └──────────────────────────────┘   │  │
  │  └────────────────────────────────────────────────────────────┘  │
  │  [ daemon-A ]      [ daemon-B ]      [ daemon-C ]                 │
  └──────────────────────────────────────────────────────────────────┘
           │ reads inputs/  │ writes outputs/
           │ shared storage │
  ┌────────▼────────────────▼────────────────────────────────────────┐
  │                       STORAGE LAYER                               │
  │  shared_root/                                                     │
  │    runs/<run_id>/                                                  │
  │      inputs/          (client copies WDL inputs here)             │
  │      outputs/         (daemon writes collected outputs here)       │
  │      working/         (task-level MkdirTemp, same as execRoot)    │
  │      outputs.json     (final workflow outputs)                    │
  │      error.json       (failure details if any)                    │
  └──────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Existing code it uses |
|-----------|---------------|----------------------|
| `cmd/flowy` (client) | CLI UX: submit, poll, display results | parser, checker (for `flowy check`); HTTP client for all run ops |
| `cmd/flowy-server` | Accept submissions, orchestrate DAG, assign tasks to daemons | `wdl/workflow` (DAG build + topo sort), `wdl/parser`, `wdl/checker`, new `internal/store` |
| `cmd/flowy-daemon` | Poll server for assigned tasks, execute them, report results | `wdl/task.RunTask` directly (unchanged interface) |
| `internal/store` | SQLite persistence for runs and tasks | `database/sql` + `mattn/go-sqlite3` |
| `internal/api` | Shared HTTP request/response types (DTOs) | Used by both client and server to avoid type drift |
| shared filesystem | File exchange between client and daemon | OS filesystem; no new code needed |

---

## Recommended Project Structure

```
flowy/
├── cmd/
│   ├── flowy/           # existing client binary (extend with submit/status/outputs)
│   │   └── main.go
│   ├── flowy-server/    # NEW: server binary
│   │   └── main.go
│   └── flowy-daemon/    # NEW: daemon binary
│       └── main.go
├── internal/
│   ├── api/             # NEW: shared HTTP DTO types (no business logic)
│   │   ├── types.go     # RunSubmitRequest, RunStatusResponse, TaskAssignment, etc.
│   │   └── errors.go    # shared error response shape
│   ├── store/           # NEW: SQLite persistence layer
│   │   ├── store.go     # Store interface + sqlite implementation
│   │   ├── schema.sql   # embedded schema (go:embed)
│   │   └── store_test.go
│   ├── server/          # NEW: HTTP handler logic (separate from cmd for testability)
│   │   ├── handler.go   # net/http handlers for all endpoints
│   │   ├── orchestrator.go  # wraps wdl/workflow, dispatches tasks via store
│   │   └── handler_test.go
│   └── config/          # NEW: config file parsing
│       └── config.go    # flowyconfig.toml / flowyconfig.yaml parsing
├── wdl/                 # UNCHANGED: existing engine packages
│   ├── ast/
│   ├── lexer/
│   ├── parser/
│   ├── checker/
│   ├── eval/
│   ├── task/            # task.RunTask and task.ContainerRunner are the daemon integration point
│   ├── types/
│   ├── values/
│   └── workflow/        # workflow.RunWorkflow is the server orchestration point
└── go.mod
```

### Structure Rationale

- **`internal/api/`:** Shared DTO types live here so the client binary and server binary use identical wire formats. Prevents type drift between producer and consumer.
- **`internal/store/`:** Pure persistence, no HTTP or business logic. Allows the orchestrator to be tested without a running HTTP server.
- **`internal/server/`:** Handlers separated from `cmd/flowy-server/main.go` — enables `httptest.NewServer` in tests without spawning a real process.
- **`wdl/` untouched:** All existing packages stay unchanged. The distributed layer wraps them — it does not modify them.
- **`cmd/flowy-daemon/` imports `wdl/task`:** The daemon calls `task.RunTask()` exactly as the monolith did. No new interface needed.

---

## Architectural Patterns

### Pattern 1: Server-Side DAG Orchestration (Modified RunWorkflow)

**What:** The server runs the DAG scheduler from `wdl/workflow` but replaces the `runTaskFn` variable with a function that persists task work to SQLite and blocks until a daemon picks it up and reports completion. The existing in-process goroutine model becomes a database-mediated async handoff.

**When to use:** Core pattern for server-side workflow orchestration.

**Trade-offs:** The scheduler goroutine in `wdl/workflow.scheduler.run()` blocks on a channel waiting for task completions. In the distributed model the channel message arrives via HTTP callback from the daemon rather than from an in-process goroutine. The scheduler loop itself doesn't change — only the `runTaskFn` changes.

**How the hook works in the existing code:**

```go
// wdl/workflow/scheduler.go — already has this variable
var runTaskFn = task.RunTask  // package-level, overridable

// server/orchestrator.go — override at server startup
workflow.SetRunTaskFn(func(ctx context.Context, t *ast.Task, inputs map[string]interface{},
    runner task.ContainerRunner, execRoot string,
    structDefs ...map[string]*ast.StructDef) (map[string]values.Value, error) {

    taskID := store.InsertTask(runID, t, inputs, execRoot)
    return waitForDaemonCompletion(ctx, taskID)  // blocks until daemon reports done
})
```

The existing `runTaskFn` is a package-level `var` — this is the correct injection point. No changes to `scheduler.go` are needed; a public setter function is added.

**Alternative for scatter:** When scatter creates multiple `g.Go()` goroutines that each call `runTaskFn`, the distributed version creates multiple SQLite task rows simultaneously. Multiple daemons can pick these up in parallel naturally — scatter parallelism maps directly to daemon-level parallelism.

### Pattern 2: Daemon Long-Poll for Task Assignment

**What:** Daemons poll `GET /internal/tasks/next?daemon_id=X` with a long-poll timeout (30s). Server holds the request open until a task is available or the timeout expires. If a task is available, server immediately responds with the task assignment and marks it `ASSIGNED` in SQLite. Daemon executes, reports result via `POST /internal/tasks/{id}/complete`.

**When to use:** Preferred over interval polling (simpler server state, no thundering herd at small scale) and push/websocket (no persistent connection management needed).

**Trade-offs:**
- Long-poll requires the server to manage open HTTP connections per daemon (one goroutine per waiting daemon). At the scale this project targets (dozens of daemons), this is fine.
- If the daemon crashes mid-task, the task row stays in `ASSIGNED` state. A reaper goroutine (runs every 60s) claims any task that has been `ASSIGNED` for longer than a configurable timeout and resets it to `QUEUED`.
- Short interval polling (5s) is the fallback: simpler to implement, acceptable latency for bioinformatics tasks that take minutes.

**Protocol:**

```
Daemon                                    Server
  |                                          |
  |--- GET /internal/tasks/next (hold 30s) ->|
  |                                          | (waits, no task available)
  |                       (new task arrives) |
  |<-- 200 TaskAssignment{id, ast_json, ... }|
  |                                          |
  | [executes task.RunTask locally]          |
  |                                          |
  |--- POST /internal/tasks/{id}/complete -->|
  |    {status: "success", outputs_json}     |
  |                                          |
  |<-- 200 OK                                |
  |                                          |
  | (immediately re-polls)                   |
  |--- GET /internal/tasks/next (hold 30s) ->|
```

### Pattern 3: AST Serialization for Daemon Dispatch

**What:** The server needs to send a WDL `*ast.Task` and its resolved inputs to a daemon. Since the AST is a rich Go struct tree, the cleanest approach is to serialize the task definition as JSON using `encoding/json` with the existing AST types, paired with the resolved inputs map.

**When to use:** Task assignment payload.

**Trade-offs:** The `ast` package types are not currently tagged for JSON serialization. Adding `json:` struct tags to `wdl/ast/ast.go` is the minimal change needed. Alternatively, the server can send the raw WDL source + task name + inputs JSON, and the daemon re-parses locally. Re-parsing is simpler (no AST tagging needed) and more robust — the daemon is guaranteed to have the same WDL source available via the shared filesystem.

**Recommended approach: send WDL source path + task name + inputs JSON.**

```go
// internal/api/types.go
type TaskAssignment struct {
    TaskID      string          `json:"task_id"`
    RunID       string          `json:"run_id"`
    WDLPath     string          `json:"wdl_path"`     // absolute path on shared filesystem
    TaskName    string          `json:"task_name"`    // which task to run in the file
    InputsJSON  json.RawMessage `json:"inputs_json"`  // resolved inputs as JSON
    ExecRoot    string          `json:"exec_root"`    // runs/<run_id>/working/ on shared fs
}
```

The daemon re-parses `WDLPath`, extracts the named task, and calls `task.RunTask()`. This avoids all AST serialization complexity.

### Pattern 4: `flowy run` Synchronous Mode (Spectool Compatibility)

**What:** `flowy run` in synchronous mode submits the workflow to the server, then polls `GET /v1/runs/{id}` every 2s, and exits with the same JSON outputs on stdout and the same exit code as the current monolithic implementation.

**When to use:** Required for spectool conformance — the test harness calls `flowy run` and expects synchronous behavior.

**Implementation:** The client CLI either (a) detects the presence of a config file pointing to a server and uses HTTP, or (b) falls back to the existing in-process execution when no server is configured. This preserves backward compatibility without breaking spectool.

```
if config.ServerURL != "" {
    runDistributed(...)  // submit → poll → print outputs
} else {
    runLocal(...)        // existing RunWorkflow directly
}
```

---

## Data Flow

### Workflow Submission Flow

```
User: flowy run workflow.wdl --input inputs.json
    |
    | 1. Copy inputs.json + workflow.wdl to shared_root/runs/<id>/inputs/
    |
    v
POST /v1/runs  {wdl_path, inputs_path, workflow_name}
    |
    | 2. Server: parse+typecheck WDL (wdl/parser + wdl/checker)
    | 3. Server: insert run row (status=RUNNING)
    | 4. Server: launch goroutine running modified workflow.RunWorkflow
    |
    v
[Server goroutine: DAG scheduler]
    | for each ready Call node:
    | 5. insert task row (status=QUEUED, task_name, inputs_json, exec_root)
    | 6. notify waiting long-poll listeners (via channel or cond)
    | 7. block on completion channel for this task
    |
    v
[Daemon polling loop]
    | 8. GET /internal/tasks/next  → receives TaskAssignment
    | 9. daemon: re-parse WDL from WDLPath, extract task
    | 10. daemon: task.RunTask(ctx, taskDef, inputs, DockerRunner, execRoot)
    |            writes outputs to shared_root/runs/<id>/working/<task-dir>/
    | 11. daemon: POST /internal/tasks/{id}/complete  {outputs_json}
    |
    v
[Server goroutine: scheduler unblocks]
    | 12. update task row (status=COMPLETED, outputs_json)
    | 13. send outputs back to blocking runTaskFn channel
    | 14. scheduler binds outputs into workflow env, dispatches next ready nodes
    |
    v
[Workflow completes]
    | 15. server writes outputs.json to shared_root/runs/<id>/
    | 16. update run row (status=COMPLETED, outputs_json)
    |
    v
Client: GET /v1/runs/{id}  → {status: COMPLETED, outputs: {...}}
```

### State Transitions

```
Run:   SUBMITTED → RUNNING → COMPLETED | FAILED
Task:  QUEUED → ASSIGNED → COMPLETED | FAILED
```

---

## SQLite Schema

```sql
-- internal/store/schema.sql

PRAGMA journal_mode=WAL;   -- required: concurrent readers + writer
PRAGMA busy_timeout=5000;  -- retry for 5s on write contention

CREATE TABLE IF NOT EXISTS runs (
    id          TEXT PRIMARY KEY,           -- UUID v4
    status      TEXT NOT NULL DEFAULT 'SUBMITTED',
    wdl_path    TEXT NOT NULL,
    inputs_json TEXT NOT NULL DEFAULT '{}',
    outputs_json TEXT,
    error_msg   TEXT,
    created_at  DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at  DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE IF NOT EXISTS tasks (
    id           TEXT PRIMARY KEY,          -- UUID v4
    run_id       TEXT NOT NULL REFERENCES runs(id),
    task_name    TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'QUEUED',
    daemon_id    TEXT,                      -- which daemon claimed this
    inputs_json  TEXT NOT NULL DEFAULT '{}',
    outputs_json TEXT,
    error_msg    TEXT,
    exec_root    TEXT NOT NULL,
    assigned_at  DATETIME,                  -- for reaper timeout detection
    created_at   DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at   DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_run_id ON tasks(run_id);
```

**Go SQLite configuration (critical):**

```go
// internal/store/store.go
db, err := sql.Open("sqlite3", dsn+"?_journal_mode=WAL&_busy_timeout=5000")
db.SetMaxOpenConns(1)  // REQUIRED: SQLite allows only one writer; pool causes SQLITE_BUSY
```

The `SetMaxOpenConns(1)` requirement is a known Go+SQLite issue. WAL mode allows concurrent reads but serializes writes through a single connection. With `SetMaxOpenConns(1)` all operations serialize through one connection, which avoids `SQLITE_BUSY` errors without meaningful performance loss for this use case.

---

## REST API Endpoints

### Client-Facing (public)

```
POST   /v1/runs                     Submit workflow run
GET    /v1/runs                     List runs (with pagination)
GET    /v1/runs/{id}                Get run status and outputs
DELETE /v1/runs/{id}                Cancel run (best-effort)
GET    /v1/runs/{id}/outputs        Get final outputs JSON
```

### Daemon-Facing (internal, no auth initially)

```
GET    /internal/tasks/next         Long-poll for next assigned task (daemon)
POST   /internal/tasks/{id}/complete  Report task completion or failure
POST   /internal/daemons/register   (optional) daemon heartbeat / registration
```

### Design notes:

- All request/response bodies: `application/json`
- Error responses: `{"error": "message", "code": "MACHINE_CODE"}`
- Run and task IDs: UUID v4 (generated by server)
- Pagination: `GET /v1/runs?limit=50&offset=0`
- The `/internal/` prefix signals that these endpoints are for daemon use only and may have different auth or network ACL treatment later.

### Response shapes (internal/api/types.go):

```go
type SubmitRunRequest struct {
    WDLPath      string          `json:"wdl_path"`
    InputsJSON   json.RawMessage `json:"inputs_json"`
    WorkflowName string          `json:"workflow_name,omitempty"`
}

type RunStatus struct {
    ID          string          `json:"id"`
    Status      string          `json:"status"`   // SUBMITTED|RUNNING|COMPLETED|FAILED
    Outputs     json.RawMessage `json:"outputs,omitempty"`
    ErrorMsg    string          `json:"error,omitempty"`
    CreatedAt   time.Time       `json:"created_at"`
    UpdatedAt   time.Time       `json:"updated_at"`
}

type TaskAssignment struct {
    TaskID     string          `json:"task_id"`
    RunID      string          `json:"run_id"`
    WDLPath    string          `json:"wdl_path"`
    TaskName   string          `json:"task_name"`
    InputsJSON json.RawMessage `json:"inputs_json"`
    ExecRoot   string          `json:"exec_root"`
}

type TaskCompleteRequest struct {
    Status      string          `json:"status"`       // "success" | "failure"
    OutputsJSON json.RawMessage `json:"outputs_json,omitempty"`
    ErrorMsg    string          `json:"error,omitempty"`
    ExitCode    int             `json:"exit_code,omitempty"`
}
```

---

## Configuration File Structure

```toml
# flowyconfig.toml (searched in: $CWD, $HOME/.flowy/, /etc/flowy/)

[server]
listen        = "0.0.0.0:8080"   # server bind address
shared_root   = "/mnt/shared"    # shared filesystem root for runs
db_path       = "/var/lib/flowy/flowy.db"

[daemon]
server_url    = "http://flowy-server:8080"
daemon_id     = ""         # auto-generated UUID if empty (written back to config)
shared_root   = "/mnt/shared"
poll_interval = "30s"      # long-poll timeout; also fallback interval
concurrency   = 4          # max parallel tasks this daemon executes

[client]
server_url    = "http://flowy-server:8080"
shared_root   = "/mnt/shared"
```

**Config loading priority:** CLI flags > environment variables > config file > defaults.

**Env var prefix:** `FLOWY_SERVER_LISTEN`, `FLOWY_DAEMON_SERVER_URL`, `FLOWY_CLIENT_SERVER_URL` etc.

---

## Shared Filesystem Layout

```
<shared_root>/
└── runs/
    └── <run_id>/           # UUID for each workflow run
        ├── inputs/         # client copies WDL file and input files here
        │   ├── workflow.wdl
        │   ├── inputs.json
        │   └── ref.fasta   # any File inputs
        ├── working/        # daemon-side task execution root (= execRoot)
        │   ├── MyTask-a1b2c3/       # MkdirTemp result per task invocation
        │   │   ├── work/            # container /mnt/outputs bind
        │   │   ├── stdout.txt
        │   │   ├── stderr.txt
        │   │   └── rc
        │   └── MyTask-d4e5f6/       # scatter produces multiple per-item dirs
        ├── outputs/        # workflow-level outputs copied here
        │   └── result.vcf
        ├── outputs.json    # final workflow outputs (wdl/workflow writes this)
        └── error.json      # written on failure
```

**Key constraint:** The `WDLPath` in `TaskAssignment` must be a path on the shared filesystem that all daemons can read. The client copies the WDL file to `runs/<id>/inputs/workflow.wdl` before submitting. The server records this path in the run row and passes it to every task assignment.

**File input paths in inputs.json:** All `File` inputs must be absolute paths under `shared_root`. The client is responsible for copying local files to `runs/<id>/inputs/` and rewriting paths in the inputs JSON before submitting.

---

## Integration Points with Existing Packages

### What is unchanged

| Package | Status | Notes |
|---------|--------|-------|
| `wdl/ast` | Unchanged | AST node types stay as-is |
| `wdl/lexer` | Unchanged | |
| `wdl/parser` | Unchanged | Daemon calls `parser.Parse()` directly |
| `wdl/checker` | Unchanged | Server calls `checker.Check()` at submission time |
| `wdl/eval` | Unchanged | |
| `wdl/types` | Unchanged | |
| `wdl/values` | Unchanged | |
| `wdl/task` | Unchanged | Daemon calls `task.RunTask()` with same signature |

### What is minimally modified

| Package | Modification | Why |
|---------|-------------|-----|
| `wdl/workflow` | Add `SetRunTaskFn(fn)` public setter | Allows server to inject distributed task dispatch without changing internal logic |
| `cmd/flowy/main.go` | Add `submit`, `status`, `outputs` subcommands; make `run` config-aware | Client UX extension |

### New packages required

| Package | Content | Dependencies |
|---------|---------|-------------|
| `internal/api` | HTTP DTO types shared between client and server | `encoding/json`, `time` only |
| `internal/store` | SQLite persistence | `database/sql`, `mattn/go-sqlite3` (CGo) |
| `internal/server` | HTTP handlers, orchestrator | `wdl/workflow`, `wdl/parser`, `wdl/checker`, `internal/store`, `internal/api` |
| `internal/config` | Config file loading | `encoding/json` or `github.com/BurntSushi/toml` |
| `cmd/flowy-server` | Server binary entry point | `internal/server`, `internal/config` |
| `cmd/flowy-daemon` | Daemon binary entry point | `wdl/task`, `wdl/parser`, `internal/api`, `internal/config` |

---

## Build Order (dependency-respecting)

The packages form a clear dependency graph with no circular dependencies. Build order for implementation:

```
Phase 1: Foundation (no external deps)
  1. internal/api          — DTO types only, no logic
  2. internal/config        — config file parsing

Phase 2: Persistence
  3. internal/store         — SQLite schema + CRUD; unit-testable without HTTP

Phase 3: Server logic
  4. wdl/workflow change    — add SetRunTaskFn() setter (1-line change)
  5. internal/server        — HTTP handlers + orchestrator

Phase 4: Binaries
  6. cmd/flowy-server       — thin main() wiring server + store + config
  7. cmd/flowy-daemon       — poll loop + task.RunTask() invocation
  8. cmd/flowy (extend)     — add submit/status/outputs subcommands

Phase 5: Integration
  9. End-to-end test with real shared filesystem
  10. flowy run sync mode (config-aware dispatch)
```

---

## Anti-Patterns

### Anti-Pattern 1: Modifying the DAG Scheduler Internals for Distribution

**What people do:** Gut `wdl/workflow/scheduler.go` and rewrite it with HTTP calls and database state baked in.

**Why it's wrong:** The scheduler is the most complex, most tested part of the codebase (scatter, conditional, errgroup, channel drain). Modifying it introduces regression risk across all existing spectool tests. The `runTaskFn` variable injection point already exists and is specifically designed for substitution.

**Do this instead:** Use `SetRunTaskFn()` to inject a distributed task executor. The scheduler loop, errgroup concurrency, and channel protocol stay exactly as they are.

### Anti-Pattern 2: Requiring CGo in the Daemon Binary

**What people do:** Both `flowy-server` and `flowy-daemon` import `mattn/go-sqlite3` (CGo).

**Why it's wrong:** The daemon does not need SQLite. It only needs the `internal/api` DTO types and `wdl/task`. Pulling in CGo increases binary size, complicates cross-compilation, and makes the daemon harder to deploy on worker nodes.

**Do this instead:** Only `cmd/flowy-server` imports `internal/store` (and thus CGo). The daemon communicates state entirely through the REST API. The daemon build stays CGo-free.

### Anti-Pattern 3: Embedded SQLite in the Daemon (for local caching)

**What people do:** Each daemon maintains its own SQLite file for caching task state locally.

**Why it's wrong:** Creates split-brain state when the server needs authoritative task status. The server cannot recover crashed daemon tasks if state is local to the daemon.

**Do this instead:** All authoritative state lives in the server's SQLite database. Daemons are stateless workers — they poll, execute, and report. The reaper goroutine in the server handles crashed-daemon recovery.

### Anti-Pattern 4: Sending the Full AST Over the Wire

**What people do:** Serialize `*ast.Task` to JSON for the task assignment payload.

**Why it's wrong:** The AST has complex nested types without JSON tags. Adding JSON tags to all AST nodes is a large change that bleeds distribution concerns into the parser/AST layer.

**Do this instead:** Send `WDLPath` + `TaskName` in `TaskAssignment`. The daemon re-parses the WDL file from the shared filesystem. The WDL source is already available there (client copies it on submission). This reuse of the existing parser is correct behavior — same code path as local execution.

### Anti-Pattern 5: Per-Run Goroutines Without Bounded Concurrency

**What people do:** For each submitted workflow, start an uncontrolled goroutine that calls `workflow.RunWorkflow()`.

**Why it's wrong:** Each active workflow holds a goroutine that blocks waiting for task completions. Many concurrent workflow submissions can exhaust memory.

**Do this instead:** Limit active concurrent workflows with a semaphore (`make(chan struct{}, N)`). Default N = 10 (configurable). Submissions beyond the limit queue in SQLite as `SUBMITTED` status and are picked up as slots free.

---

## Scaling Considerations

| Scale | Architecture notes |
|-------|-------------------|
| 1-4 daemons, single host | Single `flowy-server` + N `flowy-daemon` processes on one machine, `/tmp/flowy-shared` as shared root. SQLite is fully sufficient. |
| 5-20 daemons, multi-host | NFS mount for shared_root. SQLite WAL handles the server-side write concurrency (one server writer at a time). No change to architecture. |
| 20+ daemons | SQLite write serialization becomes a bottleneck only if task completion rate is very high (>100 completions/second). At that point, migrate `internal/store` to PostgreSQL behind the same Store interface. No other changes needed. |

The architecture is designed so the Store interface is the only boundary that needs to change for database backend migration. The server, daemon, and client are unaffected.

---

## Sources

- Codebase analysis: `/home/uehara/flowy/wdl/task/runner.go`, `task/task.go`, `workflow/scheduler.go`, `workflow/workflow.go`, `workflow/dag.go`, `cmd/flowy/main.go`
- GA4GH Task Execution Service OpenAPI spec: [ga4gh/task-execution-schemas](https://github.com/ga4gh/task-execution-schemas) — informed REST endpoint naming conventions
- SQLite WAL concurrency in Go: [turso.tech/blog/something-you-probably-want-to-know-about-if-youre-using-sqlite-in-golang](https://turso.tech/blog/something-you-probably-want-to-know-about-if-youre-using-sqlite-in-golang-72547ad625f1) — WAL + SetMaxOpenConns(1) recommendation (HIGH confidence, verified against mattn/go-sqlite3 issue tracker)
- Cromwell architecture reference: [cromwell.readthedocs.io](https://cromwell.readthedocs.io/en/stable/) — polling interval pattern, task lifecycle states (MEDIUM confidence, different tech stack but same domain)
- goqite (SQLite queue for Go): [maragudk/goqite](https://github.com/maragudk/goqite) — confirms SQLite-backed task queue is viable pattern in Go
- miniwdl distributed execution model: [miniwdl-cloud](https://github.com/mlin/miniwdl-cloud) — shared filesystem approach validated for WDL distribution

---

*Architecture research for: flowy v1.1 distributed WDL execution*
*Researched: 2026-03-11*
