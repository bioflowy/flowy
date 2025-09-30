use crate::core::api::{ExecuteRequest, ExecuteResponse};
use crate::core::inputs::{
    bindings_from_json_for_document, bindings_from_json_for_task, set_input_base_dir,
};
use crate::env::Bindings;
use crate::parser;
use crate::runtime::{self, Config, RuntimeResult, TaskResult, WorkflowResult};
use crate::tree::{Document, Task};
use crate::value::Value;
use std::path::{Path, PathBuf};
use tempfile::TempDir;
use url::Url;
use uuid::Uuid;

#[derive(thiserror::Error, Debug)]
pub enum ExecuteJobError {
    #[error("{0}")]
    InvalidRequest(String),
    #[error("WDL error: {0}")]
    Wdl(String),
    #[error("Runtime error: {0}")]
    Runtime(String),
    #[error("IO error: {0}")]
    Io(String),
}

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

pub fn execute_request(req: ExecuteRequest) -> Result<ExecuteResponse, ExecuteJobError> {
    let version = detect_wdl_version(&req.wdl);
    let mut document = parser::parse_document(&req.wdl, &version)
        .map_err(|e| ExecuteJobError::Wdl(e.to_string()))?;

    document
        .typecheck()
        .map_err(|e| ExecuteJobError::Wdl(e.to_string()))?;

    let options = req.options.unwrap_or_default();
    let run_id = options.run_id.unwrap_or_else(|| Uuid::new_v4().to_string());

    let config = Config::default();
    let temp_dir = TempDir::new().map_err(|e| ExecuteJobError::Io(e.to_string()))?;
    let work_dir = temp_dir.path();

    let base_dir = options.base_dir.as_ref().and_then(|s| {
        let trimmed = s.trim();
        if trimmed.is_empty() {
            None
        } else {
            Some(PathBuf::from(trimmed))
        }
    });
    let _base_dir_guard = set_input_base_dir(base_dir);

    if let Some(task_name) = options.task {
        let task = find_task(&mut document, &task_name)?;
        let inputs = bindings_from_json_for_task(req.inputs.clone(), task)
            .map_err(|e| ExecuteJobError::Wdl(e.to_string()))?;
        run_task(task.clone(), inputs, config, &run_id, work_dir)
    } else if document.workflow.is_some() {
        let inputs = bindings_from_json_for_document(req.inputs.clone(), &document)
            .map_err(|e| ExecuteJobError::Wdl(e.to_string()))?;
        run_workflow(document, inputs, config, &run_id, work_dir)
    } else if document.tasks.len() == 1 {
        let task = document.tasks[0].clone();
        let inputs = bindings_from_json_for_task(req.inputs.clone(), &task)
            .map_err(|e| ExecuteJobError::Wdl(e.to_string()))?;
        run_task(task, inputs, config, &run_id, work_dir)
    } else {
        Err(ExecuteJobError::InvalidRequest(
            "Document lacks workflow; specify task name".to_string(),
        ))
    }
}

fn run_workflow(
    document: Document,
    inputs: Bindings<Value>,
    config: Config,
    run_id: &str,
    work_dir: &Path,
) -> Result<ExecuteResponse, ExecuteJobError> {
    let workflow_namespace = document
        .workflow
        .as_ref()
        .map(|workflow| workflow.name.clone());

    let result = execute_workflow_document(document, inputs, config, run_id, work_dir)
        .map_err(|e| ExecuteJobError::Runtime(e.to_string()))?;

    let outputs = crate::core::outputs::bindings_to_json_with_namespace(
        &result.outputs,
        workflow_namespace.as_deref(),
    )
    .map_err(|e| ExecuteJobError::Wdl(e.to_string()))?;

    Ok(ExecuteResponse {
        status: "ok".to_string(),
        outputs,
        stdout: None,
        stderr: None,
        duration_ms: result.duration.as_millis(),
    })
}

fn run_task(
    task: Task,
    inputs: Bindings<Value>,
    config: Config,
    run_id: &str,
    work_dir: &Path,
) -> Result<ExecuteResponse, ExecuteJobError> {
    let namespace = task.name.clone();

    let result = execute_task(task, inputs, config, run_id, work_dir)
        .map_err(|e| ExecuteJobError::Runtime(e.to_string()))?;

    let outputs = crate::core::outputs::bindings_to_json_with_namespace(
        &result.outputs,
        Some(namespace.as_str()),
    )
    .map_err(|e| ExecuteJobError::Wdl(e.to_string()))?;

    let stdout = read_file_from_url(&result.stdout)?;
    let stderr = read_file_from_url(&result.stderr)?;

    Ok(ExecuteResponse {
        status: "ok".to_string(),
        outputs,
        stdout: Some(stdout),
        stderr: Some(stderr),
        duration_ms: result.duration.as_millis(),
    })
}

fn read_file_from_url(url: &Url) -> Result<String, ExecuteJobError> {
    let path = url
        .to_file_path()
        .map_err(|_| ExecuteJobError::Runtime(format!("Invalid file URL: {}", url)))?;
    std::fs::read_to_string(&path)
        .map_err(|e| ExecuteJobError::Io(format!("Failed to read {}: {}", path.display(), e)))
}

fn find_task<'a>(document: &'a mut Document, name: &str) -> Result<&'a mut Task, ExecuteJobError> {
    document
        .tasks
        .iter_mut()
        .find(|task| task.name == name)
        .ok_or_else(|| ExecuteJobError::InvalidRequest(format!("Task '{}' not found", name)))
}

fn detect_wdl_version(source: &str) -> String {
    for line in source.lines() {
        let trimmed = line.trim();
        if let Some(rest) = trimmed.strip_prefix("version") {
            return rest.trim().to_string();
        }
    }
    "1.0".to_string()
}
