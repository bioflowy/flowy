package env

import (
	"testing"
)

func TestBinding(t *testing.T) {
	binding := NewBinding("test_var", 42, "extra_info")

	if binding.Name() != "test_var" {
		t.Errorf("Expected name 'test_var', got '%s'", binding.Name())
	}
	if binding.Value() != 42 {
		t.Errorf("Expected value 42, got %v", binding.Value())
	}
	if binding.Info() != "extra_info" {
		t.Errorf("Expected info 'extra_info', got %v", binding.Info())
	}
}

func TestBindingWithoutInfo(t *testing.T) {
	binding := NewBinding("var", "value", nil)

	if binding.Name() != "var" {
		t.Errorf("Expected name 'var', got '%s'", binding.Name())
	}
	if binding.Value() != "value" {
		t.Errorf("Expected value 'value', got %v", binding.Value())
	}
	if binding.Info() != nil {
		t.Errorf("Expected info to be nil, got %v", binding.Info())
	}
}

func TestEmptyBindings(t *testing.T) {
	env := NewBindings[int]()

	if env.Length() != 0 {
		t.Errorf("Expected empty environment length to be 0, got %d", env.Length())
	}

	if env.IsEmpty() {
		t.Log("Empty environment correctly reports as empty")
	} else {
		t.Error("Empty environment should report as empty")
	}
}

func TestSingleBinding(t *testing.T) {
	env := NewBindings[int]()
	env = env.Bind("x", 42, nil)

	if env.Length() != 1 {
		t.Errorf("Expected environment length to be 1, got %d", env.Length())
	}

	if env.IsEmpty() {
		t.Error("Non-empty environment should not report as empty")
	}

	value, err := env.Resolve("x")
	if err != nil {
		t.Fatalf("Expected to resolve 'x', got error: %v", err)
	}
	if value != 42 {
		t.Errorf("Expected value 42, got %v", value)
	}
}

func TestMultipleBindings(t *testing.T) {
	env := NewBindings[int]()
	env = env.Bind("x", 1, nil)
	env = env.Bind("y", 42, nil)
	env = env.Bind("z", 100, nil)

	if env.Length() != 3 {
		t.Errorf("Expected environment length to be 3, got %d", env.Length())
	}

	// Test all bindings exist
	tests := []struct {
		name     string
		expected int
	}{
		{"x", 1},
		{"y", 42},
		{"z", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := env.Resolve(tt.name)
			if err != nil {
				t.Fatalf("Expected to resolve '%s', got error: %v", tt.name, err)
			}
			if value != tt.expected {
				t.Errorf("Expected value %d, got %v", tt.expected, value)
			}
		})
	}
}

func TestBindingShadowing(t *testing.T) {
	env := NewBindings[string]()
	env = env.Bind("x", "first", nil)
	env = env.Bind("x", "second", nil) // Shadow the first binding

	if env.Length() != 1 {
		t.Errorf("Expected environment length to be 1 (shadowed), got %d", env.Length())
	}

	value, err := env.Resolve("x")
	if err != nil {
		t.Fatalf("Expected to resolve 'x', got error: %v", err)
	}
	if value != "second" {
		t.Errorf("Expected shadowed value 'second', got %v", value)
	}
}

func TestResolveBinding(t *testing.T) {
	env := NewBindings[string]()
	env = env.Bind("test", "value", "info")

	binding, err := env.ResolveBinding("test")
	if err != nil {
		t.Fatalf("Expected to resolve binding 'test', got error: %v", err)
	}

	if binding.Name() != "test" {
		t.Errorf("Expected binding name 'test', got '%s'", binding.Name())
	}
	if binding.Value() != "value" {
		t.Errorf("Expected binding value 'value', got %v", binding.Value())
	}
	if binding.Info() != "info" {
		t.Errorf("Expected binding info 'info', got %v", binding.Info())
	}
}

func TestResolveNonexistent(t *testing.T) {
	env := NewBindings[int]()
	env = env.Bind("x", 42, nil)

	_, err := env.Resolve("y")
	if err == nil {
		t.Error("Expected error when resolving nonexistent binding 'y'")
	}
}

func TestGet(t *testing.T) {
	env := NewBindings[int]()
	env = env.Bind("x", 42, nil)

	// Test existing binding
	value := env.Get("x", 0)
	if value != 42 {
		t.Errorf("Expected value 42, got %v", value)
	}

	// Test nonexistent binding with default
	value = env.Get("y", 999)
	if value != 999 {
		t.Errorf("Expected default value 999, got %v", value)
	}

	// Test nonexistent binding without default
	value = env.Get("z", 0)
	if value != 0 {
		t.Errorf("Expected zero value 0, got %v", value)
	}
}

func TestHasBinding(t *testing.T) {
	env := NewBindings[string]()
	env = env.Bind("exists", "value", nil)

	if !env.HasBinding("exists") {
		t.Error("Expected HasBinding to return true for existing binding")
	}

	if env.HasBinding("nonexistent") {
		t.Error("Expected HasBinding to return false for nonexistent binding")
	}
}

func TestIterator(t *testing.T) {
	env := NewBindings[int]()
	env = env.Bind("a", 1, nil)
	env = env.Bind("b", 2, nil)
	env = env.Bind("c", 3, nil)

	// Collect all bindings
	var names []string
	var values []int
	env.ForEach(func(binding *Binding[int]) {
		names = append(names, binding.Name())
		values = append(values, binding.Value())
	})

	if len(names) != 3 {
		t.Errorf("Expected 3 bindings, got %d", len(names))
	}

	// Check that all expected names are present (order may vary)
	expectedNames := map[string]bool{"a": true, "b": true, "c": true}
	for _, name := range names {
		if !expectedNames[name] {
			t.Errorf("Unexpected binding name: %s", name)
		}
	}
}

func TestNamespaces(t *testing.T) {
	env := NewBindings[int]()
	env = env.Bind("simple", 1, nil)
	env = env.Bind("ns1.var1", 2, nil)
	env = env.Bind("ns1.var2", 3, nil)
	env = env.Bind("ns1.sub.var3", 4, nil)
	env = env.Bind("ns2.var4", 5, nil)

	namespaces := env.Namespaces()
	expectedNS := []string{"ns1.", "ns1.sub.", "ns2."}

	if len(namespaces) != len(expectedNS) {
		t.Errorf("Expected %d namespaces, got %d", len(expectedNS), len(namespaces))
	}

	nsMap := make(map[string]bool)
	for _, ns := range namespaces {
		nsMap[ns] = true
	}

	for _, expected := range expectedNS {
		if !nsMap[expected] {
			t.Errorf("Expected namespace '%s' not found", expected)
		}
	}
}

func TestHasNamespace(t *testing.T) {
	env := NewBindings[int]()
	env = env.Bind("ns1.var1", 1, nil)
	env = env.Bind("ns1.sub.var2", 2, nil)

	if !env.HasNamespace("ns1") {
		t.Error("Expected HasNamespace to return true for 'ns1'")
	}

	if !env.HasNamespace("ns1.") {
		t.Error("Expected HasNamespace to return true for 'ns1.' (with dot)")
	}

	if !env.HasNamespace("ns1.sub") {
		t.Error("Expected HasNamespace to return true for 'ns1.sub'")
	}

	if env.HasNamespace("nonexistent") {
		t.Error("Expected HasNamespace to return false for 'nonexistent'")
	}
}

func TestEnterNamespace(t *testing.T) {
	env := NewBindings[int]()
	env = env.Bind("ns1.var1", 42, nil)
	env = env.Bind("ns1.var2", 100, nil)
	env = env.Bind("ns2.var3", 200, nil)
	env = env.Bind("simple", 300, nil)

	nsEnv := env.EnterNamespace("ns1")

	// Should have only ns1 variables, with namespace prefix removed
	if nsEnv.Length() != 2 {
		t.Errorf("Expected namespace environment to have 2 bindings, got %d", nsEnv.Length())
	}

	value, err := nsEnv.Resolve("var1")
	if err != nil {
		t.Fatalf("Expected to resolve 'var1' in namespace, got error: %v", err)
	}
	if value != 42 {
		t.Errorf("Expected value 42, got %v", value)
	}

	value, err = nsEnv.Resolve("var2")
	if err != nil {
		t.Fatalf("Expected to resolve 'var2' in namespace, got error: %v", err)
	}
	if value != 100 {
		t.Errorf("Expected value 100, got %v", value)
	}

	// Should not have ns2 or simple variables
	if nsEnv.HasBinding("var3") {
		t.Error("Should not have var3 from ns2 namespace")
	}
	if nsEnv.HasBinding("simple") {
		t.Error("Should not have simple variable")
	}
}

func TestWrapNamespace(t *testing.T) {
	env := NewBindings[int]()
	env = env.Bind("var1", 42, nil)
	env = env.Bind("var2", 100, nil)

	wrappedEnv := env.WrapNamespace("prefix")

	if wrappedEnv.Length() != 2 {
		t.Errorf("Expected wrapped environment to have 2 bindings, got %d", wrappedEnv.Length())
	}

	value, err := wrappedEnv.Resolve("prefix.var1")
	if err != nil {
		t.Fatalf("Expected to resolve 'prefix.var1', got error: %v", err)
	}
	if value != 42 {
		t.Errorf("Expected value 42, got %v", value)
	}

	value, err = wrappedEnv.Resolve("prefix.var2")
	if err != nil {
		t.Fatalf("Expected to resolve 'prefix.var2', got error: %v", err)
	}
	if value != 100 {
		t.Errorf("Expected value 100, got %v", value)
	}

	// Original names should not exist
	if wrappedEnv.HasBinding("var1") {
		t.Error("Should not have unwrapped var1")
	}
}

func TestMerge(t *testing.T) {
	env1 := NewBindings[int]()
	env1 = env1.Bind("a", 1, nil)
	env1 = env1.Bind("b", 2, nil)

	env2 := NewBindings[int]()
	env2 = env2.Bind("b", 20, nil) // Conflicts with env1
	env2 = env2.Bind("c", 30, nil)

	env3 := NewBindings[int]()
	env3 = env3.Bind("d", 40, nil)

	merged := Merge(env1, env2, env3)

	// Should have all unique bindings
	expectedBindings := map[string]int{
		"a": 1,  // from env1
		"b": 2,  // from env1 (precedence over env2)
		"c": 30, // from env2
		"d": 40, // from env3
	}

	if merged.Length() != len(expectedBindings) {
		t.Errorf("Expected merged environment to have %d bindings, got %d", len(expectedBindings), merged.Length())
	}

	for name, expectedValue := range expectedBindings {
		value, err := merged.Resolve(name)
		if err != nil {
			t.Errorf("Expected to resolve '%s' in merged environment, got error: %v", name, err)
			continue
		}
		if value != expectedValue {
			t.Errorf("Expected value %d for '%s', got %v", expectedValue, name, value)
		}
	}
}