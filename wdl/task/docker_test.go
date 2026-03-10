//go:build integration

package task

import (
	"context"
	"strings"
	"testing"
)

// TestDockerRunner runs integration tests against a live Docker daemon.
// Run with: go test ./wdl/task/... -run TestDockerRunner -tags integration -v
func TestDockerRunner(t *testing.T) {
	runner := NewDockerRunner()

	t.Run("echo hello", func(t *testing.T) {
		result, err := runner.Run(context.Background(), RunRequest{
			Image:   "alpine:latest",
			Command: []string{"echo", "hello"},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", result.ExitCode)
		}
		if got := string(result.Stdout); got != "hello\n" {
			t.Errorf("stdout = %q, want %q", got, "hello\n")
		}
	})

	t.Run("non-zero exit code and stderr", func(t *testing.T) {
		result, err := runner.Run(context.Background(), RunRequest{
			Image:   "alpine:latest",
			Command: []string{"sh", "-c", "echo err >&2; exit 42"},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		if result.ExitCode != 42 {
			t.Fatalf("expected exit code 42, got %d", result.ExitCode)
		}
		if !strings.Contains(string(result.Stderr), "err") {
			t.Errorf("stderr = %q, want it to contain %q", string(result.Stderr), "err")
		}
	})
}
