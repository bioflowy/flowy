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
	"github.com/bioflowy/flowy/internal/server"
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

// addProjectMember adds a user to a project with the specified role in the store.
func addProjectMember(t *testing.T, st *sqlite.SQLiteStore, projID, userID, role string) {
	t.Helper()
	now := time.Now().UTC()
	if err := st.AddMember(context.Background(), &store.ProjectMember{
		ProjectID: projID,
		UserID:    userID,
		Role:      role,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("AddMember(%s): %v", role, err)
	}
}

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

// sampleToolWDLv2 is a second tool WDL with different content (produces a different digest).
const sampleToolWDLv2 = `version 1.0
task hello {
    input {
        String name
        String greeting = "Hello"
    }
    command <<<
        echo "~{greeting} ~{name}"
    >>>
    output {
        String result = read_string(stdout())
    }
}
`

// ===== Tag tests =====

// pushWDL is a test helper that pushes WDL content to the registry and returns the push response.
func pushWDL(t *testing.T, srv *server.Server, projSlug, entryName, content, token string) api.RegistryPushResponse {
	t.Helper()
	w := doAuthRequest(t, srv, "POST", "/projects/"+projSlug+"/registry", api.RegistryPushRequest{
		Name:    entryName,
		Content: content,
	}, token)
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("pushWDL: expected 200 or 201, got %d (body: %s)", w.Code, w.Body.String())
	}
	var resp api.RegistryPushResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("pushWDL decode: %v", err)
	}
	return resp
}

func TestRegistryTag(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "admin")
	token := issueTestToken(t, "user-1", "alice", "admin")
	proj := createRegistryTestProject(t, st, "user-1")

	// Push WDL to get a digest.
	pushResp := pushWDL(t, srv, proj.Slug, "my-tool", sampleToolWDL, token)

	// Tag the version via the API.
	w := doAuthRequest(t, srv, "POST", "/projects/"+proj.Slug+"/registry/my-tool/tags", api.RegistryTagRequest{
		Tag:    "v1.0",
		Digest: pushResp.Digest,
	}, token)
	if w.Code != http.StatusOK {
		t.Fatalf("tag: expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var tagResp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&tagResp); err != nil {
		t.Fatalf("decode tag response: %v", err)
	}
	if tagResp["tag"] != "v1.0" {
		t.Errorf("tag: got %q, want %q", tagResp["tag"], "v1.0")
	}

	// Pull by tag — content must match sampleToolWDL.
	w2 := doAuthRequest(t, srv, "GET", "/projects/"+proj.Slug+"/registry/my-tool?tag=v1.0", nil, token)
	if w2.Code != http.StatusOK {
		t.Fatalf("pull by tag: expected 200, got %d (body: %s)", w2.Code, w2.Body.String())
	}
	var pullResp api.RegistryPullResponse
	if err := json.NewDecoder(w2.Body).Decode(&pullResp); err != nil {
		t.Fatalf("decode pull response: %v", err)
	}
	if pullResp.Content != sampleToolWDL {
		t.Errorf("content mismatch: got %q, want sampleToolWDL", pullResp.Content)
	}
}

func TestRegistryTag_Reassign(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "admin")
	token := issueTestToken(t, "user-1", "alice", "admin")
	proj := createRegistryTestProject(t, st, "user-1")

	// Push first version.
	push1 := pushWDL(t, srv, proj.Slug, "my-tool", sampleToolWDL, token)

	// Push second version (different content).
	push2 := pushWDL(t, srv, proj.Slug, "my-tool", sampleToolWDLv2, token)

	// Tag first version as "latest".
	w := doAuthRequest(t, srv, "POST", "/projects/"+proj.Slug+"/registry/my-tool/tags", api.RegistryTagRequest{
		Tag:    "latest",
		Digest: push1.Digest,
	}, token)
	if w.Code != http.StatusOK {
		t.Fatalf("tag v1: expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	// Reassign "latest" to second version.
	w2 := doAuthRequest(t, srv, "POST", "/projects/"+proj.Slug+"/registry/my-tool/tags", api.RegistryTagRequest{
		Tag:    "latest",
		Digest: push2.Digest,
	}, token)
	if w2.Code != http.StatusOK {
		t.Fatalf("tag v2: expected 200, got %d (body: %s)", w2.Code, w2.Body.String())
	}

	// Pull by tag — content must match the SECOND version.
	w3 := doAuthRequest(t, srv, "GET", "/projects/"+proj.Slug+"/registry/my-tool?tag=latest", nil, token)
	if w3.Code != http.StatusOK {
		t.Fatalf("pull: expected 200, got %d (body: %s)", w3.Code, w3.Body.String())
	}
	var pullResp api.RegistryPullResponse
	if err := json.NewDecoder(w3.Body).Decode(&pullResp); err != nil {
		t.Fatalf("decode pull response: %v", err)
	}
	if pullResp.Content != sampleToolWDLv2 {
		t.Errorf("content should be second version after reassignment; got first version or unexpected content")
	}
}

func TestRegistryTag_Forbidden(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	// Admin sets up the project and pushes data.
	createTestUser(t, st, "admin-1", "bob", "pass", "admin")
	adminToken := issueTestToken(t, "admin-1", "bob", "admin")

	// Executor user is added to the project with executor role.
	createTestUser(t, st, "executor-1", "alice", "pass", "executor")
	executorToken := issueTestToken(t, "executor-1", "alice", "executor")

	proj := createRegistryTestProject(t, st, "admin-1")
	addProjectMember(t, st, proj.ID, "executor-1", "executor")

	// Push WDL as admin to have something to tag.
	pushResp := pushWDL(t, srv, proj.Slug, "my-tool", sampleToolWDL, adminToken)

	// Attempt to tag as executor — must be 403.
	w := doAuthRequest(t, srv, "POST", "/projects/"+proj.Slug+"/registry/my-tool/tags", api.RegistryTagRequest{
		Tag:    "v1.0",
		Digest: pushResp.Digest,
	}, executorToken)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestRegistryTag_InvalidName(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "admin")
	token := issueTestToken(t, "user-1", "alice", "admin")
	proj := createRegistryTestProject(t, st, "user-1")

	pushResp := pushWDL(t, srv, proj.Slug, "my-tool", sampleToolWDL, token)

	// Colon in tag name — 400.
	w := doAuthRequest(t, srv, "POST", "/projects/"+proj.Slug+"/registry/my-tool/tags", api.RegistryTagRequest{
		Tag:    "bad:tag",
		Digest: pushResp.Digest,
	}, token)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("colon: expected 400, got %d (body: %s)", w.Code, w.Body.String())
	}

	// At-sign in tag name — 400.
	w2 := doAuthRequest(t, srv, "POST", "/projects/"+proj.Slug+"/registry/my-tool/tags", api.RegistryTagRequest{
		Tag:    "bad@tag",
		Digest: pushResp.Digest,
	}, token)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("at-sign: expected 400, got %d (body: %s)", w2.Code, w2.Body.String())
	}
}

func TestRegistryDeleteTag(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "admin")
	token := issueTestToken(t, "user-1", "alice", "admin")
	proj := createRegistryTestProject(t, st, "user-1")

	// Push and tag.
	pushResp := pushWDL(t, srv, proj.Slug, "my-tool", sampleToolWDL, token)
	w := doAuthRequest(t, srv, "POST", "/projects/"+proj.Slug+"/registry/my-tool/tags", api.RegistryTagRequest{
		Tag:    "v1.0",
		Digest: pushResp.Digest,
	}, token)
	if w.Code != http.StatusOK {
		t.Fatalf("tag: expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	// Delete the tag.
	wDel := doAuthRequest(t, srv, "DELETE", "/projects/"+proj.Slug+"/registry/my-tool/tags/v1.0", nil, token)
	if wDel.Code != http.StatusNoContent {
		t.Fatalf("delete tag: expected 204, got %d (body: %s)", wDel.Code, wDel.Body.String())
	}

	// Pull by tag — should be 404 (tag gone).
	w2 := doAuthRequest(t, srv, "GET", "/projects/"+proj.Slug+"/registry/my-tool?tag=v1.0", nil, token)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("pull by deleted tag: expected 404, got %d (body: %s)", w2.Code, w2.Body.String())
	}

	// Pull by digest — version still exists.
	w3 := doAuthRequest(t, srv, "GET", "/projects/"+proj.Slug+"/registry/my-tool?digest="+pushResp.Digest, nil, token)
	if w3.Code != http.StatusOK {
		t.Fatalf("pull by digest after tag delete: expected 200, got %d (body: %s)", w3.Code, w3.Body.String())
	}
}

// ===== List tests =====

func TestRegistryList(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "admin")
	token := issueTestToken(t, "user-1", "alice", "admin")
	proj := createRegistryTestProject(t, st, "user-1")

	// Push two entries.
	push1 := pushWDL(t, srv, proj.Slug, "tool-a", sampleToolWDL, token)
	pushWDL(t, srv, proj.Slug, "wf-b", sampleWorkflowWDL, token)

	// Tag "tool-a" version as "latest".
	w := doAuthRequest(t, srv, "POST", "/projects/"+proj.Slug+"/registry/tool-a/tags", api.RegistryTagRequest{
		Tag:    "latest",
		Digest: push1.Digest,
	}, token)
	if w.Code != http.StatusOK {
		t.Fatalf("tag: expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	// List entries.
	wList := doAuthRequest(t, srv, "GET", "/projects/"+proj.Slug+"/registry", nil, token)
	if wList.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d (body: %s)", wList.Code, wList.Body.String())
	}

	var entries []api.RegistryEntryResponse
	if err := json.NewDecoder(wList.Body).Decode(&entries); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Build map for easy lookup.
	byName := make(map[string]api.RegistryEntryResponse, 2)
	for _, e := range entries {
		byName[e.Name] = e
	}

	toolA, ok := byName["tool-a"]
	if !ok {
		t.Fatal("expected entry 'tool-a' in list")
	}
	if toolA.Kind != "tool" {
		t.Errorf("tool-a kind: got %q, want 'tool'", toolA.Kind)
	}
	if toolA.VersionCount != 1 {
		t.Errorf("tool-a version_count: got %d, want 1", toolA.VersionCount)
	}
	if len(toolA.Tags) != 1 || toolA.Tags[0] != "latest" {
		t.Errorf("tool-a tags: got %v, want [latest]", toolA.Tags)
	}

	wfB, ok := byName["wf-b"]
	if !ok {
		t.Fatal("expected entry 'wf-b' in list")
	}
	if wfB.Kind != "workflow" {
		t.Errorf("wf-b kind: got %q, want 'workflow'", wfB.Kind)
	}
	if wfB.VersionCount != 1 {
		t.Errorf("wf-b version_count: got %d, want 1", wfB.VersionCount)
	}
	if wfB.Tags == nil || len(wfB.Tags) != 0 {
		t.Errorf("wf-b tags: got %v, want []", wfB.Tags)
	}
}

func TestRegistryListVersions(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "admin")
	token := issueTestToken(t, "user-1", "alice", "admin")
	proj := createRegistryTestProject(t, st, "user-1")

	// Push two different versions.
	pushWDL(t, srv, proj.Slug, "my-tool", sampleToolWDL, token)
	push2 := pushWDL(t, srv, proj.Slug, "my-tool", sampleToolWDLv2, token)

	// Tag the second version.
	w := doAuthRequest(t, srv, "POST", "/projects/"+proj.Slug+"/registry/my-tool/tags", api.RegistryTagRequest{
		Tag:    "v2",
		Digest: push2.Digest,
	}, token)
	if w.Code != http.StatusOK {
		t.Fatalf("tag: expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	// List versions.
	wList := doAuthRequest(t, srv, "GET", "/projects/"+proj.Slug+"/registry/my-tool/versions", nil, token)
	if wList.Code != http.StatusOK {
		t.Fatalf("list versions: expected 200, got %d (body: %s)", wList.Code, wList.Body.String())
	}

	var versions []api.RegistryVersionResponse
	if err := json.NewDecoder(wList.Body).Decode(&versions); err != nil {
		t.Fatalf("decode versions response: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}

	// Find the version with the "v2" tag.
	var tagged *api.RegistryVersionResponse
	for i := range versions {
		if versions[i].Digest == push2.Digest {
			tagged = &versions[i]
			break
		}
	}
	if tagged == nil {
		t.Fatal("second version not found in versions list")
	}
	if len(tagged.Tags) != 1 || tagged.Tags[0] != "v2" {
		t.Errorf("second version tags: got %v, want [v2]", tagged.Tags)
	}
}

// ===== Delete tests =====

func TestRegistryDelete(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "admin")
	token := issueTestToken(t, "user-1", "alice", "admin")
	proj := createRegistryTestProject(t, st, "user-1")

	pushResp := pushWDL(t, srv, proj.Slug, "my-tool", sampleToolWDL, token)

	// Delete entry.
	wDel := doAuthRequest(t, srv, "DELETE", "/projects/"+proj.Slug+"/registry/my-tool", nil, token)
	if wDel.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d (body: %s)", wDel.Code, wDel.Body.String())
	}

	// List — entry must not appear.
	wList := doAuthRequest(t, srv, "GET", "/projects/"+proj.Slug+"/registry", nil, token)
	if wList.Code != http.StatusOK {
		t.Fatalf("list after delete: expected 200, got %d (body: %s)", wList.Code, wList.Body.String())
	}
	var entries []api.RegistryEntryResponse
	if err := json.NewDecoder(wList.Body).Decode(&entries); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	for _, e := range entries {
		if e.Name == "my-tool" {
			t.Errorf("expected 'my-tool' to be absent from list after delete, but it was found")
		}
	}

	// Pull by digest — must be 404 (entry soft-deleted).
	wPull := doAuthRequest(t, srv, "GET", "/projects/"+proj.Slug+"/registry/my-tool?digest="+pushResp.Digest, nil, token)
	if wPull.Code != http.StatusNotFound {
		t.Fatalf("pull after delete: expected 404, got %d (body: %s)", wPull.Code, wPull.Body.String())
	}
}

func TestRegistryDelete_Forbidden(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	// Admin sets up the project and data.
	createTestUser(t, st, "admin-1", "bob", "pass", "admin")
	adminToken := issueTestToken(t, "admin-1", "bob", "admin")

	// Executor user added to the project with executor role.
	createTestUser(t, st, "executor-1", "alice", "pass", "executor")
	executorToken := issueTestToken(t, "executor-1", "alice", "executor")

	proj := createRegistryTestProject(t, st, "admin-1")
	addProjectMember(t, st, proj.ID, "executor-1", "executor")

	// Push WDL as admin.
	pushWDL(t, srv, proj.Slug, "my-tool", sampleToolWDL, adminToken)

	// Attempt DELETE as executor — must be 403.
	wDel := doAuthRequest(t, srv, "DELETE", "/projects/"+proj.Slug+"/registry/my-tool", nil, executorToken)
	if wDel.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d (body: %s)", wDel.Code, wDel.Body.String())
	}
}

