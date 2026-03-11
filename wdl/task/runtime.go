package task

import (
	"fmt"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/eval"
	"github.com/bioflowy/flowy/wdl/values"
)

// ReturnCodesSpec describes acceptable exit codes for a task.
// If AllCodes is true, any exit code is acceptable.
// Otherwise, Codes lists the specific acceptable exit codes.
type ReturnCodesSpec struct {
	AllCodes bool    // true means "*" (any exit code is acceptable)
	Codes    []int64 // specific acceptable codes (default: [0])
}

// DockerConfig holds the parsed container execution parameters extracted from a
// WDL task's runtime or requirements section.
type DockerConfig struct {
	Image       string          // Docker image reference (e.g., "ubuntu:22.04")
	Memory      int64           // Memory limit in bytes (0 = no limit)
	NanoCPUs    int64           // CPU limit in nanocpus (0 = no limit)
	ReturnCodes ReturnCodesSpec // acceptable exit codes (default: only 0)
}

// ParseRuntime evaluates the WDL runtime/requirements section of a task and
// returns a DockerConfig with the resolved image, memory, and CPU values.
//
// Key rules:
//   - task.Requirements is checked first (WDL 1.2 preferred); falls back to task.Runtime
//   - For image: "container" key checked before "docker" key; default is "ubuntu:latest"
//   - For cpu: converted to NanoCPUs by multiplying by 1e9
//   - For memory: parsed with ParseMemoryString
//
// Expressions are evaluated using eval.Eval with an empty ExecContext, since
// runtime expressions must not reference task outputs or execution paths.
func ParseRuntime(task *ast.Task, env *eval.Env) (DockerConfig, error) {
	cfg := DockerConfig{
		Image: "ubuntu:latest", // WDL default
	}
	ctx := eval.ExecContext{} // empty: runtime exprs cannot reference task outputs

	// Merge Requirements (precedence) and Runtime into a single lookup map.
	// We process Requirements first, then fall through to Runtime for keys not found.
	lookup := func(key string) (ast.Expr, bool) {
		if task.Requirements != nil {
			if expr, ok := task.Requirements[key]; ok {
				return expr, true
			}
		}
		if task.Runtime != nil {
			if expr, ok := task.Runtime[key]; ok {
				return expr, true
			}
		}
		return nil, false
	}

	// Resolve container image: "container" key takes precedence over "docker"
	for _, key := range []string{"container", "docker"} {
		if expr, ok := lookup(key); ok {
			v, err := eval.Eval(expr, env, ctx)
			if err != nil {
				return cfg, fmt.Errorf("parseRuntime: evaluating %q: %w", key, err)
			}
			switch cv := v.(type) {
			case values.StringValue:
				cfg.Image = cv.Value
			case values.ArrayValue:
				if len(cv.Items) > 0 {
					if sv, ok := cv.Items[0].(values.StringValue); ok {
						cfg.Image = sv.Value
					} else {
						return cfg, fmt.Errorf("parseRuntime: %q array items must be String, got %T", key, cv.Items[0])
					}
				}
			default:
				return cfg, fmt.Errorf("parseRuntime: %q must evaluate to String or Array[String], got %T", key, v)
			}
			break // use the first key found
		}
	}

	// Resolve cpu -> NanoCPUs
	if expr, ok := lookup("cpu"); ok {
		v, err := eval.Eval(expr, env, ctx)
		if err != nil {
			return cfg, fmt.Errorf("parseRuntime: evaluating 'cpu': %w", err)
		}
		switch cv := v.(type) {
		case values.IntValue:
			cfg.NanoCPUs = cv.Value * 1_000_000_000
		case values.FloatValue:
			cfg.NanoCPUs = int64(cv.Value * 1_000_000_000)
		default:
			return cfg, fmt.Errorf("parseRuntime: 'cpu' must evaluate to Int or Float, got %T", v)
		}
	}

	// Resolve memory -> bytes
	if expr, ok := lookup("memory"); ok {
		v, err := eval.Eval(expr, env, ctx)
		if err != nil {
			return cfg, fmt.Errorf("parseRuntime: evaluating 'memory': %w", err)
		}
		var memStr string
		switch mv := v.(type) {
		case values.StringValue:
			memStr = mv.Value
		case values.IntValue:
			// bare integer already in bytes
			cfg.Memory = mv.Value
			memStr = "" // skip ParseMemoryString
		default:
			return cfg, fmt.Errorf("parseRuntime: 'memory' must evaluate to String or Int, got %T", v)
		}
		if memStr != "" {
			bytes, err := ParseMemoryString(memStr)
			if err != nil {
				return cfg, fmt.Errorf("parseRuntime: parsing memory %q: %w", memStr, err)
			}
			cfg.Memory = bytes
		}
	}

	// Resolve return_codes -> ReturnCodes
	if expr, ok := lookup("return_codes"); ok {
		v, err := eval.Eval(expr, env, ctx)
		if err != nil {
			return cfg, fmt.Errorf("parseRuntime: evaluating 'return_codes': %w", err)
		}
		switch rv := v.(type) {
		case values.StringValue:
			if rv.Value == "*" {
				cfg.ReturnCodes = ReturnCodesSpec{AllCodes: true}
			} else {
				return cfg, fmt.Errorf("parseRuntime: 'return_codes' string must be \"*\", got %q", rv.Value)
			}
		case values.IntValue:
			cfg.ReturnCodes = ReturnCodesSpec{Codes: []int64{rv.Value}}
		case values.ArrayValue:
			codes := make([]int64, 0, len(rv.Items))
			for _, item := range rv.Items {
				iv, ok := item.(values.IntValue)
				if !ok {
					return cfg, fmt.Errorf("parseRuntime: 'return_codes' array must contain Int values, got %T", item)
				}
				codes = append(codes, iv.Value)
			}
			cfg.ReturnCodes = ReturnCodesSpec{Codes: codes}
		default:
			return cfg, fmt.Errorf("parseRuntime: 'return_codes' must evaluate to Int, Array[Int], or \"*\", got %T", v)
		}
	}

	return cfg, nil
}
