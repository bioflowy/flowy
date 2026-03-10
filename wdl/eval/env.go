package eval

import "github.com/bioflowy/flowy/wdl/values"

// Env is a linked-list runtime variable scope, mirroring checker.Scope but holding values.Value.
type Env struct {
	bindings map[string]values.Value
	parent   *Env
}

// NewEnv creates a new Env child scope with the given parent (nil for top-level).
func NewEnv(parent *Env) *Env {
	return &Env{bindings: make(map[string]values.Value), parent: parent}
}

// Define binds name to v in the current (innermost) scope.
func (e *Env) Define(name string, v values.Value) {
	e.bindings[name] = v
}

// Lookup walks the scope chain from innermost to outermost, returning the first binding found.
func (e *Env) Lookup(name string) (values.Value, bool) {
	for cur := e; cur != nil; cur = cur.parent {
		if v, ok := cur.bindings[name]; ok {
			return v, true
		}
	}
	return nil, false
}
