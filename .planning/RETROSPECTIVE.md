# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.0 — MVP

**Shipped:** 2026-03-11
**Phases:** 8 | **Plans:** 26 | **Commits:** 160

### What Was Built
- Complete WDL 1.2 interpreter pipeline: lexer → parser → type checker → evaluator → task runner → workflow scheduler
- CLI tools: `flowy run` and `flowy check` with full flag support
- Docker-based task execution with input staging and output collection
- DAG workflow scheduler with scatter parallelism and conditionals
- 35+ WDL standard library functions
- Spectool conformance integration (45/160 baseline)

### What Worked
- Linear phase dependency chain (each phase builds on previous) kept architecture clean
- ShellRunner mock pattern enabled fast unit testing without Docker
- Pratt parser for expressions was a good fit for WDL's operator precedence
- Rob Pike state-function lexer handled command block mode switching cleanly
- Spectool integration in final phase provided immediate feedback on spec compliance

### What Was Inefficient
- ROADMAP.md progress table got out of sync with actual phase completion (phases 3 and 5 showed "In Progress" despite being complete)
- Some SUMMARY.md files lack one_liner frontmatter, making automated extraction harder
- jsonToValueUntyped was duplicated between task and workflow packages instead of being shared

### Patterns Established
- Type-tagged values throughout (every Value carries its WDL Type)
- ContainerRunner interface for testability
- errgroup with SetLimit for bounded parallelism
- Rustc-style error reporting (file:line:col with source line and caret)

### Key Lessons
1. Go's `flag` package stops at first non-flag arg — positional args must come last in CLI commands
2. Spectool conformance testing is invaluable for catching edge cases (found 6 bugs in phase 8)
3. Hand-written lexer with state functions is more maintainable than parser generators for complex languages like WDL

### Cost Observations
- Model mix: ~20% opus (orchestration), ~80% sonnet (execution)
- Timeline: 2 days from init to milestone complete
- Notable: 8 phases, 26 plans executed with high parallelization within waves

---

## Milestone: v1.1 — Distributed Execution

**Shipped:** 2026-03-12
**Phases:** 5 | **Plans:** 10 | **Commits:** 64

### What Was Built
- Shared foundation: YAML config, HTTP DTO types, SQLite persistence (WAL, CGO-free), filesystem layout
- flowy-server: REST API with DAG dispatch to SQLite task queue, stale task detection
- flowy-daemon: poll-based task claiming, Docker execution, heartbeat, graceful shutdown
- flowy client CLI: submit/status/outputs subcommands, synchronous `flowy run` with polling
- Tech debt cleanup: consolidated ValueToInterface, documentation fixes

### What Worked
- TDD RED-GREEN pattern used consistently across all phases — caught bugs early
- runTaskFn injection seam from v1.0 made distributed dispatch a clean extension
- httptest.NewServer for integration tests — no real server needed
- Phase 13 (tech debt) as explicit milestone phase kept code quality high
- Milestone audit before completion caught 5 documentation/code inconsistencies

### What Was Inefficient
- STATE.md progress tracking fell behind (showed 0% despite all phases complete)
- Phase 12-03 took 30 min (3x longer than average) due to complex run command rewrite
- SUMMARY.md files still lack one_liner frontmatter — extraction requires reading full files
- Long-poll was planned but short-poll (5s ticker) was implemented instead — simpler but higher overhead

### Patterns Established
- Atomic task claiming via single `UPDATE WHERE state='PENDING'` — avoids TOCTOU races
- context.Background() for task execution goroutines — survives SIGTERM for graceful shutdown
- hostname-PID as daemon_id — simple, debuggable, no UUID generation needed
- ADR process for scope changes (ADR 0001: file staging removal)

### Key Lessons
1. Single-UPDATE atomic claiming is simpler and more correct than SELECT+UPDATE for distributed task queues
2. CGO-free SQLite (modernc.org/sqlite) works well — no cross-compilation issues
3. Milestone audit phase (Phase 13) is worth the investment — catches drift between docs and implementation
4. signal.NotifyContext as root context provides clean cancellation for all HTTP calls on Ctrl+C

### Cost Observations
- Model mix: ~20% opus (orchestration), ~80% sonnet (execution)
- Timeline: 2 days (matching v1.0 pace despite architecture complexity)
- Notable: 10 plans in 5 phases, ~74 min total execution time

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Commits | Phases | Key Change |
|-----------|---------|--------|------------|
| v1.0 | 160 | 8 | Initial pipeline — linear dependency chain |
| v1.1 | 64 | 5 | Distributed architecture — server/daemon/client split |

### Cumulative Quality

| Milestone | Go LOC | Test Files | Spectool |
|-----------|--------|------------|----------|
| v1.0 | 17,648 | ~40 | 45/160 (28%) |
| v1.1 | 25,433 | ~55 | 45/160 (28%) |

### Top Lessons (Verified Across Milestones)

1. Linear phase dependencies produce clean architecture but limit parallelization
2. Mock interfaces (ContainerRunner, ShellRunner) are essential for fast test cycles
3. TDD RED-GREEN pattern catches bugs early and produces well-tested code — verified in both v1.0 and v1.1
4. Milestone audit as explicit phase catches doc/code drift — established in v1.1, should continue
