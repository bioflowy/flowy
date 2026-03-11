---
phase: 09-api-contract-persistence-foundation
verified: 2026-03-11T14:10:00Z
status: passed
score: 9/9 must-haves verified
re_verification: false
---

# Phase 9: API Contract and Persistence Foundation — Verification Report

**Phase Goal:** Shared foundation packages exist and are correct, so all three binaries can be built on stable contracts without drift or schema migration surprises
**Verified:** 2026-03-11T14:10:00Z
**Status:** passed
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A YAML config file with base_dir, server, daemon, and client sections loads without error | VERIFIED | `TestLoad` passes; `Load()` calls `yaml.Unmarshal` + `applyDefaults()` |
| 2 | Missing or malformed config file returns a clear error | VERIFIED | `TestLoadMissing` and `TestLoadMalformed` both pass |
| 3 | Config defaults are applied (base_dir, server listen, timeouts, daemon/client server_url) | VERIFIED | `TestLoadDefaults` and `TestLoadPartial` pass; `applyDefaults()` sets all 8 zero-value fields |
| 4 | Run directory helpers create {base_dir}/runs/{run_id}/inputs/ and /outputs/ | VERIFIED | `TestRunInputsDir`, `TestRunOutputsDir` pass; `CreateRunDirs` uses `os.MkdirAll` |
| 5 | HTTP DTO types compile without importing internal/store or internal/config | VERIFIED | `go list -deps internal/api` shows zero external or internal imports; only stdlib |
| 6 | SQLite database opens in WAL mode with goose migrations running on startup without error | VERIFIED | `TestOpen` passes; goose log: "OK 00001_init.sql (1.89ms)" |
| 7 | The runs and tasks tables exist with correct schema including last_heartbeat_at and shard_index | VERIFIED | `TestSchema` passes; migration SQL confirms both columns present |
| 8 | A stale task (heartbeat older than timeout) is detected by FindStaleTasks query | VERIFIED | `TestFindStaleTasks` and `TestFindStaleTasksIncludesClaimed` pass |
| 9 | Atomic task claiming via UPDATE WHERE state='PENDING' prevents TOCTOU race | VERIFIED | `TestClaimTaskAtomicNoRace` passes; single `UPDATE ... RETURNING` pattern confirmed in tasks.go |

**Score:** 9/9 truths verified

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/config/config.go` | YAML config loader with per-binary sections and defaults | VERIFIED | 121 lines; exports `Config`, `ServerConfig`, `DaemonConfig`, `ClientConfig`, `Load`, `DefaultDSN` |
| `internal/config/config_test.go` | Unit tests for config loading, defaults, error cases | VERIFIED | 7 tests (TestLoad, TestLoadDefaults, TestLoadMissing, TestLoadMalformed, TestLoadPartial, TestDBDSN, TestDBDSNOverride), all pass |
| `internal/api/types.go` | HTTP DTO types for workflow submission, status, outputs | VERIFIED | 98 lines; 10 DTO types exported; no imports at all (pure declarations) |
| `internal/api/types_test.go` | Compile check and JSON round-trip test for DTO types | VERIFIED | TestAPITypesJSON covers all 10 types with subtests, passes |
| `internal/store/rundir.go` | Run directory path helpers and creation | VERIFIED | 40 lines; exports `RunInputsDir`, `RunOutputsDir`, `CreateRunDirs` |
| `internal/store/rundir_test.go` | Unit tests for run directory creation | VERIFIED | TestRunInputsDir, TestRunOutputsDir pass |
| `internal/store/store.go` | Store interface and model types (Run, Task, RunState, TaskState) | VERIFIED | 99 lines; pure stdlib only; exports `Store`, `Run`, `Task`, `RunState`, `TaskState` |
| `internal/store/sqlite/sqlite.go` | SQLite Store implementation with WAL mode, reader/writer pools, goose migrations | VERIFIED | 99 lines; `Open`, `OpenMemory`, `WriterDB`; compile-time interface check present |
| `internal/store/sqlite/runs.go` | Run CRUD operations | VERIFIED | 105 lines; `CreateRun`, `GetRun`, `GetRunByPrefix`, `UpdateRunState` all implemented |
| `internal/store/sqlite/tasks.go` | Task CRUD operations including ClaimTask and FindStaleTasks | VERIFIED | 169 lines; all methods implemented with correct atomic/datetime patterns |
| `internal/store/sqlite/migrations/00001_init.sql` | Initial schema with runs and tasks tables | VERIFIED | Contains `CREATE TABLE` for both `runs` and `tasks`; `shard_index` and `last_heartbeat_at` present |
| `internal/store/sqlite/sqlite_test.go` | Integration tests for SQLite store | VERIFIED | 11 tests, all pass |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/config/config.go` | `gopkg.in/yaml.v3` | `yaml.Unmarshal` | WIRED | Line 72: `yaml.Unmarshal(data, &cfg)` confirmed |
| `internal/api/types.go` | no internal imports | leaf package | WIRED | `go list -deps` shows zero external/internal dependencies |
| `internal/store/sqlite/sqlite.go` | `modernc.org/sqlite` | `sql.Open("sqlite", ...)` | WIRED | Lines 54, 61: `sql.Open("sqlite", dsn)` confirmed |
| `internal/store/sqlite/sqlite.go` | `github.com/pressly/goose/v3` | `goose.Up` | WIRED | Line 74: `goose.Up(writer, "migrations")` confirmed |
| `internal/store/sqlite/sqlite.go` | `internal/store` | implements Store interface | WIRED | Line 32: `var _ store.Store = (*SQLiteStore)(nil)` confirmed |
| `internal/store/sqlite/tasks.go` | SQLite `datetime()` | stale task detection | WIRED | Line 126: `datetime('now', ?)` with string modifier confirmed |

---

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|---------|
| STR-01 | 09-01 | Config file (YAML) specifies shared filesystem base directory | SATISFIED | `Config.BaseDir` field with `yaml:"base_dir"` tag; `Load()` reads YAML; default "./flowy-data" |
| STR-02 | 09-01 | Config file has sections for server, daemon, and client settings | SATISFIED | `ServerConfig`, `DaemonConfig`, `ClientConfig` as nested YAML sections; all three binaries' settings isolated |
| STR-03 | 09-01 | Runs are organized under `{base_dir}/runs/{run_id}/` with inputs/ and outputs/ | SATISFIED | `RunInputsDir()` and `RunOutputsDir()` return `{baseDir}/runs/{runID}/inputs` and `/outputs`; `CreateRunDirs()` creates both |
| STR-04 | 09-01 | All three binaries read the same config file format | SATISFIED | Single `Config` struct with all three sections; `Load(path string)` shared across all binaries |
| SRV-07 | 09-02 | Server persists workflow runs and task state in SQLite with WAL mode | SATISFIED | `sqlite.Open()` uses `_pragma=journal_mode(WAL)` DSN; `goose.Up` creates schema; `Store` interface used for all persistence |
| SRV-08 | 09-02 | Server detects stale tasks (no heartbeat within timeout) and resets them to PENDING | SATISFIED | `FindStaleTasks(ctx, timeoutSec)` queries `WHERE state IN ('CLAIMED','RUNNING') AND last_heartbeat_at < datetime('now', modifier)` |

All 6 requirements covered. No orphaned requirements found for Phase 9.

---

### Anti-Patterns Found

None. No TODOs, FIXMEs, placeholder comments, empty implementations, or stub returns were found in any phase 9 source files.

---

### Human Verification Required

None for automated goal verification. All truths are verifiable programmatically.

One item worth noting for operational validation when Phase 10 is built:

**Test: WAL mode active on real filesystem DSN**
- **Test:** Open a file-based SQLite store using `config.DefaultDSN()`, then run `PRAGMA journal_mode` and verify it returns "wal"
- **Expected:** "wal" (not "delete" which is the SQLite default)
- **Why human:** The `TestOpen` test uses in-memory DSN with WAL pragma. The `DefaultDSN()` constructs the file-based WAL DSN but it is not exercised end-to-end until Phase 10 wires them together. This is integration smoke-test territory, not a gap in the current phase.

---

## Summary

Phase 9 achieves its stated goal. All 9 observable truths are verified against the actual codebase:

- The YAML config loader (`internal/config`) is substantive and tested: it handles all documented behaviors (load, defaults, missing file, malformed input, partial config, DSN construction).
- The HTTP DTO package (`internal/api`) is a genuine leaf package with zero imports — confirmed by `go list -deps`, not just by inspection.
- The run directory helpers (`internal/store/rundir.go`) correctly construct `{base_dir}/runs/{run_id}/inputs` and `/outputs` paths and create them idempotently.
- The Store interface (`internal/store/store.go`) is a pure stdlib package with no external dependencies.
- The SQLite implementation (`internal/store/sqlite/`) opens in WAL mode, runs goose migrations on startup, implements all Store methods substantively, and passes 11 integration tests including the atomic claim race test.
- All 6 requirements (STR-01 through STR-04, SRV-07, SRV-08) are satisfied with direct code evidence.
- `go vet ./internal/...` reports no issues.
- No anti-patterns detected.

The foundation is stable and ready for Phase 10 (server binary) and Phase 11 (daemon binary) to import from these packages.

---

_Verified: 2026-03-11T14:10:00Z_
_Verifier: Claude (gsd-verifier)_
