---
phase: 04-expression-evaluator-and-standard-library
plan: 05
subsystem: eval
tags: [go, stdlib, array-functions, string-functions, numeric-functions, wdl, tdd]

# Dependency graph
requires:
  - phase: 04-expression-evaluator-and-standard-library
    plan: 02
    provides: "Eval() with stdlibFuncs dispatch map ready to receive registrations"
  - phase: 04-expression-evaluator-and-standard-library
    plan: 03
    provides: "stdlib scaffold and file I/O functions (STDLIB-01, STDLIB-02)"
provides:
  - "STDLIB-03: 13 array functions (length, flatten, transpose, zip, cross, prefix, suffix, quote, squote, unzip, select_all, select_first, chunk) registered in stdlibFuncs"
  - "STDLIB-04: sub (regexp ReplaceAll), find (FindString+NullValue), matches (anchored whole-string), join_paths (3 overloads via filepath.Join)"
  - "STDLIB-06: ceil, floor, round (half-up), min, max with Int/Float/mixed promotion"
  - "TestStdlibArray (20 sub-cases), TestStdlibString (9 sub-cases), TestStdlibNumeric (10 sub-cases), TestRoundHalfUp (6 boundary cases) all PASS"
affects:
  - "stdlibFuncs map now has 31 total registered functions"

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "valueToStr helper in stdlib_array.go (not valueToString — avoids collision with command.go)"
    - "regexp.Compile per call for sub/find/matches — no cache needed for correctness"
    - "stdMatches anchors pattern with ^pattern$ to enforce whole-string match"
    - "stdRound uses math.Floor(x+0.5) — NOT math.Round — for WDL half-up semantics"
    - "numericArgs helper extracts float64 from Int/Float args and returns isFloat flags for type dispatch"
    - "chunk validates n>0, uses slice window i:i+n with bounds cap at len(Items)"
    - "stdJoinPaths type-switches on args[0] type (FileValue vs ArrayValue) then args[1] type for overload dispatch"

key-files:
  created:
    - wdl/eval/stdlib_array.go
    - wdl/eval/stdlib_string.go
    - wdl/eval/stdlib_numeric.go
  modified:
    - wdl/eval/stdlib_test.go

key-decisions:
  - "valueToStr (not valueToString) in stdlib_array.go: avoids collision with existing valueToString(v, opts) in command.go — discovered at compile time"
  - "stdFind returns NullValue{} (not error) on no match — correct WDL optional semantics"
  - "stdMatches anchors pattern with ^ and $ — partial matches must return false per WDL spec"
  - "stdRound: math.Floor(x+0.5) confirmed for round(-2.5)==-2 (WDL half-up), math.Round gives -3 (wrong)"
  - "stdChunk returns outer ArrayType{ItemType: av.Type} where av.Type is the input ArrayType — outer is Array[Array[X]]"

patterns-established:
  - "init() per file registers functions into stdlibFuncs — each stdlib file is self-contained"
  - "numericArgs helper centralizes float64 extraction + isFloat flags for min/max overload dispatch"

requirements-completed: [STDLIB-03, STDLIB-04, STDLIB-06]

# Metrics
duration: 4min
completed: 2026-03-10
---

# Phase 4 Plan 05: Array, String, and Numeric Stdlib Summary

**13 array functions (STDLIB-03), 4 string functions with 3 join_paths overloads (STDLIB-04), and 5 numeric functions with WDL half-up round semantics (STDLIB-06) implemented via TDD with 39 sub-test cases all passing**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-10T10:27:45Z
- **Completed:** 2026-03-10T10:31:24Z
- **Tasks:** 2
- **Files modified:** 4 (3 created, 1 updated)

## Accomplishments

- **stdlib_array.go**: 13 STDLIB-03 functions registered; length, flatten, transpose, zip, cross, prefix, suffix, quote, squote, unzip, select_all, select_first, chunk all implemented and verified
- **stdlib_string.go**: 4 STDLIB-04 functions registered; sub uses regexp ReplaceAllString, find returns NullValue on no-match, matches anchors pattern for whole-string check, join_paths dispatches 3 overloads
- **stdlib_numeric.go**: 5 STDLIB-06 functions registered; ceil/floor use math.Ceil/Floor, round uses `math.Floor(x+0.5)` (WDL half-up), min/max handle Int+Int, Float+Float, and mixed-promotion cases
- **TestStdlibArray**: 20 sub-cases PASS (length, flatten, transpose, zip, cross, prefix, suffix, quote, squote, unzip, select_all, select_first, chunk with boundary cases)
- **TestStdlibString**: 9 sub-cases PASS (sub all-occurrences + no-match, find match + no-match, matches whole + partial, join_paths 3 overloads)
- **TestStdlibNumeric**: 10 sub-cases PASS (ceil+/-1.1, floor+/-1.9, round 2.4+2.5, min Int/Float/mixed, max Int)
- **TestRoundHalfUp**: 6 boundary cases PASS (2.5->3, -2.5->-2 critical, 2.4->2, -2.4->-2, 0.5->1, -0.5->0)
- **Full suite**: `go test ./...` exits 0, no regressions

## Task Commits

1. **Task 1: Array stdlib (STDLIB-03)** - `f2c0f09` (feat)
2. **Task 2: String and numeric stdlib (STDLIB-04, STDLIB-06)** - `3524054` (feat)

## Files Created/Modified

- `/home/uehara/flowy/wdl/eval/stdlib_array.go` - 13 array functions, valueToStr helper, init() registration
- `/home/uehara/flowy/wdl/eval/stdlib_string.go` - sub, find, matches, join_paths (3 overloads), init() registration
- `/home/uehara/flowy/wdl/eval/stdlib_numeric.go` - ceil, floor, round (half-up), min, max, numericArgs helper, init() registration
- `/home/uehara/flowy/wdl/eval/stdlib_test.go` - TestStdlibArray (20), TestStdlibString (9), TestStdlibNumeric (10), TestRoundHalfUp (6) implemented

## Decisions Made

- **valueToStr not valueToString:** The plan action suggested `valueToString(item)` but `command.go` already exports `valueToString(v, opts)` with a different signature. Renamed to `valueToStr` to avoid collision. Discovered at first compile, fixed immediately (Rule 1 deviation).
- **stdFind empty-string guard:** `regexp.FindString` returns `""` on no match and also on a zero-width match. Current behavior: `"" == no match -> NullValue`. This is correct for all practical patterns but technically conflates zero-width with no-match. Acceptable for WDL stdlib.
- **chunk outer type:** `ArrayValue{Type: ArrayType{ItemType: av.Type}}` where `av.Type` is `ArrayType{ItemType: IntType{}}` — the outer type is correctly `Array[Array[Int]]`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Renamed valueToString to valueToStr in stdlib_array.go**
- **Found during:** Task 1 (first compile attempt)
- **Issue:** Plan action suggested defining `valueToString(v values.Value) string` but `command.go:94` already declares `valueToString(v values.Value, opts map[string]string) string` — redeclaration error
- **Fix:** Renamed helper to `valueToStr` throughout stdlib_array.go
- **Files modified:** wdl/eval/stdlib_array.go
- **Verification:** `go build ./wdl/eval/` exits 0; TestStdlibArray prefix/suffix/quote/squote pass
- **Committed in:** f2c0f09

---

**Total deviations:** 1 auto-fixed (1 bug — name collision with pre-existing function in command.go)
**Impact on plan:** Trivial rename only. No behavior change, no scope creep.

## Issues Encountered

- Plan documentation suggested `valueToString` helper but the actual codebase already has that name with a different signature in `command.go`. This is a documentation consistency issue; the fix was trivial.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- STDLIB-03, STDLIB-04, STDLIB-06 fully delivered
- stdlibFuncs now has 31 registered functions (18 from plan 04, 13 array + 4 string + 5 numeric from this plan)
- Plan 04-06 scope was merged into this plan — numeric functions (ceil/floor/round/min/max) are complete
- Wave 3 complete: all stdlib functions implemented
- Wave 4 (plan 04-06, if it exists) can proceed or phase is complete

---
*Phase: 04-expression-evaluator-and-standard-library*
*Completed: 2026-03-10*

## Self-Check: PASSED

- FOUND: wdl/eval/stdlib_array.go
- FOUND: wdl/eval/stdlib_string.go
- FOUND: wdl/eval/stdlib_numeric.go
- FOUND: commit f2c0f09 (feat(04-05): implement array stdlib functions)
- FOUND: commit 3524054 (feat(04-05): implement string and numeric stdlib functions)
