use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// Represents a single test case extracted from the WDL specification
#[derive(Debug, Clone)]
pub struct SpecTestCase {
    /// Unique name/identifier for the test case
    pub name: String,
    /// WDL source code for this test
    pub wdl_source: String,
    /// Optional JSON input for the test
    pub input_json: Option<String>,
    /// Optional expected JSON output
    pub expected_output: Option<String>,
    /// Test category (e.g., "syntax", "task", "workflow", "stdlib")
    pub category: TestCategory,
    /// Additional metadata about the test
    pub metadata: TestMetadata,
}

/// Categories of specification tests
#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub enum TestCategory {
    /// Basic syntax and parsing tests
    Syntax,
    /// Type system tests
    Types,
    /// Expression evaluation tests
    Expressions,
    /// Task-related tests
    Tasks,
    /// Workflow-related tests
    Workflows,
    /// Standard library function tests
    StandardLibrary,
    /// Integration tests
    Integration,
    /// Tests that should fail (negative tests)
    ShouldFail,
}

/// Additional metadata for test cases
#[derive(Debug, Clone, Default)]
pub struct TestMetadata {
    /// Line number in the spec document where this test appears
    pub line_number: Option<usize>,
    /// Section of the specification this test belongs to
    pub section: Option<String>,
    /// Whether this test should be skipped
    pub skip: bool,
    /// Reason for skipping (if applicable)
    pub skip_reason: Option<String>,
    /// Expected WDL version for this test
    pub wdl_version: Option<String>,
}

/// Result of executing a specification test
#[derive(Debug, Clone)]
pub struct TestResult {
    /// Name of the test that was executed
    pub test_name: String,
    /// Status of the test execution
    pub status: TestStatus,
    /// Optional message with details about the result
    pub message: Option<String>,
    /// Actual output produced by the test (if any)
    pub actual_output: Option<String>,
    /// Expected output (if any)
    pub expected_output: Option<String>,
    /// Time taken to execute the test in milliseconds
    pub execution_time_ms: u64,
}

/// Status of a test execution
#[derive(Debug, Clone, PartialEq)]
pub enum TestStatus {
    /// Test passed successfully
    Passed,
    /// Test failed
    Failed,
    /// Test was skipped
    Skipped,
    /// Test execution encountered an error
    Error,
}

impl SpecTestCase {
    /// Create a new test case
    pub fn new(name: String, wdl_source: String, category: TestCategory) -> Self {
        Self {
            name,
            wdl_source,
            input_json: None,
            expected_output: None,
            category,
            metadata: TestMetadata::default(),
        }
    }

    /// Set the input JSON for this test case
    pub fn with_input(mut self, input_json: String) -> Self {
        self.input_json = Some(input_json);
        self
    }

    /// Set the expected output for this test case
    pub fn with_expected_output(mut self, expected_output: String) -> Self {
        self.expected_output = Some(expected_output);
        self
    }

    /// Set metadata for this test case
    pub fn with_metadata(mut self, metadata: TestMetadata) -> Self {
        self.metadata = metadata;
        self
    }

    /// Check if this test case should be skipped
    pub fn should_skip(&self) -> bool {
        self.metadata.skip
    }

    /// Get a description of why this test should be skipped
    pub fn skip_reason(&self) -> Option<&str> {
        self.metadata.skip_reason.as_deref()
    }
}

impl TestResult {
    /// Create a new passed test result
    pub fn passed(test_name: String, execution_time_ms: u64) -> Self {
        Self {
            test_name,
            status: TestStatus::Passed,
            message: None,
            actual_output: None,
            expected_output: None,
            execution_time_ms,
        }
    }

    /// Create a new failed test result
    pub fn failed(
        test_name: String,
        message: String,
        expected_output: Option<String>,
        actual_output: Option<String>,
        execution_time_ms: u64,
    ) -> Self {
        Self {
            test_name,
            status: TestStatus::Failed,
            message: Some(message),
            actual_output,
            expected_output,
            execution_time_ms,
        }
    }

    /// Create a new skipped test result
    pub fn skipped(test_name: String, reason: String) -> Self {
        Self {
            test_name,
            status: TestStatus::Skipped,
            message: Some(reason),
            actual_output: None,
            expected_output: None,
            execution_time_ms: 0,
        }
    }

    /// Create a new error test result
    pub fn error(test_name: String, error_message: String, execution_time_ms: u64) -> Self {
        Self {
            test_name,
            status: TestStatus::Error,
            message: Some(error_message),
            actual_output: None,
            expected_output: None,
            execution_time_ms,
        }
    }
}

impl std::fmt::Display for TestCategory {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            TestCategory::Syntax => write!(f, "syntax"),
            TestCategory::Types => write!(f, "types"),
            TestCategory::Expressions => write!(f, "expressions"),
            TestCategory::Tasks => write!(f, "tasks"),
            TestCategory::Workflows => write!(f, "workflows"),
            TestCategory::StandardLibrary => write!(f, "stdlib"),
            TestCategory::Integration => write!(f, "integration"),
            TestCategory::ShouldFail => write!(f, "should_fail"),
        }
    }
}

impl std::fmt::Display for TestStatus {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            TestStatus::Passed => write!(f, "PASSED"),
            TestStatus::Failed => write!(f, "FAILED"),
            TestStatus::Skipped => write!(f, "SKIPPED"),
            TestStatus::Error => write!(f, "ERROR"),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_spec_test_case_creation() {
        let test_case = SpecTestCase::new(
            "test_hello".to_string(),
            "task hello { command { echo 'hello' } }".to_string(),
            TestCategory::Tasks,
        );

        assert_eq!(test_case.name, "test_hello");
        assert_eq!(test_case.category, TestCategory::Tasks);
        assert!(!test_case.should_skip());
    }

    #[test]
    fn test_test_result_creation() {
        let result = TestResult::passed("test_example".to_string(), 100);
        assert_eq!(result.status, TestStatus::Passed);
        assert_eq!(result.execution_time_ms, 100);
    }
}
