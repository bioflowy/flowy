package task

import (
	"fmt"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/eval"
	"github.com/bioflowy/flowy/wdl/types"
	"github.com/bioflowy/flowy/wdl/values"
)

// CollectOutputs evaluates all output declarations in the task using the provided
// ExecContext (which carries host paths for stdout, stderr, and the work directory).
//
// Steps for each output declaration:
//  1. Evaluate decl.Expr via eval.Eval with the current outEnv and ExecContext.
//  2. Apply pm.UntranslateValue as a safety net: converts any remaining container
//     paths back to host paths (e.g., if an output expr references an input path).
//  3. Store the result in the output map keyed by decl.Name.
//  4. Define the result in outEnv so later output expressions can reference it.
//
// Returns a map[string]values.Value with all output values, keyed by output name.
func CollectOutputs(task *ast.Task, env *eval.Env, ctx eval.ExecContext, pm *PathMap) (map[string]values.Value, error) {
	// Create a child scope for output evaluation — output expressions can
	// reference earlier output declarations within the same task.
	outEnv := eval.NewEnv(env)
	result := make(map[string]values.Value, len(task.Outputs))

	for _, decl := range task.Outputs {
		if decl.Expr == nil {
			return nil, fmt.Errorf("output %q has no expression", decl.Name)
		}

		// Evaluate the output expression with the exec context.
		// ctx.WorkDir, ctx.StdoutPath, and ctx.StderrPath allow stdlib functions
		// like stdout(), stderr(), glob() to resolve against host paths.
		v, err := eval.Eval(decl.Expr, outEnv, ctx)
		if err != nil {
			return nil, fmt.Errorf("evaluating output %q: %w", decl.Name, err)
		}

		// Safety net: untranslate any remaining container paths to host paths.
		v = pm.UntranslateValue(v)

		// Coerce StringValue to FileValue/DirectoryValue when the decl type requires it.
		// Output expressions often evaluate to StringValue (e.g., string interpolation),
		// but if the declared type is File or Directory the value should be typed accordingly.
		v = coerceOutputValue(v, decl.Type)

		result[decl.Name] = v
		outEnv.Define(decl.Name, v)
	}

	return result, nil
}

// coerceOutputValue coerces a value to match the declared output type.
// WDL 1.2 allows String expressions to be assigned to File/Directory outputs —
// the string is interpreted as a path. This function performs that coercion.
func coerceOutputValue(v values.Value, declType types.Type) values.Value {
	sv, isString := v.(values.StringValue)
	if !isString {
		return v
	}
	switch declType.(type) {
	case types.FileType:
		return values.FileValue{Type: types.FileType{}, Value: sv.Value}
	case types.DirectoryType:
		return values.DirectoryValue{Type: types.DirectoryType{}, Value: sv.Value}
	default:
		return v
	}
}
