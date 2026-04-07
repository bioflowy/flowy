---
phase: 26-authentication
plan: "01"
subsystem: auth
tags: [zustand, vitest, jsdom, shadcn-ui, react-hook-form, zod, testing-library]

# Dependency graph
requires:
  - phase: 25-infrastructure-foundation
    provides: "React 19 + Vite 6 + Tailwind v4 scaffold with path aliases and shadcn/ui"
provides:
  - "Zustand auth store with sessionStorage persistence (useAuthStore)"
  - "API client with automatic Bearer token injection (apiClient)"
  - "ApiError class with status property for 401 detection"
  - "Vitest test infrastructure with jsdom environment"
  - "shadcn/ui form, input, label, card, dialog components"
affects: [26-02-PLAN, 26-03-PLAN, phase-27, phase-28]

# Tech tracking
tech-stack:
  added: [vitest@3.2.4, "@testing-library/react@16.3.2", "@testing-library/jest-dom@6.9.1", "@testing-library/user-event@14.6.1", jsdom@25.0.1, react-hook-form@7.72.1, zod@4.3.6, "@hookform/resolvers@5.2.2"]
  patterns: [zustand-persist-sessionStorage, fetch-wrapper-with-token-injection, tdd-red-green-refactor]

key-files:
  created:
    - web/src/store/authStore.ts
    - web/src/lib/apiClient.ts
    - web/vitest.config.ts
    - web/src/test/setup.ts
    - web/src/store/__tests__/authStore.test.ts
    - web/src/lib/__tests__/apiClient.test.ts
    - web/src/components/ui/form.tsx
    - web/src/components/ui/input.tsx
    - web/src/components/ui/label.tsx
    - web/src/components/ui/card.tsx
    - web/src/components/ui/dialog.tsx
  modified:
    - web/package.json
    - web/tsconfig.app.json

key-decisions:
  - "jsdom v25 used instead of v29 due to top-level await ESM incompatibility with vitest worker pools"
  - "Manual jest-dom matchers setup (expect.extend) instead of auto-setup import for ESM compatibility"
  - "Test files excluded from tsconfig.app.json to prevent build errors from test-only imports"
  - "vitest pool set to threads (default) for ESM compatibility"

patterns-established:
  - "Test setup: vitest.config.ts with jsdom, globals, path aliases; setup.ts clears sessionStorage"
  - "Auth store: Zustand persist middleware with sessionStorage under 'flowy-auth' key"
  - "API client: fetch wrapper reading token from useAuthStore.getState().token"
  - "Error handling: ApiError class with status field, parsed from response .error field"

requirements-completed: [AUTH-02]

# Metrics
duration: 7min
completed: 2026-04-06
---

# Phase 26 Plan 01: Auth Foundation Summary

**Zustand auth store with sessionStorage persistence, fetch-based API client with Bearer token injection, vitest+jsdom test infrastructure, and 5 shadcn/ui form components**

## Performance

- **Duration:** 7 min
- **Started:** 2026-04-06T01:26:45Z
- **Completed:** 2026-04-06T01:33:20Z
- **Tasks:** 2
- **Files modified:** 13

## Accomplishments
- Vitest configured with jsdom environment, testing-library, and jest-dom matchers
- Auth store persists token/username/role to sessionStorage under 'flowy-auth' key with hydration support
- API client automatically injects Bearer token from store into all fetch requests
- ApiError class thrown on non-2xx responses with status code for downstream 401 detection
- All 10 unit tests passing (4 authStore + 6 apiClient)
- 5 shadcn/ui components installed: form, input, label, card, dialog

## Task Commits

Each task was committed atomically:

1. **Task 1: Install test infrastructure and shadcn/ui components** - `962d04a` (feat)
2. **Task 2: Create auth store and API client** - `4e091b1` (test: RED), `383ebc9` (feat: GREEN), `78e24ad` (fix: build compat)

_Note: Task 2 followed TDD with separate RED and GREEN commits._

## Files Created/Modified
- `web/src/store/authStore.ts` - Zustand auth store with sessionStorage persistence
- `web/src/lib/apiClient.ts` - Fetch wrapper with Bearer token injection and ApiError
- `web/vitest.config.ts` - Vitest config with jsdom, globals, path aliases
- `web/src/test/setup.ts` - Test setup with jest-dom matchers and sessionStorage cleanup
- `web/src/store/__tests__/authStore.test.ts` - 4 tests: setAuth, clearAuth, persistence, hydration
- `web/src/lib/__tests__/apiClient.test.ts` - 6 tests: GET/POST, token injection, error handling
- `web/src/components/ui/form.tsx` - shadcn/ui form component (react-hook-form integration)
- `web/src/components/ui/input.tsx` - shadcn/ui input component
- `web/src/components/ui/label.tsx` - shadcn/ui label component
- `web/src/components/ui/card.tsx` - shadcn/ui card component
- `web/src/components/ui/dialog.tsx` - shadcn/ui dialog component
- `web/package.json` - Added test/form dependencies and test script
- `web/tsconfig.app.json` - Excluded test files from build

## Decisions Made
- Used jsdom v25 instead of v29: jsdom v29 has top-level await that causes ESM/CJS incompatibility with vitest's worker pool mechanism
- Manual jest-dom matchers setup via `expect.extend(matchers)` instead of `@testing-library/jest-dom/vitest` auto-setup for ESM compatibility
- Excluded test directories from tsconfig.app.json to prevent `noUnusedLocals` errors on test-only variables during production build
- shadcn/ui CLI placed files in literal `@/` directory (alias not resolved); manually moved to `src/components/ui/`

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] jsdom v29 ESM incompatibility**
- **Found during:** Task 2 (running tests)
- **Issue:** jsdom v29 contains top-level await which prevents vitest forks/threads pool from loading test workers
- **Fix:** Downgraded jsdom from v29 to v25
- **Files modified:** web/package.json, web/pnpm-lock.yaml
- **Verification:** All 10 tests pass, build succeeds
- **Committed in:** 383ebc9 (Task 2 commit)

**2. [Rule 3 - Blocking] shadcn/ui CLI literal `@/` directory**
- **Found during:** Task 1 (installing shadcn components)
- **Issue:** shadcn CLI created files under `web/@/components/ui/` instead of resolving `@` alias to `src/`
- **Fix:** Manually copied files to `web/src/components/ui/` and removed the literal `@/` directory
- **Files modified:** 5 component files moved
- **Verification:** Files exist in correct location, build succeeds
- **Committed in:** 962d04a (Task 1 commit)

**3. [Rule 1 - Bug] Test files included in TypeScript build**
- **Found during:** Task 2 (build verification)
- **Issue:** tsconfig.app.json included test files causing `noUnusedLocals` error on test-only variables
- **Fix:** Added exclude patterns for `__tests__`, `*.test.*`, `*.spec.*`, and `test/` directories
- **Files modified:** web/tsconfig.app.json
- **Verification:** Build passes, tests pass
- **Committed in:** 78e24ad

---

**Total deviations:** 3 auto-fixed (1 bug, 2 blocking)
**Impact on plan:** All auto-fixes necessary for correctness and build compatibility. No scope creep.

## Issues Encountered
None beyond the auto-fixed deviations above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Auth store and API client ready for Plan 02 (login form) and Plan 03 (password change dialog)
- All shadcn/ui form components available for building auth UI
- Test infrastructure ready for component testing with react testing library
- Pattern established: useAuthStore.getState().token provides token to apiClient automatically

---
*Phase: 26-authentication*
*Completed: 2026-04-06*
