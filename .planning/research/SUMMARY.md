# Project Research Summary

**Project:** flowy v2.2 — Managed File Library
**Domain:** WDL 1.2 workflow engine — project-scoped file catalog and file picker
**Researched:** 2026-04-09
**Confidence:** HIGH

## Executive Summary

The v2.2 Managed File Library adds a project-scoped file catalog to a fully-operational workflow engine. Every comparable bioinformatics platform (Galaxy, Terra, Seven Bridges, DNAnexus, Seqera Tower) treats file auto-registration and type-filtered file picking as table stakes. The recommended approach follows a strict dependency graph: DB migration and Store interface first, then REST API and Backend.Stat, then auto-registration with parameter_meta extraction, then UI. The `managed_files` table is the single root dependency — nothing else can be built without it.

Zero new Go dependencies are required (stdlib and existing internal packages only). One new frontend package is needed: `@radix-ui/react-tabs` (shadcn tabs component for the Files tab in ProjectDetailPage). The most significant backend change is adding a `CompleteRun` atomic method to the `Store` interface wrapping output registration and state transition in a single SQLite transaction — without this, the system is permanently inconsistable on crash.

The highest-risk implementation details are not algorithmic but correctness-discipline issues: five independent code paths each have a specific failure mode producing silent, persistent corruption (non-atomic run completion, missing HeadObject on S3APIClient, FileValue/StringValue JSON ambiguity, parameter_meta type assertion panics, label template injection). All five are preventable with patterns already established in the codebase. Shell-style label templates in `parameter_meta` are a novel differentiator with no ecosystem precedent and should be treated as P2 scope.

## Key Findings

### Recommended Stack

**Go additions (zero new go.mod entries):**
- `os.Stat` (stdlib) — file size for local paths
- `s3.HeadObject` via existing AWS SDK for Go v2 — file size for S3 paths (requires `HeadObject` addition to `S3APIClient` interface)
- goose migration `00009_managed_files.sql` — follows exact existing pattern

**Frontend additions:**
- `@radix-ui/react-tabs@^1.1.13` — install via `npx shadcn@latest add tabs` for Files tab in ProjectDetailPage
- All other UI needs (table, dialog, search, upload) met by existing shadcn/ui components

**Explicit rejections:** No MIME-type detection library (file_type stores WDL-semantic annotations, not MIME); no external shell evaluator library for label templates (narrow Go evaluator ~50 lines handles bioinformatics cases).

### Expected Features

**Must have (P1 — table stakes):**
- `managed_files` DB table + Store CRUD (root dependency)
- Auto-register all File/Directory workflow outputs on run completion
- Files tab in ProjectDetailPage (list, delete, import)
- Import: browser upload (extend existing handler), S3/MinIO path, server local path
- File type annotation from `parameter_meta { reads: { type: "fastq" } }`
- File picker in RunWorkflowDialog (replaces text field for File-typed inputs, filter by type)
- Delete managed file record (record-only, never touches storage object)

**Should have (P2):**
- Shell-style label templates in parameter_meta (e.g. `label: "${fastq%%.fastq*}.bam"`) — novel, HIGH complexity, no ecosystem precedent
- Run provenance link in Files tab (which run produced this file)
- Pagination for large file libraries
- Search/filter by name or type in Files tab

**Anti-features (never add):**
- Automatic MIME-type detection replacing WDL semantic types
- Cross-project file sharing (RBAC boundary violation)
- File versioning (out of scope, different domain problem)

### Architecture Approach

**New components:**
- `00009_managed_files.sql` — goose migration (root dependency for all phases)
- `internal/store/managed_files.go` — Store interface +5 methods + sqlite implementation
- `internal/server/handlers_files.go` — List/Delete/Import endpoints (`/projects/{slug}/files`)
- `internal/label/label.go` — pure Go label template engine (no external deps)
- `web/src/features/files/` — FilesTab, ImportFileDialog, FilePicker, useFiles hook

**Modified components:**
- `storage/backend.go` — add `Stat(path string) (int64, error)` method
- `storage/local.go` + `storage/s3.go` — implement `Stat()`; `s3.go` also adds `HeadObject` to S3APIClient interface
- `internal/store/store.go` — add `CompleteRun(ctx, runID, outputs, files)` atomic method
- `internal/server/dispatch.go` — call `registerRunOutputFiles()` in run completion branch
- `internal/server/handlers_registry.go` — extend `extractInputs()` to include `file_type`/`label_template` from parameter_meta
- `internal/api/types.go` — extend `RegistryInputItem` with `FileType` and `LabelTemplate` fields
- `web/src/features/registry/RunWorkflowDialog.tsx` — integrate FilePicker for File-typed inputs

**Build order (mandatory dependency chain):**
1. DB migration + Store interface (including `CompleteRun`) + Backend.Stat + S3APIClient HeadObject — all must be atomic to keep codebase compilable
2. File Library REST API — `handlers_files.go` + route registration + ImportRoot guard; Phase 2 and 3 can proceed in parallel after this
3. Workflow completion hook + parameter_meta extension — `label.go` + `registerRunOutputFiles()` + `extractInputs()` extension
4. UI — Files tab + ImportFileDialog + FilePicker + RunWorkflowDialog integration (file picker last — highest-risk change)

### Critical Pitfalls

1. **Non-atomic run completion** — `Store.CompleteRun` must wrap `UpdateRunOutputs` + managed_files inserts + `UpdateRunState` in one SQLite transaction. The current server holds `store.Store` interface only; `WriterDB()` on the concrete type is inaccessible. Design `CompleteRun` in Phase 1.

2. **S3APIClient missing HeadObject** — add it in the same phase as the migration. S3-sourced managed files will have size=0 permanently if deferred; the `S3APIClient` interface in `s3.go` currently only has `PutObject`, `GetObject`, `ListObjectsV2`.

3. **FileValue/StringValue JSON ambiguity** — `ValueToInterface` in `values/convert.go` produces identical JSON for `FileValue`, `DirectoryValue`, and `StringValue`. Auto-registration must correlate against `doc.Workflows[0].Outputs` WDL type declarations — never use path heuristics. Handle `Array[File]` by walking each element individually.

4. **parameter_meta backward compatibility** — existing `ex_paramter_meta_task.wdl` uses `{ help: "...", suggestions: [...] }` object hints with no `type` or `label` keys. Every type assertion on `ParamMeta` must use comma-ok checks or the conformance suite will panic.

5. **Label template injection** — `${fastq%%.fastq*}.bam` must be evaluated by a narrow Go evaluator (~50 lines), not `text/template`. The `{{.Method}}` Go template syntax allows calling arbitrary methods against Go objects.

6. **Path traversal in local-path import** — add `ImportRoot` config field (default: `cfg.Storage.BaseDir`); always run `filepath.Clean` + HasPrefix assertion before accepting any user-supplied local path. Return 400 with a generic error message (do not leak the ImportRoot value).

## Implications for Roadmap

**Suggested phases: 4**

| Phase | Name | Delivers |
|-------|------|----------|
| 37 | Backend Foundation | DB migration, Store interface + `CompleteRun`, Backend.Stat, S3APIClient HeadObject |
| 38 | File Library REST API | GET/POST/DELETE /projects/{slug}/files, ImportRoot path guard, integration tests |
| 39 | Workflow Completion Hook + parameter_meta | `label.go`, `registerRunOutputFiles()` in dispatch.go, `RegistryInputItem` extension |
| 40 | UI — Files Tab + File Picker | FilesTab, ImportFileDialog, FilePicker, RunWorkflowDialog integration |

Phase 37→38→40 and 37→39→40 are hard dependency chains. Phases 38 and 39 can be developed in parallel after Phase 37 completes.

**Research flags:** No phase needs `/gsd-research-phase` — all patterns established in existing codebase.

**Test coverage gates (not research gaps):**
- Phase 38: path-traversal negative test (`path=../../etc/passwd` returns 400)
- Phase 39: label template edge cases (empty match, glob suffix, missing variable, adversarial input); `Array[File]` output registration
- Phase 39: parameter_meta backward compat — `ex_paramter_meta_task.wdl` must still pass

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Direct `go.mod` and `package.json` inspection; zero new Go deps confirmed |
| Features | HIGH | Ecosystem analysis of 5 platforms + direct codebase insertion point inspection |
| Architecture | HIGH | All integration points from direct file inspection with specific line references |
| Pitfalls | HIGH | All pitfalls identified from specific files and lines; not theoretical |

**Overall confidence:** HIGH

## Sources

- Codebase — `internal/store/store.go`, `internal/server/dispatch.go`, `storage/backend.go`, `storage/s3.go`
- Codebase — `values/convert.go`, `wdl/ast/ast.go`, `wdl/parser/parser.go`
- Codebase — `web/src/features/registry/RunWorkflowDialog.tsx`, `web/src/features/projects/ProjectDetailPage.tsx`
- Codebase — `internal/server/migrations/` (goose pattern), `internal/api/types.go`
- Terra, Galaxy, Seven Bridges, DNAnexus, Seqera Tower — file library / file picker patterns
- pkg.go.dev/mvdan.cc/sh/v3/shell — confirmed `%%` suffix stripping is NOT supported
- ui.shadcn.com/docs/components/tabs, npmjs.com/package/@radix-ui/react-tabs — version confirmed

---
*Research completed: 2026-04-09*
*Ready for roadmap: yes*
