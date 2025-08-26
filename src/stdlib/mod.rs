//! WDL Standard Library implementation
//!
//! This module provides the standard library functions and operators for WDL,
//! similar to miniwdl's StdLib.py

use crate::error::WdlError;
use crate::types::Type;
use crate::value::Value;
use std::collections::HashMap;

// Import submodules
pub mod arrays;
pub mod io;
pub mod math;
pub mod operators;
pub mod strings;
pub mod types;

// Re-export all function structs for convenience
pub use arrays::*;
pub use io::*;
pub use math::*;
pub use operators::*;
pub use strings::*;
pub use types::*;

/// Function trait for standard library functions
pub trait Function: Send + Sync {
    /// Check argument types and return the result type
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError>;

    /// Evaluate the function with given arguments
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError>;

    /// Get the function name
    fn name(&self) -> &str;
}

/// Standard library containing all built-in functions and operators
pub struct StdLib {
    functions: HashMap<String, Box<dyn Function>>,
    #[allow(dead_code)]
    wdl_version: String,
}

impl StdLib {
    /// Create a new standard library instance for the given WDL version
    pub fn new(wdl_version: &str) -> Self {
        let mut stdlib = StdLib {
            functions: HashMap::new(),
            wdl_version: wdl_version.to_string(),
        };

        // Register built-in functions
        stdlib.register_builtin_functions();
        stdlib.register_operators();

        stdlib
    }

    /// Get a function by name
    pub fn get_function(&self, name: &str) -> Option<&dyn Function> {
        self.functions.get(name).map(|f| f.as_ref())
    }

    /// Register all built-in functions
    fn register_builtin_functions(&mut self) {
        // Math functions
        self.register_function(Box::new(FloorFunction));
        self.register_function(Box::new(CeilFunction));
        self.register_function(Box::new(RoundFunction));
        self.register_function(Box::new(MinFunction));
        self.register_function(Box::new(MaxFunction));

        // Array functions
        self.register_function(Box::new(LengthFunction));
        self.register_function(Box::new(SelectFirstFunction));
        self.register_function(Box::new(SelectAllFunction));
        self.register_function(Box::new(FlattenFunction));
        self.register_function(Box::new(RangeFunction));

        // String functions
        self.register_function(Box::new(SubFunction));
        self.register_function(Box::new(BasenameFunction));
        self.register_function(Box::new(SepFunction));

        // Type functions
        self.register_function(Box::new(DefinedFunction));

        // I/O functions
        self.register_function(Box::new(StdoutFunction));
        self.register_function(Box::new(StderrFunction));
        self.register_function(Box::new(WriteLinesFunction));
        self.register_function(Box::new(ReadLinesFunction));
    }

    /// Register all operators
    fn register_operators(&mut self) {
        // Arithmetic operators
        self.register_function(Box::new(AddOperator));
        self.register_function(Box::new(SubtractOperator));
        self.register_function(Box::new(MultiplyOperator));
        self.register_function(Box::new(DivideOperator));
        self.register_function(Box::new(RemainderOperator));

        // Comparison operators
        self.register_function(Box::new(EqualOperator));
        self.register_function(Box::new(NotEqualOperator));
        self.register_function(Box::new(LessThanOperator));
        self.register_function(Box::new(LessThanEqualOperator));
        self.register_function(Box::new(GreaterThanOperator));
        self.register_function(Box::new(GreaterThanEqualOperator));

        // Logical operators
        self.register_function(Box::new(LogicalAndOperator));
        self.register_function(Box::new(LogicalOrOperator));
        self.register_function(Box::new(LogicalNotOperator));
    }

    /// Register a function with the library
    fn register_function(&mut self, function: Box<dyn Function>) {
        let name = function.name().to_string();
        self.functions.insert(name, function);
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_stdlib_creation() {
        let stdlib = StdLib::new("1.0");

        // Test that functions are registered
        assert!(stdlib.get_function("floor").is_some());
        assert!(stdlib.get_function("length").is_some());
        assert!(stdlib.get_function("_add").is_some());
        assert!(stdlib.get_function("nonexistent").is_none());
    }

    #[test]
    fn test_math_functions() {
        let stdlib = StdLib::new("1.0");

        let floor_fn = stdlib.get_function("floor").unwrap();
        let result = floor_fn.eval(&[Value::float(3.7)]).unwrap();
        assert_eq!(result.as_int().unwrap(), 3);

        let min_fn = stdlib.get_function("min").unwrap();
        let result = min_fn.eval(&[Value::int(5), Value::int(3)]).unwrap();
        assert_eq!(result.as_int().unwrap(), 3);
    }

    #[test]
    fn test_array_functions() {
        let stdlib = StdLib::new("1.0");

        let length_fn = stdlib.get_function("length").unwrap();
        let arr = Value::array(
            Type::int(false),
            vec![Value::int(1), Value::int(2), Value::int(3)],
        );
        let result = length_fn.eval(&[arr]).unwrap();
        assert_eq!(result.as_int().unwrap(), 3);

        let range_fn = stdlib.get_function("range").unwrap();
        let result = range_fn.eval(&[Value::int(3)]).unwrap();
        let values = result.as_array().unwrap();
        assert_eq!(values.len(), 3);
        assert_eq!(values[0].as_int().unwrap(), 0);
        assert_eq!(values[1].as_int().unwrap(), 1);
        assert_eq!(values[2].as_int().unwrap(), 2);
    }

    #[test]
    fn test_operators() {
        let stdlib = StdLib::new("1.0");

        let add_fn = stdlib.get_function("_add").unwrap();
        let result = add_fn.eval(&[Value::int(5), Value::int(3)]).unwrap();
        assert_eq!(result.as_int().unwrap(), 8);

        let eq_fn = stdlib.get_function("_eqeq").unwrap();
        let result = eq_fn.eval(&[Value::int(5), Value::int(5)]).unwrap();
        assert!(result.as_bool().unwrap());
    }
}
