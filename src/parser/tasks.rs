//! Token-based task and workflow parsing for WDL

use super::token_stream::TokenStream;
use super::tokens::Token;
use super::parser_utils::ParseResult;
use super::declarations::{parse_input_section, parse_output_section};
use super::statements::parse_workflow_element;
use super::expressions::parse_expression;
use crate::tree::{Task, Workflow, WorkflowElement, Declaration};
use crate::expr::{Expression, ExpressionBase};
use crate::error::WdlError;
use std::collections::HashMap;

/// Parse metadata section: meta { key: value, ... }
fn parse_meta_section(stream: &mut TokenStream) -> ParseResult<HashMap<String, serde_json::Value>> {
    // Expect "meta" or "parameter_meta" keyword
    let keyword = match stream.peek_token() {
        Some(Token::Keyword(kw)) if kw == "meta" || kw == "parameter_meta" => {
            let kw = kw.clone();
            stream.next();
            kw
        }
        _ => {
            return Err(WdlError::syntax_error(
                stream.current_position(),
                "Expected 'meta' or 'parameter_meta' keyword".to_string(),
                "1.0".to_string(),
                None,
            ));
        }
    };
    
    stream.expect(Token::LeftBrace)?;
    
    let mut meta = HashMap::new();
    
    // Parse key-value pairs
    while stream.peek_token() != Some(&Token::RightBrace) && !stream.is_eof() {
        // Skip newlines
        while stream.peek_token() == Some(&Token::Newline) {
            stream.next();
        }
        
        if stream.peek_token() == Some(&Token::RightBrace) {
            break;
        }
        
        // Parse key (identifier or keyword)
        let key = match stream.peek_token() {
            Some(Token::Identifier(k)) | Some(Token::Keyword(k)) => {
                let key = k.clone();
                stream.next();
                key
            }
            _ => {
                return Err(WdlError::syntax_error(
                    stream.current_position(),
                    "Expected metadata key".to_string(),
                    "1.0".to_string(),
                    None,
                ));
            }
        };
        
        stream.expect(Token::Colon)?;
        
        // Parse value - for now, parse as expression and convert to JSON
        // In a real implementation, we'd parse JSON-like values directly
        let value_expr = parse_expression(stream)?;
        
        // Convert expression to JSON value (simplified)
        let json_value = expression_to_json(&value_expr);
        meta.insert(key, json_value);
        
        // Optional comma
        if stream.peek_token() == Some(&Token::Comma) {
            stream.next();
        }
        
        // Skip newlines
        while stream.peek_token() == Some(&Token::Newline) {
            stream.next();
        }
    }
    
    stream.expect(Token::RightBrace)?;
    
    Ok(meta)
}

/// Parse runtime section: runtime { key: expression, ... }
fn parse_runtime_section(stream: &mut TokenStream) -> ParseResult<HashMap<String, Expression>> {
    // Expect "runtime" keyword
    match stream.peek_token() {
        Some(Token::Keyword(kw)) if kw == "runtime" => {
            stream.next();
        }
        _ => {
            return Err(WdlError::syntax_error(
                stream.current_position(),
                "Expected 'runtime' keyword".to_string(),
                "1.0".to_string(),
                None,
            ));
        }
    }
    
    stream.expect(Token::LeftBrace)?;
    
    let mut runtime = HashMap::new();
    
    // Parse key-value pairs
    while stream.peek_token() != Some(&Token::RightBrace) && !stream.is_eof() {
        // Skip newlines
        while stream.peek_token() == Some(&Token::Newline) {
            stream.next();
        }
        
        if stream.peek_token() == Some(&Token::RightBrace) {
            break;
        }
        
        // Parse key
        let key = match stream.peek_token() {
            Some(Token::Identifier(k)) | Some(Token::Keyword(k)) => {
                let key = k.clone();
                stream.next();
                key
            }
            _ => {
                return Err(WdlError::syntax_error(
                    stream.current_position(),
                    "Expected runtime key".to_string(),
                    "1.0".to_string(),
                    None,
                ));
            }
        };
        
        stream.expect(Token::Colon)?;
        
        // Parse value expression
        let value = parse_expression(stream)?;
        runtime.insert(key, value);
        
        // Optional comma or newline
        if stream.peek_token() == Some(&Token::Comma) {
            stream.next();
        }
        
        // Skip newlines
        while stream.peek_token() == Some(&Token::Newline) {
            stream.next();
        }
    }
    
    stream.expect(Token::RightBrace)?;
    
    Ok(runtime)
}

/// Parse command section: command { ... } or command <<< ... >>>
fn parse_command_section(stream: &mut TokenStream) -> ParseResult<Expression> {
    let pos = stream.current_position();
    
    // Expect "command" keyword
    match stream.peek_token() {
        Some(Token::Keyword(kw)) if kw == "command" => {
            stream.next();
        }
        _ => {
            return Err(WdlError::syntax_error(
                pos,
                "Expected 'command' keyword".to_string(),
                "1.0".to_string(),
                None,
            ));
        }
    }
    
    // For now, we'll parse the command as a simple string expression
    // In a real implementation, we'd handle heredoc syntax and string interpolation
    
    // Check for { or <<<
    match stream.peek_token() {
        Some(Token::LeftBrace) => {
            stream.next();
            
            // Parse command text until }
            // This is simplified - real implementation would handle whitespace preservation
            let command_text = parse_command_block_content(stream)?;
            
            stream.expect(Token::RightBrace)?;
            
            Ok(Expression::string_literal(pos, command_text))
        }
        _ => {
            // Try to parse heredoc syntax <<<...>>>
            // For now, just parse as a string expression
            parse_expression(stream)
        }
    }
}

/// Parse command block content (simplified)
fn parse_command_block_content(stream: &mut TokenStream) -> ParseResult<String> {
    // This is a simplified implementation
    // Real implementation would preserve all whitespace and handle interpolations
    let mut content = String::new();
    let mut depth = 1;
    
    while !stream.is_eof() && depth > 0 {
        match stream.peek_token() {
            Some(Token::LeftBrace) => {
                content.push('{');
                stream.next();
                depth += 1;
            }
            Some(Token::RightBrace) => {
                depth -= 1;
                if depth > 0 {
                    content.push('}');
                    stream.next();
                } else {
                    break;
                }
            }
            Some(token) => {
                // Add token to content (simplified)
                content.push_str(&format!("{:?} ", token));
                stream.next();
            }
            None => break,
        }
    }
    
    Ok(content)
}

/// Convert expression to JSON value (simplified)
fn expression_to_json(expr: &Expression) -> serde_json::Value {
    // This is a simplified conversion
    // Real implementation would handle all expression types
    if let Some(literal_value) = expr.literal() {
        if let Some(s) = literal_value.as_string() {
            serde_json::Value::String(s.to_string())
        } else if let Some(i) = literal_value.as_int() {
            serde_json::Value::Number(serde_json::Number::from(i))
        } else if let Some(f) = literal_value.as_float() {
            serde_json::Number::from_f64(f)
                .map(serde_json::Value::Number)
                .unwrap_or(serde_json::Value::Null)
        } else if let Some(b) = literal_value.as_bool() {
            serde_json::Value::Bool(b)
        } else {
            serde_json::Value::Null
        }
    } else {
        match expr {
            Expression::String { parts, .. } => {
                // Join string parts
                let s: String = parts.iter()
                    .filter_map(|part| {
                        if let crate::expr::StringPart::Text(text) = part {
                            Some(text.clone())
                        } else {
                            None
                        }
                    })
                    .collect();
                serde_json::Value::String(s)
            }
            _ => serde_json::Value::String(format!("{:?}", expr)),
        }
    }
}

/// Parse a task definition
pub fn parse_task(stream: &mut TokenStream) -> ParseResult<Task> {
    let pos = stream.current_position();
    
    // Expect "task" keyword
    match stream.peek_token() {
        Some(Token::Keyword(kw)) if kw == "task" => {
            stream.next();
        }
        _ => {
            return Err(WdlError::syntax_error(
                pos,
                "Expected 'task' keyword".to_string(),
                "1.0".to_string(),
                None,
            ));
        }
    }
    
    // Parse task name
    let name = match stream.peek_token() {
        Some(Token::Identifier(n)) => {
            let name = n.clone();
            stream.next();
            name
        }
        _ => {
            return Err(WdlError::syntax_error(
                stream.current_position(),
                "Expected task name".to_string(),
                "1.0".to_string(),
                None,
            ));
        }
    };
    
    stream.expect(Token::LeftBrace)?;
    
    // Parse task sections
    let mut inputs: Option<Vec<Declaration>> = None;
    let mut postinputs: Vec<Declaration> = Vec::new();
    let mut command: Option<Expression> = None;
    let mut outputs: Vec<Declaration> = Vec::new();
    let mut runtime: HashMap<String, Expression> = HashMap::new();
    let mut meta: HashMap<String, serde_json::Value> = HashMap::new();
    let mut parameter_meta: HashMap<String, serde_json::Value> = HashMap::new();
    
    // Parse task body
    while stream.peek_token() != Some(&Token::RightBrace) && !stream.is_eof() {
        // Skip newlines
        while stream.peek_token() == Some(&Token::Newline) {
            stream.next();
        }
        
        if stream.peek_token() == Some(&Token::RightBrace) {
            break;
        }
        
        match stream.peek_token() {
            Some(Token::Keyword(kw)) => {
                match kw.as_str() {
                    "input" => {
                        inputs = Some(parse_input_section(stream)?);
                    }
                    "command" => {
                        command = Some(parse_command_section(stream)?);
                    }
                    "output" => {
                        outputs = parse_output_section(stream)?;
                    }
                    "runtime" => {
                        runtime = parse_runtime_section(stream)?;
                    }
                    "meta" => {
                        meta = parse_meta_section(stream)?;
                    }
                    "parameter_meta" => {
                        parameter_meta = parse_meta_section(stream)?;
                    }
                    // Type keywords indicate a declaration
                    "String" | "Int" | "Float" | "Boolean" | "File" | "Directory" |
                    "Array" | "Map" | "Pair" | "Object" => {
                        let decl = super::declarations::parse_declaration(stream, "decl")?;
                        postinputs.push(decl);
                    }
                    _ => {
                        return Err(WdlError::syntax_error(
                            stream.current_position(),
                            format!("Unexpected keyword in task: {}", kw),
                            "1.0".to_string(),
                            None,
                        ));
                    }
                }
            }
            Some(Token::Identifier(_)) => {
                // Could be a struct type declaration
                let decl = super::declarations::parse_declaration(stream, "decl")?;
                postinputs.push(decl);
            }
            _ => {
                return Err(WdlError::syntax_error(
                    stream.current_position(),
                    "Expected task section or declaration".to_string(),
                    "1.0".to_string(),
                    None,
                ));
            }
        }
        
        // Skip newlines
        while stream.peek_token() == Some(&Token::Newline) {
            stream.next();
        }
    }
    
    stream.expect(Token::RightBrace)?;
    
    // Command is required
    let command = command.ok_or_else(|| {
        WdlError::syntax_error(
            pos.clone(),
            "Task must have a command section".to_string(),
            "1.0".to_string(),
            None,
        )
    })?;
    
    Ok(Task::new(
        pos,
        name,
        inputs,
        postinputs,
        command,
        outputs,
        parameter_meta,
        runtime,
        meta,
    ))
}

/// Parse a workflow definition
pub fn parse_workflow(stream: &mut TokenStream) -> ParseResult<Workflow> {
    let pos = stream.current_position();
    
    // Expect "workflow" keyword
    match stream.peek_token() {
        Some(Token::Keyword(kw)) if kw == "workflow" => {
            stream.next();
        }
        _ => {
            return Err(WdlError::syntax_error(
                pos,
                "Expected 'workflow' keyword".to_string(),
                "1.0".to_string(),
                None,
            ));
        }
    }
    
    // Parse workflow name
    let name = match stream.peek_token() {
        Some(Token::Identifier(n)) => {
            let name = n.clone();
            stream.next();
            name
        }
        _ => {
            return Err(WdlError::syntax_error(
                stream.current_position(),
                "Expected workflow name".to_string(),
                "1.0".to_string(),
                None,
            ));
        }
    };
    
    stream.expect(Token::LeftBrace)?;
    
    // Parse workflow sections
    let mut inputs: Option<Vec<Declaration>> = None;
    let mut postinputs: Vec<Declaration> = Vec::new();
    let mut body: Vec<WorkflowElement> = Vec::new();
    let mut outputs: Option<Vec<Declaration>> = None;
    let mut meta: HashMap<String, serde_json::Value> = HashMap::new();
    let mut parameter_meta: HashMap<String, serde_json::Value> = HashMap::new();
    
    // Parse workflow body
    while stream.peek_token() != Some(&Token::RightBrace) && !stream.is_eof() {
        // Skip newlines
        while stream.peek_token() == Some(&Token::Newline) {
            stream.next();
        }
        
        if stream.peek_token() == Some(&Token::RightBrace) {
            break;
        }
        
        match stream.peek_token() {
            Some(Token::Keyword(kw)) => {
                match kw.as_str() {
                    "input" => {
                        inputs = Some(parse_input_section(stream)?);
                    }
                    "output" => {
                        outputs = Some(parse_output_section(stream)?);
                    }
                    "meta" => {
                        meta = parse_meta_section(stream)?;
                    }
                    "parameter_meta" => {
                        parameter_meta = parse_meta_section(stream)?;
                    }
                    _ => {
                        // Try to parse as workflow element
                        let element = parse_workflow_element(stream)?;
                        body.push(element);
                    }
                }
            }
            Some(Token::Identifier(_)) => {
                // Try to parse as workflow element (could be struct type declaration)
                let element = parse_workflow_element(stream)?;
                body.push(element);
            }
            _ => {
                return Err(WdlError::syntax_error(
                    stream.current_position(),
                    "Expected workflow section or element".to_string(),
                    "1.0".to_string(),
                    None,
                ));
            }
        }
        
        // Skip newlines
        while stream.peek_token() == Some(&Token::Newline) {
            stream.next();
        }
    }
    
    stream.expect(Token::RightBrace)?;
    
    Ok(Workflow::new(
        pos,
        name,
        inputs,
        postinputs,
        body,
        outputs,
        parameter_meta,
        meta,
    ))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::parser::token_stream::TokenStream;
    
    #[test]
    fn test_parse_simple_task() {
        let input = r#"task hello {
            command {
                echo "Hello, World!"
            }
            output {
                String message = "done"
            }
        }"#;
        
        let mut stream = TokenStream::new(input, "1.0").unwrap();
        let result = parse_task(&mut stream);
        assert!(result.is_ok());
        
        let task = result.unwrap();
        assert_eq!(task.name, "hello");
        assert!(task.inputs.is_none());
        assert_eq!(task.outputs.len(), 1);
    }
    
    #[test]
    fn test_parse_task_with_inputs() {
        let input = r#"task process {
            input {
                File input_file
                Int threads = 4
            }
            command {
                echo "hello"
            }
            output {
                File result = "output.txt"
            }
        }"#;
        
        let mut stream = TokenStream::new(input, "1.0").unwrap();
        let result = parse_task(&mut stream);
        if let Err(e) = &result {
            eprintln!("Task with inputs parse error: {:?}", e);
        }
        assert!(result.is_ok());
        
        let task = result.unwrap();
        assert_eq!(task.name, "process");
        assert!(task.inputs.is_some());
        assert_eq!(task.inputs.as_ref().unwrap().len(), 2);
    }
    
    #[test]
    fn test_parse_task_with_runtime() {
        let input = r#"task run {
            command {
                echo "running"
            }
            runtime {
                docker: "ubuntu:20.04"
                memory: "4GB"
                cpu: 2
            }
        }"#;
        
        let mut stream = TokenStream::new(input, "1.0").unwrap();
        let result = parse_task(&mut stream);
        assert!(result.is_ok());
        
        let task = result.unwrap();
        assert_eq!(task.runtime.len(), 3);
        assert!(task.runtime.contains_key("docker"));
        assert!(task.runtime.contains_key("memory"));
        assert!(task.runtime.contains_key("cpu"));
    }
    
    #[test]
    fn test_parse_simple_workflow() {
        let input = r#"workflow my_workflow {
            input {
                String name
            }
            call hello { input: name }
            output {
                String result = "done"
            }
        }"#;
        
        let mut stream = TokenStream::new(input, "1.0").unwrap();
        let result = parse_workflow(&mut stream);
        if let Err(e) = &result {
            eprintln!("Workflow parse error: {:?}", e);
        }
        assert!(result.is_ok());
        
        let workflow = result.unwrap();
        assert_eq!(workflow.name, "my_workflow");
        assert!(workflow.inputs.is_some());
        assert_eq!(workflow.body.len(), 1);
        assert!(matches!(workflow.body[0], WorkflowElement::Call(_)));
        assert!(workflow.outputs.is_some());
    }
    
    #[test]
    fn test_parse_workflow_with_scatter() {
        let input = r#"workflow batch_process {
            input {
                Array[File] files
            }
            scatter (file in files) {
                call process { input: file }
            }
            output {
                Array[File] results = process.result
            }
        }"#;
        
        let mut stream = TokenStream::new(input, "1.0").unwrap();
        let result = parse_workflow(&mut stream);
        assert!(result.is_ok());
        
        let workflow = result.unwrap();
        assert_eq!(workflow.name, "batch_process");
        assert_eq!(workflow.body.len(), 1);
        assert!(matches!(workflow.body[0], WorkflowElement::Scatter(_)));
    }
}