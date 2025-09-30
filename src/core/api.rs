use serde::{Deserialize, Serialize};

#[derive(Debug, Serialize, Deserialize)]
pub struct ExecuteRequest {
    pub wdl: String,
    #[serde(default)]
    pub inputs: serde_json::Value,
    #[serde(skip_serializing_if = "Option::is_none", default)]
    pub options: Option<ExecuteOptions>,
}

#[derive(Debug, Default, Serialize, Deserialize, Clone)]
pub struct ExecuteOptions {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub task: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub run_id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub base_dir: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ExecuteResponse {
    pub status: String,
    pub outputs: serde_json::Value,
    #[serde(default)]
    pub stdout: Option<String>,
    #[serde(default)]
    pub stderr: Option<String>,
    pub duration_ms: u128,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ErrorResponse {
    pub status: String,
    pub message: String,
}
