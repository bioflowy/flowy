# Phase 42: File Provenance Metadata - Context

**Gathered:** 2026-04-19
**Status:** Completed

## Phase Boundary

Managed-file API responses should expose explicit source metadata so users can distinguish workflow outputs from manual imports and navigate back to the originating run when provenance exists.

## Decisions

- Add `source_type` to stored managed-file records.
- Use `workflow`, `upload`, and `import` as the explicit source values for new writes.
- Keep `run_id` as the provenance pointer for workflow-generated files.
- Backfill existing rows during migration using `run_id IS NULL` as the boundary.

## Result

- Added migration `00010_managed_file_source_type.sql`.
- Extended store and API models with `source_type` and `run_id`.
- Updated import and workflow registration code paths to populate provenance consistently.
