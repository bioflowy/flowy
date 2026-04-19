# Milestones

## v2.3 Managed File Labels & UX (Shipped: 2026-04-19)

**Phases completed:** 3 phases, 6 plans | Go: 43,591 LOC | TS: 9,995 LOC | 566 Go tests | 165 frontend tests | 1 day

**Key accomplishments:**

- Safe pure-Go `parameter_meta.label` evaluation for workflow-generated managed-file labels, including fallback behavior for invalid or missing template inputs
- Workflow output auto-registration now stores human-readable labels derived from workflow input values instead of always defaulting to the basename
- Managed-file persistence and API responses now expose explicit provenance via `source_type` and optional `run_id`
- Files tab now shows source badges, conditional run links, and client-side search/type filtering for faster file discovery
- `RunWorkflowDialog` now renders managed files label-first while preserving path-based execution semantics and manual path fallback
- Added a Japanese verification guide for the milestone in `docs/v2.3-verification-guide.md`

---

## v2.2 Managed File Library (Shipped: 2026-04-19)

**Phases completed:** 4 phases, 8 plans | Go: 43,239 LOC | TS: 9,829 LOC | 564 Go tests | 162 frontend tests | 11 days

**Key accomplishments:**

- Project-scoped managed file persistence with `managed_files` schema, store CRUD, and atomic `CompleteRun(...)` support
- Managed-file REST API for list/import/delete, including browser upload, S3 URI import, and guarded server-path import via `ImportRoot`
- Automatic registration of workflow `File` and `Directory` outputs, including array outputs and `parameter_meta.type` propagation
- Browser Files tab with import dialog, delete flow, and managed-file metadata rendering in project detail pages
- `RunWorkflowDialog` now uses a managed-file picker for `File` inputs, filtered by `parameter_meta.type`, while preserving manual path fallback
- Registry input lookup accepts digest-based selection and registry push auto-assigns `latest`, eliminating 404s when the UI selects immutable digests

---

## v2.1 Job Execution & Registry CLI (Shipped: 2026-04-09)

**Phases completed:** 5 phases, 10 plans | Go: 41,483 LOC | TS: 9,193 LOC | 533 Go tests | 3 days

**Key accomplishments:**

- Input schema introspection endpoint (`GET /registry/{name}/inputs`) returns structured WDL input declarations enabling dynamic browser forms
- File upload endpoint (`POST /projects/{slug}/uploads`) with path-traversal protection, 100 MB limit, executor role gating, and 12 passing tests
- `flowy login` with secure JWT credential storage (~/.flowy/credentials.yaml, 0600/0700 permissions) and `flowy registry` subcommands (push/list/pull/tag/delete-tag/delete)
- PushWDLDialog for browser-based WDL upload with FileReader, client-side .wdl validation, 422/409 error handling, and executor+ role gating
- `usePollingRuns` hook with exported `TERMINAL_STATES` — auto-refreshes at 3s for active runs, stops on terminal state; pulsing dot in RunsPage and RunDetailPage
- RunWorkflowDialog with dynamic zod schema and per-type controls (String/Int/Float/Boolean/File/JSON) driven by the inputs endpoint; submission navigates to RunDetailPage

---

## v2.1 Job Execution & Registry CLI (Shipped: 2026-04-08)

**Phases completed:** 5 phases, 10 plans, 9 tasks

**Key accomplishments:**

- One-liner:
- One-liner:
- One-liner:
- One-liner:
- `usePushWDL(slug)`
- One-liner:
- One-liner:
- RunsPage and RunDetailPage wired to usePollingRuns with pulsing yellow dot for PENDING/RUNNING states and 1-second duration tick for RUNNING runs on detail page.
- One-liner:
- One-liner:

---

## v2.0 Web UI — Management (Shipped: 2026-04-07)

**Phases completed:** 7 phases, 16 plans, 2 tasks

**Key accomplishments:**

- (none recorded)

---

## v1.3 Project & Registry (Shipped: 2026-03-20)

**Phases completed:** 6 phases, 14 plans, 2 tasks

**Key accomplishments:**

- (none recorded)

---

## v1.2 S3 Object Storage (Shipped: 2026-03-13)

**Phases completed:** 5 phases, 10 plans
**Timeline:** 2 days (2026-03-12 → 2026-03-13)
**Codebase:** 29,277 lines Go (+14,332 / -1,286), 92 files modified, 81 commits
**Git range:** `22a01d1..2589b52`

**Key accomplishments:**

1. Storage Backend interface with 13 methods and LocalBackend — all 156 spectool tests pass unchanged after refactor
2. S3Backend implementation via AWS SDK v2 with streaming I/O, multipart upload, and MinIO path-style compatibility
3. `--storage` CLI flag, YAML storage config, and s3:// URI path guard preventing filepath.Clean corruption
4. Daemon LED pattern (Localize-Execute-Delocalize) with API-based input/output delivery — containers always see local POSIX paths
5. MinIO integration tests via testcontainers-go and `make spectool-s3` full conformance target
6. 3 quick-fix tasks for S3 mode spectool compatibility (DB-based outputs, path normalization, URI leak fix)

**Tech debt accepted:**

- `make integration` excluded daemon LED tests (fixed post-audit)
- MinIO image tag requires local re-tagging in some environments
- SUMMARY.md frontmatter missing requirements_completed fields across phases

**Archive:** `.planning/milestones/v1.2-ROADMAP.md`, `.planning/milestones/v1.2-REQUIREMENTS.md`

---

## v1.1 Distributed Execution (Shipped: 2026-03-12)

**Phases completed:** 5 phases, 10 plans
**Timeline:** 2 days (2026-03-10 → 2026-03-12)
**Codebase:** 25,433 lines Go (+13,504 / -627), 86 files modified, 64 commits
**Git range:** test(09-01) → docs(13-01)

**Key accomplishments:**

1. Shared foundation — YAML config, HTTP DTO types, SQLite persistence (WAL mode, CGO-free), filesystem layout helpers
2. flowy-server binary — REST API (submit/status/outputs/abort/health), DAG dispatch to SQLite task queue, stale task detection
3. flowy-daemon binary — poll-based task claiming, Docker execution, heartbeat reporting, graceful SIGTERM shutdown
4. flowy client CLI — submit/status/outputs subcommands, synchronous `flowy run` with polling, spectool compatibility preserved
5. Code quality cleanup — consolidated ValueToInterface into shared wdl/values package, documentation fixes, API surface cleanup

**Archive:** `.planning/milestones/v1.1-ROADMAP.md`, `.planning/milestones/v1.1-REQUIREMENTS.md`

---

## v1.0 MVP (Shipped: 2026-03-11)

**Phases completed:** 8 phases, 26 plans
**Timeline:** 2 days (2026-03-10 to 2026-03-11)
**Codebase:** 17,648 lines Go, 204 files, 160 commits

**Key accomplishments:**

1. WDL 1.2 type system — all primitive, compound, optional types with type-tagged values
2. Full parser with Pratt expression sub-parser — lexer, parser, AST with command block modes and string interpolation
3. Type checker with forward references — coercion rules, scatter/if promotion, struct validation
4. Expression evaluator + 35+ stdlib functions — all WDL 1.2 expressions and standard library
5. Docker-based task execution — input staging, container lifecycle, output collection with mock-testable interface
6. DAG workflow scheduler — scatter parallelism, conditionals, errgroup-based concurrency, fail-fast cancellation
7. CLI tools — `flowy run` and `flowy check` with all flags and proper exit codes
8. Spectool conformance baseline — 45/160 WDL 1.2 conformance tests passing

**Tech debt accepted:**

- 45/160 spectool conformance (28% baseline) — missing None keyword, sep(), shorthand call syntax
- jsonToValueUntyped duplicated between task and workflow packages
- Workflow input binding uses untyped JSON conversion

**Archive:** `.planning/milestones/v1.0-ROADMAP.md`, `.planning/milestones/v1.0-REQUIREMENTS.md`

---
