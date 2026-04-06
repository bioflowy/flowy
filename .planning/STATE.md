---
gsd_state_version: 1.0
milestone: v2.0
milestone_name: Web UI — Management
status: unknown
stopped_at: "Completed 28-01-PLAN.md"
last_updated: "2026-04-06T08:36:40Z"
last_activity: 2026-04-06
progress:
  total_phases: 6
  completed_phases: 3
  total_plans: 10
  completed_plans: 8
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-20)

**Core value:** WDL 1.2で記述されたワークフローをDockerコンテナ上で正確に実行できること
**Current focus:** Phase 28 — project-management

## Current Position

Phase: 28 (project-management) — EXECUTING
Plan: 2 of 2

## Performance Metrics

**Velocity:**

- Total plans completed: 6 (v2.0)
- Average duration: — (no data yet)
- Total execution time: —

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 25-infrastructure-foundation P01 | 2 tasks | 4 min | 19 files |
| 25-infrastructure-foundation P03 | 3 tasks | 15 min | 4 files |
| 26-authentication P01 | 2 tasks | 7 min | 13 files |
| 26-authentication P02 | 2 tasks | 3 min | 5 files |
| 26-authentication P03 | 3 tasks | 5 min | 7 files |

*Updated after each plan completion*
| Phase 27 P01 | 4 | 2 tasks | 14 files |
| Phase 27 P02 | 20 | 2 tasks | 9 files |

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
- [Phase 25-03]: go:embed lives in web/embed.go (not cmd/flowy-ui) — Go forbids .. in embed paths; exports DistFS
- [Phase 25-03]: vite.config.ts base must be '/' not './' — relative base breaks asset URLs when served by Go http.FileServer at root
- [Phase 25-03]: SPA fallback uses pre-flight fs.Open check to avoid ResponseWriter-already-written problem
- [Phase 26-01]: jsdom v25 used instead of v29 due to top-level await ESM incompatibility with vitest worker pools
- [Phase 26-01]: Manual jest-dom matchers setup (expect.extend) instead of auto-setup import for ESM compatibility
- [Phase 26-01]: Test files excluded from tsconfig.app.json build to prevent noUnusedLocals errors
- [Phase 27]: shadcn CLI places files in @/ directory; move manually to src/components/ui/ (Phase 27 pattern consistent with Phase 26)
- [Phase 27]: PATCH added to CORS AllowedMethods to support new PATCH /users/{id}/reactivate route
- [Phase 27]: AlertDialog async confirm uses e.preventDefault() to block default close, then calls onOpenChange(false) after await
- [Phase 27]: Reactivate is inline (no confirmation dialog) — destructive actions require dialog, restorative do not
- [Phase 27]: Self-row protection in UsersPage based on username string comparison against authStore.username
- [Phase 28-01]: GET /users relaxed from admin-only to any-authenticated-user — required for Plan 02 Add Member dropdown
- [Phase 28-01]: No member_count on project cards — ProjectResponse has no such field; avoids N+1 queries
- [Phase 28-01]: slugManuallyEdited flag stops slug auto-generation once user directly edits slug field
- [Phase 28-01]: Zustand selector mock pattern: mockImplementation((selector) => selector(state)) for (s) => s.field usage

### Pending Todos

None.

### Blockers/Concerns

- [Phase 25] Directory naming: `web/` selected; must be used consistently in `//go:embed all:web/dist`, vite.config.ts, and Makefile.
- [Phase 26] JWT storage strategy must be finalized before any auth code is written — sessionStorage chosen as v2.0 default.
- [Phase 30] Three new backend endpoints needed: `GET /projects/{slug}/runs`, `GET /projects/{slug}/runs/{id}/files`, `GET /projects/{slug}/runs/{id}/files/download` — Store-layer design required during Phase 30 planning.

## Session Continuity

Last activity: 2026-04-06
Stopped at: Completed 28-01-PLAN.md
Resume file: .planning/phases/28-project-management/28-01-SUMMARY.md
