package workflow

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/eval"
	"github.com/bioflowy/flowy/wdl/task"
	"github.com/bioflowy/flowy/wdl/types"
	"github.com/bioflowy/flowy/wdl/values"
)

// runTaskFn is the package-level variable that points to the task execution function.
// Tests override this to bypass container execution and return predefined outputs.
var runTaskFn = task.RunTask

// completionMsg carries the result of a dispatched DAG node when it finishes.
// outputs maps call alias names to their output StructValues (or ArrayValue for scatter).
// Each entry is bound directly into the workflow env: env.Define(alias, structOrArray).
type completionMsg struct {
	nodeID  string
	outputs map[string]values.Value // alias → StructValue or ArrayValue
	err     error
}

// scheduler executes a WDL workflow body in DAG (topological) order.
// It uses a single-writer pattern: only the scheduler goroutine writes to env,
// while concurrent task executions communicate results via a buffered channel.
type scheduler struct {
	doc        *ast.Document
	taskIndex  map[string]*ast.Task
	runner     task.ContainerRunner
	execRoot   string
	env        *eval.Env           // workflow-level environment (call outputs bound here)
	nodes      map[string]*dagNode // from buildDAG
	order      []string            // topological order from topoSort
	done       map[string]bool     // tracks completed node IDs
	taskRunner func(ctx context.Context, t *ast.Task, inputs map[string]interface{}, runner task.ContainerRunner, execRoot string) (map[string]values.Value, error)
}

// run executes the scheduler in topological order.
//
// Algorithm:
//  1. Process nodes in topo order.
//  2. For each node, wait until all deps are done (drain resultCh if needed).
//  3. Dispatch node: Decls run synchronously; Calls/Scatter/Conditional via errgroup goroutines.
//  4. After all nodes dispatched, drain any remaining running goroutines.
func (s *scheduler) run(ctx context.Context) error {
	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(8) // global concurrency limit for scatter parallelism

	// Buffered channel prevents goroutine leaks if the scheduler stops early.
	resultCh := make(chan completionMsg, len(s.nodes))

	running := make(map[string]bool)

	// handleCompletion processes a finished node's result.
	handleCompletion := func(msg completionMsg) error {
		if msg.err != nil {
			return fmt.Errorf("node %q failed: %w", msg.nodeID, msg.err)
		}
		// Bind outputs into workflow env: each key is a call alias (or wrapped alias),
		// each value is a StructValue (for calls) or ArrayValue (for scatter outputs).
		for alias, v := range msg.outputs {
			s.env.Define(alias, v)
		}
		s.done[msg.nodeID] = true
		delete(running, msg.nodeID)
		return nil
	}

	// allDepsDone returns true if all declared dependencies of node are in s.done.
	allDepsDone := func(node *dagNode) bool {
		for _, dep := range node.deps {
			if !s.done[dep] {
				return false
			}
		}
		return true
	}

	for _, id := range s.order {
		node := s.nodes[id]

		// Wait until all deps are done.
		for !allDepsDone(node) {
			msg := <-resultCh
			if err := handleCompletion(msg); err != nil {
				// Wait for remaining goroutines before returning.
				_ = g.Wait()
				return err
			}
		}

		// Dispatch the node.
		switch node.node.(type) {
		case *ast.Decl:
			// Decls are synchronous — evaluate and bind immediately.
			decl := node.node.(*ast.Decl)
			v, err := eval.Eval(decl.Expr, s.env, eval.ExecContext{})
			if err != nil {
				_ = g.Wait()
				return fmt.Errorf("decl %q: %w", decl.Name, err)
			}
			s.env.Define(decl.Name, v)
			s.done[id] = true

		default:
			// Async dispatch: copy id and node for closure capture.
			capturedID := id
			capturedNode := node
			running[id] = true

			g.Go(func() error {
				outputs, err := s.dispatch(gCtx, capturedNode)
				resultCh <- completionMsg{
					nodeID:  capturedID,
					outputs: outputs,
					err:     err,
				}
				return err
			})
		}
	}

	// Drain remaining running nodes.
	for len(running) > 0 {
		msg := <-resultCh
		if err := handleCompletion(msg); err != nil {
			_ = g.Wait()
			return err
		}
	}

	// Wait for errgroup to finalize (catches any panics or context errors).
	return g.Wait()
}

// dispatch executes a single DAG node and returns its bound outputs.
// Outputs are keyed by call alias; each value is a StructValue (for calls) or
// an ArrayValue of StructValues (for scatter), etc.
func (s *scheduler) dispatch(ctx context.Context, node *dagNode) (map[string]values.Value, error) {
	switch n := node.node.(type) {
	case *ast.Call:
		return s.runCall(ctx, n)

	case *ast.Scatter:
		g, gCtx := errgroup.WithContext(ctx)
		g.SetLimit(8)
		result, err := s.runScatter(gCtx, g, n, s.env)
		if err != nil {
			return nil, err
		}
		// Wait for errgroup completion to ensure goroutines are cleaned up.
		if waitErr := g.Wait(); waitErr != nil && err == nil {
			return nil, waitErr
		}
		return result, nil

	case *ast.Conditional:
		return s.runConditional(ctx, n)

	case *ast.Decl:
		// Decls are handled synchronously in run(); this should not be reached.
		return nil, fmt.Errorf("unexpected Decl in dispatch: %q", n.Name)

	default:
		return nil, fmt.Errorf("unknown workflow node type %T", node.node)
	}
}

// runCall executes a single call statement: resolves the callee task, evaluates
// inputs, runs the task, and returns outputs as {alias: StructValue{rawOutputs}}.
//
// The StructValue binding allows downstream GetExpr (alias.outputName) to work
// via the eval package's field access on StructValue.
func (s *scheduler) runCall(ctx context.Context, call *ast.Call) (map[string]values.Value, error) {
	alias := callID(call)

	// Resolve the callee task.
	taskName := call.Callee[len(call.Callee)-1]
	t, ok := s.taskIndex[taskName]
	if !ok {
		return nil, fmt.Errorf("task %q not found", taskName)
	}

	// Evaluate call inputs.
	inputs, err := evalCallInputs(call, s.env)
	if err != nil {
		return nil, fmt.Errorf("call %q input evaluation: %w", alias, err)
	}

	// Execute the task via the scheduler's task runner (default: runTaskFn).
	fn := s.taskRunner
	if fn == nil {
		fn = runTaskFn
	}
	rawOutputs, err := fn(ctx, t, inputs, s.runner, s.execRoot)
	if err != nil {
		return nil, fmt.Errorf("call %q execution: %w", alias, err)
	}

	// Wrap outputs in a StructValue keyed by alias.
	// This enables `alias.outputName` GetExpr evaluation in downstream calls.
	structVal := values.StructValue{
		Type:   types.StructType{Name: alias},
		Fields: rawOutputs,
	}
	return map[string]values.Value{alias: structVal}, nil
}
