package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/bioflowy/flowy/internal/api"
	"github.com/bioflowy/flowy/internal/store"
	"github.com/bioflowy/flowy/internal/store/sqlite"
)

// createRunsTestProject creates a project with a unique slug and adds the given user as admin.
func createRunsTestProject(t *testing.T, st *sqlite.SQLiteStore, userID string) *store.Project {
	t.Helper()
	now := time.Now().UTC()
	projID := uuid.New().String()
	proj := &store.Project{
		ID:        projID,
		Slug:      "runs-proj-" + projID[:8],
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := st.CreateProject(context.Background(), proj); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := st.AddMember(context.Background(), &store.ProjectMember{
		ProjectID: proj.ID,
		UserID:    userID,
		Role:      "admin",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("AddMember(admin): %v", err)
	}
	return proj
}

// createProjectRun creates a run record in the store for the given project.
func createProjectRun(t *testing.T, st *sqlite.SQLiteStore, id, projectID string) *store.Run {
	t.Helper()
	r := &store.Run{
		ID:         id,
		ProjectID:  projectID,
		WDLPath:    "/test/workflow.wdl",
		InputsJSON: `{}`,
		State:      store.RunPending,
	}
	if err := st.CreateRun(context.Background(), r); err != nil {
		t.Fatalf("CreateRun(%s): %v", id, err)
	}
	return r
}

// ===== TestHandleListRuns =====

func TestHandleListRuns_Success(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	aliceToken := issueTestToken(t, "user-1", "alice", "executor")

	proj := createRunsTestProject(t, st, "user-1")

	createProjectRun(t, st, uuid.New().String(), proj.ID)
	createProjectRun(t, st, uuid.New().String(), proj.ID)

	w := doAuthRequest(t, srv, "GET", "/projects/"+proj.Slug+"/runs", nil, aliceToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp []api.RunListItem
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp) != 2 {
		t.Errorf("expected 2 runs, got %d", len(resp))
	}
	for _, item := range resp {
		if item.ID == "" {
			t.Error("expected non-empty ID")
		}
		if item.State == "" {
			t.Error("expected non-empty State")
		}
		if item.CreatedAt == "" {
			t.Error("expected non-empty CreatedAt")
		}
	}
}

func TestHandleListRuns_StatusFilter(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	aliceToken := issueTestToken(t, "user-1", "alice", "executor")

	proj := createRunsTestProject(t, st, "user-1")

	r1 := createProjectRun(t, st, uuid.New().String(), proj.ID)
	createProjectRun(t, st, uuid.New().String(), proj.ID)

	// Transition r1 to COMPLETED; r2 stays PENDING.
	if err := st.UpdateRunState(context.Background(), r1.ID, store.RunCompleted); err != nil {
		t.Fatalf("UpdateRunState: %v", err)
	}

	w := doAuthRequest(t, srv, "GET", "/projects/"+proj.Slug+"/runs?status=COMPLETED", nil, aliceToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp []api.RunListItem
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp) != 1 {
		t.Errorf("expected 1 completed run, got %d", len(resp))
	}
	if len(resp) > 0 && resp[0].State != "COMPLETED" {
		t.Errorf("expected state COMPLETED, got %q", resp[0].State)
	}
}

func TestHandleListRuns_Empty(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	aliceToken := issueTestToken(t, "user-1", "alice", "executor")

	proj := createRunsTestProject(t, st, "user-1")

	w := doAuthRequest(t, srv, "GET", "/projects/"+proj.Slug+"/runs", nil, aliceToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	// Must return JSON array, not null.
	body := strings.TrimSpace(w.Body.String())
	if !strings.HasPrefix(body, "[") {
		t.Errorf("expected JSON array response, got: %s", body)
	}

	var resp []api.RunListItem
	if err := json.NewDecoder(strings.NewReader(body)).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp) != 0 {
		t.Errorf("expected empty array, got %d items", len(resp))
	}
}

// ===== TestHandleFileDownload =====

func TestHandleFileDownload_Success(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	aliceToken := issueTestToken(t, "user-1", "alice", "executor")

	proj := createRunsTestProject(t, st, "user-1")
	runID := uuid.New().String()
	createProjectRun(t, st, runID, proj.ID)

	// Write a test file at an absolute temp path.
	tmpFile, err := os.CreateTemp(t.TempDir(), "testfile-*.txt")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	content := "hello from test file"
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	tmpFile.Close()
	filePath := tmpFile.Name()

	url := fmt.Sprintf("/projects/%s/runs/%s/files?path=%s", proj.Slug, runID, filePath)
	w := doAuthRequest(t, srv, "GET", url, nil, aliceToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	disposition := w.Header().Get("Content-Disposition")
	if !strings.HasPrefix(disposition, "attachment;") {
		t.Errorf("Content-Disposition: got %q, want attachment prefix", disposition)
	}

	if w.Body.String() != content {
		t.Errorf("body: got %q, want %q", w.Body.String(), content)
	}
}

func TestHandleFileDownload_Preview(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	aliceToken := issueTestToken(t, "user-1", "alice", "executor")

	proj := createRunsTestProject(t, st, "user-1")
	runID := uuid.New().String()
	createProjectRun(t, st, runID, proj.ID)

	tmpFile, err := os.CreateTemp(t.TempDir(), "preview-*.txt")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	content := "preview content here"
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	tmpFile.Close()
	filePath := tmpFile.Name()

	url := fmt.Sprintf("/projects/%s/runs/%s/files?path=%s&preview=true", proj.Slug, runID, filePath)
	w := doAuthRequest(t, srv, "GET", url, nil, aliceToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/plain") {
		t.Errorf("Content-Type: got %q, want text/plain prefix", contentType)
	}

	if w.Body.String() != content {
		t.Errorf("body: got %q, want %q", w.Body.String(), content)
	}
}

func TestHandleFileDownload_PreviewTruncated(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	aliceToken := issueTestToken(t, "user-1", "alice", "executor")

	proj := createRunsTestProject(t, st, "user-1")
	runID := uuid.New().String()
	createProjectRun(t, st, runID, proj.ID)

	const maxPreview = 100 * 1024
	bigContent := make([]byte, maxPreview+1024)
	for i := range bigContent {
		bigContent[i] = 'A'
	}

	tmpFile, err := os.CreateTemp(t.TempDir(), "bigfile-*.bin")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := tmpFile.Write(bigContent); err != nil {
		t.Fatalf("Write: %v", err)
	}
	tmpFile.Close()
	filePath := tmpFile.Name()

	url := fmt.Sprintf("/projects/%s/runs/%s/files?path=%s&preview=true", proj.Slug, runID, filePath)
	w := doAuthRequest(t, srv, "GET", url, nil, aliceToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if w.Body.Len() != maxPreview {
		t.Errorf("preview body length: got %d, want %d", w.Body.Len(), maxPreview)
	}
}

func TestHandleFileDownload_PathTraversal(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	aliceToken := issueTestToken(t, "user-1", "alice", "executor")

	proj := createRunsTestProject(t, st, "user-1")
	runID := uuid.New().String()
	createProjectRun(t, st, runID, proj.ID)

	url := fmt.Sprintf("/projects/%s/runs/%s/files?path=../../etc/passwd", proj.Slug, runID)
	w := doAuthRequest(t, srv, "GET", url, nil, aliceToken)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", w.Code, w.Body.String())
	}

	var errResp api.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if !strings.Contains(errResp.Error, "must not contain") {
		t.Errorf("error message: got %q, want to contain 'must not contain'", errResp.Error)
	}
}

func TestHandleFileDownload_MissingPath(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	aliceToken := issueTestToken(t, "user-1", "alice", "executor")

	proj := createRunsTestProject(t, st, "user-1")
	runID := uuid.New().String()
	createProjectRun(t, st, runID, proj.ID)

	url := fmt.Sprintf("/projects/%s/runs/%s/files", proj.Slug, runID)
	w := doAuthRequest(t, srv, "GET", url, nil, aliceToken)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", w.Code, w.Body.String())
	}
}
