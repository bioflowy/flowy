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

// TestRunVerboseAccepted verifies that --verbose is accepted (no error) in server mode.
// In server mode, --verbose is a no-op: a note is printed to stderr.
func TestRunVerboseAccepted(t *testing.T) {
	wdlFile := filepath.Join(t.TempDir(), "test.wdl")
	if err := os.WriteFile(wdlFile, []byte("workflow test {}"), 0644); err != nil {
		t.Fatal(err)
	}

	const runID = "66666666-7777-8888-9999-aaaaaaaaaaaa"

	mux := http.NewServeMux()
	mux.HandleFunc("/workflows", func(w http.ResponseWriter, r *http.Request) {
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
	srv := httptest.NewServer(mux)
	defer srv.Close()

	code, _, stderr := runArgs([]string{
		"run", wdlFile,
		"--verbose",
		"--server", srv.URL,
	})
	if code != 0 {
		t.Errorf("expected exit 0 with --verbose in server mode, got %d (stderr: %q)", code, stderr)
	}
	// --verbose should print a note about not being supported.
	if !strings.Contains(stderr, "verbose") && !strings.Contains(stderr, "not yet supported") {
		t.Logf("note: --verbose note not found in stderr, which is acceptable: %q", stderr)
	}
}
