---
phase: 36-ui-workflow-execution
fixed_at: 2026-04-07T00:00:00Z
review_path: .planning/phases/36-ui-workflow-execution/36-REVIEW.md
iteration: 1
findings_in_scope: 4
fixed: 4
skipped: 0
status: all_fixed
---

# Phase 36: Code Review Fix Report

**Fixed at:** 2026-04-07T00:00:00Z
**Source review:** .planning/phases/36-ui-workflow-execution/36-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 4
- Fixed: 4
- Skipped: 0

## Fixed Issues

### WR-02: Unchecked property access on `inputsData.inputs` could throw at runtime

**Files modified:** `web/src/features/registry/RunWorkflowDialog.tsx`
**Commit:** 051d358
**Applied fix:** Changed `inputsData?.inputs.length` to `inputsData?.inputs?.length` on line 291, adding optional chaining on `inputs` to prevent a potential TypeError if the API returns `{ inputs: undefined }`.

### WR-03: Form `handleSubmit` type cast silences a type error

**Files modified:** `web/src/features/registry/RunWorkflowDialog.tsx`
**Commit:** 051d358
**Applied fix:** Added explicit `Record<string, unknown>` generic to `useForm<Record<string, unknown>>(...)` so that `onSubmit(data: Record<string, unknown>)` matches the form type without a cast. Removed the `onSubmit as Parameters<typeof form.handleSubmit>[0]` cast from the JSX `onSubmit` handler.

### WR-04: Tooltip test passes vacuously when Info icon is not found

**Files modified:** `web/src/features/registry/RunWorkflowDialog.tsx`, `web/src/features/registry/__tests__/RunWorkflowDialog.test.tsx`
**Commit:** 2ab3ef8
**Applied fix:** Added `data-testid="file-input-tooltip-trigger"` to the `<Info>` element in `RunWorkflowDialog.tsx`. Updated the test to use `screen.getByTestId('file-input-tooltip-trigger')` (which throws if absent) instead of the fragile `document.querySelector('svg.lucide-info')` with a conditional hover guard.

### WR-01: Tagless registry versions are silently unselectable in the Run dialog

**Files modified:** `web/src/features/registry/RunWorkflowDialog.tsx`
**Commit:** 5f616b2
**Applied fix:**
- `getDefaultTag`: falls back to `versions[0].digest` when the first version has no tags (previously returned `''`, disabling inputs fetch).
- `getDigestForTag` (renamed param to `tagOrDigest`): added a secondary match on `v.digest === tagOrDigest` so digest-valued selections resolve correctly.
- `SelectContent`: replaced the flat `v.tags.map(...)` with a branch — tagged versions render as before; tagless versions render a single `<SelectItem>` keyed and valued by the full digest, displaying the truncated digest via the existing `truncateDigest` helper.

---

_Fixed: 2026-04-07T00:00:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
