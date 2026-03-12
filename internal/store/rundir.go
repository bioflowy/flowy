// Package store provides filesystem layout helpers for workflow run data.
// Run data is stored under {baseDir}/runs/{runID}/ with inputs/ and outputs/
// subdirectories for each task's data exchange with the daemon.
package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// RunInputsDir returns the inputs directory path for a run.
// It is used internally by CreateRunDirs and in tests; production code receives
// paths from the server via TaskClaimResponse.
// The directory is at: {baseDir}/runs/{runID}/inputs
func RunInputsDir(baseDir, runID string) string {
	return filepath.Join(baseDir, "runs", runID, "inputs")
}

// RunOutputsDir returns the outputs directory path for a run.
// It is used internally by CreateRunDirs and in tests; production code receives
// paths from the server via TaskClaimResponse.
// The directory is at: {baseDir}/runs/{runID}/outputs
func RunOutputsDir(baseDir, runID string) string {
	return filepath.Join(baseDir, "runs", runID, "outputs")
}

// TaskDir returns the absolute path to a per-task working directory.
// The directory is at: {baseDir}/runs/{runID}/tasks/{taskName}_{shardIndex}
// ShardIndex distinguishes scatter shards; non-scattered tasks use shardIndex=0.
func TaskDir(baseDir, runID, taskName string, shardIndex int) string {
	return filepath.Join(baseDir, "runs", runID, "tasks",
		fmt.Sprintf("%s_%d", taskName, shardIndex))
}

// CreateTaskDirs creates the per-task directory for the given task shard.
// It uses os.MkdirAll, so it is idempotent — calling it multiple times does
// not error or remove existing content.
func CreateTaskDirs(baseDir, runID, taskName string, shardIndex int) error {
	dir := TaskDir(baseDir, runID, taskName, shardIndex)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("store: create task dir %q: %w", dir, err)
	}
	return nil
}

// CreateRunDirs creates the inputs and outputs directories for a run.
// It uses os.MkdirAll, so it is idempotent — calling it multiple times on the
// same run does not error or remove existing content.
// Returns an error if either directory cannot be created.
func CreateRunDirs(baseDir, runID string) error {
	dirs := []string{
		RunInputsDir(baseDir, runID),
		RunOutputsDir(baseDir, runID),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("store: create run dir %q: %w", dir, err)
		}
	}
	return nil
}
