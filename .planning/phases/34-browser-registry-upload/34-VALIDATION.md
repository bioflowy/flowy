---
phase: 34
slug: browser-registry-upload
status: draft
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-07
---

# Phase 34 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | vitest (via `npx vitest run`) |
| **Config file** | web/vite.config.ts |
| **Quick run command** | `cd web && npx vitest run src/features/registry/PushWDLDialog.test.tsx --reporter=verbose` |
| **Full suite command** | `cd web && npx vitest run --reporter=verbose` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run quick command
- **After every plan wave:** Run full suite + `cd web && npx tsc --noEmit`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 34-01-01 | 01 | 1 | REG-UI-01 | — | N/A | build | `cd web && npx tsc --noEmit` | ✅ | ⬜ pending |
| 34-01-02 | 01 | 1 | REG-UI-01 | — | N/A | build | `cd web && npx tsc --noEmit` | ✅ | ⬜ pending |
| 34-01-03 | 01 | 1 | REG-UI-01 | — | .wdl validation | unit | `cd web && npx vitest run src/features/registry --reporter=verbose` | TDD | ⬜ pending |
| 34-02-01 | 02 | 2 | REG-UI-01 | — | Role gating (executor+) | unit | `cd web && npx vitest run src/features/registry --reporter=verbose` | TDD | ⬜ pending |
| 34-02-02 | 02 | 2 | REG-UI-01 | — | Role gating tests | unit | `cd web && npx vitest run src/features/registry --reporter=verbose` | TDD | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

*No Wave 0 needed. `tsc --noEmit` is the per-task fast gate for Tasks 1-2. Tests are created via TDD in Task 3 (Plan 01) and Tasks 1-2 (Plan 02). Full vitest suite runs as wave-level gate.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| File picker opens native dialog | REG-UI-01 | jsdom lacks native file dialog | Click file input, verify native dialog opens |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
