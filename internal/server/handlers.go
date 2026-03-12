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
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "id")
	run, err := s.store.GetRun(r.Context(), runID)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("run not found: %s", runID))
		return
	}
	writeJSON(w, http.StatusOK, api.StatusResponse{
		RunID: run.ID,
		State: string(run.State),
	})
}

// handleOutputs handles GET /workflows/{id}/outputs.
func (s *Server) handleOutputs(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "id")
	run, err := s.store.GetRun(r.Context(), runID)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("run not found: %s", runID))
		return
	}
	if run.State != store.RunCompleted {
		writeError(w, http.StatusConflict, fmt.Sprintf("run not completed (state: %s)", run.State))
		return
	}

	// Read outputs.json from the run directory.
	outputsPath := filepath.Join(s.cfg.BaseDir, "runs", runID, "outputs.json")
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
		RunID:   runID,
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

	// Parse the WDL file.
	doc, err := parser.Parse(req.WDLPath, nil)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("parse WDL: %v", err))
		return
	}

	// Generate run ID.
	runID := uuid.New().String()

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

