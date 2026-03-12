# Milestones

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
