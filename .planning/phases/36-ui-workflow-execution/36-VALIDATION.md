---
phase: 36
slug: ui-workflow-execution
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-08
---

# Phase 36 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | vitest |
| **Config file** | web/vitest.config.ts |
| **Quick run command** | `cd web && npx vitest run --reporter=verbose src/features/registry/` |
| **Full suite command** | `cd web && npx vitest run --reporter=verbose` |
| **Estimated runtime** | ~20 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd web && npx vitest run --reporter=verbose src/features/registry/`
- **After wave completion:** Run full suite + `cd web && npx tsc --noEmit`

---

## Validation Architecture

### Wave 0: Prerequisites
- Verify Checkbox component installed via shadcn

### Wave 1: Core Hooks & Dialog
- `useRegistryInputs` hook returns input schema for given tag
- `useRunWorkflow` mutation calls submit endpoint
- `RunWorkflowDialog` renders dynamic form with correct field types per WDL type
- Zod schema validates required fields, allows optional fields to be empty
- File input shows placeholder and tooltip per SC-4

### Wave 2: Page Integration & Tests
- "Run" button visible for executor+ role, absent for viewers
- Clicking "Run" opens dialog with version selector
- Form submission creates run and navigates to RunDetailPage
- Page tests verify role gating, dialog interaction, form rendering
