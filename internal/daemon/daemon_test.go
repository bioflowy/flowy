// Package daemon tests exercise the daemon's HTTP client methods and lifecycle.
// Tests use httptest.NewServer to mock the flowy-server API without real Docker.
package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bioflowy/flowy/internal/api"
	"github.com/bioflowy/flowy/internal/config"
	"github.com/bioflowy/flowy/wdl/values"
)

// newTestDaemon creates a Daemon configured to talk to the given test server URL.
func newTestDaemon(t *testing.T, serverURL string) *Daemon {
	t.Helper()
	cfg := &config.Config{}
	// Apply defaults then override server URL.
	cfg.Daemon.ServerURL = serverURL
	cfg.Daemon.HeartbeatSec = 30
	cfg.Daemon.Workers = 1
	return New(cfg, "test-daemon-1234")
}

// TestClaimTask_Success verifies that claimTask parses a 200 TaskClaimResponse correctly.
func TestClaimTask_Success(t *testing.T) {
	want := api.TaskClaimResponse{
		TaskID:     "42",
		RunID:      "run-abc",
		TaskName:   "hello",
		ShardIndex: 0,
		InputsDir:  "/data/inputs",
		OutputsDir: "/data/outputs",
		WDLPath:    "/wdl/hello.wdl",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/tasks/claim" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(want); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	d := newTestDaemon(t, srv.URL)
	got, err := d.claimTask(context.Background())
	if err != nil {
		t.Fatalf("claimTask returned error: %v", err)
	}
	if got == nil {
		t.Fatal("claimTask returned nil claim, expected non-nil")
	}
	if got.TaskID != want.TaskID {
		t.Errorf("TaskID: got %q, want %q", got.TaskID, want.TaskID)
	}
	if got.RunID != want.RunID {
		t.Errorf("RunID: got %q, want %q", got.RunID, want.RunID)
	}
	if got.TaskName != want.TaskName {
		t.Errorf("TaskName: got %q, want %q", got.TaskName, want.TaskName)
	}
	if got.InputsDir != want.InputsDir {
		t.Errorf("InputsDir: got %q, want %q", got.InputsDir, want.InputsDir)
	}
	if got.OutputsDir != want.OutputsDir {
		t.Errorf("OutputsDir: got %q, want %q", got.OutputsDir, want.OutputsDir)
	}
	if got.WDLPath != want.WDLPath {
		t.Errorf("WDLPath: got %q, want %q", got.WDLPath, want.WDLPath)
	}
}

// TestClaimTask_NoContent verifies that a 204 response returns (nil, nil).
func TestClaimTask_NoContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	d := newTestDaemon(t, srv.URL)
	claim, err := d.claimTask(context.Background())
	if err != nil {
		t.Fatalf("claimTask returned error: %v", err)
	}
	if claim != nil {
		t.Errorf("claimTask returned non-nil claim on 204: %+v", claim)
	}
}

// TestClaimTask_Error verifies that a 500 response returns an error.
func TestClaimTask_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	d := newTestDaemon(t, srv.URL)
	claim, err := d.claimTask(context.Background())
	if err == nil {
		t.Fatal("claimTask returned nil error on 500, expected non-nil")
	}
	if claim != nil {
		t.Errorf("claimTask returned non-nil claim on error: %+v", claim)
	}
}

// TestSendHeartbeat verifies that sendHeartbeat POSTs to /heartbeat with the correct DaemonID.
func TestSendHeartbeat(t *testing.T) {
	var gotReq api.HeartbeatRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/heartbeat" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("decode heartbeat body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := newTestDaemon(t, srv.URL)
	if err := d.sendHeartbeat(context.Background()); err != nil {
		t.Fatalf("sendHeartbeat returned error: %v", err)
	}
	if gotReq.DaemonID != d.daemonID {
		t.Errorf("DaemonID in heartbeat body: got %q, want %q", gotReq.DaemonID, d.daemonID)
	}
}

// TestReportResult verifies that reportResult POSTs to /tasks/{id}/result with the correct state.
func TestReportResult(t *testing.T) {
	const taskID = "123"
	var gotReq api.TaskResultRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/tasks/" + taskID + "/result"
		if r.Method != http.MethodPost || r.URL.Path != expectedPath {
			t.Errorf("unexpected request: %s %s (want POST %s)", r.Method, r.URL.Path, expectedPath)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("decode result body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := newTestDaemon(t, srv.URL)
	d.reportResult(context.Background(), taskID, "COMPLETED", "")
	if gotReq.State != "COMPLETED" {
		t.Errorf("State in result body: got %q, want %q", gotReq.State, "COMPLETED")
	}
	if gotReq.Error != "" {
		t.Errorf("Error field should be empty, got %q", gotReq.Error)
	}
}

// TestOutputsJSON verifies values.ValuesToJSON converts all WDL value types to correct Go types.
func TestOutputsJSON(t *testing.T) {
	inputs := map[string]values.Value{
		"str":     values.StringValue{Value: "hello"},
		"intVal":  values.IntValue{Value: 42},
		"flt":     values.FloatValue{Value: 3.14},
		"boolVal": values.BooleanValue{Value: true},
		"file":    values.FileValue{Value: "/path/to/file"},
		"null":    values.NullValue{},
		"arr":     values.ArrayValue{Items: []values.Value{values.StringValue{Value: "a"}, values.StringValue{Value: "b"}}},
	}

	result, err := values.ValuesToJSON(inputs)
	if err != nil {
		t.Fatalf("valuesToJSON returned error: %v", err)
	}

	if v, ok := result["str"].(string); !ok || v != "hello" {
		t.Errorf("str: got %T(%v), want string(hello)", result["str"], result["str"])
	}
	if v, ok := result["intVal"].(int64); !ok || v != 42 {
		t.Errorf("intVal: got %T(%v), want int64(42)", result["intVal"], result["intVal"])
	}
	if v, ok := result["flt"].(float64); !ok || v != 3.14 {
		t.Errorf("flt: got %T(%v), want float64(3.14)", result["flt"], result["flt"])
	}
	if v, ok := result["boolVal"].(bool); !ok || !v {
		t.Errorf("boolVal: got %T(%v), want bool(true)", result["boolVal"], result["boolVal"])
	}
	if v, ok := result["file"].(string); !ok || v != "/path/to/file" {
		t.Errorf("file: got %T(%v), want string(/path/to/file)", result["file"], result["file"])
	}
	if result["null"] != nil {
		t.Errorf("null: got %v, want nil", result["null"])
	}
	arr, ok := result["arr"].([]interface{})
	if !ok {
		t.Fatalf("arr: got %T, want []interface{}", result["arr"])
	}
	if len(arr) != 2 {
		t.Fatalf("arr len: got %d, want 2", len(arr))
	}
	if arr[0] != "a" || arr[1] != "b" {
		t.Errorf("arr values: got %v, want [a b]", arr)
	}
}

// TestDaemonRunShutdown verifies that a daemon with no tasks exits cleanly when context is cancelled.
func TestDaemonRunShutdown(t *testing.T) {
	// Mock server: always return 204 for /tasks/claim, 200 for /heartbeat.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tasks/claim":
			w.WriteHeader(http.StatusNoContent)
		case "/heartbeat":
			w.WriteHeader(http.StatusOK)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{}
	cfg.Daemon.ServerURL = srv.URL
	cfg.Daemon.HeartbeatSec = 1
	cfg.Daemon.Workers = 1
	d := New(cfg, "test-shutdown-5678")

	ctx, cancel := context.WithCancel(context.Background())

	runDone := make(chan error, 1)
	go func() {
		runDone <- d.Run(ctx)
	}()

	// Let the daemon tick at least once by waiting a moment, then cancel.
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Wait for Run to finish with a generous timeout.
	select {
	case err := <-runDone:
		if err != nil {
			t.Errorf("Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("daemon Run did not exit within 5 seconds after context cancel")
	}

	// Shutdown should return quickly (no in-flight tasks).
	shutdownDone := make(chan struct{})
	go func() {
		d.Shutdown()
		close(shutdownDone)
	}()
	select {
	case <-shutdownDone:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("Shutdown did not return within 2 seconds")
	}
}
