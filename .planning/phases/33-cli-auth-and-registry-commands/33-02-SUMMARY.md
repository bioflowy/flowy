---
phase: 33-cli-auth-and-registry-commands
plan: "02"
subsystem: cli
tags: [cli, registry, client, http, tdd]
dependency_graph:
  requires: ["33-01"]
  provides: ["flowy registry subcommands", "registry client methods"]
  affects: ["cmd/flowy", "internal/client"]
tech_stack:
  added: []
  patterns: ["httptest mock servers", "tabwriter for aligned output", "flag.FlagSet per subcommand", "sentinel error mapping"]
key_files:
  created:
    - internal/client/registry_test.go
    - cmd/flowy/registry.go
    - cmd/flowy/registry_test.go
  modified:
    - internal/client/client.go
    - cmd/flowy/main.go
decisions:
  - "pull prints raw content via fmt.Fprint (no added newline) for pipeability"
  - "loadCredsAndClient helper centralises credential loading and client construction for all registry subcommands"
  - "handleClientError maps ErrUnauthorized/ErrForbidden sentinels to human-readable messages without token leakage"
  - "reorderArgs extended with --name, --tag, --digest so flags after positional args work correctly"
metrics:
  duration_minutes: 12
  completed_date: "2026-04-07"
  tasks_completed: 2
  files_changed: 5
---

# Phase 33 Plan 02: Registry CLI Subcommands Summary

**One-liner:** `flowy registry` with 6 subcommands (push/list/pull/tag/delete-tag/delete) backed by 6 Bearer-auth client methods and human-readable error mapping for missing/expired credentials.

## Tasks Completed

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | Registry client methods | a4a4b8a | internal/client/client.go, internal/client/registry_test.go |
| 2 | Registry subcommands and main.go wiring | b4839b0 | cmd/flowy/registry.go, cmd/flowy/registry_test.go, cmd/flowy/main.go |

## What Was Built

### Task 1 — Registry client methods (TDD)

Added 6 methods to `internal/client/client.go`:

- `RegistryPush(ctx, projectSlug, name, content)` — POST /projects/{slug}/registry
- `RegistryList(ctx, projectSlug)` — GET /projects/{slug}/registry
- `RegistryPull(ctx, projectSlug, name, tag, digest)` — GET /projects/{slug}/registry/{name}?tag=X or ?digest=X
- `RegistryTag(ctx, projectSlug, name, tag, digest)` — POST /projects/{slug}/registry/{name}/tags
- `RegistryDeleteTag(ctx, projectSlug, name, tag)` — DELETE /projects/{slug}/registry/{name}/tags/{tag}
- `RegistryDelete(ctx, projectSlug, name)` — DELETE /projects/{slug}/registry/{name}

All methods use `doRequest` for uniform Bearer token injection. 401 returns `ErrUnauthorized`, 403 returns `ErrForbidden`.

13 httptest-based tests in `internal/client/registry_test.go` covering success paths, 401/403 error mapping, and digest vs. tag query parameter selection.

### Task 2 — Registry subcommands (TDD)

Created `cmd/flowy/registry.go` with:

- `runRegistry` — dispatcher printing usage on no args, dispatching on subcommand name
- `runRegistryPush` — reads WDL file, calls RegistryPush, prints `Pushed {name} digest:{hash}`
- `runRegistryList` — calls RegistryList, renders table via `text/tabwriter` with NAME/KIND/LATEST TAG/VERSIONS columns
- `runRegistryPull` — calls RegistryPull, prints raw content via `fmt.Fprint` (no trailing newline)
- `runRegistryTag` — calls RegistryTag, prints `Tagged {name} {tag}`
- `runRegistryDeleteTag` — calls RegistryDeleteTag, prints `Deleted tag {tag} from {name}`
- `runRegistryDelete` — calls RegistryDelete, prints `Deleted {name}`
- `loadCredsAndClient` — shared helper loading `~/.flowy/credentials.yaml`
- `handleClientError` — maps ErrUnauthorized/ErrForbidden to human-readable messages without token data

Updated `cmd/flowy/main.go`:
- Added `case "registry": return runRegistry(args[1:], os.Stdout, os.Stderr)` to the command switch
- Added `registry` to usage text
- Added `--name`, `--tag`, `--digest` to `reorderArgs` knownValueFlags map

13 tests in `cmd/flowy/registry_test.go` covering all subcommands, error paths, missing flags, and credential errors using httptest mock servers and `t.Setenv("HOME", tmpDir)`.

## Deviations from Plan

None — plan executed exactly as written.

## Verification Results

- `go test ./internal/client/... -run TestRegistry` — 13/13 PASS
- `go test ./cmd/flowy/... -run TestRegistry` — 13/13 PASS
- `go test ./... -count=1 -timeout 120s` — all packages PASS
- `go vet ./...` — no issues

## Self-Check

- [x] `internal/client/client.go` contains all 6 registry methods
- [x] `cmd/flowy/registry.go` exists with all 6 subcommand functions
- [x] `cmd/flowy/registry_test.go` exists with 13 tests
- [x] `internal/client/registry_test.go` exists with 13 tests
- [x] `cmd/flowy/main.go` contains `case "registry"`
- [x] Commit a4a4b8a (Task 1) exists
- [x] Commit b4839b0 (Task 2) exists
