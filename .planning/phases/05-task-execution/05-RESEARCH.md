# Phase 5: Task Execution - Research

**Researched:** 2026-03-10
**Domain:** WDL task execution — Docker lifecycle, input binding, path mapping, output collection
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **ContainerRunner interface:** Single `Run(ctx context.Context, ...)` method handling full Docker lifecycle internally (pull if needed, create, start, wait, capture logs, remove). Image pulling implicit inside `Run()`. Accepts `context.Context` as first param. Returns exit code, stdout bytes, stderr bytes, and error.
- **Mock implementation:** Shell-based mock executing command via `os/exec` (`sh -c`) on host — real shell execution without Docker daemon.
- **Input file staging:** Bind-mount host files into container as read-only. Flat container-internal path: `/mnt/inputs/<filename>`. Collision: append hash suffix. Directory-type inputs: bind-mount entire host directory as `/mnt/inputs/<dirname>/` read-only.
- **Bidirectional path map:** Host↔container path map maintained so output expressions can translate paths back to host.
- **Output collection:** Container writes to `/mnt/outputs/` bind-mounted to host task execution directory. No `docker cp` needed.
- **Execution directory layout (miniwdl-style):** `execution/<workflow>/<call-name>/` with `work/`, `stdout.txt`, `stderr.txt`, `command`, `rc` files. Execution root defaults to `./execution/` under CWD.
- **Command invocation:** `bash /mnt/inputs/_command` (script file bind-mounted into container).
- **Runtime section:** v1 supports only `docker`/`container`, `cpu`, `memory`. `maxRetries`, `returnCodes`, `gpu`, `disks`, `preemptible` deferred. Memory as Docker `--memory` hard limit. CPU as Docker `--cpus`. Default image: `ubuntu:latest`.

### Claude's Discretion
- Exact `ContainerRunner` method signature (parameter struct vs. individual args)
- `wdl/task` package structure and public API design
- Memory string parsing (`"2 GiB"`, `"512 MB"` -> bytes conversion)
- How to handle `env` modifier on task input declarations (WDL 1.2: inject as environment variable)
- Error wrapping strategy for Docker client errors
- Whether to stream or batch stdout/stderr capture

### Deferred Ideas (OUT OF SCOPE)
- `maxRetries` and `returnCodes` runtime keys — v2 scope
- `gpu`, `disks`, `preemptible` runtime keys — v2 scope
- Task output caching/memoization — v2 (EXT-02)
- Streaming stdout/stderr during execution (for `--verbose` mode) — Phase 7 CLI concern
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| TASK-01 | Parse task input section, bind JSON inputs to declarations | JSON input binding, type coercion from `map[string]interface{}` to WDL Values, eval.Env population |
| TASK-02 | Parse runtime/requirements section (container, cpu, memory), reflect in Docker execution | `ast.Task.Runtime` and `ast.Task.Requirements` maps, eval expressions, Docker HostConfig fields |
| TASK-03 | Expand command template, produce string for container | `eval.Eval()` on `ast.Task.Command` (CommandExpr), `eval.ExecContext` struct |
| TASK-04 | Build bidirectional host↔container path map for input file mounting | FileValue/DirectoryValue traversal, bind-mount slice construction, flat `/mnt/inputs/` scheme |
| TASK-05 | Docker lifecycle: pull → create → start → wait → capture → remove | `docker/docker` client API, `ContainerCreate`/`ContainerStart`/`ContainerWait`/`ContainerLogs`/`ContainerRemove`, `stdcopy.StdCopy` |
| TASK-06 | Evaluate task output section, collect outputs to host paths | `eval.Eval()` with populated `ExecContext{WorkDir, StdoutPath, StderrPath}`, output expressions |
| TASK-07 | `ContainerRunner` interface for Docker, mockable in unit tests | Interface design, shell-based mock using `os/exec.Command("sh", "-c", ...)` |
| TASK-08 | Return exit code + stderr on task failure | `ContainerWait` StatusCode, `ErrTaskFailed` type with ExitCode and Stderr fields |
</phase_requirements>

---

## Summary

Phase 5 builds the `wdl/task` package — the bridge between WDL AST and Docker container execution. The package takes a parsed `*ast.Task` plus a JSON input map, orchestrates the full execution lifecycle, and returns a `map[string]values.Value` of outputs (or an error with exit code and stderr).

The execution pipeline is a sequential five-step process: (1) bind JSON inputs to the task's input section, populating `eval.Env`; (2) evaluate the runtime section to extract Docker image, CPU, and memory; (3) build the bidirectional host↔container path map and construct bind-mount specs; (4) expand the command template via `eval.Eval()` and write it as a script file; (5) run the Docker container via `ContainerRunner`, capture stdout/stderr, write `rc`, then evaluate the output section with a populated `ExecContext`.

The `ContainerRunner` interface is the key abstraction for testability: a real `DockerRunner` uses the `github.com/docker/docker` client (v28.x), while a `ShellRunner` mock executes commands via `os/exec.Command("sh", "-c", ...)` on the host, enabling full end-to-end testing without a live Docker daemon.

**Primary recommendation:** Implement `wdl/task` as a new package with a `RunTask(ctx, task, inputs, runner, execRoot)` public function. Keep DockerRunner in a sub-file; keep ShellRunner in `_test.go` or a separate `testutil` package. Prefer a struct parameter for `ContainerRunner.Run()` — it is cleaner than 8+ individual positional arguments.

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/docker/docker/client` | v28.x+incompatible | Docker Engine API client | Official moby client; established in STATE.md decisions |
| `github.com/docker/docker/api/types/container` | same | Config, HostConfig, WaitCondition structs | Part of same module |
| `github.com/docker/docker/pkg/stdcopy` | same | Demultiplex container log stream | Required for non-TTY containers |
| Go stdlib `os/exec` | 1.24.6 | Shell-based mock runner | Zero dependency, matches decision |
| Go stdlib `context` | 1.24.6 | Cancellation propagation | Phase 6 errgroup integration requirement |
| Go stdlib `crypto/sha256` | 1.24.6 | Filename collision hash suffix | Simple, no external dep |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/docker/docker/api/types/image` | same | ImagePullOptions | ImagePull call |
| `github.com/opencontainers/image-spec` | v1.x | `ocispec.Platform` for ContainerCreate | Pass `nil` for default platform |
| Go stdlib `path/filepath` | 1.24.6 | Execution dir construction, glob | Host path operations |
| Go stdlib `fmt`, `errors` | 1.24.6 | Error wrapping with `%w` | Standard Go error patterns |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `docker/docker` client | `fsouza/go-dockerclient` | fsouza lags behind official API; docker/docker is the canonical moby client |
| `docker/docker` client | `docker/go-sdk` (new Jul 2025) | go-sdk is very new, API unstable; docker/docker is battle-tested |
| `stdcopy.StdCopy` | Manual stream parsing | StdCopy handles the 8-byte multiplexed header correctly; hand-rolling is error-prone |

**Installation:**
```bash
go get github.com/docker/docker@v28.5.2+incompatible
go get github.com/docker/docker/api/types/container
```

Note: `+incompatible` suffix is expected — docker/docker does not follow Go module v2+ conventions. This is standard for the ecosystem.

---

## Architecture Patterns

### Recommended Project Structure
```
wdl/task/
├── task.go          # RunTask() public entry point, TaskResult, TaskError types
├── runner.go        # ContainerRunner interface + RunRequest/RunResult structs
├── docker.go        # DockerRunner implementation (ContainerRunner)
├── inputs.go        # bindInputs(): JSON map → eval.Env + path-map building
├── runtime.go       # parseRuntime(): ast.Task.Runtime/Requirements → DockerConfig
├── pathmap.go       # PathMap type, host↔container translation, bind-mount construction
├── command.go       # buildCommand(): evalCommand + write script file to disk
├── outputs.go       # collectOutputs(): evaluate output section with ExecContext
├── memory.go        # parseMemoryString(): "2 GiB" → int64 bytes
└── task_test.go     # End-to-end tests using ShellRunner mock
```

### Pattern 1: ContainerRunner Interface with Struct Parameter

**What:** Single-method interface with a request struct for clean extensibility.
**When to use:** Phase 7 can add fields (timeout, env vars) without changing the interface signature.

```go
// Source: wdl/task/runner.go (to be created)

// RunRequest carries all parameters needed to execute one container invocation.
type RunRequest struct {
    Image    string            // Docker image reference (e.g., "ubuntu:latest")
    Command  []string          // ["bash", "/mnt/inputs/_command"]
    Binds    []string          // Docker bind-mount specs: ["host:container:ro", ...]
    Env      []string          // Environment variables: ["KEY=VALUE", ...]
    Memory   int64             // Memory limit in bytes (0 = no limit)
    NanoCPUs int64             // CPU limit in nanocpus (0 = no limit)
    WorkDir  string            // Container working directory (typically "/mnt/outputs")
}

// RunResult carries the outcome of a container execution.
type RunResult struct {
    ExitCode int
    Stdout   []byte
    Stderr   []byte
}

// ContainerRunner abstracts Docker for unit-testability.
type ContainerRunner interface {
    Run(ctx context.Context, req RunRequest) (RunResult, error)
}
```

### Pattern 2: Input Binding — JSON → eval.Env

**What:** Walk the task's `Inputs []*ast.Decl`, match each to the JSON input map by name (bare name for standalone task execution, `TaskName.InputName` for workflow). Evaluate default expression if JSON key absent. Convert JSON values to `values.Value` using the existing `jsonToValue` pattern (already in `wdl/eval/stdlib_fileio.go`).

**When to use:** TASK-01. This is the entry to the task execution pipeline.

```go
// Source: wdl/task/inputs.go (to be created)

func bindInputs(task *ast.Task, jsonInputs map[string]interface{}) (*eval.Env, *PathMap, error) {
    env := eval.NewEnv(nil)
    pm := NewPathMap()
    for _, decl := range task.Inputs {
        raw, ok := jsonInputs[decl.Name]
        if !ok && decl.Expr == nil {
            // Required input missing
            return nil, nil, fmt.Errorf("missing required input %q", decl.Name)
        }
        var val values.Value
        if ok {
            val = jsonToWDLValue(raw, decl.Type)
        } else {
            var err error
            val, err = eval.Eval(decl.Expr, env, eval.ExecContext{})
            if err != nil {
                return nil, nil, err
            }
        }
        // Collect file/directory values into path map
        collectPaths(val, pm)
        env.Define(decl.Name, val)
    }
    return env, pm, nil
}
```

### Pattern 3: Docker Lifecycle — ContainerCreate → Wait → Logs

**What:** Full lifecycle in `DockerRunner.Run()`. Pulls image if not present (ImagePull discards response body by draining it to satisfy Docker daemon). Creates container with bind-mounts. Starts container. Waits with `"next-exit"` condition (set BEFORE Start to avoid race). Fetches logs with `stdcopy.StdCopy`. Removes container.

**When to use:** TASK-05. Critical: set up ContainerWait channels BEFORE calling ContainerStart.

```go
// Source: wdl/task/docker.go (to be created)
// Reference: https://pkg.go.dev/github.com/docker/docker/client

func (r *DockerRunner) Run(ctx context.Context, req RunRequest) (RunResult, error) {
    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        return RunResult{}, fmt.Errorf("docker client: %w", err)
    }
    defer cli.Close()

    // Pull image (implicit; discard output)
    pullResp, err := cli.ImagePull(ctx, req.Image, image.PullOptions{})
    if err != nil {
        return RunResult{}, fmt.Errorf("image pull %q: %w", req.Image, err)
    }
    io.Copy(io.Discard, pullResp)
    pullResp.Close()

    // Create container
    resp, err := cli.ContainerCreate(ctx,
        &container.Config{
            Image: req.Image,
            Cmd:   req.Command,
            Env:   req.Env,
        },
        &container.HostConfig{
            Binds: req.Binds,
            Resources: container.Resources{
                Memory:   req.Memory,
                NanoCPUs: req.NanoCPUs,
            },
        },
        nil, nil, "",
    )
    if err != nil {
        return RunResult{}, fmt.Errorf("container create: %w", err)
    }
    containerID := resp.ID

    // Set up wait BEFORE start (avoid race condition)
    waitCh, errCh := cli.ContainerWait(ctx, containerID, container.WaitConditionNextExit)

    // Start container
    if err := cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
        cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
        return RunResult{}, fmt.Errorf("container start: %w", err)
    }

    // Wait for completion
    var exitCode int
    select {
    case waitResp := <-waitCh:
        exitCode = int(waitResp.StatusCode)
    case err := <-errCh:
        cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
        return RunResult{}, fmt.Errorf("container wait: %w", err)
    case <-ctx.Done():
        cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
        return RunResult{}, ctx.Err()
    }

    // Collect logs (batch — not streaming; Phase 7 adds streaming)
    logResp, err := cli.ContainerLogs(ctx, containerID, container.LogsOptions{
        ShowStdout: true,
        ShowStderr: true,
    })
    if err != nil {
        cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
        return RunResult{}, fmt.Errorf("container logs: %w", err)
    }
    var stdoutBuf, stderrBuf bytes.Buffer
    stdcopy.StdCopy(&stdoutBuf, &stderrBuf, logResp)
    logResp.Close()

    // Remove container
    cli.ContainerRemove(ctx, containerID, container.RemoveOptions{})

    return RunResult{
        ExitCode: exitCode,
        Stdout:   stdoutBuf.Bytes(),
        Stderr:   stderrBuf.Bytes(),
    }, nil
}
```

### Pattern 4: PathMap — Bidirectional Host↔Container Translation

**What:** A struct holding a map from host path → container path and a slice for bind-mount specs. When building it, walk all FileValue and DirectoryValue in the input env. Translate host paths to container paths during command evaluation (by replacing values in a pre-eval step or using a translate-env function). After container exits, translate container paths back to host paths in output values.

**When to use:** TASK-04. Must be built before command evaluation.

```go
// Source: wdl/task/pathmap.go (to be created)

// PathMap maintains the bidirectional mapping between host paths and container paths.
type PathMap struct {
    hostToContainer map[string]string
    containerToHost map[string]string
    Binds           []string // Docker bind-mount specs: "hostPath:containerPath:ro"
}

// AddFile registers a host file path, assigning it a container path under /mnt/inputs/.
// Returns the container path.
func (pm *PathMap) AddFile(hostPath string) string {
    if cp, ok := pm.hostToContainer[hostPath]; ok {
        return cp
    }
    base := filepath.Base(hostPath)
    containerPath := "/mnt/inputs/" + base
    // Collision detection: append sha256 prefix if container path already taken
    if _, taken := pm.containerToHost[containerPath]; taken {
        h := sha256.Sum256([]byte(hostPath))
        containerPath = fmt.Sprintf("/mnt/inputs/%x_%s", h[:4], base)
    }
    pm.hostToContainer[hostPath] = containerPath
    pm.containerToHost[containerPath] = hostPath
    pm.Binds = append(pm.Binds, hostPath+":"+containerPath+":ro")
    return containerPath
}

// TranslateEnv returns a new eval.Env where all FileValue/DirectoryValue paths
// are replaced with their container-side equivalents.
func (pm *PathMap) TranslateEnv(env *eval.Env) *eval.Env { ... }

// UntranslateValue reverses container paths back to host paths in output values.
func (pm *PathMap) UntranslateValue(v values.Value) values.Value { ... }
```

### Pattern 5: Shell-Based Mock Runner

**What:** Implements `ContainerRunner` using `os/exec.Command("sh", "-c", script)`. Writes command script to a temp file, runs it on the host, captures stdout/stderr. Bind-mounts are ignored (files are already host-local). Enables full end-to-end testing of input binding, command expansion, and output collection without Docker.

**When to use:** TASK-07. Used exclusively in `task_test.go`.

```go
// Source: wdl/task/task_test.go or testutil package

type ShellRunner struct{}

func (r *ShellRunner) Run(ctx context.Context, req RunRequest) (RunResult, error) {
    // Find the _command bind: hostPath:/mnt/inputs/_command:ro
    var commandScript string
    for _, bind := range req.Binds {
        parts := strings.SplitN(bind, ":", 3)
        if len(parts) >= 2 && parts[1] == "/mnt/inputs/_command" {
            commandScript = parts[0]
            break
        }
    }
    cmd := exec.CommandContext(ctx, "sh", commandScript)
    cmd.Env = append(os.Environ(), req.Env...)
    // For output dir: ensure WorkDir bind exists on host
    var stdoutBuf, stderrBuf bytes.Buffer
    cmd.Stdout = &stdoutBuf
    cmd.Stderr = &stderrBuf
    err := cmd.Run()
    exitCode := 0
    if err != nil {
        var exitErr *exec.ExitError
        if errors.As(err, &exitErr) {
            exitCode = exitErr.ExitCode()
            err = nil // non-zero exit is not a runner error
        } else {
            return RunResult{}, err
        }
    }
    return RunResult{
        ExitCode: exitCode,
        Stdout:   stdoutBuf.Bytes(),
        Stderr:   stderrBuf.Bytes(),
    }, nil
}
```

### Pattern 6: Memory String Parsing

**What:** Parse `"2 GiB"`, `"512 MB"`, `"1024"` (bare integer = bytes) to int64 bytes. The existing `wdl/eval/stdlib_fileio.go` already defines `sizeUnits` map — reuse the same unit multipliers but return int64 instead of float64.

**When to use:** TASK-02. Called from `parseRuntime()` when processing memory key.

```go
// Source: wdl/task/memory.go (to be created)
// Reuse unit multipliers from wdl/eval sizeUnits pattern

var memUnits = map[string]int64{
    "B": 1, "bytes": 1,
    "kB": 1_000, "MB": 1_000_000, "GB": 1_000_000_000, "TB": 1_000_000_000_000,
    "KiB": 1024, "MiB": 1024 * 1024, "GiB": 1024 * 1024 * 1024, "TiB": 1024 * 1024 * 1024 * 1024,
}

func parseMemoryString(s string) (int64, error) {
    s = strings.TrimSpace(s)
    // Try bare integer (bytes)
    if n, err := strconv.ParseInt(s, 10, 64); err == nil {
        return n, nil
    }
    // Split on first space: "2 GiB" -> "2", "GiB"
    parts := strings.Fields(s)
    if len(parts) != 2 {
        return 0, fmt.Errorf("invalid memory string %q: expected '<number> <unit>'", s)
    }
    n, err := strconv.ParseFloat(parts[0], 64)
    if err != nil {
        return 0, fmt.Errorf("invalid memory value in %q: %w", s, err)
    }
    mult, ok := memUnits[parts[1]]
    if !ok {
        return 0, fmt.Errorf("unknown memory unit %q in %q", parts[1], s)
    }
    return int64(n * float64(mult)), nil
}
```

### Pattern 7: Task Error Type

**What:** A dedicated error type carrying exit code and stderr. Returned by `RunTask()` when container exits non-zero. Planner and Phase 6 can type-assert to inspect exit code.

```go
// Source: wdl/task/task.go (to be created)

// TaskError is returned when a WDL task exits with a non-zero exit code.
// It contains the exit code and captured stderr for diagnostic purposes.
type TaskError struct {
    TaskName string
    ExitCode int
    Stderr   string
}

func (e *TaskError) Error() string {
    return fmt.Sprintf("task %q failed with exit code %d: %s", e.TaskName, e.ExitCode, e.Stderr)
}
```

### Anti-Patterns to Avoid

- **Start before Wait setup:** Calling `ContainerStart` before setting up `ContainerWait` channels creates a race condition where the container can exit before the wait is registered. Always call `ContainerWait` first.
- **Raw host paths in command:** Passing `FileValue.Value` (host path) directly into the command string — container will not see them. All file paths must go through `PathMap.TranslateEnv()` before command evaluation.
- **Using `AutoRemove: true` in HostConfig:** Prevents log capture because the container is removed before logs can be fetched. Remove explicitly after log collection.
- **Forgetting to drain ImagePull response:** Docker daemon holds the connection open until the response body is read. Always drain and close the ImagePull `io.ReadCloser`.
- **String-based JSON input type matching without schema:** `map[string]interface{}` JSON values need type-aware conversion: bare `string` → StringValue or FileValue depending on `decl.Type`, `float64` whole number → IntValue. Follow the `jsonToValue` pattern already in `eval/stdlib_fileio.go`, extended with type hints from `decl.Type`.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Log stream demultiplexing | Custom 8-byte header parser | `docker/docker/pkg/stdcopy.StdCopy` | Multiplexed stream format has edge cases (partial reads, error stream type) |
| Docker API JSON marshalling | Custom HTTP client | `docker/docker/client.Client` | API versioning, TLS, Unix socket all handled |
| Image pull progress | Custom progress parser | `io.Copy(io.Discard, pullResp)` | Pull is implicit; progress display deferred to Phase 7 |

**Key insight:** The Docker client handles API negotiation, TLS, Unix socket vs TCP, and JSON serialization. Any custom HTTP implementation will miss these subtleties, especially API version negotiation (`WithAPIVersionNegotiation()`).

---

## Common Pitfalls

### Pitfall 1: ContainerWait Race Condition
**What goes wrong:** Container exits before `ContainerWait` is called → wait hangs or returns stale results.
**Why it happens:** `ContainerStart` is async; fast commands (like `echo`) exit almost instantly.
**How to avoid:** Set up `waitCh, errCh := cli.ContainerWait(ctx, id, container.WaitConditionNextExit)` BEFORE calling `ContainerStart`.
**Warning signs:** Tests pass locally (slow CI) but flake in fast environments.

### Pitfall 2: Raw Host Paths in Container Command
**What goes wrong:** Container command contains `/home/user/input.bam` — not visible inside container → file not found error.
**Why it happens:** `eval.Eval(task.Command, env, ctx)` uses `values.FileValue.Value` as-is; env holds host paths.
**How to avoid:** Before command evaluation, call `PathMap.TranslateEnv(env)` to replace all FileValue/DirectoryValue with container-internal `/mnt/inputs/...` paths. Pass the translated env to `eval.Eval`.
**Warning signs:** Container stderr contains "No such file or directory" for paths that exist on host.

### Pitfall 3: AutoRemove Prevents Log Capture
**What goes wrong:** Logs are empty even though task ran successfully.
**Why it happens:** `HostConfig.AutoRemove: true` causes Docker to remove the container immediately on exit, before `ContainerLogs` can be called.
**How to avoid:** Do not use `AutoRemove`. Always call `ContainerRemove` manually after log collection.
**Warning signs:** `ContainerLogs` returns empty body or error "no such container".

### Pitfall 4: ImagePull Response Not Drained
**What goes wrong:** Subsequent API calls hang or fail with connection errors.
**Why it happens:** Docker HTTP/1.1 connection pool cannot reuse connection until response body is fully consumed.
**How to avoid:** Always `io.Copy(io.Discard, resp)` then `resp.Close()` after `ImagePull`.
**Warning signs:** Intermittent timeouts on `ContainerCreate` after image pull.

### Pitfall 5: `+incompatible` Module Version Confusion
**What goes wrong:** `go get github.com/docker/docker` may resolve to an unexpected version.
**Why it happens:** Docker hasn't adopted Go module v2+ versioning; uses pseudo-version tagging.
**How to avoid:** Pin explicitly: `go get github.com/docker/docker@v28.5.2+incompatible`. Run `go mod tidy` after.
**Warning signs:** Compile errors about missing types in `container` package (API changed between major versions).

### Pitfall 6: JSON Float64 → WDL Type Mismatch
**What goes wrong:** Integer inputs from JSON (e.g., `{"cpu": 4}`) become `float64` in Go's `json.Unmarshal`. If naively converted to `FloatValue`, downstream expressions that expect `IntValue` fail.
**Why it happens:** `encoding/json` decodes all numbers as `float64` into `interface{}`.
**How to avoid:** Use `math.Trunc(x) == x` guard (already established in `eval/stdlib_fileio.go` `jsonToValue`) to promote whole-number float64 to `IntValue`. Additionally use `decl.Type` to guide conversion when binding inputs.
**Warning signs:** Type assertion panics in arithmetic expressions expecting `IntValue`.

### Pitfall 7: Output Expression File Paths Still Point to Container
**What goes wrong:** Output FileValue contains `/mnt/outputs/result.txt` (container path) instead of `execution/task/work/result.txt` (host path).
**Why it happens:** Output expressions are evaluated after container exit; `WorkDir` is set to host path, but `glob()` and file reads use the bind-mounted directory which already has the files at host path. However if any expression re-creates a FileValue from a container-side string literal, it will have wrong path.
**How to avoid:** Evaluate output expressions with `ExecContext{WorkDir: hostWorkDir, ...}`. The `glob()` stdlib already uses `ctx.WorkDir` for matching. FileValue results from glob are already host-absolute. Ensure path un-translation only needed for explicit string→File coercions.
**Warning signs:** Output FileValues that reference `/mnt/outputs/` instead of the host execution directory.

---

## Code Examples

Verified patterns from official sources:

### ContainerRunner Interface and DockerRunner
```go
// Source: https://pkg.go.dev/github.com/docker/docker/client (verified)
import (
    "context"
    "io"
    "bytes"

    "github.com/docker/docker/client"
    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/api/types/image"
    "github.com/docker/docker/pkg/stdcopy"
)

func newDockerClient() (*client.Client, error) {
    return client.NewClientWithOpts(
        client.FromEnv,
        client.WithAPIVersionNegotiation(), // automatically negotiates API version
    )
}
```

### Correct ContainerWait + ContainerStart Ordering
```go
// Source: https://pkg.go.dev/github.com/docker/docker/client#Client.ContainerWait (verified)
// CRITICAL: set up wait channels BEFORE calling ContainerStart
waitCh, errCh := cli.ContainerWait(ctx, containerID, container.WaitConditionNextExit)
if err := cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
    return err
}
select {
case res := <-waitCh:
    exitCode = int(res.StatusCode)
case err := <-errCh:
    return fmt.Errorf("wait error: %w", err)
case <-ctx.Done():
    return ctx.Err()
}
```

### Log Demultiplexing
```go
// Source: https://pkg.go.dev/github.com/docker/docker/pkg/stdcopy (verified)
// signature: func StdCopy(dstout, dsterr io.Writer, src io.Reader) (written int64, _ error)
logResp, _ := cli.ContainerLogs(ctx, id, container.LogsOptions{
    ShowStdout: true,
    ShowStderr: true,
})
defer logResp.Close()
var stdout, stderr bytes.Buffer
stdcopy.StdCopy(&stdout, &stderr, logResp)
```

### Bind Mount Spec Format
```go
// Docker bind-mount spec format: "hostPath:containerPath[:options]"
// Source: https://pkg.go.dev/github.com/docker/docker/api/types/container#HostConfig (verified)
binds := []string{
    "/host/input.bam:/mnt/inputs/input.bam:ro",
    "/host/work:/mnt/outputs:rw",
    "/host/exec/command:/mnt/inputs/_command:ro",
}
hostConfig := &container.HostConfig{
    Binds: binds,
    Resources: container.Resources{
        Memory:   2 * 1024 * 1024 * 1024, // 2 GiB
        NanoCPUs: 2_000_000_000,           // 2 CPUs (in nanocpus)
    },
}
```

### RunTask() Public API
```go
// Source: wdl/task/task.go (to be created)

// RunTask executes a single WDL task and returns its outputs.
// execRoot is the base execution directory (e.g., "./execution").
// Returns map of output declaration names to values, or *TaskError on non-zero exit.
func RunTask(
    ctx context.Context,
    task *ast.Task,
    jsonInputs map[string]interface{},
    runner ContainerRunner,
    execRoot string,
) (map[string]values.Value, error)
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `docker/docker v1.x` direct use | `client.WithAPIVersionNegotiation()` | Docker API 1.30+ | Negotiation prevents version mismatch panics |
| `container.WaitConditionRemoved` | `container.WaitConditionNextExit` | Always use NextExit when logs needed after | NextExit allows log collection before removal |
| TTY containers with merged logs | Non-TTY with `stdcopy.StdCopy` | Standard for task runners | Separate stdout/stderr capture |
| `docker cp` for output collection | Bind-mount `/mnt/outputs/` | miniwdl established this pattern | Zero-copy; files available immediately on host |

**Deprecated/outdated:**
- `container.WaitConditionRemoved`: Use this only if you don't need to read logs. For task execution always use `WaitConditionNextExit`.
- `io.Copy` on raw `ContainerLogs` reader: For non-TTY containers, raw copy mixes stdout/stderr into a single stream. Always use `stdcopy.StdCopy`.

---

## Open Questions

1. **`env` modifier on task input declarations (WDL 1.2 `ast.Decl.Env == true`)**
   - What we know: `ast.Decl.Env` field exists; WDL 1.2 spec says env-modified inputs are injected as environment variables in the container.
   - What's unclear: The exact environment variable name (declaration name? or declaration name uppercased?). WDL 1.2 spec says the variable name is the declaration name as-is.
   - Recommendation: When `decl.Env == true`, add `"DECL_NAME=stringified_value"` to `RunRequest.Env`. The `valueToString()` function in `eval/command.go` can be reused for serialization.

2. **`requirements` vs `runtime` section key priority**
   - What we know: `ast.Task` has both `Runtime map[string]Expr` and `Requirements map[string]Expr`. WDL 1.2 introduced `requirements` as the preferred section; `runtime` is kept for backward compat.
   - What's unclear: If both specify `docker`/`container`, which wins?
   - Recommendation: Check `Requirements` first (WDL 1.2 preferred), fall back to `Runtime`. Log a warning if both are present (LOW confidence on exact spec behavior; use Requirements-wins as safe default matching Cromwell behavior).

3. **`docker` vs `container` key naming**
   - What we know: WDL 1.2 spec says `container` is canonical; `docker` is an alias with identical semantics.
   - Recommendation: Check both keys; `container` takes priority if both present.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib `testing` package) |
| Config file | none — `go test ./...` discovers all `*_test.go` files |
| Quick run command | `go test ./wdl/task/... -run TestBind -v` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TASK-01 | JSON inputs bound to task.Inputs, defaults evaluated | unit | `go test ./wdl/task/... -run TestBindInputs -v` | ❌ Wave 0 |
| TASK-02 | Runtime/requirements parsed; docker image, cpu, memory extracted | unit | `go test ./wdl/task/... -run TestParseRuntime -v` | ❌ Wave 0 |
| TASK-03 | Command template expanded with container-translated paths | unit | `go test ./wdl/task/... -run TestBuildCommand -v` | ❌ Wave 0 |
| TASK-04 | PathMap assigns unique container paths; bind specs correct | unit | `go test ./wdl/task/... -run TestPathMap -v` | ❌ Wave 0 |
| TASK-05 | Docker lifecycle completes; stdout/stderr captured | integration (needs daemon) | `go test ./wdl/task/... -run TestDockerRunner -tags integration` | ❌ Wave 0 |
| TASK-06 | Output section evaluated with ExecContext; FileValues collected | unit | `go test ./wdl/task/... -run TestCollectOutputs -v` | ❌ Wave 0 |
| TASK-07 | End-to-end task execution via ShellRunner mock | unit | `go test ./wdl/task/... -run TestRunTask -v` | ❌ Wave 0 |
| TASK-08 | Non-zero exit returns *TaskError with ExitCode and Stderr | unit | `go test ./wdl/task/... -run TestTaskError -v` | ❌ Wave 0 |

**Note on TASK-05:** `DockerRunner` integration tests require a live Docker daemon and should be gated with a build tag (`//go:build integration`) or skipped with `t.Skip("requires docker daemon")` when `DOCKER_HOST` is unset. All other requirements are fully testable with `ShellRunner`.

### Sampling Rate
- **Per task commit:** `go test ./wdl/task/... -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green (`go test ./... -count=1`) before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `wdl/task/task_test.go` — covers TASK-01, TASK-03, TASK-06, TASK-07, TASK-08 (ShellRunner end-to-end)
- [ ] `wdl/task/inputs_test.go` — covers TASK-01 (unit: bindInputs)
- [ ] `wdl/task/runtime_test.go` — covers TASK-02 (unit: parseRuntime)
- [ ] `wdl/task/pathmap_test.go` — covers TASK-04 (unit: PathMap)
- [ ] `wdl/task/memory_test.go` — covers memory string parsing (TASK-02 sub-function)
- [ ] `wdl/task/docker_test.go` — covers TASK-05 (integration, `//go:build integration` tag)

---

## Sources

### Primary (HIGH confidence)
- `https://pkg.go.dev/github.com/docker/docker/client` — ContainerCreate, ContainerStart, ContainerWait, ContainerLogs, ContainerRemove, ImagePull signatures
- `https://pkg.go.dev/github.com/docker/docker/api/types/container` — Config, HostConfig, Resources, WaitCondition, WaitResponse structs
- `https://pkg.go.dev/github.com/docker/docker/pkg/stdcopy` — StdCopy signature and multiplexed stream protocol
- `wdl/eval/context.go` — ExecContext struct (read directly)
- `wdl/eval/env.go` — Env linked-list API (read directly)
- `wdl/eval/eval.go` — Eval() dispatch (read directly)
- `wdl/eval/command.go` — evalCommand, valueToString (read directly)
- `wdl/eval/stdlib_fileio.go` — jsonToValue, sizeUnits, stdGlob, stdStdout, stdStderr (read directly)
- `wdl/values/values.go` — FileValue, DirectoryValue, NullValue (read directly)
- `wdl/ast/ast.go` — Task, Decl, CommandExpr structs (read directly)

### Secondary (MEDIUM confidence)
- `https://docs.docker.com/engine/api/sdk/examples/` — Docker Go SDK usage patterns (official docs, verified against client package)
- `https://docs.openwdl.org/language-guide/tasks.html` — WDL task runtime/requirements attributes; memory "GiB" unit example
- `https://miniwdl.readthedocs.io/en/latest/runner_advanced.html` — miniwdl bind-mount strategy and path mapping approach

### Tertiary (LOW confidence)
- WebSearch result summaries on `requirements` vs `runtime` section priority — verified against spec summary but not against spec raw text (GitHub returned 429 during fetch)
- WDL 1.2 `env` modifier behavior (variable name form) — confirmed field exists in AST but spec text not directly fetched

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — pkg.go.dev documentation fetched and verified; module version confirmed (v28.5.2+incompatible)
- Architecture: HIGH — patterns derived from existing codebase (eval package) plus verified Docker client API
- Pitfalls: HIGH — ContainerWait race condition and AutoRemove+logs issue are well-documented Docker client gotchas; path translation pitfall derived from codebase analysis
- Memory parsing: HIGH — unit table mirrors already-implemented sizeUnits in stdlib_fileio.go
- `env` modifier: LOW — field exists in AST but spec text not verified

**Research date:** 2026-03-10
**Valid until:** 2026-06-10 (Docker client API stable; WDL 1.2 spec frozen)
