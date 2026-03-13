# Phase 14: Storage Abstraction - Research

**Researched:** 2026-03-12
**Domain:** Go interface design, filesystem abstraction, internal package structure
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- Interface granularity: whole-file operations — Upload(path, []byte), Download(path) []byte, ReadJSON(path, &v), WriteJSON(path, v)
- NOT stream-based (io.Reader/io.Writer) — keep callers simple
- Path-building helpers (RunInputsDir, TaskDir, etc.) included in the Backend interface — encapsulates both local FS paths and S3 key layout
- Separate methods for File and Directory types — WDL distinguishes these, so the interface should too (UploadFile + UploadDirectory / DownloadFile + DownloadDirectory)
- Multipart upload threshold configurable in YAML (e.g. `storage.multipart_threshold: 100MB`) with a sensible default (e.g. 64MB)
- CreateRunDirs/CreateTaskDirs create zero-byte marker objects on S3 (e.g. `runs/RUN_ID/.marker`) so ListObjects shows the "directory"
- WDL Directory type: recursive upload (each file individually under S3 key prefix), not tar/archive
- File listing uses SQLite tracking (record uploaded files in DB), not ListObjectsV2 prefix scan
- Cleanup uses soft delete: mark deleted in SQLite, keep S3 objects for recovery

### Claude's Discretion
- Exact Backend interface method signatures
- Internal package structure within internal/storage/
- How LocalBackend wraps rundir.go (thin delegation vs copy)
- ParseS3URI implementation details

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| STOR-01 | Storage Backend interface defined in internal/storage/ with methods for file upload, download, JSON read/write, and directory operations | Interface design section; method signatures derived from all call sites in dispatch.go, handlers.go, daemon.go |
| STOR-02 | LocalBackend implementation wraps existing shared filesystem code behind the Backend interface with zero behavior change | LocalBackend pattern section; all 5 rundir.go functions become thin delegates |
| STOR-03 | All existing spectool tests (156/156) continue to pass after LocalBackend refactor | Test infrastructure section; spectool is an end-to-end harness, all go unit tests (248 cases) also checked |
</phase_requirements>

## Summary

Phase 14 introduces a `Backend` interface in `internal/storage/` that abstracts all filesystem I/O currently scattered across `internal/server/dispatch.go`, `internal/server/handlers.go`, and `internal/daemon/daemon.go`. The existing five functions in `internal/store/rundir.go` (path computation + `os.MkdirAll`) become the core of `LocalBackend`. All callers are updated to receive a Backend via constructor injection, exactly mirroring how `store.Store` is already injected into `Server`.

The refactor is a pure rename/redirect — zero semantic change. `LocalBackend` delegates directly to the existing `os.*` calls already in place. The `ParseS3URI` helper is a small standalone function with table-driven tests, independent from the backend wiring.

The 156 spectool conformance tests run end-to-end against real binaries (server + daemon + flowy CLI). They will continue to pass because LocalBackend preserves identical filesystem layout and all I/O semantics. The 248 Go unit tests provide the inner safety net.

**Primary recommendation:** Create `internal/storage/` with `backend.go` (interface + ParseS3URI), `local.go` (LocalBackend), and `factory.go` (NewBackend). Inject Backend into Server via `server.New()`, update the three call sites in handlers.go and dispatch.go. Phase is purely mechanical refactoring — no new behavior.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| stdlib `os` | go 1.25 | File I/O in LocalBackend | Already in use across all three call sites |
| stdlib `encoding/json` | go 1.25 | JSON marshal/unmarshal in ReadJSON/WriteJSON | Already used for inputs.json and outputs.json |
| stdlib `path/filepath` | go 1.25 | Path construction in LocalBackend | Already used in rundir.go |
| stdlib `net/url` or string parsing | go 1.25 | ParseS3URI implementation | No third-party S3 URI parser needed |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `gopkg.in/yaml.v3` | already in go.mod | StorageConfig (multipart_threshold) in config.go | When adding storage.multipart_threshold to Config struct |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Whole-file `[]byte` methods | `io.Reader`/`io.Writer` streams | Streams avoid buffering large files in memory but make callers significantly more complex; deferred to PERF-01 |
| `internal/storage/` new package | Extending `internal/store/` | store already has a semantic boundary (SQLite persistence); mixing filesystem abstraction would muddy it |

**Installation:** No new dependencies needed for Phase 14.

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── storage/
│   ├── backend.go     # Backend interface + ParseS3URI helper
│   ├── local.go       # LocalBackend struct implementing Backend
│   └── factory.go     # NewBackend(cfg) — returns LocalBackend for now
├── store/
│   ├── rundir.go      # KEEP: still used directly in LocalBackend
│   ├── store.go       # unchanged
│   └── sqlite/        # unchanged
├── server/
│   ├── server.go      # add `backend storage.Backend` field; update New()
│   ├── dispatch.go    # replace store.CreateTaskDirs / store.TaskDir / os.WriteFile / os.ReadFile calls
│   └── handlers.go    # replace store.CreateRunDirs / os.ReadFile calls
└── config/
    └── config.go      # add StorageConfig struct with MultipartThreshold
```

### Pattern 1: Backend Interface (mirrors store.Store pattern)

**What:** Define Backend as a pure Go interface in `internal/storage/backend.go`. LocalBackend is the concrete type in `internal/storage/local.go`. Factory in `factory.go` constructs the right implementation.

**When to use:** This is the single pattern for all storage access in Phase 14. S3Backend will implement the same interface in Phase 16.

**Example:**
```go
// internal/storage/backend.go
package storage

// Backend abstracts all filesystem operations for workflow run data.
// Implementations must be safe for concurrent use.
// Path arguments use the same layout as store.TaskDir / store.RunInputsDir
// for LocalBackend; S3Backend maps them to object key prefixes.
type Backend interface {
    // Path helpers (same signatures as store package functions)
    RunInputsDir(baseDir, runID string) string
    RunOutputsDir(baseDir, runID string) string
    TaskDir(baseDir, runID, taskName string, shardIndex int) string

    // Directory lifecycle
    CreateRunDirs(baseDir, runID string) error
    CreateTaskDirs(baseDir, runID, taskName string, shardIndex int) error

    // File I/O
    UploadFile(path string, data []byte) error
    DownloadFile(path string) ([]byte, error)
    UploadDirectory(path string, localSrc string) error
    DownloadDirectory(path string, localDst string) error

    // Structured JSON I/O (convenience wrappers over UploadFile/DownloadFile)
    WriteJSON(path string, v interface{}) error
    ReadJSON(path string, v interface{}) error
}
```

**Example — LocalBackend:**
```go
// internal/storage/local.go
package storage

import (
    "encoding/json"
    "fmt"
    "os"

    "github.com/bioflowy/flowy/internal/store"
)

// LocalBackend implements Backend using the local filesystem.
// It delegates path computation to internal/store/rundir.go and uses
// os.* for all I/O — zero behavior change from pre-Phase-14 code.
type LocalBackend struct{}

func (b *LocalBackend) RunInputsDir(baseDir, runID string) string {
    return store.RunInputsDir(baseDir, runID)
}

func (b *LocalBackend) RunOutputsDir(baseDir, runID string) string {
    return store.RunOutputsDir(baseDir, runID)
}

func (b *LocalBackend) TaskDir(baseDir, runID, taskName string, shardIndex int) string {
    return store.TaskDir(baseDir, runID, taskName, shardIndex)
}

func (b *LocalBackend) CreateRunDirs(baseDir, runID string) error {
    return store.CreateRunDirs(baseDir, runID)
}

func (b *LocalBackend) CreateTaskDirs(baseDir, runID, taskName string, shardIndex int) error {
    return store.CreateTaskDirs(baseDir, runID, taskName, shardIndex)
}

func (b *LocalBackend) UploadFile(path string, data []byte) error {
    if err := os.WriteFile(path, data, 0o644); err != nil {
        return fmt.Errorf("storage: write %q: %w", path, err)
    }
    return nil
}

func (b *LocalBackend) DownloadFile(path string) ([]byte, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("storage: read %q: %w", path, err)
    }
    return data, nil
}

func (b *LocalBackend) UploadDirectory(path string, localSrc string) error {
    // For LocalBackend, path IS localSrc — no-op if same, or os.Rename.
    // In Phase 14, callers always use the path returned by TaskDir,
    // which is already created by CreateTaskDirs. No-op is correct.
    return nil
}

func (b *LocalBackend) DownloadDirectory(path string, localDst string) error {
    // Same reasoning as UploadDirectory. No-op for local.
    return nil
}

func (b *LocalBackend) WriteJSON(path string, v interface{}) error {
    data, err := json.Marshal(v)
    if err != nil {
        return fmt.Errorf("storage: marshal JSON for %q: %w", path, err)
    }
    return b.UploadFile(path, data)
}

func (b *LocalBackend) ReadJSON(path string, v interface{}) error {
    data, err := b.DownloadFile(path)
    if err != nil {
        return err
    }
    if err := json.Unmarshal(data, v); err != nil {
        return fmt.Errorf("storage: unmarshal JSON from %q: %w", path, err)
    }
    return nil
}
```

### Pattern 2: Constructor Injection (mirrors store injection into Server)

**What:** Add `backend storage.Backend` field to `Server` struct. Pass it into `server.New()`. Callers of server.New (cmd/flowy-server/main.go) construct the backend via `storage.NewBackend(cfg)`.

**Example:**
```go
// internal/server/server.go — updated New()
func New(cfg *config.Config, st store.Store, backend storage.Backend) *Server {
    s := &Server{
        cfg:     cfg,
        store:   st,
        backend: backend,
        // ...
    }
    // ...
}
```

```go
// internal/storage/factory.go
package storage

import "github.com/bioflowy/flowy/internal/config"

// NewBackend returns the appropriate Backend for the given config.
// In Phase 14 only LocalBackend is supported.
func NewBackend(cfg *config.Config) Backend {
    return &LocalBackend{}
}
```

### Pattern 3: ParseS3URI

**What:** A standalone pure function in `internal/storage/backend.go` that parses `s3://bucket/key` URIs. No external dependencies.

**Example:**
```go
// ParseS3URI parses an S3 URI of the form s3://bucket/key.
// Returns an error if the URI scheme is not "s3", if the bucket is empty,
// or if the key is empty (a trailing slash does not count as a key).
func ParseS3URI(uri string) (bucket, key string, err error) {
    const prefix = "s3://"
    if !strings.HasPrefix(uri, prefix) {
        return "", "", fmt.Errorf("storage: not an S3 URI: %q", uri)
    }
    rest := uri[len(prefix):]
    idx := strings.IndexByte(rest, '/')
    if idx < 0 || idx == 0 {
        return "", "", fmt.Errorf("storage: S3 URI missing bucket: %q", uri)
    }
    bucket = rest[:idx]
    key = rest[idx+1:]
    if key == "" {
        return "", "", fmt.Errorf("storage: S3 URI missing key (trailing slash only): %q", uri)
    }
    return bucket, key, nil
}
```

**Test cases required (table-driven):**
- `s3://mybucket/path/to/file` — valid
- `s3://mybucket/file.txt` — valid (single-level key)
- `s3://` — error: missing bucket
- `s3:///key` — error: empty bucket
- `s3://bucket/` — error: trailing slash, empty key
- `s3://bucket` — error: no slash after bucket (no key)
- `http://bucket/key` — error: wrong scheme
- `""` — error: empty string

### Anti-Patterns to Avoid

- **Duplicating rundir.go logic:** LocalBackend must delegate to `store.RunInputsDir`, `store.RunOutputsDir`, `store.TaskDir`, `store.CreateRunDirs`, and `store.CreateTaskDirs` — never copy-paste the path-building logic. This keeps a single source of truth until those functions are retired in a later phase.
- **Importing `internal/storage` from `internal/store`:** Would create a circular dependency. The flow is: `store` has no deps on `storage`; `storage/local.go` imports `store`.
- **Changing daemon.go in Phase 14:** The daemon does not use Backend in Phase 14 — it receives absolute local paths via `TaskClaimResponse.InputsDir` / `TaskClaimResponse.OutputsDir`. Daemon integration is Phase 17. Do not inject Backend into the daemon struct yet.
- **Making the Backend interface context-aware (ctx context.Context):** The locked decisions use value-in/value-out signatures without context. Adding context parameters would invalidate the interface design decisions. (Can be revisited in Phase 16 if S3 needs cancellation.)

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON read/write | Custom JSON helper | `WriteJSON`/`ReadJSON` on Backend interface | Already encapsulated; callers get free S3 compatibility |
| S3 URI parsing | Full URL parser | Simple `strings.HasPrefix` + `strings.IndexByte` | S3 URIs are structurally simple; `net/url` is overkill and normalizes paths in ways that corrupt S3 keys |
| Directory existence check on S3 | `ListObjectsV2` scan | Marker objects (`.marker`) + SQLite tracking | Locked decision; ListObjectsV2 is eventually consistent and expensive |

**Key insight:** Phase 14 is mechanical refactoring. All complexity — S3 operations, multipart upload, path corruption fixes — belongs in later phases. The job here is building the injection seam, not implementing new behavior.

## Common Pitfalls

### Pitfall 1: Breaking Server Tests by Changing New() Signature

**What goes wrong:** `internal/server/server_test.go` calls `server.New(cfg, store)`. Adding a third `backend` parameter breaks compilation.

**Why it happens:** Test files that construct a Server directly (for integration tests) will not compile after the signature change.

**How to avoid:** Update server_test.go at the same time as server.go. Pass `storage.NewBackend(cfg)` (or `&storage.LocalBackend{}`) in test constructors.

**Warning signs:** Compilation error `too few arguments in call to server.New` in test files.

### Pitfall 2: Circular Import Between storage and store

**What goes wrong:** If `internal/storage/backend.go` imports `internal/store` for type definitions, and `internal/store/rundir.go` imports `internal/storage`, Go will refuse to compile.

**Why it happens:** LocalBackend needs to call `store.CreateRunDirs` etc., which means `storage` imports `store`. If `store` were to import `storage`, that's the cycle.

**How to avoid:** The dependency arrow must be one-way: `storage` -> `store`. Never add an import of `storage` inside the `store` package.

**Warning signs:** `import cycle not allowed` build error.

### Pitfall 3: dispatch.go Uses filepath.Join After Backend Refactor

**What goes wrong:** After switching to `s.backend.TaskDir(...)`, a developer still uses `filepath.Join(taskDir, "inputs.json")` to construct the file path, then calls `os.WriteFile` directly. This bypasses the Backend entirely.

**Why it happens:** The old code pattern `filepath.Join(taskDir, "inputs.json") + os.WriteFile` is deeply familiar.

**How to avoid:** Replace ALL `os.WriteFile`/`os.ReadFile` calls in dispatch.go and handlers.go with `s.backend.UploadFile` / `s.backend.DownloadFile`. The path passed in must come from `s.backend.TaskDir(...)`.

**Warning signs:** Any remaining `os.WriteFile` or `os.ReadFile` import in dispatch.go or handlers.go after the refactor.

### Pitfall 4: daemon.go Still Uses Direct os.* Calls

**What goes wrong:** Trying to inject Backend into daemon.go in Phase 14 causes scope creep and may break daemon tests.

**Why it happens:** The daemon also uses `os.ReadFile`/`os.WriteFile` for inputs.json / outputs.json (lines 166 and 235 in daemon.go).

**How to avoid:** Leave daemon.go unchanged in Phase 14. The daemon receives absolute local paths from the server via `TaskClaimResponse`, so it continues to use `os.*` directly. Backend injection into the daemon is Phase 17.

**Warning signs:** Any modification to `internal/daemon/daemon.go` or `daemon_test.go`.

### Pitfall 5: spec tool Tests Check Binary Behavior End-to-End

**What goes wrong:** A subtle behavior change in path construction (e.g., different path separator on edge cases) causes spectool tests to fail with opaque errors.

**Why it happens:** spectool runs full workflows end-to-end. If any path is wrong, the task silently reads/writes the wrong location and produces unexpected outputs.

**How to avoid:** LocalBackend must produce byte-for-byte identical paths to the functions it wraps. Verify with the existing `rundir_test.go` tests — they exercise all 5 functions. Add parallel tests in `internal/storage/` that call LocalBackend methods and compare to the old store functions.

**Warning signs:** spectool test failures after the refactor; `go test ./internal/store/...` still passing but `go test ./internal/storage/...` showing wrong paths.

## Code Examples

### All os.ReadFile / os.WriteFile Call Sites to Replace (dispatch.go)

The following patterns in `dispatch.go` must be replaced:

```go
// BEFORE (dispatch.go ~line 47):
if err := os.WriteFile(inputsPath, inputsData, 0o644); err != nil { ... }

// AFTER:
if err := s.backend.UploadFile(filepath.Join(taskDir, "inputs.json"), inputsData); err != nil { ... }

// BEFORE (dispatch.go loadTaskOutputs ~line 108):
data, err := os.ReadFile(outputsPath)

// AFTER (in loadTaskOutputs, receives backend parameter):
data, err := backend.DownloadFile(outputsPath)
```

### All os.ReadFile Call Sites to Replace (handlers.go)

```go
// BEFORE (handlers.go handleOutputs ~line 73):
outputsPath := filepath.Join(s.cfg.BaseDir, "runs", run.ID, "outputs.json")
data, err := os.ReadFile(outputsPath)

// AFTER:
outputsPath := filepath.Join(s.backend.RunOutputsDir(s.cfg.BaseDir, run.ID), "..", "outputs.json")
// OR more cleanly:
execRoot := filepath.Join(s.cfg.BaseDir, "runs", run.ID)
data, err := s.backend.DownloadFile(filepath.Join(execRoot, "outputs.json"))
```

Note: `runWorkflow` and `waitForTaskAndFinalize` in dispatch.go also use `os.WriteFile` for the run-level `outputs.json`. These must also be replaced with `s.backend.UploadFile`.

### store.CreateRunDirs / store.CreateTaskDirs / store.TaskDir Call Sites (handlers.go)

```go
// BEFORE (handlers.go handleSubmit ~line 124):
if err := store.CreateRunDirs(s.cfg.BaseDir, runID); err != nil { ... }

// AFTER:
if err := s.backend.CreateRunDirs(s.cfg.BaseDir, runID); err != nil { ... }

// BEFORE (dispatch.go ~line 36):
if err := store.CreateTaskDirs(s.cfg.BaseDir, runID, t.Name, shardIndex); err != nil { ... }
taskDir := store.TaskDir(s.cfg.BaseDir, runID, t.Name, shardIndex)

// AFTER:
if err := s.backend.CreateTaskDirs(s.cfg.BaseDir, runID, t.Name, shardIndex); err != nil { ... }
taskDir := s.backend.TaskDir(s.cfg.BaseDir, runID, t.Name, shardIndex)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Direct `store.RunInputsDir` / `os.WriteFile` calls in server | Backend interface with LocalBackend | Phase 14 | All storage I/O routed through a single abstraction; S3Backend can be plugged in Phase 16 |
| `store` package owns both SQLite AND filesystem layout | `store` = SQLite persistence, `storage` = file I/O abstraction | Phase 14 | Cleaner single-responsibility; store package untouched |

**Deprecated/outdated after this phase:**
- Direct imports of `internal/store` in `server/dispatch.go` and `server/handlers.go` for `CreateRunDirs`, `CreateTaskDirs`, `TaskDir` — these are replaced by Backend method calls. The functions remain in `store/rundir.go` (used by LocalBackend) but server code no longer calls them directly.

## Open Questions

1. **Exact path for run-level outputs.json**
   - What we know: `runWorkflow` writes to `filepath.Join(execRoot, "outputs.json")` where execRoot = `{baseDir}/runs/{runID}`. `handleOutputs` reads from the same path.
   - What's unclear: The Backend interface as designed takes a `path string` for UploadFile/DownloadFile — the run-level outputs.json sits at the run root, not inside `RunOutputsDir` or `RunInputsDir`. The path must be constructed as `filepath.Join(s.cfg.BaseDir, "runs", run.ID, "outputs.json")` which is outside the named subdirs.
   - Recommendation: This is fine — UploadFile/DownloadFile accept any path string. LocalBackend will write it correctly. Document that this path is the run root outputs.json (not inside inputs/ or outputs/ subdirs).

2. **Should loadTaskOutputs receive Backend as a parameter or method?**
   - What we know: `loadTaskOutputs` is a standalone function (not a method) in dispatch.go. It currently takes (baseDir, runID, taskName, shardIndex).
   - What's unclear: Make it `(backend storage.Backend, baseDir, runID, taskName string, shardIndex int)` or convert to a Server method.
   - Recommendation: Add `backend storage.Backend` as first parameter. This keeps it testable without a full Server.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) |
| Config file | none — standard `go test` |
| Quick run command | `go test ./internal/storage/... ./internal/server/... ./internal/store/... -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements -> Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|--------------|
| STOR-01 | Backend interface defined with all required methods | compilation | `go build ./internal/storage/...` | Wave 0 |
| STOR-01 | ParseS3URI handles valid S3 URIs | unit | `go test ./internal/storage/... -run TestParseS3URI -count=1` | Wave 0 |
| STOR-01 | ParseS3URI rejects empty bucket, empty key, trailing slash, wrong scheme | unit | `go test ./internal/storage/... -run TestParseS3URI -count=1` | Wave 0 |
| STOR-02 | LocalBackend.RunInputsDir returns same path as store.RunInputsDir | unit | `go test ./internal/storage/... -run TestLocalBackend -count=1` | Wave 0 |
| STOR-02 | LocalBackend.CreateRunDirs creates directories on disk | unit | `go test ./internal/storage/... -run TestLocalBackend -count=1` | Wave 0 |
| STOR-02 | LocalBackend.WriteJSON + ReadJSON roundtrip | unit | `go test ./internal/storage/... -run TestLocalBackendJSON -count=1` | Wave 0 |
| STOR-03 | All server tests pass after Backend injection | integration | `go test ./internal/server/... -count=1` | exists (server_test.go) |
| STOR-03 | All existing store tests pass unchanged | unit | `go test ./internal/store/... -count=1` | exists (rundir_test.go) |
| STOR-03 | Full suite green | integration | `go test ./... -count=1` | 248 cases currently pass |

### Sampling Rate
- **Per task commit:** `go test ./internal/storage/... ./internal/server/... ./internal/store/... -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green (248+ cases) before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/storage/backend_test.go` — covers ParseS3URI (STOR-01)
- [ ] `internal/storage/local_test.go` — covers LocalBackend path helpers and JSON roundtrip (STOR-02)
- [ ] `internal/storage/` package itself — does not exist yet

## Sources

### Primary (HIGH confidence)
- Direct code inspection of `/home/uehara/flowy/internal/store/rundir.go` — all 5 functions: RunInputsDir, RunOutputsDir, TaskDir, CreateRunDirs, CreateTaskDirs
- Direct code inspection of `/home/uehara/flowy/internal/server/dispatch.go` — all os.WriteFile/ReadFile call sites, all store.* call sites
- Direct code inspection of `/home/uehara/flowy/internal/server/handlers.go` — store.CreateRunDirs and os.ReadFile call sites
- Direct code inspection of `/home/uehara/flowy/internal/store/store.go` — Store interface as the reference pattern for Backend interface design
- Direct code inspection of `/home/uehara/flowy/internal/server/server.go` — Server struct and New() constructor pattern
- `go test ./... -count=1` — confirmed all tests pass, 248 test cases across 19 packages

### Secondary (MEDIUM confidence)
- Go language specification on interface satisfaction — method set must match exactly; interface with embedded smaller interfaces is valid but not used in this project
- Existing Store/SQLite pattern in this codebase — confirms constructor injection is the established convention

### Tertiary (LOW confidence)
None required — all findings backed by primary sources.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies; entire phase uses stdlib
- Architecture: HIGH — all call sites catalogued from direct code inspection; pattern mirrors existing Store interface exactly
- Pitfalls: HIGH — all pitfalls derived from direct observation of the codebase (circular import topology, test constructor signatures, daemon scope boundary)

**Research date:** 2026-03-12
**Valid until:** 2026-04-11 (stable codebase; no external dependency changes)
