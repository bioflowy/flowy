---
phase: quick-5
plan: 1
type: execute
wave: 1
depends_on: []
files_modified:
  - wdl/task/outputs_test.go
autonomous: true
requirements: [QUICK-5]

must_haves:
  truths:
    - "coerceOutputValue resolves relative paths against workDir for File outputs"
    - "coerceOutputValue resolves relative paths against workDir for Directory outputs"
    - "coerceOutputValue leaves absolute paths unchanged"
    - "coerceOutputValue returns non-string values unchanged"
  artifacts:
    - path: "wdl/task/outputs_test.go"
      provides: "Regression tests for coerceOutputValue relative path fix"
      min_lines: 40
  key_links:
    - from: "wdl/task/outputs_test.go"
      to: "wdl/task/outputs.go"
      via: "direct function call"
      pattern: "coerceOutputValue"
---

<objective>
Add regression tests for the relative path coercion fix in task File/Directory outputs.

Purpose: The bug where relative paths in File/Directory task outputs were not resolved against workDir has been fixed in outputs.go. This plan adds test coverage to prevent regression.
Output: wdl/task/outputs_test.go with comprehensive tests for coerceOutputValue.
</objective>

<execution_context>
@/home/uehara/.claude/get-shit-done/workflows/execute-plan.md
@/home/uehara/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@wdl/task/outputs.go
@wdl/task/inputs_test.go (for test patterns in task package)
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Add regression tests for coerceOutputValue</name>
  <files>wdl/task/outputs_test.go</files>
  <behavior>
    - Relative path + File type: coerceOutputValue(StringValue{"output.txt"}, FileType{}, "/work") returns FileValue with Value="/work/output.txt"
    - Relative path + Directory type: coerceOutputValue(StringValue{"results"}, DirectoryType{}, "/work") returns DirectoryValue with Value="/work/results"
    - Absolute path + File type: coerceOutputValue(StringValue{"/abs/file.txt"}, FileType{}, "/work") returns FileValue with Value="/abs/file.txt" (unchanged)
    - Absolute path + Directory type: coerceOutputValue(StringValue{"/abs/dir"}, DirectoryType{}, "/work") returns DirectoryValue with Value="/abs/dir" (unchanged)
    - String type (no coercion): coerceOutputValue(StringValue{"hello"}, StringType{}, "/work") returns original StringValue
    - Non-string value: coerceOutputValue(IntValue{42}, FileType{}, "/work") returns original IntValue unchanged
    - Empty workDir with relative path: coerceOutputValue(StringValue{"output.txt"}, FileType{}, "") returns FileValue with relative path "output.txt"
  </behavior>
  <action>
Create wdl/task/outputs_test.go in the task_test package (external test package, matching existing test file conventions in this package). Use table-driven tests with subtests.

Import: "testing", types, values packages from github.com/bioflowy/flowy/wdl/.

Note: coerceOutputValue is unexported, so use package `task` (not `task_test`) for the test file to access the unexported function directly. Check existing test files first -- if they use `package task_test` with exported wrappers, follow that pattern; if any use `package task`, follow that.
  </action>
  <verify>
    <automated>cd /home/uehara/flowy && go test ./wdl/task/ -run TestCoerceOutputValue -v</automated>
  </verify>
  <done>All 7 test cases pass, covering relative path resolution, absolute path passthrough, non-File/Directory type passthrough, non-string value passthrough, and empty workDir edge case.</done>
</task>

</tasks>

<verification>
cd /home/uehara/flowy && go test ./wdl/task/ -v
</verification>

<success_criteria>
- outputs_test.go exists with table-driven tests for coerceOutputValue
- All tests pass including the relative path resolution cases
- Full task package test suite still passes
</success_criteria>

<output>
After completion, create `.planning/quick/5-fix-relative-path-coercion-in-task-file-/5-SUMMARY.md`
</output>
