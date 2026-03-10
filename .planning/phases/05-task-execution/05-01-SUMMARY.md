---
phase: 05-task-execution
plan: "01"
subsystem: wdl/task
tags: [task-execution, container, path-mapping, input-binding, runtime-parsing]

dependency_graph:
  requires:
    - wdl/eval (eval.Eval, eval.Env, eval.ExecContext)
    - wdl/ast (ast.Task, ast.Decl)
    - wdl/values (all Value types)
    - wdl/types (all Type structs)
  provides:
    - ContainerRunner interface
    - RunRequest, RunResult structs
    - TaskError type
    - PathMap with bidirectional host/container path translation
    - BindInputs for JSON-to-WDL input conversion
    - ParseMemoryString for WDL memory unit parsing
    - ParseRuntime for docker image/cpu/memory extraction
  affects:
    - Plan 05-02 (command evaluation and output binding build on BindInputs and PathMap)
    - Plan 05-03 (DockerRunner implements ContainerRunner, receives RunRequest)

tech_stack:
  added:
    - wdl/task package (new)
  patterns:
    - TDD red-green cycle (failing tests committed before implementation)
    - Bidirectional map for O(1) path translation in both directions
    - SHA-256 hash prefix for filename collision avoidance

key_files:
  created:
    - wdl/task/runner.go
    - wdl/task/task.go
    - wdl/task/pathmap.go
    - wdl/task/inputs.go
    - wdl/task/memory.go
    - wdl/task/runtime.go
    - wdl/task/pathmap_test.go
    - wdl/task/inputs_test.go
    - wdl/task/memory_test.go
    - wdl/task/runtime_test.go
  modified: []

decisions:
  - "PathMap collision strategy: 8 hex chars from SHA-256 of host path as prefix (e.g., /mnt/inputs/a1b2c3d4-input.bam) — deterministic, compact, avoids runtime state"
  - "BindInputs and ParseRuntime exported (capital B/P) — test package is task_test (external), requiring exported symbols; plan used lowercase names but external test package mandates export"
  - "ParseMemoryString exported as well for the same reason"
  - "jsonToWDLValue falls back to jsonToValueUntyped (copy of eval.jsonToValue logic) for AnyType — avoids cross-package dependency on unexported eval.jsonToValue"

metrics:
  duration: "4 minutes"
  completed_date: "2026-03-10"
  tasks_completed: 2
  files_created: 10
---

# Phase 5 Plan 1: wdl/task Package Foundation Summary

**One-liner:** wdl/task package with ContainerRunner interface, bidirectional PathMap with SHA-256 collision handling, type-aware JSON input binding, WDL memory unit parser, and Requirements-first runtime section parser.

## Tasks Completed

| Task | Description | Commit | Files |
|------|-------------|--------|-------|
| 1 | Package contracts: ContainerRunner, TaskError, PathMap | 145933c | runner.go, task.go, pathmap.go, pathmap_test.go |
| 2 | Input binding, memory parsing, runtime parsing | dc2bbb6 | inputs.go, memory.go, runtime.go + 3 test files |

## What Was Built

### ContainerRunner Interface (runner.go)
Defines `RunRequest` (Image, Command, Binds, Env, Memory, NanoCPUs, WorkDir), `RunResult` (ExitCode, Stdout, Stderr), and the `ContainerRunner` interface with `Run(ctx, RunRequest) (RunResult, error)`. Plan 03 (DockerRunner) implements this interface.

### TaskError (task.go)
Carries `TaskName`, `ExitCode`, and `Stderr []byte`. `Error()` produces a human-readable message including all three fields. `RunTask` stub returns "not yet implemented" — wired in Plan 02.

### PathMap (pathmap.go)
Maintains `hostToContainer` and `containerToHost` maps for O(1) bidirectional lookup. `AddFile` and `AddDir` assign paths under `/mnt/inputs/`; collisions are resolved with an 8-hex-char SHA-256 prefix of the host path. `TranslateValue` and `UntranslateValue` recurse into Array, Map, Pair, and Struct values. `Binds()` returns `host:container:ro` Docker mount specs.

### BindInputs (inputs.go)
Walks `task.Inputs` declarations. JSON values are converted via `jsonToWDLValue` (type-aware: string → FileValue for FileType, float64 → IntValue for IntType). Missing required inputs produce an error; missing optional inputs bind `NullValue{}`. Default expressions are evaluated via `eval.Eval`. `collectPaths` registers all File/Directory values in the PathMap. `env=true` declarations produce `NAME=value` env var strings.

### ParseMemoryString (memory.go)
Handles IEC binary units (KiB/MiB/GiB/TiB), SI decimal units (kB/MB/GB/TB), "B"/"bytes", bare integers, and float values (e.g., "1.5 GiB").

### ParseRuntime (runtime.go)
Evaluates `task.Requirements` first (WDL 1.2 preferred), falls back to `task.Runtime`. "container" key takes precedence over "docker" for image; default is "ubuntu:latest". CPU is converted to nanocpus (× 1e9). Memory is parsed with ParseMemoryString.

## Test Results

All 27 tests pass:
- 9 PathMap/TaskError tests
- 8 BindInputs tests
- 17 ParseMemoryString cases
- 6 ParseRuntime tests

Full project suite: 8/8 packages green.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Exported symbols] BindInputs, ParseMemoryString, ParseRuntime exported**
- **Found during:** Task 1 (RED phase — test file uses `package task_test` external test package)
- **Issue:** Plan action specified lowercase `bindInputs`, `parseMemoryString`, `parseRuntime`. External test packages cannot access unexported identifiers.
- **Fix:** Exported all three function names. No behavior change.
- **Files modified:** inputs.go, memory.go, runtime.go, inputs_test.go, memory_test.go, runtime_test.go

**2. [Rule 2 - Missing unexported copy] jsonToValueUntyped duplicated from eval package**
- **Found during:** Task 2 implementation
- **Issue:** `eval.jsonToValue` is unexported; inputs.go cannot call it across packages.
- **Fix:** Added `jsonToValueUntyped` in inputs.go — identical logic, local to the task package.
- **Files modified:** inputs.go

## Self-Check: PASSED
