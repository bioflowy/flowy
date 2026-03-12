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
}

// HeartbeatRequest is the request body for POST /heartbeat.
// A daemon sends this periodically to signal it is still alive.
type HeartbeatRequest struct {
	// DaemonID is the stable identifier for the daemon sending the heartbeat.
	DaemonID string `json:"daemon_id"`
}

// TaskResultRequest is the request body for POST /tasks/{task_id}/result.
// A daemon sends this after task execution completes (or fails).
type TaskResultRequest struct {
	// State is the terminal task state: COMPLETED or FAILED.
	State string `json:"state"`

	// Error is an optional human-readable error message when State is FAILED.
	Error string `json:"error,omitempty"`
}
