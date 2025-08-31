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

        // Return stdout.txt file path - this should be overridden by task-specific stdlib
        Value::file("stdout.txt".to_string())
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

        // Return stderr.txt file path - this should be overridden by task-specific stdlib
        Value::file("stderr.txt".to_string())
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
        let _array = args[0].as_array().ok_or_else(|| WdlError::RuntimeError {
            message: "write_lines() argument must be Array[String]".to_string(),
        })?;

        // For now, create a temporary file name - runtime should handle actual file creation
        let filename = format!(
            "__WRITE_LINES_{}.txt",
            chrono::Utc::now().timestamp_nanos_opt().unwrap_or(0)
        );

        // In a real implementation, we would write the lines to the file here
        // For now, just return the filename
        Value::file(filename)
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
        let filename = match &args[0] {
            Value::String { value, .. } => value.clone(),
            Value::File { value, .. } => value.clone(),
            _ => {
                return Err(WdlError::RuntimeError {
                    message: "read_lines() argument must be String or File".to_string(),
                })
            }
        };

        // Read the file and split into lines
        match std::fs::read_to_string(&filename) {
            Ok(content) => {
                let lines: Vec<Value> = content
                    .lines()
                    .map(|line| Value::string(line.to_string()))
                    .collect();
                Ok(Value::array(Type::string(false), lines))
            }
            Err(e) => Err(WdlError::RuntimeError {
                message: format!("Failed to read file {}: {}", filename, e),
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
        let filename = match &args[0] {
            Value::String { value, .. } => value.clone(),
            Value::File { value, .. } => value.clone(),
            _ => {
                return Err(WdlError::RuntimeError {
                    message: "read_string() argument must be String or File".to_string(),
                })
            }
        };

        // Read the entire file as a string
        match std::fs::read_to_string(&filename) {
            Ok(content) => Ok(Value::string(content)),
            Err(e) => Err(WdlError::RuntimeError {
                message: format!("Failed to read file {}: {}", filename, e),
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
        let filename = match &args[0] {
            Value::String { value, .. } => value.clone(),
            Value::File { value, .. } => value.clone(),
            _ => {
                return Err(WdlError::RuntimeError {
                    message: "read_int() argument must be String or File".to_string(),
                })
            }
        };

        // Read the file and parse as integer
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
}
