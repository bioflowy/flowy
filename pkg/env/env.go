// Package env provides environments for identifier resolution during WDL typechecking and evaluation
package env

import (
	"fmt"
	"strings"
)

// Binding represents an individual, immutable binding of a name to a value
type Binding[T any] struct {
	name  string
	value T
	info  any
}

// NewBinding creates a new binding with the given name, value, and optional info
func NewBinding[T any](name string, value T, info any) *Binding[T] {
	return &Binding[T]{
		name:  name,
		value: value,
		info:  info,
	}
}

// Name returns the binding name (may be namespaced with dot-separated strings)
func (b *Binding[T]) Name() string {
	return b.name
}

// Value returns the bound value
func (b *Binding[T]) Value() T {
	return b.value
}

// Info returns the additional informational value (if any)
func (b *Binding[T]) Info() any {
	return b.info
}

// Bindings represents an environment consisting of an immutable linked-list of Binding objects
type Bindings[T any] struct {
	binding *Binding[T]
	next    *Bindings[T]
}

// NewBindings creates a new empty environment
func NewBindings[T any]() *Bindings[T] {
	return &Bindings[T]{}
}

// IsEmpty returns true if the environment has no bindings
func (b *Bindings[T]) IsEmpty() bool {
	return b.binding == nil
}

// Length returns the number of unique bindings (after shadowing)
func (b *Bindings[T]) Length() int {
	count := 0
	b.ForEach(func(*Binding[T]) {
		count++
	})
	return count
}

// Bind returns a new environment with a new binding prepended
// Any existing binding for the same name is shadowed by the new one
func (b *Bindings[T]) Bind(name string, value T, info any) *Bindings[T] {
	if name == "" || strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") {
		panic(fmt.Sprintf("invalid binding name: %s", name))
	}

	binding := NewBinding(name, value, info)
	return &Bindings[T]{
		binding: binding,
		next:    b,
	}
}

// ResolveBinding looks up a Binding object by name
func (b *Bindings[T]) ResolveBinding(name string) (*Binding[T], error) {
	seen := make(map[string]bool)
	current := b

	for current != nil && current.binding != nil {
		if current.binding.name == name && !seen[name] {
			return current.binding, nil
		}
		seen[current.binding.name] = true
		current = current.next
	}

	return nil, fmt.Errorf("binding not found: %s", name)
}

// Resolve looks up a bound value by name
func (b *Bindings[T]) Resolve(name string) (T, error) {
	binding, err := b.ResolveBinding(name)
	if err != nil {
		var zero T
		return zero, err
	}
	return binding.Value(), nil
}

// Get looks up a bound value by name, returning the default value if not found
func (b *Bindings[T]) Get(name string, defaultValue T) T {
	if value, err := b.Resolve(name); err == nil {
		return value
	}
	return defaultValue
}

// HasBinding determines existence of a binding for the name
func (b *Bindings[T]) HasBinding(name string) bool {
	_, err := b.ResolveBinding(name)
	return err == nil
}

// ForEach iterates over all unique bindings (shadowing removes duplicates)
func (b *Bindings[T]) ForEach(fn func(*Binding[T])) {
	seen := make(map[string]bool)
	current := b

	for current != nil && current.binding != nil {
		if !seen[current.binding.name] {
			seen[current.binding.name] = true
			fn(current.binding)
		}
		current = current.next
	}
}

// Map copies the environment with each binding transformed by the given function
// If the function returns nil, the binding is excluded
func (b *Bindings[T]) Map(fn func(*Binding[T]) *Binding[T]) *Bindings[T] {
	var result *Bindings[T] = NewBindings[T]()
	var bindings []*Binding[T]

	// Collect transformed bindings
	b.ForEach(func(binding *Binding[T]) {
		if transformed := fn(binding); transformed != nil {
			bindings = append(bindings, transformed)
		}
	})

	// Add in reverse order to maintain original order
	for i := len(bindings) - 1; i >= 0; i-- {
		result = &Bindings[T]{
			binding: bindings[i],
			next:    result,
		}
	}

	return result
}

// Filter copies the environment with only those bindings for which pred returns true
func (b *Bindings[T]) Filter(pred func(*Binding[T]) bool) *Bindings[T] {
	return b.Map(func(binding *Binding[T]) *Binding[T] {
		if pred(binding) {
			return binding
		}
		return nil
	})
}

// Subtract copies the environment excluding any binding for which rhs has a binding with the same name
func (b *Bindings[T]) Subtract(rhs *Bindings[any]) *Bindings[T] {
	return b.Filter(func(binding *Binding[T]) bool {
		return !rhs.HasBinding(binding.Name())
	})
}

// Namespaces returns all distinct dot-separated prefixes of the binding names
// Each element ends with a dot
func (b *Bindings[T]) Namespaces() []string {
	nsSet := make(map[string]bool)

	b.ForEach(func(binding *Binding[T]) {
		parts := strings.Split(binding.Name(), ".")
		if len(parts) > 1 {
			for i := 0; i < len(parts)-1; i++ {
				ns := strings.Join(parts[:i+1], ".") + "."
				nsSet[ns] = true
			}
		}
	})

	var namespaces []string
	for ns := range nsSet {
		namespaces = append(namespaces, ns)
	}

	return namespaces
}

// HasNamespace determines existence of a namespace in the environment
func (b *Bindings[T]) HasNamespace(namespace string) bool {
	if !strings.HasSuffix(namespace, ".") {
		namespace += "."
	}

	found := false
	b.ForEach(func(binding *Binding[T]) {
		if strings.HasPrefix(binding.Name()+".", namespace) {
			found = true
		}
	})

	return found
}

// EnterNamespace generates an environment with only bindings in the given namespace,
// with the namespace prefix removed from each binding's name
func (b *Bindings[T]) EnterNamespace(namespace string) *Bindings[T] {
	if !strings.HasSuffix(namespace, ".") {
		namespace += "."
	}

	return b.Map(func(binding *Binding[T]) *Binding[T] {
		if strings.HasPrefix(binding.Name(), namespace) {
			newName := strings.TrimPrefix(binding.Name(), namespace)
			return NewBinding(newName, binding.Value(), binding.Info())
		}
		return nil
	})
}

// WrapNamespace copies the environment with the given namespace prefixed to each binding name
func (b *Bindings[T]) WrapNamespace(namespace string) *Bindings[T] {
	if !strings.HasSuffix(namespace, ".") {
		namespace += "."
	}

	return b.Map(func(binding *Binding[T]) *Binding[T] {
		newName := namespace + binding.Name()
		return NewBinding(newName, binding.Value(), binding.Info())
	})
}

// Merge merges several Bindings environments into one
// Should the same name appear in multiple arguments, the first (leftmost) occurrence takes precedence
func Merge[T any](envs ...*Bindings[T]) *Bindings[T] {
	if len(envs) == 0 {
		return NewBindings[T]()
	}

	// Start with the last environment for efficiency
	result := envs[len(envs)-1]
	if result == nil {
		result = NewBindings[T]()
	}

	// Add bindings from earlier environments (in reverse order to maintain precedence)
	for i := len(envs) - 2; i >= 0; i-- {
		if envs[i] != nil {
			// Collect bindings in reverse order to maintain original order
			var bindings []*Binding[T]
			envs[i].ForEach(func(binding *Binding[T]) {
				bindings = append(bindings, binding)
			})

			// Add in reverse order
			for j := len(bindings) - 1; j >= 0; j-- {
				b := bindings[j]
				result = &Bindings[T]{
					binding: b,
					next:    result,
				}
			}
		}
	}

	return result
}
