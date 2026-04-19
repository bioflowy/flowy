// Package api defines the HTTP DTO types used by the flowy HTTP API.
// This is a leaf package with no internal imports — only stdlib is used.
package api

// SubmitRequest is the request body for POST /workflows to submit a new workflow run.
type SubmitRequest struct {
	// WDLPath is the absolute path to the WDL file on the shared filesystem.
	WDLPath string `json:"wdl_path"`

	// RegistryRef is a registry reference (e.g. "name:tag" or "name@sha256:hex").
	// Mutually exclusive with WDLPath. Exactly one must be set.
	RegistryRef string `json:"registry_ref,omitempty"`

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

// RunListItem is a single row in the project runs list.
type RunListItem struct {
	ID          string `json:"id"`
	State       string `json:"state"`
	RegistryRef string `json:"registry_ref,omitempty"`
	WDLPath     string `json:"wdl_path,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
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

// AdminResetPasswordRequest is the request body for PUT /users/{id}/password (admin only).
type AdminResetPasswordRequest struct {
	NewPassword string `json:"new_password"`
}

// TaskRunRequest is the request body for POST /tasks/run.
// Runs a single named task from a WDL file (used by flowy run --task).
type TaskRunRequest struct {
	// WDLPath is the absolute path to the WDL file on the shared filesystem.
	WDLPath string `json:"wdl_path"`

	// RegistryRef is a registry reference (e.g. "name:tag" or "name@sha256:hex").
	// Mutually exclusive with WDLPath. Exactly one must be set.
	RegistryRef string `json:"registry_ref,omitempty"`

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

// RegistryPushRequest is the request body for POST /projects/{slug}/registry.
type RegistryPushRequest struct {
	// Name is the entry name (slug format: lowercase alphanumeric + hyphens, 1-64 chars).
	Name string `json:"name"`

	// Content is the raw WDL file content as a string.
	Content string `json:"content"`
}

// RegistryPushResponse is the response body for POST /projects/{slug}/registry.
type RegistryPushResponse struct {
	Name      string `json:"name"`
	Digest    string `json:"digest"`
	Kind      string `json:"kind"`
	VersionID string `json:"version_id"`
}

// RegistryPullResponse is the response body for GET /projects/{slug}/registry/{name}.
type RegistryPullResponse struct {
	Name    string `json:"name"`
	Digest  string `json:"digest"`
	Kind    string `json:"kind"`
	Content string `json:"content"`
}

// RegistryEntryResponse is a single entry in the list response.
type RegistryEntryResponse struct {
	Name         string   `json:"name"`
	Kind         string   `json:"kind"`
	VersionCount int      `json:"version_count"`
	Tags         []string `json:"tags"`
	CreatedAt    string   `json:"created_at"`
}

// RegistryVersionResponse is a single version in the versions list response.
type RegistryVersionResponse struct {
	ID        string   `json:"id"`
	Digest    string   `json:"digest"`
	Tags      []string `json:"tags"`
	CreatedAt string   `json:"created_at"`
}

// RegistryTagRequest is the request body for POST /projects/{slug}/registry/{name}/tags.
type RegistryTagRequest struct {
	// Tag is the tag name (any string up to 128 chars, no '@' or ':').
	Tag string `json:"tag"`

	// Digest is the version digest to tag ("sha256:<hex>").
	Digest string `json:"digest"`
}

// RegistryInputItem describes a single declared input from a WDL workflow or task.
type RegistryInputItem struct {
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Optional bool    `json:"optional"`
	Default  *string `json:"default"`
	FileType *string `json:"file_type"`
}

// RegistryInputsResponse is the response body for GET /projects/{slug}/registry/{name}/inputs.
type RegistryInputsResponse struct {
	Inputs []RegistryInputItem `json:"inputs"`
}

// UploadResponse is the response body for POST /projects/{slug}/uploads.
type UploadResponse struct {
	// Path is the server-accessible absolute path where the uploaded file is stored.
	// Clients use this path as a File-typed workflow input value.
	Path string `json:"path"`
}

// ManagedFileResponse is a single file entry in library responses.
type ManagedFileResponse struct {
	ID         string  `json:"id"`
	Path       string  `json:"path"`
	FileType   string  `json:"file_type"`
	Label      string  `json:"label"`
	SourceType string  `json:"source_type"`
	RunID      *string `json:"run_id,omitempty"`
	Size       int64   `json:"size"`
	CreatedAt  string  `json:"created_at"`
}

// ListManagedFilesResponse is the response body for GET /projects/{slug}/files.
type ListManagedFilesResponse struct {
	Files []ManagedFileResponse `json:"files"`
}

// ImportFileRequest is the JSON request body for POST /projects/{slug}/files
// when importing by URI or server-local path.
type ImportFileRequest struct {
	URI      string `json:"uri"`
	FileType string `json:"file_type,omitempty"`
}

// ImportFileResponse is the response body for POST /projects/{slug}/files.
type ImportFileResponse struct {
	File ManagedFileResponse `json:"file"`
}

// ProvenanceSummaryResponse is a single item in the provenance list response.
type ProvenanceSummaryResponse struct {
	RunID          string `json:"run_id"`
	RegistryRef    string `json:"registry_ref"`
	RegistryDigest string `json:"registry_digest"`
	Username       string `json:"username"`
	RunState       string `json:"run_state"`
	CreatedAt      string `json:"created_at"`
}

// ProvenanceDetailResponse is the full provenance record for a single run.
type ProvenanceDetailResponse struct {
	RunID             string                 `json:"run_id"`
	RegistryRef       string                 `json:"registry_ref"`
	RegistryDigest    string                 `json:"registry_digest"`
	Username          string                 `json:"username"`
	RunState          string                 `json:"run_state"`
	Inputs            map[string]interface{} `json:"inputs,omitempty"`
	Outputs           map[string]interface{} `json:"outputs,omitempty"`
	RegistryEntryName string                 `json:"registry_entry_name"`
	RegistryEntryKind string                 `json:"registry_entry_kind"`
	CreatedAt         string                 `json:"created_at"`
	UpdatedAt         string                 `json:"updated_at"`
}
