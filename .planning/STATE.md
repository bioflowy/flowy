---
gsd_state_version: 1.0
milestone: v1.3
milestone_name: Project & Registry
status: complete
stopped_at: Completed 21-03-PLAN.md (registry tag/list/delete handlers)
last_updated: "2026-03-20T01:56:28Z"
progress:
  total_phases: 5
  completed_phases: 3
  total_plans: 9
  completed_plans: 9
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-17)

**Core value:** WDL 1.2で記述されたワークフローをDockerコンテナ上で正確に実行できること
**Current focus:** Phase 21 — wdl-registry

## Current Position

Phase: 21 (wdl-registry) — COMPLETE
Plan: 3 of 3 (all plans complete)

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
- [Phase 21-wdl-registry]: PushRegistryVersion uses writer connection for both SELECT and INSERT to avoid TOCTOU races
- [Phase 21-wdl-registry]: Registry entry kind is immutable after first push; re-push with different kind returns error mapped to 409 by handler
- [Phase 21-02]: Kind mismatch detection uses strings.Contains(err.Error(), "kind") — matches PushRegistryVersion error format
- [Phase 21-02]: Stub handlers (returning 501) wired in Plan 02 so server compiles before Plan 03 implements list/versions/delete/tag
- [Phase 21-03]: Forbidden tests require separate admin+executor users — createRegistryTestProject always makes the user project admin, so addProjectMember helper added for executor role setup

### Pending Todos

None.

### Blockers/Concerns

- [Phase 19] Admin seed decision: POST /users is admin-only (chicken-and-egg). Must decide between migration-seeded default admin or --init-admin CLI flag before implementation.
- [Phase 21] Registry schema canonical version: three-table schema (registry_entries/registry_versions/registry_tags) chosen; must be finalized before migrations are written.

## Session Continuity

Last session: 2026-03-20T01:56:28Z
Stopped at: Completed 21-03-PLAN.md (registry tag/list/delete handlers — phase 21 complete)
Resume file: None
