---
phase: 36-ui-workflow-execution
plan: "01"
subsystem: frontend
tags: [react, typescript, shadcn, tanstack-query, zod, react-hook-form]
dependency_graph:
  requires:
    - "Phase 32: GET /projects/{slug}/registry/{name}/inputs endpoint"
    - "Phase 35: Run detail page (navigation target)"
  provides:
    - RunWorkflowDialog component
    - useRegistryInputs query hook
    - useRunWorkflow mutation hook
    - shadcn Checkbox component
  affects:
    - web/src/features/registry/useRegistry.ts
    - web/src/components/ui/ (new Checkbox)
    - web/src/test/setup.ts (ResizeObserver polyfill)
tech_stack:
  added:
    - "@radix-ui/react-checkbox (already in package.json, now actively used)"
  patterns:
    - "Dynamic zod schema built from server-returned input list"
    - "z.preprocess for optional numeric fields to avoid empty-string→0 coercion"
    - "ResizeObserver polyfill in test setup for Radix UI components"
key_files:
  created:
    - web/src/components/ui/checkbox.tsx
    - web/src/features/registry/RunWorkflowDialog.tsx
    - web/src/features/registry/__tests__/RunWorkflowDialog.test.tsx
  modified:
    - web/src/features/registry/useRegistry.ts
    - web/src/test/setup.ts
decisions:
  - "Use z.preprocess for optional Int/Float fields: empty string becomes undefined before coercion, avoiding z.coerce.number('') === 0 false positive"
  - "ResizeObserver polyfill added globally in setup.ts: Radix UI Select/Tooltip components call it on mount in jsdom"
  - "shadcn CLI misresolves @/ alias in worktrees — created checkbox.tsx manually from the generated output"
metrics:
  completed_date: "2026-04-08"
  tasks: 2
  files: 5
---

# Phase 36 Plan 01: RunWorkflowDialog Component Summary

**One-liner:** Dynamic workflow execution dialog with per-type form controls (Input/Checkbox/Textarea) driven by server-returned WDL input schema, with full error handling and navigation to run detail.

## What Was Built

### Task 1: shadcn Checkbox + useRegistry hooks

Added `web/src/components/ui/checkbox.tsx` (shadcn Checkbox wrapping `@radix-ui/react-checkbox`).

Extended `web/src/features/registry/useRegistry.ts` with:
- `RegistryInputItem` and `RegistryInputsResponse` interfaces
- `useRegistryInputs(slug, name, tag)` — TanStack Query hook for `GET /projects/{slug}/registry/{name}/inputs?tag={tag}`
- `useRunWorkflow(slug)` — TanStack Mutation for `POST /projects/{slug}/workflows`, invalidates `['projects', slug, 'runs']` on success

### Task 2: RunWorkflowDialog component + tests

`web/src/features/registry/RunWorkflowDialog.tsx` — complete dialog featuring:

- **Version/tag selector**: `<Select>` defaulting to "latest" tag; shows truncated digest below
- **Dynamic zod schema**: built via `useMemo` from `inputsData.inputs` — one schema field per WDL input
- **Dynamic form fields**: per-type controls as specified in UI spec
  - String → `<Input type="text">`
  - Int → `<Input type="number" step="1">`
  - Float → `<Input type="number" step="any">`
  - Boolean → `<Checkbox>` in flex row with inline label
  - File → `<Input type="text">` with `Info` icon tooltip explaining server-accessible paths
  - Array/Map/Struct → `<Textarea>` with JSON format hint
- **Loading state**: 3 skeleton rows while inputs fetch
- **Error state**: error message + disabled submit
- **Empty inputs state**: informational message
- **Optional field handling**: label suffixed "(optional)"; empty optionals omitted from submission JSON
- **Default values**: pre-populated from `input.default`; shown as "(default: ...)" annotation
- **Submission**: builds inputs dict, calls `runWorkflow.mutateAsync`, navigates to `/projects/{slug}/runs/{run_id}` on success
- **Error mapping**: 400 → "Invalid inputs...", 404 → "Workflow version not found.", else → "Server error..."

`web/src/features/registry/__tests__/RunWorkflowDialog.test.tsx` — 15 tests, all passing.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] shadcn CLI misresolves @/ alias in git worktrees**
- **Found during:** Task 1
- **Issue:** `npx shadcn@latest add checkbox` created `web/@/components/ui/checkbox.tsx` instead of `web/src/components/ui/checkbox.tsx`. The CLI reads `components.json` but doesn't honor the `tsconfig.app.json` `baseUrl`/`paths` in the worktree context.
- **Fix:** Manually created `web/src/components/ui/checkbox.tsx` from the generated file content.
- **Files modified:** `web/src/components/ui/checkbox.tsx`
- **Commit:** 7b14fec

**2. [Rule 2 - Missing infrastructure] ResizeObserver not defined in jsdom**
- **Found during:** Task 2 (test execution)
- **Issue:** Radix UI's `@radix-ui/react-use-size` calls `new ResizeObserver()` in a `useEffect` when `Select` and `Tooltip` components mount. jsdom doesn't implement ResizeObserver, causing all tests using these components to throw.
- **Fix:** Added `global.ResizeObserver = vi.fn().mockImplementation(...)` polyfill to `web/src/test/setup.ts`.
- **Files modified:** `web/src/test/setup.ts`
- **Commit:** 7e0675d

**3. [Rule 1 - Bug] z.coerce.number("") === 0 causes false positive for optional numeric fields**
- **Found during:** Task 2 (test "optional fields left empty are omitted from submission")
- **Issue:** When an optional Int/Float field is cleared, `z.union([z.coerce.number(), z.literal('')])` tries the `number` branch first: `Number("") === 0`, so the value is coerced to `0` and included in submission rather than omitted.
- **Fix:** Used `z.preprocess((v) => v === '' ? undefined : v, z.coerce.number().optional())` so empty strings are treated as `undefined` before coercion.
- **Files modified:** `web/src/features/registry/RunWorkflowDialog.tsx`
- **Commit:** 7e0675d

**4. [Rule 1 - Bug] Test selector ambiguity — "Run Workflow" and "latest" match multiple elements**
- **Found during:** Task 2 (test execution)
- **Issue:** `screen.getByText('Run Workflow')` matched both the `<h2>` dialog title and the `<button>` submit button. `screen.getByText('latest')` matched both the Select trigger value and the SelectItem option.
- **Fix:** Used `screen.getByRole('heading', { name: 'Run Workflow' })` and `screen.getAllByText('latest')` with `length > 0` assertion.
- **Files modified:** `web/src/features/registry/__tests__/RunWorkflowDialog.test.tsx`
- **Commit:** 7e0675d

## Test Results

```
Test Files  22 passed (22)
     Tests  154 passed (154)
```

All 15 new RunWorkflowDialog tests pass. All 139 pre-existing tests continue to pass.

## Known Stubs

None. All form controls are wired to live hooks. Navigation targets a real run_id from the API response.

## Threat Flags

No new threat surface beyond what was modeled in the plan. The `RunWorkflowDialog` component is not yet wired into `RegistryEntryPage` — that integration (including the role gate for executor/admin) is the subject of Plan 36-02.

## Self-Check: PASSED

| Item | Status |
|------|--------|
| `web/src/components/ui/checkbox.tsx` | FOUND |
| `web/src/features/registry/RunWorkflowDialog.tsx` | FOUND |
| `web/src/features/registry/__tests__/RunWorkflowDialog.test.tsx` | FOUND |
| Commit `7b14fec` (Task 1) | FOUND |
| Commit `7e0675d` (Task 2) | FOUND |
| TypeScript: `npx tsc --noEmit` | PASS (0 errors) |
| Tests: 154 passed / 0 failed | PASS |
