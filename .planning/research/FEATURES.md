# Features Research: WDL Workflow Engine (flowy)

**Domain:** WDL 1.2 workflow engine (CLI tool, Go port of miniwdl)
**Researched:** 2026-03-10
**Sources:** OpenWDL spec, docs.openwdl.org, miniwdl source (../miniwdl/WDL/), miniwdl readthedocs

---

## Table Stakes

Features without which the engine cannot execute real WDL 1.2 files. Missing any one = users cannot run their workflows.

### Type System

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Primitive types: Boolean, Int, Float, String, File | Core types used in every WDL file | Low | Required since WDL 1.0 |
| Directory type | Added in WDL 1.2; bioinformatics tools produce directories | Medium | `Directory` is first-class in 1.2 |
| Optional types (T?) | Workflow inputs are routinely optional | Low | `?` postfix quantifier |
| Non-empty arrays (Array[T]+) | Frequently used in function signatures | Low | `+` quantifier |
| Compound types: Array[T], Map[K,V], Pair[X,Y] | Essential for scatter/gather and data passing | Medium | Parametric types |
| Struct types | User-defined record types; widely used in modern WDL | Medium | Declared at document level; can have meta/parameter_meta in 1.2 |
| Object type (read-only, deprecated) | Legacy code uses it; read_object/read_objects return it | Medium | Deprecated in 1.2 but still required for spec compliance |
| Type coercion rules | Int→Float, Boolean/Int/Float/File→String, String→File/Int/Float | Medium | 6 coercion rules; check_quant flag controls optional leniency |

### Expression Evaluation

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Arithmetic operators: +, -, *, /, % | Every command template uses these | Low | `/` is integer division for Int operands |
| Exponentiation operator: ** | New in WDL 1.2 | Low | Int**Int → Int, Float**Float → Float |
| Comparison operators: ==, !=, <, <=, >, >= | Conditional expressions | Low | |
| Logical operators: &&, \|\|, ! | if/conditional blocks | Low | |
| String interpolation: ~{} and ${} | Both syntaxes appear in real WDL; ~{} preferred in 1.2 | Medium | ${}  is legacy but still valid |
| Multi-line strings | New in WDL 1.2; same whitespace rules as command block | Medium | Task command uses multi-line string under the hood |
| If-then-else expression | Inline conditional values | Low | `if cond then val else val` |
| Array/Map/Pair literal expressions | Construction of compound values inline | Medium | `[1,2,3]`, `{"k": v}`, `(a, b)` |
| Struct literal construction | Creating struct values | Medium | `MyStruct { field: value }` |
| Member access (`.`) | Struct field access, call output access | Low | `call_name.output_name` |
| Array index (`[]`) | Array and Map element access | Low | |
| Null / None handling | Optional value handling | Medium | `select_first`, `defined()` |
| Identifier resolution (scoped) | Variables from enclosing scope | Medium | Scatter variable, call outputs |

### Task Definition

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| `input {}` section | Standard task structure | Low | Declares task parameters |
| Private (post-input) declarations | Intermediate computed values | Low | Declarations outside input block |
| `command {}` section | Core of every task; executes shell script | Medium | Template expansion with ~{} |
| `output {}` section | Captures task results | Low | Expressions evaluated after command runs |
| `runtime {}` section | Specifies docker image, cpu, memory | Medium | Maps to `requirements` in WDL 1.2 |
| `requirements {}` section | WDL 1.2 replacement for runtime | Medium | Aliases to same dict as runtime |
| `hints {}` section | Optional resource hints in WDL 1.2 | Low | gpu, fpga, disks hints |
| `meta {}` section | Task-level documentation | Low | Key-value pairs, no evaluation |
| `parameter_meta {}` section | Per-parameter documentation | Low | Keys must match input/output names |
| Runtime keys: container/docker | Specifies the OCI image | Medium | Both `container` and `docker` accepted |
| Runtime keys: cpu, memory | Resource declarations | Low | Memory parsed with units (GiB, MiB, etc.) |
| Runtime keys: disks | Storage allocation | Low | |
| Runtime keys: maxRetries | Retry on failure | Medium | miniwdl implements this |
| Runtime keys: returnCodes | Treat specific exit codes as success | Medium | `returnCodes: [0, 1]` or `"*"` |
| Environment variable inputs | WDL 1.2 task inputs as env vars | Medium | New in 1.2; env var mode |
| Implicit `task` variable | Runtime introspection in WDL 1.2 | Medium | Access to name, meta at runtime |

### Workflow Definition

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| `input {}` section | Workflow parameters | Low | |
| `output {}` section | Final workflow outputs | Low | |
| Declarations (intermediate values) | Computed workflow-level values | Low | |
| `call task_name` | Invoke a task | Low | Core workflow primitive |
| `call task_name as alias` | Call aliasing for multiple invocations | Low | |
| `call` with `input:` block | Pass inputs to a task call | Low | `input:` keyword is optional in WDL 1.2 |
| `after` dependency in call | Explicit ordering without data dependency | Medium | `call B after A {}` — confirmed in miniwdl Tree.py |
| `scatter (x in array) {}` | Parallel fan-out over array | High | Core bioinformatics pattern |
| `if (condition) {}` | Conditional execution of call block | Medium | Results are Optional[T] outside the block |
| Nested scatter/if | Scatter inside scatter; if inside scatter | High | Multi-dimensional results |
| Gather semantics | Collecting scatter outputs into arrays | High | Implicit; synthesized by engine |
| Sub-workflow calls | Calling another workflow as a task | High | Imported document's workflow |
| Local imports | `import "path/to/file.wdl"` with namespacing | Medium | HTTP imports deferred to v2 |
| Struct type imports | Structs re-exported across import boundaries | Medium | |
| `meta {}` and `parameter_meta {}` | Workflow-level docs | Low | |
| `hints {}` section | Workflow-level hints in WDL 1.2 | Low | |

### Standard Library — File/IO Functions

| Function | Signature | Complexity | Notes |
|----------|-----------|------------|-------|
| `stdout()` | `() → File` | Low | Only valid in task output section |
| `stderr()` | `() → File` | Low | Only valid in task output section |
| `glob(pattern)` | `String → Array[File]` | Medium | Shell glob inside task working dir |
| `basename(path[, suffix])` | `File\|Directory[, String] → String` | Low | |
| `size(f[, unit])` | `File\|File?\|Directory\|... → Float` | Medium | WDL 1.2: accepts compound values |
| `join_paths(a, b)` | Various overloads → File | Low | New in WDL 1.2; preferred over string concat |
| `read_string(f)` | `File → String` | Low | Strips trailing newline |
| `read_int(f)` | `File → Int` | Low | |
| `read_float(f)` | `File → Float` | Low | |
| `read_boolean(f)` | `File → Boolean` | Low | Parses "true"/"false" |
| `read_lines(f)` | `File → Array[String]` | Low | One element per line |
| `write_lines(arr)` | `Array[String] → File` | Low | |
| `read_tsv(f[, header[, cols]])` | Multiple overloads | Medium | WDL 1.2 adds header + Array[Object] variant |
| `write_tsv(arr[, header])` | `Array[Array[String]]\|Array[Struct] → File` | Medium | WDL 1.2 adds header support |
| `read_map(f)` | `File → Map[String,String]` | Low | |
| `write_map(m)` | `Map[String,String] → File` | Low | |
| `read_json(f)` | `File → Union` | Medium | Returns typed WDL value |
| `write_json(v)` | `X → File` | Medium | Serializes any WDL value |
| `read_object(f)` | `File → Object` | Low | |
| `read_objects(f)` | `File → Array[Object]` | Low | |
| `write_object(v)` | `Struct\|Object → File` | Low | |
| `write_objects(arr)` | `Array[Struct\|Object] → File` | Low | |

### Standard Library — Array Functions

| Function | Signature | Complexity | Notes |
|----------|-----------|------------|-------|
| `length(v)` | `Array\|Map\|Object\|String → Int` | Low | WDL 1.2: accepts Map, Object, String |
| `range(n)` | `Int → Array[Int]` | Low | 0..n-1 |
| `transpose(arr)` | `Array[Array[X]] → Array[Array[X]]` | Low | |
| `cross(a, b)` | `Array[X], Array[Y] → Array[Pair[X,Y]]` | Low | Cartesian product |
| `zip(a, b)` | `Array[X], Array[Y] → Array[Pair[X,Y]]` | Low | |
| `unzip(arr)` | `Array[Pair[X,Y]] → Pair[Array[X],Array[Y]]` | Low | |
| `contains(arr, v)` | `Array[P], P → Boolean` | Low | New in WDL 1.2 |
| `chunk(arr, n)` | `Array[X], Int → Array[Array[X]]` | Low | New in WDL 1.2 |
| `flatten(arr)` | `Array[Array[X]] → Array[X]` | Low | |
| `select_first(arr[, default])` | `Array[X?][, X] → X` | Low | WDL 1.2 adds default parameter |
| `select_all(arr)` | `Array[X?] → Array[X]` | Low | |
| `prefix(s, arr)` | `String, Array[String] → Array[String]` | Low | |
| `suffix(s, arr)` | `String, Array[String] → Array[String]` | Low | |
| `quote(arr)` | `Array[String] → Array[String]` | Low | Wraps elements in double quotes |
| `squote(arr)` | `Array[String] → Array[String]` | Low | Wraps elements in single quotes |
| `sep(delim, arr)` | `String, Array[String] → String` | Low | |

### Standard Library — Map/Object Functions

| Function | Signature | Complexity | Notes |
|----------|-----------|------------|-------|
| `keys(m)` | `Map[P,Y]\|Struct\|Object → Array[P]\|Array[String]` | Low | WDL 1.2 adds Struct/Object variant |
| `values(m)` | `Map[P,Y] → Array[Y]` | Low | New in WDL 1.2 |
| `contains_key(m, k)` | Multiple overloads → Boolean | Low | New in WDL 1.2 |
| `as_pairs(m)` | `Map[P,Y] → Array[Pair[P,Y]]` | Low | |
| `as_map(arr)` | `Array[Pair[P,Y]] → Map[P,Y]` | Low | |
| `collect_by_key(arr)` | `Array[Pair[P,Y]] → Map[P,Array[Y]]` | Low | |

### Standard Library — String Functions

| Function | Signature | Complexity | Notes |
|----------|-----------|------------|-------|
| `sub(s, pattern, replace)` | `String, String, String → String` | Low | POSIX ERE |
| `find(s, pattern)` | `String, String → String?` | Low | New in WDL 1.2 |
| `matches(s, pattern)` | `String, String → Boolean` | Low | New in WDL 1.2 |

### Standard Library — Numeric Functions

| Function | Signature | Complexity | Notes |
|----------|-----------|------------|-------|
| `floor(f)` | `Float → Int` | Low | |
| `ceil(f)` | `Float → Int` | Low | |
| `round(f)` | `Float → Int` | Low | Rounds half-up (not banker's rounding) |
| `min(a, b)` | `Int\|Float, Int\|Float → Int\|Float` | Low | 4 overloads for Int/Float combinations |
| `max(a, b)` | `Int\|Float, Int\|Float → Int\|Float` | Low | 4 overloads |

### Other stdlib

| Function | Signature | Complexity | Notes |
|----------|-----------|------------|-------|
| `defined(v)` | `X? → Boolean` | Low | |

### Task Execution

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Docker container execution | Every real WDL task specifies a container | High | Docker SDK for Go |
| Input file staging into container | Files passed to task must be visible inside container | High | Volume mount or copy strategy |
| Output file collection | Files produced by command must be captured | High | Copy from container working dir |
| Command template expansion | ~{} and ${} substitution with proper type rendering | Medium | Int/Float/Boolean/String/File formatting rules |
| Working directory management | Per-task isolated working directory | Medium | |
| stdout/stderr capture | Available via `stdout()` / `stderr()` in output section | Medium | |
| Exit code handling | Non-zero exit = task failure (unless returnCodes) | Low | |
| Task retry on failure | `maxRetries` runtime key | Medium | |

### Workflow Execution

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Dependency graph construction | Determines execution order | High | DAG from call inputs/after dependencies |
| Parallel task execution | Scatter shards, independent calls run concurrently | High | goroutines or worker pool |
| Scatter fan-out and gather | Core bioinformatics primitive | High | Each shard is an independent task invocation |
| Conditional block execution | `if` blocks produce Optional outputs | Medium | |
| Output directory organization | Structured output with outputs.json | Medium | Per-call subdirectories |
| outputs.json generation | Standard output format expected by tooling | Low | |
| error.json / structured errors | Programmatic error inspection | Medium | |

### CLI

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| `flowy run <wdl> [inputs...]` | Primary use case: execute a workflow | Medium | Accepts key=value or -i JSON |
| `flowy check <wdl>` | Static validation (parse + typecheck) | Low | Returns exit 0/non-zero |
| Input JSON via `-i / --input` | Cromwell-style JSON input files | Low | Standard interoperability format |
| `--task <name>` flag | Run a single task instead of workflow | Low | Useful for development |
| `-d / --dir` output directory | User-specified run directory | Low | |
| `-p / --path` import search path | Find imported WDL files | Low | |
| `--verbose` / `--debug` logging | Development and troubleshooting | Low | |
| Exit code reflects success/failure | Shell scripting integration | Low | |

---

## Differentiators

Features that distinguish flowy from existing engines. Not baseline requirements, but competitive advantages worth building.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Native Go binary (no JVM, no Python) | Zero-dependency install, fast startup, low memory | Medium | Cromwell requires JVM; miniwdl requires Python |
| Single binary distribution | `brew install flowy` or download binary; no runtime deps | Low | Built-in during Go cross-compilation |
| Fast cold-start for CI/CD | Bioinformatics pipelines run in automated pipelines; startup latency matters | Low | JVM Cromwell takes 10-30s to start |
| Structured JSON output by default | Machine-readable results without extra flags | Low | outputs.json, error.json at run completion |
| Strict WDL 1.2-only focus | No legacy version baggage; cleaner parser and type system | Low | Deliberate scope reduction = less code to maintain |
| Informative static error messages | `flowy check` with precise line/column and type mismatch descriptions | Medium | Cromwell errors are notoriously opaque |
| Parallel scatter with configurable concurrency | Limit number of concurrent shards to avoid resource exhaustion | Medium | miniwdl has this; Cromwell is harder to configure locally |
| Deterministic output layout | Reproducible directory structure aids debugging | Low | |

---

## Anti-Features (Defer for v1)

Features to explicitly NOT build in v1. Either too complex, low value, or out of project scope.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| HTTP/HTTPS import resolution | Adds significant complexity (auth, caching, network errors); v1 is local tooling | Only support local file imports; error clearly if HTTP URI is used |
| Call output caching / call cache | Requires stable hashing of inputs + WDL source; first build correctness, then performance | Document as v2 feature; users can reuse output directories manually |
| WDL draft-2 / 1.0 / 1.1 support | Each version has grammar differences; maintaining multiple parsers is high cost | Parse `version` declaration first; emit clear error if not 1.2 |
| Non-Docker backends (Podman, Singularity, udocker) | miniwdl supports 5 backends; v1 should focus on Docker correctness | Single well-tested backend is better than 5 partially-working ones |
| REST API server / web UI | v2 milestone; requires auth, job persistence, multi-user concerns | v1 is a CLI tool only |
| Multi-user / project management | v2 milestone with SQLite backend | Not needed for CLI tool |
| `flowy zip` command | Useful but not critical; users can use zip manually | Defer to v1.x or v2 |
| `flowy lint` / style checks | Separate concern from execution correctness | Defer; `flowy check` covers type errors |
| Workflow resumption / partial re-run | Requires persistent state; complex to implement correctly | Users re-run from scratch in v1 |
| Cloud backend execution (AWS Batch, GCP Life Sciences) | miniwdl-aws is a separate plugin; enormous scope | Explicitly out of scope; pure local Docker execution |
| Download caching (S3/GCS URIs) | HTTP imports already deferred; URI downloads are downstream of that | Not applicable if imports are local-only |
| GPU/FPGA runtime hints enforcement | Runtime hint is optional by spec; engines may ignore hints | Emit warning if gpu/fpga hint is present; don't fail |
| `Object` type write support (write_object, write_objects) | Object is deprecated in 1.2; minimal real-world use | Implement read-only Object support for read_object/read_objects; defer write |

---

## WDL 1.2 Spec Coverage

Comprehensive checklist of language features that must be implemented for spec compliance. Ordered by logical build dependency.

### Phase 1: Parser and AST

- [ ] `version 1.2` declaration (reject other versions)
- [ ] Import statement: `import "path.wdl" [as namespace]`
- [ ] Struct type definition: `struct Name { Type field }`
- [ ] Task definition with all sections (input, private decls, command, output, runtime/requirements, hints, meta, parameter_meta)
- [ ] Workflow definition with all sections (input, output, decls, call, scatter, if, meta, parameter_meta, hints)
- [ ] Command block: `command <<<...>>>` (heredoc) and `command {...}` (curly) with placeholder expansion
- [ ] String literals: single-line, escaped characters
- [ ] Multi-line string literals (WDL 1.2)
- [ ] All primitive literal expressions: Boolean, Int (decimal/hex/octal), Float, String, None
- [ ] All compound literal expressions: Array `[a,b,c]`, Map `{k:v}`, Pair `(a,b)`, Struct `Name{field:val}`
- [ ] All binary operators: +, -, *, /, %, **, ==, !=, <, <=, >, >=, &&, ||
- [ ] Unary operators: -, !
- [ ] If-then-else expression
- [ ] Function application: `func(arg1, arg2)`
- [ ] Member access: `obj.field`
- [ ] Array index: `arr[i]`
- [ ] Call statement: `call task_name [as alias] [after dep] { input: k=v }`
- [ ] Scatter statement: `scatter (var in array) { ... }`
- [ ] Conditional statement: `if (expr) { ... }`
- [ ] Decl with optional expression: `Type name [= expr]`

### Phase 2: Type System and Type Checking

- [ ] All primitive types: Boolean, Int, Float, String, File, Directory
- [ ] Optional types: `T?`
- [ ] Non-empty array type: `Array[T]+`
- [ ] Compound types: `Array[T]`, `Map[K,V]`, `Pair[X,Y]`, `Object`, `StructInstance`
- [ ] Type coercion rules (6 rules from miniwdl Type.py)
- [ ] Quantifier check mode (check_quant flag for lenient mode)
- [ ] Struct type definition resolution and member type checking
- [ ] Struct coercion between compatible types (WDL 1.2)
- [ ] Scope resolution: workflow-level, call output namespace, scatter variable
- [ ] Type inference for all expression nodes
- [ ] Implicit optionality: inputs with defaults are Optional (WDL 1.2)

### Phase 3: Expression Evaluation

- [ ] All arithmetic operators with type-appropriate semantics (integer vs float division)
- [ ] Exponentiation (`**`) for WDL 1.2
- [ ] String interpolation expansion (both `~{expr}` and `${expr}`)
- [ ] String interpolation type rendering rules (Int no leading zeros, Float 6 decimal digits, Boolean → "true"/"false", File → string path, Directory → path without trailing slash)
- [ ] If-then-else evaluation
- [ ] Logical short-circuit evaluation for && and ||
- [ ] Member access on Struct, Object, call output namespace
- [ ] Array/Map index access
- [ ] Null propagation for Optional types
- [ ] All standard library function implementations (see table above)
- [ ] `defined()` and `select_first()` / `select_all()` for optional handling

### Phase 4: Task Execution

- [ ] Input binding: match provided inputs to declared task inputs
- [ ] Default value evaluation for missing optional inputs
- [ ] Command template expansion: substitute all ~{} and ${} placeholders
- [ ] Container image selection from `container`/`docker` runtime key
- [ ] CPU and memory resource reservation (passed to Docker)
- [ ] Docker container creation with working directory
- [ ] Input file/directory staging into container (volume mounts)
- [ ] Command execution inside container
- [ ] stdout/stderr stream capture to files
- [ ] Exit code evaluation (success vs failure)
- [ ] `returnCodes` handling (list or `"*"`)
- [ ] Output section evaluation (with access to task working dir)
- [ ] `glob()` pattern matching against task output files
- [ ] `stdout()` and `stderr()` file references in output section
- [ ] Output file collection from container to host
- [ ] `maxRetries` retry loop on task failure
- [ ] Per-task run directory creation
- [ ] Environment variable inputs (WDL 1.2)
- [ ] Implicit `task` variable for runtime introspection (WDL 1.2)

### Phase 5: Workflow Execution

- [ ] Build call dependency graph (data dependencies + `after` dependencies)
- [ ] Topological sort / async execution schedule
- [ ] Independent calls executed concurrently (goroutine-based scheduler)
- [ ] Scatter shard creation: one invocation per array element
- [ ] Scatter variable binding in each shard's scope
- [ ] Gather: collect shard outputs into `Array[T]` (or `Array[T?]` if conditional)
- [ ] `if` block: evaluate condition; produce `T?` typed outputs
- [ ] Nested scatter (scatter inside scatter → `Array[Array[T]]`)
- [ ] Scatter inside if and if inside scatter
- [ ] Sub-workflow calls (call an imported workflow)
- [ ] Workflow output collection and outputs.json generation
- [ ] Structured error reporting with call path context

### Phase 6: CLI

- [ ] `flowy run <wdl> [key=value...] [-i input.json]` — execute workflow or task
- [ ] `flowy check <wdl>` — parse and typecheck, report errors
- [ ] `--task <name>` to run a single task
- [ ] `-d / --dir` for output directory
- [ ] `-p / --path` for import search paths
- [ ] `-v / --verbose`, `--debug` logging levels
- [ ] Cromwell-style input JSON parsing
- [ ] `key=value` and `key:=value` input syntax on command line
- [ ] Colored terminal output (with `--no-color` opt-out)
- [ ] Exit code 0 on success, non-zero on failure

---

## Feature Dependencies

```
Phase 1: Parser/AST
    └→ Phase 2: Type System
         └→ Phase 3: Expression Evaluation
              ├→ Phase 4: Task Execution (needs expression eval for command, runtime, output sections)
              └→ Phase 5: Workflow Execution (needs expression eval + task execution)
                   └→ Phase 6: CLI (wraps workflow execution)

Sub-dependencies:
  scatter execution → gather semantics (array wrapping of outputs)
  if execution → optional type wrapping of outputs
  nested scatter → multi-dimensional Array types
  call output access → scatter result collection (Gather nodes)
  sub-workflow calls → local import resolution
  glob() → Docker output file staging
  stdout()/stderr() → per-task run directory + stream capture
```

---

## Sources

- [WDL 1.2 SPEC.md on GitHub](https://github.com/openwdl/wdl/blob/wdl-1.2/SPEC.md) — authoritative spec (HIGH confidence)
- [Announcing WDL 1.2.0 — OpenWDL](https://openwdl.org/wdl/bioinformatics/workflows/announcing-wdl-1-2-0/) — official feature announcement (HIGH confidence)
- [docs.openwdl.org — File Functions](https://docs.openwdl.org/reference/stdlib/file.html) — complete stdlib signatures (HIGH confidence)
- [docs.openwdl.org — Array Functions](https://docs.openwdl.org/reference/stdlib/array.html) — complete stdlib signatures (HIGH confidence)
- [docs.openwdl.org — String Functions](https://docs.openwdl.org/reference/stdlib/string.html) — (HIGH confidence)
- [docs.openwdl.org — Map Functions](https://docs.openwdl.org/reference/stdlib/map) — (HIGH confidence)
- [docs.openwdl.org — Numeric Functions](https://docs.openwdl.org/reference/stdlib/numeric.html) — (HIGH confidence)
- [docs.openwdl.org — Tasks](https://docs.openwdl.org/language-guide/tasks.html) — task section reference (HIGH confidence)
- [miniwdl runner CLI reference](https://miniwdl.readthedocs.io/en/latest/runner_cli.html) — CLI flags (HIGH confidence)
- [miniwdl runner reference](https://miniwdl.readthedocs.io/en/latest/runner_reference.html) — caching, retry, execution model (HIGH confidence)
- miniwdl source `../miniwdl/WDL/StdLib.py` — stdlib function implementations and signatures (HIGH confidence, ground truth)
- miniwdl source `../miniwdl/WDL/Expr.py` — expression AST node classes (HIGH confidence, ground truth)
- miniwdl source `../miniwdl/WDL/Type.py` — type classes and coercion rules (HIGH confidence, ground truth)
- miniwdl source `../miniwdl/WDL/Tree.py` — Task, Workflow, Call (with `after`), Scatter, Conditional AST classes (HIGH confidence, ground truth)
- [InfoQ: WDL 1.2.0 Enhancing Workflow Description Language](https://www.infoq.com/news/2024/06/workflow-description-language/) — feature summary (MEDIUM confidence)
