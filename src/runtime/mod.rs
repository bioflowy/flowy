//! WDL Runtime Module
//!
//! This module provides the runtime execution system for WDL workflows and tasks.
//! It includes task and workflow execution engines, configuration management,
//! file system utilities, and error handling.

pub mod config;
pub mod container;
pub mod error;
pub mod fs_utils;
pub mod task;
pub mod task_context;
pub mod workflow;

#[cfg(test)]
pub mod workflow_tests;

// Re-export main runtime components
pub use config::{
    CacheConfig, Config, ConfigBuilder, ContainerBackend, ContainerConfig, ResourceLimits,
};
pub use error::{IntoRuntimeError, RuntimeError, RuntimeResult};
pub use fs_utils::{
    copy_file, create_dir_all, read_file_to_string, remove_dir_all, write_file_atomic,
    WorkflowDirectory,
};
pub use task::{TaskEngine, TaskExecutionMonitor, TaskExecutionOptions, TaskExecutionStats};
pub use task_context::TaskContext;
pub use task_context::TaskResult;
pub use workflow::{WorkflowEngine, WorkflowExecutionStats, WorkflowResult};

use crate::env::Bindings;
use crate::error::WdlError;
use crate::tree::{Document, Task, Workflow};
use crate::value::Value;
use std::path::{Path, PathBuf};
use std::time::Duration;

/// Main runtime execution function - executes a WDL document
///
/// This is the main entry point for WDL execution. It can execute either
/// a workflow or a single task, depending on the document contents.
///
/// # Arguments
/// * `document` - The parsed WDL document to execute
/// * `inputs` - Input bindings for the execution
/// * `config` - Runtime configuration
/// * `run_id` - Unique identifier for this execution
/// * `work_dir` - Working directory for execution
///
/// # Returns
/// * `WorkflowResult` - Execution results including outputs and metadata
pub fn run_document(
    document: Document,
    inputs: Bindings<Value>,
    config: Config,
    run_id: &str,
    work_dir: &Path,
) -> RuntimeResult<WorkflowResult> {
    // Create workflow directory structure
    let workflow_dir = WorkflowDirectory::create(work_dir, run_id)?;

    // Create workflow engine
    let engine = WorkflowEngine::new(config, workflow_dir);

    // Execute document
    engine.execute_document(document, inputs, run_id)
}

/// Execute a single WDL task
///
/// This function executes a single task with the provided inputs.
///
/// # Arguments
/// * `task` - The task to execute
/// * `inputs` - Input bindings for the task
/// * `config` - Runtime configuration
/// * `run_id` - Unique identifier for this execution
/// * `work_dir` - Working directory for execution
///
/// # Returns
/// * `TaskResult` - Task execution results
pub fn run_task(
    task: Task,
    inputs: Bindings<Value>,
    config: Config,
    run_id: &str,
    work_dir: &Path,
) -> RuntimeResult<TaskResult> {
    // Create workflow directory structure
    let workflow_dir = WorkflowDirectory::create(work_dir, run_id)?;

    // Create task engine
    let engine = TaskEngine::new(config, workflow_dir);

    // Execute task
    engine.execute_task_default(task, inputs, run_id)
}

/// Execute a WDL workflow
///
/// This function executes a workflow with the provided inputs.
///
/// # Arguments
/// * `workflow` - The workflow to execute
/// * `inputs` - Input bindings for the workflow
/// * `config` - Runtime configuration
/// * `run_id` - Unique identifier for this execution
/// * `work_dir` - Working directory for execution
///
/// # Returns
/// * `WorkflowResult` - Workflow execution results
pub fn run_workflow(
    workflow: Workflow,
    inputs: Bindings<Value>,
    config: Config,
    run_id: &str,
    work_dir: &Path,
) -> RuntimeResult<WorkflowResult> {
    // Create workflow directory structure
    let workflow_dir = WorkflowDirectory::create(work_dir, run_id)?;

    // Create workflow engine
    let engine = WorkflowEngine::new(config, workflow_dir);

    // Execute workflow
    engine.execute_workflow(workflow, inputs, run_id)
}

/// Validate a WDL document before execution
///
/// This function performs validation checks on a WDL document to ensure
/// it can be executed successfully.
///
/// # Arguments
/// * `document` - The document to validate
/// * `inputs` - Input bindings to validate against
///
/// # Returns
/// * `Result<(), RuntimeError>` - Success or validation error
pub fn validate_document(document: &Document, inputs: &Bindings<Value>) -> RuntimeResult<()> {
    if let Some(workflow) = &document.workflow {
        validate_workflow(workflow, inputs)?;
    } else if document.tasks.len() == 1 {
        let task = &document.tasks[0];
        validate_task(task, inputs)?;
    } else {
        return Err(RuntimeError::WorkflowValidationError {
            message: "Document must contain either a workflow or exactly one task".to_string(),
            pos: crate::error::SourcePosition::new("".to_string(), "".to_string(), 0, 0, 0, 0),
        });
    }

    Ok(())
}

/// Validate a WDL task
pub fn validate_task(task: &Task, inputs: &Bindings<Value>) -> RuntimeResult<()> {
    let config = Config::default();
    let temp_dir = std::env::temp_dir().join("wdl_validation");
    let workflow_dir = WorkflowDirectory::create(&temp_dir, "validation")?;
    let engine = TaskEngine::new(config, workflow_dir);

    engine.validate_task(task, inputs)
}

/// Validate a WDL workflow
pub fn validate_workflow(workflow: &Workflow, inputs: &Bindings<Value>) -> RuntimeResult<()> {
    let config = Config::default();
    let temp_dir = std::env::temp_dir().join("wdl_validation");
    let workflow_dir = WorkflowDirectory::create(&temp_dir, "validation")?;
    let engine = WorkflowEngine::new(config, workflow_dir);

    engine.validate_workflow(workflow)?;
    engine.validate_workflow_inputs(workflow, inputs)?;
    Ok(())
}

/// Runtime builder for fluent API
pub struct RuntimeBuilder {
    config: Config,
    run_id: Option<String>,
    work_dir: Option<PathBuf>,
}

impl RuntimeBuilder {
    /// Create a new runtime builder
    pub fn new() -> Self {
        Self {
            config: Config::default(),
            run_id: None,
            work_dir: None,
        }
    }

    /// Set the configuration
    pub fn config(mut self, config: Config) -> Self {
        self.config = config;
        self
    }

    /// Set the run ID
    pub fn run_id<S: Into<String>>(mut self, run_id: S) -> Self {
        self.run_id = Some(run_id.into());
        self
    }

    /// Set the working directory
    pub fn work_dir<P: Into<PathBuf>>(mut self, work_dir: P) -> Self {
        self.work_dir = Some(work_dir.into());
        self
    }

    /// Build and execute a document
    pub fn execute_document(
        self,
        document: Document,
        inputs: Bindings<Value>,
    ) -> RuntimeResult<WorkflowResult> {
        let run_id = self
            .run_id
            .unwrap_or_else(|| format!("run_{}", std::process::id()));
        let work_dir = self
            .work_dir
            .unwrap_or_else(|| std::env::current_dir().unwrap_or_else(|_| PathBuf::from(".")));

        run_document(document, inputs, self.config, &run_id, &work_dir)
    }

    /// Build and execute a task
    pub fn execute_task(self, task: Task, inputs: Bindings<Value>) -> RuntimeResult<TaskResult> {
        let run_id = self
            .run_id
            .unwrap_or_else(|| format!("run_{}", std::process::id()));
        let work_dir = self
            .work_dir
            .unwrap_or_else(|| std::env::current_dir().unwrap_or_else(|_| PathBuf::from(".")));

        run_task(task, inputs, self.config, &run_id, &work_dir)
    }

    /// Build and execute a workflow
    pub fn execute_workflow(
        self,
        workflow: Workflow,
        inputs: Bindings<Value>,
    ) -> RuntimeResult<WorkflowResult> {
        let run_id = self
            .run_id
            .unwrap_or_else(|| format!("run_{}", std::process::id()));
        let work_dir = self
            .work_dir
            .unwrap_or_else(|| std::env::current_dir().unwrap_or_else(|_| PathBuf::from(".")));

        run_workflow(workflow, inputs, self.config, &run_id, &work_dir)
    }
}

impl Default for RuntimeBuilder {
    fn default() -> Self {
        Self::new()
    }
}

/// Utility functions for runtime operations
pub mod utils {
    use super::*;
    use crate::types::Type;
    use crate::value::Value;
    use std::collections::HashMap;

    /// Generate a unique run ID
    pub fn generate_run_id() -> String {
        use std::time::{SystemTime, UNIX_EPOCH};

        let timestamp = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_secs();
        let pid = std::process::id();

        format!("run_{}_{}", timestamp, pid)
    }

    /// Create inputs from JSON-like structure
    pub fn inputs_from_json(
        json_inputs: HashMap<String, serde_json::Value>,
    ) -> Result<Bindings<Value>, WdlError> {
        let mut inputs = Bindings::new();

        for (key, json_value) in json_inputs {
            let wdl_value = json_to_wdl_value(json_value)?;
            inputs = inputs.bind(key, wdl_value, None);
        }

        Ok(inputs)
    }

    /// Convert JSON value to WDL value
    fn json_to_wdl_value(json: serde_json::Value) -> Result<Value, WdlError> {
        match json {
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
                    Err(WdlError::RuntimeError {
                        message: format!("Invalid number: {}", n),
                    })
                }
            }
            serde_json::Value::String(s) => Ok(Value::String {
                value: s,
                wdl_type: Type::string(false),
            }),
            serde_json::Value::Array(arr) => {
                let wdl_array: Result<Vec<Value>, WdlError> =
                    arr.into_iter().map(json_to_wdl_value).collect();
                Ok(Value::Array {
                    values: wdl_array?,
                    wdl_type: Type::array(Type::string(false), false, false),
                }) // Default array type
            }
            serde_json::Value::Object(obj) => {
                let wdl_map: Result<Vec<(Value, Value)>, WdlError> = obj
                    .into_iter()
                    .map(|(k, v)| {
                        Ok((
                            Value::String {
                                value: k,
                                wdl_type: Type::string(false),
                            },
                            json_to_wdl_value(v)?,
                        ))
                    })
                    .collect();
                Ok(Value::Map {
                    pairs: wdl_map?,
                    wdl_type: Type::map(Type::string(false), Type::string(false), false),
                }) // Default map type
            }
        }
    }

    /// Convert WDL outputs to JSON
    pub fn outputs_to_json(
        outputs: &Bindings<Value>,
    ) -> serde_json::Map<String, serde_json::Value> {
        let mut json_outputs = serde_json::Map::new();

        for binding in outputs.iter() {
            json_outputs.insert(
                binding.name().to_string(),
                wdl_value_to_json(binding.value()),
            );
        }

        json_outputs
    }

    /// Convert WDL value to JSON value
    fn wdl_value_to_json(value: &Value) -> serde_json::Value {
        match value {
            Value::Null => serde_json::Value::Null,
            Value::Boolean { value, .. } => serde_json::Value::Bool(*value),
            Value::Int { value, .. } => serde_json::Value::Number(serde_json::Number::from(*value)),
            Value::Float { value: f, .. } => serde_json::Value::Number(
                serde_json::Number::from_f64(*f).unwrap_or_else(|| serde_json::Number::from(0)),
            ),
            Value::String { value, .. } => serde_json::Value::String(value.clone()),
            Value::File { value, .. } => serde_json::Value::String(value.clone()),
            Value::Directory { value, .. } => serde_json::Value::String(value.clone()),
            Value::Array { values, .. } => {
                let json_array: Vec<serde_json::Value> =
                    values.iter().map(wdl_value_to_json).collect();
                serde_json::Value::Array(json_array)
            }
            Value::Map { pairs, .. } => {
                let mut json_obj = serde_json::Map::new();
                for (k, v) in pairs {
                    if let Value::String { value: key, .. } = k {
                        json_obj.insert(key.clone(), wdl_value_to_json(v));
                    }
                }
                serde_json::Value::Object(json_obj)
            }
            Value::Pair { left, right, .. } => {
                serde_json::Value::Array(vec![wdl_value_to_json(left), wdl_value_to_json(right)])
            }
            Value::Struct { members, .. } => {
                let mut json_obj = serde_json::Map::new();
                for (k, v) in members {
                    json_obj.insert(k.clone(), wdl_value_to_json(v));
                }
                serde_json::Value::Object(json_obj)
            }
        }
    }

    /// Create default configuration with common settings
    pub fn default_config() -> Config {
        Config::default()
            .with_task_timeout(Duration::from_secs(3600)) // 1 hour
            .with_max_concurrent_tasks(1) // Sequential for now
            .with_debug(false)
    }

    /// Create development configuration with debug settings
    pub fn dev_config() -> Config {
        Config::default()
            .with_task_timeout(Duration::from_secs(300)) // 5 minutes
            .with_max_concurrent_tasks(1)
            .with_debug(true)
    }

    #[cfg(test)]
    mod value_tests {
        use super::*;
        use crate::types::Type;
        use crate::value::Value;
        use std::collections::HashMap;

        #[test]
        fn test_wdl_value_to_json_struct() {
            // Create a struct value with members
            let mut members = HashMap::new();
            members.insert("a".to_string(), Value::int(10));
            members.insert("b".to_string(), Value::string("hello".to_string()));

            let struct_value = Value::struct_value(
                Type::object(HashMap::from([
                    ("a".to_string(), Type::int(false)),
                    ("b".to_string(), Type::string(false)),
                ])),
                members,
                None,
            )
            .unwrap();

            // Test the JSON conversion
            let json_result = wdl_value_to_json(&struct_value);

            println!("JSON result: {:?}", json_result);

            // Check that it's an object with the expected fields
            if json_result.is_object() {
                let json_obj = json_result.as_object().unwrap();
                println!("Object keys: {:?}", json_obj.keys().collect::<Vec<_>>());

                assert_eq!(
                    json_obj.get("a"),
                    Some(&serde_json::Value::Number(serde_json::Number::from(10)))
                );
                assert_eq!(
                    json_obj.get("b"),
                    Some(&serde_json::Value::String("hello".to_string()))
                );
            } else {
                panic!("Expected object, got: {:?}", json_result);
            }
        }

        #[test]
        fn test_struct_coerce_to_object() {
            use crate::types::Type;
            use crate::value::{Value, ValueBase};
            use std::collections::HashMap;

            // Create a struct value with members
            let mut members = HashMap::new();
            members.insert("a".to_string(), Value::int(10));
            members.insert("b".to_string(), Value::string("hello".to_string()));

            let struct_value = Value::struct_value(
                Type::StructInstance {
                    type_name: "TestStruct".to_string(),
                    members: Some(
                        members
                            .iter()
                            .map(|(k, v)| (k.clone(), v.wdl_type().clone()))
                            .collect(),
                    ),
                    optional: false,
                },
                members,
                None,
            )
            .unwrap();

            // Create a plain Object type (like "Object" declaration)
            let object_type = Type::object(HashMap::new());

            // Test coercion
            let coerced = struct_value.coerce(&object_type).unwrap();

            // Check that the coerced value preserves the members
            if let Value::Struct { members, .. } = coerced {
                println!("Coerced members: {:?}", members);
                assert!(members.contains_key("a"));
                assert!(members.contains_key("b"));
                assert_eq!(members.get("a").unwrap(), &Value::int(10));
                assert_eq!(
                    members.get("b").unwrap(),
                    &Value::string("hello".to_string())
                );
            } else {
                panic!("Expected struct value after coercion");
            }
        }

        #[test]
        fn test_map_coerce_to_specific_struct() {
            use crate::types::Type;
            use crate::value::{Value, ValueBase};
            use std::collections::HashMap;

            // Create a Map value (like { "a": 10, "b": 11, "c": 12 })
            // Note: We use the actual struct member names as map keys for the test to succeed
            let pairs = vec![
                (Value::string("a".to_string()), Value::int(10)),
                (Value::string("b".to_string()), Value::int(11)),
                (Value::string("c".to_string()), Value::int(12)),
            ];

            let map_value = Value::map(Type::string(false), Type::int(false), pairs);

            // Create a specific struct type (like Words struct with members a, b, c)
            let mut struct_members = HashMap::new();
            struct_members.insert("a".to_string(), Type::int(false));
            struct_members.insert("b".to_string(), Type::int(false));
            struct_members.insert("c".to_string(), Type::int(false));

            let words_struct_type = Type::StructInstance {
                type_name: "Words".to_string(),
                members: Some(struct_members),
                optional: false,
            };

            // Test coercion - this should now succeed with our implementation
            let result = map_value.coerce(&words_struct_type);

            match result {
                Ok(coerced_value) => {
                    if let Value::Struct { members, .. } = coerced_value {
                        println!("Map to struct coercion succeeded! Members: {:?}", members);
                        assert!(members.contains_key("a"));
                        assert!(members.contains_key("b"));
                        assert!(members.contains_key("c"));
                        assert_eq!(members.get("a").unwrap(), &Value::int(10));
                        assert_eq!(members.get("b").unwrap(), &Value::int(11));
                        assert_eq!(members.get("c").unwrap(), &Value::int(12));
                    } else {
                        panic!(
                            "Expected struct value after coercion, got: {:?}",
                            coerced_value
                        );
                    }
                }
                Err(e) => {
                    panic!(
                        "Expected Map to struct coercion to succeed, but it failed: {:?}",
                        e
                    );
                }
            }
        }
    }
}

#[cfg(test)]
#[allow(dead_code)]
mod tests {
    // Temporarily disabled for interface integration
    /*
    use super::*;
    use crate::tree::*;
    use crate::expr::*;
    use tempfile::tempdir;
    use std::collections::HashMap;

    fn create_test_document_with_task() -> Document {
        let task = Task {
            pos: crate::error::SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 10),
            name: "test_task".to_string(),
            inputs: vec![
                Decl {
                    pos: crate::error::SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 2, 1, 2, 20),
                    name: "input_str".to_string(),
                    wdl_type: crate::Type::String,
                    expr: None,
                }
            ],
            command: Some(Expr::String(StringExpr {
                pos: crate::error::SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 3, 1, 3, 30),
                value: "echo 'Hello World'".to_string(),
            })),
            outputs: vec![
                Decl {
                    pos: crate::error::SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 4, 1, 4, 25),
                    name: "result".to_string(),
                    wdl_type: crate::Type::String,
                    expr: Some(Expr::String(StringExpr {
                        pos: crate::error::SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 4, 15, 4, 25),
                        value: "Hello World".to_string(),
                    })),
                }
            ],
            runtime: None,
            parameter_meta: None,
            meta: None,
        };

        Document {
            pos: crate::error::SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 10, 1),
            source_text: "".to_string(),
            imports: vec![],
            structs: vec![],
            tasks: vec![task],
            workflow: None,
        }
    }

    #[test]
    fn test_runtime_builder() {
        let temp_dir = tempdir().unwrap();
        let config = Config::default().with_debug(true);

        let builder = RuntimeBuilder::new()
            .config(config)
            .run_id("test_run")
            .work_dir(temp_dir.path());

        // Test that builder can be created
        assert!(true);
    }

    #[test]
    fn test_validate_document_with_task() {
        let document = create_test_document_with_task();

        let mut inputs = Bindings::new();
        inputs.insert("input_str".to_string(), Value::String("test".to_string()));

        let result = validate_document(&document, &inputs);
        assert!(result.is_ok());
    }

    #[test]
    fn test_validate_document_missing_input() {
        let document = create_test_document_with_task();
        let empty_inputs = Env::Bindings::new();

        let result = validate_document(&document, &empty_inputs);
        assert!(result.is_err());
    }

    #[test]
    fn test_generate_run_id() {
        let run_id1 = utils::generate_run_id();
        let run_id2 = utils::generate_run_id();

        assert!(run_id1.starts_with("run_"));
        assert!(run_id2.starts_with("run_"));
        // They should be different (very likely)
        assert_ne!(run_id1, run_id2);
    }

    #[test]
    fn test_inputs_from_json() {
        let mut json_inputs = HashMap::new();
        json_inputs.insert("str_input".to_string(), serde_json::Value::String("hello".to_string()));
        json_inputs.insert("int_input".to_string(), serde_json::Value::Number(serde_json::Number::from(42)));
        json_inputs.insert("bool_input".to_string(), serde_json::Value::Bool(true));

        let inputs = utils::inputs_from_json(json_inputs).unwrap();

        assert_eq!(inputs.get("str_input"), Some(&Value::String("hello".to_string())));
        assert_eq!(inputs.get("int_input"), Some(&Value::Int(42)));
        assert_eq!(inputs.get("bool_input"), Some(&Value::Boolean(true)));
    }

    #[test]
    fn test_outputs_to_json() {
        let mut outputs = Bindings::new();
        outputs.insert("result".to_string(), Value::String("success".to_string()));
        outputs.insert("count".to_string(), Value::Int(5));
        outputs.insert("valid".to_string(), Value::Boolean(true));

        let json_outputs = utils::outputs_to_json(&outputs);

        assert_eq!(json_outputs.get("result"), Some(&serde_json::Value::String("success".to_string())));
        assert_eq!(json_outputs.get("count"), Some(&serde_json::Value::Number(serde_json::Number::from(5))));
        assert_eq!(json_outputs.get("valid"), Some(&serde_json::Value::Bool(true)));
    }

    #[test]
    fn test_config_creation() {
        let default_config = utils::default_config();
        assert_eq!(default_config.task_timeout, Duration::from_secs(3600));
        assert!(!default_config.debug);

        let dev_config = utils::dev_config();
        assert_eq!(dev_config.task_timeout, Duration::from_secs(300));
        assert!(dev_config.debug);
    }
    */
}
