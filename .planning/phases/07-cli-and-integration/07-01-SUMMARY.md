---
phase: 07-cli-and-integration
plan: "01"
subsystem: cli
tags: [go, cli, parser, flag, rustc-diagnostics]

# Dependency graph
requires:
  - phase: 02-lexer-parser-and-ast
    provides: parser.Parse function and ParseError with rustc-style formatting
  - phase: 03-type-checker
    provides: checker.Check function and TypeError with Pos for diagnostics

provides:
  - ParseWithSearchPaths exported function in wdl/parser package
  - cmd/flowy binary with check subcommand and testable-main pattern
  - rustc-style error diagnostics for parse and type errors

affects:
  - 07-02 (run subcommand will use same run() pattern from main.go)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - testable-main pattern with run(args, stdout, stderr) for CLI testing
    - flag.NewFlagSet with ContinueOnError for per-subcommand flag parsing
    - import search path extension via ParseWithSearchPaths

key-files:
  created:
    - wdl/parser/parser_paths_test.go
    - cmd/flowy/main.go
    - cmd/flowy/main_test.go
    - cmd/flowy/testdata/valid.wdl
    - cmd/flowy/testdata/invalid.wdl
    - cmd/flowy/testdata/type_error.wdl
    - cmd/flowy/testdata/with_import.wdl
    - cmd/flowy/testdata/importlib/shared_lib.wdl
  modified:
    - wdl/parser/parser.go

key-decisions:
  - "ParseWithSearchPaths delegates to parseFileWithPaths(path, src, visited, chain, searchPaths) — parseFile now calls parseFileWithPaths with nil searchPaths for backward compat"
  - "Import resolution tries os.Stat on primary dir first, then searchPaths in order — first match wins without reading file content"
  - "run(args, stdout, stderr) testable-main pattern with no subprocess needed in tests"
  - "TypeError rustc-style formatting done in main.go readSourceLine+printRustcError — checker.TypeError.Error() lacks source line"
  - "Unknown subcommands exit 2; parse/type errors exit 1; success exits 0"

patterns-established:
  - "testable-main: func run(args []string, stdout, stderr io.Writer) int with func main() { os.Exit(run(...)) }"
  - "per-subcommand flag sets: flag.NewFlagSet(name, flag.ContinueOnError) with SetOutput(stderr)"
  - "rustc-style diagnostics: error: message / --> file:line:col / N | source / N |  ^"

requirements-completed: [CLI-01, CLI-04, CLI-06]

# Metrics
duration: 3min
completed: 2026-03-10
---

# Phase 7 Plan 01: CLI Skeleton and ParseWithSearchPaths Summary

**flowy check CLI with rustc-style diagnostics and ParseWithSearchPaths for multi-directory import resolution**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-10T23:54:23Z
- **Completed:** 2026-03-10T23:58:16Z
- **Tasks:** 2
- **Files modified:** 9

## Accomplishments

- Added `ParseWithSearchPaths` to the parser package with full search-path fallback logic
- Created `cmd/flowy` binary with `check` subcommand, `--version`, and `--path` flag
- Rustc-style error output for both parse errors (from ParseError.Error()) and type errors (readSourceLine + printRustcError)
- All 7 CLI tests and 4 search-path tests pass; full test suite (`go test ./...`) passes

## Task Commits

Each task was committed atomically:

1. **Task 1: Add ParseWithSearchPaths to parser package** - `757c690` (feat)
2. **Task 2: Create cmd/flowy/main.go with check subcommand** - `ef8aed4` (feat)

**Plan metadata:** (committed with this summary)

_Note: Both tasks used TDD: tests written first (RED), then implementation (GREEN)._

## Files Created/Modified

- `wdl/parser/parser.go` - Added ParseWithSearchPaths, refactored parseFile to parseFileWithPaths with searchPaths propagation
- `wdl/parser/parser_paths_test.go` - 4 tests for ParseWithSearchPaths covering nil paths, secondary dir, precedence, and not-found
- `cmd/flowy/main.go` - CLI entry point with run(), runCheck(), printRustcError(), readSourceLine()
- `cmd/flowy/main_test.go` - 7 tests for all subcommand behaviors
- `cmd/flowy/testdata/valid.wdl` - Valid WDL 1.2 task for success test
- `cmd/flowy/testdata/invalid.wdl` - WDL with @@@ syntax error for failure test
- `cmd/flowy/testdata/type_error.wdl` - WDL with Int->String type mismatch
- `cmd/flowy/testdata/with_import.wdl` - WDL that imports shared_lib.wdl from secondary dir
- `cmd/flowy/testdata/importlib/shared_lib.wdl` - Shared lib for --path flag test

## Decisions Made

- `ParseWithSearchPaths` delegates to new `parseFileWithPaths(path, src, visited, chain, searchPaths)`. The existing `parseFile` is preserved as a wrapper calling `parseFileWithPaths` with nil searchPaths, ensuring all existing callers are unaffected.
- Import resolution uses `os.Stat` to test existence before reading: try primary dir, then searchPaths in order.
- `TypeError` does not include source line in its Error() method, so `cmd/flowy/main.go` provides `readSourceLine` and `printRustcError` helpers to produce full rustc-style output.
- Testable-main pattern: `run(args []string, stdout, stderr io.Writer) int` — no subprocess needed in tests, all 7 tests call `run()` directly.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `cmd/flowy` binary is ready for extension with the `run` subcommand (Plan 02)
- The `run` stub already returns 0, so unknown-subcommand behavior is not impacted
- `ParseWithSearchPaths` is fully available for any future CLI flags requiring multi-path import resolution

---
*Phase: 07-cli-and-integration*
*Completed: 2026-03-10*
