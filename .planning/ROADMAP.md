# Roadmap: flowy

## Overview

flowy is a WDL 1.2 workflow engine in Go, porting miniwdl's Python implementation. The project follows the interpreter pipeline dependency chain: type data structures define the WDL type system, the parser produces an AST using those types, the type checker validates forward references and coercion rules, the expression evaluator executes it, task execution runs individual units via Docker, the workflow scheduler coordinates multi-task DAGs, and the CLI ties everything together. Each phase delivers one complete, verifiable capability and is a hard prerequisite for the next. After Phase 3, `flowy check` is already useful. After Phase 7, the full engine is usable end-to-end.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Type Data Structures** - Define all WDL 1.2 type and value data structures in Go; initialize the Go module (completed 2026-03-10)
- [ ] **Phase 2: Lexer, Parser, and AST** - Parse any valid WDL 1.2 file into a typed AST with line/column error reporting
- [ ] **Phase 3: Type Checker** - Validate AST types with full forward-reference resolution, coercion rules, and scatter/if promotion; `flowy check` becomes usable
- [ ] **Phase 4: Expression Evaluator and Standard Library** - Evaluate all WDL 1.2 expressions and implement all ~30 stdlib functions
- [ ] **Phase 5: Task Execution** - Run a single WDL task end-to-end in Docker with input staging and output collection
- [ ] **Phase 6: Workflow Execution (DAG Scheduler)** - Execute multi-task workflows with scatter parallelism, conditionals, and outputs.json
- [ ] **Phase 7: CLI and Integration** - Ship `flowy run` and `flowy check` as usable CLI tools with all flags and exit codes

## Phase Details

### Phase 1: Type Data Structures
**Goal**: All WDL 1.2 types and values are representable as Go structs with embedded type tags; the Go module is initialized with correct dependencies
**Depends on**: Nothing (first phase)
**Requirements**: TYPE-01, TYPE-02, TYPE-03, TYPE-04
**Success Criteria** (what must be TRUE):
  1. `go.mod` is initialized as `github.com/bioflowy/flowy` with Go 1.24
  2. All primitive types (Boolean, Int, Float, String, File, Directory) are defined as Go structs implementing a `Type` interface
  3. All compound types (Array[T], Map[K,V], Pair[L,R], Struct) are defined with type parameters and implement the `Type` interface
  4. Optional type (T?) is defined; the None literal has a representable type
  5. Every concrete `Value` struct carries an embedded WDL `Type` field so an empty `ArrayValue` can distinguish `Array[Int]` from `Array[String]`
**Plans**: 1 plan

Plans:
- [ ] 01-01-PLAN.md — Initialize go module, implement wdl/types and wdl/values packages

### Phase 2: Lexer, Parser, and AST
**Goal**: Any valid WDL 1.2 source file can be parsed into a typed, traversable AST; parse errors are reported with precise source locations
**Depends on**: Phase 1
**Requirements**: PARSE-01, PARSE-02, PARSE-03, PARSE-04, PARSE-05, PARSE-06, PARSE-07, PARSE-08, PARSE-09
**Success Criteria** (what must be TRUE):
  1. A WDL 1.2 file containing version declaration, task, workflow, struct, and import statements produces a complete `ast.Document` with no missing nodes
  2. A `command {}` block with `~{}` and `${}` placeholders and a `command <<<>>>` block with only `~{}` placeholders are each parsed correctly, with `${}` treated as a bash literal in the heredoc style
  3. All operators including `**` (exponentiation) are parsed with correct precedence by the Pratt sub-parser
  4. A WDL file that imports another local WDL file produces a merged AST spanning both files
  5. A file with a syntax error produces an error message containing the line number and column of the first bad token, with the source line displayed (rustc-style)
**Plans**: TBD

### Phase 3: Type Checker
**Goal**: Every type in a WDL 1.2 document is resolved and validated, including forward references, scatter/if promotion, and struct fields; `flowy check` reports precise type errors
**Depends on**: Phase 2
**Requirements**: TYPE-05, TYPE-06, TYPE-07, TYPE-08
**Success Criteria** (what must be TRUE):
  1. A task or workflow with a declaration that references a name declared later in the same scope is resolved without error (forward reference)
  2. A `scatter` block's output variable is typed as `Array[T]` and an `if` block's output variable is typed as `T?`, with nested combinations compounding correctly and never producing `T??`
  3. A WDL file with a type mismatch (e.g., assigning a String to an Int input) produces a type error with the source location and a description of the mismatch
  4. All 6 WDL 1.2 coercion rules (Int→Float, T→T?, String↔File, T→Array[T], etc.) are applied shallowly and never recursively into compound type parameters
  5. Struct field types are validated; struct literal and field access expressions are correctly typed
**Plans**: TBD

### Phase 4: Expression Evaluator and Standard Library
**Goal**: Any WDL 1.2 expression can be evaluated to a typed Value, and all ~30 WDL 1.2 stdlib functions return correct results without requiring a running Docker daemon
**Depends on**: Phase 3
**Requirements**: EXPR-01, EXPR-02, EXPR-03, EXPR-04, EXPR-05, EXPR-06, EXPR-07, STDLIB-01, STDLIB-02, STDLIB-03, STDLIB-04, STDLIB-05, STDLIB-06
**Success Criteria** (what must be TRUE):
  1. Arithmetic, comparison, and logical operators (including `**`) evaluate to correct typed Values; if-then-else returns the appropriate branch value
  2. Struct field access (`struct.field`), pair access (`pair.left`, `pair.right`), and array/map index access return the correct element or a runtime error on out-of-bounds
  3. A command template with `~{}` placeholders and leading whitespace is expanded by replacing placeholders with sentinel strings, computing the minimum indent, stripping it from every non-empty line, then substituting actual values (dedent-before-interpolation algorithm)
  4. All file I/O stdlib functions (`read_lines`, `write_lines`, `read_json`, etc.) read from and write to the filesystem and return correctly typed Values
  5. All array, string, numeric, and utility stdlib functions (`length`, `flatten`, `sub`, `defined`, `ceil`, `round` with half-up rounding, WDL 1.2 additions `chunk`, `find`, `matches`, `join_paths`, `values`, `contains`) return correct results on unit test inputs
**Plans**: TBD

### Phase 5: Task Execution
**Goal**: A single WDL task can be executed end-to-end: inputs bound, command expanded, Docker container run, outputs collected on the host
**Depends on**: Phase 4
**Requirements**: TASK-01, TASK-02, TASK-03, TASK-04, TASK-05, TASK-06, TASK-07, TASK-08
**Success Criteria** (what must be TRUE):
  1. A task with JSON inputs is parsed, bound to the task's input section, and a command string is produced with all placeholders replaced
  2. Input files are staged to a container-internal path and a bidirectional host↔container path map is maintained; no raw host paths appear inside the container
  3. The Docker lifecycle (image pull → container create → start → wait → stdout/stderr capture → remove) completes successfully for a simple task, and the task's output files are collected to the host execution directory
  4. A task that exits with a non-zero exit code returns an error value containing the exit code and captured stderr text
  5. The `ContainerRunner` interface can be replaced with a mock in unit tests, running task logic without a live Docker daemon
**Plans**: TBD

### Phase 6: Workflow Execution (DAG Scheduler)
**Goal**: A multi-task WDL workflow executes in correct dependency order with scatter parallelism, conditional optional wrapping, and structured JSON output
**Depends on**: Phase 5
**Requirements**: WF-01, WF-02, WF-03, WF-04, WF-05, WF-06, WF-07, WF-08
**Success Criteria** (what must be TRUE):
  1. A workflow with multiple `call` statements executes tasks in dependency order derived from data flow; tasks with no interdependency run concurrently
  2. A `scatter` block fans out tasks over its array, runs them in parallel (bounded by `errgroup` with `SetLimit`), and collects outputs into a correctly typed `Array[T]`
  3. An `if` block evaluates its condition; when true the task runs and its output is `T`; when false the output is `None` typed as `T?`
  4. An `after` keyword dependency between two calls is respected regardless of data flow order
  5. On successful completion `outputs.json` is written to the execution directory; on any task failure remaining tasks are cancelled and `error.json` is written with the failing task name and error
**Plans**: TBD

### Phase 7: CLI and Integration
**Goal**: `flowy run` and `flowy check` are usable CLI tools that accept all documented flags, produce structured output, and exit with correct codes
**Depends on**: Phase 6
**Requirements**: CLI-01, CLI-02, CLI-03, CLI-04, CLI-05, CLI-06, CLI-07
**Success Criteria** (what must be TRUE):
  1. `flowy check <file.wdl>` parses and type-checks a WDL file, prints any errors with line numbers, and exits 0 on success or non-zero on error — without executing any task
  2. `flowy run <file.wdl> --input inputs.json` executes the workflow and writes `outputs.json` to the execution directory
  3. `flowy run <file.wdl> --task <name>` runs only the named task from the WDL file, ignoring the workflow definition
  4. `--path <dir>` extends the import search path and `--dir <dir>` sets the working directory for execution; both flags are accepted by `flowy run` and `flowy check`
  5. `--verbose` causes task stdout and stderr to be printed to the terminal during execution
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5 → 6 → 7

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Type Data Structures | 1/1 | Complete   | 2026-03-10 |
| 2. Lexer, Parser, and AST | 0/TBD | Not started | - |
| 3. Type Checker | 0/TBD | Not started | - |
| 4. Expression Evaluator and Standard Library | 0/TBD | Not started | - |
| 5. Task Execution | 0/TBD | Not started | - |
| 6. Workflow Execution (DAG Scheduler) | 0/TBD | Not started | - |
| 7. CLI and Integration | 0/TBD | Not started | - |
