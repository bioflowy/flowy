---
phase: quick
plan: 260320-6ed
subsystem: demo/e2e
tags: [demo, e2e, auth, projects, registry, execution, provenance]
dependency_graph:
  requires: [phase-19-auth, phase-20-projects-rbac, phase-21-wdl-registry, phase-22-registry-exec, phase-23-provenance]
  provides: [e2e-demo-script]
  affects: []
tech_stack:
  added: []
  patterns: [bash-e2e-script, curl-api-test]
key_files:
  created:
    - scripts/demo-v1.3.sh
    - scripts/demo-hello.wdl
  modified: []
decisions:
  - Used /health endpoint for server readiness check (returns 200 without auth, unlike most other endpoints)
  - Used `jq -n --arg` idiom to safely embed WDL file content in JSON body without shell escaping issues
  - Script uses PORT=8199 to avoid conflicts with default 8124 and S3 test port 8125
  - TaskRunRequest requires explicit task_name field alongside registry_ref
  - Registry pull uses ?tag= query parameter (not ?ref= as the plan suggested)
  - RegistryPushRequest uses "content" field (not "source" as the plan suggested)
  - CreateProjectRequest uses "slug" field (not "name" as the plan suggested)
metrics:
  duration: "~8 minutes (including build time)"
  completed: "2026-03-20T04:42:58Z"
  tasks_completed: 2
  files_created: 2
---

# Quick Task 260320-6ed: Create End-to-End Demo Script Summary

**One-liner:** Self-contained bash demo exercising all v1.3 features (auth, projects/RBAC, WDL registry, registry-based execution, provenance) via curl+jq against a live flowy-server and flowy-daemon.

## What Was Built

Two files created in `scripts/`:

### scripts/demo-hello.wdl
A minimal WDL 1.2 task that writes a greeting to a file and returns it as a String output. Uses `ubuntu:22.04` Docker image as runtime. This serves as the test WDL used for registry push and execution in the demo.

### scripts/demo-v1.3.sh
A self-contained bash script (272 lines) structured as follows:

1. **Build** — runs `make build` to compile all three binaries
2. **Config** — writes a temporary YAML config to `/tmp/flowy-demo-v1.3.yaml` with auth enabled, JWT secret, and daemon token
3. **Init admin** — runs `flowy-server --init-admin` to create the admin user (solves chicken-and-egg problem)
4. **Start server** — starts flowy-server in background, polls `/health` up to 15 seconds
5. **Start daemon** — starts flowy-daemon in background with 2 workers
6. **Feature 1 (Auth)** — `POST /auth/login`, extracts JWT, verifies with `GET /users`
7. **Feature 2 (Projects & RBAC)** — creates project, creates second user, adds member with executor role, retrieves project details
8. **Feature 3 (WDL Registry)** — pushes hello.wdl via `POST /projects/demo/registry`, tags with v1, lists versions, pulls by tag
9. **Feature 4 (Registry Execution)** — submits `POST /projects/demo/tasks/run` with `registry_ref: hello:v1`, polls status up to 60s, retrieves outputs
10. **Feature 5 (Provenance)** — lists provenance records, retrieves detail for the run
11. **Cleanup trap** — `trap cleanup EXIT` kills server+daemon, removes temp config and data dir
12. **Summary** — prints pass/fail table with ANSI colors

## Test Results

The script ran successfully on the first attempt with **all 20 checks passing**:

- Login succeeded, JWT token obtained
- Project created, user created, member added, project retrieved
- Registry push (kind=tool), tag v1 created, 1 version listed, pull by tag succeeded
- Task run submitted, completed in ~9 seconds with `greeting='Hello, Demo!'`
- Provenance listed (1 record), detail shows `ref='hello:v1'` and `user='demoadmin'`

## Deviations from Plan

### Minor API Shape Corrections (Rule 1 — auto-corrected during authoring)

The plan described the API shapes based on memory; the actual implementation differed in three fields:

**1. RegistryPushRequest: `content` not `source`**
- Plan said: `{"name":"hello","kind":"task","source":"<wdl content>"}`
- Actual API: `{"name":"hello","content":"<wdl content>"}` — the server also auto-classifies kind from WDL parse, so `kind` field is not accepted
- Fixed: used `content` field; removed `kind` from push payload

**2. Registry pull: `?tag=` query param not `?ref=`**
- Plan said: `GET /projects/demo/registry/hello?ref=hello:v1`
- Actual API: `GET /projects/demo/registry/hello?tag=v1` — the endpoint accepts `?tag=` or `?digest=` as separate params
- Fixed: used `?tag=v1` query parameter

**3. CreateProjectRequest: `slug` not `name`**
- Plan said: `{"name":"Demo Project","slug":"demo"}`
- Actual API: `{"slug":"demo","description":"..."}` — no `name` field, description is separate
- Fixed: used `slug` and `description` fields correctly

These were all caught by reading the actual handler code before writing the script, so no fix iterations were needed.

## Self-Check

### Created files exist:
- `scripts/demo-v1.3.sh` — FOUND
- `scripts/demo-hello.wdl` — FOUND

### Commits exist:
- `c91520f` — feat(260320-6ed): create end-to-end v1.3 demo script and WDL test file — FOUND

## Self-Check: PASSED
