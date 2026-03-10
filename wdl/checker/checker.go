// Package checker implements the WDL 1.2 type checker.
// It traverses the AST produced by the parser and verifies type correctness.
package checker

import (
	"fmt"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/types"
)

// TypeError reports a type-checking error at a specific source location.
type TypeError struct {
	Pos     ast.Pos
	Message string
}

// Error implements the error interface.
func (e *TypeError) Error() string {
	return fmt.Sprintf("type error: %s\n  --> %s:%d:%d", e.Message, e.Pos.File, e.Pos.Line, e.Pos.Col)
}

// TypeEnv maps declaration names to their resolved WDL types.
// The top-level TypeEnv contains all declarations visible at document scope.
type TypeEnv map[string]types.Type

// Scope is a linked-list node representing a lexical scope.
// Each Scope holds its own declarations and a pointer to the enclosing parent scope.
type Scope struct {
	decls  map[string]types.Type
	parent *Scope
}

// newScope creates a new Scope whose parent is p (may be nil for the root scope).
func newScope(p *Scope) *Scope {
	return &Scope{
		decls:  make(map[string]types.Type),
		parent: p,
	}
}

// lookup searches for name in this scope and all enclosing scopes.
// Returns the type and true if found, or nil and false otherwise.
func (s *Scope) lookup(name string) (types.Type, bool) {
	for cur := s; cur != nil; cur = cur.parent {
		if t, ok := cur.decls[name]; ok {
			return t, true
		}
	}
	return nil, false
}

// define binds name to t in the current (innermost) scope.
func (s *Scope) define(name string, t types.Type) {
	s.decls[name] = t
}

// Check performs type checking on doc and returns the top-level TypeEnv and
// any type errors found. The stub always returns an empty TypeEnv and no errors.
func Check(doc *ast.Document) (TypeEnv, []error) {
	return make(TypeEnv), nil
}
