---
phase: quick-4
plan: 1
type: execute
wave: 1
depends_on: []
files_modified:
  - wdl/workflow/convert.go
  - wdl/workflow/convert_test.go
autonomous: true
requirements: [QUICK-4]
must_haves:
  truths:
    - "bindWorkflowInputs resolves fully-qualified JSON keys like 'hello.name'"
    - "bindWorkflowInputs still resolves unqualified keys like 'name' as fallback"
    - "All existing tests continue to pass"
  artifacts:
    - path: "wdl/workflow/convert.go"
      provides: "Fully-qualified key lookup in bindWorkflowInputs"
      contains: "jsonInputs[wf.Name+\".\"+decl.Name]"
    - path: "wdl/workflow/convert_test.go"
      provides: "Unit tests for bindWorkflowInputs key resolution"
      contains: "TestBindWorkflowInputs"
  key_links:
    - from: "wdl/workflow/convert.go"
      to: "JSON input map"
      via: "fully-qualified key lookup with unqualified fallback"
      pattern: "wf\\.Name.*decl\\.Name"
---

<objective>
Add unit tests for the bindWorkflowInputs fix that supports fully-qualified JSON keys.

Purpose: The bug fix (fully-qualified key lookup in bindWorkflowInputs) is already applied and working. This plan adds regression tests to ensure it stays correct.
Output: Unit tests in convert_test.go covering fully-qualified keys, unqualified fallback, and mixed scenarios.
</objective>

<execution_context>
@/home/uehara/.claude/get-shit-done/workflows/execute-plan.md
@/home/uehara/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@wdl/workflow/convert.go
@wdl/workflow/convert_test.go
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Add unit tests for bindWorkflowInputs fully-qualified key support</name>
  <files>wdl/workflow/convert_test.go</files>
  <behavior>
    - TestBindWorkflowInputs_FullyQualified: workflow "hello" with input "name", JSON key "hello.name" -> resolves correctly
    - TestBindWorkflowInputs_UnqualifiedFallback: workflow "hello" with input "name", JSON key "name" -> resolves correctly (backward compat)
    - TestBindWorkflowInputs_FullyQualifiedTakesPrecedence: both "hello.name" and "name" in JSON -> fully-qualified value wins
    - TestBindWorkflowInputs_DefaultUsedWhenMissing: no JSON key for input with default expr -> default used
  </behavior>
  <action>
Add test functions to wdl/workflow/convert_test.go that exercise bindWorkflowInputs directly.
Each test should:
1. Construct a minimal ast.Workflow with Name set and Inputs containing declarations
2. Build a jsonInputs map with the appropriate key format
3. Call bindWorkflowInputs(wf, jsonInputs)
4. Assert the returned env contains the expected value via env.Get(declName)

For the precedence test, provide both "hello.name" = "qualified" and "name" = "unqualified" in the JSON map, assert the env gets "qualified".

Use ast.Decl with Name, Type (types.StringType{}), and optionally Expr for the default-value test case. For the default expression test, use a simple ast.StringExpr literal.

Note: bindWorkflowInputs is unexported but the test file is in package workflow (internal tests), so direct access is fine.
  </action>
  <verify>
    <automated>cd /home/uehara/flowy && go test ./wdl/workflow/ -run TestBindWorkflowInputs -v</automated>
  </verify>
  <done>All four test cases pass, covering fully-qualified lookup, unqualified fallback, precedence, and default value scenarios</done>
</task>

<task type="auto">
  <name>Task 2: Commit fix and tests</name>
  <files>wdl/workflow/convert.go, wdl/workflow/convert_test.go</files>
  <action>
Stage both the already-modified convert.go and the new tests in convert_test.go. Create a commit with message:

fix(workflow): support fully-qualified JSON keys in bindWorkflowInputs

bindWorkflowInputs now tries the fully-qualified key (e.g. "hello.name")
before falling back to the unqualified key (e.g. "name"). This matches
the standard WDL input JSON format where keys are workflow-qualified.

Adds regression tests for qualified lookup, unqualified fallback,
precedence, and default value scenarios.
  </action>
  <verify>
    <automated>cd /home/uehara/flowy && go test ./wdl/workflow/ -count=1</automated>
  </verify>
  <done>Commit created with fix and tests, all workflow tests pass</done>
</task>

</tasks>

<verification>
- `go test ./wdl/workflow/ -v -run TestBindWorkflowInputs` — all new tests pass
- `go test ./...` — no regressions across entire codebase
</verification>

<success_criteria>
- bindWorkflowInputs fix is committed with regression tests
- Tests cover: fully-qualified key, unqualified fallback, precedence, default value
- All existing tests continue to pass
</success_criteria>

<output>
After completion, create `.planning/quick/4-fix-workflow-input-binding-to-support-fu/4-SUMMARY.md`
</output>
