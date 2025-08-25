//! # miniwdl-rust
//!
//! Rust port of miniwdl - Workflow Description Language (WDL) parser and runtime.
//! 
//! This crate provides parsing, static analysis, and runtime capabilities for WDL workflows.

pub mod error;
pub mod env;
pub mod types;
pub mod value;
pub mod expr;
pub mod tree;
pub mod parser;

pub use error::{SourcePosition, WdlError};
pub use env::{Binding, Bindings};
pub use types::Type;
pub use value::{Value, ValueBase};
pub use expr::{Expression, StringPart, BinaryOperator, UnaryOperator, ExpressionBase};
pub use tree::{Document, Workflow, Task, Declaration, Call, Scatter, Conditional, ASTNode, WorkflowNode};