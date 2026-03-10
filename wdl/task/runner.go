// Package task implements WDL task execution: input binding, path mapping,
// runtime parsing, and the ContainerRunner interface contract.
package task

import "context"

// RunRequest is the input specification for a single container execution.
type RunRequest struct {
	Image    string   // Docker image reference (e.g., "ubuntu:22.04")
	Command  []string // Command and arguments (e.g., ["bash", "/mnt/inputs/_command"])
	Binds    []string // Docker bind-mount specs (e.g., "host:container:ro")
	Env      []string // Environment variable assignments (e.g., "KEY=VALUE")
	Memory   int64    // Memory limit in bytes (0 = no limit)
	NanoCPUs int64    // CPU limit in nanocpus (0 = no limit)
	WorkDir  string   // Container working directory (e.g., "/mnt/outputs")
}

// RunResult holds the outcome of a container execution.
type RunResult struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
}

// ContainerRunner is the interface that abstracts container execution.
// Implementations include DockerRunner (Plan 03) and mock runners for tests.
type ContainerRunner interface {
	Run(ctx context.Context, req RunRequest) (RunResult, error)
}
