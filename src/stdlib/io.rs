//! I/O functions for the WDL standard library
//!
//! This module provides file reading and writing functions similar to miniwdl's I/O functions.

use crate::error::{SourcePosition, WdlError};
use crate::expr::ExpressionBase;
use crate::stdlib::Function;
use crate::types::Type;
use crate::value::{Value, ValueBase};
use std::fs;
use std::io::Write;
use std::os::unix::fs::PermissionsExt;

/// Write file function that follows the original WriteFileFunction pattern
/// Uses eval_with_stdlib for proper file handling in different contexts
pub struct WriteFileFunction<F>
where
    F: Fn(&Value, &mut dyn Write) -> Result<(), WdlError> + Send + Sync + 'static,
{
    name: String,
    argument_type: Type,
    serializer: Box<F>,
    path_mapper: Box<dyn crate::stdlib::PathMapper>,
    write_dir: String,
}

impl<F> Function for WriteFileFunction<F>
where
    F: Fn(&Value, &mut dyn Write) -> Result<(), WdlError> + Send + Sync + 'static,
{
    fn infer_type(
        &self,
        args: &mut [crate::expr::Expression],
        type_env: &crate::env::Bindings<Type>,
        stdlib: &crate::stdlib::StdLib,
        struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<Type, WdlError> {
        if args.len() != 1 {
            let pos = if args.is_empty() {
                SourcePosition::new("unknown".to_string(), "unknown".to_string(), 0, 0, 0, 0)
            } else {
                args[0].source_position().clone()
            };
            return Err(WdlError::Validation {
                pos,
                message: format!("{} expects 1 argument, got {}", self.name, args.len()),
                source_text: None,
                declared_wdl_version: None,
            });
        }

        let arg_type = args[0].infer_type(type_env, stdlib, struct_typedefs)?;
        if !arg_type.coerces(&self.argument_type, true) {
            let pos = args[0].source_position().clone();
            return Err(WdlError::Validation {
                pos,
                message: format!(
                    "{} expects {}, got {}",
                    self.name, self.argument_type, arg_type
                ),
                source_text: None,
                declared_wdl_version: None,
            });
        }

        Ok(Type::file(false))
    }

    fn name(&self) -> &str {
        &self.name
    }

    fn eval(
        &self,
        args: &[crate::expr::Expression],
        env: &crate::env::Bindings<Value>,
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Value, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::RuntimeError {
                message: format!("{} expects 1 argument, got {}", self.name, args.len()),
            });
        }

        let value = args[0].eval(env, stdlib)?;

        // Use task directory if available, otherwise use write_dir
        let target_dir = if let Some(task_dir) = stdlib.task_dir() {
            task_dir.as_path()
        } else {
            std::path::Path::new(&self.write_dir)
        };

        // Create target directory if it doesn't exist
        std::fs::create_dir_all(target_dir).map_err(|e| WdlError::RuntimeError {
            message: format!(
                "Failed to create directory '{}': {}",
                target_dir.display(),
                e
            ),
        })?;

        // Create a temporary file in the target directory
        let mut temp_file =
            tempfile::NamedTempFile::new_in(target_dir).map_err(|e| WdlError::RuntimeError {
                message: format!("Failed to create temporary file: {}", e),
            })?;

        let temp_path = temp_file.path().to_path_buf();

        // Serialize the value to the file and handle persistence
        let virtual_filename = {
            (self.serializer)(&value, &mut temp_file)?;
            temp_file.flush().map_err(|e| WdlError::RuntimeError {
                message: format!("Failed to flush file: {}", e),
            })?;

            // Set file permissions (equivalent to chmod 0o660)
            let mut perms = std::fs::metadata(&temp_path)
                .map_err(|e| WdlError::RuntimeError {
                    message: format!("Failed to get file metadata: {}", e),
                })?
                .permissions();
            perms.set_mode(0o660);
            std::fs::set_permissions(&temp_path, perms).map_err(|e| WdlError::RuntimeError {
                message: format!("Failed to set file permissions: {}", e),
            })?;

            // Persist the temp file to prevent automatic deletion
            temp_file
                .persist(&temp_path)
                .map_err(|e| WdlError::RuntimeError {
                    message: format!("Failed to persist temporary file: {}", e),
                })?;

            // Virtualize the filename using PathMapper
            let virtual_name = self.path_mapper.virtualize_filename(&temp_path)?;

            // Debug output to see the actual file paths
            if std::env::var("RUST_BACKTRACE").is_ok() || std::env::var("DEBUG").is_ok() {
                eprintln!(
                    "DEBUG: {} created file: real_path={:?}, virtual_name={}",
                    self.name, temp_path, virtual_name
                );
            }

            virtual_name
        };

        Value::file(virtual_filename)
    }
}

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

    create_static_function(name, vec![Type::file(false)], return_type, move |args| {
        // args[0] is guaranteed to be a File value due to create_static_function's type checking
        let file_value = &args[0];

        // Get the filename from the File value
        let virtual_filename = file_value
            .as_string()
            .ok_or_else(|| WdlError::RuntimeError {
                message: "Invalid file value".to_string(),
            })?;

        // Use PathMapper to devirtualize the filename
        let real_filename = path_mapper.devirtualize_filename(virtual_filename)?;

        // Read the file content
        let content = fs::read_to_string(&real_filename).map_err(|e| WdlError::RuntimeError {
            message: format!("Failed to read file '{}': {}", real_filename.display(), e),
        })?;

        // Parse the content using the provided parse function
        parse(&content)
    })
}

/// Create a write function implementation based on a serialize function
///
/// This is similar to miniwdl's _write() method that generates write_* function
/// implementations based on a serialize function.
///
/// # Arguments
/// * `name` - Function name (e.g., "write_lines", "write_tsv")
/// * `argument_type` - The expected argument type
/// * `serialize` - Function that serializes the value to bytes
/// * `path_mapper` - PathMapper instance for file virtualization
/// * `write_dir` - Directory where files should be written
///
/// # Returns
/// A Function implementation that writes values to files
fn create_write_function<F>(
    name: String,
    argument_type: Type,
    serialize: F,
    path_mapper: Box<dyn crate::stdlib::PathMapper>,
    write_dir: String,
) -> Box<dyn Function>
where
    F: Fn(&Value, &mut dyn Write) -> Result<(), WdlError> + Send + Sync + 'static,
{
    use crate::stdlib::create_static_function;

    Box::new(WriteFileFunction {
        name,
        argument_type,
        serializer: Box::new(serialize),
        path_mapper,
        write_dir,
    })
}

/// Create read_string function: read_string(File) -> String
/// Reads a file and returns its content as a string, with trailing newline removed
pub fn create_read_string_function(
    path_mapper: Box<dyn crate::stdlib::PathMapper>,
) -> Box<dyn Function> {
    create_read_function(
        "read_string".to_string(),
        Type::string(false),
        |content: &str| {
            // Remove trailing newline like miniwdl does
            let trimmed = if let Some(stripped) = content.strip_suffix('\n') {
                stripped
            } else {
                content
            };
            Ok(Value::string(trimmed.to_string()))
        },
        path_mapper,
    )
}

/// Create read_lines function: read_lines(File) -> Array[String]
/// Reads a file and returns its content as an array of lines
pub fn create_read_lines_function(
    path_mapper: Box<dyn crate::stdlib::PathMapper>,
) -> Box<dyn Function> {
    create_read_function(
        "read_lines".to_string(),
        Type::array(Type::string(false), false, false),
        |content: &str| {
            // Split content into lines and convert to Value array
            let lines: Vec<Value> = content
                .lines()
                .map(|line| Value::string(line.to_string()))
                .collect();

            Ok(Value::array(Type::string(false), lines))
        },
        path_mapper,
    )
}

/// Create read_int function: read_int(File) -> Int
/// Reads a file and parses its content as an integer
pub fn create_read_int_function(
    path_mapper: Box<dyn crate::stdlib::PathMapper>,
) -> Box<dyn Function> {
    create_read_function(
        "read_int".to_string(),
        Type::int(false),
        |content: &str| {
            let trimmed = content.trim();
            let value = trimmed.parse::<i64>().map_err(|e| WdlError::RuntimeError {
                message: format!("Cannot parse '{}' as integer: {}", trimmed, e),
            })?;
            Ok(Value::int(value))
        },
        path_mapper,
    )
}

/// Create read_float function: read_float(File) -> Float
/// Reads a file and parses its content as a float
pub fn create_read_float_function(
    path_mapper: Box<dyn crate::stdlib::PathMapper>,
) -> Box<dyn Function> {
    create_read_function(
        "read_float".to_string(),
        Type::float(false),
        |content: &str| {
            let trimmed = content.trim();
            let value = trimmed.parse::<f64>().map_err(|e| WdlError::RuntimeError {
                message: format!("Cannot parse '{}' as float: {}", trimmed, e),
            })?;
            Ok(Value::float(value))
        },
        path_mapper,
    )
}

/// Create read_boolean function: read_boolean(File) -> Boolean
/// Reads a file and parses its content as a boolean
pub fn create_read_boolean_function(
    path_mapper: Box<dyn crate::stdlib::PathMapper>,
) -> Box<dyn Function> {
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

/// Create write_lines function: write_lines(Array[String]) -> File
/// Writes an array of strings to a file, one line per string
pub fn create_write_lines_function(
    path_mapper: Box<dyn crate::stdlib::PathMapper>,
    write_dir: String,
) -> Box<dyn Function> {
    create_write_function(
        "write_lines".to_string(),
        Type::array(Type::string(false), false, false),
        |value: &Value, file: &mut dyn Write| {
            let array = value.as_array().ok_or_else(|| WdlError::RuntimeError {
                message: "Expected array value for write_lines".to_string(),
            })?;

            for item in array {
                let line = item.as_string().ok_or_else(|| WdlError::RuntimeError {
                    message: "All array items must be strings for write_lines".to_string(),
                })?;
                file.write_all(line.as_bytes())
                    .map_err(|e| WdlError::RuntimeError {
                        message: format!("Failed to write line: {}", e),
                    })?;
                file.write_all(b"\n").map_err(|e| WdlError::RuntimeError {
                    message: format!("Failed to write newline: {}", e),
                })?;
            }
            Ok(())
        },
        path_mapper,
        write_dir,
    )
}

/// Create write_tsv function: write_tsv(Array[Array[String]]) -> File
/// Writes a 2D array as tab-separated values
pub fn create_write_tsv_function(
    path_mapper: Box<dyn crate::stdlib::PathMapper>,
    write_dir: String,
) -> Box<dyn Function> {
    create_write_function(
        "write_tsv".to_string(),
        Type::array(Type::array(Type::string(false), false, false), false, false),
        |value: &Value, file: &mut dyn Write| {
            let array = value.as_array().ok_or_else(|| WdlError::RuntimeError {
                message: "Expected array value for write_tsv".to_string(),
            })?;

            for row in array {
                let row_array = row.as_array().ok_or_else(|| WdlError::RuntimeError {
                    message: "Expected array of arrays for write_tsv".to_string(),
                })?;

                let line_parts: Result<Vec<String>, WdlError> = row_array
                    .iter()
                    .map(|cell| {
                        cell.as_string().map(|s| s.to_string()).ok_or_else(|| {
                            WdlError::RuntimeError {
                                message: "All TSV cells must be strings".to_string(),
                            }
                        })
                    })
                    .collect();

                let line = line_parts?.join("\t");
                file.write_all(line.as_bytes())
                    .map_err(|e| WdlError::RuntimeError {
                        message: format!("Failed to write TSV line: {}", e),
                    })?;
                file.write_all(b"\n").map_err(|e| WdlError::RuntimeError {
                    message: format!("Failed to write newline: {}", e),
                })?;
            }
            Ok(())
        },
        path_mapper,
        write_dir,
    )
}

/// Create write_map function: write_map(Map[String, String]) -> File
/// Writes a map as tab-separated key-value pairs
pub fn create_write_map_function(
    path_mapper: Box<dyn crate::stdlib::PathMapper>,
    write_dir: String,
) -> Box<dyn Function> {
    create_write_function(
        "write_map".to_string(),
        Type::map(Type::string(false), Type::string(false), false),
        |value: &Value, file: &mut dyn Write| {
            let pairs = match value {
                Value::Map { pairs, .. } => pairs,
                _ => {
                    return Err(WdlError::RuntimeError {
                        message: "Expected map value for write_map".to_string(),
                    });
                }
            };

            for (key, val) in pairs {
                let key_str = key.as_string().ok_or_else(|| WdlError::RuntimeError {
                    message: "Map keys must be strings for write_map".to_string(),
                })?;
                let val_str = val.as_string().ok_or_else(|| WdlError::RuntimeError {
                    message: "Map values must be strings for write_map".to_string(),
                })?;

                // Check for forbidden characters like miniwdl does
                if key_str.contains('\t')
                    || key_str.contains('\n')
                    || val_str.contains('\t')
                    || val_str.contains('\n')
                {
                    return Err(WdlError::RuntimeError {
                        message:
                            "write_map(): keys & values must not contain tab or newline characters"
                                .to_string(),
                    });
                }

                let line = format!("{}\t{}\n", key_str, val_str);
                file.write_all(line.as_bytes())
                    .map_err(|e| WdlError::RuntimeError {
                        message: format!("Failed to write map line: {}", e),
                    })?;
            }
            Ok(())
        },
        path_mapper,
        write_dir,
    )
}

/// Create write_json function: write_json(Any) -> File
/// Writes any value as JSON
pub fn create_write_json_function(
    path_mapper: Box<dyn crate::stdlib::PathMapper>,
    write_dir: String,
) -> Box<dyn Function> {
    create_write_function(
        "write_json".to_string(),
        Type::any(),
        |value: &Value, file: &mut dyn Write| {
            let json_value = value.to_json();
            let json_string =
                serde_json::to_string(&json_value).map_err(|e| WdlError::RuntimeError {
                    message: format!("Failed to serialize value as JSON: {}", e),
                })?;

            file.write_all(json_string.as_bytes())
                .map_err(|e| WdlError::RuntimeError {
                    message: format!("Failed to write JSON: {}", e),
                })?;
            Ok(())
        },
        path_mapper,
        write_dir,
    )
}

/// Create stdout function: stdout() -> File
/// Returns a file representing standard output (only available in task context)
pub fn create_stdout_function() -> Box<dyn Function> {
    Box::new(StdoutFunction)
}

/// Stdout function implementation that returns the task's stdout.txt file
struct StdoutFunction;

impl Function for StdoutFunction {
    fn name(&self) -> &str {
        "stdout"
    }

    fn infer_type(
        &self,
        args: &mut [crate::expr::Expression],
        _type_env: &crate::env::Bindings<crate::types::Type>,
        _stdlib: &crate::stdlib::StdLib,
        _struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<crate::types::Type, crate::error::WdlError> {
        if !args.is_empty() {
            return Err(crate::error::WdlError::Validation {
                pos: args[0].source_position().clone(),
                message: "stdout() takes no arguments".to_string(),
                source_text: None,
                declared_wdl_version: None,
            });
        }
        Ok(crate::types::Type::file(false))
    }

    fn eval(
        &self,
        args: &[crate::expr::Expression],
        _env: &crate::env::Bindings<crate::value::Value>,
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<crate::value::Value, crate::error::WdlError> {
        if !args.is_empty() {
            return Err(crate::error::WdlError::RuntimeError {
                message: "stdout() takes no arguments".to_string(),
            });
        }

        // Use path mapper to resolve stdout path
        let real_path = stdlib.path_mapper().devirtualize_filename("stdout.txt")?;
        let virtual_path = stdlib.path_mapper().virtualize_filename(&real_path)?;
        crate::value::Value::file(virtual_path)
    }
}

/// Create stderr function: stderr() -> File
/// Returns a file representing standard error (only available in task context)
pub fn create_stderr_function() -> Box<dyn Function> {
    Box::new(StderrFunction)
}

/// Stderr function implementation that returns the task's stderr.txt file
struct StderrFunction;

impl Function for StderrFunction {
    fn name(&self) -> &str {
        "stderr"
    }

    fn infer_type(
        &self,
        args: &mut [crate::expr::Expression],
        _type_env: &crate::env::Bindings<crate::types::Type>,
        _stdlib: &crate::stdlib::StdLib,
        _struct_typedefs: &[crate::tree::StructTypeDef],
    ) -> Result<crate::types::Type, crate::error::WdlError> {
        if !args.is_empty() {
            return Err(crate::error::WdlError::Validation {
                pos: args[0].source_position().clone(),
                message: "stderr() takes no arguments".to_string(),
                source_text: None,
                declared_wdl_version: None,
            });
        }
        Ok(crate::types::Type::file(false))
    }

    fn eval(
        &self,
        args: &[crate::expr::Expression],
        _env: &crate::env::Bindings<crate::value::Value>,
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<crate::value::Value, crate::error::WdlError> {
        if !args.is_empty() {
            return Err(crate::error::WdlError::RuntimeError {
                message: "stderr() takes no arguments".to_string(),
            });
        }

        // Use path mapper to resolve stderr path
        let real_path = stdlib.path_mapper().devirtualize_filename("stderr.txt")?;
        let virtual_path = stdlib.path_mapper().virtualize_filename(&real_path)?;
        crate::value::Value::file(virtual_path)
    }
}

/// Create glob function: glob(pattern: String) -> Array[File]
/// Returns an array of files matching the given glob pattern
pub fn create_glob_function(path_mapper: Box<dyn crate::stdlib::PathMapper>) -> Box<dyn Function> {
    use crate::stdlib::create_static_function;

    create_static_function(
        "glob".to_string(),
        vec![Type::string(false)],
        Type::array(Type::file(false), false, false),
        move |args| {
            let pattern = args[0].as_string().ok_or_else(|| WdlError::RuntimeError {
                message: "glob() requires a string pattern".to_string(),
            })?;

            // Use glob crate to find matching files
            let glob_result = glob::glob(pattern).map_err(|e| WdlError::RuntimeError {
                message: format!("Invalid glob pattern '{}': {}", pattern, e),
            })?;

            let mut files = Vec::new();
            for entry in glob_result {
                match entry {
                    Ok(path) => {
                        // Convert path to string and virtualize through PathMapper
                        let path_str = path.to_string_lossy().to_string();
                        match path_mapper.virtualize_filename(&path) {
                            Ok(virtual_path) => {
                                files.push(Value::file(virtual_path)?);
                            }
                            Err(_) => {
                                // If virtualization fails, use the original path
                                files.push(Value::file(path_str)?);
                            }
                        }
                    }
                    Err(e) => {
                        return Err(WdlError::RuntimeError {
                            message: format!("Glob error: {}", e),
                        });
                    }
                }
            }

            Ok(Value::array(Type::file(false), files))
        },
    )
}

/// Create size function with multiple signatures:
/// - size(file: File?) -> Float
/// - size(file: File?, unit: String) -> Float
/// - size(files: Array[File?]) -> Float
/// - size(files: Array[File?], unit: String) -> Float
///
/// Returns the size of file(s) in bytes or specified unit
pub fn create_size_function(path_mapper: Box<dyn crate::stdlib::PathMapper>) -> Box<dyn Function> {
    use crate::env::Bindings;
    use crate::expr::Expression;
    use crate::stdlib::Function;

    struct SizeFunction {
        path_mapper: Box<dyn crate::stdlib::PathMapper>,
    }

    impl Function for SizeFunction {
        fn name(&self) -> &str {
            "size"
        }

        fn infer_type(
            &self,
            args: &mut [Expression],
            type_env: &Bindings<Type>,
            stdlib: &crate::stdlib::StdLib,
            struct_typedefs: &[crate::tree::StructTypeDef],
        ) -> Result<Type, WdlError> {
            if args.is_empty() || args.len() > 2 {
                return Err(WdlError::Validation {
                    pos: if args.is_empty() {
                        SourcePosition::new(
                            "unknown".to_string(),
                            "unknown".to_string(),
                            0,
                            0,
                            0,
                            0,
                        )
                    } else {
                        args[0].source_position().clone()
                    },
                    message: format!("size() expects 1 or 2 arguments, got {}", args.len()),
                    source_text: None,
                    declared_wdl_version: None,
                });
            }

            // Infer the first argument type
            let first_arg_type = args[0].infer_type(type_env, stdlib, struct_typedefs)?;

            // Check if it's File? or Array[File?]
            match &first_arg_type {
                Type::File { .. } => {
                    // Valid: size(File?) or size(File?, String)
                }
                Type::Array { item_type, .. } => {
                    // Check if it's Array[File?]
                    if !matches!(item_type.as_ref(), Type::File { .. }) {
                        return Err(WdlError::Validation {
                            pos: args[0].source_position().clone(),
                            message: format!(
                                "size() first argument must be File? or Array[File?], got {}",
                                first_arg_type
                            ),
                            source_text: None,
                            declared_wdl_version: None,
                        });
                    }
                }
                _ => {
                    return Err(WdlError::Validation {
                        pos: args[0].source_position().clone(),
                        message: format!(
                            "size() first argument must be File? or Array[File?], got {}",
                            first_arg_type
                        ),
                        source_text: None,
                        declared_wdl_version: None,
                    });
                }
            }

            // Check the optional second argument (unit)
            if args.len() > 1 {
                let unit_type = args[1].infer_type(type_env, stdlib, struct_typedefs)?;
                if !matches!(unit_type, Type::String { .. }) {
                    return Err(WdlError::Validation {
                        pos: args[1].source_position().clone(),
                        message: format!("size() unit argument must be String, got {}", unit_type),
                        source_text: None,
                        declared_wdl_version: None,
                    });
                }
            }

            Ok(Type::float(false))
        }

        fn eval(
            &self,
            args: &[Expression],
            env: &Bindings<Value>,
            stdlib: &crate::stdlib::StdLib,
        ) -> Result<Value, WdlError> {
            if args.is_empty() || args.len() > 2 {
                return Err(WdlError::RuntimeError {
                    message: format!("size() expects 1 or 2 arguments, got {}", args.len()),
                });
            }

            // Define byte unit conversion factors
            let get_unit_factor = |unit: &str| -> Result<f64, WdlError> {
                match unit.to_uppercase().as_str() {
                    "B" | "BYTE" | "BYTES" => Ok(1.0),
                    "KB" | "KILOBYTE" | "KILOBYTES" => Ok(1000.0),
                    "MB" | "MEGABYTE" | "MEGABYTES" => Ok(1_000_000.0),
                    "GB" | "GIGABYTE" | "GIGABYTES" => Ok(1_000_000_000.0),
                    "TB" | "TERABYTE" | "TERABYTES" => Ok(1_000_000_000_000.0),
                    "KIB" | "KIBIBYTE" | "KIBIBYTES" => Ok(1024.0),
                    "MIB" | "MEBIBYTE" | "MEBIBYTES" => Ok(1024.0 * 1024.0),
                    "GIB" | "GIBIBYTE" | "GIBIBYTES" => Ok(1024.0 * 1024.0 * 1024.0),
                    "TIB" | "TEBIBYTE" | "TEBIBYTES" => Ok(1024.0 * 1024.0 * 1024.0 * 1024.0),
                    _ => Err(WdlError::RuntimeError {
                        message: format!("Unknown size unit: {}", unit),
                    }),
                }
            };

            let get_file_size = |file_value: &Value| -> Result<f64, WdlError> {
                // Check if file is null/None (optional values)
                if matches!(file_value, Value::Null) {
                    return Ok(0.0);
                }

                let file_path = file_value
                    .as_string()
                    .ok_or_else(|| WdlError::RuntimeError {
                        message: "size() requires a File argument".to_string(),
                    })?;

                // Devirtualize the file path through PathMapper
                let real_path = self.path_mapper.devirtualize_filename(file_path)?;

                // Get file size
                let metadata =
                    std::fs::metadata(&real_path).map_err(|e| WdlError::RuntimeError {
                        message: format!("Cannot get size of file '{}': {}", file_path, e),
                    })?;

                Ok(metadata.len() as f64)
            };

            // Evaluate arguments
            let first_arg = args[0].eval(env, stdlib)?;

            // Get unit factor if provided
            let unit_factor = if args.len() > 1 {
                let unit_arg = args[1].eval(env, stdlib)?;
                let unit = unit_arg.as_string().ok_or_else(|| WdlError::RuntimeError {
                    message: "size() unit argument must be a string".to_string(),
                })?;
                get_unit_factor(unit)?
            } else {
                1.0 // Default to bytes
            };

            // Handle different first argument types
            let total_size = match &first_arg {
                // Single file case
                file_val if matches!(file_val, Value::File { .. } | Value::Null) => {
                    get_file_size(file_val)?
                }
                // Array of files case
                Value::Array { values, .. } => {
                    let mut total = 0.0;
                    for file_val in values {
                        total += get_file_size(file_val)?;
                    }
                    total
                }
                _ => {
                    return Err(WdlError::RuntimeError {
                        message: "size() first argument must be File? or Array[File?]".to_string(),
                    });
                }
            };

            Ok(Value::float(total_size / unit_factor))
        }
    }

    Box::new(SizeFunction { path_mapper })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::env::Bindings;
    use crate::error::SourcePosition;
    use crate::expr::{Expression, StringPart};
    use crate::stdlib::{DefaultPathMapper, StdLib};
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
    #[allow(clippy::approx_constant)]
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
        assert!(result.as_bool().unwrap());
    }
}
