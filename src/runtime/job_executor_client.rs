//! Job executor client for managing task execution in separate processes
//!
//! This module provides a client interface for communicating with job executor
//! processes, enabling task isolation and future distributed execution.

use crate::env::Bindings;
use crate::runtime::error::{RuntimeError, RuntimeResult};
use crate::runtime::job_executor_schema::*;
use crate::runtime::task_context::TaskResult;
use crate::tree::Task;
use crate::types::Type;
use crate::value::{Value, ValueBase};
use chrono::Utc;
use serde_json;
use std::collections::HashMap;
use std::io::{BufRead, BufReader, Write};
use std::path::PathBuf;
use std::process::{Child, Command, ExitStatus, Stdio};
use std::sync::{Arc, Mutex};
use std::time::Duration;
use tokio::sync::mpsc;
use uuid::Uuid;

/// Configuration for job executor client
#[derive(Debug, Clone)]
pub struct JobExecutorClientConfig {
    /// Mode of execution (Local or Remote)
    pub mode: ExecutorMode,
    /// Path to local executor binary
    pub local_executor_path: Option<PathBuf>,
    /// Remote API endpoint URL
    pub remote_api_endpoint: Option<String>,
    /// Authentication token for remote API
    pub auth_token: Option<String>,
    /// Maximum concurrent jobs
    pub max_concurrent_jobs: usize,
    /// Default job timeout
    pub job_timeout: Duration,
}

impl Default for JobExecutorClientConfig {
    fn default() -> Self {
        Self {
            mode: ExecutorMode::Local,
            local_executor_path: None,
            remote_api_endpoint: None,
            auth_token: None,
            max_concurrent_jobs: 10,
            job_timeout: Duration::from_secs(3600),
        }
    }
}

/// Execution mode for job executor
#[derive(Debug, Clone, PartialEq)]
pub enum ExecutorMode {
    /// Execute jobs as local child processes
    Local,
    /// Execute jobs via remote REST API
    Remote,
}

/// Job executor client
pub struct JobExecutorClient {
    pub(crate) config: JobExecutorClientConfig,
    #[allow(dead_code)]
    active_jobs: Arc<Mutex<HashMap<String, JobHandle>>>,
}

/// Handle for an active job
#[allow(dead_code)]
struct JobHandle {
    job_id: String,
    process: Option<Child>,
    status: JobStatus,
    result_receiver: Option<mpsc::Receiver<JobExecutionResult>>,
}

impl JobExecutorClient {
    /// Create a new job executor client
    pub fn new(config: JobExecutorClientConfig) -> Self {
        Self {
            config,
            active_jobs: Arc::new(Mutex::new(HashMap::new())),
        }
    }

    /// Execute a task using the job executor
    pub async fn execute_task(
        &self,
        task: &Task,
        inputs: &Bindings<Value>,
        work_directory: PathBuf,
        container_backend: ContainerBackendType,
        environment_variables: HashMap<String, String>,
    ) -> RuntimeResult<TaskResult> {
        let job_id = Uuid::new_v4().to_string();

        // Build job execution request
        let request = self.build_execution_request(
            job_id.clone(),
            task,
            inputs,
            work_directory,
            container_backend,
            environment_variables,
        )?;

        // Execute based on mode
        match self.config.mode {
            ExecutorMode::Local => self.execute_local(request).await,
            ExecutorMode::Remote => self.execute_remote(request).await,
        }
    }

    /// Build job execution request from task and inputs
    fn build_execution_request(
        &self,
        job_id: String,
        task: &Task,
        inputs: &Bindings<Value>,
        work_directory: PathBuf,
        container_backend: ContainerBackendType,
        environment_variables: HashMap<String, String>,
    ) -> RuntimeResult<JobExecutionRequest> {
        // Convert inputs to InputParameter format
        let mut input_params = HashMap::new();
        for binding in inputs.iter() {
            let param = self.value_to_input_parameter(binding.value())?;
            input_params.insert(binding.name().to_string(), param);
        }

        // Extract runtime requirements
        let runtime_requirements = self.extract_runtime_requirements(task)?;

        // Extract docker image if present
        let docker_image = task.runtime.get("docker").and_then(|expr| {
            // For now, just extract string literals
            // In full implementation, would evaluate the expression
            if let crate::expr::Expression::String { parts, .. } = expr {
                // Extract plain string from parts
                if parts.len() == 1 {
                    if let crate::expr::StringPart::Text(s) = &parts[0] {
                        Some(s.clone())
                    } else {
                        None
                    }
                } else {
                    None
                }
            } else {
                None
            }
        });

        Ok(JobExecutionRequest {
            job_id,
            job_type: JobType::Task,
            task_definition: TaskDefinition {
                name: task.name.clone(),
                wdl_version: "1.0".to_string(), // TODO: Get from context
                docker_image,
                command_template: format!("{:?}", task.command), // TODO: Proper serialization
                runtime_requirements,
            },
            inputs: input_params,
            execution_config: ExecutionConfig {
                work_directory,
                container_backend,
                copy_input_files: false, // TODO: Get from config
                environment_variables,
                resource_limits: None,
            },
            callback: None,
        })
    }

    /// Convert WDL Value to InputParameter
    pub(crate) fn value_to_input_parameter(&self, value: &Value) -> RuntimeResult<InputParameter> {
        let wdl_type = format!("{:?}", value.wdl_type());
        let json_value = value.to_json();

        // Check if this is a File type and extract source location
        let source_location = match value {
            Value::File { value: path, .. } => {
                // Check if path is a URL (S3, GS, HTTP, etc.)
                Some(path.clone())
            }
            _ => None,
        };

        Ok(InputParameter {
            wdl_type,
            value: json_value,
            source_location,
        })
    }

    /// Extract runtime requirements from task
    pub(crate) fn extract_runtime_requirements(
        &self,
        task: &Task,
    ) -> RuntimeResult<RuntimeRequirements> {
        // Default values
        let mut cpu = 1.0;
        let mut memory_gb = 2.0;
        let mut disk_gb = 10.0;
        let mut gpu_count = None;
        let timeout_seconds = self.config.job_timeout.as_secs();

        // Extract from task runtime block
        // CPU
        if let Some(cpu_expr) = task.runtime.get("cpu") {
            // For now, handle literal values
            if let crate::expr::Expression::Int { value, .. } = cpu_expr {
                cpu = *value as f64;
            } else if let crate::expr::Expression::Float { value, .. } = cpu_expr {
                cpu = *value;
            }
        }

        // Memory
        if let Some(mem_expr) = task.runtime.get("memory") {
            // Handle string literals like "4 GB"
            if let crate::expr::Expression::String { parts, .. } = mem_expr {
                if parts.len() == 1 {
                    if let crate::expr::StringPart::Text(s) = &parts[0] {
                        memory_gb = self.parse_memory_string(s)?;
                    }
                }
            }
        }

        // Disk
        if let Some(disk_expr) = task.runtime.get("disks") {
            // Handle string literals like "local-disk 10 HDD"
            if let crate::expr::Expression::String { parts, .. } = disk_expr {
                if parts.len() == 1 {
                    if let crate::expr::StringPart::Text(s) = &parts[0] {
                        disk_gb = self.parse_disk_string(s)?;
                    }
                }
            }
        }

        // GPU
        if let Some(gpu_expr) = task.runtime.get("gpuCount") {
            if let crate::expr::Expression::Int { value, .. } = gpu_expr {
                gpu_count = Some(*value as u32);
            }
        }

        Ok(RuntimeRequirements {
            cpu,
            memory_gb,
            disk_gb,
            gpu_count,
            timeout_seconds,
        })
    }

    /// Parse memory string like "4 GB" or "4096 MB"
    pub(crate) fn parse_memory_string(&self, mem_str: &str) -> RuntimeResult<f64> {
        let parts: Vec<&str> = mem_str.split_whitespace().collect();
        if parts.len() != 2 {
            return Ok(2.0); // Default
        }

        let value = parts[0].parse::<f64>().unwrap_or(2.0);
        let unit = parts[1].to_uppercase();

        match unit.as_str() {
            "GB" | "GIB" => Ok(value),
            "MB" | "MIB" => Ok(value / 1024.0),
            "KB" | "KIB" => Ok(value / 1024.0 / 1024.0),
            _ => Ok(2.0), // Default
        }
    }

    /// Parse disk string like "local-disk 10 HDD"
    pub(crate) fn parse_disk_string(&self, disk_str: &str) -> RuntimeResult<f64> {
        let parts: Vec<&str> = disk_str.split_whitespace().collect();
        if parts.len() < 2 {
            return Ok(10.0); // Default
        }

        // Find the numeric value
        for part in parts {
            if let Ok(value) = part.parse::<f64>() {
                return Ok(value);
            }
        }

        Ok(10.0) // Default
    }

    /// Execute job locally as a child process
    async fn execute_local(&self, request: JobExecutionRequest) -> RuntimeResult<TaskResult> {
        let job_id = request.job_id.clone();

        // Find or use default executor path
        let executor_path = self.config.local_executor_path.clone().unwrap_or_else(|| {
            // Look for daemon-flowy in same directory as current binary
            std::env::current_exe()
                .ok()
                .and_then(|p| p.parent().map(|d| d.join("daemon-flowy")))
                .unwrap_or_else(|| PathBuf::from("daemon-flowy"))
        });

        // Spawn executor process
        let mut child = Command::new(&executor_path)
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()
            .map_err(|e| RuntimeError::ExecutorError {
                message: format!("Failed to spawn job executor: {}", e),
                job_id: Some(job_id.clone()),
                cause: Some(Box::new(e)),
            })?;

        // Send execution request
        let request_json = serde_json::to_string(&JobExecutorMessage::ExecutionRequest(request))
            .map_err(|e| RuntimeError::ExecutorError {
                message: format!("Failed to serialize request: {}", e),
                job_id: Some(job_id.clone()),
                cause: Some(Box::new(e)),
            })?;

        if let Some(mut stdin) = child.stdin.take() {
            stdin
                .write_all(request_json.as_bytes())
                .map_err(|e| RuntimeError::ExecutorError {
                    message: format!("Failed to send request to executor: {}", e),
                    job_id: Some(job_id.clone()),
                    cause: Some(Box::new(e)),
                })?;
            stdin
                .write_all(b"\n")
                .map_err(|e| RuntimeError::ExecutorError {
                    message: format!("Failed to send request to executor: {}", e),
                    job_id: Some(job_id.clone()),
                    cause: Some(Box::new(e)),
                })?;
        }

        // Read responses from executor
        let stdout = child
            .stdout
            .take()
            .ok_or_else(|| RuntimeError::ExecutorError {
                message: "Failed to get executor stdout".to_string(),
                job_id: Some(job_id.clone()),
                cause: None,
            })?;

        let reader = BufReader::new(stdout);
        let mut result = None;

        for line in reader.lines() {
            let line = line.map_err(|e| RuntimeError::ExecutorError {
                message: format!("Failed to read executor output: {}", e),
                job_id: Some(job_id.clone()),
                cause: Some(Box::new(e)),
            })?;

            // Parse message
            let message: JobExecutorMessage =
                serde_json::from_str(&line).map_err(|e| RuntimeError::ExecutorError {
                    message: format!("Failed to parse executor message: {}", e),
                    job_id: Some(job_id.clone()),
                    cause: Some(Box::new(e)),
                })?;

            match message {
                JobExecutorMessage::StatusUpdate(update) => {
                    // Log status update
                    eprintln!("Job {} status: {:?}", update.job_id, update.status);
                }
                JobExecutorMessage::ExecutionResult(exec_result) => {
                    result = Some(exec_result);
                    break;
                }
                _ => {
                    // Unexpected message type
                    eprintln!("Unexpected message from executor: {:?}", message);
                }
            }
        }

        // Wait for child process to complete
        let exit_status = child.wait().map_err(|e| RuntimeError::ExecutorError {
            message: format!("Failed to wait for executor: {}", e),
            job_id: Some(job_id.clone()),
            cause: Some(Box::new(e)),
        })?;

        // Convert result to TaskResult
        let exec_result = result.ok_or_else(|| RuntimeError::ExecutorError {
            message: "No result received from executor".to_string(),
            job_id: Some(job_id.clone()),
            cause: None,
        })?;

        self.convert_to_task_result(exec_result, exit_status)
    }

    /// Execute job remotely via REST API
    async fn execute_remote(&self, _request: JobExecutionRequest) -> RuntimeResult<TaskResult> {
        Err(RuntimeError::ExecutorError {
            message: "Remote execution not yet implemented".to_string(),
            job_id: None,
            cause: None,
        })
    }

    /// Convert JobExecutionResult to TaskResult
    fn convert_to_task_result(
        &self,
        result: JobExecutionResult,
        exit_status: ExitStatus,
    ) -> RuntimeResult<TaskResult> {
        // Convert outputs back to WDL Values
        let mut outputs = Bindings::new();
        for (name, param) in result.outputs {
            let value = self.output_parameter_to_value(&param)?;
            outputs = outputs.bind(name, value, None);
        }

        // Parse duration
        let duration = Duration::from_secs_f64(result.metrics.duration_seconds);

        Ok(TaskResult {
            outputs,
            exit_status,
            stdout: result.stdout,
            stderr: result.stderr,
            duration,
            work_dir: result.artifacts.work_directory,
        })
    }

    /// Convert OutputParameter to WDL Value
    pub(crate) fn output_parameter_to_value(
        &self,
        param: &OutputParameter,
    ) -> RuntimeResult<Value> {
        // For now, handle basic types
        match param.wdl_type.as_str() {
            "String" => {
                if let serde_json::Value::String(s) = &param.value {
                    Ok(Value::String {
                        value: s.clone(),
                        wdl_type: Type::string(false),
                    })
                } else {
                    Err(RuntimeError::ExecutorError {
                        message: format!("Invalid String value: {:?}", param.value),
                        job_id: None,
                        cause: None,
                    })
                }
            }
            "File" => {
                if let Some(path) = &param.file_path {
                    Ok(Value::File {
                        value: path.clone(),
                        wdl_type: Type::file(false),
                    })
                } else if let serde_json::Value::String(s) = &param.value {
                    Ok(Value::File {
                        value: s.clone(),
                        wdl_type: Type::file(false),
                    })
                } else {
                    Err(RuntimeError::ExecutorError {
                        message: format!("Invalid File value: {:?}", param.value),
                        job_id: None,
                        cause: None,
                    })
                }
            }
            "Int" => {
                if let serde_json::Value::Number(n) = &param.value {
                    Ok(Value::Int {
                        value: n.as_i64().unwrap_or(0),
                        wdl_type: Type::int(false),
                    })
                } else {
                    Err(RuntimeError::ExecutorError {
                        message: format!("Invalid Int value: {:?}", param.value),
                        job_id: None,
                        cause: None,
                    })
                }
            }
            // Add more type conversions as needed
            _ => {
                // For complex types, use Value::from_json
                Ok(Value::from_json(param.value.clone()))
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_memory_parsing() {
        let client = JobExecutorClient::new(JobExecutorClientConfig::default());

        assert_eq!(client.parse_memory_string("4 GB").unwrap(), 4.0);
        assert_eq!(client.parse_memory_string("4096 MB").unwrap(), 4.0);
        assert_eq!(client.parse_memory_string("8 GiB").unwrap(), 8.0);
    }

    #[test]
    fn test_disk_parsing() {
        let client = JobExecutorClient::new(JobExecutorClientConfig::default());

        assert_eq!(client.parse_disk_string("local-disk 10 HDD").unwrap(), 10.0);
        assert_eq!(client.parse_disk_string("20 SSD").unwrap(), 20.0);
        assert_eq!(
            client.parse_disk_string("local-disk 100 SSD").unwrap(),
            100.0
        );
    }
}
