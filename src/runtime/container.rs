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
    // Extract runtime values
    let docker_image = runtime_env
        .resolve("docker")
        .and_then(|v| v.as_string())
        .ok_or_else(|| RuntimeError::ConfigurationError {
            message: "No docker image specified in task runtime".to_string(),
            key: Some("runtime.docker".to_string()),
        })?;

    // Build command to execute the script file
    // The actual command is written to container_command.sh by the task context
    let command = vec![
        "/bin/bash".to_string(),
        "/tmp/work/container_command.sh".to_string(),
    ];

    // Set up working directory
    let working_dir = "/tmp/work".to_string();

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
        container_path: PathBuf::from("/tmp/work"),
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
}
