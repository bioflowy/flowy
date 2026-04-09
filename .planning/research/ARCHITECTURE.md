# Architecture Research: Managed File Library (v2.2)

**Domain:** Managed file library integrated into a Go REST API + React SPA workflow engine
**Researched:** 2026-04-09
**Confidence:** HIGH (based on direct codebase analysis)

---

## System Overview

```
┌──────────────────────────────────────────────────────────────────────┐
│  flowy-ui (embedded SPA host + reverse proxy)                        │
│  ┌───────────────────────────────────────────────────────────────┐   │
│  │  React SPA (React 19, TypeScript, TanStack Query, Zustand)    │   │
│  │  ┌────────────────┐  ┌──────────────────────┐                 │   │
│  │  │ ProjectDetail  │  │  RunWorkflowDialog   │                 │   │
│  │  │ Page           │  │  (MODIFIED)          │                 │   │
│  │  │ + Files tab    │  │  File input →        │                 │   │
│  │  │   (NEW)        │  │  FilePicker modal    │                 │   │
│  │  └────────────────┘  │  (NEW)               │                 │   │
│  │                      └──────────────────────┘                 │   │
│  └───────────────────────────────────────────────────────────────┘   │
└─────────────────────────┬────────────────────────────────────────────┘
                          │ reverse proxy /api/*
┌─────────────────────────▼────────────────────────────────────────────┐
│  flowy-server (chi v5 REST API)                                       │
│                                                                       │
│  NEW route group: /projects/{slug}/files                             │
│    GET    /files             → handleListFiles                        │
│    POST   /files             → handleCreateFile (browser upload path) │
│    DELETE /files/{id}        → handleDeleteFile                       │
│    POST   /files/import      → handleImportFile (S3/local path)       │
│                                                                       │
│  MODIFIED: /projects/{slug}/registry/{name}/inputs                   │
│    extractInputs() now reads ParamMeta for file_type + label_template │
│                                                                       │
│  MODIFIED: dispatch.go runWorkflow()                                  │
│    After UpdateRunOutputs → registerRunOutputFiles()                  │
│    calls LabelEngine.Resolve() + store.CreateManagedFile()            │
│                                                                       │
│  EXISTING unchanged: /projects/{slug}/uploads                        │
└───────────────────────────────────────────────────────────────────────┤
                          │
         ┌────────────────▼──────────────────────┐
         │  store.Store interface (EXTENDED)      │
         │  CreateManagedFile, ListManagedFiles   │
         │  GetManagedFile, DeleteManagedFile     │
         └────────────────┬──────────────────────┘
                          │
┌─────────────────────────▼──────────────────────────────────────────┐
│  SQLite (WAL, modernc.org/sqlite)                                   │
│  ┌──────────────┐  ┌─────────────────┐  ┌──────────────────────┐  │
│  │  runs        │  │ registry_entries│  │  managed_files (NEW) │  │
│  │  tasks       │  │ registry_vers.  │  │  id, project_id,     │  │
│  │  provenance  │  │ registry_tags   │  │  path, backend, type,│  │
│  │  users       │  │                 │  │  label, size, source,│  │
│  │  projects    │  │                 │  │  run_id, created_at  │  │
│  └──────────────┘  └─────────────────┘  └──────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
         │
┌────────▼──────────────────────────────────────┐
│  storage.Backend (EXTENDED)                   │
│  Existing 13 methods + Stat(path) NEW         │
│  LocalBackend: os.Stat                        │
│  S3Backend: s3.HeadObject                     │
└───────────────────────────────────────────────┘
```

---

## Component Responsibilities

| Component | Responsibility | New / Modified |
|-----------|----------------|----------------|
| `managed_files` DB table | Project-scoped file catalog (path, type, label, size, source, run_id) | NEW |
| `store.ManagedFile` model | Store-layer model; `Store` interface extended with CRUD methods | NEW |
| `sqlite/managed_files.go` | SQL implementations of CRUD for managed_files | NEW |
| `handlers_files.go` | REST handlers for `/projects/{slug}/files` routes | NEW |
| `label.go` + `LabelEngine` | Shell-style `${var%%pattern}` template expansion | NEW |
| `dispatch.go` `runWorkflow()` | Calls `registerRunOutputFiles()` in success branch | MODIFIED |
| `extractInputs()` | Reads `ParamMeta` to populate `FileType` / `LabelTemplate` on RegistryInputItem | MODIFIED |
| `api.RegistryInputItem` | Add `FileType string` and `LabelTemplate string` fields | MODIFIED |
| `api.ManagedFileResponse` etc. | New DTOs for file library endpoints | NEW |
| `storage.Backend` interface | Add `Stat(path) (int64, time.Time, error)` as method 14 | MODIFIED |
| `LocalBackend.Stat()` | `os.Stat()` wrapper | NEW method |
| `S3Backend.Stat()` | `s3.HeadObject()` wrapper | NEW method |
| `server.go` `newRouter()` | Register files route group | MODIFIED |
| `FilesTab.tsx` | Project files table with import and upload actions | NEW |
| `ImportFileDialog.tsx` | Form for S3/local path import | NEW |
| `FilePicker.tsx` | Modal for selecting a managed file as a workflow input | NEW |
| `useFiles.ts` | TanStack Query hooks for files CRUD and import | NEW |
| `ProjectDetailPage.tsx` | Add "Files" tab | MODIFIED |
| `RunWorkflowDialog.tsx` | Use `FilePicker` for File-typed inputs that have `file_type` hint | MODIFIED |

---

## Recommended Project Structure

```
internal/server/
├── handlers.go                  (existing, unchanged)
├── handlers_files.go            (NEW — files CRUD + import)
├── handlers_registry.go         (MODIFIED — extractInputs reads ParamMeta)
├── handlers_uploads.go          (existing, unchanged — reused by browser upload flow)
├── dispatch.go                  (MODIFIED — runWorkflow calls registerRunOutputFiles)
├── label.go                     (NEW — LabelEngine: ${var%%pattern} expansion)
├── label_test.go                (NEW)
└── server.go                    (MODIFIED — add files route group)

internal/store/
└── sqlite/
    ├── migrations/
    │   └── 00009_managed_files.sql   (NEW)
    └── managed_files.go              (NEW — CRUD implementations)
    └── managed_files_test.go         (NEW)

internal/api/
└── types.go                     (MODIFIED — add ManagedFileResponse, FileImportRequest,
                                              extend RegistryInputItem)

internal/storage/
├── backend.go                   (MODIFIED — add Stat() to interface)
├── local.go                     (MODIFIED — implement Stat())
└── s3.go                        (MODIFIED — implement Stat())

web/src/features/
├── files/                       (NEW directory)
│   ├── FilesTab.tsx             (NEW)
│   ├── ImportFileDialog.tsx     (NEW)
│   ├── FilePicker.tsx           (NEW)
│   └── useFiles.ts              (NEW)
├── projects/
│   └── ProjectDetailPage.tsx    (MODIFIED — add Files tab)
└── registry/
    └── RunWorkflowDialog.tsx    (MODIFIED — FilePicker for File-typed inputs)
```

---

## Architectural Patterns

### Pattern 1: Run-Completion Hook in `runWorkflow()`

**What:** After `UpdateRunOutputs` and `UpdateRunState(COMPLETED)` succeed in `dispatch.go`, call `registerRunOutputFiles()` to iterate the outputs map, identify File/Directory values, stat each path, resolve its label via `LabelEngine`, and call `store.CreateManagedFile()`.

**Why server, not daemon:** The daemon is project-unaware — it only knows `run_id` and `task_id`. The server already aggregates all outputs into `RunOutputsJSON` in `runWorkflow()`. The daemon returns raw output paths via `POST /tasks/{id}/result`; the server is the single place that knows both the project context and the final aggregated outputs.

**Non-fatal error pattern:** Registration failures must never fail the run. Follow the same pattern as `TouchProvenanceUpdatedAt`:

```go
// dispatch.go runWorkflow(), success branch (after UpdateRunState COMPLETED)
if touchErr := s.store.TouchProvenanceUpdatedAt(ctx, runID); touchErr != nil {
    log.Printf("runWorkflow %s: TouchProvenanceUpdatedAt: %v", runID, touchErr)
}
// Non-fatal: register workflow outputs as managed files.
s.registerRunOutputFiles(ctx, proj.ID, runID, doc, inputs, outputs)
```

`registerRunOutputFiles` receives the `doc *ast.Document` (already passed to `runWorkflow`), the decoded `inputs map[string]interface{}` from `runs.inputs_json`, and the aggregated `outputs map[string]values.Value`. It iterates outputs, identifies `values.FileValue` entries, stats each path, resolves the label, and calls `store.CreateManagedFile`.

**`doc` availability:** `runWorkflow()` already receives `doc *ast.Document` as a parameter. `doc.Workflows[0].ParamMeta` gives the label template for each output name.

### Pattern 2: Parameter Meta at Query Time (not stored separately)

**What:** Extract `parameter_meta` type and label hints from the WDL AST in `handleRegistryInputs()`, alongside the existing input schema. No new storage column.

**Rationale:**
- `registry_versions.wdl_content` is already stored. `handleRegistryInputs` already calls `parser.Parse()` on each request. `doc.Workflows[0].ParamMeta` (type `map[string]any`) is directly available on the parsed document.
- Adding a separate storage column requires a migration AND a backfill strategy for existing registry versions.
- WDL files are small (<100KB); parsing is fast (<1ms). Query-time extraction adds no meaningful latency.
- Extending `RegistryInputItem` with two optional string fields is backward-compatible — clients that do not understand them simply ignore them.

**Extended `RegistryInputItem`:**
```go
type RegistryInputItem struct {
    Name          string  `json:"name"`
    Type          string  `json:"type"`
    Optional      bool    `json:"optional"`
    Default       *string `json:"default"`
    FileType      string  `json:"file_type,omitempty"`      // from parameter_meta: {type: "fastq"}
    LabelTemplate string  `json:"label_template,omitempty"` // from parameter_meta: {label: "${reads%%.fastq}.bam"}
}
```

**`extractInputs` change:** For each `Decl`, look up `paramMeta[decl.Name]`. If it is a `map[string]any`, extract `"type"` and `"label"` string fields.

### Pattern 3: Shell-Style Label Engine (`internal/server/label.go`)

**What:** A pure Go function that expands `${var%%pattern}` (and `${var%pattern}`) templates using values from the workflow inputs map.

**Rules:**
- `%%pattern` strips the longest matching suffix (greedy, like bash).
- `%pattern` strips the shortest matching suffix.
- Glob-style `*` in pattern matches any sequence of characters.
- If `var` resolves to a file path, look up the path in `managed_files` for its label; fall back to `filepath.Base(path)`.
- If a variable is missing or template is malformed, fall back to `filepath.Base(outputPath)`.

**Scope:** Used only by `registerRunOutputFiles()` in `dispatch.go`. No dependency on daemon or AST package at call time (the template strings are plain strings extracted from `ParamMeta` and passed in).

**Example resolution:**
```
template:  "${reads%%.fastq*}.trimmed.bam"
inputs:    {"reads": "/data/projects/proj/uploads/abc/sample.fastq.gz"}
file lookup for "/data/.../sample.fastq.gz" → label "sample.fastq.gz" (or filepath.Base)
%%".fastq*": strip longest suffix matching ".fastq*" from "sample.fastq.gz"
  ".fastq.gz" matches ".fastq*" → strip it → "sample"
result:    "sample.trimmed.bam"
```

### Pattern 4: Two-Step Browser Upload → Managed File

**What:** Browser upload uses the existing `POST /projects/{slug}/uploads` endpoint unchanged. A separate `POST /projects/{slug}/files/import` call then registers the returned path as a managed file.

**Why two steps:** The upload endpoint is storage-agnostic and handles multipart streaming. Mixing file content upload with metadata (type, label) into one endpoint would require a different handler design. Two-step separation keeps each endpoint focused. It also means the import endpoint can be reused for S3/local paths that are already in storage.

**Flow:**
```
POST /projects/{slug}/uploads   (multipart form, existing)
← {path: "/data/.../uploads/{uuid}/sample.fastq.gz"}

POST /projects/{slug}/files/import
  body: {path: "/data/.../sample.fastq.gz", type: "fastq", label: "sample"}
← ManagedFileResponse {id, path, label, type, size, source: "upload", created_at}
```

---

## Data Flow

### Flow 1: Browser Upload → Managed File

```
User drags file into FilesTab
    → POST /projects/{slug}/uploads  (existing, unchanged)
    ← {path: "..."}
    
User enters type/label in ImportFileDialog
    → POST /projects/{slug}/files/import
        body: {path, type, label, source: "upload"}
    Server:
        → backend.Stat(path) → size
        → store.CreateManagedFile(...)
    ← ManagedFileResponse
    
TanStack Query invalidates ['projects', slug, 'files']
    → FilesTab re-renders with new row
```

### Flow 2: S3/Local Path Import

```
User enters "s3://bucket/inputs/sample.fastq.gz" in ImportFileDialog
    → POST /projects/{slug}/files/import
        body: {path: "s3://...", type: "fastq", label: "sample", source: "import_s3"}
    Server:
        → storage.ParseS3URI(path) — validate scheme
        → backend.Stat(path) → HeadObject → size
        → store.CreateManagedFile(..., source: "import_s3")
    ← ManagedFileResponse
```

### Flow 3: Workflow Completion → Auto-Registered Outputs

```
runWorkflow() goroutine [dispatch.go]
    → RunWorkflowWithDispatch(ctx, doc, ...)  → outputs map[string]values.Value
    → store.UpdateRunOutputs(runID, outputsJSON)
    → store.UpdateRunState(runID, COMPLETED)
    → store.TouchProvenanceUpdatedAt(runID)
    → s.registerRunOutputFiles(ctx, proj.ID, runID, doc, inputs, outputs)
        for each output key, value:
            if value is FileValue or StringValue with s3:// prefix:
                path = value.Value
                template = doc.Workflows[0].ParamMeta[key]["label"] (may be empty)
                label = LabelEngine.Resolve(template, inputs) or filepath.Base(path)
                fileType = doc.Workflows[0].ParamMeta[key]["type"] (may be empty)
                size, _ = backend.Stat(path)
                store.CreateManagedFile(ManagedFile{
                    ProjectID: proj.ID,
                    Path:      path,
                    Backend:   cfg.Storage.Type,
                    Type:      fileType,
                    Label:     label,
                    Size:      size,
                    Source:    "workflow_output",
                    RunID:     &runID,
                })  // INSERT OR IGNORE — idempotent
```

### Flow 4: FilePicker in RunWorkflowDialog

```
User opens RunWorkflowDialog
    → GET /projects/{slug}/registry/{name}/inputs?tag=latest
    ← [{name:"reads", type:"File", file_type:"fastq", label_template:"${reads%%.fastq*}.bam"}, ...]

For input "reads" where type=="File":
    Render: [text field showing selected path] + [Pick File button]
    
User clicks "Pick File"
    FilePicker modal opens
    → GET /projects/{slug}/files?type=fastq    (filter by file_type hint)
    ← [{id, path, label, type, size, created_at}, ...]
    
User selects a managed file
    → form field "reads" = selected_file.path
    
User submits
    → POST /projects/{slug}/workflows
        body: {registry_ref: "name:latest", inputs: {reads: "/path/to/sample.fastq.gz"}}
```

---

## Managed Files Table Schema

```sql
-- 00009_managed_files.sql
-- +goose Up
CREATE TABLE IF NOT EXISTS managed_files (
    id         TEXT     PRIMARY KEY,
    project_id TEXT     NOT NULL REFERENCES projects(id),
    path       TEXT     NOT NULL,
    backend    TEXT     NOT NULL DEFAULT 'local',
    type       TEXT     NOT NULL DEFAULT '',
    label      TEXT     NOT NULL DEFAULT '',
    size       INTEGER  NOT NULL DEFAULT 0,
    source     TEXT     NOT NULL DEFAULT 'upload',
    run_id     TEXT     REFERENCES runs(id),
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE (project_id, path)
);
CREATE INDEX IF NOT EXISTS idx_managed_files_project ON managed_files(project_id);
CREATE INDEX IF NOT EXISTS idx_managed_files_type    ON managed_files(project_id, type);
CREATE INDEX IF NOT EXISTS idx_managed_files_run     ON managed_files(run_id);

-- +goose Down
DROP TABLE IF EXISTS managed_files;
```

**Schema decisions:**
- `UNIQUE (project_id, path)` prevents duplicate registrations. `registerRunOutputFiles` uses `INSERT OR IGNORE` for idempotency.
- No `updated_at` — managed files are immutable records; deletion is hard-delete.
- `run_id` nullable TEXT (SQL NULL; Go `sql.NullString`). Set only for `source = "workflow_output"`.
- `backend` stored per-file for future multi-backend correctness.
- `source` enum: `"upload"` | `"import_s3"` | `"import_local"` | `"workflow_output"`.

---

## Integration Points: New vs. Modified

### New (no existing code touched)

| Artifact | Type | Notes |
|----------|------|-------|
| `00009_managed_files.sql` | Migration | goose Up/Down |
| `sqlite/managed_files.go` | Store impl | CreateManagedFile, ListManagedFiles, GetManagedFile, DeleteManagedFile |
| `handlers_files.go` | HTTP handlers | GET/POST/DELETE /files, POST /files/import |
| `label.go` + `label_test.go` | Engine | `${var%%pattern}` expansion; pure function; no external deps |
| `api.ManagedFileResponse` | DTO | id, project_id, path, backend, type, label, size, source, run_id, created_at |
| `api.FileImportRequest` | DTO | path, type, label |
| `api.FileListResponse` | DTO | `[]ManagedFileResponse` wrapper or direct slice |
| `web/src/features/files/FilesTab.tsx` | React | Files table, import button, upload shortcut |
| `web/src/features/files/ImportFileDialog.tsx` | React | Path import form |
| `web/src/features/files/FilePicker.tsx` | React | Modal file selector with type filter |
| `web/src/features/files/useFiles.ts` | React | TanStack Query hooks |

### Modified (targeted changes to existing files)

| Artifact | Change | Risk |
|----------|--------|------|
| `internal/api/types.go` | Add new DTOs; add `FileType`/`LabelTemplate` to `RegistryInputItem` | LOW — additive |
| `internal/server/handlers_registry.go` `extractInputs()` | Read `doc.*.ParamMeta` per input for type/label | LOW — additive return fields |
| `internal/server/dispatch.go` `runWorkflow()` | Call `registerRunOutputFiles()` in COMPLETED success branch | LOW — non-fatal; same pattern as `TouchProvenanceUpdatedAt` |
| `internal/server/server.go` `newRouter()` | Add `/projects/{slug}/files` route group | LOW — new routes only |
| `internal/storage/backend.go` | Add `Stat(path string) (int64, time.Time, error)` to `Backend` interface | MEDIUM — interface change requires both implementations updated simultaneously |
| `internal/storage/local.go` | `Stat()` via `os.Stat()` | LOW |
| `internal/storage/s3.go` | `Stat()` via `s3.HeadObject()` | LOW |
| `web/src/features/projects/ProjectDetailPage.tsx` | Add "Files" tab with `FilesTab` | LOW — additive |
| `web/src/features/registry/RunWorkflowDialog.tsx` | Detect `file_type` hint; use `FilePicker` for File inputs | MEDIUM — form logic change; `schema` and `defaultValues` construction affected |

---

## Build Order

Dependencies drive a strict phase ordering:

**Phase 1: Backend foundation (no UI)**
- `00009_managed_files.sql` migration
- `store.ManagedFile` model + `Store` interface extensions (4 methods)
- `sqlite/managed_files.go` CRUD
- `storage.Backend.Stat()` interface method + `LocalBackend` + `S3Backend` implementations
- New DTOs in `api/types.go`

Rationale: everything downstream depends on the store and backend interface. The `Backend` interface change must be complete before any handler or dispatch code can compile.

**Phase 2: REST API for files**
- `handlers_files.go` (all 4 handlers)
- Route registration in `server.go`
- Integration tests

Rationale: depends on Phase 1 store. Frontend depends on Phase 2 API.

**Phase 3: Workflow completion hook + label engine + parameter_meta extension**
- `label.go` + `label_test.go` (pure logic, no external deps — can be developed first)
- `registerRunOutputFiles()` in `dispatch.go`
- `extractInputs()` extended in `handlers_registry.go`
- `RegistryInputItem.FileType` / `RegistryInputItem.LabelTemplate` in `api/types.go`

Rationale: label engine has no external dependencies and can be written and tested in isolation. The dispatch hook requires Phase 1 store. The `/inputs` endpoint extension only requires the AST (already available) and the DTO change.

**Phase 4: UI**
- `useFiles.ts` + `FilesTab.tsx` + `ImportFileDialog.tsx`
- `ProjectDetailPage.tsx` tab addition
- `FilePicker.tsx`
- `RunWorkflowDialog.tsx` FilePicker integration

Rationale: all UI depends on Phases 1-3 being complete and deployed. `FilePicker` integration into `RunWorkflowDialog` is the highest-risk UI change and should be the last item within Phase 4.

---

## Anti-Patterns

### Anti-Pattern 1: Registering Files in the Daemon

**What people do:** Add a new daemon-to-server API call for output file metadata so the daemon registers files directly.

**Why it's wrong:** The daemon is project-unaware (only knows run_id/task_id). The server already has all outputs aggregated in `runWorkflow()`. Adding daemon-side registration creates a race: daemon may report files before the server has set run state to COMPLETED, and requires the daemon to understand the file library concept.

**Do this instead:** Hook into `runWorkflow()` in `dispatch.go` immediately after `UpdateRunOutputs`, using the server's already-aggregated output map.

### Anti-Pattern 2: Storing Parameter Meta as a DB Column

**What people do:** Add `inputs_schema` JSON column to `registry_versions`, populated at push time.

**Why it's wrong:** `wdl_content` is already stored verbatim. Re-parsing on the `/inputs` endpoint is fast (<1ms). A storage column needs a migration plus backfill for existing versions. It creates a consistency risk if stored schema diverges from actual WDL content.

**Do this instead:** Extract from `doc.*.ParamMeta` at query time in `handleRegistryInputs()`. No migration, no consistency risk.

### Anti-Pattern 3: Combining Upload and Import in One Endpoint

**What people do:** Support both multipart file upload and path-by-reference in `POST /files/import`.

**Why it's wrong:** The existing `POST /uploads` already handles multipart with path-traversal protection and size limits. Duplicating that logic in a new handler creates two paths to maintain. The two-step design (upload → import) is cleaner.

**Do this instead:** Keep `POST /uploads` for file content transfer. `POST /files/import` only accepts a path string, calls `backend.Stat()`, and calls `store.CreateManagedFile()`.

### Anti-Pattern 4: Failing the Run on File Registration Failure

**What people do:** Return an error from `registerRunOutputFiles()` and set run state to FAILED.

**Why it's wrong:** The workflow itself succeeded; outputs are already in `runs.outputs_json`. Failing the run for bookkeeping failure is misleading and prevents users from seeing their results.

**Do this instead:** Log registration failures at `log.Printf` level and continue, matching the existing `TouchProvenanceUpdatedAt` pattern.

---

## Scaling Considerations

| Scale | Notes |
|-------|-------|
| Single lab (1-50 users) | SQLite WAL handles concurrent reads; no pagination needed initially |
| Multi-lab (50-500 users) | Add `LIMIT`/`OFFSET` or cursor pagination to `ListManagedFiles` if table grows large; the composite index `(project_id, type)` handles filtered queries efficiently |
| High run volume | `INSERT OR IGNORE` on `UNIQUE (project_id, path)` means re-registration is safe and cheap; no pre-check SELECT needed |

---

## Sources

- Direct codebase analysis: `internal/server/dispatch.go`, `internal/server/handlers_registry.go`, `internal/storage/backend.go`, `internal/store/sqlite/migrations/`, `wdl/ast/ast.go` — HIGH confidence
- Existing pattern: `TouchProvenanceUpdatedAt` non-fatal error in `dispatch.go` — HIGH confidence
- Existing pattern: `extractInputs()` AST-to-DTO without persistence in `handlers_registry.go` — HIGH confidence
- Existing pattern: `INSERT OR IGNORE` idiom for idempotent inserts — standard SQLite — HIGH confidence
- Shell parameter expansion (`%%`, `%`) semantics: POSIX shell spec — MEDIUM confidence (mapping to Go implementation is new)

---

*Architecture research for: Managed File Library (flowy v2.2)*
*Researched: 2026-04-09*
