//! Token-based expression parsing for WDL
//!
//! This module implements expression parsing following the Python miniwdl grammar structure.
//! Each function corresponds to a specific grammar rule from miniwdl/WDL/_grammar.py

use super::literals::{parse_array_literal, parse_literal, parse_map_literal};
use super::parser_utils::{parse_delimited_list, ParseResult};
use super::token_stream::TokenStream;
use super::tokens::Token;
use crate::error::WdlError;
use crate::expr::{BinaryOperator, Expression, StringPart, UnaryOperator};

/// Parse an identifier expression  
/// Python grammar: CNAME -> left_name
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

/// Parse a parenthesized expression or pair literal
/// Python grammar: "(" expr ")" or "(" expr "," expr ")" -> pair
pub fn parse_paren_or_pair(stream: &mut TokenStream) -> ParseResult<Expression> {
    stream.expect(Token::LeftParen)?;
    let first = parse_expression(stream)?;

    if stream.peek_token() == Some(Token::Comma) {
        // It's a pair literal: (expr, expr)
        let pos = stream.current_position();
        stream.next(); // consume ,
        let second = parse_expression(stream)?;
        stream.expect(Token::RightParen)?;
        Ok(Expression::pair(pos, first, second))
    } else {
        // It's a parenthesized expression: (expr)
        stream.expect(Token::RightParen)?;
        Ok(first)
    }
}

/// Parse member access (obj.field)
/// Python grammar: expr_core "." CNAME -> get_name
pub fn parse_member_access(stream: &mut TokenStream, base: Expression) -> ParseResult<Expression> {
    let pos = stream.current_position();
    stream.expect(Token::Dot)?;

    let field = match stream.peek_token() {
        Some(Token::Identifier(name)) => {
            let name = name.clone();
            stream.next();
            name
        }
        Some(Token::Keyword(name)) => {
            // Allow keywords as field names for member access
            // This is common for Pair access like .left and .right
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
        Expression::string(stream.current_position(), vec![StringPart::Text(field)]),
    ))
}

/// Parse array index (arr[index])
/// Python grammar: expr_core "[" expr "]" -> at
pub fn parse_array_index(stream: &mut TokenStream, base: Expression) -> ParseResult<Expression> {
    let pos = stream.current_position();
    stream.expect(Token::LeftBracket)?;
    let index = parse_expression(stream)?;
    stream.expect(Token::RightBracket)?;
    Ok(Expression::get(pos, base, index))
}

/// Parse function call (func(args))
/// Python grammar: CNAME "(" [expr ("," expr)*] ")" -> apply
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
/// This handles the postfix operations on expr_core
pub fn parse_postfix_expr(stream: &mut TokenStream) -> ParseResult<Expression> {
    let mut expr = parse_expr_core_base(stream)?;

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
/// Python grammar: "!" expr_core -> negate
/// Note: Unary minus is also handled here though not explicitly in expr_core grammar
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

/// Get operator precedence level (matching Python grammar hierarchy)
/// Lower numbers = lower precedence (evaluated later)
fn get_precedence_level(token: &Token) -> Option<u8> {
    match token {
        Token::Or => Some(0),                // expr_infix0: ||  
        Token::And => Some(1),               // expr_infix1: &&
        Token::Equal | Token::NotEqual |     // expr_infix2: ==, !=, <, <=, >, >=
        Token::Less | Token::LessEqual |
        Token::Greater | Token::GreaterEqual => Some(2),
        Token::Plus | Token::Minus => Some(3), // expr_infix3: +, -
        Token::Star | Token::Slash | Token::Percent => Some(4), // expr_infix4: *, /, %
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

/// Parse infix expressions with proper precedence
/// Python grammar: expr_infix0 through expr_infix4
/// This uses precedence climbing to handle left-associative binary operators
pub fn parse_expr_infix(stream: &mut TokenStream, min_precedence: u8) -> ParseResult<Expression> {
    let mut left = parse_expr_infix5(stream)?;

    loop {
        // Check for binary operator at current precedence level
        let precedence = match stream.peek_token().as_ref() {
            Some(token) => match get_precedence_level(token) {
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
        // For left-associative operators, use precedence + 1
        let right = parse_expr_infix(stream, precedence + 1)?;
        left = Expression::binary_op(pos, op, left, right);
    }

    Ok(left)
}

/// Parse expressions (may include ternary ? : operator)
/// Note: This handles the C-style ternary operator, not the if-then-else expression
pub fn parse_expr_with_ternary(stream: &mut TokenStream) -> ParseResult<Expression> {
    let expr = parse_expr_infix(stream, 0)?;

    // Check for ternary operator
    if stream.peek_token() == Some(Token::Question) {
        let pos = stream.current_position();
        stream.next(); // consume ?

        let if_true = parse_expression(stream)?;
        stream.expect(Token::Colon)?;
        let if_false = parse_expression(stream)?;

        Ok(Expression::if_then_else(pos, expr, if_true, if_false))
    } else {
        Ok(expr)
    }
}

/// Parse expr_core base elements (without postfix operations)
/// Python grammar: expr_core -> various primary expressions
pub fn parse_expr_core_base(stream: &mut TokenStream) -> ParseResult<Expression> {
    // Try different primary expression types
    if let Some(token) = stream.peek_token() {
        match token {
            Token::LeftParen => parse_paren_or_pair(stream),
            Token::LeftBracket => parse_array_literal(stream),
            Token::LeftBrace => parse_map_literal(stream),
            Token::Identifier(name) => {
                // Could be: identifier, function call, or object literal
                let name = name.clone();
                let pos = stream.current_position();
                stream.next();

                // Check what follows the identifier
                match stream.peek_token() {
                    Some(Token::LeftParen) => {
                        // Function call: CNAME "(" ... ")"
                        let args = parse_delimited_list(
                            stream,
                            Token::LeftParen,
                            Token::RightParen,
                            Token::Comma,
                            parse_expression,
                        )?;
                        Ok(Expression::apply(pos, name, args))
                    }
                    Some(Token::LeftBrace) => {
                        // Object literal: CNAME "{" ... "}"
                        parse_object_literal(stream, name, pos)
                    }
                    _ => {
                        // Just an identifier
                        Ok(Expression::ident(pos, name))
                    }
                }
            }
            Token::IntLiteral(_)
            | Token::FloatLiteral(_)
            | Token::BoolLiteral(_)
            | Token::StringLiteral(_) => parse_literal(stream),
            Token::Keyword(kw) if kw == "true" || kw == "false" || kw == "None" => {
                parse_literal(stream)
            }
            Token::Keyword(kw) if kw == "if" => {
                // if-then-else expression
                parse_if_then_else(stream)
            }
            Token::HeredocStart => {
                // Multistring (heredoc): <<<...>>>
                parse_multistring(stream)
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

/// Python grammar: expr_infix5 -> expr_core
/// This is the bridge between infix precedence levels and core expressions
pub fn parse_expr_infix5(stream: &mut TokenStream) -> ParseResult<Expression> {
    parse_expr_core(stream)
}

/// Python grammar: expr_core
/// Handles all core expressions including unary operations and postfix operations
pub fn parse_expr_core(stream: &mut TokenStream) -> ParseResult<Expression> {
    // Check for unary operators first
    if let Some(token) = stream.peek_token() {
        match token {
            Token::Not => {
                let pos = stream.current_position();
                stream.next();
                let expr = parse_expr_core(stream)?;
                return Ok(Expression::unary_op(pos, UnaryOperator::Not, expr));
            }
            Token::Minus => {
                // Need to distinguish between unary minus and binary minus
                // This is unary if it's at the start of an expression
                let pos = stream.current_position();
                stream.next();
                let expr = parse_expr_core(stream)?;
                return Ok(Expression::unary_op(pos, UnaryOperator::Negate, expr));
            }
            _ => {}
        }
    }

    // Parse postfix expressions (includes primary expressions)
    parse_postfix_expr(stream)
}

/// Python grammar: expr -> expr_infix
/// This is the main entry point for expression parsing
pub fn parse_expr(stream: &mut TokenStream) -> ParseResult<Expression> {
    parse_expr_with_ternary(stream)
}

/// Parse any expression (convenience function maintaining backward compatibility)
pub fn parse_expression(stream: &mut TokenStream) -> ParseResult<Expression> {
    parse_expr(stream)
}

/// Parse if-then-else expression
/// Python grammar: "if" expr "then" expr "else" expr -> ifthenelse
pub fn parse_if_then_else(stream: &mut TokenStream) -> ParseResult<Expression> {
    let pos = stream.current_position();

    // Expect 'if' keyword
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
            ))
        }
    }

    // Parse condition
    let condition = parse_expr(stream)?;

    // Expect 'then' keyword
    match stream.peek_token() {
        Some(Token::Keyword(kw)) if kw == "then" => {
            stream.next();
        }
        _ => {
            return Err(WdlError::syntax_error(
                stream.current_position(),
                "Expected 'then' keyword after condition".to_string(),
                "1.0".to_string(),
                None,
            ))
        }
    }

    // Parse true expression
    let true_expr = parse_expr(stream)?;

    // Expect 'else' keyword
    match stream.peek_token() {
        Some(Token::Keyword(kw)) if kw == "else" => {
            stream.next();
        }
        _ => {
            return Err(WdlError::syntax_error(
                stream.current_position(),
                "Expected 'else' keyword".to_string(),
                "1.0".to_string(),
                None,
            ))
        }
    }

    // Parse false expression
    let false_expr = parse_expr(stream)?;

    Ok(Expression::if_then_else(
        pos, condition, true_expr, false_expr,
    ))
}

/// Parse multistring (heredoc) expression
/// Python grammar: /<<</ (COMMAND2_FRAGMENT? "~{" placeholder "}")* COMMAND2_FRAGMENT? />>>/ -> string
pub fn parse_multistring(stream: &mut TokenStream) -> ParseResult<Expression> {
    let pos = stream.current_position();

    // Expect heredoc start
    stream.expect(Token::HeredocStart)?;

    // Switch to command mode for proper tokenization
    stream.enter_command_mode();

    let mut parts = Vec::new();
    let mut current_text = String::new();

    loop {
        match stream.peek_token() {
            Some(Token::HeredocEnd) => {
                stream.next();
                break;
            }
            Some(Token::TildeBrace) => {
                // Save any accumulated text
                if !current_text.is_empty() {
                    parts.push(StringPart::Text(current_text.clone()));
                    current_text.clear();
                }

                // Parse placeholder
                stream.next(); // consume ~{
                let expr = parse_expr(stream)?;
                stream.expect(Token::PlaceholderEnd)?;
                parts.push(StringPart::Placeholder {
                    expr: Box::new(expr),
                    options: std::collections::HashMap::new(),
                });
            }
            Some(Token::CommandText(text)) => {
                current_text.push_str(&text);
                stream.next();
            }
            Some(Token::Newline) => {
                current_text.push('\n');
                stream.next();
            }
            _ => {
                stream.exit_command_mode();
                return Err(WdlError::syntax_error(
                    stream.current_position(),
                    "Unexpected token in multistring".to_string(),
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

    stream.exit_command_mode();
    Ok(Expression::string(pos, parts))
}

/// Parse object literal
/// Python grammar: CNAME "{" [object_kv ("," object_kv)* ","?] "}" -> obj
pub fn parse_object_literal(
    stream: &mut TokenStream,
    type_name: String,
    pos: crate::error::SourcePosition,
) -> ParseResult<Expression> {
    stream.expect(Token::LeftBrace)?;

    let mut members = Vec::new();

    // Parse object key-value pairs
    while stream.peek_token() != Some(Token::RightBrace) {
        // Parse key (can be identifier or string literal)
        let key = match stream.peek_token() {
            Some(Token::Identifier(name)) => {
                let name = name.clone();
                stream.next();
                name
            }
            Some(Token::StringLiteral(s)) => {
                let s = s.clone();
                stream.next();
                s
            }
            _ => {
                return Err(WdlError::syntax_error(
                    stream.current_position(),
                    "Expected identifier or string literal for object key".to_string(),
                    "1.0".to_string(),
                    None,
                ));
            }
        };

        // Expect colon
        stream.expect(Token::Colon)?;

        // Parse value expression
        let value = parse_expr(stream)?;

        members.push((key, value));

        // Check for comma or end of object
        if stream.peek_token() == Some(Token::Comma) {
            stream.next();
            // Allow trailing comma
            if stream.peek_token() == Some(Token::RightBrace) {
                break;
            }
        } else if stream.peek_token() != Some(Token::RightBrace) {
            return Err(WdlError::syntax_error(
                stream.current_position(),
                "Expected ',' or '}' in object literal".to_string(),
                "1.0".to_string(),
                None,
            ));
        }
    }

    stream.expect(Token::RightBrace)?;

    // Create a Struct expression with the type name embedded
    // The type_name can be used during type checking to validate against the struct definition
    Ok(Expression::struct_expr(pos, members))
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

    #[test]
    fn test_parse_if_then_else() {
        let mut stream = TokenStream::new("if x > 0 then x else 0", "1.0").unwrap();
        let result = parse_expression(&mut stream);
        assert!(result.is_ok());

        let expr = result.unwrap();
        if let Expression::IfThenElse {
            condition,
            true_expr,
            false_expr,
            ..
        } = expr
        {
            // Verify it's an if-then-else expression
            assert!(matches!(condition.as_ref(), Expression::BinaryOp { .. }));
            assert!(matches!(true_expr.as_ref(), Expression::Ident { .. }));
            assert!(matches!(false_expr.as_ref(), Expression::Int { .. }));
        } else {
            panic!("Expected if-then-else expression");
        }
    }

    #[test]
    fn test_parse_multistring_simple() {
        let mut stream = TokenStream::new("<<<hello world>>>", "1.0").unwrap();
        let result = parse_expression(&mut stream);
        assert!(result.is_ok());

        let expr = result.unwrap();
        if let Expression::String { parts, .. } = expr {
            assert_eq!(parts.len(), 1);
            if let StringPart::Text(text) = &parts[0] {
                assert_eq!(text, "hello world");
            } else {
                panic!("Expected text part");
            }
        } else {
            panic!("Expected string expression");
        }
    }

    #[test]
    fn test_parse_object_literal() {
        let mut stream = TokenStream::new("Person { name: \"John\", age: 30 }", "1.0").unwrap();
        let result = parse_expression(&mut stream);
        assert!(result.is_ok());

        let expr = result.unwrap();
        if let Expression::Struct { members, .. } = expr {
            assert_eq!(members.len(), 2);
            assert_eq!(members[0].0, "name");
            assert_eq!(members[1].0, "age");
        } else {
            panic!("Expected struct expression");
        }
    }

    #[test]
    fn test_operator_precedence() {
        // Test that || has lower precedence than &&
        let mut stream = TokenStream::new("a && b || c", "1.0").unwrap();
        let result = parse_expression(&mut stream);
        assert!(result.is_ok());

        let expr = result.unwrap();
        if let Expression::BinaryOp {
            op, left, right, ..
        } = expr
        {
            // Should parse as (a && b) || c, so top-level operator is ||
            assert!(matches!(op, BinaryOperator::Or));
            // Left side should be a && b
            if let Expression::BinaryOp { op, .. } = left.as_ref() {
                assert!(matches!(op, BinaryOperator::And));
            } else {
                panic!("Expected left side to be && expression");
            }
        } else {
            panic!("Expected binary operation");
        }
    }

    #[test]
    fn test_arithmetic_precedence() {
        // Test that * has higher precedence than +
        let mut stream = TokenStream::new("1 + 2 * 3", "1.0").unwrap();
        let result = parse_expression(&mut stream);
        assert!(result.is_ok());

        let expr = result.unwrap();
        if let Expression::BinaryOp {
            op, left, right, ..
        } = expr
        {
            // Should parse as 1 + (2 * 3), so top-level operator is +
            assert!(matches!(op, BinaryOperator::Add));
            // Right side should be 2 * 3
            if let Expression::BinaryOp { op, .. } = right.as_ref() {
                assert!(matches!(op, BinaryOperator::Multiply));
            } else {
                panic!("Expected right side to be * expression");
            }
        } else {
            panic!("Expected binary operation");
        }
    }

    #[test]
    fn test_parse_pair_member_access() {
        // This should reproduce the bug with data.left
        let mut stream = TokenStream::new("data.left", "1.0").unwrap();
        let result = parse_expression(&mut stream);
        assert!(result.is_ok(), "Failed to parse data.left: {:?}", result);
        assert!(stream.is_eof());
    }

    #[test]
    fn test_parse_pair_member_access_right() {
        // This should also reproduce the bug with data.right
        let mut stream = TokenStream::new("data.right", "1.0").unwrap();
        let result = parse_expression(&mut stream);
        assert!(result.is_ok(), "Failed to parse data.right: {:?}", result);
        assert!(stream.is_eof());
    }

    #[test]
    fn test_parse_array_index_arithmetic() {
        // Test the specific case that was failing: arr[n-1]
        let mut stream = TokenStream::new("arr[num_files-1]", "1.0").unwrap();
        let result = parse_expression(&mut stream);
        assert!(
            result.is_ok(),
            "Failed to parse arr[num_files-1]: {:?}",
            result
        );
        assert!(stream.is_eof());
    }

    #[test]
    fn test_parse_array_index_addition() {
        // Test arithmetic with addition
        let mut stream = TokenStream::new("arr[i+1]", "1.0").unwrap();
        let result = parse_expression(&mut stream);
        assert!(result.is_ok(), "Failed to parse arr[i+1]: {:?}", result);
        assert!(stream.is_eof());
    }

    #[test]
    fn test_parse_array_index_unary_minus() {
        // Test unary minus (negative index)
        let mut stream = TokenStream::new("arr[-1]", "1.0").unwrap();
        let result = parse_expression(&mut stream);
        assert!(result.is_ok(), "Failed to parse arr[-1]: {:?}", result);
        assert!(stream.is_eof());
    }

    #[test]
    fn test_parse_array_index_complex_arithmetic() {
        // Test complex arithmetic expression
        let mut stream = TokenStream::new("arr[a*b-c+1]", "1.0").unwrap();
        let result = parse_expression(&mut stream);
        assert!(result.is_ok(), "Failed to parse arr[a*b-c+1]: {:?}", result);
        assert!(stream.is_eof());
    }

    #[test]
    fn test_parse_negative_literals() {
        // Test that negative numbers are parsed as unary minus + positive literal
        let mut stream = TokenStream::new("-42", "1.0").unwrap();
        let result = parse_expression(&mut stream);
        assert!(result.is_ok(), "Failed to parse -42: {:?}", result);

        let expr = result.unwrap();
        if let Expression::UnaryOp { op, operand, .. } = expr {
            assert!(matches!(op, UnaryOperator::Negate));
            if let Expression::Int { value, .. } = operand.as_ref() {
                assert_eq!(*value, 42);
            } else {
                panic!("Expected positive integer operand");
            }
        } else {
            panic!("Expected unary negation expression");
        }
    }

    #[test]
    fn test_parse_negative_floats() {
        // Test that negative floats are parsed as unary minus + positive literal
        let mut stream = TokenStream::new("-3.14", "1.0").unwrap();
        let result = parse_expression(&mut stream);
        assert!(result.is_ok(), "Failed to parse -3.14: {:?}", result);

        let expr = result.unwrap();
        if let Expression::UnaryOp { op, operand, .. } = expr {
            assert!(matches!(op, UnaryOperator::Negate));
            if let Expression::Float { value, .. } = operand.as_ref() {
                assert!((value - std::f64::consts::PI).abs() < 1e-2);
            } else {
                panic!("Expected positive float operand");
            }
        } else {
            panic!("Expected unary negation expression");
        }
    }
}
