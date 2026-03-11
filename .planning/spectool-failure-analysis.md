# Spectool Failure Analysis

**Date:** 2026-03-11
**Baseline:** 124/160 passed (36 failures, 1 skipped)

## Summary Table

| # | Test | Category | Root Cause |
|---|------|----------|------------|
| 1 | hello.wdl | **flowy bug** | `FileValue` retains relative path; PathMap stores absolute path; container path translation fails |
| 2 | struct_to_struct.wdl | **unsupported feature** | Struct-to-struct coercion not implemented in `Coerces()` |
| 3 | nested_access.wdl | **flowy bug** | Workflow input binding uses untyped JSON conversion — struct inputs become `MapValue` |
| 4 | file_directory_equality.wdl | **flowy bug** | `PathMap.AddFile/AddDir` don't canonicalize `..` or trailing slash before key lookup |
| 5 | flags_task.wdl | **flowy bug** | Same relative path → container translation failure as test 1 |
| 6 | test_placeholders_task.wdl | **flowy bug** | `read_lines(File)` in command template evaluated on host with container-translated path |
| 7 | input_hint_task.wdl | **flowy bug** | Task struct inputs deserialized as `MapValue` (no `StructType` case in `jsonToWDLValue`) |
| 8 | test_scatter.wdl | **flowy bug** | False cycle: scatter-local declarations create self-loop in DAG dependency tracking |
| 9 | test_allow_nested_inputs.wdl | **unsupported feature** | `allow_nested_inputs` workflow hint not implemented |
| 10 | read_string_task.wdl | **flowy bug** | Private decl evaluation missing `WorkDir` in `ExecContext`; `write_lines` temp file not container-accessible |

## Detailed Analysis

### 1. hello.wdl — "no output produced"

**Category: flowy bug**

When spectool runs with `"hello.infile": "data/greetings.txt"` (relative path), the workflow's `bindWorkflowInputs` converts the JSON string using `jsonToValueUntyped()` — which produces a `StringValue`, not a `FileValue`. The `StringValue` is passed through to the task call. Task `BindInputs` then creates a `FileValue("data/greetings.txt")`. In `collectPaths`, `pm.AddFile("data/greetings.txt")` resolves the relative path to an absolute path (e.g. `/tmp/spectool-tests/data/greetings.txt`) and stores it in the PathMap under that absolute key. However, the `FileValue` in the environment retains the original relative path string `"data/greetings.txt"`. When `translateEnv` calls `pm.TranslateValue(FileValue("data/greetings.txt"))`, the lookup in `pm.hostToContainer` fails because the key was stored as the absolute path, not the relative one. As a result, the path is not translated to `/mnt/inputs/greetings.txt`, and the container command contains `grep -E 'hello.*' 'data/greetings.txt'` which fails.

**Fix location:** `wdl/task/inputs.go`, in `jsonToWDLValue` for `FileType`/`DirectoryType`: resolve the path to absolute when creating `FileValue`, not just during `collectPaths`. Alternatively, canonicalize paths in `PathMap.AddFile` and also canonicalize on lookup in `TranslateValue`.

**Affects:** hello.wdl, hello_parallel.wdl, flags_task.wdl (test 5)

---

### 2. struct_to_struct.wdl — "no output produced"

**Category: unsupported feature**

The test assigns `D my_d = my_b` where `my_b` is of type `B` and `my_d` expects type `D`. According to WDL 1.2 spec, this coercion is legal if the structs have the same field names with compatible types (structural compatibility). `B` has fields `{a_struct: A, i: Int}` and `D` has `{a_struct: C, i: Int}`, where `A` and `C` are both `{s: String}`. The type checker in `wdl/checker/coerce.go` has no `StructType` case in the `Coerces()` function and therefore rejects the assignment as a type error.

**Error:** `cannot use B where D is expected`

---

### 3. nested_access.wdl — "no output produced"

**Category: flowy bug**

`bindWorkflowInputs` in `wdl/workflow/convert.go` uses `jsonToValueUntyped()` (line 115) regardless of the declared input type. This function converts JSON objects (`map[string]interface{}`) to `MapValue`, not `StructValue`. So the `Array[Experiment]+` input becomes an array of `MapValue` items. When the workflow output evaluates `first_experiment.variables[0]`, it calls `GetExpr` on a `MapValue`, which fails.

**Error:** `eval error: cannot access field "variables" on values.MapValue`

**Fix location:** `wdl/workflow/convert.go` — `bindWorkflowInputs` should use typed JSON conversion that considers the declared input type, converting JSON objects to `StructValue` when the expected type is a struct.

---

### 4. file_directory_equality.wdl — "boolean mismatch: expected true, got false"

**Category: flowy bug**

The spec test provides `file_a = "data/hello.txt"` and `file_b = "data/../data/hello.txt"` (different string representations of the same file), and `dir_a = "data/testdir/"` (trailing slash) vs `dir_b = "data/testdir"` (no slash).

In `wdl/task/pathmap.go`, `AddFile` only resolves relative paths to absolute (by checking `!filepath.IsAbs()`). Since both paths are resolved to absolute, the `../` is NOT cleaned out. `AddFile("/tmp/.../data/../data/hello.txt")` gets stored as a new key because `"/tmp/.../data/../data/hello.txt"` ≠ `"/tmp/.../data/hello.txt"`, causing a hash-prefixed collision container path. Similarly, `AddDir("testdir/")` vs `AddDir("testdir")` gives different container paths because the key normalization doesn't strip the trailing slash before the first lookup. The container command then compares different paths and returns `false`.

**Fix location:** `wdl/task/pathmap.go` — apply `filepath.Clean()` to all paths (not just relative ones) before using them as map keys in `AddFile` and `AddDir`.

---

### 5. flags_task.wdl — "number mismatch: expected 2, got 0"

**Category: flowy bug**

Same relative file path → container path translation failure as test 1 (hello.wdl). Spectool runs with `"flags.infile": "data/greetings.txt"` (relative). The task's `FileValue` retains the relative path, which doesn't match the absolute path stored in `PathMap`. The container receives the literal relative path, which doesn't exist inside the container.

**Root cause is shared with tests 1 and hello_parallel.wdl.**

---

### 6. test_placeholders_task.wdl — "no output produced"

**Category: flowy bug**

The command template contains `~{sep(" ", read_lines(infile))}`. `BuildCommand` in `wdl/task/command.go` evaluates the command template using `translatedEnv`, where `infile` has already been replaced with its container path `/mnt/inputs/greetings.txt`. When `eval.Eval` encounters `read_lines(infile)`, it calls `stdReadLines` on the host, attempting to open `/mnt/inputs/greetings.txt` — a path that only exists inside the Docker container, not on the host filesystem.

**Error:** `eval error: read_lines: cannot read file "/mnt/inputs/greetings.txt": open /mnt/inputs/greetings.txt: no such file or directory`

**Fix:** `read_lines(infile)` (and other file-reading stdlib functions used in command templates) should be evaluated with the host path (pre-translation), and only the resulting string values should be embedded into the command. The command builder needs a two-phase approach: evaluate expressions with host paths, then translate file references in the final command string.

---

### 7. input_hint_task.wdl — "no output produced"

**Category: flowy bug**

In `wdl/task/inputs.go`, `jsonToWDLValue()` has no case for `types.StructType`. It falls through to the `default` branch which calls `jsonToValueUntyped()`, converting the JSON object `{"name": "Joe"}` into a `MapValue`. When the command template evaluates `~{defined(person.cv)}`, `person` is a `MapValue` and `GetExpr(person, "cv")` fails.

**Error:** `eval error: cannot access field "cv" on values.MapValue`

**Note:** `BindInputs` in `inputs.go` doesn't receive `structDefs` as a parameter, so even if a `StructType` case were added, it would need access to struct definitions to resolve field types recursively.

**Related to test 3 (nested_access.wdl) — same underlying issue of untyped JSON deserialization.**

---

### 8. test_scatter.wdl — "no output produced"

**Category: flowy bug**

False cycle detection in workflow DAG when a scatter body contains a declaration that is referenced by a call within the same scatter block.

The scatter body contains:
```wdl
String greeting = "~{salutation} ~{name}"
call say_hello { greeting = greeting }
```

In `wdl/workflow/dag.go`, `collectBodyOutputNames` maps both the `greeting` declaration and `say_hello` call alias to the same owner ID `"scatter_1"`. This means `varToNodeID["greeting"] = "scatter_1"`. When `collectBodyDeps2` processes the call `say_hello`, it evaluates the input expression `IdentExpr("greeting")`. `collectExprDepsNode` finds `"greeting"` in `declVarToID`, returns `"scatter_1"` as a dependency — creating a self-loop: `scatter_1` depends on `scatter_1`.

**Error:** `topological sort: workflow dependency graph contains a cycle`

**Fix:** In `collectBodyDeps2`, filter out any dependency that maps to the current scatter/conditional node's own ID (self-dependencies within the same scatter/if block should not be considered DAG edges).

---

### 9. test_allow_nested_inputs.wdl — "string mismatch: expected 'Hello John', got 'Hello Joe'"

**Category: unsupported feature**

When the `allow_nested_inputs: true` workflow hint is set, inputs with fully-qualified names like `"test_allow_nested_inputs.nested.name": "John"` should override the default value of the task input `nested.name`. flowy has no implementation of `allow_nested_inputs` (no references to `allow_nested` in the codebase). The default value `"Joe"` is used instead of the provided `"John"`.

---

### 10. read_string_task.wdl — "no output produced"

**Category: flowy bug**

Private task declarations that call `write_*` functions don't have `WorkDir` set in `ExecContext`, causing temp files to be created in the OS temp directory instead of the task's work directory.

In `wdl/task/task.go` line 99:
```go
v, err := eval.Eval(decl.Expr, env, eval.ExecContext{StructDefs: sd})
```
`WorkDir` is not set. `write_lines(["this","file","has","five","lines"])` calls `writeTempDir("wdl-write-*", ctx)` which falls back to `os.MkdirTemp("", ...)` creating `/tmp/wdl-write-*/lines.txt`. This temp directory is not bind-mounted into the container. The command becomes `cat /tmp/wdl-write-971827486/lines.txt`, which fails inside the container.

**Error:** `task "read_string" failed with exit code 1: cat: /tmp/wdl-write-.../lines.txt: No such file or directory`

**Fix:** Change line 99 in `wdl/task/task.go` to include `WorkDir: workDir` in the `ExecContext`. This would cause `write_lines` to create files in `workDir` (which is mounted as `/mnt/outputs`), and the existing path substitution logic in `BuildCommand` would correctly translate the path for the container.

---

## Failure Grouping by Root Cause

### Group A: Relative path → container path translation (3 tests)
- hello.wdl, hello_parallel.wdl, flags_task.wdl
- `FileValue` retains relative path string, but `PathMap` stores absolute path as key

### Group B: Untyped JSON deserialization of struct inputs (3 tests)
- nested_access.wdl, input_hint_task.wdl (+ likely others)
- `jsonToValueUntyped()` creates `MapValue` instead of `StructValue` for struct-typed inputs

### Group C: Command template evaluation with container paths (1 test)
- test_placeholders_task.wdl
- File-reading stdlib functions in command templates fail because paths are already container-translated

### Group D: False cycle in DAG (1 test)
- test_scatter.wdl
- Scatter-internal declaration referenced by scatter-internal call creates false self-dependency

### Group E: PathMap path canonicalization (1 test)
- file_directory_equality.wdl
- Missing `filepath.Clean()` and trailing slash normalization

### Group F: Missing WorkDir in private decl evaluation (1 test)
- read_string_task.wdl
- `write_*` temp files created outside container-mounted directories

### Group G: Unsupported features (2 tests)
- struct_to_struct.wdl — struct-to-struct coercion
- test_allow_nested_inputs.wdl — `allow_nested_inputs` hint
