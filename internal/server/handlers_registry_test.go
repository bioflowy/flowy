package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/bioflowy/flowy/internal/api"
	"github.com/bioflowy/flowy/internal/store"
	"github.com/bioflowy/flowy/internal/store/sqlite"
)

// sampleToolWDL is a minimal valid WDL file defining a single task.
const sampleToolWDL = `version 1.0
task hello {
    input {
        String name
    }
    command <<<
        echo "Hello ~{name}"
    >>>
    output {
        String greeting = read_string(stdout())
    }
}
`

// sampleWorkflowWDL is a minimal valid WDL file defining a workflow.
const sampleWorkflowWDL = `version 1.0
workflow greet {
    input {
        String name
    }
    call hello { input: name = name }
    output {
        String greeting = hello.greeting
    }
}

task hello {
    input {
        String name
    }
    command <<<
        echo "Hello ~{name}"
    >>>
    output {
        String greeting = read_string(stdout())
    }
}
`

// createRegistryTestProject creates a project with a unique slug in the store and adds
// the given user as admin. Returns the created project.
func createRegistryTestProject(t *testing.T, st *sqlite.SQLiteStore, userID string) *store.Project {
	t.Helper()
	now := time.Now().UTC()
	projID := uuid.New().String()
	proj := &store.Project{
		ID:       projID,
		Slug:     "test-project-" + projID[:8],
		IsActive: true,
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
		t.Fatalf("AddMember: %v", err)
	}
	return proj
}

// ===== Push tests =====

func TestRegistryPush(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	token := issueTestToken(t, "user-1", "alice", "executor")
	proj := createRegistryTestProject(t, st, "user-1")

	w := doAuthRequest(t, srv, "POST", "/projects/"+proj.Slug+"/registry", api.RegistryPushRequest{
		Name:    "my-tool",
		Content: sampleToolWDL,
	}, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp api.RegistryPushResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Name != "my-tool" {
		t.Errorf("name: got %q, want %q", resp.Name, "my-tool")
	}
	if !strings.HasPrefix(resp.Digest, "sha256:") {
		t.Errorf("digest: got %q, expected prefix sha256:", resp.Digest)
	}
	if resp.Kind != "tool" {
		t.Errorf("kind: got %q, want %q", resp.Kind, "tool")
	}
	if resp.VersionID == "" {
		t.Error("expected non-empty version_id")
	}
}

func TestRegistryPush_Idempotent(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	token := issueTestToken(t, "user-1", "alice", "executor")
	proj := createRegistryTestProject(t, st, "user-1")

	body := api.RegistryPushRequest{
		Name:    "my-tool",
		Content: sampleToolWDL,
	}

	// First push — should be 201.
	w1 := doAuthRequest(t, srv, "POST", "/projects/"+proj.Slug+"/registry", body, token)
	if w1.Code != http.StatusCreated {
		t.Fatalf("first push: expected 201, got %d (body: %s)", w1.Code, w1.Body.String())
	}
	var resp1 api.RegistryPushResponse
	if err := json.NewDecoder(w1.Body).Decode(&resp1); err != nil {
		t.Fatalf("decode first response: %v", err)
	}

	// Second push of identical content — should be 200 with the same digest.
	w2 := doAuthRequest(t, srv, "POST", "/projects/"+proj.Slug+"/registry", body, token)
	if w2.Code != http.StatusOK {
		t.Fatalf("second push: expected 200, got %d (body: %s)", w2.Code, w2.Body.String())
	}
	var resp2 api.RegistryPushResponse
	if err := json.NewDecoder(w2.Body).Decode(&resp2); err != nil {
		t.Fatalf("decode second response: %v", err)
	}

	if resp1.Digest != resp2.Digest {
		t.Errorf("digest mismatch: first %q, second %q", resp1.Digest, resp2.Digest)
	}
}

func TestRegistryPush_InvalidWDL(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	token := issueTestToken(t, "user-1", "alice", "executor")
	proj := createRegistryTestProject(t, st, "user-1")

	w := doAuthRequest(t, srv, "POST", "/projects/"+proj.Slug+"/registry", api.RegistryPushRequest{
		Name:    "bad-wdl",
		Content: "not valid wdl",
	}, token)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestRegistryPush_InvalidName(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	token := issueTestToken(t, "user-1", "alice", "executor")
	proj := createRegistryTestProject(t, st, "user-1")

	w := doAuthRequest(t, srv, "POST", "/projects/"+proj.Slug+"/registry", api.RegistryPushRequest{
		Name:    "INVALID NAME",
		Content: sampleToolWDL,
	}, token)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestRegistryPush_Classification(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	token := issueTestToken(t, "user-1", "alice", "executor")
	proj := createRegistryTestProject(t, st, "user-1")

	// Push a workflow WDL — should be classified as "workflow".
	w := doAuthRequest(t, srv, "POST", "/projects/"+proj.Slug+"/registry", api.RegistryPushRequest{
		Name:    "my-workflow",
		Content: sampleWorkflowWDL,
	}, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("workflow push: expected 201, got %d (body: %s)", w.Code, w.Body.String())
	}
	var wfResp api.RegistryPushResponse
	if err := json.NewDecoder(w.Body).Decode(&wfResp); err != nil {
		t.Fatalf("decode workflow response: %v", err)
	}
	if wfResp.Kind != "workflow" {
		t.Errorf("workflow kind: got %q, want %q", wfResp.Kind, "workflow")
	}

	// Push a tool WDL — should be classified as "tool".
	w2 := doAuthRequest(t, srv, "POST", "/projects/"+proj.Slug+"/registry", api.RegistryPushRequest{
		Name:    "my-tool",
		Content: sampleToolWDL,
	}, token)
	if w2.Code != http.StatusCreated {
		t.Fatalf("tool push: expected 201, got %d (body: %s)", w2.Code, w2.Body.String())
	}
	var toolResp api.RegistryPushResponse
	if err := json.NewDecoder(w2.Body).Decode(&toolResp); err != nil {
		t.Fatalf("decode tool response: %v", err)
	}
	if toolResp.Kind != "tool" {
		t.Errorf("tool kind: got %q, want %q", toolResp.Kind, "tool")
	}
}

func TestRegistryPush_KindMismatch(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	token := issueTestToken(t, "user-1", "alice", "executor")
	proj := createRegistryTestProject(t, st, "user-1")

	// Push a tool WDL to "my-entry".
	w1 := doAuthRequest(t, srv, "POST", "/projects/"+proj.Slug+"/registry", api.RegistryPushRequest{
		Name:    "my-entry",
		Content: sampleToolWDL,
	}, token)
	if w1.Code != http.StatusCreated {
		t.Fatalf("first push (tool): expected 201, got %d (body: %s)", w1.Code, w1.Body.String())
	}

	// Push a workflow WDL to the same "my-entry" — should be 409 kind mismatch.
	w2 := doAuthRequest(t, srv, "POST", "/projects/"+proj.Slug+"/registry", api.RegistryPushRequest{
		Name:    "my-entry",
		Content: sampleWorkflowWDL,
	}, token)
	if w2.Code != http.StatusConflict {
		t.Fatalf("kind mismatch: expected 409, got %d (body: %s)", w2.Code, w2.Body.String())
	}
}

// ===== Pull tests =====

func TestRegistryPull_ByTag(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	token := issueTestToken(t, "user-1", "alice", "executor")
	proj := createRegistryTestProject(t, st, "user-1")

	// Push a WDL to get a version.
	w := doAuthRequest(t, srv, "POST", "/projects/"+proj.Slug+"/registry", api.RegistryPushRequest{
		Name:    "my-tool",
		Content: sampleToolWDL,
	}, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("push: expected 201, got %d (body: %s)", w.Code, w.Body.String())
	}
	var pushResp api.RegistryPushResponse
	if err := json.NewDecoder(w.Body).Decode(&pushResp); err != nil {
		t.Fatalf("decode push response: %v", err)
	}

	// Tag the version directly via the store (tag handler is Plan 03).
	entry, err := st.GetRegistryEntry(context.Background(), proj.ID, "my-tool")
	if err != nil {
		t.Fatalf("GetRegistryEntry: %v", err)
	}
	if err := st.UpsertRegistryTag(context.Background(), entry.ID, "latest", pushResp.VersionID); err != nil {
		t.Fatalf("UpsertRegistryTag: %v", err)
	}

	// Pull by tag.
	w2 := doAuthRequest(t, srv, "GET", "/projects/"+proj.Slug+"/registry/my-tool?tag=latest", nil, token)
	if w2.Code != http.StatusOK {
		t.Fatalf("pull by tag: expected 200, got %d (body: %s)", w2.Code, w2.Body.String())
	}
	var pullResp api.RegistryPullResponse
	if err := json.NewDecoder(w2.Body).Decode(&pullResp); err != nil {
		t.Fatalf("decode pull response: %v", err)
	}
	if pullResp.Content != sampleToolWDL {
		t.Errorf("content mismatch: got %q, want sampleToolWDL", pullResp.Content[:min(len(pullResp.Content), 50)])
	}
}

func TestRegistryPull_ByDigest(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	token := issueTestToken(t, "user-1", "alice", "executor")
	proj := createRegistryTestProject(t, st, "user-1")

	// Push a WDL to get a digest.
	w := doAuthRequest(t, srv, "POST", "/projects/"+proj.Slug+"/registry", api.RegistryPushRequest{
		Name:    "my-tool",
		Content: sampleToolWDL,
	}, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("push: expected 201, got %d (body: %s)", w.Code, w.Body.String())
	}
	var pushResp api.RegistryPushResponse
	if err := json.NewDecoder(w.Body).Decode(&pushResp); err != nil {
		t.Fatalf("decode push response: %v", err)
	}

	// Pull by digest.
	w2 := doAuthRequest(t, srv, "GET", "/projects/"+proj.Slug+"/registry/my-tool?digest="+pushResp.Digest, nil, token)
	if w2.Code != http.StatusOK {
		t.Fatalf("pull by digest: expected 200, got %d (body: %s)", w2.Code, w2.Body.String())
	}
	var pullResp api.RegistryPullResponse
	if err := json.NewDecoder(w2.Body).Decode(&pullResp); err != nil {
		t.Fatalf("decode pull response: %v", err)
	}
	if pullResp.Digest != pushResp.Digest {
		t.Errorf("digest mismatch: got %q, want %q", pullResp.Digest, pushResp.Digest)
	}
	if pullResp.Content != sampleToolWDL {
		t.Errorf("content mismatch")
	}
}

func TestRegistryPull_NotFound(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	token := issueTestToken(t, "user-1", "alice", "executor")
	proj := createRegistryTestProject(t, st, "user-1")

	// Push something so the entry exists.
	w := doAuthRequest(t, srv, "POST", "/projects/"+proj.Slug+"/registry", api.RegistryPushRequest{
		Name:    "my-tool",
		Content: sampleToolWDL,
	}, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("push: expected 201, got %d (body: %s)", w.Code, w.Body.String())
	}

	// Pull with a nonexistent tag — should be 404 with "tag" in the error message.
	w2 := doAuthRequest(t, srv, "GET", "/projects/"+proj.Slug+"/registry/my-tool?tag=nonexistent", nil, token)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body: %s)", w2.Code, w2.Body.String())
	}
	var errResp api.ErrorResponse
	if err := json.NewDecoder(w2.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if !strings.Contains(errResp.Error, "tag") {
		t.Errorf("expected error message to contain 'tag', got %q", errResp.Error)
	}
}

func TestRegistryPull_NoParam(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	token := issueTestToken(t, "user-1", "alice", "executor")
	proj := createRegistryTestProject(t, st, "user-1")

	// Pull without any query parameter — should be 400.
	w := doAuthRequest(t, srv, "GET", "/projects/"+proj.Slug+"/registry/my-tool", nil, token)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", w.Code, w.Body.String())
	}
}

