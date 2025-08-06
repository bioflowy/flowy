package tree

import (
	"fmt"

	"github.com/bioflowy/flowy/pkg/env"
	"github.com/bioflowy/flowy/pkg/errors"
	"github.com/bioflowy/flowy/pkg/expr"
	"github.com/bioflowy/flowy/pkg/types"
)

// Scatter represents a scatter section in a workflow
type Scatter struct {
	BaseWorkflowNode
	Variable   string
	Collection expr.Expr
	Body       []WorkflowNode
}

// NewScatter creates a new scatter section
func NewScatter(variable string, collection expr.Expr, body []WorkflowNode, pos errors.SourcePosition) *Scatter {
	return &Scatter{
		BaseWorkflowNode: NewBaseWorkflowNode(pos),
		Variable:         variable,
		Collection:       collection,
		Body:             body,
	}
}

func (s *Scatter) WorkflowNodeName() string {
	// Scatter sections don't have a single name, but we use a placeholder
	return fmt.Sprintf("scatter_%s", s.Variable)
}

func (s *Scatter) Dependencies(availableInputs *env.Bindings[types.Base]) ([]string, error) {
	// Dependencies from the collection expression
	deps := collectExpressionDependencies(s.Collection, availableInputs)

	// Dependencies from the body nodes (but these are internal to the scatter)
	// We don't include them as external dependencies

	return deps, nil
}

func (s *Scatter) AddToTypeEnv(typeEnv *env.Bindings[types.Base]) (*env.Bindings[types.Base], error) {
	// Scatter transforms the types of its body outputs
	// Each output becomes an Array[T] where T is the original type

	// For now, we'll implement a simplified version
	// In a complete implementation, we'd need to:
	// 1. Infer the collection element type
	// 2. Create a scoped type environment with the scatter variable
	// 3. Process the body and collect outputs
	// 4. Transform output types to Array[T]

	return typeEnv, nil
}

// GetScatterVariable returns the scatter variable name
func (s *Scatter) GetScatterVariable() string {
	return s.Variable
}

// GetCollection returns the collection expression
func (s *Scatter) GetCollection() expr.Expr {
	return s.Collection
}

// GetBody returns the scatter body
func (s *Scatter) GetBody() []WorkflowNode {
	return s.Body
}

// Conditional represents a conditional (if) section in a workflow
type Conditional struct {
	BaseWorkflowNode
	Condition expr.Expr
	Body      []WorkflowNode
}

// NewConditional creates a new conditional section
func NewConditional(condition expr.Expr, body []WorkflowNode, pos errors.SourcePosition) *Conditional {
	return &Conditional{
		BaseWorkflowNode: NewBaseWorkflowNode(pos),
		Condition:        condition,
		Body:             body,
	}
}

func (c *Conditional) WorkflowNodeName() string {
	// Conditional sections don't have a single name, use a placeholder
	return "conditional"
}

func (c *Conditional) Dependencies(availableInputs *env.Bindings[types.Base]) ([]string, error) {
	// Dependencies from the condition expression
	deps := collectExpressionDependencies(c.Condition, availableInputs)

	// Dependencies from the body nodes (but these are internal to the conditional)
	// We don't include them as external dependencies

	return deps, nil
}

func (c *Conditional) AddToTypeEnv(typeEnv *env.Bindings[types.Base]) (*env.Bindings[types.Base], error) {
	// Conditional transforms the types of its body outputs
	// Each output becomes T? (optional) where T is the original type

	// For now, we'll implement a simplified version
	// In a complete implementation, we'd need to:
	// 1. Process the body and collect outputs
	// 2. Transform output types to optional (T?)

	return typeEnv, nil
}

// GetCondition returns the condition expression
func (c *Conditional) GetCondition() expr.Expr {
	return c.Condition
}

// GetBody returns the conditional body
func (c *Conditional) GetBody() []WorkflowNode {
	return c.Body
}

// String returns a string representation of the conditional
func (c *Conditional) String() string {
	return "if (" + c.Condition.String() + ")"
}
