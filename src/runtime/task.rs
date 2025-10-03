//! Task execution engine
//!
//! This module provides the core task execution functionality for WDL workflows,
//! handling task setup, execution, and result collection.

// Note: error types available if needed
use crate::env::Bindings;
use crate::runtime::config::Config;
use crate::runtime::error::{RuntimeError, RuntimeResult};
use crate::runtime::fs_utils::{
    create_dir_all, read_file_to_string, write_file_atomic, WorkflowDirectory,
};
use crate::runtime::task_context::TaskResult;
use crate::runtime::task_runner::{
    deserialize_bindings, serialize_bindings, RunnerConfig, TaskRunnerRequest, TaskRunnerResponse,
    TASK_RUNNER_PROTOCOL_VERSION,
};
use crate::tree::Task;
use crate::value::Value;
use serde_json;
use std::env;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, ExitStatus, Stdio};
use std::time::{Duration, Instant};
use url::Url;

/// Task execution engine
pub struct TaskEngine {
    /// Configuration for execution
    config: Config,
    /// Base workflow directory
    workflow_dir: WorkflowDirectory,
}

/// Task execution options
#[derive(Debug, Clone, Default)]
pub struct TaskExecutionOptions {
    /// Override default task timeout
    pub timeout_override: Option<std::time::Duration>,
    /// Additional environment variables
    pub env_vars: std::collections::HashMap<String, String>,
    /// Copy input files instead of symlinking
    pub copy_inputs: Option<bool>,
    /// Enable verbose logging
    pub verbose: bool,
}

impl TaskEngine {
    /// Create a new task engine
    pub fn new(config: Config, workflow_dir: WorkflowDirectory) -> Self {
        Self {
            config,
            workflow_dir,
        }
    }

    /// Execute a single task
    pub fn execute_task(
        &self,
        task: Task,
        inputs: Bindings<Value>,
        run_id: &str,
        options: TaskExecutionOptions,
    ) -> RuntimeResult<TaskResult> {
        // Prepare configuration for this execution
        let mut config = self.config.clone();

        if let Some(timeout) = options.timeout_override {
            config.task_timeout = timeout;
        }
        if let Some(copy_inputs) = options.copy_inputs {
            config.copy_input_files = copy_inputs;
        }
        for (key, value) in options.env_vars {
            config.env_vars.insert(key, value);
        }

        let task_name = task.name.clone();
        if options.verbose {
            println!("Executing task: {}", task_name);
        }

        // Ensure task directory exists
        let task_dir = self.workflow_dir.work.join(&task_name);
        create_dir_all(&task_dir)?;
        create_dir_all(task_dir.join("work"))?;

        // Serialize request for the runner
        let serialized_inputs = serialize_bindings(&inputs);
        drop(inputs);
        let request = TaskRunnerRequest {
            version: TASK_RUNNER_PROTOCOL_VERSION,
            run_id: run_id.to_string(),
            workflow_dir: self.workflow_dir.clone(),
            task,
            inputs: serialized_inputs,
            config: RunnerConfig::from(&config),
        };

        let mut request_json =
            serde_json::to_vec_pretty(&request).map_err(|e| RuntimeError::RuntimeError {
                message: format!("Failed to serialize task runner request: {e}"),
            })?;
        request_json.push(b'\n');

        let request_path = task_dir.join("task_request.json");
        write_file_atomic(&request_path, &request_json)?;

        // Spawn the subprocess task runner
        let runner_executable = resolve_task_runner_executable();
        let runner_label = runner_executable.display().to_string();
        let mut command = Command::new(&runner_executable);
        command
            .arg(&request_path)
            .current_dir(&task_dir)
            .stdin(Stdio::null())
            .stdout(Stdio::null())
            .stderr(Stdio::null());

        let child = command.spawn().map_err(|e| RuntimeError::RuntimeError {
            message: format!("Failed to spawn task runner '{}': {}", runner_label, e),
        })?;

        let timeout = config.task_timeout;
        let runner_status = wait_for_runner(child, timeout, &task_name, &runner_label)?;
        if options.verbose {
            if let Some(code) = runner_status.code() {
                println!("Task runner exited with status code {code}");
            }
        }

        // Load runner response
        let response_path = task_dir.join("task_response.json");
        let response_text = read_file_to_string(&response_path)?;
        let response: TaskRunnerResponse =
            serde_json::from_str(&response_text).map_err(|e| RuntimeError::RuntimeError {
                message: format!("Failed to deserialize task runner response: {e}"),
            })?;

        if response.version != TASK_RUNNER_PROTOCOL_VERSION {
            return Err(RuntimeError::RuntimeError {
                message: format!(
                    "Task runner protocol mismatch: expected {}, got {}",
                    TASK_RUNNER_PROTOCOL_VERSION, response.version
                ),
            });
        }

        if !response.success {
            let message = response
                .error
                .unwrap_or_else(|| "Task runner reported failure without message".to_string());
            return Err(RuntimeError::RunFailed {
                message,
                cause: None,
                pos: None,
            });
        }

        let outputs = response
            .outputs
            .map(deserialize_bindings)
            .unwrap_or_else(Bindings::new);

        let stdout_str = response.stdout.ok_or_else(|| RuntimeError::RuntimeError {
            message: "Task runner response missing stdout location".to_string(),
        })?;
        let stderr_str = response.stderr.ok_or_else(|| RuntimeError::RuntimeError {
            message: "Task runner response missing stderr location".to_string(),
        })?;

        let stdout = Url::parse(&stdout_str).map_err(|e| RuntimeError::RuntimeError {
            message: format!("Invalid stdout URL from task runner: {e}"),
        })?;
        let stderr = Url::parse(&stderr_str).map_err(|e| RuntimeError::RuntimeError {
            message: format!("Invalid stderr URL from task runner: {e}"),
        })?;

        let duration = response
            .duration_ms
            .map(Duration::from_millis)
            .unwrap_or_else(|| Duration::from_millis(0));

        let exit_status =
            build_exit_status(response.exit_success, response.exit_code, response.signal);

        let work_dir = response.work_dir.unwrap_or_else(|| task_dir.join("work"));

        if options.verbose {
            println!(
                "Task {} completed in {:?} with exit code {:?}",
                task_name, duration, response.exit_code
            );
        }

        Ok(TaskResult {
            outputs,
            exit_status,
            stdout,
            stderr,
            duration,
            work_dir,
        })
    }

    /// Execute a task with default options
    pub fn execute_task_default(
        &self,
        task: Task,
        inputs: Bindings<Value>,
        run_id: &str,
    ) -> RuntimeResult<TaskResult> {
        self.execute_task(task, inputs, run_id, TaskExecutionOptions::default())
    }

    /// Validate a task before execution
    pub fn validate_task(&self, task: &Task, inputs: &Bindings<Value>) -> RuntimeResult<()> {
        // Check that all required inputs are provided
        if !task.inputs.is_empty() {
            for input_decl in &task.inputs {
                if input_decl.expr.is_none() {
                    // Required input
                    if !inputs.has_binding(&input_decl.name) {
                        return Err(RuntimeError::WorkflowValidationError {
                            message: format!(
                                "Missing required input for task {}: {}",
                                task.name, input_decl.name
                            ),
                            pos: input_decl.pos.clone(),
                        });
                    }
                }
            }
        }

        // Task command is required (it's an Expression, not Option<Expression>)
        // No validation needed here as it's guaranteed by the type system

        // Validate outputs have expressions
        for output_decl in &task.outputs {
            if output_decl.expr.is_none() {
                return Err(RuntimeError::WorkflowValidationError {
                    message: format!(
                        "Output declaration missing expression in task {}: {}",
                        task.name, output_decl.name
                    ),
                    pos: output_decl.pos.clone(),
                });
            }
        }

        Ok(())
    }

    /// Get task input requirements
    pub fn get_task_inputs(&self, task: &Task) -> Vec<(String, crate::Type, bool)> {
        task.inputs
            .iter()
            .map(|decl| {
                let required = decl.expr.is_none();
                (decl.name.clone(), decl.decl_type.clone(), required)
            })
            .collect()
    }

    /// Get task output types
    pub fn get_task_outputs(&self, task: &Task) -> Vec<(String, crate::Type)> {
        task.outputs
            .iter()
            .map(|decl| (decl.name.clone(), decl.decl_type.clone()))
            .collect()
    }

    /// Create task-specific working directory
    pub fn create_task_directory(&self, task_name: &str, run_id: &str) -> RuntimeResult<PathBuf> {
        let task_dir = self
            .workflow_dir
            .work
            .join(format!("{}_{}", task_name, run_id));
        crate::runtime::fs_utils::create_dir_all(&task_dir)?;
        Ok(task_dir)
    }

    /// Clean up task directory
    pub fn cleanup_task_directory(&self, task_dir: &PathBuf) -> RuntimeResult<()> {
        if task_dir.exists() {
            crate::runtime::fs_utils::remove_dir_all(task_dir)?;
        }
        Ok(())
    }
}

/// Task execution statistics
#[derive(Debug, Clone)]
pub struct TaskExecutionStats {
    /// Task name
    pub task_name: String,
    /// Execution start time
    pub start_time: Instant,
    /// Execution duration
    pub duration: std::time::Duration,
    /// Exit status code
    pub exit_code: Option<i32>,
    /// Memory usage (if available)
    pub memory_usage: Option<u64>,
    /// CPU time (if available)
    pub cpu_time: Option<std::time::Duration>,
    /// Working directory size
    pub work_dir_size: Option<u64>,
}

fn resolve_task_runner_executable() -> PathBuf {
    if let Ok(explicit) = env::var("FLOWY_TASK_RUNNER").or_else(|_| env::var("MINIWDL_TASK_RUNNER"))
    {
        return PathBuf::from(explicit);
    }

    if let Ok(current_exe) = env::current_exe() {
        if let Some(dir) = current_exe.parent() {
            if let Some(found) = find_runner_in_dir(dir) {
                return found;
            }
            if let Some(parent) = dir.parent() {
                if let Some(found) = find_runner_in_dir(parent) {
                    return found;
                }
            }
        }
    }

    PathBuf::from(default_runner_binary_name())
}

fn find_runner_in_dir(dir: &Path) -> Option<PathBuf> {
    let candidate = dir.join(default_runner_binary_name());
    if candidate.exists() {
        Some(candidate)
    } else {
        None
    }
}

fn default_runner_binary_name() -> &'static str {
    if cfg!(windows) {
        "flowy-task-runner.exe"
    } else {
        "flowy-task-runner"
    }
}

fn wait_for_runner(
    mut child: Child,
    timeout: Duration,
    task_name: &str,
    command: &str,
) -> RuntimeResult<ExitStatus> {
    use std::sync::mpsc;
    use std::thread;

    let (tx, rx) = mpsc::channel();

    thread::spawn(move || {
        let result = child.wait();
        tx.send(result).ok();
    });

    match rx.recv_timeout(timeout) {
        Ok(Ok(status)) => Ok(status),
        Ok(Err(e)) => Err(RuntimeError::RuntimeError {
            message: format!("Failed to wait for task runner: {e}"),
        }),
        Err(_) => Err(RuntimeError::TaskTimeout {
            timeout,
            task_name: task_name.to_string(),
            command: command.to_string(),
        }),
    }
}

fn build_exit_status(
    exit_success: bool,
    exit_code: Option<i32>,
    signal: Option<i32>,
) -> ExitStatus {
    #[cfg(unix)]
    {
        use std::os::unix::process::ExitStatusExt;

        if let Some(code) = exit_code {
            return ExitStatus::from_raw((code & 0xff) << 8);
        }

        if let Some(sig) = signal {
            return ExitStatus::from_raw(sig & 0x7f);
        }

        if exit_success {
            ExitStatus::from_raw(0)
        } else {
            ExitStatus::from_raw(1 << 8)
        }
    }

    #[cfg(not(unix))]
    {
        use std::os::windows::process::ExitStatusExt;

        let code = exit_code.unwrap_or(if exit_success { 0 } else { 1 });
        ExitStatus::from_raw(code as u32)
    }
}

#[cfg(test)]
mod exit_status_tests {
    use super::*;

    #[test]
    fn test_build_exit_status_success() {
        let status = build_exit_status(true, Some(0), None);
        assert!(status.success());
        assert_eq!(status.code(), Some(0));
    }

    #[test]
    fn test_build_exit_status_failure_code() {
        let status = build_exit_status(false, Some(3), None);
        assert_eq!(status.success(), false);
        assert_eq!(status.code(), Some(3));
    }

    #[cfg(unix)]
    #[test]
    fn test_build_exit_status_signal() {
        use std::os::unix::process::ExitStatusExt;
        let status = build_exit_status(false, None, Some(9));
        assert_eq!(status.signal(), Some(9));
    }
}

/// Task execution monitor for collecting statistics
pub struct TaskExecutionMonitor {
    /// Task being monitored
    task_name: String,
    /// Start time
    start_time: Instant,
}

impl TaskExecutionMonitor {
    /// Start monitoring a task
    pub fn start(task_name: String) -> Self {
        Self {
            task_name,
            start_time: Instant::now(),
        }
    }

    /// Finish monitoring and collect stats
    pub fn finish(self, task_result: &TaskResult) -> TaskExecutionStats {
        TaskExecutionStats {
            task_name: self.task_name.clone(),
            start_time: self.start_time,
            duration: self.start_time.elapsed(),
            exit_code: task_result.exit_status.code(),
            memory_usage: None, // Would need system monitoring
            cpu_time: None,     // Would need system monitoring
            work_dir_size: self.calculate_work_dir_size(&task_result.work_dir),
        }
    }

    fn calculate_work_dir_size(&self, work_dir: &PathBuf) -> Option<u64> {
        use std::fs;

        fn dir_size(path: &std::path::Path) -> std::io::Result<u64> {
            let mut size = 0;
            for entry in fs::read_dir(path)? {
                let entry = entry?;
                let metadata = entry.metadata()?;
                if metadata.is_dir() {
                    size += dir_size(&entry.path())?;
                } else {
                    size += metadata.len();
                }
            }
            Ok(size)
        }

        dir_size(work_dir).ok()
    }
}

/// Utility functions for task execution
pub mod utils {
    use super::*;
    use crate::value::Value;
    // Note: Path available if needed

    /// Convert a file path value to a system path
    pub fn value_to_path(value: &Value) -> Option<PathBuf> {
        match value {
            Value::File { value: path, .. } => Some(PathBuf::from(path)),
            Value::String { value: path, .. } => Some(PathBuf::from(path)),
            _ => None,
        }
    }

    /// Convert array of file values to paths
    pub fn array_to_paths(value: &Value) -> Option<Vec<PathBuf>> {
        match value {
            Value::Array { values: arr, .. } => {
                let paths: Option<Vec<PathBuf>> = arr.iter().map(value_to_path).collect();
                paths
            }
            _ => None,
        }
    }

    /// Check if a file value exists
    pub fn file_exists(value: &Value) -> bool {
        if let Some(path) = value_to_path(value) {
            path.exists()
        } else {
            false
        }
    }

    /// Get file size for a file value
    pub fn file_size(value: &Value) -> Option<u64> {
        if let Some(path) = value_to_path(value) {
            std::fs::metadata(path).map(|m| m.len()).ok()
        } else {
            None
        }
    }

    /// Create a temporary file with given content
    pub fn create_temp_file(content: &str, suffix: &str) -> RuntimeResult<PathBuf> {
        use std::io::Write;

        let temp_dir = std::env::temp_dir();
        let formatted_suffix = if suffix.starts_with('.') {
            suffix.to_string()
        } else {
            format!(".{}", suffix)
        };
        let filename = format!("wdl_temp_{}{}", std::process::id(), formatted_suffix);
        let temp_file = temp_dir.join(filename);

        let mut file = std::fs::File::create(&temp_file).map_err(|e| {
            RuntimeError::file_system_error(
                "Failed to create temporary file".to_string(),
                Some(temp_file.display().to_string()),
                e,
            )
        })?;

        file.write_all(content.as_bytes()).map_err(|e| {
            RuntimeError::file_system_error(
                "Failed to write temporary file".to_string(),
                Some(temp_file.display().to_string()),
                e,
            )
        })?;

        Ok(temp_file)
    }

    /// Format duration for human-readable output
    pub fn format_duration(duration: std::time::Duration) -> String {
        let secs = duration.as_secs();
        let millis = duration.subsec_millis();

        if secs >= 3600 {
            format!("{}h{}m{}s", secs / 3600, (secs % 3600) / 60, secs % 60)
        } else if secs >= 60 {
            format!("{}m{}s", secs / 60, secs % 60)
        } else if secs > 0 {
            format!("{}.{}s", secs, millis / 100)
        } else {
            format!("{}ms", millis)
        }
    }
}

#[cfg(test)]
#[allow(dead_code)]
mod tests {
    // Temporarily disabled for interface integration
    /*
    use super::*;
    use crate::tree::*;
    use crate::expr::*;
    use tempfile::tempdir;
    use std::time::Duration;

    fn create_simple_task() -> Task {
        Task {
            pos: SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 10),
            name: "simple_task".to_string(),
            inputs: vec![
                Decl {
                    pos: SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 2, 1, 2, 20),
                    name: "message".to_string(),
                    wdl_type: crate::Type::String,
                    expr: None, // Required input
                }
            ],
            command: Some(Expr::String(StringExpr {
                pos: SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 3, 1, 3, 30),
                value: "echo '${message}' > output.txt".to_string(),
            })),
            outputs: vec![
                Decl {
                    pos: SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 4, 1, 4, 25),
                    name: "output_file".to_string(),
                    wdl_type: crate::Type::File,
                    expr: Some(Expr::String(StringExpr {
                        pos: SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 4, 20, 4, 35),
                        value: "output.txt".to_string(),
                    })),
                }
            ],
            runtime: None,
            parameter_meta: None,
            meta: None,
        }
    }

    #[test]
    fn test_task_engine_creation() {
        let config = Config::default();
        let temp_dir = tempdir().unwrap();
        let workflow_dir = WorkflowDirectory::create(temp_dir.path(), "test_run").unwrap();

        let engine = TaskEngine::new(config, workflow_dir);
        assert_eq!(engine.config.max_concurrent_tasks, 1);
    }

    #[test]
    fn test_task_validation() {
        let config = Config::default();
        let temp_dir = tempdir().unwrap();
        let workflow_dir = WorkflowDirectory::create(temp_dir.path(), "test_run").unwrap();
        let engine = TaskEngine::new(config, workflow_dir);

        let task = create_simple_task();

        // Valid inputs
        let mut inputs = Env::Bindings::new();
        inputs.insert("message".to_string(), Value::String("Hello World".to_string()));
        assert!(engine.validate_task(&task, &inputs).is_ok());

        // Missing required input
        let empty_inputs = Env::Bindings::new();
        let result = engine.validate_task(&task, &empty_inputs);
        assert!(result.is_err());

        if let Err(RuntimeError::WorkflowValidationError { message, .. }) = result {
            assert!(message.contains("Missing required input"));
        } else {
            panic!("Expected WorkflowValidationError");
        }
    }

    #[test]
    fn test_get_task_inputs() {
        let config = Config::default();
        let temp_dir = tempdir().unwrap();
        let workflow_dir = WorkflowDirectory::create(temp_dir.path(), "test_run").unwrap();
        let engine = TaskEngine::new(config, workflow_dir);

        let task = create_simple_task();
        let inputs = engine.get_task_inputs(&task);

        assert_eq!(inputs.len(), 1);
        assert_eq!(inputs[0].0, "message");
        assert_eq!(inputs[0].1, crate::Type::String);
        assert!(inputs[0].2); // Required
    }

    #[test]
    fn test_get_task_outputs() {
        let config = Config::default();
        let temp_dir = tempdir().unwrap();
        let workflow_dir = WorkflowDirectory::create(temp_dir.path(), "test_run").unwrap();
        let engine = TaskEngine::new(config, workflow_dir);

        let task = create_simple_task();
        let outputs = engine.get_task_outputs(&task);

        assert_eq!(outputs.len(), 1);
        assert_eq!(outputs[0].0, "output_file");
        assert_eq!(outputs[0].1, crate::Type::File);
    }

    #[test]
    fn test_task_directory_creation() {
        let config = Config::default();
        let temp_dir = tempdir().unwrap();
        let workflow_dir = WorkflowDirectory::create(temp_dir.path(), "test_run").unwrap();
        let engine = TaskEngine::new(config, workflow_dir);

        let task_dir = engine.create_task_directory("test_task", "run_123").unwrap();
        assert!(task_dir.exists());
        assert!(task_dir.file_name().unwrap().to_str().unwrap().contains("test_task"));
        assert!(task_dir.file_name().unwrap().to_str().unwrap().contains("run_123"));

        // Cleanup
        engine.cleanup_task_directory(&task_dir).unwrap();
        assert!(!task_dir.exists());
    }

    #[test]
    fn test_task_execution_options() {
        let mut options = TaskExecutionOptions::default();
        options.timeout_override = Some(Duration::from_secs(300));
        options.env_vars.insert("TEST_VAR".to_string(), "test_value".to_string());
        options.copy_inputs = Some(true);
        options.verbose = true;

        assert_eq!(options.timeout_override, Some(Duration::from_secs(300)));
        assert_eq!(options.env_vars.get("TEST_VAR"), Some(&"test_value".to_string()));
        assert_eq!(options.copy_inputs, Some(true));
        assert!(options.verbose);
    }

    #[test]
    fn test_task_execution_monitor() {
        let monitor = TaskExecutionMonitor::start("test_task".to_string());
        assert_eq!(monitor.task_name, "test_task");

        // Simulate task completion (we can't easily test the full execution here)
        std::thread::sleep(Duration::from_millis(10));

        // Would need a real TaskResult to test finish()
        // This is tested more thoroughly in integration tests
    }

    #[test]
    fn test_utils_value_to_path() {
        use super::utils::*;

        let file_value = Value::File("/path/to/file".to_string());
        let string_value = Value::String("/path/to/string".to_string());
        let int_value = Value::Int(42);

        assert_eq!(value_to_path(&file_value), Some(PathBuf::from("/path/to/file")));
        assert_eq!(value_to_path(&string_value), Some(PathBuf::from("/path/to/string")));
        assert_eq!(value_to_path(&int_value), None);
    }

    #[test]
    fn test_utils_array_to_paths() {
        use super::utils::*;

        let array_value = Value::Array(vec![
            Value::File("/file1".to_string()),
            Value::File("/file2".to_string()),
        ]);

        let paths = array_to_paths(&array_value).unwrap();
        assert_eq!(paths.len(), 2);
        assert_eq!(paths[0], PathBuf::from("/file1"));
        assert_eq!(paths[1], PathBuf::from("/file2"));

        let non_array = Value::String("not an array".to_string());
        assert_eq!(array_to_paths(&non_array), None);
    }

    #[test]
    fn test_utils_format_duration() {
        use super::utils::format_duration;

        assert_eq!(format_duration(Duration::from_millis(500)), "500ms");
        assert_eq!(format_duration(Duration::from_secs(2)), "2.0s");
        assert_eq!(format_duration(Duration::from_secs(70)), "1m10s");
        assert_eq!(format_duration(Duration::from_secs(3661)), "1h1m1s");
    }
    */
}
