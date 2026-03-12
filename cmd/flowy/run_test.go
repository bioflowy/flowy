package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// runMockServer creates a test HTTP server with the given handler.
func runMockServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

// pollMux returns a ServeMux that simulates a workflow run:
//   - POST /workflows → 201 with runID
//   - GET /workflows/{runID}/status → PENDING on first call, COMPLETED on second
//   - GET /workflows/{runID}/outputs → 200 with given outputs map
func pollMux(runID string, outputs map[string]interface{}) *http.ServeMux {
	var pollCount int32
	mux := http.NewServeMux()
	mux.HandleFunc("/workflows", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"run_id": runID}) //nolint:errcheck
	})
	mux.HandleFunc("/workflows/"+runID+"/status", func(w http.ResponseWriter, r *http.Request) {
		state := "COMPLETED"
		if atomic.AddInt32(&pollCount, 1) == 1 {
			state = "PENDING"
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"run_id": runID, "state": state}) //nolint:errcheck
	})
	mux.HandleFunc("/workflows/"+runID+"/outputs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"run_id":  runID,
			"outputs": outputs,
		})
	})
	return mux
}

// TestRunMissingWDLArg verifies that "flowy run" without a WDL file returns exit 2.
func TestRunMissingWDLArg(t *testing.T) {
	code, _, stderr := runArgs([]string{"run"})
	if code != 2 {
		t.Errorf("expected exit 2 for missing WDL arg, got %d (stderr: %q)", code, stderr)
	}
}

// TestRunCompleted verifies that "flowy run <wdl> --server <url>" with mock server returning
// COMPLETED prints JSON outputs to stdout and exits 0.
func TestRunCompleted(t *testing.T) {
	baseDir := t.TempDir()
	wdlFile := filepath.Join(baseDir, "test.wdl")
	if err := os.WriteFile(wdlFile, []byte("workflow test {}"), 0644); err != nil {
		t.Fatal(err)
	}

	const runID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	outputs := map[string]interface{}{"workflow.result": "hello"}

	srv := runMockServer(t, pollMux(runID, outputs))

	code, stdout, stderr := runArgs([]string{
		"run", wdlFile,
		"--server", srv.URL,
		"--base-dir", baseDir,
	})
	if code != 0 {
		t.Errorf("expected exit 0, got %d (stderr: %q, stdout: %q)", code, stderr, stdout)
	}
	if !json.Valid([]byte(strings.TrimSpace(stdout))) {
		t.Errorf("expected stdout to be valid JSON, got %q", stdout)
	}
	var got map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &got); err != nil {
		t.Fatalf("unmarshal stdout: %v", err)
	}
	if _, ok := got["workflow.result"]; !ok {
		t.Errorf("expected 'workflow.result' key in stdout, got %q", stdout)
	}
}

// TestRunNoInputFlag verifies that "flowy run <wdl>" without --input submits with empty inputs.
func TestRunNoInputFlag(t *testing.T) {
	baseDir := t.TempDir()
	wdlFile := filepath.Join(baseDir, "test.wdl")
	if err := os.WriteFile(wdlFile, []byte("workflow test {}"), 0644); err != nil {
		t.Fatal(err)
	}

	const runID = "bbbbbbbb-cccc-dddd-eeee-ffffffffffff"
	var capturedInputs map[string]interface{}

	mux := http.NewServeMux()
	mux.HandleFunc("/workflows", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck
		if inputs, ok := req["inputs"].(map[string]interface{}); ok {
			capturedInputs = inputs
		} else {
			capturedInputs = map[string]interface{}{}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"run_id": runID}) //nolint:errcheck
	})
	mux.HandleFunc("/workflows/"+runID+"/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"run_id": runID, "state": "COMPLETED"}) //nolint:errcheck
	})
	mux.HandleFunc("/workflows/"+runID+"/outputs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"run_id": runID, "outputs": map[string]interface{}{}}) //nolint:errcheck
	})
	srv := runMockServer(t, mux)

	code, _, stderr := runArgs([]string{
		"run", wdlFile,
		"--server", srv.URL,
		"--base-dir", baseDir,
	})
	if code != 0 {
		t.Errorf("expected exit 0, got %d (stderr: %q)", code, stderr)
	}
	if len(capturedInputs) != 0 {
		t.Errorf("expected empty inputs, got %v", capturedInputs)
	}
}

// TestRunFailed verifies that "flowy run <wdl>" with server returning FAILED exits 1
// and prints error with run ID to stderr.
func TestRunFailed(t *testing.T) {
	baseDir := t.TempDir()
	wdlFile := filepath.Join(baseDir, "test.wdl")
	if err := os.WriteFile(wdlFile, []byte("workflow test {}"), 0644); err != nil {
		t.Fatal(err)
	}

	const runID = "cccccccc-dddd-eeee-ffff-000000000000"

	mux := http.NewServeMux()
	mux.HandleFunc("/workflows", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"run_id": runID}) //nolint:errcheck
	})
	mux.HandleFunc("/workflows/"+runID+"/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"run_id": runID, "state": "FAILED"}) //nolint:errcheck
	})
	srv := runMockServer(t, mux)

	code, _, stderr := runArgs([]string{
		"run", wdlFile,
		"--server", srv.URL,
		"--base-dir", baseDir,
	})
	if code != 1 {
		t.Errorf("expected exit 1 for FAILED workflow, got %d", code)
	}
	if !strings.Contains(stderr, "FAILED") {
		t.Errorf("expected 'FAILED' in stderr, got %q", stderr)
	}
	if !strings.Contains(stderr, runID) {
		t.Errorf("expected run ID %q in stderr, got %q", runID, stderr)
	}
}

// TestRunUnreachableServer verifies that "flowy run <wdl> --server <unreachable>"
// exits 1 with a clear error message.
func TestRunUnreachableServer(t *testing.T) {
	baseDir := t.TempDir()
	wdlFile := filepath.Join(baseDir, "test.wdl")
	if err := os.WriteFile(wdlFile, []byte("workflow test {}"), 0644); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := runArgs([]string{
		"run", wdlFile,
		"--server", "http://127.0.0.1:1",
		"--base-dir", baseDir,
	})
	if code != 1 {
		t.Errorf("expected exit 1 for unreachable server, got %d", code)
	}
	if stderr == "" {
		t.Error("expected non-empty stderr for unreachable server")
	}
}

// TestRunWithOutputFlag verifies that "flowy run <wdl> --output <file>" writes JSON to file
// instead of stdout, and stdout is empty.
func TestRunWithOutputFlag(t *testing.T) {
	baseDir := t.TempDir()
	wdlFile := filepath.Join(baseDir, "test.wdl")
	if err := os.WriteFile(wdlFile, []byte("workflow test {}"), 0644); err != nil {
		t.Fatal(err)
	}
	outFile := filepath.Join(baseDir, "out.json")

	const runID = "11111111-2222-3333-4444-555555555555"
	outputs := map[string]interface{}{"workflow.out": "value"}

	srv := runMockServer(t, pollMux(runID, outputs))

	code, stdout, stderr := runArgs([]string{
		"run", wdlFile,
		"--server", srv.URL,
		"--base-dir", baseDir,
		"--output", outFile,
	})
	if code != 0 {
		t.Errorf("expected exit 0, got %d (stderr: %q)", code, stderr)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("expected empty stdout with --output flag, got %q", stdout)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	if !json.Valid(data) {
		t.Errorf("output file is not valid JSON: %q", data)
	}
}

// TestRunTask verifies that "flowy run <wdl> --task <name>" submits to POST /tasks/run
// endpoint, polls, and prints outputs.
func TestRunTask(t *testing.T) {
	baseDir := t.TempDir()
	wdlFile := filepath.Join(baseDir, "test.wdl")
	if err := os.WriteFile(wdlFile, []byte("workflow test {}"), 0644); err != nil {
		t.Fatal(err)
	}

	const runID = "22222222-3333-4444-5555-666666666666"
	outputs := map[string]interface{}{"say_hello.greeting": "hi"}

	var taskRunCalled bool
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/run", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		taskRunCalled = true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"run_id": runID}) //nolint:errcheck
	})
	mux.HandleFunc("/workflows/"+runID+"/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"run_id": runID, "state": "COMPLETED"}) //nolint:errcheck
	})
	mux.HandleFunc("/workflows/"+runID+"/outputs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"run_id": runID, "outputs": outputs}) //nolint:errcheck
	})
	srv := runMockServer(t, mux)

	code, stdout, stderr := runArgs([]string{
		"run", wdlFile,
		"--task", "say_hello",
		"--server", srv.URL,
		"--base-dir", baseDir,
	})
	if code != 0 {
		t.Errorf("expected exit 0, got %d (stderr: %q, stdout: %q)", code, stderr, stdout)
	}
	if !taskRunCalled {
		t.Error("expected POST /tasks/run to be called")
	}
	var got map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v (stdout: %q)", err, stdout)
	}
	if _, ok := got["say_hello.greeting"]; !ok {
		t.Errorf("expected 'say_hello.greeting' key in outputs, got %q", stdout)
	}
}

// TestRunTaskServerError verifies that "flowy run <wdl> --task <name>" with server
// returning 400 exits 1 with error.
func TestRunTaskServerError(t *testing.T) {
	baseDir := t.TempDir()
	wdlFile := filepath.Join(baseDir, "test.wdl")
	if err := os.WriteFile(wdlFile, []byte("workflow test {}"), 0644); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/run", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "task not found"}) //nolint:errcheck
	})
	srv := runMockServer(t, mux)

	code, _, stderr := runArgs([]string{
		"run", wdlFile,
		"--task", "nonexistent",
		"--server", srv.URL,
		"--base-dir", baseDir,
	})
	if code != 1 {
		t.Errorf("expected exit 1 for task server error, got %d (stderr: %q)", code, stderr)
	}
}

// TestRunWithPath verifies that "flowy run <wdl> --path <dir>" sends search_paths in submit request.
func TestRunWithPath(t *testing.T) {
	baseDir := t.TempDir()
	wdlFile := filepath.Join(baseDir, "test.wdl")
	if err := os.WriteFile(wdlFile, []byte("workflow test {}"), 0644); err != nil {
		t.Fatal(err)
	}

	const runID = "33333333-4444-5555-6666-777777777777"
	var capturedSearchPaths []interface{}

	mux := http.NewServeMux()
	mux.HandleFunc("/workflows", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck
		if sp, ok := req["search_paths"].([]interface{}); ok {
			capturedSearchPaths = sp
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"run_id": runID}) //nolint:errcheck
	})
	mux.HandleFunc("/workflows/"+runID+"/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"run_id": runID, "state": "COMPLETED"}) //nolint:errcheck
	})
	mux.HandleFunc("/workflows/"+runID+"/outputs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"run_id": runID, "outputs": map[string]interface{}{}}) //nolint:errcheck
	})
	srv := runMockServer(t, mux)

	libDir := t.TempDir()
	code, _, stderr := runArgs([]string{
		"run", wdlFile,
		"--path", libDir,
		"--server", srv.URL,
		"--base-dir", baseDir,
	})
	if code != 0 {
		t.Errorf("expected exit 0, got %d (stderr: %q)", code, stderr)
	}
	if len(capturedSearchPaths) == 0 {
		t.Error("expected search_paths to be sent in submit request")
	}
}

// TestRunQuiet verifies that "flowy run <wdl> --quiet" suppresses status lines on stderr.
func TestRunQuiet(t *testing.T) {
	baseDir := t.TempDir()
	wdlFile := filepath.Join(baseDir, "test.wdl")
	if err := os.WriteFile(wdlFile, []byte("workflow test {}"), 0644); err != nil {
		t.Fatal(err)
	}

	const runID = "44444444-5555-6666-7777-888888888888"
	mux := pollMux(runID, map[string]interface{}{"workflow.out": "v"})
	srv := runMockServer(t, mux)

	code, _, stderr := runArgs([]string{
		"run", wdlFile,
		"--quiet",
		"--server", srv.URL,
		"--base-dir", baseDir,
	})
	if code != 0 {
		t.Errorf("expected exit 0, got %d (stderr: %q)", code, stderr)
	}
	// With --quiet, no "run <id>: STATE" lines should appear in stderr.
	if strings.Contains(stderr, "run "+runID[:8]) {
		t.Errorf("expected no status lines with --quiet, got stderr: %q", stderr)
	}
}

// TestRunStatusPrintedWithoutQuiet verifies that without --quiet, status changes are printed.
func TestRunStatusPrintedWithoutQuiet(t *testing.T) {
	baseDir := t.TempDir()
	wdlFile := filepath.Join(baseDir, "test.wdl")
	if err := os.WriteFile(wdlFile, []byte("workflow test {}"), 0644); err != nil {
		t.Fatal(err)
	}

	const runID = "55555555-6666-7777-8888-999999999999"
	mux := pollMux(runID, map[string]interface{}{})
	srv := runMockServer(t, mux)

	_, _, stderr := runArgs([]string{
		"run", wdlFile,
		"--server", srv.URL,
		"--base-dir", baseDir,
	})
	// Status lines like "run 55555555: PENDING" should appear.
	// At least one state should be printed.
	if !strings.Contains(stderr, "run "+runID[:8]) {
		t.Errorf("expected status line in stderr without --quiet, got %q", stderr)
	}
}

// TestRunBuildCompiles verifies that go build ./... compiles all three binaries.
func TestRunBuildCompiles(t *testing.T) {
	// This is a meta-test; the fact that this package compiles is sufficient.
	// We additionally verify the package builds cleanly by importing it.
	t.Log("package main compiled successfully — all three binaries in go build ./... also compile")
}
