# Roadmap: flowy

## Overview

flowy is a WDL 1.2 workflow engine in Go. v1.0 shipped as a CLI tool with full parse → typecheck → evaluate → execute pipeline. Spectool conformance baseline: 45/160. v1.1 shipped distributed server/daemon/client architecture for multi-host workflow execution. v1.2 shipped S3/MinIO object storage as an alternative backend to shared filesystem, enabling cloud-native WDL execution. Spectool conformance: 156/156. v1.3 adds multi-user authentication, project-scoped RBAC, a Docker-registry-style WDL content-addressed registry, registry-based workflow execution, and run provenance tracking.

## Milestones

- ✅ **v1.0 MVP** — Phases 1-8 (shipped 2026-03-11)
- ✅ **v1.1 Distributed Execution** — Phases 9-13 (shipped 2026-03-12)
- ✅ **v1.2 S3 Object Storage** — Phases 14-18 (shipped 2026-03-13)
- 🚧 **v1.3 Project & Registry** — Phases 19-23 (in progress)

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

### 🚧 v1.3 Project & Registry (In Progress)

**Milestone Goal:** Add multi-user authentication, project-scoped RBAC, a content-addressed WDL registry, registry-based execution, and run provenance tracking.

- [x] **Phase 19: User Authentication** — Users can log in with username/password and receive JWT tokens; all routes protected; admin seeds on first run (completed 2026-03-17)
- [ ] **Phase 20: Projects and RBAC** — Users can create projects, manage members, and access control enforces admin/executor/viewer roles
- [ ] **Phase 21: WDL Registry** — Executors can push WDL files by content-hash digest and pull by tag or digest within a project
- [ ] **Phase 22: Registry-Based Execution** — Executors can submit workflow runs using registry references instead of file paths
- [ ] **Phase 23: Provenance Tracking** — System records and exposes the registry version, inputs, and submitting user for every run

## Phase Details

### Phase 19: User Authentication
**Goal**: Users can securely authenticate with the server and all non-public routes are protected by JWT tokens
**Depends on**: Phase 18 (v1.2 complete)
**Requirements**: AUTH-01, AUTH-02, AUTH-03, AUTH-04, AUTH-05, AUTH-06
**Success Criteria** (what must be TRUE):
  1. User can POST to `/auth/login` with username and password and receive a signed JWT token
  2. Requests to protected routes without a valid JWT token return 401 Unauthorized
  3. On first server start, an initial admin user exists and can log in without any setup step
  4. Admin can create new user accounts and list or delete existing users
  5. When `auth.enabled: false` is set in config, all existing spectool conformance tests pass unchanged
**Plans:** 3/3 plans complete
Plans:
- [ ] 19-01-PLAN.md — Config, migration, store layer foundation (User model, server_config)
- [ ] 19-02-PLAN.md — Auth package, middleware, handlers, and server router restructure
- [ ] 19-03-PLAN.md — CLI --init-admin command and daemon bearer token injection

### Phase 20: Projects and RBAC
**Goal**: Users can create isolated project workspaces and control who can do what via role-based membership
**Depends on**: Phase 19
**Requirements**: PROJ-01, PROJ-02, PROJ-03, PROJ-04, PROJ-05, PROJ-06, PROJ-07
**Success Criteria** (what must be TRUE):
  1. User can create a project, then list only projects they are a member of (not all projects on the server)
  2. Project admin can add a member with a specific role and change or remove that membership
  3. A viewer-role member can read project details but cannot update or delete the project
  4. Workflow runs submitted within a project context are associated with that project (project_id present on run records)
**Plans**: TBD

### Phase 21: WDL Registry
**Goal**: Executors can push versioned WDL tool and workflow files to a project registry and retrieve them by tag or immutable digest
**Depends on**: Phase 20
**Requirements**: REG-01, REG-02, REG-03, REG-04, REG-05, REG-06
**Success Criteria** (what must be TRUE):
  1. Executor can push a WDL file to a project registry and the response includes an auto-computed SHA-256 digest
  2. Executor can assign a human-readable tag to a registry version and later reassign that tag to a different version
  3. Viewer can retrieve the exact WDL content by tag (`name:tag`) or by digest (`name@sha256:hex`)
  4. Viewer can list all registry entries in a project and see their versions and tags
  5. Registry entries are automatically classified as tool (WDL task) or workflow (WDL workflow); project admin can delete entries
**Plans**: TBD

### Phase 22: Registry-Based Execution
**Goal**: Executors can submit workflow runs using a registry reference instead of a file path, and the server resolves the WDL content automatically
**Depends on**: Phase 21
**Requirements**: EXEC-01, EXEC-02
**Success Criteria** (what must be TRUE):
  1. Executor can submit a run using a registry reference (`project/name:tag` or `project/name@sha256:hex`) and the run starts successfully
  2. Submitting via registry reference and submitting via direct WDL path both produce identical run behavior; neither mode breaks the other
**Plans**: TBD

### Phase 23: Provenance Tracking
**Goal**: The system records a provenance record for every run linking it to its registry version, inputs, and submitting user, and viewers can query this audit trail
**Depends on**: Phase 22
**Requirements**: PROV-01, PROV-02, PROV-03, PROV-04
**Success Criteria** (what must be TRUE):
  1. After submitting a registry-based run, a provenance record exists immediately containing the registry version digest, inputs, and submitting user
  2. When the run completes, the provenance record is updated with the run outputs
  3. Viewer can list all provenance records for a project
  4. Viewer can retrieve a single provenance record by run ID showing version, digest, inputs, outputs, and submitting user
**Plans**: TBD

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
| 19. User Authentication | 3/3 | Complete    | 2026-03-17 | - |
| 20. Projects and RBAC | v1.3 | 0/TBD | Not started | - |
| 21. WDL Registry | v1.3 | 0/TBD | Not started | - |
| 22. Registry-Based Execution | v1.3 | 0/TBD | Not started | - |
| 23. Provenance Tracking | v1.3 | 0/TBD | Not started | - |
