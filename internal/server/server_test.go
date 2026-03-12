package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/bioflowy/flowy/internal/api"
	"github.com/bioflowy/flowy/internal/config"
	"github.com/bioflowy/flowy/internal/server"
	"github.com/bioflowy/flowy/internal/storage"
	"github.com/bioflowy/flowy/internal/store"
	"github.com/bioflowy/flowy/internal/store/sqlite"
)

// newTestServer creates a Server instance backed by an in-memory SQLiteStore
// and a temporary base directory. Callers must call cleanup() when done.
func newTestServer(t *testing.T) (*server.Server, *sqlite.SQLiteStore, string, func()) {
	t.Helper()

	st, err := sqlite.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}

	baseDir := t.TempDir()
	cfg := &config.Config{
		BaseDir: baseDir,
		Server: config.ServerConfig{
			ListenAddr:          ":0",
			HeartbeatTimeoutSec: 60,
		},
	}

	backend := storage.NewBackend(cfg)
	srv := server.New(cfg, st, backend)
	return srv, st, baseDir, func() {
		st.Close()
	}
}

// doRequest performs an HTTP request against the server using httptest.
func doRequest(t *testing.T, srv *server.Server, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()

	var reqBody *bytes.Buffer
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reqBody = bytes.NewBuffer(data)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, path, reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

// decodeJSON decodes the response body into v.
func decodeJSON(t *testing.T, w *httptest.ResponseRecorder, v interface{}) {
	t.Helper()
	if err := json.NewDecoder(w.Body).Decode(v); err != nil {
		t.Fatalf("decode JSON response: %v (body: %s)", err, w.Body.String())
	}
}

// ===== Task 1 Tests: Read-only endpoints =====

func TestHealth(t *testing.T) {
	srv, _, _, cleanup := newTestServer(t)
	defer cleanup()

	w := doRequest(t, srv, "GET", "/health", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}
	// Body must contain "ok"
	body := w.Body.String()
	if body == "" {
		t.Error("expected non-empty body")
	}
}

func TestStatus_Found(t *testing.T) {
	srv, st, _, cleanup := newTestServer(t)
	defer cleanup()

	// Create a run directly in the store.
	run := &store.Run{
		ID:         "test-run-001",
		WDLPath:    "/tmp/test.wdl",
		InputsJSON: "{}",
		State:      store.RunRunning,
	}
	if err := st.CreateRun(context.Background(), run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	w := doRequest(t, srv, "GET", "/workflows/test-run-001/status", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp api.StatusResponse
	decodeJSON(t, w, &resp)
	if resp.RunID != "test-run-001" {
		t.Errorf("run_id mismatch: got %q", resp.RunID)
	}
	if resp.State != "RUNNING" {
		t.Errorf("state mismatch: got %q", resp.State)
	}
}

func TestStatus_NotFound(t *testing.T) {
	srv, _, _, cleanup := newTestServer(t)
	defer cleanup()

	w := doRequest(t, srv, "GET", "/workflows/nonexistent/status", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp api.ErrorResponse
	decodeJSON(t, w, &resp)
	if resp.Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestOutputs_Completed(t *testing.T) {
	srv, st, baseDir, cleanup := newTestServer(t)
	defer cleanup()

	runID := "completed-run-001"

	// Create run dirs and write outputs.json.
	if err := store.CreateRunDirs(baseDir, runID); err != nil {
		t.Fatalf("CreateRunDirs: %v", err)
	}

	outputsJSON := map[string]interface{}{"result": "hello"}
	outputsData, _ := json.Marshal(outputsJSON)
	outputsPath := filepath.Join(baseDir, "runs", runID, "outputs.json")
	if err := os.WriteFile(outputsPath, outputsData, 0o644); err != nil {
		t.Fatalf("WriteFile outputs.json: %v", err)
	}

	// Create run with COMPLETED state.
	run := &store.Run{
		ID:         runID,
		WDLPath:    "/tmp/test.wdl",
		InputsJSON: "{}",
		State:      store.RunCompleted,
	}
	if err := st.CreateRun(context.Background(), run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	w := doRequest(t, srv, "GET", fmt.Sprintf("/workflows/%s/outputs", runID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp api.OutputsResponse
	decodeJSON(t, w, &resp)
	if resp.RunID != runID {
		t.Errorf("run_id mismatch: got %q", resp.RunID)
	}
	if resp.Outputs["result"] != "hello" {
		t.Errorf("outputs mismatch: got %v", resp.Outputs)
	}
}

func TestOutputs_NotCompleted(t *testing.T) {
	srv, st, _, cleanup := newTestServer(t)
	defer cleanup()

	run := &store.Run{
		ID:         "running-run-001",
		WDLPath:    "/tmp/test.wdl",
		InputsJSON: "{}",
		State:      store.RunRunning,
	}
	if err := st.CreateRun(context.Background(), run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	w := doRequest(t, srv, "GET", "/workflows/running-run-001/outputs", nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp api.ErrorResponse
	decodeJSON(t, w, &resp)
	if resp.Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestOutputs_NotFound(t *testing.T) {
	srv, _, _, cleanup := newTestServer(t)
	defer cleanup()

	w := doRequest(t, srv, "GET", "/workflows/nonexistent/outputs", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func Test404JSON(t *testing.T) {
	srv, _, _, cleanup := newTestServer(t)
	defer cleanup()

	w := doRequest(t, srv, "GET", "/nonexistent-route", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body: %s)", w.Code, w.Body.String())
	}

	// Must be JSON, not HTML
	contentType := w.Header().Get("Content-Type")
	if contentType == "" {
		t.Error("expected Content-Type header")
	}

	var resp api.ErrorResponse
	decodeJSON(t, w, &resp)
	if resp.Error == "" {
		t.Error("expected non-empty error in JSON 404 response")
	}
}

// ===== Task 2 Tests: Submit, abort, dispatch, stale detection =====

// minimalWDL writes a minimal valid WDL file to a temp dir and returns the path.
func minimalWDL(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	content := `version 1.0
workflow simple {
  input {
    String name = "world"
  }
  call hello { input: name = name }
  output {
    String greeting = hello.out
  }
}
task hello {
  input {
    String name
  }
  command <<<
    echo "Hello, ~{name}!"
  >>>
  output {
    String out = read_string(stdout())
  }
  runtime {
    container: "ubuntu:20.04"
  }
}
`
	path := filepath.Join(dir, "test.wdl")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write WDL: %v", err)
	}
	return path
}

// noOutputWDL writes a minimal WDL with no outputs (for simple abort test).
func noOutputWDL(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	content := `version 1.0
workflow simple {
  call hello
}
task hello {
  command <<<
    sleep 60
  >>>
  runtime {
    container: "ubuntu:20.04"
  }
}
`
	path := filepath.Join(dir, "test.wdl")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write WDL: %v", err)
	}
	return path
}

func TestSubmit_Valid(t *testing.T) {
	srv, st, _, cleanup := newTestServer(t)
	defer cleanup()

	wdlPath := minimalWDL(t)

	reqBody := api.SubmitRequest{
		WDLPath: wdlPath,
		InputsJSON: map[string]interface{}{
			"simple.name": "test",
		},
	}
	w := doRequest(t, srv, "POST", "/workflows", reqBody)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp api.SubmitResponse
	decodeJSON(t, w, &resp)
	if resp.RunID == "" {
		t.Error("expected non-empty run_id")
	}

	// Verify run exists in store.
	// The state may already have transitioned from PENDING to RUNNING by the goroutine.
	run, err := st.GetRun(context.Background(), resp.RunID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if run.State != store.RunPending && run.State != store.RunRunning {
		t.Errorf("expected PENDING or RUNNING state, got %q", run.State)
	}

	// Abort the workflow to unblock the goroutine, then shutdown.
	doRequest(t, srv, "POST", fmt.Sprintf("/workflows/%s/abort", resp.RunID), nil)
	srv.Shutdown()
}

func TestSubmit_EmptyWDL(t *testing.T) {
	srv, _, _, cleanup := newTestServer(t)
	defer cleanup()
	defer srv.Shutdown()

	reqBody := api.SubmitRequest{WDLPath: "", InputsJSON: map[string]interface{}{}}
	w := doRequest(t, srv, "POST", "/workflows", reqBody)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestSubmit_InvalidWDL(t *testing.T) {
	srv, _, _, cleanup := newTestServer(t)
	defer cleanup()
	defer srv.Shutdown()

	reqBody := api.SubmitRequest{WDLPath: "/nonexistent/path/file.wdl", InputsJSON: map[string]interface{}{}}
	w := doRequest(t, srv, "POST", "/workflows", reqBody)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestAbort(t *testing.T) {
	srv, st, _, cleanup := newTestServer(t)
	defer cleanup()

	wdlPath := noOutputWDL(t)

	// Submit a workflow.
	reqBody := api.SubmitRequest{
		WDLPath:    wdlPath,
		InputsJSON: map[string]interface{}{},
	}
	w := doRequest(t, srv, "POST", "/workflows", reqBody)
	if w.Code != http.StatusCreated {
		t.Fatalf("submit: expected 201, got %d", w.Code)
	}

	var submitResp api.SubmitResponse
	decodeJSON(t, w, &submitResp)
	runID := submitResp.RunID

	// Wait a short time for goroutine to start and transition to RUNNING.
	// Poll for up to 2 seconds.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		run, _ := st.GetRun(context.Background(), runID)
		if run != nil && run.State == store.RunRunning {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Abort the workflow.
	abortW := doRequest(t, srv, "POST", fmt.Sprintf("/workflows/%s/abort", runID), nil)
	if abortW.Code != http.StatusOK {
		t.Fatalf("abort: expected 200, got %d (body: %s)", abortW.Code, abortW.Body.String())
	}

	var abortResp api.AbortResponse
	decodeJSON(t, abortW, &abortResp)
	if abortResp.State != "ABORTED" {
		t.Errorf("expected ABORTED state, got %q", abortResp.State)
	}

	// Check store.
	run, err := st.GetRun(context.Background(), runID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if run.State != store.RunAborted {
		t.Errorf("expected ABORTED in store, got %q", run.State)
	}

	srv.Shutdown()
}

func TestDispatch_Integration(t *testing.T) {
	srv, st, baseDir, cleanup := newTestServer(t)
	defer cleanup()

	wdlPath := minimalWDL(t)

	// Submit a workflow.
	reqBody := api.SubmitRequest{
		WDLPath:    wdlPath,
		InputsJSON: map[string]interface{}{"simple.name": "world"},
	}
	w := doRequest(t, srv, "POST", "/workflows", reqBody)
	if w.Code != http.StatusCreated {
		t.Fatalf("submit: expected 201, got %d (body: %s)", w.Code, w.Body.String())
	}

	var submitResp api.SubmitResponse
	decodeJSON(t, w, &submitResp)
	runID := submitResp.RunID

	// Wait for a task to appear in the store (dispatchFn was called).
	var claimedTask *store.Task
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		task, err := st.ClaimTask(context.Background(), "test-daemon")
		if err != nil {
			t.Fatalf("ClaimTask error: %v", err)
		}
		if task != nil {
			claimedTask = task
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if claimedTask == nil {
		t.Fatal("no task appeared within 5 seconds")
	}

	if claimedTask.RunID != runID {
		t.Errorf("task run_id mismatch: got %q, want %q", claimedTask.RunID, runID)
	}

	// Write outputs.json to the task directory.
	// Keys must be plain output names (without task prefix), as the scheduler
	// wraps them in a StructValue keyed by the call alias.
	taskDir := store.TaskDir(baseDir, runID, claimedTask.TaskName, claimedTask.ShardIndex)
	outputsData := `{"out": "Hello, world!"}`
	if err := os.WriteFile(filepath.Join(taskDir, "outputs.json"), []byte(outputsData), 0o644); err != nil {
		t.Fatalf("write task outputs.json: %v", err)
	}

	// Mark task as COMPLETED.
	if err := st.UpdateTaskState(context.Background(), claimedTask.ID, store.TaskCompleted); err != nil {
		t.Fatalf("UpdateTaskState: %v", err)
	}

	// Wait for run to reach COMPLETED.
	deadline = time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		run, _ := st.GetRun(context.Background(), runID)
		if run != nil && (run.State == store.RunCompleted || run.State == store.RunFailed) {
			if run.State == store.RunFailed {
				t.Errorf("run failed: got FAILED state")
			}
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	run, err := st.GetRun(context.Background(), runID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if run.State != store.RunCompleted {
		t.Errorf("expected COMPLETED, got %q", run.State)
	}

	srv.Shutdown()
}

func TestStaleDetection(t *testing.T) {
	_, st, _, cleanup := newTestServer(t)
	defer cleanup()

	// Create a run and task directly.
	run := &store.Run{
		ID:         "stale-run-001",
		WDLPath:    "/tmp/test.wdl",
		InputsJSON: "{}",
		State:      store.RunRunning,
	}
	if err := st.CreateRun(context.Background(), run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	task := &store.Task{
		RunID:    "stale-run-001",
		TaskName: "stale_task",
	}
	if err := st.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	// Claim the task so it gets a heartbeat.
	claimed, err := st.ClaimTask(context.Background(), "stale-daemon")
	if err != nil || claimed == nil {
		t.Fatalf("ClaimTask: %v, claimed=%v", err, claimed)
	}

	// Backdate the heartbeat to make it stale.
	_, err = st.WriterDB().ExecContext(context.Background(),
		"UPDATE tasks SET last_heartbeat_at = datetime('now', '-120 seconds') WHERE id = ?",
		claimed.ID,
	)
	if err != nil {
		t.Fatalf("backdate heartbeat: %v", err)
	}

	// Start stale detection with short timeout and short interval for test.
	cfg := &config.Config{
		BaseDir: t.TempDir(),
		Server:  config.ServerConfig{HeartbeatTimeoutSec: 1},
	}
	srv2 := server.New(cfg, st, storage.NewBackend(cfg))
	srv2.SetStaleInterval(500 * time.Millisecond) // short interval for test
	staleCtx, staleCancel := context.WithCancel(context.Background())
	defer staleCancel()
	srv2.StartStaleDetection(staleCtx)

	// Wait for stale detection to run.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		t2, err2 := st.GetTaskByID(context.Background(), claimed.ID)
		if err2 != nil {
			t.Fatalf("GetTaskByID: %v", err2)
		}
		if t2.State == store.TaskPending {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	finalTask, err := st.GetTaskByID(context.Background(), claimed.ID)
	if err != nil {
		t.Fatalf("GetTaskByID: %v", err)
	}
	if finalTask.State != store.TaskPending {
		t.Errorf("expected task reset to PENDING, got %q", finalTask.State)
	}
}

// ===== Task 3 Tests: Daemon-facing endpoints =====

func TestClaim_HasPending(t *testing.T) {
	srv, st, baseDir, cleanup := newTestServer(t)
	defer cleanup()

	// Create a run and task.
	run := &store.Run{ID: "claim-run-001", WDLPath: "/tmp/t.wdl", InputsJSON: "{}", State: store.RunRunning}
	if err := st.CreateRun(context.Background(), run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	task := &store.Task{RunID: "claim-run-001", TaskName: "my_task", ShardIndex: 0}
	if err := st.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	reqBody := api.TaskClaimRequest{DaemonID: "daemon-001"}
	w := doRequest(t, srv, "POST", "/tasks/claim", reqBody)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp api.TaskClaimResponse
	decodeJSON(t, w, &resp)
	if resp.TaskID == "" {
		t.Error("expected non-empty task_id")
	}
	if resp.RunID != "claim-run-001" {
		t.Errorf("run_id mismatch: got %q", resp.RunID)
	}
	if resp.TaskName != "my_task" {
		t.Errorf("task_name mismatch: got %q", resp.TaskName)
	}

	expectedTaskDir := store.TaskDir(baseDir, "claim-run-001", "my_task", 0)
	if resp.InputsDir != expectedTaskDir {
		t.Errorf("inputs_dir mismatch: got %q, want %q", resp.InputsDir, expectedTaskDir)
	}
	if resp.OutputsDir != expectedTaskDir {
		t.Errorf("outputs_dir mismatch: got %q, want %q", resp.OutputsDir, expectedTaskDir)
	}
}

func TestClaim_NoPending(t *testing.T) {
	srv, _, _, cleanup := newTestServer(t)
	defer cleanup()

	reqBody := api.TaskClaimRequest{DaemonID: "daemon-001"}
	w := doRequest(t, srv, "POST", "/tasks/claim", reqBody)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestClaim_EmptyDaemon(t *testing.T) {
	srv, _, _, cleanup := newTestServer(t)
	defer cleanup()

	reqBody := api.TaskClaimRequest{DaemonID: ""}
	w := doRequest(t, srv, "POST", "/tasks/claim", reqBody)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestResult_Completed(t *testing.T) {
	srv, st, _, cleanup := newTestServer(t)
	defer cleanup()

	// Create run and task.
	run := &store.Run{ID: "result-run-001", WDLPath: "/tmp/t.wdl", InputsJSON: "{}", State: store.RunRunning}
	if err := st.CreateRun(context.Background(), run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	task := &store.Task{RunID: "result-run-001", TaskName: "my_task"}
	if err := st.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	// Claim it.
	claimed, err := st.ClaimTask(context.Background(), "daemon-001")
	if err != nil || claimed == nil {
		t.Fatalf("ClaimTask: %v, claimed=%v", err, claimed)
	}

	reqBody := api.TaskResultRequest{State: "COMPLETED"}
	taskIDStr := strconv.FormatInt(claimed.ID, 10)
	w := doRequest(t, srv, "POST", fmt.Sprintf("/tasks/%s/result", taskIDStr), reqBody)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	// Verify task state in store.
	updatedTask, err := st.GetTaskByID(context.Background(), claimed.ID)
	if err != nil {
		t.Fatalf("GetTaskByID: %v", err)
	}
	if updatedTask.State != store.TaskCompleted {
		t.Errorf("expected COMPLETED task state, got %q", updatedTask.State)
	}
}

func TestResult_Failed(t *testing.T) {
	srv, st, _, cleanup := newTestServer(t)
	defer cleanup()

	run := &store.Run{ID: "result-run-002", WDLPath: "/tmp/t.wdl", InputsJSON: "{}", State: store.RunRunning}
	if err := st.CreateRun(context.Background(), run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	task := &store.Task{RunID: "result-run-002", TaskName: "my_task"}
	if err := st.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	claimed, err := st.ClaimTask(context.Background(), "daemon-001")
	if err != nil || claimed == nil {
		t.Fatalf("ClaimTask: %v, claimed=%v", err, claimed)
	}

	reqBody := api.TaskResultRequest{State: "FAILED", Error: "task failed"}
	taskIDStr := strconv.FormatInt(claimed.ID, 10)
	w := doRequest(t, srv, "POST", fmt.Sprintf("/tasks/%s/result", taskIDStr), reqBody)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	updatedTask, err := st.GetTaskByID(context.Background(), claimed.ID)
	if err != nil {
		t.Fatalf("GetTaskByID: %v", err)
	}
	if updatedTask.State != store.TaskFailed {
		t.Errorf("expected FAILED task state, got %q", updatedTask.State)
	}
}

func TestResult_InvalidState(t *testing.T) {
	srv, st, _, cleanup := newTestServer(t)
	defer cleanup()

	run := &store.Run{ID: "result-run-003", WDLPath: "/tmp/t.wdl", InputsJSON: "{}", State: store.RunRunning}
	if err := st.CreateRun(context.Background(), run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	task := &store.Task{RunID: "result-run-003", TaskName: "my_task"}
	if err := st.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	claimed, err := st.ClaimTask(context.Background(), "daemon-001")
	if err != nil || claimed == nil {
		t.Fatalf("ClaimTask: %v, claimed=%v", err, claimed)
	}

	reqBody := api.TaskResultRequest{State: "INVALID"}
	taskIDStr := strconv.FormatInt(claimed.ID, 10)
	w := doRequest(t, srv, "POST", fmt.Sprintf("/tasks/%s/result", taskIDStr), reqBody)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestHeartbeat(t *testing.T) {
	srv, st, _, cleanup := newTestServer(t)
	defer cleanup()

	run := &store.Run{ID: "hb-run-001", WDLPath: "/tmp/t.wdl", InputsJSON: "{}", State: store.RunRunning}
	if err := st.CreateRun(context.Background(), run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	task := &store.Task{RunID: "hb-run-001", TaskName: "my_task"}
	if err := st.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	// Claim task.
	claimed, err := st.ClaimTask(context.Background(), "daemon-001")
	if err != nil || claimed == nil {
		t.Fatalf("ClaimTask: %v, claimed=%v", err, claimed)
	}

	reqBody := api.HeartbeatRequest{DaemonID: "daemon-001"}
	w := doRequest(t, srv, "POST", "/heartbeat", reqBody)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	// Verify heartbeat was updated (timestamp should be recent).
	updatedTask, err := st.GetTaskByID(context.Background(), claimed.ID)
	if err != nil {
		t.Fatalf("GetTaskByID: %v", err)
	}
	if updatedTask.LastHeartbeatAt == nil {
		t.Error("expected LastHeartbeatAt to be set")
	}
}

func TestHeartbeat_EmptyDaemon(t *testing.T) {
	srv, _, _, cleanup := newTestServer(t)
	defer cleanup()

	reqBody := api.HeartbeatRequest{DaemonID: ""}
	w := doRequest(t, srv, "POST", "/heartbeat", reqBody)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", w.Code, w.Body.String())
	}
}
