use serde::{Deserialize, Serialize};
use std::env;
use std::fs;
use std::io;
use std::path::PathBuf;

#[derive(Default, Serialize, Deserialize, Clone)]
pub struct CliConfig {
    #[serde(rename = "SERVER_URL", skip_serializing_if = "Option::is_none")]
    pub server_url: Option<String>,
    #[serde(rename = "DEBUG", skip_serializing_if = "Option::is_none")]
    pub debug: Option<bool>,
}

pub fn config_file_path() -> Result<PathBuf, String> {
    if let Some(home) = env::var_os("HOME") {
        if !home.is_empty() {
            return Ok(PathBuf::from(home).join(".flowy"));
        }
    }

    if cfg!(windows) {
        if let Some(profile) = env::var_os("USERPROFILE") {
            if !profile.is_empty() {
                return Ok(PathBuf::from(profile).join(".flowy"));
            }
        }
    }

    Err("Unable to determine home directory; set --server each time".to_string())
}

pub fn load_config(path: &PathBuf) -> Result<CliConfig, String> {
    match fs::read_to_string(path) {
        Ok(contents) => toml::from_str::<CliConfig>(&contents)
            .map_err(|e| format!("Failed to parse {} as TOML: {}", path.display(), e)),
        Err(err) if err.kind() == io::ErrorKind::NotFound => Ok(CliConfig::default()),
        Err(err) if err.kind() == io::ErrorKind::IsADirectory => Err(format!(
            "Expected {} to be a TOML file, but found a directory",
            path.display()
        )),
        Err(err) => Err(format!("Failed to read {}: {}", path.display(), err)),
    }
}

pub fn save_config(path: &PathBuf, config: &CliConfig) -> Result<(), String> {
    let serialized = toml::to_string(config)
        .map_err(|e| format!("Failed to serialize config to TOML: {}", e))?;
    fs::write(path, serialized)
        .map_err(|e| format!("Failed to write config to {}: {}", path.display(), e))
}
