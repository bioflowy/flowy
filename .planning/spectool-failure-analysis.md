# Spectool Conformance Test Failure Analysis

**Date:** 2026-03-11
**Baseline:** 45/160 tests passing

## Summary

All 10 investigated failures are flowy bugs or missing features. None are spec-side issues.

| # | Test | Category | Root Cause | Impact |
|---|------|----------|------------|--------|
| 1 | hello.wdl | Missing feature | Shorthand call syntax not parsed | ~10+ tests |
| 2 | optionals.wdl | Missing feature | `None` keyword not recognized | ~8+ tests |
| 3 | test_pairs.wdl | Bug | Pair `.left`/`.right` rejected by type checker | ~2+ tests |
| 4 | nested_placeholders.wdl | Missing feature | Single-quoted strings not supported | ~1+ tests |
| 5 | test_struct.wdl | Bug | Output block can't reference earlier declarations in same block | ~1+ tests |
| 6 | if_else.wdl | Bug | Conditional call outputs not wrapped as Optional | ~3+ tests |
| 7 | test_map_ordering.wdl | Bug | False circular dependency in scatter | ~1+ tests |
| 8 | test_cross.wdl | Missing feature | Array equality comparison not implemented | ~2+ tests |
| 9 | test_length.wdl | Missing feature | `length()` only accepts Array, not Map/String | ~1+ tests |
| 10 | (all task tests) | Bug | CLI `--task` flag ignored after positional arg | ~30+ task tests |

---

## Issue 1: WDL 1.2 Shorthand Call Syntax

**Tests:** hello.wdl, workflow_with_comments.wdl, test_scatter.wdl, test_conditional.wdl, ternary.wdl, test_range.wdl, is_defined.wdl, test_contains.wdl, etc.

**Error:**
```
expected token type 70, got 3 ("infile")
  --> hello.wdl:30:7
   | call hello_task { infile, pattern }
```

**Root Cause:** `wdl/parser/parser.go:1062-1095` (`parseCall()`)

The parser requires `input:` keyword inside `{ }` blocks. WDL 1.2 allows shorthand: `call task { input1, input2 }` is equivalent to `call task { input: input1 = input1, input2 = input2 }`.

Current code at line 1065 checks `if p.peek().Type == lexer.TokInput` and skips input parsing otherwise.

**Category:** Missing feature

---

## Issue 2: `None` Keyword Not Supported

**Tests:** optionals.wdl, non_empty_optional.wdl, test_select_first.wdl, compare_optionals.wdl, placeholder_coercion.wdl, placeholder_none.wdl, etc.

**Error:**
```
undefined name "None"
  --> optionals.wdl:11:7
   | String? also_maybe_five_but_is_not = None
```

**Root Cause:** `wdl/lexer/lexer.go:116-153` (keywords map)

The lexer has `TokNull` mapped to `"null"`, but no `TokNone` mapped to `"None"`. WDL 1.2 uses `None` as the explicit null literal for optional types. The lexer tokenizes `None` as `TokIdent`, so the evaluator treats it as an undefined variable.

**Category:** Missing feature

**Fix:** Add `TokNone` token, map `"None"` in keywords, handle in parser and evaluator to produce NullValue.

---

## Issue 3: Pair `.left`/`.right` Field Access Rejected by Type Checker

**Tests:** test_pairs.wdl

**Error:**
```
field access on non-struct type Pair[Int,Array[String]]
  --> test_pairs.wdl:8:5
   | Int five = data.left
```

**Root Cause:** `wdl/checker/struct_check.go:116-166` (`checkGetExpr()`)

The type checker only accepts `types.StructType` for field access (line 136). Pair field access `.left`/`.right` is rejected. However, the evaluator at `wdl/eval/eval.go:357-365` (`evalGet`) DOES correctly handle `values.PairValue` with "left" and "right" — the type checker blocks execution before reaching it.

**Category:** Bug (type checker too restrictive)

**Fix:** Extend `checkGetExpr` to accept `types.PairType` and validate "left"/"right" fields.

---

## Issue 4: Single-Quoted Strings Not Supported

**Tests:** nested_placeholders.wdl, concat_optional.wdl, test_sep.wdl

**Error:**
```
unexpected character: '
  --> nested_placeholders.wdl:11:29
   | String s = "~{if b then '~{1 + i}' else '0'}"
```

**Root Cause:** `wdl/lexer/lexer.go:379-385` (string handling)

The lexer only recognizes `"` (double quote) as string delimiters. WDL 1.2 allows single-quoted strings (`'...'`), which are particularly useful inside string interpolations to avoid escaping.

**Category:** Missing feature

**Fix:** Add `case '\''` handling in `lexNormal` to enter string lexing mode with single-quote delimiter.

---

## Issue 5: Output Block Cannot Reference Earlier Declarations

**Tests:** test_struct.wdl

**Error:**
```
undefined variable "john"
  --> test_struct.wdl:29:35
   | Boolean has_account = defined(john.account)
```

**WDL code:**
```wdl
output {
    Person john = Person { name: "John", account: BankAccount { ... } }
    Boolean has_account = defined(john.account)  # references john above
}
```

**Root Cause:** `wdl/checker/checker.go` (output validation scope management)

Declarations within the output block are not properly registered in scope for later declarations in the same block. The `validateGroups` function (lines 313-375) builds a dependency graph but doesn't make earlier output declarations visible to later ones.

**Category:** Bug (scope management)

**Fix:** Ensure output section declarations are added to the validation scope sequentially so later declarations can reference earlier ones.

---

## Issue 6: Conditional Call Outputs Not Wrapped as Optional

**Tests:** if_else.wdl, nested_if.wdl, test_conditional.wdl

**Error:**
```
cannot access field "greeting" on values.NullValue
  --> if_else.wdl:34:44
   | String greeting = select_first([morning.greeting, afternoon.greeting])
```

**WDL code:**
```wdl
if (is_morning) {
    call greet as morning { input: time = "morning" }
}
if (!is_morning) {
    call greet as afternoon { input: time = "afternoon" }
}
output {
    String greeting = select_first([morning.greeting, afternoon.greeting])
}
```

**Root Cause:** `wdl/eval/eval.go` (conditional evaluation)

When a call is inside an unexecuted conditional branch, accessing its output (`morning.greeting`) should return a NullValue that can be placed in an array for `select_first()`. Instead, the field access `.greeting` on NullValue throws an error. The type checker at `checker.go:215-226` correctly promotes conditional call outputs to Optional type, but the evaluator doesn't handle field access on NullValue gracefully.

**Category:** Bug (conditional execution semantics)

**Fix:** `evalGet` should return NullValue when accessing a field on NullValue (propagate null), rather than throwing an error.

---

## Issue 7: False Circular Dependency in Scatter

**Tests:** test_map_ordering.wdl

**Error:**
```
cycle: circular dependency among declarations: [i ints]
```

**WDL code:**
```wdl
scatter (ints in as_pairs(int_to_int)) {
    Array[Int] i = [ints.left, ints.right]
}
output {
    Array[Array[Int]] ints = i
}
```

**Root Cause:** `wdl/checker/checker.go:313-352` (`validateGroups`)

The dependency graph treats the scatter variable `ints` (from `scatter (ints in ...)`) and the output declaration `ints` as the same name, creating a false cycle. The scatter variable `ints` is a loop variable, not a declaration, and the output `ints = i` references `i` (the scatter body result), not the scatter variable `ints`.

**Category:** Bug (incorrect cycle detection)

**Fix:** Exclude scatter variables from peer declaration cycle detection, or use fully qualified names to distinguish scatter variables from output declarations.

---

## Issue 8: Array Equality Comparison Not Implemented

**Tests:** test_cross.wdl, test_zip.wdl, test_flatten.wdl, array_map_equality.wdl

**Error:**
```
cannot compare values.ArrayValue and values.ArrayValue
```

**Root Cause:** `wdl/eval/eval.go:239-315` (`evalComparison`)

The comparison function handles Int, Float, Boolean, and String, but not Array, Map, or Pair. WDL spec requires that compound types support equality comparison (element-wise for arrays, entry-wise for maps).

**Category:** Missing feature

**Fix:** Add cases for `values.ArrayValue`, `values.MapValue`, and `values.PairValue` in `evalComparison` with recursive element-wise comparison.

---

## Issue 9: `length()` Only Accepts Array

**Tests:** test_length.wdl

**Error:**
```
length: expected ArrayValue, got values.MapValue
```

**Root Cause:** `wdl/eval/stdlib_array.go:48-58` (`stdLength`)

The function type-asserts to `values.ArrayValue` only (line 53-54). WDL spec requires `length()` to work on:
- `Array[X]` → number of elements
- `Map[K,V]` → number of key-value pairs
- `String` → number of characters (WDL 1.2)

**Category:** Missing feature

**Fix:** Add `case values.MapValue` (return `len(m.Entries)`) and `case values.StringValue` (return `len(s.Value)`) to `stdLength`.

---

## Issue 10: CLI `--task` Flag Ignored After Positional Arg

**Tests:** All spectool task-level tests (~30+ tests)

**Error:**
```
no workflow defined in sum_task.wdl (use --task to run a task)
```

**Context:** spectool runs `flowy run --input inputs.json path.wdl --task taskname` — the `--task` flag comes AFTER the positional `path.wdl` argument.

**Root Cause:** `cmd/flowy/main.go:137-158` (`runRun`)

Go's `flag.FlagSet` stops parsing flags when it encounters a positional argument. Flags after the positional arg are silently ignored. The command line `flowy run --input inputs.json path.wdl --task taskname` parses `--input` correctly but treats `--task taskname` as additional positional args.

**Category:** Bug (CLI argument parsing)

**Fix:** Pre-process args to extract the WDL file path and re-order flags before positional args, or switch to a CLI library that supports mixed flag/positional ordering (e.g., pflag).

---

## Priority Ranking by Test Impact

1. **Issue 10 (CLI arg ordering)** — ~30+ task tests. Single fix, huge impact.
2. **Issue 1 (Shorthand call syntax)** — ~10+ workflow tests. Parser enhancement.
3. **Issue 2 (None keyword)** — ~8+ tests. Lexer/parser/evaluator change.
4. **Issue 6 (Conditional call outputs)** — ~3+ tests. Evaluator fix.
5. **Issue 8 (Array equality)** — ~2+ tests. Evaluator enhancement.
6. **Issue 3 (Pair field access)** — ~2+ tests. Type checker fix.
7. **Issue 4 (Single-quoted strings)** — ~1+ tests. Lexer enhancement.
8. **Issue 9 (length on Map)** — ~1+ tests. Small stdlib fix.
9. **Issue 5 (Output scope)** — ~1+ tests. Scope management fix.
10. **Issue 7 (Scatter cycle)** — ~1+ tests. Cycle detection fix.
