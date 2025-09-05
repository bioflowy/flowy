//! Input/Output functions for WDL standard library

use super::Function;
use crate::error::WdlError;
use crate::types::Type;
use crate::value::Value;
use std::collections::{HashMap, HashSet};
use std::path::{Path, PathBuf};

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

/// Enhanced read_tsv function that supports multiple variants:
/// 1. read_tsv(File) -> Array[Array[String]]
/// 2. read_tsv(File, Boolean) -> Array[Object] (when true, first line is headers)
/// 3. read_tsv(File, Boolean, Array[String]) -> Array[Object] (custom field names)
pub struct ReadTsvFunction;

impl Function for ReadTsvFunction {
    fn name(&self) -> &str {
        "read_tsv"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        match args.len() {
            1 => {
                // read_tsv(File) -> Array[Array[String]]
                if !matches!(args[0], Type::String { .. } | Type::File { .. }) {
                    return Err(WdlError::RuntimeError {
                        message: "read_tsv() first argument must be String or File".to_string(),
                    });
                }
                Ok(Type::Array {
                    item_type: Box::new(Type::Array {
                        item_type: Box::new(Type::String { optional: false }),
                        optional: false,
                        nonempty: false,
                    }),
                    optional: false,
                    nonempty: false,
                })
            }
            2 => {
                // read_tsv(File, Boolean) -> Array[Object] (per WDL spec)
                if !matches!(args[0], Type::String { .. } | Type::File { .. }) {
                    return Err(WdlError::RuntimeError {
                        message: "read_tsv() first argument must be String or File".to_string(),
                    });
                }
                if !matches!(args[1], Type::Boolean { .. }) {
                    return Err(WdlError::RuntimeError {
                        message: "read_tsv() second argument must be Boolean".to_string(),
                    });
                }
                Ok(Type::Array {
                    item_type: Box::new(Type::Object {
                        members: std::collections::HashMap::new(),
                    }),
                    optional: false,
                    nonempty: false,
                })
            }
            3 => {
                // read_tsv(File, Boolean, Array[String]) -> Array[Object] (per WDL spec)
                if !matches!(args[0], Type::String { .. } | Type::File { .. }) {
                    return Err(WdlError::RuntimeError {
                        message: "read_tsv() first argument must be String or File".to_string(),
                    });
                }
                if !matches!(args[1], Type::Boolean { .. }) {
                    return Err(WdlError::RuntimeError {
                        message: "read_tsv() second argument must be Boolean".to_string(),
                    });
                }
                if !matches!(args[2], Type::Array { .. }) {
                    return Err(WdlError::RuntimeError {
                        message: "read_tsv() third argument must be Array[String]".to_string(),
                    });
                }
                Ok(Type::Array {
                    item_type: Box::new(Type::Object {
                        members: std::collections::HashMap::new(),
                    }),
                    optional: false,
                    nonempty: false,
                })
            }
            _ => Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 1,
                actual: args.len(),
            }),
        }
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        let filename = match &args[0] {
            Value::String { value, .. } => value.clone(),
            Value::File { value, .. } => value.clone(),
            _ => {
                return Err(WdlError::RuntimeError {
                    message: "read_tsv() first argument must be String or File".to_string(),
                })
            }
        };

        // Read the file content (without path mapping for default implementation)
        let content = match std::fs::read_to_string(&filename) {
            Ok(content) => content,
            Err(e) => {
                return Err(WdlError::RuntimeError {
                    message: format!("Failed to read file {}: {}", filename, e),
                })
            }
        };

        match args.len() {
            1 => {
                // read_tsv(File) -> Array[Array[String]] - original behavior
                self.parse_as_string_arrays(&content)
            }
            2 => {
                // read_tsv(File, Boolean) -> Array[Object]
                let has_headers = match &args[1] {
                    Value::Boolean { value, .. } => *value,
                    _ => {
                        return Err(WdlError::RuntimeError {
                            message: "read_tsv() second argument must be Boolean".to_string(),
                        })
                    }
                };
                self.parse_as_objects(&content, has_headers, None)
            }
            3 => {
                // read_tsv(File, Boolean, Array[String]) -> Array[Object]
                let has_headers = match &args[1] {
                    Value::Boolean { value, .. } => *value,
                    _ => {
                        return Err(WdlError::RuntimeError {
                            message: "read_tsv() second argument must be Boolean".to_string(),
                        })
                    }
                };
                let field_names = match &args[2] {
                    Value::Array { values, .. } => {
                        let mut names = Vec::new();
                        for value in values {
                            match value {
                                Value::String { value, .. } => names.push(value.clone()),
                                _ => {
                                    return Err(WdlError::RuntimeError {
                                        message: "read_tsv() third argument must be Array[String]"
                                            .to_string(),
                                    })
                                }
                            }
                        }
                        Some(names)
                    }
                    _ => {
                        return Err(WdlError::RuntimeError {
                            message: "read_tsv() third argument must be Array[String]".to_string(),
                        })
                    }
                };
                self.parse_as_objects(&content, has_headers, field_names)
            }
            _ => Err(WdlError::ArgumentCountMismatch {
                function: "read_tsv".to_string(),
                expected: 1,
                actual: args.len(),
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
                    message: "read_tsv() first argument must be String or File".to_string(),
                })
            }
        };

        // Use path mapper to devirtualize filename
        let real_path = stdlib.path_mapper().devirtualize_filename(&filename)?;

        // Read the file content
        let content = match std::fs::read_to_string(&real_path) {
            Ok(content) => content,
            Err(e) => {
                return Err(WdlError::RuntimeError {
                    message: format!("Failed to read file {}: {}", real_path.display(), e),
                })
            }
        };

        match args.len() {
            1 => {
                // read_tsv(File) -> Array[Array[String]] - original behavior
                self.parse_as_string_arrays(&content)
            }
            2 => {
                // read_tsv(File, Boolean) -> Array[Object]
                let has_headers = match &args[1] {
                    Value::Boolean { value, .. } => *value,
                    _ => {
                        return Err(WdlError::RuntimeError {
                            message: "read_tsv() second argument must be Boolean".to_string(),
                        })
                    }
                };
                self.parse_as_objects(&content, has_headers, None)
            }
            3 => {
                // read_tsv(File, Boolean, Array[String]) -> Array[Object]
                let has_headers = match &args[1] {
                    Value::Boolean { value, .. } => *value,
                    _ => {
                        return Err(WdlError::RuntimeError {
                            message: "read_tsv() second argument must be Boolean".to_string(),
                        })
                    }
                };
                let field_names = match &args[2] {
                    Value::Array { values, .. } => {
                        let mut names = Vec::new();
                        for value in values {
                            match value {
                                Value::String { value, .. } => names.push(value.clone()),
                                _ => {
                                    return Err(WdlError::RuntimeError {
                                        message: "read_tsv() third argument must be Array[String]"
                                            .to_string(),
                                    })
                                }
                            }
                        }
                        Some(names)
                    }
                    _ => {
                        return Err(WdlError::RuntimeError {
                            message: "read_tsv() third argument must be Array[String]".to_string(),
                        })
                    }
                };
                self.parse_as_objects(&content, has_headers, field_names)
            }
            _ => Err(WdlError::ArgumentCountMismatch {
                function: "read_tsv".to_string(),
                expected: 1,
                actual: args.len(),
            }),
        }
    }
}

impl ReadTsvFunction {
    /// Parse TSV content as Array[Array[String]] (original behavior)
    fn parse_as_string_arrays(&self, content: &str) -> Result<Value, WdlError> {
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
    }

    /// Parse TSV content as Array[Object] (per WDL spec)
    fn parse_as_objects(
        &self,
        content: &str,
        has_headers: bool,
        field_names: Option<Vec<String>>,
    ) -> Result<Value, WdlError> {
        let lines: Vec<&str> = content
            .lines()
            .filter(|line| !line.trim().is_empty())
            .collect();

        if lines.is_empty() {
            return Ok(Value::Array {
                values: Vec::new(),
                wdl_type: Type::Array {
                    item_type: Box::new(Type::Object {
                        members: std::collections::HashMap::new(),
                    }),
                    optional: false,
                    nonempty: false,
                },
            });
        }

        let (headers, data_start_idx) = if let Some(names) = field_names {
            // When field_names is provided, it overrides file headers
            // If has_headers=true, skip the header line
            let start_idx = if has_headers { 1 } else { 0 };
            (names, start_idx)
        } else if has_headers {
            // Use first line as headers when field_names is not provided
            let header: Vec<&str> = lines[0].split('\t').collect();
            (header.into_iter().map(|s| s.to_string()).collect(), 1)
        } else {
            return Err(WdlError::RuntimeError {
                message: "read_tsv() with has_headers=false must provide field names".to_string(),
            });
        };

        let mut objects = Vec::new();

        // Parse data rows - create Objects (using struct values)
        for line in &lines[data_start_idx..] {
            let fields: Vec<&str> = line.split('\t').collect();
            if fields.len() != headers.len() {
                return Err(WdlError::RuntimeError {
                    message: "read_tsv(): inconsistent number of fields".to_string(),
                });
            }

            let mut object_members = std::collections::HashMap::new();
            for (header, field) in headers.iter().zip(fields.iter()) {
                let value = Value::String {
                    value: field.to_string(),
                    wdl_type: Type::String { optional: false },
                };
                object_members.insert(header.clone(), value);
            }

            // Create Object as struct value with Object type
            let object_type = Type::Object {
                members: std::collections::HashMap::new(),
            };
            let object_value = Value::struct_value_unchecked(object_type, object_members, None);
            objects.push(object_value);
        }

        Ok(Value::Array {
            values: objects,
            wdl_type: Type::Array {
                item_type: Box::new(Type::Object {
                    members: std::collections::HashMap::new(),
                }),
                optional: false,
                nonempty: false,
            },
        })
    }
}

/// Helper function to create read_tsv function
pub fn create_read_tsv() -> Box<dyn Function> {
    Box::new(ReadTsvFunction)
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

/// Glob function that returns an array of files matching a pattern
pub struct GlobFunction {
    name: String,
    task_dir: PathBuf,
}

impl GlobFunction {
    pub fn new(task_dir: PathBuf) -> Self {
        Self {
            name: "glob".to_string(),
            task_dir,
        }
    }
}

impl Function for GlobFunction {
    fn name(&self) -> &str {
        &self.name
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 1,
                actual: args.len(),
            });
        }

        // Expect String argument
        match &args[0] {
            Type::String { .. } => Ok(Type::Array {
                item_type: Box::new(Type::File { optional: false }),
                optional: false,
                nonempty: false,
            }),
            _ => Err(WdlError::TypeMismatch {
                expected: Type::String { optional: false },
                actual: args[0].clone(),
            }),
        }
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::ArgumentCountMismatch {
                function: "glob".to_string(),
                expected: 1,
                actual: args.len(),
            });
        }

        let pattern = match &args[0] {
            Value::String { value, .. } => value.clone(),
            _ => {
                return Err(WdlError::RuntimeError {
                    message: "glob() expects a String argument".to_string(),
                });
            }
        };

        // Use glob crate to find matching files
        let glob_pattern = if pattern.starts_with('/') {
            // Absolute path
            pattern
        } else {
            // Relative path - make it relative to task directory
            self.task_dir.join(&pattern).to_string_lossy().to_string()
        };

        match glob::glob(&glob_pattern) {
            Ok(paths) => {
                let mut files = Vec::new();
                for entry in paths {
                    match entry {
                        Ok(path) => {
                            if path.is_file() {
                                files.push(Value::File {
                                    value: path.to_string_lossy().to_string(),
                                    wdl_type: Type::File { optional: false },
                                });
                            }
                        }
                        Err(e) => {
                            return Err(WdlError::RuntimeError {
                                message: format!("glob() error: {}", e),
                            });
                        }
                    }
                }

                // Sort files for consistent results
                files.sort_by(|a, b| {
                    if let (Value::File { value: a_val, .. }, Value::File { value: b_val, .. }) =
                        (a, b)
                    {
                        a_val.cmp(b_val)
                    } else {
                        std::cmp::Ordering::Equal
                    }
                });

                Ok(Value::Array {
                    values: files,
                    wdl_type: Type::Array {
                        item_type: Box::new(Type::File { optional: false }),
                        optional: false,
                        nonempty: false,
                    },
                })
            }
            Err(e) => Err(WdlError::RuntimeError {
                message: format!("glob() pattern error: {}", e),
            }),
        }
    }
}

/// Helper function to create glob function
pub fn create_glob(task_dir: PathBuf) -> Box<dyn Function> {
    Box::new(GlobFunction::new(task_dir))
}

/// Generic write file function that serializes data using a provided serializer
pub struct WriteFileFunction<F> {
    name: &'static str,
    input_type: Type,
    serializer: F,
}

/// Helper function to create write_lines function
pub fn create_write_lines() -> Box<dyn Function> {
    Box::new(WriteFileFunction::new(
        "write_lines",
        Type::Array {
            item_type: Box::new(Type::String { optional: false }),
            optional: false,
            nonempty: false,
        },
        |value| {
            let array = value.as_array().ok_or_else(|| WdlError::RuntimeError {
                message: "write_lines() argument must be Array[String]".to_string(),
            })?;

            let mut lines = Vec::new();
            for item in array {
                let line = match item {
                    Value::String { value, .. } => value.clone(),
                    _ => item.to_string(),
                };
                lines.push(line);
            }

            // For empty arrays, return empty string (no newline)
            // For non-empty arrays, join with newlines and add trailing newline
            if lines.is_empty() {
                Ok(String::new())
            } else {
                Ok(lines.join("\n") + "\n")
            }
        },
    ))
}

/// Multi-signature write_tsv function that supports multiple variants:
/// 1. write_tsv(Array[Array[String]]) -> File (no headers)
/// 2. write_tsv(Array[Array[String]], true, Array[String]) -> File (with custom headers)
/// 3. write_tsv(Array[Struct]) -> File (no headers, fields in definition order)
/// 4. write_tsv(Array[Struct], Boolean) -> File (optional headers from field names)
/// 5. write_tsv(Array[Struct], Boolean, Array[String]) -> File (optional headers with custom names)
pub struct WriteTsvFunction;

impl Function for WriteTsvFunction {
    fn name(&self) -> &str {
        "write_tsv"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.is_empty() || args.len() > 3 {
            return Err(WdlError::ArgumentCountMismatch {
                function: "write_tsv".to_string(),
                expected: 1, // Can be 1, 2, or 3, but we'll validate in detail below
                actual: args.len(),
            });
        }

        match args.len() {
            1 => {
                // write_tsv(Array[Array[String]]) or write_tsv(Array[Struct])
                match &args[0] {
                    Type::Array { item_type, .. } => {
                        match item_type.as_ref() {
                            Type::Array {
                                item_type: inner, ..
                            } => {
                                // Array[Array[String]]
                                if let Type::String { .. } = inner.as_ref() {
                                    Ok(Type::file(false))
                                } else {
                                    Err(WdlError::TypeMismatch {
                                        expected: Type::Array {
                                            item_type: Box::new(Type::Array {
                                                item_type: Box::new(Type::string(false)),
                                                optional: false,
                                                nonempty: false,
                                            }),
                                            optional: false,
                                            nonempty: false,
                                        },
                                        actual: args[0].clone(),
                                    })
                                }
                            }
                            Type::StructInstance { .. } => {
                                // Array[Struct]
                                Ok(Type::file(false))
                            }
                            _ => Err(WdlError::TypeMismatch {
                                expected: Type::Array {
                                    item_type: Box::new(Type::Array {
                                        item_type: Box::new(Type::string(false)),
                                        optional: false,
                                        nonempty: false,
                                    }),
                                    optional: false,
                                    nonempty: false,
                                },
                                actual: args[0].clone(),
                            }),
                        }
                    }
                    _ => Err(WdlError::TypeMismatch {
                        expected: Type::Array {
                            item_type: Box::new(Type::Array {
                                item_type: Box::new(Type::string(false)),
                                optional: false,
                                nonempty: false,
                            }),
                            optional: false,
                            nonempty: false,
                        },
                        actual: args[0].clone(),
                    }),
                }
            }
            2 => {
                // write_tsv(Array[Struct], Boolean)
                match &args[0] {
                    Type::Array { item_type, .. } => {
                        if let Type::StructInstance { .. } = item_type.as_ref() {
                            if let Type::Boolean { .. } = &args[1] {
                                Ok(Type::file(false))
                            } else {
                                Err(WdlError::TypeMismatch {
                                    expected: Type::boolean(false),
                                    actual: args[1].clone(),
                                })
                            }
                        } else {
                            Err(WdlError::TypeMismatch {
                                expected: Type::Array {
                                    item_type: Box::new(Type::StructInstance {
                                        type_name: "Any".to_string(),
                                        members: Some(std::collections::HashMap::new()),
                                        optional: false,
                                    }),
                                    optional: false,
                                    nonempty: false,
                                },
                                actual: args[0].clone(),
                            })
                        }
                    }
                    _ => Err(WdlError::TypeMismatch {
                        expected: Type::Array {
                            item_type: Box::new(Type::StructInstance {
                                type_name: "Any".to_string(),
                                members: Some(std::collections::HashMap::new()),
                                optional: false,
                            }),
                            optional: false,
                            nonempty: false,
                        },
                        actual: args[0].clone(),
                    }),
                }
            }
            3 => {
                // write_tsv(Array[Array[String]], true, Array[String]) or
                // write_tsv(Array[Struct], Boolean, Array[String])
                match &args[0] {
                    Type::Array { item_type, .. } => {
                        match item_type.as_ref() {
                            Type::Array {
                                item_type: inner, ..
                            } => {
                                // Array[Array[String]], true, Array[String]
                                if let Type::String { .. } = inner.as_ref() {
                                    // Second argument must be Boolean (will be true at runtime)
                                    if let Type::Boolean { .. } = &args[1] {
                                        // Third argument must be Array[String]
                                        if let Type::Array {
                                            item_type: header_type,
                                            ..
                                        } = &args[2]
                                        {
                                            if let Type::String { .. } = header_type.as_ref() {
                                                Ok(Type::file(false))
                                            } else {
                                                Err(WdlError::TypeMismatch {
                                                    expected: Type::Array {
                                                        item_type: Box::new(Type::string(false)),
                                                        optional: false,
                                                        nonempty: false,
                                                    },
                                                    actual: args[2].clone(),
                                                })
                                            }
                                        } else {
                                            Err(WdlError::TypeMismatch {
                                                expected: Type::Array {
                                                    item_type: Box::new(Type::string(false)),
                                                    optional: false,
                                                    nonempty: false,
                                                },
                                                actual: args[2].clone(),
                                            })
                                        }
                                    } else {
                                        Err(WdlError::TypeMismatch {
                                            expected: Type::boolean(false),
                                            actual: args[1].clone(),
                                        })
                                    }
                                } else {
                                    Err(WdlError::TypeMismatch {
                                        expected: Type::Array {
                                            item_type: Box::new(Type::Array {
                                                item_type: Box::new(Type::string(false)),
                                                optional: false,
                                                nonempty: false,
                                            }),
                                            optional: false,
                                            nonempty: false,
                                        },
                                        actual: args[0].clone(),
                                    })
                                }
                            }
                            Type::StructInstance { .. } => {
                                // Array[Struct], Boolean, Array[String]
                                if let Type::Boolean { .. } = &args[1] {
                                    if let Type::Array {
                                        item_type: header_type,
                                        ..
                                    } = &args[2]
                                    {
                                        if let Type::String { .. } = header_type.as_ref() {
                                            Ok(Type::file(false))
                                        } else {
                                            Err(WdlError::TypeMismatch {
                                                expected: Type::Array {
                                                    item_type: Box::new(Type::string(false)),
                                                    optional: false,
                                                    nonempty: false,
                                                },
                                                actual: args[2].clone(),
                                            })
                                        }
                                    } else {
                                        Err(WdlError::TypeMismatch {
                                            expected: Type::Array {
                                                item_type: Box::new(Type::string(false)),
                                                optional: false,
                                                nonempty: false,
                                            },
                                            actual: args[2].clone(),
                                        })
                                    }
                                } else {
                                    Err(WdlError::TypeMismatch {
                                        expected: Type::boolean(false),
                                        actual: args[1].clone(),
                                    })
                                }
                            }
                            _ => Err(WdlError::TypeMismatch {
                                expected: Type::Array {
                                    item_type: Box::new(Type::Array {
                                        item_type: Box::new(Type::string(false)),
                                        optional: false,
                                        nonempty: false,
                                    }),
                                    optional: false,
                                    nonempty: false,
                                },
                                actual: args[0].clone(),
                            }),
                        }
                    }
                    _ => Err(WdlError::TypeMismatch {
                        expected: Type::Array {
                            item_type: Box::new(Type::Array {
                                item_type: Box::new(Type::string(false)),
                                optional: false,
                                nonempty: false,
                            }),
                            optional: false,
                            nonempty: false,
                        },
                        actual: args[0].clone(),
                    }),
                }
            }
            _ => unreachable!("Already checked arg length"),
        }
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        use std::io::Write;

        if args.is_empty() || args.len() > 3 {
            return Err(WdlError::ArgumentCountMismatch {
                function: "write_tsv".to_string(),
                expected: 1,
                actual: args.len(),
            });
        }

        let content = match args.len() {
            1 => {
                // write_tsv(Array[Array[String]]) or write_tsv(Array[Struct])
                // For Array[Struct], include headers by default per WDL spec expectation
                let include_headers_for_struct = if let Value::Array { values, .. } = &args[0] {
                    if let Some(first_item) = values.first() {
                        matches!(first_item, Value::Struct { .. })
                    } else {
                        false
                    }
                } else {
                    false
                };
                self.generate_tsv_content(&args[0], include_headers_for_struct, None)?
            }
            2 => {
                // write_tsv(Array[Struct], Boolean)
                let write_headers = match &args[1] {
                    Value::Boolean { value, .. } => *value,
                    _ => {
                        return Err(WdlError::RuntimeError {
                            message: "write_tsv() second argument must be Boolean".to_string(),
                        })
                    }
                };
                self.generate_tsv_content(&args[0], write_headers, None)?
            }
            3 => {
                // write_tsv(Array[Array[String]], true, Array[String]) or
                // write_tsv(Array[Struct], Boolean, Array[String])
                let write_headers = match &args[1] {
                    Value::Boolean { value, .. } => *value,
                    _ => {
                        return Err(WdlError::RuntimeError {
                            message: "write_tsv() second argument must be Boolean".to_string(),
                        })
                    }
                };

                let custom_headers = match &args[2] {
                    Value::Array { values, .. } => {
                        let mut headers = Vec::new();
                        for val in values {
                            match val {
                                Value::String { value, .. } => headers.push(value.clone()),
                                _ => {
                                    return Err(WdlError::RuntimeError {
                                        message: "write_tsv() third argument must be Array[String]"
                                            .to_string(),
                                    })
                                }
                            }
                        }
                        Some(headers)
                    }
                    _ => {
                        return Err(WdlError::RuntimeError {
                            message: "write_tsv() third argument must be Array[String]".to_string(),
                        })
                    }
                };

                // For Array[Array[String]], true, Array[String] - headers must be true
                if let Value::Array { values, .. } = &args[0] {
                    if let Some(first_item) = values.first() {
                        if let Value::Array { .. } = first_item {
                            // This is Array[Array[String]] case - second arg must be true
                            if !write_headers {
                                return Err(WdlError::RuntimeError {
                                    message: "write_tsv() with Array[Array[String]] and Array[String] headers requires second argument to be true".to_string(),
                                });
                            }
                        }
                    }
                }

                self.generate_tsv_content(&args[0], write_headers, custom_headers)?
            }
            _ => unreachable!("Already checked arg length"),
        };

        // Create a temporary file - this is the non-stdlib version
        let mut temp_file = std::env::temp_dir();
        temp_file.push(format!(
            "write_tsv_{}.txt",
            std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap_or_default()
                .as_nanos()
        ));

        let mut file = std::fs::File::create(&temp_file).map_err(|e| WdlError::RuntimeError {
            message: format!("Failed to create temporary file: {}", e),
        })?;

        file.write_all(content.as_bytes())
            .map_err(|e| WdlError::RuntimeError {
                message: format!("Failed to write to file: {}", e),
            })?;

        Value::file(temp_file.to_string_lossy().to_string())
    }

    fn eval_with_stdlib(
        &self,
        args: &[Value],
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Value, WdlError> {
        use std::io::Write;

        if args.is_empty() || args.len() > 3 {
            return Err(WdlError::ArgumentCountMismatch {
                function: "write_tsv".to_string(),
                expected: 1,
                actual: args.len(),
            });
        }

        let content = match args.len() {
            1 => {
                // write_tsv(Array[Array[String]]) or write_tsv(Array[Struct])
                // For Array[Struct], include headers by default per WDL spec expectation
                let include_headers_for_struct = if let Value::Array { values, .. } = &args[0] {
                    if let Some(first_item) = values.first() {
                        matches!(first_item, Value::Struct { .. })
                    } else {
                        false
                    }
                } else {
                    false
                };
                self.generate_tsv_content(&args[0], include_headers_for_struct, None)?
            }
            2 => {
                // write_tsv(Array[Struct], Boolean)
                let write_headers = match &args[1] {
                    Value::Boolean { value, .. } => *value,
                    _ => {
                        return Err(WdlError::RuntimeError {
                            message: "write_tsv() second argument must be Boolean".to_string(),
                        })
                    }
                };
                self.generate_tsv_content(&args[0], write_headers, None)?
            }
            3 => {
                // write_tsv(Array[Array[String]], true, Array[String]) or
                // write_tsv(Array[Struct], Boolean, Array[String])
                let write_headers = match &args[1] {
                    Value::Boolean { value, .. } => *value,
                    _ => {
                        return Err(WdlError::RuntimeError {
                            message: "write_tsv() second argument must be Boolean".to_string(),
                        })
                    }
                };

                let custom_headers = match &args[2] {
                    Value::Array { values, .. } => {
                        let mut headers = Vec::new();
                        for val in values {
                            match val {
                                Value::String { value, .. } => headers.push(value.clone()),
                                _ => {
                                    return Err(WdlError::RuntimeError {
                                        message: "write_tsv() third argument must be Array[String]"
                                            .to_string(),
                                    })
                                }
                            }
                        }
                        Some(headers)
                    }
                    _ => {
                        return Err(WdlError::RuntimeError {
                            message: "write_tsv() third argument must be Array[String]".to_string(),
                        })
                    }
                };

                // For Array[Array[String]], true, Array[String] - headers must be true
                if let Value::Array { values, .. } = &args[0] {
                    if let Some(first_item) = values.first() {
                        if let Value::Array { .. } = first_item {
                            // This is Array[Array[String]] case - second arg must be true
                            if !write_headers {
                                return Err(WdlError::RuntimeError {
                                    message: "write_tsv() with Array[Array[String]] and Array[String] headers requires second argument to be true".to_string(),
                                });
                            }
                        }
                    }
                }

                self.generate_tsv_content(&args[0], write_headers, custom_headers)?
            }
            _ => unreachable!("Already checked arg length"),
        };

        // Create file in task directory if available, otherwise fall back to temp dir
        let output_file = if let Some(task_dir) = stdlib.task_dir() {
            let mut path = task_dir.clone();
            path.push(format!(
                "write_tsv_{}.txt",
                std::time::SystemTime::now()
                    .duration_since(std::time::UNIX_EPOCH)
                    .unwrap_or_default()
                    .as_nanos()
            ));
            path
        } else {
            let mut temp_file = std::env::temp_dir();
            temp_file.push(format!(
                "write_tsv_{}.txt",
                std::time::SystemTime::now()
                    .duration_since(std::time::UNIX_EPOCH)
                    .unwrap_or_default()
                    .as_nanos()
            ));
            temp_file
        };

        let mut file = std::fs::File::create(&output_file).map_err(|e| WdlError::RuntimeError {
            message: format!("Failed to create output file: {}", e),
        })?;

        file.write_all(content.as_bytes())
            .map_err(|e| WdlError::RuntimeError {
                message: format!("Failed to write to file: {}", e),
            })?;

        // Use path mapper to virtualize the filename
        let virtual_path = stdlib.path_mapper().virtualize_filename(&output_file)?;
        Value::file(virtual_path)
    }
}

impl WriteTsvFunction {
    /// Generate TSV content based on the input array and options
    fn generate_tsv_content(
        &self,
        array_value: &Value,
        write_headers: bool,
        custom_headers: Option<Vec<String>>,
    ) -> Result<String, WdlError> {
        let array = array_value
            .as_array()
            .ok_or_else(|| WdlError::RuntimeError {
                message: "write_tsv() first argument must be an Array".to_string(),
            })?;

        if array.is_empty() {
            return Ok(String::new());
        }

        let mut rows = Vec::new();

        // Check if this is Array[Array[String]] or Array[Struct]
        match &array[0] {
            Value::Array { .. } => {
                // Array[Array[String]] case
                if write_headers {
                    if let Some(headers) = custom_headers {
                        // Validate headers match data width
                        if let Value::Array {
                            values: row_values, ..
                        } = &array[0]
                        {
                            if headers.len() != row_values.len() {
                                return Err(WdlError::RuntimeError {
                                    message: format!(
                                        "write_tsv(): header count ({}) doesn't match data column count ({})",
                                        headers.len(), row_values.len()
                                    ),
                                });
                            }
                        }
                        rows.push(headers.join("\t"));
                    } else {
                        return Err(WdlError::RuntimeError {
                            message: "write_tsv(): Array[Array[String]] with headers requires custom header names".to_string(),
                        });
                    }
                }

                // Process data rows
                for row_value in array {
                    let row_array = row_value.as_array().ok_or_else(|| WdlError::RuntimeError {
                        message: "write_tsv() argument must be Array[Array[String]]".to_string(),
                    })?;

                    let mut columns = Vec::new();
                    for col_value in row_array {
                        let col_str = match col_value {
                            Value::String { value, .. } => value.clone(),
                            _ => col_value.to_string(),
                        };

                        // Validate that the string doesn't contain tabs or newlines
                        if col_str.contains('\t') || col_str.contains('\n') {
                            return Err(WdlError::RuntimeError {
                                message: format!(
                                    "write_tsv() cannot write string containing tab or newline: '{}'",
                                    col_str
                                ),
                            });
                        }

                        columns.push(col_str);
                    }
                    rows.push(columns.join("\t"));
                }
            }
            Value::Struct { members, .. } => {
                // Array[Struct] case
                let mut field_names: Vec<String> = members.keys().cloned().collect();
                field_names.sort(); // Ensure consistent ordering

                if write_headers {
                    let header_names = if let Some(custom_headers) = custom_headers {
                        // Validate custom headers count matches struct fields
                        if custom_headers.len() != field_names.len() {
                            return Err(WdlError::RuntimeError {
                                message: format!(
                                    "write_tsv(): header count ({}) doesn't match struct field count ({})",
                                    custom_headers.len(), field_names.len()
                                ),
                            });
                        }
                        custom_headers
                    } else {
                        field_names.clone()
                    };
                    rows.push(header_names.join("\t"));
                }

                // Process data rows
                for struct_value in array {
                    if let Value::Struct {
                        members: struct_members,
                        ..
                    } = struct_value
                    {
                        let mut columns = Vec::new();

                        // Output fields in the same order as the first struct
                        for field_name in &field_names {
                            if let Some(field_value) = struct_members.get(field_name) {
                                let field_str = match field_value {
                                    Value::String { value, .. } => value.clone(),
                                    _ => field_value.to_string(),
                                };

                                // Validate that the string doesn't contain tabs or newlines
                                if field_str.contains('\t') || field_str.contains('\n') {
                                    return Err(WdlError::RuntimeError {
                                        message: format!(
                                            "write_tsv() cannot write string containing tab or newline: '{}'",
                                            field_str
                                        ),
                                    });
                                }

                                columns.push(field_str);
                            } else {
                                return Err(WdlError::RuntimeError {
                                    message: format!(
                                        "write_tsv(): struct missing field '{}'",
                                        field_name
                                    ),
                                });
                            }
                        }
                        rows.push(columns.join("\t"));
                    } else {
                        return Err(WdlError::RuntimeError {
                            message: "write_tsv() array contains non-struct values".to_string(),
                        });
                    }
                }
            }
            _ => {
                return Err(WdlError::RuntimeError {
                    message:
                        "write_tsv() first argument must be Array[Array[String]] or Array[Struct]"
                            .to_string(),
                });
            }
        }

        Ok(rows.join("\n") + "\n")
    }
}

/// Helper function to create write_tsv function
pub fn create_write_tsv() -> Box<dyn Function> {
    Box::new(WriteTsvFunction)
}

/// Helper function to create write_map function
pub fn create_write_map() -> Box<dyn Function> {
    Box::new(WriteFileFunction::new(
        "write_map",
        Type::Map {
            key_type: Box::new(Type::String { optional: false }),
            value_type: Box::new(Type::String { optional: false }),
            optional: false,
            literal_keys: None,
        },
        |value| {
            let pairs = match value {
                Value::Map { pairs, .. } => pairs,
                _ => {
                    return Err(WdlError::RuntimeError {
                        message: "write_map() argument must be a Map".to_string(),
                    });
                }
            };

            let mut lines = Vec::new();
            for (key_val, value_val) in pairs {
                let key_str = match key_val {
                    Value::String { value, .. } => value.clone(),
                    _ => key_val.to_string(),
                };

                let val_str = match value_val {
                    Value::String { value, .. } => value.clone(),
                    _ => value_val.to_string(),
                };

                // Validate that keys and values don't contain tabs or newlines
                if key_str.contains('\t') || key_str.contains('\n') {
                    return Err(WdlError::RuntimeError {
                        message: format!(
                            "write_map() cannot write key containing tab or newline: '{}'",
                            key_str
                        ),
                    });
                }
                if val_str.contains('\t') || val_str.contains('\n') {
                    return Err(WdlError::RuntimeError {
                        message: format!(
                            "write_map() cannot write value containing tab or newline: '{}'",
                            val_str
                        ),
                    });
                }

                lines.push(format!("{}\t{}", key_str, val_str));
            }

            Ok(lines.join("\n") + "\n")
        },
    ))
}

/// Helper function to create write_json function  
pub fn create_write_json() -> Box<dyn Function> {
    Box::new(WriteFileFunction::new(
        "write_json",
        Type::Any { optional: false }, // Any type can be serialized to JSON
        |value| {
            // Convert WDL Value to JSON using serde_json
            let json_value = value_to_json(value)?;
            serde_json::to_string_pretty(&json_value)
                .map_err(|e| WdlError::RuntimeError {
                    message: format!("Failed to serialize to JSON: {}", e),
                })
                .map(|s| s + "\n") // Add newline at end
        },
    ))
}

/// Convert WDL Value to serde_json::Value for JSON serialization
fn value_to_json(value: &Value) -> Result<serde_json::Value, WdlError> {
    match value {
        Value::Null => Ok(serde_json::Value::Null),
        Value::Boolean { value, .. } => Ok(serde_json::Value::Bool(*value)),
        Value::Int { value, .. } => Ok(serde_json::Value::Number(serde_json::Number::from(*value))),
        Value::Float { value, .. } => serde_json::Number::from_f64(*value)
            .map(serde_json::Value::Number)
            .ok_or_else(|| WdlError::RuntimeError {
                message: format!("Cannot represent float {} in JSON", value),
            }),
        Value::String { value, .. } => Ok(serde_json::Value::String(value.clone())),
        Value::File { value, .. } => Ok(serde_json::Value::String(value.clone())),
        Value::Directory { value, .. } => Ok(serde_json::Value::String(value.clone())),
        Value::Array { values, .. } => {
            let mut json_array = Vec::new();
            for item in values {
                json_array.push(value_to_json(item)?);
            }
            Ok(serde_json::Value::Array(json_array))
        }
        Value::Map { pairs, .. } => {
            let mut json_map = serde_json::Map::new();
            for (key, val) in pairs {
                let key_str = match key {
                    Value::String { value, .. } => value.clone(),
                    _ => key.to_string(),
                };
                json_map.insert(key_str, value_to_json(val)?);
            }
            Ok(serde_json::Value::Object(json_map))
        }
        Value::Pair { left, right, .. } => {
            let json_array = vec![value_to_json(left)?, value_to_json(right)?];
            Ok(serde_json::Value::Array(json_array))
        }
        Value::Struct { members, .. } => {
            let mut json_map = serde_json::Map::new();
            for (name, val) in members {
                json_map.insert(name.clone(), value_to_json(val)?);
            }
            Ok(serde_json::Value::Object(json_map))
        }
    }
}

impl<F> WriteFileFunction<F>
where
    F: Fn(&Value) -> Result<String, WdlError> + Send + Sync,
{
    pub fn new(name: &'static str, input_type: Type, serializer: F) -> Self {
        Self {
            name,
            input_type,
            serializer,
        }
    }
}

impl<F> Function for WriteFileFunction<F>
where
    F: Fn(&Value) -> Result<String, WdlError> + Send + Sync,
{
    fn name(&self) -> &str {
        self.name
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 1 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name.to_string(),
                expected: 1,
                actual: args.len(),
            });
        }

        // Check if input type matches expected
        if !args[0].coerces(&self.input_type, false) {
            return Err(WdlError::TypeMismatch {
                expected: self.input_type.clone(),
                actual: args[0].clone(),
            });
        }

        Ok(Type::file(false))
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        // Default implementation - create temp file without path mapping
        use std::io::Write;

        if args.len() != 1 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name.to_string(),
                expected: 1,
                actual: args.len(),
            });
        }

        // Serialize the data using the provided serializer
        let content = (self.serializer)(&args[0])?;

        // Create a temporary file
        let mut temp_file = std::env::temp_dir();
        temp_file.push(format!(
            "{}_{}.txt",
            self.name,
            std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap_or_default()
                .as_nanos()
        ));

        let mut file = std::fs::File::create(&temp_file).map_err(|e| WdlError::RuntimeError {
            message: format!("Failed to create temporary file: {}", e),
        })?;

        file.write_all(content.as_bytes())
            .map_err(|e| WdlError::RuntimeError {
                message: format!("Failed to write to file: {}", e),
            })?;

        Value::file(temp_file.to_string_lossy().to_string())
    }

    fn eval_with_stdlib(
        &self,
        args: &[Value],
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Value, WdlError> {
        use std::io::Write;

        if args.len() != 1 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name.to_string(),
                expected: 1,
                actual: args.len(),
            });
        }

        // Serialize the data using the provided serializer
        let content = (self.serializer)(&args[0])?;

        // Create file in task directory if available, otherwise fall back to temp dir
        let output_file = if let Some(task_dir) = stdlib.task_dir() {
            let mut path = task_dir.clone();
            path.push(format!(
                "{}_{}.txt",
                self.name,
                std::time::SystemTime::now()
                    .duration_since(std::time::UNIX_EPOCH)
                    .unwrap_or_default()
                    .as_nanos()
            ));
            path
        } else {
            let mut temp_file = std::env::temp_dir();
            temp_file.push(format!(
                "{}_{}.txt",
                self.name,
                std::time::SystemTime::now()
                    .duration_since(std::time::UNIX_EPOCH)
                    .unwrap_or_default()
                    .as_nanos()
            ));
            temp_file
        };

        let mut file = std::fs::File::create(&output_file).map_err(|e| WdlError::RuntimeError {
            message: format!("Failed to create output file: {}", e),
        })?;

        file.write_all(content.as_bytes())
            .map_err(|e| WdlError::RuntimeError {
                message: format!("Failed to write to file: {}", e),
            })?;

        // Use path mapper to virtualize the filename
        let virtual_path = stdlib.path_mapper().virtualize_filename(&output_file)?;
        Value::file(virtual_path)
    }
}

// For backward compatibility with tests, we keep type aliases
pub type ReadIntFunction = ReadFileFunction<fn(&str) -> Result<Value, WdlError>>;
pub type ReadFloatFunction = ReadFileFunction<fn(&str) -> Result<Value, WdlError>>;
pub type ReadBooleanFunction = ReadFileFunction<fn(&str) -> Result<Value, WdlError>>;

/// Size function that calculates the size of files, directories, or compound values
pub struct SizeFunction;

impl Function for SizeFunction {
    fn name(&self) -> &str {
        "size"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.is_empty() || args.len() > 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: "size".to_string(),
                expected: 1, // Can be 1 or 2, but we'll check in eval
                actual: args.len(),
            });
        }

        // First argument can be File, Directory, Array, Map, StructInstance, or compound types
        match &args[0] {
            Type::File { .. } | Type::Directory { .. } => {}
            Type::Array { .. } | Type::Map { .. } | Type::StructInstance { .. } => {}
            _ => {
                return Err(WdlError::TypeMismatch {
                    expected: Type::File { optional: false },
                    actual: args[0].clone(),
                });
            }
        }

        // Second argument (if present) must be String (unit)
        if args.len() == 2 {
            match &args[1] {
                Type::String { .. } => {}
                _ => {
                    return Err(WdlError::TypeMismatch {
                        expected: Type::String { optional: false },
                        actual: args[1].clone(),
                    });
                }
            }
        }

        Ok(Type::Float { optional: false })
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        if args.is_empty() || args.len() > 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: "size".to_string(),
                expected: 1,
                actual: args.len(),
            });
        }

        let unit = if args.len() == 2 {
            match &args[1] {
                Value::String { value, .. } => value.as_str(),
                _ => "B", // Default to bytes
            }
        } else {
            "B" // Default to bytes
        };

        let size_bytes = calculate_size(&args[0])?;
        let size_in_unit = convert_bytes_to_unit(size_bytes, unit)?;

        Ok(Value::Float {
            value: size_in_unit,
            wdl_type: Type::Float { optional: false },
        })
    }

    fn eval_with_stdlib(
        &self,
        args: &[Value],
        stdlib: &crate::stdlib::StdLib,
    ) -> Result<Value, WdlError> {
        if args.is_empty() || args.len() > 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: "size".to_string(),
                expected: 1,
                actual: args.len(),
            });
        }

        let unit = if args.len() == 2 {
            match &args[1] {
                Value::String { value, .. } => value.as_str(),
                _ => "B", // Default to bytes
            }
        } else {
            "B" // Default to bytes
        };

        // Calculate size with path resolution through stdlib
        let size_bytes = calculate_size_with_stdlib(&args[0], stdlib)?;
        let size_in_unit = convert_bytes_to_unit(size_bytes, unit)?;

        Ok(Value::Float {
            value: size_in_unit,
            wdl_type: Type::Float { optional: false },
        })
    }
}

/// Calculate the total size of a value in bytes with path resolution
fn calculate_size_with_stdlib(
    value: &Value,
    stdlib: &crate::stdlib::StdLib,
) -> Result<f64, WdlError> {
    match value {
        Value::File { value: path, .. } => {
            // Resolve the file path using the path mapper
            let resolved_path = stdlib.path_mapper().devirtualize_filename(path)?;
            if resolved_path.exists() {
                if resolved_path.is_file() {
                    std::fs::metadata(&resolved_path)
                        .map_err(|e| WdlError::RuntimeError {
                            message: format!("Failed to get file size for {}: {}", path, e),
                        })
                        .map(|m| m.len() as f64)
                } else {
                    Ok(0.0) // Non-file paths have size 0
                }
            } else {
                Ok(0.0) // Non-existent files have size 0
            }
        }
        Value::Directory { value: path, .. } => {
            // Resolve the directory path using the path mapper
            let resolved_path = stdlib.path_mapper().devirtualize_filename(path)?;
            if resolved_path.exists() && resolved_path.is_dir() {
                calculate_directory_size(&resolved_path)
            } else {
                Ok(0.0) // Non-existent directories have size 0
            }
        }
        Value::Array { values, .. } => {
            let mut total = 0.0;
            for item in values {
                total += calculate_size_with_stdlib(item, stdlib)?;
            }
            Ok(total)
        }
        Value::Map { pairs, .. } => {
            let mut total = 0.0;
            for (_key, val) in pairs {
                total += calculate_size_with_stdlib(val, stdlib)?;
            }
            Ok(total)
        }
        Value::Struct { members, .. } => {
            let mut total = 0.0;
            for val in members.values() {
                total += calculate_size_with_stdlib(val, stdlib)?;
            }
            Ok(total)
        }
        Value::Pair { left, right, .. } => {
            Ok(calculate_size_with_stdlib(left, stdlib)?
                + calculate_size_with_stdlib(right, stdlib)?)
        }
        // For non-file values, treat as 0 bytes
        Value::Null => Ok(0.0),
        _ => Ok(0.0),
    }
}

/// Calculate the total size of a value in bytes
fn calculate_size(value: &Value) -> Result<f64, WdlError> {
    match value {
        Value::File { value: path, .. } => {
            let file_path = Path::new(path);
            if file_path.exists() {
                if file_path.is_file() {
                    std::fs::metadata(file_path)
                        .map_err(|e| WdlError::RuntimeError {
                            message: format!("Failed to get file size for {}: {}", path, e),
                        })
                        .map(|m| m.len() as f64)
                } else {
                    Ok(0.0) // Non-existent files have size 0
                }
            } else {
                Ok(0.0) // Non-existent files have size 0
            }
        }
        Value::Directory { value: path, .. } => {
            let dir_path = Path::new(path);
            if dir_path.exists() && dir_path.is_dir() {
                calculate_directory_size(dir_path)
            } else {
                Ok(0.0) // Non-existent directories have size 0
            }
        }
        Value::Array { values, .. } => {
            let mut total = 0.0;
            for item in values {
                total += calculate_size(item)?;
            }
            Ok(total)
        }
        Value::Map { pairs, .. } => {
            let mut total = 0.0;
            for (_key, val) in pairs {
                total += calculate_size(val)?;
            }
            Ok(total)
        }
        Value::Struct { members, .. } => {
            let mut total = 0.0;
            for val in members.values() {
                total += calculate_size(val)?;
            }
            Ok(total)
        }
        Value::Pair { left, right, .. } => Ok(calculate_size(left)? + calculate_size(right)?),
        // For non-file values, treat as 0 bytes
        Value::Null => Ok(0.0),
        _ => Ok(0.0),
    }
}

/// Recursively calculate directory size
fn calculate_directory_size(dir_path: &Path) -> Result<f64, WdlError> {
    let mut total = 0.0;

    let entries = std::fs::read_dir(dir_path).map_err(|e| WdlError::RuntimeError {
        message: format!("Failed to read directory {}: {}", dir_path.display(), e),
    })?;

    for entry in entries {
        let entry = entry.map_err(|e| WdlError::RuntimeError {
            message: format!("Failed to read directory entry: {}", e),
        })?;

        let path = entry.path();
        if path.is_file() {
            let metadata = std::fs::metadata(&path).map_err(|e| WdlError::RuntimeError {
                message: format!("Failed to get metadata for {}: {}", path.display(), e),
            })?;
            total += metadata.len() as f64;
        } else if path.is_dir() {
            total += calculate_directory_size(&path)?;
        }
    }

    Ok(total)
}

/// Convert bytes to the specified unit
/// According to WDL spec: decimal units (K, KB, MB, etc.) use 1000, binary units (KiB, MiB, etc.) use 1024
fn convert_bytes_to_unit(bytes: f64, unit: &str) -> Result<f64, WdlError> {
    match unit.to_uppercase().as_str() {
        "B" => Ok(bytes),
        // Decimal units (base 1000)
        "K" | "KB" => Ok(bytes / 1000.0),
        "M" | "MB" => Ok(bytes / (1000.0 * 1000.0)),
        "G" | "GB" => Ok(bytes / (1000.0 * 1000.0 * 1000.0)),
        "T" | "TB" => Ok(bytes / (1000.0 * 1000.0 * 1000.0 * 1000.0)),
        "P" | "PB" => Ok(bytes / (1000.0 * 1000.0 * 1000.0 * 1000.0 * 1000.0)),
        // Binary units (base 1024)
        "KIB" => Ok(bytes / 1024.0),
        "MIB" => Ok(bytes / (1024.0 * 1024.0)),
        "GIB" => Ok(bytes / (1024.0 * 1024.0 * 1024.0)),
        "TIB" => Ok(bytes / (1024.0 * 1024.0 * 1024.0 * 1024.0)),
        "PIB" => Ok(bytes / (1024.0 * 1024.0 * 1024.0 * 1024.0 * 1024.0)),
        _ => Err(WdlError::RuntimeError {
            message: format!(
                "Invalid size unit: {}. Valid units are B, K/KB, M/MB, G/GB, T/TB, P/PB (decimal) or KiB, MiB, GiB, TiB, PiB (binary)",
                unit
            ),
        }),
    }
}

/// Helper function to create size function
pub fn create_size() -> Box<dyn Function> {
    Box::new(SizeFunction)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::stdlib::StdLib;
    use crate::Type;
    use crate::Value;
    use crate::WdlError;
    use std::fs;
    use std::path::PathBuf;
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
    #[allow(clippy::approx_constant)]
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
    fn test_read_tsv_with_headers_now_implemented() {
        let stdlib = StdLib::new("1.2");

        // This test now verifies that read_tsv supports multiple arguments
        let read_tsv_func = stdlib
            .get_function("read_tsv")
            .expect("read_tsv function should exist");

        // Test type inference for 2 arguments
        let args = vec![
            Type::String { optional: false },
            Type::Boolean { optional: false },
        ];
        let result = read_tsv_func.infer_type(&args);
        assert!(result.is_ok(), "read_tsv should now accept 2 arguments");

        // Test type inference for 3 arguments
        let args = vec![
            Type::String { optional: false },
            Type::Boolean { optional: false },
            Type::Array {
                item_type: Box::new(Type::String { optional: false }),
                optional: false,
                nonempty: false,
            },
        ];
        let result = read_tsv_func.infer_type(&args);
        assert!(result.is_ok(), "read_tsv should now accept 3 arguments");
    }

    #[test]
    fn test_read_tsv_with_headers_expected_behavior() {
        use std::io::Write;
        let stdlib = StdLib::new("1.2");

        // Create a test TSV file with headers
        let mut temp_file = std::env::temp_dir();
        temp_file.push("test_headers.tsv");
        let mut file = std::fs::File::create(&temp_file).unwrap();
        writeln!(file, "name\tvalue").unwrap();
        writeln!(file, "row1\tvalue1").unwrap();
        writeln!(file, "row2\tvalue2").unwrap();

        let read_tsv_func = stdlib.get_function("read_tsv").unwrap();

        // Test read_tsv(file, true) - use headers from file
        let args = vec![
            Value::String {
                value: temp_file.to_string_lossy().to_string(),
                wdl_type: Type::String { optional: false },
            },
            Value::Boolean {
                value: true,
                wdl_type: Type::Boolean { optional: false },
            },
        ];

        let result = read_tsv_func.eval(&args);
        assert!(result.is_ok(), "read_tsv with headers should work");

        if let Ok(Value::Array { values, .. }) = result {
            assert_eq!(values.len(), 2, "Should have 2 data rows");

            // Check first object
            if let Value::Map { pairs, .. } = &values[0] {
                assert_eq!(pairs.len(), 2, "Each object should have 2 fields");
                // Find the "name" field
                let name_value = pairs
                    .iter()
                    .find(|(k, _)| {
                        if let Value::String { value, .. } = k {
                            value == "name"
                        } else {
                            false
                        }
                    })
                    .map(|(_, v)| v);

                if let Some(Value::String { value, .. }) = name_value {
                    assert_eq!(value, "row1", "First row name should be 'row1'");
                }
            }
        }

        // Clean up
        std::fs::remove_file(temp_file).ok();
    }

    #[test]
    fn test_read_tsv_object_coercion_issue() {
        use crate::{value::ValueBase, Type};
        use std::io::Write;

        let stdlib = StdLib::new("1.2");

        // Create a test TSV file with headers
        let mut temp_file = std::env::temp_dir();
        temp_file.push("test_object_coercion.tsv");
        let mut file = std::fs::File::create(&temp_file).unwrap();
        writeln!(file, "header1\theader2").unwrap();
        writeln!(file, "row1\tvalue1").unwrap();
        writeln!(file, "row2\tvalue2").unwrap();

        let read_tsv_func = stdlib.get_function("read_tsv").unwrap();

        // Test: WDL expects Array[Object] but we return Array[Map[String,String]]
        let args = vec![
            Value::String {
                value: temp_file.to_string_lossy().to_string(),
                wdl_type: Type::String { optional: false },
            },
            Value::Boolean {
                value: true,
                wdl_type: Type::Boolean { optional: false },
            },
        ];

        // This should return Array[Object] according to WDL spec
        let result_type = read_tsv_func.infer_type(&[
            Type::String { optional: false },
            Type::Boolean { optional: false },
        ]);

        println!(
            "read_tsv(File, Boolean) infer_type result: {:?}",
            result_type
        );

        // The issue: we return Array[Map[String,String]] but WDL expects Array[Object]
        // This should demonstrate the coercion issue
        let result = read_tsv_func.eval(&args);

        if let Ok(value) = &result {
            // Check what type we actually get
            match value {
                Value::Array {
                    wdl_type, values, ..
                } => {
                    println!("Returned Array with type: {:?}", wdl_type);
                    if let Some(first) = values.first() {
                        match first {
                            Value::Map {
                                wdl_type: map_type, ..
                            } => {
                                println!("First element is Map with type: {:?}", map_type);
                            }
                            _ => println!("First element is not a Map: {:?}", first),
                        }
                    }
                }
                _ => println!("Result is not an Array: {:?}", value),
            }

            // Try to coerce to Array[Object]
            let object_array_type = Type::Array {
                item_type: Box::new(Type::Object {
                    members: std::collections::HashMap::new(),
                }),
                optional: false,
                nonempty: false,
            };

            let coerced = value.coerce(&object_array_type);
            println!("Coercion to Array[Object] result: {:?}", coerced.is_ok());

            if let Err(e) = coerced {
                println!("Coercion error: {:?}", e);
                // This demonstrates our problem - Map can't coerce to Object
                println!("SUCCESS: Test reproduced the bug - Map cannot coerce to Object");
            } else {
                println!("WARNING: Coercion succeeded, but we expected it to fail");
            }
        }

        // Clean up
        std::fs::remove_file(temp_file).ok();
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

    #[test]
    fn test_write_lines_empty_array() {
        // Test that write_lines with empty array produces an empty file (no newline)
        let func = create_write_lines();

        // Create empty array
        let empty_array = Value::Array {
            values: vec![],
            wdl_type: Type::Array {
                item_type: Box::new(Type::String { optional: false }),
                optional: false,
                nonempty: false,
            },
        };

        let result = func.eval(&[empty_array]);
        assert!(result.is_ok());

        // Get the file path and read its contents
        if let Ok(Value::File { value: path, .. }) = result {
            let contents = std::fs::read_to_string(&path).unwrap();
            // Empty array should produce empty file (no content, no newline)
            assert_eq!(
                contents, "",
                "Empty array should produce empty file, but got: {:?}",
                contents
            );
        } else {
            panic!("Expected File value");
        }
    }

    #[test]
    fn test_write_lines_with_content() {
        // Test that write_lines with non-empty array works correctly
        let func = create_write_lines();

        // Create array with strings
        let array = Value::Array {
            values: vec![
                Value::String {
                    value: "line1".to_string(),
                    wdl_type: Type::String { optional: false },
                },
                Value::String {
                    value: "line2".to_string(),
                    wdl_type: Type::String { optional: false },
                },
            ],
            wdl_type: Type::Array {
                item_type: Box::new(Type::String { optional: false }),
                optional: false,
                nonempty: false,
            },
        };

        let result = func.eval(&[array]);
        assert!(result.is_ok());

        // Get the file path and read its contents
        if let Ok(Value::File { value: path, .. }) = result {
            let contents = std::fs::read_to_string(&path).unwrap();
            // Non-empty array should have lines joined with newlines and a trailing newline
            assert_eq!(
                contents, "line1\nline2\n",
                "Expected 'line1\\nline2\\n', but got: {:?}",
                contents
            );
        } else {
            panic!("Expected File value");
        }
    }
}
