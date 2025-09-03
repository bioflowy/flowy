//! Workflow execution engine
//!
//! This module provides workflow-level execution capabilities, coordinating
//! task execution and managing data flow between tasks.

use crate::env::Bindings;
use crate::error::{SourcePosition, WdlError};
use crate::expr::ExpressionBase;
use crate::runtime::config::Config;
use crate::runtime::error::{RuntimeError, RuntimeResult};
use crate::runtime::fs_utils::WorkflowDirectory;
use crate::runtime::task::TaskEngine;
use crate::runtime::task_context::TaskResult;
use crate::tree::{Call, Conditional, Document, Scatter, Workflow, WorkflowElement};
use crate::types::Type;
use crate::value::{Value, ValueBase};
use std::collections::HashMap;
use std::path::PathBuf;
use std::time::{Duration, Instant};

/// Workflow execution engine
pub struct WorkflowEngine {
    /// Task execution engine
    task_engine: TaskEngine,
    /// Configuration
    #[allow(dead_code)]
    config: Config,
    /// Base workflow directory
    workflow_dir: WorkflowDirectory,
    /// Document with task definitions (optional)
    document: Option<Document>,
}

/// Workflow execution result
#[derive(Debug)]
pub struct WorkflowResult {
    /// Output bindings from workflow execution
    pub outputs: Bindings<Value>,
    /// Execution duration
    pub duration: Duration,
    /// Task execution results
    pub task_results: HashMap<String, TaskResult>,
    /// Working directory used
    pub work_dir: PathBuf,
}

/// Workflow execution context
#[derive(Debug)]
struct WorkflowContext {
    /// Current variable bindings
    bindings: Bindings<Value>,
    /// Task results by call name
    task_results: HashMap<String, TaskResult>,
    /// Execution start time
    start_time: Instant,
}

impl WorkflowEngine {
    /// Create a new workflow engine
    pub fn new(config: Config, workflow_dir: WorkflowDirectory) -> Self {
        let task_engine = TaskEngine::new(config.clone(), workflow_dir.clone());
        Self {
            task_engine,
            config,
            workflow_dir,
            document: None,
        }
    }

    /// Create a new workflow engine with document
    pub fn new_with_document(
        config: Config,
        workflow_dir: WorkflowDirectory,
        document: Document,
    ) -> Self {
        let task_engine = TaskEngine::new(config.clone(), workflow_dir.clone());
        Self {
            task_engine,
            config,
            workflow_dir,
            document: Some(document),
        }
    }

    /// Execute a workflow
    pub fn execute_workflow(
        &self,
        workflow: Workflow,
        inputs: Bindings<Value>,
        run_id: &str,
    ) -> RuntimeResult<WorkflowResult> {
        let start_time = Instant::now();

        // Validate workflow inputs
        self.validate_workflow_inputs(&workflow, &inputs)?;

        // Resolve workflow inputs (prefer prefixed names)
        let resolved_inputs = self.resolve_workflow_inputs(&workflow, inputs)?;

        // Create execution context
        let mut context = WorkflowContext {
            bindings: resolved_inputs,
            task_results: HashMap::new(),
            start_time,
        };

        // Execute workflow body
        self.execute_workflow_body(&workflow, &mut context, run_id)?;

        // Create stdlib for output evaluation
        let stdlib = crate::stdlib::StdLib::new("1.0");

        // Collect workflow outputs
        let outputs = self.collect_workflow_outputs(&workflow, &context, &stdlib)?;

        let duration = start_time.elapsed();

        Ok(WorkflowResult {
            outputs,
            duration,
            task_results: context.task_results,
            work_dir: self.workflow_dir.work.clone(),
        })
    }

    /// Execute a complete WDL document
    pub fn execute_document(
        &self,
        document: Document,
        inputs: Bindings<Value>,
        run_id: &str,
    ) -> RuntimeResult<WorkflowResult> {
        // Create engine with document for task resolution
        let engine_with_doc = Self::new_with_document(
            self.config.clone(),
            self.workflow_dir.clone(),
            document.clone(),
        );

        // Find the main workflow
        if let Some(workflow) = document.workflow {
            engine_with_doc.execute_workflow(workflow, inputs, run_id)
        } else {
            // If no workflow, try to find a single task to execute
            if document.tasks.len() == 1 {
                let task = document.tasks.into_iter().next().unwrap();
                let task_result = self
                    .task_engine
                    .execute_task_default(task, inputs, run_id)?;

                let mut outputs = Bindings::new();
                for binding in task_result.outputs.iter() {
                    outputs =
                        outputs.bind(binding.name().to_string(), binding.value().clone(), None);
                }

                let mut task_results = HashMap::new();
                task_results.insert("main".to_string(), task_result);

                Ok(WorkflowResult {
                    outputs,
                    duration: Duration::default(),
                    task_results,
                    work_dir: self.workflow_dir.work.clone(),
                })
            } else {
                Err(RuntimeError::WorkflowValidationError {
                    message: "Document must contain either a workflow or exactly one task"
                        .to_string(),
                    pos: SourcePosition::new("".to_string(), "".to_string(), 0, 0, 0, 0),
                })
            }
        }
    }

    /// Resolve workflow inputs from prefixed/unprefixed input bindings
    fn resolve_workflow_inputs(
        &self,
        workflow: &Workflow,
        inputs: Bindings<Value>,
    ) -> RuntimeResult<Bindings<Value>> {
        let mut resolved = Bindings::new();

        if !workflow.inputs.is_empty() {
            for input_decl in &workflow.inputs {
                let prefixed_name = format!("{}.{}", workflow.name, input_decl.name);

                // Check for prefixed input first, then unprefixed
                if let Some(binding) = inputs.resolve(&prefixed_name) {
                    resolved = resolved.bind(input_decl.name.clone(), binding.clone(), None);
                } else if let Some(binding) = inputs.resolve(&input_decl.name) {
                    resolved = resolved.bind(input_decl.name.clone(), binding.clone(), None);
                }
                // If neither exists, it will be handled by validation or default values
            }
        }

        // Add any additional bindings that don't match workflow inputs
        for binding in inputs.iter() {
            let name = binding.name();
            // Skip if it's a prefixed workflow input we already handled
            if !name.starts_with(&format!("{}.", workflow.name))
                || !resolved.has_binding(
                    name.strip_prefix(&format!("{}.", workflow.name))
                        .unwrap_or(name),
                )
            {
                resolved = resolved.bind(name.to_string(), binding.value().clone(), None);
            }
        }

        Ok(resolved)
    }

    /// Validate workflow inputs
    pub fn validate_workflow_inputs(
        &self,
        workflow: &Workflow,
        inputs: &Bindings<Value>,
    ) -> RuntimeResult<()> {
        if !workflow.inputs.is_empty() {
            for input_decl in &workflow.inputs {
                if input_decl.expr.is_none() && !input_decl.decl_type.is_optional() {
                    // Required input (no default AND not optional) - check both prefixed and unprefixed forms
                    let prefixed_name = format!("{}.{}", workflow.name, input_decl.name);
                    let has_prefixed = inputs.has_binding(&prefixed_name);
                    let has_unprefixed = inputs.has_binding(&input_decl.name);

                    if !has_prefixed && !has_unprefixed {
                        return Err(RuntimeError::WorkflowValidationError {
                            message: format!(
                                "Missing required workflow input: '{}'\nExpected: '{}' or '{}' in input JSON",
                                input_decl.name,
                                prefixed_name,
                                input_decl.name
                            ),
                            pos: input_decl.pos.clone(),
                        });
                    }
                }
            }
        }
        Ok(())
    }

    /// Execute the workflow body (sequence of nodes)
    fn execute_workflow_body(
        &self,
        workflow: &Workflow,
        context: &mut WorkflowContext,
        run_id: &str,
    ) -> RuntimeResult<()> {
        // Create stdlib for expression evaluation
        let stdlib = crate::stdlib::StdLib::new("1.0");

        // Add workflow inputs to context
        if !workflow.inputs.is_empty() {
            for input_decl in &workflow.inputs {
                if let Some(input_expr) = &input_decl.expr {
                    // Evaluate default value if input not provided
                    if !context.bindings.has_binding(&input_decl.name) {
                        let default_value = input_expr.eval(&context.bindings, &stdlib)?;
                        context.bindings =
                            context
                                .bindings
                                .bind(input_decl.name.clone(), default_value, None);
                    }
                } else if input_decl.decl_type.is_optional()
                    && !context.bindings.has_binding(&input_decl.name)
                {
                    // Optional input without explicit default - initialize with None
                    context.bindings =
                        context
                            .bindings
                            .bind(input_decl.name.clone(), Value::Null, None);
                }
            }
        }

        // Execute workflow body nodes in sequence
        for node in &workflow.body {
            self.execute_workflow_node(node, context, run_id, &stdlib)?;
        }

        Ok(())
    }

    /// Execute a single workflow node
    fn execute_workflow_node(
        &self,
        node: &WorkflowElement,
        context: &mut WorkflowContext,
        run_id: &str,
        stdlib: &crate::stdlib::StdLib,
    ) -> RuntimeResult<()> {
        match node {
            WorkflowElement::Call(call) => {
                self.execute_call(call, context, run_id, stdlib)?;
            }
            WorkflowElement::Scatter(scatter) => {
                self.execute_scatter(scatter, context, run_id, stdlib)?;
            }
            WorkflowElement::Conditional(conditional) => {
                self.execute_conditional(conditional, context, run_id, stdlib)?;
            }
            WorkflowElement::Declaration(decl) => {
                // Execute variable declaration
                if let Some(expr) = &decl.expr {
                    let value = expr.eval(&context.bindings, stdlib)?;
                    context.bindings = context.bindings.bind(decl.name.clone(), value, None);
                }
            }
        }
        Ok(())
    }

    /// Execute a task call
    fn execute_call(
        &self,
        call: &Call,
        context: &mut WorkflowContext,
        run_id: &str,
        stdlib: &crate::stdlib::StdLib,
    ) -> RuntimeResult<()> {
        // Evaluate call inputs
        let mut call_inputs = Bindings::new();

        for (input_name, input_expr) in &call.inputs {
            let input_value = input_expr.eval(&context.bindings, stdlib)?;
            call_inputs = call_inputs.bind(input_name.clone(), input_value, None);
        }

        // Use resolved callee if available, otherwise fall back to document search
        let task = if let Some(ref callee) = call.callee {
            match callee {
                crate::tree::CalleeRef::Task(task) => task.clone(),
                crate::tree::CalleeRef::Workflow(_) => {
                    return Err(RuntimeError::WorkflowValidationError {
                        message: format!("Cannot execute workflow '{}' as a task call", call.task),
                        pos: call.pos.clone(),
                    });
                }
            }
        } else if let Some(ref document) = self.document {
            // Fall back to old behavior for backwards compatibility
            document
                .tasks
                .iter()
                .find(|t| t.name == call.task)
                .cloned()
                .ok_or_else(|| RuntimeError::WorkflowValidationError {
                    message: format!("Task '{}' not found in document", call.task),
                    pos: call.pos.clone(),
                })?
        } else {
            return Err(RuntimeError::WorkflowValidationError {
                message: "No document available for task resolution".to_string(),
                pos: call.pos.clone(),
            });
        };

        // Execute task
        let call_name = if let Some(alias) = &call.alias {
            alias.clone()
        } else {
            // Use the full task name including namespace for proper resolution
            call.task.clone()
        };
        let unique_run_id = format!("{}_{}", run_id, call_name.replace('.', "_"));

        let task_result =
            self.task_engine
                .execute_task_default(task, call_inputs, &unique_run_id)?;

        // Add task outputs to workflow context
        for binding in task_result.outputs.iter() {
            let qualified_name = format!("{}.{}", call_name, binding.name());
            context.bindings = context
                .bindings
                .bind(qualified_name, binding.value().clone(), None);
        }

        // Store task result using the call name that the aggregation expects
        context.task_results.insert(call_name, task_result);

        Ok(())
    }

    /// Execute a scatter block
    fn execute_scatter(
        &self,
        scatter: &Scatter,
        context: &mut WorkflowContext,
        run_id: &str,
        stdlib: &crate::stdlib::StdLib,
    ) -> RuntimeResult<()> {
        // Evaluate scatter collection
        let collection_value = scatter.expr.eval(&context.bindings, stdlib)?;

        // Extract array values
        let array_values = match collection_value {
            Value::Array { values, .. } => values,
            _ => {
                return Err(WdlError::output_error(
                    "Scatter collection must be an array".to_string(),
                    "Array".to_string(),
                    format!("{:?}", collection_value.wdl_type()),
                    Some(scatter.expr.pos().clone()),
                ));
            }
        };

        // Execute scatter body for each array element
        let mut scatter_results = Vec::new();

        for (index, item_value) in array_values.iter().enumerate() {
            // Create new context with scatter variable
            let mut scatter_context = WorkflowContext {
                bindings: context.bindings.clone(),
                task_results: HashMap::new(),
                start_time: context.start_time,
            };

            // Add scatter item to context
            scatter_context.bindings =
                scatter_context
                    .bindings
                    .bind(scatter.variable.clone(), item_value.clone(), None);

            // Execute scatter body
            let scatter_run_id = format!("{}_scatter_{}", run_id, index);
            for node in &scatter.body {
                self.execute_workflow_node(node, &mut scatter_context, &scatter_run_id, stdlib)?;
            }

            // Collect scatter results
            scatter_results.push(scatter_context.bindings.clone());

            // Merge task results
            for (name, result) in scatter_context.task_results {
                let indexed_name = format!("{}_{}", name, index);
                context.task_results.insert(indexed_name, result);
            }
        }

        // Aggregate scatter outputs properly (following miniwdl's arrayize pattern)
        self.aggregate_scatter_outputs(scatter, scatter_results, context)?;

        Ok(())
    }

    /// Aggregate scatter outputs into arrays (following miniwdl's arrayize pattern)
    fn aggregate_scatter_outputs(
        &self,
        scatter: &Scatter,
        scatter_results: Vec<Bindings<Value>>,
        context: &mut WorkflowContext,
    ) -> RuntimeResult<()> {
        use std::collections::HashMap;

        // Track all bindings that were created in the scatter
        let mut aggregated_bindings: HashMap<String, Vec<Value>> = HashMap::new();

        // Collect values from each scatter iteration
        for scatter_binding in &scatter_results {
            for binding in scatter_binding.iter() {
                let name = binding.name().to_string();

                // Skip the scatter variable itself
                if name == scatter.variable {
                    continue;
                }

                // Skip qualified task outputs for now (they need special handling)
                if name.contains('.') {
                    continue;
                }

                // Add to aggregated collection
                aggregated_bindings
                    .entry(name)
                    .or_default()
                    .push(binding.value().clone());
            }
        }

        // Create array bindings for each collected output
        for (name, values) in aggregated_bindings {
            // Determine the array element type from the first value
            let element_type = if let Some(first_value) = values.first() {
                first_value.wdl_type().clone()
            } else {
                // Empty array - use a generic type
                Type::string(false)
            };

            // Create array type
            let array_type = Type::array(element_type, false, !values.is_empty());

            // Create array value
            let array_value = Value::Array {
                values,
                wdl_type: array_type,
            };

            // Bind the array to the context
            context.bindings = context.bindings.bind(name, array_value, None);
        }

        // Handle task call outputs separately
        self.aggregate_task_call_outputs(scatter, scatter_results.len(), context)?;

        Ok(())
    }

    /// Aggregate task call outputs into arrays
    fn aggregate_task_call_outputs(
        &self,
        scatter: &Scatter,
        num_iterations: usize,
        context: &mut WorkflowContext,
    ) -> RuntimeResult<()> {
        use std::collections::HashMap;

        // Find task calls in the scatter body
        for element in &scatter.body {
            if let WorkflowElement::Call(call) = element {
                // Track outputs for this task across all iterations
                let mut task_outputs: HashMap<String, Vec<Value>> = HashMap::new();

                // For scattered calls, determine the binding name
                // For namespaced calls like "hello.hello_task", the output should be bound as "hello_task.matches"
                // to match WDL conventions where the namespace is stripped from output references
                let binding_call_name = if let Some(alias) = &call.alias {
                    alias.clone()
                } else if call.task.contains('.') {
                    // For namespaced calls, use just the task name part
                    call.task.split('.').next_back().unwrap().to_string()
                } else {
                    call.task.clone()
                };

                // The task results are stored using the full call name with index
                let storage_call_name = if let Some(alias) = &call.alias {
                    alias.clone()
                } else {
                    call.task.clone()
                };

                // Collect outputs from each iteration
                for i in 0..num_iterations {
                    // The key format matches what execute_scatter creates: "{name}_{index}"
                    let indexed_name = format!("{}_{}", storage_call_name, i);

                    if let Some(task_result) = context.task_results.get(&indexed_name) {
                        // Collect each output field
                        for output in task_result.outputs.iter() {
                            let output_name = output.name().to_string();
                            task_outputs
                                .entry(output_name)
                                .or_default()
                                .push(output.value().clone());
                        }
                    }
                }

                // Create array bindings for task outputs using the binding name (not storage name)
                for (output_name, values) in task_outputs {
                    let qualified_name = format!("{}.{}", binding_call_name, output_name);

                    // Determine array element type
                    let element_type = if let Some(first_value) = values.first() {
                        first_value.wdl_type().clone()
                    } else {
                        Type::string(false)
                    };

                    // Create array type and value
                    let array_type = Type::array(element_type, false, !values.is_empty());
                    let array_value = Value::Array {
                        values,
                        wdl_type: array_type,
                    };

                    // Bind to context
                    context.bindings = context.bindings.bind(qualified_name, array_value, None);
                }
            }
        }

        Ok(())
    }

    /// Execute a conditional block
    fn execute_conditional(
        &self,
        conditional: &Conditional,
        context: &mut WorkflowContext,
        run_id: &str,
        stdlib: &crate::stdlib::StdLib,
    ) -> RuntimeResult<()> {
        // Evaluate condition
        let condition_value = conditional.expr.eval(&context.bindings, stdlib)?;

        // Check if condition is true
        let should_execute = match condition_value {
            Value::Boolean { value, .. } => value,
            Value::Null => false,
            _ => {
                return Err(WdlError::output_error(
                    "Conditional condition must be Boolean or None".to_string(),
                    "Boolean".to_string(),
                    format!("{:?}", condition_value.wdl_type()),
                    Some(conditional.expr.pos().clone()),
                ));
            }
        };

        // Collect all potential variables defined in this conditional
        let potential_variables = self.collect_conditional_variables(conditional);

        if should_execute {
            // Create isolated context for conditional execution
            let mut conditional_context = WorkflowContext {
                bindings: context.bindings.clone(),
                task_results: HashMap::new(),
                start_time: context.start_time,
            };

            // Execute conditional body
            for node in &conditional.body {
                self.execute_workflow_node(node, &mut conditional_context, run_id, stdlib)?;
            }

            // Aggregate conditional results as optional values
            self.aggregate_conditional_outputs(
                conditional,
                &conditional_context,
                context,
                &potential_variables,
                true,
            )?;

            // Merge task results
            for (name, result) in conditional_context.task_results {
                context.task_results.insert(name, result);
            }
        } else {
            // Condition is false - create null values for all potential variables
            self.aggregate_conditional_outputs(
                conditional,
                &WorkflowContext {
                    bindings: context.bindings.clone(),
                    task_results: HashMap::new(),
                    start_time: context.start_time,
                },
                context,
                &potential_variables,
                false,
            )?;
        }

        Ok(())
    }

    /// Collect all variables that could potentially be defined in a conditional block
    fn collect_conditional_variables(&self, conditional: &Conditional) -> Vec<String> {
        let mut variables = Vec::new();

        for element in &conditional.body {
            match element {
                WorkflowElement::Declaration(decl) => {
                    variables.push(decl.name.clone());
                }
                WorkflowElement::Call(call) => {
                    // Task calls create qualified variable names
                    let call_name = call.alias.as_ref().unwrap_or(&call.task).clone();

                    // Find the task definition to get output names using resolved callee
                    let task_opt = if let Some(ref callee) = call.callee {
                        match callee {
                            crate::tree::CalleeRef::Task(task) => Some(task),
                            crate::tree::CalleeRef::Workflow(_) => None, // Workflows not supported in task calls
                        }
                    } else if let Some(ref document) = self.document {
                        // Fall back to document search
                        document.tasks.iter().find(|t| t.name == call.task)
                    } else {
                        None
                    };

                    if let Some(task) = task_opt {
                        // task.outputs is Vec<Declaration>, not Option<Vec<Declaration>>
                        for output in &task.outputs {
                            let qualified_name = format!("{}.{}", call_name, output.name);
                            variables.push(qualified_name);
                        }
                    }

                    // Also add the general call namespace
                    variables.push(call_name);
                }
                WorkflowElement::Scatter(scatter) => {
                    // Recursively collect from scatter body
                    let scatter_vars = self.collect_scatter_conditional_variables(&scatter.body);
                    variables.extend(scatter_vars);
                }
                WorkflowElement::Conditional(nested_conditional) => {
                    // Recursively collect from nested conditional
                    let nested_vars = self.collect_conditional_variables(nested_conditional);
                    variables.extend(nested_vars);
                }
            }
        }

        variables
    }

    /// Helper to collect variables from scatter body within conditionals
    fn collect_scatter_conditional_variables(&self, elements: &[WorkflowElement]) -> Vec<String> {
        let mut variables = Vec::new();

        for element in elements {
            match element {
                WorkflowElement::Declaration(decl) => {
                    variables.push(decl.name.clone());
                }
                WorkflowElement::Call(call) => {
                    let call_name = call.alias.as_ref().unwrap_or(&call.task).clone();

                    // Find the task definition to get output names using resolved callee
                    let task_opt = if let Some(ref callee) = call.callee {
                        match callee {
                            crate::tree::CalleeRef::Task(task) => Some(task),
                            crate::tree::CalleeRef::Workflow(_) => None, // Workflows not supported in task calls
                        }
                    } else if let Some(ref document) = self.document {
                        // Fall back to document search
                        document.tasks.iter().find(|t| t.name == call.task)
                    } else {
                        None
                    };

                    if let Some(task) = task_opt {
                        // task.outputs is Vec<Declaration>, not Option<Vec<Declaration>>
                        for output in &task.outputs {
                            let qualified_name = format!("{}.{}", call_name, output.name);
                            variables.push(qualified_name);
                        }
                    }

                    variables.push(call_name);
                }
                WorkflowElement::Scatter(nested_scatter) => {
                    let nested_vars =
                        self.collect_scatter_conditional_variables(&nested_scatter.body);
                    variables.extend(nested_vars);
                }
                WorkflowElement::Conditional(nested_conditional) => {
                    let nested_vars = self.collect_conditional_variables(nested_conditional);
                    variables.extend(nested_vars);
                }
            }
        }

        variables
    }

    /// Aggregate conditional outputs into optional values
    fn aggregate_conditional_outputs(
        &self,
        conditional: &Conditional,
        conditional_context: &WorkflowContext,
        main_context: &mut WorkflowContext,
        potential_variables: &[String],
        condition_was_true: bool,
    ) -> RuntimeResult<()> {
        use crate::types::Type;

        if condition_was_true {
            // Condition was true - use actual values but make them optional
            for binding in conditional_context.bindings.iter() {
                let name = binding.name().to_string();

                // Skip variables that were already in the outer context
                if main_context.bindings.has_binding(&name) && !potential_variables.contains(&name)
                {
                    continue;
                }

                // Convert to optional type if this is a variable defined in the conditional
                if potential_variables.contains(&name) {
                    let value = binding.value().clone();
                    let optional_value = self.make_optional_value(value, true);
                    main_context.bindings = main_context.bindings.bind(name, optional_value, None);
                }
            }

            // Handle task call outputs specially
            for (task_name, task_result) in &conditional_context.task_results {
                for output_binding in task_result.outputs.iter() {
                    let qualified_name = format!("{}.{}", task_name, output_binding.name());
                    if potential_variables.contains(&qualified_name) {
                        let value = output_binding.value().clone();
                        let optional_value = self.make_optional_value(value, true);
                        main_context.bindings =
                            main_context
                                .bindings
                                .bind(qualified_name, optional_value, None);
                    }
                }
            }
        } else {
            // Condition was false - create null values for all potential variables
            for var_name in potential_variables {
                let null_value = Value::Null;
                main_context.bindings =
                    main_context
                        .bindings
                        .bind(var_name.clone(), null_value, None);
            }
        }

        Ok(())
    }

    /// Convert a value to an optional value
    fn make_optional_value(&self, value: Value, has_value: bool) -> Value {
        use crate::types::Type;
        use crate::value::Value;

        if has_value {
            // The value exists - create new value with optional type
            match value {
                Value::Null => value, // Already null/optional
                Value::Boolean { value: v, wdl_type } => Value::Boolean {
                    value: v,
                    wdl_type: wdl_type.with_optional(true),
                },
                Value::Int { value: v, wdl_type } => Value::Int {
                    value: v,
                    wdl_type: wdl_type.with_optional(true),
                },
                Value::Float { value: v, wdl_type } => Value::Float {
                    value: v,
                    wdl_type: wdl_type.with_optional(true),
                },
                Value::String { value: v, wdl_type } => Value::String {
                    value: v,
                    wdl_type: wdl_type.with_optional(true),
                },
                Value::File { value: v, wdl_type } => Value::File {
                    value: v,
                    wdl_type: wdl_type.with_optional(true),
                },
                Value::Directory { value: v, wdl_type } => Value::Directory {
                    value: v,
                    wdl_type: wdl_type.with_optional(true),
                },
                Value::Array {
                    values: v,
                    wdl_type,
                } => Value::Array {
                    values: v,
                    wdl_type: wdl_type.with_optional(true),
                },
                Value::Map { pairs: p, wdl_type } => Value::Map {
                    pairs: p,
                    wdl_type: wdl_type.with_optional(true),
                },
                Value::Pair {
                    left: l,
                    right: r,
                    wdl_type,
                } => Value::Pair {
                    left: l,
                    right: r,
                    wdl_type: wdl_type.with_optional(true),
                },
                Value::Struct {
                    members: m,
                    extra_keys: e,
                    wdl_type,
                } => Value::Struct {
                    members: m,
                    extra_keys: e,
                    wdl_type: wdl_type.with_optional(true),
                },
            }
        } else {
            // No value - return null
            Value::Null
        }
    }

    /// Collect workflow outputs
    fn collect_workflow_outputs(
        &self,
        workflow: &Workflow,
        context: &WorkflowContext,
        stdlib: &crate::stdlib::StdLib,
    ) -> RuntimeResult<Bindings<Value>> {
        let mut outputs = Bindings::new();
        // Create a mutable copy of context bindings to include evaluated outputs
        let mut extended_bindings = context.bindings.clone();

        if !workflow.outputs.is_empty() {
            for output_decl in &workflow.outputs {
                if let Some(output_expr) = &output_decl.expr {
                    // Evaluate using extended bindings that include previously evaluated outputs
                    let output_value = output_expr.eval(&extended_bindings, stdlib)?;

                    // Try to coerce the output value to the expected type
                    let expected_type = &output_decl.decl_type;
                    let output_value = output_value.coerce(expected_type).map_err(|_e| {
                        WdlError::output_error(
                            format!(
                                "Cannot coerce workflow output '{}' to expected type",
                                output_decl.name
                            ),
                            format!("{:?}", expected_type),
                            format!("{:?}", output_value.wdl_type()),
                            Some(output_decl.pos.clone()),
                        )
                    })?;

                    // Add to output bindings
                    outputs = outputs.bind(output_decl.name.clone(), output_value.clone(), None);

                    // Also add to extended bindings so subsequent outputs can reference this one
                    extended_bindings =
                        extended_bindings.bind(output_decl.name.clone(), output_value, None);
                } else {
                    return Err(WdlError::workflow_validation_error(
                        format!("Workflow output missing expression: {}", output_decl.name),
                        output_decl.pos.clone(),
                    ));
                }
            }
        }

        Ok(outputs)
    }

    /// Get workflow input requirements
    pub fn get_workflow_inputs(&self, workflow: &Workflow) -> Vec<(String, Type, bool)> {
        workflow
            .inputs
            .iter()
            .map(|decl| {
                let required = decl.expr.is_none();
                (decl.name.clone(), decl.decl_type.clone(), required)
            })
            .collect()
    }

    /// Get workflow output types
    pub fn get_workflow_outputs(&self, workflow: &Workflow) -> Vec<(String, Type)> {
        workflow
            .outputs
            .iter()
            .map(|decl| (decl.name.clone(), decl.decl_type.clone()))
            .collect()
    }

    /// Validate a workflow before execution
    pub fn validate_workflow(&self, workflow: &Workflow) -> RuntimeResult<()> {
        // Check that workflow has inputs and outputs
        let has_inputs = !workflow.inputs.is_empty();
        let has_outputs = !workflow.outputs.is_empty();
        if !has_inputs && !has_outputs {
            return Err(RuntimeError::WorkflowValidationError {
                message: "Workflow has no inputs or outputs".to_string(),
                pos: workflow.pos.clone(),
            });
        }

        // Validate that all outputs have expressions
        if !workflow.outputs.is_empty() {
            for output_decl in &workflow.outputs {
                if output_decl.expr.is_none() {
                    return Err(RuntimeError::WorkflowValidationError {
                        message: format!(
                            "Workflow output missing expression: {}",
                            output_decl.name
                        ),
                        pos: output_decl.pos.clone(),
                    });
                }
            }
        }

        // TODO: Add more sophisticated validation
        // - Check that all referenced tasks exist
        // - Validate data flow between tasks
        // - Check for circular dependencies

        Ok(())
    }
}

/// Workflow execution statistics
#[derive(Debug, Clone)]
pub struct WorkflowExecutionStats {
    /// Workflow name
    pub workflow_name: String,
    /// Total execution duration
    pub total_duration: Duration,
    /// Number of tasks executed
    pub tasks_executed: usize,
    /// Task execution statistics
    pub task_stats: HashMap<String, Duration>,
    /// Memory usage (if available)
    pub memory_usage: Option<u64>,
}

/// Utilities for workflow execution
pub mod utils {
    use super::*;

    /// Create a simple workflow execution report
    pub fn create_execution_report(result: &WorkflowResult) -> String {
        let mut report = String::new();

        report.push_str("Workflow Execution Report\n");
        report.push_str("========================\n");
        report.push_str(&format!("Duration: {:?}\n", result.duration));
        report.push_str(&format!("Tasks executed: {}\n", result.task_results.len()));
        report.push_str(&format!("Outputs: {}\n", result.outputs.len()));
        report.push_str(&format!("Work directory: {}\n", result.work_dir.display()));

        if !result.task_results.is_empty() {
            report.push_str("\nTask Results:\n");
            for (task_name, task_result) in &result.task_results {
                report.push_str(&format!(
                    "  {}: {:?} ({} outputs)\n",
                    task_name,
                    task_result.duration,
                    task_result.outputs.len()
                ));
            }
        }

        if !result.outputs.is_empty() {
            report.push_str("\nOutputs:\n");
            for binding in result.outputs.iter() {
                report.push_str(&format!(
                    "  {}: {:?}\n",
                    binding.name(),
                    binding.value().wdl_type()
                ));
            }
        }

        report
    }

    /// Extract file outputs from workflow result
    pub fn extract_file_outputs(result: &WorkflowResult) -> Vec<(String, PathBuf)> {
        let mut files = Vec::new();

        for binding in result.outputs.iter() {
            let name = binding.name();
            let value = binding.value();
            if let Value::File { value: path, .. } = value {
                files.push((name.to_string(), PathBuf::from(path)));
            } else if let Value::Array { values: arr, .. } = value {
                for (i, item) in arr.iter().enumerate() {
                    if let Value::File { value: path, .. } = item {
                        files.push((format!("{}[{}]", name, i), PathBuf::from(path)));
                    }
                }
            }
        }

        files
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

    fn create_simple_workflow() -> Workflow {
        Workflow {
            pos: SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 10),
            name: "simple_workflow".to_string(),
            inputs: vec![
                Decl {
                    pos: SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 2, 1, 2, 20),
                    name: "message".to_string(),
                    wdl_type: Type::String,
                    expr: None, // Required
                }
            ],
            body: vec![
                WorkflowNode::Declaration(Decl {
                    pos: SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 3, 1, 3, 30),
                    name: "processed_message".to_string(),
                    wdl_type: Type::String,
                    expr: Some(Expr::Apply(ApplyExpr {
                        pos: SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 3, 25, 3, 45),
                        function: "+".to_string(),
                        arguments: vec![
                            Expr::Get(GetExpr {
                                pos: SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 3, 28, 3, 35),
                                expr: Box::new(Expr::Ident(IdentExpr {
                                    pos: SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 3, 28, 3, 35),
                                    name: "message".to_string(),
                                })),
                                key: "length".to_string(),
                            }),
                            Expr::String(StringExpr {
                                pos: SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 3, 39, 3, 44),
                                value: " processed".to_string(),
                            }),
                        ],
                    })),
                })
            ],
            outputs: vec![
                Decl {
                    pos: SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 4, 1, 4, 25),
                    name: "result".to_string(),
                    wdl_type: Type::String,
                    expr: Some(Expr::Ident(IdentExpr {
                        pos: SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 4, 15, 4, 32),
                        name: "processed_message".to_string(),
                    })),
                }
            ],
            parameter_meta: None,
            meta: None,
        }
    }

    #[test]
    fn test_workflow_engine_creation() {
        let config = Config::default();
        let temp_dir = tempdir().unwrap();
        let workflow_dir = WorkflowDirectory::create(temp_dir.path(), "test_run").unwrap();

        let engine = WorkflowEngine::new(config, workflow_dir);
        assert_eq!(engine.config.max_concurrent_tasks, 1);
    }

    #[test]
    fn test_workflow_input_validation() {
        let config = Config::default();
        let temp_dir = tempdir().unwrap();
        let workflow_dir = WorkflowDirectory::create(temp_dir.path(), "test_run").unwrap();
        let engine = WorkflowEngine::new(config, workflow_dir);

        let workflow = create_simple_workflow();

        // Valid inputs
        let mut inputs = Env::Bindings::new();
        inputs.insert("message".to_string(), Value::String("Hello".to_string()));
        assert!(engine.validate_workflow_inputs(&workflow, &inputs).is_ok());

        // Missing required input
        let empty_inputs = Env::Bindings::new();
        let result = engine.validate_workflow_inputs(&workflow, &empty_inputs);
        assert!(result.is_err());
    }

    #[test]
    fn test_get_workflow_inputs() {
        let config = Config::default();
        let temp_dir = tempdir().unwrap();
        let workflow_dir = WorkflowDirectory::create(temp_dir.path(), "test_run").unwrap();
        let engine = WorkflowEngine::new(config, workflow_dir);

        let workflow = create_simple_workflow();
        let inputs = engine.get_workflow_inputs(&workflow);

        assert_eq!(inputs.len(), 1);
        assert_eq!(inputs[0].0, "message");
        assert_eq!(inputs[0].1, Type::String);
        assert!(inputs[0].2); // Required
    }

    #[test]
    fn test_get_workflow_outputs() {
        let config = Config::default();
        let temp_dir = tempdir().unwrap();
        let workflow_dir = WorkflowDirectory::create(temp_dir.path(), "test_run").unwrap();
        let engine = WorkflowEngine::new(config, workflow_dir);

        let workflow = create_simple_workflow();
        let outputs = engine.get_workflow_outputs(&workflow);

        assert_eq!(outputs.len(), 1);
        assert_eq!(outputs[0].0, "result");
        assert_eq!(outputs[0].1, Type::String);
    }

    #[test]
    fn test_workflow_validation() {
        let config = Config::default();
        let temp_dir = tempdir().unwrap();
        let workflow_dir = WorkflowDirectory::create(temp_dir.path(), "test_run").unwrap();
        let engine = WorkflowEngine::new(config, workflow_dir);

        let workflow = create_simple_workflow();
        assert!(engine.validate_workflow(&workflow).is_ok());

        // Test workflow with missing output expression
        let mut invalid_workflow = workflow.clone();
        invalid_workflow.outputs[0].expr = None;
        let result = engine.validate_workflow(&invalid_workflow);
        assert!(result.is_err());
    }

    #[test]
    fn test_execution_report_creation() {
        use super::utils::create_execution_report;

        let temp_dir = tempdir().unwrap();
        let result = WorkflowResult {
            outputs: {
                let mut outputs = Bindings::new();
                outputs.insert("result".to_string(), Value::String("test".to_string()));
                outputs
            },
            duration: Duration::from_secs(10),
            task_results: HashMap::new(),
            work_dir: temp_dir.path().to_path_buf(),
        };

        let report = create_execution_report(&result);
        assert!(report.contains("Workflow Execution Report"));
        assert!(report.contains("Duration: 10s"));
        assert!(report.contains("Outputs: 1"));
    }

    #[test]
    fn test_extract_file_outputs() {
        use super::utils::extract_file_outputs;

        let temp_dir = tempdir().unwrap();
        let result = WorkflowResult {
            outputs: {
                let mut outputs = Bindings::new();
                outputs.insert("output_file".to_string(), Value::File("/path/to/file.txt".to_string()));
                outputs.insert("output_array".to_string(), Value::Array(vec![
                    Value::File("/path/to/file1.txt".to_string()),
                    Value::File("/path/to/file2.txt".to_string()),
                ]));
                outputs
            },
            duration: Duration::from_secs(1),
            task_results: HashMap::new(),
            work_dir: temp_dir.path().to_path_buf(),
        };

        let files = extract_file_outputs(&result);
        assert_eq!(files.len(), 3);
        assert_eq!(files[0].0, "output_file");
        assert_eq!(files[0].1, PathBuf::from("/path/to/file.txt"));
    }
    */
}
