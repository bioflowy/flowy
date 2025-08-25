//! Token-based declaration parsing for WDL

use super::token_stream::TokenStream;
use super::tokens::Token;
use super::parser_utils::ParseResult;
use super::types::parse_type;
use super::expressions::parse_expression;
use crate::tree::Declaration;
use crate::error::WdlError;

/// Parse a declaration: Type name = expression
pub fn parse_declaration(stream: &mut TokenStream, id_prefix: &str) -> ParseResult<Declaration> {
    let pos = stream.current_position();
    
    // Parse the type
    let decl_type = parse_type(stream)?;
    
    // Parse the variable name
    let name = match stream.peek_token() {
        Some(Token::Identifier(n)) => {
            let name = n.clone();
            stream.next();
            name
        }
        _ => {
            return Err(WdlError::syntax_error(
                stream.current_position(),
                "Expected variable name after type".to_string(),
                "1.0".to_string(),
                None,
            ));
        }
    };
    
    // Check for assignment
    let expr = if stream.peek_token() == Some(&Token::Assign) {
        stream.next(); // consume =
        Some(parse_expression(stream)?)
    } else {
        None
    };
    
    Ok(Declaration::new(
        pos,
        decl_type,
        name,
        expr,
        id_prefix,
    ))
}

/// Parse an input declaration (may have no initializer in inputs section)
pub fn parse_input_declaration(stream: &mut TokenStream) -> ParseResult<Declaration> {
    parse_declaration(stream, "input")
}

/// Parse an output declaration (usually has an initializer)
pub fn parse_output_declaration(stream: &mut TokenStream) -> ParseResult<Declaration> {
    parse_declaration(stream, "output")
}

/// Parse a list of declarations
pub fn parse_declaration_list(stream: &mut TokenStream, id_prefix: &str) -> ParseResult<Vec<Declaration>> {
    let mut declarations = Vec::new();
    
    // Parse declarations until we hit a closing brace or EOF
    while stream.peek_token() != Some(&Token::RightBrace) && !stream.is_eof() {
        // Skip any newlines
        while stream.peek_token() == Some(&Token::Newline) {
            stream.next();
        }
        
        // Check if we've reached the end
        if stream.peek_token() == Some(&Token::RightBrace) || stream.is_eof() {
            break;
        }
        
        // Parse a declaration
        let decl = parse_declaration(stream, id_prefix)?;
        declarations.push(decl);
        
        // Consume optional newlines after declaration
        while stream.peek_token() == Some(&Token::Newline) {
            stream.next();
        }
    }
    
    Ok(declarations)
}

/// Parse an input section: input { declarations }
pub fn parse_input_section(stream: &mut TokenStream) -> ParseResult<Vec<Declaration>> {
    // Expect "input" keyword
    match stream.peek_token() {
        Some(Token::Keyword(kw)) if kw == "input" => {
            stream.next();
        }
        _ => {
            return Err(WdlError::syntax_error(
                stream.current_position(),
                "Expected 'input' keyword".to_string(),
                "1.0".to_string(),
                None,
            ));
        }
    }
    
    // Expect opening brace
    stream.expect(Token::LeftBrace)?;
    
    // Parse declarations
    let declarations = parse_declaration_list(stream, "input")?;
    
    // Expect closing brace
    stream.expect(Token::RightBrace)?;
    
    Ok(declarations)
}

/// Parse an output section: output { declarations }
pub fn parse_output_section(stream: &mut TokenStream) -> ParseResult<Vec<Declaration>> {
    // Expect "output" keyword
    match stream.peek_token() {
        Some(Token::Keyword(kw)) if kw == "output" => {
            stream.next();
        }
        _ => {
            return Err(WdlError::syntax_error(
                stream.current_position(),
                "Expected 'output' keyword".to_string(),
                "1.0".to_string(),
                None,
            ));
        }
    }
    
    // Expect opening brace
    stream.expect(Token::LeftBrace)?;
    
    // Parse declarations
    let declarations = parse_declaration_list(stream, "output")?;
    
    // Expect closing brace
    stream.expect(Token::RightBrace)?;
    
    Ok(declarations)
}

/// Parse a private declarations section (for workflow/task body)
pub fn parse_private_declaration(stream: &mut TokenStream) -> ParseResult<Declaration> {
    parse_declaration(stream, "decl")
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::parser::token_stream::TokenStream;
    use crate::types::Type;
    
    #[test]
    fn test_parse_simple_declaration() {
        let mut stream = TokenStream::new("Int x = 42", "1.0").unwrap();
        let result = parse_declaration(&mut stream, "test");
        assert!(result.is_ok());
        
        let decl = result.unwrap();
        assert_eq!(decl.name, "x");
        assert_eq!(decl.decl_type, Type::int(false));
        assert!(decl.expr.is_some());
    }
    
    #[test]
    fn test_parse_declaration_without_init() {
        let mut stream = TokenStream::new("String name", "1.0").unwrap();
        let result = parse_declaration(&mut stream, "test");
        assert!(result.is_ok());
        
        let decl = result.unwrap();
        assert_eq!(decl.name, "name");
        assert_eq!(decl.decl_type, Type::string(false));
        assert!(decl.expr.is_none());
    }
    
    #[test]
    fn test_parse_optional_declaration() {
        let mut stream = TokenStream::new("File? optional_file", "1.0").unwrap();
        let result = parse_declaration(&mut stream, "test");
        assert!(result.is_ok());
        
        let decl = result.unwrap();
        assert_eq!(decl.name, "optional_file");
        assert_eq!(decl.decl_type, Type::file(true));
        assert!(decl.expr.is_none());
    }
    
    #[test]
    fn test_parse_array_declaration() {
        let mut stream = TokenStream::new("Array[String] items = [\"a\", \"b\"]", "1.0").unwrap();
        let result = parse_declaration(&mut stream, "test");
        assert!(result.is_ok());
        
        let decl = result.unwrap();
        assert_eq!(decl.name, "items");
        assert!(matches!(decl.decl_type, Type::Array { .. }));
        assert!(decl.expr.is_some());
    }
    
    #[test]
    fn test_parse_input_section() {
        let input = r#"input {
            String name
            Int count = 10
            File? optional_file
        }"#;
        
        let mut stream = TokenStream::new(input, "1.0").unwrap();
        let result = parse_input_section(&mut stream);
        assert!(result.is_ok());
        
        let declarations = result.unwrap();
        assert_eq!(declarations.len(), 3);
        assert_eq!(declarations[0].name, "name");
        assert_eq!(declarations[1].name, "count");
        assert_eq!(declarations[2].name, "optional_file");
    }
    
    #[test]
    fn test_parse_output_section() {
        let input = r#"output {
            String result = "done"
            Array[File] files = glob("*.txt")
        }"#;
        
        let mut stream = TokenStream::new(input, "1.0").unwrap();
        let result = parse_output_section(&mut stream);
        assert!(result.is_ok());
        
        let declarations = result.unwrap();
        assert_eq!(declarations.len(), 2);
        assert_eq!(declarations[0].name, "result");
        assert_eq!(declarations[1].name, "files");
    }
    
    #[test]
    fn test_parse_map_declaration() {
        let mut stream = TokenStream::new("Map[String, Int] counts = {\"a\": 1}", "1.0").unwrap();
        let result = parse_declaration(&mut stream, "test");
        assert!(result.is_ok());
        
        let decl = result.unwrap();
        assert_eq!(decl.name, "counts");
        assert!(matches!(decl.decl_type, Type::Map { .. }));
        assert!(decl.expr.is_some());
    }
    
    #[test]
    fn test_parse_complex_declaration() {
        let mut stream = TokenStream::new(
            "Array[Pair[String, File]]? results = [(\"a\", \"file.txt\")]",
            "1.0"
        ).unwrap();
        let result = parse_declaration(&mut stream, "test");
        assert!(result.is_ok());
        
        let decl = result.unwrap();
        assert_eq!(decl.name, "results");
        // Check it's an optional array of pairs
        if let Type::Array { optional, .. } = decl.decl_type {
            assert!(optional);
        } else {
            panic!("Expected Array type");
        }
    }
}