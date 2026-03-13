---
phase: quick-8
plan: 1
subsystem: server/dispatch, wdl/workflow, wdl/task
tags: [s3, spectool, file-type, map-ordering, determinism]
dependency_graph:
  requires: []
  provides: [s3-uri-filetype-detection, deterministic-mapvalue-entries]
  affects: [internal/server/dispatch.go, wdl/workflow/convert.go, wdl/task/inputs.go]
tech_stack:
  added: []
  patterns: [sorted-map-iteration]
key_files:
  created: []
  modified:
    - internal/server/dispatch.go
    - wdl/workflow/convert.go
    - wdl/task/inputs.go
decisions:
  - "s3:// prefix detection happens only in jsonToValue (dispatch.go untyped fallback); typed paths in convert.go/inputs.go already handle File/Directory via decl type"
  - "sort.Strings on extracted keys slice — no third-party dependency needed"
metrics:
  duration: "~4 min"
  completed: "2026-03-13T08:14:59Z"
  tasks_completed: 2
  files_modified: 3
---

# Quick Task 8: Fix S3 URI Leak in File Outputs and Deterministic Map Key Ordering

**One-liner:** Detect s3:// strings as FileValue in dispatch.go untyped parser and sort map keys at all five MapValue construction sites to eliminate non-deterministic entry ordering.

## Objective

Fix two S3 spectool failures:
1. **primitive_literals** — `jsonToValue` in dispatch.go treated all strings as `StringValue`, causing s3:// paths from task outputs to lose their File type. Workflow evaluator and spectool then received `StringValue` instead of `FileValue`.
2. **serde_homogeneous_pair** — Go map iteration order is non-deterministic; `MapValue.Entries` had random key ordering across all five construction sites, causing spectool comparison failures.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Fix S3 URI leak and sorted map keys in dispatch.go | 8f65459 | internal/server/dispatch.go |
| 2 | Add deterministic map key ordering in convert.go and inputs.go | 21b8ade | wdl/workflow/convert.go, wdl/task/inputs.go |

## Changes Made

### Task 1: dispatch.go

**S3 URI detection (jsonToValue `case string:`):**
```go
case string:
    if strings.HasPrefix(val, "s3://") {
        return values.FileValue{Type: types.FileType{}, Value: val}, nil
    }
    return values.StringValue{Type: types.StringType{}, Value: val}, nil
```

**Sorted map keys in jsonToValue `case map[string]interface{}:`** — collect keys, `sort.Strings`, iterate sorted slice.

**Sorted map keys in rawToValues** — same pattern applied to the raw outputs map.

### Task 2: convert.go and inputs.go

Four map iteration sites updated to use sorted key iteration:
- `jsonToValueTyped` `case types.MapType:` in convert.go
- `jsonToValueUntyped` `case map[string]interface{}:` in convert.go
- `jsonToWDLValue` `case types.MapType:` in inputs.go
- `jsonToValueUntyped` `case map[string]interface{}:` in inputs.go

All sites: extract keys into slice, `sort.Strings`, iterate sorted slice.

## Verification

```
go build ./... && go test ./internal/server/ ./wdl/workflow/ ./wdl/task/ ./wdl/values/ -count=1 -timeout=120s
```

Result: All packages compile. All tests pass (4/4 packages: PASS).

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check: PASSED

- Commits exist: 8f65459, 21b8ade (verified via git log)
- Files modified: internal/server/dispatch.go, wdl/workflow/convert.go, wdl/task/inputs.go
- All 5 map iteration sites use sorted keys
- All packages compile and tests pass
