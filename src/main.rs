//! miniwdl-rust CLI
//!
//! Command-line interface for executing WDL workflows and tasks.

use miniwdl_rust::{
    parser, runtime, Bindings, RuntimeBuilder, Value, WdlError, Type,
};
use std::fs;
use std::path::{Path, PathBuf};
use std::process;
use std::time::Duration;

/// CLI arguments structure
struct Args {
    /// WDL file to execute
    wdl_file: PathBuf,
    /// Input JSON file (optional)
    input_file: Option<PathBuf>,
    /// Working directory (optional)
    work_dir: Option<PathBuf>,
    /// Task to run (if not running workflow)
    task: Option<String>,
    /// Enable debug output
    debug: bool,
}

fn main() {
    // Parse command-line arguments
    let args = parse_args();
    
    // Run the WDL file
    if let Err(e) = run(args) {
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
    
    let mut wdl_file = None;
    let mut input_file = None;
    let mut work_dir = None;
    let mut task = None;
    let mut debug = false;
    
    let mut i = 1;
    while i < args.len() {
        match args[i].as_str() {
            "-h" | "--help" => {
                print_help(&args[0]);
                process::exit(0);
            }
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
            "--debug" => {
                debug = true;
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
    
    Args {
        wdl_file,
        input_file,
        work_dir,
        task,
        debug,
    }
}

fn print_help(program: &str) {
    eprintln!("miniwdl-rust - WDL workflow executor");
    eprintln!();
    eprintln!("Usage: {} <wdl_file> [options]", program);
    eprintln!();
    eprintln!("Options:");
    eprintln!("  -i, --input <file>    Input JSON file");
    eprintln!("  -d, --dir <dir>       Working directory (default: temp)");
    eprintln!("  -t, --task <name>     Run specific task instead of workflow");
    eprintln!("  --debug               Enable debug output");
    eprintln!("  -h, --help            Show this help message");
}

fn run(args: Args) -> Result<(), WdlError> {
    // Read WDL file
    let wdl_content = fs::read_to_string(&args.wdl_file)
        .map_err(|e| WdlError::RuntimeError {
            message: format!("Failed to read WDL file: {}", e),
        })?;
    
    // Parse WDL document
    eprintln!("Parsing {}...", args.wdl_file.display());
    let document = parser::parse_document(&wdl_content, "1.0")?;
    
    // Load inputs if provided
    let inputs = if let Some(input_file) = args.input_file {
        eprintln!("Loading inputs from {}...", input_file.display());
        load_inputs(&input_file)?
    } else {
        Bindings::new()
    };
    
    // Set up working directory
    let work_dir = args.work_dir.unwrap_or_else(|| {
        std::env::temp_dir().join("miniwdl-rust")
    });
    fs::create_dir_all(&work_dir)
        .map_err(|e| WdlError::RuntimeError {
            message: format!("Failed to create working directory: {}", e),
        })?;
    
    eprintln!("Working directory: {}", work_dir.display());
    
    // Build runtime configuration
    let mut config = miniwdl_rust::runtime::Config::default();
    config.work_dir = work_dir.clone();
    if args.debug {
        config.debug = true;
    }
    
    // Generate run ID
    let run_id = format!("run_{}", chrono::Utc::now().timestamp());
    
    // Execute workflow or task
    let result = if let Some(task_name) = args.task {
        // Run specific task
        eprintln!("Running task '{}'...", task_name);
        
        // Find the task in the document
        let task = document.tasks.iter()
            .find(|t| t.name == task_name)
            .ok_or_else(|| WdlError::Validation {
                message: format!("Task '{}' not found in document", task_name),
                pos: document.pos.clone(),
                source_text: Some(String::new()),
                declared_wdl_version: Some("1.0".to_string()),
            })?;
        
        runtime::run_task(
            task.clone(),
            inputs,
            config,
            &run_id,
            &work_dir,
        ).map_err(|e| WdlError::RuntimeError {
            message: format!("Task execution failed: {}", e),
        })?
    } else {
        // Run workflow
        if let Some(workflow) = &document.workflow {
            eprintln!("Running workflow '{}'...", workflow.name);
        } else {
            eprintln!("No workflow found, running tasks...");
        }
        
        let workflow_result = runtime::run_document(
            document,
            inputs,
            config,
            &run_id,
            &work_dir,
        ).map_err(|e| WdlError::RuntimeError {
            message: format!("Workflow execution failed: {}", e),
        })?;
        
        // Convert WorkflowResult to TaskResult-like output
        runtime::task_context::TaskResult {
            outputs: workflow_result.outputs,
            stdout: String::new(),
            stderr: String::new(),
            exit_status: unsafe { std::mem::transmute(0u32) }, // Placeholder exit status
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

fn load_inputs(path: &Path) -> Result<Bindings<Value>, WdlError> {
    let content = fs::read_to_string(path)
        .map_err(|e| WdlError::RuntimeError {
            message: format!("Failed to read input file: {}", e),
        })?;
    
    let json: serde_json::Value = serde_json::from_str(&content)
        .map_err(|e| WdlError::Validation {
            message: format!("Invalid JSON in input file: {}", e),
            pos: miniwdl_rust::SourcePosition::new(
                path.display().to_string(),
                path.display().to_string(),
                1, 1, 1, 1,
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
        },
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
                        String::new(), String::new(), 0, 0, 0, 0,
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
            let values: Result<Vec<_>, _> = arr.into_iter()
                .map(json_to_value)
                .collect();
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
        Value::Null { .. } => Ok(serde_json::Value::Null),
        Value::Boolean { value, .. } => Ok(serde_json::Value::Bool(*value)),
        Value::Int { value, .. } => Ok(serde_json::Value::Number((*value).into())),
        Value::Float { value, .. } => {
            serde_json::Number::from_f64(*value)
                .map(serde_json::Value::Number)
                .ok_or_else(|| WdlError::Validation {
                    message: format!("Invalid float value: {}", value),
                    pos: miniwdl_rust::SourcePosition::new(
                        String::new(), String::new(), 0, 0, 0, 0,
                    ),
                    source_text: Some(String::new()),
                    declared_wdl_version: Some("1.0".to_string()),
                })
        }
        Value::String { value, .. } => Ok(serde_json::Value::String(value.clone())),
        Value::File { value, .. } => Ok(serde_json::Value::String(value.clone())),
        Value::Directory { value, .. } => Ok(serde_json::Value::String(value.clone())),
        Value::Array { values, .. } => {
            let arr: Result<Vec<_>, _> = values.iter()
                .map(value_to_json)
                .collect();
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