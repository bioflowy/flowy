# Requirements: flowy v2.0 Web UI — Management

**Defined:** 2026-03-20
**Core Value:** WDL 1.2で記述されたワークフローをDockerコンテナ上で正確に実行できること

## v2.0 Requirements

Requirements for Web UI management milestone. Each maps to roadmap phases.

### Infrastructure

- [ ] **INFRA-01**: flowy-ui Go binary serves embedded React SPA via go:embed with SPA fallback routing
- [ ] **INFRA-02**: flowy-ui reverse-proxies /api/* requests to flowy-server
- [x] **INFRA-03**: Vite + React 19 + TypeScript + shadcn/ui + Tailwind v4 project scaffold
- [x] **INFRA-04**: flowy-server supports config-gated CORS middleware for development mode
- [ ] **INFRA-05**: Makefile `build-ui` target builds frontend and Go binary

### Authentication

- [x] **AUTH-01**: User can log in with username and password via login screen
- [x] **AUTH-02**: JWT token is stored in session and injected into all API requests
- [x] **AUTH-03**: User is automatically redirected to login on 401 response
- [x] **AUTH-04**: User can change their own password
- [x] **AUTH-05**: User can log out (token cleared, redirect to login)

### User Management

- [x] **USER-01**: Admin can view all users in a table
- [x] **USER-02**: Admin can create a new user
- [x] **USER-03**: Admin can deactivate/reactivate a user
- [x] **USER-04**: Admin can reset another user's password

### Project Management

- [x] **PROJ-01**: User can view list of projects they belong to
- [x] **PROJ-02**: User can create a new project
- [ ] **PROJ-03**: Admin can manage project members (add/remove/change role)
- [ ] **PROJ-04**: User can view project dashboard with summary info

### Registry

- [ ] **REG-01**: User can view list of registry entries in a project
- [ ] **REG-02**: User can view versions and tags for a registry entry
- [ ] **REG-03**: User can view WDL source code of a registry version
- [ ] **REG-04**: Admin can create/move/delete tags on registry entries

### File Browsing

- [ ] **FILE-01**: User can view list of workflow runs in a project
- [ ] **FILE-02**: User can view output files for a completed run
- [ ] **FILE-03**: User can download output files (S3/local)
- [ ] **FILE-04**: User can preview small text files inline

## v2.1 Requirements (Deferred)

### Job Execution

- **EXEC-01**: User can select a workflow from registry to execute
- **EXEC-02**: User can input workflow parameters via dynamically generated form
- **EXEC-03**: User can submit workflow execution from the UI
- **EXEC-04**: User can monitor running workflow status in real-time
- **EXEC-05**: User can view execution results and output files

## Out of Scope

| Feature | Reason |
|---------|--------|
| Server-side rendering (Next.js/Remix) | Pure SPA served from Go binary; no Node.js runtime in production |
| OAuth2/LDAP/SSO login | Simple username/password sufficient; deferred |
| WebSocket/SSE real-time updates | Polling sufficient for management screens; revisit in v2.1 |
| File upload | Read-only file browsing; upload via CLI |
| User profile editing (display name, avatar) | No profile fields exist in current schema |
| Dark mode | Cosmetic; can be added later without architecture changes |
| Mobile-responsive layout | Desktop-first admin UI; responsive not required for v2.0 |
| Internationalization (i18n) | English-only UI for v2.0 |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| INFRA-01 | Phase 25 | Pending |
| INFRA-02 | Phase 25 | Pending |
| INFRA-03 | Phase 25 | Complete |
| INFRA-04 | Phase 25 | Complete |
| INFRA-05 | Phase 25 | Pending |
| AUTH-01 | Phase 26 | Complete |
| AUTH-02 | Phase 26 | Complete |
| AUTH-03 | Phase 26 | Complete |
| AUTH-04 | Phase 26 | Complete |
| AUTH-05 | Phase 26 | Complete |
| USER-01 | Phase 27 | Complete |
| USER-02 | Phase 27 | Complete |
| USER-03 | Phase 27 | Complete |
| USER-04 | Phase 27 | Complete |
| PROJ-01 | Phase 28 | Complete |
| PROJ-02 | Phase 28 | Complete |
| PROJ-03 | Phase 28 | Pending |
| PROJ-04 | Phase 28 | Pending |
| REG-01 | Phase 29 | Pending |
| REG-02 | Phase 29 | Pending |
| REG-03 | Phase 29 | Pending |
| REG-04 | Phase 29 | Pending |
| FILE-01 | Phase 30 | Pending |
| FILE-02 | Phase 30 | Pending |
| FILE-03 | Phase 30 | Pending |
| FILE-04 | Phase 30 | Pending |

**Coverage:**
- v2.0 requirements: 26 total
- Mapped to phases: 26
- Unmapped: 0 ✓

---
*Requirements defined: 2026-03-20*
*Last updated: 2026-03-20 after roadmap creation*
