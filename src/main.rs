//! miniwdl-rust CLI
//!
//! Command-line interface for executing WDL workflows and tasks.

#![allow(clippy::result_large_err)]
#![allow(clippy::field_reassign_with_default)]
#![allow(clippy::missing_transmute_annotations)]
#![allow(clippy::unneeded_struct_pattern)]

use miniwdl_rust::{parser, runtime, Bindings, Type, Value, WdlError};
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
    /// Run specification tests
    SpecTests {
        /// Path to WDL specification file
        spec_file: PathBuf,
        /// Path to test data directory
        data_dir: PathBuf,
        /// Pattern to match test names (optional)
        pattern: Option<String>,
        /// Exact test name to run (optional)
        test_name: Option<String>,
        /// Maximum number of tests to run
        max_tests: Option<usize>,
        /// List all available tests
        list_tests: bool,
    },
}

fn main() {
    // Parse command-line arguments
    let args = parse_args();

    // Execute the command
    let result = match args.command {
        Command::Run { .. } => run_wdl(args),
        Command::SpecTests { .. } => run_spec_tests(args),
    };

    if let Err(e) = result {
        eprintln!("Error: {}", e);
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

    // Check if first argument is a command
    let command = match args[1].as_str() {
        "run" => parse_run_command(&args[2..]),
        "spec-tests" => parse_spec_tests_command(&args[2..]),
        "-h" | "--help" => {
            print_help(&args[0]);
            process::exit(0);
        }
        "--debug" => {
            debug = true;
            if args.len() < 3 {
                print_help(&args[0]);
                process::exit(1);
            }
            match args[2].as_str() {
                "run" => parse_run_command(&args[3..]),
                "spec-tests" => parse_spec_tests_command(&args[3..]),
                _ => {
                    // Assume it's a WDL file for backward compatibility
                    parse_run_command(&args[1..])
                }
            }
        }
        _ => {
            // Assume it's a WDL file for backward compatibility
            parse_run_command(&args[1..])
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

fn parse_spec_tests_command(args: &[String]) -> Command {
    if args.len() < 2 {
        eprintln!("Error: spec-tests requires <spec-file> <data-dir>");
        process::exit(1);
    }

    let spec_file = PathBuf::from(&args[0]);
    let data_dir = PathBuf::from(&args[1]);
    let mut pattern = None;
    let mut test_name = None;
    let mut max_tests = None;
    let mut list_tests = false;

    let mut i = 2;
    while i < args.len() {
        match args[i].as_str() {
            "-p" | "--pattern" => {
                i += 1;
                if i < args.len() {
                    pattern = Some(args[i].clone());
                } else {
                    eprintln!("Error: --pattern requires a pattern string");
                    process::exit(1);
                }
            }
            "-n" | "--name" => {
                i += 1;
                if i < args.len() {
                    test_name = Some(args[i].clone());
                } else {
                    eprintln!("Error: --name requires a test name");
                    process::exit(1);
                }
            }
            "-m" | "--max" => {
                i += 1;
                if i < args.len() {
                    max_tests = Some(args[i].parse().unwrap_or_else(|_| {
                        eprintln!("Error: --max requires a valid number");
                        process::exit(1);
                    }));
                } else {
                    eprintln!("Error: --max requires a number");
                    process::exit(1);
                }
            }
            "-l" | "--list" => {
                list_tests = true;
            }
            _ => {
                eprintln!("Error: Unknown option for spec-tests: {}", args[i]);
                process::exit(1);
            }
        }
        i += 1;
    }

    Command::SpecTests {
        spec_file,
        data_dir,
        pattern,
        test_name,
        max_tests,
        list_tests,
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
    eprintln!(
        "  {} spec-tests <spec_file> <data_dir>  Run specification tests",
        program
    );
    eprintln!();
    eprintln!("Run command options:");
    eprintln!("  -i, --input <file>    Input JSON file");
    eprintln!("  -d, --dir <dir>       Working directory (default: temp)");
    eprintln!("  -t, --task <name>     Run specific task instead of workflow");
    eprintln!("  -c, --config <file>   Configuration JSON file");
    eprintln!();
    eprintln!("Spec-tests command options:");
    eprintln!("  -p, --pattern <pattern>  Only run tests matching pattern");
    eprintln!("  -n, --name <name>        Run a specific test by exact name");
    eprintln!("  -m, --max <number>       Maximum number of tests to run");
    eprintln!("  -l, --list               List all available test names");
    eprintln!();
    eprintln!("Global options:");
    eprintln!("  --debug               Enable debug output");
    eprintln!("  -h, --help            Show this help message");
    eprintln!();
    eprintln!("Examples:");
    eprintln!("  {} run workflow.wdl -i inputs.json", program);
    eprintln!(
        "  {} spec-tests ./spec/wdl-1.2/SPEC.md ./spec/wdl-1.2/tests/data",
        program
    );
    eprintln!(
        "  {} spec-tests ./spec/wdl-1.2/SPEC.md ./spec/wdl-1.2/tests/data --list",
        program
    );
    eprintln!("  {} spec-tests ./spec/wdl-1.2/SPEC.md ./spec/wdl-1.2/tests/data --name spec_line_259_task_hello_task", program);
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
        _ => {
            return Err(WdlError::RuntimeError {
                message: "Invalid command for WDL execution".to_string(),
            })
        }
    };

    // Read WDL file
    let wdl_content = fs::read_to_string(&wdl_file).map_err(|e| WdlError::RuntimeError {
        message: format!("Failed to read WDL file: {}", e),
    })?;

    // Parse WDL document
    eprintln!("Parsing {}...", wdl_file.display());
    let document = parser::parse_document(&wdl_content, "1.2")?;

    // Load inputs if provided
    let inputs = if let Some(input_file) = input_file {
        eprintln!("Loading inputs from {}...", input_file.display());
        load_inputs(&input_file)?
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
    let result = if let Some(task_name) = task {
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

        runtime::run_task(task.clone(), inputs, config, &run_id, &work_dir).map_err(|e| {
            WdlError::RuntimeError {
                message: format!("Task execution failed: {}", e),
            }
        })?
    } else {
        // Run workflow
        if let Some(workflow) = &document.workflow {
            eprintln!("Running workflow '{}'...", workflow.name);
        } else {
            eprintln!("No workflow found, running tasks...");
        }

        let workflow_result = runtime::run_document(document, inputs, config, &run_id, &work_dir)
            .map_err(|e| WdlError::RuntimeError {
            message: format!("Workflow execution failed: {}", e),
        })?;

        // Convert WorkflowResult to TaskResult-like output
        runtime::task_context::TaskResult {
            outputs: workflow_result.outputs,
            stdout: url::Url::parse("file:///dev/null").unwrap(), // Placeholder stdout
            stderr: url::Url::parse("file:///dev/null").unwrap(), // Placeholder stderr
            exit_status: unsafe { std::mem::transmute(0u32) },    // Placeholder exit status
            duration: Duration::from_secs(0),
            work_dir: work_dir.clone(),
        }
    };

    // Print outputs as JSON
    eprintln!("\nExecution completed successfully!");
    eprintln!("Outputs:");

    let output_json = outputs_to_json(&result.outputs)?;
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

fn load_inputs(path: &Path) -> Result<Bindings<Value>, WdlError> {
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

    json_to_bindings(json)
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

fn outputs_to_json(outputs: &Bindings<Value>) -> Result<serde_json::Value, WdlError> {
    let mut map = serde_json::Map::new();

    for binding in outputs.iter() {
        let json_value = value_to_json(binding.value())?;
        map.insert(binding.name().to_string(), json_value);
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

fn run_spec_tests(args: Args) -> Result<(), WdlError> {
    let (spec_file, data_dir, pattern, test_name, max_tests, list_tests) = match args.command {
        Command::SpecTests {
            spec_file,
            data_dir,
            pattern,
            test_name,
            max_tests,
            list_tests,
        } => (
            spec_file, data_dir, pattern, test_name, max_tests, list_tests,
        ),
        _ => {
            return Err(WdlError::RuntimeError {
                message: "Invalid command for spec tests".to_string(),
            })
        }
    };

    // Create spec test configuration
    let mut config = runtime::SpecTestConfig::new();
    if args.debug {
        config = config.with_verbose(true);
    }
    if let Some(max) = max_tests {
        config = config.with_max_tests(max);
    }

    // Create spec test runner
    let runner = runtime::SpecTestRunner::with_config(config);

    // Handle different command options
    if list_tests {
        // List all available tests
        eprintln!("Listing all available tests...");
        eprintln!("Spec file: {}", spec_file.display());

        match runner.list_tests(&spec_file) {
            Ok(_) => Ok(()),
            Err(e) => {
                eprintln!("Failed to list tests: {}", e);
                Err(WdlError::RuntimeError {
                    message: format!("Failed to list tests: {}", e),
                })
            }
        }
    } else if let Some(name) = test_name {
        // Run a specific test by name
        eprintln!("Running specific test: {}", name);
        eprintln!("Spec file: {}", spec_file.display());
        eprintln!("Data directory: {}", data_dir.display());

        match runner.run_single_test(&spec_file, &data_dir, &name) {
            Ok(Some(_)) => {
                eprintln!("\nTest completed successfully!");
                Ok(())
            }
            Ok(None) => {
                eprintln!("Test '{}' not found", name);
                Err(WdlError::RuntimeError {
                    message: format!("Test '{}' not found", name),
                })
            }
            Err(e) => {
                eprintln!("Test execution failed: {}", e);
                Err(WdlError::RuntimeError {
                    message: format!("Test execution failed: {}", e),
                })
            }
        }
    } else {
        // Run tests (all or by pattern)
        eprintln!("Running WDL specification tests...");
        eprintln!("Spec file: {}", spec_file.display());
        eprintln!("Data directory: {}", data_dir.display());

        let results = if let Some(pattern) = pattern {
            eprintln!("Running tests matching pattern: {}", pattern);
            runner.run_tests_matching(&spec_file, &data_dir, &pattern)
        } else {
            runner.run_all_tests(&spec_file, &data_dir)
        };

        match results {
            Ok(_) => {
                eprintln!("\nSpec tests completed successfully!");
                Ok(())
            }
            Err(e) => {
                eprintln!("Spec tests failed: {}", e);
                Err(WdlError::RuntimeError {
                    message: format!("Spec tests failed: {}", e),
                })
            }
        }
    }
}
