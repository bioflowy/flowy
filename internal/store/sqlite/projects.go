package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/bioflowy/flowy/internal/store"
)

// CreateProject inserts a new project record into the database.
// p.ID must be set by the caller (typically a UUID).
// Returns an error if the slug is already taken.
func (s *SQLiteStore) CreateProject(ctx context.Context, p *store.Project) error {
	now := time.Now().UTC()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = now
	}
	isActive := 0
	if p.IsActive {
		isActive = 1
	}
	_, err := s.writer.ExecContext(ctx,
		`INSERT INTO projects (id, slug, description, is_active, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		p.ID, p.Slug, p.Description, isActive,
		p.CreatedAt.Format(time.RFC3339), p.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("sqlite: CreateProject %q: %w", p.Slug, err)
	}
	return nil
}

// GetProjectBySlug returns the active project with the given slug.
// Returns an error if the project does not exist or is inactive.
func (s *SQLiteStore) GetProjectBySlug(ctx context.Context, slug string) (*store.Project, error) {
	row := s.reader.QueryRowContext(ctx,
		`SELECT id, slug, description, is_active, created_at, updated_at
		 FROM projects WHERE slug = ? AND is_active = 1`,
		slug,
	)
	p, err := scanProject(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("sqlite: project not found: %q", slug)
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: GetProjectBySlug %q: %w", slug, err)
	}
	return p, nil
}

// GetProjectByID returns the active project with the given ID.
// Returns an error if the project does not exist or is inactive.
func (s *SQLiteStore) GetProjectByID(ctx context.Context, id string) (*store.Project, error) {
	row := s.reader.QueryRowContext(ctx,
		`SELECT id, slug, description, is_active, created_at, updated_at
		 FROM projects WHERE id = ? AND is_active = 1`,
		id,
	)
	p, err := scanProject(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("sqlite: project not found: %q", id)
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: GetProjectByID %q: %w", id, err)
	}
	return p, nil
}

// ListProjectsForUser returns all active projects where the given user is a member.
// Results are ordered by created_at ascending.
func (s *SQLiteStore) ListProjectsForUser(ctx context.Context, userID string) ([]store.Project, error) {
	rows, err := s.reader.QueryContext(ctx,
		`SELECT p.id, p.slug, p.description, p.is_active, p.created_at, p.updated_at
		 FROM projects p
		 INNER JOIN project_members pm ON pm.project_id = p.id
		 WHERE pm.user_id = ? AND p.is_active = 1
		 ORDER BY p.created_at ASC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: ListProjectsForUser: %w", err)
	}
	defer rows.Close()

	return scanProjects(rows)
}

// ListAllProjects returns all active projects ordered by created_at ascending.
func (s *SQLiteStore) ListAllProjects(ctx context.Context) ([]store.Project, error) {
	rows, err := s.reader.QueryContext(ctx,
		`SELECT id, slug, description, is_active, created_at, updated_at
		 FROM projects WHERE is_active = 1 ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: ListAllProjects: %w", err)
	}
	defer rows.Close()

	return scanProjects(rows)
}

// UpdateProject persists description changes to an existing project record.
// updated_at is always refreshed on a successful update.
func (s *SQLiteStore) UpdateProject(ctx context.Context, p *store.Project) error {
	now := time.Now().UTC()
	p.UpdatedAt = now
	_, err := s.writer.ExecContext(ctx,
		`UPDATE projects SET description = ?, updated_at = ? WHERE id = ?`,
		p.Description, p.UpdatedAt.Format(time.RFC3339), p.ID,
	)
	if err != nil {
		return fmt.Errorf("sqlite: UpdateProject %q: %w", p.ID, err)
	}
	return nil
}

// DeactivateProject sets is_active=false for the project with the given ID.
func (s *SQLiteStore) DeactivateProject(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.writer.ExecContext(ctx,
		`UPDATE projects SET is_active = 0, updated_at = ? WHERE id = ?`,
		now, id,
	)
	if err != nil {
		return fmt.Errorf("sqlite: DeactivateProject %q: %w", id, err)
	}
	return nil
}

// projectRowScanner abstracts *sql.Row for scanProject.
type projectRowScanner interface {
	Scan(dest ...any) error
}

// scanProject reads a project from a single row scanner.
func scanProject(row projectRowScanner) (*store.Project, error) {
	var isActive int
	var createdAtStr, updatedAtStr string
	var p store.Project
	if err := row.Scan(
		&p.ID, &p.Slug, &p.Description, &isActive, &createdAtStr, &updatedAtStr,
	); err != nil {
		return nil, err
	}
	p.IsActive = isActive == 1
	p.CreatedAt, _ = parseTime(createdAtStr)
	p.UpdatedAt, _ = parseTime(updatedAtStr)
	return &p, nil
}

// scanProjects scans multiple project rows.
func scanProjects(rows *sql.Rows) ([]store.Project, error) {
	var projects []store.Project
	for rows.Next() {
		var isActive int
		var createdAtStr, updatedAtStr string
		var p store.Project
		if err := rows.Scan(
			&p.ID, &p.Slug, &p.Description, &isActive, &createdAtStr, &updatedAtStr,
		); err != nil {
			return nil, fmt.Errorf("sqlite: scan project: %w", err)
		}
		p.IsActive = isActive == 1
		p.CreatedAt, _ = parseTime(createdAtStr)
		p.UpdatedAt, _ = parseTime(updatedAtStr)
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: projects rows: %w", err)
	}
	return projects, nil
}
