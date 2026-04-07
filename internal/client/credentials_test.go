package client_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/bioflowy/flowy/internal/client"
)

// TestSaveCredentialsPermission verifies that SaveCredentials creates the
// ~/.flowy/ directory with mode 0700 and the credentials file with mode 0600.
func TestSaveCredentialsPermission(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	creds := &client.Credentials{
		Server: "http://localhost:8124",
		Token:  "my-jwt-token",
	}
	if err := client.SaveCredentials(creds); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}

	// Check directory permissions.
	flowyDir := filepath.Join(tmpDir, ".flowy")
	dirInfo, err := os.Stat(flowyDir)
	if err != nil {
		t.Fatalf("stat ~/.flowy: %v", err)
	}
	if dirInfo.Mode().Perm() != 0700 {
		t.Errorf("~/.flowy dir perm = %o, want 0700", dirInfo.Mode().Perm())
	}

	// Check file permissions.
	credsPath := filepath.Join(flowyDir, "credentials.yaml")
	fileInfo, err := os.Stat(credsPath)
	if err != nil {
		t.Fatalf("stat credentials file: %v", err)
	}
	if fileInfo.Mode().Perm() != 0600 {
		t.Errorf("credentials file perm = %o, want 0600", fileInfo.Mode().Perm())
	}
}

// TestLoadCredentialsRoundTrip verifies that SaveCredentials followed by
// LoadCredentials returns the same Credentials struct.
func TestLoadCredentialsRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	want := &client.Credentials{
		Server: "http://flowy.example.com:8124",
		Token:  "eyJhbGciOiJIUzI1NiJ9.test",
	}
	if err := client.SaveCredentials(want); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}

	got, err := client.LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}
	if got.Server != want.Server {
		t.Errorf("Server = %q, want %q", got.Server, want.Server)
	}
	if got.Token != want.Token {
		t.Errorf("Token = %q, want %q", got.Token, want.Token)
	}
}

// TestLoadCredentialsMissing verifies that LoadCredentials returns an error
// when no credentials file exists.
func TestLoadCredentialsMissing(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	_, err := client.LoadCredentials()
	if err == nil {
		t.Fatal("expected error for missing credentials file, got nil")
	}
}

// TestLoginSuccess verifies that Login returns the JWT token on HTTP 200.
func TestLoginSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/auth/login" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"token":"test-jwt-token"}`)
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	token, err := c.Login(t.Context(), "testuser", "testpass")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if token != "test-jwt-token" {
		t.Errorf("token = %q, want %q", token, "test-jwt-token")
	}
}

// TestLoginInvalidCredentials verifies that Login returns an error containing
// "login failed" when the server returns HTTP 401.
func TestLoginInvalidCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintln(w, `{"error":"invalid credentials"}`)
	}))
	defer srv.Close()

	c := client.New(srv.URL)
	_, err := c.Login(t.Context(), "bad", "creds")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !contains(err.Error(), "login failed") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "login failed")
	}
}

// TestLoginConnectionError verifies that Login returns an error containing
// "cannot connect" when the server URL is unreachable.
func TestLoginConnectionError(t *testing.T) {
	c := client.New("http://localhost:19999") // presumably not listening
	_, err := c.Login(t.Context(), "user", "pass")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !contains(err.Error(), "cannot connect") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "cannot connect")
	}
}

// TestNewWithTokenInjectsBearer verifies that NewWithToken creates a Client
// that sets the Authorization Bearer header on requests.
func TestNewWithTokenInjectsBearer(t *testing.T) {
	var receivedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		// Return a valid status response.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"run_id": "abc", "state": "COMPLETED"})
	}))
	defer srv.Close()

	c := client.NewWithToken(srv.URL, "my-bearer-token")
	// Use Status to exercise doRequest through an existing method.
	_, _ = c.Status(t.Context(), "abc")

	want := "Bearer my-bearer-token"
	if receivedAuth != want {
		t.Errorf("Authorization header = %q, want %q", receivedAuth, want)
	}
}

// contains is a helper for substring checking.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		findSubstring(s, sub))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
