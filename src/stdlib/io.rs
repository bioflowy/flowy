//! Input/Output functions for WDL standard library

use super::Function;
use crate::error::WdlError;
use crate::types::Type;
use crate::value::Value;

/// Stdout function - returns reference to stdout file
pub struct StdoutFunction;

impl Function for StdoutFunction {
    fn name(&self) -> &str {
        "stdout"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if !args.is_empty() {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 0,
                actual: args.len(),
            });
        }

        Ok(Type::file(false))
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        if !args.is_empty() {
            return Err(WdlError::RuntimeError {
                message: "stdout() takes no arguments".to_string(),
            });
        }

        // Default implementation - should not be called in task context
        Value::file("stdout.txt".to_string())
    }

    fn eval_with_stdlib(
        &self,
        args: &[Value],
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Value, WdlError> {
        if !args.is_empty() {
            return Err(WdlError::RuntimeError {
                message: "stdout() takes no arguments".to_string(),
            });
        }

        // Use path mapper to resolve stdout path
        let real_path = stdlib.path_mapper().devirtualize_filename("stdout.txt")?;
        let virtual_path = stdlib.path_mapper().virtualize_filename(&real_path)?;
        Value::file(virtual_path)
    }
}

/// Stderr function - returns reference to stderr file
pub struct StderrFunction;

impl Function for StderrFunction {
    fn name(&self) -> &str {
        "stderr"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if !args.is_empty() {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 0,
                actual: args.len(),
            });
        }

        Ok(Type::file(false))
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        if !args.is_empty() {
            return Err(WdlError::RuntimeError {
                message: "stderr() takes no arguments".to_string(),
            });
        }

        // Default implementation - should not be called in task context
        Value::file("stderr.txt".to_string())
    }

    fn eval_with_stdlib(
        &self,
        args: &[Value],
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Value, WdlError> {
        if !args.is_empty() {
            return Err(WdlError::RuntimeError {
                message: "stderr() takes no arguments".to_string(),
            });
        }

        // Use path mapper to resolve stderr path
        let real_path = stdlib.path_mapper().devirtualize_filename("stderr.txt")?;
        let virtual_path = stdlib.path_mapper().virtualize_filename(&real_path)?;
        Value::file(virtual_path)
    }
}

/// Write lines function - writes array of strings to a file
pub struct WriteLinesFunction;

impl Function for WriteLinesFunction {
    fn name(&self) -> &str {
        "write_lines"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 1,
                actual: args.len(),
            });
        }

        // Expect Array[String]
        if !matches!(args[0], Type::Array { .. }) {
            return Err(WdlError::RuntimeError {
                message: "write_lines() argument must be Array[String]".to_string(),
            });
        }

        Ok(Type::file(false))
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        // Default implementation - create temp file without path mapping
        use std::io::Write;

        let array = args[0].as_array().ok_or_else(|| WdlError::RuntimeError {
            message: "write_lines() argument must be Array[String]".to_string(),
        })?;

        // Create a temporary file
        let mut temp_file = std::env::temp_dir();
        temp_file.push(format!(
            "write_lines_{}.txt",
            std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap_or_default()
                .as_nanos()
        ));

        let mut file = std::fs::File::create(&temp_file).map_err(|e| WdlError::RuntimeError {
            message: format!("Failed to create temporary file: {}", e),
        })?;

        // Write each line
        for value in array {
            let line = match value {
                Value::String { value, .. } => value.clone(),
                _ => value.to_string(),
            };
            writeln!(file, "{}", line).map_err(|e| WdlError::RuntimeError {
                message: format!("Failed to write to file: {}", e),
            })?;
        }

        Value::file(temp_file.to_string_lossy().to_string())
    }

    fn eval_with_stdlib(
        &self,
        args: &[Value],
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Value, WdlError> {
        use std::io::Write;

        let array = args[0].as_array().ok_or_else(|| WdlError::RuntimeError {
            message: "write_lines() argument must be Array[String]".to_string(),
        })?;

        // Create a temporary file
        let mut temp_file = std::env::temp_dir();
        temp_file.push(format!(
            "write_lines_{}.txt",
            std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap_or_default()
                .as_nanos()
        ));

        let mut file = std::fs::File::create(&temp_file).map_err(|e| WdlError::RuntimeError {
            message: format!("Failed to create temporary file: {}", e),
        })?;

        // Write each line
        for value in array {
            let line = match value {
                Value::String { value, .. } => value.clone(),
                _ => value.to_string(),
            };
            writeln!(file, "{}", line).map_err(|e| WdlError::RuntimeError {
                message: format!("Failed to write to file: {}", e),
            })?;
        }

        // Use path mapper to virtualize the filename
        let virtual_path = stdlib.path_mapper().virtualize_filename(&temp_file)?;
        Value::file(virtual_path)
    }
}

/// Read lines function - reads lines from a file into an array
pub struct ReadLinesFunction;

impl Function for ReadLinesFunction {
    fn name(&self) -> &str {
        "read_lines"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 1,
                actual: args.len(),
            });
        }

        // Expect String or File
        if !matches!(args[0], Type::String { .. } | Type::File { .. }) {
            return Err(WdlError::RuntimeError {
                message: "read_lines() argument must be String or File".to_string(),
            });
        }

        Ok(Type::array(Type::string(false), false, false))
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        // Default implementation for backward compatibility
        let filename = match &args[0] {
            Value::String { value, .. } => value.clone(),
            Value::File { value, .. } => value.clone(),
            _ => {
                return Err(WdlError::RuntimeError {
                    message: "read_lines() argument must be String or File".to_string(),
                })
            }
        };

        // Read the file and split into lines (without path mapping)
        match std::fs::read_to_string(&filename) {
            Ok(content) => {
                let lines: Vec<Value> = content
                    .lines()
                    .map(|line| Value::string(line.to_string()))
                    .collect();
                let result = Value::array(Type::string(false), lines);
                Ok(result)
            }
            Err(e) => Err(WdlError::RuntimeError {
                message: format!("Failed to read file {}: {}", filename, e),
            }),
        }
    }

    fn eval_with_stdlib(
        &self,
        args: &[Value],
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Value, WdlError> {
        let filename = match &args[0] {
            Value::String { value, .. } => value.clone(),
            Value::File { value, .. } => value.clone(),
            _ => {
                return Err(WdlError::RuntimeError {
                    message: "read_lines() argument must be String or File".to_string(),
                })
            }
        };

        // Use path mapper to devirtualize filename
        let real_path = stdlib.path_mapper().devirtualize_filename(&filename)?;

        // Read the file and split into lines
        match std::fs::read_to_string(&real_path) {
            Ok(content) => {
                let lines: Vec<Value> = content
                    .lines()
                    .map(|line| Value::string(line.to_string()))
                    .collect();
                let result = Value::array(Type::string(false), lines);
                Ok(result)
            }
            Err(e) => Err(WdlError::RuntimeError {
                message: format!("Failed to read file {}: {}", real_path.display(), e),
            }),
        }
    }
}

/// Read string function - reads entire file content as a single string
pub struct ReadStringFunction;

impl Function for ReadStringFunction {
    fn name(&self) -> &str {
        "read_string"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 1,
                actual: args.len(),
            });
        }

        // Expect String or File
        if !matches!(args[0], Type::String { .. } | Type::File { .. }) {
            return Err(WdlError::RuntimeError {
                message: "read_string() argument must be String or File".to_string(),
            });
        }

        Ok(Type::string(false))
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        // Default implementation for backward compatibility - should not be called
        // when stdlib context is available
        let filename = match &args[0] {
            Value::String { value, .. } => value.clone(),
            Value::File { value, .. } => value.clone(),
            _ => {
                return Err(WdlError::RuntimeError {
                    message: "read_string() argument must be String or File".to_string(),
                })
            }
        };

        // Read the entire file as a string (without path mapping)
        match std::fs::read_to_string(&filename) {
            Ok(content) => Ok(Value::string(content)),
            Err(e) => Err(WdlError::RuntimeError {
                message: format!("Failed to read file {}: {}", filename, e),
            }),
        }
    }

    fn eval_with_stdlib(
        &self,
        args: &[Value],
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Value, WdlError> {
        let filename = match &args[0] {
            Value::String { value, .. } => value.clone(),
            Value::File { value, .. } => value.clone(),
            _ => {
                return Err(WdlError::RuntimeError {
                    message: "read_string() argument must be String or File".to_string(),
                })
            }
        };

        // Use path mapper to devirtualize filename
        let real_path = stdlib.path_mapper().devirtualize_filename(&filename)?;

        // Read the entire file as a string
        match std::fs::read_to_string(&real_path) {
            Ok(content) => Ok(Value::string(content)),
            Err(e) => Err(WdlError::RuntimeError {
                message: format!("Failed to read file {}: {}", real_path.display(), e),
            }),
        }
    }
}

/// Read int function - reads a file and parses content as an integer
pub struct ReadIntFunction;

impl Function for ReadIntFunction {
    fn name(&self) -> &str {
        "read_int"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 1,
                actual: args.len(),
            });
        }

        // Expect String or File
        if !matches!(args[0], Type::String { .. } | Type::File { .. }) {
            return Err(WdlError::RuntimeError {
                message: "read_int() argument must be String or File".to_string(),
            });
        }

        Ok(Type::int(false))
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        // Default implementation for backward compatibility
        let filename = match &args[0] {
            Value::String { value, .. } => value.clone(),
            Value::File { value, .. } => value.clone(),
            _ => {
                return Err(WdlError::RuntimeError {
                    message: "read_int() argument must be String or File".to_string(),
                })
            }
        };

        // Read the file and parse as integer (without path mapping)
        match std::fs::read_to_string(&filename) {
            Ok(content) => {
                let trimmed = content.trim();
                match trimmed.parse::<i64>() {
                    Ok(value) => Ok(Value::int(value)),
                    Err(e) => Err(WdlError::RuntimeError {
                        message: format!("Failed to parse '{}' as integer: {}", trimmed, e),
                    }),
                }
            }
            Err(e) => Err(WdlError::RuntimeError {
                message: format!("Failed to read file {}: {}", filename, e),
            }),
        }
    }

    fn eval_with_stdlib(
        &self,
        args: &[Value],
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Value, WdlError> {
        let filename = match &args[0] {
            Value::String { value, .. } => value.clone(),
            Value::File { value, .. } => value.clone(),
            _ => {
                return Err(WdlError::RuntimeError {
                    message: "read_int() argument must be String or File".to_string(),
                })
            }
        };

        // Use path mapper to devirtualize filename
        let real_path = stdlib.path_mapper().devirtualize_filename(&filename)?;

        // Read the file and parse as integer
        match std::fs::read_to_string(&real_path) {
            Ok(content) => {
                let trimmed = content.trim();
                match trimmed.parse::<i64>() {
                    Ok(value) => Ok(Value::int(value)),
                    Err(e) => Err(WdlError::RuntimeError {
                        message: format!("Failed to parse '{}' as integer: {}", trimmed, e),
                    }),
                }
            }
            Err(e) => Err(WdlError::RuntimeError {
                message: format!("Failed to read file {}: {}", real_path.display(), e),
            }),
        }
    }
}
