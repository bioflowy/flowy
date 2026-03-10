---
phase: 04-expression-evaluator-and-standard-library
plan: "04"
subsystem: wdl/eval
tags: [go, stdlib, file-io, filesystem, tdd, wdl]

dependency_graph:
  requires:
    - phase: 04-expression-evaluator-and-standard-library
      plan: "02"
      provides: "Eval() with evalApply dispatch into stdlibFuncs map"
    - phase: 04-expression-evaluator-and-standard-library
      plan: "03"
      provides: "evalCommand and evalString; ExecContext with WorkDir/StdoutPath/StderrPath"
  provides:
    - "18 STDLIB-01/STDLIB-02 functions registered in stdlibFuncs via init()"
    - "read_lines/read_tsv/read_map/read_json: file -> WDL value parsing"
    - "read_boolean/read_int/read_float/read_string: scalar file parsing"
    - "write_lines/write_tsv/write_map/write_json: WDL value -> temp file"
    - "glob/basename/dirname/size/stdout/stderr: filesystem query functions"
    - "jsonToValue/valueToJson helpers for recursive JSON conversion"
  affects:
    - "Phase 4 plans 04-05 (array/string stdlib) and 04-06 (numeric stdlib)"
    - "Phase 5 task executor: stdout()/stderr()/glob() used after task completes"

tech_stack:
  added: []
  patterns:
    - "init() registration pattern: stdlib_fileio.go init() populates stdlibFuncs map at package load"
    - "fileArg() helper accepts FileValue or StringValue for File-typed function arguments"
    - "jsonToValue: float64 whole-number -> IntValue, fractional -> FloatValue"
    - "write_* functions: os.MkdirTemp('', 'wdl-write-*') then os.WriteFile per locked decision"
    - "ExecContext guard pattern: WorkDir/StdoutPath/StderrPath empty -> EvalError immediately"
    - "sizeUnits map: string unit -> float64 divisor for byte->unit conversion"

key_files:
  created:
    - wdl/eval/stdlib_fileio.go
  modified:
    - wdl/eval/stdlib_test.go

key_decisions:
  - "Both tasks implemented in single TDD cycle: tests and implementation committed together since they cover the same file (stdlib_fileio.go)"
  - "read_string uses strings.TrimSpace (not just TrimRight newline) matching spec intent for scalar reads"
  - "jsonToValue integer detection: math.Trunc(x)==x guards whole-number float64 -> IntValue promotion"

metrics:
  duration: "5min"
  completed_date: "2026-03-10"
  tasks_completed: 2
  files_modified: 2
---

# Phase 04 Plan 04: File I/O and Filesystem Stdlib Summary

**One-liner:** 18 WDL stdlib functions (STDLIB-01 read/write + STDLIB-02 filesystem) registered via init() in stdlib_fileio.go, with real temp-file I/O and ExecContext guard patterns.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Implement STDLIB-01 read functions (TDD RED+GREEN) | 10b1d50 | wdl/eval/stdlib_fileio.go (created), wdl/eval/stdlib_test.go |
| 2 | Implement STDLIB-01 write and STDLIB-02 filesystem functions | 10b1d50 | wdl/eval/stdlib_fileio.go, wdl/eval/stdlib_test.go |

## What Was Built

### wdl/eval/stdlib_fileio.go (new file)

**Registration (init())**
- Registers all 18 functions into `stdlibFuncs` at package initialization

**STDLIB-01 Read functions**
- `stdReadLines`: os.ReadFile -> TrimRight("\n") -> Split("\n") -> ArrayValue[StringValue]
- `stdReadTsv`: os.ReadFile -> split lines -> Split("\t") -> ArrayValue[ArrayValue[StringValue]]
- `stdReadMap`: os.ReadFile -> split lines -> SplitN("\t", 2) -> MapValue[String, String]
- `stdReadJson`: os.ReadFile -> json.Unmarshal -> jsonToValue recursive converter
- `stdReadBoolean`: os.ReadFile -> TrimSpace -> strconv.ParseBool -> BooleanValue
- `stdReadInt`: os.ReadFile -> TrimSpace -> strconv.ParseInt -> IntValue
- `stdReadFloat`: os.ReadFile -> TrimSpace -> strconv.ParseFloat -> FloatValue
- `stdReadString`: os.ReadFile -> TrimSpace -> StringValue

**STDLIB-01 Write functions (all use os.MkdirTemp per locked decision)**
- `stdWriteLines`: builds "line\n" per item -> writes to lines.txt
- `stdWriteTsv`: joins fields with "\t", rows with "\n" -> writes to tsv.tsv
- `stdWriteMap`: writes "key\tvalue\n" per entry -> writes to map.tsv
- `stdWriteJson`: valueToJson -> json.Marshal -> writes to data.json

**JSON helpers**
- `jsonToValue(interface{}) values.Value`: bool->BooleanValue, float64 whole->IntValue/frac->FloatValue, string->StringValue, []interface{}->ArrayValue, map[string]interface{}->MapValue, nil->NullValue
- `valueToJson(values.Value) interface{}`: inverse direction for write_json

**STDLIB-02 Filesystem functions**
- `stdGlob`: guards WorkDir=="" -> EvalError; filepath.Glob(WorkDir+"/"+pattern) -> ArrayValue[FileValue]
- `stdBasename`: filepath.Base(path) -> StringValue; accepts FileValue or StringValue
- `stdDirname`: filepath.Dir(path) -> StringValue; accepts FileValue or StringValue
- `stdSize`: os.Stat(path).Size() -> float64 / unit factor -> FloatValue; 9 units supported
- `stdStdout`: guards StdoutPath=="" -> EvalError; returns FileValue{StdoutPath}
- `stdStderr`: guards StderrPath=="" -> EvalError; returns FileValue{StderrPath}

### wdl/eval/stdlib_test.go (modified)

- `TestStdlibFileIO`: 20 subtests covering all 12 STDLIB-01 functions with real temp files
- `TestStdlibFilesystem`: 11 subtests covering all 6 STDLIB-02 functions including error cases
- Helper `writeFile()` creates temp files using t.TempDir() per locked decision
- Helper `callStdlib()` dispatches directly into stdlibFuncs map

## Test Coverage

| Function | Test Cases |
|----------|-----------|
| read_string | 2 (with/without trailing newline) |
| read_int | 1 |
| read_float | 1 |
| read_boolean | 2 (true/false) |
| read_lines | 2 (with/without trailing newline) |
| read_tsv | 1 (multi-row, multi-column) |
| read_map | 1 (two entries) |
| read_json | 6 (object, array, string, boolean, integer, float) |
| write_lines | 1 |
| write_tsv | 1 |
| write_map | 1 |
| write_json | 1 |
| glob | 2 (matches, empty WorkDir error) |
| basename | 2 (FileValue, StringValue) |
| dirname | 1 |
| size | 2 (bytes default, KiB unit) |
| stdout | 2 (success, empty path error) |
| stderr | 2 (success, empty path error) |

**Total: 31 new passing tests**

## Deviations from Plan

None - plan executed exactly as written. Both tasks were implemented in a single TDD cycle since they cover the same file (stdlib_fileio.go). Tests were written first (RED), then the full implementation (GREEN), verified in one pass.

## Self-Check: PASSED

Files created/modified:
- wdl/eval/stdlib_fileio.go: FOUND
- wdl/eval/stdlib_test.go: FOUND

Commits:
- 10b1d50: FOUND
