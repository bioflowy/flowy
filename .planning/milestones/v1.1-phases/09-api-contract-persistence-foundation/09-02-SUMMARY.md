---
phase: 09-api-contract-persistence-foundation
plan: "02"
subsystem: database
tags: [sqlite, modernc-sqlite, goose, migrations, wal-mode, store-interface]

requires:
  - phase: 09-01
    provides: "store package skeleton (rundir.go), config.DefaultDSN() with WAL DSN construction"

provides:
  - "Store interface (internal/store/store.go) with Run/Task model types and RunState/TaskState constants"
  - "SQLiteStore implementing Store with WAL mode, separate writer/reader pools, goose embedded migrations"
  - "Atomic ClaimTask via UPDATE+RETURNING — no TOCTOU race"
  - "FindStaleTasks with SQLite datetime() string modifier"
  - "Full integration test suite (11 tests, all green)"

affects:
  - phase-10-server
  - phase-11-daemon

tech-stack:
  added:
    - "modernc.org/sqlite v1.46.1 (already in go.mod)"
    - "github.com/pressly/goose/v3 v3.27.0 (already in go.mod)"
  patterns:
    - "Writer pool: MaxOpenConns(1) to serialise SQLite writes"
    - "Reader pool: no connection limit for concurrent reads"
    - "Embedded migrations: //go:embed migrations/*.sql + goose.SetBaseFS()"
    - "Atomic task claiming: single UPDATE WHERE id=(SELECT id ... LIMIT 1) RETURNING *"
    - "Stale detection: datetime('now', '-N seconds') string modifier (not integer)"
    - "RETURNING clause used in ClaimTask to avoid second query while rows cursor is open"

key-files:
  created:
    - internal/store/store.go
    - internal/store/sqlite/sqlite.go
    - internal/store/sqlite/runs.go
    - internal/store/sqlite/tasks.go
    - internal/store/sqlite/migrations/00001_init.sql
    - internal/store/sqlite/sqlite_test.go
  modified: []

key-decisions:
  - "ClaimTask uses RETURNING with all columns in single UPDATE — avoids deadlock from opening reader while writer rows cursor is still open under single-connection writer pool"
  - "OpenMemory() uses file::memory:?cache=shared DSN so writer and reader see the same in-memory database"
  - "WriterDB() method exposes *sql.DB for test helpers that need to backdate timestamps or inspect state directly"
  - "FindStaleTasks uses fmt.Sprintf('-%d seconds', timeoutSec) string modifier — SQLite datetime() requires string, not integer"

patterns-established:
  - "Store is a pure interface package — no external imports, no coupling to config or api packages"
  - "Integration tests use OpenMemory() for isolation — each test creates its own store instance"
  - "Timestamps backdated via WriterDB() in tests, not via test-only interface methods"

requirements-completed:
  - SRV-07
  - SRV-08

duration: 7min
completed: 2026-03-11
---

# Phase 09 Plan 02: Store Interface and SQLite Implementation Summary

**SQLite WAL store with goose migrations, atomic ClaimTask via UPDATE+RETURNING, and stale heartbeat detection — Store interface future-proofs for PostgreSQL**

## Performance

- **Duration:** 7 min
- **Started:** 2026-03-11T13:54:57Z
- **Completed:** 2026-03-11T14:01:54Z
- **Tasks:** 2 (Task 1: Store interface; Task 2: SQLite implementation with TDD)
- **Files modified:** 6 created

## Accomplishments

- Store interface with Run/Task model types, RunState/TaskState constants — pure stdlib package, no external dependencies
- SQLiteStore with WAL mode, separate writer (MaxOpenConns=1) and reader pools, goose embedded migrations
- Atomic ClaimTask using single `UPDATE WHERE id=(SELECT id ... LIMIT 1) RETURNING *` — prevents TOCTOU races
- FindStaleTasks correctly handles SQLite datetime() string modifier (`-N seconds`)
- 11 integration tests all green including concurrent claim test

## Task Commits

Each task was committed atomically:

1. **Task 1: Store interface and model types** - `db38dd1` (feat)
2. **Task 2: TDD RED — failing tests** - `3d40881` (test)
3. **Task 2: TDD GREEN — SQLite implementation** - `a8a60a0` (feat)

**Plan metadata:** (this commit) (docs: complete plan)

_Note: TDD tasks have multiple commits (test → feat)_

## Files Created/Modified

- `internal/store/store.go` — Store interface, Run/Task structs, RunState/TaskState constants
- `internal/store/sqlite/sqlite.go` — Open/OpenMemory, writer/reader pools, goose migration runner, WriterDB() test helper
- `internal/store/sqlite/runs.go` — CreateRun, GetRun, GetRunByPrefix (ambiguity detection), UpdateRunState
- `internal/store/sqlite/tasks.go` — CreateTask, ListTasksByRun, ClaimTask (atomic), UpdateTaskState, UpdateHeartbeat, FindStaleTasks
- `internal/store/sqlite/migrations/00001_init.sql` — runs and tasks tables with shard_index, last_heartbeat_at, indexes
- `internal/store/sqlite/sqlite_test.go` — 11 integration tests covering all Store methods

## Decisions Made

- **ClaimTask RETURNING design:** Initial implementation called `getTaskByID` after the UPDATE while the RETURNING rows cursor was still open. Under the single-connection writer pool this caused a deadlock (SQLite mutex contention). Fixed by expanding the RETURNING clause to include all columns, eliminating the second query entirely.
- **WriterDB() test helper:** Exposed to allow tests to backdate `last_heartbeat_at` directly for staleness simulation without adding test-only methods to the Store interface.
- **OpenMemory() cache=shared:** Required so the separate writer and reader connections see the same in-memory database rather than two isolated empty databases.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed ClaimTask deadlock under single-connection writer pool**
- **Found during:** Task 2 (TestClaimTask timed out after 60s)
- **Issue:** `ClaimTask` queried `getTaskByID` using the reader while the writer's `RETURNING` rows cursor was still open. With `MaxOpenConns(1)` on the writer and `cache=shared` on the in-memory DSN, both connections competed for the same SQLite mutex, causing a deadlock.
- **Fix:** Expanded `RETURNING` clause from `id, run_id, task_name, shard_index` to all nine columns; removed `getTaskByID` helper entirely; `scanTask` called directly on the RETURNING rows.
- **Files modified:** `internal/store/sqlite/tasks.go`
- **Verification:** `TestClaimTask`, `TestClaimTaskAtomicNoRace`, and all 11 tests pass
- **Committed in:** `a8a60a0` (Task 2 GREEN commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 - Bug)
**Impact on plan:** Auto-fix corrected a correctness/liveness bug in the implementation. No scope creep.

## Issues Encountered

- The `cache=shared` DSN needed for in-memory test isolation changed the locking behaviour compared to file-based SQLite, exposing a deadlock that would not occur in production (where writer and reader use the same file without `MaxOpenConns` contention on reader). The fix (RETURNING all columns) is actually cleaner than the original design.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `store.Store` interface is stable and ready to be injected into Phase 10 (HTTP server) and Phase 11 (daemon)
- `sqlite.Open(dsn)` accepts the WAL DSN produced by `config.DefaultDSN()` from Phase 09-01
- All schema columns required by Phase 11 (`last_heartbeat_at`, `shard_index`) are present from day one

---
*Phase: 09-api-contract-persistence-foundation*
*Completed: 2026-03-11*

## Self-Check: PASSED

- All 6 files exist on disk
- All 3 commits verified in git history (db38dd1, 3d40881, a8a60a0)
