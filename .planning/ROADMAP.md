# Roadmap: flowy

## Overview

flowy is a WDL 1.2 workflow engine in Go. v1.0 shipped as a CLI tool with full parse → typecheck → evaluate → execute pipeline. Spectool conformance baseline: 45/160. v1.1 shipped distributed server/daemon/client architecture for multi-host workflow execution. v1.2 shipped S3/MinIO object storage as an alternative backend to shared filesystem, enabling cloud-native WDL execution. Spectool conformance: 156/156. v1.3 shipped multi-user authentication, project-scoped RBAC, a Docker-registry-style WDL content-addressed registry, registry-based workflow execution, and run provenance tracking. v2.0 shipped a browser-based management SPA embedded in a new `flowy-ui` binary, covering authentication, user/project/registry management, and file browsing with download. v2.1 closes the remaining capability gaps: CLI registry management, browser WDL upload, poll-based run monitoring, and full workflow execution from the browser.

## Milestones

- ✅ **v1.0 MVP** — Phases 1-8 (shipped 2026-03-11)
- ✅ **v1.1 Distributed Execution** — Phases 9-13 (shipped 2026-03-12)
- ✅ **v1.2 S3 Object Storage** — Phases 14-18 (shipped 2026-03-13)
- ✅ **v1.3 Project & Registry** — Phases 19-24 (shipped 2026-03-20)
- ✅ **v2.0 Web UI — Management** — Phases 25-31 (shipped 2026-04-07)
- 🚧 **v2.1 Job Execution & Registry CLI** — Phases 32-36 (in progress)

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

<details>
<summary>✅ v2.0 Web UI — Management (Phases 25-31) — SHIPPED 2026-04-07</summary>

- [x] **Phase 25: Infrastructure Foundation** (3/3 plans) — completed 2026-03-24
- [x] **Phase 26: Authentication** (3/3 plans) — completed 2026-04-06
- [x] **Phase 27: User Management** (2/2 plans) — completed 2026-04-06
- [x] **Phase 28: Project Management** (2/2 plans) — completed 2026-04-06
- [x] **Phase 29: Registry Browsing** (2/2 plans) — completed 2026-04-07
- [x] **Phase 30: File Browsing** (2/2 plans) — completed 2026-04-07
- [x] **Phase 31: Audit Gap Closure** (2/2 plans) — completed 2026-04-07

Full details: `.planning/milestones/v2.0-ROADMAP.md`

</details>

### 🚧 v2.1 Job Execution & Registry CLI (In Progress)

**Milestone Goal:** Users can execute workflows from the web UI with parameter input and poll-based status monitoring, push/pull/manage registry entries via CLI, and upload WDL files from the browser.

- [x] **Phase 32: API Foundation** - Add input schema endpoint and file upload endpoint to flowy-server (completed 2026-04-07)
- [x] **Phase 33: CLI Auth and Registry Commands** - Add `flowy login` and `flowy registry` subcommands (completed 2026-04-07)
- [ ] **Phase 34: Browser Registry Upload** - Add WDL upload dialog on RegistryPage
- [ ] **Phase 35: Run Status Polling** - Add poll-based auto-refresh to RunDetailPage and RunsPage
- [ ] **Phase 36: UI Workflow Execution** - Add execution dialog with dynamic input form and result display

## Phase Details

### Phase 32: API Foundation
**Goal**: flowy-server exposes stable API contracts for input schema introspection and file upload, unblocking all frontend execution work
**Depends on**: Phase 31
**Requirements**: API-01, API-02
**Success Criteria** (what must be TRUE):
  1. `GET /projects/{slug}/registry/{name}/inputs?tag=<tag>` returns structured JSON listing each workflow input with name, type string, and optional flag
  2. Calling the inputs endpoint for a workflow with no declared inputs returns an empty inputs array (not an error)
  3. `POST /projects/{slug}/uploads` (or equivalent file upload endpoint) accepts a multipart or base64 body and returns a server-accessible path
  4. Both endpoints are protected by JWT and respect project viewer/executor role gating
**Plans**: 2 plans
Plans:
- [x] 32-01-PLAN.md — Input schema introspection endpoint (API-01)� Input schema introspection endpoint (API-01)
- [x] 32-02-PLAN.md — File upload endpoint (API-02)� File upload endpoint (API-02)

### Phase 33: CLI Auth and Registry Commands
**Goal**: CLI users can authenticate once with `flowy login` and then push, list, pull, tag, and delete registry entries without repeating credentials
**Depends on**: Phase 32
**Requirements**: CLI-01, CLI-02, REG-CLI-01, REG-CLI-02, REG-CLI-03, REG-CLI-04, REG-CLI-05
**Success Criteria** (what must be TRUE):
  1. `flowy login --server <url>` prompts for username/password, saves JWT to `~/.flowy/credentials.yaml` (mode 0600), and prints a confirmation
  2. Running any `flowy registry` subcommand with a valid saved token succeeds without prompting for credentials
  3. Running any `flowy registry` subcommand with an expired or missing token fails with a human-readable message directing the user to run `flowy login` (not a raw JSON 401)
  4. `flowy registry push --project <slug> --name <name> <file.wdl>` pushes the WDL content and prints the resulting digest
  5. `flowy registry list --project <slug>` prints a human-readable table of registry entries
  6. `flowy registry pull --project <slug> --name <name> [--tag <tag>]` prints WDL source to stdout
  7. `flowy registry tag` and `flowy registry delete-tag` create and remove tags on existing versions
  8. `flowy registry delete --project <slug> --name <name>` removes the entry (admin only)
**Plans**: 2 plans
Plans:
- [x] 33-01-PLAN.md — Credential infrastructure and login command (CLI-01, CLI-02)
- [x] 33-02-PLAN.md — Registry subcommands (REG-CLI-01, REG-CLI-02, REG-CLI-03, REG-CLI-04, REG-CLI-05)

### Phase 34: Browser Registry Upload
**Goal**: Browser users with executor role can push a WDL file to the registry without leaving the UI
**Depends on**: Phase 32
**Requirements**: REG-UI-01
**Success Criteria** (what must be TRUE):
  1. A "Push WDL" button appears on the RegistryPage for users with executor role and is absent for viewers
  2. Clicking the button opens a dialog where the user can select a `.wdl` file and enter a registry entry name
  3. After submitting, the registry entry list refreshes and the new entry appears with the correct name and digest
  4. Submitting a non-`.wdl` file is rejected client-side with an inline validation message before any network request is made
**Plans**: 2 plans
Plans:
- [ ] 34-01-PLAN.md — PushWDLDialog component + usePushWDL hook + dialog tests
- [ ] 34-02-PLAN.md — Wire into RegistryPage with role gating + page tests + visual verification

### Phase 35: Run Status Polling
**Goal**: Browser users see live run status without manual page refresh, and polling stops automatically when a run reaches a terminal state
**Depends on**: Phase 32
**Requirements**: EXEC-04
**Success Criteria** (what must be TRUE):
  1. RunDetailPage polls the run status endpoint every 3 seconds while the run is in PENDING or RUNNING state and updates the status badge in place
  2. Polling stops automatically when the run reaches COMPLETED, FAILED, or ABORTED — no further requests are made
  3. RunsPage auto-refreshes the run list while any run in the visible list is in a non-terminal state, then stops
  4. Opening a RunDetailPage for an already-completed run never triggers polling
**Plans**: 2 plans
Plans:
- [ ] 34-01-PLAN.md — PushWDLDialog component + usePushWDL hook + dialog tests
- [ ] 34-02-PLAN.md — Wire into RegistryPage with role gating + page tests + visual verification

### Phase 36: UI Workflow Execution
**Goal**: Browser users with executor role can select a workflow from the registry, fill in input parameters, submit execution, and view results — all from the browser
**Depends on**: Phase 32, Phase 35
**Requirements**: EXEC-01, EXEC-02, EXEC-03, EXEC-05
**Success Criteria** (what must be TRUE):
  1. A "Run" button appears on the RegistryEntryPage for users with executor role and is absent for viewers
  2. Clicking "Run" opens a dialog showing a dynamically generated form with one field per declared workflow input, using appropriate controls per type (text for String, number for Int/Float, checkbox for Boolean, path-entry text for File, JSON textarea for Array/Map/Struct)
  3. Submitting the form creates a new run and navigates the user to the RunDetailPage for that run
  4. File-typed inputs display a placeholder and tooltip explaining that server-accessible paths are required (e.g., `s3://` or absolute paths), not browser-local file paths
  5. After the run completes, the RunDetailPage shows the output files section reusing the Phase 30 output viewer
**Plans**: 2 plans
Plans:
- [ ] 34-01-PLAN.md — PushWDLDialog component + usePushWDL hook + dialog tests
- [ ] 34-02-PLAN.md — Wire into RegistryPage with role gating + page tests + visual verification

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1-8 | v1.0 | 26/26 | Complete | 2026-03-11 |
| 9-13 | v1.1 | 10/10 | Complete | 2026-03-12 |
| 14-18 | v1.2 | 10/10 | Complete | 2026-03-13 |
| 19-24 | v1.3 | 14/14 | Complete | 2026-03-20 |
| 25-31 | v2.0 | 16/16 | Complete | 2026-04-07 |
| 32. API Foundation | v2.1 | 2/2 | Complete    | 2026-04-07 |
| 33. CLI Auth and Registry Commands | v2.1 | 2/2 | Complete    | 2026-04-07 |
| 34. Browser Registry Upload | v2.1 | 0/2 | Planning done | - |
| 35. Run Status Polling | v2.1 | 0/TBD | Not started | - |
| 36. UI Workflow Execution | v2.1 | 0/TBD | Not started | - |
