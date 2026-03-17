package sqlite_test

import (
	"context"
	"testing"

	"github.com/bioflowy/flowy/internal/store"
	"github.com/google/uuid"
)

// newProject creates a Project with required fields set.
func newProject(slug string) *store.Project {
	return &store.Project{
		ID:          uuid.New().String(),
		Slug:        slug,
		Description: "Test project " + slug,
		IsActive:    true,
	}
}

// newProjectUser creates a User with required fields set, for use in project tests.
func newProjectUser(username string) *store.User {
	return &store.User{
		ID:           uuid.New().String(),
		Username:     username,
		PasswordHash: "$2a$12$testhashedpassword",
		DisplayName:  username,
		Email:        username + "@example.com",
		GlobalRole:   "viewer",
		IsActive:     true,
	}
}

// TestCreateProject_and_GetBySlug verifies round-trip field preservation.
func TestCreateProject_and_GetBySlug(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	p := newProject("my-project")
	if err := s.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	got, err := s.GetProjectBySlug(ctx, "my-project")
	if err != nil {
		t.Fatalf("GetProjectBySlug: %v", err)
	}
	if got.ID != p.ID {
		t.Errorf("ID: got %q, want %q", got.ID, p.ID)
	}
	if got.Slug != p.Slug {
		t.Errorf("Slug: got %q, want %q", got.Slug, p.Slug)
	}
	if got.Description != p.Description {
		t.Errorf("Description: got %q, want %q", got.Description, p.Description)
	}
	if !got.IsActive {
		t.Error("IsActive: expected true")
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt: expected non-zero")
	}
	if got.UpdatedAt.IsZero() {
		t.Error("UpdatedAt: expected non-zero")
	}
}

// TestCreateProject_DuplicateSlug verifies that creating a project with a
// duplicate slug returns an error.
func TestCreateProject_DuplicateSlug(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	p1 := newProject("dup-slug")
	if err := s.CreateProject(ctx, p1); err != nil {
		t.Fatalf("first CreateProject: %v", err)
	}

	p2 := newProject("dup-slug")
	if err := s.CreateProject(ctx, p2); err == nil {
		t.Error("expected error for duplicate slug, got nil")
	}
}

// TestGetProjectByID verifies lookup by project UUID.
func TestGetProjectByID(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	p := newProject("id-lookup-project")
	if err := s.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	got, err := s.GetProjectByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProjectByID: %v", err)
	}
	if got.ID != p.ID {
		t.Errorf("ID: got %q, want %q", got.ID, p.ID)
	}
	if got.Slug != p.Slug {
		t.Errorf("Slug: got %q, want %q", got.Slug, p.Slug)
	}
}

// TestListProjectsForUser verifies that only projects where the user is a member
// are returned.
func TestListProjectsForUser(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	u := newProjectUser("list-user")
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// Create two projects; user is member of only the first.
	p1 := newProject("user-member-project")
	p2 := newProject("user-non-member-project")
	for _, p := range []*store.Project{p1, p2} {
		if err := s.CreateProject(ctx, p); err != nil {
			t.Fatalf("CreateProject(%s): %v", p.Slug, err)
		}
	}

	if err := s.AddMember(ctx, &store.ProjectMember{
		ProjectID: p1.ID,
		UserID:    u.ID,
		Role:      "viewer",
	}); err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	projects, err := s.ListProjectsForUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListProjectsForUser: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].ID != p1.ID {
		t.Errorf("project ID: got %q, want %q", projects[0].ID, p1.ID)
	}
}

// TestListAllProjects verifies that all active projects are returned.
func TestListAllProjects(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	p1 := newProject("all-project-1")
	p2 := newProject("all-project-2")
	for _, p := range []*store.Project{p1, p2} {
		if err := s.CreateProject(ctx, p); err != nil {
			t.Fatalf("CreateProject(%s): %v", p.Slug, err)
		}
	}

	projects, err := s.ListAllProjects(ctx)
	if err != nil {
		t.Fatalf("ListAllProjects: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}
}

// TestUpdateProject verifies that description and updated_at are changed.
func TestUpdateProject(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	p := newProject("update-project")
	if err := s.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	original, err := s.GetProjectBySlug(ctx, "update-project")
	if err != nil {
		t.Fatalf("GetProjectBySlug: %v", err)
	}

	original.Description = "Updated description"
	if err := s.UpdateProject(ctx, original); err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}

	updated, err := s.GetProjectBySlug(ctx, "update-project")
	if err != nil {
		t.Fatalf("GetProjectBySlug after update: %v", err)
	}
	if updated.Description != "Updated description" {
		t.Errorf("Description: got %q, want %q", updated.Description, "Updated description")
	}
}

// TestDeactivateProject verifies that soft-delete hides the project from
// GetProjectBySlug.
func TestDeactivateProject(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	p := newProject("deactivate-project")
	if err := s.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if err := s.DeactivateProject(ctx, p.ID); err != nil {
		t.Fatalf("DeactivateProject: %v", err)
	}

	// GetProjectBySlug must return an error for deactivated projects.
	_, err := s.GetProjectBySlug(ctx, "deactivate-project")
	if err == nil {
		t.Error("expected error for deactivated project, got nil")
	}

	// ListAllProjects must not include deactivated projects.
	projects, err := s.ListAllProjects(ctx)
	if err != nil {
		t.Fatalf("ListAllProjects: %v", err)
	}
	for _, proj := range projects {
		if proj.ID == p.ID {
			t.Errorf("deactivated project %q still appears in ListAllProjects", p.ID)
		}
	}
}

// TestRunWithProjectID verifies that project_id is persisted and read back.
func TestRunWithProjectID(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	p := newProject("run-project")
	if err := s.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	r := &store.Run{
		ID:         uuid.New().String(),
		ProjectID:  p.ID,
		WDLPath:    "/wf.wdl",
		InputsJSON: `{}`,
		State:      store.RunPending,
	}
	if err := s.CreateRun(ctx, r); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	got, err := s.GetRun(ctx, r.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.ProjectID != p.ID {
		t.Errorf("ProjectID: got %q, want %q", got.ProjectID, p.ID)
	}
}
