---
phase: 32
status: findings
depth: standard
files_reviewed: 6
findings:
  critical: 1
  warning: 4
  info: 2
  total: 7
reviewed: 2026-04-07
---

# Phase 32 Code Review

## Summary

The API foundation code is generally well-structured with good separation of concerns and consistent error handling patterns. One critical issue was found: the server silently continues with a nil JWT secret when secret resolution fails at startup, which allows any JWT token to potentially be accepted. Four warnings cover route shadowing, swallowed internal errors, and minor safety gaps.

## Findings

### CR-01: Silent nil JWT secret on startup allows auth bypass

**Severity:** critical
**File:** internal/server/server.go:59-64
**Category:** security

**Issue:** When `auth.ResolveJWTSecret` returns an error, the server logs the error but assigns `nil` to `s.jwtSecret` and continues. `JWTMiddleware` is then registered with a nil secret. In `go-jose`/`golang-jwt`, calling `jwt.ParseWithClaims` with a nil key causes the key function to return `nil`, which the HMAC signing method accepts as a zero-length key — meaning any token signed with an empty secret passes validation. All JWT-protected routes are therefore effectively unprotected.

```go
// Current — dangerous fallthrough
secret, err := auth.ResolveJWTSecret(context.Background(), cfg.Auth.JWTSecret, st)
if err != nil {
    log.Printf("server: resolve jwt secret: %v", err)
}
s.jwtSecret = secret  // nil on error
```

**Suggested fix:** Treat a secret resolution failure as a fatal startup error. Either `log.Fatal` or return an error from `New`:

```go
secret, err := auth.ResolveJWTSecret(context.Background(), cfg.Auth.JWTSecret, st)
if err != nil {
    log.Fatalf("server: resolve jwt secret: %v", err)
}
s.jwtSecret = secret
```

Alternatively, add a guard in `JWTMiddleware` to reject all requests when the secret is nil or empty.

---

### WR-01: Daemon route POST /tasks/run shadows the user-authenticated project route

**Severity:** warning
**File:** internal/server/server.go:163-194
**Category:** bug

**Issue:** In the auth-enabled routing block, `POST /projects/{slug}/tasks/run` (line 164) is registered under the JWT project group, and separately `POST /tasks/run` (line 194) is registered under the daemon group. However, a flat `POST /tasks/run` is also registered at the top level inside the daemon group. Chi resolves the most specific match first, so the daemon route is correct at the top level. The project-scoped `/projects/{slug}/tasks/run` route is distinct and should not conflict. However, the flat `POST /tasks/run` at line 194 duplicates the already-registered daemon route. If chi registers conflicting routes (two groups both registering `POST /tasks/run` without a common prefix), a panic or silent route shadowing can occur. This should be verified against the chi version in use and a test added for the daemon `POST /tasks/run` path specifically.

**Suggested fix:** Confirm there is no overlap and add an integration test that POSTs to the bare `/tasks/run` with a daemon token to ensure the daemon middleware fires, not the JWT one.

---

### WR-02: handleRegistryDelete conflates internal errors with not-found

**Severity:** warning
**File:** internal/server/handlers_registry.go:236-239
**Category:** error handling

**Issue:** Any error returned by `s.store.DeleteRegistryEntry` — including transient database errors — is mapped unconditionally to HTTP 404. A database failure will look like "entry not found" to the caller, hiding the true cause and making server-side failures undiagnosable from the client or in logs.

```go
if err := s.store.DeleteRegistryEntry(r.Context(), proj.ID, name); err != nil {
    writeError(w, http.StatusNotFound, fmt.Sprintf("entry %q not found", name))
    return
}
```

**Suggested fix:** Distinguish not-found from other errors using a sentinel error or error type inspection:

```go
if err := s.store.DeleteRegistryEntry(r.Context(), proj.ID, name); err != nil {
    if isNotFound(err) {
        writeError(w, http.StatusNotFound, fmt.Sprintf("entry %q not found", name))
    } else {
        writeError(w, http.StatusInternalServerError, "failed to delete registry entry")
    }
    return
}
```

The same pattern applies to `handleRegistryListVersions` (line 204) which also maps all store errors to 404.

---

### WR-03: handleRegistryPush error discrimination relies on string matching

**Severity:** warning
**File:** internal/server/handlers_registry.go:87-90
**Category:** bug

**Issue:** The conflict check for kind mismatch uses `strings.Contains(err.Error(), "kind")` to decide whether to return 409. This is fragile: any error whose message happens to contain the substring "kind" (e.g., a DB driver error mentioning "what kind of...") will be incorrectly mapped to 409 Conflict, and the converse — a genuine kind mismatch with a different error message phrasing — will silently fall through to 500.

```go
if strings.Contains(err.Error(), "kind") {
    writeError(w, http.StatusConflict, err.Error())
    return
}
```

**Suggested fix:** Define a sentinel error type in the store package (e.g., `store.ErrKindMismatch`) and use `errors.Is` or `errors.As` for discrimination.

---

### WR-04: handleUpload stores files using proj.Slug in path without validating it is safe for filesystem use

**Severity:** warning
**File:** internal/server/handlers_uploads.go:43
**Category:** security

**Issue:** `proj.Slug` is taken directly from the database and interpolated into a filesystem path via `filepath.Join`. While `validSlug` (lowercase alphanumeric + hyphens) is enforced at project creation time, there is no re-validation at upload time. If a slug somehow contained a path separator (e.g., through a direct DB write or a future code path that bypasses validation), the resulting path could escape `BaseDir`. The `filepath.Base` call on `header.Filename` protects the filename component but `proj.Slug` is not sanitised here.

```go
storagePath := filepath.Join(s.cfg.BaseDir, "projects", proj.Slug, "uploads", uploadID, filename)
```

**Suggested fix:** Apply `filepath.Base` or re-run `validSlug` on `proj.Slug` before using it in path construction, or use `filepath.Clean` and verify the result has the expected prefix:

```go
storagePath := filepath.Join(s.cfg.BaseDir, "projects", proj.Slug, "uploads", uploadID, filename)
if !strings.HasPrefix(storagePath, filepath.Clean(s.cfg.BaseDir)+string(filepath.Separator)) {
    writeError(w, http.StatusBadRequest, "invalid upload path")
    return
}
```

---

### IR-01: handleRegistryInputs only exposes inputs reachable via a tag; digest lookup is not supported

**Severity:** info
**File:** internal/server/handlers_registry.go:332-336
**Category:** quality

**Issue:** The `/inputs` endpoint requires `?tag=` and has no `?digest=` alternative, unlike `handleRegistryPull` which supports both. This inconsistency means a client that resolves a version by digest cannot introspect its inputs without first knowing a tag. This is a design limitation worth documenting or aligning.

**Suggested fix:** Add optional `?digest=` support mirroring `handleRegistryPull`, or add a comment explaining why tag-only is intentional.

---

### IR-02: writeJSON silently discards JSON encoding errors

**Severity:** info
**File:** internal/server/server.go:267-271
**Category:** quality

**Issue:** `writeJSON` calls `writeJSONBody` and ignores its return value with `_ =`. By this point `w.WriteHeader` has already been called, so no status change is possible. However, a silent failure means the client receives a truncated or empty body with a 200/201/etc. status and no indication of the problem. The server also has no visibility into serialisation failures.

```go
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    _ = writeJSONBody(w, v)  // error silently dropped
}
```

**Suggested fix:** Log encoding errors to aid server-side diagnostics:

```go
if err := writeJSONBody(w, v); err != nil {
    log.Printf("writeJSON: encode response: %v", err)
}
```
