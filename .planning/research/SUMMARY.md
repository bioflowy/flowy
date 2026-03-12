# Project Research Summary

**Project:** flowy — WDL 1.2 workflow engine (Go)
**Domain:** S3/object storage backend for distributed WDL execution engine
**Researched:** 2026-03-12
**Confidence:** HIGH

## Executive Summary

flowy v1.2 adds S3/MinIO object storage as an alternative backend to the existing shared-filesystem model. The core challenge is not the S3 API itself — it is the impedance mismatch between S3 (remote, key-value store) and Docker container execution (local POSIX filesystem, bind mounts). The industry-standard answer, validated by Cromwell, Nextflow, and miniwdl, is the Localize-Execute-Delocalize (LED) pattern: the daemon downloads S3 inputs to a local staging directory before launching the container, invokes the container against local POSIX paths (unchanged), then uploads outputs back to S3 after the container exits. The `wdl/task` execution layer remains entirely storage-agnostic.

The recommended approach centers on a narrow `Backend` interface in a new `internal/storage/` package. A `LocalBackend` wraps existing `store/rundir.go` logic (zero regression on all 156 spectool tests), and an `S3Backend` uses AWS SDK for Go v2 with streaming I/O. Storage backend selection flows from a `--storage` CLI flag through `SubmitRequest` to the server, which embeds addressing config (bucket, endpoint, prefix — but never credentials) in `TaskClaimResponse`. Daemons load credentials independently from their own config or IAM role. The build order is strict: interface first, then `LocalBackend` with spectool gate, then API extension, then `S3Backend`, then daemon staging logic, then full MinIO conformance.

The top risks are well-defined and preventable at design time. The most severe is allowing `s3://` URIs to reach Docker container command templates — bioinformatics tools do not understand S3 URIs and this failure mode is indistinguishable from a task failure. The second is scatter output key collision when the shard index is omitted from S3 key names (silent last-writer-wins overwrites). The MinIO GitHub repository was archived in February 2026, requiring all test configuration to pin to a specific MinIO release tag rather than use `minio:latest`. All three are architecture-time decisions, not patch-time decisions, and must be baked into the initial interface design.

---

## Key Findings

### Recommended Stack

The v1.0/v1.1 stack (Go 1.25, cobra CLI, chi v5, modernc.org/sqlite, goose, Docker SDK, testify, slog) is unchanged and carries forward. For v1.2, three package groups are added. The primary S3 client is `github.com/aws/aws-sdk-go-v2/service/s3` (v1.78.x) — the official AWS SDK that is also MinIO-compatible via `BaseEndpoint` + `UsePathStyle: true` options. Credentials loading uses `github.com/aws/aws-sdk-go-v2/config` (v1.32.x) with `credentials` for static/MinIO configuration. Integration testing uses `github.com/testcontainers/testcontainers-go/modules/minio` (v0.36.x) which manages MinIO container lifecycle within `go test` — no separate docker-compose file needed. Key rejections: aws-sdk-go v1 (end-of-support July 2025), `feature/s3/manager` (deprecated March 2026; superseded by pre-1.0 `transfermanager`), and `minio/minio-go/v7` as primary client (introduces a second S3 interface when AWS SDK v2 already speaks S3 to both AWS and MinIO). No third-party storage abstraction libraries (afero leaks POSIX semantics incompatible with S3; sajari/storage is unmaintained since 2020).

**Core technologies (v1.2 additions):**
- `aws-sdk-go-v2/service/s3` v1.78.x: S3 API — sole client for both AWS S3 and MinIO; v1 is end-of-support; verified March 2026
- `aws-sdk-go-v2/config` v1.32.x + `credentials` v1.17.x: credential chain (env vars, ~/.aws, IAM, static for MinIO) — same package for AWS and MinIO
- `testcontainers-go/modules/minio` v0.36.x: real S3-compatible server in CI — Docker-native lifecycle; spectool conformance requires real wire-protocol I/O, not mocks
- Hand-written `Backend` interface in `internal/storage/`: storage abstraction — narrower than any third-party library; covers exactly the staging lifecycle WDL needs
- Direct `PutObject`/`GetObject` with streaming `io.Copy`: single-part I/O; multipart deferred until transfer manager reaches v1.0 stability

See `.planning/research/STACK.md` for full rationale, version compatibility, and what NOT to use.

### Expected Features

The v1.2 MVP consists of 8 P1 deliverables. All are required for the milestone. The dependency chain is strictly ordered: storage abstraction interface must exist before any other feature. `LocalBackend` must pass the spectool gate before `S3Backend` code is written.

**Must have (table stakes):**
- Storage abstraction interface (`Backend` in `internal/storage/`) — foundation for all other v1.2 features
- `LocalBackend` wrapping existing `rundir.go` — preserves v1.1 behavior byte-for-byte; spectool gate
- `s3://` URI detection in `cleanPath()` (`wdl/task/inputs.go`) — correctness prerequisite; current code corrupts S3 URIs via `filepath.Abs`/`filepath.Clean`
- `S3Backend`: input staging (download to temp dir) and output collection (upload to S3) — core LED implementation
- `StorageConfig` struct in `config.go` with `type`, `bucket`, `region`, `endpoint`, `prefix`, `path_style` — MinIO compatibility
- `--storage` flag on `flowy submit`/`flowy run`, stored per-run in DB, forwarded in `TaskClaimResponse` — per-run backend control
- MinIO integration testing (testcontainers with pinned image tag) — CI without AWS credentials
- Spectool 156/156 passing on both local and S3 backends — non-negotiable quality gate

**Should have (competitive):**
- Path-scheme auto-routing: `s3://` URI = S3 backend regardless of default, absolute path = local FS, relative = default backend — enables mixed-source inputs without per-input configuration
- Concurrent multipart upload/download tuning (part size, concurrency) — performance for large genomic files; defer until benchmarks show need

**Defer (v2+):**
- GCS / Azure Blob Storage backends — separate SDK integrations; GCS supports S3-compatible API as a bridge path
- Reference data cache (skip re-download of unchanged `s3://` inputs across runs) — requires input hashing
- Pre-signed URL output sharing — useful for result distribution; not needed for correctness
- Execution cache with S3-backed call cache — complex input fingerprinting

See `.planning/research/FEATURES.md` for the full feature dependency graph and competitor feature analysis (Cromwell, Nextflow, miniwdl).

### Architecture Approach

The architecture introduces a new `internal/storage/` package as the sole boundary between workflow orchestration and object storage. The server gains a `backend storage.Backend` field injected at startup; `dispatch.go` replaces four direct `os.ReadFile`/`os.WriteFile` calls with backend method calls. The daemon gains two new phases around the existing `task.RunTask()` invocation: `localizeInputs` (downloads `s3://` File inputs to a per-task staging dir, rewrites paths to local POSIX paths) and `delocalizeOutputs` (uploads local output files to S3, rewrites paths to `s3://` URIs in outputs.json). The `wdl/task` package and `RunTask` are untouched — they see only local POSIX paths regardless of backend. Storage addressing config flows to the daemon via new fields in `TaskClaimResponse`; credentials never travel over the API (daemon reads credentials from its own config file or IAM role).

**Major components:**
1. `internal/storage/backend.go` — `Backend` interface: WriteTaskInputs, ReadTaskOutputs, WriteTaskOutputs, ReadRunOutputs, WriteRunOutputs, DownloadFile, UploadFile, CreateRunDirs
2. `internal/storage/local.go` — `LocalBackend`: wraps `store/rundir.go`; DownloadFile/UploadFile are no-ops returning the local path unchanged; enables spectool gating before any S3 code
3. `internal/storage/s3.go` — `S3Backend`: AWS SDK v2 client; streaming `io.Copy` for downloads; `PutObject` for uploads; key layout mirrors local FS (`{prefix}/runs/{runID}/tasks/{name}_{shard}/...`)
4. `internal/storage/factory.go` — `NewBackendFromConfig` (server startup) and `NewBackendFromClaim` (daemon per-task); credential injection separated from addressing config
5. `internal/daemon/daemon.go` (modified) — `localizeInputs` and `delocalizeOutputs` helpers bracket the existing `RunTask` call; staging dir cleaned up after delocalization
6. `internal/server/dispatch.go` (modified) — writes `inputs.json` to backend before creating the task in SQLite; reads `outputs.json` via backend after task completes

See `.planning/research/ARCHITECTURE.md` for full data flow diagrams, component change table, anti-patterns, and the 7-step suggested build order.

### Critical Pitfalls

1. **s3:// URI reaching Docker container command** — daemon must download and rewrite all File inputs to local POSIX paths before `RunTask`; this must be baked into the initial `Backend` interface contract, not added as a patch; verify with an integration test that inspects actual Docker run arguments; Cromwell encountered this exact issue (issue #4356)
2. **Scatter output key collision** — shard index must be in every S3 output key (`{prefix}/runs/{runID}/tasks/{name}_{shard}/outputs/{file}`); omitting it causes silent last-writer-wins overwrites when scatter tasks produce identically named outputs; verify with ListObjectsV2 count == scatter array length
3. **Incomplete multipart upload parts accumulating** — always call `AbortMultipartUpload` on any upload error or context cancellation; add S3 lifecycle rule `AbortIncompleteMultipartUploads` after 7 days as a safety net; this is invisible during MinIO testing (no billing) but a real cost on AWS S3
4. **Large file OOM** — never use `io.ReadAll(getObjectResp.Body)` for downloads; always stream via `io.Copy(localFile, resp.Body)`; genomic files routinely exceed available daemon host RAM; also pass `os.File` to upload, not a preloaded `[]byte`
5. **MinIO repo archived (Feb 2026), `minio:latest` unsafe** — pin the MinIO Docker image to a specific release tag (e.g., `RELEASE.2025-12-18T17-24-25Z`); read endpoint/credentials from env vars so the test backend is swappable to SeaweedFS or Garage if needed

See `.planning/research/PITFALLS.md` for 2 additional critical pitfalls (aws-sdk-go v1 import, inline URI string parsing), full integration gotchas, performance traps, security mistakes, and the "looks done but isn't" checklist.

---

## Implications for Roadmap

Based on combined research, five phases with a strict dependency chain. Phases 1-2 are pure refactors with spectool gates. Phases 3-5 introduce external network I/O. Any spectool regression in phases 1-2 is attributable to the refactor, not S3, which isolates debugging.

### Phase 1: Storage Abstraction Interface and LocalBackend

**Rationale:** Every subsequent component depends on the `Backend` interface existing and `LocalBackend` passing all 156 spectool tests. This phase proves the abstraction does not regress existing behavior before any S3 code is written. It is the highest-leverage, lowest-risk starting point and unblocks all other phases for parallel development.
**Delivers:** `Backend` interface definition, `LocalBackend` wrapping `store/rundir.go`, `factory.go` with `NewBackendFromConfig`, `StorageConfig` struct with defaults in `config.go`, server `Server` struct wired to use `LocalBackend`, daemon with empty `localizeInputs`/`delocalizeOutputs` stubs that pass through, `ParseS3URI` canonical helper with edge-case tests.
**Addresses:** Storage abstraction interface (P1), LocalBackend (P1), StorageConfig (P1 partial), ParseS3URI utility (prevents Pitfall 7)
**Avoids:** S3 awareness inside `wdl/task` (Anti-Pattern 1 in ARCHITECTURE.md — baked out by interface boundary), inline URI string parsing scattered across call sites (Pitfall 7)
**Gate:** spectool 156/156 passing against LocalBackend with no behavior change from v1.1

### Phase 2: API Extension and s3:// Path Guard

**Rationale:** The client `--storage` flag, the `TaskClaimResponse` storage fields, and the `cleanPath` guard must exist before daemon S3 logic is written. These are pure interface/data changes with no network calls. Must be gated by spectool to confirm local backend still works. Adding new fields alongside existing `InputsDir`/`OutputsDir` (not replacing them) preserves backward compatibility.
**Delivers:** `SubmitRequest.StorageOverride` field, `TaskClaimResponse` four new storage fields (StorageType, StorageBucket, StorageEndpoint, StoragePrefix), `handleClaim` populates storage fields from server config, `cleanPath` s3:// guard in `wdl/task/inputs.go`, `--storage {local|s3}` CLI flag wired through `flowy submit`/`flowy run`, `storage_backend` column in runs table.
**Addresses:** Workflow-level backend selection (P1), s3:// scheme detection (P1)
**Avoids:** Anti-Pattern 5 (replacing InputsDir/OutputsDir with S3 URIs — adds new fields alongside existing ones), credential leak in claim response (Anti-Pattern 2 — storage addressing only, no credentials)
**Gate:** spectool 156/156; confirm `s3://` path survives `cleanPath` without corruption

### Phase 3: S3Backend Core Implementation

**Rationale:** First phase introducing external network calls. Isolated in `internal/storage/s3.go`. Unit-tested with a mocked or minimal S3 client before wiring into daemon. All streaming and error-path patterns (OOM, abort on cancel) must be correct in this phase — they cannot be retrofitted.
**Delivers:** `S3Backend` implementing full `Backend` interface, streaming `DownloadFile` via `io.Copy` (not `io.ReadAll`), `UploadFile` via `PutObject` with `os.File` body, `WriteTaskInputs`/`ReadTaskOutputs`/`WriteRunOutputs`/`ReadRunOutputs` for JSON blobs, S3 client constructor with `UsePathStyle: true` for MinIO, `NewBackendFromClaim` factory combining claim addressing + daemon credentials.
**Uses:** aws-sdk-go-v2/service/s3 v1.78.x, aws-sdk-go-v2/config, aws-sdk-go-v2/credentials
**Avoids:** Large file OOM (Pitfall 5 — streaming from the start), aws-sdk-go v1 (Pitfall 4), credentials in TaskClaimResponse (Anti-Pattern 2), S3 operations in handleClaim hot path (Anti-Pattern 4)
**Gate:** Unit tests with mocked S3 client; `ParseS3URI` edge-case tests passing (empty bucket, empty key, trailing slash, double slash)

### Phase 4: Daemon Staging and Delocalization Logic

**Rationale:** Wires `S3Backend` into the daemon execution loop. This is the phase where the LED pattern is fully implemented and all three critical pitfalls (Pitfalls 1, 2, 3) can manifest. Must be tested against a real MinIO instance. The `localizeInputs` function must produce local POSIX paths only — no `s3://` URIs must reach `RunTask`. Scatter output keys must include shard index from the first implementation.
**Delivers:** `localizeInputs` (detects `s3://` inputs, downloads to staging dir, rewrites to local paths), `delocalizeOutputs` (uploads local File/Directory outputs, rewrites to `s3://` URIs), `newStagingDir` with deferred cleanup, daemon reads `inputs.json` via `backend.ReadTaskInputs`, daemon writes `outputs.json` with `s3://` paths via backend, S3 output key includes `{taskName}_{shardIndex}` to prevent scatter collision.
**Implements:** Localize-Execute-Delocalize pattern (Pattern 1 in ARCHITECTURE.md)
**Avoids:** s3:// URI in container command (Pitfall 1 — CRITICAL), scatter key collision (Pitfall 2 — shard index in key naming from start), incomplete multipart cleanup (Pitfall 3 — AbortMultipartUpload in error paths)
**Gate:** Integration tests against MinIO testcontainer; Docker run args inspection confirms no `s3://` in container paths; scatter test: ListObjectsV2 count == scatter array width after scatter workflow

### Phase 5: MinIO Integration Testing and Spectool Conformance on S3

**Rationale:** End-to-end validation. Spectool 156/156 on S3 backend is the non-negotiable milestone quality gate. MinIO test infrastructure must use a pinned image (not `latest`) and read endpoint/credentials from env vars to be swappable. All previous phases must pass their gates before this phase begins.
**Delivers:** MinIO testcontainer configuration with pinned image tag (`RELEASE.2025-12-18T17-24-25Z` or similar), spectool harness pointed at MinIO endpoint, 156/156 spectool passing on S3 backend, CI configuration with `TESTCONTAINERS_RYUK_DISABLED=true` for Docker-in-Docker environments, env var overrides (`S3_ENDPOINT`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`) documented for alternative backends.
**Addresses:** MinIO compatibility (P1), spectool 156/156 on S3 (P1), MinIO integration testing infrastructure (P1)
**Avoids:** MinIO `latest` pinning (Pitfall 6), silent fallback from S3 to local FS (UX pitfall)
**Gate:** spectool 156/156 on both local and S3 backends; CI passes without AWS credentials

### Phase Ordering Rationale

- Phases 1 and 2 have zero external network calls. Any spectool regression in these phases is attributable to the storage abstraction refactor alone, not S3 I/O. This isolation makes failures fast to diagnose.
- Phase 3 introduces external I/O but only within `internal/storage/s3.go`. Unit tests with mocked clients can verify the streaming and error-path patterns before wiring into daemon, keeping the feedback loop short.
- Phase 4 is the only phase that can trigger the three critical pitfalls (Pitfalls 1, 2, 3). Running it after Phase 3 unit tests ensures the S3Backend implementation is correct before the daemon wiring is added to the debugging surface.
- Phase 5 is purely verification. It cannot succeed until all previous phases pass their gates. Running spectool last means any failure is in end-to-end wiring or MinIO-specific behavior, not in component logic.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 3 (S3Backend):** `feature/s3/manager` is deprecated and `transfermanager` is pre-1.0 unstable. For files > 5 GB, manual multipart upload (~80 lines) is required. Validate whether any spectool conformance test uses files > 5 GB before finalizing whether multipart is in Phase 3 scope or deferred. If spectool uses large files, multipart must move to Phase 3 core.
- **Phase 4 (Daemon staging):** The `AbortMultipartUpload` cancel-path test requires MinIO. MinIO's behavior on partial upload cleanup may differ from AWS S3 error codes. Plan a dedicated cancel-mid-upload integration test and verify against both MinIO and real AWS S3 before declaring Phase 4 complete.
- **Phase 5 (Conformance on S3):** MinIO repo was archived February 2026. If the pinned MinIO testcontainers module stops working in CI, a fallback decision between SeaweedFS and Garage is needed. Confirm testcontainers module availability for these alternatives before locking in MinIO as the sole test backend.

Phases with standard patterns (skip research-phase):
- **Phase 1 (Storage abstraction + LocalBackend):** Pure Go refactor wrapping existing `store/rundir.go`. Established interface pattern. No new external dependencies. Direct codebase analysis already identifies all 4 call sites in `dispatch.go` and `handlers.go` that need updating.
- **Phase 2 (API extension + path guard):** Struct field additions and a 5-line guard in `cleanPath`. chi + JSON patterns are established in the existing codebase. No novel decisions.

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All v1.2 libraries verified via pkg.go.dev (March 2026). Key deprecations (aws-sdk-go v1, feature/s3/manager) confirmed from official sources. MinIO testcontainers module verified via official docs. UsePathStyle workaround confirmed via GitHub issue #2573. |
| Features | HIGH | Feature set derived from direct codebase analysis of 6 source files plus 3 competitor engines (Cromwell, Nextflow, miniwdl). Interface method signatures are MEDIUM confidence — exact signatures should be finalized in Phase 1 implementation using ARCHITECTURE.md as the starting point. |
| Architecture | HIGH | Based on direct codebase analysis of `internal/daemon/daemon.go`, `internal/server/dispatch.go`, `wdl/task/inputs.go`, `internal/api/types.go`, `internal/config/config.go`, `internal/store/rundir.go`. Build order derived from actual code dependency analysis. LED pattern validated against Cromwell and miniwdl. |
| Pitfalls | HIGH | 7 pitfalls verified against official AWS docs, Cromwell issue tracker (#3804, #4356), and production post-mortems. MinIO archive status confirmed from GitHub (Feb 13, 2026) and InfoQ (Dec 2025). OOM pitfall confirmed from aws-sdk-go GitHub issue #2591. |

**Overall confidence:** HIGH

### Gaps to Address

- **Exact `Backend` interface method signatures:** FEATURES.md and ARCHITECTURE.md show two slightly different interface designs. The canonical design must be finalized in Phase 1. ARCHITECTURE.md's version (coarse-grained whole-file operations with explicit `ReadTaskInputs`) is more complete and should be preferred. Difference: FEATURES.md uses `StageInputs`/`CollectOutputs` aggregate methods; ARCHITECTURE.md uses individual `DownloadFile`/`UploadFile` primitives. Individual primitives are easier to test in isolation.
- **Multipart upload threshold and spectool file sizes:** Whether single-part upload (covers files up to 5 GB) is sufficient for all 156 spectool tests must be verified before Phase 3 begins. If any spectool test uses files > 5 GB, manual multipart must be in Phase 3 core scope.
- **testcontainers-go vs. docker-compose:** STACK.md recommends testcontainers-go/modules/minio; FEATURES.md mentions docker-compose. Standardize on testcontainers (already a project dependency via v0.36.x in existing go.mod) for Phase 5. Use docker-compose only as a documented fallback.
- **MinIO alternative readiness:** If pinned MinIO release breaks in CI before v1.2 ships, the contingency is SeaweedFS or Garage. Verify testcontainers module availability for the chosen alternative before committing to MinIO as the sole CI backend.

---

## Sources

### Primary (HIGH confidence)
- flowy codebase (direct inspection): `internal/daemon/daemon.go`, `wdl/task/inputs.go`, `internal/server/dispatch.go`, `internal/config/config.go`, `internal/api/types.go`, `internal/store/rundir.go`
- [pkg.go.dev: aws-sdk-go-v2/service/s3 v1.78.x](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/s3) — S3 client API verified March 2026; 5,227 importers
- [pkg.go.dev: feature/s3/manager](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/feature/s3/manager) — deprecated March 2026 status confirmed
- [pkg.go.dev: feature/s3/transfermanager](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager) — v0.1.8 unstable status confirmed
- [golang.testcontainers.org/modules/minio](https://golang.testcontainers.org/modules/minio/) — Run() function signature, MinIO module docs
- [github.com/aws/aws-sdk-go-v2 issue #2573](https://github.com/aws/aws-sdk-go-v2/issues/2573) — UsePathStyle confirmed working for MinIO
- [github.com/aws/aws-sdk-go](https://github.com/aws/aws-sdk-go) — v1 end-of-support declaration
- [AWS: end-of-support for aws-sdk-go v1 (July 31, 2025)](https://aws.amazon.com/blogs/developer/announcing-end-of-support-for-aws-sdk-for-go-v1-on-july-31-2025/)
- [AWS: Discovering and Deleting Incomplete Multipart Uploads](https://aws.amazon.com/blogs/aws-cloud-financial-management/discovering-and-deleting-incomplete-multipart-uploads-to-lower-amazon-s3-costs/)
- [Cromwell issue #4356: S3 localization mount point missed](https://github.com/broadinstitute/cromwell/issues/4356) — s3:// URI in container commands confirmed real production issue
- [Cromwell filesystems overview](https://cromwell.readthedocs.io/en/stable/filesystems/Filesystems/) — LED pattern validated

### Secondary (MEDIUM confidence)
- [miniwdl runner reference](https://miniwdl.readthedocs.io/en/latest/runner_reference.html) — built-in `s3:` URI support; download-to-run-dir pattern
- [Nextflow Amazon S3 docs](https://www.nextflow.io/docs/latest/amazons3.html) — transparent s3:// path handling; MinIO custom endpoint config
- [Cromwell issue #3804: S3 input/output staging](https://github.com/broadinstitute/cromwell/issues/3804)
- [MinIO docs: UsePathStyle requirement](https://docs.min.io/docs/how-to-use-aws-sdk-for-go-with-minio-server.html)
- [aws-sdk-go-v2 discussions #1650: S3 upload manager memory](https://github.com/aws/aws-sdk-go-v2/discussions/1650)

### Tertiary (supporting/informational)
- [InfoQ Dec 2025: MinIO in maintenance mode](https://www.infoq.com/news/2025/12/minio-s3-api-alternatives/)
- [github.com/minio/minio archived Feb 13, 2026](https://github.com/minio/minio)
- [MinIO alternatives: SeaweedFS, Garage, RustFS, Ceph RGW](https://dev.to/arash_ezazy_f69fb13acdd37/minio-alternatives-open-source-on-prem-real-world-credible-seaweedfs-garage-rustfs-and-ceph-36om)
- [aws-sdk-go issue #2591: S3 manager OOM](https://github.com/aws/aws-sdk-go/issues/2591)

---
*Research completed: 2026-03-12*
*Ready for roadmap: yes*
