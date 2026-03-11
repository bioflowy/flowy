---
phase: 08-add-spectool-integration-for-wdl-specification-compliance-testing
verified: 2026-03-11T04:00:00Z
status: human_needed
score: 8/8 must-haves verified
re_verification: false
human_verification:
  - test: "Run make spectool end-to-end"
    expected: "spectool installs (requires Rust/Cargo), WDL spec clones to /tmp/wdl-spec, flowy binary built, conformance tests execute showing pass/fail counts"
    why_human: "Requires Rust/Cargo toolchain and network access to GitHub; cannot run in automated verification environment"
  - test: "Verify excluded tests are skipped in spectool output"
    expected: "test_object, read_object_task, read_objects_task, write_object_task, write_objects_task, multiline_strings*, multiline_string_placeholders do not appear in failure list"
    why_human: "Depends on actual spectool run completing; cannot verify exclusion behavior without running the tool"
---

# Phase 8: Add spectool Integration for WDL Specification Compliance Testing

**Phase Goal:** Add spectool integration for WDL specification compliance testing
**Verified:** 2026-03-11T04:00:00Z
**Status:** human_needed (all automated checks passed; 2 items require human execution)
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | flowy run prints valid JSON to stdout on successful workflow execution | VERIFIED | TestRunWorkflow, TestRunWorkflowStdoutJSON, TestRunStdoutIsValidJSON all pass; `json.Valid([]byte(stdout))` asserted in tests |
| 2 | flowy run prints valid JSON to stdout on successful task execution with taskName.outputName keys | VERIFIED | TestRunTask, TestRunTaskStdoutJSON assert `say_hello.` key prefix on JSON stdout |
| 3 | flowy run --output <path> writes JSON to specified file instead of stdout | VERIFIED | TestRunWithOutputFlag: asserts stdout empty, output file exists with valid JSON; `outputFlag` at main.go:146,279-285 |
| 4 | File and Directory values in outputs JSON use absolute paths | VERIFIED | valueToJSONOutput returns val.Value directly (no relativization); TestWriteOutputsJSON_FileAbsolutePath and TestWriteOutputsJSON_DirectoryAbsolutePath pass |
| 5 | stdout contains ONLY JSON — no "Outputs written to" text, no verbose logs | VERIFIED | Grep confirms "Outputs written to" absent from main.go; TestRunWorkflowStdoutJSON asserts `!strings.Contains(stdout, "Outputs written to")` |
| 6 | execRoot/outputs.json is still written as before (both destinations) | VERIFIED | workflow path: RunWorkflow still calls writeOutputsJSON internally (workflow.go:112); task path: os.WriteFile at main.go:246 |
| 7 | make spectool builds flowy, installs spectool, clones WDL spec, and runs conformance tests | VERIFIED (automated: make build) | Makefile spectool target wired (lines 51-60); `make build` succeeds; `go build ./cmd/flowy` builds; spectool run requires human |
| 8 | Unsupported tests are excluded via spectool-exclude.txt | VERIFIED (file content) | spectool-exclude.txt has 10 patterns (5 Object type, 5 multiline string); Makefile reads patterns via grep/paste and passes to --exclude |

**Score:** 8/8 truths verified (2 require human confirmation for end-to-end execution)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `wdl/workflow/output.go` | SerializeOutputs() exported function, valueToJSONOutput using absolute paths | VERIFIED | 93 lines; SerializeOutputs at line 15; valueToJSONOutput returns val.Value directly for File/Directory |
| `cmd/flowy/main.go` | Updated runRun with --output flag, stdout JSON output, task output serialization | VERIFIED | outputFlag at line 146; workflow.SerializeOutputs called at lines 240, 266; stdout write at line 285 |
| `Makefile` | Build and spectool conformance test targets with spectool written | VERIFIED | spectool target at line 51; go build at line 17; cargo install at line 30; EXCLUDE_PATTERNS loaded from spectool-exclude.txt |
| `spectool-exclude.txt` | List of test patterns to exclude from conformance run | VERIFIED | 10 exclusion patterns in two categories (Object type: 5, multiline strings: 5) |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| cmd/flowy/main.go | wdl/workflow/output.go | workflow.SerializeOutputs() call | WIRED | Called at lines 240 and 266 in runRun; both task and workflow paths |
| cmd/flowy/main.go | stdout io.Writer | fmt.Fprintf(stdout, "%s\n", jsonBytes) | WIRED | Line 285; conditional on outputFlag == "" |
| Makefile | cmd/flowy/main.go | go build -o $(FLOWY_BIN) ./cmd/flowy | WIRED | Line 17; `make build` confirmed working |
| Makefile | spectool binary | cargo install from git clone | WIRED | Lines 27-30; vendored-openssl workaround applied |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| SPEC-01 | 08-01 | flowy run outputs JSON to stdout on success | SATISFIED | main.go line 285; TestRunWorkflowStdoutJSON passes |
| SPEC-02 | 08-01 | --output flag writes JSON to file, no stdout | SATISFIED | main.go lines 279-285; TestRunWithOutputFlag passes |
| SPEC-03 | 08-01 | File/Directory values use absolute paths in outputs JSON | SATISFIED | valueToJSONOutput returns val.Value directly; tests TestWriteOutputsJSON_FileAbsolutePath, TestSerializeOutputs_FileAbsolute pass |
| SPEC-04 | 08-01 | --task outputs use taskName.outputName dot-notation keys | SATISFIED | SerializeOutputs prefixes entityName; TestRunTask, TestRunTaskStdoutJSON assert "say_hello." prefix |
| SPEC-05 | 08-02 | make spectool runs conformance tests | SATISFIED (partial: automated build verified; human run needed) | Makefile spectool target exists with correct flags; make build succeeds |
| SPEC-06 | 08-02 | Unsupported tests excluded via spectool-exclude.txt | SATISFIED (file verified; human run needed) | spectool-exclude.txt has 10 patterns; Makefile loads via grep/paste into --exclude |

No orphaned requirements found. All 6 SPEC requirements are claimed by a plan and have implementation evidence.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| spectool-exclude.txt | 35 | "multiline_string_placeholders" in comment | Info | False positive — it's just a comment documenting test name, not an anti-pattern |

No blockers or warnings found. The only grep hit on "placeholder" in spectool-exclude.txt is a comment referencing the test name `multiline_string_placeholders` — this is correct documentation.

### Human Verification Required

#### 1. spectool End-to-End Run

**Test:** On a machine with Rust/Cargo installed, run `cd /home/uehara/flowy && make spectool`
**Expected:** spectool installs from GitHub (takes a few minutes on first run), WDL spec clones to /tmp/wdl-spec, flowy binary builds, conformance tests execute and report pass/fail counts. Expected baseline per SUMMARY: 45/160 passing, 10 excluded.
**Why human:** Requires Rust/Cargo toolchain and network access to GitHub. Cannot run in automated verification.

#### 2. Exclusion Pattern Effectiveness

**Test:** In spectool output from the above run, verify that excluded test names (test_object, read_object_task, read_objects_task, write_object_task, write_objects_task, multiline_strings1-4, multiline_string_placeholders) do not appear in the failure list.
**Why human:** Depends on spectool run completing; the exclusion mechanism uses comma-separated patterns via --exclude and correctness of pattern matching is spectool-internal behavior.

### Gaps Summary

No gaps. All must-haves from both plan frontmatters are satisfied:

- Plan 01 (SPEC-01 through SPEC-04): All 6 truths verified with passing tests and direct code evidence. Key link from main.go to SerializeOutputs is wired at two call sites. outputFlag conditional correctly routes JSON to file vs stdout.

- Plan 02 (SPEC-05 through SPEC-06): Makefile exists with correct spectool flags (--workflow-target-args, --task-target-args, --redirect-stdout, --all-capabilities, --exclude). spectool-exclude.txt has 10 exclusion patterns in two documented categories. The vendored-openssl workaround (sed patch of Cargo.toml) is present in the Makefile install step. All documented commits (0f4adfd, 1eb848a, 8e5a000, 6d69ad5, 06d4045, 1a5b019) exist in git history.

Full test suite: all 10 packages pass (go test ./... -count=1). Binary builds cleanly.

---

_Verified: 2026-03-11T04:00:00Z_
_Verifier: Claude (gsd-verifier)_
