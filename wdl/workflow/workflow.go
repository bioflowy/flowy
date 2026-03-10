// Package workflow provides the WDL workflow execution engine.
// It builds a DAG of task dependencies, performs topological sort for ordering,
// and provides the RunWorkflow entry point for executing a complete WDL workflow.
package workflow

import (
	"context"
	"fmt"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/task"
	"github.com/bioflowy/flowy/wdl/values"
)

// RunWorkflow is the public entry point for executing a WDL workflow.
// It looks up the named workflow in doc, builds the execution DAG,
// binds inputs, and executes tasks in dependency order.
//
// Currently a stub — full implementation is provided by Plan 02.
func RunWorkflow(
	ctx context.Context,
	doc *ast.Document,
	wfName string,
	jsonInputs map[string]interface{},
	runner task.ContainerRunner,
	execRoot string,
) (map[string]values.Value, error) {
	return nil, fmt.Errorf("not yet implemented")
}

// buildTaskIndex constructs a map from task name to *ast.Task for fast lookup
// during workflow execution when dispatching call statements.
func buildTaskIndex(doc *ast.Document) map[string]*ast.Task {
	index := make(map[string]*ast.Task, len(doc.Tasks))
	for _, t := range doc.Tasks {
		index[t.Name] = t
	}
	return index
}
