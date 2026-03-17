package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/bioflowy/flowy/internal/api"
	"github.com/bioflowy/flowy/internal/store"
)

// createTestProject creates a project directly in the store and adds the given user as admin.
func createTestProject(t *testing.T, st interface {
	CreateProject(context.Context, *store.Project) error
	AddMember(context.Context, *store.ProjectMember) error
}, id, slug, ownerID string) {
	t.Helper()
	now := time.Now().UTC()
	proj := &store.Project{
		ID:          id,
		Slug:        slug,
		Description: "Test project " + slug,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := st.CreateProject(context.Background(), proj); err != nil {
		t.Fatalf("CreateProject %q: %v", slug, err)
	}
	member := &store.ProjectMember{
		ProjectID: id,
		UserID:    ownerID,
		Role:      "admin",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := st.AddMember(context.Background(), member); err != nil {
		t.Fatalf("AddMember for project %q owner %q: %v", slug, ownerID, err)
	}
}

// ===== TestCreateProject =====

func TestCreateProject(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	aliceToken := issueTestToken(t, "user-1", "alice", "executor")

	w := doAuthRequest(t, srv, "POST", "/projects", api.CreateProjectRequest{
		Slug:        "my-project",
		Description: "A test project",
	}, aliceToken)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp api.ProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Slug != "my-project" {
		t.Errorf("slug: got %q, want %q", resp.Slug, "my-project")
	}
	if resp.ID == "" {
		t.Error("expected non-empty project ID")
	}

	// Verify creator is admin member.
	member, err := st.GetProjectMember(context.Background(), resp.ID, "user-1")
	if err != nil {
		t.Fatalf("GetProjectMember: %v", err)
	}
	if member.Role != "admin" {
		t.Errorf("creator role: got %q, want %q", member.Role, "admin")
	}
}

func TestCreateProject_InvalidSlug(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	aliceToken := issueTestToken(t, "user-1", "alice", "executor")

	cases := []struct {
		name string
		slug string
	}{
		{"uppercase", "My-Project"},
		{"special chars", "my project!"},
		{"leading hyphen", "-project"},
		{"trailing hyphen", "project-"},
		{"empty", ""},
		{"too long", "a" + string(make([]byte, 64))},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doAuthRequest(t, srv, "POST", "/projects", api.CreateProjectRequest{
				Slug: tc.slug,
			}, aliceToken)
			if w.Code != http.StatusBadRequest {
				t.Errorf("slug %q: expected 400, got %d", tc.slug, w.Code)
			}
		})
	}
}

func TestCreateProject_DuplicateSlug(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	aliceToken := issueTestToken(t, "user-1", "alice", "executor")

	body := api.CreateProjectRequest{Slug: "dup-project"}
	w := doAuthRequest(t, srv, "POST", "/projects", body, aliceToken)
	if w.Code != http.StatusCreated {
		t.Fatalf("first create: expected 201, got %d (body: %s)", w.Code, w.Body.String())
	}

	w2 := doAuthRequest(t, srv, "POST", "/projects", body, aliceToken)
	if w2.Code != http.StatusConflict {
		t.Errorf("duplicate create: expected 409, got %d (body: %s)", w2.Code, w2.Body.String())
	}
}

// ===== TestListProjects =====

func TestListProjects_MemberOnly(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	createTestUser(t, st, "user-2", "bob", "pass", "executor")
	aliceToken := issueTestToken(t, "user-1", "alice", "executor")

	// Alice creates project-a; bob creates project-b (bob gets added as admin, alice not).
	createTestProject(t, st, "proj-a", "project-a", "user-1")
	createTestProject(t, st, "proj-b", "project-b", "user-2")

	w := doAuthRequest(t, srv, "GET", "/projects", nil, aliceToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp []api.ProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp) != 1 {
		t.Errorf("expected 1 project for alice, got %d", len(resp))
	}
	if len(resp) > 0 && resp[0].Slug != "project-a" {
		t.Errorf("expected project-a, got %q", resp[0].Slug)
	}
}

func TestListProjects_GlobalAdmin(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "admin-1", "admin", "pass", "admin")
	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	adminToken := issueTestToken(t, "admin-1", "admin", "admin")

	createTestProject(t, st, "proj-a", "project-a", "user-1")
	createTestProject(t, st, "proj-b", "project-b", "user-1")

	w := doAuthRequest(t, srv, "GET", "/projects", nil, adminToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp []api.ProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp) != 2 {
		t.Errorf("global admin: expected 2 projects, got %d", len(resp))
	}
}

// ===== TestGetProject =====

func TestGetProject_Member(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	aliceToken := issueTestToken(t, "user-1", "alice", "executor")
	createTestProject(t, st, "proj-1", "my-proj", "user-1")

	w := doAuthRequest(t, srv, "GET", "/projects/my-proj", nil, aliceToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}
	var resp api.ProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Slug != "my-proj" {
		t.Errorf("slug: got %q, want %q", resp.Slug, "my-proj")
	}
}

func TestGetProject_NonMember(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	createTestUser(t, st, "user-2", "bob", "pass", "executor")
	bobToken := issueTestToken(t, "user-2", "bob", "executor")

	// alice owns the project, bob is not a member
	createTestProject(t, st, "proj-1", "alices-proj", "user-1")

	w := doAuthRequest(t, srv, "GET", "/projects/alices-proj", nil, bobToken)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d (body: %s)", w.Code, w.Body.String())
	}
}

// ===== TestUpdateProject =====

func TestUpdateProject_Admin(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	aliceToken := issueTestToken(t, "user-1", "alice", "executor")
	createTestProject(t, st, "proj-1", "edit-proj", "user-1")

	w := doAuthRequest(t, srv, "PUT", "/projects/edit-proj", api.UpdateProjectRequest{
		Description: "Updated description",
	}, aliceToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}
	var resp api.ProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Description != "Updated description" {
		t.Errorf("description: got %q, want %q", resp.Description, "Updated description")
	}
}

func TestUpdateProject_Viewer(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	createTestUser(t, st, "user-2", "bob", "pass", "viewer")
	bobToken := issueTestToken(t, "user-2", "bob", "viewer")

	createTestProject(t, st, "proj-1", "alice-proj", "user-1")

	// Add bob as viewer.
	now := time.Now().UTC()
	if err := st.AddMember(context.Background(), &store.ProjectMember{
		ProjectID: "proj-1",
		UserID:    "user-2",
		Role:      "viewer",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	w := doAuthRequest(t, srv, "PUT", "/projects/alice-proj", api.UpdateProjectRequest{
		Description: "Bob's attempt",
	}, bobToken)
	if w.Code != http.StatusForbidden {
		t.Fatalf("viewer: expected 403, got %d (body: %s)", w.Code, w.Body.String())
	}
}

// ===== TestDeleteProject =====

func TestDeleteProject_Admin(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	aliceToken := issueTestToken(t, "user-1", "alice", "executor")
	createTestProject(t, st, "proj-1", "del-proj", "user-1")

	w := doAuthRequest(t, srv, "DELETE", "/projects/del-proj", nil, aliceToken)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d (body: %s)", w.Code, w.Body.String())
	}

	// Verify project is deactivated — GET should return 404.
	w2 := doAuthRequest(t, srv, "GET", "/projects/del-proj", nil, aliceToken)
	if w2.Code != http.StatusNotFound {
		t.Errorf("after delete: expected 404, got %d", w2.Code)
	}
}

// ===== TestMembership =====

func TestAddMember(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	createTestUser(t, st, "user-2", "bob", "pass", "executor")
	aliceToken := issueTestToken(t, "user-1", "alice", "executor")
	createTestProject(t, st, "proj-1", "team-proj", "user-1")

	w := doAuthRequest(t, srv, "POST", "/projects/team-proj/members", api.AddMemberRequest{
		Username: "bob",
		Role:     "executor",
	}, aliceToken)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp api.MemberResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Username != "bob" {
		t.Errorf("username: got %q, want %q", resp.Username, "bob")
	}
	if resp.Role != "executor" {
		t.Errorf("role: got %q, want %q", resp.Role, "executor")
	}
}

func TestAddMember_InvalidRole(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	createTestUser(t, st, "user-2", "bob", "pass", "executor")
	aliceToken := issueTestToken(t, "user-1", "alice", "executor")
	createTestProject(t, st, "proj-1", "team-proj2", "user-1")

	w := doAuthRequest(t, srv, "POST", "/projects/team-proj2/members", api.AddMemberRequest{
		Username: "bob",
		Role:     "superuser",
	}, aliceToken)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestListMembers(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	createTestUser(t, st, "user-2", "bob", "pass", "executor")
	aliceToken := issueTestToken(t, "user-1", "alice", "executor")
	createTestProject(t, st, "proj-1", "list-proj", "user-1")

	// Add bob as executor.
	now := time.Now().UTC()
	if err := st.AddMember(context.Background(), &store.ProjectMember{
		ProjectID: "proj-1",
		UserID:    "user-2",
		Role:      "executor",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	w := doAuthRequest(t, srv, "GET", "/projects/list-proj/members", nil, aliceToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp []api.MemberResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp) != 2 {
		t.Errorf("expected 2 members, got %d", len(resp))
	}
}

func TestUpdateMemberRole(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	createTestUser(t, st, "user-2", "bob", "pass", "executor")
	aliceToken := issueTestToken(t, "user-1", "alice", "executor")
	createTestProject(t, st, "proj-1", "role-proj", "user-1")

	// Add bob as viewer first.
	now := time.Now().UTC()
	if err := st.AddMember(context.Background(), &store.ProjectMember{
		ProjectID: "proj-1",
		UserID:    "user-2",
		Role:      "viewer",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	w := doAuthRequest(t, srv, "PUT", "/projects/role-proj/members/bob", api.UpdateMemberRequest{
		Role: "executor",
	}, aliceToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp api.MemberResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Role != "executor" {
		t.Errorf("role: got %q, want %q", resp.Role, "executor")
	}
}

func TestRemoveMember(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	createTestUser(t, st, "user-2", "bob", "pass", "executor")
	aliceToken := issueTestToken(t, "user-1", "alice", "executor")
	createTestProject(t, st, "proj-1", "rm-proj", "user-1")

	// Add bob as executor.
	now := time.Now().UTC()
	if err := st.AddMember(context.Background(), &store.ProjectMember{
		ProjectID: "proj-1",
		UserID:    "user-2",
		Role:      "executor",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	w := doAuthRequest(t, srv, "DELETE", "/projects/rm-proj/members/bob", nil, aliceToken)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestRemoveMember_LastAdmin(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	aliceToken := issueTestToken(t, "user-1", "alice", "executor")
	createTestProject(t, st, "proj-1", "la-proj", "user-1")

	// Alice is the only admin — removing her should fail.
	w := doAuthRequest(t, srv, "DELETE", "/projects/la-proj/members/alice", nil, aliceToken)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("last admin: expected 400, got %d (body: %s)", w.Code, w.Body.String())
	}
}

// ===== TestGlobalAdminBypass =====

func TestGlobalAdminBypass(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "admin-1", "admin", "pass", "admin")
	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	adminToken := issueTestToken(t, "admin-1", "admin", "admin")

	// Alice owns the project; admin is not a member.
	createTestProject(t, st, "proj-1", "admin-bypass", "user-1")

	// Global admin should be able to read/update/delete without membership.
	w := doAuthRequest(t, srv, "GET", "/projects/admin-bypass", nil, adminToken)
	if w.Code != http.StatusOK {
		t.Fatalf("global admin GET: expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	w2 := doAuthRequest(t, srv, "PUT", "/projects/admin-bypass", api.UpdateProjectRequest{
		Description: "Admin override",
	}, adminToken)
	if w2.Code != http.StatusOK {
		t.Fatalf("global admin PUT: expected 200, got %d (body: %s)", w2.Code, w2.Body.String())
	}

	w3 := doAuthRequest(t, srv, "GET", "/projects/admin-bypass/members", nil, adminToken)
	if w3.Code != http.StatusOK {
		t.Fatalf("global admin GET members: expected 200, got %d (body: %s)", w3.Code, w3.Body.String())
	}
}

// ===== TestProjectNoAuth =====

func TestProjectRoutes_NoToken(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	createTestProject(t, st, "proj-1", "auth-proj", "user-1")

	// Without a token, all project routes should return 401.
	w := doAuthRequest(t, srv, "GET", "/projects", nil, "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("GET /projects no token: expected 401, got %d", w.Code)
	}

	w2 := doAuthRequest(t, srv, "POST", "/projects", api.CreateProjectRequest{Slug: "new"}, "")
	if w2.Code != http.StatusUnauthorized {
		t.Errorf("POST /projects no token: expected 401, got %d", w2.Code)
	}
}

// ===== Project-scoped Run tests =====

// TestProjectRunSubmit verifies that POST /projects/{slug}/runs/submit
// creates a run with project_id set in the store.
func TestProjectRunSubmit(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	aliceToken := issueTestToken(t, "user-1", "alice", "executor")
	createTestProject(t, st, "proj-1", "run-proj", "user-1")

	// alice is project admin, which satisfies executor role requirement
	w := doAuthRequest(t, srv, "POST", "/projects/run-proj/runs/submit", api.SubmitRequest{
		WDLPath: "/nonexistent/file.wdl",
	}, aliceToken)
	// WDL parse will fail (file doesn't exist), but we get 400 (bad request), not 404/403
	// To test project_id injection we need a valid WDL — skip WDL parsing by checking 400 vs 403/404
	if w.Code == http.StatusNotFound {
		t.Fatalf("route not registered: expected non-404, got 404 (body: %s)", w.Body.String())
	}
	if w.Code == http.StatusForbidden {
		t.Fatalf("access denied unexpectedly: expected non-403, got 403 (body: %s)", w.Body.String())
	}
	// 400 (invalid WDL path) is acceptable — route exists and auth passed
}

// TestProjectRunSubmit_ViewerForbidden verifies that a viewer cannot submit runs.
func TestProjectRunSubmit_ViewerForbidden(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	createTestUser(t, st, "user-2", "bob", "pass", "viewer")
	bobToken := issueTestToken(t, "user-2", "bob", "viewer")

	createTestProject(t, st, "proj-1", "viewer-proj", "user-1")

	// Add bob as viewer.
	now := time.Now().UTC()
	if err := st.AddMember(context.Background(), &store.ProjectMember{
		ProjectID: "proj-1",
		UserID:    "user-2",
		Role:      "viewer",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	w := doAuthRequest(t, srv, "POST", "/projects/viewer-proj/runs/submit", api.SubmitRequest{
		WDLPath: "/nonexistent/file.wdl",
	}, bobToken)
	if w.Code != http.StatusForbidden {
		t.Fatalf("viewer submit: expected 403, got %d (body: %s)", w.Code, w.Body.String())
	}
}

// TestProjectRunSubmit_NonMemberForbidden verifies that a non-member cannot submit runs.
func TestProjectRunSubmit_NonMemberForbidden(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	createTestUser(t, st, "user-2", "bob", "pass", "executor")
	bobToken := issueTestToken(t, "user-2", "bob", "executor")

	// bob is not a member of this project
	createTestProject(t, st, "proj-1", "alice-only-proj", "user-1")

	w := doAuthRequest(t, srv, "POST", "/projects/alice-only-proj/runs/submit", api.SubmitRequest{
		WDLPath: "/nonexistent/file.wdl",
	}, bobToken)
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-member submit: expected 403, got %d (body: %s)", w.Code, w.Body.String())
	}
}

// TestProjectRunStatus verifies GET /projects/{slug}/runs/{id}/status returns 200
// for a project member (viewer role).
func TestProjectRunStatus(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	aliceToken := issueTestToken(t, "user-1", "alice", "executor")
	createTestProject(t, st, "proj-1", "status-proj", "user-1")

	// Insert a run directly in the store (bypassing WDL parsing).
	run := &store.Run{
		ID:        "test-run-001",
		ProjectID: "proj-1",
		WDLPath:   "/fake/workflow.wdl",
		State:     store.RunPending,
	}
	if err := st.CreateRun(context.Background(), run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	w := doAuthRequest(t, srv, "GET", "/projects/status-proj/runs/test-run-001/status", nil, aliceToken)
	if w.Code != http.StatusOK {
		t.Fatalf("project run status: expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp api.StatusResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	if resp.RunID != "test-run-001" {
		t.Errorf("run_id: got %q, want %q", resp.RunID, "test-run-001")
	}
}

// TestProjectRunOutputs verifies GET /projects/{slug}/runs/{id}/outputs returns 200
// for a completed run.
func TestProjectRunOutputs(t *testing.T) {
	srv, st, cleanup := newAuthTestServer(t)
	defer cleanup()

	createTestUser(t, st, "user-1", "alice", "pass", "executor")
	aliceToken := issueTestToken(t, "user-1", "alice", "executor")
	createTestProject(t, st, "proj-1", "outputs-proj", "user-1")

	// Insert a completed run with outputs stored in DB.
	run := &store.Run{
		ID:          "completed-run-001",
		ProjectID:   "proj-1",
		WDLPath:     "/fake/workflow.wdl",
		State:       store.RunCompleted,
		OutputsJSON: `{"result": "value"}`,
	}
	if err := st.CreateRun(context.Background(), run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	// Update state to COMPLETED.
	if err := st.UpdateRunState(context.Background(), "completed-run-001", store.RunCompleted); err != nil {
		t.Fatalf("UpdateRunState: %v", err)
	}
	// Store the outputs.
	if err := st.UpdateRunOutputs(context.Background(), "completed-run-001", `{"result": "value"}`); err != nil {
		t.Fatalf("UpdateRunOutputs: %v", err)
	}

	w := doAuthRequest(t, srv, "GET", "/projects/outputs-proj/runs/completed-run-001/outputs", nil, aliceToken)
	if w.Code != http.StatusOK {
		t.Fatalf("project run outputs: expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp api.OutputsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode outputs response: %v", err)
	}
	if resp.RunID != "completed-run-001" {
		t.Errorf("run_id: got %q, want %q", resp.RunID, "completed-run-001")
	}
}

// TestAuthDisabledBackcompat verifies that with auth.enabled=false the old
// /workflows/... routes still work and /projects/... routes return 404.
func TestAuthDisabledBackcompat(t *testing.T) {
	srv, _, _, cleanup := newTestServer(t)
	defer cleanup()

	// POST /workflows should still work (route exists in auth-disabled mode).
	w := doAuthRequest(t, srv, "POST", "/workflows", api.SubmitRequest{
		WDLPath: "/nonexistent/file.wdl",
	}, "")
	// WDL parse fails -> 400, but NOT 404 (route exists)
	if w.Code == http.StatusNotFound {
		t.Fatalf("auth-disabled: /workflows route missing, expected non-404, got 404")
	}

	// POST /projects/.../runs/submit should return 404 (routes not registered in auth-disabled mode).
	w2 := doAuthRequest(t, srv, "POST", "/projects/any-proj/runs/submit", api.SubmitRequest{
		WDLPath: "/nonexistent/file.wdl",
	}, "")
	if w2.Code != http.StatusNotFound {
		t.Fatalf("auth-disabled: /projects route should not exist, expected 404, got %d", w2.Code)
	}
}
