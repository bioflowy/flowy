// Package client provides an HTTP client for communicating with the flowy-server API.
// CLI subcommands use this package as the transport layer for all server requests.
package client

import (
	"context"
	"net/http"
	"time"

	"github.com/bioflowy/flowy/internal/api"
)

// Client is an HTTP client for the flowy-server REST API.
// All methods are safe for concurrent use.
type Client struct {
	serverURL  string
	httpClient *http.Client
}

// New constructs a new Client targeting the given server URL (e.g. "http://localhost:8124").
// The underlying HTTP client has a 30-second timeout.
func New(serverURL string) *Client {
	return &Client{
		serverURL:  serverURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Submit sends POST /workflows with the given SubmitRequest and returns the SubmitResponse.
// Returns an error if the server returns a non-201 status.
func (c *Client) Submit(ctx context.Context, req api.SubmitRequest) (*api.SubmitResponse, error) {
	panic("not implemented")
}

// Status sends GET /workflows/{runID}/status and returns the StatusResponse.
// Returns an error if the server returns a non-200 status.
func (c *Client) Status(ctx context.Context, runID string) (*api.StatusResponse, error) {
	panic("not implemented")
}

// Outputs sends GET /workflows/{runID}/outputs and returns the OutputsResponse.
// On 409 (run not completed), returns a descriptive error including the current state.
// On 404 (run not found), returns a descriptive error.
func (c *Client) Outputs(ctx context.Context, runID string) (*api.OutputsResponse, error) {
	panic("not implemented")
}

// TaskRun sends POST /tasks/run with the given TaskRunRequest and returns TaskRunResponse.
// Returns an error if the server returns a non-201 status.
func (c *Client) TaskRun(ctx context.Context, req api.TaskRunRequest) (*api.TaskRunResponse, error) {
	panic("not implemented")
}
