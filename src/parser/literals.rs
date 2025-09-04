//! Token-based literal parsing for WDL

use super::expressions::parse_expression;
use super::parser_utils::ParseResult;
use super::token_stream::TokenStream;
use super::tokens::Token;
use crate::error::{SourcePosition, WdlError};
use crate::expr::{Expression, ExpressionBase, StringPart};
use std::collections::HashMap;

/// Parse an integer literal
pub fn parse_int_literal(stream: &mut TokenStream) -> ParseResult<Expression> {
    let pos = stream.current_position();

    match stream.peek_token() {
        Some(Token::IntLiteral(n)) => {
            let value = n;
            stream.next();
            Ok(Expression::int(pos, value))
        }
        _ => Err(WdlError::syntax_error(
            pos,
            "Expected integer literal".to_string(),
            "1.0".to_string(),
            None,
        )),
    }
}

/// Parse a float literal
pub fn parse_float_literal(stream: &mut TokenStream) -> ParseResult<Expression> {
    let pos = stream.current_position();

    match stream.peek_token() {
        Some(Token::FloatLiteral(n)) => {
            let value = n;
            stream.next();
            Ok(Expression::float(pos, value))
        }
        _ => Err(WdlError::syntax_error(
            pos,
            "Expected float literal".to_string(),
            "1.0".to_string(),
            None,
        )),
    }
}

/// Parse a boolean literal
pub fn parse_bool_literal(stream: &mut TokenStream) -> ParseResult<Expression> {
    let pos = stream.current_position();

    match stream.peek_token() {
        Some(Token::BoolLiteral(b)) => {
            let value = b;
            stream.next();
            Ok(Expression::boolean(pos, value))
        }
        Some(Token::Keyword(kw)) if kw == "true" || kw == "false" => {
            let value = stream.next().unwrap();
            let is_true = matches!(value.token, Token::Keyword(ref k) if k == "true");
            Ok(Expression::boolean(pos, is_true))
        }
        _ => Err(WdlError::syntax_error(
            pos,
            "Expected boolean literal".to_string(),
            "1.0".to_string(),
            None,
        )),
    }
}

/// Parse None literal
pub fn parse_none_literal(stream: &mut TokenStream) -> ParseResult<Expression> {
    let pos = stream.current_position();

    match stream.peek_token() {
        Some(Token::Keyword(kw)) if kw == "None" => {
            stream.next();
            Ok(Expression::null(pos))
        }
        _ => Err(WdlError::syntax_error(
            pos,
            "Expected None literal".to_string(),
            "1.0".to_string(),
            None,
        )),
    }
}

/// Parse a string literal with proper lexer mode switching
/// This handles interpolation by switching lexer modes during tokenization
pub fn parse_string_literal(stream: &mut TokenStream) -> ParseResult<Expression> {
    let pos = stream.current_position();

    // Check for the start of a string literal (quote character)
    let quote_char = match stream.peek_token() {
        Some(Token::SingleQuote) => {
            stream.next(); // consume opening quote
            '\''
        }
        Some(Token::DoubleQuote) => {
            stream.next(); // consume opening quote
            '"'
        }
        _ => {
            return Err(WdlError::syntax_error(
                pos,
                "Expected string literal (opening quote)".to_string(),
                "1.0".to_string(),
                None,
            ));
        }
    };

    // Switch lexer to string literal mode
    stream.push_lexer_mode(crate::parser::lexer::LexerMode::StringLiteral);

    let mut parts = Vec::new();

    loop {
        match stream.peek_token() {
            // String end - matching closing quote
            Some(Token::StringEnd(c)) if c == quote_char => {
                stream.next(); // consume closing quote
                stream.pop_lexer_mode(); // Return to normal mode
                break;
            }

            // String text content
            Some(Token::StringText(text)) => {
                let text_content = text.clone();
                stream.next();
                parts.push(StringPart::Text(text_content));
            }

            // Placeholder start
            Some(Token::TildeBrace) | Some(Token::DollarBrace) => {
                stream.next(); // consume placeholder start
                stream.push_lexer_mode(crate::parser::lexer::LexerMode::Placeholder);

                // Parse the expression inside the placeholder
                let expr = parse_expression(stream)?;

                // Expect placeholder end
                stream.expect(Token::PlaceholderEnd)?;
                stream.pop_lexer_mode(); // Return to string literal mode

                parts.push(StringPart::Placeholder {
                    expr: Box::new(expr),
                    options: std::collections::HashMap::new(),
                });
            }

            // Unexpected end of input
            None => {
                return Err(WdlError::syntax_error(
                    stream.current_position(),
                    "Unterminated string literal".to_string(),
                    "1.0".to_string(),
                    None,
                ));
            }

            // Other tokens should not appear in string literal mode
            Some(token) => {
                return Err(WdlError::syntax_error(
                    stream.current_position(),
                    format!("Unexpected token in string literal: {:?}", token),
                    "1.0".to_string(),
                    None,
                ));
            }
        }
    }

    // If no parts were added, add an empty text part
    if parts.is_empty() {
        parts.push(StringPart::Text(String::new()));
    }

    Ok(Expression::string(pos, parts))
}

/// Parse a simple string literal value (for imports and other non-interpolated contexts)
/// Returns the string content directly, not as an Expression
pub fn parse_simple_string_value(stream: &mut TokenStream) -> ParseResult<String> {
    let pos = stream.current_position();

    // Check for the start of a string literal (quote character)
    let quote_char = match stream.peek_token() {
        Some(Token::SingleQuote) => {
            stream.next(); // consume opening quote
            '\''
        }
        Some(Token::DoubleQuote) => {
            stream.next(); // consume opening quote
            '"'
        }
        _ => {
            return Err(WdlError::syntax_error(
                pos,
                "Expected string literal (opening quote)".to_string(),
                "1.0".to_string(),
                None,
            ));
        }
    };

    // Switch lexer to string literal mode
    stream.push_lexer_mode(crate::parser::lexer::LexerMode::StringLiteral);

    let mut result = String::new();

    loop {
        match stream.peek_token() {
            // String end - matching closing quote
            Some(Token::StringEnd(c)) if c == quote_char => {
                stream.next(); // consume closing quote
                stream.pop_lexer_mode(); // Return to normal mode
                break;
            }

            // String text content
            Some(Token::StringText(text)) => {
                let text_content = text.clone();
                stream.next();
                result.push_str(&text_content);
            }

            // Placeholder start - not supported for simple strings
            Some(Token::TildeBrace) | Some(Token::DollarBrace) => {
                return Err(WdlError::syntax_error(
                    stream.current_position(),
                    "String interpolation not supported in this context".to_string(),
                    "1.0".to_string(),
                    None,
                ));
            }

            // Unexpected end of input
            None => {
                return Err(WdlError::syntax_error(
                    stream.current_position(),
                    "Unterminated string literal".to_string(),
                    "1.0".to_string(),
                    None,
                ));
            }

            // Other tokens should not appear in string literal mode
            Some(token) => {
                return Err(WdlError::syntax_error(
                    stream.current_position(),
                    format!("Unexpected token in string literal: {:?}", token),
                    "1.0".to_string(),
                    None,
                ));
            }
        }
    }

    Ok(result)
}

/// Parse any literal expression
pub fn parse_literal(stream: &mut TokenStream) -> ParseResult<Expression> {
    // Try each literal type
    if let Some(token) = stream.peek_token() {
        match token {
            Token::IntLiteral(_) => parse_int_literal(stream),
            Token::FloatLiteral(_) => parse_float_literal(stream),
            Token::BoolLiteral(_) => parse_bool_literal(stream),
            Token::StringLiteral(_) => parse_string_literal(stream),
            Token::SingleQuote | Token::DoubleQuote => parse_string_literal(stream),
            Token::Keyword(kw) if kw == "true" || kw == "false" => parse_bool_literal(stream),
            Token::Keyword(kw) if kw == "None" => parse_none_literal(stream),
            _ => {
                let pos = stream.current_position();
                Err(WdlError::syntax_error(
                    pos,
                    format!("Expected literal, found {:?}", token),
                    "1.0".to_string(),
                    None,
                ))
            }
        }
    } else {
        Err(WdlError::syntax_error(
            stream.current_position(),
            "Expected literal, found EOF".to_string(),
            "1.0".to_string(),
            None,
        ))
    }
}

/// Parse an array literal [elem1, elem2, ...]
pub fn parse_array_literal(stream: &mut TokenStream) -> ParseResult<Expression> {
    let pos = stream.current_position();
    stream.expect(Token::LeftBracket)?;

    let mut elements = Vec::new();

    // Check for empty array
    if stream.peek_token() == Some(Token::RightBracket) {
        stream.next();
        return Ok(Expression::array(pos, elements));
    }

    // Parse first element
    elements.push(parse_expression(stream)?);

    // Parse remaining elements
    while stream.peek_token() == Some(Token::Comma) {
        stream.next(); // consume comma

        // Allow trailing comma
        if stream.peek_token() == Some(Token::RightBracket) {
            break;
        }

        elements.push(parse_expression(stream)?);
    }

    stream.expect(Token::RightBracket)?;
    Ok(Expression::array(pos, elements))
}

/// Parse a map literal {key: value, ...}
pub fn parse_map_literal(stream: &mut TokenStream) -> ParseResult<Expression> {
    let pos = stream.current_position();
    stream.expect(Token::LeftBrace)?;

    let mut pairs = Vec::new();

    // Check for empty map
    if stream.peek_token() == Some(Token::RightBrace) {
        stream.next();
        return Ok(Expression::map(pos, pairs));
    }

    // Parse first pair
    let key = parse_expression(stream)?;
    stream.expect(Token::Colon)?;
    let value = parse_expression(stream)?;
    pairs.push((key, value));

    // Parse remaining pairs
    while stream.peek_token() == Some(Token::Comma) {
        stream.next(); // consume comma

        // Allow trailing comma
        if stream.peek_token() == Some(Token::RightBrace) {
            break;
        }

        let key = parse_expression(stream)?;
        stream.expect(Token::Colon)?;
        let value = parse_expression(stream)?;
        pairs.push((key, value));
    }

    stream.expect(Token::RightBrace)?;
    Ok(Expression::map(pos, pairs))
}

/// Parse a pair literal (left, right)
pub fn parse_pair_literal(stream: &mut TokenStream) -> ParseResult<Expression> {
    let pos = stream.current_position();
    stream.expect(Token::LeftParen)?;

    let left = parse_expression(stream)?;
    stream.expect(Token::Comma)?;
    let right = parse_expression(stream)?;

    stream.expect(Token::RightParen)?;
    Ok(Expression::pair(pos, left, right))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::parser::token_stream::TokenStream;

    #[test]
    fn test_parse_int_literal() {
        let mut stream = TokenStream::new("42", "1.0").unwrap();
        let result = parse_int_literal(&mut stream);
        assert!(result.is_ok());

        let expr = result.unwrap();
        if let Some(value) = expr.literal() {
            assert_eq!(value.as_int(), Some(42));
        } else {
            panic!("Expected literal expression");
        }
    }

    #[test]
    fn test_parse_float_literal() {
        let mut stream = TokenStream::new("3.11", "1.0").unwrap();
        let result = parse_float_literal(&mut stream);
        assert!(result.is_ok());

        let expr = result.unwrap();
        if let Some(value) = expr.literal() {
            assert_eq!(value.as_float(), Some(3.11));
        } else {
            panic!("Expected literal expression");
        }
    }

    #[test]
    fn test_parse_bool_literal() {
        let mut stream = TokenStream::new("true", "1.0").unwrap();
        let result = parse_bool_literal(&mut stream);
        assert!(result.is_ok());

        let expr = result.unwrap();
        if let Some(value) = expr.literal() {
            assert_eq!(value.as_bool(), Some(true));
        } else {
            panic!("Expected literal expression");
        }
    }

    #[test]
    fn test_parse_array_literal() {
        let mut stream = TokenStream::new("[1, 2, 3]", "1.0").unwrap();
        let result = parse_array_literal(&mut stream);
        assert!(result.is_ok());

        let expr = result.unwrap();
        if let Expression::Array { items, .. } = expr {
            assert_eq!(items.len(), 3);
        } else {
            panic!("Expected array expression");
        }
    }

    #[test]
    fn test_parse_empty_array() {
        let mut stream = TokenStream::new("[]", "1.0").unwrap();
        let result = parse_array_literal(&mut stream);
        assert!(result.is_ok());

        let expr = result.unwrap();
        if let Expression::Array { items, .. } = expr {
            assert_eq!(items.len(), 0);
        } else {
            panic!("Expected array expression");
        }
    }

    #[test]
    fn test_parse_literal_auto() {
        // Test integer
        let mut stream = TokenStream::new("42", "1.0").unwrap();
        let result = parse_literal(&mut stream);
        assert!(result.is_ok());

        // Test float
        let mut stream = TokenStream::new("3.11", "1.0").unwrap();
        let result = parse_literal(&mut stream);
        assert!(result.is_ok());

        // Test boolean
        let mut stream = TokenStream::new("true", "1.0").unwrap();
        let result = parse_literal(&mut stream);
        assert!(result.is_ok());

        // Test string
        let mut stream = TokenStream::new("'hello'", "1.0").unwrap();
        let result = parse_literal(&mut stream);
        assert!(result.is_ok());
    }

    #[test]
    fn test_string_interpolation_simple() {
        // Test basic string interpolation
        let mut stream = TokenStream::new("\"Hello ~{name}\"", "1.0").unwrap();
        let result = parse_string_literal(&mut stream);
        assert!(result.is_ok());

        let expr = result.unwrap();
        if let Expression::String { parts, .. } = expr {
            // Should be: "Hello ", placeholder
            assert_eq!(parts.len(), 2);
            assert!(matches!(parts[0], StringPart::Text(ref s) if s == "Hello "));
            assert!(matches!(parts[1], StringPart::Placeholder { .. }));
        } else {
            panic!("Expected String expression");
        }
    }

    #[test]
    fn test_string_interpolation_multiple_variables() {
        // Test multiple variables in one string
        let mut stream = TokenStream::new("\"~{greeting} ~{name}, how are you?\"", "1.0").unwrap();
        let result = parse_string_literal(&mut stream);
        assert!(result.is_ok());

        let expr = result.unwrap();
        if let Expression::String { parts, .. } = expr {
            // Should be: placeholder, " ", placeholder, ", how are you?"
            assert_eq!(parts.len(), 4);
            assert!(matches!(parts[0], StringPart::Placeholder { .. }));
            assert!(matches!(parts[1], StringPart::Text(ref s) if s == " "));
            assert!(matches!(parts[2], StringPart::Placeholder { .. }));
            assert!(matches!(parts[3], StringPart::Text(ref s) if s == ", how are you?"));
        } else {
            panic!("Expected String expression");
        }
    }

    #[test]
    fn test_string_interpolation_no_placeholders() {
        // Test string without placeholders (should still work)
        let mut stream = TokenStream::new("\"Just plain text\"", "1.0").unwrap();
        let result = parse_string_literal(&mut stream);
        assert!(result.is_ok());

        let expr = result.unwrap();
        if let Expression::String { parts, .. } = expr {
            assert_eq!(parts.len(), 1);
            assert!(matches!(parts[0], StringPart::Text(ref s) if s == "Just plain text"));
        } else {
            panic!("Expected String expression");
        }
    }

    #[test]
    fn test_string_interpolation_only_placeholder() {
        // Test string with only a placeholder
        let mut stream = TokenStream::new("\"~{variable}\"", "1.0").unwrap();
        let result = parse_string_literal(&mut stream);
        assert!(result.is_ok());

        let expr = result.unwrap();
        if let Expression::String { parts, .. } = expr {
            assert_eq!(parts.len(), 1);
            assert!(matches!(parts[0], StringPart::Placeholder { .. }));
        } else {
            panic!("Expected String expression");
        }
    }

    #[test]
    fn test_string_literal_scatter_reproduction() {
        use crate::env::Bindings;
        use crate::types::Type;
        use crate::value::Value;

        // This test reproduces the scatter bug scenario
        // When fixed, this should pass
        let mut stream = TokenStream::new("\"~{salutation} ~{name}\"", "1.0").unwrap();
        let result = parse_string_literal(&mut stream);
        assert!(result.is_ok());

        let expr = result.unwrap();

        // Create environment with variables
        let mut env = Bindings::new();
        env = env.bind(
            "salutation".to_string(),
            Value::String {
                value: "Hello".to_string(),
                wdl_type: Type::String { optional: false },
            },
            None,
        );
        env = env.bind(
            "name".to_string(),
            Value::String {
                value: "Joe".to_string(),
                wdl_type: Type::String { optional: false },
            },
            None,
        );

        // Evaluate the expression
        let stdlib = crate::stdlib::StdLib::new("1.2");
        let result = expr.eval(&env, &stdlib);

        // After the fix, this should evaluate to "Hello Joe"
        match result {
            Ok(Value::String { value, .. }) => {
                assert_eq!(value, "Hello Joe");
            }
            _ => {
                panic!("String interpolation should work now");
            }
        }
    }
}
