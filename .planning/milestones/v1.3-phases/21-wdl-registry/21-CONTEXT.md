# Phase 21: WDL Registry - Context

**Gathered:** 2026-03-20
**Status:** Ready for planning

<domain>
## Phase Boundary

Add a WDL registry to flowy-server where executors can push versioned WDL tool and workflow files to a project registry and retrieve them by tag or immutable SHA-256 digest. Registry is project-scoped and only available when auth is enabled. Execution by registry reference (EXEC-01/EXEC-02) is a separate phase.

</domain>

<decisions>
## Implementation Decisions

### Push & versioning behavior
- Entry name is user-specified in the push request (not derived from WDL source)
- Entry names follow the same slug rules as projects: lowercase alphanumeric + hyphens, max 64 chars
- SHA-256 digest is auto-computed from WDL content on push
- Idempotent re-push: if the same content (identical SHA-256) is pushed to the same entry, no new version is created — return the existing version with its digest
- Auto-classification on push: use the existing WDL parser to detect task vs workflow; fail push if WDL doesn't parse (strict, not lenient)
- Kind (tool/workflow) stored on registry_entries

### Tag semantics
- Tags are mutable pointers (can be reassigned to a different version)
- No default tag on push — push creates a version with digest only; tags are assigned explicitly via a separate endpoint
- Tags can be deleted independently from versions (version and its digest remain accessible)
- Only project admin can create, reassign, and delete tags (executors cannot tag)
- Tag names: any string up to 128 chars, except empty strings and strings containing '@' or ':' (reserved for reference parsing)

### Pull & reference format
- Reference format: `name:tag` or `name@sha256:hex` (already decided in REQUIREMENTS)
- Pull response: JSON wrapper with metadata — `{"name": "...", "digest": "sha256:...", "kind": "tool|workflow", "content": "version 1.0\n..."}`
- 404 errors include specific reason: `{"error": "tag 'prod' not found for entry 'align-reads'"}` — consistent with existing ErrorResponse pattern
- List entries endpoint returns: name, kind, version_count, tags (names only), created_at — not the full version tree
- Dedicated versions endpoint: GET /projects/{slug}/registry/{name}/versions returns all versions (digest, kind, created_at) and their tags

### Auth-disabled behavior
- Registry endpoints do not exist when auth.enabled=false — registry is a multi-user feature requiring project context
- Existing /workflows routes and spectool tests are completely unaffected — registry is purely additive (new endpoints, new tables, no changes to existing paths)

### Claude's Discretion
- Three-table schema details (registry_entries/registry_versions/registry_tags column specifics beyond what's locked above)
- Migration numbering and strategy (next after 00005_projects.sql)
- API response shapes for list and version endpoints beyond the fields specified
- Route group structure within /projects/{slug}/registry/...
- WDL parser integration approach (how to call existing parser for classification)

</decisions>

<specifics>
## Specific Ideas

- Registry references use project slug: `project/name:tag` — this format is used by Phase 22 (EXEC-01) for run submission by registry reference
- Tagging is an admin-only operation — separates "push code" (executor) from "bless a version" (admin) workflows
- Idempotent push enables safe CI/CD retries without version noise

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `parser.Parse(path, content)` in WDL parser — can detect task vs workflow blocks for auto-classification
- `WDLContent` field already used in `TaskClaimResponse` — established pattern for passing WDL as text
- `projectContext` + `projectRoleRequired` middleware (Phase 20) — reuse for registry route groups
- `Store` interface pattern with SQLite implementation — add registry method groups
- `ErrorResponse` struct for JSON error responses
- `validSlug` function in handlers_projects.go — reuse for entry name validation

### Established Patterns
- goose migrations in `internal/store/sqlite/migrations/` — next is `00006`
- chi route groups with middleware (Phases 19-20)
- UUID primary keys (projects, users, runs)
- Soft delete via `is_active` boolean
- SHA-256 in crypto/sha256 stdlib — no external dependency needed

### Integration Points
- `server.go:newRouter()` — add `/projects/{slug}/registry/...` sub-resource inside auth-enabled project route group
- `store.go:Store` — add CreateRegistryEntry, GetRegistryEntry, ListRegistryEntries, CreateVersion, GetVersionByDigest, GetVersionByTag, CreateTag, UpdateTag, DeleteTag, DeleteRegistryEntry methods
- `api/types.go` — add registry request/response types

</code_context>

<deferred>
## Deferred Ideas

- Cross-project registry sharing (REG-F01: `is_public` flag) — future milestone. Consider adding a top-level `/registry/{project}/{name}` alias path when implementing to avoid URL breakage for public entries.
- GA4GH TRS API compatibility (REG-F02) — future milestone

</deferred>

---

*Phase: 21-wdl-registry*
*Context gathered: 2026-03-20*
