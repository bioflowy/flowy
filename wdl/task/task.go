package task

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/eval"
	"github.com/bioflowy/flowy/wdl/values"
)

// TaskError is returned when a task's container exits with a non-zero status,
// or when the container runner encounters a fatal error.
type TaskError struct {
	TaskName string
	ExitCode int
	Stderr   []byte
}

// Error implements the error interface. The message includes the task name,
// exit code, and the last lines of stderr for diagnostic purposes.
func (e *TaskError) Error() string {
	return fmt.Sprintf("task %q failed with exit code %d: %s", e.TaskName, e.ExitCode, string(e.Stderr))
}

// RunTask is the public entry point for executing a WDL task. It orchestrates the
// full pipeline:
//
//  1. Create execution directory layout: execRoot/taskName/{work/, stdout.txt, stderr.txt}
//  2. Create a PathMap and bind all input files/directories
//  3. Bind JSON inputs to a typed env via BindInputs
//  4. Parse runtime/requirements for Docker image, memory, cpu
//  5. Evaluate private declarations (task.Decls) into env
//  6. Expand command template and write script to execDir/command
//  7. Construct RunRequest and call runner.Run()
//  8. Write stdout.txt, stderr.txt, rc files to execDir
//  9. Return *TaskError if exit code is non-zero
//  10. Collect and return output values via CollectOutputs
//
// The runner parameter accepts any ContainerRunner implementation, enabling
// testing with ShellRunner without a live Docker daemon.
func RunTask(ctx context.Context, task *ast.Task, jsonInputs map[string]interface{}, runner ContainerRunner, execRoot string) (map[string]values.Value, error) {
	// Step 1: create execution directory layout.
	execDir := filepath.Join(execRoot, task.Name)
	workDir := filepath.Join(execDir, "work")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("creating work dir for task %q: %w", task.Name, err)
	}

	// Step 2: create a PathMap for file/directory path translation.
	pm := NewPathMap()

	// Step 3: bind inputs — converts JSON values to typed WDL values and
	// registers all File/Directory paths in the PathMap.
	env, envVars, err := BindInputs(task, jsonInputs, pm)
	if err != nil {
		return nil, fmt.Errorf("binding inputs for task %q: %w", task.Name, err)
	}

	// Step 4: parse runtime/requirements for image, memory, cpu.
	config, err := ParseRuntime(task, env)
	if err != nil {
		return nil, fmt.Errorf("parsing runtime for task %q: %w", task.Name, err)
	}

	// Step 5 + 6: BuildCommand evaluates private decls and writes the command script.
	commandPath, err := BuildCommand(task, env, pm, execDir)
	if err != nil {
		return nil, fmt.Errorf("building command for task %q: %w", task.Name, err)
	}

	// Step 7: construct binds list.
	// Start with file/directory input binds from the PathMap.
	binds := pm.Binds()
	// Add the command script bind: host script -> container read-only path
	binds = append(binds, commandPath+":/mnt/inputs/_command:ro")
	// Add the work dir bind: host work dir -> container /mnt/outputs read-write
	binds = append(binds, workDir+":/mnt/outputs:rw")

	req := RunRequest{
		Image:    config.Image,
		Command:  []string{"bash", "/mnt/inputs/_command"},
		Binds:    binds,
		Env:      envVars,
		Memory:   config.Memory,
		NanoCPUs: config.NanoCPUs,
		WorkDir:  "/mnt/outputs",
	}

	// Run the container.
	result, err := runner.Run(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("running task %q: %w", task.Name, err)
	}

	// Step 8: write stdout.txt, stderr.txt, rc files.
	stdoutPath := filepath.Join(execDir, "stdout.txt")
	stderrPath := filepath.Join(execDir, "stderr.txt")
	rcPath := filepath.Join(execDir, "rc")

	if writeErr := os.WriteFile(stdoutPath, result.Stdout, 0644); writeErr != nil {
		return nil, fmt.Errorf("writing stdout for task %q: %w", task.Name, writeErr)
	}
	if writeErr := os.WriteFile(stderrPath, result.Stderr, 0644); writeErr != nil {
		return nil, fmt.Errorf("writing stderr for task %q: %w", task.Name, writeErr)
	}
	if writeErr := os.WriteFile(rcPath, []byte(strconv.Itoa(result.ExitCode)), 0644); writeErr != nil {
		return nil, fmt.Errorf("writing rc for task %q: %w", task.Name, writeErr)
	}

	// Step 9: return TaskError if non-zero exit.
	if result.ExitCode != 0 {
		return nil, &TaskError{
			TaskName: task.Name,
			ExitCode: result.ExitCode,
			Stderr:   result.Stderr,
		}
	}

	// Step 10: collect and return output values.
	execCtx := eval.ExecContext{
		WorkDir:    workDir,
		StdoutPath: stdoutPath,
		StderrPath: stderrPath,
	}
	outputs, err := CollectOutputs(task, env, execCtx, pm)
	if err != nil {
		return nil, fmt.Errorf("collecting outputs for task %q: %w", task.Name, err)
	}

	return outputs, nil
}
