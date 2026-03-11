# Phase 8: Add spectool Integration for WDL Specification Compliance Testing - Research

**Researched:** 2026-03-11
**Domain:** WDL conformance testing, CLI output format, Makefile integration, Rust/Cargo toolchain
**Confidence:** HIGH

## Summary

Phase 8 integrates openwdl/spectool against flowy to verify WDL 1.2 specification compliance. The work has three distinct tracks: (1) modify flowy's CLI to emit outputs JSON to stdout on success (matching miniwdl behavior), (2) change File/Directory paths in outputs JSON from relative to absolute, and (3) create a Makefile with a `spectool` target that installs spectool via `cargo install`, builds flowy, and runs conformance tests with an `--exclude` file managing unsupported capabilities.

All implementation decisions are locked by CONTEXT.md. The `--redirect-stdout` flag in the spectool command template means flowy must print its JSON outputs to stdout; spectool reads that stdout as the engine's output. The existing `run(args, stdout, stderr)` testable-main pattern in `cmd/flowy/main.go` makes stdout-directed output straightforward to implement and test.

**Primary recommendation:** Implement stdout JSON output directly in the existing `runRun()` function by replacing the current `"Outputs written to ..."` message with the actual JSON content. Update `valueToJSONOutput` to use absolute paths. Build the Makefile target and `spectool-exclude.txt` as the final integration layer.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Output delivery method**
- Default output to stdout: `flowy run` prints outputs JSON to stdout on success (like miniwdl)
- Add `--output <path>` flag to write outputs JSON to a specified file path instead of stdout
- Verbose task logs (`--verbose`) go to stderr only, keeping stdout clean for JSON output
- Error output goes to stderr only; no JSON on stdout for failed executions
- Continue writing `execRoot/outputs.json` as before (both stdout/file output AND execRoot copy)

**Output format changes**
- File/Directory values in outputs JSON use absolute paths (currently relative to execRoot) — matches miniwdl behavior
- Task single execution (`--task`) uses `taskName.outputName` dot-notation keys (same as workflow)
- None/Optional values remain as JSON `null` (current behavior preserved)

**Project integration**
- Makefile target: `make spectool` builds spectool, builds flowy, runs conformance tests
- spectool binary managed via `cargo install` from git clone (no submodule, no vendored Rust code)
- Expected failures managed via `--exclude` patterns in a file (e.g., `spectool-exclude.txt`), patterns removed as features are implemented

**Compliance scope**
- Test against WDL 1.2 release branch of openwdl/wdl repository
- All capabilities enabled (`--all-capabilities`); unsupported tests excluded via `--exclude`

### Claude's Discretion
- spectool command template construction (exact placeholder mapping)
- Makefile structure and caching of built artifacts
- Which tests to initially exclude based on known unsupported features (HTTP imports, sub-workflows, etc.)
- Exact stderr formatting for verbose output

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

## Standard Stack

### Core

| Library/Tool | Version | Purpose | Why Standard |
|-------------|---------|---------|--------------|
| openwdl/spectool | latest main | WDL conformance test runner | Official WDL conformance tool |
| Rust/Cargo | system | Build spectool | spectool is a Rust binary |
| Go standard library | go 1.25 | CLI flag parsing (`flag` package) | Already used in codebase |
| GNU Make | system | Orchestrate build + test pipeline | Standard Unix build tool |

### No New Go Dependencies

This phase requires no new Go library dependencies. All changes use:
- `encoding/json` (already imported in `cmd/flowy/main.go`)
- `io` (already imported — `stdout io.Writer` flows through)
- `flag` (already imported — add `--output` flag to existing FlagSet)
- `os` (already imported — needed if writing to `--output` path)

**Installation (spectool — done in Makefile):**
```bash
git clone https://github.com/openwdl/spectool /tmp/spectool
cargo install --path /tmp/spectool
```

**Installation (WDL 1.2 spec — done in Makefile):**
```bash
git clone --branch wdl-1.2 --depth 1 https://github.com/openwdl/wdl /tmp/wdl-spec
```

## Architecture Patterns

### Recommended Project Structure After Phase 8

```
flowy/
├── cmd/flowy/
│   ├── main.go          # add --output flag; print JSON to stdout on success
│   ├── main_test.go     # existing tests
│   ├── run_test.go      # update + add tests for stdout JSON, --output flag, task dot-notation
│   └── testdata/
├── wdl/workflow/
│   ├── output.go        # change File/Directory to absolute paths; extract JSON serializer
│   └── output_test.go   # update existing tests; add absolute-path tests
├── Makefile             # new: spectool target + build targets
└── spectool-exclude.txt # new: one pattern per line, tests to skip
```

### Pattern 1: spectool Command Template

spectool expects the engine command as a shell template with four placeholders:

```
~{path}    — absolute path to the WDL test file
~{input}   — absolute path to the input JSON file
~{output}  — absolute path where engine should write outputs JSON (used without --redirect-stdout)
~{target}  — workflow or task name to execute
```

**With `--redirect-stdout`** (flowy's approach): spectool reads the engine's stdout as the outputs JSON. The `~{output}` placeholder is omitted from the template. The engine must print valid JSON to stdout on success and exit 0. On failure the engine exits non-zero.

Proposed flowy template:
```
spectool test "flowy run ~{path} --input ~{input} --task ~{target}" \
  --redirect-stdout \
  --all-capabilities \
  --exclude-file spectool-exclude.txt \
  -s /path/to/wdl-spec
```

Note: the `--task` flag currently only accepts a task name. When spectool passes a workflow name as `~{target}`, flowy ignores it for workflow execution (no `--task` flag). A conditional template may be needed; alternatively, always pass `--task ~{target}` and let flowy handle both task and workflow names by checking both lists.

**Alternative approach** (using `--output`): flowy writes outputs JSON to a file at `~{output}` path. This avoids the stdout-parsing mode and is simpler to implement but requires spectool to not use `--redirect-stdout`. Per CONTEXT.md the `--output` flag will be added, enabling this approach.

### Pattern 2: stdout JSON Output (flowy run)

Current behavior of `runRun()` on success (line 257 of `cmd/flowy/main.go`):
```go
fmt.Fprintf(stdout, "Outputs written to %s/outputs.json\n", execRoot)
```

New behavior: write the JSON content to stdout. The `writeOutputsJSON` function currently writes to a file; we need the JSON bytes to also go to `stdout io.Writer`.

**Approach A: return JSON bytes from writeOutputsJSON**
```go
// In workflow/output.go — change signature to return encoded bytes
func writeOutputsJSON(wfName string, outputs map[string]values.Value, execRoot string, destPath string) ([]byte, error)
// caller writes bytes to both file and stdout
```

**Approach B: add a separate serialization function**
```go
// In workflow/output.go
func OutputsToJSON(wfName string, outputs map[string]values.Value) ([]byte, error)
// caller: json bytes → stdout, then write to file
```

Approach B is cleaner (single responsibility). The planner should prefer Approach B.

### Pattern 3: Absolute Paths for File/Directory Outputs

Current `valueToJSONOutput` in `wdl/workflow/output.go` (line 48-58):
```go
case values.FileValue:
    rel, err := filepath.Rel(execRoot, val.Value)
    if err != nil {
        return val.Value
    }
    return rel
```

New behavior: return `val.Value` directly (the absolute path). The `execRoot` parameter can be removed from the function signature since it is no longer needed. Existing tests (`TestWriteOutputsJSON_FileRelativePath`, `TestWriteOutputsJSON_DirectoryRelativePath`) must be updated to expect absolute paths.

### Pattern 4: Task Single Execution Dot-Notation Keys

When `--task` flag is used, `task.RunTask()` returns `map[string]values.Value` but flowy does not currently write outputs.json for task runs (see `cmd/flowy/main.go` line 230-235 — no outputs.json write for `--task` path). Phase 8 must:

1. Write outputs.json to execRoot with keys `taskName.outputName` (matching workflow behavior)
2. Print the JSON to stdout

### Pattern 5: Makefile Structure

```makefile
SPECTOOL_BIN := $(HOME)/.cargo/bin/spectool
WDL_SPEC_DIR := /tmp/wdl-spec
FLOWY_BIN := ./flowy

.PHONY: spectool build

build: $(FLOWY_BIN)

$(FLOWY_BIN):
    go build -o flowy ./cmd/flowy

$(SPECTOOL_BIN):
    git clone --branch main --depth 1 https://github.com/openwdl/spectool /tmp/spectool-src
    cargo install --path /tmp/spectool-src

$(WDL_SPEC_DIR):
    git clone --branch wdl-1.2 --depth 1 https://github.com/openwdl/wdl $(WDL_SPEC_DIR)

spectool: $(FLOWY_BIN) $(SPECTOOL_BIN) $(WDL_SPEC_DIR)
    $(SPECTOOL_BIN) test "$(FLOWY_BIN) run ~{path} --input ~{input} --task ~{target}" \
        --redirect-stdout \
        --all-capabilities \
        --exclude-file spectool-exclude.txt \
        -s $(WDL_SPEC_DIR)
```

### Anti-Patterns to Avoid

- **Mixing stdout and stderr in runRun:** All verbose output and "Execution directory:" messages already go to `stderr`. Keep this invariant — stdout must be clean JSON only.
- **Removing the execRoot copy of outputs.json:** CONTEXT.md requires both stdout AND execRoot/outputs.json. Write to both.
- **Using `--task` for workflow runs:** spectool passes `~{target}` which is either a workflow or task name. The command template must handle this distinction.
- **Vendoring Rust source:** No submodule, no checked-in Rust code. `cargo install` from a temporary clone is the decided approach.
- **Forgetting to update existing tests:** `TestWriteOutputsJSON_FileRelativePath` expects relative paths. These tests must be updated to expect absolute paths.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| WDL conformance test extraction | Custom test parser of SPEC.md | spectool | Tests are embedded in the spec as code blocks; spectool handles extraction |
| Cargo binary installation | Custom Rust build scripts | `cargo install --path` | Standard Rust toolchain handles compilation and installation |
| Exclude pattern matching | Custom pattern filter | spectool `--exclude-file` | Built-in spectool flag reads patterns from a file |
| JSON output formatting | Custom JSON serializer | `encoding/json.MarshalIndent` | Already used with 2-space indent in existing code |

**Key insight:** spectool handles ALL the complexity of extracting tests from the spec, running the engine, comparing outputs, and reporting pass/fail. Flowy only needs to emit the right output format.

## Common Pitfalls

### Pitfall 1: stdout Contamination

**What goes wrong:** Any non-JSON bytes written to stdout (e.g., "Execution directory: /tmp/flowy-...") cause spectool's JSON parser to fail when using `--redirect-stdout`.

**Why it happens:** The current code writes "Execution directory:" to stderr correctly, but the final line writes to stdout. If any logging leaks to stdout, spectool fails all tests.

**How to avoid:** After the change, stdout must emit ONLY the JSON object (with newline). "Execution directory:" already goes to stderr. Verify with: `flowy run test.wdl | jq .` — must parse cleanly.

**Warning signs:** spectool reports "invalid JSON" or "parse error" on every test.

### Pitfall 2: Relative vs. Absolute Paths Breaking spectool Comparison

**What goes wrong:** spectool compares File/Directory output values against expected values from the spec. If flowy emits relative paths but spectool expects absolute (or vice versa), all file-output tests fail.

**Why it happens:** Current `valueToJSONOutput` uses `filepath.Rel()`. The spec tests expect the engine to report the actual file location.

**How to avoid:** Emit absolute paths as stored in `values.FileValue.Value` and `values.DirectoryValue.Value`. These are already absolute (task execution stores absolute host paths).

**Warning signs:** File-output tests all fail; non-file tests pass.

### Pitfall 3: Task Run Missing outputs.json

**What goes wrong:** When running with `--task`, flowy currently returns after `task.RunTask()` succeeds without writing outputs.json or printing to stdout. spectool sees no output and marks the test as failed.

**Why it happens:** The original design focused on workflow runs; task runs were a debugging shortcut without full output handling.

**How to avoid:** After `task.RunTask()` succeeds, serialize outputs with `taskName.outputName` keys, write to execRoot/outputs.json, and print to stdout.

**Warning signs:** All `--task`-based spectool tests fail; workflow tests pass.

### Pitfall 4: spectool --exclude-file Flag Availability

**What goes wrong:** Assuming spectool supports `--exclude-file` without verifying. The flag may be named differently (e.g., `--exclude` takes a single pattern, not a file).

**Why it happens:** Limited spectool documentation available; relying on assumed flag names.

**How to avoid (LOW confidence):** After installing spectool, run `spectool test --help` to inspect all available flags. If `--exclude-file` does not exist, use multiple `--exclude pattern` flags from a shell loop reading `spectool-exclude.txt`, or use shell expansion: `$(cat spectool-exclude.txt | xargs -I{} echo --exclude {})`.

**Warning signs:** `spectool test --help` does not list `--exclude-file`.

### Pitfall 5: Cargo Not in PATH During Make

**What goes wrong:** `cargo` is installed at `~/.cargo/bin/cargo` but not in the system PATH when Make runs.

**Why it happens:** Cargo's shell integration adds `source $HOME/.cargo/env` to `.bashrc`, but Make does not source shell profiles.

**How to avoid:** In Makefile, prefix cargo commands with `PATH := $(HOME)/.cargo/bin:$(PATH)` or use the full path `$(HOME)/.cargo/bin/cargo install ...`.

**Warning signs:** `make spectool` fails with "cargo: command not found".

### Pitfall 6: ~{target} for Workflow vs Task Runs

**What goes wrong:** spectool passes the workflow name as `~{target}`. If flowy's template always includes `--task ~{target}`, workflow tests will fail (--task looks for a task, not a workflow).

**Why it happens:** spectool uses `~{target}` for both workflow and task names.

**How to avoid:** Two options:
1. Omit `--task ~{target}` from the template — flowy then runs the first workflow, which works if the test WDL has one workflow. Tasks-only WDL files would need a separate template.
2. Teach flowy to check both task list and workflow list when `--task` is provided, falling back to workflow execution if no matching task is found.

Option 2 is more robust. Per CONTEXT.md the exact template construction is Claude's discretion.

## Code Examples

Verified patterns from existing codebase:

### Current stdout message (to be replaced)

```go
// cmd/flowy/main.go line ~257 — current:
fmt.Fprintf(stdout, "Outputs written to %s/outputs.json\n", execRoot)

// After change — print actual JSON:
fmt.Fprintf(stdout, "%s\n", jsonBytes)
```

### Extracting JSON bytes from outputs (new helper)

```go
// wdl/workflow/output.go — new function
func SerializeOutputs(wfName string, outputs map[string]values.Value) ([]byte, error) {
    data := make(map[string]interface{}, len(outputs))
    for name, v := range outputs {
        key := wfName + "." + name
        data[key] = valueToJSONOutput(v)
    }
    return json.MarshalIndent(data, "", "  ")
}
```

### Updated valueToJSONOutput (absolute paths, no execRoot)

```go
// wdl/workflow/output.go — updated FileValue/DirectoryValue cases
case values.FileValue:
    return val.Value  // already absolute; no filepath.Rel needed
case values.DirectoryValue:
    return val.Value  // already absolute
```

### Task outputs serialization (new in runRun --task path)

```go
// cmd/flowy/main.go — after task.RunTask() succeeds
jsonBytes, err := workflow.SerializeOutputs(*taskFlag, taskOutputs)
if err != nil {
    fmt.Fprintf(stderr, "error: serializing task outputs: %v\n", err)
    return 1
}
// Write to execRoot/outputs.json
outPath := filepath.Join(execRoot, "outputs.json")
if err := os.WriteFile(outPath, jsonBytes, 0644); err != nil {
    fmt.Fprintf(stderr, "error: writing outputs.json: %v\n", err)
    return 1
}
// Print to stdout
fmt.Fprintf(stdout, "%s\n", jsonBytes)
```

### spectool test invocation (Makefile)

```makefile
spectool: $(FLOWY_BIN) $(SPECTOOL_BIN) $(WDL_SPEC_DIR)
	$(SPECTOOL_BIN) test \
		"$(abspath $(FLOWY_BIN)) run ~{path} --input ~{input} --task ~{target}" \
		--redirect-stdout \
		--all-capabilities \
		--exclude-file spectool-exclude.txt \
		-s $(WDL_SPEC_DIR)
```

### spectool-exclude.txt initial patterns

```
# HTTP imports — not supported (v2 roadmap)
HttpImport
http_import

# Sub-workflows — not supported (v2 roadmap)
SubWorkflow
sub_workflow
call_workflow

# Object type — deprecated, not implemented
ObjectType
object_literal
```

(Exact patterns depend on spectool test names; populate by running spectool once and observing failures.)

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Outputs written as message to stdout | Outputs JSON to stdout (miniwdl-style) | Phase 8 | spectool compatibility |
| File paths relative to execRoot | File paths absolute | Phase 8 | miniwdl compatibility |
| No task outputs.json | Task run writes outputs.json + stdout JSON | Phase 8 | spectool --task test support |
| No Makefile | Makefile with spectool target | Phase 8 | Reproducible conformance testing |

**Deprecated/outdated:**
- Relative File/Directory paths in outputs JSON: being replaced by absolute paths in this phase.
- "Outputs written to X/outputs.json" stdout message: replaced with actual JSON content.

## Open Questions

1. **spectool --exclude-file flag name**
   - What we know: spectool has `--exclude` for single patterns; a file-based variant may exist
   - What's unclear: exact flag name (`--exclude-file`, `--excludes-file`, etc.)
   - Recommendation: run `spectool test --help` after installation; fall back to shell loop if no file variant exists
   - Confidence: LOW

2. **~{target} semantics — workflow vs. task**
   - What we know: spectool passes the WDL entity name (workflow or task) as `~{target}`
   - What's unclear: whether spectool uses different templates for workflow vs. task tests, or always passes `--task`-equivalent
   - Recommendation: plan to teach flowy to check task list first, then workflow list when `--task` is given; implement as a fallback in `runRun()`
   - Confidence: MEDIUM

3. **spectool --output-selector need**
   - What we know: miniwdl requires `--output-selector '.outputs'` because its stdout JSON has extra fields beyond the outputs map
   - What's unclear: whether flowy's stdout JSON (which will be exactly the outputs map) needs `--output-selector`
   - Recommendation: do NOT add `--output-selector` initially; flowy's stdout is the outputs JSON directly with no wrapper. If spectool fails, investigate.
   - Confidence: MEDIUM

4. **WDL 1.2 spec test count and categories**
   - What we know: spectool extracts tests from SPEC.md; WDL 1.2 branch is at openwdl/wdl wdl-1.2
   - What's unclear: exact number of tests and which are gated by capabilities (HTTP imports, sub-workflows, etc.)
   - Recommendation: run `spectool test --dry-run` (if available) or a no-op engine to enumerate tests before implementing excludes
   - Confidence: LOW

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go testing (`go test`) |
| Config file | none — standard `go test ./...` |
| Quick run command | `go test ./cmd/flowy/ ./wdl/workflow/` |
| Full suite command | `go test ./... && make spectool` |

### Phase Requirements → Test Map

| Behavior | Test Type | Automated Command | File Exists? |
|----------|-----------|-------------------|-------------|
| `flowy run` prints outputs JSON to stdout on success | unit | `go test ./cmd/flowy/ -run TestRunWorkflowStdoutJSON` | Wave 0 |
| `flowy run --output <path>` writes JSON to specified file | unit | `go test ./cmd/flowy/ -run TestRunWithOutputFlag` | Wave 0 |
| `flowy run --task` prints outputs JSON with taskName.key | unit | `go test ./cmd/flowy/ -run TestRunTaskStdoutJSON` | Wave 0 |
| File/Directory values use absolute paths in JSON | unit | `go test ./wdl/workflow/ -run TestWriteOutputsJSON_FileAbsolutePath` | Wave 0 |
| stdout is clean JSON (no extra text) | unit | `go test ./cmd/flowy/ -run TestRunStdoutIsValidJSON` | Wave 0 |
| `make spectool` runs spectool without build errors | integration | `make spectool` | Wave 0 (Makefile) |
| spectool passes non-excluded tests | integration | `make spectool` | depends on spectool install |

### Sampling Rate

- **Per task commit:** `go test ./cmd/flowy/ ./wdl/workflow/ -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** `go test ./... -count=1 && make spectool` green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] Update `TestWriteOutputsJSON_FileRelativePath` → `TestWriteOutputsJSON_FileAbsolutePath` in `wdl/workflow/output_test.go`
- [ ] `TestRunWorkflowStdoutJSON` — stdout is valid JSON, keys have `wfName.` prefix
- [ ] `TestRunWithOutputFlag` — `--output <path>` writes JSON to that file, NOT stdout
- [ ] `TestRunTaskStdoutJSON` — `--task` stdout JSON has `taskName.outputName` keys
- [ ] `TestRunStdoutIsValidJSON` — `json.Unmarshal(stdout)` succeeds, no extra text
- [ ] `Makefile` — targets: `build`, `spectool`; needs to exist before spectool integration

## Sources

### Primary (HIGH confidence)

- Official spectool README (github.com/openwdl/spectool) — command template format, `--redirect-stdout`, `--all-capabilities`, `--exclude` flags
- `cmd/flowy/main.go` (codebase) — existing `runRun()` function, `run(args, stdout, stderr)` pattern, current stdout behavior
- `wdl/workflow/output.go` (codebase) — `writeOutputsJSON()`, `valueToJSONOutput()`, current relative path logic
- `wdl/task/task.go` (codebase) — `RunTask()` return value, no current outputs.json write for task path
- CONTEXT.md (project) — locked decisions, template format, exclude file approach

### Secondary (MEDIUM confidence)

- miniwdl runner reference (miniwdl.readthedocs.io) — confirms stdout JSON output on success, absolute paths behavior
- spectool engine examples (multiple search results) — Cromwell/MiniWDL/Sprocket command templates confirming `~{path}`, `~{input}`, `~{target}` substitution pattern

### Tertiary (LOW confidence)

- spectool `--exclude-file` flag name — assumed from `--exclude` pattern; needs verification via `spectool test --help`
- WDL 1.2 test categories and counts — unknown without running spectool against the spec

## Metadata

**Confidence breakdown:**
- CLI output changes (stdout JSON, --output flag): HIGH — exact code locations known, pattern clear
- valueToJSONOutput absolute paths: HIGH — two-line change, existing tests need update
- Task run outputs.json + stdout: HIGH — gap clearly identified in code
- spectool command template: MEDIUM — confirmed `~{path}`, `~{input}`, `~{target}` pattern; `--redirect-stdout` usage confirmed
- spectool exclude mechanism: LOW — `--exclude-file` flag name unverified; fallback approach documented
- Initial exclude list contents: LOW — depends on running spectool and observing failures

**Research date:** 2026-03-11
**Valid until:** 2026-04-11 (spectool is actively developed; verify flag names at install time)
