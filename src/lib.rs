//! # miniwdl-rust
//!
//! Rust port of miniwdl - Workflow Description Language (WDL) parser and runtime.
//!
//! This crate provides parsing, static analysis, and runtime capabilities for WDL workflows.

// Allow large error enum variants for now - this is a known tradeoff for comprehensive error handling
#![allow(clippy::result_large_err)]
// Temporarily allow these clippy warnings while focusing on functionality
#![allow(clippy::approx_constant)]
#![allow(clippy::collapsible_match)]
#![allow(clippy::single_match)]
#![allow(clippy::needless_borrows_for_generic_args)]
#![allow(clippy::useless_vec)]
#![allow(unused_imports)]
#![allow(clippy::while_let_loop)]
#![allow(clippy::type_complexity)]
#![allow(clippy::should_implement_trait)]
#![allow(clippy::too_many_arguments)]
#![allow(clippy::let_and_return)]
#![allow(clippy::large_enum_variant)]
#![allow(clippy::map_clone)]
#![allow(clippy::unnecessary_map_or)]
#![allow(clippy::derivable_impls)]
#![allow(clippy::ptr_arg)]
#![allow(clippy::only_used_in_recursion)]
#![allow(unused_variables)]
#![allow(clippy::missing_transmute_annotations)]

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
