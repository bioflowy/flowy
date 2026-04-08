# flowy

## What This Is

WDL (Workflow Description Language) 1.2 workflow engine implemented in Go. Parses WDL 1.2 source files, type-checks them, and executes tasks via Docker containers with scatter parallelism and conditional support. Ships as four binaries: `flowy` (client CLI), `flowy-server` (REST API + DAG scheduler), `flowy-daemon` (Docker task executor), and `flowy-ui` (web management UI). Supports local single-host execution, distributed multi-host execution via shared filesystem, and S3/MinIO object storage backends for cloud-native deployment. Provides project-based multi-user access control with JWT authentication and role-based permissions, a Docker-registry-style WDL tool/workflow registry with content-hash versioning and mutable tags, registry-based workflow execution, run provenance tracking, and a browser-based management interface for users, projects, registry, and run output browsing.

## Core Value

WDL 1.2で記述されたワークフローをDockerコンテナ上で正確に実行できること。

## Requirements

### Validated

- ✓ WDL 1.2 full parser with AST generation — v1.0
- ✓ WDL type system (Primitive, Compound, Optional, File, Directory) — v1.0
- ✓ Expression evaluator (operators, function calls, type coercion) — v1.0
- ✓ Task input binding and command template expansion — v1.0
- ✓ Docker container task execution with input staging and output collection — v1.0
- ✓ Workflow DAG scheduling with scatter/if support — v1.0
- ✓ Standard library functions (35+ functions) — v1.0
- ✓ CLI tools (`flowy run`, `flowy check`) with all flags — v1.0
- ✓ Spectool conformance testing integration — v1.0
- ✓ Workflow server with REST API (flowy-server binary) — v1.1
- ✓ Execution daemon with task polling (flowy-daemon binary) — v1.1
- ✓ Client CLI with submit/status/outputs/run commands (flowy binary) — v1.1
- ✓ Shared filesystem storage with config-driven directory — v1.1
- ✓ SQLite persistence for workflow runs, task state, execution history — v1.1
- ✓ `flowy run` synchronous mode (submit → poll → results, spectool compatible) — v1.1
- ✓ Multi-daemon scale-out for scatter parallelism — v1.1
- ✓ Storage abstraction interface (Backend with 13 methods, LocalBackend + S3Backend) — v1.2
- ✓ S3 backend implementation (AWS SDK v2, streaming I/O, multipart upload, MinIO compatible) — v1.2
- ✓ Workflow-level backend selection (`--storage` CLI flag + config default) — v1.2
- ✓ Input path scheme resolution (relative → default backend, s3:// → S3, absolute → local FS) — v1.2
- ✓ Daemon LED pattern (Localize-Execute-Delocalize) for S3-backed task execution — v1.2
- ✓ MinIO integration testing (testcontainers-go, spectool conformance on S3 backend) — v1.2
- ✓ User authentication (username/password, JWT tokens) — v1.3
- ✓ Project management with role-based access control (admin/executor/viewer) — v1.3
- ✓ WDL tool/workflow registry with content-hash versioning and tags — v1.3
- ✓ Execution integration with registered tools/workflows — v1.3
- ✓ Provenance tracking for execution output files — v1.3
- ✓ Web UI SPA with React + TypeScript, shadcn/ui, Tailwind CSS — v2.0
- ✓ Separate `flowy-ui` binary serving embedded SPA + proxying API — v2.0
- ✓ Login screen with JWT authentication — v2.0
- ✓ User management (list, create, activate/deactivate, password change) — v2.0
- ✓ Project management (create, list, members, role assignment) — v2.0
- ✓ Registry browsing (entries, versions, tags) — v2.0
- ✓ File browsing and download (S3/local storage output files) — v2.0
- ✓ CORS support on flowy-server for development mode — v2.0
- ✓ UI workflow execution from registry (select, input parameters, submit) — v2.1
- ✓ Poll-based run status monitoring in browser — v2.1
- ✓ Dynamic input form for basic WDL types (String, Int, Float, Boolean, File) — v2.1
- ✓ CLI registry commands (push, list, pull, tag, delete) — v2.1
- ✓ UI registry upload (push WDL from browser) — v2.1

### Active

(None — v2.1 complete, next milestone not yet defined)

### Out of Scope

- Object type — deprecated in WDL 1.2, use Struct instead
- WDL draft-2/1.0/1.1 support — different grammar, high maintenance cost
- WDL lint/zip features — execution engine is priority
- HTTP(S) import resolution — deferred to future milestone
- Execution cache (task output reuse) — deferred to future milestone
- Workflow resume from checkpoint — deferred to future milestone
- Docker-alternative backends (Podman, Singularity) — deferred to future milestone
- WebSocket/SSE for status — polling chosen for v2.1; revisit if real-time needs grow
- OAuth2/LDAP/SSO authentication — simple username/password sufficient for now
- gRPC API — REST is standard in bioinformatics tooling
- Kubernetes integration — enormous scope; Docker per node sufficient for HPC

## Current State

Shipped v2.1 with 37,776 LOC Go + 8,582 LOC TypeScript/React. Six milestones complete (v1.0–v2.1).

v2.1 delivered CLI authentication (`flowy login` with JWT credential storage), CLI registry commands (push/list/pull/tag/delete), browser-based WDL upload, poll-based run status monitoring with auto-stop, and browser workflow execution with dynamic input forms per WDL type. 158 frontend tests across 22 files, all passing.

v2.0 delivered a browser-based management UI as a single-binary deployment (`flowy-ui`). The SPA covers authentication (login, password change, logout with session persistence), user administration (create, deactivate, reactivate, reset password — admin only), project management (create, edit, delete projects; add/remove members; inline role changes), registry browsing (entry list with kind badges, version history, WDL source viewer with line numbers and copy, tag create/move/delete for admins), and file browsing (workflow run list with status badges, output files table, file download via backend proxy, inline text preview for small files). 112 frontend tests across 19 files, all passing. Backend test suite covers all new endpoints.

Phase 32 (API Foundation) complete — added input schema introspection endpoint (`GET /projects/{slug}/registry/{name}/inputs`) and file upload endpoint (`POST /projects/{slug}/uploads`) with 12 tests.

**Capabilities:** Full WDL 1.2 parse → typecheck → evaluate → execute pipeline, distributed server/daemon/client architecture, S3/MinIO storage backends, JWT authentication, project-scoped RBAC (admin/executor/viewer), content-addressed WDL registry with tags, registry-based execution, run provenance tracking, web management UI for users, projects, registry, and run output browsing, API input schema introspection, and file upload endpoint.

## Context

Tech stack: Go 1.24, Docker SDK for Go, AWS SDK for Go v2, hand-written lexer (Rob Pike pattern), Pratt expression parser, chi v5 HTTP router, modernc.org/sqlite (CGO-free), goose migrations, testcontainers-go, golang-jwt/jwt/v5, bcrypt. Frontend: React 19, TypeScript, Vite, shadcn/ui, Tailwind CSS v4, React Router, TanStack Query, Zustand.
Spectool conformance: 156/156 WDL 1.2 tests passing (100% of supported features; 4 categories excluded: Object type, sub-workflow calls, deferred input defaults, read_tsv with Object).
Architecture: four binaries (flowy, flowy-server, flowy-daemon, flowy-ui), SQLite task queue with WAL, local filesystem or S3/MinIO storage backends, JWT-protected REST API with RBAC middleware, embedded React SPA with reverse proxy.
Reference implementation: `../miniwdl` (Python, ~20k lines).

## Constraints

- **Tech stack**: Go 1.24+, Docker SDK for Go, AWS SDK for Go v2
- **WDL version**: 1.2 only (no older version support)
- **Import**: Local files only (HTTP import deferred)
- **Compatibility**: Spectool conformance suite as compliance benchmark
- **Frontend**: React 19, TypeScript, Vite, shadcn/ui — established in v2.0

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| WDL 1.2 only | Scope reduction for reliable implementation | ✓ Good — clean implementation |
| Hand-written lexer (Rob Pike) | Full control over command block modes | ✓ Good — handles all WDL syntax |
| Pratt parser for expressions | Clean operator precedence handling | ✓ Good — all operators correct |
| ShellRunner mock for tests | No Docker dependency in unit tests | ✓ Good — fast CI-friendly tests |
| errgroup for scatter parallelism | Bounded concurrency with cancellation | ✓ Good — clean fail-fast behavior |
| Spectool for conformance | Official WDL compliance benchmark | ✓ Good — 156/156 supported tests passing |
| Separate binaries for server/daemon/client | Clean process separation, independent deployment | ✓ Good — clean architecture |
| Daemon polls server (not push) | Simpler scale-out, daemon only needs server address | ✓ Good — simple and reliable |
| SQLite with WAL + modernc.org/sqlite | CGO-free, single-file DB, WAL for concurrent reads | ✓ Good — zero external dependencies |
| chi v5 for HTTP routing | Lightweight, idiomatic Go, middleware support | ✓ Good — clean handler code |
| Atomic task claiming (single UPDATE) | Avoid TOCTOU race in multi-daemon environments | ✓ Good — correct concurrency |
| runTaskFn injection for dispatch | Extension seam in scheduler for distributed execution | ✓ Good — minimal refactoring |
| ADR 0001: Remove heuristic file staging | Client cannot reliably detect File-typed inputs from JSON | ✓ Good — correct by construction |
| Backend interface for storage | Swap FS/S3 without changing server/daemon code | ✓ Good — clean abstraction |
| LED pattern for daemon S3 I/O | Containers always see local POSIX paths; daemon handles all S3 | ✓ Good — simple container contract |
| ParseS3URI with string ops (not net/url) | Avoids S3 key corruption from path normalization | ✓ Good — preserves key fidelity |
| AWS SDK v2 only | v1 end-of-support July 2025; transfermanager for multipart | ✓ Good — future-proof |
| No credentials in TaskClaimResponse | Security: daemon uses own config/IAM role | ✓ Good — no credential leaks |
| API-based input/output delivery | DB stores inputs/outputs for S3 mode; FS fallback for local | ✓ Good — works for both backends |
| JWT + bcrypt for auth | Simple, stateless auth; no external dependencies | ✓ Good — fits single-server model |
| Project-scoped RBAC middleware chain | JWTMiddleware → projectContext → projectRoleRequired | ✓ Good — clean separation of concerns |
| Content-addressed registry (SHA-256) | Immutable versions, mutable tags — Docker registry model | ✓ Good — familiar UX, integrity guarantees |
| Registry ref resolution at submit + claim | Server resolves by tag at submit, daemon re-resolves by digest | ✓ Good — daemon always gets exact version |
| Run-level provenance (not per-task) | Simpler scope; per-task possible via existing tasks table later | ✓ Good — right granularity for v1.3 |
| Separate flowy-ui binary for SPA hosting | Decouples frontend dev from server, independent deploy | ✓ Good — single binary ships UI + API proxy |
| React + TypeScript + shadcn/ui | Rich ecosystem, copy-paste components, small bundle | ✓ Good — rapid UI development |
| Embedded SPA in Go binary (go:embed) | Single binary deployment, no separate static file hosting | ✓ Good — zero-config deployment |
| Backend proxies file downloads | No presigned S3 URLs — frontend stays simple and secure | ✓ Good — storage-agnostic downloads |
| Project-scoped user list for AddMemberDialog | Avoids relaxing global user list to non-admins | ✓ Good — proper security boundary |

## Current Milestone: v2.1 Job Execution & Registry CLI

**Goal:** Users can execute workflows from the web UI with parameter input and poll-based status monitoring, push/pull/manage registry entries via CLI, and upload WDL files from the browser.

**Target features:**
- UI workflow execution: select from registry, input parameters (basic WDL types), submit, poll-based status monitoring, results display
- CLI registry commands: push, list, pull, tag, delete
- UI registry upload: upload WDL file from browser to push to registry
- Dynamic input form for String, Int, Float, Boolean, File; Array/Map/Struct via JSON input

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-04-07 after v2.1 milestone start*
