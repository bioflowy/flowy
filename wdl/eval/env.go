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

// Parent returns the parent scope of this Env (nil for top-level).
// Used by code that needs to reconstruct a scope chain with translated bindings.
func (e *Env) Parent() *Env {
	return e.parent
}

// ForEach calls fn for each name/value pair in the current (innermost) scope only.
// It does NOT walk parent scopes. This is useful for copying or transforming the
// current scope's bindings without exposing the underlying map.
func (e *Env) ForEach(fn func(name string, v values.Value)) {
	for name, v := range e.bindings {
		fn(name, v)
	}
}
