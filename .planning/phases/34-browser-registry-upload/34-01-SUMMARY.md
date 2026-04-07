---
phase: 34-browser-registry-upload
plan: "01"
subsystem: frontend/registry
tags: [react, tanstack-query, react-hook-form, zod, shadcn, file-upload]
one_liner: "PushWDLDialog with FileReader upload and usePushWDL mutation hook posting to /projects/{slug}/registry"

dependency_graph:
  requires:
    - web/src/features/registry/useRegistry.ts (existing hooks, apiClient patterns)
    - web/src/lib/apiClient.ts (ApiError, apiClient.post)
    - web/src/features/registry/CreateTagDialog.tsx (pattern reference)
  provides:
    - usePushWDL hook (POST /projects/{slug}/registry)
    - RegistryPushResponse type
    - PushWDLDialog component
  affects:
    - web/src/features/registry/useRegistry.ts (extended)
    - Plan 34-02 (wires PushWDLDialog into RegistryPage)

tech_stack:
  added: []
  patterns:
    - FileReader wrapped in Promise for async file reading
    - manual form.setError for file extension validation (z.any() + jsdom compat)
    - fireEvent.change in tests for file input inside Radix Dialog (pointer-events workaround)

key_files:
  created:
    - web/src/features/registry/PushWDLDialog.tsx
    - web/src/features/registry/__tests__/PushWDLDialog.test.tsx
  modified:
    - web/src/features/registry/useRegistry.ts

decisions:
  - "Manual setError for file extension validation instead of relying on zodResolver propagation through react-hook-form formState proxy in jsdom"
  - "fireEvent.change + Object.defineProperty(files) for non-.wdl validation test — userEvent.upload does not trigger onChange through Radix Slot in jsdom dialog context"
  - "URL confirmed as /projects/{slug}/registry (no trailing slash) per backend handlers_registry_test.go:112"

metrics:
  duration: "~10 minutes"
  completed_date: "2026-04-07"
  tasks_completed: 3
  tasks_total: 3
  files_created: 2
  files_modified: 1
---

# Phase 34 Plan 01: PushWDLDialog Component Summary

PushWDLDialog with FileReader upload and usePushWDL mutation hook posting to `/projects/{slug}/registry`.

## Tasks Completed

| Task | Description | Commit |
|------|-------------|--------|
| 1 | Add usePushWDL hook and RegistryPushResponse to useRegistry.ts | 679de16 |
| 2 | Create PushWDLDialog component | 604feb2 |
| 3 | Add PushWDLDialog unit tests | cd6cd18 |
| -  | Fix: manual setError for file validation (jsdom compat) | c9a4d2c |

## What Was Built

**`usePushWDL(slug)`** — TanStack Query mutation hook that POSTs `{ name, content }` to `/projects/${slug}/registry`, typed with `RegistryPushResponse`, and invalidates the `['projects', slug, 'registry']` query key on success.

**`PushWDLDialog`** — Dialog component following the `CreateTagDialog` pattern:
- Entry name input with zod validation (`^[a-z0-9-]+$`, max 64)
- File picker (`accept=".wdl"`) with `fileInputRef` for DOM reset on close
- FileReader wrapped in a Promise for async content reading
- Manual `form.setError('file', ...)` for `.wdl` extension validation (reliable in jsdom)
- Error handling: 422 → "Invalid WDL: {message}", 409 → name conflict copy, else generic
- Success: toast with `{name} pushed. Digest: {short}`, form reset, dialog close
- `mode: 'onChange'` + `disabled={isPending || !form.formState.isValid}` on submit button

**Tests (8/8 passing):** rendering, submit disabled when empty, validation error for non-.wdl, mutateAsync called with correct payload, success toast, 422 error toast, 409 error toast, form reset on close.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] zodResolver refine errors not propagating through FormMessage in jsdom**
- **Found during:** Task 3
- **Issue:** `z.any().refine(...)` errors on the file field did not propagate to `FormMessage` when triggered via react-hook-form's `mode: 'onChange'` in the jsdom test environment. `aria-invalid` remained "false" even after uploading a non-.wdl file.
- **Root cause:** react-hook-form v7's formState proxy does not reliably surface zodResolver errors for `z.any()` fields to subscribers in jsdom. `form.setValue({ shouldValidate: true })` and `form.trigger('file')` both failed to update `fieldState.error`.
- **Fix:** Replaced zodResolver-based file validation with explicit `form.setError('file', { type: 'manual', message: '...' })` / `form.clearErrors('file')` in the `onChange` handler. Browser behavior is identical; zod schema still validates `name` field. Kept `z.any().refine(...)` on the schema for submit-time safety.
- **Files modified:** `web/src/features/registry/PushWDLDialog.tsx`
- **Commit:** c9a4d2c

**2. [Rule 1 - Bug] userEvent.upload does not trigger onChange in Radix Dialog jsdom context**
- **Found during:** Task 3
- **Issue:** `userEvent.upload` did not fire the React `onChange` handler on a file `<Input>` inside a shadcn `FormControl` (Radix `Slot`) within a Radix `Dialog`. Confirmed: the Radix dialog sets `pointer-events: none` on `<body>`, and `userEvent.upload` requires pointer interaction.
- **Fix:** The non-.wdl validation test uses `Object.defineProperty(input, 'files', ...)` + `fireEvent.change(input)` to directly trigger the change event. The valid-file tests use `userEvent.upload` which works because the test checks `mutateAsync` calls (triggered by button click) rather than DOM change events.
- **Files modified:** `web/src/features/registry/__tests__/PushWDLDialog.test.tsx`
- **Commit:** cd6cd18

## Self-Check

- [x] `web/src/features/registry/PushWDLDialog.tsx` exists
- [x] `web/src/features/registry/useRegistry.ts` exports `usePushWDL` and `RegistryPushResponse`
- [x] `web/src/features/registry/__tests__/PushWDLDialog.test.tsx` exists with 8 tests
- [x] `cd web && npx tsc --noEmit` — PASS
- [x] `npx vitest run src/features/registry` — 24 tests, all pass
- [x] Commits: 679de16, 604feb2, cd6cd18, c9a4d2c — all present

## Self-Check: PASSED
