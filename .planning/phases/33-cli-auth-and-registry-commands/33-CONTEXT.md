# Phase 33: CLI Auth and Registry Commands - Context

**Gathered:** 2026-04-07
**Status:** Ready for planning

<domain>
## Phase Boundary

CLI users can authenticate once with `flowy login` and then push, list, pull, tag, and delete registry entries without repeating credentials. This phase adds the `login` command and the full `registry` subcommand tree to the `flowy` CLI binary.

</domain>

<decisions>
## Implementation Decisions

### Login Flow & Credential Storage
- Use `fmt.Print` for username prompt and `golang.org/x/term` `ReadPassword` for hidden password input
- Credential file: YAML format with `server` and `token` fields at `~/.flowy/credentials.yaml` (mode 0600), using `gopkg.in/yaml.v3` consistent with existing config pattern
- Single server support only — credentials file stores one server+token pair (multi-server deferred)
- No automatic token refresh — if token expired, print "Token expired. Run `flowy login` to authenticate." and exit code 1

### Registry Command Structure
- Single-level nesting under `registry`: `flowy registry push/list/pull/tag/delete-tag/delete`
- `list` output: human-readable table with columns (Name, Kind, Latest Tag, Versions) via `text/tabwriter`
- `pull` prints WDL source to stdout (pipeable with `> file.wdl`)
- `push` prints one line on success: `Pushed {name} digest:{hash}`

### Error Handling & UX
- Missing credentials file: "Not authenticated. Run `flowy login --server <url>` first." (exit 1)
- Server connection failure: "Cannot connect to server at {url}: {error}" (exit 1)
- Permission denied (403): "Permission denied: {role} role required for this operation" (exit 1)
- Expired/invalid token (401): "Token expired or invalid. Run `flowy login` to authenticate." (exit 1) — NOT raw JSON 401
- `delete` has no confirmation prompt — admin-only, keep CLI scriptable per Unix conventions

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- Custom flag-based CLI dispatch in `cmd/flowy/main.go` — switch statement on subcommand
- `internal/client/client.go` — HTTP client wrapper with JSON serialization, 30s timeout
- `internal/api/types.go` — `LoginRequest`, `LoginResponse`, `RegistryPushRequest`, `RegistryPullResponse`, etc. already defined
- `internal/config/config.go` — YAML config loader using `gopkg.in/yaml.v3`
- `internal/auth/auth.go` — `IssueToken()`, `ValidateToken()` for JWT handling

### Established Patterns
- Subcommand pattern: `func runXxxCommand(args []string) int` returning exit code
- Flag parsing: `flag.NewFlagSet(name, flag.ExitOnError)` per subcommand
- Client methods: marshal JSON request, HTTP call, decode response
- Error output to stderr: `fmt.Fprintf(os.Stderr, ...)`

### Integration Points
- New `login` and `registry` cases in `cmd/flowy/main.go` switch statement
- New client methods in `internal/client/client.go` for registry operations
- New credentials file loader (new file, e.g., `internal/client/credentials.go`)
- Server's `POST /auth/login` endpoint returns JWT (already exists)
- Server's registry endpoints (push, list, pull, tag, delete) are JWT-protected (Phase 32 verified)

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches following existing codebase patterns.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>
