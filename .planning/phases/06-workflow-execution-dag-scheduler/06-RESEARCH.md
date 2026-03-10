# Phase 6: Workflow Execution (DAG Scheduler) - Research

**Researched:** 2026-03-10
**Domain:** Go DAG scheduler for WDL workflow execution — topological sort, errgroup concurrency, scatter/if type promotion
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Scatter concurrency:**
- Fixed concurrency limit of 8 via `errgroup.SetLimit(8)`
- Global shared limit across all scatter blocks, including nested scatter-in-scatter — prevents resource explosion
- No CLI override in v1; concurrency is hardcoded (configurable override deferred to v2)

**Task failure behavior:**
- Fail-fast: on first task failure, cancel all running tasks immediately via `context.Cancel()`
- All downstream and unrelated tasks are stopped — no partial execution continues
- Only `error.json` is written on failure; `outputs.json` is NOT written
- error.json contains minimal info: `{"error": "message", "task": "task_name", "exit_code": N}` — stderr available in the task's `stderr.txt` file

**Nested scatter/if output types:**
- WDL spec compliant: `scatter(if(call))` → `Array[T?]`, `if(scatter(call))` → `Array[T]?`
- scatter-in-scatter supported in v1 — produces `Array[Array[T]]`
- `if(false) { scatter(...) { call foo } }` → `None` (the if block is not entered, scatter never executes)
- Phase 3 type checker already implements these promotion rules; runtime must match

**outputs.json format:**
- miniwdl-compatible key format: `{"workflow_name.output_name": value, ...}` — flat key-value structure
- File/Directory values written as relative paths from the execution directory (not absolute host paths)
- Written only on successful completion of all tasks

**error.json format:**
- Minimal structure: `{"error": "human-readable message", "task": "failing_task_name", "exit_code": N}`
- Written when any task fails; stderr content is in the task's execution directory (`stderr.txt`), not duplicated in error.json

### Claude's Discretion
- DAG construction algorithm (topological sort approach, adjacency list representation)
- How to resolve `call` callee names to `*ast.Task` nodes (lookup strategy)
- Internal scheduler state machine design
- How to pass workflow-level input bindings through to individual call input expressions
- Error wrapping and propagation strategy within the scheduler

### Deferred Ideas (OUT OF SCOPE)
- Configurable concurrency limit via CLI flag (`--concurrency`) — v2
- Resource-aware scheduling (CPU/memory-based concurrency) — v2
- Partial execution output on failure (error.json + partial outputs.json) — v2
- Sub-workflow calling (workflow calling another workflow) — out of scope per REQUIREMENTS.md
- Task output caching/memoization — v2 (EXT-02)
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| WF-01 | Build a DAG from the workflow call graph and determine execution order | Topological sort via Kahn's algorithm over adjacency list; data-flow edges + `after` edges |
| WF-02 | Execute `call` blocks and bind outputs into the workflow environment | `task.RunTask()` is the existing entry point; call inputs evaluated via `eval.Eval` against current env |
| WF-03 | Expand `scatter` blocks in parallel (errgroup + SetLimit), collect into `Array[T]` | `errgroup.WithContext` + `SetLimit(8)`; results collected into pre-allocated slice by scatter index |
| WF-04 | Evaluate `if` blocks; false condition yields `None` (T?) | Evaluate condition with `eval.Eval`; wrap outputs in `values.NullValue{}` when false |
| WF-05 | Respect `after` keyword explicit ordering dependencies | Add `after` names as DAG edges in addition to data-flow edges |
| WF-06 | Use `errgroup.WithContext` + `SetLimit` to prevent goroutine leaks | Single global errgroup with `SetLimit(8)` shared across all scatter blocks |
| WF-07 | Write `outputs.json` on successful completion | `encoding/json` marshal of workflow output env; `workflow_name.output_name` key format; relative file paths |
| WF-08 | Cancel running tasks on failure; write `error.json` | Context cancellation propagates through all `RunTask` calls; `error.json` written by scheduler before returning |
</phase_requirements>

---

## Summary

Phase 6 builds the `wdl/workflow` package — a DAG scheduler that orchestrates multi-task WDL workflow execution. The scheduler is the capstone of the runtime: it consumes the existing `task.RunTask`, `eval.Eval`, and `eval.Env` infrastructure from Phases 4 and 5, adding the dependency resolution, parallel dispatch, and structured output layers on top.

The core pattern is a single-goroutine scheduler loop that maintains three sets — `pending` (dependencies unresolved), `running` (goroutine dispatched), `done` (outputs collected). Each iteration dispatches all ready nodes via a single shared `errgroup`, blocks on a result channel until one node completes, then re-evaluates readiness. This channel-based single-writer design eliminates shared-state data races (Pitfall 14 from prior research) without needing any mutexes.

Scatter handling requires evaluating the scatter array expression, pre-allocating a `[]values.Value` result slice, and dispatching one goroutine per element through the shared errgroup. The errgroup's `SetLimit(8)` naturally rate-limits all scatter goroutines globally. Conditional handling is synchronous: evaluate the boolean condition, execute the body when true and bind outputs, or bind `values.NullValue{}` for all body outputs when false. Both patterns are straightforward in the single-goroutine scheduler model.

**Primary recommendation:** Implement the `wdl/workflow` package as a single `RunWorkflow` function backed by a private `scheduler` struct with a channel-based state machine. Use one global errgroup with `SetLimit(8)`. Resolve callee names via a pre-built `map[string]*ast.Task` from the document.

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `golang.org/x/sync/errgroup` | v0.20.0 | Bounded parallel goroutines with context cancellation | Locked decision; `SetLimit` + `WithContext` is the idiomatic Go pattern for scatter |
| `encoding/json` | stdlib | Marshal outputs.json and error.json | No external dependency needed; workflow outputs are already `values.Value` structs |
| `context` | stdlib | Cancellation propagation through RunTask calls | Already threaded through all task.RunTask calls |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `sync` | stdlib | `sync.Mutex` is NOT needed (single-writer pattern eliminates it) | If single-writer is abandoned, protect shared output map |
| `path/filepath` | stdlib | Compute relative file paths for outputs.json | File/Directory values need to be made relative to execRoot |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Channel-based single-writer scheduler | sync.Mutex on output map | Mutex approach is simpler to write but fails under `go test -race` for large scatter widths |
| Pre-allocated slice for scatter results | Result channel per scatter | Channel approach risks "send on closed channel" panics on cancellation |

**Installation:**
```bash
go get golang.org/x/sync@v0.20.0
```

---

## Architecture Patterns

### Recommended Project Structure
```
wdl/
├── workflow/
│   ├── workflow.go        # RunWorkflow entry point; public API
│   ├── scheduler.go       # scheduler struct; state machine; DAG traversal loop
│   ├── dag.go             # buildDAG(); topological sort (Kahn's algorithm)
│   ├── scatter.go         # runScatter(); errgroup fan-out + gather
│   ├── conditional.go     # runConditional(); condition eval + optional wrapping
│   ├── output.go          # writeOutputsJSON(); writeErrorJSON(); value serialization
│   └── workflow_test.go   # Tests using ShellRunner (no Docker daemon)
```

### Pattern 1: DAG Construction (Kahn's Algorithm)

**What:** Build an adjacency list from data-flow dependencies and `after` edges, then topologically sort with Kahn's algorithm to produce an execution order. Each `call`, `scatter`, and `conditional` is a node.

**When to use:** At workflow entry, before any tasks execute. The sorted order tells the scheduler which nodes can run immediately (no predecessors) vs. which must wait.

**How to identify data-flow edges:**
Every `ast.Call.Inputs` expression may reference outputs of other calls via identifiers like `call_name.output_name`. Walk each input expression's `ast.IdentExpr` (and `ast.GetExpr.Object`) to find which call names they reference. A call `foo` depends on call `bar` if any of `foo`'s inputs reference `bar.something`.

**How to identify `after` edges:**
`ast.Call.After []string` lists explicit names. Add a directed edge `after_name → call_name` for each.

**Node naming convention:**
Use `ast.Call.Alias` if non-empty, otherwise the last element of `ast.Call.Callee`.

```go
// Source: established in-project pattern; see wdl/ast/ast.go
type dagNode struct {
    id       string   // call alias or last callee name
    node     ast.WorkflowNode
    deps     []string // names this node depends on
    indegree int
}

// Kahn's algorithm: O(V+E)
func buildDAG(body []ast.WorkflowNode) ([]*dagNode, error) {
    // 1. Build nodes map
    // 2. Walk each call's input expressions for IdentExpr/GetExpr references
    // 3. Add after edges from ast.Call.After
    // 4. Kahn's: start with indegree-0 nodes; process queue; detect cycles
}
```

**Example:**
```go
// Source: WDL 1.2 spec — call dependency from data flow
// call foo { input: x = bar.out }  → bar must run before foo
// call baz after foo { ... }        → foo must run before baz
```

### Pattern 2: Scheduler State Machine (Channel-Based Single Writer)

**What:** A single goroutine owns all mutable scheduler state (pending/running/done sets, output env). It dispatches goroutines via errgroup.Go and receives results via a buffered channel. No mutexes needed.

**When to use:** Always. This is the only safe design for scatter with `go test -race`.

```go
// Source: established pattern from ARCHITECTURE.md + PITFALLS.md research
type completionMsg struct {
    nodeID string
    env    *eval.Env   // nil on error
    err    error
}

func (s *scheduler) run(ctx context.Context) error {
    g, gCtx := errgroup.WithContext(ctx)
    g.SetLimit(8) // locked decision: fixed limit of 8

    resultCh := make(chan completionMsg, len(s.nodes))

    for len(s.done) < len(s.nodes) {
        // Dispatch all ready nodes
        for _, node := range s.readyNodes() {
            node := node // capture
            s.markRunning(node.id)
            g.Go(func() error {
                env, err := s.dispatch(gCtx, node)
                resultCh <- completionMsg{nodeID: node.id, env: env, err: err}
                return err // propagates cancellation via errgroup
            })
        }

        // Block until one node completes (or ctx cancelled)
        select {
        case msg := <-resultCh:
            if msg.err != nil {
                return msg.err // g.Wait() will handle cancellation
            }
            s.markDone(msg.nodeID, msg.env)
        case <-gCtx.Done():
            _ = g.Wait()
            return gCtx.Err()
        }
    }

    return g.Wait()
}
```

**Critical detail:** The `resultCh` buffer size must be `len(s.nodes)` (never unbuffered). If the scheduler exits before draining all results (on error path), goroutines must not block on `resultCh <-` or they leak. The large buffer ensures goroutines can always send even when the scheduler has stopped reading.

### Pattern 3: Scatter Fan-Out and Gather

**What:** Evaluate the scatter array expression, pre-allocate `results []values.Value`, dispatch one goroutine per element through the **global** errgroup (not a new local one), collect by index.

**When to use:** When executing an `ast.Scatter` node.

**Critical:** Use the SAME global errgroup (from the scheduler), not a new local `errgroup.WithContext`. A local errgroup ignores the global `SetLimit(8)`.

```go
// Source: locked decision from CONTEXT.md + PITFALLS.md Pitfall 6
func (s *scheduler) runScatter(gCtx context.Context, g *errgroup.Group,
    scatter *ast.Scatter, env *eval.Env) (values.Value, error) {

    arrVal, err := eval.Eval(scatter.Expr, env, eval.ExecContext{})
    // ... type assert to values.ArrayValue

    results := make([]values.Value, len(arr.Items))
    var mu sync.Mutex // ONLY to protect results slice writes by index

    for i, item := range arr.Items {
        i, item := i, item // capture
        g.Go(func() error {
            // Extend env with scatter variable
            scatterEnv := eval.NewEnv(env)
            scatterEnv.Define(scatter.Variable, item)
            // Execute scatter body (single call or nested structure)
            out, err := s.executeBody(gCtx, scatter.Body, scatterEnv)
            if err != nil {
                return err
            }
            mu.Lock()
            results[i] = out
            mu.Unlock()
            return nil
        })
    }
    // Note: caller calls g.Wait() — do not wait here
    // Return type: values.ArrayValue wrapping results
}
```

**Wait strategy:** The scatter dispatcher does NOT call `g.Wait()` — the scheduler's main loop does. This allows all scatter goroutines plus other concurrent nodes to share the single wait.

**Alternative for simple linear scatter:** When scatter contains a single `call`, each goroutine calls `task.RunTask` directly and writes to `results[i]`. When scatter contains nested scatter/if, recurse into `executeBody`.

### Pattern 4: Conditional Execution

**What:** Evaluate the boolean condition. When true, execute the body and bind outputs into the returned env. When false, bind `values.NullValue{}` for every output name in the body.

**When to use:** When executing an `ast.Conditional` node.

**Key insight:** The false-branch names must be discoverable without executing the body. Walk the body statically to collect all `ast.Call` aliases (call names) whose outputs will appear in scope as `T?` outside the conditional.

```go
// Source: WDL 1.2 spec + CONTEXT.md locked decision
func (s *scheduler) runConditional(ctx context.Context, cond *ast.Conditional,
    env *eval.Env) (*eval.Env, error) {

    condVal, err := eval.Eval(cond.Expr, env, eval.ExecContext{})
    // ... type assert to values.BooleanValue

    if bv.Value {
        // Execute body normally
        return s.runBody(ctx, cond.Body, env)
    }

    // False branch: bind None for all outputs in body scope
    falseEnv := eval.NewEnv(env)
    for _, name := range collectBodyOutputNames(cond.Body) {
        falseEnv.Define(name, values.NullValue{})
    }
    return falseEnv, nil
}
```

### Pattern 5: Callee Resolution

**What:** Map `ast.Call.Callee []string` to `*ast.Task`. Build a lookup table once at workflow start.

**Algorithm:**
- `ast.Document.Tasks` contains all tasks (including those from resolved imports, since the parser flattens imports into one Document per Phase 2 design).
- Key: `task.Name` (the last element of Callee, or the full dot-joined name for imported tasks).
- For Phase 6 (v1), use simple last-element lookup: `Callee[len(Callee)-1]`.

```go
// Source: wdl/ast/ast.go Document structure
func buildTaskIndex(doc *ast.Document) map[string]*ast.Task {
    idx := make(map[string]*ast.Task, len(doc.Tasks))
    for _, t := range doc.Tasks {
        idx[t.Name] = t
    }
    return idx
}
```

### Pattern 6: Call Input Binding

**What:** Evaluate each `ast.Call.Inputs map[string]Expr` expression against the current workflow env to produce a `map[string]interface{}` suitable for `task.RunTask`.

**Note:** `task.RunTask` accepts `map[string]interface{}` (JSON-like). The workflow scheduler must convert `values.Value` to their JSON-equivalent Go types before passing to RunTask.

```go
// Source: wdl/task/inputs.go + wdl/task/task.go signatures
func evalCallInputs(call *ast.Call, env *eval.Env) (map[string]interface{}, error) {
    result := make(map[string]interface{}, len(call.Inputs))
    for name, expr := range call.Inputs {
        v, err := eval.Eval(expr, env, eval.ExecContext{})
        if err != nil {
            return nil, err
        }
        result[name] = valueToJSON(v) // convert values.Value → interface{}
    }
    return result, nil
}
```

**Important:** `task.RunTask` re-evaluates these through `BindInputs`/`jsonToWDLValue`, so the conversion need not be perfectly typed — strings, ints, floats, bools, and nested slices/maps are sufficient.

### Pattern 7: Output JSON Serialization

**What:** Convert workflow output `values.Value` to JSON-compatible types for `outputs.json`. File/Directory paths must be made relative to `execRoot`.

**Key format:** `{"workflow_name.output_name": value}` (miniwdl-compatible, locked decision).

```go
// Source: CONTEXT.md locked decision + encoding/json stdlib
func writeOutputsJSON(wf *ast.Workflow, outputs map[string]values.Value,
    execRoot string, path string) error {

    data := make(map[string]interface{}, len(outputs))
    for name, val := range outputs {
        key := wf.Name + "." + name
        data[key] = valueToJSONOutput(val, execRoot) // relativize File/Directory
    }
    b, err := json.MarshalIndent(data, "", "  ")
    // ... write to path
}
```

### Anti-Patterns to Avoid

- **Local errgroup per scatter:** Creates a nested errgroup that ignores the global `SetLimit(8)`. All scatter goroutines MUST go through the single global errgroup.
- **Closing result channel from goroutines:** Always let the scheduler own channel lifecycle. "Send on closed channel" panic is non-recoverable.
- **Direct map writes from goroutines:** The scatter result slice is pre-allocated and indexed, requiring only a per-element mutex, not a map. Never write into a shared `map` from goroutines.
- **Blocking g.Wait() inside scatter dispatcher:** The scatter function dispatches goroutines but returns before they complete. The scheduler's main loop owns the single `g.Wait()`.
- **String-only callee lookup for nested calls:** For v1, last-element lookup is sufficient since Phase 2 flattens imports into one Document.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Bounded goroutine pool | Custom semaphore + WaitGroup | `errgroup.SetLimit(8)` | errgroup propagates the first error + cancels context automatically; WaitGroup has no cancellation |
| Cycle detection in DAG | DFS coloring | Kahn's algorithm (indegree count) | Kahn's is O(V+E), naturally detects cycles when nodes remain in pending after sort |
| JSON serialization | Custom marshaler | `encoding/json` with `map[string]interface{}` | All WDL value types map cleanly to Go primitives; no custom marshaler needed |
| Context propagation through goroutines | Manual done channels | `errgroup.WithContext` | The derived context is auto-cancelled on first error |

**Key insight:** The errgroup does not replace the scheduler state machine — it is a bounded goroutine launcher within it. The scheduler remains single-threaded; goroutines are only execution workers.

---

## Common Pitfalls

### Pitfall 1: Shared Output Map Written by Goroutines (Race Condition)
**What goes wrong:** Goroutines write their outputs into a shared `map[string]values.Value` concurrently. `go test -race` reports "concurrent map writes".
**Why it happens:** Natural first implementation — goroutines collect outputs and "send them back" by writing to a shared map.
**How to avoid:** Single-writer pattern. Goroutines send results via `resultCh`. Only the scheduler goroutine reads from `resultCh` and updates the output env. No mutex needed on the env.
**Warning signs:** Tests pass without `-race` flag but fail with `go test -race ./wdl/workflow/...`.

### Pitfall 2: Goroutine Leak on Errgroup Cancellation
**What goes wrong:** A scatter goroutine blocked on `task.RunTask` (waiting for Docker) does not check `ctx.Done()`. When the errgroup cancels the context, the goroutine continues running until Docker finishes.
**Why it happens:** `task.RunTask` already accepts `context.Context` — it propagates cancellation to Docker. This is NOT a problem IF `gCtx` (the errgroup's derived context) is passed to RunTask.
**How to avoid:** Always pass the errgroup's derived context (`gCtx` from `errgroup.WithContext`) to `task.RunTask`, not the original parent context.
**Warning signs:** Workflow cancellation leaves orphaned Docker containers visible in `docker ps`.

### Pitfall 3: Scatter Result Slice Index Mismatch
**What goes wrong:** Scatter results are appended to a shared slice without indexing. Goroutines complete out of order; the collected `Array[T]` has wrong element order.
**Why it happens:** `results = append(results, v)` is order-dependent on goroutine scheduling.
**How to avoid:** Pre-allocate `results := make([]values.Value, len(arr.Items))` and write by index `results[i] = v`. Protect index writes with a small `sync.Mutex` (write-by-index, not append).
**Warning signs:** Scatter output array has correct length but wrong element order; only detected by tests that check specific values.

### Pitfall 4: Forgetting to Collect Workflow Decl Outputs
**What goes wrong:** `ast.Workflow.Body` contains `*ast.Decl` nodes in addition to `*ast.Call`, `*ast.Scatter`, `*ast.Conditional`. Decls are simple variable bindings (e.g., `Boolean flag = true`). If the scheduler only handles Call/Scatter/Conditional, these decls are never bound into the env, causing "undefined variable" errors when later calls reference them.
**Why it happens:** The scheduler is designed around task dispatching; Decls feel trivial and are easy to overlook.
**How to avoid:** In the scheduler loop, handle `*ast.Decl` as a synchronous eval: `v, err := eval.Eval(decl.Expr, env, eval.ExecContext{})` and bind into env immediately. No goroutine needed.
**Warning signs:** Workflows with `Boolean flag = true` in the body followed by `if (flag)` fail with "undefined variable flag".

### Pitfall 5: Nested Scatter Uses Wrong errgroup
**What goes wrong:** `runScatter` creates a local `errgroup.WithContext` to manage its goroutines. Nested scatter-in-scatter creates two independent errgroups, each with their own limit. Total concurrent goroutines = 8 * 8 = 64.
**Why it happens:** Wanting a clean "local" errgroup for each scatter block.
**How to avoid:** Pass the single global errgroup `g` down through all recursive calls. All goroutines — regardless of nesting depth — share `g.SetLimit(8)`.
**Warning signs:** Nested scatter executes more than 8 tasks simultaneously (check Docker `docker stats` output).

### Pitfall 6: Static Walk for False-Branch Names Misses Nested Calls
**What goes wrong:** When evaluating `if(false)`, the scheduler must bind `NullValue` for ALL call outputs that would have been produced. A shallow walk only finds top-level calls; calls nested inside scatter blocks inside the conditional are missed.
**Why it happens:** `collectBodyOutputNames` only recurses one level.
**How to avoid:** Recursively walk all `ast.WorkflowNode` types (`*ast.Call`, `*ast.Scatter`, `*ast.Conditional`) to collect all names that would be added to scope.
**Warning signs:** `call_outside_if` that references `if_scatter_inner_call.output` gets "undefined variable" when the if-condition is false.

---

## Code Examples

Verified patterns from project codebase and official sources:

### errgroup Setup (Locked Decision)
```go
// Source: pkg.go.dev/golang.org/x/sync@v0.20.0/errgroup
import "golang.org/x/sync/errgroup"

g, gCtx := errgroup.WithContext(ctx)
g.SetLimit(8) // fixed limit — locked decision from CONTEXT.md

// Dispatch a node:
g.Go(func() error {
    out, err := task.RunTask(gCtx, astTask, jsonInputs, runner, execDir)
    resultCh <- completionMsg{nodeID: id, outputs: out, err: err}
    return err // non-nil cancels gCtx
})

if err := g.Wait(); err != nil {
    // first non-nil error from any goroutine
}
```

### Workflow Env Construction
```go
// Source: wdl/eval/env.go — established project pattern
workflowEnv := eval.NewEnv(nil)

// Bind workflow inputs from JSON
for _, decl := range wf.Inputs {
    val, err := bindWorkflowInput(decl, jsonInputs)
    // ...
    workflowEnv.Define(decl.Name, val)
}

// After call completes, bind outputs as "callAlias.outputName"
for name, val := range callOutputs {
    workflowEnv.Define(callAlias+"."+name, val)
}
```

### Kahn's Topological Sort
```go
// Source: classic algorithm — O(V+E), detects cycles naturally
func topoSort(nodes map[string]*dagNode) ([]string, error) {
    queue := []string{}
    for id, n := range nodes {
        if n.indegree == 0 {
            queue = append(queue, id)
        }
    }
    var order []string
    for len(queue) > 0 {
        id := queue[0]
        queue = queue[1:]
        order = append(order, id)
        for _, succ := range nodes[id].successors {
            nodes[succ].indegree--
            if nodes[succ].indegree == 0 {
                queue = append(queue, succ)
            }
        }
    }
    if len(order) != len(nodes) {
        return nil, fmt.Errorf("cycle detected in workflow dependency graph")
    }
    return order, nil
}
```

### outputs.json Writing
```go
// Source: encoding/json stdlib + CONTEXT.md locked format
func writeOutputsJSON(wfName string, outputs map[string]values.Value,
    execRoot, destPath string) error {
    data := make(map[string]interface{}, len(outputs))
    for name, val := range outputs {
        data[wfName+"."+name] = valueToJSONSerializable(val, execRoot)
    }
    b, err := json.MarshalIndent(data, "", "  ")
    if err != nil {
        return fmt.Errorf("serializing outputs: %w", err)
    }
    return os.WriteFile(destPath, b, 0644)
}

// File/Directory: make relative to execRoot
func valueToJSONSerializable(v values.Value, execRoot string) interface{} {
    switch val := v.(type) {
    case values.FileValue:
        rel, _ := filepath.Rel(execRoot, val.Value)
        return rel
    case values.DirectoryValue:
        rel, _ := filepath.Rel(execRoot, val.Value)
        return rel
    case values.ArrayValue:
        items := make([]interface{}, len(val.Items))
        for i, item := range val.Items {
            items[i] = valueToJSONSerializable(item, execRoot)
        }
        return items
    case values.NullValue:
        return nil
    // ... other types map directly
    }
}
```

### error.json Writing
```go
// Source: CONTEXT.md locked format
type errorJSON struct {
    Error    string `json:"error"`
    Task     string `json:"task"`
    ExitCode int    `json:"exit_code"`
}

func writeErrorJSON(te *task.TaskError, destPath string) error {
    data := errorJSON{
        Error:    te.Error(),
        Task:     te.TaskName,
        ExitCode: te.ExitCode,
    }
    b, err := json.MarshalIndent(data, "", "  ")
    if err != nil {
        return fmt.Errorf("serializing error: %w", err)
    }
    return os.WriteFile(destPath, b, 0644)
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `sync.WaitGroup` for scatter | `errgroup.SetLimit` + `WithContext` | errgroup added SetLimit in x/sync v0.1.0 (2022) | Automatic context cancellation; bounded concurrency in one call |
| Mutex on shared output map | Channel + single-writer scheduler | Established pattern in prior research | Eliminates all data races; `go test -race` clean |

**Deprecated/outdated:**
- `sync.WaitGroup` alone: lacks context cancellation; goroutine leaks on error paths. Never use alone for scatter.
- Global errgroup shared across multiple workflow runs: errgroups are single-use; create a new one per `RunWorkflow` call.

---

## Open Questions

1. **How to convert `values.Value` back to `map[string]interface{}` for `task.RunTask`**
   - What we know: `task.RunTask` accepts `map[string]interface{}` (JSON-decoded types). `task.BindInputs` calls `jsonToWDLValue` to convert back.
   - What's unclear: Whether passing typed values (e.g., `values.IntValue`) directly wrapped as `interface{}` works, or whether a full serialization round-trip is needed.
   - Recommendation: Implement a `valuesToJSONMap` helper that converts `values.Value` → Go primitives (int64, float64, string, bool, []interface{}, map[string]interface{}). This is the most robust approach and avoids depending on `BindInputs` internals.

2. **Execution directory structure for scatter elements**
   - What we know: `task.RunTask` creates `execRoot/taskName/` for a single task. For scatter, multiple elements run the same task.
   - What's unclear: How to separate scatter element execution directories to avoid collision.
   - Recommendation: Use `execRoot/taskName/scatter_N/` where N is the scatter index. Pass a modified `execRoot` per element: `filepath.Join(execRoot, fmt.Sprintf("scatter_%d", i))`.

3. **Whether workflow Decls need a separate DAG node type**
   - What we know: `ast.Decl` in `Workflow.Body` are sequential value bindings. They may reference prior call outputs.
   - What's unclear: Whether Decls should participate in the DAG (with edges to the calls they reference) or be processed sequentially before/between call dispatches.
   - Recommendation: Process Decls synchronously in body order within the scheduler loop, before checking for ready call/scatter/conditional nodes. This avoids adding Decl nodes to the DAG and keeps the scheduler simpler.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` package (stdlib) |
| Config file | none — `go test ./wdl/workflow/...` |
| Quick run command | `go test ./wdl/workflow/... -count=1` |
| Full suite command | `go test -race ./wdl/workflow/... -count=1` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| WF-01 | DAG built in correct dependency order; tasks with no deps run first | unit | `go test ./wdl/workflow/... -run TestBuildDAG -count=1` | ❌ Wave 0 |
| WF-02 | Single call executes and binds output into workflow env | integration | `go test ./wdl/workflow/... -run TestSingleCall -count=1` | ❌ Wave 0 |
| WF-03 | Scatter fans out over 3-element array; outputs collected as Array[T] | integration | `go test ./wdl/workflow/... -run TestScatter -count=1` | ❌ Wave 0 |
| WF-04 | If(true) executes body; if(false) binds NullValue for all body outputs | unit | `go test ./wdl/workflow/... -run TestConditional -count=1` | ❌ Wave 0 |
| WF-05 | `after` dependency respected: task B never starts before task A | integration | `go test ./wdl/workflow/... -run TestAfterDependency -count=1` | ❌ Wave 0 |
| WF-06 | Scatter goroutine count never exceeds 8 (verified via mock counting runner) | unit | `go test -race ./wdl/workflow/... -run TestScatterConcurrencyLimit -count=1` | ❌ Wave 0 |
| WF-07 | outputs.json written with correct miniwdl-format keys; File paths relative | integration | `go test ./wdl/workflow/... -run TestOutputsJSON -count=1` | ❌ Wave 0 |
| WF-08 | Failing task cancels others; error.json written; outputs.json absent | integration | `go test ./wdl/workflow/... -run TestFailFast -count=1` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./wdl/workflow/... -count=1`
- **Per wave merge:** `go test -race ./wdl/workflow/... -count=1`
- **Phase gate:** Full suite green (`go test -race ./...`) before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `wdl/workflow/workflow_test.go` — covers WF-01 through WF-08; uses ShellRunner mock (no Docker)
- [ ] `wdl/workflow/workflow.go` — package stub with `RunWorkflow` signature
- [ ] `wdl/workflow/dag.go` — package stub for `buildDAG`
- [ ] Add `golang.org/x/sync` to `go.mod`: `go get golang.org/x/sync@v0.20.0`

---

## Sources

### Primary (HIGH confidence)
- `wdl/task/task.go` — `RunTask` signature and behavior (project source, confirmed)
- `wdl/eval/env.go` — `Env` API: `NewEnv`, `Define`, `Lookup`, `ForEach`, `Parent` (project source)
- `wdl/ast/ast.go` — `ast.Call`, `ast.Scatter`, `ast.Conditional`, `ast.Workflow`, `ast.Document` structures (project source)
- `wdl/values/values.go` — all `values.Value` concrete types including `NullValue`, `ArrayValue` (project source)
- `pkg.go.dev/golang.org/x/sync@v0.20.0/errgroup` — `WithContext`, `SetLimit`, `Go`, `Wait` signatures (official docs, fetched 2026-03-10)
- `.planning/phases/06-workflow-execution-dag-scheduler/06-CONTEXT.md` — all locked decisions

### Secondary (MEDIUM confidence)
- `.planning/research/ARCHITECTURE.md` — scheduler state machine pattern; channel-based single-writer design
- `.planning/research/PITFALLS.md` — Pitfalls 6 (goroutine leaks), 14 (concurrent map writes), confirmed by prior research

### Tertiary (LOW confidence)
- None — all findings verified against project source or official docs.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — errgroup v0.20.0 confirmed via pkg.go.dev; encoding/json and context are stdlib
- Architecture: HIGH — channel-based scheduler and Kahn's algorithm are well-established; directly grounded in existing project patterns
- Pitfalls: HIGH — all pitfalls verified against prior research (PITFALLS.md) and existing codebase patterns

**Research date:** 2026-03-10
**Valid until:** 2026-09-10 (errgroup API stable; locked decisions don't change)
