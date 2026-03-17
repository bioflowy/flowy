// Package store defines the Store interface and shared model types for workflow
// run and task persistence. It is a pure interface + model package with no
// external dependencies — callers may swap in PostgreSQL or other backends.
package store

import (
	"context"
	"time"
)

// RunState represents the lifecycle state of a workflow run.
type RunState string

const (
	RunPending   RunState = "PENDING"
	RunRunning   RunState = "RUNNING"
	RunCompleted RunState = "COMPLETED"
	RunFailed    RunState = "FAILED"
	RunAborted   RunState = "ABORTED"
)

// TaskState represents the lifecycle state of a single workflow task.
type TaskState string

const (
	TaskPending   TaskState = "PENDING"
	TaskClaimed   TaskState = "CLAIMED"
	TaskRunning   TaskState = "RUNNING"
	TaskCompleted TaskState = "COMPLETED"
	TaskFailed    TaskState = "FAILED"
	TaskAborted   TaskState = "ABORTED"
)

// User represents an authenticated user of the flowy server.
// GlobalRole is one of "admin", "executor", or "viewer".
// IsActive is false for deactivated (soft-deleted) accounts.
type User struct {
	ID           string
	Username     string
	PasswordHash string
	DisplayName  string
	Email        string
	GlobalRole   string // "admin", "executor", or "viewer"
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Run holds metadata for a submitted workflow run.
type Run struct {
	ID          string
	WDLPath     string
	InputsJSON  string
	State       RunState
	OutputsJSON string    // JSON-encoded run outputs; empty until set by UpdateRunOutputs
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Task represents a single WDL task execution within a run.
// ID is auto-incremented by the database; ShardIndex distinguishes scatter shards.
// LastHeartbeatAt is nullable — it is set when a daemon claims the task and
// periodically updated while the task is running.
// InputsJSON and OutputsJSON hold JSON-encoded task inputs/outputs stored in
// the database for API-based delivery (set by UpdateTaskInputs/UpdateTaskOutputs).
type Task struct {
	ID              int64
	RunID           string
	TaskName        string
	State           TaskState
	ShardIndex      int
	DaemonID        string
	LastHeartbeatAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
	InputsJSON      string // JSON-encoded task inputs; empty until set by UpdateTaskInputs
	OutputsJSON     string // JSON-encoded task outputs; empty until set by UpdateTaskOutputs
}

// Store is the persistence abstraction for runs and tasks.
// Implementations must be safe for concurrent use.
type Store interface {
	// Run operations

	// CreateRun persists a new run record. r.ID must be set by the caller.
	CreateRun(ctx context.Context, r *Run) error
	// GetRun returns the run with the given exact ID, or an error if not found.
	GetRun(ctx context.Context, id string) (*Run, error)
	// GetRunByPrefix returns the run whose ID starts with prefix.
	// Returns an error if the prefix matches zero or more than one run.
	GetRunByPrefix(ctx context.Context, prefix string) (*Run, error)
	// UpdateRunState sets the state of a run and updates its updated_at timestamp.
	UpdateRunState(ctx context.Context, id string, state RunState) error
	// UpdateRunOutputs stores the JSON-encoded outputs for a completed run.
	UpdateRunOutputs(ctx context.Context, id string, outputsJSON string) error

	// Task operations

	// CreateTask persists a new task record. t.RunID, t.TaskName, and t.ShardIndex
	// must be set; the database assigns t.ID.
	CreateTask(ctx context.Context, t *Task) error
	// ListTasksByRun returns all tasks for the given run in insertion order.
	ListTasksByRun(ctx context.Context, runID string) ([]Task, error)
	// ClaimTask atomically picks a PENDING task, marks it CLAIMED, sets daemon_id
	// and last_heartbeat_at, and returns it. Returns nil, nil when no task is
	// available — callers should treat this as a signal to back off.
	ClaimTask(ctx context.Context, daemonID string) (*Task, error)
	// UpdateTaskState sets the state of a task and updates its updated_at timestamp.
	UpdateTaskState(ctx context.Context, taskID int64, state TaskState) error
	// GetTaskByID returns the task with the given ID, or an error if not found.
	GetTaskByID(ctx context.Context, taskID int64) (*Task, error)
	// UpdateHeartbeat refreshes last_heartbeat_at for a running task.
	UpdateHeartbeat(ctx context.Context, taskID int64) error
	// UpdateHeartbeatByDaemon refreshes last_heartbeat_at for ALL CLAIMED/RUNNING
	// tasks owned by daemonID in a single UPDATE. This is used by the daemon to
	// refresh all its active tasks in one call per heartbeat interval.
	UpdateHeartbeatByDaemon(ctx context.Context, daemonID string) error
	// FindStaleTasks returns tasks in CLAIMED or RUNNING state whose
	// last_heartbeat_at is older than timeoutSec seconds ago.
	FindStaleTasks(ctx context.Context, timeoutSec int) ([]Task, error)
	// UpdateTaskOutputs stores the JSON-encoded outputs for a task.
	// Called by handleResult after the daemon reports task completion.
	UpdateTaskOutputs(ctx context.Context, taskID int64, outputsJSON string) error
	// UpdateTaskInputs stores the JSON-encoded inputs for a task.
	// Called by makeDispatchFn and handleTaskRun after creating the task record.
	UpdateTaskInputs(ctx context.Context, taskID int64, inputsJSON string) error

	// User operations

	// CreateUser persists a new user record. u.ID must be set by the caller.
	CreateUser(ctx context.Context, u *User) error
	// GetUserByUsername returns the active user with the given username, or an error if not found.
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	// GetUserByID returns the user with the given ID regardless of active state, or an error if not found.
	GetUserByID(ctx context.Context, id string) (*User, error)
	// ListUsers returns all users ordered by created_at ascending.
	ListUsers(ctx context.Context) ([]User, error)
	// DeactivateUser sets is_active=false for the user with the given ID.
	DeactivateUser(ctx context.Context, id string) error
	// CountUsers returns the total number of user records (including inactive).
	CountUsers(ctx context.Context) (int, error)

	// Server config operations (key-value store for auto-generated secrets)

	// GetServerConfig returns the value for the given key, or an error wrapping
	// sql.ErrNoRows if the key does not exist.
	GetServerConfig(ctx context.Context, key string) (string, error)
	// SetServerConfig upserts a key-value pair in the server_config table.
	SetServerConfig(ctx context.Context, key, value string) error

	// Lifecycle

	// Close releases all resources held by the store.
	Close() error
}
