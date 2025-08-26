//! String manipulation functions for WDL standard library

use super::Function;
use crate::error::WdlError;
use crate::types::Type;
use crate::value::Value;

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
