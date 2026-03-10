# Phase 7: CLI and Integration - Research

**Researched:** 2026-03-10
**Domain:** Go CLI (stdlib flag package), binary wiring of parser/checker/task/workflow
**Confidence:** HIGH

## Summary

Phase 7 is a wiring phase: all six underlying packages (parser, checker, task, workflow, eval, values) are fully built and green. The only work is creating `cmd/flowy/main.go`, which glues them together behind two subcommands. There are no new library choices to make — the user locked Go stdlib `flag` package + manual subcommand dispatch. No external dependencies are introduced.

The one non-trivial technical challenge is `--verbose`: `task.RunTask` and `workflow.RunWorkflow` currently have no way to stream or print container stdout/stderr during execution. The current design captures them into `RunResult.Stdout`/`RunResult.Stderr` after the container finishes. `--verbose` requires printing those to the terminal after each task, which can be done post-hoc in the CLI without touching inner packages. A richer approach (streaming) would require modifying `RunRequest`/`ContainerRunner`, but post-hoc is sufficient for v1.

The `--path` flag for import search paths is not yet supported by `parser.Parse(file, src)` — that function resolves imports relative to the file's directory only. Adding `--path` support requires either extending `Parse` or adding a `ParseWithSearchPaths` variant. This is the only package-level API gap.

**Primary recommendation:** Create `cmd/flowy/main.go` with `flag`-based subcommand dispatch, extend `parser.Parse` minimally to accept search paths, and print verbose output post-hoc from `RunResult`.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- Go standard library only (flag package + manual subcommand dispatch)
- No external CLI framework (cobra, kong, etc.) — only 2 subcommands, matches project's minimal-dependency style
- Entry point at `cmd/flowy/main.go` — standard Go project layout, `go install github.com/bioflowy/flowy/cmd/flowy@latest` works
- `--version` flag with build-time injection via `-ldflags -X main.version=...`; prints "dev" if unset
- Unknown subcommands/flags: print usage + exit 1
- `flowy check`: rustc-style diagnostics with file:line:col, source line, and caret pointing to the error position
- `flowy check` success: print short confirmation (e.g., "hello.wdl: OK")
- `flowy run` success: print path to outputs.json (e.g., "Outputs written to /tmp/flowy-abc123/outputs.json")
- `flowy run` failure: print human-readable error to stderr + point to error.json path for details; exit non-zero
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

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CLI-01 | `flowy check <file.wdl>` — parse + type-check only, print errors with line numbers, exit 0/non-zero | `parser.Parse` returns `*ParseError{File,Line,Col,Message}`; `checker.Check` returns `[]error` of `*TypeError{Pos,Message}`. Both carry full location. Rustc-style formatting adds caret under error column. |
| CLI-02 | `flowy run <file.wdl>` — execute workflow, `--input` flag accepts JSON file | `workflow.RunWorkflow(ctx, doc, wfName, jsonInputs, runner, execRoot)` is the entry point. `wfName` is looked up from `doc.Workflows[0].Name` when only one workflow exists. `jsonInputs` is `map[string]interface{}` loaded via `json.Unmarshal`. |
| CLI-03 | `--task <name>` — run only named task, skip workflow | `task.RunTask(ctx, task, jsonInputs, runner, execRoot)` — lookup `task` by name from `doc.Tasks`. Skip `RunWorkflow`. |
| CLI-04 | `--path <dir>` — extend import search path | `parser.Parse` currently resolves imports relative to the source file's directory only. A `ParseWithSearchPaths(file, src, paths []string)` variant (or extended `parseFile` with paths parameter) is needed. |
| CLI-05 | `--dir <dir>` — set working directory for execution | Maps directly to `execRoot` parameter in `RunWorkflow`/`RunTask`. Non-empty directory check before use. |
| CLI-06 | Exit 0 on success, non-zero on failure | Standard `os.Exit(0/1)`. Planner decides specific exit codes per failure mode (Claude's discretion). |
| CLI-07 | `--verbose` — print task stdout/stderr to terminal during execution | `RunResult.Stdout` and `RunResult.Stderr` are available after each task. Post-hoc print to `os.Stdout`/`os.Stderr` is sufficient for v1. Does not require streaming or modifying inner packages. |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `flag` (stdlib) | go1.25 | Subcommand + flag parsing | Project constraint; sufficient for 2 subcommands |
| `os` (stdlib) | go1.25 | Exit codes, file I/O, TempDir | Standard Go CLI plumbing |
| `encoding/json` (stdlib) | go1.25 | Decode `--input` JSON file | Already used in workflow/task packages |
| `fmt` (stdlib) | go1.25 | Diagnostic formatting | Standard |
| `time` (stdlib) | go1.25 | Unique exec dir name generation | `time.Now().Format("20060102-150405")` |
| `path/filepath` (stdlib) | go1.25 | Path joining, abs resolution | Already used throughout codebase |
| `crypto/rand` or `math/rand` (stdlib) | go1.25 | Random suffix for exec dir uniqueness | Short hex suffix prevents collisions |

### No New External Dependencies
This phase adds zero new external dependencies. All existing deps (Docker moby client, errgroup) are already in `go.mod`.

## Architecture Patterns

### Recommended Project Structure
```
cmd/
└── flowy/
    └── main.go          # entry point — subcommand dispatch, flag parsing
wdl/
└── parser/
    └── parser.go        # extend Parse() to accept search paths (or add ParseWithPaths)
```

Single file `cmd/flowy/main.go` is appropriate given only 2 subcommands and minimal logic. If the file exceeds ~200 lines the planner should split into `cmd/flowy/check.go` and `cmd/flowy/run.go`, but a single file is the default.

### Pattern 1: Manual Subcommand Dispatch with `flag.FlagSet`
**What:** Parse global flags first, then dispatch to per-subcommand FlagSets.
**When to use:** Always — this is the only flag approach.

```go
// Source: Go stdlib flag package docs
func main() {
    if len(os.Args) < 2 {
        usage()
        os.Exit(1)
    }
    switch os.Args[1] {
    case "check":
        runCheck(os.Args[2:])
    case "run":
        runRun(os.Args[2:])
    default:
        fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", os.Args[1])
        usage()
        os.Exit(1)
    }
}

func runCheck(args []string) {
    fs := flag.NewFlagSet("check", flag.ExitOnError)
    pathFlag := fs.String("path", "", "additional import search directory")
    dirFlag  := fs.String("dir", "", "working directory")
    fs.Parse(args)
    // ...
}
```

### Pattern 2: Exec Directory Creation
**What:** Generate unique temp dir or validate user-specified dir.

```go
// Default: unique dir under os.TempDir()
func makeExecDir() (string, error) {
    suffix := fmt.Sprintf("flowy-%s-%x",
        time.Now().Format("20060102"),
        rand6bytes())
    dir := filepath.Join(os.TempDir(), suffix)
    return dir, os.MkdirAll(dir, 0755)
}

// --dir specified: refuse if non-empty
func validateExecDir(dir string) error {
    entries, err := os.ReadDir(dir)
    if err == nil && len(entries) > 0 {
        return fmt.Errorf("--dir %q already exists and is non-empty", dir)
    }
    return os.MkdirAll(dir, 0755)
}
```

### Pattern 3: Rustc-style Diagnostic Formatting (CLI-01)
**What:** Print `file:line:col: error: message`, then source line, then caret.
**Constraint:** Requires reading the source file to display the error line. `parser.ParseError` has `File`/`Line`/`Col`/`Message`; `checker.TypeError` has `Pos` (same fields).

```go
func printDiagnostic(w io.Writer, file string, line, col int, msg string) {
    fmt.Fprintf(w, "%s:%d:%d: error: %s\n", file, line, col, msg)
    // Read source line for context
    if src := readSourceLine(file, line); src != "" {
        fmt.Fprintln(w, " "+src)
        fmt.Fprintln(w, " "+strings.Repeat(" ", col-1)+"^")
    }
}
```

### Pattern 4: `--verbose` Post-hoc Output
**What:** After `RunTask` completes, print `RunResult.Stdout`/`Stderr` if verbose flag is set.
**Why post-hoc:** `RunResult` already captures both streams. `RunTask` writes them to `stdout.txt`/`stderr.txt` in the exec dir. The CLI can read those files or pass captured bytes.
**Important:** The current `RunTask` returns `(map[string]values.Value, error)`, not `RunResult`. Stdout/stderr are written to disk but not returned. For `--verbose`, the CLI can read `execRoot/taskName/stdout.txt` after each task. Alternatively, `RunTask` signature can be extended to optionally pass an `io.Writer` for streaming — but this changes the existing API.

**Recommended approach for v1:** After `RunWorkflow` completes (or fails), walk `execRoot` task subdirectories and print `stdout.txt`/`stderr.txt`. This avoids API changes and is correct for v1.

**Alternative (cleaner for verbose during execution):** Pass `verbose bool` or `io.Writer` through `RunWorkflow` → `RunTask`. This requires modifying `workflow.go` and `task.go` signatures but is cleaner. Given the phase is integration-focused, this API extension is acceptable.

### Pattern 5: `--path` Import Search Path
**What:** Extend parser to try additional directories when resolving relative imports.
**Gap identified:** `parser.Parse(file, src)` resolves imports relative to `filepath.Dir(file)` only (confirmed in `parseFile`). `--path` requires searching additional dirs.

**Minimal extension:** Add `ParseWithSearchPaths(file string, src []byte, searchPaths []string)` that passes paths into `parseFile`. The `parseFile` function tries `filepath.Dir(file)` first, then each search path in order.

```go
// New public function in parser package
func ParseWithSearchPaths(file string, src []byte, searchPaths []string) (*ast.Document, error) {
    abs, err := filepath.Abs(file)
    // ...
    return parseFileWithPaths(abs, src, map[string]bool{}, []string{}, searchPaths)
}
```

### Pattern 6: `--version` Build-time Injection
```go
var version = "dev" // overridden at build: -ldflags "-X main.version=v1.0.0"

// In main():
if len(os.Args) == 2 && os.Args[1] == "--version" {
    fmt.Println("flowy " + version)
    os.Exit(0)
}
```

### Anti-Patterns to Avoid
- **Global `flag.Parse()` without FlagSet:** Breaks per-subcommand flags. Use `flag.NewFlagSet` per subcommand.
- **Auto-cleaning exec dirs:** Explicitly disallowed — never call `os.RemoveAll` on exec dirs.
- **Printing exec dir after task start:** Must print to stderr BEFORE any task execution begins.
- **Silently overwriting non-empty `--dir`:** Must check and refuse.
- **Single `os.Exit` at the end:** Subcommand functions should call `os.Exit` directly, not return error codes to main.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON input parsing | Custom JSON reader | `encoding/json` stdlib | Already used in workflow package |
| Temp dir generation | Complex naming scheme | `os.TempDir()` + `time.Now().Format()` + short rand | Sufficient uniqueness, simple |
| Workflow execution | Custom DAG runner | `workflow.RunWorkflow()` | Fully implemented in Phase 6 |
| Task execution | Custom container launcher | `task.RunTask()` | Fully implemented in Phase 5 |
| Type checking | Custom type verifier | `checker.Check()` | Fully implemented in Phase 3 |
| WDL parsing | Custom parser | `parser.Parse()` | Fully implemented in Phase 2 |

**Key insight:** This is an integration phase. All hard problems are solved. The CLI is a thin dispatcher.

## Common Pitfalls

### Pitfall 1: `--path` Flag Has No Parser Support
**What goes wrong:** CLI accepts `--path` but parser ignores it — imports from search paths fail silently with "cannot read file" error.
**Why it happens:** `parser.Parse` only looks in `filepath.Dir(file)`. The `--path` support was never needed by earlier phases.
**How to avoid:** Add `ParseWithSearchPaths` to the parser package in Wave 0 (before the CLI uses it). Extend `parseFile` to accept `searchPaths []string` and try each when the primary path fails.
**Warning signs:** Tests with `--path` fail with "import not found" even when the file exists in the search path.

### Pitfall 2: `--verbose` Requires Walking Exec Dir Post-Run
**What goes wrong:** `RunWorkflow` returns after all tasks complete; there's no hook to print per-task output during execution.
**Why it happens:** `RunTask` writes stdout.txt/stderr.txt to disk but doesn't return them — only `RunResult` in `DockerRunner.Run` has the raw bytes, but `RunTask` discards them after writing to disk.
**How to avoid:** Either (a) walk `execRoot/*/stdout.txt` and `execRoot/*/stderr.txt` after `RunWorkflow` returns, or (b) extend `RunTask`/`RunWorkflow` to accept an `io.Writer` for verbose output. Option (b) is cleaner for the user experience. Option (a) requires zero API changes.
**Warning signs:** `--verbose` flag is parsed but output is never printed.

### Pitfall 3: Non-unique Exec Dir Race Condition
**What goes wrong:** Two concurrent `flowy run` invocations at the same second produce the same temp dir name.
**Why it happens:** `time.Now().Format("20060102-150405")` has 1-second resolution.
**How to avoid:** Append at least 4 random bytes (8 hex chars) after the timestamp. `math/rand` or `crypto/rand` both work.

### Pitfall 4: Rustc-style Diagnostic Requires Source File Read
**What goes wrong:** Printing the source line and caret requires re-reading the WDL file. The error only carries `File`/`Line`/`Col`.
**Why it happens:** Errors are returned after parse/check; raw source bytes are not preserved in the error.
**How to avoid:** After collecting errors, read the source file(s) and extract the relevant lines. Cache by filename to avoid redundant reads. For `flowy check`, the CLI already has the source file path from the command argument. For imported files, use `ast.Pos.File` (absolute path).

### Pitfall 5: `wfName` When File Has One Workflow
**What goes wrong:** `RunWorkflow(ctx, doc, wfName, ...)` requires a workflow name, but the CLI user doesn't specify one.
**Why it happens:** The public API requires an explicit name (not "first workflow").
**How to avoid:** When no `--workflow` flag is given (not in scope, but implicit), use `doc.Workflows[0].Name`. If `doc.Workflows` is empty, print error and exit non-zero.

### Pitfall 6: Exit Codes Must Be Consistent
**What goes wrong:** Using `os.Exit(1)` everywhere makes it impossible to distinguish usage errors from runtime errors.
**Why it happens:** Exit code values are Claude's discretion — but the choice should be made once and documented in help text.
**Recommendation:** `2` = usage error (bad flags, unknown subcommand), `1` = execution error (parse/check failure, task failure).

## Code Examples

Verified patterns from existing codebase:

### Parser Entry Point (confirmed from source)
```go
// Source: wdl/parser/parser.go line 78
// Parse(file, src) — src can be nil to read from disk
doc, err := parser.Parse(file, nil)
if err != nil {
    // err is *parser.ParseError with File, Line, Col, Message fields
}
```

### Checker Entry Point (confirmed from source)
```go
// Source: wdl/checker/checker.go line 67
typeEnv, errs := checker.Check(doc)
for _, e := range errs {
    var te *checker.TypeError
    if errors.As(e, &te) {
        // te.Pos.File, te.Pos.Line, te.Pos.Col, te.Message
    }
}
```

### RunWorkflow Entry Point (confirmed from source)
```go
// Source: wdl/workflow/workflow.go line 28
runner := task.NewDockerRunner()
ctx := context.Background()
outputs, err := workflow.RunWorkflow(ctx, doc, wfName, jsonInputs, runner, execRoot)
// On success: outputs.json written to execRoot/outputs.json
// On failure: error.json written to execRoot/error.json
```

### RunTask Entry Point (confirmed from source)
```go
// Source: wdl/task/task.go line 45
runner := task.NewDockerRunner()
outputs, err := task.RunTask(ctx, taskDef, jsonInputs, runner, execRoot)
```

### JSON Input Loading
```go
// Standard Go pattern — encoding/json already used in workflow package
data, err := os.ReadFile(inputFile)
if err != nil { /* ... */ }
var jsonInputs map[string]interface{}
if err := json.Unmarshal(data, &jsonInputs); err != nil { /* ... */ }
```

### Exec Dir Path Print (before task start)
```go
// Must print to stderr BEFORE RunWorkflow/RunTask is called
fmt.Fprintf(os.Stderr, "Execution directory: %s\n", execRoot)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Cobra/Viper for CLI | stdlib `flag` + manual dispatch | Phase 7 decision | Zero deps, simple |
| Streaming verbose output | Post-hoc file read from exec dir | Phase 7 decision | No API changes needed |

**Deprecated/outdated:**
- N/A — this is a new package with no legacy.

## Open Questions

1. **`--verbose` implementation strategy**
   - What we know: `RunTask` writes stdout.txt/stderr.txt to disk; `RunResult` bytes are available inside `DockerRunner.Run` but not returned to callers.
   - What's unclear: Should verbose print during execution (streaming) or after each task completes? The current API makes post-run reading the path of least resistance.
   - Recommendation: Planner should choose between (a) post-hoc file walk after `RunWorkflow` returns — zero API changes, output appears after all tasks finish; or (b) extend `RunTask`/`RunWorkflow` to accept an `io.Writer` verbose parameter — API change but output appears task-by-task. Option (b) is recommended for better UX.

2. **Workflow name resolution**
   - What we know: `RunWorkflow` requires `wfName` string. WDL files typically have one workflow.
   - What's unclear: Should CLI accept `--workflow <name>` flag or always use `doc.Workflows[0]`?
   - Recommendation: Default to `doc.Workflows[0].Name`; add `--workflow` flag only if multiple workflows are detected (error out with helpful message if ambiguous and no flag given). This is Claude's discretion per CONTEXT.md.

3. **`--path` parser extension**
   - What we know: `parseFile` in parser.go is unexported and resolves imports relative to `filepath.Dir(path)` only.
   - What's unclear: How invasive is the search-path extension?
   - Recommendation: Add `ParseWithSearchPaths(file, src, paths)` that passes `paths` into `parseFileWithPaths`. Inner function tries primary dir first, then each path in `searchPaths`. If no import found in any location, return the original "cannot read file" error pointing to the primary path. This is a clean additive API change.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing stdlib (`testing` package) |
| Config file | none — `go test ./...` from repo root |
| Quick run command | `go test ./cmd/flowy/... ./wdl/parser/...` |
| Full suite command | `go test ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CLI-01 | `flowy check` parses+checks, prints rustc-style errors, exits 0/1 | integration | `go test ./cmd/flowy/ -run TestCheck` | ❌ Wave 0 |
| CLI-02 | `flowy run --input` executes workflow, writes outputs.json | integration | `go test ./cmd/flowy/ -run TestRun_Workflow` | ❌ Wave 0 |
| CLI-03 | `--task` runs single task, skips workflow | integration | `go test ./cmd/flowy/ -run TestRun_Task` | ❌ Wave 0 |
| CLI-04 | `--path` extends import search, imports resolve | integration | `go test ./cmd/flowy/ -run TestCheck_WithPath` | ❌ Wave 0 |
| CLI-05 | `--dir` sets exec dir; refuses non-empty dir | unit | `go test ./cmd/flowy/ -run TestExecDir` | ❌ Wave 0 |
| CLI-06 | Exit 0 on success, non-zero on error | integration | covered by CLI-01/02 tests via `exec.Command` | ❌ Wave 0 |
| CLI-07 | `--verbose` prints stdout/stderr | integration | `go test ./cmd/flowy/ -run TestRun_Verbose` | ❌ Wave 0 |

**Note:** CLI integration tests should use `os/exec` to invoke the binary, or use a `main`-like function that accepts `io.Writer` and `[]string` args for testability without subprocess.

**Recommended testable main pattern:**
```go
// cmd/flowy/main.go
func run(args []string, stdout, stderr io.Writer) int { ... }
func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
// Tests call run() directly without subprocess
```

### Sampling Rate
- **Per task commit:** `go test ./cmd/flowy/... -count=1`
- **Per wave merge:** `go test ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `cmd/flowy/main_test.go` — covers CLI-01 through CLI-07
- [ ] `cmd/flowy/main.go` — the entry point itself
- [ ] `wdl/parser/parser_paths_test.go` — covers `ParseWithSearchPaths` (CLI-04)

## Sources

### Primary (HIGH confidence)
- Direct codebase inspection — `wdl/parser/parser.go`, `wdl/checker/checker.go`, `wdl/task/task.go`, `wdl/workflow/workflow.go`, `wdl/task/runner.go` (all read 2026-03-10)
- `go.mod` — confirmed module path `github.com/bioflowy/flowy`, Go 1.25, no CLI deps
- `.planning/phases/07-cli-and-integration/07-CONTEXT.md` — locked decisions verbatim

### Secondary (MEDIUM confidence)
- Go stdlib `flag` package documentation — standard subcommand pattern using `flag.NewFlagSet` is the idiomatic Go approach, confirmed by standard Go project layouts

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all packages confirmed by reading source; no new deps
- Architecture: HIGH — all entry-point function signatures confirmed from source
- Pitfalls: HIGH — `--path` gap and `--verbose` limitation confirmed by reading `parseFile` and `RunTask`
- Parser search path gap: HIGH — confirmed `parseFile` only uses `filepath.Dir(path)`
- Verbose limitation: HIGH — confirmed `RunTask` discards `RunResult` bytes after writing to disk

**Research date:** 2026-03-10
**Valid until:** 2026-04-10 (stable — no external deps, all internal code)
