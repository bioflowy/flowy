# Phase 8: Add spectool integration for WDL specification compliance testing - Context

**Gathered:** 2026-03-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Integrate openwdl/spectool conformance testing tool against flowy to verify WDL 1.2 specification compliance. This includes adjusting flowy's CLI output behavior to be spectool-compatible and creating Makefile targets for running conformance tests. No new WDL language features are added — this phase validates existing implementation.

</domain>

<decisions>
## Implementation Decisions

### Output delivery method
- Default output to stdout: `flowy run` prints outputs JSON to stdout on success (like miniwdl)
- Add `--output <path>` flag to write outputs JSON to a specified file path instead of stdout
- Verbose task logs (`--verbose`) go to stderr only, keeping stdout clean for JSON output
- Error output goes to stderr only; no JSON on stdout for failed executions
- Continue writing `execRoot/outputs.json` as before (both stdout/file output AND execRoot copy)

### Output format changes
- File/Directory values in outputs JSON use absolute paths (currently relative to execRoot) — matches miniwdl behavior
- Task single execution (`--task`) uses `taskName.outputName` dot-notation keys (same as workflow)
- None/Optional values remain as JSON `null` (current behavior preserved)

### Project integration
- Makefile target: `make spectool` builds spectool, builds flowy, runs conformance tests
- spectool binary managed via `cargo install` from git clone (no submodule, no vendored Rust code)
- Expected failures managed via `--exclude` patterns in a file (e.g., `spectool-exclude.txt`), patterns removed as features are implemented

### Compliance scope
- Test against WDL 1.2 release branch of openwdl/wdl repository
- All capabilities enabled (`--all-capabilities`); unsupported tests excluded via `--exclude`

### Claude's Discretion
- spectool command template construction (exact placeholder mapping)
- Makefile structure and caching of built artifacts
- Which tests to initially exclude based on known unsupported features (HTTP imports, sub-workflows, etc.)
- Exact stderr formatting for verbose output

</decisions>

<specifics>
## Specific Ideas

- miniwdl is the reference for output format behavior: stdout JSON, absolute File paths, dot-notation keys
- spectool template will look like: `flowy run ~{path} --input ~{input} --task ~{target} --redirect-stdout`
- The `--output` flag provides an alternative to stdout for scripts that need explicit file output

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `wdl/workflow/output.go`: `writeOutputsJSON()` and `valueToJSONOutput()` — needs modification for absolute paths and stdout output
- `cmd/flowy/main.go`: CLI entry point with testable `run()` pattern — needs `--output` flag and stdout JSON output
- `cmd/flowy/run_test.go`: `mockRunner` infrastructure — can test new output behavior without Docker

### Established Patterns
- Testable-main pattern: `run(args, stdout, stderr)` — stdout JSON output fits naturally
- `newRunner` factory var for dependency injection — existing pattern for mock testing
- Pretty-printed JSON with 2-space indent for outputs.json

### Integration Points
- `cmd/flowy/main.go` line ~90: where outputs.json path is currently hardcoded to execRoot
- `wdl/workflow/output.go` line ~30: `valueToJSONOutput()` uses `filepath.Rel()` for path — change to absolute
- `wdl/workflow/workflow.go` line ~100-114: workflow output evaluation and writing

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 08-add-spectool-integration-for-wdl-specification-compliance-testing*
*Context gathered: 2026-03-11*
