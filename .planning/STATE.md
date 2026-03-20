---
gsd_state_version: 1.0
milestone: v2.0
milestone_name: Web UI — Management
status: planning
stopped_at: Phase 25 context gathered
last_updated: "2026-03-20T06:12:11.190Z"
last_activity: 2026-03-20 — v2.0 roadmap created (6 phases, 26 requirements mapped)
progress:
  total_phases: 6
  completed_phases: 0
  total_plans: 0
  completed_plans: 0
  percent: 0
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-20)

**Core value:** WDL 1.2で記述されたワークフローをDockerコンテナ上で正確に実行できること
**Current focus:** Phase 25 — Infrastructure Foundation

## Current Position

Phase: 25 of 30 (Infrastructure Foundation)
Plan: — (not yet planned)
Status: Ready to plan
Last activity: 2026-03-20 — v2.0 roadmap created (6 phases, 26 requirements mapped)

Progress: [░░░░░░░░░░] 0%

## Performance Metrics

**Velocity:**

- Total plans completed: 0 (v2.0)
- Average duration: — (no data yet)
- Total execution time: —

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| - | - | - | - |

*Updated after each plan completion*

## Accumulated Context

### Decisions

Key decisions archived in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- v2.0 stack: React 19 + TypeScript 5.x + Vite 6 + Tailwind v4 + shadcn/ui (pinned for shadcn compatibility)
- v2.0: `web/` chosen as frontend workspace directory (consistent with STACK.md; resolves `web/` vs `ui/` gap)
- v2.0: JWT stored in sessionStorage for v2.0 MVP; httpOnly cookie deferred (acceptable for internal tool)
- v2.0: flowy-ui is pure static-file server + reverse proxy — no domain knowledge; flowy-server unchanged except CORS
- v2.0: Phase 30 (File Browsing) requires 3 new backend endpoints not yet in flowy-server store layer

### Pending Todos

None.

### Blockers/Concerns

- [Phase 25] Directory naming: `web/` selected; must be used consistently in `//go:embed all:web/dist`, vite.config.ts, and Makefile.
- [Phase 26] JWT storage strategy must be finalized before any auth code is written — sessionStorage chosen as v2.0 default.
- [Phase 30] Three new backend endpoints needed: `GET /projects/{slug}/runs`, `GET /projects/{slug}/runs/{id}/files`, `GET /projects/{slug}/runs/{id}/files/download` — Store-layer design required during Phase 30 planning.

## Session Continuity

Last activity: 2026-03-20
Stopped at: Phase 25 context gathered
Resume file: .planning/phases/25-infrastructure-foundation/25-CONTEXT.md
