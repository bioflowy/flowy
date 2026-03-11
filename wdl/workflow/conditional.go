package workflow

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/eval"
	"github.com/bioflowy/flowy/wdl/values"
)

// runConditional evaluates an if statement:
//   - If the condition is true: executes the body and returns call outputs keyed by alias.
//   - If the condition is false: returns NullValue for each call alias in the body.
//
// The env is the enclosing scope (workflow-level or scatter-element env).
func (s *scheduler) runConditional(ctx context.Context, cond *ast.Conditional) (map[string]values.Value, error) {
	condVal, err := eval.Eval(cond.Expr, s.env, eval.ExecContext{})
	if err != nil {
		return nil, fmt.Errorf("conditional expression: %w", err)
	}

	bv, ok := condVal.(values.BooleanValue)
	if !ok {
		return nil, fmt.Errorf("conditional expression must evaluate to Boolean, got %T", condVal)
	}

	if bv.Value {
		// True branch: execute the body and collect outputs.
		return s.executeBody(ctx, cond.Body, s.env)
	}

	// False branch: bind NullValue for each call alias that would have been produced.
	return s.falseBodyOutputs(cond.Body), nil
}

// falseBodyOutputs returns a map of NullValue bindings for all call aliases in body.
// NullValue represents the "None" state for Optional outputs when a conditional is false.
func (s *scheduler) falseBodyOutputs(body []ast.WorkflowNode) map[string]values.Value {
	result := make(map[string]values.Value)
	s.collectFalseAliases(body, result)
	return result
}

// collectFalseAliases recursively walks body nodes to find all call aliases,
// binding each as NullValue in result.
func (s *scheduler) collectFalseAliases(body []ast.WorkflowNode, result map[string]values.Value) {
	for _, node := range body {
		switch n := node.(type) {
		case *ast.Call:
			alias := callID(n)
			result[alias] = values.NullValue{}
		case *ast.Scatter:
			// Scatter inside a false conditional — scatter alias becomes NullValue.
			s.collectFalseAliases(n.Body, result)
		case *ast.Conditional:
			// Nested conditional inside a false branch — inner aliases become NullValue.
			s.collectFalseAliases(n.Body, result)
		}
	}
}

// childScheduler creates a child scheduler that inherits all settings from s
// but operates with the given env. Used by executeBody and nested scatter/conditional.
func (s *scheduler) childScheduler(env *eval.Env) *scheduler {
	return &scheduler{
		doc:        s.doc,
		taskIndex:  s.taskIndex,
		runner:     s.runner,
		execRoot:   s.execRoot,
		env:        env,
		taskRunner: s.taskRunner,
	}
}

// executeBody executes a slice of workflow body nodes sequentially within the given env.
// This is used for conditional true-branch and scatter element bodies.
// Returns a merged map of all outputs produced by the body nodes, keyed by call alias.
//
// Each call output is bound as {alias: StructValue{fields}} in both the local env
// and the returned result map.
func (s *scheduler) executeBody(ctx context.Context, body []ast.WorkflowNode, env *eval.Env) (map[string]values.Value, error) {
	result := make(map[string]values.Value)
	child := s.childScheduler(env)

	for _, node := range body {
		switch n := node.(type) {
		case *ast.Call:
			outputs, err := child.runCall(ctx, n)
			if err != nil {
				return nil, err
			}
			// Bind outputs into the local env for subsequent nodes in this body.
			for alias, v := range outputs {
				env.Define(alias, v)
			}
			mergeOutputs(result, outputs)

		case *ast.Decl:
			v, err := eval.Eval(n.Expr, env, eval.ExecContext{})
			if err != nil {
				return nil, fmt.Errorf("decl %q: %w", n.Name, err)
			}
			env.Define(n.Name, v)
			result[n.Name] = v

		case *ast.Scatter:
			// Scatter within a body (nested scatter or scatter inside conditional).
			g, gCtx := errgroup.WithContext(ctx)
			g.SetLimit(8)
			outputs, err := child.runScatter(gCtx, g, n, env)
			if err != nil {
				return nil, err
			}
			// Bind scatter outputs into local env.
			for alias, v := range outputs {
				env.Define(alias, v)
			}
			mergeOutputs(result, outputs)
			// Wait for errgroup to finalize.
			if waitErr := g.Wait(); waitErr != nil && err == nil {
				return nil, waitErr
			}

		case *ast.Conditional:
			// Conditional within a body.
			outputs, err := child.runConditional(ctx, n)
			if err != nil {
				return nil, err
			}
			for alias, v := range outputs {
				env.Define(alias, v)
			}
			mergeOutputs(result, outputs)
		}
	}

	return result, nil
}

// mergeOutputs copies all entries from src into dst.
func mergeOutputs(dst, src map[string]values.Value) {
	for k, v := range src {
		dst[k] = v
	}
}
