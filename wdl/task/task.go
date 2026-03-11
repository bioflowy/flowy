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

// isAcceptableExitCode checks whether the given exit code is acceptable
// according to the return_codes specification. If no return_codes is specified
// (empty Codes and AllCodes=false), only exit code 0 is acceptable.
func isAcceptableExitCode(exitCode int, spec ReturnCodesSpec) bool {
	if spec.AllCodes {
		return true
	}
	if len(spec.Codes) == 0 {
		// Default: only 0 is acceptable.
		return exitCode == 0
	}
	for _, code := range spec.Codes {
		if int64(exitCode) == code {
			return true
		}
	}
	return false
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
func RunTask(ctx context.Context, task *ast.Task, jsonInputs map[string]interface{}, runner ContainerRunner, execRoot string, structDefs ...map[string]*ast.StructDef) (map[string]values.Value, error) {
	// Step 1: create execution directory layout.
	// Use MkdirTemp to generate a unique directory per invocation, avoiding
	// collisions when the same task is called from scatter iterations.
	execDir, err := os.MkdirTemp(execRoot, task.Name+"-")
	if err != nil {
		return nil, fmt.Errorf("creating exec dir for task %q: %w", task.Name, err)
	}
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

	// Extract struct definitions from variadic parameter.
	var sd map[string]*ast.StructDef
	if len(structDefs) > 0 {
		sd = structDefs[0]
	}

	// Step 4: evaluate private declarations (task.Decls) into env so they
	// are available for both runtime/requirements and command expansion.
	for _, decl := range task.Decls {
		if decl.Expr == nil {
			continue
		}
		v, err := eval.Eval(decl.Expr, env, eval.ExecContext{StructDefs: sd})
		if err != nil {
			return nil, fmt.Errorf("evaluating private decl %q for task %q: %w", decl.Name, task.Name, err)
		}
		env.Define(decl.Name, v)
	}

	// Step 5: parse runtime/requirements for image, memory, cpu.
	config, err := ParseRuntime(task, env)
	if err != nil {
		return nil, fmt.Errorf("parsing runtime for task %q: %w", task.Name, err)
	}

	// Step 6: BuildCommand writes the command script (private decls already evaluated).
	commandPath, err := BuildCommand(task, env, pm, execDir, workDir)
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

	// Step 9: return TaskError if exit code is not acceptable.
	// WDL 1.2 return_codes requirement allows specifying acceptable exit codes.
	if !isAcceptableExitCode(result.ExitCode, config.ReturnCodes) {
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
		StructDefs: sd,
	}
	outputs, err := CollectOutputs(task, env, execCtx, pm)
	if err != nil {
		return nil, fmt.Errorf("collecting outputs for task %q: %w", task.Name, err)
	}

	return outputs, nil
}
