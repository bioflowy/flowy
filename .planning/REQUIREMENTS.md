# Requirements: flowy v2.2 Managed File Library

**Milestone:** v2.2
**Status:** Defined
**Created:** 2026-04-09

## v1 Requirements

### File Library

- [ ] **FILE-01**: User can view a list of managed files in a project, showing path, file type, size, and creation date
- [ ] **FILE-02**: User can delete a managed file record from the library (record-only; does not modify the underlying storage object)

### Import

- [ ] **IMPORT-01**: User can import a file into the project library by uploading from their browser
- [ ] **IMPORT-02**: User can import a file into the project library by entering an S3/MinIO URI
- [ ] **IMPORT-03**: User can import a file into the project library by entering a server-accessible local path (confined to server's storage base directory)

### Auto-Registration

- [ ] **AUTOREG-01**: System automatically registers all File and Directory outputs from a completed workflow run as managed files in the project library

### Type Annotation

- [ ] **ANNOT-01**: Managed files carry a file type label when the WDL workflow's parameter_meta defines a type annotation for that output (e.g. `parameter_meta { reads: { type: "fastq" } }`)

### File Picker

- [ ] **PICKER-01**: User can select a managed file from the project library as input when executing a workflow that has File-typed inputs
- [ ] **PICKER-02**: File picker filters the displayed managed files by type when the WDL parameter_meta specifies a type constraint for that input field

## Future Requirements

These requirements were considered but deferred to a later milestone:

- Shell-style label templates in parameter_meta (e.g. `label: "${fastq%%.fastq*}.bam"`) — novel feature with HIGH implementation complexity; no ecosystem precedent; defer to v2.3+
- Source badge and run provenance link in the Files tab — useful but non-essential for v2.2 core use case
- Pagination for large file libraries — acceptable to add if file counts make it necessary
- Search/filter by name in Files tab

## Out of Scope

- Label template evaluation via system shell — security boundary; any template evaluation must be a sandboxed Go implementation
- Cross-project file sharing — violates project-scoped RBAC boundary
- File versioning — different domain problem; out of scope for this milestone
- Automatic MIME-type detection to populate file_type — file_type is a WDL-semantic annotation (e.g. "fastq", "bam"), not a MIME string
- Deleting the underlying storage object when removing a library record — record-only deletion was chosen to avoid data loss

## Traceability

| REQ-ID | Phase | Plan |
|--------|-------|------|
| FILE-01 | — | — |
| FILE-02 | — | — |
| IMPORT-01 | — | — |
| IMPORT-02 | — | — |
| IMPORT-03 | — | — |
| AUTOREG-01 | — | — |
| ANNOT-01 | — | — |
| PICKER-01 | — | — |
| PICKER-02 | — | — |
