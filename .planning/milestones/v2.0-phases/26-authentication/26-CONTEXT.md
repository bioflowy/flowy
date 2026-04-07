# Phase 26: Authentication - Context

**Gathered:** 2026-04-06
**Status:** Ready for planning

<domain>
## Phase Boundary

Users can log in with username and password, maintain a JWT session across the app, and are automatically redirected to login when their session expires or they access protected routes unauthenticated. Users can change their own password and log out from any page.

</domain>

<decisions>
## Implementation Decisions

### Session lifecycle
- JWT stored in sessionStorage (decided in v2.0 architecture)
- No silent refresh — token expiry redirects to login (security-first for a tool that may handle sensitive research data)
- Session duration: 8 hours (one working day)
- Multi-tab sync via storage event: logout in one tab clears all tabs and redirects to login
- No refresh endpoint needed on the backend

### Error UX
- Login failure: always generic "Invalid username or password" — never reveal whether username exists
- Deactivated user login: same generic message (consistent security posture)
- 401 mid-session: silent redirect to login with no toast/message — return-to-URL preserved
- Network errors on login (server unreachable, timeout): show inline in the form error area, same location as credential errors

### Post-login flow
- Default landing page: `/projects` (consistent with existing App.tsx redirect)
- Return-to-URL: store attempted path in query param (`/login?redirect=/projects/foo`), navigate there after successful login
- Sidebar user menu: username + dropdown at bottom of Sidebar component with Logout and Change Password items
- Initial auth check loading state: blank screen briefly (sessionStorage check is <100ms, no spinner needed)

### Password change UX
- Dialog overlay triggered from sidebar user menu (matches UI-SPEC wireframe)
- After success: stay logged in, close dialog, show success toast via Sonner
- Client-side validation: minimum 8 characters only; backend handles additional validation if any
- Form fields: current password, new password, confirm new password

### Claude's Discretion
- Exact auth context/provider implementation pattern
- API client interceptor design for token injection
- Route guard component naming and structure
- Loading skeleton or transition animations

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Backend auth API
- `internal/server/handlers_auth.go` — Login handler (`POST /auth/login`), change password (`PUT /users/me/password`), login-disabled handler
- `internal/api/types.go` — `LoginRequest`, `LoginResponse` structs (request/response contracts)
- `internal/server/server.go` — Route registration, JWT middleware groups, public vs protected endpoints

### Frontend scaffold
- `web/src/App.tsx` — Current routing setup with BrowserRouter, QueryClient config (already skips retry on 401/403)
- `web/src/components/layout/AppShell.tsx` — Shell layout with Sidebar + Outlet
- `web/src/components/layout/Sidebar.tsx` — Navigation sidebar (user menu goes at bottom)
- `web/components.json` — shadcn/ui config (style=default, base=radix, Tailwind v4)

### UI design contract
- `.planning/phases/26-authentication/26-UI-SPEC.md` — Login page wireframe, password change dialog, typography/spacing specs, component list

### Phase research
- `.planning/phases/26-authentication/26-RESEARCH.md` — Technical research on auth implementation patterns

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- shadcn/ui components already installed: `Button`, `Separator`, `Tooltip`
- `Sonner` toast already configured in App.tsx — use for success notifications
- `@tanstack/react-query` already set up with 401/403 retry skip
- `react-router` BrowserRouter already in place

### Established Patterns
- Layout: AppShell wraps all routes via `<Outlet />` — login page must be OUTSIDE this wrapper
- Styling: Tailwind v4 + shadcn/ui CSS variables — no custom theme overrides
- State: React Query for server state — auth state likely needs a React context or similar

### Integration Points
- App.tsx Routes: login route outside AppShell, protected routes inside AppShell
- Sidebar.tsx: add user menu section at bottom
- QueryClient: already configured to not retry on 401 — hook into this for session expiry detection
- Vite proxy: `web/vite.config.ts` proxies `/api` to backend in dev mode

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 26-authentication*
*Context gathered: 2026-04-06*
