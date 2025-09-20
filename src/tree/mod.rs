//! WDL Abstract Syntax Tree (AST) for documents, tasks, and workflows
//!
//! This module contains the AST representation for WDL documents, including tasks,
//! workflows, declarations, calls, and control flow sections. The AST is typically
//! constructed by the parser and used for type checking and execution.

use crate::env::Bindings;
use crate::error::{HasSourcePosition, SourceNode, SourcePosition, WdlError};
use crate::expr::{Expression, ExpressionBase};
use crate::types::Type;
use indexmap::IndexMap;
use serde::{Deserialize, Serialize};
use std::collections::HashMap;

// Re-export submodules
pub mod control_flow;
pub mod declarations;
pub mod document;
pub mod task;
pub mod traversal;
pub mod validation;
pub mod workflow;

#[cfg(test)]
mod doc_tests;

#[cfg(test)]
mod call_tests;

/// Base trait for WDL AST nodes
pub trait ASTNode: SourceNode {
    /// Get the children of this AST node
    fn children(&self) -> Vec<&dyn SourceNode>;

    /// Accept a visitor for AST traversal
    fn accept<T>(&self, visitor: &mut dyn ASTVisitor<T>) -> Result<T, WdlError>;
}

/// Visitor pattern for AST traversal
pub trait ASTVisitor<T> {
    fn visit_document(&mut self, node: &Document) -> Result<T, WdlError>;
    fn visit_workflow(&mut self, node: &Workflow) -> Result<T, WdlError>;
    fn visit_task(&mut self, node: &Task) -> Result<T, WdlError>;
    fn visit_declaration(&mut self, node: &Declaration) -> Result<T, WdlError>;
    fn visit_call(&mut self, node: &Call) -> Result<T, WdlError>;
    fn visit_scatter(&mut self, node: &Scatter) -> Result<T, WdlError>;
    fn visit_conditional(&mut self, node: &Conditional) -> Result<T, WdlError>;
    fn visit_gather(&mut self, node: &Gather) -> Result<T, WdlError>;
}

/// Base trait for workflow nodes (declarations, calls, sections)
pub trait WorkflowNode: ASTNode {
    /// Get the unique workflow node ID
    fn workflow_node_id(&self) -> &str;

    /// Get the dependencies of this workflow node
    fn workflow_node_dependencies(&self) -> Vec<String>;

    /// Get the scatter depth of this node
    fn scatter_depth(&self) -> u32;

    /// Set the scatter depth (used during scatter analysis)
    fn set_scatter_depth(&mut self, depth: u32);
}

/// WDL struct type definition
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct StructTypeDef {
    pub pos: SourcePosition,
    pub name: String,
    pub members: IndexMap<String, Type>,
    pub meta: HashMap<String, serde_json::Value>,
    pub parameter_meta: HashMap<String, serde_json::Value>,
    pub imported: Option<(Box<Document>, Box<StructTypeDef>)>,
}

impl StructTypeDef {
    pub fn new(
        pos: SourcePosition,
        name: String,
        members: IndexMap<String, Type>,
        meta: HashMap<String, serde_json::Value>,
        parameter_meta: HashMap<String, serde_json::Value>,
        imported: Option<(Box<Document>, Box<StructTypeDef>)>,
    ) -> Self {
        Self {
            pos,
            name,
            members,
            meta,
            parameter_meta,
            imported,
        }
    }

    /// Get a canonical type ID for this struct
    pub fn type_id(&self) -> String {
        // Create a canonical representation of member types
        let mut member_strs: Vec<String> = self
            .members
            .iter()
            .map(|(name, ty)| format!("{}:{}", name, ty))
            .collect();
        member_strs.sort();
        member_strs.join(",")
    }
}

impl HasSourcePosition for StructTypeDef {
    fn source_position(&self) -> &SourcePosition {
        &self.pos
    }

    fn set_source_position(&mut self, new_pos: SourcePosition) {
        self.pos = new_pos;
    }
}

impl SourceNode for StructTypeDef {
    fn parent(&self) -> Option<&dyn SourceNode> {
        None
    }

    fn set_parent(&mut self, _parent: Option<&dyn SourceNode>) {
        // Not implemented for simplicity
    }

    fn children(&self) -> Vec<&dyn SourceNode> {
        Vec::new() // Struct definitions don't have AST children
    }
}

/// Value declaration within a task or workflow
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Declaration {
    pub pos: SourcePosition,
    pub workflow_node_id: String,
    pub scatter_depth: u32,
    pub decl_type: Type,
    pub name: String,
    pub expr: Option<Expression>,
    pub decor: HashMap<String, serde_json::Value>, // For experimental decorations
}

impl Declaration {
    pub fn new(
        pos: SourcePosition,
        decl_type: Type,
        name: String,
        expr: Option<Expression>,
        id_prefix: &str,
    ) -> Self {
        Self {
            pos,
            workflow_node_id: format!("{}-{}", id_prefix, name),
            scatter_depth: 0,
            decl_type,
            name,
            expr,
            decor: HashMap::new(),
        }
    }

    /// Add this declaration to a type environment
    pub fn add_to_type_env(
        &self,
        _struct_types: &Bindings<IndexMap<String, Type>>,
        type_env: Bindings<Type>,
        collision_ok: bool,
    ) -> Result<Bindings<Type>, WdlError> {
        if !collision_ok {
            if type_env.resolve(&self.name).is_some() {
                return Err(WdlError::validation_error(
                    self.pos.clone(),
                    format!("Multiple declarations of {}", self.name),
                ));
            }
            if type_env.has_namespace(&self.name) {
                return Err(WdlError::validation_error(
                    self.pos.clone(),
                    format!("Value/call name collision on {}", self.name),
                ));
            }
        }

        // TODO: Resolve struct types
        Ok(type_env.bind(self.name.clone(), self.decl_type.clone(), None))
    }

    /// Type check this declaration
    pub fn typecheck(
        &mut self,
        type_env: &Bindings<Type>,
        stdlib: &crate::stdlib::StdLib,
        struct_typedefs: &[StructTypeDef],
    ) -> Result<(), WdlError> {
        // First, resolve struct types if this is a struct instance
        if let Type::StructInstance {
            type_name,
            members: None,
            optional,
        } = &self.decl_type
        {
            // Look up the struct definition to get member types
            if let Some(struct_def) = struct_typedefs.iter().find(|s| s.name == *type_name) {
                self.decl_type = Type::StructInstance {
                    type_name: type_name.clone(),
                    members: Some(struct_def.members.clone()),
                    optional: *optional,
                };
            }
        }

        if let Some(ref mut expr) = self.expr {
            let inferred_type = expr.infer_type(type_env, stdlib, struct_typedefs)?;
            inferred_type.check_coercion(&self.decl_type, true)?;
        }
        Ok(())
    }
}

impl HasSourcePosition for Declaration {
    fn source_position(&self) -> &SourcePosition {
        &self.pos
    }

    fn set_source_position(&mut self, new_pos: SourcePosition) {
        self.pos = new_pos;
    }
}

impl SourceNode for Declaration {
    fn parent(&self) -> Option<&dyn SourceNode> {
        None
    }

    fn set_parent(&mut self, _parent: Option<&dyn SourceNode>) {
        // Not implemented for simplicity
    }

    fn children(&self) -> Vec<&dyn SourceNode> {
        if let Some(ref expr) = self.expr {
            vec![expr]
        } else {
            Vec::new()
        }
    }
}

impl WorkflowNode for Declaration {
    fn workflow_node_id(&self) -> &str {
        &self.workflow_node_id
    }

    fn workflow_node_dependencies(&self) -> Vec<String> {
        // TODO: Extract dependencies from expression
        Vec::new()
    }

    fn scatter_depth(&self) -> u32 {
        self.scatter_depth
    }

    fn set_scatter_depth(&mut self, depth: u32) {
        self.scatter_depth = depth;
    }
}

impl ASTNode for Declaration {
    fn children(&self) -> Vec<&dyn SourceNode> {
        SourceNode::children(self)
    }

    fn accept<T>(&self, visitor: &mut dyn ASTVisitor<T>) -> Result<T, WdlError> {
        visitor.visit_declaration(self)
    }
}

/// WDL Task definition
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Task {
    pub pos: SourcePosition,
    pub name: String,
    pub inputs: Vec<Declaration>,
    pub postinputs: Vec<Declaration>,
    pub command: Expression, // Should be a String expression
    pub outputs: Vec<Declaration>,
    pub parameter_meta: HashMap<String, serde_json::Value>,
    pub runtime: HashMap<String, Expression>,
    pub requirements: HashMap<String, Expression>,
    pub hints: HashMap<String, Expression>,
    pub meta: HashMap<String, serde_json::Value>,
    pub effective_wdl_version: String,
}

impl Task {
    pub fn new(
        pos: SourcePosition,
        name: String,
        inputs: Vec<Declaration>,
        postinputs: Vec<Declaration>,
        command: Expression,
        outputs: Vec<Declaration>,
        parameter_meta: HashMap<String, serde_json::Value>,
        runtime: HashMap<String, Expression>,
        meta: HashMap<String, serde_json::Value>,
    ) -> Self {
        Self {
            pos,
            name,
            inputs,
            postinputs,
            command,
            outputs,
            parameter_meta,
            runtime,
            requirements: HashMap::new(),
            hints: HashMap::new(),
            meta,
            effective_wdl_version: "1.0".to_string(),
        }
    }

    /// Create a new Task with requirements and hints
    pub fn new_with_requirements_hints(
        pos: SourcePosition,
        name: String,
        inputs: Vec<Declaration>,
        postinputs: Vec<Declaration>,
        command: Expression,
        outputs: Vec<Declaration>,
        parameter_meta: HashMap<String, serde_json::Value>,
        runtime: HashMap<String, Expression>,
        requirements: HashMap<String, Expression>,
        hints: HashMap<String, Expression>,
        meta: HashMap<String, serde_json::Value>,
    ) -> Self {
        Self {
            pos,
            name,
            inputs,
            postinputs,
            command,
            outputs,
            parameter_meta,
            runtime,
            requirements,
            hints,
            meta,
            effective_wdl_version: "1.0".to_string(),
        }
    }

    /// Get available inputs for this task
    pub fn available_inputs(&self) -> Bindings<&Declaration> {
        let mut bindings = Bindings::new();

        let declarations = if !self.inputs.is_empty() {
            &self.inputs
        } else {
            &self.postinputs
        };

        for decl in declarations.iter().rev() {
            bindings = bindings.bind(decl.name.clone(), decl, None);
        }

        bindings
    }

    /// Get required inputs for this task (unbound and non-optional)
    pub fn required_inputs(&self) -> Bindings<&Declaration> {
        let mut bindings = Bindings::new();

        let available = self.available_inputs();
        for binding in available.iter() {
            let name = binding.name();
            let decl = binding.value();
            if decl.expr.is_none() && !decl.decl_type.is_optional() {
                bindings = bindings.bind(name.to_string(), *decl, None);
            }
        }

        bindings
    }

    /// Type check the task - performs type inference on all declarations and expressions
    pub fn typecheck(&mut self, struct_typedefs: &[StructTypeDef]) -> Result<(), WdlError> {
        // Create standard library for type checking
        let stdlib = crate::stdlib::StdLib::new("1.2");

        // Build type environment from inputs
        let mut type_env = Bindings::new();

        // Process input declarations: type check and add to environment
        // Inputs can reference previous inputs in their default expressions
        for input in &mut self.inputs {
            // Type check the default expression if it exists
            input.typecheck(&type_env, &stdlib, struct_typedefs)?;
            if let Some(ref mut expr) = input.expr {
                let inferred_type = expr.infer_type(&type_env, &stdlib, struct_typedefs)?;
                // Check that the inferred type is compatible with the declared type
                if !inferred_type.coerces(&input.decl_type, true) {
                    return Err(WdlError::StaticTypeMismatch {
                        pos: expr.pos().clone(),
                        expected: format!("{:?}", input.decl_type),
                        actual: format!("{:?}", inferred_type),
                        message: format!("Type mismatch in input declaration '{}'", input.name),
                        source_text: None,
                        declared_wdl_version: Some("1.2".to_string()),
                    });
                }
            }
            // Add this input to the type environment for subsequent declarations
            type_env = type_env.bind(input.name.clone(), input.decl_type.clone(), None);
        }

        // Process postinput declarations: type check and add to environment
        for postinput in &mut self.postinputs {
            // Type check the initialization expression if it exists
            postinput.typecheck(&type_env, &stdlib, struct_typedefs)?;
            if let Some(ref mut expr) = postinput.expr {
                let inferred_type = expr.infer_type(&type_env, &stdlib, struct_typedefs)?;
                // Check that the inferred type is compatible with the declared type
                if !inferred_type.coerces(&postinput.decl_type, true) {
                    return Err(WdlError::StaticTypeMismatch {
                        pos: expr.pos().clone(),
                        expected: format!("{:?}", postinput.decl_type),
                        actual: format!("{:?}", inferred_type),
                        message: format!(
                            "Type mismatch in postinput declaration '{}'",
                            postinput.name
                        ),
                        source_text: None,
                        declared_wdl_version: Some("1.2".to_string()),
                    });
                }
            }
            // Add to type environment for subsequent declarations
            type_env = type_env.bind(postinput.name.clone(), postinput.decl_type.clone(), None);
        }

        // Type check the command expression (must be String)
        let command_type = self
            .command
            .infer_type(&type_env, &stdlib, struct_typedefs)?;
        if !command_type.coerces(&Type::string(false), true) {
            return Err(WdlError::StaticTypeMismatch {
                pos: self.command.pos().clone(),
                expected: format!("{:?}", Type::string(false)),
                actual: format!("{:?}", command_type),
                message: "Task command must be a String".to_string(),
                source_text: None,
                declared_wdl_version: Some("1.2".to_string()),
            });
        }

        // Type check runtime expressions
        for expr in self.runtime.values_mut() {
            expr.infer_type(&type_env, &stdlib, struct_typedefs)?;
        }

        // Type check requirements expressions
        for expr in self.requirements.values_mut() {
            expr.infer_type(&type_env, &stdlib, struct_typedefs)?;
        }

        // Type check hints expressions
        for expr in self.hints.values_mut() {
            expr.infer_type(&type_env, &stdlib, struct_typedefs)?;
        }

        // Create output-specific standard library for output declarations
        let output_stdlib = crate::stdlib::task_output::create_task_output_stdlib(
            "1.2",
            std::path::PathBuf::from("/tmp"), // Placeholder path for type checking
        );

        // Type check output declarations
        for output in &mut self.outputs {
            output.typecheck(&type_env, &output_stdlib, struct_typedefs)?;
            type_env = type_env.bind(output.name.clone(), output.decl_type.clone(), None);
        }

        Ok(())
    }
}

impl HasSourcePosition for Task {
    fn source_position(&self) -> &SourcePosition {
        &self.pos
    }

    fn set_source_position(&mut self, new_pos: SourcePosition) {
        self.pos = new_pos;
    }
}

impl SourceNode for Task {
    fn parent(&self) -> Option<&dyn SourceNode> {
        None
    }

    fn set_parent(&mut self, _parent: Option<&dyn SourceNode>) {
        // Not implemented for simplicity
    }

    fn children(&self) -> Vec<&dyn SourceNode> {
        let mut children: Vec<&dyn SourceNode> = Vec::new();

        for input in &self.inputs {
            children.push(input);
        }

        for postinput in &self.postinputs {
            children.push(postinput);
        }

        children.push(&self.command);

        for output in &self.outputs {
            children.push(output);
        }

        for expr in self.runtime.values() {
            children.push(expr);
        }

        for expr in self.requirements.values() {
            children.push(expr);
        }

        for expr in self.hints.values() {
            children.push(expr);
        }

        children
    }
}

impl ASTNode for Task {
    fn children(&self) -> Vec<&dyn SourceNode> {
        SourceNode::children(self)
    }

    fn accept<T>(&self, visitor: &mut dyn ASTVisitor<T>) -> Result<T, WdlError> {
        visitor.visit_task(self)
    }
}

/// Task or workflow call
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Call {
    pub pos: SourcePosition,
    pub workflow_node_id: String,
    pub scatter_depth: u32,
    pub task: String, // Task or workflow name, potentially namespaced (e.g., "lib.task_name")
    pub alias: Option<String>,
    pub inputs: HashMap<String, Expression>,
    pub afters: Vec<String>,

    // Resolved call information (filled during type checking)
    #[serde(skip)]
    pub callee: Option<CalleeRef>, // Reference to the actual Task or Workflow
}

/// Reference to a callable (Task or Workflow) that can be called
#[derive(Debug, Clone, PartialEq)]
pub enum CalleeRef {
    Task(Task),
    Workflow(Workflow),
}

impl Call {
    pub fn new(
        pos: SourcePosition,
        task: String,
        alias: Option<String>,
        inputs: HashMap<String, Expression>,
        afters: Vec<String>,
    ) -> Self {
        let workflow_node_id = format!("call-{}", alias.as_ref().unwrap_or(&task));

        Self {
            pos,
            workflow_node_id,
            scatter_depth: 0,
            task,
            alias,
            inputs,
            afters,
            callee: None, // Will be resolved during type checking
        }
    }

    /// Resolve the call to its actual task or workflow
    pub fn resolve(&mut self, doc: &Document) -> Result<(), WdlError> {
        if self.callee.is_some() {
            return Ok(()); // Already resolved
        }

        let parts: Vec<&str> = self.task.split('.').collect();

        match parts.len() {
            1 => {
                // Local call - look in this document
                let task_name = parts[0];

                // First check workflows
                if let Some(ref workflow) = doc.workflow {
                    if workflow.name == task_name {
                        // Check if workflow is callable (has complete calls)
                        if workflow.complete_calls.unwrap_or(false) {
                            self.callee = Some(CalleeRef::Workflow(workflow.clone()));
                            return Ok(());
                        } else {
                            return Err(WdlError::validation_error(
                                self.pos.clone(),
                                format!(
                                    "Workflow {} is not callable (incomplete calls)",
                                    task_name
                                ),
                            ));
                        }
                    }
                }

                // Then check tasks
                for task in &doc.tasks {
                    if task.name == task_name {
                        self.callee = Some(CalleeRef::Task(task.clone()));
                        return Ok(());
                    }
                }

                Err(WdlError::validation_error(
                    self.pos.clone(),
                    format!("No such task or workflow: {}", task_name),
                ))
            }
            2 => {
                // Namespaced call - look in imported documents
                let namespace = parts[0];
                let task_name = parts[1];

                // Find the import with matching namespace
                for import in &doc.imports {
                    if import.namespace == namespace {
                        if let Some(ref imported_doc) = import.doc {
                            // First check workflows in imported document
                            if let Some(ref workflow) = imported_doc.workflow {
                                if workflow.name == task_name {
                                    if workflow.complete_calls.unwrap_or(false) {
                                        self.callee = Some(CalleeRef::Workflow(workflow.clone()));
                                        return Ok(());
                                    } else {
                                        return Err(WdlError::validation_error(
                                            self.pos.clone(),
                                            format!(
                                                "Workflow {}.{} is not callable (incomplete calls)",
                                                namespace, task_name
                                            ),
                                        ));
                                    }
                                }
                            }

                            // Then check tasks in imported document
                            for task in &imported_doc.tasks {
                                if task.name == task_name {
                                    self.callee = Some(CalleeRef::Task(task.clone()));
                                    return Ok(());
                                }
                            }

                            return Err(WdlError::validation_error(
                                self.pos.clone(),
                                format!("No such task or workflow in {}: {}", namespace, task_name),
                            ));
                        } else {
                            return Err(WdlError::validation_error(
                                self.pos.clone(),
                                format!("Import {} not resolved", namespace),
                            ));
                        }
                    }
                }

                Err(WdlError::validation_error(
                    self.pos.clone(),
                    format!("No such import namespace: {}", namespace),
                ))
            }
            _ => Err(WdlError::validation_error(
                self.pos.clone(),
                format!("Invalid call syntax: {}", self.task),
            )),
        }
    }

    /// Get the effective name of this call (alias if present, otherwise task name)
    pub fn name(&self) -> &str {
        match &self.alias {
            Some(alias) => alias,
            None => {
                // For namespaced calls like "hello.hello_task", return the last segment "hello_task"
                self.task.split('.').next_back().unwrap_or(&self.task)
            }
        }
    }
}

impl HasSourcePosition for Call {
    fn source_position(&self) -> &SourcePosition {
        &self.pos
    }

    fn set_source_position(&mut self, new_pos: SourcePosition) {
        self.pos = new_pos;
    }
}

impl SourceNode for Call {
    fn parent(&self) -> Option<&dyn SourceNode> {
        None
    }

    fn set_parent(&mut self, _parent: Option<&dyn SourceNode>) {
        // Not implemented for simplicity
    }

    fn children(&self) -> Vec<&dyn SourceNode> {
        let mut children: Vec<&dyn SourceNode> = Vec::new();
        for expr in self.inputs.values() {
            children.push(expr);
        }
        children
    }
}

impl WorkflowNode for Call {
    fn workflow_node_id(&self) -> &str {
        &self.workflow_node_id
    }

    fn workflow_node_dependencies(&self) -> Vec<String> {
        // TODO: Add dependencies from input expressions
        self.afters.clone()
    }

    fn scatter_depth(&self) -> u32 {
        self.scatter_depth
    }

    fn set_scatter_depth(&mut self, depth: u32) {
        self.scatter_depth = depth;
    }
}

impl ASTNode for Call {
    fn children(&self) -> Vec<&dyn SourceNode> {
        SourceNode::children(self)
    }

    fn accept<T>(&self, visitor: &mut dyn ASTVisitor<T>) -> Result<T, WdlError> {
        visitor.visit_call(self)
    }
}

/// Gather operation for arrays (implicit in scatter/conditional sections)
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Gather {
    pub pos: SourcePosition,
    pub workflow_node_id: String,
    pub scatter_depth: u32,
    pub section: String, // ID of the scatter/conditional section
    pub final_type: Type,
    pub referee_id: Option<String>, // ID of the node being gathered (for miniwdl compatibility)
}

impl Gather {
    pub fn new(pos: SourcePosition, section: String, final_type: Type) -> Self {
        let workflow_node_id = format!("gather-{}", section);

        Self {
            pos,
            workflow_node_id,
            scatter_depth: 0,
            section,
            final_type,
            referee_id: None,
        }
    }

    pub fn new_with_referee(
        pos: SourcePosition,
        section: String,
        referee_id: String,
        final_type: Type,
    ) -> Self {
        let workflow_node_id = format!("gather-{}", referee_id);

        Self {
            pos,
            workflow_node_id,
            scatter_depth: 0,
            section,
            final_type,
            referee_id: Some(referee_id),
        }
    }
}

impl HasSourcePosition for Gather {
    fn source_position(&self) -> &SourcePosition {
        &self.pos
    }

    fn set_source_position(&mut self, new_pos: SourcePosition) {
        self.pos = new_pos;
    }
}

impl SourceNode for Gather {
    fn parent(&self) -> Option<&dyn SourceNode> {
        None
    }

    fn set_parent(&mut self, _parent: Option<&dyn SourceNode>) {
        // Not implemented for simplicity
    }

    fn children(&self) -> Vec<&dyn SourceNode> {
        Vec::new() // Gather nodes don't have direct children
    }
}

impl WorkflowNode for Gather {
    fn workflow_node_id(&self) -> &str {
        &self.workflow_node_id
    }

    fn workflow_node_dependencies(&self) -> Vec<String> {
        vec![self.section.clone()]
    }

    fn scatter_depth(&self) -> u32 {
        self.scatter_depth
    }

    fn set_scatter_depth(&mut self, depth: u32) {
        self.scatter_depth = depth;
    }
}

impl ASTNode for Gather {
    fn children(&self) -> Vec<&dyn SourceNode> {
        SourceNode::children(self)
    }

    fn accept<T>(&self, visitor: &mut dyn ASTVisitor<T>) -> Result<T, WdlError> {
        visitor.visit_gather(self)
    }
}

/// Workflow node enum for unified handling
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub enum WorkflowElement {
    Declaration(Declaration),
    Call(Call),
    Scatter(Box<Scatter>),
    Conditional(Box<Conditional>),
}

/// Base class for workflow sections (scatter, conditional)
pub trait WorkflowSection: WorkflowNode {
    /// Get the body elements of this section
    fn body(&self) -> &[WorkflowElement];

    /// Get the mutable body elements of this section
    fn body_mut(&mut self) -> &mut Vec<WorkflowElement>;
}

/// Scatter section for parallel execution
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Scatter {
    pub pos: SourcePosition,
    pub workflow_node_id: String,
    pub scatter_depth: u32,
    pub variable: String,
    pub expr: Expression,
    pub body: Vec<WorkflowElement>,
    pub gathers: HashMap<String, Gather>, // Gather nodes for collecting outputs
}

impl Scatter {
    pub fn new(
        pos: SourcePosition,
        variable: String,
        expr: Expression,
        body: Vec<WorkflowElement>,
    ) -> Self {
        let workflow_node_id = format!("scatter-{}", variable);
        let mut scatter = Self {
            pos: pos.clone(),
            workflow_node_id,
            scatter_depth: 0,
            variable,
            expr,
            body,
            gathers: HashMap::new(),
        };

        // Create gather nodes for declarations and calls in the scatter body
        scatter.create_gathers();
        scatter
    }

    /// Create gather nodes for scatter body elements
    fn create_gathers(&mut self) {
        for element in &self.body {
            match element {
                WorkflowElement::Declaration(decl) => {
                    let gather = Gather::new_with_referee(
                        decl.pos.clone(),
                        self.workflow_node_id.clone(),
                        decl.workflow_node_id.clone(),
                        decl.decl_type.clone(),
                    );
                    self.gathers.insert(decl.workflow_node_id.clone(), gather);
                }
                WorkflowElement::Call(call) => {
                    // For task calls, we need to create gathers for each output
                    // For now, create a simple gather for the call itself
                    let gather = Gather::new_with_referee(
                        call.pos.clone(),
                        self.workflow_node_id.clone(),
                        call.workflow_node_id.clone(),
                        Type::object(HashMap::new()), // Placeholder - should be task outputs
                    );
                    self.gathers.insert(call.workflow_node_id.clone(), gather);
                }
                WorkflowElement::Scatter(nested_scatter) => {
                    // Handle nested scatters - gather their gathers
                    for (gather_id, gather) in &nested_scatter.gathers {
                        let nested_gather = Gather::new_with_referee(
                            gather.pos.clone(),
                            self.workflow_node_id.clone(),
                            gather.workflow_node_id.clone(),
                            gather.final_type.clone(),
                        );
                        self.gathers.insert(gather_id.clone(), nested_gather);
                    }
                }
                WorkflowElement::Conditional(_conditional) => {
                    // Handle conditional sections - gather their potential outputs
                    // For now, skip conditionals as they need different handling (optional types)
                }
            }
        }
    }
}

impl HasSourcePosition for Scatter {
    fn source_position(&self) -> &SourcePosition {
        &self.pos
    }

    fn set_source_position(&mut self, new_pos: SourcePosition) {
        self.pos = new_pos;
    }
}

impl SourceNode for Scatter {
    fn parent(&self) -> Option<&dyn SourceNode> {
        None
    }

    fn set_parent(&mut self, _parent: Option<&dyn SourceNode>) {
        // Not implemented for simplicity
    }

    fn children(&self) -> Vec<&dyn SourceNode> {
        let children: Vec<&dyn SourceNode> = vec![&self.expr];
        // TODO: Add body elements as children
        children
    }
}

impl WorkflowNode for Scatter {
    fn workflow_node_id(&self) -> &str {
        &self.workflow_node_id
    }

    fn workflow_node_dependencies(&self) -> Vec<String> {
        // TODO: Extract dependencies from scatter expression
        Vec::new()
    }

    fn scatter_depth(&self) -> u32 {
        self.scatter_depth
    }

    fn set_scatter_depth(&mut self, depth: u32) {
        self.scatter_depth = depth;
        // Increment scatter depth for all body elements
        for element in &mut self.body {
            match element {
                WorkflowElement::Declaration(decl) => decl.set_scatter_depth(depth + 1),
                WorkflowElement::Call(call) => call.set_scatter_depth(depth + 1),
                WorkflowElement::Scatter(scatter) => scatter.set_scatter_depth(depth + 1),
                WorkflowElement::Conditional(cond) => cond.set_scatter_depth(depth + 1),
            }
        }
    }
}

impl WorkflowSection for Scatter {
    fn body(&self) -> &[WorkflowElement] {
        &self.body
    }

    fn body_mut(&mut self) -> &mut Vec<WorkflowElement> {
        &mut self.body
    }
}

impl ASTNode for Scatter {
    fn children(&self) -> Vec<&dyn SourceNode> {
        SourceNode::children(self)
    }

    fn accept<T>(&self, visitor: &mut dyn ASTVisitor<T>) -> Result<T, WdlError> {
        visitor.visit_scatter(self)
    }
}

/// Conditional section
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Conditional {
    pub pos: SourcePosition,
    pub workflow_node_id: String,
    pub scatter_depth: u32,
    pub expr: Expression,
    pub body: Vec<WorkflowElement>,
}

impl Conditional {
    pub fn new(pos: SourcePosition, expr: Expression, body: Vec<WorkflowElement>) -> Self {
        // Create a unique ID for this conditional
        let workflow_node_id = format!("if-{}", pos.line);

        Self {
            pos,
            workflow_node_id,
            scatter_depth: 0,
            expr,
            body,
        }
    }
}

impl HasSourcePosition for Conditional {
    fn source_position(&self) -> &SourcePosition {
        &self.pos
    }

    fn set_source_position(&mut self, new_pos: SourcePosition) {
        self.pos = new_pos;
    }
}

impl SourceNode for Conditional {
    fn parent(&self) -> Option<&dyn SourceNode> {
        None
    }

    fn set_parent(&mut self, _parent: Option<&dyn SourceNode>) {
        // Not implemented for simplicity
    }

    fn children(&self) -> Vec<&dyn SourceNode> {
        let children: Vec<&dyn SourceNode> = vec![&self.expr];
        // TODO: Add body elements as children
        children
    }
}

impl WorkflowNode for Conditional {
    fn workflow_node_id(&self) -> &str {
        &self.workflow_node_id
    }

    fn workflow_node_dependencies(&self) -> Vec<String> {
        // TODO: Extract dependencies from conditional expression
        Vec::new()
    }

    fn scatter_depth(&self) -> u32 {
        self.scatter_depth
    }

    fn set_scatter_depth(&mut self, depth: u32) {
        self.scatter_depth = depth;
        // Set scatter depth for all body elements (same depth, not incremented)
        for element in &mut self.body {
            match element {
                WorkflowElement::Declaration(decl) => decl.set_scatter_depth(depth),
                WorkflowElement::Call(call) => call.set_scatter_depth(depth),
                WorkflowElement::Scatter(scatter) => scatter.set_scatter_depth(depth),
                WorkflowElement::Conditional(cond) => cond.set_scatter_depth(depth),
            }
        }
    }
}

impl WorkflowSection for Conditional {
    fn body(&self) -> &[WorkflowElement] {
        &self.body
    }

    fn body_mut(&mut self) -> &mut Vec<WorkflowElement> {
        &mut self.body
    }
}

impl ASTNode for Conditional {
    fn children(&self) -> Vec<&dyn SourceNode> {
        SourceNode::children(self)
    }

    fn accept<T>(&self, visitor: &mut dyn ASTVisitor<T>) -> Result<T, WdlError> {
        visitor.visit_conditional(self)
    }
}

/// WDL Workflow definition
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Workflow {
    pub pos: SourcePosition,
    pub name: String,
    pub inputs: Vec<Declaration>,
    pub postinputs: Vec<Declaration>,
    pub body: Vec<WorkflowElement>,
    pub outputs: Vec<Declaration>,
    pub parameter_meta: HashMap<String, serde_json::Value>,
    pub meta: HashMap<String, serde_json::Value>,
    pub effective_wdl_version: String,
    pub complete_calls: Option<bool>, // Whether all calls have complete inputs

    // Type environment after typechecking (for runtime use)
    #[serde(skip)]
    #[cfg_attr(test, allow(unused))]
    pub type_env: Option<Bindings<Type>>,
}

impl Workflow {
    pub fn new(
        pos: SourcePosition,
        name: String,
        inputs: Vec<Declaration>,
        postinputs: Vec<Declaration>,
        body: Vec<WorkflowElement>,
        outputs: Vec<Declaration>,
        parameter_meta: HashMap<String, serde_json::Value>,
        meta: HashMap<String, serde_json::Value>,
    ) -> Self {
        Self {
            pos,
            name,
            inputs,
            postinputs,
            body,
            outputs,
            parameter_meta,
            meta,
            effective_wdl_version: "1.0".to_string(),
            complete_calls: None,
            type_env: None,
        }
    }

    /// Type check this workflow
    pub fn typecheck(&mut self, struct_typedefs: &[StructTypeDef]) -> Result<(), WdlError> {
        // Create stdlib instance for type checking
        let stdlib = crate::stdlib::StdLib::new(&self.effective_wdl_version);

        // Single-phase type checking: build type environment and typecheck simultaneously
        let mut type_env = Bindings::new();

        // 1. Add input declarations to type environment and typecheck
        for input in &mut self.inputs {
            type_env = input.add_to_type_env(&Bindings::new(), type_env, false)?;
            input.typecheck(&type_env, &stdlib, struct_typedefs)?;
        }

        // 2. Add postinput declarations to type environment and typecheck
        for postinput in &mut self.postinputs {
            type_env = postinput.add_to_type_env(&Bindings::new(), type_env, false)?;
            postinput.typecheck(&type_env, &stdlib, struct_typedefs)?;
        }

        // 3. Process workflow body elements (type environment built on-demand)
        Self::typecheck_workflow_elements(&mut self.body, &mut type_env, &stdlib, struct_typedefs)?;
        // 4. Process output declarations (type check and add to environment)
        for output in &mut self.outputs {
            output.typecheck(&type_env, &stdlib, struct_typedefs)?;
            type_env = type_env.bind(output.name.clone(), output.decl_type.clone(), None);
        }

        // 5. Save type environment for runtime use
        self.type_env = Some(type_env);

        Ok(())
    }
}

impl PartialEq for Workflow {
    fn eq(&self, other: &Self) -> bool {
        // Compare all fields except type_env (which is runtime data)
        self.pos == other.pos
            && self.name == other.name
            && self.inputs == other.inputs
            && self.postinputs == other.postinputs
            && self.body == other.body
            && self.outputs == other.outputs
            && self.parameter_meta == other.parameter_meta
            && self.meta == other.meta
            && self.effective_wdl_version == other.effective_wdl_version
            && self.complete_calls == other.complete_calls
    }
}

impl Workflow {
    /// Type check a list of workflow elements
    fn typecheck_workflow_elements(
        elements: &mut [WorkflowElement],
        type_env: &mut Bindings<Type>,
        stdlib: &crate::stdlib::StdLib,
        struct_typedefs: &[StructTypeDef],
    ) -> Result<(), WdlError> {
        for element in elements {
            Self::typecheck_workflow_element(element, type_env, stdlib, struct_typedefs)?;
        }
        Ok(())
    }

    fn typecheck_workflow_element(
        element: &mut WorkflowElement,
        type_env: &mut Bindings<Type>,
        stdlib: &crate::stdlib::StdLib,
        struct_typedefs: &[StructTypeDef],
    ) -> Result<(), WdlError> {
        match element {
            WorkflowElement::Declaration(decl) => {
                // Type check the declaration and add to environment
                if let Some(ref mut expr) = decl.expr {
                    expr.infer_type(type_env, stdlib, struct_typedefs)?;
                    let inferred_type = expr
                        .get_type()
                        .unwrap_or(&Type::String { optional: false })
                        .clone();

                    // Check if inferred type can be coerced to declared type
                    if !inferred_type.coerces(&decl.decl_type, false) {
                        return Err(WdlError::static_type_mismatch(
                            decl.pos.clone(),
                            decl.decl_type.to_string(),
                            inferred_type.to_string(),
                            format!(
                                "Cannot coerce expression type '{}' to declared type '{}'",
                                inferred_type, decl.decl_type
                            ),
                        ));
                    }
                }

                // If this is a struct type, resolve the members from struct_typedefs
                let resolved_type = match &decl.decl_type {
                    Type::StructInstance {
                        type_name,
                        members: None,
                        optional,
                    } => {
                        // Look up the struct definition to get member types
                        if let Some(struct_def) =
                            struct_typedefs.iter().find(|s| s.name == *type_name)
                        {
                            Type::StructInstance {
                                type_name: type_name.clone(),
                                members: Some(struct_def.members.clone()),
                                optional: *optional,
                            }
                        } else {
                            decl.decl_type.clone()
                        }
                    }
                    _ => decl.decl_type.clone(),
                };

                // Update the declaration type itself with resolved members
                decl.decl_type = resolved_type.clone();
                *type_env = type_env.bind(decl.name.clone(), resolved_type, None);
            }
            WorkflowElement::Call(call) => {
                // Type check call inputs
                for expr in call.inputs.values_mut() {
                    expr.infer_type(type_env, stdlib, struct_typedefs)?;
                }

                // Add call outputs to type environment
                if let Some(ref callee) = call.callee {
                    match callee {
                        CalleeRef::Task(task) => {
                            // Create an object type with task outputs
                            let mut output_members = HashMap::new();
                            for output in &task.outputs {
                                output_members
                                    .insert(output.name.clone(), output.decl_type.clone());
                            }
                            let call_output_type = Type::object_call_output(output_members);
                            *type_env =
                                type_env.bind(call.name().to_string(), call_output_type, None);
                        }
                        CalleeRef::Workflow(workflow) => {
                            // Create an object type with workflow outputs
                            let mut output_members = HashMap::new();
                            for output in &workflow.outputs {
                                output_members
                                    .insert(output.name.clone(), output.decl_type.clone());
                            }
                            let call_output_type = Type::object_call_output(output_members);
                            *type_env =
                                type_env.bind(call.name().to_string(), call_output_type, None);
                        }
                    }
                } else {
                    return Err(WdlError::validation_error(
                        call.pos.clone(),
                        format!("Call {} has not been resolved", call.name()),
                    ));
                }
            }
            WorkflowElement::Scatter(scatter) => {
                // Type check scatter expression with stdlib access
                scatter.expr.infer_type(type_env, stdlib, struct_typedefs)?;

                // Create nested type environment with scatter variable
                let mut scatter_env = type_env.clone();
                if let Some(item_type) = scatter.expr.get_type() {
                    if let Type::Array {
                        item_type: inner, ..
                    } = item_type
                    {
                        scatter_env = scatter_env.bind(
                            scatter.variable.clone(),
                            inner.as_ref().clone(),
                            None,
                        );
                    }
                }

                // Store the type_env state after adding scatter variable to identify new bindings
                let original_env = scatter_env.clone();

                // Process scatter body and collect declarations for parent scope
                Self::typecheck_workflow_elements(
                    &mut scatter.body,
                    &mut scatter_env,
                    stdlib,
                    struct_typedefs,
                )?;

                // Use iterate_until_binding to find all variables added during scatter processing
                // This correctly handles nested conditionals, calls, etc.
                // Since original_env includes the scatter variable, iterate_until_binding automatically excludes it
                let added_bindings = scatter_env.iterate_until_binding(
                    original_env
                        .iter()
                        .next()
                        .expect("original_env should contain scatter variable"),
                );

                for (name, declared_type) in added_bindings {
                    let array_type = Type::array(declared_type, false, false);
                    *type_env = type_env.bind(name, array_type, None);
                }
            }
            WorkflowElement::Conditional(conditional) => {
                // Type check conditional expression with stdlib access
                conditional
                    .expr
                    .infer_type(type_env, stdlib, struct_typedefs)?;

                // Create nested type environment
                let mut cond_env = type_env.clone();

                // Store the original type environment state to identify new bindings
                let original_env = cond_env.clone();

                // Process conditional body and collect declarations for parent scope
                Self::typecheck_workflow_elements(
                    &mut conditional.body,
                    &mut cond_env,
                    stdlib,
                    struct_typedefs,
                )?;

                // Use iterate_until_binding to find all variables added during conditional processing
                // This correctly handles nested scatters, calls, etc.
                if let Some(first_binding) = original_env.iter().next() {
                    let added_bindings = cond_env.iterate_until_binding(first_binding);

                    for (name, declared_type) in added_bindings {
                        let optional_type = declared_type.with_optional(true);
                        *type_env = type_env.bind(name, optional_type, None);
                    }
                } else {
                    // If original_env is empty, all bindings in cond_env are new
                    for binding in cond_env.iter() {
                        let optional_type = binding.value().clone().with_optional(true);
                        *type_env = type_env.bind(binding.name().to_string(), optional_type, None);
                    }
                }
            }
        }
        Ok(())
    }
}

impl HasSourcePosition for Workflow {
    fn source_position(&self) -> &SourcePosition {
        &self.pos
    }

    fn set_source_position(&mut self, new_pos: SourcePosition) {
        self.pos = new_pos;
    }
}

impl SourceNode for Workflow {
    fn parent(&self) -> Option<&dyn SourceNode> {
        None
    }

    fn set_parent(&mut self, _parent: Option<&dyn SourceNode>) {
        // Not implemented for simplicity
    }

    fn children(&self) -> Vec<&dyn SourceNode> {
        let mut children: Vec<&dyn SourceNode> = Vec::new();

        for input in &self.inputs {
            children.push(input);
        }

        for postinput in &self.postinputs {
            children.push(postinput);
        }

        // TODO: Add body elements as children

        for output in &self.outputs {
            children.push(output);
        }

        children
    }
}

impl ASTNode for Workflow {
    fn children(&self) -> Vec<&dyn SourceNode> {
        SourceNode::children(self)
    }

    fn accept<T>(&self, visitor: &mut dyn ASTVisitor<T>) -> Result<T, WdlError> {
        visitor.visit_workflow(self)
    }
}

/// WDL Document (top-level container)
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Document {
    pub pos: SourcePosition,
    pub version: Option<String>,
    pub imports: Vec<ImportDoc>,
    pub struct_typedefs: Vec<StructTypeDef>,
    pub tasks: Vec<Task>,
    pub workflow: Option<Workflow>,
    pub effective_wdl_version: String,
}

impl Document {
    /// Initialize and resolve nested struct types (similar to miniwdl's _initialize_struct_typedefs)
    pub fn resolve_struct_types(&mut self) -> Result<(), WdlError> {
        use std::collections::HashSet;

        // Keep track of structs being resolved to prevent infinite recursion
        let mut resolving: HashSet<String> = HashSet::new();

        // Resolve each struct typedef
        for i in 0..self.struct_typedefs.len() {
            Self::resolve_struct_members(&mut self.struct_typedefs, i, &mut resolving)?;
        }

        Ok(())
    }

    /// Recursively resolve struct member types (similar to miniwdl's _resolve_struct_types)
    fn resolve_struct_members(
        struct_typedefs: &mut [StructTypeDef],
        struct_index: usize,
        resolving: &mut std::collections::HashSet<String>,
    ) -> Result<(), WdlError> {
        let struct_name = struct_typedefs[struct_index].name.clone();

        // Prevent circular dependencies
        if resolving.contains(&struct_name) {
            return Err(WdlError::Validation {
                pos: struct_typedefs[struct_index].pos.clone(),
                message: format!(
                    "Circular dependency detected in struct definition: {}",
                    struct_name
                ),
                source_text: None,
                declared_wdl_version: None,
            });
        }

        resolving.insert(struct_name.clone());

        // Resolve nested struct types in members
        let mut updated_members = IndexMap::new();
        for (member_name, member_type) in &struct_typedefs[struct_index].members {
            let resolved_type =
                Self::resolve_type_recursively(member_type, struct_typedefs, resolving)?;
            updated_members.insert(member_name.clone(), resolved_type);
        }

        // Update the struct definition with resolved member types
        struct_typedefs[struct_index].members = updated_members;

        resolving.remove(&struct_name);
        Ok(())
    }

    /// Recursively resolve a single type, handling nested struct instances
    fn resolve_type_recursively(
        ty: &Type,
        struct_typedefs: &[StructTypeDef],
        resolving: &std::collections::HashSet<String>,
    ) -> Result<Type, WdlError> {
        match ty {
            Type::StructInstance {
                type_name,
                members: None,
                optional,
            } => {
                // Find the struct definition
                if let Some(struct_def) = struct_typedefs.iter().find(|s| s.name == *type_name) {
                    // Prevent circular reference
                    if resolving.contains(type_name) {
                        return Ok(ty.clone()); // Return as-is for circular references
                    }

                    // Recursively resolve member types
                    let mut resolved_members = IndexMap::new();
                    for (member_name, member_type) in &struct_def.members {
                        let resolved_member_type = Self::resolve_type_recursively(
                            member_type,
                            struct_typedefs,
                            resolving,
                        )?;
                        resolved_members.insert(member_name.clone(), resolved_member_type);
                    }

                    Ok(Type::StructInstance {
                        type_name: type_name.clone(),
                        members: Some(resolved_members),
                        optional: *optional,
                    })
                } else {
                    // Struct definition not found - return as-is
                    Ok(ty.clone())
                }
            }
            Type::Array {
                item_type,
                optional,
                nonempty,
            } => {
                let resolved_item_type =
                    Self::resolve_type_recursively(item_type, struct_typedefs, resolving)?;
                Ok(Type::Array {
                    item_type: Box::new(resolved_item_type),
                    optional: *optional,
                    nonempty: *nonempty,
                })
            }
            Type::Map {
                key_type,
                value_type,
                optional,
                literal_keys,
            } => {
                let resolved_key_type =
                    Self::resolve_type_recursively(key_type, struct_typedefs, resolving)?;
                let resolved_value_type =
                    Self::resolve_type_recursively(value_type, struct_typedefs, resolving)?;
                Ok(Type::Map {
                    key_type: Box::new(resolved_key_type),
                    value_type: Box::new(resolved_value_type),
                    optional: *optional,
                    literal_keys: literal_keys.clone(),
                })
            }
            Type::Pair {
                left_type,
                right_type,
                optional,
            } => {
                let resolved_left_type =
                    Self::resolve_type_recursively(left_type, struct_typedefs, resolving)?;
                let resolved_right_type =
                    Self::resolve_type_recursively(right_type, struct_typedefs, resolving)?;
                Ok(Type::Pair {
                    left_type: Box::new(resolved_left_type),
                    right_type: Box::new(resolved_right_type),
                    optional: *optional,
                })
            }
            _ => Ok(ty.clone()),
        }
    }
}

/// Import statement in a WDL document  
/// Matches Python's DocImport NamedTuple structure
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct ImportDoc {
    pub pos: SourcePosition,
    pub uri: String,
    pub namespace: String, // Required - inferred from filename if not provided
    pub aliases: Vec<(String, String)>, // List of (name, alias) pairs
    pub doc: Option<Box<Document>>, // Resolved document after loading
}

impl ImportDoc {
    /// Create a new ImportDoc instance
    pub fn new(
        pos: SourcePosition,
        uri: String,
        namespace: Option<String>,
        aliases: Vec<(String, String)>,
    ) -> Self {
        let namespace = namespace.unwrap_or_else(|| {
            // Infer namespace from filename/URI (matches Python implementation)
            let mut ns = uri.clone();

            // Remove path components
            if let Some(slash_pos) = ns.rfind('/') {
                ns = ns[slash_pos + 1..].to_string();
            }

            // Remove query parameters and file extension
            if let Some(question_pos) = ns.find('?') {
                ns = ns[..question_pos].to_string();
            }
            if let Some(dot_pos) = ns.rfind('.') {
                ns = ns[..dot_pos].to_string();
            }

            ns
        });

        Self {
            pos,
            uri,
            namespace,
            aliases,
            doc: None,
        }
    }

    /// Create a new ImportDoc with a resolved document
    pub fn with_document(mut self, doc: Box<Document>) -> Self {
        self.doc = Some(doc);
        self
    }
}

impl HasSourcePosition for ImportDoc {
    fn source_position(&self) -> &SourcePosition {
        &self.pos
    }

    fn set_source_position(&mut self, new_pos: SourcePosition) {
        self.pos = new_pos;
    }
}

impl SourceNode for ImportDoc {
    fn parent(&self) -> Option<&dyn SourceNode> {
        None
    }

    fn set_parent(&mut self, _parent: Option<&dyn SourceNode>) {
        // Not implemented for simplicity
    }

    fn children(&self) -> Vec<&dyn SourceNode> {
        if let Some(ref doc) = self.doc {
            vec![doc.as_ref()]
        } else {
            Vec::new()
        }
    }
}

impl Document {
    pub fn new(
        pos: SourcePosition,
        version: Option<String>,
        imports: Vec<ImportDoc>,
        struct_typedefs: Vec<StructTypeDef>,
        tasks: Vec<Task>,
        workflow: Option<Workflow>,
    ) -> Self {
        let effective_version = version.clone().unwrap_or_else(|| "draft-2".to_string());

        Self {
            pos,
            version,
            imports,
            struct_typedefs,
            tasks,
            workflow,
            effective_wdl_version: effective_version,
        }
    }

    /// Type check this document
    pub fn typecheck(&mut self) -> Result<(), WdlError> {
        // 1. Check for duplicate import namespaces
        self.check_import_namespaces()?;

        // 2. Import structs from imported documents
        self.import_structs()?;

        // 3. Resolve nested struct types (similar to miniwdl's _initialize_struct_typedefs)
        self.resolve_struct_types()?;

        // 4. Type check all tasks
        for task in &mut self.tasks {
            task.typecheck(&self.struct_typedefs)?;
        }

        // 5. Resolve all calls in workflows
        self.resolve_calls()?;

        // 6. Type check workflow if present
        if let Some(ref mut workflow) = self.workflow {
            // Actually call workflow typecheck instead of just setting complete_calls
            workflow.typecheck(&self.struct_typedefs)?;
            // Set complete_calls based on successful typechecking
            workflow.complete_calls = Some(true);
        }

        Ok(())
    }

    /// Check for duplicate import namespaces
    fn check_import_namespaces(&self) -> Result<(), WdlError> {
        let mut seen_namespaces = std::collections::HashSet::new();

        for import in &self.imports {
            if seen_namespaces.contains(&import.namespace) {
                return Err(WdlError::multiple_definitions_error(
                    import.pos.clone(),
                    format!("Multiple imports with namespace {}", import.namespace),
                ));
            }
            seen_namespaces.insert(&import.namespace);
        }

        Ok(())
    }

    /// Import struct types from imported documents
    fn import_structs(&mut self) -> Result<(), WdlError> {
        let mut imported_structs = std::collections::HashMap::new();

        for import in &self.imports {
            if let Some(ref imported_doc) = import.doc {
                // Collect structs from imported document
                let mut doc_structs = std::collections::HashMap::new();
                for struct_def in &imported_doc.struct_typedefs {
                    doc_structs.insert(struct_def.name.clone(), struct_def.clone());
                }

                // Process aliases
                for (original_name, alias_name) in &import.aliases {
                    // Check if the aliased struct exists in the imported document
                    if !doc_structs.contains_key(original_name) {
                        return Err(WdlError::no_such_member_error(
                            import.pos.clone(),
                            original_name.clone(),
                        ));
                    }

                    // Check for collisions with other imported structs in this import
                    if alias_name != original_name && doc_structs.contains_key(alias_name) {
                        return Err(WdlError::multiple_definitions_error(
                            import.pos.clone(),
                            format!(
                                "struct type alias {} collides with another struct type in the imported document",
                                alias_name
                            ),
                        ));
                    }

                    // Check for collisions with existing structs in this document
                    if self.struct_typedefs.iter().any(|s| s.name == *alias_name) {
                        return Err(WdlError::multiple_definitions_error(
                            import.pos.clone(),
                            format!(
                                "struct type alias {} collides with a struct type in this document",
                                alias_name
                            ),
                        ));
                    }

                    // Add/rename the struct
                    if let Some(original_struct) = doc_structs.get(original_name) {
                        let mut aliased_struct = original_struct.clone();
                        aliased_struct.name = alias_name.clone();
                        aliased_struct.imported = Some((
                            Box::new(imported_doc.as_ref().clone()),
                            Box::new(original_struct.clone()),
                        ));
                        imported_structs.insert(alias_name.clone(), aliased_struct);
                    }

                    // Remove original if renamed
                    if alias_name != original_name {
                        doc_structs.remove(original_name);
                    }
                }

                // Add remaining non-aliased structs
                for (name, struct_def) in doc_structs {
                    // Check for naming conflicts with existing structs
                    if let Some(existing) = self.struct_typedefs.iter().find(|s| s.name == name) {
                        // Check if types are compatible (same structure)
                        if existing.type_id() != struct_def.type_id() {
                            return Err(WdlError::multiple_definitions_error(
                                import.pos.clone(),
                                format!(
                                    "imported struct {} must be aliased because it collides with a struct type in this document",
                                    name
                                ),
                            ));
                        }
                        // Types are compatible, skip adding
                        continue;
                    }

                    // Create imported struct with reference to original
                    let mut imported_struct = struct_def.clone();
                    imported_struct.imported = Some((
                        Box::new(imported_doc.as_ref().clone()),
                        Box::new(struct_def),
                    ));
                    imported_structs.insert(name, imported_struct);
                }
            }
        }

        // Add imported structs to this document
        for (_, imported_struct) in imported_structs {
            self.struct_typedefs.push(imported_struct);
        }

        Ok(())
    }

    /// Resolve all calls in the workflow
    fn resolve_calls(&mut self) -> Result<(), WdlError> {
        // Take ownership of workflow temporarily to avoid borrowing issues
        if let Some(mut workflow) = self.workflow.take() {
            Self::resolve_calls_in_workflow(self, &mut workflow)?;
            self.workflow = Some(workflow);
        }
        Ok(())
    }

    /// Recursively resolve calls in workflow sections
    fn resolve_calls_in_workflow(doc: &Document, workflow: &mut Workflow) -> Result<(), WdlError> {
        for node in &mut workflow.body {
            Self::resolve_calls_in_node(doc, node)?;
        }
        Ok(())
    }

    /// Resolve calls in a workflow node (recursively for sections)
    fn resolve_calls_in_node(doc: &Document, node: &mut WorkflowElement) -> Result<(), WdlError> {
        match node {
            WorkflowElement::Call(ref mut call) => {
                call.resolve(doc)?;
            }
            WorkflowElement::Scatter(ref mut scatter) => {
                for inner_node in &mut scatter.body {
                    Self::resolve_calls_in_node(doc, inner_node)?;
                }
            }
            WorkflowElement::Conditional(ref mut conditional) => {
                for inner_node in &mut conditional.body {
                    Self::resolve_calls_in_node(doc, inner_node)?;
                }
            }
            WorkflowElement::Declaration(_) => {
                // Declarations don't contain calls
            }
        }
        Ok(())
    }
}

impl HasSourcePosition for Document {
    fn source_position(&self) -> &SourcePosition {
        &self.pos
    }

    fn set_source_position(&mut self, new_pos: SourcePosition) {
        self.pos = new_pos;
    }
}

impl SourceNode for Document {
    fn parent(&self) -> Option<&dyn SourceNode> {
        None
    }

    fn set_parent(&mut self, _parent: Option<&dyn SourceNode>) {
        // Not implemented for simplicity
    }

    fn children(&self) -> Vec<&dyn SourceNode> {
        let mut children: Vec<&dyn SourceNode> = Vec::new();

        for typedef in &self.struct_typedefs {
            children.push(typedef);
        }

        for task in &self.tasks {
            children.push(task);
        }

        if let Some(ref workflow) = self.workflow {
            children.push(workflow);
        }

        children
    }
}

impl ASTNode for Document {
    fn children(&self) -> Vec<&dyn SourceNode> {
        SourceNode::children(self)
    }

    fn accept<T>(&self, visitor: &mut dyn ASTVisitor<T>) -> Result<T, WdlError> {
        visitor.visit_document(self)
    }
}

#[cfg(test)]
mod scatter_call_tests {
    use super::*;
    use crate::parser::document::parse_document;
    use crate::stdlib::StdLib;
    use std::sync::Arc;

    #[test]
    fn test_scatter_call_type_inference() {
        let wdl_code = r#"
version 1.2

task gt_three {
  command <<< >>>
  output {
    Boolean valid = true
  }
}

workflow test_conditional {
  input {
    Array[Int] scatter_range = [1, 2, 3, 4, 5]
  }

  scatter (i in scatter_range) {
    call gt_three 
    
    Boolean result2 = gt_three.valid
  }

  output {
    Array[Boolean]? maybe_result2 = result2
  }
}
"#;

        // Parse the document
        let mut document = parse_document(wdl_code, "1.2").expect("Failed to parse WDL");

        // Type check the document
        let result = document.typecheck();

        // Print debug information if it fails
        if let Err(ref e) = result {
            eprintln!("Type check error: {:?}", e);
        }

        // The type check should succeed
        assert!(
            result.is_ok(),
            "Type checking should succeed for scatter call"
        );
    }

    #[test]
    fn test_scatter_call_minimal_reproduction() {
        // Test the core issue: call outputs not being properly bound in scatter
        let mut type_env = Bindings::new();

        // Add scatter variable
        type_env = type_env.bind("i".to_string(), Type::int(false), None);

        // Add task call result (what should happen during call processing)
        let task_result_type = Type::object(
            [("valid".to_string(), Type::boolean(false))]
                .into_iter()
                .collect(),
        );
        type_env = type_env.bind("gt_three".to_string(), task_result_type, None);

        // Verify the binding exists
        let resolved = type_env.resolve("gt_three");
        assert!(
            resolved.is_some(),
            "Task call result should be bound in type environment"
        );

        // Test dotted access resolution
        if let Some(Type::Object { members, .. }) = resolved {
            let valid_type = members.get("valid");
            assert!(
                valid_type.is_some(),
                "Task output field should be accessible"
            );
            assert_eq!(valid_type.unwrap(), &Type::boolean(false));
        } else {
            panic!("Task result should be Object type");
        }
    }
}
#[cfg(test)]
mod tests {
    use super::*;
    use crate::expr::Expression;
    use std::collections::HashMap;

    #[test]
    fn test_declaration_creation() {
        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 10);
        let decl = Declaration::new(
            pos.clone(),
            Type::int(false),
            "x".to_string(),
            Some(Expression::int(pos.clone(), 42)),
            "decl",
        );

        assert_eq!(decl.name, "x");
        assert_eq!(decl.decl_type, Type::int(false));
        assert_eq!(decl.workflow_node_id, "decl-x");
        assert!(decl.expr.is_some());
    }

    #[test]
    fn test_task_creation() {
        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 10);
        let task = Task::new(
            pos.clone(),
            "my_task".to_string(),
            Vec::new(),
            Vec::new(),
            Expression::string_literal(pos.clone(), "echo hello".to_string()),
            Vec::new(),
            HashMap::new(),
            HashMap::new(),
            HashMap::new(),
        );

        assert_eq!(task.name, "my_task");
        assert_eq!(task.effective_wdl_version, "1.0");
        assert!(task.inputs.is_empty());
        assert!(task.postinputs.is_empty());
        assert!(task.outputs.is_empty());
    }

    #[test]
    fn test_call_creation() {
        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 10);
        let call = Call::new(
            pos.clone(),
            "my_task".to_string(),
            Some("alias_name".to_string()),
            HashMap::new(),
            Vec::new(),
        );

        assert_eq!(call.task, "my_task");
        assert_eq!(call.alias, Some("alias_name".to_string()));
        assert_eq!(call.name(), "alias_name");
        assert_eq!(call.workflow_node_id, "call-alias_name");
    }

    #[test]
    fn test_workflow_creation() {
        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 10);
        let workflow = Workflow::new(
            pos.clone(),
            "my_workflow".to_string(),
            Vec::new(),
            Vec::new(),
            Vec::new(),
            Vec::new(),
            HashMap::new(),
            HashMap::new(),
        );

        assert_eq!(workflow.name, "my_workflow");
        assert_eq!(workflow.effective_wdl_version, "1.0");
        assert!(workflow.inputs.is_empty());
        assert!(workflow.body.is_empty());
        assert!(workflow.outputs.is_empty());
    }

    #[test]
    fn test_document_creation() {
        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 10);
        let doc = Document::new(
            pos.clone(),
            Some("1.0".to_string()),
            Vec::new(),
            Vec::new(),
            Vec::new(),
            None,
        );

        assert_eq!(doc.version, Some("1.0".to_string()));
        assert_eq!(doc.effective_wdl_version, "1.0");
        assert!(doc.imports.is_empty());
        assert!(doc.tasks.is_empty());
        assert!(doc.workflow.is_none());
    }

    #[test]
    fn test_scatter_creation() {
        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 10);
        let scatter = Scatter::new(
            pos.clone(),
            "item".to_string(),
            Expression::ident(pos.clone(), "items".to_string()),
            Vec::new(),
        );

        assert_eq!(scatter.variable, "item");
        assert_eq!(scatter.workflow_node_id, "scatter-item");
        assert_eq!(scatter.scatter_depth, 0);
        assert!(scatter.body.is_empty());
    }

    #[test]
    fn test_conditional_creation() {
        let pos = SourcePosition::new("test.wdl".to_string(), "test.wdl".to_string(), 1, 1, 1, 10);
        let conditional = Conditional::new(
            pos.clone(),
            Expression::boolean(pos.clone(), true),
            Vec::new(),
        );

        assert_eq!(conditional.workflow_node_id, "if-1");
        assert_eq!(conditional.scatter_depth, 0);
        assert!(conditional.body.is_empty());
    }
}
