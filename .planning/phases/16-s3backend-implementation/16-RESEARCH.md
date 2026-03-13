# Phase 16: S3Backend Implementation - Research

**Researched:** 2026-03-13
**Domain:** AWS SDK v2 for Go, S3/MinIO object storage, multipart upload, mocking patterns
**Confidence:** HIGH

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| S3-01 | S3Backend implementation supports file upload to S3 via AWS SDK v2 PutObject | `s3.Client.PutObject()` with `io.Reader` body; `transfermanager.Client.UploadObject()` for large files |
| S3-02 | S3Backend implementation supports file download from S3 via AWS SDK v2 GetObject | `s3.Client.GetObject()` returns `io.ReadCloser` body; must close after reading |
| S3-03 | S3Backend supports multipart upload for files exceeding a configurable size threshold | `transfermanager.Client.UploadObject()` handles multipart automatically above `MultipartUploadThreshold`; use `cfg.Storage.MultipartThreshold` (100 MiB default, already in config) |
| S3-04 | S3Backend works with MinIO via UsePathStyle and custom endpoint configuration | `s3.NewFromConfig(cfg, func(o *s3.Options) { o.BaseEndpoint = aws.String(endpoint); o.UsePathStyle = true })` |
| S3-05 | Scatter task outputs use shard-indexed S3 keys to prevent key collision | S3Backend.TaskDir returns `prefix/runs/runID/tasks/taskName_shardIndex` — mirrors existing `store.TaskDir` layout |
</phase_requirements>

## Summary

Phase 16 implements `S3Backend` in `internal/storage/s3.go`, satisfying the `Backend` interface. It uses AWS SDK v2 (`github.com/aws/aws-sdk-go-v2/service/s3`) for `PutObject`/`GetObject` and `github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager` for multipart-aware uploads. The `NewBackend()` factory in `factory.go` is updated to construct a real `S3Backend` instead of the Phase 15 placeholder.

The Backend interface has no `context.Context` parameter (project decision). S3Backend adapts this by using `context.Background()` internally for all SDK calls — correct because uploads are synchronous and short-lived within each Backend method call.

The scatter key collision requirement (S3-05) is automatically satisfied by mirroring the existing `store.TaskDir` path layout as an S3 key: `{prefix}/runs/{runID}/tasks/{taskName}_{shardIndex}`. The `_{shardIndex}` suffix already exists in `store.TaskDir`; S3Backend's `TaskDir()` helper produces the equivalent key string.

The `feature/s3/manager` package was deprecated March 2026 and officially superseded by `feature/s3/transfermanager` (v0.1.9, released January 30, 2026). Use `transfermanager` for new code. The pre-v1 version number is expected — AWS released it as the production replacement.

**Primary recommendation:** Add `github.com/aws/aws-sdk-go-v2/service/s3` and `github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager` to go.mod, then implement `S3Backend` as a struct wrapping both `*s3.Client` and `*transfermanager.Client`. Use an interface-based mock for unit tests.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/aws/aws-sdk-go-v2/service/s3` | v1.97.0 (Mar 12, 2026) | S3 GetObject; CreateMultipartUpload primitives | Official AWS SDK v2; aws-sdk-go v1 end-of-support July 2025 |
| `github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager` | v0.1.9 (Mar 12, 2026) | Multipart-aware upload above threshold | Official replacement for deprecated `feature/s3/manager` |
| `github.com/aws/aws-sdk-go-v2/config` | v1.x | Load AWS config (credentials, region) | Standard SDK v2 config loading |
| `github.com/aws/aws-sdk-go-v2/credentials` | v1.x | `StaticCredentialsProvider` for MinIO | Required when not using IAM |
| `github.com/aws/aws-sdk-go-v2/aws` | v1.x | `aws.String()` pointer helper | Used pervasively in SDK v2 API |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `bytes` (stdlib) | go1.25 | `bytes.NewReader(data)` for PutObject body | Wrapping `[]byte` as `io.Reader` for upload |
| `io` (stdlib) | go1.25 | `io.ReadAll()` for GetObject body | Consuming streaming response |
| `fmt` (stdlib) | go1.25 | Error wrapping | All S3Backend error returns |
| `strings` (stdlib) | go1.25 | Key prefix construction | Path helper methods in S3Backend |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `feature/s3/transfermanager` | `feature/s3/manager` | `manager` is deprecated March 2026; `transfermanager` is official replacement |
| `transfermanager` for multipart | Manual CreateMultipartUpload/UploadPart/Complete | Hand-rolling multipart is ~100 lines of error-prone retry logic |
| `transfermanager` for download | `s3.GetObject` directly | `GetObject` is fine for sequential streaming download; `transfermanager.GetObject` adds concurrent range-get (unnecessary for Phase 16) |

**Installation:**
```bash
go get github.com/aws/aws-sdk-go-v2/service/s3
go get github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager
go get github.com/aws/aws-sdk-go-v2/config
go get github.com/aws/aws-sdk-go-v2/credentials
go get github.com/aws/aws-sdk-go-v2/aws
```

## Architecture Patterns

### Recommended Project Structure
```
internal/storage/
├── backend.go       # Backend interface (unchanged)
├── backend_test.go  # ParseS3URI tests (unchanged)
├── factory.go       # Update NewBackend() to return real S3Backend
├── local.go         # LocalBackend (unchanged)
├── local_test.go    # LocalBackend tests (unchanged)
├── s3.go            # New: S3Backend implementation
└── s3_test.go       # New: unit tests with mocked S3 client
```

### Pattern 1: S3Backend Struct

**What:** A struct wrapping `*s3.Client` for downloads and `*transfermanager.Client` for uploads with threshold dispatch.
**When to use:** Single construction point; both clients share the same `*s3.Client` underneath.

```go
// Source: official AWS SDK v2 docs pattern
// internal/storage/s3.go

package storage

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "strings"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
    "github.com/bioflowy/flowy/internal/config"
)

// S3Backend implements Backend using Amazon S3 (or MinIO) object storage.
// All keys are scoped under cfg.Storage.Prefix + a path that mirrors the
// local filesystem layout used by LocalBackend.
type S3Backend struct {
    bucket    string
    prefix    string // optional, may be empty
    threshold int64  // multipart threshold in bytes
    s3c       S3APIClient
    tm        *transfermanager.Client
}

// S3APIClient is the subset of *s3.Client methods that S3Backend needs.
// Defined as an interface so unit tests can inject a mock.
type S3APIClient interface {
    PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
    GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}
```

### Pattern 2: S3Backend Construction (MinIO config)

**What:** `NewS3Backend` accepts a `*config.Config` and wires up credentials, endpoint, and path-style.
**When to use:** Called from `factory.go` `NewBackend()` switch case "s3".

```go
// Source: docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-endpoints.html
// internal/storage/s3.go

func NewS3Backend(cfg *config.Config) (*S3Backend, error) {
    sc := cfg.Storage

    loadOpts := []func(*awsconfig.LoadOptions) error{
        awsconfig.WithRegion(sc.Region),
    }
    // Static credentials for MinIO / non-IAM deployments.
    // Production AWS deployments may omit this and rely on IAM roles.
    // (Region == "" is fine for MinIO; SDK falls back to "us-east-1")

    awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), loadOpts...)
    if err != nil {
        return nil, fmt.Errorf("storage: load AWS config: %w", err)
    }

    s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
        if sc.Endpoint != "" {
            o.BaseEndpoint = aws.String(sc.Endpoint)
            o.UsePathStyle = true // required for MinIO path-style addressing
        }
    })

    tm := transfermanager.New(s3Client, func(o *transfermanager.Options) {
        o.MultipartUploadThreshold = sc.MultipartThreshold
    })

    return &S3Backend{
        bucket:    sc.Bucket,
        prefix:    sc.Prefix,
        threshold: sc.MultipartThreshold,
        s3c:       s3Client,
        tm:        tm,
    }, nil
}
```

Note: `awsconfig` refers to `github.com/aws/aws-sdk-go-v2/config` imported with alias `awsconfig` to avoid collision with the project's own `config` package.

### Pattern 3: S3 Key Layout (scatter collision prevention)

**What:** S3Backend key helpers mirror the `store.TaskDir` path layout.
**When to use:** All Backend path helper methods in S3Backend.

The local layout is: `{baseDir}/runs/{runID}/tasks/{taskName}_{shardIndex}`

The S3 key layout is: `{prefix}/runs/{runID}/tasks/{taskName}_{shardIndex}`

```go
// internal/storage/s3.go

// keyJoin joins key segments, prepending the optional prefix.
// Uses "/" as separator regardless of OS (S3 keys are not filesystem paths).
func (b *S3Backend) keyJoin(segments ...string) string {
    if b.prefix != "" {
        segments = append([]string{b.prefix}, segments...)
    }
    // Filter empty segments to avoid double slashes.
    var parts []string
    for _, s := range segments {
        if s != "" {
            parts = append(parts, s)
        }
    }
    return strings.Join(parts, "/")
}

func (b *S3Backend) RunInputsDir(baseDir, runID string) string {
    return b.keyJoin("runs", runID, "inputs")
}

func (b *S3Backend) RunOutputsDir(baseDir, runID string) string {
    return b.keyJoin("runs", runID, "outputs")
}

// TaskDir returns the S3 key prefix for a task's working directory.
// Format: [prefix/]runs/{runID}/tasks/{taskName}_{shardIndex}
// The _{shardIndex} suffix prevents key collision when scatter produces
// multiple shards of the same task name writing identically named output files.
func (b *S3Backend) TaskDir(baseDir, runID, taskName string, shardIndex int) string {
    return b.keyJoin("runs", runID, "tasks",
        fmt.Sprintf("%s_%d", taskName, shardIndex))
}
```

Note: `baseDir` parameter is ignored by S3Backend — the bucket was provided at construction time and the prefix serves the same scoping purpose.

### Pattern 4: UploadFile with threshold dispatch

**What:** For small files use `s3.PutObject`; for large files use `transfermanager.UploadObject`.
**When to use:** `S3Backend.UploadFile()` implementation.

```go
// Source: AWS SDK v2 docs + transfermanager docs
// internal/storage/s3.go

func (b *S3Backend) UploadFile(path string, data []byte) error {
    if int64(len(data)) < b.threshold {
        // Small file: single PutObject call.
        _, err := b.s3c.PutObject(context.Background(), &s3.PutObjectInput{
            Bucket: aws.String(b.bucket),
            Key:    aws.String(path),
            Body:   bytes.NewReader(data),
        })
        if err != nil {
            return fmt.Errorf("storage: S3 PutObject %q: %w", path, err)
        }
        return nil
    }
    // Large file: transfermanager handles multipart automatically.
    _, err := b.tm.UploadObject(context.Background(), &transfermanager.UploadObjectInput{
        Bucket: aws.String(b.bucket),
        Key:    aws.String(path),
        Body:   bytes.NewReader(data),
    })
    if err != nil {
        return fmt.Errorf("storage: S3 multipart upload %q: %w", path, err)
    }
    return nil
}
```

### Pattern 5: DownloadFile (streaming GetObject)

**What:** Use `s3.GetObject` and read the streaming body.
**When to use:** `S3Backend.DownloadFile()` implementation.

```go
// Source: AWS SDK v2 docs — GetObject streaming pattern
// internal/storage/s3.go

func (b *S3Backend) DownloadFile(path string) ([]byte, error) {
    out, err := b.s3c.GetObject(context.Background(), &s3.GetObjectInput{
        Bucket: aws.String(b.bucket),
        Key:    aws.String(path),
    })
    if err != nil {
        return nil, fmt.Errorf("storage: S3 GetObject %q: %w", path, err)
    }
    defer out.Body.Close()

    data, err := io.ReadAll(out.Body)
    if err != nil {
        return nil, fmt.Errorf("storage: S3 read body %q: %w", path, err)
    }
    return data, nil
}
```

### Pattern 6: Interface-based mock for unit tests

**What:** Define `S3APIClient` interface in s3.go; create `mockS3Client` in s3_test.go.
**When to use:** All unit tests — no real S3/MinIO needed for CI.

```go
// Source: docs.aws.amazon.com/sdk-for-go/v2/developer-guide/unit-testing.html
// internal/storage/s3_test.go

type mockS3Client struct {
    putObjectFn func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
    getObjectFn func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

func (m *mockS3Client) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
    return m.putObjectFn(ctx, params, optFns...)
}

func (m *mockS3Client) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
    return m.getObjectFn(ctx, params, optFns...)
}
```

Note on mocking `transfermanager.Client`: `transfermanager.Client` is a concrete struct, not an interface. The threshold dispatch in `UploadFile` means: when `len(data) < threshold`, the mock `S3APIClient.PutObject` is called (easily testable). For the multipart path, set `threshold = 0` in the test backend so all uploads go through `transfermanager`. However, `transfermanager.New()` requires a real `S3APIClient`, so the mock can satisfy its `S3APIClient` interface. Verify the `transfermanager.S3APIClient` interface matches what `transfermanager.New()` accepts.

Alternative: expose a constructor that accepts pre-built `s3c` and `tm` so tests can inject mocks for both paths independently.

### Pattern 7: factory.go update

**What:** Replace Phase 15 placeholder in `NewBackend()` with real `NewS3Backend()`.

```go
// internal/storage/factory.go

func NewBackend(cfg *config.Config) Backend {
    switch cfg.Storage.Type {
    case "s3":
        b, err := NewS3Backend(cfg)
        if err != nil {
            // config.Validate() already checked bucket is set;
            // this error means AWS config loading failed (bad credentials, etc.)
            panic(fmt.Sprintf("storage: failed to create S3Backend: %v", err))
        }
        return b
    default:
        return &LocalBackend{}
    }
}
```

The panic is acceptable because `Validate()` already ran; this path should only fail for environment misconfiguration (missing AWS_ACCESS_KEY_ID etc.), which is a startup error.

### Anti-Patterns to Avoid

- **Using `feature/s3/manager`:** Deprecated March 2026. Use `transfermanager` instead.
- **Using `filepath.Join` for S3 keys:** On Linux `filepath.Join` uses `/` and works, but the explicit intent must be `strings.Join(..., "/")` — S3 keys are not OS filesystem paths.
- **Not closing GetObject body:** `out.Body` is a streaming HTTP response. Not closing it leaks connections. Always `defer out.Body.Close()`.
- **Passing context from Backend interface:** The Backend interface has no context parameter (project decision). Use `context.Background()` inside S3Backend methods.
- **Using `path` (not `path/filepath`) for S3 keys:** `path.Join` is correct for URL-like paths on any OS, but `strings.Join(..., "/")` is more explicit and avoids edge cases with empty segments.
- **Ignoring the `baseDir` parameter in S3Backend:** S3Backend must accept it (it's in the Backend interface) but must not use it for key construction — the bucket was set at construction time.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Multipart upload | Custom CreateMultipartUpload + UploadPart + Complete loop | `transfermanager.UploadObject()` | Handles retry on part failure, concurrent parts, abort-on-error, ETag tracking — ~300 lines of correct error handling |
| Credential loading | Custom `os.Getenv("AWS_ACCESS_KEY_ID")` parsing | `awsconfig.LoadDefaultConfig()` | Handles env vars, config files, IAM roles, instance metadata, Web Identity tokens |
| S3 URI parsing | Custom string splitting | `storage.ParseS3URI()` (already exists in Phase 14) | Phase 14 implemented this correctly with string ops (not net/url) |
| Concurrent multipart download | Custom range-get + goroutine pool | `transfermanager.GetObject()` | Not needed for Phase 16 (sequential download is fine); use if performance becomes a concern |

**Key insight:** The AWS SDK v2 handles all retry logic, credential rotation, connection pooling, and S3 protocol quirks. S3Backend should be a thin adapter between the Backend interface and the SDK.

## Common Pitfalls

### Pitfall 1: filepath.Clean corrupts S3 keys
**What goes wrong:** If `filepath.Join` or `filepath.Clean` is applied to the S3 key returned by `TaskDir()`, the key loses its `s3://` prefix or gets double-slashes cleaned incorrectly.
**Why it happens:** S3Backend.TaskDir returns a plain key string (no `s3://` prefix), not a URI. But calling code in the server uses `backend.TaskDir()` result as an opaque token and may pass it back to `UploadFile`/`DownloadFile`. No filepath operations should be applied to S3 keys.
**How to avoid:** S3Backend key helpers return plain S3 key strings. Never pass them through `filepath.Join`. The existing `wdl/pathutil.CleanPath()` guard (Phase 15) protects `s3://` URIs; but keys returned by `TaskDir()` are key prefixes, not `s3://` URIs.
**Warning signs:** Missing objects in S3 because key has wrong path separators.

### Pitfall 2: `io.ReadAll` buffers entire file in memory
**What goes wrong:** For very large files, `DownloadFile()` using `io.ReadAll(out.Body)` loads the full content into a `[]byte`. This is a fundamental constraint of the Backend interface (`DownloadFile` returns `[]byte`).
**Why it happens:** The Backend interface uses whole-file semantics (by design from Phase 14 decision — "value-in/value-out").
**How to avoid:** This is intentional for Phase 16. The PERF-01 requirement (streaming I/O without buffering) is a Future Requirement, not in scope for v1.2. Accept the whole-file approach for now.
**Warning signs:** OOM errors on very large files — acceptable limitation until PERF-01 is addressed.

### Pitfall 3: transfermanager `S3APIClient` interface mismatch
**What goes wrong:** `transfermanager.New()` accepts an `S3APIClient` interface. If the project's `S3APIClient` interface doesn't exactly match what `transfermanager` requires, compilation fails.
**Why it happens:** `transfermanager` defines its own internal interface for the S3 client it requires. The methods it needs include more than just PutObject/GetObject.
**How to avoid:** For the `S3APIClient` interface used in `s3.go`, only declare the methods that `S3Backend` calls directly (PutObject, GetObject). Pass `*s3.Client` (concrete type) to `transfermanager.New()` — do not try to pass the mock through `transfermanager`.
**Warning signs:** `cannot use mockS3Client as type transfermanager.S3APIClient` compile error.

### Pitfall 4: Missing `UsePathStyle = true` for MinIO
**What goes wrong:** Without `UsePathStyle = true`, the SDK constructs virtual-hosted-style URLs (`bucket.host:9000/key`). MinIO requires path-style (`host:9000/bucket/key`).
**Why it happens:** Default S3 behavior uses virtual-hosted-style which works for AWS but not MinIO.
**How to avoid:** Always set `o.UsePathStyle = true` when `cfg.Storage.Endpoint != ""` (i.e., non-AWS endpoint).
**Warning signs:** "dial tcp: no such host bucket.localhost" or 404/403 errors on MinIO.

### Pitfall 5: context.Context required by SDK but absent from Backend interface
**What goes wrong:** Trying to thread context through the Backend interface breaks the project decision. Alternatively, forgetting to pass any context causes SDK panic.
**Why it happens:** AWS SDK v2 requires context for all calls. Backend interface forbids context parameters (project decision from Phase 14).
**How to avoid:** Use `context.Background()` inside S3Backend method bodies. This is correct — S3Backend methods are synchronous from the caller's perspective and don't need cancellation.
**Warning signs:** `cannot use nil as context.Context` compile error.

### Pitfall 6: Package import collision between project `config` and `awsconfig`
**What goes wrong:** Both `github.com/bioflowy/flowy/internal/config` and `github.com/aws/aws-sdk-go-v2/config` need to be imported in `s3.go`.
**Why it happens:** Both packages are named `config`.
**How to avoid:** Import the AWS config package with an alias: `awsconfig "github.com/aws/aws-sdk-go-v2/config"`. The project config remains `"github.com/bioflowy/flowy/internal/config"` unaliased.
**Warning signs:** `imported and not used: "github.com/aws/aws-sdk-go-v2/config"` or ambiguous `config.` usage.

## Code Examples

Verified patterns from official sources:

### MinIO-compatible S3 client construction
```go
// Source: docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-endpoints.html
awsconfig "github.com/aws/aws-sdk-go-v2/config"
"github.com/aws/aws-sdk-go-v2/service/s3"
"github.com/aws/aws-sdk-go-v2/aws"

awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
    awsconfig.WithRegion(region), // "us-east-1" if empty for MinIO
)

s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
    if endpoint != "" {
        o.BaseEndpoint = aws.String(endpoint)
        o.UsePathStyle = true
    }
})
```

### transfermanager upload with threshold
```go
// Source: pkg.go.dev/github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager
tm := transfermanager.New(s3Client, func(o *transfermanager.Options) {
    o.MultipartUploadThreshold = thresholdBytes // e.g. 100 * 1024 * 1024
})

_, err := tm.UploadObject(context.Background(), &transfermanager.UploadObjectInput{
    Bucket: aws.String(bucket),
    Key:    aws.String(key),
    Body:   bytes.NewReader(data),
})
```

### GetObject with streaming body
```go
// Source: pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3
out, err := s3Client.GetObject(context.Background(), &s3.GetObjectInput{
    Bucket: aws.String(bucket),
    Key:    aws.String(key),
})
if err != nil {
    return nil, fmt.Errorf("storage: GetObject %q: %w", key, err)
}
defer out.Body.Close()
data, err := io.ReadAll(out.Body)
```

### Mock S3 client for unit tests
```go
// Source: docs.aws.amazon.com/sdk-for-go/v2/developer-guide/unit-testing.html

type mockS3Client struct {
    putObjectFn func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
    getObjectFn func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

func (m *mockS3Client) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
    return m.putObjectFn(ctx, params, optFns...)
}
```

### S3 key layout (scatter prevention)
```go
// S3 key for taskName="scatter_task", shardIndex=3:
// → "flowy-runs/runs/run-abc/tasks/scatter_task_3"
// Two shards with same taskName produce DIFFERENT keys:
// shard 0: "flowy-runs/runs/run-abc/tasks/scatter_task_0"
// shard 3: "flowy-runs/runs/run-abc/tasks/scatter_task_3"
func (b *S3Backend) TaskDir(baseDir, runID, taskName string, shardIndex int) string {
    return b.keyJoin("runs", runID, "tasks",
        fmt.Sprintf("%s_%d", taskName, shardIndex))
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `github.com/aws/aws-sdk-go` (v1) | `github.com/aws/aws-sdk-go-v2` | End-of-support July 2025 | Must use v2 exclusively |
| `feature/s3/manager` (Uploader/Downloader) | `feature/s3/transfermanager` (unified Client) | Deprecated March 2026, replacement released Jan 30, 2026 | Use transfermanager for new code |
| `s3iface` package for mocking | Custom interface + mock struct | Removed from SDK v2 | Define own `S3APIClient` interface with only needed methods |
| `EndpointResolver` (deprecated) | `BaseEndpoint` option on client | SDK v2 ~2023 | Use `o.BaseEndpoint = aws.String(url)` |
| `NewBackend()` returns LocalBackend for "s3" | `NewBackend()` returns `NewS3Backend()` | Phase 16 | Enables real S3 I/O |

**Deprecated/outdated:**
- `feature/s3/manager`: deprecated March 2026, replaced by `feature/s3/transfermanager`
- `s3iface` package: not in SDK v2; use interface definition in your own package
- `EndpointResolver` in SDK v2: replaced by `BaseEndpoint` option

## Open Questions

1. **transfermanager mock for multipart path**
   - What we know: `transfermanager.New()` accepts a `S3APIClient` defined within the `transfermanager` package, not an interface we define. The mock we define for `S3APIClient` (PutObject/GetObject) won't satisfy `transfermanager.S3APIClient` which needs more methods.
   - What's unclear: Exact method set of `transfermanager.S3APIClient` — needs verification at implementation time via `go doc`.
   - Recommendation: During implementation, run `go doc github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager S3APIClient` to check. If it requires many methods, use threshold dispatch test strategy: set `threshold=0` in unit tests to force all uploads through PutObject (skipping transfermanager), and trust that transfermanager's own tests cover the multipart path.

2. **Region for MinIO**
   - What we know: MinIO doesn't require a real AWS region but the SDK v2 will use `us-east-1` as default if region is empty.
   - What's unclear: Whether empty-string region in `StorageConfig.Region` causes SDK loading errors.
   - Recommendation: In `NewS3Backend`, if `sc.Region == ""` use `"us-east-1"` as default for MinIO compatibility. Add `applyDefaults()` change if needed.

3. **CreateRunDirs / CreateTaskDirs for S3Backend**
   - What we know: S3 has no concept of directories — objects exist or don't; there's no "create directory" operation.
   - What's unclear: Whether callers depend on `CreateRunDirs`/`CreateTaskDirs` succeeding for any initialization logic.
   - Recommendation: Implement both as no-ops for S3Backend (identical rationale to `UploadDirectory`/`DownloadDirectory` being no-ops for LocalBackend). S3 objects are created by `PutObject`; no pre-creation needed.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib testing (go test) |
| Config file | none (go test ./... discovers tests by convention) |
| Quick run command | `go test ./internal/storage/... -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| S3-01 | UploadFile calls PutObject with correct bucket, key, body | unit | `go test ./internal/storage/... -run TestS3Backend_UploadFile -count=1` | ❌ Wave 0 |
| S3-01 | UploadFile returns error when PutObject fails | unit | `go test ./internal/storage/... -run TestS3Backend_UploadFile_Error -count=1` | ❌ Wave 0 |
| S3-02 | DownloadFile calls GetObject with correct bucket/key and returns body bytes | unit | `go test ./internal/storage/... -run TestS3Backend_DownloadFile -count=1` | ❌ Wave 0 |
| S3-02 | DownloadFile returns error when GetObject fails | unit | `go test ./internal/storage/... -run TestS3Backend_DownloadFile_Error -count=1` | ❌ Wave 0 |
| S3-03 | UploadFile with len(data) >= threshold uses transfermanager (or PutObject with threshold=0 test) | unit | `go test ./internal/storage/... -run TestS3Backend_UploadFile_Multipart -count=1` | ❌ Wave 0 |
| S3-04 | NewS3Backend sets UsePathStyle=true and BaseEndpoint when endpoint is configured | unit | `go test ./internal/storage/... -run TestNewS3Backend_MinIOConfig -count=1` | ❌ Wave 0 |
| S3-05 | TaskDir returns key with _{shardIndex} suffix; two shards produce distinct keys | unit | `go test ./internal/storage/... -run TestS3Backend_TaskDir -count=1` | ❌ Wave 0 |
| S3-05 | UploadFile with scatter key (taskName_3/file.txt) produces correct S3 key | unit | `go test ./internal/storage/... -run TestS3Backend_UploadFile_ScatterKey -count=1` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/storage/... -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green (`go test ./... -count=1` all 19+ packages pass) before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/storage/s3.go` — S3Backend implementation (the package itself)
- [ ] `internal/storage/s3_test.go` — mock client + unit tests covering S3-01 through S3-05
- [ ] `go.mod` / `go.sum` additions: `github.com/aws/aws-sdk-go-v2/service/s3`, `github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager`, `github.com/aws/aws-sdk-go-v2/config`, `github.com/aws/aws-sdk-go-v2/credentials`, `github.com/aws/aws-sdk-go-v2/aws` — run `go get` for each

## Sources

### Primary (HIGH confidence)
- `pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3` v1.97.0 — PutObject/GetObject signatures confirmed
- `pkg.go.dev/github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager` v0.1.9 — UploadObjectInput.Body type (io.Reader), MultipartUploadThreshold option confirmed
- `docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-endpoints.html` — BaseEndpoint + UsePathStyle for MinIO pattern confirmed
- `docs.aws.amazon.com/sdk-for-go/v2/developer-guide/unit-testing.html` — interface-based mock pattern confirmed
- Direct code inspection: `internal/storage/backend.go` — Backend interface, no context.Context confirmed
- Direct code inspection: `internal/store/rundir.go` — TaskDir key layout `{taskName}_{shardIndex}` confirmed
- Direct code inspection: `internal/config/config.go` — StorageConfig fields (Endpoint, Region, Bucket, Prefix, MultipartThreshold) confirmed
- Direct code inspection: `internal/storage/factory.go` — Phase 15 placeholder confirmed, ready for Phase 16 replacement
- `go test ./... -count=1` — all 19 packages pass; baseline confirmed

### Secondary (MEDIUM confidence)
- `github.com/aws/aws-sdk-go-v2/discussions/3306` — deprecation of `feature/s3/manager` announced; `transfermanager` released January 30, 2026 as production replacement
- `pkg.go.dev/github.com/aws/aws-sdk-go-v2/feature/s3/manager` — deprecation notice text confirmed: "Deprecated: superseded by feature/s3/transfermanager"

### Tertiary (LOW confidence)
- WebSearch finding: `transfermanager v0.1.9` is pre-v1 but AWS explicitly released it as production replacement — LOW confidence only because version < 1.0 is unusual signal despite official announcement

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — SDK v2 confirmed via pkg.go.dev; deprecation confirmed via official package page and GitHub discussion
- Architecture: HIGH — patterns derived from official AWS docs and existing project code
- Pitfalls: HIGH — derived from Go filepath behavior (verified), SDK v2 interface removal (verified), and existing project constraints (no context in Backend interface)

**Research date:** 2026-03-13
**Valid until:** 2026-06-13 (AWS SDK v2 is stable; transfermanager at v0.1.x may change API before v1.0 — re-check if upgrading)
