---
phase: 05-task-execution
plan: 03
subsystem: infra
tags: [docker, container, go, integration-test]

# Dependency graph
requires:
  - phase: 05-02
    provides: ContainerRunner interface, RunTask orchestrator

provides:
  - DockerRunner implementing ContainerRunner via Docker Engine API
  - Full container lifecycle management (pull → create → wait-setup → start → wait → logs → remove)
  - Integration test with live Docker daemon gated by build tag

affects:
  - 06-workflow-executor (uses DockerRunner as the real ContainerRunner)

# Tech tracking
tech-stack:
  added:
    - github.com/docker/docker v28.5.2+incompatible (Docker Engine API client)
  patterns:
    - Stateless runner: DockerRunner holds no fields, creates Docker client per Run call for thread-safety
    - Wait-before-start: ContainerWait set up before ContainerStart to prevent race condition on fast-exiting containers
    - Drain-on-pull: ImagePull response body drained with io.Copy(io.Discard, pullResp) to prevent connection pool exhaustion
    - Deferred cleanup: force-remove container on all error paths after ContainerCreate succeeds
    - Integration build tag: //go:build integration gates live-daemon tests from normal go test ./...

key-files:
  created:
    - wdl/task/docker.go
    - wdl/task/docker_test.go
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "DockerRunner is stateless (no fields) — creates Docker client per Run call for thread-safety and simplicity"
  - "ContainerWait called before ContainerStart to avoid race condition where fast-exiting container finishes before wait is registered"
  - "AutoRemove: false — container kept until after log collection, then manually removed"
  - "Integration test gated with //go:build integration so normal go test ./... does not require live Docker daemon"

patterns-established:
  - "Integration test pattern: //go:build integration build tag with go test -tags integration to opt in"

requirements-completed: [TASK-05]

# Metrics
duration: ~10min
completed: 2026-03-10
---

# Phase 5 Plan 03: DockerRunner Summary

**DockerRunner implementing ContainerRunner via Docker Engine API with wait-before-start race-free lifecycle and integration test verified against live daemon**

## Performance

- **Duration:** ~10 min
- **Started:** 2026-03-10T13:09:41Z
- **Completed:** 2026-03-10T13:12:24Z
- **Tasks:** 2 (1 auto + 1 human-verify checkpoint)
- **Files modified:** 4

## Accomplishments

- Implemented DockerRunner with full container lifecycle: image pull, create, wait (pre-start), start, wait for exit, log capture, and cleanup
- Used stdcopy.StdCopy to correctly demultiplex Docker's multiplexed stdout/stderr stream
- Added integration test with build tag that verifies echo output capture and non-zero exit code handling
- Integration test verified passing against live Docker daemon (user-approved checkpoint)

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Docker dependency, implement DockerRunner, and write integration test** - `44bba61` (feat)
2. **Task 2: Verify DockerRunner integration test with live Docker daemon** - checkpoint:human-verify (approved, no additional commit)

## Files Created/Modified

- `wdl/task/docker.go` - DockerRunner struct with Run() implementing ContainerRunner; full lifecycle with error cleanup
- `wdl/task/docker_test.go` - Integration test gated by //go:build integration; tests echo output and non-zero exit code
- `go.mod` - Added github.com/docker/docker v28.5.2+incompatible dependency
- `go.sum` - Updated checksums for Docker client and transitive dependencies

## Decisions Made

- DockerRunner is stateless (no struct fields) — creates a new Docker client per Run() call for simplicity and thread-safety. Negligible overhead since container execution dominates latency.
- ContainerWait called before ContainerStart to prevent race condition: a fast-exiting container could finish between Start and Wait, causing the wait channel to miss the exit event.
- AutoRemove disabled on HostConfig — keeping the container available after exit allows ContainerLogs to collect output before the container is manually removed.
- Integration build tag pattern chosen over t.Skip()+docker-available-check so that normal CI (without Docker) never incurs Docker ping overhead.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - implementation followed the RESEARCH.md Pattern 3 lifecycle without issues.

## User Setup Required

None - no external service configuration required. Docker daemon must be available at runtime for actual task execution (not for unit tests).

## Next Phase Readiness

- DockerRunner is complete and ready to be injected into RunTask orchestrator (from Plan 05-02)
- Phase 05 task execution subsystem is now fully implemented: type system, expression evaluator, task orchestrator, and real Docker runner
- Phase 06 (workflow executor) can proceed: it needs RunTask and DockerRunner to execute WDL workflow call statements

---
*Phase: 05-task-execution*
*Completed: 2026-03-10*
