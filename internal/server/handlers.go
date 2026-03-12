// Package server handlers: all HTTP handler methods for the flowy server.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/bioflowy/flowy/internal/api"
	"github.com/bioflowy/flowy/internal/store"
	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/parser"
)

// writeJSONBody encodes v as JSON to w and returns any error.
func writeJSONBody(w io.Writer, v interface{}) error {
	enc := json.NewEncoder(w)
	return enc.Encode(v)
}

// handleHealth handles GET /health.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleStatus handles GET /workflows/{id}/status.
// The {id} parameter may be a full UUID or a unique prefix of a run ID.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "id")
	run, err := s.store.GetRun(r.Context(), runID)
	if err != nil {
		// Try prefix match as fallback.
		run, err = s.store.GetRunByPrefix(r.Context(), runID)
		if err != nil {
			writeError(w, http.StatusNotFound, fmt.Sprintf("run not found: %s", runID))
			return
		}
	}
	writeJSON(w, http.StatusOK, api.StatusResponse{
		RunID: run.ID,
		State: string(run.State),
	})
}

// handleOutputs handles GET /workflows/{id}/outputs.
// The {id} parameter may be a full UUID or a unique prefix of a run ID.
func (s *Server) handleOutputs(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "id")
	run, err := s.store.GetRun(r.Context(), runID)
	if err != nil {
		// Try prefix match as fallback.
		run, err = s.store.GetRunByPrefix(r.Context(), runID)
		if err != nil {
			writeError(w, http.StatusNotFound, fmt.Sprintf("run not found: %s", runID))
			return
		}
	}
	if run.State != store.RunCompleted {
		writeError(w, http.StatusConflict, fmt.Sprintf("run not completed (state: %s)", run.State))
		return
	}

	// Read outputs.json from the run directory using the resolved full run ID.
	outputsPath := filepath.Join(s.cfg.BaseDir, "runs", run.ID, "outputs.json")
	data, err := os.ReadFile(outputsPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("read outputs: %v", err))
		return
	}

	var outputs map[string]interface{}
	if err := json.Unmarshal(data, &outputs); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("parse outputs: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, api.OutputsResponse{
		RunID:   run.ID,
		Outputs: outputs,
	})
}

// handleSubmit handles POST /workflows.
func (s *Server) handleSubmit(w http.ResponseWriter, r *http.Request) {
	var req api.SubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}
	if req.WDLPath == "" {
		writeError(w, http.StatusBadRequest, "wdl_path is required")
		return
	}

	// Parse the WDL file, honoring client-provided import search paths.
	var doc *ast.Document
	var err error
	if len(req.SearchPaths) > 0 {
		doc, err = parser.ParseWithSearchPaths(req.WDLPath, nil, req.SearchPaths)
	} else {
		doc, err = parser.Parse(req.WDLPath, nil)
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("parse WDL: %v", err))
		return
	}

	// Use client-provided run ID if given, otherwise generate a new UUID.
	runID := req.RunID
	if runID == "" {
		runID = uuid.New().String()
	}

	// Create run directories.
	if err := store.CreateRunDirs(s.cfg.BaseDir, runID); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("create run dirs: %v", err))
		return
	}

	// Marshal inputs to JSON string.
	inputsJSON, err := json.Marshal(req.InputsJSON)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("marshal inputs: %v", err))
		return
	}

	// Create the run record in the store.
	run := &store.Run{
		ID:         runID,
		WDLPath:    req.WDLPath,
		InputsJSON: string(inputsJSON),
		State:      store.RunPending,
	}
	if err := s.store.CreateRun(r.Context(), run); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("create run: %v", err))
		return
	}

	// Create a run context tied to context.Background() (not r.Context()).
	// This ensures the run goroutine outlives the HTTP request.
	runCtx, cancel := context.WithCancel(context.Background())

	// Store the cancel function so handleAbort can cancel the run.
	s.mu.Lock()
	s.runs[runID] = cancel
	s.mu.Unlock()

	// Launch the run goroutine.
	s.wg.Add(1)
	go s.runWorkflow(runCtx, runID, doc, req.InputsJSON)

	writeJSON(w, http.StatusCreated, api.SubmitResponse{RunID: runID})
}

// handleTaskRun handles POST /tasks/run.
// It runs a single named task from a WDL file by directly dispatching it
// to the daemon, bypassing the workflow engine entirely.
func (s *Server) handleTaskRun(w http.ResponseWriter, r *http.Request) {
	var req api.TaskRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}
	if req.WDLPath == "" {
		writeError(w, http.StatusBadRequest, "wdl_path is required")
		return
	}
	if req.TaskName == "" {
		writeError(w, http.StatusBadRequest, "task_name is required")
		return
	}

	// Parse the WDL file, honoring client-provided import search paths.
	var doc *ast.Document
	var err error
	if len(req.SearchPaths) > 0 {
		doc, err = parser.ParseWithSearchPaths(req.WDLPath, nil, req.SearchPaths)
	} else {
		doc, err = parser.Parse(req.WDLPath, nil)
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("parse WDL: %v", err))
		return
	}

	// Find the named task in the document.
	var targetTask *ast.Task
	for _, t := range doc.Tasks {
		if t.Name == req.TaskName {
			targetTask = t
			break
		}
	}
	if targetTask == nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("task not found: %s", req.TaskName))
		return
	}
	_ = targetTask // validated; daemon re-parses WDL to execute

	// Use client-provided run ID if given, otherwise generate a new UUID.
	runID := req.RunID
	if runID == "" {
		runID = uuid.New().String()
	}

	// Create run directories.
	if err := store.CreateRunDirs(s.cfg.BaseDir, runID); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("create run dirs: %v", err))
		return
	}

	// Create task directories (shard 0 for single-task dispatch).
	if err := store.CreateTaskDirs(s.cfg.BaseDir, runID, req.TaskName, 0); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("create task dirs: %v", err))
		return
	}
	taskDir := store.TaskDir(s.cfg.BaseDir, runID, req.TaskName, 0)

	// Write inputs.json directly from client inputs (no prefix transformation).
	inputsData, err := json.Marshal(req.InputsJSON)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("marshal inputs: %v", err))
		return
	}
	if err := os.WriteFile(filepath.Join(taskDir, "inputs.json"), inputsData, 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("write inputs.json: %v", err))
		return
	}

	// Create Run record (state=RUNNING, task is immediately dispatchable).
	run := &store.Run{
		ID:         runID,
		WDLPath:    req.WDLPath,
		InputsJSON: string(inputsData),
		State:      store.RunRunning,
	}
	if err := s.store.CreateRun(r.Context(), run); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("create run: %v", err))
		return
	}

	// Create Task record.
	storeTask := &store.Task{
		RunID:      runID,
		TaskName:   req.TaskName,
		ShardIndex: 0,
	}
	if err := s.store.CreateTask(r.Context(), storeTask); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("create task: %v", err))
		return
	}

	// Launch background goroutine to wait for task completion.
	runCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.runs[runID] = cancel
	s.mu.Unlock()
	s.wg.Add(1)
	go s.waitForTaskAndFinalize(runCtx, runID, storeTask.ID, req.TaskName)

	writeJSON(w, http.StatusCreated, api.TaskRunResponse{RunID: runID})
}

// handleAbort handles POST /workflows/{id}/abort.
func (s *Server) handleAbort(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "id")

	// Look up the cancel function for the in-flight run.
	s.mu.Lock()
	cancel, ok := s.runs[runID]
	s.mu.Unlock()

	if !ok {
		// Check if the run exists at all.
		_, err := s.store.GetRun(r.Context(), runID)
		if err != nil {
			writeError(w, http.StatusNotFound, fmt.Sprintf("run not found: %s", runID))
			return
		}
		// Run exists but not in the active map — already completed.
		writeError(w, http.StatusNotFound, "run not found or already completed")
		return
	}

	// Cancel the run goroutine.
	cancel()

	// Transition run to ABORTED.
	if err := s.store.UpdateRunState(r.Context(), runID, store.RunAborted); err != nil {
		log.Printf("handleAbort: UpdateRunState %s: %v", runID, err)
	}

	// Abort all pending/claimed tasks for this run.
	tasks, err := s.store.ListTasksByRun(r.Context(), runID)
	if err != nil {
		log.Printf("handleAbort: ListTasksByRun %s: %v", runID, err)
	}
	for _, t := range tasks {
		if t.State == store.TaskPending || t.State == store.TaskClaimed || t.State == store.TaskRunning {
			if abortErr := s.store.UpdateTaskState(r.Context(), t.ID, store.TaskAborted); abortErr != nil {
				log.Printf("handleAbort: UpdateTaskState %d: %v", t.ID, abortErr)
			}
		}
	}

	writeJSON(w, http.StatusOK, api.AbortResponse{
		RunID: runID,
		State: "ABORTED",
	})
}

// handleClaim handles POST /tasks/claim.
func (s *Server) handleClaim(w http.ResponseWriter, r *http.Request) {
	var req api.TaskClaimRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}
	if req.DaemonID == "" {
		writeError(w, http.StatusBadRequest, "daemon_id is required")
		return
	}

	task, err := s.store.ClaimTask(r.Context(), req.DaemonID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("claim task: %v", err))
		return
	}
	if task == nil {
		// No pending tasks.
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Retrieve the WDL path for the parent run so the daemon can parse it.
	var wdlPath string
	run, err := s.store.GetRun(r.Context(), task.RunID)
	if err != nil {
		log.Printf("handleClaim: GetRun %s: %v — daemon will fail on WDL parse", task.RunID, err)
	} else {
		wdlPath = run.WDLPath
	}

	taskDir := store.TaskDir(s.cfg.BaseDir, task.RunID, task.TaskName, task.ShardIndex)
	writeJSON(w, http.StatusOK, api.TaskClaimResponse{
		TaskID:     strconv.FormatInt(task.ID, 10),
		RunID:      task.RunID,
		TaskName:   task.TaskName,
		ShardIndex: task.ShardIndex,
		InputsDir:  taskDir,
		OutputsDir: taskDir,
		WDLPath:    wdlPath,
	})
}

// handleResult handles POST /tasks/{id}/result.
func (s *Server) handleResult(w http.ResponseWriter, r *http.Request) {
	taskIDStr := chi.URLParam(r, "id")
	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid task id: %s", taskIDStr))
		return
	}

	var req api.TaskResultRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}

	var taskState store.TaskState
	switch req.State {
	case "COMPLETED":
		taskState = store.TaskCompleted
	case "FAILED":
		taskState = store.TaskFailed
	default:
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid state %q: must be COMPLETED or FAILED", req.State))
		return
	}

	if err := s.store.UpdateTaskState(r.Context(), taskID, taskState); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("update task state: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleHeartbeat handles POST /heartbeat.
func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	var req api.HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}
	if req.DaemonID == "" {
		writeError(w, http.StatusBadRequest, "daemon_id is required")
		return
	}

	if err := s.store.UpdateHeartbeatByDaemon(r.Context(), req.DaemonID); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("update heartbeat: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

