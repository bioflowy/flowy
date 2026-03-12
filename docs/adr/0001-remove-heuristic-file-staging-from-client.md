# ADR 0001: Remove Heuristic File Staging from Client

## Status

Accepted

## Context

The flowy CLI client currently includes logic that scans JSON input values and heuristically determines whether string values represent file paths. When a string looks like a file path (contains a path separator or is an absolute path) and the file exists on disk, the client automatically copies it into the shared `base_dir` staging area (`{baseDir}/runs/{runID}/inputs/`). This heuristic approach has several problems:

- It is fragile and imprecise. A `String`-typed WDL input that happens to contain a path-like value (e.g., `"/usr/bin/python"`) could be incorrectly treated as a file to stage, while a `File`-typed input with an unusual name might be missed.
- It adds significant complexity to the client. The `stageInputFiles()` function recursively walks the JSON structure, applies heuristics, copies files, and rewrites paths — all of which must be maintained and tested.
- It creates an implicit contract that is difficult for users to reason about. Whether a file gets staged depends on runtime conditions (does the file exist? is it already under `base_dir`?), making behavior unpredictable.

At the same time, the spectool conformance test integration needs a way to ensure test data files are accessible to the flowy engine. The current approach uses a separate `-c ./conformance-tests` directory, requiring the client's heuristic staging to copy files into `base_dir` during test execution.

Two alternative approaches were considered:

1. **Type-information-based staging**: Parse WDL in the client to identify `File`/`Directory`-typed inputs and stage only those. This eliminates false positives but adds WDL parsing to the client, increasing its complexity.
2. **Wrapper script**: Remove staging from the client entirely and use a wrapper script for spectool that copies files and rewrites JSON paths before calling flowy. This merely moves the heuristic problem to a shell script.

## Decision

We will remove all heuristic file staging logic from the flowy client. Instead, we will adopt the convention that all file paths referenced in workflow inputs must already reside under `base_dir` (or be resolvable relative to the WDL file's parent directory, per WDL 1.2 spec rules for relative paths).

For the spectool integration, we will change the `-c` flag from `./conformance-tests` to `./flowy-data/spectool`. Since spectool extracts WDL files and their associated data resources (under `data/`) into the `-c` directory, placing this directory inside `base_dir` means all test files are already where flowy expects them — no copying or path rewriting is needed.

The WDL 1.2 specification itself supports this approach: relative paths in input and private declarations are resolved relative to the WDL file's parent directory. Since spectool places WDL files alongside their `data/` directory, references like `"data/greetings.txt"` resolve naturally.

For general usage, users are responsible for placing input data under `base_dir` before invoking flowy, or organizing files relative to their WDL documents. This is a clear, explicit contract that replaces the previous implicit heuristic behavior.

## Consequences

The client codebase becomes simpler. The `stageFile()`, `stageInputFiles()`, and related heuristic logic in `cmd/flowy/main.go` can be removed entirely. The client no longer needs to scan JSON values, check file existence, or copy files.

The spectool Makefile target requires a one-line change (`-c ./flowy-data/spectool` instead of `-c ./conformance-tests`). The `conformance-tests/` directory at the project root is no longer needed.

Users who previously relied on automatic file staging will need to adjust their workflow. They must either place data files under `base_dir` or alongside their WDL files. This is a breaking change for any existing users who depend on the implicit staging behavior.

The shared filesystem requirement between client, server, and daemon becomes more explicit. Rather than the client silently copying files to make them accessible, the user must ensure files are in the right place from the start. This makes the system's constraints more transparent and easier to debug when things go wrong.

The `flowy-data/` directory now serves a dual purpose during spectool runs: it contains both the spectool-extracted test files (under `spectool/`) and the runtime execution data (under `runs/`). The existing cleanup logic (which wipes `flowy-data/` before each spectool run) handles both correctly.
