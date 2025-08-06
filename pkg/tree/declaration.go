package tree

import (
	"github.com/bioflowy/flowy/pkg/env"
	"github.com/bioflowy/flowy/pkg/errors"
	"github.com/bioflowy/flowy/pkg/expr"
	"github.com/bioflowy/flowy/pkg/types"
)

// Decl represents a variable declaration
type Decl struct {
	BaseWorkflowNode
	Name string
	Type types.Base
	Expr expr.Expr // Optional initialization expression
}

// NewDecl creates a new variable declaration
func NewDecl(name string, declType types.Base, initExpr expr.Expr, pos errors.SourcePosition) *Decl {
	return &Decl{
		BaseWorkflowNode: NewBaseWorkflowNode(pos),
		Name:             name,
		Type:             declType,
		Expr:             initExpr,
	}
}

func (d *Decl) WorkflowNodeName() string {
	return d.Name
}

func (d *Decl) Dependencies(availableInputs *env.Bindings[types.Base]) ([]string, error) {
	if d.Expr == nil {
		return []string{}, nil
	}

	// Collect dependencies from the initialization expression
	deps := collectExpressionDependencies(d.Expr, availableInputs)
	return deps, nil
}

func (d *Decl) AddToTypeEnv(typeEnv *env.Bindings[types.Base]) (*env.Bindings[types.Base], error) {
	// Add this declaration's type binding to the environment
	return typeEnv.Bind(d.Name, d.Type, &d.Pos), nil
}

// Input represents an input declaration
type Input struct {
	BaseSourceNode
	Decls []*Decl
}

// NewInput creates a new input section
func NewInput(decls []*Decl, pos errors.SourcePosition) *Input {
	return &Input{
		BaseSourceNode: NewBaseSourceNode(pos),
		Decls:          decls,
	}
}

// Output represents an output declaration
type Output struct {
	BaseSourceNode
	Decls []*Decl
}

// NewOutput creates a new output section
func NewOutput(decls []*Decl, pos errors.SourcePosition) *Output {
	return &Output{
		BaseSourceNode: NewBaseSourceNode(pos),
		Decls:          decls,
	}
}

// collectExpressionDependencies is a helper function to collect variable dependencies
// from an expression tree. This is a simplified implementation.
func collectExpressionDependencies(e expr.Expr, availableInputs *env.Bindings[types.Base]) []string {
	var deps []string

	// Walk the expression tree and collect identifier references
	walkExpressionTree(e, func(node expr.Expr) {
		// Check if this is an identifier expression that references a variable
		// This would need to be implemented based on the actual expr package structure
		// For now, we return an empty list as a placeholder
	})

	return deps
}

// walkExpressionTree is a helper to traverse expression trees
func walkExpressionTree(e expr.Expr, visitor func(expr.Expr)) {
	if e == nil {
		return
	}

	visitor(e)

	// Recursively visit children
	for _, child := range e.Children() {
		walkExpressionTree(child, visitor)
	}
}
