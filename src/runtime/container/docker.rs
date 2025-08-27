//! Docker container runtime implementation
//!
//! This module implements the ContainerRuntime trait for Docker,
//! providing container execution capabilities using the bollard Docker API client.

use bollard::container::{
    Config as DockerConfig, CreateContainerOptions, LogsOptions, RemoveContainerOptions,
    StartContainerOptions, WaitContainerOptions,
};
use bollard::errors::Error as BollardError;
use bollard::models::{
    ContainerCreateResponse, HostConfig, Mount, MountTypeEnum, RestartPolicy, RestartPolicyNameEnum,
};
use bollard::Docker;
use futures_util::stream::StreamExt;
use std::collections::HashMap;
use std::time::{Duration, Instant};
use tokio::time::timeout;

use super::{ContainerExecution, ContainerResult, ContainerRuntime, ContainerStats};
use crate::runtime::config::Config;
use crate::runtime::error::RuntimeError;

/// Docker-specific container runtime implementation
pub struct DockerRuntime {
    // Note: We create clients dynamically rather than storing them
    // because ContainerRuntime trait methods take &self, not &mut self
}

impl Default for DockerRuntime {
    fn default() -> Self {
        Self::new()
    }
}

impl DockerRuntime {
    /// Create a new Docker runtime instance
    pub fn new() -> Self {
        Self {}
    }

    /// Create Docker client connection
    async fn create_client() -> ContainerResult<Docker> {
        Docker::connect_with_local_defaults().map_err(|e| RuntimeError::ContainerError {
            message: "Failed to connect to Docker daemon".to_string(),
            cause: Some(Box::new(std::io::Error::other(format!(
                "Docker connection error: {}",
                e
            )))),
            container_id: None,
        })
    }

    /// Convert ContainerExecution to Docker container configuration
    fn create_docker_config(execution: &ContainerExecution) -> DockerConfig<String> {
        // Prepare environment variables
        let env: Vec<String> = execution
            .environment
            .iter()
            .map(|(k, v)| format!("{}={}", k, v))
            .collect();

        // Prepare volume mounts
        let mounts: Vec<Mount> = execution
            .path_mappings
            .iter()
            .map(|mapping| Mount {
                typ: Some(MountTypeEnum::BIND),
                source: Some(mapping.host_path.to_string_lossy().to_string()),
                target: Some(mapping.container_path.to_string_lossy().to_string()),
                read_only: Some(mapping.read_only),
                ..Default::default()
            })
            .collect();

        // Prepare resource limits
        let mut host_config = HostConfig {
            mounts: Some(mounts),
            restart_policy: Some(RestartPolicy {
                name: Some(RestartPolicyNameEnum::NO),
                maximum_retry_count: Some(0),
            }),
            ..Default::default()
        };

        // Set CPU limits
        if let Some(cpu_limit) = execution.cpu_limit {
            host_config.nano_cpus = Some((cpu_limit * 1_000_000_000.0) as i64);
        }

        // Set memory limits
        if let Some(memory_limit) = execution.memory_limit {
            host_config.memory = Some(memory_limit as i64);
        }

        if let Some(memory_reservation) = execution.memory_reservation {
            host_config.memory_reservation = Some(memory_reservation as i64);
        }

        DockerConfig {
            image: Some(execution.image.clone()),
            cmd: Some(execution.command.clone()),
            working_dir: Some(execution.working_dir.clone()),
            env: Some(env),
            host_config: Some(host_config),
            ..Default::default()
        }
    }
}

#[async_trait::async_trait]
impl ContainerRuntime for DockerRuntime {
    async fn global_init(&self, _config: &Config) -> ContainerResult<()> {
        // Test Docker connection
        let client = Self::create_client().await?;

        // Verify we can communicate with Docker daemon
        client
            .ping()
            .await
            .map_err(|e| RuntimeError::ContainerError {
                message: "Docker daemon ping failed".to_string(),
                cause: Some(Box::new(std::io::Error::other(format!(
                    "Docker ping error: {}",
                    e
                )))),
                container_id: None,
            })?;

        // TODO: Store client in self (requires refactoring to use Arc<Mutex<>>)

        Ok(())
    }

    async fn detect_resource_limits(&self) -> ContainerResult<HashMap<String, u64>> {
        let client = Self::create_client().await?;

        let info = client
            .info()
            .await
            .map_err(|e| RuntimeError::ContainerError {
                message: "Failed to get Docker system info".to_string(),
                cause: Some(Box::new(std::io::Error::other(format!(
                    "Docker info error: {}",
                    e
                )))),
                container_id: None,
            })?;

        let mut limits = HashMap::new();

        // Extract CPU and memory limits from Docker info
        if let Some(ncpu) = info.ncpu {
            limits.insert("cpu".to_string(), ncpu as u64);
        }

        if let Some(mem_total) = info.mem_total {
            limits.insert("mem_bytes".to_string(), mem_total as u64);
        }

        Ok(limits)
    }

    async fn create_container(
        &self,
        run_id: &str,
        execution: &ContainerExecution,
    ) -> ContainerResult<String> {
        let client = Self::create_client().await?;

        let config = Self::create_docker_config(execution);
        let container_name = format!("miniwdl_{}", run_id);

        let options = CreateContainerOptions {
            name: container_name.clone(),
            platform: None,
        };

        let response: ContainerCreateResponse = client
            .create_container(Some(options), config)
            .await
            .map_err(|e| RuntimeError::ContainerError {
                message: "Failed to create Docker container".to_string(),
                cause: Some(Box::new(std::io::Error::other(format!(
                    "Container creation error: {}",
                    e
                )))),
                container_id: None,
            })?;

        Ok(response.id)
    }

    async fn start_container(&self, container_id: &str) -> ContainerResult<()> {
        let client = Self::create_client().await?;

        client
            .start_container(container_id, None::<StartContainerOptions<String>>)
            .await
            .map_err(|e| RuntimeError::ContainerError {
                message: "Failed to start Docker container".to_string(),
                cause: Some(Box::new(std::io::Error::other(format!(
                    "Container start error: {}",
                    e
                )))),
                container_id: Some(container_id.to_string()),
            })?;

        Ok(())
    }

    async fn wait_for_completion(&self, container_id: &str) -> ContainerResult<ContainerStats> {
        let client = Self::create_client().await?;
        let start_time = Instant::now();

        let options = WaitContainerOptions {
            condition: "not-running",
        };

        let mut stream = client.wait_container(container_id, Some(options));

        // Wait for container to complete with a timeout
        let wait_result = timeout(Duration::from_secs(3600), stream.next())
            .await
            .map_err(|_| RuntimeError::TaskTimeout {
                timeout: Duration::from_secs(3600),
                task_name: container_id.to_string(),
                command: "container execution".to_string(),
            })?;

        let container_result = wait_result
            .ok_or_else(|| RuntimeError::ContainerError {
                message: "Container wait stream ended unexpectedly".to_string(),
                cause: Some(Box::new(std::io::Error::other("Wait stream ended"))),
                container_id: Some(container_id.to_string()),
            })?
            .map_err(|e| RuntimeError::ContainerError {
                message: "Error waiting for container completion".to_string(),
                cause: Some(Box::new(std::io::Error::other(format!(
                    "Wait error: {}",
                    e
                )))),
                container_id: Some(container_id.to_string()),
            })?;

        let exit_code = container_result.status_code as i32;
        let execution_time_ms = start_time.elapsed().as_millis() as u64;

        // TODO: Get actual CPU and memory usage statistics from Docker
        let stats = ContainerStats {
            container_id: container_id.to_string(),
            exit_code,
            cpu_usage: None,
            memory_usage: None,
            execution_time_ms,
        };

        Ok(stats)
    }

    async fn get_logs(&self, container_id: &str) -> ContainerResult<(String, String)> {
        let client = Self::create_client().await?;

        let options = LogsOptions::<String> {
            stdout: true,
            stderr: true,
            ..Default::default()
        };

        let mut log_stream = client.logs(container_id, Some(options));
        let mut stdout = String::new();
        let stderr = String::new();

        while let Some(log_result) = log_stream.next().await {
            match log_result {
                Ok(log_output) => {
                    let bytes = log_output.into_bytes();
                    let content = String::from_utf8_lossy(&bytes);
                    // For simplicity, we'll put all logs in stdout
                    // In a real implementation, we'd properly separate stdout/stderr
                    stdout.push_str(&content);
                }
                Err(e) => {
                    return Err(RuntimeError::ContainerError {
                        message: "Error reading container logs".to_string(),
                        cause: Some(Box::new(std::io::Error::other(format!("Log error: {}", e)))),
                        container_id: Some(container_id.to_string()),
                    });
                }
            }
        }

        Ok((stdout, stderr))
    }

    async fn cleanup_container(&self, container_id: &str) -> ContainerResult<()> {
        let client = Self::create_client().await?;

        let options = RemoveContainerOptions {
            force: true,
            v: true, // Remove volumes
            ..Default::default()
        };

        client
            .remove_container(container_id, Some(options))
            .await
            .map_err(|e| RuntimeError::ContainerError {
                message: "Failed to remove Docker container".to_string(),
                cause: Some(Box::new(std::io::Error::other(format!(
                    "Container removal error: {}",
                    e
                )))),
                container_id: Some(container_id.to_string()),
            })?;

        Ok(())
    }

    async fn is_available(&self) -> bool {
        match Self::create_client().await {
            Ok(client) => client.ping().await.is_ok(),
            Err(_) => false,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::runtime::container::{ContainerExecution, PathMapping};
    use std::path::PathBuf;

    #[test]
    fn test_docker_runtime_creation() {
        let runtime = DockerRuntime::new();
        // DockerRuntime should be created successfully
        // Client connections are created dynamically as needed
        assert_eq!(std::mem::size_of_val(&runtime), 0); // Zero-sized struct
    }

    #[test]
    fn test_docker_config_creation() {
        let execution = ContainerExecution {
            image: "ubuntu:20.04".to_string(),
            command: vec!["echo".to_string(), "hello".to_string()],
            working_dir: "/tmp".to_string(),
            environment: {
                let mut env = HashMap::new();
                env.insert("TEST_VAR".to_string(), "test_value".to_string());
                env
            },
            path_mappings: vec![PathMapping {
                host_path: PathBuf::from("/host"),
                container_path: PathBuf::from("/container"),
                read_only: false,
            }],
            cpu_limit: Some(1.0),
            memory_limit: Some(1024 * 1024 * 1024), // 1GB
            memory_reservation: None,
        };

        let config = DockerRuntime::create_docker_config(&execution);

        assert_eq!(config.image, Some("ubuntu:20.04".to_string()));
        assert_eq!(
            config.cmd,
            Some(vec!["echo".to_string(), "hello".to_string()])
        );
        assert_eq!(config.working_dir, Some("/tmp".to_string()));
        assert!(config.env.is_some());
        assert!(config.host_config.is_some());

        let host_config = config.host_config.unwrap();
        assert!(host_config.mounts.is_some());
        assert_eq!(host_config.nano_cpus, Some(1_000_000_000));
        assert_eq!(host_config.memory, Some(1024 * 1024 * 1024));
    }
}
