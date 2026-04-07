package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// loginRunArgs calls run() with the given args and a fake stdin, capturing stdout/stderr.
func loginRunArgs(args []string, stdinData string) (code int, stdout, stderr string) {
	var outBuf, errBuf strings.Builder
	// Override readPasswordFn so no real terminal is needed.
	old := readPasswordFn
	readPasswordFn = func() ([]byte, error) { return []byte("testpass"), nil }
	defer func() { readPasswordFn = old }()

	code = run(args, strings.NewReader(stdinData), &outBuf, &errBuf)
	return code, outBuf.String(), errBuf.String()
}

// TestLoginSuccess verifies that a successful login prints "Login successful."
// and writes credentials to the file.
func TestLoginSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/auth/login" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"token":"jwt-test-token"}`)
	}))
	defer srv.Close()

	code, stdout, stderr := loginRunArgs([]string{"login", "--server", srv.URL}, "testuser\n")
	if code != 0 {
		t.Errorf("expected exit 0, got %d (stderr: %q)", code, stderr)
	}
	if !strings.Contains(stdout, "Login successful.") {
		t.Errorf("expected stdout to contain 'Login successful.', got %q", stdout)
	}

	// Verify credentials file was written.
	credsPath := filepath.Join(tmpDir, ".flowy", "credentials.yaml")
	data, err := os.ReadFile(credsPath)
	if err != nil {
		t.Fatalf("credentials file not found: %v", err)
	}
	var creds struct {
		Server string `yaml:"server"`
		Token  string `yaml:"token"`
	}
	if err := yaml.Unmarshal(data, &creds); err != nil {
		t.Fatalf("failed to parse credentials: %v", err)
	}
	if creds.Token != "jwt-test-token" {
		t.Errorf("token = %q, want %q", creds.Token, "jwt-test-token")
	}
	if creds.Server != srv.URL {
		t.Errorf("server = %q, want %q", creds.Server, srv.URL)
	}
}

// TestLoginMissingServer verifies that --server flag is required and
// missing it exits 2 with a usage message.
func TestLoginMissingServer(t *testing.T) {
	code, _, stderr := loginRunArgs([]string{"login"}, "testuser\n")
	if code != 2 {
		t.Errorf("expected exit 2, got %d", code)
	}
	if !strings.Contains(strings.ToLower(stderr), "usage") && !strings.Contains(stderr, "--server") {
		t.Errorf("expected usage message or --server mention in stderr, got %q", stderr)
	}
}

// TestLoginServerError verifies that a 401 response exits 1 and stderr
// contains "Login failed".
func TestLoginServerError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintln(w, `{"error":"invalid credentials"}`)
	}))
	defer srv.Close()

	code, _, stderr := loginRunArgs([]string{"login", "--server", srv.URL}, "baduser\n")
	if code != 1 {
		t.Errorf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "Login failed") {
		t.Errorf("expected 'Login failed' in stderr, got %q", stderr)
	}
}

// TestLoginConnectionError verifies that an unreachable server exits 1
// and stderr contains "Cannot connect".
func TestLoginConnectionError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	code, _, stderr := loginRunArgs([]string{"login", "--server", "http://localhost:19999"}, "user\n")
	if code != 1 {
		t.Errorf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "Cannot connect") {
		t.Errorf("expected 'Cannot connect' in stderr, got %q", stderr)
	}
}

// TestLoginEmptyToken verifies that when the server returns an empty token
// (auth disabled), no credentials file is created and stderr contains
// "authentication disabled".
func TestLoginEmptyToken(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"token":""}`)
	}))
	defer srv.Close()

	code, _, stderr := loginRunArgs([]string{"login", "--server", srv.URL}, "user\n")
	if code != 0 {
		t.Errorf("expected exit 0, got %d (stderr: %q)", code, stderr)
	}
	if !strings.Contains(stderr, "authentication disabled") {
		t.Errorf("expected 'authentication disabled' in stderr, got %q", stderr)
	}

	// No credentials file should be created.
	credsPath := filepath.Join(tmpDir, ".flowy", "credentials.yaml")
	if _, err := os.Stat(credsPath); err == nil {
		t.Error("expected no credentials file to be created when token is empty")
	}
}

// TestLoginRequestBody verifies that the login request sends the correct
// username and password in the JSON body.
func TestLoginRequestBody(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	var capturedBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody) //nolint:errcheck
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"token":"tok"}`)
	}))
	defer srv.Close()

	// Override readPasswordFn to return a known password.
	old := readPasswordFn
	readPasswordFn = func() ([]byte, error) { return []byte("mypassword"), nil }
	defer func() { readPasswordFn = old }()

	code, _, _ := run([]string{"login", "--server", srv.URL}, strings.NewReader("myusername\n"), &strings.Builder{}, &strings.Builder{})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	if capturedBody["username"] != "myusername" {
		t.Errorf("username = %q, want %q", capturedBody["username"], "myusername")
	}
	if capturedBody["password"] != "mypassword" {
		t.Errorf("password = %q, want %q", capturedBody["password"], "mypassword")
	}
}
