# Phase 15: Configuration and Path Resolution - Context

**Gathered:** 2026-03-13
**Status:** Ready for planning

<domain>
## Phase Boundary

Add YAML configuration for storage backend selection (type, bucket, endpoint, region, prefix), a `--storage` CLI flag on `flowy run` and `flowy submit`, guard `cleanPath()` against s3:// URI corruption, and extend TaskClaimResponse with storage addressing fields for the daemon. All 156 spectool tests must continue to pass.

</domain>

<decisions>
## Implementation Decisions

### YAML config structure
- Nested `storage:` top-level section with sub-keys: type, bucket, endpoint, region, prefix, multipart_threshold
- Consistent with existing server/daemon/client sections in Config struct
- Default storage type is "local" when no storage section exists — zero-config for existing users
- Validate at startup: if type is "s3", require bucket to be non-empty at config load time — fail fast
- Prefix is optional, defaults to empty string (keys go to bucket root). Users set prefix to namespace multiple flowy instances

### CLI flag behavior
- `--storage` flag overrides only storage.type from YAML — all other S3 settings (bucket, endpoint, region) come from YAML
- Error before submit if `--storage s3` is used but S3 config (bucket) missing from YAML — actionable error message
- Only `flowy run` and `flowy submit` accept `--storage` — other commands don't create runs
- Validate against known values whitelist: "local" and "s3" — error on unknown values with available list

### Path resolution strategy
- Guard-and-passthrough: if strings.HasPrefix(p, "s3://") { return p } — no filepath.Abs, no filepath.Clean on S3 URIs
- Deduplicate the two identical cleanPath() functions (wdl/task/inputs.go and wdl/workflow/convert.go) into a shared package (e.g. wdl/pathutil)
- Per-input scheme detection: s3:// → S3 backend, /abs/path → local filesystem, relative path → configured default backend
- Guard s3:// only — no future-proofing for gs://, az:// until those backends exist (YAGNI)

### TaskClaimResponse extensions
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

</decisions>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches. The key constraints are: zero-config backward compatibility for existing users, fail-fast validation for S3 config, and minimal change to cleanPath() (guard-and-passthrough).

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/config/config.go`: Config struct with Load(), applyDefaults(), Default() — add StorageConfig as a new field alongside Server/Daemon/Client
- `internal/storage/backend.go`: ParseS3URI already exists — can be used for s3:// detection in path resolution
- `internal/storage/factory.go`: NewBackend(cfg) currently returns LocalBackend unconditionally — extend to switch on cfg.Storage.Type

### Established Patterns
- Config struct uses `yaml:"field"` tags with nested sub-structs (ServerConfig, DaemonConfig, ClientConfig)
- CLI flag parsing in `cmd/flowy/main.go` uses stdlib `flag` package
- `cmd/flowy-server/main.go` loads config with `config.Load(cfgPath)` and passes to server.New()

### Integration Points
- `cmd/flowy/main.go`: Add `--storage` flag to run and submit commands, pass to server via submit request
- `cmd/flowy-server/main.go`: Config.Storage feeds into NewBackend(cfg) and TaskClaimResponse population
- `internal/api/types.go`: TaskClaimResponse gets 4 new fields
- `internal/server/handlers.go`: Populate TaskClaimResponse storage fields from server config
- `wdl/task/inputs.go:281` and `wdl/workflow/convert.go:142`: Both cleanPath() functions → deduplicate into shared package

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 15-configuration-and-path-resolution*
*Context gathered: 2026-03-13*
