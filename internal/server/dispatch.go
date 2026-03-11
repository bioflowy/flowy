// Package server dispatch: task dispatching and workflow run goroutine logic.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
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
func (s *Server) makeDispatchFn(runID string) workflow.TaskDispatchFn {
	return func(ctx context.Context, t *ast.Task, inputs map[string]interface{},
		runner task.ContainerRunner, execRoot string,
		structDefs ...map[string]*ast.StructDef) (map[string]values.Value, error) {

		shardIndex := 0 // non-scatter default; scatter iteration is handled by scheduler

		// Create per-task directory.
		if err := store.CreateTaskDirs(s.cfg.BaseDir, runID, t.Name, shardIndex); err != nil {
			return nil, fmt.Errorf("create task dirs for %q: %w", t.Name, err)
		}
		taskDir := store.TaskDir(s.cfg.BaseDir, runID, t.Name, shardIndex)

		// Write inputs.json for the daemon to read.
		inputsData, err := json.Marshal(inputs)
		if err != nil {
			return nil, fmt.Errorf("marshal inputs for task %q: %w", t.Name, err)
		}
		inputsPath := filepath.Join(taskDir, "inputs.json")
		if err := os.WriteFile(inputsPath, inputsData, 0o644); err != nil {
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
				return loadTaskOutputs(s.cfg.BaseDir, runID, taskName, t.ShardIndex)
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

// loadTaskOutputs reads and parses outputs.json from the per-task directory.
// Returns a map of WDL values converted from JSON primitive types.
func loadTaskOutputs(baseDir, runID, taskName string, shardIndex int) (map[string]values.Value, error) {
	taskDir := store.TaskDir(baseDir, runID, taskName, shardIndex)
	outputsPath := filepath.Join(taskDir, "outputs.json")

	data, err := os.ReadFile(outputsPath)
	if err != nil {
		return nil, fmt.Errorf("read task outputs.json for %q: %w", taskName, err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse task outputs.json for %q: %w", taskName, err)
	}

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
// For Phase 10, only primitive types (string, bool, int/float) are handled.
// Complex WDL types (Array, Map, Struct) will be refined in Phase 11.
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
	case nil:
		return values.NullValue{}, nil
	default:
		// For complex types (arrays, objects), stringify as a best-effort.
		// This is a known limitation — see docs/technical-debt/ if needed.
		b, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("unsupported output value type %T: %w", v, err)
		}
		return values.StringValue{Type: types.StringType{}, Value: string(b)}, nil
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
	_, err := workflow.RunWorkflowWithDispatch(ctx, doc, wfName, inputs, nil, execRoot, s.makeDispatchFn(runID))

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
		// Success: only set COMPLETED if context is still valid (not aborted).
		if ctx.Err() == nil {
			if err := s.store.UpdateRunState(context.Background(), runID, store.RunCompleted); err != nil {
				log.Printf("runWorkflow %s: UpdateRunState COMPLETED: %v", runID, err)
			}
		}
	}

	s.cleanupRun(runID)
}

// cleanupRun removes the run's cancel function from the active runs map.
func (s *Server) cleanupRun(runID string) {
	s.mu.Lock()
	delete(s.runs, runID)
	s.mu.Unlock()
}
