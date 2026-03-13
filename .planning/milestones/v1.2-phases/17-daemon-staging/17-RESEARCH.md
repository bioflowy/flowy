# Phase 17: Daemon Staging - Research

**Researched:** 2026-03-13
**Domain:** Go daemon S3 localize/delocalize, API-based input/output delivery, WDL value walking
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**API-based input/output delivery**
- Inputs (parsed WDL values map) and WDL content embedded directly in TaskClaimResponse — daemon never reads inputs.json from storage
- Outputs sent via POST /tasks/{id}/result with outputs map — daemon never writes outputs.json to storage
- Both local and S3 modes use API delivery — consistent daemon behavior regardless of storage mode
- Server writes inputs into TaskClaimResponse at claim time; server reads outputs from result API and stores in SQLite
- This replaces the file-based inputs.json/outputs.json handoff entirely

**Staging directory layout**
- os.MkdirTemp("", "flowy-stage-") creates an isolated temp dir per task execution
- No configurable staging root — users control location via TMPDIR env var (standard Unix convention)
- Structure: stageDir/inputs/ for downloaded S3 files, container work dir for outputs
- Cleanup: os.RemoveAll(stageDir) immediately after successful delocalize

**Localize flow (S3 inputs → local POSIX paths)**
- Daemon receives inputs map from TaskClaimResponse API
- Walk WDL values to find all File/Directory references with s3:// URIs
- For each s3:// reference: ParseS3URI → backend.DownloadFile(key) → write to stageDir/inputs/
- Rewrite s3:// paths to local stageDir paths in-memory before passing to RunTask
- Non-s3:// paths (absolute local paths) passed through unchanged

**Delocalize flow (container outputs → S3)**
- Walk container work dir to discover output files
- For each file: compute relative path from work dir, construct S3 key as OutputsDir + "/" + relativePath
- Upload via backend.UploadFile with full s3:// URI construction (s3://bucket/prefix/key)
- Report outputs to server via result API with s3:// URIs as output paths
- Preserves directory structure in S3 (subdirs maintained via relative path)

**Output path format**
- S3 mode: full s3:// URIs (e.g. s3://mybucket/pfx/runs/r1/tasks/align_0/result.vcf)
- Local mode: absolute filesystem paths (unchanged from today)
- Consistent with Phase 15 input resolution rules (s3:// → S3 backend, /abs → local filesystem)

**Error handling**
- Localize failure: fail task immediately (AWS SDK v2 already retries 3x internally), clean up staging dir, report to server
- Delocalize failure: keep staging dir (output files preserved for recovery), report to server
- Fail on first error during delocalize (no aggregate errors) — already-uploaded files stay in S3 (no rollback)
- Error messages use phase-prefixed format: "localize: download s3://... : NoSuchKey" or "delocalize: upload runs/... : AccessDenied"

**Local-mode passthrough**
- When StorageType is "local": skip localize/delocalize entirely, run existing code path
- Early return branch: if StorageType == "s3" { localize → execute → delocalize } else { existing flow }
- Zero risk of breaking existing local behavior
- Input/output API delivery is shared between both modes (only S3 file staging is mode-specific)

### Claude's Discretion
- Internal structure of localize/delocalize functions
- How to walk WDL values to extract s3:// file references
- Backend construction from TaskClaimResponse storage fields
- How server populates Inputs and WDLContent in TaskClaimResponse
- Test structure and mock patterns for daemon S3 operations

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| DAEM-01 | Daemon downloads S3 input files to local staging directory before container execution (localize) | Covered by: staging dir pattern (os.MkdirTemp), walkInputsForS3 approach, S3Backend.DownloadFile, ParseS3URI already in storage package |
| DAEM-02 | Daemon uploads task output files from local directory to S3 after container execution (delocalize) | Covered by: S3Backend.UploadFile, UploadDirectory as reference, filepath.WalkDir pattern, full s3:// URI construction |
| DAEM-03 | Daemon cleans up local staging files after successful S3 upload | Covered by: os.RemoveAll(stageDir) — one call cleans everything; delocalize success → cleanup, failure → preserve |
| DAEM-04 | Daemon handles S3 upload/download failures with proper error reporting to server | Covered by: reportResult API (already exists), error wrapping with fmt.Errorf + %w, phase-prefixed error messages, AWS SDK v2 retries 3x internally |
</phase_requirements>

## Summary

Phase 17 implements the LED pattern (Localize-Execute-Delocalize) in the daemon. The core architectural change is dual: (1) API-based delivery replaces file-based inputs.json/outputs.json handoff, and (2) the daemon gains S3 staging logic that bridges between s3:// URIs in the inputs map and local POSIX paths that containers require.

The codebase is already well-prepared for this phase. `S3Backend` with `DownloadFile`/`UploadFile`/`UploadFileStream` is complete from Phase 16. `ParseS3URI` already lives in `internal/storage/backend.go`. `TaskClaimResponse` already carries `StorageType`, `StorageBucket`, `StorageEndpoint`, `StoragePrefix` from Phase 15. The daemon's `executeTask()` function is the single modification point; everything else threads through.

The two API contract changes are: (1) `TaskClaimResponse` gains `Inputs map[string]any` and `WDLContent string`; (2) `TaskResultRequest` gains `Outputs map[string]any`. The server's `handleClaim()` must populate these fields from the store, and `handleResult()` must persist `Outputs` instead of reading outputs.json from storage. The server's `waitForTask()` and `loadTaskOutputs()` in dispatch.go must be updated to receive outputs from the store record instead of reading a file.

**Primary recommendation:** Implement the LED pattern as three private functions (`localizeInputs`, `delocalizeOutputs`, `buildBackendFromClaim`) in `internal/daemon/daemon.go`; update API types and server handlers in a single coordinated wave.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `internal/storage` | project | S3Backend.DownloadFile/UploadFile, ParseS3URI | Already implemented in Phase 16, directly reusable |
| `os` stdlib | go 1.25 | MkdirTemp, RemoveAll, WriteFile, WalkDir | Standard POSIX file operations, no external dep |
| `path/filepath` | go 1.25 | Rel(), ToSlash() for relative path computation | Already used throughout project |
| `strings` | go 1.25 | HasPrefix("s3://") for URI detection in value walker | Project convention (ParseS3URI uses same) |
| `fmt` | go 1.25 | Errorf with %w for error chain preservation | Project-wide error wrapping convention |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `aws-sdk-go-v2/service/s3/transfermanager` | v0.1.9 | Multipart upload (S3Backend uses internally) | All uploads go through S3Backend — no direct use |
| `encoding/json` | stdlib | Marshal Inputs/Outputs in API types | API request/response serialization |
| `net/http/httptest` | stdlib | Mock server in daemon unit tests | Already used in daemon_test.go |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `os.WalkDir` for delocalize | `S3Backend.UploadDirectory` | UploadDirectory is already implemented and does exactly this — consider reusing it for delocalize instead of re-implementing walk |
| In-memory value rewriting | Separate staging manifest file | In-memory is simpler, avoids extra I/O, consistent with value-in/value-out project convention |
| New `store.Task.OutputsJSON` field | File-based outputs.json | API delivery (no new store field needed if server stores in SQLite runs.outputs_json or just as task-level field) — see Open Questions |

## Architecture Patterns

### Recommended Project Structure

No new files needed for the core implementation. All changes go into existing files:

```
internal/
├── api/types.go              # Add Inputs+WDLContent to TaskClaimResponse; add Outputs to TaskResultRequest
├── daemon/daemon.go          # Add localizeInputs, delocalizeOutputs, buildBackendFromClaim; refactor executeTask
├── server/handlers.go        # handleClaim: populate Inputs+WDLContent; handleResult: persist Outputs
└── server/dispatch.go        # waitForTask/loadTaskOutputs: receive outputs from store vs file read
```

For daemon unit tests, new test functions in `internal/daemon/daemon_test.go` following existing httptest patterns.

### Pattern 1: LED Execution Sequence

**What:** Localize → Execute → Delocalize wraps the existing `task.RunTask()` call in `executeTask()`.

**When to use:** Only when `claim.StorageType == "s3"`. Local mode falls through unchanged.

**Example:**
```go
// In executeTask(), after parsing WDL, before RunTask():
if claim.StorageType == "s3" {
    backend := buildBackendFromClaim(claim)

    stageDir, err := os.MkdirTemp("", "flowy-stage-")
    if err != nil {
        d.reportResult(taskCtx, claim.TaskID, "FAILED", fmt.Sprintf("localize: create stage dir: %v", err))
        return
    }

    localizedInputs, err := localizeInputs(claim.Inputs, backend, stageDir)
    if err != nil {
        os.RemoveAll(stageDir)
        d.reportResult(taskCtx, claim.TaskID, "FAILED", err.Error())
        return
    }

    outputs, err := task.RunTask(taskCtx, astTask, localizedInputs, ...)
    if err != nil {
        os.RemoveAll(stageDir)
        d.reportResult(taskCtx, claim.TaskID, "FAILED", fmt.Sprintf("run task: %v", err))
        return
    }

    s3Outputs, err := delocalizeOutputs(outputs, workDir, claim, backend)
    if err != nil {
        // keep stageDir for recovery
        d.reportResult(taskCtx, claim.TaskID, "FAILED", err.Error())
        return
    }
    os.RemoveAll(stageDir) // cleanup only on full success
    d.reportResultWithOutputs(taskCtx, claim.TaskID, "COMPLETED", "", s3Outputs)
    return
}
// local mode: existing flow continues below
```

### Pattern 2: WDL Value Walker for s3:// Detection

**What:** Recursively walk `map[string]any` (raw JSON inputs) to find and rewrite string values with `s3://` prefix. This operates on the raw JSON map (not typed values) since inputs arrive as `map[string]any` from the API.

**When to use:** In `localizeInputs` before passing to `task.RunTask`.

**Example:**
```go
// walkAndLocalize rewrites s3:// strings to local paths in a JSON value tree.
func walkAndLocalize(v any, backend storage.Backend, inputsDir string) (any, error) {
    switch val := v.(type) {
    case string:
        if strings.HasPrefix(val, "s3://") {
            localPath, err := downloadToStage(val, backend, inputsDir)
            if err != nil {
                return nil, fmt.Errorf("localize: download %s: %w", val, err)
            }
            return localPath, nil
        }
        return val, nil
    case []any:
        result := make([]any, len(val))
        for i, item := range val {
            rewritten, err := walkAndLocalize(item, backend, inputsDir)
            if err != nil {
                return nil, err
            }
            result[i] = rewritten
        }
        return result, nil
    case map[string]any:
        result := make(map[string]any, len(val))
        for k, item := range val {
            rewritten, err := walkAndLocalize(item, backend, inputsDir)
            if err != nil {
                return nil, err
            }
            result[k] = rewritten
        }
        return result, nil
    default:
        return val, nil // bool, float64, int64, nil — pass through
    }
}
```

### Pattern 3: Backend Construction from TaskClaimResponse

**What:** The daemon cannot use `storage.NewBackend(cfg)` directly since it doesn't have storage config in its own `Config`. It must construct a backend from the fields carried in `TaskClaimResponse`.

**When to use:** At the start of the S3 execution branch, before localizeInputs.

**Example:**
```go
// buildBackendFromClaim constructs an S3Backend from the storage config
// embedded in the claim response. The daemon reads its own AWS credentials
// from the environment / IAM role — NOT from the claim.
func buildBackendFromClaim(claim *api.TaskClaimResponse) (storage.Backend, error) {
    cfg := &config.Config{}
    cfg.Storage.Type     = claim.StorageType
    cfg.Storage.Bucket   = claim.StorageBucket
    cfg.Storage.Endpoint = claim.StorageEndpoint
    cfg.Storage.Prefix   = claim.StoragePrefix
    // MultipartThreshold uses default (100 MiB) — applyDefaults not called here,
    // so set explicitly.
    cfg.Storage.MultipartThreshold = 100 * 1024 * 1024
    b, err := storage.NewS3Backend(cfg)
    if err != nil {
        return nil, fmt.Errorf("build backend: %w", err)
    }
    return b, nil
}
```

### Pattern 4: Delocalize via WalkDir

**What:** Walk the container's work dir (mapped as `claim.OutputsDir` in S3 mode — actually the stageDir's work subdirectory after RunTask), upload each file to S3, and return a map of output names to `s3://` URIs.

**Key insight:** `task.RunTask()` returns typed `map[string]values.Value` outputs where `FileValue.Value` contains local POSIX paths (the container work dir path after UntranslateValue). These local paths must be converted to s3:// URIs for the result API.

**Example:**
```go
// delocalizeOutputs uploads all output files to S3 and rewrites FileValue paths
// to s3:// URIs. Returns a JSON-serializable map for the result API.
func delocalizeOutputs(
    outputs map[string]values.Value,
    workDir string,
    claim *api.TaskClaimResponse,
    backend storage.Backend,
) (map[string]any, error) {
    // Walk workDir, upload each file, build a path-rewrite table.
    localToS3 := make(map[string]string)
    err := filepath.WalkDir(workDir, func(filePath string, d os.DirEntry, err error) error {
        if err != nil || d.IsDir() {
            return err
        }
        rel, err := filepath.Rel(workDir, filePath)
        if err != nil {
            return fmt.Errorf("delocalize: compute rel path: %w", err)
        }
        keyRel := filepath.ToSlash(rel)
        s3Key := claim.OutputsDir + "/" + keyRel
        f, err := os.Open(filePath)
        if err != nil {
            return fmt.Errorf("delocalize: open %s: %w", filePath, err)
        }
        defer f.Close()
        if err := backend.UploadFileStream(s3Key, f); err != nil {
            return fmt.Errorf("delocalize: upload %s: %w", s3Key, err)
        }
        s3URI := "s3://" + claim.StorageBucket + "/" + s3Key
        localToS3[filePath] = s3URI
        return nil
    })
    if err != nil {
        return nil, err
    }

    // Rewrite FileValue/DirectoryValue paths in outputs to s3:// URIs.
    jsonOutputs := make(map[string]any, len(outputs))
    for k, v := range outputs {
        jsonOutputs[k] = rewriteOutputPaths(v, localToS3)
    }
    return jsonOutputs, nil
}
```

### Pattern 5: API-based Input/Output Delivery

**What:** Replace file-based inputs.json reading and outputs.json writing with fields in the HTTP API DTOs.

**Server side (handleClaim):** Populate `Inputs` and `WDLContent` from the stored task's run data. The run already has `InputsJSON` in SQLite; WDL content must be read from the WDL file path stored in `run.WDLPath`.

**Daemon side (executeTask):** Read inputs from `claim.Inputs` (already deserialized); parse WDL from `claim.WDLContent` using `parser.ParseFromString` or equivalent — **this requires checking whether the parser supports content strings, not just file paths**. If not, write WDL content to a temp file.

**Server side (handleResult):** Persist `req.Outputs` as JSON into the task or run record. Server's `waitForTask`/`loadTaskOutputs` must then read from the store rather than from a file.

### Anti-Patterns to Avoid

- **Don't construct S3Backend with `storage.NewBackend(cfg)`** from the daemon's own config: the daemon may be configured for local storage, but individual tasks may be S3 tasks. Use `buildBackendFromClaim` instead.
- **Don't use `filepath.Join` for S3 key construction:** S3 keys use `/` separators. Use string concatenation or `path.Join` (not `filepath`). The project pattern uses explicit string join (see `keyJoin` in s3.go).
- **Don't clean up stageDir on delocalize failure:** output files must be preserved for operator recovery. Only clean up on full success.
- **Don't aggregate delocalize errors:** fail on first error (project decision). Already-uploaded files stay in S3 with no rollback.
- **Don't read WDL from disk in the daemon** after this phase: WDL content arrives in `TaskClaimResponse.WDLContent`. The current `parser.Parse(claim.WDLPath, nil)` call must be replaced.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| S3 upload/download | Custom HTTP multipart code | `storage.S3Backend.UploadFile/DownloadFile/UploadFileStream` | Already implemented in Phase 16 with transfermanager, retry, MinIO compat |
| S3 URI parsing | Custom string parsing | `storage.ParseS3URI(uri)` | Already handles empty bucket, missing key, scheme validation |
| Directory walk for delocalize | Custom recursive walk | `filepath.WalkDir` | stdlib, already used in `S3Backend.UploadDirectory` |
| Backend from config | Manual SDK construction | `storage.NewS3Backend(cfg)` | Already handles region defaults, MinIO path style, credential loading |
| WDL value walking | Rewriting `values.Value` types directly | Walk raw `map[string]any` from API inputs | Simpler — inputs arrive as JSON maps before type-binding; no need to duplicate PathMap logic |

**Key insight:** `S3Backend.UploadDirectory` already implements the walkDir + upload loop. The delocalize operation is equivalent to `UploadDirectory(outputsKeyPrefix, workDir)` — verify whether this can be called directly rather than implementing a custom walker.

## Common Pitfalls

### Pitfall 1: WDL File Parsing in Daemon — No File Path After API Delivery
**What goes wrong:** `parser.Parse(claim.WDLPath, nil)` reads from disk. After API delivery, `WDLPath` may not exist on the daemon host (S3 deployment). WDLContent arrives as a string in the claim.
**Why it happens:** Current daemon code assumes shared filesystem; Phase 17 breaks that assumption.
**How to avoid:** Check whether `parser.ParseFromString(content, nil)` or equivalent exists. If not, write WDLContent to `os.CreateTemp("", "flowy-wdl-*.wdl")` and parse from there; delete after parse.
**Warning signs:** Daemon fails with "no such file or directory" on WDL parse when using S3 storage.

### Pitfall 2: filepath.Join Corrupts S3 Keys
**What goes wrong:** Using `filepath.Join(outputsDir, relPath)` on Linux works but introduces OS-specific separators on Windows or normalizes `//`.
**Why it happens:** S3 keys are URL-style paths; filepath uses OS separators.
**How to avoid:** Use `path.Join` (the path package, not filepath) or explicit `outputsDir + "/" + keyRel`. The project's `keyJoin()` in s3.go uses `strings.Join(parts, "/")` — follow that pattern.
**Warning signs:** S3 key contains `//` or backslashes.

### Pitfall 3: Staging Directory Lifetime vs. Error Paths
**What goes wrong:** stageDir is created before localize but cleanup happens only in certain code paths. If RunTask fails, stageDir leaks.
**Why it happens:** Multiple early-return paths after stageDir creation.
**How to avoid:** Use `defer os.RemoveAll(stageDir)` unconditionally, but call `os.RemoveAll` explicitly and early on delocalize failure (after the defer, so it doesn't run twice — use a bool flag). Or: use defer with a `keepStage` variable: `defer func() { if !keepStage { os.RemoveAll(stageDir) } }()`. Set `keepStage = true` only when delocalize fails.
**Warning signs:** Accumulating temp dirs in TMPDIR after task failures.

### Pitfall 4: s3:// URI vs. S3 Key in DownloadFile
**What goes wrong:** `backend.DownloadFile` takes an S3 key (not a full s3:// URI). If you pass the full URI, GetObject fails.
**Why it happens:** `S3Backend.DownloadFile(path)` uses `path` directly as the Key field in GetObjectInput — bucket is already stored in the backend struct.
**How to avoid:** Call `ParseS3URI(uri)` to extract key, verify the bucket matches `claim.StorageBucket`, then call `backend.DownloadFile(key)`.
**Warning signs:** S3 NoSuchKey error for a key that starts with "s3://".

### Pitfall 5: Store Output Persistence — Missing Migration or Field
**What goes wrong:** `TaskResultRequest.Outputs` arrives at the server but there's nowhere to store it in the current schema. The tasks table has no `outputs_json` column; the store interface has no `UpdateTaskOutputs` method.
**Why it happens:** The current design stores outputs only in the file-based outputs.json; the API-based delivery requires a database column.
**How to avoid:** Add a goose migration `00002_task_outputs.sql` adding `outputs_json TEXT` to the tasks table. Add `UpdateTaskOutputs(ctx, taskID, json)` to the store interface and SQLite implementation. Update `loadTaskOutputs` in dispatch.go to read from the store.
**Warning signs:** Server `handleResult` receives outputs but `waitForTask` returns empty map.

### Pitfall 6: WDL Content vs. WDL Path — Daemon Needs Both or Either
**What goes wrong:** If `WDLContent` is empty (local mode with shared FS), the daemon must fall back to `WDLPath`. If `WDLPath` is empty (future S3-only), the daemon must use `WDLContent`.
**Why it happens:** Transitional design — both fields exist but either may be empty.
**How to avoid:** In executeTask: if `WDLContent != ""`, parse from content (temp file or direct); else parse from `WDLPath`. This keeps backward compatibility with local mode.

## Code Examples

Verified patterns from existing codebase:

### S3Backend.DownloadFile (existing, Phase 16)
```go
// Source: internal/storage/s3.go
func (b *S3Backend) DownloadFile(path string) ([]byte, error) {
    var buf bytes.Buffer
    if err := b.DownloadFileStream(path, &buf); err != nil {
        return nil, err
    }
    return buf.Bytes(), nil
}
```
Note: `path` is the S3 key, NOT a full s3:// URI. Use `ParseS3URI` first to extract key.

### ParseS3URI (existing, Phase 14)
```go
// Source: internal/storage/backend.go
bucket, key, err := storage.ParseS3URI("s3://mybucket/runs/r1/tasks/align_0/input.bam")
// → bucket="mybucket", key="runs/r1/tasks/align_0/input.bam"
```

### NewS3Backend (existing, Phase 16)
```go
// Source: internal/storage/s3.go
// cfg.Storage.{Bucket, Endpoint, Region, Prefix, MultipartThreshold} must be set
b, err := storage.NewS3Backend(cfg)
```

### UploadDirectory (existing, Phase 16 — potential reuse for delocalize)
```go
// Source: internal/storage/s3.go
// Walks localSrc, uploads each file to S3 prefix path, preserves relative structure
func (b *S3Backend) UploadDirectory(path string, localSrc string) error
```
The delocalize operation is equivalent to `b.UploadDirectory(outputsKeyPrefix, workDir)`. Consider calling this directly rather than re-implementing the walk — the difference is that delocalize also needs to rewrite output paths in the outputs map.

### Existing httptest mock pattern (daemon_test.go)
```go
// Source: internal/daemon/daemon_test.go
srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path == "/tasks/claim" {
        json.NewEncoder(w).Encode(api.TaskClaimResponse{...})
    }
}))
defer srv.Close()
d := newTestDaemon(t, srv.URL)
```

### Store task outputs — new migration pattern needed
```sql
-- migrations/00002_task_outputs.sql
-- +goose Up
ALTER TABLE tasks ADD COLUMN outputs_json TEXT;

-- +goose Down
-- SQLite does not support DROP COLUMN in older versions; no-op for rollback
```

### Error wrapping pattern (project-wide)
```go
// Project convention: fmt.Errorf with %w preserves error chain
return nil, fmt.Errorf("localize: download %s: %w", uri, err)
return nil, fmt.Errorf("delocalize: upload %s: %w", s3Key, err)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| File-based inputs.json (daemon reads from shared FS) | API delivery: Inputs in TaskClaimResponse | Phase 17 | Daemon no longer needs shared filesystem access for inputs |
| File-based outputs.json (daemon writes to shared FS) | API delivery: Outputs in TaskResultRequest | Phase 17 | Daemon no longer writes outputs.json; server receives via HTTP |
| WDL parsed from file path on shared FS | WDL content embedded in TaskClaimResponse | Phase 17 | Daemon can execute on hosts without access to WDL file |
| No staging (shared FS only) | LED pattern: per-task temp dir | Phase 17 | S3 inputs materialized locally, containers always see POSIX paths |

**Deprecated/outdated after Phase 17:**
- `claim.InputsDir` field: no longer used by daemon (inputs come from `claim.Inputs`)
- `claim.OutputsDir` field: becomes the S3 key prefix for delocalize output keys (semantics change)
- `dispatch.go:loadTaskOutputs()`: will read from store instead of file download
- `daemon.go`: reading `inputs.json` via `os.ReadFile` and parsing WDL from `claim.WDLPath` via `parser.Parse`
- `server/handlers.go:handleTaskRun()`: writes inputs.json to backend — this path also changes

## Open Questions

1. **Does parser support parsing from string content?**
   - What we know: `parser.Parse(path, nil)` reads from disk. `parser.ParseWithSearchPaths` also reads from disk.
   - What's unclear: Is there a `parser.ParseFromString(content, searchPaths)` or `parser.ParseFromReader` function?
   - Recommendation: Check `wdl/parser/` package. If absent, add a thin wrapper that writes to a temp file and parses it; or add `ParseFromString(content string, searchPaths []string) (*ast.Document, error)` to the parser package. Given WDL content arrives as a string in the claim, this is needed.

2. **Where to persist task outputs in SQLite?**
   - What we know: Current tasks table has no `outputs_json` column. `store.Task` struct has no `OutputsJSON` field. The store interface has no `UpdateTaskOutputs` method.
   - What's unclear: Should outputs live on the task row or the run row? Task row is more granular and supports per-task output access.
   - Recommendation: Add `outputs_json TEXT` to tasks table via goose migration `00002`. Add `UpdateTaskOutputs(ctx, taskID int64, outputsJSON string) error` to Store interface + SQLite impl. Update `store.Task` struct with `OutputsJSON string`. Update `loadTaskOutputs` in dispatch.go to unmarshal from `t.OutputsJSON`.

3. **handleTaskRun inputs.json write — must also change**
   - What we know: `handleTaskRun` in handlers.go also writes inputs.json via `s.backend.UploadFile`. This path bypasses dispatch.go's makeDispatchFn.
   - What's unclear: Does Phase 17 change `handleTaskRun` or only workflow-dispatched tasks?
   - Recommendation: Change `handleTaskRun` to store inputs in the Task record instead of file. The task's inputs can be stored in a new `inputs_json TEXT` column on the tasks table (same migration), or retrieved from the run's `inputs_json`. Simplest: store the task's inputs in `store.Task.InputsJSON`.

4. **WDLContent size — is it safe to embed in API response?**
   - What we know: WDL files for conformance tests are small (< 10 KB typical). TaskClaimResponse is a JSON HTTP response body.
   - What's unclear: Are there WDL workflows with very large content that could bloat the response?
   - Recommendation: Acceptable for v1.2 scope. No streaming needed for claim response.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go standard `testing` package (go 1.25) |
| Config file | none — go test ./... |
| Quick run command | `go test ./internal/daemon/...` |
| Full suite command | `go test ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DAEM-01 | S3 inputs are downloaded to stageDir before container execution | unit | `go test ./internal/daemon/... -run TestLocalizeInputs` | Wave 0 |
| DAEM-01 | s3:// URIs in nested arrays/maps are rewritten to local paths | unit | `go test ./internal/daemon/... -run TestWalkAndLocalize` | Wave 0 |
| DAEM-02 | Output files are uploaded to S3 after container exit with s3:// URIs in result | unit | `go test ./internal/daemon/... -run TestDelocalizeOutputs` | Wave 0 |
| DAEM-02 | Outputs map in TaskResultRequest contains s3:// URIs | unit | `go test ./internal/daemon/... -run TestReportResultWithOutputs` | Wave 0 |
| DAEM-03 | stageDir removed after successful delocalize | unit | `go test ./internal/daemon/... -run TestStageDirCleanupOnSuccess` | Wave 0 |
| DAEM-03 | stageDir preserved when delocalize fails | unit | `go test ./internal/daemon/... -run TestStageDirPreservedOnDelocalizeFailure` | Wave 0 |
| DAEM-04 | Localize download failure reported as FAILED with descriptive message | unit | `go test ./internal/daemon/... -run TestLocalizeFailureReported` | Wave 0 |
| DAEM-04 | Delocalize upload failure reported as FAILED with descriptive message | unit | `go test ./internal/daemon/... -run TestDelocalizeFailureReported` | Wave 0 |
| DAEM-01..04 | API changes: TaskClaimResponse.Inputs/WDLContent, TaskResultRequest.Outputs | unit | `go test ./internal/api/... -run TestAPITypes` (if added) | Wave 0 |
| DAEM-01..04 | Server handleClaim populates Inputs and WDLContent | unit | `go test ./internal/server/... -run TestHandleClaim_Inputs` | Wave 0 |
| DAEM-01..04 | Server handleResult persists Outputs from API | unit | `go test ./internal/server/... -run TestHandleResult_Outputs` | Wave 0 |

All test cases above are in Wave 0 (new files needed). Mock S3 backend pattern from `internal/storage/s3_test.go` (Phase 16) should be reused: inject `mockS3APIClient` and `mockTransferManager` via the `newS3BackendForTest` constructor.

### Sampling Rate
- **Per task commit:** `go test ./internal/daemon/...`
- **Per wave merge:** `go test ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `internal/daemon/daemon_test.go` — add new test functions for localize/delocalize/staging (extend existing file)
- [ ] `internal/server/handlers_test.go` — new test file if absent, or extend for handleClaim/handleResult API changes
- [ ] `internal/store/sqlite/migrations/00002_task_outputs.sql` — migration for outputs_json column
- [ ] `internal/store/sqlite/tasks.go` — add `UpdateTaskOutputs` method
- [ ] No new framework install needed — `go test` already works

## Sources

### Primary (HIGH confidence)
- Direct code inspection: `internal/daemon/daemon.go` — full executeTask() flow analyzed
- Direct code inspection: `internal/storage/s3.go` — DownloadFile, UploadFile, UploadDirectory, UploadFileStream confirmed ready
- Direct code inspection: `internal/storage/backend.go` — ParseS3URI confirmed, Backend interface complete
- Direct code inspection: `internal/api/types.go` — TaskClaimResponse/TaskResultRequest current state confirmed
- Direct code inspection: `internal/server/handlers.go` — handleClaim/handleResult complete flow
- Direct code inspection: `internal/server/dispatch.go` — loadTaskOutputs, waitForTask, makeDispatchFn flow
- Direct code inspection: `wdl/task/pathmap.go` — TranslateValue, UntranslateValue patterns for value walking
- Direct code inspection: `internal/store/sqlite/migrations/00001_init.sql` — confirmed no outputs_json column
- Direct code inspection: `internal/store/sqlite/tasks.go` — scanTask, UpdateTaskState confirmed

### Secondary (MEDIUM confidence)
- CONTEXT.md decisions — locked by user in /gsd:discuss-phase session 2026-03-13

### Tertiary (LOW confidence)
- parser package `ParseFromString` availability — not verified, marked as Open Question

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries confirmed by direct code inspection; all reused from prior phases
- Architecture: HIGH — full code read of all integration points; patterns derived from existing code
- Pitfalls: HIGH for pitfalls 1-5 (code-verified); MEDIUM for pitfall 6 (design inference)
- Open questions: 4 flagged, all resolvable during plan execution

**Research date:** 2026-03-13
**Valid until:** 2026-04-13 (stable internal codebase; only Go stdlib and AWS SDK v2 involved)
