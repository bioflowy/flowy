---
phase: 29-registry-browsing
plan: 02
subsystem: ui
tags: [react, typescript, shadcn, tanstack-query, react-hook-form, zod, vitest]

# Dependency graph
requires:
  - phase: 29-01
    provides: useRegistry hooks (useRegistryVersions, useRegistryContent, useUpsertTag, useDeleteTag) and RegistryPage

provides:
  - WDLSourceViewer component: read-only code block with line numbers and clipboard copy button
  - CreateTagDialog: admin dialog for creating or moving version tags via react-hook-form + zod
  - DeleteTagDialog: admin AlertDialog for tag deletion with confirmation
  - RegistryEntryPage: full entry detail page with version history table, WDL source viewer, admin tag management
  - 10 unit tests covering all RegistryEntryPage interaction paths

affects:
  - phase-30-file-browsing (similar registry-style detail page patterns)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - WDLSourceViewer: plain HTML table for line numbers + sticky copy button bar (no react-syntax-highlighter dep)
    - useEffect to set selected state after async data loads (not useState initializer)
    - Dialog mocked as testid sentinel in unit tests to keep focus on host component behavior

key-files:
  created:
    - web/src/features/registry/WDLSourceViewer.tsx
    - web/src/features/registry/CreateTagDialog.tsx
    - web/src/features/registry/DeleteTagDialog.tsx
    - web/src/features/registry/__tests__/RegistryEntryPage.test.tsx
  modified:
    - web/src/features/registry/RegistryEntryPage.tsx

key-decisions:
  - "RegistryEntryPage kind badge container changed from <p> to <div> because Badge renders as div and <p> cannot contain block-level elements"
  - "truncateDigest uses slice(7, 23) on full sha256 string giving 16 hex chars: sha256:abcdef1234567890..."

patterns-established:
  - "WDLSourceViewer pattern: table-based line numbers with sticky top bar for action buttons"
  - "Admin gating in RegistryEntryPage follows Phase 28-02 pattern: currentMember?.role === 'admin' from members list"
  - "Child dialogs mocked as data-testid sentinels in tests (open ? <div testid /> : null) — established in RegistryPage tests"

requirements-completed: [REG-02, REG-03, REG-04]

# Metrics
duration: 4min
completed: 2026-04-07
---

# Phase 29 Plan 02: Registry Entry Detail Page Summary

**RegistryEntryPage with version history table, WDL source viewer (line-numbers + copy), and admin tag create/delete dialogs completing the registry browsing feature**

## Performance

- **Duration:** ~4 min
- **Started:** 2026-04-07T02:15:54Z
- **Completed:** 2026-04-07T02:19:39Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments

- WDLSourceViewer renders WDL content with per-line numbers and a sticky clipboard copy button (Check/Copy icon toggle with 2s reset)
- CreateTagDialog provides form-validated (zod + react-hook-form) upsert flow for admin tag assignment with version selector
- DeleteTagDialog provides destructive confirmation (Keep Tag / Delete) for admin tag removal
- RegistryEntryPage wires all components: auto-selects first version, loads WDL source on version click, guards tag controls to admin only, handles loading/error states
- All 99 tests pass (17 test files), TypeScript compiles clean

## Task Commits

1. **Task 1: Create WDLSourceViewer, CreateTagDialog, DeleteTagDialog** - `81ff0cb` (feat)
2. **Task 2: Build RegistryEntryPage and RegistryEntryPage tests** - `7d1aebf` (feat)

## Files Created/Modified

- `web/src/features/registry/WDLSourceViewer.tsx` - Read-only code block with line number table and sticky copy button
- `web/src/features/registry/CreateTagDialog.tsx` - Admin dialog: create or move tag via form with zod validation
- `web/src/features/registry/DeleteTagDialog.tsx` - Admin AlertDialog: confirm tag deletion with destructive button
- `web/src/features/registry/RegistryEntryPage.tsx` - Full detail page replacing placeholder: version table, source viewer, tag management
- `web/src/features/registry/__tests__/RegistryEntryPage.test.tsx` - 10 unit tests covering render, admin/viewer, loading, error, dialog open states

## Decisions Made

- Changed kind badge container from `<p>` to `<div>` because shadcn Badge renders as a `<div>`, and HTML forbids block-level elements inside `<p>`. This was caught during test run (Rule 1 auto-fix).
- `truncateDigest` uses `slice(7, 23)` on the full sha256 string (including `sha256:` prefix) giving 16 hex chars of display: `sha256:abcdef1234567890...`

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Changed kind badge wrapper from `<p>` to `<div>`**
- **Found during:** Task 2 (test run for RegistryEntryPage)
- **Issue:** `<p>` cannot contain a `<div>` (Badge renders as div); React hydration warning and test DOM structure issue
- **Fix:** Changed wrapper element to `<div className="text-xs text-muted-foreground mt-1 flex items-center gap-2">`
- **Files modified:** web/src/features/registry/RegistryEntryPage.tsx
- **Verification:** No HTML nesting warnings; all tests pass
- **Committed in:** 7d1aebf (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 - HTML nesting bug)
**Impact on plan:** Minor DOM structure correction required for correctness. No scope creep.

## Issues Encountered

- Test assertion `getByText('sha256:abcdef12345678...')` failed because the digest appears in both the version table row and the Source section subtitle — fixed by using `getAllByText` with `length >= 1`.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Registry browsing feature (REG-01 through REG-04) is complete: list page, entry detail page, WDL source viewer, tag management
- Phase 30 (File Browsing) can proceed; it will need 3 new backend endpoints (GET /runs, GET /runs/{id}/files, GET /runs/{id}/files/download) as noted in STATE.md blockers

---
*Phase: 29-registry-browsing*
*Completed: 2026-04-07*
