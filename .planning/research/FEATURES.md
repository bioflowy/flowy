# Feature Research

**Domain:** Managed file library for WDL 1.2 workflow engine UI (flowy v2.2)
**Researched:** 2026-04-09
**Confidence:** HIGH (existing codebase) / MEDIUM (ecosystem patterns from Terra, Galaxy, Seven Bridges, DNAnexus, Seqera)

---

## Context: What Already Exists

This milestone adds a managed file library to an existing system. Understanding what is already
built prevents re-implementing solved problems.

**Existing backend capabilities (ready to call or extend):**
- `POST /projects/{slug}/uploads` — file upload endpoint (v2.1); stores file, returns server-side path; not yet wired to any file registry
- Run completion path in `dispatch.go` calls `UpdateRunOutputs`; outputs_json is persisted before COMPLETED state transition — this is the insertion point for auto-registration
- SQLite via goose migrations (migration 00009 would be the managed_files table)
- `S3Backend` + `LocalBackend` storage abstraction; both implement `UploadFileStream` and `Stat`-equivalent operations
- Project-scoped RBAC middleware (`admin`, `executor`, `viewer`) — established pattern, reusable on new endpoints
- WDL `parameter_meta` parsing — server parses the WDL AST already; `type` key extraction requires minor addition to the inputs handler

**Existing frontend capabilities:**
- `RunWorkflowDialog` with per-type input controls; `File` inputs currently render as text fields with a tooltip explaining server-path requirement — this is the insertion point for the file picker
- TanStack Query, shadcn/ui Combobox, Dialog, Command components — available in existing stack
- React 19, TypeScript, zod, react-hook-form — established
- `ProjectDetailPage` is a single-panel page; adding a tab bar (for Members, Files) is a contained UI change

**What is NOT yet built for this milestone:**
- `managed_files` DB table
- File library REST API (`/projects/{slug}/files` CRUD)
- Shell-style label template evaluation
- File picker component replacing text fields in RunWorkflowDialog
- "Files" tab on project detail page
- Auto-registration hook on run completion

---

## Ecosystem Findings: How Comparable Platforms Handle This

### Galaxy (bioinformatics, closest structural analog)

Galaxy tracks every tool execution output as a numbered "dataset" appended to a per-user History.
Each dataset records: name (editable), file format (auto-detected datatype: FASTQ, BAM, VCF, etc.),
size, creation timestamp, the tool that produced it, and user-applied tags. Format auto-detection
runs at upload and at output collection. Tools declare accepted/produced formats; Galaxy enforces
compatibility at wiring time — input slots only show datasets whose format matches the declared type.
Collections (arrays of datasets) group related files for scatter/gather.

Galaxy also has a project-shared "Data Library" for admin-curated files, imported via: local server
path, FTP, existing History datasets, or cloud plugins (S3/Google Drive). Files have genome metadata
alongside type and size.

**Key lesson:** The datatype filter on tool inputs is the highest-value feature. When a tool needs
a BAM file, Galaxy shows only BAM datasets. Users never see irrelevant files.

### Terra (Cromwell/WDL, same language)

Terra uses workspace data tables as its file organization layer. Each table row corresponds to an
entity (sample, cohort, etc.) and columns hold file paths (GCS URIs) and metadata attributes.
After a workflow run, users configure `this.output_name` patterns to write output file paths back
into named columns of the same table row that provided inputs. There is no separate "file library"
entity — files are referenced by path, organized by table column, typed only by column naming
convention (e.g., a column named `bam_file`). The write-back pattern is the de facto standard for
Terra workflows.

**Key lesson:** Auto-registration of output file paths is expected behavior after run completion.
Requiring users to manually import their own workflow outputs is a usability regression.

### Seven Bridges (commercial bioinformatics platform)

Files are first-class project-scoped objects. Tracked metadata: name, tags (user labels), types
array, properties (arbitrary key/value pairs, up to 1,000), media type (MIME), size, creation/
modification timestamps, creator (user/job ID), folder path, state. Tags and types are independent:
types drive tool compatibility; tags are for human organization. Tools declare `patterns` arrays
(e.g., `["*.fastq", "*.fastq.gz"]`) in their input specifications to constrain accepted files.

**Key lesson:** Separating type (for filtering) from label (for human display) from tags (for
organization) is a mature pattern. File types are most valuable when they drive UI filtering.

### DNAnexus (commercial bioinformatics platform)

Files are project-scoped with: ID, name, tags, types array, properties, media type, size, MD5 hash,
creator, folder path, creation/modification timestamps, archival state. Apps declare `inputSpec` with
`patterns` (e.g., `["*.fastq", "*.fastq.gz"]`) for file extension filtering. A `suggestions` field
can direct users to specific project folders containing compatible files — the closest analog to
type-aware file browsing. Required inputs shown visually distinct from optional ones.

**Key lesson:** Visual distinction between required and optional inputs, with a state showing
"missing required files," significantly reduces submission errors.

### Seqera/Nextflow Tower (Nextflow equivalent of this project's UI)

"Tower Datasets" manages structured CSV/TSV input data at workspace level. Each dataset is versioned;
every workflow execution records which dataset version was used. Schema validation (via nf-core
parameter schema) filters available datasets to only those matching the pipeline's expected format —
only valid datasets for a pipeline input are surfaced. "Data Explorer" provides direct browse/search/
upload against cloud buckets (S3/Azure/GCS), bridging raw object storage and pipeline-ready data.

**Key lesson:** Type-filtered input pickers are table stakes for workflow tools. Showing all files
regardless of type is a usability failure. Schema-driven filtering is the preferred mechanism.

### WDL parameter_meta conventions

The WDL 1.2 spec does not standardize `parameter_meta` keys — they are community convention.
The pattern `parameter_meta { reads: { type: "fastq" } }` (structured object value, not a string)
is used at ENCODE, in Broad Institute workflows, and in Terra-based workflows, but is not formally
specified. The OpenWDL spec says items must correspond to declared inputs/outputs and that keys are
added "by convention for each code base and/or tool." The existing flowy inputs endpoint already
parses `parameter_meta` for descriptions; extracting the `type` key is a minor addition.

Shell-style label templates in `parameter_meta` (e.g., `label: "${fastq%%.fastq*}.bam"`) are
entirely custom to this project — no other platform uses this pattern. The concept is sound but
has no ecosystem precedent, which increases implementation risk.

---

## Feature Landscape

### Table Stakes (Users Expect These)

Features that users of comparable platforms treat as baseline. Missing any makes the library feel broken.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| List files in project with name, size, creation date, type, label | Every file system shows these fields; Galaxy, DNAnexus, Seqera all display them | LOW | New `GET /projects/{slug}/files` endpoint; `managed_files` table with these columns |
| Import via browser upload | Primary import path; `/uploads` endpoint already built in v2.1; wiring it to create library entries is the expected completion | LOW | Extend upload handler to insert into `managed_files` after storing file; return file ID alongside path |
| Import via S3/MinIO path | Files already live in S3; users must register by path without re-uploading | LOW | Accept `s3://bucket/key`; validate existence via S3Backend; insert managed_file with source=s3 |
| Import via server local path | Admin/HPC use case for large files mounted on server FS | LOW | Accept absolute path; server stat() to get size; role-gate to admin or executor; source=local |
| File type annotation from `parameter_meta` | Galaxy filters by datatype; Seqera filters by schema type; type-aware picking is universally expected | MEDIUM | Parse `parameter_meta { input: { type: "fastq" } }` at registry-push time; expose in inputs endpoint; store on managed_file at auto-registration |
| User-editable label (defaults to filename) | Galaxy custom dataset names; DNAnexus file renaming; Seven Bridges name field — all support this | LOW | `label` column in `managed_files`; default to `filepath.Base(path)`; editable via PATCH |
| Auto-register all File/Directory outputs on run completion | Terra write-back; Galaxy auto-appends to History; DNAnexus output folder — this is expected behavior in all WDL platforms | MEDIUM | Hook in dispatch.go after `UpdateRunOutputs`; parse outputs_json; for each File/Directory value insert managed_file row with source=workflow_output and run_id FK |
| File picker in RunWorkflowDialog for File-typed inputs | Seqera and DNAnexus both replace text fields with typed pickers; the v2.1 text field was explicitly noted as deferred to v2.2 | MEDIUM | Replace File-type text input with Combobox showing managed_files filtered by type; preserve manual path fallback as toggle |
| Filter managed files by type in picker | Galaxy and Seqera both filter; showing all files regardless of type is a usability failure | MEDIUM | `?file_type=fastq` query param on list endpoint; Combobox filters client-side from fetched list |
| "Files" tab on project detail page | Users need a browsable, importable file list; no navigation to files currently exists | MEDIUM | New tab in ProjectDetailPage; table with columns: label, type badge, size, source, created; Import button; Delete action |
| Delete file from library (record only) | Users must remove stale entries; standard CRUD | LOW | `DELETE /projects/{slug}/files/{id}`; removes DB record; does NOT delete underlying storage object |

### Differentiators (Competitive Advantage)

Features that make flowy's file library meaningfully better than a plain file list.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Shell-style label templates in `parameter_meta` (e.g., `"${fastq%%.fastq*}.bam"`) | Automatically derives meaningful output file labels from input file names without manual annotation; unique to this project | HIGH | Evaluate `${var}`, `${var%suffix}`, `${var%%suffix}`, `${var#prefix}`, `${var##prefix}` at auto-registration time; input variable values available from runs table; no ecosystem precedent — treat as P2 |
| File origin tracking with run link (source_type + run_id FK) | Answers "where did this file come from?" with one click; Seqera added this as a highlighted 2025 feature | LOW | `source_type` enum column (`upload`/`s3_path`/`local_path`/`workflow_output`); `run_id` FK nullable; link in Files tab when source=workflow_output |
| Type inherited from WDL output `parameter_meta` at auto-register | Output files automatically know their type without user annotation; enables downstream filtering immediately | MEDIUM | At auto-registration, look up the WDL output declaration's `parameter_meta` type field; store on managed_file |
| Manual path fallback in file picker | Power users who know exact S3 paths can bypass the picker; DNAnexus CLI supports this; prevents lock-in to the library | LOW | "Enter path manually" toggle in the picker; shows text input when active; always available for inputs with no type match |
| Search/filter in Files tab | As file counts grow, users need to find files quickly; Galaxy added history search as a major v2 feature | LOW | Client-side filter by label, type, path against fetched list; sufficient for typical project sizes (<1,000 files) |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Delete underlying storage object when removing from library | Users want full cleanup | Destructive and irreversible; output files may be referenced by other runs or re-registered; storage management is a separate operational concern | Remove DB record only; document that the storage path is preserved; provide admin tooling for storage cleanup in a future milestone |
| Content-hash deduplication | Avoid duplicate storage for identical files | MD5/SHA computation on genomic files (>10GB) adds significant latency to uploads and imports; complicates S3 path imports (file may not be server-accessible for hashing); adds schema complexity | Allow duplicate registrations; the library is a metadata index, not a storage deduplication layer |
| Global (cross-project) file library | Share files across projects | Breaks project-scoped RBAC isolation; requires re-evaluating the permission model across the entire system | S3 path import already works cross-project implicitly; explicit cross-project sharing is future scope |
| Folder/directory hierarchy in library | Organise files as a filesystem tree | Significant UI complexity; WDL Directory type is uncommon in practice; most bioinformatics workflows deal with flat file lists | Use type + label filtering for organization; Directory files tracked as single entries with their path |
| Automatic file type detection from extension | Infer type from `.bam`, `.fastq.gz`, `.vcf` | Extension sniffing creates false confidence; `.gz` complicates extension parsing; `parameter_meta` is the authoritative declared type | Derive type from `parameter_meta` at auto-registration; user sets type on manual import; no guessing |
| Pagination for large file libraries | Standard for large data sets | Adds API and client complexity; for typical project sizes (hundreds to low thousands) client-side filtering is sufficient | Implement client-side filtering first; server-side pagination only when profiling shows need |
| File preview / viewer | Users want to inspect file contents | Genomic files (BAM, FASTQ, VCF) require specialized viewers; serving raw content through a browser API has security implications (path traversal, content-type spoofing) | Provide download only; format-specific viewers are out of scope for this milestone |

---

## Feature Dependencies

```
[managed_files DB table + Store interface methods]
    required_by ──> [Files tab in ProjectDetailPage]
    required_by ──> [File picker in RunWorkflowDialog]
    required_by ──> [Auto-register outputs on run completion]
    required_by ──> [Import via browser upload]
    required_by ──> [Import via S3/MinIO path]
    required_by ──> [Import via server local path]

[Import via browser upload]
    requires ──> [POST /projects/{slug}/uploads] (existing v2.1)
    requires ──> [managed_files table] (new)

[Import via S3/MinIO path]
    requires ──> [S3Backend Stat/existence check] (existing v1.2)
    requires ──> [managed_files table] (new)

[Auto-register outputs on run completion]
    requires ──> [run completion hook in dispatch.go] (new insertion point)
    requires ──> [outputs_json in runs table] (existing v1.3)
    requires ──> [managed_files table] (new)
    optionally_enhanced_by ──> [Label template evaluation]

[Label template evaluation]
    requires ──> [Auto-register outputs on run completion] (called at same time)
    requires ──> [run inputs_json from runs table] (existing — to resolve variable values)
    requires ──> [managed_files of inputs] (to resolve File-typed variable labels)

[File type annotation from parameter_meta]
    requires ──> [WDL parameter_meta parsing] (existing — server parses WDL AST)
    requires ──> [inputs endpoint extension] (minor: extract "type" key from structured object value)
    enhances ──> [File picker type filter]
    enhances ──> [Auto-register output type]

[File picker in RunWorkflowDialog]
    requires ──> [GET /projects/{slug}/files?file_type=] (new endpoint)
    requires ──> [RunWorkflowDialog] (existing v2.1)
    requires ──> [File type annotation] (to enable useful filtering)
    requires ──> [manual path fallback] (preserve existing UX for users bypassing picker)

[Files tab in ProjectDetailPage]
    requires ──> [GET /projects/{slug}/files] (new)
    requires ──> [ProjectDetailPage refactor to tabs] (existing page, new tab structure)
    requires ──> [Import dialog] (Upload / S3 path / Local path modes)
    requires ──> [DELETE /projects/{slug}/files/{id}] (new)
```

### Dependency Notes

- **managed_files table is the root dependency.** Every other feature in this milestone depends on it. Migration 00009 must be the first deliverable.

- **File picker requires type annotation to be useful.** A picker showing all files regardless of type is only marginally better than a text field. Build the type annotation extraction in the same phase as the managed_files table, before wiring the picker.

- **Auto-register outputs is the highest-value feature and has medium complexity.** The insertion point (dispatch.go after `UpdateRunOutputs`) is clearly identified. The WDL type information (knowing which outputs are File/Directory) is available from the in-memory workflow object at dispatch time — this is the key data point that makes type inference at auto-registration tractable.

- **Label template evaluation depends on managed files of inputs.** The template `${fastq%%.fastq.gz}` resolves `fastq` against the run's input managed_file records. This means: (1) the file picker must store managed_file IDs alongside paths as input values, and (2) the auto-registration path must look up those IDs. This creates a bidirectional dependency that makes label templates a P2 feature, not P1.

- **Browser upload import reuses the v2.1 uploads endpoint entirely.** The handler already stores the file and returns its path. Adding managed_file insertion is a 5-line change in `handleUpload`.

---

## MVP Definition

### Launch With (v2.2)

Minimum viable product: users can browse, import, and pick files.

- [ ] `managed_files` DB table (migration 00009): id, project_id, label, path, file_type (nullable), size_bytes (nullable), source_type (upload/s3_path/local_path/workflow_output), run_id (nullable FK to runs), created_at
- [ ] Store interface additions: `CreateManagedFile`, `ListManagedFiles(projectID, fileType string)`, `GetManagedFileByID`, `DeleteManagedFile`
- [ ] `GET /projects/{slug}/files` — list files; `?file_type=` filter param; viewer role
- [ ] `POST /projects/{slug}/files` — register S3 path or local path; executor role
- [ ] `PATCH /projects/{slug}/files/{id}` — update label or file_type; executor role
- [ ] `DELETE /projects/{slug}/files/{id}` — remove DB record; executor role; does NOT touch storage
- [ ] Extend `handleUpload` (POST /uploads): after storing file, insert managed_file with source_type=upload; return file_id in response alongside path
- [ ] Auto-register all File/Directory outputs in dispatch.go run completion path (source_type=workflow_output, run_id FK populated)
- [ ] Extract `type` key from structured `parameter_meta` object values in inputs endpoint; include in `RegistryInputItem.FileType` field
- [ ] "Files" tab in ProjectDetailPage: table with label, file_type badge, human-readable size, source_type badge, created_at; Import button opens import dialog
- [ ] Import dialog: three tabs (Upload via file picker → POST /uploads, S3 Path entry, Local Path entry → POST /files)
- [ ] File picker in RunWorkflowDialog for File-typed inputs: Combobox filtered by file_type from parameter_meta; shows label + path; "Enter path manually" fallback toggle

### Add After Validation (v2.2.x)

- [ ] Shell-style label template evaluation at auto-registration — HIGH complexity, MEDIUM value; add when users report that auto-generated output labels (UUID paths) are unreadable
- [ ] Run provenance link in Files tab (click source run from a file) — LOW complexity; add as quick follow-on
- [ ] Search/filter field in Files tab — LOW complexity; add when projects accumulate >50 files

### Future Consideration (v2.3+)

- [ ] Directory type files (WDL `Directory`) — rare in practice for current workflows; defer
- [ ] Cross-project file sharing (explicit) — scope creep for v2.2; S3 path import works today
- [ ] File versioning — significant schema change; requires product validation first
- [ ] Bulk operations (select all, bulk delete) — needed only at scale; adds substantial UI complexity

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| managed_files table + Store interface | HIGH | LOW | P1 |
| Auto-register workflow outputs (no templates) | HIGH | MEDIUM | P1 |
| Files tab in ProjectDetailPage | HIGH | MEDIUM | P1 |
| Import via browser upload (extend /uploads) | HIGH | LOW | P1 |
| Import via S3 path | MEDIUM | LOW | P1 |
| File type from parameter_meta (inputs endpoint) | HIGH | LOW | P1 |
| File picker in RunWorkflowDialog (type filter) | HIGH | MEDIUM | P1 |
| Manual path fallback in file picker | MEDIUM | LOW | P1 |
| Import via local server path | LOW | LOW | P1 |
| Delete file from library | LOW | LOW | P1 |
| Edit label/file_type on existing file | MEDIUM | LOW | P2 |
| Search/filter in Files tab | MEDIUM | LOW | P2 |
| Run provenance link in Files tab | MEDIUM | LOW | P2 |
| Source type display in Files tab | MEDIUM | LOW | P2 |
| Shell-style label template evaluation | MEDIUM | HIGH | P2 |

**Priority key:**
- P1: Must have for v2.2 launch
- P2: Should add in v2.2.x follow-on
- P3: Future milestone

---

## Comparable System Analysis

| Feature | Galaxy History | Terra data tables | Seven Bridges files | DNAnexus files | Seqera Tower | flowy v2.2 approach |
|---------|--------------|-------------------|---------------------|----------------|--------------|---------------------|
| Metadata tracked | name, format, size, tags, timestamp | path (in column), attribute columns | name, tags, types, props, size, md5, timestamps | name, tags, types, props, size, md5, folder, timestamps | dataset name, version, created_by, last_used | label, file_type, size_bytes, source_type, run_id, created_at |
| Type filtering in picker | Yes — by format= datatype | Column convention only, no automated filter | Yes — types array drives tool compat | Yes — inputSpec patterns array | Yes — nf-core param schema | Yes — file_type from parameter_meta; Combobox filtered |
| Auto-register outputs | Yes — all tool outputs to History | Write-back configured per workflow, not automatic | Outputs appear in project files (automatic) | Outputs appear in output folder (automatic) | No — user creates Datasets manually | Yes — on COMPLETED, all File/Directory outputs |
| Import methods | upload, FTP, server path, S3 plugin | GCS path entry | upload, API, copy | upload, CLI, API | CSV/TSV upload, S3 event | browser upload, S3 path, local path |
| User-editable labels | Yes — dataset name editable | Column name is label | name field editable | name field editable | dataset name editable | label column, defaults to filename |
| Origin tracking | Links to generating tool | Implicit via table row | Creator (user/job) | Creator (user/job) | Version history | source_type + run_id FK |
| Search/filter | Advanced search (2023+) | Table filter | Metadata-based search | Name/tag search | Filter by metadata | Client-side by label/type (v2.2); server-side later |

---

## Sources

- Terra workflow outputs to data table: https://support.terra.bio/hc/en-us/articles/4500420806299 (403 in current session; content confirmed via web search)
- Terra data management overview: https://support.terra.bio/hc/en-us/articles/360034335332
- Galaxy datatypes: https://galaxyproject.org/learn/datatypes/
- Galaxy 2024 update (RO-Crate, workflow export): https://academic.oup.com/nar/article/52/W1/W83/7676834
- Seven Bridges file metadata: https://docs.sevenbridges.com/docs/metadata-on-the-seven-bridges-platform
- DNAnexus file objects: https://documentation.dnanexus.com/developer/api/introduction-to-data-object-classes/files
- DNAnexus app metadata (inputSpec patterns): https://documentation.dnanexus.com/developer/apps/app-metadata
- Seqera Tower Datasets (type-filtered picker): https://seqera.io/blog/introducing-tower-datasets/
- Seqera Data Explorer: https://seqera.io/blog/introducing-data-explorer/
- WDL parameter_meta spec: https://docs.openwdl.org/en/1.0.0/WDL/parameter_meta/ (404; content from OpenWDL GitHub and community)
- WDL best practices (Dockstore): https://docs.dockstore.org/en/develop/advanced-topics/best-practices/wdl-best-practices.html

---
*Feature research for: Managed File Library — flowy v2.2*
*Researched: 2026-04-09*
