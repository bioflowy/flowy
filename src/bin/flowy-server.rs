use axum::{routing::post, Json, Router};
use flowy::cli_config;
use flowy::core::{
    api::{ErrorResponse, ExecuteRequest, ExecuteResponse},
    executor,
    inputs::{bindings_from_json_for_document, bindings_from_json_for_task, set_input_base_dir},
    outputs::bindings_to_json_with_namespace,
};
use flowy::parser;
use flowy::runtime::Config;
use flowy::tree::{Document, Task};
use flowy::Bindings;
use std::env;
use std::net::SocketAddr;
use std::path::PathBuf;
use std::sync::OnceLock;
use tempfile::TempDir;
use tokio::task;
use url::Url;
use uuid::Uuid;

static DEBUG_ENABLED: OnceLock<bool> = OnceLock::new();

fn debug_enabled() -> bool {
    *DEBUG_ENABLED.get().unwrap_or(&false)
}

#[tokio::main]
async fn main() {
    let mut cli_debug = false;
    let mut show_help = false;

    for arg in env::args().skip(1) {
        match arg.as_str() {
            "--debug" => cli_debug = true,
            "-h" | "--help" => show_help = true,
            other => {
                eprintln!("Unknown option: {}", other);
                show_help = true;
            }
        }
    }

    if show_help {
        eprintln!("flowy-server - Remote execution API");
        eprintln!("Usage: flowy-server [--debug]");
        eprintln!("  --debug   Enable verbose server logging");
        return;
    }

    let mut debug = cli_debug;
    if !debug {
        if let Ok(path) = cli_config::config_file_path() {
            if let Ok(cfg) = cli_config::load_config(&path) {
                if cfg.debug.unwrap_or(false) {
                    debug = true;
                    eprintln!("[flowy-server] debug mode enabled via {}", path.display());
                }
            }
        }
    }

    if DEBUG_ENABLED.set(debug).is_err() {
        eprintln!("[flowy-server] debug flag already initialized");
    }

    if debug {
        eprintln!("[flowy-server] debug mode enabled");
    }

    let app = Router::new().route("/api/v1/tasks", post(handle_execute));

    let addr: SocketAddr = ([0, 0, 0, 0], 3030).into();
    println!("flowy-server listening on http://{}", addr);
    if debug_enabled() {
        eprintln!("[flowy-server] awaiting requests on 0.0.0.0:3030");
    }

    let listener = tokio::net::TcpListener::bind(addr)
        .await
        .expect("failed to bind to address");

    if let Err(err) = axum::serve(listener, app.into_make_service()).await {
        eprintln!("Server error: {}", err);
    }
}

async fn handle_execute(
    Json(req): Json<ExecuteRequest>,
) -> Result<Json<ExecuteResponse>, (axum::http::StatusCode, Json<ErrorResponse>)> {
    let debug = debug_enabled();
    if debug {
        let task = req
            .options
            .as_ref()
            .and_then(|opts| opts.task.as_deref())
            .unwrap_or("<workflow>");
        let input_summary = if let Some(map) = req.inputs.as_object() {
            format!("keys={:?}", map.keys().collect::<Vec<_>>())
        } else {
            "non-object inputs".to_string()
        };
        eprintln!(
            "[flowy-server] received request: task={}, wdl_bytes={}, {}",
            task,
            req.wdl.len(),
            input_summary
        );
    }

    let blocking_result = task::spawn_blocking(move || process_request(req)).await;

    match blocking_result {
        Ok(Ok(response)) => {
            if debug {
                eprintln!(
                    "[flowy-server] request succeeded: duration_ms={}",
                    response.duration_ms
                );
            }
            Ok(Json(response))
        }
        Ok(Err(err)) => {
            if debug {
                eprintln!("[flowy-server] request failed: {}", err);
            }
            Err((
                axum::http::StatusCode::BAD_REQUEST,
                Json(ErrorResponse {
                    status: "error".to_string(),
                    message: err.to_string(),
                }),
            ))
        }
        Err(join_err) => {
            if debug {
                eprintln!("[flowy-server] handler panicked: {}", join_err);
            }
            Err((
                axum::http::StatusCode::INTERNAL_SERVER_ERROR,
                Json(ErrorResponse {
                    status: "error".to_string(),
                    message: format!("Task execution panicked: {}", join_err),
                }),
            ))
        }
    }
}

fn process_request(req: ExecuteRequest) -> Result<ExecuteResponse, ServerError> {
    let version = detect_wdl_version(&req.wdl);
    let mut document =
        parser::parse_document(&req.wdl, &version).map_err(|e| ServerError::Wdl(e.to_string()))?;

    document
        .typecheck()
        .map_err(|e| ServerError::Wdl(e.to_string()))?;

    let options = req.options.unwrap_or_default();
    let run_id = options.run_id.unwrap_or_else(|| Uuid::new_v4().to_string());

    let config = Config::default();

    let base_dir_option = options.base_dir.as_ref().and_then(|s| {
        let trimmed = s.trim();
        if trimmed.is_empty() {
            None
        } else {
            Some(PathBuf::from(trimmed))
        }
    });

    let _base_dir_guard = set_input_base_dir(base_dir_option.clone());

    if debug_enabled() {
        eprintln!(
            "[flowy-server] processing request run_id={} (task={:?})",
            run_id, options.task
        );
        if let Some(dir) = base_dir_option.as_ref() {
            eprintln!("[flowy-server] base_dir = {}", dir.display());
        }
    }

    let temp_dir = TempDir::new().map_err(|e| ServerError::Io(e.to_string()))?;
    let work_dir = temp_dir.path();

    if let Some(task_name) = options.task {
        let task = find_task(&mut document, &task_name)?;
        let inputs = bindings_from_json_for_task(req.inputs.clone(), task)
            .map_err(|e| ServerError::Wdl(e.to_string()))?;
        run_task(task.clone(), inputs, config, &run_id, work_dir)
    } else if document.workflow.is_some() {
        let inputs = bindings_from_json_for_document(req.inputs.clone(), &document)
            .map_err(|e| ServerError::Wdl(e.to_string()))?;
        run_workflow(document, inputs, config, &run_id, work_dir)
    } else if document.tasks.len() == 1 {
        let task = document.tasks[0].clone();
        let inputs = bindings_from_json_for_task(req.inputs.clone(), &task)
            .map_err(|e| ServerError::Wdl(e.to_string()))?;
        run_task(task, inputs, config, &run_id, work_dir)
    } else {
        Err(ServerError::InvalidRequest(
            "Document lacks workflow; specify task name".to_string(),
        ))
    }
}

fn run_workflow(
    document: Document,
    inputs: Bindings<flowy::Value>,
    config: Config,
    run_id: &str,
    work_dir: &std::path::Path,
) -> Result<ExecuteResponse, ServerError> {
    let workflow_namespace = document
        .workflow
        .as_ref()
        .map(|workflow| workflow.name.clone());

    let result = executor::execute_workflow_document(document, inputs, config, run_id, work_dir)
        .map_err(|e| ServerError::Runtime(e.to_string()))?;

    let outputs = bindings_to_json_with_namespace(
        &result.outputs,
        workflow_namespace.as_deref(),
    )
    .map_err(|e| ServerError::Wdl(e.to_string()))?;

    if debug_enabled() {
        eprintln!(
            "[flowy-server] workflow completed run_id={} in {} ms",
            run_id,
            result.duration.as_millis()
        );
    }

    Ok(ExecuteResponse {
        status: "ok".to_string(),
        outputs,
        stdout: None,
        stderr: None,
        duration_ms: result.duration.as_millis(),
    })
}

fn run_task(
    task: Task,
    inputs: Bindings<flowy::Value>,
    config: Config,
    run_id: &str,
    work_dir: &std::path::Path,
) -> Result<ExecuteResponse, ServerError> {
    let namespace = task.name.clone();

    let result = executor::execute_task(task, inputs, config, run_id, work_dir)
        .map_err(|e| ServerError::Runtime(e.to_string()))?;

    let outputs = bindings_to_json_with_namespace(&result.outputs, Some(namespace.as_str()))
        .map_err(|e| ServerError::Wdl(e.to_string()))?;

    let stdout = read_file_from_url(&result.stdout)?;
    let stderr = read_file_from_url(&result.stderr)?;

    if debug_enabled() {
        eprintln!(
            "[flowy-server] task completed run_id={} in {} ms",
            run_id,
            result.duration.as_millis()
        );
    }

    Ok(ExecuteResponse {
        status: "ok".to_string(),
        outputs,
        stdout: Some(stdout),
        stderr: Some(stderr),
        duration_ms: result.duration.as_millis(),
    })
}

fn read_file_from_url(url: &Url) -> Result<String, ServerError> {
    let path = url
        .to_file_path()
        .map_err(|_| ServerError::Runtime(format!("Invalid file URL: {}", url)))?;
    std::fs::read_to_string(&path)
        .map_err(|e| ServerError::Io(format!("Failed to read {}: {}", path.display(), e)))
}

fn find_task<'a>(document: &'a mut Document, name: &str) -> Result<&'a mut Task, ServerError> {
    document
        .tasks
        .iter_mut()
        .find(|task| task.name == name)
        .ok_or_else(|| ServerError::InvalidRequest(format!("Task '{}' not found", name)))
}

fn detect_wdl_version(source: &str) -> String {
    for line in source.lines() {
        let trimmed = line.trim();
        if let Some(rest) = trimmed.strip_prefix("version") {
            return rest.trim().to_string();
        }
    }
    "1.0".to_string()
}

#[derive(thiserror::Error, Debug)]
enum ServerError {
    #[error("{0}")]
    InvalidRequest(String),
    #[error("WDL error: {0}")]
    Wdl(String),
    #[error("Runtime error: {0}")]
    Runtime(String),
    #[error("IO error: {0}")]
    Io(String),
}
