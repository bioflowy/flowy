---
phase: 36-ui-workflow-execution
verified: 2026-04-07T02:15:00Z
status: human_needed
score: 4/5 must-haves verified
overrides_applied: 0
human_verification:
  - test: "Visual end-to-end flow: Run button -> dialog -> submit -> RunDetailPage outputs"
    expected: "Run button visible for executor/admin, hidden for viewer; dialog opens with dynamic form; submission navigates to RunDetailPage; output files section shows after run completes"
    why_human: "Phase 30 RunDetailPage output section and full run lifecycle (wait for terminal state, display outputs) cannot be verified without a running server and a real workflow execution"
---

# Phase 36: UI Workflow Execution Verification Report

**Phase Goal:** Browser users with executor role can select a workflow from the registry, fill in input parameters, submit execution, and view results — all from the browser
**Verified:** 2026-04-07T02:15:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (from ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| SC1 | "Run" button appears on RegistryEntryPage for executor role, absent for viewers | VERIFIED | `RegistryEntryPage.tsx:61 canRun = isGlobalAdmin || admin || executor`; tests: "executor sees Run button; viewer does not" — all 4 role-gating tests pass |
| SC2 | Clicking "Run" opens dialog with dynamically generated form — one field per WDL input, per-type controls | VERIFIED | `RunWorkflowDialog.tsx` renders String→text, Int→number step=1, Float→number step=any, Boolean→Checkbox, File→text+tooltip, Array/Map/Struct→Textarea; 15 tests all pass including "renders correct input control per WDL type" |
| SC3 | Submitting form creates new run and navigates to RunDetailPage | VERIFIED | `RunWorkflowDialog.tsx:217-224` — `mutateAsync` returns `{ run_id }` from API; `navigate('/projects/${slug}/runs/${result.run_id}')` called on success; toast "Run submitted." tested |
| SC4 | File-typed inputs display placeholder and tooltip explaining server-accessible paths (s3:// or absolute paths) | VERIFIED | `RunWorkflowDialog.tsx:344-352` — Info icon with TooltipContent: "Enter a server-accessible path. Browser file paths are not supported. Use s3://, /absolute/path..."; test "File input shows tooltip with server-accessible path explanation" passes |
| SC5 | After run completes, RunDetailPage shows output files reusing Phase 30 output viewer | HUMAN_NEEDED | `RunDetailPage.tsx` exists with full output viewer (useRunOutputs hook, Outputs section, file table); route `projects/:slug/runs/:id` registered in App.tsx. Cannot verify the dynamic output display after run completion without a running server and real workflow. |

**Score:** 4/5 truths verified (SC5 needs human verification for live output display)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `web/src/components/ui/checkbox.tsx` | shadcn Checkbox for Boolean inputs | VERIFIED | 28 lines; exports Checkbox wrapping @radix-ui/react-checkbox |
| `web/src/features/registry/useRegistry.ts` | useRegistryInputs + useRunWorkflow hooks | VERIFIED | 131 lines; exports RegistryInputItem, RegistryInputsResponse, useRegistryInputs, useRunWorkflow |
| `web/src/features/registry/RunWorkflowDialog.tsx` | Complete dialog with dynamic form | VERIFIED | 443 lines; substantive implementation with per-type controls, error mapping, navigation |
| `web/src/features/registry/__tests__/RunWorkflowDialog.test.tsx` | Unit tests (13+ required) | VERIFIED | 352 lines; 15 tests, all passing |
| `web/src/features/registry/RegistryEntryPage.tsx` | Run button + RunWorkflowDialog wiring | VERIFIED | 230 lines; imports RunWorkflowDialog, canRun gate, runOpen state, dialog rendered |
| `web/src/features/registry/__tests__/RegistryEntryPage.test.tsx` | Tests for Run button role gating | VERIFIED | 271 lines; 4 new role tests added (executor/admin/global-admin/viewer), all 14 tests pass |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| RunWorkflowDialog.tsx | useRegistry.ts | useRegistryInputs, useRunWorkflow hooks | WIRED | Imported at line 10-11; used at lines 93, 107 |
| RunWorkflowDialog.tsx | POST /projects/{slug}/workflows | apiClient.post in useRunWorkflow | WIRED | `useRegistry.ts:127` — `apiClient.post('/projects/${slug}/workflows', ...)` |
| RunWorkflowDialog.tsx | GET /projects/{slug}/registry/{name}/inputs | apiClient.get in useRegistryInputs | WIRED | `useRegistry.ts:118` — `apiClient.get('/projects/${slug}/registry/${name}/inputs?tag=...')` |
| RegistryEntryPage.tsx | RunWorkflowDialog.tsx | import + render with open/onOpenChange state | WIRED | Imported at line 11; rendered at line 221 with runOpen state |
| RegistryEntryPage.tsx | useAuthStore/useProjectMembers | role check for canRun | WIRED | `RegistryEntryPage.tsx:57,61` — globalRole from authStore; canRun computed |
| RunWorkflowDialog.tsx | RunDetailPage (route) | navigate(`/projects/${slug}/runs/${result.run_id}`) | WIRED | Route `projects/:slug/runs/:id` registered in App.tsx:60 |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|-------------------|--------|
| RunWorkflowDialog.tsx | inputsData | useRegistryInputs → GET /registry/{name}/inputs | Yes — live API fetch with enabled guard | FLOWING |
| RunWorkflowDialog.tsx | result.run_id | useRunWorkflow → POST /projects/{slug}/workflows | Yes — mutateAsync returns API response | FLOWING |
| RegistryEntryPage.tsx | versions | useRegistryVersions (existing hook from Phase 34) | Yes — passed as prop to RunWorkflowDialog | FLOWING |
| RunDetailPage.tsx | outputsData | useRunOutputs → GET /projects/{slug}/runs/{id}/outputs | Yes — live API fetch; outputs rendered in table | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| TypeScript compiles cleanly | `npx tsc --noEmit` (from /home/uehara/flowy/web) | EXIT: 0 | PASS |
| RunWorkflowDialog tests pass | `npx vitest run src/features/registry/__tests__/RunWorkflowDialog.test.tsx` | 15 passed / 0 failed | PASS |
| RegistryEntryPage tests pass | `npx vitest run src/features/registry/__tests__/RegistryEntryPage.test.tsx` | 14 passed / 0 failed | PASS |
| Module exports correct | grep exports in useRegistry.ts | useRegistryInputs, useRunWorkflow, RegistryInputItem, RegistryInputsResponse found | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| EXEC-01 | 36-01, 36-02 | User can select a workflow from registry to execute (dialog with entry/version selection) | SATISFIED | RunWorkflowDialog with version selector defaulting to "latest"; wired into RegistryEntryPage |
| EXEC-02 | 36-01 | User can input workflow parameters via dynamically generated form (String/Int/Float/Boolean/File/Array/Map/Struct controls) | SATISFIED | Dynamic zod schema + per-type controls implemented; 15 tests cover all types |
| EXEC-03 | 36-01 | User can submit workflow execution from the UI and see confirmation | SATISFIED | onSubmit calls mutateAsync; toast.success('Run submitted.'); navigates to RunDetailPage |
| EXEC-05 | 36-02 | User can view execution results and output files after completion (reuses Phase 30 output viewer) | NEEDS HUMAN | RunDetailPage exists with useRunOutputs + output file table; route wired; live output after run completion needs human verification |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| No anti-patterns found | — | — | — | No TODOs, FIXMEs, placeholder returns, or empty handlers found in any phase 36 artifacts |

### Human Verification Required

#### 1. Full Workflow Execution End-to-End Flow

**Test:** Start dev server (`make dev`). Log in as executor/admin user. Navigate to a project's Registry, click an entry with a tagged version. Click "Run". Fill in required fields. Submit.

**Expected:**
- "Run" button visible in entry header for executor/admin; absent for viewer role
- RunWorkflowDialog opens with title "Run Workflow", version selector defaulting to "latest"
- Dynamic form shows correct controls per WDL type
- Submitting shows toast "Run submitted." and navigates to RunDetailPage
- After run completes, RunDetailPage Outputs section shows output files

**Why human:** The full run lifecycle — POST /workflows returns run_id, daemon executes workflow, status polling reaches terminal state, outputs endpoint returns output files — cannot be exercised programmatically without a live server. The Phase 30 output viewer integration specifically requires an actual completed run to display.

### Gaps Summary

No gaps found in automated verification. The one human_needed item (SC5 — output display after run completion) is inherently a live-system behavior. All code paths, routes, hooks, and component wiring are verified in place. The RunDetailPage output viewer (Phase 30) is confirmed implemented and routed correctly.

---

_Verified: 2026-04-07T02:15:00Z_
_Verifier: Claude (gsd-verifier)_
