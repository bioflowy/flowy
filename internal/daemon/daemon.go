// Package daemon implements the flowy-daemon worker process.
// It polls the flowy-server for pending tasks, executes them via Docker,
// writes output JSON, sends heartbeats, and shuts down gracefully on SIGTERM.
package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bioflowy/flowy/internal/api"
	"github.com/bioflowy/flowy/internal/config"
	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/parser"
	"github.com/bioflowy/flowy/wdl/task"
	"github.com/bioflowy/flowy/wdl/values"
)

// Daemon is the main worker struct that polls, executes, and reports tasks.
type Daemon struct {
	cfg      *config.Config
	daemonID string
	client   *http.Client
	wg       sync.WaitGroup
}

// New constructs a new Daemon. The daemonID should be unique per process
// (e.g. "hostname-PID", generated in main.go).
// HTTP client timeout is set to 35 seconds to accommodate long-poll responses.
func New(cfg *config.Config, daemonID string) *Daemon {
	return &Daemon{
		cfg:      cfg,
		daemonID: daemonID,
		client:   &http.Client{Timeout: 35 * time.Second},
	}
}

// Run starts the daemon: launches the heartbeat goroutine, spawns N worker
// goroutines (N = cfg.Daemon.Workers), and blocks until all workers exit.
// Workers stop claiming new tasks when ctx is cancelled (e.g. on SIGTERM).
// In-flight task executions are allowed to complete before Run returns.
func (d *Daemon) Run(ctx context.Context) error {
	log.Printf("daemon starting daemon_id=%s server=%s workers=%d base_dir=%s",
		d.daemonID, d.cfg.Daemon.ServerURL, d.cfg.Daemon.Workers, d.cfg.BaseDir)

	// Start heartbeat goroutine for the full daemon lifetime.
	go d.startHeartbeat(ctx)

	// Start N worker goroutines.
	for i := 0; i < d.cfg.Daemon.Workers; i++ {
		d.wg.Add(1)
		go func() {
			defer d.wg.Done()
			d.workerLoop(ctx)
		}()
	}

	// Wait for all workers to finish claiming.
	d.wg.Wait()
	return nil
}

// Shutdown blocks until all in-flight task executions complete.
// Call this after Run returns or after cancelling the context passed to Run.
func (d *Daemon) Shutdown() {
	d.wg.Wait()
}

// workerLoop polls for tasks on a 5-second tick and executes each claimed task.
// It stops polling when ctx is cancelled, but in-flight tasks complete naturally.
func (d *Daemon) workerLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			claim, err := d.claimTask(ctx)
			if err != nil {
				log.Printf("daemon: claimTask error: %v", err)
				continue
			}
			if claim == nil {
				// No pending tasks — continue polling.
				continue
			}

			// Execute the task in a separate goroutine with its own WaitGroup
			// entry. Use context.Background() so SIGTERM does not interrupt
			// Docker mid-execution (graceful shutdown pitfall 6 per research).
			d.wg.Add(1)
			go d.executeTask(claim)
		}
	}
}

// claimTask sends POST /tasks/claim and returns the claimed task, or nil if
// there are no pending tasks (HTTP 204). Returns an error for other failures.
func (d *Daemon) claimTask(ctx context.Context) (*api.TaskClaimResponse, error) {
	body, err := json.Marshal(api.TaskClaimRequest{DaemonID: d.daemonID})
	if err != nil {
		return nil, fmt.Errorf("marshal claim request: %w", err)
	}

	url := d.cfg.Daemon.ServerURL + "/tasks/claim"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build claim request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST /tasks/claim: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		// No pending tasks.
		return nil, nil
	case http.StatusOK:
		var claim api.TaskClaimResponse
		if err := json.NewDecoder(resp.Body).Decode(&claim); err != nil {
			return nil, fmt.Errorf("decode claim response: %w", err)
		}
		return &claim, nil
	default:
		return nil, fmt.Errorf("unexpected status from /tasks/claim: %d", resp.StatusCode)
	}
}

// executeTask runs the claimed task via Docker, writes outputs.json, and
// reports the result to the server. It uses context.Background() so that
// a SIGTERM (which cancels the poll context) does not kill in-flight Docker.
func (d *Daemon) executeTask(claim *api.TaskClaimResponse) {
	defer d.wg.Done()

	taskCtx := context.Background()

	log.Printf("daemon: executing task_id=%s task=%s run=%s", claim.TaskID, claim.TaskName, claim.RunID)

	// Read inputs.json from the task directory.
	inputsPath := filepath.Join(claim.InputsDir, "inputs.json")
	inputsData, err := os.ReadFile(inputsPath)
	if err != nil {
		log.Printf("daemon: read inputs.json task_id=%s: %v", claim.TaskID, err)
		d.reportResult(taskCtx, claim.TaskID, "FAILED", fmt.Sprintf("read inputs.json: %v", err))
		return
	}

	var jsonInputs map[string]interface{}
	if err := json.Unmarshal(inputsData, &jsonInputs); err != nil {
		log.Printf("daemon: parse inputs.json task_id=%s: %v", claim.TaskID, err)
		d.reportResult(taskCtx, claim.TaskID, "FAILED", fmt.Sprintf("parse inputs.json: %v", err))
		return
	}

	// Parse the WDL document.
	doc, err := parser.Parse(claim.WDLPath, nil)
	if err != nil {
		log.Printf("daemon: parse WDL task_id=%s path=%s: %v", claim.TaskID, claim.WDLPath, err)
		d.reportResult(taskCtx, claim.TaskID, "FAILED", fmt.Sprintf("parse WDL %s: %v", claim.WDLPath, err))
		return
	}

	// Find the ast.Task by name in doc.Tasks.
	var astTask *ast.Task
	for _, t := range doc.Tasks {
		if t.Name == claim.TaskName {
			astTask = t
			break
		}
	}
	if astTask == nil {
		log.Printf("daemon: task %q not found in WDL %s", claim.TaskName, claim.WDLPath)
		d.reportResult(taskCtx, claim.TaskID, "FAILED",
			fmt.Sprintf("task %q not found in WDL %s", claim.TaskName, claim.WDLPath))
		return
	}

	// Build struct definitions from the document (same pattern as scheduler.structDefs).
	structDefs := make(map[string]*ast.StructDef, len(doc.Structs))
	for _, sd := range doc.Structs {
		structDefs[sd.Name] = sd
	}

	// Execute the task via Docker.
	outputs, err := task.RunTask(taskCtx, astTask, jsonInputs, task.NewDockerRunner(), claim.OutputsDir, structDefs)
	if err != nil {
		log.Printf("daemon: RunTask task_id=%s: %v", claim.TaskID, err)
		d.reportResult(taskCtx, claim.TaskID, "FAILED", fmt.Sprintf("run task: %v", err))
		return
	}

	// Serialize outputs to JSON and write outputs.json.
	jsonOutputs, err := valuesToJSON(outputs)
	if err != nil {
		log.Printf("daemon: valuesToJSON task_id=%s: %v", claim.TaskID, err)
		d.reportResult(taskCtx, claim.TaskID, "FAILED", fmt.Sprintf("serialize outputs: %v", err))
		return
	}

	outputsData, err := json.Marshal(jsonOutputs)
	if err != nil {
		log.Printf("daemon: marshal outputs task_id=%s: %v", claim.TaskID, err)
		d.reportResult(taskCtx, claim.TaskID, "FAILED", fmt.Sprintf("marshal outputs: %v", err))
		return
	}

	outputsPath := filepath.Join(claim.OutputsDir, "outputs.json")
	if err := os.WriteFile(outputsPath, outputsData, 0644); err != nil {
		log.Printf("daemon: write outputs.json task_id=%s: %v", claim.TaskID, err)
		d.reportResult(taskCtx, claim.TaskID, "FAILED", fmt.Sprintf("write outputs.json: %v", err))
		return
	}

	log.Printf("daemon: task completed task_id=%s", claim.TaskID)
	d.reportResult(taskCtx, claim.TaskID, "COMPLETED", "")
}

// sendHeartbeat sends a single POST /heartbeat request. Returns an error if
// the request fails so the caller can log it.
func (d *Daemon) sendHeartbeat(ctx context.Context) error {
	body, err := json.Marshal(api.HeartbeatRequest{DaemonID: d.daemonID})
	if err != nil {
		return fmt.Errorf("marshal heartbeat: %w", err)
	}

	url := d.cfg.Daemon.ServerURL + "/heartbeat"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build heartbeat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("POST /heartbeat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status from /heartbeat: %d", resp.StatusCode)
	}
	return nil
}

// startHeartbeat runs a background goroutine that sends periodic heartbeats
// to the server until ctx is cancelled.
func (d *Daemon) startHeartbeat(ctx context.Context) {
	interval := time.Duration(d.cfg.Daemon.HeartbeatSec) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := d.sendHeartbeat(ctx); err != nil {
				log.Printf("daemon: heartbeat error: %v", err)
			}
		}
	}
}

// reportResult sends POST /tasks/{taskID}/result to the server.
// Errors are logged but not returned — task state will be recovered via
// stale detection if the report fails.
func (d *Daemon) reportResult(ctx context.Context, taskID, state, errMsg string) {
	body, err := json.Marshal(api.TaskResultRequest{State: state, Error: errMsg})
	if err != nil {
		log.Printf("daemon: marshal result task_id=%s: %v", taskID, err)
		return
	}

	url := d.cfg.Daemon.ServerURL + "/tasks/" + taskID + "/result"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		log.Printf("daemon: build result request task_id=%s: %v", taskID, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		log.Printf("daemon: POST /tasks/%s/result: %v", taskID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("daemon: unexpected status from /tasks/%s/result: %d", taskID, resp.StatusCode)
	}
}

// valuesToJSON converts a map of WDL runtime values to JSON-compatible Go types.
// Unknown value types are rendered as strings via fmt.Sprintf.
func valuesToJSON(outputs map[string]values.Value) (map[string]interface{}, error) {
	result := make(map[string]interface{}, len(outputs))
	for k, v := range outputs {
		result[k] = valueToInterface(v)
	}
	return result, nil
}

// valueToInterface converts a single WDL Value to a JSON-compatible interface{}.
func valueToInterface(v values.Value) interface{} {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case values.StringValue:
		return val.Value
	case values.IntValue:
		return val.Value
	case values.FloatValue:
		return val.Value
	case values.BooleanValue:
		return val.Value
	case values.FileValue:
		return val.Value
	case values.DirectoryValue:
		return val.Value
	case values.NullValue:
		return nil
	case values.ArrayValue:
		items := make([]interface{}, len(val.Items))
		for i, item := range val.Items {
			items[i] = valueToInterface(item)
		}
		return items
	case values.MapValue:
		m := make(map[string]interface{}, len(val.Entries))
		for _, entry := range val.Entries {
			key := fmt.Sprintf("%v", valueToInterface(entry.Key))
			m[key] = valueToInterface(entry.Value)
		}
		return m
	case values.StructValue:
		m := make(map[string]interface{}, len(val.Fields))
		for k, fv := range val.Fields {
			m[k] = valueToInterface(fv)
		}
		return m
	case values.PairValue:
		return map[string]interface{}{
			"left":  valueToInterface(val.Left),
			"right": valueToInterface(val.Right),
		}
	default:
		return fmt.Sprintf("%v", v)
	}
}
