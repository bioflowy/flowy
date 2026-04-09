# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.0 — MVP

**Shipped:** 2026-03-11
**Phases:** 8 | **Plans:** 26 | **Commits:** 160

### What Was Built
- Complete WDL 1.2 interpreter pipeline: lexer → parser → type checker → evaluator → task runner → workflow scheduler
- CLI tools: `flowy run` and `flowy check` with full flag support
- Docker-based task execution with input staging and output collection
- DAG workflow scheduler with scatter parallelism and conditionals
- 35+ WDL standard library functions
- Spectool conformance integration (45/160 baseline)

### What Worked
- Linear phase dependency chain (each phase builds on previous) kept architecture clean
- ShellRunner mock pattern enabled fast unit testing without Docker
- Pratt parser for expressions was a good fit for WDL's operator precedence
- Rob Pike state-function lexer handled command block mode switching cleanly
- Spectool integration in final phase provided immediate feedback on spec compliance

### What Was Inefficient
- ROADMAP.md progress table got out of sync with actual phase completion (phases 3 and 5 showed "In Progress" despite being complete)
- Some SUMMARY.md files lack one_liner frontmatter, making automated extraction harder
- jsonToValueUntyped was duplicated between task and workflow packages instead of being shared

### Patterns Established
- Type-tagged values throughout (every Value carries its WDL Type)
- ContainerRunner interface for testability
- errgroup with SetLimit for bounded parallelism
- Rustc-style error reporting (file:line:col with source line and caret)

### Key Lessons
1. Go's `flag` package stops at first non-flag arg — positional args must come last in CLI commands
2. Spectool conformance testing is invaluable for catching edge cases (found 6 bugs in phase 8)
3. Hand-written lexer with state functions is more maintainable than parser generators for complex languages like WDL

### Cost Observations
- Model mix: ~20% opus (orchestration), ~80% sonnet (execution)
- Timeline: 2 days from init to milestone complete
- Notable: 8 phases, 26 plans executed with high parallelization within waves

---

## Milestone: v1.1 — Distributed Execution

**Shipped:** 2026-03-12
**Phases:** 5 | **Plans:** 10 | **Commits:** 64

### What Was Built
- Shared foundation: YAML config, HTTP DTO types, SQLite persistence (WAL, CGO-free), filesystem layout
- flowy-server: REST API with DAG dispatch to SQLite task queue, stale task detection
- flowy-daemon: poll-based task claiming, Docker execution, heartbeat, graceful shutdown
- flowy client CLI: submit/status/outputs subcommands, synchronous `flowy run` with polling
- Tech debt cleanup: consolidated ValueToInterface, documentation fixes

### What Worked
- TDD RED-GREEN pattern used consistently across all phases — caught bugs early
- runTaskFn injection seam from v1.0 made distributed dispatch a clean extension
- httptest.NewServer for integration tests — no real server needed
- Phase 13 (tech debt) as explicit milestone phase kept code quality high
- Milestone audit before completion caught 5 documentation/code inconsistencies

### What Was Inefficient
- STATE.md progress tracking fell behind (showed 0% despite all phases complete)
- Phase 12-03 took 30 min (3x longer than average) due to complex run command rewrite
- SUMMARY.md files still lack one_liner frontmatter — extraction requires reading full files
- Long-poll was planned but short-poll (5s ticker) was implemented instead — simpler but higher overhead

### Patterns Established
- Atomic task claiming via single `UPDATE WHERE state='PENDING'` — avoids TOCTOU races
- context.Background() for task execution goroutines — survives SIGTERM for graceful shutdown
- hostname-PID as daemon_id — simple, debuggable, no UUID generation needed
- ADR process for scope changes (ADR 0001: file staging removal)

### Key Lessons
1. Single-UPDATE atomic claiming is simpler and more correct than SELECT+UPDATE for distributed task queues
2. CGO-free SQLite (modernc.org/sqlite) works well — no cross-compilation issues
3. Milestone audit phase (Phase 13) is worth the investment — catches drift between docs and implementation
4. signal.NotifyContext as root context provides clean cancellation for all HTTP calls on Ctrl+C

### Cost Observations
- Model mix: ~20% opus (orchestration), ~80% sonnet (execution)
- Timeline: 2 days (matching v1.0 pace despite architecture complexity)
- Notable: 10 plans in 5 phases, ~74 min total execution time

---

## Milestone: v1.2 — S3 Object Storage

**Shipped:** 2026-03-13
**Phases:** 5 | **Plans:** 10 | **Commits:** 81

### What Was Built
- Storage Backend interface (13 methods) with LocalBackend preserving all existing behavior
- S3Backend via AWS SDK v2 with streaming I/O, multipart upload, MinIO path-style compatibility
- `--storage` CLI flag, YAML storage config, s3:// URI path guard in cleanPath
- Daemon LED pattern (Localize-Execute-Delocalize) with API-based input/output delivery
- MinIO integration tests via testcontainers-go, `make integration` and `make spectool-s3` targets
- 3 quick-fix tasks: DB-based run outputs, path normalization, S3 URI leak fix

### What Worked
- Linear phase dependency chain (14→15→16→17→18) kept each phase focused and testable
- Backend interface design enabled clean S3 swap without touching server/daemon logic
- Quick-fix tasks (6, 7, 8) caught real integration issues that phased development missed
- Milestone audit caught tech debt (stale traceability, missing Makefile targets) before shipping
- Mock S3 client pattern enabled unit testing without Docker/MinIO infrastructure

### What Was Inefficient
- STATE.md progress tracking fell behind again (showed 20% despite all phases complete)
- SUMMARY.md files still lack one_liner frontmatter — pattern not enforced
- Phase 14 ROADMAP.md showed `[~]` (partial) despite being complete — status sync issue
- 3 quick-fix tasks needed after Phase 18 — integration issues between phases not caught during phased testing

### Patterns Established
- LED pattern (Localize-Execute-Delocalize) for backend-agnostic container execution
- s3:// URI guard as first statement in path processing functions
- API-based input/output delivery with DB storage and FS fallback
- testcontainers-go for integration tests with pinned image tags
- Separate Makefile targets for local vs S3 conformance testing

### Key Lessons
1. Quick-fix tasks after integration testing are valuable — real multi-component issues only surface in end-to-end scenarios
2. Backend interface abstraction pays off — S3Backend implementation was clean because interface was well-designed in Phase 14
3. Path processing is a major source of bugs — s3:// URIs interact poorly with filepath.Clean, filepath.Abs, and filepath.Join
4. MinIO testcontainers with pinned image tags are reliable for reproducible S3 testing

### Cost Observations
- Model mix: ~20% opus (orchestration), ~80% sonnet (execution)
- Timeline: 2 days (matching v1.0 and v1.1 pace)
- Notable: 10 plans in 5 phases + 3 quick-fix tasks, ~43 min total execution time

---

## Milestone: v1.3 — Project & Registry

**Shipped:** 2026-03-20
**Phases:** 6 | **Plans:** 14

### What Was Built
- JWT authentication with bcrypt password hashing, admin user seeding on first server start
- Project-scoped RBAC with admin/executor/viewer roles and middleware chain
- Content-addressed WDL registry with SHA-256 digest versioning and mutable tags
- Registry-based workflow execution — submit by name:tag or name@sha256:hex
- Run provenance tracking linking outputs to registry version, inputs, and submitting user
- Gap closure: project-scoped CLI polling (StatusInProject/OutputsInProject)

### What Worked
- Milestone audit caught a cross-phase integration gap (CLI polling used non-project-scoped routes) — Phase 24 gap closure fixed it cleanly
- RBAC middleware chain pattern (JWTMiddleware → projectContext → projectRoleRequired) kept authorization logic composable
- Content-addressed registry model (Docker-style SHA-256 + tags) provided familiar UX and integrity guarantees
- Dual-resolution pattern for registry refs (tag at submit, digest at claim) ensured daemon always gets exact version
- Auth-disabled backward compatibility preserved all spectool conformance tests unchanged

### What Was Inefficient
- SUMMARY.md one_liner frontmatter still not populated — automated extraction continues to fail
- Gap between "phase passed individual verification" and "cross-phase integration works" — audit caught this but late
- Phase 24 (gap closure) could have been avoided if Phase 22 had tested project-scoped polling directly

### Patterns Established
- JWT middleware + RBAC middleware chain for multi-layer auth
- Content-addressed registry with mutable tags (Docker model)
- Dual-resolution: resolve by tag at submit time, re-resolve by digest at claim time
- Provenance creation at submit + touch at terminal state (completed/failed)
- Project-scoped vs legacy route separation via auth.enabled config flag

### Key Lessons
1. Cross-phase integration testing is critical — individual phase verification missed the CLI polling gap
2. Gap closure phases are lightweight and effective — Phase 24 was 1 plan, 2 tasks, fixed cleanly
3. Auth-disabled backward compatibility is worth maintaining — spectool conformance acts as regression gate
4. Content-addressed registries are straightforward to implement with SHA-256 and a versions+tags table

### Cost Observations
- Model mix: ~20% opus (orchestration/planning), ~80% sonnet (execution/verification)
- Timeline: 4 days (2026-03-17 → 2026-03-20), longer than prior milestones due to scope
- Notable: 6 phases, 14 plans; first milestone with gap closure phase

---

## Milestone: v2.0 — Web UI Management

**Shipped:** 2026-04-07
**Phases:** 7 | **Plans:** 16

### What Was Built
- flowy-ui binary serving embedded React SPA with reverse proxy to flowy-server
- Login page with JWT session management, protected routes, password change, logout
- Admin user management (create, deactivate, reactivate, reset password)
- Project management (card grid, detail page with member table, inline role changes)
- Registry browsing (entry list with kind badges, version history, WDL source viewer, tag management)
- File browsing (run list with status badges, output files, download via backend proxy, inline text preview)
- Gap closure: sidebar Members route fix, admin guard on GET /users, auth-disabled redirect

### What Worked
- Phase-level context isolation: each executor agent got fresh 200k context, preventing cross-phase confusion
- UI-SPEC design contracts: catching "Cancel" as a blocked generic label before implementation saved rework
- Milestone audit with integration checker: found 3 real cross-phase wiring bugs that per-phase verification missed
- Parallel wave execution: phases with independent plans ran in parallel, cutting wall time
- Established frontend patterns: by Phase 29, the table+badge+hooks pattern was so consistent that researcher needed 0 user questions

### What Was Inefficient
- Phases 26-27 produced human_needed verification status but human verification was never formally completed
- Nyquist VALIDATION.md created for all 7 phases but none marked nyquist_compliant: true — process existed but wasn't closed
- REQUIREMENTS.md checkbox drift: INFRA-01/02/05 stayed unchecked through several phases until audit caught it

### Patterns Established
- Feature folder: `web/src/features/{domain}/` with hooks, pages, dialogs, __tests__
- React Query hooks in `use{Domain}.ts` with apiClient + Bearer token injection
- shadcn/ui Table + Badge + DropdownMenu for list views; AlertDialog for destructive actions
- Navigation: list page → detail page via clickable rows
- Project-scoped API calls using useProjectStore activeSlug
- Admin gating: check project member role for project-scoped, global role for system actions
- Backend proxy for file downloads — frontend never touches storage backend directly

### Key Lessons
1. Cross-phase integration tests matter more than per-phase verification — 3 bugs found only by integration checker
2. Sidebar navigation is a high-risk integration surface — routes, links, active-state styling all must agree
3. UI-SPEC design contracts pay off for consistency — phases 29-30 had zero design rework
4. Gap closure phases are lightweight — Phase 31 was 2 plans, both parallel, entire fix cycle under 10 minutes

### Cost Observations
- Model mix: ~60% sonnet (research, checking, execution), ~40% opus (planning, orchestration)
- Timeline: 28 days (2026-03-10 → 2026-04-07), first full-stack milestone (Go + React)
- Notable: Phases 29-31 executed in a single conversation session

---

## Milestone: v2.1 — Job Execution & Registry CLI

**Shipped:** 2026-04-09
**Phases:** 5 | **Plans:** 10 | **Commits:** 114

### What Was Built
- Input schema introspection endpoint (`GET /registry/{name}/inputs`) returning structured WDL input declarations
- File upload endpoint (`POST /projects/{slug}/uploads`) with path-traversal protection and executor role gating
- `flowy login` with secure JWT credential storage (0600/0700 permissions) and `flowy registry` subcommands (push/list/pull/tag/delete-tag/delete)
- PushWDLDialog: browser-based WDL upload with FileReader, client-side .wdl validation, and executor+ role gating
- `usePollingRuns` hook with exported `TERMINAL_STATES` — 3s polling for active runs, auto-stops on terminal state, pulsing dot indicators
- RunWorkflowDialog with dynamic zod schema driven by the inputs endpoint, per-type controls, and submission → RunDetailPage navigation

### What Worked
- Server-driven form generation (inputs endpoint → dynamic zod schema) eliminated hard-coded type assumptions in the browser
- `TERMINAL_STATES` exported from a single location ensured zero divergence between polling consumers (usePollingRuns, RunsPage, RunDetailPage)
- Integration checker caught the uploads endpoint / File input gap early — non-blocking but worth documenting for v2.2
- Milestone audit flow (audit → tech_debt status → complete) worked cleanly for a no-blocker milestone
- Dynamic zod schema with `z.preprocess` for numeric fields handled empty-string-to-number coercion correctly

### What Was Inefficient
- SUMMARY.md `one_liner` frontmatter still not populated by the executor — MILESTONES.md auto-extraction produced garbage ("One-liner:" placeholders); had to write accomplishments manually
- REQUIREMENTS.md traceability table never updated from initial draft (all `[ ]` Pending) — cosmetic but creates false signal during audit
- uploads endpoint (API-02) was built in Phase 32 as a prerequisite for File input browser upload, but Phase 36 chose the text-field path instead — coordination between Phase 32 plan and Phase 36 design could have been tighter

### Patterns Established
- `loadCredsAndClient()` helper pattern: all CLI subcommands that need auth call this first, producing human-readable errors for missing/expired tokens
- Server-driven form schema: browser form validation generated at runtime from a server endpoint, never hard-coded
- `usePollingRuns` with `refetchInterval` callback — TanStack Query v5 pattern for conditional polling using `query.state.data`
- TERMINAL_STATES as a named Set exported from the data layer — prevents magic string duplication across polling consumers
- Audit → tech_debt → complete flow: audit is not a blocker gate, it's a quality signal; tech_debt status is a valid path to shipping

### Key Lessons
1. Server-driven form schemas are resilient — adding a new WDL type requires only a server change; the browser form adapts automatically
2. Pre-built endpoints that aren't wired end-to-end are tech debt, not features — the uploads endpoint needs a browser consumer in v2.2 to justify its existence
3. TanStack Query v5 `refetchInterval` callback receives `query` (not `data`) — `query.state.data` is the correct access pattern; this tripped up the initial plan
4. `z.preprocess` is the right tool for optional numeric fields where empty string should become undefined, not 0

### Cost Observations
- Model mix: ~20% opus (orchestration), ~80% sonnet (execution, verification, audit)
- Timeline: 3 days (2026-04-07 → 2026-04-09)
- Notable: 5 phases, 10 plans, 114 commits; milestone audit found no blockers on first pass

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Commits | Phases | Key Change |
|-----------|---------|--------|------------|
| v1.0 | 160 | 8 | Initial pipeline — linear dependency chain |
| v1.1 | 64 | 5 | Distributed architecture — server/daemon/client split |
| v1.2 | 81 | 5 | S3 storage backend — cloud-native WDL execution |
| v1.3 | ~50 | 6 | Auth, RBAC, registry, provenance — multi-user platform |
| v2.0 | ~60 | 7 | Web UI — React SPA with full management interface |
| v2.1 | 114 | 5 | Job execution & CLI — browser + CLI close the loop |

### Cumulative Quality

| Milestone | Go LOC | Frontend LOC | Go Tests | Spectool |
|-----------|--------|-------------|----------|----------|
| v1.0 | 17,648 | — | ~200 | 45/160 (28%) |
| v1.1 | 25,433 | — | ~300 | 156/156 (100%) |
| v1.2 | 29,277 | — | ~350 | 156/156 (100%) |
| v1.3 | 37,776 | — | ~400 | 156/156 (100%) |
| v2.0 | 37,776 | 7,557 | ~450 | 156/156 (100%) |
| v2.1 | 41,483 | 9,193 | 533 | 156/156 (100%) |

### Top Lessons (Verified Across Milestones)

1. Linear phase dependencies produce clean architecture but limit parallelization — verified v1.0, v1.1, v1.2
2. Mock interfaces (ContainerRunner, ShellRunner, mockS3Client) are essential for fast test cycles — verified v1.0, v1.1, v1.2
3. TDD RED-GREEN pattern catches bugs early and produces well-tested code — verified v1.0, v1.1, v1.2
4. Milestone audit catches doc/code drift and tech debt — verified v1.1, v1.2, v2.0, v2.1
5. Quick-fix tasks after integration testing catch real cross-component issues — established v1.2
6. Well-designed interfaces pay off in later phases — Backend interface (Phase 14) made S3Backend (Phase 16) clean
7. Cross-phase integration testing matters more than individual phase verification — verified v1.3, v2.0, v2.1
8. Gap closure phases are lightweight and effective for fixing audit-discovered issues — verified v1.3, v2.0
9. UI-SPEC design contracts prevent design rework in frontend phases — established v2.0
10. Sidebar/navigation is a high-risk integration surface requiring dedicated wiring checks — established v2.0
11. Server-driven schemas (form, validation) are more resilient than hard-coded browser logic — established v2.1
12. Pre-built endpoints without browser consumers are tech debt — coordinate phase scopes to avoid — established v2.1
