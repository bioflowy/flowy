---
phase: 05-task-execution
plan: "02"
subsystem: wdl/task
tags: [task-execution, command-expansion, output-collection, shell-runner, end-to-end]

dependency_graph:
  requires:
    - wdl/eval (eval.Eval, eval.Env, eval.ExecContext)
    - wdl/ast (ast.Task, ast.Decl, ast.CommandExpr)
    - wdl/values (all Value types)
    - wdl/types (FileType, DirectoryType)
    - wdl/task/pathmap.go (TranslateValue, UntranslateValue, Binds)
    - wdl/task/inputs.go (BindInputs)
    - wdl/task/runtime.go (ParseRuntime)
  provides:
    - BuildCommand: expands command template with container-translated env, writes script to disk
    - CollectOutputs: evaluates output declarations with ExecContext, returns host-path values
    - RunTask: fully wired orchestrator for end-to-end WDL task execution
    - ShellRunner: host-native mock ContainerRunner for testing without Docker
  affects:
    - Plan 05-03 (DockerRunner implements ContainerRunner; receives same RunRequest as ShellRunner)

tech_stack:
  added:
    - wdl/task/command.go (BuildCommand, translateEnv)
    - wdl/task/outputs.go (CollectOutputs, coerceOutputValue)
  patterns:
    - Sentinel-before-indent dedent algorithm (inherited from eval/command.go)
    - Output type coercion: StringValue -> FileValue/DirectoryValue based on decl type
    - ShellRunner mock bypasses Docker by executing script files directly on host
    - ForEach/Parent on eval.Env expose scope iteration without breaking encapsulation

key_files:
  created:
    - wdl/task/command.go
    - wdl/task/outputs.go
    - wdl/task/command_test.go
    - wdl/task/task_test.go
  modified:
    - wdl/task/task.go (replaced stub with full RunTask orchestrator)
    - wdl/eval/env.go (added Parent() and ForEach() methods)

decisions:
  - "ForEach(fn) and Parent() added to eval.Env — minimal exports needed by translateEnv in task/command.go; preserves encapsulation, no map exposure"
  - "coerceOutputValue converts StringValue to FileValue/DirectoryValue for File/Directory output decls — eval.Eval returns StringValue for string interpolation even when decl type is File"
  - "BuildCommand and CollectOutputs exported (capital B/C) — test package is task_test (external), requiring exported symbols"
  - "ShellRunner ignores bind-mount semantics; finds script from Binds by matching /mnt/inputs/_command container path"

metrics:
  duration: "4 minutes"
  completed_date: "2026-03-10"
  tasks_completed: 2
  files_created: 4
  files_modified: 2
---

# Phase 5 Plan 2: Command Expansion, Output Collection, and RunTask Orchestrator Summary

**One-liner:** Full RunTask orchestrator with container-path command expansion via eval.Env.ForEach, type-coercing output collection, and ShellRunner mock for Docker-free end-to-end tests.

## Tasks Completed

| Task | Description | Commit | Files |
|------|-------------|--------|-------|
| 1 (RED) | Failing tests for BuildCommand and CollectOutputs | b797639 | command_test.go |
| 1 (GREEN) | Implement BuildCommand and CollectOutputs | e3806a0 | command.go, outputs.go, eval/env.go |
| 2 (RED) | Failing tests: ShellRunner mock and end-to-end RunTask tests | 7d7f712 | task_test.go |
| 2 (GREEN) | Implement full RunTask orchestrator | 4a3681e | task.go |

## What Was Built

### BuildCommand (command.go)

Evaluates private declarations into the task env, then builds a translated env by walking `env.ForEach()` and calling `pm.TranslateValue()` on each binding. The command expression is evaluated via `eval.Eval()` using the translated env (with container paths substituted for host paths). The resulting string is written to `execDir/command` with mode 0644. Returns the absolute path to the written script file.

### CollectOutputs (outputs.go)

Creates a child output env, then for each output declaration evaluates `decl.Expr` via `eval.Eval()` with the populated `ExecContext` (carrying host paths for stdout, stderr, and work directory). Applies `pm.UntranslateValue()` as a safety net. Applies `coerceOutputValue()` to convert `StringValue` results to `FileValue`/`DirectoryValue` when the declared output type is `File` or `Directory`. Returns a `map[string]values.Value` keyed by output name.

### RunTask (task.go)

Full pipeline orchestrator replacing the "not yet implemented" stub:
1. Creates `execDir/work/` layout under `execRoot/taskName/`
2. Creates PathMap, calls BindInputs for typed JSON → WDL conversion
3. Calls ParseRuntime for Docker image, memory, CPU
4. Calls BuildCommand (which also evaluates private decls)
5. Constructs RunRequest with all binds: input files + command script bind + work dir bind
6. Calls runner.Run() and writes stdout.txt, stderr.txt, rc to execDir
7. Returns `*TaskError{ExitCode, Stderr}` on non-zero exit
8. Calls CollectOutputs with ExecContext pointing to host paths
9. Returns `map[string]values.Value` of output name → value

### eval.Env additions (eval/env.go)

Added two minimal exports to support `translateEnv`:
- `Parent() *Env` — returns the parent scope (needed to construct a new env with the same parent chain)
- `ForEach(fn func(string, values.Value))` — iterates current scope bindings only, without exposing the underlying map

### ShellRunner mock (task_test.go)

Implements `ContainerRunner` for host-native execution. Finds `scriptPath` and `workDir` from `req.Binds` by matching container paths (`/mnt/inputs/_command` and `/mnt/outputs`). Executes via `exec.CommandContext(ctx, "bash", scriptPath)` with `cmd.Dir = workDir`. Captures stdout/stderr, propagates exit code via `exec.ExitError`.

## Test Results

All 43 task tests pass (38 pre-existing + 5 new RunTask + 6 new command/output):
- 6 BuildCommand/CollectOutputs tests
- 5 end-to-end RunTask tests (echo, inputs, failure, missing input, file output)
- All 27 pre-existing task tests still passing

Full project suite: 8/8 packages green.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing exports] BuildCommand and CollectOutputs exported**
- **Found during:** Task 1 RED phase
- **Issue:** Plan action described `buildCommand` and `collectOutputs` (lowercase). External test package `task_test` cannot access unexported functions.
- **Fix:** Exported as `BuildCommand` and `CollectOutputs`. No behavior change.
- **Files modified:** command.go, outputs.go, command_test.go

**2. [Rule 1 - Bug] coerceOutputValue added for String->File coercion**
- **Found during:** Task 1 GREEN phase (TestCollectOutputs_FileOutput failing)
- **Issue:** `eval.Eval` returns `StringValue` for string interpolation expressions even when the output decl type is `File`. WDL 1.2 allows `String`-producing expressions for `File` outputs (the string is treated as a path). CollectOutputs must coerce the result.
- **Fix:** Added `coerceOutputValue(v values.Value, declType types.Type) values.Value` in outputs.go that converts `StringValue` to `FileValue`/`DirectoryValue` based on the declared output type.
- **Files modified:** outputs.go

**3. [Rule 2 - Missing API] eval.Env.ForEach and Parent added**
- **Found during:** Task 1 implementation (translateEnv needs to iterate env bindings)
- **Issue:** `eval.Env.bindings` is unexported with no iterator method. `translateEnv` must walk all bindings in the current scope to apply `pm.TranslateValue` to each.
- **Fix:** Added `ForEach(fn func(string, values.Value))` and `Parent() *Env` to eval/env.go as minimal encapsulation-preserving exports. No behavior change to existing code.
- **Files modified:** wdl/eval/env.go

## Self-Check: PASSED
