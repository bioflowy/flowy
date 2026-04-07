---
phase: 33
status: findings
depth: standard
files_reviewed: 5
findings:
  critical: 2
  warning: 5
  info: 2
  total: 9
reviewed: 2026-04-07
---

# Phase 33 Code Review — CLI Auth and Registry Commands

## Files Reviewed

- `internal/client/credentials.go`
- `internal/client/client.go`
- `cmd/flowy/login.go`
- `cmd/flowy/registry.go`
- `cmd/flowy/main.go`

---

## Critical Findings

### C-1: URL path segments constructed from unescaped user input (SSRF / path traversal)
**File:** `internal/client/client.go` — all registry methods, `SubmitToProject`, `TaskRunInProject`, `StatusInProject`, `OutputsInProject`

URL paths are built by direct string concatenation of user-supplied values such as `projectSlug`, `name`, `tag`, and `runID`:

```go
// client.go line 404
reqURL := c.serverURL + "/projects/" + projectSlug + "/registry/" + name
// client.go line 478
reqURL := c.serverURL + "/projects/" + projectSlug + "/registry/" + name + "/tags/" + tag
```

A value like `name = "../../admin"` or `projectSlug = "slug/../../../other"` will silently traverse path boundaries on the server. A tag value of `"foo?injected=true"` would corrupt the query string of `RegistryDeleteTag`. Note that `RegistryPull` already uses `url.QueryEscape` for the query parameters (correct) but the path segments `projectSlug` and `name` are still unescaped.

**Fix:** Wrap every path segment with `url.PathEscape(...)` before interpolation, or build URLs with `url.URL` and `path.Join`.

---

### C-2: Credentials file permissions not enforced on overwrite
**File:** `internal/client/credentials.go` — `SaveCredentials` (line 71)

```go
if err := os.WriteFile(path, data, 0600); err != nil {
```

`os.WriteFile` passes the permission bits to `os.OpenFile` as the `perm` argument, which is only applied when the file is **created for the first time**. If `~/.flowy/credentials.yaml` already exists with broader permissions (e.g., `0644` from a previous miscreated file or a test), `WriteFile` will truncate and overwrite the content but leave the existing file mode intact. A subsequent `ls -l` would show `0644`, making the token readable by group and others.

**Fix:** After writing, call `os.Chmod(path, 0600)` unconditionally, or open the file with `os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)` followed by an explicit `os.Chmod` to handle the race between create and chmod (or accept this as a TOCTOU limitation and document it).

---

## Warning Findings

### W-1: Password bytes never zeroed after use
**File:** `cmd/flowy/login.go` — `runLogin` (lines 51–57)

```go
passwordBytes, err := readPasswordFn()
// ...
password := string(passwordBytes)
```

The `[]byte` slice returned by `term.ReadPassword` contains the plaintext password. It is immediately converted to a `string` (which copies the bytes) but the original slice is never zeroed before becoming eligible for GC. The password value will remain in memory until the GC happens to overwrite that region, and may appear in heap dumps or core dumps.

**Fix:** After `password := string(passwordBytes)`, zero the slice: `for i := range passwordBytes { passwordBytes[i] = 0 }`.

---

### W-2: `bufio.Scanner` error not checked after reading username
**File:** `cmd/flowy/login.go` — `runLogin` (lines 45–47)

```go
scanner := bufio.NewScanner(stdin)
scanner.Scan()
username := strings.TrimSpace(scanner.Text())
```

`scanner.Scan()` returns `false` both on EOF and on error. `scanner.Err()` is never consulted. If stdin is closed or a read error occurs, `username` will be an empty string and the login attempt will proceed silently with an empty username, yielding a confusing server error rather than a clear "failed to read username" message.

**Fix:** Check the return value and `scanner.Err()`:
```go
if !scanner.Scan() {
    if err := scanner.Err(); err != nil {
        fmt.Fprintf(stderr, "error: reading username: %v\n", err)
    } else {
        fmt.Fprintln(stderr, "error: unexpected EOF reading username")
    }
    return 1
}
```

---

### W-3: Error response body silently ignored; produces empty error messages
**File:** `internal/client/client.go` — multiple locations (e.g., lines 84–85, 117–118, 152–153, etc.)

```go
var errResp api.ErrorResponse
_ = json.NewDecoder(resp.Body).Decode(&errResp)
return "", fmt.Errorf("login failed: %s", errResp.Error)
```

The decode error is explicitly discarded with `_`. When the server returns a non-JSON body (e.g., an HTML error page from a proxy, or a network appliance error), `errResp.Error` remains `""`, and the message shown to the user becomes `"login failed: "` or `"client: registry push failed (status 502): "` — entirely unhelpful.

This pattern repeats across every error branch in the file (roughly 20 occurrences).

**Fix:** Include the HTTP status code (already present in some messages) and, when `errResp.Error` is empty, include the raw body up to a reasonable limit (e.g., 256 bytes):
```go
var errResp api.ErrorResponse
if decErr := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&errResp); decErr != nil || errResp.Error == "" {
    // Fall back to raw body snippet
}
```

---

### W-4: `tabwriter.Flush` error silently ignored
**File:** `cmd/flowy/registry.go` — `runRegistryList` (line 161)

```go
w.Flush()
return 0
```

`tabwriter.Writer.Flush()` returns an error, which is discarded here. If the underlying `stdout` writer fails (e.g., broken pipe from `flowy registry list | head -1`), the error is swallowed and the command exits 0.

**Fix:**
```go
if err := w.Flush(); err != nil {
    fmt.Fprintf(stderr, "error: writing output: %v\n", err)
    return 1
}
```

---

### W-5: Empty credentials not validated after load
**File:** `internal/client/credentials.go` — `LoadCredentials` (lines 45–49)

```go
var creds Credentials
if err := yaml.Unmarshal(data, &creds); err != nil { ... }
return &creds, nil
```

A YAML file that parses successfully but contains empty `server` or `token` fields (e.g., the file was partially written, or manually edited incorrectly) will return a `Credentials` with zero-value strings. The caller in `registry.go` (`loadCredsAndClient`) will then construct a `Client` with `serverURL = ""` and `token = ""`, and every subsequent HTTP request will fail with a confusing `"unsupported protocol scheme \"\""` error rather than a clear "credentials file is malformed" message.

**Fix:** Add post-unmarshal validation in `LoadCredentials`:
```go
if creds.Server == "" {
    return nil, fmt.Errorf("credentials: server URL is empty in credentials file — run `flowy login`")
}
if creds.Token == "" {
    return nil, fmt.Errorf("credentials: token is empty in credentials file — run `flowy login`")
}
```

---

## Info Findings

### I-1: `isRegistryRef` heuristic can misclassify unusual Linux paths
**File:** `cmd/flowy/main.go` — `isRegistryRef` (lines 695–708)

Any file path containing `:` that is not a single-letter Windows drive (e.g., `/mnt/data/my:file.wdl`) will be detected as a registry reference, causing the run to fail with a confusing "registry reference requires --project flag" error instead of a file-not-found error. This is an edge case (colons are rare in file paths on Linux but valid) and the detection is intentional, but the error message does not hint that the path was misinterpreted as a registry ref.

**Suggestion:** When the argument is treated as a registry ref and the file also exists on disk, or when the registry ref lookup fails, emit a note like `"(if you meant a file path, avoid ':' in file names)"`.

---

### I-2: `reorderArgs` does not handle `--flag=value` form
**File:** `cmd/flowy/main.go` — `reorderArgs` (lines 719–750)

`reorderArgs` identifies value-consuming flags by name and advances `i` to consume the next token. However, the `--flag=value` form (a single token like `--project=my-proj`) is not handled: it is treated as a no-value flag and placed in `flags`, which is correct for parsing, but if it appears after a positional argument it will still be moved before it — this part is fine. The real gap is that `--flag=value` where `flag` is not in `knownValueFlags` will also be moved correctly as a boolean flag. No incorrect behavior results, but the logic is implicitly fragile: adding a new value-accepting flag requires updating `knownValueFlags` or the flag will be treated as boolean and the value token will be treated as a positional argument.

**Suggestion:** Document this invariant prominently in the function comment, or use a different reorder strategy (scan for `=`-containing tokens before fallback).

---

## Summary

| ID  | Severity | Location | Issue |
|-----|----------|----------|-------|
| C-1 | Critical | `client.go` (all registry/project methods) | Unescaped user values interpolated into URL path segments — path traversal |
| C-2 | Critical | `credentials.go:SaveCredentials` | `os.WriteFile` mode `0600` not applied on overwrite — token may stay world-readable |
| W-1 | Warning  | `login.go:runLogin` | Password byte slice not zeroed after use |
| W-2 | Warning  | `login.go:runLogin` | `scanner.Err()` not checked; silent empty username on read error |
| W-3 | Warning  | `client.go` (all error branches) | Silently discarded decode errors produce empty error messages |
| W-4 | Warning  | `registry.go:runRegistryList` | `tabwriter.Flush` error silently ignored |
| W-5 | Warning  | `credentials.go:LoadCredentials` | Empty server/token fields not validated after unmarshal |
| I-1 | Info     | `main.go:isRegistryRef` | Colon in Linux path silently misclassified as registry ref |
| I-2 | Info     | `main.go:reorderArgs` | `--flag=value` form undocumented; adding flags requires manual `knownValueFlags` update |
