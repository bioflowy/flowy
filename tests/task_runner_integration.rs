use miniwdl_rust::env::Bindings;
use miniwdl_rust::parser;
use miniwdl_rust::runtime::config::Config;
use miniwdl_rust::runtime::fs_utils::WorkflowDirectory;
use miniwdl_rust::runtime::task::TaskEngine;
use miniwdl_rust::runtime::task_runner::TASK_RUNNER_PROTOCOL_VERSION;
use miniwdl_rust::tree::Task;
use miniwdl_rust::Value;
use once_cell::sync::Lazy;
use serde_json::Value as JsonValue;
use std::error::Error;
use std::path::PathBuf;
use std::sync::Mutex;
use tempfile::TempDir;

static RUNNER_GUARD: Lazy<Mutex<()>> = Lazy::new(|| Mutex::new(()));

struct EnvVarGuard {
    key: &'static str,
}

impl EnvVarGuard {
    fn set(value_path: &PathBuf) -> Self {
        std::env::set_var("MINIWDL_TASK_RUNNER", value_path);
        Self {
            key: "MINIWDL_TASK_RUNNER",
        }
    }
}

impl Drop for EnvVarGuard {
    fn drop(&mut self) {
        std::env::remove_var(self.key);
    }
}

#[test]
fn task_runner_creates_expected_artifacts() -> Result<(), Box<dyn Error>> {
    let _lock = RUNNER_GUARD.lock().unwrap();

    let runner_path = locate_miniwdl_task_runner();
    let _env_guard = EnvVarGuard::set(&runner_path);

    let temp_dir = TempDir::new()?;
    let workflow_dir = WorkflowDirectory::create(temp_dir.path(), "itest")?;
    let config = Config::default();
    let engine = TaskEngine::new(config, workflow_dir.clone());

    let (task, task_name) = parse_single_task(
        r#"
        version 1.2
        task hello {
            command <<<
                echo "hello-subprocess"
            >>>
            output {
                String out = read_string(stdout())
            }
        }
    "#,
    )?;

    let run_id = "run123";
    let result = engine.execute_task_default(task, Bindings::new(), run_id)?;

    // Outputs should include the stdout-based string
    let out_value = result
        .outputs
        .resolve("out")
        .expect("missing out value")
        .clone();
    let output_string = match out_value {
        Value::String { value, .. } => value,
        other => panic!("Expected string output, got {:?}", other),
    };
    assert_eq!(output_string, "hello-subprocess");

    // Task directory artifacts
    let task_dir = workflow_dir.work.join(&task_name);
    let request_path = task_dir.join("task_request.json");
    let response_path = task_dir.join("task_response.json");
    assert!(request_path.exists(), "task_request.json not created");
    assert!(response_path.exists(), "task_response.json not created");

    // Validate request JSON
    let request_json: JsonValue = serde_json::from_str(&std::fs::read_to_string(&request_path)?)?;
    assert_eq!(
        request_json["version"],
        JsonValue::from(TASK_RUNNER_PROTOCOL_VERSION)
    );
    assert_eq!(request_json["run_id"], JsonValue::from(run_id));
    assert_eq!(
        request_json["task"]["name"],
        JsonValue::from(task_name.clone())
    );

    // Validate response JSON
    let response_json: JsonValue = serde_json::from_str(&std::fs::read_to_string(&response_path)?)?;
    assert_eq!(response_json["success"], JsonValue::Bool(true));
    assert_eq!(response_json["run_id"], JsonValue::from(run_id));
    assert_eq!(
        response_json["version"],
        JsonValue::from(TASK_RUNNER_PROTOCOL_VERSION)
    );

    // Stdout/stderr URLs should point to files containing command output
    let stdout_path = url_to_path(result.stdout.as_str())?;
    let stderr_path = url_to_path(result.stderr.as_str())?;
    assert_eq!(std::fs::read_to_string(stdout_path)?, "hello-subprocess\n");
    assert_eq!(std::fs::read_to_string(stderr_path)?, "");
    Ok(())
}

fn parse_single_task(source: &str) -> Result<(Task, String), Box<dyn Error>> {
    let document = parser::parse_document(source, "1.2")?;
    let task = document
        .tasks
        .get(0)
        .cloned()
        .ok_or_else(|| "expected exactly one task".to_string())?;
    let name = task.name.clone();
    Ok((task, name))
}

fn locate_miniwdl_task_runner() -> PathBuf {
    let mut path = std::env::current_exe().expect("current_exe not available");
    // current_exe -> target/debug/deps/<test>
    path.pop(); // remove test binary name
    if path.ends_with("deps") {
        path.pop();
    }
    let binary_name = if cfg!(windows) {
        "miniwdl-task-runner.exe"
    } else {
        "miniwdl-task-runner"
    };
    path.push(binary_name);
    if !path.exists() {
        panic!("miniwdl-task-runner binary not found at {}", path.display());
    }
    path
}

fn url_to_path(url: &str) -> Result<PathBuf, Box<dyn Error>> {
    let parsed = url::Url::parse(url).map_err(|e| format!("invalid URL {}: {}", url, e))?;
    Ok(parsed
        .to_file_path()
        .map_err(|_| format!("URL {} is not a valid file path", url))?)
}
