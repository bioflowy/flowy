---
phase: 28
slug: project-management
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-06
---

# Phase 28 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Vitest (vitest.config.ts in web/) |
| **Config file** | `web/vitest.config.ts` |
| **Quick run command** | `cd web && pnpm test --run` |
| **Full suite command** | `cd web && pnpm test --run` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd web && pnpm test --run`
- **After every plan wave:** Run `cd web && pnpm test --run`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 28-01-01 | 01 | 1 | PROJ-01 | unit | `cd web && pnpm test --run useProjects` | ❌ W0 | ⬜ pending |
| 28-01-01 | 01 | 1 | PROJ-02 | unit | `cd web && pnpm test --run useProjects` | ❌ W0 | ⬜ pending |
| 28-01-01 | 01 | 1 | PROJ-03 | unit | `cd web && pnpm test --run useProjects` | ❌ W0 | ⬜ pending |
| 28-02-01 | 02 | 1 | PROJ-01 | unit | `cd web && pnpm test --run ProjectsPage` | ❌ W0 | ⬜ pending |
| 28-02-01 | 02 | 1 | PROJ-02 | unit | `cd web && pnpm test --run CreateProjectPage` | ❌ W0 | ⬜ pending |
| 28-03-01 | 03 | 2 | PROJ-03 | unit | `cd web && pnpm test --run ProjectDetailPage` | ❌ W0 | ⬜ pending |
| 28-03-01 | 03 | 2 | PROJ-03 | unit | `cd web && pnpm test --run AddMemberDialog` | ❌ W0 | ⬜ pending |
| 28-03-01 | 03 | 2 | PROJ-04 | unit | `cd web && pnpm test --run ProjectDetailPage` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `web/src/features/projects/__tests__/useProjects.test.ts` — stubs for PROJ-01, PROJ-02, PROJ-03 hooks
- [ ] `web/src/features/projects/__tests__/ProjectsPage.test.tsx` — stubs for PROJ-01 render
- [ ] `web/src/features/projects/__tests__/CreateProjectPage.test.tsx` — stubs for PROJ-02
- [ ] `web/src/features/projects/__tests__/ProjectDetailPage.test.tsx` — stubs for PROJ-03, PROJ-04
- [ ] `web/src/features/projects/__tests__/AddMemberDialog.test.tsx` — stubs for PROJ-03 add member

*Existing infrastructure covers framework needs — only test stubs required.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Card grid visual layout | PROJ-01 | CSS responsive grid layout | Open /projects, verify cards in grid |
| Sidebar project dropdown | PROJ-01 | Interaction with sidebar state | Select project, verify nav items appear |
| Slug auto-generation | PROJ-02 | Real-time input behavior | Type name, verify slug updates live |
| Role badge styling | PROJ-04 | Visual badge appearance | View member table, verify role badges |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
