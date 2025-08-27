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

/// Parse a string literal
/// Note: In the token-based approach, string parsing is simplified
/// as the lexer has already handled escape sequences and basic structure
pub fn parse_string_literal(stream: &mut TokenStream) -> ParseResult<Expression> {
    let pos = stream.current_position();

    match stream.peek_token() {
        Some(Token::StringLiteral(s)) => {
            let content = s.clone();
            stream.next();

            // For now, treat as simple string without interpolation
            // TODO: Parse string interpolation if needed
            let parts = vec![StringPart::Text(content)];
            Ok(Expression::string(pos, parts))
        }
        _ => Err(WdlError::syntax_error(
            pos,
            "Expected string literal".to_string(),
            "1.0".to_string(),
            None,
        )),
    }
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
}
