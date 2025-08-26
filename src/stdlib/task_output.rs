//! Task output-specific standard library implementation
//!
//! This module provides a specialized version of the standard library
//! that is context-aware for task execution, similar to miniwdl's OutputStdLib.

use crate::error::WdlError;
use crate::stdlib::{StdLib, Function};
use crate::types::Type;
use crate::value::Value;
use std::path::PathBuf;
use std::collections::HashMap;

/// Create a task output-specific standard library by overriding specific functions
/// 
/// This creates a regular StdLib but replaces stdout() and stderr() functions
/// with task-specific versions that know about the task execution directory.
pub fn create_task_output_stdlib(wdl_version: &str, task_dir: PathBuf) -> StdLib {
    let mut stdlib = StdLib::new(wdl_version);
    
    // Override stdout() function with task-specific version
    stdlib.add_function(Box::new(TaskStdoutFunction::new(task_dir.clone())));
    
    // Override stderr() function with task-specific version
    stdlib.add_function(Box::new(TaskStderrFunction::new(task_dir)));
    
    stdlib
}

/// Task-specific stdout function that returns the correct path
pub struct TaskStdoutFunction {
    task_dir: PathBuf,
}

impl TaskStdoutFunction {
    pub fn new(task_dir: PathBuf) -> Self {
        Self { task_dir }
    }
}

impl Function for TaskStdoutFunction {
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
                message: "stdout() takes no arguments".to_string(),
            });
        }
        
        // Return the absolute path to stdout.txt in the task directory
        let stdout_path = self.task_dir.join("stdout.txt");
        Value::file(stdout_path.to_string_lossy().to_string())
    }
}

/// Task-specific stderr function that returns the correct path
pub struct TaskStderrFunction {
    task_dir: PathBuf,
}

impl TaskStderrFunction {
    pub fn new(task_dir: PathBuf) -> Self {
        Self { task_dir }
    }
}

impl Function for TaskStderrFunction {
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
                message: "stderr() takes no arguments".to_string(),
            });
        }
        
        // Return the absolute path to stderr.txt in the task directory
        let stderr_path = self.task_dir.join("stderr.txt");
        Value::file(stderr_path.to_string_lossy().to_string())
    }
}