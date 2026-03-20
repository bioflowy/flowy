package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/bioflowy/flowy/internal/store"
)

// setupProvenanceFixture creates the prerequisite rows required by FK constraints
// before inserting a provenance record. Returns (runID, userID, entryID, versionID).
func setupProvenanceFixture(t *testing.T, s interface {
	store.Store
}) (runID, userID, entryID, versionID string) {
	t.Helper()
	ctx := context.Background()

	userID = uuid.New().String()
	user := &store.User{
		ID:           userID,
		Username:     "prov-user-" + userID[:8],
		PasswordHash: "hash",
		GlobalRole:   "executor",
		IsActive:     true,
	}
	if err := s.CreateUser(ctx, user); err != nil {
		t.Fatalf("setupProvenanceFixture: CreateUser: %v", err)
	}

	projID := uuid.New().String()
	proj := &store.Project{
		ID:       projID,
		Slug:     "prov-proj-" + projID[:8],
		IsActive: true,
	}
	if err := s.CreateProject(ctx, proj); err != nil {
		t.Fatalf("setupProvenanceFixture: CreateProject: %v", err)
	}

	digest := "sha256:provtest" + uuid.New().String()[:8]
	content := `version 1.0
workflow prov_wf {}
`
	version, _, err := s.PushRegistryVersion(ctx, projID, "prov-wf", "workflow", digest, content)
	if err != nil {
		t.Fatalf("setupProvenanceFixture: PushRegistryVersion: %v", err)
	}

	// Retrieve the entry ID via GetRegistryEntry.
	entry, err := s.GetRegistryEntry(ctx, projID, "prov-wf")
	if err != nil {
		t.Fatalf("setupProvenanceFixture: GetRegistryEntry: %v", err)
	}
	entryID = entry.ID
	versionID = version.ID

	runID = uuid.New().String()
	run := &store.Run{
		ID:             runID,
		ProjectID:      projID,
		RegistryRef:    "prov-wf:latest",
		RegistryDigest: digest,
		InputsJSON:     `{}`,
		State:          store.RunPending,
	}
	if err := s.CreateRun(ctx, run); err != nil {
		t.Fatalf("setupProvenanceFixture: CreateRun: %v", err)
	}

	return runID, userID, entryID, versionID
}

// TestProvenanceCreate inserts prerequisite rows, calls CreateProvenance, then
// verifies the record exists with correct fields via GetProvenance.
func TestProvenanceCreate(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	runID, userID, entryID, versionID := setupProvenanceFixture(t, s)

	prov := &store.Provenance{
		ID:                uuid.New().String(),
		RunID:             runID,
		UserID:            userID,
		RegistryEntryID:   entryID,
		RegistryVersionID: versionID,
		RegistryRef:       "prov-wf:latest",
		RegistryDigest:    "sha256:provtest",
	}
	if err := s.CreateProvenance(ctx, prov); err != nil {
		t.Fatalf("CreateProvenance: %v", err)
	}

	// GetProvenance requires projectID — fetch it from the run.
	run, err := s.GetRun(ctx, runID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}

	detail, err := s.GetProvenance(ctx, run.ProjectID, runID)
	if err != nil {
		t.Fatalf("GetProvenance: %v", err)
	}

	if detail.RunID != runID {
		t.Errorf("RunID: got %q, want %q", detail.RunID, runID)
	}
	if detail.RegistryRef != "prov-wf:latest" {
		t.Errorf("RegistryRef: got %q, want %q", detail.RegistryRef, "prov-wf:latest")
	}
	if detail.RegistryDigest != "sha256:provtest" {
		t.Errorf("RegistryDigest: got %q, want %q", detail.RegistryDigest, "sha256:provtest")
	}
}

// TestProvenanceCreateDuplicate verifies that inserting a second provenance record
// for the same run_id returns a non-nil error (UNIQUE constraint violation).
func TestProvenanceCreateDuplicate(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	runID, userID, entryID, versionID := setupProvenanceFixture(t, s)

	prov1 := &store.Provenance{
		ID:                uuid.New().String(),
		RunID:             runID,
		UserID:            userID,
		RegistryEntryID:   entryID,
		RegistryVersionID: versionID,
		RegistryRef:       "prov-wf:latest",
		RegistryDigest:    "sha256:provtest",
	}
	if err := s.CreateProvenance(ctx, prov1); err != nil {
		t.Fatalf("CreateProvenance first: %v", err)
	}

	prov2 := &store.Provenance{
		ID:                uuid.New().String(), // different ID
		RunID:             runID,               // same run_id => UNIQUE violation
		UserID:            userID,
		RegistryEntryID:   entryID,
		RegistryVersionID: versionID,
		RegistryRef:       "prov-wf:latest",
		RegistryDigest:    "sha256:provtest",
	}
	if err := s.CreateProvenance(ctx, prov2); err == nil {
		t.Error("expected error for duplicate run_id, got nil")
	}
}

// TestProvenanceTouchUpdatedAt verifies that TouchProvenanceUpdatedAt bumps updated_at
// to a value after created_at.
func TestProvenanceTouchUpdatedAt(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	runID, userID, entryID, versionID := setupProvenanceFixture(t, s)

	prov := &store.Provenance{
		ID:                uuid.New().String(),
		RunID:             runID,
		UserID:            userID,
		RegistryEntryID:   entryID,
		RegistryVersionID: versionID,
		RegistryRef:       "prov-wf:latest",
		RegistryDigest:    "sha256:provtest",
	}
	if err := s.CreateProvenance(ctx, prov); err != nil {
		t.Fatalf("CreateProvenance: %v", err)
	}

	// Backdate created_at in the DB so updated_at can advance past it.
	run, err := s.GetRun(ctx, runID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	writerDB := s.WriterDB()
	_, err = writerDB.ExecContext(ctx,
		"UPDATE provenance SET created_at = datetime('now', '-5 seconds') WHERE run_id = ?",
		runID)
	if err != nil {
		t.Fatalf("backdate created_at: %v", err)
	}

	// Brief sleep to ensure datetime resolution.
	time.Sleep(1100 * time.Millisecond)

	if err := s.TouchProvenanceUpdatedAt(ctx, runID); err != nil {
		t.Fatalf("TouchProvenanceUpdatedAt: %v", err)
	}

	detail, err := s.GetProvenance(ctx, run.ProjectID, runID)
	if err != nil {
		t.Fatalf("GetProvenance after touch: %v", err)
	}

	if !detail.UpdatedAt.After(detail.CreatedAt) {
		t.Errorf("expected updated_at (%v) > created_at (%v) after touch",
			detail.UpdatedAt, detail.CreatedAt)
	}
}

// TestProvenanceTouchUpdatedAtMissing verifies that TouchProvenanceUpdatedAt returns
// nil (not an error) when there is no provenance record for the given run_id.
func TestProvenanceTouchUpdatedAtMissing(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if err := s.TouchProvenanceUpdatedAt(ctx, "nonexistent-run-id"); err != nil {
		t.Errorf("expected nil error for missing run_id, got: %v", err)
	}
}
