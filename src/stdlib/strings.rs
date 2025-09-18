//! String manipulation functions for WDL standard library
//!
//! This module provides string manipulation functions as defined in the WDL specification.

use crate::error::WdlError;
use crate::types::Type;
use crate::value::Value;
use crate::stdlib::create_static_function;
use crate::expr::ExpressionBase;
use regex::Regex;

pub fn create_find_function() -> Box<dyn crate::stdlib::Function> {
    create_static_function(
        "find".to_string(),
        vec![Type::string(false), Type::string(false)], // input, pattern
        Type::string(true), // returns String? (optional)
        |args: &[Value]| -> Result<Value, WdlError> {
            let input = args[0].as_string().unwrap();
            let pattern = args[1].as_string().unwrap();

            // Compile the regex pattern as POSIX ERE
            let regex = Regex::new(&pattern).map_err(|e| WdlError::RuntimeError {
                message: format!("find(): invalid regex pattern '{}': {}", pattern, e),
            })?;

            // Find the first match
            match regex.find(&input) {
                Some(match_obj) => {
                    // Return the matched text
                    Ok(Value::string(match_obj.as_str().to_string()))
                }
                None => {
                    // No match found, return None
                    Ok(Value::null())
                }
            }
        },
    )
}

/// Create the sub function
///
/// Replaces all non-overlapping occurrences of pattern in input by replace string.
/// Pattern is evaluated as a POSIX Extended Regular Expression.
///
/// **Parameters**
/// 1. `String`: the input string
/// 2. `String`: the pattern to search for (POSIX ERE)
/// 3. `String`: the replacement string
///
/// **Returns**: The input string with all occurrences of pattern replaced by replacement string
pub fn create_sub_function() -> Box<dyn crate::stdlib::Function> {
    create_static_function(
        "sub".to_string(),
        vec![Type::string(false), Type::string(false), Type::string(false)], // input, pattern, replace
        Type::string(false), // returns String
        |args: &[Value]| -> Result<Value, WdlError> {
            let input = args[0].as_string().unwrap();
            let pattern = args[1].as_string().unwrap();
            let replace = args[2].as_string().unwrap();

            // Compile the regex pattern as POSIX ERE
            let regex = Regex::new(&pattern).map_err(|e| WdlError::RuntimeError {
                message: format!("sub(): invalid regex pattern '{}': {}", pattern, e),
            })?;

            // Replace all occurrences
            let result = regex.replace_all(&input, replace);
            Ok(Value::string(result.to_string()))
        },
    )
}

/// Basename function implementation
///
/// Returns the basename of a file or directory path, optionally removing a suffix.
/// Supports both File and Directory inputs, as well as 1 or 2 argument forms.
pub struct BasenameFunction {
    name: String,
}

/// Create the basename function
pub fn create_basename_function() -> Box<dyn crate::stdlib::Function> {
    Box::new(BasenameFunction::new())
}

/// JoinPaths function implementation
///
/// Joins together two or more paths into an absolute path.
/// Supports three variants:
/// 1. join_paths(File, String)
/// 2. join_paths(File, Array[String]+)
/// 3. join_paths(Array[String]+)
pub struct JoinPathsFunction {
    name: String,
}

impl JoinPathsFunction {
    pub fn new() -> Self {
        Self {
            name: "join_paths".to_string(),
        }
    }

    /// Join paths together using platform-specific path joining
    fn join_paths(&self, paths: &[String]) -> Result<String, WdlError> {
        use std::path::Path;
        
        if paths.is_empty() {
            return Err(WdlError::RuntimeError {
                message: "join_paths: cannot join empty path list".to_string(),
            });
        }

        let mut result = std::path::PathBuf::new();
        
        // First path can be absolute or relative
        let first_path = &paths[0];
        result.push(first_path);
        
        // Subsequent paths must be relative
        for (i, path) in paths[1..].iter().enumerate() {
            if path.starts_with('/') {
                return Err(WdlError::RuntimeError {
                    message: format!("join_paths: path at index {} ('{}') must be relative", i + 1, path),
                });
            }
            result.push(path);
        }

        // Convert to string
        result.to_str()
            .map(|s| s.to_string())
            .ok_or_else(|| WdlError::RuntimeError {
                message: "join_paths: result path contains invalid UTF-8".to_string(),
            })
    }
}

impl crate::stdlib::Function for JoinPathsFunction {
    fn infer_type(&self, args: &mut [crate::expr::Expression], _type_env: &crate::env::Bindings<crate::types::Type>, stdlib: &crate::stdlib::StdLib, struct_typedefs: &[crate::tree::StructTypeDef]) -> Result<crate::types::Type, WdlError> {
        // Check argument count (1 or 2 arguments allowed)
        if args.is_empty() || args.len() > 2 {
            let pos = if args.is_empty() {
                crate::error::SourcePosition::new("unknown".to_string(), "unknown".to_string(), 0, 0, 0, 0)
            } else {
                args[0].source_position().clone()
            };
            return Err(WdlError::Validation {
                pos,
                message: format!("Function 'join_paths' expects 1 or 2 arguments, got {}", args.len()),
                source_text: None,
                declared_wdl_version: None,
            });
        }

        let first_type = args[0].infer_type(_type_env, stdlib, struct_typedefs)?;

        if args.len() == 1 {
            // Variant 3: join_paths(Array[String]+)
            match first_type {
                Type::Array { item_type, .. } => {
                    if !matches!(item_type.as_ref(), Type::String { .. }) {
                        return Err(WdlError::Validation {
                            pos: args[0].source_position().clone(),
                            message: "Function 'join_paths' with single argument expects Array[String], got array of different type".to_string(),
                            source_text: None,
                            declared_wdl_version: None,
                        });
                    }
                }
                _ => {
                    return Err(WdlError::Validation {
                        pos: args[0].source_position().clone(),
                        message: format!("Function 'join_paths' with single argument expects Array[String], got {}", first_type),
                        source_text: None,
                        declared_wdl_version: None,
                    });
                }
            }
        } else {
            // Variant 1 or 2: join_paths(File, String) or join_paths(File, Array[String]+)
            if !matches!(first_type, Type::File { .. } | Type::String { .. }) {
                return Err(WdlError::Validation {
                    pos: args[0].source_position().clone(),
                    message: format!("Function 'join_paths' first argument must be File or String, got {}", first_type),
                    source_text: None,
                    declared_wdl_version: None,
                });
            }

            let second_type = args[1].infer_type(_type_env, stdlib, struct_typedefs)?;
            match second_type {
                Type::String { .. } => {
                    // Variant 1: join_paths(File, String)
                }
                Type::Array { item_type, .. } => {
                    // Variant 2: join_paths(File, Array[String]+)
                    if !matches!(item_type.as_ref(), Type::String { .. }) {
                        return Err(WdlError::Validation {
                            pos: args[1].source_position().clone(),
                            message: "Function 'join_paths' second argument array must contain String elements".to_string(),
                            source_text: None,
                            declared_wdl_version: None,
                        });
                    }
                }
                _ => {
                    return Err(WdlError::Validation {
                        pos: args[1].source_position().clone(),
                        message: format!("Function 'join_paths' second argument must be String or Array[String], got {}", second_type),
                        source_text: None,
                        declared_wdl_version: None,
                    });
                }
            }
        }

        // Always returns File
        Ok(Type::file(false))
    }

    fn eval(&self, args: &[crate::expr::Expression], env: &crate::env::Bindings<crate::value::Value>, stdlib: &crate::stdlib::StdLib) -> Result<crate::value::Value, WdlError> {
        // Check argument count
        if args.is_empty() || args.len() > 2 {
            return Err(WdlError::RuntimeError {
                message: format!("Function 'join_paths' expects 1 or 2 arguments, got {}", args.len()),
            });
        }

        let mut paths = Vec::new();

        if args.len() == 1 {
            // Variant 3: join_paths(Array[String]+)
            let array_value = args[0].eval(env, stdlib)?;
            match array_value {
                crate::value::Value::Array { values, .. } => {
                    for value in values {
                        match value {
                            crate::value::Value::String { value, .. } => {
                                paths.push(value);
                            }
                            _ => {
                                return Err(WdlError::RuntimeError {
                                    message: "Function 'join_paths' array must contain only String values".to_string(),
                                });
                            }
                        }
                    }
                }
                _ => {
                    return Err(WdlError::RuntimeError {
                        message: "Function 'join_paths' single argument must be Array[String]".to_string(),
                    });
                }
            }
        } else {
            // Variant 1 or 2: join_paths(File, String) or join_paths(File, Array[String]+)
            let first_value = args[0].eval(env, stdlib)?;
            let first_path = match first_value {
                crate::value::Value::File { value, .. } => value,
                crate::value::Value::String { value, .. } => value,
                _ => {
                    return Err(WdlError::RuntimeError {
                        message: "Function 'join_paths' first argument must be File or String".to_string(),
                    });
                }
            };
            paths.push(first_path);

            let second_value = args[1].eval(env, stdlib)?;
            match second_value {
                crate::value::Value::String { value, .. } => {
                    // Variant 1: join_paths(File, String)
                    paths.push(value);
                }
                crate::value::Value::Array { values, .. } => {
                    // Variant 2: join_paths(File, Array[String]+)
                    for value in values {
                        match value {
                            crate::value::Value::String { value, .. } => {
                                paths.push(value);
                            }
                            _ => {
                                return Err(WdlError::RuntimeError {
                                    message: "Function 'join_paths' array must contain only String values".to_string(),
                                });
                            }
                        }
                    }
                }
                _ => {
                    return Err(WdlError::RuntimeError {
                        message: "Function 'join_paths' second argument must be String or Array[String]".to_string(),
                    });
                }
            }
        }

        // Join the paths
        let result_path = self.join_paths(&paths)?;
        crate::value::Value::file(result_path)
    }

    fn name(&self) -> &str {
        &self.name
    }
}

/// Create the join_paths function
pub fn create_join_paths_function() -> Box<dyn crate::stdlib::Function> {
    Box::new(JoinPathsFunction::new())
}

impl BasenameFunction {
    pub fn new() -> Self {
        Self {
            name: "basename".to_string(),
        }
    }

    /// Extract basename from a path string
    fn get_basename(&self, path: &str, suffix: Option<&str>) -> String {
        use std::path::Path;
        
        let path_obj = Path::new(path);
        let basename = path_obj.file_name()
            .and_then(|name| name.to_str())
            .unwrap_or(path); // Fallback to original path if cannot extract filename
        
        // Remove suffix if provided and matches
        if let Some(suffix) = suffix {
            if basename.ends_with(suffix) && basename.len() > suffix.len() {
                let new_len = basename.len() - suffix.len();
                basename[..new_len].to_string()
            } else {
                basename.to_string()
            }
        } else {
            basename.to_string()
        }
    }
}

impl crate::stdlib::Function for BasenameFunction {
    fn infer_type(&self, args: &mut [crate::expr::Expression], _type_env: &crate::env::Bindings<crate::types::Type>, stdlib: &crate::stdlib::StdLib, struct_typedefs: &[crate::tree::StructTypeDef]) -> Result<crate::types::Type, WdlError> {
        // Check argument count (1 or 2 arguments allowed)
        if args.is_empty() || args.len() > 2 {
            let pos = if args.is_empty() {
                crate::error::SourcePosition::new("unknown".to_string(), "unknown".to_string(), 0, 0, 0, 0)
            } else {
                args[0].source_position().clone()
            };
            return Err(WdlError::Validation {
                pos,
                message: format!("Function 'basename' expects 1 or 2 arguments, got {}", args.len()),
                source_text: None,
                declared_wdl_version: None,
            });
        }

        // First argument must be File, Directory, or String (which can be coerced to File)
        let first_type = args[0].infer_type(_type_env, stdlib, struct_typedefs)?;
        if !matches!(first_type, Type::File { .. } | Type::Directory { .. } | Type::String { .. }) {
            return Err(WdlError::Validation {
                pos: args[0].source_position().clone(),
                message: format!("Function 'basename' first argument must be File, Directory, or String, got {}", first_type),
                source_text: None,
                declared_wdl_version: None,
            });
        }

        // Second argument (if present) must be String
        if args.len() == 2 {
            let second_type = args[1].infer_type(_type_env, stdlib, struct_typedefs)?;
            if !matches!(second_type, Type::String { .. }) {
                return Err(WdlError::Validation {
                    pos: args[1].source_position().clone(),
                    message: format!("Function 'basename' second argument must be String, got {}", second_type),
                    source_text: None,
                    declared_wdl_version: None,
                });
            }
        }

        // Always returns String
        Ok(Type::string(false))
    }

    fn eval(&self, args: &[crate::expr::Expression], env: &crate::env::Bindings<crate::value::Value>, stdlib: &crate::stdlib::StdLib) -> Result<crate::value::Value, WdlError> {
        // Check argument count
        if args.is_empty() || args.len() > 2 {
            return Err(WdlError::RuntimeError {
                message: format!("Function 'basename' expects 1 or 2 arguments, got {}", args.len()),
            });
        }

        // Evaluate first argument (File, Directory, or String)
        let path_value = args[0].eval(env, stdlib)?;
        let path_str = match path_value {
            crate::value::Value::File { value, .. } => value,
            crate::value::Value::Directory { value, .. } => value,
            crate::value::Value::String { value, .. } => value,
            _ => {
                return Err(WdlError::RuntimeError {
                    message: "Function 'basename' first argument must be File, Directory, or String".to_string(),
                });
            }
        };

        // Evaluate second argument (suffix) if present
        let suffix = if args.len() == 2 {
            let suffix_value = args[1].eval(env, stdlib)?;
            match suffix_value {
                crate::value::Value::String { value, .. } => Some(value),
                _ => {
                    return Err(WdlError::RuntimeError {
                        message: "Function 'basename' second argument must be String".to_string(),
                    });
                }
            }
        } else {
            None
        };

        // Get basename
        let basename = self.get_basename(&path_str, suffix.as_deref());
        Ok(crate::value::Value::string(basename))
    }

    fn name(&self) -> &str {
        &self.name
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::env::Bindings;

    #[test]
    fn test_find_function() {
        let find_fn = create_find_function();
        let env = Bindings::new();
        let stdlib = crate::stdlib::StdLib::new("1.2");

        // Test case from WDL spec: find("hello world", "e..o") -> "ello"
        let input = crate::expr::Expression::string_literal(crate::error::SourcePosition::new("test".to_string(), "test".to_string(), 1, 1, 1, 1), "hello world".to_string());
        let pattern = crate::expr::Expression::string_literal(crate::error::SourcePosition::new("test".to_string(), "test".to_string(), 1, 1, 1, 1), "e..o".to_string());
        let result = find_fn.eval(&[input, pattern], &env, &stdlib).unwrap();

        assert_eq!(result.as_string().unwrap(), "ello");

        // Test case: no match -> None
        let input2 = crate::expr::Expression::string_literal(crate::error::SourcePosition::new("test".to_string(), "test".to_string(), 1, 1, 1, 1), "hello world".to_string());
        let pattern2 = crate::expr::Expression::string_literal(crate::error::SourcePosition::new("test".to_string(), "test".to_string(), 1, 1, 1, 1), "goodbye".to_string());
        let result2 = find_fn.eval(&[input2, pattern2], &env, &stdlib).unwrap();

        assert!(result2.is_null());
    }

    #[test]
    fn test_find_with_tab_regex() {
        let find_fn = create_find_function();
        let env = Bindings::new();
        let stdlib = crate::stdlib::StdLib::new("1.2");

        // Test case from spec: find("hello\tBob", "\\t") -> "\t"
        let input = crate::expr::Expression::string_literal(crate::error::SourcePosition::new("test".to_string(), "test".to_string(), 1, 1, 1, 1), "hello\tBob".to_string());
        let pattern = crate::expr::Expression::string_literal(crate::error::SourcePosition::new("test".to_string(), "test".to_string(), 1, 1, 1, 1), "\\t".to_string());
        let result = find_fn.eval(&[input, pattern], &env, &stdlib).unwrap();

        assert_eq!(result.as_string().unwrap(), "\t");
    }

    #[test]
    fn test_sub_function() {
        let sub_fn = create_sub_function();
        let env = Bindings::new();
        let stdlib = crate::stdlib::StdLib::new("1.2");

        // Test case from WDL spec: sub("I like chocolate when\nit's late", "like", "love") -> "I love chocolate when\nit's late"
        let input = crate::expr::Expression::string_literal(crate::error::SourcePosition::new("test".to_string(), "test".to_string(), 1, 1, 1, 1), "I like chocolate when\nit's late".to_string());
        let pattern = crate::expr::Expression::string_literal(crate::error::SourcePosition::new("test".to_string(), "test".to_string(), 1, 1, 1, 1), "like".to_string());
        let replace = crate::expr::Expression::string_literal(crate::error::SourcePosition::new("test".to_string(), "test".to_string(), 1, 1, 1, 1), "love".to_string());
        let result = sub_fn.eval(&[input, pattern, replace], &env, &stdlib).unwrap();

        assert_eq!(result.as_string().unwrap(), "I love chocolate when\nit's late");

        // Test case: sub("late", "late$", "early") -> "I like chocolate when\nit's early"  
        let input2 = crate::expr::Expression::string_literal(crate::error::SourcePosition::new("test".to_string(), "test".to_string(), 1, 1, 1, 1), "I like chocolate when\nit's late".to_string());
        let pattern2 = crate::expr::Expression::string_literal(crate::error::SourcePosition::new("test".to_string(), "test".to_string(), 1, 1, 1, 1), "late$".to_string());
        let replace2 = crate::expr::Expression::string_literal(crate::error::SourcePosition::new("test".to_string(), "test".to_string(), 1, 1, 1, 1), "early".to_string());
        let result2 = sub_fn.eval(&[input2, pattern2, replace2], &env, &stdlib).unwrap();

        assert_eq!(result2.as_string().unwrap(), "I like chocolate when\nit's early");

        // Test case: newline replacement sub(chocolike, "\\n", " ") -> "I like chocolate when it's late"
        let input3 = crate::expr::Expression::string_literal(crate::error::SourcePosition::new("test".to_string(), "test".to_string(), 1, 1, 1, 1), "I like chocolate when\nit's late".to_string());
        let pattern3 = crate::expr::Expression::string_literal(crate::error::SourcePosition::new("test".to_string(), "test".to_string(), 1, 1, 1, 1), "\\n".to_string());
        let replace3 = crate::expr::Expression::string_literal(crate::error::SourcePosition::new("test".to_string(), "test".to_string(), 1, 1, 1, 1), " ".to_string());
        let result3 = sub_fn.eval(&[input3, pattern3, replace3], &env, &stdlib).unwrap();

        assert_eq!(result3.as_string().unwrap(), "I like chocolate when it's late");
    }
}