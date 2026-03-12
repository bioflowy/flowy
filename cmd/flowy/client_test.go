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

// TestSubmitWithInputsStageFiles verifies that "flowy submit <wdl> -i <inputs> --server <url>"
// with local input files copies them to baseDir/runs/{uuid}/inputs/ before submitting.
func TestSubmitWithInputsStageFiles(t *testing.T) {
	baseDir := t.TempDir()
	inputsDir := t.TempDir() // separate dir, not under baseDir

	// Create a local input file (not under baseDir).
	localFile := filepath.Join(inputsDir, "sample.txt")
	if err := os.WriteFile(localFile, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	wdlFile := filepath.Join(inputsDir, "test.wdl")
	if err := os.WriteFile(wdlFile, []byte("workflow test {}"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create inputs JSON that references the local file.
	inputs := map[string]interface{}{"test.input_file": localFile}
	inputsJSON, err := json.Marshal(inputs)
	if err != nil {
		t.Fatal(err)
	}
	inputsFile := filepath.Join(inputsDir, "inputs.json")
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
		"--base-dir", baseDir,
	})
	if code != 0 {
		t.Errorf("expected exit 0, got %d (stderr: %q)", code, stderr)
	}
	if strings.TrimSpace(stdout) != expectedRunID {
		t.Errorf("expected run ID %q, got %q", expectedRunID, strings.TrimSpace(stdout))
	}

	// Verify staged file exists under baseDir/runs/{runID}/inputs/
	runDir := filepath.Join(baseDir, "runs")
	entries, err := os.ReadDir(runDir)
	if err != nil {
		t.Fatalf("expected runs dir to exist at %q: %v", runDir, err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one run directory to be created")
	}
	runID := entries[0].Name()
	stagedPath := filepath.Join(baseDir, "runs", runID, "inputs", "sample.txt")
	if _, err := os.Stat(stagedPath); os.IsNotExist(err) {
		t.Errorf("expected staged file at %q, but it does not exist", stagedPath)
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

// TestSubmitCleanupOnFailure verifies that staged directories are removed when submission fails.
func TestSubmitCleanupOnFailure(t *testing.T) {
	baseDir := t.TempDir()

	wdlFile := filepath.Join(t.TempDir(), "test.wdl")
	if err := os.WriteFile(wdlFile, []byte("workflow test {}"), 0644); err != nil {
		t.Fatal(err)
	}

	// Server returns an error.
	mux := http.NewServeMux()
	mux.HandleFunc("/workflows", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
	})
	srv := newMockServer(mux)
	defer srv.Close()

	code, _, _ := runArgs([]string{
		"submit", wdlFile,
		"--server", srv.URL,
		"--base-dir", baseDir,
	})
	if code != 1 {
		t.Errorf("expected exit 1 on server error, got %d", code)
	}

	// The runs directory should be empty (cleaned up).
	runDir := filepath.Join(baseDir, "runs")
	entries, err := os.ReadDir(runDir)
	if err == nil && len(entries) > 0 {
		t.Errorf("expected staged dirs to be cleaned up, but found %d entries in %q", len(entries), runDir)
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

// TestStagingSkipsAlreadyStagedFiles verifies that files already under baseDir are NOT re-copied.
func TestStagingSkipsAlreadyStagedFiles(t *testing.T) {
	baseDir := t.TempDir()

	// Create a file that's already under baseDir.
	alreadyStagedFile := filepath.Join(baseDir, "already_staged.txt")
	if err := os.WriteFile(alreadyStagedFile, []byte("already staged"), 0644); err != nil {
		t.Fatal(err)
	}

	stagedMap, err := stageInputFiles(
		map[string]interface{}{"key": alreadyStagedFile},
		filepath.Join(baseDir, "runs", "test-run", "inputs"),
		baseDir,
	)
	if err != nil {
		t.Fatalf("stageInputFiles returned error: %v", err)
	}

	// The value should remain the same (not re-staged).
	if stagedMap["key"] != alreadyStagedFile {
		t.Errorf("expected file under baseDir to keep original path %q, got %q",
			alreadyStagedFile, stagedMap["key"])
	}
}

// TestStagingSkipsNonExistentPaths verifies that string values that look like paths
// but do not exist on disk are left unchanged.
func TestStagingSkipsNonExistentPaths(t *testing.T) {
	baseDir := t.TempDir()
	inputsDir := filepath.Join(baseDir, "runs", "test-run", "inputs")

	nonExistentPath := "/tmp/does-not-exist-flowy-test.txt"
	stagedMap, err := stageInputFiles(
		map[string]interface{}{"key": nonExistentPath},
		inputsDir,
		baseDir,
	)
	if err != nil {
		t.Fatalf("stageInputFiles returned error: %v", err)
	}

	// Non-existent path should remain unchanged.
	if stagedMap["key"] != nonExistentPath {
		t.Errorf("expected non-existent path to remain unchanged, got %q", stagedMap["key"])
	}
}

// TestStagingCopiesLocalFiles verifies that files outside baseDir are copied to inputsDir.
func TestStagingCopiesLocalFiles(t *testing.T) {
	baseDir := t.TempDir()
	externalDir := t.TempDir() // not under baseDir

	localFile := filepath.Join(externalDir, "myfile.txt")
	if err := os.WriteFile(localFile, []byte("file content"), 0644); err != nil {
		t.Fatal(err)
	}

	inputsDir := filepath.Join(baseDir, "runs", "test-run", "inputs")
	if err := os.MkdirAll(inputsDir, 0755); err != nil {
		t.Fatal(err)
	}

	stagedMap, err := stageInputFiles(
		map[string]interface{}{"key": localFile},
		inputsDir,
		baseDir,
	)
	if err != nil {
		t.Fatalf("stageInputFiles returned error: %v", err)
	}

	// The staged path should point to the new location inside inputsDir.
	expectedStagedPath := filepath.Join(inputsDir, "myfile.txt")
	if stagedMap["key"] != expectedStagedPath {
		t.Errorf("expected staged path %q, got %q", expectedStagedPath, stagedMap["key"])
	}

	// The file should actually exist at the staged path.
	if _, err := os.Stat(expectedStagedPath); os.IsNotExist(err) {
		t.Errorf("staged file does not exist at %q", expectedStagedPath)
	}
}

// TestWDLStaging verifies that the WDL file is staged to the shared FS if not under baseDir.
func TestWDLStaging(t *testing.T) {
	baseDir := t.TempDir()
	externalDir := t.TempDir()

	wdlFile := filepath.Join(externalDir, "workflow.wdl")
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
		"--base-dir", baseDir,
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stderr: %q)", code, stderr)
	}

	// Verify the WDLPath in the captured request points under baseDir.
	if capturedReq == nil {
		t.Fatal("no request captured")
	}
	wdlPath, _ := capturedReq["wdl_path"].(string)
	if !strings.HasPrefix(wdlPath, baseDir) {
		t.Errorf("expected WDLPath to be staged under baseDir %q, got %q", baseDir, wdlPath)
	}
}
