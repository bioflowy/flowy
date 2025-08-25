//! Error types and source position tracking for WDL parsing and evaluation.
//!
//! This module provides comprehensive error handling for WDL document processing,
//! including syntax errors, validation errors, and runtime errors.

use std::collections::HashMap;
use serde::{Deserialize, Serialize};
use thiserror::Error;

/// Source position information for AST nodes and errors.
/// 
/// Contains both the original URI/filename and resolved absolute path,
/// along with one-based line and column positions.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize)]
pub struct SourcePosition {
    /// The filename/URI passed to load or import (may be relative)
    pub uri: String,
    /// The absolute filename/URI after path resolution  
    pub abspath: String,
    /// One-based line number where the construct starts
    pub line: u32,
    /// One-based column number where the construct starts
    pub column: u32,
    /// One-based line number where the construct ends
    pub end_line: u32,
    /// One-based column number where the construct ends  
    pub end_column: u32,
}

impl SourcePosition {
    pub fn new(
        uri: String,
        abspath: String,
        line: u32,
        column: u32,
        end_line: u32,
        end_column: u32,
    ) -> Self {
        Self {
            uri,
            abspath,
            line,
            column,
            end_line,
            end_column,
        }
    }
}

/// Main error type for all WDL-related errors.
#[derive(Error, Debug)]
pub enum WdlError {
    /// Failure to lex/parse a WDL document
    #[error("Syntax error: {message}")]
    Syntax {
        pos: SourcePosition,
        message: String,
        wdl_version: String,
        declared_wdl_version: Option<String>,
    },

    /// Failure to open/retrieve an imported WDL document
    #[error("Import error: {message}")]
    Import {
        pos: SourcePosition,
        message: String,
        #[source]
        cause: Option<Box<dyn std::error::Error + Send + Sync>>,
    },

    /// Base validation error (document parses but fails static checks)
    #[error("Validation error: {message}")]
    Validation {
        pos: SourcePosition,
        message: String,
        source_text: Option<String>,
        declared_wdl_version: Option<String>,
    },

    /// Type-related validation errors
    #[error("Invalid type: {message}")]
    InvalidType {
        pos: SourcePosition,
        message: String,
        source_text: Option<String>,
        declared_wdl_version: Option<String>,
    },

    /// Indeterminate type errors
    #[error("Indeterminate type: {message}")]
    IndeterminateType {
        pos: SourcePosition,
        message: String,
        source_text: Option<String>,
        declared_wdl_version: Option<String>,
    },

    /// No such task/workflow
    #[error("No such task/workflow: {name}")]
    NoSuchTask {
        pos: SourcePosition,
        name: String,
        source_text: Option<String>,
        declared_wdl_version: Option<String>,
    },

    /// No such call in workflow  
    #[error("No such call in this workflow: {name}")]
    NoSuchCall {
        pos: SourcePosition,
        name: String,
        source_text: Option<String>,
        declared_wdl_version: Option<String>,
    },

    /// No such function
    #[error("No such function: {name}")]
    NoSuchFunction {
        pos: SourcePosition,
        name: String,
        source_text: Option<String>,
        declared_wdl_version: Option<String>,
    },

    /// Wrong number of function arguments
    #[error("{function_name} expects {expected} argument(s)")]
    WrongArity {
        pos: SourcePosition,
        function_name: String,
        expected: usize,
        source_text: Option<String>,
        declared_wdl_version: Option<String>,
    },

    /// Not an array when array expected
    #[error("Not an array")]
    NotAnArray {
        pos: SourcePosition,
        source_text: Option<String>,
        declared_wdl_version: Option<String>,
    },

    /// No such member in struct/object
    #[error("No such member '{member}'")]
    NoSuchMember {
        pos: SourcePosition,
        member: String,
        source_text: Option<String>,
        declared_wdl_version: Option<String>,
    },

    /// Static type mismatch
    #[error("Expected {expected} instead of {actual}")]
    StaticTypeMismatch {
        pos: SourcePosition,
        expected: String, // Type representation as string
        actual: String,   // Type representation as string
        message: String,
        source_text: Option<String>,
        declared_wdl_version: Option<String>,
    },

    /// Incompatible operand for operation
    #[error("Incompatible operand: {message}")]
    IncompatibleOperand {
        pos: SourcePosition,
        message: String,
        source_text: Option<String>,
        declared_wdl_version: Option<String>,
    },

    /// Unknown identifier
    #[error("Unknown identifier: {message}")]
    UnknownIdentifier {
        pos: SourcePosition,
        message: String,
        source_text: Option<String>,
        declared_wdl_version: Option<String>,
    },

    /// No such input
    #[error("No such input {name}")]
    NoSuchInput {
        pos: SourcePosition,
        name: String,
        source_text: Option<String>,
        declared_wdl_version: Option<String>,
    },

    /// Uncallable workflow (missing inputs or outputs)
    #[error("Cannot call subworkflow {name} because its own calls have missing required inputs, and/or it lacks an output section")]
    UncallableWorkflow {
        pos: SourcePosition,
        name: String,
        source_text: Option<String>,
        declared_wdl_version: Option<String>,
    },

    /// Multiple definitions of same identifier
    #[error("Multiple definitions: {message}")]
    MultipleDefinitions {
        pos: SourcePosition,
        message: String,
        source_text: Option<String>,
        declared_wdl_version: Option<String>,
    },

    /// Stray input declaration
    #[error("Stray input declaration: {message}")]
    StrayInputDeclaration {
        pos: SourcePosition,
        message: String,
        source_text: Option<String>,
        declared_wdl_version: Option<String>,
    },

    /// Circular dependencies
    #[error("Circular dependencies involving {name}")]
    CircularDependencies {
        pos: SourcePosition,
        name: String,
        source_text: Option<String>,
        declared_wdl_version: Option<String>,
    },

    /// Multiple validation errors collected together
    #[error("Multiple validation errors ({count} errors)")]
    MultipleValidation {
        exceptions: Vec<WdlError>,
        count: usize,
        source_text: Option<String>,
        declared_wdl_version: Option<String>,
    },

    /// Runtime errors during expression/task evaluation
    #[error("Runtime error: {message}")]
    Runtime {
        message: String,
        more_info: HashMap<String, String>,
    },

    /// Error evaluating WDL expression or declaration
    #[error("Evaluation error: {message}")]
    Eval {
        pos: SourcePosition,
        message: String,
    },

    /// Array index out of bounds
    #[error("Array index out of bounds")]
    OutOfBounds { pos: SourcePosition },

    /// Empty array for Array+ input/declaration
    #[error("Empty array for Array+ input/declaration")]
    EmptyArray { pos: SourcePosition },

    /// Null value encountered when non-null expected
    #[error("Null value")]
    NullValue { pos: SourcePosition },

    /// Error reading input value/file
    #[error("Input error: {message}")]
    Input {
        message: String,
        more_info: HashMap<String, String>,
    },
}

impl WdlError {
    /// Get the source position for this error, if available.
    pub fn source_position(&self) -> Option<&SourcePosition> {
        match self {
            WdlError::Syntax { pos, .. } => Some(pos),
            WdlError::Import { pos, .. } => Some(pos),
            WdlError::Validation { pos, .. } => Some(pos),
            WdlError::InvalidType { pos, .. } => Some(pos),
            WdlError::IndeterminateType { pos, .. } => Some(pos),
            WdlError::NoSuchTask { pos, .. } => Some(pos),
            WdlError::NoSuchCall { pos, .. } => Some(pos),
            WdlError::NoSuchFunction { pos, .. } => Some(pos),
            WdlError::WrongArity { pos, .. } => Some(pos),
            WdlError::NotAnArray { pos, .. } => Some(pos),
            WdlError::NoSuchMember { pos, .. } => Some(pos),
            WdlError::StaticTypeMismatch { pos, .. } => Some(pos),
            WdlError::IncompatibleOperand { pos, .. } => Some(pos),
            WdlError::UnknownIdentifier { pos, .. } => Some(pos),
            WdlError::NoSuchInput { pos, .. } => Some(pos),
            WdlError::UncallableWorkflow { pos, .. } => Some(pos),
            WdlError::MultipleDefinitions { pos, .. } => Some(pos),
            WdlError::StrayInputDeclaration { pos, .. } => Some(pos),
            WdlError::CircularDependencies { pos, .. } => Some(pos),
            WdlError::Eval { pos, .. } => Some(pos),
            WdlError::OutOfBounds { pos, .. } => Some(pos),
            WdlError::EmptyArray { pos, .. } => Some(pos),
            WdlError::NullValue { pos, .. } => Some(pos),
            _ => None,
        }
    }

    /// Create a syntax error.
    pub fn syntax_error(
        pos: SourcePosition,
        message: String,
        wdl_version: String,
        declared_wdl_version: Option<String>,
    ) -> Self {
        WdlError::Syntax {
            pos,
            message,
            wdl_version,
            declared_wdl_version,
        }
    }

    /// Create an import error.
    pub fn import_error(pos: SourcePosition, import_uri: String, message: Option<String>) -> Self {
        let msg = match message {
            Some(m) => format!("Failed to import {}, {}", import_uri, m),
            None => format!("Failed to import {}", import_uri),
        };
        WdlError::Import {
            pos,
            message: msg,
            cause: None,
        }
    }

    /// Create a validation error.
    pub fn validation_error(pos: SourcePosition, message: String) -> Self {
        WdlError::Validation {
            pos,
            message,
            source_text: None,
            declared_wdl_version: None,
        }
    }

    /// Create a static type mismatch error with helpful hints.
    pub fn static_type_mismatch(
        pos: SourcePosition,
        expected: String,
        actual: String,
        message: String,
    ) -> Self {
        let enhanced_message = if message.is_empty() {
            let mut msg = format!("Expected {} instead of {}", expected, actual);
            
            // Add helpful hints similar to Python version
            if expected == "Int" && actual == "Float" {
                msg += "; perhaps try floor() or round()";
            } else if actual.replace('?', "") == expected {
                msg += " -- to coerce T? X into T, try select_first([X,defaultValue]) or select_first([X]) (which might fail at runtime); to coerce Array[T?] X into Array[T], try select_all(X)";
            }
            msg
        } else {
            message
        };

        WdlError::StaticTypeMismatch {
            pos,
            expected,
            actual,
            message: enhanced_message,
            source_text: None,
            declared_wdl_version: None,
        }
    }

    /// Combine multiple validation errors into one.
    pub fn multiple_validation_errors(mut exceptions: Vec<WdlError>) -> Self {
        // Sort exceptions by source position
        exceptions.sort_by(|a, b| {
            match (a.source_position(), b.source_position()) {
                (Some(pos_a), Some(pos_b)) => pos_a.cmp(pos_b),
                (Some(_), None) => std::cmp::Ordering::Less,
                (None, Some(_)) => std::cmp::Ordering::Greater,
                (None, None) => std::cmp::Ordering::Equal,
            }
        });

        let count = exceptions.len();
        WdlError::MultipleValidation {
            exceptions,
            count,
            source_text: None,
            declared_wdl_version: None,
        }
    }
}

/// Context for collecting multiple validation errors.
/// 
/// This allows validation to continue after encountering errors,
/// collecting them all before reporting.
#[derive(Default)]
pub struct MultiErrorContext {
    exceptions: Vec<WdlError>,
}

impl MultiErrorContext {
    pub fn new() -> Self {
        Self::default()
    }

    /// Try to execute a closure, capturing any WdlError that occurs.
    /// Returns the result if successful, None if an error was captured.
    pub fn try_with<T, F>(&mut self, f: F) -> Option<T>
    where
        F: FnOnce() -> Result<T, WdlError>,
    {
        match f() {
            Ok(result) => Some(result),
            Err(error) => {
                self.append(error);
                None
            }
        }
    }

    /// Manually append an error to the collection.
    pub fn append(&mut self, error: WdlError) {
        match error {
            WdlError::MultipleValidation { exceptions, .. } => {
                self.exceptions.extend(exceptions);
            }
            _ => self.exceptions.push(error),
        }
    }

    /// Check if any errors have been collected.
    pub fn has_errors(&self) -> bool {
        !self.exceptions.is_empty()
    }

    /// Get the number of collected errors.
    pub fn error_count(&self) -> usize {
        self.exceptions.len()
    }

    /// Raise collected errors, if any.
    /// Returns Ok(()) if no errors were collected.
    pub fn maybe_raise(self) -> Result<(), WdlError> {
        match self.exceptions.len() {
            0 => Ok(()),
            1 => Err(self.exceptions.into_iter().next().unwrap()),
            _ => Err(WdlError::multiple_validation_errors(self.exceptions)),
        }
    }
}

/// Trait for AST nodes that have source position information.
pub trait HasSourcePosition {
    fn source_position(&self) -> &SourcePosition;
    fn set_source_position(&mut self, pos: SourcePosition);
}

/// Base trait for AST nodes with position and parent relationships.
pub trait SourceNode: HasSourcePosition {
    /// Get the parent node, if any.
    fn parent(&self) -> Option<&dyn SourceNode>;
    
    /// Set the parent node.
    fn set_parent(&mut self, parent: Option<&dyn SourceNode>);
    
    /// Get all direct children of this node.
    fn children(&self) -> Vec<&dyn SourceNode> {
        Vec::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_source_position_creation() {
        let pos = SourcePosition::new(
            "test.wdl".to_string(),
            "/abs/path/test.wdl".to_string(),
            1,
            1,
            1,
            10,
        );
        assert_eq!(pos.uri, "test.wdl");
        assert_eq!(pos.abspath, "/abs/path/test.wdl");
        assert_eq!(pos.line, 1);
        assert_eq!(pos.column, 1);
        assert_eq!(pos.end_line, 1);
        assert_eq!(pos.end_column, 10);
    }

    #[test]
    fn test_source_position_ordering() {
        let pos1 = SourcePosition::new("test.wdl".to_string(), "/test.wdl".to_string(), 1, 1, 1, 5);
        let pos2 = SourcePosition::new("test.wdl".to_string(), "/test.wdl".to_string(), 1, 6, 1, 10);
        let pos3 = SourcePosition::new("test.wdl".to_string(), "/test.wdl".to_string(), 2, 1, 2, 5);

        assert!(pos1 < pos2);
        assert!(pos2 < pos3);
        assert!(pos1 < pos3);
    }

    #[test]
    fn test_syntax_error() {
        let pos = SourcePosition::new("test.wdl".to_string(), "/test.wdl".to_string(), 1, 1, 1, 5);
        let error = WdlError::syntax_error(
            pos.clone(),
            "unexpected token".to_string(),
            "1.0".to_string(),
            Some("1.0".to_string()),
        );

        match error {
            WdlError::Syntax {
                pos: error_pos,
                message,
                wdl_version,
                declared_wdl_version,
            } => {
                assert_eq!(error_pos, pos);
                assert_eq!(message, "unexpected token");
                assert_eq!(wdl_version, "1.0");
                assert_eq!(declared_wdl_version, Some("1.0".to_string()));
            }
            _ => panic!("Expected syntax error"),
        }
    }

    #[test]
    fn test_import_error() {
        let pos = SourcePosition::new("test.wdl".to_string(), "/test.wdl".to_string(), 1, 1, 1, 5);
        let error = WdlError::import_error(pos.clone(), "missing.wdl".to_string(), None);

        match error {
            WdlError::Import { pos: error_pos, message, .. } => {
                assert_eq!(error_pos, pos);
                assert_eq!(message, "Failed to import missing.wdl");
            }
            _ => panic!("Expected import error"),
        }
    }

    #[test]
    fn test_static_type_mismatch_with_hints() {
        let pos = SourcePosition::new("test.wdl".to_string(), "/test.wdl".to_string(), 1, 1, 1, 5);
        let error = WdlError::static_type_mismatch(
            pos.clone(),
            "Int".to_string(),
            "Float".to_string(),
            "".to_string(),
        );

        match error {
            WdlError::StaticTypeMismatch { message, .. } => {
                assert!(message.contains("perhaps try floor() or round()"));
            }
            _ => panic!("Expected static type mismatch error"),
        }
    }

    #[test]
    fn test_multi_error_context() {
        let mut ctx = MultiErrorContext::new();
        assert!(!ctx.has_errors());
        assert_eq!(ctx.error_count(), 0);

        let pos = SourcePosition::new("test.wdl".to_string(), "/test.wdl".to_string(), 1, 1, 1, 5);
        ctx.append(WdlError::validation_error(pos, "error 1".to_string()));
        
        let pos2 = SourcePosition::new("test.wdl".to_string(), "/test.wdl".to_string(), 2, 1, 2, 5);
        ctx.append(WdlError::validation_error(pos2, "error 2".to_string()));

        assert!(ctx.has_errors());
        assert_eq!(ctx.error_count(), 2);

        let result = ctx.maybe_raise();
        assert!(result.is_err());
        match result.unwrap_err() {
            WdlError::MultipleValidation { count, .. } => {
                assert_eq!(count, 2);
            }
            _ => panic!("Expected multiple validation errors"),
        }
    }

    #[test]
    fn test_source_position_from_error() {
        let pos = SourcePosition::new("test.wdl".to_string(), "/test.wdl".to_string(), 1, 1, 1, 5);
        let error = WdlError::validation_error(pos.clone(), "test error".to_string());
        
        assert_eq!(error.source_position(), Some(&pos));
    }
}