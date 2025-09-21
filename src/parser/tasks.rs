//! Token-based task and workflow parsing for WDL

use super::declarations::{parse_input_section, parse_output_section};
use super::expressions::parse_expression;
use super::literals::{parse_placeholder_options, rewrite_adds_for_interpolation};
use super::parser_utils::ParseResult;
use super::statements::parse_workflow_element;
use super::token_stream::TokenStream;
use super::tokens::Token;
use crate::error::WdlError;
use crate::expr::{Expression, ExpressionBase, StringPart};
use crate::tree::{Declaration, Task, Workflow, WorkflowElement};
use std::collections::HashMap;

/// Parse metadata section: meta { key: value, ... }
pub fn parse_meta_section(
    stream: &mut TokenStream,
) -> ParseResult<HashMap<String, serde_json::Value>> {
    // Expect "meta" or "parameter_meta" keyword
    let _keyword = match stream.peek_token() {
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
    while stream.peek_token() != Some(Token::RightBrace) && !stream.is_eof() {
        // Skip newlines
        while stream.peek_token() == Some(Token::Newline) {
            stream.next();
        }

        if stream.peek_token() == Some(Token::RightBrace) {
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
        if stream.peek_token() == Some(Token::Comma) {
            stream.next();
        }

        // Skip newlines
        while stream.peek_token() == Some(Token::Newline) {
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
    while stream.peek_token() != Some(Token::RightBrace) && !stream.is_eof() {
        // Skip newlines
        while stream.peek_token() == Some(Token::Newline) {
            stream.next();
        }

        if stream.peek_token() == Some(Token::RightBrace) {
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
        if stream.peek_token() == Some(Token::Comma) {
            stream.next();
        }

        // Skip newlines
        while stream.peek_token() == Some(Token::Newline) {
            stream.next();
        }
    }

    stream.expect(Token::RightBrace)?;

    Ok(runtime)
}

/// Parse requirements section: requirements { key: expression, ... }
/// Validates that only standard WDL requirements attributes are used
fn parse_requirements_section(
    stream: &mut TokenStream,
) -> ParseResult<HashMap<String, Expression>> {
    // WDL 1.2 standard requirements attributes
    const STANDARD_REQUIREMENTS: &[&str] = &[
        "container",
        "docker",      // container (docker is alias)
        "cpu",         // CPU cores
        "memory",      // Memory (RAM)
        "gpu",         // GPU requirement
        "gpuType",     // GPU type
        "gpuCount",    // Number of GPUs
        "fpga",        // FPGA requirement
        "disks",       // Disk space
        "maxRetries",  // Maximum retry attempts
        "returnCodes", // Valid return codes
    ];

    // Expect "requirements" keyword
    match stream.peek_token() {
        Some(Token::Keyword(kw)) if kw == "requirements" => {
            stream.next();
        }
        _ => {
            return Err(WdlError::syntax_error(
                stream.current_position(),
                "Expected 'requirements' keyword".to_string(),
                "1.2".to_string(),
                None,
            ));
        }
    }

    stream.expect(Token::LeftBrace)?;

    let mut requirements = HashMap::new();

    // Parse key-value pairs
    while stream.peek_token() != Some(Token::RightBrace) && !stream.is_eof() {
        // Skip newlines
        while stream.peek_token() == Some(Token::Newline) {
            stream.next();
        }

        if stream.peek_token() == Some(Token::RightBrace) {
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
                    "Expected requirements attribute name".to_string(),
                    "1.2".to_string(),
                    None,
                ));
            }
        };

        // Validate that the key is a standard requirements attribute
        if !STANDARD_REQUIREMENTS.contains(&key.as_str()) {
            return Err(WdlError::syntax_error(
                stream.current_position(),
                format!("Unknown requirements attribute '{}'. Use hints section for arbitrary attributes.", key),
                "1.2".to_string(),
                None,
            ));
        }

        stream.expect(Token::Colon)?;

        // Parse value expression
        let value = parse_expression(stream)?;
        requirements.insert(key, value);

        // Optional comma or newline
        if stream.peek_token() == Some(Token::Comma) {
            stream.next();
        }

        // Skip newlines
        while stream.peek_token() == Some(Token::Newline) {
            stream.next();
        }
    }

    stream.expect(Token::RightBrace)?;

    Ok(requirements)
}

/// Parse hints section: hints { key: expression, ... }
/// Allows arbitrary key-value pairs for execution engine optimization hints
fn parse_hints_section(stream: &mut TokenStream) -> ParseResult<HashMap<String, Expression>> {
    // Expect "hints" keyword
    match stream.peek_token() {
        Some(Token::Keyword(kw)) if kw == "hints" => {
            stream.next();
        }
        _ => {
            return Err(WdlError::syntax_error(
                stream.current_position(),
                "Expected 'hints' keyword".to_string(),
                "1.2".to_string(),
                None,
            ));
        }
    }

    stream.expect(Token::LeftBrace)?;

    let mut hints = HashMap::new();

    // Parse key-value pairs
    while stream.peek_token() != Some(Token::RightBrace) && !stream.is_eof() {
        // Skip newlines
        while stream.peek_token() == Some(Token::Newline) {
            stream.next();
        }

        if stream.peek_token() == Some(Token::RightBrace) {
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
                    "Expected hints attribute name".to_string(),
                    "1.2".to_string(),
                    None,
                ));
            }
        };

        stream.expect(Token::Colon)?;

        // Parse value expression
        let value = parse_expression(stream)?;
        hints.insert(key, value);

        // Optional comma or newline
        if stream.peek_token() == Some(Token::Comma) {
            stream.next();
        }

        // Skip newlines
        while stream.peek_token() == Some(Token::Newline) {
            stream.next();
        }
    }

    stream.expect(Token::RightBrace)?;

    Ok(hints)
}

/// Parse command section: command { ... } or command <<< ... >>>
/// This method handles both regular commands and preprocessed placeholders
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

    // Check for regular command blocks
    match stream.peek_token() {
        Some(Token::LeftBrace) => {
            stream.next();

            // Use special command-mode parsing to build StringParts directly
            let parts = parse_command_block_with_parts(stream)?;

            // Exit command mode before expecting closing brace
            stream.exit_command_mode();

            stream.expect(Token::RightBrace)?;

            Ok(Expression::task_command(pos, parts))
        }
        Some(Token::HeredocStart) => {
            stream.next();

            // Use special command-mode parsing for heredoc to build StringParts directly
            let parts = parse_heredoc_with_parts(stream)?;

            // Exit command mode before expecting closing heredoc marker
            stream.exit_command_mode();

            stream.expect(Token::HeredocEnd)?;

            Ok(Expression::task_command(pos, parts))
        }
        _ => Err(WdlError::syntax_error(
            stream.current_position(),
            "Expected '{' or '<<<' after 'command' keyword".to_string(),
            "1.0".to_string(),
            None,
        )),
    }
}

/// Parse command block content into StringParts using command-mode tokenization
fn parse_command_block_with_parts(stream: &mut TokenStream) -> ParseResult<Vec<StringPart>> {
    // Enter command mode for proper tokenization
    stream.enter_command_mode();

    let mut parts = Vec::new();
    let mut current_text = String::new();
    let mut depth = 1;

    while !stream.is_eof() && depth > 0 {
        match stream.peek_token() {
            Some(Token::LeftBrace) => {
                current_text.push('{');
                stream.next();
                depth += 1;
            }
            Some(Token::RightBrace) => {
                depth -= 1;
                if depth > 0 {
                    current_text.push('}');
                    stream.next();
                } else {
                    // Don't consume the closing brace - let caller handle it
                    break;
                }
            }
            Some(Token::CommandText(text)) => {
                current_text.push_str(&text);
                stream.next();
            }
            Some(Token::TildeBrace) => {
                // Save any accumulated text as a StringPart::Text
                if !current_text.is_empty() {
                    parts.push(StringPart::Text(current_text.clone()));
                    current_text.clear();
                }

                // Parse the placeholder expression directly
                stream.next(); // consume ~{ or ${
                stream.push_lexer_mode(crate::parser::lexer::LexerMode::Normal);

                // Parse placeholder options first
                let options = parse_placeholder_options(stream)?;

                // Then parse the expression
                let mut expr = parse_expression(stream)?;

                // Apply interpolation-specific rewriting for optional concatenation handling
                rewrite_adds_for_interpolation(&mut expr);

                // Expect placeholder end (either PlaceholderEnd or RightBrace in Normal mode)
                match stream.peek_token() {
                    Some(Token::PlaceholderEnd) | Some(Token::RightBrace) => {
                        stream.next(); // consume closing }
                    }
                    _ => {
                        return Err(WdlError::syntax_error(
                            stream.current_position(),
                            "Expected '}' to close placeholder".to_string(),
                            "1.0".to_string(),
                            None,
                        ));
                    }
                }
                stream.pop_lexer_mode(); // Return to command mode

                parts.push(StringPart::Placeholder {
                    expr: Box::new(expr),
                    options,
                });
            }
            Some(token) => {
                // Handle other tokens (whitespace, newlines, etc.)
                match token {
                    Token::Whitespace(s) => current_text.push_str(&s),
                    Token::Newline => current_text.push('\n'),
                    Token::Identifier(s) | Token::Keyword(s) => current_text.push_str(&s),
                    Token::IntLiteral(n) => current_text.push_str(&n.to_string()),
                    Token::FloatLiteral(f) => current_text.push_str(&f.to_string()),
                    _ => current_text.push_str(&format!("{}", token)),
                }
                stream.next();
            }
            None => break,
        }
    }

    // Add any remaining text
    if !current_text.is_empty() {
        parts.push(StringPart::Text(current_text));
    }

    // If no parts were created, add an empty text part
    if parts.is_empty() {
        parts.push(StringPart::Text(String::new()));
    }

    Ok(parts)
}

/// Parse heredoc content into StringParts using command-mode tokenization
fn parse_heredoc_with_parts(stream: &mut TokenStream) -> ParseResult<Vec<StringPart>> {
    // Enter command mode for proper tokenization
    stream.enter_command_mode();

    let mut parts = Vec::new();
    let mut current_text = String::new();

    while !stream.is_eof() {
        match stream.peek_token() {
            Some(Token::HeredocEnd) => {
                // Don't consume the closing >>> - let caller handle it
                break;
            }
            Some(Token::CommandText(text)) => {
                current_text.push_str(&text);
                stream.next();
            }
            Some(Token::TildeBrace) => {
                // Save any accumulated text as a StringPart::Text
                if !current_text.is_empty() {
                    parts.push(StringPart::Text(current_text.clone()));
                    current_text.clear();
                }

                // Parse the placeholder expression directly
                stream.next(); // consume ~{ or ${
                stream.push_lexer_mode(crate::parser::lexer::LexerMode::Normal);

                // Parse placeholder options first
                let options = parse_placeholder_options(stream)?;

                // Then parse the expression
                let mut expr = parse_expression(stream)?;

                // Apply interpolation-specific rewriting for optional concatenation handling
                rewrite_adds_for_interpolation(&mut expr);

                // Expect placeholder end (either PlaceholderEnd or RightBrace in Normal mode)
                match stream.peek_token() {
                    Some(Token::PlaceholderEnd) | Some(Token::RightBrace) => {
                        stream.next(); // consume closing }
                    }
                    _ => {
                        return Err(WdlError::syntax_error(
                            stream.current_position(),
                            "Expected '}' to close placeholder".to_string(),
                            "1.0".to_string(),
                            None,
                        ));
                    }
                }
                stream.pop_lexer_mode(); // Return to command mode

                parts.push(StringPart::Placeholder {
                    expr: Box::new(expr),
                    options,
                });
            }
            Some(token) => {
                // Handle other tokens (whitespace, newlines, etc.)
                match token {
                    Token::Whitespace(s) => current_text.push_str(&s),
                    Token::Newline => current_text.push('\n'),
                    Token::Identifier(s) | Token::Keyword(s) => current_text.push_str(&s),
                    Token::IntLiteral(n) => current_text.push_str(&n.to_string()),
                    Token::FloatLiteral(f) => current_text.push_str(&f.to_string()),
                    _ => current_text.push_str(&format!("{}", token)),
                }
                stream.next();
            }
            None => {
                return Err(WdlError::syntax_error(
                    stream.current_position(),
                    "Unexpected end of file in heredoc command".to_string(),
                    "1.0".to_string(),
                    None,
                ));
            }
        }
    }

    // Add any remaining text
    if !current_text.is_empty() {
        parts.push(StringPart::Text(current_text));
    }

    // If no parts were created, add an empty text part
    if parts.is_empty() {
        parts.push(StringPart::Text(String::new()));
    }

    Ok(parts)
}

/// Parse heredoc content until >>>
#[allow(dead_code)]
fn parse_heredoc_content(stream: &mut TokenStream) -> ParseResult<String> {
    let mut content = String::new();
    let mut buffer = String::new();

    while !stream.is_eof() {
        match stream.peek_token() {
            Some(Token::HeredocEnd) => {
                // Found the end marker
                break;
            }
            Some(Token::TildeBrace) => {
                // Start of placeholder - for now just record it
                content.push_str(&buffer);
                buffer.clear();

                let placeholder_type = stream.peek_token().unwrap().clone();
                stream.next();

                // Parse placeholder content until } (simplified handling)
                let mut placeholder_text = String::new();
                let mut depth = 1;
                while !stream.is_eof() && depth > 0 {
                    match stream.peek_token() {
                        Some(Token::LeftBrace) | Some(Token::TildeBrace) => {
                            depth += 1;
                            placeholder_text.push_str(&format!("{}", stream.peek_token().unwrap()));
                            stream.next();
                        }
                        Some(Token::RightBrace) | Some(Token::PlaceholderEnd) => {
                            depth -= 1;
                            if depth > 0 {
                                placeholder_text.push('}');
                            }
                            stream.next();
                        }
                        Some(token) => {
                            placeholder_text.push_str(&format!("{}", token));
                            stream.next();
                        }
                        None => break,
                    }
                }

                // For now, just include placeholder as literal text
                if placeholder_type == Token::TildeBrace {
                    content.push_str(&format!("~{{{}}}", placeholder_text));
                }
            }
            Some(token) => {
                // Accumulate other tokens as text
                // In a real parser, we'd preserve the exact text representation
                match token {
                    Token::Whitespace(s) => buffer.push_str(&s),
                    Token::Newline => buffer.push('\n'),
                    Token::Identifier(s) | Token::Keyword(s) => buffer.push_str(&s),
                    Token::IntLiteral(n) => buffer.push_str(&n.to_string()),
                    Token::FloatLiteral(f) => buffer.push_str(&f.to_string()),
                    Token::StringLiteral(s) => buffer.push_str(&s),
                    _ => buffer.push_str(&format!("{}", token)),
                }
                stream.next();
            }
            None => {
                return Err(WdlError::syntax_error(
                    stream.current_position(),
                    "Unexpected end of file in heredoc command".to_string(),
                    "1.0".to_string(),
                    None,
                ));
            }
        }
    }

    content.push_str(&buffer);
    Ok(content)
}

/// Parse command block content (simplified)
#[allow(dead_code)]
fn parse_command_block_content(stream: &mut TokenStream) -> ParseResult<String> {
    let mut content = String::new();
    let mut depth = 1;

    while !stream.is_eof() && depth > 0 {
        match stream.peek_token() {
            Some(Token::LeftBrace) => {
                depth += 1;
                content.push('{');
                stream.next();
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
            Some(Token::TildeBrace) => {
                // Start of placeholder
                let placeholder_type = stream.peek_token().unwrap().clone();
                stream.next();

                // Parse placeholder content until } (simplified handling)
                let mut placeholder_text = String::new();
                let mut depth = 1;
                while !stream.is_eof() && depth > 0 {
                    match stream.peek_token() {
                        Some(Token::LeftBrace) | Some(Token::TildeBrace) => {
                            depth += 1;
                            placeholder_text.push_str(&format!("{}", stream.peek_token().unwrap()));
                            stream.next();
                        }
                        Some(Token::RightBrace) | Some(Token::PlaceholderEnd) => {
                            depth -= 1;
                            if depth > 0 {
                                placeholder_text.push('}');
                            }
                            stream.next();
                        }
                        Some(token) => {
                            placeholder_text.push_str(&format!("{}", token));
                            stream.next();
                        }
                        None => break,
                    }
                }

                // For now, just include placeholder as literal text
                if placeholder_type == Token::TildeBrace {
                    content.push_str(&format!("~{{{}}}", placeholder_text));
                }
            }
            Some(token) => {
                // Add token to content
                match token {
                    Token::Whitespace(s) => content.push_str(&s),
                    Token::Newline => content.push('\n'),
                    Token::Identifier(s) | Token::Keyword(s) => content.push_str(&s),
                    Token::IntLiteral(n) => content.push_str(&n.to_string()),
                    Token::FloatLiteral(f) => content.push_str(&f.to_string()),
                    Token::StringLiteral(s) => content.push_str(&s),
                    _ => content.push_str(&format!("{}", token)),
                }
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
                let s: String = parts
                    .iter()
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
    let mut inputs: Vec<Declaration> = Vec::new();
    let mut postinputs: Vec<Declaration> = Vec::new();
    let mut command: Option<Expression> = None;
    let mut outputs: Vec<Declaration> = Vec::new();
    let mut runtime: HashMap<String, Expression> = HashMap::new();
    let mut requirements: HashMap<String, Expression> = HashMap::new();
    let mut hints: HashMap<String, Expression> = HashMap::new();
    let mut meta: HashMap<String, serde_json::Value> = HashMap::new();
    let mut parameter_meta: HashMap<String, serde_json::Value> = HashMap::new();

    // Track which sections are present to ensure mutual exclusivity
    let mut has_runtime = false;
    let mut has_requirements = false;
    let mut has_hints = false;

    // Parse task body
    while stream.peek_token() != Some(Token::RightBrace) && !stream.is_eof() {
        // Skip newlines
        while stream.peek_token() == Some(Token::Newline) {
            stream.next();
        }

        if stream.peek_token() == Some(Token::RightBrace) {
            break;
        }

        match stream.peek_token() {
            Some(Token::Keyword(kw)) => {
                match kw.as_str() {
                    "input" => {
                        inputs = parse_input_section(stream)?;
                    }
                    "command" => {
                        command = Some(parse_command_section(stream)?);
                    }
                    "output" => {
                        outputs = parse_output_section(stream)?;
                    }
                    "runtime" => {
                        if has_requirements || has_hints {
                            return Err(WdlError::syntax_error(
                                stream.current_position(),
                                "runtime section cannot be used with requirements or hints sections".to_string(),
                                "1.2".to_string(),
                                None,
                            ));
                        }
                        has_runtime = true;
                        runtime = parse_runtime_section(stream)?;
                    }
                    "requirements" => {
                        if has_runtime {
                            return Err(WdlError::syntax_error(
                                stream.current_position(),
                                "requirements section cannot be used with runtime section"
                                    .to_string(),
                                "1.2".to_string(),
                                None,
                            ));
                        }
                        has_requirements = true;
                        requirements = parse_requirements_section(stream)?;
                    }
                    "hints" => {
                        if has_runtime {
                            return Err(WdlError::syntax_error(
                                stream.current_position(),
                                "hints section cannot be used with runtime section".to_string(),
                                "1.2".to_string(),
                                None,
                            ));
                        }
                        has_hints = true;
                        hints = parse_hints_section(stream)?;
                    }
                    "meta" => {
                        meta = parse_meta_section(stream)?;
                    }
                    "parameter_meta" => {
                        parameter_meta = parse_meta_section(stream)?;
                    }
                    // Type keywords indicate a declaration
                    "String" | "Int" | "Float" | "Boolean" | "File" | "Directory" | "Array"
                    | "Map" | "Pair" | "Object" => {
                        let decl = super::declarations::parse_declaration(stream, "decl")?;
                        postinputs.push(decl);
                    }
                    _ => {
                        let pos = stream.current_position();
                        return Err(WdlError::syntax_error(
                            pos,
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
        while stream.peek_token() == Some(Token::Newline) {
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

    Ok(Task::new_with_requirements_hints(
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
    let mut inputs: Vec<Declaration> = Vec::new();
    let postinputs: Vec<Declaration> = Vec::new();
    let mut body: Vec<WorkflowElement> = Vec::new();
    let mut outputs: Vec<Declaration> = Vec::new();
    let mut meta: HashMap<String, serde_json::Value> = HashMap::new();
    let mut parameter_meta: HashMap<String, serde_json::Value> = HashMap::new();

    // Parse workflow body
    while stream.peek_token() != Some(Token::RightBrace) && !stream.is_eof() {
        // Skip newlines
        while stream.peek_token() == Some(Token::Newline) {
            stream.next();
        }

        if stream.peek_token() == Some(Token::RightBrace) {
            break;
        }

        match stream.peek_token() {
            Some(Token::Keyword(kw)) => {
                match kw.as_str() {
                    "input" => {
                        inputs = parse_input_section(stream)?;
                    }
                    "output" => {
                        outputs = parse_output_section(stream)?;
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
        while stream.peek_token() == Some(Token::Newline) {
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
    use crate::parser::lexer::Lexer;
    use crate::parser::token_stream::TokenStream;

    #[test]
    fn test_parse_requirements_section() {
        let input = r#"requirements {
            container: "ubuntu:latest"
            cpu: 2
            memory: "4 GiB"
        }"#;

        let mut stream = TokenStream::new(input, "1.2").unwrap();

        let requirements = parse_requirements_section(&mut stream).unwrap();

        assert_eq!(requirements.len(), 3);
        assert!(requirements.contains_key("container"));
        assert!(requirements.contains_key("cpu"));
        assert!(requirements.contains_key("memory"));
    }

    #[test]
    fn test_parse_hints_section() {
        let input = r#"hints {
            localization_optional: false
            maxCpu: 4
            preemptible: 1
            custom_hint: "value"
        }"#;

        let mut stream = TokenStream::new(input, "1.2").unwrap();

        let hints = parse_hints_section(&mut stream).unwrap();

        assert_eq!(hints.len(), 4);
        assert!(hints.contains_key("localization_optional"));
        assert!(hints.contains_key("maxCpu"));
        assert!(hints.contains_key("preemptible"));
        assert!(hints.contains_key("custom_hint"));
    }

    #[test]
    fn test_requirements_invalid_attribute() {
        let input = r#"requirements {
            invalid_attribute: "should fail"
        }"#;

        let mut stream = TokenStream::new(input, "1.2").unwrap();

        let result = parse_requirements_section(&mut stream);
        assert!(result.is_err());
        assert!(result
            .unwrap_err()
            .to_string()
            .contains("Unknown requirements attribute"));
    }

    #[test]
    fn test_task_with_requirements_and_hints() {
        let input = r#"task example {
            input {
                String message = "hello"
            }
            
            command {
                echo "~{message}"
            }
            
            output {
                String result = stdout()
            }
            
            requirements {
                container: "ubuntu:latest"
                cpu: 2
                memory: "4 GiB"
            }
            
            hints {
                maxCpu: 4
                preemptible: 1
            }
        }"#;

        let mut stream = TokenStream::new(input, "1.2").unwrap();

        let task = parse_task(&mut stream).unwrap();

        assert_eq!(task.name, "example");
        assert_eq!(task.requirements.len(), 3);
        assert_eq!(task.hints.len(), 2);
        assert!(task.runtime.is_empty());

        assert!(task.requirements.contains_key("container"));
        assert!(task.requirements.contains_key("cpu"));
        assert!(task.requirements.contains_key("memory"));

        assert!(task.hints.contains_key("maxCpu"));
        assert!(task.hints.contains_key("preemptible"));
    }

    #[test]
    fn test_task_runtime_requirements_mutual_exclusion() {
        let input = r#"task example {
            command {
                echo "hello"
            }
            
            runtime {
                docker: "ubuntu:latest"
            }
            
            requirements {
                container: "ubuntu:latest"
            }
        }"#;

        let mut stream = TokenStream::new(input, "1.2").unwrap();

        let result = parse_task(&mut stream);
        assert!(result.is_err());
        assert!(result
            .unwrap_err()
            .to_string()
            .contains("cannot be used with runtime"));
    }

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
        assert!(task.inputs.is_empty());
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
        assert!(!task.inputs.is_empty());
        assert_eq!(task.inputs.len(), 2);
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
        assert!(!workflow.inputs.is_empty());
        assert_eq!(workflow.body.len(), 1);
        assert!(matches!(workflow.body[0], WorkflowElement::Call(_)));
        assert!(!workflow.outputs.is_empty());
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

    #[test]
    fn test_command_block_preserves_spaces() {
        let input = r#"task test_spaces {
            command {
                echo "Hello World"
                grep "pattern" file.txt
                cat file1 file2
            }
        }"#;

        let mut stream = TokenStream::new(input, "1.0").unwrap();
        let result = parse_task(&mut stream);
        if let Err(e) = &result {
            eprintln!("Command block spacing test error: {:?}", e);
        }
        assert!(result.is_ok());

        let task = result.unwrap();
        let command_text = match &task.command {
            Expression::String { parts, .. } => parts
                .iter()
                .filter_map(|part| match part {
                    crate::expr::StringPart::Text(text) => Some(text.as_str()),
                    _ => None,
                })
                .collect::<Vec<_>>()
                .join(""),
            _ => panic!("Command should be a string expression"),
        };
        let command = &command_text;

        // The command should preserve spaces between tokens
        assert!(
            command.contains("echo \"Hello World\""),
            "Command should contain 'echo \"Hello World\"' with proper spacing, but got: {}",
            command
        );
        assert!(
            command.contains("grep \"pattern\" file.txt"),
            "Command should contain 'grep \"pattern\" file.txt' with proper spacing, but got: {}",
            command
        );
        assert!(
            command.contains("cat file1 file2"),
            "Command should contain 'cat file1 file2' with proper spacing, but got: {}",
            command
        );

        // Should NOT contain concatenated tokens without spaces
        assert!(
            !command.contains("echo\"Hello"),
            "Command should not contain 'echo\"Hello' (missing space), but got: {}",
            command
        );
        assert!(
            !command.contains("grep\"pattern\""),
            "Command should not contain 'grep\"pattern\"' (missing spaces), but got: {}",
            command
        );
    }

    #[test]
    fn test_heredoc_command_preserves_spaces() {
        let input = r#"task test_heredoc {
            command <<<
                echo "Hello World"
                grep "pattern" file.txt
                cat file1 file2
            >>>
        }"#;

        let mut stream = TokenStream::new(input, "1.0").unwrap();
        let result = parse_task(&mut stream);
        if let Err(e) = &result {
            eprintln!("Heredoc spacing test error: {:?}", e);
        }
        assert!(result.is_ok());

        let task = result.unwrap();
        let command_text = match &task.command {
            Expression::String { parts, .. } => parts
                .iter()
                .filter_map(|part| match part {
                    crate::expr::StringPart::Text(text) => Some(text.as_str()),
                    _ => None,
                })
                .collect::<Vec<_>>()
                .join(""),
            _ => panic!("Command should be a string expression"),
        };
        let command = &command_text;

        // The command should preserve spaces between tokens
        assert!(
            command.contains("echo \"Hello World\""),
            "Heredoc should contain 'echo \"Hello World\"' with proper spacing, but got: {}",
            command
        );
        assert!(
            command.contains("grep \"pattern\" file.txt"),
            "Heredoc should contain 'grep \"pattern\" file.txt' with proper spacing, but got: {}",
            command
        );
        assert!(
            command.contains("cat file1 file2"),
            "Heredoc should contain 'cat file1 file2' with proper spacing, but got: {}",
            command
        );

        // Should NOT contain concatenated tokens without spaces
        assert!(
            !command.contains("echo\"Hello"),
            "Heredoc should not contain 'echo\"Hello' (missing space), but got: {}",
            command
        );
        assert!(
            !command.contains("grep\"pattern\""),
            "Heredoc should not contain 'grep\"pattern\"' (missing spaces), but got: {}",
            command
        );
    }

    #[test]
    fn test_command_with_variable_interpolation() {
        let input = r#"task test_vars {
            input {
                String name = "World"
            }
            command {
                echo "Hello, ~{name}!"
                mkdir ~{name}_dir
            }
        }"#;

        let mut stream = TokenStream::new(input, "1.0").unwrap();
        let result = parse_task(&mut stream);
        if let Err(e) = &result {
            eprintln!("Variable interpolation test error: {:?}", e);
        }
        assert!(result.is_ok());

        let task = result.unwrap();

        // Check that the command expression contains placeholders (this is a parsing test, not an evaluation test)
        match &task.command {
            Expression::String { parts, .. } => {
                // Should have multiple parts with placeholders
                assert!(
                    parts.len() > 1,
                    "Expected multiple parts with placeholders, got: {:?}",
                    parts
                );

                // Look for the name placeholders
                let mut found_name_placeholders = 0;

                for part in parts {
                    if let StringPart::Placeholder { expr, .. } = part {
                        if let Expression::Ident { name, .. } = expr.as_ref() {
                            if name == "name" {
                                found_name_placeholders += 1;
                            }
                        }
                    }
                }

                assert_eq!(
                    found_name_placeholders, 2,
                    "Should find 2 'name' placeholders in command: {:?}",
                    parts
                );
            }
            _ => panic!("Command should be a String expression with parts"),
        }
    }

    #[test]
    fn test_complex_command_with_pipes_and_redirection() {
        let input = r#"task test_complex {
            command {
                cat input.txt | grep "pattern" | sort > output.txt
                ls -la /path/to/files
            }
        }"#;

        let mut stream = TokenStream::new(input, "1.0").unwrap();
        let result = parse_task(&mut stream);
        if let Err(e) = &result {
            eprintln!("Complex command test error: {:?}", e);
        }
        assert!(result.is_ok());

        let task = result.unwrap();
        let command_text = match &task.command {
            Expression::String { parts, .. } => parts
                .iter()
                .filter_map(|part| match part {
                    crate::expr::StringPart::Text(text) => Some(text.as_str()),
                    _ => None,
                })
                .collect::<Vec<_>>()
                .join(""),
            _ => panic!("Command should be a string expression"),
        };
        let command = &command_text;

        // Should preserve all shell operators and spaces
        assert!(
            command.contains("cat input.txt"),
            "Command should contain 'cat input.txt' with space, but got: {}",
            command
        );
        assert!(
            command.contains("grep \"pattern\""),
            "Command should contain 'grep \"pattern\"' with spaces, but got: {}",
            command
        );
        assert!(
            command.contains("ls -la /path/to/files"),
            "Command should contain 'ls -la /path/to/files' with spaces, but got: {}",
            command
        );

        // Check for shell operators with proper spacing
        assert!(
            command.contains(" | "),
            "Command should contain pipe operator with spaces, but got: {}",
            command
        );
        assert!(
            command.contains(" > "),
            "Command should contain redirection operator with spaces, but got: {}",
            command
        );
    }

    #[test]
    fn test_variable_substitution_simple() {
        let input = r#"task test_var_sub {
            input {
                String name = "World"
            }
            command {
                echo "Hello, ~{name}!"
            }
        }"#;

        let mut stream = TokenStream::new(input, "1.0").unwrap();
        let result = parse_task(&mut stream);
        if let Err(e) = &result {
            eprintln!("Variable substitution test error: {:?}", e);
        }
        assert!(result.is_ok());

        let task = result.unwrap();

        // Check that the command expression contains placeholders
        match &task.command {
            Expression::String { parts, .. } => {
                assert_eq!(
                    parts.len(),
                    3,
                    "Expected 3 parts: text + placeholder + text, got: {:?}",
                    parts
                );

                // First part should contain "Hello, " (may have leading whitespace)
                if let StringPart::Text(text) = &parts[0] {
                    assert!(
                        text.contains("echo \"Hello, "),
                        "First part should contain 'echo \"Hello, ', got: '{}'",
                        text
                    );
                } else {
                    panic!("First part should be Text, got: {:?}", parts[0]);
                }

                // Second part should be a placeholder for 'name'
                if let StringPart::Placeholder { expr, .. } = &parts[1] {
                    if let Expression::Ident { name, .. } = expr.as_ref() {
                        assert_eq!(
                            name, "name",
                            "Placeholder should be for 'name', got: '{}'",
                            name
                        );
                    } else {
                        panic!("Placeholder expression should be Ident, got: {:?}", expr);
                    }
                } else {
                    panic!("Second part should be Placeholder, got: {:?}", parts[1]);
                }

                // Third part should contain "!" (may have trailing whitespace)
                if let StringPart::Text(text) = &parts[2] {
                    assert!(
                        text.starts_with("!"),
                        "Third part should start with '!', got: '{}'",
                        text
                    );
                } else {
                    panic!("Third part should be Text, got: {:?}", parts[2]);
                }
            }
            _ => panic!("Command should be a String expression with parts"),
        }
    }

    #[test]
    fn test_variable_substitution_heredoc() {
        let input = r#"task test_heredoc_var {
            input {
                String person = "Alice"
                String greeting = "Hello"
            }
            command <<<
                echo "~{greeting}, ~{person}!"
                echo "Welcome to WDL"
            >>>
        }"#;

        let mut stream = TokenStream::new(input, "1.0").unwrap();
        let result = parse_task(&mut stream);
        assert!(result.is_ok());

        let task = result.unwrap();

        // Check that the command expression contains placeholders
        match &task.command {
            Expression::String { parts, .. } => {
                // Should have multiple parts for the placeholders
                assert!(
                    parts.len() > 1,
                    "Expected multiple parts with placeholders, got: {:?}",
                    parts
                );

                // Look for the greeting and person placeholders
                let mut found_greeting = false;
                let mut found_person = false;

                for part in parts {
                    if let StringPart::Placeholder { expr, .. } = part {
                        if let Expression::Ident { name, .. } = expr.as_ref() {
                            if name == "greeting" {
                                found_greeting = true;
                            } else if name == "person" {
                                found_person = true;
                            }
                        }
                    }
                }

                assert!(
                    found_greeting,
                    "Should find 'greeting' placeholder in command: {:?}",
                    parts
                );
                assert!(
                    found_person,
                    "Should find 'person' placeholder in command: {:?}",
                    parts
                );
            }
            _ => panic!("Command should be a String expression with parts"),
        }
    }

    #[test]
    fn test_variable_substitution_mixed_placeholders() {
        let input = r#"task test_mixed_placeholders {
            input {
                String filename = "data.txt"
                String separator = ","
            }
            command {
                cat ~{filename} | cut -d'~{separator}' -f1
            }
        }"#;

        let mut stream = TokenStream::new(input, "1.0").unwrap();
        let result = parse_task(&mut stream);
        assert!(result.is_ok());

        let task = result.unwrap();

        match &task.command {
            Expression::String { parts, .. } => {
                let mut placeholder_count = 0;
                let mut found_filename = false;
                let mut found_separator = false;

                for part in parts {
                    match part {
                        StringPart::Placeholder { expr, .. } => {
                            placeholder_count += 1;
                            if let Expression::Ident { name, .. } = expr.as_ref() {
                                if name == "filename" {
                                    found_filename = true;
                                } else if name == "separator" {
                                    found_separator = true;
                                }
                            }
                        }
                        StringPart::Text(_) => {} // Expected
                    }
                }

                assert!(found_filename, "Should find 'filename' placeholder");
                assert!(found_separator, "Should find 'separator' placeholder");
                assert_eq!(placeholder_count, 2, "Should have exactly 2 placeholders");
            }
            _ => panic!("Command should be a String expression with parts"),
        }
    }

    #[test]
    fn test_no_variable_substitution() {
        let input = r#"task test_no_vars {
            command {
                echo "Hello, World!"
                ls -la
            }
        }"#;

        let mut stream = TokenStream::new(input, "1.0").unwrap();
        let result = parse_task(&mut stream);
        assert!(result.is_ok());

        let task = result.unwrap();

        match &task.command {
            Expression::String { parts, .. } => {
                // Should have only one Text part since there are no placeholders
                assert_eq!(
                    parts.len(),
                    1,
                    "Expected 1 part (no placeholders), got: {:?}",
                    parts
                );

                if let StringPart::Text(text) = &parts[0] {
                    assert!(text.contains("echo \"Hello, World!\""));
                    assert!(text.contains("ls -la"));
                } else {
                    panic!("Should be a single Text part, got: {:?}", parts[0]);
                }
            }
            _ => panic!("Command should be a String expression"),
        }
    }

    #[test]
    fn test_tilde_placeholder_syntax() {
        let input = r#"task test_tilde_syntax {
            input {
                String name = "test"
            }
            command {
                echo "Processing ~{name}.txt"
            }
        }"#;

        let mut stream = TokenStream::new(input, "1.0").unwrap();
        let result = parse_task(&mut stream);
        assert!(result.is_ok());

        let task = result.unwrap();

        match &task.command {
            Expression::String { parts, .. } => {
                assert_eq!(
                    parts.len(),
                    3,
                    "Expected 3 parts: text + placeholder + text"
                );

                // Check for the tilde placeholder
                let mut found_name_placeholder = false;
                for part in parts {
                    if let StringPart::Placeholder { expr, .. } = part {
                        if let Expression::Ident { name, .. } = expr.as_ref() {
                            if name == "name" {
                                found_name_placeholder = true;
                            }
                        }
                    }
                }

                assert!(found_name_placeholder, "Should find ~{{name}} placeholder");
            }
            _ => panic!("Command should be a String expression"),
        }
    }
}
