# flowy

## What This Is

WDL (Workflow Description Language) 1.2 workflow engine implemented in Go. Parses WDL 1.2 source files, type-checks them, and executes tasks via Docker containers with scatter parallelism and conditional support. Ships as three binaries: `flowy` (client CLI), `flowy-server` (REST API + DAG scheduler), and `flowy-daemon` (Docker task executor). Supports both local single-host execution and distributed multi-host execution via shared filesystem and SQLite task queue.

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

### Active

(None — next milestone requirements to be defined via `/gsd:new-milestone`)

### Out of Scope

- Object type — deprecated in WDL 1.2, use Struct instead
- WDL draft-2/1.0/1.1 support — different grammar, high maintenance cost
- WDL lint/zip features — execution engine is priority
- S3/object storage backend — shared filesystem first, S3 deferred to future milestone
- HTTP(S) import resolution — deferred to future milestone
- Execution cache (task output reuse) — deferred to future milestone
- Workflow resume from checkpoint — deferred to future milestone
- Docker-alternative backends (Podman, Singularity) — deferred to future milestone
- WebSocket/SSE for status — no web UI; polling sufficient for batch workflows
- JWT/OAuth2 authentication — HPC clusters use network-level security; add later if needed
- gRPC API — REST is standard in bioinformatics tooling
- Kubernetes integration — enormous scope; Docker per node sufficient for HPC

## Context

Shipped v1.1 with 25,433 LOC Go.
Tech stack: Go 1.24, Docker SDK for Go, hand-written lexer (Rob Pike pattern), Pratt expression parser, chi v5 HTTP router, modernc.org/sqlite (CGO-free), goose migrations.
Spectool conformance: 45/160 WDL 1.2 tests passing (28% baseline).
Architecture: three binaries (flowy, flowy-server, flowy-daemon), SQLite task queue, shared filesystem for I/O.
Known gaps: None keyword, sep() function, shorthand call syntax, length() on Map/Pair.
Reference implementation: `../miniwdl` (Python, ~20k lines).

## Constraints

- **Tech stack**: Go 1.24+, Docker SDK for Go
- **WDL version**: 1.2 only (no older version support)
- **Import**: Local files only (HTTP import deferred to v2)
- **Compatibility**: Spectool conformance suite as compliance benchmark

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| WDL 1.2 only | Scope reduction for reliable implementation | ✓ Good — clean implementation |
| v1 CLI only | Core engine first, server separation later | ✓ Good — usable end-to-end |
| Hand-written lexer (Rob Pike) | Full control over command block modes | ✓ Good — handles all WDL syntax |
| Pratt parser for expressions | Clean operator precedence handling | ✓ Good — all operators correct |
| ShellRunner mock for tests | No Docker dependency in unit tests | ✓ Good — fast CI-friendly tests |
| errgroup for scatter parallelism | Bounded concurrency with cancellation | ✓ Good — clean fail-fast behavior |
| Spectool for conformance | Official WDL compliance benchmark | ✓ Good — 45/160 baseline established |
| Separate binaries for server/daemon/client | Clean process separation, independent deployment | ✓ Good — clean architecture |
| Daemon polls server (not push) | Simpler scale-out, daemon only needs server address | ✓ Good — simple and reliable |
| Shared filesystem first, S3 later | Reduce scope, NFS ubiquitous in HPC environments | ✓ Good — ships fast |
| SQLite with WAL + modernc.org/sqlite | CGO-free, single-file DB, WAL for concurrent reads | ✓ Good — zero external dependencies |
| chi v5 for HTTP routing | Lightweight, idiomatic Go, middleware support | ✓ Good — clean handler code |
| Atomic task claiming (single UPDATE) | Avoid TOCTOU race in multi-daemon environments | ✓ Good — correct concurrency |
| runTaskFn injection for dispatch | Extension seam in scheduler for distributed execution | ✓ Good — minimal refactoring |
| ADR 0001: Remove heuristic file staging | Client cannot reliably detect File-typed inputs from JSON | ✓ Good — correct by construction |

---
*Last updated: 2026-03-12 after v1.1 milestone*
