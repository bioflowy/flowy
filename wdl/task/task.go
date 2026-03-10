package task

import (
	"fmt"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/eval"
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

// RunTask is the public entry point for executing a WDL task.
// It will be fully implemented in Plan 02 (command evaluation and output binding).
// The runner parameter accepts any ContainerRunner implementation.
func RunTask(task *ast.Task, jsonInputs map[string]interface{}, runner ContainerRunner, env *eval.Env) (map[string]interface{}, error) {
	return nil, fmt.Errorf("not yet implemented")
}
