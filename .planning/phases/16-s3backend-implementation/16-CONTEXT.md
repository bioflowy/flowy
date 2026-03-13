# Phase 16: S3Backend Implementation - Context

**Gathered:** 2026-03-13
**Status:** Ready for planning

<domain>
## Phase Boundary

Implement S3Backend that satisfies the Backend interface with streaming I/O, MinIO compatibility, multipart upload via transfermanager, and scatter key layout. S3Backend plugs into the existing factory.go placeholder. Daemon integration (localize/delocalize) is Phase 17.

</domain>

<decisions>
## Implementation Decisions

### Directory I/O on S3
- UploadDirectory: walk local directory, upload each file as a separate S3 object under a key prefix (key = prefix + relative path)
- DownloadDirectory: use ListObjectsV2 prefix scan to discover objects, download each to local path. No SQLite dependency for listing.
- CreateRunDirs/CreateTaskDirs: pure no-ops on S3 — PutObject creates key paths implicitly, no zero-byte marker objects

### Streaming variants
- Add UploadFileStream(path string, r io.Reader) error and DownloadFileStream(path string, w io.Writer) error to Backend interface
- UploadFileStream accepts io.Reader — caller controls lifecycle, S3Backend just reads
- DownloadFileStream writes to io.Writer — caller provides destination (e.g., os.File)
- Keep existing []byte methods (UploadFile/DownloadFile) for backward compatibility — LocalBackend and small-file callers still use them
- This changes the Backend interface — LocalBackend must also implement the streaming variants (thin wrappers over os.ReadFile/os.WriteFile with io.Reader/io.Writer)

### Error handling
- Use AWS SDK v2 built-in retry (default 3 attempts, exponential backoff) — no custom retry in S3Backend
- Wrap all errors with context: fmt.Errorf("s3: <operation> %s: %w", key, err)
- Map NoSuchKey to os.ErrNotExist: fmt.Errorf("s3: download %s: %w", key, os.ErrNotExist) — callers use errors.Is(err, os.ErrNotExist) consistently across backends
- No bucket existence check at construction time — fail lazily on first operation. Config.Validate() already ensures bucket is non-empty

### Multipart upload
- Use transfermanager for ALL uploads — it automatically uses PutObject for small files and multipart for large ones
- Use plain GetObject for downloads (no transfermanager) — daemon writes to local disk sequentially
- MultipartThreshold stays at 100MiB default (from Phase 15 StorageConfig)
- transfermanager threshold configured from StorageConfig.MultipartThreshold

### Claude's Discretion
- S3Backend struct fields and constructor signature
- Mock interface design for unit tests (S3APIClient with PutObject/GetObject)
- Import alias strategy (awsconfig vs config)
- Key construction helpers (how prefix + run/task paths compose into S3 keys)
- transfermanager initialization and configuration details

</decisions>

<specifics>
## Specific Ideas

- Phase 15 left a placeholder in factory.go `NewBackend()` for the `"s3"` case — S3Backend plugs in there
- Research identified `github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager` as the replacement for deprecated `feature/s3/manager`
- UsePathStyle must be enabled for MinIO compatibility (custom endpoint + path-style addressing)
- Scatter key layout already handled by existing TaskDir format: `{taskName}_{shardIndex}`

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/storage/backend.go`: Backend interface (11 methods) — S3Backend implements this, plus 2 new streaming methods
- `internal/storage/local.go`: LocalBackend — reference implementation, needs streaming variant wrappers added
- `internal/storage/factory.go`: NewBackend() with Phase 16 placeholder at the `"s3"` case
- `internal/storage/backend.go:ParseS3URI`: Already exists for s3:// URI parsing
- `internal/config/config.go:StorageConfig`: Has Type, Bucket, Endpoint, Region, Prefix, MultipartThreshold fields

### Established Patterns
- Backend interface uses value-in/value-out (no context.Context) — S3Backend uses context.Background() internally
- Config validation is fail-fast at server startup (config.Validate()), not at construction time
- Error wrapping with fmt.Errorf and %w for error chain preservation

### Integration Points
- `internal/storage/factory.go:NewBackend()` — replace LocalBackend{} placeholder with NewS3Backend(cfg)
- Backend interface definition — add UploadFileStream and DownloadFileStream methods
- `internal/storage/local.go` — add streaming variant implementations to LocalBackend

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 16-s3backend-implementation*
*Context gathered: 2026-03-13*
