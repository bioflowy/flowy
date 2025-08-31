use std::path::Path;

mod config;
mod data_loader;
mod executor;
mod spec_parser;
mod test_case;

pub use config::SpecTestConfig;
pub use executor::SpecTestExecutor;
pub use spec_parser::SpecParser;
pub use test_case::{SpecTestCase, TestResult, TestStatus};

/// Main entry point for running WDL specification tests
pub struct SpecTestRunner {
    config: SpecTestConfig,
    executor: SpecTestExecutor,
}

impl SpecTestRunner {
    /// Create a new spec test runner with default configuration
    pub fn new() -> Self {
        Self {
            config: SpecTestConfig::default(),
            executor: SpecTestExecutor::new(),
        }
    }

    /// Create a new spec test runner with custom configuration
    pub fn with_config(config: SpecTestConfig) -> Self {
        Self {
            executor: SpecTestExecutor::new(),
            config,
        }
    }

    /// Run all specification tests from the WDL spec document
    pub fn run_all_tests<P: AsRef<Path>>(
        &self,
        spec_path: P,
        test_data_dir: P,
    ) -> Result<Vec<TestResult>, Box<dyn std::error::Error>> {
        // Parse test cases from the specification document
        let parser = SpecParser::new();
        let test_cases = parser.parse_spec_file(spec_path)?;

        println!("Found {} test cases in specification", test_cases.len());

        // Execute all test cases
        let mut results = Vec::new();
        for test_case in test_cases {
            println!("Running test: {}", test_case.name);
            let result = self
                .executor
                .execute_test(&test_case, test_data_dir.as_ref())?;
            results.push(result);
        }

        self.print_summary(&results);
        Ok(results)
    }

    /// Run specific tests by name pattern
    pub fn run_tests_matching<P: AsRef<Path>>(
        &self,
        spec_path: P,
        test_data_dir: P,
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

        let mut results = Vec::new();
        for test_case in filtered_cases {
            self.print_test_details(&test_case);
            let result = self
                .executor
                .execute_test(&test_case, test_data_dir.as_ref())?;
            self.print_test_result(&result);
            results.push(result);
        }

        self.print_summary(&results);
        Ok(results)
    }

    /// Run a single test by exact name
    pub fn run_single_test<P: AsRef<Path>>(
        &self,
        spec_path: P,
        test_data_dir: P,
        test_name: &str,
    ) -> Result<Option<TestResult>, Box<dyn std::error::Error>> {
        let parser = SpecParser::new();
        let test_cases = parser.parse_spec_file(spec_path)?;

        if let Some(test_case) = test_cases.into_iter().find(|tc| tc.name == test_name) {
            println!("Running single test: {}", test_case.name);
            self.print_test_details(&test_case);

            let result = self
                .executor
                .execute_test(&test_case, test_data_dir.as_ref())?;
            self.print_test_result(&result);

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

    /// Print detailed information about a test case
    fn print_test_details(&self, test_case: &SpecTestCase) {
        println!("\n=== Test Details ===");
        println!("Name: {}", test_case.name);
        println!("Category: {}", test_case.category);

        if let Some(ref section) = test_case.metadata.section {
            println!("Section: {}", section);
        }

        if let Some(line_num) = test_case.metadata.line_number {
            println!("Line: {}", line_num);
        }

        println!("\nWDL Source:");
        println!("```wdl");
        println!("{}", test_case.wdl_source);
        println!("```");

        if let Some(ref input) = test_case.input_json {
            println!("\nInput JSON:");
            if let Ok(parsed) = serde_json::from_str::<serde_json::Value>(input) {
                println!(
                    "{}",
                    serde_json::to_string_pretty(&parsed).unwrap_or_else(|_| input.clone())
                );
            } else {
                println!("{}", input);
            }
        }

        if let Some(ref expected) = test_case.expected_output {
            println!("\nExpected Output:");
            if let Ok(parsed) = serde_json::from_str::<serde_json::Value>(expected) {
                println!(
                    "{}",
                    serde_json::to_string_pretty(&parsed).unwrap_or_else(|_| expected.clone())
                );
            } else {
                println!("{}", expected);
            }
        }

        if test_case.should_skip() {
            println!("\nStatus: SKIPPED");
            if let Some(reason) = test_case.skip_reason() {
                println!("Reason: {}", reason);
            }
        }
    }

    /// Print test execution result
    fn print_test_result(&self, result: &TestResult) {
        println!("\n=== Test Result ===");
        println!("Test: {}", result.test_name);
        println!("Status: {}", result.status);
        println!("Execution time: {}ms", result.execution_time_ms);

        if let Some(ref message) = result.message {
            println!("Message: {}", message);
        }

        if let Some(ref actual) = result.actual_output {
            println!("\nActual Output:");
            if let Ok(parsed) = serde_json::from_str::<serde_json::Value>(actual) {
                println!(
                    "{}",
                    serde_json::to_string_pretty(&parsed).unwrap_or_else(|_| actual.clone())
                );
            } else {
                println!("{}", actual);
            }
        }

        if result.status == TestStatus::Failed {
            if let Some(ref expected) = result.expected_output {
                println!("\nExpected Output:");
                if let Ok(parsed) = serde_json::from_str::<serde_json::Value>(expected) {
                    println!(
                        "{}",
                        serde_json::to_string_pretty(&parsed).unwrap_or_else(|_| expected.clone())
                    );
                } else {
                    println!("{}", expected);
                }
            }
        }
    }

    /// Print a summary of test results
    fn print_summary(&self, results: &[TestResult]) {
        let total = results.len();
        let passed = results
            .iter()
            .filter(|r| r.status == TestStatus::Passed)
            .count();
        let failed = results
            .iter()
            .filter(|r| r.status == TestStatus::Failed)
            .count();
        let skipped = results
            .iter()
            .filter(|r| r.status == TestStatus::Skipped)
            .count();

        println!("\n=== Test Summary ===");
        println!("Total:   {}", total);
        println!("Passed:  {}", passed);
        println!("Failed:  {}", failed);
        println!("Skipped: {}", skipped);

        if failed > 0 {
            println!("\nFailed tests:");
            for result in results.iter().filter(|r| r.status == TestStatus::Failed) {
                println!(
                    "  - {}: {}",
                    result.test_name,
                    result.message.as_deref().unwrap_or("No details")
                );
            }
        }
    }
}

impl Default for SpecTestRunner {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_spec_test_runner_creation() {
        let runner = SpecTestRunner::new();
        // Basic creation test - more comprehensive tests will be added as components are implemented
        assert!(true);
    }
}
