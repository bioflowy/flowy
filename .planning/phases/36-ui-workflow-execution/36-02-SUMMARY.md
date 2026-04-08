---
phase: 36-ui-workflow-execution
plan: "02"
subsystem: frontend
tags: [react, typescript, shadcn, role-gating, registry]
dependency_graph:
  requires:
    - "Phase 36 Plan 01: RunWorkflowDialog component"
    - "Phase 30: RunDetailPage (navigation target)"
  provides:
    - "Run button on RegistryEntryPage gated by executor/admin role"
    - "RunWorkflowDialog wired into RegistryEntryPage"
  affects:
    - web/src/features/registry/RegistryEntryPage.tsx
    - web/src/features/registry/__tests__/RegistryEntryPage.test.tsx
tech_stack:
  added: []
  patterns:
    - "Same canRun pattern as canPush in RegistryPage.tsx (isGlobalAdmin || admin || executor)"
    - "useAuthStore called twice for username and role selectors"
key_files:
  created: []
  modified:
    - web/src/features/registry/RegistryEntryPage.tsx
    - web/src/features/registry/__tests__/RegistryEntryPage.test.tsx
decisions:
  - "Run button placed in entry header flex row alongside h1, not in the Versions section toolbar"
  - "isGlobalAdmin and canRun follow exact same pattern as RegistryPage.tsx canPush"
metrics:
  completed_date: "2026-04-07"
  duration_minutes: 20
  tasks: 2
  files: 2
---

# Phase 36 Plan 02: RegistryEntryPage Run Button Integration Summary

**One-liner:** Role-gated Run button in RegistryEntryPage header wires into RunWorkflowDialog with executor/admin/global-admin visibility and 4 new tests covering all role cases, human-verified end-to-end.

## What Was Built

### Task 1: Wire Run button and RunWorkflowDialog into RegistryEntryPage with tests

**`web/src/features/registry/RegistryEntryPage.tsx`** modified:

- Added `import { RunWorkflowDialog } from '@/features/registry/RunWorkflowDialog'`
- Added `globalRole` selector from `useAuthStore` (alongside existing `currentUsername`)
- Added `isGlobalAdmin` and `canRun` computed values mirroring the `canPush` pattern in `RegistryPage.tsx`
- Added `runOpen` / `setRunOpen` state
- Modified entry header from a bare `<h1>` to a flex row with the title and a conditional Run button
- Added `<RunWorkflowDialog>` render at the bottom of the component alongside `CreateTagDialog` and `DeleteTagDialog`

**`web/src/features/registry/__tests__/RegistryEntryPage.test.tsx`** modified:

- Added `RunWorkflowDialog` mock (renders `data-testid="run-workflow-dialog"` when open)
- Added `executorMembers` fixture
- Updated `setupMocks` to accept `globalRole?: string` parameter and pass it to the `useAuthStore` mock alongside `username`
- Added 4 new tests:
  1. "executor sees Run button; viewer does not"
  2. "admin sees Run button"
  3. "global admin sees Run button regardless of project role"
  4. "opens RunWorkflowDialog when Run button is clicked"

### Task 2: Visual verification of full workflow execution flow

Human-verified and approved. The full end-to-end flow was confirmed working:

- Run button visible for executor/admin role users on RegistryEntryPage
- Run button hidden for viewer role users
- Clicking "Run" opens RunWorkflowDialog with version selector and dynamic input form
- Workflow submission navigates to RunDetailPage
- Output files displayed after run completion

## Test Results

```
Test Files  4 passed (4)
     Tests  49 passed (49)
```

All 14 RegistryEntryPage tests pass (10 existing + 4 new). All 49 registry tests pass overall.

## Deviations from Plan

None - plan executed exactly as written.

## Known Stubs

None. The Run button is fully wired to `RunWorkflowDialog` which was implemented in Plan 36-01 with live hooks.

## Threat Flags

No new threat surface. The `canRun` client-side check is defense-in-depth; server enforces executor role on `POST /projects/{slug}/workflows` as modeled in T-36-04.

## Self-Check: PASSED

| Item | Status |
|------|--------|
| `web/src/features/registry/RegistryEntryPage.tsx` modified | FOUND |
| `web/src/features/registry/__tests__/RegistryEntryPage.test.tsx` modified | FOUND |
| Commit `460a968` (Task 1) | FOUND |
| Task 2 human-verify | APPROVED |
| TypeScript: `npx tsc --noEmit` | PASS (0 errors) |
| Tests: 49 passed / 0 failed | PASS |
