package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/bioflowy/flowy/internal/api"
	"github.com/bioflowy/flowy/internal/auth"
	"github.com/bioflowy/flowy/internal/config"
	"github.com/bioflowy/flowy/internal/server"
	"github.com/bioflowy/flowy/internal/storage"
	"github.com/bioflowy/flowy/internal/store"
	"github.com/bioflowy/flowy/internal/store/sqlite"
)

// testJWTSecret is the known secret used in auth-enabled server tests.
var testJWTSecret = []byte("test-jwt-secret-for-auth-tests")

// testDaemonToken is the known daemon token for auth-enabled server tests.
const testDaemonToken = "test-daemon-token-12345"

// newAuthTestServer creates a Server with auth.enabled=true and a known JWT secret.
// The returned server uses an in-memory SQLite store.
func newAuthTestServer(t *testing.T) (*server.Server, *sqlite.SQLiteStore, func()) {
	t.Helper()

	st, err := sqlite.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}

	baseDir := t.TempDir()
	cfg := &config.Config{
		BaseDir: baseDir,
		Server: config.ServerConfig{
			ListenAddr:          ":0",
			HeartbeatTimeoutSec: 60,
		},
		Auth: config.AuthConfig{
			Enabled:     true,
			JWTSecret:   string(testJWTSecret),
			DaemonToken: testDaemonToken,
		},
	}

	backend := storage.NewBackend(cfg)
	srv := server.New(cfg, st, backend)
	return srv, st, func() { st.Close() }
}

// doAuthRequest performs an HTTP request with an optional Bearer token.
func doAuthRequest(t *testing.T, srv *server.Server, method, path string, body interface{}, token string) *httptest.ResponseRecorder {
	t.Helper()

	var buf *bytes.Buffer
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		buf = bytes.NewBuffer(data)
	} else {
		buf = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, path, buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

// issueTestToken creates a JWT signed with the test secret.
func issueTestToken(t *testing.T, userID, username, role string) string {
	t.Helper()
	tok, err := auth.IssueToken(testJWTSecret, userID, username, role)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	return tok
}

// createTestUser creates a user directly in the test store.
func createTestUser(t *testing.T, st *sqlite.SQLiteStore, id, username, password, role string) {
	t.Helper()
	hash, err := auth.HashPassword(password, bcrypt.MinCost)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	u := &store.User{
		ID:           id,
		Username:     username,
		PasswordHash: hash,
		DisplayName:  username,
		GlobalRole:   role,
		IsActive:     true,
	}
	if err := st.CreateUser(context.Background(), u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
}

// ===== handleLogin tests =====

func TestHandleLogin_Success(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "uid-1", "alice", "password123", "user")

	w := doAuthRequest(t, srv, "POST", "/auth/login", api.LoginRequest{
		Username: "alice",
		Password: "password123",
	}, "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}
	var resp api.LoginResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Token == "" {
		t.Error("expected non-empty token")
	}
	// Verify the token is actually valid.
	claims, err := auth.ValidateToken(testJWTSecret, resp.Token)
	if err != nil {
		t.Fatalf("ValidateToken on login response: %v", err)
	}
	if claims.Username != "alice" {
		t.Errorf("claims.Username: got %q, want %q", claims.Username, "alice")
	}
}

func TestHandleLogin_InvalidPassword(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "uid-1", "alice", "password123", "user")

	w := doAuthRequest(t, srv, "POST", "/auth/login", api.LoginRequest{
		Username: "alice",
		Password: "wrongpassword",
	}, "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestHandleLogin_InactiveUser(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "uid-1", "alice", "password123", "user")
	if err := st.DeactivateUser(context.Background(), "uid-1"); err != nil {
		t.Fatalf("DeactivateUser: %v", err)
	}

	w := doAuthRequest(t, srv, "POST", "/auth/login", api.LoginRequest{
		Username: "alice",
		Password: "password123",
	}, "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (body: %s)", w.Code, w.Body.String())
	}
	var errResp api.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if errResp.Error != "account disabled" {
		t.Errorf("error message: got %q, want %q", errResp.Error, "account disabled")
	}
}

func TestHandleLogin_UnknownUser(t *testing.T) {
	srv, _, cleanup := newAuthTestServer(t)
	defer cleanup()

	w := doAuthRequest(t, srv, "POST", "/auth/login", api.LoginRequest{
		Username: "nobody",
		Password: "password",
	}, "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (body: %s)", w.Code, w.Body.String())
	}
}

// ===== handleCreateUser tests =====

func TestHandleCreateUser_Admin(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "admin-1", "admin", "adminpass", "admin")
	adminToken := issueTestToken(t, "admin-1", "admin", "admin")

	w := doAuthRequest(t, srv, "POST", "/users", api.CreateUserRequest{
		Username:   "newuser",
		Password:   "newpass",
		GlobalRole: "user",
	}, adminToken)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", w.Code, w.Body.String())
	}
	var resp api.UserResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Username != "newuser" {
		t.Errorf("username: got %q, want %q", resp.Username, "newuser")
	}
	if resp.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestHandleCreateUser_NonAdmin(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "regularuser", "pass", "user")
	userToken := issueTestToken(t, "user-1", "regularuser", "user")

	w := doAuthRequest(t, srv, "POST", "/users", api.CreateUserRequest{
		Username:   "anotheruser",
		Password:   "pass",
		GlobalRole: "user",
	}, userToken)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d (body: %s)", w.Code, w.Body.String())
	}
}

// ===== handleListUsers tests =====

func TestHandleListUsers_Admin(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "admin-1", "admin", "adminpass", "admin")
	createTestUser(t, st, "user-1", "alice", "pass", "user")
	adminToken := issueTestToken(t, "admin-1", "admin", "admin")

	w := doAuthRequest(t, srv, "GET", "/users", nil, adminToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}
	var resp []api.UserResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp) != 2 {
		t.Errorf("expected 2 users, got %d", len(resp))
	}
}

// ===== handleDeleteUser tests =====

func TestHandleDeleteUser_Admin(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "admin-1", "admin", "adminpass", "admin")
	createTestUser(t, st, "user-1", "alice", "pass", "user")
	adminToken := issueTestToken(t, "admin-1", "admin", "admin")

	w := doAuthRequest(t, srv, "DELETE", "/users/user-1", nil, adminToken)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d (body: %s)", w.Code, w.Body.String())
	}
	// Verify soft-delete.
	u, err := st.GetUserByID(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if u.IsActive {
		t.Error("expected user to be deactivated")
	}
}

// ===== handleGetMe tests =====

func TestHandleGetMe(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "user")
	userToken := issueTestToken(t, "user-1", "alice", "user")

	w := doAuthRequest(t, srv, "GET", "/users/me", nil, userToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}
	var resp api.UserResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Username != "alice" {
		t.Errorf("username: got %q, want %q", resp.Username, "alice")
	}
	if resp.ID != "user-1" {
		t.Errorf("id: got %q, want %q", resp.ID, "user-1")
	}
}

// ===== handleChangePassword tests =====

func TestHandleChangePassword(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "oldpassword", "user")
	userToken := issueTestToken(t, "user-1", "alice", "user")

	w := doAuthRequest(t, srv, "PUT", "/users/me/password", api.ChangePasswordRequest{
		CurrentPassword: "oldpassword",
		NewPassword:     "newpassword",
	}, userToken)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d (body: %s)", w.Code, w.Body.String())
	}

	// Old password should no longer work.
	wLogin := doAuthRequest(t, srv, "POST", "/auth/login", api.LoginRequest{
		Username: "alice",
		Password: "oldpassword",
	}, "")
	if wLogin.Code != http.StatusUnauthorized {
		t.Errorf("expected old password to be rejected (401), got %d", wLogin.Code)
	}

	// New password should work.
	wLogin2 := doAuthRequest(t, srv, "POST", "/auth/login", api.LoginRequest{
		Username: "alice",
		Password: "newpassword",
	}, "")
	if wLogin2.Code != http.StatusOK {
		t.Errorf("expected new password to work (200), got %d", wLogin2.Code)
	}
}

// ===== JWTMiddleware tests =====

func TestJWTMiddleware_NoHeader(t *testing.T) {
	srv, _, cleanup := newAuthTestServer(t)
	defer cleanup()

	// Protected route — no auth header should get 401.
	w := doAuthRequest(t, srv, "GET", "/users/me", nil, "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestJWTMiddleware_InvalidToken(t *testing.T) {
	srv, _, cleanup := newAuthTestServer(t)
	defer cleanup()

	w := doAuthRequest(t, srv, "GET", "/users/me", nil, "invalid.token.here")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (body: %s)", w.Code, w.Body.String())
	}
}

// ===== DaemonAuthMiddleware tests =====

func TestDaemonAuthMiddleware_ValidToken(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	// Create a run so /tasks/claim has something to return (no-content is still a valid claim).
	run := &store.Run{ID: "daemon-run-1", WDLPath: "/tmp/t.wdl", InputsJSON: "{}", State: store.RunRunning}
	if err := st.CreateRun(context.Background(), run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	// Valid daemon token should allow access (204 = no pending task, but request passed auth).
	w := doAuthRequest(t, srv, "POST", "/tasks/claim", api.TaskClaimRequest{DaemonID: "daemon-1"}, testDaemonToken)
	if w.Code == http.StatusUnauthorized {
		t.Fatalf("expected non-401, got 401: daemon token not accepted")
	}
}

func TestDaemonAuthMiddleware_InvalidToken(t *testing.T) {
	srv, _, cleanup := newAuthTestServer(t)
	defer cleanup()

	w := doAuthRequest(t, srv, "POST", "/tasks/claim", api.TaskClaimRequest{DaemonID: "daemon-1"}, "wrong-token")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (body: %s)", w.Code, w.Body.String())
	}
}
