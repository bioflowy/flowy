use crate::env::Bindings;
use crate::runtime::{self, Config, RuntimeResult, TaskResult, WorkflowResult};
use crate::tree::{Document, Task};
use crate::value::Value;
use std::path::Path;

/// Execute a workflow document using the provided inputs and configuration.
pub fn execute_workflow_document(
    document: Document,
    inputs: Bindings<Value>,
    config: Config,
    run_id: &str,
    work_dir: &Path,
) -> RuntimeResult<WorkflowResult> {
    runtime::run_document(document, inputs, config, run_id, work_dir)
}

/// Execute a specific task using the provided inputs and configuration.
pub fn execute_task(
    task: Task,
    inputs: Bindings<Value>,
    config: Config,
    run_id: &str,
    work_dir: &Path,
) -> RuntimeResult<TaskResult> {
    runtime::run_task(task, inputs, config, run_id, work_dir)
}
