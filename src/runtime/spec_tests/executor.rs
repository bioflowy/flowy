use super::data_loader::TestFileManager;
use super::test_case::{SpecTestCase, TestCategory, TestResult, TestStatus};
use crate::env::Bindings;
use crate::parser;
use crate::runtime::{run_document, run_task, run_workflow, Config};
use crate::tree::Document;
use crate::value::Value;
use serde_json;
use std::path::Path;
use std::time::{Duration, Instant};

/// Executor for running WDL specification tests
pub struct SpecTestExecutor {
    config: Config,
}

impl SpecTestExecutor {
    /// Create a new test executor
    pub fn new() -> Self {
        Self {
            config: Config::default(),
        }
    }

    /// Execute a single specification test case
    pub fn execute_test(
        &self,
        test_case: &SpecTestCase,
        data_dir: &Path,
    ) -> Result<TestResult, Box<dyn std::error::Error>> {
        let start_time = Instant::now();

        // Skip tests that are marked for skipping
        if test_case.should_skip() {
            let reason = test_case
                .skip_reason()
                .unwrap_or("Test marked for skipping")
                .to_string();
            return Ok(TestResult::skipped(test_case.name.clone(), reason));
        }

        // Execute the test based on its category
        let result = match test_case.category {
            TestCategory::Syntax => self.execute_syntax_test(test_case, data_dir),
            TestCategory::Types => self.execute_type_test(test_case, data_dir),
            TestCategory::Expressions => self.execute_expression_test(test_case, data_dir),
            TestCategory::Tasks => self.execute_task_test(test_case, data_dir),
            TestCategory::Workflows => self.execute_workflow_test(test_case, data_dir),
            TestCategory::StandardLibrary => self.execute_stdlib_test(test_case, data_dir),
            TestCategory::Integration => self.execute_integration_test(test_case, data_dir),
            TestCategory::ShouldFail => self.execute_negative_test(test_case, data_dir),
        };

        let execution_time = start_time.elapsed().as_millis() as u64;

        match result {
            Ok(actual_output) => {
                if let Some(expected) = &test_case.expected_output {
                    self.compare_outputs(&test_case.name, expected, &actual_output, execution_time)
                } else {
                    // Test passed if it executed without error and no expected output was specified
                    Ok(TestResult::passed(test_case.name.clone(), execution_time))
                }
            }
            Err(e) => {
                if test_case.category == TestCategory::ShouldFail {
                    // For negative tests, an error is expected
                    Ok(TestResult::passed(test_case.name.clone(), execution_time))
                } else {
                    Ok(TestResult::error(
                        test_case.name.clone(),
                        e.to_string(),
                        execution_time,
                    ))
                }
            }
        }
    }

    /// Execute a syntax test (parsing only)
    fn execute_syntax_test(
        &self,
        test_case: &SpecTestCase,
        _data_dir: &Path,
    ) -> Result<String, Box<dyn std::error::Error>> {
        let _document = parser::parse_document(&test_case.wdl_source, "1.2")?;
        Ok("Syntax validation passed".to_string())
    }

    /// Execute a type system test
    fn execute_type_test(
        &self,
        test_case: &SpecTestCase,
        data_dir: &Path,
    ) -> Result<String, Box<dyn std::error::Error>> {
        // For type tests, we primarily check that parsing and type checking succeed
        let document = parser::parse_document(&test_case.wdl_source, "1.2")?;

        // If there's input/output, try to execute as well
        if test_case.input_json.is_some() || test_case.expected_output.is_some() {
            self.execute_document_with_io(&document, test_case, data_dir)
        } else {
            Ok("Type validation passed".to_string())
        }
    }

    /// Execute an expression test
    fn execute_expression_test(
        &self,
        test_case: &SpecTestCase,
        data_dir: &Path,
    ) -> Result<String, Box<dyn std::error::Error>> {
        let document = parser::parse_document(&test_case.wdl_source, "1.2")?;

        // Try to execute the document if it contains executable content
        if document.workflow.is_some() || !document.tasks.is_empty() {
            self.execute_document_with_io(&document, test_case, data_dir)
        } else {
            Ok("Expression parsing passed".to_string())
        }
    }

    /// Execute a task test
    fn execute_task_test(
        &self,
        test_case: &SpecTestCase,
        data_dir: &Path,
    ) -> Result<String, Box<dyn std::error::Error>> {
        let document = parser::parse_document(&test_case.wdl_source, "1.2")?;

        if let Some(task) = document.tasks.first() {
            self.execute_single_task(&document, task, test_case, data_dir)
        } else {
            Err("No task found in test case".into())
        }
    }

    /// Execute a workflow test
    fn execute_workflow_test(
        &self,
        test_case: &SpecTestCase,
        data_dir: &Path,
    ) -> Result<String, Box<dyn std::error::Error>> {
        let document = parser::parse_document(&test_case.wdl_source, "1.2")?;

        if let Some(workflow) = &document.workflow {
            self.execute_single_workflow(&document, workflow, test_case, data_dir)
        } else {
            Err("No workflow found in test case".into())
        }
    }

    /// Execute a standard library test
    fn execute_stdlib_test(
        &self,
        test_case: &SpecTestCase,
        data_dir: &Path,
    ) -> Result<String, Box<dyn std::error::Error>> {
        // Standard library tests are typically embedded in tasks or workflows
        let document = parser::parse_document(&test_case.wdl_source, "1.2")?;
        self.execute_document_with_io(&document, test_case, data_dir)
    }

    /// Execute an integration test
    fn execute_integration_test(
        &self,
        test_case: &SpecTestCase,
        data_dir: &Path,
    ) -> Result<String, Box<dyn std::error::Error>> {
        let document = parser::parse_document(&test_case.wdl_source, "1.2")?;
        self.execute_document_with_io(&document, test_case, data_dir)
    }

    /// Execute a negative test (expected to fail)
    fn execute_negative_test(
        &self,
        test_case: &SpecTestCase,
        data_dir: &Path,
    ) -> Result<String, Box<dyn std::error::Error>> {
        // For negative tests, we expect an error to occur
        let document = parser::parse_document(&test_case.wdl_source, "1.2")?;
        let result = self.execute_document_with_io(&document, test_case, data_dir);

        match result {
            Ok(_) => Err("Test was expected to fail but succeeded".into()),
            Err(e) => Ok(format!("Test failed as expected: {}", e)),
        }
    }

    /// Execute a document with input/output handling
    fn execute_document_with_io(
        &self,
        document: &Document,
        test_case: &SpecTestCase,
        data_dir: &Path,
    ) -> Result<String, Box<dyn std::error::Error>> {
        let mut file_manager = TestFileManager::new(data_dir);

        // Prepare input data
        let input_bindings = if let Some(input_json) = &test_case.input_json {
            let resolved_json = file_manager.loader.resolve_file_references(input_json)?;
            file_manager.prepare_test_files(Some(&resolved_json))?;
            self.parse_input_json(&resolved_json)?
        } else {
            Bindings::new()
        };

        // Create temporary working directory
        let temp_dir = std::env::temp_dir().join(format!("wdl_spec_test_{}", std::process::id()));
        std::fs::create_dir_all(&temp_dir)?;

        let run_id = format!("spec_test_{}", std::process::id());

        // Execute the document
        let workflow_result = run_document(
            document.clone(),
            input_bindings,
            self.config.clone(),
            &run_id,
            &temp_dir,
        )?;

        // Convert outputs to JSON string for comparison
        self.serialize_workflow_outputs(&workflow_result)
    }

    /// Execute a single task from a document
    fn execute_single_task(
        &self,
        document: &Document,
        task: &crate::tree::Task,
        test_case: &SpecTestCase,
        data_dir: &Path,
    ) -> Result<String, Box<dyn std::error::Error>> {
        let mut file_manager = TestFileManager::new(data_dir);

        // Prepare input data
        let input_bindings = if let Some(input_json) = &test_case.input_json {
            let resolved_json = file_manager.loader.resolve_file_references(input_json)?;
            file_manager.prepare_test_files(Some(&resolved_json))?;
            let mut bindings = self.parse_input_json(&resolved_json)?;

            // For single task execution, strip task name prefixes from input keys
            // e.g., "sum.ints" -> "ints" when executing task "sum"
            bindings = self.strip_task_prefixes(bindings, &task.name);
            bindings
        } else {
            Bindings::new()
        };

        // Create temporary working directory
        let temp_dir = std::env::temp_dir().join(format!("wdl_spec_test_{}", std::process::id()));
        std::fs::create_dir_all(&temp_dir)?;

        let run_id = format!("spec_test_{}", std::process::id());

        let task_result = run_task(
            task.clone(),
            input_bindings,
            self.config.clone(),
            &run_id,
            &temp_dir,
        )?;

        self.serialize_task_outputs(&task_result, &task.name)
    }

    /// Execute a single workflow from a document
    fn execute_single_workflow(
        &self,
        document: &Document,
        workflow: &crate::tree::Workflow,
        test_case: &SpecTestCase,
        data_dir: &Path,
    ) -> Result<String, Box<dyn std::error::Error>> {
        let mut file_manager = TestFileManager::new(data_dir);

        // Prepare input data
        let input_bindings = if let Some(input_json) = &test_case.input_json {
            let resolved_json = file_manager.loader.resolve_file_references(input_json)?;
            file_manager.prepare_test_files(Some(&resolved_json))?;
            self.parse_input_json(&resolved_json)?
        } else {
            Bindings::new()
        };

        // Create temporary working directory
        let temp_dir = std::env::temp_dir().join(format!("wdl_spec_test_{}", std::process::id()));
        std::fs::create_dir_all(&temp_dir)?;

        let run_id = format!("spec_test_{}", std::process::id());

        let workflow_result = run_workflow(
            workflow.clone(),
            input_bindings,
            self.config.clone(),
            &run_id,
            &temp_dir,
        )?;

        self.serialize_workflow_outputs(&workflow_result)
    }

    /// Parse JSON input string into Bindings
    fn parse_input_json(
        &self,
        json_str: &str,
    ) -> Result<Bindings<Value>, Box<dyn std::error::Error>> {
        let json_value: serde_json::Value = serde_json::from_str(json_str)?;
        let mut bindings = Bindings::new();

        if let serde_json::Value::Object(obj) = json_value {
            for (key, value) in obj {
                let wdl_value = self.json_to_wdl_value(value)?;
                bindings = bindings.bind(key, wdl_value, None);
            }
        }

        Ok(bindings)
    }

    /// Convert JSON value to WDL Value
    fn json_to_wdl_value(
        &self,
        json_value: serde_json::Value,
    ) -> Result<Value, Box<dyn std::error::Error>> {
        use crate::types::Type;
        match json_value {
            serde_json::Value::Null => Ok(Value::Null),
            serde_json::Value::Bool(b) => Ok(Value::Boolean {
                value: b,
                wdl_type: Type::boolean(false),
            }),
            serde_json::Value::Number(n) => {
                if let Some(i) = n.as_i64() {
                    Ok(Value::Int {
                        value: i,
                        wdl_type: Type::int(false),
                    })
                } else if let Some(f) = n.as_f64() {
                    Ok(Value::Float {
                        value: f,
                        wdl_type: Type::float(false),
                    })
                } else {
                    Err("Invalid number format".into())
                }
            }
            serde_json::Value::String(s) => Ok(Value::String {
                value: s,
                wdl_type: Type::string(false),
            }),
            serde_json::Value::Array(arr) => {
                let wdl_values: Result<Vec<Value>, _> = arr
                    .into_iter()
                    .map(|item| self.json_to_wdl_value(item))
                    .collect();
                Ok(Value::Array {
                    values: wdl_values?,
                    wdl_type: Type::array(Type::string(false), false, false),
                })
            }
            serde_json::Value::Object(obj) => {
                let wdl_pairs: Result<Vec<(Value, Value)>, Box<dyn std::error::Error>> = obj
                    .into_iter()
                    .map(|(key, value)| {
                        Ok((
                            Value::String {
                                value: key,
                                wdl_type: Type::string(false),
                            },
                            self.json_to_wdl_value(value)?,
                        ))
                    })
                    .collect();
                Ok(Value::Map {
                    pairs: wdl_pairs?,
                    wdl_type: Type::map(Type::string(false), Type::string(false), false),
                })
            }
        }
    }

    /// Serialize workflow outputs to JSON string
    fn serialize_workflow_outputs(
        &self,
        workflow_result: &crate::runtime::WorkflowResult,
    ) -> Result<String, Box<dyn std::error::Error>> {
        let mut json_obj = serde_json::Map::new();

        // Extract outputs from workflow result
        for binding in workflow_result.outputs.iter() {
            json_obj.insert(
                binding.name().to_string(),
                self.wdl_value_to_json(binding.value())?,
            );
        }

        Ok(serde_json::to_string_pretty(&serde_json::Value::Object(
            json_obj,
        ))?)
    }

    /// Strip task name prefixes from input binding keys
    /// e.g., "task_name.param" -> "param" when executing "task_name"
    fn strip_task_prefixes(&self, bindings: Bindings<Value>, task_name: &str) -> Bindings<Value> {
        let mut new_bindings = Bindings::new();
        let prefix = format!("{}.", task_name);

        for binding in bindings.iter() {
            let key = binding.name();
            let new_key = if key.starts_with(&prefix) {
                key.strip_prefix(&prefix).unwrap_or(key).to_string()
            } else {
                key.to_string()
            };
            new_bindings = new_bindings.bind(new_key, binding.value().clone(), None);
        }

        new_bindings
    }

    /// Serialize task outputs to JSON string with task name prefixes
    fn serialize_task_outputs(
        &self,
        task_result: &crate::runtime::TaskResult,
        task_name: &str,
    ) -> Result<String, Box<dyn std::error::Error>> {
        let mut json_obj = serde_json::Map::new();

        // Extract outputs from task result
        for binding in task_result.outputs.iter() {
            let prefixed_name = format!("{}.{}", task_name, binding.name());
            json_obj.insert(prefixed_name, self.wdl_value_to_json(binding.value())?);
        }

        Ok(serde_json::to_string_pretty(&serde_json::Value::Object(
            json_obj,
        ))?)
    }

    /// Convert WDL Value to JSON value
    fn wdl_value_to_json(
        &self,
        value: &Value,
    ) -> Result<serde_json::Value, Box<dyn std::error::Error>> {
        match value {
            Value::Null => Ok(serde_json::Value::Null),
            Value::Boolean { value: b, .. } => Ok(serde_json::Value::Bool(*b)),
            Value::Int { value: i, .. } => {
                Ok(serde_json::Value::Number(serde_json::Number::from(*i)))
            }
            Value::Float { value: f, .. } => Ok(serde_json::Value::Number(
                serde_json::Number::from_f64(*f).ok_or("Invalid float")?,
            )),
            Value::String { value: s, .. } => Ok(serde_json::Value::String(s.clone())),
            Value::File { value: path, .. } => Ok(serde_json::Value::String(path.clone())),
            Value::Directory { value: path, .. } => Ok(serde_json::Value::String(path.clone())),
            Value::Array { values, .. } => {
                let mut json_arr = Vec::new();
                for item in values {
                    json_arr.push(self.wdl_value_to_json(item)?);
                }
                Ok(serde_json::Value::Array(json_arr))
            }
            Value::Map { pairs, .. } => {
                let mut json_obj = serde_json::Map::new();
                for (k, v) in pairs {
                    let key_str = match k {
                        Value::String { value: s, .. } => s.clone(),
                        _ => format!("{:?}", k),
                    };
                    json_obj.insert(key_str, self.wdl_value_to_json(v)?);
                }
                Ok(serde_json::Value::Object(json_obj))
            }
            Value::Pair { left, right, .. } => {
                let mut pair_obj = serde_json::Map::new();
                pair_obj.insert("left".to_string(), self.wdl_value_to_json(left)?);
                pair_obj.insert("right".to_string(), self.wdl_value_to_json(right)?);
                Ok(serde_json::Value::Object(pair_obj))
            }
            Value::Struct { members, .. } => {
                let mut json_obj = serde_json::Map::new();
                for (field_name, field_value) in members {
                    json_obj.insert(field_name.clone(), self.wdl_value_to_json(field_value)?);
                }
                Ok(serde_json::Value::Object(json_obj))
            }
        }
    }

    /// Compare expected and actual outputs
    fn compare_outputs(
        &self,
        test_name: &str,
        expected: &str,
        actual: &str,
        execution_time: u64,
    ) -> Result<TestResult, Box<dyn std::error::Error>> {
        // Parse both as JSON for normalized comparison
        let expected_json: serde_json::Value = serde_json::from_str(expected)?;
        let actual_json: serde_json::Value = serde_json::from_str(actual)?;

        if expected_json == actual_json {
            Ok(TestResult::passed(test_name.to_string(), execution_time))
        } else {
            let message = format!(
                "Output mismatch:\nExpected: {}\nActual: {}",
                serde_json::to_string_pretty(&expected_json)?,
                serde_json::to_string_pretty(&actual_json)?
            );
            Ok(TestResult::failed(
                test_name.to_string(),
                message,
                Some(expected.to_string()),
                Some(actual.to_string()),
                execution_time,
            ))
        }
    }
}

impl Default for SpecTestExecutor {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::runtime::spec_tests::test_case::TestMetadata;
    use crate::types::Type;

    #[test]
    fn test_executor_creation() {
        let executor = SpecTestExecutor::new();
        // Basic creation test
        assert!(true);
    }

    #[test]
    fn test_json_conversion() {
        let executor = SpecTestExecutor::new();

        let json_val = serde_json::json!("hello");
        let wdl_val = executor.json_to_wdl_value(json_val).unwrap();
        assert_eq!(
            wdl_val,
            Value::String {
                value: "hello".to_string(),
                wdl_type: Type::string(false),
            }
        );

        let converted_back = executor.wdl_value_to_json(&wdl_val).unwrap();
        assert_eq!(
            converted_back,
            serde_json::Value::String("hello".to_string())
        );
    }
}
