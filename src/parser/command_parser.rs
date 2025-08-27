//! Special parser for WDL command blocks that handles shell syntax
//!
//! Command blocks need special handling because they contain shell code
//! with WDL placeholders. We can't tokenize them normally.

use crate::error::{SourcePosition, WdlError};
use crate::expr::{Expression, StringPart};
use crate::parser::expressions::parse_expression;
use crate::parser::token_stream::TokenStream;

/// Parse a raw command block with heredoc syntax <<<...>>>
pub fn parse_raw_heredoc_command(
    input: &str,
    pos: &SourcePosition,
) -> Result<Expression, WdlError> {
    // Find the opening <<<
    let start_idx = input.find("<<<").ok_or_else(|| {
        WdlError::syntax_error(
            pos.clone(),
            "Expected '<<<' to start heredoc command".to_string(),
            "1.0".to_string(),
            None,
        )
    })?;

    // Find the closing >>>
    let end_idx = input.find(">>>").ok_or_else(|| {
        WdlError::syntax_error(
            pos.clone(),
            "Expected '>>>' to end heredoc command".to_string(),
            "1.0".to_string(),
            None,
        )
    })?;

    // Extract command content between <<< and >>>
    let command_content = &input[start_idx + 3..end_idx];

    // Create a TokenStream for the command content and set it to command mode
    let mut stream = TokenStream::new(command_content, "1.0")?;
    stream.enter_command_mode();

    // Parse the command content for placeholders
    let parts = parse_command_content(&mut stream)?;

    Ok(Expression::string(pos.clone(), parts))
}

/// Parse command content for WDL placeholders (~{} and ${})
fn parse_command_content(stream: &mut TokenStream) -> Result<Vec<StringPart>, WdlError> {
    let mut parts = Vec::new();
    let mut current_text = String::new();

    // Process tokens in command mode
    while !stream.is_eof() {
        if let Some(token) = stream.peek() {
            match &token.token {
                crate::parser::tokens::Token::TildeBrace
                | crate::parser::tokens::Token::DollarBrace => {
                    // Found placeholder start
                    if !current_text.is_empty() {
                        parts.push(StringPart::Text(current_text.clone()));
                        current_text.clear();
                    }

                    stream.next(); // consume the placeholder start token

                    // Switch to normal mode for expression parsing
                    stream.exit_command_mode();

                    // Parse the expression inside the placeholder
                    let expr = parse_expression(stream).map_err(|e| WdlError::RuntimeError {
                        message: format!("Failed to parse placeholder expression: {}", e),
                    })?;

                    // Expect closing brace
                    stream
                        .expect(crate::parser::tokens::Token::RightBrace)
                        .map_err(|e| WdlError::RuntimeError {
                            message: format!("Expected '}}' to close placeholder: {}", e),
                        })?;

                    // Switch back to command mode
                    stream.enter_command_mode();

                    // Create placeholder
                    parts.push(StringPart::Placeholder {
                        expr: Box::new(expr),
                        options: std::collections::HashMap::new(),
                    });
                }
                crate::parser::tokens::Token::CommandText(text)
                | crate::parser::tokens::Token::Whitespace(text) => {
                    // Add text content
                    current_text.push_str(text);
                    stream.next(); // consume the token
                }
                crate::parser::tokens::Token::Newline => {
                    current_text.push('\n');
                    stream.next(); // consume the token
                }
                _ => {
                    // For any other token in command mode, treat as text
                    current_text.push_str(&format!("{}", token.token));
                    stream.next(); // consume the token
                }
            }
        } else {
            break;
        }
    }

    // Add any remaining text
    if !current_text.is_empty() {
        parts.push(StringPart::Text(current_text));
    }

    // If no parts, add empty string
    if parts.is_empty() {
        parts.push(StringPart::Text(String::new()));
    }

    Ok(parts)
}

/// Parse a command block with braces { ... }
pub fn parse_raw_command_block(input: &str, pos: &SourcePosition) -> Result<Expression, WdlError> {
    // Find the opening {
    let start_idx = input.find('{').ok_or_else(|| {
        WdlError::syntax_error(
            pos.clone(),
            "Expected '{' to start command block".to_string(),
            "1.0".to_string(),
            None,
        )
    })?;

    // Find matching closing } (accounting for nesting)
    let mut depth = 0;
    let mut end_idx = None;

    for (i, ch) in input[start_idx..].char_indices() {
        match ch {
            '{' => depth += 1,
            '}' => {
                depth -= 1;
                if depth == 0 {
                    end_idx = Some(start_idx + i);
                    break;
                }
            }
            _ => {}
        }
    }

    let end_idx = end_idx.ok_or_else(|| {
        WdlError::syntax_error(
            pos.clone(),
            "Unclosed command block".to_string(),
            "1.0".to_string(),
            None,
        )
    })?;

    // Extract command content
    let command_content = &input[start_idx + 1..end_idx];

    // Create a TokenStream for the command content and set it to command mode
    let mut stream = TokenStream::new(command_content, "1.0")?;
    stream.enter_command_mode();

    // Parse the command content for placeholders
    let parts = parse_command_content(&mut stream)?;

    Ok(Expression::string(pos.clone(), parts))
}
