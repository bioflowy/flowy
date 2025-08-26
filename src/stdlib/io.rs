//! Input/Output functions for WDL standard library

use crate::error::WdlError;
use crate::types::Type;
use crate::value::Value;
use super::Function;

/// Stdout function - returns reference to stdout file
pub struct StdoutFunction;

impl Function for StdoutFunction {
    fn name(&self) -> &str { "stdout" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 0 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 0,
                actual: args.len(),
            });
        }
        
        Ok(Type::file(false))
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        if args.len() != 0 {
            return Err(WdlError::RuntimeError {
                message: format!("stdout() takes no arguments"),
            });
        }
        
        // Return stdout.txt file path - this should be overridden by task-specific stdlib
        Value::file("stdout.txt".to_string())
    }
}

/// Stderr function - returns reference to stderr file
pub struct StderrFunction;

impl Function for StderrFunction {
    fn name(&self) -> &str { "stderr" }
    
    fn infer_type(&self, args: &[Type]) -> Result<Type, WdlError> {
        if args.len() != 0 {
            return Err(WdlError::ArgumentCountMismatch {
                function: self.name().to_string(),
                expected: 0,
                actual: args.len(),
            });
        }
        
        Ok(Type::file(false))
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        if args.len() != 0 {
            return Err(WdlError::RuntimeError {
                message: format!("stderr() takes no arguments"),
            });
        }
        
        // Return stderr.txt file path - this should be overridden by task-specific stdlib
        Value::file("stderr.txt".to_string())
    }
}

/// Write lines function - writes array of strings to a file
pub struct WriteLinesFunction;

impl Function for WriteLinesFunction {
    fn name(&self) -> &str { "write_lines" }
    
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
                message: format!("write_lines() argument must be Array[String]"),
            });
        }
        
        Ok(Type::file(false))
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        let _array = args[0].as_array().ok_or_else(|| WdlError::RuntimeError {
            message: format!("write_lines() argument must be Array[String]"),
        })?;
        
        // For now, create a temporary file name - runtime should handle actual file creation
        let filename = format!("__WRITE_LINES_{}.txt", chrono::Utc::now().timestamp_nanos_opt().unwrap_or(0));
        
        // In a real implementation, we would write the lines to the file here
        // For now, just return the filename
        Value::file(filename)
    }
}

/// Read lines function - reads lines from a file into an array
pub struct ReadLinesFunction;

impl Function for ReadLinesFunction {
    fn name(&self) -> &str { "read_lines" }
    
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
                message: format!("read_lines() argument must be String or File"),
            });
        }
        
        Ok(Type::array(Type::string(false), false, false))
    }
    
    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        let _filename = match &args[0] {
            Value::String { value, .. } => value.clone(),
            Value::File { value, .. } => value.clone(),
            _ => return Err(WdlError::RuntimeError {
                message: format!("read_lines() argument must be String or File"),
            }),
        };
        
        // For now, return empty array - runtime should handle actual file reading
        Ok(Value::array(Type::string(false), vec![]))
    }
}