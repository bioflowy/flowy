# Phase 6: Workflow Execution (DAG Scheduler) - Context

**Gathered:** 2026-03-10
**Status:** Ready for planning

<domain>
## Phase Boundary

Execute multi-task WDL workflows in correct dependency order: build a DAG from data flow and `after` keywords, run tasks via the existing `task.RunTask` orchestrator, support scatter parallelism with bounded concurrency, conditional optional wrapping, and write structured JSON output. Single-task execution (Phase 5) and CLI integration (Phase 7) are separate phases.

</domain>

<decisions>
## Implementation Decisions

### Scatter concurrency
- Fixed concurrency limit of 8 via `errgroup.SetLimit(8)`
- Global shared limit across all scatter blocks, including nested scatter-in-scatter — prevents resource explosion
- No CLI override in v1; concurrency is hardcoded (configurable override deferred to v2)

### Task failure behavior
- Fail-fast: on first task failure, cancel all running tasks immediately via `context.Cancel()`
- All downstream and unrelated tasks are stopped — no partial execution continues
- Only `error.json` is written on failure; `outputs.json` is NOT written
- error.json contains minimal info: `{"error": "message", "task": "task_name", "exit_code": N}` — stderr available in the task's `stderr.txt` file

### Nested scatter/if output types
- WDL spec compliant: `scatter(if(call))` → `Array[T?]`, `if(scatter(call))` → `Array[T]?`
- scatter-in-scatter supported in v1 — produces `Array[Array[T]]`
- `if(false) { scatter(...) { call foo } }` → `None` (the if block is not entered, scatter never executes)
- Phase 3 type checker already implements these promotion rules; runtime must match

### outputs.json format
- miniwdl-compatible key format: `{"workflow_name.output_name": value, ...}` — flat key-value structure
- File/Directory values written as relative paths from the execution directory (not absolute host paths)
- Written only on successful completion of all tasks

### error.json format
- Minimal structure: `{"error": "human-readable message", "task": "failing_task_name", "exit_code": N}`
- Written when any task fails; stderr content is in the task's execution directory (`stderr.txt`), not duplicated in error.json

### Claude's Discretion
- DAG construction algorithm (topological sort approach, adjacency list representation)
- How to resolve `call` callee names to `*ast.Task` nodes (lookup strategy)
- Internal scheduler state machine design
- How to pass workflow-level input bindings through to individual call input expressions
- Error wrapping and propagation strategy within the scheduler

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `task.RunTask(ctx, task, jsonInputs, runner, execRoot)`: Single-task executor — Phase 6 calls this for each `call` statement in the DAG
- `task.ContainerRunner` interface: `Run(ctx, RunRequest) (RunResult, error)` — passed through from workflow to task
- `eval.Eval(expr, env, ctx)`: Expression evaluator — needed to evaluate `call` input bindings, scatter array expressions, and conditional expressions
- `eval.Env`: Linked-list scope — workflow-level env holds call outputs; scatter extends with loop variable
- `types.Coerces()`: Type coercion — Phase 3 rules already handle scatter→Array[T] and if→T? promotion

### Established Patterns
- All errors include `ast.Pos` (File, Line, Col) where applicable
- `values.NullValue{}` is the sentinel for absent optionals (not nil)
- Package per concern: `wdl/task`, `wdl/eval`, `wdl/checker` — new package likely `wdl/workflow`
- `context.Context` threading for cancellation (already in RunTask signature)

### Integration Points
- Input: `*ast.Document` (contains workflows, tasks, structs) + JSON inputs + `ContainerRunner` + exec root path
- Output: `map[string]values.Value` (workflow outputs) or error — plus `outputs.json`/`error.json` written to disk
- AST nodes: `ast.Workflow.Body` is `[]WorkflowNode` containing `*ast.Call`, `*ast.Scatter`, `*ast.Conditional`, `*ast.Decl`
- `ast.Call.After` field: `[]string` for explicit ordering dependencies
- `ast.Call.Inputs`: `map[string]Expr` for input bindings that need evaluation against current scope
- Phase 7 CLI calls workflow executor as the main entry point for `flowy run`

</code_context>

<specifics>
## Specific Ideas

- Follow miniwdl's outputs.json format for WDL ecosystem compatibility
- File paths in outputs.json are relative to execution directory for portability
- Global shared concurrency limiter keeps resource usage predictable regardless of scatter nesting depth
- Fail-fast with immediate cancellation matches miniwdl behavior and prevents wasted computation

</specifics>

<deferred>
## Deferred Ideas

- Configurable concurrency limit via CLI flag (`--concurrency`) — v2
- Resource-aware scheduling (CPU/memory-based concurrency) — v2
- Partial execution output on failure (error.json + partial outputs.json) — v2
- Sub-workflow calling (workflow calling another workflow) — out of scope per REQUIREMENTS.md
- Task output caching/memoization — v2 (EXT-02)

</deferred>

---

*Phase: 06-workflow-execution-dag-scheduler*
*Context gathered: 2026-03-10*
