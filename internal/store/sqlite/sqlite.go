// Package sqlite provides a SQLite-backed implementation of store.Store.
// It uses two database/sql connection pools — a single-connection writer pool
// and an unbounded reader pool — to maximize read concurrency while serialising
// writes (required for SQLite WAL mode correctness).
//
// Migrations are embedded and applied automatically via goose on Open.
package sqlite

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
	"github.com/bioflowy/flowy/internal/store"

	// Register the modernc pure-Go SQLite driver under the "sqlite" name.
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// SQLiteStore implements store.Store using SQLite with WAL mode.
// Use Open or OpenMemory to construct instances.
type SQLiteStore struct {
	writer *sql.DB
	reader *sql.DB
}

// Compile-time assertion that SQLiteStore satisfies the Store interface.
var _ store.Store = (*SQLiteStore)(nil)

// Open creates a new SQLiteStore at the given DSN.
// The DSN should include WAL pragma, e.g.:
//
//	file:/path/to/flowy.db?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)
//
// Migrations are run automatically before returning.
func Open(dsn string) (*SQLiteStore, error) {
	return open(dsn)
}

// OpenMemory creates an in-memory SQLiteStore intended for tests.
// The cache=shared parameter is required for separate reader/writer connections
// to see the same in-memory database.
func OpenMemory() (*SQLiteStore, error) {
	dsn := "file::memory:?cache=shared&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)"
	return open(dsn)
}

func open(dsn string) (*SQLiteStore, error) {
	// Writer pool: single connection to serialise all writes.
	writer, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open writer: %w", err)
	}
	writer.SetMaxOpenConns(1)

	// Reader pool: no connection limit — allows concurrent reads.
	reader, err := sql.Open("sqlite", dsn)
	if err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("sqlite: open reader: %w", err)
	}

	// Run embedded migrations.
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		_ = writer.Close()
		_ = reader.Close()
		return nil, fmt.Errorf("sqlite: set dialect: %w", err)
	}
	if err := goose.Up(writer, "migrations"); err != nil {
		_ = writer.Close()
		_ = reader.Close()
		return nil, fmt.Errorf("sqlite: run migrations: %w", err)
	}

	return &SQLiteStore{writer: writer, reader: reader}, nil
}

// Close releases all database connections.
func (s *SQLiteStore) Close() error {
	werr := s.writer.Close()
	rerr := s.reader.Close()
	if werr != nil {
		return werr
	}
	return rerr
}

// WriterDB exposes the write connection pool.
// Intended for test helpers that need to backdate timestamps or inspect state
// without going through the Store interface.
func (s *SQLiteStore) WriterDB() *sql.DB {
	return s.writer
}
