// Package store provides filesystem layout helpers for workflow run data.
// Run data is stored under {baseDir}/runs/{runID}/ with inputs/ and outputs/
// subdirectories for each task's data exchange with the daemon.
package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// RunInputsDir returns the absolute path to the inputs directory for a run.
// The directory is at: {baseDir}/runs/{runID}/inputs
func RunInputsDir(baseDir, runID string) string {
	return filepath.Join(baseDir, "runs", runID, "inputs")
}

// RunOutputsDir returns the absolute path to the outputs directory for a run.
// The directory is at: {baseDir}/runs/{runID}/outputs
func RunOutputsDir(baseDir, runID string) string {
	return filepath.Join(baseDir, "runs", runID, "outputs")
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
