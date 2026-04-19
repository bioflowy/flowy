# Phase 43: Search and Picker Label Polish - Context

**Gathered:** 2026-04-19
**Status:** Completed

## Phase Boundary

The Files tab and workflow launch surfaces should present managed files label-first and make them easier to find via client-side filtering.

## Decisions

- Files tab filtering is client-side and combines free-text search with a file-type filter.
- File rows show label first and path as supporting context.
- Workflow file picker remains path-backed but renders the label as the primary visible value.
- Existing manual path entry remains available as a fallback.

## Result

- Added search and type filtering to the Files tab.
- Switched Files tab rows and workflow picker trigger/items to label-first presentation.
- Added UI tests for filtering and label-first selection rendering.
