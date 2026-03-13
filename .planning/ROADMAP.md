# Roadmap: flowy

## Overview

flowy is a WDL 1.2 workflow engine in Go. v1.0 shipped as a CLI tool with full parse → typecheck → evaluate → execute pipeline. Spectool conformance baseline: 45/160. v1.1 shipped distributed server/daemon/client architecture for multi-host workflow execution. v1.2 adds S3/MinIO object storage as an alternative backend to shared filesystem, enabling cloud-native WDL execution.

## Milestones

- ✅ **v1.0 MVP** — Phases 1-8 (shipped 2026-03-11)
- ✅ **v1.1 Distributed Execution** — Phases 9-13 (shipped 2026-03-12)
- 🚧 **v1.2 S3 Object Storage** — Phases 14-18 (in progress)

## Phases

<details>
<summary>✅ v1.0 MVP (Phases 1-8) — SHIPPED 2026-03-11</summary>

- [x] Phase 1: Type Data Structures (1/1 plans) — completed 2026-03-10
- [x] Phase 2: Lexer, Parser, and AST (5/5 plans) — completed 2026-03-10
- [x] Phase 3: Type Checker (4/4 plans) — completed 2026-03-10
- [x] Phase 4: Expression Evaluator and Standard Library (6/6 plans) — completed 2026-03-10
- [x] Phase 5: Task Execution (3/3 plans) — completed 2026-03-10
- [x] Phase 6: Workflow Execution, DAG Scheduler (3/3 plans) — completed 2026-03-10
- [x] Phase 7: CLI and Integration (2/2 plans) — completed 2026-03-11
- [x] Phase 8: Specification Compliance (2/2 plans) — completed 2026-03-11

Full details: `.planning/milestones/v1.0-ROADMAP.md`

</details>

<details>
<summary>✅ v1.1 Distributed Execution (Phases 9-13) — SHIPPED 2026-03-12</summary>

- [x] Phase 9: API Contract + Persistence Foundation (2/2 plans) — completed 2026-03-11
- [x] Phase 10: Server Binary + DAG Dispatch (2/2 plans) — completed 2026-03-11
- [x] Phase 11: Daemon Binary + Task Execution (2/2 plans) — completed 2026-03-12
- [x] Phase 12: Client CLI + Spectool Compatibility (3/3 plans) — completed 2026-03-12
- [x] Phase 13: Tech Debt Cleanup (1/1 plan) — completed 2026-03-12

Full details: `.planning/milestones/v1.1-ROADMAP.md`

</details>

### 🚧 v1.2 S3 Object Storage (In Progress)

**Milestone Goal:** Enable S3/MinIO object storage as an alternative to shared filesystem, with workflow-level backend selection and path-based input resolution. All 156 spectool conformance tests pass on both local and S3 backends.

- [~] **Phase 14: Storage Abstraction** - Define the Backend interface and wrap existing shared FS as LocalBackend (plan 1/2 complete)
- [x] **Phase 15: Configuration and Path Resolution** - Add --storage flag, S3 config, and s3:// URI detection (completed 2026-03-13)
- [ ] **Phase 16: S3Backend Implementation** - Implement S3Backend using AWS SDK v2 with streaming I/O and MinIO compatibility
- [ ] **Phase 17: Daemon Staging** - Implement localize/delocalize pattern in daemon for S3-backed task execution
- [ ] **Phase 18: MinIO Integration Testing** - End-to-end MinIO testcontainer tests and spectool conformance on S3 backend

## Phase Details

### Phase 14: Storage Abstraction
**Goal**: The storage layer is abstracted behind a Backend interface and all existing behavior is preserved through LocalBackend
**Depends on**: Nothing (continuing from Phase 13)
**Requirements**: STOR-01, STOR-02, STOR-03
**Success Criteria** (what must be TRUE):
  1. A Backend interface exists in internal/storage/ with methods covering file upload, download, JSON read/write, and directory operations
  2. LocalBackend wraps the existing store/rundir.go behavior and all 156 spectool tests pass unchanged
  3. The server binary starts and functions identically to v1.1 using LocalBackend injected via factory
  4. ParseS3URI helper function exists with tests covering edge cases (empty bucket, empty key, trailing slash)
**Plans**: 2 plans

Plans:
- [x] 14-01-PLAN.md — Create storage package with Backend interface, LocalBackend, ParseS3URI, and tests
- [ ] 14-02-PLAN.md — Inject Backend into Server and replace all direct store/os calls

### Phase 15: Configuration and Path Resolution
**Goal**: Users can select a storage backend per workflow via CLI flag, S3 settings are configurable in YAML, and s3:// URIs are safely handled in path processing
**Depends on**: Phase 14
**Requirements**: PATH-01, PATH-02, PATH-03, PATH-04, CONF-01, CONF-02, CONF-03, CONF-04
**Success Criteria** (what must be TRUE):
  1. Running `flowy run --storage s3 workflow.wdl` is accepted without error (backend selection wired end-to-end)
  2. An s3:// URI in an input JSON file passes through cleanPath() without corruption (filepath.Abs is not applied)
  3. Config YAML with storage.type, storage.bucket, storage.endpoint, storage.region, storage.prefix fields is parsed correctly
  4. TaskClaimResponse includes storage addressing fields (type, bucket, endpoint, prefix) populated from server config
  5. All 156 spectool tests continue to pass with the new config fields and path guard in place
**Plans**: 2 plans

Plans:
- [ ] 15-01-PLAN.md — Create pathutil package with s3:// guard, add StorageConfig to YAML config, update factory
- [ ] 15-02-PLAN.md — Add --storage CLI flag to run/submit, extend TaskClaimResponse with storage fields

### Phase 16: S3Backend Implementation
**Goal**: S3Backend fully implements the Backend interface with streaming I/O, MinIO compatibility, and correct scatter key layout
**Depends on**: Phase 15
**Requirements**: S3-01, S3-02, S3-03, S3-04, S3-05
**Success Criteria** (what must be TRUE):
  1. A file can be uploaded to a MinIO/S3 bucket and downloaded back via S3Backend with bit-for-bit identical content
  2. Files exceeding the configurable multipart threshold are uploaded via multipart upload without loading the entire file into memory
  3. S3Backend connects to a MinIO server using UsePathStyle and custom endpoint without error
  4. Scatter tasks produce S3 keys containing the shard index, preventing key collision when multiple shards output identically named files
  5. Unit tests with a mocked S3 client cover upload, download, and error paths
**Plans**: 2 plans

Plans:
- [ ] 16-01-PLAN.md — S3Backend implementation with streaming methods, AWS SDK v2, MinIO path-style, factory wiring
- [ ] 16-02-PLAN.md — S3Backend unit tests with mock client covering upload, download, scatter keys, directory ops

### Phase 17: Daemon Staging
**Goal**: The daemon correctly localizes S3 inputs to POSIX paths before container execution and delocalizes container outputs back to S3, with no s3:// URIs reaching Docker containers
**Depends on**: Phase 16
**Requirements**: DAEM-01, DAEM-02, DAEM-03, DAEM-04
**Success Criteria** (what must be TRUE):
  1. A WDL task with s3:// file inputs executes successfully via Docker with all inputs available as local POSIX paths inside the container
  2. Task output files are uploaded to S3 after container exit and the outputs.json contains s3:// URIs for downstream consumers
  3. Local staging files are removed after successful S3 upload
  4. An S3 upload failure during output collection is reported to the server as a task failure with a descriptive error message
**Plans**: 2 plans

Plans:
- [ ] 17-01: TBD

### Phase 18: MinIO Integration Testing
**Goal**: End-to-end integration tests using a real MinIO server validate S3 workflow execution, and spectool 156/156 passes on both local and S3 backends
**Depends on**: Phase 17
**Requirements**: TEST-01, TEST-02
**Success Criteria** (what must be TRUE):
  1. Integration tests spin up a MinIO testcontainer with a pinned image tag and run a full workflow end-to-end via S3 backend without AWS credentials
  2. A WDL workflow that uploads a file, processes it in a task, and passes the output to a downstream task executes correctly via S3 with intermediate files visible in the MinIO bucket
  3. All 156 spectool conformance tests pass when the S3/MinIO backend is configured as the default storage
**Plans**: 2 plans

Plans:
- [ ] 18-01: TBD

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
| 14. Storage Abstraction | 2/2 | Complete    | 2026-03-13 | - |
| 15. Configuration and Path Resolution | 2/2 | Complete    | 2026-03-13 | - |
| 16. S3Backend Implementation | 1/2 | In Progress|  | - |
| 17. Daemon Staging | v1.2 | 0/TBD | Not started | - |
| 18. MinIO Integration Testing | v1.2 | 0/TBD | Not started | - |
