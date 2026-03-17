package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/bioflowy/flowy/internal/store"
)

// CreateRun inserts a new run record. r.ID must be set by the caller.
// r.ProjectID may be empty when auth is disabled.
func (s *SQLiteStore) CreateRun(ctx context.Context, r *store.Run) error {
	var projectID *string
	if r.ProjectID != "" {
		projectID = &r.ProjectID
	}
	_, err := s.writer.ExecContext(ctx,
		`INSERT INTO runs (id, wdl_path, inputs_json, state, project_id) VALUES (?, ?, ?, ?, ?)`,
		r.ID, r.WDLPath, r.InputsJSON, string(r.State), projectID,
	)
	if err != nil {
		return fmt.Errorf("sqlite: CreateRun: %w", err)
	}
	return nil
}

// GetRun returns the run with the given exact ID.
func (s *SQLiteStore) GetRun(ctx context.Context, id string) (*store.Run, error) {
	row := s.reader.QueryRowContext(ctx,
		`SELECT id, wdl_path, inputs_json, state, outputs_json, created_at, updated_at, project_id
		   FROM runs WHERE id = ?`, id)
	r, err := scanRun(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("sqlite: run not found: %q", id)
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: GetRun: %w", err)
	}
	return r, nil
}

// GetRunByPrefix returns the run whose ID starts with prefix.
// Returns an error if zero or more than one run matches.
func (s *SQLiteStore) GetRunByPrefix(ctx context.Context, prefix string) (*store.Run, error) {
	rows, err := s.reader.QueryContext(ctx,
		`SELECT id, wdl_path, inputs_json, state, outputs_json, created_at, updated_at, project_id
		   FROM runs WHERE id LIKE ?`, prefix+"%")
	if err != nil {
		return nil, fmt.Errorf("sqlite: GetRunByPrefix: %w", err)
	}
	defer rows.Close()

	var results []*store.Run
	for rows.Next() {
		r, err := scanRunRows(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlite: GetRunByPrefix scan: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: GetRunByPrefix rows: %w", err)
	}

	switch len(results) {
	case 0:
		return nil, fmt.Errorf("sqlite: run not found")
	case 1:
		return results[0], nil
	default:
		return nil, fmt.Errorf("sqlite: ambiguous run ID prefix: %d matches", len(results))
	}
}

// UpdateRunState sets the state of a run.
func (s *SQLiteStore) UpdateRunState(ctx context.Context, id string, state store.RunState) error {
	_, err := s.writer.ExecContext(ctx,
		`UPDATE runs SET state = ?, updated_at = datetime('now') WHERE id = ?`,
		string(state), id)
	if err != nil {
		return fmt.Errorf("sqlite: UpdateRunState: %w", err)
	}
	return nil
}

// UpdateRunOutputs stores the JSON-encoded outputs for a completed run.
func (s *SQLiteStore) UpdateRunOutputs(ctx context.Context, id string, outputsJSON string) error {
	_, err := s.writer.ExecContext(ctx,
		`UPDATE runs SET outputs_json = ?, updated_at = datetime('now') WHERE id = ?`,
		outputsJSON, id)
	if err != nil {
		return fmt.Errorf("sqlite: UpdateRunOutputs: %w", err)
	}
	return nil
}

// scanRun scans a single run from a *sql.Row.
// Expects columns: id, wdl_path, inputs_json, state, outputs_json, created_at, updated_at, project_id
func scanRun(row *sql.Row) (*store.Run, error) {
	var r store.Run
	var state string
	var outputsJSON sql.NullString
	var projectID sql.NullString
	err := row.Scan(&r.ID, &r.WDLPath, &r.InputsJSON, &state, &outputsJSON, &r.CreatedAt, &r.UpdatedAt, &projectID)
	if err != nil {
		return nil, err
	}
	r.State = store.RunState(state)
	if outputsJSON.Valid {
		r.OutputsJSON = outputsJSON.String
	}
	if projectID.Valid {
		r.ProjectID = projectID.String
	}
	return &r, nil
}

// scanRunRows scans a single run from *sql.Rows.
// Expects columns: id, wdl_path, inputs_json, state, outputs_json, created_at, updated_at, project_id
func scanRunRows(rows *sql.Rows) (*store.Run, error) {
	var r store.Run
	var state string
	var outputsJSON sql.NullString
	var projectID sql.NullString
	err := rows.Scan(&r.ID, &r.WDLPath, &r.InputsJSON, &state, &outputsJSON, &r.CreatedAt, &r.UpdatedAt, &projectID)
	if err != nil {
		return nil, err
	}
	r.State = store.RunState(state)
	if outputsJSON.Valid {
		r.OutputsJSON = outputsJSON.String
	}
	if projectID.Valid {
		r.ProjectID = projectID.String
	}
	return &r, nil
}
