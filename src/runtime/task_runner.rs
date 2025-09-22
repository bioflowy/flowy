use crate::env::Bindings;
use crate::runtime::config::{
    CacheConfig, Config, ContainerBackend, ContainerConfig, ResourceLimits,
};
use crate::runtime::fs_utils::WorkflowDirectory;
use crate::tree::Task;
use crate::value::Value;
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::path::PathBuf;
use std::time::Duration;

/// Version marker for task runner request/response schema.
pub const TASK_RUNNER_PROTOCOL_VERSION: u32 = 1;

/// Serialized representation of a binding.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SerializedBinding<T> {
    pub name: String,
    pub value: T,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub info: Option<String>,
}

impl<T> SerializedBinding<T> {
    pub fn new(name: String, value: T, info: Option<String>) -> Self {
        Self { name, value, info }
    }
}

/// Lightweight container configuration for serialization.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RunnerContainerConfig {
    pub enabled: bool,
    pub backend: RunnerContainerBackend,
    #[serde(default)]
    pub options: HashMap<String, String>,
}

/// Container backends supported by the runner.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum RunnerContainerBackend {
    None,
    Docker,
    Podman,
    Singularity,
}

impl From<&ContainerConfig> for RunnerContainerConfig {
    fn from(config: &ContainerConfig) -> Self {
        Self {
            enabled: config.enabled,
            backend: RunnerContainerBackend::from(&config.backend),
            options: config.options.clone(),
        }
    }
}

impl From<RunnerContainerConfig> for ContainerConfig {
    fn from(config: RunnerContainerConfig) -> Self {
        let mut container = ContainerConfig::default();
        container.enabled = config.enabled;
        container.backend = ContainerBackend::from(config.backend);
        container.options = config.options;
        container
    }
}

impl From<&ContainerBackend> for RunnerContainerBackend {
    fn from(backend: &ContainerBackend) -> Self {
        match backend {
            ContainerBackend::None => Self::None,
            ContainerBackend::Docker => Self::Docker,
            ContainerBackend::Podman => Self::Podman,
            ContainerBackend::Singularity => Self::Singularity,
        }
    }
}

impl From<RunnerContainerBackend> for ContainerBackend {
    fn from(backend: RunnerContainerBackend) -> Self {
        match backend {
            RunnerContainerBackend::None => ContainerBackend::None,
            RunnerContainerBackend::Docker => ContainerBackend::Docker,
            RunnerContainerBackend::Podman => ContainerBackend::Podman,
            RunnerContainerBackend::Singularity => ContainerBackend::Singularity,
        }
    }
}

/// Cache configuration subset for the runner.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RunnerCacheConfig {
    pub enabled: bool,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub dir: Option<PathBuf>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub size_limit: Option<u64>,
}

impl From<&CacheConfig> for RunnerCacheConfig {
    fn from(config: &CacheConfig) -> Self {
        Self {
            enabled: config.enabled,
            dir: config.dir.clone(),
            size_limit: config.size_limit,
        }
    }
}

impl From<RunnerCacheConfig> for CacheConfig {
    fn from(config: RunnerCacheConfig) -> Self {
        CacheConfig {
            enabled: config.enabled,
            dir: config.dir,
            size_limit: config.size_limit,
        }
    }
}

/// Resource limits representation for the runner.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RunnerResourceLimits {
    pub max_memory: Option<u64>,
    pub max_cpu: Option<f64>,
    pub max_disk: Option<u64>,
    pub network: bool,
}

impl From<&ResourceLimits> for RunnerResourceLimits {
    fn from(limits: &ResourceLimits) -> Self {
        Self {
            max_memory: limits.max_memory,
            max_cpu: limits.max_cpu,
            max_disk: limits.max_disk,
            network: limits.network,
        }
    }
}

impl From<RunnerResourceLimits> for ResourceLimits {
    fn from(limits: RunnerResourceLimits) -> Self {
        ResourceLimits {
            max_memory: limits.max_memory,
            max_cpu: limits.max_cpu,
            max_disk: limits.max_disk,
            network: limits.network,
        }
    }
}

/// Serializable configuration payload for the task runner.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RunnerConfig {
    pub max_concurrent_tasks: usize,
    pub task_timeout_secs: u64,
    pub work_dir: PathBuf,
    pub copy_input_files: bool,
    pub debug: bool,
    pub container: RunnerContainerConfig,
    pub cache: RunnerCacheConfig,
    #[serde(default)]
    pub env_vars: HashMap<String, String>,
    pub resources: RunnerResourceLimits,
}

impl From<&Config> for RunnerConfig {
    fn from(config: &Config) -> Self {
        Self {
            max_concurrent_tasks: config.max_concurrent_tasks,
            task_timeout_secs: config.task_timeout.as_secs(),
            work_dir: config.work_dir.clone(),
            copy_input_files: config.copy_input_files,
            debug: config.debug,
            container: RunnerContainerConfig::from(&config.container),
            cache: RunnerCacheConfig::from(&config.cache),
            env_vars: config.env_vars.clone(),
            resources: RunnerResourceLimits::from(&config.resources),
        }
    }
}

impl From<RunnerConfig> for Config {
    fn from(config: RunnerConfig) -> Self {
        let mut runtime_config = Config::default();
        runtime_config.max_concurrent_tasks = config.max_concurrent_tasks;
        runtime_config.task_timeout = Duration::from_secs(config.task_timeout_secs);
        runtime_config.work_dir = config.work_dir;
        runtime_config.copy_input_files = config.copy_input_files;
        runtime_config.debug = config.debug;
        runtime_config.container = ContainerConfig::from(config.container);
        runtime_config.cache = CacheConfig::from(config.cache);
        runtime_config.env_vars = config.env_vars;
        runtime_config.resources = ResourceLimits::from(config.resources);
        runtime_config
    }
}

/// Request payload the parent process writes for the task runner.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TaskRunnerRequest {
    pub version: u32,
    pub run_id: String,
    pub workflow_dir: WorkflowDirectory,
    pub task: Task,
    pub inputs: Vec<SerializedBinding<Value>>,
    pub config: RunnerConfig,
}

/// Response payload produced by the task runner.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TaskRunnerResponse {
    pub version: u32,
    pub run_id: String,
    pub success: bool,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub exit_code: Option<i32>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub signal: Option<i32>,
    pub exit_success: bool,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub stdout: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub stderr: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub duration_ms: Option<u128>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub outputs: Option<Vec<SerializedBinding<Value>>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub work_dir: Option<PathBuf>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub error_type: Option<String>,
}

impl TaskRunnerResponse {
    pub fn success(
        run_id: String,
        exit_code: Option<i32>,
        signal: Option<i32>,
        exit_success: bool,
        stdout: String,
        stderr: String,
        duration_ms: u128,
        outputs: Vec<SerializedBinding<Value>>,
        work_dir: PathBuf,
    ) -> Self {
        Self {
            version: TASK_RUNNER_PROTOCOL_VERSION,
            run_id,
            success: true,
            exit_code,
            signal,
            exit_success,
            stdout: Some(stdout),
            stderr: Some(stderr),
            duration_ms: Some(duration_ms),
            outputs: Some(outputs),
            work_dir: Some(work_dir),
            error: None,
            error_type: None,
        }
    }

    pub fn failure(run_id: String, message: String, error_type: Option<String>) -> Self {
        Self {
            version: TASK_RUNNER_PROTOCOL_VERSION,
            run_id,
            success: false,
            exit_code: None,
            signal: None,
            exit_success: false,
            stdout: None,
            stderr: None,
            duration_ms: None,
            outputs: None,
            work_dir: None,
            error: Some(message),
            error_type,
        }
    }
}

/// Convert runtime bindings into serializable bindings.
pub fn serialize_bindings(bindings: &Bindings<Value>) -> Vec<SerializedBinding<Value>> {
    bindings
        .iter()
        .map(|binding| {
            SerializedBinding::new(
                binding.name().to_string(),
                binding.value().clone(),
                binding.info().cloned(),
            )
        })
        .collect()
}

/// Convert serialized bindings back into runtime bindings.
pub fn deserialize_bindings(bindings: Vec<SerializedBinding<Value>>) -> Bindings<Value> {
    let mut env = Bindings::new();
    for SerializedBinding { name, value, info } in bindings.into_iter().rev() {
        env = env.bind(name, value, info);
    }
    env
}

/// Convert a duration in milliseconds back into `Duration`.
pub fn duration_from_millis(millis: u128) -> Duration {
    Duration::from_millis(millis as u64)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::runtime::config::{Config, ContainerBackend};
    use crate::value::Value;

    #[test]
    fn test_serialize_deserialize_bindings_roundtrip() {
        let mut bindings = Bindings::new();
        bindings = bindings.bind(
            "alpha".to_string(),
            Value::int(1),
            Some("first".to_string()),
        );
        bindings = bindings.bind("beta".to_string(), Value::string("two".to_string()), None);

        let serialized = serialize_bindings(&bindings);
        let restored = deserialize_bindings(serialized);

        assert_eq!(restored.len(), bindings.len());
        assert_eq!(restored.resolve("alpha"), bindings.resolve("alpha"));
        assert_eq!(restored.resolve("beta"), bindings.resolve("beta"));
    }

    #[test]
    fn test_runner_config_roundtrip() {
        let mut config = Config::default();
        config.max_concurrent_tasks = 4;
        config.task_timeout = Duration::from_secs(42);
        config.copy_input_files = true;
        config.debug = true;
        config.container.enabled = true;
        config.container.backend = ContainerBackend::Docker;
        config
            .container
            .options
            .insert("network".to_string(), "host".to_string());
        config.env_vars.insert("FOO".to_string(), "BAR".to_string());
        config.resources.max_cpu = Some(2.5);
        config.resources.network = false;

        let runner_config = RunnerConfig::from(&config);
        let rebuilt = Config::from(runner_config);

        assert_eq!(rebuilt.max_concurrent_tasks, config.max_concurrent_tasks);
        assert_eq!(rebuilt.task_timeout, config.task_timeout);
        assert_eq!(rebuilt.copy_input_files, config.copy_input_files);
        assert_eq!(rebuilt.debug, config.debug);
        assert_eq!(rebuilt.container.enabled, config.container.enabled);
        assert_eq!(rebuilt.container.backend, config.container.backend);
        assert_eq!(rebuilt.container.options, config.container.options);
        assert_eq!(rebuilt.env_vars, config.env_vars);
        assert_eq!(rebuilt.resources.max_cpu, config.resources.max_cpu);
        assert_eq!(rebuilt.resources.network, config.resources.network);
    }
}
