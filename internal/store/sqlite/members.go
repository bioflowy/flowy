package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/bioflowy/flowy/internal/store"
)

// AddMember adds a user to a project with the given role.
// Returns an error if the user is already a member of the project.
func (s *SQLiteStore) AddMember(ctx context.Context, m *store.ProjectMember) error {
	now := time.Now().UTC()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	if m.UpdatedAt.IsZero() {
		m.UpdatedAt = now
	}
	_, err := s.writer.ExecContext(ctx,
		`INSERT INTO project_members (project_id, user_id, role, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		m.ProjectID, m.UserID, m.Role,
		m.CreatedAt.Format(time.RFC3339), m.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("sqlite: AddMember project=%q user=%q: %w", m.ProjectID, m.UserID, err)
	}
	return nil
}

// GetProjectMember returns the membership record for the given project and user.
// Returns an error if the membership does not exist.
func (s *SQLiteStore) GetProjectMember(ctx context.Context, projectID, userID string) (*store.ProjectMember, error) {
	row := s.reader.QueryRowContext(ctx,
		`SELECT project_id, user_id, role, created_at, updated_at
		 FROM project_members WHERE project_id = ? AND user_id = ?`,
		projectID, userID,
	)
	m, err := scanMember(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("sqlite: member not found: project=%q user=%q", projectID, userID)
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: GetProjectMember: %w", err)
	}
	return m, nil
}

// UpdateMemberRole changes the role of an existing project member.
// updated_at is always refreshed on a successful update.
func (s *SQLiteStore) UpdateMemberRole(ctx context.Context, projectID, userID, role string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.writer.ExecContext(ctx,
		`UPDATE project_members SET role = ?, updated_at = ?
		 WHERE project_id = ? AND user_id = ?`,
		role, now, projectID, userID,
	)
	if err != nil {
		return fmt.Errorf("sqlite: UpdateMemberRole project=%q user=%q: %w", projectID, userID, err)
	}
	return nil
}

// RemoveMember removes a user from a project unconditionally.
// Callers that need last-admin protection should use RemoveMemberIfNotLastAdmin.
func (s *SQLiteStore) RemoveMember(ctx context.Context, projectID, userID string) error {
	_, err := s.writer.ExecContext(ctx,
		`DELETE FROM project_members WHERE project_id = ? AND user_id = ?`,
		projectID, userID,
	)
	if err != nil {
		return fmt.Errorf("sqlite: RemoveMember project=%q user=%q: %w", projectID, userID, err)
	}
	return nil
}

// ListMembers returns all members of a project ordered by created_at ascending.
func (s *SQLiteStore) ListMembers(ctx context.Context, projectID string) ([]store.ProjectMember, error) {
	rows, err := s.reader.QueryContext(ctx,
		`SELECT project_id, user_id, role, created_at, updated_at
		 FROM project_members WHERE project_id = ? ORDER BY created_at ASC`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: ListMembers: %w", err)
	}
	defer rows.Close()

	var members []store.ProjectMember
	for rows.Next() {
		var createdAtStr, updatedAtStr string
		var m store.ProjectMember
		if err := rows.Scan(
			&m.ProjectID, &m.UserID, &m.Role, &createdAtStr, &updatedAtStr,
		); err != nil {
			return nil, fmt.Errorf("sqlite: ListMembers scan: %w", err)
		}
		m.CreatedAt, _ = parseTime(createdAtStr)
		m.UpdatedAt, _ = parseTime(updatedAtStr)
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: ListMembers rows: %w", err)
	}
	return members, nil
}

// CountProjectAdmins returns the number of members with role "admin" in a project.
func (s *SQLiteStore) CountProjectAdmins(ctx context.Context, projectID string) (int, error) {
	var count int
	err := s.reader.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM project_members WHERE project_id = ? AND role = 'admin'`,
		projectID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("sqlite: CountProjectAdmins: %w", err)
	}
	return count, nil
}

// RemoveMemberIfNotLastAdmin removes a member only if doing so would not leave
// the project with zero admins. Returns an error if the target is the last admin.
// This method uses the writer connection for both the count check and the delete
// to leverage the single-connection serialization (MaxOpenConns=1) for safety.
func (s *SQLiteStore) RemoveMemberIfNotLastAdmin(ctx context.Context, projectID, userID string) error {
	// Determine if the target member is an admin.
	var role string
	err := s.writer.QueryRowContext(ctx,
		`SELECT role FROM project_members WHERE project_id = ? AND user_id = ?`,
		projectID, userID,
	).Scan(&role)
	if err == sql.ErrNoRows {
		return fmt.Errorf("sqlite: member not found: project=%q user=%q", projectID, userID)
	}
	if err != nil {
		return fmt.Errorf("sqlite: RemoveMemberIfNotLastAdmin query role: %w", err)
	}

	if role == "admin" {
		// Count total admins in the project using the same writer connection.
		var count int
		if err := s.writer.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM project_members WHERE project_id = ? AND role = 'admin'`,
			projectID,
		).Scan(&count); err != nil {
			return fmt.Errorf("sqlite: RemoveMemberIfNotLastAdmin count admins: %w", err)
		}
		if count <= 1 {
			return fmt.Errorf("sqlite: cannot remove last admin from project %q", projectID)
		}
	}

	// Safe to delete.
	_, err = s.writer.ExecContext(ctx,
		`DELETE FROM project_members WHERE project_id = ? AND user_id = ?`,
		projectID, userID,
	)
	if err != nil {
		return fmt.Errorf("sqlite: RemoveMemberIfNotLastAdmin delete: %w", err)
	}
	return nil
}

// memberRowScanner abstracts *sql.Row for scanMember.
type memberRowScanner interface {
	Scan(dest ...any) error
}

// scanMember reads a project member from a single row scanner.
func scanMember(row memberRowScanner) (*store.ProjectMember, error) {
	var createdAtStr, updatedAtStr string
	var m store.ProjectMember
	if err := row.Scan(
		&m.ProjectID, &m.UserID, &m.Role, &createdAtStr, &updatedAtStr,
	); err != nil {
		return nil, err
	}
	m.CreatedAt, _ = parseTime(createdAtStr)
	m.UpdatedAt, _ = parseTime(updatedAtStr)
	return &m, nil
}
