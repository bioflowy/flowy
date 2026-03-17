package daemon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bioflowy/flowy/internal/config"
)

// TestBearerTransport_WithToken verifies that when cfg.Auth.DaemonToken is set,
// all HTTP requests from the daemon include the Authorization: Bearer header.
func TestBearerTransport_WithToken(t *testing.T) {
	const token = "my-daemon-token"
	var gotAuthHeader string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	transport := &bearerTransport{
		token: token,
		base:  http.DefaultTransport,
	}
	client := &http.Client{Transport: transport}

	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	resp.Body.Close()

	want := "Bearer " + token
	if gotAuthHeader != want {
		t.Errorf("Authorization header: got %q, want %q", gotAuthHeader, want)
	}
}

// TestBearerTransport_WithoutToken verifies that when token is empty, no
// Authorization header is added to requests.
func TestBearerTransport_WithoutToken(t *testing.T) {
	var gotAuthHeader string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	transport := &bearerTransport{
		token: "",
		base:  http.DefaultTransport,
	}
	client := &http.Client{Transport: transport}

	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	resp.Body.Close()

	if gotAuthHeader != "" {
		t.Errorf("expected no Authorization header, got %q", gotAuthHeader)
	}
}

// TestDaemonNew_WithToken verifies that New() configures the bearer transport
// when cfg.Auth.DaemonToken is non-empty.
func TestDaemonNew_WithToken(t *testing.T) {
	const token = "shared-secret-token"
	var gotAuthHeader string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent) // 204 = no tasks
	}))
	defer srv.Close()

	cfg := &config.Config{}
	cfg.Daemon.ServerURL = srv.URL
	cfg.Daemon.HeartbeatSec = 30
	cfg.Daemon.Workers = 1
	cfg.Auth.DaemonToken = token

	d := New(cfg, "test-daemon-token")

	// claimTask uses d.client, so it will exercise the transport.
	_, err := d.claimTask(context.Background())
	if err != nil {
		// A 204 should not error; if it does, it's a real problem.
		// But if the server was not reached, the auth header check is moot.
		t.Fatalf("claimTask: %v", err)
	}

	want := "Bearer " + token
	if gotAuthHeader != want {
		t.Errorf("Authorization header: got %q, want %q", gotAuthHeader, want)
	}
}

// TestDaemonNew_WithoutToken verifies that New() uses plain transport
// when cfg.Auth.DaemonToken is empty, adding no Authorization header.
func TestDaemonNew_WithoutToken(t *testing.T) {
	var gotAuthHeader string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent) // 204 = no tasks
	}))
	defer srv.Close()

	cfg := &config.Config{}
	cfg.Daemon.ServerURL = srv.URL
	cfg.Daemon.HeartbeatSec = 30
	cfg.Daemon.Workers = 1
	cfg.Auth.DaemonToken = "" // no token

	d := New(cfg, "test-daemon-notoken")

	_, err := d.claimTask(context.Background())
	if err != nil {
		t.Fatalf("claimTask: %v", err)
	}

	if gotAuthHeader != "" {
		t.Errorf("expected no Authorization header, got %q", gotAuthHeader)
	}
}
