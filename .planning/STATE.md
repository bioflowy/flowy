---
gsd_state_version: 1.0
milestone: v2.0
milestone_name: Web UI — Management
status: unknown
stopped_at: Completed 25-01-PLAN.md
last_updated: "2026-03-24T02:59:27.113Z"
last_activity: 2026-03-24
progress:
  total_phases: 6
  completed_phases: 0
  total_plans: 3
  completed_plans: 2
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-20)

**Core value:** WDL 1.2で記述されたワークフローをDockerコンテナ上で正確に実行できること
**Current focus:** Phase 25 — infrastructure-foundation

## Current Position

Phase: 25 (infrastructure-foundation) — EXECUTING
Plan: 2 of 3

## Performance Metrics

**Velocity:**

- Total plans completed: 2 (v2.0)
- Average duration: — (no data yet)
- Total execution time: —

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 25-infrastructure-foundation P01 | 2 tasks | 4 min | 19 files |

*Updated after each plan completion*

## Accumulated Context

### Decisions

Key decisions archived in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- v2.0 stack: React 19 + TypeScript 5.x + Vite 6 + Tailwind v4 + shadcn/ui (pinned for shadcn compatibility)
- v2.0: `web/` chosen as frontend workspace directory (consistent with STACK.md; resolves `web/` vs `ui/` gap)
- v2.0: JWT stored in sessionStorage for v2.0 MVP; httpOnly cookie deferred (acceptable for internal tool)
- v2.0: flowy-ui is pure static-file server + reverse proxy — no domain knowledge; flowy-server unchanged except CORS
- 25-02: CORS disabled by default; enabled via cors.enabled: true; middleware at top-level router before JWT groups
- 25-02: AllowCredentials=false for v2.0 (sessionStorage JWT, not cookies)
- v2.0: Phase 30 (File Browsing) requires 3 new backend endpoints not yet in flowy-server store layer
- [Phase 25-01]: pnpm create vite@6 is interactive-only; project scaffolded manually with exact Vite react-ts template file contents
- [Phase 25-01]: AppShell and Sidebar created in Task 1 commit; App.tsx requires layout components to compile so they are inseparable

### Pending Todos

None.

### Blockers/Concerns

- [Phase 25] Directory naming: `web/` selected; must be used consistently in `//go:embed all:web/dist`, vite.config.ts, and Makefile.
- [Phase 26] JWT storage strategy must be finalized before any auth code is written — sessionStorage chosen as v2.0 default.
- [Phase 30] Three new backend endpoints needed: `GET /projects/{slug}/runs`, `GET /projects/{slug}/runs/{id}/files`, `GET /projects/{slug}/runs/{id}/files/download` — Store-layer design required during Phase 30 planning.

## Session Continuity

Last activity: 2026-03-24
Stopped at: Completed 25-01-PLAN.md
Resume file: None
