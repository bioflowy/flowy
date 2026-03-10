---
phase: 02-lexer-parser-and-ast
plan: "02"
subsystem: lexer
tags: [wdl, lexer, tokenizer, go, state-machine, command-block, placeholders]

# Dependency graph
requires:
  - phase: 02-lexer-parser-and-ast
    plan: "01"
    provides: "wdl/ast package with Pos struct, TokenType alias, AST node types, and Wave 0 test stubs"
provides:
  - "wdl/lexer package: full WDL 1.2 tokenizer with Token, TokenType, Lexer, NewLexer, Next()"
  - "All WDL token types: keywords, operators, punctuation, command-mode tokens, string delimiters"
  - "modeNormal, modeCommandBrace, modeCommandHeredoc, modePlaceholder, modeString scanning states"
  - "Command brace mode: both ~{ and ${ emit TokPlaceholderOpen"
  - "Command heredoc mode: only ~{ is special; ${} passes through as raw TokCommandContent"
  - "Nested brace depth tracking: inner } does not close outer placeholder"
  - "TokStarStar emitted for **: prevents ** → TokStar TokStar split"
  - "Line/col tracking: every token carries ast.Pos{File, Line, Col}"
affects:
  - "02-03 (parser top-level): calls lexer.NewLexer, reads tokens via Next(), checks all TokXxx type constants"
  - "02-04 (Pratt parser): maps TokXxx operators to BinaryExpr.Op, UnaryExpr.Op"
  - "02-05 (parser integration): full parse pipeline uses lexer as token stream source"

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Rob Pike state-function pattern: stateFn = func(*Lexer) stateFn; each mode is a separate stateFn"
    - "Pull-model lexer: Next() drives state transitions synchronously (no goroutines, no channels)"
    - "Mode stack: []lexMode tracks re-entrant placeholder sub-lexing with pushMode/popMode"
    - "Direct byte comparison in hot path: no regexp; isIdentStart/isDigit inline helpers"
    - "Auto command-mode detection: after 'command' keyword is emitted, peek ahead for { or <<< to set mode"
    - "braceDepth int counter prevents premature TokPlaceholderClose inside nested expressions"

key-files:
  created: []
  modified:
    - wdl/lexer/lexer.go
    - wdl/lexer/lexer_test.go
    - wdl/lexer/lexer_compile_test.go

key-decisions:
  - "Pull-model (Next()) not push-model (goroutine+channel): simpler, no goroutine leak risk, easy to pause"
  - "Auto command-mode detection in lexer (after 'command' keyword) rather than parser calling SetCommandMode(): reduces parser complexity"
  - "braceDepth=0 reset on each TokPlaceholderOpen entry: each placeholder has independent depth tracking"
  - "tokenizeAll helper exported as package-level function (lowercase) in external test package, not in production code: keeps API surface clean"

patterns-established:
  - "TokXxx naming (exported) for all token type constants so external test package (lexer_test) can access them"
  - "Wave 0 t.Skip stubs are replaced entirely (not edited) when real tests are written"
  - "Table-driven tests with named subtests and check func(t, tokens) closures for flexible assertion"

requirements-completed:
  - PARSE-02
  - PARSE-03
  - PARSE-04

# Metrics
duration: 3min
completed: 2026-03-10
---

# Phase 2 Plan 02: WDL 1.2 Lexer Implementation Summary

**Rob Pike state-function lexer with three command modes (normal/brace/heredoc), nested brace depth tracking, and comprehensive table-driven tests for PARSE-02, PARSE-03, PARSE-04**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-10T06:47:17Z
- **Completed:** 2026-03-10T06:50:38Z
- **Tasks:** 2
- **Files modified:** 3 (lexer.go rewritten, lexer_test.go replaced, lexer_compile_test.go created)

## Accomplishments

- Implemented the full WDL 1.2 tokenizer: all keyword, operator, punctuation, and command-mode token types; 9 test cases across 7 test functions pass
- Command brace mode (`command { }`) correctly emits both `~{` and `${` as `TokPlaceholderOpen`, enabling PARSE-03
- Command heredoc mode (`command <<< >>>`) correctly emits only `~{` as `TokPlaceholderOpen`; `${HOME}` becomes raw `TokCommandContent`, satisfying the PARSE-04 invariant
- Brace depth tracking ensures nested `}` inside a placeholder does not prematurely close the outer placeholder

## Task Commits

Each task was committed atomically:

1. **Task 1: RED - compilation test** - `9145551` (test)
2. **Task 1: GREEN - full lexer implementation** - `aab4e1d` (feat)
3. **Task 2: real table-driven tests replacing Wave 0 stubs** - `fd41b94` (feat)

_Note: Task 1 followed TDD pattern (test commit + implementation commit)_

## Files Created/Modified

- `wdl/lexer/lexer.go` - Full lexer implementation: TokenType consts, Token struct, Lexer with Next(), stateFn for all 5 modes
- `wdl/lexer/lexer_test.go` - Replaced Wave 0 t.Skip stubs with TestCommandModes, TestCommandBracePlaceholders, TestCommandHeredocPlaceholders, TestNestedPlaceholder, TestStarStar, TestLineCol, TestDeclarationTokenization
- `wdl/lexer/lexer_compile_test.go` - TDD RED compilation test (verifies exported API surface)

## Decisions Made

- Pull-model `Next()` API rather than goroutine+channel: avoids goroutine leaks, makes it easy to pause/resume from parser
- Auto command-mode detection inside the lexer (after emitting `TokCommand`): simplifies the parser (no `SetCommandMode` call needed)
- `braceDepth` reset to 0 on each `TokPlaceholderOpen` entry: each placeholder starts fresh, preventing cross-placeholder interference
- Token constant names use `TokXxx` prefix (exported) so the external `lexer_test` package can reference them without an internal alias

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `wdl/lexer` package compiles cleanly; all 9 tests pass; `go vet` clean
- `Next()` pull API is ready for the parser (Plans 02-03, 02-04) to consume tokens
- All three command modes are implemented and tested; parser can rely on `TokCommandContent`, `TokPlaceholderOpen`, `TokPlaceholderClose`, `TokHeredocOpen`, `TokHeredocClose`
- `TokStarStar` prevents `**` from being misread as two `*` operators by the Pratt parser
- No blockers for Plan 02-03 (parser top-level) or 02-04 (Pratt expression parser)

## Self-Check: PASSED

---
*Phase: 02-lexer-parser-and-ast*
*Completed: 2026-03-10*
