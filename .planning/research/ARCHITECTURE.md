# Architecture Patterns

**Domain:** WDL 1.2 Workflow Engine (Go port of miniwdl)
**Researched:** 2026-03-10
**Confidence:** HIGH (grounded in miniwdl source + Go ecosystem docs)

---

## Recommended Architecture

The engine is a classic interpreter pipeline — source text flows through successive transformations until it produces container executions. Each stage has a clean interface; stages communicate only through well-typed structs.

```
WDL source file(s)
       |
  [1. Lexer]          raw bytes → token stream
       |
  [2. Parser]         tokens → concrete syntax tree
       |
  [3. AST builder]    CST → typed AST nodes
       |
  [4. Type checker]   AST + env → validated AST (errors surfaced here)
       |
  [5. Evaluator]      expressions → WDL values (runtime)
       |
  [6. Workflow runner] DAG traversal → task dispatches
       |
  [7. Task runner]    inputs → Docker container → outputs
       |
  [8. Output collector] container results → WDL env bindings
```

---

## Component Boundaries

### Package Organization

```
flowy/
  cmd/
    flowy/
      main.go           -- thin entry point, wires cobra + engine
  internal/
    parser/
      lexer.go          -- stateful lexer, produces token stream
      parser.go         -- hand-written recursive descent parser
      ast/
        nodes.go        -- all AST node types
        pos.go          -- SourcePosition struct
        walk.go         -- depth-first Walker (Visitor pattern)
    typesystem/
      types.go          -- WDL type hierarchy (Type interface + concrete types)
      values.go         -- WDL value hierarchy (Value interface + concrete values)
      checker.go        -- type-checking pass over validated AST
    expr/
      eval.go           -- expression evaluator (dispatches on node kind)
      stdlib.go         -- WDL standard library functions
    runtime/
      workflow.go       -- workflow state machine (DAG scheduler)
      task.go           -- task prep: input binding, command expansion
      runner.go         -- Docker container lifecycle
      env.go            -- Env.Bindings[Value] equivalent
    cli/
      run.go            -- "flowy run" command handler
      check.go          -- "flowy check" command handler
```

**Rule:** nothing in `internal/parser` imports `internal/runtime`. The parser is pure syntax; it knows nothing about execution. The type checker knows about types and AST but not runtime values.

---

## Component Details

### Component 1: Lexer (`internal/parser/lexer.go`)

**Responsibility:** Convert raw WDL source bytes into a flat token stream. Track line/column positions. Handle WDL's two string interpolation styles (`~{...}` and `${...}`) using a stateful approach (push/pop interpolation context to avoid treating `}` as a string character when inside an expression).

**Communicates with:** Parser (downstream only)

**Key design:** Handwritten stateful lexer, not a generated one. WDL string interpolation requires tracking nesting depth — participle's generated lexer cannot express this (confirmed limitation). The stateful lexer pattern from Rob Pike's talk ("Lexical Scanning in Go") is the correct approach: a `lexer` struct with a `state func(*lexer) state` field.

```go
type TokenKind int

const (
    TokIdent TokenKind = iota
    TokInt
    TokFloat
    TokString
    TokStringInterp  // opens ~{ or ${
    TokLBrace
    TokRBrace
    // ...
)

type Token struct {
    Kind TokenKind
    Text string
    Pos  SourcePos
}
```

**Build order dependency:** None (leaf component). Build first.

---

### Component 2: AST Nodes (`internal/parser/ast/`)

**Responsibility:** Define all node types representing WDL constructs. Every node embeds `SourcePos` for error reporting. Nodes are structs; `Node` is a minimal interface used only for heterogeneous collections and the Walker.

**Communicates with:** Parser (produces nodes), Type checker (reads nodes), Evaluator (reads nodes), Workflow runner (reads nodes)

**Key design decision — interface-based with struct embedding:**

Use a `Node` interface with a single `nodePos() SourcePos` method. All node structs implement it by embedding `BaseNode`. Concrete types are used directly everywhere possible — type switches, not interface dispatch, drive the evaluator. This mirrors Go's own `go/ast` design.

```go
// internal/parser/ast/nodes.go

type SourcePos struct {
    File      string
    Line      int
    Col       int
    EndLine   int
    EndCol    int
}

type BaseNode struct {
    Pos SourcePos
}

func (b BaseNode) nodePos() SourcePos { return b.Pos }

type Node interface {
    nodePos() SourcePos
}

// --- AST node hierarchy ---

type Document struct {
    BaseNode
    Version  string
    Imports  []Import
    Structs  []StructDef
    Tasks    []Task
    Workflow *Workflow  // nil if document has no workflow
}

type Task struct {
    BaseNode
    Name        string
    Inputs      []Decl
    Outputs     []Decl
    Command     CommandTemplate
    Runtime     map[string]Expr
    Meta        map[string]any
    ParameterMeta map[string]any
}

type Workflow struct {
    BaseNode
    Name    string
    Inputs  []Decl
    Body    []WorkflowNode  // Decl | Call | Scatter | Conditional
    Outputs []Decl
}

// WorkflowNode is the discriminated union type for workflow body elements.
type WorkflowNode interface {
    Node
    workflowNodeID() string
    workflowNodeDeps() []string
}
```

**Build order dependency:** Only `SourcePos`. Build second.

---

### Component 3: Parser (`internal/parser/parser.go`)

**Responsibility:** Consume the token stream from the Lexer and produce an `ast.Document`. Report `SyntaxError` with source position on failure.

**Communicates with:** Lexer (upstream), AST nodes (output)

**Key design:** Handwritten recursive descent. goyacc is not recommended for WDL because:
1. WDL has string interpolation requiring lexer state — conflicts with YACC's fixed token interface.
2. Error messages from generated parsers are notoriously poor.
3. Production language parsers (Go compiler, TypeScript, Rust) are universally handwritten for these reasons.

participle v2 is usable for simpler cases but its stateful lexer cannot handle arbitrarily nested expressions inside interpolation. Handwritten is safer for WDL 1.2 completeness.

**Build order dependency:** Lexer, AST nodes.

---

### Component 4: Type System (`internal/typesystem/`)

**Responsibility:** Define the WDL type hierarchy (`Type` interface), value hierarchy (`Value` interface), and perform the type-checking pass over the AST. The type checker walks the AST and annotates each expression node with its resolved type; it returns validation errors with source positions.

**Communicates with:** AST nodes (reads), Evaluator (provides type metadata), CLI (surfaces errors)

**Key design:**

```go
// internal/typesystem/types.go

type Type interface {
    wdlType()         // marker
    String() string
    Optional() bool
    Coerces(to Type) bool
}

// Concrete types
type TBoolean struct{ optional bool }
type TInt     struct{ optional bool }
type TFloat   struct{ optional bool }
type TString  struct{ optional bool }
type TFile    struct{ optional bool }
type TDirectory struct{ optional bool }
type TArray   struct{ ItemType Type; NonEmpty bool; optional bool }
type TMap     struct{ KeyType, ValueType Type; optional bool }
type TPair    struct{ Left, Right Type; optional bool }
type TObject  struct{ optional bool }
type TStruct  struct{ Name string; Members map[string]Type; optional bool }
type TNull    struct{}
```

The type checker is a second pass over the AST (after parsing), not interleaved. It returns a `[]ValidationError` slice. Empty slice = success.

**Build order dependency:** AST nodes. Build after parser is stable.

---

### Component 5: Expression Evaluator (`internal/expr/`)

**Responsibility:** Evaluate WDL expressions at runtime given an environment (variable bindings → values). Implements all WDL operators and dispatches to stdlib functions.

**Communicates with:** Type system (uses types/values), Runtime env (reads bindings), Stdlib (delegates function calls)

**Key design:** A single `Eval(expr ast.Expr, env *runtime.Env) (typesystem.Value, error)` function with a type switch over all expression node kinds. No visitor overhead — direct dispatch.

```go
func Eval(node ast.Expr, env *Env) (Value, error) {
    switch n := node.(type) {
    case *ast.IntLit:
        return VInt{V: n.Value}, nil
    case *ast.StringLit:
        return evalString(n, env)
    case *ast.Apply:
        return evalFuncCall(n, env)
    case *ast.BinaryOp:
        return evalBinary(n, env)
    // ...
    }
}
```

**Stdlib (`internal/expr/stdlib.go`):** WDL 1.2 standard library functions (`defined`, `length`, `flatten`, `select_first`, `zip`, `read_lines`, `write_lines`, etc.) are registered in a map keyed by function name. Each entry is a `func(args []Value) (Value, error)`. File I/O stdlib functions (`read_*`, `write_*`) operate on real filesystem paths.

**Build order dependency:** Type system, AST nodes.

---

### Component 6: Runtime Environment (`internal/runtime/env.go`)

**Responsibility:** An immutable linked-list bindings map (mirrors miniwdl's `Env.Bindings`). Maps WDL identifier names to `Value` instances. Supports lexical scoping via a parent pointer — scatter sub-environments inherit from their parent.

**Communicates with:** Evaluator, Workflow runner, Task runner

**Key design:**

```go
type Env struct {
    bindings map[string]Value
    parent   *Env
}

func (e *Env) Lookup(name string) (Value, bool)
func (e *Env) Bind(name string, val Value) *Env  // returns new child Env
func (e *Env) Child() *Env
```

**Build order dependency:** Type system (Value types). Build alongside evaluator.

---

### Component 7: Workflow Runner (`internal/runtime/workflow.go`)

**Responsibility:** Traverse the workflow DAG. Maintain a state machine tracking which nodes are `waiting`, `ready`, `running`, and `finished`. Dispatch calls as goroutines. Collect outputs and propagate to dependent nodes.

**Communicates with:** AST (reads workflow body), Evaluator (evaluates Decl expressions), Task runner (dispatches calls), Env (transmits values)

**Key design — goroutine-per-ready-node with errgroup:**

This is the most architecturally significant component. The pattern directly mirrors miniwdl's `StateMachine` but uses Go idioms instead of Python's `concurrent.futures`.

```
StateMachine state:
  waiting  set[jobID]   -- dependencies not yet met
  running  set[jobID]   -- goroutine launched
  finished set[jobID]   -- outputs available
  outputs  map[jobID]Env

Main loop (runs in a single goroutine):
  for len(finished) < len(jobs):
      for each job in waiting where all deps in finished:
          move job → running
          go func(job):
              result = dispatchJob(job)
              resultCh <- result
      select result from resultCh:
          move job → finished
          store outputs[job.id] = result.env
```

**Scatter handling:** When a `Scatter` node becomes ready, the runner evaluates the scatter array expression and spawns one sub-job per element. Each sub-job gets a child `Env` with the scatter variable bound. After all sub-jobs finish, a synthetic `Gather` job collects results into `Array[T]`. This matches miniwdl's Gather node concept.

**Concurrency primitive:** `golang.org/x/sync/errgroup` with `SetLimit(N)` for bounded parallelism. Context cancellation propagates through all goroutines on first error. `golang.org/x/sync/semaphore` for optional CPU/memory limits.

```go
g, ctx := errgroup.WithContext(ctx)
g.SetLimit(maxParallelTasks)

for ready node in iteration:
    node := node  // capture
    g.Go(func() error {
        return dispatchJob(ctx, node, inputEnv)
    })

err := g.Wait()
```

**Build order dependency:** Evaluator, Task runner, Env. This is the last internal component built.

---

### Component 8: Task Runner (`internal/runtime/task.go` + `runner.go`)

**Responsibility:** Given a `Task` AST node and bound input `Env`:
1. Evaluate the `runtime{}` section (docker image, cpu, memory).
2. Expand the command template (substitute `~{expr}` and `${expr}` placeholders).
3. Stage input files into a temporary working directory.
4. Call the Docker runner.
5. Parse outputs by evaluating output declarations against the run directory.

`task.go` — pure WDL logic (command expansion, input/output binding). No Docker imports.
`runner.go` — Docker SDK interaction only.

**Key design — file staging:**

Input files are bind-mounted read-only into the container. The run directory (a temp dir per task invocation) is mounted at a fixed container path (`/cromwell_root` or similar). Output files must be within this directory.

```go
// Lifecycle of a single task run
func RunTask(ctx context.Context, task *ast.Task, env *Env, cfg Config) (*Env, error) {
    cmd, err := expandCommand(task.Command, env)
    image, err := evalRuntimeAttr(task.Runtime["docker"], env)
    runDir, err := os.MkdirTemp("", "flowy-task-*")

    containerID, err := dockerCreate(ctx, image, cmd, runDir, inputMounts)
    defer dockerRemove(ctx, containerID)

    exitCode, err := dockerRun(ctx, containerID)
    if exitCode != 0 {
        return nil, &TaskFailedError{ExitCode: exitCode, ...}
    }

    return evalOutputs(task.Outputs, env, runDir)
}
```

**Docker SDK usage (MEDIUM confidence — verified against pkg.go.dev v28):**
- `client.ContainerCreate` — create container with mounts and env vars
- `client.ContainerStart` — start
- `client.ContainerWait` — block until exit, returns `<-chan WaitResponse`
- `client.ContainerLogs` — stream stdout/stderr to log file
- `client.ContainerRemove` — cleanup (defer pattern with `force: true`)

**Build order dependency:** Type system, Evaluator, Env. Can be built in parallel with Workflow runner.

---

### Component 9: CLI (`internal/cli/` + `cmd/flowy/`)

**Responsibility:** Wire cobra commands to engine. Parse JSON/YAML input files, call the engine, print outputs.

**Communicates with:** All of runtime (entry point)

**Key design:** `cmd/flowy/main.go` contains only `main()` and a cobra root command. Actual command logic lives in `internal/cli/`. This follows the standard Go project layout where `cmd/` is thin.

Commands:
- `flowy run <wdl-file> [--input inputs.json]` — parse, typecheck, execute
- `flowy check <wdl-file>` — parse + typecheck only (no execution)

**Build order dependency:** All other components.

---

## Data Flow

### Compile-time path (`flowy check`)

```
file path
  → os.ReadFile
  → lexer.Lex(source)          → []Token
  → parser.Parse(tokens)        → *ast.Document
  → typesystem.Check(doc)       → []ValidationError
  → CLI prints errors or "OK"
```

### Execution path (`flowy run`)

```
file path + inputs.json
  → [compile-time path] → *ast.Document (validated)
  → runtime.BindInputs(doc.Workflow.Inputs, jsonInputs) → *Env
  → runtime.RunWorkflow(ctx, workflow, env)
      for each ready WorkflowNode:
        Decl  → expr.Eval(decl.Expr, env) → Value → extend Env
        Call  → [task execution path] → output Env
        Scatter → [spawn N sub-runs] → [Gather] → Array[T] Env
        Conditional → [eval condition] → [0 or 1 sub-runs] → Optional[T] Env
  → collect output Env
  → serialize outputs to JSON → stdout
```

### Task execution path

```
*ast.Task + input Env
  → expandCommand(task.Command, env)  → string
  → evalRuntimeAttrs(task.Runtime, env) → {image, cpu, mem}
  → stageInputFiles(env, runDir)
  → docker.ContainerCreate(image, cmd, mounts)
  → docker.ContainerStart
  → docker.ContainerWait → exitCode
  → docker.ContainerLogs → write to runDir/stderr, runDir/stdout
  → evalOutputDecls(task.Outputs, env, runDir) → output Env
  → docker.ContainerRemove
```

---

## Build Order (Dependency Sequence)

This order respects inter-package import constraints (no circular imports):

```
1. internal/parser/ast/      (leaf — no internal imports)
2. internal/parser/          (imports ast only)
3. internal/typesystem/      (imports ast)
4. internal/runtime/env.go   (imports typesystem)
5. internal/expr/            (imports ast, typesystem, runtime/env)
6. internal/runtime/task.go  (imports ast, typesystem, expr, env)
7. internal/runtime/runner.go (imports task — Docker SDK isolated here)
8. internal/runtime/workflow.go (imports all above)
9. internal/cli/             (imports runtime/workflow, runtime/runner)
10. cmd/flowy/               (imports cli)
```

Phases 4, 5, and 6 can be developed in parallel since they only depend on layers 1-3.

---

## Patterns to Follow

### Pattern 1: Source Position on Every Error

Every error type carries a `SourcePos`. Errors are never bare `fmt.Errorf` strings — they are typed structs implementing `error`. This allows the CLI to print `file.wdl:12:5: error message` uniformly.

```go
// internal/parser/ast/pos.go
type SourcePos struct {
    File    string
    Line    int
    Col     int
    EndLine int
    EndCol  int
}

// Example error type
type SyntaxError struct {
    Pos SourcePos
    Msg string
}
func (e *SyntaxError) Error() string {
    return fmt.Sprintf("%s:%d:%d: syntax error: %s", e.Pos.File, e.Pos.Line, e.Pos.Col, e.Msg)
}
```

### Pattern 2: Typed Error Hierarchy

Following miniwdl's pattern (SyntaxError, ValidationError subclasses, RuntimeError), use a custom error hierarchy:

```
error (Go built-in)
  WDLError (interface, adds Pos() SourcePos)
    SyntaxError        -- lexer/parser failures
    ValidationError    -- type check failures
      InvalidType
      NoSuchTask
      UndefinedVariable
    RuntimeError       -- execution failures
      TaskFailed       -- non-zero container exit
      EvalError        -- expression evaluation failure
      InputError       -- missing/wrong type input
```

Use `errors.Is` / `errors.As` for error inspection in tests and the CLI layer.

### Pattern 3: Immutable Env (Linked Bindings)

Environments are immutable linked lists, not maps that get mutated. This eliminates data races in scatter goroutines — each scatter element gets its own `Env` child without copying the whole map.

```go
func (e *Env) With(name string, v Value) *Env {
    child := &Env{parent: e, bindings: make(map[string]Value, 1)}
    child.bindings[name] = v
    return child
}
```

### Pattern 4: Workflow State Machine with Channel-Based Completion

The workflow runner is a single-goroutine state machine that dispatches goroutines and receives results. Do not share the state machine itself across goroutines.

```go
type completionMsg struct {
    jobID  string
    env    *Env
    err    error
}

resultCh := make(chan completionMsg, len(jobs))
```

### Pattern 5: Docker Runner Behind an Interface

Define a `ContainerRunner` interface even though only Docker is implemented in v1. This makes task runner tests possible without Docker.

```go
type ContainerRunner interface {
    Run(ctx context.Context, spec ContainerSpec) (ContainerResult, error)
}

type ContainerSpec struct {
    Image   string
    Command []string
    Mounts  []Mount
    Env     map[string]string
}
```

---

## Anti-Patterns to Avoid

### Anti-Pattern 1: Global Mutable State in the Evaluator

**What goes wrong:** Storing evaluator state (variable bindings, current file path) in package-level variables.
**Why bad:** Scatter runs multiple tasks concurrently. Any global state causes data races.
**Instead:** Pass `*Env` and `context.Context` explicitly through every function that needs them.

### Anti-Pattern 2: Interleaving Parsing and Type Checking

**What goes wrong:** Performing type resolution during parsing (as some parser generators encourage).
**Why bad:** The type checker needs the full AST to resolve forward references (WDL allows calling a task defined later in the file). Interleaving forces two-pass logic inside one messy phase.
**Instead:** Two clean sequential phases: Parse → full AST, then TypeCheck → validated AST.

### Anti-Pattern 3: Mixing Docker SDK calls into Task Logic

**What goes wrong:** Calling `client.ContainerCreate` directly inside command-expansion or output-evaluation code.
**Why bad:** Unit tests for command expansion cannot run without a Docker daemon. It couples unrelated concerns.
**Instead:** `task.go` builds `ContainerSpec` from pure WDL logic. `runner.go` (or a `ContainerRunner` implementation) owns all Docker SDK calls.

### Anti-Pattern 4: Goroutine per Scatter Element with No Bound

**What goes wrong:** Launching `go func()` for every element of a large scatter array (e.g., 10,000-element array).
**Why bad:** Goroutines are cheap but not free. More importantly, this saturates the Docker daemon and the local disk with concurrent container activity.
**Instead:** Use `errgroup.SetLimit(N)` where N is a configurable max parallel task count (default: `runtime.NumCPU()`).

### Anti-Pattern 5: Panic-Based Error Propagation

**What goes wrong:** Using `panic` to propagate errors through the evaluator or parser (sometimes done to avoid `if err != nil` boilerplate).
**Why bad:** Panics escape goroutine boundaries poorly (must `recover` in every goroutine). Kills the process on any WDL file with a bad expression.
**Instead:** Return `(Value, error)` consistently. Accept the verbosity — it is idiomatic Go.

---

## Scalability Considerations

| Concern | v1 (local, ~10 tasks) | Future (100+ parallel tasks) |
|---------|----------------------|------------------------------|
| Task parallelism | errgroup.SetLimit(N) default=NumCPU | Configurable via --max-concurrent flag |
| File staging | Copy to tempdir | Hardlink when on same filesystem (os.Link) |
| Docker socket | Single local daemon | Could swap ContainerRunner for remote daemon |
| Memory (large arrays) | Values held in-memory | Stream-evaluate scatter; not needed for v1 |
| Output JSON | Marshal full env | Size limit warnings; not needed for v1 |

---

## Key Architectural Decisions

| Decision | Choice | Rationale | Confidence |
|----------|--------|-----------|------------|
| Parser strategy | Handwritten recursive descent | String interpolation requires stateful lexer; better error messages; production standard | HIGH |
| AST node design | Interface `Node` + concrete structs, type switch in evaluator | Mirrors Go's `go/ast`; avoids reflection; enables exhaustive type switches | HIGH |
| Concurrency primitive | `golang.org/x/sync/errgroup` + `SetLimit` | Propagates first error + context cancel; bounded parallelism; idiomatic | HIGH |
| Docker abstraction | `ContainerRunner` interface | Enables testing without daemon; isolates SDK dependency | HIGH |
| Type/parse separation | Two sequential passes | Forward references require full AST before type checking | HIGH |
| Error design | Typed struct hierarchy + SourcePos | Source-location errors are essential for a language tool | HIGH |
| Env mutability | Immutable linked-list `*Env` | No copying needed; scatter goroutines share safely | MEDIUM |

---

## Sources

- miniwdl source: `../miniwdl/WDL/runtime/workflow.py` (StateMachine design, Gather node pattern)
- miniwdl source: `../miniwdl/WDL/Error.py` (error hierarchy with SourcePosition)
- miniwdl source: `../miniwdl/WDL/Tree.py` (AST node design, WorkflowNode interface)
- [golang-standards/project-layout](https://github.com/golang-standards/project-layout) — cmd/, internal/, pkg/ conventions
- [go/ast package](https://pkg.go.dev/go/ast) — Node interface + Walk visitor pattern
- [alecthomas/participle v2](https://pkg.go.dev/github.com/alecthomas/participle/v2) — stateful lexer limitation confirmed
- [golang.org/x/sync/errgroup](https://pkg.go.dev/golang.org/x/sync/errgroup) — errgroup.SetLimit, context cancellation
- [golang.org/x/sync/semaphore](https://pkg.go.dev/golang.org/x/sync/semaphore) — weighted semaphore for resource limits
- [docker/docker/client](https://pkg.go.dev/github.com/docker/docker/client) — ContainerCreate/Start/Wait/Logs/Remove API (v28)
- [Handwritten parsers survey](https://lobste.rs/s/10pkib/parser_generators_vs_handwritten) — industry practice
- [miniwdl Medium article](https://medium.com/czi-technology/miniwdl-17ecdaf40944) — architecture overview
- [goyacc docs](https://pkg.go.dev/golang.org/x/tools/cmd/goyacc) — YACC-style parser for Go reference
