# Phase 33: CLI Auth and Registry Commands - Research

**Researched:** 2026-04-07
**Domain:** Go CLI (stdlib flag), credential file I/O, HTTP client with JWT Bearer auth, text/tabwriter table output
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- Use `fmt.Print` for username prompt and `golang.org/x/term` `ReadPassword` for hidden password input
- Credential file: YAML format with `server` and `token` fields at `~/.flowy/credentials.yaml` (mode 0600), using `gopkg.in/yaml.v3` consistent with existing config pattern
- Single server support only — credentials file stores one server+token pair (multi-server deferred)
- No automatic token refresh — if token expired, print "Token expired. Run `flowy login` to authenticate." and exit code 1
- Single-level nesting under `registry`: `flowy registry push/list/pull/tag/delete-tag/delete`
- `list` output: human-readable table with columns (Name, Kind, Latest Tag, Versions) via `text/tabwriter`
- `pull` prints WDL source to stdout (pipeable with `> file.wdl`)
- `push` prints one line on success: `Pushed {name} digest:{hash}`
- Missing credentials file: "Not authenticated. Run `flowy login --server <url>` first." (exit 1)
- Server connection failure: "Cannot connect to server at {url}: {error}" (exit 1)
- Permission denied (403): "Permission denied: {role} role required for this operation" (exit 1)
- Expired/invalid token (401): "Token expired or invalid. Run `flowy login` to authenticate." (exit 1) — NOT raw JSON 401
- `delete` has no confirmation prompt — admin-only, keep CLI scriptable per Unix conventions

### Claude's Discretion
No specific requirements — open to standard approaches following existing codebase patterns.

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CLI-01 | User can log in via `flowy login` and JWT token is saved to credentials file (~/.flowy/credentials.yaml) | `POST /auth/login` exists, returns `{"token":"..."}`. Need new `runLogin` function + `internal/client/credentials.go`. `golang.org/x/term` already in go.mod. |
| CLI-02 | All authenticated CLI commands automatically load saved token from credentials file | `loadCredentials()` reads `~/.flowy/credentials.yaml`, passes token as `Authorization: Bearer <token>`. Client needs `WithToken(token)` support or a `NewWithToken` constructor. |
| REG-CLI-01 | User can push a WDL file to registry via `flowy registry push --project <slug> --name <name> <file.wdl>` | `POST /projects/{slug}/registry` exists, returns `RegistryPushResponse`. Need `client.RegistryPush()` method + `runRegistryPush` subcommand. |
| REG-CLI-02 | User can list registry entries via `flowy registry list --project <slug>` | `GET /projects/{slug}/registry` exists, returns `[]RegistryEntryResponse`. Need `client.RegistryList()` + table rendering via `text/tabwriter`. |
| REG-CLI-03 | User can pull WDL source via `flowy registry pull --project <slug> --name <name> [--tag <tag>]` | `GET /projects/{slug}/registry/{name}?tag=<tag>` exists, returns `RegistryPullResponse.Content`. Pipe to stdout. |
| REG-CLI-04 | User can manage tags via `flowy registry tag` and `flowy registry delete-tag` | `POST /projects/{slug}/registry/{name}/tags` and `DELETE /projects/{slug}/registry/{name}/tags/{tag}` both exist. |
| REG-CLI-05 | Admin can delete a registry entry via `flowy registry delete --project <slug> --name <name>` | `DELETE /projects/{slug}/registry/{name}` exists, returns 204. |
</phase_requirements>

## Summary

Phase 33 adds the `flowy login` command and the full `flowy registry` subcommand tree to the existing CLI binary. All server-side endpoints (auth login, registry push/list/pull/tag/delete-tag/delete) already exist and are JWT-protected. The work is entirely client-side: credential storage, token loading, HTTP client enhancement, and CLI dispatch wiring.

The existing codebase provides strong patterns to follow: `func runXxxCommand(args []string, stdout, stderr io.Writer) int` returning exit codes, `flag.NewFlagSet` per subcommand, JSON marshalling in `internal/client/client.go`, and YAML config via `gopkg.in/yaml.v3`. The only new Go stdlib packages needed are `text/tabwriter` (for list output) and `os` file permission operations (for mode 0600). Both `golang.org/x/term` and `gopkg.in/yaml.v3` are already in `go.mod`.

The key design challenge is the credential injection path: the `Client` struct currently has no auth support. The cleanest addition is a `NewWithToken(serverURL, token string) *Client` constructor that sets an `Authorization: Bearer` header on every request, consistent with how the server's `JWTMiddleware` expects it.

**Primary recommendation:** Add `internal/client/credentials.go` for credential file I/O, add `NewWithToken` or a `WithToken` method to the Client, add `runLogin` and `runRegistry` to `cmd/flowy/main.go`, wire via `case "login":` and `case "registry":` in the switch.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `flag` (stdlib) | Go 1.25 | FlagSet per subcommand | Matches all existing subcommands exactly |
| `text/tabwriter` (stdlib) | Go 1.25 | Aligned table output for `registry list` | Zero-dependency, used by `go list` itself |
| `fmt`, `os`, `io` (stdlib) | Go 1.25 | I/O, file ops | Already used throughout |
| `gopkg.in/yaml.v3` | v3.0.1 | Credential YAML serialization | Already in go.mod; used by `internal/config` |
| `golang.org/x/term` | v0.41.0 | Hidden password prompt (`ReadPassword`) | Already in go.mod (indirect); locked decision |
| `net/http` (stdlib) | Go 1.25 | HTTP requests with Bearer token header | Already used in `internal/client/client.go` |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `os.UserHomeDir()` | stdlib | Resolve `~/.flowy/credentials.yaml` | Always — portable across Linux/macOS |
| `os.MkdirAll` + `os.WriteFile` with `0600` | stdlib | Create `~/.flowy/` dir and write credentials securely | In `saveCredentials()` |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `text/tabwriter` | `github.com/olekukonko/tablewriter` | External dep unnecessary; tabwriter is simpler and already in stdlib |
| New `NewWithToken` constructor | Modify `Client` to hold token field | Either works; constructor approach avoids zero-value confusion |

**No new dependencies required.** All needed packages are stdlib or already in go.mod.

## Architecture Patterns

### Recommended Project Structure
```
cmd/flowy/
├── main.go              # add cases "login" and "registry" to switch
├── login.go             # runLogin(args, stdout, stderr) int
├── registry.go          # runRegistry(args, stdout, stderr) int + sub-dispatch
internal/client/
├── client.go            # add NewWithToken; add registry methods
├── credentials.go       # NEW: Credentials struct, loadCredentials, saveCredentials
```

### Pattern 1: Subcommand Function Signature
**What:** Every subcommand is `func runXxx(args []string, stdout, stderr io.Writer) int`.
**When to use:** All new commands (login, registry) and registry sub-dispatch.
**Example:**
```go
// Source: cmd/flowy/main.go (existing pattern)
func runLogin(args []string, stdout, stderr io.Writer) int {
    fs := flag.NewFlagSet("login", flag.ContinueOnError)
    fs.SetOutput(stderr)
    serverFlag := fs.String("server", "", "server URL")
    if err := fs.Parse(args); err != nil {
        return 2
    }
    // ... implementation
    return 0
}
```

### Pattern 2: Registry Sub-dispatch
**What:** `runRegistry` dispatches to sub-subcommands by inspecting `args[0]`.
**When to use:** For the `registry` command which has 6 sub-subcommands.
**Example:**
```go
func runRegistry(args []string, stdout, stderr io.Writer) int {
    if len(args) == 0 {
        fmt.Fprintln(stderr, "Usage: flowy registry <subcommand>")
        return 2
    }
    switch args[0] {
    case "push":
        return runRegistryPush(args[1:], stdout, stderr)
    case "list":
        return runRegistryList(args[1:], stdout, stderr)
    case "pull":
        return runRegistryPull(args[1:], stdout, stderr)
    case "tag":
        return runRegistryTag(args[1:], stdout, stderr)
    case "delete-tag":
        return runRegistryDeleteTag(args[1:], stdout, stderr)
    case "delete":
        return runRegistryDelete(args[1:], stdout, stderr)
    default:
        fmt.Fprintf(stderr, "unknown registry subcommand: %q\n", args[0])
        return 2
    }
}
```

### Pattern 3: Credentials File
**What:** YAML file at `~/.flowy/credentials.yaml` (mode 0600) with `server` and `token` fields.
**When to use:** `saveCredentials` (in login), `loadCredentials` (in all registry commands).
**Example:**
```go
// internal/client/credentials.go
type Credentials struct {
    Server string `yaml:"server"`
    Token  string `yaml:"token"`
}

func loadCredentials() (*Credentials, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return nil, err
    }
    path := filepath.Join(home, ".flowy", "credentials.yaml")
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err // os.IsNotExist(err) => no credentials
    }
    var creds Credentials
    if err := yaml.Unmarshal(data, &creds); err != nil {
        return nil, err
    }
    return &creds, nil
}

func saveCredentials(creds *Credentials) error {
    home, err := os.UserHomeDir()
    if err != nil {
        return err
    }
    dir := filepath.Join(home, ".flowy")
    if err := os.MkdirAll(dir, 0700); err != nil {
        return err
    }
    data, err := yaml.Marshal(creds)
    if err != nil {
        return err
    }
    path := filepath.Join(dir, "credentials.yaml")
    return os.WriteFile(path, data, 0600)
}
```

### Pattern 4: Client with Token
**What:** Constructor that sets Bearer token on every request.
**When to use:** All registry commands after loading credentials.
**Example:**
```go
// internal/client/client.go addition
func NewWithToken(serverURL, token string) *Client {
    return &Client{
        serverURL:  serverURL,
        httpClient: &http.Client{Timeout: 30 * time.Second},
        token:      token,
    }
}

// In doRequest (internal helper or per-method):
if c.token != "" {
    httpReq.Header.Set("Authorization", "Bearer "+c.token)
}
```

### Pattern 5: HTTP Status Error Mapping
**What:** Map server HTTP status codes to human-readable CLI messages, not raw JSON.
**When to use:** In every registry client method, before returning errors.
**Example:**
```go
switch resp.StatusCode {
case http.StatusOK, http.StatusCreated, http.StatusNoContent:
    // success path
case http.StatusUnauthorized:
    return nil, ErrUnauthorized // caller prints "Token expired or invalid. Run `flowy login`..."
case http.StatusForbidden:
    return nil, ErrForbidden    // caller prints "Permission denied: ..."
default:
    var errResp api.ErrorResponse
    _ = json.NewDecoder(resp.Body).Decode(&errResp)
    return nil, fmt.Errorf("server error (status %d): %s", resp.StatusCode, errResp.Error)
}
```

### Pattern 6: tabwriter Table Output
**What:** Aligned tab-separated columns using `text/tabwriter`.
**When to use:** `flowy registry list` output.
**Example:**
```go
w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
fmt.Fprintln(w, "NAME\tKIND\tLATEST TAG\tVERSIONS")
for _, entry := range entries {
    latestTag := "-"
    if len(entry.Tags) > 0 {
        latestTag = entry.Tags[len(entry.Tags)-1]
    }
    fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", entry.Name, entry.Kind, latestTag, entry.VersionCount)
}
w.Flush()
```

### Pattern 7: Hidden Password Input
**What:** `golang.org/x/term.ReadPassword` reads from the terminal without echo.
**When to use:** `flowy login` password prompt.
**Example:**
```go
// locked decision: use fmt.Print for prompt, term.ReadPassword for input
fmt.Print("Password: ")
passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
fmt.Println() // newline after hidden input
if err != nil {
    fmt.Fprintf(stderr, "error reading password: %v\n", err)
    return 1
}
password := string(passwordBytes)
```

### Anti-Patterns to Avoid
- **Printing raw JSON 401/403 to stderr:** Always intercept HTTP status codes and convert to user messages before returning from client methods.
- **Reading `os.Stdin` directly for password:** Use `term.ReadPassword` — it disables terminal echo and works correctly on all platforms.
- **Storing credentials with default file permissions (0644):** Credential file MUST be written with mode 0600 (user-only read/write).
- **Using `os.Exit` inside subcommand functions:** Always return int exit code; `main()` calls `os.Exit` with the result. This allows unit testing without process termination.
- **Writing a nested subcommand via cobra/viper:** Codebase uses stdlib `flag` — do not introduce new CLI framework dependencies.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Hidden password input | Manual terminal echo disable/restore | `golang.org/x/term.ReadPassword` | Handles Ctrl+C, terminal restoration, cross-platform |
| YAML serialization | Custom parser | `gopkg.in/yaml.v3` | Already used by `internal/config`; consistent |
| Aligned table output | Manual space padding | `text/tabwriter` | stdlib; handles variable-width column data correctly |
| JWT validation | Custom token parsing | Already on server-side; client treats token as opaque string | CLI only sends token, never validates it |

**Key insight:** The CLI is a thin HTTP client. It never needs to inspect JWT claims — it only stores and forwards the opaque token string. No JWT parsing library is needed client-side.

## Common Pitfalls

### Pitfall 1: Credentials File Permissions
**What goes wrong:** If credentials file is written with `os.WriteFile(path, data, 0644)`, other users on a shared system can read the JWT token.
**Why it happens:** Go's `os.WriteFile` default perm is often copied from examples without checking.
**How to avoid:** Always use `os.WriteFile(path, data, 0600)`.
**Warning signs:** `ls -la ~/.flowy/credentials.yaml` shows `-rw-r--r--`.

### Pitfall 2: `os.MkdirAll` Permission for Directory
**What goes wrong:** `~/.flowy/` directory created with world-readable `0755` while file is `0600` — directory listing leaks file name.
**How to avoid:** Create directory with `os.MkdirAll(dir, 0700)`.

### Pitfall 3: Leaking Raw JSON Error Responses
**What goes wrong:** Registry client method returns `fmt.Errorf("server error: %s", body)` where body contains raw JSON like `{"error":"token is expired by 2h"}`. CLI prints this to user.
**How to avoid:** In client methods, detect 401/403 before attempting JSON decode, return typed sentinel errors (or error strings that match the locked decision messages). The CLI subcommand function formats the final user-facing message.
**Pattern to use:** Return `errUnauthorized = errors.New("unauthorized")` from client, check in subcommand and print the user-facing message.

### Pitfall 4: Missing `fmt.Println()` After `term.ReadPassword`
**What goes wrong:** After `term.ReadPassword` returns, the terminal cursor is still on the "Password: " line (no newline). The next CLI output appears on the same line.
**How to avoid:** Always call `fmt.Println()` immediately after `ReadPassword` returns.

### Pitfall 5: `flowy registry pull` Printing Trailing Newline
**What goes wrong:** `fmt.Fprintln(stdout, content)` adds a newline to WDL content that may already end in `\n`. When piped to a file, the output has a double newline at end.
**How to avoid:** Use `fmt.Fprint(stdout, content)` — WDL content from the server is the raw file, already newline-terminated. Check if content ends with `\n` before deciding; simplest: just `fmt.Fprint`.

### Pitfall 6: `flag.ExitOnError` vs `flag.ContinueOnError` in Tests
**What goes wrong:** If subcommand uses `flag.ExitOnError`, test calling the function with bad args will call `os.Exit`, killing the test process.
**How to avoid:** Use `flag.ContinueOnError` (already established pattern in this codebase — all existing subcommands use it).

### Pitfall 7: `reorderArgs` Not Applied to Registry Sub-dispatch
**What goes wrong:** `flowy registry push file.wdl --name foo` has a positional before flags. `flag.FlagSet.Parse` stops at first non-flag and misses `--name`.
**How to avoid:** Apply `reorderArgs` in each `runRegistryXxx` function, or confirm that registry flags always precede positional args in the UX design. For `push`, the WDL file is the only positional and all flags (`--project`, `--name`) naturally come before it in typical usage — but `reorderArgs` should be applied defensively.

## Code Examples

Verified patterns from official sources:

### Login HTTP Call
```go
// Source: internal/api/types.go (LoginRequest/LoginResponse already defined)
// Source: internal/server/handlers_auth.go (POST /auth/login returns 200 + {"token":"..."})
func (c *Client) Login(ctx context.Context, username, password string) (string, error) {
    body, _ := json.Marshal(api.LoginRequest{Username: username, Password: password})
    req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.serverURL+"/auth/login", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return "", fmt.Errorf("cannot connect to server at %s: %w", c.serverURL, err)
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        var e api.ErrorResponse
        _ = json.NewDecoder(resp.Body).Decode(&e)
        return "", fmt.Errorf("login failed: %s", e.Error)
    }
    var lr api.LoginResponse
    _ = json.NewDecoder(resp.Body).Decode(&lr)
    return lr.Token, nil
}
```

### Registry Push
```go
// Source: internal/api/types.go (RegistryPushRequest/RegistryPushResponse already defined)
// Source: internal/server/server.go (POST /projects/{slug}/registry, executor role, returns 200 or 201)
func (c *Client) RegistryPush(ctx context.Context, projectSlug, name, content string) (*api.RegistryPushResponse, error) {
    body, _ := json.Marshal(api.RegistryPushRequest{Name: name, Content: content})
    url := c.serverURL + "/projects/" + projectSlug + "/registry"
    req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    if c.token != "" {
        req.Header.Set("Authorization", "Bearer "+c.token)
    }
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("cannot connect to server at %s: %w", c.serverURL, err)
    }
    defer resp.Body.Close()
    switch resp.StatusCode {
    case http.StatusOK, http.StatusCreated:
        var r api.RegistryPushResponse
        _ = json.NewDecoder(resp.Body).Decode(&r)
        return &r, nil
    case http.StatusUnauthorized:
        return nil, errUnauthorized
    case http.StatusForbidden:
        return nil, errForbidden
    default:
        var e api.ErrorResponse
        _ = json.NewDecoder(resp.Body).Decode(&e)
        return nil, fmt.Errorf("push failed (status %d): %s", resp.StatusCode, e.Error)
    }
}
```

### Registry Pull (tag or digest)
```go
// Source: internal/server/server.go — GET /projects/{slug}/registry/{name}?tag=<tag>
// Returns RegistryPullResponse with Content field = raw WDL source
func (c *Client) RegistryPull(ctx context.Context, projectSlug, name, tag, digest string) (*api.RegistryPullResponse, error) {
    url := c.serverURL + "/projects/" + projectSlug + "/registry/" + name
    if tag != "" {
        url += "?tag=" + tag
    } else if digest != "" {
        url += "?digest=" + digest
    }
    // ... (same pattern as above)
}
```

### Registry Delete Tag
```go
// Source: internal/server/server.go — DELETE /projects/{slug}/registry/{name}/tags/{tag}
// Returns 204 No Content
func (c *Client) RegistryDeleteTag(ctx context.Context, projectSlug, name, tag string) error {
    url := c.serverURL + "/projects/" + projectSlug + "/registry/" + name + "/tags/" + tag
    req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
    if c.token != "" {
        req.Header.Set("Authorization", "Bearer "+c.token)
    }
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("cannot connect to server at %s: %w", c.serverURL, err)
    }
    defer resp.Body.Close()
    switch resp.StatusCode {
    case http.StatusNoContent:
        return nil
    // ... 401, 403, default
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| No auth in CLI | JWT Bearer token from credentials file | Phase 33 | All registry commands require `flowy login` first |
| No registry CLI | Full push/list/pull/tag/delete-tag/delete | Phase 33 | Users can manage WDL registry from terminal |

**Deprecated/outdated:**
- None — this is all new functionality.

## Open Questions

1. **`flowy registry pull` — default tag when none specified**
   - What we know: Server requires `?tag=` or `?digest=` (returns 400 without either)
   - What's unclear: Should CLI default to `?tag=latest` or require explicit `--tag`/`--digest`?
   - Recommendation: Require explicit `--tag` flag (or `--digest`). Print a helpful error: "must specify --tag or --digest". This avoids silent behavior change if "latest" doesn't exist.

2. **`flowy registry tag` — required flags**
   - What we know: Server `POST /projects/{slug}/registry/{name}/tags` requires `{"tag":"...", "digest":"..."}` body
   - What's unclear: Should CLI flag be `--tag` and `--digest` (matching server body), or `--from-digest` etc.?
   - Recommendation: `flowy registry tag --project <slug> --name <name> --tag <tag> --digest <digest>` — straightforward mapping to server API.

3. **`flowy login` when auth is disabled on server**
   - What we know: Server returns `{"token":""}` (empty token) when `handleLoginDisabled` is hit (auth.enabled=false)
   - What's unclear: Should CLI save the empty token or warn?
   - Recommendation: If token is empty in the response, print "Warning: server has authentication disabled — no credentials saved." and do not create the credentials file. This avoids writing a useless file.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) |
| Config file | none — `go test ./...` in repo root |
| Quick run command | `go test ./cmd/flowy/... ./internal/client/... -count=1 -timeout 30s` |
| Full suite command | `go test ./... -count=1 -timeout 120s` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CLI-01 | `flowy login` saves token to credentials file | unit | `go test ./cmd/flowy/... -run TestLogin -count=1` | No — Wave 0 |
| CLI-01 | `flowy login` with missing `--server` exits 2 | unit | `go test ./cmd/flowy/... -run TestLoginUsage -count=1` | No — Wave 0 |
| CLI-01 | `saveCredentials` writes with 0600 perm | unit | `go test ./internal/client/... -run TestSaveCredentials -count=1` | No — Wave 0 |
| CLI-02 | `loadCredentials` returns token from file | unit | `go test ./internal/client/... -run TestLoadCredentials -count=1` | No — Wave 0 |
| CLI-02 | Missing credentials file → clear error message | unit | `go test ./cmd/flowy/... -run TestRegistryMissingCreds -count=1` | No — Wave 0 |
| CLI-02 | 401 response → human-readable message (not raw JSON) | unit | `go test ./cmd/flowy/... -run TestRegistryUnauthorized -count=1` | No — Wave 0 |
| REG-CLI-01 | `flowy registry push` sends content, prints digest | unit | `go test ./cmd/flowy/... -run TestRegistryPush -count=1` | No — Wave 0 |
| REG-CLI-02 | `flowy registry list` prints tabwriter table | unit | `go test ./cmd/flowy/... -run TestRegistryList -count=1` | No — Wave 0 |
| REG-CLI-03 | `flowy registry pull` prints WDL to stdout | unit | `go test ./cmd/flowy/... -run TestRegistryPull -count=1` | No — Wave 0 |
| REG-CLI-04 | `flowy registry tag` sends tag+digest to server | unit | `go test ./cmd/flowy/... -run TestRegistryTag -count=1` | No — Wave 0 |
| REG-CLI-04 | `flowy registry delete-tag` sends DELETE request | unit | `go test ./cmd/flowy/... -run TestRegistryDeleteTag -count=1` | No — Wave 0 |
| REG-CLI-05 | `flowy registry delete` sends DELETE, prints nothing | unit | `go test ./cmd/flowy/... -run TestRegistryDelete -count=1` | No — Wave 0 |

All tests use `httptest.NewServer` pattern (see `internal/client/client_test.go`) — no real server needed.

### Sampling Rate
- **Per task commit:** `go test ./cmd/flowy/... ./internal/client/... -count=1 -timeout 30s`
- **Per wave merge:** `go test ./... -count=1 -timeout 120s`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `cmd/flowy/login_test.go` — covers CLI-01 login success, missing server flag, server error
- [ ] `cmd/flowy/registry_test.go` — covers REG-CLI-01 through REG-CLI-05 (push, list, pull, tag, delete-tag, delete)
- [ ] `cmd/flowy/registry_error_test.go` — covers CLI-02 missing creds, 401 human-readable message, 403 message
- [ ] `internal/client/credentials_test.go` — covers CLI-01/CLI-02: saveCredentials perm=0600, loadCredentials round-trip, missing file error

## Server Endpoint Reference

All endpoints already exist (confirmed in `internal/server/server.go`):

| Operation | Method | URL | Role | Request Body | Success |
|-----------|--------|-----|------|--------------|---------|
| Login | POST | `/auth/login` | public | `{"username":"...","password":"..."}` | 200 `{"token":"..."}` |
| Push | POST | `/projects/{slug}/registry` | executor | `{"name":"...","content":"..."}` | 200/201 `RegistryPushResponse` |
| List | GET | `/projects/{slug}/registry` | viewer | — | 200 `[]RegistryEntryResponse` |
| Pull | GET | `/projects/{slug}/registry/{name}?tag=X` | viewer | — | 200 `RegistryPullResponse` |
| Tag | POST | `/projects/{slug}/registry/{name}/tags` | admin | `{"tag":"...","digest":"..."}` | 200 |
| Delete tag | DELETE | `/projects/{slug}/registry/{name}/tags/{tag}` | admin | — | 204 |
| Delete entry | DELETE | `/projects/{slug}/registry/{name}` | admin | — | 204 |

## Sources

### Primary (HIGH confidence)
- Inspected source: `internal/server/server.go` — all registry routes confirmed
- Inspected source: `internal/server/handlers_registry.go` — server handler logic confirmed
- Inspected source: `internal/server/handlers_auth.go` — `POST /auth/login` returns `{"token":"..."}`
- Inspected source: `internal/api/types.go` — all request/response DTOs confirmed present
- Inspected source: `internal/client/client.go` — established HTTP client pattern
- Inspected source: `cmd/flowy/main.go` — established CLI dispatch and subcommand patterns
- Inspected source: `internal/config/config.go` — YAML loading pattern with `gopkg.in/yaml.v3`
- Inspected source: `go.mod` — confirms `golang.org/x/term v0.41.0` and `gopkg.in/yaml.v3 v3.0.1` already present

### Secondary (MEDIUM confidence)
- `text/tabwriter` stdlib package — well-documented, zero-risk
- `os.WriteFile` with perm `0600` — standard Unix credential security practice

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all packages confirmed in go.mod or stdlib
- Architecture: HIGH — based on direct inspection of existing codebase patterns
- Server endpoints: HIGH — confirmed from router source and handler source
- DTO types: HIGH — confirmed in `internal/api/types.go`
- Pitfalls: HIGH — derived from code inspection and established Go patterns

**Research date:** 2026-04-07
**Valid until:** 2026-06-07 (stable Go codebase; no external APIs changing)
