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

// Project represents a project workspace that groups workflow runs and controls
// membership access. Slug is a human-readable unique identifier for the project.
// IsActive is false for deactivated (soft-deleted) projects.
type Project struct {
	ID          string
	Slug        string
	Description string
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ProjectMember represents a user's membership in a project.
// Role is one of "admin", "executor", or "viewer".
type ProjectMember struct {
	ProjectID string
	UserID    string
	Role      string // "admin", "executor", or "viewer"
	CreatedAt time.Time
	UpdatedAt time.Time
}

// RegistryEntry represents a named WDL entry in a project registry.
// Kind is "tool" (WDL task) or "workflow" (WDL workflow), set on first push and immutable.
// IsActive is false for soft-deleted entries.
type RegistryEntry struct {
	ID        string
	ProjectID string
	Name      string
	Kind      string // "tool" or "workflow"
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// RegistryVersion represents an immutable content-addressed WDL blob.
// Digest is "sha256:<hex>" computed from WDLContent.
type RegistryVersion struct {
	ID         string
	EntryID    string
	Digest     string // "sha256:<hex>"
	WDLContent string
	CreatedAt  time.Time
}

// RegistryTag is a mutable pointer from a human-readable tag name to a specific version.
type RegistryTag struct {
	ID        string
	EntryID   string
	Name      string
	VersionID string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// RegistryEntryListItem is the summary row returned by ListRegistryEntries.
type RegistryEntryListItem struct {
	Name         string
	Kind         string
	VersionCount int
	Tags         []string // tag names only
	CreatedAt    time.Time
}

// RegistryVersionListItem is a version row with associated tag names.
type RegistryVersionListItem struct {
	ID        string
	Digest    string
	Tags      []string
	CreatedAt time.Time
}

// Run holds metadata for a submitted workflow run.
type Run struct {
	ID             string
	ProjectID      string   // Empty when auth is disabled; set to project UUID when auth is enabled
	WDLPath        string
	RegistryRef    string   // User-provided ref, e.g. "align-reads:prod"; empty for file-path runs
	RegistryDigest string   // Resolved digest at submission time, e.g. "sha256:abc..."; empty for file-path runs
	InputsJSON     string
	State          RunState
	OutputsJSON    string   // JSON-encoded run outputs; empty until set by UpdateRunOutputs
	CreatedAt      time.Time
	UpdatedAt      time.Time
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

// Provenance records the audit trail for a registry-based run.
// It links a run to the registry entry and version that was used, and to the
// user who submitted the run.
type Provenance struct {
	ID                string
	RunID             string
	UserID            string
	RegistryEntryID   string
	RegistryVersionID string
	RegistryRef       string
	RegistryDigest    string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// ProvenanceSummary is a lightweight row returned by ListProvenance.
// It joins provenance with runs, users, and registry_entries.
type ProvenanceSummary struct {
	RunID          string
	RegistryRef    string
	RegistryDigest string
	Username       string
	RunState       string
	CreatedAt      time.Time
}

// ProvenanceDetail is the full provenance record returned by GetProvenance.
// It joins provenance with runs, users, and registry_entries.
type ProvenanceDetail struct {
	RunID              string
	RegistryRef        string
	RegistryDigest     string
	Username           string
	RunState           string
	InputsJSON         string
	OutputsJSON        string
	RegistryEntryName  string
	RegistryEntryKind  string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// ManagedFile represents a file tracked in the project library.
// RunID is nil for manually imported files that have no originating run.
type ManagedFile struct {
	ID        string
	ProjectID string
	RunID     *string // nil for manually imported files
	Path      string
	FileType  string
	Label     string
	Size      int64
	CreatedAt time.Time
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
	// ListRunsByProject returns all runs for the given project, newest first.
	ListRunsByProject(ctx context.Context, projectID string) ([]Run, error)

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
	// GetUserByUsernameAny returns the user with the given username regardless of active state.
	// Used by login to distinguish inactive accounts from non-existent accounts.
	GetUserByUsernameAny(ctx context.Context, username string) (*User, error)
	// GetUserByID returns the user with the given ID regardless of active state, or an error if not found.
	GetUserByID(ctx context.Context, id string) (*User, error)
	// ListUsers returns all users ordered by created_at ascending.
	ListUsers(ctx context.Context) ([]User, error)
	// DeactivateUser sets is_active=false for the user with the given ID.
	DeactivateUser(ctx context.Context, id string) error
	// UpdateUser persists changes to an existing user record.
	// Used to update fields such as PasswordHash after a password change.
	UpdateUser(ctx context.Context, u *User) error
	// CountUsers returns the total number of user records (including inactive).
	CountUsers(ctx context.Context) (int, error)

	// Project operations

	// CreateProject persists a new project record. p.ID must be set by the caller.
	// Returns an error if the slug is already taken.
	CreateProject(ctx context.Context, p *Project) error
	// GetProjectBySlug returns the active project with the given slug, or an error if not found.
	GetProjectBySlug(ctx context.Context, slug string) (*Project, error)
	// GetProjectByID returns the active project with the given ID, or an error if not found.
	GetProjectByID(ctx context.Context, id string) (*Project, error)
	// ListProjectsForUser returns all active projects where the user is a member, ordered by created_at ascending.
	ListProjectsForUser(ctx context.Context, userID string) ([]Project, error)
	// ListAllProjects returns all active projects ordered by created_at ascending.
	ListAllProjects(ctx context.Context) ([]Project, error)
	// UpdateProject persists description changes to an existing project record.
	UpdateProject(ctx context.Context, p *Project) error
	// DeactivateProject sets is_active=false for the project with the given ID.
	DeactivateProject(ctx context.Context, id string) error

	// Membership operations

	// AddMember adds a user to a project with the given role.
	// Returns an error if the user is already a member.
	AddMember(ctx context.Context, m *ProjectMember) error
	// GetProjectMember returns the membership record for the given project and user.
	GetProjectMember(ctx context.Context, projectID, userID string) (*ProjectMember, error)
	// UpdateMemberRole changes the role of an existing project member.
	UpdateMemberRole(ctx context.Context, projectID, userID, role string) error
	// RemoveMember removes a user from a project unconditionally.
	// Callers should use RemoveMemberIfNotLastAdmin when admin protection is needed.
	RemoveMember(ctx context.Context, projectID, userID string) error
	// ListMembers returns all members of a project ordered by created_at ascending.
	ListMembers(ctx context.Context, projectID string) ([]ProjectMember, error)
	// CountProjectAdmins returns the number of members with role "admin" in a project.
	CountProjectAdmins(ctx context.Context, projectID string) (int, error)
	// RemoveMemberIfNotLastAdmin removes a member only if doing so would not leave
	// the project with zero admins. Returns an error if the target is the last admin.
	RemoveMemberIfNotLastAdmin(ctx context.Context, projectID, userID string) error

	// Registry operations

	// PushRegistryVersion creates or gets a version for the given entry (idempotent by digest).
	// If the entry does not exist, it is created with the given kind.
	// If the entry exists but kind differs, returns an error (kind is immutable after creation).
	// Returns the version, a boolean indicating if it was newly created, and an error.
	PushRegistryVersion(ctx context.Context, projectID, name, kind, digest, content string) (*RegistryVersion, bool, error)
	// GetRegistryVersionByTag returns the version pointed to by the tag.
	GetRegistryVersionByTag(ctx context.Context, projectID, name, tag string) (*RegistryVersion, *RegistryEntry, error)
	// GetRegistryVersionByDigest returns the version with the given digest.
	GetRegistryVersionByDigest(ctx context.Context, projectID, name, digest string) (*RegistryVersion, *RegistryEntry, error)
	// ListRegistryEntries returns summary rows for all active entries in a project.
	ListRegistryEntries(ctx context.Context, projectID string) ([]RegistryEntryListItem, error)
	// ListRegistryVersions returns all versions for the named entry with associated tag names.
	ListRegistryVersions(ctx context.Context, projectID, name string) ([]RegistryVersionListItem, error)
	// UpsertRegistryTag creates or reassigns a tag to a version.
	UpsertRegistryTag(ctx context.Context, entryID, tagName, versionID string) error
	// DeleteRegistryTag removes a tag (version remains).
	DeleteRegistryTag(ctx context.Context, entryID, tagName string) error
	// DeleteRegistryEntry soft-deletes the entry (sets is_active=0).
	DeleteRegistryEntry(ctx context.Context, projectID, name string) error
	// GetRegistryEntry returns the active entry with the given name in a project.
	GetRegistryEntry(ctx context.Context, projectID, name string) (*RegistryEntry, error)

	// Server config operations (key-value store for auto-generated secrets)

	// GetServerConfig returns the value for the given key, or an error wrapping
	// sql.ErrNoRows if the key does not exist.
	GetServerConfig(ctx context.Context, key string) (string, error)
	// SetServerConfig upserts a key-value pair in the server_config table.
	SetServerConfig(ctx context.Context, key, value string) error

	// Provenance operations

	// CreateProvenance inserts a new provenance record for a registry-based run.
	// Returns an error if the run_id already has a provenance record (UNIQUE constraint).
	CreateProvenance(ctx context.Context, p *Provenance) error
	// TouchProvenanceUpdatedAt bumps the updated_at timestamp for the provenance record
	// associated with the given run_id. Returns nil (not an error) if no record exists.
	TouchProvenanceUpdatedAt(ctx context.Context, runID string) error
	// GetProvenance returns the full provenance detail for the given run_id in a project.
	GetProvenance(ctx context.Context, projectID, runID string) (*ProvenanceDetail, error)
	// ListProvenance returns summary rows for all provenance records in a project.
	// entryFilter limits results to a specific registry entry name (empty = all entries).
	// userFilter limits results to a specific username (empty = all users).
	ListProvenance(ctx context.Context, projectID, entryFilter, userFilter string) ([]ProvenanceSummary, error)

	// ManagedFile operations

	// CreateManagedFile persists a new managed file. f.ID must be set by the caller.
	CreateManagedFile(ctx context.Context, f *ManagedFile) error
	// ListManagedFiles returns all managed files for the given project.
	ListManagedFiles(ctx context.Context, projectID string) ([]ManagedFile, error)
	// GetManagedFileByID returns the managed file with the given ID.
	GetManagedFileByID(ctx context.Context, id string) (*ManagedFile, error)
	// DeleteManagedFile removes the managed file record with the given ID.
	DeleteManagedFile(ctx context.Context, id string) error
	// CompleteRun atomically marks a run as COMPLETED with the given outputs and
	// inserts any provided managed file records in a single transaction.
	// Pass nil or empty slice for files when no file registration is needed.
	CompleteRun(ctx context.Context, runID string, projectID string, outputsJSON string, files []ManagedFile) error

	// Lifecycle

	// Close releases all resources held by the store.
	Close() error
}
