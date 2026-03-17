package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/bioflowy/flowy/internal/store"
)

// CreateUser inserts a new user record into the database.
// u.ID must be set by the caller (typically a UUID).
// Returns an error if the username is already taken.
func (s *SQLiteStore) CreateUser(ctx context.Context, u *store.User) error {
	now := time.Now().UTC()
	if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}
	if u.UpdatedAt.IsZero() {
		u.UpdatedAt = now
	}
	isActive := 0
	if u.IsActive {
		isActive = 1
	}
	_, err := s.writer.ExecContext(ctx,
		`INSERT INTO users (id, username, password_hash, display_name, email, global_role, is_active, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Username, u.PasswordHash, u.DisplayName, u.Email,
		u.GlobalRole, isActive, u.CreatedAt.Format(time.RFC3339), u.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("sqlite: CreateUser %q: %w", u.Username, err)
	}
	return nil
}

// GetUserByUsername returns the active user with the given username.
// Returns an error if the user does not exist or is inactive.
func (s *SQLiteStore) GetUserByUsername(ctx context.Context, username string) (*store.User, error) {
	row := s.reader.QueryRowContext(ctx,
		`SELECT id, username, password_hash, display_name, email, global_role, is_active, created_at, updated_at
		 FROM users WHERE username = ? AND is_active = 1`,
		username,
	)
	u, err := scanUser(row)
	if err != nil {
		return nil, fmt.Errorf("sqlite: GetUserByUsername %q: %w", username, err)
	}
	return u, nil
}

// GetUserByID returns the user with the given ID regardless of active state.
// Returns an error if the user does not exist.
func (s *SQLiteStore) GetUserByID(ctx context.Context, id string) (*store.User, error) {
	row := s.reader.QueryRowContext(ctx,
		`SELECT id, username, password_hash, display_name, email, global_role, is_active, created_at, updated_at
		 FROM users WHERE id = ?`,
		id,
	)
	u, err := scanUser(row)
	if err != nil {
		return nil, fmt.Errorf("sqlite: GetUserByID %q: %w", id, err)
	}
	return u, nil
}

// ListUsers returns all users ordered by created_at ascending.
func (s *SQLiteStore) ListUsers(ctx context.Context) ([]store.User, error) {
	rows, err := s.reader.QueryContext(ctx,
		`SELECT id, username, password_hash, display_name, email, global_role, is_active, created_at, updated_at
		 FROM users ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: ListUsers: %w", err)
	}
	defer rows.Close()

	var users []store.User
	for rows.Next() {
		var isActive int
		var createdAtStr, updatedAtStr string
		var u store.User
		if err := rows.Scan(
			&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName,
			&u.Email, &u.GlobalRole, &isActive, &createdAtStr, &updatedAtStr,
		); err != nil {
			return nil, fmt.Errorf("sqlite: ListUsers scan: %w", err)
		}
		u.IsActive = isActive == 1
		u.CreatedAt, _ = parseTime(createdAtStr)
		u.UpdatedAt, _ = parseTime(updatedAtStr)
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: ListUsers rows: %w", err)
	}
	return users, nil
}

// DeactivateUser sets is_active=false for the user with the given ID.
func (s *SQLiteStore) DeactivateUser(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.writer.ExecContext(ctx,
		`UPDATE users SET is_active = 0, updated_at = ? WHERE id = ?`,
		now, id,
	)
	if err != nil {
		return fmt.Errorf("sqlite: DeactivateUser %q: %w", id, err)
	}
	return nil
}

// CountUsers returns the total number of user records (including inactive).
func (s *SQLiteStore) CountUsers(ctx context.Context) (int, error) {
	var count int
	err := s.reader.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("sqlite: CountUsers: %w", err)
	}
	return count, nil
}

// rowScanner abstracts *sql.Row so scanUser can be unit-tested without a real DB.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanUser reads a user row from a *sql.Row.
func scanUser(row rowScanner) (*store.User, error) {
	var isActive int
	var createdAtStr, updatedAtStr string
	var u store.User
	if err := row.Scan(
		&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName,
		&u.Email, &u.GlobalRole, &isActive, &createdAtStr, &updatedAtStr,
	); err != nil {
		return nil, err
	}
	u.IsActive = isActive == 1
	u.CreatedAt, _ = parseTime(createdAtStr)
	u.UpdatedAt, _ = parseTime(updatedAtStr)
	return &u, nil
}

// parseTime parses an RFC3339 or SQLite datetime string.
var timeFormats = []string{
	time.RFC3339,
	"2006-01-02T15:04:05Z",
	"2006-01-02 15:04:05",
}

func parseTime(s string) (time.Time, error) {
	for _, layout := range timeFormats {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("sqlite: cannot parse time %q", s)
}
