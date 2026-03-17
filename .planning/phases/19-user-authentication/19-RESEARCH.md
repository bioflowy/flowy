# Phase 19: User Authentication - Research

**Researched:** 2026-03-17
**Domain:** JWT authentication, bcrypt password hashing, chi middleware, SQLite schema, CLI flag parsing
**Confidence:** HIGH

## Summary

Phase 19 adds user authentication to the flowy-server REST API. The work is well-scoped: the codebase already has chi middleware infrastructure, a goose migration system, a Store interface pattern, and `golang.org/x/crypto` (which provides bcrypt) already in go.mod as an indirect dependency. The main new dependency is `github.com/golang-jwt/jwt/v5`, which needs to be added via `go get`.

The implementation has four distinct concerns: (1) schema + store layer (migration 00004, new User model, new Store methods), (2) auth package (JWT issue/validate, bcrypt helpers, middleware functions), (3) server restructure (three chi route groups instead of flat registration), and (4) the `--init-admin` CLI command and daemon token injection. The backward-compat requirement (auth.enabled defaults false, all 156 spectool tests pass) means the auth gate must be a guarded if-branch in `newRouter()`, not an unconditional middleware.

Auto-generated JWT secret storage is decided as Claude's discretion. The simplest and most portable approach for a SQLite-only deployment is a single-row `server_config` key-value table in migration 00004; this avoids filesystem path concerns and is accessible to the same `Store` interface.

**Primary recommendation:** Add `github.com/golang-jwt/jwt/v5` via `go get`, promote `golang.org/x/crypto` to a direct dependency in go.mod, create `internal/auth/` for JWT/bcrypt logic, restructure `newRouter()` with conditional auth groups, and add migration 00004 for the users + server_config tables.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- Admin seed: `--init-admin` CLI flag on `flowy-server`; supports `--username`/`--password` args OR interactive hidden-password prompt; errors if users already exist
- Daemon authentication: pre-shared token in `auth.daemon_token` YAML; `Authorization: Bearer <token>` header; separate `DaemonAuthMiddleware`; required when auth.enabled=true
- JWT: HS256, 7-day lifetime, no refresh tokens; secret from `auth.jwt_secret` config or auto-generated and persisted in DB
- Error responses: 401 with `{"error": "..."}` body; 403 for valid-token/wrong-role
- User fields: id (UUID), username (unique), password_hash (bcrypt), display_name, email, global_role (admin/user), is_active (bool, default true), created_at, updated_at
- User management endpoints:
  - `POST /users` — admin only, create user
  - `GET /users` — admin only, list users
  - `GET /users/me` — self, own profile
  - `PUT /users/me/password` — self, change password (requires current password)
  - `DELETE /users/{id}` — admin only, soft delete (sets is_active=false)
- Route groups: public (`POST /auth/login`, `GET /health`), user-authenticated (existing workflow endpoints + user mgmt), daemon-authenticated (`POST /tasks/claim`, `POST /tasks/{id}/result`, `POST /heartbeat`, `POST /tasks/run`)
- `auth.enabled` defaults false; when false all auth is skipped and current spectool tests pass unchanged

### Claude's Discretion

- Exact bcrypt cost factor
- JWT claims structure (fields beyond user_id and role)
- Auto-generated secret storage mechanism (DB table or file)
- Exact error message wording

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| AUTH-01 | User can log in with username and password, receiving a JWT token | `POST /auth/login` handler + bcrypt.CompareHashAndPassword + jwt.NewWithClaims |
| AUTH-02 | All API routes (except login and health) are protected by JWT middleware | chi route group with JWTMiddleware applied via `r.Use()` |
| AUTH-03 | Server seeds an initial admin user on first run | `--init-admin` flag in main.go calls `store.CreateUser` after checking count=0 |
| AUTH-04 | Admin can create new users with username and password | `POST /users` handler behind JWT middleware + admin role check |
| AUTH-05 | Admin can list and delete users | `GET /users` + `DELETE /users/{id}` handlers behind JWT middleware + admin role check |
| AUTH-06 | Auth can be disabled via config flag (`auth.enabled: false`) for backward compatibility | `cfg.Auth.Enabled` gate in `newRouter()` — when false, flat registration with no middleware (current behavior) |
</phase_requirements>

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/golang-jwt/jwt/v5` | v5.x (latest) | JWT signing and validation | Zero external dependencies; HS256 support; standard claims with ExpiresAt; chosen in STATE.md decisions |
| `golang.org/x/crypto` | v0.48.0 (already in go.mod) | bcrypt password hashing | Standard Go crypto package; `bcrypt.GenerateFromPassword` / `bcrypt.CompareHashAndPassword` |
| `github.com/go-chi/chi/v5` | v5.2.5 (already in go.mod) | Route grouping + middleware | Already in use; `r.Group()` and `r.Use()` for auth route groups |
| `github.com/google/uuid` | v1.6.0 (already in go.mod) | UUID generation for user IDs | Already in use for run IDs |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `golang.org/x/term` | part of x/sys (already in go.sum) | Hidden password input for `--init-admin` prompt | Only when `--init-admin` used without `--password` flag |

### New Dependency Required
```bash
cd /home/uehara/flowy && go get github.com/golang-jwt/jwt/v5
```

Also promote `golang.org/x/crypto` from indirect to direct in go.mod (it is already at v0.48.0, just change the comment).

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| golang-jwt/jwt/v5 | lestrrat-go/jwx | jwx is full JOSE suite; jwt/v5 is simpler and sufficient for HS256 |
| Hand-rolled RBAC (~50 lines) | casbin | Casbin adds significant dependency weight; three static roles don't justify it (STATE.md decision) |

---

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── auth/
│   ├── auth.go          # JWT issue/validate, bcrypt helpers, secret management
│   └── auth_test.go
├── server/
│   ├── server.go        # Updated: newRouter() with 3 route groups
│   ├── handlers.go      # Existing handlers unchanged
│   ├── handlers_auth.go # New: handleLogin, handleCreateUser, handleListUsers,
│   │                    #      handleDeleteUser, handleGetMe, handleChangePassword
│   └── middleware.go    # New: JWTMiddleware, DaemonAuthMiddleware, roleRequired()
├── store/
│   ├── store.go         # Updated: add User model + user Store methods
│   └── sqlite/
│       ├── users.go     # New: CreateUser, GetUserByUsername, etc.
│       └── migrations/
│           └── 00004_users.sql  # New: users table + server_config table
└── config/
    └── config.go        # Updated: add AuthConfig sub-struct
```

### Pattern 1: Three Chi Route Groups
**What:** Replace flat route registration with conditional auth groups based on `cfg.Auth.Enabled`
**When to use:** Always — this is the only safe way to satisfy AUTH-06 (backward compat)

```go
// Source: chi docs — r.Group() creates an isolated sub-router with its own middleware stack
func (s *Server) newRouter() *chi.Mux {
    r := chi.NewRouter()
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)

    // Public routes — never require auth
    r.Get("/health", s.handleHealth)
    r.Post("/auth/login", s.handleLogin)

    if s.cfg.Auth.Enabled {
        // User-authenticated routes
        r.Group(func(r chi.Router) {
            r.Use(auth.JWTMiddleware(s.jwtSecret))
            r.Post("/workflows", s.handleSubmit)
            r.Get("/workflows/{id}/status", s.handleStatus)
            // ... remaining workflow routes
            r.Post("/users", s.handleCreateUser)
            r.Get("/users", s.handleListUsers)
            r.Get("/users/me", s.handleGetMe)
            r.Put("/users/me/password", s.handleChangePassword)
            r.Delete("/users/{id}", s.handleDeleteUser)
        })

        // Daemon-authenticated routes
        r.Group(func(r chi.Router) {
            r.Use(auth.DaemonAuthMiddleware(s.cfg.Auth.DaemonToken))
            r.Post("/tasks/run", s.handleTaskRun)
            r.Post("/tasks/claim", s.handleClaim)
            r.Post("/tasks/{id}/result", s.handleResult)
            r.Post("/heartbeat", s.handleHeartbeat)
        })
    } else {
        // Auth disabled: flat registration, no middleware (current behavior)
        r.Post("/workflows", s.handleSubmit)
        // ... all routes flat
    }
    return r
}
```

### Pattern 2: JWT Middleware with Context Injection
**What:** Middleware extracts and validates JWT, injects claims into request context
**When to use:** All user-authenticated routes

```go
// Source: golang-jwt/jwt/v5 docs
type contextKey string
const userClaimsKey contextKey = "user_claims"

type Claims struct {
    UserID string `json:"sub"`
    Role   string `json:"role"`
    jwt.RegisteredClaims
}

func JWTMiddleware(secret []byte) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            authHeader := r.Header.Get("Authorization")
            if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
                writeError(w, http.StatusUnauthorized, "missing authorization header")
                return
            }
            tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
            token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
                if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
                    return nil, fmt.Errorf("unexpected signing method")
                }
                return secret, nil
            })
            if err != nil || !token.Valid {
                writeError(w, http.StatusUnauthorized, "invalid or expired token")
                return
            }
            claims := token.Claims.(*Claims)
            ctx := context.WithValue(r.Context(), userClaimsKey, claims)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

### Pattern 3: Role Guard Helper
**What:** Helper function for admin-only handlers, avoids repeating 403 logic
**When to use:** Any handler that requires admin role

```go
func roleRequired(role string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            claims, ok := r.Context().Value(userClaimsKey).(*Claims)
            if !ok || claims.Role != role {
                writeError(w, http.StatusForbidden, "admin role required")
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
// Usage: r.With(roleRequired("admin")).Post("/users", s.handleCreateUser)
```

### Pattern 4: Auto-Generated JWT Secret Persistence in DB
**What:** On first server start (no `auth.jwt_secret` in config), generate a random 32-byte secret and store in a `server_config` key-value table
**When to use:** When `cfg.Auth.JWTSecret` is empty
**Storage table** (part of migration 00004):
```sql
CREATE TABLE IF NOT EXISTS server_config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
```
**Lookup pattern:**
```go
// In server.New(): resolve the effective JWT secret
func resolveJWTSecret(cfg *config.Config, st store.Store) ([]byte, error) {
    if cfg.Auth.JWTSecret != "" {
        return []byte(cfg.Auth.JWTSecret), nil
    }
    val, err := st.GetServerConfig(ctx, "jwt_secret")
    if err == nil {
        return []byte(val), nil
    }
    // Generate and persist
    secret := make([]byte, 32)
    rand.Read(secret)
    encoded := hex.EncodeToString(secret)
    st.SetServerConfig(ctx, "jwt_secret", encoded)
    return []byte(encoded), nil
}
```

### Pattern 5: Daemon Token Injection
**What:** Daemon reads `cfg.Auth.DaemonToken` and sets it on all HTTP requests via a custom `http.RoundTripper`
**When to use:** When `cfg.Auth.Enabled` is true (token may be empty string = no-op)

```go
// In daemon.go: wrap the default transport
type bearerTransport struct {
    token string
    base  http.RoundTripper
}
func (t *bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    if t.token != "" {
        req = req.Clone(req.Context())
        req.Header.Set("Authorization", "Bearer "+t.token)
    }
    return t.base.RoundTrip(req)
}
// In daemon.New():
client: &http.Client{
    Timeout:   35 * time.Second,
    Transport: &bearerTransport{token: cfg.Auth.DaemonToken, base: http.DefaultTransport},
},
```

### Anti-Patterns to Avoid
- **Global middleware for all routes:** Applying JWT middleware at the top-level `r.Use()` call would break auth-disabled mode and the public `/health` and `/auth/login` routes. Use `r.Group()` scoping.
- **Token refresh on the daemon via HTTP:** The daemon should not need to refresh tokens; pre-shared static token is the correct pattern for daemon-to-server auth.
- **Storing bcrypt hash in JWT claims:** Claims should contain user_id and role only; never include password-related data.
- **Checking `is_active` only at login:** `is_active` must also be checked by JWT middleware (or preferably: just let the token expire naturally, but on login reject inactive users). Given 7-day tokens, inactive users remain authenticated for up to 7 days if only login is checked. **Decision:** check `is_active` at login time only (acceptable for v1.3 scope; documented limitation).

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JWT signing/validation | Custom HMAC token format | `golang-jwt/jwt/v5` | Correct ExpiresAt handling, leeway, parsing edge cases |
| Password hashing | SHA-256 or MD5 of password | `bcrypt.GenerateFromPassword` | bcrypt is adaptive; MD5/SHA are not suitable for passwords |
| Hidden password terminal input | ANSI escape sequences | `golang.org/x/term.ReadPassword` | Cross-platform; handles terminal state restoration on error |
| UUID generation for user IDs | Random number as string | `github.com/google/uuid` (already used) | Correct UUID v4 format; already in use for run IDs |

**Key insight:** The most dangerous custom implementation is token validation — subtle bugs in expiry checking or signature verification create security vulnerabilities. Use the library.

---

## Common Pitfalls

### Pitfall 1: Flat Route Registration Breaks Spectool Tests
**What goes wrong:** If JWT middleware is applied globally or to all existing routes unconditionally, the 156 spectool conformance tests fail because they don't send Authorization headers.
**Why it happens:** Easy to apply middleware at the `r.Use()` level as a single change.
**How to avoid:** The `cfg.Auth.Enabled` gate in `newRouter()` must be unconditional — when false, use the current flat registration with no auth middleware.
**Warning signs:** Any spectool test failure after adding middleware is a sign this went wrong.

### Pitfall 2: JWT Secret Lost on Server Restart
**What goes wrong:** If the auto-generated secret is stored in memory only (not persisted), all existing JWT tokens become invalid on every server restart.
**Why it happens:** Forgetting to persist the generated secret.
**How to avoid:** Use the `server_config` DB table to persist and reload the secret. Always check DB first before generating.
**Warning signs:** Users get 401 errors immediately after server restart.

### Pitfall 3: Daemon Routes Without Auth When auth.enabled=true
**What goes wrong:** Daemon cannot claim tasks because its routes are accidentally put inside the JWT middleware group.
**Why it happens:** Lumping all non-public routes into a single auth group.
**How to avoid:** Three distinct groups: public, user-JWT, daemon-token. Daemon routes use `DaemonAuthMiddleware`, not `JWTMiddleware`.
**Warning signs:** Daemon logs 401 errors on `/tasks/claim`.

### Pitfall 4: Chi Route Conflicts — Login and Users Overlap
**What goes wrong:** `/auth/login` is public but `/users` routes are protected. If the chi tree is not structured correctly, the public login group's sub-paths could conflict with the protected users group.
**Why it happens:** Chi sub-router scoping confusion.
**How to avoid:** Register all public routes directly on `r`, then use `r.Group()` for protected groups. Do not mount sub-routers on overlapping prefixes.

### Pitfall 5: `--init-admin` in `main.go` vs. Ongoing Server Operation
**What goes wrong:** `--init-admin` mode must open the DB, check for existing users, create the admin, then exit — it must NOT start the HTTP server.
**Why it happens:** Putting init logic inside `server.New()` rather than as a pre-main branch.
**How to avoid:** In `main.go`, check `initAdmin` flag before constructing the `server.Server`. If set, run the init flow and `os.Exit(0)`.

### Pitfall 6: bcrypt Cost Too Low in Tests / Too High in Production
**What goes wrong:** Cost 12 takes ~300ms per hash. With many concurrent tests this slows the test suite significantly.
**Why it happens:** Using production cost in tests.
**How to avoid:** Accept cost as a parameter in the bcrypt helper; use `bcrypt.MinCost` (cost 4) in tests, cost 12 in production. The STATE.md decision says cost 12; make it configurable internally.

---

## Code Examples

### Issue a JWT Token (golang-jwt/jwt/v5)
```go
// Source: https://github.com/golang-jwt/jwt/blob/v5/README.md
import (
    "time"
    "github.com/golang-jwt/jwt/v5"
)

func IssueToken(secret []byte, userID, role string) (string, error) {
    claims := jwt.MapClaims{
        "sub":  userID,
        "role": role,
        "exp":  time.Now().Add(7 * 24 * time.Hour).Unix(),
        "iat":  time.Now().Unix(),
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(secret)
}
```

### Hash a Password (bcrypt)
```go
// Source: golang.org/x/crypto/bcrypt package docs
import "golang.org/x/crypto/bcrypt"

const BcryptCost = 12 // ~300ms on modern hardware

func HashPassword(password string) (string, error) {
    hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
    return string(hash), err
}

func CheckPassword(hash, password string) error {
    return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
```

### Read Password from Terminal (hidden input)
```go
// Source: golang.org/x/term docs
import (
    "fmt"
    "golang.org/x/term"
    "os"
)

func promptPassword(prompt string) (string, error) {
    fmt.Print(prompt)
    pw, err := term.ReadPassword(int(os.Stdin.Fd()))
    fmt.Println() // newline after hidden input
    return string(pw), err
}
```

### Migration 00004 (users + server_config)
```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    id            TEXT     PRIMARY KEY,
    username      TEXT     NOT NULL UNIQUE,
    password_hash TEXT     NOT NULL,
    display_name  TEXT     NOT NULL DEFAULT '',
    email         TEXT     NOT NULL DEFAULT '',
    global_role   TEXT     NOT NULL DEFAULT 'user',
    is_active     INTEGER  NOT NULL DEFAULT 1,
    created_at    DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at    DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);

CREATE TABLE IF NOT EXISTS server_config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS server_config;
DROP TABLE IF EXISTS users;
```

### AuthConfig Sub-Struct
```go
// Add to internal/config/config.go
type AuthConfig struct {
    // Enabled gates all JWT and daemon-token middleware.
    // Defaults to false for backward compatibility.
    Enabled bool `yaml:"enabled"`

    // JWTSecret is the HMAC-SHA256 signing secret for JWT tokens.
    // If empty, the server auto-generates one and persists it in the DB.
    JWTSecret string `yaml:"jwt_secret"`

    // DaemonToken is the pre-shared token daemons use to authenticate.
    // Required when Enabled is true.
    DaemonToken string `yaml:"daemon_token"`
}

// In Config struct:
Auth AuthConfig `yaml:"auth"`
```

### chi Route Group Pattern
```go
// Source: https://github.com/go-chi/chi/tree/v5
// chi.Router.Group creates an inline sub-router with its own middleware stack
r.Group(func(r chi.Router) {
    r.Use(someMiddleware)
    r.Get("/protected", handler)
})
// routes registered on the outer r are unaffected
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| jwt-go (v3) | golang-jwt/jwt/v5 | 2022 (jwt-go archived) | jwt/v5 is the maintained fork; v5 API changed `Valid()` to `token.Valid` field |
| bcrypt custom | `golang.org/x/crypto/bcrypt` | Standard since Go 1.x | No change needed |
| `golang.org/x/term.ReadPassword` | same | Current | Replaces deprecated `golang.org/x/crypto/ssh/terminal` |

**Deprecated/outdated:**
- `github.com/dgrijalva/jwt-go`: archived 2021; use `github.com/golang-jwt/jwt/v5` instead
- `golang.org/x/crypto/ssh/terminal.ReadPassword`: deprecated; use `golang.org/x/term.ReadPassword`

---

## Open Questions

1. **`golang.org/x/term` availability**
   - What we know: `golang.org/x/sys` is already in go.sum (pulled in by docker SDK); `golang.org/x/term` depends on `x/sys`
   - What's unclear: whether `x/term` is already transitively available or needs explicit `go get`
   - Recommendation: Run `go get golang.org/x/term` as part of Wave 0 setup; it is a tiny package and safe to add

2. **`is_active` token revocation**
   - What we know: with 7-day tokens and no token revocation mechanism, a deactivated user's token remains valid for up to 7 days
   - What's unclear: whether this is acceptable for v1.3
   - Recommendation: Document as a known limitation in the code comment; check `is_active` at login only (per scope); full revocation requires a token denylist which is out of scope for v1.3

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + net/http/httptest (stdlib) |
| Config file | none — Go test toolchain uses `go test ./...` |
| Quick run command | `go test ./internal/auth/... ./internal/server/... -run TestAuth -timeout 30s` |
| Full suite command | `go test ./... -timeout 120s` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| AUTH-01 | POST /auth/login returns JWT for valid credentials | unit | `go test ./internal/server/... -run TestHandleLogin -timeout 30s` | Wave 0 |
| AUTH-01 | POST /auth/login returns 401 for invalid credentials | unit | `go test ./internal/server/... -run TestHandleLogin -timeout 30s` | Wave 0 |
| AUTH-02 | Protected routes return 401 without token | unit | `go test ./internal/server/... -run TestJWTMiddleware -timeout 30s` | Wave 0 |
| AUTH-02 | Protected routes return 200 with valid token | unit | `go test ./internal/server/... -run TestJWTMiddleware -timeout 30s` | Wave 0 |
| AUTH-03 | --init-admin creates admin user in DB | unit | `go test ./cmd/flowy-server/... -run TestInitAdmin -timeout 30s` | Wave 0 |
| AUTH-03 | --init-admin fails if users already exist | unit | `go test ./cmd/flowy-server/... -run TestInitAdmin -timeout 30s` | Wave 0 |
| AUTH-04 | POST /users creates user (admin JWT) | unit | `go test ./internal/server/... -run TestHandleCreateUser -timeout 30s` | Wave 0 |
| AUTH-04 | POST /users returns 403 for non-admin JWT | unit | `go test ./internal/server/... -run TestHandleCreateUser -timeout 30s` | Wave 0 |
| AUTH-05 | GET /users returns user list (admin JWT) | unit | `go test ./internal/server/... -run TestHandleListUsers -timeout 30s` | Wave 0 |
| AUTH-05 | DELETE /users/{id} soft-deletes user (admin JWT) | unit | `go test ./internal/server/... -run TestHandleDeleteUser -timeout 30s` | Wave 0 |
| AUTH-06 | All existing server tests pass with auth.enabled=false | regression | `go test ./internal/server/... -timeout 60s` | Existing (`server_test.go`) |
| AUTH-06 | Daemon routes work without token when auth.enabled=false | regression | `go test ./internal/server/... -run TestClaim -timeout 30s` | Existing |

### Sampling Rate
- **Per task commit:** `go test ./internal/auth/... ./internal/server/... -timeout 60s`
- **Per wave merge:** `go test ./... -timeout 120s`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/auth/auth_test.go` — covers JWT issue/validate and bcrypt helpers
- [ ] `internal/server/handlers_auth_test.go` — covers AUTH-01 through AUTH-05 handler tests
- [ ] `cmd/flowy-server/initadmin_test.go` — covers AUTH-03 `--init-admin` logic (if testable as a function)
- [ ] Framework install: `go get github.com/golang-jwt/jwt/v5` — jwt library not yet in go.mod

---

## Sources

### Primary (HIGH confidence)
- Codebase inspection: `internal/server/server.go`, `internal/store/store.go`, `internal/config/config.go`, `cmd/flowy-server/main.go`, `internal/server/handlers.go` — established patterns verified directly
- `go.mod` inspection — confirmed existing dependencies (chi v5.2.5, google/uuid v1.6.0, golang.org/x/crypto v0.48.0, goose v3.27.0, modernc/sqlite)
- `.planning/phases/19-user-authentication/19-CONTEXT.md` — all locked decisions

### Secondary (MEDIUM confidence)
- golang-jwt/jwt v5 README (https://github.com/golang-jwt/jwt) — API patterns for ParseWithClaims, MapClaims, HS256
- chi v5 documentation (https://github.com/go-chi/chi) — Group() and Use() patterns
- golang.org/x/crypto/bcrypt package — GenerateFromPassword/CompareHashAndPassword API

### Tertiary (LOW confidence)
- None

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries either already in go.mod or are well-established packages with stable APIs
- Architecture: HIGH — patterns derived directly from existing codebase; chi Group() pattern is standard
- Pitfalls: HIGH — derived from project constraints (backward compat, daemon/user separation) and common JWT implementation errors

**Research date:** 2026-03-17
**Valid until:** 2026-09-17 (golang-jwt/jwt/v5 and bcrypt APIs are stable; chi v5 is stable)
