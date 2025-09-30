use axum::{
    extract::Path,
    http::StatusCode,
    response::{IntoResponse, Response},
    routing::{get, post},
    Json, Router,
};
use flowy::cli_config;
use flowy::core::{
    api::{ErrorResponse, ExecuteRequest, ExecuteResponse},
    executor::{self, ExecuteJobError},
};
use once_cell::sync::Lazy;
use serde::{Deserialize, Serialize};
use std::collections::{HashMap, VecDeque};
use std::env;
use std::net::SocketAddr;
use std::sync::OnceLock;
use std::time::Instant;
use tokio::sync::Mutex;
use tokio::task;
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

    let app = Router::new()
        .route("/api/v1/tasks", post(handle_execute))
        .route("/api/v1/jobs", post(handle_enqueue_job))
        .route("/api/v1/jobs/claim", post(handle_claim_job))
        .route("/api/v1/jobs/:run_id", get(handle_get_job_status))
        .route("/api/v1/jobs/:run_id/heartbeat", post(handle_heartbeat))
        .route("/api/v1/jobs/:run_id/complete", post(handle_complete))
        .route("/api/v1/jobs/:run_id/failed", post(handle_failed));

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
) -> Result<Json<ExecuteResponse>, (StatusCode, Json<ErrorResponse>)> {
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
                StatusCode::BAD_REQUEST,
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
                StatusCode::INTERNAL_SERVER_ERROR,
                Json(ErrorResponse {
                    status: "error".to_string(),
                    message: format!("Task execution panicked: {}", join_err),
                }),
            ))
        }
    }
}

fn process_request(req: ExecuteRequest) -> Result<ExecuteResponse, ServerError> {
    executor::execute_request(req).map_err(ServerError::from)
}

#[derive(Debug, Clone, Serialize, Deserialize)]
struct JobPayload {
    run_id: String,
    request: ExecuteRequest,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
struct EnqueueJobResponse {
    run_id: String,
}

#[derive(Debug, Deserialize)]
struct ClaimJobRequest {
    worker_id: String,
}

#[derive(Debug, Deserialize)]
struct HeartbeatRequest {
    worker_id: String,
}

#[derive(Debug, Deserialize)]
struct CompleteRequest {
    worker_id: String,
    response: ExecuteResponse,
}

#[derive(Debug, Deserialize)]
struct FailedRequest {
    worker_id: String,
    message: String,
}

static JOB_STORE: Lazy<Mutex<JobStore>> = Lazy::new(|| Mutex::new(JobStore::new()));

#[derive(Debug)]
struct JobStore {
    pending: VecDeque<String>,
    records: HashMap<String, JobRecord>,
}

#[allow(dead_code)]
#[derive(Debug, Clone)]
struct JobRecord {
    run_id: String,
    request: ExecuteRequest,
    state: JobState,
    worker_id: Option<String>,
    last_heartbeat: Option<Instant>,
    result: Option<JobResult>,
    created_at: Instant,
    updated_at: Instant,
}

#[derive(Debug, Clone)]
enum JobState {
    Pending,
    Running,
    Succeeded,
    Failed,
}

#[allow(dead_code)]
#[derive(Debug, Clone)]
enum JobResult {
    Success(ExecuteResponse),
    Failure { message: String },
}

#[derive(Debug, Clone, Serialize)]
#[serde(tag = "type", rename_all = "lowercase")]
enum JobResultView {
    Success { response: ExecuteResponse },
    Failure { message: String },
}

#[derive(Debug, Clone, Serialize)]
struct JobStatusResponse {
    run_id: String,
    state: String,
    worker_id: Option<String>,
    result: Option<JobResultView>,
}

impl JobStore {
    fn new() -> Self {
        Self {
            pending: VecDeque::new(),
            records: HashMap::new(),
        }
    }

    fn enqueue(&mut self, run_id: String, request: ExecuteRequest) {
        let now = Instant::now();
        let record = JobRecord {
            run_id: run_id.clone(),
            request,
            state: JobState::Pending,
            worker_id: None,
            last_heartbeat: None,
            result: None,
            created_at: now,
            updated_at: now,
        };
        self.pending.push_back(run_id.clone());
        self.records.insert(run_id, record);
    }

    fn claim(&mut self, worker_id: &str) -> Option<JobPayload> {
        while let Some(run_id) = self.pending.pop_front() {
            if let Some(record) = self.records.get_mut(&run_id) {
                if matches!(record.state, JobState::Pending) {
                    record.state = JobState::Running;
                    record.worker_id = Some(worker_id.to_string());
                    record.last_heartbeat = Some(Instant::now());
                    record.updated_at = Instant::now();
                    return Some(JobPayload {
                        run_id,
                        request: record.request.clone(),
                    });
                }
            }
        }
        None
    }

    fn heartbeat(&mut self, run_id: &str, worker_id: &str) -> Result<(), JobError> {
        let record = self.records.get_mut(run_id).ok_or(JobError::NotFound)?;
        match record.state {
            JobState::Running => {
                if record.worker_id.as_deref() != Some(worker_id) {
                    return Err(JobError::NotOwner);
                }
                record.last_heartbeat = Some(Instant::now());
                record.updated_at = Instant::now();
                Ok(())
            }
            _ => Err(JobError::InvalidState),
        }
    }

    fn complete(
        &mut self,
        run_id: &str,
        worker_id: &str,
        response: ExecuteResponse,
    ) -> Result<(), JobError> {
        let record = self.records.get_mut(run_id).ok_or(JobError::NotFound)?;
        if record.worker_id.as_deref() != Some(worker_id) {
            return Err(JobError::NotOwner);
        }
        record.state = JobState::Succeeded;
        record.result = Some(JobResult::Success(response));
        record.updated_at = Instant::now();
        Ok(())
    }

    fn fail(&mut self, run_id: &str, worker_id: &str, message: String) -> Result<(), JobError> {
        let record = self.records.get_mut(run_id).ok_or(JobError::NotFound)?;
        if record.worker_id.as_deref() != Some(worker_id) {
            return Err(JobError::NotOwner);
        }
        record.state = JobState::Failed;
        record.result = Some(JobResult::Failure { message });
        record.updated_at = Instant::now();
        Ok(())
    }

    fn status(&self, run_id: &str) -> Option<JobStatusResponse> {
        self.records.get(run_id).map(|record| JobStatusResponse {
            run_id: record.run_id.clone(),
            state: record.state.as_str().to_string(),
            worker_id: record.worker_id.clone(),
            result: record.result.as_ref().map(|res| res.to_view()),
        })
    }
}

#[derive(Debug)]
enum JobError {
    NotFound,
    NotOwner,
    InvalidState,
}

impl JobError {
    fn status_code(&self) -> StatusCode {
        match self {
            JobError::NotFound => StatusCode::NOT_FOUND,
            JobError::NotOwner => StatusCode::FORBIDDEN,
            JobError::InvalidState => StatusCode::CONFLICT,
        }
    }
}

impl JobResult {
    fn to_view(&self) -> JobResultView {
        match self {
            JobResult::Success(response) => JobResultView::Success {
                response: response.clone(),
            },
            JobResult::Failure { message } => JobResultView::Failure {
                message: message.clone(),
            },
        }
    }
}

impl JobState {
    fn as_str(&self) -> &'static str {
        match self {
            JobState::Pending => "pending",
            JobState::Running => "running",
            JobState::Succeeded => "succeeded",
            JobState::Failed => "failed",
        }
    }
}

async fn handle_enqueue_job(
    Json(mut request): Json<ExecuteRequest>,
) -> Result<Json<EnqueueJobResponse>, StatusCode> {
    let mut options = request.options.unwrap_or_default();
    let run_id = options
        .run_id
        .clone()
        .unwrap_or_else(|| Uuid::new_v4().to_string());
    options.run_id = Some(run_id.clone());
    request.options = Some(options);

    let mut store = JOB_STORE.lock().await;
    store.enqueue(run_id.clone(), request);

    Ok(Json(EnqueueJobResponse { run_id }))
}

async fn handle_claim_job(Json(payload): Json<ClaimJobRequest>) -> Result<Response, StatusCode> {
    let mut store = JOB_STORE.lock().await;
    if let Some(job) = store.claim(&payload.worker_id) {
        Ok((StatusCode::OK, Json(job)).into_response())
    } else {
        Ok(StatusCode::NO_CONTENT.into_response())
    }
}

async fn handle_get_job_status(
    Path(run_id): Path<String>,
) -> Result<Json<JobStatusResponse>, StatusCode> {
    let store = JOB_STORE.lock().await;
    store.status(&run_id).map(Json).ok_or(StatusCode::NOT_FOUND)
}

async fn handle_heartbeat(
    Path(run_id): Path<String>,
    Json(payload): Json<HeartbeatRequest>,
) -> Result<StatusCode, StatusCode> {
    let mut store = JOB_STORE.lock().await;
    store
        .heartbeat(&run_id, &payload.worker_id)
        .map_err(|err| err.status_code())?;
    Ok(StatusCode::NO_CONTENT)
}

async fn handle_complete(
    Path(run_id): Path<String>,
    Json(payload): Json<CompleteRequest>,
) -> Result<StatusCode, StatusCode> {
    let mut store = JOB_STORE.lock().await;
    store
        .complete(&run_id, &payload.worker_id, payload.response)
        .map_err(|err| err.status_code())?;
    Ok(StatusCode::NO_CONTENT)
}

async fn handle_failed(
    Path(run_id): Path<String>,
    Json(payload): Json<FailedRequest>,
) -> Result<StatusCode, StatusCode> {
    let mut store = JOB_STORE.lock().await;
    store
        .fail(&run_id, &payload.worker_id, payload.message)
        .map_err(|err| err.status_code())?;
    Ok(StatusCode::NO_CONTENT)
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

impl From<ExecuteJobError> for ServerError {
    fn from(value: ExecuteJobError) -> Self {
        match value {
            ExecuteJobError::InvalidRequest(msg) => ServerError::InvalidRequest(msg),
            ExecuteJobError::Wdl(msg) => ServerError::Wdl(msg),
            ExecuteJobError::Runtime(msg) => ServerError::Runtime(msg),
            ExecuteJobError::Io(msg) => ServerError::Io(msg),
        }
    }
}
