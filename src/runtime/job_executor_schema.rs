//! JSON schema types for job executor communication
//!
//! This module defines the data structures used for communication between
//! the miniwdl-rust process and job executor processes via JSON.

use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::path::PathBuf;

/// Job execution request sent from miniwdl-rust to job executor
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct JobExecutionRequest {
    /// Unique job identifier
    pub job_id: String,
    /// Type of job (task or workflow)
    pub job_type: JobType,
    /// Task definition details
    pub task_definition: TaskDefinition,
    /// Input parameters for the task
    pub inputs: HashMap<String, InputParameter>,
    /// Execution configuration
    pub execution_config: ExecutionConfig,
    /// Optional callback configuration
    #[serde(skip_serializing_if = "Option::is_none")]
    pub callback: Option<CallbackConfig>,
}

/// Type of job being executed
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum JobType {
    Task,
    Workflow,
}

/// Task definition details
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TaskDefinition {
    /// Task name
    pub name: String,
    /// WDL version
    pub wdl_version: String,
    /// Docker image (if containerized execution)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub docker_image: Option<String>,
    /// Command template with WDL placeholders
    pub command_template: String,
    /// Runtime requirements
    pub runtime_requirements: RuntimeRequirements,
}

/// Runtime resource requirements
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RuntimeRequirements {
    /// CPU cores required
    pub cpu: f64,
    /// Memory in GB
    pub memory_gb: f64,
    /// Disk space in GB
    pub disk_gb: f64,
    /// GPU count (optional)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub gpu_count: Option<u32>,
    /// Timeout in seconds
    pub timeout_seconds: u64,
}

/// Input parameter with type and value
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct InputParameter {
    /// WDL type (String, Int, Float, Boolean, File, Array, Map, etc)
    #[serde(rename = "type")]
    pub wdl_type: String,
    /// Parameter value (JSON value)
    pub value: serde_json::Value,
    /// For File types: source location (local path or remote URL)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub source_location: Option<String>,
}

/// Execution configuration
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ExecutionConfig {
    /// Working directory path
    pub work_directory: PathBuf,
    /// Container backend to use
    pub container_backend: ContainerBackendType,
    /// Whether to copy input files instead of symlinking
    pub copy_input_files: bool,
    /// Environment variables
    pub environment_variables: HashMap<String, String>,
    /// Additional resource limits
    #[serde(skip_serializing_if = "Option::is_none")]
    pub resource_limits: Option<HashMap<String, serde_json::Value>>,
}

/// Container backend type
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ContainerBackendType {
    Docker,
    Podman,
    Singularity,
    None,
}

/// Callback configuration for status updates
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CallbackConfig {
    /// Webhook URL for status updates
    pub url: String,
    /// Additional headers for webhook requests
    #[serde(skip_serializing_if = "Option::is_none")]
    pub headers: Option<HashMap<String, String>>,
}

/// Job status update sent from job executor to miniwdl-rust
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct JobStatusUpdate {
    /// Job identifier
    pub job_id: String,
    /// Current status
    pub status: JobStatus,
    /// Timestamp (ISO 8601)
    pub timestamp: String,
    /// Progress information
    #[serde(skip_serializing_if = "Option::is_none")]
    pub progress: Option<JobProgress>,
    /// Error information (if failed)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<JobError>,
}

/// Job execution status
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(rename_all = "lowercase")]
pub enum JobStatus {
    Queued,
    Downloading,
    Preparing,
    Running,
    Completed,
    Failed,
    Timeout,
    Cancelled,
}

/// Job progress information
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct JobProgress {
    /// Number of files downloaded
    pub files_downloaded: u32,
    /// Total number of files to download
    pub total_files: u32,
    /// Downloaded bytes
    pub download_bytes: u64,
    /// Execution start time (ISO 8601)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub execution_started_at: Option<String>,
}

/// Job error information
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct JobError {
    /// Error code
    pub code: String,
    /// Error message
    pub message: String,
    /// Additional error details
    #[serde(skip_serializing_if = "Option::is_none")]
    pub details: Option<serde_json::Value>,
}

/// Job execution result sent from job executor to miniwdl-rust
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct JobExecutionResult {
    /// Job identifier
    pub job_id: String,
    /// Final status (completed or failed)
    pub status: JobStatus,
    /// Process exit code
    pub exit_code: i32,
    /// Standard output
    pub stdout: String,
    /// Standard error
    pub stderr: String,
    /// Output values
    pub outputs: HashMap<String, OutputParameter>,
    /// Execution metrics
    pub metrics: JobMetrics,
    /// Execution artifacts (paths)
    pub artifacts: JobArtifacts,
}

/// Output parameter with type and value
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OutputParameter {
    /// WDL type
    #[serde(rename = "type")]
    pub wdl_type: String,
    /// Parameter value
    pub value: serde_json::Value,
    /// For File types: path to output file
    #[serde(skip_serializing_if = "Option::is_none")]
    pub file_path: Option<String>,
}

/// Job execution metrics
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct JobMetrics {
    /// Start time (ISO 8601)
    pub start_time: String,
    /// End time (ISO 8601)
    pub end_time: String,
    /// Duration in seconds
    pub duration_seconds: f64,
    /// CPU usage percentage
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cpu_usage_percent: Option<f64>,
    /// Memory usage in MB
    #[serde(skip_serializing_if = "Option::is_none")]
    pub memory_usage_mb: Option<u64>,
    /// Disk usage in MB
    #[serde(skip_serializing_if = "Option::is_none")]
    pub disk_usage_mb: Option<u64>,
}

/// Job execution artifacts
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct JobArtifacts {
    /// Path to command script
    pub command_script: PathBuf,
    /// Working directory path
    pub work_directory: PathBuf,
    /// Logs directory path
    pub logs_directory: PathBuf,
}

/// Job control request (cancel, pause, resume)
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct JobControlRequest {
    /// Job identifier
    pub job_id: String,
    /// Control action
    pub action: JobControlAction,
    /// Reason for action (optional)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub reason: Option<String>,
}

/// Job control action
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum JobControlAction {
    Cancel,
    Pause,
    Resume,
}

/// Message wrapper for bi-directional communication
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "message_type")]
pub enum JobExecutorMessage {
    /// Execution request
    #[serde(rename = "execution_request")]
    ExecutionRequest(JobExecutionRequest),
    /// Status update
    #[serde(rename = "status_update")]
    StatusUpdate(JobStatusUpdate),
    /// Execution result
    #[serde(rename = "execution_result")]
    ExecutionResult(JobExecutionResult),
    /// Control request
    #[serde(rename = "control_request")]
    ControlRequest(JobControlRequest),
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_job_execution_request_serialization() {
        let request = JobExecutionRequest {
            job_id: "test-job-123".to_string(),
            job_type: JobType::Task,
            task_definition: TaskDefinition {
                name: "test_task".to_string(),
                wdl_version: "1.0".to_string(),
                docker_image: Some("ubuntu:20.04".to_string()),
                command_template: "echo ~{message}".to_string(),
                runtime_requirements: RuntimeRequirements {
                    cpu: 2.0,
                    memory_gb: 4.0,
                    disk_gb: 10.0,
                    gpu_count: None,
                    timeout_seconds: 3600,
                },
            },
            inputs: HashMap::from([(
                "message".to_string(),
                InputParameter {
                    wdl_type: "String".to_string(),
                    value: serde_json::Value::String("Hello World".to_string()),
                    source_location: None,
                },
            )]),
            execution_config: ExecutionConfig {
                work_directory: PathBuf::from("/tmp/work"),
                container_backend: ContainerBackendType::Docker,
                copy_input_files: false,
                environment_variables: HashMap::new(),
                resource_limits: None,
            },
            callback: None,
        };

        let json = serde_json::to_string_pretty(&request).unwrap();
        let deserialized: JobExecutionRequest = serde_json::from_str(&json).unwrap();
        assert_eq!(deserialized.job_id, request.job_id);
    }

    #[test]
    fn test_job_status_update_serialization() {
        let update = JobStatusUpdate {
            job_id: "test-job-123".to_string(),
            status: JobStatus::Running,
            timestamp: "2024-01-01T12:00:00Z".to_string(),
            progress: Some(JobProgress {
                files_downloaded: 2,
                total_files: 5,
                download_bytes: 1024000,
                execution_started_at: Some("2024-01-01T12:00:05Z".to_string()),
            }),
            error: None,
        };

        let json = serde_json::to_string(&update).unwrap();
        let deserialized: JobStatusUpdate = serde_json::from_str(&json).unwrap();
        assert_eq!(deserialized.status, JobStatus::Running);
    }

    #[test]
    fn test_message_wrapper() {
        let request = JobExecutionRequest {
            job_id: "test-123".to_string(),
            job_type: JobType::Task,
            task_definition: TaskDefinition {
                name: "test".to_string(),
                wdl_version: "1.0".to_string(),
                docker_image: None,
                command_template: "echo test".to_string(),
                runtime_requirements: RuntimeRequirements {
                    cpu: 1.0,
                    memory_gb: 1.0,
                    disk_gb: 1.0,
                    gpu_count: None,
                    timeout_seconds: 60,
                },
            },
            inputs: HashMap::new(),
            execution_config: ExecutionConfig {
                work_directory: PathBuf::from("/tmp"),
                container_backend: ContainerBackendType::None,
                copy_input_files: false,
                environment_variables: HashMap::new(),
                resource_limits: None,
            },
            callback: None,
        };

        let message = JobExecutorMessage::ExecutionRequest(request);
        let json = serde_json::to_string(&message).unwrap();
        let deserialized: JobExecutorMessage = serde_json::from_str(&json).unwrap();

        match deserialized {
            JobExecutorMessage::ExecutionRequest(req) => {
                assert_eq!(req.job_id, "test-123");
            }
            _ => panic!("Expected ExecutionRequest"),
        }
    }
}
