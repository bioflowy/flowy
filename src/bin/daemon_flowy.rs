use std::sync::Arc;
use std::time::Duration;

use flowy::cli_config::{self, CliConfig};
use flowy::core::api::{ExecuteRequest, ExecuteResponse};
use flowy::core::executor::{self, ExecuteJobError};
use reqwest::StatusCode;
use serde::Deserialize;
use tokio::sync::{oneshot, Semaphore};
use tokio::time::{interval, sleep};

#[derive(Debug, Clone)]
struct WorkerConfig {
    server_url: String,
    worker_id: String,
    poll_interval: Duration,
    heartbeat_interval: Duration,
    max_parallel_jobs: usize,
    debug: bool,
}

impl WorkerConfig {
    fn from_args() -> Result<Self, String> {
        let args: Vec<String> = std::env::args().collect();
        let mut server = None;
        let mut worker_id = None;
        let mut poll_secs = None;
        let mut heartbeat_secs = None;
        let mut max_jobs = None;
        let mut debug_flag = false;

        let mut i = 1;
        while i < args.len() {
            match args[i].as_str() {
                "--server" => {
                    i += 1;
                    if i >= args.len() {
                        return Err("--server requires a URL".to_string());
                    }
                    server = Some(args[i].clone());
                }
                "--worker-id" => {
                    i += 1;
                    if i >= args.len() {
                        return Err("--worker-id requires a value".to_string());
                    }
                    worker_id = Some(args[i].clone());
                }
                "--poll" => {
                    i += 1;
                    if i >= args.len() {
                        return Err("--poll requires seconds".to_string());
                    }
                    poll_secs = Some(
                        args[i]
                            .parse::<u64>()
                            .map_err(|e| format!("Invalid poll seconds '{}': {}", args[i], e))?,
                    );
                }
                "--heartbeat" => {
                    i += 1;
                    if i >= args.len() {
                        return Err("--heartbeat requires seconds".to_string());
                    }
                    heartbeat_secs =
                        Some(args[i].parse::<u64>().map_err(|e| {
                            format!("Invalid heartbeat seconds '{}': {}", args[i], e)
                        })?);
                }
                "--max-jobs" => {
                    i += 1;
                    if i >= args.len() {
                        return Err("--max-jobs requires a number".to_string());
                    }
                    max_jobs = Some(
                        args[i]
                            .parse::<usize>()
                            .map_err(|e| format!("Invalid max-jobs '{}': {}", args[i], e))?,
                    );
                }
                "--debug" => debug_flag = true,
                "-h" | "--help" => {
                    print_help(&args[0]);
                    std::process::exit(0);
                }
                other => {
                    return Err(format!("Unknown argument: {}", other));
                }
            }
            i += 1;
        }

        let config_file = cli_config::config_file_path().ok();
        let config_from_file = config_file
            .as_ref()
            .and_then(|path| cli_config::load_config(path).ok())
            .unwrap_or_else(CliConfig::default);

        let server_url = server
            .or_else(|| config_from_file.server_url.clone())
            .ok_or_else(|| "Server URL must be provided via --server or ~/.flowy".to_string())?;

        let worker_id = worker_id
            .or_else(|| std::env::var("FLOWY_WORKER_ID").ok())
            .or_else(|| config_from_file.worker_id.clone())
            .unwrap_or_else(|| {
                whoami::fallible::hostname().unwrap_or_else(|_| "flowy-worker".to_string())
            });

        let poll_interval =
            Duration::from_secs(poll_secs.or(config_from_file.poll_secs).unwrap_or(10));
        let heartbeat_interval = Duration::from_secs(
            heartbeat_secs
                .or(config_from_file.heartbeat_secs)
                .unwrap_or(120),
        );
        let max_parallel_jobs = max_jobs.or(config_from_file.max_jobs).unwrap_or(1);
        let debug = debug_flag || config_from_file.debug.unwrap_or(false);

        Ok(Self {
            server_url,
            worker_id,
            poll_interval,
            heartbeat_interval,
            max_parallel_jobs,
            debug,
        })
    }
}

#[derive(Debug, Deserialize)]
struct JobPayload {
    run_id: String,
    request: ExecuteRequest,
}

struct ServerClient {
    base_url: String,
    http: reqwest::Client,
}

impl ServerClient {
    fn new(base_url: String) -> Self {
        Self {
            base_url,
            http: reqwest::Client::new(),
        }
    }

    async fn claim_job(&self, worker_id: &str) -> Result<Option<JobPayload>, reqwest::Error> {
        let url = format!("{}/api/v1/jobs/claim", self.base_url.trim_end_matches('/'));
        let response = self
            .http
            .post(&url)
            .json(&serde_json::json!({ "worker_id": worker_id }))
            .send()
            .await?;

        match response.status() {
            StatusCode::OK => {
                let job = response.json::<JobPayload>().await?;
                Ok(Some(job))
            }
            StatusCode::NO_CONTENT => Ok(None),
            status => {
                let body = response.text().await.unwrap_or_default();
                eprintln!("[daemon] claim_job unexpected status {}: {}", status, body);
                Ok(None)
            }
        }
    }

    #[allow(dead_code)]
    async fn send_heartbeat(&self, run_id: &str, worker_id: &str) -> Result<(), reqwest::Error> {
        let url = format!(
            "{}/api/v1/jobs/{}/heartbeat",
            self.base_url.trim_end_matches('/'),
            run_id
        );
        let response = self
            .http
            .post(&url)
            .json(&serde_json::json!({ "worker_id": worker_id }))
            .send()
            .await?;

        if !response.status().is_success() {
            let status = response.status();
            let body = response.text().await.unwrap_or_default();
            eprintln!(
                "[daemon] heartbeat failed for {} status={} body={}",
                run_id, status, body
            );
        }

        Ok(())
    }

    async fn report_success(
        &self,
        run_id: &str,
        worker_id: &str,
        response: &ExecuteResponse,
    ) -> Result<(), reqwest::Error> {
        let url = format!(
            "{}/api/v1/jobs/{}/complete",
            self.base_url.trim_end_matches('/'),
            run_id
        );

        self.http
            .post(&url)
            .json(&serde_json::json!({
                "worker_id": worker_id,
                "response": response,
            }))
            .send()
            .await?
            .error_for_status()?;

        Ok(())
    }

    async fn report_failure(
        &self,
        run_id: &str,
        worker_id: &str,
        message: &str,
    ) -> Result<(), reqwest::Error> {
        let url = format!(
            "{}/api/v1/jobs/{}/failed",
            self.base_url.trim_end_matches('/'),
            run_id
        );

        self.http
            .post(&url)
            .json(&serde_json::json!({
                "worker_id": worker_id,
                "message": message,
            }))
            .send()
            .await?
            .error_for_status()?;

        Ok(())
    }
}

struct JobManager {
    client: Arc<ServerClient>,
    config: WorkerConfig,
    semaphore: Arc<Semaphore>,
}

impl JobManager {
    fn new(config: WorkerConfig, client: Arc<ServerClient>) -> Self {
        let semaphore = Arc::new(Semaphore::new(config.max_parallel_jobs));
        Self {
            client,
            config,
            semaphore,
        }
    }

    async fn spawn_job(&self, job: JobPayload) {
        let permit = match self.semaphore.clone().acquire_owned().await {
            Ok(permit) => permit,
            Err(err) => {
                eprintln!("[daemon] failed to acquire concurrency permit: {}", err);
                return;
            }
        };

        let client = Arc::clone(&self.client);
        let config = self.config.clone();

        tokio::spawn(async move {
            let run_id = job.run_id.clone();
            if config.debug {
                eprintln!("[daemon] executing job {}", run_id);
            }

            let execute_req = job.request;
            let worker_id = config.worker_id.clone();
            let client_for_result = client.clone();

            let (stop_tx, mut stop_rx) = oneshot::channel::<()>();
            let heartbeat_client = client.clone();
            let heartbeat_run_id = run_id.clone();
            let heartbeat_worker = worker_id.clone();
            let heartbeat_interval = config.heartbeat_interval;
            let heartbeat_debug = config.debug;

            tokio::spawn(async move {
                let mut ticker = interval(heartbeat_interval);
                // Skip immediate tick
                let _ = ticker.tick().await;
                loop {
                    tokio::select! {
                        _ = &mut stop_rx => break,
                        _ = ticker.tick() => {
                            if let Err(err) = heartbeat_client
                                .send_heartbeat(&heartbeat_run_id, &heartbeat_worker)
                                .await
                            {
                                eprintln!(
                                    "[daemon] heartbeat error for {}: {}",
                                    heartbeat_run_id, err
                                );
                            } else if heartbeat_debug {
                                eprintln!(
                                    "[daemon] heartbeat ok {}",
                                    heartbeat_run_id
                                );
                            }
                        }
                    }
                }
            });

            let execution_result =
                tokio::task::spawn_blocking(move || executor::execute_request(execute_req))
                    .await
                    .unwrap_or_else(|err| Err(ExecuteJobError::Runtime(err.to_string())));

            let _ = stop_tx.send(());

            match execution_result {
                Ok(response) => {
                    if let Err(err) = client_for_result
                        .report_success(&run_id, &worker_id, &response)
                        .await
                    {
                        eprintln!(
                            "[daemon] failed to report completion for {}: {}",
                            run_id, err
                        );
                    }
                }
                Err(err) => {
                    let message = err.to_string();
                    if let Err(report_err) = client_for_result
                        .report_failure(&run_id, &worker_id, &message)
                        .await
                    {
                        eprintln!(
                            "[daemon] failed to report failure for {}: {} (original error: {})",
                            run_id, report_err, message
                        );
                    }
                }
            }

            drop(permit);

            if config.debug {
                eprintln!("[daemon] finished job {}", run_id);
            }
        });
    }
}

#[tokio::main]
async fn main() {
    if let Err(err) = run_daemon().await {
        eprintln!("daemon-flowy error: {}", err);
        std::process::exit(1);
    }
}

async fn run_daemon() -> Result<(), String> {
    let config = WorkerConfig::from_args()?;
    if config.debug {
        eprintln!(
            "[daemon] starting with worker_id={} server={} max_jobs={} poll={:?} heartbeat={:?}",
            config.worker_id,
            config.server_url,
            config.max_parallel_jobs,
            config.poll_interval,
            config.heartbeat_interval
        );
    }

    let client = Arc::new(ServerClient::new(config.server_url.clone()));
    let manager = JobManager::new(config.clone(), Arc::clone(&client));

    loop {
        sleep(config.poll_interval).await;

        match client.claim_job(&config.worker_id).await {
            Ok(Some(job)) => {
                if config.debug {
                    eprintln!("[daemon] claimed job {}", job.run_id);
                }
                manager.spawn_job(job).await;
            }
            Ok(None) => {
                if config.debug {
                    eprintln!("[daemon] no jobs available");
                }
            }
            Err(err) => {
                eprintln!("[daemon] failed to claim job: {}", err);
            }
        }
    }
}

fn print_help(program: &str) {
    eprintln!("daemon-flowy - Remote task worker");
    eprintln!("Usage: {} [options]", program);
    eprintln!("  --server <url>       flowy-server base URL");
    eprintln!("  --worker-id <id>     Unique worker identifier (default: hostname)");
    eprintln!("  --poll <secs>        Poll interval in seconds (default: 10)");
    eprintln!("  --heartbeat <secs>   Heartbeat interval in seconds (default: 120)");
    eprintln!("  --max-jobs <n>       Maximum parallel jobs (default: 1)");
    eprintln!("  --debug              Enable verbose logging");
    eprintln!("  -h, --help           Show this message");
}
