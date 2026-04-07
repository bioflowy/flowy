package sqlite_test

import (
	"context"
	"strings"
	"testing"

	"github.com/bioflowy/flowy/internal/store"
)

// TestCreateRunWithRegistryRef verifies that CreateRun persists RegistryRef and
// RegistryDigest fields, and that GetRun retrieves them correctly.
func TestCreateRunWithRegistryRef(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	run := &store.Run{
		ID:             "registry-run-001",
		WDLPath:        "",
		RegistryRef:    "hello:prod",
		RegistryDigest: "sha256:abc123",
		InputsJSON:     `{}`,
		State:          store.RunPending,
	}
	if err := s.CreateRun(ctx, run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	got, err := s.GetRun(ctx, "registry-run-001")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.RegistryRef != "hello:prod" {
		t.Errorf("RegistryRef: got %q, want %q", got.RegistryRef, "hello:prod")
	}
	if got.RegistryDigest != "sha256:abc123" {
		t.Errorf("RegistryDigest: got %q, want %q", got.RegistryDigest, "sha256:abc123")
	}
}

// TestCreateRunWithEmptyRegistryRef verifies that a file-path run (empty RegistryRef)
// round-trips through the store with empty strings for the registry fields.
func TestCreateRunWithEmptyRegistryRef(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	run := &store.Run{
		ID:             "file-run-001",
		WDLPath:        "/path/to/file.wdl",
		RegistryRef:    "",
		RegistryDigest: "",
		InputsJSON:     `{}`,
		State:          store.RunPending,
	}
	if err := s.CreateRun(ctx, run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	got, err := s.GetRun(ctx, "file-run-001")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.RegistryRef != "" {
		t.Errorf("RegistryRef: got %q, want empty string", got.RegistryRef)
	}
	if got.RegistryDigest != "" {
		t.Errorf("RegistryDigest: got %q, want empty string", got.RegistryDigest)
	}
}

// TestGetRunByPrefixRegistryFields verifies that GetRunByPrefix returns
// RegistryRef and RegistryDigest correctly.
func TestGetRunByPrefixRegistryFields(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	run := &store.Run{
		ID:             "prefix-registry-run-aabbcc",
		WDLPath:        "",
		RegistryRef:    "align-reads:prod",
		RegistryDigest: "sha256:deadbeef",
		InputsJSON:     `{}`,
		State:          store.RunPending,
	}
	if err := s.CreateRun(ctx, run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	got, err := s.GetRunByPrefix(ctx, "prefix-registry-run")
	if err != nil {
		t.Fatalf("GetRunByPrefix: %v", err)
	}
	if got.RegistryRef != "align-reads:prod" {
		t.Errorf("RegistryRef: got %q, want %q", got.RegistryRef, "align-reads:prod")
	}
	if !strings.HasPrefix(got.RegistryDigest, "sha256:") {
		t.Errorf("RegistryDigest: got %q, expected sha256: prefix", got.RegistryDigest)
	}
}

// TestListRunsByProject_Empty verifies that ListRunsByProject on a project with
// no runs returns an empty (non-nil) slice and no error.
func TestListRunsByProject_Empty(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	runs, err := s.ListRunsByProject(ctx, "nonexistent-project-id")
	if err != nil {
		t.Fatalf("ListRunsByProject: unexpected error: %v", err)
	}
	if runs == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(runs) != 0 {
		t.Errorf("expected 0 runs, got %d", len(runs))
	}
}

// TestListRunsByProject_OrderedNewestFirst verifies that ListRunsByProject returns
// runs in created_at DESC order (newest run first).
func TestListRunsByProject_OrderedNewestFirst(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	db := s.WriterDB()

	// Create the project first (foreign key constraint).
	p := newProject("ordered-project")
	if err := s.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	// Create 3 runs and backdate them via SQL to control ordering.
	for i, id := range []string{"run-old", "run-mid", "run-new"} {
		r := &store.Run{
			ID:         id,
			ProjectID:  p.ID,
			WDLPath:    "/wf.wdl",
			InputsJSON: `{}`,
			State:      store.RunPending,
		}
		if err := s.CreateRun(ctx, r); err != nil {
			t.Fatalf("CreateRun(%s): %v", id, err)
		}
		// Backdate: run-old gets -2h, run-mid gets -1h, run-new is 'now'.
		offset := (2 - i) * 60 // minutes to subtract: 120, 60, 0
		if offset > 0 {
			_, err := db.ExecContext(ctx,
				"UPDATE runs SET created_at = datetime('now', ? || ' minutes') WHERE id = ?",
				-offset, id)
			if err != nil {
				t.Fatalf("backdate %s: %v", id, err)
			}
		}
	}

	runs, err := s.ListRunsByProject(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListRunsByProject: %v", err)
	}
	if len(runs) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(runs))
	}
	// Newest first: run-new, run-mid, run-old.
	wantOrder := []string{"run-new", "run-mid", "run-old"}
	for i, want := range wantOrder {
		if runs[i].ID != want {
			t.Errorf("runs[%d].ID = %q, want %q", i, runs[i].ID, want)
		}
	}
}

// TestListRunsByProject_ScopedToProject verifies that ListRunsByProject only
// returns runs belonging to the requested project.
func TestListRunsByProject_ScopedToProject(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	// Create two projects (foreign key constraint).
	pA := newProject("project-scope-a")
	pB := newProject("project-scope-b")
	if err := s.CreateProject(ctx, pA); err != nil {
		t.Fatalf("CreateProject(A): %v", err)
	}
	if err := s.CreateProject(ctx, pB); err != nil {
		t.Fatalf("CreateProject(B): %v", err)
	}

	runsA := []string{"run-a1", "run-a2"}
	runsB := []string{"run-b1"}

	for _, id := range runsA {
		if err := s.CreateRun(ctx, &store.Run{
			ID: id, ProjectID: pA.ID, WDLPath: "/wf.wdl",
			InputsJSON: `{}`, State: store.RunPending,
		}); err != nil {
			t.Fatalf("CreateRun(%s): %v", id, err)
		}
	}
	for _, id := range runsB {
		if err := s.CreateRun(ctx, &store.Run{
			ID: id, ProjectID: pB.ID, WDLPath: "/wf.wdl",
			InputsJSON: `{}`, State: store.RunPending,
		}); err != nil {
			t.Fatalf("CreateRun(%s): %v", id, err)
		}
	}

	gotA, err := s.ListRunsByProject(ctx, pA.ID)
	if err != nil {
		t.Fatalf("ListRunsByProject(A): %v", err)
	}
	if len(gotA) != 2 {
		t.Errorf("expected 2 runs for project A, got %d", len(gotA))
	}
	for _, r := range gotA {
		if r.ProjectID != pA.ID {
			t.Errorf("run %s has ProjectID %q, want %q", r.ID, r.ProjectID, pA.ID)
		}
	}

	gotB, err := s.ListRunsByProject(ctx, pB.ID)
	if err != nil {
		t.Fatalf("ListRunsByProject(B): %v", err)
	}
	if len(gotB) != 1 {
		t.Errorf("expected 1 run for project B, got %d", len(gotB))
	}
}
