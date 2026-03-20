package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/bioflowy/flowy/internal/store"
)

// CreateProvenance inserts a new provenance record for a registry-based run.
// Returns an error (wrapping the UNIQUE constraint violation) if the run_id already
// has a provenance record.
func (s *SQLiteStore) CreateProvenance(ctx context.Context, p *store.Provenance) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.writer.ExecContext(ctx,
		`INSERT INTO provenance
		    (id, run_id, user_id, registry_entry_id, registry_version_id,
		     registry_ref, registry_digest, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.RunID, p.UserID, p.RegistryEntryID, p.RegistryVersionID,
		p.RegistryRef, p.RegistryDigest, now, now,
	)
	if err != nil {
		return fmt.Errorf("sqlite: CreateProvenance: %w", err)
	}
	return nil
}

// TouchProvenanceUpdatedAt updates the updated_at timestamp for the provenance record
// associated with the given run_id. Returns nil if no record exists (non-error case).
func (s *SQLiteStore) TouchProvenanceUpdatedAt(ctx context.Context, runID string) error {
	_, err := s.writer.ExecContext(ctx,
		`UPDATE provenance SET updated_at = datetime('now') WHERE run_id = ?`,
		runID,
	)
	if err != nil {
		return fmt.Errorf("sqlite: TouchProvenanceUpdatedAt: %w", err)
	}
	// No error for zero rows affected — missing record is not an error.
	return nil
}

// GetProvenance returns the full provenance detail for the given run_id within a project.
// Returns an error if no record exists.
func (s *SQLiteStore) GetProvenance(ctx context.Context, projectID, runID string) (*store.ProvenanceDetail, error) {
	row := s.reader.QueryRowContext(ctx,
		`SELECT p.run_id, p.registry_ref, p.registry_digest,
		        u.username, r.state, r.inputs_json, r.outputs_json,
		        re.name, re.kind,
		        p.created_at, p.updated_at
		 FROM provenance p
		 JOIN runs r ON r.id = p.run_id
		 JOIN users u ON u.id = p.user_id
		 JOIN registry_entries re ON re.id = p.registry_entry_id
		 WHERE re.project_id = ? AND p.run_id = ?`,
		projectID, runID,
	)

	var d store.ProvenanceDetail
	var outputsJSON sql.NullString
	var createdAtStr, updatedAtStr string
	err := row.Scan(
		&d.RunID, &d.RegistryRef, &d.RegistryDigest,
		&d.Username, &d.RunState, &d.InputsJSON, &outputsJSON,
		&d.RegistryEntryName, &d.RegistryEntryKind,
		&createdAtStr, &updatedAtStr,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("sqlite: provenance not found for run %q", runID)
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: GetProvenance: %w", err)
	}
	if outputsJSON.Valid {
		d.OutputsJSON = outputsJSON.String
	}
	d.CreatedAt, _ = parseTime(createdAtStr)
	d.UpdatedAt, _ = parseTime(updatedAtStr)
	return &d, nil
}

// ListProvenance returns summary rows for all provenance records in a project.
// entryFilter limits results to a specific registry entry name (empty = all entries).
// userFilter limits results to a specific username (empty = all users).
// Results are ordered by created_at DESC.
func (s *SQLiteStore) ListProvenance(ctx context.Context, projectID, entryFilter, userFilter string) ([]store.ProvenanceSummary, error) {
	query := `SELECT p.run_id, p.registry_ref, p.registry_digest,
	                 u.username, r.state, p.created_at
	          FROM provenance p
	          JOIN runs r ON r.id = p.run_id
	          JOIN users u ON u.id = p.user_id
	          JOIN registry_entries re ON re.id = p.registry_entry_id
	          WHERE re.project_id = ?`
	args := []interface{}{projectID}

	if entryFilter != "" {
		query += " AND re.name = ?"
		args = append(args, entryFilter)
	}
	if userFilter != "" {
		query += " AND u.username = ?"
		args = append(args, userFilter)
	}
	query += " ORDER BY p.created_at DESC"

	rows, err := s.reader.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: ListProvenance: %w", err)
	}
	defer rows.Close()

	var summaries []store.ProvenanceSummary
	for rows.Next() {
		var s store.ProvenanceSummary
		var createdAtStr string
		if err := rows.Scan(
			&s.RunID, &s.RegistryRef, &s.RegistryDigest,
			&s.Username, &s.RunState, &createdAtStr,
		); err != nil {
			return nil, fmt.Errorf("sqlite: ListProvenance scan: %w", err)
		}
		s.CreatedAt, _ = parseTime(createdAtStr)
		summaries = append(summaries, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: ListProvenance rows: %w", err)
	}
	return summaries, nil
}
