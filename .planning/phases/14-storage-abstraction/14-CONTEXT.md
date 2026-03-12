# Phase 14: Storage Abstraction - Context

**Gathered:** 2026-03-12
**Status:** Ready for planning

<domain>
## Phase Boundary

Define a Backend interface in internal/storage/ that abstracts file operations (upload, download, JSON read/write, directory operations) and path layout (run dirs, task dirs). Wrap existing shared filesystem code as LocalBackend with zero behavior change. All 156 spectool tests must pass unchanged after refactor. ParseS3URI helper for future phases.

</domain>

<decisions>
## Implementation Decisions

### Interface granularity
- Whole-file operations: Upload(path, []byte), Download(path) []byte, ReadJSON(path, &v), WriteJSON(path, v)
- NOT stream-based (io.Reader/io.Writer) — keep callers simple
- Path-building helpers (RunInputsDir, TaskDir, etc.) included in the Backend interface — encapsulates both local FS paths and S3 key layout
- Separate methods for File and Directory types — WDL distinguishes these, so the interface should too (UploadFile + UploadDirectory / DownloadFile + DownloadDirectory)
- Multipart upload threshold configurable in YAML (e.g. `storage.multipart_threshold: 100MB`) with a sensible default (e.g. 64MB)

### Directory semantics on S3
- CreateRunDirs/CreateTaskDirs create zero-byte marker objects on S3 (e.g. `runs/RUN_ID/.marker`) so ListObjects shows the "directory"
- WDL Directory type: recursive upload (each file individually under S3 key prefix), not tar/archive
- File listing uses SQLite tracking (record uploaded files in DB), not ListObjectsV2 prefix scan
- Cleanup uses soft delete: mark deleted in SQLite, keep S3 objects for recovery. Storage cost accumulates but data is never lost.

### Claude's Discretion
- Exact Backend interface method signatures
- Internal package structure within internal/storage/
- How LocalBackend wraps rundir.go (thin delegation vs copy)
- ParseS3URI implementation details

</decisions>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches. The key constraint is zero behavior change for existing shared FS workflows after the refactor.

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/store/rundir.go`: 5 functions (RunInputsDir, RunOutputsDir, TaskDir, CreateTaskDirs, CreateRunDirs) — pure path computation + os.MkdirAll. These become the core of LocalBackend.
- `internal/store/store.go`: Store interface (SQLite persistence) — separate from filesystem. File tracking for S3 listing can extend this.

### Established Patterns
- `internal/store/store.go` defines a Store interface with SQLite implementation — same pattern should be used for Backend interface
- `internal/server/dispatch.go` has `loadTaskOutputs()` using `os.ReadFile` — needs to go through Backend
- `wdl/task/inputs.go` and `wdl/workflow/convert.go` both have `cleanPath()` that uses filepath.Abs — will corrupt s3:// URIs (fix in Phase 15)

### Integration Points
- Server (`dispatch.go`): calls rundir functions + os.ReadFile for outputs.json — inject Backend
- Daemon (`daemon.go`): uses paths from TaskClaimResponse for task I/O — inject Backend for S3 localize/delocalize (Phase 17)
- Handlers (`handlers.go`): creates run dirs on workflow submit — inject Backend

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 14-storage-abstraction*
*Context gathered: 2026-03-12*
