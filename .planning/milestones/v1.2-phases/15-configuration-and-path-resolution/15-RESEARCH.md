# Phase 15: Configuration and Path Resolution - Research

**Researched:** 2026-03-13
**Domain:** Go config/YAML, CLI flag patterns, path resolution, API types extension
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**YAML config structure**
- Nested `storage:` top-level section with sub-keys: type, bucket, endpoint, region, prefix, multipart_threshold
- Consistent with existing server/daemon/client sections in Config struct
- Default storage type is "local" when no storage section exists — zero-config for existing users
- Validate at startup: if type is "s3", require bucket to be non-empty at config load time — fail fast
- Prefix is optional, defaults to empty string (keys go to bucket root). Users set prefix to namespace multiple flowy instances

**CLI flag behavior**
- `--storage` flag overrides only storage.type from YAML — all other S3 settings (bucket, endpoint, region) come from YAML
- Error before submit if `--storage s3` is used but S3 config (bucket) missing from YAML — actionable error message
- Only `flowy run` and `flowy submit` accept `--storage` — other commands don't create runs
- Validate against known values whitelist: "local" and "s3" — error on unknown values with available list

**Path resolution strategy**
- Guard-and-passthrough: if strings.HasPrefix(p, "s3://") { return p } — no filepath.Abs, no filepath.Clean on S3 URIs
- Deduplicate the two identical cleanPath() functions (wdl/task/inputs.go and wdl/workflow/convert.go) into a shared package (e.g. wdl/pathutil)
- Per-input scheme detection: s3:// → S3 backend, /abs/path → local filesystem, relative path → configured default backend
- Guard s3:// only — no future-proofing for gs://, az:// until those backends exist (YAGNI)

**TaskClaimResponse extensions**
- Four new fields: StorageType, StorageBucket, StorageEndpoint, StoragePrefix
- StorageType always populated ("local" or "s3") — daemon has a consistent contract
- S3 fields (bucket, endpoint, prefix) are empty strings for local backend
- InputsDir/OutputsDir reused for S3 key prefixes when storage_type is "s3" — daemon uses StorageType to interpret them
- UsePathStyle (for MinIO) is daemon-side config, NOT in TaskClaimResponse — keeps response focused on addressing

### Claude's Discretion
- Exact StorageConfig struct field names and YAML tag formatting
- Internal organization of config validation logic
- Shared pathutil package naming and location
- How server populates TaskClaimResponse storage fields (inline vs helper function)
- Test structure for new config parsing and path resolution

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| PATH-01 | cleanPath() in wdl/task/inputs.go preserves s3:// URIs without corruption | Guard-and-passthrough pattern; strings.HasPrefix check before filepath.Abs/Clean; shared pathutil package deduplication |
| PATH-02 | Input files with s3:// prefix are resolved via S3 backend regardless of default backend setting | Per-input scheme detection logic in cleanPath; PassThrough for s3:// bypasses all local path transforms |
| PATH-03 | Input files with absolute local paths (/mnt/...) are resolved via local filesystem | Existing filepath.IsAbs branch already handles this; no change needed for absolute paths |
| PATH-04 | Input files with relative paths are resolved via the configured default backend | Relative path handling in shared cleanPath continues to use filepath.Join(wdlDir, p) |
| CONF-01 | Storage backend type and S3 settings (bucket, endpoint, region) configurable in YAML config file | StorageConfig struct with yaml tags, nested in Config; applyDefaults sets type="local" when empty |
| CONF-02 | Default storage backend specified in config file (used when --storage flag is not provided) | Config.Storage.Type is the default; --storage flag overrides only type at runtime |
| CONF-03 | User can specify storage backend per workflow via --storage CLI flag on flowy run/submit | Add storageFlag to runRun() and runSubmit() FlagSets; validate whitelist ["local","s3"] |
| CONF-04 | TaskClaimResponse extended with storage addressing config (bucket, endpoint, prefix) for daemon | Add StorageType, StorageBucket, StorageEndpoint, StoragePrefix string fields to api.TaskClaimResponse; populate in handleClaim() |
</phase_requirements>

## Summary

Phase 15 implements the configuration and path wiring needed before S3 I/O can occur in Phase 16. The work spans four distinct areas: (1) extending the YAML config with a `StorageConfig` struct, (2) adding a `--storage` CLI flag to `flowy run` and `flowy submit`, (3) guarding `cleanPath()` against s3:// URI corruption and deduplicating it into a shared package, and (4) extending `TaskClaimResponse` with four storage addressing fields so the daemon knows where to store task data.

All four areas touch small, well-understood files. The patterns are already established by the existing `ServerConfig`, `DaemonConfig`, and `ClientConfig` structs. The path guard is a one-line prefix check. The `TaskClaimResponse` extension is additive with no breaking changes (JSON omitempty is NOT appropriate for StorageType since it must always be populated). The `--storage` flag follows the existing `--server` flag pattern exactly.

The biggest risk is accidentally breaking the 258 existing tests (the "156 spectool tests" mentioned in requirements refer to the spectool conformance tests run via `go test ./...`). Since all changes are additive — new struct fields, new flag, new package, and guarded passthrough in path logic — the risk is low.

**Primary recommendation:** Implement changes in dependency order: pathutil package first (no dependencies), then StorageConfig (standalone), then --storage flag (reads config), then TaskClaimResponse extension (reads config in handler). Each step is independently testable.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `gopkg.in/yaml.v3` | v3 (already in go.mod) | YAML struct deserialization | Already used in config.go; no new dependency |
| `flag` (stdlib) | go1.25 | CLI flag parsing | Project already uses stdlib flag exclusively |
| `strings` (stdlib) | go1.25 | HasPrefix check for s3:// guard | Zero overhead, already imported in backend.go |
| `path/filepath` (stdlib) | go1.25 | Existing local path resolution | Already used in both cleanPath() functions |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `fmt` (stdlib) | go1.25 | Error message formatting | Validation errors in config and CLI |
| `encoding/json` (stdlib) | go1.25 | JSON tags on TaskClaimResponse | Already used in api/types.go |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| stdlib `flag` | `cobra`, `urfave/cli` | Project decision: stdlib flag only; no new dependencies |
| `wdl/pathutil` package | inline guard in each file | Deduplication is locked decision; shared package prevents future drift |
| Validate in `Load()` | Validate in server startup | Load-time validation gives earlier failure; consistent with project's applyDefaults pattern |

**Installation:** No new dependencies required.

## Architecture Patterns

### Recommended Project Structure
```
internal/config/
├── config.go            # Add StorageConfig struct + applyDefaults + Validate()
└── config_test.go       # Add storage config test cases

wdl/pathutil/
├── pathutil.go          # New shared CleanPath() function
└── pathutil_test.go     # New unit tests for s3:// guard

wdl/task/
└── inputs.go            # Replace local cleanPath() call with pathutil.CleanPath()

wdl/workflow/
└── convert.go           # Replace local cleanPath() call with pathutil.CleanPath()

internal/api/
└── types.go             # Add 4 fields to TaskClaimResponse

internal/server/
└── handlers.go          # Populate storage fields in handleClaim()

cmd/flowy/
└── main.go              # Add --storage flag to runRun() and runSubmit()
```

### Pattern 1: StorageConfig Struct (mirrors existing pattern)

**What:** New nested struct in Config with yaml tags, appended to applyDefaults().
**When to use:** Whenever adding a new config section — follow existing ServerConfig/DaemonConfig shape.

```go
// Source: internal/config/config.go (established pattern)

// StorageConfig holds configuration for the storage backend.
type StorageConfig struct {
    // Type selects the storage backend: "local" (default) or "s3".
    Type string `yaml:"type"`

    // Bucket is the S3 bucket name. Required when Type is "s3".
    Bucket string `yaml:"bucket"`

    // Endpoint overrides the S3 endpoint URL (for MinIO or other S3-compatible stores).
    Endpoint string `yaml:"endpoint"`

    // Region is the AWS region for S3 operations.
    Region string `yaml:"region"`

    // Prefix is an optional key prefix applied to all S3 keys.
    Prefix string `yaml:"prefix"`

    // MultipartThreshold is the file size in bytes above which multipart upload is used.
    // Defaults to 100 MiB (104857600). Used by Phase 16 S3Backend.
    MultipartThreshold int64 `yaml:"multipart_threshold"`
}
```

The Config struct gains one field:
```go
type Config struct {
    // ... existing fields ...
    Storage StorageConfig `yaml:"storage"`
}
```

applyDefaults() addition:
```go
if cfg.Storage.Type == "" {
    cfg.Storage.Type = "local"
}
if cfg.Storage.MultipartThreshold == 0 {
    cfg.Storage.MultipartThreshold = 100 * 1024 * 1024 // 100 MiB
}
```

### Pattern 2: Config Validation (fail-fast at load time)

**What:** A `Validate()` function called after `Load()` that returns an actionable error.
**When to use:** Whenever cross-field or conditional validation is needed that can't be expressed as a default.

```go
// Source: established pattern in config.go

// Validate checks cross-field constraints that cannot be expressed as defaults.
// Returns an error describing the first invalid condition found.
func Validate(cfg *Config) error {
    switch cfg.Storage.Type {
    case "local", "s3":
        // valid
    default:
        return fmt.Errorf("config: storage.type %q is not supported (valid values: local, s3)", cfg.Storage.Type)
    }
    if cfg.Storage.Type == "s3" && cfg.Storage.Bucket == "" {
        return fmt.Errorf("config: storage.type is \"s3\" but storage.bucket is not set")
    }
    return nil
}
```

The server's main.go calls `config.Validate(cfg)` immediately after `config.Load()`.

### Pattern 3: Shared pathutil.CleanPath() with s3:// guard

**What:** New package `wdl/pathutil` exporting `CleanPath(p, wdlDir string) string`.
**When to use:** Any path resolution of WDL File/Directory inputs.

```go
// Package pathutil provides path resolution helpers for WDL input files.
// It is shared by wdl/task and wdl/workflow to ensure consistent behavior.
package pathutil

import (
    "path/filepath"
    "strings"
)

// CleanPath resolves a WDL input path to a canonical form.
// S3 URIs (s3:// prefix) are returned unchanged — filepath.Abs and
// filepath.Clean must not be applied to object storage URIs.
// For local paths: relative paths are resolved against wdlDir (or CWD if empty),
// absolute paths are cleaned in place.
func CleanPath(p string, wdlDir string) string {
    // Guard: s3:// URIs must pass through without modification.
    if strings.HasPrefix(p, "s3://") {
        return p
    }
    if !filepath.IsAbs(p) {
        if wdlDir != "" {
            return filepath.Clean(filepath.Join(wdlDir, p))
        }
        if abs, err := filepath.Abs(p); err == nil {
            return filepath.Clean(abs)
        }
    }
    return filepath.Clean(p)
}
```

Both `wdl/task/inputs.go` and `wdl/workflow/convert.go` replace their local `cleanPath()` with `pathutil.CleanPath()`. The local function definitions are removed entirely.

### Pattern 4: --storage CLI flag (mirrors --server flag)

**What:** String flag added to `runRun()` and `runSubmit()` FlagSets; validated before submit.
**When to use:** Follows `resolveServerURL()` pattern exactly.

```go
// In runRun() and runSubmit():
storageFlag := fs.String("storage", "", "storage backend override: local or s3")

// After flag parsing, before submit:
effectiveStorageType := resolveStorageType(*storageFlag, cfg)
if err := validateStorageType(effectiveStorageType); err != nil {
    fmt.Fprintln(stderr, "error:", err)
    return 1
}
// If s3 is requested but bucket is missing from config, fail fast:
if effectiveStorageType == "s3" && cfg.Storage.Bucket == "" {
    fmt.Fprintln(stderr, "error: --storage s3 requires storage.bucket to be set in config")
    return 1
}
```

Helper functions (consistent with resolveServerURL pattern):
```go
func resolveStorageType(flagValue string, cfg *config.Config) string {
    if flagValue != "" {
        return flagValue
    }
    return cfg.Storage.Type
}

func validateStorageType(t string) error {
    switch t {
    case "local", "s3":
        return nil
    default:
        return fmt.Errorf("unknown storage backend %q (valid values: local, s3)", t)
    }
}
```

Note: For Phase 15, the resolved `effectiveStorageType` does not yet need to be sent to the server — the server always reads its own config. The flag validates the user's intent and provides early error messaging. Sending it in SubmitRequest is optional scope for Phase 15.

### Pattern 5: TaskClaimResponse storage fields

**What:** Four new string fields appended to `TaskClaimResponse` in `internal/api/types.go`.
**When to use:** Additive JSON extension — old daemons simply ignore unknown fields.

```go
// In api/types.go, TaskClaimResponse additions:

// StorageType is the storage backend type: "local" or "s3".
// Always populated so the daemon has a consistent contract.
StorageType string `json:"storage_type"`

// StorageBucket is the S3 bucket name. Empty string when StorageType is "local".
StorageBucket string `json:"storage_bucket,omitempty"`

// StorageEndpoint is the custom S3 endpoint URL (for MinIO). Empty for AWS S3.
StorageEndpoint string `json:"storage_endpoint,omitempty"`

// StoragePrefix is the key prefix for all S3 objects in this deployment.
StoragePrefix string `json:"storage_prefix,omitempty"`
```

Note: `StorageType` must NOT use `omitempty` — it must always be present in the JSON response even for local backend (daemon contract requirement from locked decision).

Handler population in `handleClaim()`:
```go
writeJSON(w, http.StatusOK, api.TaskClaimResponse{
    TaskID:         strconv.FormatInt(task.ID, 10),
    RunID:          task.RunID,
    TaskName:       task.TaskName,
    ShardIndex:     task.ShardIndex,
    InputsDir:      taskDir,
    OutputsDir:     taskDir,
    WDLPath:        wdlPath,
    StorageType:    s.cfg.Storage.Type,
    StorageBucket:  s.cfg.Storage.Bucket,
    StorageEndpoint: s.cfg.Storage.Endpoint,
    StoragePrefix:  s.cfg.Storage.Prefix,
})
```

### Pattern 6: NewBackend() switch (enabling factory for Phase 16)

**What:** Extend `storage.NewBackend()` to switch on `cfg.Storage.Type`.
**When to use:** Single place to create backend instances; Phase 16 S3Backend plugs in here.

```go
// internal/storage/factory.go
func NewBackend(cfg *config.Config) Backend {
    switch cfg.Storage.Type {
    case "s3":
        // Phase 16 will implement S3Backend and replace this panic.
        // For Phase 15, s3 type is config-valid but backend construction
        // returns LocalBackend as placeholder (S3 ops not yet implemented).
        return &LocalBackend{}
    default: // "local" and any zero-value
        return &LocalBackend{}
    }
}
```

This pattern is safe for Phase 15 because no S3 I/O is attempted — the daemon receives the storage addressing fields but doesn't act on them until Phase 17.

### Anti-Patterns to Avoid

- **filepath.Clean on s3:// URIs:** The path `s3://bucket/key/../other` would become `s3:/bucket/other` — bucket name corruption. The guard must run BEFORE any filepath call.
- **omitempty on StorageType:** If StorageType is empty string and omitempty is used, local backend runs would not include `storage_type` in JSON, breaking daemon contract.
- **Validating storage type in cleanPath():** cleanPath() is a pure string transform. Storage backend selection belongs in the server config validation, not in path resolution.
- **Calling config.Validate() inside config.Load():** Load() is used by tests and the client CLI which may not have S3 config; Validate() should be called explicitly in server main.go only.
- **Adding --storage to reorderArgs knownValueFlags before adding it to FlagSets:** The reorderArgs map in cmd/flowy/main.go must be updated alongside the flag definitions.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| YAML deserialization | Custom YAML parser | `gopkg.in/yaml.v3` struct tags | Already in use; handles all edge cases |
| S3 URI validation | Custom regex | `storage.ParseS3URI()` (already exists) | Phase 14 implemented this; reuse it |
| Path normalization | Custom string manipulation | `filepath.Clean(filepath.Join(...))` | stdlib handles `..`, `.`, double slashes correctly for local paths |

**Key insight:** This phase is almost entirely about wiring existing pieces together rather than building new functionality. The storage package already has ParseS3URI; the config package already has the applyDefaults/Load pattern; the API package already has the JSON DTO pattern.

## Common Pitfalls

### Pitfall 1: filepath.Clean corrupts s3:// URIs
**What goes wrong:** `filepath.Clean("s3://bucket/key")` returns `s3:/bucket/key` on Linux because it normalizes `//` to `/`.
**Why it happens:** filepath.Clean treats `//` as redundant separator.
**How to avoid:** The s3:// guard must be the FIRST statement in CleanPath, before any filepath call.
**Warning signs:** Test case `cleanPath("s3://bucket/key", "")` returns `s3:/bucket/key`.

### Pitfall 2: reorderArgs doesn't know about --storage
**What goes wrong:** `flowy run workflow.wdl --storage s3` is silently treated as positional arg because reorderArgs doesn't list `--storage` in knownValueFlags.
**Why it happens:** reorderArgs has a hardcoded map of known flags with values.
**How to avoid:** Add `"--storage": true, "-storage": true` to knownValueFlags in reorderArgs.
**Warning signs:** `flowy run file.wdl --storage s3` gives "run requires exactly one WDL file argument".

### Pitfall 3: Config.Storage.Type is empty string after Load() without storage section
**What goes wrong:** `NewBackend(cfg)` switch has no `case ""` and no `default` — panics.
**Why it happens:** Empty YAML section leaves struct fields at zero values.
**How to avoid:** applyDefaults() sets `cfg.Storage.Type = "local"` when empty; factory default case catches any remaining gaps.
**Warning signs:** Existing tests with `{}` YAML fail at NewBackend call.

### Pitfall 4: Removing cleanPath() from task and workflow before adding pathutil import
**What goes wrong:** Compilation failure if both files are edited simultaneously in the wrong order.
**Why it happens:** Go does not allow forward references to unexported functions across packages.
**How to avoid:** Create pathutil package first, then update imports in inputs.go and convert.go.
**Warning signs:** `undefined: cleanPath` compile error.

### Pitfall 5: config_test.go TestLoad fails with unknown field warning
**What goes wrong:** If existing TestLoad YAML does not include a `storage:` section, and strict YAML parsing is enabled, it would fail. `gopkg.in/yaml.v3` by default ignores unknown fields but also silently ignores missing fields — no issue here.
**Why it happens:** N/A — yaml.v3 is permissive; adding new fields to Config struct does not break existing tests.
**How to avoid:** No action needed; existing tests are safe.
**Warning signs:** None expected.

## Code Examples

Verified patterns from official sources:

### s3:// Guard (core correctness pattern)
```go
// wdl/pathutil/pathutil.go
import (
    "path/filepath"
    "strings"
)

func CleanPath(p string, wdlDir string) string {
    if strings.HasPrefix(p, "s3://") {
        return p  // must be first — before any filepath call
    }
    if !filepath.IsAbs(p) {
        if wdlDir != "" {
            return filepath.Clean(filepath.Join(wdlDir, p))
        }
        if abs, err := filepath.Abs(p); err == nil {
            return filepath.Clean(abs)
        }
    }
    return filepath.Clean(p)
}
```

### StorageConfig Validation (fail-fast pattern)
```go
// internal/config/config.go

func Validate(cfg *Config) error {
    switch cfg.Storage.Type {
    case "local", "s3":
    default:
        return fmt.Errorf("config: storage.type %q is not supported (valid values: local, s3)", cfg.Storage.Type)
    }
    if cfg.Storage.Type == "s3" && cfg.Storage.Bucket == "" {
        return fmt.Errorf("config: storage.type is \"s3\" but storage.bucket is not set")
    }
    return nil
}
```

### YAML config example (documentation for users)
```yaml
# flowy.yaml — storage section
storage:
  type: s3
  bucket: my-workflow-bucket
  endpoint: http://localhost:9000  # MinIO; omit for AWS S3
  region: us-east-1
  prefix: flowy-runs/
  multipart_threshold: 104857600  # 100 MiB (optional)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `NewBackend()` always returns LocalBackend | `NewBackend()` switches on cfg.Storage.Type | Phase 15 | Enables Phase 16 S3Backend registration |
| Two duplicate cleanPath() definitions | Single pathutil.CleanPath() with s3:// guard | Phase 15 | Eliminates URI corruption bug |
| No storage config in YAML | StorageConfig struct in Config | Phase 15 | Enables zero-config default + S3 opt-in |

**Deprecated/outdated:**
- Local `cleanPath()` in wdl/task/inputs.go: replaced by pathutil.CleanPath()
- Local `cleanPath()` in wdl/workflow/convert.go: replaced by pathutil.CleanPath()

## Open Questions

1. **Should SubmitRequest carry the resolved storage type to the server?**
   - What we know: The server reads its own config; `--storage` on the client overrides only locally for validation purposes in Phase 15
   - What's unclear: Phase 16+ may need the client's --storage choice to be forwarded so multi-tenant scenarios work
   - Recommendation: For Phase 15, do NOT add storage type to SubmitRequest — server always uses its own config; defer to Phase 16 if needed

2. **Should config.Validate() be called in Load() or externally?**
   - What we know: Load() is used by CLI client (no S3 config), tests, and server; Validate() only makes sense for server
   - What's unclear: Consistency argument for calling Validate() everywhere
   - Recommendation: Call Validate() explicitly in cmd/flowy-server/main.go only, not in Load()

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib testing (go test) |
| Config file | none (go test ./... discovers tests by convention) |
| Quick run command | `go test ./internal/config/... ./wdl/pathutil/... ./internal/api/... ./internal/server/... -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PATH-01 | CleanPath("s3://bucket/key", "") returns "s3://bucket/key" unchanged | unit | `go test ./wdl/pathutil/... -run TestCleanPath -count=1` | ❌ Wave 0 |
| PATH-01 | CleanPath("s3://bucket/path/../key", "") returns URI unchanged (no Clean) | unit | `go test ./wdl/pathutil/... -run TestCleanPath_S3NoCorruption -count=1` | ❌ Wave 0 |
| PATH-02 | CleanPath with s3:// prefix passes through regardless of wdlDir | unit | `go test ./wdl/pathutil/... -run TestCleanPath_S3IgnoresWdlDir -count=1` | ❌ Wave 0 |
| PATH-03 | CleanPath("/abs/path", "") returns filepath.Clean("/abs/path") | unit | `go test ./wdl/pathutil/... -run TestCleanPath_AbsLocal -count=1` | ❌ Wave 0 |
| PATH-04 | CleanPath("relative.txt", "/wdl/dir") returns "/wdl/dir/relative.txt" | unit | `go test ./wdl/pathutil/... -run TestCleanPath_Relative -count=1` | ❌ Wave 0 |
| CONF-01 | Config.Load() with storage section parses all fields correctly | unit | `go test ./internal/config/... -run TestLoadStorageConfig -count=1` | ❌ Wave 0 |
| CONF-01 | Config.Load() with no storage section defaults to type="local" | unit | `go test ./internal/config/... -run TestLoadStorageDefaults -count=1` | ❌ Wave 0 |
| CONF-01 | Config.Validate() returns error when type="s3" and bucket="" | unit | `go test ./internal/config/... -run TestValidateS3MissingBucket -count=1` | ❌ Wave 0 |
| CONF-01 | Config.Validate() returns error for unknown storage type | unit | `go test ./internal/config/... -run TestValidateUnknownType -count=1` | ❌ Wave 0 |
| CONF-02 | Config.Storage.Type defaults to "local" when storage section absent | unit | covered by TestLoadStorageDefaults above | ❌ Wave 0 |
| CONF-03 | `flowy run --storage s3 workflow.wdl` accepted without error (server reachable) | integration | `go test ./cmd/flowy/... -run TestRunStorageFlag -count=1` | ❌ Wave 0 |
| CONF-03 | `flowy run --storage unknown workflow.wdl` returns exit 2 with helpful error | unit | `go test ./cmd/flowy/... -run TestRunStorageFlagUnknown -count=1` | ❌ Wave 0 |
| CONF-04 | TaskClaimResponse JSON includes storage_type field for local backend | unit | `go test ./internal/server/... -run TestHandleClaim_StorageFields -count=1` | ❌ Wave 0 |
| CONF-04 | TaskClaimResponse JSON has storage_type="s3" with bucket/endpoint/prefix when config is s3 | unit | `go test ./internal/server/... -run TestHandleClaim_StorageFieldsS3 -count=1` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/config/... ./wdl/pathutil/... -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green (`go test ./... -count=1` all pass) before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `wdl/pathutil/pathutil.go` — the package itself (Wave 0 creates it)
- [ ] `wdl/pathutil/pathutil_test.go` — covers PATH-01 through PATH-04
- [ ] `internal/config/config_test.go` additions — covers CONF-01, CONF-02 (file exists, add cases)
- [ ] `cmd/flowy/main_test.go` additions or new test — covers CONF-03 flag behavior
- [ ] `internal/server/handlers_test.go` additions — covers CONF-04 claim response fields

## Sources

### Primary (HIGH confidence)
- Direct code inspection: `/home/uehara/flowy/internal/config/config.go` — existing Config struct, applyDefaults, Load, Default patterns
- Direct code inspection: `/home/uehara/flowy/internal/api/types.go` — TaskClaimResponse current shape, JSON tag conventions
- Direct code inspection: `/home/uehara/flowy/internal/server/handlers.go` — handleClaim() current population logic
- Direct code inspection: `/home/uehara/flowy/cmd/flowy/main.go` — flag parsing patterns, reorderArgs, resolveServerURL pattern
- Direct code inspection: `/home/uehara/flowy/internal/storage/backend.go` — ParseS3URI strings-based approach
- Direct code inspection: `/home/uehara/flowy/internal/storage/factory.go` — NewBackend stub ready for extension
- Direct code inspection: `/home/uehara/flowy/wdl/task/inputs.go` line 282-292 — first cleanPath() definition
- Direct code inspection: `/home/uehara/flowy/wdl/workflow/convert.go` line 143-153 — second (duplicate) cleanPath() definition
- Go stdlib docs — `path/filepath`, `strings`, `flag` packages (stable since Go 1.0)
- `gopkg.in/yaml.v3` — struct tag conventions confirmed in existing config.go usage

### Secondary (MEDIUM confidence)
- Test run: `go test ./... -count=1` confirmed all 19 packages pass (258 test functions)
- `.planning/STATE.md` — confirmed Phase 14 decisions: no context.Context in Backend interface

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries already in use; no new dependencies
- Architecture: HIGH — all patterns derived from existing code in the same repo
- Pitfalls: HIGH — derived from filepath.Clean behavior (verified: `filepath.Clean("s3://b/k")` → `s3:/b/k` confirmed)

**Research date:** 2026-03-13
**Valid until:** 2026-06-13 (stable domain — stdlib + yaml.v3 have no breaking changes pending)
