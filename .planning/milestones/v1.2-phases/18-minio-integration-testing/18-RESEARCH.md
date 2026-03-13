# Phase 18: MinIO Integration Testing - Research

**Researched:** 2026-03-13
**Domain:** testcontainers-go, Go integration testing, MinIO/S3, Makefile orchestration
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- Use testcontainers-go for programmatic MinIO lifecycle in Go integration tests
- Pin MinIO image to `minio/minio:RELEASE.2024-11-07T00-52-28Z`
- Hardcoded test credentials: `minioadmin` / `minioadmin` (MinIO defaults)
- One MinIO container per test package via TestMain — shared across all test functions, torn down after all tests complete
- Tests live in `internal/integration/` package, separate from unit tests
- Cover single-task S3 round-trip: upload input to MinIO → submit task → daemon localizes → mock executor runs → daemon delocalizes → verify output in MinIO
- Use Go function calls (in-process Server/Daemon wiring), not binary subprocesses
- Mock executor replaces RunTask — reads inputs from stageDir, writes output files to workDir. No Docker dependency in integration tests
- New Makefile target: `make spectool-s3`
- MinIO managed via `docker run` in Makefile with pinned image tag — same PID-file pattern as existing server/daemon lifecycle
- Auto-create test bucket via `mc mb` or `aws s3 mb` CLI after MinIO readiness check
- Generate temporary `config.yaml` with S3 settings (endpoint, bucket, credentials, path-style) — server and daemon read via `-c` flag
- Integration tests gated with `//go:build integration` build tag — `go test ./...` skips them
- Timeout: 2 minutes (`-timeout 2m`)
- New Makefile target: `make integration` runs `go test -tags integration -timeout 2m ./internal/integration/...`

### Claude's Discretion
- testcontainers-go container request configuration details
- TestMain setup/teardown helper structure
- Mock executor implementation (how to inject into daemon)
- Config.yaml template for spectool-s3 (exact YAML structure)
- MinIO readiness check method in Makefile (curl vs mc vs aws cli)
- Bucket naming convention for tests

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| TEST-01 | MinIO integration tests using testcontainers-go verify end-to-end S3 workflow execution | testcontainers-go v0.41.0 minio module API documented; TestMain shared-container pattern confirmed; mock executor injection via ContainerRunner interface |
| TEST-02 | Integration tests cover file upload, download, and intermediate file passing via S3 | S3Backend UploadFile/DownloadFile/UploadDirectory/DownloadDirectory all tested against real MinIO; daemon's localizeInputs/delocalizeOutputs exercised end-to-end |
</phase_requirements>

## Summary

Phase 18 adds no production code. It wires together the real S3Backend, a real MinIO container, and the daemon's LED (Localize-Execute-Delocalize) code path using in-process Go function calls and a mock executor. The integration test package lives at `internal/integration/` behind the `//go:build integration` tag, keeping `go test ./...` fast. A parallel `make spectool-s3` target validates that all 156 spectool conformance tests still pass when S3 is the default backend.

The key technical challenge is injecting a mock executor into `executeTaskS3` without modifying the production daemon struct. The `ContainerRunner` interface (`task.ContainerRunner`) already exists and is injectable via `task.RunTask`'s runner parameter. However, `executeTaskS3` currently calls `task.RunTask` directly with `task.NewDockerRunner()`. Integration tests must either: (a) wire the daemon differently by exporting a constructor that accepts a runner, or (b) call the localize/execute/delocalize sub-functions directly in the test, bypassing `executeTaskS3`. Option (b) is preferred since it keeps the daemon struct unchanged.

The `make spectool-s3` target follows the established Makefile PID-file pattern exactly, adding MinIO as a third managed process alongside server and daemon. MinIO's readiness can be checked with `curl -s http://localhost:9000/minio/health/live` (MinIO built-in health endpoint) which avoids requiring mc or aws CLI.

**Primary recommendation:** Write integration tests that call the daemon's exported helper functions directly (localizeInputs, delocalizeOutputs, walkAndLocalize) plus a real S3Backend against a testcontainers MinIO, rather than trying to run a full Daemon.Run loop.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| testcontainers-go | v0.41.0 | MinIO container lifecycle in Go tests | Official library, active as of March 2026, minio module included |
| testcontainers-go/modules/minio | v0.41.0 | Pre-configured MinIO container with typed API | Avoids hand-rolling GenericContainer config for MinIO |
| aws-sdk-go-v2 | already in go.mod | Create bucket + verify S3 objects in test assertions | Already used by S3Backend; reuse same client config |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| Standard library `net/http/httptest` | stdlib | In-process HTTP server for server | Already used in server_test.go |
| `internal/store/sqlite` | in-repo | In-memory SQLite for server in integration tests | Already used in server_test.go newTestServer pattern |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| testcontainers-go minio module | GenericContainer with minio image | Module provides typed API and pre-configured wait strategy; GenericContainer requires manual port/health setup |
| testcontainers-go | `docker run` in Go test setup | Makefile uses docker run for spectool-s3; Go tests use testcontainers for programmatic lifecycle management |
| aws-sdk-go-v2 for bucket create | minio-go SDK | aws-sdk-go-v2 is already in go.mod; no additional dependency needed |

**Installation:**
```bash
go get github.com/testcontainers/testcontainers-go@v0.41.0
go get github.com/testcontainers/testcontainers-go/modules/minio@v0.41.0
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── integration/               # New package (this phase)
│   ├── main_test.go           # TestMain: starts MinIO container, creates bucket, tears down
│   ├── s3_roundtrip_test.go   # TEST-01: single-task S3 round-trip end-to-end
│   └── intermediate_test.go   # TEST-02: upload input → localize → execute → delocalize → verify
```

### Pattern 1: TestMain Shared Container
**What:** One MinIO container started in TestMain, shared across all test functions in the package.
**When to use:** When container startup cost (3-5 seconds) is high relative to individual test duration.

```go
//go:build integration

package integration

import (
    "context"
    "log"
    "os"
    "testing"

    "github.com/aws/aws-sdk-go-v2/aws"
    awsconfig "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/credentials"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/testcontainers/testcontainers-go"
    minioctx "github.com/testcontainers/testcontainers-go/modules/minio"
)

const (
    testBucket   = "flowy-integration-test"
    testUser     = "minioadmin"
    testPassword = "minioadmin"
    minioImage   = "minio/minio:RELEASE.2024-11-07T00-52-28Z"
)

var (
    minioEndpoint string // set by TestMain, e.g. "http://localhost:XXXXX"
)

func TestMain(m *testing.M) {
    ctx := context.Background()

    mc, err := minioctx.Run(ctx, minioImage,
        minioctx.WithUsername(testUser),
        minioctx.WithPassword(testPassword),
    )
    defer testcontainers.TerminateContainer(mc)
    if err != nil {
        log.Fatalf("start minio: %v", err)
    }

    connStr, err := mc.ConnectionString(ctx)
    if err != nil {
        log.Fatalf("minio connection string: %v", err)
    }
    minioEndpoint = "http://" + connStr

    // Create test bucket using aws-sdk-go-v2.
    awsCfg, _ := awsconfig.LoadDefaultConfig(ctx,
        awsconfig.WithRegion("us-east-1"),
        awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(testUser, testPassword, "")),
    )
    s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
        o.BaseEndpoint = aws.String(minioEndpoint)
        o.UsePathStyle = true
    })
    if _, err := s3Client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(testBucket)}); err != nil {
        log.Fatalf("create bucket: %v", err)
    }

    os.Exit(m.Run())
}
```
Source: [pkg.go.dev/github.com/testcontainers/testcontainers-go/modules/minio](https://pkg.go.dev/github.com/testcontainers/testcontainers-go/modules/minio)

### Pattern 2: In-Process S3 Round-Trip Test (Mock Executor)
**What:** Call daemon helper functions directly without running Daemon.Run. Inject a mock ContainerRunner.
**When to use:** When testing the LED path end-to-end without Docker/WDL overhead.

```go
//go:build integration

package integration

import (
    "context"
    "os"
    "path/filepath"
    "testing"

    "github.com/bioflowy/flowy/internal/config"
    "github.com/bioflowy/flowy/internal/storage"
    "github.com/bioflowy/flowy/wdl/task"
    "github.com/bioflowy/flowy/wdl/values"
)

// mockRunner implements task.ContainerRunner without Docker.
// It reads inputs from stageDir and writes a fixed output file.
type mockRunner struct{}

func (r *mockRunner) Run(_ context.Context, req task.RunRequest) (task.RunResult, error) {
    // Write a result file to the working directory.
    outFile := filepath.Join(req.WorkDir, "result.txt")
    _ = os.WriteFile(outFile, []byte("integration-test-output"), 0644)
    return task.RunResult{ExitCode: 0}, nil
}

func TestS3RoundTrip(t *testing.T) {
    ctx := context.Background()

    cfg := &config.Config{}
    cfg.Storage.Type = "s3"
    cfg.Storage.Bucket = testBucket
    cfg.Storage.Endpoint = minioEndpoint
    cfg.Storage.Region = "us-east-1"
    cfg.Storage.MultipartThreshold = 100 * 1024 * 1024

    // Set MinIO credentials in environment for aws-sdk-go-v2 default chain.
    t.Setenv("AWS_ACCESS_KEY_ID", testUser)
    t.Setenv("AWS_SECRET_ACCESS_KEY", testPassword)

    backend, err := storage.NewS3Backend(cfg)
    if err != nil {
        t.Fatalf("NewS3Backend: %v", err)
    }

    // Upload input file to MinIO.
    inputKey := "runs/test-run-1/inputs/input.txt"
    if err := backend.UploadFile(inputKey, []byte("hello from integration test")); err != nil {
        t.Fatalf("UploadFile: %v", err)
    }

    // Verify download.
    data, err := backend.DownloadFile(inputKey)
    if err != nil {
        t.Fatalf("DownloadFile: %v", err)
    }
    if string(data) != "hello from integration test" {
        t.Errorf("data mismatch: got %q", string(data))
    }
}
```

### Pattern 3: Build Tag Gating
**What:** `//go:build integration` at top of every file in `internal/integration/`, before package declaration.
**When to use:** Always, for the entire integration package.

```go
//go:build integration

package integration
```

Run with: `go test -tags integration -timeout 2m ./internal/integration/...`
Unit tests remain unaffected: `go test ./...` skips all integration files.

Source: [mickey.dev/posts/go-build-tags-testing](https://mickey.dev/posts/go-build-tags-testing/)

### Pattern 4: Makefile spectool-s3 Target
**What:** Extends the existing spectool pattern with a third process (MinIO) using PID-file lifecycle.
**When to use:** `make spectool-s3` for full 156/156 conformance validation against S3 backend.

```makefile
MINIO_BIN    := minio/minio:RELEASE.2024-11-07T00-52-28Z
MINIO_PID    := .minio.pid
MINIO_PORT   := 9000
TEST_BUCKET  := flowy-spectool-test
S3_CONFIG    := /tmp/flowy-s3.yaml

spectool-s3: $(FLOWY_BIN) $(SERVER_BIN) $(DAEMON_BIN) $(SPECTOOL_BIN) $(WDL_SPEC_DIR)
    # Start MinIO
    docker run --rm -d \
        -e MINIO_ROOT_USER=minioadmin \
        -e MINIO_ROOT_PASSWORD=minioadmin \
        -p $(MINIO_PORT):9000 \
        $(MINIO_BIN) server /data \
        > /dev/null & echo $$! > $(MINIO_PID)
    # Wait for MinIO readiness
    @for i in 1 2 3 4 5 6 7 8 9 10; do \
        if curl -s -o /dev/null http://localhost:$(MINIO_PORT)/minio/health/live; then \
            echo "MinIO ready."; break; \
        fi; \
        if [ $$i -eq 10 ]; then echo "ERROR: MinIO failed to start"; exit 1; fi; \
        sleep 1; \
    done
    # Create bucket via aws CLI (uses env creds)
    AWS_ACCESS_KEY_ID=minioadmin AWS_SECRET_ACCESS_KEY=minioadmin \
        aws --endpoint-url http://localhost:$(MINIO_PORT) \
        s3 mb s3://$(TEST_BUCKET) --region us-east-1
    # Generate S3 config.yaml
    printf 'storage:\n  type: s3\n  bucket: $(TEST_BUCKET)\n  endpoint: http://localhost:$(MINIO_PORT)\n  region: us-east-1\n' > $(S3_CONFIG)
    # ... then start server + daemon with -c $(S3_CONFIG), run spectool, cleanup all three ...
```

Note: The `aws` CLI is more universally available than `mc` on CI runners. Check if `aws` is available in the target environment; if not, `mc` is the alternative.

### Anti-Patterns to Avoid
- **Running Daemon.Run in integration tests:** The poll loop is not testable synchronously. Call exported helper functions directly instead.
- **Using GenericContainer instead of minio module:** The minio module configures the MinIO start command (`server /data`) and wait strategy automatically.
- **Omitting `//go:build integration`:** Without the tag, the testcontainers import will break `go test ./...` when Docker is unavailable.
- **Sharing minioEndpoint via package-level var without sync:** TestMain sets it before m.Run(); no goroutine races since all tests run after TestMain completes.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| MinIO container lifecycle | Custom docker.Client wrapper | testcontainers-go minio module | Wait strategy, port mapping, image entrypoint config — already handled |
| MinIO readiness detection in Go test | time.Sleep + HTTP ping loop | minio module's built-in wait strategy | MinIO module auto-waits for API availability |
| MinIO readiness in Makefile | Custom wait script | curl loop on `/minio/health/live` | MinIO exposes this endpoint by design |
| S3 bucket creation in tests | minio-go MakeBucket | aws-sdk-go-v2 CreateBucket (already in go.mod) | Avoids adding a new dependency |
| AWS credential setup in tests | Custom credential provider | `t.Setenv("AWS_ACCESS_KEY_ID", ...)` + static provider | SDK's default credential chain picks up env vars |

**Key insight:** The `ContainerRunner` interface already exists in `wdl/task/runner.go`. The mock executor for integration tests is just a struct implementing that single-method interface — it does not require any production code changes.

## Common Pitfalls

### Pitfall 1: AWS Credential Chain Picks Up Real Credentials
**What goes wrong:** On a developer machine with `~/.aws/credentials`, `awsconfig.LoadDefaultConfig` loads real AWS credentials instead of MinIO credentials, causing SDK calls to go to AWS S3 rather than the local MinIO.
**Why it happens:** AWS SDK v2 credential chain: env vars → shared credentials file → IAM role. If env vars are not set, it falls through to `~/.aws/credentials`.
**How to avoid:** In tests, explicitly set `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` env vars using `t.Setenv` before calling `NewS3Backend`. Or construct the backend with an explicit `credentials.NewStaticCredentialsProvider`.
**Warning signs:** "Access Denied" from real AWS, or test costs showing up on AWS bill.

### Pitfall 2: testcontainers-go Requires Docker daemon
**What goes wrong:** Integration tests fail with "Cannot connect to the Docker daemon" in CI environments without Docker.
**Why it happens:** testcontainers-go makes real Docker API calls.
**How to avoid:** The `//go:build integration` tag ensures these tests only run when explicitly requested with `-tags integration`. CI should only invoke `make integration` on Docker-capable runners.
**Warning signs:** `go test ./...` failing — check that the build tag is correct on all integration files.

### Pitfall 3: MinIO Container Port Is Ephemeral
**What goes wrong:** Hardcoding `localhost:9000` in tests — testcontainers-go maps the container port to a random host port to avoid conflicts.
**Why it happens:** testcontainers-go uses ephemeral port mapping by default.
**How to avoid:** Always use `mc.ConnectionString(ctx)` to get the actual host:port. The returned string is `host:port` (no scheme); prefix with `http://` for the endpoint URL.
**Warning signs:** Connection refused on port 9000 when another MinIO is already running.

### Pitfall 4: delocalizeOutputs Walks stageDir, Not workDir
**What goes wrong:** The mock executor writes output files to `req.WorkDir` (inside stageDir), but `delocalizeOutputs` is called with `stageDir` as the root. If workDir is a subdirectory of stageDir, all files under stageDir get uploaded — including inputs.
**Why it happens:** `delocalizeOutputs` in daemon.go uses `workDir` as the walk root. Integration tests must pass the correct directory.
**How to avoid:** Understand that `executeTaskS3` passes `stageDir` to `delocalizeOutputs` as the walk root. The mock executor must write outputs to a `workDir` subdirectory. Verify what directory the real RunTask uses as workDir.

### Pitfall 5: spectool-s3 MinIO Port Conflict
**What goes wrong:** If a leftover MinIO container occupies port 9000, the Makefile `docker run -p 9000:9000` fails.
**Why it happens:** PID file cleanup didn't kill the previous container.
**How to avoid:** Add a cleanup step at the beginning of spectool-s3 to kill any existing `.minio.pid` and stop any container bound to port 9000. Use `docker rm -f` on the container name if using `--name minio-spectool`.

### Pitfall 6: `os.Exit` in TestMain Skips defer
**What goes wrong:** `defer testcontainers.TerminateContainer(mc)` does not run if `os.Exit(m.Run())` is called directly.
**Why it happens:** `os.Exit` bypasses deferred functions.
**How to avoid:** Use a wrapper function pattern:
```go
func TestMain(m *testing.M) {
    os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
    // setup...
    defer testcontainers.TerminateContainer(mc)
    return m.Run()
}
```
Source: confirmed pattern from testcontainers-go getting started guide.

## Code Examples

Verified patterns from official sources:

### MinIO Container Startup (official minio module)
```go
// Source: https://pkg.go.dev/github.com/testcontainers/testcontainers-go/modules/minio@v0.41.0
mc, err := minio.Run(ctx, "minio/minio:RELEASE.2024-11-07T00-52-28Z",
    minio.WithUsername("minioadmin"),
    minio.WithPassword("minioadmin"),
)
// ConnectionString returns "host:port" (no scheme)
connStr, err := mc.ConnectionString(ctx)
endpoint := "http://" + connStr
```

### AWS SDK v2 S3 Client for MinIO
```go
// Source: internal/storage/s3.go (existing pattern, adapted for static creds)
awsCfg, _ := awsconfig.LoadDefaultConfig(ctx,
    awsconfig.WithRegion("us-east-1"),
    awsconfig.WithCredentialsProvider(
        credentials.NewStaticCredentialsProvider("minioadmin", "minioadmin", ""),
    ),
)
s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
    o.BaseEndpoint = aws.String(endpoint)
    o.UsePathStyle = true
})
_, err = s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
    Bucket: aws.String("flowy-integration-test"),
})
```

### Constructing S3Backend for Integration Test
```go
// Source: internal/config/config.go + internal/storage/factory.go (existing code)
cfg := &config.Config{}
cfg.Storage.Type = "s3"
cfg.Storage.Bucket = testBucket
cfg.Storage.Endpoint = minioEndpoint
cfg.Storage.Region = "us-east-1"
cfg.Storage.MultipartThreshold = 100 * 1024 * 1024
// credentials via AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY env vars (set by t.Setenv)
backend, err := storage.NewS3Backend(cfg)
```

### spectool-s3 config.yaml Template
```yaml
# /tmp/flowy-s3.yaml — generated by make spectool-s3
base_dir: ./flowy-data-s3
storage:
  type: s3
  bucket: flowy-spectool-test
  endpoint: http://localhost:9000
  region: us-east-1
server:
  listen_addr: ":8125"
daemon:
  server_url: "http://localhost:8125"
  workers: 4
client:
  server_url: "http://localhost:8125"
```
Note: Use a different port (8125) from the default spectool target (8124) so both can run simultaneously if needed.

### MinIO Health Check in Makefile
```bash
# MinIO built-in liveness endpoint — no mc or aws CLI required
for i in 1 2 3 4 5 6 7 8 9 10; do
    if curl -s -o /dev/null http://localhost:9000/minio/health/live; then
        echo "MinIO ready."; break
    fi
    if [ $i -eq 10 ]; then echo "ERROR: MinIO failed"; exit 1; fi
    sleep 1
done
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `GenericContainer` | `minio.Run()` / module-specific `Run()` | ~v0.28.0 | Typed container, pre-configured entrypoint and wait strategy |
| `RunContainer` | `Run` | v0.28.0+ (RunContainer deprecated) | Use `Run` — `RunContainer` still works but is deprecated |
| `// +build integration` | `//go:build integration` | Go 1.17 | New syntax required; old syntax still accepted but linters warn |

**Deprecated/outdated:**
- `testcontainers.RunContainer`: replaced by `testcontainers.Run` — still functional but deprecated
- `// +build` syntax: replaced by `//go:build` — both accepted by compiler, but go vet prefers new syntax

## Open Questions

1. **How does delocalizeOutputs determine which directory to walk?**
   - What we know: `executeTaskS3` calls `delocalizeOutputs(outputs, stageDir, claim, backend)` where `stageDir` is the temp staging dir. The mock executor must write files to a location inside stageDir.
   - What's unclear: `task.RunTask` determines workDir internally; integration tests bypassing RunTask need to place output files explicitly in the right subdirectory.
   - Recommendation: Test calls localizeInputs() to get stageDir, then creates `stageDir/work/` manually for mock executor output, then calls delocalizeOutputs with stageDir.

2. **Does spectool-s3 need `AWS_ACCESS_KEY_ID` env vars exported in Makefile?**
   - What we know: flowy-server and flowy-daemon use `awsconfig.LoadDefaultConfig`, which reads env vars. MinIO credentials must be available.
   - What's unclear: Whether the Makefile should export `AWS_ACCESS_KEY_ID=minioadmin AWS_SECRET_ACCESS_KEY=minioadmin` before starting server/daemon, or embed them in config.yaml.
   - Recommendation: Export in Makefile for simplicity; config.yaml does not support credentials (by design — daemon uses own creds/IAM role per project decision).

3. **Bucket naming: per-test or shared?**
   - What we know: Context.md delegates this to Claude's discretion.
   - Recommendation: One shared bucket per test run (`flowy-integration-test`). Prefix S3 keys with a test-specific UUID to isolate test data within the shared bucket. This avoids bucket creation overhead per test.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing stdlib + testcontainers-go v0.41.0 |
| Config file | none (go test flags) |
| Quick run command | `go test -tags integration -timeout 2m -run TestS3RoundTrip ./internal/integration/...` |
| Full suite command | `go test -tags integration -timeout 2m ./internal/integration/...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TEST-01 | MinIO container starts, S3Backend works end-to-end | integration | `go test -tags integration -run TestS3RoundTrip ./internal/integration/...` | ❌ Wave 0 |
| TEST-02 | Upload input → localize → mock execute → delocalize → verify S3 output | integration | `go test -tags integration -run TestIntermediateFile ./internal/integration/...` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test -tags integration -timeout 2m -run TestS3RoundTrip ./internal/integration/...`
- **Per wave merge:** `go test -tags integration -timeout 2m ./internal/integration/...`
- **Phase gate:** Full integration suite green + `go test ./...` (unit tests unchanged)

### Wave 0 Gaps
- [ ] `internal/integration/main_test.go` — TestMain, MinIO container startup, bucket creation
- [ ] `internal/integration/s3_roundtrip_test.go` — TEST-01 single-task S3 round-trip
- [ ] `internal/integration/intermediate_test.go` — TEST-02 intermediate file via S3
- [ ] `go get github.com/testcontainers/testcontainers-go/modules/minio@v0.41.0` — add to go.mod
- [ ] `go get github.com/testcontainers/testcontainers-go@v0.41.0` — add to go.mod
- [ ] Makefile `spectool-s3` target — MinIO + server + daemon + spectool with S3 config
- [ ] Makefile `integration` target — `go test -tags integration ...`

## Sources

### Primary (HIGH confidence)
- [pkg.go.dev/github.com/testcontainers/testcontainers-go/modules/minio@v0.41.0](https://pkg.go.dev/github.com/testcontainers/testcontainers-go/modules/minio) — Run, WithUsername, WithPassword, ConnectionString API verified
- [golang.testcontainers.org/modules/minio/](https://golang.testcontainers.org/modules/minio/) — official module docs, image format, available options
- `internal/storage/s3.go` — NewS3Backend, S3APIClient, UsePathStyle, endpoint config (read from codebase)
- `internal/daemon/daemon.go` — localizeInputs, delocalizeOutputs, walkAndLocalize, executeTaskS3 (read from codebase)
- `wdl/task/runner.go` — ContainerRunner interface (read from codebase)
- `internal/server/server_test.go` — newTestServer, in-process wiring pattern (read from codebase)
- `Makefile` — existing PID-file lifecycle pattern for server/daemon (read from codebase)

### Secondary (MEDIUM confidence)
- [mickey.dev/posts/go-build-tags-testing](https://mickey.dev/posts/go-build-tags-testing/) — `//go:build integration` pattern; verified against Go 1.17+ docs
- [pkg.go.dev/github.com/testcontainers/testcontainers-go@v0.41.0](https://pkg.go.dev/github.com/testcontainers/testcontainers-go) — TerminateContainer, CleanupContainer API

### Tertiary (LOW confidence)
- MinIO `/minio/health/live` endpoint — widely documented in MinIO docs; consistent across versions; treating as HIGH but not directly verified against the pinned image

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — testcontainers-go v0.41.0 pkg.go.dev verified; minio module API confirmed; aws-sdk-go-v2 already in go.mod
- Architecture: HIGH — derived from existing server_test.go patterns and daemon.go helper functions (both read directly)
- Pitfalls: HIGH — credential chain pitfall confirmed by S3Backend implementation; port pitfall confirmed by testcontainers-go ephemeral port design; os.Exit + defer pitfall is well-known Go pattern

**Research date:** 2026-03-13
**Valid until:** 2026-06-13 (testcontainers-go stable, but MinIO image tag pinned so image availability is the main risk)
