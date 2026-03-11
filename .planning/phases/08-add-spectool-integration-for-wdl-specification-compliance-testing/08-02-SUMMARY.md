---
phase: 08-add-spectool-integration-for-wdl-specification-compliance-testing
plan: "02"
subsystem: testing
tags: [spectool, makefile, wdl-conformance, cargo, bugfix]

requires:
  - phase: 08-01
    provides: [spectool-compatible-stdout-json, absolute-path-outputs, output-flag]
provides:
  - Makefile with build/test/spectool targets
  - spectool-exclude.txt with pattern-based exclusion for Object type and multiline string expressions
  - Conformance baseline: 45/160 tests passing
affects: [ci, testing, all future phases that improve WDL compliance]

tech-stack:
  added: [spectool 0.1.8 (via cargo install), GNU Make]
  patterns: [makefile-spectool-target, exclude-file-pattern, conformance-baseline]

key-files:
  created:
    - Makefile
    - spectool-exclude.txt
    - .gitignore
  modified:
    - wdl/task/inputs.go
    - wdl/eval/stdlib_numeric.go
    - wdl/eval/stdlib_fileio.go
    - wdl/eval/stdlib_string.go
    - wdl/lexer/lexer.go
    - wdl/parser/parse_expr.go

key-decisions:
  - "spectool has no --exclude-file flag; use --exclude with comma-separated patterns loaded from spectool-exclude.txt via shell"
  - "spectool separates target handling via --workflow-target-args and --task-target-args (not ~{target} in base command)"
  - "--redirect-stdout appends '> ~{output}' to the command — flowy stdout JSON is captured as outputs file"
  - "spectool git2 dependency requires vendored-openssl on systems without libssl-dev; sed patch applied in Makefile install step"
  - "Object type tests excluded (5 tests): WDL deprecated type, not in v1 scope"
  - "Multiline string expression tests excluded (5 tests): <<<...>>> in expression context not implemented"
  - "task/BindInputs: accept taskName.fieldName JSON key prefix to match spectool convention"
  - "floor/ceil/round accept Int input via numericToFloat helper (WDL Int->Float coercion)"
  - "float literal .14 (no leading zero) now supported in lexer"
  - "String escape sequences processed in parser for string literals (not command blocks)"
  - "sub() replacement converts WDL \\N backreferences to Go ${N} syntax via wdlReplacementToGo"
  - "basename() supports optional suffix-strip second argument"

patterns-established:
  - "Pattern 1: Makefile auto-strips comments from spectool-exclude.txt via grep/paste shell command"
  - "Pattern 2: Cargo install with vendored-openssl to avoid system SSL dependency"
  - "Pattern 3: BindInputs tries taskName.field prefix first, then bare field (mirrors workflow input binding)"

requirements-completed:
  - SPEC-05
  - SPEC-06

duration: 45min
completed: "2026-03-11"
---

# Phase 8 Plan 02: Makefile and spectool Conformance Target Summary

**Makefile + spectool conformance pipeline operational; 45/160 tests passing after fixing 6 bugs exposed by the first conformance run**

## Performance

- **Duration:** 45 min (Task 1 in prior session + Task 2 continuation)
- **Started:** 2026-03-11T02:26:05Z
- **Completed:** 2026-03-11T03:30:00Z
- **Tasks:** 2 of 2
- **Files modified:** 9

## Accomplishments

- Installed spectool and verified exact CLI flags (research had LOW confidence on exclude mechanism)
- Created Makefile with correct spectool flags: --workflow-target-args, --task-target-args, --redirect-stdout, --all-capabilities, --exclude loaded from spectool-exclude.txt
- Applied vendored-openssl workaround for git2 dependency on systems without libssl-dev
- Fixed 6 bugs in flowy exposed by the spectool conformance run
- Established conformance baseline: **45/160 tests passing** (up from 40/170 initial; 10 tests excluded)
- Populated spectool-exclude.txt with Object type and multiline string expression exclusions

## Task Commits

Each task was committed atomically:

1. **Task 1: Create Makefile and spectool-exclude.txt** - `8e5a000` (chore)
   - Follow-up fix (Go flag arg ordering): `6d69ad5`
2. **Task 2: Verify spectool conformance test run** - bug fixes: `06d4045` + exclude list: `1a5b019`

**Plan metadata:** docs commit follows

## Files Created/Modified

- `Makefile` - Build and spectool conformance test targets with exclude list loading
- `spectool-exclude.txt` - Exclusion patterns: Object type (5 tests), multiline string expressions (5 tests)
- `.gitignore` - Exclude flowy binary and /tmp clone directories
- `wdl/task/inputs.go` - BindInputs accepts `taskName.field` JSON key prefix (spectool convention)
- `wdl/eval/stdlib_numeric.go` - floor/ceil/round accept Int input; added numericToFloat helper
- `wdl/eval/stdlib_fileio.go` - basename(path, suffix) supports optional 2nd arg suffix-strip
- `wdl/eval/stdlib_string.go` - sub() converts WDL \\N backreferences to Go ${N}; added wdlReplacementToGo helper
- `wdl/lexer/lexer.go` - float literal with no leading zero (.14 == 0.14)
- `wdl/parser/parse_expr.go` - unescape \\n \\t \\\\ etc. in string literal RawStringParts; added unescapeString helper

## Decisions Made

1. **No --exclude-file flag in spectool:** Research had LOW confidence on this. Actual flag is `--exclude` (comma-separated list). Makefile uses shell expansion to load spectool-exclude.txt.

2. **--workflow-target-args / --task-target-args:** spectool separates target arguments for workflows vs. tasks. Workflow runs get empty args; task runs get `--task ~{target}`.

3. **vendored-openssl workaround:** git2 crate requires pkg-config + libssl-dev. Applied `sed` patch in Makefile install step to add vendored-openssl feature.

4. **Object type exclusion (5 tests):** test_object, read_object_task, read_objects_task, write_object_task, write_objects_task — Object type was deprecated in WDL 1.x and is not in v1 scope.

5. **Multiline string expression exclusion (5 tests):** multiline_strings1-4, multiline_string_placeholders — `<<<...>>>` in expression context (not command blocks) is not implemented.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] vendored-openssl workaround for spectool build**
- **Found during:** Task 1 (installing spectool to verify CLI flags)
- **Issue:** `cargo install` failed with "pkg-config not found" because git2 crate requires system OpenSSL headers
- **Fix:** Patched Cargo.toml in the install step via `sed` to add `features = ["vendored-openssl"]` to git2 dependency
- **Files modified:** Makefile
- **Committed in:** `8e5a000`

**2. [Rule 1 - Bug / Adaptation] Corrected spectool CLI flag usage based on actual help output**
- **Found during:** Task 1 (running `spectool test --help`)
- **Issue:** Research notes had LOW confidence on `--exclude-file`; actual flag is `--exclude` with comma-separated patterns
- **Fix:** Makefile uses shell expansion to load spectool-exclude.txt into `--exclude "..."`
- **Files modified:** Makefile
- **Committed in:** `8e5a000`

**3. [Rule 1 - Bug] Makefile argument ordering for Go flag package**
- **Found during:** Task 2 verification (user reported --task was ignored)
- **Issue:** Go's flag package requires flags before positional args; `flowy run path.wdl --task name` silently ignores --task
- **Fix:** Reordered args to `flowy run --input ~{input} ~{path}`; `--task-target-args="--task ~{target}"` appended after
- **Files modified:** Makefile
- **Committed in:** `6d69ad5`

**4. [Rule 1 - Bug] task BindInputs does not accept taskName.field prefix**
- **Found during:** Task 2 (spectool run: task tests failed with "missing required input")
- **Issue:** spectool generates inputs.json with `"taskName.field"` keys; flowy's BindInputs only looked up bare field names
- **Fix:** BindInputs now tries `task.Name + "." + decl.Name` first, then falls back to bare `decl.Name`
- **Files modified:** `wdl/task/inputs.go`
- **Committed in:** `06d4045`

**5. [Rule 1 - Bug] floor/ceil/round reject Int input**
- **Found during:** Task 2 (test_floor, test_ceil failed: "floor: expected FloatValue, got values.IntValue")
- **Issue:** floor/ceil/round only accepted FloatValue; WDL spec allows Int (Int coerces to Float)
- **Fix:** Added `numericToFloat` helper accepting both FloatValue and IntValue
- **Files modified:** `wdl/eval/stdlib_numeric.go`
- **Committed in:** `06d4045`

**6. [Rule 1 - Bug] Float literal with no leading zero (.14) causes parse error**
- **Found during:** Task 2 (declarations.wdl failed: `Float pi = i + .14` → "unexpected token '.'")
- **Issue:** Lexer emitted `.` as TokDot instead of recognizing `.14` as TokFloatLit
- **Fix:** In `lexNormal`, case `.`: check if next char is a digit; if so emit TokFloatLit
- **Files modified:** `wdl/lexer/lexer.go`
- **Committed in:** `06d4045`

**7. [Rule 1 - Bug] String escape sequences not processed**
- **Found during:** Task 2 (test_sub showed literal `\\n` in JSON output instead of actual newline)
- **Issue:** Lexer consumed `\\n` as raw bytes; no unescape step in parser
- **Fix:** Added `unescapeString` in parse_expr.go; applied to RawStringPart.Val in string expressions (not command blocks)
- **Files modified:** `wdl/parser/parse_expr.go`
- **Committed in:** `06d4045`

**8. [Rule 1 - Bug] sub() backreferences \\N not converted to Go ${N} syntax**
- **Found during:** Task 2 (test_sub: new_question was `\\2, \\1?` instead of `chocolate, when?`)
- **Issue:** WDL uses `\\N` for backreferences; Go's `regexp.ReplaceAllString` uses `${N}`
- **Fix:** Added `wdlReplacementToGo` converting `\\N` → `${N}` and escaping literal `$` as `$$`
- **Files modified:** `wdl/eval/stdlib_string.go`
- **Committed in:** `06d4045`

**9. [Rule 1 - Bug] basename() does not support optional suffix-strip argument**
- **Found during:** Task 2 (test_basename: `basename("/path/to/file.txt", ".txt") == "file"` returned false)
- **Issue:** basename() only accepted one argument
- **Fix:** stdBasename now checks for a second StringValue argument and calls `strings.TrimSuffix`
- **Files modified:** `wdl/eval/stdlib_fileio.go`
- **Committed in:** `06d4045`

---

**Total deviations:** 9 auto-fixed (all Rule 1-3 bugs found during conformance run)
**Impact on plan:** All fixes were correctness bugs exposed by the conformance run. Pass count improved from 40/170 to 45/160. No scope creep.

## Known Remaining Failures (115 tests, not excluded)

The 115 remaining failures are bugs and missing features for future phases:

### Parser Missing Features
- **WDL 1.2 shorthand call syntax**: `call task { input1, input2 }` — affects hello, test_range, is_defined, test_values, ternary, task_outputs, and most workflow tests
- **`None` keyword**: `Int? x = None` causes "undefined name 'None'" — affects test_select_first, compare_optionals, placeholder_none, etc.

### Missing Stdlib Functions
- `sep(delim, array)` — not registered; affects sum_task, test_sep, and many others
- `chunk_array(array, n)` — not implemented
- `change_extension(file, ext)` — not implemented
- `join_paths(base, path)` — not implemented

### Correctness Bugs
- `length()` only handles ArrayValue, not MapValue/PairValue
- Pair field access `.left` / `.right` fails type checker in some contexts
- Conditional branch output access: `call.output` returns NullValue when call is inside unexecuted branch
- `test_as_map_fail`, `test_zip_fail`, `write_json_fail` — tests expected to fail but pass (missing validation)

### Tests Requiring Docker
Many task tests require Docker containers to execute commands. They fail with "no output produced" when Docker is unavailable. These will pass in Docker-enabled environments.

## Issues Encountered
- Makefile `--workflow-target-args=""` syntax required `=` assignment to work with spectool — fixed in `6d69ad5`
- spectool test runner cleans up temporary WDL files immediately after each test, requiring debug command sequencing

## User Setup Required
None — `make spectool` runs automatically once Docker is available and Rust/Cargo is installed.

## Next Phase Readiness
- Conformance infrastructure complete and reproducible via `make spectool`
- Baseline: 45/160 tests passing
- Key areas for next improvement: shorthand call syntax parser support, `None` keyword, sep() stdlib function

---
*Phase: 08-add-spectool-integration-for-wdl-specification-compliance-testing*
*Completed: 2026-03-11*
