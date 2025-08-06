package tree

import (
	"fmt"

	"github.com/bioflowy/flowy/pkg/env"
	"github.com/bioflowy/flowy/pkg/errors"
	"github.com/bioflowy/flowy/pkg/expr"
	"github.com/bioflowy/flowy/pkg/types"
)

// Call represents a task or workflow call
type Call struct {
	BaseWorkflowNode
	Name   string
	Callee string
	Alias  *string
	Inputs map[string]expr.Expr
	After  []string // Dependencies specified with 'after' clause
}

// NewCall creates a new call node
func NewCall(name, callee string, alias *string, inputs map[string]expr.Expr, pos errors.SourcePosition) *Call {
	return &Call{
		BaseWorkflowNode: NewBaseWorkflowNode(pos),
		Name:             name,
		Callee:           callee,
		Alias:            alias,
		Inputs:           inputs,
		After:            []string{},
	}
}

func (c *Call) WorkflowNodeName() string {
	if c.Alias != nil {
		return *c.Alias
	}
	return c.Name
}

func (c *Call) Dependencies(availableInputs *env.Bindings[types.Base]) ([]string, error) {
	deps := make([]string, 0)

	// Add explicit 'after' dependencies
	deps = append(deps, c.After...)

	// Collect dependencies from input expressions
	for _, inputExpr := range c.Inputs {
		exprDeps := collectExpressionDependencies(inputExpr, availableInputs)
		deps = append(deps, exprDeps...)
	}

	return deps, nil
}

func (c *Call) AddToTypeEnv(typeEnv *env.Bindings[types.Base]) (*env.Bindings[types.Base], error) {
	// For now, we don't know the output types without resolving the callee
	// In a complete implementation, this would look up the callee's output types
	// and add them to the type environment with the call's name as prefix

	// Placeholder: assume calls produce a generic type
	callName := c.WorkflowNodeName()
	genericType := types.NewAny(false, false)

	return typeEnv.Bind(callName, genericType, &c.Pos), nil
}

// SetAfter sets the 'after' dependencies for this call
func (c *Call) SetAfter(after []string) {
	c.After = after
}

// AddInput adds an input binding to this call
func (c *Call) AddInput(name string, inputExpr expr.Expr) {
	if c.Inputs == nil {
		c.Inputs = make(map[string]expr.Expr)
	}
	c.Inputs[name] = inputExpr
}

// GetEffectiveName returns the effective name for this call (alias if present, otherwise name)
func (c *Call) GetEffectiveName() string {
	if c.Alias != nil {
		return *c.Alias
	}
	return c.Name
}

// String returns a string representation of the call
func (c *Call) String() string {
	name := c.GetEffectiveName()
	if c.Alias != nil {
		return fmt.Sprintf("call %s as %s", c.Callee, name)
	}
	return fmt.Sprintf("call %s", c.Callee)
}
