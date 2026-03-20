package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/bioflowy/flowy/internal/store"
)

// PushRegistryVersion creates or returns a version for the named entry (idempotent by digest).
// If the entry does not exist, it is created with the given kind.
// If the entry exists but kind differs, returns an error (kind is immutable after creation).
// Uses the writer connection throughout to avoid TOCTOU races.
// Returns the version, a boolean indicating if it was newly created, and an error.
func (s *SQLiteStore) PushRegistryVersion(ctx context.Context, projectID, name, kind, digest, content string) (*store.RegistryVersion, bool, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	// Step 1: SELECT existing entry by (project_id, name, is_active=1) — use writer to avoid TOCTOU.
	entryRow := s.writer.QueryRowContext(ctx,
		`SELECT id, project_id, name, kind, is_active, created_at, updated_at
		 FROM registry_entries WHERE project_id = ? AND name = ? AND is_active = 1`,
		projectID, name,
	)
	entry, err := scanRegistryEntry(entryRow)
	if err != nil && err != sql.ErrNoRows {
		return nil, false, fmt.Errorf("sqlite: PushRegistryVersion select entry: %w", err)
	}

	if entry == nil {
		// Entry does not exist — create it.
		entryID := uuid.New().String()
		_, err = s.writer.ExecContext(ctx,
			`INSERT INTO registry_entries (id, project_id, name, kind, is_active, created_at, updated_at)
			 VALUES (?, ?, ?, ?, 1, ?, ?)`,
			entryID, projectID, name, kind, now, now,
		)
		if err != nil {
			return nil, false, fmt.Errorf("sqlite: PushRegistryVersion insert entry: %w", err)
		}
		entry = &store.RegistryEntry{
			ID:        entryID,
			ProjectID: projectID,
			Name:      name,
			Kind:      kind,
			IsActive:  true,
		}
	} else if entry.Kind != kind {
		// Kind mismatch — kind is immutable after first push.
		return nil, false, fmt.Errorf("sqlite: entry %q is kind %q; cannot push kind %q", name, entry.Kind, kind)
	}

	// Step 2: SELECT existing version by (entry_id, digest) — use writer.
	versionRow := s.writer.QueryRowContext(ctx,
		`SELECT id, entry_id, digest, wdl_content, created_at
		 FROM registry_versions WHERE entry_id = ? AND digest = ?`,
		entry.ID, digest,
	)
	existing, err := scanRegistryVersion(versionRow)
	if err != nil && err != sql.ErrNoRows {
		return nil, false, fmt.Errorf("sqlite: PushRegistryVersion select version: %w", err)
	}
	if existing != nil {
		// Idempotent — version already exists for this digest.
		return existing, false, nil
	}

	// Step 3: INSERT new version.
	versionID := uuid.New().String()
	_, err = s.writer.ExecContext(ctx,
		`INSERT INTO registry_versions (id, entry_id, digest, wdl_content, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		versionID, entry.ID, digest, content, now,
	)
	if err != nil {
		return nil, false, fmt.Errorf("sqlite: PushRegistryVersion insert version: %w", err)
	}

	createdAt, _ := parseTime(now)
	return &store.RegistryVersion{
		ID:         versionID,
		EntryID:    entry.ID,
		Digest:     digest,
		WDLContent: content,
		CreatedAt:  createdAt,
	}, true, nil
}

// GetRegistryEntry returns the active entry with the given name in a project.
func (s *SQLiteStore) GetRegistryEntry(ctx context.Context, projectID, name string) (*store.RegistryEntry, error) {
	row := s.reader.QueryRowContext(ctx,
		`SELECT id, project_id, name, kind, is_active, created_at, updated_at
		 FROM registry_entries WHERE project_id = ? AND name = ? AND is_active = 1`,
		projectID, name,
	)
	entry, err := scanRegistryEntry(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("sqlite: registry entry not found: %q", name)
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: GetRegistryEntry %q: %w", name, err)
	}
	return entry, nil
}

// GetRegistryVersionByTag returns the version pointed to by the given tag name.
func (s *SQLiteStore) GetRegistryVersionByTag(ctx context.Context, projectID, name, tag string) (*store.RegistryVersion, *store.RegistryEntry, error) {
	row := s.reader.QueryRowContext(ctx,
		`SELECT rv.id, rv.entry_id, rv.digest, rv.wdl_content, rv.created_at,
		        re.id, re.project_id, re.name, re.kind, re.is_active, re.created_at, re.updated_at
		 FROM registry_tags rt
		 JOIN registry_versions rv ON rv.id = rt.version_id
		 JOIN registry_entries re ON re.id = rt.entry_id
		 WHERE re.project_id = ? AND re.name = ? AND rt.name = ? AND re.is_active = 1`,
		projectID, name, tag,
	)
	return scanVersionWithEntry(row)
}

// GetRegistryVersionByDigest returns the version with the given digest for the named entry.
func (s *SQLiteStore) GetRegistryVersionByDigest(ctx context.Context, projectID, name, digest string) (*store.RegistryVersion, *store.RegistryEntry, error) {
	row := s.reader.QueryRowContext(ctx,
		`SELECT rv.id, rv.entry_id, rv.digest, rv.wdl_content, rv.created_at,
		        re.id, re.project_id, re.name, re.kind, re.is_active, re.created_at, re.updated_at
		 FROM registry_versions rv
		 JOIN registry_entries re ON re.id = rv.entry_id
		 WHERE re.project_id = ? AND re.name = ? AND rv.digest = ? AND re.is_active = 1`,
		projectID, name, digest,
	)
	return scanVersionWithEntry(row)
}

// ListRegistryEntries returns summary rows for all active entries in a project.
// Note: uses an N+1 query for tags per entry.
// TODO: optimize with a single JOIN query if entry count grows large.
func (s *SQLiteStore) ListRegistryEntries(ctx context.Context, projectID string) ([]store.RegistryEntryListItem, error) {
	rows, err := s.reader.QueryContext(ctx,
		`SELECT re.name, re.kind, re.created_at,
		        (SELECT COUNT(*) FROM registry_versions rv WHERE rv.entry_id = re.id) as version_count
		 FROM registry_entries re
		 WHERE re.project_id = ? AND re.is_active = 1
		 ORDER BY re.created_at ASC`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: ListRegistryEntries: %w", err)
	}
	defer rows.Close()

	var items []store.RegistryEntryListItem
	for rows.Next() {
		var item store.RegistryEntryListItem
		var createdAtStr string
		if err := rows.Scan(&item.Name, &item.Kind, &createdAtStr, &item.VersionCount); err != nil {
			return nil, fmt.Errorf("sqlite: ListRegistryEntries scan: %w", err)
		}
		item.CreatedAt, _ = parseTime(createdAtStr)

		// Fetch tag names for this entry (N+1 — acceptable at small scale).
		tagRows, err := s.reader.QueryContext(ctx,
			`SELECT rt.name FROM registry_tags rt
			 JOIN registry_entries re ON re.id = rt.entry_id
			 WHERE re.project_id = ? AND re.name = ? AND re.is_active = 1
			 ORDER BY rt.name ASC`,
			projectID, item.Name,
		)
		if err != nil {
			return nil, fmt.Errorf("sqlite: ListRegistryEntries tags for %q: %w", item.Name, err)
		}
		var tags []string
		for tagRows.Next() {
			var t string
			if err := tagRows.Scan(&t); err != nil {
				tagRows.Close()
				return nil, fmt.Errorf("sqlite: ListRegistryEntries tag scan: %w", err)
			}
			tags = append(tags, t)
		}
		tagRows.Close()
		if err := tagRows.Err(); err != nil {
			return nil, fmt.Errorf("sqlite: ListRegistryEntries tag rows: %w", err)
		}
		item.Tags = tags
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: ListRegistryEntries rows: %w", err)
	}
	return items, nil
}

// ListRegistryVersions returns all versions for the named entry with associated tag names.
func (s *SQLiteStore) ListRegistryVersions(ctx context.Context, projectID, name string) ([]store.RegistryVersionListItem, error) {
	// First get the entry to confirm it exists and is active.
	entry, err := s.GetRegistryEntry(ctx, projectID, name)
	if err != nil {
		return nil, fmt.Errorf("sqlite: ListRegistryVersions: %w", err)
	}

	rows, err := s.reader.QueryContext(ctx,
		`SELECT id, digest, created_at FROM registry_versions WHERE entry_id = ? ORDER BY created_at ASC`,
		entry.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: ListRegistryVersions: %w", err)
	}
	defer rows.Close()

	var items []store.RegistryVersionListItem
	for rows.Next() {
		var item store.RegistryVersionListItem
		var createdAtStr string
		if err := rows.Scan(&item.ID, &item.Digest, &createdAtStr); err != nil {
			return nil, fmt.Errorf("sqlite: ListRegistryVersions scan: %w", err)
		}
		item.CreatedAt, _ = parseTime(createdAtStr)

		// Fetch tag names for this version.
		tagRows, err := s.reader.QueryContext(ctx,
			`SELECT name FROM registry_tags WHERE version_id = ? ORDER BY name ASC`,
			item.ID,
		)
		if err != nil {
			return nil, fmt.Errorf("sqlite: ListRegistryVersions tags for version %q: %w", item.ID, err)
		}
		var tags []string
		for tagRows.Next() {
			var t string
			if err := tagRows.Scan(&t); err != nil {
				tagRows.Close()
				return nil, fmt.Errorf("sqlite: ListRegistryVersions tag scan: %w", err)
			}
			tags = append(tags, t)
		}
		tagRows.Close()
		if err := tagRows.Err(); err != nil {
			return nil, fmt.Errorf("sqlite: ListRegistryVersions tag rows: %w", err)
		}
		item.Tags = tags
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: ListRegistryVersions rows: %w", err)
	}
	return items, nil
}

// UpsertRegistryTag creates or reassigns a tag to a version.
// Uses ON CONFLICT to update the version_id and updated_at when the tag already exists.
func (s *SQLiteStore) UpsertRegistryTag(ctx context.Context, entryID, tagName, versionID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.writer.ExecContext(ctx,
		`INSERT INTO registry_tags (id, entry_id, name, version_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT (entry_id, name) DO UPDATE SET version_id = ?, updated_at = ?`,
		uuid.New().String(), entryID, tagName, versionID, now, now, versionID, now,
	)
	if err != nil {
		return fmt.Errorf("sqlite: UpsertRegistryTag %q: %w", tagName, err)
	}
	return nil
}

// DeleteRegistryTag removes a tag (the version it pointed to is preserved).
// Returns an error if the tag does not exist.
func (s *SQLiteStore) DeleteRegistryTag(ctx context.Context, entryID, tagName string) error {
	result, err := s.writer.ExecContext(ctx,
		`DELETE FROM registry_tags WHERE entry_id = ? AND name = ?`,
		entryID, tagName,
	)
	if err != nil {
		return fmt.Errorf("sqlite: DeleteRegistryTag %q: %w", tagName, err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite: DeleteRegistryTag rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("sqlite: tag not found: %q", tagName)
	}
	return nil
}

// DeleteRegistryEntry soft-deletes the entry by setting is_active=0.
// Associated versions and tags are retained in the database.
func (s *SQLiteStore) DeleteRegistryEntry(ctx context.Context, projectID, name string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.writer.ExecContext(ctx,
		`UPDATE registry_entries SET is_active = 0, updated_at = ?
		 WHERE project_id = ? AND name = ? AND is_active = 1`,
		now, projectID, name,
	)
	if err != nil {
		return fmt.Errorf("sqlite: DeleteRegistryEntry %q: %w", name, err)
	}
	return nil
}

// entryRowScanner abstracts *sql.Row for scanRegistryEntry.
type entryRowScanner interface {
	Scan(dest ...any) error
}

// scanRegistryEntry reads a registry entry from a single row scanner.
func scanRegistryEntry(row entryRowScanner) (*store.RegistryEntry, error) {
	var isActive int
	var createdAtStr, updatedAtStr string
	var e store.RegistryEntry
	err := row.Scan(&e.ID, &e.ProjectID, &e.Name, &e.Kind, &isActive, &createdAtStr, &updatedAtStr)
	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}
	e.IsActive = isActive == 1
	e.CreatedAt, _ = parseTime(createdAtStr)
	e.UpdatedAt, _ = parseTime(updatedAtStr)
	return &e, nil
}

// versionRowScanner abstracts *sql.Row for scanRegistryVersion.
type versionRowScanner interface {
	Scan(dest ...any) error
}

// scanRegistryVersion reads a registry version from a single row scanner.
func scanRegistryVersion(row versionRowScanner) (*store.RegistryVersion, error) {
	var createdAtStr string
	var v store.RegistryVersion
	err := row.Scan(&v.ID, &v.EntryID, &v.Digest, &v.WDLContent, &createdAtStr)
	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}
	v.CreatedAt, _ = parseTime(createdAtStr)
	return &v, nil
}

// scanVersionWithEntry scans a joined row that contains both version and entry columns.
// Column order: rv.id, rv.entry_id, rv.digest, rv.wdl_content, rv.created_at,
//
//	re.id, re.project_id, re.name, re.kind, re.is_active, re.created_at, re.updated_at
func scanVersionWithEntry(row *sql.Row) (*store.RegistryVersion, *store.RegistryEntry, error) {
	var v store.RegistryVersion
	var e store.RegistryEntry
	var vCreatedAtStr, eCreatedAtStr, eUpdatedAtStr string
	var isActive int
	err := row.Scan(
		&v.ID, &v.EntryID, &v.Digest, &v.WDLContent, &vCreatedAtStr,
		&e.ID, &e.ProjectID, &e.Name, &e.Kind, &isActive, &eCreatedAtStr, &eUpdatedAtStr,
	)
	if err == sql.ErrNoRows {
		return nil, nil, fmt.Errorf("sqlite: registry version not found")
	}
	if err != nil {
		return nil, nil, fmt.Errorf("sqlite: scanVersionWithEntry: %w", err)
	}
	v.CreatedAt, _ = parseTime(vCreatedAtStr)
	e.IsActive = isActive == 1
	e.CreatedAt, _ = parseTime(eCreatedAtStr)
	e.UpdatedAt, _ = parseTime(eUpdatedAtStr)
	return &v, &e, nil
}
