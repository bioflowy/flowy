package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/bioflowy/flowy/internal/store"
)

// CreateTask inserts a new task. t.RunID, t.TaskName, and t.ShardIndex must be
// set; the database assigns t.ID on return.
func (s *SQLiteStore) CreateTask(ctx context.Context, t *store.Task) error {
	res, err := s.writer.ExecContext(ctx,
		`INSERT INTO tasks (run_id, task_name, shard_index) VALUES (?, ?, ?)`,
		t.RunID, t.TaskName, t.ShardIndex,
	)
	if err != nil {
		return fmt.Errorf("sqlite: CreateTask: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("sqlite: CreateTask LastInsertId: %w", err)
	}
	t.ID = id
	return nil
}

// ListTasksByRun returns all tasks for a run in insertion order.
func (s *SQLiteStore) ListTasksByRun(ctx context.Context, runID string) ([]store.Task, error) {
	rows, err := s.reader.QueryContext(ctx,
		`SELECT id, run_id, task_name, state, shard_index, daemon_id,
		        last_heartbeat_at, created_at, updated_at
		   FROM tasks WHERE run_id = ? ORDER BY id`, runID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: ListTasksByRun: %w", err)
	}
	defer rows.Close()

	var tasks []store.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlite: ListTasksByRun scan: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// ClaimTask atomically picks the oldest PENDING task, marks it CLAIMED, sets
// daemon_id and last_heartbeat_at, and returns it.
// Returns nil, nil when no PENDING task is available.
//
// Implementation note: we use a single atomic UPDATE with RETURNING to return
// all columns in one round-trip, avoiding a second query while rows are open
// (which would deadlock under the single-connection writer pool).
func (s *SQLiteStore) ClaimTask(ctx context.Context, daemonID string) (*store.Task, error) {
	// Single atomic UPDATE with subquery — prevents TOCTOU races.
	// RETURNING returns all columns so we don't need a follow-up query.
	rows, err := s.writer.QueryContext(ctx,
		`UPDATE tasks
		    SET state             = 'CLAIMED',
		        daemon_id         = ?,
		        last_heartbeat_at = datetime('now'),
		        updated_at        = datetime('now')
		  WHERE id = (
		        SELECT id FROM tasks WHERE state = 'PENDING' ORDER BY id LIMIT 1
		  )
		  RETURNING id, run_id, task_name, state, shard_index, daemon_id,
		            last_heartbeat_at, created_at, updated_at`,
		daemonID,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: ClaimTask: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		// No PENDING task available.
		return nil, nil
	}

	t, err := scanTask(rows)
	if err != nil {
		return nil, fmt.Errorf("sqlite: ClaimTask scan: %w", err)
	}
	return &t, nil
}

// UpdateTaskState sets the state of a task.
func (s *SQLiteStore) UpdateTaskState(ctx context.Context, taskID int64, state store.TaskState) error {
	_, err := s.writer.ExecContext(ctx,
		`UPDATE tasks SET state = ?, updated_at = datetime('now') WHERE id = ?`,
		string(state), taskID)
	if err != nil {
		return fmt.Errorf("sqlite: UpdateTaskState: %w", err)
	}
	return nil
}

// UpdateHeartbeat refreshes last_heartbeat_at for a running/claimed task.
func (s *SQLiteStore) UpdateHeartbeat(ctx context.Context, taskID int64) error {
	_, err := s.writer.ExecContext(ctx,
		`UPDATE tasks
		    SET last_heartbeat_at = datetime('now'),
		        updated_at        = datetime('now')
		  WHERE id = ?`, taskID)
	if err != nil {
		return fmt.Errorf("sqlite: UpdateHeartbeat: %w", err)
	}
	return nil
}

// FindStaleTasks returns tasks in CLAIMED or RUNNING state whose
// last_heartbeat_at is older than timeoutSec seconds ago.
func (s *SQLiteStore) FindStaleTasks(ctx context.Context, timeoutSec int) ([]store.Task, error) {
	// SQLite datetime() requires a string modifier, not an integer.
	modifier := fmt.Sprintf("-%d seconds", timeoutSec)

	rows, err := s.reader.QueryContext(ctx,
		`SELECT id, run_id, task_name, state, shard_index, daemon_id,
		        last_heartbeat_at, created_at, updated_at
		   FROM tasks
		  WHERE state IN ('CLAIMED', 'RUNNING')
		    AND last_heartbeat_at < datetime('now', ?)
		  ORDER BY id`,
		modifier,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: FindStaleTasks: %w", err)
	}
	defer rows.Close()

	var tasks []store.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlite: FindStaleTasks scan: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// scanTask scans a single task row from *sql.Rows.
func scanTask(rows *sql.Rows) (store.Task, error) {
	var t store.Task
	var state string
	var daemonID sql.NullString
	var heartbeat sql.NullTime
	err := rows.Scan(
		&t.ID, &t.RunID, &t.TaskName, &state, &t.ShardIndex,
		&daemonID, &heartbeat, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return t, err
	}
	t.State = store.TaskState(state)
	if daemonID.Valid {
		t.DaemonID = daemonID.String
	}
	if heartbeat.Valid {
		hb := heartbeat.Time
		t.LastHeartbeatAt = &hb
	}
	return t, nil
}
