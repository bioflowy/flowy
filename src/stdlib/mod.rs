//! WDL Standard Library implementation
//!
//! This module provides the standard library functions and operators for WDL,
//! similar to miniwdl's StdLib.py

use crate::error::WdlError;
use crate::expr::Expression;
use crate::env::Environment;
use crate::types::Type;
use crate::value::Value;
use std::collections::HashMap;
use std::path::{Path, PathBuf};

// Import submodules
pub mod task_output;

// Re-export all function structs for convenience

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
        // Make paths relative to task_dir if they're within it
        if let Ok(relative_path) = path.strip_prefix(&self.task_dir) {
            relative_path
                .to_str()
                .map(|s| s.to_string())
                .ok_or_else(|| WdlError::RuntimeError {
                    message: format!("Invalid path: {}", path.display()),
                })
        } else {
            // For paths outside task_dir, return absolute path
            path.to_str()
                .map(|s| s.to_string())
                .ok_or_else(|| WdlError::RuntimeError {
                    message: format!("Invalid path: {}", path.display()),
                })
        }
    }

    fn as_any(&self) -> &dyn std::any::Any {
        self
    }
}

/// Function trait for standard library functions
pub trait Function: Send + Sync {
    /// Check argument types and return the result type
    /// Performs type inference on the given expression arguments
    fn infer_type(&self, args: &[Expression], env: &Environment) -> Result<Type, WdlError>;

    /// Evaluate the function with given expression arguments
    /// Performs argument evaluation and then function execution
    fn eval(&self, args: &[Expression], env: &Environment, stdlib: &StdLib) -> Result<Value, WdlError>;

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

    /// Get the task directory if using TaskPathMapper, None otherwise
    pub fn task_dir(&self) -> Option<&PathBuf> {
        if let Some(task_mapper) = self.path_mapper.as_any().downcast_ref::<TaskPathMapper>() {
            Some(task_mapper.task_dir())
        } else {
            None
        }
    }

    /// Register all built-in functions
    fn register_builtin_functions(&mut self) {
        // Math functions
        // self.register_function(Box::new(FloorFunction));
        // self.register_function(Box::new(CeilFunction));
        // self.register_function(Box::new(RoundFunction));
        // self.register_function(Box::new(MinFunction));
        // self.register_function(Box::new(MaxFunction));

        // Array functions
        // self.register_function(Box::new(LengthFunction));
        // self.register_function(Box::new(SelectFirstFunction));
        // self.register_function(Box::new(SelectAllFunction));
        // self.register_function(Box::new(FlattenFunction));
        // self.register_function(Box::new(RangeFunction));
        // self.register_function(Box::new(PrefixFunction));
        // self.register_function(Box::new(SuffixFunction));
        // self.register_function(Box::new(QuoteFunction));
        // self.register_function(Box::new(SquoteFunction));
        // self.register_function(Box::new(ZipFunction));
        // self.register_function(Box::new(CrossFunction));
        // self.register_function(Box::new(TransposeFunction));
        // self.register_function(Box::new(UnzipFunction));

        // Map functions
        // self.register_function(Box::new(KeysFunction));
        // self.register_function(Box::new(ValuesFunction));
        // self.register_function(Box::new(ContainsKeyFunction));
        // self.register_function(Box::new(ContainsFunction));
        // self.register_function(Box::new(AsPairsFunction));
        // self.register_function(Box::new(AsMapFunction));

        // String functions
        // self.register_function(Box::new(FindFunction));
        // self.register_function(Box::new(SubFunction));
        // self.register_function(Box::new(BasenameFunction));
        // self.register_function(Box::new(SepFunction));
        // self.register_function(Box::new(JoinPathsFunction));

        // Type functions
        // self.register_function(Box::new(DefinedFunction));

        // I/O functions
        // stdout() and stderr() are only available in task output context
        if self.is_task_context {
            // self.register_function(Box::new(StdoutFunction));
            // self.register_function(Box::new(StderrFunction));

            // glob() function is only available in task context
            // if let Some(task_mapper) = self.path_mapper.as_any().downcast_ref::<TaskPathMapper>() {
            //     self.register_function(io::create_glob(task_mapper.task_dir().clone()));
            // }
        }

        // Write functions
        // self.register_function(io::create_write_lines());
        // self.register_function(io::create_write_tsv());
        // self.register_function(io::create_write_map());
        // self.register_function(io::create_write_json());

        // Read functions
        // self.register_function(Box::new(ReadLinesFunction));
        // self.register_function(Box::new(ReadStringFunction));
        // self.register_function(io::create_read_int());
        // self.register_function(io::create_read_float());
        // self.register_function(io::create_read_boolean());
        // self.register_function(io::create_read_json());
        // self.register_function(io::create_read_tsv());
        // self.register_function(io::create_read_map());
        // self.register_function(io::create_read_objects());
        // self.register_function(io::create_read_object());

        // File system functions
        // self.register_function(io::create_size());
    }

    /// Register all operators
    fn register_operators(&mut self) {
        // Arithmetic operators
        // self.register_function(Box::new(AddOperator));
        // self.register_function(Box::new(SubtractOperator));
        // self.register_function(Box::new(MultiplyOperator));
        // self.register_function(Box::new(DivideOperator));
        // self.register_function(Box::new(RemainderOperator));

        // Comparison operators
        // self.register_function(Box::new(EqualOperator));
        // self.register_function(Box::new(NotEqualOperator));
        // self.register_function(Box::new(LessThanOperator));
        // self.register_function(Box::new(LessThanEqualOperator));
        // self.register_function(Box::new(GreaterThanOperator));
        // self.register_function(Box::new(GreaterThanEqualOperator));

        // Logical operators
        // self.register_function(Box::new(LogicalAndOperator));
        // self.register_function(Box::new(LogicalOrOperator));
        // self.register_function(Box::new(LogicalNotOperator));

        // Unary operators
        // self.register_function(Box::new(NegateOperator));
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

        // Test that stdlib can be created
        assert_eq!(stdlib.wdl_version(), "1.0");

        // Test that no functions are registered yet (all are commented out)
        assert!(stdlib.get_function("floor").is_none());
        assert!(stdlib.get_function("length").is_none());
        assert!(stdlib.get_function("_add").is_none());
        assert!(stdlib.get_function("nonexistent").is_none());
    }
}
