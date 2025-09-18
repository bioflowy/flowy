//! WDL Standard Library implementation
//!
//! This module provides the standard library functions and operators for WDL,
//! similar to miniwdl's StdLib.py

use crate::env::Bindings;
use crate::error::{SourcePosition, WdlError};
use crate::expr::{Expression, ExpressionBase};
use crate::types::Type;
use crate::value::Value;
use std::collections::HashMap;
use std::path::{Path, PathBuf};

// Import submodules
pub mod arrays;
pub mod io;
pub mod map;
pub mod math;
pub mod operators;
pub mod strings;
pub mod task_output;

// Re-export all function structs for convenience

/// Path mapping trait for file virtualization/devirtualization
/// Similar to miniwdl's _devirtualize_filename and _virtualize_filename
pub trait PathMapper: Send + Sync {
    /// Convert a virtual filename to a real filesystem path that can be opened
    fn devirtualize_filename(&self, filename: &str) -> Result<PathBuf, WdlError>;

    /// Convert a real filesystem path to a virtual filename for WDL values
    fn virtualize_filename(&self, path: &Path) -> Result<String, WdlError>;

    /// Clone this PathMapper
    fn clone_boxed(&self) -> Box<dyn PathMapper>;

    /// For downcasting to concrete types
    fn as_any(&self) -> &dyn std::any::Any;
}

/// Default path mapper that performs no transformation
#[derive(Debug, Clone)]
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

    fn clone_boxed(&self) -> Box<dyn PathMapper> {
        Box::new(DefaultPathMapper)
    }

    fn as_any(&self) -> &dyn std::any::Any {
        self
    }
}

/// Task-specific path mapper that resolves relative paths against a task directory
#[derive(Debug, Clone)]
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

    fn clone_boxed(&self) -> Box<dyn PathMapper> {
        Box::new(TaskPathMapper {
            task_dir: self.task_dir.clone(),
        })
    }

    fn as_any(&self) -> &dyn std::any::Any {
        self
    }
}

/// Function trait for standard library functions
pub trait Function: Send + Sync {
    /// Check argument types and return the result type
    /// Performs type inference on the given expression arguments
    fn infer_type(
        &self,
        args: &mut [Expression],
        type_env: &Bindings<Type>,
        stdlib: &StdLib,
        struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<Type, WdlError>;

    /// Evaluate the function with given expression arguments
    /// Performs argument evaluation and then function execution
    fn eval(
        &self,
        args: &[Expression],
        env: &Bindings<Value>,
        stdlib: &StdLib,
    ) -> Result<Value, WdlError>;

    /// Get the function name
    fn name(&self) -> &str;
}

/// Static function implementation that takes a function operating on Values
pub struct StaticFunction {
    name: String,
    argument_types: Vec<Type>,
    return_type: Type,
    implementation: Box<dyn Fn(&[Value]) -> Result<Value, WdlError> + Send + Sync>,
}

impl StaticFunction {
    /// Create a new static function
    pub fn new<F>(
        name: String,
        argument_types: Vec<Type>,
        return_type: Type,
        implementation: F,
    ) -> Self
    where
        F: Fn(&[Value]) -> Result<Value, WdlError> + Send + Sync + 'static,
    {
        Self {
            name,
            argument_types,
            return_type,
            implementation: Box::new(implementation),
        }
    }
}

impl Function for StaticFunction {
    fn infer_type(
        &self,
        args: &mut [Expression],
        type_env: &Bindings<Type>,
        stdlib: &StdLib,
        struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<Type, WdlError> {
        // Check argument count
        if args.len() != self.argument_types.len() {
            let pos = if args.is_empty() {
                SourcePosition::new("unknown".to_string(), "unknown".to_string(), 0, 0, 0, 0)
            } else {
                args[0].source_position().clone()
            };
            return Err(WdlError::Validation {
                pos,
                message: format!(
                    "Function '{}' expects {} arguments, got {}",
                    self.name,
                    self.argument_types.len(),
                    args.len()
                ),
                source_text: None,
                declared_wdl_version: None,
            });
        }

        // Check each argument type
        for (i, (arg_expr, expected_type)) in args.iter_mut().zip(&self.argument_types).enumerate()
        {
            let actual_type = arg_expr.infer_type(type_env, stdlib, struct_typedefs)?;
            if !actual_type.coerces(expected_type, true) {
                return Err(WdlError::Validation {
                    pos: arg_expr.source_position().clone(),
                    message: format!(
                        "Function '{}' argument {} expects type {}, got {}",
                        self.name,
                        i + 1,
                        expected_type,
                        actual_type
                    ),
                    source_text: None,
                    declared_wdl_version: None,
                });
            }
        }

        Ok(self.return_type.clone())
    }

    fn eval(
        &self,
        args: &[Expression],
        env: &Bindings<Value>,
        stdlib: &StdLib,
    ) -> Result<Value, WdlError> {
        // Evaluate and coerce arguments using the utility function
        let evaluated_args =
            evaluate_and_coerce_args(args, &self.argument_types, env, stdlib, &self.name)?;

        // Call the implementation function
        let result = (self.implementation)(&evaluated_args)?;

        // Coerce result to expected return type
        result
            .coerce(&self.return_type)
            .map_err(|_| WdlError::RuntimeError {
                message: format!(
                    "Function '{}' result cannot be coerced to return type {}",
                    self.name, self.return_type
                ),
            })
    }

    fn name(&self) -> &str {
        &self.name
    }
}

/// Create a static function with the given name, argument types, return type, and implementation
///
/// This is a convenience function similar to miniwdl's StaticFunction that allows easy creation
/// of simple functions that operate on Values and return Values.
///
/// # Arguments
/// * `name` - The function name
/// * `argument_types` - Vector of expected argument types
/// * `return_type` - Expected return type
/// * `implementation` - Function that takes a slice of Values and returns a Result<Value, WdlError>
///
/// # Example
/// ```rust
/// use miniwdl_rust::stdlib::create_static_function;
/// use miniwdl_rust::{Type, Value, WdlError};
/// let _floor_fn = create_static_function(
///     "floor".to_string(),
///     vec![Type::float(false)],
///     Type::int(false),
///     |args| -> Result<Value, WdlError> {
///         let value = args[0].as_float().unwrap();
///         Ok(Value::int(value.floor() as i64))
///     }
/// );
/// ```
pub fn create_static_function<F>(
    name: String,
    argument_types: Vec<Type>,
    return_type: Type,
    implementation: F,
) -> Box<dyn Function>
where
    F: Fn(&[Value]) -> Result<Value, WdlError> + Send + Sync + 'static,
{
    Box::new(StaticFunction::new(
        name,
        argument_types,
        return_type,
        implementation,
    ))
}

/// Evaluate and coerce expressions to values with the expected types
///
/// This utility function is used by Function implementations to convert Expression
/// arguments to Values with proper type coercion and error handling.
///
/// # Arguments
/// * `args` - Array of Expression arguments
/// * `expected_types` - Expected types for each argument
/// * `env` - Environment for expression evaluation
/// * `stdlib` - Standard library for function calls
/// * `function_name` - Name of the calling function (for error messages)
///
/// # Returns
/// Vector of Values coerced to the expected types, or WdlError on failure
pub fn evaluate_and_coerce_args(
    args: &[Expression],
    expected_types: &[Type],
    env: &Bindings<Value>,
    stdlib: &StdLib,
    function_name: &str,
) -> Result<Vec<Value>, WdlError> {
    // Check argument count
    if args.len() != expected_types.len() {
        return Err(WdlError::RuntimeError {
            message: format!(
                "Function '{}' expects {} arguments, got {}",
                function_name,
                expected_types.len(),
                args.len()
            ),
        });
    }

    // Evaluate and coerce arguments
    let mut evaluated_args = Vec::new();
    for (i, (arg_expr, expected_type)) in args.iter().zip(expected_types).enumerate() {
        let arg_value = arg_expr.eval(env, stdlib)?;
        let coerced_value =
            arg_value
                .coerce(expected_type)
                .map_err(|_| WdlError::RuntimeError {
                    message: format!(
                        "Function '{}' argument {} cannot be coerced to type {}",
                        function_name,
                        i + 1,
                        expected_type
                    ),
                })?;
        evaluated_args.push(coerced_value);
    }

    Ok(evaluated_args)
}

/// Standard library containing all built-in functions and operators
pub struct StdLib {
    functions: HashMap<String, Box<dyn Function>>,
    wdl_version: String,
    path_mapper: Box<dyn PathMapper>,
    is_task_context: bool,
    write_dir: String,
}

impl StdLib {
    /// Create a new standard library instance for the given WDL version
    pub fn new(wdl_version: &str) -> Self {
        Self::with_path_mapper(
            wdl_version,
            Box::new(DefaultPathMapper),
            false,
            std::env::temp_dir().to_string_lossy().to_string(),
        )
    }

    /// Create a standard library instance with custom path mapper and context
    pub fn with_path_mapper(
        wdl_version: &str,
        path_mapper: Box<dyn PathMapper>,
        is_task_context: bool,
        write_dir: String,
    ) -> Self {
        let mut stdlib = StdLib {
            functions: HashMap::new(),
            wdl_version: wdl_version.to_string(),
            path_mapper,
            is_task_context,
            write_dir,
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
        self.register_function(math::create_floor_function());
        self.register_function(math::create_ceil_function());
        self.register_function(math::create_round_function());
        self.register_function(math::create_min_function());
        self.register_function(math::create_max_function());

        // Array functions
        self.register_function(arrays::create_length_function());
        self.register_function(arrays::create_select_first_function());
        self.register_function(arrays::create_select_all_function());
        self.register_function(arrays::create_flatten_function());
        self.register_function(arrays::create_range_function());
        self.register_function(arrays::create_prefix_function());
        self.register_function(arrays::create_suffix_function());
        self.register_function(arrays::create_zip_function());
        self.register_function(arrays::create_cross_function());
        self.register_function(arrays::create_unzip_function());
        self.register_function(arrays::create_quote_function());
        self.register_function(arrays::create_squote_function());
        self.register_function(arrays::create_transpose_function());

        // Map functions
        self.register_function(map::create_keys_function());
        self.register_function(map::create_as_pairs_function());
        self.register_function(map::create_as_map_function());
        self.register_function(map::create_collect_by_key_function());
        self.register_function(map::create_contains_key_function());
        self.register_function(map::create_values_function());

        // String functions
        self.register_function(strings::create_find_function());
        self.register_function(strings::create_sub_function());
        self.register_function(strings::create_basename_function());
        self.register_function(strings::create_join_paths_function());
        self.register_function(strings::create_sep_function());

        // Type functions
        self.register_function(strings::create_defined_function());

        // I/O functions
        // stdout() and stderr() are only available in task output context
        if self.is_task_context {
            self.register_function(io::create_stdout_function());
            self.register_function(io::create_stderr_function());

            // glob() function is only available in task context
            self.register_function(io::create_glob_function(self.path_mapper.clone_boxed()));
        }

        // size() function is available in all contexts
        self.register_function(io::create_size_function(self.path_mapper.clone_boxed()));

        // Write functions (require PathMapper and write_dir)
        self.register_write_functions();

        // Read functions (require PathMapper)
        self.register_read_functions();

        // File system functions
        // self.register_function(io::create_size());
    }

    /// Register all operators
    fn register_operators(&mut self) {
        // Arithmetic operators
        self.register_function(operators::create_add_function()); // Special add operator with string concatenation
        self.register_function(operators::create_sub_function()); // Standard subtraction
        self.register_function(operators::create_mul_function()); // Standard multiplication
        self.register_function(operators::create_div_function()); // Standard division
        self.register_function(operators::create_rem_function()); // Remainder (modulo)

        // Comparison operators
        self.register_function(operators::create_lt_function()); // Less than (<)
        self.register_function(operators::create_lte_function()); // Less than or equal (<=)
        self.register_function(operators::create_gt_function()); // Greater than (>)
        self.register_function(operators::create_gte_function()); // Greater than or equal (>=)

        // Equality operators
        self.register_function(operators::create_eqeq_function()); // Equal (==)
        self.register_function(operators::create_neq_function()); // Not equal (!=)

        // Logical operators
        self.register_function(operators::create_logical_and_function()); // Logical AND (&&)
        self.register_function(operators::create_logical_or_function()); // Logical OR (||)
        self.register_function(operators::create_logical_not_function()); // Logical NOT (!)

        // Unary operators
        self.register_function(operators::create_negate_function()); // Numeric negation (unary -)
    }

    /// Register a function with the library
    fn register_function(&mut self, function: Box<dyn Function>) {
        let name = function.name().to_string();
        self.functions.insert(name, function);
    }

    /// Register read functions
    fn register_read_functions(&mut self) {
        // Register read functions with PathMapper
        self.register_function(io::create_read_string_function(
            self.path_mapper.clone_boxed(),
        ));
        self.register_function(io::create_read_int_function(self.path_mapper.clone_boxed()));
        self.register_function(io::create_read_float_function(
            self.path_mapper.clone_boxed(),
        ));
        self.register_function(io::create_read_boolean_function(
            self.path_mapper.clone_boxed(),
        ));
    }

    /// Register write functions
    fn register_write_functions(&mut self) {
        // Register write functions with PathMapper and write_dir
        self.register_function(io::create_write_lines_function(
            self.path_mapper.clone_boxed(),
            self.write_dir.clone(),
        ));
        self.register_function(io::create_write_tsv_function(
            self.path_mapper.clone_boxed(),
            self.write_dir.clone(),
        ));
        self.register_function(io::create_write_map_function(
            self.path_mapper.clone_boxed(),
            self.write_dir.clone(),
        ));
        self.register_function(io::create_write_json_function(
            self.path_mapper.clone_boxed(),
            self.write_dir.clone(),
        ));
    }

    /// Add or replace a function in the library (public method)
    pub fn add_function(&mut self, function: Box<dyn Function>) {
        self.register_function(function);
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::error::SourcePosition;
    use crate::expr::Expression;

    #[test]
    fn test_stdlib_creation() {
        let stdlib = StdLib::new("1.0");

        // Test that stdlib can be created
        assert_eq!(stdlib.wdl_version(), "1.0");

        // Test that math functions are registered
        assert!(stdlib.get_function("floor").is_some());
        assert!(stdlib.get_function("ceil").is_some());
        assert!(stdlib.get_function("round").is_some());
        assert!(stdlib.get_function("min").is_some());
        assert!(stdlib.get_function("max").is_some());

        // Test that arithmetic operators are registered
        assert!(stdlib.get_function("_add").is_some());
        assert!(stdlib.get_function("_sub").is_some());
        assert!(stdlib.get_function("_mul").is_some());
        assert!(stdlib.get_function("_div").is_some());
        assert!(stdlib.get_function("_rem").is_some());

        // Test that read functions are registered
        assert!(stdlib.get_function("read_string").is_some());
        assert!(stdlib.get_function("read_int").is_some());
        assert!(stdlib.get_function("read_float").is_some());
        assert!(stdlib.get_function("read_boolean").is_some());

        // Test that write functions are registered
        assert!(stdlib.get_function("write_lines").is_some());
        assert!(stdlib.get_function("write_tsv").is_some());
        assert!(stdlib.get_function("write_map").is_some());
        assert!(stdlib.get_function("write_json").is_some());

        // Test that array functions are registered
        assert!(stdlib.get_function("length").is_some());
        assert!(stdlib.get_function("select_first").is_some());
        assert!(stdlib.get_function("select_all").is_some());

        // Test that non-existent functions are not registered
        assert!(stdlib.get_function("nonexistent").is_none());
    }

    #[test]
    fn test_create_static_function() {
        // Create a simple floor function
        let floor_fn = create_static_function(
            "floor".to_string(),
            vec![Type::float(false)],
            Type::int(false),
            |args| {
                let value = args[0].as_float().unwrap();
                Ok(Value::int(value.floor() as i64))
            },
        );

        assert_eq!(floor_fn.name(), "floor");

        // Test type inference
        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);
        let mut arg_exprs = vec![Expression::float(pos.clone(), 3.7)];
        let type_env = Bindings::new();
        let stdlib = StdLib::new("1.0");

        let result_type = floor_fn
            .infer_type(&mut arg_exprs, &type_env, &stdlib, &[])
            .unwrap();
        assert_eq!(result_type, Type::int(false));

        // Test evaluation
        let arg_exprs = vec![Expression::float(pos, 3.7)];
        let value_env = Bindings::new();
        let result = floor_fn.eval(&arg_exprs, &value_env, &stdlib).unwrap();
        assert_eq!(result.as_int().unwrap(), 3);
    }

    #[test]
    fn test_static_function_wrong_arity() {
        let floor_fn = create_static_function(
            "floor".to_string(),
            vec![Type::float(false)],
            Type::int(false),
            |args| {
                let value = args[0].as_float().unwrap();
                Ok(Value::int(value.floor() as i64))
            },
        );

        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);
        let mut arg_exprs = vec![
            Expression::float(pos.clone(), 3.7),
            Expression::float(pos, 4.2), // Too many args
        ];
        let type_env = Bindings::new();
        let stdlib = StdLib::new("1.0");

        let result = floor_fn.infer_type(&mut arg_exprs, &type_env, &stdlib, &[]);
        assert!(result.is_err());
    }
}
