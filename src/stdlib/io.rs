//! I/O functions for the WDL standard library
//!
//! This module provides file reading and writing functions similar to miniwdl's I/O functions.

use std::fs;
use crate::error::WdlError;
use crate::stdlib::Function;
use crate::types::Type;
use crate::value::Value;

/// Create a read function implementation based on a parse function
///
/// This is similar to miniwdl's _read() method that generates read_* function
/// implementations based on a parse function.
///
/// # Arguments
/// * `name` - Function name (e.g., "read_string", "read_int")
/// * `return_type` - The expected return type
/// * `parse` - Function that parses the file content string into a Value
/// * `path_mapper` - PathMapper instance for file virtualization
///
/// # Returns
/// A Function implementation that reads files and parses their content
fn create_read_function<F>(
    name: String,
    return_type: Type,
    parse: F,
    path_mapper: Box<dyn crate::stdlib::PathMapper>,
) -> Box<dyn Function>
where
    F: Fn(&str) -> Result<Value, WdlError> + Send + Sync + 'static,
{
    use crate::stdlib::create_static_function;

    create_static_function(
        name,
        vec![Type::file(false)],
        return_type,
        move |args| {
            // args[0] is guaranteed to be a File value due to create_static_function's type checking
            let file_value = &args[0];

            // Get the filename from the File value
            let virtual_filename = file_value.as_string().ok_or_else(|| {
                WdlError::RuntimeError {
                    message: "Invalid file value".to_string(),
                }
            })?;

            // Use PathMapper to devirtualize the filename
            let real_filename = path_mapper.devirtualize_filename(virtual_filename)?;

            // Read the file content
            let content = fs::read_to_string(&real_filename).map_err(|e| {
                WdlError::RuntimeError {
                    message: format!("Failed to read file '{}': {}", real_filename.display(), e),
                }
            })?;

            // Parse the content using the provided parse function
            parse(&content)
        }
    )
}

/// Create read_string function: read_string(File) -> String
/// Reads a file and returns its content as a string, with trailing newline removed
pub fn create_read_string_function(path_mapper: Box<dyn crate::stdlib::PathMapper>) -> Box<dyn Function> {
    create_read_function(
        "read_string".to_string(),
        Type::string(false),
        |content: &str| {
            // Remove trailing newline like miniwdl does
            let trimmed = if content.ends_with('\n') {
                &content[..content.len() - 1]
            } else {
                content
            };
            Ok(Value::string(trimmed.to_string()))
        },
        path_mapper,
    )
}

/// Create read_int function: read_int(File) -> Int
/// Reads a file and parses its content as an integer
pub fn create_read_int_function(path_mapper: Box<dyn crate::stdlib::PathMapper>) -> Box<dyn Function> {
    create_read_function(
        "read_int".to_string(),
        Type::int(false),
        |content: &str| {
            let trimmed = content.trim();
            let value = trimmed.parse::<i64>().map_err(|e| {
                WdlError::RuntimeError {
                    message: format!("Cannot parse '{}' as integer: {}", trimmed, e),
                }
            })?;
            Ok(Value::int(value))
        },
        path_mapper,
    )
}

/// Create read_float function: read_float(File) -> Float
/// Reads a file and parses its content as a float
pub fn create_read_float_function(path_mapper: Box<dyn crate::stdlib::PathMapper>) -> Box<dyn Function> {
    create_read_function(
        "read_float".to_string(),
        Type::float(false),
        |content: &str| {
            let trimmed = content.trim();
            let value = trimmed.parse::<f64>().map_err(|e| {
                WdlError::RuntimeError {
                    message: format!("Cannot parse '{}' as float: {}", trimmed, e),
                }
            })?;
            Ok(Value::float(value))
        },
        path_mapper,
    )
}

/// Create read_boolean function: read_boolean(File) -> Boolean
/// Reads a file and parses its content as a boolean
pub fn create_read_boolean_function(path_mapper: Box<dyn crate::stdlib::PathMapper>) -> Box<dyn Function> {
    create_read_function(
        "read_boolean".to_string(),
        Type::boolean(false),
        |content: &str| {
            let trimmed = content.trim().to_lowercase();
            let value = match trimmed.as_str() {
                "true" | "1" => true,
                "false" | "0" => false,
                _ => {
                    return Err(WdlError::RuntimeError {
                        message: format!("Cannot parse '{}' as boolean", trimmed),
                    });
                }
            };
            Ok(Value::boolean(value))
        },
        path_mapper,
    )
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::stdlib::{StdLib, DefaultPathMapper};
    use crate::expr::{Expression, StringPart};
    use crate::env::Bindings;
    use crate::error::SourcePosition;
    use std::fs;
    use std::io::Write;
    use tempfile::NamedTempFile;

    #[test]
    fn test_read_string_valid() {
        let mut temp_file = NamedTempFile::new().unwrap();
        writeln!(temp_file, "hello world").unwrap();
        let temp_path = temp_file.path().to_str().unwrap();

        let path_mapper = Box::new(DefaultPathMapper);
        let read_string_fn = create_read_string_function(path_mapper);

        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);
        let file_expr = Expression::string(pos, vec![StringPart::Text(temp_path.to_string())]);
        let args = vec![file_expr];
        let env = Bindings::new();
        let stdlib = StdLib::new("1.0");

        let result = read_string_fn.eval(&args, &env, &stdlib).unwrap();
        assert_eq!(result.as_string().unwrap(), "hello world");
    }

    #[test]
    fn test_read_int_valid() {
        let mut temp_file = NamedTempFile::new().unwrap();
        writeln!(temp_file, "42").unwrap();
        let temp_path = temp_file.path().to_str().unwrap();

        let path_mapper = Box::new(DefaultPathMapper);
        let read_int_fn = create_read_int_function(path_mapper);

        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);
        let file_expr = Expression::string(pos, vec![StringPart::Text(temp_path.to_string())]);
        let args = vec![file_expr];
        let env = Bindings::new();
        let stdlib = StdLib::new("1.0");

        let result = read_int_fn.eval(&args, &env, &stdlib).unwrap();
        assert_eq!(result.as_int().unwrap(), 42);
    }

    #[test]
    fn test_read_float_valid() {
        let mut temp_file = NamedTempFile::new().unwrap();
        writeln!(temp_file, "3.14").unwrap();
        let temp_path = temp_file.path().to_str().unwrap();

        let path_mapper = Box::new(DefaultPathMapper);
        let read_float_fn = create_read_float_function(path_mapper);

        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);
        let file_expr = Expression::string(pos, vec![StringPart::Text(temp_path.to_string())]);
        let args = vec![file_expr];
        let env = Bindings::new();
        let stdlib = StdLib::new("1.0");

        let result = read_float_fn.eval(&args, &env, &stdlib).unwrap();
        assert!((result.as_float().unwrap() - 3.14).abs() < f64::EPSILON);
    }

    #[test]
    fn test_read_boolean_valid() {
        let mut temp_file = NamedTempFile::new().unwrap();
        writeln!(temp_file, "true").unwrap();
        let temp_path = temp_file.path().to_str().unwrap();

        let path_mapper = Box::new(DefaultPathMapper);
        let read_boolean_fn = create_read_boolean_function(path_mapper);

        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 5);
        let file_expr = Expression::string(pos, vec![StringPart::Text(temp_path.to_string())]);
        let args = vec![file_expr];
        let env = Bindings::new();
        let stdlib = StdLib::new("1.0");

        let result = read_boolean_fn.eval(&args, &env, &stdlib).unwrap();
        assert_eq!(result.as_bool().unwrap(), true);
    }
}