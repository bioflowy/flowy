//! Special parser for WDL command blocks that handles shell syntax
//!
//! Command blocks need special handling because they contain shell code
//! with WDL placeholders. We can't tokenize them normally.

use crate::error::{SourcePosition, WdlError};
use crate::expr::{Expression, StringPart};

/// Parse a raw command block with heredoc syntax <<<...>>>
pub fn parse_raw_heredoc_command(input: &str, pos: &SourcePosition) -> Result<Expression, WdlError> {
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
    
    // Parse the command content for placeholders
    let parts = parse_command_content(command_content)?;
    
    Ok(Expression::string(pos.clone(), parts))
}

/// Parse command content for WDL placeholders (~{} and ${})
fn parse_command_content(content: &str) -> Result<Vec<StringPart>, WdlError> {
    let mut parts = Vec::new();
    let mut current = String::new();
    let mut chars = content.chars().peekable();
    
    while let Some(ch) = chars.next() {
        match ch {
            '~' | '$' => {
                // Check if this starts a placeholder
                if chars.peek() == Some(&'{') {
                    // Found placeholder start
                    if !current.is_empty() {
                        parts.push(StringPart::Text(current.clone()));
                        current.clear();
                    }
                    
                    chars.next(); // consume '{'
                    
                    // Find the matching }
                    let mut placeholder_content = String::new();
                    let mut depth = 1;
                    
                    while depth > 0 {
                        match chars.next() {
                            Some('{') => {
                                placeholder_content.push('{');
                                depth += 1;
                            }
                            Some('}') => {
                                depth -= 1;
                                if depth > 0 {
                                    placeholder_content.push('}');
                                }
                            }
                            Some(c) => {
                                placeholder_content.push(c);
                            }
                            None => {
                                return Err(WdlError::RuntimeError {
                                    message: "Unclosed placeholder in command".to_string(),
                                });
                            }
                        }
                    }
                    
                    // For now, treat placeholder as raw text
                    // In a full implementation, we'd parse the expression
                    let placeholder_marker = if ch == '~' { "~" } else { "$" };
                    parts.push(StringPart::Text(format!("{}{{{}}}", placeholder_marker, placeholder_content)));
                } else {
                    // Not a placeholder, just regular text
                    current.push(ch);
                }
            }
            _ => {
                current.push(ch);
            }
        }
    }
    
    // Add any remaining text
    if !current.is_empty() {
        parts.push(StringPart::Text(current));
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
    
    // Parse the command content for placeholders
    let parts = parse_command_content(command_content)?;
    
    Ok(Expression::string(pos.clone(), parts))
}