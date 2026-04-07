# Phase 34: Browser Registry Upload - Context

**Gathered:** 2026-04-07
**Status:** Ready for planning

<domain>
## Phase Boundary

Browser users with executor role can push a WDL file to the registry without leaving the UI. This phase adds a "Push WDL" button and dialog to the RegistryPage.

</domain>

<decisions>
## Implementation Decisions

### Upload UI Design
- File picker using `<input type="file" accept=".wdl">` — read file content via FileReader API, send as string to existing push API (`POST /projects/{slug}/registry/`)
- Dialog layout: entry name text input + file picker + submit button — follows existing CreateTagDialog pattern (shadcn/ui Dialog + react-hook-form + zod)
- Success feedback: toast notification showing entry name + digest + kind badge, then auto-refresh entry list via `useRegistryEntries` invalidation
- "Push WDL" button placement: top of RegistryPage next to existing controls, visible only for executor+ role

### Validation & Error Handling
- Client-side `.wdl` extension check before upload — inline validation message in dialog (per success criteria SC-4)
- Server errors mapped to user-friendly messages: 422 → "Invalid WDL file: {message}", 409 → "Name already used for a different kind"
- No client-side file size limit — WDL files are text, typically small

### Role Gating
- Use project members list + current username from `useAuthStore` to check executor+ role — same pattern as admin check in RegistryPage
- Global admin can always push (consistent with backend RBAC)

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `web/src/features/registry/RegistryPage.tsx` — existing registry entry list with role checks
- `web/src/features/registry/CreateTagDialog.tsx` — dialog pattern with react-hook-form + zod
- `web/src/lib/apiClient.ts` — HTTP client with Bearer auth (needs file content sent as JSON string, not multipart)
- `web/src/stores/authStore.ts` — `useAuthStore` for username/token
- shadcn/ui: Dialog, Button, Input, Form components

### Established Patterns
- Dialog: open/onOpenChange props, form validation, Loader2 loading state, reset on close
- API calls: `apiClient.post(url, body)` with JSON serialization
- Role check: `members.find(m => m.username === currentUsername)?.role` comparison
- Hooks: `useRegistryEntries(slug)` for entry list data

### Integration Points
- New PushWDLDialog component rendered in RegistryPage
- Push API: `POST /projects/{slug}/registry/` with `{name: string, content: string}` body
- Response: `{name, digest, kind, version_id}` — display in toast
- After push: invalidate `useRegistryEntries` query to refresh list

</code_context>

<specifics>
## Specific Ideas

No specific requirements — follow existing dialog and role-checking patterns in the codebase.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>
