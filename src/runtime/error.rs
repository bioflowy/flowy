//! Runtime-specific error types
//!
//! This module defines errors that occur during workflow and task execution,
//! separate from the compile-time errors in the main error module.

use crate::error::{SourcePosition, WdlError};
use std::fmt;
use std::io;
use std::process::ExitStatus;
use std::time::Duration;

/// Runtime execution errors
#[derive(Debug)]
pub enum RuntimeError {
    /// Task execution failed
    RunFailed {
        /// Error message
        message: String,
        /// Underlying cause
        cause: Box<dyn std::error::Error + Send + Sync>,
        /// Source position if available
        pos: Option<SourcePosition>,
    },

    /// Command execution failed
    CommandFailed {
        /// Command that failed
        command: String,
        /// Exit status
        exit_status: Option<ExitStatus>,
        /// Standard output
        stdout: String,
        /// Standard error
        stderr: String,
        /// Working directory
        working_dir: String,
    },

    /// Task was terminated by signal
    Terminated {
        /// Signal that terminated the task
        signal: i32,
        /// Task command
        command: String,
    },

    /// Task was interrupted by user
    Interrupted {
        /// Reason for interruption
        reason: String,
    },

    /// Task timeout
    TaskTimeout {
        /// Timeout duration
        timeout: Duration,
        /// Task name
        task_name: String,
        /// Command that timed out
        command: String,
    },

    /// Output validation failed
    OutputError {
        /// Error message
        message: String,
        /// Expected type
        expected_type: String,
        /// Actual output description
        actual: String,
        /// Source position
        pos: Option<SourcePosition>,
    },

    /// File system operation failed
    FileSystemError {
        /// Error message
        message: String,
        /// File path if relevant
        path: Option<String>,
        /// Underlying IO error
        io_error: io::Error,
    },

    /// Download failed (placeholder for future implementation)
    DownloadFailed {
        /// URL that failed to download
        url: String,
        /// Error message
        message: String,
        /// HTTP status code if applicable
        status_code: Option<u16>,
    },

    /// Container operation failed (placeholder for future implementation)
    ContainerError {
        /// Error message
        message: String,
        /// Container ID if available
        container_id: Option<String>,
        /// Underlying error
        cause: Option<Box<dyn std::error::Error + Send + Sync>>,
    },

    /// Cache operation failed (placeholder for future implementation)
    CacheError {
        /// Error message
        message: String,
        /// Cache key if relevant
        cache_key: Option<String>,
        /// Underlying error
        cause: Option<Box<dyn std::error::Error + Send + Sync>>,
    },

    /// Workflow validation failed at runtime
    WorkflowValidationError {
        /// Error message
        message: String,
        /// Source position
        pos: SourcePosition,
    },

    /// Resource limit exceeded
    ResourceLimitExceeded {
        /// Resource that was exceeded
        resource: String,
        /// Limit that was exceeded
        limit: String,
        /// Actual usage
        usage: String,
    },

    /// Configuration error
    ConfigurationError {
        /// Error message
        message: String,
        /// Configuration key if relevant
        key: Option<String>,
    },
}

impl fmt::Display for RuntimeError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            RuntimeError::RunFailed { message, pos, .. } => {
                if let Some(pos) = pos {
                    write!(f, "{}: {}", pos, message)
                } else {
                    write!(f, "Runtime error: {}", message)
                }
            }
            RuntimeError::CommandFailed {
                command,
                exit_status,
                stderr,
                working_dir,
                ..
            } => {
                write!(f, "Command failed: {}", command)?;
                if let Some(status) = exit_status {
                    write!(f, " (exit status: {})", status)?;
                }
                write!(f, " in directory: {}", working_dir)?;
                if !stderr.is_empty() {
                    write!(f, "\nStderr: {}", stderr)?;
                }
                Ok(())
            }
            RuntimeError::Terminated { signal, command } => {
                write!(f, "Command terminated by signal {}: {}", signal, command)
            }
            RuntimeError::Interrupted { reason } => {
                write!(f, "Execution interrupted: {}", reason)
            }
            RuntimeError::TaskTimeout {
                timeout,
                task_name,
                command,
            } => {
                write!(
                    f,
                    "Task '{}' timed out after {:?}: {}",
                    task_name, timeout, command
                )
            }
            RuntimeError::OutputError {
                message,
                expected_type,
                actual,
                pos,
            } => {
                let pos_str = pos.as_ref().map(|p| format!("{}: ", p)).unwrap_or_default();
                write!(
                    f,
                    "{}Output error: {} (expected {}, got {})",
                    pos_str, message, expected_type, actual
                )
            }
            RuntimeError::FileSystemError {
                message,
                path,
                io_error,
            } => {
                if let Some(path) = path {
                    write!(
                        f,
                        "File system error at {}: {} ({})",
                        path, message, io_error
                    )
                } else {
                    write!(f, "File system error: {} ({})", message, io_error)
                }
            }
            RuntimeError::DownloadFailed {
                url,
                message,
                status_code,
            } => {
                write!(f, "Download failed for {}: {}", url, message)?;
                if let Some(code) = status_code {
                    write!(f, " (HTTP {})", code)?;
                }
                Ok(())
            }
            RuntimeError::ContainerError {
                message,
                container_id,
                ..
            } => {
                if let Some(id) = container_id {
                    write!(f, "Container error ({}): {}", id, message)
                } else {
                    write!(f, "Container error: {}", message)
                }
            }
            RuntimeError::CacheError {
                message, cache_key, ..
            } => {
                if let Some(key) = cache_key {
                    write!(f, "Cache error ({}): {}", key, message)
                } else {
                    write!(f, "Cache error: {}", message)
                }
            }
            RuntimeError::WorkflowValidationError { message, pos } => {
                write!(f, "{}: Workflow validation error: {}", pos, message)
            }
            RuntimeError::ResourceLimitExceeded {
                resource,
                limit,
                usage,
            } => {
                write!(
                    f,
                    "Resource limit exceeded: {} (limit: {}, usage: {})",
                    resource, limit, usage
                )
            }
            RuntimeError::ConfigurationError { message, key } => {
                if let Some(key) = key {
                    write!(f, "Configuration error ({}): {}", key, message)
                } else {
                    write!(f, "Configuration error: {}", message)
                }
            }
        }
    }
}

impl std::error::Error for RuntimeError {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        match self {
            RuntimeError::RunFailed { cause, .. } => {
                Some(cause.as_ref() as &(dyn std::error::Error + 'static))
            }
            RuntimeError::FileSystemError { io_error, .. } => {
                Some(io_error as &(dyn std::error::Error + 'static))
            }
            RuntimeError::ContainerError { cause, .. } => cause
                .as_ref()
                .map(|e| e.as_ref() as &(dyn std::error::Error + 'static)),
            RuntimeError::CacheError { cause, .. } => cause
                .as_ref()
                .map(|e| e.as_ref() as &(dyn std::error::Error + 'static)),
            _ => None,
        }
    }
}

impl RuntimeError {
    /// Create a run failed error
    pub fn run_failed<E: std::error::Error + Send + Sync + 'static>(
        message: String,
        cause: E,
        pos: Option<SourcePosition>,
    ) -> Self {
        Self::RunFailed {
            message,
            cause: Box::new(cause),
            pos,
        }
    }

    /// Create a command failed error
    pub fn command_failed(
        command: String,
        exit_status: Option<ExitStatus>,
        stdout: String,
        stderr: String,
        working_dir: String,
    ) -> Self {
        Self::CommandFailed {
            command,
            exit_status,
            stdout,
            stderr,
            working_dir,
        }
    }

    /// Create a task timeout error
    pub fn task_timeout(timeout: Duration, task_name: String, command: String) -> Self {
        Self::TaskTimeout {
            timeout,
            task_name,
            command,
        }
    }

    /// Create an output error
    pub fn output_error(
        message: String,
        expected_type: String,
        actual: String,
        pos: Option<SourcePosition>,
    ) -> Self {
        Self::OutputError {
            message,
            expected_type,
            actual,
            pos,
        }
    }

    /// Create a file system error
    pub fn filesystem_error(message: String, path: Option<String>, io_error: io::Error) -> Self {
        Self::FileSystemError {
            message,
            path,
            io_error,
        }
    }

    /// Create a configuration error
    pub fn configuration_error(message: String, key: Option<String>) -> Self {
        Self::ConfigurationError { message, key }
    }

    /// Create a workflow validation error
    pub fn workflow_validation_error(message: String, pos: SourcePosition) -> Self {
        Self::WorkflowValidationError { message, pos }
    }

    /// Convert to WdlError for compatibility
    pub fn into_wdl_error(self) -> WdlError {
        match self {
            RuntimeError::WorkflowValidationError { message, pos } => {
                WdlError::validation_error(pos, message)
            }
            RuntimeError::OutputError { message, pos, .. } => {
                let pos = pos.unwrap_or_else(|| {
                    SourcePosition::new("".to_string(), "".to_string(), 0, 0, 0, 0)
                });
                WdlError::validation_error(pos, message)
            }
            other => WdlError::RuntimeError {
                message: other.to_string(),
            },
        }
    }

    /// Get source position if available
    pub fn source_position(&self) -> Option<&SourcePosition> {
        match self {
            RuntimeError::RunFailed { pos, .. } => pos.as_ref(),
            RuntimeError::OutputError { pos, .. } => pos.as_ref(),
            RuntimeError::WorkflowValidationError { pos, .. } => Some(pos),
            _ => None,
        }
    }
}

/// Result type for runtime operations
pub type RuntimeResult<T> = Result<T, RuntimeError>;

/// Extension trait for converting IO errors to runtime errors
pub trait IntoRuntimeError<T> {
    fn runtime_context(self, message: &str) -> RuntimeResult<T>;
    fn runtime_context_with_path(self, message: &str, path: &str) -> RuntimeResult<T>;
}

impl<T> IntoRuntimeError<T> for Result<T, io::Error> {
    fn runtime_context(self, message: &str) -> RuntimeResult<T> {
        self.map_err(|e| RuntimeError::filesystem_error(message.to_string(), None, e))
    }

    fn runtime_context_with_path(self, message: &str, path: &str) -> RuntimeResult<T> {
        self.map_err(|e| {
            RuntimeError::filesystem_error(message.to_string(), Some(path.to_string()), e)
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::{Error, ErrorKind};
    use std::os::unix::process::ExitStatusExt;

    #[test]
    fn test_runtime_error_display() {
        let error = RuntimeError::command_failed(
            "echo hello".to_string(),
            Some(ExitStatus::from_raw(256)), // exit code 1
            "hello".to_string(),
            "error message".to_string(),
            "/tmp".to_string(),
        );

        let display = format!("{}", error);
        assert!(display.contains("Command failed: echo hello"));
        assert!(display.contains("in directory: /tmp"));
        assert!(display.contains("Stderr: error message"));
    }

    #[test]
    fn test_task_timeout_error() {
        let error = RuntimeError::task_timeout(
            Duration::from_secs(30),
            "test_task".to_string(),
            "sleep 60".to_string(),
        );

        let display = format!("{}", error);
        assert!(display.contains("Task 'test_task' timed out"));
        assert!(display.contains("30s"));
        assert!(display.contains("sleep 60"));
    }

    #[test]
    fn test_output_error() {
        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 10);
        let error = RuntimeError::output_error(
            "Invalid output".to_string(),
            "String".to_string(),
            "Int".to_string(),
            Some(pos),
        );

        let display = format!("{}", error);
        assert!(display.contains("Output error: Invalid output"));
        assert!(display.contains("expected String, got Int"));
    }

    #[test]
    fn test_filesystem_error() {
        let io_error = Error::new(ErrorKind::NotFound, "File not found");
        let error = RuntimeError::filesystem_error(
            "Cannot read file".to_string(),
            Some("/path/to/file".to_string()),
            io_error,
        );

        let display = format!("{}", error);
        assert!(display.contains("File system error at /path/to/file"));
        assert!(display.contains("Cannot read file"));
        assert!(display.contains("File not found"));
    }

    #[test]
    fn test_into_runtime_error_extension() {
        let io_error = Error::new(ErrorKind::PermissionDenied, "Permission denied");
        let result: Result<(), _> = Err(io_error);

        let runtime_result = result.runtime_context_with_path("Failed to write", "/tmp/test");
        assert!(runtime_result.is_err());

        if let Err(RuntimeError::FileSystemError { message, path, .. }) = runtime_result {
            assert_eq!(message, "Failed to write");
            assert_eq!(path.as_deref(), Some("/tmp/test"));
        } else {
            panic!("Expected FileSystemError");
        }
    }

    #[test]
    fn test_error_conversion() {
        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 10);
        let runtime_error =
            RuntimeError::workflow_validation_error("Invalid workflow".to_string(), pos.clone());

        let wdl_error = runtime_error.into_wdl_error();
        match wdl_error {
            WdlError::Validation {
                message,
                pos: error_pos,
                ..
            } => {
                assert_eq!(message, "Invalid workflow");
                assert_eq!(error_pos, pos);
            }
            _ => panic!("Expected validation error"),
        }
    }
}
