# Phase 41: Label Template Engine - Context

**Gathered:** 2026-04-19
**Status:** Completed

## Phase Boundary

Workflow output auto-registration should support a safe `parameter_meta.label` template so managed-file labels can be derived from workflow input string values without invoking a shell.

## Decisions

- Support only `${var}`, `${var%suffix}`, `${var%%suffix}`, `${var#prefix}`, `${var##prefix}`.
- Evaluate templates in pure Go against declared workflow input names only.
- Any invalid template, missing variable, unsupported shape, or empty result falls back to `filepath.Base(path)`.
- Existing `parameter_meta.type` behavior must remain unchanged.

## Result

- Added a shared `parameter_meta` helper for `type` and `label`.
- Added a pure-Go label evaluator used during workflow output registration.
- Added regression tests for valid template evaluation and safe fallback behavior.
