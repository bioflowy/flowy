//! WDL Standard Library implementation
//!
//! This module provides the standard library functions and operators for WDL,
//! similar to miniwdl's StdLib.py

use crate::error::WdlError;
use crate::types::Type;
use crate::value::Value;
use std::collections::HashMap;
use std::path::{Path, PathBuf};

// Import submodules
pub mod arrays;
pub mod io;
pub mod math;
pub mod operators;
pub mod strings;
pub mod task_output;
pub mod types;

// Re-export all function structs for convenience
pub use arrays::*;
pub use io::*;
pub use math::*;
pub use operators::*;
pub use strings::*;
pub use types::*;

/// Path mapping trait for file virtualization/devirtualization
/// Similar to miniwdl's _devirtualize_filename and _virtualize_filename
pub trait PathMapper: Send + Sync {
    /// Convert a virtual filename to a real filesystem path that can be opened
    fn devirtualize_filename(&self, filename: &str) -> Result<PathBuf, WdlError>;

    /// Convert a real filesystem path to a virtual filename for WDL values
    fn virtualize_filename(&self, path: &Path) -> Result<String, WdlError>;

    /// For downcasting to concrete types
    fn as_any(&self) -> &dyn std::any::Any;
}

/// Default path mapper that performs no transformation
pub struct DefaultPathMapper;

impl PathMapper for DefaultPathMapper {
    fn devirtualize_filename(&self, filename: &str) -> Result<PathBuf, WdlError> {
        Ok(PathBuf::from(filename))
    }

    fn virtualize_filename(&self, path: &Path) -> Result<String, WdlError> {
        path.to_str()
            .map(|s| s.to_string())
            .ok_or_else(|| WdlError::RuntimeError {
                message: format!("Invalid path: {}", path.display()),
            })
    }

    fn as_any(&self) -> &dyn std::any::Any {
        self
    }
}

/// Task-specific path mapper that resolves relative paths against a task directory
pub struct TaskPathMapper {
    task_dir: PathBuf,
}

impl TaskPathMapper {
    pub fn new(task_dir: PathBuf) -> Self {
        Self { task_dir }
    }

    pub fn task_dir(&self) -> &PathBuf {
        &self.task_dir
    }
}

impl PathMapper for TaskPathMapper {
    fn devirtualize_filename(&self, filename: &str) -> Result<PathBuf, WdlError> {
        let path = Path::new(filename);
        if path.is_absolute() {
            Ok(path.to_path_buf())
        } else {
            Ok(self.task_dir.join(path))
        }
    }

    fn virtualize_filename(&self, path: &Path) -> Result<String, WdlError> {
        // For virtualization, we could potentially make paths relative to task_dir
        // For now, just return the absolute path
        path.to_str()
            .map(|s| s.to_string())
            .ok_or_else(|| WdlError::RuntimeError {
                message: format!("Invalid path: {}", path.display()),
            })
    }

    fn as_any(&self) -> &dyn std::any::Any {
        self
    }
}

/// Function trait for standard library functions
pub trait Function: Send + Sync {
    /// Check argument types and return the result type
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError>;

    /// Evaluate the function with given arguments
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError>;

    /// Evaluate the function with given arguments and access to stdlib context
    /// Default implementation calls eval() for backward compatibility
    fn eval_with_stdlib(&self, args: &[Value], _stdlib: &StdLib) -> Result<Value, WdlError> {
        self.eval(args)
    }

    /// Get the function name
    fn name(&self) -> &str;
}

/// Standard library containing all built-in functions and operators
pub struct StdLib {
    functions: HashMap<String, Box<dyn Function>>,
    wdl_version: String,
    path_mapper: Box<dyn PathMapper>,
    is_task_context: bool,
}

impl StdLib {
    /// Create a new standard library instance for the given WDL version
    pub fn new(wdl_version: &str) -> Self {
        Self::with_path_mapper(wdl_version, Box::new(DefaultPathMapper), false)
    }

    /// Create a standard library instance with custom path mapper and context
    pub fn with_path_mapper(
        wdl_version: &str,
        path_mapper: Box<dyn PathMapper>,
        is_task_context: bool,
    ) -> Self {
        let mut stdlib = StdLib {
            functions: HashMap::new(),
            wdl_version: wdl_version.to_string(),
            path_mapper,
            is_task_context,
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

    /// Get the path mapper for file operations
    pub fn path_mapper(&self) -> &dyn PathMapper {
        self.path_mapper.as_ref()
    }

    /// Get the WDL version this standard library was initialized for
    pub fn wdl_version(&self) -> &str {
        &self.wdl_version
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
        // stdout() and stderr() are only available in task output context
        if self.is_task_context {
            self.register_function(Box::new(StdoutFunction));
            self.register_function(Box::new(StderrFunction));

            // glob() function is only available in task context
            if let Some(task_mapper) = self.path_mapper.as_any().downcast_ref::<TaskPathMapper>() {
                self.register_function(io::create_glob(task_mapper.task_dir().clone()));
            }
        }

        self.register_function(Box::new(WriteLinesFunction));
        self.register_function(Box::new(ReadLinesFunction));
        self.register_function(Box::new(ReadStringFunction));
        self.register_function(io::create_read_int());
        self.register_function(io::create_read_float());
        self.register_function(io::create_read_boolean());
        self.register_function(io::create_read_json());
        self.register_function(io::create_read_tsv());
        self.register_function(io::create_read_map());
        self.register_function(io::create_read_objects());
        self.register_function(io::create_read_object());
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

    /// Add or replace a function in the library (public method)
    pub fn add_function(&mut self, function: Box<dyn Function>) {
        self.register_function(function);
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
