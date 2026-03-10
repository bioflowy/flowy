---
phase: 02-lexer-parser-and-ast
verified: 2026-03-10T07:22:08Z
status: passed
score: 9/9 must-haves verified
re_verification: false
---

# Phase 2: Lexer, Parser, and AST — Verification Report

**Phase Goal:** Any valid WDL 1.2 source file can be parsed into a typed, traversable AST; parse errors are reported with precise source locations
**Verified:** 2026-03-10T07:22:08Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | All WDL AST node structs compile without error | VERIFIED | `go build ./wdl/ast/...` exits 0; `ast_test.go` TestASTStructsCompile passes |
| 2 | Every AST struct embeds a Pos field with File, Line, Col | VERIFIED | `wdl/ast/ast.go` lines 12-16: `Pos struct { File string; Line int; Col int }`; all node structs embed `Pos` directly (not pointer) |
| 3 | Expr interface is defined and all 17 expression node types implement it | VERIFIED | `wdl/ast/expr.go`: `Expr` interface with `exprNode()` marker; 17 types verified: BoolLit, IntLit, FloatLit, NullLit, StringExpr, CommandExpr, ArrayExpr, MapExpr, PairExpr, StructExpr, IfThenElse, BinaryExpr, UnaryExpr, IndexExpr, GetExpr, ApplyExpr, IdentExpr |
| 4 | Lexer tokenizes command-brace and command-heredoc modes correctly | VERIFIED | TestCommandModes, TestCommandBracePlaceholders, TestCommandHeredocPlaceholders all pass; `lexCommandBrace` and `lexCommandHeredoc` stateFns confirmed in `lexer.go` lines 603-728 |
| 5 | `~{}` and `${}` are placeholders in brace mode; only `~{}` in heredoc mode | VERIFIED | TestCommandHeredocPlaceholders/dollar_is_NOT_a_placeholder_in_heredoc_(PARSE-04_invariant) passes; `${HOME}` emitted as `TokCommandContent` |
| 6 | Pratt parser produces correct operator precedence including right-associative `**` | VERIFIED | TestOperatorPrecedence/exponentiation_right_assoc passes; `peekInfix` uses leftBP=60, rightBP=59 for `TokStarStar` |
| 7 | String/command interpolation Parts lists correctly interleave raw and placeholder segments | VERIFIED | TestStringInterpolation passes with 3 subtests: simple placeholder, command brace, heredoc dollar-as-raw |
| 8 | ParseError has File, Line, Col, SourceLine fields and rustc-style Error() output | VERIFIED | `parser.go` lines 16-41: `ParseError` struct with `-->` arrow, source line, `^` caret formatting |
| 9 | Import resolution merges tasks/workflows/structs recursively; circular and duplicate names produce ParseError | VERIFIED | TestImportResolution/merge_imported_tasks, circular_import_detection, duplicate_task_name_detection all pass; `parseFile` with visited map and chain slice confirmed |

**Score:** 9/9 truths verified

### Required Artifacts

| Artifact | Status | Details |
|----------|--------|---------|
| `wdl/ast/ast.go` | VERIFIED | 147 lines; exports Document, Task, Workflow, Decl, StructDef, Call, Scatter, Conditional, Pos, WorkflowNode, StringPart, RawStringPart, PlaceholderPart, CommandStyle |
| `wdl/ast/expr.go` | VERIFIED | 174 lines; exports Expr interface, TokenType alias, and all 17 expression node types |
| `wdl/lexer/lexer.go` | VERIFIED | 795 lines; exports Token, TokenType, Lexer, NewLexer; all three modes (normal, command-brace, command-heredoc) implemented |
| `wdl/lexer/lexer_test.go` | VERIFIED | No `t.Skip` calls; 18 test functions covering PARSE-02, PARSE-03, PARSE-04 pass |
| `wdl/parser/parser.go` | VERIFIED | 1253 lines; exports Parse, ParseError; parseFile with recursive import resolution, circular/duplicate detection |
| `wdl/parser/parse_expr.go` | VERIFIED | 628 lines; full Pratt parser with prattExpr, parsePrefixExpr, peekInfix, parseStringExpr, parsePlaceholderPart |
| `wdl/parser/parse_types.go` | VERIFIED | 161 lines; parseType returns types.Type for all WDL type syntax including Array[T]+? |
| `wdl/parser/parser_test.go` | VERIFIED | No `t.Skip` calls; all 9 PARSE requirements covered by passing tests |
| `wdl/testdata/hello.wdl` | VERIFIED | 23 lines; version 1.2, sayHello task with command block using `~{}` placeholder, helloWorld workflow |
| `wdl/testdata/lib.wdl` | VERIFIED | 13 lines; import target with greet task |
| `wdl/testdata/main.wdl` | VERIFIED | 6 lines; imports lib.wdl, workflow calling greet |
| `wdl/testdata/types.wdl` | VERIFIED | 23 lines; Address/Person structs, processData task with Array[String]+, Map[String,Int], Pair[Boolean,File] |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `wdl/ast/ast.go` | `wdl/types` | `import; Decl.Type field is types.Type` | WIRED | `import "github.com/bioflowy/flowy/wdl/types"` confirmed; `Decl.Type types.Type` at line 98 |
| `wdl/ast/expr.go` | `wdl/ast/ast.go` | `same package; Expr nodes embed ast.Pos` | WIRED | `Pos` embedded directly in all 17 Expr types; `TokenType = int` alias prevents circular import |
| `wdl/lexer/lexer.go` | `wdl/ast/ast.go` | `import ast; Token.Pos is ast.Pos` | WIRED | `"github.com/bioflowy/flowy/wdl/ast"` imported at line 11; `Token.Pos ast.Pos` confirmed |
| `wdl/lexer/lexer.go` | `lexCommandBrace / lexCommandHeredoc` | `stateFn returned after 'command' keyword` | WIRED | Lines 551-558: after TokCommand emission, lexer peeks for `{` or `<<<` and calls pushMode |
| `wdl/parser/parser.go` | `wdl/lexer/lexer.go` | `Parser wraps lexer.Lexer; tokens consumed via peek/advance` | WIRED | `import "github.com/bioflowy/flowy/wdl/lexer"`; Parser struct holds `*lexer.Lexer` |
| `wdl/parser/parser.go` | `wdl/ast/ast.go` | `returns *ast.Document, builds Task/Workflow/StructDef/Decl` | WIRED | Parse() returns `(*ast.Document, error)`; all node constructors confirmed |
| `wdl/parser/parse_types.go` | `wdl/types` | `parseType returns types.Type` | WIRED | `import "github.com/bioflowy/flowy/wdl/types"`; returns concrete types.BooleanType, IntType, ArrayType etc. |
| `wdl/parser/parse_expr.go` | `wdl/ast/expr.go` | `returns ast.Expr; builds BinaryExpr, UnaryExpr, etc.` | WIRED | Builds `ast.BinaryExpr`, `ast.UnaryExpr`, `ast.IdentExpr` confirmed in parse_expr.go |
| `wdl/parser/parser.go (Parse)` | `parseFile` | `Parse calls parseFile with empty visited map` | WIRED | Line 78-88: `Parse` calls `parseFile(abs, src, map[string]bool{}, []string{})` |
| `wdl/parser/parser.go (parseFile)` | `path/filepath + os.ReadFile` | `import resolution reads files from disk` | WIRED | `filepath.Abs`, `filepath.Join`, `filepath.Dir`, `os.ReadFile` all confirmed at lines 91-131 |

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|----------------|-------------|--------|----------|
| PARSE-01 | 02-01, 02-03, 02-05 | WDL 1.2 document parses into complete AST | SATISFIED | TestParseDocument (hello_wdl_fixture) and TestImportFromFixture pass; doc.Version=="1.2", task+workflow verified |
| PARSE-02 | 02-01, 02-02 | Lexer handles command{} and command<<<>>> as separate modes | SATISFIED | TestCommandModes passes; modeCommandBrace and modeCommandHeredoc stateFns implemented |
| PARSE-03 | 02-01, 02-02 | `~{}` and `${}` are placeholders in command{} mode | SATISFIED | TestCommandBracePlaceholders passes; both emit TokPlaceholderOpen |
| PARSE-04 | 02-01, 02-02 | Only `~{}` is placeholder in command<<<>>> mode; `${}` passes through as raw | SATISFIED | TestCommandHeredocPlaceholders/dollar_is_NOT_a_placeholder passes |
| PARSE-05 | 02-01, 02-04 | String interpolation with nested placeholders parsed correctly | SATISFIED | TestStringInterpolation passes; Parts list alternates RawStringPart/PlaceholderPart |
| PARSE-06 | 02-01, 02-04 | All operators parse with correct precedence; `**` is right-associative | SATISFIED | TestOperatorPrecedence/exponentiation_right_assoc: 2**3**2 → 2**(3**2) |
| PARSE-07 | 02-01, 02-05 | Local file imports resolved recursively into single flat AST | SATISFIED | TestImportResolution/merge_imported_tasks passes; TestImportFromFixture (main.wdl→lib.wdl) passes |
| PARSE-08 | 02-01, 02-03 | Parse errors reported with line, col, source line, caret | SATISFIED | TestParseError passes; `-->` and `^` present in Error() output |
| PARSE-09 | 02-01, 02-03 | Struct definitions including nested struct type references parse correctly | SATISFIED | TestStructDef/nested_struct_type_reference passes; Member[0].Type is types.StructType{Name:"MyStruct"} |

All 9 requirement IDs from PLAN frontmatter are accounted for. No orphaned requirements found in REQUIREMENTS.md.

### Anti-Patterns Found

No anti-patterns found. Specifically verified:

- Zero `t.Skip` calls in any test file under `wdl/lexer/` or `wdl/parser/`
- Zero `__expr_stub__` occurrences in any parser source file
- Zero `TODO`, `FIXME`, `XXX`, or `HACK` comments in key implementation files
- Zero empty handler bodies (`return nil`, `return {}`) in exported functions
- All 18 test function calls in lexer and parser packages result in PASS (no SKIP, no FAIL)

### Human Verification Required

The following items cannot be verified programmatically:

#### 1. ParseError Caret Alignment

**Test:** Deliberately trigger a parse error at column 5 of a line, capture `err.Error()`, verify the `^` caret appears at position 5.
**Expected:** The `^` caret in the rustc-style output aligns precisely with the column of the error token, not just "somewhere on the line."
**Why human:** Requires reading the formatted string and visually or manually counting spaces before `^`.

#### 2. Import Alias Struct Renaming

**Test:** Create a WDL file that imports another with `alias OldName as NewName`, parse it, and inspect whether struct references inside the imported document are renamed in the merged AST.
**Expected:** `StructType{Name: "NewName"}` appears in the merged document; `OldName` is absent.
**Why human:** No automated test in the current suite covers the alias-rename path end-to-end through merge. The parsing of the alias syntax is tested but not the rename application across the merged doc.

## Summary

Phase 2 goal is fully achieved. The entire pipeline — from raw WDL 1.2 source bytes to a typed, traversable `*ast.Document` — is implemented and tested:

- The `wdl/ast` package provides all required node types with embedded source position
- The `wdl/lexer` package correctly handles three tokenization modes with per-token line/col tracking
- The `wdl/parser` package provides the public `Parse()` entry point, recursive import resolution, circular and duplicate name detection, and rustc-style error reporting
- The Pratt expression parser handles all WDL 1.2 operators at correct precedence with right-associative `**`
- All 9 PARSE requirements are satisfied by passing tests; 0 skips remain

The full test suite (`go test ./wdl/...`) and race detector (`go test -race ./wdl/...`) pass clean. `go vet ./wdl/...` is clean.

---

_Verified: 2026-03-10T07:22:08Z_
_Verifier: Claude (gsd-verifier)_
