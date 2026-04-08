# Phase 36: UI Workflow Execution - Context

**Gathered:** 2026-04-08
**Status:** Ready for planning

<domain>
## Phase Boundary

Browser users with executor role can select a workflow from the registry, fill in input parameters, submit execution, and view results — all from the browser. This phase adds a "Run" button and execution dialog to the RegistryEntryPage, with a dynamically generated input form and post-submission navigation to RunDetailPage.

</domain>

<decisions>
## Implementation Decisions

### Run Dialog UI Design
- "Run" button placement: next to entry name in RegistryEntryPage header, visible only for executor+ role — uses same role-check pattern as admin features (members list + useAuthStore)
- Dialog follows existing CreateTagDialog pattern: shadcn Dialog + react-hook-form + zod schema — consistent with PushWDLDialog and CreateTagDialog
- Version/tag selection via dropdown showing available tags (default "latest"), with selected version's digest displayed below — user picks which version to execute

### Input Form Field Types
- Per-type controls using shadcn/ui components: text Input for String, number Input for Int/Float, Checkbox for Boolean, path-entry text Input for File, JSON Textarea for Array/Map/Struct
- Client-side validation via zod: required inputs (no default value in schema) are mandatory fields. Optional inputs left empty are omitted from submission JSON
- Pre-fill form fields with default values from the inputs endpoint response when available

### Post-Submission Flow
- After successful submit: toast notification "Run submitted" via sonner, then navigate to RunDetailPage for the new run — Phase 35 polling shows live status automatically
- Server errors mapped to user-friendly messages in dialog: 400 → "Invalid inputs", 404 → "Workflow not found", 500 → "Server error" — same pattern as PushWDLDialog
- File-typed inputs: text Input with placeholder "e.g., s3://bucket/path or /absolute/path" and tooltip icon explaining server-accessible paths required, not browser-local files (per SC-4)

### Claude's Discretion
- Hook structure: single `useRunWorkflow` mutation hook or inline mutation — whichever is cleaner
- How to fetch and cache the input schema from GET /projects/{slug}/registry/{name}/inputs endpoint
- Whether to extract RunDetailPage output viewer into a shared component or import directly

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `web/src/features/registry/RegistryEntryPage.tsx` — registry entry page with versions, tags, role checks
- `web/src/features/registry/useRegistry.ts` — hooks for registry entries, versions, content, push, tags
- `web/src/features/registry/CreateTagDialog.tsx` — dialog pattern (react-hook-form + zod + shadcn Dialog)
- `web/src/features/registry/PushWDLDialog.tsx` — file upload dialog pattern
- `web/src/features/runs/RunDetailPage.tsx` — run detail view with outputs table (Phase 30 output viewer)
- `web/src/features/runs/useRuns.ts` — run list hooks
- `web/src/features/runs/usePollingRuns.ts` — polling hook (Phase 35)
- `web/src/lib/apiClient.ts` — HTTP client with Bearer auth

### Established Patterns
- Dialog: open/onOpenChange props, form validation, Loader2 loading state, reset on close
- API calls: `apiClient.post(url, body)` with JSON serialization
- Mutations: `useMutation` from @tanstack/react-query with `onSuccess` invalidation
- Role check: `members.find(m => m.username === currentUsername)?.role` comparison
- Navigation: `useNavigate` from react-router

### Integration Points
- New RunWorkflowDialog component rendered in RegistryEntryPage
- Inputs endpoint: GET /projects/{slug}/registry/{name}/inputs?tag={tag} — returns workflow input declarations
- Submit endpoint: POST /projects/{slug}/workflows — creates new run
- After submit: navigate to /projects/{slug}/runs/{runId} — RunDetailPage with Phase 35 polling

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches using existing patterns.

</specifics>

<deferred>
## Deferred Ideas

- Dynamic form for compound types (Array, Map, Struct) with dedicated UI widgets — v2.2+ per REQUIREMENTS.md. Phase 36 uses JSON textarea for these types.

</deferred>
