//! Literal expression constructors and utilities

use super::{Expression, StringPart};
use crate::error::SourcePosition;
use crate::parser::expressions::parse_expression;
use crate::parser::token_stream::TokenStream;

impl Expression {
    /// Create a new Boolean expression
    pub fn boolean(pos: SourcePosition, value: bool) -> Self {
        Expression::Boolean {
            pos,
            value,
            inferred_type: None,
        }
    }

    /// Create a new Int expression
    pub fn int(pos: SourcePosition, value: i64) -> Self {
        Expression::Int {
            pos,
            value,
            inferred_type: None,
        }
    }

    /// Create a new Float expression
    pub fn float(pos: SourcePosition, value: f64) -> Self {
        Expression::Float {
            pos,
            value,
            inferred_type: None,
        }
    }

    /// Create a new String expression
    pub fn string(pos: SourcePosition, parts: Vec<StringPart>) -> Self {
        Expression::String {
            pos,
            parts,
            inferred_type: None,
        }
    }

    /// Create a new simple string literal
    pub fn string_literal(pos: SourcePosition, value: String) -> Self {
        Expression::String {
            pos,
            parts: vec![StringPart::Text(value)],
            inferred_type: None,
        }
    }

    /// Create a new string with placeholder parsing for WDL variable substitution
    /// Parses ${variable} and ~{expression} patterns in the input text
    pub fn string_with_placeholders(
        pos: SourcePosition,
        text: String,
    ) -> Result<Self, crate::error::WdlError> {
        use regex::Regex;
        use std::collections::HashMap;

        // Regex to match ${variable} or ~{expression} patterns
        let placeholder_regex = Regex::new(r"(\$\{[^}]+\}|~\{[^}]+\})").unwrap();

        let mut parts = Vec::new();
        let mut last_end = 0;

        for mat in placeholder_regex.find_iter(&text) {
            // Add text before this placeholder
            if mat.start() > last_end {
                let text_part = text[last_end..mat.start()].to_string();
                if !text_part.is_empty() {
                    parts.push(StringPart::Text(text_part));
                }
            }

            let placeholder_text = mat.as_str();

            // Parse the placeholder content
            if let Some(inner) = placeholder_text
                .strip_prefix("${")
                .and_then(|s| s.strip_suffix("}"))
            {
                // Parse expression inside ${...}
                let parsed_expr = parse_placeholder_expression(inner, pos.clone())?;
                parts.push(StringPart::Placeholder {
                    expr: Box::new(parsed_expr),
                    options: HashMap::new(),
                });
            } else if let Some(inner) = placeholder_text
                .strip_prefix("~{")
                .and_then(|s| s.strip_suffix("}"))
            {
                // Parse expression inside ~{...}
                let parsed_expr = parse_placeholder_expression(inner, pos.clone())?;
                parts.push(StringPart::Placeholder {
                    expr: Box::new(parsed_expr),
                    options: HashMap::new(),
                });
            }

            last_end = mat.end();
        }

        // Add any remaining text after the last placeholder
        if last_end < text.len() {
            let remaining_text = text[last_end..].to_string();
            if !remaining_text.is_empty() {
                parts.push(StringPart::Text(remaining_text));
            }
        }

        // If no placeholders were found, return a simple string literal
        if parts.is_empty() {
            parts.push(StringPart::Text(text));
        }

        Ok(Expression::String {
            pos,
            parts,
            inferred_type: None,
        })
    }

    /// Create a new Null expression
    pub fn null(pos: SourcePosition) -> Self {
        Expression::Null {
            pos,
            inferred_type: None,
        }
    }

    /// Create a new Ident expression
    pub fn ident(pos: SourcePosition, name: String) -> Self {
        Expression::Ident {
            pos,
            name,
            inferred_type: None,
        }
    }
}

/// Parse an expression inside a placeholder (${...} or ~{...})
fn parse_placeholder_expression(
    inner: &str,
    pos: SourcePosition,
) -> Result<Expression, crate::error::WdlError> {
    // Create a TokenStream for the placeholder content
    let mut stream =
        TokenStream::new(inner, "1.0").map_err(|e| crate::error::WdlError::RuntimeError {
            message: format!(
                "Failed to tokenize placeholder expression '{}': {}",
                inner, e
            ),
        })?;

    // Parse the expression
    parse_expression(&mut stream).map_err(|e| crate::error::WdlError::RuntimeError {
        message: format!("Failed to parse placeholder expression '{}': {}", inner, e),
    })
}
