// Package client provides an HTTP client for communicating with the flowy-server API.
// CLI subcommands use this package as the transport layer for all server requests.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/bioflowy/flowy/internal/api"
)

// ErrUnauthorized is returned when the server responds with HTTP 401.
var ErrUnauthorized = errors.New("unauthorized")

// ErrForbidden is returned when the server responds with HTTP 403.
var ErrForbidden = errors.New("forbidden")

// Client is an HTTP client for the flowy-server REST API.
// All methods are safe for concurrent use.
type Client struct {
	serverURL  string
	httpClient *http.Client
	token      string
}

// New constructs a new Client targeting the given server URL (e.g. "http://localhost:8124").
// The underlying HTTP client has a 30-second timeout.
func New(serverURL string) *Client {
	return &Client{
		serverURL:  serverURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// NewWithToken constructs a Client that sets an Authorization Bearer token on
// every HTTP request. Use this when a saved JWT is available.
func NewWithToken(serverURL, token string) *Client {
	return &Client{
		serverURL:  serverURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		token:      token,
	}
}

// doRequest executes req, injecting the Authorization Bearer header when a token
// is configured. All Client methods must use doRequest instead of httpClient.Do
// so that token injection applies uniformly.
func (c *Client) doRequest(req *http.Request) (*http.Response, error) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return c.httpClient.Do(req)
}

// Login sends POST /auth/login and returns the JWT token on success.
// On network error the error message contains the server URL to aid diagnosis.
// On non-200 the error message contains the server-provided description.
func (c *Client) Login(ctx context.Context, username, password string) (string, error) {
	body, err := json.Marshal(api.LoginRequest{Username: username, Password: password})
	if err != nil {
		return "", fmt.Errorf("client: marshal login request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.serverURL+"/auth/login", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("client: build login request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(httpReq)
	if err != nil {
		return "", fmt.Errorf("cannot connect to server at %s: %w", c.serverURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp api.ErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return "", fmt.Errorf("login failed: %s", errResp.Error)
	}

	var lr api.LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return "", fmt.Errorf("client: decode login response: %w", err)
	}
	return lr.Token, nil
}

// Submit sends POST /workflows with the given SubmitRequest and returns the SubmitResponse.
// Returns an error if the server returns a non-201 status.
func (c *Client) Submit(ctx context.Context, req api.SubmitRequest) (*api.SubmitResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("client: marshal submit request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.serverURL+"/workflows", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("client: build submit request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(httpReq)
	if err != nil {
		return nil, fmt.Errorf("client: POST /workflows: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var errResp api.ErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("client: submit failed (status %d): %s", resp.StatusCode, errResp.Error)
	}

	var result api.SubmitResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("client: decode submit response: %w", err)
	}
	return &result, nil
}

// SubmitToProject sends POST /projects/{projectSlug}/workflows with the given SubmitRequest.
// Returns an error if the server returns a non-201 status.
func (c *Client) SubmitToProject(ctx context.Context, projectSlug string, req api.SubmitRequest) (*api.SubmitResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("client: marshal submit request: %w", err)
	}

	url := c.serverURL + "/projects/" + projectSlug + "/workflows"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("client: build submit request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(httpReq)
	if err != nil {
		return nil, fmt.Errorf("client: POST /projects/%s/workflows: %w", projectSlug, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var errResp api.ErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("client: submit failed (status %d): %s", resp.StatusCode, errResp.Error)
	}

	var result api.SubmitResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("client: decode submit response: %w", err)
	}
	return &result, nil
}

// TaskRunInProject sends POST /projects/{projectSlug}/tasks/run with the given TaskRunRequest.
// Returns an error if the server returns a non-201 status.
func (c *Client) TaskRunInProject(ctx context.Context, projectSlug string, req api.TaskRunRequest) (*api.TaskRunResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("client: marshal task run request: %w", err)
	}

	url := c.serverURL + "/projects/" + projectSlug + "/tasks/run"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("client: build task run request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(httpReq)
	if err != nil {
		return nil, fmt.Errorf("client: POST /projects/%s/tasks/run: %w", projectSlug, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var errResp api.ErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("client: task run failed (status %d): %s", resp.StatusCode, errResp.Error)
	}

	var result api.TaskRunResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("client: decode task run response: %w", err)
	}
	return &result, nil
}

// Status sends GET /workflows/{runID}/status and returns the StatusResponse.
// Returns an error if the server returns a non-200 status.
func (c *Client) Status(ctx context.Context, runID string) (*api.StatusResponse, error) {
	url := c.serverURL + "/workflows/" + runID + "/status"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("client: build status request: %w", err)
	}

	resp, err := c.doRequest(httpReq)
	if err != nil {
		return nil, fmt.Errorf("client: GET /workflows/%s/status: %w", runID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp api.ErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("client: status failed (status %d): %s", resp.StatusCode, errResp.Error)
	}

	var result api.StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("client: decode status response: %w", err)
	}
	return &result, nil
}

// StatusInProject sends GET /projects/{projectSlug}/runs/{runID}/status and returns the StatusResponse.
// Returns an error if the server returns a non-200 status.
func (c *Client) StatusInProject(ctx context.Context, projectSlug string, runID string) (*api.StatusResponse, error) {
	url := c.serverURL + "/projects/" + projectSlug + "/runs/" + runID + "/status"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("client: build status request: %w", err)
	}

	resp, err := c.doRequest(httpReq)
	if err != nil {
		return nil, fmt.Errorf("client: GET /projects/%s/runs/%s/status: %w", projectSlug, runID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp api.ErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("client: status failed (status %d): %s", resp.StatusCode, errResp.Error)
	}

	var result api.StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("client: decode status response: %w", err)
	}
	return &result, nil
}

// OutputsInProject sends GET /projects/{projectSlug}/runs/{runID}/outputs and returns the OutputsResponse.
// On 409 (run not completed), returns a descriptive error including the current state.
// On 404 (run not found), returns a descriptive error.
func (c *Client) OutputsInProject(ctx context.Context, projectSlug string, runID string) (*api.OutputsResponse, error) {
	url := c.serverURL + "/projects/" + projectSlug + "/runs/" + runID + "/outputs"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("client: build outputs request: %w", err)
	}

	resp, err := c.doRequest(httpReq)
	if err != nil {
		return nil, fmt.Errorf("client: GET /projects/%s/runs/%s/outputs: %w", projectSlug, runID, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var result api.OutputsResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("client: decode outputs response: %w", err)
		}
		return &result, nil
	case http.StatusConflict:
		var errResp api.ErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("client: workflow not yet completed (state: %s)", errResp.Error)
	case http.StatusNotFound:
		var errResp api.ErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("client: run not found: %s", errResp.Error)
	default:
		var errResp api.ErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("client: outputs failed (status %d): %s", resp.StatusCode, errResp.Error)
	}
}

// Outputs sends GET /workflows/{runID}/outputs and returns the OutputsResponse.
// On 409 (run not completed), returns a descriptive error including the current state.
// On 404 (run not found), returns a descriptive error.
func (c *Client) Outputs(ctx context.Context, runID string) (*api.OutputsResponse, error) {
	url := c.serverURL + "/workflows/" + runID + "/outputs"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("client: build outputs request: %w", err)
	}

	resp, err := c.doRequest(httpReq)
	if err != nil {
		return nil, fmt.Errorf("client: GET /workflows/%s/outputs: %w", runID, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var result api.OutputsResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("client: decode outputs response: %w", err)
		}
		return &result, nil
	case http.StatusConflict:
		// Run not yet completed — parse the error message to include the current state.
		var errResp api.ErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("client: workflow not yet completed (state: %s)", errResp.Error)
	case http.StatusNotFound:
		var errResp api.ErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("client: run not found: %s", errResp.Error)
	default:
		var errResp api.ErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("client: outputs failed (status %d): %s", resp.StatusCode, errResp.Error)
	}
}

// TaskRun sends POST /tasks/run with the given TaskRunRequest and returns TaskRunResponse.
// Returns an error if the server returns a non-201 status.
func (c *Client) TaskRun(ctx context.Context, req api.TaskRunRequest) (*api.TaskRunResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("client: marshal task run request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.serverURL+"/tasks/run", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("client: build task run request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(httpReq)
	if err != nil {
		return nil, fmt.Errorf("client: POST /tasks/run: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var errResp api.ErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("client: task run failed (status %d): %s", resp.StatusCode, errResp.Error)
	}

	var result api.TaskRunResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("client: decode task run response: %w", err)
	}
	return &result, nil
}
