---
phase: 04-expression-evaluator-and-standard-library
verified: 2026-03-10T00:00:00Z
status: passed
score: 13/13 must-haves verified
re_verification: false
---

# Phase 4: Expression Evaluator and Standard Library — Verification Report

**Phase Goal:** Implement a complete WDL expression evaluator (wdl/eval package) covering all 14 expression types and a full standard library of 40+ functions spanning file I/O, filesystem, array, string, numeric, and type conversion operations.
**Verified:** 2026-03-10
**Status:** PASSED
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| #  | Truth                                                                              | Status     | Evidence                                                                          |
|----|------------------------------------------------------------------------------------|------------|-----------------------------------------------------------------------------------|
| 1  | wdl/eval package compiles with zero errors                                         | VERIFIED   | `go build ./wdl/eval/` exits 0, no output                                        |
| 2  | Eval() function accepts all 14 ast.Expr types without panic                        | VERIFIED   | Full type-switch in eval.go covers all 14 cases; default returns EvalError        |
| 3  | Arithmetic/comparison/logical operators return correctly typed Values (EXPR-01)    | VERIFIED   | TestEvalBinary: 29 subtests PASS including int+float promotion, ** -> Float       |
| 4  | if-then-else evaluates only the taken branch (EXPR-02)                             | VERIFIED   | TestEvalIfThenElse: 3 subtests PASS                                               |
| 5  | struct.field and pair.left/right return correct value or EvalError (EXPR-03)       | VERIFIED   | TestEvalGetExpr: 6 subtests PASS                                                  |
| 6  | array[i] and map[k] index access return value or EvalError (EXPR-04)              | VERIFIED   | TestEvalIndexExpr: 7 subtests PASS                                                |
| 7  | ApplyExpr dispatches through evalApply() to stdlibFuncs (EXPR-05)                 | VERIFIED   | TestEvalApply: 4 subtests PASS; unknown function returns EvalError not panic      |
| 8  | CommandExpr and StringExpr evaluate with placeholder substitution (EXPR-06)        | VERIFIED   | TestEvalCommand: 5 subtests PASS                                                  |
| 9  | Sentinel-before-indent dedent algorithm correct (EXPR-07)                          | VERIFIED   | TestCommandDedent + TestCommandDedentEdgeCases: 23 subtests PASS                 |
| 10 | All 12 file I/O functions registered and tested (STDLIB-01)                       | VERIFIED   | TestStdlibFileIO: 20 subtests PASS including read/write round-trips with tempdir  |
| 11 | All 6 filesystem functions registered and tested (STDLIB-02)                      | VERIFIED   | TestStdlibFilesystem: 11 subtests PASS; glob/stdout/stderr EvalError on empty ctx |
| 12 | All 14 array functions + 4 string functions + 5 numeric functions (STDLIB-03/04/06)| VERIFIED   | TestStdlibArray: 20 PASS; TestStdlibString: 9 PASS; TestStdlibNumeric: 10 PASS   |
| 13 | All 8 utility functions registered and tested (STDLIB-05)                         | VERIFIED   | TestStdlibUtility: 11 subtests PASS; TestRoundHalfUp: PASS (round(-2.5)==-2)     |

**Score:** 13/13 truths verified

---

### Required Artifacts

| Artifact                        | Provides                                                    | Status     | Details                                                              |
|---------------------------------|-------------------------------------------------------------|------------|----------------------------------------------------------------------|
| `wdl/eval/eval.go`              | Eval() entry point, all 14 type-switch cases, helpers       | VERIFIED   | 503 lines; all cases implemented, primitiveEqual helper present      |
| `wdl/eval/env.go`               | Env linked-list runtime scope with Lookup/Define            | VERIFIED   | NewEnv, Define, Lookup all exported and correct                      |
| `wdl/eval/error.go`             | EvalError with Pos, mirrors checker.TypeError               | VERIFIED   | EvalError exported with Error() string method                        |
| `wdl/eval/context.go`           | ExecContext with WorkDir, StdoutPath, StderrPath            | VERIFIED   | All three fields present, exported                                   |
| `wdl/eval/stdlib.go`            | stdlibFn type, stdlibFuncs map, evalApply()                 | VERIFIED   | Map initialized empty; evalApply dispatches with EvalError on miss   |
| `wdl/eval/command.go`           | evalCommand, evalString, valueToString, dedent helpers      | VERIFIED   | 197 lines; sentinel algorithm, computeMinIndent, applyDedentAndSub   |
| `wdl/eval/stdlib_fileio.go`     | 18 file I/O + filesystem functions via init()               | VERIFIED   | All 18 functions registered in init(); os.MkdirTemp used for writes  |
| `wdl/eval/stdlib_array.go`      | 13 array functions via init()                               | VERIFIED   | All 13 registered; chunk validates n>0                               |
| `wdl/eval/stdlib_string.go`     | 4 string functions via init()                               | VERIFIED   | sub, find, matches, join_paths all registered                        |
| `wdl/eval/stdlib_numeric.go`    | 5 numeric functions via init()                              | VERIFIED   | ceil, floor, round (half-up), min, max registered                    |
| `wdl/eval/stdlib_util.go`       | 8 utility functions via init()                              | VERIFIED   | defined, keys, values, as_map, as_pairs, collect_by_key, contains, contains_key |
| `wdl/eval/eval_test.go`         | Tests for EXPR-01 through EXPR-07 (no t.Skip)              | VERIFIED   | All test functions implemented and passing                           |
| `wdl/eval/stdlib_test.go`       | Tests for STDLIB-01 through STDLIB-06 (no t.Skip)          | VERIFIED   | All test functions implemented and passing                           |
| `wdl/eval/command_test.go`      | TestCommandDedent, TestCommandDedentEdgeCases               | VERIFIED   | All edge cases pass including whitespace-only line exclusion          |

---

### Key Link Verification

| From                          | To                              | Via                                        | Status  | Details                                                               |
|-------------------------------|---------------------------------|--------------------------------------------|---------|-----------------------------------------------------------------------|
| `eval.go Eval()`              | `wdl/ast` all 14 Expr types     | type-switch on concrete types              | WIRED   | All 14 cases present including StructExpr (17th line of switch)       |
| `eval.go ApplyExpr case`      | `stdlib.go evalApply()`         | direct call `evalApply(e, env, ctx)`       | WIRED   | Line 123 of eval.go                                                   |
| `evalApply()`                 | `stdlibFuncs[e.Name]`           | map lookup with EvalError on miss          | WIRED   | Unknown function returns *EvalError confirmed by TestEvalApply        |
| `eval.go CommandExpr case`    | `command.go evalCommand()`      | direct function call                       | WIRED   | Line 34 of eval.go                                                    |
| `stdlib_fileio.go init()`     | `stdlib.go stdlibFuncs` map     | init() registration of 18 functions        | WIRED   | All 18 functions registered; TestStdlibCount: 48 total funcs          |
| `stdlib_array.go init()`      | `stdlib.go stdlibFuncs` map     | init() registration of 13 functions        | WIRED   | All 13 array functions registered                                     |
| `stdlib_string.go init()`     | `stdlib.go stdlibFuncs` map     | init() registration of 4 functions         | WIRED   | sub, find, matches, join_paths registered                             |
| `stdlib_numeric.go init()`    | `stdlib.go stdlibFuncs` map     | init() registration of 5 functions         | WIRED   | ceil, floor, round, min, max registered                               |
| `stdlib_util.go init()`       | `stdlib.go stdlibFuncs` map     | init() registration of 8 functions         | WIRED   | All 8 utility functions registered                                    |
| `stdWriteLines`               | `os.MkdirTemp`                  | real filesystem write per locked decision  | WIRED   | os.MkdirTemp("", "wdl-write-*") used in all write functions          |
| `stdGlob`                     | `ExecContext.WorkDir`           | guard: WorkDir == "" -> EvalError          | WIRED   | Guard on line 447 of stdlib_fileio.go; test confirms EvalError        |
| `evalGet`                     | `values.StructValue.Fields`     | map lookup by e.Member                     | WIRED   | evalGet in eval.go uses v.Fields[e.Member] with ok check              |
| `evalIndex`                   | `values.MapValue.Entries`       | linear scan with primitiveEqual            | WIRED   | primitiveEqual used in evalIndex and stdContains/stdContainsKey       |
| `stdRound`                    | `math.Floor(x + 0.5)`          | WDL half-up: NOT math.Round                | WIRED   | stdlib_numeric.go line 58; TestRoundHalfUp confirms round(-2.5)==-2  |

---

### Requirements Coverage

| Requirement | Source Plans    | Description                                                                    | Status    | Evidence                                                         |
|-------------|-----------------|--------------------------------------------------------------------------------|-----------|------------------------------------------------------------------|
| EXPR-01     | 04-01, 04-02    | Arithmetic, comparison, logical operators including **                         | SATISFIED | TestEvalBinary 29 subtests PASS; evalBinary, evalArith helpers   |
| EXPR-02     | 04-01, 04-02    | if-then-else expression evaluation                                             | SATISFIED | TestEvalIfThenElse 3 subtests PASS                               |
| EXPR-03     | 04-01, 04-02    | Member access: struct.field, pair.left, pair.right                            | SATISFIED | TestEvalGetExpr 6 subtests PASS; evalGet helper                  |
| EXPR-04     | 04-01, 04-02    | Array/map index access                                                         | SATISFIED | TestEvalIndexExpr 7 subtests PASS; evalIndex helper              |
| EXPR-05     | 04-01, 04-06    | Function call evaluation via stdlib dispatch                                   | SATISFIED | TestEvalApply 4 subtests PASS; full chain through evalApply      |
| EXPR-06     | 04-01, 04-03    | Command template expansion with placeholder substitution                       | SATISFIED | TestEvalCommand 5 subtests PASS; evalCommand in command.go       |
| EXPR-07     | 04-01, 04-03    | Sentinel-before-indent dedent algorithm                                        | SATISFIED | TestCommandDedent + TestCommandDedentEdgeCases 23 subtests PASS  |
| STDLIB-01   | 04-04           | read_lines, read_tsv, read_map, read_json, read_boolean, read_int, read_float, read_string, write_lines, write_tsv, write_map, write_json | SATISFIED | TestStdlibFileIO 20 subtests PASS; os.MkdirTemp for writes       |
| STDLIB-02   | 04-04           | glob, basename, dirname, size, stdout, stderr                                  | SATISFIED | TestStdlibFilesystem 11 subtests PASS                            |
| STDLIB-03   | 04-05           | length, flatten, transpose, zip, cross, prefix, suffix, quote, squote, unzip, select_all, select_first, chunk | SATISFIED | TestStdlibArray 20 subtests PASS                                 |
| STDLIB-04   | 04-05           | sub, find, matches, join_paths (3 overloads)                                  | SATISFIED | TestStdlibString 9 subtests PASS                                 |
| STDLIB-05   | 04-06           | defined, keys, values, as_map, as_pairs, collect_by_key, contains, contains_key | SATISFIED | TestStdlibUtility 11 subtests PASS                               |
| STDLIB-06   | 04-05           | ceil, floor, round (half-up), min, max                                         | SATISFIED | TestStdlibNumeric 10 subtests PASS; TestRoundHalfUp PASS         |

All 13 requirements satisfied. No orphaned requirements.

---

### Anti-Patterns Found

None. Scanned all 14 files in `wdl/eval/` for TODO, FIXME, t.Skip, placeholder returns, and empty implementations. No issues detected.

---

### Human Verification Required

None. All behaviors are deterministic and fully verified by automated tests.

---

### Summary

Phase 4 is fully complete. The wdl/eval package delivers:

- **14 expression types** implemented in eval.go with a full type-switch (BoolLit, IntLit, FloatLit, NullLit, StringExpr, CommandExpr, ArrayExpr, MapExpr, PairExpr, StructExpr, IfThenElse, BinaryExpr, UnaryExpr, IndexExpr, GetExpr, ApplyExpr, IdentExpr — note: GetExpr and ApplyExpr count as 2 of the 14 from spec, with IdentExpr also included in the switch).
- **48 stdlib functions** registered across 4 stdlib files, exceeding the 40+ target.
- **Sentinel-before-indent algorithm** correctly implemented and verified with edge cases (whitespace-only line exclusion, empty parts, multi-line values).
- **WDL half-up round** correctly implemented as `math.Floor(x + 0.5)` — confirmed round(-2.5)==-2.
- **Full test suite** `go test ./... -count=1` exits 0 across all 7 packages with zero regressions.
- All write functions use `os.MkdirTemp` per locked design decision.
- All context-sensitive functions (glob, stdout, stderr) return EvalError when ExecContext fields are empty.

---

_Verified: 2026-03-10_
_Verifier: Claude (gsd-verifier)_
