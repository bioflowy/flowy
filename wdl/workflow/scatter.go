package workflow

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/eval"
	"github.com/bioflowy/flowy/wdl/types"
	"github.com/bioflowy/flowy/wdl/values"
)

// runScatter executes a scatter block:
//  1. Evaluates the scatter expression to get an ArrayValue.
//  2. For each element, spawns a goroutine via the errgroup (for concurrency limiting).
//  3. Collects per-element results in index order via a buffered channel.
//  4. Transposes per-element output maps into per-alias StructValues with ArrayValue fields.
//
// The returned map has the same key structure as executeBody: {alias → StructValue{arrays}}.
//
// NOTE: The caller is responsible for calling g.Wait() after runScatter returns to ensure
// all goroutines have completed.
func (s *scheduler) runScatter(ctx context.Context, g *errgroup.Group, scatter *ast.Scatter, env *eval.Env) (map[string]values.Value, error) {
	// Evaluate scatter collection expression.
	arrVal, err := eval.Eval(scatter.Expr, env, eval.ExecContext{})
	if err != nil {
		return nil, fmt.Errorf("scatter expression: %w", err)
	}

	arr, ok := arrVal.(values.ArrayValue)
	if !ok {
		return nil, fmt.Errorf("scatter expression must produce Array, got %T", arrVal)
	}

	if len(arr.Items) == 0 {
		// Empty scatter: return empty ArrayValues for each call alias in the body.
		return emptyScatterOutputs(scatter.Body, s.taskIndex), nil
	}

	// elemResult carries the result of a single scatter element execution.
	type elemResult struct {
		idx     int
		outputs map[string]values.Value // alias → StructValue for this element
		err     error
	}

	elemCh := make(chan elemResult, len(arr.Items))

	for i, item := range arr.Items {
		i, item := i, item // capture loop vars
		g.Go(func() error {
			// Each scatter element gets its own child env with the loop variable bound.
			scatterEnv := eval.NewEnv(env)
			scatterEnv.Define(scatter.Variable, item)

			child := s.childScheduler(scatterEnv)
			outputs, execErr := child.executeBody(ctx, scatter.Body, scatterEnv)
			elemCh <- elemResult{idx: i, outputs: outputs, err: execErr}
			return execErr
		})
	}

	// Collect all element results, preserving index order.
	results := make([]map[string]values.Value, len(arr.Items))
	for range arr.Items {
		select {
		case r := <-elemCh:
			if r.err != nil {
				return nil, r.err
			}
			results[r.idx] = r.outputs
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Transpose: per-element {alias → StructValue} → {alias → StructValue{field → Array}}.
	return transposeScatterResults(results), nil
}

// transposeScatterResults converts a per-element list of output maps into a single
// output map where each alias maps to a StructValue whose fields contain ArrayValues.
//
// Input:  results[i] = {"d": StructValue{Fields: {"out": IntValue(n)}}}
// Output: {"d": StructValue{Fields: {"out": ArrayValue{[IntValue(0), IntValue(1), ...]}}}}
//
// For nested scatter (alias → StructValue{field → ArrayValue}), this produces
// Array[Array[T]] by collecting the per-element ArrayValues into a new ArrayValue.
func transposeScatterResults(results []map[string]values.Value) map[string]values.Value {
	if len(results) == 0 {
		return map[string]values.Value{}
	}

	// Gather all alias keys from all results (handles Optional outputs).
	keySet := make(map[string]bool)
	for _, r := range results {
		for k := range r {
			keySet[k] = true
		}
	}

	transposed := make(map[string]values.Value, len(keySet))

	for alias := range keySet {
		// For StructValue aliases: collect each element's StructValue, then transpose fields.
		// For NullValue aliases (false conditional branches): collect NullValues into an ArrayValue.

		// Check the type of the first non-nil entry.
		var firstVal values.Value
		for _, r := range results {
			if v, ok := r[alias]; ok {
				firstVal = v
				break
			}
		}

		switch firstVal.(type) {
		case values.StructValue:
			// Collect per-element structs and transpose fields into arrays.
			transposed[alias] = transposeStructArray(alias, results)

		case values.NullValue:
			// All elements are NullValue (false conditional in scatter) — ArrayValue of NullValues.
			items := make([]values.Value, len(results))
			for i, r := range results {
				if v, ok := r[alias]; ok {
					items[i] = v
				} else {
					items[i] = values.NullValue{}
				}
			}
			transposed[alias] = values.ArrayValue{
				Type:  types.ArrayType{ItemType: types.AnyType{}},
				Items: items,
			}

		default:
			// Unexpected type: collect as generic ArrayValue.
			items := make([]values.Value, len(results))
			for i, r := range results {
				if v, ok := r[alias]; ok {
					items[i] = v
				} else {
					items[i] = values.NullValue{}
				}
			}
			transposed[alias] = values.ArrayValue{
				Type:  types.ArrayType{ItemType: types.AnyType{}},
				Items: items,
			}
		}
	}

	return transposed
}

// transposeStructArray collects the StructValue for alias from each result element,
// then produces a new StructValue where each field is an ArrayValue of all elements' field values.
func transposeStructArray(alias string, results []map[string]values.Value) values.StructValue {
	// Gather all field names across all result structs for this alias.
	fieldSet := make(map[string]bool)
	for _, r := range results {
		if sv, ok := r[alias].(values.StructValue); ok {
			for fname := range sv.Fields {
				fieldSet[fname] = true
			}
		}
	}

	fields := make(map[string]values.Value, len(fieldSet))
	for fieldName := range fieldSet {
		items := make([]values.Value, len(results))
		for i, r := range results {
			if sv, ok := r[alias].(values.StructValue); ok {
				if fv, ok := sv.Fields[fieldName]; ok {
					items[i] = fv
				} else {
					items[i] = values.NullValue{}
				}
			} else {
				// Element had NullValue for this alias (false conditional).
				items[i] = values.NullValue{}
			}
		}
		var itemType types.Type = types.AnyType{}
		for _, item := range items {
			if item != nil {
				if tv, ok := item.(interface{ ValueType() types.Type }); ok {
					t := tv.ValueType()
					if _, isAny := t.(types.AnyType); !isAny {
						itemType = t
						break
					}
				}
			}
		}
		fields[fieldName] = values.ArrayValue{
			Type:  types.ArrayType{ItemType: itemType},
			Items: items,
		}
	}

	return values.StructValue{
		Type:   types.StructType{Name: alias},
		Fields: fields,
	}
}

// emptyScatterOutputs returns StructValues with empty ArrayValue fields for all call aliases in body.
// Called when the scatter collection is empty.
func emptyScatterOutputs(body []ast.WorkflowNode, taskIndex map[string]*ast.Task) map[string]values.Value {
	result := make(map[string]values.Value)
	collectEmptyOutputs(body, taskIndex, result)
	return result
}

// collectEmptyOutputs recursively walks body nodes to find all call aliases
// and binds each as a StructValue with empty ArrayValue fields in result.
func collectEmptyOutputs(body []ast.WorkflowNode, taskIndex map[string]*ast.Task, result map[string]values.Value) {
	for _, node := range body {
		switch n := node.(type) {
		case *ast.Call:
			alias := callID(n)
			taskName := n.Callee[len(n.Callee)-1]
			if t, ok := taskIndex[taskName]; ok {
				fields := make(map[string]values.Value, len(t.Outputs))
				for _, outDecl := range t.Outputs {
					fields[outDecl.Name] = values.ArrayValue{
						Type:  types.ArrayType{ItemType: types.AnyType{}},
						Items: nil,
					}
				}
				result[alias] = values.StructValue{
					Type:   types.StructType{Name: alias},
					Fields: fields,
				}
			}
		case *ast.Scatter:
			collectEmptyOutputs(n.Body, taskIndex, result)
		case *ast.Conditional:
			collectEmptyOutputs(n.Body, taskIndex, result)
		}
	}
}

// collectScatterOutputNames recursively walks body nodes to find all call output names.
// Returns a map of alias → []fieldNames for use in false-branch NullValue binding.
func collectScatterOutputNames(body []ast.WorkflowNode, taskIndex map[string]*ast.Task) map[string][]string {
	result := make(map[string][]string)
	for _, node := range body {
		switch n := node.(type) {
		case *ast.Call:
			alias := callID(n)
			taskName := n.Callee[len(n.Callee)-1]
			if t, ok := taskIndex[taskName]; ok {
				names := make([]string, len(t.Outputs))
				for i, outDecl := range t.Outputs {
					names[i] = outDecl.Name
				}
				result[alias] = names
			}
		case *ast.Scatter:
			for k, v := range collectScatterOutputNames(n.Body, taskIndex) {
				result[k] = v
			}
		case *ast.Conditional:
			for k, v := range collectScatterOutputNames(n.Body, taskIndex) {
				result[k] = v
			}
		}
	}
	return result
}

// formatErr wraps an error with scatter context information.
func formatErr(msg string, err error) error {
	return fmt.Errorf("%s: %w", msg, err)
}
