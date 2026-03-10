---
phase: 06-workflow-execution-dag-scheduler
plan: 02
subsystem: workflow
tags: [scheduler, scatter, conditional, errgroup, dag-execution, workflow-engine]
dependency_graph:
  requires: [wdl/ast, wdl/eval, wdl/task, wdl/values, wdl/workflow/dag, wdl/workflow/convert, golang.org/x/sync/errgroup]
  provides: [wdl/workflow (RunWorkflow fully implemented)]
  affects: [integration tests, end-to-end workflow execution]
tech_stack:
  added: [golang.org/x/sync v0.20.0]
  patterns: [channel-based single-writer scheduler, StructValue output binding for GetExpr compatibility, errgroup.SetLimit(8) for scatter concurrency, per-element scatter env via eval.NewEnv]
key_files:
  created:
    - wdl/workflow/scheduler.go
    - wdl/workflow/scatter.go
    - wdl/workflow/conditional.go
    - wdl/workflow/scheduler_test.go
  modified:
    - wdl/workflow/workflow.go
    - go.mod
    - go.sum
decisions:
  - "Call outputs bound as StructValue under call alias (not flat 'alias.outputName' keys) — enables GetExpr evaluation via eval.evalGet which expects struct field access"
  - "taskRunner field on scheduler struct propagates mock function to child schedulers (prevents global runTaskFn races in body/scatter/conditional sub-execution)"
  - "injectDeclNodes() merges Decl synthetic nodes into topo order at correct body positions — Decls excluded from buildDAG but must execute synchronously before downstream nodes"
  - "runScatter uses global errgroup g.Go() for concurrency limiting + buffered elemCh for index-ordered result collection — avoids local g.Wait() deadlock"
  - "transposeStructArray() converts per-element {alias→StructValue} into {alias→StructValue{field→ArrayValue}} — enables d.out GetExpr on scatter results"
  - "Tests use global runTaskFn override with defer restore (sequential execution avoids parallel override races)"
metrics:
  duration: 9min
  completed_date: "2026-03-10"
  tasks_completed: 2
  files_created: 4
  files_modified: 3
---

# Phase 6 Plan 2: Scheduler Core Implementation Summary

**One-liner:** DAG scheduler with StructValue output binding, scatter fan-out via errgroup.SetLimit(8), and conditional NullValue fallback for false branches.

## What Was Built

The `wdl/workflow` package gains its full execution engine:

- **scheduler.go**: `scheduler` struct with channel-based single-writer pattern. `run()` processes nodes in topological order, dispatching Decls synchronously and Calls/Scatter/Conditionals via errgroup goroutines. A `resultCh` buffered channel carries `completionMsg` results back. The `taskRunner` field enables clean test mocking without global variable races.

- **conditional.go**: `runConditional()` evaluates the condition expression; true branch runs `executeBody()`, false branch runs `falseBodyOutputs()` which binds `NullValue` for each call alias (representing Optional None). `executeBody()` handles sequential body execution for conditional true-branches and scatter elements, dispatching nested Scatter/Conditional/Call/Decl nodes recursively.

- **scatter.go**: `runScatter()` fans out over ArrayValue elements using the global errgroup (with SetLimit(8)). Per-element results flow via a buffered `elemCh` channel for index-ordered collection. `transposeScatterResults()` converts `results[i][alias] = StructValue{field: val}` into `{alias: StructValue{field: ArrayValue{[val0, val1, ...]}}}`. Supports nested scatter-in-scatter and if-in-scatter.

- **workflow.go** (updated): `RunWorkflow()` now fully implemented — finds workflow, builds task index, binds inputs, builds DAG, injects Decl nodes into topo order via `injectDeclNodes()`, creates and runs scheduler, evaluates workflow output declarations.

## Tasks Completed

| Task | Description | Commit | Files |
|------|-------------|--------|-------|
| 1 | Scheduler state machine: call, conditional, Decl | fffc7ac | scheduler.go, conditional.go, workflow.go, scheduler_test.go, go.mod, go.sum |
| 2 | Scatter fan-out with errgroup concurrency limit | 199d621 | scatter.go |

## Test Coverage

31 tests total (18 new), all passing under `-race`:

**New scheduler tests (18):**
- `TestRunWorkflow_SingleCall` — single task executes, outputs bound
- `TestRunWorkflow_TwoCalls_DataDep` — B uses A.result → A runs first
- `TestRunWorkflow_TwoCalls_Parallel` — independent calls both execute
- `TestRunWorkflow_AfterDep` — explicit `after` dep respected
- `TestRunWorkflow_Conditional_True/False` — true branch executes, false binds NullValue
- `TestRunWorkflow_Decl` — `Int x = 42` evaluated before call that uses x
- `TestRunWorkflow_Scatter_Basic` — scatter over [1,2,3] runs 3 elements
- `TestRunWorkflow_Scatter_Basic_CheckOutputs` — verifies Array[T] output with correct values [2,4,6]
- `TestRunWorkflow_Scatter_Empty` — empty scatter produces empty ArrayValue
- `TestRunWorkflow_ScatterConcurrencyLimit` — 20 elements, max concurrent never exceeds 8
- `TestRunWorkflow_ScatterInIf_True/False` — scatter inside if produces Array or NullValue
- `TestRunWorkflow_IfInScatter` — conditional inside scatter element produces per-element results
- `TestRunWorkflow_ScatterInScatter` — nested scatter produces nested Array structure
- `TestRunWorkflow_WorkflowOutputs` — workflow output decls evaluated and returned
- `TestRunWorkflow_NotFound` — unknown workflow name returns error
- `TestRunWorkflow_TaskNotFound` — call to undefined task returns error

## Decisions Made

1. **StructValue output binding**: Call outputs are bound as `env.Define(alias, StructValue{Fields: rawOutputs})`. This matches how `eval.evalGet()` resolves `alias.field` — it calls `env.Lookup(alias)` and expects a StructValue. The alternative (flat `"alias.outputName"` keys) would require a custom GetExpr handler.

2. **taskRunner field propagation**: The scheduler carries a `taskRunner func(...)` field that child schedulers inherit via `childScheduler()`. This allows tests to override the runner per-instance via the global `runTaskFn` default, without races between parallel test invocations. Tests restore `runTaskFn` via `defer`.

3. **injectDeclNodes() insertion**: `buildDAG` skips Decls (they have no DAG structure). `injectDeclNodes()` re-merges them at correct body positions using a two-pointer merge between body order and topo order. Each Decl gets a synthetic `"decl_N"` ID.

4. **Scatter result transposition**: `transposeScatterResults()` handles the per-element `{alias→StructValue}` → `{alias→StructValue{field→Array}}` transformation. This allows `scatter(x in arr) { call t }` to bind `t.out` as `Array[T]` in the workflow env.

5. **errgroup.Wait() placement**: `runScatter` does not call `g.Wait()` itself — callers do. When called from `dispatch()`, the caller creates a local errgroup and waits. When called from `executeBody()`, the same pattern applies.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed output binding format for GetExpr compatibility**
- **Found during:** Task 1 implementation
- **Issue:** Plan specified flat `"callAlias.outputName"` key binding, but `eval.evalGet` resolves `alias.field` by looking up `alias` as a StructValue, not a flat string key
- **Fix:** Bound outputs as `env.Define(alias, StructValue{Fields: rawOutputs})` instead of flat keys
- **Files modified:** scheduler.go, conditional.go, scatter.go, scheduler_test.go

**2. [Rule 2 - Critical functionality] Added taskRunner field to scheduler struct**
- **Found during:** Task 1 — parallel test interference detected
- **Issue:** Global `runTaskFn` override in parallel tests causes data races; child schedulers in executeBody/scatter created without access to the override
- **Fix:** Added `taskRunner` field to scheduler struct; `childScheduler()` propagates it; `runCall()` uses `s.taskRunner ?? runTaskFn` fallback
- **Files modified:** scheduler.go, conditional.go

**3. [Rule 1 - Bug] Fixed test expressions to use memberExpr not identExpr for "d.out"**
- **Found during:** Task 2 test failures
- **Issue:** Test workflow output expressions used `identExpr("d.out")` (flat IdentExpr) instead of `memberExpr("d", "out")` (GetExpr)
- **Fix:** Changed to `memberExpr("d", "out")` to match actual WDL expression semantics
- **Files modified:** scheduler_test.go

## Self-Check: PASSED

Files created:
- wdl/workflow/scheduler.go: FOUND
- wdl/workflow/scatter.go: FOUND
- wdl/workflow/conditional.go: FOUND
- wdl/workflow/scheduler_test.go: FOUND

Files modified:
- wdl/workflow/workflow.go: FOUND

Commits:
- fffc7ac: feat(06-02): implement scheduler state machine with call, conditional, and Decl execution: FOUND
- 199d621: feat(06-02): implement scatter fan-out with global errgroup and concurrency limit: FOUND
