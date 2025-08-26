//! # miniwdl-rust
//!
//! Rust port of miniwdl - Workflow Description Language (WDL) parser and runtime.
//!
//! This crate provides parsing, static analysis, and runtime capabilities for WDL workflows.

pub mod env;
pub mod error;
pub mod expr;
pub mod parser;
pub mod runtime;
pub mod stdlib;
pub mod tree;
pub mod types;
pub mod value;

pub use env::{Binding, Bindings};
pub use error::{SourcePosition, WdlError};
pub use expr::{BinaryOperator, Expression, ExpressionBase, StringPart, UnaryOperator};
pub use runtime::{Config, RuntimeBuilder, TaskResult, WorkflowResult};
pub use tree::{
    ASTNode, Call, Conditional, Declaration, Document, Scatter, Task, Workflow, WorkflowNode,
};
pub use types::Type;
pub use value::{Value, ValueBase};
