package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bioflowy/flowy/internal/api"
)

// setupRegistryTest creates a mock server with the given handler, writes credentials.yaml
// to a temp HOME, sets HOME env var, and returns the server URL and tmpHome.
func setupRegistryTest(t *testing.T, handler http.HandlerFunc) (serverURL, tmpHome string) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	tmpHome = t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Write credentials.yaml pointing to mock server.
	flowyDir := filepath.Join(tmpHome, ".flowy")
	if err := os.MkdirAll(flowyDir, 0700); err != nil {
		t.Fatalf("setup: mkdir .flowy: %v", err)
	}
	creds := "server: " + srv.URL + "\ntoken: test-token\n"
	if err := os.WriteFile(filepath.Join(flowyDir, "credentials.yaml"), []byte(creds), 0600); err != nil {
		t.Fatalf("setup: write credentials.yaml: %v", err)
	}

	return srv.URL, tmpHome
}

// writeRegistryJSON is a helper to write a JSON response in registry tests.
func writeRegistryJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// TestRegistryNoSubcommand tests that "flowy registry" with no subcommand prints usage and exits 2.
func TestRegistryNoSubcommand(t *testing.T) {
	var stdout, stderr strings.Builder
	code := runRegistry([]string{}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("runRegistry no args: got exit %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "Usage") {
		t.Errorf("runRegistry no args: stderr %q should contain 'Usage'", stderr.String())
	}
}

// TestRegistryUnknownSubcommand tests that unknown subcommand exits 2 with message.
func TestRegistryUnknownSubcommand(t *testing.T) {
	var stdout, stderr strings.Builder
	code := runRegistry([]string{"unknown"}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("runRegistry unknown: got exit %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown registry subcommand") {
		t.Errorf("runRegistry unknown: stderr %q should contain 'unknown registry subcommand'", stderr.String())
	}
}

// TestRegistryPushSuccess tests a successful push command.
func TestRegistryPushSuccess(t *testing.T) {
	_, _ = setupRegistryTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/myproj/registry" || r.Method != http.MethodPost {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "missing auth", http.StatusUnauthorized)
			return
		}
		writeRegistryJSON(w, http.StatusCreated, api.RegistryPushResponse{
			Name:      "hello",
			Digest:    "sha256:abc123",
			Kind:      "workflow",
			VersionID: "v1",
		})
	})

	// Write a temp WDL file.
	wdlFile := filepath.Join(t.TempDir(), "test.wdl")
	if err := os.WriteFile(wdlFile, []byte("version 1.2\nworkflow test {}"), 0644); err != nil {
		t.Fatalf("write wdl: %v", err)
	}

	var stdout, stderr strings.Builder
	code := runRegistry([]string{"push", "--project", "myproj", "--name", "hello", wdlFile}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("runRegistryPush: got exit %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Pushed hello") {
		t.Errorf("runRegistryPush: stdout %q should contain 'Pushed hello'", stdout.String())
	}
	if !strings.Contains(stdout.String(), "sha256:") {
		t.Errorf("runRegistryPush: stdout %q should contain digest 'sha256:'", stdout.String())
	}
}

// TestRegistryPushMissingFlags tests that push without required flags exits 2.
func TestRegistryPushMissingFlags(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	var stdout, stderr strings.Builder
	code := runRegistry([]string{"push", "--name", "hello", "test.wdl"}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("runRegistryPush no --project: got exit %d, want 2", code)
	}
}

// TestRegistryListSuccess tests a successful list command printing a table.
func TestRegistryListSuccess(t *testing.T) {
	_, _ = setupRegistryTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/myproj/registry" || r.Method != http.MethodGet {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		entries := []api.RegistryEntryResponse{
			{Name: "hello", Kind: "workflow", VersionCount: 3, Tags: []string{"latest", "v1.0"}},
			{Name: "greet", Kind: "task", VersionCount: 1, Tags: []string{"stable"}},
		}
		writeRegistryJSON(w, http.StatusOK, entries)
	})

	var stdout, stderr strings.Builder
	code := runRegistry([]string{"list", "--project", "myproj"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("runRegistryList: got exit %d, want 0; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"NAME", "KIND", "LATEST TAG", "VERSIONS", "hello", "workflow", "greet", "task"} {
		if !strings.Contains(out, want) {
			t.Errorf("runRegistryList: stdout %q should contain %q", out, want)
		}
	}
}

// TestRegistryPullSuccess tests a successful pull printing exact WDL content.
func TestRegistryPullSuccess(t *testing.T) {
	wdlContent := "version 1.2\nworkflow test {}"
	_, _ = setupRegistryTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/myproj/registry/hello" || r.Method != http.MethodGet {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		writeRegistryJSON(w, http.StatusOK, api.RegistryPullResponse{
			Name:    "hello",
			Digest:  "sha256:abc123",
			Kind:    "workflow",
			Content: wdlContent,
		})
	})

	var stdout, stderr strings.Builder
	code := runRegistry([]string{"pull", "--project", "myproj", "--name", "hello", "--tag", "latest"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("runRegistryPull: got exit %d, want 0; stderr: %s", code, stderr.String())
	}
	// The content should be printed exactly — no extra newline.
	if stdout.String() != wdlContent {
		t.Errorf("runRegistryPull: stdout = %q, want %q (exact, no extra newline)", stdout.String(), wdlContent)
	}
}

// TestRegistryPullMissingTagDigest tests that pull without --tag or --digest exits 2.
func TestRegistryPullMissingTagDigest(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	var stdout, stderr strings.Builder
	code := runRegistry([]string{"pull", "--project", "myproj", "--name", "hello"}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("runRegistryPull no tag: got exit %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "must specify") {
		t.Errorf("runRegistryPull no tag: stderr %q should contain 'must specify'", stderr.String())
	}
}

// TestRegistryTagSuccess tests a successful tag command.
func TestRegistryTagSuccess(t *testing.T) {
	_, _ = setupRegistryTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/myproj/registry/hello/tags" || r.Method != http.MethodPost {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	var stdout, stderr strings.Builder
	code := runRegistry([]string{"tag", "--project", "myproj", "--name", "hello", "--tag", "v1.0", "--digest", "sha256:abc123"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("runRegistryTag: got exit %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Tagged") {
		t.Errorf("runRegistryTag: stdout %q should contain 'Tagged'", stdout.String())
	}
}

// TestRegistryDeleteTagSuccess tests a successful delete-tag command.
func TestRegistryDeleteTagSuccess(t *testing.T) {
	_, _ = setupRegistryTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/myproj/registry/hello/tags/v1.0" || r.Method != http.MethodDelete {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	var stdout, stderr strings.Builder
	code := runRegistry([]string{"delete-tag", "--project", "myproj", "--name", "hello", "--tag", "v1.0"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("runRegistryDeleteTag: got exit %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Deleted tag") {
		t.Errorf("runRegistryDeleteTag: stdout %q should contain 'Deleted tag'", stdout.String())
	}
}

// TestRegistryDeleteSuccess tests a successful delete entry command.
func TestRegistryDeleteSuccess(t *testing.T) {
	_, _ = setupRegistryTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/myproj/registry/hello" || r.Method != http.MethodDelete {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	var stdout, stderr strings.Builder
	code := runRegistry([]string{"delete", "--project", "myproj", "--name", "hello"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("runRegistryDelete: got exit %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Deleted hello") {
		t.Errorf("runRegistryDelete: stdout %q should contain 'Deleted hello'", stdout.String())
	}
}

// TestRegistryMissingCredentials tests that missing credentials file gives human-readable error.
func TestRegistryMissingCredentials(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	// No credentials.yaml written.

	var stdout, stderr strings.Builder
	code := runRegistry([]string{"list", "--project", "myproj"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("runRegistry missing creds: got exit %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "Not authenticated") {
		t.Errorf("runRegistry missing creds: stderr %q should contain 'Not authenticated'", stderr.String())
	}
}

// TestRegistryUnauthorized tests that a 401 from server gives token-expired message.
func TestRegistryUnauthorized(t *testing.T) {
	_, _ = setupRegistryTest(t, func(w http.ResponseWriter, r *http.Request) {
		writeRegistryJSON(w, http.StatusUnauthorized, api.ErrorResponse{Error: "unauthorized"})
	})

	var stdout, stderr strings.Builder
	code := runRegistry([]string{"list", "--project", "myproj"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("runRegistry 401: got exit %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "Token expired or invalid") {
		t.Errorf("runRegistry 401: stderr %q should contain 'Token expired or invalid'", stderr.String())
	}
}

// TestRegistryForbidden tests that a 403 from server gives permission-denied message.
func TestRegistryForbidden(t *testing.T) {
	_, _ = setupRegistryTest(t, func(w http.ResponseWriter, r *http.Request) {
		writeRegistryJSON(w, http.StatusForbidden, api.ErrorResponse{Error: "forbidden"})
	})

	var stdout, stderr strings.Builder
	code := runRegistry([]string{"list", "--project", "myproj"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("runRegistry 403: got exit %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "Permission denied") {
		t.Errorf("runRegistry 403: stderr %q should contain 'Permission denied'", stderr.String())
	}
}
