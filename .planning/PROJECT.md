# flowy

## What This Is

WDL (Workflow Description Language) 1.2 workflow engine implemented in Go. Parses WDL 1.2 source files, type-checks them, and executes tasks via Docker containers with scatter parallelism and conditional support. Ships as a CLI tool (`flowy run`, `flowy check`) with spectool conformance testing.

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

### Active

- [ ] REST API server for workflow execution control
- [ ] Project-based user/workflow/input/output management
- [ ] SQLite persistence for projects, execution history, state
- [ ] CLI client that operates via REST API
- [ ] HTTP(S) import resolution
- [ ] Execution cache (task output reuse)
- [ ] Workflow resume from checkpoint
- [ ] Docker-alternative backends (Podman, Singularity)

### Out of Scope

- Object type — deprecated in WDL 1.2, use Struct instead
- WDL draft-2/1.0/1.1 support — different grammar, high maintenance cost
- WDL lint/zip features — execution engine is priority

## Context

Shipped v1.0 with 17,648 LOC Go across 204 files.
Tech stack: Go 1.24, Docker SDK for Go, hand-written lexer (Rob Pike pattern), Pratt expression parser.
Spectool conformance: 45/160 WDL 1.2 tests passing (28% baseline).
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

---
*Last updated: 2026-03-11 after v1.0 milestone*
