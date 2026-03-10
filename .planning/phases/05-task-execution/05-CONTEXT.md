# Phase 5: Task Execution - Context

**Gathered:** 2026-03-10
**Status:** Ready for planning

<domain>
## Phase Boundary

Run a single WDL task end-to-end in Docker: parse JSON inputs, bind them to the task's input section, expand the command template, execute in a Docker container, capture stdout/stderr, collect output files to the host. This phase delivers a `wdl/task` package (or similar) that Phase 6 (Workflow Execution) will call for each `call` statement.

</domain>

<decisions>
## Implementation Decisions

### ContainerRunner interface
- Single `Run(ctx context.Context, ...)` method that handles the full Docker lifecycle internally (pull if needed, create, start, wait, capture logs, remove)
- Image pulling is implicit inside `Run()` — pulls if not already cached, no separate `EnsureImage()` call
- Accepts `context.Context` as first parameter for cancellation support (Phase 6 errgroup integration)
- Returns exit code, stdout bytes, stderr bytes, and error
- Mock implementation for unit tests: shell-based mock that executes command via `os/exec` (`sh -c`) on the host — real shell execution without Docker daemon

### Input file staging
- Bind-mount host files into container as read-only
- Flat container-internal path scheme: all input files mounted under `/mnt/inputs/<filename>` — collision handled by appending hash suffix if needed
- Directory-type inputs: bind-mount entire host directory as `/mnt/inputs/<dirname>/` read-only (same strategy as files, consistent)
- Bidirectional host-to-container path map maintained so output expressions can translate paths back to host

### Output collection
- Container writes to `/mnt/outputs/`, which is bind-mounted to the host task execution directory
- Output files appear on host automatically via the bind mount — no `docker cp` needed
- Output expressions evaluated after container completes with `ExecContext{WorkDir, StdoutPath, StderrPath}` pointing to host paths

### Execution directory layout
- miniwdl-style nested: `execution/<workflow>/<call-name>/` containing:
  - `work/` — output files (bind-mounted as `/mnt/outputs/` in container)
  - `stdout.txt` — captured stdout
  - `stderr.txt` — captured stderr
  - `command` — expanded command script (written to disk before execution, aids debugging)
  - `rc` — exit code file (single integer, standard WDL convention)
- Execution root defaults to `./execution/` under CWD; CLI `--dir` flag (Phase 7) overrides
- Command passed to container as `bash /mnt/inputs/_command` (script file bind-mounted into container)

### Runtime section mapping
- v1 supports core keys only: `docker`/`container` (image name), `cpu`, `memory`
- `maxRetries`, `returnCodes`, `gpu`, `disks`, `preemptible` deferred to v2
- Memory applied as Docker `--memory` hard limit — container OOM-killed if exceeded (e.g., `"2 GiB"` -> 2147483648 bytes)
- CPU applied as Docker `--cpus` flag
- When `runtime.docker` is missing: use `ubuntu:latest` as default image (matches Cromwell behavior)

### Claude's Discretion
- Exact `ContainerRunner` method signature (parameter struct vs. individual args)
- `wdl/task` package structure and public API design
- Memory string parsing (`"2 GiB"`, `"512 MB"` -> bytes conversion)
- How to handle `env` modifier on task input declarations (WDL 1.2 feature: inject as environment variable)
- Error wrapping strategy for Docker client errors
- Whether to stream or batch stdout/stderr capture

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `wdl/eval.Eval(expr, env, ctx)`: Expression evaluator — Phase 5 calls this to evaluate input defaults, command template, and output expressions
- `wdl/eval.ExecContext{WorkDir, StdoutPath, StderrPath}`: Lightweight context struct — Phase 5 populates all fields before evaluating output section
- `wdl/eval.Env`: Linked-list scope for variable bindings — Phase 5 creates env with input values, then extends with intermediate declarations
- `wdl/eval/command.go`: Command template expansion with sentinel-before-indent dedent — already handles `CommandExpr` evaluation
- `wdl/values.FileValue{Value: "/abs/path"}` and `DirectoryValue`: file/dir paths stored as absolute host paths — path mapping must translate these to container paths for command expansion, and back to host paths for output evaluation
- `wdl/ast.Task`: Complete task AST node with `Inputs`, `Decls`, `Command`, `Outputs`, `Runtime`, `Requirements` fields

### Established Patterns
- All errors include `ast.Pos` (File, Line, Col) — task execution errors should follow this pattern where applicable
- `NullValue{}` (not nil) is the sentinel for absent optional values
- Package boundary: new package `wdl/task` or `wdl/runner` (mirrors `wdl/eval`, `wdl/checker` structure)
- Stdlib functions (`glob()`, `stdout()`, `stderr()`, `size()`) already guard on `ExecContext` fields being non-empty

### Integration Points
- Input: `*ast.Task` node + JSON input map (`map[string]interface{}`) + `ContainerRunner` implementation
- Output: `map[string]values.Value` (task outputs keyed by declaration name) or error
- Phase 6 calls task executor for each `call` statement in the workflow DAG
- Phase 7 CLI `--task` flag calls task executor directly for single-task execution

</code_context>

<specifics>
## Specific Ideas

- Follow miniwdl's execution directory layout conventions for familiarity with WDL ecosystem users
- Shell-based mock runner enables full end-to-end testing of the task pipeline without Docker — real command execution on host
- Write `command` file to disk before execution for debuggability (can inspect what actually ran)
- Write `rc` file with exit code following Cromwell/miniwdl convention

</specifics>

<deferred>
## Deferred Ideas

- `maxRetries` and `returnCodes` runtime keys — v2 scope
- `gpu`, `disks`, `preemptible` runtime keys — v2 scope
- Task output caching/memoization — v2 (EXT-02)
- Streaming stdout/stderr during execution (for `--verbose` mode) — Phase 7 CLI concern

</deferred>

---

*Phase: 05-task-execution*
*Context gathered: 2026-03-10*
