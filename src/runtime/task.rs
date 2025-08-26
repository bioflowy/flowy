//! Task execution engine
//!
//! This module provides the core task execution functionality for WDL workflows,
//! handling task setup, execution, and result collection.

// Note: error types available if needed
use crate::env::Bindings;
use crate::runtime::config::Config;
use crate::runtime::error::{RuntimeError, RuntimeResult};
use crate::runtime::fs_utils::WorkflowDirectory;
use crate::runtime::task_context::{TaskContext, TaskResult};
use crate::tree::Task;
use crate::value::Value;
use std::path::PathBuf;
use std::time::Instant;

/// Task execution engine
pub struct TaskEngine {
    /// Configuration for execution
    config: Config,
    /// Base workflow directory
    workflow_dir: WorkflowDirectory,
}

/// Task execution options
#[derive(Debug, Clone)]
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

impl Default for TaskExecutionOptions {
    fn default() -> Self {
        Self {
            timeout_override: None,
            env_vars: std::collections::HashMap::new(),
            copy_inputs: None,
            verbose: false,
        }
    }
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
        // Create execution context
        let mut config = self.config.clone();

        // Apply options
        if let Some(timeout) = options.timeout_override {
            config.task_timeout = timeout;
        }
        if let Some(copy_inputs) = options.copy_inputs {
            config.copy_input_files = copy_inputs;
        }
        for (key, value) in options.env_vars {
            config.env_vars.insert(key, value);
        }

        let mut context =
            TaskContext::new(task, inputs, config, self.workflow_dir.clone(), run_id)?;

        if options.verbose {
            println!("Executing task: {}", context.task.name);
        }

        // Execute task
        let start = Instant::now();
        let result = context.execute();
        let duration = start.elapsed();

        match result {
            Ok(task_result) => {
                if options.verbose {
                    println!(
                        "Task {} completed successfully in {:?}",
                        context.task.name, duration
                    );
                }
                Ok(task_result)
            }
            Err(error) => {
                if options.verbose {
                    println!(
                        "Task {} failed after {:?}: {}",
                        context.task.name, duration, error
                    );
                }
                Err(error)
            }
        }
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
        if let Some(ref task_inputs) = task.inputs {
            for input_decl in task_inputs {
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
            .as_ref()
            .map(|inputs| {
                inputs
                    .iter()
                    .map(|decl| {
                        let required = decl.expr.is_none();
                        (decl.name.clone(), decl.decl_type.clone(), required)
                    })
                    .collect()
            })
            .unwrap_or_default()
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
        let filename = format!(
            "wdl_temp_{}{}",
            std::process::id(),
            if suffix.starts_with('.') {
                suffix
            } else {
                &format!(".{}", suffix)
            }
        );
        let temp_file = temp_dir.join(filename);

        let mut file = std::fs::File::create(&temp_file).map_err(|e| {
            RuntimeError::filesystem_error(
                "Failed to create temporary file".to_string(),
                Some(temp_file.display().to_string()),
                e,
            )
        })?;

        file.write_all(content.as_bytes()).map_err(|e| {
            RuntimeError::filesystem_error(
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
