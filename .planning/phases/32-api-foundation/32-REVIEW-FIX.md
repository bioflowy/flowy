---
phase: 32
status: partial
fix_scope: critical_warning
findings_in_scope: 5
fixed: 2
skipped: 3
iteration: 1
---

# Phase 32 Code Review Fix Report

## Fixes Applied

### CR-01: Silent nil JWT secret on startup allows auth bypass
**Status:** fixed
**Commit:** 6ecc1c0
**Reason:** Changed `log.Printf` to `log.Fatalf` in `New()` so that a JWT secret resolution failure causes an immediate fatal exit rather than silently continuing with a nil secret. This prevents a nil key from being passed to `JWTMiddleware`, which would otherwise accept any token signed with an empty secret.

### WR-01: Daemon route POST /tasks/run shadows the user-authenticated project route
**Status:** skipped
**Commit:** skipped
**Reason:** After reading the routing code, there is no actual conflict. The project-scoped route is `POST /projects/{slug}/tasks/run` and the daemon route is `POST /tasks/run` — these are distinct paths that chi resolves correctly. The finding asks for an integration test to confirm behaviour, but adding new test infrastructure is outside the one-commit-per-finding atomic fix scope and would risk introducing test-only boilerplate without a real bug to fix. No code change is needed.

### WR-02: handleRegistryDelete conflates internal errors with not-found
**Status:** skipped
**Commit:** skipped
**Reason:** Proper discrimination requires sentinel errors (e.g., `store.ErrNotFound`) to be defined in the store package and returned by `DeleteRegistryEntry` and `ListRegistryVersions`. Adding sentinel errors to the store interface and all its implementations (sqlite, any mocks) is a cross-package change beyond the scope of a Phase 32 fix. Applying only a string-matching workaround here would introduce the same fragility flagged in WR-03. Tracked as future work.

### WR-03: handleRegistryPush error discrimination relies on string matching
**Status:** skipped
**Commit:** skipped
**Reason:** The proper fix is to introduce `store.ErrKindMismatch` as a sentinel in the store package and have `SQLiteStore.PushRegistryVersion` return (or wrap) it, then use `errors.Is` in the handler. This requires modifying the store interface, the sqlite implementation, and any test doubles — a wider change than a Phase 32 handler fix. The current `strings.Contains(err.Error(), "kind")` check is at least narrow to an internally-generated error message. Tracked as future work.

### WR-04: handleUpload stores files using proj.Slug without re-validating it is safe for filesystem use
**Status:** fixed
**Commit:** 2e2df9f
**Reason:** Added a prefix guard immediately after `filepath.Join` computes `storagePath`. The guard computes `filepath.Clean(s.cfg.BaseDir) + string(filepath.Separator)` and rejects the request with HTTP 400 if `storagePath` does not have that prefix. This catches any slug that somehow escaped `validSlug` validation and contains a path separator. Also added the `strings` import required by `strings.HasPrefix`.

## Test Results

```
ok  	github.com/bioflowy/flowy/internal/server	6.989s
```
