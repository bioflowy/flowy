use super::test_case::{SpecTestCase, TestCategory, TestMetadata};
use regex::Regex;
use std::fs;
use std::path::Path;

/// Parser for extracting test cases from WDL specification documents
pub struct SpecParser {
    wdl_code_regex: Regex,
    input_json_regex: Regex,
    output_json_regex: Regex,
    details_start_regex: Regex,
    details_end_regex: Regex,
}

impl SpecParser {
    /// Create a new specification parser
    pub fn new() -> Self {
        Self {
            // Regex to match WDL code blocks
            wdl_code_regex: Regex::new(r"```wdl\s*\n(.*?)\n```").unwrap(),
            // Regex to match example input JSON (flexible with HTML and whitespace)
            input_json_regex: Regex::new(r"(?s)Example input:.*?```json\s*\n(.*?)\n```").unwrap(),
            // Regex to match example output JSON (flexible with HTML and whitespace)
            output_json_regex: Regex::new(r"(?s)Example output:.*?```json\s*\n(.*?)\n```").unwrap(),
            // Regex to match HTML details start tags
            details_start_regex: Regex::new(r"<details[^>]*>").unwrap(),
            // Regex to match HTML details end tags
            details_end_regex: Regex::new(r"</details>").unwrap(),
        }
    }

    /// Parse a WDL specification file and extract test cases
    pub fn parse_spec_file<P: AsRef<Path>>(
        &self,
        spec_path: P,
    ) -> Result<Vec<SpecTestCase>, Box<dyn std::error::Error>> {
        let content = fs::read_to_string(spec_path.as_ref())?;
        self.parse_spec_content(&content)
    }

    /// Parse WDL specification content and extract test cases
    pub fn parse_spec_content(
        &self,
        content: &str,
    ) -> Result<Vec<SpecTestCase>, Box<dyn std::error::Error>> {
        let mut test_cases = Vec::new();
        let lines: Vec<&str> = content.lines().collect();

        // Find all WDL code blocks with their line numbers and surrounding context
        for (line_num, line) in lines.iter().enumerate() {
            if line.contains("```wdl") {
                if let Some(test_case) = self.extract_test_case_at_line(&lines, line_num)? {
                    test_cases.push(test_case);
                }
            }
        }

        Ok(test_cases)
    }

    /// Extract a test case starting at the given line number
    fn extract_test_case_at_line(
        &self,
        lines: &[&str],
        start_line: usize,
    ) -> Result<Option<SpecTestCase>, Box<dyn std::error::Error>> {
        // Find the WDL code block
        let wdl_source = match self.extract_wdl_code_block(lines, start_line) {
            Some(source) => source,
            None => return Ok(None),
        };

        // Generate a test name based on line number and content preview
        let test_name = self.generate_test_name(&wdl_source, start_line);

        // Determine test category based on content and context
        let category = self.determine_test_category(&wdl_source, lines, start_line);

        // Find the entire <details> block for this test case
        let (details_start, details_end) = self.find_details_block(lines, start_line);
        let full_context = lines[details_start..details_end].join("\n");

        // Extract input JSON if present
        let input_json = self.extract_example_input(&full_context);

        // Extract expected output JSON if present
        let expected_output = self.extract_example_output(&full_context);

        // Create metadata
        let metadata = TestMetadata {
            line_number: Some(start_line + 1), // 1-based line numbers
            section: self.determine_section(lines, start_line),
            skip: self.should_skip_test(&wdl_source, &category),
            skip_reason: None,
            wdl_version: self.extract_wdl_version(&wdl_source),
        };

        let mut test_case = SpecTestCase::new(test_name, wdl_source, category);

        if let Some(input) = input_json {
            test_case = test_case.with_input(input);
        }

        if let Some(output) = expected_output {
            test_case = test_case.with_expected_output(output);
        }

        test_case = test_case.with_metadata(metadata);

        Ok(Some(test_case))
    }

    /// Extract WDL code from a code block starting at the given line
    fn extract_wdl_code_block(&self, lines: &[&str], start_line: usize) -> Option<String> {
        if start_line >= lines.len() || !lines[start_line].contains("```wdl") {
            return None;
        }

        let mut wdl_lines = Vec::new();
        let mut in_code_block = false;

        for line in &lines[start_line..] {
            if line.contains("```wdl") {
                in_code_block = true;
                continue;
            }

            if in_code_block {
                if line.contains("```") {
                    break;
                }
                wdl_lines.push(*line);
            }
        }

        if wdl_lines.is_empty() {
            None
        } else {
            Some(wdl_lines.join("\n"))
        }
    }

    /// Extract JSON content using the provided regex
    fn extract_json_block(&self, context: &str, regex: &Regex) -> Option<String> {
        regex
            .captures(context)
            .and_then(|caps| caps.get(1).map(|m| m.as_str().trim().to_string()))
    }

    /// Generate a test name based on WDL source and line number
    fn generate_test_name(&self, wdl_source: &str, line_number: usize) -> String {
        // Try to extract a meaningful name from the WDL source
        if let Some(task_name) = self.extract_task_name(wdl_source) {
            format!("spec_line_{}_task_{}", line_number + 1, task_name)
        } else if let Some(workflow_name) = self.extract_workflow_name(wdl_source) {
            format!("spec_line_{}_workflow_{}", line_number + 1, workflow_name)
        } else if wdl_source.contains("struct") {
            format!("spec_line_{}_struct", line_number + 1)
        } else {
            format!("spec_line_{}", line_number + 1)
        }
    }

    /// Extract task name from WDL source
    fn extract_task_name(&self, wdl_source: &str) -> Option<String> {
        let task_regex = Regex::new(r"task\s+(\w+)").unwrap();
        task_regex
            .captures(wdl_source)
            .and_then(|caps| caps.get(1).map(|m| m.as_str().to_string()))
    }

    /// Extract workflow name from WDL source
    fn extract_workflow_name(&self, wdl_source: &str) -> Option<String> {
        let workflow_regex = Regex::new(r"workflow\s+(\w+)").unwrap();
        workflow_regex
            .captures(wdl_source)
            .and_then(|caps| caps.get(1).map(|m| m.as_str().to_string()))
    }

    /// Determine the test category based on WDL content and context
    fn determine_test_category(
        &self,
        wdl_source: &str,
        lines: &[&str],
        line_number: usize,
    ) -> TestCategory {
        // Check for negative test indicators
        let context_start = line_number.saturating_sub(5);
        let context_end = std::cmp::min(line_number + 10, lines.len());
        let surrounding_text = lines[context_start..context_end].join(" ").to_lowercase();

        if surrounding_text.contains("invalid")
            || surrounding_text.contains("error")
            || surrounding_text.contains("fail")
            || surrounding_text.contains("not allowed")
        {
            return TestCategory::ShouldFail;
        }

        // Categorize based on WDL content
        if wdl_source.contains("task") && wdl_source.contains("workflow") {
            TestCategory::Integration
        } else if wdl_source.contains("workflow") {
            TestCategory::Workflows
        } else if wdl_source.contains("task") {
            TestCategory::Tasks
        } else if wdl_source.contains("struct") {
            TestCategory::Types
        } else if self.contains_stdlib_functions(wdl_source) {
            TestCategory::StandardLibrary
        } else if self.contains_complex_expressions(wdl_source) {
            TestCategory::Expressions
        } else if self.contains_type_definitions(wdl_source) {
            TestCategory::Types
        } else {
            TestCategory::Syntax
        }
    }

    /// Check if WDL source contains standard library function calls
    fn contains_stdlib_functions(&self, wdl_source: &str) -> bool {
        let stdlib_functions = [
            "read_lines",
            "read_string",
            "read_int",
            "read_float",
            "read_boolean",
            "write_lines",
            "write_map",
            "write_object",
            "write_json",
            "write_tsv",
            "stdout",
            "stderr",
            "glob",
            "size",
            "basename",
            "dirname",
            "floor",
            "ceil",
            "round",
            "select_first",
            "select_all",
            "defined",
            "length",
            "flatten",
            "prefix",
            "suffix",
            "sub",
            "gsub",
            "sep",
            "transpose",
            "zip",
            "cross",
            "range",
            "unzip",
        ];

        stdlib_functions
            .iter()
            .any(|func| wdl_source.contains(func))
    }

    /// Check if WDL source contains complex expressions
    fn contains_complex_expressions(&self, wdl_source: &str) -> bool {
        wdl_source.contains("if") && wdl_source.contains("then") && wdl_source.contains("else")
            || wdl_source.contains("${")
            || wdl_source.contains("~{")
            || wdl_source.contains("[") && wdl_source.contains("]")
            || wdl_source.contains(".") // member access
    }

    /// Check if WDL source contains type definitions
    fn contains_type_definitions(&self, wdl_source: &str) -> bool {
        let type_keywords = [
            "Array",
            "Map",
            "Pair",
            "Object",
            "String",
            "Int",
            "Float",
            "Boolean",
            "File",
            "Directory",
        ];
        type_keywords.iter().any(|t| wdl_source.contains(t))
    }

    /// Determine which section of the spec this test belongs to
    fn determine_section(&self, lines: &[&str], line_number: usize) -> Option<String> {
        // Look backwards for the most recent heading
        for i in (0..line_number).rev() {
            let line = lines[i];
            if line.starts_with("#") {
                return Some(line.trim_start_matches("#").trim().to_string());
            }
        }
        None
    }

    /// Determine if a test should be skipped based on its content
    fn should_skip_test(&self, wdl_source: &str, category: &TestCategory) -> bool {
        // Skip tests that require features not yet implemented
        if wdl_source.contains("import") || wdl_source.contains("hints") {
            return true;
        }

        // Skip placeholder tests or incomplete examples
        if wdl_source.contains("...") || wdl_source.len() < 10 {
            return true;
        }

        false
    }

    /// Extract WDL version from source code
    fn extract_wdl_version(&self, wdl_source: &str) -> Option<String> {
        let version_regex = Regex::new(r"version\s+([\d.]+)").unwrap();
        version_regex
            .captures(wdl_source)
            .and_then(|caps| caps.get(1).map(|m| m.as_str().to_string()))
    }

    /// Find the details block that contains the given line number
    fn find_details_block(&self, lines: &[&str], start_line: usize) -> (usize, usize) {
        let mut details_start = start_line;
        let mut details_end = lines.len();

        // Look backwards for the start of the details block
        for i in (0..=start_line).rev() {
            if self.details_start_regex.is_match(lines[i]) {
                details_start = i;
                break;
            }
        }

        // Look forwards for the end of the details block
        for (i, line) in lines.iter().enumerate().skip(start_line) {
            if self.details_end_regex.is_match(line) {
                details_end = i + 1;
                break;
            }
        }

        (details_start, details_end)
    }

    /// Extract example input JSON from the full context
    fn extract_example_input(&self, full_context: &str) -> Option<String> {
        self.extract_json_block(full_context, &self.input_json_regex)
    }

    /// Extract example output JSON from the full context  
    fn extract_example_output(&self, full_context: &str) -> Option<String> {
        self.extract_json_block(full_context, &self.output_json_regex)
    }
}

impl Default for SpecParser {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_spec_parser_creation() {
        let parser = SpecParser::new();
        // Basic creation test
        assert!(true);
    }

    #[test]
    fn test_extract_task_name() {
        let parser = SpecParser::new();
        let wdl_source = "task hello_world { command { echo 'hello' } }";
        let task_name = parser.extract_task_name(wdl_source);
        assert_eq!(task_name, Some("hello_world".to_string()));
    }

    #[test]
    fn test_determine_test_category() {
        let parser = SpecParser::new();
        let lines = vec!["", "This is a task example", ""];

        // Test task categorization
        let task_source = "task example { command { echo 'test' } }";
        let category = parser.determine_test_category(task_source, &lines, 1);
        assert_eq!(category, TestCategory::Tasks);

        // Test workflow categorization
        let workflow_source = "workflow example { call some_function }";
        let category = parser.determine_test_category(workflow_source, &lines, 1);
        assert_eq!(category, TestCategory::Workflows);
    }

    #[test]
    fn test_should_skip_test() {
        let parser = SpecParser::new();

        // Should skip tests with imports
        assert!(parser.should_skip_test("import 'other.wdl'", &TestCategory::Syntax));

        // Should skip tests with incomplete content
        assert!(parser.should_skip_test("...", &TestCategory::Syntax));

        // Should not skip valid tests
        assert!(!parser.should_skip_test(
            "task test { command { echo 'hello' } }",
            &TestCategory::Tasks
        ));
    }
}
