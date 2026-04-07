# Phase 32: API Foundation - Context

**Gathered:** 2026-04-07
**Status:** Ready for planning

<domain>
## Phase Boundary

flowy-server exposes two new API endpoints: (1) input schema introspection for registry entries, and (2) file upload for File-typed workflow inputs. These endpoints unblock all frontend execution work in subsequent phases.

</domain>

<decisions>
## Implementation Decisions

### Input Schema Endpoint
- Response includes `name` (string), `type` (string in WDL notation), `optional` (bool), and `default` (string or null) per input
- WDL types represented as human-readable WDL notation: `"String"`, `"Int"`, `"Array[File]"`, `"Map[String, Int]"` — consistent with WDL spec
- Support both workflow and task registry entries — both use `ast.Decl`, same response shape
- Missing tag returns 404 with `{"error": "tag not found"}`, consistent with existing registry pull behavior

### File Upload Endpoint
- Route: `POST /projects/{slug}/uploads` — project-scoped, outside registry namespace
- Body format: multipart form-data with single file field — standard HTTP upload, works with browser and CLI
- Storage: project-scoped directory under configured storage backend (`{base_dir}/projects/{slug}/uploads/{uuid}/{filename}`) — reuses existing storage layer, S3-compatible
- Response: `{"path": "/absolute/or/s3://path"}` — the server-accessible path for File input fields per ADR 0001

### Auth & Constraints
- Input schema endpoint: `viewer` minimum role — reading schema is a read operation
- Upload endpoint: `executor` minimum role — uploading files is prerequisite to execution
- Upload size limit: 100 MB via `http.MaxBytesReader` — prevents abuse, sufficient for WDL input files
- No file type restriction — WDL File inputs can be any type (BAM, VCF, FASTQ, etc.)

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- Chi v5 router with nested groups and middleware chaining (`internal/server/server.go`)
- `projectRoleRequired(minRole)` middleware for per-project RBAC (`internal/server/middleware.go`)
- `JWTMiddleware(secret)` for Bearer token validation
- `writeJSON(w, status, dto)` helper for JSON responses (`internal/api/types.go`)
- WDL parser: `parser.Parse(filename, []byte)` returns `*ast.Document` with `Workflows[0].Inputs []*ast.Decl`
- `ast.Decl` has `Type types.Type`, `Name string`, `Expr Expr` (default expression)
- Storage backend interface with S3 support (`internal/storage/`)
- Existing `handleFileDownload()` for streaming files from storage

### Established Patterns
- Route registration: `r.Route("/path", func(r chi.Router) { r.With(middleware).Get/Post(...) })`
- Handler signature: `func (s *Server) handleXxx(w http.ResponseWriter, r *http.Request)`
- Request body: `json.NewDecoder(r.Body).Decode(&req)`
- URL params: `chi.URLParam(r, "slug")`
- Registry handlers in `internal/server/handlers_registry.go`

### Integration Points
- New routes wire into `newRouter()` in `server.go` within the JWT-protected, project-scoped group
- Input schema endpoint reuses existing registry entry lookup (get by name + tag) then parses WDL source
- Upload endpoint writes to storage backend via existing `Backend` interface

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches following existing codebase patterns.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>
