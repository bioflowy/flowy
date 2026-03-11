package task

import (
	"fmt"
	"os"
	"path/filepath"

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
		v = coerceOutputValue(v, decl.Type, ctx.WorkDir)

		result[decl.Name] = v
		outEnv.Define(decl.Name, v)
	}

	return result, nil
}

// coerceOutputValue coerces a value to match the declared output type.
// WDL 1.2 allows String expressions to be assigned to File/Directory outputs —
// the string is interpreted as a path. Relative paths are resolved against workDir.
// For optional File?/Directory? types, returns NullValue if the file doesn't exist.
func coerceOutputValue(v values.Value, declType types.Type, workDir string) values.Value {
	switch dt := declType.(type) {
	case types.FileType:
		path := resolveStringPath(v, workDir)
		if path == "" {
			return v
		}
		if dt.Optional {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				return values.NullValue{}
			}
		}
		return values.FileValue{Type: types.FileType{}, Value: path}
	case types.DirectoryType:
		path := resolveStringPath(v, workDir)
		if path == "" {
			return v
		}
		if dt.Optional {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				return values.NullValue{}
			}
		}
		return values.DirectoryValue{Type: types.DirectoryType{}, Value: path}
	case types.ArrayType:
		av, isArr := v.(values.ArrayValue)
		if !isArr {
			return v
		}
		items := make([]values.Value, len(av.Items))
		for i, item := range av.Items {
			items[i] = coerceOutputValue(item, dt.ItemType, workDir)
		}
		return values.ArrayValue{Type: av.Type, Items: items}
	default:
		return v
	}
}

// resolveStringPath extracts a file path from a StringValue, resolving relative
// paths against workDir. Returns empty string if v is not a StringValue.
func resolveStringPath(v values.Value, workDir string) string {
	sv, isString := v.(values.StringValue)
	if !isString {
		return ""
	}
	path := sv.Value
	if !filepath.IsAbs(path) && workDir != "" {
		path = filepath.Join(workDir, path)
	}
	return path
}
