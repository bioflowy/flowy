//! Standalone job executor for WDL tasks
//!
//! This binary executes WDL tasks in isolation, communicating with the main
//! miniwdl-rust process via JSON messages over stdin/stdout.

use chrono::Utc;
use miniwdl_rust::runtime::job_executor_schema::*;
use std::collections::HashMap;
use std::fs;
use std::io::{self, BufRead, Write};
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use std::time::{Duration, Instant};

/// Main entry point for job executor
fn main() {
    // Set up panic handler to report errors properly
    std::panic::set_hook(Box::new(|panic_info| {
        eprintln!("Job executor panic: {}", panic_info);
    }));

    // Read execution request from stdin
    let request = match read_execution_request() {
        Ok(req) => req,
        Err(e) => {
            eprintln!("Failed to read execution request: {}", e);
            std::process::exit(1);
        }
    };

    // Execute the job
    let result = execute_job(request);

    // Send result to stdout
    if let Err(e) = send_execution_result(result) {
        eprintln!("Failed to send execution result: {}", e);
        std::process::exit(1);
    }
}

/// Read execution request from stdin
fn read_execution_request() -> Result<JobExecutionRequest, Box<dyn std::error::Error>> {
    let stdin = io::stdin();
    let mut handle = stdin.lock();
    let mut line = String::new();
    handle.read_line(&mut line)?;

    let message: JobExecutorMessage = serde_json::from_str(&line)?;
    match message {
        JobExecutorMessage::ExecutionRequest(req) => Ok(req),
        _ => Err("Expected ExecutionRequest message".into()),
    }
}

/// Send status update to stdout
fn send_status_update(update: JobStatusUpdate) -> Result<(), Box<dyn std::error::Error>> {
    let message = JobExecutorMessage::StatusUpdate(update);
    let json = serde_json::to_string(&message)?;
    println!("{}", json);
    io::stdout().flush()?;
    Ok(())
}

/// Send execution result to stdout
fn send_execution_result(result: JobExecutionResult) -> Result<(), Box<dyn std::error::Error>> {
    let message = JobExecutorMessage::ExecutionResult(result);
    let json = serde_json::to_string(&message)?;
    println!("{}", json);
    io::stdout().flush()?;
    Ok(())
}

/// Execute a job based on the request
fn execute_job(request: JobExecutionRequest) -> JobExecutionResult {
    let start_time = Instant::now();
    let start_timestamp = Utc::now().to_rfc3339();

    // Send initial status update
    let _ = send_status_update(JobStatusUpdate {
        job_id: request.job_id.clone(),
        status: JobStatus::Preparing,
        timestamp: Utc::now().to_rfc3339(),
        progress: None,
        error: None,
    });

    // Create work directory if needed
    if let Err(e) = fs::create_dir_all(&request.execution_config.work_directory) {
        return create_error_result(
            request.job_id,
            format!("Failed to create work directory: {}", e),
            start_timestamp,
            start_time.elapsed(),
        );
    }

    // Download remote files if needed
    let download_result = download_remote_files(&request);
    if let Err(e) = download_result {
        return create_error_result(
            request.job_id,
            format!("Failed to download files: {}", e),
            start_timestamp,
            start_time.elapsed(),
        );
    }

    // Generate command from template
    let command = match generate_command(&request) {
        Ok(cmd) => cmd,
        Err(e) => {
            return create_error_result(
                request.job_id,
                format!("Failed to generate command: {}", e),
                start_timestamp,
                start_time.elapsed(),
            );
        }
    };

    // Write command to script file
    let script_path = request.execution_config.work_directory.join("command.sh");
    if let Err(e) = fs::write(&script_path, &command) {
        return create_error_result(
            request.job_id,
            format!("Failed to write command script: {}", e),
            start_timestamp,
            start_time.elapsed(),
        );
    }

    // Send running status
    let _ = send_status_update(JobStatusUpdate {
        job_id: request.job_id.clone(),
        status: JobStatus::Running,
        timestamp: Utc::now().to_rfc3339(),
        progress: None,
        error: None,
    });

    // Execute command based on container backend
    let (exit_code, stdout, stderr) = match request.execution_config.container_backend {
        ContainerBackendType::None => {
            execute_command_directly(&script_path, &request.execution_config)
        }
        ContainerBackendType::Docker => execute_command_in_docker(&script_path, &request),
        _ => {
            return create_error_result(
                request.job_id,
                format!(
                    "Container backend {:?} not yet supported",
                    request.execution_config.container_backend
                ),
                start_timestamp,
                start_time.elapsed(),
            );
        }
    };

    // Collect outputs
    let outputs = collect_outputs(&request);

    // Create execution result
    let end_timestamp = Utc::now().to_rfc3339();
    let duration = start_time.elapsed();

    JobExecutionResult {
        job_id: request.job_id,
        status: if exit_code == 0 {
            JobStatus::Completed
        } else {
            JobStatus::Failed
        },
        exit_code,
        stdout,
        stderr,
        outputs,
        metrics: JobMetrics {
            start_time: start_timestamp,
            end_time: end_timestamp,
            duration_seconds: duration.as_secs_f64(),
            cpu_usage_percent: None,
            memory_usage_mb: None,
            disk_usage_mb: None,
        },
        artifacts: JobArtifacts {
            command_script: script_path,
            work_directory: request.execution_config.work_directory.clone(),
            logs_directory: request.execution_config.work_directory.join("logs"),
        },
    }
}

/// Download remote files referenced in inputs
fn download_remote_files(request: &JobExecutionRequest) -> Result<(), Box<dyn std::error::Error>> {
    let mut total_files = 0;
    let mut downloaded_files = 0;

    // Count files to download
    for param in request.inputs.values() {
        if let Some(source) = &param.source_location {
            if source.starts_with("s3://")
                || source.starts_with("gs://")
                || source.starts_with("http://")
                || source.starts_with("https://")
            {
                total_files += 1;
            }
        }
    }

    if total_files == 0 {
        return Ok(());
    }

    // Send downloading status
    let _ = send_status_update(JobStatusUpdate {
        job_id: request.job_id.clone(),
        status: JobStatus::Downloading,
        timestamp: Utc::now().to_rfc3339(),
        progress: Some(JobProgress {
            files_downloaded: 0,
            total_files,
            download_bytes: 0,
            execution_started_at: None,
        }),
        error: None,
    });

    // Download each file
    for (name, param) in &request.inputs {
        if let Some(source) = &param.source_location {
            if source.starts_with("s3://") {
                // Download from S3
                download_s3_file(source, &request.execution_config.work_directory, name)?;
                downloaded_files += 1;
            } else if source.starts_with("http://") || source.starts_with("https://") {
                // Download from HTTP/HTTPS
                download_http_file(source, &request.execution_config.work_directory, name)?;
                downloaded_files += 1;
            }
            // Add other cloud storage providers as needed

            // Update progress
            let _ = send_status_update(JobStatusUpdate {
                job_id: request.job_id.clone(),
                status: JobStatus::Downloading,
                timestamp: Utc::now().to_rfc3339(),
                progress: Some(JobProgress {
                    files_downloaded: downloaded_files,
                    total_files,
                    download_bytes: 0, // TODO: Track actual bytes
                    execution_started_at: None,
                }),
                error: None,
            });
        }
    }

    Ok(())
}

/// Download file from S3
fn download_s3_file(
    s3_url: &str,
    work_dir: &Path,
    name: &str,
) -> Result<(), Box<dyn std::error::Error>> {
    // Parse S3 URL: s3://bucket/key
    let url_parts: Vec<&str> = s3_url.trim_start_matches("s3://").splitn(2, '/').collect();
    if url_parts.len() != 2 {
        return Err(format!("Invalid S3 URL: {}", s3_url).into());
    }

    let _bucket = url_parts[0];
    let _key = url_parts[1];
    let local_path = work_dir.join(name);

    // Use AWS CLI for now (in production, would use aws-sdk-rust)
    let output = Command::new("aws")
        .args(["s3", "cp", s3_url, local_path.to_str().unwrap()])
        .output()?;

    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        return Err(format!("Failed to download from S3: {}", stderr).into());
    }

    Ok(())
}

/// Download file from HTTP/HTTPS
fn download_http_file(
    url: &str,
    work_dir: &Path,
    name: &str,
) -> Result<(), Box<dyn std::error::Error>> {
    let local_path = work_dir.join(name);

    // Use curl for now (in production, would use reqwest)
    let output = Command::new("curl")
        .args(["-L", "-o", local_path.to_str().unwrap(), url])
        .output()?;

    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        return Err(format!("Failed to download from HTTP: {}", stderr).into());
    }

    Ok(())
}

/// Generate command from template and inputs
fn generate_command(request: &JobExecutionRequest) -> Result<String, Box<dyn std::error::Error>> {
    // For now, just return the template
    // In full implementation, would substitute placeholders with actual values
    let mut command = request.task_definition.command_template.clone();

    // Simple placeholder replacement for demonstration
    for (name, param) in &request.inputs {
        let placeholder = format!("~{{{}}}", name);
        let value_str = match &param.value {
            serde_json::Value::String(s) => s.clone(),
            serde_json::Value::Number(n) => n.to_string(),
            serde_json::Value::Bool(b) => b.to_string(),
            _ => param.value.to_string(),
        };
        command = command.replace(&placeholder, &value_str);
    }

    Ok(command)
}

/// Execute command directly on the host
fn execute_command_directly(script_path: &Path, config: &ExecutionConfig) -> (i32, String, String) {
    let mut cmd = Command::new("bash");
    cmd.arg(script_path)
        .current_dir(&config.work_directory)
        .stdout(Stdio::piped())
        .stderr(Stdio::piped());

    // Set environment variables
    for (key, value) in &config.environment_variables {
        cmd.env(key, value);
    }

    match cmd.output() {
        Ok(output) => {
            let exit_code = output.status.code().unwrap_or(-1);
            let stdout = String::from_utf8_lossy(&output.stdout).to_string();
            let stderr = String::from_utf8_lossy(&output.stderr).to_string();
            (exit_code, stdout, stderr)
        }
        Err(e) => (
            -1,
            String::new(),
            format!("Failed to execute command: {}", e),
        ),
    }
}

/// Execute command in Docker container
fn execute_command_in_docker(
    _script_path: &Path,
    request: &JobExecutionRequest,
) -> (i32, String, String) {
    let docker_image = request
        .task_definition
        .docker_image
        .as_deref()
        .unwrap_or("ubuntu:20.04");

    let mut cmd = Command::new("docker");
    cmd.args(["run", "--rm"])
        .arg("-v")
        .arg(format!(
            "{}:/work",
            request.execution_config.work_directory.display()
        ))
        .arg("-w")
        .arg("/work");

    // Set resource limits
    if request.task_definition.runtime_requirements.cpu > 0.0 {
        cmd.arg("--cpus")
            .arg(request.task_definition.runtime_requirements.cpu.to_string());
    }
    if request.task_definition.runtime_requirements.memory_gb > 0.0 {
        let memory_bytes =
            (request.task_definition.runtime_requirements.memory_gb * 1024.0 * 1024.0 * 1024.0)
                as u64;
        cmd.arg("-m").arg(memory_bytes.to_string());
    }

    // Add environment variables
    for (key, value) in &request.execution_config.environment_variables {
        cmd.arg("-e").arg(format!("{}={}", key, value));
    }

    // Add image and command
    cmd.arg(docker_image)
        .arg("bash")
        .arg("/work/command.sh")
        .stdout(Stdio::piped())
        .stderr(Stdio::piped());

    match cmd.output() {
        Ok(output) => {
            let exit_code = output.status.code().unwrap_or(-1);
            let stdout = String::from_utf8_lossy(&output.stdout).to_string();
            let stderr = String::from_utf8_lossy(&output.stderr).to_string();
            (exit_code, stdout, stderr)
        }
        Err(e) => (
            -1,
            String::new(),
            format!("Failed to execute Docker command: {}", e),
        ),
    }
}

/// Collect output files and values
fn collect_outputs(request: &JobExecutionRequest) -> HashMap<String, OutputParameter> {
    let mut outputs = HashMap::new();

    // For now, just create placeholder outputs
    // In full implementation, would parse task outputs and collect actual values

    // Check for common output files
    let stdout_path = request.execution_config.work_directory.join("stdout");
    if stdout_path.exists() {
        outputs.insert(
            "stdout".to_string(),
            OutputParameter {
                wdl_type: "File".to_string(),
                value: serde_json::Value::String(stdout_path.to_string_lossy().to_string()),
                file_path: Some(stdout_path.to_string_lossy().to_string()),
            },
        );
    }

    let stderr_path = request.execution_config.work_directory.join("stderr");
    if stderr_path.exists() {
        outputs.insert(
            "stderr".to_string(),
            OutputParameter {
                wdl_type: "File".to_string(),
                value: serde_json::Value::String(stderr_path.to_string_lossy().to_string()),
                file_path: Some(stderr_path.to_string_lossy().to_string()),
            },
        );
    }

    outputs
}

/// Create error result
fn create_error_result(
    job_id: String,
    error_message: String,
    start_time: String,
    duration: Duration,
) -> JobExecutionResult {
    JobExecutionResult {
        job_id,
        status: JobStatus::Failed,
        exit_code: -1,
        stdout: String::new(),
        stderr: error_message,
        outputs: HashMap::new(),
        metrics: JobMetrics {
            start_time,
            end_time: Utc::now().to_rfc3339(),
            duration_seconds: duration.as_secs_f64(),
            cpu_usage_percent: None,
            memory_usage_mb: None,
            disk_usage_mb: None,
        },
        artifacts: JobArtifacts {
            command_script: PathBuf::new(),
            work_directory: PathBuf::new(),
            logs_directory: PathBuf::new(),
        },
    }
}
