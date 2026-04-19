# Phase 40: UI - Files Tab and File Picker - Context

**Gathered:** 2026-04-19
**Status:** Ready for planning
**Mode:** Autonomous follow-on from Phase 38/39

<domain>
## Phase Boundary

Expose the managed file library in the browser and wire it into workflow launch so project users can browse, import, delete, and select managed files without leaving the SPA.

</domain>

<decisions>
## Implementation Decisions

### UI Structure
- Reuse the existing Project detail page instead of adding a new route.
- Add a lightweight local tab switcher (`Members` / `Files`) rather than introducing a new tabs framework.
- Keep the established shadcn/table/dialog/button visual language from v2.0/v2.1.

### File Management
- Import uses a dedicated dialog with three modes: browser upload, S3 URI, and server path.
- Delete remains record-only and is confirmed with the existing alert dialog pattern.

### Workflow Launch
- File inputs show a managed-file picker plus a manual path text field as fallback.
- Picker filtering uses only the exact `parameter_meta.type` contract surfaced by the inputs endpoint.
- Inputs without a type constraint show the full project library.

</decisions>

<canonical_refs>
## Canonical References

- `.planning/ROADMAP.md`
- `.planning/REQUIREMENTS.md`
- `.planning/phases/38-file-library-rest-api/38-CONTEXT.md`
- `.planning/phases/39-workflow-completion-hook-and-type-annotation/39-CONTEXT.md`
- `web/src/features/projects/ProjectDetailPage.tsx`
- `web/src/features/registry/RunWorkflowDialog.tsx`
- `web/src/features/projects/useProjects.ts`
- `web/src/features/registry/useRegistry.ts`

</canonical_refs>

<specifics>
## Specific Ideas

- Extend the API client just enough to support multipart `FormData`.
- Keep file picker options path-backed so selecting a managed file still submits the same server-readable string payload as before.

</specifics>
