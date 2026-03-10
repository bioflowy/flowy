# Phase 2: Lexer, Parser, and AST - Research

**Researched:** 2026-03-10
**Domain:** Hand-written lexer (Rob Pike state-function), Pratt expression parser, typed AST in Go, WDL 1.2 grammar
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **Parser approach:** Hand-written lexer + Pratt parser. Rob Pike state-function pattern for the lexer. Pratt sub-parser for expressions with correct operator precedence including `**` (exponentiation). No parser generator.
- **Error handling:** Stop at first error. Custom `ParseError` struct in `wdl/parser` package implementing `error`. Fields: `File string`, `Line int`, `Col int`, `Message string`, `SourceLine string`. Full rustc-style display (source line + caret).
- **Import resolution:** Single flat `ast.Document`. Parser recursively resolves imports and merges all task/workflow/struct definitions. Relative imports resolve from directory of importing file. Circular imports detected and rejected. Duplicate names are an error.
- **Source position:** All AST nodes carry `Pos` field embedded directly (not via interface). `Pos` has `File string` (absolute path), `Line int`, `Col int`. Direct field access — no `Node` interface with `Pos()` method.
- **Package layout:** `wdl/lexer` (tokenizer), `wdl/parser` (parser + `ParseError`), `wdl/ast` (node structs). Entry point: `parser.Parse(filename string, src []byte) (*ast.Document, error)`.

### Claude's Discretion

- Exact set of AST node types beyond top-level declarations (expression node variants, statement nodes within tasks/workflows)
- Token type enumeration and lexer state machine internals
- Exact fields on each AST node struct (beyond what's implied by the WDL spec)
- How command block content (template parts + placeholders) is represented in AST
- Test file organization (unit per package, or shared WDL fixture files)

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| PARSE-01 | Parse WDL 1.2 document (version, task, workflow, struct, import) into AST | Full WDL 1.2 grammar documented in miniwdl; top-level node types identified |
| PARSE-02 | Lexer handles `command {}` and `command <<<>>>` in separate modes | Two lexer modes required; grammar confirms `command1` vs `command2` distinction |
| PARSE-03 | Recognize `~{}` and `${}` placeholders in `command {}` blocks | COMMAND1_CHAR regex confirms both delimiters in brace-style |
| PARSE-04 | Recognize only `~{}` in `command <<<>>>` blocks, pass `${}` as bash literal | COMMAND2_CHAR regex confirms tilde-only; heredoc style excludes `${}` |
| PARSE-05 | String interpolation (`~{}` / `${}`) with nesting handled correctly | String parts list pattern (alternating str/Placeholder) handles nesting via recursive parseExpr |
| PARSE-06 | Pratt parser handles all operators including `**` with correct precedence | WDL 1.2 grammar defines 7 precedence levels; `**` not in miniwdl grammar — research shows it must be added above `*` |
| PARSE-07 | Resolve local file imports, produce merged multi-file document | Import merge algorithm: recursive parse + dedup by name; circular detection via visited set |
| PARSE-08 | Report parse errors with line number and column | ParseError struct with File/Line/Col/SourceLine + rustc-style format |
| PARSE-09 | Parse struct definitions including nested structs | `struct` keyword parses body as `unbound_decl*`; nested struct is just a type reference |
</phase_requirements>

---

## Summary

Phase 2 builds the lexer, parser, and AST for WDL 1.2. The grammar is well-defined in the reference implementation (`miniwdl/_grammar.py`) and serves as the authoritative spec — it is a Lark PEG grammar, not intended for use in Go, but its productions map cleanly to a hand-written recursive descent + Pratt hybrid.

The central complexity is the command block lexer: WDL has two syntactically distinct command blocks (`command { ... }` using both `~{}` and `${}` as placeholder delimiters vs `command <<< ... >>>` using only `~{}`) which require the lexer to enter a dedicated mode with different tokenization rules. The Pratt sub-parser for expressions is standard and well-understood; the main gotcha is adding `**` (exponentiation) which the miniwdl lark grammar omits (miniwdl delegates `**` to its Python evaluator).

String interpolation inside both string literals and command blocks is represented using a flat `parts []any` list alternating between `string` and `*Placeholder` elements — mirroring the `Expr.String.parts` pattern from miniwdl. This avoids a complex recursive AST for what is essentially a template.

Import resolution merges all imported tasks/workflows/structs into a single `ast.Document` during parse time. Circular imports and duplicate names are detected with a visited-set (`map[string]bool`) threaded through the recursive `parseFile` function.

**Primary recommendation:** Implement lexer first with modes (normal / command-brace / command-heredoc / string-interp), then top-down recursive descent for statements, then Pratt sub-parser for expressions. The miniwdl `_grammar.py` `versions["1.2"]` block is the exact grammar spec to implement.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go standard library | go 1.24.6 | Lexer, parser, AST, file I/O | No external dependencies needed for a hand-written parser |
| `github.com/bioflowy/flowy/wdl/types` | local | Type structs for declaration nodes | Already implemented in Phase 1 |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `path/filepath` | stdlib | Import path resolution (absolute paths, relative joins) | `filepath.Dir`, `filepath.Abs`, `filepath.Join` |
| `os` | stdlib | Reading imported WDL files | `os.ReadFile` |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Hand-written lexer | `text/scanner` | `text/scanner` is too opinionated about token types; breaks on WDL's context-sensitive command blocks |
| Pratt parser for exprs | Recursive descent with precedence climbing | Equivalent in power; Pratt is simpler to extend when adding `**` at a new precedence level |
| Flat parts list for strings | Recursive string-expr-string AST | Flat list directly mirrors evaluation model; recursive would complicate dedent |

**Installation:** No additional packages needed beyond the existing `go.mod`.

---

## Architecture Patterns

### Recommended Project Structure

```
wdl/
├── lexer/
│   ├── lexer.go          # Token type enum, stateFn type, Lexer struct, Next()
│   └── lexer_test.go     # Table-driven tests per token class
├── ast/
│   ├── ast.go            # All AST node structs: Document, Task, Workflow, Decl, ...
│   └── expr.go           # Expression node structs: BinaryExpr, UnaryExpr, CallExpr, ...
├── parser/
│   ├── parser.go         # Parser struct, Parse() entry point, ParseError
│   ├── parse_expr.go     # Pratt sub-parser for expressions
│   └── parser_test.go    # Table-driven tests using WDL fixture strings
└── testdata/
    └── *.wdl             # WDL fixture files for integration tests
```

### Pattern 1: Rob Pike State-Function Lexer

**What:** The lexer is a state machine where each state is a `func(*Lexer) stateFn`. States return the next state. A goroutine-free variant emits tokens into a slice or is polled via `Next()`.

**When to use:** Always — this is the locked decision. The pattern handles context-sensitive tokenization naturally: the `command1` and `command2` modes are simply different state functions.

**Example (adapted from Go's `text/template` lexer):**
```go
// Source: Rob Pike "Lexical Scanning in Go" (2011) talk, text/template/parse/lex.go
type tokenType int

const (
    tokEOF tokenType = iota
    tokIdent
    tokInt
    tokFloat
    tokString
    tokLBrace
    tokRBrace
    // ...
    tokCommandContent   // raw text inside command block
    tokPlaceholderOpen  // ~{ or ${
)

type Token struct {
    Type tokenType
    Val  string
    Pos  ast.Pos   // reuse ast.Pos
}

type stateFn func(*Lexer) stateFn

type Lexer struct {
    src    []byte
    file   string
    pos    int    // current byte position
    line   int
    col    int
    tokens []Token
}

func (l *Lexer) Next() Token { ... }

func lexNormal(l *Lexer) stateFn {
    // skip whitespace, dispatch on next rune
}

func lexCommandBrace(l *Lexer) stateFn {
    // emit COMMAND1_CHAR runs; on ~{ or ${ → emit tokPlaceholderOpen, push state
}

func lexCommandHeredoc(l *Lexer) stateFn {
    // emit COMMAND2_CHAR runs; on ~{ → emit tokPlaceholderOpen; on >>> → emit close
}
```

**Key insight:** Command block lexer modes are entered by returning a different `stateFn`. Placeholder open `~{` / `${` push the current mode onto a stack and transition to `lexNormal` for the inner expression, then pop back on `}`.

### Pattern 2: Pratt Expression Parser

**What:** Each token type has a binding power (integer). The parser calls `parseExpr(minBP int)` which consumes prefix expressions, then loops consuming infix operators whose left-binding-power exceeds `minBP`.

**When to use:** Always for expression parsing. Handles all WDL operators including `**` (right-associative, highest precedence among binary ops).

**Operator precedence table (WDL 1.2, ascending priority):**

| Operators | Left BP | Right BP | Associativity |
|-----------|---------|----------|---------------|
| `\|\|` | 10 | 11 | left |
| `&&` | 20 | 21 | left |
| `==` `!=` `<` `<=` `>` `>=` | 30 | 31 | left |
| `+` `-` | 40 | 41 | left |
| `*` `/` `%` | 50 | 51 | left |
| `**` | 60 | 60 | right (same right BP causes right-assoc) |
| unary `!` `-` | — | 70 | prefix |

Note: The miniwdl lark grammar does NOT include `**` at the grammar level — it is implemented in the Python evaluator. The WDL 1.2 spec mandates `**` as the exponentiation operator. It must be added above `*`/`/`/`%` and parsed as right-associative (right BP = left BP).

**Example:**
```go
// Pratt parser core — parse expression with minimum binding power
func (p *Parser) parseExpr(minBP int) (ast.Expr, error) {
    lhs, err := p.parsePrefixExpr()
    if err != nil {
        return nil, err
    }
    for {
        op, leftBP, rightBP, ok := p.peekInfix()
        if !ok || leftBP <= minBP {
            break
        }
        p.advance() // consume operator
        rhs, err := p.parseExpr(rightBP)
        if err != nil {
            return nil, err
        }
        lhs = &ast.BinaryExpr{Pos: op.Pos, Op: op.Type, Left: lhs, Right: rhs}
    }
    return lhs, nil
}
```

### Pattern 3: String / Command Parts List

**What:** Strings and command blocks are represented as a slice of alternating raw-string segments and placeholder expressions, mirroring `miniwdl`'s `Expr.String.parts`.

**When to use:** All string literals, command blocks, multi-line strings (`<<<`).

**AST representation:**
```go
// Source: miniwdl/WDL/Expr.py String class
type StringPart interface{ isStringPart() }
type RawStringPart struct{ Val string }
type PlaceholderPart struct {
    Pos     ast.Pos
    Options map[string]string // sep=, true=, false=, default=
    Expr    ast.Expr
}

type StringExpr struct {
    Pos     ast.Pos
    Parts   []StringPart  // first and last are RawStringPart for delimiters
    Command bool          // true if this is a command block
    Style   CommandStyle  // CommandBrace or CommandHeredoc
}
```

**Dedent algorithm (command blocks and `<<<` strings):** Replace each placeholder with a fixed-width sentinel (e.g., `~{}`), then compute minimum indentation across non-blank lines of the pseudo-string, then strip that many leading bytes from each line. This algorithm is applied at evaluation time (Phase 4), not parse time — the AST stores raw parts.

### Pattern 4: Import Merge with Visited Set

**What:** `parseFile` takes a `visited map[string]bool` parameter. Before parsing each file, check if already visited (circular import). After parse, copy tasks/workflows/structs into the parent document, checking for name collisions.

**When to use:** Import resolution inside `parser.Parse`.

```go
func parseFile(path string, visited map[string]bool) (*ast.Document, error) {
    abs, _ := filepath.Abs(path)
    if visited[abs] {
        return nil, &ParseError{File: abs, Message: "circular import: ..."}
    }
    visited[abs] = true
    // ... parse this file ...
    // for each import:
    //   imported, err := parseFile(filepath.Join(dir, importPath), visited)
    //   merge imported.Tasks, imported.Workflows, imported.Structs into doc
    //   check duplicate names
    return doc, nil
}
```

### Anti-Patterns to Avoid

- **Lexer regex matching per token:** Do not use `regexp.MustCompile` inside the hot path. Peek/advance on individual bytes/runes is faster and avoids backtracking surprises at mode boundaries.
- **Sharing token slice between goroutines:** Rob Pike's original uses a channel; for this project, a synchronous pull-model (`Next()`) is simpler and avoids goroutine lifecycle issues.
- **Storing `Pos` as a pointer on AST nodes:** The decision is to embed `Pos` directly in each struct. Pointer would force heap allocation per node.
- **Early type resolution during parsing:** The parser constructs AST nodes with `types.Type` for declared types (parsed from type syntax), but does NOT resolve struct types or validate types. That is Phase 3.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| WDL 1.2 grammar spec | Custom grammar notes | `miniwdl/WDL/_grammar.py` `versions["1.2"]` block | Authoritative, exhaustive, covers all edge cases |
| Dedent algorithm | Custom whitespace trimmer | Port the exact algorithm from `miniwdl/WDL/Expr.py` `String._dedent` | Sentinel-substitution approach handles placeholders correctly; naive approach breaks |
| Operator precedence table | Infer from spec prose | Table above derived from both lark grammar and miniwdl's `_add_parentheses` precedence dict | Already reverse-engineered |

**Key insight:** The hardest part of this phase is not the parser mechanics — it is correctly handling the three lexer modes (normal, command-brace, command-heredoc) and their re-entrant placeholder sub-lexing. Getting command block tokenization wrong will silently produce wrong parse trees.

---

## Common Pitfalls

### Pitfall 1: `${}` as Literal in Heredoc Command Blocks

**What goes wrong:** `${}` is a valid placeholder in `command { }` blocks but is treated as bash literal text in `command <<< >>>` blocks. If the lexer does not distinguish these two modes, `${}` in heredoc commands gets parsed as a WDL placeholder.

**Why it happens:** Single-mode tokenization treats `${` uniformly.

**How to avoid:** Enter `lexCommandBrace` mode on `command {` and `lexCommandHeredoc` mode on `command <<<`. In `lexCommandHeredoc`, the regex `COMMAND2_CHAR` from the grammar (`/[^~>]/ | /~(?=[^{])/ | />(?=[^>])/ | />>(?=[^>])/`) passes `$` through as raw content.

**Warning signs:** Test WDL like `command <<< echo ${HOME} >>>` — `${HOME}` must appear as raw text in the command parts list, not as a placeholder.

### Pitfall 2: Nested Placeholder Re-Entry

**What goes wrong:** A string inside a placeholder can itself contain placeholders: `~{"prefix_~{suffix}"}`. The lexer must track nesting depth of `{...}` while inside a placeholder to know when the outer `}` closes the placeholder vs. is part of an inner expression.

**Why it happens:** Naive lexer counts only the outermost `{` for placeholder close.

**How to avoid:** Maintain a `braceDepth int` counter in the lexer. On each `{` (whether in identifier like `{` in map literals or nested placeholders) increment; on `}` decrement and only emit `tokPlaceholderClose` when `braceDepth` returns to zero.

**Warning signs:** Test `~{"a~{b}c"}` — the inner `}` closes the inner `~{b}` placeholder, not the outer one.

### Pitfall 3: `**` Operator Missing / Wrong Associativity

**What goes wrong:** `2**3**2` should parse as `2**(3**2) = 512`, not `(2**3)**2 = 64`. Right-associativity requires `rightBP == leftBP` for `**` in the Pratt table (breaking the usual `rightBP = leftBP + 1` pattern).

**Why it happens:** The miniwdl lark grammar does not include `**` at the grammar level — implementors may miss it.

**How to avoid:** Add `**` to the Pratt binding power table with `leftBP = 60, rightBP = 60`.

**Warning signs:** Test `2**3**2` — result must parse as right-associative.

### Pitfall 4: Import Path Resolution Relative to Importer

**What goes wrong:** Import `"./tasks.wdl"` in `/project/main.wdl` resolves to `/project/tasks.wdl`. Import `"../shared.wdl"` resolves relative to the importer's directory, not the process cwd.

**Why it happens:** Using `os.Getwd()` or the root file's directory uniformly for all imports.

**How to avoid:** In `parseFile`, compute `dir := filepath.Dir(abs)` and resolve each import as `filepath.Join(dir, importPath)` then call `filepath.Abs` to normalize.

**Warning signs:** Test a two-level import chain: main imports `./sub/a.wdl` which imports `../lib/b.wdl`.

### Pitfall 5: Struct Literal vs. Map Literal Ambiguity

**What goes wrong:** WDL 1.1+ allows struct literals written as `MyStruct { field: value }` (CNAME followed by `{`). In older code and in the current grammar, `{}` also produces map literals. The parser must disambiguate.

**Why it happens:** The grammar rule `CNAME "{" [object_kv ...] "}"` for struct literals looks identical to a function call followed by a block at the token level.

**How to avoid:** After parsing a CNAME in expression context, peek at the next token. If it is `{`, treat it as a struct literal (WDL 1.1+). This is only valid at expression parse time — the CNAME must be followed immediately by `{` with no intervening tokens.

**Warning signs:** Test `MyStruct { x: 1 }` vs `{ x: 1 }` (map literal).

### Pitfall 6: Type Parsing — `Array[T]+?` Quantifiers

**What goes wrong:** The WDL type grammar has three quantifiers: `?` (optional), `+` (nonempty), `+?` (nonempty optional). Parsing `Array[String]+?` requires recognizing `+?` as a unit, not as `+` followed by unary `?`.

**Why it happens:** Greedy tokenization treating `+` and `?` as separate tokens.

**How to avoid:** After parsing a type name and optional type parameters, peek ahead: if `+?` is the next two characters, consume both as `tokOptionalNonempty`. Alternatively, recognize `+?` as a single token in the lexer.

---

## Code Examples

Verified patterns from official/reference sources:

### WDL 1.2 Grammar — All Top-Level Elements

```
// Source: miniwdl/WDL/_grammar.py versions["1.2"]
document: version? document_element*
version: "version" /[^ \t\r\n]+/
?document_element: import_doc | task | workflow | struct

import_doc: "import" string_literal ["as" CNAME] import_alias*
import_alias: "alias" CNAME "as" CNAME

task: "task" CNAME "{" task_section* command task_section* "}"
workflow: "workflow" CNAME "{" workflow_element* "}"
struct: "struct" CNAME "{" (unbound_decl | meta_section)* "}"
```

### WDL 1.2 Grammar — Command Blocks

```
// Source: miniwdl/WDL/_grammar.py versions["1.2"]

// command1: brace style — both ~{ and ${ are placeholders
COMMAND1_CHAR: /[^~$}]/ | /\$(?=[^{])/ | /~(?=[^{])/
COMMAND1_FRAGMENT: COMMAND1_CHAR+
command1: "{" (COMMAND1_FRAGMENT? _EITHER_DELIM placeholder "}")* COMMAND1_FRAGMENT? "}"

// command2: heredoc style — only ~{ is a placeholder; ${ is raw bash text
COMMAND2_CHAR: /[^~>]/ | /~(?=[^{])/ | />(?=[^>])/ | />>(?=[^>])/
COMMAND2_FRAGMENT: COMMAND2_CHAR+
command2: "<<<" (COMMAND2_FRAGMENT? "~{" placeholder "}")* COMMAND2_FRAGMENT? ">>>"
```

### WDL 1.2 Grammar — Operator Precedence (lark encoding)

```
// Source: miniwdl/WDL/_grammar.py versions["1.2"]
// Note: ** is NOT in the lark grammar — added per WDL 1.2 spec above mul/div
?expr_infix0: expr_infix0 "||" expr_infix1 -> lor      // lowest
?expr_infix1: expr_infix1 "&&" expr_infix2 -> land
?expr_infix2: == != <= >= < >                           // comparison
?expr_infix3: + -                                       // additive
?expr_infix4: * / %                                     // multiplicative
// ** is right-assoc above expr_infix4, below unary !
?expr_core: "!" expr_core -> negate                     // unary prefix
```

### miniwdl Precedence Dictionary (for reference)

```python
# Source: miniwdl/WDL/Expr.py _add_parentheses
precedence = {
    "_mul": 7, "_div": 7, "_rem": 7,
    "_add": 6, "_sub": 6,
    "_lt": 5, "_lte": 5, "_gt": 5, "_gte": 5,
    "_eqeq": 4, "_neq": 4,
    "_land": 3,
    "_lor": 3,
}
# ** would slot at 8 (above mul)
```

### AST Node Structure (Go target)

```go
// Source: derived from miniwdl/WDL/Tree.py and CONTEXT.md decisions

// Pos — embedded in every AST node
type Pos struct {
    File string // absolute path
    Line int
    Col  int
}

// Document — top-level result of parsing
type Document struct {
    Pos
    Version   string        // "1.2"
    Tasks     []*Task
    Workflows []*Workflow
    Structs   []*StructDef
    // Imports are resolved; not stored after merge
}

// Task — a WDL task
type Task struct {
    Pos
    Name        string
    Inputs      []*Decl  // nil if no input{} section
    Decls       []*Decl  // non-input intermediate decls
    Command     *CommandExpr
    Outputs     []*Decl
    Runtime     map[string]Expr // or Requirements in WDL 1.2
    Meta        map[string]any
    ParamMeta   map[string]any
}

// Workflow
type Workflow struct {
    Pos
    Name      string
    Inputs    []*Decl
    Body      []WorkflowNode  // Decl | Call | Scatter | Conditional
    Outputs   []*Decl
    Meta      map[string]any
    ParamMeta map[string]any
}

// Decl — a typed declaration (bound or unbound)
type Decl struct {
    Pos
    Type  types.Type
    Name  string
    Expr  Expr // nil if unbound
}

// StructDef — struct type definition
type StructDef struct {
    Pos
    Name    string
    Members []*Decl // unbound_decl only
}

// Call — a call to a task or workflow
type Call struct {
    Pos
    Callee []string        // ["taskname"] or ["ns", "taskname"]
    Alias  string          // from "as NAME"
    After  []string        // from "after NAME" list
    Inputs map[string]Expr // key: input name, value: expression
}

// Scatter
type Scatter struct {
    Pos
    Variable string
    Expr     Expr
    Body     []WorkflowNode
}

// Conditional
type Conditional struct {
    Pos
    Expr Expr
    Body []WorkflowNode
}
```

### Expression Node Types

```go
// Source: derived from miniwdl/WDL/Expr.py

type Expr interface{ exprNode() }

type BoolLit  struct { Pos; Value bool }
type IntLit   struct { Pos; Value int64 }
type FloatLit struct { Pos; Value float64 }
type NullLit  struct { Pos }

// String / command template
type StringExpr struct {
    Pos
    Parts []StringPart  // alternating RawStringPart / PlaceholderPart
    Style StringStyle   // StyleSingle, StyleDouble, StyleHeredoc
}
type CommandExpr struct {
    Pos
    Parts []StringPart
    Style CommandStyle  // CommandBrace, CommandHeredoc
}

type ArrayExpr  struct { Pos; Items []Expr }
type PairExpr   struct { Pos; Left, Right Expr }
type MapExpr    struct { Pos; Items []MapItem }
type StructExpr struct { Pos; TypeName string; Members []StructMember }

type IfThenElse struct { Pos; Cond, Then, Else Expr }
type BinaryExpr struct { Pos; Op TokenType; Left, Right Expr }
type UnaryExpr  struct { Pos; Op TokenType; Operand Expr }
type IndexExpr  struct { Pos; Expr, Index Expr }
type GetExpr    struct { Pos; Expr Expr; Member string } // struct/pair member access
type ApplyExpr  struct { Pos; Name string; Args []Expr } // function call
type IdentExpr  struct { Pos; Name string }
```

### ParseError with rustc-Style Display

```go
// Source: CONTEXT.md decisions

type ParseError struct {
    File       string
    Line       int
    Col        int
    Message    string
    SourceLine string
}

func (e *ParseError) Error() string {
    // Produces:
    // error: unexpected token
    //   --> file.wdl:12:5
    //    |
    // 12 | workflow foo {
    //    |     ^
    lineStr := strconv.Itoa(e.Line)
    pad := strings.Repeat(" ", len(lineStr))
    caret := strings.Repeat(" ", e.Col-1) + "^"
    return fmt.Sprintf("error: %s\n  --> %s:%d:%d\n%s |\n%s | %s\n%s | %s",
        e.Message,
        e.File, e.Line, e.Col,
        pad,
        lineStr, e.SourceLine,
        pad, caret,
    )
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| WDL `runtime {}` section | WDL 1.2 adds `requirements {}` as canonical name; `runtime` still accepted | WDL 1.2 | Parser must accept both keywords; they map to the same field |
| No `Directory` type | WDL 1.2 adds `Directory` as first-class primitive | WDL 1.2 | `Directory` is a keyword; already in `wdl/types` from Phase 1 |
| `env` keyword on task inputs | WDL 1.2 adds `env` modifier for task input declarations | WDL 1.2 | `env` is a keyword (priority `.2` in lark grammar); optional prefix on decl |
| `hints {}` section | WDL 1.2 adds `hints` section alongside `runtime`/`requirements` | WDL 1.2 | Must be parsed; contents can be stored as `map[string]any` for Phase 5 |
| Object literals (`object {}`) | Deprecated in WDL 1.2; struct literals (`TypeName {}`) replace it | WDL 1.0→1.2 | `object` keyword is out of scope per project requirements |

**WDL 1.2 new keywords** (beyond 1.1): `Directory`, `env`, `hints`, `requirements`

**Deprecated/outdated:**
- `Object` type: out of scope per REQUIREMENTS.md out-of-scope table
- `parameter_meta` section with typed values: store as `map[string]any` (JSON-like), don't fully type-check
- Placeholder options (`sep=`, `true=`, `false=`, `default=`): still valid in WDL 1.2 inside `~{}` / `${}`

---

## Open Questions

1. **`**` operator in WDL 1.2 spec**
   - What we know: miniwdl's lark grammar omits `**`; miniwdl evaluator handles it in Python eval. REQUIREMENTS.md PARSE-06 explicitly requires `**` parsing.
   - What's unclear: Exact WDL 1.2 spec text for `**` precedence relative to unary `-`. Does `-2**3` parse as `(-2)**3` or `-(2**3)`?
   - Recommendation: Parse `**` as right-associative above `*`/`/`/`%` and below unary `!`/`-`. This matches Python semantics (`-(2**3) = -8`). Verify against WDL 1.2 spec document if available.

2. **Placeholder options in WDL 1.2**
   - What we know: Placeholder options (`sep=`, `true=`, `false=`, `default=`) are valid in both `~{}` and `${}` per lark grammar.
   - What's unclear: WDL 1.2 spec may have deprecated some placeholder options. The `placeholder_option` rule (`placeholder_name "=" placeholder_value`) is present in the 1.2 grammar.
   - Recommendation: Parse placeholder options and store them in `PlaceholderPart.Options map[string]string`. Phase 4 evaluator will enforce semantics.

3. **`hints` section structure**
   - What we know: `hints_section` allows `io_hint` values which have `input {}` / `output {}` sub-objects with nested key-value pairs.
   - What's unclear: Phase 5 actually uses hints for container resource hints. The exact AST representation needed by Phase 5 is TBD.
   - Recommendation: For Phase 2, parse `hints {}` body as `map[string]any` (similar to `meta {}`). Use a recursive `parseHintValue` function. Phase 5 can re-parse as needed.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go built-in `testing` package (go 1.24.6) |
| Config file | none — `go test ./...` discovers tests automatically |
| Quick run command | `go test ./wdl/lexer/... ./wdl/parser/... ./wdl/ast/...` |
| Full suite command | `go test ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PARSE-01 | Parse version+task+workflow+struct+import into Document | integration | `go test ./wdl/parser/... -run TestParseDocument` | ❌ Wave 0 |
| PARSE-02 | Lexer enters separate modes for `command {}` vs `command <<<>>>` | unit | `go test ./wdl/lexer/... -run TestCommandModes` | ❌ Wave 0 |
| PARSE-03 | `~{}` and `${}` recognized as placeholders in brace command | unit | `go test ./wdl/lexer/... -run TestCommandBracePlaceholders` | ❌ Wave 0 |
| PARSE-04 | `${}` is raw text in heredoc command; `~{}` is placeholder | unit | `go test ./wdl/lexer/... -run TestCommandHeredocPlaceholders` | ❌ Wave 0 |
| PARSE-05 | Nested string interpolation produces correct parts list | unit | `go test ./wdl/parser/... -run TestStringInterpolation` | ❌ Wave 0 |
| PARSE-06 | `2**3**2` parses as `2**(3**2)` (right-assoc, higher than `*`) | unit | `go test ./wdl/parser/... -run TestOperatorPrecedence` | ❌ Wave 0 |
| PARSE-07 | Import resolves relative path, merges into single Document | integration | `go test ./wdl/parser/... -run TestImportResolution` | ❌ Wave 0 |
| PARSE-08 | Syntax error returns ParseError with correct Line, Col, SourceLine | unit | `go test ./wdl/parser/... -run TestParseError` | ❌ Wave 0 |
| PARSE-09 | Struct with nested struct type reference parses correctly | unit | `go test ./wdl/parser/... -run TestStructDef` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `go test ./wdl/lexer/... ./wdl/parser/...`
- **Per wave merge:** `go test ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `wdl/lexer/lexer_test.go` — covers PARSE-02, PARSE-03, PARSE-04
- [ ] `wdl/parser/parser_test.go` — covers PARSE-01, PARSE-05, PARSE-06, PARSE-07, PARSE-08, PARSE-09
- [ ] `wdl/testdata/*.wdl` — WDL fixture files for integration tests (PARSE-01, PARSE-07)
- [ ] Package stubs: `wdl/lexer/lexer.go`, `wdl/ast/ast.go`, `wdl/ast/expr.go`, `wdl/parser/parser.go`, `wdl/parser/parse_expr.go`

---

## Sources

### Primary (HIGH confidence)

- `miniwdl/WDL/_grammar.py` — WDL 1.2 grammar (complete, authoritative Lark PEG grammar used by the reference implementation)
- `miniwdl/WDL/Expr.py` — Expression node types, `String.parts` pattern, `_dedent` algorithm, operator precedence dict
- `miniwdl/WDL/Tree.py` — Top-level AST node structure: Document, Task, Workflow, Decl, Call, Scatter, Conditional, StructTypeDef
- `flowy/wdl/types/types.go` — Phase 1 type structs, naming conventions, interface pattern
- `flowy/wdl/types/types_test.go` — Established test pattern (table-driven, `_test` package suffix)

### Secondary (MEDIUM confidence)

- Rob Pike "Lexical Scanning in Go" (2011 GopherCon talk) — state-function lexer pattern; `text/template/parse/lex.go` in Go standard library is the canonical implementation
- Pratt "Top Down Operator Precedence" (1973) — Pratt parser theory; widely implemented in Go; binding power table above derived from grammar analysis

### Tertiary (LOW confidence)

- WDL 1.2 spec text on `**` operator: not directly verified against the official spec document (OpenWDL/wdl spec repo). The requirement is confirmed via REQUIREMENTS.md PARSE-06 and CONTEXT.md, but the exact precedence relative to unary `-` is inferred from Python convention.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — Go stdlib is sufficient; Phase 1 types package confirmed in repo
- Architecture: HIGH — miniwdl source is authoritative reference; patterns are well-established
- WDL grammar: HIGH — derived directly from `miniwdl/_grammar.py` versions["1.2"]
- `**` operator details: MEDIUM — spec mandates it (via requirements), precedence inferred from Python convention
- Pitfalls: HIGH — derived from grammar analysis and miniwdl implementation study

**Research date:** 2026-03-10
**Valid until:** 2026-09-10 (WDL 1.2 spec is stable; Go stdlib is stable)
