---
phase: 26
slug: authentication
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-24
---

# Phase 26 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | vitest |
| **Config file** | web/vitest.config.ts (Wave 0 installs) |
| **Quick run command** | `cd web && pnpm test -- --run` |
| **Full suite command** | `cd web && pnpm test -- --run --coverage` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd web && pnpm test -- --run`
- **After every plan wave:** Run `cd web && pnpm test -- --run --coverage`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 26-01-01 | 01 | 0 | ALL | infra | `cd web && pnpm test -- --run` | ❌ W0 | ⬜ pending |
| 26-02-01 | 02 | 1 | AUTH-01 | unit+integration | `cd web && pnpm test -- --run` | ❌ W0 | ⬜ pending |
| 26-02-02 | 02 | 1 | AUTH-02 | unit | `cd web && pnpm test -- --run` | ❌ W0 | ⬜ pending |
| 26-03-01 | 03 | 2 | AUTH-03, AUTH-04 | unit+integration | `cd web && pnpm test -- --run` | ❌ W0 | ⬜ pending |
| 26-03-02 | 03 | 2 | AUTH-05 | unit | `cd web && pnpm test -- --run` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `web/vitest.config.ts` — Vitest configuration with jsdom environment
- [ ] `web/src/test/setup.ts` — Test setup (cleanup, mocks)
- [ ] `vitest` + `@testing-library/react` + `@testing-library/jest-dom` + `jsdom` — devDependencies
- [ ] `web/src/lib/__tests__/auth-store.test.ts` — Stubs for auth store tests
- [ ] `web/src/components/__tests__/login-form.test.tsx` — Stubs for login form tests

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Login redirect after session expiry | AUTH-04 | Requires browser session + timing | 1. Login, 2. Clear sessionStorage, 3. Navigate, 4. Verify redirect to /login |
| Password change flow | AUTH-05 | End-to-end with live backend | 1. Login, 2. Go to Account, 3. Change password, 4. Logout, 5. Login with new password |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
