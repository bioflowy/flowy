# Phase 19: User Authentication - Context

**Gathered:** 2026-03-17
**Status:** Ready for planning

<domain>
## Phase Boundary

Add user authentication to the flowy-server REST API. Users can log in with username/password and receive JWT tokens. All non-public routes are protected by JWT middleware. Admin is created via CLI flag on first start. Daemon authenticates with pre-shared token. Auth can be disabled entirely for backward compatibility.

</domain>

<decisions>
## Implementation Decisions

### Admin seed strategy
- Use `--init-admin` CLI flag on `flowy-server`, not migration-seeded defaults
- Both modes supported: if `--username` and `--password` args provided, use them; otherwise prompt interactively with hidden password input
- If users already exist in DB, error and exit: "Users already exist. Use the API to manage users."
- `--init-admin` works independently of `auth.enabled` setting (can pre-configure admin before enabling auth)

### Daemon authentication
- Pre-shared token in config: `auth.daemon_token` in YAML
- Same token value in both server and daemon config (single shared token, not per-daemon)
- Required when `auth.enabled=true` — daemon cannot claim tasks without matching token
- When `auth.enabled=false`, all auth is skipped (user routes AND daemon routes) — current no-auth behavior preserved
- Daemon sends token as `Authorization: Bearer <daemon_token>` header
- Server validates daemon routes with separate `DaemonAuthMiddleware` that checks against config token

### Token lifetime & errors
- JWT tokens valid for 7 days (no refresh tokens in v1.3)
- HS256 signing algorithm
- JWT signing secret: if `auth.jwt_secret` in config, use it; otherwise auto-generate on first run and persist in DB
- Error responses: 401 Unauthorized with reason in JSON body (`{"error": "invalid credentials"}`, `{"error": "token expired"}`, `{"error": "missing authorization header"}`)
- 403 Forbidden for authenticated but unauthorized requests (valid token but wrong role/permission)

### User fields
- `id` (UUID), `username` (unique), `password_hash` (bcrypt), `display_name`, `email`, `global_role` (admin/user)
- `is_active` (boolean, default true) for soft delete
- `created_at`, `updated_at` timestamps

### User management
- User creation: admin-only via `POST /users`
- User listing: admin-only via `GET /users`
- Self-profile: `GET /users/me` returns own profile (username, display_name, email, global_role)
- Password change: self-service via `PUT /users/me/password` (requires current password in request body)
- User deletion: soft delete (mark `is_active=false`), disabled users cannot log in
- Admin can list and soft-delete users via `DELETE /users/{id}`

### Route structure
- Three route groups in chi: public (login, health), user-authenticated (JWT middleware), daemon-authenticated (token middleware)
- Public: `POST /auth/login`, `GET /health`
- User routes: all existing workflow endpoints + user management + future project/registry/provenance routes
- Daemon routes: `POST /tasks/claim`, `POST /tasks/{id}/result`, `POST /heartbeat`, `POST /tasks/run`

### Claude's Discretion
- Exact bcrypt cost factor
- JWT claims structure (what fields to include beyond user_id and role)
- Auto-generated secret storage mechanism (DB table or file)
- Exact error message wording

</decisions>

<specifics>
## Specific Ideas

- Future Web UI support: design API responses to be Web UI-friendly (e.g., GET /users/me for profile display)
- Docker entrypoint pattern: `--init-admin` with args mode allows `docker run flowy-server --init-admin --username=admin --password=secret` for automated deployment

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `chi.Mux` with `middleware.Logger` and `middleware.Recoverer` already in `internal/server/server.go:newRouter()`
- `ErrorResponse` struct for JSON error responses already established
- `Store` interface pattern in `internal/store/store.go` — add new method groups for user operations

### Established Patterns
- chi route registration in `newRouter()` — new route groups added here
- goose migrations in `internal/store/sqlite/migrations/` — next is `00004`
- Config loading via `internal/config/config.go` — add `AuthConfig` sub-struct
- Daemon HTTP client in `internal/daemon/daemon.go` — needs header injection for Bearer token

### Integration Points
- `server.go:New()` — receives new `AuthConfig` for JWT secret and daemon token
- `server.go:newRouter()` — restructure into 3 route groups (public/user/daemon)
- `daemon.go:New()` — inject daemon token into HTTP client headers
- `config.go:Config` — add `Auth AuthConfig` field
- `store.go:Store` — add `CreateUser`, `GetUserByUsername`, `ListUsers`, `UpdateUser`, `DeactivateUser` methods

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 19-user-authentication*
*Context gathered: 2026-03-17*
