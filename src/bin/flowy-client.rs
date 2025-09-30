use flowy::cli_config::{self, CliConfig};
use flowy::core::api::{ErrorResponse, ExecuteOptions, ExecuteRequest, ExecuteResponse};
use reqwest::blocking::Client;
use serde_json::Map;
use std::env;
use std::fs;
use std::path::PathBuf;
use std::process;

fn main() {
    if let Err(err) = run() {
        eprintln!("flowy-client error: {err}");
        process::exit(1);
    }
}

fn run() -> Result<(), String> {
    let args = parse_args();

    match args.command {
        Command::Run {
            wdl_file,
            input_file,
            work_dir,
            task,
            config_file,
            server,
            base_dir,
        } => execute_remote(
            wdl_file,
            input_file,
            work_dir,
            task,
            config_file,
            server,
            base_dir,
            args.debug,
        ),
    }
}

struct Args {
    command: Command,
    debug: bool,
}

enum Command {
    Run {
        wdl_file: PathBuf,
        input_file: Option<PathBuf>,
        work_dir: Option<PathBuf>,
        task: Option<String>,
        config_file: Option<PathBuf>,
        server: Option<String>,
        base_dir: Option<PathBuf>,
    },
}

fn execute_remote(
    wdl_file: PathBuf,
    input_file: Option<PathBuf>,
    work_dir: Option<PathBuf>,
    task: Option<String>,
    config_file: Option<PathBuf>,
    server: Option<String>,
    base_dir: Option<PathBuf>,
    cli_debug: bool,
) -> Result<(), String> {
    if let Some(path) = &work_dir {
        eprintln!(
            "Warning: --dir={} is ignored by flowy-client; remote execution uses the server working directory.",
            path.display()
        );
    }

    if let Some(path) = &config_file {
        eprintln!(
            "Warning: --config={} is ignored by flowy-client; provide remote settings via --server or the ~/.flowy TOML file.",
            path.display()
        );
    }

    let (mut config, config_path) = match cli_config::config_file_path() {
        Ok(path) => (cli_config::load_config(&path)?, Some(path)),
        Err(err) => {
            if server.is_none() {
                return Err(err);
            }
            (CliConfig::default(), None)
        }
    };

    let debug = cli_debug || config.debug.unwrap_or(false);

    if debug {
        if let Some(path) = &config_path {
            eprintln!(
                "[flowy-client] debug: loaded config from {}",
                path.display()
            );
        } else {
            eprintln!("[flowy-client] debug: no config file available; using CLI settings only");
        }
    }

    let server_url = match server {
        Some(url) => {
            if let Some(path) = config_path.as_ref() {
                config.server_url = Some(url.clone());
                if let Err(err) = cli_config::save_config(path, &config) {
                    if debug {
                        eprintln!(
                            "[flowy-client] debug: failed to persist server URL to {}: {}",
                            path.display(),
                            err
                        );
                    } else {
                        eprintln!("Warning: failed to update {}: {}", path.display(), err);
                    }
                } else if debug {
                    eprintln!(
                        "[flowy-client] debug: saved server URL to {}",
                        path.display()
                    );
                }
            } else if debug {
                eprintln!(
                    "[flowy-client] debug: config file unavailable; not persisting server URL"
                );
            }
            url
        }
        None => match config.server_url.clone() {
            Some(url) => url,
            None => {
                let hint = if let Some(path) = config_path.as_ref() {
                    format!("set --server or configure SERVER_URL in {}", path.display())
                } else {
                    "set --server or configure SERVER_URL in ~/.flowy".to_string()
                };
                return Err(format!("Server URL not specified; {}", hint));
            }
        },
    };

    if debug {
        eprintln!("[flowy-client] debug: using server {}", server_url);
        eprintln!("[flowy-client] debug: submitting {}", wdl_file.display());
        match &input_file {
            Some(path) => eprintln!("[flowy-client] debug: inputs {}", path.display()),
            None => eprintln!("[flowy-client] debug: no inputs provided"),
        }
        if let Some(task_name) = &task {
            eprintln!("[flowy-client] debug: overriding task with {}", task_name);
        }
    }

    let base_dir_path = if let Some(dir) = base_dir {
        if dir.is_absolute() {
            dir
        } else {
            std::env::current_dir()
                .map_err(|e| format!("Failed to get current directory: {}", e))?
                .join(dir)
        }
    } else {
        std::env::current_dir().map_err(|e| format!("Failed to get current directory: {}", e))?
    };

    if debug {
        eprintln!("[flowy-client] debug: base_dir {}", base_dir_path.display());
    }

    let wdl = fs::read_to_string(&wdl_file)
        .map_err(|e| format!("Failed to read WDL file {}: {}", wdl_file.display(), e))?;

    let inputs: serde_json::Value = match input_file {
        Some(ref path) => {
            let inputs_str = fs::read_to_string(path)
                .map_err(|e| format!("Failed to read inputs file {}: {}", path.display(), e))?;
            serde_json::from_str(&inputs_str)
                .map_err(|e| format!("Invalid JSON in inputs file: {}", e))?
        }
        None => serde_json::Value::Object(Map::new()),
    };

    if debug {
        eprintln!("[flowy-client] debug: WDL size {} bytes", wdl.len());
        if let Some(obj) = inputs.as_object() {
            eprintln!(
                "[flowy-client] debug: inputs keys = {:?}",
                obj.keys().collect::<Vec<_>>()
            );
        }
    }

    let request = ExecuteRequest {
        wdl,
        inputs,
        options: Some(ExecuteOptions {
            task,
            run_id: None,
            base_dir: Some(base_dir_path.to_string_lossy().to_string()),
        }),
    };

    if debug {
        if let Some(options) = &request.options {
            eprintln!(
                "[flowy-client] debug: request options = task={:?}, run_id={:?}, base_dir={:?}",
                options.task, options.run_id, options.base_dir
            );
        }
    }

    let client = Client::new();
    let url = format!("{}/api/v1/tasks", server_url.trim_end_matches('/'));

    if debug {
        eprintln!("[flowy-client] debug: POST {}", url);
    }

    let response = client
        .post(&url)
        .json(&request)
        .send()
        .map_err(|e| format!("Failed to contact server: {}", e))?;

    let status = response.status();
    if debug {
        eprintln!("[flowy-client] debug: HTTP status {}", status);
    }

    let body = response
        .text()
        .map_err(|e| format!("Failed to read server response body: {}", e))?;

    if debug {
        eprintln!("[flowy-client] debug: response body = {}", body);
    }

    if !status.is_success() {
        if let Ok(err) = serde_json::from_str::<ErrorResponse>(&body) {
            return Err(format!("Server error: {}", err.message));
        }
        return Err(format!("Server error (HTTP {}): {}", status, body));
    }

    let response: ExecuteResponse = serde_json::from_str(&body)
        .map_err(|e| format!("Failed to parse server response: {}", e))?;

    println!("status: {}", response.status);
    println!("duration_ms: {}", response.duration_ms);
    println!(
        "outputs: {}",
        serde_json::to_string_pretty(&response.outputs).unwrap()
    );
    if let Some(stdout) = response.stdout {
        println!("stdout:\n{}", stdout);
    }
    if let Some(stderr) = response.stderr {
        println!("stderr:\n{}", stderr);
    }

    if debug {
        eprintln!("[flowy-client] debug: execution finished successfully");
    }

    Ok(())
}

fn parse_args() -> Args {
    let raw_args: Vec<String> = env::args().collect();

    if raw_args.is_empty() {
        eprintln!("Internal error: argv is empty");
        process::exit(1);
    }

    let program = raw_args[0].clone();

    if raw_args.len() < 2 {
        print_help(&program);
        process::exit(1);
    }

    let mut debug = false;
    let mut filtered_args = Vec::new();
    for arg in &raw_args[1..] {
        if arg == "--debug" {
            debug = true;
        } else {
            filtered_args.push(arg.clone());
        }
    }

    if filtered_args.is_empty() {
        print_help(&program);
        process::exit(1);
    }

    let command = match filtered_args[0].as_str() {
        "run" => {
            if filtered_args.len() > 1 {
                parse_run_command(&program, &filtered_args[1..])
            } else {
                eprintln!("Error: WDL file required");
                process::exit(1);
            }
        }
        "submit" => {
            eprintln!("The 'submit' subcommand is deprecated; use 'run' instead.");
            if filtered_args.len() > 1 {
                parse_run_command(&program, &filtered_args[1..])
            } else {
                eprintln!("Error: WDL file required");
                process::exit(1);
            }
        }
        "-h" | "--help" => {
            print_help(&program);
            process::exit(0);
        }
        _ => parse_run_command(&program, &filtered_args),
    };

    Args { command, debug }
}

fn parse_run_command(program: &str, args: &[String]) -> Command {
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
            "-h" | "--help" => {
                print_help(program);
                process::exit(0);
            }
            arg if arg.starts_with('-') => {
                eprintln!("Error: Unknown option: {}", arg);
                process::exit(1);
            }
            arg => {
                if wdl_file.is_none() {
                    wdl_file = Some(PathBuf::from(arg));
                } else {
                    eprintln!("Error: Unexpected positional argument: {}", arg);
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
    eprintln!("flowy-client - Remote WDL executor");
    eprintln!();
    eprintln!("Usage:");
    eprintln!(
        "  {} run <wdl_file> [options]         Submit a WDL workflow or task to flowy-server",
        program
    );
    eprintln!();
    eprintln!("Options:");
    eprintln!("  -i, --input <file>    Input JSON file (defaults to empty object)");
    eprintln!("  -d, --dir <dir>       (Ignored) Provided for CLI compatibility");
    eprintln!("  -t, --task <name>     Run specific task instead of workflow");
    eprintln!("  -c, --config <file>   (Ignored) Provided for CLI compatibility");
    eprintln!("  -s, --server <url>    flowy-server base URL (saved to ~/.flowy)");
    eprintln!("      --basedir <dir>    Base directory for resolving relative File inputs (default: current dir)");
    eprintln!("  --debug               Enable verbose client logging");
    eprintln!("  -h, --help            Show this help message");
    eprintln!();
    eprintln!("Examples:");
    eprintln!("  {} run workflow.wdl -i inputs.json", program);
}
