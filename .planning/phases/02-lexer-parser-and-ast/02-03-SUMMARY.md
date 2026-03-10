---
phase: 02-lexer-parser-and-ast
plan: "03"
subsystem: parser
tags: [wdl, parser, ast, go, recursive-descent, pratt-stub, error-reporting]

# Dependency graph
requires:
  - phase: 02-lexer-parser-and-ast
    plan: "01"
    provides: "wdl/ast package with all AST node types (Document, Task, Workflow, Decl, StructDef, Call, Scatter, Conditional, CommandExpr, Expr)"
  - phase: 02-lexer-parser-and-ast
    plan: "02"
    provides: "wdl/lexer package with all token types (TokXxx), lexer.Lexer Next() API, command-mode tokens"
  - phase: 01-type-data-structures
    provides: "types.Type interface and all concrete WDL type structs (IntType, ArrayType, MapType, etc.)"
provides:
  - "wdl/parser package: Parse(file, src) entry point returning (*ast.Document, error)"
  - "ParseError struct with rustc-style Error() output (file:line:col + source line + caret indicator)"
  - "Parser struct with peek/advance/expect/expectName/errorf helpers wrapping lexer.Lexer"
  - "parseType() in parse_types.go: full WDL type syntax → types.Type including Array[T]+? quantifier"
  - "parseDocument/parseVersion/parseStruct/parseTask/parseWorkflow/parseDecl top-level parsers"
  - "parseCommandBlock: brace and heredoc styles; collects RawStringPart and PlaceholderPart"
  - "parseCall with dotted callee, optional alias/after/input block"
  - "parseScatter and parseConditional with nested workflow body element dispatch"
  - "parseMetaBody/parseMetaValue: JSON-like literal parser for meta/hints/param_meta sections"
  - "parseExpr stub returning ast.IdentExpr{Name:'__expr_stub__'} — replaced by Plan 04 Pratt parser"
affects:
  - "02-04 (Pratt parser): replaces parseExpr stub with real precedence-climbing implementation"
  - "02-05 (parser integration): adds import resolution using Parse as foundation"
  - "03 (type checker): consumes *ast.Document from Parse() for type resolution"
  - "04 (evaluator): walks ast.Expr nodes produced by future Plan 04 Pratt parser"
  - "05 (task runner): reads Task.Command, Task.Runtime, Task.Requirements from parsed AST"
  - "06 (workflow scheduler): reads Workflow.Body call graph from parsed AST"

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Recursive descent parser with one-token lookahead (peek/advance pattern)"
    - "ParseError with rustc-style formatting: error + location arrow + source line + caret"
    - "TDD RED (compile test) → GREEN (full implementation) for both tasks"
    - "Stub parseExpr with depth-tracking delimiter scan — temporary until Plan 04"
    - "expectName() helper accepting type-keyword tokens as valid identifier names in declaration context"

key-files:
  created:
    - wdl/parser/parse_types.go
    - wdl/parser/parser_compile_test.go
  modified:
    - wdl/parser/parser.go
    - wdl/parser/parser_test.go

key-decisions:
  - "parseExpr stub consumes to delimiter with brace/paren/bracket depth tracking; TokError tokens propagated as ParseErrors — enables correct error reporting while Plan 04 is pending"
  - "expectName() accepts type-keyword tokens (Pair, Map, Array, File, etc.) as valid struct/task/workflow names — WDL allows using type keywords as user-defined identifiers"
  - "Workflow input/output sections parsed directly in parseWorkflow loop (not via parseWorkflowElement) — cleaner design; body-only elements dispatched via parseWorkflowBodyElement"
  - "parseType handles Array[T]+? by consuming + and then checking for ? in sequence — handles all four quantifier combinations: none, +, ?, +?"

patterns-established:
  - "Parse errors always wrap *ParseError (not raw errors) for precise source location reporting"
  - "Meta/hints sections use parseMetaBody() producing map[string]any with JSON-like literal values"
  - "Command blocks collect StringPart slices (RawStringPart and PlaceholderPart) directly in parseCommandBlock"
  - "parseWorkflow uses direct token dispatch for sections; parseWorkflowBodyElement handles scatter/conditional/call/decl"

requirements-completed:
  - PARSE-01
  - PARSE-08
  - PARSE-09

# Metrics
duration: 8min
completed: 2026-03-10
---

# Phase 2 Plan 03: Parser Foundation Summary

**Recursive descent parser with rustc-style ParseError, full WDL type syntax parsing, and task/workflow/struct top-level structure; expressions stubbed as ast.IdentExpr{Name:"__expr_stub__"} pending Plan 04 Pratt sub-parser**

## Performance

- **Duration:** 8 min
- **Started:** 2026-03-10T06:52:00Z
- **Completed:** 2026-03-10T07:00:00Z
- **Tasks:** 2
- **Files modified:** 4 (parser.go rewritten, parse_types.go created, 2 test files)

## Accomplishments

- Implemented the full WDL parser foundation: Parse() entry point, ParseError with rustc-style output, all top-level structure parsers (task/workflow/struct/version)
- parseType() in parse_types.go handles all WDL type syntax including compound types (Array/Map/Pair) and all four quantifier combinations (none, +, ?, +?)
- TestParseError, TestStructDef, TestParseTypes, TestParseDocument all pass; PARSE-05/06/07 stubs remain t.Skip for Plans 04/05

## Task Commits

Each task was committed atomically:

1. **Task 1: RED - compilation test** - `8aab927` (test)
2. **Task 1: GREEN - parser scaffolding implementation** - `1bafe7c` (feat)
3. **Task 2: real tests + struct name and error token fixes** - `0e0f38e` (feat)

_Note: Task 1 followed TDD pattern (RED compile test + GREEN implementation). Task 2 also included two auto-fixed bugs discovered during testing._

## Files Created/Modified

- `wdl/parser/parser.go` - Full parser implementation: ParseError, Parser struct, Parse() entry, all parse* functions, parseExpr stub
- `wdl/parser/parse_types.go` - parseType() function handling all WDL type expressions
- `wdl/parser/parser_compile_test.go` - TDD RED compilation test (verifies exported API surface)
- `wdl/parser/parser_test.go` - Replaced Wave 0 t.Skip stubs with TestParseError, TestStructDef, TestParseTypes; updated TestParseDocument

## Decisions Made

- `parseExpr` stub propagates `TokError` tokens as `*ParseError` rather than silently skipping them — this enables TestParseError to work correctly while the Pratt parser is pending
- `expectName()` helper accepts type-keyword tokens (Pair, Map, Array, File, Boolean, etc.) as valid identifier names — WDL users commonly name structs/tasks/workflows using these identifiers
- Workflow input/output sections handled directly in the `parseWorkflow` loop rather than via a multi-return `parseWorkflowElement` — cleaner control flow with no silent data loss

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Type keywords (Pair, Map, Array, etc.) not accepted as struct/task/workflow names**
- **Found during:** Task 2 (TestStructDef simple struct test)
- **Issue:** parseStruct used `expect(TokIdent)` but `struct Pair {}` lexes "Pair" as `TokPair`, causing a parse error for any struct named with a type keyword
- **Fix:** Added `expectName()` helper that accepts `TokIdent` plus all type-keyword token types; updated parseStruct, parseTask, parseWorkflow to use it
- **Files modified:** wdl/parser/parser.go
- **Verification:** TestStructDef/simple_struct passes; struct named "Pair" parses correctly
- **Committed in:** 0e0f38e (Task 2 commit)

**2. [Rule 1 - Bug] parseExpr stub silently consumed TokError tokens without propagating errors**
- **Found during:** Task 2 (TestParseError)
- **Issue:** The stub parseExpr consumed all tokens including TokError without returning an error, so `Int x = $$$` parsed successfully instead of returning a *ParseError
- **Fix:** Added TokError check at the top of the parseExpr loop; consumed TokError tokens are wrapped in `p.errorf()` and returned immediately
- **Files modified:** wdl/parser/parser.go
- **Verification:** TestParseError passes; err is *ParseError with correct Line/Col/SourceLine
- **Committed in:** 0e0f38e (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (2 Rule 1 bugs)
**Impact on plan:** Both fixes necessary for correctness — test discovery was the intended mechanism. No scope creep.

## Issues Encountered

- Workflow element parsing had a broken multi-return design that silently discarded input/output declarations. Refactored `parseWorkflow` to use direct inline dispatch for input/output sections and `parseWorkflowBodyElement` for body nodes. This was identified before committing (caught in code review during implementation).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `wdl/parser` package compiles cleanly; all 7 tests pass, 3 skipped (Plans 04/05)
- Parse() returns *ast.Document for any valid WDL 1.2 document (with expression stubs)
- ParseError provides precise line/col/source-line for all syntax errors
- parseType() is complete and handles all WDL type syntax — Plan 04 does not need to modify it
- parseExpr stub is a clear seam for Plan 04's Pratt expression parser to slot into
- No blockers for Plan 02-04 (Pratt expression parser)

## Self-Check: PASSED

All files present and all commits verified.
