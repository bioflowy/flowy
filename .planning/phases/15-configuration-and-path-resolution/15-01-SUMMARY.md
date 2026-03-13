---
phase: 15-configuration-and-path-resolution
plan: 01
subsystem: config
tags: [go, yaml, s3, pathutil, storage, config-validation]

# Dependency graph
requires:
  - phase: 14-storage-abstraction
    provides: Backend interface, LocalBackend, ParseS3URI — storage foundation this config wires into
provides:
  - wdl/pathutil package with CleanPath(p, wdlDir) guarding s3:// URIs from filepath.Clean corruption
  - StorageConfig struct with YAML tags nested in Config (type, bucket, endpoint, region, prefix, multipart_threshold)
  - config.Validate(cfg) function failing fast on unknown type or s3 without bucket
  - config.applyDefaults sets type="local" and multipart_threshold=100MiB for zero-value
  - storage.NewBackend() switches on cfg.Storage.Type (both paths return LocalBackend in Phase 15)
  - flowy-server calls config.Validate(cfg) immediately after config.Load() for fail-fast startup
affects:
  - 15-02 (CLI --storage flag depends on StorageConfig and Validate)
  - 16-s3-backend (NewBackend switch point ready for S3Backend insertion)
  - 17-daemon-s3 (StorageType/Bucket/Endpoint/Prefix from config flows via TaskClaimResponse)

# Tech tracking
tech-stack:
  added: [wdl/pathutil (new internal package)]
  patterns:
    - Guard-and-passthrough for s3:// URIs — strings.HasPrefix check before any filepath call
    - Fail-fast config validation called in server main() after Load(), not inside Load()
    - Config section struct pattern following ServerConfig/DaemonConfig shape

key-files:
  created:
    - wdl/pathutil/pathutil.go
    - wdl/pathutil/pathutil_test.go
  modified:
    - wdl/task/inputs.go
    - wdl/workflow/convert.go
    - internal/config/config.go
    - internal/config/config_test.go
    - internal/storage/factory.go
    - cmd/flowy-server/main.go

key-decisions:
  - "s3:// guard is first statement in CleanPath — before any filepath call — to prevent filepath.Clean corrupting s3://bucket/key to s3:/bucket/key"
  - "Validate() is called in server main.go only, not inside Load() — client CLI and tests use Load() without S3 config"
  - "NewBackend() switch returns LocalBackend for s3 case in Phase 15 — Phase 16 inserts real S3Backend here"
  - "StorageConfig follows existing ServerConfig/DaemonConfig pattern with yaml struct tags"

patterns-established:
  - "Pattern: Guard-and-passthrough for URI schemes — check strings.HasPrefix(p, 'scheme://') before any path operation"
  - "Pattern: Fail-fast config validation in server startup — config.Load() then config.Validate() then sqlite.Open()"
  - "Pattern: Config section struct mirrors ServerConfig shape — all fields with yaml tags, defaults in applyDefaults()"

requirements-completed: [PATH-01, PATH-02, PATH-03, PATH-04, CONF-01, CONF-02]

# Metrics
duration: 3min
completed: 2026-03-13
---

# Phase 15 Plan 01: Configuration and Path Resolution Summary

**Shared pathutil.CleanPath with s3:// guard, StorageConfig YAML struct with Validate(), and fail-fast server startup validation**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-13T01:28:29Z
- **Completed:** 2026-03-13T01:31:34Z
- **Tasks:** 3
- **Files modified:** 8

## Accomplishments

- Created wdl/pathutil package with CleanPath() that guards s3:// URIs from filepath.Clean corruption (s3://bucket/key would become s3:/bucket/key without the guard)
- Added StorageConfig struct to Config with full YAML parsing, defaults (type="local", threshold=100MiB), and Validate() for s3 bucket requirement
- Wired config.Validate(cfg) into flowy-server main() between Load() and sqlite.Open() for fail-fast startup on invalid storage config
- Deleted duplicate cleanPath() functions from wdl/task/inputs.go and wdl/workflow/convert.go; replaced with pathutil.CleanPath calls
- Updated NewBackend() factory to switch on cfg.Storage.Type (s3 case returns LocalBackend placeholder; Phase 16 will replace it)
- 265 total tests pass (258 existing + 7 new)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create pathutil package and replace duplicate cleanPath** - `fd74c05` (feat)
2. **Task 2: Add StorageConfig to config, Validate function, and update factory** - `103f576` (feat)
3. **Task 3: Wire config.Validate into flowy-server startup for fail-fast validation** - `696d60b` (feat)

**Plan metadata:** (docs commit follows)

_Note: Tasks 1 and 2 used TDD: RED (failing tests written first), GREEN (implementation), no REFACTOR needed_

## Files Created/Modified

- `wdl/pathutil/pathutil.go` - New shared CleanPath(p, wdlDir) with s3:// guard before any filepath call
- `wdl/pathutil/pathutil_test.go` - 8 table-driven test cases covering s3 passthrough, abs, relative, and dotdot variants
- `wdl/task/inputs.go` - Import pathutil, replace 2 cleanPath calls, delete local cleanPath definition
- `wdl/workflow/convert.go` - Import pathutil, replace 2 cleanPath calls, delete local cleanPath definition
- `internal/config/config.go` - Add StorageConfig struct, Storage field to Config, applyDefaults additions, Validate() function
- `internal/config/config_test.go` - 6 new tests: TestLoadStorageConfig, TestLoadStorageDefaults, TestValidate{LocalOK,S3OK,S3MissingBucket,UnknownType}
- `internal/storage/factory.go` - Switch on cfg.Storage.Type in NewBackend(), s3 case returns LocalBackend placeholder
- `cmd/flowy-server/main.go` - config.Validate(cfg) called immediately after config.Load() with log.Fatalf on error

## Decisions Made

- s3:// guard must be the first statement in CleanPath before any filepath call — filepath.Clean("s3://bucket/key") returns "s3:/bucket/key" on Linux (confirmed in research)
- Validate() is called explicitly in server main.go only, not inside Load() — the client CLI and tests use Load() without S3 config being set
- NewBackend() returns LocalBackend for s3 case in Phase 15 — safe because Validate() ensures bucket is set in production; Phase 16 inserts real S3Backend

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- StorageConfig is ready for Plan 02 (--storage CLI flag reads cfg.Storage.Type as default)
- NewBackend() switch point ready for Phase 16 S3Backend insertion
- pathutil.CleanPath available for any future path resolution in WDL packages
- All 265 tests green; binary builds and passes vet

---
*Phase: 15-configuration-and-path-resolution*
*Completed: 2026-03-13*
