# Phase 2: Lexer, Parser, and AST - Context

**Gathered:** 2026-03-10
**Status:** Ready for planning

<domain>
## Phase Boundary

Implement the lexer, parser, and AST for WDL 1.2. Any valid WDL 1.2 source file can be parsed into a typed, traversable `ast.Document`. Parse errors are reported with precise source locations (rustc-style). No type checking, evaluation, or execution — this phase delivers the structural representation that Phase 3 (type checker) and Phase 4 (evaluator) build on.

</domain>

<decisions>
## Implementation Decisions

### Parser approach

- **Hand-written lexer + Pratt parser** (decided at project init) — Rob Pike state-function pattern for the lexer; Pratt sub-parser for expressions with correct operator precedence including `**` (exponentiation)
- No parser generator (goyacc, participle, etc.) — no viable WDL 1.2 grammar exists for Go tools

### Error handling style

- **Stop at first error** — report the first bad token and return; no error recovery or multi-error collection
- **Custom `ParseError` struct** in `wdl/parser` package, implementing the `error` interface
  - Fields: `File string`, `Line int`, `Col int`, `Message string`, `SourceLine string`
  - Callers can type-assert to extract structured position info
- **Full rustc-style display**: source line + caret pointing to error column
  ```
  error: unexpected token
    --> file.wdl:12:5
     |
  12 | workflow foo {
     |     ^
  ```
- `ParseError` lives in `wdl/parser` package (not `wdl/ast`)

### Import resolution model

- **Single flat `ast.Document`** — the parser recursively resolves imports and merges all task/workflow/struct definitions from all imported files into one `ast.Document`
- Type checker and evaluator see one document; no need to traverse a tree of linked documents
- **Relative imports resolve from the directory of the importing file** (matches miniwdl behavior)
- **Circular imports detected and rejected** — track visited file paths during recursive parsing; report `ParseError` with message like `circular import: a.wdl → b.wdl → a.wdl`
- **Duplicate names are an error** — if two imported files define a task/struct with the same name, report a `ParseError`

### Source position on AST nodes

- **All nodes carry a `Pos` field** — every AST struct embeds `Pos` directly (not via interface)
  ```go
  type Pos struct {
      File string // absolute path to source file
      Line int
      Col  int
  }
  ```
- `Pos` embedded as a field in each struct (e.g., `type TaskDef struct { Pos Pos; Name string; ... }`)
- Direct field access at call sites — no `Node` interface with `Pos()` method
- `File` field enables per-node tracking of origin file in multi-file (imported) documents

### AST package layout

- Package: `wdl/ast` — mirrors miniwdl's `WDL/Tree.py` naming where applicable
- Top-level entry point: `ast.Document` holds version, imports (resolved), tasks, workflows, structs
- Package split: `wdl/lexer` (tokenizer), `wdl/parser` (parser + `ParseError`), `wdl/ast` (node structs)

### Claude's Discretion

- Exact set of AST node types beyond the top-level declarations (expression node variants, statement nodes within tasks/workflows)
- Token type enumeration and lexer state machine internals
- Exact fields on each AST node struct (beyond what's implied by the WDL spec)
- How command block content (template parts + placeholders) is represented in AST
- Test file organization (unit per package, or shared WDL fixture files)

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets

- `wdl/types` package: `types.Type` interface + all concrete type structs (`IntType`, `FloatType`, `BooleanType`, `StringType`, `FileType`, `DirectoryType`, `ArrayType`, `MapType`, `PairType`, `StructType`, `AnyType`) — AST nodes for typed declarations will carry `types.Type` fields
- `wdl/values` package: value structs — not directly used by parser/AST, but downstream phases import it alongside `wdl/ast`

### Established Patterns

- Naming mirrors miniwdl: `wdl/ast` nodes should mirror `WDL/Tree.py` class names where there's a natural Go equivalent
- Reference implementation for AST shape: `../miniwdl/WDL/Tree.py` (task, workflow, scatter, conditional, call, declaration nodes)
- Reference for grammar: `../miniwdl/WDL/_grammar.py` (lark PEG grammar — use as spec, not as code)
- Value structs use explicit suffix (`IntValue`, etc.) — AST nodes should use descriptive names without suffix confusion

### Integration Points

- `wdl/ast` consumed by: Phase 3 type checker (walks declarations), Phase 4 expression evaluator (walks expressions), Phase 5 task runner (reads task input/runtime/command), Phase 6 workflow scheduler (reads workflow call graph)
- `wdl/parser` entry point: `parser.Parse(filename string, src []byte) (*ast.Document, error)` — return type `error` is always a `*ParseError` on failure

</code_context>

<specifics>
## Specific Ideas

- Error display should look like rustc output — the user has this mental model for what "good" parse errors look like
- `Pos.File` as absolute path ensures cross-file import errors are unambiguous

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 02-lexer-parser-and-ast*
*Context gathered: 2026-03-10*
