// Package eval implements the WDL 1.2 expression evaluator.
// Eval() is the single entry point; it dispatches over all 14 ast.Expr concrete types.
// Wave 1 plans fill in the per-case logic; this file holds only the skeleton.
package eval

import (
	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/values"
)

// Eval evaluates a WDL expression in the given environment and execution context.
// Returns the resulting value or an EvalError describing the failure.
func Eval(expr ast.Expr, env *Env, ctx ExecContext) (values.Value, error) {
	switch e := expr.(type) {
	case *ast.BoolLit:
		return nil, &EvalError{Pos: e.Pos, Message: "not yet implemented"}
	case *ast.IntLit:
		return nil, &EvalError{Pos: e.Pos, Message: "not yet implemented"}
	case *ast.FloatLit:
		return nil, &EvalError{Pos: e.Pos, Message: "not yet implemented"}
	case *ast.NullLit:
		return nil, &EvalError{Pos: e.Pos, Message: "not yet implemented"}
	case *ast.StringExpr:
		return nil, &EvalError{Pos: e.Pos, Message: "not yet implemented"}
	case *ast.CommandExpr:
		return nil, &EvalError{Pos: e.Pos, Message: "not yet implemented"}
	case *ast.ArrayExpr:
		return nil, &EvalError{Pos: e.Pos, Message: "not yet implemented"}
	case *ast.MapExpr:
		return nil, &EvalError{Pos: e.Pos, Message: "not yet implemented"}
	case *ast.PairExpr:
		return nil, &EvalError{Pos: e.Pos, Message: "not yet implemented"}
	case *ast.StructExpr:
		return nil, &EvalError{Pos: e.Pos, Message: "not yet implemented"}
	case *ast.IfThenElse:
		return nil, &EvalError{Pos: e.Pos, Message: "not yet implemented"}
	case *ast.BinaryExpr:
		return nil, &EvalError{Pos: e.Pos, Message: "not yet implemented"}
	case *ast.UnaryExpr:
		return nil, &EvalError{Pos: e.Pos, Message: "not yet implemented"}
	case *ast.IndexExpr:
		return nil, &EvalError{Pos: e.Pos, Message: "not yet implemented"}
	case *ast.GetExpr:
		return nil, &EvalError{Pos: e.Pos, Message: "not yet implemented"}
	case *ast.ApplyExpr:
		return evalApply(e, env, ctx)
	case *ast.IdentExpr:
		return nil, &EvalError{Pos: e.Pos, Message: "not yet implemented"}
	default:
		return nil, &EvalError{Message: "unknown expression type"}
	}
}

// exprPos extracts the source position from any ast.Expr concrete type.
// Used by helpers that receive an ast.Expr interface and need position for error reporting.
func exprPos(expr ast.Expr) ast.Pos {
	switch e := expr.(type) {
	case *ast.BoolLit:
		return e.Pos
	case *ast.IntLit:
		return e.Pos
	case *ast.FloatLit:
		return e.Pos
	case *ast.NullLit:
		return e.Pos
	case *ast.StringExpr:
		return e.Pos
	case *ast.CommandExpr:
		return e.Pos
	case *ast.ArrayExpr:
		return e.Pos
	case *ast.MapExpr:
		return e.Pos
	case *ast.PairExpr:
		return e.Pos
	case *ast.StructExpr:
		return e.Pos
	case *ast.IfThenElse:
		return e.Pos
	case *ast.BinaryExpr:
		return e.Pos
	case *ast.UnaryExpr:
		return e.Pos
	case *ast.IndexExpr:
		return e.Pos
	case *ast.GetExpr:
		return e.Pos
	case *ast.ApplyExpr:
		return e.Pos
	case *ast.IdentExpr:
		return e.Pos
	default:
		return ast.Pos{}
	}
}
