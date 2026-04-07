---
phase: 33
status: all_fixed
fix_scope: critical_warning
findings_in_scope: 7
fixed: 7
skipped: 0
iteration: 1
---
# Phase 33 Code Review Fix Report

## Fixes Applied

### C-1: URL path segments constructed from unescaped user input (SSRF / path traversal)
**Status:** fixed
**Commit:** 2c5d4c4
**Reason:** Wrapped every user-supplied path segment (`projectSlug`, `name`, `tag`, `runID`) with `url.PathEscape()` across all methods in `internal/client/client.go`. Also renamed shadowing local `url` variables to `reqURL` to avoid collision with the `net/url` package identifier.

### C-2: Credentials file permissions not enforced on overwrite
**Status:** fixed
**Commit:** f844c8f
**Reason:** Added `os.Chmod(path, 0600)` immediately after `os.WriteFile` in `SaveCredentials`. The chmod is unconditional so existing files with broader permissions are corrected on every save, not just on first creation.

### W-1: Password bytes never zeroed after use
**Status:** fixed
**Commit:** c130697
**Reason:** Added a zeroing loop (`for i := range passwordBytes { passwordBytes[i] = 0 }`) in `runLogin` immediately after `password := string(passwordBytes)`, before the byte slice goes out of scope.

### W-2: `bufio.Scanner` error not checked after reading username
**Status:** fixed
**Commit:** 6ecb1af
**Reason:** Replaced bare `scanner.Scan()` call with a guard that checks the bool return. On `false`, it consults `scanner.Err()` — a non-nil error is printed with its detail; a nil error (EOF) prints a clear "unexpected EOF" message. Both paths return exit code 1.

### W-3: Error response body silently ignored; produces empty error messages
**Status:** fixed
**Commit:** 2c5d4c4
**Reason:** Introduced a `readErrorMessage(body io.Reader)` helper in `internal/client/client.go` that reads up to 4096 bytes, attempts JSON decode into `api.ErrorResponse`, and falls back to a raw body snippet (up to 256 bytes) when the decode fails or `Error` is empty. All ~20 error branches now call this helper instead of discarding the decode error with `_`.

### W-4: `tabwriter.Flush` error silently ignored
**Status:** fixed
**Commit:** dd61adc
**Reason:** Changed `w.Flush()` to `if err := w.Flush(); err != nil { ... return 1 }` in `runRegistryList` in `cmd/flowy/registry.go`.

### W-5: Empty credentials not validated after load
**Status:** fixed
**Commit:** 304161c
**Reason:** Added post-unmarshal validation in `LoadCredentials` in `internal/client/credentials.go`. Returns a descriptive error directing the user to `flowy login` when `Server` or `Token` is the zero string.

## Test Results

```
ok  	github.com/bioflowy/flowy/internal/client	0.026s
ok  	github.com/bioflowy/flowy/cmd/flowy	2.049s
```
