// Package server dispatch: task dispatching and workflow run goroutine logic.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/bioflowy/flowy/internal/store"
	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/task"
	"github.com/bioflowy/flowy/wdl/types"
	"github.com/bioflowy/flowy/wdl/values"
	"github.com/bioflowy/flowy/wdl/workflow"
)

// makeDispatchFn returns a TaskDispatchFn that enqueues tasks into the SQLite
// store and waits for daemon completion, instead of running containers locally.
// Each invocation gets a unique shard index via an atomic counter so that
// concurrent scatter iterations of the same task do not overwrite each
// other's input/output directories.
func (s *Server) makeDispatchFn(runID string) workflow.TaskDispatchFn {
	var shardCounter int64 // atomic counter for unique shard indices per run
	return func(ctx context.Context, t *ast.Task, inputs map[string]interface{},
		runner task.ContainerRunner, execRoot string, wdlDir string,
		structDefs ...map[string]*ast.StructDef) (map[string]values.Value, error) {

		shardIndex := int(atomic.AddInt64(&shardCounter, 1) - 1)

		// Create per-task directory.
		if err := s.backend.CreateTaskDirs(s.cfg.BaseDir, runID, t.Name, shardIndex); err != nil {
			return nil, fmt.Errorf("create task dirs for %q: %w", t.Name, err)
		}
		taskDir := s.backend.TaskDir(s.cfg.BaseDir, runID, t.Name, shardIndex)

		// Write inputs.json for the daemon to read.
		inputsData, err := json.Marshal(inputs)
		if err != nil {
			return nil, fmt.Errorf("marshal inputs for task %q: %w", t.Name, err)
		}
		inputsPath := filepath.Join(taskDir, "inputs.json")
		if err := s.backend.UploadFile(inputsPath, inputsData); err != nil {
			return nil, fmt.Errorf("write inputs.json for task %q: %w", t.Name, err)
		}

		// Create a Task record in the store.
		storeTask := &store.Task{
			RunID:      runID,
			TaskName:   t.Name,
			ShardIndex: shardIndex,
		}
		if err := s.store.CreateTask(ctx, storeTask); err != nil {
			return nil, fmt.Errorf("create task record for %q: %w", t.Name, err)
		}

		// Store inputs in the task record for API-based delivery to the daemon.
		if updateErr := s.store.UpdateTaskInputs(ctx, storeTask.ID, string(inputsData)); updateErr != nil {
			log.Printf("makeDispatchFn: UpdateTaskInputs for task %d (%q): %v", storeTask.ID, t.Name, updateErr)
			// Non-fatal: daemon can still read inputs.json from shared FS (backward compat).
		}

		// Wait for daemon to complete the task.
		return s.waitForTask(ctx, storeTask.ID, runID, t.Name)
	}
}

// waitForTask polls the store until the task reaches a terminal state.
// Returns outputs on COMPLETED, an error on FAILED or ABORTED, and ctx.Err() on cancellation.
func (s *Server) waitForTask(ctx context.Context, taskID int64, runID, taskName string) (map[string]values.Value, error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	startTime := time.Now()
	warnThreshold := 5 * time.Minute

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			t, err := s.store.GetTaskByID(ctx, taskID)
			if err != nil {
				return nil, fmt.Errorf("poll task %d: %w", taskID, err)
			}

			switch t.State {
			case store.TaskCompleted:
				return s.loadTaskOutputs(ctx, t.ID, runID, taskName, t.ShardIndex)
			case store.TaskFailed:
				return nil, fmt.Errorf("task %q (id=%d) failed", taskName, taskID)
			case store.TaskAborted:
				return nil, context.Canceled
			default:
				// Still pending or running.
				if t.State == store.TaskPending && time.Since(startTime) > warnThreshold {
					log.Printf("warning: task %q (id=%d) has been PENDING for >5 minutes", taskName, taskID)
				}
			}
		}
	}
}

// loadTaskOutputs reads and parses task outputs.
// Primary source: OutputsJSON column in the task record (set by daemon via result API).
// Fallback: outputs.json file in the per-task directory (for backward compatibility
// with local-mode daemons that write files instead of reporting via API).
func (s *Server) loadTaskOutputs(ctx context.Context, taskID int64, runID, taskName string, shardIndex int) (map[string]values.Value, error) {
	// Try store-based outputs first.
	t, err := s.store.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("loadTaskOutputs: GetTaskByID %d: %w", taskID, err)
	}

	if t.OutputsJSON != "" {
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(t.OutputsJSON), &raw); err != nil {
			return nil, fmt.Errorf("parse task OutputsJSON for %q: %w", taskName, err)
		}
		return rawToValues(raw, taskName)
	}

	// S3 daemon always reports outputs via API. If OutputsJSON is empty, there are no outputs.
	// The file-based fallback only applies to local-mode daemons that write outputs.json.
	if s.cfg.Storage.Type == "s3" {
		return map[string]values.Value{}, nil
	}

	// Fallback to file-based outputs.json (backward compat: local-mode daemon).
	taskDir := s.backend.TaskDir(s.cfg.BaseDir, runID, taskName, shardIndex)
	outputsPath := filepath.Join(taskDir, "outputs.json")

	data, err := s.backend.DownloadFile(outputsPath)
	if err != nil {
		return nil, fmt.Errorf("read task outputs.json for %q: %w", taskName, err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse task outputs.json for %q: %w", taskName, err)
	}
	return rawToValues(raw, taskName)
}

// rawToValues converts a map of JSON-decoded Go values to WDL values.Value entries.
func rawToValues(raw map[string]interface{}, taskName string) (map[string]values.Value, error) {
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := make(map[string]values.Value, len(raw))
	for _, k := range keys {
		wv, err := jsonToValue(raw[k])
		if err != nil {
			return nil, fmt.Errorf("task %q output %q: %w", taskName, k, err)
		}
		result[k] = wv
	}
	return result, nil
}

// jsonToValue converts a JSON-decoded Go value to a WDL values.Value.
func jsonToValue(v interface{}) (values.Value, error) {
	switch val := v.(type) {
	case string:
		if strings.HasPrefix(val, "s3://") {
			return values.FileValue{Type: types.FileType{}, Value: val}, nil
		}
		return values.StringValue{Type: types.StringType{}, Value: val}, nil
	case bool:
		return values.BooleanValue{Type: types.BooleanType{}, Value: val}, nil
	case float64:
		// JSON numbers are always float64; try to keep as integer if whole number.
		if val == float64(int64(val)) {
			return values.IntValue{Type: types.IntType{}, Value: int64(val)}, nil
		}
		return values.FloatValue{Type: types.FloatType{}, Value: val}, nil
	case []interface{}:
		items := make([]values.Value, len(val))
		for i, elem := range val {
			item, err := jsonToValue(elem)
			if err != nil {
				return nil, err
			}
			items[i] = item
		}
		var itemType types.Type = types.AnyType{}
		if len(items) > 0 {
			itemType = items[0].ValueType()
		}
		return values.ArrayValue{
			Type:  types.ArrayType{ItemType: itemType},
			Items: items,
		}, nil
	case map[string]interface{}:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		entries := make([]values.MapEntry, 0, len(val))
		for _, k := range keys {
			ev, err := jsonToValue(val[k])
			if err != nil {
				return nil, err
			}
			entries = append(entries, values.MapEntry{
				Key:   values.StringValue{Type: types.StringType{}, Value: k},
				Value: ev,
			})
		}
		return values.MapValue{
			Type:    types.MapType{KeyType: types.StringType{}, ValType: types.AnyType{}},
			Entries: entries,
		}, nil
	case nil:
		return values.NullValue{}, nil
	default:
		return nil, fmt.Errorf("unsupported output value type %T", v)
	}
}

// runWorkflow is the background goroutine that executes a submitted workflow.
// It transitions the run through RUNNING -> COMPLETED/FAILED/ABORTED.
func (s *Server) runWorkflow(ctx context.Context, runID string, doc *ast.Document, inputs map[string]interface{}) {
	defer s.wg.Done()

	// Transition to RUNNING.
	if err := s.store.UpdateRunState(ctx, runID, store.RunRunning); err != nil {
		log.Printf("runWorkflow %s: UpdateRunState RUNNING: %v", runID, err)
		// Continue anyway — the run may still execute.
	}

	// Determine the workflow name (first workflow in the document).
	if len(doc.Workflows) == 0 {
		log.Printf("runWorkflow %s: no workflows in document", runID)
		_ = s.store.UpdateRunState(context.Background(), runID, store.RunFailed)
		s.cleanupRun(runID)
		return
	}
	wfName := doc.Workflows[0].Name
	execRoot := filepath.Join(s.cfg.BaseDir, "runs", runID)

	// Execute the workflow using the dispatch function (no local Docker).
	outputs, err := workflow.RunWorkflowWithDispatch(ctx, doc, wfName, inputs, nil, execRoot, s.makeDispatchFn(runID))

	// Determine terminal state.
	if err != nil {
		if ctx.Err() != nil {
			// Context was cancelled (abort requested) — don't overwrite ABORTED state.
			log.Printf("runWorkflow %s: aborted", runID)
		} else {
			log.Printf("runWorkflow %s: failed: %v", runID, err)
			_ = s.store.UpdateRunState(context.Background(), runID, store.RunFailed)
			if touchErr := s.store.TouchProvenanceUpdatedAt(context.Background(), runID); touchErr != nil {
				log.Printf("runWorkflow %s: TouchProvenanceUpdatedAt: %v", runID, touchErr)
			}
		}
	} else {
		// Success: only set COMPLETED if context is still valid (not aborted).
		if ctx.Err() == nil {
			run, getRunErr := s.store.GetRun(context.Background(), runID)
			if getRunErr != nil {
				log.Printf("runWorkflow %s: GetRun: %v", runID, getRunErr)
				_ = s.store.UpdateRunState(context.Background(), runID, store.RunFailed)
				s.cleanupRun(runID)
				return
			}

			outputsJSON, files, buildErr := buildWorkflowCompletionRecord(run.ProjectID, runID, wfName, doc.Workflows[0], inputs, outputs)
			if buildErr != nil {
				log.Printf("runWorkflow %s: build completion record: %v", runID, buildErr)
				_ = s.store.UpdateRunState(context.Background(), runID, store.RunFailed)
				s.cleanupRun(runID)
				return
			}

			if err := s.store.CompleteRun(context.Background(), runID, run.ProjectID, outputsJSON, files); err != nil {
				log.Printf("runWorkflow %s: CompleteRun: %v", runID, err)
				_ = s.store.UpdateRunState(context.Background(), runID, store.RunFailed)
				s.cleanupRun(runID)
				return
			}

			// Keep outputs.json for backward compatibility with local-mode tooling.
			outPath := filepath.Join(execRoot, "outputs.json")
			if writeErr := s.backend.UploadFile(outPath, []byte(outputsJSON)); writeErr != nil {
				log.Printf("runWorkflow %s: write outputs.json: %v", runID, writeErr)
			}
			if touchErr := s.store.TouchProvenanceUpdatedAt(context.Background(), runID); touchErr != nil {
				log.Printf("runWorkflow %s: TouchProvenanceUpdatedAt: %v", runID, touchErr)
			}
		}
	}

	s.cleanupRun(runID)
}

// waitForTaskAndFinalize polls a single task until completion,
// then writes run-level outputs.json and updates run state.
// Used by handleTaskRun for direct task dispatch (no workflow engine).
func (s *Server) waitForTaskAndFinalize(ctx context.Context, runID string, taskID int64, taskName string) {
	defer s.wg.Done()
	defer s.cleanupRun(runID)

	outputs, err := s.waitForTask(ctx, taskID, runID, taskName)
	if err != nil {
		log.Printf("runTask %s: task %q failed: %v", runID, taskName, err)
		_ = s.store.UpdateRunState(context.Background(), runID, store.RunFailed)
		if touchErr := s.store.TouchProvenanceUpdatedAt(context.Background(), runID); touchErr != nil {
			log.Printf("runTask %s: TouchProvenanceUpdatedAt: %v", runID, touchErr)
		}
		return
	}

	// Write run-level outputs with "taskName.outputName" keys.
	execRoot := filepath.Join(s.cfg.BaseDir, "runs", runID)
	jsonOutputs := make(map[string]interface{}, len(outputs))
	for k, v := range outputs {
		jsonOutputs[taskName+"."+k] = values.ValueToInterface(v)
	}
	data, err := json.MarshalIndent(jsonOutputs, "", "  ")
	if err != nil {
		log.Printf("runTask %s: marshal outputs: %v", runID, err)
		_ = s.store.UpdateRunState(context.Background(), runID, store.RunFailed)
		return
	}

	// Save outputs to DB first (works in both local and S3 mode).
	if err := s.store.UpdateRunOutputs(context.Background(), runID, string(data)); err != nil {
		log.Printf("runTask %s: UpdateRunOutputs: %v", runID, err)
	}

	// Also write outputs.json file as non-fatal backward compat (local mode).
	outPath := filepath.Join(execRoot, "outputs.json")
	if writeErr := s.backend.UploadFile(outPath, data); writeErr != nil {
		log.Printf("runTask %s: write outputs.json: %v", runID, writeErr)
	}

	_ = s.store.UpdateRunState(context.Background(), runID, store.RunCompleted)
	if touchErr := s.store.TouchProvenanceUpdatedAt(context.Background(), runID); touchErr != nil {
		log.Printf("runTask %s: TouchProvenanceUpdatedAt: %v", runID, touchErr)
	}
}

// cleanupRun removes the run's cancel function from the active runs map.
func (s *Server) cleanupRun(runID string) {
	s.mu.Lock()
	delete(s.runs, runID)
	s.mu.Unlock()
}

func buildWorkflowCompletionRecord(
	projectID string,
	runID string,
	workflowName string,
	wf *ast.Workflow,
	inputs map[string]interface{},
	outputs map[string]values.Value,
) (string, []store.ManagedFile, error) {
	jsonOutputs := make(map[string]interface{}, len(outputs))
	for k, v := range outputs {
		jsonOutputs[workflowName+"."+k] = values.ValueToInterface(v)
	}

	data, err := json.MarshalIndent(jsonOutputs, "", "  ")
	if err != nil {
		return "", nil, fmt.Errorf("marshal outputs: %w", err)
	}

	files, err := collectManagedFilesForWorkflowOutputs(projectID, runID, wf, inputs, outputs)
	if err != nil {
		return "", nil, err
	}

	return string(data), files, nil
}

func collectManagedFilesForWorkflowOutputs(
	projectID string,
	runID string,
	wf *ast.Workflow,
	inputs map[string]interface{},
	outputs map[string]values.Value,
) ([]store.ManagedFile, error) {
	if projectID == "" || wf == nil || len(outputs) == 0 {
		return nil, nil
	}

	seen := make(map[string]struct{})
	files := make([]store.ManagedFile, 0)
	for _, decl := range wf.Outputs {
		value, ok := outputs[decl.Name]
		if !ok {
			continue
		}
		fileType := parameterMetaType(wf.ParamMeta, decl.Name)
		labelTemplate := parameterMetaLabel(wf.ParamMeta, decl.Name)
		var declaredType string
		if fileType != nil {
			declaredType = *fileType
		}
		next, err := collectManagedFilesForValue(projectID, runID, decl.Name, decl.Type, value, declaredType, labelTemplate, wf, inputs, seen)
		if err != nil {
			return nil, err
		}
		files = append(files, next...)
	}
	return files, nil
}

func collectManagedFilesForValue(
	projectID string,
	runID string,
	outputName string,
	declType types.Type,
	value values.Value,
	fileType string,
	labelTemplate *string,
	wf *ast.Workflow,
	inputs map[string]interface{},
	seen map[string]struct{},
) ([]store.ManagedFile, error) {
	switch t := declType.(type) {
	case types.FileType:
		return managedFileForPathValue(projectID, runID, outputName, value, fileType, labelTemplate, wf, inputs, seen)
	case types.DirectoryType:
		return managedFileForPathValue(projectID, runID, outputName, value, fileType, labelTemplate, wf, inputs, seen)
	case types.ArrayType:
		if !isManagedPathType(t.ItemType) {
			return nil, nil
		}
		arrayValue, ok := value.(values.ArrayValue)
		if !ok {
			return nil, fmt.Errorf("output %q declared %s but got %T", outputName, declType.TypeName(), value)
		}

		files := make([]store.ManagedFile, 0, len(arrayValue.Items))
		for _, item := range arrayValue.Items {
			next, err := collectManagedFilesForValue(projectID, runID, outputName, t.ItemType, item, fileType, labelTemplate, wf, inputs, seen)
			if err != nil {
				return nil, err
			}
			files = append(files, next...)
		}
		return files, nil
	default:
		return nil, nil
	}
}

func managedFileForPathValue(
	projectID string,
	runID string,
	outputName string,
	value values.Value,
	fileType string,
	labelTemplate *string,
	wf *ast.Workflow,
	inputs map[string]interface{},
	seen map[string]struct{},
) ([]store.ManagedFile, error) {
	path, present, err := pathFromManagedValue(outputName, value)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	if _, ok := seen[path]; ok {
		return nil, nil
	}
	seen[path] = struct{}{}
	runIDCopy := runID
	return []store.ManagedFile{{
		ID:         uuid.New().String(),
		ProjectID:  projectID,
		RunID:      &runIDCopy,
		Path:       path,
		FileType:   fileType,
		Label:      managedFileLabelForPath(path, labelTemplate, wf, inputs),
		SourceType: "workflow",
	}}, nil
}

func pathFromManagedValue(outputName string, value values.Value) (string, bool, error) {
	switch v := value.(type) {
	case values.FileValue:
		return v.Value, true, nil
	case values.DirectoryValue:
		return v.Value, true, nil
	case values.NullValue:
		return "", false, nil
	default:
		return "", false, fmt.Errorf("output %q expected File/Directory-compatible value but got %T", outputName, value)
	}
}

func isManagedPathType(t types.Type) bool {
	switch t.(type) {
	case types.FileType, types.DirectoryType:
		return true
	default:
		return false
	}
}
