// Package workflow provides the WDL workflow execution engine.
// It builds a DAG of task dependencies, performs topological sort for ordering,
// and provides the RunWorkflow entry point for executing a complete WDL workflow.
package workflow

import (
	"context"
	"fmt"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/eval"
	"github.com/bioflowy/flowy/wdl/task"
	"github.com/bioflowy/flowy/wdl/values"
)

// RunWorkflow is the public entry point for executing a WDL workflow.
// It looks up the named workflow in doc, builds the execution DAG,
// binds inputs, and executes tasks in dependency order.
//
// Steps:
//  1. Find the workflow by name in doc.Workflows.
//  2. Build the task index from doc.Tasks.
//  3. Bind workflow-level inputs from jsonInputs.
//  4. Build the DAG and compute topological order.
//  5. Create a scheduler and run it.
//  6. Evaluate workflow output declarations and return them.
func RunWorkflow(
	ctx context.Context,
	doc *ast.Document,
	wfName string,
	jsonInputs map[string]interface{},
	runner task.ContainerRunner,
	execRoot string,
) (map[string]values.Value, error) {
	// Find the workflow definition.
	var wf *ast.Workflow
	for _, w := range doc.Workflows {
		if w.Name == wfName {
			wf = w
			break
		}
	}
	if wf == nil {
		return nil, fmt.Errorf("workflow %q not found in document", wfName)
	}

	// Build task index for fast lookup.
	taskIndex := buildTaskIndex(doc)

	// Bind workflow-level inputs.
	env, err := bindWorkflowInputs(wf, jsonInputs)
	if err != nil {
		return nil, fmt.Errorf("binding workflow inputs: %w", err)
	}

	// Build the execution DAG (includes only top-level non-Decl nodes).
	dagNodes, err := buildDAG(wf.Body)
	if err != nil {
		return nil, fmt.Errorf("building DAG: %w", err)
	}

	// Topological sort.
	order, err := topoSort(dagNodes)
	if err != nil {
		return nil, fmt.Errorf("topological sort: %w", err)
	}

	// Include Decl nodes in the scheduler's node map under their own synthetic IDs.
	// We handle Decl nodes directly in the scheduler by detecting *ast.Decl in the body
	// and processing them in-order relative to other nodes. However, since buildDAG
	// skips Decls, we need to process them inline.
	//
	// Strategy: augment the order to include Decl nodes in their original body position.
	// We rebuild order to include decl sentinel IDs and add them to dagNodes.
	order, dagNodes = injectDeclNodes(wf.Body, order, dagNodes)

	// Create and run the scheduler.
	s := &scheduler{
		doc:       doc,
		taskIndex: taskIndex,
		runner:    runner,
		execRoot:  execRoot,
		env:       env,
		nodes:     dagNodes,
		order:     order,
		done:      make(map[string]bool),
	}

	if err := s.run(ctx); err != nil {
		return nil, fmt.Errorf("workflow execution: %w", err)
	}

	// Evaluate workflow output declarations.
	outputs := make(map[string]values.Value, len(wf.Outputs))
	for _, outDecl := range wf.Outputs {
		v, err := eval.Eval(outDecl.Expr, env, eval.ExecContext{})
		if err != nil {
			return nil, fmt.Errorf("evaluating workflow output %q: %w", outDecl.Name, err)
		}
		outputs[outDecl.Name] = v
	}

	return outputs, nil
}

// injectDeclNodes inserts Decl nodes from the workflow body into the execution order
// at their correct positions relative to other nodes. Since topoSort only covers
// non-Decl nodes, we re-merge the Decls back in their original body order.
//
// Each Decl gets a synthetic ID "decl_N" (N = position in body). Decl nodes have no
// DAG dependencies tracked since they are evaluated synchronously before any downstream node.
func injectDeclNodes(body []ast.WorkflowNode, order []string, nodes map[string]*dagNode) ([]string, map[string]*dagNode) {
	// Build a set of already-ordered node IDs (non-Decl) for position lookup.
	orderSet := make(map[string]bool, len(order))
	for _, id := range order {
		orderSet[id] = true
	}

	// We need to interleave Decl nodes at the correct positions. The simplest correct
	// approach: reconstruct the full order by walking body in original order.
	// For non-Decl nodes, use the topo order (which may reorder them for parallelism).
	// For Decl nodes, insert them at their original body position.
	//
	// However, Decls can only be used by nodes that appear AFTER them in body order.
	// Since buildDAG excludes Decls from dependency tracking, we insert each Decl
	// immediately before the first non-Decl node that appears after it in body order.
	//
	// Simplest correct approach: build a "position in body" map, then rebuild order
	// by visiting body in body order, and for each Decl emit it now; for non-Decl
	// nodes, defer to the topo order.

	// Map from topo-ordered non-Decl node ID → its body position.
	bodyPos := make(map[string]int, len(body))
	declNodes := make([]*ast.Decl, 0)
	declBodyIdx := make([]int, 0)

	scatter_n := 0
	if_n := 0
	for i, item := range body {
		switch n := item.(type) {
		case *ast.Call:
			bodyPos[callID(n)] = i
		case *ast.Scatter:
			scatter_n++
			bodyPos[scatterID(n, scatter_n)] = i
		case *ast.Conditional:
			if_n++
			bodyPos[conditionalID(n, if_n)] = i
		case *ast.Decl:
			declNodes = append(declNodes, n)
			declBodyIdx = append(declBodyIdx, i)
		}
	}

	if len(declNodes) == 0 {
		// No Decls: return original order unchanged.
		return order, nodes
	}

	// Build synthetic decl node IDs and add them to the nodes map.
	// ID format: "decl_N" where N is position in body (0-based).
	declIDs := make([]string, len(declNodes))
	for i, d := range declNodes {
		id := "decl_" + itoa(declBodyIdx[i])
		declIDs[i] = id
		nodes[id] = &dagNode{id: id, node: d}
	}

	// Rebuild the merged order:
	// Walk through body positions in increasing order. At each decl position, emit the decl.
	// For non-Decl nodes: they appear in topo order, but we need to respect that each
	// non-Decl node must come AFTER all Decls that precede it in body order.
	//
	// Strategy: two-pointer merge.
	// - nextDeclPtr: index into declBodyIdx/declIDs for the next decl to emit.
	// - For each item in topo order, first flush any decls with bodyPos < this item's bodyPos.

	newOrder := make([]string, 0, len(order)+len(declNodes))
	nextDeclPtr := 0

	for _, id := range order {
		// Flush decls that precede this node in body order.
		pos := bodyPos[id]
		for nextDeclPtr < len(declBodyIdx) && declBodyIdx[nextDeclPtr] < pos {
			newOrder = append(newOrder, declIDs[nextDeclPtr])
			nextDeclPtr++
		}
		newOrder = append(newOrder, id)
	}
	// Flush any remaining trailing decls.
	for nextDeclPtr < len(declIDs) {
		newOrder = append(newOrder, declIDs[nextDeclPtr])
		nextDeclPtr++
	}

	return newOrder, nodes
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
