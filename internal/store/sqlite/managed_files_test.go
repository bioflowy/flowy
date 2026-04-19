package sqlite_test

import (
	"context"
	"testing"

	"github.com/bioflowy/flowy/internal/store"
	"github.com/google/uuid"
)

// newManagedFile creates a ManagedFile with required fields set.
// runID may be nil for manually imported files.
func newManagedFile(projectID string, runID *string) *store.ManagedFile {
	return &store.ManagedFile{
		ID:         uuid.New().String(),
		ProjectID:  projectID,
		RunID:      runID,
		Path:       "/data/sample.fastq",
		FileType:   "fastq",
		Label:      "sample.fastq",
		SourceType: "import",
		Size:       1024,
	}
}

// TestManagedFileSchema verifies that the managed_files table exists with the
// expected columns and indices after migration 00009 runs.
func TestManagedFileSchema(t *testing.T) {
	s := openTestStore(t)
	db := s.WriterDB()
	ctx := context.Background()

	// Check that managed_files table has the expected columns.
	rows, err := db.QueryContext(ctx, "SELECT name FROM pragma_table_info('managed_files')")
	if err != nil {
		t.Fatalf("pragma_table_info(managed_files): %v", err)
	}
	colSet := make(map[string]bool)
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			t.Fatal(err)
		}
		colSet[col] = true
	}
	_ = rows.Close()

	if len(colSet) == 0 {
		t.Fatal("managed_files table does not exist or has no columns")
	}
	for _, required := range []string{"id", "project_id", "run_id", "path", "file_type", "label", "source_type", "size", "created_at"} {
		if !colSet[required] {
			t.Errorf("managed_files table missing column %q", required)
		}
	}

	// Check that the expected indices exist.
	indexRows, err := db.QueryContext(ctx, "SELECT name FROM pragma_index_list('managed_files')")
	if err != nil {
		t.Fatalf("pragma_index_list(managed_files): %v", err)
	}
	indexSet := make(map[string]bool)
	for indexRows.Next() {
		var idx string
		if err := indexRows.Scan(&idx); err != nil {
			t.Fatal(err)
		}
		indexSet[idx] = true
	}
	_ = indexRows.Close()

	for _, required := range []string{"idx_managed_files_project", "idx_managed_files_run"} {
		if !indexSet[required] {
			t.Errorf("managed_files missing index %q", required)
		}
	}
}

// TestManagedFileCRUD_RoundTrip verifies CreateManagedFile + GetManagedFileByID
// round-trip with nil RunID.
func TestManagedFileCRUD_RoundTrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	// Create a project first (FK constraint).
	p := newProject("crud-project")
	if err := s.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	f := newManagedFile(p.ID, nil)
	if err := s.CreateManagedFile(ctx, f); err != nil {
		t.Fatalf("CreateManagedFile: %v", err)
	}

	got, err := s.GetManagedFileByID(ctx, f.ID)
	if err != nil {
		t.Fatalf("GetManagedFileByID: %v", err)
	}

	if got.ID != f.ID {
		t.Errorf("ID: got %q, want %q", got.ID, f.ID)
	}
	if got.ProjectID != f.ProjectID {
		t.Errorf("ProjectID: got %q, want %q", got.ProjectID, f.ProjectID)
	}
	if got.RunID != nil {
		t.Errorf("RunID: got %v, want nil", got.RunID)
	}
	if got.Path != f.Path {
		t.Errorf("Path: got %q, want %q", got.Path, f.Path)
	}
	if got.FileType != f.FileType {
		t.Errorf("FileType: got %q, want %q", got.FileType, f.FileType)
	}
	if got.Label != f.Label {
		t.Errorf("Label: got %q, want %q", got.Label, f.Label)
	}
	if got.SourceType != f.SourceType {
		t.Errorf("SourceType: got %q, want %q", got.SourceType, f.SourceType)
	}
	if got.Size != f.Size {
		t.Errorf("Size: got %d, want %d", got.Size, f.Size)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

// TestManagedFileCRUD_WithRunID verifies that a non-nil RunID is preserved on round-trip.
func TestManagedFileCRUD_WithRunID(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	p := newProject("runid-project")
	if err := s.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	r := newRun("run-with-files-01")
	r.ProjectID = p.ID
	if err := s.CreateRun(ctx, r); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	runID := r.ID
	f := newManagedFile(p.ID, &runID)
	if err := s.CreateManagedFile(ctx, f); err != nil {
		t.Fatalf("CreateManagedFile: %v", err)
	}

	got, err := s.GetManagedFileByID(ctx, f.ID)
	if err != nil {
		t.Fatalf("GetManagedFileByID: %v", err)
	}
	if got.RunID == nil {
		t.Fatal("RunID: got nil, want non-nil")
	}
	if *got.RunID != runID {
		t.Errorf("RunID: got %q, want %q", *got.RunID, runID)
	}
}

// TestManagedFileList verifies ListManagedFiles returns all files for a project
// and empty slice for a project with no files.
func TestManagedFileList(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	p := newProject("list-project")
	if err := s.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	// Empty project — should return empty (non-nil) slice.
	files, err := s.ListManagedFiles(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListManagedFiles (empty): %v", err)
	}
	if files == nil {
		t.Error("ListManagedFiles should return non-nil empty slice, got nil")
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}

	// Insert two files.
	f1 := newManagedFile(p.ID, nil)
	f1.Path = "/data/file1.bam"
	f2 := newManagedFile(p.ID, nil)
	f2.Path = "/data/file2.bam"
	for _, f := range []*store.ManagedFile{f1, f2} {
		if err := s.CreateManagedFile(ctx, f); err != nil {
			t.Fatalf("CreateManagedFile(%s): %v", f.Path, err)
		}
	}

	files, err = s.ListManagedFiles(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListManagedFiles: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}
}

// TestManagedFileDelete verifies DeleteManagedFile removes the record.
func TestManagedFileDelete(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	p := newProject("delete-project")
	if err := s.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	f := newManagedFile(p.ID, nil)
	if err := s.CreateManagedFile(ctx, f); err != nil {
		t.Fatalf("CreateManagedFile: %v", err)
	}

	// Verify it exists.
	if _, err := s.GetManagedFileByID(ctx, f.ID); err != nil {
		t.Fatalf("GetManagedFileByID before delete: %v", err)
	}

	// Delete it.
	if err := s.DeleteManagedFile(ctx, f.ID); err != nil {
		t.Fatalf("DeleteManagedFile: %v", err)
	}

	// Verify it's gone.
	_, err := s.GetManagedFileByID(ctx, f.ID)
	if err == nil {
		t.Error("expected error after DeleteManagedFile, got nil")
	}
}

// TestManagedFileFK verifies that CreateManagedFile returns an error for an
// invalid project_id (FK constraint).
func TestManagedFileFK(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	f := newManagedFile("non-existent-project-id", nil)
	err := s.CreateManagedFile(ctx, f)
	if err == nil {
		t.Error("expected FK constraint error for non-existent project_id, got nil")
	}
}

// TestCompleteRun_EmptyFiles verifies that CompleteRun with an empty files slice
// marks the run as COMPLETED with outputs_json set and inserts no managed files.
func TestCompleteRun_EmptyFiles(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	p := newProject("complete-empty-project")
	if err := s.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	r := newRun("run-complete-empty-01")
	r.ProjectID = p.ID
	if err := s.CreateRun(ctx, r); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	outputsJSON := `{"out": "value"}`
	if err := s.CompleteRun(ctx, r.ID, p.ID, outputsJSON, nil); err != nil {
		t.Fatalf("CompleteRun (nil files): %v", err)
	}

	got, err := s.GetRun(ctx, r.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.State != store.RunCompleted {
		t.Errorf("State: got %q, want COMPLETED", got.State)
	}
	if got.OutputsJSON != outputsJSON {
		t.Errorf("OutputsJSON: got %q, want %q", got.OutputsJSON, outputsJSON)
	}

	// No managed files should be inserted.
	files, err := s.ListManagedFiles(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListManagedFiles: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 managed files, got %d", len(files))
	}
}

// TestCompleteRun_WithFiles verifies that CompleteRun with files marks the run
// as COMPLETED AND inserts managed files.
func TestCompleteRun_WithFiles(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	p := newProject("complete-files-project")
	if err := s.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	r := newRun("run-complete-files-01")
	r.ProjectID = p.ID
	if err := s.CreateRun(ctx, r); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	runID := r.ID
	files := []store.ManagedFile{
		{
			ID:         uuid.New().String(),
			ProjectID:  p.ID,
			RunID:      &runID,
			Path:       "/data/output1.bam",
			FileType:   "bam",
			Label:      "output1.bam",
			SourceType: "workflow",
			Size:       2048,
		},
		{
			ID:         uuid.New().String(),
			ProjectID:  p.ID,
			RunID:      &runID,
			Path:       "/data/output2.bam",
			FileType:   "bam",
			Label:      "output2.bam",
			SourceType: "workflow",
			Size:       4096,
		},
	}

	outputsJSON := `{"bam": "/data/output1.bam"}`
	if err := s.CompleteRun(ctx, r.ID, p.ID, outputsJSON, files); err != nil {
		t.Fatalf("CompleteRun (with files): %v", err)
	}

	// Verify run state.
	got, err := s.GetRun(ctx, r.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.State != store.RunCompleted {
		t.Errorf("State: got %q, want COMPLETED", got.State)
	}
	if got.OutputsJSON != outputsJSON {
		t.Errorf("OutputsJSON: got %q, want %q", got.OutputsJSON, outputsJSON)
	}

	// Verify managed files were inserted.
	listed, err := s.ListManagedFiles(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListManagedFiles: %v", err)
	}
	if len(listed) != 2 {
		t.Errorf("expected 2 managed files, got %d", len(listed))
	}
}

// TestCompleteRun_Atomicity verifies that CompleteRun is atomic: if a managed
// file insert fails (duplicate PK), the run state must NOT be COMPLETED.
func TestCompleteRun_Atomicity(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	p := newProject("atomicity-project")
	if err := s.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	r := newRun("run-atomicity-01")
	r.ProjectID = p.ID
	if err := s.CreateRun(ctx, r); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	runID := r.ID
	dupID := uuid.New().String()
	// Two files with the same ID — the second insert will violate the PK constraint.
	files := []store.ManagedFile{
		{
			ID:         dupID,
			ProjectID:  p.ID,
			RunID:      &runID,
			Path:       "/data/output1.bam",
			FileType:   "bam",
			Label:      "output1.bam",
			SourceType: "workflow",
			Size:       2048,
		},
		{
			ID:         dupID, // duplicate PK — will cause insert to fail
			ProjectID:  p.ID,
			RunID:      &runID,
			Path:       "/data/output2.bam",
			FileType:   "bam",
			Label:      "output2.bam",
			SourceType: "workflow",
			Size:       4096,
		},
	}

	err := s.CompleteRun(ctx, r.ID, p.ID, `{}`, files)
	if err == nil {
		t.Fatal("expected error from CompleteRun with duplicate file IDs, got nil")
	}

	// The run state must still be PENDING (transaction was rolled back).
	got, err := s.GetRun(ctx, r.ID)
	if err != nil {
		t.Fatalf("GetRun after failed CompleteRun: %v", err)
	}
	if got.State != store.RunPending {
		t.Errorf("State after rollback: got %q, want PENDING (transaction must be rolled back)", got.State)
	}
}
