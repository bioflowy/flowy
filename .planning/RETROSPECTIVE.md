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

## Cross-Milestone Trends

### Process Evolution

| Milestone | Commits | Phases | Key Change |
|-----------|---------|--------|------------|
| v1.0 | 160 | 8 | Initial pipeline — linear dependency chain |

### Cumulative Quality

| Milestone | Go LOC | Test Files | Spectool |
|-----------|--------|------------|----------|
| v1.0 | 17,648 | ~40 | 45/160 (28%) |

### Top Lessons (Verified Across Milestones)

1. Linear phase dependencies produce clean architecture but limit parallelization
2. Mock interfaces (ContainerRunner, ShellRunner) are essential for fast test cycles
