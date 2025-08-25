//! WDL Abstract Syntax Tree (AST) for documents, tasks, and workflows
//!
//! This module contains the AST representation for WDL documents, including tasks,
//! workflows, declarations, calls, and control flow sections. The AST is typically
//! constructed by the parser and used for type checking and execution.

use crate::error::{SourcePosition, WdlError, SourceNode, HasSourcePosition};
use crate::env::Bindings;
use crate::types::Type;
use crate::expr::Expression;
use std::collections::HashMap;
use serde::{Deserialize, Serialize};

// Re-export submodules
pub mod document;
pub mod workflow;
pub mod task;
pub mod declarations;
pub mod control_flow;
pub mod validation;
pub mod traversal;

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
    pub members: HashMap<String, Type>,
    pub imported: Option<(Box<Document>, Box<StructTypeDef>)>,
}

impl StructTypeDef {
    pub fn new(
        pos: SourcePosition,
        name: String,
        members: HashMap<String, Type>,
        imported: Option<(Box<Document>, Box<StructTypeDef>)>,
    ) -> Self {
        Self {
            pos,
            name,
            members,
            imported,
        }
    }
    
    /// Get a canonical type ID for this struct
    pub fn type_id(&self) -> String {
        // Create a canonical representation of member types
        let mut member_strs: Vec<String> = self.members
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
        _struct_types: &Bindings<HashMap<String, Type>>,
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
    pub fn typecheck(&mut self, type_env: &Bindings<Type>) -> Result<(), WdlError> {
        if let Some(ref mut expr) = self.expr {
            let inferred_type = expr.infer_type(type_env)?;
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
    pub inputs: Option<Vec<Declaration>>,
    pub postinputs: Vec<Declaration>,
    pub command: Expression, // Should be a String expression
    pub outputs: Vec<Declaration>,
    pub parameter_meta: HashMap<String, serde_json::Value>,
    pub runtime: HashMap<String, Expression>,
    pub meta: HashMap<String, serde_json::Value>,
    pub effective_wdl_version: String,
}

impl Task {
    pub fn new(
        pos: SourcePosition,
        name: String,
        inputs: Option<Vec<Declaration>>,
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
            meta,
            effective_wdl_version: "1.0".to_string(),
        }
    }
    
    /// Get available inputs for this task
    pub fn available_inputs(&self) -> Bindings<&Declaration> {
        let mut bindings = Bindings::new();
        
        let declarations = if let Some(ref inputs) = self.inputs {
            inputs
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
        
        if let Some(ref inputs) = self.inputs {
            for input in inputs {
                children.push(input);
            }
        }
        
        for postinput in &self.postinputs {
            children.push(postinput);
        }
        
        children.push(&self.command);
        
        for output in &self.outputs {
            children.push(output);
        }
        
        for (_, expr) in &self.runtime {
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
    pub task: String,
    pub alias: Option<String>,
    pub inputs: HashMap<String, Expression>,
    pub afters: Vec<String>,
}

impl Call {
    pub fn new(
        pos: SourcePosition,
        task: String,
        alias: Option<String>,
        inputs: HashMap<String, Expression>,
        afters: Vec<String>,
    ) -> Self {
        let workflow_node_id = format!(
            "call-{}",
            alias.as_ref().unwrap_or(&task)
        );
        
        Self {
            pos,
            workflow_node_id,
            scatter_depth: 0,
            task,
            alias,
            inputs,
            afters,
        }
    }
    
    /// Get the effective name of this call (alias if present, otherwise task name)
    pub fn name(&self) -> &str {
        self.alias.as_ref().unwrap_or(&self.task)
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
        for (_, expr) in &self.inputs {
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
        let deps = self.afters.clone();
        // TODO: Add dependencies from input expressions
        deps
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
}

impl Gather {
    pub fn new(
        pos: SourcePosition,
        section: String,
        final_type: Type,
    ) -> Self {
        let workflow_node_id = format!("gather-{}", section);
        
        Self {
            pos,
            workflow_node_id,
            scatter_depth: 0,
            section,
            final_type,
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
}

impl Scatter {
    pub fn new(
        pos: SourcePosition,
        variable: String,
        expr: Expression,
        body: Vec<WorkflowElement>,
    ) -> Self {
        let workflow_node_id = format!("scatter-{}", variable);
        
        Self {
            pos,
            workflow_node_id,
            scatter_depth: 0,
            variable,
            expr,
            body,
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
        let mut children: Vec<&dyn SourceNode> = Vec::new();
        children.push(&self.expr);
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
    pub fn new(
        pos: SourcePosition,
        expr: Expression,
        body: Vec<WorkflowElement>,
    ) -> Self {
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
        let mut children: Vec<&dyn SourceNode> = Vec::new();
        children.push(&self.expr);
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
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Workflow {
    pub pos: SourcePosition,
    pub name: String,
    pub inputs: Option<Vec<Declaration>>,
    pub postinputs: Vec<Declaration>,
    pub body: Vec<WorkflowElement>,
    pub outputs: Option<Vec<Declaration>>,
    pub parameter_meta: HashMap<String, serde_json::Value>,
    pub meta: HashMap<String, serde_json::Value>,
    pub effective_wdl_version: String,
}

impl Workflow {
    pub fn new(
        pos: SourcePosition,
        name: String,
        inputs: Option<Vec<Declaration>>,
        postinputs: Vec<Declaration>,
        body: Vec<WorkflowElement>,
        outputs: Option<Vec<Declaration>>,
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
        }
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
        
        if let Some(ref inputs) = self.inputs {
            for input in inputs {
                children.push(input);
            }
        }
        
        for postinput in &self.postinputs {
            children.push(postinput);
        }
        
        // TODO: Add body elements as children
        
        if let Some(ref outputs) = self.outputs {
            for output in outputs {
                children.push(output);
            }
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

/// Import statement in a WDL document
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct ImportDoc {
    pub pos: SourcePosition,
    pub uri: String,
    pub namespace: Option<String>,
    pub aliases: HashMap<String, String>,
    pub doc: Option<Box<Document>>, // Resolved document
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

impl HasSourcePosition for ImportDoc {
    fn source_position(&self) -> &SourcePosition {
        &self.pos
    }
    
    fn set_source_position(&mut self, new_pos: SourcePosition) {
        self.pos = new_pos;
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
            None,
            Vec::new(),
            Expression::string_literal(pos.clone(), "echo hello".to_string()),
            Vec::new(),
            HashMap::new(),
            HashMap::new(),
            HashMap::new(),
        );
        
        assert_eq!(task.name, "my_task");
        assert_eq!(task.effective_wdl_version, "1.0");
        assert!(task.inputs.is_none());
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
            None,
            Vec::new(),
            Vec::new(),
            None,
            HashMap::new(),
            HashMap::new(),
        );
        
        assert_eq!(workflow.name, "my_workflow");
        assert_eq!(workflow.effective_wdl_version, "1.0");
        assert!(workflow.inputs.is_none());
        assert!(workflow.body.is_empty());
        assert!(workflow.outputs.is_none());
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