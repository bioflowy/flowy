---
phase: 02-lexer-parser-and-ast
plan: "01"
subsystem: ast
tags: [wdl, ast, lexer, parser, go, types]

# Dependency graph
requires:
  - phase: 01-type-data-structures
    provides: "types.Type interface and all concrete WDL type structs used in Decl.Type field"
provides:
  - "wdl/ast package with all WDL AST node structs (Document, Task, Workflow, Decl, StructDef, Call, Scatter, Conditional)"
  - "Expr interface and all 17 expression node types (BoolLit, IntLit, FloatLit, NullLit, StringExpr, CommandExpr, ArrayExpr, MapExpr, PairExpr, StructExpr, IfThenElse, BinaryExpr, UnaryExpr, IndexExpr, GetExpr, ApplyExpr, IdentExpr)"
  - "WorkflowNode and StringPart marker interfaces with all implementations"
  - "Pos struct with File/Line/Col for per-node source location tracking"
  - "Wave 0 Nyquist test stubs in wdl/lexer and wdl/parser (all t.Skip)"
affects:
  - "02-02 (lexer): imports wdl/ast for token position, defines lexer.TokenType matching ast.TokenType alias"
  - "02-03 (parser top-level): constructs all Document/Task/Workflow/Decl/StructDef/Call/Scatter/Conditional nodes"
  - "02-04 (Pratt parser): constructs all Expr node types"
  - "03 (type checker): walks ast.Document to resolve and check types"
  - "04 (evaluator): walks Expr nodes"
  - "05 (task runner): reads Task.Command, Task.Runtime, Task.Requirements"
  - "06 (workflow scheduler): reads Workflow.Body call graph"

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Marker interface pattern: WorkflowNode and StringPart interfaces use empty isX() methods"
    - "TokenType = int alias prevents circular import between wdl/ast and wdl/lexer"
    - "Direct Pos embedding (not pointer) in every struct for zero-allocation position access"
    - "Wave 0 Nyquist: package stubs + t.Skip test scaffolds ensure go test compiles before implementation"

key-files:
  created:
    - wdl/ast/ast.go
    - wdl/ast/expr.go
    - wdl/ast/ast_test.go
    - wdl/lexer/lexer.go
    - wdl/lexer/lexer_test.go
    - wdl/parser/parser.go
    - wdl/parser/parser_test.go
  modified: []

key-decisions:
  - "TokenType = int alias (not defined type) in wdl/ast/expr.go avoids circular import: lexer imports ast; ast cannot import lexer"
  - "Pos embedded as plain field (not pointer) in every struct: zero cost, direct field access without nil checks"
  - "WorkflowNode and StringPart are marker interfaces (empty method), not type enums: allows open extensibility"
  - "CommandStyle is a typed int const (iota) not a string enum: exhaustive switch checking at compile time"

patterns-established:
  - "Marker interface pattern: interface { isX() } with unexported method prevents external implementation"
  - "All Expr implementations use pointer receivers: func (*T) exprNode() {}"
  - "Pos.File is absolute path — callers must resolve relative paths before constructing Pos"

requirements-completed:
  - PARSE-01
  - PARSE-02
  - PARSE-03
  - PARSE-04
  - PARSE-05
  - PARSE-06
  - PARSE-07
  - PARSE-08
  - PARSE-09

# Metrics
duration: 3min
completed: 2026-03-10
---

# Phase 2 Plan 01: AST Node Structs and Wave 0 Test Stubs Summary

**WDL 1.2 AST node structs (17 Expr types, 8 statement types) and Wave 0 Nyquist test stubs in wdl/lexer and wdl/parser**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-10T06:41:28Z
- **Completed:** 2026-03-10T06:44:21Z
- **Tasks:** 2
- **Files modified:** 7 created

## Accomplishments

- Defined all WDL AST statement node types: Document, Task, Workflow, Decl, StructDef, Call, Scatter, Conditional — each with embedded Pos (File/Line/Col)
- Defined Expr interface and all 17 expression node types covering all WDL 1.2 expression forms
- Defined WorkflowNode and StringPart marker interfaces with all implementations; CommandStyle int constants
- Created Wave 0 Nyquist scaffolding: empty package stubs + t.Skip test functions for all 9 PARSE requirements, ensuring `go test ./wdl/lexer/... ./wdl/parser/...` compiles from day one

## Task Commits

Each task was committed atomically:

1. **Task 1: RED - AST struct compilation tests** - `aeb104a` (test)
2. **Task 1: GREEN - AST package implementation** - `0355a98` (feat)
3. **Task 2: Wave 0 Nyquist test stubs** - `f93f765` (feat)

_Note: Task 1 followed TDD pattern (test commit + implementation commit)_

## Files Created/Modified

- `wdl/ast/ast.go` - Document, Task, Workflow, Decl, StructDef, Call, Scatter, Conditional, Pos, WorkflowNode, StringPart, RawStringPart, PlaceholderPart, CommandStyle
- `wdl/ast/expr.go` - Expr interface, TokenType alias, and all 17 expression node types
- `wdl/ast/ast_test.go` - Compilation tests for all exported types and interfaces
- `wdl/lexer/lexer.go` - Empty package stub (implementation in Plan 02-02)
- `wdl/lexer/lexer_test.go` - t.Skip stubs for PARSE-02, PARSE-03, PARSE-04
- `wdl/parser/parser.go` - Empty package stub (implementation in Plans 02-03, 02-04)
- `wdl/parser/parser_test.go` - t.Skip stubs for PARSE-01, PARSE-05 through PARSE-09

## Decisions Made

- `TokenType = int` alias (not a defined type) in `wdl/ast/expr.go` avoids circular import: the lexer will import wdl/ast for token positions, so ast cannot import lexer for the TokenType definition
- `Pos` embedded as plain struct field (not pointer) in every AST node: zero allocation overhead, direct field access, no nil guard needed
- Marker interfaces (`WorkflowNode`, `StringPart`, `Expr`) use unexported methods to prevent external packages from accidentally implementing them

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `wdl/ast` package compiles cleanly and exports all required types for Plans 02-02 through 02-05
- Wave 0 test stubs in `wdl/lexer` and `wdl/parser` compile; each subsequent plan replaces the t.Skip stubs with real implementations
- Decl.Type references `types.Type` from Phase 1; the import link is verified by the build
- No blockers for Plan 02-02 (lexer implementation)

## Self-Check: PASSED

All files present and all commits verified:
- wdl/ast/ast.go: FOUND
- wdl/ast/expr.go: FOUND
- wdl/lexer/lexer.go: FOUND
- wdl/lexer/lexer_test.go: FOUND
- wdl/parser/parser.go: FOUND
- wdl/parser/parser_test.go: FOUND
- Commit aeb104a (test RED): FOUND
- Commit 0355a98 (feat GREEN): FOUND
- Commit f93f765 (feat Task 2): FOUND

---
*Phase: 02-lexer-parser-and-ast*
*Completed: 2026-03-10*
