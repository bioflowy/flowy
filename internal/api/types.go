// Package api defines the HTTP DTO types used by the flowy HTTP API.
// This is a leaf package with no internal imports — only stdlib is used.
package api

// SubmitRequest is the request body for POST /workflows to submit a new workflow run.
type SubmitRequest struct {
	// WDLPath is the absolute path to the WDL file on the shared filesystem.
	WDLPath string `json:"wdl_path"`

	// InputsJSON contains the workflow-level input values keyed by input name.
	InputsJSON map[string]interface{} `json:"inputs"`

	// RunID is an optional client-provided UUID hint. When non-empty the server
	// uses this value as the run ID instead of generating a new UUID. This enables
	// pre-submission file staging by the client before the workflow is submitted.
	RunID string `json:"run_id,omitempty"`

	// SearchPaths are optional import search directories forwarded from the
	// client's --path flag so the server-side WDL parser can resolve imports.
	SearchPaths []string `json:"search_paths,omitempty"`
}

// SubmitResponse is the response body for POST /workflows.
type SubmitResponse struct {
	// RunID is the UUID v4 assigned to the newly submitted run.
	RunID string `json:"run_id"`
}

// StatusResponse is the response body for GET /workflows/{run_id}/status.
type StatusResponse struct {
	// RunID is the UUID v4 of the run.
	RunID string `json:"run_id"`

	// State is one of: PENDING, RUNNING, COMPLETED, FAILED, ABORTED.
	State string `json:"state"`
}

// OutputsResponse is the response body for GET /workflows/{run_id}/outputs.
type OutputsResponse struct {
	// RunID is the UUID v4 of the run.
	RunID string `json:"run_id"`

	// Outputs maps output names to their values (strings, arrays, or objects).
	Outputs map[string]interface{} `json:"outputs"`
}

// AbortResponse is the response body for POST /workflows/{run_id}/abort.
type AbortResponse struct {
	// RunID is the UUID v4 of the aborted run.
	RunID string `json:"run_id"`

	// State is the run state after the abort request (typically ABORTED or RUNNING
	// if the abort is still propagating).
	State string `json:"state"`
}

// ErrorResponse is the standard error body returned by all error responses.
type ErrorResponse struct {
	// Error is a human-readable description of the error.
	Error string `json:"error"`
}

// LoginRequest is the request body for POST /auth/login.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse is the response body for a successful POST /auth/login.
type LoginResponse struct {
	Token string `json:"token"`
}

// CreateUserRequest is the request body for POST /users (admin only).
type CreateUserRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	GlobalRole  string `json:"global_role"`
}

// UserResponse is the response body for user-related endpoints.
type UserResponse struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	GlobalRole  string `json:"global_role"`
	IsActive    bool   `json:"is_active"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// ChangePasswordRequest is the request body for PUT /users/me/password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// TaskRunRequest is the request body for POST /tasks/run.
// Runs a single named task from a WDL file (used by flowy run --task).
type TaskRunRequest struct {
	// WDLPath is the absolute path to the WDL file on the shared filesystem.
	WDLPath string `json:"wdl_path"`

	// TaskName is the name of the task to execute from the WDL file.
	TaskName string `json:"task_name"`

	// InputsJSON contains the task-level input values keyed by input name.
	InputsJSON map[string]interface{} `json:"inputs"`

	// RunID is an optional client-provided UUID hint (same as SubmitRequest.RunID).
	RunID string `json:"run_id,omitempty"`

	// SearchPaths are optional import search directories.
	SearchPaths []string `json:"search_paths,omitempty"`
}

// TaskRunResponse is the response body for POST /tasks/run.
type TaskRunResponse struct {
	// RunID is the UUID assigned to this task run.
	RunID string `json:"run_id"`
}

// TaskClaimRequest is the request body for POST /tasks/claim.
// A daemon sends this to atomically claim a pending task.
type TaskClaimRequest struct {
	// DaemonID is a stable identifier for the claiming daemon instance.
	DaemonID string `json:"daemon_id"`
}

// TaskClaimResponse is the response body for a successful POST /tasks/claim.
type TaskClaimResponse struct {
	// TaskID is the integer task ID (serialised as string for JSON compatibility).
	TaskID string `json:"task_id"`

	// RunID is the UUID v4 of the parent workflow run.
	RunID string `json:"run_id"`

	// TaskName is the WDL task name (e.g. "hello").
	TaskName string `json:"task_name"`

	// ShardIndex is the scatter shard index, or 0 for non-scattered tasks.
	ShardIndex int `json:"shard_index"`

	// InputsDir is the absolute path to the task's inputs directory on the shared FS.
	InputsDir string `json:"inputs_dir"`

	// OutputsDir is the absolute path to the task's outputs directory on the shared FS.
	OutputsDir string `json:"outputs_dir"`

	// WDLPath is the absolute path to the WDL file for this run.
	WDLPath string `json:"wdl_path"`

	// StorageType is the storage backend type: "local" or "s3".
	// Always populated so the daemon has a consistent contract.
	StorageType string `json:"storage_type"`

	// StorageBucket is the S3 bucket name. Empty string when StorageType is "local".
	StorageBucket string `json:"storage_bucket,omitempty"`

	// StorageEndpoint is the custom S3 endpoint URL (for MinIO). Empty for AWS S3.
	StorageEndpoint string `json:"storage_endpoint,omitempty"`

	// StoragePrefix is the key prefix for all S3 objects in this deployment.
	StoragePrefix string `json:"storage_prefix,omitempty"`

	// Inputs contains the parsed WDL input values for this task.
	// Populated by the server from stored run/task data so the daemon does
	// not need to read inputs.json from a shared filesystem.
	Inputs map[string]interface{} `json:"inputs,omitempty"`

	// WDLContent is the raw WDL file content. The daemon parses this in-memory
	// instead of reading the WDL file from a shared filesystem.
	WDLContent string `json:"wdl_content,omitempty"`
}

// HeartbeatRequest is the request body for POST /heartbeat.
// A daemon sends this periodically to signal it is still alive.
type HeartbeatRequest struct {
	// DaemonID is the stable identifier for the daemon sending the heartbeat.
	DaemonID string `json:"daemon_id"`
}

// CreateProjectRequest is the request body for POST /projects.
type CreateProjectRequest struct {
	// Slug is the unique human-readable identifier for the project.
	// Must be lowercase alphanumeric with hyphens, max 64 chars.
	Slug string `json:"slug"`

	// Description is an optional human-readable description of the project.
	Description string `json:"description"`
}

// UpdateProjectRequest is the request body for PUT /projects/{slug}.
type UpdateProjectRequest struct {
	// Description is the new description for the project.
	Description string `json:"description"`
}

// ProjectResponse is the response body for project-related endpoints.
type ProjectResponse struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	IsActive    bool   `json:"is_active"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// AddMemberRequest is the request body for POST /projects/{slug}/members.
type AddMemberRequest struct {
	// Username is the username of the user to add to the project.
	Username string `json:"username"`

	// Role is the role to assign: "admin", "executor", or "viewer".
	Role string `json:"role"`
}

// UpdateMemberRequest is the request body for PUT /projects/{slug}/members/{username}.
type UpdateMemberRequest struct {
	// Role is the new role for the member: "admin", "executor", or "viewer".
	Role string `json:"role"`
}

// MemberResponse is the response body for membership-related endpoints.
type MemberResponse struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// TaskResultRequest is the request body for POST /tasks/{task_id}/result.
// A daemon sends this after task execution completes (or fails).
type TaskResultRequest struct {
	// State is the terminal task state: COMPLETED or FAILED.
	State string `json:"state"`

	// Error is an optional human-readable error message when State is FAILED.
	Error string `json:"error,omitempty"`

	// Outputs contains the task output values reported by the daemon after
	// successful task execution. Keyed by plain output name (no task prefix).
	Outputs map[string]interface{} `json:"outputs,omitempty"`
}
