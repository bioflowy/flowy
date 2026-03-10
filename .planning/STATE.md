---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
status: planning
stopped_at: Completed 02-lexer-parser-and-ast-02-04-PLAN.md
last_updated: "2026-03-10T07:13:06.479Z"
last_activity: 2026-03-10 — Roadmap created; all 57 v1 requirements mapped to 6 phases
progress:
  total_phases: 7
  completed_phases: 1
  total_plans: 6
  completed_plans: 5
  percent: 0
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-10)

**Core value:** WDL 1.2で記述されたワークフローをDockerコンテナ上で正確に実行できること
**Current focus:** Phase 1 — Type Data Structures

## Current Position

Phase: 1 of 6 (Lexer, Parser, and AST)
Plan: 0 of TBD in current phase
Status: Ready to plan
Last activity: 2026-03-10 — Roadmap created; all 57 v1 requirements mapped to 6 phases

Progress: [░░░░░░░░░░] 0%

## Performance Metrics

**Velocity:**
- Total plans completed: 0
- Average duration: -
- Total execution time: 0 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| - | - | - | - |

**Recent Trend:**
- Last 5 plans: -
- Trend: -

*Updated after each plan completion*
| Phase 01-type-data-structures P01 | 15min | 2 tasks | 5 files |
| Phase 02-lexer-parser-and-ast P01 | 3min | 2 tasks | 7 files |
| Phase 02-lexer-parser-and-ast P02 | 3min | 2 tasks | 3 files |
| Phase 02-lexer-parser-and-ast P03 | 8min | 2 tasks | 4 files |
| Phase 02-lexer-parser-and-ast P04 | 9min | 2 tasks | 4 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- [Init]: Hand-written lexer/parser (Rob Pike state-function pattern + Pratt) — no viable WDL 1.2 grammar for Go parser generators
- [Init]: Docker moby client v28.x behind ContainerRunner interface — enables unit testing without live daemon
- [Init]: golang.org/x/sync/errgroup with SetLimit for scatter parallelism — prevents goroutine leaks
- [Phase 01-type-data-structures]: AnyType{Optional:true} serves as None literal type — TypeName()=='None', IsOptional()==true — no separate NoneType struct needed
- [Phase 01-type-data-structures]: Value structs embed concrete type fields (not interface) so empty ArrayValue distinguishes Array[Int] from Array[String]
- [Phase 01-type-data-structures]: StructType field named 'Name' not 'TypeName' to prevent Go interface method collision
- [Phase 02-lexer-parser-and-ast]: TokenType = int alias in wdl/ast/expr.go avoids circular import between ast and lexer packages
- [Phase 02-lexer-parser-and-ast]: Pos embedded as plain struct field (not pointer) in every AST node for zero-allocation direct field access
- [Phase 02-lexer-parser-and-ast]: WorkflowNode, StringPart, Expr use unexported marker methods to prevent unintended external implementations
- [Phase 02-lexer-parser-and-ast]: Pull-model Next() lexer API with auto command-mode detection after 'command' keyword — parser does not need SetCommandMode call
- [Phase 02-lexer-parser-and-ast]: TokXxx exported naming for all token constants enables external lexer_test package to reference them directly
- [Phase 02-lexer-parser-and-ast]: parseExpr stub propagates TokError as ParseError; expectName() accepts type-keywords as struct/task/workflow names
- [Phase 02-lexer-parser-and-ast]: Workflow input/output sections parsed directly in parseWorkflow loop; body-only elements use parseWorkflowBodyElement
- [Phase 02-lexer-parser-and-ast]: ** right-associativity: rightBP=59 < leftBP=60 with loop condition leftBP<=minBP — unambiguously right-associative for 2**3**2→2**(3**2)
- [Phase 02-lexer-parser-and-ast]: Unary minus/not rightBP=55 (between */ at 50 and ** at 60): -x**2→-(x**2) matching Python/math convention
- [Phase 02-lexer-parser-and-ast]: isNameToken expanded with TokEnv — 'env' keyword valid as variable reference in expression context (needed for ${env} in command blocks)

### Pending Todos

None yet.

### Blockers/Concerns

- [Research]: Phase 5 DAG scheduler needs deeper review of miniwdl runtime/workflow.py StateMachine before planning — scatter-in-conditional nesting is complex
- [Research]: ContainerRunner interface exact signature (env vars, resource limits, timeout fields) should be finalized before Phase 4 planning begins
- [Research]: errgroup vs. channel-based scheduler pattern interaction needs resolution during Phase 5 planning

## Session Continuity

Last session: 2026-03-10T07:13:06.475Z
Stopped at: Completed 02-lexer-parser-and-ast-02-04-PLAN.md
Resume file: None
