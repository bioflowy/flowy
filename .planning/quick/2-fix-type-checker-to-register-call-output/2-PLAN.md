---
phase: quick
plan: 2
type: execute
wave: 1
depends_on: []
files_modified:
  - wdl/checker/checker.go
  - wdl/checker/checker_test.go
  - wdl/testdata/call_output.wdl
  - wdl/testdata/scatter_call.wdl
autonomous: true
requirements: []
must_haves:
  truths:
    - "Workflow output referencing call.output resolves without type error"
    - "Scatter-wrapped call outputs are promoted to Array[T]"
    - "Conditional-wrapped call outputs are promoted to T?"
    - "Call alias takes precedence over task name as scope identifier"
  artifacts:
    - path: "wdl/checker/checker.go"
      provides: "case *ast.Call in collectWorkflowBodyGroups and collectWorkflowBodyGroupsInner"
      contains: "case *ast.Call"
    - path: "wdl/checker/checker_test.go"
      provides: "Tests for call output registration"
      contains: "TestCallOutputInWorkflow"
    - path: "wdl/testdata/call_output.wdl"
      provides: "WDL fixture with workflow output referencing call output"
    - path: "wdl/testdata/scatter_call.wdl"
      provides: "WDL fixture with scatter-wrapped call"
  key_links:
    - from: "collectWorkflowBodyGroups"
      to: "scope.define(callID, StructType)"
      via: "case *ast.Call builds StructType from task outputs"
      pattern: "case \\*ast\\.Call"
    - from: "checkGetExpr"
      to: "StructType.Members"
      via: "scope.lookup resolves call alias to StructType, then member lookup resolves field"
---

<objective>
Fix the type checker to register call outputs in workflow scope so that workflow output
declarations referencing call outputs (e.g., `String result = sayHello.greeting`) resolve correctly.

Purpose: Currently `collectWorkflowBodyGroups` and `collectWorkflowBodyGroupsInner` have no
`case *ast.Call`, so call aliases are never added to the workflow scope. This causes false
"undefined name" errors on any workflow output that references a call output.

Output: Fixed checker.go with call registration, test fixtures, and passing tests.
</objective>

<execution_context>
@/home/uehara/.claude/get-shit-done/workflows/execute-plan.md
@/home/uehara/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@wdl/checker/checker.go
@wdl/checker/struct_check.go
@wdl/checker/checker_test.go
@wdl/ast/ast.go
@wdl/testdata/hello.wdl

<interfaces>
<!-- Key types the executor needs -->

From wdl/ast/ast.go:
```go
type Call struct {
    Pos
    Callee []string
    Alias  string
    After  []string
    Inputs map[string]Expr
}

type Task struct {
    Pos
    Name    string
    Inputs  []*Decl
    Decls   []*Decl
    Command *CommandExpr
    Outputs []*Decl
    // ...
}

type Document struct {
    Pos
    Version   string
    Tasks     []*Task
    Workflows []*Workflow
    Structs   []*StructDef
}
```

From wdl/types (relevant):
```go
type StructType struct {
    Name     string
    Members  map[string]Type
    Optional bool
}
```

From wdl/checker/checker.go:
```go
func collectWorkflowBodyGroups(nodes []ast.WorkflowNode, outerScope *Scope, promoteFn func(types.Type) types.Type) []declGroup
func collectWorkflowBodyGroupsInner(nodes []ast.WorkflowNode, outerScope *Scope, innerScope *Scope, promoteFn func(types.Type) types.Type) []declGroup
func checkWorkflow(wf *ast.Workflow, structDefs map[string]*ast.StructDef) (*Scope, []error)
```

From wdl/checker/struct_check.go:
```go
func checkGetExpr(expr *ast.GetExpr, pos ast.Pos, scope *Scope, structDefs map[string]*ast.StructDef) (error, types.Type)
// checkGetExpr already resolves StructType.Members — once a call is registered
// as a StructType with populated Members, `sayHello.greeting` will resolve automatically.
```
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Add test fixtures and tests for call output resolution</name>
  <files>wdl/testdata/call_output.wdl, wdl/testdata/scatter_call.wdl, wdl/checker/checker_test.go</files>
  <behavior>
    - TestCallOutputInWorkflow: Parse hello.wdl, Check() returns 0 errors, TypeEnv contains "result" with type "String"
    - TestCallOutputInWorkflowInline: Parse call_output.wdl (inline task+workflow), Check() returns 0 errors
    - TestScatterCallPromotion: Parse scatter_call.wdl (scatter wrapping a call), Check() returns 0 errors, output type is Array[String]
    - TestConditionalCallPromotion: In-memory AST with conditional wrapping a call, Check() returns 0 errors, output type is String?
  </behavior>
  <action>
Create two WDL test fixtures:

1. `wdl/testdata/call_output.wdl` — A simple task with String output + workflow calling it and referencing its output:
```wdl
version 1.2
task greet {
  input { String name }
  command { echo "Hi ~{name}" }
  output { String message = read_string(stdout()) }
}
workflow callOutputWf {
  call greet { input: name = "Alice" }
  output { String msg = greet.message }
}
```

2. `wdl/testdata/scatter_call.wdl` — A task + workflow with scatter wrapping a call:
```wdl
version 1.2
task upper {
  input { String s }
  command { echo ~{s} }
  output { String result = read_string(stdout()) }
}
workflow scatterCallWf {
  input { Array[String] items }
  scatter (item in items) {
    call upper { input: s = item }
  }
  output { Array[String] results = upper.result }
}
```
Note: `upper.result` inside scatter should be promoted to `Array[String]` at workflow scope. The output decl references `upper` which, when registered as `StructType{Members: {"result": String}}` and then arrayized by scatter promotion, becomes `Array[StructType]`. However, `upper.result` is a GetExpr — `checkGetExpr` resolves the base `upper` from scope. Inside scatter, `upper` gets arrayize() applied, making it `Array[StructType]`. But GetExpr on an Array does not work — the scatter promotion should apply to each OUTPUT field individually, not to the struct as a whole.

**IMPORTANT DESIGN INSIGHT:** The correct approach is NOT to register the call as a single StructType and then promote the struct. Instead, for calls inside scatter/conditional, each call output field should be registered individually with promotion applied. However, the simpler approach used by most WDL engines is:
- Register the call alias as a StructType whose Members already have promotion applied
- e.g., inside scatter: `upper` -> `StructType{Members: {"result": Array[String]}}`
- This way `upper.result` resolves to `Array[String]` via checkGetExpr

Actually, re-examine: the existing `checkGetExpr` resolves `ident.field` by looking up `ident` as StructType, then looking up `field` in Members. So the Members map values should already reflect the promoted types.

Add these tests to checker_test.go (they will FAIL until Task 2 implements the fix):

- `TestCallOutputInWorkflow`: uses `parseFixture(t, "wdl/testdata/hello.wdl")`, expects 0 errors and env["result"].TypeName() == "String"
- `TestCallOutputInline`: uses `parseFixture(t, "wdl/testdata/call_output.wdl")`, expects 0 errors and env["msg"].TypeName() == "String"
- `TestScatterCallPromotion`: uses `parseFixture(t, "wdl/testdata/scatter_call.wdl")`, expects 0 errors and env["results"].TypeName() == "Array[String]"
- `TestConditionalCallPromotion`: builds in-memory AST with a conditional block containing a call to a task with String output. Expects the call output to become String? in scope. Build the AST programmatically like TestCycleDetection does:
  - Create a Task "innerTask" with Output `String out = ...` (use nil Expr for output since checker only reads Type/Name)
  - Create a Workflow with Conditional body containing a Call to "innerTask"
  - Workflow output: `String? cond_out = innerTask.out` (type String?, expr is GetExpr)
  - Check() should return 0 errors
  </action>
  <verify>
    <automated>cd /home/uehara/flowy && go test ./wdl/checker/ -run "TestCallOutput|TestScatterCall|TestConditionalCall" -v 2>&1 | head -30</automated>
  </verify>
  <done>Test fixtures exist, tests are written and fail with "undefined name" errors (RED phase of TDD)</done>
</task>

<task type="auto">
  <name>Task 2: Add case *ast.Call to collectWorkflowBodyGroups and collectWorkflowBodyGroupsInner</name>
  <files>wdl/checker/checker.go</files>
  <action>
The fix requires passing a task lookup map to the collect functions so they can resolve call callee names to task output declarations.

**Step 1: Modify `checkWorkflow` signature and call site.**

In `checkWorkflow`, build a `taskMap` from `doc.Tasks` before calling `collectWorkflowBodyGroups`. But `checkWorkflow` currently receives only `(wf *ast.Workflow, structDefs ...)` — it does not have access to `doc.Tasks`.

Change `checkWorkflow` to accept an additional parameter `tasks []*ast.Task`:
```go
func checkWorkflow(wf *ast.Workflow, structDefs map[string]*ast.StructDef, tasks []*ast.Task) (*Scope, []error)
```

Update the call site in `Check()` (line ~90):
```go
scope, wfErrs := checkWorkflow(wf, structDefs, doc.Tasks)
```

Inside `checkWorkflow`, build the task map:
```go
taskMap := make(map[string]*ast.Task, len(tasks))
for _, t := range tasks {
    taskMap[t.Name] = t
}
```

**Step 2: Modify `collectWorkflowBodyGroups` to accept `taskMap`.**

Change signature:
```go
func collectWorkflowBodyGroups(nodes []ast.WorkflowNode, outerScope *Scope, promoteFn func(types.Type) types.Type, taskMap map[string]*ast.Task) []declGroup
```

Add `case *ast.Call` to the switch:
```go
case *ast.Call:
    // Resolve the called task name (last element of Callee).
    taskName := n.Callee[len(n.Callee)-1]
    task, ok := taskMap[taskName]
    if !ok {
        continue // unknown task — will be caught elsewhere
    }

    // Build StructType from task outputs, applying promoteFn to each member.
    members := make(map[string]types.Type, len(task.Outputs))
    for _, out := range task.Outputs {
        t := out.Type
        if promoteFn != nil {
            t = promoteFn(t)
        }
        members[out.Name] = t
    }

    // Call ID: alias if set, else task name.
    callID := n.Alias
    if callID == "" {
        callID = taskName
    }

    // Build the StructType. Apply promoteFn to the struct itself too?
    // NO — promotion is applied to each member individually (above).
    // The StructType wrapper itself should NOT be promoted.
    st := types.StructType{Name: callID, Members: members}
    outerScope.define(callID, st)
```

**Step 3: Modify `collectWorkflowBodyGroupsInner` similarly.**

Change signature to accept `taskMap`:
```go
func collectWorkflowBodyGroupsInner(nodes []ast.WorkflowNode, outerScope *Scope, innerScope *Scope, promoteFn func(types.Type) types.Type, taskMap map[string]*ast.Task) []declGroup
```

Add the same `case *ast.Call` logic (identical to step 2 — copy the case block).

**Step 4: Update all recursive calls to pass `taskMap`.**

In `collectWorkflowBodyGroups`:
- The `case *ast.Scatter` calls `collectWorkflowBodyGroupsInner(n.Body, outerScope, innerScope, scatterPromote)` — add `taskMap` parameter
- The `case *ast.Conditional` calls `collectWorkflowBodyGroups(n.Body, outerScope, condPromote)` — add `taskMap` parameter

In `collectWorkflowBodyGroupsInner`:
- The `case *ast.Scatter` calls `collectWorkflowBodyGroupsInner(n.Body, outerScope, nestedInner, scatterPromote)` — add `taskMap` parameter
- The `case *ast.Conditional` calls `collectWorkflowBodyGroupsInner(n.Body, outerScope, innerScope, condPromote)` — add `taskMap` parameter

In `checkWorkflow`:
- The call `collectWorkflowBodyGroups(wf.Body, outerScope, nil)` becomes `collectWorkflowBodyGroups(wf.Body, outerScope, nil, taskMap)`

**Step 5: Verify all tests pass.**

Run `go test ./wdl/checker/ -v` to confirm:
- All existing tests still pass (no regression)
- New call output tests pass (the fix works)

Also run `go vet ./wdl/checker/` to ensure no issues.
  </action>
  <verify>
    <automated>cd /home/uehara/flowy && go test ./wdl/checker/ -v && go vet ./wdl/checker/</automated>
  </verify>
  <done>All checker tests pass including TestCallOutputInWorkflow, TestCallOutputInline, TestScatterCallPromotion, and TestConditionalCallPromotion. No regressions in existing tests. go vet clean.</done>
</task>

</tasks>

<verification>
- `go test ./wdl/checker/ -v` — all tests pass, 0 failures
- `go vet ./wdl/checker/` — no issues
- `go build ./...` — full project compiles
</verification>

<success_criteria>
- hello.wdl type-checks without errors (sayHello.greeting resolves)
- call_output.wdl type-checks without errors (greet.message resolves)
- scatter_call.wdl type-checks: upper.result resolves as Array[String]
- Conditional-wrapped call output resolves as T?
- All pre-existing checker tests continue to pass
</success_criteria>

<output>
After completion, create `.planning/quick/2-fix-type-checker-to-register-call-output/2-SUMMARY.md`
</output>
