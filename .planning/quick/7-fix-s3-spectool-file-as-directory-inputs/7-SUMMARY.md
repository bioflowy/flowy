---
phase: quick-7
plan: 1
subsystem: daemon, server
tags: [s3, bug-fix, path-normalization, outputs]
dependency_graph:
  requires: [quick-6]
  provides: [correct-s3-file-inputs, s3-output-path-normalization, s3-outputs-json-fallback-fix]
  affects: [spectool-s3, daemon-executeTaskS3, server-loadTaskOutputs]
tech_stack:
  added: []
  patterns: [filepath.Clean normalization, wdlDir propagation, S3 early-return guard]
key_files:
  modified:
    - internal/daemon/daemon.go
    - internal/server/dispatch.go
decisions:
  - wdlDir is set from filepath.Dir(claim.WDLPath) in executeTaskS3, matching executeTaskLocal; empty string fallback retained when WDLPath is absent
  - S3 early-return in loadTaskOutputs uses s.cfg.Storage.Type == "s3" guard so local mode backward compat is preserved
  - filepath.Clean applied to both map keys (build side) and lookup values (read side) in delocalizeOutputs/rewriteOutputPaths for symmetric normalization
metrics:
  duration: "5 min"
  completed_date: "2026-03-13"
  tasks_completed: 2
  files_modified: 2
---

# Quick Task 7: Fix S3 spectool file-as-directory inputs and path normalization

**One-liner:** Three targeted S3-mode bug fixes: wdlDir propagation for file mounts, S3 outputs.json fallback guard, and filepath.Clean normalization on localToS3 map keys and lookups.

## Tasks Completed

| # | Name | Commit | Files |
|---|------|--------|-------|
| 1 | Fix file-as-directory inputs and outputs.json fallback | d0da317 | internal/daemon/daemon.go, internal/server/dispatch.go |
| 2 | Fix path normalization in output rewriting | 414a24f | internal/daemon/daemon.go |

## What Was Fixed

### Bug 1 — File inputs mounted as directories in Docker (daemon.go)

`executeTaskS3` called `RunTask` with `wdlDir=""`. When `claim.Inputs` contained relative local paths (not `s3://` URIs), `BindInputs` resolved them against daemon CWD instead of the WDL file's directory. The resolved path did not exist; Docker created a directory there, causing "Is a directory" errors inside containers.

**Fix:** Added `wdlDir := filepath.Dir(claim.WDLPath)` before the `RunTask` call in `executeTaskS3`, passing it as the `wdlDir` argument — identical to `executeTaskLocal`'s behavior.

### Bug 2 — outputs.json fallback always fails in S3 mode (dispatch.go)

`loadTaskOutputs` falls back to `s.backend.DownloadFile(outputsPath)` when `OutputsJSON` is empty. In S3 mode the daemon never writes task-level `outputs.json` to S3 — it reports outputs via the result API. So the file download always fails with "file does not exist" for tasks that have no output declarations or `return_codes` specs.

**Fix:** Added an S3 early-return guard immediately after the `if t.OutputsJSON != ""` block. When `Storage.Type == "s3"`, return an empty `map[string]values.Value{}` without attempting the file fallback. Local-mode file fallback is unchanged.

### Bug 3 — Local staging paths leak into S3 output values (daemon.go)

`delocalizeOutputs` builds a `localToS3` map from `filepath.WalkDir` keys, then `rewriteOutputPaths` looks up output value strings in that map by exact match. Path normalization differences (extra `/./`, `//`, trailing slash, `os.MkdirTemp` prefixes) caused lookup misses, so local staging paths leaked through into the output values sent to the server.

**Fix:** Applied `filepath.Clean` to map keys when building `localToS3` in `delocalizeOutputs`, and applied `filepath.Clean` to the string value before map lookup in `rewriteOutputPaths`. Both sides now use consistent normalization.

## Verification

- `go build ./...` — passes
- `go vet ./...` — passes
- `go test ./internal/daemon/... ./internal/server/... -count=1` — both packages pass

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check: PASSED

Files exist:
- FOUND: internal/daemon/daemon.go
- FOUND: internal/server/dispatch.go

Commits exist:
- d0da317: fix(quick-7): fix file-as-directory inputs and outputs.json S3 fallback
- 414a24f: fix(quick-7): normalize paths in delocalizeOutputs and rewriteOutputPaths
