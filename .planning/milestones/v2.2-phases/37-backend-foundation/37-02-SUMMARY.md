---
phase: 37-backend-foundation
plan: "02"
subsystem: storage
tags: [storage, backend, s3, stat, headobject, interface]
dependency_graph:
  requires: []
  provides: [Backend.Stat, LocalBackend.Stat, S3Backend.Stat, S3APIClient.HeadObject]
  affects: [internal/storage/backend.go, internal/storage/local.go, internal/storage/s3.go]
tech_stack:
  added: []
  patterns: [interface extension, TDD red-green, nil-safe pointer deref]
key_files:
  created: []
  modified:
    - internal/storage/backend.go
    - internal/storage/local.go
    - internal/storage/s3.go
    - internal/storage/s3_test.go
    - internal/storage/local_test.go
decisions:
  - "Stat has no context.Context parameter -- consistent with all other Backend methods (D-12)"
  - "S3Backend.Stat nil-checks out.ContentLength before deref (AWS SDK v2 uses *int64)"
  - "mockS3Client HeadObject stub returns (nil, nil) when headObjectFn is unset -- safe default for existing tests"
metrics:
  duration: "~5 minutes"
  completed: "2026-04-09"
  tasks_completed: 1
  files_modified: 5
---

# Phase 37 Plan 02: Backend.Stat and S3APIClient.HeadObject Summary

## One-liner

Added `Stat(path string) (int64, error)` to the `Backend` interface with `os.Stat`-based `LocalBackend` implementation and `HeadObject`-based `S3Backend` implementation, extending `S3APIClient` with `HeadObject`.

## What Was Built

Extended the storage abstraction to support file size retrieval:

- **Backend interface** (`internal/storage/backend.go`): Added `Stat(path string) (int64, error)` after the streaming I/O methods. No `context.Context` per project decision D-12.
- **LocalBackend.Stat** (`internal/storage/local.go`): Thin wrapper around `os.Stat`, returns `fi.Size()` on success or a wrapped error on failure.
- **S3APIClient interface** (`internal/storage/s3.go`): Added `HeadObject` method matching the AWS SDK v2 `*s3.Client` signature.
- **S3Backend.Stat** (`internal/storage/s3.go`): Calls `b.s3c.HeadObject` with `context.Background()`, nil-checks `out.ContentLength` before dereferencing to avoid panics on AWS SDK v2 `*int64` fields.
- **mockS3Client** (`internal/storage/s3_test.go`): Added `headObjectFn` field and `HeadObject` method stub (returns `nil, nil` when fn unset — safe for existing tests).
- **Tests** (`internal/storage/local_test.go`, `internal/storage/s3_test.go`): Added `TestLocalBackend_Stat` (success + not_found subtests) and `TestS3Backend_Stat` (success, nil_content_length, error subtests).

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 37-02-01 | Backend.Stat + S3APIClient.HeadObject interface additions and implementations | bd7b302 | backend.go, local.go, s3.go, s3_test.go, local_test.go |

## Verification Results

```
go build ./internal/storage/...   -- PASS (no errors)
go test ./internal/storage/... -count=1  -- PASS (all 48 tests passing)
```

New tests:
- `TestLocalBackend_Stat/success` -- PASS
- `TestLocalBackend_Stat/not_found` -- PASS
- `TestS3Backend_Stat/success` -- PASS
- `TestS3Backend_Stat/nil_content_length` -- PASS
- `TestS3Backend_Stat/error` -- PASS

All existing tests unchanged and passing.

## Deviations from Plan

None -- plan executed exactly as written.

## Known Stubs

None.

## Threat Flags

None -- no new HTTP endpoints, auth paths, file access patterns, or schema changes introduced. `LocalBackend.Stat` is a thin `os.Stat` wrapper; path validation is deferred to the HTTP layer in Phase 38 (per T-37-04 and T-37-05 in the threat model).

## Self-Check: PASSED

Files exist:
- internal/storage/backend.go -- FOUND (contains `Stat(path string) (int64, error)`)
- internal/storage/local.go -- FOUND (contains `func (l *LocalBackend) Stat`)
- internal/storage/s3.go -- FOUND (contains `HeadObject` in interface and `func (b *S3Backend) Stat`)
- internal/storage/s3_test.go -- FOUND (contains `headObjectFn`, `HeadObject` method, `TestS3Backend_Stat`)
- internal/storage/local_test.go -- FOUND (contains `TestLocalBackend_Stat`)

Commits exist:
- bd7b302: feat(37-02): add Backend.Stat, LocalBackend.Stat, S3Backend.Stat, HeadObject -- FOUND
