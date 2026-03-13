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
	"strings"
	"sync"
	"time"

	"github.com/bioflowy/flowy/internal/api"
	"github.com/bioflowy/flowy/internal/config"
	"github.com/bioflowy/flowy/internal/storage"
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
	// BaseDir is logged for operator visibility but is not used for task path construction.
	// All task paths (InputsDir, OutputsDir, TaskDir) come from the server via TaskClaimResponse.
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

// workerLoop polls for tasks on a 1-second tick and executes each claimed task.
// It stops polling when ctx is cancelled, but in-flight tasks complete naturally.
func (d *Daemon) workerLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Immediate first poll to pick up tasks without waiting for the ticker.
	if err := d.tryClaimAndExecute(ctx); err != nil {
		log.Printf("daemon: claimTask error: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := d.tryClaimAndExecute(ctx); err != nil {
				log.Printf("daemon: claimTask error: %v", err)
			}
		}
	}
}

// tryClaimAndExecute attempts to claim a task and execute it.
// Returns an error only for claim failures; nil if no task is available.
func (d *Daemon) tryClaimAndExecute(ctx context.Context) error {
	claim, err := d.claimTask(ctx)
	if err != nil {
		return err
	}
	if claim == nil {
		return nil
	}
	// Execute the task in a separate goroutine with its own WaitGroup
	// entry. Use context.Background() so SIGTERM does not interrupt
	// Docker mid-execution (graceful shutdown pitfall 6 per research).
	d.wg.Add(1)
	go d.executeTask(claim)
	return nil
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

// buildBackendFromClaim constructs an S3Backend from the storage fields in the claim.
// The daemon reads its own credentials from the environment/IAM role; no credentials
// are passed in the claim.
func buildBackendFromClaim(claim *api.TaskClaimResponse) (storage.Backend, error) {
	cfg := &config.Config{}
	cfg.Storage.Type = claim.StorageType
	cfg.Storage.Bucket = claim.StorageBucket
	cfg.Storage.Endpoint = claim.StorageEndpoint
	cfg.Storage.Prefix = claim.StoragePrefix
	cfg.Storage.MultipartThreshold = 100 * 1024 * 1024 // 100 MiB default

	backend, err := storage.NewS3Backend(cfg)
	if err != nil {
		return nil, fmt.Errorf("build backend: %w", err)
	}
	return backend, nil
}

// walkAndLocalize recursively walks a JSON-compatible value and rewrites any
// "s3://" strings to local staged paths by downloading the file to inputsDir.
// Non-s3:// strings, booleans, numbers, nil, arrays, and maps are passed through unchanged.
func walkAndLocalize(v any, backend storage.Backend, inputsDir string) (any, error) {
	switch val := v.(type) {
	case string:
		if !strings.HasPrefix(val, "s3://") {
			return val, nil
		}
		// Parse the S3 URI to extract bucket and key.
		_, key, err := storage.ParseS3URI(val)
		if err != nil {
			return nil, fmt.Errorf("localize: download %s: %w", val, err)
		}
		// Download the file contents.
		data, err := backend.DownloadFile(key)
		if err != nil {
			return nil, fmt.Errorf("localize: download %s: %w", val, err)
		}
		// Write to local staging directory using the base filename only.
		localPath := filepath.Join(inputsDir, filepath.Base(key))
		if err := os.WriteFile(localPath, data, 0644); err != nil {
			return nil, fmt.Errorf("localize: download %s: write local file: %w", val, err)
		}
		return localPath, nil

	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			rewritten, err := walkAndLocalize(item, backend, inputsDir)
			if err != nil {
				return nil, err
			}
			result[i] = rewritten
		}
		return result, nil

	case map[string]any:
		result := make(map[string]any, len(val))
		for k, item := range val {
			rewritten, err := walkAndLocalize(item, backend, inputsDir)
			if err != nil {
				return nil, err
			}
			result[k] = rewritten
		}
		return result, nil

	default:
		// bool, float64, nil — pass through unchanged.
		return v, nil
	}
}

// localizeInputs creates a temporary staging directory, downloads all s3:// inputs
// to the staging inputs subdirectory, and returns the rewritten inputs map and stageDir.
// On error, stageDir is cleaned up before returning.
func localizeInputs(inputs map[string]any, backend storage.Backend) (localizedInputs map[string]any, stageDir string, err error) {
	stageDir, err = os.MkdirTemp("", "flowy-stage-")
	if err != nil {
		return nil, "", fmt.Errorf("localizeInputs: create temp dir: %w", err)
	}

	inputsDir := filepath.Join(stageDir, "inputs")
	if err = os.MkdirAll(inputsDir, 0755); err != nil {
		os.RemoveAll(stageDir)
		return nil, "", fmt.Errorf("localizeInputs: create inputs dir: %w", err)
	}

	rewritten, err := walkAndLocalize(inputs, backend, inputsDir)
	if err != nil {
		os.RemoveAll(stageDir)
		return nil, "", err
	}

	localizedInputs, ok := rewritten.(map[string]any)
	if !ok {
		os.RemoveAll(stageDir)
		return nil, "", fmt.Errorf("localizeInputs: unexpected type after walk: %T", rewritten)
	}

	return localizedInputs, stageDir, nil
}

// rewriteOutputPaths recursively replaces local file paths in a JSON-compatible
// value with their corresponding s3:// URIs from the localToS3 mapping.
func rewriteOutputPaths(v any, localToS3 map[string]string) any {
	switch val := v.(type) {
	case string:
		// Use filepath.Clean so normalization differences (extra /./,  //, trailing slash)
		// do not cause staging paths to miss the map and leak into output values.
		cleaned := filepath.Clean(val)
		if s3URI, ok := localToS3[cleaned]; ok {
			return s3URI
		}
		return val
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = rewriteOutputPaths(item, localToS3)
		}
		return result
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, item := range val {
			result[k] = rewriteOutputPaths(item, localToS3)
		}
		return result
	default:
		return v
	}
}

// delocalizeOutputs walks workDir, uploads each file to S3, and returns
// the outputs map with local file paths rewritten to s3:// URIs.
// Returns an error (with "delocalize: upload" prefix) on the first upload failure.
func delocalizeOutputs(outputs map[string]values.Value, workDir string, claim *api.TaskClaimResponse, backend storage.Backend) (map[string]any, error) {
	// localToS3 maps local absolute file path -> s3:// URI
	localToS3 := make(map[string]string)

	err := filepath.WalkDir(workDir, func(filePath string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		// Compute relative path for the S3 key.
		rel, err := filepath.Rel(workDir, filePath)
		if err != nil {
			return fmt.Errorf("delocalize: compute rel path: %w", err)
		}
		// Use slash separators for S3 keys.
		keyRel := filepath.ToSlash(rel)

		// Build S3 key using OutputsDir from claim as the prefix.
		// OutputsDir is already the task's S3 key prefix in S3 mode.
		s3Key := claim.OutputsDir + "/" + keyRel

		// Open and upload the file via streaming.
		f, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("delocalize: upload %s: open file: %w", s3Key, err)
		}
		defer f.Close()

		if err := backend.UploadFileStream(s3Key, f); err != nil {
			return fmt.Errorf("delocalize: upload %s: %w", s3Key, err)
		}

		// Record the mapping: local path -> s3:// URI
		// Use filepath.Clean on the key so lookup normalization matches.
		s3URI := "s3://" + claim.StorageBucket + "/" + s3Key
		localToS3[filepath.Clean(filePath)] = s3URI
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Convert WDL values to JSON-compatible types.
	jsonOutputs, err := values.ValuesToJSON(outputs)
	if err != nil {
		return nil, fmt.Errorf("delocalize: serialize outputs: %w", err)
	}

	// Rewrite any local file paths in the outputs map to s3:// URIs.
	rewritten := rewriteOutputPaths(jsonOutputs, localToS3)
	result, ok := rewritten.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("delocalize: unexpected type after rewrite: %T", rewritten)
	}

	return result, nil
}

// reportResultWithOutputs sends POST /tasks/{taskID}/result with Outputs populated.
// Errors are logged but not returned — task state will be recovered via
// stale detection if the report fails.
func (d *Daemon) reportResultWithOutputs(ctx context.Context, taskID, state, errMsg string, outputs map[string]any) {
	body, err := json.Marshal(api.TaskResultRequest{State: state, Error: errMsg, Outputs: outputs})
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

// executeTask runs the claimed task via Docker and reports the result to the server.
// It uses context.Background() so that a SIGTERM (which cancels the poll context)
// does not kill in-flight Docker.
//
// Both local and S3 modes now read inputs from claim.Inputs (API delivery).
// S3 mode performs LED: Localize inputs from S3, Execute task, Delocalize outputs to S3.
// Local mode executes directly and also writes outputs.json for backward compatibility.
func (d *Daemon) executeTask(claim *api.TaskClaimResponse) {
	defer d.wg.Done()

	taskCtx := context.Background()

	log.Printf("daemon: executing task_id=%s task=%s run=%s", claim.TaskID, claim.TaskName, claim.RunID)

	// Parse the WDL document. Use WDLContent if provided; fall back to WDLPath for backward compat.
	var doc *ast.Document
	var parseErr error
	if claim.WDLContent != "" {
		doc, parseErr = parser.Parse(claim.WDLPath, []byte(claim.WDLContent))
	} else {
		doc, parseErr = parser.Parse(claim.WDLPath, nil)
	}
	if parseErr != nil {
		log.Printf("daemon: parse WDL task_id=%s: %v", claim.TaskID, parseErr)
		d.reportResult(taskCtx, claim.TaskID, "FAILED", fmt.Sprintf("parse WDL: %v", parseErr))
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
		log.Printf("daemon: task %q not found in WDL", claim.TaskName)
		d.reportResult(taskCtx, claim.TaskID, "FAILED",
			fmt.Sprintf("task %q not found in WDL", claim.TaskName))
		return
	}

	// Build struct definitions from the document.
	structDefs := make(map[string]*ast.StructDef, len(doc.Structs))
	for _, sd := range doc.Structs {
		structDefs[sd.Name] = sd
	}

	if claim.StorageType == "s3" {
		d.executeTaskS3(taskCtx, claim, astTask, structDefs)
	} else {
		d.executeTaskLocal(taskCtx, claim, astTask, structDefs)
	}
}

// executeTaskS3 performs the LED (Localize-Execute-Delocalize) pattern for S3-backed tasks.
// Inputs are downloaded from S3 to a temp staging dir, the task runs with local POSIX paths,
// outputs are uploaded back to S3, and results are reported with s3:// output URIs.
func (d *Daemon) executeTaskS3(taskCtx context.Context, claim *api.TaskClaimResponse, astTask *ast.Task, structDefs map[string]*ast.StructDef) {
	// Build the S3 backend from claim's storage fields.
	backend, err := buildBackendFromClaim(claim)
	if err != nil {
		log.Printf("daemon: build backend task_id=%s: %v", claim.TaskID, err)
		d.reportResult(taskCtx, claim.TaskID, "FAILED", fmt.Sprintf("build backend: %v", err))
		return
	}

	// Localize: download s3:// inputs to a temporary staging directory.
	jsonInputs := claim.Inputs
	if jsonInputs == nil {
		jsonInputs = map[string]interface{}{}
	}
	localizedInputs, stageDir, err := localizeInputs(jsonInputs, backend)
	if err != nil {
		log.Printf("daemon: localize inputs task_id=%s: %v", claim.TaskID, err)
		d.reportResult(taskCtx, claim.TaskID, "FAILED", fmt.Sprintf("localize inputs: %v", err))
		return
	}

	// keepStage controls whether stageDir is preserved after execution.
	// It is set to true on delocalize failure to aid debugging.
	keepStage := false
	defer func() {
		if !keepStage {
			os.RemoveAll(stageDir)
		}
	}()

	// Resolve wdlDir for relative input path resolution (matches executeTaskLocal behavior).
	wdlDir := ""
	if claim.WDLPath != "" {
		wdlDir = filepath.Dir(claim.WDLPath)
	}

	// Execute: run the task with localized (POSIX) inputs.
	// execRoot is stageDir; wdlDir resolved from WDLPath for relative file inputs.
	outputs, err := task.RunTask(taskCtx, astTask, localizedInputs, task.NewDockerRunner(), stageDir, wdlDir, structDefs)
	if err != nil {
		log.Printf("daemon: RunTask task_id=%s: %v", claim.TaskID, err)
		d.reportResult(taskCtx, claim.TaskID, "FAILED", fmt.Sprintf("run task: %v", err))
		return
	}

	// Delocalize: upload output files from stageDir to S3 and rewrite paths.
	jsonOutputs, err := delocalizeOutputs(outputs, stageDir, claim, backend)
	if err != nil {
		log.Printf("daemon: delocalize outputs task_id=%s: %v", claim.TaskID, err)
		keepStage = true // preserve stageDir for debugging
		d.reportResult(taskCtx, claim.TaskID, "FAILED", fmt.Sprintf("delocalize outputs: %v", err))
		return
	}

	log.Printf("daemon: task completed task_id=%s", claim.TaskID)
	d.reportResultWithOutputs(taskCtx, claim.TaskID, "COMPLETED", "", jsonOutputs)
}

// executeTaskLocal executes a task in local storage mode.
// Inputs are read from claim.Inputs (API delivery).
// Outputs are written to outputs.json (backward compat) and also reported via API.
func (d *Daemon) executeTaskLocal(taskCtx context.Context, claim *api.TaskClaimResponse, astTask *ast.Task, structDefs map[string]*ast.StructDef) {
	// Use inputs from claim (API delivery). Fall back to inputs.json for backward compat.
	var jsonInputs map[string]interface{}
	if claim.Inputs != nil {
		jsonInputs = claim.Inputs
	} else {
		// Backward compat: read inputs.json from the task directory.
		inputsPath := filepath.Join(claim.InputsDir, "inputs.json")
		inputsData, err := os.ReadFile(inputsPath)
		if err != nil {
			log.Printf("daemon: read inputs.json task_id=%s: %v", claim.TaskID, err)
			d.reportResult(taskCtx, claim.TaskID, "FAILED", fmt.Sprintf("read inputs.json: %v", err))
			return
		}
		if err := json.Unmarshal(inputsData, &jsonInputs); err != nil {
			log.Printf("daemon: parse inputs.json task_id=%s: %v", claim.TaskID, err)
			d.reportResult(taskCtx, claim.TaskID, "FAILED", fmt.Sprintf("parse inputs.json: %v", err))
			return
		}
	}

	// Resolve relative file paths in inputs against the WDL file's parent directory.
	wdlDir := filepath.Dir(claim.WDLPath)

	// Execute the task via Docker.
	outputs, err := task.RunTask(taskCtx, astTask, jsonInputs, task.NewDockerRunner(), claim.OutputsDir, wdlDir, structDefs)
	if err != nil {
		log.Printf("daemon: RunTask task_id=%s: %v", claim.TaskID, err)
		d.reportResult(taskCtx, claim.TaskID, "FAILED", fmt.Sprintf("run task: %v", err))
		return
	}

	// Serialize outputs to JSON.
	jsonOutputs, err := values.ValuesToJSON(outputs)
	if err != nil {
		log.Printf("daemon: valuesToJSON task_id=%s: %v", claim.TaskID, err)
		d.reportResult(taskCtx, claim.TaskID, "FAILED", fmt.Sprintf("serialize outputs: %v", err))
		return
	}

	// Write outputs.json for backward compatibility with local file readers.
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
	// Report with outputs so server can store them in DB.
	d.reportResultWithOutputs(taskCtx, claim.TaskID, "COMPLETED", "", jsonOutputs)
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

// reportResult sends POST /tasks/{taskID}/result to the server without outputs.
// Errors are logged but not returned — task state will be recovered via
// stale detection if the report fails.
func (d *Daemon) reportResult(ctx context.Context, taskID, state, errMsg string) {
	d.reportResultWithOutputs(ctx, taskID, state, errMsg, nil)
}

