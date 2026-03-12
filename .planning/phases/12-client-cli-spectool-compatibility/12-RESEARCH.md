# Phase 12: Client CLI + Spectool Compatibility - Research

**Researched:** 2026-03-12
**Domain:** Go HTTP client, CLI refactoring, file staging to shared filesystem, polling patterns
**Confidence:** HIGH

## Summary

Phase 12 transforms `cmd/flowy/` from a local-execution binary into a distributed-mode client that talks to `flowy-server` over HTTP. The binary keeps its existing local commands (`check`, `run --task`, etc.) and adds three new server-facing subcommands: `submit`, `status`, and `outputs`. The existing `run` command is retargeted to work synchronously over the server (submit → poll → print outputs), requiring a running server; no local fallback is provided.

The key challenge is file staging: when users pass local file paths as WDL inputs (e.g., a reference genome FASTA), those files must be copied into the shared filesystem under `{baseDir}/runs/{runID}/inputs/` before submission, and the `wdl_path` sent to the server must be an absolute path visible to all daemons on the NFS mount. The server already reads `SubmitRequest.WDLPath` as an absolute path and parses it directly on the server filesystem.

The polling pattern for `flowy run` is straightforward: submit, then poll `GET /workflows/{id}/status` in a loop with a configurable interval until a terminal state is reached (`COMPLETED`, `FAILED`, or `ABORTED`). On `COMPLETED`, fetch outputs via `GET /workflows/{id}/outputs` and print as JSON to stdout. On `FAILED`/`ABORTED`, print an error with the run ID to stderr and exit with code 1.

**Primary recommendation:** Add a new `internal/client/` package implementing `Client` struct with methods `Submit`, `Status`, `Outputs`. Wire three new subcommands (`submit`, `status`, `outputs`) into the existing `run()` dispatcher in `cmd/flowy/main.go`. Reuse the existing `runRun()` function structure for the new `runSubmit()` / `runStatus()` / `runOutputs()` functions. Keep the existing `check` subcommand unchanged.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CLI-01 | `flowy submit <wdl> -i <inputs>` copies local input files to shared filesystem, submits to server, prints run ID | `SubmitRequest{WDLPath, InputsJSON}` is the server contract; file staging writes to `store.RunInputsDir(baseDir, runID)`; the run ID printed from `SubmitResponse.RunID` |
| CLI-02 | `flowy status <run-id>` prints current workflow state | `GET /workflows/{id}/status` returns `StatusResponse{RunID, State}`; prefix matching via `GetRunByPrefix` is server-side but the client can just pass the prefix directly |
| CLI-03 | `flowy outputs <run-id>` prints output key-value pairs for a completed run | `GET /workflows/{id}/outputs` returns `OutputsResponse{RunID, Outputs}`; non-completed runs return HTTP 409 Conflict |
| CLI-04 | `flowy run <wdl> -i <inputs>` works synchronously (submit → poll → print outputs) | Polling loop over `GET /workflows/{id}/status` until terminal state; outputs fetched via `GET /workflows/{id}/outputs` on COMPLETED |
| CLI-05 | Client copies local input files to shared filesystem before submitting | `os.Copy` each file from inputs JSON that has a local path into `{baseDir}/runs/{runID}/inputs/`; update `inputs` map with new absolute paths |
| CLI-06 | `flowy` client binary ships under `cmd/flowy/`; all three binaries build via `go build ./...` | Existing `cmd/flowy/main.go` is extended in place; no new module or separate directory needed |
</phase_requirements>

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `net/http` (stdlib) | go1.25 | HTTP client for server API calls | Already used in `internal/daemon/daemon.go` for same purpose |
| `encoding/json` (stdlib) | go1.25 | Encode requests / decode responses | Already used throughout |
| `flag` (stdlib) | go1.25 | Subcommand flag parsing | Established in codebase; no cobra (see Phase 7 decision) |
| `os` (stdlib) | go1.25 | File copying, path ops | Standard |
| `path/filepath` (stdlib) | go1.25 | Absolute path resolution | Standard |
| `time` (stdlib) | go1.25 | Polling sleep interval | Standard |
| `internal/api` | project | `SubmitRequest`, `SubmitResponse`, `StatusResponse`, `OutputsResponse` DTO types | Already defined; client just decodes what server encodes |
| `internal/config` | project | `ClientConfig{ServerURL}`, `Config{BaseDir}` | Already defined; `cfg.Client.ServerURL` and `cfg.BaseDir` |

### No New External Dependencies

The client requires zero new external packages. All needed types, patterns, and infrastructure already exist in the project.

**Installation:**
```bash
go build ./...
```

## Architecture Patterns

### Recommended Project Structure

```
cmd/flowy/
├── main.go          # extend: add submit/status/outputs/run-server subcommands
├── main_test.go     # existing check tests (unchanged)
├── run_test.go      # existing local-run tests (unchanged)
├── client_test.go   # new: submit/status/outputs/run tests using httptest.NewServer
└── testdata/
internal/client/
├── client.go        # new: Client struct, Submit/Status/Outputs methods
└── client_test.go   # new: unit tests for HTTP client methods
```

The `internal/client/` package follows the exact same pattern as `internal/daemon/daemon.go`:
- Holds `*http.Client`, `serverURL string`
- Methods encode request bodies, POST/GET, decode responses
- Returns typed responses or errors

### Pattern 1: Client Package Structure

**What:** `Client` struct mirrors the daemon pattern for HTTP calls.
**When to use:** Always — single package for all server API calls from CLI.

```go
// internal/client/client.go
package client

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "github.com/bioflowy/flowy/internal/api"
)

type Client struct {
    serverURL  string
    httpClient *http.Client
}

func New(serverURL string) *Client {
    return &Client{
        serverURL:  serverURL,
        httpClient: &http.Client{Timeout: 30 * time.Second},
    }
}

func (c *Client) Submit(ctx context.Context, req api.SubmitRequest) (*api.SubmitResponse, error) {
    // POST /workflows
}

func (c *Client) Status(ctx context.Context, runID string) (*api.StatusResponse, error) {
    // GET /workflows/{runID}/status
}

func (c *Client) Outputs(ctx context.Context, runID string) (*api.OutputsResponse, error) {
    // GET /workflows/{runID}/outputs
}
```

### Pattern 2: Subcommand Wiring in main.go

**What:** Three new cases in the `switch args[0]` dispatcher. Each case calls a `runXxx()` function.
**When to use:** Follow the existing pattern exactly — `run`, `check` are already there.

```go
// cmd/flowy/main.go — extend the switch in run()
switch args[0] {
case "--version":
    // unchanged
case "check":
    return runCheck(args[1:], stdout, stderr)
case "run":
    return runRun(args[1:], stdout, stderr)      // existing local run — see note below
case "submit":
    return runSubmit(args[1:], stdout, stderr)
case "status":
    return runStatus(args[1:], stdout, stderr)
case "outputs":
    return runOutputs(args[1:], stdout, stderr)
default:
    // unchanged
}
```

**Critical decision on `run`:** The existing `runRun()` does local Docker execution. Per STATE.md: "`flowy run` maintains synchronous behavior for spectool compatibility; server is required (no local fallback)". This means the existing `runRun()` must be **replaced** with a server-based implementation. The local execution path (`workflow.RunWorkflow`, `task.RunTask`) is removed from the `run` subcommand and replaced with submit+poll behavior.

The `check` subcommand remains unchanged (local parse + type-check, no server).

### Pattern 3: Config Loading in CLI Binary

**What:** The `flowy` client binary needs `cfg.Client.ServerURL` and `cfg.BaseDir` for file staging. Config loading follows the same pattern as `flowy-daemon/main.go`.

**Decision from STATE.md (Phase 09 CONTEXT.md):** "Client (`flowy`) works without config — defaults to connecting `localhost:8124`". This means config loading is **optional** for the client — if `--config` flag is not given and `./flowy.yaml` does not exist, use hardcoded defaults.

```go
// cmd/flowy/main.go — loadConfigOptional helper
func loadConfigOptional(cfgPath string) *config.Config {
    if cfgPath != "" {
        cfg, err := config.Load(cfgPath)
        if err != nil {
            // Flag was explicitly given; failure is an error.
            return nil, err
        }
        return cfg, nil
    }
    // Try default path; missing is OK for client.
    cfg, err := config.Load("./flowy.yaml")
    if err != nil {
        // File not found: use defaults.
        var defaults config.Config
        // applyDefaults is unexported; use Load with a synthetic default or expose it.
        return &config.Config{
            BaseDir: "./flowy-data",
            Client:  config.ClientConfig{ServerURL: "http://localhost:8124"},
        }, nil
    }
    return cfg, nil
}
```

**Note:** `config.applyDefaults()` is currently unexported. The cleanest approach is to call `config.Load()` with the path, and if it returns `os.IsNotExist` error, construct a `Config{}` with zero values and rely on the fact that all three binaries would apply defaults if `Load` were called on a nonexistent file. Alternative: export `ApplyDefaults` or add a `Default()` constructor to the config package. The planner should choose the minimal approach (add `config.Default()` returning a zero-defaults config).

### Pattern 4: File Staging (CLI-05)

**What:** Before submitting, copy all local file-valued inputs to the shared filesystem.
**When to use:** Always — the server and daemons cannot read files from the client's local disk.

The challenge is identifying which JSON input values are file paths. The WDL inputs JSON is a `map[string]interface{}` — string values that look like absolute paths and exist as files on the client filesystem should be copied.

**Approach (pragmatic):** After reading `inputs.json`, iterate over the values. For any string value that is an absolute path to an existing local file, copy it to `{baseDir}/runs/{runID}/inputs/` and replace the value with the shared FS absolute path. This requires the client to know the run ID before copying — but the server assigns the run ID.

**Order of operations:**
1. Read WDL file — it must already be on the shared filesystem (or the client IS on the shared filesystem). If not, copy it too.
2. Read inputs JSON.
3. Generate a temporary run staging directory (pre-submission) OR accept that the server creates the run directories. Since the server creates `{baseDir}/runs/{runID}/inputs/` in `handleSubmit`, the client cannot stage files there before getting the run ID.

**Resolution:** The client must stage files to a pre-agreed location. Two options:
- Option A: Client generates a UUID locally as a candidate run ID, stages files, then submits with a hint — server may or may not honor it.
- Option B: Client copies files to a temporary staging directory, submits, then moves files to the final run directory. Requires access to the shared FS after getting the run ID.
- Option C: Client stages files to `{baseDir}/staging/{timestamp}-{random}/inputs/` before submission, submits with file paths pointing there, server uses those paths. After run completes, staging area can be cleaned up.
- Option D: Client copies WDL file + inputs files to `{baseDir}/staging/{runID}/` using a locally generated UUID as the staging ID. Sends `wdl_path` pointing to staged location. The staging dir IS the run's shared data location.

**Recommended approach (Option D — locally generated UUID):**
1. Client generates a UUID locally (using `github.com/google/uuid` — already in go.mod).
2. Client creates `{baseDir}/runs/{uuid}/inputs/` using `store.CreateRunDirs(baseDir, uuid)`.
3. Client copies WDL file (if not already on shared FS) and input files into `{baseDir}/runs/{uuid}/inputs/`.
4. Client rewrites the `InputsJSON` map to use the staged paths.
5. Client submits with `WDLPath` = staged WDL path, `InputsJSON` = rewritten map, and includes the pre-generated UUID as a hint to the server.

**Problem with Option D:** The server generates its own UUID in `handleSubmit` via `uuid.New().String()`. Two UUIDs for one run is confusing. The server would need to accept a client-provided run ID.

**Simplest viable approach (Option E — staging then rename):**
Since the shared filesystem is accessible to both client and server (NFS mount), the client can:
1. Stage files to a temp directory on the shared FS.
2. Submit (server creates run dirs and assigns run ID).
3. The server includes the run ID in `SubmitResponse`.
4. The client then does not need to move files — instead, stage files to a location the server can reference.

**But the server needs to know WHERE the files are.** The inputs JSON sent to the server must contain paths visible on the shared FS.

**Final recommended approach (Option F — client stages before submitting, uses its own UUID as run ID hint, server honors it):**
Extend `SubmitRequest` to optionally include a `RunID string` field. If provided and non-empty, the server uses it instead of generating a new one. The client:
1. Generates UUID locally.
2. Calls `store.CreateRunDirs(baseDir, uuid)` to create the staging dirs.
3. Copies files into `{baseDir}/runs/{uuid}/inputs/`.
4. Updates inputs JSON with staged paths.
5. Sends `SubmitRequest{RunID: uuid, WDLPath: ..., InputsJSON: ...}`.
6. Server uses the provided UUID directly.

This requires a one-field addition to `api.SubmitRequest` and a one-line change in `handleSubmit`. It is clean and avoids any race condition.

**For the WDL file:** The client must provide an absolute path to the WDL file that is accessible on the shared filesystem. If the client machine IS one of the shared FS machines (HPC node), the WDL path can be the local absolute path. If not, the WDL file must also be copied to the shared FS. The implementation should copy the WDL file to `{baseDir}/runs/{runID}/inputs/workflow.wdl` if its path is not already under `baseDir`.

### Pattern 5: Polling Loop for `flowy run`

**What:** Submit workflow, then poll status until terminal state.
**When to use:** Always — `flowy run` is synchronous.

```go
// cmd/flowy/main.go — poll loop in runRun()
func pollUntilDone(ctx context.Context, c *client.Client, runID string, pollInterval time.Duration, stderr io.Writer) (string, error) {
    ticker := time.NewTicker(pollInterval)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return "", ctx.Err()
        case <-ticker.C:
            status, err := c.Status(ctx, runID)
            if err != nil {
                fmt.Fprintf(stderr, "warning: status poll failed: %v\n", err)
                continue
            }
            switch status.State {
            case "COMPLETED", "FAILED", "ABORTED":
                return status.State, nil
            }
        }
    }
}
```

**Poll interval:** Default 2 seconds. Not configurable via flag (keep simple). The poll is cheap (GET request, SQLite read). For long workflows, 2s polling adds negligible overhead.

**Server unreachable behavior (CLI-04 requirement):** If the initial `Submit` call fails to connect, the client prints a clear error message and exits with code 1. No retry on initial connection failure — fail fast with message like "error: cannot reach server at http://localhost:8124: connection refused". Subsequent poll failures during `run` log a warning but continue polling (transient network issues).

### Pattern 6: Exit Codes and Output Format

**What:** Exit code and output format conventions for each command.

| Command | Success | Failure | Stdout | Stderr |
|---------|---------|---------|--------|--------|
| `submit` | 0 | 1 | run ID (plain string + newline) | error message |
| `status` | 0 | 1 | `<state>` (e.g., `RUNNING`) | error message |
| `outputs` | 0 | 1 | JSON outputs map | error message |
| `run` | 0 (COMPLETED) | 1 (FAILED/ABORTED/conn error) | JSON outputs (on COMPLETED) | error + task name/exit code on failure |

**`flowy submit` stdout format:**
```
<run-id>
```
Plain UUID, no decoration. Allows `RUN_ID=$(flowy submit ...)` in shell scripts.

**`flowy status` stdout format:**
```
RUNNING
```
Plain state string. Allows `if [ "$(flowy status $ID)" = "COMPLETED" ]`.

**`flowy outputs` stdout format:**
Same as existing `flowy run` outputs JSON (JSON object with `wfName.outputName` keys).

**`flowy run` failure format (stderr):**
```
error: workflow FAILED (run-id: <uuid>)
```
The server does not currently store task-level error details accessible via API, so task name + exit code are not available from the outputs API. If the server's `OutputsResponse` is extended to include failure details, the client can print them. For Phase 12, print the run ID and state — the user can inspect task logs on the shared filesystem.

### Anti-Patterns to Avoid

- **Reimplementing HTTP client logic in main.go:** All server HTTP calls go through `internal/client/Client` — not inline in subcommand functions.
- **Blocking indefinitely without timeout on poll:** The poll loop must respect `context.WithTimeout` or `context.WithCancel` tied to SIGINT. Use `signal.NotifyContext` for the CLI context.
- **Sending relative WDL paths to the server:** The server calls `parser.Parse(req.WDLPath, nil)` which reads the file from its local disk. The path MUST be absolute and accessible on the server's filesystem.
- **Removing the `check` subcommand or `--task` local path:** Phase 12 only adds new subcommands and replaces `run`. `check` stays local.
- **Using cobra:** The project explicitly uses stdlib `flag` package. No cobra.
- **Polling with sleep instead of ticker:** Use `time.NewTicker` (same as daemon pattern). Allows context cancellation to interrupt immediately.
- **Using GET with a body for status/outputs:** The server uses path parameters (`/workflows/{id}/status`) — pass the run ID in the URL, not the body.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| HTTP client for server API | Custom HTTP request builder | `net/http.Client` + `internal/api` types | Daemon already uses this pattern; `api.SubmitRequest` etc. are the exact DTOs |
| UUID generation | Custom ID generator | `github.com/google/uuid` | Already in go.mod (`github.com/google/uuid v1.6.0`); `uuid.New().String()` |
| File copy | Custom byte-stream copier | `io.Copy(dst, src)` | stdlib; use `os.Create` + `os.Open` + `io.Copy` |
| Run dir layout | Custom path construction | `store.CreateRunDirs`, `store.RunInputsDir` | Already implemented and tested in `internal/store/rundir.go` |
| Config loading | Custom YAML parser | `config.Load()` + `config.Default()` | Already implemented; `applyDefaults()` logic already exists |
| JSON output format | Custom serializer | `workflow.SerializeOutputs()` | Already used in existing `runRun()` for task runs |
| Terminal state detection | Custom state string comparison | String switch on `status.State` | `store.RunCompleted`, `store.RunFailed`, `store.RunAborted` constants exist |

**Key insight:** The daemon (`internal/daemon/daemon.go`) provides the exact blueprint for HTTP client patterns. The client package is a simpler version of the daemon — no polling loop, no heartbeats, just request/response calls.

## Common Pitfalls

### Pitfall 1: WDL Path Must Be on Shared Filesystem

**What goes wrong:** User runs `flowy submit /home/user/my.wdl -i inputs.json`. The server calls `parser.Parse("/home/user/my.wdl", nil)` but that path doesn't exist on the server machine.

**Why it happens:** `handleSubmit` calls `parser.Parse(req.WDLPath, nil)` on the server side. The WDL must be accessible to the server process.

**How to avoid:** The client detects if `wdlPath` is not under `cfg.BaseDir`, copies it to `{baseDir}/runs/{runID}/inputs/workflow.wdl`, and sends the staged path as `WDLPath` in the request.

**Warning signs:** Server returns 400 Bad Request with "parse WDL: cannot read file" even though the WDL exists locally.

### Pitfall 2: `config.applyDefaults` is Unexported

**What goes wrong:** The client binary tries to construct a `config.Config{}` with defaults but `applyDefaults()` is unexported.

**Why it happens:** `applyDefaults` in `internal/config/config.go` is a private function.

**How to avoid:** Add a `config.Default() *Config` exported function that returns a config with all defaults applied. This is a one-function addition to the config package. Alternative: call `config.Load()` with a nonexistent path and check for `os.IsNotExist` — but this is fragile. The clean fix is `config.Default()`.

**Warning signs:** Compile error "undefined: config.applyDefaults" or hardcoded URLs in client code.

### Pitfall 3: Run ID Collision from Client-Generated UUID

**What goes wrong:** Client generates UUID and creates `{baseDir}/runs/{uuid}/inputs/` before submitting. If submission fails (server error), the staged directories are left behind with no run associated.

**Why it happens:** File staging must happen before submission (to have staged paths in the request), but the run ID is only assigned at submission time.

**How to avoid:** If submission fails, the client should clean up the staged directories it created. Wrap staging + submission in a deferred cleanup: `defer os.RemoveAll(stagedDir)` only on error path. On success, the staged dir becomes the canonical run directory.

**Warning signs:** Orphaned `{baseDir}/runs/{uuid}/` directories accumulating on the shared FS.

### Pitfall 4: Poll Loop Does Not Handle SIGINT

**What goes wrong:** User presses Ctrl+C during `flowy run` polling. The poll loop keeps running even though the user cancelled.

**Why it happens:** The poll loop uses `time.Sleep` or a ticker without checking for cancellation.

**How to avoid:** Use `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)` as the root context for all operations. Pass this context to `pollUntilDone`. When the context is cancelled, the polling select unblocks immediately.

**Warning signs:** `flowy run` doesn't exit when Ctrl+C is pressed.

### Pitfall 5: `flowy run` Removes Local Execution Capability

**What goes wrong:** Existing tests in `run_test.go` test the local Docker execution path via `mockRunner`. After replacing `runRun()` with server-based implementation, these tests break.

**Why it happens:** The current `runRun()` is the local-execution function. Phase 12 replaces it with an HTTP client path.

**How to avoid:** The existing test suite for `run_test.go` (tests like `TestRunWorkflow`, `TestRunTask`) must be updated or removed. The new `run` command no longer uses `newRunner` or local Docker. New tests use `httptest.NewServer` to mock the server API, following the pattern in `internal/daemon/daemon_test.go`.

**Warning signs:** Compile errors in `run_test.go` after removing `newRunner` from `runRun()`.

### Pitfall 6: `flowy status` with Run ID Prefix

**What goes wrong:** User types `flowy status abc123` (short prefix). The server endpoint `GET /workflows/{id}/status` calls `store.GetRun(ctx, runID)` which does an exact match. Prefix matching is via `GetRunByPrefix`.

**Why it happens:** The chi route uses the ID literally. If a client sends a prefix, the server returns 404.

**How to avoid:** The server's `handleStatus` currently calls `store.GetRun(ctx, runID)` — it does NOT use prefix matching. Two options:
1. Client always passes the full UUID (obtained from `flowy submit`).
2. Server handler is updated to try `GetRun` first, then fall back to `GetRunByPrefix` on 404.

For Phase 12, the server handler should be updated to support prefix matching in `handleStatus` and `handleOutputs`. This is a small change that improves UX significantly (as documented in Phase 09 CONTEXT.md: "prefix matching should behave like git").

**Warning signs:** `flowy status abc123` returns 404 even though the run exists.

### Pitfall 7: `flowy outputs` Returns HTTP 409 on Non-Completed Run

**What goes wrong:** User calls `flowy outputs <id>` on a RUNNING workflow. Server returns 409 Conflict. Client shows a confusing error.

**Why it happens:** `handleOutputs` checks `run.State != store.RunCompleted` and returns 409. This is correct server behavior.

**How to avoid:** The client's `runOutputs()` function should handle 409 specially: print "workflow not yet completed (state: RUNNING)" to stderr and exit 1. Do not treat 409 as an unexpected error.

**Warning signs:** `flowy outputs` shows "unexpected status 409" instead of a clear message.

## Code Examples

Verified patterns from official sources and existing codebase:

### HTTP Client Pattern (from internal/daemon/daemon.go)

```go
// Source: internal/daemon/daemon.go — claimTask method
// Same pattern for client Submit/Status/Outputs:
func (c *Client) Submit(ctx context.Context, req api.SubmitRequest) (*api.SubmitResponse, error) {
    body, err := json.Marshal(req)
    if err != nil {
        return nil, fmt.Errorf("marshal submit request: %w", err)
    }
    url := c.serverURL + "/workflows"
    httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
    if err != nil {
        return nil, fmt.Errorf("build submit request: %w", err)
    }
    httpReq.Header.Set("Content-Type", "application/json")
    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("POST /workflows: %w", err)
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusCreated {
        var apiErr api.ErrorResponse
        _ = json.NewDecoder(resp.Body).Decode(&apiErr)
        return nil, fmt.Errorf("server error %d: %s", resp.StatusCode, apiErr.Error)
    }
    var result api.SubmitResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("decode submit response: %w", err)
    }
    return &result, nil
}
```

### Status GET Pattern

```go
// Source: internal/api/types.go — StatusResponse
func (c *Client) Status(ctx context.Context, runID string) (*api.StatusResponse, error) {
    url := c.serverURL + "/workflows/" + runID + "/status"
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil {
        return nil, fmt.Errorf("build status request: %w", err)
    }
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("GET /workflows/%s/status: %w", runID, err)
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        var apiErr api.ErrorResponse
        _ = json.NewDecoder(resp.Body).Decode(&apiErr)
        return nil, fmt.Errorf("server error %d: %s", resp.StatusCode, apiErr.Error)
    }
    var result api.StatusResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("decode status response: %w", err)
    }
    return &result, nil
}
```

### File Staging

```go
// Source: internal/store/rundir.go — RunInputsDir, CreateRunDirs
// Client-side staging:
func stageFile(srcPath, destDir string) (string, error) {
    filename := filepath.Base(srcPath)
    destPath := filepath.Join(destDir, filename)
    src, err := os.Open(srcPath)
    if err != nil {
        return "", fmt.Errorf("open source file %q: %w", srcPath, err)
    }
    defer src.Close()
    dst, err := os.Create(destPath)
    if err != nil {
        return "", fmt.Errorf("create staged file %q: %w", destPath, err)
    }
    defer dst.Close()
    if _, err := io.Copy(dst, src); err != nil {
        return "", fmt.Errorf("copy to %q: %w", destPath, err)
    }
    return destPath, nil
}
```

### Polling Loop

```go
// Pattern from daemon workerLoop but adapted for client-side polling:
func pollUntilDone(ctx context.Context, c *client.Client, runID string, stderr io.Writer) (string, error) {
    ticker := time.NewTicker(2 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return "", ctx.Err()
        case <-ticker.C:
            status, err := c.Status(ctx, runID)
            if err != nil {
                fmt.Fprintf(stderr, "warning: status poll error: %v\n", err)
                continue
            }
            switch status.State {
            case "COMPLETED", "FAILED", "ABORTED":
                return status.State, nil
            default:
                fmt.Fprintf(stderr, "run %s: %s\n", runID[:8], status.State)
            }
        }
    }
}
```

### Config Default Construction

```go
// Add to internal/config/config.go:
// Default returns a *Config with all default values applied.
// Useful when no config file is present (e.g., client-only usage).
func Default() *Config {
    cfg := &Config{}
    applyDefaults(cfg)
    return cfg
}
```

### httptest Server in Tests (from daemon_test.go pattern)

```go
// cmd/flowy/client_test.go — mock server pattern:
func TestRunSubmit(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method == http.MethodPost && r.URL.Path == "/workflows" {
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusCreated)
            json.NewEncoder(w).Encode(api.SubmitResponse{RunID: "test-run-id"})
        }
    }))
    defer srv.Close()
    // Set server URL in env or via test config
    // Call runSubmit() with --server flag pointing to srv.URL
}
```

### Server-Side Change: SubmitRequest RunID Hint

```go
// internal/api/types.go — extend SubmitRequest:
type SubmitRequest struct {
    RunID      string                 `json:"run_id,omitempty"` // optional client-provided ID
    WDLPath    string                 `json:"wdl_path"`
    InputsJSON map[string]interface{} `json:"inputs"`
}

// internal/server/handlers.go — handleSubmit: use provided RunID if non-empty:
runID := req.RunID
if runID == "" {
    runID = uuid.New().String()
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `flowy run` executes locally via Docker | `flowy run` submits to server and polls | Phase 12 | Server required; no local fallback |
| No HTTP client in flowy binary | `internal/client/` package for API calls | Phase 12 | Enables submit/status/outputs subcommands |
| `flowy` binary only does local work | `flowy` binary is a full distributed client | Phase 12 | Completes the three-binary architecture |
| Run ID is assigned only by server | Client can hint a pre-generated run ID | Phase 12 | Enables pre-submission file staging |

**Deprecated/outdated after Phase 12:**
- Local `runRun()` implementation using `workflow.RunWorkflow` and `task.RunTask` — replaced by server-based implementation.
- `newRunner` package variable in `cmd/flowy/main.go` — no longer needed when `run` is server-based.
- Existing `run_test.go` tests using `mockRunner` — must be replaced with `httptest` mock server tests.

## Open Questions

1. **`--server` flag vs. config-only server URL**
   - What we know: `cfg.Client.ServerURL` defaults to `http://localhost:8124`. The flag `--config` loads the whole file.
   - What's unclear: Should there be a `--server <url>` flag to override without a config file?
   - Recommendation: Add a `--server <url>` flag to `submit`, `status`, `outputs`, and `run` subcommands. If not given, use `cfg.Client.ServerURL`. This allows `flowy submit --server http://myserver:8124 ...` without a config file. Confidence: HIGH this is the right UX.

2. **Prefix matching in server handlers**
   - What we know: `handleStatus` and `handleOutputs` use `store.GetRun()` (exact match). `GetRunByPrefix()` exists in the store interface.
   - What's unclear: Should prefix matching be in the server handler or client-side?
   - Recommendation: Add prefix matching to `handleStatus` and `handleOutputs` server handlers — try `GetRun` first, then `GetRunByPrefix` on error. This is the correct layer (server has access to the store). The client just passes whatever the user typed. Confidence: HIGH.

3. **How to detect if an input JSON value is a file path**
   - What we know: WDL inputs JSON can have string values that are file paths or plain strings. There is no type annotation in the raw JSON.
   - What's unclear: How to distinguish `"reference": "/data/hg38.fa"` (a file path) from `"sample_name": "/lab/sample42"` (a plain string that looks like a path).
   - Recommendation: Stage any string value that (a) starts with `/`, (b) is an existing file on the local filesystem, AND (c) is NOT already under `cfg.BaseDir`. This is a heuristic. False positives (staging a string that happens to look like a path) are safe (extra copy). False negatives (not staging a needed file) cause server failures. Accept the heuristic. Confidence: MEDIUM.

4. **WDL file staging vs. reference by path**
   - What we know: On HPC clusters, the client machine IS one of the NFS machines. The WDL file is on the shared filesystem already.
   - What's unclear: Should the client always copy the WDL to `{baseDir}/runs/{id}/inputs/workflow.wdl` or only when it's not already under `baseDir`?
   - Recommendation: Only copy if the WDL's absolute path is not already under `cfg.BaseDir`. Otherwise send the existing absolute path. This avoids unnecessary copies when running from NFS. Confidence: HIGH.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go testing (`go test`) |
| Config file | none — standard `go test ./...` |
| Quick run command | `go test ./cmd/flowy/ ./internal/client/ -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CLI-01 | `flowy submit <wdl> -i <inputs>` stages files, submits, prints run ID | unit | `go test ./cmd/flowy/ -run TestSubmit -count=1` | ❌ Wave 0 |
| CLI-02 | `flowy status <run-id>` prints state | unit | `go test ./cmd/flowy/ -run TestStatus -count=1` | ❌ Wave 0 |
| CLI-03 | `flowy outputs <run-id>` prints JSON outputs for completed run | unit | `go test ./cmd/flowy/ -run TestOutputs -count=1` | ❌ Wave 0 |
| CLI-04 | `flowy run <wdl> -i <inputs>` submits, polls, prints outputs | unit | `go test ./cmd/flowy/ -run TestRun -count=1` | ❌ Wave 0 |
| CLI-05 | Local input files copied to shared FS before submission | unit | `go test ./internal/client/ -run TestStageFiles -count=1` | ❌ Wave 0 |
| CLI-06 | All three binaries build cleanly | build | `go build ./...` | ❌ Wave 0 (new client code) |

### Sampling Rate

- **Per task commit:** `go test ./cmd/flowy/ ./internal/client/ -count=1`
- **Per wave merge:** `go test ./... -count=1 && go build ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `internal/client/client.go` — `Client` struct + `Submit`, `Status`, `Outputs` methods
- [ ] `internal/client/client_test.go` — unit tests using `httptest.NewServer`
- [ ] `cmd/flowy/client_test.go` — integration tests for `submit`, `status`, `outputs`, `run` subcommands using `httptest.NewServer`
- [ ] `internal/config/config.go` — add `Default() *Config` exported function
- [ ] `internal/api/types.go` — add optional `run_id` field to `SubmitRequest`
- [ ] `internal/server/handlers.go` — update `handleStatus` and `handleOutputs` to support prefix matching via `GetRunByPrefix`
- [ ] Existing `cmd/flowy/run_test.go` — update or remove tests that use `mockRunner`/`newRunner` (those tests the old local execution path which is replaced)

*(The existing tests in `main_test.go` and `run_test.go` that test `check` and local task behavior should be preserved or updated; only the Docker-based tests need replacement)*

## Sources

### Primary (HIGH confidence)

- `internal/daemon/daemon.go` (codebase) — HTTP client pattern, `claimTask`/`reportResult`/`sendHeartbeat` as exact model for client methods
- `internal/api/types.go` (codebase) — `SubmitRequest`, `SubmitResponse`, `StatusResponse`, `OutputsResponse`, `ErrorResponse` — exact DTOs the client encodes/decodes
- `internal/server/handlers.go` (codebase) — server endpoint signatures, HTTP status codes, request/response bodies
- `internal/config/config.go` (codebase) — `ClientConfig{ServerURL}`, `Config{BaseDir}`, `applyDefaults()` logic
- `internal/store/rundir.go` (codebase) — `CreateRunDirs`, `RunInputsDir`, path layout for staging
- `cmd/flowy/main.go` (codebase) — existing `run()` dispatcher, `runRun()` structure to replace/extend
- `.planning/STATE.md` — "`flowy run` maintains synchronous behavior for spectool compatibility; server is required (no local fallback)"
- `.planning/phases/09-api-contract-persistence-foundation/09-CONTEXT.md` — "Client (`flowy`) works without config — defaults to connecting localhost:8124"

### Secondary (MEDIUM confidence)

- `go.mod` (codebase) — `github.com/google/uuid v1.6.0` already available for UUID generation
- Phase 07 and Phase 08 research documents — confirmed `flag` package pattern, `run(args, stdout, stderr)` testable-main pattern, `runArgs`/`runArgsWithRunner` test helpers
- `internal/daemon/daemon_test.go` pattern — `httptest.NewServer` approach for testing HTTP client code without real server

### Tertiary (LOW confidence)

None — all claims are verified from the codebase directly.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all packages verified from codebase; no new external deps
- Architecture: HIGH — HTTP client pattern confirmed from daemon implementation; file staging approach is reasoned from existing store layout
- File staging heuristic (string-is-file detection): MEDIUM — pragmatic heuristic, edge cases documented
- Prefix matching recommendation: HIGH — `GetRunByPrefix` already exists in store interface
- Config `Default()` addition: HIGH — straightforward addition to existing config package

**Research date:** 2026-03-12
**Valid until:** 2026-04-12 (stable — no external deps, all internal code)
