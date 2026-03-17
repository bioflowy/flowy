---
gsd_state_version: 1.0
milestone: v1.3
milestone_name: Project & Registry
status: planning
stopped_at: Completed 20-03-PLAN.md
last_updated: "2026-03-17T19:33:41.725Z"
last_activity: 2026-03-17 — v1.3 roadmap created; phases 19-23 defined
progress:
  total_phases: 5
  completed_phases: 2
  total_plans: 6
  completed_plans: 6
  percent: 0
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-17)

**Core value:** WDL 1.2で記述されたワークフローをDockerコンテナ上で正確に実行できること
**Current focus:** Phase 19 — User Authentication

## Current Position

Phase: 19 of 23 (User Authentication)
Plan: 0 of TBD in current phase
Status: Ready to plan
Last activity: 2026-03-17 — v1.3 roadmap created; phases 19-23 defined

Progress: [░░░░░░░░░░] 0% (0/5 v1.3 phases complete)

## Accumulated Context

### Decisions

Key decisions archived in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- v1.3: golang-jwt/jwt/v5 for JWT (zero deps, HS256); bcrypt cost 12 (~300ms)
- v1.3: Hand-rolled ~50-line RBAC middleware (no Casbin); three static roles admin/executor/viewer
- v1.3: WDL source stored as TEXT in SQLite — registry works in local-only deployments
- v1.3: Three-table registry schema (registry_entries/registry_versions/registry_tags) per Docker distribution model
- v1.3: auth.enabled defaults false — all 156 spectool tests continue to pass unchanged
- v1.3: Daemon routes in separate sub-router — must be structured before any JWT middleware is added
- [Phase 19]: AuthConfig defaults Enabled=false (bool zero value) — no applyDefaults change needed; spectool tests pass unchanged
- [Phase 19]: GetUserByUsername filters is_active=1; GetUserByID returns regardless of active state
- [Phase 19]: InitAdminStore defined as minimal interface for testability; storeAdapter bridges store.Store in production
- [Phase 19]: bearerTransport clones request before mutation; daemon.New() uses plain transport when token is empty
- [Phase 19]: GetUserByUsernameAny added to Store — login distinguishes inactive vs not-found without username enumeration
- [Phase 19]: UpdateUser added to Store interface for password change support
- [Phase 20-projects-and-rbac]: project_id on runs nullable in SQL, enforced at app layer; RemoveMemberIfNotLastAdmin uses writer connection for count+delete serialization
- [Phase 20-projects-and-rbac]: /projects route group has its own JWTMiddleware invocation; handleCreateProject reads claims directly since no project slug is known yet
- [Phase 20-projects-and-rbac]: projectIDFromContext returns empty string when no project in context — same handleSubmit serves both /workflows and project-scoped routes
- [Phase 20-projects-and-rbac]: /workflows routes removed from auth-enabled branch; project-scoped /runs routes are the auth-enabled replacement

### Pending Todos

None.

### Blockers/Concerns

- [Phase 19] Admin seed decision: POST /users is admin-only (chicken-and-egg). Must decide between migration-seeded default admin or --init-admin CLI flag before implementation.
- [Phase 21] Registry schema canonical version: three-table schema (registry_entries/registry_versions/registry_tags) chosen; must be finalized before migrations are written.

## Session Continuity

Last session: 2026-03-17T19:33:41.721Z
Stopped at: Completed 20-03-PLAN.md
Resume file: None
