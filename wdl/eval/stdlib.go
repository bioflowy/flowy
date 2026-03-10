package eval

import (
	"fmt"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/values"
)

// stdlibFn is the type of all WDL standard library function implementations.
// args is the list of already-evaluated argument values; ctx is the execution context.
type stdlibFn func(args []values.Value, ctx ExecContext) (values.Value, error)

// stdlibFuncs is the dispatch map from WDL function name to implementation.
// Plans 04-04 through 04-06 register implementations here.
var stdlibFuncs = map[string]stdlibFn{}

// evalApply evaluates a function-call expression by dispatching to stdlibFuncs.
// It evaluates each argument left-to-right before calling the function.
func evalApply(e *ast.ApplyExpr, env *Env, ctx ExecContext) (values.Value, error) {
	fn, ok := stdlibFuncs[e.Name]
	if !ok {
		return nil, &EvalError{Pos: e.Pos, Message: fmt.Sprintf("unknown function %q", e.Name)}
	}
	args := make([]values.Value, len(e.Args))
	for i, a := range e.Args {
		v, err := Eval(a, env, ctx)
		if err != nil {
			return nil, err
		}
		args[i] = v
	}
	return fn(args, ctx)
}
