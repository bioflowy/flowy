---
phase: 27
slug: user-management
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-06
---

# Phase 27 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | vitest (frontend), go test (backend) |
| **Config file** | `web/vitest.config.ts` (frontend), none (backend — standard go test) |
| **Quick run command** | `cd web && pnpm test -- --run` |
| **Full suite command** | `cd web && pnpm test -- --run && cd .. && go test ./internal/server/... -run TestUser -count=1` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd web && pnpm test -- --run`
- **After every plan wave:** Run full suite command
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 27-01-01 | 01 | 1 | USER-03, USER-04 | unit (Go) | `go test ./internal/server/... -run TestReactivate -count=1` | ❌ W0 | ⬜ pending |
| 27-01-02 | 01 | 1 | USER-04 | unit (Go) | `go test ./internal/server/... -run TestAdminResetPassword -count=1` | ❌ W0 | ⬜ pending |
| 27-02-01 | 02 | 2 | USER-01 | unit (TS) | `cd web && pnpm test -- --run src/features/users/` | ❌ W0 | ⬜ pending |
| 27-02-02 | 02 | 2 | USER-02 | unit (TS) | `cd web && pnpm test -- --run src/features/users/` | ❌ W0 | ⬜ pending |
| 27-03-01 | 03 | 3 | USER-03, USER-04 | unit (TS) | `cd web && pnpm test -- --run src/features/users/` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements. vitest and go test are both already configured.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Admin sees users table with correct data | USER-01 | Full browser rendering | Navigate to /users as admin, verify table loads |
| Create user dialog flow | USER-02 | Form interaction + toast | Click Create User, fill form, verify toast + table refresh |
| Deactivate confirmation dialog | USER-03 | Dialog interaction | Click Deactivate in dropdown, verify confirm dialog, accept |
| Sidebar hides Users for non-admin | USER-01 | Role-based rendering | Log in as non-admin, verify no Users nav item |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
