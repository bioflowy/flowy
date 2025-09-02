//! Runtime-specific error types
//!
//! This module now provides type aliases to the unified WdlError type
//! for backward compatibility.

// Type aliases for backward compatibility - RuntimeError is now unified with WdlError
pub type RuntimeError = crate::error::WdlError;
pub type RuntimeResult<T> = Result<T, crate::error::WdlError>;

// Re-export useful items for convenience
pub use crate::error::{SourcePosition, WdlError};

/// Extension trait for converting IO errors to WdlErrors
pub trait IntoRuntimeError<T> {
    fn runtime_context(self, message: &str) -> RuntimeResult<T>;
    fn runtime_context_with_path(self, message: &str, path: &str) -> RuntimeResult<T>;
}

impl<T> IntoRuntimeError<T> for Result<T, std::io::Error> {
    fn runtime_context(self, message: &str) -> RuntimeResult<T> {
        self.map_err(|e| WdlError::file_system_error(message.to_string(), None, e))
    }

    fn runtime_context_with_path(self, message: &str, path: &str) -> RuntimeResult<T> {
        self.map_err(|e| {
            WdlError::file_system_error(message.to_string(), Some(path.to_string()), e)
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::{Error, ErrorKind};

    #[test]
    fn test_into_runtime_error_extension() {
        let io_error = Error::new(ErrorKind::PermissionDenied, "Permission denied");
        let result: Result<(), _> = Err(io_error);

        let runtime_result = result.runtime_context_with_path("Failed to write", "/tmp/test");
        assert!(runtime_result.is_err());

        if let Err(WdlError::FileSystemError { message, path, .. }) = runtime_result {
            assert_eq!(message, "Failed to write");
            assert_eq!(path.as_deref(), Some("/tmp/test"));
        } else {
            panic!("Expected FileSystemError");
        }
    }
}
