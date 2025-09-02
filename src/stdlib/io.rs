//! Input/Output functions for WDL standard library

use super::Function;
use crate::error::WdlError;
use crate::types::Type;
use crate::value::Value;
use std::collections::{HashMap, HashSet};

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
            Ok(content) => {
                // Strip trailing \r and \n characters as per WDL spec
                let trimmed = content.trim_end_matches(&['\r', '\n']);
                Ok(Value::string(trimmed.to_string()))
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
                    message: "read_string() argument must be String or File".to_string(),
                })
            }
        };

        // Use path mapper to devirtualize filename
        let real_path = stdlib.path_mapper().devirtualize_filename(&filename)?;

        // Read the entire file as a string
        match std::fs::read_to_string(&real_path) {
            Ok(content) => {
                // Strip trailing \r and \n characters as per WDL spec
                let trimmed = content.trim_end_matches(&['\r', '\n']);
                Ok(Value::string(trimmed.to_string()))
            }
            Err(e) => Err(WdlError::RuntimeError {
                message: format!("Failed to read file {}: {}", real_path.display(), e),
            }),
        }
    }
}

/// Generic read file function that parses file content using a provided parser
pub struct ReadFileFunction<F> {
    name: &'static str,
    return_type: Type,
    parser: F,
}

impl<F> ReadFileFunction<F>
where
    F: Fn(&str) -> Result<Value, WdlError> + Send + Sync + 'static,
{
    pub fn new(name: &'static str, return_type: Type, parser: F) -> Self {
        Self {
            name,
            return_type,
            parser,
        }
    }
}

impl<F> Function for ReadFileFunction<F>
where
    F: Fn(&str) -> Result<Value, WdlError> + Send + Sync,
{
    fn name(&self) -> &str {
        self.name
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

        Ok(self.return_type.clone())
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
                (self.parser)(trimmed)
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
                (self.parser)(trimmed)
            }
            Err(e) => Err(WdlError::RuntimeError {
                message: format!("Failed to read file {}: {}", real_path.display(), e),
            }),
        }
    }
}

/// Helper function to create read_int function
pub fn create_read_int() -> Box<dyn Function> {
    Box::new(ReadFileFunction::new(
        "read_int",
        Type::int(false),
        |content| {
            content
                .parse::<i64>()
                .map(Value::int)
                .map_err(|e| WdlError::RuntimeError {
                    message: format!("Failed to parse '{}' as integer: {}", content, e),
                })
        },
    ))
}

/// Helper function to create read_float function
pub fn create_read_float() -> Box<dyn Function> {
    Box::new(ReadFileFunction::new(
        "read_float",
        Type::float(false),
        |content| {
            content
                .parse::<f64>()
                .map(Value::float)
                .map_err(|e| WdlError::RuntimeError {
                    message: format!("Failed to parse '{}' as float: {}", content, e),
                })
        },
    ))
}

/// Helper function to create read_boolean function
pub fn create_read_boolean() -> Box<dyn Function> {
    Box::new(ReadFileFunction::new(
        "read_boolean",
        Type::boolean(false),
        |content| {
            let lower = content.to_lowercase();
            match lower.as_str() {
                "true" => Ok(Value::boolean(true)),
                "false" => Ok(Value::boolean(false)),
                _ => Err(WdlError::RuntimeError {
                    message: format!(
                        "Failed to parse '{}' as boolean (expected 'true' or 'false')",
                        content
                    ),
                }),
            }
        },
    ))
}

/// Helper function to convert JSON to WDL value (based on main.rs implementation)
fn json_to_wdl_value(json: serde_json::Value) -> Result<Value, WdlError> {
    match json {
        serde_json::Value::Null => Ok(Value::Null),
        serde_json::Value::Bool(b) => Ok(Value::Boolean {
            value: b,
            wdl_type: Type::Boolean { optional: false },
        }),
        serde_json::Value::Number(n) => {
            if let Some(i) = n.as_i64() {
                Ok(Value::Int {
                    value: i,
                    wdl_type: Type::Int { optional: false },
                })
            } else if let Some(f) = n.as_f64() {
                Ok(Value::Float {
                    value: f,
                    wdl_type: Type::Float { optional: false },
                })
            } else {
                Err(WdlError::RuntimeError {
                    message: format!("Invalid number: {}", n),
                })
            }
        }
        serde_json::Value::String(s) => Ok(Value::String {
            value: s,
            wdl_type: Type::String { optional: false },
        }),
        serde_json::Value::Array(arr) => {
            let values: Result<Vec<_>, _> = arr.into_iter().map(json_to_wdl_value).collect();
            let converted_values = values?;

            // Determine the most appropriate array item type
            let item_type = if converted_values.is_empty() {
                Type::String { optional: false }
            } else {
                // Use String as the default item type
                Type::String { optional: false }
            };

            Ok(Value::Array {
                values: converted_values,
                wdl_type: Type::Array {
                    item_type: Box::new(item_type),
                    optional: false,
                    nonempty: false,
                },
            })
        }
        serde_json::Value::Object(map) => {
            let mut wdl_map = Vec::new();
            for (key, value) in map {
                let key_value = Value::String {
                    value: key,
                    wdl_type: Type::String { optional: false },
                };
                let converted_value = json_to_wdl_value(value)?;
                wdl_map.push((key_value, converted_value));
            }
            Ok(Value::Map {
                pairs: wdl_map,
                wdl_type: Type::Map {
                    key_type: Box::new(Type::String { optional: false }),
                    value_type: Box::new(Type::String { optional: false }), // Default to String
                    optional: false,
                    literal_keys: None,
                },
            })
        }
    }
}

/// Helper function to create read_json function
pub fn create_read_json() -> Box<dyn Function> {
    Box::new(ReadFileFunction::new(
        "read_json",
        Type::String { optional: false }, // Will be determined at runtime
        |content| {
            let json_value: serde_json::Value =
                serde_json::from_str(content).map_err(|e| WdlError::RuntimeError {
                    message: format!("Failed to parse JSON: {}", e),
                })?;
            json_to_wdl_value(json_value)
        },
    ))
}

/// Helper function to create read_tsv function
pub fn create_read_tsv() -> Box<dyn Function> {
    Box::new(ReadFileFunction::new(
        "read_tsv",
        Type::Array {
            item_type: Box::new(Type::Array {
                item_type: Box::new(Type::String { optional: false }),
                optional: false,
                nonempty: false,
            }),
            optional: false,
            nonempty: false,
        },
        |content| {
            let mut rows = Vec::new();
            for line in content.lines() {
                if line.trim().is_empty() {
                    continue; // Skip empty lines
                }
                let fields: Vec<Value> = line
                    .split('\t')
                    .map(|field| Value::String {
                        value: field.to_string(),
                        wdl_type: Type::String { optional: false },
                    })
                    .collect();

                rows.push(Value::Array {
                    values: fields,
                    wdl_type: Type::Array {
                        item_type: Box::new(Type::String { optional: false }),
                        optional: false,
                        nonempty: false,
                    },
                });
            }

            Ok(Value::Array {
                values: rows,
                wdl_type: Type::Array {
                    item_type: Box::new(Type::Array {
                        item_type: Box::new(Type::String { optional: false }),
                        optional: false,
                        nonempty: false,
                    }),
                    optional: false,
                    nonempty: false,
                },
            })
        },
    ))
}

/// Helper function to create read_map function
pub fn create_read_map() -> Box<dyn Function> {
    Box::new(ReadFileFunction::new(
        "read_map",
        Type::Map {
            key_type: Box::new(Type::String { optional: false }),
            value_type: Box::new(Type::String { optional: false }),
            optional: false,
            literal_keys: None,
        },
        |content| {
            let mut keys = HashSet::new();
            let mut pairs = Vec::new();

            for line in content.lines() {
                if line.trim().is_empty() {
                    continue; // Skip empty lines
                }
                let fields: Vec<&str> = line.split('\t').collect();
                if fields.len() != 2 {
                    return Err(WdlError::RuntimeError {
                        message: "read_map(): each line must have two fields".to_string(),
                    });
                }

                let key = fields[0].to_string();
                if keys.contains(&key) {
                    return Err(WdlError::RuntimeError {
                        message: "read_map(): duplicate key".to_string(),
                    });
                }
                keys.insert(key.clone());

                let key_value = Value::String {
                    value: key,
                    wdl_type: Type::String { optional: false },
                };
                let value_value = Value::String {
                    value: fields[1].to_string(),
                    wdl_type: Type::String { optional: false },
                };

                pairs.push((key_value, value_value));
            }

            Ok(Value::Map {
                pairs,
                wdl_type: Type::Map {
                    key_type: Box::new(Type::String { optional: false }),
                    value_type: Box::new(Type::String { optional: false }),
                    optional: false,
                    literal_keys: None,
                },
            })
        },
    ))
}

/// Helper function to create read_objects function
pub fn create_read_objects() -> Box<dyn Function> {
    Box::new(ReadFileFunction::new(
        "read_objects",
        Type::Array {
            item_type: Box::new(Type::Map {
                key_type: Box::new(Type::String { optional: false }),
                value_type: Box::new(Type::String { optional: false }),
                optional: false,
                literal_keys: None,
            }),
            optional: false,
            nonempty: false,
        },
        |content| {
            let lines: Vec<&str> = content
                .lines()
                .filter(|line| !line.trim().is_empty())
                .collect();
            if lines.is_empty() {
                return Ok(Value::Array {
                    values: Vec::new(),
                    wdl_type: Type::Array {
                        item_type: Box::new(Type::Map {
                            key_type: Box::new(Type::String { optional: false }),
                            value_type: Box::new(Type::String { optional: false }),
                            optional: false,
                            literal_keys: None,
                        }),
                        optional: false,
                        nonempty: false,
                    },
                });
            }

            // Parse header
            let header: Vec<&str> = lines[0].split('\t').collect();
            let header_set: HashSet<&str> = header.iter().cloned().collect();

            // Check for empty or duplicate column names
            if header_set.len() != header.len() || header.iter().any(|h| h.trim().is_empty()) {
                return Err(WdlError::RuntimeError {
                    message: "read_objects(): file has empty or duplicate column names".to_string(),
                });
            }

            let mut objects = Vec::new();

            // Parse data rows
            for line in &lines[1..] {
                let fields: Vec<&str> = line.split('\t').collect();
                if fields.len() != header.len() {
                    return Err(WdlError::RuntimeError {
                        message: "read_objects(): file's tab-separated lines are ragged"
                            .to_string(),
                    });
                }

                let mut pairs = Vec::new();
                for (key, value) in header.iter().zip(fields.iter()) {
                    let key_value = Value::String {
                        value: key.to_string(),
                        wdl_type: Type::String { optional: false },
                    };
                    let value_value = Value::String {
                        value: value.to_string(),
                        wdl_type: Type::String { optional: false },
                    };
                    pairs.push((key_value, value_value));
                }

                objects.push(Value::Map {
                    pairs,
                    wdl_type: Type::Map {
                        key_type: Box::new(Type::String { optional: false }),
                        value_type: Box::new(Type::String { optional: false }),
                        optional: false,
                        literal_keys: None,
                    },
                });
            }

            Ok(Value::Array {
                values: objects,
                wdl_type: Type::Array {
                    item_type: Box::new(Type::Map {
                        key_type: Box::new(Type::String { optional: false }),
                        value_type: Box::new(Type::String { optional: false }),
                        optional: false,
                        literal_keys: None,
                    }),
                    optional: false,
                    nonempty: false,
                },
            })
        },
    ))
}

/// Helper function to create read_object function
pub fn create_read_object() -> Box<dyn Function> {
    Box::new(ReadFileFunction::new(
        "read_object",
        Type::Map {
            key_type: Box::new(Type::String { optional: false }),
            value_type: Box::new(Type::String { optional: false }),
            optional: false,
            literal_keys: None,
        },
        |content| {
            // Implement read_objects logic inline and ensure exactly one object
            let lines: Vec<&str> = content
                .lines()
                .filter(|line| !line.trim().is_empty())
                .collect();
            if lines.is_empty() {
                return Err(WdlError::RuntimeError {
                    message: "read_object(): file must have exactly one object".to_string(),
                });
            }

            // Parse header
            let header: Vec<&str> = lines[0].split('\t').collect();
            let header_set: HashSet<&str> = header.iter().cloned().collect();

            // Check for empty or duplicate column names
            if header_set.len() != header.len() || header.iter().any(|h| h.trim().is_empty()) {
                return Err(WdlError::RuntimeError {
                    message: "read_object(): file has empty or duplicate column names".to_string(),
                });
            }

            // Ensure exactly one data row
            if lines.len() != 2 {
                return Err(WdlError::RuntimeError {
                    message: "read_object(): file must have exactly one object".to_string(),
                });
            }

            // Parse the single data row
            let fields: Vec<&str> = lines[1].split('\t').collect();
            if fields.len() != header.len() {
                return Err(WdlError::RuntimeError {
                    message: "read_object(): file's tab-separated lines are ragged".to_string(),
                });
            }

            let mut pairs = Vec::new();
            for (key, value) in header.iter().zip(fields.iter()) {
                let key_value = Value::String {
                    value: key.to_string(),
                    wdl_type: Type::String { optional: false },
                };
                let value_value = Value::String {
                    value: value.to_string(),
                    wdl_type: Type::String { optional: false },
                };
                pairs.push((key_value, value_value));
            }

            Ok(Value::Map {
                pairs,
                wdl_type: Type::Map {
                    key_type: Box::new(Type::String { optional: false }),
                    value_type: Box::new(Type::String { optional: false }),
                    optional: false,
                    literal_keys: None,
                },
            })
        },
    ))
}

// For backward compatibility with tests, we keep type aliases
pub type ReadIntFunction = ReadFileFunction<fn(&str) -> Result<Value, WdlError>>;
pub type ReadFloatFunction = ReadFileFunction<fn(&str) -> Result<Value, WdlError>>;
pub type ReadBooleanFunction = ReadFileFunction<fn(&str) -> Result<Value, WdlError>>;

#[cfg(test)]
mod tests {
    use super::*;
    use crate::Type;
    use crate::Value;
    use crate::WdlError;
    use std::fs;
    use tempfile::TempDir;

    fn create_test_file(content: &str) -> (TempDir, String) {
        let temp_dir = TempDir::new().unwrap();
        let file_path = temp_dir.path().join("test.txt");
        fs::write(&file_path, content).unwrap();
        (temp_dir, file_path.to_str().unwrap().to_string())
    }

    #[test]
    fn test_read_int_valid() {
        let (_temp_dir, file_path) = create_test_file("42");
        let func = create_read_int();

        // Test type inference
        let result_type = func.infer_type(&[Type::String { optional: false }]);
        assert!(result_type.is_ok());
        assert_eq!(result_type.unwrap(), Type::int(false));

        // Test evaluation
        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_ok());
        assert_eq!(result.unwrap(), Value::int(42));
    }

    #[test]
    fn test_read_int_with_whitespace() {
        let (_temp_dir, file_path) = create_test_file("  -123  \n");
        let func = create_read_int();

        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_ok());
        assert_eq!(result.unwrap(), Value::int(-123));
    }

    #[test]
    fn test_read_int_invalid() {
        let (_temp_dir, file_path) = create_test_file("not_a_number");
        let func = create_read_int();

        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_err());
        if let Err(WdlError::RuntimeError { message }) = result {
            assert!(message.contains("Failed to parse"));
            assert!(message.contains("integer"));
        } else {
            panic!("Expected RuntimeError");
        }
    }

    #[test]
    fn test_read_float_valid() {
        let (_temp_dir, file_path) = create_test_file("3.14159");
        let func = create_read_float();

        // Test type inference
        let result_type = func.infer_type(&[Type::File { optional: false }]);
        assert!(result_type.is_ok());
        assert_eq!(result_type.unwrap(), Type::float(false));

        // Test evaluation
        let result = func.eval(&[Value::File {
            value: file_path,
            wdl_type: Type::File { optional: false },
        }]);
        assert!(result.is_ok());
        assert_eq!(result.unwrap(), Value::float(3.14159));
    }

    #[test]
    fn test_read_float_integer() {
        let (_temp_dir, file_path) = create_test_file("42");
        let func = create_read_float();

        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_ok());
        assert_eq!(result.unwrap(), Value::float(42.0));
    }

    #[test]
    fn test_read_float_scientific() {
        let (_temp_dir, file_path) = create_test_file("1.23e-4");
        let func = create_read_float();

        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_ok());
        assert_eq!(result.unwrap(), Value::float(0.000123));
    }

    #[test]
    fn test_read_float_invalid() {
        let (_temp_dir, file_path) = create_test_file("not_a_float");
        let func = create_read_float();

        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_err());
        if let Err(WdlError::RuntimeError { message }) = result {
            assert!(message.contains("Failed to parse"));
            assert!(message.contains("float"));
        } else {
            panic!("Expected RuntimeError");
        }
    }

    #[test]
    fn test_read_boolean_true() {
        let (_temp_dir, file_path) = create_test_file("true");
        let func = create_read_boolean();

        // Test type inference
        let result_type = func.infer_type(&[Type::String { optional: false }]);
        assert!(result_type.is_ok());
        assert_eq!(result_type.unwrap(), Type::boolean(false));

        // Test evaluation
        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_ok());
        assert_eq!(result.unwrap(), Value::boolean(true));
    }

    #[test]
    fn test_read_boolean_false() {
        let (_temp_dir, file_path) = create_test_file("false");
        let func = create_read_boolean();

        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_ok());
        assert_eq!(result.unwrap(), Value::boolean(false));
    }

    #[test]
    fn test_read_boolean_case_insensitive() {
        let (_temp_dir, file_path) = create_test_file("TRUE");
        let func = create_read_boolean();

        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_ok());
        assert_eq!(result.unwrap(), Value::boolean(true));

        let (_temp_dir2, file_path2) = create_test_file("False");
        let func2 = create_read_boolean();
        let result2 = func2.eval(&[Value::String {
            value: file_path2,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result2.is_ok());
        assert_eq!(result2.unwrap(), Value::boolean(false));
    }

    #[test]
    fn test_read_json_valid() {
        let (_temp_dir, file_path) =
            create_test_file(r#"{"name": "John", "age": 30, "active": true}"#);
        let func = create_read_json();

        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_ok());

        if let Ok(Value::Map { pairs, .. }) = result {
            assert_eq!(pairs.len(), 3);
            // Check that we got the expected key-value pairs
            let keys: std::collections::HashSet<String> = pairs
                .iter()
                .map(|(k, _)| match k {
                    Value::String { value, .. } => value.clone(),
                    _ => String::new(),
                })
                .collect();
            assert!(keys.contains("name"));
            assert!(keys.contains("age"));
            assert!(keys.contains("active"));
        } else {
            panic!("Expected Map value");
        }
    }

    #[test]
    fn test_read_json_array() {
        let (_temp_dir, file_path) = create_test_file(r#"[1, 2, 3]"#);
        let func = create_read_json();

        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_ok());

        if let Ok(Value::Array { values, .. }) = result {
            assert_eq!(values.len(), 3);
        } else {
            panic!("Expected Array value");
        }
    }

    #[test]
    fn test_read_json_invalid() {
        let (_temp_dir, file_path) = create_test_file("{invalid json}");
        let func = create_read_json();

        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_err());
        if let Err(WdlError::RuntimeError { message }) = result {
            assert!(message.contains("Failed to parse JSON"));
        }
    }

    #[test]
    fn test_read_tsv_valid() {
        let tsv_content =
            "col1\tcol2\tcol3\nrow1val1\trow1val2\trow1val3\nrow2val1\trow2val2\trow2val3";
        let (_temp_dir, file_path) = create_test_file(tsv_content);
        let func = create_read_tsv();

        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_ok());

        if let Ok(Value::Array { values, .. }) = result {
            assert_eq!(values.len(), 3); // 3 rows
                                         // Check first row
            if let Value::Array {
                values: row_values, ..
            } = &values[0]
            {
                assert_eq!(row_values.len(), 3); // 3 columns
            } else {
                panic!("Expected Array value for row");
            }
        } else {
            panic!("Expected Array value");
        }
    }

    #[test]
    fn test_read_tsv_empty() {
        let (_temp_dir, file_path) = create_test_file("");
        let func = create_read_tsv();

        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_ok());

        if let Ok(Value::Array { values, .. }) = result {
            assert_eq!(values.len(), 0);
        } else {
            panic!("Expected Array value");
        }
    }

    #[test]
    fn test_read_map_valid() {
        let map_content = "key1\tvalue1\nkey2\tvalue2\nkey3\tvalue3";
        let (_temp_dir, file_path) = create_test_file(map_content);
        let func = create_read_map();

        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_ok());

        if let Ok(Value::Map { pairs, .. }) = result {
            assert_eq!(pairs.len(), 3);
        } else {
            panic!("Expected Map value");
        }
    }

    #[test]
    fn test_read_map_invalid_fields() {
        let map_content = "key1\tvalue1\textra\nkey2\tvalue2";
        let (_temp_dir, file_path) = create_test_file(map_content);
        let func = create_read_map();

        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_err());
        if let Err(WdlError::RuntimeError { message }) = result {
            assert!(message.contains("read_map(): each line must have two fields"));
        }
    }

    #[test]
    fn test_read_map_duplicate_key() {
        let map_content = "key1\tvalue1\nkey1\tvalue2";
        let (_temp_dir, file_path) = create_test_file(map_content);
        let func = create_read_map();

        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_err());
        if let Err(WdlError::RuntimeError { message }) = result {
            assert!(message.contains("read_map(): duplicate key"));
        }
    }

    #[test]
    fn test_read_objects_valid() {
        let objects_content = "name\tage\tcity\nJohn\t30\tNew York\nJane\t25\tBoston";
        let (_temp_dir, file_path) = create_test_file(objects_content);
        let func = create_read_objects();

        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_ok());

        if let Ok(Value::Array { values, .. }) = result {
            assert_eq!(values.len(), 2); // 2 data rows
                                         // Check first object
            if let Value::Map { pairs, .. } = &values[0] {
                assert_eq!(pairs.len(), 3); // 3 fields
            } else {
                panic!("Expected Map value for object");
            }
        } else {
            panic!("Expected Array value");
        }
    }

    #[test]
    fn test_read_objects_empty_header() {
        let objects_content = "name\tage\t\nJohn\t30\tNew York"; // Empty third column header
        let (_temp_dir, file_path) = create_test_file(objects_content);
        let func = create_read_objects();

        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_err());
        if let Err(WdlError::RuntimeError { message }) = result {
            eprintln!("Error message: {}", message);
            assert!(message.contains("read_objects(): file has empty or duplicate column names"));
        } else {
            panic!("Expected RuntimeError");
        }
    }

    #[test]
    fn test_read_objects_ragged_lines() {
        let objects_content = "name\tage\tcity\nJohn\t30\tNew York\nJane\t25"; // Missing city for Jane
        let (_temp_dir, file_path) = create_test_file(objects_content);
        let func = create_read_objects();

        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_err());
        if let Err(WdlError::RuntimeError { message }) = result {
            assert!(message.contains("read_objects(): file's tab-separated lines are ragged"));
        }
    }

    #[test]
    fn test_read_object_valid() {
        let object_content = "name\tage\tcity\nJohn\t30\tNew York";
        let (_temp_dir, file_path) = create_test_file(object_content);
        let func = create_read_object();

        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_ok());

        if let Ok(Value::Map { pairs, .. }) = result {
            assert_eq!(pairs.len(), 3);
        } else {
            panic!("Expected Map value");
        }
    }

    #[test]
    fn test_read_object_multiple_objects() {
        let object_content = "name\tage\tcity\nJohn\t30\tNew York\nJane\t25\tBoston";
        let (_temp_dir, file_path) = create_test_file(object_content);
        let func = create_read_object();

        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_err());
        if let Err(WdlError::RuntimeError { message }) = result {
            assert!(message.contains("read_object(): file must have exactly one object"));
        }
    }

    #[test]
    fn test_read_string_basic() {
        let (_temp_dir, file_path) = create_test_file("Hello World");
        let func = ReadStringFunction;

        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_ok());
        assert_eq!(result.unwrap(), Value::string("Hello World".to_string()));
    }

    #[test]
    fn test_read_string_with_trailing_newline() {
        let (_temp_dir, file_path) = create_test_file("Hello World\n");
        let func = ReadStringFunction;

        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_ok());
        assert_eq!(result.unwrap(), Value::string("Hello World".to_string()));
    }

    #[test]
    fn test_read_string_with_trailing_crlf() {
        let (_temp_dir, file_path) = create_test_file("Hello World\r\n");
        let func = ReadStringFunction;

        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_ok());
        assert_eq!(result.unwrap(), Value::string("Hello World".to_string()));
    }

    #[test]
    fn test_read_string_with_multiple_trailing_newlines() {
        let (_temp_dir, file_path) = create_test_file("Hello World\n\n\r\n");
        let func = ReadStringFunction;

        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_ok());
        assert_eq!(result.unwrap(), Value::string("Hello World".to_string()));
    }

    #[test]
    fn test_read_string_with_internal_newlines() {
        let (_temp_dir, file_path) = create_test_file("Hello\nWorld\nTest\n");
        let func = ReadStringFunction;

        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_ok());
        // Internal newlines should be preserved, only trailing ones removed
        assert_eq!(
            result.unwrap(),
            Value::string("Hello\nWorld\nTest".to_string())
        );
    }

    #[test]
    fn test_read_string_empty_file() {
        let (_temp_dir, file_path) = create_test_file("");
        let func = ReadStringFunction;

        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_ok());
        assert_eq!(result.unwrap(), Value::string("".to_string()));
    }

    #[test]
    fn test_read_string_only_newlines() {
        let (_temp_dir, file_path) = create_test_file("\n\r\n\n");
        let func = ReadStringFunction;

        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_ok());
        assert_eq!(result.unwrap(), Value::string("".to_string()));
    }

    #[test]
    fn test_read_boolean_with_whitespace() {
        let (_temp_dir, file_path) = create_test_file("  true  \n");
        let func = create_read_boolean();

        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_ok());
        assert_eq!(result.unwrap(), Value::boolean(true));
    }

    #[test]
    fn test_read_boolean_invalid() {
        let (_temp_dir, file_path) = create_test_file("yes");
        let func = create_read_boolean();

        let result = func.eval(&[Value::String {
            value: file_path,
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_err());
        if let Err(WdlError::RuntimeError { message }) = result {
            assert!(message.contains("Failed to parse"));
            assert!(message.contains("boolean"));
            assert!(message.contains("'true' or 'false'"));
        } else {
            panic!("Expected RuntimeError");
        }
    }

    #[test]
    fn test_file_not_found() {
        let func = create_read_int();

        let result = func.eval(&[Value::String {
            value: "/nonexistent/file.txt".to_string(),
            wdl_type: Type::String { optional: false },
        }]);
        assert!(result.is_err());
        if let Err(WdlError::RuntimeError { message }) = result {
            assert!(message.contains("Failed to read file"));
        } else {
            panic!("Expected RuntimeError");
        }
    }

    #[test]
    fn test_wrong_argument_type() {
        let func = create_read_int();

        let result = func.eval(&[Value::int(42)]);
        assert!(result.is_err());
        if let Err(WdlError::RuntimeError { message }) = result {
            assert!(message.contains("argument must be String or File"));
        } else {
            panic!("Expected RuntimeError");
        }
    }

    #[test]
    fn test_wrong_argument_count() {
        let func = create_read_int();

        let result_type = func.infer_type(&[]);
        assert!(result_type.is_err());
        if let Err(WdlError::ArgumentCountMismatch {
            expected, actual, ..
        }) = result_type
        {
            assert_eq!(expected, 1);
            assert_eq!(actual, 0);
        } else {
            panic!("Expected ArgumentCountMismatch");
        }

        let result_type = func.infer_type(&[
            Type::String { optional: false },
            Type::String { optional: false },
        ]);
        assert!(result_type.is_err());
        if let Err(WdlError::ArgumentCountMismatch {
            expected, actual, ..
        }) = result_type
        {
            assert_eq!(expected, 1);
            assert_eq!(actual, 2);
        } else {
            panic!("Expected ArgumentCountMismatch");
        }
    }
}
