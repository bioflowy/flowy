//! flowy CLI
//!
//! Command-line interface for executing WDL workflows and tasks.

#![allow(clippy::result_large_err)]
#![allow(clippy::field_reassign_with_default)]
#![allow(clippy::missing_transmute_annotations)]
#![allow(clippy::unneeded_struct_pattern)]

use flowy::{
    core::{
        api::{ErrorResponse as ApiErrorResponse, ExecuteOptions, ExecuteRequest, ExecuteResponse},
        inputs::{bindings_from_json_for_document, bindings_from_json_for_task},
        outputs::bindings_to_json_with_namespace,
    },
    load, runtime,
    tree::{Document, Task},
    Bindings, SourcePosition, Value, WdlError,
};
use reqwest::blocking::Client;
use serde::Deserialize;
use std::fs;
use std::path::{Path, PathBuf};
use std::process;
use std::thread;
use std::time::Duration;

const QUEUE_POLL_INTERVAL: Duration = Duration::from_secs(2);

/// CLI arguments structure
struct Args {
    /// Command to execute
    command: Command,
    /// Enable debug output
    debug: bool,
}

/// Available CLI commands
#[derive(Debug)]
enum Command {
    /// Run a WDL file
    Run {
        /// WDL file to execute
        wdl_file: PathBuf,
        /// Input JSON file (optional)
        input_file: Option<PathBuf>,
        /// Working directory (optional)
        work_dir: Option<PathBuf>,
        /// Task to run (if not running workflow)
        task: Option<String>,
        /// Configuration file (optional)
        config_file: Option<PathBuf>,
        /// Remote server URL (optional)
        server: Option<String>,
        /// Base directory for resolving relative File inputs
        base_dir: Option<PathBuf>,
    },
}

/// Display error with enhanced location information
fn display_error_with_location(error: &WdlError, wdl_file: Option<&std::path::Path>) {
    // Helper function to display error with position
    let display_with_pos = |pos: &SourcePosition, message: &str| {
        // Use the command line file path if position doesn't have filename
        let filename = if pos.uri.is_empty() {
            if let Some(file) = wdl_file {
                file.to_string_lossy().to_string()
            } else {
                String::new()
            }
        } else {
            pos.uri.clone()
        };

        // Show file location and context if available
        eprintln!(
            "Error in {}:{}:{}: {}",
            filename, pos.line, pos.column, message
        );

        // Try to show source code context
        let file_to_read = if filename.is_empty() {
            if let Some(file) = wdl_file {
                file
            } else {
                std::path::Path::new("")
            }
        } else {
            std::path::Path::new(&filename)
        };

        if let Ok(content) = std::fs::read_to_string(file_to_read) {
            let lines: Vec<&str> = content.lines().collect();
            if pos.line > 0 {
                if let Some(error_line) = lines.get((pos.line - 1) as usize) {
                    eprintln!("    {}", error_line);

                    // Create a caret pointer to show exact position
                    let pointer_pos = if pos.column > 0 { pos.column - 1 } else { 0 };
                    let pointer = " ".repeat(pointer_pos as usize) + "^";
                    eprintln!("    {}", pointer);
                }
            }
        }
    };

    display_single_error(error, &display_with_pos, wdl_file);
}

/// Display a single error, handling MultipleValidation recursively
fn display_single_error<F>(
    error: &WdlError,
    display_with_pos: &F,
    wdl_file: Option<&std::path::Path>,
) where
    F: Fn(&SourcePosition, &str),
{
    match error {
        WdlError::Syntax { pos, message, .. } => {
            display_with_pos(pos, message);
        }
        WdlError::Validation { pos, message, .. } => {
            display_with_pos(pos, message);
        }
        WdlError::UnknownIdentifier { pos, message, .. } => {
            display_with_pos(pos, &format!("Unknown identifier: {}", message));
        }
        WdlError::InvalidType { pos, message, .. } => {
            display_with_pos(pos, &format!("Invalid type: {}", message));
        }
        WdlError::IndeterminateType { pos, message, .. } => {
            display_with_pos(pos, &format!("Indeterminate type: {}", message));
        }
        WdlError::NoSuchTask { pos, name, .. } => {
            display_with_pos(pos, &format!("No such task/workflow: {}", name));
        }
        WdlError::NoSuchCall { pos, name, .. } => {
            display_with_pos(pos, &format!("No such call in this workflow: {}", name));
        }
        WdlError::NoSuchFunction { pos, name, .. } => {
            display_with_pos(pos, &format!("No such function: {}", name));
        }
        WdlError::WrongArity {
            pos,
            function_name,
            expected,
            ..
        } => {
            display_with_pos(
                pos,
                &format!("{} expects {} argument(s)", function_name, expected),
            );
        }
        WdlError::NotAnArray { pos, .. } => {
            display_with_pos(pos, "Not an array");
        }
        WdlError::NoSuchMember { pos, member, .. } => {
            display_with_pos(pos, &format!("No such member '{}'", member));
        }
        WdlError::StaticTypeMismatch {
            pos,
            expected,
            actual,
            message,
            ..
        } => {
            let full_message = if message.is_empty() {
                format!("Expected {} instead of {}", expected, actual)
            } else {
                message.clone()
            };
            display_with_pos(pos, &full_message);
        }
        WdlError::IncompatibleOperand { pos, message, .. } => {
            display_with_pos(pos, &format!("Incompatible operand: {}", message));
        }
        WdlError::NoSuchInput { pos, name, .. } => {
            display_with_pos(pos, &format!("No such input {}", name));
        }
        WdlError::UncallableWorkflow { pos, name, .. } => {
            display_with_pos(pos, &format!("Cannot call subworkflow {} because its own calls have missing required inputs, and/or it lacks an output section", name));
        }
        WdlError::MultipleDefinitions { pos, message, .. } => {
            display_with_pos(pos, &format!("Multiple definitions: {}", message));
        }
        WdlError::StrayInputDeclaration { pos, message, .. } => {
            display_with_pos(pos, &format!("Stray input declaration: {}", message));
        }
        WdlError::CircularDependencies { pos, name, .. } => {
            display_with_pos(pos, &format!("Circular dependencies involving {}", name));
        }
        WdlError::Eval { pos, message, .. } => {
            display_with_pos(pos, &format!("Evaluation error: {}", message));
        }
        WdlError::OutOfBounds { pos, .. } => {
            display_with_pos(pos, "Array index out of bounds");
        }
        WdlError::EmptyArray { pos, .. } => {
            display_with_pos(pos, "Empty array for Array+ input/declaration");
        }
        WdlError::NullValue { pos, .. } => {
            display_with_pos(pos, "Null value");
        }
        WdlError::WorkflowValidationError { pos, message, .. } => {
            display_with_pos(pos, &format!("Workflow validation error: {}", message));
        }
        WdlError::MultipleValidation {
            exceptions, count, ..
        } => {
            eprintln!("Multiple validation errors ({} errors):", count);
            for (i, exception) in exceptions.iter().enumerate() {
                eprintln!("  {}.", i + 1);
                display_error_with_location(exception, wdl_file);
            }
        }
        _ => {
            eprintln!("Error: {}", error);
        }
    }
}

fn main() {
    // Parse command-line arguments
    let args = parse_args();

    // Store the wdl_file for error reporting
    let wdl_file = match &args.command {
        Command::Run { wdl_file, .. } => wdl_file.clone(),
    };

    // Execute the command
    let result = match args.command {
        Command::Run { .. } => run_wdl(args),
    };

    if let Err(e) = result {
        display_error_with_location(&e, Some(&wdl_file));
        process::exit(1);
    }
}

fn parse_args() -> Args {
    let args: Vec<String> = std::env::args().collect();

    if args.len() < 2 {
        print_help(&args[0]);
        process::exit(1);
    }

    let mut debug = false;
    let mut filtered_args = Vec::new();

    // Filter out --debug flag and set the debug variable
    for arg in &args[1..] {
        if arg == "--debug" {
            debug = true;
        } else {
            filtered_args.push(arg.clone());
        }
    }

    // If no args left after filtering, show help
    if filtered_args.is_empty() {
        print_help(&args[0]);
        process::exit(1);
    }

    // Check if first argument is a command
    let command = match filtered_args[0].as_str() {
        "run" => {
            if filtered_args.len() > 1 {
                parse_run_command(&filtered_args[1..])
            } else {
                eprintln!("Error: WDL file required");
                process::exit(1);
            }
        }
        "-h" | "--help" => {
            print_help(&args[0]);
            process::exit(0);
        }
        _ => {
            // Assume it's a WDL file for backward compatibility
            parse_run_command(&filtered_args)
        }
    };

    Args { command, debug }
}

fn parse_run_command(args: &[String]) -> Command {
    if args.is_empty() {
        eprintln!("Error: WDL file required");
        process::exit(1);
    }

    let mut wdl_file = None;
    let mut input_file = None;
    let mut work_dir = None;
    let mut task = None;
    let mut config_file = None;
    let mut server = None;
    let mut base_dir = None;

    let mut i = 0;
    while i < args.len() {
        match args[i].as_str() {
            "-i" | "--input" => {
                i += 1;
                if i < args.len() {
                    input_file = Some(PathBuf::from(&args[i]));
                } else {
                    eprintln!("Error: --input requires a file path");
                    process::exit(1);
                }
            }
            "-d" | "--dir" => {
                i += 1;
                if i < args.len() {
                    work_dir = Some(PathBuf::from(&args[i]));
                } else {
                    eprintln!("Error: --dir requires a directory path");
                    process::exit(1);
                }
            }
            "-t" | "--task" => {
                i += 1;
                if i < args.len() {
                    task = Some(args[i].clone());
                } else {
                    eprintln!("Error: --task requires a task name");
                    process::exit(1);
                }
            }
            "-c" | "--config" => {
                i += 1;
                if i < args.len() {
                    config_file = Some(PathBuf::from(&args[i]));
                } else {
                    eprintln!("Error: --config requires a file path");
                    process::exit(1);
                }
            }
            "-s" | "--server" => {
                i += 1;
                if i < args.len() {
                    server = Some(args[i].clone());
                } else {
                    eprintln!("Error: --server requires a URL");
                    process::exit(1);
                }
            }
            "--basedir" => {
                i += 1;
                if i < args.len() {
                    base_dir = Some(PathBuf::from(&args[i]));
                } else {
                    eprintln!("Error: --basedir requires a directory path");
                    process::exit(1);
                }
            }
            "--debug" => {
                // Skip --debug flag, it's handled in parse_args()
            }
            _ => {
                if wdl_file.is_none() && !args[i].starts_with('-') {
                    wdl_file = Some(PathBuf::from(&args[i]));
                } else if args[i].starts_with('-') {
                    eprintln!("Error: Unknown option: {}", args[i]);
                    process::exit(1);
                }
            }
        }
        i += 1;
    }

    let wdl_file = wdl_file.unwrap_or_else(|| {
        eprintln!("Error: WDL file required");
        process::exit(1);
    });

    Command::Run {
        wdl_file,
        input_file,
        work_dir,
        task,
        config_file,
        server,
        base_dir,
    }
}

fn print_help(program: &str) {
    eprintln!("flowy - WDL workflow executor");
    eprintln!();
    eprintln!("Usage:");
    eprintln!(
        "  {} run <wdl_file> [options]           Run a WDL workflow or task",
        program
    );
    eprintln!();
    eprintln!("Run command options:");
    eprintln!("  -i, --input <file>    Input JSON file");
    eprintln!("  -d, --dir <dir>       Working directory (default: temp)");
    eprintln!("  -t, --task <name>     Run specific task instead of workflow");
    eprintln!("  -c, --config <file>   Configuration JSON file");
    eprintln!("  -s, --server <url>    Execute remotely via flowy-server");
    eprintln!(
        "      --basedir <dir>    Base directory for resolving relative File inputs (remote)"
    );
    eprintln!();
    eprintln!("Global options:");
    eprintln!("  --debug               Enable debug output");
    eprintln!("  -h, --help            Show this help message");
    eprintln!();
    eprintln!("Examples:");
    eprintln!("  {} run workflow.wdl -i inputs.json", program);
}

fn run_wdl(args: Args) -> Result<(), WdlError> {
    let (wdl_file, input_file, work_dir, task, config_file, server, base_dir) = match args.command {
        Command::Run {
            wdl_file,
            input_file,
            work_dir,
            task,
            config_file,
            server,
            base_dir,
        } => (
            wdl_file,
            input_file,
            work_dir,
            task,
            config_file,
            server,
            base_dir,
        ),
    };

    let base_dir_resolved = match base_dir {
        Some(dir) => {
            let resolved = if dir.is_absolute() {
                dir
            } else {
                std::env::current_dir()
                    .map_err(|e| WdlError::RuntimeError {
                        message: format!("Failed to get current directory: {}", e),
                    })?
                    .join(dir)
            };
            Some(resolved)
        }
        None => None,
    };

    if let Some(server_url) = server {
        return run_remote(&server_url, wdl_file, input_file, task, base_dir_resolved);
    }

    let _base_dir_guard = flowy::core::inputs::set_input_base_dir(base_dir_resolved.clone());

    // Read WDL file
    let _wdl_content = fs::read_to_string(&wdl_file).map_err(|e| WdlError::RuntimeError {
        message: format!("Failed to read WDL file: {}", e),
    })?;

    // Load WDL document with imports and type checking
    eprintln!("Parsing {}...", wdl_file.display());
    let filename = wdl_file.to_string_lossy();

    // Get parent directory for import resolution
    let parent_dir_str = wdl_file.parent().map(|p| p.to_string_lossy().into_owned());
    let search_paths: Option<Vec<&str>> = parent_dir_str.as_ref().map(|p| vec![p.as_str()]);
    let search_paths_slice = search_paths.as_deref();

    let document = load(&filename, search_paths_slice, true, 10)?;

    // Set up working directory
    let work_dir = work_dir.unwrap_or_else(|| std::env::temp_dir().join("flowy"));
    fs::create_dir_all(&work_dir).map_err(|e| WdlError::RuntimeError {
        message: format!("Failed to create working directory: {}", e),
    })?;

    eprintln!("Working directory: {}", work_dir.display());

    // Build runtime configuration
    let mut config = if let Some(config_file) = config_file {
        eprintln!("Loading config from {}...", config_file.display());
        load_config(&config_file)?
    } else {
        flowy::runtime::Config::default()
    };

    // Override with command-line options
    config.work_dir = work_dir.clone();
    if args.debug {
        config.debug = true;
    }

    // Generate run ID
    let run_id = format!("run_{}", chrono::Utc::now().timestamp());

    // Execute workflow or task
    let (result, workflow_name) = if let Some(task_name) = task {
        // Run specific task
        eprintln!("Running task '{}'...", task_name);

        // Find the task in the document
        let task = document
            .tasks
            .iter()
            .find(|t| t.name == task_name)
            .ok_or_else(|| WdlError::Validation {
                message: format!("Task '{}' not found in document", task_name),
                pos: document.pos.clone(),
                source_text: Some(String::new()),
                declared_wdl_version: Some("1.0".to_string()),
            })?;

        // Load inputs specific to this task
        let inputs = if let Some(input_file) = input_file {
            eprintln!("Loading inputs from {}...", input_file.display());
            load_inputs_for_task(&input_file, task)?
        } else {
            Bindings::new()
        };

        let task_result = runtime::run_task(task.clone(), inputs, config, &run_id, &work_dir)
            .map_err(|e| WdlError::RuntimeError {
                message: format!("Task execution failed: {}", e),
            })?;

        (task_result, Some(task.name.clone())) // Use task name as namespace
    } else {
        // Run workflow
        let workflow_name = if let Some(workflow) = &document.workflow {
            eprintln!("Running workflow '{}'...", workflow.name);
            Some(workflow.name.clone())
        } else {
            eprintln!("No workflow found, running tasks...");
            // If no workflow but tasks exist, use the first task name as namespace
            if !document.tasks.is_empty() {
                Some(document.tasks[0].name.clone())
            } else {
                None
            }
        };

        // Load inputs for workflow or first task
        let inputs = if let Some(input_file) = input_file {
            eprintln!("Loading inputs from {}...", input_file.display());
            load_inputs(&input_file, &document)?
        } else {
            Bindings::new()
        };

        let workflow_result = runtime::run_document(document, inputs, config, &run_id, &work_dir)?;

        // Convert WorkflowResult to TaskResult-like output
        let task_result = runtime::task_context::TaskResult {
            outputs: workflow_result.outputs,
            stdout: url::Url::parse("file:///dev/null").unwrap(), // Placeholder stdout
            stderr: url::Url::parse("file:///dev/null").unwrap(), // Placeholder stderr
            exit_status: unsafe { std::mem::transmute(0u32) },    // Placeholder exit status
            duration: Duration::from_secs(0),
            work_dir: work_dir.clone(),
        };

        (task_result, workflow_name)
    };

    // Print outputs as JSON
    eprintln!("\nExecution completed successfully!");
    eprintln!("Outputs:");

    let output_json = bindings_to_json_with_namespace(&result.outputs, workflow_name.as_deref())?;
    println!("{}", serde_json::to_string_pretty(&output_json).unwrap());

    Ok(())
}

#[allow(dead_code)]
#[derive(Debug, Deserialize)]
struct JobStatusResponse {
    run_id: String,
    state: String,
    worker_id: Option<String>,
    #[serde(default)]
    result: Option<JobResultStatus>,
}

#[derive(Debug, Deserialize)]
#[serde(tag = "type", rename_all = "lowercase")]
enum JobResultStatus {
    Success { response: ExecuteResponse },
    Failure { message: String },
}

#[derive(Debug, Deserialize)]
struct EnqueueJobResponse {
    run_id: String,
}

fn run_remote(
    server: &str,
    wdl_file: PathBuf,
    input_file: Option<PathBuf>,
    task: Option<String>,
    base_dir: Option<PathBuf>,
) -> Result<(), WdlError> {
    let wdl = fs::read_to_string(&wdl_file).map_err(|e| WdlError::RuntimeError {
        message: format!("Failed to read WDL file: {}", e),
    })?;

    let inputs = if let Some(path) = input_file {
        let content = fs::read_to_string(&path).map_err(|e| WdlError::RuntimeError {
            message: format!("Failed to read input file: {}", e),
        })?;
        serde_json::from_str(&content).map_err(|e| WdlError::Validation {
            message: format!("Invalid JSON in input file: {}", e),
            pos: flowy::SourcePosition::new(
                path.display().to_string(),
                path.display().to_string(),
                1,
                1,
                1,
                1,
            ),
            source_text: Some(content),
            declared_wdl_version: Some("1.0".to_string()),
        })?
    } else {
        serde_json::Value::Object(serde_json::Map::new())
    };

    let base_dir_path = if let Some(dir) = base_dir {
        if dir.is_absolute() {
            dir
        } else {
            std::env::current_dir()
                .map_err(|e| WdlError::RuntimeError {
                    message: format!("Failed to get current directory: {}", e),
                })?
                .join(dir)
        }
    } else {
        std::env::current_dir().map_err(|e| WdlError::RuntimeError {
            message: format!("Failed to get current directory: {}", e),
        })?
    };

    let request = ExecuteRequest {
        wdl,
        inputs,
        options: Some(ExecuteOptions {
            task,
            run_id: None,
            base_dir: Some(base_dir_path.to_string_lossy().to_string()),
        }),
    };

    let client = Client::new();
    let server_base = server.trim_end_matches('/');

    let run_id = enqueue_job(&client, server_base, &request)?;
    println!("run_id: {}", run_id);
    let response_json = wait_for_job_completion(&client, server_base, &run_id)?;

    println!("status: {}", response_json.status);
    println!("duration_ms: {}", response_json.duration_ms);
    println!(
        "outputs: {}",
        serde_json::to_string_pretty(&response_json.outputs).unwrap()
    );
    if let Some(stdout) = response_json.stdout {
        println!("stdout:\n{}", stdout);
    }
    if let Some(stderr) = response_json.stderr {
        println!("stderr:\n{}", stderr);
    }
    Ok(())
}

fn enqueue_job(
    client: &Client,
    server_base: &str,
    request: &ExecuteRequest,
) -> Result<String, WdlError> {
    let url = format!("{}/api/v1/jobs", server_base);
    let response = client
        .post(&url)
        .json(request)
        .send()
        .map_err(|e| WdlError::RuntimeError {
            message: format!("Failed to contact server: {}", e),
        })?;

    let status = response.status();
    let body = response.text().map_err(|e| WdlError::RuntimeError {
        message: format!("Failed to read enqueue response body: {}", e),
    })?;

    if !status.is_success() {
        if let Ok(err) = serde_json::from_str::<ApiErrorResponse>(&body) {
            return Err(WdlError::RuntimeError {
                message: format!("Server error: {}", err.message),
            });
        }
        return Err(WdlError::RuntimeError {
            message: format!("Server error (HTTP {}): {}", status, body),
        });
    }

    let response: EnqueueJobResponse =
        serde_json::from_str(&body).map_err(|e| WdlError::RuntimeError {
            message: format!("Failed to parse enqueue response: {}", e),
        })?;

    Ok(response.run_id)
}

fn wait_for_job_completion(
    client: &Client,
    server_base: &str,
    run_id: &str,
) -> Result<ExecuteResponse, WdlError> {
    loop {
        let status = fetch_job_status(client, server_base, run_id)?;
        match status.state.as_str() {
            "succeeded" => {
                if let Some(JobResultStatus::Success { response }) = status.result {
                    return Ok(response);
                }
                return Err(WdlError::RuntimeError {
                    message: "Job succeeded but no result returned".to_string(),
                });
            }
            "failed" => {
                if let Some(JobResultStatus::Failure { message }) = status.result {
                    return Err(WdlError::RuntimeError {
                        message: format!("Job failed: {}", message),
                    });
                }
                return Err(WdlError::RuntimeError {
                    message: "Job failed without error message".to_string(),
                });
            }
            "pending" | "running" => {
                thread::sleep(QUEUE_POLL_INTERVAL);
            }
            other => {
                return Err(WdlError::RuntimeError {
                    message: format!("Unknown job state '{}'", other),
                });
            }
        }
    }
}

fn fetch_job_status(
    client: &Client,
    server_base: &str,
    run_id: &str,
) -> Result<JobStatusResponse, WdlError> {
    let url = format!("{}/api/v1/jobs/{}", server_base, run_id);
    let response = client
        .get(&url)
        .send()
        .map_err(|e| WdlError::RuntimeError {
            message: format!("Failed to contact server: {}", e),
        })?;

    let status = response.status();
    let body = response.text().map_err(|e| WdlError::RuntimeError {
        message: format!("Failed to read job status body: {}", e),
    })?;

    if status.is_success() {
        serde_json::from_str(&body).map_err(|e| WdlError::RuntimeError {
            message: format!("Failed to parse job status: {}", e),
        })
    } else if let Ok(err) = serde_json::from_str::<ApiErrorResponse>(&body) {
        Err(WdlError::RuntimeError {
            message: format!("Server error: {}", err.message),
        })
    } else {
        Err(WdlError::RuntimeError {
            message: format!("Server error (HTTP {}): {}", status, body),
        })
    }
}

fn load_config(path: &Path) -> Result<flowy::runtime::Config, WdlError> {
    let content = fs::read_to_string(path).map_err(|e| WdlError::RuntimeError {
        message: format!("Failed to read config file: {}", e),
    })?;

    let json: serde_json::Value =
        serde_json::from_str(&content).map_err(|e| WdlError::RuntimeError {
            message: format!("Invalid JSON in config file: {}", e),
        })?;

    let mut config = flowy::runtime::Config::default();

    if let serde_json::Value::Object(map) = json {
        // Parse container configuration
        if let Some(serde_json::Value::Object(container_map)) = map.get("container") {
            if let Some(serde_json::Value::Bool(enabled)) = container_map.get("enabled") {
                config.container.enabled = *enabled;
            }
            if let Some(serde_json::Value::String(backend)) = container_map.get("backend") {
                config.container.backend = match backend.as_str() {
                    "Docker" => flowy::runtime::ContainerBackend::Docker,
                    "Podman" => flowy::runtime::ContainerBackend::Podman,
                    "Singularity" => flowy::runtime::ContainerBackend::Singularity,
                    _ => flowy::runtime::ContainerBackend::None,
                };
            }
        }

        // Parse logging configuration
        if let Some(serde_json::Value::Object(logging_map)) = map.get("logging") {
            if let Some(serde_json::Value::String(level)) = logging_map.get("level") {
                config.debug = level == "debug";
            }
        }
    }

    Ok(config)
}

fn load_inputs(path: &Path, document: &Document) -> Result<Bindings<Value>, WdlError> {
    let content = fs::read_to_string(path).map_err(|e| WdlError::RuntimeError {
        message: format!("Failed to read input file: {}", e),
    })?;

    let json: serde_json::Value =
        serde_json::from_str(&content).map_err(|e| WdlError::Validation {
            message: format!("Invalid JSON in input file: {}", e),
            pos: flowy::SourcePosition::new(
                path.display().to_string(),
                path.display().to_string(),
                1,
                1,
                1,
                1,
            ),
            source_text: Some(content.clone()),
            declared_wdl_version: Some("1.0".to_string()),
        })?;

    bindings_from_json_for_document(json, document)
}

/// Load inputs specifically for a task
fn load_inputs_for_task(path: &Path, task: &Task) -> Result<Bindings<Value>, WdlError> {
    let content = fs::read_to_string(path).map_err(|e| WdlError::RuntimeError {
        message: format!("Failed to read input file: {}", e),
    })?;

    let json: serde_json::Value =
        serde_json::from_str(&content).map_err(|e| WdlError::Validation {
            message: format!("Invalid JSON in input file: {}", e),
            pos: flowy::SourcePosition::new(
                path.display().to_string(),
                path.display().to_string(),
                1,
                1,
                1,
                1,
            ),
            source_text: Some(content.clone()),
            declared_wdl_version: Some("1.0".to_string()),
        })?;

    bindings_from_json_for_task(json, task)
}
