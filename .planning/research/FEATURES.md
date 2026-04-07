# Feature Research

**Domain:** Workflow execution UI, CLI registry management, browser-based WDL upload — additive features for a WDL 1.2 engine (flowy v2.1)
**Researched:** 2026-04-07
**Confidence:** HIGH (codebase analysis) / MEDIUM (domain patterns from comparable systems)

---

## Context: What Already Exists

These features build on v2.0 foundations. Understanding existing capabilities is essential for scoping.

**Existing backend API (all ready to call):**
- `POST /projects/{slug}/registry` — push WDL (JSON body with `name` + `content`)
- `GET /projects/{slug}/registry` — list entries
- `GET /projects/{slug}/registry/{name}` — pull WDL content
- `GET /projects/{slug}/registry/{name}/versions` — list versions
- `POST /projects/{slug}/registry/{name}/tags` — create/move tag
- `DELETE /projects/{slug}/registry/{name}/tags/{tag}` — delete tag
- `POST /projects/{slug}/runs/submit` — submit run via `{registry_ref, inputs}` JSON
- `GET /projects/{slug}/runs` — list runs (returns state, registry_ref, timing)
- `GET /projects/{slug}/runs/{id}/status` — poll status
- `GET /projects/{slug}/runs/{id}/outputs` — fetch outputs

**Existing frontend infrastructure (React 19, TanStack Query, shadcn/ui, Tailwind):**
- Registry page: lists entries, versions table, WDL source viewer, tag create/delete dialogs
- Runs page: list with status badges (PENDING/RUNNING/COMPLETED/FAILED), duration, click-through to detail
- Run detail page: outputs table, file download, inline preview

**Existing CLI client package (`internal/client`):**
- `Submit`, `SubmitToProject`, `Status`, `StatusInProject`, `Outputs`, `OutputsInProject`
- No registry methods exist in `internal/client`

---

## Feature Area 1: UI Workflow Execution (Select + Submit + Monitor)

### Table Stakes

Features users of comparable systems (Terra, Nextflow Tower, Argo Workflows UI) expect as baseline.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Select workflow from registry dropdown | Registry browsing already works; execution must follow naturally | LOW | Populated from existing `GET /projects/{slug}/registry` filtered to `kind=workflow` |
| Input form for basic scalar types (String, Int, Float, Boolean) | Cromwell/WES all require parameter input at submit time; no raw JSON editing for simple cases | MEDIUM | Requires parsing WDL inputs from WDL source — server returns raw WDL, client parses input declarations |
| Submit button with loading state | Every job-submission UI has submit + spinner | LOW | Calls `POST /projects/{slug}/runs/submit` with `{registry_ref, inputs}` |
| Post-submit redirect to run detail | Users need to know where their run went | LOW | Navigate to `/projects/{slug}/runs/{run_id}` after submit response |
| Status polling on run detail page | Runs page already shows status; detail page must refresh while RUNNING | MEDIUM | TanStack Query `refetchInterval` on `GET .../status`; stop when terminal state reached |
| Status badge updates without page reload | Users expect live-ish status; full-page reload is unacceptable | LOW | Already implemented in Runs page; apply same pattern to detail page poll |
| Outputs display on completion | Detail page already renders outputs; must trigger fetch when status transitions to COMPLETED | LOW | Already exists — `useRunOutputs` only fetches when state === 'COMPLETED' |

### Differentiators

Features that make the execution UI meaningfully better than just a raw JSON POST.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Dynamic input form derived from WDL declarations | Users see typed fields (string input, number input, checkbox) instead of raw JSON — matches what Terra/Seqera do | HIGH | Parse WDL source in frontend. Requires scanning `input {}` block declarations. Only feasible for primitive types; complex types fall back to JSON textarea |
| Optional input detection (show/hide) | WDL optional inputs (`String?`) should not require user input — mark clearly | MEDIUM | Parsing `?` type suffix from WDL input declarations |
| File-type input via path entry | WDL `File` inputs are common in bioinformatics; user must provide s3:// or absolute paths | LOW | String input with `File` label; no actual upload (files live on storage backend) |
| Registry ref display in submit form | Show which digest/tag will be used for traceability | LOW | Pull from the version selected; display sha256 prefix |
| "Run again" from run detail | Bioinformatics users re-run with tweaked parameters frequently | MEDIUM | Pre-populate submission form with previous run's inputs from `ProvenanceDetailResponse.inputs` |

### Anti-Features

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Real-time log streaming | Users want to see task logs while running | WebSocket/SSE explicitly deferred in PROJECT.md; adds server complexity for v2.1 | Poll-based status with state transitions is sufficient; logs visible post-run via file download |
| File upload for WDL File inputs | Users want to upload data files | Files must live on the shared filesystem or S3 backend — browser uploads bypass the LED execution pattern entirely; would require server-side staging | Document that File inputs must use server-accessible paths (s3://, absolute) |
| DAG visualization | Nice for understanding workflow structure | High complexity; requires WDL parsing + graph rendering library; not needed for basic execution | Show task names in run detail if/when task-level status is added |
| Cancel/abort button in UI | Users want to stop running jobs | The abort endpoint exists (`POST .../abort`) but the UI currently has no cancel UX; low priority for v2.1 | Defer to v2.2 |

---

## Feature Area 2: Run Status Monitoring (Polling)

### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Automatic status refresh on run detail page | Users leave the page open; status must update | LOW | TanStack Query `refetchInterval: 3000` while state is PENDING/RUNNING; disable on terminal state |
| Status badge that reflects live state | Text description alone is insufficient; color coding expected | LOW | Already done in RunsPage and RunDetailPage — apply same `statusBadgeClassName` |
| Duration counter updating while RUNNING | Users want to know how long a run has been going | LOW | `formatDuration` already handles RUNNING state by using `Date.now()` — just needs a re-render trigger from polling |
| Clear terminal state indication (COMPLETED/FAILED) | Users must know when to look at outputs | LOW | Already handled via state-conditional output section in RunDetailPage |
| Error message on FAILED state | Users need actionable information | MEDIUM | Currently shows "check CLI logs" — server stores error field on run record but UI does not display it |

### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Runs list page auto-refresh | Users can leave Runs page open and see new runs appear or status change | LOW | `refetchInterval` on `useRuns` hook — currently fires once on mount |
| "Submit new run" button on Runs page | Obvious entry point to execution flow | LOW | Button or link to execution dialog/page |

### Anti-Features

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| WebSocket/SSE push updates | Instant status without polling | Explicitly out of scope in PROJECT.md for v2.1; requires server-side streaming infrastructure | 3s polling interval is imperceptible for long-running bioinformatics jobs |
| Task-level progress (individual task status) | Users want fine-grained progress | Requires task polling loop and DB join; tasks table exists but has no project-scoped API endpoint | Defer — would be a compelling v2.2 feature |

---

## Feature Area 3: CLI Registry Commands

### Table Stakes

Commands that users of Docker-registry-style CLIs (Docker CLI, Dockstore CLI) expect.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| `flowy registry push <file.wdl> --name <name> --project <slug>` | Backend `POST /projects/{slug}/registry` already exists; CLI wrapper is the user's primary interface | MEDIUM | Read file, POST JSON `{name, content}`, print digest + version ID on success. Needs auth token from config |
| `flowy registry list --project <slug>` | Users need to see what's in the registry | LOW | `GET /projects/{slug}/registry`, format as table: name, kind, versions, tags |
| `flowy registry pull <name[:tag|@digest]> --project <slug>` | Users need to retrieve WDL source locally | LOW | `GET /projects/{slug}/registry/{name}` with optional tag/digest query param, write content to stdout or `--output` file |
| `flowy registry tag <name> <digest> <tag> --project <slug>` | Mutable tags are core to the registry model (Docker pattern) | LOW | `POST /projects/{slug}/registry/{name}/tags` with `{tag, digest}` body |
| `flowy registry delete-tag <name> <tag> --project <slug>` | Lifecycle management; admins need to clean up | LOW | `DELETE /projects/{slug}/registry/{name}/tags/{tag}` |
| Authentication propagation | Registry commands need auth — same JWT flow as `flowy run` | LOW | Read token from config file or `FLOWY_TOKEN` env var; add `Authorization: Bearer` header via existing client |

### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| `--output` flag on `pull` to save WDL to file | Scripting and automation workflows need file output | LOW | `os.WriteFile` if flag set, else stdout |
| Human-readable table output (default) + `--json` flag | Matches modern CLI conventions (cf. `gh`, `docker`) | LOW | For `list` command — default prints aligned columns, `--json` emits raw API response |
| Push prints tag suggestion | After push, print "use `flowy registry tag <name> <digest> latest` to tag this version" | LOW | Pure UX improvement in push output |
| Config `default_project` fallback | `flowy registry list` without `--project` falls back to `cfg.Client.DefaultProject` | LOW | Consistent with how `flowy run --project` already works |

### Anti-Features

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| `flowy registry delete` (entry-level delete) | Cleanup of entire entries | Backend delete-entry endpoint does not exist; scope creep for v2.1 | Implement tag cleanup; full entry deletion is a separate backend feature |
| Import resolution at push time | Validate imports before pushing | HTTP imports are out of scope; local imports are not resolvable from the server side | Push sends raw content; server validates WDL parse only (already implemented in handler) |
| Bulk push from directory | Push all `.wdl` files in a folder | Rarely needed; adds complexity | Shell loop: `for f in *.wdl; do flowy registry push $f ...; done` |

---

## Feature Area 4: Browser-Based WDL Upload (UI Registry Push)

### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| File picker for `.wdl` files | Browser upload convention; users should not paste WDL text manually | LOW | `<input type="file" accept=".wdl">` reads file content via FileReader API |
| Name field for registry entry | Required by `POST /projects/{slug}/registry` | LOW | Text input validated against slug format (lowercase alphanumeric + hyphens) |
| Upload button with loading state | Any form submission needs progress feedback | LOW | Show spinner during POST |
| Success feedback with digest displayed | Users need confirmation and traceability | LOW | Toast or inline message showing `sha256:...` (short) and kind detected |
| Error feedback (parse failure, name conflict) | Server returns 422 for invalid WDL, 409 for kind mismatch | LOW | Display server error message inline |
| Admin/executor role gating | Only admins and executors should push — matches RBAC model | LOW | Check `currentMember.role` in `['admin', 'executor']`; hide button for viewers |

### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| WDL content preview before upload | User can confirm they selected the right file | LOW | Read file via FileReader, display first N lines in a code block |
| Auto-populate name from filename | Remove manual entry step; strip `.wdl` extension, convert to slug format | LOW | `filename.replace(/\.wdl$/, '').toLowerCase().replace(/[^a-z0-9-]/g, '-')` |
| Tag field (optional) | User can immediately tag the version after push in the same form | MEDIUM | Make optional; if set, POST tag after successful push |
| Registry page "Upload" button | Discoverable entry point to the upload flow | LOW | Add button to RegistryPage header alongside existing entries list |

### Anti-Features

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Drag-and-drop upload | Modern UX convention | Adds complexity with no functional gain for WDL — users typically navigate to specific files | Standard file picker is sufficient; can be added later as enhancement |
| Multi-file upload (for import dependencies) | WDL workflows can import other files | HTTP imports deferred; local imports not resolvable server-side — pushing a bundle is not supported | Single-file push; document that workflows with local imports must use CLI |
| In-browser WDL editor | Users want to write WDL in the browser | High complexity (syntax highlighting, error markers); out of scope | Use WDLSourceViewer for read-only display; direct users to local editors |

---

## Feature Dependencies

```
[UI Workflow Execution]
    requires ──> [Registry entry list] (existing v2.0)
    requires ──> [WDL source retrieval] (existing v2.0 — needed to derive input schema)
    requires ──> [Run submission API] (existing v1.3)
    enhances ──> [Run status monitoring]

[Run status monitoring]
    requires ──> [Run detail page] (existing v2.0)
    enhances ──> [Runs list page] (existing v2.0)

[CLI registry push]
    requires ──> [internal/client registry push method] (new)
    requires ──> [auth token in config] (existing config structure)
    enhances ──> [UI registry upload] (same backend API)

[CLI registry list/pull/tag/delete-tag]
    requires ──> [internal/client registry methods] (new)
    enhances ──> [UI registry browsing] (same data)

[UI registry upload]
    requires ──> [Registry push API] (existing v1.3)
    requires ──> [RBAC role check] (existing v2.0 pattern)
    enhances ──> [CLI registry push] (same data; different entry point)

[Dynamic WDL input form]
    requires ──> [UI workflow execution] (context for form)
    requires ──> [WDL source content] (to derive input schema)
    enhances ──> [UI workflow execution] (replaces raw JSON textarea)
```

### Dependency Notes

- **Dynamic input form requires WDL parsing in the frontend.** The server returns raw WDL content in `RegistryPullResponse.content`. The UI must parse `input {}` declarations to render typed fields. A lightweight line-by-line scan of the WDL input block is feasible for primitive types; full AST parsing in the browser is not justified for v2.1.
- **CLI registry commands require new `internal/client` methods.** The client package has no registry methods. Six new methods are needed (`Push`, `List`, `Pull`, `CreateTag`, `DeleteTag`, and their project-scoped variants). This is the foundation for all CLI registry subcommands.
- **UI registry upload and CLI push share the same API endpoint.** The `POST /projects/{slug}/registry` endpoint accepts JSON `{name, content}`. The UI reads file content via FileReader and posts this JSON — identical to the CLI flow.
- **`flowy registry` subcommand requires a new command dispatcher.** Currently `cmd/flowy/main.go` handles `check`, `run`, `submit`, `status`, `outputs`. A new `registry` case must be added, with sub-dispatch to `push`, `list`, `pull`, `tag`, `delete-tag`.

---

## MVP Definition

### Launch With (v2.1)

These are the minimum features to satisfy the milestone goal stated in PROJECT.md.

- [ ] `flowy registry push <file> --name <name> --project <slug>` — primary registry management CLI command
- [ ] `flowy registry list --project <slug>` — list registry entries from CLI
- [ ] `flowy registry pull <name[:tag]> --project <slug>` — retrieve WDL source from CLI
- [ ] `flowy registry tag <name> <digest> <tag> --project <slug>` — tag a version
- [ ] `flowy registry delete-tag <name> <tag> --project <slug>` — remove a tag
- [ ] UI registry upload dialog on RegistryPage — file picker, name field, submit, success/error feedback
- [ ] UI workflow execution — select from registry, JSON textarea for inputs, submit, redirect to run detail
- [ ] Run detail page polling — 3s interval while RUNNING/PENDING, stop on terminal state
- [ ] Runs list auto-refresh — same polling while any run is active

### Add After Validation (v2.1.x)

- [ ] Dynamic typed input form (parse WDL input declarations) — trigger: user feedback that raw JSON is painful
- [ ] "Run again" from run detail — trigger: power users requesting it
- [ ] Tag field in UI upload dialog — trigger: users finding post-upload tag creation cumbersome
- [ ] `--json` output flag for `flowy registry list` — trigger: automation/scripting needs

### Future Consideration (v2.2+)

- [ ] Task-level status and progress display — requires task status API endpoint
- [ ] Abort button in UI — abort endpoint exists, just needs UI
- [ ] In-browser WDL source editor — large scope, requires syntax highlighting
- [ ] Registry entry deletion (full entry, not just tag) — requires new backend endpoint

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| `flowy registry push` | HIGH | MEDIUM | P1 |
| `flowy registry list` | HIGH | LOW | P1 |
| `flowy registry pull` | HIGH | LOW | P1 |
| `flowy registry tag` | MEDIUM | LOW | P1 |
| `flowy registry delete-tag` | MEDIUM | LOW | P1 |
| UI registry upload dialog | HIGH | LOW | P1 |
| UI workflow execution (JSON textarea inputs) | HIGH | MEDIUM | P1 |
| Run detail page polling | HIGH | LOW | P1 |
| Runs list auto-refresh | MEDIUM | LOW | P1 |
| Dynamic typed input form | HIGH | HIGH | P2 |
| Tag field in upload dialog | LOW | MEDIUM | P2 |
| "Run again" button | MEDIUM | MEDIUM | P2 |
| `--json` CLI output flag | LOW | LOW | P2 |
| Abort button in UI | MEDIUM | LOW | P3 |
| Task-level status display | HIGH | HIGH | P3 |

---

## Comparable System Analysis

These are reference systems studied, not competitors — flowy is a self-hosted engine.

| Feature | Terra (Cromwell) | Nextflow Tower (Seqera) | WDL Workspace (EPAM) | flowy v2.1 approach |
|---------|------------------|--------------------------|----------------------|---------------------|
| Workflow selection | Browse workspace, select WDL | Select pipeline from catalog | Upload or select from library | Select from project registry |
| Parameter input | JSON template + form hybrid | Schema-driven form (nf-core schema) | JSON input fields | JSON textarea first; typed form as differentiator |
| Submit action | "Launch" button, async | "Launch" button, async | "Run" button | Submit button then redirect to run detail |
| Status monitoring | Table with polling, colored states | Real-time via WebSocket | Table with status polling | 3s polling, status badge in detail page |
| CLI registry | `dockstore tool publish` (GitHub-linked) | `nextflow` + Tower API | None | `flowy registry push/pull/list/tag/delete-tag` |
| File upload | Via Terra workspace files | Via datasets feature | Via upload form | Single WDL file upload dialog |

---

## Sources

- Codebase analysis: `/home/uehara/flowy/internal/api/types.go`, `/home/uehara/flowy/internal/server/handlers.go`, `/home/uehara/flowy/internal/server/handlers_registry.go`, `/home/uehara/flowy/cmd/flowy/main.go`, `/home/uehara/flowy/web/src/features/runs/`, `/home/uehara/flowy/web/src/features/registry/`
- Terra/Cromwell parameter input: https://support.terra.bio/hc/en-us/articles/360037120252-Specify-inputs
- Seqera Platform (Nextflow Tower) monitoring: https://docs.seqera.io/platform-cloud/monitoring/overview
- WDL Workspace (comparable open-source UI): https://github.com/epam/wdl-workspace
- AWS HealthOmics WDL parameter detection (2025): https://aws.amazon.com/about-aws/whats-new/2025/06/aws-healthomics-automatic-detection-wdl-workflow-parameters/
- Argo Workflows UI parameter input: https://argo-workflows.readthedocs.io/en/latest/workflow-inputs/
- PROJECT.md milestone scope: `/home/uehara/flowy/.planning/PROJECT.md`

---
*Feature research for: WDL workflow execution UI, CLI registry management, browser-based WDL upload (flowy v2.1)*
*Researched: 2026-04-07*
