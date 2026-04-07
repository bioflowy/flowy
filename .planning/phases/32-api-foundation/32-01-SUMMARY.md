---
phase: 32-api-foundation
plan: "01"
subsystem: api
tags: [registry, inputs, introspection, wdl, rest-api]
dependency_graph:
  requires:
    - wdl/parser (Parse function)
    - wdl/ast (Document, Decl, Expr types)
    - internal/store (GetRegistryVersionByTag)
    - internal/server (chi router, projectRoleRequired middleware)
  provides:
    - GET /projects/{slug}/registry/{name}/inputs?tag=<tag>
    - api.RegistryInputItem DTO
    - api.RegistryInputsResponse DTO
  affects:
    - Phase 36 (UI Workflow Execution) — consumes this endpoint to generate dynamic input forms
tech_stack:
  added: []
  patterns:
    - Handler follows existing handleRegistryPull pattern (projectFromContext + chi.URLParam + writeJSON)
    - TDD: RED commit then GREEN commit
key_files:
  created:
    - internal/server/handlers_registry_inputs_test.go
  modified:
    - internal/api/types.go
    - internal/server/handlers_registry.go
    - internal/server/server.go
decisions:
  - Use TypeName() directly from wdl/types.Type — produces WDL-canonical strings (String, Int, Boolean?, Array[File])
  - declDefaultString returns nil for complex/interpolated expressions — only plain literals are serialisable
  - extractInputs prefers Workflows[0].Inputs over Tasks[0].Inputs — mirrors push classification logic
metrics:
  duration: "~7 minutes"
  completed: "2026-04-07"
  tasks_completed: 1
  files_modified: 4
  commits: 2
---

# Phase 32 Plan 01: Registry Input Schema Endpoint Summary

**One-liner:** WDL input schema introspection endpoint returning name/type/optional/default per declared input, gated by viewer role JWT.

## Objective

Added `GET /projects/{slug}/registry/{name}/inputs?tag=<tag>` to flowy-server. The endpoint parses the stored WDL content for a registry entry and returns the declared inputs as structured JSON, enabling Phase 36 UI to generate dynamic input forms.

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 (RED) | Add failing tests for registry inputs endpoint | 9058e1c | internal/server/handlers_registry_inputs_test.go |
| 1 (GREEN) | Implement DTO types, handler, route registration | 4d36cd3 | internal/api/types.go, internal/server/handlers_registry.go, internal/server/server.go |

## What Was Built

### DTOs (internal/api/types.go)

- `RegistryInputItem` — `{name, type, optional, default}` per WDL declared input
- `RegistryInputsResponse` — `{inputs: []}` wrapper; never null

### Handler (internal/server/handlers_registry.go)

- `handleRegistryInputs` — validates tag param, fetches version by tag, parses WDL, returns input list
- `extractInputs` — walks first workflow or task inputs, converts `ast.Decl` to `RegistryInputItem`
- `declDefaultString` — type-switches over literal AST nodes (IntLit, FloatLit, BoolLit, NullLit, StringExpr); returns nil for interpolated or complex expressions

### Route (internal/server/server.go)

```
r.With(s.projectRoleRequired("viewer")).Get("/inputs", s.handleRegistryInputs)
```

Registered inside `r.Route("/{name}", ...)` alongside existing pull/versions/tags routes.

## Test Coverage

Six tests in `handlers_registry_inputs_test.go`:

| Test | Scenario | Expected |
|------|----------|----------|
| TestRegistryInputs | Workflow with 3 typed inputs | 200, correct name/type/optional/default per item |
| TestRegistryInputs_WithDefaults | Task with string/int defaults | default field is pointer-to-string "hello" / "42" |
| TestRegistryInputs_Empty | Workflow with empty input block | 200, inputs:[] (not null) |
| TestRegistryInputs_TagNotFound | Nonexistent tag query | 404 with error message |
| TestRegistryInputs_MissingTagParam | No ?tag= in URL | 400 with error message |
| TestRegistryInputs_Auth | No token / viewer token | 401 / 200 |

## Verification

```
go test ./internal/server/ -run "TestRegistryInputs" -v -count=1  # PASS (6/6)
go test ./internal/server/... -count=1                             # PASS (no regressions)
```

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check

- [x] internal/api/types.go contains `type RegistryInputItem struct`
- [x] internal/api/types.go contains `type RegistryInputsResponse struct`
- [x] internal/server/handlers_registry.go contains `func (s *Server) handleRegistryInputs(`
- [x] internal/server/handlers_registry.go contains `func extractInputs(`
- [x] internal/server/handlers_registry.go contains `func declDefaultString(`
- [x] internal/server/server.go contains `Get("/inputs", s.handleRegistryInputs)`
- [x] internal/server/handlers_registry_inputs_test.go contains `func TestRegistryInputs(`
- [x] internal/server/handlers_registry_inputs_test.go contains `func TestRegistryInputs_Empty(`
- [x] internal/server/handlers_registry_inputs_test.go contains `func TestRegistryInputs_TagNotFound(`
- [x] All 6 TestRegistryInputs* tests PASS
- [x] Full server suite passes with no regressions
