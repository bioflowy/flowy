---
phase: 31-audit-gap-closure
plan: "01"
subsystem: web-frontend
tags: [sidebar, navigation, auth, bug-fix, gap-closure]
dependency_graph:
  requires: []
  provides: [corrected-sidebar-members-link, auth-disabled-redirect]
  affects: [web/src/components/layout/Sidebar.tsx, web/src/features/auth/LoginPage.tsx]
tech_stack:
  added: []
  patterns: [NavLink-end-prop, empty-token-guard-redirect]
key_files:
  modified:
    - web/src/components/layout/Sidebar.tsx
    - web/src/features/auth/LoginPage.tsx
    - web/src/features/auth/__tests__/LoginPage.test.tsx
decisions:
  - "Sidebar Members and Overview both use end=true on their NavLink since they share the same path /projects/:slug; without end, Members would remain active while navigating to /registry or /runs"
  - "NavLink key changed from item.to (URL) to item.label to avoid React duplicate-key warning when Overview and Members share the same path"
  - "Auth-disabled redirect goes to / (not /login) — the auth-disabled backend requires no credentials, so sending to / lets ProtectedRoute determine the correct destination"
metrics:
  duration: "3m 7s"
  completed: "2026-04-07"
  tasks_completed: 2
  files_modified: 3
requirements_closed: [PROJ-03, PROJ-04, AUTH-01]
---

# Phase 31 Plan 01: Audit Gap Closure — Sidebar Members Link and Auth-Disabled Redirect Summary

Sidebar Members nav item corrected from unregistered `/projects/:slug/members` to `/projects/:slug` (ProjectDetailPage), and auth-disabled login now redirects to `/` instead of hanging silently.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Fix sidebar Members link and auth-disabled login redirect | b9de75d | Sidebar.tsx, LoginPage.tsx |
| 2 | Add test for auth-disabled login redirect | bb6ebb6 | LoginPage.test.tsx |

## What Was Built

**Task 1 — Two production fixes:**

1. `Sidebar.tsx` line 68: Changed Members nav item `to` from `/projects/${activeSlug}/members` to `/projects/${activeSlug}`. The members section is already rendered inside ProjectDetailPage (the Overview route) — there is no separate members route in App.tsx, making the old link a broken dead-end.

2. `Sidebar.tsx` NavLink `end` prop: Updated from `item.label === 'Overview'` to `item.label === 'Overview' || item.label === 'Members'` so Members does not remain falsely "active" when navigating child routes like `/registry` or `/runs`.

3. `Sidebar.tsx` NavLink `key`: Changed from `item.to` (URL) to `item.label` to avoid React duplicate-key warning since Overview and Members now share the same path.

4. `LoginPage.tsx` lines 53-55: Changed silent `if (!token) return` to an explicit redirect `navigate('/', { replace: true })`. When the backend is running in auth-disabled mode, `handleLoginDisabled` returns `token: ""`. Previously, the user was silently left on the login page with no feedback. Now they are redirected to the app root.

**Task 2 — New test:**

Added test case `'redirects to / when backend returns empty token (auth-disabled mode)'` to `LoginPage.test.tsx`. The test mocks `apiClient.post` returning `{ token: '' }`, verifies `mockNavigate` was called with `('/', { replace: true })`, and confirms the auth store `token` remains `null`.

All 19 test files and 112 tests pass.

## Deviations from Plan

None — plan executed exactly as written. The only minor addition was changing the NavLink `key` from `item.to` to `item.label` (not mentioned in the plan but required to avoid a React duplicate-key warning since Overview and Members now share the same `to` path).

## Verification Results

1. `grep "activeSlug}/members" web/src/components/layout/Sidebar.tsx` — returns NO matches (PASS)
2. `grep "navigate.*replace.*true" web/src/features/auth/LoginPage.tsx` — returns 2 matches (auth-disabled + normal login PASS)
3. `cd web && npx vitest run` — 19 test files passed, 112 tests passed, 0 failures (PASS)

## Self-Check: PASSED

- `/home/uehara/flowy/web/src/components/layout/Sidebar.tsx` — modified and committed (b9de75d)
- `/home/uehara/flowy/web/src/features/auth/LoginPage.tsx` — modified and committed (b9de75d)
- `/home/uehara/flowy/web/src/features/auth/__tests__/LoginPage.test.tsx` — modified and committed (bb6ebb6)
- Commits b9de75d and bb6ebb6 verified in git log
