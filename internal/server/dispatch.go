// Package server dispatch: task dispatching and workflow run goroutine logic.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"sync/atomic"
	"time"

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
	result := make(map[string]values.Value, len(raw))
	for k, v := range raw {
		wv, err := jsonToValue(v)
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
		entries := make([]values.MapEntry, 0, len(val))
		for k, v := range val {
			ev, err := jsonToValue(v)
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
		}
	} else {
		// Write run-level outputs.json with "workflowName.outputName" keys.
		if outputs != nil {
			jsonOutputs := make(map[string]interface{}, len(outputs))
			for k, v := range outputs {
				jsonOutputs[wfName+"."+k] = values.ValueToInterface(v)
			}
			data, marshalErr := json.MarshalIndent(jsonOutputs, "", "  ")
			if marshalErr != nil {
				log.Printf("runWorkflow %s: marshal outputs: %v", runID, marshalErr)
			} else {
				outPath := filepath.Join(execRoot, "outputs.json")
				if writeErr := s.backend.UploadFile(outPath, data); writeErr != nil {
					log.Printf("runWorkflow %s: write outputs.json: %v", runID, writeErr)
				}
			}
		}

		// Success: only set COMPLETED if context is still valid (not aborted).
		if ctx.Err() == nil {
			if err := s.store.UpdateRunState(context.Background(), runID, store.RunCompleted); err != nil {
				log.Printf("runWorkflow %s: UpdateRunState COMPLETED: %v", runID, err)
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
		return
	}

	// Write run-level outputs.json with "taskName.outputName" keys.
	execRoot := filepath.Join(s.cfg.BaseDir, "runs", runID)
	outPath := filepath.Join(execRoot, "outputs.json")
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
	if err := s.backend.UploadFile(outPath, data); err != nil {
		log.Printf("runTask %s: write outputs.json: %v", runID, err)
		_ = s.store.UpdateRunState(context.Background(), runID, store.RunFailed)
		return
	}

	_ = s.store.UpdateRunState(context.Background(), runID, store.RunCompleted)
}

// cleanupRun removes the run's cancel function from the active runs map.
func (s *Server) cleanupRun(runID string) {
	s.mu.Lock()
	delete(s.runs, runID)
	s.mu.Unlock()
}

