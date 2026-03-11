# Phase 10: Server Binary + DAG Dispatch - Research

**Researched:** 2026-03-11
**Domain:** Go HTTP server binary, chi v5 routing, background goroutine DAG dispatch, context-based abort
**Confidence:** HIGH

## Summary

Phase 10 builds `cmd/flowy-server/` — the server binary that accepts workflow submissions via REST,
schedules the DAG in-process, and dispatches each task as a row in the SQLite queue for daemon
pickup. All infrastructure it depends on is already built in Phase 9: `internal/config`,
`internal/store` (Store interface + SQLiteStore), and `internal/api` (DTO types). `chi v5` is
already in `go.mod` at v5.2.5. `google/uuid` is already in `go.mod` at v1.6.0.

There are three distinct sub-problems: (1) wiring chi routes to handlers that call the Store, (2)
launching a background goroutine per run that drives the existing `wdl/workflow` scheduler but with
`runTaskFn` replaced by a dispatch function that writes to the SQLite queue, and (3) handling
SIGTERM/SIGINT for graceful shutdown — stopping the HTTP listener first, then waiting for in-flight
schedulers to drain.

The critical design decision from STATE.md is that the `runTaskFn` injection point in
`wdl/workflow/scheduler.go` is the designated extension seam. Phase 10 constructs a `dispatchFn`
that calls `store.CreateTask` instead of Docker, transitions the run state to RUNNING, and
periodically checks for abort signals. The DAG scheduler goroutine is the only writer of run state
transitions (PENDING→RUNNING→COMPLETED/FAILED/ABORTED); HTTP handlers only READ run state and
write abort requests via `store.UpdateRunState`.

**Primary recommendation:** One `internal/server` package containing router wiring + handler
implementations. `cmd/flowy-server/main.go` is thin: load config, open store, construct server,
`ListenAndServe`. Use `signal.NotifyContext` for graceful shutdown with a 30-second drain timeout.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| SRV-01 | POST /workflows accepts WDL path + inputs JSON, returns run ID + HTTP 201 | chi route, uuid.New(), store.CreateRun, background goroutine |
| SRV-02 | GET /workflows/{id}/status returns run state | chi URLParam, store.GetRun |
| SRV-03 | GET /workflows/{id}/outputs returns output key-value pairs after COMPLETED | chi URLParam, store.GetRun + read outputs.json from shared FS |
| SRV-04 | POST /workflows/{id}/abort transitions run to ABORTED, cancels in-progress tasks | per-run context.CancelFunc stored in runRegistry map, store.UpdateRunState, store.UpdateTaskState |
| SRV-05 | GET /health returns HTTP 200 | trivial chi route |
| SRV-06 | Server runs DAG scheduler in-process via runTaskFn injection to dispatch to SQLite queue | scheduler.taskRunner field injection with dispatchFn that calls store.CreateTask |
| SRV-09 | Binary ships as flowy-server under cmd/flowy-server/ | standard Go `go build -o bin/flowy-server ./cmd/flowy-server` |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/go-chi/chi/v5` | v5.2.5 | HTTP routing, URL param extraction | Already in go.mod; decided in STATE.md; zero external deps; stdlib-compatible |
| `github.com/google/uuid` | v1.6.0 | Generate UUID v4 run IDs | Already in go.mod; standard for UUID v4 in Go |
| `encoding/json` (stdlib) | — | JSON encode/decode for handlers | Stdlib; no external JSON library needed |
| `net/http` (stdlib) | — | HTTP server | Stdlib; chi wraps this, not replaces it |
| `os/signal` + `signal.NotifyContext` (stdlib) | — | SIGTERM/SIGINT graceful shutdown | Go 1.16+; idiomatic signal handling |
| `context` (stdlib) | — | Per-run cancellation for abort | Context cancel propagated to scheduler goroutine |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `chi/v5/middleware` | v5.2.5 | Logger, Recoverer | Use Logger for access log, Recoverer to prevent panics from crashing server |
| `golang.org/x/sync/errgroup` | already in go.sum | Already used by scheduler; present transitively | Do NOT add another concurrency abstraction; errgroup is already the scheduler's model |
| `internal/config` | (project) | Config loading for server binary | Config.Server.ListenAddr, HeartbeatTimeoutSec |
| `internal/store` | (project) | Store interface; all persistence calls go through this | Never import store/sqlite directly in handlers |
| `internal/api` | (project) | DTO types for request/response encode/decode | Already defined; import as-is |
| `wdl/workflow` | (project) | RunWorkflow + scheduler with taskRunner injection | Core DAG engine; inject dispatchFn via scheduler.taskRunner |
| `wdl/parser` | (project) | Parse WDL file from path at submit time | Call parser.Parse(wdlPath, nil) before CreateRun |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| chi v5 | gorilla/mux, gin | Chi already in go.mod; gorilla deprecated; gin is heavier; chi is the locked choice |
| signal.NotifyContext | manual channel + signal.Notify | NotifyContext is idiomatic Go 1.16+, fewer lines, integrates with context cancellation |
| Background goroutine per run | dedicated worker pool | Per-run goroutine is simpler and correct for low concurrency HPC; worker pool adds complexity not needed until high fan-out |

**No new go.mod additions required.** All dependencies already present.

## Architecture Patterns

### Recommended Project Structure
```
cmd/
└── flowy-server/
    └── main.go          # thin: load config, open store, construct Server, ListenAndServe, shutdown

internal/
├── config/              # already exists (Phase 9)
├── store/               # already exists (Phase 9)
│   ├── store.go
│   └── sqlite/
├── api/                 # already exists (Phase 9)
│   └── types.go
└── server/              # NEW in Phase 10
    ├── server.go        # Server struct, New(), router wiring, Serve()
    ├── handlers.go      # HTTP handlers: submit, status, outputs, abort, health
    └── dispatch.go      # dispatchFn and per-run goroutine lifecycle
```

### Pattern 1: Server Struct with Injected Dependencies

**What:** A `Server` struct that holds `store.Store`, `*config.Config`, and a `runRegistry` map
(run ID → context.CancelFunc for abort). Router is wired in `New()` and never changes after
construction.

**When to use:** Always. Avoids package-level globals; enables testability via interface injection.

**Example:**
```go
// internal/server/server.go
// Source: go-chi/chi idiomatic pattern (pkg.go.dev/github.com/go-chi/chi/v5)

package server

import (
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "github.com/bioflowy/flowy/internal/config"
    "github.com/bioflowy/flowy/internal/store"
)

type Server struct {
    cfg      *config.Config
    store    store.Store
    router   *chi.Mux
    mu       sync.Mutex
    runs     map[string]context.CancelFunc // run ID → cancel for abort
}

func New(cfg *config.Config, st store.Store) *Server {
    s := &Server{
        cfg:   cfg,
        store: st,
        runs:  make(map[string]context.CancelFunc),
    }
    s.router = s.newRouter()
    return s
}

func (s *Server) newRouter() *chi.Mux {
    r := chi.NewRouter()
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)

    r.Get("/health", s.handleHealth)
    r.Post("/workflows", s.handleSubmit)
    r.Get("/workflows/{id}/status", s.handleStatus)
    r.Get("/workflows/{id}/outputs", s.handleOutputs)
    r.Post("/workflows/{id}/abort", s.handleAbort)

    return r
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    s.router.ServeHTTP(w, r)
}
```

### Pattern 2: Graceful Shutdown with signal.NotifyContext

**What:** `main.go` uses `signal.NotifyContext` to get a context that is cancelled on SIGTERM or
SIGINT. The HTTP server is started in a goroutine; `Shutdown()` is called when the signal context
fires with a 30-second drain timeout.

**When to use:** All server binaries. Standard Go 1.16+ idiom.

**Example:**
```go
// cmd/flowy-server/main.go

package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "time"

    "github.com/bioflowy/flowy/internal/config"
    "github.com/bioflowy/flowy/internal/server"
    "github.com/bioflowy/flowy/internal/store/sqlite"
)

func main() {
    // Load config (--config flag or ./flowy.yaml default).
    cfgPath := configPath() // flag or default
    cfg, err := config.Load(cfgPath)
    if err != nil {
        log.Fatalf("config: %v", err)
    }

    // Open store.
    st, err := sqlite.Open(config.DefaultDSN(cfg))
    if err != nil {
        log.Fatalf("store: %v", err)
    }
    defer st.Close()

    // Build server.
    srv := server.New(cfg, st)

    httpSrv := &http.Server{
        Addr:    cfg.Server.ListenAddr,
        Handler: srv,
    }

    // signal.NotifyContext cancels ctx on SIGTERM or SIGINT.
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    go func() {
        log.Printf("flowy-server listening on %s", cfg.Server.ListenAddr)
        if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("listen: %v", err)
        }
    }()

    // Wait for signal.
    <-ctx.Done()
    stop() // reset signal handling

    // Graceful shutdown with 30-second timeout.
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    if err := httpSrv.Shutdown(shutdownCtx); err != nil {
        log.Printf("shutdown: %v", err)
    }
}
```

### Pattern 3: Submit Handler — Parse, Validate, Persist, Launch Goroutine

**What:** `POST /workflows` handler: (1) decode request body, (2) parse WDL to validate it exists
and is syntactically valid, (3) generate UUID run ID, (4) create run dirs, (5) call
`store.CreateRun`, (6) launch background goroutine that runs the DAG scheduler with injected
`dispatchFn`, (7) return 201 with run ID.

**Critical:** The goroutine receives a derived context with `context.WithCancel`; the cancel func
is stored in `s.runs[runID]` for abort support.

**When to use:** POST /workflows handler only.

**Example:**
```go
// internal/server/handlers.go

func (s *Server) handleSubmit(w http.ResponseWriter, r *http.Request) {
    var req api.SubmitRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }
    if req.WDLPath == "" {
        writeError(w, http.StatusBadRequest, "wdl_path is required")
        return
    }

    // Parse and validate WDL exists.
    doc, err := parser.Parse(req.WDLPath, nil)
    if err != nil {
        writeError(w, http.StatusBadRequest, "WDL parse error: "+err.Error())
        return
    }

    // Generate run ID.
    runID := uuid.New().String()

    // Create filesystem run dirs.
    if err := store.CreateRunDirs(s.cfg.BaseDir, runID); err != nil {
        writeError(w, http.StatusInternalServerError, "failed to create run dirs")
        return
    }

    // Persist run record.
    inputsJSON, _ := json.Marshal(req.InputsJSON)
    run := &storemod.Run{
        ID:         runID,
        WDLPath:    req.WDLPath,
        InputsJSON: string(inputsJSON),
        State:      storemod.RunPending,
    }
    if err := s.store.CreateRun(r.Context(), run); err != nil {
        writeError(w, http.StatusInternalServerError, "failed to create run")
        return
    }

    // Launch scheduler goroutine.
    runCtx, cancel := context.WithCancel(context.Background())
    s.mu.Lock()
    s.runs[runID] = cancel
    s.mu.Unlock()

    go s.runWorkflow(runCtx, runID, doc, req.InputsJSON)

    writeJSON(w, http.StatusCreated, api.SubmitResponse{RunID: runID})
}
```

### Pattern 4: DAG Dispatch via taskRunner Injection

**What:** `s.runWorkflow` calls `workflow.RunWorkflow` but with a custom `taskRunner` that writes
task rows to SQLite instead of executing Docker. The `scheduler.taskRunner` field is the designed
extension seam from Phase 9.

**Critical constraint from scheduler.go:** The `scheduler.taskRunner` field is an unexported struct
field. To inject from outside the package, either: (a) expose `RunWorkflowWithTaskRunner` as a new
exported function in `wdl/workflow/`, or (b) use the package-level `runTaskFn` variable (also
unexported, test-only pattern). Option (a) is the clean approach — add one exported function.

**When to use:** Always in Phase 10 — this is the core SRV-06 requirement.

**Example — new exported function in wdl/workflow/workflow.go:**
```go
// wdl/workflow/workflow.go — ADD this function

// RunWorkflowWithDispatch executes a workflow using a custom task dispatch function
// instead of the default Docker runner. This is used by flowy-server to enqueue
// tasks into the SQLite queue rather than executing them locally.
func RunWorkflowWithDispatch(
    ctx context.Context,
    doc *ast.Document,
    wfName string,
    jsonInputs map[string]interface{},
    runner task.ContainerRunner,
    execRoot string,
    dispatchFn func(ctx context.Context, t *ast.Task, inputs map[string]interface{}, runner task.ContainerRunner, execRoot string, structDefs ...map[string]*ast.StructDef) (map[string]values.Value, error),
) (map[string]values.Value, error) {
    // Same as RunWorkflow but sets s.taskRunner = dispatchFn before s.run(ctx).
}
```

**Alternatively (simpler):** The `scheduler` struct is in the same package as `RunWorkflow`. A new
`RunWorkflowWithDispatch` function can build the scheduler with `taskRunner` set, keeping all
scheduler logic private.

**dispatch.go pattern:**
```go
// internal/server/dispatch.go

// dispatchFn returns a taskRunner function that creates a task row in the store.
// When the scheduler calls this for each WDL task call, instead of running Docker,
// it persists a PENDING task row. The daemon will claim and execute it.
func (s *Server) makeDispatchFn(runID string) func(...) (map[string]values.Value, error) {
    return func(ctx context.Context, t *ast.Task, inputs map[string]interface{},
        runner task.ContainerRunner, execRoot string,
        structDefs ...map[string]*ast.StructDef) (map[string]values.Value, error) {

        // Write inputs to shared FS so daemon can find them.
        // inputs dir = {baseDir}/runs/{runID}/inputs/{taskName}/
        // (detail: exact path convention TBD in Phase 10 planning)

        task := &storemod.Task{
            RunID:    runID,
            TaskName: t.Name,
        }
        if err := s.store.CreateTask(ctx, task); err != nil {
            return nil, fmt.Errorf("dispatch %q: %w", t.Name, err)
        }

        // Wait for task to reach COMPLETED or FAILED state (poll store).
        // This blocking wait is what keeps the DAG scheduler paused until
        // the daemon finishes the task.
        return s.waitForTask(ctx, task.ID)
    }
}

// waitForTask polls the store until the task reaches a terminal state.
// Returns outputs loaded from shared FS on COMPLETED; error on FAILED/ABORTED.
func (s *Server) waitForTask(ctx context.Context, taskID int64) (map[string]values.Value, error) {
    ticker := time.NewTicker(2 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        case <-ticker.C:
            t, err := s.store.GetTaskByID(ctx, taskID)  // needs GetTaskByID in Store
            if err != nil {
                return nil, err
            }
            switch t.State {
            case storemod.TaskCompleted:
                // Load outputs from shared FS outputs dir.
                return loadTaskOutputs(s.cfg.BaseDir, t.RunID, t.TaskName)
            case storemod.TaskFailed:
                return nil, fmt.Errorf("task %q failed", t.TaskName)
            case storemod.TaskAborted:
                return nil, fmt.Errorf("task %q aborted", t.TaskName)
            }
        }
    }
}
```

**IMPORTANT DISCOVERY:** The `Store` interface does NOT have a `GetTaskByID` method. Phase 10
will need to add it to the Store interface and SQLiteStore implementation, or use
`ListTasksByRun` and scan for the specific ID. Adding `GetTaskByID` to the store interface
is the cleaner approach and MUST be done as part of Phase 10.

### Pattern 5: Abort Handler

**What:** `POST /workflows/{id}/abort` cancels the per-run context (which propagates to the
scheduler goroutine's `ctx`), then transitions the run state and any PENDING/CLAIMED tasks to
ABORTED in the store.

**When to use:** abort endpoint only.

**Example:**
```go
func (s *Server) handleAbort(w http.ResponseWriter, r *http.Request) {
    runID := chi.URLParam(r, "id")

    s.mu.Lock()
    cancel, ok := s.runs[runID]
    s.mu.Unlock()

    if !ok {
        writeError(w, http.StatusNotFound, "run not found or already completed")
        return
    }

    // Cancel the context — scheduler goroutine exits on next ctx.Done() check.
    cancel()

    // Transition run state immediately.
    if err := s.store.UpdateRunState(r.Context(), runID, storemod.RunAborted); err != nil {
        writeError(w, http.StatusInternalServerError, "failed to update run state")
        return
    }

    writeJSON(w, http.StatusOK, api.AbortResponse{RunID: runID, State: "ABORTED"})
}
```

### Pattern 6: Outputs Handler — Read from Shared FS

**What:** `GET /workflows/{id}/outputs` reads the run from store, checks it is COMPLETED, then
reads `outputs.json` written by `wdl/workflow/RunWorkflow` (which writes it to `execRoot` on
success). The `execRoot` for server-dispatched runs is `{baseDir}/runs/{runID}/`.

**When to use:** outputs endpoint only.

**Example:**
```go
func (s *Server) handleOutputs(w http.ResponseWriter, r *http.Request) {
    runID := chi.URLParam(r, "id")
    run, err := s.store.GetRun(r.Context(), runID)
    if err != nil {
        writeError(w, http.StatusNotFound, "run not found")
        return
    }
    if run.State != storemod.RunCompleted {
        writeError(w, http.StatusConflict, "run not completed (state: "+string(run.State)+")")
        return
    }

    // Read outputs.json from shared FS.
    outPath := filepath.Join(s.cfg.BaseDir, "runs", runID, "outputs.json")
    data, err := os.ReadFile(outPath)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "outputs not found")
        return
    }
    var outputs map[string]interface{}
    if err := json.Unmarshal(data, &outputs); err != nil {
        writeError(w, http.StatusInternalServerError, "outputs parse error")
        return
    }
    writeJSON(w, http.StatusOK, api.OutputsResponse{RunID: runID, Outputs: outputs})
}
```

### Pattern 7: JSON Helper Functions

**What:** Two small helpers used by all handlers: `writeJSON` (encode struct as JSON with
content-type and status code) and `writeError` (encode `api.ErrorResponse` as JSON).

**When to use:** Every handler. Do not inline the encoding logic.

**Example:**
```go
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
    writeJSON(w, status, api.ErrorResponse{Error: msg})
}
```

### Anti-Patterns to Avoid

- **Running DAG scheduler in the HTTP handler goroutine:** The scheduler blocks until the workflow
  completes. It MUST run in a background goroutine. The handler only launches it and returns 201.
- **Storing cancel func in the http.Request context:** Request context is cancelled after the handler
  returns. Use `context.Background()` as the base for the run's goroutine context, not
  `r.Context()`.
- **Using `r.Context()` as the run goroutine's context:** The request ends at 201; its context gets
  cancelled. Derive from `context.Background()` instead.
- **Direct SQLiteStore import in internal/server:** Import only `internal/store` (the interface).
  The concrete SQLiteStore is wired in `cmd/flowy-server/main.go` only.
- **No mutex around s.runs map:** The map is written by the HTTP handler goroutine and read/deleted
  by the run goroutine. Always protect with `sync.Mutex`.
- **Blocking `json.NewDecoder` without size limit:** For a server on an internal network this is
  low risk, but use `http.MaxBytesReader` to bound request body size as defense-in-depth.
- **Writing run state transitions from both the HTTP abort handler AND the scheduler goroutine
  concurrently without coordination:** The abort handler sets ABORTED; the scheduler may be
  concurrently trying to set COMPLETED. The scheduler must check `ctx.Done()` and skip the
  COMPLETED transition if cancelled. Or use a SELECT in SQL: `UPDATE runs SET state='ABORTED'
  WHERE id=? AND state != 'COMPLETED'`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| URL routing with path params | Manual `strings.Split(path, "/")` | chi v5 (already in go.mod) | chi handles radix trie routing, path params, 405/404, middleware chain |
| UUID generation | Custom random hex string | `uuid.New().String()` (google/uuid, already in go.mod) | RFC 4122 compliance, crypto/rand backed |
| Graceful shutdown | Manual `os.Signal` channel + `sync.WaitGroup` | `signal.NotifyContext` + `http.Server.Shutdown` (stdlib) | Idiomatic, context-integrated, handles SIGTERM and SIGINT |
| JSON encode/decode | Manual string building | `encoding/json` (stdlib) | No external JSON library needed at this scale |
| Per-request panic recovery | Deferred recover in every handler | `middleware.Recoverer` from chi (already bundled) | Single middleware prevents panics from crashing the server |
| Access logging | fmt.Println in every handler | `middleware.Logger` from chi | Structured request log with duration |
| Abort/cancel signaling to background goroutines | Custom boolean flag + channel | `context.WithCancel` (stdlib) | Context cancel propagates to nested calls inside scheduler |

**Key insight:** The dispatch architecture is not a new "worker queue system" — it is a thin
adapter: the existing scheduler calls `dispatchFn` instead of Docker, `dispatchFn` writes to
SQLite, and the scheduler blocks until the task is done. No message broker, no worker pool
abstraction, no separate queue process. SQLite IS the queue.

## Common Pitfalls

### Pitfall 1: Store Interface Missing GetTaskByID
**What goes wrong:** `waitForTask` needs to poll a single task's state. The current Store interface
(Phase 9) only has `ListTasksByRun`. Calling `ListTasksByRun` in a tight poll loop on large runs
with many tasks is wasteful.
**Why it happens:** Phase 9 defined the minimal interface; polling by task ID was not in scope.
**How to avoid:** Add `GetTaskByID(ctx context.Context, taskID int64) (*Task, error)` to the Store
interface and implement it in SQLiteStore as part of Phase 10 Wave 1.
**Warning signs:** N+1 store query pattern or inability to poll efficiently.

### Pitfall 2: Using r.Context() as Base for Run Goroutine
**What goes wrong:** The scheduler goroutine starts, processes tasks, but after the HTTP response
is sent the request context is cancelled. The next `ctx.Done()` check in `waitForTask` returns
immediately, causing spurious aborts of all in-flight tasks.
**Why it happens:** HTTP request contexts are tied to the connection lifetime. They are intentionally
cancelled when the response is written.
**How to avoid:** `runCtx, cancel := context.WithCancel(context.Background())`. Never use
`r.Context()` as the base for a goroutine that outlives the HTTP request.
**Warning signs:** Runs consistently fail immediately after submission; context.Canceled errors
from waitForTask.

### Pitfall 3: Data Race on s.runs Map
**What goes wrong:** race detector fires on `s.runs[runID] = cancel` from the HTTP handler and
concurrent `delete(s.runs, runID)` from the goroutine goroutine on completion.
**Why it happens:** Go maps are not goroutine-safe.
**How to avoid:** Wrap all `s.runs` accesses with `s.mu.Lock()/Unlock()`.
**Warning signs:** `go test -race` reports DATA RACE on map read/write.

### Pitfall 4: ABORTED vs COMPLETED Race Condition
**What goes wrong:** User calls abort; simultaneously the last task completes and the scheduler
goroutine calls `store.UpdateRunState(ctx, runID, RunCompleted)`. The abort handler won if its
SQL ran first, but the scheduler overwrites with COMPLETED.
**Why it happens:** Two goroutines race to update the same run row without coordination.
**How to avoid:** In the scheduler's completion logic, use a conditional UPDATE: `UPDATE runs SET
state='COMPLETED' WHERE id=? AND state='RUNNING'`. The abort handler sets ABORTED with `WHERE
state != 'COMPLETED'`. At most one wins.
**Warning signs:** Integration test for abort shows final state sometimes COMPLETED, sometimes ABORTED.

### Pitfall 5: scheduler.taskRunner is an Unexported Field
**What goes wrong:** `internal/server/dispatch.go` tries to set `scheduler.taskRunner` but
cannot access it — it is unexported.
**Why it happens:** The `scheduler` struct and `taskRunner` field are package-private to
`wdl/workflow`.
**How to avoid:** Add an exported function `RunWorkflowWithDispatch(ctx, doc, wfName, inputs,
runner, execRoot, dispatchFn)` to `wdl/workflow/workflow.go`. This is the clean API extension; do
NOT change the unexported field.
**Warning signs:** `cannot refer to unexported field 'taskRunner'` compilation error.

### Pitfall 6: WDL Parse at Submit Time vs. Execute Time
**What goes wrong:** WDL file path is stored in DB at submit time. At execution time (run goroutine
launch), the file may no longer exist if the client deleted it.
**Why it happens:** We parse at submit to validate, but the goroutine re-uses the stored WDLPath
from the DB.
**How to avoid:** Parse at submit, pass the `*ast.Document` directly into the goroutine as a
closure parameter (not re-read from disk). The goroutine captures `doc` from the submit handler.
**Warning signs:** Intermittent "no such file" errors on run execution while DB shows RUNNING.

### Pitfall 7: Long-Poll for Daemon — Out of Scope in Phase 10
**What goes wrong:** STATE.md blocker note says long-poll for daemon task claims should be
researched. This affects Phase 11 (daemon), NOT Phase 10. The server in Phase 10 only needs
`GET /tasks/claim` (short-poll response) to support future daemon integration.
**Why it happens:** The daemon endpoint is in `internal/api/types.go` (`TaskClaimRequest`,
`TaskClaimResponse`) but the Phase 10 server only needs to expose enough for Phase 11 to work.
**How to avoid:** Expose a `/tasks/claim` endpoint in Phase 10 as a simple short-poll: if no
PENDING task, return HTTP 204. Daemon (Phase 11) will add the long-poll layer.
**Warning signs:** Over-engineering with `http.Flusher` and goroutine-per-connection patterns
before daemon is written.

## Code Examples

### chi Router Setup with Middleware
```go
// Source: pkg.go.dev/github.com/go-chi/chi/v5 (verified current API)

r := chi.NewRouter()
r.Use(middleware.Logger)    // access log
r.Use(middleware.Recoverer) // panic recovery

r.Get("/health", s.handleHealth)
r.Post("/workflows", s.handleSubmit)
r.Get("/workflows/{id}/status", s.handleStatus)
r.Get("/workflows/{id}/outputs", s.handleOutputs)
r.Post("/workflows/{id}/abort", s.handleAbort)

// URL param extraction in handlers:
runID := chi.URLParam(r, "id")
```

### Graceful Shutdown with signal.NotifyContext
```go
// Source: Go stdlib os/signal.NotifyContext (Go 1.16+)

ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

go func() {
    if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Fatalf("listen: %v", err)
    }
}()

<-ctx.Done()
stop()

shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
_ = httpSrv.Shutdown(shutdownCtx)
```

### UUID v4 Generation (already in go.mod)
```go
// Source: pkg.go.dev/github.com/google/uuid v1.6.0 (verified in go.mod)
import "github.com/google/uuid"

runID := uuid.New().String()
// returns e.g. "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
```

### Per-Run Context for Abort Support
```go
// Source: Go stdlib context package

// In submit handler:
runCtx, cancel := context.WithCancel(context.Background()) // NOT r.Context()
s.mu.Lock()
s.runs[runID] = cancel
s.mu.Unlock()
go s.runWorkflow(runCtx, runID, doc, inputs)

// In abort handler:
s.mu.Lock()
cancel, ok := s.runs[runID]
s.mu.Unlock()
if ok {
    cancel() // propagates to scheduler goroutine via ctx.Done()
}

// In run goroutine on completion (either success, failure, or abort):
s.mu.Lock()
delete(s.runs, runID)
s.mu.Unlock()
```

### Conditional Run State Update (ABORTED/COMPLETED race prevention)
```sql
-- Complete only if still RUNNING (not already ABORTED):
UPDATE runs SET state = 'COMPLETED', updated_at = datetime('now')
WHERE id = ? AND state = 'RUNNING';

-- Abort only if not already COMPLETED:
UPDATE runs SET state = 'ABORTED', updated_at = datetime('now')
WHERE id = ? AND state != 'COMPLETED';
```

### Config Flag in main.go
```go
// cmd/flowy-server/main.go
flag.StringVar(&cfgPath, "config", "./flowy.yaml", "path to config file")
flag.Parse()
cfg, err := config.Load(cfgPath)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| gorilla/mux | chi v5 | gorilla deprecated ~2022 | chi is maintained, same API surface |
| os.Signal channel manually | signal.NotifyContext | Go 1.16 (2021) | One function call instead of 10 lines |
| Blocking HTTP handlers for async work | Background goroutine + 201 Accepted | HTTP/1.1 best practices always | Clients don't time out waiting for workflow completion |
| package-level global state | Struct with injected dependencies | Go testing patterns | Testable without global state mutation |

**Deprecated/outdated:**
- `gorilla/mux`: Maintenance archived; chi v5 is the locked choice for this project.
- Using `runTaskFn` package-level variable for injection (test-only pattern): Use the new
  exported `RunWorkflowWithDispatch` function instead.

## Open Questions

1. **GetTaskByID missing from Store interface**
   - What we know: Store interface has `ListTasksByRun` but not `GetTaskByID`. `waitForTask`
     needs to poll a single task by its int64 ID efficiently.
   - What's unclear: Whether to add `GetTaskByID` to the interface or implement a different polling
     mechanism (e.g., polling `ListTasksByRun` and scanning).
   - Recommendation: Add `GetTaskByID(ctx context.Context, taskID int64) (*Task, error)` to the
     Store interface and SQLiteStore in Phase 10 Wave 1. This is a minimal extension that avoids
     O(n) scans per poll tick.

2. **Outputs.json format and key naming**
   - What we know: `wdl/workflow/workflow.go` writes `outputs.json` to `execRoot` on COMPLETED.
     In the current local run model, `execRoot` is a temp dir. For server dispatch, the scheduler's
     `execRoot` will be `{baseDir}/runs/{runID}/`.
   - What's unclear: Whether `RunWorkflowWithDispatch` can pass `{baseDir}/runs/{runID}/` as
     `execRoot` and have `outputs.json` written there correctly, or whether the run goroutine needs
     to manage outputs.json differently when using the dispatch model.
   - Recommendation: In the dispatch model, workflow outputs ARE the aggregated task outputs
     collected from shared FS. The `RunWorkflowWithDispatch` function still writes `outputs.json`
     to `execRoot` = `{baseDir}/runs/{runID}/`. This should work unchanged — confirm in tests.

3. **Task inputs serialization to shared FS**
   - What we know: When `dispatchFn` is called, it receives `inputs map[string]interface{}` —
     the evaluated WDL call inputs. The daemon will need to find these inputs on shared FS.
   - What's unclear: Exact file convention for per-task inputs (JSON file? directory? which name?).
   - Recommendation: Write `inputs.json` to `{baseDir}/runs/{runID}/tasks/{taskName}/inputs.json`.
     This can be finalized in planning — the research confirms the FS path helpers in
     `internal/store/rundir.go` need to be extended for per-task paths.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go standard `testing` package (go test) |
| Config file | none — standard Go test tooling |
| Quick run command | `go test ./internal/server/... -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SRV-01 | POST /workflows returns 201 + run ID, run exists in DB as PENDING | integration | `go test ./internal/server/... -run TestSubmit -count=1` | Wave 0 |
| SRV-02 | GET /workflows/{id}/status returns current state for known run | integration | `go test ./internal/server/... -run TestStatus -count=1` | Wave 0 |
| SRV-03 | GET /workflows/{id}/outputs returns outputs after COMPLETED state | integration | `go test ./internal/server/... -run TestOutputs -count=1` | Wave 0 |
| SRV-04 | POST /workflows/{id}/abort transitions run to ABORTED | integration | `go test ./internal/server/... -run TestAbort -count=1` | Wave 0 |
| SRV-05 | GET /health returns 200 | unit | `go test ./internal/server/... -run TestHealth -count=1` | Wave 0 |
| SRV-06 | DAG scheduler goroutine dispatches tasks to SQLite queue via dispatchFn | integration | `go test ./internal/server/... -run TestDispatch -count=1` | Wave 0 |
| SRV-09 | flowy-server binary builds successfully | build | `go build ./cmd/flowy-server/` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/server/... -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/server/server_test.go` — covers SRV-01 through SRV-06 using httptest.NewRecorder
- [ ] `internal/server/handlers_test.go` — handler unit tests with mock store
- [ ] `cmd/flowy-server/main.go` — binary entry point (build test only; no unit test needed)
- [ ] Store extension: `GetTaskByID` in `internal/store/store.go` and `internal/store/sqlite/tasks.go`
- [ ] `wdl/workflow/workflow.go` — add `RunWorkflowWithDispatch` exported function

*(No new packages to install — all dependencies already in go.mod)*

## Sources

### Primary (HIGH confidence)
- `pkg.go.dev/github.com/go-chi/chi/v5` v5.2.5 — router creation, URLParam, middleware, route patterns (verified via WebFetch)
- `pkg.go.dev/github.com/google/uuid` v1.6.0 — uuid.New().String() API (verified via go.mod presence + WebSearch)
- Go stdlib `os/signal.NotifyContext` (Go 1.16+) — graceful shutdown pattern (verified via Go docs)
- Go stdlib `net/http.Server.Shutdown` — drain in-flight connections (stdlib, HIGH confidence)
- `internal/store/store.go` — Store interface (read directly from source)
- `wdl/workflow/scheduler.go` — taskRunner injection point (read directly from source)
- `internal/api/types.go` — DTO types (read directly from source)
- `internal/config/config.go` — Config struct and DefaultDSN (read directly from source)

### Secondary (MEDIUM confidence)
- WebSearch: chi v5 graceful shutdown example in `_examples/graceful/main.go` — pattern confirmed by multiple sources
- WebSearch: Go context cancellation propagation patterns — multiple authoritative sources
- WebSearch: `signal.NotifyContext` pattern for 2025 Go server shutdown — confirmed as current best practice

### Tertiary (LOW confidence)
- Exact `outputs.json` key naming convention in dispatch model — requires code walkthrough in planning
- Per-task input serialization path convention — deferred to planning

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries already in go.mod, verified versions
- Architecture: HIGH — patterns are standard Go HTTP server patterns; extension seam is verified in source
- Pitfalls: HIGH for context base / data race / unexported field (all verified from source); MEDIUM for ABORTED/COMPLETED race (logic analysis, needs test confirmation)

**Research date:** 2026-03-11
**Valid until:** 2026-09-11 (chi v5, uuid, stdlib APIs are all highly stable)
