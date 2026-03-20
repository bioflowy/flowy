# flowy

## What This Is

WDL (Workflow Description Language) 1.2 workflow engine implemented in Go. Parses WDL 1.2 source files, type-checks them, and executes tasks via Docker containers with scatter parallelism and conditional support. Ships as three binaries: `flowy` (client CLI), `flowy-server` (REST API + DAG scheduler), and `flowy-daemon` (Docker task executor). Supports local single-host execution, distributed multi-host execution via shared filesystem, and S3/MinIO object storage backends for cloud-native deployment. Provides project-based multi-user access control, a Docker-registry-style WDL tool/workflow registry with content-hash versioning and tags, and execution provenance tracking linking outputs to source tool/workflow versions and inputs.

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

### Active

- ✓ User authentication (username/password, JWT tokens) — Validated in Phase 19: User Authentication
- ✓ Project management with role-based access control (admin/executor/viewer) — Validated in Phase 20: Projects and RBAC
- ✓ WDL tool/workflow registry with content-hash versioning and tags — Validated in Phase 21: WDL Registry
- ✓ Execution integration with registered tools/workflows — Validated in Phase 22: Registry-Based Execution
- ✓ Provenance tracking for execution output files — Validated in Phase 23: Provenance Tracking

### Out of Scope

- Object type — deprecated in WDL 1.2, use Struct instead
- WDL draft-2/1.0/1.1 support — different grammar, high maintenance cost
- WDL lint/zip features — execution engine is priority
- HTTP(S) import resolution — deferred to future milestone
- Execution cache (task output reuse) — deferred to future milestone
- Workflow resume from checkpoint — deferred to future milestone
- Docker-alternative backends (Podman, Singularity) — deferred to future milestone
- WebSocket/SSE for status — polling sufficient for batch workflows; revisit when Web UI added
- OAuth2/LDAP/SSO authentication — simple username/password sufficient for now
- gRPC API — REST is standard in bioinformatics tooling
- Kubernetes integration — enormous scope; Docker per node sufficient for HPC

## Current Milestone: v1.3 Project & Registry

**Goal:** Add multi-user project management, Docker-registry-style WDL tool/workflow versioning, and execution provenance tracking.

**Target features:**
- User authentication with username/password and JWT tokens
- Projects with role-based access (admin, executor, viewer)
- WDL tool (task) and workflow registry with content-hash + tag versioning
- Execution of registered tools/workflows within project context
- Provenance tracking linking output files to their source tool/workflow, version, and inputs

## Context

Shipped v1.2 with 29,277 LOC Go.
Tech stack: Go 1.24, Docker SDK for Go, AWS SDK for Go v2, hand-written lexer (Rob Pike pattern), Pratt expression parser, chi v5 HTTP router, modernc.org/sqlite (CGO-free), goose migrations, testcontainers-go.
Spectool conformance: 156/156 WDL 1.2 tests passing (100% of supported features; 4 categories excluded: Object type, sub-workflow calls, deferred input defaults, read_tsv with Object).
Architecture: three binaries (flowy, flowy-server, flowy-daemon), SQLite task queue, local filesystem or S3/MinIO storage backends.
Reference implementation: `../miniwdl` (Python, ~20k lines).

## Constraints

- **Tech stack**: Go 1.24+, Docker SDK for Go, AWS SDK for Go v2
- **WDL version**: 1.2 only (no older version support)
- **Import**: Local files only (HTTP import deferred)
- **Compatibility**: Spectool conformance suite as compliance benchmark

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

---
*Last updated: 2026-03-20 after Phase 24 (Fix CLI Polling for Project-Scoped Runs) completion*
