# Phase 39: Workflow Completion Hook and Type Annotation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-19
**Phase:** 39-workflow-completion-hook-and-type-annotation
**Areas discussed:** Annotation contract, Duplicate path policy, Completion strictness

---

## Annotation Contract

| Option | Description | Selected |
|--------|-------------|----------|
| Workflow-output `parameter_meta.type` only | Honor only `parameter_meta { output_name: { type: "..." } }` on workflow outputs; ignore malformed or missing type metadata | ✓ |
| Fallback to task metadata | Inherit from task-level `parameter_meta` when workflow output metadata is missing | |
| Heuristic inference | Infer type from path extensions or output shapes when metadata is absent | |

**User's choice:** Workflow-output `parameter_meta.type` only
**Notes:** Recommended option accepted.

---

## Duplicate Path Policy

| Option | Description | Selected |
|--------|-------------|----------|
| Deduplicate within a run by path | If multiple outputs in one run point to the same path, create one managed-file record for that run/path | ✓ |
| Preserve every output reference | Create one managed-file record per output declaration, even if paths repeat | |
| Make it configurable later | Leave the behavior open for planning/implementation to choose | |

**User's choice:** Deduplicate within a run by path
**Notes:** Recommended option accepted.

---

## Completion Strictness

| Option | Description | Selected |
|--------|-------------|----------|
| Strict atomic completion | If any output registration fails, the run does not become `COMPLETED` | ✓ |
| Best-effort registration | Mark the run complete even if some managed-file inserts fail | |
| Partial success reporting | Complete the run but track failed registrations separately | |

**User's choice:** Strict atomic completion
**Notes:** Recommended option accepted.

---

## the agent's Discretion

- Internal helper structure for traversing workflow outputs and gathering managed-file candidates
- Exact in-memory dedupe mechanism before calling `CompleteRun(...)`

## Deferred Ideas

None.
