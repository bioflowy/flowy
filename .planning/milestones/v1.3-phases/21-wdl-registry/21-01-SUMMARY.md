---
phase: 21-wdl-registry
plan: 01
subsystem: database
tags: [sqlite, goose, registry, wdl, content-addressed]

# Dependency graph
requires:
  - phase: 20-projects-and-rbac
    provides: projects table and Store interface patterns used as FK target and code model
provides:
  - SQLite migration 00006 with three-table registry schema (registry_entries, registry_versions, registry_tags)
  - Five exported model types in store.go (RegistryEntry, RegistryVersion, RegistryTag, RegistryEntryListItem, RegistryVersionListItem)
  - Nine Store interface methods for registry operations
  - Full SQLiteStore implementation of all nine registry methods
affects: [21-02, 21-03, any phase that depends on registry persistence]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Content-addressed immutable versions: registry_versions stores WDL blobs with sha256 digest as identity"
    - "Mutable tag pointers: registry_tags provides human-readable aliases to specific versions"
    - "Soft-delete for entries: is_active=0 preserves data while hiding from active queries"
    - "Writer-for-TOCTOU: PushRegistryVersion uses writer connection for both SELECT and INSERT to avoid races"
    - "Idempotent push: digest uniqueness constraint (entry_id, digest) ensures same content returns existing version"

key-files:
  created:
    - internal/store/sqlite/migrations/00006_registry.sql
    - internal/store/sqlite/registry.go
  modified:
    - internal/store/store.go

key-decisions:
  - "Kind is immutable after first push: re-push with different kind returns error (caller maps to 409)"
  - "PushRegistryVersion uses writer for both SELECT and INSERT to avoid TOCTOU races"
  - "N+1 query pattern for ListRegistryEntries tag fetch (acceptable at small scale, TODO comment added)"
  - "scanVersionWithEntry accepts *sql.Row directly (not interface) for the two-table JOIN queries"

patterns-established:
  - "scanRegistryEntry/scanRegistryVersion helper functions follow same interface-based scanner pattern as scanProject"
  - "All mutations use s.writer; all pure reads use s.reader"
  - "Timestamps stored as RFC3339 strings, parsed via shared parseTime() helper"

requirements-completed: [REG-01, REG-03, REG-04, REG-05, REG-06]

# Metrics
duration: 15min
completed: 2026-03-20
---

# Phase 21 Plan 01: Registry Store Foundation Summary

**Three-table SQLite registry schema with content-addressed WDL blobs, idempotent push, and full SQLiteStore implementation of 9 interface methods**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-03-20T01:40:00Z
- **Completed:** 2026-03-20T01:55:00Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Created migration 00006 with registry_entries, registry_versions, and registry_tags tables including indexes and UNIQUE constraints
- Added five model types and nine Store interface method declarations to store.go
- Implemented all nine SQLiteStore registry methods in registry.go with correct writer/reader pool usage
- All existing tests (22 packages) continue to pass unchanged

## Task Commits

Each task was committed atomically:

1. **Task 1: Create registry migration and model types** - `27980a7` (feat)
2. **Task 2: Implement SQLiteStore registry methods** - `4ad7904` (feat)

**Plan metadata:** (docs commit follows this summary)

## Files Created/Modified
- `internal/store/sqlite/migrations/00006_registry.sql` - Three-table registry schema with indexes and UNIQUE constraints
- `internal/store/store.go` - Added five registry model types and nine Store interface methods
- `internal/store/sqlite/registry.go` - Full SQLiteStore implementation of all nine registry methods

## Decisions Made
- `PushRegistryVersion` uses the writer connection for both the entry SELECT and version SELECT (not just INSERTs) to avoid TOCTOU races under concurrent pushes.
- Kind is immutable after first push. Re-push with a different kind returns `fmt.Errorf("sqlite: entry %q is kind %q; cannot push kind %q", ...)` — the HTTP handler layer will map this to 409 Conflict.
- `scanVersionWithEntry` takes `*sql.Row` directly rather than an interface because the JOIN query returns columns from two tables and the pattern is only used in the two Get-by-tag/digest methods.
- N+1 query in `ListRegistryEntries` for tag names is intentional at this scale; a TODO comment is in place for optimization if needed.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None. The compile-time assertion `var _ store.Store = (*SQLiteStore)(nil)` in sqlite.go correctly flagged missing methods until Task 2 was complete — this was expected behavior.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Registry persistence layer complete; Plans 02 and 03 can now implement HTTP handlers and routes on top of these store methods.
- No blockers. Migration will auto-apply via goose on next server start.

---
*Phase: 21-wdl-registry*
*Completed: 2026-03-20*
