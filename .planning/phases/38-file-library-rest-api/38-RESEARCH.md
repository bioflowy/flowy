# Phase 38: File Library REST API - Research

**Researched:** 2026-04-09
**Domain:** Go HTTP handler layer (chi router, multipart, path traversal guard, SQLite store)
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**POST /files — Request Format**
- D-01: Content-type sniffing to distinguish import modes: `multipart/form-data` → browser upload; `application/json` → URI or local path import
- D-02: JSON body uses a single `uri` field. Server detects S3 by `strings.HasPrefix(uri, "s3://")`.
- D-03: Browser upload calls `backend.UploadFileStream` directly. Storage path: `projects/{slug}/uploads/{uuid}/{filename}`. No shared helper.
- D-04: After storing (any mode), insert via `store.CreateManagedFile`. Return created `ManagedFile` as JSON 201.

**POST /files — S3 URI Mode**
- D-05: Validate S3 URI by calling `backend.Stat(uri)`. Failure → HTTP 400 with error message.
- D-06: `size` comes from `backend.Stat(uri)`.

**POST /files — Local Path Mode (IMPORT-03)**
- D-07: Add `ImportRoot string` to `config.Config` (YAML key: `import_root`). If empty, local path import is disabled.
- D-08: If `import_root` unset and non-s3 URI submitted → HTTP 400: `"local path import not configured"`.
- D-09: If `import_root` set, reject paths that do not have `filepath.Clean(import_root)` as a prefix → HTTP 400 if path escapes.
- D-10: For valid local paths, call `backend.Stat(uri)` to get file size.

**File Label**
- D-11: Label auto-derived as `filepath.Base(path)` regardless of import mode.
- D-12: No client-provided label override at import time.

**RBAC**
- D-13: `GET /projects/{slug}/files` requires `viewer` role.
- D-14: `POST /projects/{slug}/files` requires `executor` role.
- D-15: `DELETE /projects/{slug}/files/{id}` requires `executor` role.

**Response Format**
- D-16: `GET` returns `{"files": [...]}` — each item: `id`, `path`, `file_type`, `label`, `size`, `created_at`. `run_id` omitted.
- D-17: `POST` returns `{"file": {...}}` with HTTP 201.
- D-18: `DELETE` returns HTTP 204 on success, HTTP 404 if ID not found.

**API Types**
- D-19: Add `ManagedFileResponse`, `ListManagedFilesResponse`, `ImportFileRequest` to `internal/api/types.go`.

### Claude's Discretion
- Whether `file_type` can be optionally supplied in the POST JSON body alongside `uri` — for Phase 38, `file_type` defaults to empty string.
- Exact field ordering in response JSON.
- Internal handler file name (suggest `handlers_files.go`).

### Deferred Ideas (OUT OF SCOPE)
None.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| FILE-01 | User can view a list of managed files in a project, showing path, file type, size, and creation date | `GET /projects/{slug}/files` handler reads from `store.ListManagedFiles`; store + SQLite implementation already exists from Phase 37 |
| FILE-02 | User can delete a managed file record from the library (record-only; no storage deletion) | `DELETE /projects/{slug}/files/{id}` handler: call `GetManagedFileByID` (for 404 detection), then `DeleteManagedFile`; SQLite `DELETE` does not error on missing rows so pre-check is required |
| IMPORT-01 | User can import a file by uploading from browser | `POST /projects/{slug}/files` with `multipart/form-data`; mirrors `handleUpload` pattern exactly |
| IMPORT-02 | User can import a file by entering an S3/MinIO URI | `POST /projects/{slug}/files` with `application/json`; `backend.Stat(uri)` validates existence and returns size |
| IMPORT-03 | User can import a file by entering a server-accessible local path (confined to ImportRoot) | `POST /projects/{slug}/files` with `application/json`; `ImportRoot` in config; `filepath.Clean` prefix guard |
</phase_requirements>

---

## Summary

Phase 38 adds three REST endpoints that expose the `ManagedFile` store (built in Phase 37) through the HTTP layer. All of the hard work — database schema, CRUD methods, storage `Stat`, and `UploadFileStream` — already exists. This phase is a pure handler layer addition following well-established patterns already present in the codebase.

The implementation decomposes cleanly into five concerns: (1) wiring three routes under `/{slug}/files` in `server.go`, (2) adding three API types to `api/types.go`, (3) adding `ImportRoot` to `config.Config`, (4) writing `handlers_files.go` with three handlers, and (5) writing `handlers_files_test.go` with comprehensive coverage. No new dependencies are required.

The most subtle concern is the DELETE 404 behavior: `store.DeleteManagedFile` (SQLite) does not error when no row is found because `DELETE WHERE id=?` with no match simply affects zero rows. The handler must call `GetManagedFileByID` first to detect missing records and return HTTP 404 per D-18.

**Primary recommendation:** Write `handlers_files.go` modeled exactly on `handlers_uploads.go` for the multipart branch and `handlers_projects.go` for the JSON+RBAC+store-call branch. Wire routes in `server.go` immediately after the `/uploads` route registration.

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/go-chi/chi/v5` | already in go.mod | Route registration and middleware chaining | Project standard HTTP router |
| `github.com/google/uuid` | already in go.mod | UUID generation for ManagedFile.ID | Already used for all ID generation |
| `path/filepath` | stdlib | `filepath.Base` for label extraction, `filepath.Clean` for path guard | Standard Go path manipulation |
| `strings` | stdlib | `strings.HasPrefix(uri, "s3://")` for mode detection | Already used in uploads handler |
| `encoding/json` | stdlib | JSON decode of `ImportFileRequest` body | Project standard for JSON handlers |
| `net/http` | stdlib | HTTP handler signatures, status codes, `MaxBytesReader` | Project standard |
| `mime/multipart` | stdlib (via `r.ParseMultipartForm`) | Parsing browser uploads | Already used in `handleUpload` |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/go-chi/chi/v5` `URLParam` | already in go.mod | Extract `{id}` from DELETE route | Used in existing handlers for URL params |

**Installation:** No new dependencies. All required packages are already in `go.mod`.

---

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── api/
│   └── types.go              # Add ManagedFileResponse, ListManagedFilesResponse, ImportFileRequest
├── config/
│   └── config.go             # Add ImportRoot string field to Config struct
└── server/
    ├── handlers_files.go     # New: handleListFiles, handleImportFile, handleDeleteFile
    ├── handlers_files_test.go # New: comprehensive handler tests
    └── server.go             # Wire /files routes (lines 187 area)
```

### Pattern 1: Route Registration (after `/uploads`)
**What:** Add three `/files` routes inside the `/{slug}` sub-router, immediately after the `/uploads` line.
**When to use:** All new project-scoped endpoints follow this pattern.
**Example:**
```go
// Source: internal/server/server.go line 187 (existing uploads line shown for context)
r.With(s.projectRoleRequired("executor")).Post("/uploads", s.handleUpload)

// Add immediately after:
r.With(s.projectRoleRequired("viewer")).Get("/files", s.handleListFiles)
r.With(s.projectRoleRequired("executor")).Post("/files", s.handleImportFile)
r.With(s.projectRoleRequired("executor")).Delete("/files/{id}", s.handleDeleteFile)
```

### Pattern 2: Content-Type Dispatch in handleImportFile
**What:** Read `Content-Type` header to dispatch multipart vs JSON mode.
**When to use:** Single POST endpoint accepting two distinct request formats (D-01).
**Example:**
```go
// Source: verified against existing handlers_uploads.go pattern + CONTEXT.md D-01
func (s *Server) handleImportFile(w http.ResponseWriter, r *http.Request) {
    proj := projectFromContext(r.Context())
    if proj == nil {
        writeError(w, http.StatusNotFound, "project not found")
        return
    }
    ct := r.Header.Get("Content-Type")
    if strings.HasPrefix(ct, "multipart/form-data") {
        s.handleImportFileBrowser(w, r, proj)
    } else {
        s.handleImportFileURI(w, r, proj)
    }
}
```

### Pattern 3: Browser Upload Branch (mirrors handleUpload exactly)
**What:** Parse multipart, extract "file" field, sanitize filename, store, then create managed file record.
**When to use:** IMPORT-01 — `multipart/form-data` requests.
**Example:**
```go
// Source: internal/server/handlers_uploads.go — exact pattern to mirror
r.Body = http.MaxBytesReader(w, r.Body, 100<<20)
if err := r.ParseMultipartForm(32 << 20); err != nil {
    writeError(w, http.StatusBadRequest, "failed to parse multipart form (max 100 MB)")
    return
}
file, header, err := r.FormFile("file")
// ...
filename := filepath.Base(header.Filename)
uploadID := uuid.New().String()
storagePath := filepath.Join(s.cfg.BaseDir, "projects", proj.Slug, "uploads", uploadID, filename)

expectedPrefix := filepath.Clean(s.cfg.BaseDir) + string(filepath.Separator)
if !strings.HasPrefix(storagePath, expectedPrefix) {
    writeError(w, http.StatusBadRequest, "invalid upload path")
    return
}
if err := s.backend.UploadFileStream(storagePath, file); err != nil {
    writeError(w, http.StatusInternalServerError, "failed to store uploaded file")
    return
}
// After storing: determine size via backend.Stat(storagePath), then CreateManagedFile
size, _ := s.backend.Stat(storagePath)
```

### Pattern 4: S3 URI Validation and Size Fetch
**What:** Call `backend.Stat(uri)` on the raw S3 URI to validate existence and get size.
**When to use:** IMPORT-02 — `application/json` body where `uri` starts with `s3://`.
**Critical note:** `S3Backend.Stat` takes an S3 key (the path portion), not the full `s3://bucket/key` URI. The handler must call `storage.ParseS3URI(uri)` to split the URI, then pass only the key to `backend.Stat`. Review `S3Backend.Stat` implementation:
```go
// Source: internal/storage/s3.go line 278
func (b *S3Backend) Stat(path string) (int64, error) {
    out, err := b.s3c.HeadObject(context.Background(), &s3.HeadObjectInput{
        Bucket: aws.String(b.bucket),
        Key:    aws.String(path), // <-- expects KEY, not full URI
    })
```
So for S3 import: parse the URI, pass only the key to `backend.Stat`. The full URI (`s3://bucket/key`) is what gets stored as `ManagedFile.Path` for later use.

### Pattern 5: Local Path Traversal Guard (IMPORT-03)
**What:** Reject paths that escape `ImportRoot` using `filepath.Clean` prefix check.
**When to use:** Any local path import request when `ImportRoot` is configured.
**Example:**
```go
// Source: mirrors handlers_uploads.go path traversal guard pattern + CONTEXT.md D-09
importRoot := filepath.Clean(s.cfg.ImportRoot)
cleanedPath := filepath.Clean(uri)
if !strings.HasPrefix(cleanedPath, importRoot+string(filepath.Separator)) &&
    cleanedPath != importRoot {
    writeError(w, http.StatusBadRequest, "path is outside import root")
    return
}
size, err := s.backend.Stat(cleanedPath)
```

### Pattern 6: DELETE with 404 Detection
**What:** Fetch the record first to detect missing ID, then delete. SQLite `DELETE` does not error on zero rows affected.
**When to use:** D-18 — DELETE must return 404 if ID not found.
**Example:**
```go
// Source: sqlite/managed_files.go GetManagedFileByID returns error when not found
id := chi.URLParam(r, "id")
mf, err := s.store.GetManagedFileByID(r.Context(), id)
if err != nil {
    // error message contains "not found" — return 404
    writeError(w, http.StatusNotFound, "managed file not found")
    return
}
// Verify it belongs to this project (security: prevent cross-project deletion)
if mf.ProjectID != proj.ID {
    writeError(w, http.StatusNotFound, "managed file not found")
    return
}
if err := s.store.DeleteManagedFile(r.Context(), id); err != nil {
    writeError(w, http.StatusInternalServerError, "failed to delete managed file")
    return
}
w.WriteHeader(http.StatusNoContent)
```

### Pattern 7: ManagedFile → Response Conversion
**What:** Convert `store.ManagedFile` to `api.ManagedFileResponse` (omitting `run_id` per D-16).
**Example:**
```go
// Source: projectToResponse pattern in handlers_projects.go
func managedFileToResponse(f *store.ManagedFile) api.ManagedFileResponse {
    return api.ManagedFileResponse{
        ID:        f.ID,
        Path:      f.Path,
        FileType:  f.FileType,
        Label:     f.Label,
        Size:      f.Size,
        CreatedAt: f.CreatedAt.Format("2006-01-02T15:04:05Z"),
    }
}
```

### Anti-Patterns to Avoid
- **Calling `backend.Stat` with the full `s3://bucket/key` URI:** `S3Backend.Stat` expects an S3 key (no scheme). Always use `storage.ParseS3URI` first and pass only the key.
- **Relying on `DeleteManagedFile` to detect missing records:** SQLite DELETE with no matching row returns no error; always call `GetManagedFileByID` first.
- **Skipping project-ownership check in DELETE:** After fetching by ID, verify `mf.ProjectID == proj.ID` to prevent executor in project A from deleting records in project B.
- **Using `filepath.Join` for S3 URI label extraction:** Use `filepath.Base` directly on the URI string — on Linux `filepath.Base("s3://bucket/path/to/file.bam")` = `"file.bam"` because Linux filepath uses `/` as separator.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| S3 URI parsing | Custom string split | `storage.ParseS3URI(uri)` | Already handles edge cases: missing key, empty bucket, non-s3 scheme |
| Path traversal detection | Custom regex | `filepath.Clean` + `strings.HasPrefix` | Same pattern already in `handleUpload`; handles `..` normalization |
| Multipart form parsing | Manual MIME parsing | `r.ParseMultipartForm` + `r.FormFile` | Standard library handles boundary parsing, memory limits |
| UUID generation | Random string | `uuid.New().String()` | Already imported, cryptographically random |
| JSON response writing | `fmt.Fprintf` | `writeJSON(w, status, v)` | Project standard helper; sets Content-Type correctly |

---

## Critical Technical Finding: S3 Stat Key vs URI

`S3Backend.Stat(path string)` passes `path` directly as the S3 key in `HeadObjectInput.Key`. It does NOT parse `s3://` URIs. [VERIFIED: internal/storage/s3.go line 278-290]

For IMPORT-02, the handler flow must be:
1. Receive `uri = "s3://bucket/key/path/file.bam"`
2. Call `storage.ParseS3URI(uri)` → `bucket, key, err`
3. Call `s.backend.Stat(key)` → `size, err` (pass key only, not full URI)
4. Store `ManagedFile.Path = uri` (the original full URI, for future reference)
5. Label = `filepath.Base(uri)` = `"file.bam"` (works correctly on Linux)

If instead the handler calls `s.backend.Stat(uri)` with the full `s3://bucket/key` string, S3 HeadObject will receive `s3://bucket/key` as the key name, which will fail with a NoSuchKey error since that is not a valid S3 object key.

**LocalBackend.Stat** does NOT have this issue — it calls `os.Stat(path)` directly with the local path, so `backend.Stat(localPath)` works correctly for local paths.

---

## Common Pitfalls

### Pitfall 1: S3Backend.Stat Key vs URI Confusion
**What goes wrong:** Handler calls `backend.Stat("s3://bucket/key")` — HeadObject receives the full URI as the key → always fails with NoSuchKey.
**Why it happens:** `LocalBackend.Stat` takes a file path; developers assume `S3Backend.Stat` takes a URI. The interface doc says "path" not "uri".
**How to avoid:** Always call `storage.ParseS3URI(uri)` first, then pass only the `key` portion to `backend.Stat`.
**Warning signs:** S3 imports always return 400 even for valid objects.

### Pitfall 2: DELETE 404 Not Returned for Missing Records
**What goes wrong:** Handler calls `DeleteManagedFile(id)` directly — returns nil even when no row matched → handler returns 204 for nonexistent IDs.
**Why it happens:** SQLite `DELETE WHERE id=?` with zero matches does not error; this is standard SQL behavior.
**How to avoid:** Always call `GetManagedFileByID` first; the method wraps `sql.ErrNoRows` into a descriptive error.
**Warning signs:** Test for DELETE of nonexistent ID returns 204 instead of 404.

### Pitfall 3: Cross-Project File Deletion
**What goes wrong:** Executor in project A calls `DELETE /projects/slug-A/files/{id}` where `{id}` belongs to project B. `GetManagedFileByID` finds it, `DeleteManagedFile` succeeds.
**Why it happens:** `GetManagedFileByID` queries by ID only, not by project scope.
**How to avoid:** After fetching, assert `mf.ProjectID == proj.ID`; return 404 if mismatch (not 403, to avoid disclosing existence).
**Warning signs:** Cross-project deletion test passes when it should fail.

### Pitfall 4: filepath.Base on S3 URI Label
**What goes wrong:** Developer uses `path.Base` (from `"path"` package, URL-style) vs `filepath.Base` (OS path package) — on Linux both use `/` so it works, but semantics differ.
**Clarification from CONTEXT.md specifics section:** On Linux, `filepath.Base("s3://bucket/path/to/file.bam")` = `"file.bam"`. This is correct. No issue on Linux. The `filepath` package (not `path`) should be used to stay consistent with `handlers_uploads.go`.

### Pitfall 5: ImportRoot Not Applied to Absolute Paths After filepath.Clean
**What goes wrong:** `filepath.Clean("/srv/data/../etc/passwd")` = `"/etc/passwd"` — still passes if prefix check only uses `strings.HasPrefix(cleaned, importRoot)` without the trailing separator.
**Why it happens:** `"/srv/data2/file"` has `/srv/data` as a prefix of `"/srv/data2/..."` without the separator check.
**How to avoid:** Check prefix with separator: `strings.HasPrefix(cleaned, importRoot+string(filepath.Separator))` OR `cleaned == importRoot`.

---

## Code Examples

### API Types to Add (internal/api/types.go)
```go
// Source: [ASSUMED] — modeled on existing ProjectResponse, RunListItem patterns

// ManagedFileResponse is a single file entry in library responses.
type ManagedFileResponse struct {
    ID        string `json:"id"`
    Path      string `json:"path"`
    FileType  string `json:"file_type"`
    Label     string `json:"label"`
    Size      int64  `json:"size"`
    CreatedAt string `json:"created_at"`
}

// ListManagedFilesResponse is the response body for GET /projects/{slug}/files.
type ListManagedFilesResponse struct {
    Files []ManagedFileResponse `json:"files"`
}

// ImportFileRequest is the JSON request body for POST /projects/{slug}/files
// when Content-Type is application/json (S3 URI or local path import).
type ImportFileRequest struct {
    URI string `json:"uri"`
}
```

### Config Field to Add (internal/config/config.go)
```go
// Source: [ASSUMED] — modeled on existing Config struct field additions

// In Config struct:
ImportRoot string `yaml:"import_root"`
// No default needed — empty string means disabled (D-07/D-08)
```

### GET Handler
```go
// Source: [ASSUMED] — modeled on handlers_projects.go handleListProjects pattern
func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
    proj := projectFromContext(r.Context())
    if proj == nil {
        writeError(w, http.StatusNotFound, "project not found")
        return
    }
    files, err := s.store.ListManagedFiles(r.Context(), proj.ID)
    if err != nil {
        log.Printf("handleListFiles: list files for project %q: %v", proj.Slug, err)
        writeError(w, http.StatusInternalServerError, "failed to list files")
        return
    }
    resp := make([]api.ManagedFileResponse, len(files))
    for i, f := range files {
        f := f
        resp[i] = managedFileToResponse(&f)
    }
    writeJSON(w, http.StatusOK, api.ListManagedFilesResponse{Files: resp})
}
```

### Test Setup Pattern (handlers_files_test.go)
```go
// Source: [VERIFIED: internal/server/handlers_uploads_test.go setupUploadTest pattern]
func setupFilesTest(t *testing.T) (*server.Server, *sqlite.SQLiteStore, string, string, string, func()) {
    t.Helper()
    srv, st, cleanup := newAuthTestServer(t)
    createTestUser(t, st, "exec-1", "executor_user", "pass", "executor")
    createTestUser(t, st, "view-1", "viewer_user", "pass", "user")
    proj := createRegistryTestProject(t, st, "exec-1")
    addProjectMember(t, st, proj.ID, "view-1", "viewer")
    execToken := issueTestToken(t, "exec-1", "executor_user", "executor")
    viewToken := issueTestToken(t, "view-1", "viewer_user", "user")
    return srv, st, proj.Slug, execToken, viewToken, cleanup
}
```

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go standard `testing` package |
| Config file | none — `go test ./...` convention |
| Quick run command | `go test ./internal/server/... -run TestHandleFiles -v` |
| Full suite command | `go test ./internal/server/... -count=1` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| FILE-01 | GET returns list with correct fields | unit | `go test ./internal/server/... -run TestHandleListFiles` | Wave 0 |
| FILE-01 | GET empty list returns `{"files":[]}` not null | unit | `go test ./internal/server/... -run TestHandleListFiles_Empty` | Wave 0 |
| FILE-01 | GET viewer gets 200, unauthenticated gets 401 | unit | `go test ./internal/server/... -run TestHandleListFiles_Auth` | Wave 0 |
| FILE-02 | DELETE existing record returns 204 | unit | `go test ./internal/server/... -run TestHandleDeleteFile_Success` | Wave 0 |
| FILE-02 | DELETE nonexistent ID returns 404 | unit | `go test ./internal/server/... -run TestHandleDeleteFile_NotFound` | Wave 0 |
| FILE-02 | DELETE cross-project ID returns 404 | unit | `go test ./internal/server/... -run TestHandleDeleteFile_CrossProject` | Wave 0 |
| FILE-02 | DELETE viewer gets 403 | unit | `go test ./internal/server/... -run TestHandleDeleteFile_Auth` | Wave 0 |
| IMPORT-01 | POST multipart stores file and returns 201 with ManagedFile record | unit | `go test ./internal/server/... -run TestHandleImportFile_Browser` | Wave 0 |
| IMPORT-01 | POST multipart path traversal in filename is sanitized | unit | `go test ./internal/server/... -run TestHandleImportFile_Browser_PathTraversal` | Wave 0 |
| IMPORT-02 | POST S3 URI with valid object returns 201 | unit | `go test ./internal/server/... -run TestHandleImportFile_S3` | Wave 0 |
| IMPORT-02 | POST S3 URI with nonexistent object returns 400 | unit | `go test ./internal/server/... -run TestHandleImportFile_S3_NotFound` | Wave 0 |
| IMPORT-03 | POST local path within ImportRoot returns 201 | unit | `go test ./internal/server/... -run TestHandleImportFile_LocalPath` | Wave 0 |
| IMPORT-03 | POST local path traversal outside ImportRoot returns 400 | unit | `go test ./internal/server/... -run TestHandleImportFile_LocalPath_Traversal` | Wave 0 |
| IMPORT-03 | POST local path when ImportRoot unset returns 400 | unit | `go test ./internal/server/... -run TestHandleImportFile_LocalPath_NoImportRoot` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/server/... -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/server/handlers_files_test.go` — covers all requirements above (new file)
- [ ] Mock backend for S3 tests: reuse `mockBackend` or similar pattern from existing tests

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | `JWTMiddleware` already applied at project route group level |
| V3 Session Management | no | Stateless JWT — no sessions |
| V4 Access Control | yes | `s.projectRoleRequired("viewer"/"executor")` middleware |
| V5 Input Validation | yes | `filepath.Clean` + prefix check for local paths; `storage.ParseS3URI` for S3 URIs; `filepath.Base` to strip directory components from upload filenames |
| V6 Cryptography | no | No new crypto; UUID uses `crypto/rand` internally |

### Known Threat Patterns for This Stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Path traversal via local URI | Tampering / EoP | `filepath.Clean` then `strings.HasPrefix` with separator suffix (Pitfall 5 above) |
| Path traversal via upload filename | Tampering | `filepath.Base(header.Filename)` strips all directory components |
| Cross-project file deletion | EoP | Assert `mf.ProjectID == proj.ID` after `GetManagedFileByID` |
| Unauthorized access | EoP | `projectRoleRequired` middleware; JWT verified at group level |
| Oversized upload | DoS | `http.MaxBytesReader(w, r.Body, 100<<20)` before `ParseMultipartForm` |

---

## Environment Availability

Step 2.6: SKIPPED — Phase 38 is code/config-only changes. All storage backends (`LocalBackend`, `S3Backend`) are already wired in the project. No new external services required. Tests use in-memory SQLite (`sqlite.OpenMemory()`) and `LocalBackend` backed by `t.TempDir()`.

---

## Open Questions

1. **S3 import: which bucket does `backend.Stat(key)` use?**
   - What we know: `S3Backend` is constructed with a specific bucket from `cfg.Storage.Bucket`. `Stat(key)` issues HeadObject against that configured bucket.
   - What's unclear: For IMPORT-02, the user supplies `s3://different-bucket/key`. `ParseS3URI` extracts the bucket name, but `Stat` ignores it and always uses the configured bucket. If the user's URI references a different bucket than what flowy is configured to use, `Stat` will check the wrong bucket.
   - Recommendation: For Phase 38, store the full URI as `ManagedFile.Path` and use `ParseS3URI` only to extract the key for `Stat`. Accept that the backend must be configured for the same bucket as the URI. Document this as an operational constraint. This matches the CONTEXT.md D-05/D-06 intent — validation and size retrieval are scoped to the configured backend.

2. **S3 import: should `ManagedFile.Path` store the full URI or just the key?**
   - What we know: CONTEXT.md D-02 says the user provides `s3://bucket/key`. D-16 says `path` is returned in the response.
   - What's unclear: Phase 39/40 may need to resolve the path back to a URI for workflow inputs. Storing the full `s3://bucket/key` URI preserves maximum information.
   - Recommendation: Store the full URI string (`s3://bucket/key`) as `ManagedFile.Path`. This is consistent with how daemon code identifies S3 files (by full URI) and allows future phases to detect mode without re-querying.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `api.ManagedFileResponse` field names use snake_case JSON tags matching CONTEXT.md D-16 field names | Code Examples | Wrong tag names break API contract; easy to fix |
| A2 | `Config.ImportRoot` needs no default value (empty string = disabled) and no entry in `applyDefaults` | Code Examples | If a default is required, plans would need an additional task |
| A3 | `filepath.Base("s3://bucket/path/to/file.bam")` returns `"file.bam"` on Linux | Architecture Patterns | If incorrect, label extraction is wrong for S3 URIs — but this is verified behavior of Go's `path/filepath` on Linux |
| A4 | The S3 backend's `Stat` always uses the configured bucket, not the bucket in the URI | Open Questions | If bucket-aware behavior is expected, handler logic would need to construct a per-request S3 client — out of scope for Phase 38 |

**A3 is LOW risk** — `filepath.Base` on Linux uses `/` as separator; `s3://bucket/path/file.bam` ends with `/file.bam` → `filepath.Base` returns `file.bam`. This is deterministic on the target Linux platform [ASSUMED from Go stdlib docs, not run in test].

---

## Sources

### Primary (HIGH confidence)
- `internal/server/handlers_uploads.go` — multipart upload pattern, path traversal guard, `UploadFileStream` usage [VERIFIED: read in full]
- `internal/server/server.go` — route registration structure, `/{slug}` sub-router, `projectRoleRequired` middleware [VERIFIED: read in full]
- `internal/store/store.go` — `ManagedFile` struct, `Store` interface methods [VERIFIED: read in full]
- `internal/store/sqlite/managed_files.go` — SQLite implementations; confirmed `DeleteManagedFile` does not detect missing rows [VERIFIED: read in full]
- `internal/storage/backend.go` — `Backend` interface, `Stat` and `UploadFileStream` signatures [VERIFIED: read in full]
- `internal/storage/s3.go` — `S3Backend.Stat` passes path as HeadObject key directly [VERIFIED: read in full, lines 278-290]
- `internal/storage/local.go` — `LocalBackend.Stat` uses `os.Stat(path)` [VERIFIED: read in full]
- `internal/config/config.go` — `Config` struct; `ImportRoot` field does not yet exist [VERIFIED: read in full]
- `internal/api/types.go` — existing types; confirmed `ManagedFileResponse` not yet present [VERIFIED: read in full]
- `internal/server/handlers_auth_test.go` — `newAuthTestServer`, `issueTestToken`, `createTestUser` helpers [VERIFIED: read in full]
- `internal/server/handlers_uploads_test.go` — `setupUploadTest`, `doMultipartUpload`, test coverage patterns [VERIFIED: read in full]
- `internal/server/handlers_projects.go` — `projectToResponse`, `projectFromContext` usage [VERIFIED: read in full]

### Secondary (MEDIUM confidence)
- `internal/server/handlers_runs_test.go` — `createRunsTestProject` helper pattern for project-scoped test setup [VERIFIED: read partially]
- `.planning/phases/38-file-library-rest-api/38-CONTEXT.md` — all locked decisions [VERIFIED: read in full]

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries already in go.mod, all patterns verified in codebase
- Architecture: HIGH — patterns directly traced to existing verified code
- Pitfalls: HIGH — S3 Stat key issue verified by reading s3.go source; DELETE 404 issue verified by reading managed_files.go; cross-project issue inferred from GetManagedFileByID signature
- Test patterns: HIGH — test helpers verified in auth_test.go and uploads_test.go

**Research date:** 2026-04-09
**Valid until:** 2026-05-09 (stable Go codebase, no external API changes expected)
