# Phase 39: Workflow Completion Hook and Type Annotation - Context

**Gathered:** 2026-04-19
**Status:** Ready for planning

<domain>
## Phase Boundary

When a workflow run completes successfully, register all File and Directory outputs into the project-managed file library inside the same completion transaction that marks the run `COMPLETED`. Also populate `managed_files.file_type` from workflow output `parameter_meta` when the output metadata uses the `type` annotation shape defined for this milestone.

This phase does not add new import endpoints (Phase 38 already did that) and does not add the browser file picker UI (Phase 40). It only closes the execution-to-library gap for completed runs and carries semantic file type metadata into managed-file records.

</domain>

<decisions>
## Implementation Decisions

### Annotation Contract
- **D-01:** Phase 39 honors only workflow-output `parameter_meta` entries in the exact shape `parameter_meta { output_name: { type: "fastq" } }` for `managed_files.file_type`.
- **D-02:** There is no fallback to task-level metadata, inferred MIME type, or path heuristics when workflow output metadata is missing.
- **D-03:** Malformed, non-object, or non-string `parameter_meta` values are ignored rather than causing a run failure. Backward compatibility with existing non-type `parameter_meta` usage is mandatory.

### Output Registration Scope
- **D-04:** Auto-registration applies to completed workflow outputs whose declared WDL types are `File`, `Directory`, `Array[File]`, or `Array[Directory]`.
- **D-05:** Arrays are flattened into one managed-file record per concrete file or directory path.
- **D-06:** Non-file outputs are serialized to `outputs_json` as today but are not inserted into the managed file library.

### Duplicate Path Policy
- **D-07:** If multiple outputs within the same completed run resolve to the same underlying path, Phase 39 should deduplicate them into a single managed-file record for that run/path.
- **D-08:** Deduplication is scoped to one run completion event only. Existing imported files or files from prior runs are not retroactively merged or rewritten by this phase.

### Completion Transaction Semantics
- **D-09:** Run completion remains strict and atomic. If any managed-file registration step for a completed run fails, the run must not transition to `COMPLETED`.
- **D-10:** The run completion path should move from separate `UpdateRunOutputs` + `UpdateRunState` calls to `CompleteRun(...)` so output JSON persistence and managed-file inserts share the same SQLite transaction boundary.

### Managed File Metadata
- **D-11:** Auto-registered records keep the existing Phase 37/38 defaults: `label` derives from `filepath.Base(path)`, `run_id` is set to the completed run ID, and `file_type` defaults to `""` unless workflow output `parameter_meta.type` supplies a value.
- **D-12:** Directory outputs are first-class managed library entries in this phase and should be registered using the same metadata model as files.

### the agent's Discretion
- How to structure helper functions that walk workflow outputs and extract managed-file candidates before calling `CompleteRun(...)`
- Exact internal representation for dedupe tracking before the final `[]store.ManagedFile` slice is passed to the store layer
- Whether to cover additional edge-case output shapes beyond the roadmap minimum, as long as the required File/Directory behavior and spectool compatibility remain intact

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone Requirements
- `.planning/ROADMAP.md` — Phase 39 goal, success criteria, and milestone dependency boundaries
- `.planning/REQUIREMENTS.md` — `AUTOREG-01` and `ANNOT-01`, plus milestone-wide managed-file constraints
- `.planning/PROJECT.md` — current milestone goal, active requirements, and key compatibility constraints for managed files

### Prior Phase Decisions
- `.planning/phases/37-backend-foundation/37-CONTEXT.md` — establishes `CompleteRun(...)` as the transaction boundary and the `managed_files` schema contract
- `.planning/phases/38-file-library-rest-api/38-CONTEXT.md` — Phase 38 defaults for label derivation, empty `file_type`, and managed-file response semantics
- `.planning/phases/38-file-library-rest-api/38-SECURITY.md` — accepted security boundaries and threat posture for managed file registration behavior
- `.planning/phases/38-file-library-rest-api/38-VALIDATION.md` — current automated coverage expectations for managed-file library behavior

### Completion and Output Plumbing
- `internal/server/dispatch.go` — current workflow/task completion path still uses separate `UpdateRunOutputs` and `UpdateRunState` calls that Phase 39 must replace for workflow runs
- `internal/store/store.go` — `ManagedFile` model and `CompleteRun(ctx, runID, projectID, outputsJSON, files)` contract
- `internal/store/sqlite/managed_files.go` — authoritative SQLite implementation of `CompleteRun(...)`
- `internal/store/sqlite/managed_files_test.go` — atomicity and file-insert expectations already covered at the store layer

### WDL Metadata and Output Semantics
- `wdl/ast/ast.go` — workflow/task AST includes `ParamMeta`
- `wdl/workflow/workflow.go` — workflow output declarations are evaluated here and provide the output names Phase 39 must align with
- `wdl/task/task.go` — task-level `parameter_meta` handling exists separately; useful for confirming what Phase 39 should explicitly not inherit
- `wdl/workflow/output.go` — output JSON serialization contract that must stay compatible

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/store/store.go:CompleteRun(...)` — already designed for the exact atomic completion path this phase needs
- `internal/store/sqlite/managed_files.go` — inserts managed-file rows during completion and already has atomicity tests
- `values.ValueToInterface(...)` usage in `internal/server/dispatch.go` — existing output serialization path should remain the source of `outputs_json`

### Established Patterns
- Workflow runs currently marshal outputs in `internal/server/dispatch.go`, write `outputs.json`, then update run state separately. Phase 39 must preserve the serialized output contract while moving completion into `CompleteRun(...)`.
- File identity is path-based in the existing managed file library. Phase 38 imports derive `label` from `filepath.Base(...)` and leave `file_type` empty unless future phases populate it.
- WDL `parameter_meta` is intentionally permissive (`map[string]any` / `types.AnyType{}`), so Phase 39 must parse defensively and use comma-ok style assertions instead of assuming a strict schema.

### Integration Points
- `internal/server/dispatch.go` workflow success path around run completion and provenance updates
- `internal/store/store.go` and `internal/store/sqlite/managed_files.go` for the run completion transaction and managed-file insertion API
- `wdl/workflow/workflow.go` plus any surrounding AST access needed to map workflow output names to declared output types and `parameter_meta`
- Existing tests under `internal/store/sqlite/managed_files_test.go`, `internal/server`, and spectool-related workflow execution suites that protect compatibility

</code_context>

<specifics>
## Specific Ideas

- The long-standing ambiguity in `jsonToValue(...)` between JSON strings and true WDL `File` values should not drive registration. Phase 39 should rely on declared workflow output types, not string-path heuristics.
- Existing non-type `parameter_meta` usage such as `{ help: "...", suggestions: [...] }` must continue to execute without panic or behavior change.
- Directory outputs should be treated as managed library entries in the same way as file outputs, not ignored or downgraded.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 39-workflow-completion-hook-and-type-annotation*
*Context gathered: 2026-04-19*
