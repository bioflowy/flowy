package sqlite

import (
	"context"
	"fmt"
)

// GetServerConfig returns the value for the given key from the server_config table.
// Returns an error wrapping sql.ErrNoRows if the key does not exist.
func (s *SQLiteStore) GetServerConfig(ctx context.Context, key string) (string, error) {
	var value string
	err := s.reader.QueryRowContext(ctx,
		`SELECT value FROM server_config WHERE key = ?`, key,
	).Scan(&value)
	if err != nil {
		return "", fmt.Errorf("sqlite: GetServerConfig %q: %w", key, err)
	}
	return value, nil
}

// SetServerConfig upserts a key-value pair in the server_config table.
// If the key already exists its value is overwritten.
func (s *SQLiteStore) SetServerConfig(ctx context.Context, key, value string) error {
	_, err := s.writer.ExecContext(ctx,
		`INSERT OR REPLACE INTO server_config (key, value) VALUES (?, ?)`, key, value,
	)
	if err != nil {
		return fmt.Errorf("sqlite: SetServerConfig %q: %w", key, err)
	}
	return nil
}
