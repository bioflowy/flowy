---
phase: 04-expression-evaluator-and-standard-library
plan: "03"
subsystem: wdl/eval
tags: [command-evaluation, string-interpolation, dedent-algorithm, placeholder-substitution]
dependency_graph:
  requires: [04-01]
  provides: [CommandExpr evaluation, StringExpr evaluation, evalCommand, evalString, valueToString, computeMinIndent, applyDedentAndSubstitute]
  affects: [Phase 5 task execution — generates shell command strings from CommandExpr]
tech_stack:
  added: []
  patterns:
    - "Sentinel-before-indent: placeholders replaced with ~{} marker BEFORE computing min-indent, ensuring indent calculation uses fixed-width sentinels not expanded values"
    - "Two-phase evalCommand: sentinel-build + dedent/substitute separated to enforce correct ordering"
key_files:
  created:
    - wdl/eval/command.go
  modified:
    - wdl/eval/eval.go
    - wdl/eval/eval_test.go
    - wdl/eval/command_test.go
decisions:
  - "partOpts slice removed from evalCommand: options already applied in valueToString(v, p.Options) before sentinel substitution — no need to store separately"
  - "TestCommandDedent duplicate removed from eval_test.go: moved to command_test.go as the canonical location for dedent algorithm tests"
metrics:
  duration: "2min"
  completed_date: "2026-03-10"
  tasks_completed: 2
  files_modified: 4
---

# Phase 04 Plan 03: Command Template Expansion Summary

**One-liner:** Sentinel-before-indent command dedent with full placeholder option support (sep/default/true/false) via evalCommand and evalString.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Implement command dedent algorithm in command.go | f119194 | wdl/eval/command.go (created), wdl/eval/command_test.go |
| 2 | Implement evalCommand and evalString; wire into Eval() | c3d70d5 | wdl/eval/command.go, wdl/eval/eval.go, wdl/eval/eval_test.go |

## What Was Built

### wdl/eval/command.go (new file)

**`computeMinIndent(s string) int`**
- Splits input by `\n`, skips empty and whitespace-only lines
- Counts leading space/tab characters on remaining lines
- Returns the minimum; returns 0 if no non-empty lines

**`applyDedentAndSubstitute(sentinel string, minIndent int, parts []string, marker string) string`**
- Strips `minIndent` leading characters from each non-empty line
- Whitespace-only lines are preserved as-is (not stripped)
- Replaces each `~{}` marker with the next element from `parts` in left-to-right order

**`valueToString(v values.Value, opts map[string]string) string`**
- NullValue: returns `opts["default"]` if present, else `""`
- BooleanValue: returns `opts["true"]`/`opts["false"]` if present, else `"true"`/`"false"`
- IntValue: `strconv.FormatInt`
- FloatValue: `strconv.FormatFloat` with `'g'` format
- StringValue, FileValue, DirectoryValue: returns `.Value` directly
- ArrayValue: joins items with `opts["sep"]` (default `" "`) using recursive `valueToString(item, nil)`
- Other compound types: `fmt.Sprintf("%v", v)` fallback

**`evalString(parts []ast.StringPart, env *Env, ctx ExecContext) (string, error)`**
- Direct placeholder substitution for StringExpr — no dedent step

**`evalCommand(e *ast.CommandExpr, env *Env, ctx ExecContext) (values.Value, error)`**
- Phase 1: build sentinel string (write `~{}` for each PlaceholderPart, evaluate expr, stringify with options)
- Phase 2: compute min-indent on sentinel string
- Phase 3: apply dedent then substitute sentinels with stringified values
- Returns `values.StringValue`

### wdl/eval/eval.go (modified)

- `StringExpr` case: calls `evalString()`, wraps result in `StringValue`
- `CommandExpr` case: delegates to `evalCommand()`
- Removed `"not yet implemented"` placeholder errors for both cases

## Test Coverage

- `TestCommandDedent`: 10 subtests covering `computeMinIndent` and `applyDedentAndSubstitute`
- `TestCommandDedentEdgeCases`: 13 subtests covering `valueToString` for all value types and option combinations
- `TestEvalCommand`: 5 subtests covering full `Eval()` path for StringExpr and CommandExpr

Total: 28 new passing tests.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Removed unused `partOpts` variable from `evalCommand`**
- **Found during:** Task 2 implementation
- **Issue:** `partOpts` slice was collected but never read — options were already applied in `valueToString(v, p.Options)` before the sentinel phase
- **Fix:** Removed the `partOpts` variable and the append to it
- **Files modified:** wdl/eval/command.go
- **Commit:** c3d70d5

**2. [Rule 1 - Bug] Removed duplicate `TestCommandDedent` declaration from eval_test.go**
- **Found during:** Task 1 RED phase
- **Issue:** eval_test.go had a stub `TestCommandDedent` which conflicted with the new implementation in command_test.go (Go does not allow two functions with the same name in the same package)
- **Fix:** Removed the duplicate from eval_test.go, kept only in command_test.go
- **Files modified:** wdl/eval/eval_test.go
- **Commit:** f119194

## Self-Check: PASSED

Files created/modified:
- wdl/eval/command.go: FOUND
- wdl/eval/eval.go: FOUND
- wdl/eval/eval_test.go: FOUND
- wdl/eval/command_test.go: FOUND

Commits:
- f119194: FOUND
- c3d70d5: FOUND
