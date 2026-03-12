// Tests for submit, status, and outputs subcommands using httptest mock servers.
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newMockServer creates a test HTTP server with the given handler mux.
func newMockServer(mux *http.ServeMux) *httptest.Server {
	return httptest.NewServer(mux)
}

// TestSubmitNoArgs verifies that "flowy submit" with no WDL arg returns exit 2 with usage message.
func TestSubmitNoArgs(t *testing.T) {
	code, _, stderr := runArgs([]string{"submit"})
	if code != 2 {
		t.Errorf("expected exit 2, got %d (stderr: %q)", code, stderr)
	}
	if !strings.Contains(stderr, "submit") {
		t.Errorf("expected usage message containing 'submit' in stderr, got %q", stderr)
	}
}

// TestSubmitSuccess verifies that "flowy submit <wdl> --server <url>" submits to mock server
// and prints run ID (plain UUID) to stdout.
func TestSubmitSuccess(t *testing.T) {
	wdlFile := filepath.Join(t.TempDir(), "test.wdl")
	if err := os.WriteFile(wdlFile, []byte("workflow test {}"), 0644); err != nil {
		t.Fatal(err)
	}

	const expectedRunID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

	mux := http.NewServeMux()
	mux.HandleFunc("/workflows", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"run_id": expectedRunID}) //nolint:errcheck
	})
	srv := newMockServer(mux)
	defer srv.Close()

	code, stdout, stderr := runArgs([]string{"submit", wdlFile, "--server", srv.URL})
	if code != 0 {
		t.Errorf("expected exit 0, got %d (stderr: %q)", code, stderr)
	}
	if strings.TrimSpace(stdout) != expectedRunID {
		t.Errorf("expected stdout to be plain run ID %q, got %q", expectedRunID, strings.TrimSpace(stdout))
	}
}

// TestSubmitWithInputs verifies that "flowy submit <wdl> -i <inputs> --server <url>"
// sends the input JSON to the server as-is (no staging or path rewriting).
func TestSubmitWithInputs(t *testing.T) {
	dir := t.TempDir()

	wdlFile := filepath.Join(dir, "test.wdl")
	if err := os.WriteFile(wdlFile, []byte("workflow test {}"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create inputs JSON with a file path.
	inputPath := filepath.Join(dir, "data", "sample.txt")
	inputs := map[string]interface{}{"test.input_file": inputPath}
	inputsJSON, err := json.Marshal(inputs)
	if err != nil {
		t.Fatal(err)
	}
	inputsFile := filepath.Join(dir, "inputs.json")
	if err := os.WriteFile(inputsFile, inputsJSON, 0644); err != nil {
		t.Fatal(err)
	}

	const expectedRunID = "11111111-2222-3333-4444-555555555555"

	var capturedReq map[string]interface{}
	mux := http.NewServeMux()
	mux.HandleFunc("/workflows", func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedReq) //nolint:errcheck
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"run_id": expectedRunID}) //nolint:errcheck
	})
	srv := newMockServer(mux)
	defer srv.Close()

	code, stdout, stderr := runArgs([]string{
		"submit", wdlFile,
		"-i", inputsFile,
		"--server", srv.URL,
	})
	if code != 0 {
		t.Errorf("expected exit 0, got %d (stderr: %q)", code, stderr)
	}
	if strings.TrimSpace(stdout) != expectedRunID {
		t.Errorf("expected run ID %q, got %q", expectedRunID, strings.TrimSpace(stdout))
	}

	// Verify the input path was sent as-is (not rewritten).
	if capturedReq == nil {
		t.Fatal("no request captured")
	}
	capturedInputs, ok := capturedReq["inputs"].(map[string]interface{})
	if !ok {
		t.Fatal("expected inputs in captured request")
	}
	if capturedInputs["test.input_file"] != inputPath {
		t.Errorf("expected input path %q to be passed as-is, got %q", inputPath, capturedInputs["test.input_file"])
	}
}

// TestSubmitUnreachableServer verifies that "flowy submit" with an unreachable server
// returns exit 1 with a clear error message.
func TestSubmitUnreachableServer(t *testing.T) {
	wdlFile := filepath.Join(t.TempDir(), "test.wdl")
	if err := os.WriteFile(wdlFile, []byte("workflow test {}"), 0644); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := runArgs([]string{"submit", wdlFile, "--server", "http://127.0.0.1:1"})
	if code != 1 {
		t.Errorf("expected exit 1 for unreachable server, got %d", code)
	}
	if stderr == "" {
		t.Error("expected non-empty stderr for unreachable server")
	}
}

// TestSubmitWDLPathIsAbsolute verifies that the WDL path sent to the server is absolute.
func TestSubmitWDLPathIsAbsolute(t *testing.T) {
	dir := t.TempDir()

	wdlFile := filepath.Join(dir, "workflow.wdl")
	if err := os.WriteFile(wdlFile, []byte("workflow test {}"), 0644); err != nil {
		t.Fatal(err)
	}

	const expectedRunID = "cccccccc-dddd-eeee-ffff-000000000000"

	var capturedReq map[string]interface{}
	mux := http.NewServeMux()
	mux.HandleFunc("/workflows", func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedReq) //nolint:errcheck
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"run_id": expectedRunID}) //nolint:errcheck
	})
	srv := newMockServer(mux)
	defer srv.Close()

	code, _, stderr := runArgs([]string{
		"submit", wdlFile,
		"--server", srv.URL,
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stderr: %q)", code, stderr)
	}

	// Verify the WDLPath in the captured request is absolute.
	if capturedReq == nil {
		t.Fatal("no request captured")
	}
	wdlPath, _ := capturedReq["wdl_path"].(string)
	if !filepath.IsAbs(wdlPath) {
		t.Errorf("expected absolute WDL path, got %q", wdlPath)
	}
}

// TestStatusSuccess verifies that "flowy status <run-id> --server <url>" prints state string to stdout.
func TestStatusSuccess(t *testing.T) {
	const runID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	const expectedState = "RUNNING"

	mux := http.NewServeMux()
	mux.HandleFunc("/workflows/"+runID+"/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"run_id": runID, "state": expectedState}) //nolint:errcheck
	})
	srv := newMockServer(mux)
	defer srv.Close()

	code, stdout, stderr := runArgs([]string{"status", runID, "--server", srv.URL})
	if code != 0 {
		t.Errorf("expected exit 0, got %d (stderr: %q)", code, stderr)
	}
	if strings.TrimSpace(stdout) != expectedState {
		t.Errorf("expected state %q, got %q", expectedState, strings.TrimSpace(stdout))
	}
}

// TestStatusNoArgs verifies that "flowy status" without run-id returns exit 2.
func TestStatusNoArgs(t *testing.T) {
	code, _, stderr := runArgs([]string{"status"})
	if code != 2 {
		t.Errorf("expected exit 2, got %d (stderr: %q)", code, stderr)
	}
}

// TestOutputsSuccess verifies that "flowy outputs <run-id> --server <url>" prints JSON outputs to stdout.
func TestOutputsSuccess(t *testing.T) {
	const runID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	expectedOutputs := map[string]interface{}{"workflow.result": "hello world"}

	mux := http.NewServeMux()
	mux.HandleFunc("/workflows/"+runID+"/outputs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"run_id":  runID,
			"outputs": expectedOutputs,
		})
	})
	srv := newMockServer(mux)
	defer srv.Close()

	code, stdout, stderr := runArgs([]string{"outputs", runID, "--server", srv.URL})
	if code != 0 {
		t.Errorf("expected exit 0, got %d (stderr: %q)", code, stderr)
	}
	// Verify output is valid JSON containing the expected key.
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Errorf("expected valid JSON stdout, got %q (err: %v)", stdout, err)
	}
	if _, ok := parsed["workflow.result"]; !ok {
		t.Errorf("expected output key 'workflow.result' in stdout, got %q", stdout)
	}
}

// TestOutputsNotCompleted verifies that "flowy outputs" on a non-completed run (409)
// prints a descriptive error to stderr and exits 1.
func TestOutputsNotCompleted(t *testing.T) {
	const runID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

	mux := http.NewServeMux()
	mux.HandleFunc("/workflows/"+runID+"/outputs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": "RUNNING"}) //nolint:errcheck
	})
	srv := newMockServer(mux)
	defer srv.Close()

	code, _, stderr := runArgs([]string{"outputs", runID, "--server", srv.URL})
	if code != 1 {
		t.Errorf("expected exit 1 for not-completed run, got %d", code)
	}
	if !strings.Contains(strings.ToLower(stderr), "not yet completed") && !strings.Contains(strings.ToLower(stderr), "running") {
		t.Errorf("expected descriptive error about run not being completed, got %q", stderr)
	}
}
