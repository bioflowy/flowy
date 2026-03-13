# Phase 17: Daemon Staging - Context

**Gathered:** 2026-03-13
**Status:** Ready for planning

<domain>
## Phase Boundary

The daemon correctly localizes S3 inputs to POSIX paths before container execution and delocalizes container outputs back to S3, with no s3:// URIs reaching Docker containers. Inputs and outputs are delivered via API (not file-based). Local mode skips S3 staging entirely but uses the same API-based input/output delivery.

</domain>

<decisions>
## Implementation Decisions

### API-based input/output delivery
- Inputs (parsed WDL values map) and WDL content embedded directly in TaskClaimResponse — daemon never reads inputs.json from storage
- Outputs sent via POST /tasks/{id}/result with outputs map — daemon never writes outputs.json to storage
- Both local and S3 modes use API delivery — consistent daemon behavior regardless of storage mode
- Server writes inputs into TaskClaimResponse at claim time; server reads outputs from result API and stores in SQLite
- This replaces the file-based inputs.json/outputs.json handoff entirely

### Staging directory layout
- os.MkdirTemp("", "flowy-stage-") creates an isolated temp dir per task execution
- No configurable staging root — users control location via TMPDIR env var (standard Unix convention)
- Structure: stageDir/inputs/ for downloaded S3 files, container work dir for outputs
- Cleanup: os.RemoveAll(stageDir) immediately after successful delocalize

### Localize flow (S3 inputs → local POSIX paths)
- Daemon receives inputs map from TaskClaimResponse API
- Walk WDL values to find all File/Directory references with s3:// URIs
- For each s3:// reference: ParseS3URI → backend.DownloadFile(key) → write to stageDir/inputs/
- Rewrite s3:// paths to local stageDir paths in-memory before passing to RunTask
- Non-s3:// paths (absolute local paths) passed through unchanged

### Delocalize flow (container outputs → S3)
- Walk container work dir to discover output files
- For each file: compute relative path from work dir, construct S3 key as OutputsDir + "/" + relativePath
- Upload via backend.UploadFile with full s3:// URI construction (s3://bucket/prefix/key)
- Report outputs to server via result API with s3:// URIs as output paths
- Preserves directory structure in S3 (subdirs maintained via relative path)

### Output path format
- S3 mode: full s3:// URIs (e.g. s3://mybucket/pfx/runs/r1/tasks/align_0/result.vcf)
- Local mode: absolute filesystem paths (unchanged from today)
- Consistent with Phase 15 input resolution rules (s3:// → S3 backend, /abs → local filesystem)

### Error handling
- Localize failure: fail task immediately (AWS SDK v2 already retries 3x internally), clean up staging dir, report to server
- Delocalize failure: keep staging dir (output files preserved for recovery), report to server
- Fail on first error during delocalize (no aggregate errors) — already-uploaded files stay in S3 (no rollback)
- Error messages use phase-prefixed format: "localize: download s3://... : NoSuchKey" or "delocalize: upload runs/... : AccessDenied"

### Local-mode passthrough
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

</decisions>

<specifics>
## Specific Ideas

- The LED pattern (Localize-Execute-Delocalize) was established as a project decision — daemon handles all S3 I/O, containers always see local POSIX paths
- inputs.json and outputs.json files are eliminated in favor of API-based delivery — this is a significant architectural change from the current file-based handoff
- TaskClaimResponse needs two new fields: Inputs (map[string]any) and WDLContent (string)
- The result API needs to accept an outputs map in addition to status/error

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/storage/backend.go`: Backend interface with DownloadFile/UploadFile — daemon uses for S3 I/O
- `internal/storage/s3.go`: S3Backend with ParseS3URI, transfermanager uploads — ready for daemon use
- `internal/storage/factory.go`: NewBackend(cfg) — daemon needs equivalent construction from TaskClaimResponse fields
- `wdl/task/pathmap.go`: PathMap handles host→container path translation and bind mount generation — localize integrates here
- `wdl/task/task.go:RunTask()`: Creates execution directory, binds inputs, runs container — localize feeds into this

### Established Patterns
- Backend interface: value-in/value-out, no context.Context (S3Backend uses context.Background() internally)
- Error wrapping: fmt.Errorf with %w for error chain preservation
- TaskClaimResponse carries storage config (StorageType, StorageBucket, StorageEndpoint, StoragePrefix) from Phase 15
- Server dispatch.go writes inputs.json via Backend.UploadFile — will change to embed in TaskClaimResponse
- Server dispatch.go reads outputs.json via Backend.DownloadFile — will change to read from result API

### Integration Points
- `internal/daemon/daemon.go:executeTask()`: Main modification point — add localize/delocalize around container execution
- `internal/api/types.go:TaskClaimResponse`: Add Inputs and WDLContent fields
- `internal/api/types.go:TaskResultRequest`: Add Outputs field (or extend existing)
- `internal/server/handlers.go:handleClaim()`: Populate Inputs and WDLContent from stored task data
- `internal/server/handlers.go:handleResult()`: Read outputs from API response instead of file
- `internal/server/dispatch.go`: Server-side changes to support API-based input/output delivery

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 17-daemon-staging*
*Context gathered: 2026-03-13*
