# Project Research Summary

**Project:** flowy v2.1 — Job Execution UI, CLI Registry Commands, Browser Registry Upload
**Domain:** WDL 1.2 workflow engine additive features (Go backend + React SPA)
**Researched:** 2026-04-07
**Confidence:** HIGH

## Executive Summary

flowy v2.1 is an additive milestone that closes three user-facing capability gaps in an already-functional v2.0 system: users cannot execute workflows from the browser, cannot manage the WDL registry from the CLI, and cannot push WDL files from the browser. All required backend REST endpoints already exist — the server is feature-complete for this scope. The implementation work is confined to new frontend components, new CLI subcommands, one new server endpoint (WDL input schema), and authentication wiring in the CLI client. No new databases, no schema migrations, no new binaries.

The recommended approach is strictly additive and dependency-conservative. The existing React 19 + TanStack Query + react-hook-form + zod + shadcn/ui stack handles the frontend work with zero new npm packages. The Go backend extends with zero new go.mod dependencies. The one substantive architectural decision is to add a `GET /projects/{slug}/registry/{name}/inputs` endpoint that server-parses WDL and returns structured input metadata — this is the prerequisite for the dynamic input form and must be built before the form component. Build order flows: API types and server endpoint, then CLI subcommands and browser upload in parallel, then execution dialog last.

The primary risks are (1) `File`-typed WDL inputs — the browser cannot pass local file paths to the daemon, requiring an upload-first pattern that is explicitly out of scope for v2.1 MVP (treat File inputs as path-entry strings pointing to server-accessible locations), and (2) infinite polling if `refetchInterval` is not gated on terminal run state. Both are well-understood and avoidable with patterns already present in the codebase. Overall risk is LOW given the high proportion of existing, working infrastructure being reused.

## Key Findings

### Recommended Stack

v2.1 requires no new dependencies in either Go or TypeScript. The existing stack is complete for all planned features.

**Core technologies:**
- `react-hook-form` v7.72.1 + `zod` v4: dynamic input form — already installed; build schema at render time from server-returned input descriptors via a ~100-line custom renderer loop
- `@tanstack/react-query` v5.64.2: poll-based status monitoring — `refetchInterval` function gated on terminal state, automatic cleanup on component unmount
- `shadcn/ui` (copy-paste components): `switch`, `textarea`, `progress` components needed — install via `npx shadcn@latest add`, no new npm packages added
- Go stdlib `flag.FlagSet`: CLI registry subcommands — extend existing `switch args[0]` dispatcher in `cmd/flowy/main.go`; do not add `spf13/cobra`
- Go `wdl/parser` (internal): WDL input schema endpoint — reuse existing parser to extract `Workflow.Inputs` AST declarations; zero new libraries
- `FileReader.text()` (Web API): browser WDL upload — read file as UTF-8 text, POST as JSON `{name, content}` to the existing `POST /projects/{slug}/registry` endpoint

**Explicit rejections:** `react-jsonschema-form` (200 KB bundle, over-scoped for 6 WDL primitive types), `react-dropzone` (not required for single-file admin upload), `spf13/cobra` (hybrid migration cost unjustified for 8 subcommands), WebSocket/SSE (explicitly deferred in PROJECT.md).

### Expected Features

**Must have (v2.1 table stakes):**
- `flowy registry push <file> --name <name> --project <slug>` — primary CLI registry management command
- `flowy registry list/pull/tag/delete-tag --project <slug>` — full registry lifecycle from CLI
- UI registry upload dialog — file picker + name field on RegistryPage (executor role only)
- UI workflow execution — select from registry, input form (JSON textarea for MVP), submit, redirect to run detail
- Run detail page polling — 3s interval while PENDING/RUNNING, stops on terminal state
- Runs list auto-refresh — conditional refetch while any run is active

**Should have (v2.1.x after user validation):**
- Dynamic typed input form — WDL declarations parsed into per-type fields (String/Int/Float/Boolean/File), replacing raw JSON textarea
- "Run again" from run detail — pre-populate form with previous run's `ProvenanceDetailResponse.inputs`
- Tag field in browser upload dialog — immediate tagging in same form after push
- `--json` output flag for `flowy registry list` — machine-readable output for automation

**Defer (v2.2+):**
- Task-level status and progress display — requires new task status API endpoint
- Abort button in UI — endpoint exists, just needs UI
- In-browser WDL source editor — large scope, requires syntax highlighting
- Registry entry deletion (full entry, not just tag) — requires new backend endpoint
- File upload staging for File-typed WDL inputs — upload-before-submit infrastructure, deferred per ADR 0001

### Architecture Approach

v2.1 touches six files on the backend and ten on the frontend, adding one new server endpoint and no new binaries. The system's four-binary shape (flowy-server, flowy-ui, flowy-daemon, flowy CLI) is unchanged. The single new server endpoint — `GET /projects/{slug}/registry/{name}/inputs` — reuses the existing WDL parser and `resolveRegistryRefFull` tag resolution logic. The CLI gains one new file (`cmd/flowy/registry.go`) and a new dispatch case in `main.go`. The frontend gains three new components (`UploadWDLDialog`, `RunExecutionDialog`, `WDLInputForm`) and modifications to four existing ones.

**Major components:**
1. `internal/api/types.go` — `RegistryInputDecl` and `RegistryInputsResponse` type definitions (foundation for all new features)
2. `handleRegistryInputs` + `/inputs` route in `handlers_registry.go` — server parses WDL, returns structured input schema (unblocks execution UI)
3. `internal/client/client.go` with `WithToken()` — registry methods + JWT auth support (unblocks CLI)
4. `cmd/flowy/registry.go` — CLI registry subcommands dispatcher and implementations (depends on 3)
5. `UploadWDLDialog.tsx` + `useRegistryPush()` — browser upload (uses existing endpoint; independent of CLI)
6. `WDLInputForm.tsx` + `RunExecutionDialog.tsx` + `useRegistryInputSchema()` — execution dialog (depends on 2)
7. `useRunStatus()` with `refetchInterval` + `RunDetailPage` modifications — live status polling (depends on nothing new; can land with 5 or 6)

### Critical Pitfalls

1. **WDL File inputs passed as browser-local paths** — the daemon runs server-side and cannot access the user's local filesystem. For v2.1 MVP, label `File` inputs as path-entry fields requiring server-accessible paths (e.g., `s3://` or absolute paths). Do not silently allow `C:\Users\alice\data.bam`. Full upload-then-reference infrastructure is deferred per ADR 0001.

2. **WDL input schema inferred client-side** — WDL declaration grammar is non-trivial (nested generic types, optional `?` suffix, multi-line default expressions, struct definitions). Do not regex-parse WDL source in the browser. Build `GET /registry/{name}/inputs` first; the server handles this in <1ms using the existing Go parser.

3. **Polling runs forever after terminal state** — `refetchInterval: 3000` (number literal) never stops. Always pass a function: `(query) => TERMINAL_STATES.has(query.state.data?.state) ? false : 3000`. Failure mode is continuous DB reads for every open completed-run tab and unnecessary re-renders on stale data.

4. **CLI auth not designed before registry commands** — all `/projects/*` endpoints require JWT. Do not add `--token` to every subcommand. Add `flowy login` writing credentials to `~/.flowy/credentials.yaml` (mode 0600), not `./flowy.yaml`. The `client.Client` struct needs a `token` field that injects `Authorization: Bearer` on every request.

5. **`handleOutputs` returns 409 for non-completed runs** — calling the outputs endpoint before `COMPLETED` produces an error query state, not a loading state. Gate `useRunOutputs` on `run.state === 'COMPLETED'`. The correct pattern already exists in `RunDetailPage.tsx`; replicate it consistently in all new polling code.

## Implications for Roadmap

Based on build-order dependencies and pitfall prevention requirements, five phases are suggested:

### Phase 1: API Foundation — Types and Input Schema Endpoint

**Rationale:** The new `RegistryInputDecl`/`RegistryInputsResponse` type definitions and the `/inputs` server endpoint are prerequisites for the execution dialog (Phase 5). Landing them first creates a stable API contract for all downstream work and allows the endpoint to be independently tested before the frontend is built.

**Delivers:** New API types compiled and exported; `GET /projects/{slug}/registry/{name}/inputs?tag=...` returning correct JSON schema for workflows and tasks; route registered with viewer role.

**Addresses:** Pitfall 2 (WDL parsing in browser avoided at the root); architecture foundation for dynamic form.

**Avoids:** Risk of API shape churn during execution dialog implementation.

### Phase 2: CLI Registry Commands

**Rationale:** Depends only on Phase 1 types and existing server endpoints (all already live). Independent of the frontend phases. Ships tangible value to CLI users before any UI work lands.

**Delivers:** `flowy registry push|list|pull|tag|delete-tag`; `internal/client` registry methods with `WithToken()`; `flowy login` writing credentials to `~/.flowy/credentials.yaml` (mode 0600); human-readable table output by default.

**Addresses:** All CLI registry table-stakes features; Pitfall 4 (credential design); Pitfall 7 (CLI push sends file content, not path).

**Avoids:** Token storage in project-level config; per-command `--token` flags.

**Research flag:** Standard patterns — no phase research needed. Go stdlib flag extension and HTTP client auth are well-documented; credential file handling is a one-time design decision resolved in Pitfall 4.

### Phase 3: Browser WDL Upload

**Rationale:** Depends only on the existing `POST /projects/{slug}/registry` endpoint (live since v1.3). Independent of CLI and execution dialog. Simplest frontend feature; validates the upload dialog pattern (file-read as text + mutation + cache invalidation) that the execution dialog will also use.

**Delivers:** `UploadWDLDialog.tsx`; `useRegistryPush()` mutation; "Push WDL" button on RegistryPage gated to executor role; success toast with digest shorthand; registry entry list cache invalidation on success.

**Addresses:** Browser WDL upload table-stakes; RBAC role gating pattern.

**Avoids:** Pitfall 5 — use `file.text()` not `readAsArrayBuffer`; validate `.wdl` extension client-side before submission.

**Research flag:** Standard patterns — no phase research needed. FileReader, TanStack mutation, shadcn Dialog are all established and officially documented.

### Phase 4: Run Status Polling

**Rationale:** Polling infrastructure is a prerequisite for the execution dialog to be useful (users need live feedback after submitting a run). Building and validating this independently — against existing runs in the system — reduces risk for Phase 5.

**Delivers:** `useRunStatus()` hook with conditional `refetchInterval`; `RunDetailPage` updated to poll while active and display live duration; runs list page auto-refresh while any run is in a non-terminal state.

**Addresses:** Run status monitoring table-stakes (auto-refresh, live status badge, duration counter, terminal state indication).

**Avoids:** Pitfall 3 (infinite polling — `refetchInterval` function form); Pitfall 6 (outputs 409 breaking run detail — gate `useRunOutputs` on `COMPLETED`).

**Research flag:** Standard patterns — no phase research needed. TanStack Query `refetchInterval` is a stable, well-documented API.

### Phase 5: UI Workflow Execution Dialog

**Rationale:** Most complex frontend feature. Depends on Phase 1 (input schema endpoint) and Phase 4 (polling infrastructure). Building last minimizes rework risk from API shape changes in earlier phases.

**Delivers:** `RunExecutionDialog.tsx` (modal triggered from RegistryEntryPage version rows); `WDLInputForm.tsx` (per-type field rendering: text, number, checkbox, File path, JSON textarea for complex types); `useRegistryInputSchema()` query; post-submit navigation to run detail; "Run" button on RegistryEntryPage (executor role).

**Addresses:** UI workflow execution table-stakes; optional input labeling; post-submit UX (redirect, not new route).

**Avoids:** Pitfall 1 (File inputs — document server-path requirement with clear placeholder text); anti-pattern of separate route for execution form (use modal pattern consistent with existing dialogs).

**Research flag:** No deep research needed. The one open question — exact UX for `File` input path entry (placeholder text, tooltip, documentation link) — is a design decision, not a research gap. Decide during phase planning.

### Phase Ordering Rationale

- Phase 1 must come first: the new API types underpin all downstream code
- Phases 2, 3, and 4 are mutually independent after Phase 1 and can be parallelized if resources allow; the suggested serial order delivers CLI value first, then UI upload, then UI monitoring
- Phase 5 requires Phase 1 (input schema endpoint) and Phase 4 (polling) to be stable before implementation begins
- This order delivers value incrementally: CLI users gain registry management (Phase 2) before the execution UI lands; UI users gain upload capability (Phase 3) and live run monitoring (Phase 4) as standalone improvements before the full execution dialog ships (Phase 5)

### Research Flags

Phases likely needing deeper research during planning:
- None. All five phases use patterns either already established in this codebase or covered by official library documentation with high confidence.

Phases with standard patterns (skip research-phase):
- **All five phases** — implementation patterns are well-understood; the primary unknowns are design decisions (File input UX, credential file location, polling interval for idle runs list) that should be resolved during phase planning discussions, not research tasks.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All findings from direct codebase inspection of `package.json`, `go.mod`, `cmd/flowy/main.go`, `internal/client/client.go`; confirmed against official library docs |
| Features | HIGH | Table-stakes derived from codebase analysis of existing API surface and confirmed against PROJECT.md milestone scope; comparable system analysis (Terra, Seqera, Argo) used only for differentiator framing |
| Architecture | HIGH | All integration points verified by reading actual source files; build order derived from real import dependencies, not assumption; component responsibilities match existing codebase conventions |
| Pitfalls | HIGH | Based on direct codebase inspection, ADR 0001 (File staging decision), v2.0 milestone audit, and RETROSPECTIVE.md cross-milestone lessons |

**Overall confidence:** HIGH

### Gaps to Address

- **File input UX specifics:** v2.1 MVP treats `File` inputs as path-entry strings. The placeholder text, tooltip, and help link for communicating "server-accessible path required" should be decided during Phase 5 planning. No research needed — this is a product decision.
- **`flowy login` token expiry UX:** When a stored token expires, CLI registry commands must fail with a human-readable "authentication required — run flowy login" message, not a raw `{"error":"..."}` JSON 401 body. The error handling path in `internal/client` needs to be specified during Phase 2 planning.
- **Runs list polling idle behavior:** FEATURES.md specifies auto-refresh "while any run is active." The exact condition for stopping list-level polling (all runs terminal vs. no PENDING/RUNNING runs visible) should be decided during Phase 4 planning.

## Sources

### Primary (HIGH confidence)

- Codebase — `internal/api/types.go`, `internal/server/handlers_registry.go`, `internal/server/server.go`, `internal/client/client.go`, `cmd/flowy/main.go` — server endpoint inventory, client structure, CLI dispatch pattern
- Codebase — `web/src/features/runs/`, `web/src/features/registry/` — existing React component patterns, query hook shapes, RBAC role checks
- Codebase — `wdl/ast/ast.go`, `wdl/types/types.go`, `wdl/parser/` — WDL AST structure for input schema extraction
- Codebase — `docs/adr/0001-remove-heuristic-file-staging-from-client.md` — authoritative File input staging decision
- `.planning/PROJECT.md` — v2.1 milestone scope, WebSocket/SSE deferral decision
- `.planning/RETROSPECTIVE.md` — cross-milestone lessons (v1.0 through v2.0)
- TanStack Query v5 docs (tanstack.com) — `refetchInterval` function signature and behavior
- MDN `FileReader.readAsText` — `file.text()` browser support (Baseline 2022)

### Secondary (MEDIUM confidence)

- Terra/Cromwell parameter input (support.terra.bio) — WDL input form pattern reference
- Seqera Platform monitoring docs (docs.seqera.io) — polling vs push comparison for bioinformatics jobs
- Cromwell REST API (cromwell.readthedocs.io) — server-side input schema pattern reference

### Tertiary (LOW confidence)

- WDL Workspace (EPAM, github.com/epam/wdl-workspace) — comparable open-source WDL UI; architecture reference only
- AWS HealthOmics WDL parameter detection (2025) — confirms server-side schema extraction as industry pattern; cited in FEATURES.md

---
*Research completed: 2026-04-07*
*Ready for roadmap: yes*
