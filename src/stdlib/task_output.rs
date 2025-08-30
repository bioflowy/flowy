//! Task output-specific standard library implementation
//!
//! This module provides a specialized version of the standard library
//! that is context-aware for task execution, similar to miniwdl's OutputStdLib.

use crate::error::WdlError;
use crate::stdlib::{Function, StdLib};
use crate::types::Type;
use crate::value::Value;
use std::collections::HashMap;
use std::path::PathBuf;

/// Create a task output-specific standard library by overriding specific functions
///
/// This creates a regular StdLib but replaces stdout(), stderr(), and read_string() functions
/// with task-specific versions that know about the task execution directory.
pub fn create_task_output_stdlib(wdl_version: &str, task_dir: PathBuf) -> StdLib {
    let mut stdlib = StdLib::new(wdl_version);

    // Override stdout() function with task-specific version
    stdlib.add_function(Box::new(TaskStdoutFunction::new(task_dir.clone())));

    // Override stderr() function with task-specific version
    stdlib.add_function(Box::new(TaskStderrFunction::new(task_dir.clone())));

    // Override read_string() function with task-specific version
    stdlib.add_function(Box::new(TaskReadStringFunction::new(task_dir)));

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

        Ok(Type::string(false))
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        if !args.is_empty() {
            return Err(WdlError::RuntimeError {
                message: "stdout() takes no arguments".to_string(),
            });
        }

        // Return the path to stdout.txt as a File value
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

        Ok(Type::string(false))
    }

    fn eval(&self, args: &[Value]) -> Result<Value, WdlError> {
        if !args.is_empty() {
            return Err(WdlError::RuntimeError {
                message: "stderr() takes no arguments".to_string(),
            });
        }

        // Return the path to stderr.txt as a File value
        let stderr_path = self.task_dir.join("stderr.txt");
        Value::file(stderr_path.to_string_lossy().to_string())
    }
}

/// Task-specific read_string function that knows about task execution context
pub struct TaskReadStringFunction {
    task_dir: PathBuf,
}

impl TaskReadStringFunction {
    pub fn new(task_dir: PathBuf) -> Self {
        Self { task_dir }
    }
}

impl Function for TaskReadStringFunction {
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

        // Handle relative paths relative to task directory
        let file_path = if std::path::Path::new(&filename).is_absolute() {
            std::path::PathBuf::from(filename)
        } else {
            self.task_dir.join(&filename)
        };

        // Read the entire file as a string
        match std::fs::read_to_string(&file_path) {
            Ok(content) => Ok(Value::string(content)),
            Err(e) => Err(WdlError::RuntimeError {
                message: format!("Failed to read file {}: {}", file_path.display(), e),
            }),
        }
    }
}
