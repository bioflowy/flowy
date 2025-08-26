//! String manipulation functions for WDL standard library

use crate::error::WdlError;
use crate::types::Type;
use crate::value::Value;
use super::Function;

/// Substitute function - performs regex substitution on strings
pub struct SubFunction;

impl Function for SubFunction {
    fn name(&self) -> &str { "sub" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 3 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 3,
                actual: args.len(),
            });
        }
        
        if !matches!(args[0], Type::String { .. }) || 
           !matches!(args[1], Type::String { .. }) || 
           !matches!(args[2], Type::String { .. }) {
            return Err(WdlError::RuntimeError {
                message: format!("sub() expects three String arguments"),
            });
        }
        
        Ok(Type::string(false))
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        let input = args[0].as_string().ok_or_else(|| WdlError::RuntimeError {
            message: format!("sub() first argument must be String"),
        })?;
        
        let pattern = args[1].as_string().ok_or_else(|| WdlError::RuntimeError {
            message: format!("sub() second argument must be String"),
        })?;
        
        let replacement = args[2].as_string().ok_or_else(|| WdlError::RuntimeError {
            message: format!("sub() third argument must be String"),
        })?;
        
        // Simple string replacement for now (not full regex)
        let result = input.replace(&pattern, &replacement);
        Ok(Value::string(result))
    }
}

/// Basename function - extracts the filename from a path
pub struct BasenameFunction;

impl Function for BasenameFunction {
    fn name(&self) -> &str { "basename" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() < 1 || args.len() > 2 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 1, // or 2
                actual: args.len(),
            });
        }
        
        if !matches!(args[0], Type::String { .. } | Type::File { .. }) {
            return Err(WdlError::RuntimeError {
                message: format!("basename() first argument must be String or File"),
            });
        }
        
        if args.len() == 2 && !matches!(args[1], Type::String { optional: true, .. }) {
            return Err(WdlError::RuntimeError {
                message: format!("basename() second argument must be String?"),
            });
        }
        
        Ok(Type::string(false))
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        let path = args[0].as_string().ok_or_else(|| WdlError::RuntimeError {
            message: format!("basename() first argument must be String"),
        })?;
        
        let base = path.rsplit('/').next().unwrap_or(&path);
        
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
    fn name(&self) -> &str { "sep" }
    
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
                message: format!("sep() first argument must be String"),
            });
        }
        
        if !matches!(args[1], Type::Array { .. }) {
            return Err(WdlError::RuntimeError {
                message: format!("sep() second argument must be Array"),
            });
        }
        
        Ok(Type::string(false))
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        let separator = args[0].as_string().ok_or_else(|| WdlError::RuntimeError {
            message: format!("sep() first argument must be String"),
        })?;
        
        if let Value::Array { values, .. } = &args[1] {
            let strings: Result<Vec<String>, _> = values
                .iter()
                .map(|v| v.as_string().map(|s| s.to_string()).ok_or_else(|| WdlError::RuntimeError {
                    message: format!("sep() array elements must be String"),
                }))
                .collect();
            
            Ok(Value::string(strings?.join(&separator)))
        } else {
            Err(WdlError::RuntimeError {
                message: format!("sep() second argument must be Array"),
            })
        }
    }
}