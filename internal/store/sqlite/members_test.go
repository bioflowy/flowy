package sqlite_test

import (
	"context"
	"testing"

	"github.com/bioflowy/flowy/internal/store"
)

// setupProjectAndUser creates a project and user for membership tests.
// Returns (project, user).
func setupProjectAndUser(t *testing.T, s interface {
	CreateProject(context.Context, *store.Project) error
	CreateUser(context.Context, *store.User) error
}, slug, username string) (*store.Project, *store.User) {
	t.Helper()
	ctx := context.Background()

	p := newProject(slug)
	if err := s.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject(%s): %v", slug, err)
	}
	u := newProjectUser(username)
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser(%s): %v", username, err)
	}
	return p, u
}

// TestAddMember_and_Get verifies AddMember + GetProjectMember round-trip.
func TestAddMember_and_Get(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	p, u := setupProjectAndUser(t, s, "addmember-project", "addmember-user")

	m := &store.ProjectMember{
		ProjectID: p.ID,
		UserID:    u.ID,
		Role:      "executor",
	}
	if err := s.AddMember(ctx, m); err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	got, err := s.GetProjectMember(ctx, p.ID, u.ID)
	if err != nil {
		t.Fatalf("GetProjectMember: %v", err)
	}
	if got.ProjectID != p.ID {
		t.Errorf("ProjectID: got %q, want %q", got.ProjectID, p.ID)
	}
	if got.UserID != u.ID {
		t.Errorf("UserID: got %q, want %q", got.UserID, u.ID)
	}
	if got.Role != "executor" {
		t.Errorf("Role: got %q, want %q", got.Role, "executor")
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt: expected non-zero")
	}
}

// TestAddMember_Duplicate verifies that adding the same user twice returns an error.
func TestAddMember_Duplicate(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	p, u := setupProjectAndUser(t, s, "dup-member-project", "dup-member-user")

	m := &store.ProjectMember{ProjectID: p.ID, UserID: u.ID, Role: "viewer"}
	if err := s.AddMember(ctx, m); err != nil {
		t.Fatalf("first AddMember: %v", err)
	}
	if err := s.AddMember(ctx, m); err == nil {
		t.Error("expected error for duplicate member, got nil")
	}
}

// TestUpdateMemberRole verifies that UpdateMemberRole changes the member's role.
func TestUpdateMemberRole(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	p, u := setupProjectAndUser(t, s, "updaterole-project", "updaterole-user")

	if err := s.AddMember(ctx, &store.ProjectMember{
		ProjectID: p.ID, UserID: u.ID, Role: "viewer",
	}); err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	if err := s.UpdateMemberRole(ctx, p.ID, u.ID, "admin"); err != nil {
		t.Fatalf("UpdateMemberRole: %v", err)
	}

	got, err := s.GetProjectMember(ctx, p.ID, u.ID)
	if err != nil {
		t.Fatalf("GetProjectMember: %v", err)
	}
	if got.Role != "admin" {
		t.Errorf("Role: got %q, want %q", got.Role, "admin")
	}
}

// TestRemoveMember verifies that RemoveMember deletes the membership.
func TestRemoveMember(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	p, u := setupProjectAndUser(t, s, "removemember-project", "removemember-user")

	if err := s.AddMember(ctx, &store.ProjectMember{
		ProjectID: p.ID, UserID: u.ID, Role: "viewer",
	}); err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	if err := s.RemoveMember(ctx, p.ID, u.ID); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}

	// GetProjectMember should now return an error.
	_, err := s.GetProjectMember(ctx, p.ID, u.ID)
	if err == nil {
		t.Error("expected error after RemoveMember, got nil")
	}
}

// TestListMembers verifies that ListMembers returns all members of a project.
func TestListMembers(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	p := newProject("listmembers-project")
	if err := s.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	u1 := newProjectUser("listmember-user1")
	u2 := newProjectUser("listmember-user2")
	for _, u := range []*store.User{u1, u2} {
		if err := s.CreateUser(ctx, u); err != nil {
			t.Fatalf("CreateUser(%s): %v", u.Username, err)
		}
		if err := s.AddMember(ctx, &store.ProjectMember{
			ProjectID: p.ID, UserID: u.ID, Role: "viewer",
		}); err != nil {
			t.Fatalf("AddMember(%s): %v", u.Username, err)
		}
	}

	members, err := s.ListMembers(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}
}

// TestCountProjectAdmins verifies the admin count is accurate.
func TestCountProjectAdmins(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	p := newProject("countadmins-project")
	if err := s.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	u1 := newProjectUser("admin-user1")
	u2 := newProjectUser("admin-user2")
	u3 := newProjectUser("viewer-user1")
	for _, u := range []*store.User{u1, u2, u3} {
		if err := s.CreateUser(ctx, u); err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
	}

	if err := s.AddMember(ctx, &store.ProjectMember{ProjectID: p.ID, UserID: u1.ID, Role: "admin"}); err != nil {
		t.Fatalf("AddMember u1: %v", err)
	}
	if err := s.AddMember(ctx, &store.ProjectMember{ProjectID: p.ID, UserID: u2.ID, Role: "admin"}); err != nil {
		t.Fatalf("AddMember u2: %v", err)
	}
	if err := s.AddMember(ctx, &store.ProjectMember{ProjectID: p.ID, UserID: u3.ID, Role: "viewer"}); err != nil {
		t.Fatalf("AddMember u3: %v", err)
	}

	count, err := s.CountProjectAdmins(ctx, p.ID)
	if err != nil {
		t.Fatalf("CountProjectAdmins: %v", err)
	}
	if count != 2 {
		t.Errorf("admin count: got %d, want 2", count)
	}
}

// TestRemoveMemberIfNotLastAdmin_Blocked verifies that removing the last admin
// returns an error.
func TestRemoveMemberIfNotLastAdmin_Blocked(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	p, u := setupProjectAndUser(t, s, "lastadmin-project", "lastadmin-user")

	if err := s.AddMember(ctx, &store.ProjectMember{
		ProjectID: p.ID, UserID: u.ID, Role: "admin",
	}); err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	// Single admin — removal must be rejected.
	err := s.RemoveMemberIfNotLastAdmin(ctx, p.ID, u.ID)
	if err == nil {
		t.Error("expected error when removing last admin, got nil")
	}

	// Confirm the member still exists.
	_, err = s.GetProjectMember(ctx, p.ID, u.ID)
	if err != nil {
		t.Errorf("last admin should still be a member after blocked removal: %v", err)
	}
}

// TestRemoveMemberIfNotLastAdmin_Allowed verifies that removing a non-last admin
// succeeds.
func TestRemoveMemberIfNotLastAdmin_Allowed(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	p := newProject("twoadmins-project")
	if err := s.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	u1 := newProjectUser("twoadmins-user1")
	u2 := newProjectUser("twoadmins-user2")
	for _, u := range []*store.User{u1, u2} {
		if err := s.CreateUser(ctx, u); err != nil {
			t.Fatalf("CreateUser(%s): %v", u.Username, err)
		}
		if err := s.AddMember(ctx, &store.ProjectMember{
			ProjectID: p.ID, UserID: u.ID, Role: "admin",
		}); err != nil {
			t.Fatalf("AddMember(%s): %v", u.Username, err)
		}
	}

	// With two admins, removing one must succeed.
	if err := s.RemoveMemberIfNotLastAdmin(ctx, p.ID, u1.ID); err != nil {
		t.Fatalf("RemoveMemberIfNotLastAdmin: %v", err)
	}

	// u1 should no longer be a member.
	_, err := s.GetProjectMember(ctx, p.ID, u1.ID)
	if err == nil {
		t.Error("expected error for removed member, got nil")
	}

	// u2 should still be a member.
	_, err = s.GetProjectMember(ctx, p.ID, u2.ID)
	if err != nil {
		t.Errorf("remaining admin should still be a member: %v", err)
	}
}
