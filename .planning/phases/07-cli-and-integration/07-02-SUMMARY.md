---
phase: 07-cli-and-integration
plan: "02"
subsystem: cli
tags: [cli, run-command, workflow-execution, task-execution, verbose, tdd]
dependency_graph:
  requires: [07-01]
  provides: [flowy-run-command]
  affects: [cmd/flowy]
tech_stack:
  added: [crypto/rand, encoding/hex, encoding/json]
  patterns: [testable-main, runner-injection, post-hoc-verbose]
key_files:
  created:
    - cmd/flowy/run_test.go
    - cmd/flowy/verbose_test.go
    - cmd/flowy/testdata/valid_workflow.wdl
    - cmd/flowy/testdata/inputs.json
    - cmd/flowy/testdata/task_inputs.json
  modified:
    - cmd/flowy/main.go
decisions:
  - newRunner package-level factory var enables test injection without subprocess
  - setupExecDir validates non-empty dirs to prevent accidental data clobbering
  - printVerboseOutput post-hoc approach reads stdout.txt/stderr.txt after execution (zero API changes to inner packages)
  - verboseMockRunner returns canned RunResult.Stdout/Stderr — RunTask writes these to disk files, verbose reads them back
  - WDL fixture omits task input defaults (parser does not support them) and workflow output references (type checker limitation)
metrics:
  duration: 4min
  completed_date: "2026-03-11"
  tasks_completed: 2
  files_changed: 6
---

# Phase 7 Plan 2: flowy run Subcommand Summary

One-liner: Complete `flowy run` CLI with workflow/task execution, exec-dir management, input loading, and post-hoc verbose output via mock-injectable ContainerRunner factory.

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | Implement runRun subcommand with exec dir and workflow execution | 71c0675 | cmd/flowy/main.go, cmd/flowy/run_test.go, testdata/* |
| 2 | Implement --verbose flag for task stdout/stderr output | e85a810 | cmd/flowy/verbose_test.go |

## What Was Built

### runRun subcommand (cmd/flowy/main.go)

- `runRun(args []string, stdout, stderr io.Writer) int` dispatches `flowy run`
- Flags: `--input` (JSON file), `--task` (named task), `--dir` (exec dir), `--path` (import search), `--verbose` (print task output)
- Parses and type-checks WDL file before execution
- `setupExecDir`: validates `--dir` is empty or non-existent; generates unique `flowy-YYYYMMDD-<hex>` temp dir otherwise
- Prints `Execution directory: <path>` to stderr before any execution
- On success: prints `Outputs written to <execRoot>/outputs.json` to stdout; exit 0
- On failure: prints error + `See <execRoot>/error.json for details` to stderr; exit 1
- Usage errors (missing WDL arg): exit 2

### --verbose output (printVerboseOutput)

- Post-hoc: walks exec dir after all tasks finish
- For each task subdirectory: prints `--- <taskDir> stdout ---` + content, then `--- <taskDir> stderr ---` + content
- Zero API changes to `wdl/task` or `wdl/workflow` packages

### newRunner injection pattern

- `var newRunner func() task.ContainerRunner = func() task.ContainerRunner { return task.NewDockerRunner() }`
- Tests override: `newRunner = func() task.ContainerRunner { return mockRunner }`
- Deferred restore ensures test isolation

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] WDL fixture had task input default value**
- **Found during:** Task 1 (RED phase)
- **Issue:** `String name = "world"` in task input block is not supported by the parser (`expected type, got "="`)
- **Fix:** Removed default from task input; kept default only in workflow input (which the parser supports)
- **Files modified:** cmd/flowy/testdata/valid_workflow.wdl

**2. [Rule 1 - Bug] WDL fixture referenced call output in workflow output section**
- **Found during:** Task 1 (RED phase)
- **Issue:** `String result = say_hello.greeting` in workflow output caused type checker error (`undefined name "say_hello"`)
- **Fix:** Removed workflow output section from fixture; workflow without outputs is valid and sufficient for CLI tests
- **Files modified:** cmd/flowy/testdata/valid_workflow.wdl

**3. [Rule 1 - Bug] --task test needed input JSON for required task input**
- **Found during:** Task 1 (GREEN phase)
- **Issue:** `say_hello` task has required `String name` input; running without `--input` caused `missing required input: "name"`
- **Fix:** Added `task_inputs.json` fixture and updated `TestRunTask` to pass `--input task_inputs.json`
- **Files modified:** cmd/flowy/run_test.go, cmd/flowy/testdata/task_inputs.json

**4. [Rule 1 - Bug] verboseMockRunner wrote files before RunTask overwrote them**
- **Found during:** Task 2 (GREEN phase)
- **Issue:** Initial mock wrote `stderr.txt` to execDir via bind-mount parsing, but RunTask subsequently overwrote it with empty `RunResult.Stderr`
- **Fix:** Removed manual file writes from mock; returned stderr content in `RunResult.Stderr` so RunTask writes it correctly
- **Files modified:** cmd/flowy/verbose_test.go

## Self-Check: PASSED

All created files exist. Both task commits (71c0675, e85a810) are present in git log.
Full test suite passes: `go test ./... -count=1` — all 11 packages OK.
Build passes: `go build ./cmd/flowy/`. Vet passes: `go vet ./...`.
