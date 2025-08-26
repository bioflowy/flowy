//! Parser utility functions for token-based parsing

use super::token_stream::TokenStream;
use super::tokens::Token;
use crate::error::WdlError;

/// Parse result type
pub type ParseResult<T> = Result<T, WdlError>;

/// Parser trait for token-based parsers
pub trait Parser<T> {
    fn parse(&self, stream: &mut TokenStream) -> ParseResult<T>;
}

/// Try to parse with backtracking on failure
pub fn try_parse<T, F>(stream: &mut TokenStream, parser: F) -> Option<T>
where
    F: FnOnce(&mut TokenStream) -> ParseResult<T>,
{
    let pos = stream.position();
    match parser(stream) {
        Ok(result) => Some(result),
        Err(_) => {
            stream.set_position(pos);
            None
        }
    }
}

/// Parse one of several alternatives
pub fn parse_alt<T>(stream: &mut TokenStream, parsers: &[&dyn Fn(&mut TokenStream) -> ParseResult<T>]) -> ParseResult<T> {
    let pos = stream.position();
    
    for parser in parsers {
        match parser(stream) {
            Ok(result) => return Ok(result),
            Err(_) => {
                stream.set_position(pos);
            }
        }
    }
    
    Err(WdlError::syntax_error(
        stream.current_position(),
        "No matching alternative found".to_string(),
        "1.0".to_string(), // TODO: Get version from stream
        None,
    ))
}

/// Parse a sequence of items separated by a delimiter
pub fn parse_separated<T, F>(
    stream: &mut TokenStream,
    delimiter: Token,
    parser: F,
) -> ParseResult<Vec<T>>
where
    F: Fn(&mut TokenStream) -> ParseResult<T>,
{
    let mut items = Vec::new();
    
    // Try to parse first item
    if let Some(first) = try_parse(stream, &parser) {
        items.push(first);
        
        // Parse remaining items
        while stream.try_consume(&delimiter).is_some() {
            items.push(parser(stream)?);
        }
    }
    
    Ok(items)
}

/// Parse a list of items enclosed in delimiters
pub fn parse_delimited_list<T, F>(
    stream: &mut TokenStream,
    open: Token,
    close: Token,
    separator: Token,
    parser: F,
) -> ParseResult<Vec<T>>
where
    F: Fn(&mut TokenStream) -> ParseResult<T>,
{
    stream.expect(open)?;
    
    let mut items = Vec::new();
    
    // Check for empty list
    if stream.peek_token() == Some(close.clone()) {
        stream.expect(close)?;
        return Ok(items);
    }
    
    // Parse first item
    items.push(parser(stream)?);
    
    // Parse remaining items
    while stream.peek_token() != Some(close.clone()) {
        stream.expect(separator.clone())?;
        
        // Allow trailing separator
        if stream.peek_token() == Some(close.clone()) {
            break;
        }
        
        items.push(parser(stream)?);
    }
    
    stream.expect(close)?;
    Ok(items)
}

/// Parse optional element
pub fn parse_optional<T, F>(stream: &mut TokenStream, parser: F) -> ParseResult<Option<T>>
where
    F: FnOnce(&mut TokenStream) -> ParseResult<T>,
{
    Ok(try_parse(stream, parser))
}

/// Parse many occurrences (0 or more)
pub fn parse_many<T, F>(stream: &mut TokenStream, parser: F) -> ParseResult<Vec<T>>
where
    F: Fn(&mut TokenStream) -> ParseResult<T>,
{
    let mut items = Vec::new();
    
    while let Some(item) = try_parse(stream, &parser) {
        items.push(item);
    }
    
    Ok(items)
}

/// Parse at least one occurrence
pub fn parse_many1<T, F>(stream: &mut TokenStream, parser: F) -> ParseResult<Vec<T>>
where
    F: Fn(&mut TokenStream) -> ParseResult<T>,
{
    let first = parser(stream)?;
    let mut items = vec![first];
    
    while let Some(item) = try_parse(stream, &parser) {
        items.push(item);
    }
    
    Ok(items)
}

/// Expect a specific keyword
pub fn expect_keyword(stream: &mut TokenStream, keyword: &str) -> ParseResult<()> {
    match stream.peek_token() {
        Some(Token::Keyword(kw)) if kw == keyword => {
            stream.next();
            Ok(())
        }
        _ => {
            Err(WdlError::syntax_error(
                stream.current_position(),
                format!("Expected keyword '{}'", keyword),
                "1.0".to_string(), // TODO: Get version from stream
                None,
            ))
        }
    }
}

/// Parse an identifier
pub fn parse_identifier(stream: &mut TokenStream) -> ParseResult<String> {
    match stream.peek_token() {
        Some(Token::Identifier(name)) => {
            let name = name.clone();
            stream.next();
            Ok(name)
        }
        _ => {
            Err(WdlError::syntax_error(
                stream.current_position(),
                "Expected identifier".to_string(),
                "1.0".to_string(), // TODO: Get version from stream
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
    fn test_try_parse() {
        let source = "foo bar";
        let mut stream = TokenStream::new(source, "1.0").unwrap();
        
        // Successful parse
        let result = try_parse(&mut stream, |s| parse_identifier(s));
        assert_eq!(result, Some("foo".to_string()));
        
        // Failed parse (not at identifier)
        let result = try_parse(&mut stream, |s| {
            s.expect(Token::Plus)
                .map(|_| ())
        });
        assert!(result.is_none());
        
        // Should still be at "bar"
        let result = parse_identifier(&mut stream).unwrap();
        assert_eq!(result, "bar");
    }
    
    #[test]
    fn test_parse_separated() {
        let source = "a, b, c";
        let mut stream = TokenStream::new(source, "1.0").unwrap();
        
        let result = parse_separated(&mut stream, Token::Comma, parse_identifier).unwrap();
        assert_eq!(result, vec!["a", "b", "c"]);
    }
    
    #[test]
    fn test_parse_delimited_list() {
        let source = "[1, 2, 3]";
        let mut stream = TokenStream::new(source, "1.0").unwrap();
        
        let result = parse_delimited_list(
            &mut stream,
            Token::LeftBracket,
            Token::RightBracket,
            Token::Comma,
            |s| {
                match s.peek_token() {
                    Some(Token::IntLiteral(n)) => {
                        s.next();
                        Ok(n)
                    }
                    _ => Err(WdlError::syntax_error(
                        s.current_position(),
                        "Expected integer".to_string(),
                        "1.0".to_string(),
                        None,
                    ))
                }
            },
        ).unwrap();
        
        assert_eq!(result, vec![1, 2, 3]);
    }
    
    #[test]
    fn test_parse_many() {
        let source = "foo bar baz";
        let mut stream = TokenStream::new(source, "1.0").unwrap();
        
        let result = parse_many(&mut stream, parse_identifier).unwrap();
        assert_eq!(result, vec!["foo", "bar", "baz"]);
        assert!(stream.is_eof());
    }
}