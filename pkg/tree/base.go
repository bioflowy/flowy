// Package tree provides WDL document AST nodes and tree structure
package tree

import (
	"github.com/bioflowy/flowy/pkg/env"
	"github.com/bioflowy/flowy/pkg/errors"
	"github.com/bioflowy/flowy/pkg/expr"
	"github.com/bioflowy/flowy/pkg/types"
)

// SourceNode represents a node in the WDL AST with source position information
type SourceNode interface {
	// SourcePosition returns the source position of this node
	SourcePosition() errors.SourcePosition
}

// BaseSourceNode provides common functionality for all AST nodes
type BaseSourceNode struct {
	Pos errors.SourcePosition
}

// NewBaseSourceNode creates a new base source node
func NewBaseSourceNode(pos errors.SourcePosition) BaseSourceNode {
	return BaseSourceNode{Pos: pos}
}

func (n BaseSourceNode) SourcePosition() errors.SourcePosition {
	return n.Pos
}

// WorkflowNode represents a node that can appear in workflow scope
type WorkflowNode interface {
	SourceNode

	// WorkflowNodeName returns the name identifier for this node
	WorkflowNodeName() string

	// Dependencies returns the set of other workflow nodes this depends on
	Dependencies(availableInputs *env.Bindings[types.Base]) ([]string, error)

	// AddToTypeEnv adds any type bindings this node provides to the type environment
	AddToTypeEnv(typeEnv *env.Bindings[types.Base]) (*env.Bindings[types.Base], error)
}

// BaseWorkflowNode provides common functionality for workflow nodes
type BaseWorkflowNode struct {
	BaseSourceNode
}

// NewBaseWorkflowNode creates a new base workflow node
func NewBaseWorkflowNode(pos errors.SourcePosition) BaseWorkflowNode {
	return BaseWorkflowNode{
		BaseSourceNode: NewBaseSourceNode(pos),
	}
}

// Document represents a complete WDL document
type Document struct {
	BaseSourceNode
	Imports  []*Import
	Workflow *Workflow
	Tasks    []*Task
	Structs  []*StructTypeDef
}

// NewDocument creates a new WDL document
func NewDocument(imports []*Import, workflow *Workflow, tasks []*Task, structs []*StructTypeDef, pos errors.SourcePosition) *Document {
	return &Document{
		BaseSourceNode: NewBaseSourceNode(pos),
		Imports:        imports,
		Workflow:       workflow,
		Tasks:          tasks,
		Structs:        structs,
	}
}

// Import represents an import statement
type Import struct {
	BaseSourceNode
	URI       string
	Namespace string
	Alias     *string
}

// NewImport creates a new import statement
func NewImport(uri, namespace string, alias *string, pos errors.SourcePosition) *Import {
	return &Import{
		BaseSourceNode: NewBaseSourceNode(pos),
		URI:            uri,
		Namespace:      namespace,
		Alias:          alias,
	}
}

// Task represents a WDL task definition
type Task struct {
	BaseSourceNode
	Name          string
	Inputs        []*Decl
	Outputs       []*Decl
	Command       *TaskCommand
	Runtime       map[string]expr.Expr
	Meta          map[string]interface{}
	ParameterMeta map[string]interface{}
}

// NewTask creates a new task definition
func NewTask(name string, inputs, outputs []*Decl, command *TaskCommand, pos errors.SourcePosition) *Task {
	return &Task{
		BaseSourceNode: NewBaseSourceNode(pos),
		Name:           name,
		Inputs:         inputs,
		Outputs:        outputs,
		Command:        command,
		Runtime:        make(map[string]expr.Expr),
		Meta:           make(map[string]interface{}),
		ParameterMeta:  make(map[string]interface{}),
	}
}

// TaskCommand represents the command section of a task
type TaskCommand struct {
	BaseSourceNode
	Command expr.Expr
}

// NewTaskCommand creates a new task command
func NewTaskCommand(command expr.Expr, pos errors.SourcePosition) *TaskCommand {
	return &TaskCommand{
		BaseSourceNode: NewBaseSourceNode(pos),
		Command:        command,
	}
}

// Workflow represents a WDL workflow definition
type Workflow struct {
	BaseSourceNode
	Name    string
	Inputs  []*Decl
	Body    []WorkflowNode
	Outputs []*Decl
}

// NewWorkflow creates a new workflow definition
func NewWorkflow(name string, inputs []*Decl, body []WorkflowNode, outputs []*Decl, pos errors.SourcePosition) *Workflow {
	return &Workflow{
		BaseSourceNode: NewBaseSourceNode(pos),
		Name:           name,
		Inputs:         inputs,
		Body:           body,
		Outputs:        outputs,
	}
}

// StructTypeDef represents a struct type definition
type StructTypeDef struct {
	BaseSourceNode
	Name    string
	Members []*StructMember
}

// NewStructTypeDef creates a new struct type definition
func NewStructTypeDef(name string, members []*StructMember, pos errors.SourcePosition) *StructTypeDef {
	return &StructTypeDef{
		BaseSourceNode: NewBaseSourceNode(pos),
		Name:           name,
		Members:        members,
	}
}

// StructMember represents a member of a struct type
type StructMember struct {
	BaseSourceNode
	Name string
	Type types.Base
}

// NewStructMember creates a new struct member
func NewStructMember(name string, memberType types.Base, pos errors.SourcePosition) *StructMember {
	return &StructMember{
		BaseSourceNode: NewBaseSourceNode(pos),
		Name:           name,
		Type:           memberType,
	}
}
