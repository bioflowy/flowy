package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bioflowy/flowy/internal/api"
	"github.com/bioflowy/flowy/internal/client"
)

// newTestServer creates an httptest server with the given handler mux.
func newTestServer(t *testing.T, mux *http.ServeMux) (*httptest.Server, *client.Client) {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := client.New(srv.URL)
	return srv, c
}

// writeJSON is a helper to write a JSON response.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// TestSubmit_Success tests that Submit sends POST /workflows and returns SubmitResponse on 201.
func TestSubmit_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/workflows", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "wrong method", http.StatusMethodNotAllowed)
			return
		}
		var req api.SubmitRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusCreated, api.SubmitResponse{RunID: "run-abc-123"})
	})

	_, c := newTestServer(t, mux)
	resp, err := c.Submit(context.Background(), api.SubmitRequest{
		WDLPath:    "/data/hello.wdl",
		InputsJSON: map[string]interface{}{"greeting": "Hello"},
	})
	if err != nil {
		t.Fatalf("Submit: unexpected error: %v", err)
	}
	if resp.RunID != "run-abc-123" {
		t.Errorf("Submit: got RunID %q, want %q", resp.RunID, "run-abc-123")
	}
}

// TestSubmit_Error tests that Submit returns an error with the server message on non-201 status.
func TestSubmit_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/workflows", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusBadRequest, api.ErrorResponse{Error: "wdl_path is required"})
	})

	_, c := newTestServer(t, mux)
	_, err := c.Submit(context.Background(), api.SubmitRequest{})
	if err == nil {
		t.Fatal("Submit: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "wdl_path is required") {
		t.Errorf("Submit: error %q should contain server message", err.Error())
	}
}

// TestStatus_Success tests that Status sends GET /workflows/{id}/status and returns StatusResponse on 200.
func TestStatus_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/workflows/run-abc-123/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "wrong method", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, api.StatusResponse{RunID: "run-abc-123", State: "RUNNING"})
	})

	_, c := newTestServer(t, mux)
	resp, err := c.Status(context.Background(), "run-abc-123")
	if err != nil {
		t.Fatalf("Status: unexpected error: %v", err)
	}
	if resp.State != "RUNNING" {
		t.Errorf("Status: got State %q, want %q", resp.State, "RUNNING")
	}
}

// TestStatus_NotFound tests that Status returns an error on 404 (run not found).
func TestStatus_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/workflows/bad-id/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, api.ErrorResponse{Error: "run not found: bad-id"})
	})

	_, c := newTestServer(t, mux)
	_, err := c.Status(context.Background(), "bad-id")
	if err == nil {
		t.Fatal("Status: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Status: error %q should mention 'not found'", err.Error())
	}
}

// TestOutputs_Success tests that Outputs sends GET /workflows/{id}/outputs and returns OutputsResponse on 200.
func TestOutputs_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/workflows/run-abc-123/outputs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "wrong method", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, api.OutputsResponse{
			RunID:   "run-abc-123",
			Outputs: map[string]interface{}{"result": "hello"},
		})
	})

	_, c := newTestServer(t, mux)
	resp, err := c.Outputs(context.Background(), "run-abc-123")
	if err != nil {
		t.Fatalf("Outputs: unexpected error: %v", err)
	}
	if resp.Outputs["result"] != "hello" {
		t.Errorf("Outputs: got result %v, want %q", resp.Outputs["result"], "hello")
	}
}

// TestOutputs_NotCompleted tests that Outputs returns a descriptive error on 409 (run not completed).
func TestOutputs_NotCompleted(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/workflows/run-abc-123/outputs", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusConflict, api.ErrorResponse{Error: "run not completed (state: RUNNING)"})
	})

	_, c := newTestServer(t, mux)
	_, err := c.Outputs(context.Background(), "run-abc-123")
	if err == nil {
		t.Fatal("Outputs: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not yet completed") {
		t.Errorf("Outputs: error %q should contain 'not yet completed'", err.Error())
	}
	if !strings.Contains(err.Error(), "RUNNING") {
		t.Errorf("Outputs: error %q should contain state 'RUNNING'", err.Error())
	}
}

// TestOutputs_NotFound tests that Outputs returns an error on 404.
func TestOutputs_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/workflows/bad-id/outputs", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, api.ErrorResponse{Error: "run not found: bad-id"})
	})

	_, c := newTestServer(t, mux)
	_, err := c.Outputs(context.Background(), "bad-id")
	if err == nil {
		t.Fatal("Outputs: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Outputs: error %q should mention 'not found'", err.Error())
	}
}

// TestTaskRun_Success tests that TaskRun sends POST /tasks/run and returns TaskRunResponse on 201.
func TestTaskRun_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/run", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "wrong method", http.StatusMethodNotAllowed)
			return
		}
		var req api.TaskRunRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		if req.TaskName == "" {
			writeJSON(w, http.StatusBadRequest, api.ErrorResponse{Error: "task_name is required"})
			return
		}
		writeJSON(w, http.StatusCreated, api.TaskRunResponse{RunID: "task-run-123"})
	})

	_, c := newTestServer(t, mux)
	resp, err := c.TaskRun(context.Background(), api.TaskRunRequest{
		WDLPath:  "/data/hello.wdl",
		TaskName: "greet",
	})
	if err != nil {
		t.Fatalf("TaskRun: unexpected error: %v", err)
	}
	if resp.RunID != "task-run-123" {
		t.Errorf("TaskRun: got RunID %q, want %q", resp.RunID, "task-run-123")
	}
}

// TestTaskRun_TaskNotFound tests that TaskRun returns an error when the task is not found in WDL.
func TestTaskRun_TaskNotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/run", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusBadRequest, api.ErrorResponse{Error: "task not found: nonexistent"})
	})

	_, c := newTestServer(t, mux)
	_, err := c.TaskRun(context.Background(), api.TaskRunRequest{
		WDLPath:  "/data/hello.wdl",
		TaskName: "nonexistent",
	})
	if err == nil {
		t.Fatal("TaskRun: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "task not found") {
		t.Errorf("TaskRun: error %q should mention 'task not found'", err.Error())
	}
}

// TestStatusInProject_Success tests that StatusInProject sends GET /projects/{slug}/runs/{id}/status
// and returns StatusResponse on 200.
func TestStatusInProject_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/my-project/runs/run-abc-123/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "wrong method", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, api.StatusResponse{RunID: "run-abc-123", State: "RUNNING"})
	})

	_, c := newTestServer(t, mux)
	resp, err := c.StatusInProject(context.Background(), "my-project", "run-abc-123")
	if err != nil {
		t.Fatalf("StatusInProject: unexpected error: %v", err)
	}
	if resp.State != "RUNNING" {
		t.Errorf("StatusInProject: got State %q, want %q", resp.State, "RUNNING")
	}
}

// TestStatusInProject_NotFound tests that StatusInProject returns an error on 404 (run not found).
func TestStatusInProject_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/my-project/runs/bad-id/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, api.ErrorResponse{Error: "run not found: bad-id"})
	})

	_, c := newTestServer(t, mux)
	_, err := c.StatusInProject(context.Background(), "my-project", "bad-id")
	if err == nil {
		t.Fatal("StatusInProject: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("StatusInProject: error %q should mention 'not found'", err.Error())
	}
}

// TestOutputsInProject_Success tests that OutputsInProject sends GET /projects/{slug}/runs/{id}/outputs
// and returns OutputsResponse on 200.
func TestOutputsInProject_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/my-project/runs/run-abc-123/outputs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "wrong method", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, api.OutputsResponse{
			RunID:   "run-abc-123",
			Outputs: map[string]interface{}{"result": "hello"},
		})
	})

	_, c := newTestServer(t, mux)
	resp, err := c.OutputsInProject(context.Background(), "my-project", "run-abc-123")
	if err != nil {
		t.Fatalf("OutputsInProject: unexpected error: %v", err)
	}
	if resp.Outputs["result"] != "hello" {
		t.Errorf("OutputsInProject: got result %v, want %q", resp.Outputs["result"], "hello")
	}
}

// TestOutputsInProject_NotCompleted tests that OutputsInProject returns a descriptive error on 409.
func TestOutputsInProject_NotCompleted(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/my-project/runs/run-abc-123/outputs", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusConflict, api.ErrorResponse{Error: "run not completed (state: RUNNING)"})
	})

	_, c := newTestServer(t, mux)
	_, err := c.OutputsInProject(context.Background(), "my-project", "run-abc-123")
	if err == nil {
		t.Fatal("OutputsInProject: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not yet completed") {
		t.Errorf("OutputsInProject: error %q should contain 'not yet completed'", err.Error())
	}
}

// TestOutputsInProject_NotFound tests that OutputsInProject returns an error on 404.
func TestOutputsInProject_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/my-project/runs/bad-id/outputs", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, api.ErrorResponse{Error: "run not found: bad-id"})
	})

	_, c := newTestServer(t, mux)
	_, err := c.OutputsInProject(context.Background(), "my-project", "bad-id")
	if err == nil {
		t.Fatal("OutputsInProject: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("OutputsInProject: error %q should mention 'not found'", err.Error())
	}
}

// TestConnectionRefused tests that connection refused produces a clear error message.
func TestConnectionRefused(t *testing.T) {
	// Use a server URL that nothing is listening on.
	c := client.New("http://127.0.0.1:19999")
	_, err := c.Status(context.Background(), "any-run")
	if err == nil {
		t.Fatal("Status: expected connection error, got nil")
	}
	// The error should mention the failure, not be empty.
	if err.Error() == "" {
		t.Error("Status: connection error message should not be empty")
	}
}
