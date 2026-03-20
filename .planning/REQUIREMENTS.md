# Requirements: flowy

**Defined:** 2026-03-17
**Core Value:** WDL 1.2で記述されたワークフローをDockerコンテナ上で正確に実行できること

## v1.3 Requirements

Requirements for Project & Registry milestone. Each maps to roadmap phases.

### Authentication

- [x] **AUTH-01**: User can log in with username and password, receiving a JWT token
- [x] **AUTH-02**: All API routes (except login and health) are protected by JWT middleware
- [x] **AUTH-03**: Server seeds an initial admin user on first run
- [x] **AUTH-04**: Admin can create new users with username and password
- [x] **AUTH-05**: Admin can list and delete users
- [x] **AUTH-06**: Auth can be disabled via config flag (`auth.enabled: false`) for backward compatibility

### Projects

- [x] **PROJ-01**: User can create a project with name and description
- [x] **PROJ-02**: User can list projects they are a member of
- [x] **PROJ-03**: User can view project details
- [x] **PROJ-04**: Project admin can update or delete a project
- [x] **PROJ-05**: Project admin can add/remove members with role (admin/executor/viewer)
- [x] **PROJ-06**: Project admin can change a member's role
- [x] **PROJ-07**: Workflow runs are scoped to a project (project_id on runs table)

### Registry

- [x] **REG-01**: Executor can push a WDL file to the project registry, auto-computing SHA-256 digest
- [x] **REG-02**: Executor can assign a tag to a registry version (mutable pointer)
- [x] **REG-03**: Viewer can pull WDL content by tag (`name:tag`) or digest (`name@sha256:hex`)
- [x] **REG-04**: Viewer can list registry entries, versions, and tags in a project
- [x] **REG-05**: Registry entries are classified as tool (WDL task) or workflow (WDL workflow)
- [x] **REG-06**: Project admin can delete registry entries

### Execution

- [x] **EXEC-01**: Executor can submit a run by registry reference (`project/name:tag` or `project/name@sha256:hex`)
- [x] **EXEC-02**: Server resolves registry ref to WDL content and delivers via existing `WDLContent` path

### Provenance

- [x] **PROV-01**: System records provenance on run submission (registry version digest, inputs, submitting user)
- [x] **PROV-02**: System updates provenance with outputs when run completes
- [ ] **PROV-03**: Viewer can list provenance records for a project
- [ ] **PROV-04**: Viewer can get provenance for a specific run (version, digest, inputs, outputs, user)

## Future Requirements

Deferred to future release. Tracked but not in current roadmap.

### Authentication

- **AUTH-F01**: User can refresh an expiring JWT token via refresh token endpoint
- **AUTH-F02**: OAuth2/LDAP/SSO integration for enterprise single sign-on

### Registry

- **REG-F01**: Registry entries can be marked as public (readable across projects)
- **REG-F02**: GA4GH TRS API compatibility for interoperability with Dockstore/Terra

### Provenance

- **PROV-F01**: Per-task provenance with W3C PROV entity-activity-agent graph
- **PROV-F02**: CWLProv-style research object export for academic publishing

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Semantic version enforcement on tags | Breaks scripting and CI pipelines using git SHA as tags; accept any tag string |
| Global (cross-project) registry namespace | Name collision risk; project-scoped registry with future `is_public` flag instead |
| Per-task W3C PROV lineage | 3-5x scope of run-level provenance; existing tasks table has inputs/outputs for future use |
| Real-time execution events (WebSocket/SSE) | Polling sufficient for batch workflows; revisit when Web UI is added |
| PostgreSQL migration | SQLite with WAL handles current scale; revisit when write contention is measurable |
| CLI commands (flowy push/pull/tag) | API-first for v1.3; CLI commands can be added as a quick task or future milestone |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| AUTH-01 | Phase 19 | Complete |
| AUTH-02 | Phase 19 | Complete |
| AUTH-03 | Phase 19 | Complete |
| AUTH-04 | Phase 19 | Complete |
| AUTH-05 | Phase 19 | Complete |
| AUTH-06 | Phase 19 | Complete |
| PROJ-01 | Phase 20 | Complete |
| PROJ-02 | Phase 20 | Complete |
| PROJ-03 | Phase 20 | Complete |
| PROJ-04 | Phase 20 | Complete |
| PROJ-05 | Phase 20 | Complete |
| PROJ-06 | Phase 20 | Complete |
| PROJ-07 | Phase 20 | Complete |
| REG-01 | Phase 21 | Complete |
| REG-02 | Phase 21 | Complete |
| REG-03 | Phase 21 | Complete |
| REG-04 | Phase 21 | Complete |
| REG-05 | Phase 21 | Complete |
| REG-06 | Phase 21 | Complete |
| EXEC-01 | Phase 22 | Complete |
| EXEC-02 | Phase 22 | Complete |
| PROV-01 | Phase 23 | Complete |
| PROV-02 | Phase 23 | Complete |
| PROV-03 | Phase 23 | Pending |
| PROV-04 | Phase 23 | Pending |

**Coverage:**
- v1.3 requirements: 25 total
- Mapped to phases: 25
- Unmapped: 0 ✓

---
*Requirements defined: 2026-03-17*
*Last updated: 2026-03-17 after roadmap creation (phases 19-23)*
