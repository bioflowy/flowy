---
phase: 02-lexer-parser-and-ast
plan: 05
subsystem: parser
tags: [go, wdl, parser, ast, imports, recursive-descent]

# Dependency graph
requires:
  - phase: 02-04
    provides: Pratt expression parser, parseDecl, complete parser infrastructure
provides:
  - Public Parse(filename, src) entry point with full import resolution
  - Recursive import merging with circular import and duplicate name detection
  - WDL 1.2 testdata fixtures (hello.wdl, lib.wdl, main.wdl, types.wdl)
  - Full integration test suite with no skips
affects:
  - Phase 03 and beyond: any package that calls wdl/parser.Parse

# Tech tracking
tech-stack:
  added: [os.ReadFile for file loading, path/filepath for import resolution]
  patterns: [recursive parseFile with visited map and chain for cycle detection, parseDocumentWithImports separating parse from resolve]

key-files:
  created:
    - wdl/testdata/hello.wdl
    - wdl/testdata/lib.wdl
    - wdl/testdata/main.wdl
    - wdl/testdata/types.wdl
  modified:
    - wdl/parser/parser.go
    - wdl/parser/parser_test.go

key-decisions:
  - "parseFile(path, src, visited, chain): src=nil means read from disk; src non-nil means use provided content (for root file)"
  - "Circular import detection uses visited map plus chain []string for human-readable cycle path in error message"
  - "parseDocumentWithImports returns (doc, []importStatement, error) allowing caller to resolve imports separately"
  - "Workflow input declarations use parseDecl(true) to allow optional default values per WDL 1.2 spec"

patterns-established:
  - "Import resolution: parse first (collect import stmts), then resolve recursively with merge"
  - "Duplicate name check: O(n*m) loop before appending each imported item"

requirements-completed: [PARSE-01, PARSE-07]

# Metrics
duration: 4min
completed: 2026-03-10
---

# Phase 2 Plan 5: Public Parse entry point with recursive import resolution

**parser.Parse() with recursive import resolution, circular import detection, duplicate name checking, and four WDL 1.2 testdata fixtures**

## Performance

- **Duration:** ~4 min
- **Started:** 2026-03-10T07:14:21Z
- **Completed:** 2026-03-10T07:18:02Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- Implemented `parser.Parse(filename, src)` as the public entry point that recursively resolves imports
- Added `parseFile` with visited map and chain for circular import cycle detection with descriptive error messages
- Implemented `parseImport()` parsing `import "path" [as namespace] [alias X as Y]*`
- Created four WDL 1.2 testdata fixtures covering common patterns (task, workflow, imports, complex types)
- All 9 test functions in wdl/parser pass with no skips; full `go test ./...` suite is green

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement Parse() entry point and import resolution** - `d1b6684` (feat)
2. **Task 2: WDL fixtures and full integration tests** - `ea466b3` (feat)

**Plan metadata:** (docs commit follows)

_Note: TDD tasks have failing test commits before implementation_

## Files Created/Modified
- `wdl/parser/parser.go` - Added Parse(), parseFile(), parseDocumentWithImports(), parseImport(), importStatement/importAlias types; fixed workflow input parsing
- `wdl/parser/parser_test.go` - Added TestImportResolution (3 sub-tests), TestImportFromFixture, TestTypesWDLFixture, updated TestParseDocument with hello.wdl fixture test
- `wdl/testdata/hello.wdl` - Self-contained WDL 1.2: version, sayHello task, helloWorld workflow
- `wdl/testdata/lib.wdl` - WDL 1.2 import target: version, greet task
- `wdl/testdata/main.wdl` - WDL 1.2: imports lib.wdl, main workflow calling greet
- `wdl/testdata/types.wdl` - WDL 1.2: Address/Person structs, processData task with Array[String]+, Map[String,Int], Pair[Boolean,File]

## Decisions Made
- `parseFile` takes optional `src []byte` parameter: non-nil for root (caller provides content), nil for imported files (read from disk via os.ReadFile)
- Circular import chain is reconstructed by passing `chain []string` alongside `visited map[string]bool` — visited enables fast O(1) cycle detection, chain enables human-readable error messages
- `parseDocumentWithImports` returns the importStatement list to the caller rather than resolving inline, keeping single-file parsing pure and enabling testability of the parse-without-resolve path

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed workflow input parsing to allow optional default values**
- **Found during:** Task 2 (WDL fixtures and integration tests)
- **Issue:** `parseDecl(false)` in workflow input block rejected `= expr` syntax; WDL 1.2 allows workflow inputs to have optional defaults (e.g., `String who = "World"`)
- **Fix:** Changed `parseDecl(false)` to `parseDecl(true)` for workflow input sections; `parseDecl(true)` makes `= expr` optional (only parsed if `=` token is present)
- **Files modified:** wdl/parser/parser.go
- **Verification:** TestParseDocument/hello_wdl_fixture passes; all other tests still pass
- **Committed in:** ea466b3 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 - Bug)
**Impact on plan:** Fix was necessary for correctness — hello.wdl fixture would not parse without it. No scope creep.

## Issues Encountered
None beyond the auto-fixed bug above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 2 (Lexer, Parser, and AST) is now complete: all 5 plans implemented and tested
- parser.Parse is the stable public API for Phase 3 onwards
- Any WDL 1.2 file with version, tasks, workflows, structs, and local imports can be parsed into ast.Document

---
*Phase: 02-lexer-parser-and-ast*
*Completed: 2026-03-10*

## Self-Check: PASSED

All created files verified on disk. All task commits verified in git log.
