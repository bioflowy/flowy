use flowy::runtime::config::Config;
use flowy::runtime::error::RuntimeError;
use flowy::runtime::fs_utils::write_file_atomic;
use flowy::runtime::task_context::TaskContext;
use flowy::runtime::task_runner::{
    deserialize_bindings, serialize_bindings, TaskRunnerRequest, TaskRunnerResponse,
    TASK_RUNNER_PROTOCOL_VERSION,
};
use std::env;
use std::path::{Path, PathBuf};

fn main() {
    if let Err(err) = run() {
        eprintln!("flowy-task-runner error: {err}");
        std::process::exit(1);
    }
}

fn run() -> Result<(), Box<dyn std::error::Error>> {
    let request_path = parse_args()?;
    let response_path = request_path.with_file_name("task_response.json");

    let request_text = std::fs::read_to_string(&request_path)?;
    let request: TaskRunnerRequest = serde_json::from_str(&request_text)?;

    if request.version != TASK_RUNNER_PROTOCOL_VERSION {
        let response = TaskRunnerResponse::failure(
            request.run_id,
            format!(
                "Protocol mismatch: runner version {} but request {}",
                TASK_RUNNER_PROTOCOL_VERSION, request.version
            ),
            Some("protocol_mismatch".to_string()),
        );
        write_response(&response_path, &response)?;
        return Err("task runner protocol mismatch".into());
    }

    let run_id = request.run_id.clone();

    let response = match execute_request(request) {
        Ok(response) => response,
        Err(err) => {
            let classification = classify_error(&err);
            TaskRunnerResponse::failure(run_id, err.to_string(), Some(classification.to_string()))
        }
    };

    write_response(&response_path, &response)?;

    if response.success {
        Ok(())
    } else {
        Err("task execution failed".into())
    }
}

fn parse_args() -> Result<PathBuf, String> {
    let mut args = env::args().skip(1);
    match args.next() {
        Some(path) => Ok(PathBuf::from(path)),
        None => Err("usage: flowy-task-runner <task_request.json>".to_string()),
    }
}

fn execute_request(request: TaskRunnerRequest) -> Result<TaskRunnerResponse, RuntimeError> {
    let run_id = request.run_id.clone();
    let runtime_config = Config::from(request.config);
    let inputs = deserialize_bindings(request.inputs);

    let mut context = TaskContext::new(
        request.task,
        inputs,
        runtime_config,
        request.workflow_dir,
        &run_id,
    )?;

    let task_result = context.execute()?;

    let flowy::runtime::task_context::TaskResult {
        outputs,
        exit_status,
        stdout,
        stderr,
        duration,
        work_dir,
    } = task_result;

    let outputs_serialized = serialize_bindings(&outputs);
    let stdout = stdout.to_string();
    let stderr = stderr.to_string();

    let exit_code = exit_status.code();
    let exit_success = exit_status.success();
    let signal = exit_signal(&exit_status);

    Ok(TaskRunnerResponse::success(
        run_id,
        exit_code,
        signal,
        exit_success,
        stdout,
        stderr,
        duration.as_millis(),
        outputs_serialized,
        work_dir,
    ))
}

fn write_response(
    path: &Path,
    response: &TaskRunnerResponse,
) -> Result<(), Box<dyn std::error::Error>> {
    let mut json = serde_json::to_vec_pretty(response)?;
    json.push(b'\n');
    write_file_atomic(path, &json)?;
    Ok(())
}

fn classify_error(err: &RuntimeError) -> &'static str {
    match err {
        RuntimeError::TaskTimeout { .. } => "timeout",
        RuntimeError::CommandFailed { .. } => "command_failed",
        RuntimeError::FileSystemError { .. } => "filesystem",
        RuntimeError::RunFailed { .. } => "run_failed",
        _ => "runtime_error",
    }
}

#[cfg(unix)]
fn exit_signal(status: &std::process::ExitStatus) -> Option<i32> {
    use std::os::unix::process::ExitStatusExt;
    status.signal()
}

#[cfg(not(unix))]
fn exit_signal(_status: &std::process::ExitStatus) -> Option<i32> {
    None
}
