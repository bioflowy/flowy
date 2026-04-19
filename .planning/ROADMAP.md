# Roadmap: flowy

## Overview

flowy is a WDL 1.2 workflow engine in Go. v1.0 shipped as a CLI tool with full parse → typecheck → evaluate → execute pipeline. Spectool conformance baseline: 45/160. v1.1 shipped distributed server/daemon/client architecture for multi-host workflow execution. v1.2 shipped S3/MinIO object storage as an alternative backend to shared filesystem, enabling cloud-native WDL execution. Spectool conformance: 156/156. v1.3 shipped multi-user authentication, project-scoped RBAC, a Docker-registry-style WDL content-addressed registry, registry-based workflow execution, and run provenance tracking. v2.0 shipped a browser-based management SPA embedded in a new `flowy-ui` binary, covering authentication, user/project/registry management, and file browsing with download. v2.1 shipped CLI registry management, browser WDL upload, poll-based run monitoring, and full workflow execution from the browser with dynamic input forms. v2.2 shipped a project-scoped managed file library with import, auto-registration, type annotation, and a file picker for browser workflow execution.

## Milestones

- ✅ **v1.0 MVP** — Phases 1-8 (shipped 2026-03-11)
- ✅ **v1.1 Distributed Execution** — Phases 9-13 (shipped 2026-03-12)
- ✅ **v1.2 S3 Object Storage** — Phases 14-18 (shipped 2026-03-13)
- ✅ **v1.3 Project & Registry** — Phases 19-24 (shipped 2026-03-20)
- ✅ **v2.0 Web UI — Management** — Phases 25-31 (shipped 2026-04-07)
- ✅ **v2.1 Job Execution & Registry CLI** — Phases 32-36 (shipped 2026-04-09)
- ✅ **v2.2 Managed File Library** — Phases 37-40 (shipped 2026-04-19)

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

<details>
<summary>✅ v2.1 Job Execution & Registry CLI (Phases 32-36) — SHIPPED 2026-04-09</summary>

- [x] **Phase 32: API Foundation** (2/2 plans) — completed 2026-04-07
- [x] **Phase 33: CLI Auth and Registry Commands** (2/2 plans) — completed 2026-04-07
- [x] **Phase 34: Browser Registry Upload** (2/2 plans) — completed 2026-04-07
- [x] **Phase 35: Run Status Polling** (2/2 plans) — completed 2026-04-08
- [x] **Phase 36: UI Workflow Execution** (2/2 plans) — completed 2026-04-08

Full details: `.planning/milestones/v2.1-ROADMAP.md`

</details>

<details>
<summary>✅ v2.2 Managed File Library (Phases 37-40) — SHIPPED 2026-04-19</summary>

- [x] **Phase 37: Backend Foundation** (2/2 plans) — completed 2026-04-09
- [x] **Phase 38: File Library REST API** (2/2 plans) — completed 2026-04-19
- [x] **Phase 39: Workflow Completion Hook and Type Annotation** (2/2 plans) — completed 2026-04-19
- [x] **Phase 40: UI — Files Tab and File Picker** (2/2 plans) — completed 2026-04-19

Full details: `.planning/milestones/v2.2-ROADMAP.md`

</details>

## Next Milestone

Not planned yet. Start the next cycle with `/gsd-new-milestone` to define fresh requirements and roadmap entries.
