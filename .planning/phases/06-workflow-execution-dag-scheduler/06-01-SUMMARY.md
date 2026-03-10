---
phase: 06-workflow-execution-dag-scheduler
plan: 01
subsystem: workflow
tags: [dag, topological-sort, value-conversion, workflow-execution]
dependency_graph:
  requires: [wdl/ast, wdl/eval, wdl/task, wdl/values]
  provides: [wdl/workflow]
  affects: [06-02-scheduler]
tech_stack:
  added: [wdl/workflow package]
  patterns: [Kahn's algorithm for topological sort, recursive expression walking for data-flow edge detection]
key_files:
  created:
    - wdl/workflow/workflow.go
    - wdl/workflow/dag.go
    - wdl/workflow/dag_test.go
    - wdl/workflow/convert.go
    - wdl/workflow/convert_test.go
  modified: []
decisions:
  - "dagNode tracks top-level workflow elements (call, scatter, conditional) as execution units; nested calls inside scatter/conditional are NOT separate DAG nodes"
  - "callID() returns alias if set, else last element of callee — matches WDL spec for call references"
  - "topoSort uses Kahn's algorithm with sorted queue for deterministic ordering of independent nodes"
  - "valueToJSON converts MapValue to map[string]interface{} with string keys (non-string keys formatted via fmt.Sprintf)"
  - "jsonToValueUntyped duplicated from task package (task.jsonToWDLValue unexported) — same decision as Phase 05"
  - "collectExprDeps recursively walks full expression tree (binary, unary, index, apply, array, map, pair) for thorough data-flow edge detection"
metrics:
  duration: 2min
  completed_date: "2026-03-10"
  tasks_completed: 2
  files_created: 5
  files_modified: 0
---

# Phase 6 Plan 1: Workflow Package DAG Construction Summary

**One-liner:** DAG construction for WDL workflows using Kahn's topological sort with data-flow and after-keyword edge detection.

## What Was Built

The `wdl/workflow` package provides the dependency resolution foundation for the scheduler (Plan 02):

- **dag.go**: `dagNode` struct, `buildDAG()` function that constructs a DAG from workflow body elements, and `topoSort()` using Kahn's algorithm
- **workflow.go**: `RunWorkflow` public entry point stub and `buildTaskIndex` helper
- **convert.go**: `valueToJSON` value converter, `evalCallInputs` for call input preparation, `bindWorkflowInputs` for workflow-level input binding

## Tasks Completed

| Task | Description | Commit | Files |
|------|-------------|--------|-------|
| 1 | DAG construction and topological sort | c728253 | workflow.go, dag.go, dag_test.go |
| 2 | valueToJSON converter and workflow input binding | 8e769b8 | convert.go, convert_test.go |

## Test Coverage

13 tests total, all passing:

**DAG tests (6):**
- `TestBuildDAG_LinearChain` — B uses A.output => topological order [A, B]
- `TestBuildDAG_Parallel` — independent calls A, B both have indegree 0
- `TestBuildDAG_AfterDependency` — B after A creates explicit ordering edge
- `TestBuildDAG_Cycle` — circular dependency A->B->A returns CycleError
- `TestBuildDAG_CallAlias` — call alias used as node ID, not callee name
- `TestBuildDAG_MixedDataFlowAndAfter` — C depends on A (data-flow) and B (after)

**Convert tests (7):**
- `TestValueToJSON_Primitives` — Int, Float, String, Bool conversions
- `TestValueToJSON_File` — FileValue and DirectoryValue to string paths
- `TestValueToJSON_Array` — recursive array conversion
- `TestValueToJSON_Map` — map with StringValue keys
- `TestValueToJSON_Null` — NullValue to nil
- `TestValueToJSON_Nested` — nested Array[Map[String, String]]
- `TestEvalCallInputs` — evaluates input expressions and converts results

## Decisions Made

1. **DAG scope**: Top-level `*ast.Call`, `*ast.Scatter`, `*ast.Conditional` are DAG nodes. Calls nested inside scatter/conditional blocks are NOT separate DAG nodes — they are owned by their enclosing block and executed by the scheduler as a unit.

2. **Call ID**: `callID()` returns `call.Alias` if non-empty, else `call.Callee[len(Callee)-1]`. This matches WDL's reference resolution (other calls reference by alias, not full callee path).

3. **Deterministic sort**: `topoSort()` maintains an alphabetically sorted queue so independent nodes always appear in the same order across runs.

4. **collectExprDeps recursion**: The expression walker handles all expression types (binary, unary, if-then-else, apply, array, map, pair, index) to ensure data-flow edges are detected regardless of expression complexity.

5. **jsonToValueUntyped duplication**: The `task.jsonToWDLValue` function is unexported, so a local `jsonToValueUntyped` is provided. This is consistent with the pattern established in Phase 05.

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check: PASSED

Files created:
- wdl/workflow/workflow.go: FOUND
- wdl/workflow/dag.go: FOUND
- wdl/workflow/dag_test.go: FOUND
- wdl/workflow/convert.go: FOUND
- wdl/workflow/convert_test.go: FOUND

Commits:
- c728253: feat(06-01): create wdl/workflow package with DAG construction and topological sort: FOUND
- 8e769b8: feat(06-01): implement valueToJSON converter and workflow input binding: FOUND
