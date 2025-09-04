//! miniwdl-rust CLI
//!
//! Command-line interface for executing WDL workflows and tasks.

#![allow(clippy::result_large_err)]
#![allow(clippy::field_reassign_with_default)]
#![allow(clippy::missing_transmute_annotations)]
#![allow(clippy::unneeded_struct_pattern)]

use miniwdl_rust::{
    load, runtime,
    tree::{Document, Task, Workflow},
    Bindings, SourcePosition, Type, Value, WdlError,
};
use std::fs;
use std::path::{Path, PathBuf};
use std::process;
use std::time::Duration;

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
            if let Some(error_line) = lines.get((pos.line - 1) as usize) {
                eprintln!("    {}", error_line);

                // Create a caret pointer to show exact position
                let pointer = " ".repeat((pos.column - 1) as usize) + "^";
                eprintln!("    {}", pointer);
            }
        }
    };

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
    }
}

fn print_help(program: &str) {
    eprintln!("miniwdl-rust - WDL workflow executor");
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
    eprintln!();
    eprintln!("Global options:");
    eprintln!("  --debug               Enable debug output");
    eprintln!("  -h, --help            Show this help message");
    eprintln!();
    eprintln!("Examples:");
    eprintln!("  {} run workflow.wdl -i inputs.json", program);
}

fn run_wdl(args: Args) -> Result<(), WdlError> {
    let (wdl_file, input_file, work_dir, task, config_file) = match args.command {
        Command::Run {
            wdl_file,
            input_file,
            work_dir,
            task,
            config_file,
        } => (wdl_file, input_file, work_dir, task, config_file),
    };

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

    // Load inputs if provided (with type information from document)
    let inputs = if let Some(input_file) = input_file {
        eprintln!("Loading inputs from {}...", input_file.display());
        load_inputs(&input_file, &document)?
    } else {
        Bindings::new()
    };

    // Set up working directory
    let work_dir = work_dir.unwrap_or_else(|| std::env::temp_dir().join("miniwdl-rust"));
    fs::create_dir_all(&work_dir).map_err(|e| WdlError::RuntimeError {
        message: format!("Failed to create working directory: {}", e),
    })?;

    eprintln!("Working directory: {}", work_dir.display());

    // Build runtime configuration
    let mut config = if let Some(config_file) = config_file {
        eprintln!("Loading config from {}...", config_file.display());
        load_config(&config_file)?
    } else {
        miniwdl_rust::runtime::Config::default()
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

    let output_json = outputs_to_json_with_namespace(&result.outputs, workflow_name.as_deref())?;
    println!("{}", serde_json::to_string_pretty(&output_json).unwrap());

    Ok(())
}

fn load_config(path: &Path) -> Result<miniwdl_rust::runtime::Config, WdlError> {
    let content = fs::read_to_string(path).map_err(|e| WdlError::RuntimeError {
        message: format!("Failed to read config file: {}", e),
    })?;

    let json: serde_json::Value =
        serde_json::from_str(&content).map_err(|e| WdlError::RuntimeError {
            message: format!("Invalid JSON in config file: {}", e),
        })?;

    let mut config = miniwdl_rust::runtime::Config::default();

    if let serde_json::Value::Object(map) = json {
        // Parse container configuration
        if let Some(serde_json::Value::Object(container_map)) = map.get("container") {
            if let Some(serde_json::Value::Bool(enabled)) = container_map.get("enabled") {
                config.container.enabled = *enabled;
            }
            if let Some(serde_json::Value::String(backend)) = container_map.get("backend") {
                config.container.backend = match backend.as_str() {
                    "Docker" => miniwdl_rust::runtime::ContainerBackend::Docker,
                    "Podman" => miniwdl_rust::runtime::ContainerBackend::Podman,
                    "Singularity" => miniwdl_rust::runtime::ContainerBackend::Singularity,
                    _ => miniwdl_rust::runtime::ContainerBackend::None,
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
            pos: miniwdl_rust::SourcePosition::new(
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

    // Get workflow information if available
    if let Some(ref workflow) = document.workflow {
        json_to_bindings_with_types(json, workflow)
    } else {
        // If no workflow, try to find task inputs
        if document.tasks.len() == 1 {
            let task = &document.tasks[0];
            json_to_bindings_with_task_types(json, task)
        } else {
            // Fallback to untyped conversion
            json_to_bindings(json)
        }
    }
}

/// Resolve a file path from a string, handling relative paths
fn resolve_file_path(path: &str) -> Result<PathBuf, WdlError> {
    let path_buf = PathBuf::from(path);

    let resolved = if path_buf.is_absolute() {
        path_buf
    } else {
        // Resolve relative to current directory
        std::env::current_dir()
            .map_err(|e| WdlError::RuntimeError {
                message: format!("Failed to get current directory: {}", e),
            })?
            .join(&path_buf)
    };

    // Check if file exists
    if !resolved.exists() {
        return Err(WdlError::RuntimeError {
            message: format!("Input file not found: {}", path),
        });
    }

    // Get canonical path
    resolved.canonicalize().map_err(|e| WdlError::RuntimeError {
        message: format!("Failed to resolve file path '{}': {}", path, e),
    })
}

/// Convert JSON to bindings using workflow type information (miniwdl-style)
fn json_to_bindings_with_types(
    json: serde_json::Value,
    workflow: &Workflow,
) -> Result<Bindings<Value>, WdlError> {
    let mut bindings = Bindings::new();

    if let serde_json::Value::Object(map) = json {
        for (key, json_value) in map {
            // Remove workflow name prefix if present
            let input_name = if key.starts_with(&format!("{}.", workflow.name)) {
                key.strip_prefix(&format!("{}.", workflow.name))
                    .unwrap()
                    .to_string()
            } else {
                key.clone()
            };

            // Find input type from workflow declarations
            let input_type = workflow
                .inputs
                .iter()
                .find(|decl| decl.name == input_name)
                .map(|decl| &decl.decl_type);

            // Convert JSON to WDL value using type information
            let wdl_value = if let Some(ty) = input_type {
                json_to_value_typed(json_value, ty)?
            } else {
                // Fallback to untyped conversion for unknown inputs
                json_to_value(json_value)?
            };

            bindings = bindings.bind(input_name, wdl_value, None);
        }
    }

    Ok(bindings)
}

/// Convert JSON to bindings using task type information
fn json_to_bindings_with_task_types(
    json: serde_json::Value,
    task: &Task,
) -> Result<Bindings<Value>, WdlError> {
    let mut bindings = Bindings::new();

    if let serde_json::Value::Object(map) = json {
        for (key, json_value) in map {
            // Remove task name prefix if present
            let input_name = if key.starts_with(&format!("{}.", task.name)) {
                key.strip_prefix(&format!("{}.", task.name))
                    .unwrap()
                    .to_string()
            } else {
                key.clone()
            };

            // Find input type from task declarations
            let input_type = task
                .inputs
                .iter()
                .find(|decl| decl.name == input_name)
                .map(|decl| &decl.decl_type);

            // Convert JSON to WDL value using type information
            let wdl_value = if let Some(ty) = input_type {
                json_to_value_typed(json_value, ty)?
            } else {
                json_to_value(json_value)?
            };

            bindings = bindings.bind(input_name, wdl_value, None);
        }
    }

    Ok(bindings)
}

/// Convert JSON to WDL value using type information (like miniwdl's from_json)
fn json_to_value_typed(json: serde_json::Value, wdl_type: &Type) -> Result<Value, WdlError> {
    match (json, wdl_type) {
        // File type: convert string to File value with resolved path
        (serde_json::Value::String(s), Type::File { optional }) => {
            let resolved_path = resolve_file_path(&s)?;
            Ok(Value::File {
                value: resolved_path.to_string_lossy().to_string(),
                wdl_type: Type::File {
                    optional: *optional,
                },
            })
        }
        // String type: keep as string
        (serde_json::Value::String(s), Type::String { optional }) => Ok(Value::String {
            value: s,
            wdl_type: Type::String {
                optional: *optional,
            },
        }),
        // Boolean type
        (serde_json::Value::Bool(b), Type::Boolean { optional }) => Ok(Value::Boolean {
            value: b,
            wdl_type: Type::Boolean {
                optional: *optional,
            },
        }),
        // Integer type
        (serde_json::Value::Number(n), Type::Int { optional }) => {
            if let Some(i) = n.as_i64() {
                Ok(Value::Int {
                    value: i,
                    wdl_type: Type::Int {
                        optional: *optional,
                    },
                })
            } else {
                Err(WdlError::RuntimeError {
                    message: format!("Cannot convert {} to Int", n),
                })
            }
        }
        // Float type
        (serde_json::Value::Number(n), Type::Float { optional }) => {
            if let Some(f) = n.as_f64() {
                Ok(Value::Float {
                    value: f,
                    wdl_type: Type::Float {
                        optional: *optional,
                    },
                })
            } else {
                Err(WdlError::RuntimeError {
                    message: format!("Cannot convert {} to Float", n),
                })
            }
        }
        // Array type: recursively convert elements
        (
            serde_json::Value::Array(arr),
            Type::Array {
                item_type,
                optional,
                nonempty,
            },
        ) => {
            let values: Result<Vec<_>, _> = arr
                .into_iter()
                .map(|v| json_to_value_typed(v, item_type))
                .collect();
            Ok(Value::Array {
                values: values?,
                wdl_type: Type::Array {
                    item_type: item_type.clone(),
                    optional: *optional,
                    nonempty: *nonempty,
                },
            })
        }
        // Fallback to untyped conversion for other cases
        (json_val, _) => json_to_value(json_val),
    }
}

fn json_to_bindings(json: serde_json::Value) -> Result<Bindings<Value>, WdlError> {
    let mut bindings = Bindings::new();

    if let serde_json::Value::Object(map) = json {
        for (key, value) in map {
            let wdl_value = json_to_value(value)?;
            bindings = bindings.bind(key, wdl_value, None);
        }
    }

    Ok(bindings)
}

fn json_to_value(json: serde_json::Value) -> Result<Value, WdlError> {
    match json {
        serde_json::Value::Null => {
            // Create a null string value as placeholder
            Ok(Value::String {
                value: String::new(),
                wdl_type: Type::String { optional: true },
            })
        }
        serde_json::Value::Bool(b) => Ok(Value::Boolean {
            value: b,
            wdl_type: Type::Boolean { optional: false },
        }),
        serde_json::Value::Number(n) => {
            if let Some(i) = n.as_i64() {
                Ok(Value::Int {
                    value: i,
                    wdl_type: Type::Int { optional: false },
                })
            } else if let Some(f) = n.as_f64() {
                Ok(Value::Float {
                    value: f,
                    wdl_type: Type::Float { optional: false },
                })
            } else {
                Err(WdlError::Validation {
                    message: format!("Invalid number: {}", n),
                    pos: miniwdl_rust::SourcePosition::new(
                        String::new(),
                        String::new(),
                        0,
                        0,
                        0,
                        0,
                    ),
                    source_text: Some(String::new()),
                    declared_wdl_version: Some("1.0".to_string()),
                })
            }
        }
        serde_json::Value::String(s) => Ok(Value::String {
            value: s,
            wdl_type: Type::String { optional: false },
        }),
        serde_json::Value::Array(arr) => {
            let values: Result<Vec<_>, _> = arr.into_iter().map(json_to_value).collect();
            Ok(Value::Array {
                values: values?,
                wdl_type: Type::Array {
                    item_type: Box::new(Type::String { optional: false }),
                    optional: false,
                    nonempty: false,
                },
            })
        }
        serde_json::Value::Object(map) => {
            let mut struct_map = std::collections::HashMap::new();
            for (key, value) in map {
                struct_map.insert(key, json_to_value(value)?);
            }
            Ok(Value::Struct {
                members: struct_map,
                extra_keys: std::collections::HashSet::new(),
                wdl_type: Type::String { optional: false }, // Placeholder type
            })
        }
    }
}

fn outputs_to_json_with_namespace(
    outputs: &Bindings<Value>,
    namespace: Option<&str>,
) -> Result<serde_json::Value, WdlError> {
    let mut map = serde_json::Map::new();

    // Prepare namespace prefix (like miniwdl's values_to_json)
    let namespace_prefix = if let Some(ns) = namespace {
        if ns.is_empty() {
            String::new()
        } else if ns.ends_with('.') {
            ns.to_string()
        } else {
            format!("{}.", ns)
        }
    } else {
        String::new()
    };

    for binding in outputs.iter() {
        let json_value = value_to_json(binding.value())?;
        // Add namespace prefix unless the binding name starts with "_" (following miniwdl logic)
        let key = if !binding.name().starts_with('_') && !namespace_prefix.is_empty() {
            format!("{}{}", namespace_prefix, binding.name())
        } else {
            binding.name().to_string()
        };
        map.insert(key, json_value);
    }

    Ok(serde_json::Value::Object(map))
}

fn value_to_json(value: &Value) -> Result<serde_json::Value, WdlError> {
    match value {
        Value::Null => Ok(serde_json::Value::Null),
        Value::Boolean { value, .. } => Ok(serde_json::Value::Bool(*value)),
        Value::Int { value, .. } => Ok(serde_json::Value::Number((*value).into())),
        Value::Float { value, .. } => serde_json::Number::from_f64(*value)
            .map(serde_json::Value::Number)
            .ok_or_else(|| WdlError::Validation {
                message: format!("Invalid float value: {}", value),
                pos: miniwdl_rust::SourcePosition::new(String::new(), String::new(), 0, 0, 0, 0),
                source_text: Some(String::new()),
                declared_wdl_version: Some("1.0".to_string()),
            }),
        Value::String { value, .. } => Ok(serde_json::Value::String(value.clone())),
        Value::File { value, .. } => Ok(serde_json::Value::String(value.clone())),
        Value::Directory { value, .. } => Ok(serde_json::Value::String(value.clone())),
        Value::Array { values, .. } => {
            let arr: Result<Vec<_>, _> = values.iter().map(value_to_json).collect();
            Ok(serde_json::Value::Array(arr?))
        }
        Value::Pair { left, right, .. } => {
            let mut map = serde_json::Map::new();
            map.insert("left".to_string(), value_to_json(left)?);
            map.insert("right".to_string(), value_to_json(right)?);
            Ok(serde_json::Value::Object(map))
        }
        Value::Map { pairs, .. } => {
            let mut map = serde_json::Map::new();
            for (k, v) in pairs {
                let key_str = match k {
                    Value::String { value, .. } => value.clone(),
                    _ => format!("{:?}", k),
                };
                map.insert(key_str, value_to_json(v)?);
            }
            Ok(serde_json::Value::Object(map))
        }
        Value::Struct { members, .. } => {
            let mut map = serde_json::Map::new();
            for (k, v) in members {
                map.insert(k.clone(), value_to_json(v)?);
            }
            Ok(serde_json::Value::Object(map))
        }
    }
}
