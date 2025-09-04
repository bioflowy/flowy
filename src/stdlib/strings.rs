//! String manipulation functions for WDL standard library

use super::Function;
use crate::error::WdlError;
use crate::types::Type;
use crate::value::Value;
use regex::Regex;

/// Find function - searches for regex pattern within input string
pub struct FindFunction;

impl Function for FindFunction {
    fn name(&self) -> &str {
        "find"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 2,
                actual: args.len(),
            });
        }

        if !matches!(args[0], Type::String { .. }) || !matches!(args[1], Type::String { .. }) {
            return Err(WdlError::RuntimeError {
                message: "find() expects two String arguments".to_string(),
            });
        }

        // Returns String? (optional String)
        Ok(Type::string(true))
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        let input = args[0].as_string().ok_or_else(|| WdlError::RuntimeError {
            message: "find() first argument must be String".to_string(),
        })?;

        let pattern = args[1].as_string().ok_or_else(|| WdlError::RuntimeError {
            message: "find() second argument must be String".to_string(),
        })?;

        // Compile the regex pattern as POSIX Extended Regular Expression (ERE)
        let regex = match Regex::new(pattern) {
            Ok(r) => r,
            Err(e) => {
                return Err(WdlError::RuntimeError {
                    message: format!("Invalid regex pattern '{}': {}", pattern, e),
                });
            }
        };

        // Find the first match
        match regex.find(input) {
            Some(m) => {
                // Return the matched string
                Ok(Value::String {
                    value: m.as_str().to_string(),
                    wdl_type: Type::string(false),
                })
            }
            None => {
                // No match found - return None (represented as Null with optional type)
                Ok(Value::Null)
            }
        }
    }
}

/// Substitute function - performs regex substitution on strings
pub struct SubFunction;

impl Function for SubFunction {
    fn name(&self) -> &str {
        "sub"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 3 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 3,
                actual: args.len(),
            });
        }

        if !matches!(args[0], Type::String { .. })
            || !matches!(args[1], Type::String { .. })
            || !matches!(args[2], Type::String { .. })
        {
            return Err(WdlError::RuntimeError {
                message: "sub() expects three String arguments".to_string(),
            });
        }

        Ok(Type::string(false))
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        let input = args[0].as_string().ok_or_else(|| WdlError::RuntimeError {
            message: "sub() first argument must be String".to_string(),
        })?;

        let pattern = args[1].as_string().ok_or_else(|| WdlError::RuntimeError {
            message: "sub() second argument must be String".to_string(),
        })?;

        let replacement = args[2].as_string().ok_or_else(|| WdlError::RuntimeError {
            message: "sub() third argument must be String".to_string(),
        })?;

        // Simple string replacement for now (not full regex)
        let result = input.replace(&pattern, replacement);
        Ok(Value::string(result))
    }
}

/// Basename function - extracts the filename from a path
pub struct BasenameFunction;

impl Function for BasenameFunction {
    fn name(&self) -> &str {
        "basename"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.is_empty() || args.len() > 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 1, // or 2
                actual: args.len(),
            });
        }

        if !matches!(args[0], Type::String { .. } | Type::File { .. }) {
            return Err(WdlError::RuntimeError {
                message: "basename() first argument must be String or File".to_string(),
            });
        }

        if args.len() == 2 && !matches!(args[1], Type::String { optional: true, .. }) {
            return Err(WdlError::RuntimeError {
                message: "basename() second argument must be String?".to_string(),
            });
        }

        Ok(Type::string(false))
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        let path = args[0].as_string().ok_or_else(|| WdlError::RuntimeError {
            message: "basename() first argument must be String".to_string(),
        })?;

        let base = path.rsplit('/').next().unwrap_or(path);

        if args.len() == 2 {
            if let Some(suffix) = args[1].as_string() {
                if base.ends_with(&suffix) {
                    let trimmed = &base[..base.len() - suffix.len()];
                    return Ok(Value::string(trimmed.to_string()));
                }
            }
        }

        Ok(Value::string(base.to_string()))
    }
}

/// Join paths function - joins file system paths together
pub struct JoinPathsFunction;

impl Function for JoinPathsFunction {
    fn name(&self) -> &str {
        "join_paths"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.is_empty() || args.len() > 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 2,
                actual: args.len(),
            });
        }

        match args.len() {
            1 => {
                // join_paths(Array[String]+)
                if let Type::Array {
                    item_type,
                    nonempty,
                    ..
                } = &args[0]
                {
                    if !matches!(item_type.as_ref(), Type::String { .. }) {
                        return Err(WdlError::RuntimeError {
                            message: "join_paths() with single argument requires Array[String]"
                                .to_string(),
                        });
                    }
                    if !nonempty {
                        return Err(WdlError::RuntimeError {
                            message: "join_paths() requires non-empty array".to_string(),
                        });
                    }
                } else {
                    return Err(WdlError::RuntimeError {
                        message: "join_paths() with single argument requires Array[String]+"
                            .to_string(),
                    });
                }
            }
            2 => {
                // join_paths(File, String) or join_paths(File, Array[String]+)
                if !matches!(args[0], Type::File { .. } | Type::String { .. }) {
                    return Err(WdlError::RuntimeError {
                        message: "join_paths() first argument must be File or String".to_string(),
                    });
                }

                match &args[1] {
                    Type::String { .. } => {
                        // join_paths(File, String) - OK
                    }
                    Type::Array {
                        item_type,
                        nonempty,
                        ..
                    } => {
                        if !matches!(item_type.as_ref(), Type::String { .. }) {
                            return Err(WdlError::RuntimeError {
                                message: "join_paths() second argument array must contain String elements".to_string(),
                            });
                        }
                        if !nonempty {
                            return Err(WdlError::RuntimeError {
                                message: "join_paths() requires non-empty array".to_string(),
                            });
                        }
                    }
                    _ => {
                        return Err(WdlError::RuntimeError {
                            message:
                                "join_paths() second argument must be String or Array[String]+"
                                    .to_string(),
                        });
                    }
                }
            }
            _ => unreachable!(),
        }

        // Returns File type
        Ok(Type::file(false))
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        let paths: Vec<String> = match args.len() {
            1 => {
                // join_paths(Array[String]+)
                if let Value::Array { values, .. } = &args[0] {
                    values
                        .iter()
                        .map(|v| {
                            v.as_string().ok_or_else(|| WdlError::RuntimeError {
                                message: "join_paths() array elements must be String".to_string(),
                            })
                        })
                        .collect::<Result<Vec<_>, _>>()?
                        .into_iter()
                        .map(|s| s.to_string())
                        .collect()
                } else {
                    return Err(WdlError::RuntimeError {
                        message: "join_paths() argument must be Array".to_string(),
                    });
                }
            }
            2 => {
                // join_paths(File, String) or join_paths(File, Array[String]+)
                let mut result = vec![];

                // First argument (File or String)
                let first_path = args[0].as_string().ok_or_else(|| WdlError::RuntimeError {
                    message: "join_paths() first argument must be String-like".to_string(),
                })?;
                result.push(first_path.to_string());

                // Second argument (String or Array[String])
                match &args[1] {
                    Value::String { value, .. } => {
                        result.push(value.clone());
                    }
                    Value::Array { values, .. } => {
                        for v in values {
                            let path = v.as_string().ok_or_else(|| WdlError::RuntimeError {
                                message: "join_paths() array elements must be String".to_string(),
                            })?;
                            result.push(path.to_string());
                        }
                    }
                    _ => {
                        return Err(WdlError::RuntimeError {
                            message: "join_paths() second argument must be String or Array"
                                .to_string(),
                        });
                    }
                }
                result
            }
            _ => {
                return Err(WdlError::ArgumentCountMismatch {
                    function: self.name().to_string(),
                    expected: 2,
                    actual: args.len(),
                });
            }
        };

        if paths.is_empty() {
            return Err(WdlError::RuntimeError {
                message: "join_paths() requires at least one path".to_string(),
            });
        }

        // Validate that only the first path can be absolute
        for (i, path) in paths.iter().enumerate() {
            if i > 0 && path.starts_with('/') {
                return Err(WdlError::RuntimeError {
                    message: format!("join_paths() only the first path can be absolute, but found absolute path at position {}: {}", i, path),
                });
            }
        }

        // Join the paths using std::path
        use std::path::Path;
        let mut result_path = std::path::PathBuf::from(&paths[0]);

        for path in &paths[1..] {
            result_path = result_path.join(path);
        }

        // Convert to absolute path if relative (relative to current working directory)
        let final_path = if result_path.is_absolute() {
            result_path
        } else {
            std::env::current_dir()
                .map_err(|e| WdlError::RuntimeError {
                    message: format!("join_paths() failed to get current directory: {}", e),
                })?
                .join(result_path)
        };

        // Convert to string and return as File value
        let path_str = final_path.to_string_lossy().to_string();
        Value::file(path_str)
    }
}

/// Sep function - joins array elements with a separator
pub struct SepFunction;

impl Function for SepFunction {
    fn name(&self) -> &str {
        "sep"
    }

    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 2,
                actual: args.len(),
            });
        }

        if !matches!(args[0], Type::String { .. }) {
            return Err(WdlError::RuntimeError {
                message: "sep() first argument must be String".to_string(),
            });
        }

        if !matches!(args[1], Type::Array { .. }) {
            return Err(WdlError::RuntimeError {
                message: "sep() second argument must be Array".to_string(),
            });
        }

        Ok(Type::string(false))
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        let separator = args[0].as_string().ok_or_else(|| WdlError::RuntimeError {
            message: "sep() first argument must be String".to_string(),
        })?;

        if let Value::Array { values, .. } = &args[1] {
            let strings: Result<Vec<String>, _> = values
                .iter()
                .map(|v| {
                    v.as_string()
                        .map(|s| s.to_string())
                        .ok_or_else(|| WdlError::RuntimeError {
                            message: "sep() array elements must be String".to_string(),
                        })
                })
                .collect();

            Ok(Value::string(strings?.join(separator)))
        } else {
            Err(WdlError::RuntimeError {
                message: "sep() second argument must be Array".to_string(),
            })
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::stdlib::StdLib;

    #[test]
    fn test_find_function_implemented() {
        // Test that find function is now implemented
        let stdlib = StdLib::new("1.2");

        // Try to get find function - should return Some because it's now implemented
        let find_fn = stdlib.get_function("find");

        // This should be Some because find is now implemented
        assert!(find_fn.is_some(), "find function should be implemented");
    }

    #[test]
    fn test_find_function_basic_match() {
        // Test the basic functionality we expect from find function
        // This test should also fail initially since find is not implemented
        let stdlib = StdLib::new("1.2");

        // Test case from WDL spec: find("hello world", "e..o") should return "ello"
        let find_fn = stdlib.get_function("find");

        if let Some(func) = find_fn {
            let result = func.eval(&[
                Value::string("hello world".to_string()),
                Value::string("e..o".to_string()),
            ]);

            // This will succeed once find is implemented
            match result {
                Ok(value) => {
                    // For String? return type, should be a string value with "ello"
                    if let Some(found_str) = value.as_string() {
                        assert_eq!(found_str, "ello");
                    } else {
                        panic!(
                            "find should return String value with 'ello', got: {:?}",
                            value
                        );
                    }
                }
                Err(e) => {
                    panic!("find function should work: {}", e);
                }
            }
        } else {
            // Expected to fail since find is not implemented yet
            // This test documents what the behavior should be
            panic!("find function not implemented yet - this is expected initially");
        }
    }

    #[test]
    fn test_find_function_no_match() {
        // Test no match case from WDL spec
        let stdlib = StdLib::new("1.2");

        // Test case: find("hello world", "goodbye") should return None
        let find_fn = stdlib.get_function("find");

        if let Some(func) = find_fn {
            let result = func.eval(&[
                Value::string("hello world".to_string()),
                Value::string("goodbye".to_string()),
            ]);

            // This will succeed once find is implemented
            match result {
                Ok(value) => {
                    // For String? return type with no match, should be Value::Null
                    if value.is_null() {
                        // This is correct - no match returns None (represented as Null)
                        assert!(true);
                    } else {
                        panic!(
                            "find should return None (Null) for no match, got: {:?}",
                            value
                        );
                    }
                }
                Err(e) => {
                    panic!("find function should work: {}", e);
                }
            }
        } else {
            // Expected to fail since find is not implemented yet
            panic!("find function not implemented yet - this is expected initially");
        }
    }
}
