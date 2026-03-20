package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/bioflowy/flowy/internal/api"
	"github.com/bioflowy/flowy/internal/server"
	"github.com/bioflowy/flowy/internal/store/sqlite"
)

// pushRegistryEntry is a helper that pushes WDL content to the registry,
// tags it with "latest", and returns the push response.
func pushRegistryEntry(t *testing.T, srv *server.Server, slug, name, content, token string) api.RegistryPushResponse {
	t.Helper()
	pushResp := pushWDL(t, srv, slug, name, content, token)

	// Tag the version as "latest" via the API.
	wTag := doAuthRequest(t, srv, "POST", "/projects/"+slug+"/registry/"+name+"/tags", api.RegistryTagRequest{
		Tag:    "latest",
		Digest: pushResp.Digest,
	}, token)
	if wTag.Code != http.StatusOK {
		t.Fatalf("pushRegistryEntry: tag as latest: expected 200, got %d (body: %s)", wTag.Code, wTag.Body.String())
	}
	return pushResp
}

// newRegistryExecFixture creates an auth-enabled server and sets up one admin user with a project.
// Returns (srv, st, cleanup, userID, token, projectSlug).
func newRegistryExecFixture(t *testing.T) (*server.Server, *sqlite.SQLiteStore, func(), string, string, string) {
	t.Helper()
	srv, st, cleanup := newAuthTestServer(t)
	createTestUser(t, st, "exec-user-1", "alice", "pass", "admin")
	token := issueTestToken(t, "exec-user-1", "alice", "admin")
	proj := createRegistryTestProject(t, st, "exec-user-1")
	return srv, st, cleanup, "exec-user-1", token, proj.Slug
}

// TestHandleSubmitRegistryRef verifies that POST /projects/{slug}/workflows with
// registry_ref resolves the WDL from the registry and returns 201.
func TestHandleSubmitRegistryRef(t *testing.T) {
	srv, _, cleanup, _, token, slug := newRegistryExecFixture(t)
	defer cleanup()

	pushRegistryEntry(t, srv, slug, "my-wf", sampleWorkflowWDL, token)

	w := doAuthRequest(t, srv, "POST", "/projects/"+slug+"/workflows", api.SubmitRequest{
		RegistryRef: "my-wf:latest",
		InputsJSON:  map[string]interface{}{"greet.name": "world"},
	}, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", w.Code, w.Body.String())
	}
	var resp api.SubmitResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.RunID == "" {
		t.Error("expected non-empty run_id")
	}
}

// TestHandleSubmitRegistryRefDigest verifies that POST with a digest-form registry_ref
// (name@sha256:...) also works and returns 201.
func TestHandleSubmitRegistryRefDigest(t *testing.T) {
	srv, _, cleanup, _, token, slug := newRegistryExecFixture(t)
	defer cleanup()

	pushResp := pushRegistryEntry(t, srv, slug, "my-wf", sampleWorkflowWDL, token)

	w := doAuthRequest(t, srv, "POST", "/projects/"+slug+"/workflows", api.SubmitRequest{
		RegistryRef: "my-wf@" + pushResp.Digest,
		InputsJSON:  map[string]interface{}{"greet.name": "world"},
	}, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", w.Code, w.Body.String())
	}
}

// TestHandleSubmitBothFields verifies that providing both wdl_path and registry_ref
// returns 400 with an appropriate message.
func TestHandleSubmitBothFields(t *testing.T) {
	srv, _, cleanup, _, token, slug := newRegistryExecFixture(t)
	defer cleanup()

	w := doAuthRequest(t, srv, "POST", "/projects/"+slug+"/workflows", api.SubmitRequest{
		WDLPath:     "/some/path.wdl",
		RegistryRef: "my-wf:latest",
		InputsJSON:  map[string]interface{}{},
	}, token)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", w.Code, w.Body.String())
	}
	var errResp api.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if !strings.Contains(errResp.Error, "exactly one of wdl_path or registry_ref") {
		t.Errorf("expected error to mention mutual exclusion, got: %q", errResp.Error)
	}
}

// TestHandleSubmitNeitherField verifies that providing neither wdl_path nor registry_ref
// returns 400.
func TestHandleSubmitNeitherField(t *testing.T) {
	srv, _, cleanup, _, token, slug := newRegistryExecFixture(t)
	defer cleanup()

	w := doAuthRequest(t, srv, "POST", "/projects/"+slug+"/workflows", api.SubmitRequest{
		InputsJSON: map[string]interface{}{},
	}, token)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", w.Code, w.Body.String())
	}
	var errResp api.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if !strings.Contains(errResp.Error, "one of wdl_path or registry_ref is required") {
		t.Errorf("expected error to mention required field, got: %q", errResp.Error)
	}
}

// TestHandleSubmitRegistryRefNoProject verifies that POST /workflows (non-project endpoint)
// with registry_ref returns 400 with a message about project-scoped endpoints.
func TestHandleSubmitRegistryRefNoProject(t *testing.T) {
	srv, _, _, cleanup := newTestServer(t)
	defer cleanup()

	w := doRequest(t, srv, "POST", "/workflows", api.SubmitRequest{
		RegistryRef: "my-wf:latest",
		InputsJSON:  map[string]interface{}{},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", w.Code, w.Body.String())
	}
	var errResp api.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if !strings.Contains(errResp.Error, "registry_ref requires a project-scoped endpoint") {
		t.Errorf("expected error about project-scoped endpoint, got: %q", errResp.Error)
	}
}

// TestHandleSubmitRegistryRefNotFound verifies that referencing a nonexistent registry
// entry returns 422.
func TestHandleSubmitRegistryRefNotFound(t *testing.T) {
	srv, _, cleanup, _, token, slug := newRegistryExecFixture(t)
	defer cleanup()

	w := doAuthRequest(t, srv, "POST", "/projects/"+slug+"/workflows", api.SubmitRequest{
		RegistryRef: "nonexistent:latest",
		InputsJSON:  map[string]interface{}{},
	}, token)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d (body: %s)", w.Code, w.Body.String())
	}
}

// TestHandleTaskRunRegistryRef verifies that POST /projects/{slug}/tasks/run with
// registry_ref resolves WDL from the registry and returns 201.
func TestHandleTaskRunRegistryRef(t *testing.T) {
	srv, _, cleanup, _, token, slug := newRegistryExecFixture(t)
	defer cleanup()

	pushRegistryEntry(t, srv, slug, "my-tool", sampleToolWDL, token)

	w := doAuthRequest(t, srv, "POST", "/projects/"+slug+"/tasks/run", api.TaskRunRequest{
		RegistryRef: "my-tool:latest",
		TaskName:    "hello",
		InputsJSON:  map[string]interface{}{"name": "world"},
	}, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", w.Code, w.Body.String())
	}
	var resp api.TaskRunResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.RunID == "" {
		t.Error("expected non-empty run_id")
	}
}

// TestHandleTaskRunBothFields verifies that POST with both wdl_path and registry_ref returns 400.
func TestHandleTaskRunBothFields(t *testing.T) {
	srv, _, cleanup, _, token, slug := newRegistryExecFixture(t)
	defer cleanup()

	w := doAuthRequest(t, srv, "POST", "/projects/"+slug+"/tasks/run", api.TaskRunRequest{
		WDLPath:     "/some/path.wdl",
		RegistryRef: "my-tool:latest",
		TaskName:    "hello",
		InputsJSON:  map[string]interface{}{},
	}, token)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", w.Code, w.Body.String())
	}
	var errResp api.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if !strings.Contains(errResp.Error, "exactly one of wdl_path or registry_ref") {
		t.Errorf("expected error to mention mutual exclusion, got: %q", errResp.Error)
	}
}

// TestHandleClaimRegistryRun verifies that after submitting a task run via registry_ref,
// claiming the task returns non-empty WDLContent containing the expected task definition.
func TestHandleClaimRegistryRun(t *testing.T) {
	srv, _, cleanup, _, token, slug := newRegistryExecFixture(t)
	defer cleanup()

	pushRegistryEntry(t, srv, slug, "my-tool", sampleToolWDL, token)

	// Submit task run via registry_ref.
	wRun := doAuthRequest(t, srv, "POST", "/projects/"+slug+"/tasks/run", api.TaskRunRequest{
		RegistryRef: "my-tool:latest",
		TaskName:    "hello",
		InputsJSON:  map[string]interface{}{"name": "world"},
	}, token)
	if wRun.Code != http.StatusCreated {
		t.Fatalf("task run submit: expected 201, got %d (body: %s)", wRun.Code, wRun.Body.String())
	}

	// Claim the task using the daemon token.
	wClaim := doAuthRequest(t, srv, "POST", "/tasks/claim", api.TaskClaimRequest{DaemonID: "daemon-001"}, testDaemonToken)
	if wClaim.Code != http.StatusOK {
		t.Fatalf("claim: expected 200, got %d (body: %s)", wClaim.Code, wClaim.Body.String())
	}

	var claimResp api.TaskClaimResponse
	if err := json.NewDecoder(wClaim.Body).Decode(&claimResp); err != nil {
		t.Fatalf("decode claim response: %v", err)
	}

	if claimResp.WDLContent == "" {
		t.Error("expected non-empty WDLContent for registry-based run")
	}
	if !strings.Contains(claimResp.WDLContent, "task hello") {
		t.Errorf("WDLContent should contain 'task hello', got: %q", claimResp.WDLContent[:min(len(claimResp.WDLContent), 100)])
	}
}

// TestRegistryRunPersistence verifies that after submitting via registry_ref, the
// stored Run record has non-empty RegistryRef and a sha256: RegistryDigest.
func TestRegistryRunPersistence(t *testing.T) {
	srv, st, cleanup, _, token, slug := newRegistryExecFixture(t)
	defer cleanup()

	pushRegistryEntry(t, srv, slug, "my-tool", sampleToolWDL, token)

	wRun := doAuthRequest(t, srv, "POST", "/projects/"+slug+"/tasks/run", api.TaskRunRequest{
		RegistryRef: "my-tool:latest",
		TaskName:    "hello",
		InputsJSON:  map[string]interface{}{"name": "world"},
	}, token)
	if wRun.Code != http.StatusCreated {
		t.Fatalf("task run submit: expected 201, got %d (body: %s)", wRun.Code, wRun.Body.String())
	}

	var resp api.TaskRunResponse
	if err := json.NewDecoder(wRun.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	run, err := st.GetRun(context.Background(), resp.RunID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if run.RegistryRef != "my-tool:latest" {
		t.Errorf("RegistryRef: got %q, want %q", run.RegistryRef, "my-tool:latest")
	}
	if !strings.HasPrefix(run.RegistryDigest, "sha256:") {
		t.Errorf("RegistryDigest: got %q, expected sha256: prefix", run.RegistryDigest)
	}
}
