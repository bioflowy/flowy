//! Task execution context
//!
//! This module provides the execution context for WDL tasks, including
//! input/output handling, command generation, and resource management.

// Note: error types available if needed
use crate::env::Bindings;
use crate::expr::ExpressionBase;
use crate::runtime::config::{Config, ContainerBackend, ResourceLimits};
use crate::runtime::container::{prepare_container_execution, ContainerFactory, ContainerRuntime};
use crate::runtime::error::{RuntimeError, RuntimeResult};
use crate::runtime::fs_utils::{
    create_dir_all, read_file_to_string, write_file_atomic, WorkflowDirectory,
};
use crate::tree::Task;
use crate::types::Type;
use crate::value::{Value, ValueBase};
use std::collections::HashMap;
#[cfg(unix)]
use std::os::unix::process::ExitStatusExt;
use std::path::PathBuf;
use std::process::{Command, ExitStatus, Stdio};
use std::time::{Duration, Instant};
use url::Url;
// Note: Write trait available if needed

/// Context for executing a WDL task
#[derive(Debug)]
pub struct TaskContext {
    /// Task definition from AST
    pub task: Task,
    /// Input bindings for this task execution
    pub inputs: Bindings<Value>,
    /// Configuration for execution
    pub config: Config,
    /// Workflow directory structure
    pub workflow_dir: WorkflowDirectory,
    /// Task-specific working directory
    pub task_dir: PathBuf,
    /// Environment variables for command execution
    pub env_vars: HashMap<String, String>,
    /// Start time of task execution
    pub start_time: Option<Instant>,
    /// Resource limits for this task
    pub resource_limits: ResourceLimits,
}

/// Result of command execution
#[derive(Debug)]
pub struct CommandResult {
    /// Exit status of the command
    pub exit_status: ExitStatus,
    /// Path to stdout output file (as URL)
    pub stdout_path: Url,
    /// Path to stderr output file (as URL)
    pub stderr_path: Url,
}

/// Result of task execution
#[derive(Debug)]
pub struct TaskResult {
    /// Output bindings from task execution
    pub outputs: Bindings<Value>,
    /// Exit status of the command
    pub exit_status: ExitStatus,
    /// Standard output as file URL
    pub stdout: Url,
    /// Standard error as file URL
    pub stderr: Url,
    /// Task execution duration
    pub duration: Duration,
    /// Working directory used
    pub work_dir: PathBuf,
}

impl TaskContext {
    /// Create a new task execution context
    pub fn new(
        task: Task,
        inputs: Bindings<Value>,
        config: Config,
        workflow_dir: WorkflowDirectory,
        _run_id: &str,
    ) -> RuntimeResult<Self> {
        let task_dir = workflow_dir.work.join(&task.name);
        create_dir_all(&task_dir)?;

        // Merge environment variables from config and system
        let mut env_vars = std::env::vars().collect::<HashMap<String, String>>();
        for (key, value) in &config.env_vars {
            env_vars.insert(key.clone(), value.clone());
        }

        Ok(Self {
            task,
            inputs,
            config: config.clone(),
            workflow_dir,
            task_dir,
            env_vars,
            start_time: None,
            resource_limits: config.resources,
        })
    }

    /// Execute the task
    pub fn execute(&mut self) -> RuntimeResult<TaskResult> {
        self.start_time = Some(Instant::now());

        // Validate inputs against task requirements
        self.validate_inputs()?;

        // Prepare task environment
        self.prepare_environment()?;

        // Generate command string
        let command_str = self.generate_command()?;

        // Execute command
        let command_result = self.execute_command(&command_str)?;

        // Process outputs with command result context
        let outputs = self.collect_outputs(&command_result)?;

        let duration = self.start_time.unwrap().elapsed();

        Ok(TaskResult {
            outputs,
            exit_status: command_result.exit_status,
            stdout: command_result.stdout_path.clone(),
            stderr: command_result.stderr_path.clone(),
            duration,
            work_dir: self.task_dir.clone(),
        })
    }

    /// Validate that all required inputs are provided
    fn validate_inputs(&self) -> RuntimeResult<()> {
        if let Some(inputs) = &self.task.inputs {
            for input_decl in inputs {
                if input_decl.expr.is_none() {
                    // Required input (no default)
                    if !self.inputs.has_binding(&input_decl.name) {
                        return Err(RuntimeError::WorkflowValidationError {
                            message: format!("Missing required input: {}", input_decl.name),
                            pos: input_decl.pos.clone(),
                        });
                    }
                }
            }
        }
        Ok(())
    }

    /// Prepare the execution environment
    pub fn prepare_environment(&self) -> RuntimeResult<()> {
        // Create input files symlinks/copies if needed
        for binding in self.inputs.iter() {
            let name = binding.name();
            let value = binding.value();
            if let Value::File {
                value: file_path, ..
            } = value
            {
                let dest = self.task_dir.join(name);
                if self.config.copy_input_files {
                    crate::runtime::fs_utils::copy_file(file_path, &dest)?;
                } else {
                    crate::runtime::fs_utils::symlink(file_path, &dest)?;
                }
            }
        }
        Ok(())
    }

    /// Generate the command string from the task's command template
    fn generate_command(&self) -> RuntimeResult<String> {
        let command_expr = &self.task.command;

        // Create evaluation environment with inputs and built-in variables
        let mut eval_env = self.inputs.clone();

        // Add built-in variables
        eval_env = eval_env.bind(
            "task".to_string(),
            Value::String {
                value: self.task.name.clone(),
                wdl_type: Type::string(false),
            },
            None,
        );
        eval_env = eval_env.bind(
            "sep".to_string(),
            Value::String {
                value: " ".to_string(),
                wdl_type: Type::string(false),
            },
            None,
        );
        eval_env = eval_env.bind(
            "true".to_string(),
            Value::Boolean {
                value: true,
                wdl_type: Type::boolean(false),
            },
            None,
        );
        eval_env = eval_env.bind(
            "false".to_string(),
            Value::Boolean {
                value: false,
                wdl_type: Type::boolean(false),
            },
            None,
        );

        // Create stdlib for evaluation
        let stdlib = crate::stdlib::StdLib::new("1.0");

        // Evaluate command expression
        let command_value = command_expr.eval(&eval_env, &stdlib).map_err(|e| {
            RuntimeError::run_failed(
                "Failed to evaluate task command".to_string(),
                e,
                Some(command_expr.pos().clone()),
            )
        })?;

        if let Value::String { value: cmd, .. } = command_value {
            Ok(cmd)
        } else {
            Err(RuntimeError::OutputError {
                message: "Task command must evaluate to String".to_string(),
                expected_type: "String".to_string(),
                actual: format!("{:?}", command_value.wdl_type()),
                pos: Some(command_expr.pos().clone()),
            })
        }
    }

    /// Execute the generated command
    pub fn execute_command(&self, command_str: &str) -> RuntimeResult<CommandResult> {
        // Check if container execution is enabled
        if self.config.container.enabled && self.config.container.backend != ContainerBackend::None
        {
            self.execute_command_in_container(command_str)
        } else {
            self.execute_command_directly(command_str)
        }
    }

    /// Execute command directly on the host system
    fn execute_command_directly(
        &self,
        command_str: &str,
    ) -> RuntimeResult<CommandResult> {
        // Write command to script file
        let script_path = self.task_dir.join("command.sh");
        let script_content = format!(
            "#!/bin/bash\nset -euo pipefail\ncd \"{}\"\n{}\n",
            self.task_dir.display(),
            command_str
        );
        write_file_atomic(&script_path, script_content)?;

        // Make script executable
        crate::runtime::fs_utils::make_executable(&script_path)?;

        // Execute command with timeout
        let mut cmd = Command::new("bash");
        cmd.arg(&script_path)
            .current_dir(&self.task_dir)
            .stdout(Stdio::piped())
            .stderr(Stdio::piped());

        // Set environment variables
        for (key, value) in &self.env_vars {
            cmd.env(key, value);
        }

        // Apply resource limits (placeholder - would need system-specific implementation)
        // TODO: Implement actual resource limiting using cgroups or similar

        let child = cmd.spawn().map_err(|e| {
            RuntimeError::filesystem_error(
                "Failed to spawn command".to_string(),
                Some(script_path.display().to_string()),
                e,
            )
        })?;

        // Wait for completion with timeout
        let timeout = self.config.task_timeout;
        let result = self.wait_with_timeout(child, timeout)?;

        Ok(result)
    }

    /// Execute command in a container
    fn execute_command_in_container(
        &self,
        command_str: &str,
    ) -> RuntimeResult<CommandResult> {
        // Create a blocking runtime for container operations
        let rt = tokio::runtime::Runtime::new().map_err(|e| RuntimeError::ContainerError {
            message: "Failed to create async runtime for container execution".to_string(),
            cause: Some(Box::new(e)),
            container_id: None,
        })?;

        rt.block_on(async { self.execute_command_in_container_async(command_str).await })
    }

    /// Async implementation of container execution
    async fn execute_command_in_container_async(
        &self,
        command_str: &str,
    ) -> RuntimeResult<CommandResult> {
        // Create container runtime
        let runtime = ContainerFactory::create_runtime(&self.config.container.backend)?;

        // Initialize container runtime
        runtime.global_init(&self.config).await?;

        // Check if container runtime is available
        if !runtime.is_available().await {
            return Err(RuntimeError::ContainerError {
                message: format!(
                    "Container backend {:?} is not available",
                    self.config.container.backend
                ),
                cause: Some(Box::new(RuntimeError::ConfigurationError {
                    message: "Container daemon not running or not accessible".to_string(),
                    key: None,
                })),
                container_id: None,
            });
        }

        // Prepare runtime environment from task runtime section
        let mut runtime_env = crate::env::Bindings::new();

        // Add basic runtime values that containers need
        let runtime_section = &self.task.runtime;
        for (name, expr) in runtime_section {
            // Evaluate runtime expression in context
            let stdlib = crate::stdlib::StdLib::new("1.0");
            let input_env = self.inputs.clone();
            match expr.eval(&input_env, &stdlib) {
                Ok(value) => {
                    runtime_env = runtime_env.bind(name.clone(), value, None);
                }
                Err(_) => {
                    // Skip runtime values that can't be evaluated
                    continue;
                }
            }
        }

        // Ensure docker image is specified
        if runtime_env.resolve("docker").is_none() {
            return Err(RuntimeError::ContainerError {
                message: "No docker image specified in task runtime section".to_string(),
                cause: Some(Box::new(RuntimeError::ConfigurationError {
                    message: "Container execution requires 'docker' in runtime section".to_string(),
                    key: Some("docker".to_string()),
                })),
                container_id: None,
            });
        }

        // Prepare container execution configuration
        let container_execution =
            prepare_container_execution(&self.task, &runtime_env, &self.task_dir)?;

        // Generate unique run ID for this container
        let run_id = format!("task_{}_{}", self.task.name, std::process::id());

        // Create and start container
        let container_id = runtime
            .create_container(&run_id, &container_execution)
            .await?;

        // Write command to script file in task directory (which gets mounted)
        let script_path = self.task_dir.join("container_command.sh");
        let script_content = format!(
            "#!/bin/bash\nset -euo pipefail\ncd /tmp/work\n{}\n",
            command_str
        );
        write_file_atomic(&script_path, script_content)?;
        crate::runtime::fs_utils::make_executable(&script_path)?;

        // Update container execution to run our script
        let mut updated_execution = container_execution;
        updated_execution.command = vec![
            "/bin/bash".to_string(),
            "/tmp/work/container_command.sh".to_string(),
        ];

        // Create a new container with updated command
        runtime.cleanup_container(&container_id).await?;
        let container_id = runtime
            .create_container(&run_id, &updated_execution)
            .await?;

        // Start container execution
        runtime.start_container(&container_id).await?;

        // Wait for completion
        let stats = runtime.wait_for_completion(&container_id).await?;

        // Get logs
        let (stdout, stderr) = runtime.get_logs(&container_id).await?;

        // Clean up container
        runtime.cleanup_container(&container_id).await?;

        // Convert exit code to ExitStatus
        #[cfg(unix)]
        let exit_status = ExitStatus::from_raw(stats.exit_code << 8);
        #[cfg(not(unix))]
        let exit_status = {
            // For non-Unix systems, we need to create a mock ExitStatus
            // This is a simplified approach - in a real implementation you'd want proper Windows support
            if stats.exit_code == 0 {
                std::process::Command::new("true").status().unwrap()
            } else {
                std::process::Command::new("false").status().unwrap()
            }
        };

        // Write stdout and stderr to files
        let stdout_path = self.task_dir.join("stdout.txt");
        let stderr_path = self.task_dir.join("stderr.txt");
        
        // Write outputs to files
        write_file_atomic(&stdout_path, stdout.as_bytes())?;
        write_file_atomic(&stderr_path, stderr.as_bytes())?;
        
        // Convert paths to file URLs
        let stdout_url = url::Url::from_file_path(&stdout_path)
            .map_err(|_| RuntimeError::filesystem_error(
                "Failed to create stdout URL".to_string(),
                Some(stdout_path.display().to_string()),
                std::io::Error::new(std::io::ErrorKind::InvalidInput, "Invalid path"),
            ))?;
            
        let stderr_url = url::Url::from_file_path(&stderr_path)
            .map_err(|_| RuntimeError::filesystem_error(
                "Failed to create stderr URL".to_string(),
                Some(stderr_path.display().to_string()),
                std::io::Error::new(std::io::ErrorKind::InvalidInput, "Invalid path"),
            ))?;
        
        Ok(CommandResult {
            exit_status,
            stdout_path: stdout_url,
            stderr_path: stderr_url,
        })
    }

    /// Wait for process completion with timeout and write outputs to files
    fn wait_with_timeout(
        &self,
        child: std::process::Child,
        timeout: Duration,
    ) -> RuntimeResult<CommandResult> {
        use std::sync::mpsc;
        use std::thread;

        let (tx, rx) = mpsc::channel();

        // Spawn thread to wait for process
        thread::spawn(move || {
            let output = child.wait_with_output();
            tx.send(output).ok();
        });

        // Wait with timeout
        match rx.recv_timeout(timeout) {
            Ok(Ok(output)) => {
                // Write stdout and stderr to files
                let stdout_path = self.task_dir.join("stdout.txt");
                let stderr_path = self.task_dir.join("stderr.txt");
                
                // Write stdout to file
                write_file_atomic(&stdout_path, &output.stdout)?;
                
                // Write stderr to file
                write_file_atomic(&stderr_path, &output.stderr)?;
                
                // Convert paths to file URLs
                let stdout_url = Url::from_file_path(&stdout_path)
                    .map_err(|_| RuntimeError::filesystem_error(
                        "Failed to create stdout URL".to_string(),
                        Some(stdout_path.display().to_string()),
                        std::io::Error::new(std::io::ErrorKind::InvalidInput, "Invalid path"),
                    ))?;
                    
                let stderr_url = Url::from_file_path(&stderr_path)
                    .map_err(|_| RuntimeError::filesystem_error(
                        "Failed to create stderr URL".to_string(),
                        Some(stderr_path.display().to_string()),
                        std::io::Error::new(std::io::ErrorKind::InvalidInput, "Invalid path"),
                    ))?;
                
                Ok(CommandResult {
                    exit_status: output.status,
                    stdout_path: stdout_url,
                    stderr_path: stderr_url,
                })
            }
            Ok(Err(e)) => Err(RuntimeError::filesystem_error(
                "Process execution failed".to_string(),
                None,
                e,
            )),
            Err(_) => {
                // Timeout occurred - kill process (best effort)
                Err(RuntimeError::TaskTimeout {
                    timeout,
                    task_name: self.task.name.clone(),
                    command: "bash command.sh".to_string(),
                })
            }
        }
    }

    /// Collect task outputs
    fn collect_outputs(&self, command_result: &CommandResult) -> RuntimeResult<Bindings<Value>> {
        let mut outputs = Bindings::new();

        // Create evaluation environment with inputs for output expressions
        let eval_env = self.inputs.clone();
        
        // Create task output-specific standard library that includes stdout/stderr functions
        let stdlib = crate::stdlib::task_output::create_task_output_stdlib("1.0", self.task_dir.clone());

        for output_decl in &self.task.outputs {
            if let Some(output_expr) = &output_decl.expr {
                let output_value = output_expr.eval(&eval_env, &stdlib).map_err(|e| {
                    eprintln!("Error evaluating output '{}': {}", output_decl.name, e);
                    RuntimeError::run_failed(
                        format!("Failed to evaluate output: {}", output_decl.name),
                        e,
                        Some(output_expr.pos().clone()),
                    )
                })?;

                // Validate output type matches declaration
                let expected_type = &output_decl.decl_type;
                if !self.value_matches_type(&output_value, expected_type) {
                    return Err(RuntimeError::OutputError {
                        message: format!("Output type mismatch for: {}", output_decl.name),
                        expected_type: format!("{:?}", expected_type),
                        actual: format!("{:?}", output_value.wdl_type()),
                        pos: Some(output_decl.pos.clone()),
                    });
                }

                outputs = outputs.bind(output_decl.name.clone(), output_value, None);
            } else {
                return Err(RuntimeError::WorkflowValidationError {
                    message: format!(
                        "Output declaration missing expression: {}",
                        output_decl.name
                    ),
                    pos: output_decl.pos.clone(),
                });
            }
        }

        Ok(outputs)
    }

    /// Read stdout from command execution
    #[allow(dead_code)]
    fn read_stdout(&self) -> RuntimeResult<String> {
        let stdout_path = self.task_dir.join("stdout");
        if stdout_path.exists() {
            read_file_to_string(stdout_path)
        } else {
            Ok(String::new())
        }
    }

    /// Read stderr from command execution  
    #[allow(dead_code)]
    fn read_stderr(&self) -> RuntimeResult<String> {
        let stderr_path = self.task_dir.join("stderr");
        if stderr_path.exists() {
            read_file_to_string(stderr_path)
        } else {
            Ok(String::new())
        }
    }

    /// Check if a value matches the expected type
    fn value_matches_type(&self, value: &Value, expected_type: &Type) -> bool {
        // Simple type checking - would need more sophisticated type coercion
        match (value, expected_type) {
            (Value::Boolean { .. }, Type::Boolean { .. }) => true,
            (Value::Int { .. }, Type::Int { .. }) => true,
            (Value::Float { .. }, Type::Float { .. }) => true,
            (Value::String { .. }, Type::String { .. }) => true,
            (Value::File { .. }, Type::File { .. }) => true,
            (Value::Array { values: arr, .. }, Type::Array { item_type, .. }) => {
                arr.iter().all(|v| self.value_matches_type(v, item_type))
            }
            (
                Value::Map { pairs: map, .. },
                Type::Map {
                    key_type,
                    value_type,
                    ..
                },
            ) => map.iter().all(|(k, v)| {
                self.value_matches_type(k, key_type) && self.value_matches_type(v, value_type)
            }),
            (
                Value::Pair { left, right, .. },
                Type::Pair {
                    left_type,
                    right_type,
                    ..
                },
            ) => {
                self.value_matches_type(left, left_type)
                    && self.value_matches_type(right, right_type)
            }
            (Value::Struct { .. }, Type::StructInstance { .. }) => {
                // Would need struct definition lookup for proper validation
                true // Simplified for now
            }
            // Allow null for optional types
            (Value::Null, typ) if typ.is_optional() => true,
            // Type coercion cases
            (Value::Int { .. }, Type::Float { .. }) => true, // Int can be coerced to Float
            (Value::String { .. }, Type::File { .. }) => true, // String can be coerced to File
            _ => false,
        }
    }
}

#[cfg(test)]
#[allow(dead_code)]
mod tests {
    use super::*;
    use crate::tree::*;
    use crate::expr::*;
    use crate::value::Value;
    use crate::types::Type;
    use crate::env::Bindings;
    use crate::error::SourcePosition;
    use tempfile::tempdir;
    use std::path::PathBuf;
    use url::Url;

    fn create_test_task_with_outputs() -> Task {
        use crate::tree::Declaration;
        use std::collections::HashMap;
        
        Task {
            pos: SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 10),
            name: "test_task".to_string(),
            inputs: Some(vec![
                Declaration {
                    pos: SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 2, 1, 2, 20),
                    workflow_node_id: "input1".to_string(),
                    scatter_depth: 0,
                    decl_type: Type::String { optional: false },
                    name: "input_str".to_string(),
                    expr: None,
                    decor: HashMap::new(),
                }
            ]),
            postinputs: vec![],
            command: Expression::String {
                pos: SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 3, 1, 3, 30),
                parts: vec![StringPart::Placeholder {
                    expr: Box::new(Expression::Ident {
                        pos: SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 3, 10, 3, 19),
                        name: "input_str".to_string(),
                        inferred_type: None,
                    }),
                    options: HashMap::new(),
                }],
                inferred_type: None,
            },
            outputs: vec![
                Declaration {
                    pos: SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 4, 1, 4, 25),
                    workflow_node_id: "output1".to_string(),
                    scatter_depth: 0,
                    decl_type: Type::String { optional: false },
                    name: "stdout_content".to_string(),
                    expr: Some(Expression::Apply {
                        pos: SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 4, 15, 4, 25),
                        function_name: "read_string".to_string(),
                        arguments: vec![
                            Expression::Apply {
                                pos: SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 4, 27, 4, 35),
                                function_name: "stdout".to_string(),
                                arguments: vec![],
                                inferred_type: None,
                            }
                        ],
                        inferred_type: None,
                    }),
                    decor: HashMap::new(),
                },
                Declaration {
                    pos: SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 5, 1, 5, 25),
                    workflow_node_id: "output2".to_string(),
                    scatter_depth: 0,
                    decl_type: Type::String { optional: false },
                    name: "stderr_content".to_string(),
                    expr: Some(Expression::Apply {
                        pos: SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 5, 15, 5, 25),
                        function_name: "read_string".to_string(),
                        arguments: vec![
                            Expression::Apply {
                                pos: SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 5, 27, 5, 35),
                                function_name: "stderr".to_string(),
                                arguments: vec![],
                                inferred_type: None,
                            }
                        ],
                        inferred_type: None,
                    }),
                    decor: HashMap::new(),
                }
            ],
            runtime: HashMap::new(),
            parameter_meta: HashMap::new(),
            meta: HashMap::new(),
            effective_wdl_version: "1.0".to_string(),
        }
    }

    #[test]
    fn test_execute_command_returns_file_urls() {
        let task = create_test_task_with_outputs();
        let mut inputs = Bindings::new();
        inputs = inputs.bind("input_str".to_string(), Value::String { value: "hello world".to_string(), wdl_type: Type::String { optional: false } }, None);

        let config = Config::default();
        let temp_dir = tempdir().unwrap();
        let workflow_dir = WorkflowDirectory::create(temp_dir.path(), "test_run").unwrap();

        let context = TaskContext::new(task.clone(), inputs, config.clone(), workflow_dir, "test_run").unwrap();
        
        // Prepare environment to create task directory
        context.prepare_environment().unwrap();
        
        // Generate a simple command
        let command_str = "echo 'Hello stdout' && echo 'Hello stderr' >&2";
        
        // Execute command
        let result = context.execute_command(command_str).unwrap();
        
        // Check that CommandResult contains URLs
        assert!(result.stdout_path.scheme() == "file");
        assert!(result.stderr_path.scheme() == "file");
        
        // Check that the files exist
        let stdout_path = result.stdout_path.to_file_path().unwrap();
        let stderr_path = result.stderr_path.to_file_path().unwrap();
        assert!(stdout_path.exists());
        assert!(stderr_path.exists());
        
        // Check file contents
        let stdout_content = std::fs::read_to_string(&stdout_path).unwrap();
        let stderr_content = std::fs::read_to_string(&stderr_path).unwrap();
        assert!(stdout_content.contains("Hello stdout"));
        assert!(stderr_content.contains("Hello stderr"));
    }

    #[test]
    fn test_collect_outputs_with_stdout_stderr() {
        let task = create_test_task_with_outputs();
        let mut inputs = Bindings::new();
        inputs = inputs.bind("input_str".to_string(), Value::String { value: "test input".to_string(), wdl_type: Type::String { optional: false } }, None);

        let config = Config::default();
        let temp_dir = tempdir().unwrap();
        let workflow_dir = WorkflowDirectory::create(temp_dir.path(), "test_run").unwrap();

        let context = TaskContext::new(task.clone(), inputs, config.clone(), workflow_dir, "test_run").unwrap();
        
        // Prepare environment
        context.prepare_environment().unwrap();
        
        // Create mock stdout and stderr files
        let stdout_path = context.task_dir.join("stdout.txt");
        let stderr_path = context.task_dir.join("stderr.txt");
        std::fs::write(&stdout_path, "Test stdout content").unwrap();
        std::fs::write(&stderr_path, "Test stderr content").unwrap();
        
        // Create CommandResult with file URLs
        let command_result = CommandResult {
            exit_status: std::process::ExitStatus::from_raw(0),
            stdout_path: Url::from_file_path(&stdout_path).unwrap(),
            stderr_path: Url::from_file_path(&stderr_path).unwrap(),
        };
        
        // Collect outputs with the command result
        let outputs = context.collect_outputs(&command_result).unwrap();
        
        // Check that stdout() and stderr() functions work properly
        assert!(outputs.has_binding("stdout_content"));
        assert!(outputs.has_binding("stderr_content"));
        
        let stdout_value = outputs.resolve("stdout_content").unwrap();
        let stderr_value = outputs.resolve("stderr_content").unwrap();
        
        if let Value::String { value, .. } = stdout_value {
            assert_eq!(value, "Test stdout content");
        } else {
            panic!("Expected String value for stdout_content");
        }
        
        if let Value::String { value, .. } = stderr_value {
            assert_eq!(value, "Test stderr content");
        } else {
            panic!("Expected String value for stderr_content");
        }
    }

    // Temporarily disabled for interface integration
    /*
    use super::*;
    use crate::tree::*;
    use crate::expr::*;
    use tempfile::tempdir;
    use std::path::PathBuf;

    fn create_test_task() -> Task {
        Task {
            pos: SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 10),
            name: "test_task".to_string(),
            inputs: vec![
                Decl {
                    pos: SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 2, 1, 2, 20),
                    name: "input_str".to_string(),
                    wdl_type: Type::String,
                    expr: None,
                }
            ],
            command: Some(Expr::String(StringExpr {
                pos: SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 3, 1, 3, 30),
                value: "echo ${input_str}".to_string(),
            })),
            outputs: vec![
                Decl {
                    pos: SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 4, 1, 4, 25),
                    name: "result".to_string(),
                    wdl_type: Type::String,
                    expr: Some(Expr::Apply(ApplyExpr {
                        pos: SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 4, 15, 4, 25),
                        function: "stdout".to_string(),
                        arguments: vec![],
                    })),
                }
            ],
            runtime: None,
            parameter_meta: None,
            meta: None,
        }
    }

    #[test]
    fn test_task_context_creation() {
        let task = create_test_task();
        let mut inputs = Env::Bindings::new();
        inputs.insert("input_str".to_string(), Value::String("hello world".to_string()));

        let config = Config::default();
        let temp_dir = tempdir().unwrap();
        let workflow_dir = WorkflowDirectory::create(temp_dir.path(), "test_run").unwrap();

        let context = TaskContext::new(task, inputs, config, workflow_dir, "test_run");
        assert!(context.is_ok());

        let ctx = context.unwrap();
        assert_eq!(ctx.task.name, "test_task");
        assert!(ctx.task_dir.exists());
    }

    #[test]
    fn test_input_validation() {
        let task = create_test_task();
        let inputs = Env::Bindings::new(); // Missing required input

        let config = Config::default();
        let temp_dir = tempdir().unwrap();
        let workflow_dir = WorkflowDirectory::create(temp_dir.path(), "test_run").unwrap();

        let mut context = TaskContext::new(task, inputs, config, workflow_dir, "test_run").unwrap();
        let result = context.validate_inputs();
        assert!(result.is_err());

        if let Err(RuntimeError::WorkflowValidationError { message, .. }) = result {
            assert!(message.contains("Missing required input"));
        } else {
            panic!("Expected WorkflowValidationError");
        }
    }

    #[test]
    fn test_command_generation() {
        let task = create_test_task();
        let mut inputs = Env::Bindings::new();
        inputs.insert("input_str".to_string(), Value::String("hello world".to_string()));

        let config = Config::default();
        let temp_dir = tempdir().unwrap();
        let workflow_dir = WorkflowDirectory::create(temp_dir.path(), "test_run").unwrap();

        let context = TaskContext::new(task, inputs, config, workflow_dir, "test_run").unwrap();
        let command = context.generate_command();
        assert!(command.is_ok());

        let cmd = command.unwrap();
        assert!(cmd.contains("echo"));
        assert!(cmd.contains("hello world"));
    }

    #[test]
    fn test_value_type_matching() {
        let task = create_test_task();
        let inputs = Env::Bindings::new();
        let config = Config::default();
        let temp_dir = tempdir().unwrap();
        let workflow_dir = WorkflowDirectory::create(temp_dir.path(), "test_run").unwrap();

        let context = TaskContext::new(task, inputs, config, workflow_dir, "test_run").unwrap();

        // Test basic type matching
        assert!(context.value_matches_type(&Value::String { value: "test".to_string(), wdl_type: Type::string(false) }, &Type::String { optional: false }));
        assert!(context.value_matches_type(&Value::Int { value: 42, wdl_type: Type::int(false) }, &Type::Int { optional: false }));
        assert!(context.value_matches_type(&Value::Boolean { value: true, wdl_type: Type::boolean(false) }, &Type::Boolean { optional: false }));

        // Test type coercion
        assert!(context.value_matches_type(&Value::Int { value: 42, wdl_type: Type::int(false) }, &Type::Float { optional: false }));
        assert!(context.value_matches_type(&Value::String { value: "file.txt".to_string(), wdl_type: Type::string(false) }, &Type::File { optional: false }));

        // Test optional types
        assert!(context.value_matches_type(&Value::Null, &Type::String { optional: true }));
        assert!(context.value_matches_type(&Value::String { value: "test".to_string(), wdl_type: Type::string(false) }, &Type::String { optional: true }));

        // Test mismatches
        assert!(!context.value_matches_type(&Value::String { value: "test".to_string(), wdl_type: Type::string(false) }, &Type::Int { optional: false }));
        assert!(!context.value_matches_type(&Value::Boolean { value: true, wdl_type: Type::boolean(false) }, &Type::String { optional: false }));
    }
    */
}

// Task-specific stdlib implementations are now handled directly in the expression evaluator
