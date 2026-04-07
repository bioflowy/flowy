---
phase: 29
slug: registry-browsing
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-07
---

# Phase 29 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | vitest |
| **Config file** | `web/vitest.config.ts` |
| **Quick run command** | `cd web && npx vitest run --reporter=verbose` |
| **Full suite command** | `cd web && npx vitest run --reporter=verbose` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd web && npx vitest run --reporter=verbose`
- **After every plan wave:** Run `cd web && npx vitest run --reporter=verbose`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | REG-01 | unit | `cd web && npx vitest run` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REG-02 | unit | `cd web && npx vitest run` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REG-03 | unit | `cd web && npx vitest run` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | REG-04 | unit | `cd web && npx vitest run` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `web/src/features/registry/__tests__/` — test directory for registry feature
- [ ] Test stubs for REG-01 through REG-04

*Existing infrastructure covers framework and config.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| WDL source code renders with line numbers | REG-03 | Visual rendering verification | Navigate to entry detail, select version, verify code block has line numbers and copy button |
| Tag badge appears on entry in list after creation | REG-04 | Cross-page state verification | Create tag on entry detail, navigate back to list, verify tag appears |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
