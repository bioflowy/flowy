package task

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// DockerRunner implements ContainerRunner using the Docker Engine API.
// It is stateless — a new Docker client is created per Run call for
// simplicity and thread-safety.
type DockerRunner struct{}

// NewDockerRunner returns a new DockerRunner.
func NewDockerRunner() *DockerRunner {
	return &DockerRunner{}
}

// Run executes a container via the Docker Engine API following the lifecycle:
// pull -> create -> wait-setup -> start -> wait -> logs -> remove.
//
// ContainerWait is set up BEFORE ContainerStart to avoid a race condition
// where the container exits before Wait is registered.
// AutoRemove is NOT used so that logs can be collected after the container exits.
func (r *DockerRunner) Run(ctx context.Context, req RunRequest) (RunResult, error) {
	// Create Docker client from environment (DOCKER_HOST, etc.).
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return RunResult{}, fmt.Errorf("docker: create client: %w", err)
	}
	defer cli.Close()

	// Pull image. Docker is a no-op if image is already up-to-date.
	// CRITICAL: drain and close response body to avoid connection pool exhaustion.
	pullResp, err := cli.ImagePull(ctx, req.Image, image.PullOptions{})
	if err != nil {
		return RunResult{}, fmt.Errorf("docker: pull %s: %w", req.Image, err)
	}
	io.Copy(io.Discard, pullResp) //nolint:errcheck
	pullResp.Close()

	// Create container (no AutoRemove — we collect logs after exit).
	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image:      req.Image,
			Cmd:        req.Command,
			Env:        req.Env,
			WorkingDir: req.WorkDir,
		},
		&container.HostConfig{
			Binds: req.Binds,
			Resources: container.Resources{
				Memory:   req.Memory,
				NanoCPUs: req.NanoCPUs,
			},
		},
		nil, nil, "", // no network config, no platform, no name
	)
	if err != nil {
		return RunResult{}, fmt.Errorf("docker: create container: %w", err)
	}

	// cleanup force-removes the container on any error path.
	cleanup := func() {
		cli.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true}) //nolint:errcheck
	}

	// CRITICAL: Set up wait BEFORE start to prevent race condition where
	// the container exits before we register the wait.
	waitCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNextExit)

	// Start container.
	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		cleanup()
		return RunResult{}, fmt.Errorf("docker: start container: %w", err)
	}

	// Wait for completion, respecting context cancellation.
	var exitCode int
	select {
	case waitResp := <-waitCh:
		exitCode = int(waitResp.StatusCode)
	case err := <-errCh:
		cleanup()
		return RunResult{}, fmt.Errorf("docker: wait container: %w", err)
	case <-ctx.Done():
		cleanup()
		return RunResult{}, ctx.Err()
	}

	// Collect logs. stdcopy.StdCopy demultiplexes the Docker multiplexed stream
	// into separate stdout and stderr buffers.
	logResp, err := cli.ContainerLogs(ctx, resp.ID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		cleanup()
		return RunResult{}, fmt.Errorf("docker: collect logs: %w", err)
	}
	var stdoutBuf, stderrBuf bytes.Buffer
	stdcopy.StdCopy(&stdoutBuf, &stderrBuf, logResp) //nolint:errcheck
	logResp.Close()

	// Remove container (non-force — it already exited normally).
	cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{}) //nolint:errcheck

	return RunResult{
		ExitCode: exitCode,
		Stdout:   stdoutBuf.Bytes(),
		Stderr:   stderrBuf.Bytes(),
	}, nil
}
