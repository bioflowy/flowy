# Roadmap: flowy

## Overview

flowy is a WDL 1.2 workflow engine in Go. v1.0 shipped as a CLI tool with full parse → typecheck → evaluate → execute pipeline. Spectool conformance baseline: 45/160. v1.1 shipped distributed server/daemon/client architecture for multi-host workflow execution. v1.2 shipped S3/MinIO object storage as an alternative backend to shared filesystem, enabling cloud-native WDL execution. Spectool conformance: 156/156. v1.3 shipped multi-user authentication, project-scoped RBAC, a Docker-registry-style WDL content-addressed registry, registry-based workflow execution, and run provenance tracking. v2.0 adds a browser-based management SPA embedded in a new `flowy-ui` binary, covering authentication, user/project/registry management, and file browsing with download.

## Milestones

- ✅ **v1.0 MVP** — Phases 1-8 (shipped 2026-03-11)
- ✅ **v1.1 Distributed Execution** — Phases 9-13 (shipped 2026-03-12)
- ✅ **v1.2 S3 Object Storage** — Phases 14-18 (shipped 2026-03-13)
- ✅ **v1.3 Project & Registry** — Phases 19-24 (shipped 2026-03-20)
- 🚧 **v2.0 Web UI — Management** — Phases 25-30 (in progress)

## Phases

<details>
<summary>✅ v1.0 MVP (Phases 1-8) — SHIPPED 2026-03-11</summary>

- [x] **Phase 1: Type Data Structures** (1/1 plans) — completed 2026-03-10
- [x] **Phase 2: Lexer, Parser, and AST** (5/5 plans) — completed 2026-03-10
- [x] **Phase 3: Type Checker** (4/4 plans) — completed 2026-03-10
- [x] **Phase 4: Expression Evaluator and Standard Library** (6/6 plans) — completed 2026-03-10
- [x] **Phase 5: Task Execution** (3/3 plans) — completed 2026-03-10
- [x] **Phase 6: Workflow Execution, DAG Scheduler** (3/3 plans) — completed 2026-03-10
- [x] **Phase 7: CLI and Integration** (2/2 plans) — completed 2026-03-11
- [x] **Phase 8: Specification Compliance** (2/2 plans) — completed 2026-03-11

Full details: `.planning/milestones/v1.0-ROADMAP.md`

</details>

<details>
<summary>✅ v1.1 Distributed Execution (Phases 9-13) — SHIPPED 2026-03-12</summary>

- [x] **Phase 9: API Contract + Persistence Foundation** (2/2 plans) — completed 2026-03-11
- [x] **Phase 10: Server Binary + DAG Dispatch** (2/2 plans) — completed 2026-03-11
- [x] **Phase 11: Daemon Binary + Task Execution** (2/2 plans) — completed 2026-03-12
- [x] **Phase 12: Client CLI + Spectool Compatibility** (3/3 plans) — completed 2026-03-12
- [x] **Phase 13: Tech Debt Cleanup** (1/1 plan) — completed 2026-03-12

Full details: `.planning/milestones/v1.1-ROADMAP.md`

</details>

<details>
<summary>✅ v1.2 S3 Object Storage (Phases 14-18) — SHIPPED 2026-03-13</summary>

- [x] **Phase 14: Storage Abstraction** (2/2 plans) — completed 2026-03-13
- [x] **Phase 15: Configuration and Path Resolution** (2/2 plans) — completed 2026-03-13
- [x] **Phase 16: S3Backend Implementation** (2/2 plans) — completed 2026-03-13
- [x] **Phase 17: Daemon Staging** (2/2 plans) — completed 2026-03-13
- [x] **Phase 18: MinIO Integration Testing** (2/2 plans) — completed 2026-03-13

Full details: `.planning/milestones/v1.2-ROADMAP.md`

</details>

<details>
<summary>✅ v1.3 Project & Registry (Phases 19-24) — SHIPPED 2026-03-20</summary>

- [x] **Phase 19: User Authentication** (3/3 plans) — completed 2026-03-17
- [x] **Phase 20: Projects and RBAC** (3/3 plans) — completed 2026-03-17
- [x] **Phase 21: WDL Registry** (3/3 plans) — completed 2026-03-20
- [x] **Phase 22: Registry-Based Execution** (2/2 plans) — completed 2026-03-20
- [x] **Phase 23: Provenance Tracking** (2/2 plans) — completed 2026-03-20
- [x] **Phase 24: Fix CLI Polling for Project-Scoped Runs** (1/1 plan) — completed 2026-03-20

Full details: `.planning/milestones/v1.3-ROADMAP.md`

</details>

### 🚧 v2.0 Web UI — Management (In Progress)

**Milestone Goal:** Provide a browser-based management interface for flowy covering authentication, user/project/registry management, and file browsing with download, served from a new `flowy-ui` binary embedding the React SPA.

- [x] **Phase 25: Infrastructure Foundation** — flowy-ui binary with go:embed SPA serving, Vite+React+TypeScript scaffold, build pipeline (completed 2026-03-24)
- [x] **Phase 26: Authentication** — Login screen, JWT session management, CORS support, protected route gating (completed 2026-04-06)
- [x] **Phase 27: User Management** — Admin user list, create, deactivate/reactivate, password reset (completed 2026-04-06)
- [x] **Phase 28: Project Management** — Project list/create, project detail, member management with RBAC roles (completed 2026-04-06)
- [x] **Phase 29: Registry Browsing** — Registry entry list, version history, WDL source viewer, tag management (completed 2026-04-07)
- [x] **Phase 30: File Browsing** — Run list, output file list, file download (S3/local), inline text preview (completed 2026-04-07)

## Phase Details

### Phase 25: Infrastructure Foundation
**Goal**: The flowy-ui binary exists, serves the embedded React SPA with correct SPA routing fallback, and the build pipeline produces a single deployable binary from one make target.
**Depends on**: Nothing (first v2.0 phase)
**Requirements**: INFRA-01, INFRA-02, INFRA-03, INFRA-04, INFRA-05
**Success Criteria** (what must be TRUE):
  1. Running `make build-ui` produces a `flowy-ui` binary that serves the React SPA on a configured port without errors.
  2. Pressing F5 (hard refresh) on any SPA route (e.g., `/projects`) returns the React app, not a JSON 404.
  3. All SPA assets load correctly in the browser — no 404s for JS/CSS files from the embedded FS.
  4. `flowy-ui --api-addr http://host:8124` proxies `/api/*` requests to the specified flowy-server address.
  5. flowy-server accepts requests from the Vite dev server origin when `cors.enabled: true` is set in config.
**Plans**: 3 plans
Plans:
- [ ] 25-01-PLAN.md — Vite + React + TypeScript scaffold with app shell layout
- [ ] 25-02-PLAN.md — CORS middleware on flowy-server
- [ ] 25-03-PLAN.md — flowy-ui Go binary, reverse proxy, and Makefile build-ui target

### Phase 26: Authentication
**Goal**: Users can log in with username and password, maintain a JWT session across the app, and are automatically redirected to login when their session expires or they access protected routes unauthenticated.
**Depends on**: Phase 25
**Requirements**: AUTH-01, AUTH-02, AUTH-03, AUTH-04, AUTH-05
**Success Criteria** (what must be TRUE):
  1. User submits username and password on the login screen and is redirected to the application on success; invalid credentials show an error message.
  2. All API requests include the JWT token automatically — the user never manually sets an Authorization header.
  3. Navigating to any protected route without a session redirects to the login screen; after login the user is returned to the originally requested URL.
  4. When a 401 response is received, the session is cleared and the user is redirected to login without manual action.
  5. User can change their own password via an account settings action and can log out from any page.
**Plans**: 3 plans
Plans:
- [ ] 26-01-PLAN.md — Test infra, shadcn/ui components, Zustand auth store, API client with token injection
- [ ] 26-02-PLAN.md — Login page, ProtectedRoute, App.tsx route wiring with 401 interception
- [ ] 26-03-PLAN.md — Account page, password change dialog, logout hook, sidebar user display

### Phase 27: User Management
**Goal**: Admins can view and manage all user accounts — creating new users, deactivating/reactivating existing accounts, and resetting passwords — via a dedicated users screen.
**Depends on**: Phase 26
**Requirements**: USER-01, USER-02, USER-03, USER-04
**Success Criteria** (what must be TRUE):
  1. Admin navigates to the users screen and sees a table of all registered users with their active/inactive status.
  2. Admin creates a new user by filling in a form; the new user appears in the table immediately.
  3. Admin deactivates an active user (with confirmation dialog); the user's status changes to inactive in the table.
  4. Admin reactivates an inactive user; the user's status returns to active.
  5. Admin resets another user's password; the affected user can log in with the new password.
**Plans**: 2 plans
Plans:
- [ ] 27-01-PLAN.md — Backend reactivate/reset-password endpoints, apiClient extensions, shadcn/ui components, React Query hooks
- [ ] 27-02-PLAN.md — UsersPage table with dialogs, sidebar admin gating, route wiring

### Phase 28: Project Management
**Goal**: Users can view the projects they belong to, create new projects, and admins can manage project membership and role assignments through a project detail screen.
**Depends on**: Phase 26
**Requirements**: PROJ-01, PROJ-02, PROJ-03, PROJ-04
**Success Criteria** (what must be TRUE):
  1. User navigates to projects and sees a list of only the projects they are a member of.
  2. User creates a new project by entering a name and slug; the project appears in the list immediately.
  3. Project detail screen shows a summary of the project including member list with role badges.
  4. Admin adds a member to a project with a selected role; the new member appears in the member list.
  5. Admin changes a member's role or removes a member from a project; the member list reflects the change.
**Plans**: 2 plans
Plans:
- [ ] 28-01-PLAN.md — Project data layer, ProjectsPage card grid, CreateProjectPage with slug auto-generation
- [ ] 28-02-PLAN.md — ProjectDetailPage with member management, sidebar refactor with project dropdown

### Phase 29: Registry Browsing
**Goal**: Users can browse registry entries within a project, view version history and WDL source code, and admins can manage tags on registry entries.
**Depends on**: Phase 28
**Requirements**: REG-01, REG-02, REG-03, REG-04
**Success Criteria** (what must be TRUE):
  1. User navigates to a project's registry and sees a list of entries with kind badges (tool/workflow) and associated tags.
  2. User selects a registry entry and sees its full version history with digest identifiers.
  3. User selects a version and sees the WDL source code rendered in a code viewer.
  4. Admin creates a new tag on a registry entry version; the tag appears on the entry in the list.
  5. Admin deletes or moves a tag; the change is reflected in the entry detail immediately.
**Plans**: 2 plans
Plans:
- [ ] 29-01-PLAN.md — Registry data layer (hooks/types) and entry list page with admin delete
- [ ] 29-02-PLAN.md — Entry detail page with version history, WDL source viewer, and tag management

### Phase 30: File Browsing
**Goal**: Users can view workflow runs for a project, browse output files for completed runs, and download files directly from S3 or local storage through the browser.
**Depends on**: Phase 28
**Requirements**: FILE-01, FILE-02, FILE-03, FILE-04
**Success Criteria** (what must be TRUE):
  1. User navigates to a project's runs list and sees all workflow runs with status badges and timestamps.
  2. User selects a completed run and sees its output files listed by path.
  3. User clicks a download button for an output file and the browser downloads the file (works for both S3 and local storage backends).
  4. User can preview the contents of small text output files inline without downloading.
**Plans**: 2 plans
Plans:
- [ ] 30-01-PLAN.md — Backend endpoints: ListRunsByProject store method, handleListRuns and handleFileDownload handlers
- [ ] 30-02-PLAN.md — Frontend: RunsPage, RunDetailPage with download and inline preview, route wiring

### Phase 31: Audit Gap Closure
**Goal**: Fix cross-phase integration gaps found in v2.0 milestone audit — broken sidebar Members route, missing admin guard on GET /users, and auth-disabled mode login hang.
**Depends on**: Phase 28, Phase 27, Phase 26
**Requirements**: PROJ-03, PROJ-04, USER-01, AUTH-01
**Gap Closure**: Closes gaps from v2.0 audit
**Success Criteria** (what must be TRUE):
  1. Clicking sidebar "Members" link navigates to the project detail page showing the members table (not the wildcard fallback).
  2. Non-admin authenticated user calling GET /api/users receives 403 Forbidden.
  3. When auth is disabled on the backend, the login page redirects to / instead of hanging.
  4. REQUIREMENTS.md traceability checkboxes for INFRA-01, INFRA-02, INFRA-05 are marked complete.
**Plans**: 2 plans
Plans:
- [ ] 31-01-PLAN.md — Fix sidebar Members link and auth-disabled login redirect
- [x] 31-02-PLAN.md — Restore admin guard on GET /users with project-scoped user list

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
| 14. Storage Abstraction | v1.2 | 2/2 | Complete | 2026-03-13 |
| 15. Configuration and Path Resolution | v1.2 | 2/2 | Complete | 2026-03-13 |
| 16. S3Backend Implementation | v1.2 | 2/2 | Complete | 2026-03-13 |
| 17. Daemon Staging | v1.2 | 2/2 | Complete | 2026-03-13 |
| 18. MinIO Integration Testing | v1.2 | 2/2 | Complete | 2026-03-13 |
| 19. User Authentication | v1.3 | 3/3 | Complete | 2026-03-17 |
| 20. Projects and RBAC | v1.3 | 3/3 | Complete | 2026-03-17 |
| 21. WDL Registry | v1.3 | 3/3 | Complete | 2026-03-20 |
| 22. Registry-Based Execution | v1.3 | 2/2 | Complete | 2026-03-20 |
| 23. Provenance Tracking | v1.3 | 2/2 | Complete | 2026-03-20 |
| 24. Fix CLI Polling for Project-Scoped Runs | v1.3 | 1/1 | Complete | 2026-03-20 |
| 25. Infrastructure Foundation | 3/3 | Complete    | 2026-03-24 | - |
| 26. Authentication | 3/3 | Complete    | 2026-04-06 | - |
| 27. User Management | 2/2 | Complete    | 2026-04-06 | - |
| 28. Project Management | 2/2 | Complete    | 2026-04-06 | - |
| 29. Registry Browsing | 2/2 | Complete    | 2026-04-07 | - |
| 30. File Browsing | 2/2 | Complete    | 2026-04-07 | - |
| 31. Audit Gap Closure | 2/2 | Complete   | 2026-04-07 | 2026-04-07 |
