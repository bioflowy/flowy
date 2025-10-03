//! WDL Specification Tests Runner
//!
//! A miniwdl-compatible specification test runner that parses WDL spec documents
//! and executes test cases using flowy.

use regex::Regex;
use std::collections::{HashMap, HashSet};
use std::fs;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use std::time::{SystemTime, UNIX_EPOCH};

/// Execution backend for running spec tests
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ExecutionBackend {
    /// Execute tests using the local `flowy` binary
    Flowy,
    /// Execute tests via the `flowy-client` CLI (remote server)
    FlowyClient,
}

fn parse_execution_backend(value: &str) -> Result<ExecutionBackend, String> {
    match value {
        "flowy" => Ok(ExecutionBackend::Flowy),
        "flowy-client" | "client" | "remote" => Ok(ExecutionBackend::FlowyClient),
        other => Err(format!(
            "Unknown backend '{}'. Expected 'flowy' or 'flowy-client'",
            other
        )),
    }
}

/// Test case extracted from the WDL specification
#[derive(Debug, Clone)]
pub struct SpecTestCase {
    pub name: String,
    pub wdl_source: String,
    pub input_json: Option<String>,
    pub expected_output: Option<String>,
    pub config_json: Option<HashMap<String, serde_json::Value>>,
    pub line_number: Option<usize>,
}

/// Test execution result
#[derive(Debug)]
pub struct TestResult {
    pub name: String,
    pub status: TestStatus,
    pub message: Option<String>,
    pub duration_ms: u64,
    pub actual_output: Option<String>,
}

/// Test execution status
#[derive(Debug, PartialEq)]
pub enum TestStatus {
    Passed,
    Failed,
    Error,
    Skipped,
    XFail,          // Expected failure that failed as expected
    UnexpectedPass, // Expected failure that unexpectedly passed
}

/// Main spec test runner
pub struct SpecTestRunner {
    pub shared_test_dir: PathBuf,
    pub execution_timestamp: u64,
    pub keep_files: bool,
    pub debug: bool,
    pub xfail_tests: HashSet<String>,
    pub execution_backend: ExecutionBackend,
    pub client_server: Option<String>,
}

impl Default for SpecTestRunner {
    fn default() -> Self {
        Self::new()
    }
}

impl SpecTestRunner {
    pub fn new() -> Self {
        let timestamp = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap_or_default()
            .as_secs();

        Self {
            shared_test_dir: std::env::current_dir()
                .unwrap_or_else(|_| PathBuf::from("."))
                .join(format!("spectest_{}", timestamp)),
            execution_timestamp: timestamp,
            keep_files: false,
            debug: false,
            xfail_tests: HashSet::new(),
            execution_backend: ExecutionBackend::Flowy,
            client_server: None,
        }
    }

    pub fn with_keep_files(mut self, keep: bool) -> Self {
        self.keep_files = keep;
        self
    }

    pub fn with_debug(mut self, debug: bool) -> Self {
        self.debug = debug;
        self
    }

    pub fn with_execution_backend(mut self, backend: ExecutionBackend) -> Self {
        self.execution_backend = backend;
        self
    }

    pub fn with_client_server<S: Into<String>>(mut self, server: Option<S>) -> Self {
        self.client_server = server.map(Into::into);
        self
    }

    /// Load xfail test names from config.txt in the same directory as SPEC.md
    pub fn load_xfail_config<P: AsRef<Path>>(
        &mut self,
        spec_path: P,
    ) -> Result<(), Box<dyn std::error::Error>> {
        let spec_dir = spec_path.as_ref().parent().unwrap_or(Path::new("."));
        let config_path = spec_dir.join("config.txt");

        if config_path.exists() {
            let content = fs::read_to_string(&config_path)?;

            for line in content.lines() {
                // Remove comments (everything after #) and trim whitespace
                let cleaned_line = if let Some(comment_pos) = line.find('#') {
                    &line[..comment_pos]
                } else {
                    line
                };

                let test_name = cleaned_line.trim();
                if !test_name.is_empty() {
                    self.xfail_tests.insert(test_name.to_string());
                }
            }

            if self.debug && !self.xfail_tests.is_empty() {
                eprintln!(
                    "DEBUG: Loaded {} xfail tests from config.txt",
                    self.xfail_tests.len()
                );
                for test_name in &self.xfail_tests {
                    eprintln!("DEBUG: xfail test: {}", test_name);
                }
            }
        }

        Ok(())
    }

    /// Unified test execution method that handles all scenarios
    pub fn run_unified<P: AsRef<Path>>(
        &mut self,
        spec_path: P,
        data_dir: P,
        list_only: bool,
        test_name: Option<String>,
        pattern: Option<String>,
    ) -> Result<Vec<TestResult>, Box<dyn std::error::Error>> {
        // Load xfail tests from config.txt if it exists
        self.load_xfail_config(&spec_path)?;

        let parser = SpecParser::new();
        let test_cases = parser.parse_spec_file(spec_path)?;

        // Handle --list option
        if list_only {
            println!("Available tests ({} total):", test_cases.len());
            for (index, test_case) in test_cases.iter().enumerate() {
                let is_xfail = self.xfail_tests.contains(&test_case.name);
                let suffix = if is_xfail { " (xfail)" } else { "" };
                println!("  {}: {}{}", index + 1, test_case.name, suffix);
            }
            return Ok(Vec::new());
        }

        // Filter test cases based on options
        let filtered_cases: Vec<_> = if let Some(ref name) = test_name {
            // Run single test by name
            test_cases
                .into_iter()
                .filter(|tc| tc.name == *name)
                .collect()
        } else if let Some(ref pat) = pattern {
            // Run tests matching pattern
            test_cases
                .into_iter()
                .filter(|tc| tc.name.contains(pat))
                .collect()
        } else {
            // Run all tests
            test_cases
        };

        if filtered_cases.is_empty() {
            if let Some(ref name) = test_name {
                println!("Test '{}' not found", name);
            } else if let Some(ref pat) = pattern {
                println!("No tests found matching pattern '{}'", pat);
            }
            return Ok(Vec::new());
        }

        println!("Found {} test case(s) to execute", filtered_cases.len());

        // Setup shared test directory
        self.setup_shared_test_directory()?;

        // Execute filtered tests
        let mut results = Vec::new();
        for test_case in filtered_cases {
            // Print test details for single test execution
            if test_name.is_some() {
                self.print_test_details(&test_case);
            }

            let result = self.execute_test_case(&test_case, data_dir.as_ref())?;

            // Print result for single test execution
            if test_name.is_some() {
                self.print_test_result(&result);
            }

            results.push(result);
        }

        // Clean up shared directory if not keeping files
        if !self.keep_files {
            let _ = fs::remove_dir_all(&self.shared_test_dir);
        }

        Ok(results)
    }
}

/// WDL specification parser following miniwdl pattern
pub struct SpecParser {
    details_regex: Regex,
    wdl_regex: Regex,
    input_regex: Regex,
    output_regex: Regex,
    config_regex: Regex,
}

impl Default for SpecParser {
    fn default() -> Self {
        Self::new()
    }
}

impl SpecParser {
    pub fn new() -> Self {
        Self {
            details_regex: Regex::new(r"(?s)<details>(.*?)</details>").unwrap(),
            wdl_regex: Regex::new(r"(?s)<summary>\s*Example:\s*([^\n]+)\s*.*?```wdl(.*?)```")
                .unwrap(),
            input_regex: Regex::new(r"(?s)Example input:\s*```json(.*?)```").unwrap(),
            output_regex: Regex::new(r"(?s)Example output:\s*```json(.*?)```").unwrap(),
            config_regex: Regex::new(r"(?s)Test config:\s*```json(.*?)```").unwrap(),
        }
    }

    /// Parse the WDL specification file and extract test cases
    pub fn parse_spec_file<P: AsRef<Path>>(
        &self,
        spec_path: P,
    ) -> Result<Vec<SpecTestCase>, Box<dyn std::error::Error>> {
        let content = fs::read_to_string(spec_path)?;
        let mut test_cases = Vec::new();

        // Find all <details> blocks
        for details_match in self.details_regex.captures_iter(&content) {
            if let Some(block_content) = details_match.get(1) {
                let block_text = block_content.as_str();

                // Extract WDL code and test name
                if let Some(wdl_match) = self.wdl_regex.captures(block_text) {
                    let name = wdl_match
                        .get(1)
                        .map(|m| m.as_str().trim())
                        .unwrap_or("unknown_test");

                    let wdl_source = wdl_match
                        .get(2)
                        .map(|m| m.as_str().trim())
                        .unwrap_or("")
                        .to_string();

                    // Extract optional components
                    let input_json = self
                        .input_regex
                        .captures(block_text)
                        .and_then(|m| m.get(1))
                        .map(|m| m.as_str().trim().to_string());

                    let expected_output = self
                        .output_regex
                        .captures(block_text)
                        .and_then(|m| m.get(1))
                        .map(|m| m.as_str().trim().to_string());

                    let config_json = self
                        .config_regex
                        .captures(block_text)
                        .and_then(|m| m.get(1))
                        .and_then(|m| {
                            let json_str = m.as_str().trim();
                            serde_json::from_str::<HashMap<String, serde_json::Value>>(json_str)
                                .ok()
                        });

                    // Calculate line number (approximate)
                    let line_number = content[..details_match.get(0).unwrap().start()]
                        .matches('\n')
                        .count()
                        + 1;

                    let test_case = SpecTestCase {
                        name: sanitize_name(name).to_string(),
                        wdl_source,
                        input_json,
                        expected_output,
                        config_json,
                        line_number: Some(line_number),
                    };

                    test_cases.push(test_case);
                }
            }
        }

        Ok(test_cases)
    }
}

impl SpecTestRunner {
    /// Execute a single test case
    fn execute_test_case(
        &mut self,
        test_case: &SpecTestCase,
        data_dir: &Path,
    ) -> Result<TestResult, Box<dyn std::error::Error>> {
        let start_time = SystemTime::now();

        // Check if this is an expected failure test
        let is_xfail = self.is_expected_failure(test_case);

        // Use shared directory directly without creating subdirectories
        let test_dir = &self.shared_test_dir;
        // Directory already created in setup_shared_test_directory()

        // Write WDL file
        let wdl_path = test_dir.join(format!("{}.wdl", test_case.name));
        fs::write(&wdl_path, &test_case.wdl_source)?;

        // Write input file if present
        let input_path = if let Some(ref input_json) = test_case.input_json {
            let input_file = test_dir.join(format!("{}_input.json", test_case.name));
            fs::write(&input_file, input_json)?;
            Some(input_file)
        } else {
            None
        };

        // Execute flowy
        let result = self.execute_flowy(&wdl_path, input_path.as_ref(), data_dir);

        let duration = start_time.elapsed()?.as_millis() as u64;

        let test_result = match result {
            Ok(output) => {
                if is_xfail {
                    // Expected to fail but passed - this is unexpected
                    TestResult {
                        name: test_case.name.clone(),
                        status: TestStatus::UnexpectedPass,
                        message: Some("Test was expected to fail but passed".to_string()),
                        duration_ms: duration,
                        actual_output: Some(output),
                    }
                } else if let Some(expected) = &test_case.expected_output {
                    // Regular test with expected output - compare results
                    if self.compare_outputs(&output, expected) {
                        TestResult {
                            name: test_case.name.clone(),
                            status: TestStatus::Passed,
                            message: None,
                            duration_ms: duration,
                            actual_output: Some(output),
                        }
                    } else {
                        TestResult {
                            name: test_case.name.clone(),
                            status: TestStatus::Failed,
                            message: Some("Output mismatch".to_string()),
                            duration_ms: duration,
                            actual_output: Some(output),
                        }
                    }
                } else {
                    // Regular test with no expected output - consider success if no error
                    TestResult {
                        name: test_case.name.clone(),
                        status: TestStatus::Passed,
                        message: None,
                        duration_ms: duration,
                        actual_output: Some(output),
                    }
                }
            }
            Err(error) => {
                if is_xfail {
                    // Expected to fail and did fail - this is correct
                    TestResult {
                        name: test_case.name.clone(),
                        status: TestStatus::XFail,
                        message: Some(format!("Expected failure: {}", error)),
                        duration_ms: duration,
                        actual_output: None,
                    }
                } else {
                    // Regular test that failed
                    TestResult {
                        name: test_case.name.clone(),
                        status: TestStatus::Error,
                        message: Some(error),
                        duration_ms: duration,
                        actual_output: None,
                    }
                }
            }
        };

        Ok(test_result)
    }

    /// Check if a test case is expected to fail
    fn is_expected_failure(&self, test_case: &SpecTestCase) -> bool {
        // Check if test is in config.txt xfail list
        if self.xfail_tests.contains(&test_case.name) {
            return true;
        }

        // Check if config_json has "fail": true
        if let Some(ref config) = test_case.config_json {
            if let Some(fail_value) = config.get("fail") {
                if let Some(fail_bool) = fail_value.as_bool() {
                    return fail_bool;
                }
            }
        }

        // Following miniwdl pattern: tests ending with "fail" are expected to fail
        test_case.name.contains("fail")
            || test_case.name.ends_with("_fail")
            || test_case.name.ends_with("_fail_task")
    }

    /// Set up the shared test directory for this execution
    fn setup_shared_test_directory(&mut self) -> Result<(), Box<dyn std::error::Error>> {
        fs::create_dir_all(&self.shared_test_dir)?;
        Ok(())
    }

    /// Execute flowy command
    fn execute_flowy(
        &self,
        wdl_path: &Path,
        input_path: Option<&PathBuf>,
        data_dir: &Path,
    ) -> Result<String, String> {
        match self.execution_backend {
            ExecutionBackend::Flowy => {
                let exe_path = std::env::current_dir()
                    .unwrap_or_else(|_| PathBuf::from("."))
                    .join("target/debug/flowy");

                let mut cmd = Command::new(exe_path);
                cmd.arg("run")
                    .arg(wdl_path)
                    .current_dir(data_dir)
                    .stdout(Stdio::piped())
                    .stderr(Stdio::piped());

                if let Some(input_file) = input_path {
                    cmd.arg("-i").arg(input_file);
                }

                self.log_command(&cmd, data_dir);

                match cmd.output() {
                    Ok(output) => {
                        if output.status.success() {
                            Ok(String::from_utf8_lossy(&output.stdout).to_string())
                        } else {
                            Err(String::from_utf8_lossy(&output.stderr).to_string())
                        }
                    }
                    Err(e) => Err(format!("Failed to execute flowy: {}", e)),
                }
            }
            ExecutionBackend::FlowyClient => {
                let exe_path = std::env::current_dir()
                    .unwrap_or_else(|_| PathBuf::from("."))
                    .join("target/debug/flowy-client");

                let mut cmd = Command::new(exe_path);
                cmd.arg("run")
                    .arg(wdl_path)
                    .current_dir(data_dir)
                    .stdout(Stdio::piped())
                    .stderr(Stdio::piped());

                if let Some(input_file) = input_path {
                    cmd.arg("-i").arg(input_file);
                }

                if let Some(server) = &self.client_server {
                    cmd.arg("-s").arg(server);
                }

                let base_dir_path = if data_dir.is_absolute() {
                    data_dir.to_path_buf()
                } else {
                    std::env::current_dir()
                        .unwrap_or_else(|_| PathBuf::from("."))
                        .join(data_dir)
                };

                cmd.arg("--basedir").arg(base_dir_path);

                self.log_command(&cmd, data_dir);

                match cmd.output() {
                    Ok(output) => {
                        if output.status.success() {
                            let stdout = String::from_utf8_lossy(&output.stdout).to_string();
                            Ok(Self::extract_outputs_json(&stdout)
                                .unwrap_or_else(|| stdout.trim().to_string()))
                        } else {
                            Err(String::from_utf8_lossy(&output.stderr).to_string())
                        }
                    }
                    Err(e) => Err(format!("Failed to execute flowy-client: {}", e)),
                }
            }
        }
    }

    fn log_command(&self, cmd: &Command, cwd: &Path) {
        if !self.debug {
            return;
        }

        let program = cmd.get_program().to_string_lossy();
        let args: Vec<String> = cmd
            .get_args()
            .map(|arg| arg.to_string_lossy().into_owned())
            .collect();
        let args_display = if args.is_empty() {
            String::new()
        } else {
            format!(" {}", args.join(" "))
        };

        eprintln!(
            "DEBUG: Executing command (cwd={}): {}{}",
            cwd.display(),
            program,
            args_display
        );
    }

    /// Extract the JSON payload printed by `flowy-client`'s `outputs:` section.
    fn extract_outputs_json(stdout: &str) -> Option<String> {
        let mut collecting = false;
        let mut buffer: Vec<String> = Vec::new();

        for line in stdout.lines() {
            let trimmed = line.trim_end();
            if !collecting {
                if let Some(rest) = trimmed.strip_prefix("outputs:") {
                    collecting = true;
                    let rest = rest.trim_start();
                    if !rest.is_empty() {
                        buffer.push(rest.to_string());
                    }
                }
                continue;
            }

            let lower = trimmed.to_ascii_lowercase();
            if trimmed.starts_with("stdout:")
                || trimmed.starts_with("stderr:")
                || trimmed.starts_with("status:")
                || trimmed.starts_with("duration_ms:")
                || lower.starts_with("status:")
                || lower.starts_with("stdout:")
                || lower.starts_with("stderr:")
            {
                break;
            }

            buffer.push(line.to_string());
        }

        if buffer.is_empty() {
            return None;
        }

        let json = buffer.join("\n").trim().to_string();
        if json.is_empty() {
            None
        } else {
            Some(json)
        }
    }

    /// Compare actual output with expected output (following miniwdl's subset comparison logic)
    fn compare_outputs(&self, actual: &str, expected: &str) -> bool {
        if self.debug {
            eprintln!("DEBUG: Comparing outputs");
            eprintln!("DEBUG: Expected: {}", expected);
            eprintln!("DEBUG: Actual: {}", actual);
        }

        // First try to parse both as JSON
        let actual_result = serde_json::from_str::<serde_json::Value>(actual);
        let expected_result = serde_json::from_str::<serde_json::Value>(expected);

        if self.debug {
            eprintln!(
                "DEBUG: Expected JSON parse result: {:?}",
                expected_result.is_ok()
            );
            eprintln!(
                "DEBUG: Actual JSON parse result: {:?}",
                actual_result.is_ok()
            );
        }

        match (actual_result, expected_result) {
            (Ok(actual_json), Ok(expected_json)) => {
                if self.debug {
                    eprintln!("DEBUG: Both are valid JSON, comparing values");
                }
                // Both are valid JSON - compare as JSON
                self.compare_json_values(&actual_json, &expected_json)
            }
            (Ok(actual_json), Err(expected_error)) => {
                if self.debug {
                    eprintln!("DEBUG: Expected JSON is invalid: {:?}", expected_error);
                    eprintln!("DEBUG: Trying to clean up expected JSON");
                }
                // Actual is valid JSON, expected is not - try to clean up expected and reparse
                let cleaned_expected = self.clean_invalid_json(expected);
                if self.debug {
                    eprintln!("DEBUG: Cleaned expected: {}", cleaned_expected);
                }
                if let Ok(expected_json) =
                    serde_json::from_str::<serde_json::Value>(&cleaned_expected)
                {
                    if self.debug {
                        eprintln!("DEBUG: Cleaned expected JSON successfully, retrying comparison");
                    }
                    // Retry comparison with cleaned expected
                    self.compare_json_values(&actual_json, &expected_json)
                } else {
                    if self.debug {
                        eprintln!("DEBUG: Still can't parse expected JSON, falling back to string comparison");
                    }
                    // Still can't parse expected - fall back to string comparison
                    actual.trim() == cleaned_expected.trim()
                }
            }
            (Err(actual_error), Ok(expected_json)) => {
                if self.debug {
                    eprintln!("DEBUG: Actual JSON is invalid: {:?}", actual_error);
                }
                // Expected is valid JSON, actual is not - unlikely but handle it
                let cleaned_actual = self.clean_invalid_json(actual);
                if let Ok(actual_json) = serde_json::from_str::<serde_json::Value>(&cleaned_actual)
                {
                    self.compare_json_values(&actual_json, &expected_json)
                } else {
                    false // Actual should be valid JSON from our runtime
                }
            }
            (Err(_), Err(_)) => {
                if self.debug {
                    eprintln!("DEBUG: Both JSON parse failed, using string comparison");
                }
                // Neither is valid JSON - fallback to string comparison
                actual.trim() == expected.trim()
            }
        }
    }

    /// Clean up invalid JSON by removing trailing commas
    fn clean_invalid_json(&self, json_str: &str) -> String {
        // Simple but effective approach: use regex-like logic to remove trailing commas
        let mut result = json_str.to_string();

        // Remove trailing commas before closing braces
        result = result.replace(",\n  }", "\n  }");
        result = result.replace(",\n    }", "\n    }");
        result = result.replace(",\n      }", "\n      }");
        result = result.replace(",\n        }", "\n        }");
        result = result.replace(",\n}", "\n}");

        // Remove trailing commas before closing brackets
        result = result.replace(",\n  ]", "\n  ]");
        result = result.replace(",\n    ]", "\n    ]");
        result = result.replace(",\n      ]", "\n      ]");
        result = result.replace(",\n        ]", "\n        ]");
        result = result.replace(",\n]", "\n]");

        // Handle cases with different whitespace patterns
        result = result.replace(",\r\n  }", "\r\n  }");
        result = result.replace(",\r\n}", "\r\n}");
        result = result.replace(",\r\n  ]", "\r\n  ]");
        result = result.replace(",\r\n]", "\r\n]");

        // Handle the specific case we're seeing: comma followed by whitespace and closing brace
        let lines: Vec<&str> = result.lines().collect();
        let mut cleaned_lines = Vec::new();

        for (i, line) in lines.iter().enumerate() {
            let trimmed = line.trim();
            if trimmed.ends_with(',') && i + 1 < lines.len() {
                let next_line = lines[i + 1].trim();
                if next_line == "}" || next_line == "]" {
                    // Remove the trailing comma
                    let without_comma = line.trim_end_matches(',');
                    cleaned_lines.push(without_comma.to_string());
                } else {
                    cleaned_lines.push(line.to_string());
                }
            } else {
                cleaned_lines.push(line.to_string());
            }
        }

        cleaned_lines.join("\n")
    }

    /// Helper method to compare two JSON values
    fn compare_json_values(
        &self,
        actual: &serde_json::Value,
        expected: &serde_json::Value,
    ) -> bool {
        if let (Some(actual_obj), Some(expected_obj)) = (actual.as_object(), expected.as_object()) {
            for (key, expected_value) in expected_obj {
                if let Some(actual_value) = actual_obj.get(key) {
                    if actual_value != expected_value {
                        if self.debug {
                            eprintln!(
                                "DEBUG: Value mismatch for key '{}': expected {:?}, actual {:?}",
                                key, expected_value, actual_value
                            );
                        }
                        return false;
                    }
                } else {
                    if self.debug {
                        eprintln!("DEBUG: Missing key in actual output: '{}'", key);
                    }
                    return false;
                }
            }
            true
        } else {
            actual == expected
        }
    }

    /// Print detailed test information
    fn print_test_details(&self, test_case: &SpecTestCase) {
        println!("\n=== Test Details ===");
        println!("Name: {}", test_case.name);
        if let Some(line) = test_case.line_number {
            println!("Line: {}", line);
        }

        println!("\nWDL Source:");
        println!("```wdl");
        println!("{}", test_case.wdl_source);
        println!("```");

        if let Some(ref input) = test_case.input_json {
            println!("\nInput JSON:");
            println!("{}", input);
        }

        if let Some(ref expected) = test_case.expected_output {
            println!("\nExpected Output:");
            println!("{}", expected);
        }

        if let Some(ref config) = test_case.config_json {
            println!("\nTest Config:");
            if let Ok(config_str) = serde_json::to_string_pretty(config) {
                println!("{}", config_str);
            }
        }
    }

    /// Print test result
    fn print_test_result(&self, result: &TestResult) {
        println!("\n=== Test Result ===");
        println!("Test: {}", result.name);
        println!("Status: {:?}", result.status);
        println!("Duration: {}ms", result.duration_ms);

        if let Some(ref message) = result.message {
            println!("Message: {}", message);
        }

        if let Some(ref output) = result.actual_output {
            println!("Actual Output:");
            println!("{}", output);
        }
    }
}

/// Sanitize test name for use as filename
fn sanitize_name(name: &str) -> String {
    name.trim()
        .replace(".wdl", "")
        .chars()
        .map(|c| {
            if c.is_alphanumeric() || c == '_' || c == '-' {
                c.to_ascii_lowercase()
            } else {
                '_'
            }
        })
        .collect::<String>()
        .trim_matches('_')
        .to_string()
}

fn extract_option_value<'a>(arg: &'a str, flag: &str) -> Option<&'a str> {
    arg.strip_prefix(flag)
        .map(|rest| rest.trim_start_matches(|c: char| c == '=' || c.is_whitespace()))
}

fn main() {
    let args: Vec<String> = std::env::args().collect();

    if args.len() < 3 {
        eprintln!("Usage: {} <spec_file> <data_dir> [options]", args[0]);
        eprintln!("Arguments:");
        eprintln!("  <spec_file>           Path to WDL SPEC.md file");
        eprintln!("  <data_dir>           Path to test data directory");
        eprintln!("Options:");
        eprintln!("  --list                   List all tests");
        eprintln!("  --name <test_name>       Run specific test");
        eprintln!("  --pattern <pattern>      Run tests matching pattern");
        eprintln!("  --keep-files             Keep test files after execution");
        eprintln!("  --debug                  Enable debug output");
        eprintln!("  --runner <flowy|flowy-client>");
        eprintln!("                            Choose execution backend (default: flowy)");
        eprintln!("  --client-server <url>    Server URL when using flowy-client");
        std::process::exit(1);
    }

    let spec_file = PathBuf::from(&args[1]);
    let data_dir = PathBuf::from(&args[2]);

    let mut runner = SpecTestRunner::new();
    let mut list_tests = false;
    let mut test_name: Option<String> = None;
    let mut pattern: Option<String> = None;
    let mut backend = ExecutionBackend::Flowy;
    let mut client_server: Option<String> = None;

    let mut set_backend_value = |raw: &str| match parse_execution_backend(raw) {
        Ok(value) => {
            backend = value;
        }
        Err(err) => {
            eprintln!("Error: {}", err);
            std::process::exit(1);
        }
    };

    // Parse command line arguments
    let mut i = 3;
    while i < args.len() {
        let arg = &args[i];

        if arg == "--list" {
            list_tests = true;
        } else if arg == "--name" {
            i += 1;
            if i < args.len() {
                test_name = Some(args[i].clone());
            }
        } else if arg == "--pattern" {
            i += 1;
            if i < args.len() {
                pattern = Some(args[i].clone());
            }
        } else if arg == "--keep-files" {
            runner = runner.with_keep_files(true);
        } else if arg == "--debug" {
            runner = runner.with_debug(true);
        } else if arg == "--runner" || arg == "--backend" {
            i += 1;
            if i < args.len() {
                set_backend_value(&args[i]);
            } else {
                eprintln!("Error: --runner/--backend requires a value (flowy or flowy-client)");
                std::process::exit(1);
            }
        } else if let Some(value) = extract_option_value(arg, "--runner") {
            if value.is_empty() {
                eprintln!("Error: --runner requires a value (flowy or flowy-client)");
                std::process::exit(1);
            }
            set_backend_value(value);
        } else if let Some(value) = extract_option_value(arg, "--backend") {
            if value.is_empty() {
                eprintln!("Error: --backend requires a value (flowy or flowy-client)");
                std::process::exit(1);
            }
            set_backend_value(value);
        } else if arg == "--client-server" {
            i += 1;
            if i < args.len() {
                client_server = Some(args[i].clone());
            } else {
                eprintln!("Error: --client-server requires a URL");
                std::process::exit(1);
            }
        } else if let Some(value) = extract_option_value(arg, "--client-server") {
            if value.is_empty() {
                eprintln!("Error: --client-server requires a URL");
                std::process::exit(1);
            }
            client_server = Some(value.to_string());
        }

        i += 1;
    }

    runner = runner
        .with_execution_backend(backend)
        .with_client_server(client_server);

    // Execute unified workflow
    match runner.run_unified(&spec_file, &data_dir, list_tests, test_name, pattern) {
        Ok(results) => {
            if !results.is_empty() {
                print_summary(&results);

                let has_failures = results.iter().any(|r| {
                    r.status == TestStatus::Failed
                        || r.status == TestStatus::Error
                        || r.status == TestStatus::UnexpectedPass
                });
                if has_failures {
                    std::process::exit(1);
                }
            }
        }
        Err(e) => {
            eprintln!("Error: {}", e);
            std::process::exit(1);
        }
    }
}

/// Print test execution summary
fn print_summary(results: &[TestResult]) {
    let passed = results
        .iter()
        .filter(|r| r.status == TestStatus::Passed)
        .count();
    let xfail_tests: Vec<&TestResult> = results
        .iter()
        .filter(|r| r.status == TestStatus::XFail)
        .collect();
    let failed_tests: Vec<&TestResult> = results
        .iter()
        .filter(|r| r.status == TestStatus::Failed)
        .collect();
    let error_tests: Vec<&TestResult> = results
        .iter()
        .filter(|r| r.status == TestStatus::Error)
        .collect();
    let unexpected_pass_tests: Vec<&TestResult> = results
        .iter()
        .filter(|r| r.status == TestStatus::UnexpectedPass)
        .collect();

    // Calculate successful tests (Passed + XFail)
    let successful = passed + xfail_tests.len();

    // Display failed test names if any
    if !failed_tests.is_empty() {
        println!("\n=== Failed Tests ===");
        for test in &failed_tests {
            println!("  {}", test.name);
        }
    }

    // Display error test names if any
    if !error_tests.is_empty() {
        println!("\n=== Error Tests ===");
        for test in &error_tests {
            println!("  {}", test.name);
        }
    }

    // Display unexpected pass tests (these are problematic)
    if !unexpected_pass_tests.is_empty() {
        println!("\n=== Unexpected Pass Tests ===");
        for test in &unexpected_pass_tests {
            println!("  {} (expected to fail but passed)", test.name);
        }
    }

    println!("\n=== Summary ===");
    println!("Total: {}", results.len());
    println!(
        "Successful: {} (Passed: {}, XFail: {})",
        successful,
        passed,
        xfail_tests.len()
    );
    println!("Failed: {}", failed_tests.len());
    println!("Errors: {}", error_tests.len());
    println!("Unexpected Passes: {}", unexpected_pass_tests.len());
}
