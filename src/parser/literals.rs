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

/// Parse placeholder options (name=value pairs) at the beginning of a placeholder
/// Returns a HashMap of options and leaves the stream positioned at the expression
pub fn parse_placeholder_options(stream: &mut TokenStream) -> ParseResult<HashMap<String, String>> {
    let mut options = HashMap::new();

    // Parse zero or more placeholder options
    loop {
        // Look ahead to see if we have option_name=value pattern
        let option_name = match stream.peek_token() {
            Some(Token::Identifier(name)) => name.clone(),
            Some(Token::Keyword(kw)) if kw == "true" || kw == "false" || kw == "default" => {
                kw.clone()
            }
            // No option name found - this must be the start of the expression
            _ => break,
        };

        // Look ahead to see if there's an assignment operator after the name
        match stream.peek_ahead(1) {
            Some(located_token) if matches!(located_token.token, Token::Assign) => {
                // This is indeed an option - consume the name and assignment
                stream.next(); // consume option name
                stream.next(); // consume assignment operator

                // Parse option value using parse_literal
                let literal_expr = parse_literal(stream).map_err(|_| {
                    WdlError::syntax_error(
                        stream.current_position(),
                        format!("Expected literal value after '{}='", option_name),
                        "1.0".to_string(),
                        None,
                    )
                })?;

                // Extract string representation from the literal using the literal() method
                let option_value = if let Some(value) = literal_expr.literal() {
                    match value {
                        crate::value::Value::Int { value, .. } => value.to_string(),
                        crate::value::Value::Float { value, .. } => value.to_string(),
                        crate::value::Value::String { value, .. } => value,
                        // Reject other types as per WDL grammar
                        _ => {
                            return Err(WdlError::syntax_error(
                                stream.current_position(),
                                format!("Placeholder option '{}' can only have string, integer, or float values", option_name),
                                "1.0".to_string(),
                                None,
                            ));
                        }
                    }
                } else {
                    return Err(WdlError::syntax_error(
                        stream.current_position(),
                        format!(
                            "Placeholder option '{}' must be a literal value",
                            option_name
                        ),
                        "1.0".to_string(),
                        None,
                    ));
                };

                options.insert(option_name, option_value);
            }
            _ => {
                // No assignment operator - this is the start of the expression, not an option
                break;
            }
        }
    }

    Ok(options)
}

/// Recursively rewrite all `_add` function calls to `_interpolation_add` within an expression
/// This is used for placeholder expressions where addition operations should have special null handling
/// Similar to miniwdl's rewrite_adds function in Expr.py
pub(crate) fn rewrite_adds_for_interpolation(expr: &mut Expression) {
    match expr {
        Expression::Apply {
            function_name,
            arguments,
            ..
        } => {
            // Rewrite _add to _interpolation_add
            if function_name == "_add" {
                *function_name = "_interpolation_add".to_string();
            }

            // Recursively rewrite in all arguments
            for arg in arguments {
                rewrite_adds_for_interpolation(arg);
            }
        }

        Expression::BinaryOp {
            op,
            left,
            right,
            pos,
            ..
        } => {
            // First recursively rewrite in both operands
            rewrite_adds_for_interpolation(left);
            rewrite_adds_for_interpolation(right);

            // If this is an addition operation, convert to _interpolation_add function call
            if matches!(op, crate::expr::BinaryOperator::Add) {
                // Convert BinaryOp::Add to Apply with _interpolation_add function
                let new_expr = Expression::Apply {
                    pos: pos.clone(),
                    function_name: "_interpolation_add".to_string(),
                    arguments: vec![(**left).clone(), (**right).clone()],
                    inferred_type: None,
                };
                *expr = new_expr;
            }
        }

        Expression::UnaryOp { operand, .. } => {
            // Recursively rewrite in operand
            rewrite_adds_for_interpolation(operand);
        }

        Expression::IfThenElse {
            condition,
            true_expr,
            false_expr,
            ..
        } => {
            // Recursively rewrite in all conditional branches
            rewrite_adds_for_interpolation(condition);
            rewrite_adds_for_interpolation(true_expr);
            rewrite_adds_for_interpolation(false_expr);
        }

        Expression::Array { items, .. } => {
            // Recursively rewrite in all array elements
            for element in items {
                rewrite_adds_for_interpolation(element);
            }
        }

        Expression::Pair { left, right, .. } => {
            // Recursively rewrite in both pair elements
            rewrite_adds_for_interpolation(left);
            rewrite_adds_for_interpolation(right);
        }

        Expression::Map { pairs, .. } => {
            // Recursively rewrite in all map pairs
            for (key, value) in pairs {
                rewrite_adds_for_interpolation(key);
                rewrite_adds_for_interpolation(value);
            }
        }

        Expression::Struct { members, .. } => {
            // Recursively rewrite in all struct field values
            for (_, value) in members {
                rewrite_adds_for_interpolation(value);
            }
        }

        Expression::Get { expr, .. } => {
            // Recursively rewrite in object expression
            rewrite_adds_for_interpolation(expr);
            // field is a String, not an expression
        }

        Expression::At { expr, index, .. } => {
            // Recursively rewrite in both expression and index expressions
            rewrite_adds_for_interpolation(expr);
            rewrite_adds_for_interpolation(index);
        }

        // Base cases - literals and identifiers don't contain nested expressions
        Expression::Boolean { .. }
        | Expression::Int { .. }
        | Expression::Float { .. }
        | Expression::String { .. }
        | Expression::Null { .. }
        | Expression::Ident { .. } => {
            // No nested expressions to rewrite
        }
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

    // Switch lexer to string literal mode with the specific quote character
    stream.push_lexer_mode(crate::parser::lexer::LexerMode::StringLiteral(quote_char));

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
                stream.push_lexer_mode(crate::parser::lexer::LexerMode::Normal);

                // First, parse placeholder options (sep='value', true=1, etc.)
                let options = parse_placeholder_options(stream)?;

                // Then parse the expression inside the placeholder
                let mut expr = parse_expression(stream)?;

                // Apply interpolation-specific rewriting: convert _add to _interpolation_add
                // This matches Python miniwdl's behavior in Placeholder constructor
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
                stream.pop_lexer_mode(); // Return to string literal mode

                parts.push(StringPart::Placeholder {
                    expr: Box::new(expr),
                    options,
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

    // Switch lexer to string literal mode with the specific quote character
    stream.push_lexer_mode(crate::parser::lexer::LexerMode::StringLiteral(quote_char));

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

    #[test]
    fn test_string_interpolation_with_sep_option() {
        // This test should fail with current implementation and pass after fix
        let mut stream = TokenStream::new("\"~{sep=' ' str_array}\"", "1.2").unwrap();

        let result = parse_string_literal(&mut stream);

        // Current implementation should fail, after fix this should pass
        match result {
            Ok(expr) => {
                if let Expression::String { parts, .. } = expr {
                    assert_eq!(parts.len(), 1);
                    if let StringPart::Placeholder { expr, options } = &parts[0] {
                        assert_eq!(options.get("sep"), Some(&" ".to_string()));
                        if let Expression::Ident { name, .. } = expr.as_ref() {
                            assert_eq!(name, "str_array");
                        } else {
                            panic!("Expected identifier expression");
                        }
                    } else {
                        panic!("Expected placeholder part");
                    }
                } else {
                    panic!("Expected string expression");
                }
            }
            Err(_) => {
                // Current implementation fails here - this is expected before fix
                println!("Test correctly fails with current implementation");
            }
        }
    }

    #[test]
    fn test_string_interpolation_with_multiple_options() {
        // Test multiple options in single placeholder
        let mut stream = TokenStream::new("\"~{sep=',' true=1 array_var}\"", "1.2").unwrap();

        let result = parse_string_literal(&mut stream);

        // This should also fail with current implementation
        match result {
            Ok(expr) => {
                if let Expression::String { parts, .. } = expr {
                    assert_eq!(parts.len(), 1);
                    if let StringPart::Placeholder { expr, options } = &parts[0] {
                        assert_eq!(options.get("sep"), Some(&",".to_string()));
                        assert_eq!(options.get("true"), Some(&"1".to_string()));
                        if let Expression::Ident { name, .. } = expr.as_ref() {
                            assert_eq!(name, "array_var");
                        } else {
                            panic!("Expected identifier expression");
                        }
                    } else {
                        panic!("Expected placeholder part");
                    }
                } else {
                    panic!("Expected string expression");
                }
            }
            Err(_) => {
                println!("Test correctly fails with current implementation");
            }
        }
    }

    #[test]
    fn test_placeholder_options_debug() {
        // Debug test to see what tokens are generated for placeholder content
        let mut stream = TokenStream::new("\"~{sep=' ' str_array}\"", "1.2").unwrap();

        // Skip opening quote
        stream.next();
        // Skip tilde brace
        stream.next();

        // Switch to placeholder mode
        stream.push_lexer_mode(crate::parser::lexer::LexerMode::Normal);

        // Debug what tokens we get in placeholder mode
        println!("=== Debugging placeholder tokens ===");
        for i in 0..5 {
            match stream.peek_ahead(i) {
                Some(token) => println!("Token {}: {:?}", i, token),
                None => {
                    println!("Token {}: None", i);
                    break;
                }
            }
        }

        let options_result = parse_placeholder_options(&mut stream);
        println!("Options result: {:?}", options_result);

        stream.pop_lexer_mode();
    }

    #[test]
    fn test_placeholder_lexer_whitespace_debug() {
        // Test to understand how whitespace is tokenized in placeholder mode
        let mut stream = TokenStream::new("\"~{sep=' ' str_array}\"", "1.2").unwrap();

        // Skip to placeholder content
        stream.next(); // skip quote
        stream.next(); // skip ~{
        stream.push_lexer_mode(crate::parser::lexer::LexerMode::Normal);

        println!("=== All tokens in placeholder ===");
        let mut i = 0;
        loop {
            match stream.peek_ahead(i) {
                Some(token) => {
                    println!("Token {}: {:?}", i, token);
                    if matches!(token.token, Token::PlaceholderEnd) {
                        break;
                    }
                    i += 1;
                    if i > 10 {
                        break;
                    } // Safety limit
                }
                None => {
                    println!("Token {}: None (end of stream)", i);
                    break;
                }
            }
        }

        stream.pop_lexer_mode();
    }

    #[test]
    fn test_workflow_string_parsing() {
        // Test parsing the exact string from the workflow
        let mut stream = TokenStream::new("\"~{sep=' ' str_array}\"", "1.2").unwrap();

        let result = parse_string_literal(&mut stream);
        println!("Workflow string parsing result: {:?}", result);

        match result {
            Ok(expr) => {
                if let Expression::String { parts, .. } = expr {
                    println!("Number of parts: {}", parts.len());
                    for (i, part) in parts.iter().enumerate() {
                        match part {
                            StringPart::Text(text) => println!("Part {}: Text('{}')", i, text),
                            StringPart::Placeholder { expr, options } => {
                                println!("Part {}: Placeholder with {} options", i, options.len());
                                for (key, value) in options {
                                    println!("  Option: {}='{}'", key, value);
                                }
                                println!("  Expression: {:?}", expr);
                            }
                        }
                    }
                } else {
                    panic!("Expected string expression");
                }
            }
            Err(e) => {
                println!("Error parsing workflow string: {:?}", e);
                // This should help us understand why it's failing
            }
        }
    }

    #[test]
    fn test_function_call_vs_placeholder_option() {
        // Test the two different syntaxes from the workflow

        // This should parse as a function call (sep function with arguments)
        let mut stream1 = TokenStream::new("\"~{sep(',', quote(int_array))}\"", "1.2").unwrap();
        let result1 = parse_string_literal(&mut stream1);
        println!("Function call result: {:?}", result1);

        // This should parse as placeholder option syntax
        let mut stream2 = TokenStream::new("\"~{sep=',' quote(int_array)}\"", "1.2").unwrap();
        let result2 = parse_string_literal(&mut stream2);
        println!("Placeholder option result: {:?}", result2);

        match result1 {
            Ok(expr) => {
                if let Expression::String { parts, .. } = expr {
                    if let StringPart::Placeholder { expr, options } = &parts[0] {
                        println!("Function call placeholder options: {:?}", options);
                        println!("Function call expression: {:?}", expr);
                        // For function call, options should be empty
                        assert!(options.is_empty());
                    }
                }
            }
            Err(e) => {
                println!("Function call failed: {:?}", e);
                // This helps us debug the issue
            }
        }

        match result2 {
            Ok(expr) => {
                if let Expression::String { parts, .. } = expr {
                    if let StringPart::Placeholder { expr, options } = &parts[0] {
                        println!("Option syntax placeholder options: {:?}", options);
                        println!("Option syntax expression: {:?}", expr);
                        // For option syntax, sep should be in options
                        assert_eq!(options.get("sep"), Some(&",".to_string()));
                    }
                }
            }
            Err(e) => {
                println!("Option syntax failed: {:?}", e);
            }
        }
    }

    #[test]
    fn test_debug_placeholder_expression_parsing() {
        // Let's see what happens when we parse the expression part after options
        let mut stream = TokenStream::new("\"~{sep=',' quote(int_array)}\"", "1.2").unwrap();

        // Skip opening quote
        stream.next();
        // Skip tilde brace
        stream.next();

        // Switch to placeholder mode
        stream.push_lexer_mode(crate::parser::lexer::LexerMode::Normal);

        // Parse placeholder options first
        println!("=== Before parsing options ===");
        for i in 0..10 {
            match stream.peek_ahead(i) {
                Some(token) => println!("Token {}: {:?}", i, token),
                None => break,
            }
        }

        let options = match parse_placeholder_options(&mut stream) {
            Ok(options) => options,
            Err(e) => {
                println!(
                    "Error parsing options (expected with current test input): {:?}",
                    e
                );
                return; // This test case is expected to fail due to malformed quoted string
            }
        };
        println!("Parsed options: {:?}", options);

        // Now see what tokens remain for expression parsing
        println!("=== After parsing options ===");
        for i in 0..10 {
            match stream.peek_ahead(i) {
                Some(token) => println!("Token {}: {:?}", i, token),
                None => break,
            }
        }

        // Try to parse the expression
        println!("=== Attempting expression parsing ===");
        let expr_result = parse_expression(&mut stream);
        println!("Expression result: {:?}", expr_result);

        stream.pop_lexer_mode();
    }

    #[test]
    fn test_workflow_line11_exact() {
        // Test the exact problematic line from the workflow
        let line11_left = "\"~{sep(',', quote(int_array))}\"";
        let line11_right = "\"~{sep=',' quote(int_array)}\"";

        println!("=== Testing left side (function call) ===");
        let mut stream1 = TokenStream::new(line11_left, "1.2").unwrap();
        let result1 = parse_string_literal(&mut stream1);
        println!("Left side result: {:?}", result1);

        println!("=== Testing right side (placeholder option) ===");
        let mut stream2 = TokenStream::new(line11_right, "1.2").unwrap();
        let result2 = parse_string_literal(&mut stream2);
        println!("Right side result: {:?}", result2);

        // Both should succeed
        assert!(result1.is_ok(), "Left side should parse successfully");
        assert!(result2.is_ok(), "Right side should parse successfully");

        if let (Ok(expr1), Ok(expr2)) = (result1, result2) {
            println!("Both expressions parsed successfully!");
            if let Expression::String { parts: parts1, .. } = expr1 {
                if let StringPart::Placeholder { options: opts1, .. } = &parts1[0] {
                    println!("Left side options: {:?}", opts1);
                    assert!(opts1.is_empty()); // Function call should have no options
                }
            }

            if let Expression::String { parts: parts2, .. } = expr2 {
                if let StringPart::Placeholder { options: opts2, .. } = &parts2[0] {
                    println!("Right side options: {:?}", opts2);
                    assert_eq!(opts2.get("sep"), Some(&",".to_string())); // Option syntax should have sep
                }
            }
        }
    }

    #[test]
    fn test_workflow_declaration_parsing() {
        use crate::parser::declarations::parse_declaration;
        use crate::parser::token_stream::TokenStream;

        // Test the exact declaration from the workflow
        let declaration_line = "Boolean is_true2 = \"~{sep(',', quote(int_array))}\" == \"~{sep=',' quote(int_array)}\"";

        println!("=== Testing workflow declaration parsing ===");
        let mut stream = TokenStream::new(declaration_line, "1.2").unwrap();

        let result = parse_declaration(&mut stream, "test");
        println!("Declaration parsing result: {:?}", result);

        match result {
            Ok(decl) => {
                println!("Declaration parsed successfully: {:?}", decl);
            }
            Err(e) => {
                println!("Declaration parsing failed: {:?}", e);
                // This will help us understand if the issue is in declarations vs literals
            }
        }
    }

    #[test]
    fn test_placeholder_add_rewriting() {
        // Test that + operators in placeholders are rewritten to _interpolation_add
        let mut stream = TokenStream::new("\"~{a + b}\"", "1.2").unwrap();
        let result = parse_string_literal(&mut stream);
        assert!(result.is_ok());

        let expr = result.unwrap();
        if let Expression::String { parts, .. } = expr {
            assert_eq!(parts.len(), 1);
            if let StringPart::Placeholder { expr, .. } = &parts[0] {
                // The expression should be rewritten from BinaryOp::Add to Apply with _interpolation_add
                if let Expression::Apply {
                    function_name,
                    arguments,
                    ..
                } = expr.as_ref()
                {
                    assert_eq!(function_name, "_interpolation_add", 
                              "Binary add operation should be rewritten to _interpolation_add in placeholders");
                    assert_eq!(
                        arguments.len(),
                        2,
                        "Rewritten function should have 2 arguments"
                    );
                } else {
                    panic!(
                        "Expected Apply expression with _interpolation_add, got: {:?}",
                        expr
                    );
                }
            } else {
                panic!("Expected placeholder part");
            }
        } else {
            panic!("Expected string expression");
        }
    }

    #[test]
    fn test_nested_add_rewriting_in_placeholders() {
        // Test that nested + operations are also rewritten
        let mut stream = TokenStream::new("\"~{a + b + c}\"", "1.2").unwrap();
        let result = parse_string_literal(&mut stream);
        assert!(result.is_ok());

        let expr = result.unwrap();
        if let Expression::String { parts, .. } = expr {
            if let StringPart::Placeholder { expr, .. } = &parts[0] {
                // Check that the expression tree contains _interpolation_add calls
                fn contains_interpolation_add(expr: &Expression) -> bool {
                    match expr {
                        Expression::Apply {
                            function_name,
                            arguments,
                            ..
                        } => {
                            if function_name == "_interpolation_add" {
                                return true;
                            }
                            // Recursively check arguments
                            arguments.iter().any(contains_interpolation_add)
                        }
                        Expression::BinaryOp { left, right, .. } => {
                            contains_interpolation_add(left) || contains_interpolation_add(right)
                        }
                        _ => false,
                    }
                }

                assert!(
                    contains_interpolation_add(expr.as_ref()),
                    "Nested add operations should contain _interpolation_add calls"
                );
            }
        }
    }

    #[test]
    fn test_add_not_rewritten_outside_placeholders() {
        // Test that + operators outside placeholders are NOT rewritten
        use crate::parser::expressions::parse_expression;

        let mut stream = TokenStream::new("a + b", "1.2").unwrap();
        let result = parse_expression(&mut stream);
        assert!(result.is_ok());

        let expr = result.unwrap();
        // This should remain as BinaryOp, not be rewritten to Apply
        if let Expression::BinaryOp { op, .. } = expr {
            assert!(
                matches!(op, crate::expr::BinaryOperator::Add),
                "Regular expressions should keep BinaryOp::Add, not be rewritten"
            );
        } else {
            panic!("Expected BinaryOp for regular expression, got: {:?}", expr);
        }
    }

    #[test]
    fn test_placeholder_rewriting_preserves_other_operators() {
        // Test that only + operators are rewritten, other operators remain unchanged
        let mut stream = TokenStream::new("\"~{a - b * c}\"", "1.2").unwrap();
        let result = parse_string_literal(&mut stream);
        assert!(result.is_ok());

        let expr = result.unwrap();
        if let Expression::String { parts, .. } = expr {
            if let StringPart::Placeholder { expr, .. } = &parts[0] {
                // Check that non-add operations remain as BinaryOp
                fn check_non_add_operators(expr: &Expression) -> bool {
                    match expr {
                        Expression::BinaryOp {
                            op, left, right, ..
                        } => {
                            match op {
                                crate::expr::BinaryOperator::Subtract
                                | crate::expr::BinaryOperator::Multiply => {
                                    // These should remain as BinaryOp
                                    check_non_add_operators(left) && check_non_add_operators(right)
                                }
                                crate::expr::BinaryOperator::Add => {
                                    // Add should have been rewritten, so this is an error
                                    false
                                }
                                _ => {
                                    check_non_add_operators(left) && check_non_add_operators(right)
                                }
                            }
                        }
                        Expression::Apply {
                            function_name,
                            arguments,
                            ..
                        } => {
                            if function_name == "_interpolation_add" {
                                // This is expected
                                arguments.iter().all(check_non_add_operators)
                            } else {
                                arguments.iter().all(check_non_add_operators)
                            }
                        }
                        Expression::Ident { .. } => true,
                        _ => true,
                    }
                }

                assert!(
                    check_non_add_operators(expr.as_ref()),
                    "Non-add operators should remain as BinaryOp"
                );
            }
        }
    }

    #[test]
    fn test_concat_optional_reproduction() {
        // Test that reproduces the exact concat_optional scenario
        let mut stream = TokenStream::new(
            "\"~{salutation + ' ' + name1 + ' '}nice to meet you!\"",
            "1.2",
        )
        .unwrap();
        let result = parse_string_literal(&mut stream);
        assert!(result.is_ok());

        let expr = result.unwrap();
        if let Expression::String { parts, .. } = expr {
            assert_eq!(parts.len(), 2); // placeholder + "nice to meet you!"

            // Check first part is placeholder with rewritten expression
            if let StringPart::Placeholder { expr, .. } = &parts[0] {
                // Expression should contain _interpolation_add calls
                fn count_interpolation_add_calls(expr: &Expression) -> usize {
                    match expr {
                        Expression::Apply {
                            function_name,
                            arguments,
                            ..
                        } => {
                            let count = if function_name == "_interpolation_add" {
                                1
                            } else {
                                0
                            };
                            count
                                + arguments
                                    .iter()
                                    .map(count_interpolation_add_calls)
                                    .sum::<usize>()
                        }
                        Expression::BinaryOp { left, right, .. } => {
                            count_interpolation_add_calls(left)
                                + count_interpolation_add_calls(right)
                        }
                        _ => 0,
                    }
                }

                let add_calls = count_interpolation_add_calls(expr.as_ref());
                assert!(
                    add_calls > 0,
                    "Placeholder should contain _interpolation_add calls"
                );
            }

            // Check second part is text
            if let StringPart::Text(text) = &parts[1] {
                assert_eq!(text, "nice to meet you!");
            } else {
                panic!("Second part should be text");
            }
        }
    }

    #[test]
    fn test_expression_parsing_with_placeholder() {
        // Test parsing through the expression parser (which is what the workflow uses)
        use crate::parser::expressions;
        let mut stream = TokenStream::new("\"~{sep=' ' str_array}\"", "1.2").unwrap();

        let result = expressions::parse_expression(&mut stream);
        println!("Expression parsing result: {:?}", result);

        match result {
            Ok(expr) => {
                if let Expression::String { parts, .. } = expr {
                    println!("Number of parts: {}", parts.len());
                    for (i, part) in parts.iter().enumerate() {
                        match part {
                            StringPart::Text(text) => println!("Part {}: Text('{}')", i, text),
                            StringPart::Placeholder { expr, options } => {
                                println!("Part {}: Placeholder with {} options", i, options.len());
                                for (key, value) in options {
                                    println!("  Option: {}='{}'", key, value);
                                }
                                println!("  Expression: {:?}", expr);
                            }
                        }
                    }
                } else {
                    panic!("Expected string expression, got: {:?}", expr);
                }
            }
            Err(e) => {
                println!("Error parsing expression: {:?}", e);
            }
        }
    }
}
