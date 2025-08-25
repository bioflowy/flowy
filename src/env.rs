//! Environment system for identifier resolution during WDL typechecking and evaluation.
//!
//! This module provides immutable linked-list environments for binding names to values,
//! supporting namespace operations and efficient shadowing semantics.

use std::collections::HashSet;
use std::fmt;
use serde::{Deserialize, Serialize};

/// An individual binding of a name to a value with optional metadata.
///
/// Generic over `T` which is typically a `Value` (for value environments) 
/// or `Type` (for type environments).
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Binding<T> {
    name: String,
    value: T,
    info: Option<String>, // Simplified from Python's Any to String for now
}

impl<T> Binding<T> {
    /// Create a new binding.
    pub fn new(name: String, value: T, info: Option<String>) -> Self {
        Self { name, value, info }
    }

    /// Get the binding name. Namespaced names are dot-separated.
    pub fn name(&self) -> &str {
        &self.name
    }

    /// Get the bound value.
    pub fn value(&self) -> &T {
        &self.value
    }

    /// Get the bound value by consuming the binding.
    pub fn into_value(self) -> T {
        self.value
    }

    /// Get optional metadata about this binding.
    pub fn info(&self) -> Option<&String> {
        self.info.as_ref()
    }
}

impl<T: fmt::Display> fmt::Display for Binding<T> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{} = {}", self.name, self.value)
    }
}

/// Immutable environment consisting of a linked list of bindings.
///
/// Provides O(1) prepend operations and supports shadowing where newer bindings
/// hide older ones with the same name.
#[derive(Debug, Clone)]
pub struct Bindings<T>
where
    T: Clone,
{
    binding: Option<Binding<T>>,
    next: Option<Box<Bindings<T>>>,
    #[allow(dead_code)]
    namespaces_cache: Option<HashSet<String>>,
}

impl<T> Default for Bindings<T>
where
    T: Clone,
{
    fn default() -> Self {
        Self::new()
    }
}

impl<T> Bindings<T>
where
    T: Clone,
{
    /// Create an empty environment.
    pub fn new() -> Self {
        Self {
            binding: None,
            next: None,
            namespaces_cache: None,
        }
    }

    /// Create a new environment with a single binding.
    fn new_with_binding(binding: Binding<T>, next: Option<Box<Bindings<T>>>) -> Self {
        Self {
            binding: Some(binding),
            next,
            namespaces_cache: None,
        }
    }

    /// Check if the environment is empty.
    pub fn is_empty(&self) -> bool {
        self.iter().next().is_none()
    }

    /// Get the number of unique bindings in this environment.
    pub fn len(&self) -> usize {
        self.iter().count()
    }

    /// Return a new environment with a binding added.
    /// 
    /// Any existing binding with the same name is shadowed.
    pub fn bind(&self, name: String, value: T, info: Option<String>) -> Self {
        assert!(!name.is_empty() && !name.starts_with('.') && !name.ends_with('.'));
        
        let binding = Binding::new(name, value, info);
        Self::new_with_binding(binding, Some(Box::new((*self).clone())))
    }

    /// Look up a binding by name.
    pub fn resolve_binding(&self, name: &str) -> Option<&Binding<T>> {
        for binding in self.iter() {
            if binding.name() == name {
                return Some(binding);
            }
        }
        None
    }

    /// Look up a value by name.
    pub fn resolve(&self, name: &str) -> Option<&T> {
        self.resolve_binding(name).map(|b| b.value())
    }

    /// Get a value with a default.
    pub fn get<'a>(&'a self, name: &str, default: Option<&'a T>) -> Option<&'a T> {
        self.resolve(name).or(default)
    }

    /// Check if a name has a binding.
    pub fn has_binding(&self, name: &str) -> bool {
        self.resolve_binding(name).is_some()
    }

    /// Check if a name is bound in this environment.
    pub fn contains(&self, name: &str) -> bool {
        self.has_binding(name)
    }

    /// Transform each binding with a function, filtering out None results.
    pub fn map<U, F>(&self, f: F) -> Bindings<U> 
    where
        U: Clone,
        F: Fn(&Binding<T>) -> Option<Binding<U>>,
    {
        let mut result = Bindings::new();
        let mut bindings = Vec::new();
        
        // Collect all bindings first
        for binding in self.iter() {
            if let Some(mapped) = f(binding) {
                bindings.push(mapped);
            }
        }
        
        // Add them in reverse order to maintain original ordering
        for binding in bindings.into_iter().rev() {
            let Binding { name, value, info } = binding;
            result = result.bind(name, value, info);
        }
        
        result
    }

    /// Filter bindings by a predicate.
    pub fn filter<F>(&self, pred: F) -> Self
    where 
        F: Fn(&Binding<T>) -> bool,
        T: Clone,
    {
        self.map(|b| if pred(b) { Some(b.clone()) } else { None })
    }

    /// Remove bindings whose names exist in the other environment.
    pub fn subtract<U>(&self, other: &Bindings<U>) -> Self
    where
        T: Clone,
        U: Clone,
    {
        self.filter(|b| !other.has_binding(b.name()))
    }

    /// Get all namespaces in this environment.
    /// 
    /// Returns dot-separated prefixes of binding names, each ending with a dot.
    pub fn namespaces(&self) -> HashSet<String> {
        let mut namespaces = HashSet::new();
        
        for binding in self.iter() {
            let parts: Vec<&str> = binding.name().split('.').collect();
            if parts.len() > 1 {
                for i in 0..(parts.len() - 1) {
                    let ns = parts[..=i].join(".") + ".";
                    namespaces.insert(ns);
                }
            }
        }
        
        namespaces
    }

    /// Check if a namespace exists.
    pub fn has_namespace(&self, namespace: &str) -> bool {
        let ns = if namespace.ends_with('.') {
            namespace.to_string()
        } else {
            format!("{}.", namespace)
        };
        
        self.namespaces().contains(&ns)
    }

    /// Create a new environment with only bindings in the given namespace,
    /// removing the namespace prefix from binding names.
    pub fn enter_namespace(&self, namespace: &str) -> Self
    where
        T: Clone,
    {
        let ns = if namespace.ends_with('.') {
            namespace.to_string()
        } else {
            format!("{}.", namespace)
        };

        self.map(|b| {
            if b.name().starts_with(&ns) {
                let new_name = b.name()[ns.len()..].to_string();
                Some(Binding::new(new_name, b.value().clone(), b.info().cloned()))
            } else {
                None
            }
        })
    }

    /// Create a new environment with the namespace prefix added to all binding names.
    pub fn wrap_namespace(&self, namespace: &str) -> Self
    where
        T: Clone,
    {
        let ns = if namespace.ends_with('.') {
            namespace.to_string()  
        } else {
            format!("{}.", namespace)
        };

        let mut result = Bindings::new();
        let mut bindings = Vec::new();
        
        for binding in self.iter() {
            let new_name = format!("{}{}", ns, binding.name());
            bindings.push(Binding::new(new_name, binding.value().clone(), binding.info().cloned()));
        }
        
        // Add in reverse to maintain order
        for binding in bindings.into_iter().rev() {
            let Binding { name, value, info } = binding;
            result = result.bind(name, value, info);
        }
        
        result
    }

    /// Iterator over unique bindings (shadowed bindings are skipped).
    pub fn iter(&self) -> BindingIterator<'_, T> {
        BindingIterator {
            current: Some(self),
            seen: HashSet::new(),
        }
    }
}

/// Iterator over bindings in an environment.
pub struct BindingIterator<'a, T: Clone> {
    current: Option<&'a Bindings<T>>,
    seen: HashSet<String>,
}

impl<'a, T: Clone> Iterator for BindingIterator<'a, T> {
    type Item = &'a Binding<T>;

    fn next(&mut self) -> Option<Self::Item> {
        while let Some(env) = self.current {
            if let Some(ref binding) = env.binding {
                self.current = env.next.as_deref();
                
                if !self.seen.contains(binding.name()) {
                    self.seen.insert(binding.name().to_string());
                    return Some(binding);
                }
            } else {
                self.current = env.next.as_deref();
            }
        }
        None
    }
}

/// Merge multiple environments into one.
/// 
/// When the same name appears in multiple environments, 
/// the first (leftmost) occurrence takes precedence.
pub fn merge<T>(environments: &[&Bindings<T>]) -> Bindings<T>
where
    T: Clone,
{
    if environments.is_empty() {
        return Bindings::new();
    }
    
    // Start with the last (rightmost) environment
    let mut result = (*environments.last().unwrap()).clone();
    
    // Process environments from right to left, but skip the last one we already used
    for &env in environments.iter().rev().skip(1) {
        for binding in env.iter() {
            // Prepend this binding - it will shadow any existing binding with same name
            result = result.bind(
                binding.name().to_string(),
                binding.value().clone(),
                binding.info().cloned(),
            );
        }
    }
    
    result
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_empty_environment() {
        let env: Bindings<i32> = Bindings::new();
        assert!(env.is_empty());
        assert_eq!(env.len(), 0);
        assert!(!env.has_binding("x"));
    }

    #[test]
    fn test_single_binding() {
        let env = Bindings::new().bind("x".to_string(), 42, None);
        assert!(!env.is_empty());
        assert_eq!(env.len(), 1);
        assert!(env.has_binding("x"));
        assert_eq!(env.resolve("x"), Some(&42));
        assert_eq!(env.resolve("y"), None);
    }

    #[test]
    fn test_multiple_bindings() {
        let env = Bindings::new()
            .bind("x".to_string(), 42, None)
            .bind("y".to_string(), 100, Some("test".to_string()));

        assert_eq!(env.len(), 2);
        assert_eq!(env.resolve("x"), Some(&42));
        assert_eq!(env.resolve("y"), Some(&100));
        
        let y_binding = env.resolve_binding("y").unwrap();
        assert_eq!(y_binding.info(), Some(&"test".to_string()));
    }

    #[test]
    fn test_shadowing() {
        let env = Bindings::new()
            .bind("x".to_string(), 42, None)
            .bind("x".to_string(), 100, None);

        assert_eq!(env.len(), 1); // Only one unique binding
        assert_eq!(env.resolve("x"), Some(&100)); // Newer value shadows older
    }

    #[test]
    fn test_iteration() {
        let env = Bindings::new()
            .bind("a".to_string(), 1, None)
            .bind("b".to_string(), 2, None)
            .bind("c".to_string(), 3, None);

        let names: Vec<&str> = env.iter().map(|b| b.name()).collect();
        assert_eq!(names, vec!["c", "b", "a"]); // Most recent first
    }

    #[test]
    fn test_contains() {
        let env = Bindings::new().bind("test".to_string(), 42, None);
        assert!(env.contains("test"));
        assert!(!env.contains("missing"));
    }

    #[test]
    fn test_filter() {
        let env = Bindings::new()
            .bind("x".to_string(), 1, None)
            .bind("y".to_string(), 2, None)
            .bind("z".to_string(), 3, None);

        let filtered = env.filter(|b| *b.value() > 1);
        assert_eq!(filtered.len(), 2);
        assert!(filtered.has_binding("y"));
        assert!(filtered.has_binding("z"));
        assert!(!filtered.has_binding("x"));
    }

    #[test]
    fn test_subtract() {
        let env1 = Bindings::new()
            .bind("a".to_string(), 1, None)
            .bind("b".to_string(), 2, None)
            .bind("c".to_string(), 3, None);

        let env2 = Bindings::new().bind("b".to_string(), 99, None);

        let result = env1.subtract(&env2);
        assert_eq!(result.len(), 2);
        assert!(result.has_binding("a"));
        assert!(result.has_binding("c"));
        assert!(!result.has_binding("b"));
    }

    #[test]
    fn test_namespaces() {
        let env = Bindings::new()
            .bind("x".to_string(), 1, None)
            .bind("foo.bar".to_string(), 2, None)
            .bind("foo.baz.qux".to_string(), 3, None);

        let namespaces = env.namespaces();
        assert!(namespaces.contains("foo."));
        assert!(namespaces.contains("foo.baz."));
        assert!(!namespaces.contains("x."));
    }

    #[test]
    fn test_has_namespace() {
        let env = Bindings::new().bind("foo.bar".to_string(), 1, None);
        assert!(env.has_namespace("foo"));
        assert!(env.has_namespace("foo."));
        assert!(!env.has_namespace("bar"));
    }

    #[test]
    fn test_enter_namespace() {
        let env = Bindings::new()
            .bind("foo.bar".to_string(), 1, None)
            .bind("foo.baz".to_string(), 2, None)
            .bind("other.x".to_string(), 3, None);

        let foo_env = env.enter_namespace("foo");
        assert_eq!(foo_env.len(), 2);
        assert_eq!(foo_env.resolve("bar"), Some(&1));
        assert_eq!(foo_env.resolve("baz"), Some(&2));
        assert!(!foo_env.has_binding("other.x"));
    }

    #[test]
    fn test_wrap_namespace() {
        let env = Bindings::new()
            .bind("x".to_string(), 1, None)
            .bind("y".to_string(), 2, None);

        let wrapped = env.wrap_namespace("prefix");
        assert_eq!(wrapped.len(), 2);
        assert_eq!(wrapped.resolve("prefix.x"), Some(&1));
        assert_eq!(wrapped.resolve("prefix.y"), Some(&2));
        assert!(!wrapped.has_binding("x"));
    }

    #[test]
    fn test_merge() {
        let env1 = Bindings::new()
            .bind("a".to_string(), 1, None)
            .bind("b".to_string(), 2, None);

        let env2 = Bindings::new()
            .bind("b".to_string(), 99, None) // Should be shadowed by env1
            .bind("c".to_string(), 3, None);

        let env3 = Bindings::new().bind("d".to_string(), 4, None);

        let merged = merge(&[&env1, &env2, &env3]);
        
        assert_eq!(merged.resolve("a"), Some(&1)); // From env1
        assert_eq!(merged.resolve("b"), Some(&2)); // From env1 (shadows env2)  
        assert_eq!(merged.resolve("c"), Some(&3)); // From env2
        assert_eq!(merged.resolve("d"), Some(&4)); // From env3
    }

    #[test]
    fn test_map_transformation() {
        let env = Bindings::new()
            .bind("x".to_string(), 1, None)
            .bind("y".to_string(), 2, None);

        let doubled: Bindings<i32> = env.map(|b| {
            Some(Binding::new(
                b.name().to_string(),
                b.value() * 2,
                b.info().cloned()
            ))
        });

        assert_eq!(doubled.resolve("x"), Some(&2));
        assert_eq!(doubled.resolve("y"), Some(&4));
    }

    #[test]
    fn test_get_with_default() {
        let env = Bindings::new().bind("x".to_string(), 42, None);
        let default_val = 999;
        
        assert_eq!(env.get("x", Some(&default_val)), Some(&42));
        assert_eq!(env.get("missing", Some(&default_val)), Some(&default_val));
        assert_eq!(env.get("missing", None), None);
    }
}