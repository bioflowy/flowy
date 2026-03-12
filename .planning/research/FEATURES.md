# Feature Research

**Domain:** S3/object storage backend for WDL workflow execution engine
**Researched:** 2026-03-12
**Confidence:** HIGH (storage abstraction interface, S3 staging/delocalization patterns), MEDIUM (MinIO-specific behaviors, config-driven backend selection details)
**Scope:** v1.2 milestone only. All v1.0/v1.1 features (parser, evaluator, Docker executor, DAG scheduler, REST API, distributed daemon) are already built and must remain fully functional.

---

## Feature Landscape

### Table Stakes (Users Expect These)

Features that any S3-enabled WDL engine must have. Missing any one makes the S3 backend incomplete or unusable.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Storage abstraction interface | All WDL engines (Cromwell, Nextflow, miniwdl) abstract filesystem access behind an interface so shared-FS and S3 backends are interchangeable. Without it, S3 is a bolt-on, not a real backend. | MEDIUM | Go interface with methods: `StageInputs(ctx, runID, inputs) error`, `CollectOutputs(ctx, runID, taskName, shardIndex) error`, `WriteInputsJSON(ctx, path, data) error`, `ReadOutputsJSON(ctx, path) ([]byte, error)`, `TaskDir(runID, taskName, shardIndex) string`. Existing shared-FS path logic moves into a concrete `LocalStorage` implementation. |
| S3 input staging (localization) | Cromwell, Nextflow, and miniwdl all download `s3://` File inputs to a local working directory before the Docker container starts. Containers cannot reach S3 directly in typical HPC setups. | HIGH | Before Docker task launch, daemon downloads all `s3://` paths in inputs.json to a local temp dir. Rewrites `FileValue.Value` to local path. Uses `github.com/aws/aws-sdk-go-v2/feature/s3/manager` Downloader for concurrent multipart download. |
| S3 output collection (delocalization) | After Docker task completes, output files must be uploaded to S3. The output paths returned to the scheduler become `s3://` URIs. This is the standard post-task step in every cloud workflow engine. | HIGH | After `task.RunTask` returns, daemon uploads each `FileValue` and `DirectoryValue` in the outputs map to `s3://{bucket}/{run_id}/tasks/{task_name}_{shard}/outputs/{filename}`. Rewrites value paths to `s3://` URIs. Uses `manager.Uploader` for concurrent multipart upload. |
| `s3://` scheme detection in path resolution | WDL engines treat `s3://bucket/key` URIs as distinct from local paths. Cromwell configures per-filesystem handlers; Nextflow treats `s3://` as transparent. flowy needs to dispatch to the S3 backend when the scheme is `s3://`. | MEDIUM | `cleanPath()` in `wdl/task/inputs.go` currently only handles local paths. Must be extended: if path starts with `s3://`, leave as-is and defer to the storage backend for download. Affects `jsonToWDLValue` for `FileType` and `DirectoryType`. |
| Workflow-level backend selection via `--storage` flag | Users need to specify `--storage=s3` for S3 and `--storage=local` (default) for shared FS at submit time. Cromwell uses configuration-file backend selection; Nextflow uses `process.executor`. A CLI flag is simpler and sufficient for flowy's use case. | MEDIUM | `flowy submit --storage s3` passes the backend selection through `SubmitRequest` to the server. Server stores `storage_backend` in the runs table and passes it to the dispatch function. Daemon reads backend from `TaskClaimResponse`. |
| Config-driven S3 defaults (bucket, region, endpoint, credentials) | Without a config default, every submit requires explicit `--storage` and `--bucket` flags. Production deployments configure once and run. Cromwell uses HOCON `filesystems.s3`; Nextflow uses `aws` config block. | LOW | New `StorageConfig` in `config.go`: `storage.backend` (default `"local"`), `storage.s3.bucket`, `storage.s3.region`, `storage.s3.endpoint_url` (for MinIO), `storage.s3.prefix` (key prefix, default `"flowy-runs"`). Credentials via standard AWS SDK chain: env vars → `~/.aws/credentials` → IAM role. |
| MinIO compatibility | MinIO is the standard self-hosted S3-compatible object store used in HPC environments without AWS access. All of miniwdl-aws, Cromwell, and Nextflow support MinIO via custom endpoint configuration. | MEDIUM | AWS SDK v2 supports custom endpoints via `aws.Config.EndpointResolverWithOptions`. Set `endpoint_url` in config to `http://minio:9000`. Must also set `UsePathStyle: true` (MinIO requires path-style addressing). |
| Spectool conformance on S3 backend (156/156) | The spectool conformance suite is the authoritative compliance benchmark for flowy. Running 156/156 on S3 means the S3 backend is a true drop-in for shared FS, not a partial implementation. | HIGH | Requires a MinIO instance in the test environment. Test harness substitutes MinIO endpoint + bucket for `base_dir`. Tests must pass without WDL changes — the S3 backend must be transparent to WDL tasks. |

### Differentiators (Competitive Advantage)

Features that go beyond the baseline S3 integration and add value specific to flowy's architecture.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Path-scheme auto-routing (relative → default backend, `s3://` → S3, absolute → local) | miniwdl requires explicit URI inputs; Cromwell requires per-backend config. Auto-routing based on URI scheme means users can mix local and S3 paths in the same workflow without configuration changes. | MEDIUM | Resolution logic: (1) if value starts with `s3://` → S3 backend regardless of default; (2) if value is an absolute local path → local FS; (3) if value is relative → resolve against `wdlDir` (existing behavior). This matches miniwdl's "built-in s3: support" pattern. |
| MinIO integration testing as part of CI | Most WDL engines (including miniwdl) rely on AWS accounts for S3 integration tests. A local MinIO container in `docker-compose` enables S3 tests without AWS credentials, which is critical for open-source CI. | MEDIUM | `docker-compose.yml` with MinIO service. Test harness starts MinIO, creates a test bucket, runs spectool against flowy with `--storage s3 --endpoint http://localhost:9000`. Teardown removes bucket. |
| No WDL change required for S3 backend | Cromwell requires different task `runtime` sections for cloud vs. local backends. Nextflow requires `publishDir` with S3 URI. In flowy, the storage backend is configured at runtime (`--storage` flag), not in the WDL source. A single WDL file runs on both local and S3. | LOW | This is a consequence of doing staging/delocalization in the engine layer (daemon), not in the WDL task. The design decision to keep WDL tasks storage-agnostic should be explicit and documented. |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Mount S3 bucket as filesystem via Mountpoint or s3fs | "S3 as a directory" feels simpler than explicit staging/delocalization | Mountpoint has read-only or limited write semantics; s3fs has performance problems with small files and rename operations; Docker containers cannot use host-mounted S3 without privileged mode or custom driver | Explicit staging (download before task) and delocalization (upload after task). The industry-standard WDL approach used by Cromwell, Nextflow, and miniwdl. |
| Streaming S3 inputs directly into containers | Avoids staging latency for large files | Docker containers cannot directly access S3 without AWS CLI or SDK; WDL tasks are user-defined and cannot be assumed to have AWS credentials; breaks portability | Stage to local temp dir before container start. Add a "reference data cache" optimization in v2 for frequently reused large files. |
| Per-task S3 backend (different tasks use different buckets) | Maximum flexibility for complex pipelines | WDL doesn't have per-task storage configuration; task declarations are storage-agnostic by design; adds complexity to dispatch logic and task claim protocol | Workflow-level backend selection. A single bucket per workflow run is sufficient and matches how Cromwell and Nextflow work. |
| Client-side input upload to S3 before submit | Mirrors the shared-FS pattern where the client stages files to `base_dir/runs/{id}/inputs/` | In the shared-FS model the client uploads because only the client has the files locally. For S3 inputs, files are already in S3 (that's why the user is using S3). Pre-upload from client duplicates data. | For S3 backend: inputs with `s3://` URIs are left as-is. The daemon downloads them at task execution time. Local inputs (absolute paths) are uploaded by the daemon to S3 before running the container. |
| GCS / Azure Blob Storage backend | Multi-cloud support | Tripling the storage backend implementations for a v1.2 milestone is out of scope; GCS and Azure have different SDK patterns | S3-compatible interface covers AWS S3 and MinIO. GCS supports S3-compatible API via interoperability mode. Defer native GCS/Azure support to v2+. |

---

## Feature Dependencies

```
[Storage abstraction interface]
    └──required by──> [LocalStorage implementation (wraps existing rundir.go logic)]
    └──required by──> [S3Storage implementation (new)]
    └──required by──> [Daemon: storage-aware task execution]
    └──required by──> [Server: storage-aware dispatch]

[S3 input staging (localization)]
    └──requires──> [Storage abstraction interface]
    └──requires──> [s3:// scheme detection in path resolution]
    └──requires──> [AWS SDK v2: s3/manager.Downloader]
    └──requires──> [Config: S3 bucket + region + endpoint]

[S3 output collection (delocalization)]
    └──requires──> [Storage abstraction interface]
    └──requires──> [S3 input staging] (same SDK config, same session)
    └──requires──> [AWS SDK v2: s3/manager.Uploader]

[s3:// scheme detection]
    └──requires──> [Storage abstraction interface]
    └──enhances──> [Path-scheme auto-routing differentiator]

[Workflow-level backend selection (--storage flag)]
    └──requires──> [Storage abstraction interface]
    └──requires──> [Config: StorageConfig struct in config.go]
    └──requires──> [API: SubmitRequest gains StorageBackend field]
    └──requires──> [API: TaskClaimResponse gains StorageBackend + S3Config fields]
    └──requires──> [DB: runs table gains storage_backend column]

[Config-driven S3 defaults]
    └──requires──> [Config: StorageConfig struct]
    └──enhances──> [Workflow-level backend selection] (flag overrides config default)

[MinIO compatibility]
    └──requires──> [Config: endpoint_url + path_style settings]
    └──requires──> [AWS SDK v2: custom EndpointResolver]
    └──enhances──> [S3 input staging + output collection]

[Spectool on S3 backend]
    └──requires──> [All above S3 features complete]
    └──requires──> [MinIO integration testing infrastructure]

[Path-scheme auto-routing (differentiator)]
    └──requires──> [s3:// scheme detection]
    └──requires──> [Storage abstraction interface]
```

### Dependency Notes

- **Storage abstraction interface must come first.** All other features depend on it. The interface defines the contract; both `LocalStorage` and `S3Storage` implement it. This is the single most important architectural decision for v1.2.
- **`s3://` scheme detection requires changes in `wdl/task/inputs.go`.** The `cleanPath()` function currently unconditionally calls `filepath.Clean()` and `filepath.Abs()`, which corrupts `s3://` URIs. This must be fixed as part of the abstraction work.
- **Daemon needs storage config from TaskClaimResponse.** The daemon is storage-agnostic in v1.1; it reads paths from `claim.InputsDir` and writes to `claim.OutputsDir`. For S3, the daemon needs to know the backend type and S3 config. `TaskClaimResponse` must be extended or a separate config endpoint added.
- **MinIO integration testing requires `outputs.json` path handling.** In the S3 backend, `outputs.json` is stored at `s3://bucket/prefix/runs/{id}/tasks/{name}_{shard}/outputs.json`. The server's `loadTaskOutputs()` in `dispatch.go` reads this via the storage interface, not `os.ReadFile`. This is a non-trivial change to `dispatch.go`.
- **Spectool tests pass local file paths.** Spectool generates inputs with local paths. With S3 backend, the daemon must upload those local inputs to S3 before running the container. This is a behavior change that must not break the `local` backend path (where local inputs are bind-mounted directly).

---

## MVP Definition

### Launch With (v1.2)

Minimum viable set for the S3 storage milestone.

- [ ] Storage abstraction interface (`internal/storage/storage.go`) with `Backend` interface — *foundation for everything else*
- [ ] `LocalStorage` implementation wrapping existing `store/rundir.go` logic — *preserves v1.1 behavior exactly*
- [ ] `s3://` scheme detection: fix `cleanPath()` to not corrupt S3 URIs — *correctness prerequisite*
- [ ] `S3Storage` implementation: S3 input staging (download to temp dir), output collection (upload to S3), `outputs.json` read/write via S3 — *core S3 feature*
- [ ] Config: `StorageConfig` struct with `backend`, `s3.bucket`, `s3.region`, `s3.endpoint_url`, `s3.prefix`, `s3.path_style` — *MinIO compatibility*
- [ ] Workflow-level backend selection: `--storage` flag on `flowy submit` and `flowy run`, stored in runs table, forwarded to daemon via `TaskClaimResponse` — *per-run control*
- [ ] MinIO integration testing: `docker-compose.yml` with MinIO, test harness that runs spectool against S3 backend — *compliance verification*
- [ ] Spectool 156/156 passing on both local and S3 backends — *non-negotiable quality gate*

### Add After Validation (v1.x)

Features to add once core S3 backend is working.

- [ ] Path-scheme auto-routing for mixed local/S3 inputs in the same workflow — *convenience; needs correctness validation first*
- [ ] Concurrent multipart upload/download tuning (part size, concurrency config) — *performance; not needed for correctness*
- [ ] Pre-signed URL support for output download without AWS credentials — *useful for sharing results*

### Future Consideration (v2+)

- [ ] GCS / Azure Blob Storage backends — *multi-cloud; requires separate SDK integrations*
- [ ] Reference data cache: skip re-download of unchanged `s3://` inputs across runs — *performance optimization; requires input hashing*
- [ ] HTTP import resolution for WDL files — *separate feature, not storage-related*
- [ ] Execution cache with S3-backed call cache — *complex; requires stable input fingerprinting*

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Storage abstraction interface | HIGH | MEDIUM | P1 |
| `s3://` scheme detection fix | HIGH | LOW | P1 |
| `LocalStorage` implementation | HIGH | LOW | P1 |
| `S3Storage`: input staging (download) | HIGH | HIGH | P1 |
| `S3Storage`: output collection (upload) | HIGH | HIGH | P1 |
| Config: `StorageConfig` | HIGH | LOW | P1 |
| `--storage` CLI flag + `TaskClaimResponse` extension | HIGH | MEDIUM | P1 |
| MinIO compatibility (`endpoint_url`, path-style) | HIGH | LOW | P1 |
| MinIO integration test infrastructure | HIGH | MEDIUM | P1 |
| Spectool 156/156 on S3 backend | HIGH | MEDIUM | P1 |
| Path-scheme auto-routing | MEDIUM | MEDIUM | P2 |
| Multipart upload/download tuning | LOW | LOW | P2 |
| Pre-signed URL output sharing | LOW | MEDIUM | P3 |
| GCS backend | MEDIUM | HIGH | P3 |

**Priority key:**
- P1: Must have for v1.2 launch
- P2: Should have, add in v1.2 polish
- P3: Nice to have, future consideration

---

## Competitor Feature Analysis

How Cromwell, Nextflow, and miniwdl handle S3 storage:

| Feature | Cromwell | Nextflow | miniwdl | flowy v1.2 approach |
|---------|----------|----------|---------|---------------------|
| Storage abstraction | `filesystems` config stanza; per-backend `PathBuilderFactory` | `Channel.fromPath()` handles `s3://` natively | Plugin interface (`s3:` built-in; `miniwdl-s3upload` plugin for delocalization) | Go `Backend` interface in `internal/storage/`; `LocalStorage` and `S3Storage` implementations |
| Input staging (localization) | Cromwell localizes files to VM before task; S3 via AWS CLI scripts injected by backend | Nextflow stages non-local files to `work/` dir before process; `stageInMode` controls copy/link | Downloads `s3://` URIs to `{run_dir}/download/` before task; mounts read-only | Daemon downloads `s3://` inputs to temp dir; rewrites paths in `inputs.json`; mounts to container |
| Output collection (delocalization) | Backend-specific scripts run after task; write to S3 root prefix | `publishDir "s3://bucket/path"` in process def triggers upload | `miniwdl-s3upload` plugin: progressive upload env var `MINIWDL__S3_PROGRESSIVE_UPLOAD__URI_PREFIX` | Daemon uploads `FileValue`/`DirectoryValue` outputs to S3 after `task.RunTask` returns; rewrites paths to `s3://` URIs |
| `s3://` path resolution | URI detected by `S3PathBuilderFactory`; resolves to `AmazonS3URI` | `s3://` detected in `Channel.fromPath`; transparent to Groovy code | `s3:` built-in: scheme checked before `cleanPath()` | `hasS3Scheme()` check before `filepath.Clean()` in `cleanPath()` |
| Backend selection | Config file: `filesystems.s3.enabled = true` per backend stanza | `process.executor` in `nextflow.config`; `workDir = "s3://..."` enables S3 | No per-run selection; configured at startup | `--storage=s3` CLI flag at submit; config default `storage.backend`; stored per-run in DB |
| MinIO / custom endpoint | Custom S3 endpoint via `aws.endpoint_url` in HOCON | `aws.client.endpoint = "http://..."` in `nextflow.config` | `AWS_ENDPOINT_URL` env var or `endpoint_url` in `~/.aws/config` | `storage.s3.endpoint_url` in `flowy.yaml`; passed to `aws.Config` custom resolver |
| Credentials | AWS CLI chain; IAM roles on EC2 | AWS SDK chain (env vars → `~/.aws` → IAM) | AWS SDK chain + IAM via metadata service | AWS SDK v2 default credentials chain: env vars → `~/.aws/credentials` → IAM instance role |
| Integration testing | Requires AWS account or LocalStack | Requires AWS account or LocalStack; some NF tests use S3 mock | Uses real S3 or `moto` (Python S3 mock) | MinIO in `docker-compose`; no AWS account needed for CI |

---

## Storage Interface Design Notes

Based on analysis of the existing codebase and competitor patterns, the storage abstraction interface should operate at the **task execution boundary** (daemon), not at the WDL value level. This is because:

1. **Docker containers are storage-agnostic.** Containers operate on local paths. The engine must present local paths to containers regardless of where data lives.
2. **The daemon is the only component that executes tasks.** `daemon/daemon.go` calls `task.RunTask`. This is the single place to insert staging/delocalization.
3. **The server manages paths, not data.** `dispatch.go` reads `outputs.json` from a path. The path format (`s3://` vs. local) must be abstracted.

The interface boundary should be:

```go
// Backend abstracts storage operations for task I/O.
// LocalBackend uses the shared filesystem; S3Backend uses AWS S3 / MinIO.
type Backend interface {
    // StageInputs downloads any remote (s3://) File/Directory values in inputs
    // to a local staging directory. Returns rewritten inputs with local paths.
    StageInputs(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, string, error)

    // CollectOutputs uploads local File/Directory output values to the configured
    // backend. Returns rewritten outputs with backend-native paths (s3:// or local).
    CollectOutputs(ctx context.Context, outputs map[string]values.Value, stagingDir string) (map[string]values.Value, error)

    // WriteJSON writes data to the given logical path (may be local or s3://).
    WriteJSON(ctx context.Context, path string, data []byte) error

    // ReadJSON reads data from the given logical path (may be local or s3://).
    ReadJSON(ctx context.Context, path string) ([]byte, error)

    // TaskDir returns the logical path for a task's working directory.
    TaskDir(runID, taskName string, shardIndex int) string

    // InputsJSONPath returns the logical path for a task's inputs.json.
    InputsJSONPath(runID, taskName string, shardIndex int) string

    // OutputsJSONPath returns the logical path for a task's outputs.json.
    OutputsJSONPath(runID, taskName string, shardIndex int) string
}
```

**Confidence:** MEDIUM — this is derived from codebase analysis + competitor patterns, not a verified specification. The exact method signatures should be refined during implementation.

---

## Sources

- [Cromwell Filesystems Overview](https://cromwell.readthedocs.io/en/stable/filesystems/Filesystems/) — S3 filesystem is experimental; per-backend filesystem stanza config (MEDIUM confidence)
- [Cromwell AWS Batch Tutorial](https://cromwell.readthedocs.io/en/stable/tutorials/AwsBatch101/) — S3 root prefix `s3://{bucket}/cromwell-execution`; tasks "write execution files back to S3" (MEDIUM confidence)
- [Nextflow Amazon S3 Documentation](https://www.nextflow.io/docs/latest/amazons3.html) — `s3://` scheme transparent via `Channel.fromPath`; credentials chain; MinIO custom endpoint config (HIGH confidence)
- [miniwdl Runner Reference](https://miniwdl.readthedocs.io/en/latest/runner_reference.html) — built-in `s3:` URI support; downloads to `{run_dir}/download/` before task; IAM role support (HIGH confidence)
- [miniwdl-s3upload (archived)](https://github.com/chanzuckerberg/miniwdl-s3upload) — progressive S3 upload plugin; `MINIWDL__S3_PROGRESSIVE_UPLOAD__URI_PREFIX` env var (MEDIUM confidence — archived, migrated)
- [AWS SDK for Go v2 — S3 Transfer Manager](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/sdk-utilities-s3.html) — `manager.NewUploader`, `manager.NewDownloader`; `PartSize`, `Concurrency` config; concurrent multipart (HIGH confidence)
- [pkg.go.dev — s3/manager](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/feature/s3/manager) — Uploader/Downloader API; thread-safe for concurrent calls (HIGH confidence)
- flowy source `internal/daemon/daemon.go` — `executeTask`: reads `inputs.json`, calls `task.RunTask`, writes `outputs.json`; injection point for staging/delocalization (HIGH confidence — direct code inspection)
- flowy source `wdl/task/inputs.go` — `cleanPath()`: unconditionally calls `filepath.Clean` + `filepath.Abs`; corrupts `s3://` URIs (HIGH confidence — direct code inspection)
- flowy source `internal/server/dispatch.go` — `loadTaskOutputs`: calls `os.ReadFile` on `outputs.json`; must be abstracted for S3 (HIGH confidence — direct code inspection)
- flowy source `internal/config/config.go` — current `Config` struct; no storage section; must be extended (HIGH confidence — direct code inspection)
- flowy source `internal/api/types.go` — `TaskClaimResponse.InputsDir` / `OutputsDir` are local FS paths; must be generalized for S3 (HIGH confidence — direct code inspection)

---

*Feature research for: flowy v1.2 S3 object storage backend*
*Researched: 2026-03-12*
