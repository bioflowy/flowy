use super::test_case::TestCategory;
use std::collections::HashSet;

/// Configuration for specification test execution
#[derive(Debug, Clone)]
pub struct SpecTestConfig {
    /// Categories of tests to include (empty = include all)
    pub include_categories: HashSet<TestCategory>,
    /// Categories of tests to exclude
    pub exclude_categories: HashSet<TestCategory>,
    /// Test name patterns to include (empty = include all)
    pub include_patterns: Vec<String>,
    /// Test name patterns to exclude
    pub exclude_patterns: Vec<String>,
    /// Whether to run tests that are expected to fail
    pub run_negative_tests: bool,
    /// Whether to stop execution on first failure
    pub fail_fast: bool,
    /// Maximum number of tests to run (0 = no limit)
    pub max_tests: usize,
    /// Timeout for individual test execution in milliseconds
    pub test_timeout_ms: u64,
    /// Whether to show verbose output during test execution
    pub verbose: bool,
    /// Whether to show detailed diff for failed tests
    pub show_diffs: bool,
    /// Working directory for test execution
    pub working_dir: Option<String>,
}

impl Default for SpecTestConfig {
    fn default() -> Self {
        Self {
            include_categories: HashSet::new(),
            exclude_categories: HashSet::new(),
            include_patterns: Vec::new(),
            exclude_patterns: Vec::new(),
            run_negative_tests: true,
            fail_fast: false,
            max_tests: 0,
            test_timeout_ms: 30_000, // 30 seconds default timeout
            verbose: false,
            show_diffs: true,
            working_dir: None,
        }
    }
}

impl SpecTestConfig {
    /// Create a new configuration with default settings
    pub fn new() -> Self {
        Self::default()
    }

    /// Include only tests from specific categories
    pub fn include_categories(mut self, categories: Vec<TestCategory>) -> Self {
        self.include_categories = categories.into_iter().collect();
        self
    }

    /// Exclude tests from specific categories
    pub fn exclude_categories(mut self, categories: Vec<TestCategory>) -> Self {
        self.exclude_categories = categories.into_iter().collect();
        self
    }

    /// Include only tests matching specific name patterns
    pub fn include_patterns(mut self, patterns: Vec<String>) -> Self {
        self.include_patterns = patterns;
        self
    }

    /// Exclude tests matching specific name patterns
    pub fn exclude_patterns(mut self, patterns: Vec<String>) -> Self {
        self.exclude_patterns = patterns;
        self
    }

    /// Set whether to run negative tests (tests expected to fail)
    pub fn with_negative_tests(mut self, run_negative: bool) -> Self {
        self.run_negative_tests = run_negative;
        self
    }

    /// Set fail-fast behavior
    pub fn with_fail_fast(mut self, fail_fast: bool) -> Self {
        self.fail_fast = fail_fast;
        self
    }

    /// Set maximum number of tests to run
    pub fn with_max_tests(mut self, max_tests: usize) -> Self {
        self.max_tests = max_tests;
        self
    }

    /// Set test execution timeout
    pub fn with_timeout(mut self, timeout_ms: u64) -> Self {
        self.test_timeout_ms = timeout_ms;
        self
    }

    /// Set verbose output
    pub fn with_verbose(mut self, verbose: bool) -> Self {
        self.verbose = verbose;
        self
    }

    /// Set whether to show diffs for failed tests
    pub fn with_show_diffs(mut self, show_diffs: bool) -> Self {
        self.show_diffs = show_diffs;
        self
    }

    /// Set working directory for test execution
    pub fn with_working_dir<S: Into<String>>(mut self, working_dir: S) -> Self {
        self.working_dir = Some(working_dir.into());
        self
    }

    /// Check if a test should be included based on configuration
    pub fn should_include_test(&self, test_name: &str, category: &TestCategory) -> bool {
        // Check category inclusion/exclusion
        if !self.include_categories.is_empty() && !self.include_categories.contains(category) {
            return false;
        }

        if self.exclude_categories.contains(category) {
            return false;
        }

        // Check negative test inclusion
        if *category == TestCategory::ShouldFail && !self.run_negative_tests {
            return false;
        }

        // Check name pattern inclusion
        if !self.include_patterns.is_empty() {
            let matches_include = self
                .include_patterns
                .iter()
                .any(|pattern| test_name.contains(pattern));
            if !matches_include {
                return false;
            }
        }

        // Check name pattern exclusion
        if self
            .exclude_patterns
            .iter()
            .any(|pattern| test_name.contains(pattern))
        {
            return false;
        }

        true
    }
}

/// Builder for creating test configurations with a fluent interface
pub struct SpecTestConfigBuilder {
    config: SpecTestConfig,
}

impl SpecTestConfigBuilder {
    pub fn new() -> Self {
        Self {
            config: SpecTestConfig::default(),
        }
    }

    pub fn include_category(mut self, category: TestCategory) -> Self {
        self.config.include_categories.insert(category);
        self
    }

    pub fn exclude_category(mut self, category: TestCategory) -> Self {
        self.config.exclude_categories.insert(category);
        self
    }

    pub fn include_pattern<S: Into<String>>(mut self, pattern: S) -> Self {
        self.config.include_patterns.push(pattern.into());
        self
    }

    pub fn exclude_pattern<S: Into<String>>(mut self, pattern: S) -> Self {
        self.config.exclude_patterns.push(pattern.into());
        self
    }

    pub fn fail_fast(mut self) -> Self {
        self.config.fail_fast = true;
        self
    }

    pub fn verbose(mut self) -> Self {
        self.config.verbose = true;
        self
    }

    pub fn max_tests(mut self, max: usize) -> Self {
        self.config.max_tests = max;
        self
    }

    pub fn timeout_ms(mut self, timeout: u64) -> Self {
        self.config.test_timeout_ms = timeout;
        self
    }

    pub fn build(self) -> SpecTestConfig {
        self.config
    }
}

impl Default for SpecTestConfigBuilder {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_default_config() {
        let config = SpecTestConfig::default();
        assert!(config.include_categories.is_empty());
        assert!(config.exclude_categories.is_empty());
        assert!(config.run_negative_tests);
        assert!(!config.fail_fast);
    }

    #[test]
    fn test_config_builder() {
        let config = SpecTestConfigBuilder::new()
            .include_category(TestCategory::Tasks)
            .exclude_category(TestCategory::ShouldFail)
            .verbose()
            .max_tests(10)
            .build();

        assert!(config.include_categories.contains(&TestCategory::Tasks));
        assert!(config
            .exclude_categories
            .contains(&TestCategory::ShouldFail));
        assert!(config.verbose);
        assert_eq!(config.max_tests, 10);
    }

    #[test]
    fn test_should_include_test() {
        let config = SpecTestConfig::default()
            .include_categories(vec![TestCategory::Tasks])
            .exclude_patterns(vec!["skip".to_string()]);

        assert!(config.should_include_test("test_task", &TestCategory::Tasks));
        assert!(!config.should_include_test("test_workflow", &TestCategory::Workflows));
        assert!(!config.should_include_test("skip_this_test", &TestCategory::Tasks));
    }
}
