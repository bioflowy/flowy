# Architecture Research

**Domain:** Workflow execution UI, CLI registry commands, browser-based registry upload — flowy v2.1
**Researched:** 2026-04-07
**Confidence:** HIGH

---

## Context: What Is Being Added

v2.1 adds three distinct capability clusters to the existing four-binary system:

1. **UI Workflow Execution** — Select a registry entry from the browser, fill an input form,
   submit to run, and poll for status/results. Requires a new server endpoint to expose WDL input
   schemas (type + name + optionality) without requiring the browser to parse WDL itself.

2. **CLI Registry Commands** — `flowy registry push|list|pull|tag|delete` subcommand group. The
   server already has all registry REST endpoints (`handleRegistryPush`, `handleRegistryPull`,
   `handleRegistryList`, `handleRegistryListVersions`, `handleRegistryTag`,
   `handleRegistryDelete`, `handleRegistryDeleteTag`). The CLI package currently has no registry
   client methods. Only the CLI layer needs to be added.

3. **UI Registry Upload** — A "Push WDL" dialog in the `RegistryPage` that lets an executor
   role user select a local WDL file, enter a name, and POST to `POST /projects/{slug}/registry`.
   The server already accepts this request. Only the frontend needs to be added.

No new backend persistence, no schema changes, no new daemon or binary are required.

---

## System Overview

```
┌──────────────────────────────────────────────────────────────────────────────┐
│  Browser (React SPA)                                                          │
│                                                                               │
│  NEW: RunExecutionDialog — input form (dynamic per WDL types)                │
│  NEW: UploadWDLDialog — file picker + name input                             │
│  MOD: RegistryPage — "Push WDL" button for executors                        │
│  MOD: RegistryEntryPage — "Run" button per tag/version                      │
│  NEW: useRunSubmit() mutation hook                                            │
│  NEW: useRegistryPush() mutation hook                                         │
│  NEW: useRegistryInputSchema() query hook → GET /projects/{slug}/           │
│        registry/{name}/inputs?tag=...                                         │
│  MOD: useRuns() — refetchInterval for active runs                            │
└──────────────────────────────┬───────────────────────────────────────────────┘
                               │ HTTP /api/*
                               ▼
              ┌────────────────────────────┐
              │  flowy-ui binary (UNCHANGED) │
              │  Reverse proxy /api/* only   │
              └──────────────┬─────────────┘
                             │
                             ▼
┌────────────────────────────────────────────────────────────────────────────┐
│  flowy-server                                                               │
│                                                                             │
│  EXISTING (unchanged): POST /projects/{slug}/runs/submit                   │
│  EXISTING (unchanged): GET  /projects/{slug}/runs/{id}/status              │
│  EXISTING (unchanged): GET  /projects/{slug}/runs/{id}/outputs             │
│  EXISTING (unchanged): POST /projects/{slug}/registry (push)               │
│  EXISTING (unchanged): GET  /projects/{slug}/registry                      │
│  EXISTING (unchanged): GET  /projects/{slug}/registry/{name}               │
│  EXISTING (unchanged): GET  /projects/{slug}/registry/{name}/versions      │
│  EXISTING (unchanged): POST /projects/{slug}/registry/{name}/tags          │
│  EXISTING (unchanged): DELETE /projects/{slug}/registry/{name}/tags/{tag}  │
│  EXISTING (unchanged): DELETE /projects/{slug}/registry/{name}             │
│                                                                             │
│  NEW: GET /projects/{slug}/registry/{name}/inputs?tag=<tag>               │
│        Returns: {inputs: [{name, type, optional, has_default}]}            │
│        Implementation: resolve tag → WDL content → parse → reflect inputs │
└────────────────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────────────────┐
│  flowy CLI (cmd/flowy/main.go)                                              │
│                                                                             │
│  EXISTING: check, run, submit, status, outputs                             │
│                                                                             │
│  NEW: registry subcommand group:                                           │
│    flowy registry push   --project <slug> --tag <tag> <file.wdl>           │
│    flowy registry list   --project <slug>                                  │
│    flowy registry pull   --project <slug> --tag <tag>  <name>              │
│    flowy registry tag    --project <slug> --digest <d> <name> <tag>        │
│    flowy registry delete --project <slug> <name>                           │
│                                                                             │
│  internal/client/client.go — NEW registry methods:                         │
│    RegistryPush, RegistryList, RegistryPull, RegistryTag, RegistryDelete   │
└────────────────────────────────────────────────────────────────────────────┘
```

---

## Component Responsibilities

### New and Modified Components

| Component | Status | Responsibility |
|-----------|--------|----------------|
| `GET /projects/{slug}/registry/{name}/inputs` | NEW endpoint | Parse WDL for the named version, return input schema (name, type string, optional flag, has_default flag) |
| `handleRegistryInputs` in `handlers_registry.go` | NEW handler | Resolve tag/digest → WDL content → `parser.Parse` → extract `Workflow.Inputs` or `Task.Inputs` → serialize |
| `internal/client/client.go` | MODIFIED | Add `RegistryPush`, `RegistryList`, `RegistryPull`, `RegistryTag`, `RegistryDelete`, `RegistryDeleteTag`, `RegistryListVersions` methods |
| `cmd/flowy/main.go` `run()` switch | MODIFIED | Add `"registry"` case dispatching to `runRegistry()` |
| `cmd/flowy/registry.go` (new file) | NEW | `runRegistry()` dispatcher + `runRegistryPush`, `runRegistryList`, `runRegistryPull`, `runRegistryTag`, `runRegistryDelete` implementations |
| `web/src/features/registry/useRegistry.ts` | MODIFIED | Add `useRegistryInputSchema()` query, `useRegistryPush()` mutation |
| `web/src/features/registry/RegistryPage.tsx` | MODIFIED | Add "Push WDL" button (executor+), `UploadWDLDialog` integration |
| `web/src/features/registry/UploadWDLDialog.tsx` | NEW | File picker, name input, POST to registry, close on success |
| `web/src/features/registry/RegistryEntryPage.tsx` | MODIFIED | Add "Run" button on each version row (executor+), launch `RunExecutionDialog` |
| `web/src/features/runs/RunExecutionDialog.tsx` | NEW | Load input schema, render dynamic form, POST to submit, navigate to run page on success |
| `web/src/features/runs/WDLInputForm.tsx` | NEW | Render form fields per input type: String/Int/Float → text, Boolean → checkbox, File → text, complex → JSON textarea |
| `web/src/features/runs/useRuns.ts` | MODIFIED | `useRunStatus()` hook with `refetchInterval` for live polling on active runs |
| `web/src/App.tsx` | UNCHANGED | Routing unchanged; RunExecutionDialog is a modal, not a route |

### Unchanged Components

All four binaries keep their existing shape. `flowy-server` gets one new handler and one new route
registration. `flowy-daemon`, `flowy-ui`, and `flowy-server`'s existing routes are untouched.

---

## New Endpoint: GET `/projects/{slug}/registry/{name}/inputs`

### Purpose

The browser needs to know the input declarations of a WDL workflow/task before asking the user
to fill them in. The server already stores the raw WDL content in the registry. This endpoint
parses that content and returns a structured list of inputs with type information.

### Query Parameters

- `?tag=<tag>` — resolve by tag name (e.g., `?tag=latest`)
- `?digest=<sha256:hex>` — resolve by digest (consistent with `handleRegistryPull`)

### Response Body

```json
{
  "name": "my-workflow",
  "kind": "workflow",
  "inputs": [
    {"name": "sample_name", "type": "String",  "optional": false, "has_default": false},
    {"name": "threads",     "type": "Int",     "optional": false, "has_default": true },
    {"name": "output_dir",  "type": "String",  "optional": true,  "has_default": false},
    {"name": "ref_fasta",   "type": "File",    "optional": false, "has_default": false}
  ]
}
```

### Implementation

```go
// handlers_registry.go — new handler
func (s *Server) handleRegistryInputs(w http.ResponseWriter, r *http.Request) {
    proj := projectFromContext(r.Context())
    name := chi.URLParam(r, "name")
    tag := r.URL.Query().Get("tag")
    digest := r.URL.Query().Get("digest")
    // Resolve to WDL content using existing resolveRegistryRefFull logic...
    // Parse with parser.Parse(name, []byte(wdlContent))
    // Extract inputs from doc.Workflows[0].Inputs or doc.Tasks[0].Inputs
    // Serialize each Decl as {name, type: decl.Type.TypeName(), optional: decl.Type.IsOptional(), has_default: decl.Expr != nil}
}
```

Route registration in `server.go`:

```go
// Add inside the existing registry/{name} route group:
r.With(s.projectRoleRequired("viewer")).Get("/inputs", s.handleRegistryInputs)
```

### Why This Pattern

Parsing WDL in the browser is not feasible without a WASM port of the Go parser. Returning
pre-parsed type metadata is the standard pattern used by workflow managers (e.g., Cromwell's
`/api/workflows/v1/backends`, Nextflow Tower's form generation). The endpoint is cheap:
WDL content is already in SQLite, parsing takes <1ms for typical files.

---

## Data Flows

### UI Workflow Execution Flow

```
1. User on RegistryEntryPage clicks "Run" on a version row
       ↓
2. RunExecutionDialog opens, receives {name, digest, tag}
       ↓
3. useRegistryInputSchema(slug, name, {tag}) fires
   → GET /api/projects/{slug}/registry/{name}/inputs?tag=latest
   → flowy-server: resolve tag → parse WDL → return input schema
       ↓
4. WDLInputForm renders per-input fields:
   String/Int/Float → <input type="text"> with type coercion on submit
   Boolean → <input type="checkbox">
   File → <input type="text"> (user enters path or s3:// URI)
   Array/Map/Struct → <textarea> (JSON input with parse validation)
       ↓
5. User fills form, clicks "Run"
       ↓
6. useRunSubmit() mutation fires
   → POST /api/projects/{slug}/runs/submit
     Body: {registry_ref: "name:tag", inputs: {k: v, ...}}
   → flowy-server: handleSubmit (EXISTING) → creates run → returns {run_id}
       ↓
7. Navigate to /projects/{slug}/runs/{run_id}
       ↓
8. RunDetailPage polls via useRunStatus(runID, refetchInterval: 2000)
   → GET /api/projects/{slug}/runs/{run_id}/status (EXISTING)
   → Stops polling when state is COMPLETED/FAILED/ABORTED
       ↓
9. On COMPLETED: useRunOutputs fires (EXISTING) → displays output table
```

### Browser Registry Upload Flow

```
1. Executor clicks "Push WDL" button on RegistryPage
       ↓
2. UploadWDLDialog opens
       ↓
3. User selects .wdl file via <input type="file">, enters name (slug format)
       ↓
4. useRegistryPush() mutation fires
   → POST /api/projects/{slug}/registry
     Body: {name: "my-tool", content: "<wdl file text>"}
     (apiClient reads file via FileReader API — no multipart; content is base-64-free plain text)
   → flowy-server: handleRegistryPush (EXISTING) → parse → hash → store → return {digest, kind}
       ↓
5. Dialog closes, toast success, invalidate useRegistryEntries query → table refreshes
```

Note: The existing `handleRegistryPush` accepts `{name, content}` as JSON. The browser reads
the WDL file as text with `FileReader.readAsText()` and sends it as a JSON string field. No
multipart upload, no new server endpoint needed.

### CLI Registry Push Flow

```
1. flowy registry push --project my-project --tag latest ./tool.wdl
       ↓
2. runRegistryPush reads file from disk → os.ReadFile()
       ↓
3. Authenticates via JWT (reads token from config or --token flag)
       ↓
4. client.RegistryPush(ctx, projectSlug, api.RegistryPushRequest{Name: name, Content: content})
   → POST /projects/{slug}/registry
   → Returns {digest, kind, version_id}
       ↓
5. If --tag flag set: client.RegistryTag(ctx, projectSlug, name, tag, digest)
   → POST /projects/{slug}/registry/{name}/tags
       ↓
6. Print "Pushed {name}@{digest} (kind: {kind})"
```

### CLI Registry Pull Flow

```
1. flowy registry pull --project my-project --tag latest my-tool
       ↓
2. client.RegistryPull(ctx, projectSlug, name, tag, "")
   → GET /projects/{slug}/registry/{name}?tag=latest
   → Returns {name, digest, kind, content}
       ↓
3. Write content to stdout or to --output file
```

---

## Authentication Wiring for CLI Registry Commands

The CLI currently has no authentication handling. The existing `Submit`, `Status`, `Outputs`
client methods do not send JWT tokens. The registry endpoints require `executor` role for push
and `viewer` role for list/pull.

### Solution

Add `WithToken(token string)` method or constructor option to `internal/client/client.go`.
Load the JWT from config file (`client.token` field in `flowy.yaml`) or from a `--token` flag.

```go
// internal/client/client.go
type Client struct {
    serverURL  string
    httpClient *http.Client
    token      string // NEW: JWT for authenticated requests
}

func (c *Client) WithToken(token string) *Client {
    c2 := *c
    c2.token = token
    return &c2
}

func (c *Client) addAuthHeader(req *http.Request) {
    if c.token != "" {
        req.Header.Set("Authorization", "Bearer "+c.token)
    }
}
```

Alternatively, add a `token` field to `config.ClientConfig` so existing `run`/`submit`
subcommands can also benefit when auth is enabled on the server. This is a clean backward-
compatible extension — no change to callers that don't set a token.

---

## WDLInputForm Field Rendering Rules

The `WDLInputForm` component needs a consistent mapping from WDL type names to form controls.
The server returns `type` as the canonical WDL type string (e.g., `"String"`, `"Int?"`, `"Array[String]"`).

| WDL Type Pattern | Form Control | Submission Value |
|-----------------|--------------|-----------------|
| `String`, `String?` | `<input type="text">` | string |
| `Int`, `Int?` | `<input type="number" step="1">` | parseInt |
| `Float`, `Float?` | `<input type="number" step="any">` | parseFloat |
| `Boolean`, `Boolean?` | `<input type="checkbox">` | boolean |
| `File`, `File?` | `<input type="text" placeholder="path or s3://...">` | string |
| `Directory`, `Directory?` | `<input type="text" placeholder="path or s3://...">` | string |
| Anything else (Array, Map, Struct) | `<textarea placeholder="JSON value">` | JSON.parse |

Optional inputs (`type.optional === true`) show "(optional)" label and allow empty submission
(omit key from inputs map rather than sending null).

Inputs with `has_default === true` show "(default)" note and can be left empty to use default.

---

## Polling Strategy for Run Status

The `RunDetailPage` currently loads run data via `useRuns()` (list query) and derives the
individual run. For v2.1, when a user navigates directly to a run page after submitting, it
needs real-time status updates without a full page reload.

### Approach: `refetchInterval` in TanStack Query

```typescript
// web/src/features/runs/useRuns.ts — add new hook
export function useRunStatus(slug: string, id: string) {
  const { data: runs } = useRuns(slug)
  const run = runs?.find((r) => r.id === id)
  const isActive = run?.state === 'PENDING' || run?.state === 'RUNNING'

  // Refetch the runs list every 2 seconds when the run is active
  return useQuery<RunListItem[]>({
    queryKey: ['projects', slug, 'runs'],
    queryFn: () => apiClient.get(`/projects/${slug}/runs`),
    refetchInterval: isActive ? 2000 : false,
    enabled: !!slug,
  })
}
```

This reuses the existing `GET /projects/{slug}/runs` list endpoint (returns all runs with state).
No new endpoint needed. The interval stops automatically when the run reaches a terminal state.

2-second polling is consistent with the existing CLI's 500ms polling (CLI is more aggressive
because it is transient; the browser window may stay open for hours).

---

## Project Structure Changes

```
flowy/
├── cmd/
│   └── flowy/
│       ├── main.go              MODIFIED: add "registry" case to run() switch
│       └── registry.go          NEW: runRegistry(), runRegistryPush(), runRegistryList(),
│                                     runRegistryPull(), runRegistryTag(), runRegistryDelete()
├── internal/
│   ├── api/
│   │   └── types.go             MODIFIED: add RegistryInputsResponse, RegistryInputDecl types
│   ├── client/
│   │   └── client.go            MODIFIED: add registry methods + WithToken()
│   └── server/
│       ├── server.go            MODIFIED: add /inputs route in registry/{name} group
│       └── handlers_registry.go MODIFIED: add handleRegistryInputs
└── web/src/features/
    ├── registry/
    │   ├── RegistryPage.tsx     MODIFIED: add "Push WDL" button + UploadWDLDialog
    │   ├── RegistryEntryPage.tsx MODIFIED: add "Run" button + RunExecutionDialog
    │   ├── UploadWDLDialog.tsx   NEW: file picker + push mutation
    │   └── useRegistry.ts       MODIFIED: add useRegistryInputSchema(), useRegistryPush()
    └── runs/
        ├── RunDetailPage.tsx    MODIFIED: use refetchInterval for active run polling
        ├── RunExecutionDialog.tsx NEW: input form + submit mutation
        ├── WDLInputForm.tsx     NEW: dynamic field rendering per WDL type
        └── useRuns.ts           MODIFIED: add useRunStatus() with refetchInterval
```

---

## Integration Points

### Existing Architecture Touch Points

| Existing Component | How v2.1 Integrates |
|-------------------|---------------------|
| `handleRegistryPush` (POST /registry) | Browser upload calls this directly via existing JSON contract |
| `handleSubmit` (POST /runs/submit) | UI execution calls this; `registry_ref` field already supported |
| `handleStatus` (GET /runs/{id}/status) | UI polling calls this unchanged |
| `handleOutputs` (GET /runs/{id}/outputs) | UI result display calls this unchanged |
| `handleRegistryList` (GET /registry) | CLI `registry list` calls this |
| `handleRegistryPull` (GET /registry/{name}) | CLI `registry pull` calls this |
| `handleRegistryTag` (POST /registry/{name}/tags) | CLI `registry tag` calls this |
| `handleRegistryDelete` (DELETE /registry/{name}) | CLI `registry delete` calls this |
| `handleRegistryDeleteTag` (DELETE /registry/{name}/tags/{tag}) | CLI `registry delete-tag` calls this |
| WDL `parser.Parse` | `handleRegistryInputs` reuses parser to extract input schema |
| `resolveRegistryRefFull` | `handleRegistryInputs` reuses this to resolve tag/digest → content |

### RBAC Boundaries for New Features

| Feature | Required Role | Why |
|---------|--------------|-----|
| Registry push (browser) | executor | Matches existing POST /registry permission |
| Registry pull (CLI) | viewer | Matches existing GET /registry/{name} permission |
| Registry list (CLI) | viewer | Matches existing GET /registry permission |
| Registry tag (CLI) | admin | Matches existing POST /registry/{name}/tags permission |
| Registry delete (CLI) | admin | Matches existing DELETE /registry/{name} permission |
| Get input schema | viewer | Read-only; same level as pull |
| Submit run from UI | executor | Matches existing POST /runs/submit permission |

No new RBAC roles or permission changes required.

---

## Build Order

Dependencies drive this order. Each step has a clear gate.

```
Step 1: New API type definitions
  internal/api/types.go — RegistryInputDecl, RegistryInputsResponse
  Gate: types compile cleanly

Step 2: Server endpoint for input schema
  internal/server/handlers_registry.go — handleRegistryInputs
  internal/server/server.go — add /inputs route
  Gate: GET /projects/test/registry/my-wf/inputs?tag=latest returns input JSON

Step 3: Client library registry methods
  internal/client/client.go — RegistryPush, RegistryList, RegistryPull, RegistryTag,
                               RegistryDelete, RegistryListVersions + WithToken()
  Gate: client_test.go or integration test covers each method

Step 4: CLI registry subcommand
  cmd/flowy/registry.go — new file with all subcommand implementations
  cmd/flowy/main.go — add "registry" dispatch case
  Gate: flowy registry push --project demo tool.wdl works end-to-end

Step 5: Browser registry upload
  web/src/features/registry/UploadWDLDialog.tsx — new component
  web/src/features/registry/useRegistry.ts — useRegistryPush mutation
  web/src/features/registry/RegistryPage.tsx — "Push WDL" button (executor only)
  Gate: Upload dialog pushes WDL, entry appears in registry table

Step 6: Browser workflow execution
  web/src/features/runs/WDLInputForm.tsx — new component (type-driven fields)
  web/src/features/runs/RunExecutionDialog.tsx — new component
  web/src/features/registry/RegistryEntryPage.tsx — "Run" button on version rows
  web/src/features/registry/useRegistry.ts — useRegistryInputSchema query
  web/src/features/runs/useRuns.ts — useRunStatus with refetchInterval
  web/src/features/runs/RunDetailPage.tsx — refetch when run is active
  Gate: Select registry entry → fill form → submit → poll → see outputs
```

Step 1-3 are pure backend with no frontend dependency. Step 4 (CLI) depends only on Step 3.
Steps 5-6 (frontend) depend on Step 2 for the input schema endpoint. Steps 5 and 6 are
independent of each other — Step 5 (upload) can be built before Step 6 (execution).

---

## Anti-Patterns to Avoid

### Anti-Pattern 1: WDL Parsing in the Browser

**What people do:** Attempt to parse WDL content (already available via `handleRegistryPull`)
in TypeScript to extract input types.

**Why it's wrong:** The WDL parser is 5,000+ lines of Go. There is no TypeScript WDL parser.
Implementing one in the browser is out of scope and would duplicate a complex, tested codebase.

**Do this instead:** `GET /registry/{name}/inputs?tag=...` — let the server parse WDL and return
structured metadata. One new endpoint, zero duplication.

### Anti-Pattern 2: Polling via setInterval in a Component

**What people do:** `useEffect(() => { const id = setInterval(pollStatus, 2000); ... }, [])`

**Why it's wrong:** Component unmount cancels the interval but the interval fires regardless of
whether the query is in flight, causing overlapping requests. React Strict Mode double-invokes
effects, creating duplicate intervals in development.

**Do this instead:** `refetchInterval` in TanStack Query. The library handles backoff, dedup,
and cleanup automatically. The interval pauses when the window is hidden (via `refetchIntervalInBackground: false`).

### Anti-Pattern 3: Reading WDL File as Binary / FormData for Upload

**What people do:** Use `<input type="file">` with a `FormData` POST (multipart/form-data).

**Why it's wrong:** `handleRegistryPush` accepts `application/json` with `{name, content}`. The
existing contract sends WDL as a plain string field. Multipart would require a new parser on
the server and a different content type.

**Do this instead:** Use `FileReader.readAsText(file)` to read the WDL file as a UTF-8 string,
then POST as JSON `{name: enteredName, content: wdlText}`. This is what the CLI already does
(`os.ReadFile` → string).

### Anti-Pattern 4: Token Storage in CLI Without Config

**What people do:** Require users to pass `--token <jwt>` on every CLI registry command.

**Why it's wrong:** JWTs are long (hundreds of characters). Typing or copy-pasting on every
command defeats the ergonomics of a CLI tool.

**Do this instead:** Store token in `flowy.yaml` under `client.token`. Add a `flowy login`
command (or reuse `flowy registry push --login`) that authenticates and writes the token to
the config file. The token field is already present as a natural extension of the existing
`ClientConfig` struct.

### Anti-Pattern 5: Navigation on Submit Instead of Modal

**What people do:** Create a separate `/projects/{slug}/registry/{name}/run` route for the
execution form.

**Why it's wrong:** The execution form is lightweight (one dialog). A separate route adds
browser history entries and requires a full page load. After submission the user is immediately
redirected to the run page anyway.

**Do this instead:** `RunExecutionDialog` as a modal triggered from `RegistryEntryPage`. On
successful submit, `navigate('/projects/${slug}/runs/${runId}')`. Stays in the established
shadcn/ui Dialog pattern used throughout the existing UI.

---

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| Lab / small team | Current architecture is fine; polling at 2s intervals adds negligible load |
| Many concurrent browser sessions polling | `GET /runs` returns all project runs (could grow large); consider `GET /runs/{id}/status` per-run polling if list grows beyond ~1000 items |
| Very large WDL files (>1MB) | Input schema endpoint parses WDL on every call; consider caching parsed schema in memory by digest (immutable content = safe cache key) |

Polling at 2-second intervals from the browser calls `GET /projects/{slug}/runs` which is a
single SQLite read. This is not a bottleneck at lab scale. For larger deployments,
per-run status (`GET /runs/{id}/status`) is lighter but requires the run ID to be known, which
it is after submission.

---

## Sources

- Direct codebase analysis — `internal/server/handlers_registry.go`, `internal/server/server.go`,
  `internal/api/types.go`, `cmd/flowy/main.go`, `internal/client/client.go`,
  `web/src/features/registry/`, `web/src/features/runs/`, `wdl/ast/ast.go`, `wdl/types/types.go`
  (HIGH confidence — primary source)
- TanStack Query `refetchInterval` documentation:
  https://tanstack.com/query/latest/docs/framework/react/guides/query-options#refetchinterval
  (HIGH confidence — official docs)
- FileReader.readAsText MDN documentation:
  https://developer.mozilla.org/en-US/docs/Web/API/FileReader/readAsText
  (HIGH confidence — official Web API docs)
- Cromwell input schema pattern (Swagger UI generates forms from WDL metadata):
  https://cromwell.readthedocs.io/en/stable/api/RESTAPI/
  (MEDIUM confidence — architectural reference only)

---

*Architecture research for: flowy v2.1 — Workflow execution UI, CLI registry commands, browser registry upload*
*Researched: 2026-04-07*
