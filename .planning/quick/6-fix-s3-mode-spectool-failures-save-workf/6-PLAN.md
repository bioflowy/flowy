---
phase: quick-6
plan: 1
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/store/store.go
  - internal/store/sqlite/migrations/00003_run_outputs.sql
  - internal/store/sqlite/runs.go
  - internal/store/sqlite/sqlite_test.go
  - internal/server/dispatch.go
  - internal/server/handlers.go
  - wdl/workflow/workflow.go
  - wdl/workflow/scheduler_test.go
autonomous: true
requirements: []
must_haves:
  truths:
    - "Workflow outputs are saved to DB on run completion"
    - "GET /workflows/{id}/outputs returns outputs from DB in S3 mode"
    - "GET /workflows/{id}/outputs falls back to file for backward compat"
    - "runWorkflowInternal no longer writes outputs.json/error.json to local filesystem"
    - "spectool-s3 tests no longer fail due to missing local outputs.json"
  artifacts:
    - path: "internal/store/sqlite/migrations/00003_run_outputs.sql"
      provides: "outputs_json column on runs table"
    - path: "internal/store/store.go"
      provides: "OutputsJSON field on Run struct + UpdateRunOutputs in Store interface"
    - path: "internal/store/sqlite/runs.go"
      provides: "UpdateRunOutputs implementation + updated scanRun"
  key_links:
    - from: "internal/server/dispatch.go"
      to: "store.UpdateRunOutputs"
      via: "s.store.UpdateRunOutputs after workflow completion"
      pattern: "s\\.store\\.UpdateRunOutputs"
    - from: "internal/server/handlers.go"
      to: "run.OutputsJSON"
      via: "DB-first read in handleOutputs"
      pattern: "run\\.OutputsJSON"
---

<objective>
Fix 93/96 spectool-s3 test failures caused by `runWorkflowInternal()` attempting to write
`outputs.json` and `error.json` to a local directory that does not exist in S3 mode.

Purpose: In S3 mode, `{baseDir}/runs/{runID}/` is not a local directory. The workflow engine
writes outputs.json via `os.WriteFile` which fails. The fix saves run-level outputs to the
DB (mirroring the existing pattern for task-level outputs), and removes the local file writes
from the workflow engine (callers already receive outputs as return values).

Output: Working DB-based output storage for runs; handleOutputs reads from DB first with
file fallback for backward compatibility.
</objective>

<execution_context>
@/home/uehara/.claude/get-shit-done/workflows/execute-plan.md
@/home/uehara/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@internal/store/store.go
@internal/store/sqlite/runs.go
@internal/store/sqlite/migrations/00002_task_outputs.sql
@internal/server/dispatch.go
@internal/server/handlers.go
@wdl/workflow/workflow.go
@wdl/workflow/output.go
@wdl/workflow/scheduler_test.go

<interfaces>
<!-- Key types and contracts the executor needs -->

From internal/store/store.go:
```go
type Run struct {
	ID         string
	WDLPath    string
	InputsJSON string
	State      RunState
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Store interface {
	// ... existing methods ...
	UpdateTaskOutputs(ctx context.Context, taskID int64, outputsJSON string) error
	UpdateTaskInputs(ctx context.Context, taskID int64, inputsJSON string) error
}
```

From internal/store/sqlite/runs.go:
```go
func scanRun(row *sql.Row) (*store.Run, error) // scans 6 columns: id, wdl_path, inputs_json, state, created_at, updated_at
func scanRunRows(rows *sql.Rows) (*store.Run, error) // same 6 columns
```

From internal/server/dispatch.go:
```go
func (s *Server) runWorkflow(ctx context.Context, runID string, doc *ast.Document, inputs map[string]interface{})
func (s *Server) waitForTaskAndFinalize(ctx context.Context, runID string, taskID int64, taskName string)
// Both write outputs.json via s.backend.UploadFile after workflow/task completion
```

From wdl/workflow/workflow.go:
```go
func runWorkflowInternal(...) (map[string]values.Value, error)
// Lines 141-145: writes error.json via os.WriteFile on failure
// Lines 161-165: writes outputs.json via writeOutputsJSON (os.WriteFile) on success
```
</interfaces>
</context>

<tasks>

<task type="auto">
  <name>Task 1: Add OutputsJSON to Run struct, migration, and Store methods</name>
  <files>internal/store/store.go, internal/store/sqlite/migrations/00003_run_outputs.sql, internal/store/sqlite/runs.go, internal/store/sqlite/sqlite_test.go</files>
  <action>
1. **internal/store/store.go** — Add `OutputsJSON string` field to `Run` struct (after `State` field, before `CreatedAt`). Add comment: `// JSON-encoded run outputs; empty until set by UpdateRunOutputs`. Add `UpdateRunOutputs(ctx context.Context, id string, outputsJSON string) error` method to the `Store` interface, with comment: `// UpdateRunOutputs stores the JSON-encoded outputs for a completed run.`

2. **internal/store/sqlite/migrations/00003_run_outputs.sql** — Create migration following the pattern of 00002:
   ```sql
   -- +goose Up
   -- Adds outputs_json column to the runs table.
   -- Stores JSON-encoded workflow/task outputs so handleOutputs can read from DB
   -- instead of requiring a local outputs.json file (which does not exist in S3 mode).
   ALTER TABLE runs ADD COLUMN outputs_json TEXT;

   -- +goose Down
   -- SQLite does not support DROP COLUMN in older versions; no-op for rollback.
   ```

3. **internal/store/sqlite/runs.go** — Add `UpdateRunOutputs` method following the exact pattern of `UpdateTaskOutputs` in tasks.go:
   ```go
   func (s *SQLiteStore) UpdateRunOutputs(ctx context.Context, id string, outputsJSON string) error {
       _, err := s.writer.ExecContext(ctx,
           `UPDATE runs SET outputs_json = ?, updated_at = datetime('now') WHERE id = ?`,
           outputsJSON, id)
       if err != nil {
           return fmt.Errorf("sqlite: UpdateRunOutputs: %w", err)
       }
       return nil
   }
   ```
   Update `scanRun` and `scanRunRows`: change SQL queries in `GetRun` and `GetRunByPrefix` to SELECT 7 columns (add `outputs_json`). In scan functions, scan `outputs_json` as `sql.NullString` (like tasks.go does for OutputsJSON), and set `r.OutputsJSON = outputsJSON.String` if valid.

4. **internal/store/sqlite/sqlite_test.go** — Add `TestUpdateRunOutputs` test following the pattern of `TestUpdateTaskOutputs`: create a run, call `UpdateRunOutputs` with a JSON string, verify via `GetRun` that `OutputsJSON` is set. Also add a check in `TestCreateAndGetRun` that `OutputsJSON` is initially empty string.
  </action>
  <verify>
    <automated>cd /home/uehara/flowy && go test ./internal/store/... -run "TestUpdateRunOutputs|TestCreateAndGetRun" -v -count=1</automated>
  </verify>
  <done>Run struct has OutputsJSON field, migration adds column, UpdateRunOutputs works, scanRun reads the column, tests pass.</done>
</task>

<task type="auto">
  <name>Task 2: Save run outputs to DB and remove local file writes from workflow engine</name>
  <files>internal/server/dispatch.go, internal/server/handlers.go, wdl/workflow/workflow.go, wdl/workflow/scheduler_test.go</files>
  <action>
1. **wdl/workflow/workflow.go** — Remove the outputs.json and error.json file writes from `runWorkflowInternal`:
   - Remove lines 161-165 (outputs.json write via `writeOutputsJSON`). The function already returns `outputs` to the caller.
   - Remove lines 140-146 (error.json write via `writeErrorJSON`). The function already returns the error to the caller.
   - After removing, the failure path just returns `nil, fmt.Errorf("workflow execution: %w", err)`.
   - After removing, the success path just returns `outputs, nil`.
   - Remove the `"path/filepath"` import if it becomes unused (it will since `execRoot` is only used for `filepath.Join` in those removed lines — BUT check if `execRoot` is used elsewhere in the function, e.g., passed to scheduler). `execRoot` IS used at line 130 (`execRoot: execRoot`) so the parameter stays, but `filepath` import may no longer be needed in this file. Check carefully.

2. **wdl/workflow/scheduler_test.go** — Update tests that verify outputs.json/error.json file creation:
   - `TestRunWorkflow_OutputsJSON` (line 757): Remove the file existence check and JSON content verification (lines 786-799). Keep the test but only verify the returned `outputs` map contains the expected values. Rename to `TestRunWorkflow_OutputsReturned` for clarity.
   - `TestRunWorkflow_FailFast_ErrorJSON` (line 802): Remove error.json existence check (lines 821-833) and outputs.json absence check (lines 835+). Keep the test but only verify that `RunWorkflow` returns an error. Rename to `TestRunWorkflow_FailFast`.
   - `TestRunWorkflow_WorkflowOutputs_JSON` (line 878): Remove file-based verification. Keep the parts testing returned outputs via the function return value.
   - `TestRunWorkflow_EndToEnd` (line 916): If it checks for outputs.json file, remove that check.

3. **internal/server/dispatch.go** — In `runWorkflow()`:
   - After the existing `jsonOutputs` marshaling block (lines 243-256), ADD a call to save outputs to DB:
     ```go
     if err := s.store.UpdateRunOutputs(context.Background(), runID, string(data)); err != nil {
         log.Printf("runWorkflow %s: UpdateRunOutputs: %v", runID, err)
     }
     ```
     Place this BEFORE the existing `s.backend.UploadFile` call. Keep the file write as non-fatal fallback (for local mode backward compat). Make the file write non-fatal: change line 254 from `log.Printf(...)` to just log — it already is log-only, good.
   - In `waitForTaskAndFinalize()`: Same pattern — after marshaling `data` (line 291), add `UpdateRunOutputs` call before the file write. Make the file write failure non-fatal (currently it sets run to FAILED on line 299-301 — change to just log the error and continue to set COMPLETED).

4. **internal/server/handlers.go** — In `handleOutputs()`:
   - After getting the run (line 58-66), BEFORE the file-based read (line 73), add DB-first check:
     ```go
     // Try DB-stored outputs first (S3 mode and new local-mode runs).
     if run.OutputsJSON != "" {
         var outputs map[string]interface{}
         if err := json.Unmarshal([]byte(run.OutputsJSON), &outputs); err != nil {
             writeError(w, http.StatusInternalServerError, fmt.Sprintf("parse DB outputs: %v", err))
             return
         }
         writeJSON(w, http.StatusOK, api.OutputsResponse{
             RunID:   run.ID,
             Outputs: outputs,
         })
         return
     }
     ```
   - Keep the existing file-based fallback for runs created before this change.
   - Remove the unused `"path/filepath"` import if `outputsPath` is the only use — but it's still used in the fallback, so keep it.
  </action>
  <verify>
    <automated>cd /home/uehara/flowy && go test ./wdl/workflow/... ./internal/server/... ./internal/store/... -count=1 -timeout 120s</automated>
  </verify>
  <done>
  - runWorkflowInternal no longer writes outputs.json or error.json to local filesystem
  - dispatch.go saves run outputs to DB via UpdateRunOutputs
  - handleOutputs reads from DB first, falls back to file
  - All existing tests pass (with updated file-write assertions removed)
  - spectool-s3 failures from "writing outputs.json: no such file or directory" are eliminated
  </done>
</task>

</tasks>

<verification>
1. `go test ./... -count=1` passes (no regressions)
2. `go build ./...` succeeds
3. The specific error pattern `writing outputs.json: open .../runs/.../outputs.json: no such file or directory` no longer occurs
</verification>

<success_criteria>
- Run struct has OutputsJSON field persisted in SQLite
- runWorkflowInternal returns outputs/errors without writing local files
- dispatch.go saves run outputs to DB on completion
- handleOutputs reads from DB first with file fallback
- All unit tests pass
</success_criteria>

<output>
After completion, create `.planning/quick/6-fix-s3-mode-spectool-failures-save-workf/6-SUMMARY.md`
</output>
