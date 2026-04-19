package server_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/bioflowy/flowy/internal/api"
	"github.com/bioflowy/flowy/internal/server"
	"github.com/bioflowy/flowy/internal/store/sqlite"
)

// doMultipartUpload creates a multipart POST request and executes it against srv.
// fieldName is the form field name (must be "file"), fileName is the filename hint,
// fileContent is the file body, and token is the Bearer token (may be empty).
func doMultipartUpload(
	t *testing.T,
	srv *server.Server,
	path, fieldName, fileName, fileContent, token string,
) *httptest.ResponseRecorder {
	return doMultipartUploadWithFields(t, srv, path, fieldName, fileName, fileContent, nil, token)
}

func doMultipartUploadWithFields(
	t *testing.T,
	srv *server.Server,
	path, fieldName, fileName, fileContent string,
	extraFields map[string]string,
	token string,
) *httptest.ResponseRecorder {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := io.WriteString(part, fileContent); err != nil {
		t.Fatalf("write part: %v", err)
	}
	for key, value := range extraFields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("WriteField %q: %v", key, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest("POST", path, &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

// setupUploadTest creates a server with a test project and two users (executor + viewer).
// Returns the server, store, project slug, executor token, viewer token, and cleanup func.
func setupUploadTest(t *testing.T) (*server.Server, *sqlite.SQLiteStore, string, string, string, func()) {
	t.Helper()
	srv, st, cleanup := newAuthTestServer(t)

	createTestUser(t, st, "exec-1", "executor_user", "pass", "executor")
	createTestUser(t, st, "view-1", "viewer_user", "pass", "user")

	proj := createRegistryTestProject(t, st, "exec-1")
	addProjectMember(t, st, proj.ID, "view-1", "viewer")

	execToken := issueTestToken(t, "exec-1", "executor_user", "executor")
	viewToken := issueTestToken(t, "view-1", "viewer_user", "user")

	return srv, st, proj.Slug, execToken, viewToken, cleanup
}

// ===== Upload tests =====

// TestUpload: happy-path multipart upload returns 201 with a server-accessible path.
func TestUpload(t *testing.T) {
	srv, _, slug, execToken, _, cleanup := setupUploadTest(t)
	defer cleanup()

	w := doMultipartUpload(t, srv, "/projects/"+slug+"/uploads", "file", "hello.txt", "test data", execToken)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp api.UploadResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !strings.Contains(resp.Path, "/projects/") {
		t.Errorf("path %q does not contain /projects/", resp.Path)
	}
	if !strings.Contains(resp.Path, "/uploads/") {
		t.Errorf("path %q does not contain /uploads/", resp.Path)
	}
	if !strings.HasSuffix(resp.Path, "/hello.txt") {
		t.Errorf("path %q does not end with /hello.txt", resp.Path)
	}
}

// TestUpload_PathTraversal: filename with path traversal components is sanitized.
func TestUpload_PathTraversal(t *testing.T) {
	srv, _, slug, execToken, _, cleanup := setupUploadTest(t)
	defer cleanup()

	w := doMultipartUpload(t, srv, "/projects/"+slug+"/uploads", "file", "../../../etc/passwd", "evil", execToken)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp api.UploadResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if strings.Contains(resp.Path, "..") {
		t.Errorf("path %q still contains '..'", resp.Path)
	}
	if !strings.HasSuffix(resp.Path, "/passwd") {
		t.Errorf("path %q does not end with /passwd", resp.Path)
	}
}

// TestUpload_MissingFileField: request without "file" field returns 400.
func TestUpload_MissingFileField(t *testing.T) {
	srv, _, slug, execToken, _, cleanup := setupUploadTest(t)
	defer cleanup()

	// Send multipart with a different field name (not "file").
	w := doMultipartUpload(t, srv, "/projects/"+slug+"/uploads", "data", "hello.txt", "test data", execToken)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", w.Code, w.Body.String())
	}

	var errResp api.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if !strings.Contains(strings.ToLower(errResp.Error), "missing") {
		t.Errorf("error %q should contain 'missing'", errResp.Error)
	}
}

// TestUpload_TooLarge: body exceeding 100 MB is rejected with 400.
func TestUpload_TooLarge(t *testing.T) {
	srv, _, slug, execToken, _, cleanup := setupUploadTest(t)
	defer cleanup()

	// Build a multipart body > 100 MB.
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", "large.bin")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	// Write 101 MB of zeros.
	chunk := make([]byte, 1<<20) // 1 MB chunk
	for i := 0; i < 101; i++ {
		if _, err := part.Write(chunk); err != nil {
			t.Fatalf("write chunk %d: %v", i, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest("POST", "/projects/"+slug+"/uploads", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+execToken)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", w.Code, w.Body.String())
	}

	var errResp api.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if !strings.Contains(errResp.Error, "100 MB") {
		t.Errorf("error %q should contain '100 MB'", errResp.Error)
	}
}

// TestUpload_Auth_Executor: executor gets 201, viewer gets 403, unauthenticated gets 401.
func TestUpload_Auth_Executor(t *testing.T) {
	srv, _, slug, execToken, viewToken, cleanup := setupUploadTest(t)
	defer cleanup()

	// Executor: 201.
	w := doMultipartUpload(t, srv, "/projects/"+slug+"/uploads", "file", "a.txt", "data", execToken)
	if w.Code != http.StatusCreated {
		t.Errorf("executor: expected 201, got %d (body: %s)", w.Code, w.Body.String())
	}

	// Viewer: 403.
	w = doMultipartUpload(t, srv, "/projects/"+slug+"/uploads", "file", "a.txt", "data", viewToken)
	if w.Code != http.StatusForbidden {
		t.Errorf("viewer: expected 403, got %d (body: %s)", w.Code, w.Body.String())
	}

	// Unauthenticated: 401.
	w = doMultipartUpload(t, srv, "/projects/"+slug+"/uploads", "file", "a.txt", "data", "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated: expected 401, got %d (body: %s)", w.Code, w.Body.String())
	}
}

// TestUpload_FileStored: after a successful upload, file content matches on disk.
func TestUpload_FileStored(t *testing.T) {
	srv, _, slug, execToken, _, cleanup := setupUploadTest(t)
	defer cleanup()

	const content = "file content for storage check"
	w := doMultipartUpload(t, srv, "/projects/"+slug+"/uploads", "file", "stored.txt", content, execToken)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp api.UploadResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	data, err := os.ReadFile(resp.Path)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", resp.Path, err)
	}
	if string(data) != content {
		t.Errorf("file content: got %q, want %q", string(data), content)
	}
}
