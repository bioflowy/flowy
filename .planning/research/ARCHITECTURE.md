# Architecture Research

**Domain:** WDL workflow engine — S3/object storage backend integration
**Researched:** 2026-03-12
**Confidence:** HIGH (based on direct codebase analysis + AWS SDK Go v2 docs + Cromwell S3 patterns)

---

## Standard Architecture

### System Overview: Current (v1.1 — shared FS only)

```
  ┌─────────────────────────────────────────────────────────────────┐
  │  flowy (client CLI)                                              │
  │  submit | status | outputs | run (sync mode)                    │
  └──────────────────────────┬──────────────────────────────────────┘
                             │ HTTP REST
  ┌──────────────────────────▼──────────────────────────────────────┐
  │  flowy-server                                                    │
  │  ┌──────────────┐  ┌──────────────┐  ┌───────────────────────┐  │
  │  │ REST API     │  │ DAG Scheduler│  │  Stale Detection      │  │
  │  │ (handlers.go)│  │ (dispatch.go)│  │  (server.go)          │  │
  │  └──────────────┘  └──────┬───────┘  └───────────────────────┘  │
  │                           │ os.WriteFile(inputs.json)            │
  │  ┌────────────────────────▼────────────────────────────────────┐ │
  │  │  internal/store/rundir.go  (FS path helpers)                │ │
  │  │  {baseDir}/runs/{runID}/tasks/{name}_{shard}/inputs.json    │ │
  │  └────────────────────────────────────────────────────────────┘ │
  │  ┌──────────────────────────────────────────────┐               │
  │  │  SQLite (store/sqlite/) — task queue          │               │
  │  └──────────────────────────────────────────────┘               │
  └─────────────────────────────────────────────────────────────────┘
               Shared FS (NFS or local): daemon reads/writes here
  ┌─────────────────────────────────────────────────────────────────┐
  │  flowy-daemon (1..N instances)                                   │
  │  ┌─────────────────────────────────────────────────────────┐    │
  │  │  workerLoop → executeTask                                │    │
  │  │  reads inputs.json from shared FS                        │    │
  │  │  writes outputs.json to shared FS                        │    │
  │  └─────────────────────────────┬───────────────────────────┘    │
  │  ┌─────────────────────────────▼───────────────────────────┐    │
  │  │  wdl/task.RunTask()                                      │    │
  │  │  BindInputs → PathMap → Docker bind mounts → CollectOutputs│  │
  │  └────────────────────────────────────────────────────────┘    │
  └─────────────────────────────────────────────────────────────────┘
```

### System Overview: Target (v1.2 — with S3 backend)

```
  ┌─────────────────────────────────────────────────────────────────┐
  │  flowy (client CLI)                                              │
  │  --storage {local|s3}  (new flag, passed in SubmitRequest)       │
  └──────────────────────────┬──────────────────────────────────────┘
                             │ HTTP REST (SubmitRequest.StorageOverride)
  ┌──────────────────────────▼──────────────────────────────────────┐
  │  flowy-server                                                    │
  │  ┌──────────────────────────────────────────────────────────┐   │
  │  │  Server struct gets:  backend storage.Backend  (injected) │   │
  │  └───────────────────────────────────────────────────────────┘   │
  │  ┌──────────────────────────────────────────────────────────┐   │
  │  │  DAG Scheduler (dispatch.go)                              │   │
  │  │  backend.WriteTaskInputs() instead of os.WriteFile        │   │
  │  │  backend.ReadTaskOutputs() instead of os.ReadFile         │   │
  │  └──────────────────────────────────────────────────────────┘   │
  │  ┌──────────────────────────────────────────────────────────┐   │
  │  │  handleClaim: injects storage config into TaskClaimResponse│  │
  │  │  handleOutputs: backend.ReadRunOutputs()                  │   │
  │  └──────────────────────────────────────────────────────────┘   │
  └─────────────────────────────────────────────────────────────────┘
                             │ TaskClaimResponse + StorageType/Bucket/Endpoint
  ┌──────────────────────────▼──────────────────────────────────────┐
  │  flowy-daemon                                                    │
  │  Builds Backend from claim response config                       │
  │  ┌──────────────────────────────────────────────────────────┐   │
  │  │  Download phase (NEW):                                    │   │
  │  │  localizeInputs(inputsJSON, stagingDir, backend)          │   │
  │  │  for each s3:// File input → backend.DownloadFile()       │   │
  │  │  rewrites paths to local temp paths                       │   │
  │  └────────────────────────────┬─────────────────────────────┘   │
  │  ┌─────────────────────────────▼───────────────────────────┐    │
  │  │  wdl/task.RunTask() — UNCHANGED (local paths only)       │    │
  │  └────────────────────────────┬────────────────────────────┘    │
  │  ┌─────────────────────────────▼───────────────────────────┐    │
  │  │  Upload phase (NEW):                                     │    │
  │  │  delocalizeOutputs(outputs, backend)                     │    │
  │  │  for each FileValue output → backend.UploadFile()        │    │
  │  │  rewrites paths to s3:// URIs in outputs.json            │    │
  │  └────────────────────────────────────────────────────────┘    │
  └─────────────────────────────────────────────────────────────────┘
                             │
  ┌──────────────────────────▼──────────────────────────────────────┐
  │  internal/storage  (NEW PACKAGE)                                 │
  │  ┌─────────────────┐  ┌──────────────┐  ┌──────────────────┐    │
  │  │ Backend         │  │ LocalBackend │  │ S3Backend        │    │
  │  │ (interface)     │  │ wraps rundir │  │ aws-sdk-go-v2    │    │
  │  └─────────────────┘  └──────────────┘  └──────────────────┘    │
  └─────────────────────────────────────────────────────────────────┘
                             │
  ┌──────────────────────────▼──────────────────────────────────────┐
  │  S3 / MinIO                                                      │
  │  {bucket}/{prefix}/runs/{runID}/tasks/{name}_{shard}/inputs.json │
  │  {bucket}/{prefix}/runs/{runID}/tasks/{name}_{shard}/outputs/... │
  └─────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | New vs Modified |
|-----------|----------------|-----------------|
| `internal/storage/` | `Backend` interface + `LocalBackend` + `S3Backend` | NEW |
| `internal/config.StorageConfig` | S3 endpoint, bucket, credentials, backend type | MODIFIED |
| `internal/store/rundir.go` | Owns local FS path layout; `LocalBackend` delegates here | UNCHANGED |
| `internal/api.TaskClaimResponse` | Adds `StorageType`/`StorageBucket`/`StorageEndpoint`/`StoragePrefix` | MODIFIED |
| `internal/server/server.go` | `Server` struct gets `backend storage.Backend` field | MODIFIED |
| `internal/server/dispatch.go` | `makeDispatchFn` calls `backend.WriteTaskInputs`; `loadTaskOutputs` calls `backend.ReadTaskOutputs` | MODIFIED |
| `internal/server/handlers.go` | `handleOutputs` calls `backend.ReadRunOutputs`; `handleClaim` populates storage config in response | MODIFIED |
| `internal/daemon/daemon.go` | `executeTask` adds download/upload phases around `RunTask` | MODIFIED |
| `wdl/task/inputs.go` | `cleanPath`: guard for `s3://` prefix (skip filepath resolution) | MODIFIED |
| `flowy/main.go` | `--storage` flag, passes `StorageOverride` in `SubmitRequest` | MODIFIED |

---

## New Package: `internal/storage`

### Backend Interface

```go
// Package storage defines the Backend interface and implementations for
// workflow run data I/O. Implementations: LocalBackend and S3Backend.
package storage

import "context"

// Backend abstracts file I/O for workflow run task data.
// All methods are safe for concurrent use.
type Backend interface {
    // WriteTaskInputs writes inputs.json for a task shard.
    // Returns the address string (local path or s3:// URI).
    WriteTaskInputs(ctx context.Context, runID, taskName string, shardIndex int, data []byte) (addr string, err error)

    // ReadTaskOutputs reads outputs.json for a completed task shard.
    ReadTaskOutputs(ctx context.Context, runID, taskName string, shardIndex int) ([]byte, error)

    // WriteTaskOutputs writes outputs.json for a completed task shard.
    WriteTaskOutputs(ctx context.Context, runID, taskName string, shardIndex int, data []byte) error

    // ReadRunOutputs reads the run-level outputs.json.
    ReadRunOutputs(ctx context.Context, runID string) ([]byte, error)

    // WriteRunOutputs writes the run-level outputs.json.
    WriteRunOutputs(ctx context.Context, runID string, data []byte) error

    // DownloadFile downloads a file at addr to a local directory.
    // For LocalBackend: no-op, returns addr unchanged.
    // For S3Backend: streams s3://bucket/key to localDir/filename.
    DownloadFile(ctx context.Context, addr string, localDir string) (localPath string, err error)

    // UploadFile uploads a local file to the backend and returns its address.
    // For LocalBackend: no-op, returns localPath unchanged.
    // For S3Backend: streams localPath to S3 and returns s3://bucket/key.
    UploadFile(ctx context.Context, localPath string, runID, taskName string, shardIndex int) (addr string, err error)

    // CreateRunDirs ensures the run's storage container exists.
    // For LocalBackend: os.MkdirAll.
    // For S3Backend: validates bucket accessible (no-op — S3 has no directories).
    CreateRunDirs(ctx context.Context, runID string) error
}
```

### Why This Interface Boundary

The interface is coarse-grained (whole-file operations, not streaming primitives) because:

1. `wdl/task.RunTask` must receive **local POSIX paths** — Docker bind mounts cannot accept S3 URIs. All S3 localization happens in the daemon, outside RunTask. The interface does not need to be visible to the `wdl/task` package.
2. The server only touches `inputs.json`, `outputs.json`, and the run-level `outputs.json` — all small JSON blobs. `GetObject`/`PutObject` with byte slices is sufficient; no streaming needed on the server side.
3. Large file uploads (BAM, VCF) happen in the daemon's upload phase and are handled by `UploadFile`, which internally uses multipart upload for objects over a configurable threshold.

---

## Recommended Project Structure

```
internal/
├── storage/                    # NEW — storage abstraction package
│   ├── backend.go              # Backend interface definition
│   ├── local.go                # LocalBackend: wraps os/filepath + store.rundir helpers
│   ├── s3.go                   # S3Backend: wraps aws-sdk-go-v2/service/s3
│   ├── s3_test.go              # Integration tests vs MinIO (build tag: integration)
│   └── factory.go              # NewBackendFromConfig() convenience constructor
├── config/
│   └── config.go               # + StorageConfig struct added
├── api/
│   └── types.go                # + StorageType/Bucket/Endpoint/Prefix in TaskClaimResponse
│                               # + StorageOverride in SubmitRequest
├── server/
│   ├── server.go               # + backend storage.Backend field in Server struct
│   ├── dispatch.go             # inject backend, replace os.WriteFile/os.ReadFile calls
│   └── handlers.go             # handleOutputs, handleClaim updated
└── daemon/
    └── daemon.go               # executeTask: download phase → RunTask → upload phase
```

---

## Architectural Patterns

### Pattern 1: Localize-Execute-Delocalize (Cromwell model)

**What:** Before running a Docker task, download all S3-addressed `File` inputs to a local staging directory. After Docker completes, upload all `File` outputs back to S3. `RunTask` itself operates entirely on local POSIX paths and is unchanged.

**When to use:** Required whenever the container runner uses Docker bind mounts (always true for this engine).

**Why this pattern fits flowy:** The existing `RunTask` → `PathMap` → Docker bind mount pipeline is clean and heavily tested. Inserting S3 awareness into that pipeline would break the `ContainerRunner` abstraction and make the `wdl/task` package depend on an S3 SDK. Cromwell and miniwdl both use pre/post localization for the same reason.

**Implementation sketch in daemon.go:**

```go
func (d *Daemon) executeTask(claim *api.TaskClaimResponse) {
    defer d.wg.Done()
    taskCtx := context.Background()

    // Build backend from claim config (daemon has its own credentials from config file).
    backend := storage.NewBackendFromClaim(claim, d.cfg)

    // Read inputs.json from backend.
    inputsData, err := backend.ReadTaskInputs(taskCtx, claim.RunID, claim.TaskName, claim.ShardIndex)

    // Download phase: rewrite s3:// File paths in inputsJSON to local staging paths.
    stagingDir, cleanup := d.newStagingDir(claim.TaskID)
    defer cleanup()  // removes staging dir after upload phase completes
    localInputsJSON, err := localizeInputs(taskCtx, inputsData, stagingDir, backend)

    // Execute: RunTask sees only local paths (unchanged).
    outputs, err := task.RunTask(taskCtx, astTask, localInputsJSON, task.NewDockerRunner(),
        claim.OutputsDir, wdlDir, structDefs)

    // Upload phase: rewrite FileValue paths to s3:// URIs, upload files.
    remoteOutputs, err := delocalizeOutputs(taskCtx, outputs, backend,
        claim.RunID, claim.TaskName, claim.ShardIndex)

    // Write outputs.json (with s3:// URIs) to backend.
    backend.WriteTaskOutputs(taskCtx, claim.RunID, claim.TaskName, claim.ShardIndex, outputsData)
    d.reportResult(taskCtx, claim.TaskID, "COMPLETED", "")
}
```

**Trade-off:** Adds latency for large input files. Acceptable for genomics (reference files are large but per-run; output BAMs are written once). The staging directory is cleaned up after delocalization, preventing unbounded disk growth.

### Pattern 2: S3 URI Detection at Input Path Binding

**What:** In `wdl/task/inputs.go` `cleanPath`, detect `s3://` prefix and return the path unchanged (skip `filepath.Abs` and `filepath.Clean`). The `s3://` URI is an opaque identifier at this layer; the daemon's download phase translates it to a local path before `RunTask` is called.

**When to use:** Whenever inputs.json may contain `s3://bucket/key` strings for `File`-typed inputs.

**Minimal change (5 lines):**

```go
func cleanPath(p string, wdlDir string) string {
    if strings.HasPrefix(p, "s3://") {
        return p  // opaque S3 URI — daemon localizes before RunTask
    }
    // existing local-path logic unchanged ...
    if !filepath.IsAbs(p) { ... }
    return filepath.Clean(p)
}
```

**Why here:** `cleanPath` is the single choke-point where all `File`/`Directory` string values become canonical paths. Adding the guard here is the minimal, contained change that avoids scattered S3 awareness across the `wdl/task` package.

**Note:** The inputs.json stored in the backend always has the original `s3://` URIs. The daemon's `localizeInputs` helper produces a second in-memory map with local paths for passing to `RunTask`. The s3:// values are never written to the local exec directory.

### Pattern 3: Backend Config Propagation via TaskClaimResponse

**What:** The server includes backend type and S3 config (endpoint, bucket, prefix) in `TaskClaimResponse`. The daemon constructs the appropriate `Backend` from this config. Credentials are NOT included in the claim response.

**Why:** Daemons in a distributed cluster may run on different hosts without access to the server's config file. Embedding storage addressing config (bucket, endpoint, prefix) in the claim response makes the daemon self-contained per task. Credentials come from the daemon's own config file or IAM instance role — they are never serialized into the API.

**Extended `TaskClaimResponse`:**

```go
type TaskClaimResponse struct {
    // ... existing fields (TaskID, RunID, TaskName, ShardIndex, WDLPath) ...

    // Storage backend config for this task (credentials not included).
    StorageType     string `json:"storage_type"`               // "local" or "s3"
    StorageBucket   string `json:"storage_bucket,omitempty"`   // S3 bucket name
    StorageEndpoint string `json:"storage_endpoint,omitempty"` // MinIO endpoint URL
    StoragePrefix   string `json:"storage_prefix,omitempty"`   // key prefix
}
```

The `InputsDir`/`OutputsDir` fields in the existing `TaskClaimResponse` become the `StoragePrefix` + task sub-path for S3, or remain local FS paths for `LocalBackend`. The server populates these fields in `handleClaim`.

### Pattern 4: LocalBackend as Zero-Regression Wrapper

**What:** `LocalBackend` wraps the existing `store/rundir.go` functions and `os.ReadFile`/`os.WriteFile`. The server's behavior for the local backend is byte-for-byte identical to v1.1.

**Implementation:**

```go
type LocalBackend struct{ BaseDir string }

func (b *LocalBackend) WriteTaskInputs(_ context.Context, runID, taskName string, shard int, data []byte) (string, error) {
    path := filepath.Join(store.TaskDir(b.BaseDir, runID, taskName, shard), "inputs.json")
    return path, os.WriteFile(path, data, 0o644)
}

func (b *LocalBackend) DownloadFile(_ context.Context, addr string, _ string) (string, error) {
    return addr, nil  // local path, no-op
}

func (b *LocalBackend) UploadFile(_ context.Context, localPath string, _, _ string, _ int) (string, error) {
    return localPath, nil  // local path, no-op
}
```

**Why:** All existing spectool tests run against `LocalBackend` without any changes. If spectool passes after the refactor, the storage abstraction is correct.

### Pattern 5: Input Path Resolution Rules

**What:** Clear precedence rules for how input path strings map to storage backends.

| Input path format | Resolution |
|-------------------|------------|
| `s3://bucket/key` | S3Backend downloads to staging dir (daemon localizeInputs) |
| `/absolute/path` | Local filesystem, passed as-is to RunTask |
| `relative/path` | Resolved relative to WDL file's parent directory (existing behavior) |

The `--storage` CLI flag selects where the server writes `inputs.json` and `outputs.json`. It does not prevent using `s3://` paths as File input values regardless of backend selection. Example: `--storage local` with `s3://my-bucket/ref.fasta` as a File input is valid — the daemon downloads the S3 file before running the container.

---

## Data Flow

### S3 Backend: Task Dispatch (Server Side)

```
makeDispatchFn (dispatch.go)
    ↓
backend.CreateRunDirs(ctx, runID)        ← validates bucket exists (S3 no-op)
    ↓
backend.WriteTaskInputs(ctx, runID, taskName, shard, inputsJSON)
    → PUT s3://{bucket}/{prefix}/runs/{runID}/tasks/{name}_{shard}/inputs.json
    returns: "s3://bucket/prefix/runs/..." (addr stored in task record? not needed)
    ↓
store.CreateTask(ctx, storeTask)
    → SQLite: state=PENDING
    (task will be claimed; claim response includes storage config)
```

### S3 Backend: Task Execution (Daemon Side)

```
claimTask()  ← POST /tasks/claim
    ↓
TaskClaimResponse{StorageType:"s3", StorageBucket:"my-bucket",
                  StorageEndpoint:"http://minio:9000", StoragePrefix:"flowy"}
    ↓
build S3Backend from claim config + daemon credentials (from daemon config file)
    ↓
backend.ReadTaskInputs(ctx, runID, taskName, shard)
    → GET s3://my-bucket/flowy/runs/{runID}/tasks/{name}_{shard}/inputs.json
    returns: []byte of inputs.json (with s3:// File paths)
    ↓
localizeInputs(ctx, inputsData, stagingDir, backend)
    parse inputs.json
    for each FileValue with "s3://..." path:
        backend.DownloadFile(ctx, "s3://bucket/data/ref.fasta", stagingDir)
        → streams to /tmp/task-{id}/inputs/ref.fasta
        replaces path in local inputs map: "s3://..." → "/tmp/task-{id}/inputs/ref.fasta"
    returns: localInputsJSON (local paths only)
    ↓
task.RunTask(ctx, astTask, localInputsJSON, DockerRunner, execDir, wdlDir, structDefs)
    PathMap.AddFile("/tmp/task-{id}/inputs/ref.fasta") → /mnt/inputs/ref.fasta
    Docker runs with local bind mounts (UNCHANGED)
    outputs: {output_bam: FileValue{"/execDir/work/output.bam"}}
    ↓
delocalizeOutputs(ctx, outputs, backend, runID, taskName, shard)
    for each FileValue output:
        backend.UploadFile(ctx, "/execDir/work/output.bam", runID, taskName, shard)
        → PUT s3://bucket/flowy/runs/{runID}/tasks/{name}_{shard}/outputs/output.bam
        rewrite FileValue: "/execDir/work/output.bam" → "s3://bucket/.../output.bam"
    ↓
backend.WriteTaskOutputs(ctx, runID, taskName, shard, outputsJSON_with_s3_URIs)
    → PUT s3://bucket/.../outputs.json
    ↓
reportResult(taskCtx, claim.TaskID, "COMPLETED", "")
    → cleanup(stagingDir)  // defer runs
```

### S3 Backend: Output Collection (Server Side)

```
waitForTask (dispatch.go) polls SQLite until TaskCompleted
    ↓
backend.ReadTaskOutputs(ctx, runID, taskName, shard)
    → GET s3://bucket/.../outputs.json
    ↓
jsonToValue(raw) — produces FileValue{Value: "s3://bucket/.../output.bam"}
    (s3:// path treated as opaque string by jsonToValue, no validation)
    ↓
workflow engine accumulates outputs (s3:// path strings in FileValues)
    ↓
backend.WriteRunOutputs(ctx, runID, runLevelOutputsJSON)
    → PUT s3://bucket/flowy/runs/{runID}/outputs.json
    ↓
store.UpdateRunState(ctx, runID, RunCompleted)
```

### GET /workflows/{id}/outputs (Server Side)

```
handleOutputs (handlers.go)
    ↓
backend.ReadRunOutputs(ctx, run.ID)
    → GET s3://bucket/flowy/runs/{runID}/outputs.json (S3)
    OR os.ReadFile({baseDir}/runs/{runID}/outputs.json) (local)
    ↓
json.Unmarshal → return to client
```

---

## Integration Points: What Changes Where

### `internal/storage/` (NEW — build first)

Dependencies: `internal/store/rundir.go` (LocalBackend only), `github.com/aws/aws-sdk-go-v2/service/s3` (S3Backend only).

`factory.go` exports:

```go
func NewBackendFromConfig(cfg *config.Config) (Backend, error)
func NewBackendFromClaim(claim *api.TaskClaimResponse, daemonCfg *config.Config) (Backend, error)
```

### `internal/config/config.go` (MODIFIED)

Add `StorageConfig` struct. Default `Type: "local"`.

```go
type StorageConfig struct {
    Type      string `yaml:"type"`       // "local" (default) or "s3"
    Bucket    string `yaml:"bucket"`
    Prefix    string `yaml:"prefix"`     // key prefix, default "flowy"
    Endpoint  string `yaml:"endpoint"`   // override for MinIO: "http://localhost:9000"
    Region    string `yaml:"region"`     // default "us-east-1"
    AccessKey string `yaml:"access_key"` // not propagated over API
    SecretKey string `yaml:"secret_key"` // not propagated over API
}

type Config struct {
    // ... existing fields ...
    Storage StorageConfig `yaml:"storage"`
}
```

### `internal/api/types.go` (MODIFIED)

Two changes:
1. `SubmitRequest` gets `StorageOverride string` (optional client `--storage` flag).
2. `TaskClaimResponse` gets four new fields (StorageType, StorageBucket, StorageEndpoint, StoragePrefix).

The existing `InputsDir`/`OutputsDir` fields become redundant for the S3 backend (paths are implicit from run/task/shard) but must remain for backward compatibility with the local backend.

### `internal/server/server.go` (MODIFIED)

```go
type Server struct {
    cfg     *config.Config
    store   store.Store
    backend storage.Backend  // NEW
    // ... existing fields ...
}

func New(cfg *config.Config, st store.Store, backend storage.Backend) *Server { ... }
```

### `internal/server/dispatch.go` (MODIFIED)

Replace four direct filesystem calls:

| Old call | New call |
|----------|----------|
| `store.CreateRunDirs(s.cfg.BaseDir, runID)` | `s.backend.CreateRunDirs(ctx, runID)` |
| `store.CreateTaskDirs(s.cfg.BaseDir, runID, ...)` | `s.backend.CreateTaskDirs(ctx, runID, ...)` (or absorbed into WriteTaskInputs) |
| `os.WriteFile(inputsPath, inputsData, ...)` | `s.backend.WriteTaskInputs(ctx, runID, taskName, shard, inputsData)` |
| `os.ReadFile(outputsPath)` in `loadTaskOutputs` | `s.backend.ReadTaskOutputs(ctx, runID, taskName, shard)` |

Also: `runWorkflow` writes run-level `outputs.json` via `s.backend.WriteRunOutputs(ctx, runID, data)`.

### `internal/server/handlers.go` (MODIFIED)

- `handleOutputs`: replace `os.ReadFile(outputsPath)` with `s.backend.ReadRunOutputs(r.Context(), run.ID)`.
- `handleClaim`: populate `StorageType`, `StorageBucket`, `StorageEndpoint`, `StoragePrefix` from `s.cfg.Storage` in the `TaskClaimResponse`.
- `handleSubmit` + `handleTaskRun`: replace `store.CreateRunDirs(s.cfg.BaseDir, runID)` with `s.backend.CreateRunDirs(r.Context(), runID)`.

### `internal/daemon/daemon.go` (MODIFIED)

`executeTask` gets two new phases. New helpers needed:

```go
// localizeInputs downloads s3:// File inputs to stagingDir, returns rewritten inputs map.
func localizeInputs(ctx context.Context, inputsData []byte, stagingDir string, backend storage.Backend) (map[string]interface{}, error)

// delocalizeOutputs uploads File outputs to backend, returns outputs map with s3:// paths.
func delocalizeOutputs(ctx context.Context, outputs map[string]values.Value,
    backend storage.Backend, runID, taskName string, shardIndex int) (map[string]values.Value, error)

// newStagingDir creates a temp dir for a task and returns it with a cleanup function.
func (d *Daemon) newStagingDir(taskID string) (dir string, cleanup func())
```

The daemon must read `inputs.json` via the backend now:

```go
inputsData, err := backend.ReadTaskInputs(taskCtx, claim.RunID, claim.TaskName, claim.ShardIndex)
```

(New method on Backend, or re-read from the address returned in the claim response — ReadTaskInputs is cleaner.)

### `wdl/task/inputs.go` (MODIFIED)

Single addition to `cleanPath`:

```go
import "strings"

func cleanPath(p string, wdlDir string) string {
    if strings.HasPrefix(p, "s3://") {
        return p  // opaque S3 URI — daemon localizes before RunTask
    }
    // ... existing logic ...
}
```

### `flowy/main.go` (MODIFIED)

`runRun` / `runSubmit`: add `--storage {local|s3}` flag, populate `SubmitRequest.StorageOverride`.

---

## S3Backend Implementation Notes

### AWS SDK for Go v2 configuration

```go
import (
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/credentials"
    "github.com/aws/aws-sdk-go-v2/service/s3"
)

func newS3Client(cfg StorageConfig) (*s3.Client, error) {
    opts := []func(*config.LoadOptions) error{
        config.WithRegion(cfg.Region),
    }
    if cfg.AccessKey != "" {
        opts = append(opts, config.WithCredentialsProvider(
            credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
        ))
    }
    awsCfg, err := config.LoadDefaultConfig(context.Background(), opts...)
    if err != nil {
        return nil, err
    }
    return s3.NewFromConfig(awsCfg, func(o *s3.Options) {
        if cfg.Endpoint != "" {
            o.BaseEndpoint = aws.String(cfg.Endpoint)
            o.UsePathStyle = true  // REQUIRED for MinIO compatibility
        }
    }), nil
}
```

`UsePathStyle: true` is required for MinIO. AWS S3 uses virtual-hosted style by default but accepts path style too, so this setting is safe for both.

### Streaming download (avoid OOM on large files)

```go
func (b *S3Backend) DownloadFile(ctx context.Context, addr string, localDir string) (string, error) {
    bucket, key := parseS3URI(addr)
    result, err := b.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &bucket, Key: &key})
    if err != nil {
        return "", err
    }
    defer result.Body.Close()

    localPath := filepath.Join(localDir, filepath.Base(key))
    f, err := os.Create(localPath)
    if err != nil {
        return "", err
    }
    defer f.Close()
    _, err = io.Copy(f, result.Body)  // stream, not ReadAll
    return localPath, err
}
```

### Key layout in S3

```
{prefix}/runs/{runID}/tasks/{taskName}_{shardIndex}/inputs.json
{prefix}/runs/{runID}/tasks/{taskName}_{shardIndex}/outputs.json
{prefix}/runs/{runID}/tasks/{taskName}_{shardIndex}/outputs/{filename}
{prefix}/runs/{runID}/outputs.json
```

This mirrors the local FS layout (`{baseDir}/runs/{runID}/tasks/...`) so the two backends are structurally equivalent.

---

## Scaling Considerations

| Concern | Local Backend | S3 Backend |
|---------|---------------|------------|
| Cross-host daemons | Requires NFS/POSIX shared mount | No shared FS needed — S3 is the medium |
| Large File inputs | In-place via bind mount (zero copy) | Downloaded to daemon local disk before run |
| Scatter parallelism | 100 concurrent local writes | 100 concurrent `PutObject` — S3 scales natively |
| Daemon disk | exec dir only per task | exec dir + staging dir for inputs (cleaned after delocalize) |
| Input re-use (reference files) | Instant (already on FS) | Re-downloaded per task — optional local cache deferred to v1.3 |

### Staging Directory Lifecycle

The cleanup function returned by `newStagingDir` must be called after `delocalizeOutputs` completes (not before). The exec dir created by `os.MkdirTemp` inside `RunTask` is separate and is not cleaned up by the daemon (same behavior as v1.1). Both are under a `baseDir` that operators can prune periodically.

---

## Anti-Patterns

### Anti-Pattern 1: S3 Awareness Inside `wdl/task`

**What people do:** Modify `PathMap.AddFile` to detect `s3://` and handle downloads inline; or add an S3 download step inside `RunTask`.

**Why it's wrong:** `wdl/task` is a pure execution library. Its only dependencies are stdlib and the `ast`/`eval`/`values` packages. Adding S3 SDK here introduces a heavy external dependency into a package that must remain testable without network access. Docker bind mounts also cannot accept S3 URIs directly.

**Do this instead:** All S3 localization belongs in the daemon's pre-`RunTask` phase. `wdl/task` stays clean.

### Anti-Pattern 2: Credentials in `TaskClaimResponse`

**What people do:** Include `AccessKey`/`SecretKey` in the claim response so the daemon can construct the S3 client from it.

**Why it's wrong:** The claim response travels over HTTP (potentially plain HTTP within a cluster). Any credential in a polling API response is a credential leak risk (logs, network capture, proxy).

**Do this instead:** The claim response carries only addressing config (bucket, endpoint, prefix). Credentials come from the daemon's own config file (`~/.flowy.yaml`) or from IAM instance role (environment variables / EC2 instance metadata). The `factory.go` `NewBackendFromClaim` combines addressing info from the claim with credentials from the daemon config.

### Anti-Pattern 3: `GetObject` → `ioutil.ReadAll` for File Downloads

**What people do:** Buffer the entire `GetObject` response body in memory before writing to disk.

**Why it's wrong:** Genomics files routinely exceed available RAM (multi-GB BAM files). `ReadAll` causes OOM on daemon hosts.

**Do this instead:** Stream from `GetObject` response body to `os.Create` via `io.Copy`. For uploads over ~100MB, use the S3 Transfer Manager (`github.com/aws/aws-sdk-go-v2/feature/s3/manager`) which handles automatic multipart upload.

### Anti-Pattern 4: S3 Operations in `handleClaim` Hot Path

**What people do:** Have `handleClaim` on the server read or write S3 to prepare task data as part of the claim transaction.

**Why it's wrong:** S3 operations add variable latency (10-100ms) to the claim hot path. Daemons poll every 1 second; adding S3 calls slows the entire polling loop and can cause cascading claim timeouts.

**Do this instead:** The server's `makeDispatchFn` writes `inputs.json` to S3 **before** creating the task record in SQLite. By the time the daemon claims the task, `inputs.json` is already in S3. `handleClaim` remains a pure SQLite operation.

### Anti-Pattern 5: Changing `OutputsDir`/`InputsDir` Semantics in `TaskClaimResponse`

**What people do:** Replace the existing `InputsDir`/`OutputsDir` string fields in `TaskClaimResponse` with S3 URIs.

**Why it's wrong:** The daemon's `executeTask` reads `inputs.json` from `claim.InputsDir`. Changing this to an S3 URI would require updating many call sites, risks breaking the local backend, and conflates addressing config with directory semantics.

**Do this instead:** Add new `StorageType`/`StorageBucket`/etc. fields alongside the existing dir fields. For the S3 backend the daemon ignores `InputsDir`/`OutputsDir` and uses the backend directly. For the local backend behavior is unchanged.

---

## Suggested Build Order

Dependency constraints drive this order:

1. `Backend` interface must exist before any component uses it.
2. `LocalBackend` must pass all existing spectool tests before `S3Backend` is written.
3. The daemon refactor (download/upload phases) must happen before S3Backend integration tests.

```
Step 1  internal/storage/backend.go
        Define Backend interface only. No implementations yet.
        Unblocks: all subsequent steps (type checking).

Step 2  internal/config — add StorageConfig struct + defaults
        Unblocks: server construction, factory constructor.

Step 3  internal/storage/local.go  (LocalBackend)
        internal/storage/factory.go  (NewBackendFromConfig / NewBackendFromClaim)
        Wire LocalBackend into Server (server.go, dispatch.go, handlers.go).
        Wire LocalBackend into Daemon (daemon.go — add empty localizeInputs/delocalizeOutputs).
        Gate: spectool 156/156 still passing.

Step 4  internal/api/types.go — add StorageOverride + storage fields to TaskClaimResponse
        handleClaim populates storage fields.
        cleanPath s3:// guard in wdl/task/inputs.go.
        Gate: spectool 156/156 still passing. No S3 paths in existing tests.

Step 5  internal/storage/s3.go  (S3Backend)
        Add github.com/aws/aws-sdk-go-v2/service/s3 to go.mod.
        Implement streaming DownloadFile / UploadFile.
        Implement ReadTaskInputs — server side also needs this for symmetry.
        Gate: unit tests with mocked S3 client.

Step 6  Daemon localizeInputs / delocalizeOutputs  (full S3 logic)
        Gate: integration tests against MinIO (docker-compose MinIO).

Step 7  Client --storage flag + SubmitRequest.StorageOverride
        MinIO-based spectool conformance run: 156/156 on S3 backend.
        Gate: full conformance on S3 backend.
```

Steps 1-4 are pure refactors: observable behavior on the local backend must be identical to v1.1. Step 5 is the first step introducing external network calls. Any conformance regression in steps 1-4 is attributable to the refactor, not S3. This sequencing makes debugging straightforward.

---

## Sources

- Direct codebase analysis: `internal/store/rundir.go`, `internal/server/dispatch.go`, `internal/server/handlers.go`, `internal/daemon/daemon.go`, `wdl/task/inputs.go`, `wdl/task/pathmap.go`, `wdl/task/task.go`, `internal/api/types.go`, `internal/config/config.go`
- [AWS SDK for Go v2 S3 package](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3) — HIGH confidence
- [MinIO with AWS SDK for Go — UsePathStyle requirement](https://docs.min.io/docs/how-to-use-aws-sdk-for-go-with-minio-server.html) — MEDIUM confidence (official MinIO docs)
- [Cromwell S3 file localization model](https://cromwell.readthedocs.io/en/stable/filesystems/Filesystems/) — HIGH confidence (industry-standard WDL executor, same localize-execute-delocalize pattern)
- [Cromwell FileLocalization.md](https://github.com/broadinstitute/cromwell/blob/develop/docs/optimizations/FileLocalization.md) — confirmed localize-before-Docker approach for S3

---

*Architecture research for: flowy v1.2 S3 object storage backend*
*Researched: 2026-03-12*
