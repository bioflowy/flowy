# Phase 7: CLI and Integration - Context

**Gathered:** 2026-03-10
**Status:** Ready for planning

<domain>
## Phase Boundary

Ship `flowy run` and `flowy check` as usable CLI tools that accept all documented flags, produce structured output, and exit with correct codes. This is the final v1 phase — it wires together parser, checker, task, and workflow packages into a user-facing binary.

</domain>

<decisions>
## Implementation Decisions

### CLI framework
- Go standard library only (flag package + manual subcommand dispatch)
- No external CLI framework (cobra, kong, etc.) — only 2 subcommands, matches project's minimal-dependency style
- Entry point at `cmd/flowy/main.go` — standard Go project layout, `go install github.com/bioflowy/flowy/cmd/flowy@latest` works
- `--version` flag with build-time injection via `-ldflags -X main.version=...`; prints "dev" if unset
- Unknown subcommands/flags: print usage + exit 1

### Output and error formatting
- `flowy check`: rustc-style diagnostics with file:line:col, source line, and caret pointing to the error position
- `flowy check` success: print short confirmation (e.g., "hello.wdl: OK")
- `flowy run` success: print path to outputs.json (e.g., "Outputs written to /tmp/flowy-abc123/outputs.json")
- `flowy run` failure: print human-readable error to stderr + point to error.json path for details; exit non-zero

### Execution directory structure
- Default (no --dir): auto-create unique directory under os.TempDir(), e.g., /tmp/flowy-20260310-abc123/
- Never auto-clean execution directories — always keep for inspection
- When --dir is specified: refuse to run if the directory exists and is non-empty (prevent accidental overwrite)
- Always print execution directory path to stderr at the start of a run, before any task execution begins

### Claude's Discretion
- Exact flag parsing implementation details
- Task subdirectory naming within execution directory
- Logging/verbosity levels beyond --verbose
- Help text wording and formatting
- Exit code values for different failure modes (parse error vs runtime error vs usage error)

</decisions>

<specifics>
## Specific Ideas

- CLI should feel familiar to bioinformatics users who know miniwdl — similar flag names and behavior
- `go install` should work out of the box with the cmd/flowy/ layout

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `parser.Parse(file, src)`: entry point for WDL parsing, returns `*ast.Document`
- `checker.Check(doc)`: validates types, returns `(TypeEnv, []error)` with line/column info via `ast.Pos`
- `task.RunTask()`: runs a single task with Docker, takes `ContainerRunner` + `execRoot`
- `workflow.RunWorkflow()`: executes full DAG, writes outputs.json/error.json, takes `execRoot`
- `task.DockerRunner`: concrete implementation of `ContainerRunner` interface

### Established Patterns
- All packages use clean public APIs with exported entry-point functions
- Error types carry `ast.Pos` for source location reporting
- `ContainerRunner` interface allows mock testing without Docker
- `execRoot` string parameter flows through workflow → task for file staging

### Integration Points
- `cmd/flowy/main.go` → `parser.Parse()` → `checker.Check()` → `workflow.RunWorkflow()` / `task.RunTask()`
- `--input` flag: JSON file → `map[string]interface{}` → passed to RunWorkflow/RunTask
- `--task` flag: skip workflow, call `task.RunTask()` directly with named task from `doc.Tasks`
- `--path` flag: extends import search paths used by `parser.Parse()`
- `--dir` flag: maps to `execRoot` parameter

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 07-cli-and-integration*
*Context gathered: 2026-03-10*
