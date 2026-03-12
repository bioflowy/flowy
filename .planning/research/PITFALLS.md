# Pitfalls Research

**Domain:** S3/object storage backend for WDL workflow engine (Go)
**Researched:** 2026-03-12
**Confidence:** HIGH (core pitfalls verified against official AWS docs, Cromwell issue tracker, and production post-mortems)

---

## Critical Pitfalls

### Pitfall 1: Passing s3:// URIs Into Docker Container Command Templates

**What goes wrong:**
A WDL task's command block refers to `${input_file}` expecting a POSIX path like
`/inputs/data.bam`. If the storage backend resolves WDL `File` values to `s3://bucket/key`,
the container receives an s3:// URI as its command argument. Standard bioinformatics tools
(samtools, bwa, gatk, bcftools) do not understand s3:// URIs and fail with "No such file
or directory: s3://...". The task appears to start correctly (Docker container runs, exit
code is non-zero), which looks like a task failure, not an infrastructure bug.

**Why it happens:**
It is tempting to make the S3 backend "transparent" by surfacing s3:// URIs as WDL File
values and letting the container handle them. This works only if every tool in every
container has s3:// support (via AWS CLI, mountpoint-s3, or similar). In practice, most
bioinformatics containers do not. Cromwell encountered this exact issue (issue #4356: "S3
localization not working, mount point missed in localized path").

**How to avoid:**
Use the daemon-downloads-and-mounts pattern that flowy already uses for shared FS.
The daemon (not the container) owns all S3 access. The execution flow must be:

1. Server stores task inputs as s3:// URIs in the task record.
2. Daemon receives task claim containing s3:// URIs for File inputs.
3. Daemon staging step: for each s3:// input URI, download to `<task_workdir>/inputs/<filename>`.
4. Daemon replaces s3:// URI with local POSIX path before Docker container launch.
5. Container sees only POSIX paths; executes as normal.
6. After container exits, daemon uploads output files from local paths back to s3://.

The WDL `File` type in command templates must always expand to a local POSIX path inside
the container's working directory, regardless of backend.

**Warning signs:**
- Command failures with messages like "No such file or directory: s3://..."
- Tasks succeed in local FS mode but fail in S3 mode with identical WDL.
- Container stdout/stderr contains "s3://" in its error messages.

**Phase to address:** Storage abstraction interface design phase. The abstraction must
distinguish "backend URI" (used for daemon-level I/O) from "container path" (used in
command template expansion). Bake this distinction into the initial interface design.

---

### Pitfall 2: Scatter Output Key Collision — Multiple Tasks Writing to the Same S3 Key

**What goes wrong:**
Multiple scatter tasks may produce output files with identical names (e.g., `output.bam`,
`result.vcf`). On the shared filesystem, each task runs in its own subdirectory under the
execution root, so paths are naturally isolated by directory. On S3, if the output key
prefix is derived from task name alone (e.g., `s3://bucket/<run-id>/MyTask/output.bam`),
all scatter branches write to the same key. PutObject succeeds silently — last writer wins,
no error, earlier results are permanently overwritten.

**Why it happens:**
The shared filesystem model uses the filesystem directory hierarchy for isolation. S3 has
no directories — only key strings. Key uniqueness must be enforced explicitly. It is easy
to generate key names from task names without including the scatter index, because the
scatter index is implicit in the directory structure on shared FS but must be explicit on S3.

**How to avoid:**
Output key structure must include the scatter shard index:
`<run-id>/<task-name>/<scatter-index>/<filename>`.

The scatter index (the array position, not a random UUID) must be threaded through from
the DAG scheduler to the key generation logic. Verify with an explicit integration test:
scatter over an array of 5 identical items, confirm 5 distinct output objects exist in
S3 (ListObjectsV2 count == 5), not 1.

**Warning signs:**
- Scatter output collection returns fewer WDL Array values than the input array length.
- S3 ListObjects after scatter shows fewer keys than scatter branches.
- All scatter shards produce outputs with identical content.

**Phase to address:** S3 backend implementation phase, specifically in task output key
generation. Cannot be fixed as a patch; requires the scatter index to be propagated
from DAG scheduler to key naming from the start.

---

### Pitfall 3: Incomplete Multipart Upload Parts Accumulating Silently

**What goes wrong:**
A daemon begins uploading a large task output (e.g., a 20GB BAM file) via S3 multipart
upload. The daemon host crashes, network drops, or the task is aborted mid-upload.
The initiated multipart upload is never completed or aborted. The partial parts remain in
S3 indefinitely — they do not appear in normal ListObjects results (invisible to the
workflow engine), but AWS charges for storage of every uploaded part. Over many workflow
runs with failures, hidden costs accumulate.

MinIO in test environments does not bill, so this pitfall is invisible during development
and only becomes a problem on real AWS S3.

**Why it happens:**
Developers test the happy path. The incomplete multipart upload cleanup code is the first
thing cut when under time pressure. The parts are invisible in the S3 console's object
view (only visible via ListMultipartUploads), so the problem is undetected until a billing
anomaly or an explicit audit.

**How to avoid:**
1. Use `manager.Uploader` from `github.com/aws/aws-sdk-go-v2/feature/s3/manager`. It
   wraps multipart upload and automatically calls AbortMultipartUpload on any upload
   error, including context cancellation.
2. In the daemon's task execution error path, always explicitly call AbortMultipartUpload
   for any uploads in progress when the task is cancelled or fails.
3. Configure an S3 lifecycle rule on the output bucket:
   `AbortIncompleteMultipartUploads` after 7 days. This is a safety net for any parts
   the daemon fails to clean up.
4. Write an integration test: start a large upload, cancel mid-way, verify
   ListMultipartUploads returns empty result (or the lifecycle rule is configured).

**Warning signs:**
- `aws s3api list-multipart-uploads --bucket <bucket>` returns non-empty results
  after workflow runs.
- S3 storage costs grow faster than completed output file sizes.
- No call to AbortMultipartUpload in the daemon's error handling paths.

**Phase to address:** S3 backend implementation phase. Abort-on-error must be in the
initial implementation; it cannot be added after the happy path is shipped.

---

### Pitfall 4: Using aws-sdk-go v1 Instead of aws-sdk-go-v2

**What goes wrong:**
aws-sdk-go (v1) reached end-of-support on July 31, 2025. Using v1 means unmaintained
security vulnerabilities, missing S3 features added since July 2025, and a go.mod entry
that triggers security scanners. Many tutorial articles, Stack Overflow answers, and
MinIO code samples still show v1 import paths (`github.com/aws/aws-sdk-go/service/s3`),
making it easy to import by accident.

**Why it happens:**
Go's module resolution fetches v1 without any deprecation warning. Search results and
community code samples have not yet fully migrated to v2. The import path for v1 looks
almost identical to v2, differing only in the presence of `/v2` in the path.

**How to avoid:**
Use exclusively `github.com/aws/aws-sdk-go-v2` and its sub-modules:
- Core: `github.com/aws/aws-sdk-go-v2/aws`
- Config: `github.com/aws/aws-sdk-go-v2/config`
- S3 service: `github.com/aws/aws-sdk-go-v2/service/s3`
- Multipart: `github.com/aws/aws-sdk-go-v2/feature/s3/manager`

For MinIO compatibility, set `UsePathStyle: true` in the S3 client options. MinIO in
Docker Compose environments does not resolve virtual-hosted-style bucket subdomains
(`bucket.localhost:9000`) — path-style addressing is required.

Add a `govulncheck` step to CI that will flag v1 use.

**Warning signs:**
- `go list -m all | grep aws-sdk-go` shows a version without `/v2` in the module path.
- Import statements containing `github.com/aws/aws-sdk-go/service/s3`.

**Phase to address:** Library selection at the first S3 code commit.

---

### Pitfall 5: Loading Large Files Into Memory for Upload or Download

**What goes wrong:**
The daemon downloads S3 inputs via `GetObject`, reads the entire response body into a
`[]byte` with `io.ReadAll`, then writes to disk. For genomic files (BAM files regularly
exceed 50GB, reference genomes can be 100GB+), this causes OOM on daemon hosts. Similarly,
uploading via `PutObject` with a preloaded `[]byte` body buffers the entire file in RAM
before sending.

The failure is silent during development with small test files and explosive in production
with real genomic data. The daemon host OOM-kills the entire daemon process, leaving
running tasks orphaned.

**Why it happens:**
`io.ReadAll` is idiomatic Go for small data and is the first thing that comes to mind.
The size of bioinformatics data is not obvious from the WDL workflow source — a task that
processes "one file" may be processing a 100GB reference genome.

**How to avoid:**
For downloads: stream directly from `GetObject` response body to disk:
```go
resp, err := s3Client.GetObject(ctx, &s3.GetObjectInput{...})
if err != nil { return err }
defer resp.Body.Close()
localFile, err := os.Create(localPath)
if err != nil { return err }
_, err = io.Copy(localFile, resp.Body)
```

For uploads: use `manager.Uploader.Upload` with an `os.File` as the body. The manager
handles chunking automatically with configurable part size and concurrency. Set
`PartSize: 64 * 1024 * 1024` (64MB parts) for large genomic files.

Never use `os.ReadFile` + `bytes.NewReader` for S3 upload of task outputs.

**Warning signs:**
- Any call to `io.ReadAll(resp.Body)` in the download path.
- Daemon host RSS grows proportionally to the size of files being staged.
- OOM errors in daemon logs during large file workflows.

**Phase to address:** S3 backend implementation phase, in the initial staging/collection
implementation.

---

### Pitfall 6: MinIO Repo Archived — Pinning to `latest` Docker Tag in Tests

**What goes wrong:**
The flowy test suite plans to use MinIO as the S3-compatible test backend (spectool
156/156 on S3 backend). The `minio/minio` GitHub repository was archived as read-only
on February 13, 2026 and is no longer maintained. Pinning to `minio/minio:latest` in
docker-compose.test.yml means CI may pull a broken or security-vulnerable image whenever
Docker Hub updates the latest tag, breaking CI without any code change.

Furthermore, MinIO's archived state means behavioral bugs will not be fixed. Tests that
pass on MinIO may silently not catch bugs that would appear on real AWS S3.

**Why it happens:**
Using `latest` is a convenience default. The project governance change was not publicized
widely in Go/workflow-engine circles at the time of v1.2 development.

**How to avoid:**
1. Pin the MinIO Docker image to a specific release tag in all test configuration:
   e.g., `minio/minio:RELEASE.2025-12-18T17-24-25Z`. Never use `latest`.
2. Abstract the S3 test backend: all integration tests read endpoint, access key, and
   secret key from environment variables (`S3_ENDPOINT`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`).
   This allows swapping MinIO for Garage, SeaweedFS, or real AWS S3 without changing
   test code.
3. Accept that MinIO-based testing covers API compatibility but not all behavioral
   nuances of production AWS S3 (error code formats, retry semantics differ).
4. Track alternatives: SeaweedFS and Garage are actively maintained S3-compatible stores
   that could replace MinIO as the test backend if MinIO Docker images stop working.

**Warning signs:**
- `docker-compose.test.yml` uses `minio/minio:latest`.
- Integration tests use hardcoded `http://localhost:9000` instead of env var.
- No documented procedure for swapping the S3-compatible test backend.

**Phase to address:** Integration test setup phase, before writing the first integration test.

---

### Pitfall 7: Not Implementing a `ParseS3URI` Helper — Scattered Inline String Parsing

**What goes wrong:**
S3 URIs must be parsed throughout the codebase: in the storage backend (splitting
`s3://bucket/key/path` into bucket + key), in the daemon's input staging (detecting whether
an input is an s3:// URI or a POSIX path), and in output collection (constructing return
URIs). Without a single helper, each location hand-rolls `strings.TrimPrefix`,
`strings.SplitN`, or `strings.Split` on the URI. Edge cases differ: what if the key is
empty? What if there are double slashes? What if the URI has a trailing slash? Each
location handles these differently, producing inconsistent behavior.

**Why it happens:**
String splitting is trivial, and developers implement it inline. The inconsistency is not
visible until an edge case triggers.

**How to avoid:**
Define one canonical function in the storage package at the start of implementation:

```go
// ParseS3URI parses an s3://bucket/key URI.
// Returns an error for any URI that is not a valid s3:// URI.
func ParseS3URI(uri string) (bucket, key string, err error)
```

All S3 URI parsing throughout the codebase must call this function. Test it with empty
bucket, empty key, trailing slash, double slash, and non-s3 URIs.

**Warning signs:**
- Multiple calls to `strings.Split(uri, "/")` or `strings.TrimPrefix(uri, "s3://")` in
  different files.
- Different error handling for malformed URIs in different parts of the code.

**Phase to address:** Storage abstraction interface phase, as one of the first utilities
defined before any S3 operations are implemented.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| `io.ReadAll(getObjectResp.Body)` for download | Simple one-liner | OOM on genomic files >1GB; daemon crash leaves tasks orphaned | Never for task input staging |
| Skip `AbortMultipartUpload` on error | Less error-handling code | Silent part accumulation; hidden S3 billing cost | Never |
| Hardcode `UsePathStyle: true` globally | MinIO works in dev | Virtual-hosted-style required by some S3-compatible endpoints | Only acceptable if `UsePathStyle` is config-driven per-backend |
| Pin `minio/minio:latest` in test docker-compose | No version maintenance | CI breaks on unexpected MinIO update; MinIO repo is now archived | Never in CI |
| Inline `strings.Split` for s3:// URI parsing | Quick to write | Inconsistent edge case handling across call sites | Never — use `ParseS3URI` helper |
| Check S3 file existence via ListObjects | One API call | List is eventually consistent in some MinIO versions; `HeadObject` is the correct check | Never — use `HeadObject` |
| Skip scatter index in output S3 key | Simpler key naming | Silent overwrites when scatter tasks produce same-named outputs | Never for workflow output keys |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| MinIO in Docker Compose | Use virtual-hosted-style URLs (`bucket.localhost:9000`) | Set `UsePathStyle: true` in S3 client options; MinIO in Docker doesn't resolve bucket subdomains without extra DNS config |
| aws-sdk-go-v2 `GetObject` response | Read body but skip `Close` or leave on error path | Always `defer resp.Body.Close()`; drain body before closing on error paths to allow connection reuse |
| `manager.Uploader` with unknown-size reader | Pass `io.Reader` from a stream | The uploader needs to seek to detect size for optimal part sizing; pass `os.File` or set explicit `PartSize`; for unseekable readers, set `PartSize` explicitly |
| MinIO bucket creation | Assume bucket exists | Call `CreateBucket` idempotently; MinIO returns `BucketAlreadyOwnedByYou` which should be treated as success, not error |
| S3 credentials for daemon | Embed in config YAML | Support env vars first (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`) via the SDK's default credential chain; allow explicit `StaticCredentialsProvider` for MinIO in config |
| Path-style vs virtual-hosted-style | Assume path-style works everywhere | Virtual-hosted-style is the AWS default; path-style is deprecated for new AWS buckets but required for MinIO; make this configurable |
| Real AWS S3 vs MinIO error codes | Test only on MinIO | Some S3 error responses differ between real AWS and MinIO (e.g., `NoSuchBucket` vs `NoSuchBucketPolicy`); test critical error handling against both |

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Sequential download of shared reference inputs across scatter tasks | 100-task scatter, each downloads the same 5GB reference genome sequentially | Cache input files on the daemon host by S3 URI + ETag; skip download if local copy is current | Scatter width > 5 with reference inputs > 500MB |
| Single-threaded upload of large task outputs | Upload of 20GB BAM file blocks all daemon worker slots during transfer | `manager.Uploader` with `Concurrency: 5`, `PartSize: 64MB`; upload runs concurrently with next task polling | Files > 1GB with default 5MB part size |
| Copying outputs between prefixes via GetObject+PutObject | Double data transfer cost for same-bucket reorganization | Use `CopyObject` for same-bucket copies; no data leaves S3 | Any file > 100MB copied within same bucket |
| Buffering `GetObject` body in memory | Daemon RSS grows with file size | `io.Copy(localFile, resp.Body)` — constant memory regardless of file size | Files > available daemon host RAM |
| Per-task S3 client initialization | Credential refresh overhead, connection pool not reused | Initialize one `s3.Client` per daemon process; inject into task executors; `s3.Client` is safe for concurrent use | Scatter width > 50 |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Store S3 secret key in `flowy.yaml` committed to version control | Credential exfiltration | Use env vars (`AWS_SECRET_ACCESS_KEY`) for credentials; document `flowy.yaml` as containing only non-sensitive config like endpoint and bucket name; add to `.gitignore` template |
| Use a single bucket root with no per-run prefix isolation | Task A can read or overwrite Task B's outputs | Enforce `<run-id>/` as the mandatory prefix for all S3 read/write operations; reject URIs that do not start with the expected run prefix |
| Allow arbitrary s3:// URIs from user-supplied WDL input JSON | Path traversal to buckets or prefixes the engine should not access | Validate that s3:// input URIs fall within the configured allowed bucket and optional path prefix; reject others with a clear error |
| Log presigned URLs at INFO level | URL in log allows unauthorized download for up to 7 days | Never log presigned URLs; log bucket + key as separate fields; never log credential values |

---

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Silent fallback from S3 backend to local FS when S3 is unavailable | User thinks workflow ran on S3; outputs are local; re-runs fail | Fail fast at workflow submit time: "S3 endpoint unreachable: <url>. Verify storage config." |
| Opaque s3:// paths in error messages | User sees `s3://bucket/uuid/task/output.bam` with no context for which task or scatter shard | Include human-readable task name and scatter index in all error context |
| No progress indication during large file staging | User sees no activity for minutes while 50GB reference downloads | Log download progress at 10% intervals at INFO level in daemon logs |
| Require `s3://` prefix even for local absolute paths in input JSON | Breaks existing workflows that use absolute paths | Honor path auto-resolution rules: absolute path = local FS, relative path = default backend, `s3://` = S3 — this is already in the feature spec |
| Confusing error when `UsePathStyle` is wrong | AWS returns `PermanentRedirect` or `InvalidBucket`; not obvious the issue is path-style vs virtual-hosted | Detect `PermanentRedirect` errors and suggest enabling/disabling `UsePathStyle` in config |

---

## "Looks Done But Isn't" Checklist

- [ ] **S3 backend basic upload/download:** Works for small test files — verify with files
  > 5GB (multipart threshold; tests real multipart behavior, not just `PutObject`)
- [ ] **Container path resolution:** s3:// input URIs in task records — verify container
  command arguments contain POSIX paths, never `s3://` URIs (inspect Docker run args)
- [ ] **Scatter key isolation:** Multiple scatter branches produce output — verify with
  `ListObjectsV2` that key count equals scatter array length, not 1
- [ ] **Multipart cleanup on failure:** Upload starts then task is cancelled — verify
  `ListMultipartUploads` is empty after cancellation (abort was called)
- [ ] **MinIO integration tests:** 156/156 spectool passes on MinIO — verify at least a
  sample of tests pass when pointed at real AWS S3 endpoint
- [ ] **Credential handling:** S3 client is configured — verify credentials come from env
  vars or explicit config, not hardcoded in source
- [ ] **Path-style config:** Works with MinIO (path-style) — verify `UsePathStyle` is
  configurable and not hardcoded globally
- [ ] **Error propagation:** S3 upload failure — verify daemon reports task as FAILED to
  server (not silent success with empty outputs)

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Incomplete multipart parts in S3 | LOW | Configure `AbortIncompleteMultipartUploads` lifecycle rule (one-time 5-minute fix); use `aws s3api list-multipart-uploads` to audit existing parts; DELETE is free |
| s3:// URIs reaching container | HIGH | Requires redesign of daemon input staging logic; all S3 integration tests must be re-run from scratch |
| Scatter output key collision | MEDIUM | Re-run affected workflows; fix key generation to include scatter index; outputs from corrupted runs cannot be recovered (last-writer-wins overwrote them) |
| aws-sdk-go v1 in go.mod | MEDIUM | `go get github.com/aws/aws-sdk-go-v2/...@latest`; update all import paths; automated but requires re-testing all S3 code paths |
| MinIO Docker image breaks in CI | LOW | Pin to specific release tag; if image unavailable, switch to Garage or SeaweedFS as S3 test backend (env var config makes this a one-line change) |
| OOM from large file buffering | MEDIUM | Replace `io.ReadAll` with streaming `io.Copy`; requires identifying every download/upload call site and verifying with large-file integration test |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| s3:// URI reaching container command | Storage abstraction interface design | Integration test: run WDL task with S3 input; inspect Docker container args to confirm POSIX path, never `s3://` |
| Scatter output key collision | S3 key generation (backend implementation) | Integration test: scatter 5 over same-named output; `ListObjectsV2` count == 5 |
| Incomplete multipart cleanup | S3 backend implementation (upload error path) | Unit test: AbortMultipartUpload called on cancelled upload; lifecycle rule documented |
| aws-sdk-go v1 | Library selection (first S3 code commit) | `go list -m all \| grep aws-sdk-go` shows only v2 module paths |
| MinIO version pinning | Integration test setup | `docker-compose.test.yml` pins MinIO by release tag, not `latest`; env var overrides documented |
| Large file OOM | S3 backend implementation (download/upload paths) | Memory test: stage 10GB file; verify daemon RSS stays under 500MB during transfer |
| Inline URI string parsing | Storage abstraction interface (first utility) | `ParseS3URI` exists and is unit-tested; no `strings.Split` on `s3://` outside this function |
| Credential in config YAML | S3 client initialization | Grep source for hardcoded key strings returns zero results; credentials loaded via env var |

---

## Sources

- [Amazon S3 Strong Read-After-Write Consistency (Dec 2020)](https://aws.amazon.com/blogs/aws/amazon-s3-update-strong-read-after-write-consistency/)
- [AWS All Things Distributed: Diving Deep on S3 Consistency (2021)](https://www.allthingsdistributed.com/2021/04/s3-strong-consistency.html)
- [AWS SDK for Go V2 — Retries and Timeouts](https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/retries-timeouts/)
- [AWS SDK for Go V2 — S3 Utilities (manager.Uploader)](https://aws.github.io/aws-sdk-go-v2/docs/sdk-utilities/s3/)
- [AWS: End of Support for aws-sdk-go v1 (July 31, 2025)](https://aws.amazon.com/blogs/developer/announcing-end-of-support-for-aws-sdk-for-go-v1-on-july-31-2025/)
- [AWS: Discovering and Deleting Incomplete Multipart Uploads](https://aws.amazon.com/blogs/aws-cloud-financial-management/discovering-and-deleting-incomplete-multipart-uploads-to-lower-amazon-s3-costs/)
- [AWS: Uploading and copying objects using multipart upload](https://docs.aws.amazon.com/AmazonS3/latest/userguide/mpuoverview.html)
- [MinIO GitHub in Maintenance Mode — InfoQ (Dec 2025)](https://www.infoq.com/news/2025/12/minio-s3-api-alternatives/)
- [MinIO minio/minio repository archived Feb 13, 2026](https://github.com/minio/minio)
- [MinIO Alternatives (SeaweedFS, Garage, RustFS, Ceph RGW)](https://dev.to/arash_ezazy_f69fb13acdd37/minio-alternatives-open-source-on-prem-real-world-credible-seaweedfs-garage-rustfs-and-ceph-36om)
- [Cromwell: Add support for staging INPUTS/OUTPUTS from/to AWS S3 (issue #3804)](https://github.com/broadinstitute/cromwell/issues/3804)
- [Cromwell: S3 localization mount point issue (issue #4356)](https://github.com/broadinstitute/cromwell/issues/4356)
- [miniwdl-s3upload plugin on PyPI](https://pypi.org/project/miniwdl-s3upload/)
- [Nextflow + Mountpoint for Amazon S3 on AWS Batch](https://aws.amazon.com/blogs/hpc/optimize-nextflow-workflows-on-aws-batch-with-mountpoint-for-amazon-s3/)
- [aws-sdk-go S3 manager memory issue (issue #2591)](https://github.com/aws/aws-sdk-go/issues/2591)
- [aws-sdk-go-v2 S3 upload manager memory discussion (#1650)](https://github.com/aws/aws-sdk-go-v2/discussions/1650)
- [Exploring S3 Mocking Tools: S3Mock, MinIO, LocalStack comparison](https://blog.localstack.cloud/2024-04-08-exploring-s3-mocking-tools-a-comparative-analysis-of-s3mock-minio-and-localstack/)

---
*Pitfalls research for: S3 object storage backend for flowy WDL workflow engine (v1.2)*
*Researched: 2026-03-12*
