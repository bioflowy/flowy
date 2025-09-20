//! Task output-specific standard library utilities
//!
//! This module provides utilities for creating task-aware standard libraries
//! using the PathMapper system.

use crate::stdlib::{StdLib, TaskPathMapper};
use std::path::PathBuf;

/// Create a task output-specific standard library with PathMapper
///
/// This creates a StdLib instance with TaskPathMapper for proper file path handling
/// in task execution contexts. This replaces the old approach of overriding individual functions.
pub fn create_task_output_stdlib(wdl_version: &str, task_dir: PathBuf) -> StdLib {
    let write_dir = task_dir
        .join("work")
        .join("write_")
        .to_string_lossy()
        .to_string();
    StdLib::with_path_mapper(
        wdl_version,
        Box::new(TaskPathMapper::new(task_dir)),
        true, // is_task_context = true, enables stdout/stderr functions
        write_dir,
    )
}
