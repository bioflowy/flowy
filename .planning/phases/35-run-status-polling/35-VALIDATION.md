---
phase: 35
slug: run-status-polling
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-08
---

# Phase 35 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | vitest |
| **Config file** | web/vitest.config.ts |
| **Quick run command** | `cd web && npx vitest run --reporter=verbose src/features/runs/` |
| **Full suite command** | `cd web && npx vitest run --reporter=verbose` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd web && npx vitest run --reporter=verbose src/features/runs/`
- **After wave completion:** Run full suite + `cd web && npx tsc --noEmit`

---

## Validation Architecture

### Wave 0: Test Infrastructure
- Verify vitest config handles React Query mocking (QueryClientProvider in test utils)
- Ensure fake timers work with refetchInterval tests

### Wave 1: Core Polling Logic
- `usePollingRuns` hook returns correct refetchInterval based on run states
- Polling stops (returns false) when all runs are terminal
- Polling active (returns 3000) when any run is non-terminal

### Wave 2: Page Integration
- RunDetailPage renders pulsing dot for PENDING/RUNNING
- RunDetailPage duration timer updates every 1 second
- RunsPage auto-refreshes while non-terminal runs exist
- Already-completed run detail page never triggers polling
