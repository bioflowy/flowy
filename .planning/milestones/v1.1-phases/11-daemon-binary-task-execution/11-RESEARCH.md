# Phase 11: Daemon Binary + Task Execution - Research

**Researched:** 2026-03-12
**Domain:** Go daemon binary, Docker task execution, HTTP polling, graceful shutdown
**Confidence:** HIGH

## Summary

Phase 11 builds `flowy-daemon`, a binary that polls the server for PENDING tasks,
executes them via the existing `wdl/task.RunTask` function (which invokes Docker),
writes outputs to the shared filesystem in the format `dispatch.go` already expects
(outputs.json in the task directory), and handles SIGTERM gracefully.

The server side is fully implemented: `POST /tasks/claim` returns a claimed task or
204 No Content, `POST /tasks/{id}/result` accepts COMPLETED/FAILED, and
`POST /heartbeat` refreshes all CLAIMED/RUNNING tasks for a daemon in one SQL UPDATE.
The daemon only needs to be the HTTP client side of these three endpoints.

The critical integration path is:
1. Daemon calls `POST /tasks/claim` → gets task_id, task_name, shard_index, inputs_dir, outputs_dir
2. Daemon reads `inputs.json` from inputs_dir (written by server's `makeDispatchFn`)
3. Daemon calls `task.RunTask` with an `ast.Task` loaded by parsing the WDL file — the
   run's WDL path is stored in the `runs` table; daemon must call `GET /workflows/{run_id}/status`
   to get run metadata, or read inputs_dir to find the WDL path
4. Daemon writes `outputs.json` to the task directory (outputs_dir == inputs_dir per `handleClaim`)
5. Daemon calls `POST /tasks/{id}/result` with state=COMPLETED or FAILED

A key design decision: the server's `makeDispatchFn` writes `inputs.json` to the task dir
and the server's `loadTaskOutputs` reads `outputs.json` from that same directory. The
daemon must write its outputs as plain JSON with output names as keys (no task prefix),
matching what `loadTaskOutputs` expects.

**Primary recommendation:** Implement the daemon as a simple poll-execute loop in
`cmd/flowy-daemon/` backed by an `internal/daemon/` package with a `Daemon` struct
mirroring the `internal/server/Server` pattern.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| stdlib `net/http` | go1.25 | HTTP client for server API calls | Already used throughout; no external client needed |
| stdlib `os/signal` | go1.25 | SIGTERM/SIGINT handling | Same pattern as `cmd/flowy-server/main.go` |
| `wdl/task.RunTask` | internal | Docker task execution | Existing, tested, battle-proven — reuse, don't reinvent |
| `wdl/task.DockerRunner` | internal | Docker Engine API backend | Existing implementation, CGO-free |
| `wdl/parser` | internal | WDL file parsing (to get ast.Task) | Existing parser; daemon must parse WDL to get task definition |
| `internal/config` | internal | YAML config loading | Shared across all binaries (STR-04) |
| `internal/api` | internal | HTTP DTO types for server endpoints | TaskClaimResponse, HeartbeatRequest, TaskResultRequest |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/google/uuid` | v1.6.0 | Generate stable daemon_id | Already in go.mod; UUIDs are ideal stable instance identifiers |
| stdlib `encoding/json` | go1.25 | Marshal/unmarshal outputs.json | Consistent with rest of codebase |
| stdlib `time` | go1.25 | Heartbeat ticker, poll backoff | Standard |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| stdlib `net/http` client | `resty` or similar | stdlib is sufficient; no retry library needed for Phase 11 |
| uuid for daemon_id | hostname+pid | uuid is already in go.mod and produces stable unique IDs even across restarts if persisted |

**Installation:**
No new dependencies required. All needed packages are already in `go.mod`.

## Architecture Patterns

### Recommended Project Structure
```
cmd/
└── flowy-daemon/
    └── main.go          # signal handling, config load, daemon.Run()

internal/
└── daemon/
    ├── daemon.go        # Daemon struct, Run(), Shutdown(), poll loop
    ├── client.go        # HTTP client wrapper for server API (ClaimTask, SendHeartbeat, ReportResult)
    └── daemon_test.go   # integration tests using httptest.NewServer
```

### Pattern 1: Daemon Struct (mirrors internal/server/Server)
**What:** A `Daemon` struct holds config, HTTP client, and lifecycle state (WaitGroup, stop channel)
**When to use:** Single entry point; Clean shutdown via `Shutdown()` method

```go
// Source: mirrors internal/server/server.go pattern
type Daemon struct {
    cfg      *config.Config
    daemonID string
    client   *http.Client

    wg   sync.WaitGroup
    stop chan struct{}
}

func New(cfg *config.Config) *Daemon {
    return &Daemon{
        cfg:      cfg,
        daemonID: uuid.New().String(),
        client:   &http.Client{Timeout: 35 * time.Second},
        stop:     make(chan struct{}),
    }
}

func (d *Daemon) Run(ctx context.Context) error {
    // Start heartbeat goroutine
    // Start poll-execute loop
}

func (d *Daemon) Shutdown() {
    close(d.stop)
    d.wg.Wait()
}
```

### Pattern 2: Poll-Execute Loop
**What:** Poll for a task, claim it, execute, report result. Retry (with backoff) if no task.
**When to use:** Core daemon execution loop

```go
// Source: based on server's waitForTask pattern and requirements DMN-01/02/05
func (d *Daemon) pollLoop(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-d.stop:
            return
        case <-ticker.C:
            task, err := d.claimTask(ctx)
            if err != nil {
                log.Printf("daemon: claim task: %v", err)
                continue
            }
            if task == nil {
                // No pending task — back off naturally (ticker handles it)
                continue
            }
            // Execute in-line (single task at a time per daemon instance for simplicity)
            d.executeTask(ctx, task)
        }
    }
}
```

### Pattern 3: Task Execution Flow
**What:** Given a claimed task, read inputs.json, parse WDL, call RunTask, write outputs.json
**When to use:** Every time a task is claimed

The critical path:
1. `task_dir` is `inputs_dir` from the claim response (server sets `InputsDir == OutputsDir == TaskDir(...)`)
2. Read `{task_dir}/inputs.json` — JSON written by server's `makeDispatchFn`
3. Parse WDL to get `ast.Task` — daemon needs the run's WDL path; see "WDL Path Lookup" below
4. Call `task.RunTask(ctx, astTask, jsonInputs, task.NewDockerRunner(), task_dir, structDefs)`
5. Marshal output values to JSON and write to `{task_dir}/outputs.json`
6. Call `POST /tasks/{id}/result` with state=COMPLETED or FAILED

**WDL Path Lookup — Critical Design Choice:**
The server stores `wdl_path` in the `runs` table but does NOT include it in `TaskClaimResponse`.
The daemon must retrieve it. Options:
- Option A: Add `wdl_path` to `TaskClaimResponse` (requires adding field to `api.TaskClaimResponse` and populating it in `handleClaim`)
- Option B: Daemon calls `GET /workflows/{run_id}/status` — but that only returns run state, not WDL path
- Option C: Add `GET /workflows/{run_id}` endpoint returning full run metadata

**Recommendation: Extend `TaskClaimResponse` to include `wdl_path`.**
This is the simplest change: add `WDLPath string \`json:"wdl_path"\`` to `api.TaskClaimResponse`
and populate it in `handleClaim` via a `store.GetRun()` call. The store already has `GetRun()`.
This avoids adding new HTTP endpoints.

### Pattern 4: Output Serialization
**What:** Convert `map[string]values.Value` from RunTask to JSON for outputs.json
**When to use:** After successful task execution

The server's `loadTaskOutputs` in `dispatch.go` reads outputs.json and converts via `jsonToValue`.
The daemon must write the inverse: convert `values.Value` to JSON-compatible Go types.

```go
// Source: mirrors dispatch.go jsonToValue in reverse
func valuesToJSON(outputs map[string]values.Value) (map[string]interface{}, error) {
    result := make(map[string]interface{}, len(outputs))
    for k, v := range outputs {
        raw, err := valueToRaw(v)
        if err != nil {
            return nil, fmt.Errorf("output %q: %w", k, err)
        }
        result[k] = raw
    }
    return result, nil
}

func valueToRaw(v values.Value) (interface{}, error) {
    switch val := v.(type) {
    case values.StringValue:
        return val.Value, nil
    case values.IntValue:
        return val.Value, nil
    case values.FloatValue:
        return val.Value, nil
    case values.BooleanValue:
        return val.Value, nil
    case values.FileValue:
        return val.Value, nil  // absolute host path
    case values.DirectoryValue:
        return val.Value, nil  // absolute host path
    case values.NullValue:
        return nil, nil
    case values.ArrayValue:
        items := make([]interface{}, len(val.Items))
        for i, item := range val.Items {
            raw, err := valueToRaw(item)
            if err != nil {
                return nil, err
            }
            items[i] = raw
        }
        return items, nil
    default:
        return fmt.Sprintf("%v", v), nil
    }
}
```

### Pattern 5: Heartbeat Goroutine
**What:** Background goroutine sends POST /heartbeat every HeartbeatSec seconds
**When to use:** Runs for the daemon's entire lifetime, not just during task execution

```go
// Source: mirrors server.go StartStaleDetection pattern
func (d *Daemon) startHeartbeat(ctx context.Context) {
    interval := time.Duration(d.cfg.Daemon.HeartbeatSec) * time.Second
    go func() {
        ticker := time.NewTicker(interval)
        defer ticker.Stop()
        for {
            select {
            case <-ctx.Done():
                return
            case <-d.stop:
                return
            case <-ticker.C:
                if err := d.sendHeartbeat(ctx); err != nil {
                    log.Printf("daemon: heartbeat: %v", err)
                }
            }
        }
    }()
}
```

### Pattern 6: Graceful Shutdown (SIGTERM)
**What:** On SIGTERM, stop accepting new tasks, finish in-flight task, then exit
**When to use:** Required by DMN-04; 30-second deadline

```go
// Source: mirrors cmd/flowy-server/main.go signal handling pattern
func main() {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    d := daemon.New(cfg)

    go func() {
        if err := d.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
            log.Printf("daemon: run error: %v", err)
        }
    }()

    <-ctx.Done()
    stop()
    log.Printf("flowy-daemon: shutting down...")

    shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    d.Shutdown() // waits for in-flight task to complete
    log.Printf("flowy-daemon: stopped")
}
```

### Pattern 7: Task State Transition (CLAIMED → RUNNING)
The server currently does not expose an endpoint for CLAIMED → RUNNING transition.
Looking at the store interface: `UpdateTaskState` can transition CLAIMED → RUNNING.
But the daemon only has `POST /tasks/{id}/result` (terminal state) and `POST /heartbeat`.

**Recommendation:** The daemon does NOT need to report RUNNING state separately.
The CLAIMED state is sufficient for the server's `waitForTask` poll. The server polls
every 2 seconds and the daemon will report COMPLETED/FAILED when done. The heartbeat
keeps the task from going stale. This avoids adding a new RUNNING endpoint in Phase 11.

### Anti-Patterns to Avoid
- **Calling `filepath.Abs()` on paths from inputs.json:** The server's `makeDispatchFn` writes absolute paths. `task.inputs.go` `cleanPath()` calls `filepath.Abs()` for relative paths only — absolute paths pass through cleanly. No issue with shared-FS absolute paths.
- **Creating a new Docker client per poll cycle:** The DockerRunner already creates a new client per `Run()` call. This is the existing pattern; do not pre-create a persistent Docker client.
- **Running multiple tasks concurrently per daemon:** For Phase 11, one task at a time per daemon instance is correct. The server creates one task row per task call; concurrent tasks are handled by running multiple daemon instances.
- **Not draining the context on SIGTERM before Docker stop:** If context is cancelled mid-container, DockerRunner calls `cleanup()` (force remove). This is the correct behavior — orphaned containers are prevented.
- **Hard-coding daemon_id as hostname:** Use uuid.New().String() per instance startup. This avoids collisions when two daemons run on the same host.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Docker container execution | Custom Docker API calls | `wdl/task.RunTask` + `DockerRunner` | Full pipeline: PathMap, BindInputs, command gen, logs, output collection — all tested |
| HTTP retry logic | Retry loop with backoff | Simple poll ticker | Phase 11 does not require retry; ticker naturally provides backoff |
| WDL parsing in daemon | Custom parser | `wdl/parser.Parse(wdlPath, nil)` | Existing parser handles full WDL 1.2; daemon reuses it |
| Input JSON → WDL values | Custom conversion | `task.BindInputs` (called by RunTask) | RunTask takes `map[string]interface{}` directly from JSON.Unmarshal |
| Output path collection | Custom path walker | `task.CollectOutputs` (called by RunTask) | CollectOutputs uses PathMap.UntranslateValue — handles container→host path remapping |
| Signal handling | Custom signal channels | `signal.NotifyContext` | Same pattern as flowy-server; stdlib solution is correct and tested |
| Daemon ID generation | hostname+pid | `uuid.New().String()` | Already in go.mod; stable across restarts if needed |

**Key insight:** The `task.RunTask` function is the complete execution pipeline. The daemon's
job is to be the HTTP client that moves tasks between states — not to re-implement Docker logic.

## Common Pitfalls

### Pitfall 1: WDL Path Not Available in TaskClaimResponse
**What goes wrong:** Daemon claims a task but cannot find the WDL file to parse.
**Why it happens:** `api.TaskClaimResponse` does not include `wdl_path`. The field is in the
`runs` table but not passed through to the daemon.
**How to avoid:** Extend `TaskClaimResponse` with `WDLPath string \`json:"wdl_path"\`` and
populate it in `handleClaim` by calling `s.store.GetRun(r.Context(), task.RunID)`.
**Warning signs:** Test fails with "task not found in document" when daemon tries to run task.

### Pitfall 2: Task Directory Path Mismatch
**What goes wrong:** Daemon writes outputs.json to wrong path; server's `loadTaskOutputs` cannot find it.
**Why it happens:** `handleClaim` sets `InputsDir == OutputsDir == TaskDir(baseDir, runID, taskName, shardIndex)`.
The daemon must write `{OutputsDir}/outputs.json` exactly. Using `{OutputsDir}/outputs/outputs.json` or any
subdirectory breaks the path the server reads.
**How to avoid:** Write to `filepath.Join(claimResp.OutputsDir, "outputs.json")` directly.
**Warning signs:** Server logs "read task outputs.json ... no such file or directory".

### Pitfall 3: filepath.Abs() Called on Shared-FS Paths
**What goes wrong:** `task/inputs.go` `cleanPath()` calls `filepath.Abs()` for non-absolute paths.
On shared filesystems (NFS), the daemon's CWD may differ from the server's CWD.
**Why it happens:** If inputs.json contains relative paths (it shouldn't), cleanPath() would
resolve them relative to the daemon's CWD, not the server's.
**How to avoid:** The server's `makeDispatchFn` writes absolute paths to inputs.json. This is
guaranteed because `store.TaskDir()` builds absolute paths from `cfg.BaseDir`. Verify in test
that BaseDir is absolute in config.
**Warning signs:** File-not-found errors in container; paths like `./flowy-data/...` in inputs.json.

### Pitfall 4: Orphaned Docker Containers on Daemon Crash
**What goes wrong:** Daemon killed hard (SIGKILL, OOM) leaves containers running forever.
**Why it happens:** `DockerRunner` uses `AutoRemove: false` to collect logs. If the daemon dies
before the container exits, no cleanup call is made.
**How to avoid:** Phase 11 requirement is SIGTERM only (30s deadline) — `signal.NotifyContext`
cancels the context, DockerRunner calls `cleanup()` (force remove). Document SIGKILL as
out-of-scope for Phase 11; stale detection handles it at the server level.
**Warning signs:** `docker ps` shows stopped containers accumulating.

### Pitfall 5: Heartbeat Not Updating Before Task State Change
**What goes wrong:** Daemon starts executing task, heartbeat ticker fires, server detects stale
and resets task to PENDING, second daemon claims same task — double execution.
**Why it happens:** HeartbeatSec default is 30s; HeartbeatTimeoutSec server default is 60s.
If task execution takes > 60s without heartbeat, stale detection fires.
**How to avoid:** Heartbeat goroutine must start BEFORE Docker execution begins, not after.
Send an immediate heartbeat when a task is claimed, then continue at ticker interval.
**Warning signs:** Tasks executed twice; "task X claimed by daemon B while daemon A still running".

### Pitfall 6: Context Cancellation During Docker Wait
**What goes wrong:** SIGTERM arrives during `runner.Run(ctx, req)`. The context is cancelled.
`DockerRunner` calls `cleanup()` (force remove). But SIGTERM handler expects daemon to complete
in-flight task — force-removing the container is a hard stop, not clean completion.
**Why it happens:** Signal handling cancels ctx immediately.
**How to avoid:** Use a separate context for the poll loop vs. the task execution:
- Poll loop: use `ctx` from signal.NotifyContext (cancels on SIGTERM)
- Task execution: use a separate `taskCtx` that is NOT cancelled by SIGTERM
- On SIGTERM: stop poll loop via ctx cancellation, but let current task finish with taskCtx
- `Daemon.Shutdown()` waits for taskCtx goroutine to complete (via WaitGroup)
**Warning signs:** Tasks show FAILED in server after SIGTERM even though container could have completed.

### Pitfall 7: Task Execution ExecRoot Path
**What goes wrong:** `task.RunTask` creates a temp dir under `execRoot` for each invocation.
If execRoot is the task dir itself, the temp dir collides with the outputs.json the server expects.
**Why it happens:** `RunTask` calls `os.MkdirTemp(execRoot, task.Name+"-")` — creates a random
subdir for each run. This is fine. The execRoot for the daemon should be the task directory.
The work output files (stdout.txt, stderr.txt, rc) go inside the temp dir, not the task dir root.
**How to avoid:** Pass `claimResp.OutputsDir` as `execRoot` to `RunTask`. The MkdirTemp call
creates `{task_dir}/{taskName}-XXXXX/` as the execution directory. The daemon then writes
`outputs.json` directly to `{task_dir}/outputs.json` (not inside the temp dir).
**Warning signs:** outputs.json missing; server cannot read task outputs.

## Code Examples

Verified patterns from codebase walkthrough:

### HTTP Client: Claim Task
```go
// Source: internal/api/types.go TaskClaimRequest/TaskClaimResponse
func (d *Daemon) claimTask(ctx context.Context) (*api.TaskClaimResponse, error) {
    body, _ := json.Marshal(api.TaskClaimRequest{DaemonID: d.daemonID})
    req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
        d.cfg.Daemon.ServerURL+"/tasks/claim", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")

    resp, err := d.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("claim task: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusNoContent {
        return nil, nil // no pending tasks
    }
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("claim task: server returned %d", resp.StatusCode)
    }

    var claim api.TaskClaimResponse
    if err := json.NewDecoder(resp.Body).Decode(&claim); err != nil {
        return nil, fmt.Errorf("claim task decode: %w", err)
    }
    return &claim, nil
}
```

### Execute Claimed Task
```go
// Source: wdl/task/task.go RunTask signature
func (d *Daemon) executeTask(ctx context.Context, claim *api.TaskClaimResponse) {
    defer d.wg.Done()

    // Read inputs.json from task dir
    inputsPath := filepath.Join(claim.InputsDir, "inputs.json")
    data, err := os.ReadFile(inputsPath)
    if err != nil {
        d.reportResult(context.Background(), claim.TaskID, "FAILED", err.Error())
        return
    }
    var jsonInputs map[string]interface{}
    json.Unmarshal(data, &jsonInputs)

    // Parse WDL (claim.WDLPath added in Phase 11)
    doc, err := parser.Parse(claim.WDLPath, nil)
    if err != nil {
        d.reportResult(context.Background(), claim.TaskID, "FAILED", err.Error())
        return
    }

    // Find the task definition
    var astTask *ast.Task
    for _, t := range doc.Tasks {
        if t.Name == claim.TaskName {
            astTask = t
            break
        }
    }

    // Execute via existing RunTask pipeline
    runner := task.NewDockerRunner()
    outputs, err := task.RunTask(ctx, astTask, jsonInputs, runner, claim.OutputsDir)
    if err != nil {
        d.reportResult(context.Background(), claim.TaskID, "FAILED", err.Error())
        return
    }

    // Write outputs.json for server's loadTaskOutputs
    rawOutputs, _ := valuesToJSON(outputs)
    data, _ = json.Marshal(rawOutputs)
    os.WriteFile(filepath.Join(claim.OutputsDir, "outputs.json"), data, 0o644)

    d.reportResult(context.Background(), claim.TaskID, "COMPLETED", "")
}
```

### Send Heartbeat
```go
// Source: internal/api/types.go HeartbeatRequest
func (d *Daemon) sendHeartbeat(ctx context.Context) error {
    body, _ := json.Marshal(api.HeartbeatRequest{DaemonID: d.daemonID})
    req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
        d.cfg.Daemon.ServerURL+"/heartbeat", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    resp, err := d.client.Do(req)
    if err != nil {
        return fmt.Errorf("heartbeat: %w", err)
    }
    resp.Body.Close()
    return nil
}
```

### Report Task Result
```go
// Source: internal/api/types.go TaskResultRequest
func (d *Daemon) reportResult(ctx context.Context, taskID, state, errMsg string) {
    body, _ := json.Marshal(api.TaskResultRequest{State: state, Error: errMsg})
    req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
        d.cfg.Daemon.ServerURL+"/tasks/"+taskID+"/result", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    resp, err := d.client.Do(req)
    if err != nil {
        log.Printf("daemon: report result: %v", err)
        return
    }
    resp.Body.Close()
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Long-poll for task claiming | Short poll (5s ticker) | Phase 11 decision | Long-poll (`http.Flusher`) adds goroutine-per-connection pressure; short poll is simpler and sufficient for batch workflows |
| Server-push task dispatch | Daemon-poll model | Architecture decision | Daemon polls avoids server maintaining per-daemon connections; scales to N daemons without server-side state |

**Deprecated/outdated:**
- Long-poll `PollTimeoutSec` in `DaemonConfig` is defined in config but Phase 11 uses short poll; the field can be used for a future improvement or kept as a documented option

## Open Questions

1. **WDL Path in TaskClaimResponse**
   - What we know: `handleClaim` does not currently return `wdl_path`; daemon needs it
   - What's unclear: Whether Phase 11 should modify `api.TaskClaimResponse` (a shared leaf package)
   - Recommendation: Add `WDLPath string \`json:"wdl_path"\`` to `api.TaskClaimResponse`;
     populate in `handleClaim` via `s.store.GetRun(r.Context(), task.RunID)`. This is a small,
     backward-compatible change. The planner should include this as Task 0 or part of server-side work.

2. **Struct Definitions for RunTask**
   - What we know: `task.RunTask` accepts variadic `structDefs ...map[string]*ast.StructDef`
   - What's unclear: The daemon must pass struct defs from the parsed WDL document for struct-typed inputs
   - Recommendation: Build struct defs from `doc.Structs` before calling `RunTask`, same as
     `workflow/scheduler.go structDefs()` method.

3. **Task Execution Concurrency**
   - What we know: Phase 11 success criteria do not mention concurrent tasks per daemon
   - What's unclear: Whether a daemon should execute multiple tasks concurrently
   - Recommendation: Single task at a time per daemon instance. Multiple daemons provide
     parallelism. Keeps implementation simple and avoids goroutine-per-task complexity.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) |
| Config file | none (uses `go test ./...`) |
| Quick run command | `go test ./internal/daemon/... -count=1 -timeout 30s` |
| Full suite command | `go test ./... -count=1 -timeout 120s` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DMN-01 | Two daemons poll simultaneously; exactly one claims task | unit | `go test ./internal/daemon/... -run TestConcurrentClaim -count=1` | Wave 0 |
| DMN-02 | Claimed task executes via DockerRunner; outputs written to task dir | integration | `go test ./internal/daemon/... -run TestExecuteTask -count=1` | Wave 0 |
| DMN-03 | Heartbeat sent every 30s for running tasks | unit | `go test ./internal/daemon/... -run TestHeartbeat -count=1` | Wave 0 |
| DMN-04 | SIGTERM → complete in-flight task → shutdown within 30s | integration | `go test ./internal/daemon/... -run TestGracefulShutdown -count=1` | Wave 0 |
| DMN-05 | Task outputs written to `{task_dir}/outputs.json` | unit | `go test ./internal/daemon/... -run TestOutputsJSON -count=1` | Wave 0 |
| DMN-06 | Binary ships as `cmd/flowy-daemon/main.go` | smoke | `go build ./cmd/flowy-daemon/` | Wave 0 |

**Note on DMN-01:** Concurrent-claim atomicity is already tested at the store level
(`internal/store/sqlite/sqlite_test.go`). The daemon test should verify the daemon
correctly handles HTTP 204 (no task available) without error.

**Note on DMN-04:** Full graceful shutdown with real Docker is an integration test.
Use `httptest.NewServer` to mock the server side; mock the Docker call via a fake `ContainerRunner`.

### Sampling Rate
- **Per task commit:** `go test ./internal/daemon/... -count=1 -timeout 30s`
- **Per wave merge:** `go test ./... -count=1 -timeout 120s`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/daemon/daemon_test.go` — covers DMN-01 through DMN-05
- [ ] `cmd/flowy-daemon/main.go` — covers DMN-06 (build smoke test)
- [ ] `api.TaskClaimResponse.WDLPath` field addition (modifies existing `internal/api/types.go`)
- [ ] `handleClaim` update to populate `WDLPath` (modifies existing `internal/server/handlers.go`)

## Sources

### Primary (HIGH confidence)
- `/home/uehara/flowy/internal/server/handlers.go` — server endpoint contracts (handleClaim, handleHeartbeat, handleResult)
- `/home/uehara/flowy/internal/server/dispatch.go` — dispatch pattern: inputs.json write, outputs.json read, waitForTask
- `/home/uehara/flowy/wdl/task/task.go` — RunTask pipeline (14 steps); execRoot usage
- `/home/uehara/flowy/wdl/task/inputs.go` — BindInputs and cleanPath (filepath.Abs behavior)
- `/home/uehara/flowy/wdl/task/pathmap.go` — PathMap.AddFile calls filepath.Abs on relative paths
- `/home/uehara/flowy/internal/api/types.go` — all DTO types; TaskClaimResponse missing WDLPath
- `/home/uehara/flowy/internal/store/store.go` — Store interface; ClaimTask atomicity guarantee
- `/home/uehara/flowy/internal/config/config.go` — DaemonConfig fields (ServerURL, HeartbeatSec, PollTimeoutSec)
- `/home/uehara/flowy/cmd/flowy-server/main.go` — binary entrypoint pattern to replicate
- `/home/uehara/flowy/.planning/STATE.md` — accumulated decisions (daemon polls, heartbeat=30s)

### Secondary (MEDIUM confidence)
- Go stdlib documentation for `signal.NotifyContext` — SIGTERM handling pattern
- Go stdlib `net/http` client patterns — standard HTTP client for daemon API calls

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all dependencies are already in go.mod; no new libraries needed
- Architecture: HIGH — all server endpoints, store interface, and task execution pipeline verified by direct code reading
- Pitfalls: HIGH — identified by tracing actual code paths (dispatch.go, task.go, pathmap.go, inputs.go)

**Research date:** 2026-03-12
**Valid until:** 2026-04-12 (stable codebase; only expires if server API changes)

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| DMN-01 | Daemon polls server for available tasks and claims them atomically | `ClaimTask` uses atomic UPDATE+RETURNING; HTTP 204 on no task; daemon poll loop handles both cases |
| DMN-02 | Daemon executes claimed tasks via existing Docker engine (task.RunTask) | `task.RunTask` + `DockerRunner` are the complete pipeline; daemon passes `jsonInputs` from inputs.json |
| DMN-03 | Daemon sends periodic heartbeat to server for running tasks | `POST /heartbeat` with DaemonID; `UpdateHeartbeatByDaemon` updates all CLAIMED/RUNNING tasks in one UPDATE |
| DMN-04 | Daemon handles SIGTERM gracefully, completing in-flight tasks before shutdown | Separate poll ctx vs. task ctx; WaitGroup tracks in-flight task; 30s shutdown deadline |
| DMN-05 | Daemon writes task outputs to shared filesystem convention path | `{task_dir}/outputs.json` (outputs_dir from claim response); server's `loadTaskOutputs` reads this exact path |
| DMN-06 | Daemon binary ships as `flowy-daemon` (`cmd/flowy-daemon/`) | Pattern: mirror `cmd/flowy-server/main.go`; no new dependencies |
</phase_requirements>
