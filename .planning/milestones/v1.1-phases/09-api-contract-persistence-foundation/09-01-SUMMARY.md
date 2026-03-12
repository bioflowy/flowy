---
phase: 09-api-contract-persistence-foundation
plan: 01
subsystem: api, config, database
tags: [yaml, sqlite, chi, goose, modernc-sqlite, go-config, dto, filesystem]

# Dependency graph
requires: []
provides:
  - "YAML config loader with per-binary defaults for server, daemon, and client"
  - "HTTP DTO types for workflow submission, status polling, task claiming, and heartbeating"
  - "Filesystem layout helpers for run input/output directories"
affects:
  - 09-02
  - 10-server-binary
  - 11-daemon-binary
  - 12-client-binary

# Tech tracking
tech-stack:
  added:
    - "gopkg.in/yaml.v3 — YAML config parsing"
    - "modernc.org/sqlite v1.46.1 — CGO-free SQLite driver"
    - "github.com/pressly/goose/v3 v3.27.0 — DB migrations"
    - "github.com/go-chi/chi/v5 v5.2.5 — HTTP routing"
  patterns:
    - "Leaf packages with zero cross-dependencies (internal/api imports only stdlib)"
    - "applyDefaults() called after yaml.Unmarshal to fill zero-value fields"
    - "os.MkdirAll for idempotent directory creation with 0755 permissions"

key-files:
  created:
    - "internal/config/config.go"
    - "internal/config/config_test.go"
    - "internal/api/types.go"
    - "internal/api/types_test.go"
    - "internal/store/rundir.go"
    - "internal/store/rundir_test.go"
  modified:
    - "go.mod"
    - "go.sum"

key-decisions:
  - "DefaultDSN() returns DBDSN unchanged if explicitly set, otherwise constructs SQLite WAL DSN"
  - "internal/api is a strict leaf package — only stdlib imports allowed, verified by go vet"
  - "CreateRunDirs uses os.MkdirAll (not os.Mkdir) to ensure idempotency"

patterns-established:
  - "TDD RED-GREEN pattern: commit failing tests first, then implementation"
  - "Config defaults applied post-unmarshal via applyDefaults() using zero-value checks"

requirements-completed:
  - STR-01
  - STR-02
  - STR-03
  - STR-04

# Metrics
duration: 2min
completed: 2026-03-11
---

# Phase 9 Plan 01: Config + API DTO + Run Directory Helpers Summary

**YAML config loader with per-binary defaults, HTTP DTO types as a stdlib-only leaf package, and idempotent filesystem run directory helpers using modernc.org/sqlite + chi + goose added to go.mod**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-11T13:49:55Z
- **Completed:** 2026-03-11T13:52:42Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments

- Config loader reads YAML and applies defaults (base_dir, listen_addr, heartbeat timeouts, server URLs)
- DefaultDSN() constructs SQLite WAL-mode DSN pointing to {base_dir}/flowy.db when db_dsn is unset
- 10 HTTP DTO types covering the full flowy API surface (submit, status, outputs, abort, task claiming, heartbeat)
- Run directory helpers create {base_dir}/runs/{runID}/inputs and /outputs idempotently

## Task Commits

Each task was committed atomically:

1. **Task 1 RED: failing tests for config and API types** - `135b641` (test)
2. **Task 1 GREEN: config loader and API DTO types** - `0b647b8` (feat)
3. **Task 2 RED: failing tests for run directory helpers** - `212baf9` (test)
4. **Task 2 GREEN: run directory path helpers** - `b0ff989` (feat)

_Note: TDD tasks committed in RED (test) then GREEN (feat) pairs_

## Files Created/Modified

- `internal/config/config.go` - YAML config loader with Config/ServerConfig/DaemonConfig/ClientConfig structs and DefaultDSN()
- `internal/config/config_test.go` - Unit tests: load, defaults, missing file, malformed YAML, partial config, DSN construction
- `internal/api/types.go` - HTTP DTO types: SubmitRequest/Response, StatusResponse, OutputsResponse, AbortResponse, ErrorResponse, TaskClaimRequest/Response, HeartbeatRequest, TaskResultRequest
- `internal/api/types_test.go` - JSON round-trip tests for all 10 DTO types
- `internal/store/rundir.go` - RunInputsDir(), RunOutputsDir(), CreateRunDirs() helpers
- `internal/store/rundir_test.go` - Unit tests: path construction and idempotent directory creation
- `go.mod` / `go.sum` - Added yaml.v3, modernc.org/sqlite, goose/v3, chi/v5

## Decisions Made

- DefaultDSN() returns DBDSN unchanged if explicitly set, enabling DSN override for advanced setups
- internal/api imports only stdlib packages — verified by go vet; enforces the leaf package constraint
- CreateRunDirs uses os.MkdirAll to be idempotent — calling twice does not error or remove existing content

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None — all tests passed on first GREEN implementation.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All foundation packages ready: internal/config, internal/api, internal/store/rundir
- Plan 02 (persistence/SQLite migrations) can now import from internal/config and internal/store
- No blockers

---
*Phase: 09-api-contract-persistence-foundation*
*Completed: 2026-03-11*

## Self-Check: PASSED

- All 6 source/test files created and present on disk
- All 4 task commits verified in git history (135b641, 0b647b8, 212baf9, b0ff989)
- SUMMARY.md present at expected path
