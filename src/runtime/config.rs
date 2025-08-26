//! Runtime configuration system
//!
//! This module provides configuration management for WDL workflow execution,
//! including resource limits, timeouts, and execution parameters.

use std::collections::HashMap;
use std::path::PathBuf;
use std::time::Duration;

/// Main configuration structure for workflow execution
#[derive(Debug, Clone)]
pub struct Config {
    /// Maximum number of concurrent tasks (for future parallelization)
    pub max_concurrent_tasks: usize,

    /// Default task timeout
    pub task_timeout: Duration,

    /// Working directory for execution
    pub work_dir: PathBuf,

    /// Copy input files to working directory instead of symlinking
    pub copy_input_files: bool,

    /// Enable debug logging
    pub debug: bool,

    /// Container configuration (placeholder for future implementation)
    pub container: ContainerConfig,

    /// Cache configuration (placeholder for future implementation)
    pub cache: CacheConfig,

    /// Custom environment variables
    pub env_vars: HashMap<String, String>,

    /// Resource limits
    pub resources: ResourceLimits,
}

/// Container execution configuration (placeholder)
#[derive(Debug, Clone)]
pub struct ContainerConfig {
    /// Enable container execution
    pub enabled: bool,

    /// Container backend (docker, podman, etc.)
    pub backend: ContainerBackend,

    /// Additional container options
    pub options: HashMap<String, String>,
}

/// Container backend types
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ContainerBackend {
    /// No container (direct execution)
    None,
    /// Docker backend (future implementation)
    Docker,
    /// Podman backend (future implementation)
    Podman,
    /// Singularity backend (future implementation)
    Singularity,
}

/// Cache configuration (placeholder)
#[derive(Debug, Clone, Default)]
pub struct CacheConfig {
    /// Enable caching
    pub enabled: bool,

    /// Cache directory
    pub dir: Option<PathBuf>,

    /// Cache size limit in bytes
    pub size_limit: Option<u64>,
}

/// Resource limits for task execution
#[derive(Debug, Clone)]
pub struct ResourceLimits {
    /// Maximum memory per task in bytes
    pub max_memory: Option<u64>,

    /// Maximum CPU cores per task
    pub max_cpu: Option<f64>,

    /// Maximum disk space per task in bytes
    pub max_disk: Option<u64>,

    /// Network access allowed
    pub network: bool,
}

impl Default for Config {
    fn default() -> Self {
        Self {
            max_concurrent_tasks: 1,                 // Sequential execution initially
            task_timeout: Duration::from_secs(3600), // 1 hour default
            work_dir: std::env::current_dir().unwrap_or_else(|_| PathBuf::from(".")),
            copy_input_files: false,
            debug: false,
            container: ContainerConfig::default(),
            cache: CacheConfig::default(),
            env_vars: HashMap::new(),
            resources: ResourceLimits::default(),
        }
    }
}

impl Default for ContainerConfig {
    fn default() -> Self {
        Self {
            enabled: false, // Direct execution by default
            backend: ContainerBackend::None,
            options: HashMap::new(),
        }
    }
}

impl Default for ResourceLimits {
    fn default() -> Self {
        Self {
            max_memory: None, // No limits by default
            max_cpu: None,
            max_disk: None,
            network: true, // Allow network access by default
        }
    }
}

impl Config {
    /// Create a new configuration with default values
    pub fn new() -> Self {
        Self::default()
    }

    /// Set working directory
    pub fn with_work_dir<P: Into<PathBuf>>(mut self, work_dir: P) -> Self {
        self.work_dir = work_dir.into();
        self
    }

    /// Enable debug logging
    pub fn with_debug(mut self, debug: bool) -> Self {
        self.debug = debug;
        self
    }

    /// Set task timeout
    pub fn with_task_timeout(mut self, timeout: Duration) -> Self {
        self.task_timeout = timeout;
        self
    }

    /// Set maximum concurrent tasks
    pub fn with_max_concurrent_tasks(mut self, max_tasks: usize) -> Self {
        self.max_concurrent_tasks = max_tasks;
        self
    }

    /// Add environment variable
    pub fn with_env_var<K: Into<String>, V: Into<String>>(mut self, key: K, value: V) -> Self {
        self.env_vars.insert(key.into(), value.into());
        self
    }

    /// Set memory limit
    pub fn with_max_memory(mut self, max_memory: u64) -> Self {
        self.resources.max_memory = Some(max_memory);
        self
    }

    /// Set CPU limit
    pub fn with_max_cpu(mut self, max_cpu: f64) -> Self {
        self.resources.max_cpu = Some(max_cpu);
        self
    }

    /// Enable container execution (placeholder for future implementation)
    pub fn with_container_backend(mut self, backend: ContainerBackend) -> Self {
        self.container.enabled = backend != ContainerBackend::None;
        self.container.backend = backend;
        self
    }

    /// Enable caching (placeholder for future implementation)
    pub fn with_cache_dir<P: Into<PathBuf>>(mut self, cache_dir: P) -> Self {
        self.cache.enabled = true;
        self.cache.dir = Some(cache_dir.into());
        self
    }

    /// Validate configuration
    pub fn validate(&self) -> Result<(), String> {
        if self.max_concurrent_tasks == 0 {
            return Err("max_concurrent_tasks must be greater than 0".to_string());
        }

        if self.task_timeout.is_zero() {
            return Err("task_timeout must be greater than 0".to_string());
        }

        if !self.work_dir.is_absolute() && !self.work_dir.exists() {
            if let Err(e) = std::fs::create_dir_all(&self.work_dir) {
                return Err(format!(
                    "Cannot create work directory {:?}: {}",
                    self.work_dir, e
                ));
            }
        }

        if let Some(cache_dir) = &self.cache.dir {
            if self.cache.enabled && !cache_dir.exists() {
                if let Err(e) = std::fs::create_dir_all(cache_dir) {
                    return Err(format!(
                        "Cannot create cache directory {:?}: {}",
                        cache_dir, e
                    ));
                }
            }
        }

        Ok(())
    }
}

/// Configuration builder for fluent API
pub struct ConfigBuilder {
    config: Config,
}

impl ConfigBuilder {
    /// Create a new configuration builder
    pub fn new() -> Self {
        Self {
            config: Config::default(),
        }
    }

    /// Set working directory
    pub fn work_dir<P: Into<PathBuf>>(mut self, work_dir: P) -> Self {
        self.config.work_dir = work_dir.into();
        self
    }

    /// Enable debug logging
    pub fn debug(mut self, debug: bool) -> Self {
        self.config.debug = debug;
        self
    }

    /// Set task timeout
    pub fn task_timeout(mut self, timeout: Duration) -> Self {
        self.config.task_timeout = timeout;
        self
    }

    /// Set maximum concurrent tasks
    pub fn max_concurrent_tasks(mut self, max_tasks: usize) -> Self {
        self.config.max_concurrent_tasks = max_tasks;
        self
    }

    /// Add environment variable
    pub fn env_var<K: Into<String>, V: Into<String>>(mut self, key: K, value: V) -> Self {
        self.config.env_vars.insert(key.into(), value.into());
        self
    }

    /// Build the configuration
    pub fn build(self) -> Result<Config, String> {
        self.config.validate()?;
        Ok(self.config)
    }
}

impl Default for ConfigBuilder {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::time::Duration;

    #[test]
    fn test_default_config() {
        let config = Config::default();
        assert_eq!(config.max_concurrent_tasks, 1);
        assert_eq!(config.task_timeout, Duration::from_secs(3600));
        assert!(!config.debug);
        assert!(!config.container.enabled);
        assert_eq!(config.container.backend, ContainerBackend::None);
        assert!(!config.cache.enabled);
    }

    #[test]
    fn test_config_builder() {
        let config = ConfigBuilder::new()
            .work_dir("/tmp/test")
            .debug(true)
            .task_timeout(Duration::from_secs(1800))
            .max_concurrent_tasks(4)
            .env_var("TEST_VAR", "test_value")
            .build()
            .unwrap();

        assert_eq!(config.work_dir, PathBuf::from("/tmp/test"));
        assert!(config.debug);
        assert_eq!(config.task_timeout, Duration::from_secs(1800));
        assert_eq!(config.max_concurrent_tasks, 4);
        assert_eq!(
            config.env_vars.get("TEST_VAR"),
            Some(&"test_value".to_string())
        );
    }

    #[test]
    fn test_config_fluent_api() {
        let config = Config::new()
            .with_work_dir("/tmp/test")
            .with_debug(true)
            .with_task_timeout(Duration::from_secs(900))
            .with_max_concurrent_tasks(2)
            .with_env_var("KEY", "value")
            .with_max_memory(1024 * 1024 * 1024) // 1GB
            .with_max_cpu(2.0);

        assert_eq!(config.work_dir, PathBuf::from("/tmp/test"));
        assert!(config.debug);
        assert_eq!(config.task_timeout, Duration::from_secs(900));
        assert_eq!(config.max_concurrent_tasks, 2);
        assert_eq!(config.resources.max_memory, Some(1024 * 1024 * 1024));
        assert_eq!(config.resources.max_cpu, Some(2.0));
    }

    #[test]
    fn test_config_validation() {
        // Valid configuration
        let config = Config::default();
        assert!(config.validate().is_ok());

        // Invalid max_concurrent_tasks
        let config = Config {
            max_concurrent_tasks: 0,
            ..Config::default()
        };
        assert!(config.validate().is_err());

        // Invalid task_timeout
        let config = Config {
            task_timeout: Duration::from_secs(0),
            ..Config::default()
        };
        assert!(config.validate().is_err());
    }

    #[test]
    fn test_container_config() {
        let config = Config::default().with_container_backend(ContainerBackend::Docker);

        assert!(config.container.enabled);
        assert_eq!(config.container.backend, ContainerBackend::Docker);
    }
}
