//! Container runtime abstraction for task execution
//!
//! This module provides an abstract interface for running WDL tasks in containers,
//! following the pattern from miniwdl's TaskContainer base class.

use crate::env::Bindings;
use crate::error::SourcePosition;
use crate::runtime::config::{Config, ContainerBackend};
use crate::runtime::error::RuntimeError;
use crate::tree::Task;
use crate::types::Type;
use crate::value::Value;
use std::collections::HashMap;
use std::path::{Path, PathBuf};
use std::process::ExitStatus;

/// Host task directory mount point inside the container (mirrors miniwdl)
pub const CONTAINER_TASK_DIR: &str = "/mnt/miniwdl_task_container";
/// Working directory inside the container where the command executes
pub const CONTAINER_WORK_DIR: &str = "/mnt/miniwdl_task_container/work";

/// Result type for container operations
pub type ContainerResult<T> = Result<T, RuntimeError>;

/// Container execution statistics
#[derive(Debug, Clone)]
pub struct ContainerStats {
    /// Container ID
    pub container_id: String,
    /// Exit code from the container
    pub exit_code: i32,
    /// CPU usage statistics (if available)
    pub cpu_usage: Option<u64>,
    /// Memory usage statistics (if available)
    pub memory_usage: Option<u64>,
    /// Container execution time in milliseconds
    pub execution_time_ms: u64,
}

/// Path mapping for container mounts
#[derive(Debug, Clone)]
pub struct PathMapping {
    /// Host path
    pub host_path: PathBuf,
    /// Container path
    pub container_path: PathBuf,
    /// Whether the path is read-only
    pub read_only: bool,
}

/// Container execution configuration
#[derive(Debug, Clone)]
pub struct ContainerExecution {
    /// Docker image to use
    pub image: String,
    /// Command to execute
    pub command: Vec<String>,
    /// Working directory inside container
    pub working_dir: String,
    /// Environment variables
    pub environment: HashMap<String, String>,
    /// Path mappings (host -> container)
    pub path_mappings: Vec<PathMapping>,
    /// CPU limit (in CPU units, e.g. 1.0 = 1 CPU)
    pub cpu_limit: Option<f64>,
    /// Memory limit in bytes
    pub memory_limit: Option<u64>,
    /// Memory reservation in bytes
    pub memory_reservation: Option<u64>,
}

/// Abstract container runtime trait
///
/// This trait defines the interface for container backends (Docker, Podman, etc.)
/// following the pattern from miniwdl's TaskContainer base class.
#[async_trait::async_trait]
pub trait ContainerRuntime: Send + Sync {
    /// Perform one-time initialization of the container backend
    async fn global_init(&self, config: &Config) -> ContainerResult<()>;

    /// Detect maximum available resources for container execution
    async fn detect_resource_limits(&self) -> ContainerResult<HashMap<String, u64>>;

    /// Create a new container with the given configuration
    async fn create_container(
        &self,
        run_id: &str,
        execution: &ContainerExecution,
    ) -> ContainerResult<String>;

    /// Start the container and execute the command
    async fn start_container(&self, container_id: &str) -> ContainerResult<()>;

    /// Wait for container completion and return statistics
    async fn wait_for_completion(&self, container_id: &str) -> ContainerResult<ContainerStats>;

    /// Get container logs (stdout and stderr)
    async fn get_logs(&self, container_id: &str) -> ContainerResult<(String, String)>;

    /// Clean up the container and associated resources
    async fn cleanup_container(&self, container_id: &str) -> ContainerResult<()>;

    /// Check if the container backend is available
    async fn is_available(&self) -> bool;
}

/// Container factory for creating runtime instances
pub struct ContainerFactory;

impl ContainerFactory {
    /// Create a new container runtime instance based on configuration
    pub fn create_runtime(
        backend: &ContainerBackend,
    ) -> ContainerResult<Box<dyn ContainerRuntime>> {
        match backend {
            ContainerBackend::None => Err(RuntimeError::ConfigurationError {
                message: "No container backend configured".to_string(),
                key: Some("container.backend".to_string()),
            }),
            ContainerBackend::Docker => Ok(Box::new(
                crate::runtime::container::docker::DockerRuntime::new(),
            )),
            ContainerBackend::Podman => Err(RuntimeError::ConfigurationError {
                message: "Podman backend not yet implemented".to_string(),
                key: Some("container.backend".to_string()),
            }),
            ContainerBackend::Singularity => Err(RuntimeError::ConfigurationError {
                message: "Singularity backend not yet implemented".to_string(),
                key: Some("container.backend".to_string()),
            }),
        }
    }
}

/// Prepare container execution configuration from a WDL task
pub fn prepare_container_execution(
    task: &Task,
    runtime_env: &Bindings<Value>,
    run_dir: &Path,
    input_file_mappings: &[(String, String)],
) -> ContainerResult<ContainerExecution> {
    // Extract runtime values - handle both String and Array[String] for docker images
    let docker_image = runtime_env
        .resolve("docker")
        .and_then(|v| match v {
            // Handle single string container
            Value::String { value, .. } => Some(value.clone()),
            // Handle array of containers - select the first one
            Value::Array { values, .. } => values.first().and_then(|first_val| {
                if let Value::String { value, .. } = first_val {
                    Some(value.clone())
                } else {
                    None
                }
            }),
            _ => None,
        })
        .ok_or_else(|| RuntimeError::ConfigurationError {
            message: "No docker image specified in task runtime".to_string(),
            key: Some("runtime.docker".to_string()),
        })?;

    // Build command to execute the script file
    // The actual command is written to command.sh by the task context
    let command = vec![
        "/bin/bash".to_string(),
        format!("{}/command.sh", CONTAINER_TASK_DIR),
    ];

    // Set up working directory
    let working_dir = CONTAINER_WORK_DIR.to_string();

    // Extract environment variables
    let mut environment = HashMap::new();

    // Add runtime environment variables if specified
    if let Some(env_value) = runtime_env.resolve("env") {
        if let Value::Map { pairs, .. } = env_value {
            for (key, value) in pairs {
                if let (
                    Value::String { value: key_str, .. },
                    Value::String { value: val_str, .. },
                ) = (key, value)
                {
                    environment.insert(key_str.clone(), val_str.clone());
                }
            }
        }
    }

    // Set up path mappings starting with the run directory
    let mut path_mappings = vec![PathMapping {
        host_path: run_dir.to_path_buf(),
        container_path: PathBuf::from(CONTAINER_TASK_DIR),
        read_only: false,
    }];

    // Add individual input file mappings for container execution
    for (host_path, container_path) in input_file_mappings {
        path_mappings.push(PathMapping {
            host_path: PathBuf::from(host_path),
            container_path: PathBuf::from(container_path),
            read_only: true, // Input files are read-only
        });
    }

    // Extract resource limits
    let cpu_limit = runtime_env
        .resolve("cpu")
        .and_then(|v| v.as_int())
        .map(|i| i as f64);

    let memory_limit = runtime_env
        .resolve("memory")
        .and_then(|v| v.as_string())
        .and_then(parse_memory_string);

    let memory_reservation = runtime_env
        .resolve("memory_reservation")
        .and_then(|v| v.as_string())
        .and_then(parse_memory_string);

    Ok(ContainerExecution {
        image: docker_image.to_string(),
        command,
        working_dir,
        environment,
        path_mappings,
        cpu_limit,
        memory_limit,
        memory_reservation,
    })
}

/// Parse memory specification string (e.g., "4 GB", "512 MB") to bytes
fn parse_memory_string(memory_str: &str) -> Option<u64> {
    let parts: Vec<&str> = memory_str.split_whitespace().collect();
    if parts.len() != 2 {
        return None;
    }

    let amount: f64 = parts[0].parse().ok()?;
    let unit = parts[1].to_uppercase();

    let multiplier: u64 = match unit.as_str() {
        "B" | "BYTES" => 1,
        "KB" | "K" => 1_024,
        "MB" | "M" => 1_024 * 1_024,
        "GB" | "G" => 1_024 * 1_024 * 1_024,
        "TB" | "T" => 1_024u64 * 1_024 * 1_024 * 1_024,
        _ => return None,
    };

    Some((amount * multiplier as f64) as u64)
}

// Re-export docker module
pub mod docker;

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_memory_string() {
        assert_eq!(parse_memory_string("1 GB"), Some(1_073_741_824));
        assert_eq!(parse_memory_string("512 MB"), Some(536_870_912));
        assert_eq!(parse_memory_string("1024 KB"), Some(1_048_576));
        assert_eq!(parse_memory_string("invalid"), None);
        assert_eq!(parse_memory_string("1"), None);
    }

    #[test]
    fn test_path_mapping_creation() {
        let mapping = PathMapping {
            host_path: PathBuf::from("/host/path"),
            container_path: PathBuf::from("/container/path"),
            read_only: true,
        };

        assert_eq!(mapping.host_path, PathBuf::from("/host/path"));
        assert_eq!(mapping.container_path, PathBuf::from("/container/path"));
        assert!(mapping.read_only);
    }

    #[test]
    fn test_container_string_selection() {
        use crate::env::Bindings;
        use crate::types::Type;
        use crate::value::Value;
        use std::path::PathBuf;
        use tempfile::TempDir;

        // Test single container string
        let mut runtime_env = Bindings::new();
        runtime_env = runtime_env.bind(
            "docker".to_string(),
            Value::String {
                value: "ubuntu:latest".to_string(),
                wdl_type: Type::string(false),
            },
            None,
        );

        // Create a minimal task for testing
        let temp_dir = TempDir::new().unwrap();
        let pos = crate::error::SourcePosition::new(
            "test.wdl".to_string(),
            "test.wdl".to_string(),
            1,
            1,
            1,
            10,
        );
        let task = crate::tree::Task {
            name: "test_task".to_string(),
            pos: pos.clone(),
            inputs: vec![],
            postinputs: vec![],
            command: crate::expr::Expression::string_literal(pos.clone(), "echo test".to_string()),
            outputs: vec![],
            runtime: std::collections::HashMap::new(),
            requirements: std::collections::HashMap::new(),
            hints: std::collections::HashMap::new(),
            meta: std::collections::HashMap::new(),
            parameter_meta: std::collections::HashMap::new(),
            effective_wdl_version: "1.2".to_string(),
        };

        let result = prepare_container_execution(&task, &runtime_env, temp_dir.path(), &[]);
        assert!(result.is_ok());
        let execution = result.unwrap();
        assert_eq!(execution.image, "ubuntu:latest");
    }

    #[test]
    fn test_container_array_selection() {
        use crate::env::Bindings;
        use crate::types::Type;
        use crate::value::Value;
        use std::path::PathBuf;
        use tempfile::TempDir;

        // Test container array - should select first available container
        let mut runtime_env = Bindings::new();
        runtime_env = runtime_env.bind(
            "docker".to_string(),
            Value::Array {
                values: vec![
                    Value::String {
                        value: "ubuntu:latest".to_string(),
                        wdl_type: Type::string(false),
                    },
                    Value::String {
                        value: "https://gcr.io/standard-images/ubuntu:latest".to_string(),
                        wdl_type: Type::string(false),
                    },
                ],
                wdl_type: Type::array(Type::string(false), false, false),
            },
            None,
        );

        // Create a minimal task for testing
        let temp_dir = TempDir::new().unwrap();
        let pos = crate::error::SourcePosition::new(
            "test.wdl".to_string(),
            "test.wdl".to_string(),
            1,
            1,
            1,
            10,
        );
        let task = crate::tree::Task {
            name: "test_task".to_string(),
            pos: pos.clone(),
            inputs: vec![],
            postinputs: vec![],
            command: crate::expr::Expression::string_literal(pos.clone(), "echo test".to_string()),
            outputs: vec![],
            runtime: std::collections::HashMap::new(),
            requirements: std::collections::HashMap::new(),
            hints: std::collections::HashMap::new(),
            meta: std::collections::HashMap::new(),
            parameter_meta: std::collections::HashMap::new(),
            effective_wdl_version: "1.2".to_string(),
        };

        let result = prepare_container_execution(&task, &runtime_env, temp_dir.path(), &[]);
        assert!(result.is_ok());
        let execution = result.unwrap();
        // Should select the first container in the array
        assert_eq!(execution.image, "ubuntu:latest");
    }
}
