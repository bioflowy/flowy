# Phase 26: Authentication - Research

**Researched:** 2026-03-24
**Domain:** React SPA authentication with JWT — login form, session store, protected routes, 401 interception, password change, logout
**Confidence:** HIGH

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| AUTH-01 | User can log in with username and password via login screen | Login form with shadcn/ui Input+Button; POST /api/auth/login; error message on 401 |
| AUTH-02 | JWT token is stored in session and injected into all API requests | Zustand store with `persist` middleware writing to `sessionStorage`; fetch wrapper reads token |
| AUTH-03 | User is automatically redirected to login on 401 response | `QueryClient` `QueryCache` global error handler; `ProtectedRoute` component wrapping all app routes |
| AUTH-04 | User can change their own password | Account page with ChangePasswordForm; PUT /api/users/me/password with `{current_password, new_password}` |
| AUTH-05 | User can log out (token cleared, redirect to login) | Zustand `clearAuth()` action + `useNavigate` to `/login` from any page |
</phase_requirements>

---

## Summary

Phase 26 adds authentication to the existing React SPA. The backend already provides a complete JWT auth API (`POST /auth/login`, `PUT /users/me/password`, `GET /users/me`) with HS256-signed tokens that expire in 7 days. The frontend work is entirely client-side: a login screen, a Zustand auth store persisted to `sessionStorage`, a fetch wrapper that injects the Bearer token, a `ProtectedRoute` layout component that gates all app routes, global 401 interception via TanStack Query's `QueryCache`, and a password-change form in account settings.

The key architectural constraint is already decided: **JWT stored in `sessionStorage`** (cleared on tab/window close, not `localStorage`). No httpOnly cookie path. The Zustand `persist` middleware with `createJSONStorage(() => sessionStorage)` implements this without additional libraries.

The existing `App.tsx` already demonstrates awareness of auth: the `QueryClient` retry function skips retries on 401/403. Phase 26 completes the picture by wiring up the `ProtectedRoute` wrapper, the auth store, and the login screen.

**Primary recommendation:** Implement in three sequential tasks: (1) Zustand auth store + fetch client wrapper + shadcn/ui Form/Input/Label components, (2) Login page + `ProtectedRoute` + routing wiring, (3) Account page with password-change form + logout action.

---

## Standard Stack

### Core

| Library | Version (installed) | Purpose | Why Standard |
|---------|---------------------|---------|--------------|
| zustand | 5.0.12 | Client-side auth state (token, user) | Already in `package.json`; v5 API (`create` without currying) |
| @tanstack/react-query | 5.95.2 | Server state; `QueryCache` for global 401 | Already in `package.json`; `QueryCache` onError is the v5 way |
| react-router | 7.13.2 | `ProtectedRoute`, `Navigate`, `useNavigate`, `useLocation` | Already in `package.json` |
| sonner | ^1.7.4 | Toast notifications for login errors/success | Already in `package.json`; `Toaster` already mounted in `App.tsx` |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| shadcn/ui `form` | CLI-added | Accessible form with react-hook-form integration | Login form + password-change form |
| shadcn/ui `input` | CLI-added | Styled text/password inputs | All form fields |
| shadcn/ui `label` | CLI-added | Accessible form labels | All form fields |
| shadcn/ui `card` | CLI-added | Login screen card container | Centered login card |
| shadcn/ui `dialog` | CLI-added | Change password modal | Account page |
| react-hook-form | peer dep of shadcn/ui form | Form state, validation, submission | Installed with shadcn form component |
| @hookform/resolvers | peer dep | Zod schema validation adapter | Installed with shadcn form component |
| zod | peer dep | Schema validation for form inputs | Login form + password-change form |

### New shadcn/ui Components to Install

```bash
cd web && pnpm dlx shadcn@latest add form input label card dialog
```

This installs `react-hook-form`, `@hookform/resolvers`, and `zod` as dependencies automatically.

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Zustand persist + sessionStorage | Manual `sessionStorage` calls in login/logout | Zustand persist is cleaner; auto-hydrates on load without manual `useEffect` |
| QueryCache global onError | Per-query `onError` callback | Per-query callbacks removed in TanStack Query v5; `QueryCache` is the v5 standard |
| ProtectedRoute layout component | Route loader with redirect | Loaders are the React Router v7 framework mode pattern; project uses library mode (`<BrowserRouter>`), so layout component is correct |

---

## Architecture Patterns

### Recommended Project Structure

```
web/src/
├── features/
│   └── auth/
│       ├── LoginPage.tsx          # Full-page login form
│       ├── ChangePasswordDialog.tsx  # Modal for account settings
│       └── useLogout.ts           # Hook: clearAuth() + navigate('/login')
├── components/
│   ├── layout/
│   │   ├── AppShell.tsx           # Existing — wraps authenticated app
│   │   ├── Sidebar.tsx            # Update: show username, logout button
│   │   └── ProtectedRoute.tsx     # NEW: redirects unauthenticated users
│   └── ui/
│       ├── form.tsx               # shadcn/ui (CLI-added)
│       ├── input.tsx              # shadcn/ui (CLI-added)
│       ├── label.tsx              # shadcn/ui (CLI-added)
│       ├── card.tsx               # shadcn/ui (CLI-added)
│       └── dialog.tsx             # shadcn/ui (CLI-added)
├── store/
│   └── authStore.ts               # Zustand store with persist middleware
├── lib/
│   ├── utils.ts                   # Existing
│   └── apiClient.ts               # NEW: fetch wrapper injecting Bearer token
└── App.tsx                        # Update: add /login route + ProtectedRoute wrapper
```

### Pattern 1: Zustand Auth Store with sessionStorage Persistence

**What:** Single Zustand slice holding `{ token, username, role }`. Persisted to `sessionStorage` so it survives page reload within the same tab but clears on tab close.

**When to use:** Always — this is the single source of truth for auth state across the app.

```typescript
// web/src/store/authStore.ts
import { create } from 'zustand'
import { persist, createJSONStorage } from 'zustand/middleware'

interface AuthState {
  token: string | null
  username: string | null
  role: string | null
  setAuth: (token: string, username: string, role: string) => void
  clearAuth: () => void
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      username: null,
      role: null,
      setAuth: (token, username, role) => set({ token, username, role }),
      clearAuth: () => set({ token: null, username: null, role: null }),
    }),
    {
      name: 'flowy-auth',
      storage: createJSONStorage(() => sessionStorage),
    }
  )
)
```

**Why `sessionStorage` not `localStorage`:** Project decision from STATE.md — "JWT stored in sessionStorage for v2.0 MVP; httpOnly cookie deferred."

### Pattern 2: API Client Fetch Wrapper

**What:** A module exporting typed fetch helpers that automatically inject the Bearer token from the Zustand store and throw typed errors (including a special `ApiError` with `.status`).

**When to use:** All API calls in the app use this wrapper — never raw `fetch`.

```typescript
// web/src/lib/apiClient.ts
import { useAuthStore } from '@/store/authStore'

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message)
    this.name = 'ApiError'
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const token = useAuthStore.getState().token
  const headers: HeadersInit = {
    'Content-Type': 'application/json',
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
    ...(init.headers ?? {}),
  }
  const res = await fetch(`/api${path}`, { ...init, headers })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new ApiError(res.status, body.error ?? res.statusText)
  }
  return res.json() as Promise<T>
}

export const apiClient = {
  post: <T>(path: string, body: unknown) =>
    request<T>(path, { method: 'POST', body: JSON.stringify(body) }),
  get: <T>(path: string) =>
    request<T>(path, { method: 'GET' }),
  put: <T>(path: string, body: unknown) =>
    request<T>(path, { method: 'PUT', body: JSON.stringify(body) }),
}
```

**Key detail:** `useAuthStore.getState().token` reads the store outside React — correct pattern for non-component code in Zustand v5.

### Pattern 3: Global 401 Interception via QueryCache

**What:** Configure `QueryClient` with a `QueryCache` `onError` handler that clears auth state and redirects to `/login` on any 401 response.

**When to use:** In `App.tsx` when constructing the `QueryClient`.

```typescript
// web/src/App.tsx (updated QueryClient construction)
import { QueryClient, QueryClientProvider, QueryCache, MutationCache } from '@tanstack/react-query'
import { useAuthStore } from '@/store/authStore'
import { ApiError } from '@/lib/apiClient'

// Must be outside component to avoid recreation on render
const handleAuthError = (error: unknown) => {
  if (error instanceof ApiError && error.status === 401) {
    useAuthStore.getState().clearAuth()
    // Navigation triggered by ProtectedRoute on next render (token === null)
  }
}

const queryClient = new QueryClient({
  queryCache: new QueryCache({ onError: handleAuthError }),
  mutationCache: new MutationCache({ onError: handleAuthError }),
  defaultOptions: {
    queries: {
      retry: (failureCount, error) => {
        if (error instanceof ApiError && (error.status === 401 || error.status === 403)) return false
        return failureCount < 2
      },
    },
  },
})
```

**Why `clearAuth()` triggers redirect:** `ProtectedRoute` reads `token` from the store. When `clearAuth()` sets `token: null`, the next render of `ProtectedRoute` redirects to `/login` automatically. No direct `navigate()` call needed from the QueryCache handler (which runs outside React render).

### Pattern 4: ProtectedRoute Component

**What:** A layout route component that checks `token` from the auth store. Unauthenticated users are sent to `/login` with the original URL saved in location state for post-login redirect.

**When to use:** Wrap all authenticated routes in `App.tsx`.

```typescript
// web/src/components/layout/ProtectedRoute.tsx
import { Navigate, Outlet, useLocation } from 'react-router'
import { useAuthStore } from '@/store/authStore'

export function ProtectedRoute() {
  const token = useAuthStore((s) => s.token)
  const location = useLocation()

  if (!token) {
    return <Navigate to="/login" state={{ from: location }} replace />
  }

  return <Outlet />
}
```

### Pattern 5: Login Page with Post-Login Redirect

**What:** Full-page centered card with username/password form. On success, reads `state.from` from location state and redirects to it (or `/` as fallback).

```typescript
// web/src/features/auth/LoginPage.tsx (sketch)
import { useNavigate, useLocation } from 'react-router'
import { useAuthStore } from '@/store/authStore'
import { apiClient } from '@/lib/apiClient'

// After successful login:
const { token } = await apiClient.post<{ token: string }>('/auth/login', { username, password })
// Decode username/role from JWT payload (base64 decode claims)
const payload = JSON.parse(atob(token.split('.')[1]))
useAuthStore.getState().setAuth(token, payload.username, payload.role)
const from = (location.state as { from?: Location })?.from?.pathname ?? '/'
navigate(from, { replace: true })
```

**JWT claims available:** The backend JWT payload contains `sub` (userID), `username`, `role` — all can be extracted client-side without an extra `/users/me` call at login.

### Pattern 6: App.tsx Route Structure

```typescript
// Routes structure in App.tsx
<Routes>
  <Route path="/login" element={<LoginPage />} />
  <Route element={<ProtectedRoute />}>
    <Route element={<AppShell />}>
      <Route index element={<Navigate to="/projects" replace />} />
      <Route path="account" element={<AccountPage />} />
      <Route path="*" element={<div>Select a page from the sidebar.</div>} />
    </Route>
  </Route>
</Routes>
```

**Key:** `ProtectedRoute` wraps `AppShell` — this means the sidebar and layout are only rendered when authenticated. `/login` is outside both `ProtectedRoute` and `AppShell`.

### Anti-Patterns to Avoid

- **Using `localStorage` instead of `sessionStorage`:** The project decision is `sessionStorage`. Using `localStorage` would keep the token alive after the browser is closed, which is not the intended v2.0 security posture.
- **Calling `navigate()` inside `QueryCache.onError`:** The `QueryCache` handler runs outside the React render tree — `useNavigate()` cannot be called there. The indirect approach (clearAuth → ProtectedRoute re-renders → Navigate) is correct.
- **Passing the token via React context or prop drilling:** Use `useAuthStore.getState().token` in `apiClient.ts` (outside React). Components needing the token for display use `useAuthStore((s) => s.token)`.
- **Storing user profile beyond what's in the JWT:** The JWT contains `username` and `role`. Avoid a separate "user profile" API call at login startup — decode the JWT claims directly.
- **Mounting `ProtectedRoute` inside `AppShell`:** `ProtectedRoute` must be the outer component so the redirect happens before `AppShell` renders. Nesting it inside breaks the redirect behavior.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Form state management | Custom useState per field + validation | shadcn/ui `form` (react-hook-form + zod) | Built-in validation, accessibility, error display |
| sessionStorage sync | Manual `useEffect` to read/write storage | Zustand `persist` middleware | Auto-hydration, single source of truth, no race conditions |
| JWT decode for claims | Custom base64 + JSON parse | `atob(token.split('.')[1])` is fine for display — no crypto needed | JWTs are signed, not encrypted; client can read claims without verification |
| Global 401 handling | Per-query error checks in every component | `QueryCache` / `MutationCache` `onError` | Single place, catches all queries and mutations |

**Key insight:** The auth pattern here is "read token from store, inject as header" — avoid libraries like `axios` or `ky` just to get interceptors. The project stack already uses plain `fetch`; a thin `apiClient.ts` wrapper provides the same capability.

---

## Common Pitfalls

### Pitfall 1: Zustand Store Initialized Before SessionStorage Hydration

**What goes wrong:** Components render before Zustand's persist middleware finishes hydrating from `sessionStorage`. `token` is `null` for one tick, causing a flash redirect to `/login` even for logged-in users.

**Why it happens:** Zustand persist hydration is asynchronous in some configurations.

**How to avoid:** Use `useAuthStore.persist.hasHydrated()` or Zustand's `onFinishHydration` to show a loading spinner before rendering routes. In practice with `sessionStorage` (synchronous API), `createJSONStorage(() => sessionStorage)` hydrates synchronously — the flash does not occur. No explicit hydration guard needed.

**Warning signs:** Users briefly see the login page flash before the app loads.

### Pitfall 2: QueryCache Handler Created Inside Component

**What goes wrong:** `new QueryClient(...)` is created inside the `App` component function body, recreating the client on every render and wiping the cache.

**Why it happens:** Forgetting to hoist `queryClient` outside the component.

**How to avoid:** Declare `queryClient` at module scope (outside the `App` function), as the existing `App.tsx` already does.

### Pitfall 3: Login Redirect Loop

**What goes wrong:** The `/login` route is nested inside `ProtectedRoute`. Unauthenticated users are redirected to `/login`, which triggers `ProtectedRoute` again, causing an infinite redirect.

**Why it happens:** Putting the login route inside the protected group.

**How to avoid:** The `/login` `<Route>` must be a sibling of the `ProtectedRoute` element, not a child. See Pattern 6 above.

### Pitfall 4: `state.from` Is Not Preserved on Hard Refresh

**What goes wrong:** After a hard refresh, `location.state` is lost. If `ProtectedRoute` redirects to `/login` with `state: { from }`, and the user refreshes the login page, `state.from` is undefined.

**Why it happens:** `location.state` is only preserved in the same navigation session.

**How to avoid:** Default to `/` when `state.from` is undefined. This is already handled in Pattern 5 (`?? '/'`).

### Pitfall 5: Password Inputs Not Cleared After Change

**What goes wrong:** After a successful password change, the form retains the old values. If the user re-opens the dialog it shows stale data.

**Why it happens:** `react-hook-form` values persist until explicitly reset.

**How to avoid:** Call `form.reset()` in the `onSuccess` handler after the password change mutation resolves.

### Pitfall 6: Backend Auth Enabled Check

**What goes wrong:** `flowy-server` has `auth.enabled` config flag. If it is `false`, `POST /auth/login` returns `{ token: "" }` — an empty token. The frontend must not store an empty string as a valid token.

**Why it happens:** The server supports auth-disabled mode (for backward compatibility with existing daemon setup).

**How to avoid:** After login, check `if (!token)` before calling `setAuth()`. If the token is empty string, treat as unauthenticated. In practice the UI is v2.0-only and will be used with `auth.enabled: true`, but defensive coding prevents subtle bugs.

---

## Code Examples

### Login API Call

```typescript
// POST /api/auth/login
// Request: { username: string, password: string }
// Response: { token: string }
// Error 401: { error: "invalid credentials" } or { error: "account disabled" }
const response = await apiClient.post<{ token: string }>('/auth/login', {
  username,
  password,
})
```

### Change Password API Call

```typescript
// PUT /api/users/me/password
// Request: { current_password: string, new_password: string }
// Response: 204 No Content
// Error 401: { error: "current password is incorrect" }
await apiClient.put('/users/me/password', {
  current_password: currentPassword,
  new_password: newPassword,
})
```

### Decode JWT Claims (no library needed)

```typescript
// JWT payload: { sub: userID, username: string, role: string, exp: number }
function decodeJWTPayload(token: string): { username: string; role: string } {
  const payload = JSON.parse(atob(token.split('.')[1]))
  return { username: payload.username, role: payload.role }
}
```

### Logout Hook

```typescript
// web/src/features/auth/useLogout.ts
import { useNavigate } from 'react-router'
import { useAuthStore } from '@/store/authStore'

export function useLogout() {
  const clearAuth = useAuthStore((s) => s.clearAuth)
  const navigate = useNavigate()
  return () => {
    clearAuth()
    navigate('/login', { replace: true })
  }
}
```

---

## API Contract (Backend — Already Implemented)

All endpoints are proxied via `/api/*` by `flowy-ui`.

| Method | Path | Auth | Request | Response | Errors |
|--------|------|------|---------|----------|--------|
| POST | `/api/auth/login` | None | `{username, password}` | `{token}` | 401 invalid credentials / account disabled |
| GET | `/api/users/me` | Bearer | — | `UserResponse` | 401 missing/invalid token |
| PUT | `/api/users/me/password` | Bearer | `{current_password, new_password}` | 204 | 401 incorrect current password |

`UserResponse` fields: `id`, `username`, `display_name`, `email`, `global_role`, `is_active`, `created_at`, `updated_at`.

JWT token expiry: 7 days (set by backend `IssueToken`). No refresh token mechanism exists — on expiry the user must log in again (401 is returned; frontend clears session and redirects).

---

## State of the Art

| Old Approach | Current Approach | Impact |
|--------------|------------------|--------|
| TanStack Query v4 per-query `onError` callback | v5 `QueryCache`/`MutationCache` `onError` | `onError` removed from `useQuery` in v5 — must use cache-level handlers |
| Zustand v4 curried `create<State>()()` syntax | v5 simplified `create<State>()(...)` (same syntax) | Zustand v4→v5 migration: TypeScript inference improved; no API changes needed here |
| React Router v6 `<Navigate>` in route element | v7 same pattern + `useNavigate` hook | Both still valid in v7 library mode (`<BrowserRouter>`) |

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | None detected in `web/` — no vitest/jest config exists |
| Config file | None — Wave 0 must create `web/vitest.config.ts` |
| Quick run command | `cd web && pnpm test --run` (after Wave 0 setup) |
| Full suite command | `cd web && pnpm test --run` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| AUTH-01 | Login form submits credentials, shows error on 401 | unit (component) | `pnpm test --run src/features/auth/LoginPage.test.tsx` | Wave 0 |
| AUTH-02 | apiClient injects Bearer token from store | unit | `pnpm test --run src/lib/apiClient.test.ts` | Wave 0 |
| AUTH-03 | ProtectedRoute redirects unauthenticated users | unit (component) | `pnpm test --run src/components/layout/ProtectedRoute.test.tsx` | Wave 0 |
| AUTH-03 | QueryCache clears auth and triggers redirect on 401 | unit | `pnpm test --run src/App.test.tsx` | Wave 0 |
| AUTH-04 | ChangePasswordDialog submits correct payload, resets form on success | unit (component) | `pnpm test --run src/features/auth/ChangePasswordDialog.test.tsx` | Wave 0 |
| AUTH-05 | Logout clears store and navigates to /login | unit | `pnpm test --run src/features/auth/useLogout.test.ts` | Wave 0 |

### Sampling Rate

- **Per task commit:** `cd web && pnpm test --run` (all unit tests, ~5s)
- **Per wave merge:** `cd web && pnpm test --run`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `web/vitest.config.ts` — vitest configuration
- [ ] `web/src/test/setup.ts` — jsdom setup, `@testing-library/jest-dom` matchers
- [ ] `web/src/features/auth/LoginPage.test.tsx` — covers AUTH-01
- [ ] `web/src/lib/apiClient.test.ts` — covers AUTH-02
- [ ] `web/src/components/layout/ProtectedRoute.test.tsx` — covers AUTH-03
- [ ] `web/src/App.test.tsx` — covers AUTH-03 (401 global interception)
- [ ] `web/src/features/auth/ChangePasswordDialog.test.tsx` — covers AUTH-04
- [ ] `web/src/features/auth/useLogout.test.ts` — covers AUTH-05
- [ ] Framework install: `cd web && pnpm add -D vitest @vitejs/plugin-react @testing-library/react @testing-library/user-event @testing-library/jest-dom jsdom`

---

## Open Questions

1. **JWT token expiry UX**
   - What we know: Token expires after 7 days; on expiry the API returns 401; the `QueryCache` handler clears auth and `ProtectedRoute` redirects to `/login`.
   - What's unclear: Should the app proactively check token expiry client-side (decode `exp` claim) to show an "your session expires in X minutes" warning?
   - Recommendation: Skip proactive expiry UI for v2.0. The 401 interception covers the case. The 7-day window is long enough that expiry during a session is rare.

2. **`flowy-server` auth.enabled configuration**
   - What we know: The server can run with `auth.enabled: false`. In this mode `POST /auth/login` returns `{ token: "" }`.
   - What's unclear: Will the development environment always have auth enabled?
   - Recommendation: Defensive check in the login handler (if `token === ""`, skip `setAuth`) prevents issues. Document as a pitfall.

---

## Sources

### Primary (HIGH confidence)

- Direct inspection of `/home/uehara/flowy/internal/server/handlers_auth.go` — confirmed all auth API endpoints, request/response shapes
- Direct inspection of `/home/uehara/flowy/internal/auth/auth.go` — confirmed JWT claims structure (`sub`, `username`, `role`), 7-day expiry, HS256
- Direct inspection of `/home/uehara/flowy/internal/api/types.go` — confirmed `LoginRequest`, `LoginResponse`, `ChangePasswordRequest` DTO shapes
- Direct inspection of `/home/uehara/flowy/web/package.json` + `node_modules` — confirmed installed versions: react-router 7.13.2, zustand 5.0.12, @tanstack/react-query 5.95.2
- `.planning/STATE.md` — confirmed project decision: JWT in sessionStorage, AllowCredentials=false
- TanStack Query v5 docs (GitHub discussions) — confirmed `QueryCache`/`MutationCache` `onError` is the v5 pattern; per-query `onError` removed in v5

### Secondary (MEDIUM confidence)

- WebSearch: React Router v7 protected routes — confirmed `<Navigate>` + `useLocation` + `state.from` pattern is standard
- WebSearch: Zustand v5 persist middleware + sessionStorage — confirmed `createJSONStorage(() => sessionStorage)` API
- WebSearch: TanStack Query v5 + 401 handling — confirmed `QueryCache onError` approach, `QueryClient` must be hoisted outside component

### Tertiary (LOW confidence)

- None — all critical claims verified against source code or official docs.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all library versions verified from installed `node_modules`
- API contract: HIGH — verified directly from Go source files in this repo
- Architecture patterns: HIGH — derived from project decisions in STATE.md + React Router v7 / Zustand v5 standard patterns
- Pitfalls: HIGH — derived from known v5 API changes and project-specific backend behavior (auth.enabled flag)

**Research date:** 2026-03-24
**Valid until:** 2026-06-24 (stable libraries; 90 days)
