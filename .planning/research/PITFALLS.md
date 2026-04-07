# Pitfalls Research

**Domain:** Adding workflow execution UI, CLI registry commands, and browser WDL upload to flowy v2.1
**Researched:** 2026-04-07
**Confidence:** HIGH (based on direct codebase inspection of all integration points, ADR 0001, and cross-milestone retrospective lessons)

---

## Critical Pitfalls

### Pitfall 1: File Inputs Need Upload-Then-Reference, Not Inline Data

**What goes wrong:**
The browser form collects a WDL `File`-typed input. The developer stores the browser `File` object and passes its local path string (e.g., `"C:\Users\alice\data.bam"`) or a data URL to the `InputsJSON` of `POST /projects/{slug}/workflows`. The server receives the path but the daemon has no access to the user's local machine — it tries to resolve the path on the shared filesystem and fails with "file not found" or silently passes an empty file to the container.

**Why it happens:**
Every other WDL primitive (String, Int, Float, Boolean) serialises cleanly into JSON and round-trips through `InputsJSON` without issue. The `File` type looks like a string in JSON — it is a string containing a path. The distinction between "a path that exists on the server's filesystem" and "a path that existed on the user's browser machine" is invisible until the daemon tries to localize the file and finds nothing.

ADR 0001 explicitly removed heuristic file staging from the CLI client because the client cannot reliably detect File-typed inputs. The browser UI faces the same constraint: it must upload first and substitute the resulting server-side path before submitting.

**How to avoid:**
Implement a two-step flow for `File` inputs:
1. Upload the file to the server via a dedicated endpoint (e.g., `POST /projects/{slug}/uploads`) that stores it under `base_dir/runs/{run_id}/inputs/` and returns the server-side absolute path.
2. Pre-generate the run UUID client-side (the submit request already supports `run_id` in `api.SubmitRequest`) so the staging path is deterministic before submission.
3. Substitute the returned server-side path into the inputs map before calling `POST /projects/{slug}/workflows`.

Alternatively, for the MVP scope where the server uses local storage, stage the file at a well-known location and pass its absolute path. For S3 backends, the upload endpoint must put the file into S3 and return the `s3://bucket/prefix/...` URI.

**Warning signs:**
- `File`-typed inputs are treated identically to `String` inputs in the form submission code
- The run is submitted but the daemon log shows "no such file or directory" for every File input
- No upload endpoint exists before the execution UI phase ships

**Phase to address:**
Phase that implements the execution UI dynamic input form — do not build the `File` input widget until the upload endpoint exists. Block `File` widget on upload infrastructure.

---

### Pitfall 2: WDL Input Schema Must Be Fetched from the Registry, Not Inferred Client-Side

**What goes wrong:**
The developer builds the dynamic form by fetching the WDL source string from `GET /projects/{slug}/registry/{name}?digest=...` and parsing it in the browser using a regex or ad-hoc string scan. The parser misses optional inputs (those declared with `?`), defaults (inputs with `= expression`), and compound types (Array, Map, Struct). The form shows the wrong fields, marks required inputs as optional, or omits inputs entirely.

**Why it happens:**
The WDL content is available as a plain string from the existing registry pull endpoint. Writing a quick regex to extract `input { ... }` blocks seems faster than adding a backend endpoint. But WDL's grammar for declarations is non-trivial: types can nest (e.g., `Array[Map[String, File]]?`), default expressions can span multiple lines, and struct members are not inline. The Go WDL parser already handles all of this correctly.

**How to avoid:**
Add a backend endpoint (e.g., `GET /projects/{slug}/registry/{name}/inputs?digest=...`) that parses the WDL using the existing Go parser, walks `doc.Workflows[0].Inputs` (and `doc.Tasks[0].Inputs` for tools), and returns a structured JSON schema per input:

```json
[
  { "name": "sample_name", "type": "String", "optional": false, "has_default": false },
  { "name": "reference",   "type": "File",   "optional": false, "has_default": false },
  { "name": "threads",     "type": "Int",    "optional": true,  "has_default": true  }
]
```

This reuses `wdl/parser.Parse` and `ast.Decl.Type` / `ast.Decl.Expr` — no new parsing logic is needed. The frontend receives a flat schema and renders the correct widget per type.

**Warning signs:**
- The frontend imports a WDL parsing library or does string matching on WDL content
- Input forms appear correct for the simplest WDL but fail on workflows with optional inputs or default expressions
- Struct or Map inputs appear as plain text fields instead of JSON editors

**Phase to address:**
Phase that implements the dynamic input form — add the `/inputs` endpoint before building the form component.

---

### Pitfall 3: Poll-Based Status Runs Forever if TanStack Query Refetch Is Not Gated on Terminal State

**What goes wrong:**
The developer uses TanStack Query's `refetchInterval` option to poll `GET /projects/{slug}/runs/{id}/status` every N seconds. The interval runs indefinitely, including after the run reaches `COMPLETED`, `FAILED`, or `ABORTED`. Every open run detail tab keeps polling the server indefinitely, creating unnecessary load and triggering re-renders on stale data.

**Why it happens:**
`refetchInterval: 3000` is the obvious one-liner for polling. Stopping the interval requires passing a function that checks the query data and returns `false` once a terminal state is seen. Developers copy the simple number form from examples and skip the conditional.

**How to avoid:**
Pass a function to `refetchInterval` that inspects the query data:

```typescript
const TERMINAL_STATES = new Set(['COMPLETED', 'FAILED', 'ABORTED'])

useQuery({
  queryKey: ['projects', slug, 'runs', id, 'status'],
  queryFn: () => apiClient.get(`/projects/${slug}/runs/${id}/status`),
  refetchInterval: (query) => {
    const state = (query.state.data as StatusResponse | undefined)?.state
    return state && TERMINAL_STATES.has(state) ? false : 3000
  },
})
```

Also gate the `useRunOutputs` query on `run.state === 'COMPLETED'` — this pattern already exists in `RunDetailPage.tsx` and must be followed consistently in all new polling code.

The existing `useRuns` hook (run list) already refetches on window focus (TanStack Query default). For the run detail page, suppress window-focus refetch after terminal state: `refetchOnWindowFocus: false` once the run is in a terminal state.

**Warning signs:**
- `refetchInterval` is set to a number literal (e.g., `3000`) without a conditional
- The network tab shows continuous polling calls for runs showing `COMPLETED`
- Closing and reopening a run detail tab triggers a new burst of poll requests for old runs

**Phase to address:**
Phase that implements run status monitoring UI — write the conditional `refetchInterval` before connecting the polling query. Verify by running a workflow to completion and confirming the network tab shows no further polls after `COMPLETED`.

---

### Pitfall 4: CLI Registry Commands Need Authentication — Credential Storage Must Be Designed First

**What goes wrong:**
The CLI `registry push/pull/list` commands talk to the same `/projects/{slug}/registry` REST endpoints. The developer adds a `--token` flag and hardcodes it into request headers. Users must pass `--token <jwt>` on every command, which is unusable for interactive use. Alternatively, the developer stores the token in the config file under `client.token` — but this puts a long-lived JWT into a plaintext config file, which leaks credentials if the config is checked into version control.

**Why it happens:**
The existing CLI client (`internal/client`) has no authentication layer — all prior CLI commands either hit auth-disabled routes or were designed before JWT was added. Adding registry commands to the CLI means these commands hit JWT-protected endpoints for the first time. The developer reaches for the simplest fix (flag or config field) without thinking through the credential lifecycle.

**How to avoid:**
Add a `flowy login` command that calls `POST /auth/login` and writes the token to a separate credentials file (e.g., `~/.flowy/credentials.yaml`), not `./flowy.yaml`. All registry commands load the token from the credentials file if present, with `--token` as an override for scripting and CI.

The credentials file location should follow `$FLOWY_CREDENTIALS_FILE` env var or default to `~/.flowy/credentials.yaml`. Write the file with mode 0600. Document that `./flowy.yaml` should never contain a token field.

Extend `internal/client.Client` to carry an optional token and inject `Authorization: Bearer <token>` on all requests. Registry commands pass the loaded token to `client.New`.

**Warning signs:**
- The `--token` flag appears on `flowy registry push` and every subsequent subcommand
- `flowy.yaml` adds a `client.token` or `auth.token` field
- The credentials file lives under the working directory (readable by project collaborators)
- Registry commands silently receive 401 and return cryptic errors because no token is loaded

**Phase to address:**
Phase that implements CLI registry commands — implement `flowy login` and credential loading first, before any `registry` subcommand.

---

### Pitfall 5: Browser Upload Sends Content as JSON String — Requires Text Reading, Not Binary

**What goes wrong:**
The browser upload component reads the selected file using `FileReader.readAsArrayBuffer` (the default for general file uploads) and sends the raw bytes in the request body. The server expects `RegistryPushRequest.Content` as a UTF-8 text string (JSON field). The request fails with a JSON decode error or, worse, the binary buffer is silently coerced to a string containing garbage bytes that the WDL parser rejects.

A related mistake: the developer uses `<input type="file">` with `accept="*"` and the user selects a binary file (`.bam`, `.fastq`). The server receives binary content in a field designed for WDL text, the parser returns 422, but there is no user-facing explanation that WDL text is required.

**Why it happens:**
The `RegistryPushRequest` passes WDL content as a plain JSON string. WDL files are UTF-8 text, so `FileReader.readAsText` is the correct API — but developers often default to `readAsArrayBuffer` for "file upload" tasks and then try to encode the result. The browser file upload pattern learned from image or binary file upload scenarios is wrong for text content.

**How to avoid:**
Read the file as text before submitting:
```typescript
const content = await file.text()  // or FileReader.readAsText
await apiClient.post(`/projects/${slug}/registry`, { name, content })
```

Validate file extension client-side: only accept `.wdl` files. Show a clear error if the user selects a non-WDL file. The server will reject non-parseable content with 422, but client-side validation gives a better error message before the round-trip.

**Warning signs:**
- The upload component uses `readAsArrayBuffer` or `URL.createObjectURL`
- The file input `accept` attribute allows all file types
- `422 Unprocessable Entity` is the first error users see when they accidentally select a binary file

**Phase to address:**
Phase that implements browser WDL upload — validate file type and use `file.text()` in the upload handler component.

---

### Pitfall 6: `handleOutputs` Returns 409 for Non-Completed Runs — Breaks Polling Flow If Misused

**What goes wrong:**
The existing `handleOutputs` handler returns `409 Conflict` if the run is not yet `COMPLETED`. If the developer creates a polling hook that fetches status and outputs in the same query (or calls the outputs endpoint on every poll tick regardless of status), TanStack Query receives the 409, interprets it as an error via `ApiError`, shows an error state, and stops retrying. The user sees "Error" instead of a loading indicator while the run is still active.

**Why it happens:**
The status endpoint and the outputs endpoint are separate. Polling outputs before the run completes is an anti-pattern but easy to trigger if the developer combines both concerns in a single polling loop or adds an optimistic outputs fetch "just in case" the run completes quickly.

**How to avoid:**
Follow the pattern already established in `RunDetailPage.tsx`:
```typescript
const { data: outputsData } = useRunOutputs(
  slug ?? '',
  run?.state === 'COMPLETED' ? (id ?? '') : ''
)
```
The empty string `enabled` guard prevents the outputs query from firing until `COMPLETED`. In all new execution UI code, apply the same gate. Do not call the outputs endpoint until the status poll confirms `COMPLETED`.

**Warning signs:**
- `useRunOutputs` is called with the run ID unconditionally
- The network tab shows `GET .../outputs` requests returning 409 while the run is `RUNNING`
- The run detail page shows an error state for active runs

**Phase to address:**
Phase that implements run status monitoring UI — enforce the outputs-gating pattern from the start. Verify via manual test: start a run, open the detail page while it is running, confirm no 409 error state appears.

---

### Pitfall 7: `flowy registry push` Must Read File Content Locally — Do Not Pass the Path to the Server

**What goes wrong:**
The CLI `flowy registry push my-workflow:latest ./workflow.wdl` sends the local file path as `Content` in the `RegistryPushRequest` JSON body. The server receives a path string, tries to classify it as WDL content by parsing it, the Go parser fails to parse a filesystem path as WDL syntax, and the server returns `422 Unprocessable Entity`. Alternatively, the developer adds a `Path` field to `RegistryPushRequest` and server-side file reading — but the server cannot access the CLI user's local filesystem (especially in distributed or remote deployments).

**Why it happens:**
The existing `flowy run ./my.wdl` and `flowy submit ./my.wdl` commands pass the file path as `WDLPath` in the request body — the server reads the file from the shared filesystem. Developers naturally follow this pattern for registry push. But the shared filesystem assumption breaks down for remote CLI users and contradicts the content-addressed registry model: the server should receive immutable content, not a mutable path reference.

**How to avoid:**
In `flowy registry push`, read the WDL file content locally before making the API call:
```go
content, err := os.ReadFile(wdlFilePath)
if err != nil {
    fmt.Fprintf(stderr, "error: reading WDL file %q: %v\n", wdlFilePath, err)
    return 1
}
req := api.RegistryPushRequest{
    Name:    name,
    Content: string(content),
}
```
The server-side `handleRegistryPush` already expects raw content and handles classification and digest computation. Do not modify `RegistryPushRequest` to add a `Path` field.

**Warning signs:**
- The push command sends the file path string as the `content` field value
- The server returns `422 Unprocessable Entity` with a WDL parse error on valid WDL files
- `RegistryPushRequest` gets a `Path` or `WDLPath` field added to it

**Phase to address:**
Phase that implements CLI registry commands — read file content locally in the push handler before constructing the API request. Add a test that sends real WDL content and verifies `201 Created`.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| `refetchInterval: 3000` (number literal) without terminal state check | Fast to implement | Continuous server polls after run completes; wasted compute and DB reads indefinitely | Never — always check terminal state |
| Treating `File` inputs identically to `String` inputs in the form | No upload infrastructure needed for MVP | Daemon cannot access browser-local paths; runs silently fail on every File-typed input | Never — must upload before submit |
| Parsing WDL input schema with regex in the browser | Avoids new backend endpoint | Misses optional inputs, compound types, struct definitions; form shows wrong fields | Never for production — acceptable only as a throwaway prototype to sketch the UI |
| Storing JWT in `client.token` in `flowy.yaml` | Zero new code for credential storage | Token in plaintext project config, shared with collaborators if committed | Never for shared environments; acceptable for fully isolated single-user local dev only |
| Single endpoint for both WDL text push and binary file staging | Simpler API surface | JSON string cannot carry binary data without base64 inflation; breaks `RegistryPushRequest` contract | Never — keep registry (WDL text) and file staging (binary) endpoints separate |
| Omitting `run_id` pre-generation in browser | Simpler client code | Cannot stage File inputs into `runs/{run_id}/inputs/` before submit; no deterministic staging path | Never when File inputs must be supported |
| `--token` flag on every registry command instead of `flowy login` | Avoids session management | Unusable interactively; token expires silently; users paste tokens from docs | Acceptable only for scripting/CI where token is injected via environment variable |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| `POST /projects/{slug}/registry` (browser upload) | Reading file as `ArrayBuffer` and sending binary | Read file as text (`file.text()`) since WDL is UTF-8 text; send as JSON string in `content` field |
| `POST /projects/{slug}/workflows` from UI | Passing raw browser File object path as `File`-typed input | Upload file first via staging endpoint, receive server-side path, substitute path in inputs map before submit |
| TanStack Query + status polling | `refetchInterval: 3000` number literal | `refetchInterval: (query) => TERMINAL_STATES.has(state) ? false : 3000` |
| `GET /projects/{slug}/runs/{id}/outputs` | Calling before run is `COMPLETED` | Gate on `run.state === 'COMPLETED'` as `enabled` condition — follow `RunDetailPage.tsx` pattern |
| CLI registry `push` | Passing file path string as `content` field | `os.ReadFile` locally; send string content in request body |
| CLI registry `pull` | Writing JSON response body to stdout | Extract `content` field from response; write to `name_digest.wdl` file; `--output -` for stdout |
| `flowy login` token storage | Writing to `./flowy.yaml` | Write to `~/.flowy/credentials.yaml` with mode 0600 |
| Project-scoped registry endpoints | Forgetting `slug` URL segment when building URL in new client methods | Use a consistent URL builder in `internal/client`; add tests that verify the URL path |
| `client.New(serverURL)` for auth-protected routes | Calling without auth header | Extend `client.Client` to carry a token; inject `Authorization: Bearer <token>` on all requests to JWT-protected endpoints |

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Polling all open run detail tabs simultaneously | N × 3s polls when N tabs are open; DB read pressure | Stop polling on tab blur via Page Visibility API, or gate on tab being visible | >5 concurrent open run tabs |
| Fetching full run list to resolve a single run ID | Slow load for projects with many runs (existing pattern in `RunDetailPage`) | Add `GET /projects/{slug}/runs/{id}` single-run endpoint to avoid fetching all runs | >100 runs in a project |
| No debounce on file input change in browser upload dialog | Rapid file selection triggers multiple simultaneous push requests | Disable submit button after first click; use mutation `isPending` state to prevent re-submission | User clicks upload button multiple times |
| Large WDL files as JSON string body | 1MB+ request; server-side JSON parse allocates full string in RAM | Client-side size warning above 500KB; the WDL parser is the practical ceiling (~5MB before parse becomes slow) | WDL files >1MB (rare but possible with deeply nested struct definitions) |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Storing JWT in `./flowy.yaml` or committed config | Token leak to version control or collaborators | Store credentials in `~/.flowy/credentials.yaml` with mode 0600; document `.gitignore` for credentials file |
| No validation on browser-uploaded WDL file type | User accidentally uploads a binary file; stored until parse error; confusing 422 response | Validate `file.name.endsWith('.wdl')` client-side before reading; reject non-WDL files with a clear message |
| File staging endpoint accepts arbitrary filenames from client | Path traversal via `../../../etc/passwd` in `filename` parameter | Strip directory components from uploaded filenames: `filepath.Base(clientFilename)`; the existing `handleFileDownload` has `..` check but a new staging endpoint needs the same guard |
| Displaying raw WDL content with `dangerouslySetInnerHTML` | XSS if WDL source contains `<script>` tags (unlikely but possible) | Use the existing `WDLSourceViewer` component which renders into a React-escaped `<pre>` block; never use `dangerouslySetInnerHTML` for WDL content |

---

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| No visual distinction between required and optional WDL inputs | Users omit optional inputs and see confusing submission errors | Mark optional inputs with `(optional)` label suffix; omit empty optional fields from `InputsJSON` on submit |
| Array/Map/Struct inputs rendered as raw JSON textarea with no guidance | Users enter `[item1, item2]` (invalid JSON); form accepts it and run fails at server | Show placeholder with valid JSON example; validate JSON on blur; highlight parse errors inline before submission |
| No submit button protection on workflow execution | Users double-click Run and submit two identical jobs | Disable submit button immediately on first click; show inline spinner; re-enable only on terminal state or API error |
| No feedback after browser WDL upload — user does not know the entry name or version | User re-uploads the same file thinking the first upload failed | Show a toast with entry name, digest shorthand, and a link to the new registry entry on success |
| Run list does not auto-refresh when active runs exist | Users cannot see a running job complete without manually refreshing | Apply conditional `refetchInterval` on the run list: poll while any run is non-terminal, stop when all runs are in terminal state |
| CLI `flowy registry list` outputs raw JSON by default | Humans scan JSON for entry names; poor interactive experience | Default to human-readable table; `--json` flag for machine-readable output |

---

## "Looks Done But Isn't" Checklist

- [ ] **Execution UI — submit:** Run submitted and `run_id` returned — verify the run appears in the run list and its status progresses beyond `PENDING`
- [ ] **Execution UI — File inputs:** File input widget present in the form — verify the selected file is uploaded first and the daemon log shows no "file not found" error for File-typed inputs
- [ ] **Execution UI — polling:** Status badge shows `COMPLETED` — verify the network tab shows no further polling requests after reaching a terminal state
- [ ] **Execution UI — outputs:** Run `COMPLETED` — verify the outputs section populates with values and file output links are clickable and download the correct file
- [ ] **Execution UI — optional inputs:** Form rendered — verify inputs with WDL default expressions appear with `(optional)` label and can be left blank
- [ ] **CLI registry push:** `flowy registry push name:tag ./workflow.wdl` returns success — verify `flowy registry pull name:tag` returns valid WDL starting with `version` or `workflow`
- [ ] **CLI registry pull:** Pull succeeds — verify output file is written to disk (not just JSON response logged)
- [ ] **CLI registry list:** List succeeds — verify human-readable table output (not JSON) is the default
- [ ] **Browser WDL upload:** Upload dialog shows success — verify the new entry appears in the registry list immediately without a manual page refresh
- [ ] **Auth (CLI):** `flowy login` writes credentials — verify `~/.flowy/credentials.yaml` exists with mode 0600; verify subsequent `flowy registry list` calls succeed without `--token`
- [ ] **Auth (CLI) — expiry:** Token expires — verify registry commands fail with a human-readable "authentication required — run flowy login" message, not a raw `{"error":"..."}` JSON body

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| File inputs as browser-local paths (runs fail silently) | HIGH | Add file staging endpoint (backend), add run ID pre-generation to form (frontend), refactor File widget to two-step upload-then-submit — backend and frontend must ship together |
| WDL schema parsed client-side with regex | MEDIUM | Add `/inputs` backend endpoint (clean addition, no DB changes), replace client-side regex with API call — contained to the form component |
| Polling never stops on terminal state | LOW | One-line fix in the `refetchInterval` callback; no API changes needed |
| JWT stored in `flowy.yaml` | MEDIUM | Add credentials file, add `flowy login` command, update all registry commands to read from credentials file — no API changes; all CLI users must re-authenticate |
| Registry push sends path instead of content | LOW | One-line fix in CLI: replace path argument with `os.ReadFile(path)` call; add test |
| 409 from outputs endpoint breaks run detail page | LOW | Gate `useRunOutputs` on `state === 'COMPLETED'`; one-line change to query `enabled` condition |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| File inputs as browser-local paths | Phase implementing File upload infrastructure (before or combined with execution UI) | Run a workflow with a `File` input from browser; confirm daemon log shows no "file not found" |
| WDL schema inferred client-side | Phase implementing dynamic input form — add `/inputs` endpoint before the form component | Render form for a workflow with optional and Array-typed inputs; confirm all inputs appear with correct types |
| Polling never stops on terminal state | Phase implementing run status monitoring UI | Observe network tab after run reaches `COMPLETED`; confirm no further polling |
| Outputs 409 breaks run detail | Phase implementing run status monitoring UI | Open run detail while run is `RUNNING`; confirm no error state, only loading indicator |
| CLI auth missing (`flowy login` not built first) | Phase implementing CLI registry commands | `flowy registry list --project demo` after `flowy login`; confirm no `--token` flag required |
| Browser upload reads file as binary | Phase implementing browser WDL upload | Upload a real `.wdl` file; confirm server returns `201 Created` and entry appears in registry list |
| Push sends path not content | Phase implementing CLI registry commands | `flowy registry push test:latest ./test.wdl` followed by `flowy registry pull test:latest`; verify pulled content matches original file |

---

## Sources

- `/home/uehara/flowy/.planning/PROJECT.md` — current milestone scope and key decisions
- `/home/uehara/flowy/.planning/RETROSPECTIVE.md` — cross-milestone lessons (v1.0 through v2.0)
- `/home/uehara/flowy/docs/adr/0001-remove-heuristic-file-staging-from-client.md` — File input staging decision
- `/home/uehara/flowy/internal/api/types.go` — RegistryPushRequest contract (Content field, not Path)
- `/home/uehara/flowy/internal/server/handlers.go` — handleOutputs 409 behavior, handleListRuns pattern
- `/home/uehara/flowy/internal/server/handlers_registry.go` — server-side push/pull/list handlers
- `/home/uehara/flowy/internal/client/client.go` — existing client (no auth layer)
- `/home/uehara/flowy/cmd/flowy/main.go` — existing CLI structure (no registry commands, no auth)
- `/home/uehara/flowy/web/src/features/runs/RunDetailPage.tsx` — outputs gating pattern (correct existing implementation)
- `/home/uehara/flowy/web/src/features/runs/useRuns.ts` — existing query hooks
- `/home/uehara/flowy/web/src/lib/apiClient.ts` — apiClient (always sends Bearer token from authStore)
- `/home/uehara/flowy/.planning/milestones/v2.0-MILESTONE-AUDIT.md` — tech debt items carried forward from v2.0

---
*Pitfalls research for: flowy v2.1 — workflow execution UI, CLI registry commands, browser WDL upload*
*Researched: 2026-04-07*
