//! WDL Specification Tests Runner
//!
//! A miniwdl-compatible specification test runner that parses WDL spec documents
//! and executes test cases using miniwdl-rust.

use regex::Regex;
use std::fs;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use std::time::{SystemTime, UNIX_EPOCH};

/// Test case extracted from the WDL specification
#[derive(Debug, Clone)]
pub struct SpecTestCase {
    pub name: String,
    pub wdl_source: String,
    pub input_json: Option<String>,
    pub expected_output: Option<String>,
    pub config_json: Option<String>,
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
}

/// Main spec test runner
pub struct SpecTestRunner {
    pub shared_test_dir: PathBuf,
    pub execution_timestamp: u64,
    pub keep_files: bool,
    pub debug: bool,
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
                        .map(|m| m.as_str().trim().to_string());

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
    /// Run all test cases
    pub fn run_all_tests<P: AsRef<Path>>(
        &mut self,
        spec_path: P,
        data_dir: P,
    ) -> Result<Vec<TestResult>, Box<dyn std::error::Error>> {
        let parser = SpecParser::new();
        let test_cases = parser.parse_spec_file(spec_path)?;

        println!("Found {} test cases", test_cases.len());

        // Create the shared test directory
        self.setup_shared_test_directory()?;

        let mut results = Vec::new();
        for test_case in test_cases {
            let result = self.execute_test_case(&test_case, data_dir.as_ref())?;
            results.push(result);
        }

        // Clean up shared directory if not keeping files
        if !self.keep_files {
            let _ = fs::remove_dir_all(&self.shared_test_dir);
        }

        Ok(results)
    }

    /// Run tests matching a pattern
    pub fn run_tests_matching<P: AsRef<Path>>(
        &mut self,
        spec_path: P,
        data_dir: P,
        pattern: &str,
    ) -> Result<Vec<TestResult>, Box<dyn std::error::Error>> {
        let parser = SpecParser::new();
        let test_cases = parser.parse_spec_file(spec_path)?;

        let filtered_cases: Vec<_> = test_cases
            .into_iter()
            .filter(|tc| tc.name.contains(pattern))
            .collect();

        println!(
            "Found {} test cases matching pattern '{}'",
            filtered_cases.len(),
            pattern
        );

        // Create the shared test directory
        self.setup_shared_test_directory()?;

        let mut results = Vec::new();
        for test_case in filtered_cases {
            let result = self.execute_test_case(&test_case, data_dir.as_ref())?;
            results.push(result);
        }

        // Clean up shared directory if not keeping files
        if !self.keep_files {
            let _ = fs::remove_dir_all(&self.shared_test_dir);
        }

        Ok(results)
    }

    /// Run a single test by name
    pub fn run_single_test<P: AsRef<Path>>(
        &mut self,
        spec_path: P,
        data_dir: P,
        test_name: &str,
    ) -> Result<Option<TestResult>, Box<dyn std::error::Error>> {
        let parser = SpecParser::new();
        let test_cases = parser.parse_spec_file(spec_path)?;

        if let Some(test_case) = test_cases.into_iter().find(|tc| tc.name == test_name) {
            println!("Running test: {}", test_case.name);
            self.print_test_details(&test_case);

            // Create the shared test directory
            self.setup_shared_test_directory()?;

            let result = self.execute_test_case(&test_case, data_dir.as_ref())?;
            self.print_test_result(&result);

            // Clean up shared directory if not keeping files
            if !self.keep_files {
                let _ = fs::remove_dir_all(&self.shared_test_dir);
            }

            Ok(Some(result))
        } else {
            println!("Test '{}' not found", test_name);
            Ok(None)
        }
    }

    /// List all available test names
    pub fn list_tests<P: AsRef<Path>>(
        &self,
        spec_path: P,
    ) -> Result<Vec<String>, Box<dyn std::error::Error>> {
        let parser = SpecParser::new();
        let test_cases = parser.parse_spec_file(spec_path)?;

        let test_names: Vec<String> = test_cases.iter().map(|tc| tc.name.clone()).collect();

        println!("Available tests ({} total):", test_names.len());
        for (index, name) in test_names.iter().enumerate() {
            println!("  {}: {}", index + 1, name);
        }

        Ok(test_names)
    }

    /// Execute a single test case
    fn execute_test_case(
        &mut self,
        test_case: &SpecTestCase,
        data_dir: &Path,
    ) -> Result<TestResult, Box<dyn std::error::Error>> {
        let start_time = SystemTime::now();

        // Create individual test directory within shared directory
        let test_dir = self
            .shared_test_dir
            .join(format!("test_{}", test_case.name));
        fs::create_dir_all(&test_dir)?;

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

        // Execute miniwdl-rust
        let result = self.execute_miniwdl_rust(&wdl_path, input_path.as_ref(), data_dir);

        let duration = start_time.elapsed()?.as_millis() as u64;

        let test_result = match result {
            Ok(output) => {
                if let Some(expected) = &test_case.expected_output {
                    // Compare with expected output
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
                    // No expected output, consider success if no error
                    TestResult {
                        name: test_case.name.clone(),
                        status: TestStatus::Passed,
                        message: None,
                        duration_ms: duration,
                        actual_output: Some(output),
                    }
                }
            }
            Err(error) => TestResult {
                name: test_case.name.clone(),
                status: TestStatus::Error,
                message: Some(error),
                duration_ms: duration,
                actual_output: None,
            },
        };

        Ok(test_result)
    }

    /// Set up the shared test directory for this execution
    fn setup_shared_test_directory(&mut self) -> Result<(), Box<dyn std::error::Error>> {
        fs::create_dir_all(&self.shared_test_dir)?;
        Ok(())
    }

    /// Execute miniwdl-rust command
    fn execute_miniwdl_rust(
        &self,
        wdl_path: &Path,
        input_path: Option<&PathBuf>,
        data_dir: &Path,
    ) -> Result<String, String> {
        // Get absolute path to miniwdl-rust executable
        let exe_path = std::env::current_dir()
            .unwrap_or_else(|_| PathBuf::from("."))
            .join("target/debug/miniwdl-rust");

        let mut cmd = Command::new(exe_path);
        cmd.arg("run")
            .arg(wdl_path)
            .current_dir(data_dir) // Run in data directory to resolve file paths
            .stdout(Stdio::piped())
            .stderr(Stdio::piped());

        if let Some(input_file) = input_path {
            cmd.arg("-i").arg(input_file);
        }

        if self.debug {
            cmd.arg("--debug");
        }

        match cmd.output() {
            Ok(output) => {
                if output.status.success() {
                    Ok(String::from_utf8_lossy(&output.stdout).to_string())
                } else {
                    Err(String::from_utf8_lossy(&output.stderr).to_string())
                }
            }
            Err(e) => Err(format!("Failed to execute miniwdl-rust: {}", e)),
        }
    }

    /// Compare actual output with expected output (following miniwdl's subset comparison logic)
    fn compare_outputs(&self, actual: &str, expected: &str) -> bool {
        if let (Ok(actual_json), Ok(expected_json)) = (
            serde_json::from_str::<serde_json::Value>(actual),
            serde_json::from_str::<serde_json::Value>(expected),
        ) {
            // Follow miniwdl's logic: check only that expected keys exist in actual and match
            if let (Some(actual_obj), Some(expected_obj)) =
                (actual_json.as_object(), expected_json.as_object())
            {
                // For each expected key-value pair, check if actual has the same key with same value
                for (key, expected_value) in expected_obj {
                    if let Some(actual_value) = actual_obj.get(key) {
                        if actual_value != expected_value {
                            return false;
                        }
                    } else {
                        // Missing key in actual output
                        return false;
                    }
                }
                // All expected keys matched - success (ignore extra keys in actual)
                true
            } else {
                // Not objects - fall back to strict equality
                actual_json == expected_json
            }
        } else {
            // Fallback to string comparison
            actual.trim() == expected.trim()
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
            println!("{}", config);
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

fn main() {
    let args: Vec<String> = std::env::args().collect();

    if args.len() < 3 {
        eprintln!("Usage: {} <spec_file> <data_dir> [options]", args[0]);
        eprintln!("Arguments:");
        eprintln!("  <spec_file>           Path to WDL SPEC.md file");
        eprintln!("  <data_dir>           Path to test data directory");
        eprintln!("Options:");
        eprintln!("  --list                List all tests");
        eprintln!("  --name <test_name>    Run specific test");
        eprintln!("  --pattern <pattern>   Run tests matching pattern");
        eprintln!("  --keep-files          Keep test files after execution");
        eprintln!("  --debug               Enable debug output");
        std::process::exit(1);
    }

    let spec_file = PathBuf::from(&args[1]);
    let data_dir = PathBuf::from(&args[2]);

    let mut runner = SpecTestRunner::new();
    let mut list_tests = false;
    let mut test_name: Option<String> = None;
    let mut pattern: Option<String> = None;

    let mut i = 3; // Start from index 3 since we now have spec_file and data_dir
    while i < args.len() {
        match args[i].as_str() {
            "--list" => list_tests = true,
            "--name" => {
                i += 1;
                if i < args.len() {
                    test_name = Some(args[i].clone());
                }
            }
            "--pattern" => {
                i += 1;
                if i < args.len() {
                    pattern = Some(args[i].clone());
                }
            }
            "--keep-files" => runner = runner.with_keep_files(true),
            "--debug" => runner = runner.with_debug(true),
            _ => {}
        }
        i += 1;
    }

    let result = if list_tests {
        runner.list_tests(&spec_file).map(|_| Vec::new())
    } else if let Some(name) = test_name {
        runner
            .run_single_test(&spec_file, &data_dir, &name)
            .map(|r| r.into_iter().collect())
    } else if let Some(pat) = pattern {
        runner.run_tests_matching(&spec_file, &data_dir, &pat)
    } else {
        runner.run_all_tests(&spec_file, &data_dir)
    };

    match result {
        Ok(results) => {
            if !results.is_empty() {
                let passed = results
                    .iter()
                    .filter(|r| r.status == TestStatus::Passed)
                    .count();
                let failed_tests: Vec<&TestResult> = results
                    .iter()
                    .filter(|r| r.status == TestStatus::Failed)
                    .collect();
                let error_tests: Vec<&TestResult> = results
                    .iter()
                    .filter(|r| r.status == TestStatus::Error)
                    .collect();

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

                println!("\n=== Summary ===");
                println!("Total: {}", results.len());
                println!("Passed: {}", passed);
                println!("Failed: {}", failed_tests.len());
                println!("Errors: {}", error_tests.len());

                if !failed_tests.is_empty() || !error_tests.is_empty() {
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
