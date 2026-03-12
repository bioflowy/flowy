# Roadmap: flowy

## Overview

flowy is a WDL 1.2 workflow engine in Go. v1.0 shipped as a CLI tool with full parse → typecheck → evaluate → execute pipeline. Spectool conformance baseline: 45/160. v1.1 shipped distributed server/daemon/client architecture for multi-host workflow execution.

## Milestones

- ✅ **v1.0 MVP** — Phases 1-8 (shipped 2026-03-11)
- ✅ **v1.1 Distributed Execution** — Phases 9-13 (shipped 2026-03-12)

## Phases

<details>
<summary>✅ v1.0 MVP (Phases 1-8) — SHIPPED 2026-03-11</summary>

- [x] Phase 1: Type Data Structures (1/1 plans) — completed 2026-03-10
- [x] Phase 2: Lexer, Parser, and AST (5/5 plans) — completed 2026-03-10
- [x] Phase 3: Type Checker (4/4 plans) — completed 2026-03-10
- [x] Phase 4: Expression Evaluator and Standard Library (6/6 plans) — completed 2026-03-10
- [x] Phase 5: Task Execution (3/3 plans) — completed 2026-03-10
- [x] Phase 6: Workflow Execution, DAG Scheduler (3/3 plans) — completed 2026-03-10
- [x] Phase 7: CLI and Integration (2/2 plans) — completed 2026-03-11
- [x] Phase 8: Specification Compliance (2/2 plans) — completed 2026-03-11

Full details: `.planning/milestones/v1.0-ROADMAP.md`

</details>

<details>
<summary>✅ v1.1 Distributed Execution (Phases 9-13) — SHIPPED 2026-03-12</summary>

- [x] Phase 9: API Contract + Persistence Foundation (2/2 plans) — completed 2026-03-11
- [x] Phase 10: Server Binary + DAG Dispatch (2/2 plans) — completed 2026-03-11
- [x] Phase 11: Daemon Binary + Task Execution (2/2 plans) — completed 2026-03-12
- [x] Phase 12: Client CLI + Spectool Compatibility (3/3 plans) — completed 2026-03-12
- [x] Phase 13: Tech Debt Cleanup (1/1 plan) — completed 2026-03-12

Full details: `.planning/milestones/v1.1-ROADMAP.md`

</details>

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Type Data Structures | v1.0 | 1/1 | Complete | 2026-03-10 |
| 2. Lexer, Parser, and AST | v1.0 | 5/5 | Complete | 2026-03-10 |
| 3. Type Checker | v1.0 | 4/4 | Complete | 2026-03-10 |
| 4. Expression Evaluator and Standard Library | v1.0 | 6/6 | Complete | 2026-03-10 |
| 5. Task Execution | v1.0 | 3/3 | Complete | 2026-03-10 |
| 6. Workflow Execution (DAG Scheduler) | v1.0 | 3/3 | Complete | 2026-03-10 |
| 7. CLI and Integration | v1.0 | 2/2 | Complete | 2026-03-11 |
| 8. Specification Compliance | v1.0 | 2/2 | Complete | 2026-03-11 |
| 9. API Contract + Persistence Foundation | v1.1 | 2/2 | Complete | 2026-03-11 |
| 10. Server Binary + DAG Dispatch | v1.1 | 2/2 | Complete | 2026-03-11 |
| 11. Daemon Binary + Task Execution | v1.1 | 2/2 | Complete | 2026-03-12 |
| 12. Client CLI + Spectool Compatibility | v1.1 | 3/3 | Complete | 2026-03-12 |
| 13. Tech Debt Cleanup | v1.1 | 1/1 | Complete | 2026-03-12 |
