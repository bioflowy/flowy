---
phase: 08-add-spectool-integration-for-wdl-specification-compliance-testing
plan: "01"
subsystem: cli-output
tags: [spectool, json-output, stdout, cli, tdd]
dependency_graph:
  requires: []
  provides: [spectool-compatible-stdout-json, absolute-path-outputs, output-flag]
  affects: [wdl/workflow/output.go, cmd/flowy/main.go]
tech_stack:
  added: []
  patterns: [SerializeOutputs-exported-function, stdout-json-output, output-flag]
key_files:
  created: []
  modified:
    - wdl/workflow/output.go
    - wdl/workflow/output_test.go
    - wdl/workflow/workflow.go
    - cmd/flowy/main.go
    - cmd/flowy/run_test.go
decisions:
  - "SerializeOutputs exported so CLI (cmd/flowy) can call it directly for stdout output"
  - "valueToJSONOutput drops execRoot parameter — File/Directory values already absolute from task execution"
  - "--output flag writes JSON to file instead of stdout (stdout empty when --output used)"
  - "Task runs now write outputs.json to execRoot (previously not written)"
metrics:
  duration: 3min
  completed: "2026-03-11"
  tasks_completed: 2
  files_modified: 5
---

# Phase 8 Plan 01: CLI Output JSON Compatibility Summary

**One-liner:** Emit spectool-compatible JSON to stdout on run success, absolute File/Directory paths, --output flag, task outputs serialization with dot-notation keys.

## What Was Built

Modified flowy's CLI output behavior to be compatible with spectool conformance testing:

1. **output.go refactoring:** Removed `execRoot` parameter from `valueToJSONOutput` and `writeOutputsJSON`. File/Directory values now return absolute paths directly (spectool expects absolute paths). Exported `SerializeOutputs(entityName, outputs)` function for use by CLI.

2. **CLI stdout JSON output:** `flowy run` now emits valid JSON to stdout on success (workflow and task runs). The old "Outputs written to .../outputs.json" text was removed from stdout. stderr still prints "Execution directory:".

3. **--output flag:** New `--output <path>` flag writes JSON to a file instead of stdout. When used, stdout is empty.

4. **Task output serialization:** `--task` runs now serialize outputs with `taskName.outputName` dot-notation keys and write `outputs.json` to `execRoot` (previously not written for task runs).

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Refactor output.go — absolute paths + SerializeOutputs export | 0f4adfd | wdl/workflow/output.go, output_test.go, workflow.go |
| 2 | Update runRun — stdout JSON, --output flag, task output serialization | 1eb848a | cmd/flowy/main.go, run_test.go |

## Deviations from Plan

None - plan executed exactly as written.

## Verification Results

```
go test ./wdl/workflow/ ./cmd/flowy/ -count=1  -- all tests pass
go test ./... -count=1  -- full test suite passes (no regressions)
go build ./cmd/flowy    -- binary builds successfully
```

## Self-Check: PASSED

- wdl/workflow/output.go: FOUND
- cmd/flowy/main.go: FOUND
- commit 0f4adfd: FOUND
- commit 1eb848a: FOUND
