---
phase: 34-browser-registry-upload
plan: "02"
subsystem: frontend-registry
tags: [react, role-gating, push-wdl, vitest]
dependency_graph:
  requires:
    - 34-01 (PushWDLDialog component and usePushWDL hook)
  provides:
    - Role-gated Push WDL button on RegistryPage
    - PushWDLDialog mounted on RegistryPage
  affects:
    - web/src/features/registry/RegistryPage.tsx
    - web/src/features/registry/__tests__/RegistryPage.test.tsx
tech_stack:
  added: []
  patterns:
    - Role-gating via canPush (global admin OR project admin OR project executor)
    - Global admin bypass pattern (isGlobalAdmin shortcircuits project membership check)
key_files:
  created: []
  modified:
    - web/src/features/registry/RegistryPage.tsx
    - web/src/features/registry/__tests__/RegistryPage.test.tsx
decisions:
  - Global admin check added to isProjectAdmin (was missing from original code — now consistent with canPush)
  - Mocked PushWDLDialog in tests to isolate RegistryPage behavior
metrics:
  duration: "~10 minutes"
  completed: "2026-04-07"
  tasks_completed: 3
  files_modified: 2
requirements: [REG-UI-01]
---

# Phase 34 Plan 02: RegistryPage Push WDL Integration Summary

**One-liner:** Role-gated Push WDL button on RegistryPage with executor+/global-admin visibility and 6 new vitest role-gate tests.

## What Was Built

Wired the `PushWDLDialog` component (from Plan 01) into `RegistryPage` with role-based visibility. Added `canPush` logic that grants button access to project admins, project executors, and global admins (even without project membership). Also fixed the existing `isProjectAdmin` check which was missing the global admin bypass.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Wire PushWDLDialog into RegistryPage with role gating | a56c88b | web/src/features/registry/RegistryPage.tsx |
| 2 | Add role-gate and button visibility tests to RegistryPage.test.tsx | 9bfa8fc | web/src/features/registry/__tests__/RegistryPage.test.tsx |
| 3 | Verify Push WDL flow (automated checks) | — (no code changes) | — |

## Key Changes

### RegistryPage.tsx
- Added `Upload` icon from lucide-react and `PushWDLDialog` import
- Added `globalRole` selector from `useAuthStore`
- Replaced single `isProjectAdmin` with three-way role check: `isGlobalAdmin`, `isProjectAdmin` (now includes global admin), `canPush` (admin OR executor OR global admin)
- Added `pushOpen` state to control dialog visibility
- Added Push WDL button in page header (conditionally rendered via `canPush`)
- Mounted `<PushWDLDialog>` with `pushOpen`/`setPushOpen`/`slug`
- Updated empty state text from "flowy CLI" to "CLI or the Push WDL button above"

### RegistryPage.test.tsx
- Added `usePushWDL` to the useRegistry mock block
- Added `PushWDLDialog` mock returning a `data-testid="push-wdl-dialog"` sentinel
- Updated `setupMocks` to accept `globalRole` parameter (default `'user'`)
- Added `executorMembers` helper fixture
- Added 6 new test cases:
  1. Push WDL button visible for project admin
  2. Push WDL button visible for project executor
  3. Push WDL button hidden for viewer
  4. Push WDL button visible for global admin without project membership
  5. Clicking Push WDL opens dialog
  6. Empty state mentions Push WDL button above

## Verification Results

- `npx vitest run src/features/registry`: **30 tests, 3 files, all passing**
  - RegistryPage.test.tsx: 12 tests (6 original + 6 new)
  - PushWDLDialog.test.tsx: 6 tests (from Plan 01, unchanged)
  - RegistryEntryPage.test.tsx: 12 tests (unchanged)
- `npx tsc --noEmit`: **exits 0, no errors**

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed isProjectAdmin missing global admin bypass**
- **Found during:** Task 1
- **Issue:** Original `isProjectAdmin` only checked project membership role, not global admin. Global admins could not see the delete dropdown.
- **Fix:** Changed to `isGlobalAdmin || currentMember?.role === 'admin'` — consistent with `canPush` logic and backend behavior.
- **Files modified:** web/src/features/registry/RegistryPage.tsx
- **Commit:** a56c88b

## Self-Check

- [x] `web/src/features/registry/RegistryPage.tsx` — modified, contains `canPush`, `isGlobalAdmin`, `PushWDLDialog`, `pushOpen`, `Upload`, "Push WDL button above"
- [x] `web/src/features/registry/__tests__/RegistryPage.test.tsx` — modified, contains all 6 new tests
- [x] Commit a56c88b exists
- [x] Commit 9bfa8fc exists
- [x] All 30 registry tests pass
- [x] TypeScript compiles clean

## Self-Check: PASSED
