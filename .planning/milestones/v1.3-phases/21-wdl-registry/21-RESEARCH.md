# Phase 21: WDL Registry - Research

**Researched:** 2026-03-20
**Domain:** Go HTTP API, SQLite persistence, WDL content-addressed storage
**Confidence:** HIGH

## Summary

Phase 21 adds a WDL registry sub-resource under `/projects/{slug}/registry/...` that lets executors push versioned WDL files and retrieve them by tag or digest. The registry is entirely additive — no existing routes or tables change. All required building blocks exist in the codebase: the chi route group pattern (Phase 20), the `projectContext`/`projectRoleRequired` middleware, the goose migration system, the WDL parser (`parser.Parse`), and `crypto/sha256` in stdlib.

The three-table schema (registry_entries / registry_versions / registry_tags) mirrors a simplified Docker distribution model: an entry is a named slot, a version is an immutable content-addressed blob, and a tag is a mutable pointer to a version. This design lets tags be reassigned or deleted without touching immutable version records.

The hardest implementation decision is the idempotent-push contract: if the same SHA-256 content is pushed to the same entry, no new version row is created and the existing version is returned. This requires a SELECT before INSERT in the push handler and must be done under a write-connection transaction to avoid a race.

**Primary recommendation:** Follow the exact same handler / store / migration layering used for Phase 20 projects. New files: `handlers_registry.go`, `internal/store/sqlite/registry.go`, `internal/store/sqlite/migrations/00006_registry.sql`, new types in `internal/api/types.go`, new store methods in `internal/store/store.go`.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Push & versioning behavior**
- Entry name is user-specified in the push request (not derived from WDL source)
- Entry names follow the same slug rules as projects: lowercase alphanumeric + hyphens, max 64 chars
- SHA-256 digest is auto-computed from WDL content on push
- Idempotent re-push: if the same content (identical SHA-256) is pushed to the same entry, no new version is created — return the existing version with its digest
- Auto-classification on push: use the existing WDL parser to detect task vs workflow; fail push if WDL doesn't parse (strict, not lenient)
- Kind (tool/workflow) stored on registry_entries

**Tag semantics**
- Tags are mutable pointers (can be reassigned to a different version)
- No default tag on push — push creates a version with digest only; tags are assigned explicitly via a separate endpoint
- Tags can be deleted independently from versions (version and its digest remain accessible)
- Only project admin can create, reassign, and delete tags (executors cannot tag)
- Tag names: any string up to 128 chars, except empty strings and strings containing '@' or ':' (reserved for reference parsing)

**Pull & reference format**
- Reference format: `name:tag` or `name@sha256:hex`
- Pull response: JSON wrapper with metadata — `{"name": "...", "digest": "sha256:...", "kind": "tool|workflow", "content": "version 1.0\n..."}`
- 404 errors include specific reason: `{"error": "tag 'prod' not found for entry 'align-reads'"}` — consistent with existing ErrorResponse pattern
- List entries endpoint returns: name, kind, version_count, tags (names only), created_at — not the full version tree
- Dedicated versions endpoint: GET /projects/{slug}/registry/{name}/versions returns all versions (digest, kind, created_at) and their tags

**Auth-disabled behavior**
- Registry endpoints do not exist when auth.enabled=false — registry is a multi-user feature requiring project context
- Existing /workflows routes and spectool tests are completely unaffected — registry is purely additive (new endpoints, new tables, no changes to existing paths)

### Claude's Discretion
- Three-table schema details (registry_entries/registry_versions/registry_tags column specifics beyond what's locked above)
- Migration numbering and strategy (next after 00005_projects.sql)
- API response shapes for list and version endpoints beyond the fields specified
- Route group structure within /projects/{slug}/registry/...
- WDL parser integration approach (how to call existing parser for classification)

### Deferred Ideas (OUT OF SCOPE)
- Cross-project registry sharing (REG-F01: `is_public` flag)
- GA4GH TRS API compatibility (REG-F02)
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| REG-01 | Executor can push a WDL file to the project registry, auto-computing SHA-256 digest | `crypto/sha256` stdlib; idempotent-push via digest lookup before insert; WDL parser for classification |
| REG-02 | Executor can assign a tag to a registry version (mutable pointer) | Separate tag endpoint; UPDATE existing row when tag already exists; admin-only via `projectRoleRequired("admin")` |
| REG-03 | Viewer can pull WDL content by tag (`name:tag`) or digest (`name@sha256:hex`) | Parse reference string in handler; two store query paths: GetVersionByTag / GetVersionByDigest |
| REG-04 | Viewer can list registry entries, versions, and tags in a project | Two endpoints: list entries (summary) and list versions for one entry; reader pool queries |
| REG-05 | Registry entries are classified as tool (WDL task) or workflow (WDL workflow) | `parser.Parse(name, []byte(content))` returns `*ast.Document`; inspect `doc.Tasks` and `doc.Workflows` lengths |
| REG-06 | Project admin can delete registry entries | Soft-delete via `is_active=0` on registry_entries; cascade effect on associated versions is application-layer only (versions remain queryable by digest) |
</phase_requirements>

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `crypto/sha256` | stdlib | Auto-compute digest on push | No external dep; deterministic hex encoding via `fmt.Sprintf("sha256:%x", ...)` |
| `github.com/go-chi/chi/v5` | v5.2.5 (already in go.mod) | Route group `/projects/{slug}/registry/...` | Already used for all server routing |
| `github.com/pressly/goose/v3` | v3.27.0 (already in go.mod) | Migration `00006_registry.sql` | Already used for all schema migrations |
| `github.com/google/uuid` | v1.6.0 (already in go.mod) | UUID primary keys for registry_entries and registry_versions | Already used for runs, users, projects |
| `github.com/bioflowy/flowy/wdl/parser` | local | WDL parse + classification on push | `parser.Parse(name, content)` returns `*ast.Document` with `Tasks` and `Workflows` slices |

**No new dependencies needed.** All required packages are already present in go.mod.

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `crypto/sha256` + `fmt.Sprintf("sha256:%x", ...)` | external digest library | stdlib is sufficient; no reason to add dependency |
| Soft-delete entries via `is_active` | Hard delete | Soft-delete consistent with users/projects pattern; hard delete risks foreign key issues if future phases reference versions |

## Architecture Patterns

### Recommended Project Structure

```
internal/
├── api/
│   └── types.go              # Add registry request/response types
├── store/
│   ├── store.go              # Add registry model types + Store interface methods
│   └── sqlite/
│       ├── registry.go       # New: SQLiteStore registry method implementations
│       └── migrations/
│           └── 00006_registry.sql  # New: registry_entries/versions/tags tables
└── server/
    ├── server.go             # Add /registry route group inside /{slug} route
    └── handlers_registry.go  # New: push/pull/tag/list/delete handlers
```

### Pattern 1: Three-Table Schema

**What:** `registry_entries` holds the named slot (name, kind, is_active). `registry_versions` holds immutable content blobs (digest, wdl_content, entry_id, created_at). `registry_tags` holds mutable name → version_id pointers.

**When to use:** Any content-addressable registry where tags must be mutable but blobs must be immutable.

**Schema:**

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS registry_entries (
    id         TEXT     PRIMARY KEY,
    project_id TEXT     NOT NULL REFERENCES projects(id),
    name       TEXT     NOT NULL,
    kind       TEXT     NOT NULL,     -- "tool" | "workflow"
    is_active  INTEGER  NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE (project_id, name)
);
CREATE INDEX IF NOT EXISTS idx_registry_entries_project ON registry_entries(project_id);

CREATE TABLE IF NOT EXISTS registry_versions (
    id          TEXT     PRIMARY KEY,
    entry_id    TEXT     NOT NULL REFERENCES registry_entries(id),
    digest      TEXT     NOT NULL,     -- "sha256:<hex>"
    wdl_content TEXT     NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE (entry_id, digest)
);
CREATE INDEX IF NOT EXISTS idx_registry_versions_entry ON registry_versions(entry_id);
CREATE INDEX IF NOT EXISTS idx_registry_versions_digest ON registry_versions(digest);

CREATE TABLE IF NOT EXISTS registry_tags (
    id         TEXT     PRIMARY KEY,
    entry_id   TEXT     NOT NULL REFERENCES registry_entries(id),
    name       TEXT     NOT NULL,
    version_id TEXT     NOT NULL REFERENCES registry_versions(id),
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE (entry_id, name)
);
CREATE INDEX IF NOT EXISTS idx_registry_tags_entry ON registry_tags(entry_id);
```

### Pattern 2: SHA-256 Digest Computation

**What:** Compute digest from raw WDL content bytes on the server side during push.

**Example:**

```go
// Source: stdlib crypto/sha256
import (
    "crypto/sha256"
    "fmt"
)

func computeDigest(content string) string {
    h := sha256.Sum256([]byte(content))
    return fmt.Sprintf("sha256:%x", h)
}
```

### Pattern 3: WDL Classification via Parser

**What:** Call the existing `parser.Parse` to detect whether WDL content defines a `task` (kind="tool") or `workflow` (kind="workflow"). Fail the push if parse fails.

**Example:**

```go
// Source: wdl/parser/parser.go
import "github.com/bioflowy/flowy/wdl/parser"

func classifyWDL(name, content string) (string, error) {
    doc, err := parser.Parse(name, []byte(content))
    if err != nil {
        return "", fmt.Errorf("WDL parse error: %w", err)
    }
    if len(doc.Workflows) > 0 {
        return "workflow", nil
    }
    if len(doc.Tasks) > 0 {
        return "tool", nil
    }
    return "", fmt.Errorf("WDL file defines neither a task nor a workflow")
}
```

Note: `parser.Parse` takes `(file string, src []byte)`. Pass the entry name as the file parameter (used only in error messages).

### Pattern 4: Idempotent Push Transaction

**What:** Under the write connection, check if a version with the same digest already exists for the entry before inserting. Return existing version if found.

**Example:**

```go
// Under s.writer transaction:
// 1. GetOrCreateEntry(projectID, name, kind) — upsert entry row
// 2. SELECT id FROM registry_versions WHERE entry_id=? AND digest=?
//    → if found: return existing version (idempotent)
//    → if not found: INSERT new version row
```

This must use the writer connection (not reader) for both the SELECT and INSERT to avoid TOCTOU races under WAL mode.

### Pattern 5: Reference Parsing

**What:** Parse `name:tag` vs `name@sha256:hex` in the pull handler.

**Example:**

```go
func parseRef(ref string) (name, tag, digest string, err error) {
    if i := strings.Index(ref, "@"); i >= 0 {
        return ref[:i], "", ref[i+1:], nil  // digest ref
    }
    if i := strings.Index(ref, ":"); i >= 0 {
        return ref[:i], ref[i+1:], "", nil  // tag ref
    }
    return "", "", "", fmt.Errorf("invalid registry reference %q: must be name:tag or name@sha256:hex", ref)
}
```

### Pattern 6: Route Group (following Phase 20 pattern)

**What:** Nest registry routes inside the existing `/{slug}` route group in `server.go:newRouter()`.

**Example:**

```go
// Inside r.Route("/{slug}", func(r chi.Router) { ... }) in server.go:
r.Route("/registry", func(r chi.Router) {
    r.With(s.projectRoleRequired("executor")).Post("/", s.handleRegistryPush)
    r.With(s.projectRoleRequired("viewer")).Get("/", s.handleRegistryList)
    r.Route("/{name}", func(r chi.Router) {
        r.With(s.projectRoleRequired("viewer")).Get("/", s.handleRegistryPull)
        r.With(s.projectRoleRequired("viewer")).Get("/versions", s.handleRegistryListVersions)
        r.With(s.projectRoleRequired("admin")).Delete("/", s.handleRegistryDelete)
        r.With(s.projectRoleRequired("admin")).Post("/tags", s.handleRegistryTag)
        r.With(s.projectRoleRequired("admin")).Delete("/tags/{tag}", s.handleRegistryDeleteTag)
    })
})
```

Note: Pull reference (`name:tag` or `name@sha256:hex`) is in query string `?ref=name:tag` OR the `{name}` URL param holds the entry name and `?tag=` / `?digest=` query params differentiate. Using `{name}` + query params is simpler than URL encoding colons.

**Recommendation for pull URL:** `GET /projects/{slug}/registry/{name}?ref=tag:value` or `?digest=sha256:hex`. This avoids URL encoding issues with colons in chi URL params. Alternatively, the full reference string can be passed as a query param: `?ref=name:tag`.

Actually, the cleaner REST design is:
- `GET /projects/{slug}/registry/{name}?tag=<tagname>` — pull by tag
- `GET /projects/{slug}/registry/{name}?digest=sha256:<hex>` — pull by digest

The `name` goes in the path; the version selector goes in query params. This avoids all reference parsing ambiguity in URL routing.

### Anti-Patterns to Avoid

- **Reading content from disk:** WDL content is sent as a string in the request body — never read from filesystem in registry handlers.
- **Using reader pool for idempotent-push check:** The SELECT-then-INSERT for idempotent push MUST use the writer connection to avoid TOCTOU races.
- **Kind on versions vs entries:** Kind is stored on `registry_entries`, not `registry_versions`. A re-push of the same entry with the same content keeps the same kind. If an entry is re-pushed with different content but different kind, that is a conflict — decision: kind is set on first push and never changes (the entry kind is immutable after creation; a push that would change kind should fail with a 409 Conflict).
- **Registering registry routes when auth is disabled:** Registry routes must only exist inside the `if s.cfg.Auth.Enabled` block.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Content digest | Custom hash | `crypto/sha256` stdlib | Standard; no dependency; same as Docker image digest format |
| WDL syntax validation | Custom parser | `parser.Parse` from `wdl/parser` | Already handles WDL 1.2 grammar; strict failure on parse error is the contract |
| Slug validation | New regex | `validSlug` in `handlers_projects.go` | Same rules for entry names as projects |
| UUID generation | Random hex | `github.com/google/uuid` | Already used everywhere for IDs |
| Migration | Custom schema tool | goose `00006_registry.sql` | Matches all existing migrations |

## Common Pitfalls

### Pitfall 1: Kind Mismatch on Re-Push

**What goes wrong:** An executor pushes new WDL content to an existing entry. The new content might be a workflow when the entry was originally a tool. The kind field is on `registry_entries`, not `registry_versions`, so changing kind would silently corrupt the entry's classification.

**Why it happens:** The push handler creates/gets the entry first, then adds a version. If kind is re-computed each push and stored on the entry, a re-push with different content changes the kind.

**How to avoid:** On push, if the entry already exists and the computed kind differs from the stored kind, return 409 Conflict with a message like `{"error": "entry 'align-reads' is kind 'tool'; cannot push a 'workflow' to it"}`.

**Warning signs:** Integration tests that push a workflow WDL to an entry previously used for a task and don't get an error.

### Pitfall 2: TOCTOU in Idempotent Push

**What goes wrong:** Two concurrent pushes of the same content hit the server simultaneously. Both read "no version exists" and both try to insert — one fails with a UNIQUE constraint error.

**Why it happens:** The SELECT and INSERT are not atomic by default on the reader/writer pool pattern.

**How to avoid:** Use a single writer-connection transaction: BEGIN; SELECT (on writer); INSERT if not found; COMMIT. Alternatively, use `INSERT OR IGNORE` followed by `SELECT` on the writer. A UNIQUE constraint on `(entry_id, digest)` ensures the insert is idempotent at the DB level; the handler can treat the constraint error as "already exists" and return the existing version.

**Warning signs:** Flaky tests under parallel push.

### Pitfall 3: Reference Parsing in URL

**What goes wrong:** Using `:` in chi URL params (e.g., `/registry/name:tag`) breaks chi routing because `:` has special meaning in URL patterns.

**Why it happens:** Chi uses `:` to denote URL parameters (e.g., `/{name:tag}` is misread).

**How to avoid:** Keep `{name}` in the path; use `?tag=` and `?digest=` query params for version selection.

### Pitfall 4: Forgetting Auth Guard on Registry Routes

**What goes wrong:** Registry route group is registered outside the `if s.cfg.Auth.Enabled` block, making it accessible without auth.

**Why it happens:** Copy/paste from the flat (auth-disabled) route registration.

**How to avoid:** All registry routes must be inside the `r.Route("/{slug}", ...)` sub-router, which is already inside `if s.cfg.Auth.Enabled`.

### Pitfall 5: Content Type for Push

**What goes wrong:** Client sends WDL content as `multipart/form-data` or `text/plain` but handler expects `application/json`.

**Why it happens:** WDL is a text file; common instinct is to upload as form data.

**How to avoid:** Define a JSON push request type with a `content` string field — consistent with all other handlers. Document this in the API type comment.

### Pitfall 6: List Entries N+1 Query

**What goes wrong:** List entries handler issues one query per entry to count versions and fetch tag names, resulting in O(n) queries.

**Why it happens:** Simple loop over entries.

**How to avoid:** Use a JOIN with GROUP BY or a subquery to fetch version_count and tag names in a single query. At minimum, document this as a known limitation and use a simple approach with a comment for future optimization.

## Code Examples

### Push Handler (skeleton)

```go
// Source: internal/server/handlers_registry.go (new)
func (s *Server) handleRegistryPush(w http.ResponseWriter, r *http.Request) {
    proj := projectFromContext(r.Context())
    var req api.RegistryPushRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }
    if !validSlug(req.Name) {
        writeError(w, http.StatusBadRequest, "invalid entry name: must be lowercase alphanumeric with hyphens, 1-64 chars")
        return
    }
    // Classify WDL
    kind, err := classifyWDL(req.Name, req.Content)
    if err != nil {
        writeError(w, http.StatusUnprocessableEntity, err.Error())
        return
    }
    // Compute digest
    digest := computeDigest(req.Content)
    // Persist (idempotent)
    version, created, err := s.store.PushRegistryVersion(r.Context(), proj.ID, req.Name, kind, digest, req.Content)
    if err != nil {
        // handle kind mismatch, etc.
        ...
    }
    status := http.StatusCreated
    if !created {
        status = http.StatusOK
    }
    writeJSON(w, status, api.RegistryPushResponse{
        Name:      req.Name,
        Digest:    version.Digest,
        Kind:      kind,
        VersionID: version.ID,
    })
}
```

### Pull Handler (skeleton)

```go
// GET /projects/{slug}/registry/{name}?tag=<tag> or ?digest=sha256:<hex>
func (s *Server) handleRegistryPull(w http.ResponseWriter, r *http.Request) {
    proj := projectFromContext(r.Context())
    name := chi.URLParam(r, "name")
    tag := r.URL.Query().Get("tag")
    digest := r.URL.Query().Get("digest")

    var version *store.RegistryVersion
    var entry *store.RegistryEntry
    var err error

    switch {
    case tag != "":
        version, entry, err = s.store.GetRegistryVersionByTag(r.Context(), proj.ID, name, tag)
        if err != nil {
            writeError(w, http.StatusNotFound, fmt.Sprintf("tag %q not found for entry %q", tag, name))
            return
        }
    case digest != "":
        version, entry, err = s.store.GetRegistryVersionByDigest(r.Context(), proj.ID, name, digest)
        if err != nil {
            writeError(w, http.StatusNotFound, fmt.Sprintf("digest %q not found for entry %q", digest, name))
            return
        }
    default:
        writeError(w, http.StatusBadRequest, "must specify ?tag= or ?digest=")
        return
    }

    writeJSON(w, http.StatusOK, api.RegistryPullResponse{
        Name:    entry.Name,
        Digest:  version.Digest,
        Kind:    entry.Kind,
        Content: version.WDLContent,
    })
}
```

### Store Interface Methods (additions to store.go)

```go
// Registry model types to add to store.go:

type RegistryEntry struct {
    ID        string
    ProjectID string
    Name      string
    Kind      string    // "tool" | "workflow"
    IsActive  bool
    CreatedAt time.Time
    UpdatedAt time.Time
}

type RegistryVersion struct {
    ID         string
    EntryID    string
    Digest     string    // "sha256:<hex>"
    WDLContent string
    CreatedAt  time.Time
}

type RegistryTag struct {
    ID        string
    EntryID   string
    Name      string
    VersionID string
    CreatedAt time.Time
    UpdatedAt time.Time
}

// RegistryEntryListItem is the summary row returned by ListRegistryEntries.
type RegistryEntryListItem struct {
    Name         string
    Kind         string
    VersionCount int
    Tags         []string  // tag names only
    CreatedAt    time.Time
}

// Store interface additions:
//
// PushRegistryVersion creates or gets a version for the given entry (idempotent by digest).
// Returns the version, a boolean indicating if it was newly created, and an error.
// Returns an error if the entry's kind differs from the computed kind.
PushRegistryVersion(ctx context.Context, projectID, name, kind, digest, content string) (*RegistryVersion, bool, error)
// GetRegistryVersionByTag returns the version pointed to by the tag.
GetRegistryVersionByTag(ctx context.Context, projectID, name, tag string) (*RegistryVersion, *RegistryEntry, error)
// GetRegistryVersionByDigest returns the version with the given digest.
GetRegistryVersionByDigest(ctx context.Context, projectID, name, digest string) (*RegistryVersion, *RegistryEntry, error)
// ListRegistryEntries returns summary rows for all active entries in a project.
ListRegistryEntries(ctx context.Context, projectID string) ([]RegistryEntryListItem, error)
// ListRegistryVersions returns all versions for the named entry.
ListRegistryVersions(ctx context.Context, projectID, name string) ([]RegistryVersion, error)
// UpsertRegistryTag creates or reassigns a tag to a version.
UpsertRegistryTag(ctx context.Context, entryID, tagName, versionID string) error
// DeleteRegistryTag removes a tag (version remains).
DeleteRegistryTag(ctx context.Context, entryID, tagName string) error
// DeleteRegistryEntry soft-deletes the entry (sets is_active=0).
DeleteRegistryEntry(ctx context.Context, projectID, name string) error
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Filesystem WDL files | WDL stored as TEXT in SQLite | v1.3 decision | Registry works in local deployments without shared filesystem |
| No versioning | Content-addressed versions by SHA-256 digest | Phase 21 | Immutable blobs; safe CI/CD retries |
| No tagging | Mutable tags as separate rows | Phase 21 | Tags can be reassigned without touching immutable blobs |

## Open Questions

1. **Pull endpoint URL design for the full `name:tag` reference string**
   - What we know: CONTEXT.md says reference format is `name:tag` or `name@sha256:hex`
   - What's unclear: Whether the pull endpoint takes the full reference string as one query param (`?ref=name:tag`) or splits name into path (`/{name}`) and version into query params (`?tag=` / `?digest=`)
   - Recommendation: Use `/{name}?tag=<tag>` and `/{name}?digest=<digest>` to avoid URL-encoding colons. The `name` is a slug and safe in URL paths. This is also consistent with the `{name}` URL param used in the delete and versions endpoints.

2. **Kind conflict on re-push of existing entry**
   - What we know: Kind is stored on `registry_entries`, not versions. CONTEXT.md says kind is stored on entries but doesn't specify what happens if re-push would change kind.
   - What's unclear: Return 409 or silently accept (ignoring kind change)?
   - Recommendation: Return 409 Conflict — this prevents silent corruption and is the safe default. Document in API type comment.

3. **ListRegistryVersions — include tag names per version?**
   - What we know: CONTEXT.md says GET /versions returns "all versions (digest, kind, created_at) and their tags"
   - What's unclear: "their tags" means each version row should include its associated tag names
   - Recommendation: Add a `Tags []string` field to the RegistryVersionListItem response type. Fetch via a JOIN or a second query per version (acceptable at small scale; document for future optimization).

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go testing (`testing` package), httptest |
| Config file | none — stdlib testing |
| Quick run command | `go test ./internal/server/ -run TestRegistry -v` |
| Full suite command | `go test ./...` |

### Phase Requirements -> Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REG-01 | Push WDL returns digest; idempotent re-push returns same digest | unit | `go test ./internal/server/ -run TestRegistryPush -v` | Wave 0 |
| REG-01 | Push invalid WDL returns 422 | unit | `go test ./internal/server/ -run TestRegistryPush_InvalidWDL -v` | Wave 0 |
| REG-02 | Admin can assign/reassign tag to version | unit | `go test ./internal/server/ -run TestRegistryTag -v` | Wave 0 |
| REG-02 | Executor cannot assign tags (403) | unit | `go test ./internal/server/ -run TestRegistryTag_Forbidden -v` | Wave 0 |
| REG-03 | Pull by tag returns correct WDL content | unit | `go test ./internal/server/ -run TestRegistryPull_ByTag -v` | Wave 0 |
| REG-03 | Pull by digest returns correct WDL content | unit | `go test ./internal/server/ -run TestRegistryPull_ByDigest -v` | Wave 0 |
| REG-03 | Pull unknown tag returns 404 with specific error | unit | `go test ./internal/server/ -run TestRegistryPull_NotFound -v` | Wave 0 |
| REG-04 | List entries returns name, kind, version_count, tags | unit | `go test ./internal/server/ -run TestRegistryList -v` | Wave 0 |
| REG-04 | List versions for entry returns all versions with tags | unit | `go test ./internal/server/ -run TestRegistryListVersions -v` | Wave 0 |
| REG-05 | Push WDL task → kind="tool"; push workflow → kind="workflow" | unit | `go test ./internal/server/ -run TestRegistryPush_Classification -v` | Wave 0 |
| REG-06 | Admin can delete entry; entry disappears from list | unit | `go test ./internal/server/ -run TestRegistryDelete -v` | Wave 0 |
| REG-06 | Non-admin cannot delete entry (403) | unit | `go test ./internal/server/ -run TestRegistryDelete_Forbidden -v` | Wave 0 |

### Sampling Rate

- **Per task commit:** `go test ./internal/server/ -run TestRegistry -v`
- **Per wave merge:** `go test ./internal/server/ ./internal/store/... -v`
- **Phase gate:** `go test ./...` full suite green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `internal/server/handlers_registry_test.go` — covers REG-01 through REG-06
- [ ] `internal/store/sqlite/registry.go` — store implementation (needed before handler tests can run)
- [ ] `internal/store/sqlite/migrations/00006_registry.sql` — schema needed before store tests

*(No new test framework to install — reuses stdlib testing + httptest already in use)*

## Sources

### Primary (HIGH confidence)

- Codebase direct inspection — `internal/store/store.go`, `internal/server/server.go`, `internal/server/middleware.go`, `internal/server/handlers_projects.go`, `internal/api/types.go`
- `internal/store/sqlite/projects.go` — scan/upsert pattern to replicate
- `internal/store/sqlite/sqlite.go` — writer/reader pool pattern; migration embedding
- `internal/store/sqlite/migrations/00005_projects.sql` — migration format to follow
- `wdl/parser/parser.go` line 78 — `Parse(file string, src []byte) (*ast.Document, error)` signature confirmed
- `wdl/ast/ast.go` lines 59-65 — `Document.Tasks []*Task`, `Document.Workflows []*Workflow` confirmed
- `go.mod` — confirmed `crypto/sha256` (stdlib), `chi/v5 v5.2.5`, `goose/v3 v3.27.0`, `uuid v1.6.0` all present
- `internal/server/handlers_auth_test.go` — test helper pattern (`newAuthTestServer`, `issueTestToken`, `doAuthRequest`, `createTestUser`)

### Secondary (MEDIUM confidence)

- Phase 21 CONTEXT.md — locked decisions verified against codebase patterns

### Tertiary (LOW confidence)

- None — all findings are directly from codebase inspection

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries confirmed in go.mod; no new dependencies
- Architecture: HIGH — patterns directly derived from existing Phase 20 code
- Pitfalls: HIGH — idempotent push race and kind-conflict identified from schema design; URL routing pitfall from chi conventions
- Test mapping: HIGH — test helper patterns confirmed from existing test files

**Research date:** 2026-03-20
**Valid until:** 2026-06-20 (stable codebase; only invalidated if Phase 20 architecture changes)
