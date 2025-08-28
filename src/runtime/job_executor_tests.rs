//! Tests for job executor functionality
//!
//! These tests verify the job executor client, schema serialization,
//! and integration with the main task execution system.

use super::job_executor_client::{ExecutorMode, JobExecutorClient, JobExecutorClientConfig};
use super::job_executor_schema::*;
use crate::env::Bindings;
use crate::error::SourcePosition;
use crate::expr::Expression;
use crate::tree::Task;
use crate::types::Type;
use crate::value::Value;
use serde_json;
use std::collections::HashMap;
use std::path::PathBuf;
use tempfile::TempDir;

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

    // Test serialization
    let json = serde_json::to_string_pretty(&request).unwrap();
    assert!(json.contains("test-job-123"));
    assert!(json.contains("ubuntu:20.04"));
    assert!(json.contains("Hello World"));

    // Test deserialization
    let deserialized: JobExecutionRequest = serde_json::from_str(&json).unwrap();
    assert_eq!(deserialized.job_id, "test-job-123");
    assert_eq!(deserialized.task_definition.name, "test_task");
    assert_eq!(
        deserialized.task_definition.docker_image,
        Some("ubuntu:20.04".to_string())
    );
}

#[test]
fn test_job_status_update_serialization() {
    let update = JobStatusUpdate {
        job_id: "test-job-456".to_string(),
        status: JobStatus::Running,
        timestamp: "2024-01-01T12:00:00Z".to_string(),
        progress: Some(JobProgress {
            files_downloaded: 3,
            total_files: 5,
            download_bytes: 1024000,
            execution_started_at: Some("2024-01-01T12:00:05Z".to_string()),
        }),
        error: None,
    };

    let json = serde_json::to_string(&update).unwrap();
    let deserialized: JobStatusUpdate = serde_json::from_str(&json).unwrap();

    assert_eq!(deserialized.job_id, "test-job-456");
    assert_eq!(deserialized.status, JobStatus::Running);
    assert!(deserialized.progress.is_some());

    let progress = deserialized.progress.unwrap();
    assert_eq!(progress.files_downloaded, 3);
    assert_eq!(progress.total_files, 5);
}

#[test]
fn test_job_execution_result_serialization() {
    let result = JobExecutionResult {
        job_id: "test-job-789".to_string(),
        status: JobStatus::Completed,
        exit_code: 0,
        stdout: "Task completed successfully".to_string(),
        stderr: String::new(),
        outputs: HashMap::from([(
            "result_file".to_string(),
            OutputParameter {
                wdl_type: "File".to_string(),
                value: serde_json::Value::String("/path/to/output.txt".to_string()),
                file_path: Some("/path/to/output.txt".to_string()),
            },
        )]),
        metrics: JobMetrics {
            start_time: "2024-01-01T12:00:00Z".to_string(),
            end_time: "2024-01-01T12:05:00Z".to_string(),
            duration_seconds: 300.0,
            cpu_usage_percent: Some(85.2),
            memory_usage_mb: Some(512),
            disk_usage_mb: Some(100),
        },
        artifacts: JobArtifacts {
            command_script: PathBuf::from("/tmp/command.sh"),
            work_directory: PathBuf::from("/tmp/work"),
            logs_directory: PathBuf::from("/tmp/logs"),
        },
    };

    let json = serde_json::to_string_pretty(&result).unwrap();
    let deserialized: JobExecutionResult = serde_json::from_str(&json).unwrap();

    assert_eq!(deserialized.job_id, "test-job-789");
    assert_eq!(deserialized.status, JobStatus::Completed);
    assert_eq!(deserialized.exit_code, 0);
    assert_eq!(deserialized.metrics.duration_seconds, 300.0);
}

#[test]
fn test_job_executor_message_wrapper() {
    let request = JobExecutionRequest {
        job_id: "test-wrapper".to_string(),
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
            assert_eq!(req.job_id, "test-wrapper");
            assert_eq!(req.task_definition.name, "test");
        }
        _ => panic!("Expected ExecutionRequest"),
    }
}

#[test]
fn test_job_executor_client_config() {
    let config = JobExecutorClientConfig {
        mode: ExecutorMode::Local,
        local_executor_path: Some(PathBuf::from("/usr/local/bin/daemon-flowy")),
        remote_api_endpoint: None,
        auth_token: None,
        max_concurrent_jobs: 5,
        job_timeout: std::time::Duration::from_secs(7200),
    };

    let client = JobExecutorClient::new(config.clone());

    // Verify client is created correctly
    assert_eq!(client.config.mode, ExecutorMode::Local);
    assert_eq!(client.config.max_concurrent_jobs, 5);
    assert_eq!(client.config.job_timeout.as_secs(), 7200);
}

#[test]
fn test_value_to_input_parameter_conversion() {
    let config = JobExecutorClientConfig::default();
    let client = JobExecutorClient::new(config);

    // Test String value
    let string_value = Value::String {
        value: "test string".to_string(),
        wdl_type: Type::string(false),
    };
    let param = client.value_to_input_parameter(&string_value).unwrap();
    assert_eq!(param.wdl_type, "String { optional: false }");
    assert_eq!(
        param.value,
        serde_json::Value::String("test string".to_string())
    );

    // Test File value with local path
    let file_value = Value::File {
        value: "/path/to/local/file.txt".to_string(),
        wdl_type: Type::file(false),
    };
    let param = client.value_to_input_parameter(&file_value).unwrap();
    assert!(param.source_location.is_some());
    assert_eq!(param.source_location.unwrap(), "/path/to/local/file.txt");

    // Test File value with S3 URL
    let s3_file_value = Value::File {
        value: "s3://my-bucket/data/input.txt".to_string(),
        wdl_type: Type::file(false),
    };
    let param = client.value_to_input_parameter(&s3_file_value).unwrap();
    assert!(param.source_location.is_some());
    assert_eq!(
        param.source_location.unwrap(),
        "s3://my-bucket/data/input.txt"
    );

    // Test Integer value
    let int_value = Value::Int {
        value: 42,
        wdl_type: Type::int(false),
    };
    let param = client.value_to_input_parameter(&int_value).unwrap();
    assert_eq!(param.value, serde_json::Value::Number(42.into()));
}

#[test]
fn test_runtime_requirements_extraction() {
    let config = JobExecutorClientConfig::default();
    let client = JobExecutorClient::new(config);

    // Create a simple task with empty runtime (testing default values)
    let task = Task {
        pos: SourcePosition::new("test".to_string(), "test.wdl".to_string(), 1, 1, 1, 1),
        name: "test_task".to_string(),
        inputs: None,
        postinputs: vec![],
        outputs: vec![],
        command: Expression::String {
            pos: SourcePosition::new("test".to_string(), "test.wdl".to_string(), 1, 1, 1, 1),
            parts: vec![crate::expr::StringPart::Text("echo hello".to_string())],
            inferred_type: None,
        },
        runtime: HashMap::new(),
        parameter_meta: HashMap::new(),
        meta: HashMap::new(),
        effective_wdl_version: "1.0".to_string(),
    };

    let requirements = client.extract_runtime_requirements(&task).unwrap();
    assert_eq!(requirements.cpu, 1.0);
    assert_eq!(requirements.memory_gb, 2.0);
    assert_eq!(requirements.disk_gb, 10.0);
}

#[test]
fn test_memory_string_parsing() {
    let config = JobExecutorClientConfig::default();
    let client = JobExecutorClient::new(config);

    assert_eq!(client.parse_memory_string("4 GB").unwrap(), 4.0);
    assert_eq!(client.parse_memory_string("2048 MB").unwrap(), 2.0);
    assert_eq!(client.parse_memory_string("8 GiB").unwrap(), 8.0);
    assert_eq!(client.parse_memory_string("1024 MiB").unwrap(), 1.0);

    // Test default fallback for invalid input
    assert_eq!(client.parse_memory_string("invalid").unwrap(), 2.0);
    assert_eq!(client.parse_memory_string("").unwrap(), 2.0);
}

#[test]
fn test_disk_string_parsing() {
    let config = JobExecutorClientConfig::default();
    let client = JobExecutorClient::new(config);

    assert_eq!(
        client.parse_disk_string("local-disk 100 HDD").unwrap(),
        100.0
    );
    assert_eq!(client.parse_disk_string("50 SSD").unwrap(), 50.0);
    assert_eq!(
        client.parse_disk_string("local-disk 200 SSD").unwrap(),
        200.0
    );

    // Test default fallback
    assert_eq!(client.parse_disk_string("invalid").unwrap(), 10.0);
    assert_eq!(client.parse_disk_string("").unwrap(), 10.0);
}

#[test]
fn test_output_parameter_to_value_conversion() {
    let config = JobExecutorClientConfig::default();
    let client = JobExecutorClient::new(config);

    // Test String output
    let string_param = OutputParameter {
        wdl_type: "String".to_string(),
        value: serde_json::Value::String("output string".to_string()),
        file_path: None,
    };
    let value = client.output_parameter_to_value(&string_param).unwrap();
    if let Value::String { value: v, .. } = value {
        assert_eq!(v, "output string");
    } else {
        panic!("Expected String value");
    }

    // Test File output with file_path
    let file_param = OutputParameter {
        wdl_type: "File".to_string(),
        value: serde_json::Value::String("/path/to/output.txt".to_string()),
        file_path: Some("/path/to/output.txt".to_string()),
    };
    let value = client.output_parameter_to_value(&file_param).unwrap();
    if let Value::File { value: v, .. } = value {
        assert_eq!(v, "/path/to/output.txt");
    } else {
        panic!("Expected File value");
    }

    // Test Integer output
    let int_param = OutputParameter {
        wdl_type: "Int".to_string(),
        value: serde_json::Value::Number(123.into()),
        file_path: None,
    };
    let value = client.output_parameter_to_value(&int_param).unwrap();
    if let Value::Int { value: v, .. } = value {
        assert_eq!(v, 123);
    } else {
        panic!("Expected Int value");
    }
}

#[cfg(test)]
mod integration_tests {
    use super::*;
    use tempfile::TempDir;

    #[test]
    fn test_job_executor_client_creation() {
        let temp_dir = TempDir::new().unwrap();
        let executor_path = temp_dir.path().join("daemon-flowy");

        let config = JobExecutorClientConfig {
            mode: ExecutorMode::Local,
            local_executor_path: Some(executor_path.clone()),
            remote_api_endpoint: None,
            auth_token: None,
            max_concurrent_jobs: 3,
            job_timeout: std::time::Duration::from_secs(1800),
        };

        let client = JobExecutorClient::new(config);
        assert_eq!(client.config.mode, ExecutorMode::Local);
        assert_eq!(client.config.local_executor_path.unwrap(), executor_path);
        assert_eq!(client.config.max_concurrent_jobs, 3);
    }

    #[test]
    fn test_remote_executor_config() {
        let config = JobExecutorClientConfig {
            mode: ExecutorMode::Remote,
            local_executor_path: None,
            remote_api_endpoint: Some("https://executor-api.example.com".to_string()),
            auth_token: Some("secret-token".to_string()),
            max_concurrent_jobs: 10,
            job_timeout: std::time::Duration::from_secs(3600),
        };

        let client = JobExecutorClient::new(config);
        assert_eq!(client.config.mode, ExecutorMode::Remote);
        assert!(client.config.remote_api_endpoint.is_some());
        assert!(client.config.auth_token.is_some());
    }
}
