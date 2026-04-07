# Phase 32: API Foundation - Research

**Researched:** 2026-04-07
**Domain:** Go HTTP handlers, WDL AST introspection, multipart file upload
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Input Schema Endpoint**
- Response includes `name` (string), `type` (string in WDL notation), `optional` (bool), and `default` (string or null) per input
- WDL types represented as human-readable WDL notation: `"String"`, `"Int"`, `"Array[File]"`, `"Map[String, Int]"` — consistent with WDL spec
- Support both workflow and task registry entries — both use `ast.Decl`, same response shape
- Missing tag returns 404 with `{"error": "tag not found"}`, consistent with existing registry pull behavior

**File Upload Endpoint**
- Route: `POST /projects/{slug}/uploads` — project-scoped, outside registry namespace
- Body format: multipart form-data with single file field — standard HTTP upload, works with browser and CLI
- Storage: project-scoped directory under configured storage backend (`{base_dir}/projects/{slug}/uploads/{uuid}/{filename}`) — reuses existing storage layer, S3-compatible
- Response: `{"path": "/absolute/or/s3://path"}` — the server-accessible path for File input fields per ADR 0001

**Auth & Constraints**
- Input schema endpoint: `viewer` minimum role — reading schema is a read operation
- Upload endpoint: `executor` minimum role — uploading files is prerequisite to execution
- Upload size limit: 100 MB via `http.MaxBytesReader` — prevents abuse, sufficient for WDL input files
- No file type restriction — WDL File inputs can be any type (BAM, VCF, FASTQ, etc.)

### Claude's Discretion

No specific requirements — open to standard approaches following existing codebase patterns.

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| API-01 | Server provides input schema endpoint for registry entries (`GET /projects/{slug}/registry/{name}/inputs`) | `ast.Decl.Type.TypeName()` gives WDL notation; `ast.Decl.IsOptional()` is already on the `types.Type` interface; `ast.Decl.Expr` holds default expression. Parser is already used in `handlers_registry.go`. |
| API-02 | Server provides file upload endpoint for File-typed workflow inputs | `storage.Backend.UploadFileStream()` streams multipart to storage; `http.MaxBytesReader` enforces 100 MB limit; `uuid.New().String()` provides the upload UUID. Pattern matches existing `handleFileDownload`. |
</phase_requirements>

## Summary

Phase 32 adds two new HTTP endpoints to `flowy-server`. Both are straightforward extensions within the existing Chi v5 router/handler/DTO pattern used throughout the codebase. No new dependencies are required.

The input schema endpoint (`GET /projects/{slug}/registry/{name}/inputs?tag=<tag>`) reuses the existing registry pull flow: look up the version by tag, parse the WDL content with `parser.Parse`, then walk `doc.Workflows[0].Inputs` or `doc.Tasks[0].Inputs` (both are `[]*ast.Decl`). The `types.Type` interface already provides `TypeName()` which returns proper WDL notation (`"Array[File]"`, `"Map[String,Int]"`, etc.), and `IsOptional()` for the optional flag. Extracting a `default` string from `ast.Decl.Expr` requires a type switch over literal expression types.

The file upload endpoint (`POST /projects/{slug}/uploads`) uses `r.ParseMultipartForm` with `http.MaxBytesReader` for the 100 MB guard, extracts the file from the multipart form, and streams it to `s.backend.UploadFileStream()`. The storage path follows the decided convention `{base_dir}/projects/{slug}/uploads/{uuid}/{filename}`. Both the local and S3 backends already implement `UploadFileStream`.

**Primary recommendation:** Add both handlers to a new file `internal/server/handlers_uploads.go` (upload) alongside extending `handlers_registry.go` (inputs), add DTO types to `internal/api/types.go`, and register routes in `newRouter()` following existing `r.Route("/{name}", ...)` and `r.With(...)` patterns.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/go-chi/chi/v5` | v5 (already in go.mod) | HTTP routing, URL params, middleware chaining | Already used throughout server |
| `github.com/google/uuid` | already in go.mod | Generate upload directory UUID | Already used in handlers.go |
| `net/http` (stdlib) | Go 1.25 | `ParseMultipartForm`, `MaxBytesReader`, `FormFile` | Standard library, no new dependency |
| `github.com/bioflowy/flowy/wdl/parser` | internal | Parse WDL source to `*ast.Document` | Already used in handlers_registry.go |
| `github.com/bioflowy/flowy/wdl/ast` | internal | `ast.Decl`, `ast.Workflow`, `ast.Task` | Already used in handlers.go |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `path/filepath` (stdlib) | Go 1.25 | Construct upload storage paths | In upload handler path building |
| `fmt` (stdlib) | Go 1.25 | Error message formatting | In 404 messages matching existing pattern |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `multipart/form-data` | base64 JSON body | Multipart is standard browser/curl compatible; base64 bloats payload 33% |
| `http.MaxBytesReader` | manual length check | MaxBytesReader integrates with stdlib, applies early before any reads |

**Installation:**
No new packages required — all dependencies already present in go.mod.

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── api/
│   └── types.go              # Add RegistryInputsResponse, RegistryInputItem, UploadResponse
├── server/
│   ├── handlers_registry.go  # Add handleRegistryInputs
│   ├── handlers_uploads.go   # New: handleUpload
│   └── server.go             # Register 2 new routes in newRouter()
```

### Pattern 1: Registry Handler Extension (GET sub-resource)

**What:** Add `GET /projects/{slug}/registry/{name}/inputs?tag=<tag>` inside the existing `r.Route("/{name}", ...)` block.

**When to use:** Any new read endpoint on a named registry entry.

**Example (route registration in server.go):**
```go
// Source: internal/server/server.go lines 170-176 (existing pattern)
r.Route("/{name}", func(r chi.Router) {
    r.With(s.projectRoleRequired("viewer")).Get("/", s.handleRegistryPull)
    r.With(s.projectRoleRequired("viewer")).Get("/versions", s.handleRegistryListVersions)
    r.With(s.projectRoleRequired("admin")).Delete("/", s.handleRegistryDelete)
    r.With(s.projectRoleRequired("admin")).Post("/tags", s.handleRegistryTag)
    r.With(s.projectRoleRequired("admin")).Delete("/tags/{tag}", s.handleRegistryDeleteTag)
    // ADD:
    r.With(s.projectRoleRequired("viewer")).Get("/inputs", s.handleRegistryInputs)
})
```

**Example (handler skeleton):**
```go
// Source: pattern from internal/server/handlers_registry.go handleRegistryPull
func (s *Server) handleRegistryInputs(w http.ResponseWriter, r *http.Request) {
    proj := projectFromContext(r.Context())
    if proj == nil {
        writeError(w, http.StatusNotFound, "project not found")
        return
    }
    name := chi.URLParam(r, "name")
    tag := r.URL.Query().Get("tag")
    if tag == "" {
        writeError(w, http.StatusBadRequest, "must specify ?tag= query parameter")
        return
    }
    v, entry, err := s.store.GetRegistryVersionByTag(r.Context(), proj.ID, name, tag)
    if err != nil {
        writeError(w, http.StatusNotFound, fmt.Sprintf("tag %q not found for entry %q", tag, name))
        return
    }
    doc, err := parser.Parse(entry.Name, []byte(v.WDLContent))
    if err != nil {
        writeError(w, http.StatusInternalServerError, "WDL parse error")
        return
    }
    inputs := extractInputs(doc)
    writeJSON(w, http.StatusOK, api.RegistryInputsResponse{Inputs: inputs})
}
```

### Pattern 2: WDL Input Extraction

**What:** Walk `doc.Workflows[0].Inputs` or `doc.Tasks[0].Inputs` (both `[]*ast.Decl`), calling `TypeName()` on the type and inspecting `Expr` for a default value.

**When to use:** Any time the schema of a WDL registry entry needs to be surfaced.

**Example:**
```go
// Source: wdl/ast/ast.go, wdl/types/types.go
func extractInputs(doc *ast.Document) []api.RegistryInputItem {
    var decls []*ast.Decl
    if len(doc.Workflows) > 0 {
        decls = doc.Workflows[0].Inputs
    } else if len(doc.Tasks) > 0 {
        decls = doc.Tasks[0].Inputs
    }
    items := make([]api.RegistryInputItem, 0, len(decls))
    for _, d := range decls {
        items = append(items, api.RegistryInputItem{
            Name:     d.Name,
            Type:     d.Type.TypeName(),
            Optional: d.Type.IsOptional(),
            Default:  defaultExprString(d.Expr),
        })
    }
    return items
}
```

**Note on `defaultExprString`:** `ast.Decl.Expr` is an `ast.Expr` interface (nil when no default). A helper must type-switch over known literal types (`*ast.IntLit`, `*ast.FloatLit`, `*ast.BoolLit`, `*ast.StringExpr`, `*ast.NullLit`, etc.) and return a string representation or `""` for nil/complex expressions. The CONTEXT.md decision is `default` field is "string or null" — return `nil` when `d.Expr == nil`, and a stringified value otherwise. For non-literal expressions (e.g., complex interpolated strings, function calls), returning `nil` is acceptable.

### Pattern 3: Multipart File Upload Handler

**What:** Accept multipart/form-data, enforce 100 MB limit, stream directly to storage backend.

**When to use:** Any endpoint accepting file uploads.

**Example:**
```go
// Source: stdlib net/http; storage.Backend.UploadFileStream pattern from internal/storage/local.go
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
    proj := projectFromContext(r.Context())
    if proj == nil {
        writeError(w, http.StatusNotFound, "project not found")
        return
    }
    // Enforce 100 MB body limit BEFORE parsing multipart.
    r.Body = http.MaxBytesReader(w, r.Body, 100<<20)
    if err := r.ParseMultipartForm(32 << 20); err != nil {
        writeError(w, http.StatusBadRequest, "failed to parse multipart form (max 100 MB)")
        return
    }
    file, header, err := r.FormFile("file")
    if err != nil {
        writeError(w, http.StatusBadRequest, "missing 'file' field in multipart form")
        return
    }
    defer file.Close()
    uploadID := uuid.New().String()
    storagePath := filepath.Join(s.cfg.BaseDir, "projects", proj.Slug, "uploads", uploadID, header.Filename)
    if err := s.backend.UploadFileStream(storagePath, file); err != nil {
        writeError(w, http.StatusInternalServerError, "failed to store uploaded file")
        return
    }
    writeJSON(w, http.StatusOK, api.UploadResponse{Path: storagePath})
}
```

**Route registration:**
```go
// Inside r.Route("/{slug}", ...) block after provenance routes:
r.With(s.projectRoleRequired("executor")).Post("/uploads", s.handleUpload)
```

### Pattern 4: DTO Types in api/types.go

**What:** Add new response structs following existing naming convention.

**Example:**
```go
// Add to internal/api/types.go

// RegistryInputItem describes a single declared input from a WDL workflow or task.
type RegistryInputItem struct {
    Name     string  `json:"name"`
    Type     string  `json:"type"`
    Optional bool    `json:"optional"`
    Default  *string `json:"default"`
}

// RegistryInputsResponse is the response body for GET /projects/{slug}/registry/{name}/inputs.
type RegistryInputsResponse struct {
    Inputs []RegistryInputItem `json:"inputs"`
}

// UploadResponse is the response body for POST /projects/{slug}/uploads.
type UploadResponse struct {
    Path string `json:"path"`
}
```

### Anti-Patterns to Avoid

- **Calling `parser.Parse` and ignoring `entry.Kind`:** When `Kind == "workflow"` there is no `doc.Tasks` and vice versa. Always check `doc.Workflows` first, then `doc.Tasks`, and return empty inputs (not 500) if neither slice has entries (handles edge cases).
- **Parsing multipart without `MaxBytesReader`:** `ParseMultipartForm` will accept unlimited data if the body is not pre-limited. `MaxBytesReader` must be set BEFORE `ParseMultipartForm`.
- **Allocating the full file in memory:** Do not use `ioutil.ReadAll` on the uploaded file. Use `backend.UploadFileStream(path, file)` to stream directly.
- **Path traversal via filename:** `header.Filename` from multipart comes from the client. Use only `filepath.Base(header.Filename)` to strip any directory component before building the storage path.
- **Returning error for empty inputs:** A workflow with no declared inputs (`input {}` block present but empty, or no input block at all) MUST return `{"inputs":[]}` with HTTP 200, not 404 or 500.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Upload size limiting | Custom `io.LimitReader` wrapper | `http.MaxBytesReader` | Stdlib integrates with http.ResponseWriter error signaling |
| WDL type string serialization | Custom type-to-string map | `types.Type.TypeName()` | Already returns correct WDL notation (`"Array[File]+"`, etc.) including optional `?` suffix |
| UUID for upload directory | Custom random name generator | `github.com/google/uuid` (already imported) | Already used in project; prevents collisions |

**Key insight:** The WDL `types.Type` interface already does all the heavy lifting for type string representation. Do not build a parallel type-name mapping.

## Common Pitfalls

### Pitfall 1: `default` field — complex expressions silently dropped
**What goes wrong:** The handler tries to stringify every `ast.Expr` and panics or returns garbage for function calls, identifier references, or interpolated strings.
**Why it happens:** The full AST expression tree is rich; only literals have simple string representations.
**How to avoid:** Type-switch only over known literal types (`*ast.IntLit`, `*ast.FloatLit`, `*ast.BoolLit`, `*ast.StringExpr` with a single `RawStringPart`, `*ast.NullLit`). For anything else, return `nil` — the CONTEXT.md spec allows `null` for the default field.
**Warning signs:** Tests with a WDL containing `String name = read_string("...")` as default would fail if complex expressions are forced to string.

### Pitfall 2: `MapType.TypeName()` uses no space around comma
**What goes wrong:** Frontend expects `"Map[String, Int]"` (with space per CONTEXT.md decision) but `MapType.TypeName()` returns `"Map[String,Int]"` (no space per current implementation in `wdl/types/types.go` line 113).
**Why it happens:** The type's `TypeName()` implementation does not add spaces.
**How to avoid:** The CONTEXT.md decision says "WDL notation consistent with WDL spec." The WDL spec itself uses no space. Confirm with the planner whether the space is mandatory — if so, a post-processing pass on the type string is needed, but this would be non-trivial for nested types. **Recommendation:** Accept the no-space variant (`"Map[String,Int]"`) as spec-compliant and document it.
**Warning signs:** If a test asserts `"Map[String, Int]"` (with space), it will fail.

### Pitfall 3: S3 upload path format
**What goes wrong:** `filepath.Join` on an S3 path (`s3://bucket/key/...`) corrupts the URI because `filepath.Join` normalizes slashes and drops the double slash in `s3://`.
**Why it happens:** `filepath.Join` is POSIX path-aware, not URI-aware.
**How to avoid:** The CONTEXT.md decision uses `{base_dir}/projects/{slug}/uploads/{uuid}/{filename}`. On local backend `base_dir` is a filesystem path so `filepath.Join` is correct. On S3, `base_dir` is typically a key prefix, not a full S3 URI — the S3 backend itself builds the full URI internally. Use `filepath.Join` only for path construction; the backend interface handles URI translation internally.
**Warning signs:** S3 URLs returned by the endpoint contain `s3:/bucket/...` (single slash) if `filepath.Join` is applied to an S3 URI.

### Pitfall 4: Missing `inputs` key vs empty array
**What goes wrong:** Handler returns `{"inputs": null}` for a WDL with no inputs instead of `{"inputs": []}`.
**Why it happens:** `make([]T, 0)` vs nil slice serialize differently in Go's `encoding/json`.
**How to avoid:** Always initialize the slice with `make([]api.RegistryInputItem, 0)` or return an allocated empty slice, not `nil`. Existing handlers in the codebase already do this (see `handleRegistryList` line 175: `resp := make([]api.RegistryEntryResponse, 0, len(items))`).

### Pitfall 5: Multipart memory vs disk tradeoff
**What goes wrong:** `ParseMultipartForm(maxMemory)` puts the first `maxMemory` bytes in RAM; the rest spills to a temp file. With 100 MB allowed uploads, using 100 MB as `maxMemory` would hold the whole file in RAM.
**Why it happens:** The `maxMemory` argument to `ParseMultipartForm` is often confused with the max upload size.
**How to avoid:** Use a small `maxMemory` (e.g., `32 << 20` = 32 MB) with `MaxBytesReader` set to 100 MB. Large files spill to temp disk then get streamed to storage — acceptable.

## Code Examples

### Full `extractInputs` helper with default stringification
```go
// Source: wdl/ast/expr.go, wdl/types/types.go — verified field names
func extractInputs(doc *ast.Document) []api.RegistryInputItem {
    var decls []*ast.Decl
    if len(doc.Workflows) > 0 {
        decls = doc.Workflows[0].Inputs
    } else if len(doc.Tasks) > 0 {
        decls = doc.Tasks[0].Inputs
    }
    items := make([]api.RegistryInputItem, 0, len(decls))
    for _, d := range decls {
        items = append(items, api.RegistryInputItem{
            Name:     d.Name,
            Type:     d.Type.TypeName(),
            Optional: d.Type.IsOptional(),
            Default:  declDefaultString(d.Expr),
        })
    }
    return items
}

// declDefaultString converts a WDL literal expression to its string representation.
// Returns nil for nil Expr or any non-literal expression (function calls, identifiers, etc.).
func declDefaultString(e ast.Expr) *string {
    if e == nil {
        return nil
    }
    var s string
    switch v := e.(type) {
    case *ast.IntLit:
        s = strconv.FormatInt(v.Value, 10)
    case *ast.FloatLit:
        s = strconv.FormatFloat(v.Value, 'f', -1, 64)
    case *ast.BoolLit:
        if v.Value {
            s = "true"
        } else {
            s = "false"
        }
    case *ast.NullLit:
        s = "null"
    case *ast.StringExpr:
        if len(v.Parts) == 1 {
            if raw, ok := v.Parts[0].(*ast.RawStringPart); ok {
                s = raw.Val
            } else {
                return nil // interpolated string — too complex
            }
        } else if len(v.Parts) == 0 {
            s = ""
        } else {
            return nil // multi-part string
        }
    default:
        return nil
    }
    return &s
}
```

### Upload handler (complete)
```go
// Source: stdlib net/http docs + internal/storage/backend.go UploadFileStream
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
    proj := projectFromContext(r.Context())
    if proj == nil {
        writeError(w, http.StatusNotFound, "project not found")
        return
    }
    r.Body = http.MaxBytesReader(w, r.Body, 100<<20) // 100 MB hard limit
    if err := r.ParseMultipartForm(32 << 20); err != nil {
        writeError(w, http.StatusBadRequest, "failed to parse multipart form (max 100 MB)")
        return
    }
    file, header, err := r.FormFile("file")
    if err != nil {
        writeError(w, http.StatusBadRequest, "missing 'file' field in multipart form")
        return
    }
    defer file.Close()

    filename := filepath.Base(header.Filename) // strip any directory traversal
    uploadID := uuid.New().String()
    storagePath := filepath.Join(s.cfg.BaseDir, "projects", proj.Slug, "uploads", uploadID, filename)

    if err := s.backend.UploadFileStream(storagePath, file); err != nil {
        writeError(w, http.StatusInternalServerError, "failed to store uploaded file")
        return
    }
    writeJSON(w, http.StatusCreated, api.UploadResponse{Path: storagePath})
}
```

**Note:** HTTP 201 Created is appropriate for the upload response since a new resource (the stored file) was created.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Shared filesystem paths for WDL inputs | Server-accessible path strings via upload endpoint (ADR 0001) | Phase 32 | File inputs are always paths — browser uploads via this endpoint, then uses the returned path as input value |

## Open Questions

1. **`Map[String,Int]` vs `Map[String, Int]` (space after comma)**
   - What we know: `MapType.TypeName()` returns `"Map[String,Int]"` without space (verified in wdl/types/types.go line 113)
   - What's unclear: CONTEXT.md example uses `"Map[String, Int]"` (with space) — this may just be prose style, not a hard requirement
   - Recommendation: Use the WDL spec format (no space) as returned by `TypeName()`. Document the decision in the plan. If the frontend specifically needs spaces, a post-processing step can be added.

2. **S3 upload path with `filepath.Join`**
   - What we know: Local backend uses OS filesystem paths; S3 backend has its own URI construction
   - What's unclear: Does `s.cfg.BaseDir` ever hold an `s3://` prefix when the S3 backend is active?
   - Recommendation: Review how `handleFileDownload` builds paths. If `cfg.BaseDir` is always a local path prefix (not an S3 URI), `filepath.Join` is safe. The S3 backend's `UploadFileStream` accepts a local-style key path and maps it to S3 internally.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) + `net/http/httptest` |
| Config file | none (Go test tool, no config file required) |
| Quick run command | `cd /home/uehara/flowy && go test ./internal/server/ -run TestRegistryInputs -v` |
| Full suite command | `cd /home/uehara/flowy && go test ./internal/server/... -count=1` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| API-01 | `GET /projects/{slug}/registry/{name}/inputs?tag=<tag>` returns structured input list | unit (httptest) | `go test ./internal/server/ -run TestRegistryInputs -v` | ❌ Wave 0 |
| API-01 | Empty inputs array for workflow with no inputs | unit (httptest) | `go test ./internal/server/ -run TestRegistryInputs_Empty -v` | ❌ Wave 0 |
| API-01 | Missing tag returns 404 `{"error":"tag not found"}` | unit (httptest) | `go test ./internal/server/ -run TestRegistryInputs_TagNotFound -v` | ❌ Wave 0 |
| API-01 | Viewer role can access; unauthenticated gets 401 | unit (httptest) | `go test ./internal/server/ -run TestRegistryInputs_Auth -v` | ❌ Wave 0 |
| API-02 | `POST /projects/{slug}/uploads` accepts multipart, returns path | unit (httptest) | `go test ./internal/server/ -run TestUpload -v` | ❌ Wave 0 |
| API-02 | Upload exceeding 100 MB is rejected | unit (httptest) | `go test ./internal/server/ -run TestUpload_TooLarge -v` | ❌ Wave 0 |
| API-02 | Executor role required; viewer gets 403 | unit (httptest) | `go test ./internal/server/ -run TestUpload_Auth -v` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `cd /home/uehara/flowy && go test ./internal/server/ -run TestRegistryInputs -run TestUpload -v`
- **Per wave merge:** `cd /home/uehara/flowy && go test ./internal/server/... -count=1`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/server/handlers_registry_inputs_test.go` — covers API-01 (or add to `handlers_registry_test.go`)
- [ ] `internal/server/handlers_uploads_test.go` — covers API-02

*(Framework install: none — Go stdlib testing is already used throughout the project)*

## Sources

### Primary (HIGH confidence)
- `internal/server/handlers_registry.go` — verified handler structure, `GetRegistryVersionByTag` usage, `parser.Parse` call, `chi.URLParam` pattern
- `internal/server/server.go` — verified route registration pattern, `r.Route("/{name}", ...)`, `projectRoleRequired` usage
- `internal/server/middleware.go` — verified `projectContext`, `projectFromContext`, `projectRoleRequired`
- `internal/api/types.go` — verified existing DTO pattern; confirmed `RegistryInputsResponse`, `UploadResponse` do not yet exist
- `wdl/ast/ast.go` — verified `ast.Decl` fields: `Type types.Type`, `Name string`, `Expr ast.Expr`
- `wdl/types/types.go` — verified `TypeName()` returns WDL notation (e.g., `"Array[File]"`, `"Map[String,Int]"`), `IsOptional()` exists on `Type` interface
- `wdl/ast/expr.go` — verified literal expression types: `*ast.IntLit`, `*ast.FloatLit`, `*ast.BoolLit`, `*ast.StringExpr`, `*ast.NullLit`, `*ast.RawStringPart`
- `internal/storage/backend.go` — verified `UploadFileStream(path string, r io.Reader) error` signature
- `internal/storage/factory.go` — verified `NewBackend(cfg)` returns `Backend` (local or S3)
- Go stdlib `net/http` — `http.MaxBytesReader`, `r.ParseMultipartForm`, `r.FormFile` are standard API

### Secondary (MEDIUM confidence)
- `internal/server/handlers_auth_test.go` — verified test helper signatures (`newAuthTestServer`, `doAuthRequest`, `issueTestToken`) to inform test plan

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries already in use; no new dependencies
- Architecture: HIGH — patterns directly verified from existing handler code
- Pitfalls: HIGH — most identified from direct inspection of WDL types implementation and Go stdlib behavior
- Test patterns: HIGH — existing test helpers fully verified

**Research date:** 2026-04-07
**Valid until:** 2026-05-07 (stable internal codebase; no upstream dependency churn expected)
