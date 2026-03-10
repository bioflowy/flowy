---
phase: 02-lexer-parser-and-ast
plan: "04"
subsystem: parser
tags: [wdl, parser, pratt, ast, expression, operator-precedence, string-interpolation, go]

# Dependency graph
requires:
  - phase: 02-lexer-parser-and-ast
    plan: "03"
    provides: "wdl/parser package with recursive descent scaffold and parseExpr stub"
  - phase: 02-lexer-parser-and-ast
    plan: "02"
    provides: "wdl/lexer package with all token types (TokXxx), TokStarStar, placeholder tokens"
provides:
  - "wdl/parser/parse_expr.go: full Pratt expression parser (parseExpr, prattExpr, parsePrefixExpr, peekInfix, parseStringExpr, parsePlaceholderPart, parseArrayExpr, parseMapExpr, parseIfThenElse, parseIdentOrCallOrStruct)"
  - "Correct operator precedence: || < && < comparisons < +- < */ % < ** (right-associative)"
  - "String interpolation: StringExpr.Parts with interleaved RawStringPart and PlaceholderPart"
  - "Placeholder options map (sep/true/false/default) parsed for both string and command blocks"
  - "TestOperatorPrecedence and TestStringInterpolation replace t.Skip stubs (PARSE-05, PARSE-06)"
affects:
  - "02-05 (parser integration): import resolution uses parseExpr for all expressions"
  - "03 (type checker): consumes fully-typed ast.Expr trees from Parse()"
  - "04 (evaluator): walks BinaryExpr/UnaryExpr/IdentExpr/ApplyExpr/StringExpr nodes"
  - "05 (task runner): command block PlaceholderPart expressions fully parsed"

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Pratt precedence-climbing with (leftBP, rightBP) binding power table"
    - "Right-associativity via rightBP < leftBP: ** uses leftBP=60, rightBP=59"
    - "Unary lower than ** via rightBP=55 (between */ BP=50 and ** BP=60)"
    - "Break condition: leftBP <= minBP for left-assoc; rightBP=leftBP-1 for right-assoc"
    - "Postfix (index/member) handled outside main Pratt loop at highest priority"
    - "isNameToken() used in expression context to accept keyword names as identifiers"

key-files:
  created:
    - wdl/parser/parse_expr.go
  modified:
    - wdl/parser/parser.go
    - wdl/parser/parser_test.go
    - wdl/lexer/lexer.go

key-decisions:
  - "** right-associativity: leftBP=60, rightBP=59 (not leftBP=rightBP=60 as RESEARCH.md noted); with loop condition leftBP <= minBP, this correctly makes 2**3**2 → 2**(3**2)"
  - "Unary minus/not rightBP=55: lower than ** leftBP=60 so -x**2 → -(x**2), matching Python/math convention"
  - "parseExpr variadic delimiters preserved for backward compat with parser.go call sites; Pratt terminates naturally at non-infix tokens"
  - "isNameToken expanded to include TokEnv so 'env' keyword is valid as a variable name in expression context (e.g., ${env} in command blocks)"

patterns-established:
  - "Pratt parser: prattExpr(minBP) calls parsePrefixExpr for lhs, then loops on peekInfix binding powers"
  - "String interpolation: lexer emits TokStringOpen/TokCommandContent/TokPlaceholderOpen/TokPlaceholderClose/TokStringClose; parser collects into Parts slice"
  - "Placeholder options (sep/true/false/default) parsed as IDENT=VALUE before the expression token"

requirements-completed:
  - PARSE-05
  - PARSE-06

# Metrics
duration: 9min
completed: 2026-03-10
---

# Phase 2 Plan 04: Pratt Expression Parser Summary

**Pratt precedence-climbing expression parser in parse_expr.go: full WDL 1.2 operator precedence with right-associative ** exponentiation, string/command interpolation Parts list, and parseExpr stub replaced**

## Performance

- **Duration:** 9 min
- **Started:** 2026-03-10T07:02:19Z
- **Completed:** 2026-03-10T07:11:36Z
- **Tasks:** 2
- **Files modified:** 4 (parse_expr.go created, parser.go + parser_test.go + lexer.go modified)

## Accomplishments

- Implemented the full Pratt expression sub-parser in `parse_expr.go`: all WDL 1.2 expression forms (literals, unary, binary, index, member access, function call, struct literal, array, map, pair, if-then-else, string interpolation)
- Binding power table with correct precedence: `||`(10) < `&&`(20) < comparisons(30) < `+/-`(40) < `*/%`(50) < `**`(60, right-assoc via rightBP=59)
- String interpolation: `parseStringExpr` and `parsePlaceholderPart` produce `StringExpr.Parts` with interleaved `RawStringPart` and `PlaceholderPart`; placeholder options map parsed
- TestOperatorPrecedence (PARSE-06) and TestStringInterpolation (PARSE-05) replace t.Skip stubs; 7 total test cases all pass

## Task Commits

Each task was committed atomically:

1. **Task 1: Pratt expression parser implementation** - `9de066b` (feat)
2. **Task 2: Operator precedence and string interpolation tests + bug fixes** - `56e26d5` (feat)

## Files Created/Modified

- `wdl/parser/parse_expr.go` - New file: full Pratt expression parser implementation
- `wdl/parser/parser.go` - Stub parseExpr removed; isNameToken expanded with TokEnv
- `wdl/parser/parser_test.go` - TestOperatorPrecedence and TestStringInterpolation implemented
- `wdl/lexer/lexer.go` - Removed broken negative-number shortcut causing infinite loop

## Decisions Made

- `**` right-associativity implemented via `rightBP=59 < leftBP=60` (explicit asymmetry), not `leftBP=rightBP=60` — the RESEARCH.md notation was ambiguous about the loop condition direction
- Unary minus/not use `rightBP=55` (between `*/` at 50 and `**` at 60) so `-x**2 → -(x**2)` matches Python/math convention and matches the plan's specified behavior
- `parseExpr` retains variadic `...lexer.TokenType` delimiters parameter for backward compat with all call sites in `parser.go`; Pratt terminates naturally at non-infix tokens without checking delimiter set
- `isNameToken` expanded with `TokEnv` to allow `env` (a WDL task-input modifier keyword) as a valid variable reference in expressions (needed for `${env}` in command brace blocks)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Lexer infinite loop on negative numbers in expression context (-2**3)**
- **Found during:** Task 2 (TestOperatorPrecedence/unary_minus_lower_than_power)
- **Issue:** `lexNormal` had a shortcut calling `lexNumber` when seeing `-` followed by a digit. But `lexNumber` does not consume the leading `-` (comment in file even notes this). Result: `lexNumber` emits `TokIntLit{""}` without advancing, causing an infinite token loop that produced 1.9GB of output.
- **Fix:** Removed the `b == '-' && isDigit(next)` branch from the `lexNormal` numeric-literal condition. The `-` now correctly emits `TokMinus`, and the Pratt parser's unary minus case handles negative values.
- **Files modified:** wdl/lexer/lexer.go
- **Verification:** All lexer tests still pass; `-2**3` now correctly tokenizes as TokMinus, TokIntLit{"2"}, TokStarStar, TokIntLit{"3"}
- **Committed in:** 56e26d5 (Task 2 commit)

**2. [Rule 1 - Bug] TokEnv not handled in Pratt expression parser**
- **Found during:** Task 2 (TestStringInterpolation/command_brace_placeholder)
- **Issue:** The command block `${env}` causes the lexer to emit `TokEnv` inside the placeholder (since `env` is a keyword). `parsePrefixExpr` had no case for `TokEnv`, triggering `unexpected token "env" in expression`.
- **Fix:** Added `TokEnv` to `isNameToken()` in parser.go. The `default:` branch of `parsePrefixExpr` already checks `isNameToken` and produces an `IdentExpr`, so this was a one-line fix.
- **Files modified:** wdl/parser/parser.go
- **Verification:** TestStringInterpolation/command_brace_placeholder passes; `${env}` produces PlaceholderPart{Expr:IdentExpr{"env"}}
- **Committed in:** 56e26d5 (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (2 Rule 1 bugs)
**Impact on plan:** Both bugs were pre-existing and triggered by the new Pratt parser being exercised. Both fixes necessary for correctness. No scope creep.

## Issues Encountered

- The RESEARCH.md binding power table notation `leftBP=60, rightBP=60` for `**` was ambiguous — whether the loop break condition is `<` or `<=` determines whether this produces left- or right-associativity. Resolved by using `rightBP=59` (explicit asymmetry) with `leftBP <= minBP` break condition, which unambiguously produces right-associativity regardless of the condition variant.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `wdl/parser` package compiles cleanly; all non-skipped tests pass (TestImportResolution still skipped for Plan 05)
- `parseExpr` is now the real Pratt implementation; all call sites in `parser.go` already use it
- Expression trees fully typed: BinaryExpr, UnaryExpr, IdentExpr, ApplyExpr, StringExpr, etc.
- Plan 05 (import resolution) can proceed without any expression-related changes
- Plan 03+ (type checker, evaluator): complete ast.Expr trees available from Parse()

## Self-Check: PASSED
