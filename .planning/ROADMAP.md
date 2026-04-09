# Roadmap: flowy

## Overview

flowy is a WDL 1.2 workflow engine in Go. v1.0 shipped as a CLI tool with full parse → typecheck → evaluate → execute pipeline. Spectool conformance baseline: 45/160. v1.1 shipped distributed server/daemon/client architecture for multi-host workflow execution. v1.2 shipped S3/MinIO object storage as an alternative backend to shared filesystem, enabling cloud-native WDL execution. Spectool conformance: 156/156. v1.3 shipped multi-user authentication, project-scoped RBAC, a Docker-registry-style WDL content-addressed registry, registry-based workflow execution, and run provenance tracking. v2.0 shipped a browser-based management SPA embedded in a new `flowy-ui` binary, covering authentication, user/project/registry management, and file browsing with download. v2.1 shipped CLI registry management, browser WDL upload, poll-based run monitoring, and full workflow execution from the browser with dynamic input forms. v2.2 ships a project-scoped managed file library with import, auto-registration, type annotation, and a file picker for browser workflow execution.

## Milestones

- ✅ **v1.0 MVP** — Phases 1-8 (shipped 2026-03-11)
- ✅ **v1.1 Distributed Execution** — Phases 9-13 (shipped 2026-03-12)
- ✅ **v1.2 S3 Object Storage** — Phases 14-18 (shipped 2026-03-13)
- ✅ **v1.3 Project & Registry** — Phases 19-24 (shipped 2026-03-20)
- ✅ **v2.0 Web UI — Management** — Phases 25-31 (shipped 2026-04-07)
- ✅ **v2.1 Job Execution & Registry CLI** — Phases 32-36 (shipped 2026-04-09)
- 🚧 **v2.2 Managed File Library** — Phases 37-40 (in progress)

## Phases

<details>
<summary>✅ v1.0 MVP (Phases 1-8) — SHIPPED 2026-03-11</summary>

- [x] **Phase 1: Type Data Structures** (1/1 plans) — completed 2026-03-10
- [x] **Phase 2: Lexer, Parser, and AST** (5/5 plans) — completed 2026-03-10
- [x] **Phase 3: Type Checker** (4/4 plans) — completed 2026-03-10
- [x] **Phase 4: Expression Evaluator and Standard Library** (6/6 plans) — completed 2026-03-10
- [x] **Phase 5: Task Execution** (3/3 plans) — completed 2026-03-10
- [x] **Phase 6: Workflow Execution, DAG Scheduler** (3/3 plans) — completed 2026-03-10
- [x] **Phase 7: CLI and Integration** (2/2 plans) — completed 2026-03-11
- [x] **Phase 8: Specification Compliance** (2/2 plans) — completed 2026-03-11

Full details: `.planning/milestones/v1.0-ROADMAP.md`

</details>

<details>
<summary>✅ v1.1 Distributed Execution (Phases 9-13) — SHIPPED 2026-03-12</summary>

- [x] **Phase 9: API Contract + Persistence Foundation** (2/2 plans) — completed 2026-03-11
- [x] **Phase 10: Server Binary + DAG Dispatch** (2/2 plans) — completed 2026-03-11
- [x] **Phase 11: Daemon Binary + Task Execution** (2/2 plans) — completed 2026-03-12
- [x] **Phase 12: Client CLI + Spectool Compatibility** (3/3 plans) — completed 2026-03-12
- [x] **Phase 13: Tech Debt Cleanup** (1/1 plan) — completed 2026-03-12

Full details: `.planning/milestones/v1.1-ROADMAP.md`

</details>

<details>
<summary>✅ v1.2 S3 Object Storage (Phases 14-18) — SHIPPED 2026-03-13</summary>

- [x] **Phase 14: Storage Abstraction** (2/2 plans) — completed 2026-03-13
- [x] **Phase 15: Configuration and Path Resolution** (2/2 plans) — completed 2026-03-13
- [x] **Phase 16: S3Backend Implementation** (2/2 plans) — completed 2026-03-13
- [x] **Phase 17: Daemon Staging** (2/2 plans) — completed 2026-03-13
- [x] **Phase 18: MinIO Integration Testing** (2/2 plans) — completed 2026-03-13

Full details: `.planning/milestones/v1.2-ROADMAP.md`

</details>

<details>
<summary>✅ v1.3 Project & Registry (Phases 19-24) — SHIPPED 2026-03-20</summary>

- [x] **Phase 19: User Authentication** (3/3 plans) — completed 2026-03-17
- [x] **Phase 20: Projects and RBAC** (3/3 plans) — completed 2026-03-17
- [x] **Phase 21: WDL Registry** (3/3 plans) — completed 2026-03-20
- [x] **Phase 22: Registry-Based Execution** (2/2 plans) — completed 2026-03-20
- [x] **Phase 23: Provenance Tracking** (2/2 plans) — completed 2026-03-20
- [x] **Phase 24: Fix CLI Polling for Project-Scoped Runs** (1/1 plan) — completed 2026-03-20

Full details: `.planning/milestones/v1.3-ROADMAP.md`

</details>

<details>
<summary>✅ v2.0 Web UI — Management (Phases 25-31) — SHIPPED 2026-04-07</summary>

- [x] **Phase 25: Infrastructure Foundation** (3/3 plans) — completed 2026-03-24
- [x] **Phase 26: Authentication** (3/3 plans) — completed 2026-04-06
- [x] **Phase 27: User Management** (2/2 plans) — completed 2026-04-06
- [x] **Phase 28: Project Management** (2/2 plans) — completed 2026-04-06
- [x] **Phase 29: Registry Browsing** (2/2 plans) — completed 2026-04-07
- [x] **Phase 30: File Browsing** (2/2 plans) — completed 2026-04-07
- [x] **Phase 31: Audit Gap Closure** (2/2 plans) — completed 2026-04-07

Full details: `.planning/milestones/v2.0-ROADMAP.md`

</details>

<details>
<summary>✅ v2.1 Job Execution & Registry CLI (Phases 32-36) — SHIPPED 2026-04-09</summary>

- [x] **Phase 32: API Foundation** (2/2 plans) — completed 2026-04-07
- [x] **Phase 33: CLI Auth and Registry Commands** (2/2 plans) — completed 2026-04-07
- [x] **Phase 34: Browser Registry Upload** (2/2 plans) — completed 2026-04-07
- [x] **Phase 35: Run Status Polling** (2/2 plans) — completed 2026-04-08
- [x] **Phase 36: UI Workflow Execution** (2/2 plans) — completed 2026-04-08

Full details: `.planning/milestones/v2.1-ROADMAP.md`

</details>

### 🚧 v2.2 Managed File Library (In Progress)

**Milestone Goal:** Introduce a project-scoped managed file library so workflow inputs and outputs are tracked centrally, enabling file-picker-based workflow execution.

- [ ] **Phase 37: Backend Foundation** - DB migration, Store interface with CompleteRun, Backend.Stat, S3APIClient HeadObject
- [ ] **Phase 38: File Library REST API** - GET/POST/DELETE /projects/{slug}/files endpoints, ImportRoot path guard
- [ ] **Phase 39: Workflow Completion Hook and Type Annotation** - Auto-register run outputs, parameter_meta type extraction
- [ ] **Phase 40: UI — Files Tab and File Picker** - Files tab in ProjectDetailPage, ImportFileDialog, FilePicker in RunWorkflowDialog

## Phase Details

### Phase 37: Backend Foundation
**Goal**: The data layer and storage interfaces needed to persist and measure managed files are in place — all subsequent phases can build on a stable, consistent foundation
**Depends on**: Phase 36
**Requirements**: (foundational — no direct user-facing requirements; enables FILE-01, FILE-02, IMPORT-01, IMPORT-02, IMPORT-03, AUTOREG-01, ANNOT-01, PICKER-01, PICKER-02)
**Success Criteria** (what must be TRUE):
  1. A `managed_files` table exists in the SQLite DB after goose migration `00009_managed_files.sql` runs
  2. The Store interface exposes CRUD methods for managed files and a `CompleteRun` method that wraps output registration and state transition in a single SQLite transaction
  3. `Backend.Stat(path)` returns the byte size of a file for both LocalBackend (via `os.Stat`) and S3Backend (via `HeadObject`)
  4. The S3APIClient interface includes `HeadObject` and the real AWS client satisfies it
  5. All existing tests pass unchanged after the interface additions
**Plans**: TBD

### Phase 38: File Library REST API
**Goal**: Users and the system can list, import, and delete managed files in a project through a well-guarded REST API
**Depends on**: Phase 37
**Requirements**: FILE-01, FILE-02, IMPORT-01, IMPORT-02, IMPORT-03
**Success Criteria** (what must be TRUE):
  1. `GET /projects/{slug}/files` returns the list of managed files for the project with path, file_type, size, and created_at fields
  2. `DELETE /projects/{slug}/files/{id}` removes the managed file record without modifying the underlying storage object
  3. `POST /projects/{slug}/files` with a browser multipart upload stores the file and registers it in the library
  4. `POST /projects/{slug}/files` with an `s3://` URI validates the path and registers an S3-backed managed file record with size from HeadObject
  5. `POST /projects/{slug}/files` with a server local path is rejected with HTTP 400 if the path traverses outside the configured ImportRoot directory; a valid path within ImportRoot registers the file
**Plans**: TBD

### Phase 39: Workflow Completion Hook and Type Annotation
**Goal**: Every completed workflow run automatically populates the project file library, and file type metadata from WDL parameter_meta flows through to managed file records
**Depends on**: Phase 37
**Requirements**: AUTOREG-01, ANNOT-01
**Success Criteria** (what must be TRUE):
  1. When a workflow run reaches the COMPLETED state, all File and Directory outputs are automatically inserted as managed file records in the project library within the same `CompleteRun` transaction
  2. `Array[File]` and `Array[Directory]` outputs are each individually registered as separate managed file records
  3. When a WDL workflow output has `parameter_meta { output_name: { type: "fastq" } }`, the registered managed file record carries `file_type = "fastq"`
  4. Existing WDL files that use non-type-annotated `parameter_meta` (e.g., `{ help: "...", suggestions: [...] }`) continue to execute correctly without panics
  5. All 156 spectool conformance tests continue to pass after the dispatch.go changes
**Plans**: TBD

### Phase 40: UI — Files Tab and File Picker
**Goal**: Users can manage their project's file library from the browser and select managed files directly when launching workflows
**Depends on**: Phase 38, Phase 39
**Requirements**: PICKER-01, PICKER-02
**Success Criteria** (what must be TRUE):
  1. A "Files" tab is visible in ProjectDetailPage and displays the managed file list with columns for path, file type, size, and creation date
  2. User can open an ImportFileDialog from the Files tab and import a file via browser upload, S3 URI, or server local path
  3. User can delete a managed file record from the Files tab; the row disappears and a confirmation is shown
  4. File-typed inputs in RunWorkflowDialog display a file picker component that lists managed files from the project library instead of a free-text field
  5. When a WDL input has a type constraint in parameter_meta, the file picker shows only managed files whose `file_type` matches that constraint; a manual path fallback remains available
**Plans**: TBD
**UI hint**: yes

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1-8 | v1.0 | 26/26 | Complete | 2026-03-11 |
| 9-13 | v1.1 | 10/10 | Complete | 2026-03-12 |
| 14-18 | v1.2 | 10/10 | Complete | 2026-03-13 |
| 19-24 | v1.3 | 14/14 | Complete | 2026-03-20 |
| 25-31 | v2.0 | 16/16 | Complete | 2026-04-07 |
| 32-36 | v2.1 | 10/10 | Complete | 2026-04-09 |
| 37. Backend Foundation | v2.2 | 0/? | Not started | - |
| 38. File Library REST API | v2.2 | 0/? | Not started | - |
| 39. Workflow Completion Hook and Type Annotation | v2.2 | 0/? | Not started | - |
| 40. UI — Files Tab and File Picker | v2.2 | 0/? | Not started | - |
