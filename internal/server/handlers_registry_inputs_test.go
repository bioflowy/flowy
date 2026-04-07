package server_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/bioflowy/flowy/internal/api"
	"github.com/bioflowy/flowy/internal/server"
	"github.com/bioflowy/flowy/internal/store/sqlite"
)

// wdlWithInputs is a workflow WDL with String name, Int count, Boolean? verbose.
const wdlWithInputs = `version 1.0
workflow test_wf {
    input {
        String name
        Int count
        Boolean? verbose
    }
    output {
        String result = name
    }
}
`

// wdlWithDefaults is a task WDL with defaulted string and int inputs.
const wdlWithDefaults = `version 1.0
task greet {
    input {
        String greeting = "hello"
        Int num = 42
    }
    command <<<
        echo "~{greeting} ~{num}"
    >>>
    output {
        String out = read_string(stdout())
    }
}
`

// wdlEmptyInputs is a workflow with an empty input block.
const wdlEmptyInputs = `version 1.0
workflow empty_wf {
    input {
    }
    output {
    }
}
`

// pushAndTagEntry pushes WDL content under name and creates a tag.
func pushAndTagEntry(t *testing.T, srv *server.Server, projSlug, name, content, tag string, adminToken string) {
	t.Helper()
	// Push the WDL.
	pushW := doAuthRequest(t, srv, "POST",
		"/projects/"+projSlug+"/registry",
		api.RegistryPushRequest{Name: name, Content: content},
		adminToken,
	)
	if pushW.Code != http.StatusCreated && pushW.Code != http.StatusOK {
		t.Fatalf("push: expected 200/201, got %d (body: %s)", pushW.Code, pushW.Body.String())
	}
	var pushResp api.RegistryPushResponse
	if err := json.NewDecoder(pushW.Body).Decode(&pushResp); err != nil {
		t.Fatalf("decode push response: %v", err)
	}
	// Create tag.
	tagW := doAuthRequest(t, srv, "POST",
		"/projects/"+projSlug+"/registry/"+name+"/tags",
		api.RegistryTagRequest{Tag: tag, Digest: pushResp.Digest},
		adminToken,
	)
	if tagW.Code != http.StatusOK {
		t.Fatalf("tag: expected 200, got %d (body: %s)", tagW.Code, tagW.Body.String())
	}
}

// registryInputsSetup creates a test server, admin user, and project. Returns cleanup func.
func registryInputsSetup(t *testing.T) (*server.Server, *sqlite.SQLiteStore, string, string, func()) {
	t.Helper()
	srv, st, cleanup := newAuthTestServer(t)
	createTestUser(t, st, "user-1", "alice", "pass", "admin")
	adminToken := issueTestToken(t, "user-1", "alice", "admin")
	proj := createRegistryTestProject(t, st, "user-1")
	return srv, st, proj.Slug, adminToken, cleanup
}

// TestRegistryInputs verifies the basic inputs endpoint returns correct schema.
func TestRegistryInputs(t *testing.T) {
	srv, _, projSlug, adminToken, cleanup := registryInputsSetup(t)
	defer cleanup()

	pushAndTagEntry(t, srv, projSlug, "my-workflow", wdlWithInputs, "latest", adminToken)

	w := doAuthRequest(t, srv, "GET",
		"/projects/"+projSlug+"/registry/my-workflow/inputs?tag=latest",
		nil, adminToken,
	)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp api.RegistryInputsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Inputs) != 3 {
		t.Fatalf("expected 3 inputs, got %d: %+v", len(resp.Inputs), resp.Inputs)
	}

	byName := make(map[string]api.RegistryInputItem, len(resp.Inputs))
	for _, item := range resp.Inputs {
		byName[item.Name] = item
	}

	nameItem, ok := byName["name"]
	if !ok {
		t.Fatal("expected input 'name' in response")
	}
	if nameItem.Type != "String" {
		t.Errorf("name.Type: got %q, want %q", nameItem.Type, "String")
	}
	if nameItem.Optional {
		t.Error("name.Optional: expected false")
	}
	if nameItem.Default != nil {
		t.Errorf("name.Default: expected nil, got %q", *nameItem.Default)
	}

	countItem, ok := byName["count"]
	if !ok {
		t.Fatal("expected input 'count' in response")
	}
	if countItem.Type != "Int" {
		t.Errorf("count.Type: got %q, want %q", countItem.Type, "Int")
	}
	if countItem.Optional {
		t.Error("count.Optional: expected false")
	}

	verboseItem, ok := byName["verbose"]
	if !ok {
		t.Fatal("expected input 'verbose' in response")
	}
	if !verboseItem.Optional {
		t.Error("verbose.Optional: expected true")
	}
	if verboseItem.Default != nil {
		t.Errorf("verbose.Default: expected nil, got %q", *verboseItem.Default)
	}
}

// TestRegistryInputs_WithDefaults verifies that literal default values are returned.
func TestRegistryInputs_WithDefaults(t *testing.T) {
	srv, _, projSlug, adminToken, cleanup := registryInputsSetup(t)
	defer cleanup()

	pushAndTagEntry(t, srv, projSlug, "greet-task", wdlWithDefaults, "v1", adminToken)

	w := doAuthRequest(t, srv, "GET",
		"/projects/"+projSlug+"/registry/greet-task/inputs?tag=v1",
		nil, adminToken,
	)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp api.RegistryInputsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Inputs) != 2 {
		t.Fatalf("expected 2 inputs, got %d: %+v", len(resp.Inputs), resp.Inputs)
	}

	byName := make(map[string]api.RegistryInputItem, len(resp.Inputs))
	for _, item := range resp.Inputs {
		byName[item.Name] = item
	}

	greeting, ok := byName["greeting"]
	if !ok {
		t.Fatal("expected input 'greeting'")
	}
	if greeting.Default == nil {
		t.Fatal("greeting.Default: expected non-nil")
	}
	if *greeting.Default != "hello" {
		t.Errorf("greeting.Default: got %q, want %q", *greeting.Default, "hello")
	}

	num, ok := byName["num"]
	if !ok {
		t.Fatal("expected input 'num'")
	}
	if num.Default == nil {
		t.Fatal("num.Default: expected non-nil")
	}
	if *num.Default != "42" {
		t.Errorf("num.Default: got %q, want %q", *num.Default, "42")
	}
}

// TestRegistryInputs_Empty verifies that an empty input block returns {"inputs":[]}.
func TestRegistryInputs_Empty(t *testing.T) {
	srv, _, projSlug, adminToken, cleanup := registryInputsSetup(t)
	defer cleanup()

	pushAndTagEntry(t, srv, projSlug, "empty-wf", wdlEmptyInputs, "latest", adminToken)

	w := doAuthRequest(t, srv, "GET",
		"/projects/"+projSlug+"/registry/empty-wf/inputs?tag=latest",
		nil, adminToken,
	)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp api.RegistryInputsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Inputs) != 0 {
		t.Errorf("expected empty inputs slice, got %+v", resp.Inputs)
	}
}

// TestRegistryInputs_TagNotFound verifies that a missing tag returns 404.
func TestRegistryInputs_TagNotFound(t *testing.T) {
	srv, _, projSlug, adminToken, cleanup := registryInputsSetup(t)
	defer cleanup()

	pushAndTagEntry(t, srv, projSlug, "my-wf", wdlEmptyInputs, "latest", adminToken)

	w := doAuthRequest(t, srv, "GET",
		"/projects/"+projSlug+"/registry/my-wf/inputs?tag=nonexistent",
		nil, adminToken,
	)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body: %s)", w.Code, w.Body.String())
	}

	var errResp api.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if errResp.Error == "" {
		t.Error("expected non-empty error message")
	}
}

// TestRegistryInputs_MissingTagParam verifies that omitting ?tag= returns 400.
func TestRegistryInputs_MissingTagParam(t *testing.T) {
	srv, _, projSlug, adminToken, cleanup := registryInputsSetup(t)
	defer cleanup()

	pushAndTagEntry(t, srv, projSlug, "my-wf", wdlEmptyInputs, "latest", adminToken)

	w := doAuthRequest(t, srv, "GET",
		"/projects/"+projSlug+"/registry/my-wf/inputs",
		nil, adminToken,
	)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", w.Code, w.Body.String())
	}

	var errResp api.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if errResp.Error == "" {
		t.Error("expected non-empty error message")
	}
}

// TestRegistryInputs_Auth verifies that unauthenticated requests get 401, viewer gets 200.
func TestRegistryInputs_Auth(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "admin-1", "admin", "adminpass", "admin")
	createTestUser(t, st, "viewer-1", "viewer", "viewerpass", "user")

	adminToken := issueTestToken(t, "admin-1", "admin", "admin")
	proj := createRegistryTestProject(t, st, "admin-1")

	// Add viewer as project viewer.
	addProjectMember(t, st, proj.ID, "viewer-1", "viewer")

	pushAndTagEntry(t, srv, proj.Slug, "my-wf", wdlEmptyInputs, "latest", adminToken)

	// No token -> 401.
	wNoAuth := doAuthRequest(t, srv, "GET",
		"/projects/"+proj.Slug+"/registry/my-wf/inputs?tag=latest",
		nil, "",
	)
	if wNoAuth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", wNoAuth.Code)
	}

	// Viewer token -> 200.
	viewerToken := issueTestToken(t, "viewer-1", "viewer", "user")
	wViewer := doAuthRequest(t, srv, "GET",
		"/projects/"+proj.Slug+"/registry/my-wf/inputs?tag=latest",
		nil, viewerToken,
	)
	if wViewer.Code != http.StatusOK {
		t.Fatalf("expected 200 with viewer token, got %d (body: %s)", wViewer.Code, wViewer.Body.String())
	}
}
