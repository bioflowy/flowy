// Package ast defines the Abstract Syntax Tree node types for WDL 1.2.
// All AST structs embed a Pos field (not a pointer) carrying the absolute
// source file path and the 1-based line/column of the node's opening token.
// This enables precise per-node origin tracking in multi-file (imported) documents.
package ast

import "github.com/bioflowy/flowy/wdl/types"

// Pos records the source location of an AST node.
// File is an absolute path to the source file, enabling unambiguous cross-file
// error reporting in documents that resolve imports from multiple locations.
type Pos struct {
	File string // absolute path to source file
	Line int    // 1-based line number
	Col  int    // 1-based column number
}

// WorkflowNode is a marker interface for nodes that may appear in a Workflow body.
// Decl, Call, Scatter, and Conditional all implement this interface.
type WorkflowNode interface {
	isWorkflowNode()
}

// StringPart is a marker interface for segments within a StringExpr or CommandExpr.
// RawStringPart and PlaceholderPart implement this interface.
type StringPart interface {
	isStringPart()
}

// RawStringPart is a literal text segment within a string or command expression.
type RawStringPart struct {
	Val string
}

func (*RawStringPart) isStringPart() {}

// PlaceholderPart is an interpolation placeholder within a string or command expression.
// Options holds key=value pairs from the placeholder (e.g., "sep", "true", "false", "default").
type PlaceholderPart struct {
	Pos     Pos
	Options map[string]string
	Expr    Expr
}

func (*PlaceholderPart) isStringPart() {}

// CommandStyle indicates whether a command block uses brace syntax ({{{ }}} / { }) or
// heredoc syntax (<<~ COMMAND ... COMMAND).
type CommandStyle int

const (
	CommandBrace   CommandStyle = iota // task command { ... }
	CommandHeredoc                     // task command <<< ... >>>
)

// Document is the top-level AST node produced by parsing a WDL source file.
// When import resolution is enabled, all imported tasks/workflows/structs
// are merged into a single flat Document.
type Document struct {
	Pos
	Version   string
	Tasks     []*Task
	Workflows []*Workflow
	Structs   []*StructDef
}

// Task represents a WDL task definition.
type Task struct {
	Pos
	Name         string
	Inputs       []*Decl
	Decls        []*Decl          // private declarations in the task body
	Command      *CommandExpr
	Outputs      []*Decl
	Runtime      map[string]Expr
	Requirements map[string]Expr
	Hints        map[string]any
	Meta         map[string]any
	ParamMeta    map[string]any
}

// Workflow represents a WDL workflow definition.
type Workflow struct {
	Pos
	Name      string
	Inputs    []*Decl
	Body      []WorkflowNode
	Outputs   []*Decl
	Meta      map[string]any
	ParamMeta map[string]any
}

// Decl represents a WDL declaration (typed variable binding).
// Expr is nil for unbound declarations (e.g., struct members, input stubs).
// Env is true for WDL 1.2 "env" modifier on task inputs.
type Decl struct {
	Pos
	Type types.Type
	Name string
	Expr Expr
	Env  bool
}

func (*Decl) isWorkflowNode() {}

// StructDef represents a WDL struct type definition.
// Members are []*Decl with Expr==nil (struct members are always unbound).
type StructDef struct {
	Pos
	Name    string
	Members []*Decl
}

// Call represents a WDL call statement within a workflow.
// Callee is the dot-separated task/workflow name (e.g., ["pkg", "my_task"]).
// After lists explicit ordering dependencies introduced with "after NAME".
// Inputs maps input names to their binding expressions.
type Call struct {
	Pos
	Callee []string
	Alias  string
	After  []string
	Inputs map[string]Expr
}

func (*Call) isWorkflowNode() {}

// Scatter represents a WDL scatter statement.
// Variable is the loop variable name; Expr is the collection expression.
type Scatter struct {
	Pos
	Variable string
	Expr     Expr
	Body     []WorkflowNode
}

func (*Scatter) isWorkflowNode() {}

// Conditional represents a WDL if statement within a workflow.
type Conditional struct {
	Pos
	Expr Expr
	Body []WorkflowNode
}

func (*Conditional) isWorkflowNode() {}
