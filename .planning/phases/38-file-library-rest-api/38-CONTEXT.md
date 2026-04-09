# Phase 38: File Library REST API - Context

**Gathered:** 2026-04-09
**Status:** Ready for planning

<domain>
## Phase Boundary

Implement three REST endpoints under `/projects/{slug}/files`:
- `GET /projects/{slug}/files` — list managed files for the project
- `POST /projects/{slug}/files` — import a file (browser upload, S3 URI, or local server path)
- `DELETE /projects/{slug}/files/{id}` — remove a managed file record (record-only; no storage deletion)

No UI changes in this phase. No auto-registration (Phase 39). No file picker (Phase 40).

</domain>

<decisions>
## Implementation Decisions

### POST /files — Request Format
- **D-01:** Content-type sniffing to distinguish import modes:
  - `multipart/form-data` → browser file upload (same as `/uploads` handler pattern)
  - `application/json` → URI or local path import
- **D-02:** JSON body uses a single `uri` field: `{"uri": "s3://bucket/key"}` or `{"uri": "/srv/data/file.bam"}`. Server detects mode by `strings.HasPrefix(uri, "s3://")`.
- **D-03:** For browser upload, call `backend.UploadFileStream` directly (same storage path pattern as `/uploads` handler: `projects/{slug}/uploads/{uuid}/{filename}`). No shared helper — keeps handlers independent.
- **D-04:** After storing the file (any mode), insert a managed file record via `store.CreateManagedFile`. The handler returns the created `ManagedFile` record as JSON with HTTP 201.

### POST /files — S3 URI Mode
- **D-05:** Validate `s3://` URI by calling `backend.Stat(uri)` (which calls `HeadObject` internally). If stat fails (object not found, no access), return HTTP 400 with the error message.
- **D-06:** `size` in the managed file record comes from `backend.Stat(uri)`.

### POST /files — Local Path Mode (IMPORT-03)
- **D-07:** Add `ImportRoot string` field to `config.Config` (YAML key: `import_root`). If empty, local path import is disabled.
- **D-08:** If `import_root` is unset and a JSON `uri` that doesn't start with `s3://` is submitted, return HTTP 400 with message `"local path import not configured"`.
- **D-09:** If `import_root` is set, reject paths that do not have `filepath.Clean(import_root)` as a prefix (path traversal guard). Return HTTP 400 if the path escapes the root.
- **D-10:** For valid local paths, call `backend.Stat(uri)` to get file size.

### File Label
- **D-11:** Label is always auto-derived as `filepath.Base(path)` regardless of import mode:
  - Browser upload: `filepath.Base(header.Filename)` (multipart filename, sanitized)
  - S3 URI: `filepath.Base(key)` where key is the part after `s3://bucket/`
  - Local path: `filepath.Base(uri)`
- **D-12:** No client-provided label override at import time. Label editing is deferred to a future milestone.

### RBAC
- **D-13:** `GET /projects/{slug}/files` — requires `viewer` role (consistent with `GET /runs/`)
- **D-14:** `POST /projects/{slug}/files` — requires `executor` role (consistent with `POST /uploads`)
- **D-15:** `DELETE /projects/{slug}/files/{id}` — requires `executor` role (same as POST; any executor can manage the project file library)

### Response Format
- **D-16:** `GET` returns `{"files": [...]}` where each item has: `id`, `path`, `file_type`, `label`, `size`, `created_at`. `run_id` is omitted (not user-facing in Phase 38).
- **D-17:** `POST` returns the created file record as `{"file": {...}}` with HTTP 201.
- **D-18:** `DELETE` returns HTTP 204 No Content on success, HTTP 404 if the ID is not found.

### API Types
- **D-19:** Add `ManagedFileResponse`, `ListManagedFilesResponse`, and `ImportFileRequest` structs to `internal/api/types.go`.

### Claude's Discretion
- Whether `file_type` can be optionally supplied in the POST JSON body alongside `uri`. REQUIREMENTS.md only defines type via parameter_meta (Phase 39 ANNOT-01) — for Phase 38, `file_type` defaults to empty string.
- Exact field ordering in response JSON.
- Internal handler file name (suggest `handlers_files.go` following the `handlers_*.go` naming convention).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing Handler Patterns
- `internal/server/handlers_uploads.go` — browser upload pattern (multipart, BaseDir path, `backend.UploadFileStream`)
- `internal/server/handlers_projects.go` — project handler pattern (`projectFromContext`, `writeJSON`, `writeError`)
- `internal/server/server.go` lines 130-187 — route registration pattern under `/projects/{slug}`

### Store and Model
- `internal/store/store.go` — `ManagedFile` struct and CRUD interface methods (`CreateManagedFile`, `ListManagedFiles`, `GetManagedFileByID`, `DeleteManagedFile`)
- `internal/store/sqlite/managed_files.go` — SQLite implementations of all ManagedFile methods

### Storage Backend
- `internal/storage/backend.go` — `Backend` interface including `Stat(path string) (int64, error)` and `UploadFileStream`
- `internal/storage/s3.go` — `S3Backend.Stat` implementation (calls HeadObject)
- `internal/storage/local.go` — `LocalBackend.Stat` implementation (calls os.Stat)

### Config
- `internal/config/config.go` — `Config` struct; add `ImportRoot string \`yaml:"import_root"\`` field here

### API Types
- `internal/api/types.go` — Add `ManagedFileResponse`, `ListManagedFilesResponse`, `ImportFileRequest` here

### Requirements
- `.planning/REQUIREMENTS.md` — FILE-01, FILE-02, IMPORT-01, IMPORT-02, IMPORT-03 acceptance criteria
- `.planning/ROADMAP.md` Phase 38 success criteria — authoritative checklist

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `backend.UploadFileStream(path, reader)` — stores file bytes; works for both local and S3 backends
- `backend.Stat(path)` — returns file size; available for S3 URI and local path validation
- `projectFromContext(r.Context())` — extracts project from chi middleware context
- `writeJSON(w, statusCode, v)` / `writeError(w, statusCode, msg)` — standard response helpers
- `s.projectRoleRequired("viewer"|"executor")` — role middleware

### Established Patterns
- Route registration: `r.With(s.projectRoleRequired("executor")).Post("/files", s.handleImportFile)` under the `/{slug}` sub-router
- Handler file naming: `handlers_files.go`
- UUID generation: `uuid.New().String()` (github.com/google/uuid)
- Path traversal guard: `strings.HasPrefix(storagePath, expectedPrefix)` — already used in `handlers_uploads.go`

### Integration Points
- Route wiring: `internal/server/server.go` lines 187 area — add `/files` routes alongside `/uploads`
- Config: `internal/config/config.go` `Config` struct — add `ImportRoot` field
- API types: `internal/api/types.go` — add three new types
- Test file: `internal/server/handlers_files_test.go` (new)

</code_context>

<specifics>
## Specific Ideas

- For S3 URI label extraction: `filepath.Base` works on S3 keys since they use `/` separators and `filepath.Base` on Linux uses `/` as separator. Key `bucket/path/to/file.bam` → `Base("s3://bucket/path/to/file.bam")` = `file.bam`. Confirm this is correct behavior.
- `run_id` is NULL for all Phase 38 imports (manual imports have no originating run). Phase 39 auto-registration will populate `run_id`.
- `file_type` defaults to empty string `""` for all Phase 38 imports. Phase 39 ANNOT-01 populates it for auto-registered outputs.

</specifics>

<deferred>
## Deferred Ideas

None surfaced during discussion.

</deferred>

---

*Phase: 38-file-library-rest-api*
*Context gathered: 2026-04-09*
