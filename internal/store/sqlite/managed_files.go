package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/bioflowy/flowy/internal/store"
)

// CreateManagedFile inserts a new managed file record.
// f.ID must be set by the caller (UUID).
// All SQL parameters use ? placeholders to prevent SQL injection.
func (s *SQLiteStore) CreateManagedFile(ctx context.Context, f *store.ManagedFile) error {
	_, err := s.writer.ExecContext(ctx,
		`INSERT INTO managed_files (id, project_id, run_id, path, file_type, label, size)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.ProjectID, f.RunID, f.Path, f.FileType, f.Label, f.Size,
	)
	if err != nil {
		return fmt.Errorf("sqlite: CreateManagedFile: %w", err)
	}
	return nil
}

// ListManagedFiles returns all managed files for the given project, ordered by
// created_at DESC (newest first). Returns a non-nil empty slice when no records exist.
func (s *SQLiteStore) ListManagedFiles(ctx context.Context, projectID string) ([]store.ManagedFile, error) {
	rows, err := s.reader.QueryContext(ctx,
		`SELECT id, project_id, run_id, path, file_type, label, size, created_at
		 FROM managed_files
		 WHERE project_id = ?
		 ORDER BY created_at DESC`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: ListManagedFiles: %w", err)
	}
	defer rows.Close()

	files := []store.ManagedFile{}
	for rows.Next() {
		f, err := scanManagedFileRows(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlite: ListManagedFiles scan: %w", err)
		}
		files = append(files, *f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: ListManagedFiles rows: %w", err)
	}
	return files, nil
}

// GetManagedFileByID returns the managed file with the given ID.
// Returns a wrapped error if no record is found.
func (s *SQLiteStore) GetManagedFileByID(ctx context.Context, id string) (*store.ManagedFile, error) {
	row := s.reader.QueryRowContext(ctx,
		`SELECT id, project_id, run_id, path, file_type, label, size, created_at
		 FROM managed_files
		 WHERE id = ?`,
		id,
	)
	f, err := scanManagedFile(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("sqlite: managed file not found: %q", id)
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: GetManagedFileByID: %w", err)
	}
	return f, nil
}

// DeleteManagedFile removes the managed file record with the given ID.
func (s *SQLiteStore) DeleteManagedFile(ctx context.Context, id string) error {
	_, err := s.writer.ExecContext(ctx,
		`DELETE FROM managed_files WHERE id = ?`,
		id,
	)
	if err != nil {
		return fmt.Errorf("sqlite: DeleteManagedFile: %w", err)
	}
	return nil
}

// CompleteRun atomically marks a run as COMPLETED, sets outputs_json, and
// inserts any provided managed file records in a single SQLite transaction.
// Pass nil or empty slice for files when no file registration is needed.
// If the transaction fails for any reason (including FK or PK violations from
// file inserts), the entire operation is rolled back.
func (s *SQLiteStore) CompleteRun(ctx context.Context, runID string, projectID string, outputsJSON string, files []store.ManagedFile) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: CompleteRun begin tx: %w", err)
	}
	// defer Rollback is a no-op after a successful Commit.
	defer tx.Rollback() //nolint:errcheck

	// Update run state and outputs atomically.
	_, err = tx.ExecContext(ctx,
		`UPDATE runs SET state = 'COMPLETED', outputs_json = ?, updated_at = datetime('now') WHERE id = ?`,
		outputsJSON, runID,
	)
	if err != nil {
		return fmt.Errorf("sqlite: CompleteRun update run: %w", err)
	}

	// Insert managed file records (if any).
	for i := range files {
		f := &files[i]
		_, err = tx.ExecContext(ctx,
			`INSERT INTO managed_files (id, project_id, run_id, path, file_type, label, size)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			f.ID, f.ProjectID, f.RunID, f.Path, f.FileType, f.Label, f.Size,
		)
		if err != nil {
			return fmt.Errorf("sqlite: CompleteRun insert managed file %q: %w", f.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: CompleteRun commit: %w", err)
	}
	return nil
}

// scanManagedFile scans a single ManagedFile from a *sql.Row.
// Expects columns: id, project_id, run_id, path, file_type, label, size, created_at
func scanManagedFile(row *sql.Row) (*store.ManagedFile, error) {
	var f store.ManagedFile
	var runID sql.NullString
	var createdAtStr string
	err := row.Scan(
		&f.ID, &f.ProjectID, &runID,
		&f.Path, &f.FileType, &f.Label, &f.Size,
		&createdAtStr,
	)
	if err != nil {
		return nil, err
	}
	if runID.Valid {
		f.RunID = &runID.String
	}
	f.CreatedAt, _ = parseTime(createdAtStr)
	return &f, nil
}

// scanManagedFileRows scans a single ManagedFile from *sql.Rows.
// Expects columns: id, project_id, run_id, path, file_type, label, size, created_at
func scanManagedFileRows(rows *sql.Rows) (*store.ManagedFile, error) {
	var f store.ManagedFile
	var runID sql.NullString
	var createdAtStr string
	err := rows.Scan(
		&f.ID, &f.ProjectID, &runID,
		&f.Path, &f.FileType, &f.Label, &f.Size,
		&createdAtStr,
	)
	if err != nil {
		return nil, err
	}
	if runID.Valid {
		f.RunID = &runID.String
	}
	f.CreatedAt, _ = parseTime(createdAtStr)
	return &f, nil
}
