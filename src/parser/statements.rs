//! Token-based statement parsing for WDL (control flow, calls, etc.)

use super::token_stream::TokenStream;
use super::tokens::Token;
use super::parser_utils::ParseResult;
use super::expressions::parse_expression;
use super::declarations::parse_declaration;
use crate::tree::{Call, Scatter, Conditional, WorkflowElement};
use crate::error::WdlError;
use std::collections::HashMap;

/// Parse a call statement
/// call task_name [as alias] [{ input_mappings }] [after dependencies]
pub fn parse_call_statement(stream: &mut TokenStream) -> ParseResult<Call> {
    let pos = stream.current_position();
    
    // Expect "call" keyword
    match stream.peek_token() {
        Some(Token::Keyword(kw)) if kw == "call" => {
            stream.next();
        }
        _ => {
            return Err(WdlError::syntax_error(
                pos,
                "Expected 'call' keyword".to_string(),
                "1.0".to_string(),
                None,
            ));
        }
    }
    
    // Parse task name (could be namespaced like lib.task)
    let mut task_name = String::new();
    
    // Parse first identifier
    match stream.peek_token() {
        Some(Token::Identifier(name)) => {
            task_name = name.clone();
            stream.next();
        }
        _ => {
            return Err(WdlError::syntax_error(
                stream.current_position(),
                "Expected task name after 'call'".to_string(),
                "1.0".to_string(),
                None,
            ));
        }
    }
    
    // Check for namespace (lib.task)
    while stream.peek_token() == Some(&Token::Dot) {
        stream.next(); // consume dot
        match stream.peek_token() {
            Some(Token::Identifier(name)) => {
                task_name.push('.');
                task_name.push_str(name);
                stream.next();
            }
            Some(Token::Keyword(kw)) => {
                // Allow keywords after dot in namespaced calls
                task_name.push('.');
                task_name.push_str(kw);
                stream.next();
            }
            _ => {
                return Err(WdlError::syntax_error(
                    stream.current_position(),
                    "Expected identifier after '.'".to_string(),
                    "1.0".to_string(),
                    None,
                ));
            }
        }
    }
    
    // Parse optional alias
    // "as" might be a keyword or identifier depending on WDL version
    let alias = if matches!(stream.peek_token(), Some(Token::Keyword(s)) if s == "as") ||
                   matches!(stream.peek_token(), Some(Token::Identifier(s)) if s == "as") {
        stream.next(); // consume "as"
        match stream.peek_token() {
            Some(Token::Identifier(name)) => {
                let alias = name.clone();
                stream.next();
                Some(alias)
            }
            _ => {
                return Err(WdlError::syntax_error(
                    stream.current_position(),
                    "Expected alias name after 'as'".to_string(),
                    "1.0".to_string(),
                    None,
                ));
            }
        }
    } else {
        None
    };
    
    // Parse optional input mappings
    let inputs = if stream.peek_token() == Some(&Token::LeftBrace) {
        parse_call_inputs(stream)?
    } else {
        HashMap::new()
    };
    
    // Parse optional after clause (WDL 1.0+)
    // "after" is not a reserved keyword, but a contextual identifier
    let afters = if matches!(stream.peek_token(), Some(Token::Identifier(s)) if s == "after") {
        stream.next(); // consume "after"
        
        // Parse list of dependencies
        let mut deps = Vec::new();
        
        // Parse first dependency
        match stream.peek_token() {
            Some(Token::Identifier(name)) => {
                deps.push(name.clone());
                stream.next();
            }
            _ => {
                return Err(WdlError::syntax_error(
                    stream.current_position(),
                    "Expected dependency name after 'after'".to_string(),
                    "1.0".to_string(),
                    None,
                ));
            }
        }
        
        // Parse remaining dependencies
        while stream.peek_token() == Some(&Token::Comma) {
            stream.next(); // consume comma
            match stream.peek_token() {
                Some(Token::Identifier(name)) => {
                    deps.push(name.clone());
                    stream.next();
                }
                _ => break, // Allow trailing comma
            }
        }
        
        deps
    } else {
        Vec::new()
    };
    
    Ok(Call::new(pos, task_name, alias, inputs, afters))
}

/// Parse call input mappings: { input_name: expression, ... }
fn parse_call_inputs(stream: &mut TokenStream) -> ParseResult<HashMap<String, crate::expr::Expression>> {
    stream.expect(Token::LeftBrace)?;
    
    let mut inputs = HashMap::new();
    
    // Check for empty inputs
    if stream.peek_token() == Some(&Token::RightBrace) {
        stream.next();
        return Ok(inputs);
    }
    
    // Parse input mappings
    loop {
        // Parse input name (could be an identifier or keyword used as identifier)
        let name = match stream.peek_token() {
            Some(Token::Identifier(n)) => {
                let name = n.clone();
                stream.next();
                name
            }
            Some(Token::Keyword(kw)) => {
                // Allow keywords to be used as input names in call blocks
                let name = kw.clone();
                stream.next();
                name
            }
            _ => {
                // Check if we've reached the end
                if stream.peek_token() == Some(&Token::RightBrace) {
                    break;
                }
                return Err(WdlError::syntax_error(
                    stream.current_position(),
                    "Expected input name".to_string(),
                    "1.0".to_string(),
                    None,
                ));
            }
        };
        
        // Check for : or = (both are allowed)
        match stream.peek_token() {
            Some(Token::Colon) | Some(Token::Assign) => {
                stream.next(); // consume : or =
            }
            _ => {
                // Shorthand: input_name is both the key and refers to a variable
                let pos = stream.current_position();
                let expr = crate::expr::Expression::ident(pos, name.clone());
                inputs.insert(name, expr);
                
                // Check for comma or end
                if stream.peek_token() == Some(&Token::Comma) {
                    stream.next(); // consume comma
                    continue;
                } else {
                    break;
                }
            }
        }
        
        // Parse expression
        let expr = parse_expression(stream)?;
        inputs.insert(name, expr);
        
        // Check for comma or end
        if stream.peek_token() == Some(&Token::Comma) {
            stream.next(); // consume comma
        } else {
            break;
        }
    }
    
    stream.expect(Token::RightBrace)?;
    Ok(inputs)
}

/// Parse a scatter statement
/// scatter (variable in expression) { body }
pub fn parse_scatter_statement(stream: &mut TokenStream) -> ParseResult<Scatter> {
    let pos = stream.current_position();
    
    // Expect "scatter" keyword
    match stream.peek_token() {
        Some(Token::Keyword(kw)) if kw == "scatter" => {
            stream.next();
        }
        _ => {
            return Err(WdlError::syntax_error(
                pos,
                "Expected 'scatter' keyword".to_string(),
                "1.0".to_string(),
                None,
            ));
        }
    }
    
    // Expect opening paren
    stream.expect(Token::LeftParen)?;
    
    // Parse variable name
    let variable = match stream.peek_token() {
        Some(Token::Identifier(name)) => {
            let var = name.clone();
            stream.next();
            var
        }
        _ => {
            return Err(WdlError::syntax_error(
                stream.current_position(),
                "Expected variable name in scatter".to_string(),
                "1.0".to_string(),
                None,
            ));
        }
    };
    
    // Expect "in" keyword (might be keyword or identifier)
    if matches!(stream.peek_token(), Some(Token::Keyword(s)) if s == "in") ||
       matches!(stream.peek_token(), Some(Token::Identifier(s)) if s == "in") {
        stream.next();
    } else {
            return Err(WdlError::syntax_error(
                stream.current_position(),
                "Expected 'in' keyword in scatter".to_string(),
                "1.0".to_string(),
                None,
            ));
    }
    
    // Parse expression to iterate over
    let expr = parse_expression(stream)?;
    
    // Expect closing paren
    stream.expect(Token::RightParen)?;
    
    // Parse body
    let body = parse_workflow_body(stream)?;
    
    Ok(Scatter::new(pos, variable, expr, body))
}

/// Parse a conditional statement
/// if (expression) { body }
pub fn parse_conditional_statement(stream: &mut TokenStream) -> ParseResult<Conditional> {
    let pos = stream.current_position();
    
    // Expect "if" keyword
    match stream.peek_token() {
        Some(Token::Keyword(kw)) if kw == "if" => {
            stream.next();
        }
        _ => {
            return Err(WdlError::syntax_error(
                pos,
                "Expected 'if' keyword".to_string(),
                "1.0".to_string(),
                None,
            ));
        }
    }
    
    // Expect opening paren
    stream.expect(Token::LeftParen)?;
    
    // Parse condition expression
    let expr = parse_expression(stream)?;
    
    // Expect closing paren
    stream.expect(Token::RightParen)?;
    
    // Parse body
    let body = parse_workflow_body(stream)?;
    
    Ok(Conditional::new(pos, expr, body))
}

/// Parse a workflow body (list of workflow elements)
pub fn parse_workflow_body(stream: &mut TokenStream) -> ParseResult<Vec<WorkflowElement>> {
    stream.expect(Token::LeftBrace)?;
    
    let mut elements = Vec::new();
    
    // Parse elements until closing brace
    while stream.peek_token() != Some(&Token::RightBrace) && !stream.is_eof() {
        // Skip any newlines
        while stream.peek_token() == Some(&Token::Newline) {
            stream.next();
        }
        
        // Check if we've reached the end
        if stream.peek_token() == Some(&Token::RightBrace) || stream.is_eof() {
            break;
        }
        
        // Parse workflow element
        let element = parse_workflow_element(stream)?;
        elements.push(element);
        
        // Skip any newlines
        while stream.peek_token() == Some(&Token::Newline) {
            stream.next();
        }
    }
    
    stream.expect(Token::RightBrace)?;
    
    Ok(elements)
}

/// Parse a single workflow element (declaration, call, scatter, or conditional)
pub fn parse_workflow_element(stream: &mut TokenStream) -> ParseResult<WorkflowElement> {
    // Look at the next token to determine what kind of element this is
    match stream.peek_token() {
        Some(Token::Keyword(kw)) => {
            match kw.as_str() {
                "call" => {
                    let call = parse_call_statement(stream)?;
                    Ok(WorkflowElement::Call(call))
                }
                "scatter" => {
                    let scatter = parse_scatter_statement(stream)?;
                    Ok(WorkflowElement::Scatter(Box::new(scatter)))
                }
                "if" => {
                    let conditional = parse_conditional_statement(stream)?;
                    Ok(WorkflowElement::Conditional(Box::new(conditional)))
                }
                // Type keywords indicate a declaration
                "String" | "Int" | "Float" | "Boolean" | "File" | "Directory" | 
                "Array" | "Map" | "Pair" | "Object" => {
                    let decl = parse_declaration(stream, "decl")?;
                    Ok(WorkflowElement::Declaration(decl))
                }
                _ => {
                    Err(WdlError::syntax_error(
                        stream.current_position(),
                        format!("Unexpected keyword in workflow body: {}", kw),
                        "1.0".to_string(),
                        None,
                    ))
                }
            }
        }
        Some(Token::Identifier(_)) => {
            // Could be a struct type declaration
            let decl = parse_declaration(stream, "decl")?;
            Ok(WorkflowElement::Declaration(decl))
        }
        _ => {
            Err(WdlError::syntax_error(
                stream.current_position(),
                "Expected workflow element (declaration, call, scatter, or if)".to_string(),
                "1.0".to_string(),
                None,
            ))
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::parser::token_stream::TokenStream;
    
    #[test]
    fn test_parse_simple_call() {
        let mut stream = TokenStream::new("call my_task", "1.0").unwrap();
        let result = parse_call_statement(&mut stream);
        assert!(result.is_ok());
        
        let call = result.unwrap();
        assert_eq!(call.task, "my_task");
        assert_eq!(call.alias, None);
        assert!(call.inputs.is_empty());
        assert!(call.afters.is_empty());
    }
    
    #[test]
    fn test_parse_call_with_alias() {
        let mut stream = TokenStream::new("call my_task as task_alias", "1.0").unwrap();
        let result = parse_call_statement(&mut stream);
        assert!(result.is_ok());
        
        let call = result.unwrap();
        assert_eq!(call.task, "my_task");
        assert_eq!(call.alias, Some("task_alias".to_string()));
        assert_eq!(call.name(), "task_alias");
    }
    
    #[test]
    fn test_parse_call_with_inputs() {
        let input = r#"call my_task {
            input1: "value1",
            input2: 42
        }"#;
        
        let mut stream = TokenStream::new(input, "1.0").unwrap();
        let result = parse_call_statement(&mut stream);
        assert!(result.is_ok());
        
        let call = result.unwrap();
        assert_eq!(call.task, "my_task");
        assert_eq!(call.inputs.len(), 2);
        assert!(call.inputs.contains_key("input1"));
        assert!(call.inputs.contains_key("input2"));
    }
    
    #[test]
    fn test_parse_call_with_shorthand_inputs() {
        let input = r#"call my_task {
            input1,
            input2: value2
        }"#;
        
        let mut stream = TokenStream::new(input, "1.0").unwrap();
        let result = parse_call_statement(&mut stream);
        assert!(result.is_ok());
        
        let call = result.unwrap();
        assert_eq!(call.inputs.len(), 2);
        assert!(call.inputs.contains_key("input1"));
        assert!(call.inputs.contains_key("input2"));
    }
    
    #[test]
    fn test_parse_call_with_after() {
        let mut stream = TokenStream::new("call my_task after other_task", "1.0").unwrap();
        let result = parse_call_statement(&mut stream);
        assert!(result.is_ok());
        
        let call = result.unwrap();
        assert_eq!(call.task, "my_task");
        assert_eq!(call.afters, vec!["other_task"]);
    }
    
    #[test]
    fn test_parse_namespaced_call() {
        let mut stream = TokenStream::new("call lib.task", "1.0").unwrap();
        let result = parse_call_statement(&mut stream);
        if let Err(e) = &result {
            eprintln!("Namespaced call error: {:?}", e);
        }
        assert!(result.is_ok());
        
        let call = result.unwrap();
        assert_eq!(call.task, "lib.task");
    }
    
    #[test]
    fn test_parse_scatter() {
        let input = r#"scatter (item in items) {
            call process { input: item }
        }"#;
        
        let mut stream = TokenStream::new(input, "1.0").unwrap();
        let result = parse_scatter_statement(&mut stream);
        if let Err(e) = &result {
            eprintln!("Scatter parse error: {:?}", e);
        }
        assert!(result.is_ok());
        
        let scatter = result.unwrap();
        assert_eq!(scatter.variable, "item");
        assert_eq!(scatter.body.len(), 1);
        assert!(matches!(scatter.body[0], WorkflowElement::Call(_)));
    }
    
    #[test]
    fn test_parse_conditional() {
        let input = r#"if (flag) {
            String message = "enabled"
        }"#;
        
        let mut stream = TokenStream::new(input, "1.0").unwrap();
        let result = parse_conditional_statement(&mut stream);
        assert!(result.is_ok());
        
        let conditional = result.unwrap();
        assert_eq!(conditional.body.len(), 1);
        assert!(matches!(conditional.body[0], WorkflowElement::Declaration(_)));
    }
    
    #[test]
    fn test_parse_nested_control_flow() {
        let input = r#"scatter (x in xs) {
            if (x > 0) {
                call process { input: x }
            }
        }"#;
        
        let mut stream = TokenStream::new(input, "1.0").unwrap();
        let result = parse_scatter_statement(&mut stream);
        assert!(result.is_ok());
        
        let scatter = result.unwrap();
        assert_eq!(scatter.body.len(), 1);
        assert!(matches!(scatter.body[0], WorkflowElement::Conditional(_)));
    }
    
    #[test]
    fn test_parse_workflow_body() {
        let input = r#"{
            String prefix = "test"
            call task1
            scatter (x in [1, 2, 3]) {
                call task2 { input: x }
            }
        }"#;
        
        let mut stream = TokenStream::new(input, "1.0").unwrap();
        let result = parse_workflow_body(&mut stream);
        assert!(result.is_ok());
        
        let body = result.unwrap();
        assert_eq!(body.len(), 3);
        assert!(matches!(body[0], WorkflowElement::Declaration(_)));
        assert!(matches!(body[1], WorkflowElement::Call(_)));
        assert!(matches!(body[2], WorkflowElement::Scatter(_)));
    }
}