//! Token-based expression parsing for WDL

use super::literals::{parse_array_literal, parse_literal, parse_map_literal};
use super::parser_utils::{parse_delimited_list, ParseResult};
use super::token_stream::TokenStream;
use super::tokens::Token;
use crate::error::WdlError;
use crate::expr::{BinaryOperator, Expression, UnaryOperator};

/// Parse an identifier expression
pub fn parse_identifier(stream: &mut TokenStream) -> ParseResult<Expression> {
    let pos = stream.current_position();

    match stream.peek_token() {
        Some(Token::Identifier(name)) => {
            let name = name.clone();
            stream.next();
            Ok(Expression::ident(pos, name))
        }
        _ => Err(WdlError::syntax_error(
            pos,
            "Expected identifier".to_string(),
            "1.0".to_string(),
            None,
        )),
    }
}

/// Parse a parenthesized expression
pub fn parse_paren_expr(stream: &mut TokenStream) -> ParseResult<Expression> {
    stream.expect(Token::LeftParen)?;
    let expr = parse_expression(stream)?;
    stream.expect(Token::RightParen)?;
    Ok(expr)
}

/// Parse member access (obj.field)
pub fn parse_member_access(stream: &mut TokenStream, base: Expression) -> ParseResult<Expression> {
    let pos = stream.current_position();
    stream.expect(Token::Dot)?;

    let field = match stream.peek_token() {
        Some(Token::Identifier(name)) => {
            let name = name.clone();
            stream.next();
            name
        }
        _ => {
            return Err(WdlError::syntax_error(
                stream.current_position(),
                "Expected field name after '.'".to_string(),
                "1.0".to_string(),
                None,
            ));
        }
    };

    // Use Get with string literal as index for member access
    Ok(Expression::get(
        pos,
        base,
        Expression::string(
            stream.current_position(),
            vec![crate::expr::StringPart::Text(field)],
        ),
    ))
}

/// Parse array index (arr[index])
pub fn parse_array_index(stream: &mut TokenStream, base: Expression) -> ParseResult<Expression> {
    let pos = stream.current_position();
    stream.expect(Token::LeftBracket)?;
    let index = parse_expression(stream)?;
    stream.expect(Token::RightBracket)?;
    Ok(Expression::get(pos, base, index))
}

/// Parse function call (func(args))
pub fn parse_function_call(stream: &mut TokenStream, func: Expression) -> ParseResult<Expression> {
    let pos = stream.current_position();

    let args = parse_delimited_list(
        stream,
        Token::LeftParen,
        Token::RightParen,
        Token::Comma,
        parse_expression,
    )?;

    // If func is an identifier, create a proper function call
    if let Expression::Ident { name, .. } = func {
        Ok(Expression::apply(pos, name, args))
    } else {
        // For non-identifier function expressions, wrap in special apply
        Ok(Expression::apply(
            pos,
            "__apply__".to_string(),
            vec![func].into_iter().chain(args).collect(),
        ))
    }
}

/// Parse postfix expressions (member access, array indexing, function calls)
pub fn parse_postfix_expr(stream: &mut TokenStream) -> ParseResult<Expression> {
    let mut expr = parse_primary_expr(stream)?;

    loop {
        match stream.peek_token() {
            Some(Token::Dot) => {
                expr = parse_member_access(stream, expr)?;
            }
            Some(Token::LeftBracket) => {
                expr = parse_array_index(stream, expr)?;
            }
            Some(Token::LeftParen) => {
                expr = parse_function_call(stream, expr)?;
            }
            _ => break,
        }
    }

    Ok(expr)
}

/// Parse unary expression
pub fn parse_unary_expr(stream: &mut TokenStream) -> ParseResult<Expression> {
    let pos = stream.current_position();

    // Check for unary operators
    let op = match stream.peek_token() {
        Some(Token::Not) => {
            stream.next();
            Some(UnaryOperator::Not)
        }
        Some(Token::Minus) => {
            stream.next();
            Some(UnaryOperator::Negate)
        }
        _ => None,
    };

    if let Some(op) = op {
        let expr = parse_unary_expr(stream)?;
        Ok(Expression::unary_op(pos, op, expr))
    } else {
        parse_postfix_expr(stream)
    }
}

/// Get operator precedence
fn get_precedence(token: &Token) -> Option<u8> {
    match token {
        Token::Or => Some(1),
        Token::And => Some(2),
        Token::Equal | Token::NotEqual => Some(3),
        Token::Less | Token::LessEqual | Token::Greater | Token::GreaterEqual => Some(4),
        Token::Plus | Token::Minus => Some(5),
        Token::Star | Token::Slash | Token::Percent => Some(6),
        _ => None,
    }
}

/// Convert token to binary operator
fn token_to_binary_op(token: &Token) -> Option<BinaryOperator> {
    match token {
        Token::Plus => Some(BinaryOperator::Add),
        Token::Minus => Some(BinaryOperator::Subtract),
        Token::Star => Some(BinaryOperator::Multiply),
        Token::Slash => Some(BinaryOperator::Divide),
        Token::Percent => Some(BinaryOperator::Modulo),
        Token::Equal => Some(BinaryOperator::Equal),
        Token::NotEqual => Some(BinaryOperator::NotEqual),
        Token::Less => Some(BinaryOperator::Less),
        Token::LessEqual => Some(BinaryOperator::LessEqual),
        Token::Greater => Some(BinaryOperator::Greater),
        Token::GreaterEqual => Some(BinaryOperator::GreaterEqual),
        Token::And => Some(BinaryOperator::And),
        Token::Or => Some(BinaryOperator::Or),
        _ => None,
    }
}

/// Parse binary expression with operator precedence
pub fn parse_binary_expr(stream: &mut TokenStream, min_precedence: u8) -> ParseResult<Expression> {
    let mut left = parse_unary_expr(stream)?;

    loop {
        // Check for binary operator
        let precedence = match stream.peek_token().as_ref() {
            Some(token) => match get_precedence(token) {
                Some(prec) if prec >= min_precedence => prec,
                _ => break,
            },
            None => break,
        };

        let op = match stream.peek_token().as_ref().and_then(token_to_binary_op) {
            Some(op) => {
                stream.next();
                op
            }
            None => break,
        };

        let pos = stream.current_position();
        let right = parse_binary_expr(stream, precedence + 1)?;
        left = Expression::binary_op(pos, op, left, right);
    }

    Ok(left)
}

/// Parse ternary conditional expression (test ? if_true : if_false)
pub fn parse_ternary_expr(stream: &mut TokenStream) -> ParseResult<Expression> {
    let condition = parse_binary_expr(stream, 1)?;

    // Check for ternary operator
    if stream.peek_token() == Some(Token::Question) {
        let pos = stream.current_position();
        stream.next(); // consume ?

        let if_true = parse_expression(stream)?;
        stream.expect(Token::Colon)?;
        let if_false = parse_expression(stream)?;

        Ok(Expression::if_then_else(pos, condition, if_true, if_false))
    } else {
        Ok(condition)
    }
}

/// Parse primary expression
pub fn parse_primary_expr(stream: &mut TokenStream) -> ParseResult<Expression> {
    // Try different primary expression types
    if let Some(token) = stream.peek_token() {
        match token {
            Token::LeftParen => {
                // Could be parenthesized expression or pair literal
                // Look ahead to distinguish
                let _pos = stream.position();
                stream.next(); // consume (

                let first = parse_expression(stream)?;

                if stream.peek_token() == Some(Token::Comma) {
                    // It's a pair literal
                    stream.next(); // consume ,
                    let second = parse_expression(stream)?;
                    stream.expect(Token::RightParen)?;
                    let pair_pos = stream.current_position();
                    Ok(Expression::pair(pair_pos, first, second))
                } else {
                    // It's a parenthesized expression
                    stream.expect(Token::RightParen)?;
                    Ok(first)
                }
            }
            Token::LeftBracket => parse_array_literal(stream),
            Token::LeftBrace => parse_map_literal(stream),
            Token::Identifier(_) => parse_identifier(stream),
            Token::IntLiteral(_)
            | Token::FloatLiteral(_)
            | Token::BoolLiteral(_)
            | Token::StringLiteral(_) => parse_literal(stream),
            Token::Keyword(kw) if kw == "true" || kw == "false" || kw == "None" => {
                parse_literal(stream)
            }
            _ => {
                let pos = stream.current_position();
                Err(WdlError::syntax_error(
                    pos,
                    format!("Unexpected token in expression: {:?}", token),
                    "1.0".to_string(),
                    None,
                ))
            }
        }
    } else {
        Err(WdlError::syntax_error(
            stream.current_position(),
            "Unexpected end of input in expression".to_string(),
            "1.0".to_string(),
            None,
        ))
    }
}

/// Parse any expression
pub fn parse_expression(stream: &mut TokenStream) -> ParseResult<Expression> {
    parse_ternary_expr(stream)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::parser::token_stream::TokenStream;

    #[test]
    fn test_parse_identifier() {
        let mut stream = TokenStream::new("foo", "1.0").unwrap();
        let result = parse_identifier(&mut stream);
        assert!(result.is_ok());

        let expr = result.unwrap();
        if let Expression::Ident { name, .. } = expr {
            assert_eq!(name, "foo");
        } else {
            panic!("Expected identifier expression");
        }
    }

    #[test]
    fn test_parse_binary_expr() {
        let mut stream = TokenStream::new("1 + 2 * 3", "1.0").unwrap();
        let result = parse_expression(&mut stream);
        assert!(result.is_ok());
        assert!(stream.is_eof());
    }

    #[test]
    fn test_parse_function_call() {
        let mut stream = TokenStream::new("func(1, 2, 3)", "1.0").unwrap();
        let result = parse_expression(&mut stream);
        assert!(result.is_ok());

        let expr = result.unwrap();
        if let Expression::Apply {
            function_name,
            arguments,
            ..
        } = expr
        {
            assert_eq!(function_name, "func");
            assert_eq!(arguments.len(), 3);
        } else {
            panic!("Expected function call expression");
        }
    }

    #[test]
    fn test_parse_member_access() {
        let mut stream = TokenStream::new("obj.field", "1.0").unwrap();
        let result = parse_expression(&mut stream);
        assert!(result.is_ok());

        // Member access is implemented as Get with string index
        let expr = result.unwrap();
        if let Expression::Get { .. } = expr {
            // OK
        } else {
            panic!("Expected Get expression for member access");
        }
    }

    #[test]
    fn test_parse_array_index() {
        let mut stream = TokenStream::new("arr[0]", "1.0").unwrap();
        let result = parse_expression(&mut stream);
        assert!(result.is_ok());
        assert!(stream.is_eof());
    }

    #[test]
    fn test_parse_ternary() {
        let mut stream = TokenStream::new("x > 0 ? x : -x", "1.0").unwrap();
        let result = parse_expression(&mut stream);
        assert!(result.is_ok());

        let expr = result.unwrap();
        if let Expression::IfThenElse { .. } = expr {
            // OK
        } else {
            panic!("Expected ternary expression");
        }
    }

    #[test]
    fn test_parse_complex_expr() {
        let mut stream = TokenStream::new("foo.bar[0] + baz(1, 2) * 3", "1.0").unwrap();
        let result = parse_expression(&mut stream);
        assert!(result.is_ok());
        assert!(stream.is_eof());
    }

    #[test]
    fn test_parse_pair_literal() {
        let mut stream = TokenStream::new("(1, 2)", "1.0").unwrap();
        let result = parse_expression(&mut stream);
        assert!(result.is_ok());

        let expr = result.unwrap();
        if let Expression::Pair { .. } = expr {
            // OK
        } else {
            panic!("Expected pair expression");
        }
    }
}
